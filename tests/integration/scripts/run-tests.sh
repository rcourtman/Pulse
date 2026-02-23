#!/bin/bash
#
# Run Pulse integration tests with different suites
# Usage: ./run-tests.sh [suite]
#   suite: all, core, diagnostic, perf, visual, multi-tenant, trial, cloud-hosting, cloud-lifecycle, evals, updates-api
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
REPO_ROOT="$(cd "$TEST_ROOT/../.." && pwd)"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD=(docker compose)
else
    COMPOSE_CMD=(docker-compose)
fi

compose() {
    "${COMPOSE_CMD[@]}" -f docker-compose.test.yml "$@"
}

ensure_test_images() {
    if ! docker image inspect pulse-mock-github:test >/dev/null 2>&1; then
        echo "Building missing image: pulse-mock-github:test"
        docker build -t pulse-mock-github:test "$TEST_ROOT/mock-github-server"
    fi

    if ! docker image inspect pulse:test >/dev/null 2>&1; then
        echo "Building missing image: pulse:test"
        docker build -t pulse:test -f "$REPO_ROOT/Dockerfile" "$REPO_ROOT"
    fi
}

# Function to run suite with specific mock config
run_suite() {
    local name="$1"
    local suite="$2"
    local checksum_error="${3:-false}"
    local network_error="${4:-false}"
    local rate_limit="${5:-false}"
    local stale_release="${6:-false}"
    local multi_tenant_enabled="${7:-false}"

    echo ""
    echo -e "${YELLOW}Running: $name${NC}"
    echo "-----------------------------------"

    # Set environment variables
    export MOCK_CHECKSUM_ERROR="$checksum_error"
    export MOCK_NETWORK_ERROR="$network_error"
    export MOCK_RATE_LIMIT="$rate_limit"
    export MOCK_STALE_RELEASE="$stale_release"
    export PULSE_MULTI_TENANT_ENABLED="$multi_tenant_enabled"
    local pulse_base_url="${PULSE_BASE_URL:-http://localhost:${PULSE_E2E_PORT:-7655}}"
    pulse_base_url="${pulse_base_url%/}"
    local health_url="${pulse_base_url}/api/health"

    # Start services
    echo "Starting test environment..."
    if ! compose up -d; then
        echo -e "${RED}❌ Failed to start docker services${NC}"
        compose logs
        compose down -v
        return 1
    fi

    # Wait for services
    echo "Waiting for services to be ready..."
    local health_ok=0
    for i in {1..60}; do
        if curl -fsS "$health_url" >/dev/null 2>&1; then
            health_ok=1
            break
        fi
        sleep 1
    done

    # Check if the Pulse test container is actually running and reachable.
    local pulse_running
    pulse_running="$(docker inspect -f '{{.State.Running}}' pulse-test-server 2>/dev/null || true)"
    if [ "$health_ok" -ne 1 ] || [ "$pulse_running" != "true" ]; then
        echo -e "${RED}❌ Services failed to start${NC}"
        compose ps
        compose logs
        compose down -v
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
            PULSE_E2E_PERF=1 npx playwright test "tests/02-navigation-perf.spec.ts" --project=chromium --reporter=list
            ;;
        visual)
            npx playwright test "tests/06-theme-visual.spec.ts" --project=chromium --reporter=list
            ;;
        multi-tenant)
            npx playwright test "tests/03-multi-tenant.spec.ts" --project=chromium --reporter=list
            ;;
        trial)
            npx playwright test "tests/07-trial-signup-return.spec.ts" --project=chromium --reporter=list
            ;;
        cloud-hosting)
            npx playwright test "tests/08-cloud-hosting.spec.ts" --project=chromium --reporter=list
            ;;
        cloud-lifecycle)
            npx playwright test "tests/09-cloud-billing-lifecycle.spec.ts" --project=chromium --reporter=list
            ;;
        evals)
            node ./scripts/run-evals.mjs --mode deterministic
            ;;
        updates-api)
            (
                cd "$REPO_ROOT"
                UPDATE_API_BASE_URL="$pulse_base_url" \
                go test ./internal/api -run 'TestHandleCheckUpdates|TestHandleApplyUpdate|TestHandleUpdateStatus' -count=1
            )
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
    compose down -v

    return $TEST_RESULT
}

# Run specific test suite or all tests
FAILED_TESTS=()
ensure_test_images

case "$SUITE" in
    all)
        echo "Running all suites..."
        run_suite "Diagnostic Smoke" "diagnostic" || FAILED_TESTS+=("Diagnostic Smoke")
        run_suite "Core E2E" "core" || FAILED_TESTS+=("Core E2E")
        run_suite "Multi-tenant E2E" "multi-tenant" "false" "false" "false" "false" "true" || FAILED_TESTS+=("Multi-tenant E2E")
        run_suite "Trial Signup E2E" "trial" || FAILED_TESTS+=("Trial Signup E2E")
        run_suite "Cloud Hosting E2E" "cloud-hosting" || FAILED_TESTS+=("Cloud Hosting E2E")
        run_suite "Cloud Billing Lifecycle E2E" "cloud-lifecycle" || FAILED_TESTS+=("Cloud Billing Lifecycle E2E")
        run_suite "Navigation Performance" "perf" || FAILED_TESTS+=("Navigation Performance")
        run_suite "Theme Visual Regression" "visual" || FAILED_TESTS+=("Theme Visual Regression")
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

    visual)
        run_suite "Theme Visual Regression" "visual" || FAILED_TESTS+=("Theme Visual Regression")
        ;;

    multi-tenant)
        run_suite "Multi-tenant E2E" "multi-tenant" "false" "false" "false" "false" "true" || FAILED_TESTS+=("Multi-tenant E2E")
        ;;

    trial)
        run_suite "Trial Signup E2E" "trial" || FAILED_TESTS+=("Trial Signup E2E")
        ;;

    cloud-hosting)
        run_suite "Cloud Hosting E2E" "cloud-hosting" || FAILED_TESTS+=("Cloud Hosting E2E")
        ;;

    cloud-lifecycle)
        run_suite "Cloud Billing Lifecycle E2E" "cloud-lifecycle" || FAILED_TESTS+=("Cloud Billing Lifecycle E2E")
        ;;

    evals)
        run_suite "Agentic Eval Pack (Deterministic)" "evals" || FAILED_TESTS+=("Agentic Eval Pack (Deterministic)")
        ;;

    updates-api)
        run_suite "Update API Integration" "updates-api" || FAILED_TESTS+=("Update API Integration")
        ;;

    *)
        echo "Unknown suite: $SUITE"
        echo "Available suites: all, diagnostic, core, perf, visual, multi-tenant, trial, cloud-hosting, cloud-lifecycle, evals, updates-api"
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
