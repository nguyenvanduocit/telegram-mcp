package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

type sendReactionInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
	Reaction  string `json:"reaction" jsonschema:"required"`
}

type getMessageReactionsInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
}

func RegisterReactionTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_send_reaction",
			mcp.WithDescription("Send a reaction to a message. Use an emoji like '👍' or a custom emoji document ID. Send empty string to remove reaction."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to react to")),
			mcp.WithString("reaction", mcp.Required(), mcp.Description("Emoji like '👍' or custom emoji document ID. Empty string to remove reaction.")),
		),
		mcp.NewTypedToolHandler(handleSendReaction),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_message_reactions",
			mcp.WithDescription("Get reactions on a message, showing emoji and count"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to get reactions for")),
		),
		mcp.NewTypedToolHandler(handleGetMessageReactions),
	)
}

func handleSendReaction(_ context.Context, _ mcp.CallToolRequest, input sendReactionInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	req := &tg.MessagesSendReactionRequest{
		Peer:  peer,
		MsgID: input.MessageID,
	}

	if input.Reaction != "" {
		var reaction tg.ReactionClass

		// If the reaction is a numeric string, treat it as a custom emoji document ID
		if docID, parseErr := strconv.ParseInt(input.Reaction, 10, 64); parseErr == nil {
			reaction = &tg.ReactionCustomEmoji{DocumentID: docID}
		} else {
			reaction = &tg.ReactionEmoji{Emoticon: input.Reaction}
		}

		req.SetReaction([]tg.ReactionClass{reaction})
	} else {
		req.SetReaction(nil)
	}

	_, err = services.API().MessagesSendReaction(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send reaction: %v", err)), nil
	}

	if input.Reaction == "" {
		return mcp.NewToolResultText("Reaction removed successfully."), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Reaction %s sent successfully.", input.Reaction)), nil
}

func handleGetMessageReactions(_ context.Context, _ mcp.CallToolRequest, input getMessageReactionsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	msg, err := getMessageByID(tgCtx, peer, input.MessageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get message: %v", err)), nil
	}

	reactions, ok := msg.GetReactions()
	if !ok || len(reactions.Results) == 0 {
		return mcp.NewToolResultText("No reactions on this message."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Reactions for message %d:\n", input.MessageID)
	for _, rc := range reactions.Results {
		switch r := rc.Reaction.(type) {
		case *tg.ReactionEmoji:
			fmt.Fprintf(&sb, "  %s: %d\n", r.Emoticon, rc.Count)
		case *tg.ReactionCustomEmoji:
			fmt.Fprintf(&sb, "  [custom:%d]: %d\n", r.DocumentID, rc.Count)
		case *tg.ReactionPaid:
			fmt.Fprintf(&sb, "  [paid]: %d\n", rc.Count)
		default:
			fmt.Fprintf(&sb, "  [unknown]: %d\n", rc.Count)
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}
