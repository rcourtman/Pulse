# Build stage for frontend (must be built first for embedding)
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend-modern

# Copy package files
COPY frontend-modern/package*.json ./
RUN npm ci

# Copy frontend source
COPY frontend-modern/ ./

# Build frontend
RUN npm run build

# Build stage for Go backend
FROM golang:1.24-alpine AS backend-builder

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

# Copy built frontend from frontend-builder stage for embedding
# Must be at internal/api/frontend-modern for Go embed
COPY --from=frontend-builder /app/frontend-modern/dist ./internal/api/frontend-modern/dist

# Build the binaries with embedded frontend
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o pulse ./cmd/pulse

# Build docker-agent for multiple architectures so users can download any arch
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o pulse-docker-agent-linux-amd64 ./cmd/pulse-docker-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o pulse-docker-agent-linux-arm64 ./cmd/pulse-docker-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o pulse-docker-agent-linux-armv7 ./cmd/pulse-docker-agent

# Keep a host-arch symlink for backward compatibility
RUN cp pulse-docker-agent-linux-amd64 pulse-docker-agent

# Runtime image for the Docker agent (offered via --target agent_runtime)
FROM alpine:latest AS agent_runtime

# Use TARGETARCH to select the correct binary for the build platform
ARG TARGETARCH
ARG TARGETVARIANT

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy all agent binaries first
COPY --from=backend-builder /app/pulse-docker-agent-linux-* /tmp/

# Select the appropriate architecture binary
# Docker buildx automatically sets TARGETARCH (amd64, arm64, arm) and TARGETVARIANT (v7)
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        cp /tmp/pulse-docker-agent-linux-arm64 /usr/local/bin/pulse-docker-agent; \
    elif [ "$TARGETARCH" = "arm" ]; then \
        cp /tmp/pulse-docker-agent-linux-armv7 /usr/local/bin/pulse-docker-agent; \
    else \
        cp /tmp/pulse-docker-agent-linux-amd64 /usr/local/bin/pulse-docker-agent; \
    fi && \
    chmod +x /usr/local/bin/pulse-docker-agent && \
    rm -rf /tmp/pulse-docker-agent-*

COPY --from=backend-builder /app/VERSION /VERSION

ENTRYPOINT ["/usr/local/bin/pulse-docker-agent"]

# Final stage (Pulse server runtime)
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata su-exec

WORKDIR /app

# Copy binaries from builder (frontend is embedded)
COPY --from=backend-builder /app/pulse .
COPY --from=backend-builder /app/pulse-docker-agent .

# Copy VERSION file
COPY --from=backend-builder /app/VERSION .

# Copy entrypoint script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Provide docker-agent installer script for HTTP download endpoint
RUN mkdir -p /opt/pulse/scripts
COPY scripts/install-docker-agent.sh /opt/pulse/scripts/install-docker-agent.sh
RUN chmod 755 /opt/pulse/scripts/install-docker-agent.sh

# Copy multi-arch docker-agent binaries for download endpoint
RUN mkdir -p /opt/pulse/bin
COPY --from=backend-builder /app/pulse-docker-agent-linux-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-armv7 /opt/pulse/bin/

# Create config directory
RUN mkdir -p /etc/pulse /data

# Expose port
EXPOSE 7655

# Set environment variables
# Only PULSE_DATA_DIR is used - all node config is done via web UI
ENV PULSE_DATA_DIR=/data
ENV PULSE_DOCKER=true

# Create default user (will be adjusted by entrypoint if PUID/PGID are set)
RUN adduser -D -u 1000 -g 1000 pulse && \
    chown -R pulse:pulse /app /etc/pulse /data /opt/pulse

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7655 || exit 1

# Use entrypoint script to handle UID/GID
ENTRYPOINT ["/docker-entrypoint.sh"]

# Run the binary
CMD ["./pulse"]
