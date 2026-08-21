package internalhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapi "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/server/http/openapi"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

const (
	validCreateBody = `{
		"id":"event-1",
		"title":"Team meeting",
		"start_at":"2026-08-21T10:00:00Z",
		"end_at":"2026-08-21T11:00:00Z",
		"description":"Discuss release",
		"user_id":"user-1",
		"notify_before_seconds":900
	}`
	validUpdateBody = `{
		"title":"Updated meeting",
		"start_at":"2026-08-21T11:00:00Z",
		"end_at":"2026-08-21T12:00:00Z",
		"user_id":"user-2"
	}`
)

var apiTestStart = time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)

type applicationStub struct {
	createdEvent storage.Event
	updatedEvent storage.Event
	deletedID    string
	listPeriod   string
	listedUserID string
	listedDate   time.Time
	events       []storage.Event
	err          error
	calls        int
}

func (s *applicationStub) CreateEvent(_ context.Context, event storage.Event) error {
	s.calls++
	s.createdEvent = event
	return s.err
}

func (s *applicationStub) UpdateEvent(_ context.Context, event storage.Event) error {
	s.calls++
	s.updatedEvent = event
	return s.err
}

func (s *applicationStub) DeleteEvent(_ context.Context, id string) error {
	s.calls++
	s.deletedID = id
	return s.err
}

func (s *applicationStub) ListEventsForDay(
	_ context.Context,
	userID string,
	date time.Time,
) ([]storage.Event, error) {
	return s.listEvents("day", userID, date)
}

func (s *applicationStub) ListEventsForWeek(
	_ context.Context,
	userID string,
	date time.Time,
) ([]storage.Event, error) {
	return s.listEvents("week", userID, date)
}

func (s *applicationStub) ListEventsForMonth(
	_ context.Context,
	userID string,
	date time.Time,
) ([]storage.Event, error) {
	return s.listEvents("month", userID, date)
}

func (s *applicationStub) listEvents(period, userID string, date time.Time) ([]storage.Event, error) {
	s.calls++
	s.listPeriod = period
	s.listedUserID = userID
	s.listedDate = date
	return s.events, s.err
}

func TestCreateEventAPI(t *testing.T) {
	calendar := &applicationStub{}
	response := serveAPIRequest(t, calendar, http.MethodPost, "/api/v1/events", validCreateBody, true)

	assertStatus(t, response, http.StatusCreated)
	if response.Header().Get("Location") != "/api/v1/events/event-1" {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
	if calendar.calls != 1 {
		t.Fatalf("application calls = %d, want 1", calendar.calls)
	}
	want := storage.Event{
		ID:           "event-1",
		Title:        "Team meeting",
		StartAt:      apiTestStart,
		EndAt:        apiTestStart.Add(time.Hour),
		Description:  "Discuss release",
		UserID:       "user-1",
		NotifyBefore: 15 * time.Minute,
	}
	if calendar.createdEvent != want {
		t.Fatalf("created event = %#v, want %#v", calendar.createdEvent, want)
	}
	assertEventResponse(t, response, want)
}

func TestUpdateEventAPI(t *testing.T) {
	calendar := &applicationStub{}
	response := serveAPIRequest(
		t,
		calendar,
		http.MethodPut,
		"/api/v1/events/event-42",
		validUpdateBody,
		true,
	)

	assertStatus(t, response, http.StatusOK)
	want := storage.Event{
		ID:      "event-42",
		Title:   "Updated meeting",
		StartAt: apiTestStart.Add(time.Hour),
		EndAt:   apiTestStart.Add(2 * time.Hour),
		UserID:  "user-2",
	}
	if calendar.updatedEvent != want {
		t.Fatalf("updated event = %#v, want %#v", calendar.updatedEvent, want)
	}
	assertEventResponse(t, response, want)
}

func TestDeleteEventAPI(t *testing.T) {
	calendar := &applicationStub{}
	response := serveAPIRequest(t, calendar, http.MethodDelete, "/api/v1/events/event-7", "", false)

	assertStatus(t, response, http.StatusNoContent)
	if calendar.deletedID != "event-7" {
		t.Fatalf("deleted ID = %q, want event-7", calendar.deletedID)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty", response.Body.String())
	}
}

func TestListEventsAPI(t *testing.T) {
	event := storage.Event{
		ID:           "event-1",
		Title:        "Team meeting",
		StartAt:      apiTestStart,
		EndAt:        apiTestStart.Add(time.Hour),
		Description:  "Discuss release",
		UserID:       "user-1",
		NotifyBefore: time.Minute,
	}
	tests := []struct {
		name   string
		period string
		path   string
	}{
		{name: "day", period: "day", path: "/api/v1/events/day"},
		{name: "week", period: "week", path: "/api/v1/events/week"},
		{name: "month", period: "month", path: "/api/v1/events/month"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calendar := &applicationStub{events: []storage.Event{event}}
			response := serveAPIRequest(
				t,
				calendar,
				http.MethodGet,
				test.path+"?user_id=user-1&date=2026-08-21",
				"",
				false,
			)

			assertStatus(t, response, http.StatusOK)
			if calendar.listPeriod != test.period || calendar.listedUserID != "user-1" {
				t.Fatalf("list call = (%q, %q)", calendar.listPeriod, calendar.listedUserID)
			}
			if !calendar.listedDate.Equal(apiTestStart.Truncate(24 * time.Hour)) {
				t.Fatalf("listed date = %v", calendar.listedDate)
			}

			var events []openapi.Event
			decodeResponse(t, response, &events)
			if len(events) != 1 || events[0] != eventResponse(event) {
				t.Fatalf("response events = %#v", events)
			}
		})
	}
}

func TestListEventsAPIEncodesEmptyArray(t *testing.T) {
	response := serveAPIRequest(
		t,
		&applicationStub{},
		http.MethodGet,
		"/api/v1/events/day?user_id=user-1&date=2026-08-21",
		"",
		false,
	)

	assertStatus(t, response, http.StatusOK)
	if response.Body.String() != "[]\n" {
		t.Fatalf("response body = %q, want []", response.Body.String())
	}
}

func TestCalendarAPIErrors(t *testing.T) {
	internalErr := errors.New("database password must not leak")
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType bool
		appErr      error
		wantStatus  int
		wantCode    string
		wantCalls   int
	}{
		{
			name:   "malformed JSON",
			method: http.MethodPost, path: "/api/v1/events", body: "{", contentType: true,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name:   "unknown JSON field",
			method: http.MethodPost, path: "/api/v1/events", body: `{"unknown":true}`, contentType: true,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name:   "missing content type",
			method: http.MethodPost, path: "/api/v1/events", body: validCreateBody,
			wantStatus: http.StatusUnsupportedMediaType, wantCode: "unsupported_media_type",
		},
		{
			name:   "invalid event",
			method: http.MethodPost, path: "/api/v1/events",
			body:        strings.Replace(validCreateBody, `"notify_before_seconds":900`, `"notify_before_seconds":-1`, 1),
			contentType: true, wantStatus: http.StatusBadRequest, wantCode: "invalid_event",
		},
		{
			name:   "event already exists",
			method: http.MethodPost, path: "/api/v1/events", body: validCreateBody, contentType: true,
			appErr: storage.ErrEventAlreadyExists, wantStatus: http.StatusConflict,
			wantCode: "event_already_exists", wantCalls: 1,
		},
		{
			name:   "date busy",
			method: http.MethodPost, path: "/api/v1/events", body: validCreateBody, contentType: true,
			appErr: storage.ErrDateBusy, wantStatus: http.StatusConflict, wantCode: "date_busy", wantCalls: 1,
		},
		{
			name:   "event not found",
			method: http.MethodDelete, path: "/api/v1/events/missing",
			appErr: storage.ErrEventNotFound, wantStatus: http.StatusNotFound,
			wantCode: "event_not_found", wantCalls: 1,
		},
		{
			name:   "invalid date",
			method: http.MethodGet, path: "/api/v1/events/day?user_id=user-1&date=21-08-2026",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name:   "missing generated query parameter",
			method: http.MethodGet, path: "/api/v1/events/day?date=2026-08-21",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name:   "internal error is hidden",
			method: http.MethodGet, path: "/api/v1/events/month?user_id=user-1&date=2026-08-21",
			appErr: internalErr, wantStatus: http.StatusInternalServerError,
			wantCode: "internal_error", wantCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calendar := &applicationStub{err: test.appErr}
			response := serveAPIRequest(
				t,
				calendar,
				test.method,
				test.path,
				test.body,
				test.contentType,
			)

			assertStatus(t, response, test.wantStatus)
			var apiError openapi.Error
			decodeResponse(t, response, &apiError)
			if apiError.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", apiError.Code, test.wantCode)
			}
			if strings.Contains(apiError.Message, "password") {
				t.Fatalf("internal error leaked in response: %q", apiError.Message)
			}
			if calendar.calls != test.wantCalls {
				t.Fatalf("application calls = %d, want %d", calendar.calls, test.wantCalls)
			}
		})
	}
}

func serveAPIRequest(
	t *testing.T,
	calendar Application,
	method string,
	path string,
	body string,
	contentType bool,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	newHTTPHandler(calendar).ServeHTTP(response, request)
	return response
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, want, response.Body.String())
	}
}

func assertEventResponse(t *testing.T, response *httptest.ResponseRecorder, want storage.Event) {
	t.Helper()
	var event openapi.Event
	decodeResponse(t, response, &event)
	if event != eventResponse(want) {
		t.Fatalf("response event = %#v, want %#v", event, eventResponse(want))
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, response.Body.String())
	}
}
