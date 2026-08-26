package internalhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/monitoring"
)

func TestMetricsEndpointRecordsHTTPAndBusinessScenarios(t *testing.T) {
	calendar := &applicationStub{}
	metrics := monitoring.NewAPI()
	handler := newHTTPHandler(calendar, metrics)

	requests := []struct {
		method      string
		path        string
		body        string
		contentType bool
		wantStatus  int
	}{
		{
			method: http.MethodPost, path: "/api/v1/events", body: validCreateBody,
			contentType: true, wantStatus: http.StatusCreated,
		},
		{
			method: http.MethodPut, path: "/api/v1/events/private-event-id", body: validUpdateBody,
			contentType: true, wantStatus: http.StatusOK,
		},
		{
			method: http.MethodDelete, path: "/api/v1/events/private-event-id",
			wantStatus: http.StatusNoContent,
		},
		{
			method: http.MethodGet, path: "/api/v1/events/day?user_id=user-1&date=2026-08-21",
			wantStatus: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/api/v1/events/week?user_id=user-1&date=2026-08-21",
			wantStatus: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/api/v1/events/month?user_id=user-1&date=2026-08-21",
			wantStatus: http.StatusOK,
		},
		{
			method: http.MethodPost, path: "/api/v1/events", body: "{",
			contentType: true, wantStatus: http.StatusBadRequest,
		},
		{
			method: "CUSTOM", path: "/not-found", wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, request := range requests {
		response := serveMetricsRequest(
			t,
			handler,
			request.method,
			request.path,
			request.body,
			request.contentType,
		)
		if response.Code != request.wantStatus {
			t.Fatalf("%s %s status = %d, want %d; body: %s",
				request.method, request.path, response.Code, request.wantStatus, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/openmetrics-text")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d; body: %s",
			response.Code, http.StatusOK, response.Body.String())
	}
	contentType := response.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/openmetrics-text") {
		t.Fatalf("GET /metrics Content-Type = %q, want OpenMetrics", contentType)
	}

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, request.Clone(request.Context()))
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second GET /metrics status = %d, want %d", secondResponse.Code, http.StatusOK)
	}
	body := secondResponse.Body.String()
	wantMetrics := []string{
		`calendar_http_requests_total{method="POST",route="/api/v1/events",status_code="201"} 1`,
		`calendar_http_requests_total{method="POST",route="/api/v1/events",status_code="400"} 1`,
		`calendar_http_requests_total{method="PUT",route="/api/v1/events/{eventId}",status_code="200"} 1`,
		`calendar_http_requests_total{method="OTHER",route="unmatched",status_code="405"} 1`,
		`calendar_http_request_duration_seconds_count{method="GET",route="/api/v1/events/month"} 1`,
		`calendar_event_operations_total{operation="create",result="success"} 1`,
		`calendar_event_operations_total{operation="create",result="error"} 1`,
		`calendar_event_operations_total{operation="update",result="success"} 1`,
		`calendar_event_operations_total{operation="delete",result="success"} 1`,
		`calendar_event_operations_total{operation="list_day",result="success"} 1`,
		`calendar_event_operations_total{operation="list_week",result="success"} 1`,
		`calendar_event_operations_total{operation="list_month",result="success"} 1`,
	}
	for _, want := range wantMetrics {
		if !strings.Contains(body, want) {
			t.Errorf("GET /metrics body does not contain %q\nbody:\n%s", want, body)
		}
	}
	if strings.Contains(body, `route="/metrics"`) {
		t.Fatalf("metrics scrape requests must not affect API metrics:\n%s", body)
	}
	if strings.Contains(body, "private-event-id") || strings.Contains(body, "user-1") {
		t.Fatalf("metrics expose a high-cardinality event or user identifier:\n%s", body)
	}
}

func serveMetricsRequest(
	t *testing.T,
	handler http.Handler,
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
	handler.ServeHTTP(response, request)
	return response
}
