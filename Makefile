.PHONY: build run test clean deps lint fmt vet benchmark docker-build

# Binary name
BINARY_NAME=balance

# Version information
VERSION?=dev
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

all: deps fmt vet test build

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) cmd/balance/main.go

# Build with optimizations for production
build-prod:
	@echo "Building $(BINARY_NAME) for production..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o bin/$(BINARY_NAME) cmd/balance/main.go

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME) -config config/example.yaml

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t balance:$(VERSION) .

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/

# Show help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the binary"
	@echo "  make build-prod    - Build optimized production binary"
	@echo "  make run           - Build and run the application"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make benchmark     - Run benchmarks"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make deps          - Install dependencies"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make install       - Install binary to GOPATH/bin"
	@echo "  make help          - Show this help message"
