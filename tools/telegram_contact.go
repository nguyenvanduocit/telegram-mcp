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

type getContactsInput struct{}

type importContactsInput struct {
	Phone     string `json:"phone" jsonschema:"required"`
	FirstName string `json:"first_name" jsonschema:"required"`
	LastName  string `json:"last_name"`
}

type blockPeerInput struct {
	Peer    string `json:"peer" jsonschema:"required"`
	Unblock bool   `json:"unblock"`
}

func RegisterContactTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_get_contacts",
			mcp.WithDescription("Get the user's contact list"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTypedToolHandler(handleGetContacts),
	)

	s.AddTool(
		mcp.NewTool("telegram_import_contacts",
			mcp.WithDescription("Import a contact by phone number"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("phone", mcp.Required(), mcp.Description("Phone number in international format")),
			mcp.WithString("first_name", mcp.Required(), mcp.Description("Contact's first name")),
			mcp.WithString("last_name", mcp.Description("Contact's last name (optional)")),
		),
		mcp.NewTypedToolHandler(handleImportContacts),
	)

	s.AddTool(
		mcp.NewTool("telegram_block_peer",
			mcp.WithDescription("Block or unblock a user"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithBoolean("unblock", mcp.Description("Set to true to unblock instead of block (default false)")),
		),
		mcp.NewTypedToolHandler(handleBlockPeer),
	)
}

func handleGetContacts(_ context.Context, _ mcp.CallToolRequest, input getContactsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	result, err := services.API().ContactsGetContacts(tgCtx, 0)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get contacts: %v", err)), nil
	}

	contacts, ok := result.(*tg.ContactsContacts)
	if !ok {
		return mcp.NewToolResultText("No contacts found."), nil
	}

	services.StorePeers(tgCtx, nil, contacts.Users)

	if len(contacts.Users) == 0 {
		return mcp.NewToolResultText("No contacts found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Contacts (%d):\n", len(contacts.Users))

	for _, u := range contacts.Users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		b.WriteString("\n")
		formatUser(&b, user)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleImportContacts(_ context.Context, _ mcp.CallToolRequest, input importContactsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	result, err := services.API().ContactsImportContacts(tgCtx, []tg.InputPhoneContact{
		{
			ClientID:  randomID(),
			Phone:     input.Phone,
			FirstName: input.FirstName,
			LastName:  input.LastName,
		},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to import contact: %v", err)), nil
	}

	services.StorePeers(tgCtx, nil, result.Users)

	var b strings.Builder
	fmt.Fprintf(&b, "Imported: %d\n", len(result.Imported))

	for _, u := range result.Users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "User ID: %d\n", user.ID)
	}

	if len(result.RetryContacts) > 0 {
		fmt.Fprintf(&b, "Retry contacts: %d\n", len(result.RetryContacts))
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleBlockPeer(_ context.Context, _ mcp.CallToolRequest, input blockPeerInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	if input.Unblock {
		_, err = services.API().ContactsUnblock(tgCtx, &tg.ContactsUnblockRequest{
			ID: peer,
		})
	} else {
		_, err = services.API().ContactsBlock(tgCtx, &tg.ContactsBlockRequest{
			ID: peer,
		})
	}
	if err != nil {
		action := "block"
		if input.Unblock {
			action = "unblock"
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to %s peer: %v", action, err)), nil
	}

	action := "blocked"
	if input.Unblock {
		action = "unblocked"
	}
	return mcp.NewToolResultText(fmt.Sprintf("Peer %s successfully.", action)), nil
}
