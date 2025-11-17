# Getting Started with Balance

This guide will help you get Balance up and running quickly.

## Prerequisites

- Go 1.22 or higher
- Basic understanding of networking and load balancers

## Quick Start (5 minutes)

### 1. Install Dependencies

```bash
go mod download
```

### 2. Start Test Backend Servers

Open 3 terminal windows and run these commands to simulate backend servers:

**Terminal 1:**
```bash
# Simple echo server on port 9001
while true; do echo -e "HTTP/1.1 200 OK\n\nBackend 1" | nc -l 9001; done
```

**Terminal 2:**
```bash
# Simple echo server on port 9002
while true; do echo -e "HTTP/1.1 200 OK\n\nBackend 2" | nc -l 9002; done
```

**Terminal 3:**
```bash
# Simple echo server on port 9003
while true; do echo -e "HTTP/1.1 200 OK\n\nBackend 3" | nc -l 9003; done
```

Or use Go's built-in HTTP server (better option):

**Terminal 1:**
```bash
go run -e 'package main; import ("fmt"; "net/http"); func main() { http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Backend 1\n") }); http.ListenAndServe(":9001", nil) }'
```

Or create a simple test script `scripts/test-backend.sh`:

```bash
#!/bin/bash
PORT=${1:-9001}
NAME=${2:-"Backend"}

echo "Starting $NAME on port $PORT"
while true; do
    echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n$NAME on port $PORT" | nc -l -p $PORT
done
```

### 3. Build Balance

```bash
make build
```

### 4. Run Balance

```bash
./bin/balance -config config/example.yaml
```

You should see output like:
```
Starting Balance proxy (version: dev)
Loaded configuration from: config/example.yaml
Proxy listening on :8080 (mode: tcp)
```

### 5. Test the Proxy

In a new terminal, send requests:

```bash
# Using curl
curl http://localhost:8080

# Using telnet
telnet localhost 8080

# Using wrk for load testing
wrk -t4 -c100 -d30s http://localhost:8080
```

You should see responses being load-balanced across the three backends!

## Configuration

### Basic TCP Proxy Configuration

```yaml
mode: tcp
listen: ":8080"

backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1
```

### Change Load Balancing Algorithm

Edit `config/example.yaml`:

```yaml
load_balancer:
  algorithm: least-connections  # or "round-robin"
```

### Add More Backends

```yaml
backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1

  - name: backend-2
    address: "localhost:9002"
    weight: 2  # 2x traffic
```

## Current Features (Phase 1)

✅ Basic TCP proxying
✅ Multiple backends
✅ Round-robin load balancing
✅ Least-connections load balancing
✅ Configurable timeouts
✅ Connection statistics
✅ Graceful shutdown

## Coming Soon

⏳ HTTP/HTTPS proxy (Phase 3)
⏳ TLS termination (Phase 4)
⏳ Health checks (Phase 5)
⏳ Circuit breaking (Phase 5)
⏳ Connection pooling (Phase 6)
⏳ Metrics endpoint (Phase 6)

## Development Workflow

### Run Tests
```bash
make test
```

### Format Code
```bash
make fmt
```

### Run Linter
```bash
make lint
```

### Build Production Binary
```bash
make build-prod
```

## Testing the Load Balancer

### Test Round-Robin

```bash
# Send 10 requests and observe the distribution
for i in {1..10}; do
  curl -s http://localhost:8080
  echo ""
done
```

You should see responses alternating between backends.

### Test Least-Connections

Change config to `algorithm: least-connections` and restart.

Open multiple connections simultaneously:

```bash
# Terminal 1
while true; do curl http://localhost:8080; sleep 0.1; done

# Terminal 2
while true; do curl http://localhost:8080; sleep 0.1; done
```

The load balancer will route to the backend with fewer active connections.

## Troubleshooting

### "Failed to connect to backend"

- Ensure your backend servers are running
- Check the backend addresses in config
- Verify ports are not blocked by firewall

### "Address already in use"

- Another process is using port 8080
- Change the `listen` address in config
- Or kill the process: `lsof -ti:8080 | xargs kill`

### "No healthy backend available"

- All backends are down or unreachable
- Check backend server logs
- Verify network connectivity

## Performance Testing

### Using wrk (HTTP load testing)

```bash
# Install wrk
brew install wrk  # macOS
sudo apt install wrk  # Ubuntu

# Run load test
wrk -t4 -c100 -d30s http://localhost:8080

# Results will show:
# - Requests/sec
# - Latency distribution
# - Transfer rate
```

### Using hey (Alternative to wrk)

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Run load test
hey -n 10000 -c 100 http://localhost:8080
```

## Next Steps

1. **Read the Roadmap**: See [ROADMAP.md](ROADMAP.md) for the full project plan
2. **Explore Code**: Start with `cmd/balance/main.go` and trace through
3. **Add Features**: Pick a task from Phase 2 and implement it
4. **Write Tests**: Add unit tests for the components
5. **Optimize**: Profile and optimize performance

## Useful Commands

```bash
# Show version
./bin/balance -version

# Validate config without starting
go run cmd/balance/main.go -config config/example.yaml -validate

# Run with custom config
./bin/balance -config my-config.yaml

# View logs with timestamps
./bin/balance -config config/example.yaml 2>&1 | ts

# Monitor connections (Linux)
watch -n1 'netstat -an | grep 8080'
```

## Resources

- [ROADMAP.md](ROADMAP.md) - Full implementation roadmap
- [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md) - Project vision and architecture
- [Go net package](https://pkg.go.dev/net) - Standard library documentation
- [Load Balancing Algorithms](https://www.nginx.com/blog/choosing-nginx-plus-load-balancing-techniques/)

## Getting Help

- Check existing issues on GitHub
- Read the documentation
- Look at example configurations
- Review the code comments

---

**Happy Load Balancing! 🚀**
