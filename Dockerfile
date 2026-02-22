# syntax=docker/dockerfile:1.7-labs
ARG BUILD_AGENT=1
ARG PULSE_LICENSE_PUBLIC_KEY

# Build stage for frontend (must be built first for embedding)
# Force amd64 platform to avoid slow QEMU emulation during multi-arch builds
FROM --platform=linux/amd64 node:20-alpine AS frontend-builder

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
# Force amd64 platform - Go cross-compiles for all targets anyway,
# and this avoids slow QEMU emulation during multi-arch builds
FROM --platform=linux/amd64 golang:1.25.7-alpine AS backend-builder

ARG BUILD_AGENT
ARG PULSE_LICENSE_PUBLIC_KEY
ARG VERSION
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

# Build the main pulse binary for all target architectures
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="${VERSION:-v$(cat VERSION | tr -d '\n')}" && \
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    LICENSE_LDFLAGS="" && \
    if [ -n "${PULSE_LICENSE_PUBLIC_KEY}" ]; then \
      LICENSE_LDFLAGS="-X github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey=${PULSE_LICENSE_PUBLIC_KEY} -X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=${PULSE_LICENSE_PUBLIC_KEY}"; \
    fi && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION} ${LICENSE_LDFLAGS}" \
      -trimpath \
      -o pulse-linux-amd64 ./cmd/pulse && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${VERSION} ${LICENSE_LDFLAGS}" \
      -trimpath \
      -o pulse-linux-arm64 ./cmd/pulse



# Build host-agent binaries for all platforms (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="${VERSION:-v$(cat VERSION | tr -d '\n')}" && \
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
      -o pulse-host-agent-windows-386.exe ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-freebsd-amd64 ./cmd/pulse-host-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build \
      -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=${VERSION}" \
      -trimpath \
      -o pulse-host-agent-freebsd-arm64 ./cmd/pulse-host-agent

# Build unified agent binaries for all platforms (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    VERSION="${VERSION:-v$(cat VERSION | tr -d '\n')}" && \
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
      -o pulse-agent-windows-386.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-freebsd-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -trimpath \
      -o pulse-agent-freebsd-arm64 ./cmd/pulse-agent


# Runtime image for the Docker agent (offered via --target agent_runtime)
FROM alpine:3.20 AS agent_runtime

# Use TARGETARCH to select the correct binary for the build platform
ARG TARGETARCH
ARG TARGETVARIANT

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy all unified agent binaries first
COPY --from=backend-builder /app/pulse-agent-linux-* /tmp/

# Select the appropriate architecture binary
# Docker buildx automatically sets TARGETARCH (amd64, arm64, arm) and TARGETVARIANT (v7)
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        cp /tmp/pulse-agent-linux-arm64 /usr/local/bin/pulse-agent; \
    elif [ "$TARGETARCH" = "arm" ]; then \
        cp /tmp/pulse-agent-linux-armv7 /usr/local/bin/pulse-agent; \
    else \
        cp /tmp/pulse-agent-linux-amd64 /usr/local/bin/pulse-agent; \
    fi && \
    chmod +x /usr/local/bin/pulse-agent && \
    rm -rf /tmp/pulse-agent-*

# Create shim for pulse-docker-agent to maintain backward compatibility
RUN echo '#!/bin/sh' > /usr/local/bin/pulse-docker-agent && \
    echo 'exec /usr/local/bin/pulse-agent --enable-docker "$@"' >> /usr/local/bin/pulse-docker-agent && \
    chmod +x /usr/local/bin/pulse-docker-agent

COPY --from=backend-builder /app/VERSION /VERSION

ENV PULSE_NO_AUTO_UPDATE=true

ENTRYPOINT ["/usr/local/bin/pulse-docker-agent"]

# Final stage (Pulse server runtime)
FROM alpine:3.20 AS runtime

# Use TARGETARCH to select the correct binary for the build platform
ARG TARGETARCH

RUN apk --no-cache add ca-certificates tzdata su-exec openssh-client

WORKDIR /app

# Copy the correct pulse binary for target architecture directly
# Use separate COPY commands with TARGETARCH to avoid copying both binaries
# (copying to /tmp then deleting wastes space due to Docker layer immutability)
COPY --from=backend-builder /app/pulse-linux-${TARGETARCH:-amd64} ./pulse
RUN chmod +x ./pulse



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
COPY scripts/install-docker.sh /opt/pulse/scripts/install-docker.sh
COPY scripts/install.sh /opt/pulse/scripts/install.sh
COPY scripts/install.ps1 /opt/pulse/scripts/install.ps1
RUN chmod 755 /opt/pulse/scripts/*.sh /opt/pulse/scripts/*.ps1

# Copy all binaries for download endpoint
RUN mkdir -p /opt/pulse/bin

# Main pulse server binary (for validation) - copy both architectures
COPY --from=backend-builder /app/pulse-linux-amd64 /opt/pulse/bin/pulse-linux-amd64
COPY --from=backend-builder /app/pulse-linux-arm64 /opt/pulse/bin/pulse-linux-arm64
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        ln -s pulse-linux-arm64 /opt/pulse/bin/pulse; \
    else \
        ln -s pulse-linux-amd64 /opt/pulse/bin/pulse; \
    fi


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
COPY --from=backend-builder /app/pulse-host-agent-freebsd-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-host-agent-freebsd-arm64 /opt/pulse/bin/
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
COPY --from=backend-builder /app/pulse-agent-freebsd-amd64 /opt/pulse/bin/
COPY --from=backend-builder /app/pulse-agent-freebsd-arm64 /opt/pulse/bin/
# Create symlinks for Windows without .exe extension
RUN ln -s pulse-agent-windows-amd64.exe /opt/pulse/bin/pulse-agent-windows-amd64 && \
    ln -s pulse-agent-windows-arm64.exe /opt/pulse/bin/pulse-agent-windows-arm64 && \
    ln -s pulse-agent-windows-386.exe /opt/pulse/bin/pulse-agent-windows-386

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
