package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/telegram-mcp/services"
)

// Input structs

type downloadMediaInput struct {
	Peer        string `json:"peer" jsonschema:"required"`
	MessageID   int    `json:"message_id" jsonschema:"required"`
	DownloadDir string `json:"download_dir"`
}

type sendMediaInput struct {
	Peer     string `json:"peer" jsonschema:"required"`
	FilePath string `json:"file_path" jsonschema:"required"`
	Caption  string `json:"caption"`
}

type getFileInfoInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
}

type viewImageInput struct {
	Peer      string `json:"peer" jsonschema:"required"`
	MessageID int    `json:"message_id" jsonschema:"required"`
}

func RegisterMediaTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("telegram_download_media",
			mcp.WithDescription("Download media from a Telegram message"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message containing media")),
			mcp.WithString("download_dir", mcp.Description("Directory to save the file (default ./downloads)")),
		),
		mcp.NewTypedToolHandler(handleDownloadMedia),
	)

	s.AddTool(
		mcp.NewTool("telegram_send_media",
			mcp.WithDescription("Send a file/media to a Telegram chat"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the file to send")),
			mcp.WithString("caption", mcp.Description("Caption for the media (optional)")),
		),
		mcp.NewTypedToolHandler(handleSendMedia),
	)

	s.AddTool(
		mcp.NewTool("telegram_get_file_info",
			mcp.WithDescription("Get information about media in a Telegram message without downloading"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message to inspect")),
		),
		mcp.NewTypedToolHandler(handleGetFileInfo),
	)

	s.AddTool(
		mcp.NewTool("telegram_view_image",
			mcp.WithDescription("Download and return a photo from a Telegram message as image content for AI viewing"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("peer", mcp.Required(), mcp.Description("Chat ID or @username")),
			mcp.WithNumber("message_id", mcp.Required(), mcp.Description("ID of the message containing the photo")),
		),
		mcp.NewTypedToolHandler(handleViewImage),
	)
}

// Helper: get a single message by ID, handling both channel and non-channel peers

func getMessageByID(ctx context.Context, peer tg.InputPeerClass, msgID int) (*tg.Message, error) {
	ids := []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}}

	var result tg.MessagesMessagesClass
	var err error

	if ch, ok := peer.(*tg.InputPeerChannel); ok {
		result, err = services.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  ch.ChannelID,
				AccessHash: ch.AccessHash,
			},
			ID: ids,
		})
	} else {
		result, err = services.API().MessagesGetMessages(ctx, ids)
	}

	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}

	msgs := extractMessages(ctx, result)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message %d not found", msgID)
	}

	msg, ok := msgs[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("message %d is not a regular message", msgID)
	}

	return msg, nil
}

// Helper: detect MIME type from file extension

func mimeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func handleDownloadMedia(_ context.Context, _ mcp.CallToolRequest, input downloadMediaInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	msg, err := getMessageByID(tgCtx, peer, input.MessageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get message: %v", err)), nil
	}

	if msg.Media == nil {
		return mcp.NewToolResultError("message has no media"), nil
	}

	downloadDir := input.DownloadDir
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	if strings.Contains(downloadDir, "..") {
		return mcp.NewToolResultError("path traversal not allowed in download_dir"), nil
	}
	if err := os.MkdirAll(downloadDir, 0700); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create download dir: %v", err)), nil
	}

	d := downloader.NewDownloader()

	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return mcp.NewToolResultError("photo not available"), nil
		}

		// Find the largest photo size
		var bestType string
		for _, size := range photo.Sizes {
			t := size.GetType()
			// Prefer larger sizes: y > x > m > s
			if bestType == "" || t > bestType {
				bestType = t
			}
		}
		if bestType == "" {
			return mcp.NewToolResultError("no photo sizes available"), nil
		}

		loc := &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     bestType,
		}

		filePath := filepath.Join(downloadDir, fmt.Sprintf("photo_%d_%d.jpg", msg.ID, photo.ID))
		_, err = d.Download(services.API(), loc).ToPath(tgCtx, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to download photo: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Photo downloaded to: %s", filePath)), nil

	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return mcp.NewToolResultError("document not available"), nil
		}

		// Determine filename from attributes
		filename := fmt.Sprintf("doc_%d_%d", msg.ID, doc.ID)
		for _, attr := range doc.Attributes {
			if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
				filename = fn.FileName
				break
			}
		}

		loc := &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}

		filePath := filepath.Join(downloadDir, filename)
		_, err = d.Download(services.API(), loc).ToPath(tgCtx, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to download document: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Document downloaded to: %s", filePath)), nil

	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported media type: %T", msg.Media)), nil
	}
}

func handleSendMedia(_ context.Context, _ mcp.CallToolRequest, input sendMediaInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	if strings.Contains(input.FilePath, "..") {
		return mcp.NewToolResultError("path traversal not allowed in file_path"), nil
	}
	if _, err := os.Stat(input.FilePath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("file not found: %v", err)), nil
	}

	u := uploader.NewUploader(services.API())
	uploaded, err := u.FromPath(tgCtx, input.FilePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to upload file: %v", err)), nil
	}

	mimeType := mimeFromPath(input.FilePath)

	_, err = services.API().MessagesSendMedia(tgCtx, &tg.MessagesSendMediaRequest{
		Peer: peer,
		Media: &tg.InputMediaUploadedDocument{
			File:     uploaded,
			MimeType: mimeType,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: filepath.Base(input.FilePath)},
			},
		},
		Message:  input.Caption,
		RandomID: randomID(),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send media: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Media sent successfully: %s", filepath.Base(input.FilePath))), nil
}

func handleGetFileInfo(_ context.Context, _ mcp.CallToolRequest, input getFileInfoInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	msg, err := getMessageByID(tgCtx, peer, input.MessageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get message: %v", err)), nil
	}

	if msg.Media == nil {
		return mcp.NewToolResultError("message has no media"), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Message ID: %d\n", msg.ID)

	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		b.WriteString("Type: Photo\n")

		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			b.WriteString("Photo data not available\n")
			break
		}

		fmt.Fprintf(&b, "Photo ID: %d\n", photo.ID)
		fmt.Fprintf(&b, "Available sizes:\n")
		for _, size := range photo.Sizes {
			switch s := size.(type) {
			case *tg.PhotoSize:
				fmt.Fprintf(&b, "  - %s: %dx%d (%s)\n", s.Type, s.W, s.H, formatSize(int64(s.Size)))
			case *tg.PhotoSizeProgressive:
				fmt.Fprintf(&b, "  - %s: %dx%d (progressive)\n", s.Type, s.W, s.H)
			case *tg.PhotoCachedSize:
				fmt.Fprintf(&b, "  - %s: %dx%d (cached)\n", s.Type, s.W, s.H)
			case *tg.PhotoStrippedSize:
				fmt.Fprintf(&b, "  - %s: (stripped thumbnail)\n", s.Type)
			}
		}

	case *tg.MessageMediaDocument:
		b.WriteString("Type: Document\n")

		doc, ok := media.Document.(*tg.Document)
		if !ok {
			b.WriteString("Document data not available\n")
			break
		}

		fmt.Fprintf(&b, "Document ID: %d\n", doc.ID)
		fmt.Fprintf(&b, "MIME Type: %s\n", doc.MimeType)
		fmt.Fprintf(&b, "Size: %s\n", formatSize(doc.Size))

		for _, attr := range doc.Attributes {
			switch a := attr.(type) {
			case *tg.DocumentAttributeFilename:
				fmt.Fprintf(&b, "Filename: %s\n", a.FileName)
			case *tg.DocumentAttributeVideo:
				fmt.Fprintf(&b, "Video: %dx%d, duration %.1fs\n", a.W, a.H, a.Duration)
			case *tg.DocumentAttributeAudio:
				fmt.Fprintf(&b, "Audio: duration %ds", a.Duration)
				if a.Title != "" {
					fmt.Fprintf(&b, ", title: %s", a.Title)
				}
				if a.Performer != "" {
					fmt.Fprintf(&b, ", performer: %s", a.Performer)
				}
				b.WriteString("\n")
			case *tg.DocumentAttributeImageSize:
				fmt.Fprintf(&b, "Image: %dx%d\n", a.W, a.H)
			case *tg.DocumentAttributeSticker:
				fmt.Fprintf(&b, "Sticker: %s\n", a.Alt)
			case *tg.DocumentAttributeAnimated:
				b.WriteString("Animated: yes\n")
			}
		}

	default:
		fmt.Fprintf(&b, "Type: %T (unsupported for detailed info)\n", msg.Media)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleViewImage(_ context.Context, _ mcp.CallToolRequest, input viewImageInput) (*mcp.CallToolResult, error) {
	tgCtx := services.Context()

	peer, err := services.ResolvePeer(tgCtx, input.Peer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve peer: %v", err)), nil
	}

	msg, err := getMessageByID(tgCtx, peer, input.MessageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get message: %v", err)), nil
	}

	if msg.Media == nil {
		return mcp.NewToolResultError("message has no media"), nil
	}

	photo, ok := msg.Media.(*tg.MessageMediaPhoto)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("message media is %T, not a photo", msg.Media)), nil
	}

	p, ok := photo.Photo.(*tg.Photo)
	if !ok {
		return mcp.NewToolResultError("photo not available"), nil
	}

	// Pick the best size for AI viewing (x=800px is a good balance)
	bestType := "x"
	hasBest := false
	for _, size := range p.Sizes {
		if size.GetType() == bestType {
			hasBest = true
			break
		}
	}
	if !hasBest {
		// Fallback to largest available
		for _, size := range p.Sizes {
			t := size.GetType()
			if t > bestType || bestType == "x" {
				bestType = t
			}
		}
	}

	loc := &tg.InputPhotoFileLocation{
		ID:            p.ID,
		AccessHash:    p.AccessHash,
		FileReference: p.FileReference,
		ThumbSize:     bestType,
	}

	var buf bytes.Buffer
	d := downloader.NewDownloader()
	if _, err := d.Download(services.API(), loc).Stream(tgCtx, &buf); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to download photo: %v", err)), nil
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return mcp.NewToolResultImage(fmt.Sprintf("Photo from message %d", msg.ID), b64, "image/jpeg"), nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
