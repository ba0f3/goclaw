package store

import (
	"context"
	"fmt"
	"time"
)

// RAGDocument stores RAG document metadata.
type RAGDocument struct {
	ID          string
	SourceType  string
	SourceRef   string
	ContentHash string
	Title       string
	MimeType    string
	ByteSize    int
	FetchedAt   time.Time
	ExpiresAt   *time.Time
	Meta        []byte
}

// RAGChunk is a single indexed chunk.
type RAGChunk struct {
	ID         string
	DocumentID string
	ChunkIndex int
	Content    string
	Embedding  []float32
	TokenCount int
}

// RAGQuery configures RAG hybrid search.
type RAGQuery struct {
	Text           string
	Embedding      []float32
	TopK           int
	MinScore       float64
	SourceTypes    []string
	ExcludeExpired bool
}

// RAGResult is a ranked retrieval result.
type RAGResult struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	Score      float64   `json:"score"`
	SourceType string    `json:"source_type"`
	SourceRef  string    `json:"source_ref"`
	Title      string    `json:"title,omitempty"`
	FetchedAt  time.Time `json:"fetched_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// RAGDocumentInfo is used by management APIs.
type RAGDocumentInfo struct {
	ID         string     `json:"id"`
	SourceType string     `json:"source_type"`
	SourceRef  string     `json:"source_ref"`
	Title      string     `json:"title,omitempty"`
	MimeType   string     `json:"mime_type,omitempty"`
	ByteSize   int        `json:"byte_size"`
	ChunkCount int        `json:"chunk_count"`
	FetchedAt  time.Time  `json:"fetched_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

var (
	ErrNotFound  = fmt.Errorf("not found")
	ErrForbidden = fmt.Errorf("forbidden")
)

// RAGStore stores and searches retrieved knowledge chunks.
type RAGStore interface {
	UpsertDocument(ctx context.Context, doc RAGDocument) (string, bool, error)
	InsertChunks(ctx context.Context, chunks []RAGChunk) error
	Search(ctx context.Context, query RAGQuery) ([]RAGResult, error)
	PurgeExpired(ctx context.Context) (int64, error)
	DeleteDocument(ctx context.Context, agentID, ownerID, docID string) error
	LookupCache(ctx context.Context, sourceType, sourceRef, contentHash string) (string, bool, error)
	ListDocuments(ctx context.Context, agentID, ownerID, sourceType string, limit int) ([]RAGDocumentInfo, error)
	ReindexDocument(ctx context.Context, docID string) error
}

