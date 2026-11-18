-- Hotel OHIP (OPERA Cloud) connector config and idempotent PMS daily snapshots.
CREATE TABLE hotel_ohip_configs (
    workspace_id            uuid PRIMARY KEY REFERENCES workspaces (id),
    enabled                 boolean NOT NULL DEFAULT false,
    host_url                text NOT NULL DEFAULT '',
    hotel_id                text NOT NULL DEFAULT '',
    client_id               text NOT NULL DEFAULT '',
    encrypted_client_secret text NOT NULL DEFAULT '',
    secret_hint             text NOT NULL DEFAULT '',
    encrypted_app_key       text NOT NULL DEFAULT '',
    app_key_hint            text NOT NULL DEFAULT '',
    sync_mode               text NOT NULL DEFAULT 'opera_export' CHECK (sync_mode IN ('ohip_api', 'opera_export')),
    last_sync_at            timestamptz,
    last_sync_status        text NOT NULL DEFAULT 'never' CHECK (last_sync_status IN ('never', 'ok', 'error')),
    last_error              text NOT NULL DEFAULT '',
    updated_by              uuid NOT NULL REFERENCES users (id),
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL
);

CREATE TABLE hotel_pms_snapshots (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces (id),
    business_date       date NOT NULL,
    source              text NOT NULL CHECK (source IN ('ohip_live', 'opera_export', 'manual')),
    external_batch_ref  text NOT NULL DEFAULT '',
    currency            text NOT NULL DEFAULT 'USD',
    total_rooms         integer NOT NULL CHECK (total_rooms >= 0),
    occupied_rooms      integer NOT NULL CHECK (occupied_rooms >= 0),
    out_of_order_rooms  integer NOT NULL CHECK (out_of_order_rooms >= 0),
    room_revenue_minor  bigint NOT NULL DEFAULT 0,
    fb_revenue_minor    bigint NOT NULL DEFAULT 0,
    other_revenue_minor bigint NOT NULL DEFAULT 0,
    arrivals_count      integer NOT NULL DEFAULT 0,
    departures_count    integer NOT NULL DEFAULT 0,
    in_house_guests     integer NOT NULL DEFAULT 0,
    reservations_count  integer NOT NULL DEFAULT 0,
    cancelled_count     integer NOT NULL DEFAULT 0,
    no_show_count       integer NOT NULL DEFAULT 0,
    readiness           jsonb NOT NULL DEFAULT '[]',
    reservations        jsonb NOT NULL DEFAULT '[]',
    captured_at         timestamptz NOT NULL,
    recorded_by         uuid NOT NULL REFERENCES users (id),
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    UNIQUE (workspace_id, business_date)
);

CREATE INDEX hotel_pms_snapshots_ws_date_idx ON hotel_pms_snapshots (workspace_id, business_date DESC);
