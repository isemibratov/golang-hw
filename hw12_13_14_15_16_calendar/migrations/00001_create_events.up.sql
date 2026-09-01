CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE events (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL CHECK (btrim(user_id) <> ''),
    notify_before_ns BIGINT NOT NULL DEFAULT 0 CHECK (notify_before_ns >= 0),
    CONSTRAINT events_valid_period CHECK (start_at < end_at),
    CONSTRAINT events_no_overlap EXCLUDE USING gist (
        user_id WITH =,
        tstzrange(start_at, end_at, '[)') WITH &&
    )
);

CREATE INDEX events_user_start_at_idx ON events (user_id, start_at);
