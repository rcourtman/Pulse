#!/bin/bash
#
# Run Pulse integration tests with different suites
# Usage: ./run-tests.sh [suite]
#   suite: all, core, diagnostic, perf, visual, multi-tenant, retired-trial-acquisition, cloud-hosting, cloud-lifecycle, demo-contract, evals, updates-api
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

# Unique per invocation, including simultaneous runs of the same checkout.
# Never inherit another runner's project, image tags or fixed host ports.
export PULSE_E2E_RUN_ID="pulse-e2e-$(node -e "process.stdout.write(require('crypto').randomBytes(16).toString('hex'))")"
export PULSE_E2E_SERVER_IMAGE="pulse:$PULSE_E2E_RUN_ID"
export PULSE_E2E_MOCK_IMAGE="pulse-mock-github:$PULSE_E2E_RUN_ID"
export PULSE_E2E_SERVER_CONTAINER="$PULSE_E2E_RUN_ID-server"
export PULSE_E2E_MOCK_CONTAINER="$PULSE_E2E_RUN_ID-mock"
export PULSE_E2E_SEED_CONTAINER="$PULSE_E2E_RUN_ID-seed"
export PULSE_E2E_PORT=0 PULSE_E2E_AGENT_PORT=0 PULSE_E2E_MOCK_GITHUB_PORT=0
echo "Isolated integration project: $PULSE_E2E_RUN_ID"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD=(docker compose)
else
    COMPOSE_CMD=(docker-compose)
fi

compose() {
    "${COMPOSE_CMD[@]}" -p "$PULSE_E2E_RUN_ID" -f docker-compose.test.yml "$@"
}

ensure_test_images() {
    # A local tag may belong to another checkout. Always build from these
    # sources; Docker can reuse unchanged layers, but tag existence is not
    # evidence that the image includes the code we are qualifying.
    echo "Building test images from: $REPO_ROOT"
    docker build -t "$PULSE_E2E_MOCK_IMAGE" "$TEST_ROOT/mock-github-server"

    # Test image drops the release build tag so the suite can enable mock
    # fixtures without a demo entitlement. A build failure must stop the run,
    # never fall back to a previously tagged image.
    docker build -t "$PULSE_E2E_SERVER_IMAGE" --build-arg GO_BUILD_TAGS="" -f "$REPO_ROOT/Dockerfile" "$REPO_ROOT"
}

# EXIT also handles build/start/test failures; signals preserve failure status.
cleanup() {
    compose down -v || true
    docker image rm "$PULSE_E2E_SERVER_IMAGE" "$PULSE_E2E_MOCK_IMAGE" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

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
    if [ "$multi_tenant_enabled" = "true" ]; then
        export PULSE_E2E_ENTITLEMENT_PROFILE="multi-tenant"
    else
        unset PULSE_E2E_ENTITLEMENT_PROFILE
    fi
    export PULSE_E2E_PORT=0 PULSE_E2E_AGENT_PORT=0 PULSE_E2E_MOCK_GITHUB_PORT=0

    # Start services
    echo "Starting test environment..."
    if ! compose up -d; then
        echo -e "${RED}❌ Failed to start docker services${NC}"
        compose logs
        compose down -v
        return 1
    fi

    # Resolve Docker-allocated ports only after successful startup. A failed
    # bind must never fall back to probing a different worktree's listener.
    local binding
    binding="$(compose port pulse-test 7655)" || return 1
    [[ "$binding" =~ :([0-9]+)$ ]] || return 1
    export PULSE_BASE_URL="http://127.0.0.1:${BASH_REMATCH[1]}"
    local pulse_base_url="$PULSE_BASE_URL"
    local health_url="$pulse_base_url/api/health"
    binding="$(compose port pulse-test 7656)" || return 1
    [[ "$binding" =~ :([0-9]+)$ ]] || return 1
    export PULSE_E2E_AGENT_PORT="${BASH_REMATCH[1]}"
    export PULSE_AGENT_BASE_URL="http://127.0.0.1:${BASH_REMATCH[1]}"
    binding="$(compose port mock-github 8080)" || return 1
    [[ "$binding" =~ :([0-9]+)$ ]] || return 1
    export MOCK_GITHUB_URL="http://127.0.0.1:${BASH_REMATCH[1]}"

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
    pulse_running="$(docker inspect -f '{{.State.Running}}' "$PULSE_E2E_SERVER_CONTAINER" 2>/dev/null || true)"
    if [ "$health_ok" -ne 1 ] || [ "$pulse_running" != "true" ]; then
        echo -e "${RED}❌ Services failed to start${NC}"
        compose ps
        compose logs
        compose down -v
        return 1
    fi

    if ! node ./scripts/apply-entitlement-profile.mjs; then
        echo -e "${RED}❌ Failed to apply entitlement bootstrap${NC}"
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
        retired-trial-acquisition)
            npx playwright test "tests/07-retired-trial-acquisition.spec.ts" --project=chromium --reporter=list
            ;;
        cloud-hosting)
            npx playwright test "tests/08-cloud-hosting.spec.ts" --project=chromium --reporter=list
            ;;
        cloud-lifecycle)
            npx playwright test "tests/09-cloud-billing-lifecycle.spec.ts" --project=chromium --reporter=list
            ;;
        demo-contract)
            npx playwright test "tests/53-demo-mode-commercial-boundary.spec.ts" --project=chromium --project=mobile-chrome --reporter=list
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
        run_suite "Retired Trial Acquisition E2E" "retired-trial-acquisition" || FAILED_TESTS+=("Retired Trial Acquisition E2E")
        run_suite "Cloud Hosting E2E" "cloud-hosting" || FAILED_TESTS+=("Cloud Hosting E2E")
        run_suite "Cloud Billing Lifecycle E2E" "cloud-lifecycle" || FAILED_TESTS+=("Cloud Billing Lifecycle E2E")
        run_suite "Public Demo Contract" "demo-contract" || FAILED_TESTS+=("Public Demo Contract")
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

    retired-trial-acquisition)
        run_suite "Retired Trial Acquisition E2E" "retired-trial-acquisition" || FAILED_TESTS+=("Retired Trial Acquisition E2E")
        ;;

    cloud-hosting)
        run_suite "Cloud Hosting E2E" "cloud-hosting" || FAILED_TESTS+=("Cloud Hosting E2E")
        ;;

    cloud-lifecycle)
        run_suite "Cloud Billing Lifecycle E2E" "cloud-lifecycle" || FAILED_TESTS+=("Cloud Billing Lifecycle E2E")
        ;;

    demo-contract)
        run_suite "Public Demo Contract" "demo-contract" || FAILED_TESTS+=("Public Demo Contract")
        ;;

    evals)
        run_suite "Agentic Eval Pack (Deterministic)" "evals" || FAILED_TESTS+=("Agentic Eval Pack (Deterministic)")
        ;;

    updates-api)
        run_suite "Update API Integration" "updates-api" || FAILED_TESTS+=("Update API Integration")
        ;;

    *)
        echo "Unknown suite: $SUITE"
        echo "Available suites: all, diagnostic, core, perf, visual, multi-tenant, retired-trial-acquisition, cloud-hosting, cloud-lifecycle, demo-contract, evals, updates-api"
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
