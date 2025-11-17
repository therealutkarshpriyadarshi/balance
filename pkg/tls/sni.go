package tls

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

// SNIRouter routes connections based on Server Name Indication (SNI)
type SNIRouter struct {
	mu sync.RWMutex

	// routes maps SNI hostnames to backend addresses
	routes map[string][]string

	// defaultBackends is used when no SNI match is found
	defaultBackends []string

	// certManager manages certificates for different domains
	certManager *CertificateManager

	// Statistics
	totalRequests atomic.Int64
	routedByHost  map[string]*atomic.Int64 // Per-host routing stats
	statsMu       sync.RWMutex
}

// NewSNIRouter creates a new SNI router
func NewSNIRouter(certManager *CertificateManager) *SNIRouter {
	return &SNIRouter{
		routes:       make(map[string][]string),
		certManager:  certManager,
		routedByHost: make(map[string]*atomic.Int64),
	}
}

// AddRoute adds a route for an SNI hostname to backend addresses
func (r *SNIRouter) AddRoute(hostname string, backends []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if len(backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	r.routes[hostname] = backends

	// Initialize stats counter for this host
	r.statsMu.Lock()
	if _, exists := r.routedByHost[hostname]; !exists {
		r.routedByHost[hostname] = &atomic.Int64{}
	}
	r.statsMu.Unlock()

	log.Printf("Added SNI route: %s -> %v", hostname, backends)
	return nil
}

// RemoveRoute removes a route for an SNI hostname
func (r *SNIRouter) RemoveRoute(hostname string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.routes, hostname)
	log.Printf("Removed SNI route: %s", hostname)
}

// SetDefaultBackends sets the default backends when no SNI match is found
func (r *SNIRouter) SetDefaultBackends(backends []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultBackends = backends
	log.Printf("Set default backends: %v", backends)
}

// Route returns the backend addresses for the given SNI hostname
func (r *SNIRouter) Route(hostname string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.totalRequests.Add(1)

	// Try exact match
	if backends, ok := r.routes[hostname]; ok {
		r.incrementHostStats(hostname)
		return backends
	}

	// Try wildcard match
	if backends := r.matchWildcard(hostname); backends != nil {
		r.incrementHostStats(hostname)
		return backends
	}

	// Return default backends
	return r.defaultBackends
}

// matchWildcard matches wildcard hostnames (e.g., *.example.com)
func (r *SNIRouter) matchWildcard(hostname string) []string {
	for pattern, backends := range r.routes {
		if matchWildcardPattern(pattern, hostname) {
			return backends
		}
	}
	return nil
}

// matchWildcardPattern checks if hostname matches a wildcard pattern
func matchWildcardPattern(pattern, hostname string) bool {
	// Simple wildcard matching
	// Supports patterns like: *.example.com

	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	suffix := pattern[1:] // Remove the '*'
	return strings.HasSuffix(hostname, suffix)
}

// incrementHostStats increments the routing statistics for a host
func (r *SNIRouter) incrementHostStats(hostname string) {
	r.statsMu.RLock()
	if counter, ok := r.routedByHost[hostname]; ok {
		counter.Add(1)
	}
	r.statsMu.RUnlock()
}

// Stats returns routing statistics
func (r *SNIRouter) Stats() map[string]interface{} {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()

	hostStats := make(map[string]int64)
	for host, counter := range r.routedByHost {
		hostStats[host] = counter.Load()
	}

	return map[string]interface{}{
		"total_requests": r.totalRequests.Load(),
		"by_host":        hostStats,
	}
}

// GetCertificateForSNI returns the certificate for a given SNI hostname
// This can be used as tls.Config.GetCertificate
func (r *SNIRouter) GetCertificateForSNI(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if r.certManager == nil {
		return nil, fmt.Errorf("certificate manager not configured")
	}

	return r.certManager.GetCertificate(hello)
}

// SNIHandler handles SNI-based routing at the TLS level
type SNIHandler struct {
	router      *SNIRouter
	certManager *CertificateManager
}

// NewSNIHandler creates a new SNI handler
func NewSNIHandler(router *SNIRouter, certManager *CertificateManager) *SNIHandler {
	return &SNIHandler{
		router:      router,
		certManager: certManager,
	}
}

// GetCertificate is a callback for tls.Config.GetCertificate
func (h *SNIHandler) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	serverName := hello.ServerName

	log.Printf("SNI request for: %s", serverName)

	// Get certificate from certificate manager
	cert, err := h.certManager.GetCertificate(hello)
	if err != nil {
		log.Printf("Failed to get certificate for %s: %v", serverName, err)
		return nil, err
	}

	// Route the request (for statistics/logging)
	if h.router != nil {
		backends := h.router.Route(serverName)
		log.Printf("SNI routing %s to backends: %v", serverName, backends)
	}

	return cert, nil
}

// ParseSNI extracts the SNI hostname from a TLS ClientHello message
// This is a utility function that can be used for early SNI inspection
func ParseSNI(data []byte) (string, error) {
	// This is a simplified SNI parser
	// In production, you might want to use a more robust implementation

	// TLS record header: 1 byte type, 2 bytes version, 2 bytes length
	if len(data) < 5 {
		return "", fmt.Errorf("data too short for TLS record")
	}

	// Check if this is a handshake record (type 22)
	if data[0] != 22 {
		return "", fmt.Errorf("not a TLS handshake record")
	}

	// Skip to handshake message
	pos := 5

	// Handshake header: 1 byte type, 3 bytes length
	if len(data) < pos+4 {
		return "", fmt.Errorf("data too short for handshake header")
	}

	// Check if this is a ClientHello (type 1)
	if data[pos] != 1 {
		return "", fmt.Errorf("not a ClientHello message")
	}

	// Skip handshake header and ClientHello fixed fields
	// This is a simplified version - real parsing would be more complex
	pos += 4 + 2 + 32 // handshake header + version + random

	if len(data) < pos+1 {
		return "", fmt.Errorf("data too short for session ID")
	}

	// Skip session ID
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen

	if len(data) < pos+2 {
		return "", fmt.Errorf("data too short for cipher suites")
	}

	// Skip cipher suites
	cipherSuitesLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2 + cipherSuitesLen

	if len(data) < pos+1 {
		return "", fmt.Errorf("data too short for compression methods")
	}

	// Skip compression methods
	compressionMethodsLen := int(data[pos])
	pos += 1 + compressionMethodsLen

	if len(data) < pos+2 {
		return "", fmt.Errorf("no extensions present")
	}

	// Parse extensions
	extensionsLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2

	end := pos + extensionsLen
	if len(data) < end {
		return "", fmt.Errorf("data too short for extensions")
	}

	// Look for SNI extension (type 0)
	for pos < end {
		if len(data) < pos+4 {
			break
		}

		extType := int(data[pos])<<8 | int(data[pos+1])
		extLen := int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4

		if extType == 0 && len(data) >= pos+extLen {
			// Found SNI extension
			// Parse server name list
			if extLen < 2 {
				return "", fmt.Errorf("invalid SNI extension length")
			}

			listLen := int(data[pos])<<8 | int(data[pos+1])
			pos += 2

			if listLen < 3 || len(data) < pos+listLen {
				return "", fmt.Errorf("invalid SNI list length")
			}

			// Parse first server name
			nameType := data[pos]
			nameLen := int(data[pos+1])<<8 | int(data[pos+2])
			pos += 3

			if nameType == 0 && len(data) >= pos+nameLen { // hostname type
				hostname := string(data[pos : pos+nameLen])
				return hostname, nil
			}
		}

		pos += extLen
	}

	return "", fmt.Errorf("SNI extension not found")
}
