#!/usr/bin/env bash
# check-bench-regression.sh — Parse benchstat output for significant regressions.
#
# Usage: bash scripts/check-bench-regression.sh <benchstat-output-file>
# Exits 0 if no regressions >10% with p<0.05, exits 1 otherwise. More time,
# bytes/op and allocs/op are worse; less byte throughput is worse. A significant
# increase from zero in a cost metric is also a regression: the percent change
# is undefined. Comparisons with missing pairs or fewer than 10 samples on
# either side are rejected.
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

# Parse the evidence and judge the same delta field, not a sign elsewhere in
# the row (for example in a benchmark label). Incomplete evidence takes
# precedence over the regression verdict below.
LC_ALL=C awk -v minimum="$MIN_SAMPLES" -v threshold="$THRESHOLD" -v p_max="$P_MAX" \
  -v insufficient="$LOW_SAMPLE_FILE" -v findings="$TMPFILE" '
BEGIN {
  number = "[0-9]+(\\.[0-9]+)?"
  scale = "(m|µ|n|p|f|a|z|y|[kKMGTPEZY]i?)?"
  interval = "(" number "%|\\?|∞)([[:space:]]+(⁰|¹|²|³|⁴|⁵|⁶|⁷|⁸|⁹)+)?"
}
# When benchstat prints ? for the delta, require two real measurement columns
# with a zero base and a positive candidate. A ? in an interval is allowed:
# benchstat cannot express a relative interval around some near-zero medians.
function zero_to_positive(prefix, columns, candidate) {
  if (split(prefix, columns, /[[:space:]]+±[[:space:]]+/) != 3 ||
      columns[1] !~ ("^[^[:space:]]+[[:space:]]+0(\\.0+)?" scale "$") ||
      columns[2] !~ ("^" interval "[[:space:]]+" number scale "$") ||
      columns[3] !~ ("^" interval "$")) {
    return 0
  }
  candidate = columns[2]
  sub(/^.*[[:space:]]/, "", candidate)
  return candidate + 0 > 0
}
{
  row = $0
  sub(/^[[:space:]]+/, "", row)
  # Go SetBytes reports throughput (normally tidied to B/s), where a
  # decrease is worse. Other metrics and headerless input are costs.
  if (row ~ /^(│|\|).*vs base/) {
    header = row
    gsub(/│/, "|", header)
    if (split(header, columns, /\|/) >= 3) {
      metric = columns[2]
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", metric)
      higher_is_better = (metric ~ /^([kKMGTPE]i?)?B\/s$/)
    }
    next
  }
  if (row !~ /^[^[:space:]]+[[:space:]]+[0-9]/ ||
      row ~ /^[^[:space:]]+:[[:space:]]/ || row ~ /^geomean[[:space:]]/) {
    next
  }
  rows++
  intervals = row
  # Only a change rounded to zero can be unsigned. Match the entire delta
  # token so its suffix cannot impersonate a zero.
  if (gsub(/±/, "", intervals) != 2 ||
      !match(row, /[[:space:]]+(~|0(\.0+)?%|[-+][0-9]+(\.[0-9]+)?%|\?)[[:space:]]+\(p=[0-9]+\.[0-9]+ (n=[0-9]+|n1=[0-9]+ n2=[0-9]+)\)/)) {
    print "Unpaired or malformed benchmark: " row > insufficient
    next
  }
  prefix = substr(row, 1, RSTART-1)
  comparison = substr(row, RSTART, RLENGTH)
  delta = comparison
  sub(/^[[:space:]]+/, "", delta)
  sub(/[[:space:]].*$/, "", delta)
  if (delta == "?" && !zero_to_positive(prefix)) {
    print "Cannot determine direction of undefined benchmark delta: " row > insufficient
    next
  }
  match(comparison, /p=[0-9]+\.[0-9]+/)
  pvalue = substr(comparison, RSTART+2, RLENGTH-2) + 0
  if (pvalue > 1) {
    print "Invalid p-value: " row > insufficient
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
    print "Under-sampled benchmark: " row > insufficient
    next
  }
  if (pvalue >= p_max) {
    next
  }
  label = metric == "" ? "" : "[" metric "] "
  if (delta == "?") {
    if (!higher_is_better) {
      print label row " (zero baseline; percent change undefined)" > findings
    }
  } else if (delta ~ /^[-+]/) {
    pct = delta + 0
    if ((higher_is_better ? -pct : pct) > threshold) {
      print label row > findings
    }
  }
}
END {
  if (rows == 0) {
    print "No paired benchmark comparisons found." > insufficient
  }
}' "$COMPARISON_FILE"

low_sample_count=$(wc -l < "$LOW_SAMPLE_FILE" | tr -d ' ')
if [ "$low_sample_count" -ne 0 ]; then
  echo "BENCHMARK EVIDENCE INSUFFICIENT"
  echo "==============================="
  echo "Every measured row needs a complete comparison with at least ${MIN_SAMPLES} samples per side:"
  sed 's/^/  /' "$LOW_SAMPLE_FILE"
  exit 1
fi

count=$(wc -l < "$TMPFILE" | tr -d ' ')

if [ "$count" -eq 0 ]; then
  echo "No significant benchmark regressions detected (threshold: >${THRESHOLD}% or cost increase from zero, p<${P_MAX})."
  exit 0
else
  echo "BENCHMARK REGRESSION DETECTED"
  echo "============================="
  echo "Threshold: >${THRESHOLD}% or cost increase from zero, with p<${P_MAX}"
  echo ""
  echo "$count regressed benchmark(s):"
  sed 's/^/  /' "$TMPFILE"
  echo ""
  echo "Reproduce the paired base/candidate comparison on the same idle host before"
  echo "classifying the change as intentional."
  exit 1
fi
