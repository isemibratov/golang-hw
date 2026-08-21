package sqlstorage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

func TestStorageRequiresConnection(t *testing.T) {
	calendarStorage := New("postgres://calendar:calendar@localhost/calendar?sslmode=disable")
	event := storage.Event{
		ID:      "event-1",
		Title:   "Meeting",
		StartAt: time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, time.August, 21, 11, 0, 0, 0, time.UTC),
		UserID:  "user-1",
	}

	if err := calendarStorage.CreateEvent(context.Background(), event); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("CreateEvent() error = %v, want %v", err, ErrNotConnected)
	}
	if err := calendarStorage.UpdateEvent(context.Background(), event); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("UpdateEvent() error = %v, want %v", err, ErrNotConnected)
	}
	if err := calendarStorage.DeleteEvent(context.Background(), event.ID); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("DeleteEvent() error = %v, want %v", err, ErrNotConnected)
	}
	_, err := calendarStorage.ListEvents(
		context.Background(),
		event.UserID,
		event.StartAt,
		event.EndAt,
	)
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("ListEvents() error = %v, want %v", err, ErrNotConnected)
	}
}

func TestStorageValidatesInputBeforeQuery(t *testing.T) {
	calendarStorage := New("unused")

	err := calendarStorage.CreateEvent(context.Background(), storage.Event{})
	if !errors.Is(err, storage.ErrInvalidEvent) {
		t.Fatalf("CreateEvent() error = %v, want %v", err, storage.ErrInvalidEvent)
	}
	if err := calendarStorage.DeleteEvent(context.Background(), ""); !errors.Is(err, storage.ErrInvalidEvent) {
		t.Fatalf("DeleteEvent() error = %v, want %v", err, storage.ErrInvalidEvent)
	}
	_, err = calendarStorage.ListEvents(context.Background(), "user-1", time.Time{}, time.Now())
	if !errors.Is(err, storage.ErrInvalidEvent) {
		t.Fatalf("ListEvents() error = %v, want %v", err, storage.ErrInvalidEvent)
	}
	if err := calendarStorage.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
