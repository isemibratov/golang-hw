package sqlstorage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
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
	_, err = calendarStorage.ListEventsForNotification(context.Background(), event.StartAt, 10)
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("ListEventsForNotification() error = %v, want %v", err, ErrNotConnected)
	}
	_, err = calendarStorage.MarkNotificationSent(
		context.Background(),
		event,
		event.StartAt,
	)
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("MarkNotificationSent() error = %v, want %v", err, ErrNotConnected)
	}
	if err = calendarStorage.DeleteEventsBefore(
		context.Background(),
		event.StartAt,
	); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("DeleteEventsBefore() error = %v, want %v", err, ErrNotConnected)
	}
	value := notification.Notification{
		EventID: event.ID,
		Title:   event.Title,
		EventAt: event.StartAt,
		UserID:  event.UserID,
	}
	if err = calendarStorage.SaveNotification(
		context.Background(),
		value,
	); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("SaveNotification() error = %v, want %v", err, ErrNotConnected)
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

func TestSchedulerStorageValidatesInputBeforeQuery(t *testing.T) {
	calendarStorage := New("unused")
	now := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC)
	validEvent := storage.Event{
		ID:           "event-1",
		Title:        "Meeting",
		StartAt:      now.Add(time.Hour),
		EndAt:        now.Add(2 * time.Hour),
		UserID:       "user-1",
		NotifyBefore: 15 * time.Minute,
	}

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "zero notification time",
			call: func() error {
				_, err := calendarStorage.ListEventsForNotification(context.Background(), time.Time{}, 10)
				return err
			},
		},
		{
			name: "non-positive notification limit",
			call: func() error {
				_, err := calendarStorage.ListEventsForNotification(context.Background(), now, 0)
				return err
			},
		},
		{
			name: "empty event ID",
			call: func() error {
				event := validEvent
				event.ID = " "
				_, err := calendarStorage.MarkNotificationSent(context.Background(), event, now)
				return err
			},
		},
		{
			name: "zero sent time",
			call: func() error {
				_, err := calendarStorage.MarkNotificationSent(
					context.Background(),
					validEvent,
					time.Time{},
				)
				return err
			},
		},
		{
			name: "zero retention cutoff",
			call: func() error {
				return calendarStorage.DeleteEventsBefore(context.Background(), time.Time{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, storage.ErrInvalidEvent) {
				t.Fatalf("operation error = %v, want %v", err, storage.ErrInvalidEvent)
			}
		})
	}
}

func TestSaveNotificationValidatesInputBeforeQuery(t *testing.T) {
	calendarStorage := New("unused")

	err := calendarStorage.SaveNotification(context.Background(), notification.Notification{})
	if !errors.Is(err, notification.ErrInvalidNotification) {
		t.Fatalf("SaveNotification() error = %v, want %v", err, notification.ErrInvalidNotification)
	}
}
