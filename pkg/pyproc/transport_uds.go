package pyproc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/framing"
	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

// UDSTransport implements Transport using Unix Domain Sockets
type UDSTransport struct {
	config   TransportConfig
	logger   *Logger
	conn     net.Conn
	framer   *framing.Framer
	codec    Codec
	mu       sync.Mutex
	closed   bool
	healthy  bool
	lastUsed time.Time
}

// NewUDSTransport creates a new UDS transport
func NewUDSTransport(config TransportConfig, logger *Logger) (*UDSTransport, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("address is required for UDS transport")
	}

	// Create codec (default to JSON)
	codecType := CodecJSON
	if codecTypeStr, ok := config.Options["codec"].(string); ok {
		codecType = CodecType(codecTypeStr)
	}

	codec, err := NewCodec(codecType)
	if err != nil {
		return nil, fmt.Errorf("failed to create codec: %w", err)
	}

	transport := &UDSTransport{
		config:  config,
		logger:  logger,
		codec:   codec,
		healthy: false,
	}

	// Establish connection
	if err := transport.connect(); err != nil {
		return nil, err
	}

	return transport, nil
}

// connect establishes the UDS connection
func (t *UDSTransport) connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		_ = t.conn.Close()
	}

	// Connect with timeout
	timeout := 5 * time.Second
	if timeoutVal, ok := t.config.Options["timeout"].(time.Duration); ok {
		timeout = timeoutVal
	}

	conn, err := net.DialTimeout("unix", t.config.Address, timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", t.config.Address, err)
	}

	t.conn = conn
	t.framer = framing.NewFramer(conn)
	t.healthy = true
	t.lastUsed = time.Now()

	t.logger.Debug("UDS transport connected", "address", t.config.Address)
	return nil
}

// Call sends a request and receives a response
func (t *UDSTransport) Call(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	// Check connection health
	if !t.healthy || t.conn == nil {
		if err := t.reconnect(); err != nil {
			return nil, fmt.Errorf("failed to reconnect: %w", err)
		}
	}

	// Set deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		if err := t.conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set deadline: %w", err)
		}
		defer func() { _ = t.conn.SetDeadline(time.Time{}) }()
	}

	// Send request
	reqData, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := t.framer.WriteMessage(reqData); err != nil {
		t.healthy = false
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	respData, err := t.framer.ReadMessage()
	if err != nil {
		t.healthy = false
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Unmarshal response
	var resp protocol.Response
	if err := resp.Unmarshal(respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	t.lastUsed = time.Now()
	return &resp, nil
}

// reconnect attempts to reconnect the transport
func (t *UDSTransport) reconnect() error {
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}

	// Reconnect with timeout
	timeout := 5 * time.Second
	if timeoutVal, ok := t.config.Options["timeout"].(time.Duration); ok {
		timeout = timeoutVal
	}

	conn, err := net.DialTimeout("unix", t.config.Address, timeout)
	if err != nil {
		return fmt.Errorf("failed to reconnect to %s: %w", t.config.Address, err)
	}

	t.conn = conn
	t.framer = framing.NewFramer(conn)
	t.healthy = true
	t.lastUsed = time.Now()

	t.logger.Debug("UDS transport reconnected", "address", t.config.Address)
	return nil
}

// Close closes the transport connection
func (t *UDSTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.healthy = false

	if t.conn != nil {
		if err := t.conn.Close(); err != nil {
			t.logger.Debug("error closing connection", "error", err)
		}
		t.conn = nil
	}

	return nil
}

// IsHealthy checks if the transport is healthy
func (t *UDSTransport) IsHealthy() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.conn == nil {
		return false
	}

	// Check if connection has been idle too long
	idleTimeout := 30 * time.Second
	if idleVal, ok := t.config.Options["idle_timeout"].(time.Duration); ok {
		idleTimeout = idleVal
	}

	if time.Since(t.lastUsed) > idleTimeout {
		// Try a simple ping to verify connection
		if err := t.ping(); err != nil {
			t.healthy = false
			return false
		}
	}

	return t.healthy
}

// ping sends a health check request
func (t *UDSTransport) ping() error {
	req, err := protocol.NewRequest(0, "health", nil)
	if err != nil {
		return err
	}

	reqData, err := req.Marshal()
	if err != nil {
		return err
	}

	// Set a short timeout for ping
	_ = t.conn.SetDeadline(time.Now().Add(1 * time.Second))
	defer func() { _ = t.conn.SetDeadline(time.Time{}) }()

	if err := t.framer.WriteMessage(reqData); err != nil {
		return err
	}

	_, err = t.framer.ReadMessage()
	return err
}
