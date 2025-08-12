#!/bin/bash

echo "=== Testing Threshold Edit UI Refresh Fix ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. Check current alert config
echo "1. Checking current alert configuration..."
OVERRIDES=$(curl -s http://localhost:7655/api/alerts/config | jq '.overrides')
if [ "$OVERRIDES" = "{}" ]; then
    echo -e "${YELLOW}   No overrides configured yet${NC}"
else
    echo -e "${GREEN}   Found existing overrides${NC}"
fi

# 2. Get a node to test with
echo ""
echo "2. Getting available nodes..."
NODE_ID=$(curl -s http://localhost:7655/api/state | jq -r '.nodes[0].id' 2>/dev/null)
NODE_NAME=$(curl -s http://localhost:7655/api/state | jq -r '.nodes[0].name' 2>/dev/null)

if [ -z "$NODE_ID" ] || [ "$NODE_ID" = "null" ]; then
    echo -e "${RED}   No nodes available for testing${NC}"
    exit 1
fi

echo -e "${GREEN}   Found node: $NODE_NAME (ID: $NODE_ID)${NC}"

# 3. Create a test override
echo ""
echo "3. Creating test threshold override for $NODE_NAME..."
TEST_CONFIG="{
  \"overrides\": {
    \"$NODE_ID\": {
      \"cpu\": { \"trigger\": 75, \"clear\": 70 },
      \"memory\": { \"trigger\": 80, \"clear\": 75 }
    }
  }
}"

# Save the override
curl -s -X PUT http://localhost:7655/api/alerts/config \
    -H "Content-Type: application/json" \
    -d "$TEST_CONFIG" > /dev/null

# Verify it was saved
SAVED_OVERRIDE=$(curl -s http://localhost:7655/api/alerts/config | jq ".overrides[\"$NODE_ID\"]")
if [ "$SAVED_OVERRIDE" != "null" ]; then
    echo -e "${GREEN}   ✓ Override created successfully${NC}"
else
    echo -e "${RED}   ✗ Failed to create override${NC}"
    exit 1
fi

# 4. Test instructions
echo ""
echo "4. Manual UI Test Instructions:"
echo "   ================================"
echo -e "${YELLOW}"
echo "   a) Open Pulse in your browser: http://localhost:7655"
echo "   b) Navigate to Alerts → Thresholds tab"
echo "   c) Find the override for '$NODE_NAME'"
echo "   d) Click 'Edit' button"
echo "   e) Wait 15 seconds (3 refresh cycles)"
echo "   f) Verify Save/Cancel buttons are STILL VISIBLE"
echo ""
echo "   Expected: Buttons remain visible during all refreshes"
echo "   Old Bug: Buttons would disappear after 5 seconds"
echo -e "${NC}"

# 5. Verification check
echo "5. Code verification:"
echo "   Checking if fix is implemented in code..."

# Check for the editingOverrideId state management
if grep -q "editingOverrideId" /opt/pulse/frontend-modern/src/pages/Alerts.tsx; then
    echo -e "${GREEN}   ✓ Fix is present in code (editingOverrideId state management found)${NC}"
else
    echo -e "${RED}   ✗ Fix may not be implemented${NC}"
fi

# Check for proper prop passing
if grep -q "isEditing={editingOverrideId()" /opt/pulse/frontend-modern/src/pages/Alerts.tsx; then
    echo -e "${GREEN}   ✓ Edit state properly passed to components${NC}"
else
    echo -e "${YELLOW}   ⚠ Could not verify prop passing${NC}"
fi

echo ""
echo "=== Test Setup Complete ==="
echo -e "${GREEN}Override created for $NODE_NAME - Please test manually in browser${NC}"
echo ""
echo "To clean up test data later, run:"
echo "curl -X PUT http://localhost:7655/api/alerts/config -H 'Content-Type: application/json' -d '{\"overrides\":{}}'"