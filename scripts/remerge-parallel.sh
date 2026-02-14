#!/usr/bin/env bash
set -euo pipefail

# ── Re-merge parallel branches via chronological cherry-pick ────────
#
# Instead of merging 44 branches (which causes massive conflicts on
# hotspot files), this script cherry-picks every individual commit
# in the order it was originally authored. Since each commit is a
# small, focused refactoring change, conflicts are minimal (~2.5%)
# and easy to resolve.
#
# Usage:
#   ./scripts/remerge-parallel.sh              # dry-run (default)
#   ./scripts/remerge-parallel.sh --execute    # actually do it
#   ./scripts/remerge-parallel.sh --continue   # resume after fixing a conflict
#   ./scripts/remerge-parallel.sh --status     # show progress
#
# Prerequisites:
#   - On the pulse/v6 branch
#   - All parallel branches exist locally

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"

PRE_MERGE_BASE="27bf6ab7"  # v6 before any parallel merges
STATE_DIR="$REPO_DIR/scripts/.remerge"
TIMELINE_FILE="$STATE_DIR/timeline.txt"
PROGRESS_FILE="$STATE_DIR/progress"
LOG_FILE="$STATE_DIR/remerge.log"
BUILD_ERRORS="$STATE_DIR/build-errors.txt"

# ── Helpers ──────────────────────────────────────────────────────────

log() {
  local msg="[$(date '+%H:%M:%S')] $*"
  echo "$msg"
  echo "$msg" >> "$LOG_FILE"
}

check_build() {
  log "  Checking build..."
  if go build ./... 2>"$BUILD_ERRORS"; then
    return 0
  else
    return 1
  fi
}

get_progress() {
  if [ -f "$PROGRESS_FILE" ]; then
    cat "$PROGRESS_FILE"
  else
    echo "0"
  fi
}

save_progress() {
  echo "$1" > "$PROGRESS_FILE"
}

# ── Build the timeline ───────────────────────────────────────────────

build_timeline() {
  log "Building chronological timeline of all commits..."
  mkdir -p "$STATE_DIR"

  # Collect every commit from every parallel branch, sorted by author date
  > "$TIMELINE_FILE"
  for b in $(git branch --list '*parallel*' | sed 's/^[+* ]*//'); do
    git log --format="%ai|%H|$b|%s" "$PRE_MERGE_BASE".."$b" 2>/dev/null
  done | sort >> "$TIMELINE_FILE"

  local total
  total=$(wc -l < "$TIMELINE_FILE" | tr -d ' ')
  log "Timeline built: $total commits from $(git branch --list '*parallel*' | wc -l | tr -d ' ') branches"
}

# ── Mode parsing ─────────────────────────────────────────────────────

MODE="dry-run"
BUILD_GATE="final"  # "every" = build after each commit, "batch" = every 25, "final" = end only
while [[ $# -gt 0 ]]; do
  case $1 in
    --execute)     MODE="execute"; shift ;;
    --continue)    MODE="continue"; shift ;;
    --dry-run)     MODE="dry-run"; shift ;;
    --status)      MODE="status"; shift ;;
    --build-every) BUILD_GATE="every"; shift ;;
    --build-batch) BUILD_GATE="batch"; shift ;;
    --build-final) BUILD_GATE="final"; shift ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ── Status ───────────────────────────────────────────────────────────

if [ "$MODE" = "status" ]; then
  if [ ! -f "$TIMELINE_FILE" ]; then
    echo "No re-merge in progress."
    exit 0
  fi
  total=$(wc -l < "$TIMELINE_FILE" | tr -d ' ')
  done=$(get_progress)
  remaining=$((total - done))
  echo "Re-merge progress: $done / $total commits ($remaining remaining)"
  if [ "$done" -gt 0 ] && [ "$done" -lt "$total" ]; then
    next_line=$(sed -n "$((done + 1))p" "$TIMELINE_FILE")
    echo "Next: $(echo "$next_line" | cut -d'|' -f4)"
  fi
  exit 0
fi

# ── Dry run ──────────────────────────────────────────────────────────

if [ "$MODE" = "dry-run" ]; then
  # Build a temporary timeline for display
  tmpfile=$(mktemp)
  for b in $(git branch --list '*parallel*' | sed 's/^[+* ]*//'); do
    git log --format="%ai|%H|$b|%s" "$PRE_MERGE_BASE".."$b" 2>/dev/null
  done | sort > "$tmpfile"

  total=$(wc -l < "$tmpfile" | tr -d ' ')
  branches=$(git branch --list '*parallel*' | wc -l | tr -d ' ')

  # Count potential conflicts (adjacent commits from different branches touching same files)
  prev_branch=""
  prev_hash=""
  conflict_count=0
  while IFS='|' read -r ts hash branch msg; do
    if [ -n "$prev_hash" ] && [ "$branch" != "$prev_branch" ]; then
      overlap=$(comm -12 \
        <(git diff --name-only "${hash}^" "$hash" 2>/dev/null | sort) \
        <(git diff --name-only "${prev_hash}^" "$prev_hash" 2>/dev/null | sort) \
        | wc -l | tr -d ' ')
      [ "$overlap" -gt 0 ] && conflict_count=$((conflict_count + 1))
    fi
    prev_branch="$branch"
    prev_hash="$hash"
  done < "$tmpfile"

  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║  Chronological Cherry-Pick Re-merge (DRY RUN)            ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║  Pre-merge base:    $PRE_MERGE_BASE"
  echo "║  Branches:          $branches"
  echo "║  Total commits:     $total"
  echo "║  Potential conflicts: ~$conflict_count ($(echo "scale=1; $conflict_count * 100 / $total" | bc)%)"
  echo "║  Strategy:          cherry-pick in author-date order"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""
  echo "First 10 commits:"
  head -10 "$tmpfile" | while IFS='|' read -r ts hash branch msg; do
    echo "  $(echo "$ts" | cut -d' ' -f1,2) $(git log --oneline -1 "$hash") [$(echo "$branch" | sed 's|refactor/parallel-||')]"
  done
  echo "  ..."
  echo ""
  echo "Last 5 commits:"
  tail -5 "$tmpfile" | while IFS='|' read -r ts hash branch msg; do
    echo "  $(echo "$ts" | cut -d' ' -f1,2) $(git log --oneline -1 "$hash") [$(echo "$branch" | sed 's|refactor/parallel-||')]"
  done
  echo ""
  echo "Build gate options (default: --build-final):"
  echo "  --build-every   Check build after each commit (slowest, safest)"
  echo "  --build-batch   Check build every 25 commits"
  echo "  --build-final   Check build only at the end (fastest)"
  echo ""
  echo "To execute:  $0 --execute [--build-batch]"
  echo "To resume:   $0 --continue"

  rm -f "$tmpfile"
  exit 0
fi

# ── Execute mode ─────────────────────────────────────────────────────

if [ "$MODE" = "execute" ]; then
  current_branch=$(git branch --show-current)
  if [ "$current_branch" != "pulse/v6" ]; then
    echo "Error: must be on pulse/v6 branch (currently on: $current_branch)"
    exit 1
  fi

  if ! git diff --quiet HEAD 2>/dev/null; then
    echo ""
    echo "WARNING: You have uncommitted changes in the working tree."
    read -rp "Continue? [y/N] " confirm
    [ "$confirm" = "y" ] || [ "$confirm" = "Y" ] || exit 1
  fi

  # Build the timeline
  build_timeline
  total=$(wc -l < "$TIMELINE_FILE" | tr -d ' ')

  echo ""
  echo "This will reset pulse/v6 to $PRE_MERGE_BASE and cherry-pick $total commits."
  echo "Current HEAD: $(git log --oneline -1 HEAD)"
  echo ""
  read -rp "Type 'yes' to proceed: " confirm
  [ "$confirm" = "yes" ] || { echo "Aborted."; exit 1; }

  # Save backup
  git branch -f pulse/v6-pre-remerge-backup HEAD
  log "Backup saved: pulse/v6-pre-remerge-backup -> $(git rev-parse --short HEAD)"

  # Reset
  git reset --hard "$PRE_MERGE_BASE"
  log "Reset pulse/v6 to $PRE_MERGE_BASE"

  save_progress 0
fi

# ── Cherry-pick loop (used by both --execute and --continue) ─────────

if [ ! -f "$TIMELINE_FILE" ]; then
  echo "Error: no timeline found. Run with --execute first."
  exit 1
fi

total=$(wc -l < "$TIMELINE_FILE" | tr -d ' ')
start_idx=$(get_progress)

if [ "$MODE" = "continue" ]; then
  # Check if there's a cherry-pick in progress that needs finishing
  if [ -d "$REPO_DIR/.git/sequencer" ] || [ -f "$REPO_DIR/.git/CHERRY_PICK_HEAD" ]; then
    echo "There's an unfinished cherry-pick. Please resolve it first:"
    echo "  git add <resolved files>"
    echo "  git cherry-pick --continue"
    echo "Then run: $0 --continue"
    exit 1
  fi

  # The progress file points to the commit that failed.
  # If we're continuing, the user has resolved it, so advance.
  start_idx=$((start_idx + 1))
  save_progress "$start_idx"
  log "Resuming from commit $((start_idx + 1))/$total"
fi

succeeded=0
failed=0
skipped=0

for (( i=start_idx; i<total; i++ )); do
  line=$(sed -n "$((i + 1))p" "$TIMELINE_FILE")
  IFS='|' read -r ts hash branch msg <<< "$line"

  short=$(git log --oneline -1 "$hash" 2>/dev/null || echo "$hash")
  pass_name=$(echo "$branch" | sed 's|refactor/parallel-||')

  printf "\r[%d/%d] %s" $((i + 1)) "$total" "$short"

  # Skip if this commit is already an ancestor (e.g., from a resumed run)
  if git merge-base --is-ancestor "$hash" HEAD 2>/dev/null; then
    skipped=$((skipped + 1))
    continue
  fi

  # Cherry-pick
  if git cherry-pick --no-commit "$hash" 2>>"$LOG_FILE"; then
    git commit --no-edit -m "$msg" --author="$(git log --format='%an <%ae>' -1 "$hash")" 2>>"$LOG_FILE" || true
    succeeded=$((succeeded + 1))
  else
    # Conflict — check if it's trivially resolvable (empty diff = already applied)
    if git diff --cached --quiet 2>/dev/null && git diff --quiet 2>/dev/null; then
      # No actual changes — skip this commit (content already present)
      git cherry-pick --abort 2>/dev/null || git reset --hard HEAD 2>/dev/null
      skipped=$((skipped + 1))
      save_progress "$i"
      continue
    fi

    echo ""
    log "CONFLICT at commit $((i + 1))/$total: $short [$pass_name]"
    echo ""
    echo "  Conflicted files:"
    git diff --name-only --diff-filter=U 2>/dev/null | sed 's/^/    /'
    echo ""
    echo "  To resolve:"
    echo "    1. Fix the conflicts in the files above"
    echo "    2. git add <resolved files>"
    echo "    3. git cherry-pick --continue"
    echo "    4. $0 --continue"
    echo ""
    echo "  To skip this commit:"
    echo "    git cherry-pick --abort"
    echo "    $0 --continue"
    echo ""
    save_progress "$i"
    failed=$((failed + 1))

    echo "Progress: $succeeded applied, $skipped skipped, $failed conflicts, $((total - i - 1)) remaining"
    exit 1
  fi

  save_progress "$i"

  # Build gates
  if [ "$BUILD_GATE" = "every" ]; then
    if ! check_build; then
      echo ""
      log "BUILD FAILED after commit $((i + 1))/$total: $short"
      cat "$BUILD_ERRORS"
      echo ""
      echo "Fix the build, commit the fix, then: $0 --continue"
      exit 1
    fi
  elif [ "$BUILD_GATE" = "batch" ] && [ $(( (i + 1) % 25 )) -eq 0 ]; then
    echo ""
    log "Build check at commit $((i + 1))/$total..."
    if ! check_build; then
      log "BUILD FAILED at batch checkpoint $((i + 1))/$total"
      cat "$BUILD_ERRORS"
      echo ""
      echo "Fix the build, commit the fix, then: $0 --continue"
      exit 1
    fi
    log "Build: PASS ($((i + 1))/$total)"
  fi
done

echo ""
echo ""

# Final build check
log "Running final build check..."
if check_build; then
  log "Final build: PASS"
else
  log "Final build: FAIL"
  echo ""
  echo "All commits applied but the build has errors:"
  cat "$BUILD_ERRORS"
  echo ""
  echo "Fix the build issues and commit the fixes."
  exit 1
fi

# ── Summary ──────────────────────────────────────────────────────────

log ""
log "╔══════════════════════════════════════════════════════════╗"
log "║  Re-merge Complete!                                      ║"
log "╠══════════════════════════════════════════════════════════╣"
log "║  Commits applied:  $succeeded"
log "║  Commits skipped:  $skipped"
log "║  Final build:      PASS"
log "║  HEAD:             $(git log --oneline -1 HEAD)"
log "╚══════════════════════════════════════════════════════════╝"
log ""
log "Backup of old v6: pulse/v6-pre-remerge-backup"
log "To delete: git branch -D pulse/v6-pre-remerge-backup"

# Clean up state
rm -rf "$STATE_DIR"
