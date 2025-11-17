package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Request metrics (RED - Rate, Errors, Duration)
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_requests_total",
			Help: "Total number of requests handled",
		},
		[]string{"backend", "method", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "balance_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend", "method"},
	)

	requestErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_request_errors_total",
			Help: "Total number of request errors",
		},
		[]string{"backend", "error_type"},
	)

	// Backend metrics
	backendConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_backend_connections_active",
			Help: "Number of active connections to backend",
		},
		[]string{"backend"},
	)

	backendHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_backend_health_status",
			Help: "Backend health status (1=healthy, 0=unhealthy)",
		},
		[]string{"backend"},
	)

	backendRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_backend_requests_in_flight",
			Help: "Number of requests currently being processed by backend",
		},
		[]string{"backend"},
	)

	// Connection pool metrics
	poolConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_pool_connections_active",
			Help: "Number of active connections in pool",
		},
		[]string{"backend"},
	)

	poolConnectionsIdle = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_pool_connections_idle",
			Help: "Number of idle connections in pool",
		},
		[]string{"backend"},
	)

	poolConnectionsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_pool_connections_created_total",
			Help: "Total number of connections created in pool",
		},
		[]string{"backend"},
	)

	poolConnectionsReused = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_pool_connections_reused_total",
			Help: "Total number of connections reused from pool",
		},
		[]string{"backend"},
	)

	// Circuit breaker metrics
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "balance_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"backend"},
	)

	circuitBreakerOpenTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_circuit_breaker_open_total",
			Help: "Total number of times circuit breaker opened",
		},
		[]string{"backend"},
	)

	// Retry metrics
	retriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_retries_total",
			Help: "Total number of request retries",
		},
		[]string{"backend"},
	)

	retriesExhausted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_retries_exhausted_total",
			Help: "Total number of requests that exhausted all retries",
		},
		[]string{"backend"},
	)

	// TLS metrics
	tlsHandshakesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_tls_handshakes_total",
			Help: "Total number of TLS handshakes",
		},
		[]string{"status"},
	)

	tlsHandshakeDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "balance_tls_handshake_duration_seconds",
			Help:    "TLS handshake duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// Rate limiting metrics
	rateLimitedRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_rate_limited_requests_total",
			Help: "Total number of rate limited requests",
		},
		[]string{"client_ip"},
	)
)

// Collector manages metrics collection
type Collector struct {
	registry *prometheus.Registry
	mu       sync.RWMutex
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		registry: prometheus.NewRegistry(),
	}
}

// RecordRequest records a request metric
func RecordRequest(backend, method, status string, duration time.Duration) {
	requestsTotal.WithLabelValues(backend, method, status).Inc()
	requestDuration.WithLabelValues(backend, method).Observe(duration.Seconds())
}

// RecordRequestError records a request error
func RecordRequestError(backend, errorType string) {
	requestErrors.WithLabelValues(backend, errorType).Inc()
}

// SetBackendConnectionsActive sets the active connections gauge
func SetBackendConnectionsActive(backend string, count int) {
	backendConnectionsActive.WithLabelValues(backend).Set(float64(count))
}

// SetBackendHealthStatus sets the backend health status
func SetBackendHealthStatus(backend string, healthy bool) {
	status := 0.0
	if healthy {
		status = 1.0
	}
	backendHealthStatus.WithLabelValues(backend).Set(status)
}

// IncBackendRequestsInFlight increments in-flight requests
func IncBackendRequestsInFlight(backend string) {
	backendRequestsInFlight.WithLabelValues(backend).Inc()
}

// DecBackendRequestsInFlight decrements in-flight requests
func DecBackendRequestsInFlight(backend string) {
	backendRequestsInFlight.WithLabelValues(backend).Dec()
}

// SetPoolConnectionsActive sets pool active connections
func SetPoolConnectionsActive(backend string, count int) {
	poolConnectionsActive.WithLabelValues(backend).Set(float64(count))
}

// SetPoolConnectionsIdle sets pool idle connections
func SetPoolConnectionsIdle(backend string, count int) {
	poolConnectionsIdle.WithLabelValues(backend).Set(float64(count))
}

// IncPoolConnectionsCreated increments pool connections created
func IncPoolConnectionsCreated(backend string) {
	poolConnectionsCreated.WithLabelValues(backend).Inc()
}

// IncPoolConnectionsReused increments pool connections reused
func IncPoolConnectionsReused(backend string) {
	poolConnectionsReused.WithLabelValues(backend).Inc()
}

// SetCircuitBreakerState sets circuit breaker state
// 0=closed, 1=open, 2=half-open
func SetCircuitBreakerState(backend string, state int) {
	circuitBreakerState.WithLabelValues(backend).Set(float64(state))
}

// IncCircuitBreakerOpen increments circuit breaker open count
func IncCircuitBreakerOpen(backend string) {
	circuitBreakerOpenTotal.WithLabelValues(backend).Inc()
}

// IncRetries increments retry count
func IncRetries(backend string) {
	retriesTotal.WithLabelValues(backend).Inc()
}

// IncRetriesExhausted increments exhausted retries count
func IncRetriesExhausted(backend string) {
	retriesExhausted.WithLabelValues(backend).Inc()
}

// RecordTLSHandshake records a TLS handshake
func RecordTLSHandshake(status string, duration time.Duration) {
	tlsHandshakesTotal.WithLabelValues(status).Inc()
	tlsHandshakeDuration.Observe(duration.Seconds())
}

// IncRateLimitedRequests increments rate limited requests
func IncRateLimitedRequests(clientIP string) {
	rateLimitedRequests.WithLabelValues(clientIP).Inc()
}

// MetricsHandler returns an HTTP handler for Prometheus metrics
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RequestMetricsMiddleware wraps an HTTP handler with metrics collection
func RequestMetricsMiddleware(backend string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Increment in-flight requests
			IncBackendRequestsInFlight(backend)
			defer DecBackendRequestsInFlight(backend)

			// Handle request
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			status := strconv.Itoa(rw.statusCode)
			RecordRequest(backend, r.Method, status, duration)

			// Record error if status >= 500
			if rw.statusCode >= 500 {
				RecordRequestError(backend, "server_error")
			} else if rw.statusCode >= 400 {
				RecordRequestError(backend, "client_error")
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
