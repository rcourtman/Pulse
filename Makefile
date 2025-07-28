.PHONY: help build run test clean docker-build docker-run dev lint

# Default target
help:
	@echo "Available targets:"
	@echo "  make build        - Build the Go binary"
	@echo "  make run          - Run the application locally"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Run in Docker container"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo "  make lint         - Run linters"

# Build the binary
build:
	@echo "Building Pulse..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		go build -o bin/pulse ./cmd/pulse

# Run the application
run: build
	@echo "Running Pulse..."
	@./bin/pulse

# Run tests
test:
	@echo "Running tests..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -f Dockerfile.go -t pulse-go:latest .

# Run in Docker
docker-run:
	@echo "Running in Docker..."
	@docker-compose -f docker-compose.go.yml up -d

# Development mode
dev:
	@echo "Starting development mode..."
	@docker-compose -f docker-compose.go.yml --profile dev up pulse-go-dev

# Run linters
lint:
	@echo "Running linters..."
	@docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:latest \
		golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		go mod download

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		go fmt ./...