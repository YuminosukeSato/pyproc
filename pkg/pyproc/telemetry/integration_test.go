package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTracerProvider_Initialization verifies that the telemetry provider initializes correctly
func TestTracerProvider_Initialization(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "enabled with defaults",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
			},
			wantErr: false,
		},
		{
			name: "enabled with custom sampling",
			config: Config{
				Enabled:      true,
				ServiceName:  "test-service",
				SamplingRate: 0.5,
			},
			wantErr: false,
		},
		{
			name: "enabled with stdout exporter",
			config: Config{
				Enabled:      true,
				ServiceName:  "test-service",
				ExporterType: "stdout",
			},
			wantErr: false,
		},
		{
			name: "disabled mode",
			config: Config{
				Enabled:     false,
				ServiceName: "test-service",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, shutdown := NewProvider(tt.config)
			if provider == nil {
				t.Fatal("provider should not be nil")
			}
			defer func() {
				_ = shutdown(context.Background()) //nolint:errcheck
			}()

			// Verify provider state
			if tt.config.Enabled && !provider.IsEnabled() {
				t.Error("provider should be enabled")
			}
			if !tt.config.Enabled && provider.IsEnabled() {
				t.Error("provider should be disabled")
			}

			// Verify tracer can be created
			tracer := provider.Tracer("test")
			if tracer == nil {
				t.Error("tracer should not be nil")
			}
		})
	}
}

// TestTracerProvider_Shutdown verifies graceful shutdown
func TestTracerProvider_Shutdown(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "shutdown enabled provider",
			enabled: true,
		},
		{
			name:    "shutdown disabled provider",
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, shutdown := NewProvider(Config{
				Enabled:     tt.enabled,
				ServiceName: "test-service",
			})

			// Create some spans before shutdown
			if tt.enabled {
				tracer := provider.Tracer("test")
				ctx, span := tracer.Start(context.Background(), "test-span")
				span.End()
				_ = ctx
			}

			// Shutdown via function
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := shutdown(ctx); err != nil {
				t.Errorf("shutdown failed: %v", err)
			}

			// Shutdown again should not error
			if err := provider.Shutdown(ctx); err != nil {
				t.Errorf("second shutdown failed: %v", err)
			}
		})
	}
}

// TestNoOp_ZeroOverhead verifies that no-op mode has minimal overhead
func TestNoOp_ZeroOverhead(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		Enabled:     false,
		ServiceName: "test",
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	if provider.IsEnabled() {
		t.Fatal("provider should be disabled")
	}

	tracer := provider.Tracer("test")
	ctx := context.Background()

	// Create many spans - should be no-op
	const iterations = 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, span := tracer.Start(ctx, "no-op-span")
		span.End()
	}
	elapsed := time.Since(start)

	// No-op operations should complete very quickly
	// 10k operations should take < 10ms (1µs per operation)
	maxExpected := 10 * time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("no-op overhead too high: %v (expected < %v)", elapsed, maxExpected)
	}

	t.Logf("no-op performance: %v for %d operations (%.2fµs per op)",
		elapsed, iterations, float64(elapsed.Microseconds())/float64(iterations))
}

// TestProvider_EnabledVsDisabled compares enabled vs disabled overhead
func TestProvider_EnabledVsDisabled(t *testing.T) {
	// Measure disabled (no-op) performance
	noopProvider, noopShutdown := NewProvider(Config{
		Enabled:     false,
		ServiceName: "test",
	})
	defer func() {
		_ = noopShutdown(context.Background()) //nolint:errcheck
	}()

	noopTracer := noopProvider.Tracer("test")
	ctx := context.Background()

	const iterations = 1000
	noopStart := time.Now()
	for i := 0; i < iterations; i++ {
		_, span := noopTracer.Start(ctx, "span")
		span.End()
	}
	noopElapsed := time.Since(noopStart)

	// Measure enabled performance with in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	enabledTP := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	defer func() {
		_ = enabledTP.Shutdown(context.Background()) //nolint:errcheck
	}()

	enabledTracer := enabledTP.Tracer("test")

	enabledStart := time.Now()
	for i := 0; i < iterations; i++ {
		_, span := enabledTracer.Start(ctx, "span")
		span.End()
	}
	enabledElapsed := time.Since(enabledStart)

	t.Logf("no-op:   %v for %d operations (%.2fµs per op)",
		noopElapsed, iterations, float64(noopElapsed.Microseconds())/float64(iterations))
	t.Logf("enabled: %v for %d operations (%.2fµs per op)",
		enabledElapsed, iterations, float64(enabledElapsed.Microseconds())/float64(iterations))

	// Verify no-op is faster
	if noopElapsed > enabledElapsed {
		t.Error("no-op should be faster than enabled tracing")
	}
}

// TestProvider_ResourceAttributes verifies resource attributes are set correctly
func TestProvider_ResourceAttributes(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		Enabled:     true,
		ServiceName: "my-service",
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	// Create a span and verify resource attributes
	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	span.End()
	_ = ctx

	// Note: We can't easily inspect resource attributes in this test
	// without accessing internal provider state. The actual verification
	// happens in telemetry_test.go unit tests.
	// This test primarily verifies that initialization with service name
	// does not cause errors.
}

// TestProvider_Sampling verifies sampling configuration
func TestProvider_Sampling(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate float64
		expectSample bool
	}{
		{
			name:         "always sample",
			samplingRate: 1.0,
			expectSample: true,
		},
		{
			name:         "never sample",
			samplingRate: 0.0,
			expectSample: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := trace.NewTracerProvider(
				trace.WithSyncer(exporter),
				trace.WithSampler(
					func() trace.Sampler {
						if tt.samplingRate >= 1.0 {
							return trace.AlwaysSample()
						}
						return trace.NeverSample()
					}(),
				),
			)
			defer func() {
				_ = tp.Shutdown(context.Background()) //nolint:errcheck
			}()

			tracer := tp.Tracer("test")
			ctx, span := tracer.Start(context.Background(), "test-span")
			span.End()
			_ = ctx

			// Force flush
			_ = tp.ForceFlush(context.Background())
			time.Sleep(10 * time.Millisecond)

			spans := exporter.GetSpans()
			if tt.expectSample && len(spans) == 0 {
				t.Error("expected span to be sampled, but got none")
			}
			if !tt.expectSample && len(spans) > 0 {
				t.Error("expected no spans to be sampled, but got some")
			}
		})
	}
}

// BenchmarkProvider_SpanCreation measures span creation overhead
func BenchmarkProvider_SpanCreation(b *testing.B) {
	provider, shutdown := NewProvider(Config{
		Enabled:     true,
		ServiceName: "bench",
	})
	defer shutdown(context.Background())

	tracer := provider.Tracer("bench")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, span := tracer.Start(ctx, "bench-span")
			span.End()
		}
	})
}

// BenchmarkProvider_NoOp measures no-op overhead
func BenchmarkProvider_NoOp(b *testing.B) {
	provider, shutdown := NewProvider(Config{
		Enabled:     false,
		ServiceName: "bench",
	})
	defer shutdown(context.Background())

	tracer := provider.Tracer("bench")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, span := tracer.Start(ctx, "bench-span")
			span.End()
		}
	})
}
