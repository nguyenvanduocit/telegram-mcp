package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	pebbledb "github.com/cockroachdb/pebble"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/contrib/pebble"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type AuthState string

const (
	AuthStateConnecting      AuthState = "connecting"
	AuthStateWaitingCode     AuthState = "waiting_code"
	AuthStateWaitingPassword AuthState = "waiting_password"
	AuthStateAuthenticated   AuthState = "authenticated"
	AuthStateError           AuthState = "error"
)

var (
	telegramAPI  *tg.Client
	telegramCtx  context.Context
	peerDB       *pebble.PeerStorage
	peerResolver *storage.ResolverCache
	selfUser     *tg.User
	ready        = make(chan struct{})
	readyOnce    sync.Once

	// Auth state
	authMu       sync.Mutex
	authCond     *sync.Cond
	authState    AuthState = AuthStateConnecting
	authErrorMsg string

	// Channels for MCP-driven auth
	authCodeCh     = make(chan string)
	authPasswordCh = make(chan string)
)

func init() {
	authCond = sync.NewCond(&authMu)
}

func setAuthState(s AuthState, errMsg string) {
	authMu.Lock()
	authState = s
	authErrorMsg = errMsg
	authMu.Unlock()
	authCond.Broadcast()
}

func GetAuthState() AuthState {
	authMu.Lock()
	defer authMu.Unlock()
	return authState
}

func GetAuthError() string {
	authMu.Lock()
	defer authMu.Unlock()
	return authErrorMsg
}

func waitAuthStateChange(from AuthState) AuthState {
	authMu.Lock()
	defer authMu.Unlock()
	for authState == from {
		authCond.Wait()
	}
	return authState
}

func SubmitCode(code string) (AuthState, error) {
	current := GetAuthState()
	if current != AuthStateWaitingCode {
		return current, fmt.Errorf("not waiting for code, current state: %s", current)
	}
	authCodeCh <- code
	newState := waitAuthStateChange(AuthStateWaitingCode)
	if newState == AuthStateError {
		return newState, fmt.Errorf("%s", GetAuthError())
	}
	return newState, nil
}

func SubmitPassword(password string) (AuthState, error) {
	current := GetAuthState()
	if current != AuthStateWaitingPassword {
		return current, fmt.Errorf("not waiting for password, current state: %s", current)
	}
	authPasswordCh <- password
	newState := waitAuthStateChange(AuthStateWaitingPassword)
	if newState == AuthStateError {
		return newState, fmt.Errorf("%s", GetAuthError())
	}
	return newState, nil
}

func ReadyCh() <-chan struct{} {
	return ready
}

func API() *tg.Client {
	<-ready
	return telegramAPI
}

func PeerStorage() *pebble.PeerStorage {
	<-ready
	return peerDB
}

func Resolver() *storage.ResolverCache {
	<-ready
	return peerResolver
}

func Self() *tg.User {
	<-ready
	return selfUser
}

func Context() context.Context {
	<-ready
	return telegramCtx
}

type mcpAuth struct {
	phone string
}

func (a mcpAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a mcpAuth) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	setAuthState(AuthStateWaitingCode, "")
	select {
	case code := <-authCodeCh:
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (a mcpAuth) Password(ctx context.Context) (string, error) {
	setAuthState(AuthStateWaitingPassword, "")
	select {
	case pwd := <-authPasswordCh:
		return pwd, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (mcpAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("signing up not supported")
}

func (mcpAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func StartTelegram(ctx context.Context) error {
	appID, err := strconv.Atoi(os.Getenv("TELEGRAM_API_ID"))
	if err != nil {
		return fmt.Errorf("invalid TELEGRAM_API_ID: %w", err)
	}
	appHash := os.Getenv("TELEGRAM_API_HASH")
	phone := os.Getenv("TELEGRAM_PHONE")

	sessionDir := os.Getenv("TELEGRAM_SESSION_DIR")
	if sessionDir == "" {
		home, _ := os.UserHomeDir()
		sessionDir = filepath.Join(home, ".telegram-mcp")
	}
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	lg, _ := zap.NewProduction()

	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(sessionDir, "session.json"),
	}

	db, err := pebbledb.Open(filepath.Join(sessionDir, "peers.pebble.db"), &pebbledb.Options{})
	if err != nil {
		return fmt.Errorf("open peer storage: %w", err)
	}
	defer func() { _ = db.Close() }()
	peerDB = pebble.NewPeerStorage(db)

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		lg.Warn("Flood wait", zap.Duration("wait", wait.Duration))
	})

	client := telegram.NewClient(appID, appHash, telegram.Options{
		Logger:         lg,
		SessionStorage: sessionStorage,
		Middlewares: []telegram.Middleware{
			waiter,
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
		},
	})

	return waiter.Run(ctx, func(ctx context.Context) error {
		return client.Run(ctx, func(ctx context.Context) error {
			flow := auth.NewFlow(mcpAuth{phone: phone}, auth.SendCodeOptions{})
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				setAuthState(AuthStateError, err.Error())
				return fmt.Errorf("auth: %w", err)
			}

			self, err := client.Self(ctx)
			if err != nil {
				return fmt.Errorf("get self: %w", err)
			}

			api := client.API()
			telegramAPI = api
			telegramCtx = ctx
			selfUser = self
			rc := storage.NewResolverCache(peer.Plain(api), peerDB)
			peerResolver = &rc

			log.Printf("Logged in as %s (@%s)\n", self.FirstName, self.Username)

			setAuthState(AuthStateAuthenticated, "")
			readyOnce.Do(func() { close(ready) })

			<-ctx.Done()
			return ctx.Err()
		})
	})
}

func GetInputPeerByID(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	p, err := PeerStorage().Find(ctx, storage.PeerKey{ID: chatID})
	if err != nil {
		return nil, fmt.Errorf("peer %d not found in local storage: %w", chatID, err)
	}
	return p.AsInputPeer(), nil
}

func ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	username = strings.TrimPrefix(username, "@")
	p, err := Resolver().ResolveDomain(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("resolve @%s: %w", username, err)
	}
	return p, nil
}

func ResolvePeer(ctx context.Context, identifier string) (tg.InputPeerClass, error) {
	if strings.HasPrefix(identifier, "@") {
		return ResolveUsername(ctx, identifier)
	}
	id, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil {
		return ResolveUsername(ctx, identifier)
	}
	return GetInputPeerByID(ctx, id)
}
