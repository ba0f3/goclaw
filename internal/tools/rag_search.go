package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/rag"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// RAGSearchTool implements the rag_search tool for hybrid semantic + FTS retrieval.
type RAGSearchTool struct {
	ingester *rag.Ingester
}

func NewRAGSearchTool() *RAGSearchTool { return &RAGSearchTool{} }

// SetRAGIngester wires the internal RAG ingester.
func (t *RAGSearchTool) SetRAGIngester(i *rag.Ingester) { t.ingester = i }

func (t *RAGSearchTool) Name() string { return "rag_search" }

func (t *RAGSearchTool) Description() string {
	return `Search indexed knowledge from attached files and visited web pages.
Use when you need to recall content from documents the user has shared, or from
web pages previously fetched/searched in this session or earlier sessions.
Returns ranked text chunks with source attribution.`
}

func (t *RAGSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language search query.",
			},
			"top_k": map[string]any{
				"type":        "number",
				"description": "Number of results to return (1-10, default 5).",
				"minimum":     1.0,
				"maximum":     10.0,
			},
			"source_types": map[string]any{
				"type":        "array",
				"description": "Filter by source: 'file', 'web_fetch', 'web_search'.",
				"items": map[string]any{
					"type": "string",
					"enum": []string{"file", "web_fetch", "web_search"},
				},
			},
		},
		"required": []string{"query"},
	}
}

func (t *RAGSearchTool) Execute(ctx context.Context, args map[string]any) *Result {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("query is required")
	}
	topK := 5
	if v, ok := args["top_k"].(float64); ok {
		topK = int(v)
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 10 {
		topK = 10
	}
	var sourceTypes []string
	if arr, ok := args["source_types"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				sourceTypes = append(sourceTypes, s)
			}
		}
	}

	agentID := store.AgentIDFromContext(ctx)
	if t.ingester == nil || agentID == uuid.Nil {
		return ErrorResult("rag system not available")
	}

	emb, err := t.ingester.EmbedQuery(ctx, query)
	if err != nil {
		return ErrorResult(fmt.Sprintf("rag_search embedding failed: %v", err))
	}

	results, err := t.ingester.Search(ctx, query, emb, topK, sourceTypes)
	if err != nil {
		return ErrorResult(fmt.Sprintf("rag_search failed: %v", err))
	}
	if len(results) == 0 {
		return NewResult("No RAG results found for query: " + query)
	}

	// Return structured JSON for providers; frontends can format.
	data, _ := json.MarshalIndent(map[string]any{
		"results": results,
		"count":   len(results),
	}, "", "  ")
	return NewResult(string(data))
}

