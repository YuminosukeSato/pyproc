package pyproc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestPoolCall_SerializesPerWorker(t *testing.T) {
	paths := []string{
		tempSocketPath(t, "serial-w0"),
		tempSocketPath(t, "serial-w1"),
	}
	var collisions atomic.Int32
	for _, path := range paths {
		path := path
		var inflight atomic.Int32
		stop := startUnixServer(t, path, func(req protocol.Request) *protocol.Response {
			if inflight.Add(1) > 1 {
				collisions.Add(1)
			}
			time.Sleep(120 * time.Millisecond)
			inflight.Add(-1)
			resp, _ := protocol.NewResponse(req.ID, map[string]bool{"ok": true})
			return resp
		})
		t.Cleanup(stop)
	}

	workers := []workerHandle{
		newStubWorker(paths[0], true),
		newStubWorker(paths[1], true),
	}
	p := newPoolWithWorkers(PoolConfig{
		Workers:              2,
		MaxInFlight:          4,
		MaxInFlightPerWorker: 1,
		HealthInterval:       10 * time.Millisecond,
	}, workers)
	for _, pw := range p.workers {
		pw.healthy.Store(true)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := p.Call(ctx, "echo", map[string]string{"msg": "x"}, &map[string]any{}); err != nil {
				t.Errorf("call failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if collisions.Load() != 0 {
		t.Fatalf("expected no concurrent requests per worker, got %d", collisions.Load())
	}
}

func TestPoolCall_OversubscribeBlocksWithContext(t *testing.T) {
	path := tempSocketPath(t, "oversub")
	firstStarted := make(chan struct{})
	var reqCount atomic.Int32
	stop := startUnixServer(t, path, func(req protocol.Request) *protocol.Response {
		if reqCount.Add(1) == 1 {
			close(firstStarted)
			time.Sleep(200 * time.Millisecond)
		}
		resp, _ := protocol.NewResponse(req.ID, map[string]bool{"ok": true})
		return resp
	})
	t.Cleanup(stop)

	workers := []workerHandle{newStubWorker(path, true)}
	p := newPoolWithWorkers(PoolConfig{
		Workers:              1,
		MaxInFlight:          2,
		MaxInFlightPerWorker: 1,
	}, workers)
	p.workers[0].healthy.Store(true)

	firstErrCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		firstErrCh <- p.Call(ctx, "echo", map[string]string{"msg": "first"}, &map[string]any{})
	}()

	<-firstStarted

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.Call(ctx, "echo", map[string]string{"msg": "second"}, &map[string]any{})
	if err == nil {
		t.Fatal("expected deadline exceeded for oversubscribed call")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	if firstErr := <-firstErrCh; firstErr != nil {
		t.Fatalf("first call failed: %v", firstErr)
	}
}

func TestPoolCall_ShutdownConcurrent(t *testing.T) {
	path := tempSocketPath(t, "shutdown")
	started := make(chan struct{})
	stop := startUnixServer(t, path, func(req protocol.Request) *protocol.Response {
		close(started)
		time.Sleep(150 * time.Millisecond)
		resp, _ := protocol.NewResponse(req.ID, map[string]bool{"ok": true})
		return resp
	})
	t.Cleanup(stop)

	workers := []workerHandle{newStubWorker(path, true)}
	p := newPoolWithWorkers(PoolConfig{
		Workers:              1,
		MaxInFlight:          1,
		MaxInFlightPerWorker: 1,
	}, workers)
	p.workers[0].healthy.Store(true)

	callErrCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		callErrCh <- p.Call(ctx, "echo", map[string]string{"msg": "x"}, &map[string]any{})
	}()

	<-started

	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if callErr := <-callErrCh; callErr != nil {
		t.Fatalf("call failed during shutdown: %v", callErr)
	}
}
