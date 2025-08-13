# Pulse Makefile for development

.PHONY: build run dev frontend backend all clean

# Build everything
all: frontend backend

# Build frontend only
frontend:
	cd frontend-modern && npm run build
	rm -rf internal/api/frontend-modern/dist
	cp -r frontend-modern/dist internal/api/frontend-modern/

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
	sudo systemctl restart pulse-backend

# Clean build artifacts
clean:
	rm -f pulse
	rm -rf frontend-modern/dist
	rm -rf internal/api/frontend-modern/dist

# Quick rebuild and restart for development
restart: frontend backend
	sudo systemctl restart pulse-backend