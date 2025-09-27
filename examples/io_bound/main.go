// Package main demonstrates IO-bound task handling with pyproc.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/YuminosukeSato/pyproc/pkg/pyproc"
)

func main() {
	// Create pool configuration with multi-threading enabled
	poolOpts := pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:        2,  // Number of worker processes
			MaxInFlight:    10, // Max concurrent requests per worker
			StartTimeout:   5 * time.Second,
			HealthInterval: 30 * time.Second,
		},
		WorkerConfig: pyproc.WorkerConfig{
			SocketPath:        "/tmp/pyproc-io-bound",
			PythonExec:        "python3",
			WorkerScript:      "./worker.py",
			StartTimeout:      5 * time.Second,
			WorkerConcurrency: 4, // Enable 4 threads per worker for IO-bound tasks
		},
	}

	// Create and start the pool
	pool, err := pyproc.NewPool(poolOpts, nil)
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start pool: %v", err)
	}
	defer func() {
		if err := pool.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown pool: %v", err)
		}
	}()

	fmt.Println("=== IO-Bound Example ===")
	fmt.Println("This example demonstrates handling multiple IO-bound requests concurrently")
	fmt.Println("Each worker process has 4 threads to handle requests in parallel")
	fmt.Println()

	// Example 1: Concurrent fetch operations
	fmt.Println("Example 1: Simulating concurrent API fetches...")
	demonstrateConcurrentFetches(ctx, pool)

	// Example 2: Batch operations
	fmt.Println("\nExample 2: Simulating batch API operations...")
	demonstrateBatchOperations(ctx, pool)

	// Example 3: Mixed IO operations
	fmt.Println("\nExample 3: Simulating mixed IO operations...")
	demonstrateMixedOperations(ctx, pool)

	// Example 4: Performance comparison
	fmt.Println("\nExample 4: Performance comparison (sequential vs concurrent)...")
	demonstratePerformance(ctx, pool)
}

func demonstrateConcurrentFetches(ctx context.Context, pool *pyproc.Pool) {
	var wg sync.WaitGroup
	start := time.Now()

	// Launch 8 concurrent requests
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := map[string]interface{}{
				"url":   fmt.Sprintf("http://api%d.example.com", id),
				"delay": 1.0, // Each request takes 1 second
			}

			var result map[string]interface{}
			err := pool.Call(ctx, "fetch_remote_data", req, &result)
			if err != nil {
				log.Printf("Request %d failed: %v", id, err)
				return
			}
			fmt.Printf("  Request %d completed on thread %v\n", id, result["thread_id"])
		}(i)
	}

	wg.Wait()
	fmt.Printf("  Total time for 8 requests (1s each): %.2fs\n", time.Since(start).Seconds())
	fmt.Println("  (With threading, these run concurrently instead of taking 8s)")
}

func demonstrateBatchOperations(ctx context.Context, pool *pyproc.Pool) {
	req := map[string]interface{}{
		"urls": []string{
			"http://api1.com",
			"http://api2.com",
			"http://api3.com",
			"http://api4.com",
			"http://api5.com",
		},
		"delay": 0.5,
	}

	start := time.Now()
	var result map[string]interface{}
	err := pool.Call(ctx, "batch_fetch", req, &result)
	if err != nil {
		log.Printf("Batch fetch failed: %v", err)
		return
	}
	fmt.Printf("  Fetched %v URLs in %.2fs on thread %v\n",
		result["total_urls"],
		time.Since(start).Seconds(),
		result["thread_id"])
}

func demonstrateMixedOperations(ctx context.Context, pool *pyproc.Pool) {
	var wg sync.WaitGroup
	start := time.Now()

	// Mix of different IO operations
	operations := []struct {
		method string
		req    map[string]interface{}
	}{
		{"database_query", map[string]interface{}{"query": "SELECT * FROM orders", "delay": 0.3}},
		{"fetch_remote_data", map[string]interface{}{"url": "http://api.example.com", "delay": 0.5}},
		{"heavy_io_operation", map[string]interface{}{"steps": 3, "delay": 0.2}},
		{"database_query", map[string]interface{}{"query": "UPDATE users SET active=true", "delay": 0.4}},
	}

	for i, op := range operations {
		wg.Add(1)
		go func(id int, method string, req map[string]interface{}) {
			defer wg.Done()

			var result map[string]interface{}
			err := pool.Call(ctx, method, req, &result)
			if err != nil {
				log.Printf("Operation %d (%s) failed: %v", id, method, err)
				return
			}
			fmt.Printf("  Operation %d (%s) completed on thread %v\n", id, method, result["thread_id"])
		}(i, op.method, op.req)
	}

	wg.Wait()
	fmt.Printf("  Mixed operations completed in %.2fs\n", time.Since(start).Seconds())
}

func demonstratePerformance(ctx context.Context, pool *pyproc.Pool) {
	numRequests := 20
	requestDelay := 0.2 // 200ms per request

	// Sequential simulation (what it would be without threading)
	sequentialTime := float64(numRequests) * requestDelay
	fmt.Printf("  Expected sequential time for %d requests: %.2fs\n", numRequests, sequentialTime)

	// Actual concurrent execution
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := map[string]interface{}{
				"query": fmt.Sprintf("SELECT * FROM table_%d", id),
				"delay": requestDelay,
			}

			var result map[string]interface{}
			err := pool.Call(ctx, "database_query", req, &result)
			if err != nil {
				log.Printf("Query %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	actualTime := time.Since(start).Seconds()

	fmt.Printf("  Actual concurrent time: %.2fs\n", actualTime)
	fmt.Printf("  Speedup: %.2fx faster\n", sequentialTime/actualTime)
	fmt.Printf("  Time saved: %.2fs\n", sequentialTime-actualTime)
}