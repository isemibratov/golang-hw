package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerExposesOnlyMetrics(t *testing.T) {
	metrics := NewAPI()
	server := NewServer("127.0.0.1:0", metrics.Handler())

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", response.Code, http.StatusOK)
	}

	request = httptest.NewRequest(http.MethodGet, "/hello", nil)
	response = httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected route status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestServerConfiguresSafetyTimeouts(t *testing.T) {
	server := NewServer("127.0.0.1:0", NewAPI().Handler())
	if server.httpServer.ReadHeaderTimeout != readHeaderTimeout ||
		server.httpServer.ReadTimeout != readTimeout ||
		server.httpServer.WriteTimeout != writeTimeout ||
		server.httpServer.IdleTimeout != idleTimeout {
		t.Fatal("metrics HTTP server safety timeouts are not configured")
	}
}

func TestServerStartStopsForCancelledContext(t *testing.T) {
	server := NewServer("127.0.0.1:0", NewAPI().Handler())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
}

func TestServerStartReturnsListenError(t *testing.T) {
	server := NewServer("127.0.0.1:not-a-port", NewAPI().Handler())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Start(ctx); err == nil {
		t.Fatal("Start() error = nil, want listen error")
	}
}

func TestServerStopBeforeStart(t *testing.T) {
	server := NewServer("127.0.0.1:0", NewAPI().Handler())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}
}

func TestRunWithServerPropagatesProcessError(t *testing.T) {
	wantErr := errors.New("process failed")
	server := NewServer("127.0.0.1:0", NewScheduler().Handler())

	err := RunWithServer(context.Background(), func(context.Context) error {
		return wantErr
	}, server)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunWithServer() error = %v, want %v", err, wantErr)
	}
}

func TestRunWithServerPropagatesMetricsListenError(t *testing.T) {
	server := NewServer("127.0.0.1:not-a-port", NewStorer().Handler())

	err := RunWithServer(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, server)
	if err == nil || !strings.Contains(err.Error(), "run metrics server") {
		t.Fatalf("RunWithServer() error = %v, want metrics server error", err)
	}
}

func TestRunWithServerStopsForCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := NewServer("127.0.0.1:0", NewScheduler().Handler())

	err := RunWithServer(ctx, func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, server)
	if err != nil {
		t.Fatalf("RunWithServer() error = %v, want nil", err)
	}
}
