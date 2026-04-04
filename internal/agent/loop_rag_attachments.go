package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/rag"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ragDocumentPlaceholderNames returns candidate file names that may appear in the channel
// placeholder "[File: NAME — use read_document ...]". Persisted paths use uuid basenames while
// Telegram (and ExtractDocumentContent) use the original display name — we must match both.
func ragDocumentPlaceholderNames(content, refPath string) []string {
	add := func(out *[]string, s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		for _, e := range *out {
			if e == s {
				return
			}
		}
		*out = append(*out, s)
	}
	var names []string
	if n := documentNameFromMediaTagByPath(content, refPath); n != "" {
		add(&names, n)
	}
	add(&names, filepath.Base(refPath))
	return names
}

// documentNameFromMediaTagByPath returns name="..." from the <media:document> tag that contains path="refPath".
func documentNameFromMediaTagByPath(content, refPath string) string {
	if refPath == "" {
		return ""
	}
	pathAttr := `path="` + refPath + `"`
	pos := 0
	for {
		idx := strings.Index(content[pos:], "<media:document")
		if idx < 0 {
			return ""
		}
		idx += pos
		endRel := strings.IndexByte(content[idx:], '>')
		if endRel < 0 {
			return ""
		}
		end := idx + endRel + 1
		tag := content[idx:end]
		pos = end
		if !strings.Contains(tag, pathAttr) {
			continue
		}
		namePrefix := `name="`
		n := strings.Index(tag, namePrefix)
		if n < 0 {
			return ""
		}
		rest := tag[n+len(namePrefix):]
		q := strings.IndexByte(rest, '"')
		if q < 0 {
			return ""
		}
		return rest[:q]
	}
}

func lastUserMessageIndex(messages []providers.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return -1
}

// applyRAGAttachmentExtraction replaces read_document placeholders with extracted text when
// rag_indexing is enabled, and asynchronously indexes supported attachments into memory.
func (l *Loop) applyRAGAttachmentExtraction(ctx context.Context, req *RunRequest, messages []providers.Message, mediaRefs []providers.MediaRef) {
	if !l.ragIndexing.Enabled {
		return
	}
	idx := lastUserMessageIndex(messages)
	if idx < 0 {
		return
	}
	content := messages[idx].Content
	memUser := store.MemoryUserID(ctx)
	changed := false

	for _, ref := range mediaRefs {
		if ref.Kind != "document" || ref.Path == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(ref.Path))
		if !l.ragIndexing.SupportsExt(ext) {
			continue
		}
		base := filepath.Base(ref.Path)
		if base == "" || base == "." {
			base = "file"
		}
		isTextInline := ext == ".md" || ext == ".txt" || strings.EqualFold(ext, ".csv")
		replaced := false
		for _, name := range ragDocumentPlaceholderNames(content, ref.Path) {
			hint := rag.ReadDocumentPlaceholder(name)
			if !strings.Contains(content, hint) {
				continue
			}
			text, err := rag.ExtractText(ctx, ref.Path)
			if err != nil {
				slog.Warn("rag.extract_failed", "file", name, "error", err)
				break
			}
			content = strings.Replace(content, hint, rag.ExtractedContentBlock(name, text), 1)
			changed = true
			replaced = true
			rag.IndexAttachmentAsync(rag.IndexAttachmentParams{
				Ctx:        ctx,
				Memory:     l.memStore,
				AgentID:    l.agentUUID,
				MemoryUser: memUser,
				SessionKey: req.SessionKey,
				FilePath:   ref.Path,
				Text:       text,
				HasMemory:  l.hasMemory,
			})
			break
		}
		if replaced {
			continue
		}
		if isTextInline {
			data, err := os.ReadFile(ref.Path)
			if err != nil {
				slog.Warn("rag.read_text_attachment", "file", base, "error", err)
				continue
			}
			t := strings.TrimSpace(string(data))
			if t == "" {
				continue
			}
			rag.IndexAttachmentAsync(rag.IndexAttachmentParams{
				Ctx:        ctx,
				Memory:     l.memStore,
				AgentID:    l.agentUUID,
				MemoryUser: memUser,
				SessionKey: req.SessionKey,
				FilePath:   ref.Path,
				Text:       t,
				HasMemory:  l.hasMemory,
			})
		}
	}
	if changed {
		messages[idx].Content = content
	}
}
