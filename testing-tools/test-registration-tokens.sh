#!/bin/bash

echo "=== Testing Registration Token Feature ==="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PULSE_URL="http://localhost:7655"

# Test 1: Try auto-register without token (should work by default in homelab mode)
echo "Test 1: Auto-register without token (homelab mode)..."
TEST_NODE='{
  "type": "pve",
  "host": "test-node.local:8006",
  "name": "Test Node",
  "username": "root@pam",
  "password": "test-password"
}'

RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
  -H "Content-Type: application/json" \
  -d "$TEST_NODE" 2>&1)

if echo "$RESPONSE" | grep -q "success"; then
  echo -e "${GREEN}   ✓ Auto-registration without token succeeded (homelab mode)${NC}"
else
  echo -e "${YELLOW}   ⚠ Auto-registration response: $RESPONSE${NC}"
fi

# Test 2: Check if we can enable token requirement
echo ""
echo "Test 2: Checking registration token configuration..."

# Check if token requirement is enabled
if [ -n "$REQUIRE_REGISTRATION_TOKEN" ]; then
  echo -e "${YELLOW}   Registration tokens are required (REQUIRE_REGISTRATION_TOKEN=$REQUIRE_REGISTRATION_TOKEN)${NC}"
else
  echo -e "${GREEN}   Registration tokens are optional (default homelab mode)${NC}"
fi

# Test 3: Generate a setup script with token
echo ""
echo "Test 3: Generating setup script..."
SETUP_RESPONSE=$(curl -s "$PULSE_URL/api/setup-script?node=test&token=test-token-123")

if echo "$SETUP_RESPONSE" | grep -q "PULSE_URL"; then
  echo -e "${GREEN}   ✓ Setup script generated successfully${NC}"
  
  # Check if token is included
  if echo "$SETUP_RESPONSE" | grep -q "REG_TOKEN"; then
    echo -e "${GREEN}   ✓ Registration token included in script${NC}"
  else
    echo -e "${YELLOW}   ⚠ No registration token in script${NC}"
  fi
else
  echo -e "${RED}   ✗ Failed to generate setup script${NC}"
fi

# Test 4: Test with invalid token when tokens are required
echo ""
echo "Test 4: Testing token validation..."

# This would only fail if REQUIRE_REGISTRATION_TOKEN=true
INVALID_RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
  -H "Content-Type: application/json" \
  -H "X-Registration-Token: invalid-token" \
  -d "$TEST_NODE" 2>&1)

if echo "$INVALID_RESPONSE" | grep -q "Unauthorized\|Invalid token"; then
  echo -e "${GREEN}   ✓ Invalid token rejected${NC}"
else
  echo -e "${YELLOW}   ⚠ Token validation may not be active${NC}"
fi

# Test 5: Check registration endpoints
echo ""
echo "Test 5: Checking registration endpoints..."

# Check if setup script endpoint works
SETUP_CHECK=$(curl -s -o /dev/null -w "%{http_code}" "$PULSE_URL/api/setup-script")
if [ "$SETUP_CHECK" = "200" ]; then
  echo -e "${GREEN}   ✓ Setup script endpoint available (/api/setup-script)${NC}"
else
  echo -e "${RED}   ✗ Setup script endpoint returned: $SETUP_CHECK${NC}"
fi

# Check if auto-register endpoint works
REGISTER_CHECK=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PULSE_URL/api/auto-register" \
  -H "Content-Type: application/json" \
  -d '{}')
if [ "$REGISTER_CHECK" = "400" ] || [ "$REGISTER_CHECK" = "401" ] || [ "$REGISTER_CHECK" = "200" ]; then
  echo -e "${GREEN}   ✓ Auto-register endpoint available (/api/auto-register)${NC}"
else
  echo -e "${RED}   ✗ Auto-register endpoint returned: $REGISTER_CHECK${NC}"
fi

echo ""
echo "=== Summary ==="
echo "The registration token feature allows:"
echo "1. Secure node auto-registration with tokens"
echo "2. Optional token requirement (via REQUIRE_REGISTRATION_TOKEN env var)"
echo "3. Setup scripts with embedded tokens"
echo "4. Default homelab mode (no token required)"
echo ""
echo "To enable token requirement, set:"
echo "  REQUIRE_REGISTRATION_TOKEN=true"
echo "in the Pulse service environment"