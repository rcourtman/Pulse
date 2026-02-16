#!/usr/bin/env bash
#
# Pulse Lint Fixer v2 - Autonomous Package-Level Fixing
#
# Fixes golangci-lint errcheck and dupl warnings using OpenCode + MiniMax M2.5.
# OpenCode provides autonomous code editing with built-in verification.
#
# Key change: Gives OpenCode the FULL package+linter problem, not file-by-file micromanagement.
# Let the AI decide how to approach the fixes - it's smarter than our loops.

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd -P)"
SCRIPT_DIR="$REPO_DIR/scripts/lint-fixer"

MODEL="${MODEL:-opencode/minimax-m2.5-free}"
LINTERS="${LINTERS:-errcheck,dupl}"
PACKAGES="${PACKAGES:-}"  # Empty = all internal packages
BRANCH=""
USE_WORKTREE=false
AIDER_TIMEOUT_SECS="${AIDER_TIMEOUT_SECS:-900}"

LOG_FILE="/tmp/lint-fixer-runner.log"
PROGRESS_FILE="$SCRIPT_DIR/PROGRESS.md"

# Parse args
while [[ $# -gt 0 ]]; do
  case $1 in
    --branch)      BRANCH="$2"; shift 2 ;;
    --worktree)    USE_WORKTREE=true; shift ;;
    --no-worktree) USE_WORKTREE=false; shift ;;
    *)             echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# Determine branch
if [ -z "$BRANCH" ]; then
  BRANCH="lint-fixes/$(date +%Y%m%d-%H%M%S)"
fi

if [ "$BRANCH" = "current" ]; then
  BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi

# Find golangci-lint
if ! command -v golangci-lint >/dev/null 2>&1; then
  GOLANGCI_LINT="$HOME/go/bin/golangci-lint"
  if [ ! -f "$GOLANGCI_LINT" ]; then
    echo "Error: golangci-lint not found"
    exit 1
  fi
else
  GOLANGCI_LINT="golangci-lint"
fi

# Find opencode
if ! command -v opencode >/dev/null 2>&1; then
  OPENCODE="/opt/homebrew/bin/opencode"
  if [ ! -f "$OPENCODE" ]; then
    echo "Error: opencode not found"
    exit 1
  fi
else
  OPENCODE="opencode"
fi

# Helper: run with timeout
run_with_timeout() {
  local timeout_secs=$1
  shift
  timeout "$timeout_secs" "$@"
}

# ── Header ──────────────────────────────────────────────────────────────

{
  echo "╔══════════════════════════════════════════════════════╗"
  echo "║     Pulse Lint Fixer v2 (Autonomous)                 ║"
  echo "╠══════════════════════════════════════════════════════╣"
  printf "║  Model:       %-39s║\n" "$MODEL"
  printf "║  Linters:     %-39s║\n" "$LINTERS"
  printf "║  Branch:      %-39s║\n" "$BRANCH"
  printf "║  Started:     %-39s║\n" "$(date)"
  echo "╚══════════════════════════════════════════════════════╝"
  echo ""
} | tee "$LOG_FILE"

cd "$REPO_DIR"

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
  git checkout -b "$BRANCH" 2>/dev/null || git checkout "$BRANCH"
  echo "Working on branch: $BRANCH"
else
  echo "Working on current branch: $BRANCH"
fi
echo "Base commit: $(git rev-parse HEAD)"
echo ""

# ── Scan for warnings ──────────────────────────────────────────────────

echo "Scanning for lint warnings..."

IFS=',' read -ra LINT_ARRAY <<< "$LINTERS"
SCAN_TARGETS=("./internal/...")

# Build package warning map
PKG_WARNING_TMP=$(mktemp -t lint-fixer-pkg-warnings.XXXXXX)
for linter in "${LINT_ARRAY[@]}"; do
  for target in "${SCAN_TARGETS[@]}"; do
    while IFS= read -r line; do
      if [[ "$line" =~ ^internal/([^/]+)/ ]]; then
        pkg="${BASH_REMATCH[1]}"
        echo "$pkg" >> "$PKG_WARNING_TMP"
      fi
    done < <($GOLANGCI_LINT run --enable "$linter" --disable-all "$target" 2>&1 | grep "$linter" || true)
  done
done

# Get unique packages sorted by warning count
PACKAGES_TO_FIX=($(sort "$PKG_WARNING_TMP" | uniq -c | sort -n | awk '{print $2}'))
PACKAGE_COUNTS=($(sort "$PKG_WARNING_TMP" | uniq -c | sort -n | awk '{print $1}'))
rm -f "$PKG_WARNING_TMP"

if [ ${#PACKAGES_TO_FIX[@]} -eq 0 ]; then
  echo "✓ No warnings found!"
  exit 0
fi

echo "Found ${#PACKAGES_TO_FIX[@]} packages with warnings:"
for idx in "${!PACKAGES_TO_FIX[@]}"; do
  pkg="${PACKAGES_TO_FIX[$idx]}"
  count="${PACKAGE_COUNTS[$idx]}"
  echo "  internal/$pkg: $count warnings"
done
echo ""

# ── Process packages ───────────────────────────────────────────────────

TOTAL_COMMITS=0
START_TIME=$(date +%s)

for pkg in "${PACKAGES_TO_FIX[@]}"; do
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Package: internal/$pkg"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""

  for linter in "${LINT_ARRAY[@]}"; do
    echo "→ Running $linter on internal/$pkg..."

    # Get ALL warnings for this package+linter
    WARNINGS=$($GOLANGCI_LINT run --enable "$linter" --disable-all "./internal/$pkg/..." 2>&1 | grep "$linter" || true)

    if [ -z "$WARNINGS" ]; then
      echo "  ✓ No $linter warnings"
      continue
    fi

    WARNING_COUNT=$(echo "$WARNINGS" | wc -l | tr -d ' ')
    echo "  Found $WARNING_COUNT $linter warnings"

    # Build prompt - give OpenCode the FULL problem
    case "$linter" in
      errcheck)
        PROMPT="## Task: Fix all $WARNING_COUNT errcheck warnings in internal/$pkg

**Why this matters:** Unchecked errors can cause silent failures, data corruption, or security issues in production. We need proper error handling to make Pulse reliable for users monitoring critical infrastructure.

**All errcheck warnings in this package:**
\`\`\`
$WARNINGS
\`\`\`

**Your approach:**
You decide how to tackle these - all at once, group by file, whatever makes sense to you.

**Guidelines:**

For **test files** (\`*_test.go\`):
- If the error is truly irrelevant (e.g., \`defer file.Close()\` in test setup), use: \`_ = expr\`

For **production code**:
- If function returns \`error\`: propagate it
  - Good: \`return fmt.Errorf(\"failed to save: %w\", err)\`
- If function doesn't return error: log and continue
  - Example: \`if err := conn.Close(); err != nil { log.Warn(\"close failed: %v\", err) }\`
- For critical operations: ALWAYS handle errors

**Constraints:**
- Do NOT change function signatures
- Preserve all existing behavior
- Ensure tests pass

**Expected output:** All errors properly handled."
        ;;

      dupl)
        PROMPT="## Task: Eliminate all $WARNING_COUNT code duplication warnings in internal/$pkg

**Why this matters:** Duplicated code creates maintenance burden. When we fix bugs or add features, we have to find and update multiple copies, which leads to inconsistencies.

**All dupl warnings in this package:**
\`\`\`
$WARNINGS
\`\`\`

**Your approach:**
Figure out how to best eliminate these duplications - extract helpers, refactor shared logic, whatever makes sense to you.

**Guidelines:**
1. **Identify** duplicated logic blocks
2. **Extract** into well-named helper functions:
   - Use descriptive names (e.g., \`buildMetricPayload\`, \`formatTimestamp\`)
   - Keep helpers private unless they need export
   - Place near call sites
3. **Replace** duplicated code with helper calls

**Constraints:**
- Do NOT over-abstract
- Preserve exact behavior
- Ensure tests pass

**Expected output:** DRY code with clear, well-named helpers."
        ;;

      *)
        PROMPT="## Task: Fix all $WARNING_COUNT $linter warnings in internal/$pkg

**All warnings:**
\`\`\`
$WARNINGS
\`\`\`

**Instructions:**
Fix these warnings. Keep changes minimal and ensure tests pass."
        ;;
    esac

    echo "  Calling AI model..." | tee -a "$LOG_FILE"

    # Run OpenCode - let it figure out the approach
    if /opt/homebrew/bin/opencode run \
      --model "$MODEL" \
      "$PROMPT" >> "$LOG_FILE" 2>&1; then
      echo "  ✓ AI completed successfully" | tee -a "$LOG_FILE"
    else
      AI_EXIT_CODE=$?
      if [ "$AI_EXIT_CODE" -eq 124 ]; then
        echo "  ✗ AI call timed out after ${AIDER_TIMEOUT_SECS}s" | tee -a "$LOG_FILE"
      else
        echo "  ✗ AI call failed (exit $AI_EXIT_CODE)" | tee -a "$LOG_FILE"
      fi
      continue
    fi

    # Check if changes were made
    if ! git diff --quiet HEAD -- './internal'; then
      echo "  Testing changes..." | tee -a "$LOG_FILE"

      # Run package tests
      if go test "./internal/$pkg/..." >> "$LOG_FILE" 2>&1; then
        echo "  ✓ Tests passed" | tee -a "$LOG_FILE"

        COMMIT_MSG="fix($linter): internal/$pkg ($WARNING_COUNT warnings)

Model: $MODEL
Context: Pulse infrastructure monitoring platform"

        git add "./internal/$pkg/"
        git commit -m "$COMMIT_MSG" >> "$LOG_FILE" 2>&1
        TOTAL_COMMITS=$((TOTAL_COMMITS + 1))
        echo "  ✓ Committed ($TOTAL_COMMITS total)" | tee -a "$LOG_FILE"
      else
        echo "  ✗ Tests failed — reverting changes" | tee -a "$LOG_FILE"
        git checkout HEAD -- "./internal/$pkg/"
      fi
    else
      echo "  ⚠ No changes made by AI"
    fi

    echo ""
  done
done

# ── Summary ────────────────────────────────────────────────────────────

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
DURATION_MIN=$((DURATION / 60))
DURATION_SEC=$((DURATION % 60))

{
  echo ""
  echo ""
  echo "╔══════════════════════════════════════════════════════╗"
  echo "║  Lint Fixer Complete                                 ║"
  echo "╠══════════════════════════════════════════════════════╣"
  printf "║  Commits:   %-41s║\n" "$TOTAL_COMMITS"
  printf "║  Duration:  %-41s║\n" "${DURATION_MIN}h ${DURATION_SEC}m"
  echo "╚══════════════════════════════════════════════════════╝"
  echo ""
} | tee -a "$LOG_FILE"

echo "Full log: $LOG_FILE"
