CREATE TABLE rag_documents (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id         VARCHAR(255) NOT NULL,
    source_type     VARCHAR(20) NOT NULL,
    source_ref      TEXT NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    title           TEXT,
    mime_type       VARCHAR(100),
    byte_size       INT,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    meta            JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, agent_id, user_id, source_type, content_hash)
);

CREATE INDEX idx_rag_doc_owner ON rag_documents (tenant_id, agent_id, user_id);
CREATE INDEX idx_rag_doc_source ON rag_documents (tenant_id, source_type, source_ref);
CREATE INDEX idx_rag_doc_expires ON rag_documents (expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE rag_chunks (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES rag_documents(id) ON DELETE CASCADE,
    agent_id    UUID NOT NULL,
    user_id     VARCHAR(255) NOT NULL,
    chunk_index INT NOT NULL,
    content     TEXT NOT NULL,
    embedding   vector(1536),
    tsv         tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
    token_count INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX idx_rag_chunk_doc ON rag_chunks (document_id);
CREATE INDEX idx_rag_chunk_owner ON rag_chunks (tenant_id, agent_id, user_id);
CREATE INDEX idx_rag_chunk_tsv ON rag_chunks USING GIN (tsv);
CREATE INDEX idx_rag_chunk_vec ON rag_chunks USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
