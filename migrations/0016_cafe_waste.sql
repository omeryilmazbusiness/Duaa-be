-- CAF-02 recorded recipe book + waste tickets. Empty days stay unavailable; cost is never invented from POS.
CREATE TABLE waste_tickets (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id),
    recorded_by    uuid NOT NULL REFERENCES users (id),
    business_date  date NOT NULL,
    station        text NOT NULL CHECK (station IN ('kitchen', 'bar')),
    item           text NOT NULL,
    recipe_id      uuid,
    grams          bigint NOT NULL CHECK (grams > 0),
    cost           bigint NOT NULL CHECK (cost > 0),
    reason         text NOT NULL,
    note           text NOT NULL DEFAULT '',
    recorded_at    timestamptz NOT NULL
);

CREATE INDEX waste_tickets_day_idx ON waste_tickets (workspace_id, business_date);

CREATE TABLE cafe_recipes (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id),
    items        jsonb NOT NULL DEFAULT '[]',
    updated_at   timestamptz NOT NULL,
    updated_by   uuid NOT NULL REFERENCES users (id)
);
