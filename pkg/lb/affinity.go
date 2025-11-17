package lb

import (
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// SessionAffinity implements session affinity (sticky sessions) wrapper
// It wraps any LoadBalancer and adds session persistence based on client IP
type SessionAffinity struct {
	balancer   LoadBalancer
	sessions   map[string]*session
	mu         sync.RWMutex
	timeout    time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// session represents a sticky session binding
type session struct {
	backend    *backend.Backend
	lastAccess time.Time
}

// NewSessionAffinity creates a new session affinity wrapper
// timeout specifies how long a session should persist after last use
func NewSessionAffinity(balancer LoadBalancer, timeout time.Duration) *SessionAffinity {
	if timeout <= 0 {
		timeout = 10 * time.Minute // Default 10 minutes
	}

	sa := &SessionAffinity{
		balancer:    balancer,
		sessions:    make(map[string]*session),
		timeout:     timeout,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine to remove expired sessions
	sa.cleanupTicker = time.NewTicker(1 * time.Minute)
	go sa.cleanupLoop()

	return sa
}

// SelectWithClientIP selects a backend using session affinity based on client IP
// If the client has an existing session, it returns the same backend
// Otherwise, it uses the underlying load balancer to select a new backend
func (sa *SessionAffinity) SelectWithClientIP(clientIP string) *backend.Backend {
	// Check if we have an existing session
	sa.mu.RLock()
	if sess, exists := sa.sessions[clientIP]; exists {
		// Check if session is still valid and backend is healthy
		if time.Since(sess.lastAccess) < sa.timeout && sess.backend.IsHealthy() {
			sa.mu.RUnlock()

			// Update last access time
			sa.mu.Lock()
			sess.lastAccess = time.Now()
			sa.mu.Unlock()

			return sess.backend
		}
	}
	sa.mu.RUnlock()

	// No valid session, select a new backend
	selectedBackend := sa.balancer.Select()
	if selectedBackend == nil {
		return nil
	}

	// Store the session
	sa.mu.Lock()
	sa.sessions[clientIP] = &session{
		backend:    selectedBackend,
		lastAccess: time.Now(),
	}
	sa.mu.Unlock()

	return selectedBackend
}

// Select implements the LoadBalancer interface
// This uses the underlying balancer directly (no affinity)
func (sa *SessionAffinity) Select() *backend.Backend {
	return sa.balancer.Select()
}

// Name returns the algorithm name
func (sa *SessionAffinity) Name() string {
	return sa.balancer.Name() + "-with-affinity"
}

// cleanupLoop periodically removes expired sessions
func (sa *SessionAffinity) cleanupLoop() {
	for {
		select {
		case <-sa.cleanupTicker.C:
			sa.cleanup()
		case <-sa.stopCleanup:
			sa.cleanupTicker.Stop()
			return
		}
	}
}

// cleanup removes expired sessions
func (sa *SessionAffinity) cleanup() {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	now := time.Now()
	for clientIP, sess := range sa.sessions {
		if now.Sub(sess.lastAccess) > sa.timeout {
			delete(sa.sessions, clientIP)
		}
	}
}

// ClearSession removes a specific client's session
func (sa *SessionAffinity) ClearSession(clientIP string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	delete(sa.sessions, clientIP)
}

// ClearAllSessions removes all sessions
func (sa *SessionAffinity) ClearAllSessions() {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.sessions = make(map[string]*session)
}

// SessionCount returns the number of active sessions
func (sa *SessionAffinity) SessionCount() int {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return len(sa.sessions)
}

// Stop stops the cleanup goroutine
func (sa *SessionAffinity) Stop() {
	close(sa.stopCleanup)
}
