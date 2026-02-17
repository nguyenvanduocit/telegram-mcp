# Telegram MCP

Go-based [MCP](https://modelcontextprotocol.io/) server for Telegram using MTProto. Enables AI assistants to interact with Telegram as a **user account** (not a bot).

Built on [gotd/td](https://github.com/gotd/td) — a pure Go MTProto 2.0 implementation.

## Features

- **Full user-account access** via MTProto (not Bot API) — access everything a real user can
- **54 tools** across 12 categories: messages, chats, media, contacts, reactions, stories, forums, admin, and more
- **MCP-driven auth** — no terminal interaction needed, authenticate entirely through your AI client
- **Session persistence** — authenticate once, auto-reconnect on restart
- **Dual transport** — stdio (for Claude Code, Cursor, etc.) and HTTP (streamable)
- **Docker ready** — deploy anywhere with a single container

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
telegram-mcp --env .env
```

### 3. Run

```bash
# stdio mode (for MCP clients like Claude Code)
telegram-mcp --env .env

# HTTP mode
telegram-mcp --env .env --http_port 3002
```

### 4. Authenticate

On first run, use the auth tools through your MCP client:

1. `telegram_auth_status` — check state (will show `waiting_code`)
2. `telegram_auth_send_code` — submit the code from SMS/Telegram
3. `telegram_auth_send_password` — submit 2FA password if enabled

Session is saved to disk. Subsequent runs auto-authenticate.

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

### Docker

```bash
docker build -t telegram-mcp .
docker run -e TELEGRAM_API_ID=... -e TELEGRAM_API_HASH=... -e TELEGRAM_PHONE=... -p 3002:8080 telegram-mcp --http_port 8080
```

## Tools (54)

### Auth (3)

| Tool | Description |
|------|-------------|
| `telegram_auth_status` | Check authentication state |
| `telegram_auth_send_code` | Submit SMS/app verification code |
| `telegram_auth_send_password` | Submit 2FA password |

### Messages (14)

| Tool | Description |
|------|-------------|
| `telegram_send_message` | Send a message (supports replies and scheduled messages) |
| `telegram_get_history` | Get message history with pagination |
| `telegram_search_messages` | Search messages in a specific chat |
| `telegram_search_global` | Search messages across all chats |
| `telegram_forward_message` | Forward messages between chats |
| `telegram_edit_message` | Edit a sent message |
| `telegram_delete_message` | Delete messages |
| `telegram_pin_message` | Pin a message |
| `telegram_unpin_all_messages` | Unpin all pinned messages |
| `telegram_read_history` | Mark messages as read |
| `telegram_set_typing` | Set typing/recording status |
| `telegram_delete_history` | Delete entire chat history |
| `telegram_translate` | Translate a message to another language |
| `telegram_send_poll` | Send a poll or quiz |

### Chats (8)

| Tool | Description |
|------|-------------|
| `telegram_list_chats` | List dialogs/chats with pagination |
| `telegram_get_chat` | Get detailed chat/channel/user info |
| `telegram_search_chats` | Search chats and channels globally |
| `telegram_join_chat` | Join by username or invite link |
| `telegram_leave_chat` | Leave a chat or channel |
| `telegram_create_group` | Create a new group chat |
| `telegram_toggle_dialog_pin` | Pin/unpin a chat in the chat list |
| `telegram_mark_dialog_unread` | Mark/unmark a chat as unread |

### Media (4)

| Tool | Description |
|------|-------------|
| `telegram_download_media` | Download media from a message |
| `telegram_send_media` | Upload and send a file |
| `telegram_get_file_info` | Get media metadata without downloading |
| `telegram_view_image` | Download photo and return as image content for AI viewing |

### Users (4)

| Tool | Description |
|------|-------------|
| `telegram_get_me` | Get current user info |
| `telegram_resolve_username` | Resolve @username to user/channel |
| `telegram_get_user` | Get user details by ID or username |
| `telegram_search_contacts` | Search contacts by name or username |

### Contacts (3)

| Tool | Description |
|------|-------------|
| `telegram_get_contacts` | Get the full contact list |
| `telegram_import_contacts` | Import a contact by phone number |
| `telegram_block_peer` | Block or unblock a user |

### Reactions (2)

| Tool | Description |
|------|-------------|
| `telegram_send_reaction` | React to a message (emoji or custom) |
| `telegram_get_message_reactions` | Get reactions on a message |

### Invite Links (3)

| Tool | Description |
|------|-------------|
| `telegram_export_invite_link` | Create a new invite link |
| `telegram_get_invite_links` | List exported invite links |
| `telegram_revoke_invite_link` | Revoke an invite link |

### Notifications (2)

| Tool | Description |
|------|-------------|
| `telegram_get_notify_settings` | Get notification settings for a chat |
| `telegram_set_notify_settings` | Update mute/silent/preview settings |

### Forum Topics (3)

| Tool | Description |
|------|-------------|
| `telegram_create_forum_topic` | Create a topic in a forum supergroup |
| `telegram_get_forum_topics` | List forum topics |
| `telegram_edit_forum_topic` | Edit topic title or open/close state |

### Stories (4)

| Tool | Description |
|------|-------------|
| `telegram_get_peer_stories` | Get active stories of a peer |
| `telegram_get_all_stories` | Get all active stories from all peers |
| `telegram_send_story` | Post a photo or video story |
| `telegram_delete_stories` | Delete stories |

### Admin (4)

| Tool | Description |
|------|-------------|
| `telegram_edit_admin` | Edit admin rights for a user |
| `telegram_edit_banned` | Ban/restrict a user |
| `telegram_get_participants` | List channel/supergroup members |
| `telegram_get_admin_log` | View admin action log |

## Architecture

```
main.go                       Entry point, server setup, tool registration
services/telegram.go          Telegram client, auth state machine, peer resolution
tools/
  telegram_auth.go            Auth (status, code, password)
  telegram_message.go         Messages (send, search, forward, edit, delete, pin, polls, translate)
  telegram_chat.go            Chats (list, get, search, join, leave, create, pin/unread dialogs)
  telegram_media.go           Media (download, upload, file info, view image)
  telegram_user.go            Users (get me, resolve, get user, search contacts)
  telegram_contact.go         Contacts (get all, import, block/unblock)
  telegram_reaction.go        Reactions (send, get)
  telegram_invite.go          Invite links (export, list, revoke)
  telegram_notification.go    Notifications (get/set settings)
  telegram_forum.go           Forum topics (create, list, edit)
  telegram_story.go           Stories (get, send, delete)
  telegram_admin.go           Admin (rights, bans, participants, action log)
```

## License

MIT
