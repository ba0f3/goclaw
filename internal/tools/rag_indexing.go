package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/rag"
)

// RAGIngester is a narrow interface implemented by *rag.Ingester.
// Tools depend on this interface to avoid importing store/pg details.
type RAGIngester interface {
	Enabled() bool
	IndexWebContent(ctx context.Context, wc rag.WebContent) error
}

// IndexWebContentAsync indexes web content in a detached goroutine.
// It always recovers from panics and never blocks the caller.
func IndexWebContentAsync(parent context.Context, ing RAGIngester, sourceType, url, content string, ttl time.Duration) {
	if ing == nil || !ing.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	go func() {
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Warn("rag indexer panic recovered", "panic", rec)
			}
		}()
		_ = ing.IndexWebContent(ctx, rag.WebContent{
			URL:        url,
			Content:    content,
			SourceType: sourceType,
			TTL:        ttl,
		})
	}()
}

