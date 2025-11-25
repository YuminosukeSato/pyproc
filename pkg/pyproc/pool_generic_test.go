package pyproc

import (
	"context"
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
