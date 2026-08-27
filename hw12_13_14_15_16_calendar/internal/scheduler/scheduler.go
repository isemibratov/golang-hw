// Package scheduler periodically publishes notifications for due calendar events.
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

// ErrInvalidConfig means that scheduler settings cannot produce a valid schedule.
var ErrInvalidConfig = errors.New("invalid scheduler config")

// Storage contains the event operations required by the scheduler.
type Storage interface {
	ListEventsForNotification(ctx context.Context, at time.Time, limit int) ([]storage.Event, error)
	MarkNotificationSent(ctx context.Context, event storage.Event, sentAt time.Time) (bool, error)
	DeleteEventsBefore(ctx context.Context, cutoff time.Time) error
}

// Producer sends serialized notifications to a message broker.
type Producer interface {
	Send(ctx context.Context, key, payload []byte) error
}

// Logger records scheduler cycle failures.
type Logger interface {
	Error(message string)
}

// Config contains scheduler execution and retention settings.
type Config struct {
	Interval       time.Duration
	BatchSize      int
	RetentionYears int
}

// Validate checks that all scheduler settings are positive.
func (c Config) Validate() error {
	switch {
	case c.Interval <= 0:
		return fmt.Errorf("%w: interval must be positive", ErrInvalidConfig)
	case c.BatchSize <= 0:
		return fmt.Errorf("%w: batch size must be positive", ErrInvalidConfig)
	case c.RetentionYears <= 0:
		return fmt.Errorf("%w: retention years must be positive", ErrInvalidConfig)
	default:
		return nil
	}
}

// Scheduler removes expired events and publishes notifications for due events.
type Scheduler struct {
	storage   Storage
	producer  Producer
	logger    Logger
	config    Config
	now       func() time.Time
	newTicker func(time.Duration) (<-chan time.Time, func())
}

// New constructs a scheduler from its dependencies and validated settings.
func New(storageClient Storage, producer Producer, logger Logger, config Config) (*Scheduler, error) {
	if storageClient == nil {
		return nil, errors.New("scheduler storage is nil")
	}
	if producer == nil {
		return nil, errors.New("scheduler producer is nil")
	}
	if logger == nil {
		return nil, errors.New("scheduler logger is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Scheduler{
		storage:   storageClient,
		producer:  producer,
		logger:    logger,
		config:    config,
		now:       time.Now,
		newTicker: newTicker,
	}, nil
}

// RunOnce performs one cleanup and notification cycle at the supplied time.
func (s *Scheduler) RunOnce(ctx context.Context, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cutoff := now.AddDate(-s.config.RetentionYears, 0, 0)
	if err := s.storage.DeleteEventsBefore(ctx, cutoff); err != nil {
		return fmt.Errorf("delete expired events: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		events, err := s.storage.ListEventsForNotification(ctx, now, s.config.BatchSize)
		if err != nil {
			return fmt.Errorf("list events for notification: %w", err)
		}
		if len(events) == 0 {
			return nil
		}

		staleSnapshot := false
		for _, event := range events {
			marked, publishErr := s.publish(ctx, event, now)
			if publishErr != nil {
				return publishErr
			}
			if !marked {
				staleSnapshot = true
			}
		}

		if len(events) < s.config.BatchSize && !staleSnapshot {
			return nil
		}
	}
}

// Run performs a cycle immediately and then at every configured interval until cancellation.
func (s *Scheduler) Run(ctx context.Context) error {
	s.runAndLog(ctx, s.now())

	ticks, stopTicker := s.newTicker(s.config.Interval)
	defer stopTicker()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
			s.runAndLog(ctx, s.now())
		}
	}
}

func newTicker(interval time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(interval)
	return ticker.C, ticker.Stop
}

func (s *Scheduler) publish(
	ctx context.Context,
	event storage.Event,
	sentAt time.Time,
) (bool, error) {
	message := notification.Notification{
		EventID: event.ID,
		Title:   event.Title,
		EventAt: event.StartAt,
		UserID:  event.UserID,
	}
	if err := message.Validate(); err != nil {
		return false, fmt.Errorf("build notification for event %q: %w", event.ID, err)
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return false, fmt.Errorf("serialize notification for event %q: %w", event.ID, err)
	}
	if err = s.producer.Send(ctx, []byte(event.ID), payload); err != nil {
		return false, fmt.Errorf("send notification for event %q: %w", event.ID, err)
	}

	marked, err := s.storage.MarkNotificationSent(ctx, event, sentAt)
	if err != nil {
		return false, fmt.Errorf("mark notification sent for event %q: %w", event.ID, err)
	}

	return marked, nil
}

func (s *Scheduler) runAndLog(ctx context.Context, now time.Time) {
	if err := s.RunOnce(ctx, now); err != nil && ctx.Err() == nil {
		s.logger.Error("scheduler cycle failed: " + err.Error())
	}
}
