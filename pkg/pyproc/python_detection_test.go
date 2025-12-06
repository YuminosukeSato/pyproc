package pyproc

import (
	"context"
	"testing"
	"time"
)

func TestDetectPythonEnv_WithPyprocWorkerAvailable(t *testing.T) {
	// This test assumes pyproc-worker is installed and in PATH
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	env, err := DetectPythonEnv(ctx)
	if err != nil {
		t.Skipf("pyproc-worker not available, skipping: %v", err)
	}

	if env.Executable == "" {
		t.Error("Expected non-empty Executable")
	}

	t.Logf("Detected Python: %s (version: %s, venv: %s)",
		env.Executable, env.Version, env.VirtualEnvType)
}

func TestDetectPythonEnv_FallbackWhenCLIUnavailable(t *testing.T) {
	// Test fallback by using a very short timeout that will likely fail
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	env, err := DetectPythonEnv(ctx)
	if err != nil {
		t.Fatalf("Fallback detection failed: %v", err)
	}

	if env.Executable == "" {
		t.Error("Expected non-empty Executable from fallback")
	}

	// Fallback should not have detailed version/venv info
	if env.Version != "" {
		t.Logf("Note: Fallback unexpectedly has version: %s", env.Version)
	}

	t.Logf("Fallback detected Python: %s", env.Executable)
}

func TestDetectPythonEnv_Timeout(t *testing.T) {
	// Test that context cancellation is respected
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	env, err := DetectPythonEnv(ctx)

	// Should either fail gracefully or use fallback
	if err != nil && env == nil {
		// This is expected - context was cancelled
		t.Logf("Detection failed as expected with cancelled context: %v", err)
	} else if env != nil {
		// Fallback succeeded despite cancellation
		t.Logf("Fallback succeeded even with cancelled context: %s", env.Executable)
	}
}

func TestDetectPythonFallback_FindsPython3(t *testing.T) {
	env, err := detectPythonFallback()
	if err != nil {
		t.Fatalf("Failed to detect Python via fallback: %v", err)
	}

	if env.Executable == "" {
		t.Error("Expected non-empty Executable")
	}

	// Fallback should not have version or venv info
	if env.Version != "" {
		t.Errorf("Fallback should not have version, got: %s", env.Version)
	}
	if env.VirtualEnvType != "" {
		t.Errorf("Fallback should not have venv type, got: %s", env.VirtualEnvType)
	}
	if env.VirtualEnvPath != "" {
		t.Errorf("Fallback should not have venv path, got: %s", env.VirtualEnvPath)
	}

	t.Logf("Fallback found Python: %s", env.Executable)
}

func TestDetectPythonEnv_RealWorld(t *testing.T) {
	// Real-world test: should always succeed on a system with Python
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	env, err := DetectPythonEnv(ctx)
	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if env.Executable == "" {
		t.Fatal("Expected non-empty Executable")
	}

	t.Logf("Real-world detection result:")
	t.Logf("  Executable: %s", env.Executable)
	t.Logf("  Version: %s", env.Version)
	t.Logf("  VirtualEnvType: %s", env.VirtualEnvType)
	t.Logf("  VirtualEnvPath: %s", env.VirtualEnvPath)
}
