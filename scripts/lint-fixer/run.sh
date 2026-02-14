#!/usr/bin/env bash
set -euo pipefail

# ── Overnight Lint Fixer — errcheck + dupl ─────────────────────────────
#
# Fixes golangci-lint errcheck and dupl warnings using MiniMax M2.5 via OpenRouter.
# Optimized for MiniMax M2.5's capabilities:
# - Interleaved thinking (reasoning between tool calls)
# - Purpose-driven prompts (explain "why")
# - Optimal parameters (temp=1.0, top_p=0.95, top_k=40)
#
# Usage:
#   export OPENROUTER_API_KEY="sk-or-v1-..."
#   ./scripts/lint-fixer/run.sh --model minimax/minimax-m2.5
#
# Options:
#   --model MODEL       OpenRouter model (default: minimax/minimax-m2.5)
#   --lint LINTERS      Comma-separated linters to fix (default: errcheck,dupl)
#   --packages PKGS     Comma-separated packages to target (default: all with warnings)
#   --max-iters N       Max iterations (default: 100)
#   --branch NAME       Branch name (default: lint-fixes/TIMESTAMP)
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
MODEL="openrouter/minimax/minimax-m2.5"
LINTERS="errcheck,dupl"
PACKAGES=""
MAX_ITERS=100
BRANCH=""
USE_WORKTREE=false  # Default to no worktree (simpler, avoids build issues)
ENGINE="aider"

while [[ $# -gt 0 ]]; do
  case $1 in
    --model)       MODEL="$2"; shift 2 ;;
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

if ! command -v aider >/dev/null 2>&1 && [ "$ENGINE" = "aider" ]; then
  echo "Error: aider not found"
  echo "  Install with: pip install aider-chat"
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

STOP_FILE="$SCRIPT_DIR/.stop"
PROGRESS_FILE="$SCRIPT_DIR/PROGRESS.md"
LOG_FILE="$SCRIPT_DIR/run.log"

rm -f "$STOP_FILE"
mkdir -p "$(dirname "$LOG_FILE")"

should_stop() {
  [ -f "$STOP_FILE" ]
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

  git checkout -b "$BRANCH" 2>/dev/null || git checkout "$BRANCH"
  echo "Working directly in repo (no worktree isolation)"
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

# Build package list
declare -A PKG_WARNINGS
for linter in "${LINT_ARRAY[@]}"; do
  while IFS= read -r line; do
    if [[ "$line" =~ ^internal/([^/]+)/ ]]; then
      pkg="${BASH_REMATCH[1]}"
      PKG_WARNINGS["$pkg"]=$((${PKG_WARNINGS[$pkg]:-0} + 1))
    fi
  done < <($GOLANGCI_LINT run --enable "$linter" --disable-all ./internal/... 2>&1 | grep "$linter" || true)
done

# Convert to sorted array (by warning count descending)
PACKAGES_TO_FIX=()
while IFS= read -r pkg; do
  PACKAGES_TO_FIX+=("$pkg")
done < <(for pkg in "${!PKG_WARNINGS[@]}"; do
  echo "${PKG_WARNINGS[$pkg]} $pkg"
done | sort -rn | cut -d' ' -f2)

if [ ${#PACKAGES_TO_FIX[@]} -eq 0 ]; then
  echo "No packages with warnings found!"
  exit 0
fi

echo "Found ${#PACKAGES_TO_FIX[@]} packages with warnings:"
for pkg in "${PACKAGES_TO_FIX[@]}"; do
  echo "  internal/$pkg: ${PKG_WARNINGS[$pkg]} warnings"
done
echo ""

# ── Main loop: fix packages one by one ────────────────────────────────

TOTAL_COMMITS=0
TOTAL_WARNINGS_FIXED=0
PACKAGES_COMPLETED=0

for pkg in "${PACKAGES_TO_FIX[@]}"; do
  if should_stop; then
    echo "Stop signal received — shutting down gracefully"
    break
  fi

  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Package: internal/$pkg (${PKG_WARNINGS[$pkg]} warnings)"
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

    # Extract affected files
    FILES=$(echo "$WARNINGS" | grep -o '^[^:]*\.go' | sort -u || true)
    if [ -z "$FILES" ]; then
      echo "  ⚠ Could not extract file list from warnings"
      continue
    fi

    FILE_COUNT=$(echo "$FILES" | wc -l | tr -d ' ')
    echo "  Affected files: $FILE_COUNT"

    # Build MiniMax M2.5 optimized prompt (purpose-driven, explain "why")
    case "$linter" in
      errcheck)
        PROMPT="## Task: Fix errcheck warnings in internal/$pkg

**Why this matters:** Unchecked errors can cause silent failures, data corruption, or security issues in production. We need proper error handling to make Pulse reliable for users monitoring critical infrastructure.

**Context:** This is production code for Pulse, an infrastructure monitoring platform. Users depend on accurate metrics and reliable operations. Every unchecked error is a potential production incident.

**Warnings to fix:**
\`\`\`
$WARNINGS
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
- Keep changes minimal — fix only the error handling
- Preserve all existing behavior
- Ensure all tests still pass

**Expected output:** Clean, production-ready code with all errors properly handled."
        ;;

      dupl)
        PROMPT="## Task: Eliminate code duplication in internal/$pkg

**Why this matters:** Duplicated code creates maintenance burden. When we fix a bug or add a feature, we have to find and update multiple copies, which leads to inconsistencies and missed updates. DRY code is easier to maintain and less error-prone.

**Context:** This is production code for Pulse. We're refactoring to improve long-term maintainability without changing any behavior.

**Duplication warnings:**
\`\`\`
$WARNINGS
\`\`\`

**How to fix them:**

1. **Identify** the duplicated logic blocks
2. **Extract** into well-named helper functions:
   - Use descriptive names that explain what they do (e.g., \`buildProxmoxMetricPayload\`, \`formatTimestampISO8601\`)
   - Keep helpers private (lowercase) unless they need to be exported
   - Place helpers near their call sites (same file if only used there, separate file if shared across package)
   - Add brief comment explaining purpose if not obvious from name

3. **Replace** duplicated code with calls to the helper

**Example:**
\`\`\`go
// BEFORE (duplicated):
payload := map[string]interface{}{
    \"vmid\": vm.ID,
    \"node\": vm.Node,
    \"cpu\": vm.CPU,
}
// ... later ...
payload := map[string]interface{}{
    \"vmid\": vm.ID,
    \"node\": vm.Node,
    \"cpu\": vm.CPU,
}

// AFTER (DRY):
func buildVMPayload(vm *VM) map[string]interface{} {
    return map[string]interface{}{
        \"vmid\": vm.ID,
        \"node\": vm.Node,
        \"cpu\": vm.CPU,
    }
}
payload := buildVMPayload(vm)
// ... later ...
payload := buildVMPayload(vm)
\`\`\`

**Constraints:**
- Do NOT over-abstract — only extract what's actually duplicated
- Preserve exact behavior (no logic changes)
- Ensure all tests still pass
- Keep changes minimal

**Expected output:** DRY code with shared logic extracted into clear, well-named helper functions."
        ;;

      *)
        PROMPT="## Task: Fix $linter warnings in internal/$pkg

**Warnings:**
\`\`\`
$WARNINGS
\`\`\`

**Instructions:**
Keep changes minimal and focused. Do not change function signatures or add unnecessary imports. Fix only what the linter flagged."
        ;;
    esac

    # Run aider with MiniMax M2.5 optimized settings
    # The .aider.model.settings.yml file configures:
    # - temperature: 1.0, top_p: 0.95, top_k: 40
    # - reasoning_split: true (enables interleaved thinking)
    echo "  Calling AI model..."

    if [ "$ENGINE" = "aider" ]; then
      # Run aider in non-interactive mode
      if aider \
        --model "$MODEL" \
        --message "$PROMPT" \
        --yes-always \
        --no-auto-commits \
        $FILES >> "$LOG_FILE" 2>&1; then
        echo "  ✓ AI completed successfully"
      else
        echo "  ✗ AI call failed (exit $?)"
        continue
      fi
    else
      echo "  ✗ Unknown engine: $ENGINE"
      continue
    fi

    # Check if changes were ACTUALLY made (not just aider metadata)
    # This fixes the bug where we committed with no code changes
    if ! git diff --quiet HEAD -- './internal'; then
      echo "  Testing changes..."

      # Run package tests
      if go test "./internal/$pkg/..." >> "$LOG_FILE" 2>&1; then
        echo "  ✓ Tests passed"

        # Commit ONLY if there are actual code changes
        COMMIT_MSG="fix($linter): resolve warnings in internal/$pkg

Fixed $WARNING_COUNT $linter warnings.

Model: $MODEL
Context: Pulse infrastructure monitoring platform"

        # Stage only internal/ files (exclude PROGRESS.md, .aider files, etc.)
        git add internal/

        if ! git diff --cached --quiet; then
          git commit -m "$COMMIT_MSG" >> "$LOG_FILE" 2>&1

          TOTAL_COMMITS=$((TOTAL_COMMITS + 1))
          TOTAL_WARNINGS_FIXED=$((TOTAL_WARNINGS_FIXED + WARNING_COUNT))

          echo "  ✓ Committed ($TOTAL_COMMITS total)"
        else
          echo "  ⚠ No changes to commit (AI may have only added comments)"
          git reset HEAD internal/ >/dev/null 2>&1
        fi
      else
        echo "  ✗ Tests failed — reverting changes"
        git checkout HEAD -- internal/ >> "$LOG_FILE" 2>&1
        git clean -fd >> "$LOG_FILE" 2>&1
      fi
    else
      echo "  ⚠ No code changes made by AI"
    fi

    echo ""
  done

  PACKAGES_COMPLETED=$((PACKAGES_COMPLETED + 1))
  PKG_ELAPSED=$(( $(date +%s) - PKG_START ))

  # Update progress file
  cat >> "$PROGRESS_FILE" << PKG_EOF

### internal/$pkg
- Warnings: ${PKG_WARNINGS[$pkg]}
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
