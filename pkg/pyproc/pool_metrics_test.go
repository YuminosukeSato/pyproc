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
