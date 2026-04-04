package rag

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/safego"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Limit concurrent attachment indexing to avoid unbounded goroutine spikes.
// Indexing may call chunking + embeddings; keep this conservative.
var indexAttachmentSem = make(chan struct{}, 8)

// IndexAttachmentParams carries everything needed to persist RAG chunks asynchronously.
type IndexAttachmentParams struct {
	Ctx         context.Context
	Memory      store.MemoryStore
	AgentID     uuid.UUID
	MemoryUser  string
	SessionKey  string
	FilePath    string
	Text        string
	HasMemory   bool
}

// Index writes extracted attachment text to scoped memory paths and re-indexes chunks.
func Index(ctx context.Context, scope UploadScope, agentID, fileName, text string, mem store.MemoryStore) error {
	full := strings.TrimSpace(text)
	if mem == nil || full == "" {
		return nil
	}
	path := scope.MemoryPath(fileName)
	if err := mem.PutDocument(ctx, agentID, scope.OwnerID, path, full); err != nil {
		return err
	}
	return mem.IndexDocument(ctx, agentID, scope.OwnerID, path)
}

// IndexAttachmentAsync indexes supported attachments under rag/dm/ or rag/group/{groupID}/.
// Runs in a background goroutine; errors are logged only.
func IndexAttachmentAsync(p IndexAttachmentParams) {
	if p.Memory == nil || !p.HasMemory || p.Text == "" {
		return
	}
	if p.AgentID == uuid.Nil {
		return
	}
	agentStr := p.AgentID.String()
	base := filepath.Base(p.FilePath)
	if base == "" || base == "." {
		base = "attachment"
	}
	scope := ParseScope(p.SessionKey, p.MemoryUser)
	if scope.OwnerID == "" {
		slog.Warn("rag.index.skip_no_owner", "session", p.SessionKey)
		return
	}

	bg := context.WithoutCancel(p.Ctx)

	go func() {
		indexAttachmentSem <- struct{}{}
		defer func() { <-indexAttachmentSem }()
		defer safego.Recover(nil, "component", "rag_index_attachment", "session", p.SessionKey)
		if err := Index(bg, scope, agentStr, base, p.Text, p.Memory); err != nil {
			slog.Warn("rag.index.failed", "path", scope.MemoryPath(base), "error", err)
		}
	}()
}

// ExtractedContentBlock wraps extracted text for the user message.
func ExtractedContentBlock(fileName, text string) string {
	return fmt.Sprintf("[File: %s]\n<extracted_content>\n%s\n</extracted_content>", fileName, text)
}
