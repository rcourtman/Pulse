#!/usr/bin/env bash
# Run the CI benchmark sample. When a baseline tree is supplied, execute the
# base and candidate on the same host in alternating order so host-to-host
# variance and run order are not mistaken for code regressions.

set -euo pipefail

CURRENT_DIR="${PULSE_BENCH_CURRENT_DIR:-$(pwd)}"
BASELINE_DIR="${PULSE_BENCH_BASELINE_DIR:-}"
SAMPLE_COUNT="${PULSE_BENCH_SAMPLE_COUNT:-10}"
BENCHTIME="${PULSE_BENCH_BENCHTIME:-100ms}"
# Wall-clock seconds a paired run may take, warm-up included; unset means no
# limit. Checked after every round, so a suite that has outgrown its budget
# stops early and names each package's cost instead of meeting a step timeout.
BUDGET_SECONDS="${PULSE_BENCH_BUDGET_SECONDS:-}"

if ! [[ "${SAMPLE_COUNT}" =~ ^[1-9][0-9]*$ ]]; then
  echo "PULSE_BENCH_SAMPLE_COUNT must be a positive integer." >&2
  exit 2
fi
if [[ -n "${BUDGET_SECONDS}" && ! "${BUDGET_SECONDS}" =~ ^[0-9]+$ ]]; then
  echo "PULSE_BENCH_BUDGET_SECONDS must be a whole number of seconds." >&2
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

# An archived tree nested inside another checkout must not inherit its SHA.
is_git_root() {
  local prefix
  prefix="$(git -C "$1" rev-parse --show-prefix 2>/dev/null)" && [[ -z "${prefix}" ]]
}

# Paths whose committed content differs between the two trees. Uncommitted
# edits to tracked files would be measured but not seen, so they fail.
changed_paths() {
  local label tree
  for label in baseline candidate; do
    tree="${CURRENT_DIR}"
    [[ "${label}" != baseline ]] || tree="${BASELINE_DIR}"
    is_git_root "${tree}" || return 1
    [[ -z "$(git -C "${tree}" status --porcelain --untracked-files=no)" ]] || return 1
    git -C "${tree}" ls-tree -r --full-tree HEAD | LC_ALL=C sort > "${work_dir}/${label}.files" ||
      return 1
  done
  LC_ALL=C comm -3 "${work_dir}/baseline.files" "${work_dir}/candidate.files" |
    awk -F '\t' '{ print $NF }' | LC_ALL=C sort -u
}

# Exit 0 when a changed path reaches the package's benchmarks, 1 when none
# does, and 2 when the package graph cannot be read. A test file compiles only
# into its own package's test binary; any other file reaches every package
# that imports its directory, including through embeds and testdata below it.
package_affected() {
  local package="$1" changed="$2" closure="${work_dir}/closure" root
  root="$(cd "${CURRENT_DIR}" && go list -m -f '{{.Dir}}')" || return 2
  (cd "${CURRENT_DIR}" && go list -e -deps -test \
    -f '{{with .Module}}{{if .Main}}{{$.Dir}}{{end}}{{end}}' "${package}") > "${closure}" ||
    return 2
  [[ -s "${closure}" ]] || return 2
  package="${package#./}"
  awk -v root="${root}" -v own="${package%/}" '
    FILENAME == ARGV[1] {
      if ($0 == root) dirs["."]
      else if (index($0, root "/") == 1) dirs[substr($0, length(root) + 2)]
      next
    }
    /_test\.go$/ {
      dir = $0
      if (!sub(/\/[^\/]*$/, "", dir)) dir = "."
      if (dir == own) { hit = 1; exit }
      next
    }
    {
      if ("." in dirs) { hit = 1; exit }
      dir = $0
      while (sub(/\/[^\/]*$/, "", dir)) if (dir in dirs) { hit = 1; exit }
    }
    END { exit hit ? 0 : 1 }
  ' "${closure}" "${changed}"
}

# A paired run measures only packages whose in-module dependency closure the
# candidate changes. Elsewhere both trees compile identical code, so samples
# add runner time and noise but no evidence. Module files, or trees or a
# package graph that cannot be read, select every package.
SELECTED=("${PACKAGES[@]}")
selection="all (no baseline)"
select_packages() {
  local changed="${work_dir}/changed-paths" package status picked=()
  if ! changed_paths > "${changed}"; then
    selection="all (trees are not both clean Git checkouts)"
    return
  fi
  if grep -qxE 'go\.(mod|sum|work|work\.sum)' "${changed}"; then
    selection="all (module files changed)"
    return
  fi
  for package in "${PACKAGES[@]}"; do
    status=0
    package_affected "${package}" "${changed}" || status=$?
    case "${status}" in
      0) picked+=("${package}") ;;
      1) ;;
      *)
        selection="all (package graph unreadable)"
        return
        ;;
    esac
  done
  SELECTED=(${picked[@]+"${picked[@]}"})
  selection="affected"
}
if [[ -n "${BASELINE_DIR}" ]]; then
  select_packages
  echo "Benchmark packages, ${selection}: ${SELECTED[*]:-none}"
fi

# Retain provenance without dumping credentials, remote URLs or working paths.
# PR checkouts may be synthetic merges; Go selects a toolchain for each tree.
metadata="${CURRENT_DIR}/bench-metadata.txt"
revision() {
  is_git_root "$1" || { echo unavailable; return; }
  git -C "$1" rev-parse --verify "$2" 2>/dev/null || echo unavailable
}
{
  printf 'started_at=%s\nsamples=%s\nbenchtime=%s\n' \
    "$(date -u +%FT%TZ)" "${SAMPLE_COUNT}" "${BENCHTIME}"
  printf 'platform=%s\n' "$(uname -sm)"
  printf 'gomaxprocs=%s\n' "${GOMAXPROCS:-runtime-default}"
  printf 'selection=%s\n' "${selection}"
  printf 'packages=%s\n' "${SELECTED[*]:-}"
  for label in candidate baseline; do
    tree="${CURRENT_DIR}"
    [[ "${label}" != baseline ]] || tree="${BASELINE_DIR}"
    [[ -n "${tree}" ]] || continue
    printf '%s.commit=%s\n' "${label}" "$(revision "${tree}" HEAD)"
    printf '%s.tree=%s\n' "${label}" "$(revision "${tree}" 'HEAD^{tree}')"
    printf '%s.go=%s\n' "${label}" "$(cd "${tree}" && go version)"
  done
} > "${metadata}"

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
      "${SELECTED[@]}"
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

if ((${#SELECTED[@]} == 0)); then
  echo "No benchmarked package depends on a changed file; nothing to compare."
  exit 0
fi

# Project the whole run from the rounds measured so far. Stop once it would
# reach the budget, naming each package's mean seconds per sample.
check_budget() {
  local round="$1" elapsed="${SECONDS}" projected
  [[ -n "${BUDGET_SECONDS}" ]] || return 0
  ((round < SAMPLE_COUNT)) || return 0
  projected=$((elapsed + (elapsed - sampling_started) * (SAMPLE_COUNT - round) / round))
  ((projected >= BUDGET_SECONDS)) || return 0
  {
    echo "Benchmark budget exceeded: ${round} of ${SAMPLE_COUNT} paired rounds took ${elapsed}s" \
      "including warm-up, projecting ${projected}s against a ${BUDGET_SECONDS}s budget."
    echo "Mean seconds per sample by package:"
    awk '$1 == "ok" { total[$2] += $3; count[$2]++ }
      END { for (p in total) printf "%8.1f  %s\n", total[p] / count[p], p }' \
      "${baseline_output}" "${candidate_output}" | sort -rn
  } >&2
  exit 1
}

# Populate build and filesystem caches without adding a timed sample. This
# avoids consistently charging the first measured tree for cold setup.
for tree in "${BASELINE_DIR}" "${CURRENT_DIR}"; do
  (
    cd "${tree}"
    PULSE_DATA_DIR="${work_dir}/warmup" go test \
      -bench=. -count=1 -run='^$' -benchtime=1x -timeout=5m \
      "${SELECTED[@]}" >/dev/null
  )
done

sampling_started="${SECONDS}"
for ((round = 1; round <= SAMPLE_COUNT; round++)); do
  if ((round % 2 == 1)); then
    run_sample "${BASELINE_DIR}" "${baseline_output}" baseline "${round}"
    run_sample "${CURRENT_DIR}" "${candidate_output}" candidate "${round}"
  else
    run_sample "${CURRENT_DIR}" "${candidate_output}" candidate "${round}"
    run_sample "${BASELINE_DIR}" "${baseline_output}" baseline "${round}"
  fi
  check_budget "${round}"
done
