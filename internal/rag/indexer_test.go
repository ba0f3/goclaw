package rag

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type mockMemoryStore struct {
	putCalls   int
	indexCalls int
	putErr     error
	indexErr   error
	mu         sync.Mutex
}

func (m *mockMemoryStore) GetDocument(_ context.Context, _, _, _ string) (string, error) {
	return "", errors.New("not implemented")
}
func (m *mockMemoryStore) PutDocument(_ context.Context, _, _, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putCalls++
	return m.putErr
}
func (m *mockMemoryStore) DeleteDocument(_ context.Context, _, _, _ string) error { return nil }
func (m *mockMemoryStore) DeleteDocumentsByPathPrefix(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockMemoryStore) DeleteDocumentsByPathPrefixAndUser(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockMemoryStore) ListDocuments(_ context.Context, _, _ string) ([]store.DocumentInfo, error) {
	return nil, nil
}
func (m *mockMemoryStore) ListAllDocumentsGlobal(_ context.Context) ([]store.DocumentInfo, error) {
	return nil, nil
}
func (m *mockMemoryStore) ListAllDocuments(_ context.Context, _ string) ([]store.DocumentInfo, error) {
	return nil, nil
}
func (m *mockMemoryStore) GetDocumentDetail(_ context.Context, _, _, _ string) (*store.DocumentDetail, error) {
	return nil, nil
}
func (m *mockMemoryStore) ListChunks(_ context.Context, _, _, _ string) ([]store.ChunkInfo, error) {
	return nil, nil
}
func (m *mockMemoryStore) Search(_ context.Context, _ string, _, _ string, _ store.MemorySearchOptions) ([]store.MemorySearchResult, error) {
	return nil, nil
}
func (m *mockMemoryStore) IndexDocument(_ context.Context, _, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexCalls++
	return m.indexErr
}
func (m *mockMemoryStore) IndexAll(_ context.Context, _, _ string) error { return nil }
func (m *mockMemoryStore) BackfillEmbeddings(_ context.Context) (int, error) { return 0, nil }
func (m *mockMemoryStore) SetEmbeddingProvider(_ store.EmbeddingProvider)     {}
func (m *mockMemoryStore) Close() error                                      { return nil }

func TestIndex_NoMemOrEmptyTextIsNoop(t *testing.T) {
	if err := Index(context.Background(), UploadScope{OwnerID: "u1"}, "a", "f.txt", " ", nil); err != nil {
		t.Fatal(err)
	}
	ms := &mockMemoryStore{}
	if err := Index(context.Background(), UploadScope{OwnerID: "u1"}, "a", "f.txt", "", ms); err != nil {
		t.Fatal(err)
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.putCalls != 0 || ms.indexCalls != 0 {
		t.Fatalf("calls = put:%d index:%d, want 0", ms.putCalls, ms.indexCalls)
	}
}

func TestIndex_PutThenIndex(t *testing.T) {
	ms := &mockMemoryStore{}
	err := Index(context.Background(), UploadScope{OwnerID: "u1"}, "a", "f.txt", "hello", ms)
	if err != nil {
		t.Fatal(err)
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.putCalls != 1 || ms.indexCalls != 1 {
		t.Fatalf("calls = put:%d index:%d, want 1/1", ms.putCalls, ms.indexCalls)
	}
}

func TestExtractedContentBlock_Format(t *testing.T) {
	got := ExtractedContentBlock("a.pdf", "TEXT")
	if !strings.Contains(got, "[File: a.pdf]") || !strings.Contains(got, "<extracted_content>") {
		t.Fatalf("unexpected block: %q", got)
	}
}

func TestIndexAttachmentAsync_SkipsWithoutOwnerOrDeps(t *testing.T) {
	// No memory / no text / no agent should all short-circuit without incrementing calls.
	ms := &mockMemoryStore{}
	IndexAttachmentAsync(IndexAttachmentParams{Memory: nil, HasMemory: true, Text: "x"})
	IndexAttachmentAsync(IndexAttachmentParams{Memory: ms, HasMemory: false, Text: "x"})
	IndexAttachmentAsync(IndexAttachmentParams{Memory: ms, HasMemory: true, Text: ""})
	IndexAttachmentAsync(IndexAttachmentParams{Memory: ms, HasMemory: true, Text: "x"}) // AgentID nil
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.putCalls != 0 || ms.indexCalls != 0 {
		t.Fatalf("calls = put:%d index:%d, want 0", ms.putCalls, ms.indexCalls)
	}
}

