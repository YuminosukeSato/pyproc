package pyproc

import (
	"context"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestNewTransport(t *testing.T) {
	tests := []struct {
		name    string
		config  TransportConfig
		wantErr bool
	}{
		{
			name: "UDS transport",
			config: TransportConfig{
				Type:    "uds",
				Address: "/tmp/test.sock",
			},
			wantErr: true, // Will fail because socket doesn't exist
		},
		{
			name: "Default to UDS",
			config: TransportConfig{
				Type:    "",
				Address: "/tmp/test.sock",
			},
			wantErr: true, // Will fail because socket doesn't exist
		},
		{
			name: "Multiplexed transport",
			config: TransportConfig{
				Type:    "multiplexed",
				Address: "/tmp/multiplex.sock",
			},
			wantErr: true, // Will fail because socket doesn't exist
		},
		{
			name: "gRPC-TCP not implemented",
			config: TransportConfig{
				Type:    "grpc-tcp",
				Address: "localhost:50051",
			},
			wantErr: true,
		},
		{
			name: "gRPC-UDS not implemented",
			config: TransportConfig{
				Type:    "grpc-uds",
				Address: "/tmp/grpc.sock",
			},
			wantErr: true,
		},
		{
			name: "Unknown transport type",
			config: TransportConfig{
				Type:    "unknown",
				Address: "something",
			},
			wantErr: true,
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error", Format: "text"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTransport(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTransport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// MockTransport implements Transport interface for testing
type MockTransport struct {
	healthy  bool
	callFunc func(ctx context.Context, req *protocol.Request) (*protocol.Response, error)
	closed   bool
}

func (m *MockTransport) Call(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, req)
	}
	return &protocol.Response{
		ID:   req.ID,
		OK:   true,
		Body: []byte(`{"result": "mock"}`),
	}, nil
}

func (m *MockTransport) Close() error {
	m.closed = true
	return nil
}

func (m *MockTransport) IsHealthy() bool {
	return m.healthy
}

func TestTransportPool(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "error", Format: "text"})

	t.Run("Create pool with no configs", func(t *testing.T) {
		_, err := NewTransportPool([]TransportConfig{}, logger)
		if err == nil {
			t.Error("Expected error for empty configs")
		}
	})

	t.Run("Pool operations", func(t *testing.T) {
		// Create a mock transport pool manually
		pool := &TransportPool{
			transports: []Transport{
				&MockTransport{healthy: true},
				&MockTransport{healthy: false},
				&MockTransport{healthy: true},
			},
			logger: logger,
		}

		// Test Call with healthy transports
		ctx := context.Background()
		req, _ := protocol.NewRequest(1, "test", nil)
		resp, err := pool.Call(ctx, req)
		if err != nil {
			t.Errorf("Call failed: %v", err)
		}
		if resp == nil || !resp.OK {
			t.Error("Expected successful response")
		}

		// Test Health check
		healthy, total := pool.Health()
		if total != 3 {
			t.Errorf("Expected 3 total transports, got %d", total)
		}
		if healthy != 2 {
			t.Errorf("Expected 2 healthy transports, got %d", healthy)
		}

		// Test Close
		err = pool.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Verify all transports were closed
		for _, transport := range []Transport{
			&MockTransport{healthy: true},
			&MockTransport{healthy: false},
			&MockTransport{healthy: true},
		} {
			if mock, ok := transport.(*MockTransport); ok {
				if mock.closed {
					t.Error("Expected transport to be closed")
				}
			}
		}
	})

	t.Run("All transports unhealthy", func(t *testing.T) {
		pool := &TransportPool{
			transports: []Transport{
				&MockTransport{healthy: false},
				&MockTransport{healthy: false},
			},
			logger: logger,
		}

		ctx := context.Background()
		req, _ := protocol.NewRequest(1, "test", nil)
		_, err := pool.Call(ctx, req)
		if err == nil {
			t.Error("Expected error when all transports are unhealthy")
		}
	})

	t.Run("Round-robin selection", func(t *testing.T) {
		callCounts := make([]int, 3)
		pool := &TransportPool{
			transports: []Transport{
				&MockTransport{
					healthy: true,
					callFunc: func(_ context.Context, _ *protocol.Request) (*protocol.Response, error) {
						callCounts[0]++
						return &protocol.Response{OK: true}, nil
					},
				},
				&MockTransport{
					healthy: true,
					callFunc: func(_ context.Context, _ *protocol.Request) (*protocol.Response, error) {
						callCounts[1]++
						return &protocol.Response{OK: true}, nil
					},
				},
				&MockTransport{
					healthy: true,
					callFunc: func(_ context.Context, _ *protocol.Request) (*protocol.Response, error) {
						callCounts[2]++
						return &protocol.Response{OK: true}, nil
					},
				},
			},
			logger: logger,
		}

		ctx := context.Background()
		// Make 9 calls
		for i := 0; i < 9; i++ {
			req, _ := protocol.NewRequest(uint64(i), "test", nil)
			_, err := pool.Call(ctx, req)
			if err != nil {
				t.Errorf("Call %d failed: %v", i, err)
			}
		}

		// Each transport should have been called 3 times
		for i, count := range callCounts {
			if count != 3 {
				t.Errorf("Transport %d called %d times, expected 3", i, count)
			}
		}
	})
}

func TestPoolWithTransport(t *testing.T) {
	t.Run("Invalid workers count", func(t *testing.T) {
		opts := PoolOptions{
			Config: PoolConfig{
				Workers: 0,
			},
		}
		_, err := NewPoolWithTransport(opts, nil)
		if err == nil {
			t.Error("Expected error for zero workers")
		}
	})

	t.Run("Default values", func(t *testing.T) {
		opts := PoolOptions{
			Config: PoolConfig{
				Workers: 2,
			},
			WorkerConfig: WorkerConfig{
				SocketPath:   "/tmp/test",
				PythonExec:   "python3",
				WorkerScript: "test.py",
			},
		}

		pool, err := NewPoolWithTransport(opts, nil)
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}

		if pool.opts.Config.MaxInFlight != 10 {
			t.Errorf("Expected MaxInFlight to be 10, got %d", pool.opts.Config.MaxInFlight)
		}

		if pool.opts.Config.HealthInterval != 30*time.Second {
			t.Errorf("Expected HealthInterval to be 30s, got %v", pool.opts.Config.HealthInterval)
		}

		if len(pool.workers) != 2 {
			t.Errorf("Expected 2 workers, got %d", len(pool.workers))
		}
	})
}
