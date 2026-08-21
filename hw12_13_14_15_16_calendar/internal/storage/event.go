// Package storage defines the calendar event model and storage-level business errors.
package storage

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrEventAlreadyExists means that an event with the same ID is already stored.
	ErrEventAlreadyExists = errors.New("event already exists")
	// ErrEventNotFound means that an event does not exist.
	ErrEventNotFound = errors.New("event not found")
	// ErrDateBusy means that an event overlaps another event owned by the same user.
	ErrDateBusy = errors.New("event date is busy")
	// ErrInvalidEvent means that an event or query range does not satisfy the domain rules.
	ErrInvalidEvent = errors.New("invalid event")
)

// Event is a calendar entry owned by one user.
type Event struct {
	ID           string
	Title        string
	StartAt      time.Time
	EndAt        time.Time
	Description  string
	UserID       string
	NotifyBefore time.Duration
}

// ValidateEvent checks the required event fields and interval boundaries.
func ValidateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" ||
		strings.TrimSpace(event.Title) == "" ||
		strings.TrimSpace(event.UserID) == "" ||
		event.StartAt.IsZero() ||
		event.EndAt.IsZero() ||
		!event.EndAt.After(event.StartAt) ||
		event.NotifyBefore < 0 {
		return ErrInvalidEvent
	}

	return nil
}
