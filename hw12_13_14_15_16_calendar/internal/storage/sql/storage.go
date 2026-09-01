// Package sqlstorage implements PostgreSQL-backed calendar storage.
package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/lib/pq"
)

// ErrNotConnected means that Connect has not completed successfully.
var ErrNotConnected = errors.New("storage is not connected")

// Storage persists events in PostgreSQL.
type Storage struct {
	dsn string

	mu sync.RWMutex
	db *sql.DB
}

// New creates a disconnected SQL storage for the supplied DSN.
func New(dsn string) *Storage {
	return &Storage{dsn: dsn}
}

// Connect opens and verifies the PostgreSQL connection.
func (s *Storage) Connect(ctx context.Context) error {
	if strings.TrimSpace(s.dsn) == "" {
		return fmt.Errorf("connect to database: empty DSN")
	}

	db, err := sql.Open("postgres", s.dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	s.mu.Lock()
	oldDB := s.db
	s.db = db
	s.mu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}

	return nil
}

// Close releases the PostgreSQL connection.
func (s *Storage) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	db := s.db
	s.db = nil
	s.mu.Unlock()

	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// CreateEvent inserts an event and maps database constraints to business errors.
func (s *Storage) CreateEvent(ctx context.Context, event storage.Event) error {
	if err := storage.ValidateEvent(event); err != nil {
		return err
	}

	db, err := s.database()
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO events (
			id, title, start_at, end_at, description, user_id, notify_before_ns
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = db.ExecContext(
		ctx,
		query,
		event.ID,
		event.Title,
		event.StartAt,
		event.EndAt,
		event.Description,
		event.UserID,
		int64(event.NotifyBefore),
	)
	if err != nil {
		return mapDatabaseError("create event", err)
	}
	return nil
}

// UpdateEvent replaces an existing event.
func (s *Storage) UpdateEvent(ctx context.Context, event storage.Event) error {
	if err := storage.ValidateEvent(event); err != nil {
		return err
	}

	db, err := s.database()
	if err != nil {
		return err
	}

	const query = `
		UPDATE events
		SET title = $2,
			start_at = $3,
			end_at = $4,
			description = $5,
			user_id = $6,
			notify_before_ns = $7,
			notification_sent_at = NULL
		WHERE id = $1`

	result, err := db.ExecContext(
		ctx,
		query,
		event.ID,
		event.Title,
		event.StartAt,
		event.EndAt,
		event.Description,
		event.UserID,
		int64(event.NotifyBefore),
	)
	if err != nil {
		return mapDatabaseError("update event", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update event: get affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return storage.ErrEventNotFound
	}
	return nil
}

// DeleteEvent removes an event by ID.
func (s *Storage) DeleteEvent(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: empty event ID", storage.ErrInvalidEvent)
	}

	db, err := s.database()
	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, "DELETE FROM events WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete event: get affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return storage.ErrEventNotFound
	}
	return nil
}

// ListEvents returns sorted user events that overlap the half-open interval [from, to).
func (s *Storage) ListEvents(
	ctx context.Context,
	userID string,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: empty user ID", storage.ErrInvalidEvent)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return nil, fmt.Errorf("%w: invalid time range", storage.ErrInvalidEvent)
	}

	db, err := s.database()
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT id, title, start_at, end_at, description, user_id, notify_before_ns
		FROM events
		WHERE user_id = $1 AND start_at < $3 AND end_at > $2
		ORDER BY start_at, id`

	rows, err := db.QueryContext(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows, "list events")
}

func (s *Storage) database() (*sql.DB, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, ErrNotConnected
	}
	return s.db, nil
}

func mapDatabaseError(operation string, err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23505":
			return fmt.Errorf("%s: %w", operation, storage.ErrEventAlreadyExists)
		case "23P01":
			return fmt.Errorf("%s: %w", operation, storage.ErrDateBusy)
		case "23514":
			return fmt.Errorf("%s: %w", operation, storage.ErrInvalidEvent)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
