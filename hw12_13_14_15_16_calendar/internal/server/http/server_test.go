package internalhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServer(t *testing.T) {
	log := &recordingLogger{}
	server := NewServer(log, nil, "127.0.0.1:8888")

	if server.httpServer.Addr != "127.0.0.1:8888" {
		t.Fatalf("unexpected server address: %q", server.httpServer.Addr)
	}
	if server.httpServer.ReadHeaderTimeout != readHeaderTimeout ||
		server.httpServer.ReadTimeout != readTimeout ||
		server.httpServer.WriteTimeout != writeTimeout ||
		server.httpServer.IdleTimeout != idleTimeout {
		t.Fatal("HTTP server safety limits are not configured")
	}

	request := newTestRequest(t, http.MethodGet, "http://calendar.test/hello")
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if body := response.Body.String(); body != "Hello, world!\n" {
		t.Fatalf("unexpected response body: %q", body)
	}
}
