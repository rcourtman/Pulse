#!/bin/bash

# Toggle script for mock mode
# Automatically restarts backend to apply changes

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
MOCK_ENV_FILE="${ROOT_DIR}/mock.env"
BACKEND_PORT=7656
DEV_DATA_DIR="${ROOT_DIR}/tmp/dev-config"
DEV_KEY_FILE="${DEV_DATA_DIR}/.encryption.key"

# Create default mock.env if it doesn't exist
if [ ! -f "$MOCK_ENV_FILE" ]; then
    cat > "$MOCK_ENV_FILE" << 'EOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=false
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_DOCKER_HOSTS=3
PULSE_MOCK_DOCKER_CONTAINERS=12
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
EOF
fi

restart_backend() {
    echo -e "${YELLOW}Restarting backend...${NC}"

    # Check if running under systemd
    if systemctl is-active --quiet pulse-hot-dev; then
        echo "Detected systemd hot-dev service, restarting it..."
        sudo systemctl restart pulse-hot-dev
        sleep 3
        if systemctl is-active --quiet pulse-hot-dev; then
            echo -e "${GREEN}✓ Hot-dev service restarted successfully${NC}"
            return 0
        else
            echo -e "${RED}✗ Hot-dev service failed to start. Check: sudo journalctl -u pulse-hot-dev -n 50${NC}"
            return 1
        fi
    fi

    # Fallback: manual restart (for standalone hot-dev.sh)
    # Kill existing pulse backend
    pkill -x pulse 2>/dev/null || true
    sleep 1
    pkill -9 -x pulse 2>/dev/null || true

    # Build if needed (rebuild when any Go file is newer than the binary)
    build_needed=false
    if [ ! -f "$ROOT_DIR/pulse" ]; then
        build_needed=true
    else
        changed=$(find "$ROOT_DIR" \
            \( -path "$ROOT_DIR/node_modules" -o -path "$ROOT_DIR/frontend-modern/node_modules" \) -prune \
            -o -name '*.go' -newer "$ROOT_DIR/pulse" -print -quit)
        if [ -n "$changed" ]; then
            build_needed=true
        fi
    fi

    if [ "$build_needed" = true ]; then
        echo "Building backend..."
        cd "$ROOT_DIR"
        go build -o pulse ./cmd/pulse || {
            echo -e "${RED}Failed to build backend${NC}"
            return 1
        }
    fi

    # Start backend with proper environment
    cd "$ROOT_DIR"

    # Load and export all mock env vars
    set -a
    source "$MOCK_ENV_FILE"
    set +a

    # Set data directory based on mock mode
    if [ "$PULSE_MOCK_MODE" = "true" ]; then
        export PULSE_DATA_DIR=/opt/pulse/tmp/mock-data
    else
        mkdir -p "$DEV_DATA_DIR"
        export PULSE_DATA_DIR="$DEV_DATA_DIR"
        if [ ! -f "$DEV_KEY_FILE" ]; then
            openssl rand -hex 32 > "$DEV_KEY_FILE"
            chmod 600 "$DEV_KEY_FILE"
        fi
        if [ -z "${PULSE_ENCRYPTION_KEY:-}" ]; then
            export PULSE_ENCRYPTION_KEY="$(<"$DEV_KEY_FILE")"
        fi
    fi

    export PORT=$BACKEND_PORT

    nohup ./pulse > /tmp/pulse-backend.log 2>&1 &

    sleep 2

    # Check if it started
    if pgrep -x pulse > /dev/null; then
        echo -e "${GREEN}✓ Backend restarted successfully${NC}"
        return 0
    else
        echo -e "${RED}✗ Backend failed to start. Check /tmp/pulse-backend.log${NC}"
        return 1
    fi
}

show_status() {
    source "$MOCK_ENV_FILE"
	if [ "$PULSE_MOCK_MODE" = "true" ]; then
		echo -e "${GREEN}Mock Mode: ENABLED${NC}"
		echo "  Nodes: ${PULSE_MOCK_NODES:-0}"
		echo "  VMs per node: ${PULSE_MOCK_VMS_PER_NODE:-0}"
		echo "  LXCs per node: ${PULSE_MOCK_LXCS_PER_NODE:-0}"
		echo "  Host agents: ${PULSE_MOCK_GENERIC_HOSTS:-0}"
		echo "  Docker hosts: ${PULSE_MOCK_DOCKER_HOSTS:-0}"
		echo "  Docker containers/host: ${PULSE_MOCK_DOCKER_CONTAINERS:-0}"
		echo "  Data dir: ${PULSE_DATA_DIR:-/opt/pulse/tmp/mock-data}"
    else
        echo -e "${BLUE}Mock Mode: DISABLED${NC}"
		echo "  Data dir: ${DEV_DATA_DIR}"
    fi
}

enable_mock() {
    echo -e "${YELLOW}Enabling mock mode...${NC}"

    # Ensure mock data directory exists
    source "$MOCK_ENV_FILE"
    MOCK_DATA_DIR=$(grep PULSE_DATA_DIR "$MOCK_ENV_FILE" | cut -d= -f2)
    if [ -n "$MOCK_DATA_DIR" ] && [ "$MOCK_DATA_DIR" != "/etc/pulse" ]; then
        mkdir -p "$MOCK_DATA_DIR"
        echo "Created mock data directory: $MOCK_DATA_DIR"
    fi

    sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=true/' "$MOCK_ENV_FILE"
    touch "$MOCK_ENV_FILE"
    echo -e "${GREEN}✓ Mock mode enabled!${NC}"
    echo ""
    restart_backend
}

disable_mock() {
    echo -e "${YELLOW}Disabling mock mode...${NC}"

    # Sync production config before switching back
    if [ -f "$ROOT_DIR/scripts/sync-production-config.sh" ]; then
        echo "Syncing production configuration..."
        "$ROOT_DIR/scripts/sync-production-config.sh"
        echo ""
    fi

    sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=false/' "$MOCK_ENV_FILE"
    touch "$MOCK_ENV_FILE"
    echo -e "${GREEN}✓ Mock mode disabled!${NC}"
    echo "Using local dev configuration"
    echo ""
    restart_backend
}

edit_config() {
    echo -e "${YELLOW}Opening mock configuration for editing...${NC}"
    ${EDITOR:-nano} "$MOCK_ENV_FILE"
    touch "$MOCK_ENV_FILE"
    echo ""
    echo -e "${GREEN}Configuration updated!${NC}"
    echo "Run '$0 on' or '$0 off' to apply changes"
}

sync_config() {
    echo -e "${YELLOW}Syncing production configuration to dev...${NC}"

    if [ -f "$ROOT_DIR/scripts/sync-production-config.sh" ]; then
        "$ROOT_DIR/scripts/sync-production-config.sh"
        echo ""
        echo -e "${GREEN}✓ Configuration synced!${NC}"

        # Only restart if backend is currently running
        if pgrep -x pulse > /dev/null; then
            echo ""
            restart_backend
        else
            echo "Backend not running. Start hot-dev to use the updated config."
        fi
    else
        echo -e "${RED}✗ Sync script not found${NC}"
        return 1
    fi
}

case "$1" in
    on)
        enable_mock
        ;;
    off)
        disable_mock
        ;;
    status)
        show_status
        ;;
    edit)
        edit_config
        ;;
    sync)
        sync_config
        ;;
    *)
        echo "Mock Data Mode Control for Pulse"
        echo ""
        echo "Usage: $0 {on|off|status|edit|sync}"
        echo ""
        echo "  on      - Enable mock data mode"
        echo "  off     - Disable mock mode, use real nodes"
        echo "  status  - Show current mock mode status"
        echo "  edit    - Edit mock configuration"
        echo "  sync    - Sync production config to dev (use after adding nodes)"
        echo ""
        echo "After changing modes, restart hot-dev:"
        echo "  Ctrl+C in hot-dev terminal"
        echo "  ./scripts/hot-dev.sh"
        echo ""
        show_status
        ;;
esac
