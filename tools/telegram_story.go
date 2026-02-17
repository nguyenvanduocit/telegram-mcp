package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

type getPeerStoriesInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

type getAllStoriesInput struct {
	Next  bool   `json:"next"`
	State string `json:"state"`
}

type sendStoryInput struct {
	Peer     string `json:"peer" jsonschema:"required"`
	FilePath string `json:"file_path" jsonschema:"required"`
	Caption  string `json:"caption"`
	Pin      bool   `json:"pin"`
}

type deleteStoriesInput struct {
	Peer     string `json:"peer" jsonschema:"required"`
	StoryIDs string `json:"story_ids" jsonschema:"required"`
}

func RegisterStoryTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_peer_stories",
			mcp.WithDescription("Get active stories of a specific peer"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleGetPeerStories),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_all_stories",
			mcp.WithDescription("Get all active stories from all peers"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithBoolean("next", mcp.Description("Set to true to paginate to next results")),
			mcp.WithString("state", mcp.Description("Pagination state token from previous response")),
		),
		mcp.NewTypedToolHandler(handleGetAllStories),
	)

	s.AddTool(
		mcp.NewTool("telegram_send_story",
			mcp.WithDescription("Send a story to a peer (photo or video)"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the photo or video file")),
			mcp.WithString("caption", mcp.Description("Story caption text")),
			mcp.WithBoolean("pin", mcp.Description("Pin story to profile on expiration")),
		),
		mcp.NewTypedToolHandler(handleSendStory),
	)

	s.AddTool(
		mcp.NewTool("telegram_delete_stories",
			mcp.WithDescription("Delete stories from a peer"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("story_ids", mcp.Required(), mcp.Description("Comma-separated story IDs to delete")),
		),
		mcp.NewTypedToolHandler(handleDeleteStories),
	)
}

func handleGetPeerStories(_ context.Context, _ mcp.CallToolRequest, input getPeerStoriesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	result, err := services.API().StoriesGetPeerStories(tgCtx, peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get peer stories: %v", err)), nil
	}

	services.StorePeers(tgCtx, result.Chats, result.Users)

	var b strings.Builder
	stories := result.Stories.Stories
	if len(stories) == 0 {
		return mcp.NewToolResultText("No active stories found."), nil
	}

	fmt.Fprintf(&b, "Stories (%d):\n", len(stories))
	for _, sc := range stories {
		story, ok := sc.(*tg.StoryItem)
		if !ok {
			continue
		}
		formatStoryItem(&b, story)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetAllStories(_ context.Context, _ mcp.CallToolRequest, input getAllStoriesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	req := &tg.StoriesGetAllStoriesRequest{
		Next: input.Next,
	}
	if input.State != "" {
		req.SetState(input.State)
	}
	result, err := services.API().StoriesGetAllStories(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get all stories: %v", err)), nil
	}

	allStories, ok := result.(*tg.StoriesAllStories)
	if !ok {
		return mcp.NewToolResultText("No stories available."), nil
	}

	services.StorePeers(tgCtx, allStories.Chats, allStories.Users)

	if len(allStories.PeerStories) == 0 {
		return mcp.NewToolResultText("No active stories found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "All Stories (peers: %d, total count: %d):\n", len(allStories.PeerStories), allStories.Count)

	for _, ps := range allStories.PeerStories {
		peerID := formatPeerID(ps.Peer)
		storyIDs := make([]string, 0, len(ps.Stories))
		for _, sc := range ps.Stories {
			storyIDs = append(storyIDs, fmt.Sprintf("%d", sc.GetID()))
		}
		fmt.Fprintf(&b, "\nPeer %s: stories [%s]\n", peerID, strings.Join(storyIDs, ", "))
	}

	if allStories.State != "" {
		fmt.Fprintf(&b, "\nPagination state: %s\n", allStories.State)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleSendStory(_ context.Context, _ mcp.CallToolRequest, input sendStoryInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	cleanPath := filepath.Clean(input.FilePath)
	if !filepath.IsAbs(cleanPath) {
		return mcp.NewToolResultError("file_path must be an absolute path"), nil
	}
	if _, err := os.Stat(cleanPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("file not found: %v", err)), nil
	}

	u := uploader.NewUploader(services.API())
	uploaded, err := u.FromPath(tgCtx, cleanPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to upload file: %v", err)), nil
	}

	var media tg.InputMediaClass
	ext := strings.ToLower(filepath.Ext(cleanPath))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		media = &tg.InputMediaUploadedPhoto{File: uploaded}
	default:
		media = &tg.InputMediaUploadedDocument{
			File:     uploaded,
			MimeType: mimeFromPath(cleanPath),
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{},
			},
		}
	}

	req := &tg.StoriesSendStoryRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
		Pinned:   input.Pin,
		PrivacyRules: []tg.InputPrivacyRuleClass{
			&tg.InputPrivacyValueAllowAll{},
		},
	}
	if input.Caption != "" {
		req.SetCaption(input.Caption)
	}

	_, err = services.API().StoriesSendStory(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send story: %v", err)), nil
	}

	return mcp.NewToolResultText("Story sent successfully."), nil
}

func handleDeleteStories(_ context.Context, _ mcp.CallToolRequest, input deleteStoriesInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	ids, err := parseMessageIDs(input.StoryIDs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid story_ids: %v", err)), nil
	}

	_, err = services.API().StoriesDeleteStories(tgCtx, &tg.StoriesDeleteStoriesRequest{
		Peer: peer,
		ID:   ids,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete stories: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Deleted %d story(ies) successfully.", len(ids))), nil
}

func formatStoryItem(b *strings.Builder, story *tg.StoryItem) {
	date := time.Unix(int64(story.Date), 0).UTC().Format("2006-01-02 15:04:05")
	expire := time.Unix(int64(story.ExpireDate), 0).UTC().Format("2006-01-02 15:04:05")

	fmt.Fprintf(b, "\n[Story %d] posted: %s, expires: %s", story.ID, date, expire)

	if views, ok := story.GetViews(); ok {
		fmt.Fprintf(b, ", views: %d", views.ViewsCount)
	}

	if caption, ok := story.GetCaption(); ok && caption != "" {
		fmt.Fprintf(b, "\n  Caption: %s", caption)
	}

	if story.Pinned {
		b.WriteString(" [pinned]")
	}
	b.WriteString("\n")
}

func formatPeerID(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("user:%d", p.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("chat:%d", p.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("channel:%d", p.ChannelID)
	default:
		return "unknown"
	}
}
