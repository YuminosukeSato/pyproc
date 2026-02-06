package pyproc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

// TransportPool manages a pool of transports for load balancing
type TransportPool struct {
	transports []Transport
	nextIdx    atomic.Uint64
	logger     *Logger
	mu         sync.RWMutex
}

// NewTransportPool creates a new transport pool
func NewTransportPool(configs []TransportConfig, logger *Logger) (*TransportPool, error) {
	if len(configs) == 0 {
		return nil, errors.New("at least one transport config is required")
	}

	pool := &TransportPool{
		transports: make([]Transport, 0, len(configs)),
		logger:     logger,
	}

	for i, config := range configs {
		transport, err := NewTransport(config, logger)
		if err != nil {
			// Clean up already created transports
			for _, t := range pool.transports {
				_ = t.Close()
			}
			return nil, fmt.Errorf("failed to create transport %d: %w", i, err)
		}
		pool.transports = append(pool.transports, transport)
	}

	return pool, nil
}

// Call selects a transport and makes a call
func (p *TransportPool) Call(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.transports) == 0 {
		return nil, errors.New("no transports available")
	}

	// Try round-robin with fallback
	startIdx := p.nextIdx.Add(1) - 1
	for i := 0; i < len(p.transports); i++ {
		idx := (startIdx + uint64(i)) % uint64(len(p.transports))
		transport := p.transports[idx]

		if transport.IsHealthy() {
			resp, err := transport.Call(ctx, req)
			if err == nil {
				return resp, nil
			}
			p.logger.Warn("transport call failed, trying next",
				"index", idx,
				"error", err)
		}
	}

	return nil, errors.New("all transports failed")
}

// Close closes all transports in the pool
func (p *TransportPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for i, transport := range p.transports {
		if err := transport.Close(); err != nil {
			errs = append(errs, fmt.Errorf("transport %d: %w", i, err))
		}
	}

	p.transports = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to close transports: %v", errs)
	}
	return nil
}

// Health returns the health status of the pool
func (p *TransportPool) Health() (healthy, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total = len(p.transports)
	for _, transport := range p.transports {
		if transport.IsHealthy() {
			healthy++
		}
	}
	return
}

// PoolWithTransport updates the Pool to use Transport interface
type PoolWithTransport struct {
	opts          PoolOptions
	logger        *Logger
	transportPool *TransportPool
	workers       []*Worker // Still manage worker processes
	shutdown      atomic.Bool
	wg            sync.WaitGroup

	// Backpressure control
	semaphore chan struct{}

	// Health monitoring
	healthMu     sync.RWMutex
	healthStatus HealthStatus
	healthCancel context.CancelFunc
}

// NewPoolWithTransport creates a new pool using the Transport interface
func NewPoolWithTransport(opts PoolOptions, logger *Logger) (*PoolWithTransport, error) {
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

	pool := &PoolWithTransport{
		opts:      opts,
		logger:    logger,
		workers:   make([]*Worker, opts.Config.Workers),
		semaphore: make(chan struct{}, opts.Config.MaxInFlight),
	}

	// Create workers (they still manage the Python processes)
	for i := 0; i < opts.Config.Workers; i++ {
		workerCfg := opts.WorkerConfig
		workerCfg.ID = fmt.Sprintf("worker-%d", i)
		workerCfg.SocketPath = fmt.Sprintf("%s-%d", opts.WorkerConfig.SocketPath, i)
		if workerCfg.StartTimeout == 0 {
			workerCfg.StartTimeout = 5 * time.Second
		}
		// Security configuration will be handled in WorkerConfig directly

		worker := NewWorker(workerCfg, logger)
		pool.workers[i] = worker
	}

	return pool, nil
}

// Start starts all workers and creates transports
func (p *PoolWithTransport) Start(ctx context.Context) error {
	p.logger.Info("starting worker pool with transports", "workers", p.opts.Config.Workers)

	// Start all workers
	for i, worker := range p.workers {
		if err := worker.Start(ctx); err != nil {
			// Stop already started workers
			for j := 0; j < i; j++ {
				_ = p.workers[j].Stop()
			}
			return fmt.Errorf("failed to start worker %d: %w", i, err)
		}
	}

	// Give workers time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Create transport configurations for each worker
	configs := make([]TransportConfig, len(p.workers))
	for i, worker := range p.workers {
		configs[i] = TransportConfig{
			Type:    "uds",
			Address: worker.GetSocketPath(),
			Options: map[string]interface{}{
				"timeout":      5 * time.Second,
				"idle_timeout": 30 * time.Second,
			},
		}
	}

	// Create transport pool
	transportPool, err := NewTransportPool(configs, p.logger)
	if err != nil {
		// Stop all workers if transport creation fails
		for _, worker := range p.workers {
			_ = worker.Stop()
		}
		return fmt.Errorf("failed to create transport pool: %w", err)
	}
	p.transportPool = transportPool

	// Start health monitoring
	healthCtx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel
	p.wg.Add(1)
	go p.healthMonitor(healthCtx)

	// Initial health check
	p.updateHealthStatus()
	p.logger.Info("worker pool with transports started successfully")
	return nil
}

// Call invokes a method using the transport pool
func (p *PoolWithTransport) Call(ctx context.Context, method string, input interface{}, output interface{}) error {
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

	// Create request
	req, err := protocol.NewRequest(0, method, input)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Call through transport pool
	resp, err := p.transportPool.Call(ctx, req)
	if err != nil {
		return fmt.Errorf("transport call failed: %w", err)
	}

	if !resp.OK {
		return resp.Error()
	}

	return resp.UnmarshalBody(output)
}

// Shutdown gracefully shuts down the pool
func (p *PoolWithTransport) Shutdown(_ context.Context) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // Already shutting down
	}

	p.logger.Info("shutting down worker pool with transports")

	// Cancel health monitoring
	if p.healthCancel != nil {
		p.healthCancel()
	}

	// Close transport pool
	if p.transportPool != nil {
		if err := p.transportPool.Close(); err != nil {
			p.logger.Error("failed to close transport pool", "error", err)
		}
	}

	// Stop all workers
	var errs []error
	for i, worker := range p.workers {
		if err := worker.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("worker %d: %w", i, err))
		}
	}

	// Wait for goroutines
	p.wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	p.logger.Info("worker pool with transports shut down successfully")
	return nil
}

// healthMonitor periodically checks worker health
func (p *PoolWithTransport) healthMonitor(ctx context.Context) {
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

// updateHealthStatus updates the health status
func (p *PoolWithTransport) updateHealthStatus() {
	healthy, total := p.transportPool.Health()

	p.healthMu.Lock()
	p.healthStatus = HealthStatus{
		TotalWorkers:   total,
		HealthyWorkers: healthy,
		LastCheck:      time.Now(),
	}
	p.healthMu.Unlock()

	if healthy < total {
		p.logger.Warn("some transports are unhealthy",
			"healthy", healthy, "total", total)
	}
}

// Health returns the current health status
func (p *PoolWithTransport) Health() HealthStatus {
	p.healthMu.RLock()
	defer p.healthMu.RUnlock()
	return p.healthStatus
}
