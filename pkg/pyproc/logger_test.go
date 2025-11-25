package pyproc

import (
	"context"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	if level := parseLogLevel("debug"); level != -4 {
		t.Fatalf("expected debug level, got %d", level)
	}
	if level := parseLogLevel("unknown"); level != 0 {
		t.Fatalf("expected info level fallback, got %d", level)
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
