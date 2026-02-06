package pyproc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsHandler(t *testing.T) {
	pm := &PoolWithMetrics{
		Pool:    &Pool{workers: make([]*poolWorker, 2)},
		metrics: NewPoolMetrics(),
	}
	// Populate some metrics
	pm.metrics.RequestsTotal.Store(10)
	pm.metrics.RequestsSucceeded.Store(8)
	pm.metrics.RequestsFailed.Store(1)
	pm.metrics.RequestsTimeout.Store(1)
	pm.metrics.WorkerRestarts.Store(2)
	pm.metrics.QueueDepth.Store(3)
	pm.metrics.RecordLatency(50 * time.Millisecond)
	pm.metrics.RecordLatency(100 * time.Millisecond)
	pm.metrics.RecordLatency(200 * time.Millisecond)

	handler := MetricsHandler(pm)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("unexpected Content-Type: %s", ct)
	}

	body := rec.Body.String()

	// Check required metrics are present
	required := []string{
		"pyproc_requests_total",
		"pyproc_request_duration_seconds",
		"pyproc_workers_total",
		"pyproc_workers_healthy",
		"pyproc_inflight_requests",
		"pyproc_worker_restarts_total",
	}
	for _, metric := range required {
		if !strings.Contains(body, metric) {
			t.Errorf("missing metric: %s", metric)
		}
	}

	// Check specific values
	if !strings.Contains(body, `pyproc_requests_total{status="success"} 8`) {
		t.Error("expected success count 8")
	}
	if !strings.Contains(body, `pyproc_requests_total{status="failed"} 1`) {
		t.Error("expected failed count 1")
	}
	if !strings.Contains(body, "pyproc_inflight_requests 3") {
		t.Error("expected inflight 3")
	}
	if !strings.Contains(body, "pyproc_worker_restarts_total 2") {
		t.Error("expected restarts 2")
	}
}

func TestMetricsHandlerEmpty(t *testing.T) {
	pm := &PoolWithMetrics{
		Pool:    &Pool{workers: make([]*poolWorker, 0)},
		metrics: NewPoolMetrics(),
	}

	handler := MetricsHandler(pm)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestGetLatencyPercentileSorted(t *testing.T) {
	m := NewPoolMetrics()
	// Insert in reverse order to verify sorting
	m.RecordLatency(300 * time.Millisecond)
	m.RecordLatency(100 * time.Millisecond)
	m.RecordLatency(200 * time.Millisecond)

	p50 := m.GetLatencyPercentile(50)
	// With 3 items sorted [100ms, 200ms, 300ms], index = (3-1)*50/100 = 1 → 200ms
	if p50 != 200*time.Millisecond {
		t.Fatalf("expected p50=200ms after sort, got %v", p50)
	}

	p0 := m.GetLatencyPercentile(0)
	if p0 != 100*time.Millisecond {
		t.Fatalf("expected p0=100ms, got %v", p0)
	}

	p100 := m.GetLatencyPercentile(100)
	if p100 != 300*time.Millisecond {
		t.Fatalf("expected p100=300ms, got %v", p100)
	}
}
