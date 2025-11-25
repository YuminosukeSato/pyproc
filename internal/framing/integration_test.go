package framing_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/framing"
	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

// TestGoToPythonCommunication tests actual communication between Go and Python
func TestGoToPythonCommunication(t *testing.T) {
	requireUnixSocket(t)
	// Create a temporary Python worker script
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "test_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Write the test worker script
	pythonScript := `
import sys
import os
# Get the absolute path to the worker/python directory
base_dir = os.path.dirname(os.path.abspath(__file__))
for _ in range(5):  # Go up several levels to find the worker directory
    worker_path = os.path.join(base_dir, 'worker', 'python')
    if os.path.exists(worker_path):
        sys.path.insert(0, worker_path)
        break
    base_dir = os.path.dirname(base_dir)

from pyproc_worker import expose, run_worker

@expose
def echo(req):
    """Echo function for testing."""
    return {"message": req.get("message", ""), "echoed": True}

@expose
def add(req):
    """Add two numbers."""
    a = req.get("a", 0)
    b = req.get("b", 0)
    return {"result": a + b}

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`

	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	// Start the Python worker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the project root
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	cmd := exec.CommandContext(ctx, "python3", workerScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(projectRoot, "worker/python"))

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Python worker: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for the socket to be available
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Connect to the Python worker
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to worker: %v", err)
	}
	defer func() { _ = conn.Close() }()

	framer := framing.NewFramer(conn)

	// Test 1: Echo function
	t.Run("Echo", func(t *testing.T) {
		req, err := protocol.NewRequest(1, "echo", map[string]interface{}{
			"message": "Hello from Go!",
		})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Send request
		reqData, err := req.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		if err := framer.WriteMessage(reqData); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read response
		respData, err := framer.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var resp protocol.Response
		if err := resp.Unmarshal(respData); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if !resp.OK {
			t.Errorf("Response not OK: %s", resp.ErrorMsg)
		}

		var result map[string]interface{}
		if err := resp.UnmarshalBody(&result); err != nil {
			t.Fatalf("Failed to unmarshal body: %v", err)
		}

		if result["message"] != "Hello from Go!" {
			t.Errorf("Unexpected message: %v", result["message"])
		}
		if result["echoed"] != true {
			t.Errorf("Expected echoed=true")
		}
	})

	// Test 2: Add function
	t.Run("Add", func(t *testing.T) {
		req, err := protocol.NewRequest(2, "add", map[string]interface{}{
			"a": 10,
			"b": 32,
		})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Send request
		reqData, err := req.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		if err := framer.WriteMessage(reqData); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read response
		respData, err := framer.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var resp protocol.Response
		if err := resp.Unmarshal(respData); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if !resp.OK {
			t.Errorf("Response not OK: %s", resp.ErrorMsg)
		}

		var result map[string]interface{}
		if err := resp.UnmarshalBody(&result); err != nil {
			t.Fatalf("Failed to unmarshal body: %v", err)
		}

		// JSON unmarshals numbers as float64
		if result["result"].(float64) != 42 {
			t.Errorf("Expected 42, got %v", result["result"])
		}
	})

	// Test 3: Non-existent method
	t.Run("NonExistentMethod", func(t *testing.T) {
		req, err := protocol.NewRequest(3, "nonexistent", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Send request
		reqData, err := req.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		if err := framer.WriteMessage(reqData); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read response
		respData, err := framer.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var resp protocol.Response
		if err := resp.Unmarshal(respData); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.OK {
			t.Error("Expected error response for non-existent method")
		}
		if resp.ErrorMsg == "" {
			t.Error("Expected error message")
		}
	})

	// Test 4: Health check (built-in method)
	t.Run("Health", func(t *testing.T) {
		req, err := protocol.NewRequest(4, "health", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Send request
		reqData, err := req.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		if err := framer.WriteMessage(reqData); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read response
		respData, err := framer.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var resp protocol.Response
		if err := resp.Unmarshal(respData); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if !resp.OK {
			t.Errorf("Health check failed: %s", resp.ErrorMsg)
		}

		var result map[string]interface{}
		if err := resp.UnmarshalBody(&result); err != nil {
			t.Fatalf("Failed to unmarshal body: %v", err)
		}

		if result["status"] != "healthy" {
			t.Errorf("Expected healthy status, got %v", result["status"])
		}
		if _, ok := result["pid"]; !ok {
			t.Error("Expected pid in health response")
		}
	})
}

// TestMultipleRequests tests sending multiple requests over the same connection
func TestMultipleRequests(t *testing.T) {
	requireUnixSocket(t)
	// Skip if Python is not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("Python3 not available")
	}

	// Setup similar to TestGoToPythonCommunication
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "test_worker.py")
	socketPath := filepath.Join(tmpDir, "test.sock")

	pythonScript := `
import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../worker/python'))

from pyproc_worker import expose, run_worker

@expose
def counter(req):
    """Increment a counter."""
    return {"value": req.get("value", 0) + 1}

if __name__ == "__main__":
    run_worker("` + socketPath + `")
`

	if err := os.WriteFile(workerScript, []byte(pythonScript), 0644); err != nil {
		t.Fatalf("Failed to write worker script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get the project root
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	cmd := exec.CommandContext(ctx, "python3", workerScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(projectRoot, "worker/python"))

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Python worker: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for socket with shorter interval and longer timeout
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			// Give it a bit more time to fully initialize
			time.Sleep(100 * time.Millisecond)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Retry connection to handle timing issues in CI
	var conn net.Conn
	for retry := 0; retry < 10; retry++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		if retry < 9 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err != nil {
		t.Fatalf("Failed to connect to worker after retries: %v", err)
	}
	defer func() { _ = conn.Close() }()

	framer := framing.NewFramer(conn)

	// Send multiple requests
	for i := 0; i < 10; i++ {
		req, err := protocol.NewRequest(uint64(i), "counter", map[string]interface{}{
			"value": i,
		})
		if err != nil {
			t.Fatalf("Failed to create request %d: %v", i, err)
		}

		// Send request
		reqData, err := req.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal request %d: %v", i, err)
		}

		if err := framer.WriteMessage(reqData); err != nil {
			t.Fatalf("Failed to write message %d: %v", i, err)
		}

		// Read response
		respData, err := framer.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read response %d: %v", i, err)
		}

		var resp protocol.Response
		if err := resp.Unmarshal(respData); err != nil {
			t.Fatalf("Failed to unmarshal response %d: %v", i, err)
		}

		if !resp.OK {
			t.Errorf("Response %d not OK: %s", i, resp.ErrorMsg)
		}

		var result map[string]interface{}
		if err := resp.UnmarshalBody(&result); err != nil {
			t.Fatalf("Failed to unmarshal body %d: %v", i, err)
		}

		expected := float64(i + 1)
		if result["value"].(float64) != expected {
			t.Errorf("Request %d: expected value %v, got %v", i, expected, result["value"])
		}

		// Verify response ID matches request ID
		if resp.ID != uint64(i) {
			t.Errorf("Request %d: response ID mismatch, got %d", i, resp.ID)
		}
	}
}
