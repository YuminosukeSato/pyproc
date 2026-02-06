// Package telemetry provides OpenTelemetry tracing infrastructure for pyproc.
//
// This package implements distributed tracing support with the following key features:
//   - Zero-overhead no-op mode when tracing is disabled
//   - Backward compatibility (existing API unchanged)
//   - Trace context propagation over Unix Domain Sockets
//   - Integration with Pool.Call() for automatic span creation
//
// Usage:
//
//	// Initialize telemetry provider
//	provider, shutdown := telemetry.NewProvider(telemetry.Config{
//	    ServiceName: "my-service",
//	    Enabled:     true,
//	})
//	defer shutdown(context.Background())
//
//	// Create pool with telemetry
//	pool, _ := pyproc.NewPool(poolOpts, logger)
//	pool.WithTelemetry(provider.Tracer("my-service"))
//
//	// Calls are automatically traced
//	ctx := context.Background()
//	var result map[string]interface{}
//	pool.Call(ctx, "predict", input, &result)
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config holds configuration for telemetry provider
type Config struct {
	// ServiceName is the name of the service for tracing
	ServiceName string

	// Enabled determines whether tracing is active
	// When false, a no-op tracer is used with zero overhead
	Enabled bool

	// SamplingRate controls the fraction of traces to record (0.0 to 1.0)
	// Default is 1.0 (record all traces)
	SamplingRate float64

	// ExporterType determines which exporter to use
	// Supported values: "stdout", "otlp" (future)
	// Default is "stdout"
	ExporterType string
}

// Provider wraps an OpenTelemetry TracerProvider
type Provider struct {
	provider trace.TracerProvider
	shutdown func(context.Context) error
}

// NewProvider creates a new telemetry provider based on the given configuration.
// Returns a Provider and a shutdown function that should be called on application exit.
//
// When Config.Enabled is false, returns a no-op provider with zero overhead.
func NewProvider(cfg Config) (*Provider, func(context.Context) error) {
	if !cfg.Enabled {
		return &Provider{
			provider: noop.NewTracerProvider(),
			shutdown: func(context.Context) error { return nil },
		}, func(context.Context) error { return nil }
	}

	// Set defaults
	if cfg.ServiceName == "" {
		cfg.ServiceName = "pyproc"
	}
	if cfg.SamplingRate == 0 {
		cfg.SamplingRate = 1.0
	}
	if cfg.ExporterType == "" {
		cfg.ExporterType = "stdout"
	}

	// Create resource with service name
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		// Fallback to default resource if creation fails
		res = resource.Default()
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch cfg.ExporterType {
	case "stdout":
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			// Fallback to no-op on error
			return &Provider{
				provider: noop.NewTracerProvider(),
				shutdown: func(context.Context) error { return nil },
			}, func(context.Context) error { return nil }
		}
	default:
		// Future: Add OTLP exporter support
		return &Provider{
			provider: noop.NewTracerProvider(),
			shutdown: func(context.Context) error { return nil },
		}, func(context.Context) error { return nil }
	}

	// Create sampler based on sampling rate
	var sampler sdktrace.Sampler
	if cfg.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global provider
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	return &Provider{
		provider: tp,
		shutdown: shutdown,
	}, shutdown
}

// Tracer returns a tracer with the given name
func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return p.provider.Tracer(name, opts...)
}

// Shutdown gracefully shuts down the provider, flushing any remaining spans
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.shutdown != nil {
		return p.shutdown(ctx)
	}
	return nil
}

// IsEnabled returns true if tracing is enabled (not a no-op provider)
func (p *Provider) IsEnabled() bool {
	_, ok := p.provider.(noop.TracerProvider)
	return !ok
}

// ExtractTraceContext extracts OpenTelemetry trace context from a map.
// This is used for propagating trace context across process boundaries (UDS).
//
// The trace context is stored in W3C Trace Context format:
//   - "traceparent": "00-<trace-id>-<span-id>-<flags>"
//   - "tracestate": "<vendor-specific-state>" (optional)
//
// Returns a new context with the extracted span context.
func ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	if headers == nil {
		return ctx
	}

	// Parse traceparent header (W3C Trace Context format)
	// Format: version-trace-id-parent-id-flags
	traceparent := headers["traceparent"]
	if traceparent == "" {
		return ctx
	}

	// Parse the traceparent string
	var version, traceID, spanID, flags string
	n, err := fmt.Sscanf(traceparent, "%2s-%32s-%16s-%2s", &version, &traceID, &spanID, &flags)
	if err != nil || n != 4 {
		return ctx
	}

	// Parse trace ID
	tid, err := trace.TraceIDFromHex(traceID)
	if err != nil {
		return ctx
	}

	// Parse span ID
	sid, err := trace.SpanIDFromHex(spanID)
	if err != nil {
		return ctx
	}

	// Parse flags
	var flagsByte byte
	_, err = fmt.Sscanf(flags, "%02x", &flagsByte)
	if err != nil {
		return ctx
	}

	// Create span context
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.TraceFlags(flagsByte),
		Remote:     true,
	})

	// Return context with span context
	return trace.ContextWithRemoteSpanContext(ctx, spanCtx)
}

// InjectTraceContext injects OpenTelemetry trace context into a map.
// This is used for propagating trace context across process boundaries (UDS).
//
// The trace context is stored in W3C Trace Context format:
//   - "traceparent": "00-<trace-id>-<span-id>-<flags>"
//
// If the context does not contain a span, returns nil without modifying headers.
func InjectTraceContext(ctx context.Context, headers map[string]string) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return
	}

	// Format traceparent header
	// TraceFlags is a byte, format as 2-digit hex
	traceparent := fmt.Sprintf("00-%s-%s-%02x",
		spanCtx.TraceID().String(),
		spanCtx.SpanID().String(),
		byte(spanCtx.TraceFlags()),
	)
	headers["traceparent"] = traceparent

	// Future: Add tracestate support if needed
}
