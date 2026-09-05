#!/usr/bin/env bash
# Contract tests for paired CI benchmark collection and verdict quality.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${WORK_DIR}/bin" "${WORK_DIR}/candidate" "${WORK_DIR}/baseline"
cat > "${WORK_DIR}/bin/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\t%s\n' "$PWD" "$*" >> "${FAKE_GO_LOG}"
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
