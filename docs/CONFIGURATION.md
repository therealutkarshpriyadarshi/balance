# Configuration Guide

This guide covers all configuration options for Balance.

## Configuration File Format

Balance uses YAML for configuration. The default location is `config.yaml` but you can specify a custom path:

```bash
balance -config /path/to/config.yaml
```

## Complete Configuration Example

```yaml
# Proxy mode: "tcp" or "http"
mode: http

# Address to listen on
listen: ":8080"

# Backend servers
backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1
    max_connections: 1000

  - name: backend-2
    address: "localhost:9002"
    weight: 2
    max_connections: 1000

# Load balancing configuration
load_balancer:
  algorithm: round-robin
  # Options: round-robin, least-connections, weighted-round-robin,
  #          weighted-least-connections, consistent-hash, bounded-load

# Timeouts
timeouts:
  connect: 5s
  read: 30s
  write: 30s
  idle: 60s

# TLS configuration
tls:
  enabled: true
  cert_file: /path/to/cert.pem
  key_file: /path/to/key.pem
  min_version: "1.2"
  max_version: "1.3"
  cipher_suites:
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384

# Health check configuration
health_check:
  enabled: true
  interval: 10s
  timeout: 3s
  healthy_threshold: 2
  unhealthy_threshold: 3
  type: http
  http_path: /health
  http_method: GET
  http_expected_status: 200

# Circuit breaker configuration
circuit_breaker:
  enabled: true
  failure_threshold: 5
  success_threshold: 2
  timeout: 60s

# Retry configuration
retry:
  enabled: true
  max_attempts: 3
  backoff_base: 100ms
  backoff_max: 5s

# Admin API configuration
admin:
  enabled: true
  listen: ":9090"

# Metrics configuration
metrics:
  enabled: true

# Logging configuration
logging:
  level: info  # debug, info, warn, error
  format: json # json or text

# HTTP-specific configuration
http:
  # HTTP/2 support
  http2:
    enabled: true

  # WebSocket support
  websocket:
    enabled: true

  # Request/response transformation
  transform:
    # Add headers to requests
    add_request_headers:
      X-Forwarded-For: "{client_ip}"
      X-Forwarded-Proto: "{scheme}"

    # Remove headers from requests
    remove_request_headers:
      - X-Internal-Header

    # Add headers to responses
    add_response_headers:
      X-Proxy: Balance

    # Remove headers from responses
    remove_response_headers:
      - Server

# Rate limiting
rate_limit:
  enabled: true
  requests_per_second: 1000
  burst: 2000

# Security configuration
security:
  # IP blocklist
  blocklist:
    - 192.168.1.100
    - 10.0.0.0/8

  # Connection limits
  max_connections_per_ip: 100

  # Request limits
  max_request_size: 10485760  # 10MB
  max_header_size: 1048576    # 1MB

# Performance tuning
performance:
  # Buffer pool
  buffer_pool_size: 8192

  # Worker pool
  worker_pool_size: 1000

  # Zero-copy transfer (Linux only)
  zero_copy: true
```

## Configuration Sections

### Basic Settings

#### mode
- Type: `string`
- Required: Yes
- Options: `tcp`, `http`
- Description: Proxy mode. TCP for Layer 4, HTTP for Layer 7 proxying.

#### listen
- Type: `string`
- Required: Yes
- Format: `host:port` or `:port`
- Description: Address to listen on for incoming connections.

### Backends

Array of backend servers to proxy to.

#### name
- Type: `string`
- Required: Yes
- Description: Unique identifier for the backend.

#### address
- Type: `string`
- Required: Yes
- Format: `host:port`
- Description: Backend server address.

#### weight
- Type: `integer`
- Required: No
- Default: `1`
- Description: Weight for weighted load balancing algorithms.

#### max_connections
- Type: `integer`
- Required: No
- Default: `0` (unlimited)
- Description: Maximum concurrent connections to this backend.

### Load Balancer

#### algorithm
- Type: `string`
- Required: No
- Default: `round-robin`
- Options:
  - `round-robin`: Simple round-robin selection
  - `least-connections`: Select backend with fewest active connections
  - `weighted-round-robin`: Round-robin with backend weights
  - `weighted-least-connections`: Least connections with backend weights
  - `consistent-hash`: Consistent hashing for session persistence
  - `bounded-load`: Consistent hashing with load protection

### Timeouts

#### connect
- Type: `duration`
- Default: `5s`
- Description: Timeout for establishing backend connections.

#### read
- Type: `duration`
- Default: `30s`
- Description: Timeout for reading from connections.

#### write
- Type: `duration`
- Default: `30s`
- Description: Timeout for writing to connections.

#### idle
- Type: `duration`
- Default: `60s`
- Description: Timeout for idle connections before closing.

### TLS

#### enabled
- Type: `boolean`
- Default: `false`
- Description: Enable TLS termination.

#### cert_file
- Type: `string`
- Required: If TLS enabled
- Description: Path to TLS certificate file (PEM format).

#### key_file
- Type: `string`
- Required: If TLS enabled
- Description: Path to TLS private key file (PEM format).

#### min_version
- Type: `string`
- Default: `1.2`
- Options: `1.0`, `1.1`, `1.2`, `1.3`
- Description: Minimum TLS version to accept.

### Health Check

#### enabled
- Type: `boolean`
- Default: `false`
- Description: Enable active health checking.

#### interval
- Type: `duration`
- Default: `10s`
- Description: Interval between health checks.

#### timeout
- Type: `duration`
- Default: `3s`
- Description: Timeout for each health check.

#### type
- Type: `string`
- Default: `tcp`
- Options: `tcp`, `http`, `https`
- Description: Type of health check to perform.

#### http_path
- Type: `string`
- Default: `/`
- Description: Path for HTTP health checks.

### Circuit Breaker

#### enabled
- Type: `boolean`
- Default: `false`
- Description: Enable circuit breaker for backends.

#### failure_threshold
- Type: `integer`
- Default: `5`
- Description: Failures before opening circuit.

#### timeout
- Type: `duration`
- Default: `60s`
- Description: Time before attempting to close circuit.

### Admin API

#### enabled
- Type: `boolean`
- Default: `false`
- Description: Enable admin HTTP API.

#### listen
- Type: `string`
- Default: `:9090`
- Description: Address for admin API.

Admin endpoints:
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /status` - Service status
- `GET /version` - Version information
- `GET /metrics` - Prometheus metrics

## Environment Variables

You can override configuration with environment variables:

- `BALANCE_CONFIG` - Config file path
- `BALANCE_LISTEN` - Listen address
- `BALANCE_MODE` - Proxy mode
- `BALANCE_LOG_LEVEL` - Log level

## Validation

Validate your configuration before deploying:

```bash
balance-validate -config config.yaml
```

## Hot Reload

Balance supports configuration hot reload:

```bash
kill -HUP $(pidof balance)
```

Or use the admin API:

```bash
curl -X POST http://localhost:9090/reload
```

## Best Practices

1. **Start Simple**: Begin with minimal configuration and add features as needed
2. **Test Configuration**: Always validate before deploying
3. **Monitor First**: Enable metrics and health checks from the start
4. **Tune Timeouts**: Adjust based on your backend response times
5. **Use Version Control**: Track configuration changes in Git
6. **Document Changes**: Comment your configuration files
7. **Backup**: Keep backups of working configurations
8. **Security**: Never commit TLS keys to version control

## Examples

See the `config/` directory for example configurations:
- `config/example.yaml` - Basic HTTP proxy
- `config/tcp-example.yaml` - TCP proxy
- `config/tls-example.yaml` - TLS termination
- `config/advanced-example.yaml` - All features enabled
