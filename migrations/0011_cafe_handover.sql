-- Shift handover: one open file per cafe. Snapshot is frozen at open.
CREATE TABLE cafe_handovers (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    business_date  date NOT NULL,
    from_shift     text NOT NULL DEFAULT '',
    to_shift       text NOT NULL DEFAULT '',
    status         text NOT NULL,
    note           text NOT NULL DEFAULT '',
    tasks          jsonb NOT NULL DEFAULT '[]'::jsonb,
    recoveries     jsonb NOT NULL DEFAULT '[]'::jsonb,
    breakdowns     jsonb NOT NULL DEFAULT '[]'::jsonb,
    opened_by      uuid NOT NULL,
    accepted_by    uuid,
    opened_at      timestamptz NOT NULL,
    accepted_at    timestamptz,
    updated_at     timestamptz NOT NULL,
    CONSTRAINT cafe_handovers_status_chk CHECK (status IN ('open', 'accepted'))
);

CREATE UNIQUE INDEX cafe_handovers_one_open_idx ON cafe_handovers (workspace_id) WHERE status = 'open';
CREATE INDEX cafe_handovers_workspace_opened_idx ON cafe_handovers (workspace_id, opened_at DESC);
