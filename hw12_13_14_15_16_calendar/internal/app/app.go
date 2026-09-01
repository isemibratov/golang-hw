// Package app implements the calendar use cases independently of transports and storage details.
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

// App coordinates calendar operations through an event storage.
type App struct {
	logger  Logger
	storage Storage
}

// Logger contains the logging operations used by the application.
type Logger interface {
	Debug(msg string)
	Error(msg string)
}

// Storage describes persistent operations required by the application.
type Storage interface {
	CreateEvent(ctx context.Context, event storage.Event) error
	UpdateEvent(ctx context.Context, event storage.Event) error
	DeleteEvent(ctx context.Context, id string) error
	ListEvents(ctx context.Context, userID string, from, to time.Time) ([]storage.Event, error)
}

// New constructs a calendar application from its dependencies.
func New(logger Logger, storage Storage) *App {
	return &App{logger: logger, storage: storage}
}

// CreateEvent adds an event to the calendar.
func (a *App) CreateEvent(ctx context.Context, event storage.Event) error {
	if err := a.storage.CreateEvent(ctx, event); err != nil {
		a.logger.Error(fmt.Sprintf("failed to create event %q: %v", event.ID, err))
		return err
	}
	a.logger.Debug(fmt.Sprintf("event %q created", event.ID))
	return nil
}

// UpdateEvent replaces an existing event.
func (a *App) UpdateEvent(ctx context.Context, event storage.Event) error {
	if err := a.storage.UpdateEvent(ctx, event); err != nil {
		a.logger.Error(fmt.Sprintf("failed to update event %q: %v", event.ID, err))
		return err
	}
	a.logger.Debug(fmt.Sprintf("event %q updated", event.ID))
	return nil
}

// DeleteEvent removes an event by ID.
func (a *App) DeleteEvent(ctx context.Context, id string) error {
	if err := a.storage.DeleteEvent(ctx, id); err != nil {
		a.logger.Error(fmt.Sprintf("failed to delete event %q: %v", id, err))
		return err
	}
	a.logger.Debug(fmt.Sprintf("event %q deleted", id))
	return nil
}

// ListEvents returns user events that overlap the half-open interval [from, to).
func (a *App) ListEvents(
	ctx context.Context,
	userID string,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	events, err := a.storage.ListEvents(ctx, userID, from, to)
	if err != nil {
		a.logger.Error(fmt.Sprintf("failed to list events for user %q: %v", userID, err))
		return nil, err
	}
	return events, nil
}

// ListEventsForDay returns events for the calendar day containing date.
func (a *App) ListEventsForDay(ctx context.Context, userID string, date time.Time) ([]storage.Event, error) {
	from := beginningOfDay(date)
	return a.ListEvents(ctx, userID, from, from.AddDate(0, 0, 1))
}

// ListEventsForWeek returns events for seven days starting at weekStart.
func (a *App) ListEventsForWeek(ctx context.Context, userID string, weekStart time.Time) ([]storage.Event, error) {
	from := beginningOfDay(weekStart)
	return a.ListEvents(ctx, userID, from, from.AddDate(0, 0, 7))
}

// ListEventsForMonth returns events for the month containing date.
func (a *App) ListEventsForMonth(ctx context.Context, userID string, date time.Time) ([]storage.Event, error) {
	from := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	return a.ListEvents(ctx, userID, from, from.AddDate(0, 1, 0))
}

func beginningOfDay(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}
