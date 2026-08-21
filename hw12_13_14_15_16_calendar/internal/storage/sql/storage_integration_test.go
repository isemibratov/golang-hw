package sqlstorage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

func TestStorageIntegration(t *testing.T) {
	dsn := os.Getenv("CALENDAR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CALENDAR_TEST_POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	calendarStorage := New(dsn)
	if err := calendarStorage.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = calendarStorage.Close(context.Background())
	})

	start := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	first := storage.Event{
		ID:           "integration-event-1",
		Title:        "First event",
		StartAt:      start,
		EndAt:        start.Add(time.Hour),
		Description:  "integration test",
		UserID:       "integration-user",
		NotifyBefore: 15 * time.Minute,
	}
	second := first
	second.ID = "integration-event-2"
	second.Title = "Second event"
	second.StartAt = first.EndAt
	second.EndAt = second.StartAt.Add(time.Hour)

	cleanupIntegrationEvents(t, calendarStorage, first.ID, second.ID)
	t.Cleanup(func() {
		cleanupIntegrationEvents(t, calendarStorage, first.ID, second.ID)
	})

	if err := calendarStorage.CreateEvent(ctx, first); err != nil {
		t.Fatalf("CreateEvent(first) error = %v", err)
	}
	if err := calendarStorage.CreateEvent(ctx, first); !errors.Is(err, storage.ErrEventAlreadyExists) {
		t.Fatalf("duplicate CreateEvent() error = %v, want %v", err, storage.ErrEventAlreadyExists)
	}

	overlapping := second
	overlapping.StartAt = first.StartAt.Add(30 * time.Minute)
	if err := calendarStorage.CreateEvent(ctx, overlapping); !errors.Is(err, storage.ErrDateBusy) {
		t.Fatalf("overlapping CreateEvent() error = %v, want %v", err, storage.ErrDateBusy)
	}
	if err := calendarStorage.CreateEvent(ctx, second); err != nil {
		t.Fatalf("CreateEvent(second) error = %v", err)
	}

	events, err := calendarStorage.ListEvents(ctx, first.UserID, first.StartAt, second.EndAt)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 2 || events[0].ID != first.ID || events[1].ID != second.ID {
		t.Fatalf("ListEvents() = %#v, want events ordered by start time", events)
	}

	second.Title = "Updated event"
	if err := calendarStorage.UpdateEvent(ctx, second); err != nil {
		t.Fatalf("UpdateEvent() error = %v", err)
	}
	if err := calendarStorage.DeleteEvent(ctx, first.ID); err != nil {
		t.Fatalf("DeleteEvent() error = %v", err)
	}
	if err := calendarStorage.DeleteEvent(ctx, first.ID); !errors.Is(err, storage.ErrEventNotFound) {
		t.Fatalf("second DeleteEvent() error = %v, want %v", err, storage.ErrEventNotFound)
	}
}

func cleanupIntegrationEvents(t *testing.T, calendarStorage *Storage, ids ...string) {
	t.Helper()

	for _, id := range ids {
		err := calendarStorage.DeleteEvent(context.Background(), id)
		if err != nil && !errors.Is(err, storage.ErrEventNotFound) {
			t.Errorf("cleanup event %q: %v", id, err)
		}
	}
}
