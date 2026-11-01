-- CAF-01 recorded POS tickets. Empty days stay unavailable; approve/close never invent a live zero.
CREATE TABLE pos_tickets (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id),
    recorded_by   uuid NOT NULL REFERENCES users (id),
    external_ref  text NOT NULL,
    business_date date NOT NULL,
    kind          text NOT NULL CHECK (kind IN ('sale', 'return')),
    lines         int NOT NULL CHECK (lines >= 1),
    gross         bigint NOT NULL CHECK (gross >= 0),
    discount      bigint NOT NULL CHECK (discount >= 0),
    net           bigint NOT NULL,
    tender        text NOT NULL CHECK (tender IN ('cash', 'card', 'other')),
    recorded_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, external_ref)
);

CREATE INDEX pos_tickets_day_idx ON pos_tickets (workspace_id, business_date);

CREATE TABLE pos_closes (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id),
    closed_by     uuid NOT NULL REFERENCES users (id),
    business_date date NOT NULL,
    ticket_count  int NOT NULL,
    sale_count    int NOT NULL,
    return_count  int NOT NULL,
    gross         bigint NOT NULL,
    discount      bigint NOT NULL,
    returns       bigint NOT NULL,
    net           bigint NOT NULL,
    basket        bigint NOT NULL,
    closed_at     timestamptz NOT NULL,
    UNIQUE (workspace_id, business_date)
);
