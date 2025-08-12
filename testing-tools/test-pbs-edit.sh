#!/bin/bash

# Test PBS edit form data loading
echo "PBS Edit Form Test Script"
echo "========================="

# API endpoint and token
API_URL="http://localhost:7655/api"
API_TOKEN="test-token-123"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "\n${YELLOW}Step 1: Fetching current PBS nodes...${NC}"
RESPONSE=$(curl -s -H "X-API-Token: $API_TOKEN" "$API_URL/config/nodes")

# Check if we have PBS nodes (response is an array)
PBS_COUNT=$(echo "$RESPONSE" | jq '[.[] | select(.type == "pbs")] | length')
echo "Found $PBS_COUNT PBS instances"

if [ "$PBS_COUNT" -eq 0 ]; then
    echo -e "${YELLOW}No PBS nodes found. Creating a test PBS node...${NC}"
    
    # Create a test PBS node
    PBS_DATA='{
        "node": {
            "type": "pbs",
            "name": "Test PBS",
            "host": "192.168.0.8:8007",
            "tokenName": "testuser@pbs!test-token",
            "tokenValue": "test-token-value",
            "verifySSL": false,
            "monitorDatastores": true,
            "monitorSyncJobs": true,
            "monitorVerifyJobs": true,
            "monitorPruneJobs": true,
            "monitorGarbageJobs": false
        }
    }'
    
    CREATE_RESPONSE=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Token: $API_TOKEN" \
        -d "$PBS_DATA" \
        "$API_URL/config/nodes")
    
    echo "PBS node created: $CREATE_RESPONSE"
    
    # Fetch nodes again
    RESPONSE=$(curl -s -H "X-API-Token: $API_TOKEN" "$API_URL/config/nodes")
fi

# Get first PBS node details (filter from array)
PBS_NODE=$(echo "$RESPONSE" | jq '[.[] | select(.type == "pbs")] | .[0]')

if [ "$PBS_NODE" != "null" ]; then
    echo -e "\n${GREEN}PBS Node Data:${NC}"
    echo "$PBS_NODE" | jq '.'
    
    # Check critical fields
    echo -e "\n${YELLOW}Checking PBS node fields:${NC}"
    
    # Check if tokenName contains username
    TOKEN_NAME=$(echo "$PBS_NODE" | jq -r '.tokenName // empty')
    HAS_TOKEN=$(echo "$PBS_NODE" | jq -r '.hasToken // false')
    HAS_PASSWORD=$(echo "$PBS_NODE" | jq -r '.hasPassword // false')
    
    echo "- tokenName: $TOKEN_NAME"
    echo "- hasToken: $HAS_TOKEN"
    echo "- hasPassword: $HAS_PASSWORD"
    
    if [[ "$TOKEN_NAME" == *"!"* ]]; then
        echo -e "${GREEN}✓ Token name contains username separator (!)${NC}"
        USERNAME=$(echo "$TOKEN_NAME" | cut -d'!' -f1)
        TOKEN_PART=$(echo "$TOKEN_NAME" | cut -d'!' -f2)
        echo "  - Extracted username: $USERNAME"
        echo "  - Token part: $TOKEN_PART"
    else
        echo -e "${YELLOW}Note: Token name doesn't contain separator, might be using password auth${NC}"
    fi
    
    # Check auth type detection
    if [ "$HAS_TOKEN" == "true" ]; then
        echo -e "${GREEN}✓ Node is using token authentication${NC}"
    elif [ "$HAS_PASSWORD" == "true" ]; then
        echo -e "${GREEN}✓ Node is using password authentication${NC}"
    else
        echo -e "${RED}✗ No authentication method detected${NC}"
    fi
    
    # Check monitoring settings
    echo -e "\n${YELLOW}PBS Monitoring Settings:${NC}"
    echo "- monitorDatastores: $(echo "$PBS_NODE" | jq -r '.monitorDatastores')"
    echo "- monitorSyncJobs: $(echo "$PBS_NODE" | jq -r '.monitorSyncJobs')"
    echo "- monitorVerifyJobs: $(echo "$PBS_NODE" | jq -r '.monitorVerifyJobs')"
    echo "- monitorPruneJobs: $(echo "$PBS_NODE" | jq -r '.monitorPruneJobs')"
    echo "- monitorGarbageJobs: $(echo "$PBS_NODE" | jq -r '.monitorGarbageJobs')"
    
else
    echo -e "${RED}No PBS nodes found in the system${NC}"
fi

echo -e "\n${YELLOW}Test Complete!${NC}"
echo "To verify in UI:"
echo "1. Open http://localhost:7655"
echo "2. Go to Settings → Nodes"
echo "3. Click edit on a PBS node"
echo "4. Check that all fields are populated correctly"