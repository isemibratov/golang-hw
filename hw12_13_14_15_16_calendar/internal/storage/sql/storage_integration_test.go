package sqlstorage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
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

func TestSchedulerStorageIntegration(t *testing.T) {
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

	// Keep the retention cutoff far outside realistic calendar data in case the
	// opt-in test is accidentally pointed at a non-empty development database.
	now := time.Date(1002, time.August, 24, 10, 0, 0, 0, time.UTC)
	fixture := newSchedulerIntegrationFixture(now)
	prepareSchedulerIntegrationEvents(t, calendarStorage, fixture)
	assertOptimisticNotificationMark(t, calendarStorage, fixture)
	assertSchedulerRetentionBoundary(t, calendarStorage, fixture)

	value := notification.Notification{
		EventID: fixture.due.ID,
		Title:   fixture.due.Title,
		EventAt: fixture.due.StartAt,
		UserID:  fixture.due.UserID,
	}
	if err := calendarStorage.SaveNotification(ctx, value); err != nil {
		t.Fatalf("SaveNotification() error = %v", err)
	}
	value.Title = "Upserted notification"
	if err := calendarStorage.SaveNotification(ctx, value); err != nil {
		t.Fatalf("SaveNotification() upsert error = %v", err)
	}
	assertIntegrationNotification(t, calendarStorage, value)
}

type schedulerIntegrationFixture struct {
	now               time.Time
	due               storage.Event
	notDue            storage.Event
	old               storage.Event
	startsNow         storage.Event
	retentionBoundary storage.Event
}

func newSchedulerIntegrationFixture(now time.Time) *schedulerIntegrationFixture {
	due := storage.Event{
		ID:           "integration-scheduler-due",
		Title:        "Due event",
		StartAt:      now.Add(5 * time.Minute),
		EndAt:        now.Add(35 * time.Minute),
		UserID:       "integration-scheduler-user",
		NotifyBefore: 10 * time.Minute,
	}
	notDue := due
	notDue.ID = "integration-scheduler-not-due"
	notDue.Title = "Not due event"
	notDue.StartAt = due.EndAt
	notDue.EndAt = notDue.StartAt.Add(30 * time.Minute)
	old := due
	old.ID = "integration-scheduler-old"
	old.Title = "Old event"
	old.StartAt = now.AddDate(-2, 0, 0)
	old.EndAt = old.StartAt.Add(30 * time.Minute)
	startsNow := due
	startsNow.ID = "integration-scheduler-starts-now"
	startsNow.Title = "Event starting now"
	startsNow.StartAt = now
	startsNow.EndAt = now.Add(30 * time.Minute)
	startsNow.UserID = "integration-scheduler-boundary-user"
	retentionBoundary := due
	retentionBoundary.ID = "integration-scheduler-retention-boundary"
	retentionBoundary.Title = "Retention boundary event"
	retentionBoundary.StartAt = now.AddDate(-1, 0, 0)
	retentionBoundary.EndAt = retentionBoundary.StartAt.Add(30 * time.Minute)

	return &schedulerIntegrationFixture{
		now:               now,
		due:               due,
		notDue:            notDue,
		old:               old,
		startsNow:         startsNow,
		retentionBoundary: retentionBoundary,
	}
}

func prepareSchedulerIntegrationEvents(
	t *testing.T,
	calendarStorage *Storage,
	fixture *schedulerIntegrationFixture,
) {
	t.Helper()

	events := []storage.Event{
		fixture.due,
		fixture.notDue,
		fixture.old,
		fixture.startsNow,
		fixture.retentionBoundary,
	}
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}

	cleanupIntegrationEvents(t, calendarStorage, eventIDs...)
	cleanupIntegrationNotifications(t, calendarStorage, fixture.due.ID)
	t.Cleanup(func() {
		cleanupIntegrationNotifications(t, calendarStorage, fixture.due.ID)
		cleanupIntegrationEvents(t, calendarStorage, eventIDs...)
	})

	for _, event := range events {
		if err := calendarStorage.CreateEvent(context.Background(), event); err != nil {
			t.Fatalf("CreateEvent(%q) error = %v", event.ID, err)
		}
	}
}

func assertOptimisticNotificationMark(
	t *testing.T,
	calendarStorage *Storage,
	fixture *schedulerIntegrationFixture,
) {
	t.Helper()

	ctx := context.Background()
	pending, err := calendarStorage.ListEventsForNotification(ctx, fixture.now, 10)
	if err != nil {
		t.Fatalf("ListEventsForNotification() error = %v", err)
	}
	if len(pending) != 1 || pending[0].ID != fixture.due.ID {
		t.Fatalf("ListEventsForNotification() = %#v, want only %q", pending, fixture.due.ID)
	}

	publishedSnapshot := pending[0]
	fixture.due.Title = "Concurrently updated due event"
	if err = calendarStorage.UpdateEvent(ctx, fixture.due); err != nil {
		t.Fatalf("UpdateEvent() during notification error = %v", err)
	}
	marked, err := calendarStorage.MarkNotificationSent(ctx, publishedSnapshot, fixture.now)
	if err != nil {
		t.Fatalf("MarkNotificationSent(stale snapshot) error = %v", err)
	}
	if marked {
		t.Fatal("MarkNotificationSent(stale snapshot) = true, want false")
	}
	pending, err = calendarStorage.ListEventsForNotification(ctx, fixture.now, 10)
	if err != nil {
		t.Fatalf("ListEventsForNotification() after concurrent update error = %v", err)
	}
	if len(pending) != 1 || pending[0].Title != fixture.due.Title {
		t.Fatalf("pending after concurrent update = %#v, want updated event", pending)
	}

	marked, err = calendarStorage.MarkNotificationSent(ctx, pending[0], fixture.now)
	if err != nil {
		t.Fatalf("MarkNotificationSent(current snapshot) error = %v", err)
	}
	if !marked {
		t.Fatal("MarkNotificationSent(current snapshot) = false, want true")
	}
	pending, err = calendarStorage.ListEventsForNotification(ctx, fixture.now, 10)
	if err != nil {
		t.Fatalf("ListEventsForNotification() after mark error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("ListEventsForNotification() after mark = %#v, want no events", pending)
	}

	fixture.due.Title = "Updated due event"
	if err = calendarStorage.UpdateEvent(ctx, fixture.due); err != nil {
		t.Fatalf("UpdateEvent() error = %v", err)
	}
	pending, err = calendarStorage.ListEventsForNotification(ctx, fixture.now, 1)
	if err != nil {
		t.Fatalf("ListEventsForNotification() after update error = %v", err)
	}
	if len(pending) != 1 || pending[0].Title != fixture.due.Title {
		t.Fatalf("ListEventsForNotification() after update = %#v, want updated event", pending)
	}
}

func assertSchedulerRetentionBoundary(
	t *testing.T,
	calendarStorage *Storage,
	fixture *schedulerIntegrationFixture,
) {
	t.Helper()

	ctx := context.Background()
	if err := calendarStorage.DeleteEventsBefore(ctx, fixture.now.AddDate(-1, 0, 0)); err != nil {
		t.Fatalf("DeleteEventsBefore() error = %v", err)
	}
	if err := calendarStorage.DeleteEvent(ctx, fixture.old.ID); !errors.Is(err, storage.ErrEventNotFound) {
		t.Fatalf("DeleteEvent(old) error = %v, want %v", err, storage.ErrEventNotFound)
	}
	if err := calendarStorage.DeleteEvent(ctx, fixture.retentionBoundary.ID); err != nil {
		t.Fatalf("DeleteEvent(retention boundary) error = %v, want event to remain", err)
	}
}

func cleanupIntegrationNotifications(t *testing.T, calendarStorage *Storage, eventIDs ...string) {
	t.Helper()

	db, err := calendarStorage.database()
	if err != nil {
		t.Errorf("access database for notification cleanup: %v", err)
		return
	}
	for _, eventID := range eventIDs {
		if _, err = db.ExecContext(
			context.Background(),
			"DELETE FROM notifications WHERE event_id = $1",
			eventID,
		); err != nil {
			t.Errorf("cleanup notification %q: %v", eventID, err)
		}
	}
}

func assertIntegrationNotification(
	t *testing.T,
	calendarStorage *Storage,
	want notification.Notification,
) {
	t.Helper()

	db, err := calendarStorage.database()
	if err != nil {
		t.Fatalf("access database for notification assertion: %v", err)
	}
	var got notification.Notification
	err = db.QueryRowContext(
		context.Background(),
		"SELECT event_id, title, event_at, user_id FROM notifications WHERE event_id = $1",
		want.EventID,
	).Scan(&got.EventID, &got.Title, &got.EventAt, &got.UserID)
	if err != nil {
		t.Fatalf("read notification: %v", err)
	}
	if got.EventID != want.EventID ||
		got.Title != want.Title ||
		!got.EventAt.Equal(want.EventAt) ||
		got.UserID != want.UserID {
		t.Fatalf("stored notification = %#v, want %#v", got, want)
	}
}
