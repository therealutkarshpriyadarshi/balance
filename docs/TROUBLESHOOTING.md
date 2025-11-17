# Troubleshooting Guide

This guide helps you diagnose and fix common issues with Balance.

## Quick Diagnostics

### Check Service Status

```bash
# Check if Balance is running
ps aux | grep balance

# Check admin API
curl http://localhost:9090/status
curl http://localhost:9090/health

# Check metrics
curl http://localhost:9090/metrics
```

### Check Logs

```bash
# If running as systemd service
journalctl -u balance -f

# If running in Docker
docker logs -f balance

# If running in Kubernetes
kubectl logs -f deployment/balance
```

## Common Issues

### Issue: Balance won't start

#### Symptoms
- Process exits immediately
- "Address already in use" error
- "Permission denied" error

#### Diagnosis
```bash
# Check if port is already in use
sudo netstat -tlnp | grep :8080

# Check configuration
balance-validate -config config.yaml

# Check permissions
ls -l config.yaml
ls -l /path/to/cert.pem
```

#### Solutions

**Port already in use:**
```bash
# Find and kill process using the port
sudo lsof -ti:8080 | xargs kill -9

# Or change the port in config.yaml
listen: ":8081"
```

**Permission denied:**
```bash
# Ports below 1024 require root (don't do this)
# Instead, use a port >= 1024 or capabilities
sudo setcap 'cap_net_bind_service=+ep' /path/to/balance
```

**Invalid configuration:**
```bash
# Validate and see specific errors
balance-validate -config config.yaml -verbose
```

### Issue: Backends not reachable

#### Symptoms
- "502 Bad Gateway" errors
- "Connection refused" in logs
- All requests timing out

#### Diagnosis
```bash
# Check backend health
curl http://localhost:9090/metrics | grep backend_healthy

# Test backend directly
curl http://backend-host:port/

# Check network connectivity
ping backend-host
telnet backend-host port
```

#### Solutions

**Backend down:**
```bash
# Check backend service status
systemctl status backend-service

# Start backend
systemctl start backend-service
```

**Wrong backend address:**
```yaml
# Check config.yaml has correct addresses
backends:
  - name: backend-1
    address: "correct-host:correct-port"
```

**Network/firewall issue:**
```bash
# Check firewall rules
sudo iptables -L
sudo ufw status

# Allow traffic
sudo ufw allow from proxy-ip to any port backend-port
```

### Issue: High latency

#### Symptoms
- Slow response times
- p99 latency > 1 second
- Timeouts under load

#### Diagnosis
```bash
# Check metrics
curl http://localhost:9090/metrics | grep duration

# Check backend latency
time curl http://backend:port/

# Check system resources
top
htop
vmstat 1
```

#### Solutions

**Backend is slow:**
- Investigate backend performance
- Add more backend capacity
- Optimize backend code

**Timeout too aggressive:**
```yaml
timeouts:
  read: 60s  # Increase if backends are legitimately slow
  write: 60s
```

**Resource exhaustion:**
```bash
# Check connection count
curl http://localhost:9090/metrics | grep active_connections

# Check goroutines
curl http://localhost:9090/metrics | grep go_goroutines
```

**Connection pooling not enabled:**
```yaml
http:
  connection_pool:
    enabled: true
    max_idle_per_host: 100
```

### Issue: High error rate

#### Symptoms
- Many 5xx errors
- Circuit breakers opening
- Backends marked unhealthy

#### Diagnosis
```bash
# Check error metrics
curl http://localhost:9090/metrics | grep -E "5.."

# Check backend health
curl http://localhost:9090/metrics | grep backend_healthy

# Check circuit breaker state
curl http://localhost:9090/metrics | grep circuit_breaker
```

#### Solutions

**Backend failures:**
```bash
# Check backend logs
journalctl -u backend-service -f

# Check backend health endpoint
curl http://backend:port/health
```

**Circuit breaker too sensitive:**
```yaml
circuit_breaker:
  failure_threshold: 10  # Increase threshold
  timeout: 120s          # Increase recovery time
```

**Insufficient retries:**
```yaml
retry:
  enabled: true
  max_attempts: 3
  backoff_max: 10s
```

### Issue: Memory leak

#### Symptoms
- Memory usage constantly growing
- OOM killer terminating process
- Goroutine count increasing

#### Diagnosis
```bash
# Check memory metrics
curl http://localhost:9090/metrics | grep go_memstats

# Check goroutine count
curl http://localhost:9090/metrics | grep go_goroutines

# Enable profiling
curl http://localhost:9090/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

#### Solutions

**Connection leak:**
```yaml
timeouts:
  idle: 30s  # Close idle connections sooner
```

**Goroutine leak:**
```bash
# Take goroutine profile
curl http://localhost:9090/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof

# Look for goroutine leaks in profile
```

**Too many metrics:**
```yaml
metrics:
  enabled: true
  # Reduce cardinality by removing high-cardinality labels
```

### Issue: TLS handshake failures

#### Symptoms
- "TLS handshake timeout" errors
- Certificate errors
- Cipher suite mismatch

#### Diagnosis
```bash
# Test TLS connection
openssl s_client -connect localhost:8080 -tls1_2

# Check certificate
openssl x509 -in cert.pem -text -noout

# Check certificate expiry
openssl x509 -in cert.pem -noout -enddate
```

#### Solutions

**Certificate expired:**
```bash
# Renew certificate (example with certbot)
certbot renew
```

**Wrong certificate:**
```yaml
tls:
  cert_file: /correct/path/to/cert.pem
  key_file: /correct/path/to/key.pem
```

**Cipher suite mismatch:**
```yaml
tls:
  min_version: "1.2"
  cipher_suites:
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_RSA_WITH_AES_128_GCM_SHA256
```

### Issue: Load imbalance

#### Symptoms
- One backend receiving more traffic
- Uneven request distribution
- One backend overloaded

#### Diagnosis
```bash
# Check requests per backend
curl http://localhost:9090/metrics | grep backend_requests_total

# Check connections per backend
curl http://localhost:9090/metrics | grep backend_connections
```

#### Solutions

**Wrong algorithm:**
```yaml
load_balancer:
  algorithm: least-connections  # Better for variable request times
```

**Uneven weights:**
```yaml
backends:
  - name: backend-1
    weight: 1  # Ensure weights match capacity
  - name: backend-2
    weight: 1
```

**Session affinity causing imbalance:**
```yaml
# Disable or adjust session affinity
session_affinity:
  enabled: false
```

## Performance Troubleshooting

### CPU Usage High

```bash
# Take CPU profile
curl http://localhost:9090/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Check for hot spots
(pprof) top10
(pprof) list <function>
```

**Solutions:**
- Enable zero-copy transfer (Linux only)
- Increase worker pool size
- Optimize load balancing algorithm

### Connection Exhaustion

```bash
# Check connection limits
ulimit -n

# Increase limits
ulimit -n 65535

# Or in /etc/security/limits.conf
* soft nofile 65535
* hard nofile 65535
```

### DNS Resolution Slow

```bash
# Test DNS resolution
time nslookup backend-host

# Use IP addresses instead
backends:
  - address: "10.0.1.100:8080"
```

## Debug Mode

Enable debug logging for more information:

```yaml
logging:
  level: debug
  format: text
```

Or via environment:

```bash
BALANCE_LOG_LEVEL=debug balance -config config.yaml
```

## Profiling

Enable pprof endpoints:

```bash
# CPU profile
curl http://localhost:9090/debug/pprof/profile?seconds=30 > cpu.prof

# Memory profile
curl http://localhost:9090/debug/pprof/heap > mem.prof

# Goroutine profile
curl http://localhost:9090/debug/pprof/goroutine > goroutine.prof

# Analyze
go tool pprof cpu.prof
```

## Getting Help

If you're still stuck:

1. Check the [documentation](../README.md)
2. Search [GitHub Issues](https://github.com/therealutkarshpriyadarshi/balance/issues)
3. Enable debug logging and collect logs
4. Gather metrics and profiles
5. Create a new issue with:
   - Balance version
   - Configuration (sanitized)
   - Logs showing the problem
   - Steps to reproduce

## Health Check Checklist

- [ ] Balance process is running
- [ ] Configuration is valid
- [ ] All backends are reachable
- [ ] Health checks passing
- [ ] No errors in logs
- [ ] Metrics look normal
- [ ] Response times acceptable
- [ ] No resource exhaustion
- [ ] TLS certificates valid
- [ ] Monitoring and alerting configured
