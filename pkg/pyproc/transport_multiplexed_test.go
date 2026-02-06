package pyproc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestMultiplexedTransport(t *testing.T) {
	requireUnixSocket(t)
	t.Run("Concurrent Requests", func(t *testing.T) {
		// Start a test worker
		tmpDir := filepath.Join("/tmp", "pyproc")
		_ = os.MkdirAll(tmpDir, 0o755)
		socketPath := filepath.Join(tmpDir, fmt.Sprintf("mux-%d.sock", time.Now().UnixNano()))
		cfg := WorkerConfig{
			ID:           "test-worker",
			SocketPath:   socketPath,
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

		transport := newMultiplexedTransportWithRetry(t, transportConfig, logger, 2*time.Second)
		defer func() { _ = transport.Close() }()

		// Send multiple concurrent requests
		const numRequests = 10
		var wg sync.WaitGroup
		errors := make(chan error, numRequests)
		ready := make(chan struct{}, numRequests)
		start := make(chan struct{})

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

				ready <- struct{}{}
				<-start

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

		for i := 0; i < numRequests; i++ {
			<-ready
		}
		close(start)

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

		// Give worker time to stabilize
		time.Sleep(100 * time.Millisecond)

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

func newMultiplexedTransportWithRetry(t *testing.T, cfg TransportConfig, logger *Logger, timeout time.Duration) *MultiplexedTransport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		transport, err := NewMultiplexedTransport(cfg, logger)
		if err == nil {
			return transport
		}
		if ctx.Err() != nil {
			t.Fatalf("Failed to create transport: %v", err)
		}
		_ = sleepWithCtx(ctx, 10*time.Millisecond)
	}
}

func TestNewMultiplexedTransport_EmptyAddress(t *testing.T) {
	cfg := TransportConfig{
		Type:    "multiplexed",
		Address: "",
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	_, err := NewMultiplexedTransport(cfg, logger)
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestNewMultiplexedTransport_NonExistentSocket(t *testing.T) {
	cfg := TransportConfig{
		Type:    "multiplexed",
		Address: "/tmp/nonexistent-multiplex-12345.sock",
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	_, err := NewMultiplexedTransport(cfg, logger)
	if err == nil {
		t.Error("expected error for non-existent socket")
	}
}

func TestMultiplexedTransport_CallAfterClose(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "mux-call-close")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
		_ = conn.Close()
	}()

	cfg := TransportConfig{
		Type:    "multiplexed",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewMultiplexedTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	_ = transport.Close()

	req, _ := protocol.NewRequest(1, "test", nil)
	_, err = transport.Call(context.Background(), req)
	if err == nil {
		t.Error("expected error when calling after close")
	}
}

func TestMultiplexedTransport_IsHealthy(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "mux-health")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
		_ = conn.Close()
	}()

	cfg := TransportConfig{
		Type:    "multiplexed",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewMultiplexedTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	if !transport.IsHealthy() {
		t.Error("transport should be healthy initially")
	}

	_ = transport.Close()

	if transport.IsHealthy() {
		t.Error("transport should not be healthy after close")
	}
}

func TestMultiplexedTransport_ReadError(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "mux-read-error")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	serverConn := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		serverConn <- conn
	}()

	cfg := TransportConfig{Type: "multiplexed", Address: socketPath}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewMultiplexedTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	conn := <-serverConn
	_ = conn.Close()

	time.Sleep(50 * time.Millisecond)

	req, _ := protocol.NewRequest(1, "test", nil)
	_, err = transport.Call(context.Background(), req)
	if err == nil {
		t.Error("expected error after server closed connection")
	}
}

func TestMultiplexedTransport_DoubleClose(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "mux-dclose")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}()

	cfg := TransportConfig{
		Type:    "multiplexed",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewMultiplexedTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	err1 := transport.Close()
	err2 := transport.Close()

	if err1 != nil {
		t.Errorf("first close should succeed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second close should succeed (no-op): %v", err2)
	}
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

	// Give worker time to stabilize
	time.Sleep(100 * time.Millisecond)

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
