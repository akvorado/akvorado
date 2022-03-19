package metrics_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"akvorado/helpers"
	"akvorado/reporter/logger"
	"akvorado/reporter/metrics"
)

func TestNew(t *testing.T) {
	l, err := logger.New(logger.DefaultConfiguration)
	if err != nil {
		t.Fatalf("logger.New() err:\n%+v", err)
	}
	m, err := metrics.New(l, metrics.DefaultConfiguration)
	if err != nil {
		t.Fatalf("metrics.New() err:\n%+v", err)
	}

	counter := m.Factory(0).NewCounter(prometheus.CounterOpts{
		Name: "counter1",
		Help: "Some counter",
	})
	counter.Add(18)

	gauge := m.Factory(0).NewGauge(prometheus.GaugeOpts{
		Name: "gauge1",
		Help: "Some gauge",
	})
	gauge.Set(4)

	// Use the HTTP handler for testing
	req := httptest.NewRequest("GET", "/api/v0/metrics", nil)
	w := httptest.NewRecorder()
	m.HTTPHandler().ServeHTTP(w, req)
	got := strings.Split(w.Body.String(), "\n")

	// We expect some go_* and process_* gauges
	for _, expected := range []string{"process_open_fds", "go_threads", "go_sched_goroutines_goroutines"} {
		found := false
		for _, line := range got {
			if line == fmt.Sprintf("# TYPE %s gauge", expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GET /api/v0/metrics missing: %s", expected)
		}
	}

	// Otherwise, we expect the metrics we have defined:
	gotFiltered := []string{}
	for _, line := range got {
		if strings.Contains(line, "go_") || strings.Contains(line, "process_") {
			continue
		}
		gotFiltered = append(gotFiltered, line)
	}
	expected := []string{
		"# HELP akvorado_reporter_metrics_test_counter1 Some counter",
		"# TYPE akvorado_reporter_metrics_test_counter1 counter",
		"akvorado_reporter_metrics_test_counter1 18",
		"# HELP akvorado_reporter_metrics_test_gauge1 Some gauge",
		"# TYPE akvorado_reporter_metrics_test_gauge1 gauge",
		"akvorado_reporter_metrics_test_gauge1 4",
		"",
	}
	if diff := helpers.Diff(gotFiltered, expected); diff != "" {
		t.Fatalf("GET /api/v0/metrics (-got, +want):\n%s", diff)
	}
}

func TestFactoryCache(t *testing.T) {
	l, err := logger.New(logger.DefaultConfiguration)
	if err != nil {
		t.Fatalf("logger.New() err:\n%+v", err)
	}
	m, err := metrics.New(l, metrics.DefaultConfiguration)
	if err != nil {
		t.Fatalf("metrics.New() err:\n%+v", err)
	}

	factory1 := m.Factory(0)
	factory2 := m.Factory(0)
	if factory1 != factory2 {
		t.Fatalf("Factory caching not working as expected")
	}
}
