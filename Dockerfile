# syntax=docker/dockerfile:1.7-labs
ARG BUILD_AGENT=1
ARG PULSE_LICENSE_PUBLIC_KEY

# Build stage for frontend (must be built first for embedding)
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend-modern

# Copy package files
COPY frontend-modern/package*.json ./
RUN --mount=type=cache,id=pulse-npm-cache,target=/root/.npm \
    npm ci

# Copy frontend source
COPY frontend-modern/ ./

# Build frontend
RUN --mount=type=cache,id=pulse-npm-cache,target=/root/.npm \
    npm run build

# Build stage for Go backend
FROM golang:1.24-alpine AS backend-builder

ARG BUILD_AGENT
ARG PULSE_LICENSE_PUBLIC_KEY
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files for better layer caching
COPY go.mod go.sum ./
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    go mod download

# Copy only necessary source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY VERSION ./

# Copy built frontend from frontend-builder stage for embedding
# Must be at internal/api/frontend-modern for Go embed
COPY --from=frontend-builder /app/frontend-modern/dist ./internal/api/frontend-modern/dist

# Build the binaries with embedded frontend
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="v$(cat VERSION | tr -d '\n')" && \
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    LICENSE_LDFLAGS="" && \
    if [ -n "${PULSE_LICENSE_PUBLIC_KEY}" ]; then \
      LICENSE_LDFLAGS="-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=${PULSE_LICENSE_PUBLIC_KEY}"; \
    fi && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION} ${LICENSE_LDFLAGS}" \
      -trimpath \
      -o pulse ./cmd/pulse

# Build docker-agent binaries (optional cross-arch builds controlled by BUILD_AGENT)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="v$(cat VERSION | tr -d '\n')" && \
    if [ "${BUILD_AGENT:-1}" = "1" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-amd64 ./cmd/pulse-docker-agent && \
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-arm64 ./cmd/pulse-docker-agent && \
      CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-armv7 ./cmd/pulse-docker-agent && \
      CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-armv6 ./cmd/pulse-docker-agent && \
      CGO_ENABLED=0 GOOS=linux GOARCH=386 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-386 ./cmd/pulse-docker-agent; \
    else \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION}" \
        -trimpath \
        -o pulse-docker-agent-linux-amd64 ./cmd/pulse-docker-agent && \
      cp pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-arm64 && \
      cp pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-armv7 && \
      cp pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-armv6 && \
      cp pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-386; \
    fi && \
    cp pulse-docker-agent-linux-amd64 pulse-docker-agent

# Build host-agent binaries for all platforms (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="v$(cat VERSION | tr -d '\n')" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-linux-amd64 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-linux-arm64 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-linux-armv7 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-linux-armv6 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-linux-386 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-darwin-amd64 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-darwin-arm64 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-windows-amd64.exe ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-windows-arm64.exe ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-windows-386.exe ./cmd/pulse-host-agent

# Build unified agent binaries for all platforms (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="v$(cat VERSION | tr -d '\n')" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-linux-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-linux-arm64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-linux-armv7 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-linux-armv6 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-linux-386 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-darwin-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-darwin-arm64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-windows-amd64.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-windows-arm64.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-windows-386.exe ./cmd/pulse-agent

# Build pulse-sensor-proxy for all Linux architectures (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="v$(cat VERSION | tr -d '\n')" && \
    BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S') && \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
      -trimpath \
      -o pulse-sensor-proxy-linux-amd64 ./cmd/pulse-sensor-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
      -trimpath \
      -o pulse-sensor-proxy-linux-arm64 ./cmd/pulse-sensor-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
      -trimpath \
      -o pulse-sensor-proxy-linux-armv7 ./cmd/pulse-sensor-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
      -trimpath \
      -o pulse-sensor-proxy-linux-armv6 ./cmd/pulse-sensor-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
      -trimpath \
      -o pulse-sensor-proxy-linux-386 ./cmd/pulse-sensor-proxy && \
    cp pulse-sensor-proxy-linux-amd64 pulse-sensor-proxy

# Runtime image for the Docker agent (offered via --target agent_runtime)
FROM alpine:3.20 AS agent_runtime

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

ENV PULSE_NO_AUTO_UPDATE=true

ENTRYPOINT ["/usr/local/bin/pulse-docker-agent"]

# Final stage (Pulse server runtime)
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates tzdata su-exec openssh-client

WORKDIR /app

# Copy binaries from builder (frontend is embedded)
COPY --from=backend-builder /app/pulse .
COPY --from=backend-builder /app/pulse-docker-agent .

# Copy VERSION file
COPY --from=backend-builder /app/VERSION .

# Copy entrypoint script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Provide installer scripts for HTTP download endpoints
RUN mkdir -p /opt/pulse/scripts
COPY scripts/install-docker-agent.sh /opt/pulse/scripts/install-docker-agent.sh
COPY scripts/install-container-agent.sh /opt/pulse/scripts/install-container-agent.sh
COPY scripts/install-host-agent.ps1 /opt/pulse/scripts/install-host-agent.ps1
COPY scripts/uninstall-host-agent.sh /opt/pulse/scripts/uninstall-host-agent.sh
COPY scripts/uninstall-host-agent.ps1 /opt/pulse/scripts/uninstall-host-agent.ps1
COPY scripts/install-sensor-proxy.sh /opt/pulse/scripts/install-sensor-proxy.sh
COPY scripts/install-docker.sh /opt/pulse/scripts/install-docker.sh
COPY scripts/install.sh /opt/pulse/scripts/install.sh
COPY scripts/install.ps1 /opt/pulse/scripts/install.ps1
RUN chmod 755 /opt/pulse/scripts/*.sh /opt/pulse/scripts/*.ps1

# Copy all binaries for download endpoint
RUN mkdir -p /opt/pulse/bin

# Main pulse server binary (for validation)
COPY --from=backend-builder /app/pulse /opt/pulse/bin/pulse

# Docker agent binaries (all architectures)
COPY --from=backend-builder /app/pulse-docker-agent-linux-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-armv7 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-armv6 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent-linux-386 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-docker-agent /opt/pulse/bin/pulse-docker-agent

# Host agent binaries (all platforms and architectures)
COPY --from=backend-builder /app/pulse-host-agent-linux-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-linux-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-linux-armv7 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-linux-armv6 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-linux-386 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-darwin-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-darwin-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-windows-amd64.exe /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-windows-arm64.exe /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-windows-386.exe /opt/pulse/bin/
# Create symlinks for Windows without .exe extension
RUN ln -s pulse-host-agent-windows-amd64.exe /opt/pulse/bin/pulse-host-agent-windows-amd64 && \
    ln -s pulse-host-agent-windows-arm64.exe /opt/pulse/bin/pulse-host-agent-windows-arm64 && \
    ln -s pulse-host-agent-windows-386.exe /opt/pulse/bin/pulse-host-agent-windows-386

# Unified agent binaries (all platforms and architectures)
COPY --from=backend-builder /app/pulse-agent-linux-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-linux-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-linux-armv7 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-linux-armv6 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-linux-386 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-darwin-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-darwin-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-windows-amd64.exe /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-windows-arm64.exe /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-windows-386.exe /opt/pulse/bin/
# Create symlinks for Windows without .exe extension
RUN ln -s pulse-agent-windows-amd64.exe /opt/pulse/bin/pulse-agent-windows-amd64 && \
    ln -s pulse-agent-windows-arm64.exe /opt/pulse/bin/pulse-agent-windows-arm64 && \
    ln -s pulse-agent-windows-386.exe /opt/pulse/bin/pulse-agent-windows-386

# Sensor proxy binaries (all Linux architectures)
COPY --from=backend-builder /app/pulse-sensor-proxy-linux-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-sensor-proxy-linux-arm64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-sensor-proxy-linux-armv7 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-sensor-proxy-linux-armv6 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-sensor-proxy-linux-386 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-sensor-proxy /opt/pulse/bin/pulse-sensor-proxy

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

# Health check script (handles both HTTP and HTTPS)
COPY docker-healthcheck.sh /docker-healthcheck.sh
RUN chmod +x /docker-healthcheck.sh

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD /docker-healthcheck.sh

# Use entrypoint script to handle UID/GID
ENTRYPOINT ["/docker-entrypoint.sh"]

# Run the binary
CMD ["./pulse"]
