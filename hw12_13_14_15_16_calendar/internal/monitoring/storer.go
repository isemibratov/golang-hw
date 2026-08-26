package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	resultStored  = "stored"
	resultInvalid = "invalid"
)

// Storer contains metrics emitted by the notification storer process.
type Storer struct {
	registry             *prometheus.Registry
	running              prometheus.Gauge
	notifications        *prometheus.CounterVec
	lastSuccessTimestamp prometheus.Gauge
}

// NewStorer creates an independent metrics registry for one storer process.
func NewStorer() *Storer {
	registry := newRegistry()
	metrics := &Storer{
		registry: registry,
		running: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "calendar",
			Subsystem: "storer",
			Name:      "running",
			Help:      "Whether the calendar storer loop is running (1 for running, 0 for stopped).",
		}),
		notifications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "calendar",
			Name:      "notifications_processed_total",
			Help:      "Total number of notifications processed by the calendar storer.",
		}, []string{"result"}),
		lastSuccessTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "calendar",
			Subsystem: "storer",
			Name:      "last_success_timestamp_seconds",
			Help:      "Unix timestamp of the last notification successfully stored.",
		}),
	}

	registry.MustRegister(metrics.running, metrics.notifications, metrics.lastSuccessTimestamp)
	return metrics
}

// Handler returns the Prometheus exposition handler for the storer registry.
func (m *Storer) Handler() http.Handler {
	return metricsHandler(m.registry)
}

// SetStorerRunning records whether the storer consumer loop is active.
func (m *Storer) SetStorerRunning(running bool) {
	if running {
		m.running.Set(1)
		return
	}
	m.running.Set(0)
}

// ObserveNotificationStored records one attempt to persist a valid notification.
func (m *Storer) ObserveNotificationStored(success bool, finishedAt time.Time) {
	operationResult := resultError
	if success {
		operationResult = resultStored
	}
	m.notifications.WithLabelValues(operationResult).Inc()
	if success && !finishedAt.IsZero() {
		m.lastSuccessTimestamp.Set(float64(finishedAt.Unix()))
	}
}

// ObserveInvalidNotification records one rejected notification payload.
func (m *Storer) ObserveInvalidNotification() {
	m.notifications.WithLabelValues(resultInvalid).Inc()
}
