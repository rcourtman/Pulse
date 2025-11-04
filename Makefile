# Pulse Makefile for development

.PHONY: build run dev frontend backend all clean distclean dev-hot lint lint-backend lint-frontend format format-backend format-frontend

FRONTEND_DIR := frontend-modern
FRONTEND_DIST := $(FRONTEND_DIR)/dist
FRONTEND_EMBED_DIR := internal/api/frontend-modern

# Build everything
all: frontend backend

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
