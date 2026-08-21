package internalhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	openapi "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/server/http/openapi"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage"
)

const (
	calendarDateLayout = "2006-01-02"
	maxRequestBodySize = 1 << 20
)

var errUnsupportedMediaType = errors.New("unsupported media type")

type apiHandler struct {
	app Application
}

var _ openapi.ServerInterface = (*apiHandler)(nil)

func newHTTPHandler(app Application) http.Handler {
	router := chi.NewRouter()
	router.Get("/hello", helloHandler)

	return openapi.HandlerWithOptions(&apiHandler{app: app}, openapi.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeError(w, http.StatusBadRequest, "invalid_request", "request parameters are invalid")
		},
	})
}

// CreateEvent handles POST /api/v1/events.
func (h *apiHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreateEventRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	event, err := newStorageEvent(
		request.Id,
		request.Title,
		request.StartAt,
		request.EndAt,
		request.Description,
		request.UserId,
		request.NotifyBeforeSeconds,
	)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	if err = h.app.CreateEvent(r.Context(), event); err != nil {
		writeApplicationError(w, err)
		return
	}

	w.Header().Set("Location", "/api/v1/events/"+url.PathEscape(event.ID))
	writeJSON(w, http.StatusCreated, eventResponse(event))
}

// UpdateEvent handles PUT /api/v1/events/{eventId}.
func (h *apiHandler) UpdateEvent(w http.ResponseWriter, r *http.Request, eventID openapi.EventID) {
	var request openapi.UpdateEventRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	event, err := newStorageEvent(
		eventID,
		request.Title,
		request.StartAt,
		request.EndAt,
		request.Description,
		request.UserId,
		request.NotifyBeforeSeconds,
	)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	if err = h.app.UpdateEvent(r.Context(), event); err != nil {
		writeApplicationError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, eventResponse(event))
}

// DeleteEvent handles DELETE /api/v1/events/{eventId}.
func (h *apiHandler) DeleteEvent(w http.ResponseWriter, r *http.Request, eventID openapi.EventID) {
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	if err := h.app.DeleteEvent(r.Context(), eventID); err != nil {
		writeApplicationError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListEventsForDay handles GET /api/v1/events/day.
func (h *apiHandler) ListEventsForDay(
	w http.ResponseWriter,
	r *http.Request,
	params openapi.ListEventsForDayParams,
) {
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	h.listEvents(w, r, params.UserId, params.Date, h.app.ListEventsForDay)
}

// ListEventsForWeek handles GET /api/v1/events/week.
func (h *apiHandler) ListEventsForWeek(
	w http.ResponseWriter,
	r *http.Request,
	params openapi.ListEventsForWeekParams,
) {
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	h.listEvents(w, r, params.UserId, params.Date, h.app.ListEventsForWeek)
}

// ListEventsForMonth handles GET /api/v1/events/month.
func (h *apiHandler) ListEventsForMonth(
	w http.ResponseWriter,
	r *http.Request,
	params openapi.ListEventsForMonthParams,
) {
	if h.app == nil {
		writeApplicationError(w, errors.New("calendar application is not configured"))
		return
	}
	h.listEvents(w, r, params.UserId, params.Date, h.app.ListEventsForMonth)
}

func (h *apiHandler) listEvents(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	dateValue string,
	list func(context.Context, string, time.Time) ([]storage.Event, error),
) {
	date, err := time.Parse(calendarDateLayout, dateValue)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "date must use YYYY-MM-DD format")
		return
	}
	if strings.TrimSpace(userID) == "" {
		writeApplicationError(w, storage.ErrInvalidEvent)
		return
	}
	events, err := list(r.Context(), userID, date)
	if err != nil {
		writeApplicationError(w, err)
		return
	}

	response := make([]openapi.Event, 0, len(events))
	for _, event := range events {
		response = append(response, eventResponse(event))
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if err := decodeJSON(w, r, target); err != nil {
		if errors.Is(err, errUnsupportedMediaType) {
			writeError(
				w,
				http.StatusUnsupportedMediaType,
				"unsupported_media_type",
				"Content-Type must be application/json",
			)
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain one valid JSON object")
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errUnsupportedMediaType
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON request: %w", err)
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing JSON data: %w", err)
	}
	return nil
}

func newStorageEvent(
	id string,
	title string,
	startAt time.Time,
	endAt time.Time,
	description *string,
	userID string,
	notifyBeforeSeconds *int64,
) (storage.Event, error) {
	notifyBefore, err := notificationDuration(notifyBeforeSeconds)
	if err != nil {
		return storage.Event{}, err
	}

	event := storage.Event{
		ID:           id,
		Title:        title,
		StartAt:      startAt,
		EndAt:        endAt,
		UserID:       userID,
		NotifyBefore: notifyBefore,
	}
	if description != nil {
		event.Description = *description
	}
	if err = storage.ValidateEvent(event); err != nil {
		return storage.Event{}, err
	}
	return event, nil
}

func notificationDuration(seconds *int64) (time.Duration, error) {
	if seconds == nil {
		return 0, nil
	}
	const maxSeconds = int64(math.MaxInt64) / int64(time.Second)
	if *seconds < 0 || *seconds > maxSeconds {
		return 0, storage.ErrInvalidEvent
	}
	return time.Duration(*seconds) * time.Second, nil
}

func eventResponse(event storage.Event) openapi.Event {
	return openapi.Event{
		Description:         event.Description,
		EndAt:               event.EndAt,
		Id:                  event.ID,
		NotifyBeforeSeconds: int64(event.NotifyBefore / time.Second),
		StartAt:             event.StartAt,
		Title:               event.Title,
		UserId:              event.UserID,
	}
}

func writeApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrInvalidEvent):
		writeError(w, http.StatusBadRequest, "invalid_event", storage.ErrInvalidEvent.Error())
	case errors.Is(err, storage.ErrEventNotFound):
		writeError(w, http.StatusNotFound, "event_not_found", storage.ErrEventNotFound.Error())
	case errors.Is(err, storage.ErrEventAlreadyExists):
		writeError(w, http.StatusConflict, "event_already_exists", storage.ErrEventAlreadyExists.Error())
	case errors.Is(err, storage.ErrDateBusy):
		writeError(w, http.StatusConflict, "date_busy", storage.ErrDateBusy.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, openapi.Error{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
