package pyproc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestMultiplexedTransport(t *testing.T) {
	t.Run("Concurrent Requests", func(t *testing.T) {
		// Start a test worker
		cfg := WorkerConfig{
			ID:           "test-worker",
			SocketPath:   "/tmp/test-multiplex.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		}

		logger := NewLogger(LoggingConfig{Level: "debug"})
		worker := NewWorker(cfg, logger)

		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("Failed to start worker: %v", err)
		}
		defer func() { _ = worker.Stop() }()

		// Create multiplexed transport
		transportConfig := TransportConfig{
			Type:    "multiplexed",
			Address: cfg.SocketPath,
			Options: map[string]interface{}{
				"timeout": 5 * time.Second,
			},
		}

		transport, err := NewMultiplexedTransport(transportConfig, logger)
		if err != nil {
			t.Fatalf("Failed to create transport: %v", err)
		}
		defer func() { _ = transport.Close() }()

		// Send multiple concurrent requests
		const numRequests = 10
		var wg sync.WaitGroup
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create request
				req, err := protocol.NewRequest(0, "predict", map[string]interface{}{
					"value": id,
				})
				if err != nil {
					errors <- err
					return
				}

				// Send request
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				resp, err := transport.Call(ctx, req)
				if err != nil {
					errors <- err
					return
				}

				// Verify response
				if !resp.OK {
					errors <- resp.Error()
					return
				}

				var result map[string]interface{}
				if err := resp.UnmarshalBody(&result); err != nil {
					errors <- err
					return
				}

				// Check result
				expected := float64(id * 2) // predict doubles the value
				if result["result"] != expected {
					errors <- fmt.Errorf("unexpected result: got %v, want %v", result["result"], expected)
				}
			}(i)
		}

		// Wait for all requests to complete
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				t.Errorf("Request failed: %v", err)
			}
		}
	})

	t.Run("Request Timeout", func(t *testing.T) {
		// Create transport with non-existent socket
		transportConfig := TransportConfig{
			Type:    "multiplexed",
			Address: "/tmp/nonexistent.sock",
			Options: map[string]interface{}{
				"timeout": 100 * time.Millisecond,
			},
		}

		logger := NewLogger(LoggingConfig{Level: "error"})
		_, err := NewMultiplexedTransport(transportConfig, logger)
		if err == nil {
			t.Error("Expected error for non-existent socket")
		}
	})

	t.Run("Large Payload", func(t *testing.T) {
		// Start a test worker
		cfg := WorkerConfig{
			ID:           "test-worker-large",
			SocketPath:   "/tmp/test-multiplex-large.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		}

		logger := NewLogger(LoggingConfig{Level: "error"})
		worker := NewWorker(cfg, logger)

		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("Failed to start worker: %v", err)
		}
		defer func() { _ = worker.Stop() }()

		// Create multiplexed transport
		transportConfig := TransportConfig{
			Type:    "multiplexed",
			Address: cfg.SocketPath,
		}

		transport, err := NewMultiplexedTransport(transportConfig, logger)
		if err != nil {
			t.Fatalf("Failed to create transport: %v", err)
		}
		defer func() { _ = transport.Close() }()

		// Create large payload
		largeData := make([]byte, 1024*1024) // 1MB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		req, err := protocol.NewRequest(0, "transform_text", map[string]interface{}{
			"text": string(largeData),
		})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Send request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := transport.Call(ctx, req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if !resp.OK {
			t.Fatalf("Response not OK: %v", resp.Error())
		}
	})
}

func BenchmarkMultiplexedTransport(b *testing.B) {
	// Start a test worker
	cfg := WorkerConfig{
		ID:           "bench-worker",
		SocketPath:   "/tmp/bench-multiplex.sock",
		PythonExec:   "python3",
		WorkerScript: "../../examples/basic/worker.py",
		StartTimeout: 5 * time.Second,
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	worker := NewWorker(cfg, logger)

	ctx := context.Background()
	if err := worker.Start(ctx); err != nil {
		b.Fatalf("Failed to start worker: %v", err)
	}
	defer func() { _ = worker.Stop() }()

	// Create multiplexed transport
	transportConfig := TransportConfig{
		Type:    "multiplexed",
		Address: cfg.SocketPath,
	}

	transport, err := NewMultiplexedTransport(transportConfig, logger)
	if err != nil {
		b.Fatalf("Failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Create request
	req, _ := protocol.NewRequest(0, "predict", map[string]interface{}{
		"value": 42,
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			resp, err := transport.Call(ctx, req)
			cancel()

			if err != nil {
				b.Errorf("Request failed: %v", err)
			}
			if resp != nil && !resp.OK {
				b.Errorf("Response not OK: %v", resp.Error())
			}
		}
	})
}
