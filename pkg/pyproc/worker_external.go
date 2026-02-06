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

const defaultConnectTimeout = 5 * time.Second

// ExternalWorker represents a pre-existing Python worker process managed
// outside of pyproc (e.g. a Kubernetes sidecar container). It connects to the
// worker via a well-known Unix Domain Socket path rather than spawning a child
// process.
type ExternalWorker struct {
	socketPath     string
	connectTimeout time.Duration
	state          atomic.Int32
}

// NewExternalWorker creates a new ExternalWorker that connects to the given
// Unix Domain Socket path. The connectTimeout controls how long dial attempts
// wait; if zero, a default of 5 s is used.
func NewExternalWorker(socketPath string, connectTimeout time.Duration) *ExternalWorker {
	if connectTimeout <= 0 {
		connectTimeout = defaultConnectTimeout
	}
	return &ExternalWorker{
		socketPath:     socketPath,
		connectTimeout: connectTimeout,
	}
}

// Start verifies that the external worker's socket is reachable. It does not
// spawn a process; the worker must already be running.
func (w *ExternalWorker) Start(_ context.Context) error {
	conn, err := net.DialTimeout("unix", w.socketPath, w.connectTimeout)
	if err != nil {
		return fmt.Errorf("external worker socket unreachable at %s: %w", w.socketPath, err)
	}
	_ = conn.Close()

	w.state.Store(int32(ExternalWorkerRunning))
	return nil
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
