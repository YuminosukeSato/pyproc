package pyproc

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
)

// createTestPoolForTracing creates a simple pool for tracing tests
func createTestPoolForTracing(tb testing.TB, workers int) *Pool {
	tb.Helper()

	opts := PoolOptions{
		Config: PoolConfig{
			Workers:              workers,
			MaxInFlight:          10,
			MaxInFlightPerWorker: 1,
			HealthInterval:       30 * time.Second,
		},
		WorkerConfig: WorkerConfig{
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			SocketPath:   "/tmp/test-pool-tracing.sock",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error", Format: "json"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		tb.Fatalf("Failed to create pool: %v", err)
	}

	return pool
}

func TestPool_WithTracer(t *testing.T) {
	t.Skip("Skipping integration test for now - requires Python worker")
	// TODO: Re-enable once we have proper test fixtures
}

func TestPool_WithoutTracer(t *testing.T) {
	t.Skip("Skipping integration test for now - requires Python worker")
	// TODO: Re-enable once we have proper test fixtures
}

func TestPool_TracingWithError(t *testing.T) {
	t.Skip("Skipping integration test for now - requires Python worker")
	// TODO: Re-enable once we have proper test fixtures
}

func TestPool_TracingWithCancellation(t *testing.T) {
	t.Skip("Skipping integration test for now - requires Python worker")
	// TODO: Re-enable once we have proper test fixtures
}

// TestPool_TracerSetAndGet tests the WithTracer method
func TestPool_TracerSetAndGet(t *testing.T) {
	pool := createTestPoolForTracing(t, 1)

	// Initially tracer should be nil
	if pool.tracer != nil {
		t.Error("Expected tracer to be nil initially")
	}

	// Create a tracer
	tp := trace.NewTracerProvider()
	defer func() {
		_ = tp.Shutdown(context.Background()) //nolint:errcheck
	}()
	tracer := tp.Tracer("test")

	// Set tracer
	returnedPool := pool.WithTracer(tracer)

	// Verify it returns the same pool (builder pattern)
	if returnedPool != pool {
		t.Error("WithTracer should return the same pool instance")
	}

	// Verify tracer is set
	if pool.tracer == nil {
		t.Error("Expected tracer to be set")
	}
}

// TestPool_TracingNilSpan verifies that nil span checks work correctly
func TestPool_TracingNilSpan(t *testing.T) {
	// This test verifies the nil span checks don't cause panics
	// by calling Pool.Call without setting a tracer

	pool := createTestPoolForTracing(t, 1)

	// pool.tracer is nil, so all span operations should be skipped
	// This should not panic
	ctx := context.Background()
	input := map[string]interface{}{"value": 42}
	var output map[string]interface{}

	// This will fail because pool is not started, but it should not panic
	_ = pool.Call(ctx, "predict", input, &output)

	// If we get here without panic, the nil checks worked
}

func BenchmarkPool_CallTracingOverhead(b *testing.B) {
	// This benchmark measures the overhead of tracing checks
	// even when tracer is nil (zero-overhead mode)

	pool := createTestPoolForTracing(b, 1)

	// Don't set a tracer - measure nil check overhead
	ctx := context.Background()
	input := map[string]interface{}{"value": 42}
	var output map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail, but we're measuring the overhead of the nil checks
		_ = pool.Call(ctx, "predict", input, &output)
	}
}
