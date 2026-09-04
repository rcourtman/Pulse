#!/usr/bin/env bash
# check-bench-regression.sh — Parse benchstat output for significant regressions.
#
# Usage: bash scripts/check-bench-regression.sh <benchstat-output-file>
# Exits 0 if no regressions >10% with p<0.05, exits 1 otherwise. Comparisons
# with fewer than 10 samples are rejected because benchstat cannot treat them
# as a statistically trustworthy CI signal.
#
# Expected input: benchstat comparison output containing lines like:
#   BenchmarkName-N  1.23µ ± 1%  1.45µ ± 2%  +17.89% (p=0.001 n=10)

set -euo pipefail

THRESHOLD=10   # percent
P_MAX="0.05"   # p-value significance level
MIN_SAMPLES=10  # benchstat's documented minimum for meaningful comparisons

COMPARISON_FILE="${1:-}"
if [ -z "$COMPARISON_FILE" ]; then
  echo "Usage: $0 <benchstat-output-file>"
  exit 1
fi

if [ ! -f "$COMPARISON_FILE" ]; then
  echo "Error: comparison file not found: $COMPARISON_FILE"
  exit 1
fi

TMPFILE=$(mktemp)
LOW_SAMPLE_FILE=$(mktemp)
trap 'rm -f "$TMPFILE" "$LOW_SAMPLE_FILE"' EXIT

# Refuse to turn an under-sampled comparison into a regression verdict. Five
# samples produce infinite 95% confidence intervals and were the source of
# repeated false failures on otherwise unrelated changes.
awk -v minimum="$MIN_SAMPLES" '
/p=[^)]*n=[0-9]+/ {
  match($0, /n=[0-9]+/)
  samples = substr($0, RSTART+2, RLENGTH-2) + 0
  if (samples < minimum) {
    print
  }
}' "$COMPARISON_FILE" > "$LOW_SAMPLE_FILE"

low_sample_count=$(wc -l < "$LOW_SAMPLE_FILE" | tr -d ' ')
if [ "$low_sample_count" -ne 0 ]; then
  echo "BENCHMARK EVIDENCE INSUFFICIENT"
  echo "==============================="
  echo "At least ${MIN_SAMPLES} samples are required; ${low_sample_count} comparison(s) are under-sampled:"
  sed 's/^/  /' "$LOW_SAMPLE_FILE"
  exit 1
fi

# Use POSIX-compatible awk to find statistically significant regressions.
# Matches lines with +XX.XX% (p=0.XXX ...) and checks both thresholds.
awk -v threshold="$THRESHOLD" -v p_max="$P_MAX" '
/\+[0-9].*%.*\(p=/ {
  match($0, /\+[0-9]+\.?[0-9]*%/)
  pct = substr($0, RSTART+1, RLENGTH-2) + 0

  match($0, /p=[0-9]+\.[0-9]+/)
  pval = substr($0, RSTART+2, RLENGTH-2) + 0

  if (pct > threshold && pval < p_max) {
    print
  }
}' "$COMPARISON_FILE" > "$TMPFILE"

count=$(wc -l < "$TMPFILE" | tr -d ' ')

if [ "$count" -eq 0 ]; then
  echo "No significant benchmark regressions detected (threshold: >${THRESHOLD}%, p<${P_MAX})."
  exit 0
else
  echo "BENCHMARK REGRESSION DETECTED"
  echo "============================="
  echo "Threshold: >${THRESHOLD}% with p<${P_MAX}"
  echo ""
  echo "$count regressed benchmark(s):"
  sed 's/^/  /' "$TMPFILE"
  echo ""
  echo "Reproduce the paired base/candidate comparison on the same idle host before"
  echo "classifying the change as intentional."
  exit 1
fi
