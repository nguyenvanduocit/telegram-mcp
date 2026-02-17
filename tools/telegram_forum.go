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

type createForumTopicInput struct {
	Peer        string `json:"peer" jsonschema:"required"`
	Title       string `json:"title" jsonschema:"required"`
	IconEmojiID int64  `json:"icon_emoji_id"`
}

type getForumTopicsInput struct {
	Peer  string `json:"peer" jsonschema:"required"`
	Limit int    `json:"limit"`
}

type editForumTopicInput struct {
	Peer    string `json:"peer" jsonschema:"required"`
	TopicID int    `json:"topic_id" jsonschema:"required"`
	Title   string `json:"title"`
	Closed  *bool  `json:"closed"`
}

func RegisterForumTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_create_forum_topic",
			mcp.WithDescription("Create a new forum topic in a supergroup with forum enabled"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of a supergroup with forum enabled")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Title of the forum topic")),
			mcp.WithNumber("icon_emoji_id", mcp.Description("Custom emoji ID for the topic icon (optional)")),
		),
		mcp.NewTypedToolHandler(handleCreateForumTopic),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_forum_topics",
			mcp.WithDescription("List forum topics in a supergroup"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of a supergroup with forum enabled")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of topics to retrieve (default 20)")),
		),
		mcp.NewTypedToolHandler(handleGetForumTopics),
	)

	s.AddTool(
		mcp.NewTool("telegram_edit_forum_topic",
			mcp.WithDescription("Edit a forum topic title or open/close state"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of a supergroup with forum enabled")),
			mcp.WithNumber("topic_id", mcp.Required(), mcp.Description("ID of the forum topic to edit")),
			mcp.WithString("title", mcp.Description("New title for the topic (optional)")),
			mcp.WithBoolean("closed", mcp.Description("Set to true to close, false to reopen (optional)")),
		),
		mcp.NewTypedToolHandler(handleEditForumTopic),
	)
}

func handleCreateForumTopic(_ context.Context, _ mcp.CallToolRequest, input createForumTopicInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	if _, ok := peer.(*tg.InputPeerChannel); !ok {
		return mcp.NewToolResultError("peer must be a supergroup/channel with forum enabled"), nil
	}

	req := &tg.MessagesCreateForumTopicRequest{
		Peer:     peer,
		Title:    input.Title,
		RandomID: randomID(),
	}

	if input.IconEmojiID != 0 {
		req.SetIconEmojiID(input.IconEmojiID)
	}

	result, err := services.API().MessagesCreateForumTopic(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create forum topic: %v", err)), nil
	}

	// Extract topic ID from the updates response
	var topicID int
	switch u := result.(type) {
	case *tg.Updates:
		for _, update := range u.Updates {
			if mt, ok := update.(*tg.UpdateMessageID); ok {
				topicID = mt.ID
				break
			}
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("Forum topic %q created successfully (topic ID: %d).", input.Title, topicID)), nil
}

func handleGetForumTopics(_ context.Context, _ mcp.CallToolRequest, input getForumTopicsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	if _, ok := peer.(*tg.InputPeerChannel); !ok {
		return mcp.NewToolResultError("peer must be a supergroup/channel with forum enabled"), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().MessagesGetForumTopics(tgCtx, &tg.MessagesGetForumTopicsRequest{
		Peer:  peer,
		Limit: limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get forum topics: %v", err)), nil
	}

	services.StorePeers(tgCtx, result.Chats, result.Users)

	if len(result.Topics) == 0 {
		return mcp.NewToolResultText("No forum topics found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Forum topics (%d):\n", len(result.Topics))

	for _, tc := range result.Topics {
		topic, ok := tc.(*tg.ForumTopic)
		if !ok {
			continue
		}

		date := time.Unix(int64(topic.Date), 0).UTC().Format("2006-01-02 15:04:05")
		status := "open"
		if topic.Closed {
			status = "closed"
		}

		fmt.Fprintf(&b, "\n[%d] %s (%s, %s)", topic.ID, topic.Title, status, date)
		if topic.UnreadCount > 0 {
			fmt.Fprintf(&b, " [%d unread]", topic.UnreadCount)
		}
		fmt.Fprintf(&b, " [top_message: %d]", topic.TopMessage)
		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleEditForumTopic(_ context.Context, _ mcp.CallToolRequest, input editForumTopicInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	if _, ok := peer.(*tg.InputPeerChannel); !ok {
		return mcp.NewToolResultError("peer must be a supergroup/channel with forum enabled"), nil
	}

	req := &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: input.TopicID,
	}

	if input.Title != "" {
		req.SetTitle(input.Title)
	}

	if input.Closed != nil {
		req.SetClosed(*input.Closed)
	}

	_, err = services.API().MessagesEditForumTopic(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to edit forum topic: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Forum topic %d edited successfully.", input.TopicID)), nil
}
