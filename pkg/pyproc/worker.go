package pyproc

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerState represents the state of a worker
type WorkerState int32

const (
	// WorkerStateStopped indicates the worker is not running
	WorkerStateStopped WorkerState = iota
	// WorkerStateStarting indicates the worker is in the process of starting
	WorkerStateStarting
	// WorkerStateRunning indicates the worker is running and ready to accept requests
	WorkerStateRunning
	// WorkerStateStopping indicates the worker is in the process of stopping
	WorkerStateStopping
)

// WorkerConfig defines configuration for a single worker
type WorkerConfig struct {
	ID                string
	SocketPath        string
	PythonExec        string
	WorkerScript      string
	Env               map[string]string
	StartTimeout      time.Duration
	WorkerConcurrency int
}

// Worker represents a single Python worker process
type Worker struct {
	cfg    WorkerConfig
	logger *Logger

	cmd      *exec.Cmd
	cmdMu    sync.RWMutex
	waitOnce sync.Once
	waitErr  error
	state    atomic.Int32
	pid      atomic.Int32

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewWorker creates a new worker instance
func NewWorker(cfg WorkerConfig, logger *Logger) *Worker {
	if logger == nil {
		logger = NewLogger(LoggingConfig{Level: "info", Format: "text"})
	}

	return &Worker{
		cfg:    cfg,
		logger: logger.WithWorker(cfg.ID),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start starts the worker process
func (w *Worker) Start(ctx context.Context) error {
	if !w.state.CompareAndSwap(int32(WorkerStateStopped), int32(WorkerStateStarting)) {
		return fmt.Errorf("worker already started or starting")
	}

	w.logger.InfoContext(ctx, "Starting worker",
		"socket_path", w.cfg.SocketPath,
		"script", w.cfg.WorkerScript)

	// Reset wait-related fields for new process
	w.cmdMu.Lock()
	w.waitOnce = sync.Once{}
	w.waitErr = nil
	w.cmdMu.Unlock()

	// Clean up any existing socket file
	if err := os.Remove(w.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		w.logger.WarnContext(ctx, "Failed to remove existing socket file",
			"error", err)
	}

	// Create the command
	cmd := exec.CommandContext(ctx, w.cfg.PythonExec, w.cfg.WorkerScript)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range w.cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("PYPROC_SOCKET_PATH=%s", w.cfg.SocketPath))
	if w.cfg.WorkerConcurrency > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PYPROC_WORKER_CONCURRENCY=%d", w.cfg.WorkerConcurrency))
	}

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		w.state.Store(int32(WorkerStateStopped))
		return fmt.Errorf("failed to start worker process: %w", err)
	}

	w.cmdMu.Lock()
	w.cmd = cmd
	w.cmdMu.Unlock()

	w.pid.Store(int32(cmd.Process.Pid))
	w.logger.InfoContext(ctx, "Worker process started", "pid", cmd.Process.Pid)

	// Wait for the socket to be available
	socketReady := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		timeout := time.After(w.cfg.StartTimeout)
		for {
			select {
			case <-ticker.C:
				// Try to connect to the socket
				conn, err := net.Dial("unix", w.cfg.SocketPath)
				if err == nil {
					_ = conn.Close()
					socketReady <- nil
					return
				}
			case <-timeout:
				socketReady <- fmt.Errorf("worker start timeout after %v", w.cfg.StartTimeout)
				return
			case <-ctx.Done():
				socketReady <- ctx.Err()
				return
			}
		}
	}()

	// Start monitoring goroutine
	go w.monitor()

	// Wait for socket to be ready
	if err := <-socketReady; err != nil {
		if err := w.Stop(); err != nil {
			w.logger.Error("failed to stop worker after socket error", "error", err)
		}
		return err
	}

	w.state.Store(int32(WorkerStateRunning))
	w.logger.InfoContext(ctx, "Worker ready")

	return nil
}

// Stop stops the worker process
func (w *Worker) Stop() error {
	if !w.state.CompareAndSwap(int32(WorkerStateRunning), int32(WorkerStateStopping)) {
		// Also try from starting state
		if !w.state.CompareAndSwap(int32(WorkerStateStarting), int32(WorkerStateStopping)) {
			return nil // Already stopped or stopping
		}
	}

	w.logger.Info("Stopping worker")

	// Signal stop
	close(w.stopCh)

	// Get the command
	w.cmdMu.RLock()
	cmd := w.cmd
	w.cmdMu.RUnlock()

	if cmd != nil && cmd.Process != nil {
		// Try graceful shutdown first
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			w.logger.Warn("Failed to send interrupt signal", "error", err)
		}

		// Wait for process to exit with timeout
		done := make(chan error, 1)
		go func() {
			done <- w.wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill after timeout
			w.logger.Warn("Worker did not exit gracefully, forcing kill")
			if err := cmd.Process.Kill(); err != nil {
				w.logger.Error("Failed to kill worker process", "error", err)
			}
			<-done // Wait for process to be reaped
		}
	}

	// Clean up socket file
	if err := os.Remove(w.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		w.logger.Warn("Failed to remove socket file", "error", err)
	}

	// Wait for monitor to finish
	<-w.doneCh

	w.state.Store(int32(WorkerStateStopped))
	w.pid.Store(0)
	w.logger.Info("Worker stopped")

	return nil
}

// wait wraps cmd.Wait() to ensure it's called only once
func (w *Worker) wait() error {
	w.cmdMu.RLock()
	cmd := w.cmd
	w.cmdMu.RUnlock()

	if cmd != nil {
		w.waitOnce.Do(func() {
			err := cmd.Wait()
			w.cmdMu.Lock()
			w.waitErr = err
			w.cmdMu.Unlock()
		})
	}

	w.cmdMu.RLock()
	err := w.waitErr
	w.cmdMu.RUnlock()
	return err
}

// Restart restarts the worker process
func (w *Worker) Restart(ctx context.Context) error {
	w.logger.InfoContext(ctx, "Restarting worker")

	if err := w.Stop(); err != nil {
		return fmt.Errorf("failed to stop worker: %w", err)
	}

	// Reset channels for new process
	// Note: We don't reset waitOnce here as it can cause race conditions
	// Each new process will get its own waitOnce via Start()
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})

	if err := w.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	return nil
}

// monitor monitors the worker process and handles unexpected exits
func (w *Worker) monitor() {
	defer close(w.doneCh)

	w.cmdMu.RLock()
	cmd := w.cmd
	w.cmdMu.RUnlock()

	if cmd == nil {
		return
	}

	// Wait for either stop signal or process exit
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- w.wait()
	}()

	select {
	case <-w.stopCh:
		// Normal stop requested
		return
	case err := <-waitCh:
		// Process exited unexpectedly
		if w.state.Load() == int32(WorkerStateRunning) {
			if err != nil {
				w.logger.Error("Worker process exited unexpectedly", "error", err)
			} else {
				w.logger.Warn("Worker process exited unexpectedly with status 0")
			}
			w.state.Store(int32(WorkerStateStopped))
			w.pid.Store(0)
		}
	}
}

// IsRunning returns true if the worker is running
func (w *Worker) IsRunning() bool {
	return w.state.Load() == int32(WorkerStateRunning)
}

// GetState returns the current worker state
func (w *Worker) GetState() WorkerState {
	return WorkerState(w.state.Load())
}

// GetPID returns the process ID of the worker
func (w *Worker) GetPID() int {
	return int(w.pid.Load())
}

// GetID returns the worker ID
func (w *Worker) GetID() string {
	return w.cfg.ID
}

// GetSocketPath returns the socket path
func (w *Worker) GetSocketPath() string {
	return w.cfg.SocketPath
}
