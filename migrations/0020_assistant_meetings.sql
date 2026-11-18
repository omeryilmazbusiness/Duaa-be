-- Meeting notes, translations and decision extractions (MEM-02 / AT-08).
CREATE TABLE assistant_meetings (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid NOT NULL REFERENCES workspaces (id),
    created_by            uuid NOT NULL REFERENCES users (id),
    business_date         date NOT NULL,
    title                 text NOT NULL,
    participants          jsonb NOT NULL DEFAULT '[]',
    transcript            text NOT NULL,
    translated_transcript text NOT NULL DEFAULT '',
    summary               text NOT NULL DEFAULT '',
    decisions             jsonb NOT NULL DEFAULT '[]',
    action_items          jsonb NOT NULL DEFAULT '[]',
    source_locale         text NOT NULL DEFAULT 'auto',
    target_locale         text NOT NULL DEFAULT 'ar',
    ai_used               boolean NOT NULL DEFAULT false,
    created_at            timestamptz NOT NULL
);

CREATE INDEX assistant_meetings_ws_idx ON assistant_meetings (workspace_id, created_at DESC);
