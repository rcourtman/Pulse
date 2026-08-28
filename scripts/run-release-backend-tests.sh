#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: run-release-backend-tests.sh [options]

Options:
  --data-root DIR     Isolated test-data root (required)
  --api-shards VALUE  auto or a positive integer (default: auto)
  --batch-size VALUE  Top-level tests per API test-binary invocation (default: 10000)
  --max-regex-bytes VALUE
                      Maximum encoded -test.run regex bytes (default: 120000)
  --memory-wait-seconds VALUE
                      Bounded wait for multi-shard memory admission (default: 120)
  --api-shard-timeout VALUE
                      Cumulative watchdog for each API shard (default: 45m)
EOF
}

DATA_ROOT=""
API_SHARDS="${PULSE_BACKEND_TEST_SHARDS:-auto}"
BATCH_SIZE=10000
MAX_REGEX_BYTES="${PULSE_BACKEND_TEST_MAX_REGEX_BYTES:-120000}"
MEMORY_WAIT_SECONDS="${PULSE_BACKEND_TEST_MEMORY_WAIT_SECONDS:-120}"
API_SHARD_TIMEOUT="${PULSE_BACKEND_API_SHARD_TIMEOUT:-45m}"
SHARD_WEIGHTS="${PULSE_BACKEND_TEST_SHARD_WEIGHTS:-}"
SHARD_BOUNDARIES="${PULSE_BACKEND_TEST_SHARD_BOUNDARIES:-}"

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
    --max-regex-bytes)
      MAX_REGEX_BYTES="${2:-}"
      shift 2
      ;;
    --memory-wait-seconds)
      MEMORY_WAIT_SECONDS="${2:-}"
      shift 2
      ;;
    --api-shard-timeout)
      API_SHARD_TIMEOUT="${2:-}"
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
if [[ ! "$MAX_REGEX_BYTES" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: --max-regex-bytes must be a positive integer." >&2
  exit 2
fi
if [ "$MAX_REGEX_BYTES" -gt 120000 ]; then
  echo "Error: --max-regex-bytes cannot exceed the safe 120000-byte per-argument ceiling." >&2
  exit 2
fi
if [[ ! "$MEMORY_WAIT_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "Error: --memory-wait-seconds must be a non-negative integer." >&2
  exit 2
fi
if [[ ! "$API_SHARD_TIMEOUT" =~ ^[1-9][0-9]*(ms|s|m|h)$ ]]; then
  echo "Error: --api-shard-timeout must be a positive Go duration using ms, s, m, or h." >&2
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
read_available_kib() {
  awk '/^MemAvailable:/ { print $2; exit }' /proc/meminfo
}

AVAILABLE_KIB="$(read_available_kib)"
if [ -z "$AVAILABLE_KIB" ]; then AVAILABLE_KIB=0; fi

if [ "$API_SHARDS" = auto ]; then
  if [ "$VCPUS" -ge 8 ]; then
    cpu_shards=3
  elif [ "$VCPUS" -ge 4 ]; then
    cpu_shards=2
  else
    cpu_shards=1
  fi

  # A release starts the public and private credential-free compilers beside
  # this lane on the same worker VM. Their short peak must not permanently
  # collapse an otherwise capable 8-vCPU worker to a long API shard. Wait only
  # while those useful jobs finish, then admit the widest shard count the
  # measured headroom supports instead of failing the release at admission.
  # A 2026-08-21 direct probe on the 8-vCPU/18-GiB PVE worker measured a
  # ~7.5 GiB gate footprint for three race shards plus the concurrent non-API
  # package graph (8.9 GiB MemAvailable floor from a 16.4 GiB idle start, zero
  # swap), so these thresholds keep >2x margin while remaining reachable
  # beside the compile peak (~14.1 GiB observed mid-release). The previous
  # 16 GiB requirement exceeded the worker's own idle availability (16.1 GiB)
  # and would have hard-failed the next release at admission.
  shard_admission_required_kib() {
    case "$1" in
      2) echo $((8 * 1024 * 1024)) ;;
      *) echo $((10 * 1024 * 1024)) ;;
    esac
  }
  if [ "$cpu_shards" -gt 1 ]; then
    required_kib="$(shard_admission_required_kib "$cpu_shards")"
    waited_seconds=0
    while [ "$AVAILABLE_KIB" -lt "$required_kib" ] && [ "$waited_seconds" -lt "$MEMORY_WAIT_SECONDS" ]; do
      remaining_seconds=$((MEMORY_WAIT_SECONDS - waited_seconds))
      wait_seconds=5
      if [ "$remaining_seconds" -lt "$wait_seconds" ]; then wait_seconds="$remaining_seconds"; fi
      echo "Waiting ${wait_seconds}s for backend shard admission: $((AVAILABLE_KIB / 1024)) MiB available, $((required_kib / 1024)) MiB required."
      sleep "$wait_seconds"
      waited_seconds=$((waited_seconds + wait_seconds))
      AVAILABLE_KIB="$(read_available_kib)"
      if [ -z "$AVAILABLE_KIB" ]; then AVAILABLE_KIB=0; fi
    done
    while [ "$cpu_shards" -gt 1 ] && [ "$AVAILABLE_KIB" -lt "$(shard_admission_required_kib "$cpu_shards")" ]; do
      cpu_shards=$((cpu_shards - 1))
      echo "Degrading to $cpu_shards API shard(s): $((AVAILABLE_KIB / 1024)) MiB available after the ${MEMORY_WAIT_SECONDS}s bounded wait."
    done
  fi
  API_SHARDS="$cpu_shards"
elif [[ ! "$API_SHARDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: --api-shards must be auto or a positive integer." >&2
  exit 2
fi
if [ "$API_SHARDS" -gt "$VCPUS" ]; then
  echo "Error: API shard count ($API_SHARDS) cannot exceed available vCPUs ($VCPUS)." >&2
  exit 2
fi

if [ -n "$SHARD_WEIGHTS" ] && [ -n "$SHARD_BOUNDARIES" ]; then
  echo "Error: configure shard weights or shard boundaries, not both." >&2
  exit 2
fi
if [ -z "$SHARD_WEIGHTS" ] && [ -z "$SHARD_BOUNDARIES" ] && [ "$API_SHARDS" -eq 3 ]; then
  # RC.6 exact-SHA bisection found the repeated integration-server cost in the
  # final 31 tests. Keep the fast prefix together, then divide that hot tail
  # across the remaining two processes at stable top-level test boundaries.
  SHARD_BOUNDARIES="TestWebSocketOriginAllowsTrustedForwardedHostedOriginIPv6Loopback,TestServerInfoEndpointMethodNotAllowed"
fi

echo "Release backend test plan"
echo "  vCPUs:        $VCPUS"
echo "  Available MiB: $((AVAILABLE_KIB / 1024))"
echo "  API shards:   $API_SHARDS"
echo "  Shard weights: ${SHARD_WEIGHTS:-none}"
echo "  Shard boundaries: ${SHARD_BOUNDARIES:-automatic}"
echo "  Memory wait:  ${MEMORY_WAIT_SECONDS}s max"
echo "  API watchdog: $API_SHARD_TIMEOUT per shard"
echo "  Batch size:   $BATCH_SIZE"
echo "  Regex bytes:  $MAX_REGEX_BYTES max"

./scripts/ensure_test_assets.sh

echo "Compiling the race-enabled internal/api test binary once..."
go test -c -race -o "$API_BINARY" ./internal/api
(
  cd internal/api
  "$API_BINARY" -test.list '^Test'
) | awk '/^Test[A-Za-z0-9_]+$/ { print }' > "$TEST_NAMES_FILE"

plan_args=(
  --tests-file "$TEST_NAMES_FILE"
  --shards "$API_SHARDS"
  --batch-size "$BATCH_SIZE"
  --max-regex-bytes "$MAX_REGEX_BYTES"
  --output-dir "$PLAN_DIR"
)
if [ -n "$SHARD_WEIGHTS" ]; then
  plan_args+=(--shard-weights "$SHARD_WEIGHTS")
fi
if [ -n "$SHARD_BOUNDARIES" ]; then
  plan_args+=(--shard-boundaries "$SHARD_BOUNDARIES")
fi
python3 scripts/shard_go_tests.py "${plan_args[@]}"

# Partition the worker CPU budget between the API shards and the concurrently
# running non-API package graph. Top-level API tests execute serially inside
# one test-binary process, so a shard's wall time tracks the serial duration
# of its range, not its CPU width — but the runtime, GC, and race detector
# behind the canonical prefix shard's thousands of unit tests still benefit
# from width the ~15-test wait-bound tail shards cannot use.
#
# Direct 2026-08-21 worker probes measured the prefix shard at 569s with 2
# procs and 484s with 4. RC.10 exposed that allocating all eight worker CPUs
# to API shards while also launching the non-API graph left that graph's Go
# package concurrency unbounded. Reserve two package slots where possible,
# cap each non-API test binary to one CPU, and keep the combined declared
# width at the worker's vCPU count.
mapfile -t CPU_PLAN < <(python3 - "$PLAN_DIR/manifest.json" "$VCPUS" <<'PY'
import json
import sys

sys.path.insert(0, "scripts")
from shard_go_tests import allocate_cpu_plan

manifest = json.load(open(sys.argv[1]))
vcpus = int(sys.argv[2])
counts = [shard["test_count"] for shard in manifest["shards"]]
widths, reserved_other = allocate_cpu_plan(counts, vcpus)
print(reserved_other)
print("\n".join(str(width) for width in widths))
PY
)
RESERVED_OTHER_PACKAGE_PROCS="${CPU_PLAN[0]}"
OTHER_PACKAGE_PROCS="$RESERVED_OTHER_PACKAGE_PROCS"
if [ "$OTHER_PACKAGE_PROCS" -eq 0 ]; then OTHER_PACKAGE_PROCS=1; fi
SHARD_GOMAXPROCS=("${CPU_PLAN[@]:1}")
if [ "${#SHARD_GOMAXPROCS[@]}" -ne "$API_SHARDS" ]; then
  echo "Error: shard GOMAXPROCS plan produced ${#SHARD_GOMAXPROCS[@]} entries for $API_SHARDS shards." >&2
  exit 4
fi
echo "API shard GOMAXPROCS: ${SHARD_GOMAXPROCS[*]}"
echo "Non-API package workers: $OTHER_PACKAGE_PROCS (GOMAXPROCS=1 each)"

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
  env GOMAXPROCS=1 PULSE_DATA_DIR="$RUN_ROOT/data/other" \
    go test -race -p "$OTHER_PACKAGE_PROCS" -timeout 30m "${OTHER_PACKAGES[@]}"
}

run_api_shard() {
  local shard_index="$1"
  local shard_procs="$2"
  local shard_dir="$RUN_ROOT/data/api-shard-$shard_index"
  local regex_file regex batch_index=0
  local shard_started_seconds="$SECONDS"
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
        GOMAXPROCS="$shard_procs" \
        PULSE_DATA_DIR="$shard_dir/batch-$batch_index" \
        "$API_BINARY" -test.run "$regex" -test.timeout "$API_SHARD_TIMEOUT"
    )
  done
  echo "Completed internal/api shard $shard_index in $((SECONDS - shard_started_seconds))s."
}

pids=()
# A one-vCPU host cannot reserve a concurrent package slot. Complete the
# non-API graph first there; release workers have enough width for concurrent
# execution through the normal path below.
if [ "$RESERVED_OTHER_PACKAGE_PROCS" -eq 0 ]; then
  run_other_packages
else
  run_other_packages &
  pids+=("$!")
fi
for shard_number in $(seq 1 "$API_SHARDS"); do
  shard_index="$(printf '%02d' "$shard_number")"
  run_api_shard "$shard_index" "${SHARD_GOMAXPROCS[$((shard_number - 1))]}" &
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
