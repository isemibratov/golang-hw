package sqlstorage

import (
	"context"
	"fmt"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
)

// SaveNotification persists a notification, replacing stale data for the same event.
func (s *Storage) SaveNotification(ctx context.Context, value notification.Notification) error {
	if err := value.Validate(); err != nil {
		return err
	}

	db, err := s.database()
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO notifications (event_id, title, event_at, user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id) DO UPDATE
		SET title = EXCLUDED.title,
			event_at = EXCLUDED.event_at,
			user_id = EXCLUDED.user_id`

	if _, err = db.ExecContext(
		ctx,
		query,
		value.EventID,
		value.Title,
		value.EventAt,
		value.UserID,
	); err != nil {
		return fmt.Errorf("save notification: %w", err)
	}

	return nil
}
