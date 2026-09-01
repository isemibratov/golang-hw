DROP TABLE IF EXISTS notifications;

DROP INDEX IF EXISTS events_pending_notification_idx;

ALTER TABLE events
    DROP COLUMN IF EXISTS notification_sent_at;
