-- CAF-04 light: recorded opening/closing lines. Empty list is valid. Maintenance photos later.
CREATE TABLE cafe_checklist_templates (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    items        jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at   timestamptz NOT NULL,
    updated_by   uuid NOT NULL
);

CREATE TABLE cafe_checklist_runs (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    business_date date NOT NULL,
    kind          text NOT NULL,
    marks         jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at    timestamptz NOT NULL,
    updated_by    uuid NOT NULL,
    CONSTRAINT cafe_checklist_runs_kind_chk CHECK (kind IN ('open', 'close')),
    UNIQUE (workspace_id, business_date, kind)
);

CREATE INDEX cafe_checklist_runs_workspace_idx ON cafe_checklist_runs (workspace_id, business_date DESC);
