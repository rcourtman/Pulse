#!/bin/bash
# Clean mock data contamination from production alerts
# This removes any alerts with "mock" in the resourceId

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ALERT_HISTORY="/etc/pulse/alerts/alert-history.json"

if ! command -v jq >/dev/null 2>&1; then
    echo -e "${RED}Error: jq is required but not installed${NC}"
    exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
elif command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
else
    echo -e "${RED}Error: sudo is required when running as non-root${NC}"
    exit 1
fi

run_privileged() {
    if [ -n "$SUDO" ]; then
        "$SUDO" "$@"
    else
        "$@"
    fi
}

if [ ! -f "$ALERT_HISTORY" ]; then
    echo -e "${RED}Error: Alert history file not found at $ALERT_HISTORY${NC}"
    exit 1
fi

# Count mock alerts
MOCK_COUNT=$(jq '[.[] | select((.alert.resourceId // "" | contains("mock")))] | length' "$ALERT_HISTORY")
ORIGINAL_COUNT=$(jq 'length' "$ALERT_HISTORY")

if [ "$MOCK_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ No mock alerts found in production data${NC}"
    exit 0
fi

echo -e "${YELLOW}Found $MOCK_COUNT mock alerts in production data${NC}"
echo "Creating backup..."

# Create backup
BACKUP_FILE="${ALERT_HISTORY}.backup-$(date +%Y%m%d-%H%M%S)"
run_privileged cp "$ALERT_HISTORY" "$BACKUP_FILE"
echo -e "${GREEN}✓ Backup created: $BACKUP_FILE${NC}"

# Stop backend to prevent writes during cleanup
echo "Stopping backend..."
pkill -x pulse 2>/dev/null || true
run_privileged systemctl stop pulse-hot-dev 2>/dev/null || true
run_privileged systemctl stop pulse 2>/dev/null || true
run_privileged systemctl stop pulse-backend 2>/dev/null || true
sleep 2

# Filter out mock alerts
TMP_CLEANED=$(mktemp "${TMPDIR:-/tmp}/alert-history-cleaned.XXXXXX.json")
trap 'rm -f "$TMP_CLEANED"' EXIT
echo "Filtering out mock alerts..."
jq '[.[] | select((.alert.resourceId // "" | contains("mock")) | not)]' "$ALERT_HISTORY" > "$TMP_CLEANED"

# Verify the filtered file
CLEANED_COUNT=$(jq 'length' "$TMP_CLEANED")
REMOVED_COUNT=$((ORIGINAL_COUNT - CLEANED_COUNT))

echo "Original alerts: $ORIGINAL_COUNT"
echo "Cleaned alerts: $CLEANED_COUNT"
echo "Removed alerts: $REMOVED_COUNT"

# Apply cleaned file
run_privileged cp "$TMP_CLEANED" "$ALERT_HISTORY"
run_privileged chown pulse:pulse "$ALERT_HISTORY"

echo -e "${GREEN}✓ Mock alerts removed successfully${NC}"
echo ""
echo "To restart the backend, run:"
echo "  ./scripts/hot-dev.sh    (for development)"
echo "  sudo systemctl start pulse           (systemd)"
echo "  sudo systemctl start pulse-backend   (legacy)"
