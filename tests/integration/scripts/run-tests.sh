#!/bin/bash
#
# Run update integration tests with different configurations
# Usage: ./run-tests.sh [test-suite]
#   test-suite: all, happy, checksums, rate-limit, network, stale, frontend
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TEST_SUITE="${1:-all}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "==================================="
echo "Pulse Update Integration Tests"
echo "==================================="
echo ""

cd "$TEST_ROOT"

# Function to run test with specific config
run_test() {
    local name="$1"
    local file="$2"
    local checksum_error="${3:-false}"
    local network_error="${4:-false}"
    local rate_limit="${5:-false}"
    local stale_release="${6:-false}"

    echo ""
    echo -e "${YELLOW}Running: $name${NC}"
    echo "-----------------------------------"

    # Set environment variables
    export MOCK_CHECKSUM_ERROR="$checksum_error"
    export MOCK_NETWORK_ERROR="$network_error"
    export MOCK_RATE_LIMIT="$rate_limit"
    export MOCK_STALE_RELEASE="$stale_release"

    # Start services
    echo "Starting test environment..."
    docker-compose -f docker-compose.test.yml up -d

    # Wait for services
    echo "Waiting for services to be ready..."
    sleep 15

    # Check if services are healthy
    if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${RED}❌ Services failed to start${NC}"
        docker-compose -f docker-compose.test.yml logs
        docker-compose -f docker-compose.test.yml down -v
        return 1
    fi

    # Run tests
    echo "Running tests..."
    if npx playwright test "$file" --reporter=list; then
        echo -e "${GREEN}✅ $name passed${NC}"
        TEST_RESULT=0
    else
        echo -e "${RED}❌ $name failed${NC}"
        TEST_RESULT=1
    fi

    # Cleanup
    echo "Cleaning up..."
    docker-compose -f docker-compose.test.yml down -v

    return $TEST_RESULT
}

# Run specific test suite or all tests
FAILED_TESTS=()

case "$TEST_SUITE" in
    all)
        echo "Running all test suites..."

        run_test "Happy Path" "tests/01-happy-path.spec.ts" || FAILED_TESTS+=("Happy Path")
        run_test "Bad Checksums" "tests/02-bad-checksums.spec.ts" "true" || FAILED_TESTS+=("Bad Checksums")
        run_test "Rate Limiting" "tests/03-rate-limiting.spec.ts" "false" "false" "true" || FAILED_TESTS+=("Rate Limiting")
        run_test "Network Failures" "tests/04-network-failure.spec.ts" "false" "true" || FAILED_TESTS+=("Network Failures")
        run_test "Stale Releases" "tests/05-stale-release.spec.ts" "false" "false" "false" "true" || FAILED_TESTS+=("Stale Releases")
        run_test "Frontend Validation" "tests/06-frontend-validation.spec.ts" || FAILED_TESTS+=("Frontend Validation")
        ;;

    happy)
        run_test "Happy Path" "tests/01-happy-path.spec.ts" || FAILED_TESTS+=("Happy Path")
        ;;

    checksums)
        run_test "Bad Checksums" "tests/02-bad-checksums.spec.ts" "true" || FAILED_TESTS+=("Bad Checksums")
        ;;

    rate-limit)
        run_test "Rate Limiting" "tests/03-rate-limiting.spec.ts" "false" "false" "true" || FAILED_TESTS+=("Rate Limiting")
        ;;

    network)
        run_test "Network Failures" "tests/04-network-failure.spec.ts" "false" "true" || FAILED_TESTS+=("Network Failures")
        ;;

    stale)
        run_test "Stale Releases" "tests/05-stale-release.spec.ts" "false" "false" "false" "true" || FAILED_TESTS+=("Stale Releases")
        ;;

    frontend)
        run_test "Frontend Validation" "tests/06-frontend-validation.spec.ts" || FAILED_TESTS+=("Frontend Validation")
        ;;

    *)
        echo "Unknown test suite: $TEST_SUITE"
        echo "Available suites: all, happy, checksums, rate-limit, network, stale, frontend"
        exit 1
        ;;
esac

# Summary
echo ""
echo "==================================="
echo "Test Summary"
echo "==================================="

if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed:${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo -e "${RED}  - $test${NC}"
    done
    exit 1
fi
