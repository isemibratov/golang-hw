package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// API contains metrics emitted by the calendar HTTP API.
type API struct {
	registry        *prometheus.Registry
	requests        *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	eventOperations *prometheus.CounterVec
}

// NewAPI creates an independent metrics registry for one calendar API process.
func NewAPI() *API {
	registry := newRegistry()
	metrics := &API{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "calendar",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of calendar HTTP requests.",
		}, []string{"method", "route", "status_code"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "calendar",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Calendar HTTP request processing duration in seconds.",
		}, []string{"method", "route"}),
		eventOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "calendar",
			Subsystem: "event",
			Name:      "operations_total",
			Help:      "Total number of calendar event operations.",
		}, []string{"operation", "result"}),
	}

	registry.MustRegister(metrics.requests, metrics.requestDuration, metrics.eventOperations)
	return metrics
}

// Handler returns the Prometheus exposition handler for the API registry.
func (m *API) Handler() http.Handler {
	return metricsHandler(m.registry)
}

// ObserveHTTPRequest records one completed HTTP request.
func (m *API) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}

	m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.requestDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// ObserveEventOperation records the outcome of one calendar event operation.
func (m *API) ObserveEventOperation(operation string, success bool) {
	m.eventOperations.WithLabelValues(operation, result(success)).Inc()
}
