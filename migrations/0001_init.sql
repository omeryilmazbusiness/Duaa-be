-- Executive OS kernel. Every business row carries workspace_id (Duaa spec ARC-03).
-- Cafe is the first vertical; hotel/supply reuse the same identity and work tables.

CREATE TABLE organizations (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL
);

CREATE TABLE workspaces (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    kind             text NOT NULL CHECK (kind IN ('cafe', 'hotel', 'supply')),
    name             text NOT NULL,
    slug             text NOT NULL UNIQUE,
    timezone         text NOT NULL DEFAULT 'UTC',
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL
);

CREATE INDEX workspaces_org_idx ON workspaces (organization_id);

CREATE TABLE users (
    id             uuid PRIMARY KEY,
    email          text UNIQUE,
    password_hash  text,
    first_name     text NOT NULL,
    last_name      text NOT NULL,
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL
);

CREATE TABLE memberships (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id),
    user_id          uuid NOT NULL REFERENCES users (id),
    role             text NOT NULL CHECK (role IN ('manager', 'personnel')),
    department       text,
    employee_number  text,
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    UNIQUE (workspace_id, user_id),
    UNIQUE (workspace_id, employee_number)
);

CREATE INDEX memberships_workspace_idx ON memberships (workspace_id);
CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE routine_tasks (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id),
    created_by_user_id   uuid NOT NULL REFERENCES users (id),
    assignee_user_id     uuid NOT NULL REFERENCES users (id),
    title                text NOT NULL,
    description          text,
    start_time           text NOT NULL,
    end_time             text NOT NULL,
    is_active            boolean NOT NULL DEFAULT true,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

CREATE INDEX routine_tasks_workspace_idx ON routine_tasks (workspace_id);

CREATE TABLE staff_tasks (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id),
    assignee_user_id     uuid NOT NULL REFERENCES users (id),
    created_by_user_id   uuid NOT NULL REFERENCES users (id),
    title                text NOT NULL,
    description          text,
    scheduled_start_at   timestamptz NOT NULL,
    scheduled_end_at     timestamptz NOT NULL,
    status               text NOT NULL DEFAULT 'pending',
    routine_id           uuid REFERENCES routine_tasks (id) ON DELETE SET NULL,
    source_date          date,
    completed_at         timestamptz,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

CREATE INDEX staff_tasks_workspace_start_idx ON staff_tasks (workspace_id, scheduled_start_at);
CREATE INDEX staff_tasks_assignee_idx ON staff_tasks (workspace_id, assignee_user_id);

CREATE TABLE audit_events (
    id              uuid PRIMARY KEY,
    workspace_id    uuid REFERENCES workspaces (id),
    actor_user_id   uuid REFERENCES users (id),
    action          text NOT NULL,
    entity_type     text NOT NULL,
    entity_id       uuid,
    created_at      timestamptz NOT NULL
);
