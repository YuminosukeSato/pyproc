package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestNewProvider_Disabled(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		ServiceName: "test",
		Enabled:     false,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	if provider.IsEnabled() {
		t.Error("provider should be disabled")
	}

	tracer := provider.Tracer("test")
	if tracer == nil {
		t.Error("tracer should not be nil")
	}
}

func TestNewProvider_Enabled(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		ServiceName: "test",
		Enabled:     true,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	if !provider.IsEnabled() {
		t.Error("provider should be enabled")
	}

	tracer := provider.Tracer("test")
	if tracer == nil {
		t.Error("tracer should not be nil")
	}
}

func TestNewProvider_Defaults(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		Enabled: true,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	// Should not panic with default config
	tracer := provider.Tracer("test")
	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-span")
	span.End()
}

func TestExtractTraceContext_ValidTraceparent(t *testing.T) {
	headers := map[string]string{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}

	ctx := context.Background()
	newCtx := ExtractTraceContext(ctx, headers)

	spanCtx := trace.SpanContextFromContext(newCtx)
	if !spanCtx.IsValid() {
		t.Error("span context should be valid")
	}

	expectedTraceID := "0af7651916cd43dd8448eb211c80319c"
	if spanCtx.TraceID().String() != expectedTraceID {
		t.Errorf("trace ID mismatch: got %s, want %s", spanCtx.TraceID().String(), expectedTraceID)
	}

	expectedSpanID := "b7ad6b7169203331"
	if spanCtx.SpanID().String() != expectedSpanID {
		t.Errorf("span ID mismatch: got %s, want %s", spanCtx.SpanID().String(), expectedSpanID)
	}

	if spanCtx.TraceFlags() != 0x01 {
		t.Errorf("trace flags mismatch: got %x, want 01", spanCtx.TraceFlags())
	}

	if !spanCtx.IsRemote() {
		t.Error("span context should be marked as remote")
	}
}

func TestExtractTraceContext_InvalidTraceparent(t *testing.T) {
	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name:    "empty headers",
			headers: map[string]string{},
		},
		{
			name:    "nil headers",
			headers: nil,
		},
		{
			name: "invalid format",
			headers: map[string]string{
				"traceparent": "invalid",
			},
		},
		{
			name: "invalid trace ID",
			headers: map[string]string{
				"traceparent": "00-INVALID-b7ad6b7169203331-01",
			},
		},
		{
			name: "invalid span ID",
			headers: map[string]string{
				"traceparent": "00-0af7651916cd43dd8448eb211c80319c-INVALID-01",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			newCtx := ExtractTraceContext(ctx, tc.headers)

			spanCtx := trace.SpanContextFromContext(newCtx)
			if spanCtx.IsValid() {
				t.Error("span context should not be valid for invalid traceparent")
			}
		})
	}
}

func TestInjectTraceContext_ValidSpan(t *testing.T) {
	// Create a provider with enabled tracing
	provider, shutdown := NewProvider(Config{
		ServiceName: "test",
		Enabled:     true,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	headers := make(map[string]string)
	InjectTraceContext(ctx, headers)

	traceparent, ok := headers["traceparent"]
	if !ok {
		t.Fatal("traceparent header should be set")
	}

	// Verify format: 00-<32-hex>-<16-hex>-<2-hex>
	t.Logf("traceparent: %s", traceparent)

	// Basic format check
	if len(traceparent) != 55 { // 2 + 1 + 32 + 1 + 16 + 1 + 2
		t.Errorf("traceparent length mismatch: got %d, want 55", len(traceparent))
	}
}

func TestInjectTraceContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	headers := make(map[string]string)

	InjectTraceContext(ctx, headers)

	if _, ok := headers["traceparent"]; ok {
		t.Error("traceparent should not be set when no span is present")
	}
}

func TestRoundTrip_InjectAndExtract(t *testing.T) {
	// Create a provider
	provider, shutdown := NewProvider(Config{
		ServiceName: "test",
		Enabled:     true,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Inject into headers
	headers := make(map[string]string)
	InjectTraceContext(ctx, headers)

	// Extract from headers
	newCtx := ExtractTraceContext(context.Background(), headers)

	// Verify trace context is preserved
	originalSpanCtx := trace.SpanContextFromContext(ctx)
	extractedSpanCtx := trace.SpanContextFromContext(newCtx)

	if originalSpanCtx.TraceID() != extractedSpanCtx.TraceID() {
		t.Errorf("trace ID mismatch: got %s, want %s",
			extractedSpanCtx.TraceID().String(),
			originalSpanCtx.TraceID().String())
	}

	if originalSpanCtx.SpanID() != extractedSpanCtx.SpanID() {
		t.Errorf("span ID mismatch: got %s, want %s",
			extractedSpanCtx.SpanID().String(),
			originalSpanCtx.SpanID().String())
	}
}

func TestProvider_Shutdown(t *testing.T) {
	provider, shutdown := NewProvider(Config{
		ServiceName: "test",
		Enabled:     true,
	})

	// Shutdown via function
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	// Shutdown via provider method
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Errorf("provider shutdown failed: %v", err)
	}
}

func TestProvider_ShutdownNoOp(t *testing.T) {
	provider, _ := NewProvider(Config{
		ServiceName: "test",
		Enabled:     false,
	})

	// Should not panic
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func BenchmarkInjectTraceContext(b *testing.B) {
	provider, shutdown := NewProvider(Config{
		ServiceName: "bench",
		Enabled:     true,
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	tracer := provider.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "bench-span")
	defer span.End()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		headers := make(map[string]string)
		InjectTraceContext(ctx, headers)
	}
}

func BenchmarkExtractTraceContext(b *testing.B) {
	headers := map[string]string{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractTraceContext(ctx, headers)
	}
}

func BenchmarkNoOpTracer(b *testing.B) {
	provider, shutdown := NewProvider(Config{
		ServiceName: "bench",
		Enabled:     false, // No-op mode
	})
	defer func() {
		_ = shutdown(context.Background()) //nolint:errcheck
	}()

	tracer := provider.Tracer("bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "bench-span")
		span.End()
	}
}
