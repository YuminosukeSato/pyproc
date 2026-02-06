# Benchmarks

Performance measurement and CI gates.

## Commands

```bash
# Quick run
go test -bench=BenchmarkPool -benchtime=10x .

# Full run (with memory profiling)
go test -bench=. -benchtime=100x -benchmem .

# Latency percentiles
go test -bench=BenchmarkLatencyPercentiles -benchtime=10s .
```

## CI Gate Thresholds

| Metric | Target | Notes |
|--------|--------|-------|
| p50 | < 100us | Simple function call |
| p99 | < 500us | Includes GC and process overhead |

Threshold violations trigger CI warnings (currently non-blocking).

## Adding Benchmarks

- `Benchmark` prefix required
- Reflect real use cases (synthetic benchmarks must be labeled separately)
- Include parallelism tests with varying worker counts
- Output both req/s and latency

## Notes

- Python worker required (`uv sync` must be completed)
- Socket files created at `/tmp/pyproc-*.sock`; cleaned up on exit
- CI and local results may differ (use relative comparisons, not absolute values)

## Review Guidelines

- Benchmark changes must include before/after comparison results
- Do not lower CI gate thresholds without explicit approval
- New benchmarks must test realistic workloads
