#!/bin/bash

# Toggle script for switching between real and mock mode
# This updates the systemd service to use mock data or real Proxmox nodes

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SERVICE_NAME="pulse-backend"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service.d/mock.conf"
MOCK_ENV_FILE="/opt/pulse/mock.env"

# Create service override directory if it doesn't exist
sudo mkdir -p /etc/systemd/system/${SERVICE_NAME}.service.d/

show_status() {
    if [ -f "$SERVICE_FILE" ]; then
        echo -e "${GREEN}Mock Mode: ENABLED${NC}"
        if [ -f "$MOCK_ENV_FILE" ]; then
            source "$MOCK_ENV_FILE"
            echo "  Nodes: $PULSE_MOCK_NODES"
            echo "  VMs per node: $PULSE_MOCK_VMS_PER_NODE"
            echo "  LXCs per node: $PULSE_MOCK_LXCS_PER_NODE"
        fi
    else
        echo -e "${BLUE}Mock Mode: DISABLED${NC} (using real Proxmox nodes)"
    fi
}

enable_mock() {
    echo -e "${YELLOW}Enabling mock mode...${NC}"
    
    # Create default mock.env if it doesn't exist
    if [ ! -f "$MOCK_ENV_FILE" ]; then
        cat > "$MOCK_ENV_FILE" << 'EOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=true
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
EOF
        echo -e "${GREEN}Created default mock.env${NC}"
    fi
    
    # Create systemd override
    sudo tee "$SERVICE_FILE" > /dev/null << 'EOF'
[Service]
# Mock mode environment variables
Environment="PULSE_MOCK_MODE=true"
EnvironmentFile=-/opt/pulse/mock.env
EOF
    
    # Rebuild with mock support
    echo -e "${YELLOW}Building Pulse with mock support...${NC}"
    cd /opt/pulse
    go build -tags="!production" -o pulse ./cmd/pulse
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
    
    # Reload and restart service
    sudo systemctl daemon-reload
    sudo systemctl restart ${SERVICE_NAME}
    
    echo -e "${GREEN}✓ Mock mode enabled!${NC}"
    echo -e "Access Pulse at: ${GREEN}http://localhost:7655${NC}"
    echo ""
    echo "To adjust mock settings, edit: $MOCK_ENV_FILE"
    echo "Then run: sudo systemctl restart ${SERVICE_NAME}"
}

disable_mock() {
    echo -e "${YELLOW}Disabling mock mode...${NC}"
    
    # Remove systemd override
    sudo rm -f "$SERVICE_FILE"
    
    # Rebuild without mock support (production build)
    echo -e "${YELLOW}Building Pulse for production...${NC}"
    cd /opt/pulse
    go build -o pulse ./cmd/pulse
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
    
    # Reload and restart service
    sudo systemctl daemon-reload
    sudo systemctl restart ${SERVICE_NAME}
    
    echo -e "${GREEN}✓ Mock mode disabled!${NC}"
    echo -e "Now using real Proxmox nodes from your configuration"
}

edit_config() {
    if [ ! -f "$MOCK_ENV_FILE" ]; then
        echo -e "${YELLOW}Creating default mock.env first...${NC}"
        cat > "$MOCK_ENV_FILE" << 'EOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=true
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
EOF
    fi
    
    ${EDITOR:-nano} "$MOCK_ENV_FILE"
    
    if [ -f "$SERVICE_FILE" ]; then
        echo -e "${YELLOW}Restarting service with new configuration...${NC}"
        sudo systemctl restart ${SERVICE_NAME}
        echo -e "${GREEN}✓ Configuration updated and service restarted${NC}"
    else
        echo -e "${YELLOW}Note: Mock mode is not enabled. Run '$0 on' to enable it.${NC}"
    fi
}

case "$1" in
    on|enable)
        enable_mock
        ;;
    off|disable)
        disable_mock
        ;;
    status)
        show_status
        ;;
    edit|config)
        edit_config
        ;;
    *)
        echo "Pulse Mock Mode Toggle"
        echo "====================="
        echo ""
        show_status
        echo ""
        echo "Usage: $0 {on|off|status|edit}"
        echo ""
        echo "  on|enable    - Enable mock mode with simulated data"
        echo "  off|disable  - Disable mock mode (use real Proxmox)"
        echo "  status       - Show current mock mode status"
        echo "  edit|config  - Edit mock configuration"
        echo ""
        echo "Examples:"
        echo "  $0 on        # Enable mock mode with defaults"
        echo "  $0 edit      # Change number of nodes, VMs, etc."
        echo "  $0 off       # Go back to real Proxmox nodes"
        ;;
esac