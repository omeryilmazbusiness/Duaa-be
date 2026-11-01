-- GM morning-OS write model. Brief itself is composed live and is not stored.
CREATE TABLE gm_decisions (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id),
    created_by_user_id   uuid NOT NULL REFERENCES users (id),
    source_key           text NOT NULL,
    fingerprint          text NOT NULL DEFAULT '',
    kind                 text NOT NULL,
    title_key            text NOT NULL DEFAULT '',
    detail               text NOT NULL DEFAULT '',
    severity             text NOT NULL DEFAULT '',
    target_tab           text NOT NULL DEFAULT '',
    status               text NOT NULL,
    linked_task_id       uuid REFERENCES staff_tasks (id) ON DELETE SET NULL,
    assignee_user_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    snooze_until         timestamptz,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

CREATE INDEX gm_decisions_workspace_idx ON gm_decisions (workspace_id, status);

CREATE UNIQUE INDEX gm_decisions_active_source_idx
    ON gm_decisions (workspace_id, source_key)
    WHERE status IN ('open', 'acknowledged', 'assigned');
