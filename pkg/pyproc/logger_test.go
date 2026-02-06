package pyproc

import (
	"context"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"unknown", 0}, // fallback to info
	}

	for _, tt := range tests {
		if level := parseLogLevel(tt.input); int(level) != tt.expected {
			t.Errorf("parseLogLevel(%q) = %d, want %d", tt.input, level, tt.expected)
		}
	}
}

func TestTraceIDHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx)
	if id, ok := GetTraceID(ctx); !ok || id == 0 {
		t.Fatalf("trace id not set: %d %v", id, ok)
	}
}

func TestLoggerTraceEnabled(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "info", Format: "json", TraceEnabled: true})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	ctx := WithTraceID(context.Background())
	logger.InfoContext(ctx, "msg")
}

func TestLoggerAllLevels(_ *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "debug", Format: "json", TraceEnabled: true})
	ctx := WithTraceID(context.Background())

	// Test all log level methods with trace enabled
	logger.DebugContext(ctx, "debug message")
	logger.InfoContext(ctx, "info message")
	logger.WarnContext(ctx, "warn message")
	logger.ErrorContext(ctx, "error message")

	// Test with attributes
	logger.DebugContext(ctx, "debug with attr", "key", "value")
	logger.ErrorContext(ctx, "error with attr", "error", "test error")
	logger.WarnContext(ctx, "warn with attr", "warning", "test warning")

	// Test with trace disabled
	loggerNoTrace := NewLogger(LoggingConfig{Level: "debug"})
	ctxNoTrace := context.Background()
	loggerNoTrace.DebugContext(ctxNoTrace, "debug no trace")
	loggerNoTrace.WarnContext(ctxNoTrace, "warn no trace")
	loggerNoTrace.ErrorContext(ctxNoTrace, "error no trace")
}

func TestLoggerWithMethod(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "info"})
	methodLogger := logger.WithMethod("test_method")

	if methodLogger == nil {
		t.Fatal("expected non-nil logger from WithMethod")
	}

	ctx := context.Background()
	methodLogger.InfoContext(ctx, "message with method")
}

func TestLoggerWithWorker(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "info", TraceEnabled: true})
	workerLogger := logger.WithWorker("worker-1")

	if workerLogger == nil {
		t.Fatal("expected non-nil logger from WithWorker")
	}

	ctx := context.Background()
	workerLogger.InfoContext(ctx, "message with worker")
}

func TestLoggerWithRequestID(t *testing.T) {
	logger := NewLogger(LoggingConfig{Level: "info", TraceEnabled: true})
	reqLogger := logger.WithRequestID(42)

	if reqLogger == nil {
		t.Fatal("expected non-nil logger from WithRequestID")
	}

	ctx := context.Background()
	reqLogger.InfoContext(ctx, "message with request id")
}
