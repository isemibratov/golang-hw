//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const (
	defaultHTTPURL     = "http://calendar:8080"
	defaultDatabaseDSN = "postgres://calendar:calendar@postgres:5432/calendar?sslmode=disable"
	eventsPath         = "/api/v1/events"
	calendarDateLayout = "2006-01-02"
	requestTimeout     = 5 * time.Second
	pipelineTimeout    = 30 * time.Second
)

type eventRequest struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	StartAt             time.Time `json:"start_at"`
	EndAt               time.Time `json:"end_at"`
	Description         string    `json:"description"`
	UserID              string    `json:"user_id"`
	NotifyBeforeSeconds int64     `json:"notify_before_seconds"`
}

type eventResponse struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	StartAt             time.Time `json:"start_at"`
	EndAt               time.Time `json:"end_at"`
	Description         string    `json:"description"`
	UserID              string    `json:"user_id"`
	NotifyBeforeSeconds int64     `json:"notify_before_seconds"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type testEnvironment struct {
	baseURL string
	client  *http.Client
	db      *sql.DB
}

type storedNotification struct {
	title   string
	eventAt time.Time
	userID  string
	sentAt  sql.NullTime
}

func TestCalendarAPI(t *testing.T) {
	environment := newTestEnvironment(t)
	resetDatabase(t, environment.db)

	anchor := futureMonthStart(time.Now().UTC())
	userID := "integration-list-user"
	dayEvent := newEventRequest("integration-day", "Day event", userID, anchor.Add(10*time.Hour))
	weekEvent := newEventRequest("integration-week", "Week event", userID, anchor.AddDate(0, 0, 2))
	monthEvent := newEventRequest("integration-month", "Month event", userID, anchor.AddDate(0, 0, 10))

	created := environment.createEvent(t, dayEvent)
	assertCreatedEvent(t, created, dayEvent)

	t.Run("business errors", func(t *testing.T) {
		assertCreateError(t, environment, dayEvent, http.StatusConflict, "event_already_exists")

		overlap := newEventRequest(
			"integration-overlap",
			"Overlapping event",
			userID,
			dayEvent.StartAt.Add(30*time.Minute),
		)
		assertCreateError(t, environment, overlap, http.StatusConflict, "date_busy")

		invalid := newEventRequest("integration-invalid", "Invalid event", userID, anchor.Add(15*time.Hour))
		invalid.EndAt = invalid.StartAt.Add(-time.Hour)
		assertCreateError(t, environment, invalid, http.StatusBadRequest, "invalid_event")
	})

	environment.createEvent(t, weekEvent)
	environment.createEvent(t, monthEvent)

	t.Run("list events for day", func(t *testing.T) {
		assertListedEventIDs(t, environment, "day", userID, anchor, []string{dayEvent.ID})
	})
	t.Run("list events for week", func(t *testing.T) {
		want := []string{dayEvent.ID, weekEvent.ID}
		assertListedEventIDs(t, environment, "week", userID, anchor, want)
	})
	t.Run("list events for month", func(t *testing.T) {
		want := []string{dayEvent.ID, weekEvent.ID, monthEvent.ID}
		assertListedEventIDs(t, environment, "month", userID, anchor, want)
	})
}

func TestNotificationPipelinePersistsNotification(t *testing.T) {
	environment := newTestEnvironment(t)
	resetDatabase(t, environment.db)

	request := newEventRequest(
		"integration-notification",
		"Notification pipeline event",
		"integration-notification-user",
		time.Now().UTC().Add(5*time.Minute).Truncate(time.Second),
	)
	request.NotifyBeforeSeconds = int64((10 * time.Minute) / time.Second)
	environment.createEvent(t, request)

	notification := waitForNotification(t, environment.db, request.ID)
	if notification.title != request.Title || notification.userID != request.UserID {
		t.Fatalf("unexpected stored notification: %#v", notification)
	}
	if !notification.eventAt.Equal(request.StartAt) {
		t.Fatalf("notification event time = %s, want %s", notification.eventAt, request.StartAt)
	}
	if !notification.sentAt.Valid {
		t.Fatal("event notification_sent_at is NULL after notification was stored")
	}
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	databaseDSN := environmentValue("CALENDAR_DATABASE_DSN", defaultDatabaseDSN)
	database, err := sql.Open("postgres", databaseDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close PostgreSQL connection: %v", closeErr)
		}
	})
	waitForDatabase(t, database)

	return &testEnvironment{
		baseURL: strings.TrimRight(environmentValue("CALENDAR_HTTP_URL", defaultHTTPURL), "/"),
		client:  &http.Client{Timeout: requestTimeout},
		db:      database,
	}
}

func environmentValue(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func waitForDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		lastErr = database.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("PostgreSQL did not become ready: %v", lastErr)
}

func resetDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := database.ExecContext(ctx, "TRUNCATE TABLE notifications, events"); err != nil {
		t.Fatalf("reset integration database: %v", err)
	}
}

func newEventRequest(id, title, userID string, startAt time.Time) eventRequest {
	return eventRequest{
		ID:          id,
		Title:       title,
		StartAt:     startAt,
		EndAt:       startAt.Add(time.Hour),
		Description: title + " description",
		UserID:      userID,
	}
}

func futureMonthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, 0, time.UTC)
}

func (environment *testEnvironment) createEvent(t *testing.T, request eventRequest) eventResponse {
	t.Helper()

	status, _, body := environment.sendJSON(t, http.MethodPost, eventsPath, request)
	if status != http.StatusCreated {
		t.Fatalf("create event status = %d, want %d; body: %s", status, http.StatusCreated, body)
	}

	var response eventResponse
	decodeResponse(t, body, &response)
	return response
}

func assertCreatedEvent(t *testing.T, got eventResponse, want eventRequest) {
	t.Helper()

	if got.ID != want.ID || got.Title != want.Title || got.UserID != want.UserID {
		t.Fatalf("created event = %#v, want request %#v", got, want)
	}
	if !got.StartAt.Equal(want.StartAt) || !got.EndAt.Equal(want.EndAt) {
		t.Fatalf("created event period = [%s, %s), want [%s, %s)",
			got.StartAt, got.EndAt, want.StartAt, want.EndAt)
	}
}

func assertCreateError(
	t *testing.T,
	environment *testEnvironment,
	request eventRequest,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	status, _, body := environment.sendJSON(t, http.MethodPost, eventsPath, request)
	if status != wantStatus {
		t.Fatalf("create event status = %d, want %d; body: %s", status, wantStatus, body)
	}

	var response errorResponse
	decodeResponse(t, body, &response)
	if response.Code != wantCode {
		t.Fatalf("business error code = %q, want %q; response: %#v", response.Code, wantCode, response)
	}
}

func assertListedEventIDs(
	t *testing.T,
	environment *testEnvironment,
	period string,
	userID string,
	date time.Time,
	want []string,
) {
	t.Helper()

	query := url.Values{}
	query.Set("user_id", userID)
	query.Set("date", date.Format(calendarDateLayout))
	path := fmt.Sprintf("%s/%s?%s", eventsPath, period, query.Encode())
	status, _, body := environment.sendJSON(t, http.MethodGet, path, nil)
	if status != http.StatusOK {
		t.Fatalf("list events for %s status = %d, want %d; body: %s",
			period, status, http.StatusOK, body)
	}

	var events []eventResponse
	decodeResponse(t, body, &events)
	got := make([]string, 0, len(events))
	for _, event := range events {
		got = append(got, event.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("list events for %s IDs = %v, want %v", period, got, want)
	}
}

func (environment *testEnvironment) sendJSON(
	t *testing.T,
	method string,
	path string,
	payload interface{},
) (int, http.Header, []byte) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode request body: %v", err)
		}
		body = bytes.NewReader(encoded)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, environment.baseURL+path, body)
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := environment.client.Do(request)
	if err != nil {
		t.Fatalf("perform HTTP request: %v", err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("read HTTP response: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close HTTP response: %v", closeErr)
	}
	return response.StatusCode, response.Header.Clone(), responseBody
}

func decodeResponse(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
}

func waitForNotification(t *testing.T, database *sql.DB, eventID string) storedNotification {
	t.Helper()

	const query = `
		SELECT n.title, n.event_at, n.user_id, e.notification_sent_at
		FROM notifications n
		JOIN events e ON e.id = n.event_id
		WHERE n.event_id = $1`

	deadline := time.Now().Add(pipelineTimeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		var notification storedNotification
		err := database.QueryRowContext(ctx, query, eventID).Scan(
			&notification.title,
			&notification.eventAt,
			&notification.userID,
			&notification.sentAt,
		)
		cancel()
		switch {
		case err == nil && notification.sentAt.Valid:
			return notification
		case err == nil, errors.Is(err, sql.ErrNoRows):
			time.Sleep(200 * time.Millisecond)
		default:
			t.Fatalf("query stored notification: %v", err)
		}
	}

	t.Fatalf("notification %q was not persisted within %s", eventID, pipelineTimeout)
	return storedNotification{}
}
