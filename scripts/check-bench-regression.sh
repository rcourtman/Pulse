#!/usr/bin/env bash
# check-bench-regression.sh — Parse benchstat output for significant regressions.
#
# Usage: bash scripts/check-bench-regression.sh <benchstat-output-file>
# Exits 0 if no regressions >10% with p<0.05, exits 1 otherwise. More time,
# bytes/op and allocs/op are worse; less byte throughput is worse. Comparisons
# with missing pairs or fewer than 10 samples on either side are rejected:
# absent evidence must not become a successful regression verdict.
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

# Refuse empty, one-sided, malformed and under-sampled comparisons before
# judging deltas. Benchstat can successfully print a table with only one
# input's rows, and an empty file used to say "no regressions". Ignore table
# headers, file configuration, footnotes and geomeans, not measured rows.
awk -v minimum="$MIN_SAMPLES" '
{
  row = $0
  sub(/^[[:space:]]+/, "", row)
  if (row !~ /^[^[:space:]]+[[:space:]]+[0-9]/ ||
      row ~ /^[^[:space:]]+:/ || row ~ /^geomean[[:space:]]/) {
    next
  }
  rows++
  intervals = row
  if (gsub(/±/, "", intervals) != 2 ||
      !match(row, /(~|[-+][0-9]+(\.[0-9]+)?%)[[:space:]]+\(p=[0-9]+\.[0-9]+ (n=[0-9]+|n1=[0-9]+ n2=[0-9]+)\)/)) {
    print "Unpaired or malformed benchmark: " row
    next
  }
  comparison = substr(row, RSTART, RLENGTH)
  match(comparison, /p=[0-9]+\.[0-9]+/)
  pvalue = substr(comparison, RSTART+2, RLENGTH-2) + 0
  if (pvalue > 1) {
    print "Invalid p-value: " row
    next
  }
  if (match(comparison, /n=[0-9]+/)) {
    samples = substr(comparison, RSTART+2, RLENGTH-2) + 0
    other_samples = samples
  } else {
    match(comparison, /n1=[0-9]+/)
    samples = substr(comparison, RSTART+3, RLENGTH-3) + 0
    match(comparison, /n2=[0-9]+/)
    other_samples = substr(comparison, RSTART+3, RLENGTH-3) + 0
  }
  if (samples < minimum || other_samples < minimum) {
    print "Under-sampled benchmark: " row
  }
}
END {
  if (rows == 0) {
    print "No paired benchmark comparisons found."
  }
}' "$COMPARISON_FILE" > "$LOW_SAMPLE_FILE"

low_sample_count=$(wc -l < "$LOW_SAMPLE_FILE" | tr -d ' ')
if [ "$low_sample_count" -ne 0 ]; then
  echo "BENCHMARK EVIDENCE INSUFFICIENT"
  echo "==============================="
  echo "Every measured row needs a complete comparison with at least ${MIN_SAMPLES} samples per side:"
  sed 's/^/  /' "$LOW_SAMPLE_FILE"
  exit 1
fi

# Benchstat prints a separate table for each metric. Go SetBytes benchmarks
# report byte throughput (normally tidied to B/s), where a decrease is worse.
# Other metrics, including headerless legacy comparisons, retain the existing
# lower-is-better rule. Include the unit in each finding so scaled values such
# as 63Mi cannot be mistaken for throughput when they actually mean bytes/op.
awk -v threshold="$THRESHOLD" -v p_max="$P_MAX" '
/vs base/ {
  header = $0
  gsub(/│/, "|", header)
  if (split(header, columns, /\|/) >= 3) {
    metric = columns[2]
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", metric)
    higher_is_better = (metric ~ /^([kKMGTPE]i?)?B\/s$/)
  }
}
/[-+][0-9].*%.*\(p=/ {
  match($0, /[-+][0-9]+\.?[0-9]*%/)
  pct = substr($0, RSTART, RLENGTH-1) + 0

  match($0, /p=[0-9]+\.[0-9]+/)
  pval = substr($0, RSTART+2, RLENGTH-2) + 0

  regression = higher_is_better ? -pct : pct
  if (regression > threshold && pval < p_max) {
    print (metric == "" ? "" : "[" metric "] ") $0
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
