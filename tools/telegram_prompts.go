package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterPrompts(s *server.MCPServer) {
	// daily_digest — no arguments
	dailyDigest := mcp.NewPrompt("daily_digest",
		mcp.WithPromptDescription("Guide for creating a daily digest of unread Telegram messages"),
	)
	s.AddPrompt(dailyDigest, handleDailyDigest)

	// community_manager — requires peer argument
	communityManager := mcp.NewPrompt("community_manager",
		mcp.WithPromptDescription("Guide for managing a Telegram community/group"),
		mcp.WithArgument("peer",
			mcp.RequiredArgument(),
			mcp.ArgumentDescription("Chat ID or @username of the community to manage"),
		),
	)
	s.AddPrompt(communityManager, handleCommunityManager)

	// content_broadcaster — requires source_peer and destinations arguments
	contentBroadcaster := mcp.NewPrompt("content_broadcaster",
		mcp.WithPromptDescription("Guide for cross-posting content to multiple Telegram channels"),
		mcp.WithArgument("source_peer",
			mcp.RequiredArgument(),
			mcp.ArgumentDescription("Source chat to get content from"),
		),
		mcp.WithArgument("destinations",
			mcp.RequiredArgument(),
			mcp.ArgumentDescription("Comma-separated destination chat IDs or @usernames"),
		),
	)
	s.AddPrompt(contentBroadcaster, handleContentBroadcaster)
}

func handleDailyDigest(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "Guide for creating a daily digest of unread Telegram messages",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: `Create a daily digest of my unread Telegram messages:

1. Call telegram_get_unread to fetch all unread conversations
2. Group the results by priority:
   - URGENT: Direct messages (DMs) with multiple unread messages
   - IMPORTANT: Group chats where I was mentioned or replied to
   - NORMAL: Channel updates and other group messages
3. For each conversation, provide:
   - Chat name and unread count
   - Key topics or questions that need my attention
   - Any action items or decisions needed
4. End with a summary: total unread count, conversations needing response`,
				},
			},
		},
	}, nil
}

func handleCommunityManager(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	peer := request.Params.Arguments["peer"]
	return &mcp.GetPromptResult{
		Description: "Guide for managing a Telegram community/group",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`Help me manage the Telegram community %s:

1. Call telegram_chat_context with peer="%s" to get the full community snapshot
2. Analyze recent messages and identify:
   - Unanswered questions from members
   - Spam or off-topic messages that should be removed
   - Active discussions that might need moderation
   - New members who should be welcomed
3. Suggest specific actions:
   - Draft responses to unanswered questions
   - List messages to delete (with message IDs)
   - Identify members to warn or ban
4. Provide engagement insights:
   - Most active members
   - Peak activity times
   - Trending topics`, peer, peer),
				},
			},
		},
	}, nil
}

func handleContentBroadcaster(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	sourcePeer := request.Params.Arguments["source_peer"]
	destinations := request.Params.Arguments["destinations"]
	return &mcp.GetPromptResult{
		Description: "Guide for cross-posting content to multiple Telegram channels",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`Help me broadcast content from %s to these destinations: %s

1. Call telegram_get_history with peer="%s" to see recent content
2. Identify the most relevant/recent content to share
3. For each piece of content, ask me to confirm before forwarding
4. Use telegram_forward_bulk with:
   - from_peer="%s"
   - to_peers="%s"
   - message_ids=[confirmed message IDs]
5. Report the delivery status for each destination`, sourcePeer, destinations, sourcePeer, sourcePeer, destinations),
				},
			},
		},
	}, nil
}
