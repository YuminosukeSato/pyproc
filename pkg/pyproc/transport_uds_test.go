package pyproc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestNewUDSTransport_EmptyAddress(t *testing.T) {
	cfg := TransportConfig{
		Type:    "uds",
		Address: "",
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	_, err := NewUDSTransport(cfg, logger)
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestNewUDSTransport_InvalidCodec(t *testing.T) {
	cfg := TransportConfig{
		Type:    "uds",
		Address: "/tmp/test.sock",
		Options: map[string]interface{}{
			"codec": "invalid-codec",
		},
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	_, err := NewUDSTransport(cfg, logger)
	if err == nil {
		t.Error("expected error for invalid codec")
	}
}

func TestNewUDSTransport_NonExistentSocket(t *testing.T) {
	cfg := TransportConfig{
		Type:    "uds",
		Address: "/tmp/nonexistent-socket-12345.sock",
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	_, err := NewUDSTransport(cfg, logger)
	if err == nil {
		t.Error("expected error for non-existent socket")
	}
}

func TestUDSTransport_CallAfterClose(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-call-close")

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
		Type:    "uds",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
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

func TestUDSTransport_Health(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-health")

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
		Type:    "uds",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	_ = transport.Close()

	if transport.IsHealthy() {
		t.Error("transport should not be healthy after close")
	}
}

func TestUDSTransport_Reconnect(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-reconnect")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	cfg := TransportConfig{Type: "uds", Address: socketPath}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	transport.healthy = false

	req, _ := protocol.NewRequest(1, "test", nil)
	_, _ = transport.Call(context.Background(), req)
}

func TestUDSTransport_ReconnectFail(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-reconnect-fail")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}()

	cfg := TransportConfig{Type: "uds", Address: socketPath}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_ = listener.Close()
	transport.healthy = false

	req, _ := protocol.NewRequest(1, "test", nil)
	_, err = transport.Call(context.Background(), req)
	if err == nil {
		t.Error("expected reconnect error")
	}
}

func TestUDSTransport_PingFail(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-ping-fail")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close()
	}()

	cfg := TransportConfig{
		Type:    "uds",
		Address: socketPath,
		Options: map[string]interface{}{
			"idle_timeout": 1 * time.Nanosecond,
		},
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_ = listener.Close()
	time.Sleep(10 * time.Millisecond)

	if transport.IsHealthy() {
		t.Error("expected unhealthy after ping failure")
	}
}

func TestUDSTransport_DoubleClose(t *testing.T) {
	requireUnixSocket(t)
	socketPath := tempSocketPath(t, "uds-dclose")

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
		Type:    "uds",
		Address: socketPath,
	}
	logger := NewLogger(LoggingConfig{Level: "error"})

	transport, err := NewUDSTransport(cfg, logger)
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

func TestUDSTransport_IsHealthy_NilConn(t *testing.T) {
	transport := &UDSTransport{
		conn:   nil,
		closed: false,
	}
	if transport.IsHealthy() {
		t.Error("expected unhealthy for nil connection")
	}
}
