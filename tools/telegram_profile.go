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

type updateProfileInput struct {
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name"`
	About     *string `json:"about"`
}

type getReadParticipantsInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
}

func RegisterProfileTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_update_profile",
			mcp.WithDescription("Update the current user's profile (first name, last name, bio)"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("first_name", mcp.Description("New first name")),
			mcp.WithString("last_name", mcp.Description("New last name")),
			mcp.WithString("about", mcp.Description("New bio/about text")),
		),
		mcp.NewTypedToolHandler(handleUpdateProfile),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_read_participants",
			mcp.WithDescription("Get which users read a specific message (small groups only)"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to check")),
		),
		mcp.NewTypedToolHandler(handleGetReadParticipants),
	)
}

func handleUpdateProfile(_ context.Context, _ mcp.CallToolRequest, input updateProfileInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	if input.FirstName == "" && input.LastName == nil && input.About == nil {
		return mcp.NewToolResultError("at least one of first_name, last_name, or about must be provided"), nil
	}

	req := &tg.AccountUpdateProfileRequest{}
	if input.FirstName != "" {
		req.SetFirstName(input.FirstName)
	}
	if input.LastName != nil {
		req.SetLastName(*input.LastName)
	}
	if input.About != nil {
		req.SetAbout(*input.About)
	}

	result, err := services.API().AccountUpdateProfile(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update profile: %v", err)), nil
	}

	user, ok := result.(*tg.User)
	if !ok {
		return mcp.NewToolResultText("Profile updated successfully."), nil
	}

	var b strings.Builder
	b.WriteString("Profile updated successfully.\n")
	fmt.Fprintf(&b, "Name: %s", user.FirstName)
	if user.LastName != "" {
		fmt.Fprintf(&b, " %s", user.LastName)
	}
	b.WriteString("\n")
	if user.Username != "" {
		fmt.Fprintf(&b, "Username: @%s\n", user.Username)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetReadParticipants(_ context.Context, _ mcp.CallToolRequest, input getReadParticipantsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	participants, err := services.API().MessagesGetMessageReadParticipants(tgCtx, &tg.MessagesGetMessageReadParticipantsRequest{
		Peer:  peer,
		MsgID: input.MessageID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get read participants: %v", err)), nil
	}

	if len(participants) == 0 {
		return mcp.NewToolResultText("No read participants found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Read participants for message %d (%d):\n", input.MessageID, len(participants))

	for _, p := range participants {
		readTime := time.Unix(int64(p.Date), 0).UTC().Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "  User ID: %d, Read at: %s\n", p.UserID, readTime)
	}

	return mcp.NewToolResultText(b.String()), nil
}
