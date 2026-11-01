-- Floor reports from staff. GM/floor turn them into tasks or close them.
CREATE TABLE staff_reports (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces (id),
    reporter_user_id    uuid NOT NULL REFERENCES users (id),
    kind                text NOT NULL,
    title               text NOT NULL,
    detail              text NOT NULL DEFAULT '',
    status              text NOT NULL,
    linked_task_id      uuid REFERENCES staff_tasks (id) ON DELETE SET NULL,
    assignee_user_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL
);

CREATE INDEX staff_reports_workspace_idx ON staff_reports (workspace_id, status, created_at DESC);
