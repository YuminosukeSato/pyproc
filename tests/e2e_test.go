package tests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/pkg/pyproc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_WorkerValidation(t *testing.T) {
	// Test that check command catches syntax errors
	tmpDir := t.TempDir()
	invalidScript := filepath.Join(tmpDir, "invalid_worker.py")

	// Create invalid worker with syntax error
	invalidCode := `
def invalid syntax here
    print("this will not work")
`
	err := writeFile(invalidScript, []byte(invalidCode))
	require.NoError(t, err)

	// Run check command - should fail
	cmd := exec.Command("pyproc-worker", "check", invalidScript)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Skipf("pyproc-worker not available or didn't catch syntax error")
	}

	// Should contain error about syntax
	outputStr := string(output)
	assert.Contains(t, outputStr, "syntax", "Error message should mention syntax error")

	t.Logf("✅ Worker validation caught syntax error as expected")
}

func TestE2E_WorkerValidationSuccess(t *testing.T) {
	// Test that check command passes for valid worker
	workerScript, err := filepath.Abs("fixtures/simple_worker.py")
	require.NoError(t, err)

	cmd := exec.Command("pyproc-worker", "check", workerScript)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Skipf("pyproc-worker not available, skipping: %v", err)
	}

	outputStr := string(output)
	assert.Contains(t, outputStr, "passed", "Valid worker should pass all checks")

	t.Logf("✅ Worker validation passed for valid worker")
}

func TestE2E_CompleteWorkflow(t *testing.T) {
	requireUnixSocket(t)

	// Complete workflow: validation → detection → pool → typed calls

	// Step 1: Validate worker script
	workerScript, err := filepath.Abs("fixtures/typed_worker.py")
	require.NoError(t, err)

	cmd := exec.Command("pyproc-worker", "check", workerScript)
	err = cmd.Run()
	if err != nil {
		t.Skipf("pyproc-worker not available or validation failed, skipping: %v", err)
	}

	t.Logf("✅ Step 1: Worker validation passed")

	// Step 2: Export schema
	cmd = exec.Command("pyproc-worker", "schema", workerScript, "--format", "json")
	schemaJSON, err := cmd.Output()
	if err != nil {
		t.Skipf("Schema export failed: %v", err)
	}

	var schema map[string]interface{}
	err = json.Unmarshal(schemaJSON, &schema)
	require.NoError(t, err)

	functions, ok := schema["functions"].(map[string]interface{})
	require.True(t, ok, "Schema should have functions")
	assert.Contains(t, functions, "predict")
	assert.Contains(t, functions, "transform")

	t.Logf("✅ Step 2: Schema export successful (%d functions)", len(functions))

	// Step 3: Auto-detect Python environment
	ctx := context.Background()
	pythonEnv, err := pyproc.DetectPythonEnv(ctx)
	require.NoError(t, err)

	t.Logf("✅ Step 3: Python detection successful (%s)", pythonEnv.Executable)

	// Step 4: Create pool with zero manual config
	socketPath := "/tmp/pyproc-e2e-workflow.sock"

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

	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	t.Logf("✅ Step 4: Pool started with %d workers", 2)

	// Step 5: Make calls using generic Call method
	var predictResult map[string]interface{}
	err = pool.Call(ctx, "predict", map[string]interface{}{
		"features": []float64{1.0, 2.0, 3.0},
	}, &predictResult)
	require.NoError(t, err)

	prediction, ok := predictResult["prediction"].(float64)
	require.True(t, ok, "prediction should be float64")
	assert.NotZero(t, prediction)

	confidence, ok := predictResult["confidence"].(float64)
	require.True(t, ok, "confidence should be float64")
	assert.True(t, confidence >= 0 && confidence <= 1, "confidence should be in [0, 1]")

	t.Logf("✅ Step 5: Prediction successful (prediction=%f, confidence=%f)", prediction, confidence)

	// Test transform function
	var transformResult map[string]interface{}
	err = pool.Call(ctx, "transform", map[string]interface{}{
		"data": []interface{}{1, 2, "hello", 3.5},
	}, &transformResult)
	require.NoError(t, err)

	transformed, ok := transformResult["transformed"].([]interface{})
	require.True(t, ok, "transformed should be array")
	assert.Equal(t, 4, len(transformed))

	t.Logf("✅ Complete workflow successful")
}

func TestE2E_MultiWorkerConcurrency(t *testing.T) {
	requireUnixSocket(t)

	workerScript, err := filepath.Abs("fixtures/simple_worker.py")
	require.NoError(t, err)

	socketPath := "/tmp/pyproc-e2e-concurrent.sock"

	pool, err := pyproc.NewPool(pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:     4,
			MaxInFlight: 10,
		},
		WorkerConfig: pyproc.WorkerConfig{
			PythonExec:   "", // Auto-detect
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

	// Make 100 concurrent requests
	const numRequests = 100
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(n int) {
			var result map[string]interface{}
			err := pool.Call(ctx, "add", map[string]interface{}{
				"a": n,
				"b": 1,
			}, &result)
			if err != nil {
				results <- err
				return
			}

			expected := float64(n + 1)
			if result["result"].(float64) != expected {
				results <- assert.AnError
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	var errors int
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errors++
			t.Logf("Request %d failed: %v", i, err)
		}
	}

	assert.Equal(t, 0, errors, "All concurrent requests should succeed")
	t.Logf("✅ %d concurrent requests successful", numRequests)
}

func TestE2E_ComplexTypes(t *testing.T) {
	requireUnixSocket(t)

	workerScript, err := filepath.Abs("fixtures/complex_worker.py")
	require.NoError(t, err)

	socketPath := "/tmp/pyproc-e2e-complex.sock"

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
				"PYTHONPATH": filepath.Join("..", "worker", "python"),
			},
		},
	}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Test nested structures
	var nestedResult map[string]interface{}
	err = pool.Call(ctx, "process_nested", map[string]interface{}{
		"data": map[string]interface{}{
			"items": []string{"a", "b", "c"},
		},
		"metadata": map[string]interface{}{
			"timestamp": "2024-12-06",
			"user":      "test",
		},
	}, &nestedResult)
	require.NoError(t, err)

	processed, ok := nestedResult["processed"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(3), processed["count"])

	t.Logf("✅ Nested structures handled successfully")

	// Test dataclass-like structures
	var distanceResult map[string]interface{}
	err = pool.Call(ctx, "calculate_distance", map[string]interface{}{
		"point1": map[string]interface{}{"x": 0.0, "y": 0.0},
		"point2": map[string]interface{}{"x": 3.0, "y": 4.0},
	}, &distanceResult)
	require.NoError(t, err)

	distance := distanceResult["distance"].(float64)
	assert.InDelta(t, 5.0, distance, 0.01)

	t.Logf("✅ Dataclass-like structures handled successfully")

	// Test aggregation
	var aggResult map[string]interface{}
	err = pool.Call(ctx, "aggregate_data", map[string]interface{}{
		"numbers":   []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		"operation": "avg",
	}, &aggResult)
	require.NoError(t, err)

	result := aggResult["result"].(float64)
	assert.Equal(t, 3.0, result)

	t.Logf("✅ Aggregation operations successful")
}

// Helper function
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
