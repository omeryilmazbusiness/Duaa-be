-- Recovery file for unhappy guest voice and floor requests. Compensation is a record, never a payment.
CREATE TABLE cafe_recoveries (
    id                      uuid PRIMARY KEY,
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    voice_id                uuid NOT NULL UNIQUE REFERENCES cafe_voices (id) ON DELETE CASCADE,
    status                  text NOT NULL CHECK (status IN ('open', 'assigned', 'closed')),
    assignee_user_id        uuid,
    linked_task_id          uuid,
    compensation_amount     bigint NOT NULL DEFAULT 0,
    compensation_currency   text NOT NULL DEFAULT '',
    compensated_at          timestamptz,
    closed_at               timestamptz,
    note                    text NOT NULL DEFAULT '',
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL
);

CREATE INDEX cafe_recoveries_workspace_idx ON cafe_recoveries (workspace_id, created_at DESC);
