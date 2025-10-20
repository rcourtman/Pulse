#!/bin/bash
# Dev Environment Orchestrator
# Provides complete state detection and control for development tools

set -eo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/.." && pwd)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

#########################################
# STATE DETECTION
#########################################

detect_backend_service() {
    local services=("pulse-hot-dev" "pulse" "pulse-backend")
    for svc in "${services[@]}"; do
        if systemctl list-unit-files --no-legend 2>/dev/null | grep -q "^${svc}\\.service"; then
            echo "$svc"
            return 0
        fi
    done
    echo ""
}

detect_running_backend_service() {
    local services=("pulse-hot-dev" "pulse" "pulse-backend")
    for svc in "${services[@]}"; do
        if systemctl is-active --quiet "$svc" 2>/dev/null; then
            echo "$svc"
            return 0
        fi
    done
    echo ""
}

detect_backend_state() {
    local state="{}"
    local running_service=$(detect_running_backend_service)

    if [[ -n "$running_service" ]]; then
        local backend_type="systemd"
        if [[ "$running_service" == "pulse-hot-dev" ]]; then
            backend_type="hot-dev"
        fi

        state=$(echo "$state" | jq ". + {backend_running: true, backend_type: \"$backend_type\", backend_service: \"$running_service\"}")

        # Check mock mode from logs (multiple possible indicators, look at last 2 minutes for reliability)
        if sudo journalctl -u "$running_service" --since "2 minutes ago" | grep -qE "(Mock mode enabled|mockEnabled=true|mock mode trackedNodes)"; then
            state=$(echo "$state" | jq '. + {mock_mode: true}')
        else
            state=$(echo "$state" | jq '. + {mock_mode: false}')
        fi
    else
        state=$(echo "$state" | jq '. + {backend_running: false}')
        local configured_service=$(detect_backend_service)
        if [[ -n "$configured_service" ]]; then
            local backend_type="systemd"
            if [[ "$configured_service" == "pulse-hot-dev" ]]; then
                backend_type="hot-dev"
            fi
            state=$(echo "$state" | jq ". + {backend_service: \"$configured_service\", backend_type: \"$backend_type\"}")
        fi
    fi

    # Check what's configured in mock.env.local
    if [ -f "$ROOT_DIR/mock.env.local" ]; then
        if grep -q "PULSE_MOCK_MODE=true" "$ROOT_DIR/mock.env.local"; then
            state=$(echo "$state" | jq '. + {mock_configured: true}')
        else
            state=$(echo "$state" | jq '. + {mock_configured: false}')
        fi
    else
        # Check mock.env
        if grep -q "PULSE_MOCK_MODE=true" "$ROOT_DIR/mock.env" 2>/dev/null; then
            state=$(echo "$state" | jq '. + {mock_configured: true}')
        else
            state=$(echo "$state" | jq '. + {mock_configured: false}')
        fi
    fi

    echo "$state"
}

detect_frontend_state() {
    local state="{}"

    # Check if frontend dev server is running (Vite)
    if pgrep -f "vite.*7655" > /dev/null 2>&1; then
        state=$(echo "$state" | jq '. + {frontend_running: true, frontend_type: "vite-dev"}')
    elif [ -d "$ROOT_DIR/frontend-modern/dist" ]; then
        state=$(echo "$state" | jq '. + {frontend_running: false, frontend_built: true}')
    else
        state=$(echo "$state" | jq '. + {frontend_running: false, frontend_built: false}')
    fi

    echo "$state"
}

get_full_state() {
    local backend=$(detect_backend_state)
    local frontend=$(detect_frontend_state)

    echo "{}" | jq ". + {backend: $backend, frontend: $frontend}"
}

#########################################
# MODE SWITCHING
#########################################

switch_to_mock() {
    echo -e "${YELLOW}Switching to mock mode...${NC}"
    local service=$(detect_backend_service)
    if [[ -z "$service" ]]; then
        echo -e "${RED}✗ No Pulse systemd service detected${NC}"
        return 1
    fi

    # Update mock.env.local (preferred) or mock.env
    if [ -f "$ROOT_DIR/mock.env.local" ]; then
        sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=true/' "$ROOT_DIR/mock.env.local"
        echo -e "${GREEN}✓ Updated mock.env.local${NC}"
    else
        sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=true/' "$ROOT_DIR/mock.env"
        echo -e "${GREEN}✓ Updated mock.env${NC}"
    fi

    # Restart backend
    sudo systemctl restart "$service"
    echo -e "${GREEN}✓ Backend restarted${NC}"

    # Wait for backend to be ready
    sleep 3

    # Verify
    if sudo journalctl -u "$service" --since "5 seconds ago" | grep -qE "(Mock mode enabled|mockEnabled=true|mock mode trackedNodes)"; then
        echo -e "${GREEN}✓ Mock mode ACTIVE${NC}"
        return 0
    else
        echo -e "${RED}✗ Mock mode failed to activate${NC}"
        return 1
    fi
}

switch_to_production() {
    echo -e "${YELLOW}Switching to production mode...${NC}"
    local service=$(detect_backend_service)
    if [[ -z "$service" ]]; then
        echo -e "${RED}✗ No Pulse systemd service detected${NC}"
        return 1
    fi

    # Sync production config first
    if [ -f "$ROOT_DIR/scripts/sync-production-config.sh" ]; then
        echo "Syncing production configuration..."
        "$ROOT_DIR/scripts/sync-production-config.sh"
    fi

    # Update mock.env.local (preferred) or mock.env
    if [ -f "$ROOT_DIR/mock.env.local" ]; then
        sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=false/' "$ROOT_DIR/mock.env.local"
        echo -e "${GREEN}✓ Updated mock.env.local${NC}"
    else
        sed -i 's/PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=false/' "$ROOT_DIR/mock.env"
        echo -e "${GREEN}✓ Updated mock.env${NC}"
    fi

    # Restart backend
    sudo systemctl restart "$service"
    echo -e "${GREEN}✓ Backend restarted${NC}"

    # Wait for backend to be ready
    sleep 3

    echo -e "${GREEN}✓ Production mode ACTIVE${NC}"
    return 0
}

#########################################
# FRONTEND MANAGEMENT
#########################################

rebuild_frontend() {
    echo -e "${YELLOW}Rebuilding frontend...${NC}"

    cd "$ROOT_DIR/frontend-modern"
    npm run build

    echo -e "${GREEN}✓ Frontend rebuilt${NC}"
}

#########################################
# COMMANDS
#########################################

cmd_status() {
    local state=$(get_full_state)

    echo -e "${BLUE}=== Dev Environment Status ===${NC}"
    echo ""

    # Backend
    local backend_running=$(echo "$state" | jq -r '.backend.backend_running')
    local backend_type=$(echo "$state" | jq -r '.backend.backend_type // "none"')
    local mock_mode=$(echo "$state" | jq -r '.backend.mock_mode // false')
    local mock_configured=$(echo "$state" | jq -r '.backend.mock_configured // false')

    echo -e "${BLUE}Backend:${NC}"
    if [ "$backend_running" = "true" ]; then
        echo -e "  Status: ${GREEN}Running${NC} ($backend_type)"
        if [ "$mock_mode" = "true" ]; then
            echo -e "  Mode: ${GREEN}Mock${NC}"
        else
            echo -e "  Mode: ${YELLOW}Production${NC}"
        fi
    else
        echo -e "  Status: ${RED}Stopped${NC}"
    fi
    echo -e "  Configured: $([ "$mock_configured" = "true" ] && echo -e "${GREEN}Mock${NC}" || echo -e "${YELLOW}Production${NC}")"

    # Frontend
    local frontend_running=$(echo "$state" | jq -r '.frontend.frontend_running')
    local frontend_type=$(echo "$state" | jq -r '.frontend.frontend_type // "none"')
    local frontend_built=$(echo "$state" | jq -r '.frontend.frontend_built // false')

    echo ""
    echo -e "${BLUE}Frontend:${NC}"
    if [ "$frontend_running" = "true" ]; then
        echo -e "  Status: ${GREEN}Running${NC} ($frontend_type)"
    else
        echo -e "  Status: ${RED}Not running${NC}"
        echo -e "  Built: $([ "$frontend_built" = "true" ] && echo -e "${GREEN}Yes${NC}" || echo -e "${RED}No${NC}")"
    fi

    # JSON output for automation tools
    if [ "$1" = "--json" ]; then
        echo ""
        echo "$state"
    fi
}

cmd_mock() {
    switch_to_mock
}

cmd_prod() {
    switch_to_production
}

cmd_restart() {
    echo -e "${YELLOW}Restarting backend...${NC}"
    local service=$(detect_backend_service)
    if [[ -z "$service" ]]; then
        echo -e "${RED}✗ No Pulse systemd service detected${NC}"
        return 1
    fi
    sudo systemctl restart "$service"
    sleep 2
    echo -e "${GREEN}✓ Backend restarted${NC}"
}

cmd_help() {
    cat << EOF
Dev Environment Orchestrator

Usage: $0 <command>

Commands:
  status [--json]  Show current environment state
  mock             Switch to mock mode
  prod             Switch to production mode
  restart          Restart backend service
  help             Show this help

Examples:
  $0 status        # Human-readable status
  $0 status --json # JSON output for automation
  $0 mock          # Switch to mock mode
  $0 prod          # Switch to production mode

EOF
}

#########################################
# MAIN
#########################################

case "${1:-status}" in
    status)
        cmd_status "$2"
        ;;
    mock)
        cmd_mock
        ;;
    prod)
        cmd_prod
        ;;
    restart)
        cmd_restart
        ;;
    help|--help|-h)
        cmd_help
        ;;
    *)
        echo "Unknown command: $1"
        cmd_help
        exit 1
        ;;
esac
