# Telegram MCP

Go-based [MCP](https://modelcontextprotocol.io/) server for Telegram using MTProto. Enables AI assistants to interact with Telegram as a **user account** (not a bot).

Built on [gotd/td](https://github.com/gotd/td) — a pure Go MTProto 2.0 implementation.

## Features

- Full user-account access via MTProto (not Bot API)
- Auth flow exposed as MCP tools — no terminal interaction needed
- Session persistence — authenticate once, auto-reconnect on restart
- Supports both stdio and HTTP (streamable) transport
- 22 tools covering messages, chats, media, users, and auth

## Installation

```bash
go install github.com/nguyenvanduocit/telegram-mcp@latest
```

## Setup

### 1. Get Telegram API credentials

Go to [https://my.telegram.org/apps](https://my.telegram.org/apps) and create an application.

### 2. Configure environment

```bash
export TELEGRAM_API_ID=12345
export TELEGRAM_API_HASH=your_api_hash
export TELEGRAM_PHONE=+1234567890  # your Telegram account phone number
export TELEGRAM_SESSION_DIR=~/.telegram-mcp  # optional
```

Or use an `.env` file:

```bash
go run . --env .env
```

### 3. Run

```bash
# stdio mode (for MCP clients like Claude Code)
telegram-mcp

# HTTP mode
telegram-mcp --http_port 3002

# or with env file
telegram-mcp --env .env
```

### 4. Authenticate

On first run, use the auth tools through your MCP client:

1. `telegram_auth_status` — check state (will show `waiting_code`)
2. `telegram_auth_send_code` — submit the code from SMS/Telegram
3. `telegram_auth_send_password` — submit 2FA password if enabled

Session is saved to disk. Subsequent runs auto-authenticate.

## Docker

```bash
docker build -t telegram-mcp .
docker run -e TELEGRAM_API_ID=... -e TELEGRAM_API_HASH=... -e TELEGRAM_PHONE=... -p 3002:8080 telegram-mcp --http_port 8080
```

## MCP Client Configuration

### Claude Code

```json
{
  "mcpServers": {
    "telegram": {
      "command": "telegram-mcp",
      "env": {
        "TELEGRAM_API_ID": "12345",
        "TELEGRAM_API_HASH": "your_api_hash",
        "TELEGRAM_PHONE": "+1234567890"
      }
    }
  }
}
```

## Tools

### Auth

| Tool | Description |
|------|-------------|
| `telegram_auth_status` | Check authentication state |
| `telegram_auth_send_code` | Submit SMS/app verification code |
| `telegram_auth_send_password` | Submit 2FA password |

### Messages

| Tool | Description |
|------|-------------|
| `telegram_send_message` | Send a message to a chat |
| `telegram_get_history` | Get message history from a chat |
| `telegram_search_messages` | Search messages in a chat |
| `telegram_forward_message` | Forward messages between chats |
| `telegram_edit_message` | Edit a message |
| `telegram_delete_message` | Delete messages |
| `telegram_pin_message` | Pin a message |

### Chats

| Tool | Description |
|------|-------------|
| `telegram_list_chats` | List user's dialogs/chats |
| `telegram_get_chat` | Get chat/channel/user details |
| `telegram_search_chats` | Search chats and channels globally |
| `telegram_join_chat` | Join a public chat or channel |
| `telegram_leave_chat` | Leave a chat or channel |
| `telegram_create_group` | Create a new group chat |

### Media

| Tool | Description |
|------|-------------|
| `telegram_download_media` | Download media from a message |
| `telegram_send_media` | Send a file/media to a chat |
| `telegram_get_file_info` | Get media info without downloading |

### Users

| Tool | Description |
|------|-------------|
| `telegram_get_me` | Get current user info |
| `telegram_resolve_username` | Resolve @username to user/channel |
| `telegram_get_user` | Get user details by ID or username |
| `telegram_search_contacts` | Search contacts by name or username |

## Architecture

```
main.go                    Entry point, env validation, MCP server setup
services/telegram.go       Telegram client singleton, auth state machine, peer resolution
tools/
  telegram_auth.go         Auth tools (status, code, password)
  telegram_message.go      Message tools (send, read, search, forward, edit, delete, pin)
  telegram_chat.go         Chat tools (list, get, search, join, leave, create)
  telegram_media.go        Media tools (download, upload, file info)
  telegram_user.go         User tools (get me, resolve, get user, search contacts)
```

## License

MIT
