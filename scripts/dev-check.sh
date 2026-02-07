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
    pkill -f "node.*vite" 2>/dev/null || true
    pkill -f "watch-snapshot.sh" 2>/dev/null || true
    sleep 2
    echo -e "${GREEN}✓${NC} All dev processes stopped"
    exit 0
fi

echo "=== Pulse Dev Environment Check ==="
echo ""

# Check for duplicate Pulse processes (CRITICAL!)
# Count both installed binary and dev binary patterns
# Note: macOS pgrep doesn't support -c, so we use wc -l
PULSE_COUNT_BIN=$(pgrep -f "bin/pulse$" 2>/dev/null | wc -l | tr -d ' ')
PULSE_COUNT_DEV=$(pgrep -f "^\./pulse$" 2>/dev/null | wc -l | tr -d ' ')
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
elif [[ "$(uname -s)" == "Linux" ]] && systemctl is-active --quiet pulse 2>/dev/null; then
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
    echo "   Fix: cd \${PULSE_REPOS_DIR:-/Volumes/Development/pulse/repos}/pulse/frontend-modern && npm run dev"
fi

# Check AI status
echo -n "AI service: "
AI_STATUS=$(curl -s -u admin:admin http://127.0.0.1:7655/api/ai/status 2>/dev/null | jq -r '.running // false')
if [[ "$AI_STATUS" == "true" ]]; then
    echo -e "${GREEN}✓ Running (direct integration)${NC}"
else
    echo -e "${YELLOW}⚠ Not running (enable in settings)${NC}"
fi

# Check snapshot watcher
echo -n "Snapshot watcher: "
SNAPSHOT_PID=$(pgrep -f "watch-snapshot.sh" 2>/dev/null | head -1)
if [[ -n "$SNAPSHOT_PID" ]]; then
    SNAPSHOT_COUNT=$(git -C ~/.pulse-snapshots rev-list --count HEAD 2>/dev/null || echo 0)
    echo -e "${GREEN}✓ Running (PID: $SNAPSHOT_PID, $SNAPSHOT_COUNT snapshots)${NC}"
else
    echo -e "${YELLOW}⚠ Not running (optional - protects against accidental file loss)${NC}"
    echo "   Start: ./scripts/watch-snapshot.sh &"
fi

# Show recent errors
echo ""
echo "=== Recent Pulse Errors (last 5) ==="
if [[ "$(uname -s)" == "Darwin" ]]; then
    # macOS: check the debug log
    grep -i "error\|fatal\|panic" /tmp/pulse-debug.log 2>/dev/null | tail -5 || echo "None found"
else
    # Linux: use journalctl
    journalctl -u pulse-dev --no-pager -n 20 2>/dev/null | grep -i "error\|fatal\|panic" | tail -5 || echo "None found"
fi

echo ""
echo "Done."
