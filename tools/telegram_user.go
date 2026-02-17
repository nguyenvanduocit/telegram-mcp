package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

type getMeInput struct{}

type resolveUsernameInput struct {
	Username string `json:"username" jsonschema:"required"`
}

type getUserInput struct {
	UserID string `json:"user_id" jsonschema:"required"`
}

type searchContactsInput struct {
	Query string `json:"query" jsonschema:"required"`
	Limit int    `json:"limit,omitempty"`
}

func RegisterUserTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_me",
			mcp.WithDescription("Get information about the currently logged-in Telegram user"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTypedToolHandler(handleGetMe),
	)

	s.AddTool(
		mcp.NewTool("telegram_resolve_username",
			mcp.WithDescription("Resolve a @username to get user or channel info"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("username",
				mcp.Description("Telegram username with or without @ prefix"),
				mcp.Required(),
			),
		),
		mcp.NewTypedToolHandler(handleResolveUsername),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_user",
			mcp.WithDescription("Get detailed information about a Telegram user by ID or @username"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("user_id",
				mcp.Description("User ID (numeric) or @username"),
				mcp.Required(),
			),
		),
		mcp.NewTypedToolHandler(handleGetUser),
	)

	s.AddTool(
		mcp.NewTool("telegram_search_contacts",
			mcp.WithDescription("Search for Telegram users and chats by name or username substring"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("query",
				mcp.Description("Search query string"),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20)"),
			),
		),
		mcp.NewTypedToolHandler(handleSearchContacts),
	)
}

func handleGetMe(_ context.Context, _ mcp.CallToolRequest, input getMeInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	self := services.Self()
	if self == nil {
		return mcp.NewToolResultError("not logged in"), nil
	}

	fullResult, err := services.API().UsersGetFullUser(tgCtx, &tg.InputUserSelf{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get full user info: %v", err)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "User: %s", self.FirstName)
	if self.LastName != "" {
		fmt.Fprintf(&b, " %s", self.LastName)
	}
	if self.Username != "" {
		fmt.Fprintf(&b, " (@%s)", self.Username)
	}
	fmt.Fprintf(&b, "\nID: %d", self.ID)
	if self.Phone != "" {
		fmt.Fprintf(&b, "\nPhone: +%s", self.Phone)
	}
	if self.Bot {
		b.WriteString("\nType: Bot")
	} else {
		b.WriteString("\nType: User")
	}
	if fullResult.FullUser.About != "" {
		fmt.Fprintf(&b, "\nBio: %s", fullResult.FullUser.About)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleResolveUsername(_ context.Context, _ mcp.CallToolRequest, input resolveUsernameInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	username := strings.TrimPrefix(input.Username, "@")
	if username == "" {
		return mcp.NewToolResultError("username is required"), nil
	}

	resolved, err := services.API().ContactsResolveUsername(tgCtx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve @%s: %v", username, err)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Resolved @%s\n", username)

	// Show peer type
	switch p := resolved.Peer.(type) {
	case *tg.PeerUser:
		fmt.Fprintf(&b, "Peer Type: User\nPeer ID: %d\n", p.UserID)
	case *tg.PeerChat:
		fmt.Fprintf(&b, "Peer Type: Chat\nPeer ID: %d\n", p.ChatID)
	case *tg.PeerChannel:
		fmt.Fprintf(&b, "Peer Type: Channel\nPeer ID: %d\n", p.ChannelID)
	}

	// Show users
	for _, u := range resolved.Users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		b.WriteString("\n")
		formatUser(&b, user)
	}

	// Show chats/channels
	for _, c := range resolved.Chats {
		b.WriteString("\n")
		formatChat(&b, c)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetUser(_ context.Context, _ mcp.CallToolRequest, input getUserInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	if input.UserID == "" {
		return mcp.NewToolResultError("user_id is required"), nil
	}

	peer, err := services.ResolvePeer(tgCtx, input.UserID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	inputUser, ok := toInputUser(peer)
	if !ok {
		return mcp.NewToolResultError("the provided identifier does not resolve to a user"), nil
	}

	fullResult, err := services.API().UsersGetFullUser(tgCtx, inputUser)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get user info: %v", err)), nil
	}

	var b strings.Builder

	// Find the user object from the Users array
	for _, u := range fullResult.Users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		formatUser(&b, user)
		break
	}

	full := &fullResult.FullUser
	if full.About != "" {
		fmt.Fprintf(&b, "Bio: %s\n", full.About)
	}
	fmt.Fprintf(&b, "Common Chats: %d\n", full.CommonChatsCount)

	return mcp.NewToolResultText(b.String()), nil
}

func handleSearchContacts(_ context.Context, _ mcp.CallToolRequest, input searchContactsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	if input.Query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	found, err := services.API().ContactsSearch(tgCtx, &tg.ContactsSearchRequest{
		Q:     input.Query,
		Limit: limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for %q\n", input.Query)

	if len(found.Users) > 0 {
		fmt.Fprintf(&b, "\nUsers (%d):\n", len(found.Users))
		for _, u := range found.Users {
			user, ok := u.(*tg.User)
			if !ok {
				continue
			}
			b.WriteString("\n")
			formatUser(&b, user)
		}
	}

	if len(found.Chats) > 0 {
		fmt.Fprintf(&b, "\nChats/Channels (%d):\n", len(found.Chats))
		for _, c := range found.Chats {
			b.WriteString("\n")
			formatChat(&b, c)
		}
	}

	if len(found.Users) == 0 && len(found.Chats) == 0 {
		b.WriteString("\nNo results found.")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func toInputUser(p tg.InputPeerClass) (*tg.InputUser, bool) {
	u, ok := p.(*tg.InputPeerUser)
	if !ok {
		return nil, false
	}
	return &tg.InputUser{UserID: u.UserID, AccessHash: u.AccessHash}, true
}

func formatUser(b *strings.Builder, user *tg.User) {
	fmt.Fprintf(b, "User: %s", user.FirstName)
	if user.LastName != "" {
		fmt.Fprintf(b, " %s", user.LastName)
	}
	if user.Username != "" {
		fmt.Fprintf(b, " (@%s)", user.Username)
	}
	fmt.Fprintf(b, "\nID: %d\n", user.ID)
	if user.Phone != "" {
		fmt.Fprintf(b, "Phone: +%s\n", user.Phone)
	}
	if user.Bot {
		b.WriteString("Type: Bot\n")
	}
}

func formatChat(b *strings.Builder, chat tg.ChatClass) {
	switch c := chat.(type) {
	case *tg.Chat:
		fmt.Fprintf(b, "Chat: %s\nID: %d\nType: Group\nMembers: %d\n", c.Title, c.ID, c.ParticipantsCount)
	case *tg.Channel:
		fmt.Fprintf(b, "Channel: %s\nID: %d\n", c.Title, c.ID)
		if c.Username != "" {
			fmt.Fprintf(b, "Username: @%s\n", c.Username)
		}
		if c.Megagroup {
			b.WriteString("Type: Supergroup\n")
		} else if c.Broadcast {
			b.WriteString("Type: Broadcast Channel\n")
		} else {
			b.WriteString("Type: Channel\n")
		}
		if c.ParticipantsCount != 0 {
			fmt.Fprintf(b, "Members: %d\n", c.ParticipantsCount)
		}
	}
}
