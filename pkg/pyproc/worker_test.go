package pyproc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorker_Start(t *testing.T) {
	requireUnixSocket(t)
	// Create a test worker script
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "test_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	pythonScript := `
import sys
import os
# Add the pyproc_worker module to path
project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(project_root, 'worker', 'python'))

from pyproc_worker import expose, run_worker

@expose
def test_func(req):
    return {"result": "test"}

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`
	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Get project root and set PYTHONPATH
	projectRoot, _ := filepath.Abs("../..")
	pythonPath := filepath.Join(projectRoot, "worker", "python")

	// Create worker configuration
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   socketPath,
		PythonExec:   "python3",
		WorkerScript: workerScript,
		StartTimeout: 5 * time.Second,
		Env: map[string]string{
			"PYTHONPATH": pythonPath,
		},
	}

	// Create and start worker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker := NewWorker(cfg, nil)

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}
	defer func() { _ = worker.Stop() }()

	// Verify the socket exists
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("Socket file not created: %v", err)
	}

	// Try to connect to the socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Errorf("Failed to connect to worker socket: %v", err)
	} else {
		_ = conn.Close()
	}

	// Check worker status
	if !worker.IsRunning() {
		t.Error("Worker should be running")
	}
}

func TestWorker_Stop(t *testing.T) {
	requireUnixSocket(t)
	// Similar setup as TestWorker_Start
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "test_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	pythonScript := `
import sys
import os
project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(project_root, 'worker', 'python'))

from pyproc_worker import expose, run_worker

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`
	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Get project root and set PYTHONPATH
	projectRoot, _ := filepath.Abs("../..")
	pythonPath := filepath.Join(projectRoot, "worker", "python")

	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   socketPath,
		PythonExec:   "python3",
		WorkerScript: workerScript,
		StartTimeout: 5 * time.Second,
		Env: map[string]string{
			"PYTHONPATH": pythonPath,
		},
	}

	ctx := context.Background()
	worker := NewWorker(cfg, nil)

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Stop the worker
	if err := worker.Stop(); err != nil {
		t.Errorf("Failed to stop worker: %v", err)
	}

	// Verify worker is not running
	if worker.IsRunning() {
		t.Error("Worker should not be running after stop")
	}

	// Verify socket is cleaned up
	time.Sleep(100 * time.Millisecond) // Give some time for cleanup
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket file should be removed after stop")
	}
}

func TestWorker_Restart(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "test_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	pythonScript := `
import sys
import os
project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(project_root, 'worker', 'python'))

from pyproc_worker import expose, run_worker

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`
	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Get project root and set PYTHONPATH
	projectRoot, _ := filepath.Abs("../..")
	pythonPath := filepath.Join(projectRoot, "worker", "python")

	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   socketPath,
		PythonExec:   "python3",
		WorkerScript: workerScript,
		StartTimeout: 5 * time.Second,
		Env: map[string]string{
			"PYTHONPATH": pythonPath,
		},
	}

	ctx := context.Background()
	worker := NewWorker(cfg, nil)

	// Start worker
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}
	defer func() { _ = worker.Stop() }()

	// Get initial PID
	initialPID := worker.GetPID()
	if initialPID == 0 {
		t.Fatal("Worker PID should not be 0")
	}

	// Restart worker
	if err := worker.Restart(ctx); err != nil {
		t.Fatalf("Failed to restart worker: %v", err)
	}

	// Get new PID
	newPID := worker.GetPID()
	if newPID == 0 {
		t.Fatal("Worker PID should not be 0 after restart")
	}

	// Verify PID changed
	if initialPID == newPID {
		t.Error("Worker PID should change after restart")
	}

	// Verify worker is running
	if !worker.IsRunning() {
		t.Error("Worker should be running after restart")
	}
}

func TestWorker_StartTimeout(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "slow_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create a worker that sleeps before binding to socket
	pythonScript := `
import time
import sys
import os

# Sleep longer than the start timeout
time.sleep(10)

project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(project_root, 'worker', 'python'))

from pyproc_worker import run_worker
run_worker("` + socketPath + `")
`
	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Get project root and set PYTHONPATH
	projectRoot, _ := filepath.Abs("../..")
	pythonPath := filepath.Join(projectRoot, "worker", "python")

	cfg := WorkerConfig{
		ID:           "slow-worker",
		SocketPath:   socketPath,
		PythonExec:   "python3",
		WorkerScript: workerScript,
		StartTimeout: 1 * time.Second, // Short timeout
		Env: map[string]string{
			"PYTHONPATH": pythonPath,
		},
	}

	ctx := context.Background()
	worker := NewWorker(cfg, nil)

	// Start should timeout
	err := worker.Start(ctx)
	if err == nil {
		_ = worker.Stop()
		t.Fatal("Expected start to timeout")
	}

	// Verify error message contains timeout
	if err.Error() != fmt.Sprintf("worker start timeout after %v", cfg.StartTimeout) {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestWorker_InvalidScript(t *testing.T) {
	requireUnixSocket(t)
	cfg := WorkerConfig{
		ID:           "invalid-worker",
		SocketPath:   "/tmp/test.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 5 * time.Second,
	}

	ctx := context.Background()
	worker := NewWorker(cfg, nil)

	// Start should fail
	err := worker.Start(ctx)
	if err == nil {
		_ = worker.Stop()
		t.Fatal("Expected start to fail with invalid script")
	}

	// Worker should not be running
	if worker.IsRunning() {
		t.Error("Worker should not be running with invalid script")
	}
}

func TestWorker_AutoDetectPython(t *testing.T) {
	requireUnixSocket(t)

	// Create a simple Python worker script
	pythonScript := `
from pyproc_worker import expose, run_worker

@expose
def echo(req):
    return {"echo": req.get("message", "")}

if __name__ == "__main__":
    run_worker()
`

	scriptPath := filepath.Join(t.TempDir(), "auto_detect_worker.py")
	if err := os.WriteFile(scriptPath, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Create worker with empty PythonExec - should auto-detect
	cfg := WorkerConfig{
		ID:           "auto-detect-test",
		SocketPath:   "/tmp/pyproc-autodetect-test.sock",
		PythonExec:   "", // Empty - should auto-detect
		WorkerScript: scriptPath,
		StartTimeout: 5 * time.Second,
		Env: map[string]string{
			"PYTHONPATH": filepath.Join("..", "..", "worker", "python"),
		},
	}

	worker := NewWorker(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker with auto-detect: %v", err)
	}
	defer worker.Stop()

	// Verify worker is actually running
	if !worker.IsRunning() {
		t.Fatal("Worker should be running after auto-detect")
	}

	t.Logf("✅ Worker successfully started with auto-detected Python")
}
