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

type listChatsInput struct {
	Limit    int `json:"limit"`
	OffsetID int `json:"offset_id"`
}

type getChatInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

type searchChatsInput struct {
	Query string `json:"query" jsonschema:"required"`
	Limit int    `json:"limit"`
}

type joinChatInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

type leaveChatInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

type createGroupInput struct {
	Title string `json:"title" jsonschema:"required"`
	Users string `json:"users" jsonschema:"required"`
}

func RegisterChatTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_list_chats",
			mcp.WithDescription("List the user's dialogs/chats"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithNumber("limit", mcp.Description("Number of chats to retrieve (default 20)")),
			mcp.WithNumber("offset_id", mcp.Description("Offset message ID for pagination (default 0)")),
		),
		mcp.NewTypedToolHandler(handleListChats),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_chat",
			mcp.WithDescription("Get detailed information about a specific chat, channel, or user"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleGetChat),
	)

	s.AddTool(
		mcp.NewTool("telegram_search_chats",
			mcp.WithDescription("Search for chats and channels globally"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 20)")),
		),
		mcp.NewTypedToolHandler(handleSearchChats),
	)

	s.AddTool(
		mcp.NewTool("telegram_join_chat",
			mcp.WithDescription("Join a public chat/channel by username or invite link"),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("@username or invite link (https://t.me/+ or https://t.me/joinchat/)")),
		),
		mcp.NewTypedToolHandler(handleJoinChat),
	)

	s.AddTool(
		mcp.NewTool("telegram_leave_chat",
			mcp.WithDescription("Leave a chat or channel"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleLeaveChat),
	)

	s.AddTool(
		mcp.NewTool("telegram_create_group",
			mcp.WithDescription("Create a new group chat"),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("title", mcp.Required(), mcp.Description("Group title")),
			mcp.WithString("users", mcp.Required(), mcp.Description("Comma-separated user IDs or @usernames to invite")),
		),
		mcp.NewTypedToolHandler(handleCreateGroup),
	)
}

func handleListChats(_ context.Context, _ mcp.CallToolRequest, input listChatsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().MessagesGetDialogs(tgCtx, &tg.MessagesGetDialogsRequest{
		OffsetID:   input.OffsetID,
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get dialogs: %v", err)), nil
	}

	modified, ok := result.AsModified()
	if !ok {
		return mcp.NewToolResultError("no dialogs returned"), nil
	}

	dialogs := modified.GetDialogs()
	chats := modified.GetChats()
	users := modified.GetUsers()

	// Build lookup maps
	chatMap := make(map[int64]tg.ChatClass)
	for _, c := range chats {
		switch v := c.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v
		case *tg.Channel:
			chatMap[v.ID] = v
		}
	}

	userMap := make(map[int64]*tg.User)
	for _, u := range users {
		user, ok := u.(*tg.User)
		if ok {
			userMap[user.ID] = user
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Dialogs (%d):\n", len(dialogs))

	for _, dc := range dialogs {
		d, ok := dc.(*tg.Dialog)
		if !ok {
			continue
		}

		switch p := d.Peer.(type) {
		case *tg.PeerUser:
			if user, ok := userMap[p.UserID]; ok {
				name := user.FirstName
				if user.LastName != "" {
					name += " " + user.LastName
				}
				fmt.Fprintf(&b, "\n[User] %s (ID: %d)", name, p.UserID)
				if user.Username != "" {
					fmt.Fprintf(&b, " @%s", user.Username)
				}
			} else {
				fmt.Fprintf(&b, "\n[User] ID: %d", p.UserID)
			}
		case *tg.PeerChat:
			if chat, ok := chatMap[p.ChatID]; ok {
				if c, ok := chat.(*tg.Chat); ok {
					fmt.Fprintf(&b, "\n[Group] %s (ID: %d)", c.Title, p.ChatID)
				}
			} else {
				fmt.Fprintf(&b, "\n[Group] ID: %d", p.ChatID)
			}
		case *tg.PeerChannel:
			if chat, ok := chatMap[p.ChannelID]; ok {
				if ch, ok := chat.(*tg.Channel); ok {
					chatType := "Channel"
					if ch.Megagroup {
						chatType = "Supergroup"
					}
					fmt.Fprintf(&b, "\n[%s] %s (ID: %d)", chatType, ch.Title, p.ChannelID)
					if ch.Username != "" {
						fmt.Fprintf(&b, " @%s", ch.Username)
					}
				}
			} else {
				fmt.Fprintf(&b, "\n[Channel] ID: %d", p.ChannelID)
			}
		}

		if d.UnreadCount > 0 {
			fmt.Fprintf(&b, " [%d unread]", d.UnreadCount)
		}
		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetChat(_ context.Context, _ mcp.CallToolRequest, input getChatInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	var b strings.Builder

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		fullResult, err := services.API().ChannelsGetFullChannel(tgCtx, &tg.InputChannel{
			ChannelID:  p.ChannelID,
			AccessHash: p.AccessHash,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get channel info: %v", err)), nil
		}

		// Find channel in chats list
		for _, c := range fullResult.Chats {
			if ch, ok := c.(*tg.Channel); ok && ch.ID == p.ChannelID {
				fmt.Fprintf(&b, "Title: %s\n", ch.Title)
				fmt.Fprintf(&b, "ID: %d\n", ch.ID)
				if ch.Username != "" {
					fmt.Fprintf(&b, "Username: @%s\n", ch.Username)
				}
				if ch.Megagroup {
					b.WriteString("Type: Supergroup\n")
				} else if ch.Broadcast {
					b.WriteString("Type: Broadcast Channel\n")
				} else {
					b.WriteString("Type: Channel\n")
				}
				break
			}
		}

		switch full := fullResult.FullChat.(type) {
		case *tg.ChannelFull:
			if full.About != "" {
				fmt.Fprintf(&b, "Description: %s\n", full.About)
			}
			if count, ok := full.GetParticipantsCount(); ok {
				fmt.Fprintf(&b, "Members: %d\n", count)
			}
			if count, ok := full.GetAdminsCount(); ok {
				fmt.Fprintf(&b, "Admins: %d\n", count)
			}
		}

	case *tg.InputPeerChat:
		fullResult, err := services.API().MessagesGetFullChat(tgCtx, p.ChatID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get chat info: %v", err)), nil
		}

		for _, c := range fullResult.Chats {
			if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
				fmt.Fprintf(&b, "Title: %s\n", chat.Title)
				fmt.Fprintf(&b, "ID: %d\n", chat.ID)
				b.WriteString("Type: Group\n")
				fmt.Fprintf(&b, "Members: %d\n", chat.ParticipantsCount)
				break
			}
		}

		if full, ok := fullResult.FullChat.(*tg.ChatFull); ok {
			if full.About != "" {
				fmt.Fprintf(&b, "Description: %s\n", full.About)
			}
		}

	case *tg.InputPeerUser:
		fmt.Fprintf(&b, "Type: User\n")
		fmt.Fprintf(&b, "User ID: %d\n", p.UserID)

	default:
		return mcp.NewToolResultError("unsupported peer type"), nil
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleSearchChats(_ context.Context, _ mcp.CallToolRequest, input searchChatsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

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

	if len(found.Chats) > 0 {
		fmt.Fprintf(&b, "\nChats/Channels (%d):\n", len(found.Chats))
		for _, c := range found.Chats {
			b.WriteString("\n")
			formatChat(&b, c)
		}
	}

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

	if len(found.Chats) == 0 && len(found.Users) == 0 {
		b.WriteString("\nNo results found.")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleJoinChat(_ context.Context, _ mcp.CallToolRequest, input joinChatInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peerStr := input.Peer

	// Handle invite links
	var inviteHash string
	if strings.HasPrefix(peerStr, "https://t.me/+") {
		inviteHash = strings.TrimPrefix(peerStr, "https://t.me/+")
	} else if strings.HasPrefix(peerStr, "https://t.me/joinchat/") {
		inviteHash = strings.TrimPrefix(peerStr, "https://t.me/joinchat/")
	}
	if inviteHash != "" {
		_, err := services.API().MessagesImportChatInvite(tgCtx, inviteHash)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to join via invite link: %v", err)), nil
		}
		return mcp.NewToolResultText("Joined chat via invite link successfully."), nil
	}

	// Handle username - resolve and join channel
	peer, err := services.ResolvePeer(tgCtx, peerStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	channelPeer, ok := peer.(*tg.InputPeerChannel)
	if !ok {
		return mcp.NewToolResultError("peer is not a channel or supergroup"), nil
	}

	_, err = services.API().ChannelsJoinChannel(tgCtx, &tg.InputChannel{
		ChannelID:  channelPeer.ChannelID,
		AccessHash: channelPeer.AccessHash,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to join channel: %v", err)), nil
	}

	return mcp.NewToolResultText("Joined channel successfully."), nil
}

func handleLeaveChat(_ context.Context, _ mcp.CallToolRequest, input leaveChatInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = services.API().ChannelsLeaveChannel(tgCtx, &tg.InputChannel{
			ChannelID:  p.ChannelID,
			AccessHash: p.AccessHash,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to leave channel: %v", err)), nil
		}

	case *tg.InputPeerChat:
		_, err = services.API().MessagesDeleteChatUser(tgCtx, &tg.MessagesDeleteChatUserRequest{
			ChatID: p.ChatID,
			UserID: services.Self().AsInput(),
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to leave chat: %v", err)), nil
		}

	default:
		return mcp.NewToolResultError("cannot leave this type of peer"), nil
	}

	return mcp.NewToolResultText("Left chat successfully."), nil
}

func handleCreateGroup(_ context.Context, _ mcp.CallToolRequest, input createGroupInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	parts := strings.Split(input.Users, ",")
	var inputUsers []tg.InputUserClass

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		peer, err := services.ResolvePeer(tgCtx, part)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to resolve user %q: %v", part, err)), nil
		}

		userPeer, ok := peer.(*tg.InputPeerUser)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("%q is not a user", part)), nil
		}

		inputUsers = append(inputUsers, &tg.InputUser{
			UserID:     userPeer.UserID,
			AccessHash: userPeer.AccessHash,
		})
	}

	if len(inputUsers) == 0 {
		return mcp.NewToolResultError("no valid users provided"), nil
	}

	_, err := services.API().MessagesCreateChat(tgCtx, &tg.MessagesCreateChatRequest{
		Title: input.Title,
		Users: inputUsers,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create group: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Group %q created successfully.", input.Title)), nil
}
