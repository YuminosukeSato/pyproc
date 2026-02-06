package pyproc

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

// traceIDKey is the context key for trace ID
type traceIDKey struct{}

// traceIDCounter is used to generate unique trace IDs
var traceIDCounter atomic.Uint64

// Logger wraps slog.Logger with trace ID support
type Logger struct {
	*slog.Logger
	traceEnabled bool
}

// NewLogger creates a new logger with the specified configuration
func NewLogger(cfg LoggingConfig) *Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Level),
	}

	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger:       slog.New(handler),
		traceEnabled: cfg.TraceEnabled,
	}
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context) context.Context {
	traceID := traceIDCounter.Add(1)
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// GetTraceID retrieves the trace ID from the context
func GetTraceID(ctx context.Context) (uint64, bool) {
	id, ok := ctx.Value(traceIDKey{}).(uint64)
	return id, ok
}

// InfoContext logs an info message with trace ID if enabled
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	if l.traceEnabled {
		if traceID, ok := GetTraceID(ctx); ok {
			args = append([]any{"trace_id", traceID}, args...)
		}
	}
	l.Logger.InfoContext(ctx, msg, args...)
}

// ErrorContext logs an error message with trace ID if enabled
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	if l.traceEnabled {
		if traceID, ok := GetTraceID(ctx); ok {
			args = append([]any{"trace_id", traceID}, args...)
		}
	}
	l.Logger.ErrorContext(ctx, msg, args...)
}

// DebugContext logs a debug message with trace ID if enabled
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	if l.traceEnabled {
		if traceID, ok := GetTraceID(ctx); ok {
			args = append([]any{"trace_id", traceID}, args...)
		}
	}
	l.Logger.DebugContext(ctx, msg, args...)
}

// WarnContext logs a warning message with trace ID if enabled
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	if l.traceEnabled {
		if traceID, ok := GetTraceID(ctx); ok {
			args = append([]any{"trace_id", traceID}, args...)
		}
	}
	l.Logger.WarnContext(ctx, msg, args...)
}

// WithWorker returns a logger with worker ID attached
func (l *Logger) WithWorker(workerID string) *Logger {
	return &Logger{
		Logger:       l.With("worker_id", workerID),
		traceEnabled: l.traceEnabled,
	}
}

// WithMethod returns a logger with method name attached
func (l *Logger) WithMethod(method string) *Logger {
	return &Logger{
		Logger:       l.With("method", method),
		traceEnabled: l.traceEnabled,
	}
}

// WithRequestID returns a logger with request ID attached
func (l *Logger) WithRequestID(requestID uint64) *Logger {
	return &Logger{
		Logger:       l.With("request_id", requestID),
		traceEnabled: l.traceEnabled,
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
