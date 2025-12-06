package tests

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/pkg/pyproc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets not supported on Windows")
	}
}

func TestE2E_EnvironmentAutoDetection(t *testing.T) {
	requireUnixSocket(t)

	// 1. Detect environment
	ctx := context.Background()
	pythonEnv, err := pyproc.DetectPythonEnv(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, pythonEnv.Executable)

	t.Logf("Detected Python: %s (version: %s, venv: %s)",
		pythonEnv.Executable, pythonEnv.Version, pythonEnv.VirtualEnvType)

	// 2. Create pool with detected config
	workerScript, err := filepath.Abs("fixtures/simple_worker.py")
	require.NoError(t, err)

	socketPath := "/tmp/pyproc-e2e-env-auto.sock"

	pool, err := pyproc.NewPool(pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:     2,
			MaxInFlight: 10,
		},
		WorkerConfig: pyproc.WorkerConfig{
			PythonExec:   pythonEnv.Executable,
			WorkerScript: workerScript,
			SocketPath:   socketPath,
			StartTimeout: 5 * time.Second,
			Env: map[string]string{
				"PYTHONPATH": filepath.Join("..", "worker", "python"),
			},
		},
	}, nil)
	require.NoError(t, err)

	// 3. Start pool
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// 4. Make calls using generic Call method
	var echoResult map[string]interface{}
	err = pool.Call(ctx, "echo", map[string]interface{}{
		"message": "hello from auto-detected Python",
	}, &echoResult)
	require.NoError(t, err)
	assert.Equal(t, "hello from auto-detected Python", echoResult["echo"])

	var addResult map[string]interface{}
	err = pool.Call(ctx, "add", map[string]interface{}{
		"a": 42,
		"b": 8,
	}, &addResult)
	require.NoError(t, err)
	assert.Equal(t, float64(50), addResult["result"])

	t.Logf("✅ Environment auto-detection successful")
}

func TestE2E_AutoDetectWithEmptyPythonExec(t *testing.T) {
	requireUnixSocket(t)

	// Create pool with empty PythonExec - should auto-detect
	workerScript, err := filepath.Abs("fixtures/simple_worker.py")
	require.NoError(t, err)

	socketPath := "/tmp/pyproc-e2e-empty-exec.sock"

	pool, err := pyproc.NewPool(pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:     1,
			MaxInFlight: 10,
		},
		WorkerConfig: pyproc.WorkerConfig{
			PythonExec:   "", // Empty - should trigger auto-detection
			WorkerScript: workerScript,
			SocketPath:   socketPath,
			StartTimeout: 5 * time.Second,
			Env: map[string]string{
				"PYTHONPATH": filepath.Join("..", "worker", "python"),
			},
		},
	}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Verify pool is functional
	var result map[string]interface{}
	err = pool.Call(ctx, "uppercase", map[string]interface{}{
		"text": "auto-detect works",
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, "AUTO-DETECT WORKS", result["result"])

	t.Logf("✅ Auto-detection with empty PythonExec successful")
}

func TestE2E_DetectionFallback(t *testing.T) {
	// Test fallback detection when CLI unavailable
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	env, err := pyproc.DetectPythonEnv(ctx)
	require.NoError(t, err, "Fallback should succeed even with cancelled context")
	assert.NotEmpty(t, env.Executable)

	t.Logf("Fallback detected: %s", env.Executable)
}

func TestE2E_VirtualEnvDetection(t *testing.T) {
	// Test that virtual environment is detected if active
	ctx := context.Background()
	env, err := pyproc.DetectPythonEnv(ctx)
	require.NoError(t, err)

	if env.VirtualEnvType != "" {
		t.Logf("✅ Virtual environment detected: type=%s, path=%s",
			env.VirtualEnvType, env.VirtualEnvPath)
	} else {
		t.Logf("No virtual environment detected (system Python: %s)", env.Executable)
	}

	// Verify VIRTUAL_ENV is set in worker config
	if env.VirtualEnvPath != "" {
		workerScript, err := filepath.Abs("fixtures/simple_worker.py")
		require.NoError(t, err)

		socketPath := "/tmp/pyproc-e2e-venv.sock"

		pool, err := pyproc.NewPool(pyproc.PoolOptions{
			Config: pyproc.PoolConfig{
				Workers:     1,
				MaxInFlight: 10,
			},
			WorkerConfig: pyproc.WorkerConfig{
				PythonExec:   "", // Trigger auto-detection
				WorkerScript: workerScript,
				SocketPath:   socketPath,
				StartTimeout: 5 * time.Second,
				Env: map[string]string{
					"PYTHONPATH": filepath.Join("..", "worker", "python"),
				},
			},
		}, nil)
		require.NoError(t, err)

		err = pool.Start(ctx)
		require.NoError(t, err)
		defer pool.Shutdown(ctx)

		t.Logf("✅ Worker started successfully with virtual environment")
	}
}

func TestE2E_PythonPathSetup(t *testing.T) {
	requireUnixSocket(t)

	// Test that PYTHONPATH is correctly set for worker
	workerScript, err := filepath.Abs("fixtures/simple_worker.py")
	require.NoError(t, err)

	// Get absolute path to worker/python
	pythonPath, err := filepath.Abs(filepath.Join("..", "worker", "python"))
	require.NoError(t, err)

	// Verify directory exists
	info, err := os.Stat(pythonPath)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "PYTHONPATH should be a directory")

	socketPath := "/tmp/pyproc-e2e-pythonpath.sock"

	pool, err := pyproc.NewPool(pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:     1,
			MaxInFlight: 10,
		},
		WorkerConfig: pyproc.WorkerConfig{
			PythonExec:   "", // Auto-detect
			WorkerScript: workerScript,
			SocketPath:   socketPath,
			StartTimeout: 5 * time.Second,
			Env: map[string]string{
				"PYTHONPATH": pythonPath,
			},
		},
	}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Verify worker can import pyproc_worker (implicitly tested by successful start)
	t.Logf("✅ PYTHONPATH setup successful: %s", pythonPath)
}
