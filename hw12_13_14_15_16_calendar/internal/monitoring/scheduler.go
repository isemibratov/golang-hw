package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Scheduler contains metrics emitted by the calendar scheduler process.
type Scheduler struct {
	registry               *prometheus.Registry
	running                prometheus.Gauge
	cycles                 *prometheus.CounterVec
	cycleDuration          prometheus.Histogram
	lastSuccessTimestamp   prometheus.Gauge
	notificationsPublished *prometheus.CounterVec
}

// NewScheduler creates an independent metrics registry for one scheduler process.
func NewScheduler() *Scheduler {
	registry := newRegistry()
	metrics := &Scheduler{
		registry: registry,
		running: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "calendar",
			Subsystem: "scheduler",
			Name:      "running",
			Help:      "Whether the calendar scheduler loop is running (1 for running, 0 for stopped).",
		}),
		cycles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "calendar",
			Subsystem: "scheduler",
			Name:      "cycles_total",
			Help:      "Total number of completed calendar scheduler cycles.",
		}, []string{"result"}),
		cycleDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "calendar",
			Subsystem: "scheduler",
			Name:      "cycle_duration_seconds",
			Help:      "Calendar scheduler cycle duration in seconds.",
		}),
		lastSuccessTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "calendar",
			Subsystem: "scheduler",
			Name:      "last_success_timestamp_seconds",
			Help:      "Unix timestamp of the last successful calendar scheduler cycle.",
		}),
		notificationsPublished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "calendar",
			Name:      "notifications_published_total",
			Help:      "Total number of notification publication attempts.",
		}, []string{"result"}),
	}

	registry.MustRegister(
		metrics.running,
		metrics.cycles,
		metrics.cycleDuration,
		metrics.lastSuccessTimestamp,
		metrics.notificationsPublished,
	)
	return metrics
}

// Handler returns the Prometheus exposition handler for the scheduler registry.
func (m *Scheduler) Handler() http.Handler {
	return metricsHandler(m.registry)
}

// SetSchedulerRunning records whether the scheduler loop is active.
func (m *Scheduler) SetSchedulerRunning(running bool) {
	if running {
		m.running.Set(1)
		return
	}
	m.running.Set(0)
}

// ObserveSchedulerCycle records one completed scheduler cycle.
func (m *Scheduler) ObserveSchedulerCycle(success bool, finishedAt time.Time, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}

	m.cycles.WithLabelValues(result(success)).Inc()
	m.cycleDuration.Observe(duration.Seconds())
	if success && !finishedAt.IsZero() {
		m.lastSuccessTimestamp.Set(float64(finishedAt.Unix()))
	}
}

// ObserveNotificationPublished records one notification publication attempt.
func (m *Scheduler) ObserveNotificationPublished(success bool) {
	m.notificationsPublished.WithLabelValues(result(success)).Inc()
}
