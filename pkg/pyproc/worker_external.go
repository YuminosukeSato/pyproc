package pyproc

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// ExternalWorkerState represents the state of an external worker.
type ExternalWorkerState int32

const (
	// ExternalWorkerStopped indicates the external worker is not connected.
	ExternalWorkerStopped ExternalWorkerState = iota
	// ExternalWorkerRunning indicates the external worker is connected and healthy.
	ExternalWorkerRunning
)

const (
	defaultConnectTimeout = 5 * time.Second
	// defaultMaxRetries with defaultRetryInterval yields ~4 min total wait
	// (500ms + 1s + 2s + 4s + 8s + 16s + 32s + 64s + 128s ≈ 255.5s).
	defaultMaxRetries    = 10
	defaultRetryInterval = 500 * time.Millisecond
)

// ExternalWorkerOptions configures an ExternalWorker.
type ExternalWorkerOptions struct {
	// SocketPath is the Unix Domain Socket path to connect to.
	SocketPath string
	// ConnectTimeout controls how long each dial attempt waits.
	// If zero, defaults to 5s.
	ConnectTimeout time.Duration
	// MaxRetries is the maximum number of connection retry attempts in Start.
	// If zero, defaults to 10.
	MaxRetries int
	// RetryInterval is the initial interval between retries. Each subsequent
	// retry doubles the interval (exponential backoff).
	// If zero, defaults to 500ms.
	RetryInterval time.Duration
}

// ExternalWorker represents a pre-existing Python worker process managed
// outside of pyproc (e.g. a Kubernetes sidecar container). It connects to the
// worker via a well-known Unix Domain Socket path rather than spawning a child
// process.
type ExternalWorker struct {
	socketPath     string
	connectTimeout time.Duration
	maxRetries     int
	retryInterval  time.Duration
	state          atomic.Int32
}

// NewExternalWorker creates a new ExternalWorker that connects to the given
// Unix Domain Socket path. The connectTimeout controls how long dial attempts
// wait; if zero, a default of 5 s is used. For retry support, use
// NewExternalWorkerWithOptions instead.
func NewExternalWorker(socketPath string, connectTimeout time.Duration) *ExternalWorker {
	return NewExternalWorkerWithOptions(ExternalWorkerOptions{
		SocketPath:     socketPath,
		ConnectTimeout: connectTimeout,
		MaxRetries:     1, // no retry for backward compat
	})
}

// NewExternalWorkerWithOptions creates a new ExternalWorker from options.
func NewExternalWorkerWithOptions(opts ExternalWorkerOptions) *ExternalWorker {
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = defaultConnectTimeout
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = defaultMaxRetries
	}
	if opts.RetryInterval <= 0 {
		opts.RetryInterval = defaultRetryInterval
	}
	return &ExternalWorker{
		socketPath:     opts.SocketPath,
		connectTimeout: opts.ConnectTimeout,
		maxRetries:     opts.MaxRetries,
		retryInterval:  opts.RetryInterval,
	}
}

// Start verifies that the external worker's socket is reachable. It retries
// with exponential backoff according to the configured MaxRetries and
// RetryInterval. It does not spawn a process; the worker must already be
// running.
//
// In production, callers should pass a context with a deadline to bound the
// total wait time (e.g. context.WithTimeout). Without a deadline, Start may
// block for the full backoff duration (~4 min with defaults).
func (w *ExternalWorker) Start(ctx context.Context) error {
	var lastErr error
	interval := w.retryInterval
	for i := 0; i < w.maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("external worker connection cancelled: %w", ctx.Err())
			case <-time.After(interval):
			}
			interval *= 2
		}
		conn, err := net.DialTimeout("unix", w.socketPath, w.connectTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.Close()
		w.state.Store(int32(ExternalWorkerRunning))
		return nil
	}
	return fmt.Errorf("external worker socket unreachable at %s after %d attempts: %w",
		w.socketPath, w.maxRetries, lastErr)
}

// Stop transitions the external worker to the stopped state. It does not
// terminate the remote process since pyproc does not own it.
func (w *ExternalWorker) Stop() error {
	w.state.Store(int32(ExternalWorkerStopped))
	return nil
}

// IsHealthy returns true if the external worker's socket is connectable.
func (w *ExternalWorker) IsHealthy(_ context.Context) bool {
	conn, err := net.DialTimeout("unix", w.socketPath, w.connectTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// GetSocketPath returns the Unix Domain Socket path for this worker.
func (w *ExternalWorker) GetSocketPath() string {
	return w.socketPath
}
