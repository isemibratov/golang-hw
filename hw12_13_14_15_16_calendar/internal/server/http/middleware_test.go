package internalhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *recordingLogger) Info(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, message)
}

func (l *recordingLogger) lastMessage(t *testing.T) string {
	t.Helper()

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.messages) != 1 {
		t.Fatalf("expected one log message, got %d", len(l.messages))
	}
	return l.messages[0]
}

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.Handler
		wantStatus int
		wantBody   string
	}{
		{
			name:       "implicit status without body",
			handler:    http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			wantStatus: http.StatusOK,
		},
		{
			name: "implicit status on write",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("response")); err != nil {
					t.Errorf("write response: %v", err)
				}
			}),
			wantStatus: http.StatusOK,
			wantBody:   "response",
		},
		{
			name: "explicit status",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			wantStatus: http.StatusNoContent,
		},
		{
			name: "first explicit status wins",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				w.WriteHeader(http.StatusInternalServerError)
			}),
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &recordingLogger{}
			request := newTestRequest(t, http.MethodPost, "http://calendar.test/hello?q=1")
			request.RemoteAddr = "203.0.113.7:4242"
			request.Header.Set("User-Agent", "calendar-test/1.0")
			response := httptest.NewRecorder()

			loggingMiddleware(log, tt.handler).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("unexpected response status: got %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Body.String() != tt.wantBody {
				t.Fatalf("unexpected response body: got %q, want %q", response.Body.String(), tt.wantBody)
			}

			message := log.lastMessage(t)
			for _, fragment := range []string{
				"203.0.113.7",
				"POST",
				"/hello?q=1",
				"HTTP/1.1",
				"calendar-test/1.0",
			} {
				if !strings.Contains(message, fragment) {
					t.Errorf("log entry %q does not contain %q", message, fragment)
				}
			}
			assertTimestamp(t, message)
			if !strings.Contains(message, " "+strconv.Itoa(tt.wantStatus)+" latency=") {
				t.Errorf("log entry %q does not contain status %d", message, tt.wantStatus)
			}
			assertLatency(t, message)
		})
	}
}

func TestLoggingMiddlewareSanitizesControlCharacters(t *testing.T) {
	log := &recordingLogger{}
	request := newTestRequest(t, http.MethodGet, "http://calendar.test/hello")
	request.RequestURI = "/hello\nforged-entry"
	response := httptest.NewRecorder()

	loggingMiddleware(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(response, request)

	message := log.lastMessage(t)
	if strings.Contains(message, "\n") {
		t.Fatalf("log entry contains a raw newline: %q", message)
	}
	if !strings.Contains(message, `/hello\nforged-entry`) {
		t.Fatalf("log entry does not contain sanitized URI: %q", message)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name          string
		remoteAddress string
		want          string
	}{
		{name: "IPv4 with port", remoteAddress: "192.0.2.1:8080", want: "192.0.2.1"},
		{name: "IPv6 with port", remoteAddress: "[2001:db8::1]:8080", want: "2001:db8::1"},
		{name: "address without port", remoteAddress: "192.0.2.1", want: "192.0.2.1"},
		{name: "empty address", want: unknownClientIP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientIP(tt.remoteAddress); got != tt.want {
				t.Fatalf("unexpected client IP: got %q, want %q", got, tt.want)
			}
		})
	}
}

func assertLatency(t *testing.T, message string) {
	t.Helper()

	const prefix = "latency="
	position := strings.Index(message, prefix)
	if position == -1 {
		t.Fatalf("log entry %q does not contain latency", message)
	}
	fields := strings.Fields(message[position+len(prefix):])
	if len(fields) == 0 {
		t.Fatalf("log entry %q contains an empty latency", message)
	}
	latency := fields[0]
	if _, err := time.ParseDuration(latency); err != nil {
		t.Fatalf("log entry contains invalid latency %q: %v", latency, err)
	}
}

func newTestRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	request, err := http.NewRequestWithContext(context.Background(), method, target, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return request
}

func assertTimestamp(t *testing.T, message string) {
	t.Helper()

	start := strings.Index(message, "[")
	end := strings.Index(message, "]")
	if start == -1 || end <= start {
		t.Fatalf("log entry %q does not contain a timestamp", message)
	}
	if _, err := time.Parse("02/Jan/2006:15:04:05 -0700", message[start+1:end]); err != nil {
		t.Fatalf("log entry contains invalid timestamp: %v", err)
	}
}
