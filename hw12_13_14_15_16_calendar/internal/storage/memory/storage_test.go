package memorystorage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

const updatedTitle = "updated"

var testStart = time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)

func TestValidateEvent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*storage.Event)
		valid  bool
	}{
		{name: "valid", valid: true},
		{name: "empty id", mutate: func(event *storage.Event) { event.ID = "" }},
		{name: "blank id", mutate: func(event *storage.Event) { event.ID = "  " }},
		{name: "empty title", mutate: func(event *storage.Event) { event.Title = "" }},
		{name: "blank title", mutate: func(event *storage.Event) { event.Title = "\t" }},
		{name: "empty user id", mutate: func(event *storage.Event) { event.UserID = "" }},
		{name: "zero start", mutate: func(event *storage.Event) { event.StartAt = time.Time{} }},
		{name: "zero end", mutate: func(event *storage.Event) { event.EndAt = time.Time{} }},
		{name: "equal boundaries", mutate: func(event *storage.Event) { event.EndAt = event.StartAt }},
		{name: "reversed boundaries", mutate: func(event *storage.Event) {
			event.EndAt = event.StartAt.Add(-time.Nanosecond)
		}},
		{name: "negative notification interval", mutate: func(event *storage.Event) {
			event.NotifyBefore = -time.Nanosecond
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newEvent("event-1", "user-1", testStart, testStart.Add(time.Hour))
			if tt.mutate != nil {
				tt.mutate(&event)
			}

			err := storage.ValidateEvent(event)
			if tt.valid {
				if err != nil {
					t.Fatalf("ValidateEvent() returned an unexpected error: %v", err)
				}
				return
			}
			assertErrorIs(t, err, storage.ErrInvalidEvent)
		})
	}
}

func TestStorageCreateEvent(t *testing.T) {
	ctx := context.Background()
	event := newEvent("event-1", "user-1", testStart, testStart.Add(time.Hour))
	calendar := New()

	if err := calendar.CreateEvent(ctx, event); err != nil {
		t.Fatalf("CreateEvent() returned an unexpected error: %v", err)
	}
	assertErrorIs(t, calendar.CreateEvent(ctx, event), storage.ErrEventAlreadyExists)

	invalid := event
	invalid.ID = ""
	assertErrorIs(t, calendar.CreateEvent(ctx, invalid), storage.ErrInvalidEvent)

	events, err := calendar.ListEvents(ctx, event.UserID, event.StartAt, event.EndAt)
	if err != nil {
		t.Fatalf("ListEvents() returned an unexpected error: %v", err)
	}
	if len(events) != 1 || events[0] != event {
		t.Fatalf("ListEvents() = %#v, want %#v", events, []storage.Event{event})
	}
}

func TestStorageCreateEventChecksHalfOpenIntervalsPerUser(t *testing.T) {
	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		userID    string
		wantError error
	}{
		{
			name:   "adjacent before",
			start:  testStart.Add(-time.Hour),
			end:    testStart,
			userID: "user-1",
		},
		{
			name:   "adjacent after",
			start:  testStart.Add(time.Hour),
			end:    testStart.Add(2 * time.Hour),
			userID: "user-1",
		},
		{
			name:      "overlaps start",
			start:     testStart.Add(-time.Minute),
			end:       testStart.Add(time.Minute),
			userID:    "user-1",
			wantError: storage.ErrDateBusy,
		},
		{
			name:      "inside existing",
			start:     testStart.Add(15 * time.Minute),
			end:       testStart.Add(45 * time.Minute),
			userID:    "user-1",
			wantError: storage.ErrDateBusy,
		},
		{
			name:      "contains existing",
			start:     testStart.Add(-time.Hour),
			end:       testStart.Add(2 * time.Hour),
			userID:    "user-1",
			wantError: storage.ErrDateBusy,
		},
		{
			name:      "overlaps end",
			start:     testStart.Add(59 * time.Minute),
			end:       testStart.Add(2 * time.Hour),
			userID:    "user-1",
			wantError: storage.ErrDateBusy,
		},
		{
			name:   "same slot for another user",
			start:  testStart,
			end:    testStart.Add(time.Hour),
			userID: "user-2",
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calendar := New()
			mustCreateEvent(t, calendar, newEvent("existing", "user-1", testStart, testStart.Add(time.Hour)))
			candidate := newEvent(fmt.Sprintf("candidate-%d", index), tt.userID, tt.start, tt.end)

			err := calendar.CreateEvent(context.Background(), candidate)
			if tt.wantError == nil {
				if err != nil {
					t.Fatalf("CreateEvent() returned an unexpected error: %v", err)
				}
				return
			}
			assertErrorIs(t, err, tt.wantError)
		})
	}
}

func TestStorageUpdateEvent(t *testing.T) {
	ctx := context.Background()
	calendar := New()
	first := newEvent("event-1", "user-1", testStart, testStart.Add(time.Hour))
	second := newEvent("event-2", "user-1", testStart.Add(2*time.Hour), testStart.Add(3*time.Hour))
	mustCreateEvent(t, calendar, first)
	mustCreateEvent(t, calendar, second)

	updated := first
	updated.Title = "updated title"
	updated.StartAt = testStart.Add(time.Hour)
	updated.EndAt = second.StartAt
	if err := calendar.UpdateEvent(ctx, updated); err != nil {
		t.Fatalf("UpdateEvent() returned an unexpected error: %v", err)
	}

	conflicting := updated
	conflicting.StartAt = second.StartAt.Add(30 * time.Minute)
	conflicting.EndAt = second.EndAt.Add(30 * time.Minute)
	assertErrorIs(t, calendar.UpdateEvent(ctx, conflicting), storage.ErrDateBusy)

	missing := newEvent("missing", "user-1", testStart.Add(4*time.Hour), testStart.Add(5*time.Hour))
	assertErrorIs(t, calendar.UpdateEvent(ctx, missing), storage.ErrEventNotFound)

	invalid := updated
	invalid.Title = ""
	assertErrorIs(t, calendar.UpdateEvent(ctx, invalid), storage.ErrInvalidEvent)

	events, err := calendar.ListEvents(ctx, "user-1", updated.StartAt, updated.EndAt)
	if err != nil {
		t.Fatalf("ListEvents() returned an unexpected error: %v", err)
	}
	if len(events) != 1 || events[0] != updated {
		t.Fatalf("event changed after rejected update: got %#v, want %#v", events, []storage.Event{updated})
	}
}

func TestStorageDeleteEvent(t *testing.T) {
	ctx := context.Background()
	calendar := New()
	event := newEvent("event-1", "user-1", testStart, testStart.Add(time.Hour))
	mustCreateEvent(t, calendar, event)

	assertErrorIs(t, calendar.DeleteEvent(ctx, " "), storage.ErrInvalidEvent)
	assertErrorIs(t, calendar.DeleteEvent(ctx, "missing"), storage.ErrEventNotFound)
	if err := calendar.DeleteEvent(ctx, event.ID); err != nil {
		t.Fatalf("DeleteEvent() returned an unexpected error: %v", err)
	}
	assertErrorIs(t, calendar.DeleteEvent(ctx, event.ID), storage.ErrEventNotFound)

	events, err := calendar.ListEvents(ctx, event.UserID, event.StartAt, event.EndAt)
	if err != nil {
		t.Fatalf("ListEvents() returned an unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("ListEvents() returned a deleted event: %#v", events)
	}
}

func TestStorageListEventsFiltersAndSorts(t *testing.T) {
	from := testStart
	to := testStart.Add(4 * time.Hour)
	events := []storage.Event{
		newEvent("event-b", "user-1", testStart.Add(time.Hour), testStart.Add(2*time.Hour)),
		newEvent("event-a", "user-1", testStart.Add(time.Hour), testStart.Add(2*time.Hour)),
		newEvent("overlaps-from", "user-1", testStart.Add(-time.Hour), testStart.Add(time.Minute)),
		newEvent("ends-at-from", "user-1", testStart.Add(-time.Hour), from),
		newEvent("starts-at-to", "user-1", to, to.Add(time.Hour)),
		newEvent("other-user", "user-2", testStart.Add(2*time.Hour), testStart.Add(3*time.Hour)),
	}
	calendar := &Storage{events: make(map[string]storage.Event, len(events))}
	for _, event := range events {
		calendar.events[event.ID] = event
	}

	got, err := calendar.ListEvents(context.Background(), "user-1", from, to)
	if err != nil {
		t.Fatalf("ListEvents() returned an unexpected error: %v", err)
	}
	wantIDs := []string{"overlaps-from", "event-a", "event-b"}
	if len(got) != len(wantIDs) {
		t.Fatalf("ListEvents() returned %d events (%#v), want %d", len(got), got, len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if got[i].ID != wantID {
			t.Fatalf("ListEvents()[%d].ID = %q, want %q; all events: %#v", i, got[i].ID, wantID, got)
		}
	}
}

func TestStorageListEventsValidatesRange(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		from   time.Time
		to     time.Time
	}{
		{name: "empty user", from: testStart, to: testStart.Add(time.Hour)},
		{name: "zero from", userID: "user-1", to: testStart.Add(time.Hour)},
		{name: "zero to", userID: "user-1", from: testStart},
		{name: "empty range", userID: "user-1", from: testStart, to: testStart},
		{name: "reversed range", userID: "user-1", from: testStart, to: testStart.Add(-time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := New().ListEvents(context.Background(), tt.userID, tt.from, tt.to)
			assertErrorIs(t, err, storage.ErrInvalidEvent)
			if events != nil {
				t.Fatalf("ListEvents() returned events for an invalid range: %#v", events)
			}
		})
	}
}

func TestStorageConcurrentAccess(t *testing.T) {
	const workers = 32

	ctx := context.Background()
	calendar := New()
	from := testStart.Add(-time.Hour)
	to := testStart.Add((workers*2 + 1) * time.Hour)
	errorsChannel := make(chan error, workers)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()

			start := testStart.Add(time.Duration(worker*2) * time.Hour)
			event := newEvent(fmt.Sprintf("event-%02d", worker), "user-1", start, start.Add(time.Hour))
			if err := calendar.CreateEvent(ctx, event); err != nil {
				errorsChannel <- fmt.Errorf("worker %d create: %w", worker, err)
				return
			}

			event.Title = updatedTitle
			if err := calendar.UpdateEvent(ctx, event); err != nil {
				errorsChannel <- fmt.Errorf("worker %d update: %w", worker, err)
				return
			}
			if _, err := calendar.ListEvents(ctx, "user-1", from, to); err != nil {
				errorsChannel <- fmt.Errorf("worker %d list: %w", worker, err)
				return
			}
			if worker%2 == 0 {
				if err := calendar.DeleteEvent(ctx, event.ID); err != nil {
					errorsChannel <- fmt.Errorf("worker %d delete: %w", worker, err)
				}
			}
		}()
	}

	wg.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Error(err)
	}

	events, err := calendar.ListEvents(ctx, "user-1", from, to)
	if err != nil {
		t.Fatalf("ListEvents() returned an unexpected error: %v", err)
	}
	if len(events) != workers/2 {
		t.Fatalf("ListEvents() returned %d events, want %d", len(events), workers/2)
	}
	for i, event := range events {
		if event.Title != updatedTitle {
			t.Fatalf("event %q was not updated: %#v", event.ID, event)
		}
		if i > 0 && !events[i-1].StartAt.Before(event.StartAt) {
			t.Fatalf("events are not sorted by start: %#v", events)
		}
	}
}

func TestStorageConcurrentCreateReservesSlotAtomically(t *testing.T) {
	const workers = 32

	calendar := New()
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := newEvent(fmt.Sprintf("event-%02d", worker), "user-1", testStart, testStart.Add(time.Hour))
			results <- calendar.CreateEvent(context.Background(), event)
		}()
	}
	wg.Wait()
	close(results)

	created := 0
	busy := 0
	for err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, storage.ErrDateBusy):
			busy++
		default:
			t.Fatalf("CreateEvent() returned an unexpected error: %v", err)
		}
	}
	if created != 1 || busy != workers-1 {
		t.Fatalf("concurrent CreateEvent() results: created=%d busy=%d, want created=1 busy=%d", created, busy, workers-1)
	}
}

func newEvent(id, userID string, start, end time.Time) storage.Event {
	return storage.Event{
		ID:           id,
		Title:        "event " + id,
		StartAt:      start,
		EndAt:        end,
		Description:  "description",
		UserID:       userID,
		NotifyBefore: 15 * time.Minute,
	}
}

func mustCreateEvent(t *testing.T, calendar *Storage, event storage.Event) {
	t.Helper()
	if err := calendar.CreateEvent(context.Background(), event); err != nil {
		t.Fatalf("CreateEvent(%q) returned an unexpected error: %v", event.ID, err)
	}
}

func assertErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want errors.Is(_, %v)", got, want)
	}
}
