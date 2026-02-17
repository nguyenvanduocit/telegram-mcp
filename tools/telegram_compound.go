package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

// Get Unread

type getUnreadInput struct {
	Limit           int `json:"limit"`
	MessagesPerChat int `json:"messages_per_chat"`
}

// Chat Context

type chatContextInput struct {
	Peer         string `json:"peer" jsonschema:"required"`
	MessageLimit int    `json:"message_limit"`
}

// Forward Bulk

type forwardBulkInput struct {
	FromPeer   string `json:"from_peer" jsonschema:"required"`
	MessageIDs string `json:"message_ids" jsonschema:"required"`
	ToPeers    string `json:"to_peers" jsonschema:"required"`
}

// Export Messages

type exportMessagesInput struct {
	Peer  string `json:"peer" jsonschema:"required"`
	Limit int    `json:"limit"`
	Since int    `json:"since"`
}

// Search Cross Chat

type searchCrossChatInput struct {
	Query       string `json:"query" jsonschema:"required"`
	Peers       string `json:"peers" jsonschema:"required"`
	LimitPerChat int   `json:"limit_per_chat"`
}

func RegisterCompoundTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_unread",
			mcp.WithDescription("Get all unread dialogs with their latest messages in a single call"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithNumber("limit", mcp.Description("Max number of dialogs to scan (default 20)")),
			mcp.WithNumber("messages_per_chat", mcp.Description("Number of recent messages per unread chat (default 3)")),
		),
		mcp.NewTypedToolHandler(handleGetUnread),
	)

	s.AddTool(
		mcp.NewTool("telegram_chat_context",
			mcp.WithDescription("Get complete context for a chat: info, recent messages, pinned messages, and participants"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_limit", mcp.Description("Number of recent messages to retrieve (default 20)")),
		),
		mcp.NewTypedToolHandler(handleChatContext),
	)

	s.AddTool(
		mcp.NewTool("telegram_forward_bulk",
			mcp.WithDescription("Forward messages to multiple destinations in a single call"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("from_peer", mcp.Required(), mcp.Description("Source chat ID or @username")),
			mcp.WithString("message_ids", mcp.Required(), mcp.Description("Comma-separated message IDs to forward")),
			mcp.WithString("to_peers", mcp.Required(), mcp.Description("Comma-separated destination chat IDs or @usernames")),
		),
		mcp.NewTypedToolHandler(handleForwardBulk),
	)

	s.AddTool(
		mcp.NewTool("telegram_export_messages",
			mcp.WithDescription("Export message history with auto-pagination, retrieving more messages than single-call limit"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("limit", mcp.Description("Total number of messages to export (default 100, max 500)")),
			mcp.WithNumber("since", mcp.Description("Unix timestamp to filter messages after this date (optional)")),
		),
		mcp.NewTypedToolHandler(handleExportMessages),
	)

	s.AddTool(
		mcp.NewTool("telegram_search_cross_chat",
			mcp.WithDescription("Search for a query across multiple specific chats in a single call"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
			mcp.WithString("peers", mcp.Required(), mcp.Description("Comma-separated list of chat IDs or @usernames to search in")),
			mcp.WithNumber("limit_per_chat", mcp.Description("Maximum results per chat (default 10)")),
		),
		mcp.NewTypedToolHandler(handleSearchCrossChat),
	)
}

func handleGetUnread(_ context.Context, _ mcp.CallToolRequest, input getUnreadInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	msgsPerChat := input.MessagesPerChat
	if msgsPerChat <= 0 {
		msgsPerChat = 3
	}
	if msgsPerChat > 20 {
		msgsPerChat = 20
	}

	result, err := services.API().MessagesGetDialogs(tgCtx, &tg.MessagesGetDialogsRequest{
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

	services.StorePeers(tgCtx, chats, users)

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

	var sb strings.Builder
	unreadCount := 0

	for _, dc := range dialogs {
		d, ok := dc.(*tg.Dialog)
		if !ok || d.UnreadCount <= 0 {
			continue
		}
		unreadCount++

		// Identify the dialog
		var peerName string
		var peerInput tg.InputPeerClass

		switch p := d.Peer.(type) {
		case *tg.PeerUser:
			if user, ok := userMap[p.UserID]; ok {
				peerName = user.FirstName
				if user.LastName != "" {
					peerName += " " + user.LastName
				}
				if user.Username != "" {
					peerName += fmt.Sprintf(" (@%s)", user.Username)
				}
			} else {
				peerName = fmt.Sprintf("User %d", p.UserID)
			}
			peer, err := services.GetInputPeerByID(tgCtx, p.UserID)
			if err != nil {
				continue
			}
			peerInput = peer
		case *tg.PeerChat:
			if chat, ok := chatMap[p.ChatID]; ok {
				if c, ok := chat.(*tg.Chat); ok {
					peerName = fmt.Sprintf("[Group] %s", c.Title)
				}
			}
			if peerName == "" {
				peerName = fmt.Sprintf("[Group] %d", p.ChatID)
			}
			peer, err := services.GetInputPeerByID(tgCtx, p.ChatID)
			if err != nil {
				continue
			}
			peerInput = peer
		case *tg.PeerChannel:
			if chat, ok := chatMap[p.ChannelID]; ok {
				if ch, ok := chat.(*tg.Channel); ok {
					chatType := "Channel"
					if ch.Megagroup {
						chatType = "Supergroup"
					}
					peerName = fmt.Sprintf("[%s] %s", chatType, ch.Title)
					if ch.Username != "" {
						peerName += fmt.Sprintf(" (@%s)", ch.Username)
					}
				}
			}
			if peerName == "" {
				peerName = fmt.Sprintf("[Channel] %d", p.ChannelID)
			}
			peer, err := services.GetInputPeerByID(tgCtx, p.ChannelID)
			if err != nil {
				continue
			}
			peerInput = peer
		default:
			continue
		}

		fmt.Fprintf(&sb, "\n%s — %d unread\n", peerName, d.UnreadCount)

		// Fetch recent messages for this dialog
		histResult, err := services.API().MessagesGetHistory(tgCtx, &tg.MessagesGetHistoryRequest{
			Peer:  peerInput,
			Limit: msgsPerChat,
		})
		if err != nil {
			fmt.Fprintf(&sb, "  (failed to fetch messages: %v)\n", err)
			continue
		}

		msgs := extractMessages(tgCtx, histResult)
		if len(msgs) == 0 {
			sb.WriteString("  (no messages)\n")
			continue
		}

		for _, mc := range msgs {
			msg, ok := mc.(*tg.Message)
			if !ok {
				continue
			}
			t := time.Unix(int64(msg.Date), 0).UTC().Format("2006-01-02 15:04:05")
			text := msg.Message
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			fmt.Fprintf(&sb, "  [%d] (%s): %s\n", msg.ID, t, text)
		}
	}

	if unreadCount == 0 {
		return mcp.NewToolResultText("No unread dialogs."), nil
	}

	header := fmt.Sprintf("Unread dialogs: %d\n", unreadCount)
	return mcp.NewToolResultText(header + sb.String()), nil
}

func handleChatContext(_ context.Context, _ mcp.CallToolRequest, input chatContextInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	msgLimit := input.MessageLimit
	if msgLimit <= 0 {
		msgLimit = 20
	}
	if msgLimit > 100 {
		msgLimit = 100
	}

	var sb strings.Builder

	// Section 1: Chat info
	sb.WriteString("== Chat Info ==\n")

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		channel := &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash}

		fullResult, err := services.API().ChannelsGetFullChannel(tgCtx, channel)
		if err != nil {
			fmt.Fprintf(&sb, "Failed to get channel info: %v\n", err)
		} else {
			services.StorePeers(tgCtx, fullResult.Chats, fullResult.Users)
			for _, c := range fullResult.Chats {
				if ch, ok := c.(*tg.Channel); ok && ch.ID == p.ChannelID {
					fmt.Fprintf(&sb, "Title: %s\n", ch.Title)
					fmt.Fprintf(&sb, "ID: %d\n", ch.ID)
					if ch.Username != "" {
						fmt.Fprintf(&sb, "Username: @%s\n", ch.Username)
					}
					if ch.Megagroup {
						sb.WriteString("Type: Supergroup\n")
					} else if ch.Broadcast {
						sb.WriteString("Type: Broadcast Channel\n")
					} else {
						sb.WriteString("Type: Channel\n")
					}
					break
				}
			}
			if full, ok := fullResult.FullChat.(*tg.ChannelFull); ok {
				if full.About != "" {
					fmt.Fprintf(&sb, "Description: %s\n", full.About)
				}
				if count, ok := full.GetParticipantsCount(); ok {
					fmt.Fprintf(&sb, "Members: %d\n", count)
				}
				if count, ok := full.GetAdminsCount(); ok {
					fmt.Fprintf(&sb, "Admins: %d\n", count)
				}
			}
		}

		// Participants (for supergroups)
		sb.WriteString("\n== Participants (up to 50) ==\n")
		participants, err := services.API().ChannelsGetParticipants(tgCtx, &tg.ChannelsGetParticipantsRequest{
			Channel: channel,
			Filter:  &tg.ChannelParticipantsRecent{},
			Limit:   50,
		})
		if err != nil {
			fmt.Fprintf(&sb, "Failed to get participants: %v\n", err)
		} else if cp, ok := participants.(*tg.ChannelsChannelParticipants); ok {
			services.StorePeers(tgCtx, cp.Chats, cp.Users)
			userMap := make(map[int64]*tg.User)
			for _, u := range cp.Users {
				if user, ok := u.(*tg.User); ok {
					userMap[user.ID] = user
				}
			}
			for _, pp := range cp.Participants {
				var userID int64
				switch pt := pp.(type) {
				case *tg.ChannelParticipant:
					userID = pt.UserID
				case *tg.ChannelParticipantSelf:
					userID = pt.UserID
				case *tg.ChannelParticipantCreator:
					userID = pt.UserID
				case *tg.ChannelParticipantAdmin:
					userID = pt.UserID
				case *tg.ChannelParticipantBanned:
					continue
				case *tg.ChannelParticipantLeft:
					continue
				default:
					continue
				}
				if user, ok := userMap[userID]; ok {
					name := user.FirstName
					if user.LastName != "" {
						name += " " + user.LastName
					}
					fmt.Fprintf(&sb, "  %s (ID: %d)", name, user.ID)
					if user.Username != "" {
						fmt.Fprintf(&sb, " @%s", user.Username)
					}
					sb.WriteString("\n")
				}
			}
		}

	case *tg.InputPeerChat:
		fullResult, err := services.API().MessagesGetFullChat(tgCtx, p.ChatID)
		if err != nil {
			fmt.Fprintf(&sb, "Failed to get chat info: %v\n", err)
		} else {
			services.StorePeers(tgCtx, fullResult.Chats, fullResult.Users)
			for _, c := range fullResult.Chats {
				if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
					fmt.Fprintf(&sb, "Title: %s\n", chat.Title)
					fmt.Fprintf(&sb, "ID: %d\n", chat.ID)
					sb.WriteString("Type: Group\n")
					fmt.Fprintf(&sb, "Members: %d\n", chat.ParticipantsCount)
					break
				}
			}
			if full, ok := fullResult.FullChat.(*tg.ChatFull); ok {
				if full.About != "" {
					fmt.Fprintf(&sb, "Description: %s\n", full.About)
				}
			}
		}

	case *tg.InputPeerUser:
		result, err := services.API().UsersGetFullUser(tgCtx, &tg.InputUser{
			UserID:     p.UserID,
			AccessHash: p.AccessHash,
		})
		if err != nil {
			fmt.Fprintf(&sb, "Failed to get user info: %v\n", err)
		} else {
			services.StorePeers(tgCtx, result.Chats, result.Users)
			for _, u := range result.Users {
				if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
					sb.WriteString("Type: User\n")
					fmt.Fprintf(&sb, "Name: %s", user.FirstName)
					if user.LastName != "" {
						fmt.Fprintf(&sb, " %s", user.LastName)
					}
					sb.WriteString("\n")
					fmt.Fprintf(&sb, "ID: %d\n", user.ID)
					if user.Username != "" {
						fmt.Fprintf(&sb, "Username: @%s\n", user.Username)
					}
					if user.Phone != "" {
						fmt.Fprintf(&sb, "Phone: +%s\n", user.Phone)
					}
					break
				}
			}
			if result.FullUser.About != "" {
				fmt.Fprintf(&sb, "Bio: %s\n", result.FullUser.About)
			}
		}

	default:
		return mcp.NewToolResultError("unsupported peer type"), nil
	}

	// Section 2: Recent messages
	sb.WriteString("\n== Recent Messages ==\n")
	histResult, err := services.API().MessagesGetHistory(tgCtx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: msgLimit,
	})
	if err != nil {
		fmt.Fprintf(&sb, "Failed to get history: %v\n", err)
	} else {
		msgs := extractMessages(tgCtx, histResult)
		sb.WriteString(formatMessages(msgs))
	}

	// Section 3: Pinned messages
	sb.WriteString("\n== Pinned Messages ==\n")
	pinnedResult, err := services.API().MessagesSearch(tgCtx, &tg.MessagesSearchRequest{
		Peer:   peer,
		Q:      "",
		Filter: &tg.InputMessagesFilterPinned{},
		Limit:  20,
	})
	if err != nil {
		fmt.Fprintf(&sb, "Failed to get pinned messages: %v\n", err)
	} else {
		pinned := extractMessages(tgCtx, pinnedResult)
		if len(pinned) == 0 {
			sb.WriteString("No pinned messages.\n")
		} else {
			sb.WriteString(formatMessages(pinned))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleForwardBulk(_ context.Context, _ mcp.CallToolRequest, input forwardBulkInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	ids, err := parseMessageIDs(input.MessageIDs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid message_ids: %v", err)), nil
	}

	fromPeer, err := services.ResolvePeer(tgCtx, input.FromPeer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve from_peer: %v", err)), nil
	}

	destinations := strings.Split(input.ToPeers, ",")
	if len(destinations) == 0 {
		return mcp.NewToolResultError("no destinations provided"), nil
	}
	if len(destinations) > 20 {
		return mcp.NewToolResultError("too many destinations (max 20)"), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Forwarding %d message(s) to %d destination(s):\n", len(ids), len(destinations))

	successCount := 0
	for _, dest := range destinations {
		dest = strings.TrimSpace(dest)
		if dest == "" {
			continue
		}

		toPeer, err := services.ResolvePeer(tgCtx, dest)
		if err != nil {
			fmt.Fprintf(&sb, "\n  %s: FAILED (resolve: %v)", dest, err)
			continue
		}

		randomIDs := make([]int64, len(ids))
		for i := range randomIDs {
			randomIDs[i] = randomID()
		}

		_, err = services.API().MessagesForwardMessages(tgCtx, &tg.MessagesForwardMessagesRequest{
			FromPeer: fromPeer,
			ToPeer:   toPeer,
			ID:       ids,
			RandomID: randomIDs,
		})
		if err != nil {
			fmt.Fprintf(&sb, "\n  %s: FAILED (%v)", dest, err)
			continue
		}

		fmt.Fprintf(&sb, "\n  %s: OK", dest)
		successCount++
	}

	fmt.Fprintf(&sb, "\n\nCompleted: %d/%d destinations succeeded.", successCount, len(destinations))
	return mcp.NewToolResultText(sb.String()), nil
}

func handleExportMessages(_ context.Context, _ mcp.CallToolRequest, input exportMessagesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	totalLimit := input.Limit
	if totalLimit <= 0 {
		totalLimit = 100
	}
	if totalLimit > 500 {
		totalLimit = 500
	}

	var allMessages []tg.MessageClass
	offsetID := 0
	batchSize := 100

	for len(allMessages) < totalLimit {
		remaining := totalLimit - len(allMessages)
		fetchLimit := batchSize
		if remaining < fetchLimit {
			fetchLimit = remaining
		}

		result, err := services.API().MessagesGetHistory(tgCtx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			Limit:    fetchLimit,
			OffsetID: offsetID,
		})
		if err != nil {
			if len(allMessages) > 0 {
				break // return what we have so far
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to get history: %v", err)), nil
		}

		msgs := extractMessages(tgCtx, result)
		if len(msgs) == 0 {
			break
		}

		// Check since filter and collect messages
		hitSince := false
		for _, mc := range msgs {
			msg, ok := mc.(*tg.Message)
			if !ok {
				continue
			}
			if input.Since > 0 && msg.Date < input.Since {
				hitSince = true
				break
			}
			allMessages = append(allMessages, mc)
		}

		if hitSince {
			break
		}

		// Update offset for next batch
		lastMsg, ok := msgs[len(msgs)-1].(*tg.Message)
		if !ok {
			break
		}
		offsetID = lastMsg.ID

		if len(msgs) < fetchLimit {
			break // no more messages available
		}
	}

	if len(allMessages) == 0 {
		return mcp.NewToolResultText("No messages found."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Exported %d messages:\n\n", len(allMessages))
	sb.WriteString(formatMessages(allMessages))
	return mcp.NewToolResultText(sb.String()), nil
}

func handleSearchCrossChat(_ context.Context, _ mcp.CallToolRequest, input searchCrossChatInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	limitPerChat := input.LimitPerChat
	if limitPerChat <= 0 {
		limitPerChat = 10
	}
	if limitPerChat > 100 {
		limitPerChat = 100
	}

	peerList := strings.Split(input.Peers, ",")
	if len(peerList) == 0 {
		return mcp.NewToolResultError("no peers provided"), nil
	}
	if len(peerList) > 20 {
		return mcp.NewToolResultError("too many peers (max 20)"), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Cross-chat search for %q across %d chat(s):\n", input.Query, len(peerList))

	totalResults := 0
	for _, peerStr := range peerList {
		peerStr = strings.TrimSpace(peerStr)
		if peerStr == "" {
			continue
		}

		peer, err := services.ResolvePeer(tgCtx, peerStr)
		if err != nil {
			fmt.Fprintf(&sb, "\n--- %s ---\n", peerStr)
			fmt.Fprintf(&sb, "  Failed to resolve: %v\n", err)
			continue
		}

		result, err := services.API().MessagesSearch(tgCtx, &tg.MessagesSearchRequest{
			Peer:   peer,
			Q:      input.Query,
			Filter: &tg.InputMessagesFilterEmpty{},
			Limit:  limitPerChat,
		})
		if err != nil {
			fmt.Fprintf(&sb, "\n--- %s ---\n", peerStr)
			fmt.Fprintf(&sb, "  Search failed: %v\n", err)
			continue
		}

		msgs := extractMessages(tgCtx, result)

		fmt.Fprintf(&sb, "\n--- %s (%d results) ---\n", peerStr, len(msgs))
		if len(msgs) == 0 {
			sb.WriteString("  No results.\n")
		} else {
			sb.WriteString(formatMessages(msgs))
			totalResults += len(msgs)
		}
	}

	fmt.Fprintf(&sb, "\nTotal results: %d\n", totalResults)
	return mcp.NewToolResultText(sb.String()), nil
}
