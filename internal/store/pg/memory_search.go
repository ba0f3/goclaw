package pg

import (
	"context"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/rag"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Search performs hybrid search (FTS + vector) over memory_chunks.
// Merges global (user_id IS NULL) + per-user chunks, with user boost.
func (s *PGMemoryStore) Search(ctx context.Context, query string, agentID, userID string, opts store.MemorySearchOptions) ([]store.MemorySearchResult, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = s.cfg.MaxResults
	}

	aid := mustParseUUID(agentID)
	ragGroupPrefix := rag.GroupDocumentPathPrefix(opts.RAGGroupID)

	// FTS search using tsvector
	ftsResults, err := s.ftsSearch(ctx, query, aid, userID, ragGroupPrefix, opts.RAGPersonalOwnerID, maxResults*2)
	if err != nil {
		return nil, err
	}

	// Vector search if provider available
	var vecResults []scoredChunk
	if s.provider != nil {
		embeddings, err := s.provider.Embed(ctx, []string{query})
		if err == nil && len(embeddings) > 0 {
			vecResults, err = s.vectorSearch(ctx, embeddings[0], aid, userID, ragGroupPrefix, opts.RAGPersonalOwnerID, maxResults*2)
			if err != nil {
				vecResults = nil
			}
		}
	}

	sharedMem := store.IsSharedMemory(ctx)
	ftsResults = filterRAGScoredChunks(ftsResults, userID, opts.RAGGroupID, opts.RAGPersonalOwnerID, sharedMem)
	vecResults = filterRAGScoredChunks(vecResults, userID, opts.RAGGroupID, opts.RAGPersonalOwnerID, sharedMem)

	// Merge results — use per-query overrides if set, else store defaults
	textW, vecW := s.cfg.TextWeight, s.cfg.VectorWeight
	if opts.TextWeight > 0 {
		textW = opts.TextWeight
	}
	if opts.VectorWeight > 0 {
		vecW = opts.VectorWeight
	}
	if len(ftsResults) == 0 && len(vecResults) > 0 {
		textW, vecW = 0, 1.0
	} else if len(vecResults) == 0 && len(ftsResults) > 0 {
		textW, vecW = 1.0, 0
	}
	merged := hybridMerge(ftsResults, vecResults, textW, vecW, userID)

	// Apply min score filter
	var filtered []store.MemorySearchResult
	for _, m := range merged {
		if opts.MinScore > 0 && m.Score < opts.MinScore {
			continue
		}
		if opts.PathPrefix != "" && len(m.Path) < len(opts.PathPrefix) {
			continue
		}
		filtered = append(filtered, m)
		if len(filtered) >= maxResults {
			break
		}
	}

	return filtered, nil
}

type scoredChunk struct {
	Path      string
	StartLine int
	EndLine   int
	Text      string
	Score     float64
	UserID    *string
}

func filterRAGScoredChunks(chunks []scoredChunk, querierID, ragGroupID, ragPersonalOwnerID string, sharedMemory bool) []scoredChunk {
	if len(chunks) == 0 {
		return chunks
	}
	out := make([]scoredChunk, 0, len(chunks))
	for _, c := range chunks {
		if rag.VisibleRAGMemoryChunk(c.Path, c.UserID, querierID, ragGroupID, ragPersonalOwnerID, sharedMemory) {
			out = append(out, c)
		}
	}
	return out
}

func (s *PGMemoryStore) ftsSearch(ctx context.Context, query string, agentID any, userID, ragGroupPrefix, ragPersonalOwnerID string, limit int) ([]scoredChunk, error) {
	var q string
	var args []any

	if store.IsSharedMemory(ctx) {
		// Shared: no user_id filter — search ALL chunks for agent
		tc, tcArgs, _, err := scopeClause(ctx, 4)
		if err != nil {
			return nil, err
		}
		limitN := 4 + len(tcArgs)
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND tsv @@ plainto_tsquery('simple', $3)%s
			ORDER BY score DESC LIMIT $%d`, tc, limitN)
		args = append([]any{query, agentID, query}, tcArgs...)
		args = append(args, limit)
	} else if userID != "" {
		var userSQL string
		var core []any
		tcStart := 5
		if ragGroupPrefix != "" {
			if ragPersonalOwnerID != "" {
				userSQL = `AND (
				user_id IS NULL OR user_id = $4
				OR position($5 in path) = 1
				OR (user_id = $6 AND path LIKE 'rag/dm/%')
			)`
				core = []any{query, agentID, query, userID, ragGroupPrefix, ragPersonalOwnerID}
				tcStart = 7
			} else {
				userSQL = `AND (
				user_id IS NULL OR user_id = $4
				OR position($5 in path) = 1
			)`
				core = []any{query, agentID, query, userID, ragGroupPrefix}
				tcStart = 6
			}
		} else {
			userSQL = `AND (user_id IS NULL OR user_id = $4)`
			core = []any{query, agentID, query, userID}
			tcStart = 5
		}
		tc, tcArgs, _, err := scopeClause(ctx, tcStart)
		if err != nil {
			return nil, err
		}
		limitN := tcStart + len(tcArgs)
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND tsv @@ plainto_tsquery('simple', $3)
			%s%s
			ORDER BY score DESC LIMIT $%d`, userSQL, tc, limitN)
		args = append(core, tcArgs...)
		args = append(args, limit)
	} else {
		var userSQL string
		var core []any
		tcStart := 4
		if ragGroupPrefix != "" {
			userSQL = `AND (
				user_id IS NULL
				OR position($4 in path) = 1
			)`
			core = []any{query, agentID, query, ragGroupPrefix}
			tcStart = 5
		} else {
			userSQL = `AND user_id IS NULL`
			core = []any{query, agentID, query}
			tcStart = 4
		}
		tc, tcArgs, _, err := scopeClause(ctx, tcStart)
		if err != nil {
			return nil, err
		}
		limitN := tcStart + len(tcArgs)
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND tsv @@ plainto_tsquery('simple', $3)
			%s%s
			ORDER BY score DESC LIMIT $%d`, userSQL, tc, limitN)
		args = append(core, tcArgs...)
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var r scoredChunk
		rows.Scan(&r.Path, &r.StartLine, &r.EndLine, &r.Text, &r.UserID, &r.Score)
		results = append(results, r)
	}
	return results, nil
}

func (s *PGMemoryStore) vectorSearch(ctx context.Context, embedding []float32, agentID any, userID, ragGroupPrefix, ragPersonalOwnerID string, limit int) ([]scoredChunk, error) {
	vecStr := vectorToString(embedding)

	var q string
	var args []any

	if store.IsSharedMemory(ctx) {
		// Shared: no user_id filter — search ALL chunks for agent
		tc, tcArgs, _, err := scopeClause(ctx, 3)
		if err != nil {
			return nil, err
		}
		orderN := 3 + len(tcArgs)
		limitN := orderN + 1
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				1 - (embedding <=> $1::vector) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND embedding IS NOT NULL%s
			ORDER BY embedding <=> $%d::vector LIMIT $%d`, tc, orderN, limitN)
		args = append([]any{vecStr, agentID}, tcArgs...)
		args = append(args, vecStr, limit)
	} else if userID != "" {
		var userSQL string
		var core []any
		tcStart := 4
		if ragGroupPrefix != "" {
			if ragPersonalOwnerID != "" {
				userSQL = `AND (
				user_id IS NULL OR user_id = $3
				OR position($4 in path) = 1
				OR (user_id = $5 AND path LIKE 'rag/dm/%')
			)`
				core = []any{vecStr, agentID, userID, ragGroupPrefix, ragPersonalOwnerID}
				tcStart = 6
			} else {
				userSQL = `AND (
				user_id IS NULL OR user_id = $3
				OR position($4 in path) = 1
			)`
				core = []any{vecStr, agentID, userID, ragGroupPrefix}
				tcStart = 5
			}
		} else {
			userSQL = `AND (user_id IS NULL OR user_id = $3)`
			core = []any{vecStr, agentID, userID}
			tcStart = 4
		}
		tc, tcArgs, _, err := scopeClause(ctx, tcStart)
		if err != nil {
			return nil, err
		}
		orderN := tcStart + len(tcArgs)
		limitN := orderN + 1
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				1 - (embedding <=> $1::vector) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND embedding IS NOT NULL
			%s%s
			ORDER BY embedding <=> $%d::vector LIMIT $%d`, userSQL, tc, orderN, limitN)
		args = append(core, tcArgs...)
		args = append(args, vecStr, limit)
	} else {
		var userSQL string
		var core []any
		tcStart := 3
		if ragGroupPrefix != "" {
			userSQL = `AND (
				user_id IS NULL
				OR position($3 in path) = 1
			)`
			core = []any{vecStr, agentID, ragGroupPrefix}
			tcStart = 4
		} else {
			userSQL = `AND user_id IS NULL`
			core = []any{vecStr, agentID}
			tcStart = 3
		}
		tc, tcArgs, _, err := scopeClause(ctx, tcStart)
		if err != nil {
			return nil, err
		}
		orderN := tcStart + len(tcArgs)
		limitN := orderN + 1
		q = fmt.Sprintf(`SELECT path, start_line, end_line, text, user_id,
				1 - (embedding <=> $1::vector) AS score
			FROM memory_chunks
			WHERE agent_id = $2 AND embedding IS NOT NULL
			%s%s
			ORDER BY embedding <=> $%d::vector LIMIT $%d`, userSQL, tc, orderN, limitN)
		args = append(core, tcArgs...)
		args = append(args, vecStr, limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var r scoredChunk
		rows.Scan(&r.Path, &r.StartLine, &r.EndLine, &r.Text, &r.UserID, &r.Score)
		results = append(results, r)
	}
	return results, nil
}

// hybridMerge combines FTS and vector results with weighted scoring.
// Per-user results get a 1.2x boost. Deduplication: user copy wins over global.
// NOTE: when shared memory is active, the 1.2x personal boost still applies —
// consider removing it in shared mode if all docs should be treated equally.
func hybridMerge(fts, vec []scoredChunk, textWeight, vectorWeight float64, currentUserID string) []store.MemorySearchResult {
	type key struct {
		Path      string
		StartLine int
	}
	seen := make(map[key]*store.MemorySearchResult)

	addResult := func(r scoredChunk, weight float64) {
		k := key{r.Path, r.StartLine}
		scope := "global"
		boost := 1.0
		if r.UserID != nil && *r.UserID != "" {
			scope = "personal"
			boost = 1.2
		}
		score := r.Score * weight * boost

		if existing, ok := seen[k]; ok {
			existing.Score += score
			// User copy wins
			if scope == "personal" {
				existing.Scope = "personal"
				existing.Snippet = r.Text
				if r.UserID != nil {
					existing.ChunkUserID = *r.UserID
				}
			}
		} else {
			chunkUID := ""
			if r.UserID != nil {
				chunkUID = *r.UserID
			}
			seen[k] = &store.MemorySearchResult{
				Path:        r.Path,
				StartLine:   r.StartLine,
				EndLine:     r.EndLine,
				Score:       score,
				Snippet:     r.Text,
				Source:      "memory",
				Scope:       scope,
				ChunkUserID: chunkUID,
			}
		}
	}

	for _, r := range fts {
		addResult(r, textWeight)
	}
	for _, r := range vec {
		addResult(r, vectorWeight)
	}

	// Collect and sort by score
	results := make([]store.MemorySearchResult, 0, len(seen))
	for _, r := range seen {
		results = append(results, *r)
	}

	// Simple sort (descending score)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

