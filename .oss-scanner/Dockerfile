# Scanner image for https://github.com/rcourtman/Pulse (main branch).
# Pulse is a Go server, agents and helpers, with a Node-built frontend embedded in the server binary.
# While the network is available this fetches every dependency and builds what ships, so the audit
# can rebuild and rerun tests offline. threat_model.md, next to this file, describes how to exercise it.

# Pinned to the governed Go 1.26.11 line matching go.mod, refreshed by Dependabot.
FROM golang:1.26.11-bookworm@sha256:ea47ee45f1284d60f9cc09cb2c3bb4f508c256b640bd8bb30342ff13d1adcb52 AS go-toolchain

# Node 24 is what the frontend stage of Pulse's own Dockerfile uses.
FROM node:24-bookworm@sha256:6b33b0be1647d1a577f9cebb6c8fc2587cc02c98c9956af8d0289df1791a3d10
COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}" \
    CGO_ENABLED=0 \
    NODE_OPTIONS="--max-old-space-size=4096"

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git make openssh-client python3 \
    && rm -rf /var/lib/apt/lists/*

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src
RUN git config --system --add safe.directory /src

# Fetch every dependency the build or the tests need. After the image build there is no network.
RUN go mod download && go version
RUN cd frontend-modern && npm ci

# Build as Pulse's Dockerfile does: the frontend first, because the server embeds it, then the Go binaries
# with the release tag that shipped images use.
RUN cd frontend-modern && npm run build
RUN go build -tags release -buildvcs=false -o /src/bin/ \
      ./cmd/pulse ./cmd/pulse-agent ./cmd/pulse-agent-helper ./cmd/pulse-agent-runner ./cmd/pulse-mcp ./cmd/pulse-control-plane

# Everything is fetched. From here on the go command must not reach a proxy, so the steps below also
# prove the image works offline. The go.mod toolchain stays usable from the module cache.
ENV GOPROXY=off \
    GOSUMDB=off

# Compile the Go tests without running them, so reruns in the audit are quick. Run a few cheap suites so the
# log shows the image works. Neither step may fail the build.
RUN go test -run '^$' ./internal/... ./pkg/... ./cmd/... || echo "WARNING: some Go test packages did not compile"
RUN go test ./internal/securityutil/ ./internal/updatesignature/ ./pkg/auth/ || echo "WARNING: some Go tests failed"
