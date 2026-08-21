package internalhttp

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// Server is the HTTP transport for the calendar service.
type Server struct {
	httpServer *http.Server
}

// Logger writes HTTP access log entries.
type Logger interface {
	Info(string)
}

// Application is reserved for calendar use cases exposed by later homeworks.
type Application interface{}

// NewServer creates an HTTP server bound to address.
func NewServer(logger Logger, app Application, address string) *Server {
	_ = app

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           loggingMiddleware(logger, mux),
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
	}
}

// Start serves HTTP requests until the context is cancelled or the server stops.
func (s *Server) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return normalizeServerError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		stopErr := s.Stop(shutdownCtx)
		err := <-serveErr
		if stopErr != nil {
			return stopErr
		}
		return normalizeServerError(err)
	}
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return normalizeServerError(s.httpServer.Shutdown(ctx))
}

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Hello, world!\n"))
}

func normalizeServerError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
