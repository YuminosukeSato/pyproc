package pyproc

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// startTestUnixListener creates a temporary Unix socket listener for testing.
// It uses /tmp directly to avoid macOS socket path length limits.
func startTestUnixListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	f, err := os.CreateTemp("/tmp", "pyproc-test-*.sock")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	sockPath := f.Name()
	f.Close()
	os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create test unix listener: %v", err)
	}
	t.Cleanup(func() {
		ln.Close()
		os.Remove(sockPath)
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
	ln.Close()
	_ = os.Remove(sockPath)

	// Unhealthy after listener is gone
	if w.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy false after listener closed")
	}
}

// Verify ExternalWorker satisfies workerHandle interface at compile time.
var _ workerHandle = (*ExternalWorker)(nil)
