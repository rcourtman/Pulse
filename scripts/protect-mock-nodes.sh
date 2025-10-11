#!/bin/bash

# This script patches test scripts to protect mock nodes from deletion
# It ensures that mock nodes (pve1-pve7, mock-*) are never deleted during tests

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Protecting mock nodes from test cleanup..."

# List of test scripts that delete nodes
TEST_SCRIPTS=(
    "/opt/pulse/scripts/test-persistence.sh"
    "/opt/pulse/scripts/test-recovery.sh"
    "/opt/pulse/scripts/test-backup.sh"
    "/opt/pulse/scripts/test-load.sh"
    "/opt/pulse/scripts/test-config-validation.sh"
)

for script in "${TEST_SCRIPTS[@]}"; do
    if [ -f "$script" ]; then
        echo -n "Patching $(basename $script)... "
        
        # Create backup
        cp "$script" "${script}.backup" 2>/dev/null
        
        # Add protection for mock nodes in cleanup sections
        # This looks for DELETE commands and adds a check to skip mock nodes
        
        # Find lines with curl DELETE and add protection
        sed -i '/curl.*DELETE.*nodes/i\
        # Skip mock nodes and production nodes\
        if [[ "$NODE_NAME" == "pve"* ]] || [[ "$NODE_NAME" == "mock"* ]] || [[ "$NODE_NAME" == "delly" ]] || [[ "$NODE_NAME" == "minipc" ]] || [[ "$NODE_NAME" == "pimox" ]]; then\
            continue\
        fi' "$script" 2>/dev/null
        
        echo -e "${GREEN}✓${NC}"
    fi
done

echo -e "${GREEN}Mock nodes are now protected!${NC}"