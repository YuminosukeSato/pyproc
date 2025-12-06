package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/pkg/pyproc"
)

func requireUnixSocket(b *testing.B) {
	if runtime.GOOS == "windows" {
		b.Skip("Unix sockets not supported on Windows")
	}
}

func setupBenchPool(b *testing.B) *pyproc.Pool {
	requireUnixSocket(b)

	// Create simple benchmark worker
	tmpDir := b.TempDir()
	workerScript := filepath.Join(tmpDir, "bench_worker.py")

	workerCode := `
from pyproc_worker import expose, run_worker

@expose
def echo(req):
    return {"value": req["value"]}

@expose
def add(req):
    return {"result": req["a"] + req["b"]}

@expose
def compute(req):
    # Simple computation
    n = req["n"]
    result = sum(i * i for i in range(n))
    return {"result": result}

if __name__ == "__main__":
    run_worker()
`

	err := os.WriteFile(workerScript, []byte(workerCode), 0644)
	if err != nil {
		b.Fatalf("Failed to create worker script: %v", err)
	}

	socketPath := filepath.Join(tmpDir, "bench.sock")

	pool, err := pyproc.NewPool(pyproc.PoolOptions{
		Config: pyproc.PoolConfig{
			Workers:     4,
			MaxInFlight: 100,
		},
		WorkerConfig: pyproc.WorkerConfig{
			PythonExec:   "", // Auto-detect
			WorkerScript: workerScript,
			SocketPath:   socketPath,
			StartTimeout: 10 * time.Second,
			Env: map[string]string{
				"PYTHONPATH": filepath.Join("..", "worker", "python"),
			},
		},
	}, nil)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}

	ctx := context.Background()
	err = pool.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start pool: %v", err)
	}

	b.Cleanup(func() {
		_ = pool.Shutdown(context.Background())
	})

	return pool
}

func BenchmarkEnvironmentDetection(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pyproc.DetectPythonEnv(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSchemaExport(b *testing.B) {
	// Create a simple worker for benchmarking
	tmpDir := b.TempDir()
	workerScript := filepath.Join(tmpDir, "schema_bench_worker.py")

	workerCode := `
from pyproc_worker import expose, run_worker

@expose
def func1(req):
    return {"result": req["value"]}

@expose
def func2(req):
    return {"result": req["value"]}

@expose
def func3(req):
    return {"result": req["value"]}

if __name__ == "__main__":
    run_worker()
`

	err := os.WriteFile(workerScript, []byte(workerCode), 0644)
	if err != nil {
		b.Fatalf("Failed to create worker script: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("pyproc-worker", "schema", workerScript, "--format", "json")
		_, err := cmd.Output()
		if err != nil {
			b.Skip("pyproc-worker not available")
		}
	}
}

func BenchmarkPoolEcho(b *testing.B) {
	pool := setupBenchPool(b)
	ctx := context.Background()

	var result map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pool.Call(ctx, "echo", map[string]interface{}{
			"value": 42,
		}, &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPoolAdd(b *testing.B) {
	pool := setupBenchPool(b)
	ctx := context.Background()

	var result map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pool.Call(ctx, "add", map[string]interface{}{
			"a": 10,
			"b": 32,
		}, &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPoolCompute(b *testing.B) {
	pool := setupBenchPool(b)
	ctx := context.Background()

	var result map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pool.Call(ctx, "compute", map[string]interface{}{
			"n": 100,
		}, &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPoolConcurrent(b *testing.B) {
	pool := setupBenchPool(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var result map[string]interface{}
		for pb.Next() {
			err := pool.Call(ctx, "echo", map[string]interface{}{
				"value": 42,
			}, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkAutoDetectWorkerStart(b *testing.B) {
	requireUnixSocket(b)

	tmpDir := b.TempDir()
	workerScript := filepath.Join(tmpDir, "start_bench_worker.py")

	workerCode := `
from pyproc_worker import expose, run_worker

@expose
def health(req):
    return {"status": "ok"}

if __name__ == "__main__":
    run_worker()
`

	err := os.WriteFile(workerScript, []byte(workerCode), 0644)
	if err != nil {
		b.Fatalf("Failed to create worker script: %v", err)
	}

	pythonPath, _ := filepath.Abs(filepath.Join("..", "worker", "python"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		socketPath := filepath.Join(tmpDir, "start.sock")

		cfg := pyproc.WorkerConfig{
			ID:           "bench-worker",
			SocketPath:   socketPath,
			PythonExec:   "", // Auto-detect
			WorkerScript: workerScript,
			StartTimeout: 5 * time.Second,
			Env: map[string]string{
				"PYTHONPATH": pythonPath,
			},
		}

		worker := pyproc.NewWorker(cfg, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := worker.Start(ctx)
		cancel()

		if err != nil {
			b.Fatal(err)
		}

		_ = worker.Stop()
	}
}
