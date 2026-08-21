package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

type loggerStub struct{}

func (loggerStub) Debug(string) {}
func (loggerStub) Error(string) {}

type storageStub struct {
	from time.Time
	to   time.Time
	err  error
}

func (s *storageStub) CreateEvent(context.Context, storage.Event) error { return s.err }
func (s *storageStub) UpdateEvent(context.Context, storage.Event) error { return s.err }
func (s *storageStub) DeleteEvent(context.Context, string) error        { return s.err }
func (s *storageStub) ListEvents(
	_ context.Context,
	_ string,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	s.from = from
	s.to = to
	return nil, s.err
}

func TestAppDelegatesStorageErrors(t *testing.T) {
	wantErr := errors.New("storage failure")
	storageClient := &storageStub{err: wantErr}
	calendar := New(loggerStub{}, storageClient)

	err := calendar.CreateEvent(context.Background(), storage.Event{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateEvent() error = %v, want %v", err, wantErr)
	}
}

func TestAppListPeriods(t *testing.T) {
	location := time.FixedZone("test", 3*60*60)
	date := time.Date(2026, time.March, 29, 15, 30, 0, 0, location)

	tests := []struct {
		name     string
		call     func(*App) error
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			name: "day",
			call: func(calendar *App) error {
				_, err := calendar.ListEventsForDay(context.Background(), "user", date)
				return err
			},
			wantFrom: time.Date(2026, time.March, 29, 0, 0, 0, 0, location),
			wantTo:   time.Date(2026, time.March, 30, 0, 0, 0, 0, location),
		},
		{
			name: "week",
			call: func(calendar *App) error {
				_, err := calendar.ListEventsForWeek(context.Background(), "user", date)
				return err
			},
			wantFrom: time.Date(2026, time.March, 29, 0, 0, 0, 0, location),
			wantTo:   time.Date(2026, time.April, 5, 0, 0, 0, 0, location),
		},
		{
			name: "month",
			call: func(calendar *App) error {
				_, err := calendar.ListEventsForMonth(context.Background(), "user", date)
				return err
			},
			wantFrom: time.Date(2026, time.March, 1, 0, 0, 0, 0, location),
			wantTo:   time.Date(2026, time.April, 1, 0, 0, 0, 0, location),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storageClient := &storageStub{}
			calendar := New(loggerStub{}, storageClient)
			if err := test.call(calendar); err != nil {
				t.Fatalf("list period: %v", err)
			}
			if !storageClient.from.Equal(test.wantFrom) || !storageClient.to.Equal(test.wantTo) {
				t.Fatalf(
					"range = [%v, %v), want [%v, %v)",
					storageClient.from,
					storageClient.to,
					test.wantFrom,
					test.wantTo,
				)
			}
		})
	}
}
