package pyproc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/framing"
	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func newPipeTransport(t *testing.T, options map[string]interface{}) (*MultiplexedTransport, net.Conn) {
	t.Helper()
	if options == nil {
		options = map[string]interface{}{}
	}
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	transport := &MultiplexedTransport{
		config:  TransportConfig{Options: options},
		logger:  NewLogger(LoggingConfig{Level: "error"}),
		conn:    client,
		framer:  framing.NewFramer(client),
		pending: make(map[uint64]*pendingRequest),
		closeCh: make(chan struct{}),
	}
	return transport, server
}

func startReadLoop(t *testing.T, transport *MultiplexedTransport) {
	t.Helper()
	transport.readerWg.Add(1)
	go transport.readLoop()
	t.Cleanup(func() { _ = transport.Close() })
}

func TestMultiplexedTransportCallSuccess(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	startReadLoop(t, transport)
	serverFramer := framing.NewFramer(server)

	done := make(chan struct{})
	go func() {
		defer close(done)
		data, err := serverFramer.ReadMessage()
		if err != nil {
			t.Errorf("server read failed: %v", err)
			return
		}
		var req protocol.Request
		if err := req.Unmarshal(data); err != nil {
			t.Errorf("request unmarshal failed: %v", err)
			return
		}
		resp, err := protocol.NewResponse(req.ID, map[string]interface{}{"result": float64(req.ID)})
		if err != nil {
			t.Errorf("response marshal failed: %v", err)
			return
		}
		respData, err := resp.Marshal()
		if err != nil {
			t.Errorf("response marshal failed: %v", err)
			return
		}
		if err := serverFramer.WriteMessage(respData); err != nil {
			t.Errorf("server write failed: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	req, _ := protocol.NewRequest(0, "predict", map[string]interface{}{"value": 1})
	resp, err := transport.Call(ctx, req)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	var body map[string]interface{}
	if err := resp.UnmarshalBody(&body); err != nil {
		t.Fatalf("unmarshal body failed: %v", err)
	}
	if body["result"] != float64(resp.ID) {
		t.Fatalf("unexpected result: %v", body["result"])
	}
	<-done
}

func TestMultiplexedTransportCallClosed(t *testing.T) {
	transport := &MultiplexedTransport{}
	transport.closed.Store(true)
	_, err := transport.Call(context.Background(), &protocol.Request{})
	if err == nil {
		t.Fatal("expected error on closed transport")
	}
}

func TestMultiplexedTransportCallContextCanceledBeforeWrite(t *testing.T) {
	transport, _ := newPipeTransport(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := transport.Call(ctx, &protocol.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestMultiplexedTransportCallWriteError(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	_ = server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	_, err := transport.Call(ctx, &protocol.Request{})
	if err == nil || !strings.Contains(err.Error(), "failed to write frame") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestMultiplexedTransportCallContextDeadline(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	serverFramer := framing.NewFramer(server)

	// Drain the request but do not respond.
	go func() {
		_, _ = serverFramer.ReadMessage()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	t.Cleanup(cancel)
	_, err := transport.Call(ctx, &protocol.Request{})
	var te *TimeoutError
	if !errors.As(err, &te) || te.Kind != TimeoutKindContext {
		t.Fatalf("expected context TimeoutError, got %T (%v)", err, err)
	}
}

func TestMultiplexedTransportCallTransportTimeout(t *testing.T) {
	transport, server := newPipeTransport(t, map[string]interface{}{
		"request_timeout": 20 * time.Millisecond,
	})
	serverFramer := framing.NewFramer(server)

	go func() {
		_, _ = serverFramer.ReadMessage()
	}()

	_, err := transport.Call(context.Background(), &protocol.Request{})
	var te *TimeoutError
	if !errors.As(err, &te) || te.Kind != TimeoutKindTransport {
		t.Fatalf("expected transport TimeoutError, got %T (%v)", err, err)
	}
}

func TestMultiplexedTransportCallErrCh(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	serverFramer := framing.NewFramer(server)

	requestRead := make(chan struct{})
	go func() {
		_, _ = serverFramer.ReadMessage()
		close(requestRead)
	}()

	errCh := make(chan error, 1)
	go func() {
		_, err := transport.Call(context.Background(), &protocol.Request{})
		errCh <- err
	}()

	<-requestRead
	transport.handleReadError(errors.New("boom"))
	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "connection error") {
		t.Fatalf("expected connection error, got %v", err)
	}
}

func TestMultiplexedTransportReadLoopUnmarshalError(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	startReadLoop(t, transport)
	serverFramer := framing.NewFramer(server)
	if err := serverFramer.WriteMessage([]byte("not-json")); err != nil {
		t.Fatalf("failed to write invalid message: %v", err)
	}
	_ = server.Close()
	time.Sleep(10 * time.Millisecond)
}

func TestMultiplexedTransportReadLoopUnknownRequest(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	startReadLoop(t, transport)
	serverFramer := framing.NewFramer(server)

	resp := &protocol.Response{
		ID:   999,
		OK:   true,
		Body: []byte(`{}`),
	}
	data, _ := resp.Marshal()
	if err := serverFramer.WriteMessage(data); err != nil {
		t.Fatalf("failed to write response: %v", err)
	}
	_ = server.Close()
	time.Sleep(10 * time.Millisecond)
}

func TestMultiplexedTransportReadLoopHeaderFallback(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	transport.framer = framing.NewEnhancedFramer(transport.conn)
	startReadLoop(t, transport)
	serverFramer := framing.NewEnhancedFramer(server)

	pending := &pendingRequest{
		id:         7,
		responseCh: make(chan *protocol.Response, 1),
		errCh:      make(chan error, 1),
	}
	transport.mu.Lock()
	transport.pending[7] = pending
	transport.mu.Unlock()

	resp := &protocol.Response{
		ID:   0,
		OK:   true,
		Body: []byte(`{}`),
	}
	respData, _ := resp.Marshal()
	frame := framing.NewFrame(7, respData)
	if err := serverFramer.WriteFrame(frame); err != nil {
		t.Fatalf("failed to write frame: %v", err)
	}

	select {
	case got := <-pending.responseCh:
		if got.ID != 7 {
			t.Fatalf("expected response ID 7, got %d", got.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for response")
	}
}

func TestMultiplexedTransportReadLoopClosedSkipsHandle(t *testing.T) {
	transport, server := newPipeTransport(t, nil)
	transport.closed.Store(true)
	transport.readerWg.Add(1)
	go transport.readLoop()
	_ = server.Close()
	transport.readerWg.Wait()
}

func TestMultiplexedTransportHandleReadErrorClosedChannel(t *testing.T) {
	transport := &MultiplexedTransport{
		logger:  NewLogger(LoggingConfig{Level: "error"}),
		pending: make(map[uint64]*pendingRequest),
		closeCh: make(chan struct{}),
	}
	pending := &pendingRequest{
		id:         1,
		responseCh: make(chan *protocol.Response, 1),
		errCh:      make(chan error, 1),
	}
	transport.pending[1] = pending
	close(transport.closeCh)

	transport.handleReadError(errors.New("boom"))
	if !transport.closed.Load() {
		t.Fatal("expected transport to be closed")
	}
	if len(transport.pending) != 0 {
		t.Fatal("expected pending to be cleared")
	}
}

func TestMultiplexedTransportCloseAndIsHealthy(t *testing.T) {
	transport, _ := newPipeTransport(t, nil)
	pending := &pendingRequest{
		id:         1,
		responseCh: make(chan *protocol.Response, 1),
		errCh:      make(chan error, 1),
	}
	transport.pending[1] = pending
	if !transport.IsHealthy() {
		t.Fatal("expected transport to be healthy")
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if transport.IsHealthy() {
		t.Fatal("expected transport to be unhealthy after close")
	}
	if len(transport.pending) != 0 {
		t.Fatal("expected pending to be cleared on close")
	}
}

func TestMultiplexedTransportCloseWithClosedChannel(t *testing.T) {
	transport, _ := newPipeTransport(t, nil)
	close(transport.closeCh)
	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestMultiplexedTransportConnectSuccess(t *testing.T) {
	requireUnixSocket(t)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("mux-connect-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	})

	acceptDone := make(chan struct{})
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			_ = conn.Close()
		}
		close(acceptDone)
	}()

	transport := &MultiplexedTransport{
		config: TransportConfig{
			Address: socketPath,
			Options: map[string]interface{}{
				"timeout": 50 * time.Millisecond,
			},
		},
		logger: NewLogger(LoggingConfig{Level: "error"}),
	}
	if err := transport.connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if transport.conn == nil || transport.framer == nil {
		t.Fatal("expected connection and framer to be initialized")
	}
	<-acceptDone
	_ = transport.conn.Close()
}

func TestMultiplexedTransportConnectError(t *testing.T) {
	transport := &MultiplexedTransport{
		config: TransportConfig{
			Address: "/tmp/nonexistent-transport.sock",
		},
		logger: NewLogger(LoggingConfig{Level: "error"}),
	}
	if err := transport.connect(); err == nil {
		t.Fatal("expected connect error")
	}
}

func TestNewMultiplexedTransportSuccess(t *testing.T) {
	requireUnixSocket(t)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("mux-new-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	})

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			time.Sleep(20 * time.Millisecond)
			_ = conn.Close()
		}
	}()

	transport, err := NewMultiplexedTransport(TransportConfig{
		Type:    "multiplexed",
		Address: socketPath,
	}, NewLogger(LoggingConfig{Level: "error"}))
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}
