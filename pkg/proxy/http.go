package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
	"github.com/therealutkarshpriyadarshi/balance/pkg/lb"
	"github.com/therealutkarshpriyadarshi/balance/pkg/router"
	"golang.org/x/net/http2"
)

// HTTPServer represents an HTTP/HTTPS reverse proxy server
type HTTPServer struct {
	config    *config.Config
	server    *http.Server
	pool      *backend.Pool
	balancer  lb.LoadBalancer
	router    *router.Router
	transport *http.Transport

	// Graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	// Statistics
	totalRequests      atomic.Int64
	activeRequests     atomic.Int64
	totalBytesReceived atomic.Int64
	totalBytesSent     atomic.Int64
	totalErrors        atomic.Int64
}

// NewHTTPServer creates a new HTTP reverse proxy server
func NewHTTPServer(cfg *config.Config) (*Server, error) {
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
	case "weighted-round-robin":
		balancer = lb.NewWeightedRoundRobin(pool)
	case "weighted-least-connections":
		balancer = lb.NewWeightedLeastConnections(pool)
	case "consistent-hash":
		balancer = lb.NewConsistentHash(pool, lb.DefaultVirtualNodes, cfg.LoadBalancer.HashKey)
	case "bounded-consistent-hash":
		balancer = lb.NewBoundedLoadConsistentHash(pool, lb.DefaultVirtualNodes, cfg.LoadBalancer.HashKey, 1.25)
	default:
		return nil, fmt.Errorf("unsupported load balancer algorithm: %s", cfg.LoadBalancer.Algorithm)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create HTTP transport
	transport := &http.Transport{
		MaxIdleConnsPerHost: cfg.HTTP.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.HTTP.IdleConnTimeout,
		DisableKeepAlives:   false,
		DisableCompression:  false,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeouts.Connect,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     cfg.HTTP.EnableHTTP2,
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: cfg.Timeouts.Read,
		WriteBufferSize:       4096,
		ReadBufferSize:        4096,
	}

	// Enable HTTP/2 if configured
	if cfg.HTTP.EnableHTTP2 {
		if err := http2.ConfigureTransport(transport); err != nil {
			log.Printf("Warning: Failed to configure HTTP/2: %v", err)
		}
	}

	// Create router if routes are configured
	var rt *router.Router
	if cfg.HTTP != nil && len(cfg.HTTP.Routes) > 0 {
		rt = router.NewRouter(cfg.HTTP.Routes, pool)
	}

	httpServer := &HTTPServer{
		config:     cfg,
		pool:       pool,
		balancer:   balancer,
		router:     rt,
		transport:  transport,
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// Create HTTP server with handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", httpServer.handleRequest)

	httpServer.server = &http.Server{
		Addr:           cfg.Listen,
		Handler:        mux,
		ReadTimeout:    cfg.Timeouts.Read,
		WriteTimeout:   cfg.Timeouts.Write,
		IdleTimeout:    cfg.Timeouts.Idle,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Enable HTTP/2 on the server if configured
	if cfg.HTTP.EnableHTTP2 {
		http2.ConfigureServer(httpServer.server, &http2.Server{})
	}

	// Return as generic Server type for compatibility
	return &Server{
		config:          cfg,
		pool:            pool,
		balancer:        balancer,
		ctx:             ctx,
		cancelFunc:      cancel,
		httpServer:      httpServer,
	}, nil
}

// handleRequest handles incoming HTTP requests
func (h *HTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Update statistics
	h.totalRequests.Add(1)
	h.activeRequests.Add(1)
	defer h.activeRequests.Add(-1)

	// Check if this is a WebSocket upgrade request
	if h.config.HTTP.EnableWebSocket && isWebSocketRequest(r) {
		h.handleWebSocket(w, r)
		return
	}

	// Select backend pool (use router if configured, otherwise default pool)
	// Note: For now, we use the global load balancer.
	// TODO: In future, create per-route load balancers for better isolation
	if h.router != nil {
		_ = h.router.Match(r) // Route matching for future enhancement
	}

	// Select a backend using load balancer
	var selectedBackend *backend.Backend

	// Check if the balancer supports key-based selection
	clientIP := getClientIP(r)
	switch balancer := h.balancer.(type) {
	case interface{ SelectWithKey(string) *backend.Backend }:
		// Use consistent hash with client IP or custom key
		selectedBackend = balancer.SelectWithKey(clientIP)
	case interface{ SelectWithClientIP(string) *backend.Backend }:
		// Use session affinity with client IP
		selectedBackend = balancer.SelectWithClientIP(clientIP)
	default:
		// Use standard selection
		selectedBackend = h.balancer.Select()
	}

	if selectedBackend == nil {
		h.totalErrors.Add(1)
		http.Error(w, "No healthy backend available", http.StatusServiceUnavailable)
		log.Printf("No healthy backend available for request: %s %s", r.Method, r.URL.Path)
		return
	}

	// Track connection for this backend
	selectedBackend.IncrementConnections()
	defer selectedBackend.DecrementConnections()

	// Build target URL
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     selectedBackend.Address(),
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	log.Printf("Proxying %s %s from %s to backend: %s", r.Method, r.URL.Path, clientIP, selectedBackend.Address())

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = h.transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		h.totalErrors.Add(1)
		log.Printf("Backend error for %s: %v", selectedBackend.Address(), err)
		selectedBackend.MarkUnhealthy()
		http.Error(w, "Backend error", http.StatusBadGateway)
	}

	// Modify request headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Add X-Forwarded headers
		req.Header.Set("X-Forwarded-For", clientIP)
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", getScheme(r))
		req.Header.Set("X-Real-IP", clientIP)
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
}

// handleWebSocket handles WebSocket upgrade and proxying
func (h *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Select backend
	var selectedBackend *backend.Backend
	clientIP := getClientIP(r)

	switch balancer := h.balancer.(type) {
	case interface{ SelectWithKey(string) *backend.Backend }:
		selectedBackend = balancer.SelectWithKey(clientIP)
	case interface{ SelectWithClientIP(string) *backend.Backend }:
		selectedBackend = balancer.SelectWithClientIP(clientIP)
	default:
		selectedBackend = h.balancer.Select()
	}

	if selectedBackend == nil {
		h.totalErrors.Add(1)
		http.Error(w, "No healthy backend available", http.StatusServiceUnavailable)
		return
	}

	selectedBackend.IncrementConnections()
	defer selectedBackend.DecrementConnections()

	log.Printf("WebSocket upgrade: %s -> %s", clientIP, selectedBackend.Address())

	// Dial backend
	backendConn, err := net.DialTimeout("tcp", selectedBackend.Address(), h.config.Timeouts.Connect)
	if err != nil {
		h.totalErrors.Add(1)
		log.Printf("Failed to connect to backend for WebSocket: %v", err)
		selectedBackend.MarkUnhealthy()
		http.Error(w, "Failed to connect to backend", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		h.totalErrors.Add(1)
		http.Error(w, "WebSocket hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		h.totalErrors.Add(1)
		log.Printf("Failed to hijack connection: %v", err)
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Forward the upgrade request to backend
	if err := r.Write(backendConn); err != nil {
		h.totalErrors.Add(1)
		log.Printf("Failed to write upgrade request: %v", err)
		return
	}

	// Proxy WebSocket data bidirectionally
	h.proxyWebSocket(clientConn, backendConn)
}

// proxyWebSocket proxies WebSocket data between client and backend
func (h *HTTPServer) proxyWebSocket(clientConn, backendConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Backend
	go func() {
		defer wg.Done()
		n, err := io.Copy(backendConn, clientConn)
		if err != nil && err != io.EOF {
			log.Printf("Error copying WebSocket client -> backend: %v", err)
		}
		h.totalBytesSent.Add(n)
	}()

	// Backend -> Client
	go func() {
		defer wg.Done()
		n, err := io.Copy(clientConn, backendConn)
		if err != nil && err != io.EOF {
			log.Printf("Error copying WebSocket backend -> client: %v", err)
		}
		h.totalBytesReceived.Add(n)
	}()

	wg.Wait()
}

// Start starts the HTTP server
func (h *HTTPServer) Start() error {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (h *HTTPServer) Shutdown() error {
	log.Println("Shutting down HTTP proxy server...")

	h.cancelFunc()

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.server.Shutdown(ctx); err != nil {
		log.Printf("Error during HTTP server shutdown: %v", err)
	}

	// Close transport
	h.transport.CloseIdleConnections()

	// Wait for all goroutines
	h.wg.Wait()

	// Print final statistics
	log.Printf("Final statistics:")
	log.Printf("  Total requests: %d", h.totalRequests.Load())
	log.Printf("  Total errors: %d", h.totalErrors.Load())
	log.Printf("  Bytes received: %d", h.totalBytesReceived.Load())
	log.Printf("  Bytes sent: %d", h.totalBytesSent.Load())

	return nil
}

// Stats returns current HTTP server statistics
func (h *HTTPServer) Stats() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":       h.totalRequests.Load(),
		"active_requests":      h.activeRequests.Load(),
		"total_errors":         h.totalErrors.Load(),
		"total_bytes_received": h.totalBytesReceived.Load(),
		"total_bytes_sent":     h.totalBytesSent.Load(),
	}
}

// Helper functions

// isWebSocketRequest checks if the request is a WebSocket upgrade
func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Use RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// getScheme returns the request scheme (http or https)
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if scheme := r.Header.Get("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	return "http"
}
