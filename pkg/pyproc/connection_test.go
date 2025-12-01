package pyproc

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestConnectToWorker_Success(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

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

	conn, err := ConnectToWorker(socketPath, 5*time.Second)
	if err != nil {
		t.Fatalf("ConnectToWorker failed: %v", err)
	}
	defer func() { _ = conn.Close() }()
}

func TestConnectToWorker_Timeout(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	start := time.Now()
	_, err := ConnectToWorker(socketPath, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error for non-existent socket")
	}

	if elapsed < 200*time.Millisecond {
		t.Errorf("expected to wait at least 200ms, waited %v", elapsed)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("expected to timeout around 200ms, waited %v", elapsed)
	}
}

func TestSleepWithCtx_NormalCompletion(t *testing.T) {
	ctx := context.Background()
	start := time.Now()

	err := sleepWithCtx(ctx, 50*time.Millisecond)

	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected to sleep at least 50ms, slept %v", elapsed)
	}
}

func TestSleepWithCtx_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := sleepWithCtx(ctx, 1*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected context canceled error")
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("expected early exit on cancel, waited %v", elapsed)
	}
}

func TestSleepWithCtx_ContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sleepWithCtx(ctx, 1*time.Second)
	if err == nil {
		t.Error("expected context canceled error")
	}
}
