# Multi-stage build for Balance load balancer
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version information
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o balance \
    ./cmd/balance

# Build the validator tool
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o balance-validate \
    ./cmd/validate

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 balance && \
    adduser -D -u 1000 -G balance balance

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/balance /app/balance
COPY --from=builder /build/balance-validate /app/balance-validate

# Copy example configuration
COPY config/example.yaml /app/config/config.yaml

# Change ownership
RUN chown -R balance:balance /app

# Switch to non-root user
USER balance

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Set entrypoint
ENTRYPOINT ["/app/balance"]
CMD ["-config", "/app/config/config.yaml"]
