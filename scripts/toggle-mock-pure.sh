#!/bin/bash

# Pure mock mode toggle - disables real nodes completely
# This is a workaround until we properly integrate mock mode to skip node initialization

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

NODES_FILE="/etc/pulse/nodes.enc"
NODES_BACKUP="/etc/pulse/nodes.enc.real"

enable_pure_mock() {
    echo -e "${YELLOW}Enabling PURE mock mode (no real nodes)...${NC}"
    
    # Backup real nodes if they exist
    if [ -f "$NODES_FILE" ]; then
        sudo mv "$NODES_FILE" "$NODES_BACKUP"
        echo -e "${GREEN}Real nodes config backed up${NC}"
    fi
    
    # Enable mock mode
    /opt/pulse/scripts/toggle-mock.sh on
    
    echo -e "${GREEN}✓ Pure mock mode enabled!${NC}"
    echo -e "${YELLOW}Real nodes are completely disabled${NC}"
}

disable_pure_mock() {
    echo -e "${YELLOW}Restoring real nodes...${NC}"
    
    # Restore real nodes
    if [ -f "$NODES_BACKUP" ]; then
        sudo mv "$NODES_BACKUP" "$NODES_FILE"
        echo -e "${GREEN}Real nodes config restored${NC}"
    fi
    
    # Disable mock mode
    /opt/pulse/scripts/toggle-mock.sh off
    
    echo -e "${GREEN}✓ Back to real nodes!${NC}"
}

case "$1" in
    on)
        enable_pure_mock
        ;;
    off)
        disable_pure_mock
        ;;
    *)
        echo "Pure Mock Mode Toggle"
        echo "====================="
        echo ""
        echo "This completely disables real nodes for pure mock testing"
        echo ""
        echo "Usage: $0 {on|off}"
        echo ""
        echo "  on  - Enable pure mock mode (no real nodes)"
        echo "  off - Restore real nodes"
        ;;
esac