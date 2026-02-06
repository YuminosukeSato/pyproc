package pyproc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewPool_ZeroWorkers(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers: 0,
		},
	}

	_, err := NewPool(opts, nil)
	if err == nil {
		t.Error("expected error for zero workers")
	}
}

func TestNewPool_NegativeWorkers(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers: -1,
		},
	}

	_, err := NewPool(opts, nil)
	if err == nil {
		t.Error("expected error for negative workers")
	}
}

func TestPoolCall_AfterShutdown(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 5,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-shutdown-call.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool.Start failed: %v", err)
	}

	_ = pool.Shutdown(ctx)

	input := map[string]int{"value": 42}
	var output map[string]int
	err = pool.Call(ctx, "predict", input, &output)
	if err == nil {
		t.Error("expected error when calling after shutdown")
	}
}

func TestPoolCall_ContextCanceled(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 1,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-cancel.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool.Start failed: %v", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	input := map[string]int{"value": 42}
	var output map[string]int
	err = pool.Call(canceledCtx, "predict", input, &output)
	if err == nil {
		t.Error("expected error with canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Logf("got error: %v (may be acceptable)", err)
	}
}

func TestPoolShutdown_Double(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 5,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-double-shutdown.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool.Start failed: %v", err)
	}

	err1 := pool.Shutdown(ctx)
	err2 := pool.Shutdown(ctx)

	if err1 != nil {
		t.Errorf("first shutdown should succeed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second shutdown should succeed (no-op): %v", err2)
	}
}

func TestPool_DefaultMaxInFlight(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 0,
		},
		WorkerConfig: WorkerConfig{
			SocketPath: "/tmp/test.sock",
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	if pool.opts.Config.MaxInFlight != 10 {
		t.Errorf("expected default MaxInFlight 10, got %d", pool.opts.Config.MaxInFlight)
	}
}

func TestPool_DefaultMaxInFlightPerWorker(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:              1,
			MaxInFlight:          1,
			MaxInFlightPerWorker: 0,
		},
		WorkerConfig: WorkerConfig{
			SocketPath: "/tmp/test.sock",
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	if pool.opts.Config.MaxInFlightPerWorker != 1 {
		t.Errorf("expected default MaxInFlightPerWorker 1, got %d", pool.opts.Config.MaxInFlightPerWorker)
	}
}

func TestPool_DefaultHealthInterval(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:        1,
			HealthInterval: 0,
		},
		WorkerConfig: WorkerConfig{
			SocketPath: "/tmp/test.sock",
		},
	}

	pool, err := NewPool(opts, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	expected := 30 * time.Second
	if pool.opts.Config.HealthInterval != expected {
		t.Errorf("expected default HealthInterval %v, got %v", expected, pool.opts.Config.HealthInterval)
	}
}
