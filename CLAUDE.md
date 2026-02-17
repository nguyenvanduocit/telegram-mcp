# Telegram MCP

Go-based MCP server for Telegram using MTProto (gotd/td). Enables AI assistants to interact with Telegram as a user account.

## Development Commands

```bash
go build -o telegram-mcp .
go run . --env .env --http_port 3002
go run . --env .env  # stdio mode
go test ./...
```

## Architecture

- **main.go** - Entry point, env validation, MCP server setup, tool registration
- **services/telegram.go** - Telegram client singleton (gotd/td), auth state machine, peer resolution
- **tools/** - MCP tool implementations organized by category
  - `telegram_auth.go` - Auth status, send code, send 2FA password (MCP-driven auth)
  - `telegram_message.go` - Send, read, search, forward, edit, delete, pin messages
  - `telegram_chat.go` - List, get, search, join, leave, create chats
  - `telegram_media.go` - Download, upload media, get file info
  - `telegram_user.go` - Get user info, resolve usernames, search contacts

## Key Dependencies

- `github.com/gotd/td` - Pure Go MTProto 2.0 client (MIT)
- `github.com/gotd/contrib` - Peer storage, flood wait, rate limiting
- `github.com/mark3labs/mcp-go` - Go MCP protocol implementation

## Tool Pattern

```go
type SomeInput struct {
    Field string `json:"field" validate:"required"`
}

func RegisterCategoryTools(s *server.MCPServer) {
    tool := mcp.NewTool("telegram_action",
        mcp.WithDescription("..."),
        mcp.WithString("field", mcp.Required(), mcp.Description("...")),
    )
    s.AddTool(tool, mcp.NewTypedToolHandler(handler))
}

func handler(ctx context.Context, request mcp.CallToolRequest, input SomeInput) (*mcp.CallToolResult, error) {
    api := services.API()
    // ...
}
```

## Configuration

Environment variables:
- `TELEGRAM_API_ID` - From https://my.telegram.org/apps
- `TELEGRAM_API_HASH` - From https://my.telegram.org/apps
- `TELEGRAM_PHONE` - Phone in international format (+1234567890)
- `TELEGRAM_SESSION_DIR` - Session storage path (default: ~/.telegram-mcp)

## Auth

Auth is exposed as MCP tools — no terminal interaction needed. MCP server starts immediately.

- `telegram_auth_status` — check state (`connecting`, `waiting_code`, `waiting_password`, `authenticated`, `error`)
- `telegram_auth_send_code` — submit verification code when state is `waiting_code`
- `telegram_auth_send_password` — submit 2FA password when state is `waiting_password`

Session is persisted to disk — subsequent runs auto-authenticate without needing code.

## Code Conventions

- Typed MCP handlers with input struct validation
- Services use singleton pattern with ready channel for async init
- Peer resolution: accepts both numeric IDs and @usernames
- Responses formatted as readable text for AI consumption
