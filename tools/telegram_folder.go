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

type getFoldersInput struct{}

type updateFolderInput struct {
	ID           int    `json:"id" jsonschema:"required"`
	Title        string `json:"title" jsonschema:"required"`
	IncludePeers string `json:"include_peers"`
	ExcludePeers string `json:"exclude_peers"`
	PinnedPeers  string `json:"pinned_peers"`
}

type deleteFolderInput struct {
	ID int `json:"id" jsonschema:"required"`
}

func RegisterFolderTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_folders",
			mcp.WithDescription("Get all dialog folders/filters"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTypedToolHandler(handleGetFolders),
	)

	s.AddTool(
		mcp.NewTool("telegram_update_folder",
			mcp.WithDescription("Create or update a dialog folder/filter"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Folder ID")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Folder name (max 12 UTF-8 chars)")),
			mcp.WithString("include_peers", mcp.Description("Comma-separated peer identifiers (IDs or @usernames) to include")),
			mcp.WithString("exclude_peers", mcp.Description("Comma-separated peer identifiers (IDs or @usernames) to exclude")),
			mcp.WithString("pinned_peers", mcp.Description("Comma-separated peer identifiers (IDs or @usernames) to pin")),
		),
		mcp.NewTypedToolHandler(handleUpdateFolder),
	)

	s.AddTool(
		mcp.NewTool("telegram_delete_folder",
			mcp.WithDescription("Delete a dialog folder/filter"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Folder ID to delete")),
		),
		mcp.NewTypedToolHandler(handleDeleteFolder),
	)
}

func handleGetFolders(_ context.Context, _ mcp.CallToolRequest, _ getFoldersInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	result, err := services.API().MessagesGetDialogFilters(tgCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get folders: %v", err)), nil
	}

	if len(result.Filters) == 0 {
		return mcp.NewToolResultText("No folders found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Folders (%d):\n", len(result.Filters))

	for _, fc := range result.Filters {
		switch f := fc.(type) {
		case *tg.DialogFilter:
			fmt.Fprintf(&b, "\n[Folder] ID: %d, Title: %s\n", f.ID, f.Title.Text)
			fmt.Fprintf(&b, "  Pinned peers: %d, Included peers: %d, Excluded peers: %d\n",
				len(f.PinnedPeers), len(f.IncludePeers), len(f.ExcludePeers))
		case *tg.DialogFilterChatlist:
			fmt.Fprintf(&b, "\n[Chatlist] ID: %d, Title: %s\n", f.ID, f.Title.Text)
			fmt.Fprintf(&b, "  Pinned peers: %d, Included peers: %d\n",
				len(f.PinnedPeers), len(f.IncludePeers))
		case *tg.DialogFilterDefault:
			b.WriteString("\n[Default] All Chats\n")
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleUpdateFolder(_ context.Context, _ mcp.CallToolRequest, input updateFolderInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	includePeers, err := resolvePeerList(tgCtx, input.IncludePeers)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve include_peers: %v", err)), nil
	}

	excludePeers, err := resolvePeerList(tgCtx, input.ExcludePeers)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve exclude_peers: %v", err)), nil
	}

	pinnedPeers, err := resolvePeerList(tgCtx, input.PinnedPeers)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve pinned_peers: %v", err)), nil
	}

	filter := &tg.DialogFilter{
		ID:           input.ID,
		Title:        tg.TextWithEntities{Text: input.Title},
		IncludePeers: includePeers,
		ExcludePeers: excludePeers,
		PinnedPeers:  pinnedPeers,
	}

	req := &tg.MessagesUpdateDialogFilterRequest{ID: input.ID}
	req.SetFilter(filter)

	_, err = services.API().MessagesUpdateDialogFilter(tgCtx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update folder: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Folder %q (ID: %d) updated successfully.", input.Title, input.ID)), nil
}

func handleDeleteFolder(_ context.Context, _ mcp.CallToolRequest, input deleteFolderInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	_, err := services.API().MessagesUpdateDialogFilter(tgCtx, &tg.MessagesUpdateDialogFilterRequest{ID: input.ID})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete folder: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Folder ID %d deleted successfully.", input.ID)), nil
}

func resolvePeerList(ctx context.Context, commaSeparated string) ([]tg.InputPeerClass, error) {
	if commaSeparated == "" {
		return nil, nil
	}

	parts := strings.Split(commaSeparated, ",")
	peers := make([]tg.InputPeerClass, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		peer, err := services.ResolvePeer(ctx, part)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", part, err)
		}
		peers = append(peers, peer)
	}

	return peers, nil
}
