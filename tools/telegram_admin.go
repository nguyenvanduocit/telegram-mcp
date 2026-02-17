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

type editAdminInput struct {
	Peer        string `json:"peer" jsonschema:"required"`
	UserID      string `json:"user_id" jsonschema:"required"`
	AdminRights string `json:"admin_rights" jsonschema:"required"`
	Rank        string `json:"rank"`
}

type editBannedInput struct {
	Peer         string `json:"peer" jsonschema:"required"`
	UserID       string `json:"user_id" jsonschema:"required"`
	BannedRights string `json:"banned_rights"`
	UntilDate    int    `json:"until_date"`
}

type getParticipantsInput struct {
	Peer   string `json:"peer" jsonschema:"required"`
	Filter string `json:"filter"`
	Limit  int    `json:"limit"`
	Query  string `json:"query"`
}

type getAdminLogInput struct {
	Peer  string `json:"peer" jsonschema:"required"`
	Limit int    `json:"limit"`
	Query string `json:"query"`
}

func RegisterAdminTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_edit_admin",
			mcp.WithDescription("Edit admin rights for a user in a channel/supergroup"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of the channel/supergroup")),
			mcp.WithString("user_id", mcp.Required(), mcp.Description("User ID or @username of the user to promote")),
			mcp.WithString("admin_rights", mcp.Required(), mcp.Description("Comma-separated admin rights: change_info,post_messages,edit_messages,delete_messages,ban_users,invite_users,pin_messages,manage_call,add_admins,anonymous,manage_topics,post_stories,edit_stories,delete_stories")),
			mcp.WithString("rank", mcp.Description("Custom admin title/rank (optional)")),
		),
		mcp.NewTypedToolHandler(handleEditAdmin),
	)

	s.AddTool(
		mcp.NewTool("telegram_edit_banned",
			mcp.WithDescription("Edit banned rights for a user in a channel/supergroup"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of the channel/supergroup")),
			mcp.WithString("user_id", mcp.Required(), mcp.Description("User ID or @username of the user to ban/restrict")),
			mcp.WithString("banned_rights", mcp.Description("Comma-separated banned rights: view_messages,send_messages,send_media,send_stickers,send_gifs,send_games,send_inline,embed_links,send_polls,change_info,invite_users,pin_messages,manage_topics,send_photos,send_videos,send_roundvideos,send_audios,send_voices,send_docs,send_plain")),
			mcp.WithNumber("until_date", mcp.Description("Ban expiry as unix timestamp (0 = forever, default 0)")),
		),
		mcp.NewTypedToolHandler(handleEditBanned),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_participants",
			mcp.WithDescription("Get participants list of a channel/supergroup"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of the channel/supergroup")),
			mcp.WithString("filter", mcp.Description("Filter type: recent, admins, kicked, banned, bots, search (default: recent)")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of participants to return (default 20)")),
			mcp.WithString("query", mcp.Description("Search query for kicked, banned, and search filters")),
		),
		mcp.NewTypedToolHandler(handleGetParticipants),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_admin_log",
			mcp.WithDescription("Get admin/action log of a channel/supergroup"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username of the channel/supergroup")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of log entries to return (default 20)")),
			mcp.WithString("query", mcp.Description("Search query to filter log events")),
		),
		mcp.NewTypedToolHandler(handleGetAdminLog),
	)
}

func toInputChannel(peer tg.InputPeerClass) (*tg.InputChannel, bool) {
	ch, ok := peer.(*tg.InputPeerChannel)
	if !ok {
		return nil, false
	}
	return &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash}, true
}

func parseAdminRights(s string) tg.ChatAdminRights {
	rights := tg.ChatAdminRights{}
	for _, r := range strings.Split(s, ",") {
		switch strings.TrimSpace(r) {
		case "change_info":
			rights.ChangeInfo = true
		case "post_messages":
			rights.PostMessages = true
		case "edit_messages":
			rights.EditMessages = true
		case "delete_messages":
			rights.DeleteMessages = true
		case "ban_users":
			rights.BanUsers = true
		case "invite_users":
			rights.InviteUsers = true
		case "pin_messages":
			rights.PinMessages = true
		case "manage_call":
			rights.ManageCall = true
		case "add_admins":
			rights.AddAdmins = true
		case "anonymous":
			rights.Anonymous = true
		case "manage_topics":
			rights.ManageTopics = true
		case "post_stories":
			rights.PostStories = true
		case "edit_stories":
			rights.EditStories = true
		case "delete_stories":
			rights.DeleteStories = true
		}
	}
	return rights
}

func parseBannedRights(s string, untilDate int) tg.ChatBannedRights {
	rights := tg.ChatBannedRights{UntilDate: untilDate}
	for _, r := range strings.Split(s, ",") {
		switch strings.TrimSpace(r) {
		case "view_messages":
			rights.ViewMessages = true
		case "send_messages":
			rights.SendMessages = true
		case "send_media":
			rights.SendMedia = true
		case "send_stickers":
			rights.SendStickers = true
		case "send_gifs":
			rights.SendGifs = true
		case "send_games":
			rights.SendGames = true
		case "send_inline":
			rights.SendInline = true
		case "embed_links":
			rights.EmbedLinks = true
		case "send_polls":
			rights.SendPolls = true
		case "change_info":
			rights.ChangeInfo = true
		case "invite_users":
			rights.InviteUsers = true
		case "pin_messages":
			rights.PinMessages = true
		case "manage_topics":
			rights.ManageTopics = true
		case "send_photos":
			rights.SendPhotos = true
		case "send_videos":
			rights.SendVideos = true
		case "send_roundvideos":
			rights.SendRoundvideos = true
		case "send_audios":
			rights.SendAudios = true
		case "send_voices":
			rights.SendVoices = true
		case "send_docs":
			rights.SendDocs = true
		case "send_plain":
			rights.SendPlain = true
		}
	}
	return rights
}

func handleEditAdmin(_ context.Context, _ mcp.CallToolRequest, input editAdminInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	inputChannel, ok := toInputChannel(peer)
	if !ok {
		return mcp.NewToolResultError("peer is not a channel or supergroup"), nil
	}

	userPeer, err := services.ResolvePeer(tgCtx, input.UserID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve user: %v", err)), nil
	}

	inputUser, ok := toInputUser(userPeer)
	if !ok {
		return mcp.NewToolResultError("user_id does not resolve to a user"), nil
	}

	rights := parseAdminRights(input.AdminRights)

	_, err = services.API().ChannelsEditAdmin(tgCtx, &tg.ChannelsEditAdminRequest{
		Channel:     inputChannel,
		UserID:      inputUser,
		AdminRights: rights,
		Rank:        input.Rank,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to edit admin rights: %v", err)), nil
	}

	return mcp.NewToolResultText("Admin rights updated successfully."), nil
}

func handleEditBanned(_ context.Context, _ mcp.CallToolRequest, input editBannedInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	inputChannel, ok := toInputChannel(peer)
	if !ok {
		return mcp.NewToolResultError("peer is not a channel or supergroup"), nil
	}

	participantPeer, err := services.ResolvePeer(tgCtx, input.UserID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve user: %v", err)), nil
	}

	rights := parseBannedRights(input.BannedRights, input.UntilDate)

	_, err = services.API().ChannelsEditBanned(tgCtx, &tg.ChannelsEditBannedRequest{
		Channel:      inputChannel,
		Participant:  participantPeer,
		BannedRights: rights,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to edit banned rights: %v", err)), nil
	}

	return mcp.NewToolResultText("Banned rights updated successfully."), nil
}

func handleGetParticipants(_ context.Context, _ mcp.CallToolRequest, input getParticipantsInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	inputChannel, ok := toInputChannel(peer)
	if !ok {
		return mcp.NewToolResultError("peer is not a channel or supergroup"), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	var filter tg.ChannelParticipantsFilterClass
	switch input.Filter {
	case "admins":
		filter = &tg.ChannelParticipantsAdmins{}
	case "kicked":
		filter = &tg.ChannelParticipantsKicked{Q: input.Query}
	case "banned":
		filter = &tg.ChannelParticipantsBanned{Q: input.Query}
	case "bots":
		filter = &tg.ChannelParticipantsBots{}
	case "search":
		filter = &tg.ChannelParticipantsSearch{Q: input.Query}
	default:
		filter = &tg.ChannelParticipantsRecent{}
	}

	result, err := services.API().ChannelsGetParticipants(tgCtx, &tg.ChannelsGetParticipantsRequest{
		Channel: inputChannel,
		Filter:  filter,
		Limit:   limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get participants: %v", err)), nil
	}

	participants, ok := result.(*tg.ChannelsChannelParticipants)
	if !ok {
		return mcp.NewToolResultError("unexpected response type"), nil
	}

	services.StorePeers(tgCtx, participants.Chats, participants.Users)

	userMap := make(map[int64]*tg.User)
	for _, u := range participants.Users {
		user, ok := u.(*tg.User)
		if ok {
			userMap[user.ID] = user
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Participants (%d):\n", participants.Count)

	for _, p := range participants.Participants {
		switch v := p.(type) {
		case *tg.ChannelParticipant:
			if user, ok := userMap[v.UserID]; ok {
				fmt.Fprintf(&b, "\n[Member] ")
				formatUserInline(&b, user)
				fmt.Fprintf(&b, " (joined: %s)", time.Unix(int64(v.Date), 0).UTC().Format("2006-01-02"))
			}
		case *tg.ChannelParticipantSelf:
			if user, ok := userMap[v.UserID]; ok {
				fmt.Fprintf(&b, "\n[Self] ")
				formatUserInline(&b, user)
			}
		case *tg.ChannelParticipantCreator:
			if user, ok := userMap[v.UserID]; ok {
				fmt.Fprintf(&b, "\n[Creator] ")
				formatUserInline(&b, user)
				if v.Rank != "" {
					fmt.Fprintf(&b, " rank: %s", v.Rank)
				}
			}
		case *tg.ChannelParticipantAdmin:
			if user, ok := userMap[v.UserID]; ok {
				fmt.Fprintf(&b, "\n[Admin] ")
				formatUserInline(&b, user)
				if v.Rank != "" {
					fmt.Fprintf(&b, " rank: %s", v.Rank)
				}
				fmt.Fprintf(&b, " (promoted: %s)", time.Unix(int64(v.Date), 0).UTC().Format("2006-01-02"))
			}
		case *tg.ChannelParticipantBanned:
			peerID := peerToID(v.Peer)
			if user, ok := userMap[peerID]; ok {
				fmt.Fprintf(&b, "\n[Banned] ")
				formatUserInline(&b, user)
			} else {
				fmt.Fprintf(&b, "\n[Banned] ID: %d", peerID)
			}
			fmt.Fprintf(&b, " (until: %s)", formatUntilDate(v.BannedRights.UntilDate))
		case *tg.ChannelParticipantLeft:
			peerID := peerToID(v.Peer)
			if user, ok := userMap[peerID]; ok {
				fmt.Fprintf(&b, "\n[Left] ")
				formatUserInline(&b, user)
			} else {
				fmt.Fprintf(&b, "\n[Left] ID: %d", peerID)
			}
		}
		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleGetAdminLog(_ context.Context, _ mcp.CallToolRequest, input getAdminLogInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	inputChannel, ok := toInputChannel(peer)
	if !ok {
		return mcp.NewToolResultError("peer is not a channel or supergroup"), nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := services.API().ChannelsGetAdminLog(tgCtx, &tg.ChannelsGetAdminLogRequest{
		Channel: inputChannel,
		Q:       input.Query,
		Limit:   limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get admin log: %v", err)), nil
	}

	services.StorePeers(tgCtx, result.Chats, result.Users)

	userMap := make(map[int64]*tg.User)
	for _, u := range result.Users {
		user, ok := u.(*tg.User)
		if ok {
			userMap[user.ID] = user
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Admin Log (%d events):\n", len(result.Events))

	for _, event := range result.Events {
		t := time.Unix(int64(event.Date), 0).UTC().Format("2006-01-02 15:04:05")

		userName := fmt.Sprintf("ID:%d", event.UserID)
		if user, ok := userMap[event.UserID]; ok {
			userName = user.FirstName
			if user.LastName != "" {
				userName += " " + user.LastName
			}
			if user.Username != "" {
				userName += fmt.Sprintf(" (@%s)", user.Username)
			}
		}

		action := describeAdminAction(event.Action)
		fmt.Fprintf(&b, "\n[%d] %s | %s | %s\n", event.ID, t, userName, action)
	}

	if len(result.Events) == 0 {
		b.WriteString("\nNo events found.")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func formatUserInline(b *strings.Builder, user *tg.User) {
	fmt.Fprintf(b, "%s", user.FirstName)
	if user.LastName != "" {
		fmt.Fprintf(b, " %s", user.LastName)
	}
	if user.Username != "" {
		fmt.Fprintf(b, " (@%s)", user.Username)
	}
	fmt.Fprintf(b, " [ID: %d]", user.ID)
}

func formatUntilDate(untilDate int) string {
	if untilDate == 0 {
		return "forever"
	}
	return time.Unix(int64(untilDate), 0).UTC().Format("2006-01-02 15:04:05")
}

func peerToID(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerUser:
		return v.UserID
	case *tg.PeerChat:
		return v.ChatID
	case *tg.PeerChannel:
		return v.ChannelID
	default:
		return 0
	}
}

func describeAdminAction(action tg.ChannelAdminLogEventActionClass) string {
	switch a := action.(type) {
	case *tg.ChannelAdminLogEventActionChangeTitle:
		return fmt.Sprintf("Changed title: %q -> %q", a.PrevValue, a.NewValue)
	case *tg.ChannelAdminLogEventActionChangeAbout:
		return fmt.Sprintf("Changed description: %q -> %q", a.PrevValue, a.NewValue)
	case *tg.ChannelAdminLogEventActionChangeUsername:
		return fmt.Sprintf("Changed username: @%s -> @%s", a.PrevValue, a.NewValue)
	case *tg.ChannelAdminLogEventActionChangePhoto:
		return "Changed photo"
	case *tg.ChannelAdminLogEventActionToggleInvites:
		return fmt.Sprintf("Toggle invites: %v", a.NewValue)
	case *tg.ChannelAdminLogEventActionToggleSignatures:
		return fmt.Sprintf("Toggle signatures: %v", a.NewValue)
	case *tg.ChannelAdminLogEventActionUpdatePinned:
		return "Updated pinned message"
	case *tg.ChannelAdminLogEventActionEditMessage:
		return "Edited message"
	case *tg.ChannelAdminLogEventActionDeleteMessage:
		return "Deleted message"
	case *tg.ChannelAdminLogEventActionParticipantJoin:
		return "User joined"
	case *tg.ChannelAdminLogEventActionParticipantLeave:
		return "User left"
	case *tg.ChannelAdminLogEventActionParticipantInvite:
		return "Invited user"
	case *tg.ChannelAdminLogEventActionParticipantToggleBan:
		return "Changed ban rights"
	case *tg.ChannelAdminLogEventActionParticipantToggleAdmin:
		return "Changed admin rights"
	case *tg.ChannelAdminLogEventActionChangeStickerSet:
		return "Changed sticker set"
	case *tg.ChannelAdminLogEventActionTogglePreHistoryHidden:
		return fmt.Sprintf("Toggle pre-history hidden: %v", a.NewValue)
	case *tg.ChannelAdminLogEventActionChangeLinkedChat:
		return "Changed linked chat"
	case *tg.ChannelAdminLogEventActionChangeLocation:
		return "Changed location"
	case *tg.ChannelAdminLogEventActionToggleSlowMode:
		return fmt.Sprintf("Toggle slow mode: %d seconds", a.NewValue)
	case *tg.ChannelAdminLogEventActionStartGroupCall:
		return "Started group call"
	case *tg.ChannelAdminLogEventActionDiscardGroupCall:
		return "Ended group call"
	case *tg.ChannelAdminLogEventActionParticipantMute:
		return "Muted participant in call"
	case *tg.ChannelAdminLogEventActionParticipantUnmute:
		return "Unmuted participant in call"
	case *tg.ChannelAdminLogEventActionToggleGroupCallSetting:
		return "Changed group call settings"
	case *tg.ChannelAdminLogEventActionParticipantJoinByInvite:
		return "User joined via invite link"
	case *tg.ChannelAdminLogEventActionExportedInviteDelete:
		return "Deleted invite link"
	case *tg.ChannelAdminLogEventActionExportedInviteRevoke:
		return "Revoked invite link"
	case *tg.ChannelAdminLogEventActionExportedInviteEdit:
		return "Edited invite link"
	case *tg.ChannelAdminLogEventActionParticipantVolume:
		return "Changed participant volume"
	case *tg.ChannelAdminLogEventActionChangeHistoryTTL:
		return fmt.Sprintf("Changed message auto-delete: %d seconds", a.NewValue)
	case *tg.ChannelAdminLogEventActionParticipantJoinByRequest:
		return "User join request approved"
	case *tg.ChannelAdminLogEventActionToggleNoForwards:
		return fmt.Sprintf("Toggle no forwards: %v", a.NewValue)
	case *tg.ChannelAdminLogEventActionSendMessage:
		return "Sent message"
	case *tg.ChannelAdminLogEventActionChangeAvailableReactions:
		return "Changed available reactions"
	case *tg.ChannelAdminLogEventActionChangeUsernames:
		return "Changed usernames"
	case *tg.ChannelAdminLogEventActionToggleForum:
		return fmt.Sprintf("Toggle forum: %v", a.NewValue)
	case *tg.ChannelAdminLogEventActionCreateTopic:
		return "Created topic"
	case *tg.ChannelAdminLogEventActionEditTopic:
		return "Edited topic"
	case *tg.ChannelAdminLogEventActionDeleteTopic:
		return "Deleted topic"
	case *tg.ChannelAdminLogEventActionPinTopic:
		return "Pinned topic"
	case *tg.ChannelAdminLogEventActionToggleAntiSpam:
		return fmt.Sprintf("Toggle anti-spam: %v", a.NewValue)
	default:
		return fmt.Sprintf("Unknown action: %T", action)
	}
}
