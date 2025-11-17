package health

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// Checker orchestrates health checking for a pool of backends
type Checker struct {
	// Backend pool to monitor
	pool *backend.Pool

	// Active health checker
	activeChecker *ActiveChecker

	// Passive health checker
	passiveChecker *PassiveChecker

	// State machines for each backend
	stateMachines map[string]*backend.StateMachine
	mu            sync.RWMutex

	// Configuration
	interval           time.Duration
	healthyThreshold   int
	unhealthyThreshold int

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	totalChecks   int64
	successChecks int64
	failedChecks  int64
}

// CheckerConfig configures the health checker
type CheckerConfig struct {
	// Interval between health checks
	Interval time.Duration

	// Timeout for each health check
	Timeout time.Duration

	// HealthyThreshold is the number of consecutive successes before marking healthy
	HealthyThreshold int

	// UnhealthyThreshold is the number of consecutive failures before marking unhealthy
	UnhealthyThreshold int

	// ActiveCheck configuration
	ActiveCheckType CheckType
	HTTPPath        string

	// PassiveCheck configuration
	EnablePassiveChecks  bool
	ErrorRateThreshold   float64
	ConsecutiveFailures  int
	PassiveCheckWindow   time.Duration
}

// NewChecker creates a new health checker
func NewChecker(pool *backend.Pool, config CheckerConfig) *Checker {
	// Set defaults
	if config.Interval == 0 {
		config.Interval = 10 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}
	if config.HealthyThreshold == 0 {
		config.HealthyThreshold = 2
	}
	if config.UnhealthyThreshold == 0 {
		config.UnhealthyThreshold = 3
	}
	if config.ActiveCheckType == "" {
		config.ActiveCheckType = CheckTypeTCP
	}

	ctx, cancel := context.WithCancel(context.Background())

	checker := &Checker{
		pool:               pool,
		interval:           config.Interval,
		healthyThreshold:   config.HealthyThreshold,
		unhealthyThreshold: config.UnhealthyThreshold,
		stateMachines:      make(map[string]*backend.StateMachine),
		ctx:                ctx,
		cancel:             cancel,
	}

	// Create active checker
	checker.activeChecker = NewActiveChecker(ActiveCheckerConfig{
		CheckType: config.ActiveCheckType,
		Timeout:   config.Timeout,
		HTTPPath:  config.HTTPPath,
	})

	// Create passive checker if enabled
	if config.EnablePassiveChecks {
		checker.passiveChecker = NewPassiveChecker(PassiveCheckerConfig{
			ErrorRateThreshold:  config.ErrorRateThreshold,
			ConsecutiveFailures: config.ConsecutiveFailures,
			Window:              config.PassiveCheckWindow,
		})
	}

	// Initialize state machines for all backends
	for _, b := range pool.All() {
		sm := backend.NewStateMachine(b, config.HealthyThreshold, config.UnhealthyThreshold)
		sm.AddListener(checker.onStateChange)
		checker.stateMachines[b.Name()] = sm
	}

	return checker
}

// Start begins health checking
func (c *Checker) Start() error {
	log.Printf("[Health] Starting health checker with interval %s", c.interval)

	c.wg.Add(1)
	go c.runHealthChecks()

	return nil
}

// Stop stops health checking
func (c *Checker) Stop() error {
	log.Println("[Health] Stopping health checker")
	c.cancel()
	c.wg.Wait()
	log.Println("[Health] Health checker stopped")
	return nil
}

// runHealthChecks runs periodic health checks
func (c *Checker) runHealthChecks() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run initial health check
	c.performHealthChecks()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all backends
func (c *Checker) performHealthChecks() {
	backends := c.pool.All()
	if len(backends) == 0 {
		return
	}

	// Create a context with timeout for all checks
	ctx, cancel := context.WithTimeout(c.ctx, c.interval)
	defer cancel()

	// Perform health checks concurrently
	results := c.activeChecker.CheckMultiple(ctx, backends)

	// Process results
	for _, result := range results {
		c.processResult(result)
	}
}

// processResult processes a health check result
func (c *Checker) processResult(result CheckResult) {
	c.mu.RLock()
	sm, exists := c.stateMachines[result.Backend.Name()]
	c.mu.RUnlock()

	if !exists {
		// Backend was added after checker started, create state machine
		c.mu.Lock()
		sm = backend.NewStateMachine(result.Backend, c.healthyThreshold, c.unhealthyThreshold)
		sm.AddListener(c.onStateChange)
		c.stateMachines[result.Backend.Name()] = sm
		c.mu.Unlock()
	}

	if result.Success {
		sm.RecordSuccess()
		c.successChecks++
	} else {
		sm.RecordFailure()
		c.failedChecks++
		log.Printf("[Health] Backend %s health check failed: %v (duration: %s)",
			result.Backend.Name(), result.Error, result.Duration)
	}

	c.totalChecks++
}

// RecordRequest records a request result for passive health checking
func (c *Checker) RecordRequest(b *backend.Backend, success bool, responseTime time.Duration) {
	if c.passiveChecker == nil {
		return
	}

	c.mu.RLock()
	sm, exists := c.stateMachines[b.Name()]
	c.mu.RUnlock()

	if !exists {
		return
	}

	// Record in state machine metrics
	sm.RecordRequest(success, responseTime)

	// Record in passive checker
	if success {
		c.passiveChecker.RecordSuccess(b, responseTime)
	} else {
		shouldMarkUnhealthy := c.passiveChecker.RecordFailure(b)
		if shouldMarkUnhealthy {
			// Passive check indicates backend is unhealthy
			sm.RecordFailure()
			log.Printf("[Health] Passive check marked backend %s as potentially unhealthy", b.Name())
		}
	}
}

// onStateChange is called when a backend's state changes
func (c *Checker) onStateChange(b *backend.Backend, oldState, newState backend.State) {
	log.Printf("[Health] Backend %s state changed: %s -> %s", b.Name(), oldState, newState)

	// Reset passive check failures when transitioning to healthy
	if newState == backend.StateHealthy && c.passiveChecker != nil {
		c.passiveChecker.Reset(b)
	}
}

// GetStateMachine returns the state machine for a backend
func (c *Checker) GetStateMachine(backendName string) (*backend.StateMachine, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sm, exists := c.stateMachines[backendName]
	if !exists {
		return nil, fmt.Errorf("state machine not found for backend: %s", backendName)
	}

	return sm, nil
}

// GetAllStateMachines returns all state machines
func (c *Checker) GetAllStateMachines() map[string]*backend.StateMachine {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*backend.StateMachine, len(c.stateMachines))
	for k, v := range c.stateMachines {
		result[k] = v
	}

	return result
}

// GetStats returns health check statistics
func (c *Checker) GetStats() (total, success, failed int64) {
	return c.totalChecks, c.successChecks, c.failedChecks
}

// AddBackend adds a new backend to health checking
func (c *Checker) AddBackend(b *backend.Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.stateMachines[b.Name()]; exists {
		return
	}

	sm := backend.NewStateMachine(b, c.healthyThreshold, c.unhealthyThreshold)
	sm.AddListener(c.onStateChange)
	c.stateMachines[b.Name()] = sm

	log.Printf("[Health] Added backend %s to health checking", b.Name())
}

// RemoveBackend removes a backend from health checking
func (c *Checker) RemoveBackend(backendName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.stateMachines, backendName)
	log.Printf("[Health] Removed backend %s from health checking", backendName)
}
