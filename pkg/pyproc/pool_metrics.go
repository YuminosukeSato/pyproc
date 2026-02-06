package pyproc

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// PoolMetrics tracks metrics for connection pooling
type PoolMetrics struct {
	// Connection metrics
	ConnectionsCreated   atomic.Uint64
	ConnectionsDestroyed atomic.Uint64
	ConnectionsActive    atomic.Int32
	ConnectionsIdle      atomic.Int32

	// Request metrics
	RequestsTotal     atomic.Uint64
	RequestsSucceeded atomic.Uint64
	RequestsFailed    atomic.Uint64
	RequestsTimeout   atomic.Uint64

	// Latency tracking
	latencyMu    sync.RWMutex
	latencies    []time.Duration
	maxLatencies int

	// Worker metrics
	WorkerRestarts atomic.Uint64
	WorkerFailures atomic.Uint64

	// Pool utilization
	PoolUtilization atomic.Uint64 // percentage * 100
	QueueDepth      atomic.Int32
}

// NewPoolMetrics creates a new metrics tracker
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		maxLatencies: 10000, // Keep last 10k latencies for percentile calculation
		latencies:    make([]time.Duration, 0, 10000),
	}
}

// RecordLatency records a request latency
func (m *PoolMetrics) RecordLatency(latency time.Duration) {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()

	if len(m.latencies) >= m.maxLatencies {
		// Remove oldest entry
		m.latencies = m.latencies[1:]
	}
	m.latencies = append(m.latencies, latency)
}

// GetLatencyPercentile calculates latency percentile
func (m *PoolMetrics) GetLatencyPercentile(percentile float64) time.Duration {
	m.latencyMu.RLock()
	defer m.latencyMu.RUnlock()

	if len(m.latencies) == 0 {
		return 0
	}

	// Create a copy for sorting
	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	slices.Sort(sorted)

	// Simple percentile calculation (not perfectly accurate but fast)
	index := int(float64(len(sorted)-1) * percentile / 100.0)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// GetMetricsSnapshot returns a snapshot of current metrics.
// Percentiles are computed inline to avoid nested RLock on latencyMu,
// which would deadlock under concurrent writes (Go RWMutex is not reentrant).
func (m *PoolMetrics) GetMetricsSnapshot() MetricsSnapshot {
	m.latencyMu.RLock()
	var p50, p95, p99 time.Duration
	if len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		slices.Sort(sorted)
		p50 = percentileFromSorted(sorted, 50)
		p95 = percentileFromSorted(sorted, 95)
		p99 = percentileFromSorted(sorted, 99)
	}
	m.latencyMu.RUnlock()

	return MetricsSnapshot{
		ConnectionsCreated:   m.ConnectionsCreated.Load(),
		ConnectionsDestroyed: m.ConnectionsDestroyed.Load(),
		ConnectionsActive:    m.ConnectionsActive.Load(),
		ConnectionsIdle:      m.ConnectionsIdle.Load(),
		RequestsTotal:        m.RequestsTotal.Load(),
		RequestsSucceeded:    m.RequestsSucceeded.Load(),
		RequestsFailed:       m.RequestsFailed.Load(),
		RequestsTimeout:      m.RequestsTimeout.Load(),
		WorkerRestarts:       m.WorkerRestarts.Load(),
		WorkerFailures:       m.WorkerFailures.Load(),
		PoolUtilization:      float64(m.PoolUtilization.Load()) / 100.0,
		QueueDepth:           m.QueueDepth.Load(),
		LatencyP50:           p50,
		LatencyP95:           p95,
		LatencyP99:           p99,
	}
}

// percentileFromSorted returns the value at the given percentile from a pre-sorted slice.
func percentileFromSorted(sorted []time.Duration, percentile float64) time.Duration {
	index := int(float64(len(sorted)-1) * percentile / 100.0)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// MetricsSnapshot represents a point-in-time metrics snapshot
type MetricsSnapshot struct {
	// Connections
	ConnectionsCreated   uint64
	ConnectionsDestroyed uint64
	ConnectionsActive    int32
	ConnectionsIdle      int32

	// Requests
	RequestsTotal     uint64
	RequestsSucceeded uint64
	RequestsFailed    uint64
	RequestsTimeout   uint64

	// Workers
	WorkerRestarts uint64
	WorkerFailures uint64

	// Performance
	PoolUtilization float64
	QueueDepth      int32
	LatencyP50      time.Duration
	LatencyP95      time.Duration
	LatencyP99      time.Duration

	// Timestamp
	Timestamp time.Time
}

// PoolWithMetrics wraps a pool with metrics collection
type PoolWithMetrics struct {
	*Pool
	metrics *PoolMetrics
}

// NewPoolWithMetrics creates a pool with metrics tracking
func NewPoolWithMetrics(opts PoolOptions, logger *Logger) (*PoolWithMetrics, error) {
	pool, err := NewPool(opts, logger)
	if err != nil {
		return nil, err
	}

	return &PoolWithMetrics{
		Pool:    pool,
		metrics: NewPoolMetrics(),
	}, nil
}

// Call wraps the pool Call method with metrics
func (p *PoolWithMetrics) Call(ctx context.Context, method string, input interface{}, output interface{}) error {
	start := time.Now()
	p.metrics.RequestsTotal.Add(1)
	p.metrics.QueueDepth.Add(1)
	defer p.metrics.QueueDepth.Add(-1)

	err := p.Pool.Call(ctx, method, input, output)

	latency := time.Since(start)
	p.metrics.RecordLatency(latency)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			p.metrics.RequestsTimeout.Add(1)
		} else {
			p.metrics.RequestsFailed.Add(1)
		}
	} else {
		p.metrics.RequestsSucceeded.Add(1)
	}

	// Update utilization
	activeConns := p.metrics.ConnectionsActive.Load()
	totalConns := activeConns + p.metrics.ConnectionsIdle.Load()
	if totalConns > 0 {
		utilization := uint64(activeConns * 100 / totalConns)
		p.metrics.PoolUtilization.Store(utilization)
	}

	return err
}

// GetMetrics returns the current metrics snapshot
func (p *PoolWithMetrics) GetMetrics() MetricsSnapshot {
	snapshot := p.metrics.GetMetricsSnapshot()
	snapshot.Timestamp = time.Now()
	return snapshot
}

// ResetMetrics resets all metrics counters
func (p *PoolWithMetrics) ResetMetrics() {
	p.metrics = NewPoolMetrics()
}
