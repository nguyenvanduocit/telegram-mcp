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
- **tools/** - MCP tool implementations organized by category (59 tools, 15 categories)
  - `telegram_auth.go` - Auth status, send code, send 2FA password
  - `telegram_message.go` - Send, search, forward, edit, delete, pin, translate, polls, typing, read history
  - `telegram_chat.go` - List, get, search, join, leave, create, pin/unread dialogs
  - `telegram_media.go` - Download, upload, file info, view image
  - `telegram_user.go` - Get user info, resolve usernames, search contacts
  - `telegram_contact.go` - Get contacts, import, block/unblock
  - `telegram_reaction.go` - Send reactions, get message reactions
  - `telegram_invite.go` - Export, list, revoke invite links
  - `telegram_notification.go` - Get/set notification settings
  - `telegram_forum.go` - Create, list, edit forum topics
  - `telegram_story.go` - Get, send, delete stories
  - `telegram_admin.go` - Admin rights, bans, participants, admin log
  - `telegram_draft.go` - Set and clear draft messages
  - `telegram_folder.go` - Get folders, get folder chats
  - `telegram_profile.go` - Update profile, get read participants
  - `telegram_compound.go` - Compound tools: get unread, chat context, bulk forward, export messages, cross-chat search
  - `telegram_prompts.go` - MCP Prompts: daily digest, community manager, content broadcaster

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

## Compound Tools

High-level tools that aggregate multiple API calls into one, reducing round-trips for AI agents:

- `telegram_get_unread` — All unread dialogs + preview messages (replaces list_chats + get_history × N)
- `telegram_chat_context` — Full chat snapshot: info + messages + pinned + participants (replaces 3-4 separate calls)
- `telegram_forward_bulk` — Forward to multiple destinations (replaces forward × N)
- `telegram_export_messages` — Auto-paginated history export up to 500 messages
- `telegram_search_cross_chat` — Search across multiple chats simultaneously

## MCP Prompts

Workflow recipes that guide AI through common tasks:

- `daily_digest` — Prioritized unread message digest
- `community_manager` — Community analysis and moderation (requires `peer` arg)
- `content_broadcaster` — Cross-post content to multiple channels (requires `source_peer`, `destinations` args)

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
