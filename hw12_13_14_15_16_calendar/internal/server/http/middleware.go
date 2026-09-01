// Package internalhttp provides the calendar HTTP transport and access logging.
package internalhttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const unknownClientIP = "-"

const unmatchedRoute = "unmatched"

const otherHTTPMethod = "OTHER"

// Metrics records HTTP and calendar business-operation measurements.
type Metrics interface {
	Handler() http.Handler
	ObserveHTTPRequest(method, route string, status int, duration time.Duration)
	ObserveEventOperation(operation string, success bool)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func loggingMiddleware(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := &statusWriter{ResponseWriter: w}

		defer func() {
			if logger == nil {
				return
			}

			requestURI := r.RequestURI
			if requestURI == "" {
				requestURI = r.URL.RequestURI()
			}

			message := fmt.Sprintf(
				"%s [%s] %s %s %s %d latency=%s",
				clientIP(r.RemoteAddr),
				startedAt.Format("02/Jan/2006:15:04:05 -0700"),
				sanitizeLogField(r.Method),
				sanitizeLogField(requestURI),
				sanitizeLogField(r.Proto),
				writer.statusCode(),
				time.Since(startedAt).Round(time.Microsecond),
			)
			if userAgent := r.UserAgent(); userAgent != "" {
				message += fmt.Sprintf(" %q", userAgent)
			}

			logger.Info(message)
		}()

		next.ServeHTTP(writer, r)
	})
}

func metricsMiddleware(metrics Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			startedAt := time.Now()
			writer := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(writer, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = unmatchedRoute
			}
			status := writer.statusCode()
			metrics.ObserveHTTPRequest(metricsMethod(r.Method), route, status, time.Since(startedAt))
			if operation := eventOperation(r.Method, route); operation != "" {
				metrics.ObserveEventOperation(operation, status >= http.StatusOK && status < http.StatusMultipleChoices)
			}
		})
	}
}

func metricsMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
		return method
	default:
		return otherHTTPMethod
	}
}

func eventOperation(method, route string) string {
	switch {
	case method == http.MethodPost && route == "/api/v1/events":
		return "create"
	case method == http.MethodPut && route == "/api/v1/events/{eventId}":
		return "update"
	case method == http.MethodDelete && route == "/api/v1/events/{eventId}":
		return "delete"
	case method == http.MethodGet && route == "/api/v1/events/day":
		return "list_day"
	case method == http.MethodGet && route == "/api/v1/events/week":
		return "list_week"
	case method == http.MethodGet && route == "/api/v1/events/month":
		return "list_month"
	default:
		return ""
	}
}

func clientIP(remoteAddress string) string {
	if remoteAddress == "" {
		return unknownClientIP
	}

	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		return host
	}
	return sanitizeLogField(remoteAddress)
}

func sanitizeLogField(value string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(value)
}
