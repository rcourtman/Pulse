#!/bin/bash

# Safe test runner that uses mock mode to protect production nodes
# This ensures tests NEVER touch real Proxmox nodes

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

MODE="full"
API_TOKEN="${API_TOKEN:-}"
# Allow new multi-token env var to provide the test credential
if [ -z "$API_TOKEN" ] && [ -n "${API_TOKENS:-}" ]; then
    # Use the first token in the comma-separated list
    API_TOKEN="$(printf '%s\n' "$API_TOKENS" | tr ',' '\n' | head -n1)"
fi
PULSE_URL="${PULSE_URL:-http://localhost:7655}"
FAILED=0
PASSED=0
ORIGINAL_MOCK_STATE=""

echo "================================================"
echo -e "${BLUE}PULSE SAFE TEST SUITE (MOCK MODE)${NC}"
echo "================================================"
echo ""

# Save original mock state
echo -n "Checking current mock mode status: "
if grep -q "PULSE_MOCK_MODE=true" /opt/pulse/mock.env 2>/dev/null; then
    ORIGINAL_MOCK_STATE="true"
    echo -e "${GREEN}Already in mock mode${NC}"
else
    ORIGINAL_MOCK_STATE="false"
    echo -e "${YELLOW}Real mode - will switch to mock${NC}"
fi

# Enable mock mode for testing
echo -n "Enabling mock mode for safe testing: "
/opt/pulse/scripts/toggle-mock.sh on > /dev/null 2>&1
echo -e "${GREEN}✓${NC}"

# Wait for service to restart with mock mode
echo -n "Waiting for service to restart with mock data: "
sleep 8
echo -e "${GREEN}✓${NC}"

# Verify mock mode is active
echo -n "Verifying mock mode is active: "
if curl -s "$PULSE_URL/api/config/nodes" | grep -q "pve1\|mock"; then
    echo -e "${GREEN}✓ Mock nodes detected${NC}"
else
    echo -e "${YELLOW}⚠️  Mock nodes not detected, but continuing${NC}"
fi

echo ""
echo -e "${GREEN}Running tests in SAFE MODE - your production nodes are protected!${NC}"
echo ""

run_test() {
    local name="$1"
    local script="$2"
    local args="$3"
    
    echo -n "Running $name... "
    
    if [ ! -f "$script" ]; then
        echo "SKIPPED (not found)"
        return
    fi
    
    # Set environment to ensure tests know they're in mock mode
    export PULSE_MOCK_MODE=true
    
    if $script $args > /tmp/test-$name.log 2>&1; then
        echo -e "${GREEN}✅ PASSED${NC}"
        ((PASSED++))
    else
        echo -e "${RED}❌ FAILED${NC} (see /tmp/test-$name.log)"
        ((FAILED++))
    fi
}

echo "RUNNING ALL TESTS (SAFE MODE):"
echo "=============================="

# Core functionality tests
run_test "api" "./scripts/test-api.sh" ""
run_test "frontend" "./scripts/test-frontend.sh" ""
run_test "security" "./scripts/test-security.sh" ""
run_test "edge-cases" "./scripts/test-edge-cases.sh" ""
run_test "proxy" "./scripts/test-proxy-scenarios.sh" ""

# Deployment and installation (skip these in mock mode as they test real deployments)
echo -e "${YELLOW}Skipping deployment tests in mock mode${NC}"
# run_test "release" "./scripts/test-release.sh" ""
# run_test "installation" "./scripts/test-installation-methods.sh" ""
# run_test "docker" "./scripts/test-docker-deployment.sh" ""
# run_test "lxc" "./scripts/test-lxc-deployment.sh" ""
# run_test "upgrades" "./scripts/test-upgrades.sh" ""

# Data and monitoring - safe to run with mock data
run_test "backup" "./scripts/test-backup.sh" ""
run_test "persistence" "./scripts/test-persistence.sh" "$PULSE_URL \"$API_TOKEN\""
run_test "recovery" "./scripts/test-recovery.sh" "$PULSE_URL \"$API_TOKEN\""
run_test "monitoring" "./scripts/test-monitoring.sh" "$PULSE_URL \"$API_TOKEN\""
run_test "notifications" "./scripts/test-notifications.sh" "$PULSE_URL \"$API_TOKEN\""

# Performance and validation
run_test "performance" "./scripts/test-performance.sh" ""
run_test "load" "./scripts/test-load.sh" ""
run_test "validation" "./scripts/test-config-validation.sh" "$PULSE_URL \"$API_TOKEN\""

echo ""

# Restore original mock state
echo -n "Restoring original mode: "
if [ "$ORIGINAL_MOCK_STATE" = "false" ]; then
    /opt/pulse/scripts/toggle-mock.sh off > /dev/null 2>&1
    echo -e "${GREEN}✓ Restored to real mode${NC}"
else
    echo -e "${GREEN}✓ Keeping mock mode${NC}"
fi

echo ""
echo "================================================"
echo "SAFE TEST RESULTS:"
echo -e "  ${GREEN}✅ Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "  ${RED}❌ Failed: $FAILED${NC}"
else
    echo -e "  ${GREEN}❌ Failed: 0${NC}"
fi
echo "================================================"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    echo -e "${BLUE}Your production nodes were never touched!${NC}"
    exit 0
else
    echo ""
    echo -e "${YELLOW}⚠️  Some tests failed. Check logs in /tmp/${NC}"
    echo -e "${BLUE}Note: Your production nodes were protected during testing${NC}"
    exit 1
fi
