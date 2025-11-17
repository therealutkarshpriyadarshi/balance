package security

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// Allow checks if a request should be allowed
	Allow(key string) bool

	// Reset resets the rate limiter for a specific key
	Reset(key string)

	// Stats returns rate limiter statistics
	Stats() map[string]interface{}
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu sync.RWMutex

	// rate is the number of tokens added per second
	rate float64

	// capacity is the maximum number of tokens
	capacity int64

	// buckets maps keys to their token buckets
	buckets map[string]*bucket

	// cleanupInterval is how often to clean up old buckets
	cleanupInterval time.Duration

	// bucketTTL is how long to keep inactive buckets
	bucketTTL time.Duration

	// Statistics
	totalRequests  atomic.Int64
	allowedCount   atomic.Int64
	blockedCount   atomic.Int64
}

// bucket represents a token bucket for a single key
type bucket struct {
	tokens       float64
	lastRefill   time.Time
	mu           sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter
// rate: tokens per second
// capacity: maximum tokens
func NewTokenBucket(rate float64, capacity int64) *TokenBucket {
	tb := &TokenBucket{
		rate:            rate,
		capacity:        capacity,
		buckets:         make(map[string]*bucket),
		cleanupInterval: 1 * time.Minute,
		bucketTTL:       5 * time.Minute,
	}

	// Start cleanup goroutine
	go tb.cleanup()

	return tb
}

// Allow checks if a request should be allowed for the given key
func (tb *TokenBucket) Allow(key string) bool {
	tb.totalRequests.Add(1)

	tb.mu.Lock()
	b, exists := tb.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(tb.capacity),
			lastRefill: time.Now(),
		}
		tb.buckets[key] = b
	}
	tb.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > float64(tb.capacity) {
		b.tokens = float64(tb.capacity)
	}
	b.lastRefill = now

	// Check if we have at least 1 token
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		tb.allowedCount.Add(1)
		return true
	}

	tb.blockedCount.Add(1)
	return false
}

// Reset resets the rate limiter for a specific key
func (tb *TokenBucket) Reset(key string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	delete(tb.buckets, key)
}

// cleanup periodically removes old buckets
func (tb *TokenBucket) cleanup() {
	ticker := time.NewTicker(tb.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		tb.mu.Lock()
		now := time.Now()
		for key, b := range tb.buckets {
			b.mu.Lock()
			if now.Sub(b.lastRefill) > tb.bucketTTL {
				delete(tb.buckets, key)
			}
			b.mu.Unlock()
		}
		tb.mu.Unlock()
	}
}

// Stats returns rate limiter statistics
func (tb *TokenBucket) Stats() map[string]interface{} {
	tb.mu.RLock()
	activeBuckets := len(tb.buckets)
	tb.mu.RUnlock()

	return map[string]interface{}{
		"total_requests":  tb.totalRequests.Load(),
		"allowed":         tb.allowedCount.Load(),
		"blocked":         tb.blockedCount.Load(),
		"active_buckets":  activeBuckets,
		"rate":            tb.rate,
		"capacity":        tb.capacity,
	}
}

// SlidingWindow implements a sliding window rate limiter
type SlidingWindow struct {
	mu sync.RWMutex

	// limit is the maximum number of requests in the window
	limit int64

	// window is the time window duration
	window time.Duration

	// windows maps keys to their request windows
	windows map[string]*requestWindow

	// cleanupInterval is how often to clean up old windows
	cleanupInterval time.Duration

	// Statistics
	totalRequests atomic.Int64
	allowedCount  atomic.Int64
	blockedCount  atomic.Int64
}

// requestWindow tracks requests in a sliding time window
type requestWindow struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindow creates a new sliding window rate limiter
func NewSlidingWindow(limit int64, window time.Duration) *SlidingWindow {
	sw := &SlidingWindow{
		limit:           limit,
		window:          window,
		windows:         make(map[string]*requestWindow),
		cleanupInterval: 1 * time.Minute,
	}

	// Start cleanup goroutine
	go sw.cleanup()

	return sw
}

// Allow checks if a request should be allowed for the given key
func (sw *SlidingWindow) Allow(key string) bool {
	sw.totalRequests.Add(1)

	sw.mu.Lock()
	w, exists := sw.windows[key]
	if !exists {
		w = &requestWindow{
			requests: make([]time.Time, 0),
		}
		sw.windows[key] = w
	}
	sw.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// Remove old requests outside the window
	validRequests := make([]time.Time, 0)
	for _, t := range w.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	w.requests = validRequests

	// Check if we're under the limit
	if int64(len(w.requests)) < sw.limit {
		w.requests = append(w.requests, now)
		sw.allowedCount.Add(1)
		return true
	}

	sw.blockedCount.Add(1)
	return false
}

// Reset resets the rate limiter for a specific key
func (sw *SlidingWindow) Reset(key string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	delete(sw.windows, key)
}

// cleanup periodically removes old windows
func (sw *SlidingWindow) cleanup() {
	ticker := time.NewTicker(sw.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sw.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-sw.window * 2) // Keep for 2x window duration

		for key, w := range sw.windows {
			w.mu.Lock()
			if len(w.requests) == 0 || (len(w.requests) > 0 && w.requests[len(w.requests)-1].Before(cutoff)) {
				delete(sw.windows, key)
			}
			w.mu.Unlock()
		}
		sw.mu.Unlock()
	}
}

// Stats returns rate limiter statistics
func (sw *SlidingWindow) Stats() map[string]interface{} {
	sw.mu.RLock()
	activeWindows := len(sw.windows)
	sw.mu.RUnlock()

	return map[string]interface{}{
		"total_requests":  sw.totalRequests.Load(),
		"allowed":         sw.allowedCount.Load(),
		"blocked":         sw.blockedCount.Load(),
		"active_windows":  activeWindows,
		"limit":           sw.limit,
		"window_duration": sw.window.String(),
	}
}

// PerIPRateLimiter wraps a rate limiter to work with IP addresses
type PerIPRateLimiter struct {
	limiter RateLimiter
}

// NewPerIPRateLimiter creates a new per-IP rate limiter
func NewPerIPRateLimiter(limiter RateLimiter) *PerIPRateLimiter {
	return &PerIPRateLimiter{
		limiter: limiter,
	}
}

// AllowIP checks if a request from the given IP should be allowed
func (l *PerIPRateLimiter) AllowIP(ip string) bool {
	return l.limiter.Allow(ip)
}

// ResetIP resets the rate limiter for a specific IP
func (l *PerIPRateLimiter) ResetIP(ip string) {
	l.limiter.Reset(ip)
}

// Stats returns rate limiter statistics
func (l *PerIPRateLimiter) Stats() map[string]interface{} {
	return l.limiter.Stats()
}

// CombinedRateLimiter combines multiple rate limiters with AND logic
// All limiters must allow the request for it to be allowed
type CombinedRateLimiter struct {
	limiters []RateLimiter
}

// NewCombinedRateLimiter creates a new combined rate limiter
func NewCombinedRateLimiter(limiters ...RateLimiter) (*CombinedRateLimiter, error) {
	if len(limiters) == 0 {
		return nil, fmt.Errorf("at least one rate limiter is required")
	}

	return &CombinedRateLimiter{
		limiters: limiters,
	}, nil
}

// Allow checks if a request should be allowed by all limiters
func (c *CombinedRateLimiter) Allow(key string) bool {
	for _, limiter := range c.limiters {
		if !limiter.Allow(key) {
			return false
		}
	}
	return true
}

// Reset resets all rate limiters for a specific key
func (c *CombinedRateLimiter) Reset(key string) {
	for _, limiter := range c.limiters {
		limiter.Reset(key)
	}
}

// Stats returns combined statistics from all limiters
func (c *CombinedRateLimiter) Stats() map[string]interface{} {
	stats := make(map[string]interface{})
	for i, limiter := range c.limiters {
		key := fmt.Sprintf("limiter_%d", i)
		stats[key] = limiter.Stats()
	}
	return stats
}
