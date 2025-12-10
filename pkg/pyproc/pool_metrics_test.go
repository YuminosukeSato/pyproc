package pyproc

import (
	"testing"
	"time"
)

func TestPoolMetricsLatency(t *testing.T) {
	metrics := NewPoolMetrics()
	metrics.RecordLatency(10 * time.Millisecond)
	metrics.RecordLatency(20 * time.Millisecond)
	metrics.RecordLatency(30 * time.Millisecond)

	p50 := metrics.GetLatencyPercentile(50)
	if p50 <= 0 {
		t.Fatalf("expected latency percentile > 0, got %v", p50)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	metrics := NewPoolMetrics()
	metrics.ConnectionsCreated.Store(2)
	metrics.ConnectionsDestroyed.Store(1)
	metrics.ConnectionsActive.Store(1)
	metrics.ConnectionsIdle.Store(1)
	metrics.RequestsTotal.Store(3)
	metrics.RequestsSucceeded.Store(2)
	metrics.RequestsFailed.Store(1)
	metrics.RequestsTimeout.Store(0)
	metrics.WorkerRestarts.Store(1)
	metrics.WorkerFailures.Store(0)
	metrics.RecordLatency(5 * time.Millisecond)
	metrics.PoolUtilization.Store(75)
	metrics.QueueDepth.Store(2)

	snap := metrics.GetMetricsSnapshot()
	if snap.ConnectionsCreated != 2 || snap.RequestsTotal != 3 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.PoolUtilization != 0.75 {
		t.Fatalf("expected utilization 0.75, got %f", snap.PoolUtilization)
	}
}

func TestPoolWithMetricsReset(t *testing.T) {
	pm := &PoolWithMetrics{Pool: &Pool{}, metrics: NewPoolMetrics()}
	pm.metrics.RequestsTotal.Store(5)
	pm.ResetMetrics()
	if pm.metrics.RequestsTotal.Load() != 0 {
		t.Fatalf("metrics were not reset")
	}
	snap := pm.GetMetrics()
	if snap.Timestamp.IsZero() {
		t.Fatalf("timestamp should be set in snapshot")
	}
}

func TestNewPoolWithMetrics_InvalidConfig(t *testing.T) {
	opts := PoolOptions{Config: PoolConfig{Workers: 0}}
	_, err := NewPoolWithMetrics(opts, nil)
	if err == nil {
		t.Error("expected error for zero workers")
	}
}

func TestNewPoolWithMetrics_Valid(t *testing.T) {
	opts := PoolOptions{
		Config:       PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{SocketPath: "/tmp/test-metrics.sock"},
	}
	pm, err := NewPoolWithMetrics(opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pm.Pool == nil || pm.metrics == nil {
		t.Error("pool or metrics is nil")
	}
}

func TestRecordLatency_Empty(t *testing.T) {
	metrics := NewPoolMetrics()
	p50 := metrics.GetLatencyPercentile(50)
	if p50 != 0 {
		t.Errorf("expected 0 for empty latencies, got %v", p50)
	}
}

func TestRecordLatency_Percentiles(t *testing.T) {
	metrics := NewPoolMetrics()
	for i := 1; i <= 100; i++ {
		metrics.RecordLatency(time.Duration(i) * time.Millisecond)
	}
	p50 := metrics.GetLatencyPercentile(50)
	p99 := metrics.GetLatencyPercentile(99)
	if p50 < 40*time.Millisecond || p50 > 60*time.Millisecond {
		t.Errorf("p50 out of range: %v", p50)
	}
	if p99 < 90*time.Millisecond {
		t.Errorf("p99 too low: %v", p99)
	}
}
