# Build stage for Go backend
FROM golang:1.23-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy only necessary source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY VERSION ./

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o pulse ./cmd/pulse

# Build stage for frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend-modern

# Copy package files
COPY frontend-modern/package*.json ./
RUN npm ci

# Copy frontend source
COPY frontend-modern/ ./

# Build frontend
RUN npm run build

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/pulse .

# Copy VERSION file
COPY --from=backend-builder /app/VERSION .

# Copy frontend build
COPY --from=frontend-builder /app/frontend-modern/dist ./frontend-modern/dist

# Service files not needed in container

# Create config directory
RUN mkdir -p /etc/pulse /data

# Expose port
EXPOSE 7655

# Set environment variables
# Only PULSE_DATA_DIR is used - all node config is done via web UI
ENV PULSE_DATA_DIR=/data

# Create non-root user
RUN adduser -D -u 1000 pulse && \
    chown -R pulse:pulse /app /etc/pulse /data

USER pulse

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7655 || exit 1

# Run the binary
CMD ["./pulse"]