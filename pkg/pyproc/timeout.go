package pyproc

import (
	"context"
	"fmt"
	"time"
)

// TimeoutKind identifies the source of a timeout.
type TimeoutKind string

// Timeout kind constants identify the source of a timeout.
const (
	TimeoutKindContext   TimeoutKind = "Context"
	TimeoutKindPerCall   TimeoutKind = "PerCall"
	TimeoutKindTransport TimeoutKind = "Transport"
)

// TimeoutError represents a classified timeout error.
// It unwraps to context.DeadlineExceeded for errors.Is compatibility.
type TimeoutError struct {
	Kind    TimeoutKind
	Timeout time.Duration
	Cause   error
}

// Error returns a human-readable timeout error message.
func (e *TimeoutError) Error() string {
	if e == nil {
		return "timeout"
	}
	if e.Timeout > 0 {
		return fmt.Sprintf("%s timeout after %s", e.Kind, e.Timeout)
	}
	return fmt.Sprintf("%s timeout", e.Kind)
}

// Unwrap returns the underlying cause.
func (e *TimeoutError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newTimeoutError(kind TimeoutKind, timeout time.Duration, cause error) *TimeoutError {
	return &TimeoutError{
		Kind:    kind,
		Timeout: timeout,
		Cause:   cause,
	}
}

func timeoutDuration(start, deadline time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	d := deadline.Sub(start)
	if d < 0 {
		return 0
	}
	return d
}

// effectiveDeadline returns the earliest deadline based on context, per-call, and transport defaults.
// The returned kind indicates which source won.
func effectiveDeadline(ctx context.Context, start time.Time, perCall *time.Duration, transportDefault time.Duration) (time.Time, TimeoutKind, bool) {
	var (
		deadline time.Time
		kind     TimeoutKind
		ok       bool
	)

	if ctxDeadline, hasCtx := ctx.Deadline(); hasCtx {
		deadline = ctxDeadline
		kind = TimeoutKindContext
		ok = true
	}

	if perCall != nil {
		perDeadline := start.Add(*perCall)
		if !ok || perDeadline.Before(deadline) {
			deadline = perDeadline
			kind = TimeoutKindPerCall
			ok = true
		}
	}

	if transportDefault > 0 {
		transportDeadline := start.Add(transportDefault)
		if !ok || transportDeadline.Before(deadline) {
			deadline = transportDeadline
			kind = TimeoutKindTransport
			ok = true
		}
	}

	return deadline, kind, ok
}

func timeoutErrorForContext(ctx context.Context, start time.Time) error {
	if ctx.Err() != context.DeadlineExceeded {
		return ctx.Err()
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return newTimeoutError(TimeoutKindContext, 0, context.DeadlineExceeded)
	}
	return newTimeoutError(TimeoutKindContext, timeoutDuration(start, deadline), context.DeadlineExceeded)
}
