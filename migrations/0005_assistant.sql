-- DUAA AI assistant: one config per workspace (key sealed with AES-GCM, never stored in clear)
-- and a per-day request counter used to enforce the daily limit atomically.
CREATE TABLE ai_settings (
    workspace_id        uuid PRIMARY KEY REFERENCES workspaces (id),
    provider            text NOT NULL CHECK (provider IN ('openai', 'gemini')),
    model               text NOT NULL,
    encrypted_key       text NOT NULL,
    key_hint            text NOT NULL DEFAULT '',
    daily_limit         integer NOT NULL CHECK (daily_limit BETWEEN 1 AND 500),
    enabled             boolean NOT NULL DEFAULT true,
    updated_by_user_id  uuid NOT NULL REFERENCES users (id),
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL
);

CREATE TABLE ai_usage (
    workspace_id  uuid NOT NULL REFERENCES workspaces (id),
    usage_date    date NOT NULL,
    requests      integer NOT NULL DEFAULT 0 CHECK (requests >= 0),
    PRIMARY KEY (workspace_id, usage_date)
);
