#!/bin/bash

# Simple toggle script for mock mode
# Just updates mock.env and restarts the dev service

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

MOCK_ENV_FILE="/opt/pulse/mock.env"

# Create default mock.env if it doesn't exist
if [ ! -f "$MOCK_ENV_FILE" ]; then
    cat > "$MOCK_ENV_FILE" << 'EOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=false
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
EOF
fi

show_status() {
    source "$MOCK_ENV_FILE"
    if [ "$PULSE_MOCK_MODE" = "true" ]; then
        echo -e "${GREEN}Mock Mode: ENABLED${NC}"
        echo "  Nodes: $PULSE_MOCK_NODES"
        echo "  VMs per node: $PULSE_MOCK_VMS_PER_NODE"
        echo "  LXCs per node: $PULSE_MOCK_LXCS_PER_NODE"
    else
        echo -e "${BLUE}Mock Mode: DISABLED${NC} (using real Proxmox nodes)"
    fi
}

enable_mock() {
    echo -e "${YELLOW}Enabling mock mode...${NC}"

    # Update mock.env to enable mock mode
    sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=true/' "$MOCK_ENV_FILE"

    # Touch the file to trigger file watcher (for hot-dev mode)
    touch "$MOCK_ENV_FILE"

    # If pulse-dev systemd service is running, restart it
    if systemctl is-active --quiet pulse-dev 2>/dev/null; then
        sudo systemctl restart pulse-dev
        echo -e "${GREEN}✓ Mock mode enabled! (systemd service restarted)${NC}"
    else
        echo -e "${GREEN}✓ Mock mode enabled!${NC}"
        echo -e "${BLUE}Note: Backend will auto-reload if running in hot-dev mode${NC}"
    fi

    echo ""
    echo "To adjust mock settings, edit: $MOCK_ENV_FILE"
}

disable_mock() {
    echo -e "${YELLOW}Disabling mock mode...${NC}"

    # Sync production config before switching back
    if [ -f "/opt/pulse/scripts/sync-production-config.sh" ]; then
        echo "Syncing production configuration..."
        /opt/pulse/scripts/sync-production-config.sh
        echo ""
    fi

    # Update mock.env to disable mock mode
    sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=false/' "$MOCK_ENV_FILE"

    # Touch the file to trigger file watcher (for hot-dev mode)
    touch "$MOCK_ENV_FILE"

    # If pulse-dev systemd service is running, restart it
    if systemctl is-active --quiet pulse-dev 2>/dev/null; then
        sudo systemctl restart pulse-dev
        echo -e "${GREEN}✓ Mock mode disabled! (systemd service restarted)${NC}"
    else
        echo -e "${GREEN}✓ Mock mode disabled!${NC}"
        echo -e "${BLUE}Note: Backend will auto-reload if running in hot-dev mode${NC}"
    fi

    echo "Using real Proxmox nodes"
}

edit_config() {
    echo -e "${YELLOW}Opening mock configuration for editing...${NC}"
    ${EDITOR:-nano} "$MOCK_ENV_FILE"

    # Touch the file to trigger file watcher (for hot-dev mode)
    touch "$MOCK_ENV_FILE"

    echo ""
    echo -e "${GREEN}Configuration updated!${NC}"

    # If pulse-dev systemd service is running, suggest restart
    if systemctl is-active --quiet pulse-dev 2>/dev/null; then
        echo "Run 'sudo systemctl restart pulse-dev' to apply changes"
    else
        echo -e "${BLUE}Note: Backend will auto-reload if running in hot-dev mode${NC}"
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
    *)
        echo "Mock Data Mode Control for Pulse"
        echo ""
        echo "Usage: $0 {on|off|status|edit}"
        echo ""
        echo "  on     - Enable mock data mode"
        echo "  off    - Disable mock mode, use real nodes"
        echo "  status - Show current mock mode status"
        echo "  edit   - Edit mock configuration"
        echo ""
        show_status
        ;;
esac