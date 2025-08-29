#!/bin/bash

# Pure mock mode toggle - completely disables real nodes when mock is enabled
# This ensures no mixing of real and mock data

MODE="${1:-status}"
MOCK_ENV="/opt/pulse/mock.env"
SERVICE_DIR="/etc/systemd/system/pulse-backend.service.d"
MOCK_CONF="$SERVICE_DIR/mock.conf"

# Colors for output
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
RESET='\033[0m'

case "$MODE" in
    on)
        echo -e "${YELLOW}Enabling PURE mock mode (disabling all real nodes)...${RESET}"
        
        # Create the service override directory if it doesn't exist
        sudo mkdir -p "$SERVICE_DIR"
        
        # Create override to set mock mode and disable real nodes
        sudo tee "$MOCK_CONF" > /dev/null <<EOF
[Service]
Environment="PULSE_MOCK_MODE=true"
Environment="PULSE_DISABLE_REAL_NODES=true"
EnvironmentFile=-/opt/pulse/mock.env
EOF
        
        # Ensure mock.env exists with good defaults
        if [ ! -f "$MOCK_ENV" ]; then
            cat > "$MOCK_ENV" <<'EOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=true
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
EOF
        else
            # Ensure mock mode is enabled in the file
            sed -i 's/PULSE_MOCK_MODE=false/PULSE_MOCK_MODE=true/' "$MOCK_ENV"
        fi
        
        # Build with mock support
        echo -e "${YELLOW}Building Pulse with mock support...${RESET}"
        cd /opt/pulse
        go build -tags "mock" -o bin/pulse ./cmd/pulse
        
        # Reload and restart
        sudo systemctl daemon-reload
        sudo systemctl restart pulse-backend
        
        echo -e "${GREEN}✓ PURE mock mode enabled!${RESET}"
        echo -e "Real nodes are completely disabled."
        echo -e "Access Pulse at: ${GREEN}http://localhost:7655${RESET}"
        ;;
        
    off)
        echo -e "${YELLOW}Disabling mock mode (restoring real nodes)...${RESET}"
        
        # Remove the mock override
        sudo rm -f "$MOCK_CONF"
        
        # Update mock.env to disable mock mode
        if [ -f "$MOCK_ENV" ]; then
            sed -i 's/PULSE_MOCK_MODE=true/PULSE_MOCK_MODE=false/' "$MOCK_ENV"
        fi
        
        # Build without mock support for production
        echo -e "${YELLOW}Building Pulse for production...${RESET}"
        cd /opt/pulse
        go build -o bin/pulse ./cmd/pulse
        
        # Reload and restart
        sudo systemctl daemon-reload
        sudo systemctl restart pulse-backend
        
        echo -e "${GREEN}✓ Mock mode disabled!${RESET}"
        echo -e "Real nodes are now active."
        ;;
        
    status)
        if [ -f "$MOCK_CONF" ] && grep -q "PULSE_MOCK_MODE=true" "$MOCK_CONF" 2>/dev/null; then
            echo -e "${GREEN}Mock mode: ENABLED (Pure mode - real nodes disabled)${RESET}"
            if [ -f "$MOCK_ENV" ]; then
                echo "Mock configuration:"
                grep -E "PULSE_MOCK" "$MOCK_ENV" | grep -v "^#"
            fi
        else
            echo -e "${YELLOW}Mock mode: DISABLED (using real nodes)${RESET}"
        fi
        ;;
        
    *)
        echo "Usage: $0 {on|off|status}"
        echo ""
        echo "  on     - Enable PURE mock mode (no real nodes)"
        echo "  off    - Disable mock mode (use real nodes)"
        echo "  status - Show current mode"
        exit 1
        ;;
esac