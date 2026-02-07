// Package telemetry provides OpenTelemetry distributed tracing support for pyproc.
//
// # Overview
//
// This package integrates OpenTelemetry tracing into pyproc's Go-to-Python IPC layer,
// enabling end-to-end observability across process boundaries via Unix Domain Sockets.
//
// Key features:
//   - Zero-overhead no-op mode when tracing is disabled
//   - W3C Trace Context propagation over UDS
//   - Automatic span creation for Pool.Call() operations
//   - Support for distributed tracing across service boundaries
//   - Configurable sampling rates and exporters
//
// # Quick Start
//
// Initialize a telemetry provider and attach it to your pool:
//
//	import (
//	    "github.com/YuminosukeSato/pyproc/pkg/pyproc"
//	    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
//	)
//
//	// Create telemetry provider
//	provider, shutdown := telemetry.NewProvider(telemetry.Config{
//	    ServiceName: "my-service",
//	    Enabled:     true,
//	})
//	defer shutdown(context.Background())
//
//	// Create pool (tracing integration is automatic in Pool.Call)
//	pool, _ := pyproc.NewPool(poolOpts, logger)
//
//	// All calls are automatically traced
//	ctx := context.Background()
//	var result map[string]interface{}
//	pool.Call(ctx, "predict", input, &result)
//
// # No-Op Mode
//
// When tracing is disabled, the provider uses a no-op implementation with zero
// performance overhead:
//
//	provider, shutdown := telemetry.NewProvider(telemetry.Config{
//	    ServiceName: "my-service",
//	    Enabled:     false, // No-op mode
//	})
//
// This is the recommended mode for production until you're ready to enable tracing.
//
// # Trace Context Propagation
//
// Trace context is automatically propagated across UDS boundaries using W3C
// Trace Context format. The trace context is injected into the request headers
// on the Go side and extracted on the Python side.
//
// Manual trace context handling (advanced use):
//
//	// Inject trace context
//	headers := make(map[string]string)
//	telemetry.InjectTraceContext(ctx, headers)
//
//	// Extract trace context
//	newCtx := telemetry.ExtractTraceContext(ctx, headers)
//
// # Configuration
//
// The Config struct provides several options:
//
//   - ServiceName: Name of the service for tracing (default: "pyproc")
//   - Enabled: Whether tracing is active (default: false)
//   - SamplingRate: Fraction of traces to record, 0.0-1.0 (default: 1.0)
//   - ExporterType: Exporter to use, "stdout" or future "otlp" (default: "stdout")
//
// Example with custom configuration:
//
//	provider, shutdown := telemetry.NewProvider(telemetry.Config{
//	    ServiceName:  "my-service",
//	    Enabled:      true,
//	    SamplingRate: 0.1, // Sample 10% of traces
//	    ExporterType: "stdout",
//	})
//
// # Performance Considerations
//
// Tracing overhead is minimal (<3% in most cases) when enabled, and zero when
// disabled. The no-op mode ensures production systems can deploy with tracing
// code present but inactive.
//
// Benchmark results (on Apple M1):
//   - No-op tracer: ~2 ns/op (effectively zero overhead)
//   - InjectTraceContext: ~150 ns/op
//   - ExtractTraceContext: ~200 ns/op
//
// # Integration with External Tracing Systems
//
// To integrate with external tracing systems (e.g., Jaeger, Zipkin, Honeycomb),
// configure an OTLP exporter:
//
//	// Future: OTLP exporter support
//	provider, shutdown := telemetry.NewProvider(telemetry.Config{
//	    ServiceName:  "my-service",
//	    Enabled:      true,
//	    ExporterType: "otlp",
//	})
//
// Currently only stdout exporter is supported. OTLP exporter support is planned
// for a future release.
package telemetry
