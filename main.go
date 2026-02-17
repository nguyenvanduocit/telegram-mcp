package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
	"github.com/nguyenvanduocit/telegram-mcp/tools"
)

func main() {
	envFile := flag.String("env", "", "Path to environment file")
	httpPort := flag.String("http_port", "", "Port for HTTP server. If not provided, will use stdio")
	flag.Parse()

	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			fmt.Printf("Warning: Error loading env file %s: %v\n", *envFile, err)
		}
	}

	requiredEnvs := []string{"TELEGRAM_API_ID", "TELEGRAM_API_HASH", "TELEGRAM_PHONE"}
	var missing []string
	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		fmt.Println("Missing required environment variables:")
		for _, env := range missing {
			fmt.Printf("  - %s\n", env)
		}
		fmt.Println()
		fmt.Println("Setup:")
		fmt.Println("1. Get API credentials from https://my.telegram.org/apps")
		fmt.Println("2. Set environment variables:")
		fmt.Println("   TELEGRAM_API_ID=12345")
		fmt.Println("   TELEGRAM_API_HASH=your_api_hash")
		fmt.Println("   TELEGRAM_PHONE=+1234567890  (your Telegram account phone number)")
		fmt.Println("   TELEGRAM_SESSION_DIR=~/.telegram-mcp  (optional)")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := services.StartTelegram(ctx); err != nil && !isContextCanceled(err) {
			log.Printf("Telegram client error: %v", err)
		}
	}()

	mcpServer := server.NewMCPServer(
		"Telegram MCP",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	tools.RegisterAuthTools(mcpServer)
	tools.RegisterMessageTools(mcpServer)
	tools.RegisterChatTools(mcpServer)
	tools.RegisterMediaTools(mcpServer)
	tools.RegisterUserTools(mcpServer)
	tools.RegisterReactionTools(mcpServer)
	tools.RegisterInviteTools(mcpServer)
	tools.RegisterNotificationTools(mcpServer)
	tools.RegisterContactTools(mcpServer)
	tools.RegisterForumTools(mcpServer)
	tools.RegisterStoryTools(mcpServer)
	tools.RegisterAdminTools(mcpServer)
	tools.RegisterFolderTools(mcpServer)
	tools.RegisterProfileTools(mcpServer)
	tools.RegisterDraftTools(mcpServer)
	tools.RegisterCompoundTools(mcpServer)
	tools.RegisterPrompts(mcpServer)

	if *httpPort != "" {
		fmt.Printf("Starting Telegram MCP Server on http://localhost:%s/mcp\n", *httpPort)
		httpServer := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/mcp"))
		if err := httpServer.Start(fmt.Sprintf(":%s", *httpPort)); err != nil && !isContextCanceled(err) {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil && !isContextCanceled(err) {
			log.Fatalf("Server error: %v", err)
		}
	}
}

func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
