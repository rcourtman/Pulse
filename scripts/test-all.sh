#!/bin/bash

# Master test script that runs all test suites
# Run this before any release!

set -e

VERSION=${1:-$(cat VERSION)}
API_TOKEN=${2:-""}
PULSE_URL=${3:-"http://localhost:7655"}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "================================================"
echo -e "${BLUE}PULSE COMPREHENSIVE TEST SUITE v${VERSION}${NC}"
echo "================================================"
echo ""

TOTAL_PASSED=0
TOTAL_FAILED=0
FAILED_SUITES=()

run_test_suite() {
    local suite_name="$1"
    local script_path="$2"
    shift 2
    local args="$@"
    
    echo -e "${YELLOW}Running: $suite_name${NC}"
    echo "----------------------------------------"
    
    if [ -f "$script_path" ]; then
        if timeout 60 $script_path $args 2>&1 | tee /tmp/test_output.txt | grep -E "✓|✗|passed|failed|PASS|FAIL"; then
            # Extract pass/fail counts if possible
            if grep -q "All tests passed" /tmp/test_output.txt; then
                echo -e "${GREEN}✅ $suite_name: PASSED${NC}"
                ((TOTAL_PASSED++))
            elif grep -q "FAILED\|Failed" /tmp/test_output.txt; then
                echo -e "${RED}❌ $suite_name: FAILED${NC}"
                ((TOTAL_FAILED++))
                FAILED_SUITES+=("$suite_name")
            else
                echo -e "${YELLOW}⚠️  $suite_name: COMPLETED (check output)${NC}"
            fi
        else
            echo -e "${RED}❌ $suite_name: ERROR or TIMEOUT${NC}"
            ((TOTAL_FAILED++))
            FAILED_SUITES+=("$suite_name")
        fi
    else
        echo -e "${YELLOW}⚠️  $suite_name: Script not found${NC}"
    fi
    echo ""
}

# Change to Pulse directory
cd /opt/pulse

echo -e "${BLUE}Starting comprehensive test suite...${NC}"
echo ""

# 1. Main release tests
if [ -n "$API_TOKEN" ]; then
    export API_TOKEN
fi
run_test_suite "Release Tests" "./scripts/test-release.sh"

# 2. Edge case tests
run_test_suite "Edge Case Tests" "./scripts/test-edge-cases.sh" "$PULSE_URL"

# 3. Proxy scenario tests
run_test_suite "Proxy Scenarios" "./scripts/test-proxy-scenarios.sh" "$PULSE_URL" "$API_TOKEN"

# 4. Security tests
run_test_suite "Security Tests" "./scripts/test-security.sh" "$PULSE_URL" "$API_TOKEN"

# 5. Installation method tests (non-destructive)
run_test_suite "Installation Tests" "./scripts/test-installation-methods.sh" "$VERSION"

echo "================================================"
echo -e "${BLUE}TEST SUITE SUMMARY${NC}"
echo "================================================"
echo ""
echo -e "Test Suites Passed: ${GREEN}$TOTAL_PASSED${NC}"
echo -e "Test Suites Failed: ${RED}$TOTAL_FAILED${NC}"

if [ ${#FAILED_SUITES[@]} -gt 0 ]; then
    echo ""
    echo -e "${RED}Failed test suites:${NC}"
    for suite in "${FAILED_SUITES[@]}"; do
        echo "  - $suite"
    done
    echo ""
    echo -e "${RED}⚠️  DO NOT RELEASE - Tests failed!${NC}"
    echo ""
    echo "Run individual test suites for details:"
    echo "  ./scripts/test-release.sh"
    echo "  ./scripts/test-edge-cases.sh"
    echo "  ./scripts/test-proxy-scenarios.sh"
    echo "  ./scripts/test-security.sh"
    echo "  ./scripts/test-installation-methods.sh"
    exit 1
else
    echo ""
    echo -e "${GREEN}✅ ALL TEST SUITES PASSED!${NC}"
    echo -e "${GREEN}Safe to proceed with release v${VERSION}${NC}"
fi

echo ""
echo "Additional manual testing recommended:"
echo "  1. Test actual ProxmoxVE LXC installation"
echo "  2. Test Docker deployment on different architectures"
echo "  3. Test behind real nginx/Cloudflare"
echo "  4. Test upgrade from previous version"
echo "  5. Load test with many concurrent users"