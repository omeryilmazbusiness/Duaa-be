-- CAF-03 recorded kitchen/bar prep clocks. Empty days stay unavailable; times are never invented.
CREATE TABLE prep_tickets (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id),
    opened_by      uuid NOT NULL REFERENCES users (id),
    ready_by       uuid REFERENCES users (id),
    served_by      uuid REFERENCES users (id),
    business_date  date NOT NULL,
    station        text NOT NULL CHECK (station IN ('kitchen', 'bar')),
    ticket_ref     text NOT NULL,
    table_label    text NOT NULL DEFAULT '',
    received_at    timestamptz NOT NULL,
    ready_at       timestamptz,
    served_at      timestamptz,
    UNIQUE (workspace_id, business_date, ticket_ref)
);

CREATE INDEX prep_tickets_day_idx ON prep_tickets (workspace_id, business_date);
