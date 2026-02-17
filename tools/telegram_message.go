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
	ScheduleDate int    `json:"schedule_date"`
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

// Search Global

type searchGlobalInput struct {
	Query string `json:"query" jsonschema:"required"`
	Limit int    `json:"limit"`
}

// Read History

type readHistoryInput struct {
	Peer  string `json:"peer" jsonschema:"required"`
	MaxID int    `json:"max_id"`
}

// Set Typing

type setTypingInput struct {
	Peer   string `json:"peer" jsonschema:"required"`
	Action string `json:"action"`
}

// Unpin All Messages

type unpinAllMessagesInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

// Delete History

type deleteHistoryInput struct {
	Peer   string `json:"peer" jsonschema:"required"`
	MaxID  int    `json:"max_id"`
	Revoke *bool  `json:"revoke"`
}

// Translate

type translateInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
	ToLang    string `json:"to_lang" jsonschema:"required"`
}

// Send Poll

type sendPollInput struct {
	Peer           string `json:"peer" jsonschema:"required"`
	Question       string `json:"question" jsonschema:"required"`
	Options        string `json:"options" jsonschema:"required"`
	MultipleChoice bool   `json:"multiple_choice"`
	Quiz           bool   `json:"quiz"`
	CorrectOption  int    `json:"correct_option"`
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
			mcp.WithNumber("schedule_date", mcp.Description("Unix timestamp to schedule message for future delivery")),
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

	s.AddTool(
		mcp.NewTool("telegram_search_global",
			mcp.WithDescription("Search messages across all chats globally"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 20)")),
		),
		mcp.NewTypedToolHandler(handleSearchGlobal),
	)

	s.AddTool(
		mcp.NewTool("telegram_read_history",
			mcp.WithDescription("Mark messages as read in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("max_id", mcp.Description("Mark messages up to this ID as read (default 0 = read all)")),
		),
		mcp.NewTypedToolHandler(handleReadHistory),
	)

	s.AddTool(
		mcp.NewTool("telegram_set_typing",
			mcp.WithDescription("Set typing status in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("action", mcp.Description("Typing action: typing, cancel, record_video, upload_video, record_audio, upload_audio, upload_document, choose_sticker, game (default: typing)")),
		),
		mcp.NewTypedToolHandler(handleSetTyping),
	)

	s.AddTool(
		mcp.NewTool("telegram_unpin_all_messages",
			mcp.WithDescription("Unpin all pinned messages in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleUnpinAllMessages),
	)

	s.AddTool(
		mcp.NewTool("telegram_delete_history",
			mcp.WithDescription("Delete chat history in a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("max_id", mcp.Description("Delete messages up to this ID (default 0 = delete all)")),
			mcp.WithBoolean("revoke", mcp.Description("Delete for everyone (default true)")),
		),
		mcp.NewTypedToolHandler(handleDeleteHistory),
	)

	s.AddTool(
		mcp.NewTool("telegram_translate",
			mcp.WithDescription("Translate a message to a specified language"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to translate")),
			mcp.WithString("to_lang", mcp.Required(), mcp.Description("Two-letter ISO 639-1 language code (e.g. \"en\", \"vi\", \"ja\")")),
		),
		mcp.NewTypedToolHandler(handleTranslate),
	)

	s.AddTool(
		mcp.NewTool("telegram_send_poll",
			mcp.WithDescription("Send a poll to a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("question", mcp.Required(), mcp.Description("Poll question text")),
			mcp.WithString("options", mcp.Required(), mcp.Description("Comma-separated poll options")),
			mcp.WithBoolean("multiple_choice", mcp.Description("Allow multiple answers")),
			mcp.WithBoolean("quiz", mcp.Description("Quiz mode with correct answer")),
			mcp.WithNumber("correct_option", mcp.Description("0-indexed correct option for quiz mode")),
		),
		mcp.NewTypedToolHandler(handleSendPoll),
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

	if input.ScheduleDate > 0 {
		req.SetScheduleDate(input.ScheduleDate)
	}

	_, err = services.API().MessagesSendMessage(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	if input.ScheduleDate > 0 {
		return mcp.NewToolResultText("Message scheduled successfully."), nil
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

func handleSearchGlobal(_ context.Context, _ mcp.CallToolRequest, input searchGlobalInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().MessagesSearchGlobal(tgCtx, &tg.MessagesSearchGlobalRequest{
		Q:          input.Query,
		Filter:     &tg.InputMessagesFilterEmpty{},
		Limit:      limit,
		OffsetPeer: &tg.InputPeerEmpty{},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search globally: %v", err)), nil
	}

	msgs := extractMessages(tgCtx, result)
	return mcp.NewToolResultText(formatMessages(msgs)), nil
}

func handleReadHistory(_ context.Context, _ mcp.CallToolRequest, input readHistoryInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	maxID := input.MaxID

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = services.API().ChannelsReadHistory(tgCtx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			MaxID:   maxID,
		})
	default:
		_, err = services.API().MessagesReadHistory(tgCtx, &tg.MessagesReadHistoryRequest{
			Peer:  peer,
			MaxID: maxID,
		})
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read history: %v", err)), nil
	}

	return mcp.NewToolResultText("History marked as read."), nil
}

func handleSetTyping(_ context.Context, _ mcp.CallToolRequest, input setTypingInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	var action tg.SendMessageActionClass
	switch input.Action {
	case "cancel":
		action = &tg.SendMessageCancelAction{}
	case "record_video":
		action = &tg.SendMessageRecordVideoAction{}
	case "upload_video":
		action = &tg.SendMessageUploadVideoAction{}
	case "record_audio":
		action = &tg.SendMessageRecordRoundAction{}
	case "upload_audio":
		action = &tg.SendMessageUploadAudioAction{}
	case "upload_document":
		action = &tg.SendMessageUploadDocumentAction{}
	case "choose_sticker":
		action = &tg.SendMessageChooseStickerAction{}
	case "game":
		action = &tg.SendMessageGamePlayAction{}
	default:
		action = &tg.SendMessageTypingAction{}
	}

	_, err = services.API().MessagesSetTyping(tgCtx, &tg.MessagesSetTypingRequest{
		Peer:   peer,
		Action: action,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set typing: %v", err)), nil
	}

	return mcp.NewToolResultText("Typing status set."), nil
}

func handleUnpinAllMessages(_ context.Context, _ mcp.CallToolRequest, input unpinAllMessagesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	_, err = services.API().MessagesUnpinAllMessages(tgCtx, &tg.MessagesUnpinAllMessagesRequest{
		Peer: peer,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to unpin all messages: %v", err)), nil
	}

	return mcp.NewToolResultText("All messages unpinned successfully."), nil
}

func handleDeleteHistory(_ context.Context, _ mcp.CallToolRequest, input deleteHistoryInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	maxID := input.MaxID

	revoke := true
	if input.Revoke != nil {
		revoke = *input.Revoke
	}

	_, err = services.API().MessagesDeleteHistory(tgCtx, &tg.MessagesDeleteHistoryRequest{
		Peer:   peer,
		MaxID:  maxID,
		Revoke: revoke,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete history: %v", err)), nil
	}

	return mcp.NewToolResultText("Chat history deleted successfully."), nil
}

func handleTranslate(_ context.Context, _ mcp.CallToolRequest, input translateInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	req := &tg.MessagesTranslateTextRequest{
		ToLang: input.ToLang,
	}
	req.SetPeer(peer)
	req.SetID([]int{input.MessageID})

	result, err := services.API().MessagesTranslateText(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to translate message: %v", err)), nil
	}

	var sb strings.Builder
	for _, r := range result.Result {
		sb.WriteString(r.Text)
		sb.WriteString("\n")
	}

	text := strings.TrimSpace(sb.String())
	if text == "" {
		return mcp.NewToolResultText("No translation available."), nil
	}
	return mcp.NewToolResultText(text), nil
}

func handleSendPoll(_ context.Context, _ mcp.CallToolRequest, input sendPollInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	optionParts := strings.Split(input.Options, ",")
	if len(optionParts) < 2 {
		return mcp.NewToolResultError("poll requires at least 2 options"), nil
	}

	answers := make([]tg.PollAnswer, len(optionParts))
	for i, opt := range optionParts {
		answers[i] = tg.PollAnswer{
			Text:   tg.TextWithEntities{Text: strings.TrimSpace(opt)},
			Option: []byte{byte(i)},
		}
	}

	poll := tg.Poll{
		ID:             randomID(),
		Question:       tg.TextWithEntities{Text: input.Question},
		Answers:        answers,
		MultipleChoice: input.MultipleChoice,
		Quiz:           input.Quiz,
	}

	media := &tg.InputMediaPoll{
		Poll: poll,
	}

	if input.Quiz {
		media.SetCorrectAnswers([][]byte{{byte(input.CorrectOption)}})
	}

	_, err = services.API().MessagesSendMedia(tgCtx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send poll: %v", err)), nil
	}

	return mcp.NewToolResultText("Poll sent successfully."), nil
}
