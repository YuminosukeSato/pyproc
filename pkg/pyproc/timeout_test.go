package pyproc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTimeoutErrorMessagesAndUnwrap(t *testing.T) {
	var nilErr *TimeoutError
	if got := nilErr.Error(); got != "timeout" {
		t.Fatalf("unexpected nil error message: %s", got)
	}
	if nilErr.Unwrap() != nil {
		t.Fatal("expected nil unwrap for nil error")
	}

	err := &TimeoutError{
		Kind:    TimeoutKindContext,
		Timeout: 10 * time.Millisecond,
		Cause:   context.DeadlineExceeded,
	}
	if !strings.Contains(err.Error(), "Context timeout after") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expected errors.Is to match context.DeadlineExceeded")
	}

	err.Timeout = 0
	if got := err.Error(); got != "Context timeout" {
		t.Fatalf("unexpected error message without timeout: %s", got)
	}
}

func TestTimeoutDuration(t *testing.T) {
	start := time.Unix(0, 0)
	if got := timeoutDuration(start, time.Time{}); got != 0 {
		t.Fatalf("expected zero duration for zero deadline, got %s", got)
	}

	if got := timeoutDuration(start, start.Add(-time.Second)); got != 0 {
		t.Fatalf("expected zero duration for negative deadline, got %s", got)
	}

	if got := timeoutDuration(start, start.Add(time.Second)); got != time.Second {
		t.Fatalf("expected 1s duration, got %s", got)
	}
}

func TestEffectiveDeadline(t *testing.T) {
	start := time.Unix(0, 0)

	// Context deadline wins when shortest.
	ctxDeadline := start.Add(50 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), ctxDeadline)
	t.Cleanup(cancel)
	perCall := 100 * time.Millisecond
	deadline, kind, ok := effectiveDeadline(ctx, start, &perCall, 200*time.Millisecond)
	if !ok || kind != TimeoutKindContext || !deadline.Equal(ctxDeadline) {
		t.Fatalf("unexpected deadline selection: ok=%v kind=%s deadline=%v", ok, kind, deadline)
	}

	// Per-call wins when shortest.
	perCall = 20 * time.Millisecond
	deadline, kind, ok = effectiveDeadline(ctx, start, &perCall, 200*time.Millisecond)
	if !ok || kind != TimeoutKindPerCall || !deadline.Equal(start.Add(perCall)) {
		t.Fatalf("unexpected per-call deadline: ok=%v kind=%s deadline=%v", ok, kind, deadline)
	}

	// Transport wins when no context/per-call.
	deadline, kind, ok = effectiveDeadline(context.Background(), start, nil, 30*time.Millisecond)
	if !ok || kind != TimeoutKindTransport || !deadline.Equal(start.Add(30*time.Millisecond)) {
		t.Fatalf("unexpected transport deadline: ok=%v kind=%s deadline=%v", ok, kind, deadline)
	}

	// No deadlines at all.
	deadline, kind, ok = effectiveDeadline(context.Background(), start, nil, 0)
	if ok || kind != "" || !deadline.IsZero() {
		t.Fatalf("expected no deadline, got ok=%v kind=%s deadline=%v", ok, kind, deadline)
	}
}

type deadlineLessContext struct{}

func (deadlineLessContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (deadlineLessContext) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (deadlineLessContext) Err() error { return context.DeadlineExceeded }
func (deadlineLessContext) Value(_ interface{}) interface{} {
	return nil
}

func TestTimeoutErrorForContext(t *testing.T) {
	start := time.Unix(0, 0)

	ctxCanceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := timeoutErrorForContext(ctxCanceled, start); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	base := time.Now()
	ctxDeadline, cancelDeadline := context.WithDeadline(context.Background(), base.Add(-50*time.Millisecond))
	t.Cleanup(cancelDeadline)
	timeoutErr := timeoutErrorForContext(ctxDeadline, base.Add(-100*time.Millisecond))
	var te *TimeoutError
	if !errors.As(timeoutErr, &te) {
		t.Fatalf("expected TimeoutError, got %T", timeoutErr)
	}
	if te.Kind != TimeoutKindContext {
		t.Fatalf("unexpected TimeoutError.Kind: %s", te.Kind)
	}
	if te.Timeout != 50*time.Millisecond {
		t.Fatalf("unexpected timeout duration: %s", te.Timeout)
	}

	timeoutErr = timeoutErrorForContext(deadlineLessContext{}, start)
	if !errors.As(timeoutErr, &te) {
		t.Fatalf("expected TimeoutError, got %T", timeoutErr)
	}
	if te.Kind != TimeoutKindContext || te.Timeout != 0 {
		t.Fatalf("unexpected timeout error for deadline-less context: %+v", te)
	}
}
