#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: run-release-backend-tests.sh [options]

Options:
  --data-root DIR     Isolated test-data root (required)
  --api-shards VALUE  auto or a positive integer (default: auto)
  --batch-size VALUE  Top-level tests per API test-binary invocation (default: 10000)
EOF
}

DATA_ROOT=""
API_SHARDS="${PULSE_BACKEND_TEST_SHARDS:-auto}"
BATCH_SIZE=10000

while [ "$#" -gt 0 ]; do
  case "$1" in
    --data-root)
      DATA_ROOT="${2:-}"
      shift 2
      ;;
    --api-shards)
      API_SHARDS="${2:-}"
      shift 2
      ;;
    --batch-size)
      BATCH_SIZE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ -z "$DATA_ROOT" ]; then
  echo "Error: --data-root is required." >&2
  exit 2
fi
if [[ ! "$BATCH_SIZE" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: --batch-size must be a positive integer." >&2
  exit 2
fi

for command_name in go python3 getconf awk pgrep; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Error: required backend-test command is missing: $command_name" >&2
    exit 3
  fi
done

REPOSITORY_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPOSITORY_ROOT"
# This is the canonical release gate even when exercised directly on the
# worker. Preserve hosted-CI security and isolation semantics in both paths.
export GITHUB_ACTIONS=true
export CI=true
RUN_ROOT="$DATA_ROOT/release-backend-run"
PLAN_DIR="$RUN_ROOT/api-plan"
API_BINARY="$RUN_ROOT/internal-api.test"
TEST_NAMES_FILE="$RUN_ROOT/internal-api-tests.txt"
rm -rf "$RUN_ROOT"
mkdir -p "$PLAN_DIR" "$RUN_ROOT/data/other"

VCPUS="$(getconf _NPROCESSORS_ONLN)"
AVAILABLE_KIB="$(awk '/^MemAvailable:/ { print $2; exit }' /proc/meminfo)"
if [ -z "$AVAILABLE_KIB" ]; then
  AVAILABLE_KIB=0
fi

if [ "$API_SHARDS" = auto ]; then
  cpu_shards=$((VCPUS / 4))
  memory_shards=$(((AVAILABLE_KIB - 2097152) / 4194304))
  if [ "$cpu_shards" -lt 1 ]; then cpu_shards=1; fi
  if [ "$memory_shards" -lt 1 ]; then memory_shards=1; fi
  API_SHARDS="$cpu_shards"
  if [ "$memory_shards" -lt "$API_SHARDS" ]; then API_SHARDS="$memory_shards"; fi
  if [ "$API_SHARDS" -gt 2 ]; then API_SHARDS=2; fi
elif [[ ! "$API_SHARDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: --api-shards must be auto or a positive integer." >&2
  exit 2
fi

echo "Release backend test plan"
echo "  vCPUs:        $VCPUS"
echo "  Available MiB: $((AVAILABLE_KIB / 1024))"
echo "  API shards:   $API_SHARDS"
echo "  Batch size:   $BATCH_SIZE"

./scripts/ensure_test_assets.sh

echo "Compiling the race-enabled internal/api test binary once..."
go test -c -race -o "$API_BINARY" ./internal/api
(
  cd internal/api
  "$API_BINARY" -test.list '^Test'
) | awk '/^Test[A-Za-z0-9_]+$/ { print }' > "$TEST_NAMES_FILE"

python3 scripts/shard_go_tests.py \
  --tests-file "$TEST_NAMES_FILE" \
  --shards "$API_SHARDS" \
  --batch-size "$BATCH_SIZE" \
  --output-dir "$PLAN_DIR"

API_IMPORT_PATH="$(go list -f '{{.ImportPath}}' ./internal/api)"
mapfile -t OTHER_PACKAGES < <(
  go list ./cmd/... ./internal/... ./pkg/... ./scripts/... ./tests/... \
    | awk -v api="$API_IMPORT_PATH" '$0 != api'
)
if [ "${#OTHER_PACKAGES[@]}" -eq 0 ]; then
  echo "Error: no non-API backend test packages were discovered." >&2
  exit 4
fi

run_other_packages() {
  echo "Running ${#OTHER_PACKAGES[@]} non-API packages..."
  env PULSE_DATA_DIR="$RUN_ROOT/data/other" \
    go test -race -timeout 30m "${OTHER_PACKAGES[@]}"
}

run_api_shard() {
  local shard_index="$1"
  local shard_dir="$RUN_ROOT/data/api-shard-$shard_index"
  local regex_file regex batch_index=0
  mkdir -p "$shard_dir"
  shopt -s nullglob
  local regex_files=("$PLAN_DIR"/shard-"$shard_index"-batch-*.regex)
  if [ "${#regex_files[@]}" -eq 0 ]; then
    echo "Error: API shard $shard_index has no batches." >&2
    return 4
  fi
  for regex_file in "${regex_files[@]}"; do
    batch_index=$((batch_index + 1))
    mkdir -p "$shard_dir/batch-$batch_index"
    regex="$(tr -d '\r\n' < "$regex_file")"
    echo "Running internal/api shard $shard_index batch $batch_index/${#regex_files[@]}..."
    (
      cd internal/api
      env \
        GOMAXPROCS="$((VCPUS / API_SHARDS))" \
        PULSE_DATA_DIR="$shard_dir/batch-$batch_index" \
        "$API_BINARY" -test.run "$regex" -test.timeout 30m
    )
  done
}

pids=()
run_other_packages &
pids+=("$!")
for shard_number in $(seq 1 "$API_SHARDS"); do
  shard_index="$(printf '%02d' "$shard_number")"
  run_api_shard "$shard_index" &
  pids+=("$!")
done

remaining="${#pids[@]}"
terminate_tree() {
  local parent_pid="$1"
  local child_pid
  while IFS= read -r child_pid; do
    if [ -n "$child_pid" ]; then
      terminate_tree "$child_pid"
    fi
  done < <(pgrep -P "$parent_pid" || true)
  kill -TERM "$parent_pid" >/dev/null 2>&1 || true
}

while [ "$remaining" -gt 0 ]; do
  if wait -n; then
    remaining=$((remaining - 1))
  else
    status=$?
    for pid in "${pids[@]}"; do
      terminate_tree "$pid"
    done
    wait "${pids[@]}" >/dev/null 2>&1 || true
    exit "$status"
  fi
done

echo "Race-enabled backend release tests passed with $API_SHARDS API shard(s)."
