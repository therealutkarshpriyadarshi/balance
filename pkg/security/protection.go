package security

import (
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ProtectionConfig configures security protections
type ProtectionConfig struct {
	// MaxConnectionsPerIP limits concurrent connections per IP
	MaxConnectionsPerIP int

	// MaxConnectionRate limits new connections per second per IP
	MaxConnectionRate float64

	// ReadTimeout for reading request headers (Slowloris protection)
	ReadTimeout time.Duration

	// WriteTimeout for writing responses
	WriteTimeout time.Duration

	// MaxRequestSize limits the maximum request size in bytes
	MaxRequestSize int64

	// MaxHeaderSize limits the maximum header size in bytes
	MaxHeaderSize int64

	// ConnectionTimeout limits how long a connection can be open
	ConnectionTimeout time.Duration
}

// DefaultProtectionConfig returns a default protection configuration
func DefaultProtectionConfig() *ProtectionConfig {
	return &ProtectionConfig{
		MaxConnectionsPerIP: 100,
		MaxConnectionRate:   10.0,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxRequestSize:      10 * 1024 * 1024, // 10 MB
		MaxHeaderSize:       1024 * 1024,      // 1 MB
		ConnectionTimeout:   300 * time.Second, // 5 minutes
	}
}

// ConnectionGuard protects against connection-based attacks
type ConnectionGuard struct {
	config *ProtectionConfig

	mu sync.RWMutex

	// Track connections per IP
	connectionsPerIP map[string]*ipConnections

	// Rate limiter for new connections
	connectionRateLimiter *TokenBucket

	// Statistics
	totalConnections     atomic.Int64
	rejectedConnections  atomic.Int64
	activeConnections    atomic.Int64
	slowlorisDetections  atomic.Int64
}

// ipConnections tracks connections for a single IP
type ipConnections struct {
	count        int
	lastActivity time.Time
}

// NewConnectionGuard creates a new connection guard
func NewConnectionGuard(config *ProtectionConfig) *ConnectionGuard {
	if config == nil {
		config = DefaultProtectionConfig()
	}

	cg := &ConnectionGuard{
		config:                config,
		connectionsPerIP:      make(map[string]*ipConnections),
		connectionRateLimiter: NewTokenBucket(config.MaxConnectionRate, int64(config.MaxConnectionRate*10)),
	}

	// Start cleanup goroutine
	go cg.cleanup()

	return cg
}

// AllowConnection checks if a new connection from the given IP should be allowed
func (cg *ConnectionGuard) AllowConnection(ip string) bool {
	cg.totalConnections.Add(1)

	// Check connection rate limit
	if !cg.connectionRateLimiter.Allow(ip) {
		cg.rejectedConnections.Add(1)
		log.Printf("Connection rate limit exceeded for IP: %s", ip)
		return false
	}

	cg.mu.Lock()
	defer cg.mu.Unlock()

	// Check concurrent connection limit per IP
	if ipConns, exists := cg.connectionsPerIP[ip]; exists {
		if ipConns.count >= cg.config.MaxConnectionsPerIP {
			cg.rejectedConnections.Add(1)
			log.Printf("Max connections exceeded for IP: %s (current: %d, max: %d)",
				ip, ipConns.count, cg.config.MaxConnectionsPerIP)
			return false
		}
		ipConns.count++
		ipConns.lastActivity = time.Now()
	} else {
		cg.connectionsPerIP[ip] = &ipConnections{
			count:        1,
			lastActivity: time.Now(),
		}
	}

	cg.activeConnections.Add(1)
	return true
}

// ReleaseConnection releases a connection for the given IP
func (cg *ConnectionGuard) ReleaseConnection(ip string) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if ipConns, exists := cg.connectionsPerIP[ip]; exists {
		ipConns.count--
		ipConns.lastActivity = time.Now()
		if ipConns.count <= 0 {
			delete(cg.connectionsPerIP, ip)
		}
	}

	cg.activeConnections.Add(-1)
}

// cleanup periodically removes old IP tracking entries
func (cg *ConnectionGuard) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cg.mu.Lock()
		now := time.Now()
		for ip, ipConns := range cg.connectionsPerIP {
			// Remove entries with no connections that haven't been active in 5 minutes
			if ipConns.count == 0 && now.Sub(ipConns.lastActivity) > 5*time.Minute {
				delete(cg.connectionsPerIP, ip)
			}
		}
		cg.mu.Unlock()
	}
}

// DetectSlowloris detects potential Slowloris attacks
// Returns true if the connection appears to be a Slowloris attack
func (cg *ConnectionGuard) DetectSlowloris(conn net.Conn, readDeadline time.Time) bool {
	// Set read deadline to detect slow readers
	if err := conn.SetReadDeadline(readDeadline); err != nil {
		log.Printf("Failed to set read deadline: %v", err)
		return false
	}

	// If the connection is too slow, it might be a Slowloris attack
	// This is a simplified detection - production systems might use more sophisticated methods

	// The actual detection happens when reading from the connection times out
	// This method just sets up the deadline

	return false
}

// Stats returns connection guard statistics
func (cg *ConnectionGuard) Stats() map[string]interface{} {
	cg.mu.RLock()
	trackedIPs := len(cg.connectionsPerIP)
	cg.mu.RUnlock()

	return map[string]interface{}{
		"total_connections":     cg.totalConnections.Load(),
		"rejected_connections":  cg.rejectedConnections.Load(),
		"active_connections":    cg.activeConnections.Load(),
		"slowloris_detections":  cg.slowlorisDetections.Load(),
		"tracked_ips":           trackedIPs,
		"max_connections_per_ip": cg.config.MaxConnectionsPerIP,
	}
}

// RequestSizeGuard protects against large request attacks
type RequestSizeGuard struct {
	maxRequestSize int64
	maxHeaderSize  int64

	totalRequests    atomic.Int64
	rejectedRequests atomic.Int64
}

// NewRequestSizeGuard creates a new request size guard
func NewRequestSizeGuard(maxRequestSize, maxHeaderSize int64) *RequestSizeGuard {
	return &RequestSizeGuard{
		maxRequestSize: maxRequestSize,
		maxHeaderSize:  maxHeaderSize,
	}
}

// CheckRequestSize checks if a request size is within limits
func (g *RequestSizeGuard) CheckRequestSize(size int64) bool {
	g.totalRequests.Add(1)

	if size > g.maxRequestSize {
		g.rejectedRequests.Add(1)
		log.Printf("Request size exceeded limit: %d > %d", size, g.maxRequestSize)
		return false
	}

	return true
}

// CheckHeaderSize checks if a header size is within limits
func (g *RequestSizeGuard) CheckHeaderSize(size int64) bool {
	if size > g.maxHeaderSize {
		log.Printf("Header size exceeded limit: %d > %d", size, g.maxHeaderSize)
		return false
	}

	return true
}

// Stats returns request size guard statistics
func (g *RequestSizeGuard) Stats() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":    g.totalRequests.Load(),
		"rejected_requests": g.rejectedRequests.Load(),
		"max_request_size":  g.maxRequestSize,
		"max_header_size":   g.maxHeaderSize,
	}
}

// IPBlocklist manages a blocklist of IP addresses
type IPBlocklist struct {
	mu sync.RWMutex

	// blocked maps IP addresses to block expiry time
	blocked map[string]time.Time

	// Permanent blocks (never expire)
	permanent map[string]bool

	// Statistics
	totalBlocks   atomic.Int64
	activeBlocks  atomic.Int64
	blockedRequests atomic.Int64
}

// NewIPBlocklist creates a new IP blocklist
func NewIPBlocklist() *IPBlocklist {
	bl := &IPBlocklist{
		blocked:   make(map[string]time.Time),
		permanent: make(map[string]bool),
	}

	// Start cleanup goroutine
	go bl.cleanup()

	return bl
}

// Block blocks an IP address for the specified duration
func (bl *IPBlocklist) Block(ip string, duration time.Duration) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.blocked[ip] = time.Now().Add(duration)
	bl.totalBlocks.Add(1)
	bl.activeBlocks.Add(1)

	log.Printf("Blocked IP %s for %s", ip, duration)
}

// BlockPermanent permanently blocks an IP address
func (bl *IPBlocklist) BlockPermanent(ip string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.permanent[ip] = true
	bl.totalBlocks.Add(1)
	bl.activeBlocks.Add(1)

	log.Printf("Permanently blocked IP %s", ip)
}

// Unblock removes an IP from the blocklist
func (bl *IPBlocklist) Unblock(ip string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	if _, exists := bl.blocked[ip]; exists {
		delete(bl.blocked, ip)
		bl.activeBlocks.Add(-1)
	}

	if _, exists := bl.permanent[ip]; exists {
		delete(bl.permanent, ip)
		bl.activeBlocks.Add(-1)
	}

	log.Printf("Unblocked IP %s", ip)
}

// IsBlocked checks if an IP address is blocked
func (bl *IPBlocklist) IsBlocked(ip string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	// Check permanent blocks
	if bl.permanent[ip] {
		bl.blockedRequests.Add(1)
		return true
	}

	// Check temporary blocks
	if expiry, exists := bl.blocked[ip]; exists {
		if time.Now().Before(expiry) {
			bl.blockedRequests.Add(1)
			return true
		}
	}

	return false
}

// cleanup periodically removes expired blocks
func (bl *IPBlocklist) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		bl.mu.Lock()
		now := time.Now()
		for ip, expiry := range bl.blocked {
			if now.After(expiry) {
				delete(bl.blocked, ip)
				bl.activeBlocks.Add(-1)
			}
		}
		bl.mu.Unlock()
	}
}

// Stats returns blocklist statistics
func (bl *IPBlocklist) Stats() map[string]interface{} {
	bl.mu.RLock()
	permanentCount := len(bl.permanent)
	temporaryCount := len(bl.blocked)
	bl.mu.RUnlock()

	return map[string]interface{}{
		"total_blocks":      bl.totalBlocks.Load(),
		"active_blocks":     bl.activeBlocks.Load(),
		"blocked_requests":  bl.blockedRequests.Load(),
		"permanent_blocks":  permanentCount,
		"temporary_blocks":  temporaryCount,
	}
}

// SecurityManager combines all security protections
type SecurityManager struct {
	connectionGuard   *ConnectionGuard
	requestSizeGuard  *RequestSizeGuard
	rateLimiter       RateLimiter
	blocklist         *IPBlocklist
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *ProtectionConfig, rateLimiter RateLimiter) *SecurityManager {
	if config == nil {
		config = DefaultProtectionConfig()
	}

	return &SecurityManager{
		connectionGuard:  NewConnectionGuard(config),
		requestSizeGuard: NewRequestSizeGuard(config.MaxRequestSize, config.MaxHeaderSize),
		rateLimiter:      rateLimiter,
		blocklist:        NewIPBlocklist(),
	}
}

// AllowConnection checks if a connection should be allowed
func (sm *SecurityManager) AllowConnection(ip string) (bool, string) {
	// Check blocklist first
	if sm.blocklist.IsBlocked(ip) {
		return false, "IP is blocked"
	}

	// Check rate limit
	if sm.rateLimiter != nil && !sm.rateLimiter.Allow(ip) {
		return false, "Rate limit exceeded"
	}

	// Check connection guard
	if !sm.connectionGuard.AllowConnection(ip) {
		return false, "Too many connections"
	}

	return true, ""
}

// ReleaseConnection releases a connection
func (sm *SecurityManager) ReleaseConnection(ip string) {
	sm.connectionGuard.ReleaseConnection(ip)
}

// CheckRequestSize checks if a request size is acceptable
func (sm *SecurityManager) CheckRequestSize(size int64) bool {
	return sm.requestSizeGuard.CheckRequestSize(size)
}

// BlockIP blocks an IP address
func (sm *SecurityManager) BlockIP(ip string, duration time.Duration) {
	sm.blocklist.Block(ip, duration)
}

// Stats returns combined security statistics
func (sm *SecurityManager) Stats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["connection_guard"] = sm.connectionGuard.Stats()
	stats["request_size_guard"] = sm.requestSizeGuard.Stats()
	stats["blocklist"] = sm.blocklist.Stats()

	if sm.rateLimiter != nil {
		stats["rate_limiter"] = sm.rateLimiter.Stats()
	}

	return stats
}

// GetClientIP extracts the client IP from a network address
func GetClientIP(addr net.Addr) string {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}

	// Fallback: parse the string representation
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// ValidateIP checks if an IP address is valid
func ValidateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}
