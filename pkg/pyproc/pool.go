package pyproc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/framing"
	"github.com/YuminosukeSato/pyproc/internal/protocol"
	"github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// PoolOptions provides additional options for creating a pool
type PoolOptions struct {
	Config       PoolConfig   // Base pool configuration
	WorkerConfig WorkerConfig // Configuration for each worker

	// ExternalMode, when true, tells the pool to connect to pre-existing
	// worker processes (e.g. Kubernetes sidecar containers) instead of
	// spawning child processes. ExternalSocketPaths must list the UDS paths.
	ExternalMode        bool
	ExternalSocketPaths []string

	// ExternalMaxRetries controls the number of connection retry attempts
	// for external workers. If zero, defaults to 10.
	ExternalMaxRetries int
	// ExternalRetryInterval is the initial retry interval for external
	// workers. Each retry doubles the interval. If zero, defaults to 500ms.
	ExternalRetryInterval time.Duration
}

type workerHandle interface {
	Start(context.Context) error
	Stop() error
	IsHealthy(context.Context) bool
	GetSocketPath() string
}

// Pool manages multiple Python workers with load balancing
type Pool struct {
	opts          PoolOptions
	logger        *Logger
	workers       []*poolWorker
	nextIdx       atomic.Uint64
	shutdown      atomic.Bool
	started       atomic.Bool
	activeCallsWG sync.WaitGroup
	callsMu       sync.Mutex
	wg            sync.WaitGroup

	// Backpressure control
	semaphore       chan struct{}
	workerAvailable chan struct{}
	shutdownCh      chan struct{}

	// Health monitoring
	healthMu     sync.RWMutex
	healthStatus HealthStatus
	healthCancel context.CancelFunc

	// Request tracking for cancellation
	activeRequests   map[uint64]*activeRequest
	activeRequestsMu sync.RWMutex

	// Observability
	tracer trace.Tracer
}

// activeRequest tracks an in-flight request for cancellation support
type activeRequest struct {
	id         uint64
	workerIdx  int
	conn       net.Conn
	cancelOnce sync.Once
	done       chan struct{}
}

// poolWorker wraps a Worker with connection pooling
type poolWorker struct {
	worker       workerHandle
	connPool     chan net.Conn
	inflightGate chan struct{}
	requestID    atomic.Uint64
	healthy      atomic.Bool
}

// HealthStatus represents the health of the pool
type HealthStatus struct {
	TotalWorkers   int
	HealthyWorkers int
	LastCheck      time.Time
}

// NewPool creates a new worker pool. When opts.ExternalMode is true the pool
// connects to pre-existing worker processes at the paths given in
// opts.ExternalSocketPaths instead of spawning child processes.
func NewPool(opts PoolOptions, logger *Logger) (*Pool, error) {
	if opts.ExternalMode {
		return newExternalPool(opts, logger)
	}

	if opts.Config.Workers <= 0 {
		return nil, errors.New("workers must be > 0")
	}
	if opts.Config.MaxInFlight <= 0 {
		opts.Config.MaxInFlight = 10
	}
	if opts.Config.MaxInFlightPerWorker <= 0 {
		opts.Config.MaxInFlightPerWorker = 1
	}
	if opts.Config.HealthInterval <= 0 {
		opts.Config.HealthInterval = 30 * time.Second
	}

	if logger == nil {
		logger = NewLogger(LoggingConfig{Level: "info", Format: "json"})
	}

	pool := &Pool{
		opts:            opts,
		logger:          logger,
		workers:         make([]*poolWorker, opts.Config.Workers),
		semaphore:       make(chan struct{}, opts.Config.MaxInFlight),
		workerAvailable: make(chan struct{}, opts.Config.Workers*opts.Config.MaxInFlightPerWorker),
		shutdownCh:      make(chan struct{}),
		activeRequests:  make(map[uint64]*activeRequest),
	}

	// Create workers
	for i := 0; i < opts.Config.Workers; i++ {
		workerCfg := opts.WorkerConfig
		workerCfg.ID = fmt.Sprintf("worker-%d", i)
		workerCfg.SocketPath = fmt.Sprintf("%s-%d", opts.WorkerConfig.SocketPath, i)
		if workerCfg.StartTimeout == 0 {
			workerCfg.StartTimeout = 5 * time.Second
		}

		worker := NewWorker(workerCfg, logger)
		pool.workers[i] = &poolWorker{
			worker:       worker,
			connPool:     make(chan net.Conn, opts.Config.MaxInFlightPerWorker),
			inflightGate: make(chan struct{}, opts.Config.MaxInFlightPerWorker),
		}
	}

	return pool, nil
}

// newExternalPool creates a pool that uses ExternalWorker instances.
func newExternalPool(opts PoolOptions, logger *Logger) (*Pool, error) {
	if len(opts.ExternalSocketPaths) == 0 {
		return nil, errors.New("ExternalSocketPaths must not be empty in ExternalMode")
	}

	numWorkers := len(opts.ExternalSocketPaths)
	opts.Config.Workers = numWorkers

	if opts.Config.MaxInFlight <= 0 {
		opts.Config.MaxInFlight = 10
	}
	if opts.Config.MaxInFlightPerWorker <= 0 {
		opts.Config.MaxInFlightPerWorker = 1
	}
	if opts.Config.HealthInterval <= 0 {
		opts.Config.HealthInterval = 30 * time.Second
	}

	if logger == nil {
		logger = NewLogger(LoggingConfig{Level: "info", Format: "json"})
	}

	pool := &Pool{
		opts:            opts,
		logger:          logger,
		workers:         make([]*poolWorker, numWorkers),
		semaphore:       make(chan struct{}, opts.Config.MaxInFlight),
		workerAvailable: make(chan struct{}, numWorkers*opts.Config.MaxInFlightPerWorker),
		shutdownCh:      make(chan struct{}),
		activeRequests:  make(map[uint64]*activeRequest),
	}

	for i, sockPath := range opts.ExternalSocketPaths {
		worker := NewExternalWorkerWithOptions(ExternalWorkerOptions{
			SocketPath:     sockPath,
			ConnectTimeout: opts.WorkerConfig.StartTimeout,
			MaxRetries:     opts.ExternalMaxRetries,
			RetryInterval:  opts.ExternalRetryInterval,
		})
		pool.workers[i] = &poolWorker{
			worker:       worker,
			connPool:     make(chan net.Conn, opts.Config.MaxInFlightPerWorker),
			inflightGate: make(chan struct{}, opts.Config.MaxInFlightPerWorker),
		}
	}

	return pool, nil
}

// WithTracer sets the OpenTelemetry tracer for the pool.
// This enables distributed tracing for all Pool.Call() operations.
// If not set, no tracing will be performed (zero overhead).
func (p *Pool) WithTracer(tracer trace.Tracer) *Pool {
	p.tracer = tracer
	return p
}

// Start starts all workers in the pool
func (p *Pool) Start(ctx context.Context) error {
	p.logger.Info("starting worker pool", "workers", p.opts.Config.Workers)

	// Start all workers
	for i, pw := range p.workers {
		if err := pw.worker.Start(ctx); err != nil {
			// Stop already started workers
			for j := 0; j < i; j++ {
				_ = p.workers[j].worker.Stop()
			}
			return fmt.Errorf("failed to start worker %d: %w", i, err)
		}
		// Don't mark as healthy until first health check succeeds
		// pw.healthy.Store(true) - removed, will be set by health check
	}

	// Pre-populate connection pools (synchronous to avoid race conditions)
	// We do minimal pre-population to reduce startup latency while avoiding complexity
	for _, pw := range p.workers {
		// Pre-populate just one connection per worker for faster first call
		conn, err := p.connect(pw.worker.GetSocketPath())
		if err != nil {
			p.logger.Debug("failed to pre-populate connection", "error", err)
			continue
		}

		select {
		case pw.connPool <- conn:
		default:
			// Pool is full (shouldn't happen with MaxInFlightPerWorker=1), close connection
			if err := conn.Close(); err != nil {
				p.logger.Error("failed to close connection", "error", err)
			}
		}
	}

	// Give workers time to stabilize before health check
	time.Sleep(100 * time.Millisecond)

	// Start health monitoring
	healthCtx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel
	p.wg.Add(1)
	go p.healthMonitor(healthCtx)

	// Initial health check
	p.updateHealthStatus()

	p.started.Store(true)
	p.logger.Info("worker pool started successfully")
	return nil
}

// Call invokes a method on one of the workers using round-robin
func (p *Pool) Call(ctx context.Context, method string, input interface{}, output interface{}) error {
	// Start tracing span if tracer is configured
	var span trace.Span
	if p.tracer != nil {
		ctx, span = p.tracer.Start(ctx, "pyproc.Pool.Call",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("pyproc.method", method),
			),
		)
		defer func() {
			// Span will be ended here, status is set in error paths
			span.End()
		}()
	}

	p.callsMu.Lock()
	if p.shutdown.Load() {
		p.callsMu.Unlock()
		err := errors.New("pool is shut down")
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
	p.activeCallsWG.Add(1)
	p.callsMu.Unlock()
	defer p.activeCallsWG.Done()

	// Acquire semaphore for backpressure
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		err := ctx.Err()
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	case <-p.shutdownCh:
		err := errors.New("pool is shut down")
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}

	pw, workerIdx, err := p.acquireWorker(ctx)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
	defer func() {
		<-pw.inflightGate
		p.signalWorkerAvailable()
	}()

	// Add worker ID to span attributes
	if span != nil {
		span.SetAttributes(attribute.Int("pyproc.worker_id", workerIdx))
	}

	// Get connection from pool
	var conn net.Conn
	select {
	case pooledConn, ok := <-pw.connPool:
		if !ok {
			err := errors.New("connection pool closed")
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return err
		}
		conn = pooledConn
	default:
		// Create new connection if pool is empty
		var err error
		conn, err = p.connect(pw.worker.GetSocketPath())
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to connect")
			}
			return fmt.Errorf("failed to connect: %w", err)
		}
	}

	// Generate request ID
	reqID := pw.requestID.Add(1)

	// Track active request for cancellation
	activeReq := &activeRequest{
		id:        reqID,
		workerIdx: workerIdx,
		conn:      conn,
		done:      make(chan struct{}),
	}
	p.activeRequestsMu.Lock()
	p.activeRequests[reqID] = activeReq
	p.activeRequestsMu.Unlock()

	// Monitor context for cancellation
	go p.monitorCancellation(ctx, activeReq)

	// Flag to track if connection was closed due to cancellation
	connClosed := false

	// Clean up active request and return connection on exit
	defer func() {
		close(activeReq.done)
		p.activeRequestsMu.Lock()
		delete(p.activeRequests, reqID)
		p.activeRequestsMu.Unlock()

		// Return connection to pool only if not closed
		if !connClosed && !p.shutdown.Load() {
			select {
			case pw.connPool <- conn:
			default:
				_ = conn.Close()
			}
		} else if !connClosed {
			_ = conn.Close()
		}
	}()

	// Send request
	req, err := protocol.NewRequest(reqID, method, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create request")
		}
		return err
	}

	// Inject trace context into request headers if tracing is enabled
	if span != nil {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		telemetry.InjectTraceContext(ctx, req.Headers)
	}

	// For now, send in legacy format for backward compatibility
	// TODO: Switch to wrapped format once Python side is fully tested
	framer := framing.NewFramer(conn)
	reqData, err := req.Marshal()
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal request")
		}
		return err
	}

	if err := framer.WriteMessage(reqData); err != nil {
		connClosed = true
		_ = conn.Close() // Connection is bad, don't return to pool
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to send request")
		}
		return err
	}

	// Read response
	respData, err := framer.ReadMessage()
	if err != nil {
		// Check if error is due to cancellation
		select {
		case <-ctx.Done():
			connClosed = true
			err := ctx.Err()
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return err
		default:
			connClosed = true
			_ = conn.Close() // Connection is bad, don't return to pool
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to read response")
			}
			return err
		}
	}

	// For now, handle legacy format
	// TODO: Switch to wrapped format once Python side is fully tested
	var resp protocol.Response
	if err := resp.Unmarshal(respData); err != nil {
		err := fmt.Errorf("failed to unmarshal response: %w", err)
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "unmarshal failed")
		}
		return err
	}

	if !resp.OK {
		err := resp.Error()
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "python worker error")
		}
		return err
	}

	// Handle special methods for testing
	if method == "echo_worker_id" {
		// Add worker ID to response
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Body, &result); err == nil {
			result["worker_id"] = float64(workerIdx)
			modifiedBody, _ := json.Marshal(result)
			resp.Body = modifiedBody
		}
	}

	err = resp.UnmarshalBody(output)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal output")
		}
		return err
	}

	// Success: set span status to OK
	if span != nil {
		span.SetStatus(codes.Ok, "")
	}
	return nil
}

// Shutdown gracefully shuts down all workers
func (p *Pool) Shutdown(_ context.Context) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // Already shutting down
	}
	close(p.shutdownCh)

	p.logger.Info("shutting down worker pool")

	// Cancel health monitoring
	if p.healthCancel != nil {
		p.healthCancel()
	}

	// Wait for in-flight calls to complete before closing pools.
	// The lock ensures all in-progress Call() goroutines have finished
	// their activeCallsWG.Add(1) before we start waiting.
	p.callsMu.Lock()
	p.activeCallsWG.Wait()
	p.callsMu.Unlock()

	// Close all connection pools
	for _, pw := range p.workers {
		close(pw.connPool)
		for conn := range pw.connPool {
			_ = conn.Close()
		}
	}

	// Stop all workers
	var errs []error
	for i, pw := range p.workers {
		if err := pw.worker.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("worker %d: %w", i, err))
		}
	}

	// Wait for goroutines
	p.wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	p.logger.Info("worker pool shut down successfully")
	return nil
}

func (p *Pool) signalWorkerAvailable() {
	select {
	case p.workerAvailable <- struct{}{}:
	default:
	}
}

func (p *Pool) acquireWorker(ctx context.Context) (*poolWorker, int, error) {
	workers := p.workers
	if len(workers) == 0 {
		return nil, -1, errors.New("no workers available")
	}

	for {
		startIdx := int(p.nextIdx.Add(1) - 1)
		healthyFound := false

		for i := 0; i < len(workers); i++ {
			idx := (startIdx + i) % len(workers)
			pw := workers[idx]
			if !pw.healthy.Load() {
				continue
			}
			healthyFound = true
			select {
			case pw.inflightGate <- struct{}{}:
				return pw, idx, nil
			default:
			}
		}

		if !healthyFound {
			return nil, -1, errors.New("no healthy workers available")
		}

		select {
		case <-ctx.Done():
			return nil, -1, ctx.Err()
		case <-p.shutdownCh:
			return nil, -1, errors.New("pool is shut down")
		case <-p.workerAvailable:
		}
	}
}

// Health returns the current health status of the pool
func (p *Pool) Health() HealthStatus {
	p.healthMu.RLock()
	defer p.healthMu.RUnlock()
	return p.healthStatus
}

// IsHealthy checks if a worker is healthy
func (w *Worker) IsHealthy(_ context.Context) bool {
	// Check if process is running
	if w.state.Load() != int32(WorkerStateRunning) {
		return false
	}

	// For now, just check if the process is running
	// TODO: Implement actual health RPC call
	return w.IsRunning()
}

// connect establishes a connection to a worker
func (p *Pool) connect(socketPath string) (net.Conn, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", socketPath, err)
	}
	return conn, nil
}

// healthMonitor periodically checks worker health
func (p *Pool) healthMonitor(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.opts.Config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.updateHealthStatus()
		}
	}
}

// updateHealthStatus updates the health status of all workers
func (p *Pool) updateHealthStatus() {
	healthy := 0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, pw := range p.workers {
		if pw.worker.IsHealthy(ctx) {
			pw.healthy.Store(true)
			healthy++
		} else {
			pw.healthy.Store(false)
		}
	}

	p.healthMu.Lock()
	p.healthStatus = HealthStatus{
		TotalWorkers:   len(p.workers),
		HealthyWorkers: healthy,
		LastCheck:      time.Now(),
	}
	p.healthMu.Unlock()

	if healthy < len(p.workers) {
		p.logger.Warn("some workers are unhealthy",
			"healthy", healthy, "total", len(p.workers))
	}
}

// monitorCancellation monitors the context and sends cancellation if needed
func (p *Pool) monitorCancellation(ctx context.Context, req *activeRequest) {
	select {
	case <-ctx.Done():
		// Context was cancelled, send cancellation message and close connection
		req.cancelOnce.Do(func() {
			p.logger.Debug("context cancelled, sending cancellation", "request_id", req.id)
			// Try to send cancellation message first
			p.sendCancellation(req)
			// Then close the connection to ensure cancellation
			if req.conn != nil {
				_ = req.conn.Close()
			}
		})
	case <-req.done:
		// Request completed normally
		return
	}
}

// sendCancellation sends a cancellation message to the Python worker
func (p *Pool) sendCancellation(req *activeRequest) {
	p.logger.Debug("sending cancellation", "request_id", req.id)

	// Create cancellation request
	cancelReq := protocol.NewCancellationRequest(req.id, "context cancelled")

	// Wrap in message envelope
	msgEnvelope, err := protocol.WrapMessage(protocol.MessageTypeCancellation, cancelReq)
	if err != nil {
		p.logger.Error("failed to create cancellation message", "error", err)
		return
	}

	// Marshal to JSON
	data, err := json.Marshal(msgEnvelope)
	if err != nil {
		p.logger.Error("failed to marshal cancellation message", "error", err)
		return
	}

	// Send cancellation message using the same connection
	// This is best-effort, if it fails we still close the connection
	framer := framing.NewFramer(req.conn)
	if err := framer.WriteMessage(data); err != nil {
		p.logger.Debug("failed to send cancellation message, will rely on connection close", "error", err)
	}
}
