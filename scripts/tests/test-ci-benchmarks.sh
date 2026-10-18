#!/usr/bin/env bash
# Contract tests for paired CI benchmark collection and verdict quality.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${WORK_DIR}/bin" "${WORK_DIR}/candidate" "${WORK_DIR}/baseline"
cat > "${WORK_DIR}/bin/fixture.test" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == '-test.bench=.' ]] || exit 91
[[ "${FAKE_BINARY_FAIL:-0}" == 0 ]] || exit 23
echo 'BenchmarkExample-4  1  100 ns/op  0 B/op  0 allocs/op'
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
  [[ "${FAKE_GO_LIST_FAIL:-0}" == 0 ]] || exit 1
  if [[ "$2" == -m ]]; then
    echo "$PWD"
    exit 0
  fi
  package="${!#}"
  package="${package#./}"
  package="${package%/}"
  printf '%s/%s\n\n' "$PWD" "${package}"
  [[ "${package}" == pkg/auth ]] || echo "$PWD/internal/shared"
  exit 0
fi
printf '%s\t%s\n' "$PWD" "$*" >> "${FAKE_GO_LOG}"
args=("$@")
for ((i=0; i<${#args[@]}; i++)); do
  if [[ "${args[i]}" == -exec ]]; then
    "${args[i+1]}" "${FAKE_BINARY}" '-test.bench=.' || exit
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
run_paired
grep -qFx 'selection=affected' "${metadata}"
grep -qFx 'packages=' "${metadata}"
grep -qFx "candidate.commit=$(git -C "${WORK_DIR}/candidate" rev-parse HEAD)" "${metadata}"
grep -qFx "baseline.tree=$(git -C "${WORK_DIR}/baseline" rev-parse 'HEAD^{tree}')" "${metadata}"
[[ "$(grep -c '^sample=' "${metadata}" || true)" == 0 ]]
[[ ! -s "${WORK_DIR}/candidate/bench-results.txt" ]]
[[ ! -s "${WORK_DIR}/candidate/bench-baseline.txt" ]]
[[ ! -s "${WORK_DIR}/go.log" ]]

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

echo "all CI benchmark tests passed"
