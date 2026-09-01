package sqlstorage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

// ListEventsForNotification returns pending events whose notification time has arrived.
func (s *Storage) ListEventsForNotification(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]storage.Event, error) {
	if now.IsZero() {
		return nil, fmt.Errorf("%w: notification time is zero", storage.ErrInvalidEvent)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: notification limit must be positive", storage.ErrInvalidEvent)
	}

	db, err := s.database()
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT id, title, start_at, end_at, description, user_id, notify_before_ns
		FROM events
		WHERE notify_before_ns > 0
			AND notification_sent_at IS NULL
			AND start_at > $1
			AND start_at <= $1 +
				(notify_before_ns::double precision / 1000000000) * INTERVAL '1 second'
		ORDER BY start_at, id
		LIMIT $2`

	rows, err := db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list events for notification: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows, "list events for notification")
}

// MarkNotificationSent records a broker acknowledgement if the event still matches the published snapshot.
// It returns false when the event changed, disappeared, or was marked concurrently.
func (s *Storage) MarkNotificationSent(
	ctx context.Context,
	event storage.Event,
	sentAt time.Time,
) (bool, error) {
	if err := storage.ValidateEvent(event); err != nil {
		return false, err
	}
	if sentAt.IsZero() {
		return false, fmt.Errorf("%w: notification sent time is zero", storage.ErrInvalidEvent)
	}

	db, err := s.database()
	if err != nil {
		return false, err
	}

	const query = `
		UPDATE events
		SET notification_sent_at = $6
		WHERE id = $1
			AND title = $2
			AND start_at = $3
			AND user_id = $4
			AND notify_before_ns = $5
			AND notification_sent_at IS NULL`

	result, err := db.ExecContext(
		ctx,
		query,
		event.ID,
		event.Title,
		event.StartAt,
		event.UserID,
		int64(event.NotifyBefore),
		sentAt,
	)
	if err != nil {
		return false, fmt.Errorf("mark notification sent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mark notification sent: get affected rows: %w", err)
	}

	return rowsAffected == 1, nil
}

// DeleteEventsBefore removes events that started before the supplied cutoff.
func (s *Storage) DeleteEventsBefore(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return fmt.Errorf("%w: event retention cutoff is zero", storage.ErrInvalidEvent)
	}

	db, err := s.database()
	if err != nil {
		return err
	}

	if _, err = db.ExecContext(ctx, "DELETE FROM events WHERE start_at < $1", cutoff); err != nil {
		return fmt.Errorf("delete old events: %w", err)
	}

	return nil
}

func scanEvents(rows *sql.Rows, operation string) ([]storage.Event, error) {
	events := make([]storage.Event, 0)
	for rows.Next() {
		var event storage.Event
		var notifyBefore int64
		if err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.StartAt,
			&event.EndAt,
			&event.Description,
			&event.UserID,
			&notifyBefore,
		); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", operation, err)
		}
		event.NotifyBefore = time.Duration(notifyBefore)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: read rows: %w", operation, err)
	}

	return events, nil
}
