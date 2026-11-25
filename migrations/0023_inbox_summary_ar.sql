-- Brief Arabic AI summary + optional per-workspace Outlook (Azure) app credentials.
ALTER TABLE inbox_messages
    ADD COLUMN IF NOT EXISTS summary_ar text NOT NULL DEFAULT '';

ALTER TABLE inbox_connections
    ADD COLUMN IF NOT EXISTS client_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS encrypted_client_secret text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tenant text NOT NULL DEFAULT '';
