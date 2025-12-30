#!/bin/bash
# Dev deployment script for Pulse agents
# Usage: ./scripts/dev-deploy-agent.sh [host1] [host2] ...
# Example: ./scripts/dev-deploy-agent.sh tower pimox minipc

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default hosts if none specified
DEFAULT_HOSTS=("tower")

# Target architecture (most home servers are amd64)
GOARCH="${GOARCH:-amd64}"
GOOS="${GOOS:-linux}"

# SSH options
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10"

# Remote paths
REMOTE_AGENT_PATH="/usr/local/bin/pulse-agent"
REMOTE_SERVICE="pulse-agent"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get list of hosts
if [ $# -eq 0 ]; then
    HOSTS=("${DEFAULT_HOSTS[@]}")
    log_info "No hosts specified, using defaults: ${HOSTS[*]}"
else
    HOSTS=("$@")
fi

# Build the agent
log_info "Building pulse-agent for ${GOOS}/${GOARCH}..."
cd "$PROJECT_ROOT"

BINARY_PATH="$PROJECT_ROOT/bin/pulse-agent-${GOOS}-${GOARCH}"
CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="-s -w" -o "$BINARY_PATH" ./cmd/pulse-agent

if [ ! -f "$BINARY_PATH" ]; then
    log_error "Build failed - binary not found at $BINARY_PATH"
    exit 1
fi

log_success "Built $(du -h "$BINARY_PATH" | cut -f1) binary"

# Deploy to each host
FAILED_HOSTS=()
SUCCESS_HOSTS=()

for host in "${HOSTS[@]}"; do
    echo ""
    log_info "Deploying to $host..."
    
    # Check if host is reachable and get architecture
    HOST_ARCH=$(ssh $SSH_OPTS "$host" "uname -m" 2>/dev/null || echo "unknown")
    if [ "$HOST_ARCH" == "unknown" ]; then
        log_error "Cannot connect to $host or determine architecture - skipping"
        FAILED_HOSTS+=("$host")
        continue
    fi
    
    # Map uname -m to GOARCH
    case $HOST_ARCH in
        x86_64)  TARGET_GOARCH="amd64" ;;
        aarch64) TARGET_GOARCH="arm64" ;;
        armv7l)  TARGET_GOARCH="arm" ;;
        *)       log_error "Unsupported architecture: $HOST_ARCH"; FAILED_HOSTS+=("$host"); continue ;;
    esac

    log_info "  Host architecture: $HOST_ARCH (building for $TARGET_GOARCH)..."
    
    # Build specifically for this host's arch
    BINARY_PATH="$PROJECT_ROOT/bin/pulse-agent-${GOOS}-${TARGET_GOARCH}"
    if [ ! -f "$BINARY_PATH" ] || [ $(( $(date +%s) - $(stat -c %Y "$BINARY_PATH" 2>/dev/null || echo 0) )) -gt 60 ]; then
        log_info "  Building pulse-agent for $TARGET_GOARCH..."
        CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$TARGET_GOARCH" go build -ldflags="-s -w" -o "$BINARY_PATH" ./cmd/pulse-agent
    fi
    
    # Stop the service
    log_info "  Stopping pulse-agent..."
    ssh $SSH_OPTS "$host" "sudo systemctl stop $REMOTE_SERVICE 2>/dev/null || pkill -f pulse-agent || true"
    
    # Copy the binary
    log_info "  Copying binary..."
    if ! scp $SSH_OPTS "$BINARY_PATH" "$host:/tmp/pulse-agent-new"; then
        log_error "  Failed to copy binary to $host"
        FAILED_HOSTS+=("$host")
        continue
    fi
    
    # Install the binary
    log_info "  Installing binary..."
    if ! ssh $SSH_OPTS "$host" "sudo mv /tmp/pulse-agent-new $REMOTE_AGENT_PATH && sudo chmod +x $REMOTE_AGENT_PATH"; then
        log_error "  Failed to install binary on $host"
        FAILED_HOSTS+=("$host")
        continue
    fi
    
    # Start the service
    log_info "  Starting pulse-agent..."
    # Try systemd first, then Unraid go.d script, then manual start via existing scripts
    if ! ssh $SSH_OPTS "$host" "sudo systemctl start $REMOTE_SERVICE 2>/dev/null"; then
        if ssh $SSH_OPTS "$host" "test -f /boot/config/go.d/pulse-agent.sh" 2>/dev/null; then
            log_info "  Using Unraid startup script..."
            ssh $SSH_OPTS "$host" "bash /boot/config/go.d/pulse-agent.sh" >/dev/null 2>&1
        elif ssh $SSH_OPTS "$host" "test -f /etc/init.d/pulse-agent" 2>/dev/null; then
            log_info "  Using init.d script..."
            ssh $SSH_OPTS "$host" "sudo /etc/init.d/pulse-agent start" >/dev/null 2>&1
        fi
    fi
    
    # Verify it's running
    sleep 2
    if ssh $SSH_OPTS "$host" "pgrep -x pulse-agent >/dev/null 2>&1"; then
        log_success "  Agent deployed and running on $host"
        SUCCESS_HOSTS+=("$host")
    else
        # Try one last ditch effort: run it via the background helper if we can find it
        log_warn "  Agent not running, checking logs..."
        ssh $SSH_OPTS "$host" "tail -n 5 /var/log/pulse-agent.log /boot/logs/pulse-agent.log 2>/dev/null" | log_warn
        FAILED_HOSTS+=("$host")
    fi
done

# Summary
echo ""
echo "========================================"
log_info "Deployment Summary"
echo "========================================"

if [ ${#SUCCESS_HOSTS[@]} -gt 0 ]; then
    log_success "Successfully deployed to: ${SUCCESS_HOSTS[*]}"
fi

if [ ${#FAILED_HOSTS[@]} -gt 0 ]; then
    log_error "Failed to deploy to: ${FAILED_HOSTS[*]}"
    exit 1
fi

log_success "All deployments complete!"
