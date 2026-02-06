package bench

import (
	"context"
	"testing"
	"time"

	"github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

// BenchmarkPool_Call_NoTracing measures baseline performance without OpenTelemetry.
// This is the reference baseline for all overhead calculations.
func BenchmarkPool_Call_NoTracing(b *testing.B) {
	pool := createTestPool(b, 4, "/tmp/bench-otel-baseline")
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	req := map[string]interface{}{"value": 42}
	var resp map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Call(ctx, "predict", req, &resp); err != nil {
			b.Fatalf("call failed: %v", err)
		}
	}
}

// BenchmarkPool_Call_TracingDisabled measures overhead when OTel SDK is present
// but tracing is disabled (no-op tracer). Overhead should be <1%.
func BenchmarkPool_Call_TracingDisabled(b *testing.B) {
	// Initialize OTel SDK with no-op tracer provider
	_, shutdown := telemetry.NewProvider(telemetry.Config{
		ServiceName: "bench-disabled",
		Enabled:     false, // No-op mode
	})
	defer shutdown(context.Background())

	pool := createTestPool(b, 4, "/tmp/bench-otel-disabled")
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	req := map[string]interface{}{"value": 42}
	var resp map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Call(ctx, "predict", req, &resp); err != nil {
			b.Fatalf("call failed: %v", err)
		}
	}
}

// BenchmarkPool_Call_TracingEnabled_NoSampling measures overhead when tracing
// is enabled but sampling rate is 0%. Should be close to TracingDisabled.
func BenchmarkPool_Call_TracingEnabled_NoSampling(b *testing.B) {
	// Initialize OTel SDK with 0% sampling
	_, shutdown := telemetry.NewProvider(telemetry.Config{
		ServiceName:  "bench-0pct",
		Enabled:      true,
		SamplingRate: 0.0, // Never sample
		ExporterType: "stdout",
	})
	defer shutdown(context.Background())

	pool := createTestPool(b, 4, "/tmp/bench-otel-0pct")
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	req := map[string]interface{}{"value": 42}
	var resp map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Call(ctx, "predict", req, &resp); err != nil {
			b.Fatalf("call failed: %v", err)
		}
	}
}

// BenchmarkPool_Call_TracingEnabled_1pctSampling measures overhead with 1% sampling.
// This is the target production configuration. Overhead must be <3%.
func BenchmarkPool_Call_TracingEnabled_1pctSampling(b *testing.B) {
	// Initialize OTel SDK with 1% sampling
	_, shutdown := telemetry.NewProvider(telemetry.Config{
		ServiceName:  "bench-1pct",
		Enabled:      true,
		SamplingRate: 0.01, // 1% sampling
		ExporterType: "stdout",
	})
	defer shutdown(context.Background())

	pool := createTestPool(b, 4, "/tmp/bench-otel-1pct")
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	req := map[string]interface{}{"value": 42}
	var resp map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Call(ctx, "predict", req, &resp); err != nil {
			b.Fatalf("call failed: %v", err)
		}
	}
}

// BenchmarkPool_Call_TracingEnabled_100pctSampling measures overhead with 100% sampling.
// This represents the worst-case scenario for overhead measurement.
func BenchmarkPool_Call_TracingEnabled_100pctSampling(b *testing.B) {
	// Initialize OTel SDK with 100% sampling
	_, shutdown := telemetry.NewProvider(telemetry.Config{
		ServiceName:  "bench-100pct",
		Enabled:      true,
		SamplingRate: 1.0, // 100% sampling
		ExporterType: "stdout",
	})
	defer shutdown(context.Background())

	pool := createTestPool(b, 4, "/tmp/bench-otel-100pct")
	defer func() { _ = pool.Shutdown(context.Background()) }()

	ctx := context.Background()
	req := map[string]interface{}{"value": 42}
	var resp map[string]interface{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Call(ctx, "predict", req, &resp); err != nil {
			b.Fatalf("call failed: %v", err)
		}
	}
}

// BenchmarkPool_Call_ObservabilityLatency measures latency percentiles with
// various tracing configurations to ensure performance gates are met.
func BenchmarkPool_Call_ObservabilityLatency(b *testing.B) {
	testCases := []struct {
		name         string
		socketPrefix string
		config       telemetry.Config
	}{
		{
			name:         "NoTracing",
			socketPrefix: "/tmp/bench-otel-latency-none",
			config:       telemetry.Config{ServiceName: "bench-lat-none", Enabled: false},
		},
		{
			name:         "1pctSampling",
			socketPrefix: "/tmp/bench-otel-latency-1pct",
			config: telemetry.Config{
				ServiceName:  "bench-lat-1pct",
				Enabled:      true,
				SamplingRate: 0.01,
				ExporterType: "stdout",
			},
		},
		{
			name:         "100pctSampling",
			socketPrefix: "/tmp/bench-otel-latency-100pct",
			config: telemetry.Config{
				ServiceName:  "bench-lat-100pct",
				Enabled:      true,
				SamplingRate: 1.0,
				ExporterType: "stdout",
			},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Setup tracer provider based on tc configuration
			_, shutdown := telemetry.NewProvider(tc.config)
			defer shutdown(context.Background())

			pool := createTestPool(b, 4, tc.socketPrefix)
			defer func() { _ = pool.Shutdown(context.Background()) }()

			ctx := context.Background()
			req := map[string]interface{}{"value": 42}
			var resp map[string]interface{}

			latencies := make([]time.Duration, 0, b.N)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				if err := pool.Call(ctx, "predict", req, &resp); err != nil {
					b.Fatalf("call failed: %v", err)
				}
				latencies = append(latencies, time.Since(start))
			}

			// Calculate percentiles
			p50 := calculatePercentile(latencies, 50)
			p95 := calculatePercentile(latencies, 95)
			p99 := calculatePercentile(latencies, 99)

			b.ReportMetric(float64(p50.Microseconds()), "p50_μs")
			b.ReportMetric(float64(p95.Microseconds()), "p95_μs")
			b.ReportMetric(float64(p99.Microseconds()), "p99_μs")

			// Performance gates
			if p50 > 100*time.Microsecond {
				b.Logf("WARNING: p50 latency %v exceeds target of 100µs", p50)
			}
			if p99 > 500*time.Microsecond {
				b.Logf("WARNING: p99 latency %v exceeds target of 500µs", p99)
			}
		})
	}
}

// BenchmarkPool_Call_ObservabilityOverhead measures the overhead of various
// tracing configurations compared to baseline. This is used for CI gates.
func BenchmarkPool_Call_ObservabilityOverhead(b *testing.B) {
	configurations := []struct {
		name         string
		socketPrefix string
		config       telemetry.Config
		maxOverhead  float64 // Maximum acceptable overhead percentage
		description  string
	}{
		{
			name:         "Baseline",
			socketPrefix: "/tmp/bench-overhead-baseline",
			config:       telemetry.Config{ServiceName: "bench-base", Enabled: false},
			maxOverhead:  0.0,
			description:  "No tracing",
		},
		{
			name:         "NoOp",
			socketPrefix: "/tmp/bench-overhead-noop",
			config:       telemetry.Config{ServiceName: "bench-noop", Enabled: false},
			maxOverhead:  1.0,
			description:  "OTel SDK present but disabled",
		},
		{
			name:         "1pct",
			socketPrefix: "/tmp/bench-overhead-1pct",
			config: telemetry.Config{
				ServiceName:  "bench-oh-1pct",
				Enabled:      true,
				SamplingRate: 0.01,
				ExporterType: "stdout",
			},
			maxOverhead: 3.0,
			description: "1% sampling (production target)",
		},
		{
			name:         "100pct",
			socketPrefix: "/tmp/bench-overhead-100pct",
			config: telemetry.Config{
				ServiceName:  "bench-oh-100pct",
				Enabled:      true,
				SamplingRate: 1.0,
				ExporterType: "stdout",
			},
			maxOverhead: 5.0,
			description: "100% sampling (worst case)",
		},
	}

	// Store baseline performance for comparison
	var baselineNsPerOp float64

	for i, cfg := range configurations {
		b.Run(cfg.name, func(b *testing.B) {
			// Setup appropriate tracer provider based on configuration
			_, shutdown := telemetry.NewProvider(cfg.config)
			defer shutdown(context.Background())

			pool := createTestPool(b, 4, cfg.socketPrefix)
			defer func() { _ = pool.Shutdown(context.Background()) }()

			ctx := context.Background()
			req := map[string]interface{}{"value": 42}
			var resp map[string]interface{}

			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				if err := pool.Call(ctx, "predict", req, &resp); err != nil {
					b.Fatalf("call failed: %v", err)
				}
			}

			// Calculate and report overhead
			if i == 0 {
				// This is the baseline
				baselineNsPerOp = float64(b.Elapsed().Nanoseconds()) / float64(b.N)
				b.ReportMetric(0, "overhead_%")
			} else if baselineNsPerOp > 0 {
				currentNsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
				overheadPct := ((currentNsPerOp - baselineNsPerOp) / baselineNsPerOp) * 100
				b.ReportMetric(overheadPct, "overhead_%")

				// Check against gate
				if overheadPct > cfg.maxOverhead {
					b.Errorf("PERFORMANCE GATE FAILED: %s overhead %.2f%% exceeds limit %.2f%%",
						cfg.description, overheadPct, cfg.maxOverhead)
				} else {
					b.Logf("PASS: %s overhead %.2f%% within limit %.2f%%",
						cfg.description, overheadPct, cfg.maxOverhead)
				}
			}
		})
	}
}

// BenchmarkPool_Call_ObservabilityMemory measures memory overhead of tracing.
func BenchmarkPool_Call_ObservabilityMemory(b *testing.B) {
	configurations := []struct {
		name         string
		socketPrefix string
		config       telemetry.Config
	}{
		{
			name:         "NoTracing",
			socketPrefix: "/tmp/bench-mem-none",
			config:       telemetry.Config{ServiceName: "bench-mem-none", Enabled: false},
		},
		{
			name:         "1pctSampling",
			socketPrefix: "/tmp/bench-mem-1pct",
			config: telemetry.Config{
				ServiceName:  "bench-mem-1pct",
				Enabled:      true,
				SamplingRate: 0.01,
				ExporterType: "stdout",
			},
		},
		{
			name:         "100pctSampling",
			socketPrefix: "/tmp/bench-mem-100pct",
			config: telemetry.Config{
				ServiceName:  "bench-mem-100pct",
				Enabled:      true,
				SamplingRate: 1.0,
				ExporterType: "stdout",
			},
		},
	}

	for _, cfg := range configurations {
		b.Run(cfg.name, func(b *testing.B) {
			// Setup appropriate tracer provider
			_, shutdown := telemetry.NewProvider(cfg.config)
			defer shutdown(context.Background())

			pool := createTestPool(b, 4, cfg.socketPrefix)
			defer func() { _ = pool.Shutdown(context.Background()) }()

			ctx := context.Background()
			req := map[string]interface{}{"value": 42}
			var resp map[string]interface{}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := pool.Call(ctx, "predict", req, &resp); err != nil {
					b.Fatalf("call failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkPool_Call_ObservabilityStats reports detailed statistics for analysis
func BenchmarkPool_Call_ObservabilityStats(b *testing.B) {
	b.Run("DetailedAnalysis", func(b *testing.B) {
		// Use 1% sampling as production target
		_, shutdown := telemetry.NewProvider(telemetry.Config{
			ServiceName:  "bench-stats",
			Enabled:      true,
			SamplingRate: 0.01,
			ExporterType: "stdout",
		})
		defer shutdown(context.Background())

		pool := createTestPool(b, 4, "/tmp/bench-otel-stats")
		defer func() { _ = pool.Shutdown(context.Background()) }()

		ctx := context.Background()
		req := map[string]interface{}{"value": 42}
		var resp map[string]interface{}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			if err := pool.Call(ctx, "predict", req, &resp); err != nil {
				b.Fatalf("call failed: %v", err)
			}
		}

		// Log additional metrics for analysis
		nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		opsPerSec := 1e9 / nsPerOp

		b.Logf("Performance: %.2f ns/op, %.2f ops/sec", nsPerOp, opsPerSec)
	})
}
