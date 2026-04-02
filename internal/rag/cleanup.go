package rag

import (
	"context"
	"log/slog"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type CleanupConfig struct {
	Interval        time.Duration
	MaxDocsPerAgent int
	MaxChunkAge     time.Duration
}

// StartCleanupWorker runs periodic cleanup for expired web content and quota enforcement.
// It logs IDs and counts only (never user content).
func StartCleanupWorker(ctx context.Context, ragStore store.RAGStore, cfg CleanupConfig) {
	if ragStore == nil {
		return
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
			func() {
				defer cancel()
				deletedDocs, err := ragStore.PurgeExpired(bg)
				if err != nil {
					slog.Warn("rag.cleanup: purge expired failed", "error", err)
					return
				}
				if deletedDocs > 0 {
					slog.Info("rag.cleanup", "expired_documents_deleted", deletedDocs)
				}
			}()
			// NOTE: Per-agent quota eviction and MaxChunkAge enforcement require
			// management queries (count/oldest) and are implemented in the gateway
			// methods / store extensions in the next iteration.
		}
	}
}

