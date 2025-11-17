# Monitoring Setup

This directory contains monitoring configuration for Balance including Grafana dashboards and Prometheus alerting rules.

## Components

### Grafana Dashboard
- **grafana-dashboard.json**: Comprehensive dashboard showing:
  - Request rate and throughput
  - Response time (p95, p99)
  - Error rates
  - Active connections
  - Backend health status
  - Resource usage (memory, goroutines)

### Prometheus Alerts
- **prometheus-alerts.yaml**: Production-ready alerting rules:
  - High error rate (>5%)
  - High latency (p99 >1s)
  - Backend health issues
  - Connection saturation
  - Memory pressure
  - Circuit breaker states
  - Service availability

## Setup

### Grafana Dashboard

1. **Import via UI**:
   - Open Grafana UI
   - Go to Dashboards → Import
   - Upload `grafana-dashboard.json`
   - Select Prometheus datasource
   - Click Import

2. **Provisioning** (for automated setup):
   ```yaml
   # Add to Grafana provisioning config
   apiVersion: 1
   providers:
     - name: 'Balance'
       orgId: 1
       folder: 'Load Balancers'
       type: file
       options:
         path: /var/lib/grafana/dashboards
   ```

### Prometheus Alerts

1. **Add to Prometheus config**:
   ```yaml
   # prometheus.yml
   rule_files:
     - 'alerts/*.yaml'

   alerting:
     alertmanagers:
       - static_configs:
           - targets:
               - alertmanager:9093
   ```

2. **Reload Prometheus**:
   ```bash
   curl -X POST http://localhost:9090/-/reload
   ```

3. **Verify rules**:
   ```bash
   # Check rules are loaded
   curl http://localhost:9090/api/v1/rules
   ```

## Metrics Reference

### HTTP Metrics
- `balance_http_requests_total`: Total HTTP requests (counter)
- `balance_http_request_duration_seconds`: Request duration histogram
- `balance_http_request_size_bytes`: Request size histogram
- `balance_http_response_size_bytes`: Response size histogram

### Backend Metrics
- `balance_backend_healthy`: Backend health status (1=healthy, 0=unhealthy)
- `balance_backend_connections`: Active connections per backend
- `balance_backend_requests_total`: Total requests per backend
- `balance_backend_retries_total`: Retry attempts per backend

### Connection Metrics
- `balance_active_connections`: Currently active connections
- `balance_connections_total`: Total connections handled (counter)

### Circuit Breaker Metrics
- `balance_circuit_breaker_state`: Circuit breaker state (closed/open/half-open)
- `balance_circuit_breaker_failures_total`: Total failures

### System Metrics
- `go_goroutines`: Number of goroutines
- `go_memstats_alloc_bytes`: Allocated memory
- `go_memstats_sys_bytes`: System memory

## Alert Severity Levels

### Critical (Page immediately)
- **BalanceAllBackendsDown**: No backends available
- **BalanceDown**: Service completely unavailable
- **BalanceHighErrorRate**: >5% error rate

### Warning (Investigate soon)
- **BalanceBackendDown**: Single backend unhealthy
- **BalanceHighLatency**: P99 >1s
- **BalanceHighConnections**: Approaching capacity
- **BalanceHighMemoryUsage**: Memory pressure
- **BalanceCircuitBreakerOpen**: Circuit breaker triggered
- **BalanceHighRetryRate**: Many retries occurring

## Alert Configuration

### Slack Integration
```yaml
# alertmanager.yml
receivers:
  - name: 'slack'
    slack_configs:
      - api_url: 'YOUR_WEBHOOK_URL'
        channel: '#alerts'
        title: 'Balance Alert'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

### PagerDuty Integration
```yaml
receivers:
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_SERVICE_KEY'
        description: '{{ .GroupLabels.alertname }}'
```

### Email Integration
```yaml
receivers:
  - name: 'email'
    email_configs:
      - to: 'oncall@example.com'
        from: 'alertmanager@example.com'
        smarthost: 'smtp.example.com:587'
```

## Dashboard Panels

### Request Rate
Shows requests per second over time, broken down by HTTP method and status code.

### Response Time
Displays p95 and p99 latency percentiles to identify performance degradation.

### Error Rate
Tracks 4xx and 5xx errors to detect service issues.

### Active Connections
Monitors current connection count to identify capacity issues.

### Backend Health
Real-time status of each backend server (green=healthy, red=unhealthy).

### Resource Usage
Tracks goroutines and memory to identify resource leaks.

## Troubleshooting

### No data in Grafana
1. Verify Prometheus is scraping Balance:
   ```bash
   curl http://localhost:9090/api/v1/targets
   ```
2. Check Balance metrics endpoint:
   ```bash
   curl http://localhost:9090/metrics
   ```

### Alerts not firing
1. Check alert rules are loaded:
   ```bash
   curl http://localhost:9090/api/v1/rules
   ```
2. Verify Alertmanager is configured:
   ```bash
   curl http://localhost:9090/api/v1/alertmanagers
   ```

### Missing metrics
1. Ensure metrics are enabled in Balance config
2. Verify Prometheus scrape interval matches data retention
3. Check for label mismatches in queries

## Best Practices

1. **Alert Tuning**: Adjust thresholds based on your workload
2. **Runbooks**: Document response procedures for each alert
3. **SLOs**: Set Service Level Objectives and monitor against them
4. **Regular Reviews**: Review and update alerts quarterly
5. **Testing**: Regularly test alert channels and on-call procedures
