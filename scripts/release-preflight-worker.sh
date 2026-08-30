#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: release-preflight-worker.sh <40-character-source-sha> <rehearsal|release>

Runs the portable, high-cost release checks for an exact pushed commit in a
dedicated Linux amd64 checkout. The checkout and dependency caches persist
between runs; release credentials and signing keys are neither required nor
accepted.
EOF
}

SOURCE_SHA="${1:-}"
PROFILE="${2:-}"

if [ "$SOURCE_SHA" = "--help" ] || [ "$SOURCE_SHA" = "-h" ]; then
  usage
  exit 0
fi
if [[ ! "$SOURCE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Error: source SHA must be a lowercase 40-character Git commit id." >&2
  exit 2
fi
if [ "$PROFILE" != "rehearsal" ] && [ "$PROFILE" != "release" ]; then
  echo "Error: profile must be rehearsal or release." >&2
  exit 2
fi

WORKER_ROOT="${PULSE_RELEASE_PREFLIGHT_ROOT:-/opt/pulse-release-worker}"
REPOSITORY_URL="${PULSE_RELEASE_PREFLIGHT_REPOSITORY_URL:-https://github.com/rcourtman/Pulse.git}"
REPOSITORY_DIR="${WORKER_ROOT}/repo"
CACHE_DIR="${WORKER_ROOT}/cache"
RECEIPT_DIR="${WORKER_ROOT}/receipts"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-${SOURCE_SHA:0:12}-${PROFILE}"
RUN_DIR="${WORKER_ROOT}/tmp/${RUN_ID}"
GO_TMP_DIR="${RUN_DIR}/go-tmp"
TIMINGS_FILE="${RUN_DIR}/timings.tsv"
TEST_DATA_DIR="${WORKER_ROOT}/test-data/${PROFILE}"

for command_name in git go node npm docker curl flock timeout python3; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Error: required worker command is missing: ${command_name}" >&2
    exit 3
  fi
done
if ! docker compose version >/dev/null 2>&1; then
  echo "Error: Docker Compose v2 is required on the worker." >&2
  exit 3
fi
if [ "$(node -p "process.versions.node.split('.')[0]")" != "24" ]; then
  echo "Error: the worker must use Node.js 24 to match the release workflows." >&2
  exit 3
fi

mkdir -p \
  "$CACHE_DIR/go-build" \
  "$CACHE_DIR/go-mod" \
  "$CACHE_DIR/npm" \
  "$RECEIPT_DIR" \
  "$RUN_DIR" \
  "$GO_TMP_DIR" \
  "$(dirname "$TEST_DATA_DIR")"

exec 9>"${WORKER_ROOT}/worker.lock"
if ! flock -n 9; then
  echo "Error: another release preflight is already using this worker." >&2
  exit 5
fi
WALL_STARTED="$(date +%s)"

# A preflight compiles and tests only. Keep publication and signing authority
# out of the worker even if its login shell happens to define these names.
unset GH_TOKEN GITHUB_TOKEN PULSE_LICENSE_PRIVATE_KEY PULSE_UPDATE_SIGNING_KEY

export GOCACHE="$CACHE_DIR/go-build"
export GOMODCACHE="$CACHE_DIR/go-mod"
export GOTMPDIR="$GO_TMP_DIR"
export npm_config_cache="$CACHE_DIR/npm"
# Match the canonical workflow's isolated single-repository checkout. Tests
# that explicitly require private sibling repositories use this signal to
# apply their documented hosted-CI skip instead of inventing local evidence.
export GITHUB_ACTIONS=true
export CI=true

phase() {
  local name="$1"
  shift
  local started finished
  started="$(date +%s)"
  echo
  echo "==> ${name}"
  "$@"
  finished="$(date +%s)"
  printf '%s\t%s\n' "$name" "$((finished - started))" >> "$TIMINGS_FILE"
  echo "<== ${name}: $((finished - started))s"
}

cleanup() {
  rm -rf "$GO_TMP_DIR"
  if [ -d "$REPOSITORY_DIR/tests/integration" ]; then
    (
      cd "$REPOSITORY_DIR/tests/integration"
      docker compose -f docker-compose.test.yml down -v >/dev/null 2>&1 || true
    )
  fi
}
trap cleanup EXIT

if [ ! -d "$REPOSITORY_DIR/.git" ]; then
  phase clone git clone "$REPOSITORY_URL" "$REPOSITORY_DIR"
fi

phase fetch git -C "$REPOSITORY_DIR" fetch --force --no-tags origin "$SOURCE_SHA"
FETCHED_SHA="$(git -C "$REPOSITORY_DIR" rev-parse 'FETCH_HEAD^{commit}')"
if [ "$FETCHED_SHA" != "$SOURCE_SHA" ]; then
  echo "Error: origin returned ${FETCHED_SHA}, expected ${SOURCE_SHA}." >&2
  exit 4
fi
git -C "$REPOSITORY_DIR" checkout --detach --force "$SOURCE_SHA"
git -C "$REPOSITORY_DIR" clean -ffdx

cd "$REPOSITORY_DIR"
EXPECTED_GO="$(awk '/^toolchain go/ { sub(/^toolchain /, ""); print; exit }' go.mod)"
ACTUAL_GO="$(go env GOVERSION)"
if [ -n "$EXPECTED_GO" ] && [ "$ACTUAL_GO" != "$EXPECTED_GO" ]; then
  echo "Error: worker Go toolchain is ${ACTUAL_GO}; exact-SHA source requires ${EXPECTED_GO}." >&2
  exit 3
fi

phase frontend-dependencies npm --prefix frontend-modern ci
phase frontend-build npm --prefix frontend-modern run build

rm -rf internal/api/frontend-modern
mkdir -p internal/api/frontend-modern
cp -R frontend-modern/dist internal/api/frontend-modern/
run_frontend_static_quality() {
  phase frontend-lint npm --prefix frontend-modern run lint
  phase frontend-headers npm --prefix frontend-modern run lint:headers
  phase frontend-duplication npm --prefix frontend-modern run lint:cpd
  phase frontend-types npm --prefix frontend-modern run type-check
}

run_frontend_tests() {
  phase frontend-tests npm --prefix frontend-modern test
}

run_backend() {
  rm -rf "$TEST_DATA_DIR"
  mkdir -p "$TEST_DATA_DIR"
  if [ "$PROFILE" = "rehearsal" ]; then
    phase backend-serial env PULSE_DATA_DIR="$TEST_DATA_DIR" go test -p 1 ./...
  else
    phase backend-race-sharded ./scripts/run-release-backend-tests.sh \
      --data-root "$TEST_DATA_DIR" \
      --api-shards auto
  fi
}

run_integration_prep() {
  phase integration-dependencies npm --prefix tests/integration ci
  PLAYWRIGHT_VERSION="$(node -p "require('./tests/integration/node_modules/@playwright/test/package.json').version")"
  PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v${PLAYWRIGHT_VERSION}-noble"
  phase playwright-image docker pull "$PLAYWRIGHT_IMAGE"
  phase mock-github-image docker build --tag pulse-mock-github:test tests/integration/mock-github-server
}

# Static frontend checks and integration preparation are bounded enough to
# overlap safely. Keep the full frontend and race-enabled backend test suites
# serial: both saturate this worker, and concurrent execution can turn healthy
# monitoring tests into load-induced release-gate failures.
parallel_pids=()
run_frontend_static_quality &
parallel_pids+=("$!")
run_integration_prep &
parallel_pids+=("$!")

remaining="${#parallel_pids[@]}"
while [ "$remaining" -gt 0 ]; do
  if wait -n; then
    remaining=$((remaining - 1))
  else
    status=$?
    kill "${parallel_pids[@]}" >/dev/null 2>&1 || true
    wait "${parallel_pids[@]}" >/dev/null 2>&1 || true
    exit "$status"
  fi
done

run_frontend_tests
run_backend

PLAYWRIGHT_VERSION="$(node -p "require('./tests/integration/node_modules/@playwright/test/package.json').version")"
PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v${PLAYWRIGHT_VERSION}-noble"

VERSION="$(tr -d '\r\n' < VERSION)"
if [ "$PROFILE" = "rehearsal" ]; then
  phase pulse-image docker build \
    --build-arg "VERSION=${VERSION}" \
    --platform linux/amd64 \
    --target runtime \
    --tag pulse:test \
    .
else
  phase pulse-image docker build \
    --build-arg GO_BUILD_TAGS= \
    --build-arg "VERSION=${VERSION}" \
    --platform linux/amd64 \
    --target e2e_runtime \
    --tag pulse:test \
    .
fi
run_playwright() {
  docker run --rm \
    --network host \
    --ipc host \
    --user "$(id -u):$(id -g)" \
    --env CI=true \
    --env HOME=/tmp \
    --env "PULSE_E2E_DIAGNOSTIC=${PULSE_E2E_DIAGNOSTIC:-}" \
    --volume "$REPOSITORY_DIR/tests/integration:/work" \
    --workdir /work \
    "$PLAYWRIGHT_IMAGE" \
    npx playwright test "$@"
}

run_rehearsal_smoke() {
  cd tests/integration
  export MOCK_CHECKSUM_ERROR=false
  export MOCK_NETWORK_ERROR=false
  export MOCK_RATE_LIMIT=false
  export MOCK_STALE_RELEASE=false
  export PULSE_E2E_DIAGNOSTIC=1
  docker compose -f docker-compose.test.yml up -d --wait
  timeout 60 sh -c 'until curl -fsS http://localhost:7655/api/health >/dev/null; do sleep 2; done'
  run_playwright tests/00-diagnostic.spec.ts --project=chromium --reporter=list
  local status
  status="$(curl -s -o "$RUN_DIR/update-status.json" -w '%{http_code}' http://localhost:7655/api/updates/status || true)"
  case "$status" in
    200|401|403) ;;
    *)
      echo "Unexpected /api/updates/status response: ${status}" >&2
      cat "$RUN_DIR/update-status.json" >&2 || true
      return 1
      ;;
  esac
  docker compose -f docker-compose.test.yml down -v
}

run_release_smoke() {
  cd tests/integration
  export MOCK_CHECKSUM_ERROR=false
  export MOCK_NETWORK_ERROR=false
  export MOCK_RATE_LIMIT=false
  export MOCK_STALE_RELEASE=false
  export PULSE_E2E_BOOTSTRAP_TOKEN=0123456789abcdef0123456789abcdef0123456789abcdef
  docker compose -f docker-compose.test.yml up -d
  timeout 60 sh -c 'until docker inspect --format="{{json .State.Health.Status}}" pulse-mock-github | grep -q healthy; do sleep 2; done'
  timeout 60 sh -c 'until docker inspect --format="{{json .State.Health.Status}}" pulse-test-server | grep -q healthy; do sleep 2; done'
  timeout 60 sh -c 'until curl -fsS http://localhost:7655/api/health >/dev/null; do sleep 2; done'
  run_playwright tests/95-release-smoke.spec.ts --project=chromium --reporter=list
  docker compose -f docker-compose.test.yml down -v
}

if [ "$PROFILE" = "rehearsal" ]; then
  phase rehearsal-smoke run_rehearsal_smoke
else
  phase release-smoke run_release_smoke
fi

FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
WALL_SECONDS="$(($(date +%s) - WALL_STARTED))"
TOTAL_SECONDS="$(awk -F '\t' '{ total += $2 } END { print total + 0 }' "$TIMINGS_FILE")"
RECEIPT_PATH="${RECEIPT_DIR}/${RUN_ID}.json"
{
  printf '{\n'
  printf '  "schema_version": 2,\n'
  printf '  "source_sha": "%s",\n' "$SOURCE_SHA"
  printf '  "profile": "%s",\n' "$PROFILE"
  printf '  "architecture": "%s",\n' "$(uname -m)"
  printf '  "finished_at": "%s",\n' "$FINISHED_AT"
  printf '  "wall_seconds": %s,\n' "$WALL_SECONDS"
  printf '  "total_phase_seconds": %s,\n' "$TOTAL_SECONDS"
  printf '  "result": "success"\n'
  printf '}\n'
} > "$RECEIPT_PATH"

echo
echo "Exact-SHA release preflight passed."
echo "Source SHA: ${SOURCE_SHA}"
echo "Profile: ${PROFILE}"
echo "Wall time: ${WALL_SECONDS}s"
echo "Phase time: ${TOTAL_SECONDS}s"
echo "Receipt: ${RECEIPT_PATH}"
