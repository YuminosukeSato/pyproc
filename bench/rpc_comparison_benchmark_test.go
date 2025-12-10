package bench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	rpc_clients "github.com/YuminosukeSato/pyproc/bench/rpc_clients"
)

// Global variables for Python server processes
var (
	pythonServers map[string]*exec.Cmd
	serversMutex  sync.Mutex
)

// TestMain sets up and tears down Python RPC servers
func TestMain(m *testing.M) {
	// Setup Python servers
	if err := startPythonServers(); err != nil {
		fmt.Printf("Failed to start Python servers: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	stopPythonServers()

	os.Exit(code)
}

// startPythonServers starts all Python RPC server processes
func startPythonServers() error {
	pythonServers = make(map[string]*exec.Cmd)

	// Get the project root directory
	projectRoot := getProjectRoot()
	serversDir := filepath.Join(projectRoot, "bench", "python_servers")

	// Define server configurations
	servers := []struct {
		name   string
		script string
		socket string
	}{
		{"jsonrpc", "jsonrpc_server.py", "/tmp/bench-jsonrpc.sock"},
		{"xmlrpc", "xmlrpc_server.py", "/tmp/bench-xmlrpc.sock"},
		{"msgpack", "msgpack_server.py", "/tmp/bench-msgpack.sock"},
	}

	// Start each server
	for _, server := range servers {
		scriptPath := filepath.Join(serversDir, server.script)

		// Check if script exists
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			// Create a simple test script if it doesn't exist
			// This is for testing purposes only
			continue
		}

		cmd := exec.Command("python3", scriptPath, server.socket)
		cmd.Dir = serversDir

		// Set Python path to include the servers directory
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("PYTHONPATH=%s:%s", serversDir, os.Getenv("PYTHONPATH")))

		// Start the server
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start %s server: %w", server.name, err)
		}

		pythonServers[server.name] = cmd

		// Give server time to start
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// stopPythonServers stops all Python RPC server processes
func stopPythonServers() {
	serversMutex.Lock()
	defer serversMutex.Unlock()

	for name, cmd := range pythonServers {
		if cmd != nil && cmd.Process != nil {
			// Try graceful shutdown first
			_ = cmd.Process.Signal(os.Interrupt)

			// Wait a bit for graceful shutdown
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-done:
				// Process exited gracefully
			case <-time.After(2 * time.Second):
				// Force kill if not exited
				_ = cmd.Process.Kill()
			}

			fmt.Printf("Stopped %s server\n", name)
		}
	}

	// Clean up socket files
	sockets := []string{
		"/tmp/bench-jsonrpc.sock",
		"/tmp/bench-xmlrpc.sock",
		"/tmp/bench-msgpack.sock",
		"/tmp/bench-pyproc.sock",
	}

	for _, socket := range sockets {
		_ = os.Remove(socket)
	}
}

// getProjectRoot returns the project root directory
func getProjectRoot() string {
	// Get the directory of the current file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Go up to the project root (assuming we're in bench/)
	return filepath.Dir(dir)
}

// BenchmarkRPCProtocols compares different RPC protocols
func BenchmarkRPCProtocols(b *testing.B) {
	// Create clients for each protocol
	clients := []struct {
		name   string
		client rpc_clients.RPCClient
		socket string
	}{
		{
			name:   "pyproc",
			client: rpc_clients.NewPyprocClient("python3", "../examples/basic/worker.py"),
			socket: "/tmp/bench-pyproc.sock",
		},
		{
			name:   "json-rpc",
			client: rpc_clients.NewJSONRPCClient(),
			socket: "/tmp/bench-jsonrpc.sock",
		},
		{
			name:   "xml-rpc",
			client: rpc_clients.NewXMLRPCClient(),
			socket: "/tmp/bench-xmlrpc.sock",
		},
		{
			name:   "msgpack-rpc",
			client: rpc_clients.NewMsgpackRPCClient(),
			socket: "/tmp/bench-msgpack.sock",
		},
	}

	// Test with different payload sizes
	payloads := []rpc_clients.TestPayload{
		rpc_clients.SmallPayload(),
		rpc_clients.MediumPayload(),
		rpc_clients.LargePayload(),
	}

	// Run benchmarks for each combination
	for _, clientConfig := range clients {
		for _, payload := range payloads {
			benchName := fmt.Sprintf("%s/%s", clientConfig.name, payload.Size)

			b.Run(benchName, func(b *testing.B) {
				// Connect to server
				err := clientConfig.client.Connect(clientConfig.socket)
				if err != nil {
					b.Skipf("Failed to connect to %s: %v", clientConfig.name, err)
					return
				}
				defer func() { _ = clientConfig.client.Close() }()

				// Warmup
				ctx := context.Background()
				for i := 0; i < 100; i++ {
					var result map[string]interface{}
					_ = clientConfig.client.Call(ctx, payload.Method, payload.Data, &result)
				}

				// Reset timer after warmup
				b.ResetTimer()

				// Run benchmark
				for i := 0; i < b.N; i++ {
					var result map[string]interface{}
					if err := clientConfig.client.Call(ctx, payload.Method, payload.Data, &result); err != nil {
						b.Fatalf("Call failed: %v", err)
					}
				}

				// Report additional metrics
				b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "req/s")
			})
		}
	}
}

// BenchmarkRPCLatency measures latency percentiles for each protocol
func BenchmarkRPCLatency(b *testing.B) {
	clients := []struct {
		name   string
		client rpc_clients.RPCClient
		socket string
	}{
		{
			name:   "pyproc",
			client: rpc_clients.NewPyprocClient("python3", "../examples/basic/worker.py"),
			socket: "/tmp/bench-pyproc.sock",
		},
		{
			name:   "json-rpc",
			client: rpc_clients.NewJSONRPCClient(),
			socket: "/tmp/bench-jsonrpc.sock",
		},
		{
			name:   "msgpack-rpc",
			client: rpc_clients.NewMsgpackRPCClient(),
			socket: "/tmp/bench-msgpack.sock",
		},
	}

	payload := rpc_clients.SmallPayload()

	for _, clientConfig := range clients {
		b.Run(clientConfig.name, func(b *testing.B) {
			// Connect to server
			err := clientConfig.client.Connect(clientConfig.socket)
			if err != nil {
				b.Skipf("Failed to connect to %s: %v", clientConfig.name, err)
				return
			}
			defer func() { _ = clientConfig.client.Close() }()

			// Warmup
			ctx := context.Background()
			for i := 0; i < 100; i++ {
				var result map[string]interface{}
				_ = clientConfig.client.Call(ctx, payload.Method, payload.Data, &result)
			}

			// Collect latency measurements
			latencies := make([]time.Duration, b.N)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				var result map[string]interface{}
				if err := clientConfig.client.Call(ctx, payload.Method, payload.Data, &result); err != nil {
					b.Fatalf("Call failed: %v", err)
				}
				latencies[i] = time.Since(start)
			}

			// Calculate percentiles
			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})

			p50 := latencies[len(latencies)*50/100]
			p95 := latencies[len(latencies)*95/100]
			p99 := latencies[len(latencies)*99/100]

			// Report metrics
			b.ReportMetric(float64(p50.Microseconds()), "p50_μs")
			b.ReportMetric(float64(p95.Microseconds()), "p95_μs")
			b.ReportMetric(float64(p99.Microseconds()), "p99_μs")
		})
	}
}

// BenchmarkRPCParallel tests parallel request handling
func BenchmarkRPCParallel(b *testing.B) {
	clients := []struct {
		name   string
		client func() rpc_clients.RPCClient
		socket string
	}{
		{
			name: "pyproc",
			client: func() rpc_clients.RPCClient {
				return rpc_clients.NewPyprocClient("python3", "../examples/basic/worker.py")
			},
			socket: "/tmp/bench-pyproc.sock",
		},
		{
			name: "json-rpc",
			client: func() rpc_clients.RPCClient {
				return rpc_clients.NewJSONRPCClient()
			},
			socket: "/tmp/bench-jsonrpc.sock",
		},
	}

	payload := rpc_clients.SmallPayload()

	for _, clientConfig := range clients {
		b.Run(clientConfig.name, func(b *testing.B) {
			// Create a client for testing connection
			testClient := clientConfig.client()
			if err := testClient.Connect(clientConfig.socket); err != nil {
				b.Skipf("Failed to connect to %s: %v", clientConfig.name, err)
				return
			}
			_ = testClient.Close()

			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				// Each goroutine gets its own client
				client := clientConfig.client()
				if err := client.Connect(clientConfig.socket); err != nil {
					b.Fatalf("Failed to connect: %v", err)
				}
				defer func() { _ = client.Close() }()

				ctx := context.Background()
				var result map[string]interface{}

				for pb.Next() {
					if err := client.Call(ctx, payload.Method, payload.Data, &result); err != nil {
						b.Fatalf("Call failed: %v", err)
					}
				}
			})

			// Report throughput
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "req/s")
		})
	}
}
