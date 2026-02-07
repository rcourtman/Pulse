#!/bin/bash
#
# Run Pulse integration tests with different suites
# Usage: ./run-tests.sh [suite]
#   suite: all, core, diagnostic, perf, updates-api
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SUITE="${1:-all}"

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

# Function to run suite with specific mock config
run_suite() {
    local name="$1"
    local suite="$2"
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
    for i in {1..60}; do
        if curl -fsS "http://localhost:7655/api/health" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done

    # Check if services are healthy
    if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${RED}❌ Services failed to start${NC}"
        docker-compose -f docker-compose.test.yml logs
        docker-compose -f docker-compose.test.yml down -v
        return 1
    fi

    # Run tests
    echo "Running tests..."
    set +e
    case "$suite" in
        diagnostic)
            npx playwright test "tests/00-diagnostic.spec.ts" --reporter=list
            ;;
        core)
            npx playwright test "tests/01-core-e2e.spec.ts" --reporter=list
            ;;
        perf)
            PULSE_E2E_PERF=1 npx playwright test "tests/02-navigation-perf.spec.ts" --reporter=list
            ;;
        updates-api)
            UPDATE_API_BASE_URL=http://localhost:7655 go test ./api -run TestUpdateFlowIntegration -count=1
            ;;
        *)
            echo "Unknown suite: $suite"
            set -e
            return 1
            ;;
    esac
    TEST_RESULT=$?
    set -e

    if [ $TEST_RESULT -eq 0 ]; then
        echo -e "${GREEN}✅ $name passed${NC}"
    else
        echo -e "${RED}❌ $name failed${NC}"
    fi

    # Cleanup
    echo "Cleaning up..."
    docker-compose -f docker-compose.test.yml down -v

    return $TEST_RESULT
}

# Run specific test suite or all tests
FAILED_TESTS=()

case "$SUITE" in
    all)
        echo "Running all suites..."
        run_suite "Diagnostic Smoke" "diagnostic" || FAILED_TESTS+=("Diagnostic Smoke")
        run_suite "Core E2E" "core" || FAILED_TESTS+=("Core E2E")
        run_suite "Navigation Performance" "perf" || FAILED_TESTS+=("Navigation Performance")
        run_suite "Update API Integration" "updates-api" || FAILED_TESTS+=("Update API Integration")
        ;;

    diagnostic)
        run_suite "Diagnostic Smoke" "diagnostic" || FAILED_TESTS+=("Diagnostic Smoke")
        ;;

    core)
        run_suite "Core E2E" "core" || FAILED_TESTS+=("Core E2E")
        ;;

    perf)
        run_suite "Navigation Performance" "perf" || FAILED_TESTS+=("Navigation Performance")
        ;;

    updates-api)
        run_suite "Update API Integration" "updates-api" || FAILED_TESTS+=("Update API Integration")
        ;;

    *)
        echo "Unknown suite: $SUITE"
        echo "Available suites: all, diagnostic, core, perf, updates-api"
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
