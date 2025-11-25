.PHONY: demo bench bench-quick bench-full bench-comparison fmt lint test deps-python coverage

demo:
	GO111MODULE=on go run ./examples/basic

bench: bench-quick

bench-quick:
	@echo "Running quick benchmarks..."
	@go test -bench=BenchmarkPool -benchtime=10x ./bench

bench-full:
	@echo "Running full benchmark suite..."
	@go test -bench=. -benchtime=100x -benchmem ./bench

bench-comparison: deps-python
	@echo "Running RPC protocol comparison benchmarks..."
	@echo "Installing Python dependencies for RPC servers..."
	@pip install -q msgpack 2>/dev/null || true
	@echo "Running benchmarks..."
	@go test -bench=BenchmarkRPC -benchtime=100x ./bench -v
	@echo ""
	@echo "=== Benchmark Summary ==="
	@go test -bench=BenchmarkRPC -benchtime=100x ./bench 2>/dev/null | grep -E "Benchmark|req/s|μs"

bench-comparison-report: deps-python
	@echo "Generating detailed comparison report..."
	@go test -bench=BenchmarkRPC -benchtime=1000x ./bench -benchmem > bench_results.txt
	@echo "Results saved to bench_results.txt"
	@echo ""
	@echo "=== Performance Comparison ==="
	@cat bench_results.txt | grep -E "Benchmark|req/s|μs" | column -t

deps-python:
	@echo "Checking Python dependencies..."
	@python3 -c "import msgpack" 2>/dev/null || pip install msgpack

test:
	@echo "Running tests..."
	@go test -v ./...

fmt:
	go fmt ./...

lint:
	@echo "Running linters..."
	@golangci-lint run ./...
	@cd worker/python && ruff check . || true

coverage:
	@echo "Running coverage..."
	@export GOMODCACHE=$(PWD)/.gomodcache; export GOCACHE=$(PWD)/.gocache; \
	pkgs=$$(go list ./internal/... | grep -v '/bench'); [ -z "$$pkgs" ] || TMPDIR=/tmp GOMODCACHE=$(PWD)/.gomodcache GOCACHE=$(PWD)/.gocache go test -covermode=atomic -coverprofile=coverage.out $$pkgs
	@GOMODCACHE=$(PWD)/.gomodcache GOCACHE=$(PWD)/.gocache go tool cover -func=coverage.out | tee coverage.txt
