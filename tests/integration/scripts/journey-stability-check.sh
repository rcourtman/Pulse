#!/usr/bin/env bash
# journey-stability-check.sh — Run L12 journey tests N times and report pass rate.
#
# Usage (from workspace root):
#   bash tests/integration/scripts/journey-stability-check.sh [RUNS]
#
# RUNS defaults to 7 (the L12 score-8 SLO requirement).
# Pass rate must be >=95% for score 8.
#
# Outputs a JSON summary to tests/integration/test-results/stability-report.json

set -euo pipefail

# Resolve workspace root relative to this script's location.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$REPO_ROOT"

RUNS="${1:-7}"

if ! [[ "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: RUNS must be a positive integer (got: $RUNS)" >&2
  exit 1
fi

PASS=0
FAIL=0
RESULTS=()

REPORT_DIR="tests/integration/test-results"
mkdir -p "$REPORT_DIR"

echo "=== L12 Journey Stability Check ==="
echo "Workspace: $REPO_ROOT"
echo "Running $RUNS consecutive iterations..."
echo ""

for i in $(seq 1 "$RUNS"); do
  echo "--- Run $i/$RUNS ---"
  START=$(date +%s)

  if npx playwright test tests/journeys --project=chromium --reporter=list 2>&1 | tail -5; then
    STATUS="pass"
    PASS=$((PASS + 1))
  else
    STATUS="fail"
    FAIL=$((FAIL + 1))
  fi

  END=$(date +%s)
  DURATION=$((END - START))
  RESULTS+=("{\"run\":$i,\"status\":\"$STATUS\",\"duration_s\":$DURATION}")
  echo "Run $i: $STATUS (${DURATION}s)"
  echo ""
done

TOTAL=$((PASS + FAIL))
if [ "$TOTAL" -gt 0 ]; then
  PASS_RATE=$(echo "scale=1; $PASS * 100 / $TOTAL" | bc)
else
  PASS_RATE="0"
fi

SLO_MET="false"
if echo "$PASS_RATE >= 95.0" | bc -l | grep -q 1; then
  SLO_MET="true"
fi

echo "=== Stability Report ==="
echo "Passed: $PASS / $TOTAL"
echo "Pass rate: ${PASS_RATE}%"
echo "SLO (>=95%): $SLO_MET"

# Build JSON array of results
RESULTS_JSON="["
for idx in "${!RESULTS[@]}"; do
  if [ "$idx" -gt 0 ]; then
    RESULTS_JSON+=","
  fi
  RESULTS_JSON+="${RESULTS[$idx]}"
done
RESULTS_JSON+="]"

cat > "$REPORT_DIR/stability-report.json" <<EOF
{
  "lane": "L12",
  "score_gate": 8,
  "slo_target_pct": 95,
  "total_runs": $TOTAL,
  "passed": $PASS,
  "failed": $FAIL,
  "pass_rate_pct": $PASS_RATE,
  "slo_met": $SLO_MET,
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "runs": $RESULTS_JSON
}
EOF

echo ""
echo "Report written to $REPORT_DIR/stability-report.json"

if [ "$SLO_MET" = "true" ]; then
  echo "RESULT: SLO MET — score 8 criteria satisfied"
  exit 0
else
  echo "RESULT: SLO NOT MET — need >=95% pass rate"
  exit 1
fi
