#!/usr/bin/env bash
# Contract tests for paired CI benchmark collection and verdict quality.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${WORK_DIR}/bin" "${WORK_DIR}/candidate" "${WORK_DIR}/baseline"
cat > "${WORK_DIR}/bin/fixture.test" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == -test.bench=* ]] || exit 91
[[ "${FAKE_BINARY_FAIL:-0}" == 0 ]] || exit 23
filter="${1#-test.bench=}"
if [[ "${filter}" == '.' ]]; then
  echo 'BenchmarkExample-4  1  100 ns/op  0 B/op  0 allocs/op'
else
  inner="${filter#^(}"
  inner="${inner%)\$}"
  IFS='|' read -r -a names <<< "${inner}"
  for name in "${names[@]}"; do
    echo "${name}-4  1  100 ns/op  0 B/op  0 allocs/op"
  done
fi
EOF
chmod +x "${WORK_DIR}/bin/fixture.test"
export FAKE_BINARY="${WORK_DIR}/bin/fixture.test"
cat > "${WORK_DIR}/bin/go" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == version ]]; then
  echo 'go version go1.26.7 linux/amd64'
  exit 0
fi
# Every package's closure is itself plus internal/shared, except pkg/auth.
if [[ "$1" == list ]]; then
  if [[ -n "${FAKE_GO_LIST_LOG:-}" ]]; then
    printf '%s\n' "$*" >> "${FAKE_GO_LIST_LOG}"
  fi
  [[ "${FAKE_GO_LIST_FAIL:-0}" == 0 ]] || exit 1
  if [[ "$2" == -m ]]; then
    if [[ "${FAKE_GO_MULTI_ROOT:-0}" == 1 ]]; then
      printf '%s/second-module\n' "$PWD"
    fi
    echo "$PWD"
    exit 0
  fi
  package="${!#}"
  package="${package#./}"
  package="${package%/}"
  printf '%s/%s\n\n' "$PWD" "${package}"
  if [[ "${FAKE_GO_PARTIAL_GRAPH:-0}" == 1 && "${package}" == internal/hostmetrics ]]; then
    # go list -e can exit zero with only the surviving directories. The
    # strict call can also have written stdout, but signals the failure.
    if [[ " $* " == *' -e '* ]]; then
      exit 0
    fi
    exit 1
  fi
  [[ "${package}" == pkg/auth ]] || echo "$PWD/internal/shared"
  exit 0
fi
if [[ "$1" == test && "$2" == -list ]]; then
  [[ "${FAKE_GO_BENCH_LIST_FAIL:-0}" == 0 ]] || exit 1
  package="${!#}"
  if [[ "${FAKE_GO_BENCH_LIST_EMPTY:-0}" == 1 ]]; then
    printf 'ok  \t%s\t0.001s\n' "${package}"
    exit 0
  fi
  if [[ "${FAKE_GO_BENCH_LIST_SINGLE:-0}" == 1 ]]; then
    printf 'BenchmarkSolo\nok  \t%s\t0.001s\n' "${package}"
    exit 0
  fi
  case "${package}" in
    ./internal/monitoring/)
      printf 'BenchmarkMetadataMonitorProjection\nBenchmarkBroadcastMetricsSnapshot\nBenchmarkPMGBackupReadPoll\n'
      if [[ "${FAKE_GO_BENCH_DIFF:-0}" == 1 ]]; then
        if [[ "$PWD" == */candidate ]]; then
          printf 'BenchmarkCandidateOnly\n'
        else
          printf 'BenchmarkBaselineOnly\n'
        fi
      fi
      ;;
    ./internal/unifiedresources/)
      printf 'BenchmarkRegistryProjectionSnapshot\nBenchmarkHistoryIdentityQuery\nBenchmarkFindCandidates\n'
      ;;
    *)
      printf 'BenchmarkExample\n'
      ;;
  esac
  printf 'ok  \t%s\t0.001s\n' "${package}"
  exit 0
fi
printf '%s\t%s\n' "$PWD" "$*" >> "${FAKE_GO_LOG}"
args=("$@")
bench_filter='.'
for ((i=0; i<${#args[@]}; i++)); do
  if [[ "${args[i]}" == -bench=* ]]; then
    bench_filter="${args[i]#-bench=}"
  fi
done
for ((i=0; i<${#args[@]}; i++)); do
  if [[ "${args[i]}" == -exec ]]; then
    "${args[i+1]}" "${FAKE_BINARY}" "-test.bench=${bench_filter}" || exit
    printf 'ok  \texample.test/bench\t1.500s\n'
    exit 0
  fi
done
cat <<'RESULT'
goos: linux
goarch: amd64
pkg: example.test/bench
cpu: test
BenchmarkExample-4  1  100 ns/op  0 B/op  0 allocs/op
PASS
ok  example.test/bench  0.001s
RESULT
EOF
chmod +x "${WORK_DIR}/bin/go"

(
  cd "${WORK_DIR}/candidate"
  PATH="${WORK_DIR}/bin:${PATH}" \
    FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
    PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" \
    PULSE_BENCH_SAMPLE_COUNT=2 \
    bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
)

[[ "$(grep -c '^BenchmarkExample' "${WORK_DIR}/candidate/bench-baseline.txt")" == 2 ]]
[[ "$(grep -c '^BenchmarkExample' "${WORK_DIR}/candidate/bench-results.txt")" == 2 ]]

# Warm-up is baseline then candidate. Measured rounds must alternate which tree
# goes first, avoiding a permanent order advantage.
mapfile -t calls < "${WORK_DIR}/go.log"
[[ "${#calls[@]}" == 6 ]]
[[ "${calls[2]}" == "${WORK_DIR}/baseline"$'\t'* ]]
[[ "${calls[3]}" == "${WORK_DIR}/candidate"$'\t'* ]]
[[ "${calls[4]}" == "${WORK_DIR}/candidate"$'\t'* ]]
[[ "${calls[5]}" == "${WORK_DIR}/baseline"$'\t'* ]]

metadata="${WORK_DIR}/candidate/bench-metadata.txt"
grep -qFx 'samples=2' "${metadata}"
grep -qFx 'shard=all' "${metadata}"
grep -qFx 'shard_count=3' "${metadata}"
grep -qFx 'selection=all (trees are not both clean Git checkouts)' "${metadata}"
grep -qFx 'packages=./pkg/metrics/ ./pkg/auth/ ./internal/api/ ./internal/monitoring/ ./internal/unifiedresources/ ./internal/dockeragent/ ./cmd/pulse-agent/ ./internal/hostagent/ ./internal/hostmetrics/' "${metadata}"
grep -qFx 'benchtime=100ms' "${metadata}"
grep -qFx 'candidate.commit=unavailable' "${metadata}"
grep -qFx 'baseline.go=go version go1.26.7 linux/amd64' "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}")" == 4 ]]
! grep -qF "${WORK_DIR}" "${metadata}"
digest="$(sha256sum "${FAKE_BINARY}")"
digest="${digest%% *}"
[[ "$(grep -c '^binary=' "${metadata}")" == 4 ]]
for tuple in baseline,1 candidate,1 candidate,2 baseline,2; do
  grep -qFx "binary=${tuple},fixture.test,${digest}" "${metadata}"
done

# Real Git roots retain exact identities and measure only the packages whose
# dependency closure the candidate changes; a repeated run replaces metadata.
for tree in candidate baseline; do
  git -C "${WORK_DIR}/${tree}" init -q
  mkdir -p "${WORK_DIR}/${tree}/internal/shared" "${WORK_DIR}/${tree}/internal/api"
  echo 'module example.test' > "${WORK_DIR}/${tree}/go.mod"
  echo 'package shared' > "${WORK_DIR}/${tree}/internal/shared/shared.go"
  echo 'package api' > "${WORK_DIR}/${tree}/internal/api/handler.go"
done
change() {
  mkdir -p "$(dirname "${WORK_DIR}/candidate/$1")"
  echo "// $2" >> "${WORK_DIR}/candidate/$1"
}
commit() {
  git -C "${WORK_DIR}/$1" add go.mod internal frontend 2>/dev/null || git -C "${WORK_DIR}/$1" add go.mod internal
  git -C "${WORK_DIR}/$1" -c user.name=Test -c user.email=test@example.invalid commit -qm fixture
}
run_paired() {
  : > "${WORK_DIR}/go.log"
  PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
    PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" PULSE_BENCH_SAMPLE_COUNT=1 \
    bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" "$@" >/dev/null
}
commit baseline

# Frontend files and another package's tests reach no benchmark binary.
change frontend/app.ts frontend
change internal/shared/shared_test.go shared-test
commit candidate
list_log="${WORK_DIR}/go-list.log"
FAKE_GO_LIST_LOG="${list_log}" run_paired
grep -qFx 'selection=affected' "${metadata}"
grep -qFx 'packages=' "${metadata}"
grep -qFx "candidate.commit=$(git -C "${WORK_DIR}/candidate" rev-parse HEAD)" "${metadata}"
grep -qFx "baseline.tree=$(git -C "${WORK_DIR}/baseline" rev-parse 'HEAD^{tree}')" "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}" || true)" == 0 ]]
[[ ! -s "${WORK_DIR}/candidate/bench-results.txt" ]]
[[ ! -s "${WORK_DIR}/candidate/bench-baseline.txt" ]]
[[ ! -s "${WORK_DIR}/go.log" ]]
# A complete but unaffected graph still checks all nine closures. Do not
# rediscover the same candidate module root for each lookup.
[[ "$(grep -c '^list -m ' "${list_log}")" == 1 ]]
[[ "$(grep -cv '^list -m ' "${list_log}")" == 9 ]]

# A benchmark package's own test file selects that package alone.
change internal/api/handler_test.go api-test
commit candidate
run_paired
grep -qFx 'packages=./internal/api/' "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}")" == 2 ]]
[[ "$(grep -c '^BenchmarkExample' "${WORK_DIR}/candidate/bench-results.txt")" == 1 ]]
mapfile -t calls < "${WORK_DIR}/go.log"
[[ "${#calls[@]}" == 4 ]]
for call in "${calls[@]}"; do
  [[ "${call}" == *' ./internal/api/' && "${call}" != *pkg/metrics* ]]
done

# Package selection is intersected with the fixed shard, not expanded. A shard
# with no affected package must not invoke go test with an empty argument list
# (which would unexpectedly run the repository root).
PULSE_BENCH_SHARD=0 run_paired
grep -qFx 'selection=affected' "${metadata}"
grep -qFx 'shard=0' "${metadata}"
grep -qFx 'packages=' "${metadata}"
[[ ! -s "${WORK_DIR}/go.log" ]]
[[ ! -s "${WORK_DIR}/candidate/bench-baseline.txt" ]]
[[ ! -s "${WORK_DIR}/candidate/bench-results.txt" ]]
PULSE_BENCH_SHARD=1 run_paired
grep -qFx 'packages=./internal/api/' "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}")" == 2 ]]

# A shared non-test file selects every package that imports its directory.
change internal/shared/shared.go shared
commit candidate
run_paired
grep -qFx 'selection=affected' "${metadata}"
grep -qFx 'packages=./pkg/metrics/ ./internal/api/ ./internal/monitoring/ ./internal/unifiedresources/ ./internal/dockeragent/ ./cmd/pulse-agent/ ./internal/hostagent/ ./internal/hostmetrics/' "${metadata}"

# An unreadable package graph measures everything rather than guessing.
FAKE_GO_LIST_FAIL=1 run_paired
grep -qFx 'selection=all (package graph unreadable)' "${metadata}"
grep -qF 'packages=./pkg/metrics/ ./pkg/auth/' "${metadata}"

# A late partial graph must discard any earlier narrowed selection, even if
# stdout holds a directory and none of the visible dirs matches the diff.
FAKE_GO_PARTIAL_GRAPH=1 PULSE_BENCH_SHARD=2 run_paired
grep -qFx 'selection=all (package graph unreadable)' "${metadata}"
grep -qFx 'packages=./pkg/metrics/ ./internal/monitoring/ ./cmd/pulse-agent/ ./internal/hostmetrics/' "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}")" == 2 ]]

# Several main modules cannot safely be compared against one directory root.
FAKE_GO_MULTI_ROOT=1 PULSE_BENCH_SHARD=0 run_paired
grep -qFx 'selection=all (package graph unreadable)' "${metadata}"
grep -qFx 'packages=./internal/monitoring/ ./internal/unifiedresources/ ./internal/dockeragent/' "${metadata}"

# Uncommitted edits are measured but invisible to the committed trees.
echo '// local edit' >> "${WORK_DIR}/baseline/internal/api/handler.go"
run_paired
grep -qFx 'selection=all (trees are not both clean Git checkouts)' "${metadata}"
git -C "${WORK_DIR}/baseline" checkout -q -- internal/api/handler.go

# Module changes can move any package.
change go.mod module
commit candidate
run_paired
grep -qFx 'selection=all (module files changed)' "${metadata}"
grep -qF 'packages=./pkg/metrics/ ./pkg/auth/' "${metadata}"

# Every unsplit package belongs to exactly one shard, and split packages
# (internal/monitoring on 0+2; internal/unifiedresources on 0+1) partition
# their discovered top-level Benchmark* union disjointly across their two
# shards. Each shard retains both warmups, all ten base/candidate samples,
# alternating order, filter provenance and executable hashes.
: > "${WORK_DIR}/unsplit-partition"
: > "${WORK_DIR}/mon-partition"
: > "${WORK_DIR}/ur-partition"
for shard in 0 1 2; do
  case "${shard}" in
    0)
      expected='./internal/monitoring/ ./internal/unifiedresources/ ./internal/dockeragent/'
      expected_unsplit='./internal/dockeragent/'
      calls_per_tree=3
      ;;
    1)
      expected='./pkg/auth/ ./internal/api/ ./internal/unifiedresources/ ./internal/hostagent/'
      expected_unsplit='./pkg/auth/ ./internal/api/ ./internal/hostagent/'
      calls_per_tree=2
      ;;
    2)
      expected='./pkg/metrics/ ./internal/monitoring/ ./cmd/pulse-agent/ ./internal/hostmetrics/'
      expected_unsplit='./pkg/metrics/ ./cmd/pulse-agent/ ./internal/hostmetrics/'
      calls_per_tree=2
      ;;
  esac
  : > "${WORK_DIR}/go.log"
  PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
    PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" PULSE_BENCH_SAMPLE_COUNT=10 \
    PULSE_BENCH_SHARD="${shard}" \
    bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
  grep -qFx "shard=${shard}" "${metadata}"
  grep -qFx "packages=${expected}" "${metadata}"
  printf '%s\n' "${expected_unsplit}" | tr ' ' '\n' >> "${WORK_DIR}/unsplit-partition"
  [[ "$(grep -c '^sample=' "${metadata}")" == 20 ]]
  [[ "$(grep -c '^binary=' "${metadata}")" == $((20 * calls_per_tree)) ]]
  [[ "$(grep -c '^BenchmarkExample' "${WORK_DIR}/candidate/bench-baseline.txt")" == 10 ]]
  [[ "$(grep -c '^BenchmarkExample' "${WORK_DIR}/candidate/bench-results.txt")" == 10 ]]
  if [[ "${shard}" == 0 || "${shard}" == 2 ]]; then
    mon_filter="$(awk -F, '$1 == "bench_filter=./internal/monitoring/" { print $2 }' "${metadata}")"
    [[ "${mon_filter}" == ^\(*\)\$ ]]
    inner="${mon_filter#^(}"; inner="${inner%)\$}"
    printf '%s\n' "${inner}" | tr '|' '\n' >> "${WORK_DIR}/mon-partition"
  fi
  if [[ "${shard}" == 0 || "${shard}" == 1 ]]; then
    ur_filter="$(awk -F, '$1 == "bench_filter=./internal/unifiedresources/" { print $2 }' "${metadata}")"
    [[ "${ur_filter}" == ^\(*\)\$ ]]
    inner="${ur_filter#^(}"; inner="${inner%)\$}"
    printf '%s\n' "${inner}" | tr '|' '\n' >> "${WORK_DIR}/ur-partition"
  fi
  mapfile -t calls < "${WORK_DIR}/go.log"
  [[ "${#calls[@]}" == $((22 * calls_per_tree)) ]]
  for ((round=1; round<=10; round++)); do
    first=baseline; second=candidate
    if ((round % 2 == 0)); then first=candidate; second=baseline; fi
    base_idx=$((2 * round * calls_per_tree))
    for ((offset=0; offset<calls_per_tree; offset++)); do
      [[ "${calls[base_idx + offset]}" == "${WORK_DIR}/${first}"$'\t'* ]]
      [[ "${calls[base_idx + calls_per_tree + offset]}" == "${WORK_DIR}/${second}"$'\t'* ]]
    done
    for label in baseline candidate; do
      grep -qFx "binary=${label},${round},fixture.test,${digest}" "${metadata}"
    done
  done
  # Branch collection partitions the same inventory using Go's ten counts per
  # invocation; it must not fall back to the full package set.
  : > "${WORK_DIR}/go.log"
  PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" PULSE_BENCH_BASELINE_DIR='' \
    PULSE_BENCH_SAMPLE_COUNT=10 PULSE_BENCH_SHARD="${shard}" \
    bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
  grep -qFx 'selection=all (no baseline)' "${metadata}"
  grep -qFx "packages=${expected}" "${metadata}"
  [[ "$(wc -l < "${WORK_DIR}/go.log")" == "${calls_per_tree}" ]]
  grep -qF -- "-bench=. -benchmem -count=10 -run=^$ -benchtime=100ms -timeout=5m ${expected_unsplit}" "${WORK_DIR}/go.log"
done
[[ "$(wc -l < "${WORK_DIR}/unsplit-partition")" == 7 ]]
[[ -z "$(sort "${WORK_DIR}/unsplit-partition" | uniq -d)" ]]
[[ "$(sort "${WORK_DIR}/mon-partition" | paste -sd',')" == 'BenchmarkBroadcastMetricsSnapshot,BenchmarkMetadataMonitorProjection,BenchmarkPMGBackupReadPoll' ]]
[[ -z "$(sort "${WORK_DIR}/mon-partition" | uniq -d)" ]]
[[ "$(sort "${WORK_DIR}/ur-partition" | paste -sd',')" == 'BenchmarkFindCandidates,BenchmarkHistoryIdentityQuery,BenchmarkRegistryProjectionSnapshot' ]]
[[ -z "$(sort "${WORK_DIR}/ur-partition" | uniq -d)" ]]

# Newly added (candidate-only) and removed (baseline-only) benchmarks in a split
# package are both discovered from the two-tree union and assigned to exactly
# one shard.
: > "${WORK_DIR}/mon-diff-partition"
for shard in 0 2; do
  FAKE_GO_BENCH_DIFF=1 PULSE_BENCH_SHARD="${shard}" run_paired
  mon_filter="$(awk -F, '$1 == "bench_filter=./internal/monitoring/" { print $2 }' "${metadata}")"
  inner="${mon_filter#^(}"; inner="${inner%)\$}"
  printf '%s\n' "${inner}" | tr '|' '\n' >> "${WORK_DIR}/mon-diff-partition"
done
[[ "$(sort "${WORK_DIR}/mon-diff-partition" | paste -sd',')" == 'BenchmarkBaselineOnly,BenchmarkBroadcastMetricsSnapshot,BenchmarkCandidateOnly,BenchmarkMetadataMonitorProjection,BenchmarkPMGBackupReadPoll' ]]
[[ -z "$(sort "${WORK_DIR}/mon-diff-partition" | uniq -d)" ]]

# A single-benchmark split package runs on its primary shard and drops cleanly
# from the secondary shard rather than invoking go test with an empty regex.
FAKE_GO_BENCH_LIST_SINGLE=1 PULSE_BENCH_SHARD=0 run_paired
grep -qFx 'bench_filter=./internal/monitoring/,^(BenchmarkSolo)$' "${metadata}"
FAKE_GO_BENCH_LIST_SINGLE=1 PULSE_BENCH_SHARD=2 run_paired
! grep -qF './internal/monitoring/' "${metadata}"
! grep -qF 'bench_filter=./internal/monitoring/' "${metadata}"

# Failed or empty benchmark discovery for a split package fails closed before
# running any timed benchmark.
for bad_env in FAKE_GO_BENCH_LIST_FAIL=1 FAKE_GO_BENCH_LIST_EMPTY=1; do
  : > "${WORK_DIR}/go.log"
  set +e
  output="$(env "${bad_env}" PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
    PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" PULSE_BENCH_SAMPLE_COUNT=1 \
    PULSE_BENCH_SHARD=0 bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" 2>&1)"
  status=$?
  set -e
  [[ "${status}" != 0 ]]
  [[ ! -s "${WORK_DIR}/go.log" ]]
done

# A malformed shard is a collection error, never a silent no-op or an all-tree
# fallback. This must stop before any Go invocation.
for invalid in 3 -1 01 x all,0; do
  : > "${WORK_DIR}/go.log"
  set +e
  output="$(PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
    PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" PULSE_BENCH_SHARD="${invalid}" \
    bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" 2>&1)"
  status=$?
  set -e
  [[ "${status}" == 2 ]]
  grep -qF 'PULSE_BENCH_SHARD must be all, 0, 1 or 2.' <<<"${output}"
  [[ ! -s "${WORK_DIR}/go.log" ]]
done

# A newly added inventory member without a fixed assignment fails rather than
# disappearing from all three jobs. Exercise the actual collector, not a model
# of the partition function.
sed '/^PACKAGES=(/a\  ./internal/new-benchmark/' \
  "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" > "${WORK_DIR}/unassigned.sh"
: > "${WORK_DIR}/go.log"
set +e
output="$(PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
  PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" PULSE_BENCH_BASELINE_DIR='' \
  PULSE_BENCH_SHARD=0 bash "${WORK_DIR}/unassigned.sh" 2>&1)"
status=$?
set -e
[[ "${status}" == 2 ]]
grep -qFx 'Benchmark package has no shard: ./internal/new-benchmark/' <<<"${output}"
[[ ! -s "${WORK_DIR}/go.log" ]]

# A nested archive has no identity of its own; never attribute its parent's SHA.
mkdir -p "${WORK_DIR}/candidate/archive"
PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
  PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate/archive" \
  PULSE_BENCH_BASELINE_DIR='' PULSE_BENCH_SAMPLE_COUNT=1 \
  bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
grep -qFx 'candidate.commit=unavailable' "${WORK_DIR}/candidate/archive/bench-metadata.txt"
! grep -q '^baseline\.' "${WORK_DIR}/candidate/archive/bench-metadata.txt"
grep -qFx 'selection=all (no baseline)' "${WORK_DIR}/candidate/archive/bench-metadata.txt"

grep -qFx "binary=candidate,unpaired,fixture.test,${digest}" \
  "${WORK_DIR}/candidate/archive/bench-metadata.txt"

# The wrapper must not turn a failed executable into successful evidence.
set +e
PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
  FAKE_BINARY_FAIL=1 PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
  PULSE_BENCH_BASELINE_DIR='' PULSE_BENCH_SAMPLE_COUNT=1 \
  bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
status=$?
set -e
[[ "${status}" == 23 ]]
grep -qFx "binary=candidate,unpaired,fixture.test,${digest}" "${metadata}"

# A run projected to reach its budget stops after the round that shows it,
# naming each package's mean cost; one inside its budget completes.
set +e
output="$(PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
  PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
  PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" PULSE_BENCH_SAMPLE_COUNT=3 \
  PULSE_BENCH_BUDGET_SECONDS=0 \
  bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" 2>&1 >/dev/null)"
status=$?
set -e
[[ "${status}" == 1 ]]
grep -qF 'Benchmark budget exceeded: 1 of 3 paired rounds' <<<"${output}"
grep -qE '^ +1\.5  example\.test/bench$' <<<"${output}"
[[ "$(grep -c '^sample=' "${metadata}")" == 2 ]]
PATH="${WORK_DIR}/bin:${PATH}" FAKE_GO_LOG="${WORK_DIR}/go.log" \
  PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" \
  PULSE_BENCH_BASELINE_DIR="${WORK_DIR}/baseline" PULSE_BENCH_SAMPLE_COUNT=3 \
  PULSE_BENCH_BUDGET_SECONDS=3600 \
  bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null
[[ "$(grep -c '^sample=' "${metadata}")" == 6 ]]
set +e
PULSE_BENCH_CURRENT_DIR="${WORK_DIR}/candidate" PULSE_BENCH_BUDGET_SECONDS=soon \
  bash "${ROOT_DIR}/scripts/run-ci-benchmarks.sh" >/dev/null 2>&1
status=$?
set -e
[[ "${status}" == 2 ]]

cat > "${WORK_DIR}/adequate.txt" <<'EOF'
Example-4  100.0n ± 1%  111.0n ± 1%  +11.00% (p=0.001 n=10)
EOF
set +e
output="$(bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/adequate.txt" 2>&1)"
status=$?
set -e
if [[ "${status}" == 0 ]]; then
  echo "adequately sampled regression was not rejected" >&2
  exit 1
fi
grep -qF "BENCHMARK REGRESSION DETECTED" <<<"${output}"

cat > "${WORK_DIR}/undersampled.txt" <<'EOF'
Example-4  100.0n ± ∞ ¹  111.0n ± ∞ ¹  +11.00% (p=0.008 n=5)
EOF
set +e
output="$(bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/undersampled.txt" 2>&1)"
status=$?
set -e
[[ "${status}" != 0 ]]
grep -qF "BENCHMARK EVIDENCE INSUFFICIENT" <<<"${output}"

cat > "${WORK_DIR}/clean.txt" <<'EOF'
Example-4  100.0n ± 1%  105.0n ± 1%  +5.00% (p=0.001 n=10)
EOF
bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/clean.txt" >/dev/null

# Missing evidence, including a lost row beside a valid pair, cannot turn into
# a clean verdict. Both sides of unequal-sized samples need the same minimum.
assert_insufficient() {
  local output status
  status=0
  output="$(bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/incomplete.txt" 2>&1)" || status=$?
  [[ "${status}" == 1 ]]
  grep -qF 'BENCHMARK EVIDENCE INSUFFICIENT' <<<"${output}"
  ! grep -qF 'No significant benchmark regressions' <<<"${output}"
}
: > "${WORK_DIR}/incomplete.txt"
assert_insufficient
printf 'pkg: example.test/bench\n' > "${WORK_DIR}/incomplete.txt"
assert_insufficient
while IFS= read -r row; do
  printf '%s\n' "${row}" > "${WORK_DIR}/incomplete.txt"
  assert_insufficient
  # One good comparison must not hide another missing or invalid one.
  cat "${WORK_DIR}/clean.txt" >> "${WORK_DIR}/incomplete.txt"
  assert_insufficient
done <<'EOF'
MissingCandidate-4  100.0n ± 1%
MissingBaseline-4                     105.0n ± 1%
Named/with:colon-4  100.0n ± 1%
MissingStatistics-4  100.0n ± 1%  105.0n ± 1%  ~
MalformedStatistics-4  100.0n ± 1%  105.0n ± 1%  ~ (p=unknown n=10)
InvalidProbability-4  100.0n ± 1%  105.0n ± 1%  ~ (p=1.100 n=10)
MissingCount-4  100.0n ± 1%  105.0n ± 1%  ~ (p=0.100)
MissingSecondCount-4  100.0n ± 1%  105.0n ± 1%  ~ (p=0.100 n1=10)
ShortBaseline-4  100.0n ± 1%  105.0n ± 1%  ~ (p=0.100 n1=9 n2=10)
ShortCandidate-4  100.0n ± 1%  105.0n ± 1%  ~ (p=0.100 n1=10 n2=9)
UnsignedRegression-4  100.0n ± 1%  111.0n ± 1%  11.00% (p=0.001 n=10)
UnsignedRoundTen-4  100.0n ± 1%  110.0n ± 1%  10% (p=0.001 n=10)
UnsignedFraction-4  100.0n ± 1%  100.5n ± 1%  0.50% (p=0.001 n=10)
EOF
cat > "${WORK_DIR}/complete.txt" <<'EOF'
goos: linux
goarch: amd64
pkg: example.test/bench
cpu: fixture 100
          │ baseline.txt │ candidate.txt │
          │ sec/op       │ sec/op vs base │
Equal-4      100.0n ± 0%   100.0n ± 0%  ~ (p=1.000 n=10) ¹
Unequal-4    100.0n ± 1%   105.0n ± 1%  +5.00% (p=0.100 n1=10 n2=11)
Named/with:colon-4  100.0n ± 1%  105.0n ± 1%  +5.00% (p=0.100 n=10)
Unchanged/sorted-by-cpu-4  3.928k ± 0%  3.928k ± 0%  0.00% (p=0.044 n=10)
geomean      100.0n        102.5n       +2.47%
¹ all samples are equal
EOF
bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/complete.txt" >/dev/null

# Go SetBytes benchmarks add byte-throughput tables alongside time and
# allocation costs. Respect their opposite direction without relaxing either
# the significance/sample gates or the existing cost-metric verdicts.
while read -r metric delta pvalue samples expected; do
  candidate="$(awk -v delta="${delta}" 'BEGIN { printf "%.2f", 100 * (1 + delta / 100) }')"
  cat > "${WORK_DIR}/metric.txt" <<EOF
          │ baseline.txt │     candidate.txt      │
          │ ${metric}   │ ${metric} vs base      │
Example-4  100.00 ± 1%  ${candidate} ± 1%  ${delta}% (p=${pvalue} n=${samples})
EOF
  set +e
  output="$(bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/metric.txt" 2>&1)"
  status=$?
  set -e
  if [[ "${status}" != "${expected}" ]]; then
    printf 'wrong verdict for %s %s%% p=%s n=%s: %s\n%s\n' \
      "${metric}" "${delta}" "${pvalue}" "${samples}" "${status}" "${output}" >&2
    exit 1
  fi
  if [[ "${status}" == 1 && "${samples}" == 10 ]]; then
    grep -qF "[${metric}] Example-4" <<<"${output}"
  fi
done <<'EOF'
B/s +11.00 0.001 10 0
B/s -11.00 0.001 10 1
MB/s +11.00 0.001 10 0
MB/s -11.00 0.001 10 1
KiB/s -11.00 0.001 10 1
B/s -10.00 0.001 10 0
B/s -11.00 0.050 10 0
B/s -11.00 0.001 5 1
B/s +11.00 0.001 5 1
sec/op -11.00 0.001 10 0
sec/op +11.00 0.001 10 1
B/op -11.00 0.001 10 0
B/op +12.37 0.043 10 1
allocs/op -11.00 0.001 10 0
allocs/op +11.00 0.001 10 1
widgets/op +11.00 0.001 10 1
EOF

# Reset direction at every table, even when a cost table follows throughput.
# Retain the observed allocation finding from job121147231331: this repair
# must not turn its real B/op increase into a throughput improvement.
cat > "${WORK_DIR}/mixed.txt" <<'EOF'
          │ baseline.txt │     candidate.txt      │
          │ B/s          │ B/s vs base            │
TransferGain-4  100.0Mi ± 1%  111.0Mi ± 1%  +11.00% (p=0.001 n=10)
          │ baseline.txt │     candidate.txt      │
          │ sec/op       │ sec/op vs base         │
Slower-4  100.0n ± 1%  111.0n ± 1%  +11.00% (p=0.001 n=10)
          │ baseline.txt │     candidate.txt      │
          │ B/s          │ B/s vs base            │
TransferLoss-4  100.0Mi ± 1%  89.00Mi ± 1%  -11.00% (p=0.001 n=10)
          │ baseline.txt │     candidate.txt      │
          │ B/op         │ B/op vs base           │
BroadcastMetricsSnapshot/views-clean/json-frame/point-targets-4  63.15Mi ± 13%  70.96Mi ± 1%  +12.37% (p=0.043 n=10)
EOF
set +e
output="$(bash "${ROOT_DIR}/scripts/check-bench-regression.sh" "${WORK_DIR}/mixed.txt" 2>&1)"
status=$?
set -e
[[ "${status}" == 1 ]]
grep -qF '3 regressed benchmark(s):' <<<"${output}"
grep -qF '[sec/op] Slower-4' <<<"${output}"
grep -qF '[B/s] TransferLoss-4' <<<"${output}"
grep -qF '[B/op] BroadcastMetricsSnapshot/views-clean/json-frame/point-targets-4' <<<"${output}"
! grep -qF 'TransferGain-4' <<<"${output}"

echo "all CI benchmark tests passed"
