// Package memorystorage implements concurrent in-memory calendar storage.
package memorystorage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

// Storage keeps events in process memory.
type Storage struct {
	mu     sync.RWMutex
	events map[string]storage.Event
}

// New creates an empty in-memory storage.
func New() *Storage {
	return &Storage{events: make(map[string]storage.Event)}
}

// CreateEvent stores an event when its ID and time interval are available.
func (s *Storage) CreateEvent(ctx context.Context, event storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := storage.ValidateEvent(event); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := s.events[event.ID]; exists {
		return storage.ErrEventAlreadyExists
	}
	if s.dateBusy(event, "") {
		return storage.ErrDateBusy
	}
	if s.events == nil {
		s.events = make(map[string]storage.Event)
	}
	s.events[event.ID] = event

	return nil
}

// UpdateEvent replaces an existing event when the new time interval is available.
func (s *Storage) UpdateEvent(ctx context.Context, event storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := storage.ValidateEvent(event); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := s.events[event.ID]; !exists {
		return storage.ErrEventNotFound
	}
	if s.dateBusy(event, event.ID) {
		return storage.ErrDateBusy
	}
	s.events[event.ID] = event

	return nil
}

// DeleteEvent removes an event by ID.
func (s *Storage) DeleteEvent(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return storage.ErrInvalidEvent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := s.events[id]; !exists {
		return storage.ErrEventNotFound
	}
	delete(s.events, id)

	return nil
}

// ListEvents returns sorted user events that overlap the half-open interval [from, to).
func (s *Storage) ListEvents(
	ctx context.Context,
	userID string,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" || from.IsZero() || to.IsZero() || !to.After(from) {
		return nil, storage.ErrInvalidEvent
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]storage.Event, 0)
	for _, event := range s.events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if event.UserID == userID && intervalsOverlap(event.StartAt, event.EndAt, from, to) {
			events = append(events, event)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].StartAt.Equal(events[j].StartAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].StartAt.Before(events[j].StartAt)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// dateBusy must be called while s.mu is held for writing.
func (s *Storage) dateBusy(candidate storage.Event, excludedID string) bool {
	for id, event := range s.events {
		if id == excludedID || event.UserID != candidate.UserID {
			continue
		}
		if intervalsOverlap(event.StartAt, event.EndAt, candidate.StartAt, candidate.EndAt) {
			return true
		}
	}

	return false
}

func intervalsOverlap(firstStart, firstEnd, secondStart, secondEnd time.Time) bool {
	return firstStart.Before(secondEnd) && secondStart.Before(firstEnd)
}
