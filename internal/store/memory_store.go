package store

import "context"

// DocumentInfo describes a memory document.
type DocumentInfo struct {
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	AgentID   string `json:"agent_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// MemorySearchResult is a single result from memory search.
type MemorySearchResult struct {
	Path         string  `json:"path"`
	StartLine    int     `json:"start_line"`
	EndLine      int     `json:"end_line"`
	Score        float64 `json:"score"`
	Snippet      string  `json:"snippet"`
	Source       string  `json:"source"`
	Scope        string  `json:"scope,omitempty"` // "global" or "personal"
	ChunkUserID  string  `json:"chunk_user_id,omitempty"` // memory_chunks.user_id (for RAG path filtering)
}

// MemorySearchOptions configures a memory search query.
type MemorySearchOptions struct {
	MaxResults   int
	MinScore     float64
	Source       string  // "memory", "sessions", ""
	PathPrefix   string
	VectorWeight float64 // per-agent override (0 = use store default)
	TextWeight   float64 // per-agent override (0 = use store default)
	// RAGGroupID is the current chat's group scope (e.g. telegram:group:-100). Empty for DM / WS direct.
	// When set, rag/group/{RAGGroupID}/... is visible to all participants; rag/dm/... only if chunk owner matches querier.
	RAGGroupID string
	// RAGPersonalOwnerID is the real sender identity in group chats (e.g. Telegram numeric user id).
	// Memory rows for rag/dm/ are keyed by that id from DM uploads; group-scoped UserID does not match.
	// When set with RAGGroupID, search includes rag/dm/ chunks owned by this id.
	RAGPersonalOwnerID string
}

// EmbeddingProvider generates vector embeddings for text.
type EmbeddingProvider interface {
	Name() string
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// DocumentDetail provides full document info including chunk/embedding stats.
type DocumentDetail struct {
	Path          string `json:"path"`
	Content       string `json:"content"`
	Hash          string `json:"hash"`
	UserID        string `json:"user_id,omitempty"`
	ChunkCount    int    `json:"chunk_count"`
	EmbeddedCount int    `json:"embedded_count"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// ChunkInfo describes a single memory chunk.
type ChunkInfo struct {
	ID           string `json:"id"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	TextPreview  string `json:"text_preview"`
	HasEmbedding bool   `json:"has_embedding"`
}

// MemoryStore manages memory documents and search.
type MemoryStore interface {
	// Document CRUD
	GetDocument(ctx context.Context, agentID, userID, path string) (string, error)
	PutDocument(ctx context.Context, agentID, userID, path, content string) error
	DeleteDocument(ctx context.Context, agentID, userID, path string) error
	// DeleteDocumentsByPathPrefix removes all documents (and chunks, via FK) whose path starts with prefix for the agent.
	DeleteDocumentsByPathPrefix(ctx context.Context, agentID, prefix string) error
	// DeleteDocumentsByPathPrefixAndUser removes documents for one owner whose path starts with prefix.
	DeleteDocumentsByPathPrefixAndUser(ctx context.Context, agentID, userID, prefix string) error
	ListDocuments(ctx context.Context, agentID, userID string) ([]DocumentInfo, error)

	// Admin queries
	ListAllDocumentsGlobal(ctx context.Context) ([]DocumentInfo, error)
	ListAllDocuments(ctx context.Context, agentID string) ([]DocumentInfo, error)
	GetDocumentDetail(ctx context.Context, agentID, userID, path string) (*DocumentDetail, error)
	ListChunks(ctx context.Context, agentID, userID, path string) ([]ChunkInfo, error)

	// Search
	Search(ctx context.Context, query string, agentID, userID string, opts MemorySearchOptions) ([]MemorySearchResult, error)

	// Indexing
	IndexDocument(ctx context.Context, agentID, userID, path string) error
	IndexAll(ctx context.Context, agentID, userID string) error

	SetEmbeddingProvider(provider EmbeddingProvider)
	Close() error
}
