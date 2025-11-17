package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrGoroutinePoolClosed is returned when trying to submit work to a closed pool
	ErrGoroutinePoolClosed = errors.New("goroutine pool is closed")

	// ErrGoroutinePoolTimeout is returned when work submission times out
	ErrGoroutinePoolTimeout = errors.New("goroutine pool submission timeout")
)

// Task represents a unit of work to be executed by the pool
type Task func()

// GoroutinePool manages a pool of goroutines for executing tasks
type GoroutinePool struct {
	// Configuration
	maxWorkers   int32
	maxIdleTime  time.Duration
	queueSize    int
	nonBlocking  bool

	// Runtime state
	workers      int32
	running      int32
	closed       int32

	// Channels
	taskQueue    chan Task
	stopChan     chan struct{}

	// Sync
	wg           sync.WaitGroup
	once         sync.Once

	// Metrics
	submitted    uint64
	completed    uint64
	rejected     uint64
}

// GoroutinePoolConfig contains configuration for the goroutine pool
type GoroutinePoolConfig struct {
	// MaxWorkers is the maximum number of goroutines in the pool
	MaxWorkers int

	// MaxIdleTime is how long idle workers wait before terminating
	MaxIdleTime time.Duration

	// QueueSize is the size of the task queue (0 for unbounded)
	QueueSize int

	// NonBlocking determines if Submit should return immediately if queue is full
	NonBlocking bool
}

// DefaultGoroutinePoolConfig returns default pool configuration
func DefaultGoroutinePoolConfig() GoroutinePoolConfig {
	return GoroutinePoolConfig{
		MaxWorkers:  100,
		MaxIdleTime: 10 * time.Second,
		QueueSize:   1000,
		NonBlocking: false,
	}
}

// NewGoroutinePool creates a new goroutine pool
func NewGoroutinePool(config GoroutinePoolConfig) *GoroutinePool {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 100
	}
	if config.MaxIdleTime <= 0 {
		config.MaxIdleTime = 10 * time.Second
	}

	queueSize := config.QueueSize
	if queueSize < 0 {
		queueSize = 0
	}

	pool := &GoroutinePool{
		maxWorkers:   int32(config.MaxWorkers),
		maxIdleTime:  config.MaxIdleTime,
		queueSize:    queueSize,
		nonBlocking:  config.NonBlocking,
		taskQueue:    make(chan Task, queueSize),
		stopChan:     make(chan struct{}),
	}

	return pool
}

// Submit submits a task to the pool
func (p *GoroutinePool) Submit(task Task) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}

	atomic.AddUint64(&p.submitted, 1)

	// Try to send task to queue
	if p.nonBlocking {
		select {
		case p.taskQueue <- task:
			p.ensureWorker()
			return nil
		default:
			atomic.AddUint64(&p.rejected, 1)
			return ErrGoroutinePoolTimeout
		}
	}

	// Blocking send
	select {
	case p.taskQueue <- task:
		p.ensureWorker()
		return nil
	case <-p.stopChan:
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}
}

// SubmitWithTimeout submits a task with a timeout
func (p *GoroutinePool) SubmitWithTimeout(task Task, timeout time.Duration) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}

	atomic.AddUint64(&p.submitted, 1)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case p.taskQueue <- task:
		p.ensureWorker()
		return nil
	case <-timer.C:
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolTimeout
	case <-p.stopChan:
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}
}

// SubmitWithContext submits a task with context
func (p *GoroutinePool) SubmitWithContext(ctx context.Context, task Task) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}

	atomic.AddUint64(&p.submitted, 1)

	select {
	case p.taskQueue <- task:
		p.ensureWorker()
		return nil
	case <-ctx.Done():
		atomic.AddUint64(&p.rejected, 1)
		return ctx.Err()
	case <-p.stopChan:
		atomic.AddUint64(&p.rejected, 1)
		return ErrGoroutinePoolClosed
	}
}

// ensureWorker ensures at least one worker is running
func (p *GoroutinePool) ensureWorker() {
	currentWorkers := atomic.LoadInt32(&p.workers)
	if currentWorkers < p.maxWorkers {
		if atomic.CompareAndSwapInt32(&p.workers, currentWorkers, currentWorkers+1) {
			p.wg.Add(1)
			go p.worker()
		}
	}
}

// worker is the main worker goroutine
func (p *GoroutinePool) worker() {
	defer func() {
		atomic.AddInt32(&p.workers, -1)
		p.wg.Done()
	}()

	timer := time.NewTimer(p.maxIdleTime)
	defer timer.Stop()

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}

			// Reset idle timer
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(p.maxIdleTime)

			// Execute task
			p.executeTask(task)

		case <-timer.C:
			// Worker has been idle too long, check if we should terminate
			currentWorkers := atomic.LoadInt32(&p.workers)
			running := atomic.LoadInt32(&p.running)

			// Keep at least one worker if there's work in queue
			if currentWorkers > 1 || (len(p.taskQueue) == 0 && running == 0) {
				return
			}

			timer.Reset(p.maxIdleTime)

		case <-p.stopChan:
			return
		}
	}
}

// executeTask executes a task with panic recovery
func (p *GoroutinePool) executeTask(task Task) {
	atomic.AddInt32(&p.running, 1)
	defer atomic.AddInt32(&p.running, -1)
	defer atomic.AddUint64(&p.completed, 1)

	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the worker
			// In production, you'd log this properly
			_ = r
		}
	}()

	task()
}

// Close shuts down the pool and waits for all workers to finish
func (p *GoroutinePool) Close() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.closed, 1)
		close(p.stopChan)
		close(p.taskQueue)
		p.wg.Wait()
	})
}

// CloseWithTimeout shuts down the pool with a timeout
func (p *GoroutinePool) CloseWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})

	go func() {
		p.Close()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("pool close timeout")
	}
}

// Stats returns pool statistics
func (p *GoroutinePool) Stats() GoroutinePoolStats {
	return GoroutinePoolStats{
		Workers:    atomic.LoadInt32(&p.workers),
		Running:    atomic.LoadInt32(&p.running),
		QueueSize:  len(p.taskQueue),
		Submitted:  atomic.LoadUint64(&p.submitted),
		Completed:  atomic.LoadUint64(&p.completed),
		Rejected:   atomic.LoadUint64(&p.rejected),
		IsClosed:   atomic.LoadInt32(&p.closed) == 1,
	}
}

// GoroutinePoolStats contains pool statistics
type GoroutinePoolStats struct {
	Workers    int32   // Current number of workers
	Running    int32   // Number of tasks currently running
	QueueSize  int     // Number of tasks in queue
	Submitted  uint64  // Total tasks submitted
	Completed  uint64  // Total tasks completed
	Rejected   uint64  // Total tasks rejected
	IsClosed   bool    // Whether pool is closed
}

// Global default pool
var defaultPool *GoroutinePool
var poolOnce sync.Once

// GetDefaultPool returns the global default goroutine pool
func GetDefaultPool() *GoroutinePool {
	poolOnce.Do(func() {
		config := DefaultGoroutinePoolConfig()
		config.MaxWorkers = 1000
		config.QueueSize = 10000
		defaultPool = NewGoroutinePool(config)
	})
	return defaultPool
}

// Submit submits a task to the default pool
func Submit(task Task) error {
	return GetDefaultPool().Submit(task)
}

// SubmitWithTimeout submits a task to the default pool with timeout
func SubmitWithTimeout(task Task, timeout time.Duration) error {
	return GetDefaultPool().SubmitWithTimeout(task, timeout)
}
