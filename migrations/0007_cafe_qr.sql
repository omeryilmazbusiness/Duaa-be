-- Public QR tokens (one live token per site) and guest voice. Recovery stays a later vertical.
CREATE TABLE cafe_qr_codes (
    site_id       uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    token         text NOT NULL UNIQUE,
    issued_at     timestamptz NOT NULL,
    rotated_at    timestamptz
);

CREATE INDEX cafe_qr_codes_workspace_idx ON cafe_qr_codes (workspace_id);

CREATE TABLE cafe_voices (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    site_id        uuid NOT NULL,
    token          text NOT NULL,
    rating         smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment        text NOT NULL DEFAULT '',
    contact_name   text NOT NULL DEFAULT '',
    contact_phone  text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL
);

CREATE INDEX cafe_voices_workspace_created_idx ON cafe_voices (workspace_id, created_at DESC);
