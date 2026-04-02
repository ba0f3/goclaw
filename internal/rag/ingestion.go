package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type EmbeddingProvider interface {
	Name() string
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Config struct {
	Enabled           bool
	MaxFileBytes      int
	ChunkTokens       int
	ChunkOverlapPct   int
	WebIndexEnabled   bool
	DefaultWebTTL     time.Duration
	MaxDocsPerAgent   int
	DefaultSearchTopK int
}

func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		MaxFileBytes:      20 * 1024 * 1024,
		ChunkTokens:       400,
		ChunkOverlapPct:   15,
		WebIndexEnabled:   true,
		DefaultWebTTL:     48 * time.Hour,
		MaxDocsPerAgent:   1000,
		DefaultSearchTopK: 5,
	}
}

type AttachmentInput struct {
	FileName string
	MimeType string
	Bytes    []byte
}

type WebContent struct {
	URL        string
	Content    string
	SourceType string
	TTL        time.Duration
}

type IngestResult struct {
	DocumentID string
	ChunkCount int
	IsNew      bool
}

type Ingester struct {
	cfg       Config
	store     store.RAGStore
	extractor *Extractor
	provider  EmbeddingProvider
}

func NewIngester(cfg Config, ragStore store.RAGStore, provider EmbeddingProvider) *Ingester {
	return &Ingester{
		cfg:       cfg,
		store:     ragStore,
		extractor: NewExtractor(),
		provider:  provider,
	}
}

func (i *Ingester) Enabled() bool { return i != nil && i.cfg.Enabled }

func (i *Ingester) IndexAttachment(ctx context.Context, in AttachmentInput) (*IngestResult, error) {
	if len(in.Bytes) == 0 {
		return nil, fmt.Errorf("empty attachment")
	}
	if i.cfg.MaxFileBytes > 0 && len(in.Bytes) > i.cfg.MaxFileBytes {
		return nil, fmt.Errorf("file too large: %d bytes > %d", len(in.Bytes), i.cfg.MaxFileBytes)
	}
	contentHash := sha256Hex(in.Bytes)
	safeName := sanitizeFileName(in.FileName)
	docID, hit, err := i.store.LookupCache(ctx, "file", safeName, contentHash)
	if err != nil {
		return nil, err
	}
	if hit {
		return &IngestResult{DocumentID: docID, ChunkCount: 0, IsNew: false}, nil
	}

	text, detectedMime, err := i.extractor.ExtractText(ctx, in.MimeType, in.Bytes, safeName)
	if err != nil {
		return nil, err
	}
	chunks := ChunkText(text, ChunkConfig{
		MaxTokens:  i.cfg.ChunkTokens,
		OverlapPct: i.cfg.ChunkOverlapPct,
		Strategy:   "sentence",
	})
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks extracted")
	}
	chunkTexts := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		chunkTexts = append(chunkTexts, ch.Content)
	}
	embeddings, err := i.embedChunks(ctx, chunkTexts)
	if err != nil {
		return nil, err
	}
	doc := store.RAGDocument{
		SourceType:  "file",
		SourceRef:   safeName,
		ContentHash: contentHash,
		Title:       safeName,
		MimeType:    detectedMime,
		ByteSize:    len(in.Bytes),
		FetchedAt:   time.Now(),
	}
	insertedDocID, isNew, err := i.store.UpsertDocument(ctx, doc)
	if err != nil {
		return nil, err
	}
	ragChunks := make([]store.RAGChunk, 0, len(chunks))
	for idx, ch := range chunks {
		ragChunks = append(ragChunks, store.RAGChunk{
			ID:         uuid.Must(uuid.NewV7()).String(),
			DocumentID: insertedDocID,
			ChunkIndex: idx,
			Content:    ch.Content,
			Embedding:  embeddings[idx],
			TokenCount: estimateTokens(ch.Content),
		})
	}
	if err := i.store.InsertChunks(ctx, ragChunks); err != nil {
		return nil, err
	}
	return &IngestResult{DocumentID: insertedDocID, ChunkCount: len(ragChunks), IsNew: isNew}, nil
}

func (i *Ingester) IndexWebContent(ctx context.Context, wc WebContent) error {
	if !i.cfg.WebIndexEnabled {
		return nil
	}
	contentHash := sha256Hex([]byte(wc.Content))
	docID, hit, err := i.store.LookupCache(ctx, wc.SourceType, wc.URL, contentHash)
	if err != nil {
		return err
	}
	if hit && docID != "" {
		return nil
	}
	chunks := ChunkText(wc.Content, ChunkConfig{
		MaxTokens:  i.cfg.ChunkTokens,
		OverlapPct: i.cfg.ChunkOverlapPct,
		Strategy:   "sentence",
	})
	if len(chunks) == 0 {
		return nil
	}
	chunkTexts := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		chunkTexts = append(chunkTexts, ch.Content)
	}
	embeddings, err := i.embedChunks(ctx, chunkTexts)
	if err != nil {
		return err
	}
	fetched := time.Now()
	ttl := wc.TTL
	if ttl <= 0 {
		ttl = i.cfg.DefaultWebTTL
	}
	expires := fetched.Add(ttl)
	docID, _, err = i.store.UpsertDocument(ctx, store.RAGDocument{
		SourceType:  wc.SourceType,
		SourceRef:   wc.URL,
		ContentHash: contentHash,
		Title:       wc.URL,
		MimeType:    "text/plain",
		ByteSize:    len(wc.Content),
		FetchedAt:   fetched,
		ExpiresAt:   &expires,
	})
	if err != nil {
		return err
	}
	ragChunks := make([]store.RAGChunk, 0, len(chunks))
	for idx, ch := range chunks {
		ragChunks = append(ragChunks, store.RAGChunk{
			ID:         uuid.Must(uuid.NewV7()).String(),
			DocumentID: docID,
			ChunkIndex: idx,
			Content:    ch.Content,
			Embedding:  embeddings[idx],
			TokenCount: estimateTokens(ch.Content),
		})
	}
	return i.store.InsertChunks(ctx, ragChunks)
}

func (i *Ingester) Search(ctx context.Context, query string, embedding []float32, topK int, sourceTypes []string) ([]store.RAGResult, error) {
	if topK <= 0 {
		topK = i.cfg.DefaultSearchTopK
	}
	return i.store.Search(ctx, store.RAGQuery{
		Text:           query,
		Embedding:      embedding,
		TopK:           topK,
		MinScore:       0.0,
		SourceTypes:    sourceTypes,
		ExcludeExpired: true,
	})
}

func (i *Ingester) EmbedQuery(ctx context.Context, q string) ([]float32, error) {
	if i.provider == nil {
		return nil, fmt.Errorf("embedding provider not configured")
	}
	embs, err := i.provider.Embed(ctx, []string{q})
	if err != nil {
		return nil, err
	}
	if len(embs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return embs[0], nil
}

func (i *Ingester) embedChunks(ctx context.Context, chunks []string) ([][]float32, error) {
	if i.provider == nil {
		return nil, fmt.Errorf("embedding provider not configured")
	}
	embs, err := i.provider.Embed(ctx, chunks)
	if err != nil {
		return nil, err
	}
	if len(embs) != len(chunks) {
		return nil, fmt.Errorf("embedding count mismatch")
	}
	return embs, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment.bin"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}

// estimateTokens is a conservative approximation to support token_count metadata.
// RAG search does not depend on exact token counts.
func estimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Approx: 1 token ~= 4 chars for English-ish text.
	return max(1, len(s)/4)
}

