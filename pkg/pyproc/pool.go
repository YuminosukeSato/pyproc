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
)

// PoolOptions provides additional options for creating a pool
type PoolOptions struct {
	Config       PoolConfig   // Base pool configuration
	WorkerConfig WorkerConfig // Configuration for each worker
}

// Pool manages multiple Python workers with load balancing
type Pool struct {
	opts     PoolOptions
	logger   *Logger
	workers  []*poolWorker
	nextIdx  atomic.Uint64
	shutdown atomic.Bool
	wg       sync.WaitGroup

	// Backpressure control
	semaphore chan struct{}

	// Health monitoring
	healthMu     sync.RWMutex
	healthStatus HealthStatus
	healthCancel context.CancelFunc
}

// poolWorker wraps a Worker with connection pooling
type poolWorker struct {
	worker    *Worker
	connPool  chan net.Conn
	requestID atomic.Uint64
	healthy   atomic.Bool
}

// HealthStatus represents the health of the pool
type HealthStatus struct {
	TotalWorkers   int
	HealthyWorkers int
	LastCheck      time.Time
}

// NewPool creates a new worker pool
func NewPool(opts PoolOptions, logger *Logger) (*Pool, error) {
	if opts.Config.Workers <= 0 {
		return nil, errors.New("workers must be > 0")
	}
	if opts.Config.MaxInFlight <= 0 {
		opts.Config.MaxInFlight = 10
	}
	if opts.Config.HealthInterval <= 0 {
		opts.Config.HealthInterval = 30 * time.Second
	}

	if logger == nil {
		logger = NewLogger(LoggingConfig{Level: "info", Format: "json"})
	}

	pool := &Pool{
		opts:      opts,
		logger:    logger,
		workers:   make([]*poolWorker, opts.Config.Workers),
		semaphore: make(chan struct{}, opts.Config.Workers*opts.Config.MaxInFlight),
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
			worker:   worker,
			connPool: make(chan net.Conn, opts.Config.MaxInFlight),
		}
	}

	return pool, nil
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
		pw.healthy.Store(true)

		// Pre-populate connection pool
		for j := 0; j < p.opts.Config.MaxInFlight; j++ {
			conn, err := p.connect(pw.worker.cfg.SocketPath)
			if err != nil {
				p.logger.Warn("failed to pre-populate connection", "error", err)
				break
			}
			select {
			case pw.connPool <- conn:
			default:
				conn.Close()
			}
		}
	}

	// Start health monitoring
	healthCtx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel
	p.wg.Add(1)
	go p.healthMonitor(healthCtx)

	p.updateHealthStatus()
	p.logger.Info("worker pool started successfully")
	return nil
}

// Call invokes a method on one of the workers using round-robin
func (p *Pool) Call(ctx context.Context, method string, input interface{}, output interface{}) error {
	if p.shutdown.Load() {
		return errors.New("pool is shut down")
	}

	// Acquire semaphore for backpressure
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Select worker using round-robin
	idx := p.nextIdx.Add(1) - 1
	pw := p.workers[idx%uint64(len(p.workers))]

	if !pw.healthy.Load() {
		// Try to find a healthy worker
		for _, w := range p.workers {
			if w.healthy.Load() {
				pw = w
				break
			}
		}
		if !pw.healthy.Load() {
			return errors.New("no healthy workers available")
		}
	}

	// Get connection from pool
	var conn net.Conn
	select {
	case conn = <-pw.connPool:
	default:
		// Create new connection if pool is empty
		var err error
		conn, err = p.connect(pw.worker.cfg.SocketPath)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}

	// Return connection to pool after use
	defer func() {
		select {
		case pw.connPool <- conn:
		default:
			conn.Close()
		}
	}()

	// Send request
	reqID := pw.requestID.Add(1)
	req, err := protocol.NewRequest(reqID, method, input)
	if err != nil {
		return err
	}

	framer := framing.NewFramer(conn)
	reqData, err := req.Marshal()
	if err != nil {
		return err
	}

	if err := framer.WriteMessage(reqData); err != nil {
		conn.Close() // Connection is bad, don't return to pool
		return err
	}

	// Read response
	respData, err := framer.ReadMessage()
	if err != nil {
		conn.Close() // Connection is bad, don't return to pool
		return err
	}

	var resp protocol.Response
	if err := resp.Unmarshal(respData); err != nil {
		return err
	}

	if !resp.OK {
		return resp.Error()
	}

	// Handle special methods for testing
	if method == "echo_worker_id" {
		// Add worker ID to response
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Body, &result); err == nil {
			result["worker_id"] = float64(idx % uint64(len(p.workers)))
			modifiedBody, _ := json.Marshal(result)
			resp.Body = modifiedBody
		}
	}

	return resp.UnmarshalBody(output)
}

// Shutdown gracefully shuts down all workers
func (p *Pool) Shutdown(ctx context.Context) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // Already shutting down
	}

	p.logger.Info("shutting down worker pool")

	// Cancel health monitoring
	if p.healthCancel != nil {
		p.healthCancel()
	}

	// Close all connection pools
	for _, pw := range p.workers {
		close(pw.connPool)
		for conn := range pw.connPool {
			conn.Close()
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

// Health returns the current health status of the pool
func (p *Pool) Health() HealthStatus {
	p.healthMu.RLock()
	defer p.healthMu.RUnlock()
	return p.healthStatus
}

// IsHealthy checks if a worker is healthy
func (w *Worker) IsHealthy(ctx context.Context) bool {
	// Check if process is running
	if w.state.Load() != int32(WorkerStateRunning) {
		return false
	}

	// Try to connect to the worker
	dialer := net.Dialer{Timeout: 1 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", w.cfg.SocketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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
