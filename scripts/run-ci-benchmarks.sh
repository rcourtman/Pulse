#!/usr/bin/env bash
# Run the CI benchmark sample. When a baseline tree is supplied, execute the
# base and candidate on the same host in alternating order so host-to-host
# variance and run order are not mistaken for code regressions.

set -euo pipefail

CURRENT_DIR="${PULSE_BENCH_CURRENT_DIR:-$(pwd)}"
BASELINE_DIR="${PULSE_BENCH_BASELINE_DIR:-}"
SAMPLE_COUNT="${PULSE_BENCH_SAMPLE_COUNT:-10}"
BENCHTIME="${PULSE_BENCH_BENCHTIME:-100ms}"

if ! [[ "${SAMPLE_COUNT}" =~ ^[1-9][0-9]*$ ]]; then
  echo "PULSE_BENCH_SAMPLE_COUNT must be a positive integer." >&2
  exit 2
fi
if [[ ! -d "${CURRENT_DIR}" ]]; then
  echo "Benchmark candidate tree not found: ${CURRENT_DIR}" >&2
  exit 2
fi
if [[ -n "${BASELINE_DIR}" && ! -d "${BASELINE_DIR}" ]]; then
  echo "Benchmark baseline tree not found: ${BASELINE_DIR}" >&2
  exit 2
fi

PACKAGES=(
  ./pkg/metrics/
  ./pkg/auth/
  ./internal/api/
  ./internal/monitoring/
  ./internal/unifiedresources/
  ./internal/dockeragent/
  ./cmd/pulse-agent/
  ./internal/hostagent/
  ./internal/hostmetrics/
)

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

run_sample() {
  local tree="$1"
  local output="$2"
  local label="$3"
  local round="$4"
  local data_dir="${work_dir}/${label}-${round}"

  mkdir -p "${data_dir}"
  echo "=== ${label} benchmark sample ${round}/${SAMPLE_COUNT} ==="
  (
    cd "${tree}"
    PULSE_DATA_DIR="${data_dir}" go test \
      -bench=. -benchmem -count=1 -run='^$' \
      -benchtime="${BENCHTIME}" -timeout=5m \
      "${PACKAGES[@]}"
  ) | tee -a "${output}"
}

run_unpaired() {
  local output="${CURRENT_DIR}/bench-results.txt"
  local data_dir="${work_dir}/candidate"

  mkdir -p "${data_dir}"
  (
    cd "${CURRENT_DIR}"
    PULSE_DATA_DIR="${data_dir}" go test \
      -bench=. -benchmem -count="${SAMPLE_COUNT}" -run='^$' \
      -benchtime="${BENCHTIME}" -timeout=5m \
      "${PACKAGES[@]}"
  ) | tee "${output}"
}

if [[ -z "${BASELINE_DIR}" ]]; then
  run_unpaired
  exit 0
fi

baseline_output="${CURRENT_DIR}/bench-baseline.txt"
candidate_output="${CURRENT_DIR}/bench-results.txt"
: > "${baseline_output}"
: > "${candidate_output}"

# Populate build and filesystem caches without adding a timed sample. This
# avoids consistently charging the first measured tree for cold setup.
for tree in "${BASELINE_DIR}" "${CURRENT_DIR}"; do
  (
    cd "${tree}"
    PULSE_DATA_DIR="${work_dir}/warmup" go test \
      -bench=. -count=1 -run='^$' -benchtime=1x -timeout=5m \
      "${PACKAGES[@]}" >/dev/null
  )
done

for ((round = 1; round <= SAMPLE_COUNT; round++)); do
  if ((round % 2 == 1)); then
    run_sample "${BASELINE_DIR}" "${baseline_output}" baseline "${round}"
    run_sample "${CURRENT_DIR}" "${candidate_output}" candidate "${round}"
  else
    run_sample "${CURRENT_DIR}" "${candidate_output}" candidate "${round}"
    run_sample "${BASELINE_DIR}" "${baseline_output}" baseline "${round}"
  fi
done
