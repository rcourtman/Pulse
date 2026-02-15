#!/usr/bin/env bash
set -euo pipefail

# ── Overnight Lint Fixer — errcheck + dupl ─────────────────────────────
#
# Fixes golangci-lint errcheck and dupl warnings using OpenCode + MiniMax M2.5.
# OpenCode provides autonomous code editing with built-in verification.
#
# Usage:
#   export OPENROUTER_API_KEY="sk-or-v1-..."
#   ./scripts/lint-fixer/run.sh
#
# Options:
#   --model MODEL       OpenCode model (default: opencode/minimax-m2.5-free)
#   --lint LINTERS      Comma-separated linters to fix (default: errcheck,dupl)
#   --packages PKGS     Comma-separated packages to target (default: all with warnings)
#   --max-iters N       Max iterations (default: 100)
#   --branch NAME       Branch name (default: lint-fixes/TIMESTAMP, use "current" for current branch)
#   --no-worktree       Work directly on current branch (DANGEROUS — no isolation)
#
# Environment:
#   OPENROUTER_API_KEY  Required. Get one at https://openrouter.ai/keys
#
# Overnight execution:
#   nohup ./scripts/lint-fixer/run.sh > /tmp/lint-fixer.log 2>&1 &
#
# Graceful shutdown:
#   touch scripts/lint-fixer/.stop
#
# Monitoring:
#   ./scripts/lint-fixer/watch.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORKTREE_BASE="$REPO_DIR/../pulse-lint-fixes"

# Survive terminal disconnection
trap '' HUP

# Defaults
MODEL="opencode/minimax-m2.5-free"
LINTERS="errcheck,dupl"
PACKAGES=""
MAX_ITERS=100
BRANCH=""
USE_WORKTREE=false  # Default to no worktree (simpler, avoids build issues)
AIDER_TIMEOUT_SECS="${AIDER_TIMEOUT_SECS:-900}"

while [[ $# -gt 0 ]]; do
  case $1 in
    --model)       MODEL="$2"
                   # Normalize model name to include openrouter/ prefix.
                   if [[ "$MODEL" == minimax/* ]] || [[ "$MODEL" == deepseek/* ]]; then
                     MODEL="openrouter/$MODEL"
                   fi
                   shift 2 ;;
    --lint)        LINTERS="$2"; shift 2 ;;
    --packages)    PACKAGES="$2"; shift 2 ;;
    --max-iters)   MAX_ITERS="$2"; shift 2 ;;
    --branch)      BRANCH="$2"; shift 2 ;;
    --worktree)    USE_WORKTREE=true; shift ;;
    --no-worktree) USE_WORKTREE=false; shift ;;
    --engine)      ENGINE="$2"; shift 2 ;;
    *)             echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# Validate environment
if [ -z "${OPENROUTER_API_KEY:-}" ]; then
  echo "Error: OPENROUTER_API_KEY not set"
  echo "  Get one at: https://openrouter.ai/keys"
  echo "  Then: export OPENROUTER_API_KEY='sk-or-v1-...'"
  exit 1
fi

if ! command -v opencode >/dev/null 2>&1; then
  echo "Error: opencode not found"
  echo "  Install with: brew install opencode"
  exit 1
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  GOLANGCI_LINT="$HOME/go/bin/golangci-lint"
  if [ ! -f "$GOLANGCI_LINT" ]; then
    echo "Error: golangci-lint not found in PATH or ~/go/bin/"
    exit 1
  fi
else
  GOLANGCI_LINT="golangci-lint"
fi

cd "$REPO_DIR"

# Generate branch name if not provided
if [ -z "$BRANCH" ]; then
  BRANCH="lint-fixes/$(date +%Y%m%d-%H%M%S)"
fi

# Special handling: if branch is "current", use the current branch
if [ "$BRANCH" = "current" ]; then
  BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi

STOP_FILE="$SCRIPT_DIR/.stop"
PROGRESS_FILE="$SCRIPT_DIR/PROGRESS.md"
LOG_FILE="$SCRIPT_DIR/run.log"

rm -f "$STOP_FILE"
mkdir -p "$(dirname "$LOG_FILE")"

should_stop() {
  [ -f "$STOP_FILE" ]
}

# Run a command with a hard timeout (bash 3 compatible).
run_with_timeout() {
  local timeout_secs="$1"
  shift

  "$@" &
  local cmd_pid=$!
  local start_time
  start_time=$(date +%s)

  while kill -0 "$cmd_pid" 2>/dev/null; do
    if [ $(( $(date +%s) - start_time )) -ge "$timeout_secs" ]; then
      kill "$cmd_pid" 2>/dev/null || true
      sleep 2
      kill -9 "$cmd_pid" 2>/dev/null || true
      wait "$cmd_pid" 2>/dev/null || true
      return 124
    fi
    sleep 2
  done

  wait "$cmd_pid"
}

STARTED_AT=$(date +%s)

echo "╔══════════════════════════════════════════════════════╗"
echo "║     Pulse Overnight Lint Fixer                       ║"
echo "╠══════════════════════════════════════════════════════╣"
echo "║  Model:       $MODEL"
echo "║  Linters:     $LINTERS"
echo "║  Branch:      $BRANCH"
echo "║  Max iters:   $MAX_ITERS"
echo "║  Started:     $(date)"
echo "╚══════════════════════════════════════════════════════╝"
echo ""

# Set up worktree or work in-place
if [ "$USE_WORKTREE" = true ]; then
  WORK_DIR="$WORKTREE_BASE"

  if [ -d "$WORK_DIR" ]; then
    git worktree remove --force "$WORK_DIR" 2>/dev/null || rm -rf "$WORK_DIR"
  fi

  BASE_SHA=$(git rev-parse HEAD)
  git worktree add -b "$BRANCH" "$WORK_DIR" "$BASE_SHA" 2>/dev/null

  echo "Working in isolated worktree: $WORK_DIR"
  echo "Base commit: $BASE_SHA"
  echo ""

  cd "$WORK_DIR"

  # Copy frontend dist if it exists (required for embed directives)
  if [ -d "$REPO_DIR/frontend-modern/dist" ]; then
    echo "Copying frontend-modern/dist for Go build..."
    cp -r "$REPO_DIR/frontend-modern/dist" frontend-modern/
  fi

  # Build internal/ packages to populate build cache
  echo "Building internal/ packages..."
  if go build ./internal/... >> "$LOG_FILE" 2>&1; then
    echo "✓ Build successful"
  else
    echo "Warning: go build failed, some linters may not work correctly"
  fi
  echo ""
else
  WORK_DIR="$REPO_DIR"
  BASE_SHA=$(git rev-parse HEAD)

  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
    git checkout -b "$BRANCH" 2>/dev/null || git checkout "$BRANCH"
    echo "Working directly in repo on branch: $BRANCH"
  else
    echo "Working directly in repo on current branch: $BRANCH"
  fi
  echo "Base commit: $BASE_SHA"
  echo ""
fi

# Initialize progress file
cat > "$PROGRESS_FILE" << 'PROGRESS_EOF'
# Lint Fixer Progress

## Status
- **Started**: (timestamp)
- **Packages completed**: 0
- **Commits**: 0
- **Warnings fixed**: 0

## Package Log
(will be populated as we go)
PROGRESS_EOF

sed -i.bak "s/(timestamp)/$(date)/" "$PROGRESS_FILE" && rm -f "$PROGRESS_FILE.bak"

# ── Get list of packages with warnings ────────────────────────────────

echo "Scanning for lint warnings..."

# Parse LINTERS into array
IFS=',' read -ra LINT_ARRAY <<< "$LINTERS"

# Parse requested packages (optional)
REQUESTED_PACKAGES=()
if [ -n "$PACKAGES" ]; then
  IFS=',' read -ra RAW_PACKAGES <<< "$PACKAGES"
  for raw_pkg in "${RAW_PACKAGES[@]}"; do
    raw_pkg="$(echo "$raw_pkg" | xargs)"
    [ -n "$raw_pkg" ] || continue
    raw_pkg="${raw_pkg#./}"
    raw_pkg="${raw_pkg#internal/}"
    raw_pkg="${raw_pkg%/...}"
    raw_pkg="${raw_pkg%/}"
    [ -n "$raw_pkg" ] || continue
    REQUESTED_PACKAGES+=("$raw_pkg")
  done
fi

SCAN_TARGETS=()
if [ ${#REQUESTED_PACKAGES[@]} -gt 0 ]; then
  for pkg in "${REQUESTED_PACKAGES[@]}"; do
    SCAN_TARGETS+=("./internal/$pkg/...")
  done
else
  SCAN_TARGETS+=("./internal/...")
fi

# Build package list (bash 3 compatible: avoid associative arrays)
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

# Convert to sorted array (by warning count ascending for quick wins first)
PACKAGES_TO_FIX=()
PACKAGE_WARNING_COUNTS=()
if [ -s "$PKG_WARNING_TMP" ]; then
  while read -r count pkg; do
    [ -n "$pkg" ] || continue
    PACKAGES_TO_FIX+=("$pkg")
    PACKAGE_WARNING_COUNTS+=("$count")
  done < <(sort "$PKG_WARNING_TMP" | uniq -c | awk '{print $1 " " $2}' | sort -n)
fi
rm -f "$PKG_WARNING_TMP"

if [ ${#PACKAGES_TO_FIX[@]} -eq 0 ]; then
  echo "No packages with warnings found!"
  exit 0
fi

echo "Found ${#PACKAGES_TO_FIX[@]} packages with warnings:"
for idx in "${!PACKAGES_TO_FIX[@]}"; do
  pkg="${PACKAGES_TO_FIX[$idx]}"
  pkg_warning_count="${PACKAGE_WARNING_COUNTS[$idx]}"
  echo "  internal/$pkg: ${pkg_warning_count} warnings"
done
echo ""

# ── Main loop: fix packages one by one ────────────────────────────────

TOTAL_COMMITS=0
TOTAL_WARNINGS_FIXED=0
PACKAGES_COMPLETED=0

for idx in "${!PACKAGES_TO_FIX[@]}"; do
  pkg="${PACKAGES_TO_FIX[$idx]}"
  pkg_warning_count="${PACKAGE_WARNING_COUNTS[$idx]}"
  if should_stop; then
    echo "Stop signal received — shutting down gracefully"
    break
  fi

  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Package: internal/$pkg (${pkg_warning_count} warnings)"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""

  PKG_START=$(date +%s)
  BEFORE_SHA=$(git rev-parse HEAD)

  for linter in "${LINT_ARRAY[@]}"; do
    if should_stop; then break; fi

    echo "→ Running $linter on internal/$pkg..."

    # Get warnings for this package + linter
    WARNINGS=$($GOLANGCI_LINT run --enable "$linter" --disable-all "./internal/$pkg/..." 2>&1 | grep "$linter" || true)

    if [ -z "$WARNINGS" ]; then
      echo "  ✓ No $linter warnings"
      continue
    fi

    WARNING_COUNT=$(echo "$WARNINGS" | wc -l | tr -d ' ')
    echo "  Found $WARNING_COUNT $linter warnings"

    # Extract affected files and process one file at a time to avoid model context overflow.
    FILES=$(echo "$WARNINGS" | awk -F: '/^[^:]+\.go:/{print $1}' | sort -u || true)
    if [ -z "$FILES" ]; then
      echo "  ⚠ Could not extract file list from warnings"
      continue
    fi

    FILE_COUNT=$(echo "$FILES" | wc -l | tr -d ' ')
    echo "  Affected files: $FILE_COUNT"

    while IFS= read -r file; do
      if should_stop; then break; fi
      [ -n "$file" ] || continue

      if [ ! -f "$file" ]; then
        echo "  ⚠ Skipping missing file: $file"
        continue
      fi

      FILE_WARNINGS=$(echo "$WARNINGS" | awk -v f="$file" 'index($0, f ":") == 1')
      if [ -z "$FILE_WARNINGS" ]; then
        continue
      fi
      FILE_WARNING_COUNT=$(echo "$FILE_WARNINGS" | wc -l | tr -d ' ')
      echo "  → File: $file ($FILE_WARNING_COUNT warnings)"

      # Build MiniMax M2.5 optimized prompt (purpose-driven, explain "why")
      case "$linter" in
        errcheck)
          PROMPT="## Task: Fix errcheck warnings in $file (internal/$pkg)

**Why this matters:** Unchecked errors can cause silent failures, data corruption, or security issues in production. We need proper error handling to make Pulse reliable for users monitoring critical infrastructure.

**Context:** This is production code for Pulse, an infrastructure monitoring platform. Users depend on accurate metrics and reliable operations. Every unchecked error is a potential production incident.

**Warnings to fix (this file only):**
\`\`\`
$FILE_WARNINGS
\`\`\`

**How to fix them:**

For **test files** (\`*_test.go\`):
- If the error is truly irrelevant (e.g., \`defer file.Close()\` in test setup), use: \`_ = expr\`
- Example: \`_ = os.Remove(tmpFile)\` when cleanup failure doesn't affect test validity

For **production code**:
- If function returns \`error\`: propagate it
  - Good: \`return fmt.Errorf(\"failed to save config: %w\", err)\`
  - Bad: ignoring the error
- If function doesn't return error but should handle it: log and continue gracefully
  - Example: \`if err := conn.Close(); err != nil { log.Warn(\"connection close failed: %v\", err) }\`
- For critical operations (database writes, API calls, file I/O): ALWAYS handle errors

**Constraints:**
- Do NOT change function signatures
- Do NOT add new imports unless absolutely necessary
- Keep changes minimal — fix only the error handling for this file
- Preserve all existing behavior
- Ensure all tests still pass

**Expected output:** Clean, production-ready code with all errors properly handled."
          ;;

        dupl)
          PROMPT="## Task: Eliminate code duplication tied to warnings in $file (internal/$pkg)

**Why this matters:** Duplicated code creates maintenance burden. When we fix a bug or add a feature, we have to find and update multiple copies, which leads to inconsistencies and missed updates. DRY code is easier to maintain and less error-prone.

**Context:** This is production code for Pulse. We're refactoring to improve long-term maintainability without changing any behavior.

**Duplication warnings (anchored to this file):**
\`\`\`
$FILE_WARNINGS
\`\`\`

**How to fix them:**

1. **Identify** the duplicated logic blocks
2. **Extract** into well-named helper functions:
   - Use descriptive names that explain what they do (e.g., \`buildProxmoxMetricPayload\`, \`formatTimestampISO8601\`)
   - Keep helpers private (lowercase) unless they need to be exported
   - Place helpers near their call sites (same file if only used there, separate file if shared across package)
   - Add brief comment explaining purpose if not obvious from name

3. **Replace** duplicated code with calls to the helper

**Constraints:**
- Do NOT over-abstract — only extract what's actually duplicated
- Preserve exact behavior (no logic changes)
- Ensure all tests still pass
- Keep changes minimal

**Expected output:** DRY code with shared logic extracted into clear, well-named helper functions."
          ;;

        *)
          PROMPT="## Task: Fix $linter warnings in $file (internal/$pkg)

**Warnings (this file only):**
\`\`\`
$FILE_WARNINGS
\`\`\`

**Instructions:**
Keep changes minimal and focused. Do not change function signatures or add unnecessary imports. Fix only what the linter flagged."
          ;;
      esac

      echo "    Calling AI model..."

      # Run OpenCode with MiniMax M2.5
      # OpenCode automatically applies changes and verifies builds
      if run_with_timeout "$AIDER_TIMEOUT_SECS" opencode run \
        --model "$MODEL" \
        "$PROMPT

File to fix: $file

After making changes, run 'go test ./internal/$pkg/...' to verify the code compiles and tests pass." >> "$LOG_FILE" 2>&1; then
        echo "    ✓ AI completed successfully"
      else
        AI_EXIT_CODE=$?
        if [ "$AI_EXIT_CODE" -eq 124 ]; then
          echo "    ✗ AI call timed out after ${AIDER_TIMEOUT_SECS}s"
        else
          echo "    ✗ AI call failed (exit $AI_EXIT_CODE)"
        fi
        continue
      fi

      # Check if changes were ACTUALLY made
      # OpenCode should only modify code files, not metadata
      if ! git diff --quiet HEAD -- './internal'; then
        echo "    Testing changes..."

        # Run package tests
        if go test "./internal/$pkg/..." >> "$LOG_FILE" 2>&1; then
          echo "    ✓ Tests passed"

          FILE_BASENAME="$(basename "$file")"
          COMMIT_MSG="fix($linter): $FILE_BASENAME in internal/$pkg

Fixed $FILE_WARNING_COUNT $linter warnings in $file.

Model: $MODEL
Context: Pulse infrastructure monitoring platform"

          # Stage only internal/ files (exclude PROGRESS.md, session files, etc.)
          git add internal/

          if ! git diff --cached --quiet; then
            git commit -m "$COMMIT_MSG" >> "$LOG_FILE" 2>&1

            TOTAL_COMMITS=$((TOTAL_COMMITS + 1))
            TOTAL_WARNINGS_FIXED=$((TOTAL_WARNINGS_FIXED + FILE_WARNING_COUNT))

            echo "    ✓ Committed ($TOTAL_COMMITS total)"
          else
            echo "    ⚠ No changes to commit (AI may have only added comments)"
            git reset HEAD internal/ >/dev/null 2>&1
          fi
        else
          echo "    ✗ Tests failed — reverting changes"
          git checkout HEAD -- internal/ >> "$LOG_FILE" 2>&1
          git clean -fd >> "$LOG_FILE" 2>&1
        fi
      else
        echo "    ⚠ No code changes made by AI"
      fi

      echo ""
    done <<< "$FILES"

    if should_stop; then break; fi
  done

  PACKAGES_COMPLETED=$((PACKAGES_COMPLETED + 1))
  PKG_ELAPSED=$(( $(date +%s) - PKG_START ))

  # Update progress file
  cat >> "$PROGRESS_FILE" << PKG_EOF

### internal/$pkg
- Warnings: ${pkg_warning_count}
- Time: ${PKG_ELAPSED}s
- Commits: $(git log --oneline "$BEFORE_SHA..HEAD" 2>/dev/null | wc -l | tr -d ' ')
PKG_EOF

  # Update status
  sed -i.bak "s/Packages completed: [0-9]*/Packages completed: $PACKAGES_COMPLETED/" "$PROGRESS_FILE" && rm -f "$PROGRESS_FILE.bak"
  sed -i.bak "s/Commits: [0-9]*/Commits: $TOTAL_COMMITS/" "$PROGRESS_FILE" && rm -f "$PROGRESS_FILE.bak"
  sed -i.bak "s/Warnings fixed: [0-9]*/Warnings fixed: $TOTAL_WARNINGS_FIXED/" "$PROGRESS_FILE" && rm -f "$PROGRESS_FILE.bak"

  echo ""
done

# ── Summary ────────────────────────────────────────────────────────────

ELAPSED=$(( $(date +%s) - STARTED_AT ))
HOURS=$(( ELAPSED / 3600 ))
MINS=$(( (ELAPSED % 3600) / 60 ))

echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║  Lint Fixer Complete                                 ║"
echo "╠══════════════════════════════════════════════════════╣"
echo "║  Packages:  $PACKAGES_COMPLETED / ${#PACKAGES_TO_FIX[@]}"
echo "║  Commits:   $TOTAL_COMMITS"
echo "║  Warnings:  $TOTAL_WARNINGS_FIXED fixed"
echo "║  Duration:  ${HOURS}h ${MINS}m"
echo "╚══════════════════════════════════════════════════════╝"
echo ""

if [ "$USE_WORKTREE" = true ]; then
  echo "Review changes:"
  echo "  cd $WORK_DIR"
  echo "  git log --oneline"
  echo ""
  echo "Merge to main branch:"
  echo "  git checkout $(git rev-parse --abbrev-ref HEAD@{-1})"
  echo "  git merge $BRANCH"
fi

echo ""
echo "Progress saved to: $PROGRESS_FILE"
echo "Full log: $LOG_FILE"
