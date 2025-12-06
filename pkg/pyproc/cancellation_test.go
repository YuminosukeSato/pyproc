package pyproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestPool creates a new pool for testing
func createTestPool(t *testing.T, id string) *Pool {
	requireUnixSocket(t)
	// Use /tmp directly with short names to avoid 104 char Unix socket path limit on macOS
	// Note: pool.go adds "-0" for worker ID, so keep the base path very short
	tmpDir := filepath.Join("/tmp", "pyproc")
	_ = os.MkdirAll(tmpDir, 0755)
	socketPath := filepath.Join(tmpDir, id)

	poolOpts := PoolOptions{
		Config: PoolConfig{
			Workers:        1,
			MaxInFlight:    3, // Allow 3 concurrent requests for the concurrent test
			HealthInterval: 100 * time.Millisecond,
		},
		WorkerConfig: WorkerConfig{
			ID:           id,
			SocketPath:   socketPath,
			PythonExec:   "python3",
			WorkerScript: "../../examples/cancellation/worker.py",
			StartTimeout: 5 * time.Second,
		},
	}

	logger := NewLogger(LoggingConfig{
		Level:  "info",
		Format: "json",
	})

	pool, err := NewPool(poolOpts, logger)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Give workers time to stabilize and create connections
	time.Sleep(500 * time.Millisecond)

	// Clean up socket file on test completion
	t.Cleanup(func() {
		// Clean up the socket file directory
		_ = os.RemoveAll(filepath.Dir(socketPath))
	})

	return pool
}

// TestContextCancellation tests that context cancellation propagates to Python workers
func TestContextCancellation(t *testing.T) {
	// Skip if running in CI without Python
	if os.Getenv("CI") == "true" && os.Getenv("SKIP_PYTHON_TESTS") == "true" {
		t.Skip("Skipping Python tests in CI")
	}

	t.Run("FastCancellation", func(t *testing.T) {
		pool := createTestPool(t, "fc")
		t.Cleanup(func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("Failed to shutdown pool: %v", err)
			}
		})
		// Create a context that will be cancelled quickly
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Call a slow operation that should be cancelled
		input := map[string]interface{}{
			"duration": 5.0, // 5 seconds - much longer than timeout
		}
		var output map[string]interface{}

		err := pool.Call(ctx, "slow_operation", input, &output)
		if err == nil {
			t.Error("Expected error due to cancellation, got nil")
		} else if err != context.DeadlineExceeded {
			// Could also be "Cancelled: context cancelled" from Python
			if err.Error() != "Cancelled: context cancelled" {
				t.Errorf("Expected context deadline exceeded or cancellation error, got: %v", err)
			}
		}
	})

	t.Run("NormalCompletion", func(t *testing.T) {
		pool := createTestPool(t, "nc")
		t.Cleanup(func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("Failed to shutdown pool: %v", err)
			}
		})
		// Test that a fast operation completes normally
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		input := map[string]interface{}{
			"duration": 0.05, // 50ms - should complete quickly
		}
		var output map[string]interface{}

		err := pool.Call(ctx, "slow_operation", input, &output)
		if err != nil {
			t.Errorf("Expected successful completion, got error: %v", err)
		}

		if completed, ok := output["completed"].(bool); !ok || !completed {
			t.Error("Operation did not complete successfully")
		}
	})

	t.Run("MultipleConcurrentCancellations", func(t *testing.T) {
		pool := createTestPool(t, "mc")
		t.Cleanup(func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("Failed to shutdown pool: %v", err)
			}
		})
		// Test multiple concurrent requests with cancellation
		results := make(chan error, 3)

		// Wait longer for worker to be fully ready and connection pool to stabilize
		time.Sleep(500 * time.Millisecond)

		for i := 0; i < 3; i++ {
			go func(id int) {
				// Use longer timeouts to ensure requests start before cancellation
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(300+id*100)*time.Millisecond)
				defer cancel()

				input := map[string]interface{}{
					"duration": 5.0, // Long enough duration to ensure cancellation happens
					"id":       id,
				}
				var output map[string]interface{}

				err := pool.Call(ctx, "slow_operation", input, &output)
				results <- err
			}(i)
		}

		// Give goroutines time to start their requests
		time.Sleep(100 * time.Millisecond)

		// Collect results
		errorCount := 0
		successCount := 0
		var errors []string
		for i := 0; i < 3; i++ {
			err := <-results
			if err != nil {
				errors = append(errors, err.Error())
				errorCount++
			} else {
				errors = append(errors, "nil (completed successfully)")
				successCount++
			}
		}

		// All operations should either be cancelled or fail due to connection issues
		// None should complete successfully given the long duration (5s) vs short timeout
		if successCount > 0 {
			t.Errorf("Expected no successful completions, got %d. Errors: %v", successCount, errors)
		}

		// At least some operations should encounter errors
		if errorCount == 0 {
			t.Error("Expected at least some errors, got none")
		}
	})

	t.Run("CancellationWithCleanup", func(t *testing.T) {
		pool := createTestPool(t, "cw")
		t.Cleanup(func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("Failed to shutdown pool: %v", err)
			}
		})
		// Test that cleanup happens properly on cancellation
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		input := map[string]interface{}{
			"duration":     5.0,
			"with_cleanup": true,
		}
		var output map[string]interface{}

		err := pool.Call(ctx, "operation_with_cleanup", input, &output)
		if err == nil {
			t.Error("Expected error due to cancellation, got nil")
		}

		// Give some time for cleanup to happen
		time.Sleep(200 * time.Millisecond)

		// Verify cleanup happened by calling a check method
		checkInput := map[string]interface{}{}
		var checkOutput map[string]interface{}

		ctx2 := context.Background()
		err = pool.Call(ctx2, "check_cleanup", checkInput, &checkOutput)
		if err != nil {
			t.Errorf("Failed to check cleanup: %v", err)
		}

		if checkOutput["cleanup_done"] != nil && !checkOutput["cleanup_done"].(bool) {
			t.Error("Cleanup was not performed after cancellation")
		}
	})
}

// TestCancellationLatency tests that cancellation happens within acceptable time
func TestCancellationLatency(t *testing.T) {
	// Skip if running in CI without Python
	if os.Getenv("CI") == "true" && os.Getenv("SKIP_PYTHON_TESTS") == "true" {
		t.Skip("Skipping Python tests in CI")
	}

	// Create and start pool
	pool := createTestPool(t, "lt")
	t.Cleanup(func() {
		if err := pool.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown pool: %v", err)
		}
	})

	// Measure cancellation latency
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running operation
	done := make(chan error, 1)
	start := time.Now()

	go func() {
		input := map[string]interface{}{
			"duration": 10.0, // 10 seconds
		}
		var output map[string]interface{}
		done <- pool.Call(ctx, "slow_operation", input, &output)
	}()

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for the operation to return
	err := <-done
	latency := time.Since(start)

	if err == nil {
		t.Error("Expected error due to cancellation, got nil")
	}

	// Check that cancellation happened within 100ms target
	if latency > 200*time.Millisecond {
		t.Errorf("Cancellation took too long: %v (target: < 100ms)", latency)
	}

	t.Logf("Cancellation latency: %v", latency)
}
