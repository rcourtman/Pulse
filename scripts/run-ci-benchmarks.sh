#!/usr/bin/env bash
# Run the CI benchmark sample. When a baseline tree is supplied, execute the
# base and candidate on the same host in alternating order so host-to-host
# variance and run order are not mistaken for code regressions.

set -euo pipefail

CURRENT_DIR="${PULSE_BENCH_CURRENT_DIR:-$(pwd)}"
BASELINE_DIR="${PULSE_BENCH_BASELINE_DIR:-}"
SAMPLE_COUNT="${PULSE_BENCH_SAMPLE_COUNT:-10}"
BENCHTIME="${PULSE_BENCH_BENCHTIME:-100ms}"
# CI partitions packages, never a package's paired samples, across three
# isolated runners. Ordinary local collection still covers the full inventory.
SHARD="${PULSE_BENCH_SHARD:-all}"
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
if [[ ! "${SHARD}" =~ ^(all|0|1|2)$ ]]; then
  echo "PULSE_BENCH_SHARD must be all, 0, 1 or 2." >&2
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
  local package="$1" changed="$2" root="$3" closure="${work_dir}/closure"
  # Do not ask Go to continue on errors here. A deleted dependency has no
  # directory in the partial graph, so its changed files can otherwise
  # disappear from the intersection and be reported as no work.
  (cd "${CURRENT_DIR}" && go list -deps -test \
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
  local changed="${work_dir}/changed-paths" package root status picked=()
  if ! changed_paths > "${changed}"; then
    selection="all (trees are not both clean Git checkouts)"
    return
  fi
  if grep -qxE 'go\.(mod|sum|work|work\.sum)' "${changed}"; then
    selection="all (module files changed)"
    return
  fi
  # The root is the same for every dependency lookup in this checkout.
  if ! root="$(cd "${CURRENT_DIR}" && go list -m -f '{{.Dir}}')" ||
      [[ "${root}" != /* || "${root}" == *$'\n'* ]]; then
    selection="all (package graph unreadable)"
    return
  fi
  for package in "${PACKAGES[@]}"; do
    status=0
    package_affected "${package}" "${changed}" "${root}" || status=$?
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
fi

# Balance benchmark work across the three isolated runners while keeping every
# benchmark function's baseline, candidate, warm-up and all ten paired rounds on
# the same host. The two heaviest packages (internal/monitoring and
# internal/unifiedresources) split their discovered top-level Benchmark* union
# across two shards using measured per-sample weights from main run 45933545439
# (unknown/new benchmarks default to 1.00s and still land in exactly one shard).
# Fail closed if the package inventory grows without a shard assignment.
benchmark_weight_table() {
  cat <<'WEIGHTS'
./internal/monitoring/ BenchmarkMetadataMonitorProjection 5.87
./internal/monitoring/ BenchmarkBroadcastMetricsSnapshot 4.87
./internal/monitoring/ BenchmarkMockMetricsTargetResolutionAfterTick 3.77
./internal/monitoring/ BenchmarkGetGuestMetricsForChartBatch 1.63
./internal/monitoring/ BenchmarkPMGBackupReadPoll 1.48
./internal/monitoring/ BenchmarkHistoryReadFilter 1.22
./internal/monitoring/ BenchmarkHostSensorTemperature 1.18
./internal/monitoring/ BenchmarkBackupPollingReadCost 1.09
./internal/monitoring/ BenchmarkLivePointReads 0.99
./internal/monitoring/ BenchmarkAgentFleetDiagnosticRead 0.93
./internal/monitoring/ BenchmarkGetNodeMetricsForChartBatch 0.80
./internal/monitoring/ BenchmarkPVEBackupCarryForwardCost 0.74
./internal/monitoring/ BenchmarkLiveRegistryMetricsTargetResolution 0.73
./internal/monitoring/ BenchmarkAgentIdentityLookup 0.55
./internal/monitoring/ BenchmarkBroadcastInventoryCounts 0.55
./internal/monitoring/ BenchmarkPreviousGuestContextHostProjection 0.46
./internal/monitoring/ BenchmarkMetricWindowNodeAlertPass 0.44
./internal/monitoring/ BenchmarkBroadcastProxmoxPlatformJSON 0.37
./internal/monitoring/ BenchmarkSnapshotGuestReadCost 0.29
./internal/monitoring/ BenchmarkAgentLivenessSweep 0.23
./internal/unifiedresources/ BenchmarkRegistryProjectionSnapshot 4.11
./internal/unifiedresources/ BenchmarkHistoryIdentityQuery 2.90
./internal/unifiedresources/ BenchmarkTopLevelOwnerIndexPass 2.73
./internal/unifiedresources/ BenchmarkTopLevelOwnerIndexResolver 2.65
./internal/unifiedresources/ BenchmarkRegistryMetadataDirtyReads 1.62
./internal/unifiedresources/ BenchmarkIngestRecords_MergeDedup 0.80
./internal/unifiedresources/ BenchmarkFindCandidates 0.75
./internal/unifiedresources/ BenchmarkCanonicalAliasesVocabulary 0.59
./internal/unifiedresources/ BenchmarkRegistryMetadataBulkReads 0.57
./internal/unifiedresources/ BenchmarkTopLevelOwnerIndexConnectedDemo 0.54
./internal/unifiedresources/ BenchmarkIngestRecords_NewResources 0.53
./internal/unifiedresources/ BenchmarkResourceRegistry_GetByCanonicalAlias 0.29
./internal/unifiedresources/ BenchmarkRegistryChangeEmission1000VMs 0.29
./internal/unifiedresources/ BenchmarkSuccessionGroupSettlement 0.26
./internal/unifiedresources/ BenchmarkCanonicalResourceSort 0.24
./internal/unifiedresources/ BenchmarkMonitorAdapterReplaceRegistry1000VMs 0.23
./internal/unifiedresources/ BenchmarkRegistry_CachedVMReads 0.20
./internal/unifiedresources/ BenchmarkClassifyResourceSensitivity 0.17
./internal/unifiedresources/ BenchmarkRegistry_DirtyVMCacheRead 0.15
./internal/unifiedresources/ BenchmarkRegistryList1000VMs 0.15
./internal/unifiedresources/ BenchmarkIngestMixed 0.14
./internal/unifiedresources/ BenchmarkIdentityMatcher_Add 0.14
./internal/unifiedresources/ BenchmarkRefreshPlatformScopes 0.12
./internal/unifiedresources/ BenchmarkRegistryIngestSnapshot1000VMs 0.12
WEIGHTS
}

list_package_benchmarks() {
  local package="$1"
  local combined="${work_dir}/bench-list.raw"
  local list_dir="${work_dir}/list-data"
  local label tree out
  mkdir -p "${list_dir}"
  : > "${combined}"
  for label in baseline candidate; do
    tree="${CURRENT_DIR}"
    [[ "${label}" != baseline ]] || tree="${BASELINE_DIR}"
    [[ -n "${tree}" ]] || continue
    out="${work_dir}/bench-list-${label}.out"
    if ! (cd "${tree}" && PULSE_DATA_DIR="${list_dir}" go test -list '^Benchmark' "${package}") > "${out}"; then
      echo "Failed to list benchmarks for ${package} (${label})." >&2
      return 1
    fi
    grep -E '^Benchmark[A-Za-z0-9_]+$' "${out}" >> "${combined}" || true
  done
  if [[ ! -s "${combined}" ]]; then
    echo "No benchmarks discovered for split package: ${package}" >&2
    return 1
  fi
  LC_ALL=C sort -u "${combined}"
}

select_split_filter() {
  local package="$1" target_slot="$2"
  local names_file="${work_dir}/bench-names"
  local weights_file="${work_dir}/bench-weights"
  local picked_file="${work_dir}/bench-picked"
  list_package_benchmarks "${package}" > "${names_file}" || return 1
  benchmark_weight_table > "${weights_file}"
  awk -v pkg="${package}" '
    FILENAME == ARGV[1] {
      if ($1 == pkg) w[$2] = $3 + 0
      next
    }
    {
      weight = ($1 in w) ? w[$1] : 1.0
      printf "%.6f\t%s\n", weight, $1
    }
  ' "${weights_file}" "${names_file}" |
    LC_ALL=C sort -k1,1nr -k2,2 |
    awk -F '\t' -v target="${target_slot}" '
      BEGIN { load[0] = 0; load[1] = 0 }
      {
        slot = (load[0] <= load[1]) ? 0 : 1
        load[slot] += $1 + 0
        if (slot == target) print $2
      }
    ' | LC_ALL=C sort > "${picked_file}"
  if [[ ! -s "${picked_file}" ]]; then
    printf ''
    return 0
  fi
  local joined
  joined="$(paste -sd'|' "${picked_file}")"
  printf '^(%s)$' "${joined}"
}

UNSPLIT_PACKAGES=()
SPLIT_PACKAGES=()
SPLIT_FILTERS=()

shard_packages() {
  local package owner slot filter picked=()
  for package in "${SELECTED[@]}"; do
    slot=""
    case "${package}" in
      ./internal/monitoring/)
        if [[ "${SHARD}" == 0 ]]; then owner=0; slot=0
        elif [[ "${SHARD}" == 2 ]]; then owner=2; slot=1
        else owner=split
        fi
        ;;
      ./internal/unifiedresources/)
        if [[ "${SHARD}" == 0 ]]; then owner=0; slot=0
        elif [[ "${SHARD}" == 1 ]]; then owner=1; slot=1
        else owner=split
        fi
        ;;
      ./internal/dockeragent/) owner=0 ;;
      ./internal/api/|./pkg/auth/|./internal/hostagent/) owner=1 ;;
      ./pkg/metrics/|./cmd/pulse-agent/|./internal/hostmetrics/) owner=2 ;;
      *) echo "Benchmark package has no shard: ${package}" >&2; exit 2 ;;
    esac
    if [[ "${SHARD}" == all ]]; then
      picked+=("${package}")
      UNSPLIT_PACKAGES+=("${package}")
    elif [[ "${SHARD}" == "${owner}" ]]; then
      if [[ -n "${slot}" ]]; then
        filter="$(select_split_filter "${package}" "${slot}")" || exit 1
        if [[ -n "${filter}" ]]; then
          picked+=("${package}")
          SPLIT_PACKAGES+=("${package}")
          SPLIT_FILTERS+=("${filter}")
        fi
      else
        picked+=("${package}")
        UNSPLIT_PACKAGES+=("${package}")
      fi
    fi
  done
  SELECTED=(${picked[@]+"${picked[@]}"})
}
shard_packages
echo "Benchmark packages, ${selection}, shard ${SHARD}: ${SELECTED[*]:-none}"

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
  printf 'shard=%s\nshard_count=3\n' "${SHARD}"
  printf 'packages=%s\n' "${SELECTED[*]:-}"
  for ((i = 0; i < ${#SPLIT_PACKAGES[@]}; i++)); do
    printf 'bench_filter=%s,%s\n' "${SPLIT_PACKAGES[i]}" "${SPLIT_FILTERS[i]}"
  done
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

run_tree_benchmarks() {
  local tree="$1"
  local label="$2"
  local round="$3"
  local count="$4"
  local benchtime="$5"
  local use_exec="$6"
  local data_dir="$7"
  local i exec_args=()
  if [[ "${use_exec}" == yes ]]; then
    exec_args=(-exec "${binary_wrapper}")
  fi
  (
    cd "${tree}"
    if ((${#UNSPLIT_PACKAGES[@]} > 0)); then
      PULSE_BENCH_LABEL="${label}" PULSE_BENCH_ROUND="${round}" \
      PULSE_DATA_DIR="${data_dir}" go test "${exec_args[@]}" \
        -bench=. -benchmem -count="${count}" -run='^$' \
        -benchtime="${benchtime}" -timeout=5m \
        "${UNSPLIT_PACKAGES[@]}"
    fi
    for ((i = 0; i < ${#SPLIT_PACKAGES[@]}; i++)); do
      PULSE_BENCH_LABEL="${label}" PULSE_BENCH_ROUND="${round}" \
      PULSE_DATA_DIR="${data_dir}" go test "${exec_args[@]}" \
        -bench="${SPLIT_FILTERS[i]}" -benchmem -count="${count}" -run='^$' \
        -benchtime="${benchtime}" -timeout=5m \
        "${SPLIT_PACKAGES[i]}"
    done
  )
}

run_sample() {
  local tree="$1"
  local output="$2"
  local label="$3"
  local round="$4"
  local data_dir="${work_dir}/${label}-${round}"

  mkdir -p "${data_dir}"
  printf 'sample=%s,%s,%s\n' "${label}" "${round}" "$(date -u +%FT%TZ)" >> "${metadata}"
  echo "=== ${label} benchmark sample ${round}/${SAMPLE_COUNT} ==="
  run_tree_benchmarks "${tree}" "${label}" "${round}" 1 "${BENCHTIME}" yes "${data_dir}" | tee -a "${output}"
}

run_unpaired() {
  local output="${CURRENT_DIR}/bench-results.txt"
  local data_dir="${work_dir}/candidate"

  mkdir -p "${data_dir}"
  run_tree_benchmarks "${CURRENT_DIR}" candidate unpaired "${SAMPLE_COUNT}" "${BENCHTIME}" yes "${data_dir}" | tee "${output}"
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
mkdir -p "${work_dir}/warmup"
for tree in "${BASELINE_DIR}" "${CURRENT_DIR}"; do
  run_tree_benchmarks "${tree}" warmup warmup 1 1x no "${work_dir}/warmup" >/dev/null
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
