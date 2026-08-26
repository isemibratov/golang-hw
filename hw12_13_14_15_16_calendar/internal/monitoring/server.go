package monitoring

import (
	"context"
	"errors"
	"fmt"
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

// Server exposes a single Prometheus metrics endpoint.
type Server struct {
	httpServer *http.Server
}

// NewServer creates a metrics HTTP server bound to address.
func NewServer(address string, handler http.Handler) *Server {
	if handler == nil {
		handler = http.NotFoundHandler()
	}

	router := http.NewServeMux()
	router.Handle("/metrics", handler)
	return &Server{httpServer: &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}}
}

// Start serves metrics until the context is cancelled or the server stops.
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
		serveErrValue := <-serveErr
		if stopErr != nil {
			return stopErr
		}
		return normalizeServerError(serveErrValue)
	}
}

// Stop gracefully shuts down the metrics HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return normalizeServerError(s.httpServer.Shutdown(ctx))
}

// RunWithServer runs a background process and its metrics server as one unit.
// When either component stops, the other one is cancelled and awaited.
func RunWithServer(
	ctx context.Context,
	process func(context.Context) error,
	metricsServer *Server,
) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	processErr := make(chan error, 1)
	go func() {
		processErr <- process(runCtx)
	}()
	metricsErr := make(chan error, 1)
	go func() {
		metricsErr <- metricsServer.Start(runCtx)
	}()

	select {
	case err := <-processErr:
		cancel()
		serverErr := <-metricsErr
		if err != nil {
			return err
		}
		return serverErr
	case err := <-metricsErr:
		cancel()
		serviceErr := <-processErr
		if err != nil {
			return fmt.Errorf("run metrics server: %w", err)
		}
		return serviceErr
	}
}

func normalizeServerError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
