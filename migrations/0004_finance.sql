-- Manual till close (not POS) and spend-approval packs. Approving a pack does not move money.
CREATE TABLE cash_closes (
    id                 uuid PRIMARY KEY,
    workspace_id       uuid NOT NULL REFERENCES workspaces (id),
    business_date      date NOT NULL,
    counted_amount     bigint NOT NULL,
    expected_amount    bigint,
    variance           bigint NOT NULL,
    note               text NOT NULL DEFAULT '',
    closed_by_user_id  uuid NOT NULL REFERENCES users (id),
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    UNIQUE (workspace_id, business_date)
);

CREATE TABLE spend_approvals (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id),
    created_by_user_id   uuid NOT NULL REFERENCES users (id),
    title                text NOT NULL,
    supplier             text NOT NULL DEFAULT '',
    amount               bigint NOT NULL,
    currency             text NOT NULL,
    evidence             text NOT NULL DEFAULT '',
    status               text NOT NULL,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

CREATE INDEX spend_approvals_workspace_idx ON spend_approvals (workspace_id, status, created_at DESC);
