package pyproc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startTestUnixListener creates a temporary Unix socket listener for testing.
// It uses /tmp directly to avoid macOS socket path length limits.
func startTestUnixListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	baseDir := filepath.Join(os.TempDir(), "pyproc")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	f, err := os.CreateTemp(baseDir, "pyproc-test-*.sock")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	sockPath := f.Name()
	_ = f.Close()
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create test unix listener: %v", err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
	})
	// Accept connections in background to avoid blocking dials
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			_ = conn.Close()
		}
	}()
	return ln, sockPath
}

func TestNewExternalWorker(t *testing.T) {
	w := NewExternalWorker("/tmp/test.sock", 0)
	if w.socketPath != "/tmp/test.sock" {
		t.Errorf("expected socketPath /tmp/test.sock, got %q", w.socketPath)
	}
	if w.connectTimeout != defaultConnectTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultConnectTimeout, w.connectTimeout)
	}
}

func TestNewExternalWorker_CustomTimeout(t *testing.T) {
	w := NewExternalWorker("/tmp/test.sock", 10*time.Second)
	if w.connectTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", w.connectTimeout)
	}
}

func TestExternalWorker_GetSocketPath(t *testing.T) {
	w := NewExternalWorker("/tmp/myworker.sock", 0)
	if got := w.GetSocketPath(); got != "/tmp/myworker.sock" {
		t.Errorf("expected /tmp/myworker.sock, got %q", got)
	}
}

func TestExternalWorker_Start_Success(t *testing.T) {
	_, sockPath := startTestUnixListener(t)

	w := NewExternalWorker(sockPath, time.Second)
	err := w.Start(context.Background())
	if err != nil {
		t.Fatalf("expected Start to succeed, got: %v", err)
	}
	if ExternalWorkerState(w.state.Load()) != ExternalWorkerRunning {
		t.Error("expected state to be Running after Start")
	}
}

func TestExternalWorker_Start_Failure(t *testing.T) {
	sockPath := "/tmp/pyproc-nonexistent-test.sock"
	w := NewExternalWorker(sockPath, 100*time.Millisecond)
	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail for nonexistent socket")
	}
	if ExternalWorkerState(w.state.Load()) != ExternalWorkerStopped {
		t.Error("expected state to remain Stopped after failed Start")
	}
}

func TestExternalWorker_Stop(t *testing.T) {
	_, sockPath := startTestUnixListener(t)

	w := NewExternalWorker(sockPath, time.Second)
	_ = w.Start(context.Background())

	err := w.Stop()
	if err != nil {
		t.Fatalf("expected Stop to succeed, got: %v", err)
	}
	if ExternalWorkerState(w.state.Load()) != ExternalWorkerStopped {
		t.Error("expected state to be Stopped after Stop")
	}
}

func TestExternalWorker_IsHealthy_Reachable(t *testing.T) {
	_, sockPath := startTestUnixListener(t)

	w := NewExternalWorker(sockPath, time.Second)
	if !w.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return true for reachable socket")
	}
}

func TestExternalWorker_IsHealthy_Unreachable(t *testing.T) {
	sockPath := "/tmp/pyproc-gone-test.sock"
	w := NewExternalWorker(sockPath, 100*time.Millisecond)
	if w.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return false for unreachable socket")
	}
}

func TestExternalWorker_IsHealthy_AfterListenerClose(t *testing.T) {
	ln, sockPath := startTestUnixListener(t)
	w := NewExternalWorker(sockPath, 100*time.Millisecond)

	// Healthy while listener is up
	if !w.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy true while listener is up")
	}

	// Close listener and remove socket
	_ = ln.Close()
	_ = os.Remove(sockPath)

	// Unhealthy after listener is gone
	if w.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy false after listener closed")
	}
}

// Verify ExternalWorker satisfies workerHandle interface at compile time.
var _ workerHandle = (*ExternalWorker)(nil)

// --- New tests for ExternalWorkerOptions and retry logic ---

func TestExternalWorkerWithOptions_Defaults(t *testing.T) {
	w := NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath: "/tmp/test.sock",
	})
	if w.connectTimeout != defaultConnectTimeout {
		t.Errorf("expected default connect timeout %v, got %v", defaultConnectTimeout, w.connectTimeout)
	}
	if w.maxRetries != defaultMaxRetries {
		t.Errorf("expected default max retries %d, got %d", defaultMaxRetries, w.maxRetries)
	}
	if w.retryInterval != defaultRetryInterval {
		t.Errorf("expected default retry interval %v, got %v", defaultRetryInterval, w.retryInterval)
	}
}

func TestExternalWorkerWithOptions_CustomValues(t *testing.T) {
	w := NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath:     "/tmp/custom.sock",
		ConnectTimeout: 2 * time.Second,
		MaxRetries:     3,
		RetryInterval:  100 * time.Millisecond,
	})
	if w.socketPath != "/tmp/custom.sock" {
		t.Errorf("expected socketPath /tmp/custom.sock, got %q", w.socketPath)
	}
	if w.connectTimeout != 2*time.Second {
		t.Errorf("expected connect timeout 2s, got %v", w.connectTimeout)
	}
	if w.maxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", w.maxRetries)
	}
	if w.retryInterval != 100*time.Millisecond {
		t.Errorf("expected retry interval 100ms, got %v", w.retryInterval)
	}
}

func TestNewExternalWorker_BackwardCompat(t *testing.T) {
	w := NewExternalWorker("/tmp/compat.sock", 3*time.Second)
	if w.maxRetries != 1 {
		t.Errorf("expected maxRetries=1 for backward compat, got %d", w.maxRetries)
	}
	if w.connectTimeout != 3*time.Second {
		t.Errorf("expected connectTimeout 3s, got %v", w.connectTimeout)
	}
}

func TestExternalWorker_Start_RetrySuccess(t *testing.T) {
	f, err := os.CreateTemp("/tmp", "pyproc-retry-*.sock")
	if err != nil {
		t.Fatal(err)
	}
	sockPath := f.Name()
	_ = f.Close()
	_ = os.Remove(sockPath)
	t.Cleanup(func() { _ = os.Remove(sockPath) })

	// Start listener after a short delay to simulate slow sidecar startup
	lnCh := make(chan net.Listener, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, listenErr := net.Listen("unix", sockPath)
		if listenErr != nil {
			return
		}
		lnCh <- ln
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		select {
		case ln := <-lnCh:
			_ = ln.Close()
		default:
		}
	})

	w := NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath:     sockPath,
		ConnectTimeout: 100 * time.Millisecond,
		MaxRetries:     5,
		RetryInterval:  100 * time.Millisecond,
	})
	err = w.Start(context.Background())
	if err != nil {
		t.Fatalf("expected Start with retry to succeed, got: %v", err)
	}
	if ExternalWorkerState(w.state.Load()) != ExternalWorkerRunning {
		t.Error("expected Running state")
	}
}

func TestExternalWorker_Start_AllRetriesFail(t *testing.T) {
	w := NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath:     "/tmp/pyproc-noexist-retry.sock",
		ConnectTimeout: 50 * time.Millisecond,
		MaxRetries:     3,
		RetryInterval:  50 * time.Millisecond,
	})
	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail after all retries")
	}
}

func TestExternalWorker_Start_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	w := NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath:     "/tmp/pyproc-cancel-retry.sock",
		ConnectTimeout: 50 * time.Millisecond,
		MaxRetries:     10,
		RetryInterval:  200 * time.Millisecond,
	})
	err := w.Start(ctx)
	if err == nil {
		t.Fatal("expected Start to fail when context cancelled")
	}
}
