-- Cafe identity aggregate: one JSON document per workspace.
-- Branches, QR sites, shifts and SLA travel together; reviews read completeness from it.
CREATE TABLE cafe_definitions (
    workspace_id        uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    document            jsonb NOT NULL,
    updated_at          timestamptz NOT NULL,
    updated_by_user_id  uuid REFERENCES users (id)
);
