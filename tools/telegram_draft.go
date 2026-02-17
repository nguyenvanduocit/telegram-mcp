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

type saveDraftInput struct {
	Peer         string `json:"peer" jsonschema:"required"`
	Message      string `json:"message" jsonschema:"required"`
	ReplyToMsgID int    `json:"reply_to_msg_id"`
}

type getDraftsInput struct{}

type clearDraftInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

func RegisterDraftTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_save_draft",
			mcp.WithDescription("Save a message draft for a chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("message", mcp.Required(), mcp.Description("Draft message text")),
			mcp.WithNumber("reply_to_msg_id", mcp.Description("Message ID to reply to (optional)")),
		),
		mcp.NewTypedToolHandler(handleSaveDraft),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_drafts",
			mcp.WithDescription("Get all saved message drafts"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTypedToolHandler(handleGetDrafts),
	)

	s.AddTool(
		mcp.NewTool("telegram_clear_draft",
			mcp.WithDescription("Clear the message draft for a chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleClearDraft),
	)
}

func handleSaveDraft(_ context.Context, _ mcp.CallToolRequest, input saveDraftInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	req := &tg.MessagesSaveDraftRequest{
		Peer:    peer,
		Message: input.Message,
	}

	if input.ReplyToMsgID > 0 {
		req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: input.ReplyToMsgID})
	}

	_, err = services.API().MessagesSaveDraft(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save draft: %v", err)), nil
	}

	return mcp.NewToolResultText("Draft saved successfully."), nil
}

func handleGetDrafts(_ context.Context, _ mcp.CallToolRequest, _ getDraftsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	result, err := services.API().MessagesGetAllDrafts(tgCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get drafts: %v", err)), nil
	}

	updates, ok := result.(*tg.Updates)
	if !ok {
		return mcp.NewToolResultText("No drafts found."), nil
	}

	var b strings.Builder
	draftsFound := 0

	for _, update := range updates.Updates {
		draftUpdate, ok := update.(*tg.UpdateDraftMessage)
		if !ok {
			continue
		}

		draft, ok := draftUpdate.Draft.AsNotEmpty()
		if !ok {
			continue
		}

		draftsFound++

		var peerStr string
		switch p := draftUpdate.Peer.(type) {
		case *tg.PeerUser:
			peerStr = fmt.Sprintf("User %d", p.UserID)
		case *tg.PeerChat:
			peerStr = fmt.Sprintf("Chat %d", p.ChatID)
		case *tg.PeerChannel:
			peerStr = fmt.Sprintf("Channel %d", p.ChannelID)
		}

		fmt.Fprintf(&b, "[%s] %s\n", peerStr, draft.Message)
	}

	if draftsFound == 0 {
		return mcp.NewToolResultText("No drafts found."), nil
	}

	header := fmt.Sprintf("Drafts (%d):\n", draftsFound)
	return mcp.NewToolResultText(header + b.String()), nil
}

func handleClearDraft(_ context.Context, _ mcp.CallToolRequest, input clearDraftInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	_, err = services.API().MessagesSaveDraft(tgCtx, &tg.MessagesSaveDraftRequest{
		Peer:    peer,
		Message: "",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to clear draft: %v", err)), nil
	}

	return mcp.NewToolResultText("Draft cleared successfully."), nil
}
