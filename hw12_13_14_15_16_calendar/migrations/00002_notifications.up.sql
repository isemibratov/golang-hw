ALTER TABLE events
    ADD COLUMN notification_sent_at TIMESTAMPTZ;

CREATE INDEX events_pending_notification_idx
    ON events (start_at, id)
    WHERE notify_before_ns > 0 AND notification_sent_at IS NULL;

CREATE TABLE notifications (
    event_id TEXT PRIMARY KEY,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    event_at TIMESTAMPTZ NOT NULL,
    user_id TEXT NOT NULL CHECK (btrim(user_id) <> '')
);

CREATE INDEX notifications_user_event_at_idx
    ON notifications (user_id, event_at);
