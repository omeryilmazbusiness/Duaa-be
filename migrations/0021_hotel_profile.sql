-- Executive-editable hotel identity (name, rooms, currency, contact) per hotel workspace.
CREATE TABLE hotel_profiles (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id),
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    total_rooms  integer NOT NULL DEFAULT 0 CHECK (total_rooms BETWEEN 0 AND 10000),
    currency     text NOT NULL DEFAULT 'USD' CHECK (currency ~ '^[A-Z]{3}$'),
    timezone     text NOT NULL DEFAULT '',
    star_rating  smallint NOT NULL DEFAULT 0 CHECK (star_rating BETWEEN 0 AND 5),
    city         text NOT NULL DEFAULT '',
    address      text NOT NULL DEFAULT '',
    phone        text NOT NULL DEFAULT '',
    gm_name      text NOT NULL DEFAULT '',
    notes        text NOT NULL DEFAULT '',
    updated_by   uuid NOT NULL REFERENCES users (id),
    updated_at   timestamptz NOT NULL
);
