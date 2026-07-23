# syntax=docker/dockerfile:1.7-labs
ARG BUILD_AGENT=1

# Build stage for frontend (must be built first for embedding)
# Force amd64 platform to avoid slow QEMU emulation during multi-arch builds
FROM --platform=linux/amd64 node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS frontend-builder

WORKDIR /app/frontend-modern

# Copy package files
COPY frontend-modern/package*.json ./
RUN --mount=type=cache,id=pulse-npm-cache,target=/root/.npm \
    npm ci

# Copy frontend source
COPY frontend-modern/ ./
COPY scripts/exclusive-lock.mjs /app/scripts/exclusive-lock.mjs
COPY docs/ /app/docs/
COPY SECURITY.md TERMS.md /app/

# Build frontend
RUN --mount=type=cache,id=pulse-npm-cache,target=/root/.npm \
    npm run build

# Build stage for Go backend
# Force amd64 platform - Go cross-compiles for all targets anyway,
# and this avoids slow QEMU emulation during multi-arch builds
FROM --platform=linux/amd64 golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS backend-builder

ARG BUILD_AGENT
ARG VERSION
ARG PULSE_LICENSE_PUBLIC_KEY_SHA256
ARG PULSE_UPDATE_SIGNING_PUBLIC_KEY
# Go build tags for the server binary. Shipped images use "release", which
# fail-closes mock fixtures, admin bypass, and licensing env overrides. The
# E2E test image builds with GO_BUILD_TAGS="" so the suite can drive mock
# fixtures the same way the local dev harness does.
ARG GO_BUILD_TAGS=release
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git openssh-client

# Copy go mod files for better layer caching
COPY go.mod go.sum ./
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    go mod download

# Copy only necessary source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY scripts/release_ldflags.sh ./scripts/release_ldflags.sh
COPY scripts/release_update_key.go ./scripts/release_update_key.go
COPY scripts/render_installers.go ./scripts/render_installers.go
COPY scripts/install.sh ./scripts/install.sh
COPY scripts/install.ps1 ./scripts/install.ps1
COPY VERSION ./

# Copy the synced embed artifact from the frontend builder stage.
COPY --from=frontend-builder /app/internal/api/frontend-modern/dist ./internal/api/frontend-modern/dist

# Build the main pulse binary for all target architectures
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    --mount=type=secret,id=pulse_license_public_key,required=false \
    --mount=type=secret,id=pulse_update_signing_key,required=false \
    VERSION="${VERSION:-v$(cat VERSION | tr -d '\n')}" && \
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    LICENSE_PUBLIC_KEY="" && \
    EXPECTED_LICENSE_PUBLIC_KEY_SHA256="${PULSE_LICENSE_PUBLIC_KEY_SHA256#SHA256:}" && \
    UPDATE_SIGNING_KEY="" && \
    UPDATE_PUBLIC_KEYS="" && \
    if [ -f /run/secrets/pulse_license_public_key ]; then LICENSE_PUBLIC_KEY="$(tr -d '\r\n' < /run/secrets/pulse_license_public_key)"; fi && \
    if [ -n "${LICENSE_PUBLIC_KEY}" ]; then \
      LICENSE_PUBLIC_KEY_BYTES="$(printf '%s' "${LICENSE_PUBLIC_KEY}" | base64 -d | wc -c | tr -d ' ')" && \
      if [ "${LICENSE_PUBLIC_KEY_BYTES}" != "32" ]; then echo "Error: mounted license public key must decode to 32 bytes." >&2; exit 1; fi; \
    fi && \
    if [ -n "${EXPECTED_LICENSE_PUBLIC_KEY_SHA256}" ]; then \
      if [ -z "${LICENSE_PUBLIC_KEY}" ]; then echo "Error: PULSE_LICENSE_PUBLIC_KEY_SHA256 was provided but no license public key was mounted." >&2; exit 1; fi && \
      ACTUAL_LICENSE_PUBLIC_KEY_SHA256="$(printf '%s' "${LICENSE_PUBLIC_KEY}" | base64 -d | sha256sum | awk '{print $1}')" && \
      if [ "${ACTUAL_LICENSE_PUBLIC_KEY_SHA256}" != "${EXPECTED_LICENSE_PUBLIC_KEY_SHA256}" ]; then echo "Error: mounted license public key does not match PULSE_LICENSE_PUBLIC_KEY_SHA256." >&2; exit 1; fi; \
    fi && \
    if [ -f /run/secrets/pulse_update_signing_key ]; then UPDATE_SIGNING_KEY="$(tr -d '\r\n' < /run/secrets/pulse_update_signing_key)"; fi && \
    if [ -n "${UPDATE_SIGNING_KEY}" ]; then UPDATE_PUBLIC_KEYS="$(go run ./scripts/release_update_key.go public-key --private-key "${UPDATE_SIGNING_KEY}")"; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ -z "${UPDATE_PUBLIC_KEYS}" ]; then echo "Error: PULSE_UPDATE_SIGNING_PUBLIC_KEY was provided but no update signing key was mounted." >&2; exit 1; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ "${UPDATE_PUBLIC_KEYS}" != "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" ]; then echo "Error: mounted update signing key does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY." >&2; echo "Expected public key: ${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" >&2; echo "Actual public key:   ${UPDATE_PUBLIC_KEYS}" >&2; exit 1; fi && \
    SERVER_LDFLAGS="$(./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}" $(if [ -n "${LICENSE_PUBLIC_KEY}" ]; then printf '%s %s' --license-public-key "${LICENSE_PUBLIC_KEY}"; fi) $(if [ -n "${UPDATE_PUBLIC_KEYS}" ]; then printf '%s %s' --update-public-keys "${UPDATE_PUBLIC_KEYS}"; fi))" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -tags "${GO_BUILD_TAGS}" \
      -ldflags="${SERVER_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-linux-amd64 ./cmd/pulse && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -tags "${GO_BUILD_TAGS}" \
      -ldflags="${SERVER_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-linux-arm64 ./cmd/pulse


FROM backend-builder AS release-assets-builder

# Build unified agent binaries for all platforms (for download endpoint)
RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    --mount=type=secret,id=pulse_update_signing_key,required=false \
    VERSION="${VERSION:-v$(cat VERSION | tr -d '\n')}" && \
    UPDATE_SIGNING_KEY="" && \
    UPDATE_PUBLIC_KEYS="" && \
    if [ -f /run/secrets/pulse_update_signing_key ]; then UPDATE_SIGNING_KEY="$(tr -d '\r\n' < /run/secrets/pulse_update_signing_key)"; fi && \
    if [ -n "${UPDATE_SIGNING_KEY}" ]; then UPDATE_PUBLIC_KEYS="$(go run ./scripts/release_update_key.go public-key --private-key "${UPDATE_SIGNING_KEY}")"; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ -z "${UPDATE_PUBLIC_KEYS}" ]; then echo "Error: PULSE_UPDATE_SIGNING_PUBLIC_KEY was provided but no update signing key was mounted." >&2; exit 1; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ "${UPDATE_PUBLIC_KEYS}" != "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" ]; then echo "Error: mounted update signing key does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY." >&2; echo "Expected public key: ${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" >&2; echo "Actual public key:   ${UPDATE_PUBLIC_KEYS}" >&2; exit 1; fi && \
    AGENT_LDFLAGS="$(./scripts/release_ldflags.sh agent --version "${VERSION}" $(if [ -n "${UPDATE_PUBLIC_KEYS}" ]; then printf '%s %s' --update-public-keys "${UPDATE_PUBLIC_KEYS}"; fi))" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-linux-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-linux-arm64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-linux-armv7 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-linux-armv6 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-linux-386 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-darwin-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-darwin-arm64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-windows-amd64.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-windows-arm64.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-windows-386.exe ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-freebsd-amd64 ./cmd/pulse-agent && \
    CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build \
      -ldflags="${AGENT_LDFLAGS}" \
      -buildvcs=false \
      -trimpath \
      -o pulse-agent-freebsd-arm64 ./cmd/pulse-agent

RUN --mount=type=cache,id=pulse-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=pulse-go-build,target=/root/.cache/go-build \
    --mount=type=secret,id=pulse_update_signing_key,required=false \
    mkdir -p /app/rendered-installers && \
    UPDATE_SIGNING_KEY="" && \
    INSTALLER_SSH_PUBLIC_KEY="" && \
    UPDATE_PUBLIC_KEYS="" && \
    if [ -f /run/secrets/pulse_update_signing_key ]; then UPDATE_SIGNING_KEY="$(tr -d '\r\n' < /run/secrets/pulse_update_signing_key)"; fi && \
    if [ -n "${UPDATE_SIGNING_KEY}" ]; then UPDATE_PUBLIC_KEYS="$(go run ./scripts/release_update_key.go public-key --private-key "${UPDATE_SIGNING_KEY}")"; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ -z "${UPDATE_PUBLIC_KEYS}" ]; then echo "Error: PULSE_UPDATE_SIGNING_PUBLIC_KEY was provided but no update signing key was mounted." >&2; exit 1; fi && \
    if [ -n "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ] && [ "${UPDATE_PUBLIC_KEYS}" != "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" ]; then echo "Error: mounted update signing key does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY." >&2; echo "Expected public key: ${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" >&2; echo "Actual public key:   ${UPDATE_PUBLIC_KEYS}" >&2; exit 1; fi && \
    if [ -n "${UPDATE_SIGNING_KEY}" ]; then INSTALLER_SSH_PUBLIC_KEY="$(go run ./scripts/release_update_key.go public-key-ssh --private-key "${UPDATE_SIGNING_KEY}" --comment pulse-installer)"; fi && \
    if [ -n "${INSTALLER_SSH_PUBLIC_KEY}" ]; then \
      go run ./scripts/render_installers.go --source-dir ./scripts --output-dir /app/rendered-installers --installer-ssh-public-key "${INSTALLER_SSH_PUBLIC_KEY}"; \
    else \
      go run ./scripts/render_installers.go --source-dir ./scripts --output-dir /app/rendered-installers --installer-ssh-public-key "" --allow-empty-installer-ssh-public-key; \
    fi && \
    if [ -n "${UPDATE_SIGNING_KEY}" ]; then \
      OPENSSH_SIGNING_KEY=/tmp/pulse-update-signing-key && \
      go run ./scripts/release_update_key.go openssh-private-key --private-key "${UPDATE_SIGNING_KEY}" --comment pulse-installer > "${OPENSSH_SIGNING_KEY}" && \
      chmod 600 "${OPENSSH_SIGNING_KEY}" && \
      for file in /app/pulse-agent-* /app/rendered-installers/install.sh /app/rendered-installers/install.ps1; do \
        ssh-keygen -q -Y sign -f "${OPENSSH_SIGNING_KEY}" -n pulse-install "${file}" >/dev/null && \
        mv "${file}.sig" "${file}.sshsig" && \
        go run ./scripts/release_update_key.go sign --private-key "${UPDATE_SIGNING_KEY}" --file "${file}" > "${file}.sig"; \
      done && \
      rm -f "${OPENSSH_SIGNING_KEY}"; \
    else \
      for file in /app/pulse-agent-* /app/rendered-installers/install.sh /app/rendered-installers/install.ps1; do \
        : > "${file}.sig" && : > "${file}.sshsig"; \
      done; \
    fi


# Runtime image for the Docker agent (offered via --target agent_runtime)
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS agent_runtime

# Use TARGETARCH to select the correct binary for the build platform
ARG TARGETARCH
ARG TARGETVARIANT

RUN apk --no-cache add ca-certificates tzdata && \
    mkdir -p /var/lib/pulse-agent

WORKDIR /app

# Copy all unified agent binaries first
COPY --from=release-assets-builder /app/pulse-agent-linux-* /tmp/

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

COPY --from=release-assets-builder /app/VERSION /VERSION

ENV PULSE_NO_AUTO_UPDATE=true \
    PULSE_DISABLE_AUTO_UPDATE=true \
    PULSE_ENABLE_HOST=true \
    PULSE_ENABLE_DOCKER=true \
    PULSE_AGENT_ID_FILE=/var/lib/pulse-agent/agent-id \
    PULSE_STATE_DIR=/var/lib/pulse-agent

VOLUME ["/var/lib/pulse-agent"]

ENTRYPOINT ["/usr/local/bin/pulse-agent"]

# Base Pulse server runtime shared by self-hosted and hosted tenant images.
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS pulse-runtime-base

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

# Create config directory
RUN mkdir -p /etc/pulse /data

# Expose port
EXPOSE 7655

# Set environment variables
# Only PULSE_DATA_DIR is used - all node config is done via web UI
ENV PULSE_DATA_DIR=/data
ENV PULSE_DOCKER=true
ENV PULSE_DEPLOYMENT_METHOD=container_other

# Create default user (will be adjusted by entrypoint if PUID/PGID are set)
RUN adduser -D -u 1000 -g 1000 pulse && \
    chown -R pulse:pulse /app /etc/pulse /data

# Health check script (handles both HTTP and HTTPS)
COPY docker-healthcheck.sh /docker-healthcheck.sh
RUN chmod +x /docker-healthcheck.sh

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD /docker-healthcheck.sh

# Use entrypoint script to handle UID/GID
ENTRYPOINT ["/docker-entrypoint.sh"]

# Run the binary
CMD ["./pulse"]

# Hosted tenant runtime excludes embedded release installer and agent artifacts.
# Those endpoints can still proxy canonical release assets instead of requiring
# production tenant hotfix builds to carry installer-signing material.
FROM pulse-runtime-base AS hosted_runtime

# Core browser integration tests exercise the running Pulse application and
# mock remote update downloads. They do not serve production installer or agent
# artifacts, so keep their image on the canonical runtime base instead of
# rebuilding the release packet once per Playwright shard.
FROM pulse-runtime-base AS e2e_runtime

# Final stage (Pulse server runtime)
FROM pulse-runtime-base AS runtime

# Provide installer scripts for HTTP download endpoints
RUN mkdir -p /opt/pulse/scripts
COPY scripts/install-container-agent.sh /opt/pulse/scripts/install-container-agent.sh
COPY scripts/install-docker.sh /opt/pulse/scripts/install-docker.sh
COPY --from=release-assets-builder /app/rendered-installers/install.sh /opt/pulse/scripts/install.sh
COPY --from=release-assets-builder /app/rendered-installers/install.sh.sig /opt/pulse/scripts/install.sh.sig
COPY --from=release-assets-builder /app/rendered-installers/install.sh.sshsig /opt/pulse/scripts/install.sh.sshsig
COPY --from=release-assets-builder /app/rendered-installers/install.ps1 /opt/pulse/scripts/install.ps1
COPY --from=release-assets-builder /app/rendered-installers/install.ps1.sig /opt/pulse/scripts/install.ps1.sig
COPY --from=release-assets-builder /app/rendered-installers/install.ps1.sshsig /opt/pulse/scripts/install.ps1.sshsig
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


# Unified agent binaries (all platforms and architectures) plus detached signatures
COPY --from=release-assets-builder /app/pulse-agent-* /opt/pulse/bin/
# Create symlinks for Windows without .exe extension
RUN ln -s pulse-agent-windows-amd64.exe /opt/pulse/bin/pulse-agent-windows-amd64 && \
    ln -s pulse-agent-windows-arm64.exe /opt/pulse/bin/pulse-agent-windows-arm64 && \
    ln -s pulse-agent-windows-386.exe /opt/pulse/bin/pulse-agent-windows-386 && \
    chown -R pulse:pulse /opt/pulse

# Arch-resolved /usr/local/bin/pulse-agent so the helm chart's agent workload
# and `docker run rcourtman/pulse --entrypoint /usr/local/bin/pulse-agent`
# can invoke the right unified-agent binary without juggling arch suffixes.
# The chart's agent.enabled=true previously defaulted to a separate
# ghcr.io/rcourtman/pulse-agent image that is no longer published; pointing it
# at this image plus this symlink restores agent.enabled to a working state.
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        ln -s /opt/pulse/bin/pulse-agent-linux-arm64 /usr/local/bin/pulse-agent; \
    elif [ "$TARGETARCH" = "arm" ]; then \
        ln -s /opt/pulse/bin/pulse-agent-linux-armv7 /usr/local/bin/pulse-agent; \
    else \
        ln -s /opt/pulse/bin/pulse-agent-linux-amd64 /usr/local/bin/pulse-agent; \
    fi
