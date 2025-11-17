# Docker Deployment

This directory contains Docker and Docker Compose configurations for deploying Balance.

## Quick Start

1. Start the complete stack:

```bash
cd deployments/docker
docker-compose up -d
```

2. Access the services:
   - Balance proxy: http://localhost:8080
   - Admin/Health: http://localhost:9090
   - Prometheus: http://localhost:9091
   - Grafana: http://localhost:3000 (admin/admin)

3. Test load balancing:

```bash
# Send multiple requests
for i in {1..10}; do
  curl http://localhost:8080
  echo ""
done
```

4. View metrics:
   - Prometheus metrics: http://localhost:9090/metrics
   - Health check: http://localhost:9090/health
   - Status: http://localhost:9090/status

## Services

### Balance
The main load balancer service proxying traffic to three backend Nginx servers.

### Backends
Three Nginx servers serving different content for easy identification.

### Prometheus
Metrics collection and storage. Scrapes metrics from Balance every 15 seconds.

### Grafana
Visualization dashboard. Pre-configured with Prometheus datasource.

## Configuration

Edit `config.yaml` to modify Balance configuration:
- Change load balancing algorithm
- Adjust timeouts
- Configure health checks
- Enable/disable TLS

## Stopping

```bash
docker-compose down
```

To remove volumes as well:

```bash
docker-compose down -v
```

## Building Custom Image

```bash
# From project root
docker build -t balance:latest .
```

## Production Notes

1. Use proper version tags instead of `latest`
2. Configure appropriate resource limits
3. Set up proper logging (e.g., to ELK stack)
4. Use secrets management for TLS certificates
5. Configure proper network policies
6. Set up backup for metrics data
