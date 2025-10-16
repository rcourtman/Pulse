#!/bin/bash
# Clean mock data contamination from production alerts
# This removes any alerts with "mock" in the resourceId

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ALERT_HISTORY="/etc/pulse/alerts/alert-history.json"

if [ ! -f "$ALERT_HISTORY" ]; then
    echo -e "${RED}Error: Alert history file not found at $ALERT_HISTORY${NC}"
    exit 1
fi

# Count mock alerts
MOCK_COUNT=$(jq '[.[] | select((.alert.resourceId // "" | contains("mock")))] | length' "$ALERT_HISTORY")

if [ "$MOCK_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ No mock alerts found in production data${NC}"
    exit 0
fi

echo -e "${YELLOW}Found $MOCK_COUNT mock alerts in production data${NC}"
echo "Creating backup..."

# Create backup
BACKUP_FILE="${ALERT_HISTORY}.backup-$(date +%Y%m%d-%H%M%S)"
sudo cp "$ALERT_HISTORY" "$BACKUP_FILE"
echo -e "${GREEN}✓ Backup created: $BACKUP_FILE${NC}"

# Stop backend to prevent writes during cleanup
echo "Stopping backend..."
pkill -x pulse 2>/dev/null || true
sudo systemctl stop pulse-hot-dev 2>/dev/null || true
sudo systemctl stop pulse 2>/dev/null || true
sudo systemctl stop pulse-backend 2>/dev/null || true
sleep 2

# Filter out mock alerts
echo "Filtering out mock alerts..."
jq '[.[] | select((.alert.resourceId // "" | contains("mock")) | not)]' "$ALERT_HISTORY" > /tmp/alert-history-cleaned.json

# Verify the filtered file
CLEANED_COUNT=$(jq 'length' /tmp/alert-history-cleaned.json)
REMOVED_COUNT=$(($(jq 'length' "$ALERT_HISTORY") - CLEANED_COUNT))

echo "Original alerts: $(jq 'length' "$ALERT_HISTORY")"
echo "Cleaned alerts: $CLEANED_COUNT"
echo "Removed alerts: $REMOVED_COUNT"

# Apply cleaned file
sudo cp /tmp/alert-history-cleaned.json "$ALERT_HISTORY"
sudo chown pulse:pulse "$ALERT_HISTORY"

echo -e "${GREEN}✓ Mock alerts removed successfully${NC}"
echo ""
echo "To restart the backend, run:"
echo "  ./scripts/hot-dev.sh    (for development)"
echo "  sudo systemctl start pulse           (systemd)"
echo "  sudo systemctl start pulse-backend   (legacy)"
