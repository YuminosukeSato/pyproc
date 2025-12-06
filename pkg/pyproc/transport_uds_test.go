package pyproc

import (
	"context"
	"net"
	"path/filepath"
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
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "t.sock")

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
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "h.sock")

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

func TestUDSTransport_DoubleClose(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "d.sock")

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

// mockConnWithCloseError is a mock connection that returns error on Close()
type mockConnWithCloseError struct {
	net.Conn
	closed bool
}

func (m *mockConnWithCloseError) Close() error {
	if m.closed {
		return net.ErrClosed
	}
	m.closed = true
	return net.ErrClosed // Simulate close error
}

func (m *mockConnWithCloseError) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConnWithCloseError) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConnWithCloseError) LocalAddr() net.Addr                { return nil }
func (m *mockConnWithCloseError) RemoteAddr() net.Addr               { return nil }
func (m *mockConnWithCloseError) SetDeadline(t time.Time) error      { return nil }
func (m *mockConnWithCloseError) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConnWithCloseError) SetWriteDeadline(t time.Time) error { return nil }

func TestUDSTransport_CloseWithError(t *testing.T) {
	requireUnixSocket(t)

	cfg := TransportConfig{
		Type:    "uds",
		Address: "/tmp/test.sock",
	}
	logger := NewLogger(LoggingConfig{Level: "debug", Format: "text"})

	// Create transport with empty config (will fail to connect, but that's ok)
	transport := &UDSTransport{
		config:  cfg,
		logger:  logger,
		codec:   &JSONCodec{},
		healthy: true,
	}

	// Inject mock connection that returns error on Close()
	transport.conn = &mockConnWithCloseError{}

	// Close should succeed but log the error
	err := transport.Close()
	if err != nil {
		t.Errorf("Close() should return nil even with conn.Close() error, got: %v", err)
	}

	// Verify connection is nil after close
	if transport.conn != nil {
		t.Error("expected conn to be nil after Close()")
	}
}
