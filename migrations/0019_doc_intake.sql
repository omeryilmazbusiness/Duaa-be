-- Uploaded/ingested operational & financial documents with deterministic + AI simplified summaries.
CREATE TABLE workspace_documents (
    id                 uuid PRIMARY KEY,
    workspace_id       uuid NOT NULL REFERENCES workspaces (id),
    uploaded_by        uuid NOT NULL REFERENCES users (id),
    business_date      date NOT NULL,
    file_name          text NOT NULL,
    source_kind        text NOT NULL,
    content_sha256     text NOT NULL,
    raw_excerpt        text NOT NULL DEFAULT '',
    status             text NOT NULL CHECK (status IN ('analyzed', 'fallback_structured')),
    ai_used            boolean NOT NULL DEFAULT false,
    data_coverage_pct  integer NOT NULL DEFAULT 0 CHECK (data_coverage_pct BETWEEN 0 AND 100),
    simplified_summary jsonb NOT NULL,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    UNIQUE (workspace_id, business_date, content_sha256)
);

CREATE INDEX workspace_documents_ws_date_idx ON workspace_documents (workspace_id, business_date DESC, created_at DESC);
