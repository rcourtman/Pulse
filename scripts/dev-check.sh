#!/bin/bash
# Quick dev environment health check
# Run this before debugging frontend issues!
#
# Usage:
#   ./scripts/dev-check.sh          # Check and report
#   ./scripts/dev-check.sh --kill   # Kill all dev processes

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Handle --kill flag
if [[ "${1:-}" == "--kill" ]]; then
    echo "Stopping all dev processes..."
    pkill -9 -f "bin/pulse$" 2>/dev/null || true
    pkill -9 -f "^\./pulse$" 2>/dev/null || true
    # Kill all OpenCode processes (pattern matches any opencode serve invocation)
    pkill -f "opencode.*serve" 2>/dev/null || true
    sleep 1
    pkill -9 -f "opencode.*serve" 2>/dev/null || true
    pkill -f "node.*vite" 2>/dev/null || true
    sleep 2
    echo -e "${GREEN}✓${NC} All dev processes stopped"
    exit 0
fi

echo "=== Pulse Dev Environment Check ==="
echo ""

# Check for duplicate Pulse processes (CRITICAL!)
# Count both installed binary and dev binary patterns
# Use tr to strip newlines for robustness
PULSE_COUNT_BIN=$(pgrep -c -f "bin/pulse$" 2>/dev/null | tr -d '\n' || echo 0)
PULSE_COUNT_DEV=$(pgrep -c -f "^\./pulse$" 2>/dev/null | tr -d '\n' || echo 0)
PULSE_COUNT=$(( ${PULSE_COUNT_BIN:-0} + ${PULSE_COUNT_DEV:-0} ))
echo -n "Pulse processes: "
if [[ $PULSE_COUNT -eq 0 ]]; then
    echo -e "${YELLOW}⚠ None running${NC}"
elif [[ $PULSE_COUNT -eq 1 ]]; then
    PULSE_PID=$(pgrep -f "bin/pulse$" 2>/dev/null || pgrep -f "^\./pulse$" 2>/dev/null || echo "?")
    echo -e "${GREEN}✓ 1 running (PID: $PULSE_PID)${NC}"
else
    echo -e "${RED}✗ MULTIPLE RUNNING ($PULSE_COUNT)! This causes database locks!${NC}"
    pgrep -af "pulse$" | grep -E "(bin/pulse|^\./pulse)" | head -5
    echo -e "   ${YELLOW}Fix: ./scripts/dev-check.sh --kill${NC}"
fi

# Check Pulse backend connectivity
echo -n "Pulse backend (port 7655): "
if curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:7655/api/health 2>/dev/null | grep -q "200\|401\|403"; then
    echo -e "${GREEN}✓ Responding${NC}"
elif systemctl is-active --quiet pulse 2>/dev/null; then
    echo -e "${YELLOW}⚠ Service running but not responding${NC}"
else
    echo -e "${RED}✗ NOT RESPONDING${NC}"
fi

# Check Vite frontend
echo -n "Vite frontend (port 5173): "
if curl -s -o /dev/null http://127.0.0.1:5173/ 2>/dev/null; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${RED}✗ NOT RUNNING${NC}"
    echo "   Fix: cd /opt/pulse/frontend-modern && npm run dev"
fi

# Check OpenCode sidecar
echo -n "OpenCode sidecar: "
# Count unique ports - each OpenCode instance runs as node + binary pair on same port
OPENCODE_PORTS=$(ps aux | grep -o "opencode.*--port [0-9]*" | grep -v grep | grep -o "[0-9]*$" | sort -u)
OPENCODE_COUNT=$(echo "$OPENCODE_PORTS" | grep -c . 2>/dev/null || echo 0)
OPENCODE_PORT=$(echo "$OPENCODE_PORTS" | head -1)
if [[ $OPENCODE_COUNT -eq 0 ]] || [[ -z "$OPENCODE_PORT" ]]; then
    echo -e "${YELLOW}⚠ Not detected (starts with AI features)${NC}"
elif [[ $OPENCODE_COUNT -gt 1 ]]; then
    echo -e "${RED}✗ MULTIPLE INSTANCES ($OPENCODE_COUNT)! This causes context loss!${NC}"
    echo "   Ports: $OPENCODE_PORTS"
    echo -e "   ${YELLOW}Fix: ./scripts/dev-check.sh --kill${NC}"
elif curl -s -o /dev/null http://127.0.0.1:$OPENCODE_PORT/config 2>/dev/null; then
    echo -e "${GREEN}✓ Running on port $OPENCODE_PORT${NC}"

    # Check MCP connection
    echo -n "MCP tools connection: "
    MCP_STATUS=$(curl -s http://127.0.0.1:$OPENCODE_PORT/mcp 2>/dev/null | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    if [[ "$MCP_STATUS" == "connected" ]]; then
        echo -e "${GREEN}✓ Connected${NC}"
    else
        echo -e "${RED}✗ Not connected (tools won't work)${NC}"
        echo -e "   ${YELLOW}Fix: Restart Pulse to regenerate MCP config${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Running but not responding${NC}"
fi

# Show recent errors
echo ""
echo "=== Recent Pulse Errors (last 5) ==="
journalctl -u pulse-dev --no-pager -n 20 2>/dev/null | grep -i "error\|fatal\|panic" | tail -5 || echo "None found"

echo ""
echo "Done."
