package storer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/notification"
)

type consumerStub struct {
	payload    []byte
	err        error
	handlerErr error
	committed  bool
}

func (c *consumerStub) Consume(ctx context.Context, handler func(context.Context, []byte) error) error {
	if c.err != nil {
		return c.err
	}
	c.handlerErr = handler(ctx, c.payload)
	c.committed = c.handlerErr == nil
	return c.handlerErr
}

type storageStub struct {
	values  []notification.Notification
	results []error
}

func (s *storageStub) SaveNotification(_ context.Context, value notification.Notification) error {
	s.values = append(s.values, value)
	call := len(s.values) - 1
	if call < len(s.results) {
		return s.results[call]
	}
	return nil
}

type loggerStub struct {
	messages []string
}

func (l *loggerStub) Error(message string) {
	l.messages = append(l.messages, message)
}

type storageObservation struct {
	success    bool
	finishedAt time.Time
}

type metricsStub struct {
	running []bool
	stored  []storageObservation
	invalid int
}

func (m *metricsStub) SetStorerRunning(running bool) {
	m.running = append(m.running, running)
}

func (m *metricsStub) ObserveNotificationStored(success bool, finishedAt time.Time) {
	m.stored = append(m.stored, storageObservation{success: success, finishedAt: finishedAt})
}

func (m *metricsStub) ObserveInvalidNotification() {
	m.invalid++
}

func TestStorerSavesValidNotification(t *testing.T) {
	want := notification.Notification{
		EventID: "event-1",
		Title:   "Team meeting",
		EventAt: time.Date(2026, time.August, 25, 9, 30, 0, 0, time.UTC),
		UserID:  "user-1",
	}
	payload := []byte(`{
		"eventId":"event-1",
		"title":"Team meeting",
		"eventAt":"2026-08-25T09:30:00Z",
		"userId":"user-1"
	}`)
	storageClient := &storageStub{}
	metrics := &metricsStub{}
	service := newStorer(t, &consumerStub{payload: payload}, storageClient, &loggerStub{}, metrics)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(storageClient.values) != 1 {
		t.Fatalf("saved notifications count = %d, want 1", len(storageClient.values))
	}
	if !reflect.DeepEqual(storageClient.values[0], want) {
		t.Fatalf("saved notification = %+v, want %+v", storageClient.values[0], want)
	}
	if !reflect.DeepEqual(metrics.running, []bool{true, false}) {
		t.Fatalf("storer running observations = %v, want [true false]", metrics.running)
	}
	if len(metrics.stored) != 1 || !metrics.stored[0].success || metrics.stored[0].finishedAt.IsZero() {
		t.Fatalf("storage metric observations = %#v, want one successful timestamped observation", metrics.stored)
	}
}

func TestStorerSkipsAndLogsInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "malformed", payload: `{`},
		{
			name: "unknown field",
			payload: `{"eventId":"event-1","title":"Meeting","eventAt":"2026-08-25T09:30:00Z",` +
				`"userId":"user-1","unexpected":true}`,
		},
		{
			name: "trailing object",
			payload: `{"eventId":"event-1","title":"Meeting","eventAt":"2026-08-25T09:30:00Z",` +
				`"userId":"user-1"} {}`,
		},
		{name: "missing required value", payload: `{}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consumer := &consumerStub{payload: []byte(test.payload)}
			storageClient := &storageStub{}
			logg := &loggerStub{}
			metrics := &metricsStub{}
			service := newStorer(t, consumer, storageClient, logg, metrics)

			if err := service.Run(context.Background()); err != nil {
				t.Fatalf("Run() error = %v, want nil so the payload is committed", err)
			}
			if consumer.handlerErr != nil {
				t.Fatalf("handler error = %v, want nil so the payload is committed", consumer.handlerErr)
			}
			if !consumer.committed {
				t.Fatal("invalid payload was not committed")
			}
			if len(storageClient.values) != 0 {
				t.Fatalf("saved notifications count = %d, want 0", len(storageClient.values))
			}
			if len(logg.messages) != 1 || !strings.Contains(logg.messages[0], "skipping invalid notification") {
				t.Fatalf("log messages = %#v, want one skip message", logg.messages)
			}
			if metrics.invalid != 1 || len(metrics.stored) != 0 {
				t.Fatalf("metrics = invalid:%d stored:%#v, want one invalid observation",
					metrics.invalid, metrics.stored)
			}
		})
	}
}

func TestStorerReturnsStorageFailureWithoutCommitting(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	payload := []byte(
		`{"eventId":"event-1","title":"Meeting","eventAt":"2026-08-25T09:30:00Z","userId":"user-1"}`,
	)
	storageClient := &storageStub{results: []error{wantErr}}
	consumer := &consumerStub{payload: payload}
	metrics := &metricsStub{}
	service := newStorer(t, consumer, storageClient, &loggerStub{}, metrics)

	err := service.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if len(storageClient.values) != 1 {
		t.Fatalf("save attempts = %d, want 1", len(storageClient.values))
	}
	if consumer.committed {
		t.Fatal("message was committed after notification storage failure")
	}
	if len(metrics.stored) != 1 || metrics.stored[0].success || metrics.stored[0].finishedAt.IsZero() {
		t.Fatalf("storage metric observations = %#v, want one failed timestamped observation", metrics.stored)
	}
}

func TestStorerDoesNotReportCancelledStorageAsFailure(t *testing.T) {
	metrics := &metricsStub{}
	storageClient := &storageStub{results: []error{context.Canceled}}
	service := newStorer(t, &consumerStub{}, storageClient, &loggerStub{}, metrics)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := []byte(`{"eventId":"e","title":"x","eventAt":"2026-08-25T09:30:00Z","userId":"u"}`)
	err := service.handle(ctx, payload)
	if !errors.Is(err, context.Canceled) || len(metrics.stored) != 0 {
		t.Fatalf("handle() error = %v, metrics = %#v; want cancellation without observations", err, metrics.stored)
	}
}

func TestNewValidatesDependencies(t *testing.T) {
	validConsumer := &consumerStub{}
	validStorage := &storageStub{}
	validLogger := &loggerStub{}

	tests := []struct {
		name     string
		consumer Consumer
		storage  Storage
		logger   Logger
	}{
		{name: "nil consumer", storage: validStorage, logger: validLogger},
		{name: "nil storage", consumer: validConsumer, logger: validLogger},
		{name: "nil logger", consumer: validConsumer, storage: validStorage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.consumer, test.storage, test.logger); err == nil {
				t.Fatal("New() error = nil, want an error")
			}
		})
	}
}

func TestStorerDelegatesConsumerError(t *testing.T) {
	wantErr := errors.New("consumer failure")
	service := newStorer(t, &consumerStub{err: wantErr}, &storageStub{}, &loggerStub{})

	err := service.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func newStorer(t *testing.T, consumer Consumer, storage Storage, logger Logger, metrics ...Metrics) *Storer {
	t.Helper()

	service, err := New(consumer, storage, logger, metrics...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}
