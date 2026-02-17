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

type exportInviteLinkInput struct {
	Peer          string `json:"peer" jsonschema:"required"`
	ExpireDate    int    `json:"expire_date"`
	UsageLimit    int    `json:"usage_limit"`
	RequestNeeded bool   `json:"request_needed"`
	Title         string `json:"title"`
}

type getInviteLinksInput struct {
	Peer    string `json:"peer" jsonschema:"required"`
	AdminID string `json:"admin_id"`
}

type revokeInviteLinkInput struct {
	Peer string `json:"peer" jsonschema:"required"`
	Link string `json:"link" jsonschema:"required"`
}

func RegisterInviteTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_export_invite_link",
			mcp.WithDescription("Export a new invite link for a chat/channel"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("expire_date", mcp.Description("Unix timestamp when the link expires (optional)")),
			mcp.WithNumber("usage_limit", mcp.Description("Maximum number of times the link can be used (optional)")),
			mcp.WithBoolean("request_needed", mcp.Description("Whether admin approval is required to join (optional)")),
			mcp.WithString("title", mcp.Description("Title for the invite link (optional)")),
		),
		mcp.NewTypedToolHandler(handleExportInviteLink),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_invite_links",
			mcp.WithDescription("Get exported invite links for a chat/channel"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("admin_id", mcp.Description("Admin user ID or @username (defaults to self)")),
		),
		mcp.NewTypedToolHandler(handleGetInviteLinks),
	)

	s.AddTool(
		mcp.NewTool("telegram_revoke_invite_link",
			mcp.WithDescription("Revoke an invite link for a chat/channel"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("link", mcp.Required(), mcp.Description("The invite link to revoke")),
		),
		mcp.NewTypedToolHandler(handleRevokeInviteLink),
	)
}

func handleExportInviteLink(_ context.Context, _ mcp.CallToolRequest, input exportInviteLinkInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	req := &tg.MessagesExportChatInviteRequest{
		Peer: peer,
	}
	if input.ExpireDate != 0 {
		req.SetExpireDate(input.ExpireDate)
	}
	if input.UsageLimit != 0 {
		req.SetUsageLimit(input.UsageLimit)
	}
	if input.RequestNeeded {
		req.SetRequestNeeded(true)
	}
	if input.Title != "" {
		req.SetTitle(input.Title)
	}

	result, err := services.API().MessagesExportChatInvite(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to export invite link: %v", err)), nil
	}

	invite, ok := result.(*tg.ChatInviteExported)
	if !ok {
		return mcp.NewToolResultError("unexpected invite link type"), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Link: %s\n", invite.Link)
	if invite.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", invite.Title)
	}
	if expDate, ok := invite.GetExpireDate(); ok {
		t := time.Unix(int64(expDate), 0).UTC().Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "Expires: %s\n", t)
	}
	fmt.Fprintf(&b, "Usage: %d", invite.Usage)
	if invite.UsageLimit != 0 {
		fmt.Fprintf(&b, " / %d", invite.UsageLimit)
	}
	b.WriteString("\n")
	if invite.RequestNeeded {
		b.WriteString("Admin approval required: yes\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetInviteLinks(_ context.Context, _ mcp.CallToolRequest, input getInviteLinksInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	var adminUser tg.InputUserClass
	if input.AdminID == "" {
		adminUser = services.Self().AsInput()
	} else {
		adminPeer, err := services.ResolvePeer(tgCtx, input.AdminID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to resolve admin_id: %v", err)), nil
		}
		u, ok := toInputUser(adminPeer)
		if !ok {
			return mcp.NewToolResultError("admin_id does not resolve to a user"), nil
		}
		adminUser = u
	}

	result, err := services.API().MessagesGetExportedChatInvites(tgCtx, &tg.MessagesGetExportedChatInvitesRequest{
		Peer:    peer,
		AdminID: adminUser,
		Limit:   50,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get invite links: %v", err)), nil
	}

	if len(result.Invites) == 0 {
		return mcp.NewToolResultText("No invite links found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Invite links (%d):\n", len(result.Invites))

	for _, inv := range result.Invites {
		invite, ok := inv.(*tg.ChatInviteExported)
		if !ok {
			continue
		}

		b.WriteString("\n")
		fmt.Fprintf(&b, "Link: %s\n", invite.Link)
		if invite.Title != "" {
			fmt.Fprintf(&b, "Title: %s\n", invite.Title)
		}
		fmt.Fprintf(&b, "Usage: %d", invite.Usage)
		if invite.UsageLimit != 0 {
			fmt.Fprintf(&b, " / %d", invite.UsageLimit)
		}
		b.WriteString("\n")
		if expDate, ok := invite.GetExpireDate(); ok {
			t := time.Unix(int64(expDate), 0).UTC().Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "Expires: %s\n", t)
		}
		if invite.Revoked {
			b.WriteString("Status: revoked\n")
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleRevokeInviteLink(_ context.Context, _ mcp.CallToolRequest, input revokeInviteLinkInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	_, err = services.API().MessagesEditExportedChatInvite(tgCtx, &tg.MessagesEditExportedChatInviteRequest{
		Peer:    peer,
		Link:    input.Link,
		Revoked: true,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to revoke invite link: %v", err)), nil
	}

	return mcp.NewToolResultText("Invite link revoked successfully."), nil
}
