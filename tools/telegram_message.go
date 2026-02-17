package tools

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

func randomID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

func formatMessages(msgs []tg.MessageClass) string {
	if len(msgs) == 0 {
		return "No messages found."
	}

	var sb strings.Builder
	for _, mc := range msgs {
		msg, ok := mc.(*tg.Message)
		if !ok {
			continue
		}

		t := time.Unix(int64(msg.Date), 0).UTC().Format("2006-01-02 15:04:05")

		var senderID int64
		if msg.FromID != nil {
			switch p := msg.FromID.(type) {
			case *tg.PeerUser:
				senderID = p.UserID
			case *tg.PeerChat:
				senderID = p.ChatID
			case *tg.PeerChannel:
				senderID = p.ChannelID
			}
		}

		fmt.Fprintf(&sb, "[%d] %d (%s): %s\n", msg.ID, senderID, t, msg.Message)
	}

	return sb.String()
}

func extractMessages(ctx context.Context, result tg.MessagesMessagesClass) []tg.MessageClass {
	modified, ok := result.AsModified()
	if !ok {
		return nil
	}
	services.StorePeers(ctx, modified.GetChats(), modified.GetUsers())
	return modified.GetMessages()
}

func parseMessageIDs(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	if len(parts) > 100 {
		return nil, fmt.Errorf("too many message IDs (max 100)")
	}
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid message ID %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no message IDs provided")
	}
	return ids, nil
}

// Send Message

type sendMessageInput struct {
	Peer         string `json:"peer" jsonschema:"required"`
	Message      string `json:"message" jsonschema:"required"`
	ReplyToMsgID int    `json:"reply_to_msg_id"`
}

// Get History

type getHistoryInput struct {
	Peer     string `json:"peer" jsonschema:"required"`
	Limit    int    `json:"limit"`
	OffsetID int    `json:"offset_id"`
}

// Search Messages

type searchMessagesInput struct {
	Peer  string `json:"peer" jsonschema:"required"`
	Query string `json:"query" jsonschema:"required"`
	Limit int    `json:"limit"`
}

// Forward Message

type forwardMessageInput struct {
	FromPeer   string `json:"from_peer" jsonschema:"required"`
	ToPeer     string `json:"to_peer" jsonschema:"required"`
	MessageIDs string `json:"message_ids" jsonschema:"required"`
}

// Delete Message

type deleteMessageInput struct {
	Peer       string `json:"peer" jsonschema:"required"`
	MessageIDs string `json:"message_ids" jsonschema:"required"`
	Revoke     *bool  `json:"revoke"`
}

// Edit Message

type editMessageInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
	Message   string `json:"message" jsonschema:"required"`
}

// Pin Message

type pinMessageInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
	Silent    bool   `json:"silent"`
}

func RegisterMessageTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_send_message",
			mcp.WithDescription("Send a message to a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("message", mcp.Required(), mcp.Description("Message text to send")),
			mcp.WithNumber("reply_to_msg_id", mcp.Description("Message ID to reply to (optional)")),
		),
		mcp.NewTypedToolHandler(handleSendMessage),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_history",
			mcp.WithDescription("Get message history from a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("limit", mcp.Description("Number of messages to retrieve (default 20)")),
			mcp.WithNumber("offset_id", mcp.Description("Offset message ID for pagination (default 0)")),
		),
		mcp.NewTypedToolHandler(handleGetHistory),
	)

	s.AddTool(
		mcp.NewTool("telegram_search_messages",
			mcp.WithDescription("Search messages in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 20)")),
		),
		mcp.NewTypedToolHandler(handleSearchMessages),
	)

	s.AddTool(
		mcp.NewTool("telegram_forward_message",
			mcp.WithDescription("Forward messages between Telegram chats"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("from_peer", mcp.Required(), mcp.Description("Source chat ID or @username")),
			mcp.WithString("to_peer", mcp.Required(), mcp.Description("Destination chat ID or @username")),
			mcp.WithString("message_ids", mcp.Required(), mcp.Description("Comma-separated message IDs to forward")),
		),
		mcp.NewTypedToolHandler(handleForwardMessage),
	)

	s.AddTool(
		mcp.NewTool("telegram_delete_message",
			mcp.WithDescription("Delete messages from a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("message_ids", mcp.Required(), mcp.Description("Comma-separated message IDs to delete")),
			mcp.WithBoolean("revoke", mcp.Description("Delete for everyone (default true)")),
		),
		mcp.NewTypedToolHandler(handleDeleteMessage),
	)

	s.AddTool(
		mcp.NewTool("telegram_edit_message",
			mcp.WithDescription("Edit a message in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to edit")),
			mcp.WithString("message", mcp.Required(), mcp.Description("New message text")),
		),
		mcp.NewTypedToolHandler(handleEditMessage),
	)

	s.AddTool(
		mcp.NewTool("telegram_pin_message",
			mcp.WithDescription("Pin a message in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to pin")),
			mcp.WithBoolean("silent", mcp.Description("Pin silently without notification")),
		),
		mcp.NewTypedToolHandler(handlePinMessage),
	)
}

func handleSendMessage(_ context.Context, _ mcp.CallToolRequest, input sendMessageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	req := &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  input.Message,
		RandomID: randomID(),
	}

	if input.ReplyToMsgID != 0 {
		req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: input.ReplyToMsgID})
	}

	_, err = services.API().MessagesSendMessage(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	return mcp.NewToolResultText("Message sent successfully."), nil
}

func handleGetHistory(_ context.Context, _ mcp.CallToolRequest, input getHistoryInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().MessagesGetHistory(tgCtx, &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		Limit:    limit,
		OffsetID: input.OffsetID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get history: %v", err)), nil
	}

	msgs := extractMessages(tgCtx, result)
	return mcp.NewToolResultText(formatMessages(msgs)), nil
}

func handleSearchMessages(_ context.Context, _ mcp.CallToolRequest, input searchMessagesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().MessagesSearch(tgCtx, &tg.MessagesSearchRequest{
		Peer:   peer,
		Q:      input.Query,
		Filter: &tg.InputMessagesFilterEmpty{},
		Limit:  limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search messages: %v", err)), nil
	}

	msgs := extractMessages(tgCtx, result)
	return mcp.NewToolResultText(formatMessages(msgs)), nil
}

func handleForwardMessage(_ context.Context, _ mcp.CallToolRequest, input forwardMessageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	fromPeer, err := services.ResolvePeer(tgCtx, input.FromPeer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve from_peer: %v", err)), nil
	}

	toPeer, err := services.ResolvePeer(tgCtx, input.ToPeer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve to_peer: %v", err)), nil
	}

	ids, err := parseMessageIDs(input.MessageIDs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid message_ids: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to forward messages: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Forwarded %d message(s) successfully.", len(ids))), nil
}

func handleDeleteMessage(_ context.Context, _ mcp.CallToolRequest, input deleteMessageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	ids, err := parseMessageIDs(input.MessageIDs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid message_ids: %v", err)), nil
	}

	revoke := true
	if input.Revoke != nil {
		revoke = *input.Revoke
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = services.API().ChannelsDeleteMessages(tgCtx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			ID:      ids,
		})
	default:
		_, err = services.API().MessagesDeleteMessages(tgCtx, &tg.MessagesDeleteMessagesRequest{
			ID:     ids,
			Revoke: revoke,
		})
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete messages: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Deleted %d message(s) successfully.", len(ids))), nil
}

func handleEditMessage(_ context.Context, _ mcp.CallToolRequest, input editMessageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	editReq := &tg.MessagesEditMessageRequest{
		Peer: peer,
		ID:   input.MessageID,
	}
	editReq.SetMessage(input.Message)

	_, err = services.API().MessagesEditMessage(tgCtx, editReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to edit message: %v", err)), nil
	}

	return mcp.NewToolResultText("Message edited successfully."), nil
}

func handlePinMessage(_ context.Context, _ mcp.CallToolRequest, input pinMessageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	_, err = services.API().MessagesUpdatePinnedMessage(tgCtx, &tg.MessagesUpdatePinnedMessageRequest{
		Peer:   peer,
		ID:     input.MessageID,
		Silent: input.Silent,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to pin message: %v", err)), nil
	}

	return mcp.NewToolResultText("Message pinned successfully."), nil
}
