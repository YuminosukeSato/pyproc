package pyproc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandler_AlwaysReturns200(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}

	handler := LivenessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

func TestLivenessHandler_ContentType(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}

	handler := LivenessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestLivenessHandler_ShutdownPool(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}
	pool.shutdown.Store(true)

	handler := LivenessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Liveness should always return 200, even when shutting down
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 even during shutdown, got %d", rec.Code)
	}
}

func TestReadinessHandler_HealthyWorkers(t *testing.T) {
	pool := &Pool{
		workers: make([]*poolWorker, 2),
	}
	pool.healthMu.Lock()
	pool.healthStatus = HealthStatus{
		TotalWorkers:   2,
		HealthyWorkers: 2,
	}
	pool.healthMu.Unlock()

	handler := ReadinessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("expected status=ready, got %v", body["status"])
	}
}

func TestReadinessHandler_NoHealthyWorkers(t *testing.T) {
	pool := &Pool{
		workers: make([]*poolWorker, 2),
	}
	pool.healthMu.Lock()
	pool.healthStatus = HealthStatus{
		TotalWorkers:   2,
		HealthyWorkers: 0,
	}
	pool.healthMu.Unlock()

	handler := ReadinessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Errorf("expected status=not_ready, got %v", body["status"])
	}
}

func TestReadinessHandler_ShutdownPool(t *testing.T) {
	pool := &Pool{
		workers: make([]*poolWorker, 2),
	}
	pool.healthMu.Lock()
	pool.healthStatus = HealthStatus{
		TotalWorkers:   2,
		HealthyWorkers: 2,
	}
	pool.healthMu.Unlock()
	pool.shutdown.Store(true)

	handler := ReadinessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 during shutdown, got %d", rec.Code)
	}
}

func TestStartupHandler_NotStarted(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}

	handler := StartupHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 before start, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "not_started" {
		t.Errorf("expected status=not_started, got %q", body["status"])
	}
}

func TestStartupHandler_Started(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}
	pool.started.Store(true)

	handler := StartupHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 after start, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "started" {
		t.Errorf("expected status=started, got %q", body["status"])
	}
}

func TestReadinessHandler_ContentType(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}

	handler := ReadinessHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestStartupHandler_ContentType(t *testing.T) {
	pool := &Pool{workers: make([]*poolWorker, 0)}

	handler := StartupHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
