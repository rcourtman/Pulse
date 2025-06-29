#!/bin/bash

# Test script for PBS Push Mode
# This script sends a test metric payload to verify the push endpoint is working

# Configuration - update these values
PULSE_SERVER_URL="${PULSE_SERVER_URL:-https://localhost:7655}"
PULSE_API_KEY="${PULSE_API_KEY:-your-api-key-here}"
PBS_ID="${PBS_ID:-test-pbs-01}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "PBS Push Mode Test Script"
echo "========================="
echo ""

# Check if API key is set
if [ "$PULSE_API_KEY" = "your-api-key-here" ]; then
    echo -e "${RED}ERROR: Please set PULSE_API_KEY environment variable${NC}"
    echo "Example: export PULSE_API_KEY=your-actual-key"
    exit 1
fi

echo "Configuration:"
echo "- Pulse Server: $PULSE_SERVER_URL"
echo "- PBS ID: $PBS_ID"
echo ""

# Create test payload
TIMESTAMP=$(date +%s)000
PAYLOAD=$(cat <<EOF
{
  "pbsId": "$PBS_ID",
  "nodeStatus": {
    "uptime": 86400,
    "cpu": 0.25,
    "memory": {
      "used": 4294967296,
      "total": 8589934592
    },
    "loadavg": [1.5, 1.2, 1.0]
  },
  "datastores": [
    {
      "name": "backup",
      "total": 1099511627776,
      "used": 549755813888,
      "available": 549755813888,
      "type": "directory"
    }
  ],
  "tasks": [],
  "version": "2.4.1",
  "timestamp": $TIMESTAMP,
  "agentVersion": "test-1.0.0",
  "pushInterval": 30000
}
EOF
)

echo "Sending test metrics to push endpoint..."
echo ""

# Send request
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$PULSE_SERVER_URL/api/push/metrics" \
  -H "X-API-Key: $PULSE_API_KEY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" 2>&1)

# Extract HTTP code and response body
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

# Check response
if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}SUCCESS: Metrics pushed successfully!${NC}"
    echo ""
    echo "Response:"
    echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
    echo ""
    
    # Test listing agents
    echo "Fetching connected agents..."
    AGENTS=$(curl -s -H "X-API-Key: $PULSE_API_KEY" "$PULSE_SERVER_URL/api/push/agents")
    echo "$AGENTS" | jq . 2>/dev/null || echo "$AGENTS"
    
else
    echo -e "${RED}ERROR: Failed to push metrics (HTTP $HTTP_CODE)${NC}"
    echo ""
    echo "Response:"
    echo "$BODY"
    echo ""
    
    if [ "$HTTP_CODE" = "401" ]; then
        echo "Authentication failed. Check your API key."
    elif [ "$HTTP_CODE" = "503" ]; then
        echo "Push mode not configured on server. Set PULSE_PUSH_API_KEY on the server."
    elif [ "$HTTP_CODE" = "000" ]; then
        echo "Could not connect to server. Check the URL and network connectivity."
    fi
fi