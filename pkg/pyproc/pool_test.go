package pyproc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/framing"
	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

type stubWorker struct {
	socketPath string
	startErr   error
	stopErr    error
	healthy    atomic.Bool
	startCalls atomic.Int32
	stopCalls  atomic.Int32
}

func newStubWorker(path string, healthy bool) *stubWorker {
	sw := &stubWorker{socketPath: path}
	sw.healthy.Store(healthy)
	return sw
}

func (s *stubWorker) Start(_ context.Context) error {
	s.startCalls.Add(1)
	return s.startErr
}

func (s *stubWorker) Stop() error {
	s.stopCalls.Add(1)
	return s.stopErr
}

func (s *stubWorker) IsHealthy(_ context.Context) bool {
	return s.healthy.Load()
}

func (s *stubWorker) GetSocketPath() string {
	return s.socketPath
}

func newPoolWithWorkers(cfg PoolConfig, workers []workerHandle) *Pool {
	if cfg.Workers == 0 {
		cfg.Workers = len(workers)
	}
	if cfg.MaxInFlight == 0 {
		cfg.MaxInFlight = 1
	}
	p := &Pool{
		opts:           PoolOptions{Config: cfg},
		logger:         NewLogger(LoggingConfig{Level: "error", Format: "json"}),
		workers:        make([]*poolWorker, len(workers)),
		semaphore:      make(chan struct{}, cfg.Workers*cfg.MaxInFlight),
		activeRequests: make(map[uint64]*activeRequest),
	}
	for i, w := range workers {
		p.workers[i] = &poolWorker{
			worker:   w,
			connPool: make(chan net.Conn, cfg.MaxInFlight),
		}
	}
	return p
}

func newPipeConn(t *testing.T, handler func(protocol.Request) *protocol.Response) (net.Conn, func()) {
	t.Helper()
	client, server := net.Pipe()
	done := make(chan struct{})
	go func() {
		fr := framing.NewFramer(server)
		for {
			msg, err := fr.ReadMessage()
			if err != nil {
				close(done)
				return
			}
			var req protocol.Request
			if err := req.Unmarshal(msg); err != nil {
				close(done)
				return
			}
			if handler == nil {
				continue
			}
			resp := handler(req)
			if resp == nil {
				continue
			}
			data, _ := resp.Marshal()
			if err := fr.WriteMessage(data); err != nil {
				close(done)
				return
			}
		}
	}()
	cleanup := func() {
		_ = client.Close()
		_ = server.Close()
		<-done
	}
	return client, cleanup
}

func startUnixServer(t *testing.T, path string, handler func(protocol.Request) *protocol.Response) func() {
	t.Helper()
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("failed to start unix server: %v", err)
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				fr := framing.NewFramer(c)
				for {
					msg, err := fr.ReadMessage()
					if err != nil {
						_ = c.Close()
						return
					}
					var req protocol.Request
					if err := req.Unmarshal(msg); err != nil {
						_ = c.Close()
						return
					}
					if handler == nil {
						continue
					}
					resp := handler(req)
					if resp == nil {
						continue
					}
					data, _ := resp.Marshal()
					if err := fr.WriteMessage(data); err != nil {
						_ = c.Close()
						return
					}
				}
			}(conn)
		}
	}()
	return func() {
		_ = ln.Close()
		<-stopped
		_ = os.Remove(path)
	}
}

func TestNewPoolDefaultsAndInvalidWorkers(t *testing.T) {
	_, err := NewPool(PoolOptions{Config: PoolConfig{Workers: 0}}, nil)
	if err == nil {
		t.Fatal("expected error for zero workers")
	}

	pool, err := NewPool(PoolOptions{Config: PoolConfig{Workers: 1, MaxInFlight: 0, HealthInterval: 0}, WorkerConfig: WorkerConfig{}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.opts.Config.MaxInFlight != 10 {
		t.Fatalf("MaxInFlight default mismatch: %d", pool.opts.Config.MaxInFlight)
	}
	if pool.opts.Config.HealthInterval != 30*time.Second {
		t.Fatalf("HealthInterval default mismatch: %v", pool.opts.Config.HealthInterval)
	}
	worker, ok := pool.workers[0].worker.(*Worker)
	if !ok {
		t.Fatalf("worker type mismatch")
	}
	if worker.cfg.StartTimeout != 5*time.Second {
		t.Fatalf("StartTimeout default mismatch: %v", worker.cfg.StartTimeout)
	}
	if pool.logger == nil {
		t.Fatal("logger should be set")
	}
}

func TestPoolStartPrepopulateAndHealth(t *testing.T) {
	tmp := t.TempDir()
	paths := []string{filepath.Join(tmp, "w0.sock"), filepath.Join(tmp, "w1.sock")}
	servers := []func(){
		startUnixServer(t, paths[0], func(req protocol.Request) *protocol.Response {
			resp, _ := protocol.NewResponse(req.ID, map[string]string{"ok": "yes"})
			return resp
		}),
		startUnixServer(t, paths[1], func(req protocol.Request) *protocol.Response {
			resp, _ := protocol.NewResponse(req.ID, map[string]string{"ok": "yes"})
			return resp
		}),
	}
	for _, stop := range servers {
		t.Cleanup(stop)
	}
	workers := []*stubWorker{newStubWorker(paths[0], true), newStubWorker(paths[1], true)}
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1, HealthInterval: 10 * time.Millisecond}, []workerHandle{workers[0], workers[1]})
	preConnClient, preConnServer := net.Pipe()
	t.Cleanup(func() {
		_ = preConnClient.Close()
		_ = preConnServer.Close()
	})
	p.workers[0].connPool <- preConnClient
	for _, pw := range p.workers {
		pw.healthy.Store(true)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("prepopulated connection should remain")
	}
	if len(p.workers[1].connPool) != 1 {
		t.Fatalf("connection not prepopulated")
	}
	status := p.Health()
	if status.HealthyWorkers != 2 {
		t.Fatalf("unexpected healthy workers: %d", status.HealthyWorkers)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestPoolStartWorkerErrorStopsPrevious(t *testing.T) {
	workers := []*stubWorker{newStubWorker("", true), newStubWorker("", true)}
	workers[1].startErr = errors.New("boom")
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1}, []workerHandle{workers[0], workers[1]})
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("expected start error")
	}
	if workers[0].stopCalls.Load() != 1 {
		t.Fatalf("previous worker not stopped")
	}
}

func TestPoolCallSuccessReturnsConnection(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	conn, cleanup := newPipeConn(t, func(req protocol.Request) *protocol.Response {
		resp, _ := protocol.NewResponse(req.ID, map[string]bool{"ok": true})
		return resp
	})
	t.Cleanup(cleanup)
	p.workers[0].connPool <- conn
	var out map[string]bool
	if err := p.Call(context.Background(), "echo", map[string]string{"msg": "hi"}, &out); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if !out["ok"] {
		t.Fatalf("unexpected response: %v", out)
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("connection not returned to pool")
	}
	if len(p.activeRequests) != 0 {
		t.Fatalf("active requests not cleaned")
	}
}

func TestPoolCallRoundRobinAndFallback(t *testing.T) {
	workers := []*stubWorker{newStubWorker("", true), newStubWorker("", true)}
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1}, []workerHandle{workers[0], workers[1]})
	for _, pw := range p.workers {
		pw.healthy.Store(true)
	}
	for i := range p.workers {
		conn, cleanup := newPipeConn(t, func(req protocol.Request) *protocol.Response {
			resp, _ := protocol.NewResponse(req.ID, map[string]string{"msg": "ok"})
			return resp
		})
		t.Cleanup(cleanup)
		p.workers[i].connPool <- conn
	}
	ids := make([]int, 3)
	for i := 0; i < 3; i++ {
		out := make(map[string]any)
		if err := p.Call(context.Background(), "echo_worker_id", map[string]string{"msg": "x"}, &out); err != nil {
			t.Fatalf("call failed: %v", err)
		}
		workerID, ok := out["worker_id"].(float64)
		if !ok {
			t.Fatalf("worker_id missing: %v", out)
		}
		ids[i] = int(workerID)
	}
	if ids[0] != 0 || ids[1] != 1 || ids[2] != 0 {
		t.Fatalf("unexpected round robin order: %v", ids)
	}
	p.workers[0].healthy.Store(false)
	out := make(map[string]any)
	if err := p.Call(context.Background(), "echo_worker_id", map[string]string{"msg": "x"}, &out); err != nil {
		t.Fatalf("fallback call failed: %v", err)
	}
	if int(out["worker_id"].(float64)) != 1 {
		t.Fatalf("fallback did not choose healthy worker: %v", out)
	}
}

func TestPoolCallNoHealthyWorkers(t *testing.T) {
	workers := []*stubWorker{newStubWorker("", false), newStubWorker("", false)}
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1}, []workerHandle{workers[0], workers[1]})
	for _, pw := range p.workers {
		pw.healthy.Store(false)
	}
	out := make(map[string]any)
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &out)
	if err == nil || err.Error() != "no healthy workers available" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolCallConnectError(t *testing.T) {
	w := newStubWorker(filepath.Join(t.TempDir(), "missing.sock"), true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &map[string]any{})
	if err == nil {
		t.Fatal("expected connect error")
	}
}

func TestPoolCallCreatesConnectionAndReturns(t *testing.T) {
	path := "/tmp/pool-creates-conn.sock"
	stop := startUnixServer(t, path, func(req protocol.Request) *protocol.Response {
		resp, _ := protocol.NewResponse(req.ID, map[string]string{"ok": "yes"})
		return resp
	})
	t.Cleanup(stop)
	w := newStubWorker(path, true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	out := make(map[string]string)
	if err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &out); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("connection not returned after new dial")
	}
}

func TestPoolCallWriteMessageError(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	client, server := net.Pipe()
	_ = server.Close()
	p.workers[0].connPool <- client
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &map[string]any{})
	if err == nil {
		t.Fatal("expected write error")
	}
	if len(p.workers[0].connPool) != 0 {
		t.Fatalf("broken connection should not return to pool")
	}
}

func TestPoolCallReadCancelled(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	client, server := net.Pipe()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		fr := framing.NewFramer(server)
		_, _ = fr.ReadMessage()
		time.Sleep(100 * time.Millisecond)
		_ = server.Close()
	}()
	p.workers[0].connPool <- client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := p.Call(ctx, "echo", map[string]string{"msg": "x"}, &map[string]any{})
	<-serverDone
	_ = client.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestPoolCallErrorResponse(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	conn, cleanup := newPipeConn(t, func(req protocol.Request) *protocol.Response {
		return protocol.NewErrorResponse(req.ID, errors.New("bad"))
	})
	t.Cleanup(cleanup)
	p.workers[0].connPool <- conn
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &map[string]any{})
	if err == nil || err.Error() != "bad" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("error response should return connection")
	}
}

func TestPoolCallRequestMarshalError(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	conn, cleanup := newPipeConn(t, func(req protocol.Request) *protocol.Response {
		resp, _ := protocol.NewResponse(req.ID, map[string]string{"ok": "yes"})
		return resp
	})
	t.Cleanup(cleanup)
	p.workers[0].connPool <- conn
	err := p.Call(context.Background(), "echo", func() {}, &map[string]any{})
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("connection not returned on marshal error")
	}
}

func TestPoolCallResponseUnmarshalError(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.workers[0].healthy.Store(true)
	client, server := net.Pipe()
	done := make(chan struct{})
	go func() {
		fr := framing.NewFramer(server)
		msg, _ := fr.ReadMessage()
		var req protocol.Request
		_ = req.Unmarshal(msg)
		_ = fr.WriteMessage([]byte("{invalid}"))
		close(done)
	}()
	p.workers[0].connPool <- client
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &map[string]any{})
	<-done
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if len(p.workers[0].connPool) != 1 {
		t.Fatalf("connection should return on unmarshal error")
	}
}

func TestPoolShutdownHandlesErrorsAndIsIdempotent(t *testing.T) {
	workers := []*stubWorker{newStubWorker("", true), newStubWorker("", true)}
	workers[0].stopErr = errors.New("stop failed")
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1}, []workerHandle{workers[0], workers[1]})
	_, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel
	c1a, c1b := net.Pipe()
	c2a, c2b := net.Pipe()
	t.Cleanup(func() {
		_ = c1a.Close()
		_ = c1b.Close()
		_ = c2a.Close()
		_ = c2b.Close()
	})
	p.workers[0].connPool <- c1a
	p.workers[1].connPool <- c2a
	if err := p.Shutdown(context.Background()); err == nil {
		t.Fatal("expected shutdown error")
	}
	if workers[0].stopCalls.Load() != 1 || workers[1].stopCalls.Load() != 1 {
		t.Fatalf("stop not called on workers")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown should be nil, got %v", err)
	}
}

func TestUpdateHealthStatusAndHealthAccessor(t *testing.T) {
	workers := []*stubWorker{newStubWorker("", true), newStubWorker("", false)}
	p := newPoolWithWorkers(PoolConfig{Workers: 2, MaxInFlight: 1}, []workerHandle{workers[0], workers[1]})
	p.updateHealthStatus()
	if p.Health().HealthyWorkers != 1 {
		t.Fatalf("health count mismatch: %d", p.Health().HealthyWorkers)
	}
	if !p.workers[0].healthy.Load() || p.workers[1].healthy.Load() {
		t.Fatalf("worker health flags not updated")
	}
}

func TestHealthMonitorTicker(t *testing.T) {
	t.Helper()
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1, HealthInterval: 5 * time.Millisecond}, []workerHandle{newStubWorker("", true)})
	p.wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.healthMonitor(ctx)
		close(done)
	}()
	time.Sleep(15 * time.Millisecond)
	cancel()
	<-done
}

func TestMonitorCancellationPaths(t *testing.T) {
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{newStubWorker("", true)})
	client, server := net.Pipe()
	done := make(chan struct{})
	go func() {
		fr := framing.NewFramer(server)
		_, _ = fr.ReadMessage()
		close(done)
	}()
	req := &activeRequest{id: 1, conn: client, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.monitorCancellation(ctx, req)
	<-done
	if err := client.Close(); err != nil {
		t.Fatalf("client close failed: %v", err)
	}
	req2 := &activeRequest{id: 2, conn: client, done: make(chan struct{})}
	close(req2.done)
	p.monitorCancellation(context.Background(), req2)
}

func TestSendCancellationWriteError(t *testing.T) {
	t.Helper()
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{newStubWorker("", true)})
	client, server := net.Pipe()
	_ = server.Close()
	req := &activeRequest{id: 3, conn: client, done: make(chan struct{})}
	p.sendCancellation(req)
}

func TestWorkerIsHealthyStates(t *testing.T) {
	w := &Worker{}
	w.state.Store(int32(WorkerStateRunning))
	if !w.IsHealthy(context.Background()) {
		t.Fatal("running worker should be healthy")
	}
	w.state.Store(int32(WorkerStateStopped))
	if w.IsHealthy(context.Background()) {
		t.Fatal("stopped worker should be unhealthy")
	}
}

func TestPoolConnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connect.sock")
	stop := startUnixServer(t, path, func(req protocol.Request) *protocol.Response {
		resp, _ := protocol.NewResponse(req.ID, map[string]string{"ok": "yes"})
		return resp
	})
	t.Cleanup(stop)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{newStubWorker(path, true)})
	conn, err := p.connect(path)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	_ = conn.Close()
	if _, err := p.connect(filepath.Join(t.TempDir(), "missing.sock")); err == nil {
		t.Fatal("expected connect error")
	}
}

func TestPoolCallAfterShutdown(t *testing.T) {
	w := newStubWorker("", true)
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{w})
	p.shutdown.Store(true)
	err := p.Call(context.Background(), "echo", map[string]string{"msg": "x"}, &map[string]any{})
	if err == nil {
		t.Fatal("expected shutdown error")
	}
}

func TestMonitorCancellationDoesNotCancelCompleted(t *testing.T) {
	t.Helper()
	p := newPoolWithWorkers(PoolConfig{Workers: 1, MaxInFlight: 1}, []workerHandle{newStubWorker("", true)})
	req := &activeRequest{id: 4, done: make(chan struct{})}
	close(req.done)
	p.monitorCancellation(context.Background(), req)
}
