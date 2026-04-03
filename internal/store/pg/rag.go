package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type PGRAGStore struct {
	db *sql.DB
}

func NewPGRAGStore(db *sql.DB) *PGRAGStore {
	return &PGRAGStore{db: db}
}

func (s *PGRAGStore) UpsertDocument(ctx context.Context, doc store.RAGDocument) (string, bool, error) {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return "", false, fmt.Errorf("agent_id required in context")
	}
	userID := store.UserIDFromContext(ctx)
	if userID == "" {
		return "", false, fmt.Errorf("user_id required in context")
	}
	tid := tenantIDForInsert(ctx)
	id := uuid.Must(uuid.NewV7())

	var rawMeta any = []byte("{}")
	if len(doc.Meta) > 0 {
		rawMeta = doc.Meta
	}

	var returnedID uuid.UUID
	var inserted bool
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO rag_documents (
			id, tenant_id, agent_id, user_id, source_type, source_ref, content_hash,
			title, mime_type, byte_size, fetched_at, expires_at, meta
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, COALESCE($11, NOW()), $12, $13
		)
		ON CONFLICT (tenant_id, agent_id, user_id, source_type, content_hash)
		DO UPDATE SET
			source_ref = EXCLUDED.source_ref,
			title = EXCLUDED.title,
			mime_type = EXCLUDED.mime_type,
			byte_size = EXCLUDED.byte_size,
			fetched_at = EXCLUDED.fetched_at,
			expires_at = EXCLUDED.expires_at,
			meta = EXCLUDED.meta
		RETURNING id, (xmax = 0)
	`, id, tid, agentID, userID, doc.SourceType, doc.SourceRef, doc.ContentHash, nilStr(doc.Title), nilStr(doc.MimeType), nilInt(doc.ByteSize), nilTime(&doc.FetchedAt), nilTime(doc.ExpiresAt), rawMeta).Scan(&returnedID, &inserted)
	if err != nil {
		return "", false, err
	}
	return returnedID.String(), inserted, nil
}

func (s *PGRAGStore) InsertChunks(ctx context.Context, chunks []store.RAGChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return fmt.Errorf("agent_id required in context")
	}
	userID := store.UserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user_id required in context")
	}
	tid := tenantIDForInsert(ctx)

	const batchSize = 100
	for start := 0; start < len(chunks); start += batchSize {
		end := min(start+batchSize, len(chunks))
		batch := chunks[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO rag_chunks (id, tenant_id, document_id, agent_id, user_id, chunk_index, content, embedding, token_count) VALUES `)
		args := make([]any, 0, len(batch)*9)
		for i, ch := range batch {
			if i > 0 {
				sb.WriteByte(',')
			}
			base := i * 9
			fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d::vector,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9)

			chID := ch.ID
			if chID == "" {
				chID = uuid.Must(uuid.NewV7()).String()
			}
			args = append(args,
				chID,
				tid,
				mustParseUUID(ch.DocumentID),
				agentID,
				userID,
				ch.ChunkIndex,
				ch.Content,
				vectorToString(ch.Embedding),
				nilInt(ch.TokenCount),
			)
		}
		sb.WriteString(` ON CONFLICT (document_id, chunk_index) DO NOTHING`)
		if _, err := s.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *PGRAGStore) Search(ctx context.Context, query store.RAGQuery) ([]store.RAGResult, error) {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return nil, fmt.Errorf("agent_id required in context")
	}
	userID := store.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user_id required in context")
	}
	scope, err := store.ScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	topK := query.TopK
	if topK <= 0 {
		topK = 5
	}
	minScore := query.MinScore
	excludeExpired := true
	if !query.ExcludeExpired {
		excludeExpired = false
	}
	if len(query.Embedding) == 0 {
		return nil, fmt.Errorf("query embedding required")
	}

	filterBySource := len(query.SourceTypes) > 0
	sources := pq.Array(query.SourceTypes)
	vec := vectorToString(query.Embedding)

	sqlText := `
WITH vec_ranked AS (
	SELECT c.id, c.document_id, c.content,
		ROW_NUMBER() OVER (ORDER BY c.embedding <=> $1::vector) AS rank
	FROM rag_chunks c
	JOIN rag_documents d ON d.id = c.document_id
	WHERE c.tenant_id = $2 AND c.agent_id = $3 AND c.user_id = $4
		AND c.embedding IS NOT NULL
		AND ($5::boolean IS FALSE OR d.expires_at IS NULL OR d.expires_at > NOW())
		AND ($6::boolean IS FALSE OR d.source_type = ANY($7::text[]))
	ORDER BY c.embedding <=> $1::vector
	LIMIT 60
),
fts_ranked AS (
	SELECT c.id, c.document_id, c.content,
		ROW_NUMBER() OVER (ORDER BY ts_rank_cd(c.tsv, query) DESC) AS rank
	FROM rag_chunks c
	JOIN rag_documents d ON d.id = c.document_id,
		plainto_tsquery('simple', $8) query
	WHERE c.tenant_id = $2 AND c.agent_id = $3 AND c.user_id = $4
		AND c.tsv @@ query
		AND ($5::boolean IS FALSE OR d.expires_at IS NULL OR d.expires_at > NOW())
		AND ($6::boolean IS FALSE OR d.source_type = ANY($7::text[]))
	ORDER BY ts_rank_cd(c.tsv, query) DESC
	LIMIT 60
),
rrf AS (
	SELECT COALESCE(v.id, f.id) AS id,
		COALESCE(v.content, f.content) AS content,
		COALESCE(v.document_id, f.document_id) AS document_id,
		(COALESCE(1.0/(60+v.rank), 0) + COALESCE(1.0/(60+f.rank), 0)) AS score
	FROM vec_ranked v
	FULL OUTER JOIN fts_ranked f USING (id)
)
SELECT r.id::text, r.document_id::text, r.content, r.score, d.source_type, d.source_ref,
	COALESCE(d.title, ''), d.fetched_at, d.expires_at
FROM rrf r
JOIN rag_documents d ON d.id = r.document_id
WHERE r.score >= $9
ORDER BY r.score DESC
LIMIT $10
`

	rows, err := s.db.QueryContext(ctx, sqlText, vec, scope.TenantID, agentID, userID, excludeExpired, filterBySource, sources, query.Text, minScore, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.RAGResult, 0, topK)
	for rows.Next() {
		var r store.RAGResult
		var fetched time.Time
		var expires sql.NullTime
		if err := rows.Scan(&r.ID, &r.DocumentID, &r.Content, &r.Score, &r.SourceType, &r.SourceRef, &r.Title, &fetched, &expires); err != nil {
			return nil, err
		}
		r.FetchedAt = fetched
		if expires.Valid {
			t := expires.Time
			r.ExpiresAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGRAGStore) PurgeExpired(ctx context.Context) (int64, error) {
	tc, tcArgs, _, err := scopeClauseAlias(ctx, 1, "d")
	if err != nil {
		return 0, err
	}
	q := "DELETE FROM rag_documents d WHERE d.expires_at IS NOT NULL AND d.expires_at < NOW()" + tc
	res, err := s.db.ExecContext(ctx, q, tcArgs...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *PGRAGStore) DeleteDocument(ctx context.Context, agentID, ownerID, docID string) error {
	_ = agentID
	_ = ownerID
	aid := store.AgentIDFromContext(ctx)
	if aid == uuid.Nil {
		return fmt.Errorf("agent_id required in context")
	}
	uid := store.UserIDFromContext(ctx)
	if uid == "" {
		return fmt.Errorf("user_id required in context")
	}

	allowAdmin := store.IsOwnerRole(ctx)
	tc, tcArgs, _, err := scopeClause(ctx, 4)
	if err != nil {
		return err
	}
	baseArgs := []any{mustParseUUID(docID), aid}
	if allowAdmin {
		res, err := s.db.ExecContext(ctx,
			"DELETE FROM rag_documents WHERE id = $1 AND agent_id = $2"+tc,
			append(baseArgs, tcArgs...)...,
		)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return store.ErrNotFound
		}
		return nil
	}

	res, err := s.db.ExecContext(ctx,
		"DELETE FROM rag_documents WHERE id = $1 AND agent_id = $2 AND user_id = $3"+tc,
		append(baseArgs, append([]any{uid}, tcArgs...)...)...,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PGRAGStore) LookupCache(ctx context.Context, sourceType, sourceRef, contentHash string) (string, bool, error) {
	aid := store.AgentIDFromContext(ctx)
	if aid == uuid.Nil {
		return "", false, fmt.Errorf("agent_id required in context")
	}
	uid := store.UserIDFromContext(ctx)
	if uid == "" {
		return "", false, fmt.Errorf("user_id required in context")
	}
	tc, tcArgs, _, err := scopeClause(ctx, 5)
	if err != nil {
		return "", false, err
	}

	args := append([]any{aid, uid, sourceType, sourceRef, contentHash}, tcArgs...)
	var id uuid.UUID
	err = s.db.QueryRowContext(ctx, `
		SELECT id FROM rag_documents
		WHERE agent_id = $1 AND user_id = $2
			AND source_type = $3 AND source_ref = $4 AND content_hash = $5
			AND (expires_at IS NULL OR expires_at > NOW())`+tc+`
		ORDER BY created_at DESC
		LIMIT 1
	`, args...).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return id.String(), true, nil
}

func (s *PGRAGStore) ListDocuments(ctx context.Context, agentID, ownerID, sourceType string, limit int) ([]store.RAGDocumentInfo, error) {
	_ = agentID
	_ = ownerID

	aid := store.AgentIDFromContext(ctx)
	if aid == uuid.Nil {
		return nil, fmt.Errorf("agent_id required in context")
	}
	uid := store.UserIDFromContext(ctx)
	if uid == "" {
		return nil, fmt.Errorf("user_id required in context")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	tc, tcArgs, _, err := scopeClauseAlias(ctx, 5, "d")
	if err != nil {
		return nil, err
	}

	args := []any{aid, uid, sourceType == "", sourceType}
	args = append(args, tcArgs...)
	args = append(args, limit)
	limitParam := 5 + len(tcArgs)
	q := fmt.Sprintf(`
		SELECT
			d.id::text,
			d.source_type,
			d.source_ref,
			COALESCE(d.title, ''),
			COALESCE(d.mime_type, ''),
			COALESCE(d.byte_size, 0),
			COUNT(c.id) AS chunk_count,
			d.fetched_at,
			d.expires_at
		FROM rag_documents d
		LEFT JOIN rag_chunks c ON c.document_id = d.id
		WHERE d.agent_id = $1
			AND d.user_id = $2
			AND ($3::boolean OR d.source_type = $4)%s
		GROUP BY d.id
		ORDER BY d.fetched_at DESC
		LIMIT $%d
	`, tc, limitParam)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.RAGDocumentInfo, 0, limit)
	for rows.Next() {
		var d store.RAGDocumentInfo
		var expires sql.NullTime
		if err := rows.Scan(&d.ID, &d.SourceType, &d.SourceRef, &d.Title, &d.MimeType, &d.ByteSize, &d.ChunkCount, &d.FetchedAt, &expires); err != nil {
			return nil, err
		}
		if expires.Valid {
			t := expires.Time
			d.ExpiresAt = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGRAGStore) ReindexDocument(ctx context.Context, docID string) error {
	// v1: store-level reindex is not implemented; ingestion pipeline owns re-fetch/re-chunk.
	// Keep method to satisfy store.RAGStore interface and WS method stub.
	return fmt.Errorf("reindex not implemented")
}

