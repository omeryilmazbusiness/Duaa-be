-- Manager daily close: shift, waste, incident. One file per cafe day. No invented fire cost.
CREATE TABLE cafe_intakes (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    business_date  date NOT NULL,
    shift_name     text NOT NULL DEFAULT '',
    close_note     text NOT NULL DEFAULT '',
    waste_note     text NOT NULL DEFAULT '',
    incident_note  text NOT NULL DEFAULT '',
    author_user_id uuid NOT NULL,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    UNIQUE (workspace_id, business_date)
);

CREATE INDEX cafe_intakes_workspace_date_idx ON cafe_intakes (workspace_id, business_date DESC);
