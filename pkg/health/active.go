package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// CheckType represents the type of health check
type CheckType string

const (
	// CheckTypeTCP performs a TCP connection check
	CheckTypeTCP CheckType = "tcp"

	// CheckTypeHTTP performs an HTTP GET request
	CheckTypeHTTP CheckType = "http"

	// CheckTypeHTTPS performs an HTTPS GET request
	CheckTypeHTTPS CheckType = "https"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	// Backend that was checked
	Backend *backend.Backend

	// Success indicates if the check passed
	Success bool

	// Error contains any error that occurred
	Error error

	// Duration of the health check
	Duration time.Duration

	// Timestamp of the check
	Timestamp time.Time

	// StatusCode for HTTP checks
	StatusCode int
}

// ActiveChecker performs active health checks on backends
type ActiveChecker struct {
	// Type of health check to perform
	checkType CheckType

	// Timeout for health checks
	timeout time.Duration

	// HTTP path for HTTP health checks
	httpPath string

	// Expected HTTP status codes (default: 200)
	expectedStatusCodes []int

	// HTTP client for HTTP health checks
	httpClient *http.Client
}

// ActiveCheckerConfig configures an active health checker
type ActiveCheckerConfig struct {
	// CheckType specifies the type of health check
	CheckType CheckType

	// Timeout for each health check
	Timeout time.Duration

	// HTTPPath is the path for HTTP health checks (e.g., "/health")
	HTTPPath string

	// ExpectedStatusCodes are the HTTP status codes considered healthy
	ExpectedStatusCodes []int
}

// NewActiveChecker creates a new active health checker
func NewActiveChecker(config ActiveCheckerConfig) *ActiveChecker {
	// Default values
	if config.CheckType == "" {
		config.CheckType = CheckTypeTCP
	}
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}
	if config.HTTPPath == "" {
		config.HTTPPath = "/health"
	}
	if len(config.ExpectedStatusCodes) == 0 {
		config.ExpectedStatusCodes = []int{http.StatusOK}
	}

	return &ActiveChecker{
		checkType:           config.CheckType,
		timeout:             config.Timeout,
		httpPath:            config.HTTPPath,
		expectedStatusCodes: config.ExpectedStatusCodes,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				DisableKeepAlives:   true,
				MaxIdleConnsPerHost: 1,
				IdleConnTimeout:     config.Timeout,
			},
		},
	}
}

// Check performs a health check on the given backend
func (ac *ActiveChecker) Check(ctx context.Context, b *backend.Backend) CheckResult {
	start := time.Now()
	result := CheckResult{
		Backend:   b,
		Timestamp: start,
	}

	var err error
	switch ac.checkType {
	case CheckTypeTCP:
		err = ac.checkTCP(ctx, b.Address())
	case CheckTypeHTTP:
		result.StatusCode, err = ac.checkHTTP(ctx, "http://"+b.Address()+ac.httpPath)
	case CheckTypeHTTPS:
		result.StatusCode, err = ac.checkHTTP(ctx, "https://"+b.Address()+ac.httpPath)
	default:
		err = fmt.Errorf("unsupported check type: %s", ac.checkType)
	}

	result.Duration = time.Since(start)
	result.Error = err
	result.Success = err == nil

	return result
}

// checkTCP performs a TCP connection check
func (ac *ActiveChecker) checkTCP(ctx context.Context, address string) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("TCP connection failed: %w", err)
	}
	conn.Close()
	return nil
}

// checkHTTP performs an HTTP health check
func (ac *ActiveChecker) checkHTTP(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if status code is expected
	for _, code := range ac.expectedStatusCodes {
		if resp.StatusCode == code {
			return resp.StatusCode, nil
		}
	}

	return resp.StatusCode, fmt.Errorf("unexpected status code: %d (expected: %v)", resp.StatusCode, ac.expectedStatusCodes)
}

// CheckMultiple checks multiple backends concurrently
func (ac *ActiveChecker) CheckMultiple(ctx context.Context, backends []*backend.Backend) []CheckResult {
	results := make([]CheckResult, len(backends))
	resultChan := make(chan struct {
		index  int
		result CheckResult
	}, len(backends))

	// Launch health checks concurrently
	for i, b := range backends {
		go func(index int, backend *backend.Backend) {
			result := ac.Check(ctx, backend)
			resultChan <- struct {
				index  int
				result CheckResult
			}{index, result}
		}(i, b)
	}

	// Collect results
	for i := 0; i < len(backends); i++ {
		res := <-resultChan
		results[res.index] = res.result
	}

	return results
}

// CheckWithTimeout performs a health check with a timeout
func (ac *ActiveChecker) CheckWithTimeout(b *backend.Backend, timeout time.Duration) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ac.Check(ctx, b)
}
