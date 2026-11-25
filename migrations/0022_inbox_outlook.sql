-- GM Outlook inbox: sealed OAuth connection + cached messages with AI enrichment.
CREATE TABLE inbox_connections (
    workspace_id             uuid PRIMARY KEY REFERENCES workspaces (id),
    provider                 text NOT NULL CHECK (provider IN ('outlook', 'demo')),
    email                    text NOT NULL DEFAULT '',
    display_name             text NOT NULL DEFAULT '',
    encrypted_access_token   text NOT NULL DEFAULT '',
    encrypted_refresh_token  text NOT NULL DEFAULT '',
    token_expires_at         timestamptz,
    scopes                   text NOT NULL DEFAULT '',
    last_sync_at             timestamptz,
    last_sync_status         text NOT NULL DEFAULT 'never' CHECK (last_sync_status IN ('never', 'ok', 'error')),
    last_error               text NOT NULL DEFAULT '',
    connected_by             uuid NOT NULL REFERENCES users (id),
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL
);

CREATE TABLE inbox_messages (
    id                 uuid PRIMARY KEY,
    workspace_id       uuid NOT NULL REFERENCES workspaces (id),
    external_id        text NOT NULL,
    conversation_id    text NOT NULL DEFAULT '',
    folder             text NOT NULL DEFAULT 'inbox' CHECK (folder IN ('inbox', 'sent')),
    from_name          text NOT NULL DEFAULT '',
    from_email         text NOT NULL DEFAULT '',
    to_emails          text NOT NULL DEFAULT '',
    subject            text NOT NULL DEFAULT '',
    body_text          text NOT NULL DEFAULT '',
    preview            text NOT NULL DEFAULT '',
    received_at        timestamptz NOT NULL,
    is_read            boolean NOT NULL DEFAULT false,
    has_attachments    boolean NOT NULL DEFAULT false,
    category           text NOT NULL DEFAULT '' CHECK (category IN ('', 'guest', 'reservation', 'supplier', 'staff', 'finance', 'complaint', 'marketing', 'internal', 'other')),
    summary            text NOT NULL DEFAULT '',
    body_ar            text NOT NULL DEFAULT '',
    detected_language  text NOT NULL DEFAULT '',
    enriched_at        timestamptz,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    UNIQUE (workspace_id, external_id)
);

CREATE INDEX inbox_messages_ws_received_idx ON inbox_messages (workspace_id, received_at DESC);
CREATE INDEX inbox_messages_ws_category_idx ON inbox_messages (workspace_id, category);
