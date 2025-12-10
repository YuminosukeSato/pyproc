package pyproc

import (
	"context"
	"testing"
	"time"
)

func TestPoolWithTransport_StartShutdown(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{
			Workers:        1,
			MaxInFlight:    5,
			HealthInterval: 100 * time.Millisecond,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-transport.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	pool, err := NewPoolWithTransport(opts, nil)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	health := pool.Health()
	if health.TotalWorkers != 1 {
		t.Errorf("expected 1 total worker, got %d", health.TotalWorkers)
	}

	if err := pool.Shutdown(ctx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	if err := pool.Shutdown(ctx); err != nil {
		t.Errorf("second shutdown should succeed: %v", err)
	}
}

func TestPoolWithTransport_Call(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-transport-call.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	pool, err := NewPoolWithTransport(opts, nil)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	var output map[string]interface{}
	err = pool.Call(ctx, "predict", map[string]interface{}{"value": 10}, &output)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
}

func TestPoolWithTransport_CallAfterShutdown(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-transport-shutdown.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	pool, err := NewPoolWithTransport(opts, nil)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_ = pool.Shutdown(ctx)

	var output map[string]interface{}
	err = pool.Call(ctx, "predict", map[string]interface{}{"value": 10}, &output)
	if err == nil {
		t.Error("expected error when calling after shutdown")
	}
}

func TestNewPoolWithTransport_InvalidConfig(t *testing.T) {
	_, err := NewPoolWithTransport(PoolOptions{
		Config: PoolConfig{Workers: 0},
	}, nil)
	if err == nil {
		t.Error("expected error for zero workers")
	}
}

func TestTransportPool_EmptyConfigs(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "error"})
	_, err := NewTransportPool(nil, logger)
	if err == nil {
		t.Error("expected error for empty configs")
	}
}

func TestTransportPool_Health(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-transport-pool-health.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	pool, err := NewPoolWithTransport(opts, nil)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	healthy, total := pool.transportPool.Health()
	if total != 1 {
		t.Errorf("expected 1 total transport, got %d", total)
	}
	if healthy < 0 || healthy > total {
		t.Errorf("healthy count %d out of range [0, %d]", healthy, total)
	}
}
