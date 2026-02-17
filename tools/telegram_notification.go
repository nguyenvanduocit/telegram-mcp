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

type getNotifySettingsInput struct {
	Peer string `json:"peer" jsonschema:"required"`
}

type setNotifySettingsInput struct {
	Peer         string `json:"peer" jsonschema:"required"`
	MuteUntil    int    `json:"mute_until"`
	Silent       *bool  `json:"silent"`
	ShowPreviews *bool  `json:"show_previews"`
}

func RegisterNotificationTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_notify_settings",
			mcp.WithDescription("Get notification settings for a chat"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
		),
		mcp.NewTypedToolHandler(handleGetNotifySettings),
	)

	s.AddTool(
		mcp.NewTool("telegram_set_notify_settings",
			mcp.WithDescription("Update notification settings for a chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("mute_until", mcp.Description("Unix timestamp until muted (0 = unmute, 2147483647 = forever)")),
			mcp.WithBoolean("silent", mcp.Description("Whether to send notifications silently")),
			mcp.WithBoolean("show_previews", mcp.Description("Whether to show message previews in notifications")),
		),
		mcp.NewTypedToolHandler(handleSetNotifySettings),
	)
}

func handleGetNotifySettings(_ context.Context, _ mcp.CallToolRequest, input getNotifySettingsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	settings, err := services.API().AccountGetNotifySettings(tgCtx, &tg.InputNotifyPeer{Peer: peer})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get notify settings: %v", err)), nil
	}

	var b strings.Builder
	b.WriteString("Notification settings:\n")

	if muteUntil, ok := settings.GetMuteUntil(); ok {
		switch muteUntil {
		case 0:
			b.WriteString("Muted: no\n")
		case 2147483647:
			b.WriteString("Muted: forever\n")
		default:
			t := time.Unix(int64(muteUntil), 0).UTC().Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "Muted until: %s\n", t)
		}
	}

	if showPreviews, ok := settings.GetShowPreviews(); ok {
		fmt.Fprintf(&b, "Show previews: %v\n", showPreviews)
	}

	if silent, ok := settings.GetSilent(); ok {
		fmt.Fprintf(&b, "Silent: %v\n", silent)
	}

	if sound, ok := settings.GetOtherSound(); ok {
		fmt.Fprintf(&b, "Sound: %T\n", sound)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleSetNotifySettings(_ context.Context, _ mcp.CallToolRequest, input setNotifySettingsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	var settings tg.InputPeerNotifySettings
	settings.SetMuteUntil(input.MuteUntil)
	if input.Silent != nil {
		settings.SetSilent(*input.Silent)
	}
	if input.ShowPreviews != nil {
		settings.SetShowPreviews(*input.ShowPreviews)
	}

	_, err = services.API().AccountUpdateNotifySettings(tgCtx, &tg.AccountUpdateNotifySettingsRequest{
		Peer:     &tg.InputNotifyPeer{Peer: peer},
		Settings: settings,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update notify settings: %v", err)), nil
	}

	return mcp.NewToolResultText("Notification settings updated successfully."), nil
}
