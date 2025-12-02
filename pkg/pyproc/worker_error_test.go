package pyproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorker_StopNotStarted(t *testing.T) {
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   "/tmp/test-stop-not-started.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 1 * time.Second,
	}

	worker := NewWorker(cfg, nil)

	err := worker.Stop()
	if err != nil {
		t.Errorf("stopping a not-started worker should not error: %v", err)
	}
}

func TestWorker_IsRunning_NotStarted(t *testing.T) {
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   "/tmp/test-isrunning.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 1 * time.Second,
	}

	worker := NewWorker(cfg, nil)

	if worker.IsRunning() {
		t.Error("worker should not be running before start")
	}
}

func TestWorker_GetPID_NotStarted(t *testing.T) {
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   "/tmp/test-getpid.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 1 * time.Second,
	}

	worker := NewWorker(cfg, nil)

	pid := worker.GetPID()
	if pid != 0 {
		t.Errorf("expected PID 0 for not-started worker, got %d", pid)
	}
}

func TestWorker_IsHealthy_NotRunning(t *testing.T) {
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   "/tmp/test-healthy.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 1 * time.Second,
	}

	worker := NewWorker(cfg, nil)
	ctx := context.Background()

	if worker.IsHealthy(ctx) {
		t.Error("worker should not be healthy when not running")
	}
}

func TestWorker_DoubleStop(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	workerScript := tmpDir + "/test_worker.py"
	socketPath := tmpDir + "/t.sock"

	pythonScript := `
import sys
import os
project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(project_root, 'worker', 'python'))

from pyproc_worker import expose, run_worker

@expose
def health(req):
    return {"status": "ok"}

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`
	if err := writeTestFile(workerScript, pythonScript); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	projectRoot, _ := absPath("../..")
	pythonPath := projectRoot + "/worker/python"

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

	err1 := worker.Stop()
	err2 := worker.Stop()

	if err1 != nil {
		t.Errorf("first stop should succeed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second stop should succeed (no-op): %v", err2)
	}
}

func TestWorker_RestartNotStarted(t *testing.T) {
	cfg := WorkerConfig{
		ID:           "test-worker",
		SocketPath:   "/tmp/test-restart.sock",
		PythonExec:   "python3",
		WorkerScript: "/nonexistent/script.py",
		StartTimeout: 1 * time.Second,
	}

	worker := NewWorker(cfg, nil)
	ctx := context.Background()

	err := worker.Restart(ctx)
	if err == nil {
		_ = worker.Stop()
		t.Error("expected restart to fail for non-started worker with invalid script")
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func absPath(path string) (string, error) {
	return filepath.Abs(path)
}
