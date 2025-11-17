package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"
)

// Terminator handles TLS termination
type Terminator struct {
	config    *Config
	certMgr   *CertificateManager
	listener  net.Listener
	tlsConfig *tls.Config

	// Session cache for TLS session resumption
	sessionCache tls.ClientSessionCache

	// Statistics
	totalConnections     atomic.Int64
	activeConnections    atomic.Int64
	totalHandshakes      atomic.Int64
	failedHandshakes     atomic.Int64
	resumedSessions      atomic.Int64
	handshakeDuration    atomic.Int64 // Total handshake time in microseconds
}

// NewTerminator creates a new TLS terminator
func NewTerminator(config *Config, certMgr *CertificateManager) (*Terminator, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid TLS config: %w", err)
	}

	if certMgr == nil {
		return nil, fmt.Errorf("certificate manager is required")
	}

	t := &Terminator{
		config:       config,
		certMgr:      certMgr,
		sessionCache: tls.NewLRUClientSessionCache(1024), // Cache up to 1024 sessions
	}

	// Build tls.Config
	t.tlsConfig = t.buildTLSConfig()

	return t, nil
}

// buildTLSConfig builds the crypto/tls.Config from our configuration
func (t *Terminator) buildTLSConfig() *tls.Config {
	tlsConfig := t.config.ToStdConfig()

	// Set certificate callback for SNI support
	tlsConfig.GetCertificate = t.certMgr.GetCertificate

	// Enable session ticket support for resumption (unless disabled)
	if !t.config.SessionTicketsDisabled {
		tlsConfig.SessionTicketsDisabled = false
		// Optionally set session ticket key for multi-instance deployments
		if t.config.SessionTicketKey != [32]byte{} {
			tlsConfig.SetSessionTicketKeys([][32]byte{t.config.SessionTicketKey})
		}
	}

	// Set client session cache for outgoing connections (backend TLS)
	tlsConfig.ClientSessionCache = t.sessionCache

	return tlsConfig
}

// Listen creates a TLS listener on the specified address
func (t *Terminator) Listen(address string) error {
	// Create base listener
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	// Wrap with TLS
	t.listener = tls.NewListener(listener, t.tlsConfig)

	log.Printf("TLS listener started on %s", address)
	log.Printf("TLS configuration: MinVersion=%s, CipherSuites=%d",
		tlsVersionString(t.config.MinVersion), len(t.config.CipherSuites))

	return nil
}

// Accept accepts a new TLS connection
func (t *Terminator) Accept() (net.Conn, error) {
	if t.listener == nil {
		return nil, fmt.Errorf("listener not initialized")
	}

	// Accept connection
	conn, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}

	// Update statistics
	t.totalConnections.Add(1)
	t.activeConnections.Add(1)

	// Wrap connection to track statistics
	return &trackedConn{
		Conn:       conn,
		terminator: t,
	}, nil
}

// AcceptWithContext accepts a new TLS connection with context
func (t *Terminator) AcceptWithContext(ctx context.Context) (net.Conn, error) {
	// Create a channel for the result
	type result struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan result, 1)

	go func() {
		conn, err := t.Accept()
		resultCh <- result{conn, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.conn, res.err
	}
}

// Close closes the TLS listener
func (t *Terminator) Close() error {
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

// Addr returns the listener's network address
func (t *Terminator) Addr() net.Addr {
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}

// PerformHandshake performs the TLS handshake on a connection
func (t *Terminator) PerformHandshake(conn net.Conn) (*tls.Conn, error) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("connection is not a TLS connection")
	}

	// Track handshake
	t.totalHandshakes.Add(1)
	start := time.Now()

	// Perform handshake
	if err := tlsConn.Handshake(); err != nil {
		t.failedHandshakes.Add(1)
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Record handshake duration
	duration := time.Since(start)
	t.handshakeDuration.Add(int64(duration.Microseconds()))

	// Check if session was resumed
	state := tlsConn.ConnectionState()
	if state.DidResume {
		t.resumedSessions.Add(1)
	}

	return tlsConn, nil
}

// Stats returns current terminator statistics
func (t *Terminator) Stats() map[string]interface{} {
	totalHandshakes := t.totalHandshakes.Load()
	avgHandshakeDuration := int64(0)
	if totalHandshakes > 0 {
		avgHandshakeDuration = t.handshakeDuration.Load() / totalHandshakes
	}

	return map[string]interface{}{
		"total_connections":       t.totalConnections.Load(),
		"active_connections":      t.activeConnections.Load(),
		"total_handshakes":        totalHandshakes,
		"failed_handshakes":       t.failedHandshakes.Load(),
		"resumed_sessions":        t.resumedSessions.Load(),
		"avg_handshake_duration":  avgHandshakeDuration, // microseconds
	}
}

// GetTLSConfig returns the underlying TLS configuration
func (t *Terminator) GetTLSConfig() *tls.Config {
	return t.tlsConfig
}

// UpdateConfig updates the TLS configuration (for hot-reload)
func (t *Terminator) UpdateConfig(config *Config) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid TLS config: %w", err)
	}

	t.config = config
	t.tlsConfig = t.buildTLSConfig()

	log.Printf("TLS configuration updated")
	return nil
}

// trackedConn wraps a connection to track statistics
type trackedConn struct {
	net.Conn
	terminator *Terminator
	closed     bool
}

// Close closes the connection and updates statistics
func (c *trackedConn) Close() error {
	if !c.closed {
		c.terminator.activeConnections.Add(-1)
		c.closed = true
	}
	return c.Conn.Close()
}

// Helper functions

func tlsVersionString(version TLSVersion) string {
	switch version {
	case VersionTLS10:
		return "TLS 1.0"
	case VersionTLS11:
		return "TLS 1.1"
	case VersionTLS12:
		return "TLS 1.2"
	case VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%d)", version)
	}
}

// DialTLS creates a TLS connection to a backend server
func (t *Terminator) DialTLS(network, address string, timeout time.Duration) (*tls.Conn, error) {
	// Create dialer
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	// Create TLS config for client connection
	clientConfig := t.tlsConfig.Clone()
	// For backend connections, we might want different settings
	// For now, use the same config

	// Dial with TLS
	conn, err := tls.DialWithDialer(dialer, network, address, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TLS: %w", err)
	}

	return conn, nil
}

// DialTLSContext creates a TLS connection to a backend server with context
func (t *Terminator) DialTLSContext(ctx context.Context, network, address string) (*tls.Conn, error) {
	// Create dialer
	dialer := &net.Dialer{}

	// Dial plain connection first
	plainConn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	// Upgrade to TLS
	clientConfig := t.tlsConfig.Clone()
	tlsConn := tls.Client(plainConn, clientConfig)

	// Perform handshake
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		plainConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}
