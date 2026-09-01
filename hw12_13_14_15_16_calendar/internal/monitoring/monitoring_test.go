package monitoring

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestAPIMetrics(t *testing.T) {
	metrics := NewAPI()
	metrics.ObserveHTTPRequest(
		http.MethodGet,
		"/api/v1/events/{eventId}",
		http.StatusNotFound,
		250*time.Millisecond,
	)
	metrics.ObserveEventOperation("create", true)
	metrics.ObserveEventOperation("create", false)

	assertMetricValue(t, metrics.registry, "calendar_http_requests_total", map[string]string{
		"method":      http.MethodGet,
		"route":       "/api/v1/events/{eventId}",
		"status_code": "404",
	}, 1)
	assertHistogramCount(t, metrics.registry, "calendar_http_request_duration_seconds", map[string]string{
		"method": http.MethodGet,
		"route":  "/api/v1/events/{eventId}",
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_event_operations_total", map[string]string{
		"operation": "create",
		"result":    resultSuccess,
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_event_operations_total", map[string]string{
		"operation": "create",
		"result":    resultError,
	}, 1)
}

func TestAPIMetricsClampNegativeDuration(t *testing.T) {
	metrics := NewAPI()
	metrics.ObserveHTTPRequest(http.MethodGet, "/hello", http.StatusOK, -time.Second)

	assertHistogramSum(t, metrics.registry, "calendar_http_request_duration_seconds", map[string]string{
		"method": http.MethodGet,
		"route":  "/hello",
	}, 0)
}

func TestSchedulerMetrics(t *testing.T) {
	metrics := NewScheduler()
	finishedAt := time.Unix(1_700_000_000, 0)

	metrics.SetSchedulerRunning(true)
	metrics.ObserveSchedulerCycle(true, finishedAt, 2*time.Second)
	metrics.ObserveSchedulerCycle(false, finishedAt.Add(time.Minute), 3*time.Second)
	metrics.ObserveNotificationPublished(true)
	metrics.ObserveNotificationPublished(false)

	assertMetricValue(t, metrics.registry, "calendar_scheduler_running", nil, 1)
	assertMetricValue(t, metrics.registry, "calendar_scheduler_cycles_total", map[string]string{
		"result": resultSuccess,
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_scheduler_cycles_total", map[string]string{
		"result": resultError,
	}, 1)
	assertHistogramCount(t, metrics.registry, "calendar_scheduler_cycle_duration_seconds", nil, 2)
	assertMetricValue(
		t,
		metrics.registry,
		"calendar_scheduler_last_success_timestamp_seconds",
		nil,
		float64(finishedAt.Unix()),
	)
	assertMetricValue(t, metrics.registry, "calendar_notifications_published_total", map[string]string{
		"result": resultSuccess,
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_notifications_published_total", map[string]string{
		"result": resultError,
	}, 1)

	metrics.SetSchedulerRunning(false)
	assertMetricValue(t, metrics.registry, "calendar_scheduler_running", nil, 0)
}

func TestStorerMetrics(t *testing.T) {
	metrics := NewStorer()
	finishedAt := time.Unix(1_700_000_000, 0)

	metrics.SetStorerRunning(true)
	metrics.ObserveNotificationStored(true, finishedAt)
	metrics.ObserveNotificationStored(false, finishedAt.Add(time.Minute))
	metrics.ObserveInvalidNotification()

	assertMetricValue(t, metrics.registry, "calendar_storer_running", nil, 1)
	assertMetricValue(t, metrics.registry, "calendar_notifications_processed_total", map[string]string{
		"result": resultStored,
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_notifications_processed_total", map[string]string{
		"result": resultError,
	}, 1)
	assertMetricValue(t, metrics.registry, "calendar_notifications_processed_total", map[string]string{
		"result": resultInvalid,
	}, 1)
	assertMetricValue(
		t,
		metrics.registry,
		"calendar_storer_last_success_timestamp_seconds",
		nil,
		float64(finishedAt.Unix()),
	)

	metrics.SetStorerRunning(false)
	assertMetricValue(t, metrics.registry, "calendar_storer_running", nil, 0)
}

func TestMetricsHandlersExposeOpenMetricsAndRuntimeCollectors(t *testing.T) {
	apiMetrics := NewAPI()
	apiMetrics.ObserveHTTPRequest(http.MethodGet, "/hello", http.StatusOK, time.Millisecond)

	tests := []struct {
		name       string
		handler    http.Handler
		metricName string
	}{
		{name: "api", handler: apiMetrics.Handler(), metricName: "calendar_http_requests"},
		{name: "scheduler", handler: NewScheduler().Handler(), metricName: "calendar_scheduler_running"},
		{name: "storer", handler: NewStorer().Handler(), metricName: "calendar_storer_running"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			request.Header.Set("Accept", "application/openmetrics-text")
			response := httptest.NewRecorder()

			test.handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("metrics status = %d, want %d", response.Code, http.StatusOK)
			}
			contentType := response.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/openmetrics-text") {
				t.Fatalf("metrics Content-Type = %q, want OpenMetrics", contentType)
			}
			body := response.Body.String()
			fragments := []string{test.metricName, "go_", "# EOF"}
			if runtime.GOOS == "linux" {
				fragments = append(fragments, "process_")
			}
			for _, fragment := range fragments {
				if !strings.Contains(body, fragment) {
					t.Errorf("metrics body does not contain %q", fragment)
				}
			}
		})
	}
}

func TestMetricsUseIndependentRegistries(t *testing.T) {
	first := NewAPI()
	second := NewAPI()
	first.ObserveEventOperation("delete", true)

	assertMetricValue(t, first.registry, "calendar_event_operations_total", map[string]string{
		"operation": "delete",
		"result":    resultSuccess,
	}, 1)
	if hasMetric(t, second.registry, "calendar_event_operations_total", map[string]string{
		"operation": "delete",
		"result":    resultSuccess,
	}) {
		t.Fatal("independent API registry contains a metric observed in another registry")
	}
}

func assertMetricValue(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
	want float64,
) {
	t.Helper()

	value, found := findMetricValue(t, registry, name, labels)
	if !found {
		t.Fatalf("metric %q with labels %v was not found", name, labels)
	}
	if value != want {
		t.Fatalf("metric %q with labels %v = %v, want %v", name, labels, value, want)
	}
}

func hasMetric(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
) bool {
	t.Helper()

	_, found := findMetricValue(t, registry, name, labels)
	return found
}

func findMetricValue(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
) (float64, bool) {
	t.Helper()

	value, found, err := findMetricValueWithError(registry, name, labels)
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	return value, found
}

func findMetricValueWithError(
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
) (float64, bool, error) {
	families, err := registry.Gather()
	if err != nil {
		return 0, false, err
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !metricHasLabels(metric.GetLabel(), labels) {
				continue
			}
			switch {
			case metric.Counter != nil:
				return metric.GetCounter().GetValue(), true, nil
			case metric.Gauge != nil:
				return metric.GetGauge().GetValue(), true, nil
			}
		}
	}
	return 0, false, nil
}

func metricHasLabels(metricLabels []*dto.LabelPair, want map[string]string) bool {
	if len(metricLabels) != len(want) {
		return false
	}
	for _, label := range metricLabels {
		if want[label.GetName()] != label.GetValue() {
			return false
		}
	}
	return true
}

func assertHistogramCount(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
	want uint64,
) {
	t.Helper()

	histogram := findHistogram(t, registry, name, labels)
	if got := histogram.GetSampleCount(); got != want {
		t.Fatalf("histogram %q count = %d, want %d", name, got, want)
	}
}

func assertHistogramSum(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
	want float64,
) {
	t.Helper()

	histogram := findHistogram(t, registry, name, labels)
	if got := histogram.GetSampleSum(); got != want {
		t.Fatalf("histogram %q sum = %v, want %v", name, got, want)
	}
}

func findHistogram(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	labels map[string]string,
) *dto.Histogram {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricHasLabels(metric.GetLabel(), labels) && metric.Histogram != nil {
				return metric.Histogram
			}
		}
	}
	t.Fatalf("histogram %q with labels %v was not found", name, labels)
	return nil
}
