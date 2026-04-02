package channels

import "context"

// Attachment is a raw in-memory file payload suitable for ingestion.
// Filenames must be sanitized (no path traversal, no null bytes).
type Attachment struct {
	Bytes    []byte
	MimeType string // from magic bytes; do not trust client headers
	FileName string // sanitized base name
	Size     int
}

// SessionInfo provides minimal routing context for attachment ingestion.
// Keep this small; most identity must come from ctx.
type SessionInfo struct {
	SessionKey string
}

// AttachmentHandler ingests and indexes attachments.
// Returns a short user-facing confirmation message.
type AttachmentHandler interface {
	HandleAttachment(ctx context.Context, att Attachment, session SessionInfo) (string, error)
}

