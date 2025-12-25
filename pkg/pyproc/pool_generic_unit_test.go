package pyproc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newShortSocketPath(t *testing.T, id string) string {
	t.Helper()
	tmpDir := filepath.Join("/tmp", "pyproc")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return filepath.Join(tmpDir, id)
}

func newPoolForTests(t *testing.T, id string) *Pool {
	t.Helper()
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 2},
		WorkerConfig: WorkerConfig{
			SocketPath:   newShortSocketPath(t, id),
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
			StartTimeout: 5 * time.Second,
		},
	}

	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPool(opts, logger)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	return pool
}

func TestCallTypedErrorOnUnstartedPool(t *testing.T) {
	pool := newPoolForTests(t, "typed-no-start")
	input := PredictRequest{Value: 1}
	_, err := CallTyped[PredictRequest, PredictResponse](context.Background(), pool, "predict", input)
	if err == nil {
		t.Fatal("expected error from CallTyped on unstarted pool")
	}
}

func TestCallTypedWithTransportErrorOnUnstartedPool(t *testing.T) {
	requireUnixSocket(t)

	opts := PoolOptions{
		Config: PoolConfig{Workers: 1, MaxInFlight: 1},
		WorkerConfig: WorkerConfig{
			SocketPath:   newShortSocketPath(t, "typed-transport-no-start"),
			PythonExec:   "python3",
			WorkerScript: "../../examples/basic/worker.py",
		},
	}
	logger := NewLogger(LoggingConfig{Level: "error"})
	pool, err := NewPoolWithTransport(opts, logger)
	if err != nil {
		t.Fatalf("failed to create pool with transport: %v", err)
	}
	// Avoid nil transport pool panic by providing an empty pool.
	pool.transportPool = &TransportPool{logger: logger}

	_, err = CallTypedWithTransport[PredictRequest, PredictResponse](context.Background(), pool, "predict", PredictRequest{Value: 1})
	if err == nil {
		t.Fatal("expected error from CallTypedWithTransport on unstarted pool")
	}
}

func TestTypedPoolLifecycleAndHealth(t *testing.T) {
	pool := newPoolForTests(t, "typed-pool-lifecycle")
	typedPool := NewTypedPool[PredictRequest, PredictResponse](pool)

	ctx := context.Background()
	if err := typedPool.Start(ctx); err != nil {
		t.Fatalf("typed pool start failed: %v", err)
	}
	t.Cleanup(func() { _ = typedPool.Shutdown(ctx) })

	health := typedPool.Health()
	if health.TotalWorkers == 0 {
		t.Fatal("expected total workers to be > 0")
	}

	output, err := typedPool.Call(ctx, "predict", PredictRequest{Value: 2})
	if err != nil {
		t.Fatalf("typed pool call failed: %v", err)
	}
	if output.Result != 4 {
		t.Fatalf("unexpected result: %v", output.Result)
	}
}

func TestTypedWorkerClientBatchCallEmpty(t *testing.T) {
	client := NewTypedWorkerClient[PredictRequest, PredictResponse](nil, "predict")
	results, errs := client.BatchCall(context.Background(), nil)
	if len(results) != 0 || len(errs) != 0 {
		t.Fatalf("expected empty results, got %d results and %d errors", len(results), len(errs))
	}
}

func TestTypedWorkerClientBatchCallSuccess(t *testing.T) {
	pool := newPoolForTests(t, "typed-batch-success")
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	t.Cleanup(func() { _ = pool.Shutdown(ctx) })

	client := NewTypedWorkerClient[PredictRequest, PredictResponse](pool, "predict")
	inputs := []PredictRequest{{Value: 1}, {Value: 2}}
	results, errs := client.BatchCall(ctx, inputs)

	if len(results) != len(inputs) || len(errs) != len(inputs) {
		t.Fatalf("unexpected batch sizes: results=%d errors=%d", len(results), len(errs))
	}
	for i := range inputs {
		if errs[i] != nil {
			t.Fatalf("unexpected error at index %d: %v", i, errs[i])
		}
	}
	if results[0].Result != 2 || results[1].Result != 4 {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestTimeoutErrorIsWrapped(t *testing.T) {
	err := newTimeoutError(TimeoutKindTransport, time.Second, context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expected TimeoutError to unwrap to context.DeadlineExceeded")
	}
}
