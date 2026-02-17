package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

type SendCodeInput struct {
	Code string `json:"code" validate:"required"`
}

type SendPasswordInput struct {
	Password string `json:"password" validate:"required"`
}

func RegisterAuthTools(s *server.MCPServer) {
	statusTool := mcp.NewTool("telegram_auth_status",
		mcp.WithDescription("Check current Telegram authentication status"),
	)
	s.AddTool(statusTool, handleAuthStatus)

	codeTool := mcp.NewTool("telegram_auth_send_code",
		mcp.WithDescription("Submit the verification code received via SMS or Telegram app"),
		mcp.WithString("code", mcp.Required(), mcp.Description("Verification code")),
	)
	s.AddTool(codeTool, mcp.NewTypedToolHandler(handleSendCode))

	passwordTool := mcp.NewTool("telegram_auth_send_password",
		mcp.WithDescription("Submit 2FA password if required"),
		mcp.WithString("password", mcp.Required(), mcp.Description("Two-factor authentication password")),
	)
	s.AddTool(passwordTool, mcp.NewTypedToolHandler(handleSendPassword))
}

func handleAuthStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := services.GetAuthState()
	msg := fmt.Sprintf("Auth state: %s", state)
	if state == services.AuthStateError {
		msg += fmt.Sprintf("\nError: %s", services.GetAuthError())
	}
	return mcp.NewToolResultText(msg), nil
}

func handleSendCode(_ context.Context, _ mcp.CallToolRequest, input SendCodeInput) (*mcp.CallToolResult, error) {
	newState, err := services.SubmitCode(input.Code)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("auth failed: %v", err)), nil
	}
	switch newState {
	case services.AuthStateWaitingPassword:
		return mcp.NewToolResultText("Code accepted. 2FA password required — use telegram_auth_send_password."), nil
	case services.AuthStateAuthenticated:
		return mcp.NewToolResultText("Authenticated successfully."), nil
	default:
		return mcp.NewToolResultText(fmt.Sprintf("Code submitted. State: %s", newState)), nil
	}
}

func handleSendPassword(_ context.Context, _ mcp.CallToolRequest, input SendPasswordInput) (*mcp.CallToolResult, error) {
	newState, err := services.SubmitPassword(input.Password)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("auth failed: %v", err)), nil
	}
	if newState == services.AuthStateAuthenticated {
		return mcp.NewToolResultText("Authenticated successfully."), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Password submitted. State: %s", newState)), nil
}
