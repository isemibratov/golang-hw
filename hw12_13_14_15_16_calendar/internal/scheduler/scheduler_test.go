package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

type storageStub struct {
	batches       [][]storage.Event
	deleteErr     error
	listErr       error
	markErr       error
	markResults   []bool
	cutoff        time.Time
	listedAt      []time.Time
	listedLimits  []int
	markedEventID []string
	markedEvents  []storage.Event
	markedAt      []time.Time
	listCalls     int
	onList        func(int)
}

func (s *storageStub) ListEventsForNotification(
	_ context.Context,
	at time.Time,
	limit int,
) ([]storage.Event, error) {
	s.listedAt = append(s.listedAt, at)
	s.listedLimits = append(s.listedLimits, limit)
	if s.onList != nil {
		s.onList(len(s.listedAt))
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listCalls >= len(s.batches) {
		return nil, nil
	}

	batch := s.batches[s.listCalls]
	s.listCalls++
	return batch, nil
}

func (s *storageStub) MarkNotificationSent(
	_ context.Context,
	event storage.Event,
	sentAt time.Time,
) (bool, error) {
	s.markedEventID = append(s.markedEventID, event.ID)
	s.markedEvents = append(s.markedEvents, event)
	s.markedAt = append(s.markedAt, sentAt)
	if s.markErr != nil {
		return false, s.markErr
	}

	index := len(s.markedEvents) - 1
	if index < len(s.markResults) {
		return s.markResults[index], nil
	}
	return true, nil
}

func (s *storageStub) DeleteEventsBefore(_ context.Context, cutoff time.Time) error {
	s.cutoff = cutoff
	return s.deleteErr
}

type sentMessage struct {
	key     []byte
	payload []byte
}

type producerStub struct {
	messages []sentMessage
	failAt   int
	err      error
}

func (p *producerStub) Send(_ context.Context, key, payload []byte) error {
	call := len(p.messages) + 1
	if call == p.failAt {
		return p.err
	}
	p.messages = append(p.messages, sentMessage{
		key:     append([]byte(nil), key...),
		payload: append([]byte(nil), payload...),
	})
	return nil
}

type loggerStub struct {
	messages []string
	onError  func()
}

func (l *loggerStub) Error(message string) {
	l.messages = append(l.messages, message)
	if l.onError != nil {
		l.onError()
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{Interval: time.Minute, BatchSize: 10, RetentionYears: 1}
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{name: "zero interval", change: func(config *Config) { config.Interval = 0 }},
		{name: "negative interval", change: func(config *Config) { config.Interval = -time.Second }},
		{name: "zero batch size", change: func(config *Config) { config.BatchSize = 0 }},
		{name: "negative batch size", change: func(config *Config) { config.BatchSize = -1 }},
		{name: "zero retention", change: func(config *Config) { config.RetentionYears = 0 }},
		{name: "negative retention", change: func(config *Config) { config.RetentionYears = -1 }},
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.change(&config)
			if err := config.Validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrInvalidConfig)
			}
		})
	}
}

func TestRunOncePublishesBatchesAndMarksEvents(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	events := []storage.Event{
		newEvent("event-1", now.Add(time.Hour)),
		newEvent("event-2", now.Add(2*time.Hour)),
		newEvent("event-3", now.Add(3*time.Hour)),
		newEvent("event-4", now.Add(4*time.Hour)),
		newEvent("event-5", now.Add(5*time.Hour)),
	}
	storageClient := &storageStub{batches: [][]storage.Event{
		events[:2],
		events[2:4],
		events[4:],
	}}
	producer := &producerStub{}
	scheduler := newScheduler(t, storageClient, producer, &loggerStub{}, Config{
		Interval:       time.Minute,
		BatchSize:      2,
		RetentionYears: 2,
	})

	if err := scheduler.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("RunOnce() returned an unexpected error: %v", err)
	}

	wantCutoff := now.AddDate(-2, 0, 0)
	if !storageClient.cutoff.Equal(wantCutoff) {
		t.Fatalf("cleanup cutoff = %v, want %v", storageClient.cutoff, wantCutoff)
	}
	if storageClient.listCalls != 3 {
		t.Fatalf("list calls = %d, want 3", storageClient.listCalls)
	}
	for index, at := range storageClient.listedAt {
		if !at.Equal(now) || storageClient.listedLimits[index] != 2 {
			t.Fatalf(
				"list call %d = (%v, %d), want (%v, 2)",
				index,
				at,
				storageClient.listedLimits[index],
				now,
			)
		}
	}
	if len(producer.messages) != len(events) {
		t.Fatalf("sent messages = %d, want %d", len(producer.messages), len(events))
	}
	if len(storageClient.markedEventID) != len(events) {
		t.Fatalf("marked events = %d, want %d", len(storageClient.markedEventID), len(events))
	}

	for index, event := range events {
		assertPublishedEvent(t, producer.messages[index], event)
		if storageClient.markedEventID[index] != event.ID || !storageClient.markedAt[index].Equal(now) {
			t.Fatalf(
				"mark %d = (%q, %v), want (%q, %v)",
				index,
				storageClient.markedEventID[index],
				storageClient.markedAt[index],
				event.ID,
				now,
			)
		}
	}
}

func TestRunOnceDoesNotMarkEventWhenPublishFails(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	events := []storage.Event{
		newEvent("event-1", now.Add(time.Hour)),
		newEvent("event-2", now.Add(2*time.Hour)),
	}
	wantErr := errors.New("broker unavailable")
	storageClient := &storageStub{batches: [][]storage.Event{events}}
	producer := &producerStub{failAt: 2, err: wantErr}
	scheduler := newScheduler(t, storageClient, producer, &loggerStub{}, Config{
		Interval:       time.Minute,
		BatchSize:      10,
		RetentionYears: 1,
	})

	err := scheduler.RunOnce(context.Background(), now)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunOnce() error = %v, want %v", err, wantErr)
	}
	if len(storageClient.markedEventID) != 1 || storageClient.markedEventID[0] != events[0].ID {
		t.Fatalf("marked events = %v, want only %q", storageClient.markedEventID, events[0].ID)
	}
}

func TestRunOnceRetriesUpdatedSnapshotAfterConditionalMark(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	oldEvent := newEvent("event-1", now.Add(time.Hour))
	updatedEvent := oldEvent
	updatedEvent.Title = "Updated title"
	storageClient := &storageStub{
		batches:     [][]storage.Event{{oldEvent}, {updatedEvent}},
		markResults: []bool{false, true},
	}
	producer := &producerStub{}
	scheduler := newScheduler(t, storageClient, producer, &loggerStub{}, Config{
		Interval:       time.Minute,
		BatchSize:      10,
		RetentionYears: 1,
	})

	if err := scheduler.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("RunOnce() returned an unexpected error: %v", err)
	}
	if storageClient.listCalls != 2 {
		t.Fatalf("list calls = %d, want 2", storageClient.listCalls)
	}
	if len(storageClient.markedEvents) != 2 ||
		storageClient.markedEvents[0].Title != oldEvent.Title ||
		storageClient.markedEvents[1].Title != updatedEvent.Title {
		t.Fatalf("marked snapshots = %#v, want old and updated snapshots", storageClient.markedEvents)
	}
	if len(producer.messages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(producer.messages))
	}
	assertPublishedEvent(t, producer.messages[1], updatedEvent)
}

func TestRunLogsCycleErrorAndStopsOnCancellation(t *testing.T) {
	wantErr := errors.New("database unavailable")
	storageClient := &storageStub{deleteErr: wantErr}
	ctx, cancel := context.WithCancel(context.Background())
	log := &loggerStub{onError: cancel}
	scheduler := newScheduler(t, storageClient, &producerStub{}, log, Config{
		Interval:       time.Hour,
		BatchSize:      10,
		RetentionYears: 1,
	})

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("Run() returned an unexpected error: %v", err)
	}
	if len(log.messages) != 1 || !strings.Contains(log.messages[0], wantErr.Error()) {
		t.Fatalf("log messages = %q, want cycle error containing %q", log.messages, wantErr)
	}
}

func TestRunStopsWhenContextIsAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storageClient := &storageStub{}
	scheduler := newScheduler(t, storageClient, &producerStub{}, &loggerStub{}, Config{
		Interval:       time.Hour,
		BatchSize:      10,
		RetentionYears: 1,
	})

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("Run() returned an unexpected error: %v", err)
	}
	if !storageClient.cutoff.IsZero() {
		t.Fatalf("storage was called for an already cancelled context: cutoff = %v", storageClient.cutoff)
	}
}

func TestRunUsesCurrentTimeInsteadOfBufferedTickTimestamp(t *testing.T) {
	initialTime := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	currentTime := initialTime.Add(3 * time.Minute)
	staleTickTime := initialTime.Add(time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	storageClient := &storageStub{}
	storageClient.onList = func(call int) {
		if call == 2 {
			cancel()
		}
	}
	scheduler := newScheduler(t, storageClient, &producerStub{}, &loggerStub{}, Config{
		Interval:       time.Minute,
		BatchSize:      10,
		RetentionYears: 1,
	})

	times := []time.Time{initialTime, currentTime}
	nowCall := 0
	scheduler.now = func() time.Time {
		value := times[nowCall]
		nowCall++
		return value
	}
	ticks := make(chan time.Time, 1)
	ticks <- staleTickTime
	tickerStopped := false
	scheduler.newTicker = func(time.Duration) (<-chan time.Time, func()) {
		return ticks, func() { tickerStopped = true }
	}

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("Run() returned an unexpected error: %v", err)
	}
	if len(storageClient.listedAt) != 2 ||
		!storageClient.listedAt[0].Equal(initialTime) ||
		!storageClient.listedAt[1].Equal(currentTime) {
		t.Fatalf(
			"cycle times = %v, want current times [%v %v] and not stale tick %v",
			storageClient.listedAt,
			initialTime,
			currentTime,
			staleTickTime,
		)
	}
	if !tickerStopped {
		t.Fatal("ticker was not stopped")
	}
}

func newScheduler(
	t *testing.T,
	storageClient Storage,
	producer Producer,
	logger Logger,
	config Config,
) *Scheduler {
	t.Helper()

	value, err := New(storageClient, producer, logger, config)
	if err != nil {
		t.Fatalf("New() returned an unexpected error: %v", err)
	}
	return value
}

func newEvent(id string, eventAt time.Time) storage.Event {
	return storage.Event{
		ID:      id,
		Title:   "Title for " + id,
		StartAt: eventAt,
		EndAt:   eventAt.Add(time.Hour),
		UserID:  "user-for-" + id,
	}
}

func assertPublishedEvent(t *testing.T, message sentMessage, event storage.Event) {
	t.Helper()

	if string(message.key) != event.ID {
		t.Fatalf("message key = %q, want %q", message.key, event.ID)
	}

	var value notification.Notification
	if err := json.Unmarshal(message.payload, &value); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	want := notification.Notification{
		EventID: event.ID,
		Title:   event.Title,
		EventAt: event.StartAt,
		UserID:  event.UserID,
	}
	if value != want {
		t.Fatalf("notification = %#v, want %#v", value, want)
	}
}
