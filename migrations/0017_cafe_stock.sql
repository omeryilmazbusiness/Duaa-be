-- Cafe stock catalog + adjustment ledger. Empty catalogs stay unavailable; on-hand is never invented.
CREATE TABLE cafe_stock_catalog (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id),
    items        jsonb NOT NULL DEFAULT '[]',
    updated_at   timestamptz NOT NULL,
    updated_by   uuid NOT NULL REFERENCES users (id)
);

CREATE TABLE stock_adjustments (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id),
    item_id        uuid NOT NULL,
    recorded_by    uuid NOT NULL REFERENCES users (id),
    business_date  date NOT NULL,
    delta          bigint NOT NULL,
    on_hand_after  bigint NOT NULL CHECK (on_hand_after >= 0),
    reason         text NOT NULL,
    note           text NOT NULL DEFAULT '',
    recorded_at    timestamptz NOT NULL
);

CREATE INDEX stock_adjustments_day_idx ON stock_adjustments (workspace_id, business_date);
