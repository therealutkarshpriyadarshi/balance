package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
	"github.com/therealutkarshpriyadarshi/balance/pkg/lb"
)

// Server represents a proxy server
type Server struct {
	config   *config.Config
	listener net.Listener
	pool     *backend.Pool
	balancer lb.LoadBalancer

	// Graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	// Statistics
	totalConnections    atomic.Int64
	activeConnections   atomic.Int64
	totalBytesReceived  atomic.Int64
	totalBytesSent      atomic.Int64
}

// NewTCPServer creates a new TCP proxy server
func NewTCPServer(cfg *config.Config) (*Server, error) {
	// Create backend pool
	pool := backend.NewPool()
	for _, backendCfg := range cfg.Backends {
		b := backend.NewBackend(backendCfg.Name, backendCfg.Address, backendCfg.Weight)
		pool.Add(b)
	}

	// Create load balancer
	var balancer lb.LoadBalancer

	switch cfg.LoadBalancer.Algorithm {
	case "round-robin":
		balancer = lb.NewRoundRobin(pool)
	case "least-connections":
		balancer = lb.NewLeastConnections(pool)
	default:
		return nil, fmt.Errorf("unsupported load balancer algorithm: %s", cfg.LoadBalancer.Algorithm)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:     cfg,
		pool:       pool,
		balancer:   balancer,
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

// NewHTTPServer creates a new HTTP proxy server
func NewHTTPServer(cfg *config.Config) (*Server, error) {
	// TODO: Implement HTTP server in Phase 3
	return nil, fmt.Errorf("HTTP mode not yet implemented (coming in Phase 3)")
}

// Start starts the proxy server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.Listen)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				// Server is shutting down
				return
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		// Handle connection in a goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(clientConn net.Conn) {
	defer s.wg.Done()
	defer clientConn.Close()

	// Update statistics
	s.totalConnections.Add(1)
	s.activeConnections.Add(1)
	defer s.activeConnections.Add(-1)

	// Select a backend using load balancer
	selectedBackend := s.balancer.Select()
	if selectedBackend == nil {
		log.Printf("No healthy backend available")
		return
	}

	log.Printf("Routing connection to backend: %s", selectedBackend.Address())

	// Connect to backend with timeout
	dialer := net.Dialer{
		Timeout: s.config.Timeouts.Connect,
	}

	backendConn, err := dialer.DialContext(s.ctx, "tcp", selectedBackend.Address())
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", selectedBackend.Address(), err)
		selectedBackend.MarkUnhealthy()
		return
	}
	defer backendConn.Close()

	// Set timeouts
	if s.config.Timeouts.Read > 0 {
		clientConn.SetReadDeadline(time.Now().Add(s.config.Timeouts.Read))
		backendConn.SetReadDeadline(time.Now().Add(s.config.Timeouts.Read))
	}
	if s.config.Timeouts.Write > 0 {
		clientConn.SetWriteDeadline(time.Now().Add(s.config.Timeouts.Write))
		backendConn.SetWriteDeadline(time.Now().Add(s.config.Timeouts.Write))
	}

	// Proxy data bidirectionally
	s.proxyData(clientConn, backendConn)
}

// proxyData proxies data between client and backend connections
func (s *Server) proxyData(clientConn, backendConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Backend
	go func() {
		defer wg.Done()
		n, err := io.Copy(backendConn, clientConn)
		if err != nil && err != io.EOF {
			log.Printf("Error copying client -> backend: %v", err)
		}
		s.totalBytesReceived.Add(n)
		// Close write side to signal EOF
		if conn, ok := backendConn.(*net.TCPConn); ok {
			conn.CloseWrite()
		}
	}()

	// Backend -> Client
	go func() {
		defer wg.Done()
		n, err := io.Copy(clientConn, backendConn)
		if err != nil && err != io.EOF {
			log.Printf("Error copying backend -> client: %v", err)
		}
		s.totalBytesSent.Add(n)
		// Close write side to signal EOF
		if conn, ok := clientConn.(*net.TCPConn); ok {
			conn.CloseWrite()
		}
	}()

	wg.Wait()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	log.Println("Shutting down proxy server...")

	// Stop accepting new connections
	s.cancelFunc()

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
	}

	// Wait for all active connections to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		log.Println("All connections closed")
	case <-time.After(30 * time.Second):
		log.Println("Shutdown timeout exceeded, forcing shutdown")
	}

	// Print final statistics
	log.Printf("Final statistics:")
	log.Printf("  Total connections: %d", s.totalConnections.Load())
	log.Printf("  Bytes received: %d", s.totalBytesReceived.Load())
	log.Printf("  Bytes sent: %d", s.totalBytesSent.Load())

	return nil
}

// Stats returns current server statistics
func (s *Server) Stats() map[string]interface{} {
	return map[string]interface{}{
		"total_connections":   s.totalConnections.Load(),
		"active_connections":  s.activeConnections.Load(),
		"total_bytes_received": s.totalBytesReceived.Load(),
		"total_bytes_sent":    s.totalBytesSent.Load(),
	}
}
