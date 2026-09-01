// Package storer implements notification persistence independently of Kafka details.
package storer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
)

// Consumer delivers serialized notifications to a handler.
type Consumer interface {
	Consume(ctx context.Context, handler func(context.Context, []byte) error) error
}

// Storage persists validated notifications.
type Storage interface {
	SaveNotification(ctx context.Context, value notification.Notification) error
}

// Logger records skipped payloads.
type Logger interface {
	Error(message string)
}

// Metrics records storer health and notification processing results.
type Metrics interface {
	SetStorerRunning(running bool)
	ObserveNotificationStored(success bool, finishedAt time.Time)
	ObserveInvalidNotification()
}

type noopMetrics struct{}

func (noopMetrics) SetStorerRunning(bool) {}

func (noopMetrics) ObserveNotificationStored(bool, time.Time) {}

func (noopMetrics) ObserveInvalidNotification() {}

// Storer validates consumed notifications and persists them.
type Storer struct {
	consumer Consumer
	storage  Storage
	logger   Logger
	metrics  Metrics
}

// New creates a notification storer.
func New(consumer Consumer, storage Storage, logger Logger, metrics ...Metrics) (*Storer, error) {
	if consumer == nil {
		return nil, errors.New("storer consumer is nil")
	}
	if storage == nil {
		return nil, errors.New("storer storage is nil")
	}
	if logger == nil {
		return nil, errors.New("storer logger is nil")
	}

	return &Storer{
		consumer: consumer,
		storage:  storage,
		logger:   logger,
		metrics:  firstMetrics(metrics),
	}, nil
}

func firstMetrics(values []Metrics) Metrics {
	if len(values) == 0 || values[0] == nil {
		return noopMetrics{}
	}
	return values[0]
}

// Run consumes notifications until the consumer stops.
func (s *Storer) Run(ctx context.Context) error {
	s.metrics.SetStorerRunning(true)
	defer s.metrics.SetStorerRunning(false)

	return s.consumer.Consume(ctx, s.handle)
}

func (s *Storer) handle(ctx context.Context, payload []byte) error {
	value, valid := s.validNotification(payload)
	if !valid {
		s.metrics.ObserveInvalidNotification()
		// Invalid payloads are non-retryable, so nil tells the consumer to
		// commit the offset and continue with the next message.
		return nil
	}

	err := s.storage.SaveNotification(ctx, value)
	if err != nil && errors.Is(err, ctx.Err()) {
		return err
	}
	s.metrics.ObserveNotificationStored(err == nil, time.Now())
	return err
}

func (s *Storer) validNotification(payload []byte) (notification.Notification, bool) {
	value, err := decodeNotification(payload)
	if err != nil {
		s.logger.Error("skipping invalid notification payload: " + err.Error())
		return notification.Notification{}, false
	}
	return value, true
}

func decodeNotification(payload []byte) (notification.Notification, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var value notification.Notification
	if err := decoder.Decode(&value); err != nil {
		return notification.Notification{}, fmt.Errorf("decode notification: %w", err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("payload contains more than one JSON value")
		}
		return notification.Notification{}, fmt.Errorf("decode notification: %w", err)
	}

	if err := value.Validate(); err != nil {
		return notification.Notification{}, fmt.Errorf("validate notification: %w", err)
	}

	return value, nil
}
