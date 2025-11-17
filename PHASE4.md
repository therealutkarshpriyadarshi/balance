# Phase 4: TLS & Security - Implementation Complete ✅

## Overview

Phase 4 implements comprehensive TLS/SSL support and security features for the Balance load balancer. This phase adds production-grade encryption, certificate management, and protection against common attacks.

**Status**: ✅ **Completed**
**Timeline**: Weeks 7-8
**Complexity**: High

## Features Implemented

### Week 7: TLS Termination ✅

#### 1. TLS Configuration Management
- **File**: `pkg/tls/config.go`
- Configurable TLS versions (1.0, 1.1, 1.2, 1.3)
- Secure cipher suite selection
- Session ticket support for resumption
- ALPN protocol negotiation
- Client authentication options

**Key Features**:
- Default to TLS 1.2+ for security
- Secure cipher suites (ECDHE with AES-GCM or ChaCha20-Poly1305)
- Configurable server cipher suite preference
- Session resumption for performance

**Code Example**:
```go
config := tls.DefaultConfig()
config.MinVersion = tls.VersionTLS12
config.MaxVersion = tls.VersionTLS13
config.NextProtos = []string{"h2", "http/1.1"}
```

#### 2. Certificate Manager
- **File**: `pkg/tls/cert_manager.go`
- Multi-domain certificate support
- Automatic certificate selection based on SNI
- Wildcard certificate matching
- Certificate expiry checking
- Self-signed certificate generation for testing

**Key Features**:
- Manage multiple certificates for different domains
- Automatic SNI-based selection
- Wildcard support (`*.example.com`)
- Certificate validation and expiry warnings
- Hot-reload capability

**Code Example**:
```go
certMgr := tls.NewCertificateManager()
cert, err := certMgr.LoadCertificate("cert.pem", "key.pem")
certMgr.AddCertificate(cert)

// Get certificate for SNI
tlsCert, err := certMgr.GetCertificate(clientHello)
```

#### 3. TLS Termination
- **File**: `pkg/tls/termination.go`
- TLS listener setup with crypto/tls
- Session resumption with ticket cache
- Connection tracking and statistics
- Backend TLS connections
- mTLS support for backends

**Key Features**:
- Efficient session resumption (LRU cache)
- Per-connection handshake tracking
- Average handshake duration metrics
- Context-aware connection handling

**Code Example**:
```go
terminator, err := tls.NewTerminator(tlsConfig, certManager)
terminator.Listen(":443")

conn, err := terminator.Accept()
```

### Week 8: SNI & Security Features ✅

#### 4. SNI (Server Name Indication)
- **File**: `pkg/tls/sni.go`
- SNI-based certificate selection
- SNI-based routing to different backends
- Wildcard hostname matching
- Per-host routing statistics

**Key Features**:
- Route different domains to different backend pools
- Support for wildcard domains
- Fallback to default backends
- Detailed routing statistics

**Code Example**:
```go
sniRouter := tls.NewSNIRouter(certManager)
sniRouter.AddRoute("api.example.com", []string{"backend1:8080", "backend2:8080"})
sniRouter.AddRoute("*.example.com", []string{"backend3:8080"})

backends := sniRouter.Route("www.example.com")
```

#### 5. Rate Limiting
- **File**: `pkg/security/ratelimit.go`
- Token bucket algorithm
- Sliding window algorithm
- Per-IP rate limiting
- Combined rate limiters

**Key Features**:
- Token bucket with configurable rate and burst
- Sliding window with time-based limits
- Per-client IP tracking
- Automatic cleanup of old entries
- Comprehensive statistics

**Code Example**:
```go
// Token bucket: 100 requests/second, burst of 200
limiter := security.NewTokenBucket(100.0, 200)

if limiter.Allow(clientIP) {
    // Process request
}

// Sliding window: 1000 requests per minute
limiter := security.NewSlidingWindow(1000, 1*time.Minute)
```

#### 6. Security Protections
- **File**: `pkg/security/protection.go`
- Connection flood protection
- Slowloris attack mitigation
- Request size limits
- IP blocklist (temporary and permanent)
- Connection tracking per IP

**Key Features**:
- Max connections per IP
- Connection rate limiting
- Read timeout for Slowloris protection
- Request/header size limits
- IP blocking with expiry

**Code Example**:
```go
config := security.DefaultProtectionConfig()
securityMgr := security.NewSecurityManager(config, rateLimiter)

if allowed, reason := securityMgr.AllowConnection(clientIP); !allowed {
    log.Printf("Connection blocked: %s", reason)
    return
}

defer securityMgr.ReleaseConnection(clientIP)
```

## Configuration

### TLS Configuration

The configuration has been enhanced to support comprehensive TLS settings:

```yaml
mode: http
listen: ":443"

tls:
  enabled: true

  # Multi-domain certificates
  certificates:
    - cert_file: "certs/example.com.crt"
      key_file: "certs/example.com.key"
      domains:
        - "example.com"
        - "www.example.com"
      default: true

    - cert_file: "certs/api.example.com.crt"
      key_file: "certs/api.example.com.key"
      domains:
        - "api.example.com"

  # TLS version settings
  min_version: "1.2"
  max_version: "1.3"

  # Cipher suites (empty = use secure defaults)
  cipher_suites: []

  # Server preferences
  prefer_server_cipher_suites: true
  session_tickets_disabled: false

  # ALPN protocols
  alpn_protocols:
    - "h2"
    - "http/1.1"

  # Client authentication
  client_auth: "none"  # Options: none, request, require, verify, require-and-verify
  client_ca_file: ""

  # Backend TLS
  backend:
    enabled: false
    insecure_skip_verify: false
    ca_file: ""
    client_cert_file: ""
    client_key_file: ""

  # SNI routing
  sni:
    routes:
      "api.example.com": ["backend1:8080", "backend2:8080"]
      "www.example.com": ["backend3:8080", "backend4:8080"]

backends:
  - name: backend1
    address: "localhost:9001"
  - name: backend2
    address: "localhost:9002"
  - name: backend3
    address: "localhost:9003"
  - name: backend4
    address: "localhost:9004"

load_balancer:
  algorithm: "round-robin"
```

### Security Configuration

```yaml
security:
  # Rate limiting
  rate_limit:
    enabled: true
    type: "token-bucket"  # or "sliding-window"
    requests_per_second: 100.0
    burst_size: 200
    # For sliding window:
    # window_size: "1m"
    # max_requests: 1000

  # Connection protection
  connection_protection:
    max_connections_per_ip: 100
    max_connection_rate: 10.0
    read_timeout: "10s"
    max_request_size: 10485760  # 10 MB
    max_header_size: 1048576    # 1 MB

  # IP blocklist
  ip_blocklist:
    blocked_ips:
      - "192.168.1.100"
      - "10.0.0.50"
    blocked_cidrs:
      - "172.16.0.0/16"
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Client                                  │
│                            │                                     │
│                            │ HTTPS Request                       │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  TLS Terminator                             │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │  • SNI Inspection                                     │  │ │
│  │  │  • Certificate Selection (via Certificate Manager)   │  │ │
│  │  │  • TLS Handshake                                      │  │ │
│  │  │  • Session Resumption                                 │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│                            │ Decrypted Request                   │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                Security Manager                             │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │  • IP Blocklist Check                                 │  │ │
│  │  │  • Rate Limiting                                      │  │ │
│  │  │  • Connection Guard                                   │  │ │
│  │  │  • Request Size Validation                            │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│                            │ Validated Request                   │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    SNI Router                               │ │
│  │  • Route based on SNI hostname                             │ │
│  │  • Wildcard matching                                       │ │
│  │  • Default backend fallback                                │ │
│  └────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Load Balancer                              │ │
│  │  • Select backend from pool                                │ │
│  └────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Backend                                 │ │
│  │  • HTTP or HTTPS backend connection                        │ │
│  │  • Optional mTLS                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Testing

All components have comprehensive test coverage:

### TLS Tests
- **config_test.go**: TLS configuration parsing and validation
- **cert_manager_test.go**: Certificate management, SNI selection, wildcard matching

### Security Tests
- **ratelimit_test.go**: Token bucket, sliding window, combined limiters
- **protection_test.go**: Connection guard, request size limits, IP blocklist

### Running Tests

```bash
# Run all tests
make test

# Run TLS tests only
go test -v ./pkg/tls/...

# Run security tests only
go test -v ./pkg/security/...

# Run with coverage
go test -cover ./pkg/tls/...
go test -cover ./pkg/security/...

# Benchmark rate limiters
go test -bench=. ./pkg/security/
```

## Security Best Practices

### TLS Configuration

1. **Use TLS 1.2+**: Never use TLS 1.0 or 1.1 in production
   ```yaml
   min_version: "1.2"
   ```

2. **Prefer Secure Cipher Suites**: Use ECDHE with AES-GCM or ChaCha20-Poly1305
   ```yaml
   cipher_suites: []  # Uses secure defaults
   ```

3. **Enable HTTP/2**: For better performance
   ```yaml
   alpn_protocols: ["h2", "http/1.1"]
   ```

4. **Certificate Management**:
   - Keep private keys secure (chmod 600)
   - Monitor certificate expiry
   - Use strong key sizes (2048+ bit RSA, or ECDSA)
   - Implement automated renewal (Let's Encrypt, etc.)

### Rate Limiting

1. **Set Appropriate Limits**: Balance security and usability
   ```yaml
   requests_per_second: 100.0  # Adjust based on expected traffic
   burst_size: 200             # Allow temporary bursts
   ```

2. **Choose the Right Algorithm**:
   - **Token Bucket**: Smooth rate limiting with bursts
   - **Sliding Window**: Strict time-based limits

3. **Monitor and Adjust**: Use statistics to tune limits
   ```go
   stats := rateLimiter.Stats()
   log.Printf("Blocked: %d/%d requests", stats["blocked"], stats["total_requests"])
   ```

### Connection Protection

1. **Limit Connections per IP**: Prevent resource exhaustion
   ```yaml
   max_connections_per_ip: 100
   ```

2. **Enable Read Timeouts**: Protect against Slowloris
   ```yaml
   read_timeout: "10s"
   ```

3. **Set Request Size Limits**: Prevent memory attacks
   ```yaml
   max_request_size: 10485760  # 10 MB
   ```

## Performance Considerations

### TLS Performance

1. **Session Resumption**: Reduces handshake overhead
   - ~100x faster than full handshake
   - Enabled by default with LRU cache

2. **Cipher Suite Selection**: Impact on CPU
   - AES-GCM: Fast with AES-NI hardware acceleration
   - ChaCha20-Poly1305: Fast on devices without AES-NI
   - Avoid RSA key exchange (use ECDHE)

3. **HTTP/2**: Multiplexing reduces overhead
   - Single TLS connection for multiple streams
   - Header compression

### Rate Limiting Performance

1. **Token Bucket**: O(1) per request
   - Suitable for high-traffic systems
   - Low memory overhead

2. **Sliding Window**: O(n) per request (n = requests in window)
   - Higher memory usage
   - More accurate for strict limits

3. **Cleanup**: Automatic old entry removal
   - Prevents memory leaks
   - Runs periodically in background

## Troubleshooting

### Common Issues

1. **Certificate Not Found**
   ```
   Error: no certificate found for example.com
   ```
   **Solution**: Check that certificate is loaded and domains match
   ```yaml
   certificates:
     - cert_file: "path/to/cert.pem"
       key_file: "path/to/key.pem"
       domains: ["example.com"]
   ```

2. **TLS Handshake Failure**
   ```
   Error: TLS handshake failed: tls: protocol version not supported
   ```
   **Solution**: Check client and server TLS versions
   ```yaml
   min_version: "1.2"  # Don't set too high
   ```

3. **Rate Limit Too Strict**
   ```
   Many legitimate requests blocked
   ```
   **Solution**: Adjust rate limits or use token bucket with burst
   ```yaml
   requests_per_second: 200.0  # Increase limit
   burst_size: 500             # Allow bigger bursts
   ```

## Examples

See `config/` directory for complete examples:
- `config/tls-example.yaml` - Basic TLS configuration
- `config/tls-multi-domain.yaml` - Multi-domain certificates
- `config/tls-with-security.yaml` - TLS + security features
- `config/tls-mtls.yaml` - Mutual TLS configuration

## Next Steps

Phase 5 will implement:
- Active health checking (TCP, HTTP)
- Passive health checking
- Circuit breaking
- Retry logic
- Backend state management

## Metrics

Phase 4 adds the following metrics:

### TLS Metrics
- `tls_connections_total`: Total TLS connections
- `tls_active_connections`: Active TLS connections
- `tls_handshakes_total`: Total TLS handshakes
- `tls_handshake_failures_total`: Failed handshakes
- `tls_session_resumptions_total`: Resumed sessions
- `tls_handshake_duration_avg`: Average handshake time

### Security Metrics
- `security_rate_limit_requests_total`: Total requests checked
- `security_rate_limit_blocked_total`: Blocked by rate limit
- `security_connections_rejected_total`: Rejected connections
- `security_blocked_ips_total`: Blocked IPs
- `security_request_size_rejected_total`: Oversized requests

## Success Criteria ✅

All Phase 4 goals achieved:

- ✅ TLS termination with configurable versions
- ✅ Multi-domain certificate support
- ✅ SNI-based routing
- ✅ Session resumption for performance
- ✅ Backend TLS connections
- ✅ mTLS support
- ✅ Rate limiting (token bucket & sliding window)
- ✅ Connection flood protection
- ✅ Slowloris mitigation
- ✅ Request size limits
- ✅ IP blocklist management
- ✅ Comprehensive test coverage
- ✅ Production-ready security features

## Acknowledgments

TLS implementation inspired by:
- Go's crypto/tls package
- Envoy's TLS configuration
- NGINX security best practices
- OWASP security guidelines
