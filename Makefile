# Pulse Makefile for development

.PHONY: build run dev frontend backend all clean distclean dev-hot lint lint-backend lint-frontend format format-backend format-frontend build-agents

FRONTEND_DIR := frontend-modern
FRONTEND_DIST := $(FRONTEND_DIR)/dist
FRONTEND_EMBED_DIR := internal/api/frontend-modern

# Build everything (including all agent binaries)
all: frontend backend build-agents

# Build frontend only
frontend:
	npm --prefix $(FRONTEND_DIR) run build
	@echo "================================================"
	@echo "Copying frontend to internal/api/ for Go embed"
	@echo "This is REQUIRED - Go cannot embed external paths"
	@echo "================================================"
	rm -rf $(FRONTEND_EMBED_DIR)
	mkdir -p $(FRONTEND_EMBED_DIR)
	cp -r $(FRONTEND_DIST) $(FRONTEND_EMBED_DIR)/
	@echo "✓ Frontend copied for embedding"

# Build backend only (includes embedded frontend)
backend:
	go build -o pulse ./cmd/pulse

# Build both and run
build: frontend backend

# Run the built binary
run: build
	./pulse

# Development - rebuild everything and restart service
dev: frontend backend
	sudo systemctl restart pulse-hot-dev

dev-hot:
	./scripts/hot-dev.sh

# Clean build artifacts
clean:
	rm -f pulse
	rm -rf $(FRONTEND_DIST) $(FRONTEND_EMBED_DIR)

distclean: clean
	./scripts/cleanup.sh

# Quick rebuild and restart for development
restart: frontend backend
	sudo systemctl restart pulse-hot-dev

# Run linters for both backend and frontend
lint: lint-backend lint-frontend

lint-backend:
	golangci-lint run ./...

lint-frontend:
	npm --prefix $(FRONTEND_DIR) run lint

# Apply formatters
format: format-backend format-frontend

format-backend:
	gofmt -w cmd internal pkg

format-frontend:
	npm --prefix $(FRONTEND_DIR) run format

# Build all agent binaries for all platforms
build-agents:
	@echo "Building agent binaries for all platforms..."
	@mkdir -p bin
	@VERSION=$$(cat VERSION | tr -d '\n') && \
	echo "Building docker agent binaries (version: v$$VERSION)..." && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v$$VERSION" -trimpath -o bin/pulse-docker-agent-linux-amd64 ./cmd/pulse-docker-agent && \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v$$VERSION" -trimpath -o bin/pulse-docker-agent-linux-arm64 ./cmd/pulse-docker-agent && \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v$$VERSION" -trimpath -o bin/pulse-docker-agent-linux-armv7 ./cmd/pulse-docker-agent && \
	echo "Building host agent binaries..." && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-linux-amd64 ./cmd/pulse-host-agent && \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-linux-arm64 ./cmd/pulse-host-agent && \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-linux-armv7 ./cmd/pulse-host-agent && \
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-darwin-amd64 ./cmd/pulse-host-agent && \
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-darwin-arm64 ./cmd/pulse-host-agent && \
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o bin/pulse-host-agent-windows-amd64.exe ./cmd/pulse-host-agent
	@ln -sf pulse-host-agent-windows-amd64.exe bin/pulse-host-agent-windows-amd64
	@echo "✓ All agent binaries built in bin/"
