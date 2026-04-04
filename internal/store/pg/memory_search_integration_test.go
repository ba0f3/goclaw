//go:build integration

package pg

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pgxstdlib"
)

// This test is intentionally guarded by the "integration" build tag.
//
// It validates the most brittle part of RAG search: the SQL branching + parameter
// numbering in ftsSearch/vectorSearch when RAGGroupID/RAGPersonalOwnerID are set.
//
// To run:
//   PG_TEST_DSN='postgres://...' go test -tags integration ./internal/store/pg -run TestMemorySearch_RAGBranches
func TestMemorySearch_RAGBranches(t *testing.T) {
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := store.WithTenantID(context.Background(), store.MasterTenantID)

	// Use a temporary agent UUID to avoid colliding with real data.
	agentID := uuid.New()

	// Create store and ensure schema preconditions. (We rely on existing migrations in the DB.)
	mem := NewPGMemoryStore(db, DefaultPGMemoryConfig())
	defer mem.Close()
	_ = pgxstdlib.EnsureSearchExtensions(ctx, db) // best-effort

	// Insert two documents: one personal DM and one group-shared.
	userID := "group:telegram:-100"      // group-scoped id
	personal := "12345"                  // personal sender id
	ragGroupID := "telegram:group:-100"  // visibility group
	groupPath := "rag/group/" + ragGroupID + "/g.pdf"
	dmPath := "rag/dm/d.pdf"

	if err := mem.PutDocument(ctx, agentID.String(), userID, groupPath, "hello group"); err != nil {
		t.Fatal(err)
	}
	if err := mem.IndexDocument(ctx, agentID.String(), userID, groupPath); err != nil {
		t.Fatal(err)
	}
	if err := mem.PutDocument(ctx, agentID.String(), personal, dmPath, "hello dm"); err != nil {
		t.Fatal(err)
	}
	if err := mem.IndexDocument(ctx, agentID.String(), personal, dmPath); err != nil {
		t.Fatal(err)
	}

	// Search from group context: should see groupPath + personal dmPath via RAGPersonalOwnerID.
	res, err := mem.Search(ctx, "hello", agentID.String(), userID, store.MemorySearchOptions{
		MaxResults:          10,
		RAGGroupID:          ragGroupID,
		RAGPersonalOwnerID:  personal,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatalf("expected results, got 0")
	}
}

