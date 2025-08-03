# Build stage for Go backend
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pulse ./cmd/pulse

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

# Copy frontend build
COPY --from=frontend-builder /app/frontend-modern/dist ./frontend-modern/dist

# Copy service files
COPY pulse-backend.service pulse-frontend.service ./

# Create config directory
RUN mkdir -p /etc/pulse /data

# Expose ports
EXPOSE 3000 7655

# Set environment variables
ENV PULSE_CONFIG_DIR=/etc/pulse
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