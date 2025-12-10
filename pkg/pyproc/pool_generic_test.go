package pyproc

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTypedPool(t *testing.T) {
	requireUnixSocket(t)
	t.Run("TypedPool with PredictRequest", func(t *testing.T) {
		// Create a regular pool
		opts := PoolOptions{
			Config: PoolConfig{
				Workers:     2,
				MaxInFlight: 10,
			},
			WorkerConfig: WorkerConfig{
				SocketPath:   "/tmp/test-typed-predict.sock",
				PythonExec:   "python3",
				WorkerScript: "../../examples/basic/worker.py",
			},
		}

		logger := NewLogger(LoggingConfig{Level: "error"})
		pool, err := NewPool(opts, logger)
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}

		ctx := context.Background()
		if err := pool.Start(ctx); err != nil {
			t.Fatalf("Failed to start pool: %v", err)
		}
		defer func() { _ = pool.Shutdown(ctx) }()

		// Give workers time to stabilize
		time.Sleep(100 * time.Millisecond)

		// Create typed pool
		typedPool := NewTypedPool[PredictRequest, PredictResponse](pool)

		// Make typed call
		input := PredictRequest{Value: 42}
		output, err := typedPool.Call(ctx, "predict", input)
		if err != nil {
			t.Fatalf("Typed call failed: %v", err)
		}

		expected := 84.0 // predict doubles the value
		if output.Result != expected {
			t.Errorf("Unexpected result: got %v, want %v", output.Result, expected)
		}
	})

	t.Run("TypedWorkerClient", func(t *testing.T) {
		t.Skip("Skipping flaky batch call test - needs investigation")
		// Create a regular pool
		opts := PoolOptions{
			Config: PoolConfig{
				Workers:     2,
				MaxInFlight: 10,
			},
			WorkerConfig: WorkerConfig{
				SocketPath:   "/tmp/test-typed-client.sock",
				PythonExec:   "python3",
				WorkerScript: "../../examples/basic/worker.py",
			},
		}

		logger := NewLogger(LoggingConfig{Level: "error"})
		pool, err := NewPool(opts, logger)
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}

		ctx := context.Background()
		if err := pool.Start(ctx); err != nil {
			t.Fatalf("Failed to start pool: %v", err)
		}
		defer func() { _ = pool.Shutdown(ctx) }()

		// Give workers time to stabilize
		time.Sleep(100 * time.Millisecond)

		// Create typed client for predict method
		predictClient := NewTypedWorkerClient[PredictRequest, PredictResponse](pool, "predict")

		// Single call
		input := PredictRequest{Value: 100}
		output, err := predictClient.Call(ctx, input)
		if err != nil {
			t.Fatalf("Client call failed: %v", err)
		}

		if output.Result != 200 {
			t.Errorf("Unexpected result: got %v, want 200", output.Result)
		}

		// Batch call
		inputs := []PredictRequest{
			{Value: 1},
			{Value: 2},
			{Value: 3},
		}

		outputs, errors := predictClient.BatchCall(ctx, inputs)

		for i, err := range errors {
			if err != nil {
				t.Errorf("Batch call %d failed: %v", i, err)
			}
		}

		expectedResults := []float64{2, 4, 6}
		for i, output := range outputs {
			if output.Result != expectedResults[i] {
				t.Errorf("Batch result %d: got %v, want %v", i, output.Result, expectedResults[i])
			}
		}
	})

	t.Run("CallTyped Function", func(t *testing.T) {
		t.Skip("Skipping flaky test - needs investigation")
		// Create a regular pool
		opts := PoolOptions{
			Config: PoolConfig{
				Workers:     1,
				MaxInFlight: 5,
			},
			WorkerConfig: WorkerConfig{
				SocketPath:   "/tmp/test-call-typed.sock",
				PythonExec:   "python3",
				WorkerScript: "../../examples/basic/worker.py",
			},
		}

		logger := NewLogger(LoggingConfig{Level: "error"})
		pool, err := NewPool(opts, logger)
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}

		ctx := context.Background()
		if err := pool.Start(ctx); err != nil {
			t.Fatalf("Failed to start pool: %v", err)
		}
		defer func() { _ = pool.Shutdown(ctx) }()

		// Give workers time to stabilize
		time.Sleep(100 * time.Millisecond)

		// Test with transform
		transformInput := TransformRequest{Text: "hello world"}
		transformOutput, err := CallTyped[TransformRequest, TransformResponse](ctx, pool, "transform_text", transformInput)
		if err != nil {
			t.Fatalf("Transform call failed: %v", err)
		}

		if transformOutput.TransformedText != "HELLO WORLD" {
			t.Errorf("Unexpected transform: got %v, want HELLO WORLD", transformOutput.TransformedText)
		}

		if transformOutput.WordCount != 2 {
			t.Errorf("Unexpected word count: got %v, want 2", transformOutput.WordCount)
		}

		// Test with stats
		statsInput := StatsRequest{Numbers: []float64{1, 2, 3, 4, 5}}
		statsOutput, err := CallTyped[StatsRequest, StatsResponse](ctx, pool, "compute_stats", statsInput)
		if err != nil {
			t.Fatalf("Stats call failed: %v", err)
		}

		if statsOutput.Mean != 3.0 {
			t.Errorf("Unexpected mean: got %v, want 3.0", statsOutput.Mean)
		}

		if statsOutput.Min != 1.0 || statsOutput.Max != 5.0 {
			t.Errorf("Unexpected min/max: got %v/%v, want 1.0/5.0", statsOutput.Min, statsOutput.Max)
		}
	})
}

func BenchmarkTypedPool(b *testing.B) {
	// Create a regular pool
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     4,
			MaxInFlight: 100,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/bench-typed.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		b.Fatalf("Failed to start pool: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	// Give workers time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Create typed client
	client := NewTypedWorkerClient[PredictRequest, PredictResponse](pool, "predict")

	b.Run("TypedCall", func(b *testing.B) {
		input := PredictRequest{Value: 42}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.Call(ctx, input)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RegularCall", func(b *testing.B) {
		input := map[string]interface{}{"value": 42}
		var output map[string]interface{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := pool.Call(ctx, "predict", input, &output)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestTypedAPI_JSONMarshalError tests that CallTyped returns a clear error
// when the request type cannot be marshaled to JSON
func TestTypedAPI_JSONMarshalError(t *testing.T) {
	requireUnixSocket(t)

	// Create pool
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 5,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-marshal-error.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	// Give workers time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Define a type with an unmarshalable field (function)
	type UnmarshalableRequest struct {
		Value int           `json:"value"`
		Fn    func() string `json:"fn"` // Functions cannot be JSON-marshaled
	}

	type SimpleResponse struct {
		Result int `json:"result"`
	}

	// Try to call with unmarshalable type
	input := UnmarshalableRequest{
		Value: 42,
		Fn:    func() string { return "test" },
	}

	_, err = CallTyped[UnmarshalableRequest, SimpleResponse](ctx, pool, "predict", input)

	// We expect an error about JSON marshaling
	if err == nil {
		t.Fatal("Expected error for unmarshalable type, got nil")
	}

	// Check that error message mentions marshaling
	errMsg := err.Error()
	if !strings.Contains(errMsg, "marshal") && !strings.Contains(errMsg, "json") {
		t.Errorf("Error message should mention marshaling or JSON, got: %v", errMsg)
	}
}

func TestTypedPool_Lifecycle(t *testing.T) {
	pool := &Pool{}
	tp := NewTypedPool[PredictRequest, PredictResponse](pool)

	health := tp.Health()
	if health.TotalWorkers != 0 {
		t.Errorf("expected 0 workers for unstarted pool")
	}
}

// TestTypedAPI_ResponseTypeMismatch tests that CallTyped returns a clear error
// when the response from Python doesn't match the expected type
func TestTypedAPI_ResponseTypeMismatch(t *testing.T) {
	requireUnixSocket(t)

	// Create pool
	opts := PoolOptions{
		Config: PoolConfig{
			Workers:     1,
			MaxInFlight: 5,
		},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-type-mismatch.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = pool.Shutdown(shutdownCtx)
	}()

	// Give workers time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Define a response type that expects int, but Python returns float
	type WrongTypeResponse struct {
		Result int `json:"result"` // Python returns float, we expect int
	}

	// Call predict which returns {"result": 84.0} (float)
	// We expect int, so JSON unmarshaling should work but value will be truncated
	input := PredictRequest{Value: 42}
	response, err := CallTyped[PredictRequest, WrongTypeResponse](ctx, pool, "predict", input)

	// JSON unmarshaling is permissive - float 84.0 can unmarshal to int 84
	// This is not an error in Go's JSON package
	if err != nil {
		t.Logf("Got error (this is acceptable): %v", err)
	}

	// The important test: if unmarshaling succeeds, value should be reasonable
	if err == nil && response.Result != 84 {
		t.Errorf("Expected result 84, got %d", response.Result)
	}

	// Test with a more incompatible type - expecting struct but getting string
	type StructResponse struct {
		Nested struct {
			Field string `json:"field"`
		} `json:"result"`
	}

	// This should fail because Python returns float, not nested object
	_, err = CallTyped[PredictRequest, StructResponse](ctx, pool, "predict", input)
	if err == nil {
		t.Error("Expected error when response type is incompatible, got nil")
	} else {
		// Verify error message is helpful
		errMsg := err.Error()
		t.Logf("Got expected error: %v", errMsg)
	}
}

func TestTypedPool_StartShutdownHealth(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-typed-pool-lifecycle.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	typedPool := NewTypedPool[PredictRequest, PredictResponse](pool)

	ctx := context.Background()
	if err := typedPool.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	health := typedPool.Health()
	if health.TotalWorkers != 1 {
		t.Errorf("expected 1 total worker, got %d", health.TotalWorkers)
	}

	if err := typedPool.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestTypedWorkerClient_Call(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-typed-worker-client.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	client := NewTypedWorkerClient[PredictRequest, PredictResponse](pool, "predict")
	input := PredictRequest{Value: 21}
	output, err := client.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	expected := 42.0
	if output.Result != expected {
		t.Errorf("expected %v, got %v", expected, output.Result)
	}
}

func TestCallTypedWithTransport(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 5},
		WorkerConfig: WorkerConfig{
			SocketPath:   "/tmp/test-call-typed-transport.sock",
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPoolWithTransport(opts, logger)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = pool.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	input := PredictRequest{Value: 50}
	output, err := CallTypedWithTransport[PredictRequest, PredictResponse](ctx, pool, "predict", input)
	if err != nil {
		t.Fatalf("CallTypedWithTransport failed: %v", err)
	}

	expected := 100.0
	if output.Result != expected {
		t.Errorf("expected %v, got %v", expected, output.Result)
	}
}

func TestTypedWorkerClient_BatchCall(t *testing.T) {
	t.Skip("Skipping flaky batch call test - Pool.Call blocks with concurrent requests")
}
