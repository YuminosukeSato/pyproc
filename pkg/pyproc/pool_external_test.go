package pyproc

import (
	"net"
	"os"
	"testing"
	"time"
)

// startTestSocket creates a temporary Unix socket listener for external pool tests.
func startTestSocket(t *testing.T) (net.Listener, string) {
	t.Helper()
	f, err := os.CreateTemp("/tmp", "pyproc-ext-pool-*.sock")
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
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return ln, sockPath
}

func TestNewPool_ExternalMode(t *testing.T) {
	_, sock1 := startTestSocket(t)
	_, sock2 := startTestSocket(t)

	pool, err := NewPool(PoolOptions{
		ExternalMode:        true,
		ExternalSocketPaths: []string{sock1, sock2},
	}, nil)
	if err != nil {
		t.Fatalf("NewPool with ExternalMode failed: %v", err)
	}
	if len(pool.workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(pool.workers))
	}

	// Verify workers are ExternalWorker instances
	for i, pw := range pool.workers {
		ew, ok := pw.worker.(*ExternalWorker)
		if !ok {
			t.Fatalf("worker %d is not ExternalWorker", i)
		}
		if ew.GetSocketPath() != pool.opts.ExternalSocketPaths[i] {
			t.Errorf("worker %d socket mismatch: got %q", i, ew.GetSocketPath())
		}
	}
}

func TestNewPool_ExternalMode_EmptyPaths(t *testing.T) {
	_, err := NewPool(PoolOptions{
		ExternalMode:        true,
		ExternalSocketPaths: []string{},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty ExternalSocketPaths")
	}
}

func TestNewPool_ExternalMode_NilPaths(t *testing.T) {
	_, err := NewPool(PoolOptions{
		ExternalMode: true,
	}, nil)
	if err == nil {
		t.Fatal("expected error for nil ExternalSocketPaths")
	}
}

func TestNewPool_ExternalMode_DefaultMaxInFlight(t *testing.T) {
	_, sock := startTestSocket(t)

	pool, err := NewPool(PoolOptions{
		ExternalMode:        true,
		ExternalSocketPaths: []string{sock},
	}, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	if pool.opts.Config.MaxInFlight != 10 {
		t.Errorf("expected default MaxInFlight=10, got %d", pool.opts.Config.MaxInFlight)
	}
}

func TestNewPool_ExternalMode_DefaultHealthInterval(t *testing.T) {
	_, sock := startTestSocket(t)

	pool, err := NewPool(PoolOptions{
		ExternalMode:        true,
		ExternalSocketPaths: []string{sock},
	}, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	if pool.opts.Config.HealthInterval != 30*time.Second {
		t.Errorf("expected default HealthInterval=30s, got %v", pool.opts.Config.HealthInterval)
	}
}

func TestNewPool_ExternalMode_WorkersCountMatchesPaths(t *testing.T) {
	_, s1 := startTestSocket(t)
	_, s2 := startTestSocket(t)
	_, s3 := startTestSocket(t)

	pool, err := NewPool(PoolOptions{
		ExternalMode:        true,
		ExternalSocketPaths: []string{s1, s2, s3},
	}, nil)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	if pool.opts.Config.Workers != 3 {
		t.Errorf("expected Workers=3, got %d", pool.opts.Config.Workers)
	}
}

func TestNewPool_NonExternalMode_Unchanged(t *testing.T) {
	// Verify that the normal path still requires Workers > 0
	_, err := NewPool(PoolOptions{
		Config: PoolConfig{Workers: 0},
	}, nil)
	if err == nil {
		t.Fatal("expected error for Workers=0 in normal mode")
	}
}
