package pyproc

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     2,
			MaxInFlight: 10,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool.sock",
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

	if pool == nil {
		t.Fatal("pool is nil")
	}

	if len(pool.workers) != opts.Config.Workers {
		t.Errorf("expected %d workers, got %d", opts.Config.Workers, len(pool.workers))
	}
}

func TestPoolStart(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     2,
			MaxInFlight: 10,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-start.sock",
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

	// Check pool is running
	if pool.shutdown.Load() {
		t.Error("pool should not be shutdown")
	}
}

func TestPoolCall(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     2,
			MaxInFlight: 10,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-call.sock",
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

	// Test single call
	input := map[string]interface{}{"value": 42}
	var output map[string]interface{}
	if err := pool.Call(ctx, "predict", input, &output); err != nil {
		t.Fatalf("pool.Call failed: %v", err)
	}

	result, ok := output["result"].(float64)
	if !ok {
		t.Fatalf("result is not a number: %v", output["result"])
	}

	expected := float64(84)
	if result != expected {
		t.Errorf("expected result %f, got %f", expected, result)
	}
}

func TestPoolRoundRobin(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     3,
			MaxInFlight: 10,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-rr.sock",
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

	// Track which workers handle requests
	workerCounts := make(map[int]int)
	var mu sync.Mutex

	// Send multiple requests
	numRequests := 9
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			defer wg.Done()

			input := map[string]interface{}{
				"value":     idx,
				"worker_id": true, // Request worker ID in response
			}
			var output map[string]interface{}

			if err := pool.Call(ctx, "echo_worker_id", input, &output); err != nil {
				t.Errorf("Call %d failed: %v", idx, err)
				return
			}

			if workerID, ok := output["worker_id"].(float64); ok {
				mu.Lock()
				workerCounts[int(workerID)]++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify round-robin distribution
	for workerID, count := range workerCounts {
		expectedCount := numRequests / opts.Config.Workers
		if count != expectedCount {
			t.Errorf("worker %d handled %d requests, expected %d (round-robin)",
				workerID, count, expectedCount)
		}
	}
}

func TestPoolBackpressure(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 1, // Reduced to ensure backpressure
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-bp.sock",
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

	// Send more requests than MaxInFlight
	numRequests := 3 // Reduced for faster testing
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			input := map[string]interface{}{
				"value": idx,
				"sleep": 0.1, // Reduced sleep but still longer than timeout
			}
			var output map[string]interface{}

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			err := pool.Call(ctx, "slow_process", input, &output)
			errors <- err
		}(i)
	}

	// Collect results
	var timeoutCount int
	for i := 0; i < numRequests; i++ {
		err := <-errors
		if err != nil && err == context.DeadlineExceeded {
			timeoutCount++
		}
	}

	// Should have some timeouts due to backpressure
	if timeoutCount == 0 {
		t.Error("expected some requests to timeout due to backpressure")
	}
}

func TestPoolShutdown(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     2,
			MaxInFlight: 10,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-shutdown.sock",
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

	// Shutdown pool
	if err := pool.Shutdown(ctx); err != nil {
		t.Fatalf("pool.Shutdown failed: %v", err)
	}

	// Try to call after shutdown - should fail
	input := map[string]int{"value": 42}
	var output map[string]int
	err = pool.Call(ctx, "predict", input, &output)
	if err == nil {
		t.Error("expected error when calling after shutdown")
	}
}

func TestPoolHealthCheck(t *testing.T) {
	requireUnixSocket(t)
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:        2,
			MaxInFlight:    10,
			HealthInterval: 100 * time.Millisecond,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-pool-health.sock",
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

	// Wait for health checks to run and stabilize
	time.Sleep(500 * time.Millisecond)

	// Check health status
	health := pool.Health()
	if health.TotalWorkers != opts.Config.Workers {
		t.Errorf("expected %d total workers, got %d", opts.Config.Workers, health.TotalWorkers)
	}
	if health.HealthyWorkers != opts.Config.Workers {
		t.Errorf("expected %d healthy workers, got %d", opts.Config.Workers, health.HealthyWorkers)
	}
}
