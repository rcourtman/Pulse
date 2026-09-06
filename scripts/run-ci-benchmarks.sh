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

# Retain provenance without dumping credentials, remote URLs or working paths.
# PR checkouts may be synthetic merges; Go selects a toolchain for each tree.
metadata="${CURRENT_DIR}/bench-metadata.txt"
revision() {
  # An archived tree nested inside another checkout must not inherit its SHA.
  local prefix
  prefix="$(git -C "$1" rev-parse --show-prefix 2>/dev/null)" || { echo unavailable; return; }
  [[ -z "${prefix}" ]] || { echo unavailable; return; }
  git -C "$1" rev-parse --verify "$2" 2>/dev/null || echo unavailable
}
{
  printf 'started_at=%s\nsamples=%s\nbenchtime=%s\n' \
    "$(date -u +%FT%TZ)" "${SAMPLE_COUNT}" "${BENCHTIME}"
  printf 'platform=%s\n' "$(uname -sm)"
  printf 'gomaxprocs=%s\n' "${GOMAXPROCS:-runtime-default}"
  printf 'packages=%s\n' "${PACKAGES[*]}"
  for label in candidate baseline; do
    tree="${CURRENT_DIR}"
    [[ "${label}" != baseline ]] || tree="${BASELINE_DIR}"
    [[ -n "${tree}" ]] || continue
    printf '%s.commit=%s\n' "${label}" "$(revision "${tree}" HEAD)"
    printf '%s.tree=%s\n' "${label}" "$(revision "${tree}" 'HEAD^{tree}')"
    printf '%s.go=%s\n' "${label}" "$(cd "${tree}" && go version)"
  done
} > "${metadata}"

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

# Hash the actual executable Go is about to run, not a later reconstruction.
# Keep hashing outside benchmark timing and never log binary paths or arguments.
binary_wrapper="${work_dir}/record-binary"
cat > "${binary_wrapper}" <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
digest="$(sha256sum -- "$1")"
digest="${digest%% *}"
printf 'binary=%s,%s,%s,%s\n' "${PULSE_BENCH_LABEL}" \
  "${PULSE_BENCH_ROUND}" "${1##*/}" "${digest}" >> "${PULSE_BENCH_METADATA}"
exec "$@"
WRAPPER
chmod +x "${binary_wrapper}"
export PULSE_BENCH_METADATA="${metadata}"

run_sample() {
  local tree="$1"
  local output="$2"
  local label="$3"
  local round="$4"
  local data_dir="${work_dir}/${label}-${round}"

  mkdir -p "${data_dir}"
  printf 'sample=%s,%s,%s\n' "${label}" "${round}" "$(date -u +%FT%TZ)" >> "${metadata}"
  echo "=== ${label} benchmark sample ${round}/${SAMPLE_COUNT} ==="
  (
    cd "${tree}"
    PULSE_BENCH_LABEL="${label}" PULSE_BENCH_ROUND="${round}" \
    PULSE_DATA_DIR="${data_dir}" go test -exec "${binary_wrapper}" \
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
    PULSE_BENCH_LABEL=candidate PULSE_BENCH_ROUND=unpaired \
    PULSE_DATA_DIR="${data_dir}" go test -exec "${binary_wrapper}" \
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
