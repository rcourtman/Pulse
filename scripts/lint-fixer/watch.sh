#!/usr/bin/env bash
# Real-time dashboard for lint-fixer progress

LOG_FILE="/tmp/lint-fixer-runner.log"
PROGRESS_FILE="scripts/lint-fixer/PROGRESS.md"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# Function to get current branch
get_lint_branch() {
  git rev-parse --abbrev-ref HEAD
}

# Function to calculate stats
calculate_stats() {
  local branch="$1"

  # Get base commit from log file
  local base_commit=$(grep "Base commit:" "$LOG_FILE" 2>/dev/null | tail -1 | awk '{print $3}')
  if [ -z "$base_commit" ]; then
    base_commit="HEAD~10"  # Fallback
  fi

  if [ -z "$branch" ]; then
    echo "0|0|0|0"
    return
  fi

  # Get commit count since base
  local commits=$(git log --oneline "${base_commit}..${branch}" 2>/dev/null | wc -l | tr -d ' ')

  # Get line changes (additions and deletions)
  local stats=$(git diff --shortstat "${base_commit}..${branch}" 2>/dev/null)
  local insertions=0
  local deletions=0

  if [ -n "$stats" ]; then
    insertions=$(echo "$stats" | grep -o '[0-9]* insertion' | grep -o '[0-9]*' || echo "0")
    deletions=$(echo "$stats" | grep -o '[0-9]* deletion' | grep -o '[0-9]*' || echo "0")
  fi

  # Get files changed
  local files=$(git diff --name-only "${base_commit}..${branch}" 2>/dev/null | grep -v 'PROGRESS.md\|\.gitignore' | wc -l | tr -d ' ')

  echo "${commits}|${insertions}|${deletions}|${files}"
}

# Function to get current activity from log
get_current_activity() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "Not running"
    return
  fi

  # Get last few lines and find current package/status
  local last_lines=$(tail -50 "$LOG_FILE" 2>/dev/null)

  # Check if it's currently calling AI
  if echo "$last_lines" | grep -q "Calling AI model..."; then
    local pkg=$(echo "$last_lines" | grep "Package: " | tail -1 | sed 's/.*Package: \(.*\) (.*/\1/')
    local linter=$(echo "$last_lines" | grep "→ Running" | tail -1 | sed 's/.*Running \([^ ]*\).*/\1/')
    echo "🤖 AI fixing ${linter} in ${pkg}"
    return
  fi

  # Check if testing
  if echo "$last_lines" | grep -q "Testing changes..."; then
    local pkg=$(echo "$last_lines" | grep "Package: " | tail -1 | sed 's/.*Package: \(.*\) (.*/\1/')
    echo "🧪 Testing fixes in ${pkg}"
    return
  fi

  # Check if scanning
  if echo "$last_lines" | grep -q "Scanning for lint warnings..."; then
    echo "🔍 Scanning for lint warnings..."
    return
  fi

  # Check if building
  if echo "$last_lines" | grep -q "Building"; then
    echo "🔨 Building code..."
    return
  fi

  # Check if completed
  if echo "$last_lines" | tail -5 | grep -q "Lint Fixer Complete"; then
    echo "✅ COMPLETED"
    return
  fi

  # Default
  echo "⏳ Processing..."
}

# Function to parse package results from log
get_package_results() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "0|0|0"
    return
  fi

  local passed=$(grep -c "✓ Tests passed" "$LOG_FILE" 2>/dev/null || echo "0")
  local failed=$(grep -c "✗ Tests failed" "$LOG_FILE" 2>/dev/null || echo "0")
  local ai_completed=$(grep -c "✓ AI completed successfully" "$LOG_FILE" 2>/dev/null || echo "0")

  echo "${passed}|${failed}|${ai_completed}"
}

# Function to get package queue status
get_queue_status() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "0|0"
    return
  fi

  # Find the "Found N packages" line
  local total=$(grep "Found .* packages with warnings:" "$LOG_FILE" 2>/dev/null | head -1 | grep -o '[0-9]*' | head -1 || echo "0")

  # Count completed packages (those with ━━━ header that have been processed)
  local completed=$(grep -c "Package: internal/" "$LOG_FILE" 2>/dev/null || echo "0")

  # Adjust completed count (each package gets 2 passes: errcheck + dupl)
  # So completed packages = lines / 2, but be conservative
  completed=$((completed > 0 ? (completed + 1) / 2 : 0))

  echo "${completed}|${total}"
}

# Function to get current package name
get_current_package() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "-"
    return
  fi

  grep "Package: " "$LOG_FILE" 2>/dev/null | tail -1 | sed 's/.*Package: \(.*\) (.*/\1/' || echo "-"
}

# Function to get elapsed time
get_elapsed_time() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "0s"
    return
  fi

  # Get start time from log file
  local start_date=$(grep "Started:" "$LOG_FILE" 2>/dev/null | tail -1 | sed 's/.*Started: *//')
  if [ -z "$start_date" ]; then
    echo "0s"
    return
  fi

  local start_time=$(date -j -f "%a %d %b %Y %H:%M:%S %Z" "$start_date" +%s 2>/dev/null)
  if [ -z "$start_time" ]; then
    echo "0s"
    return
  fi

  local now=$(date +%s)
  local elapsed=$((now - start_time))

  if [ $elapsed -ge 3600 ]; then
    echo "$((elapsed / 3600))h $((elapsed % 3600 / 60))m"
  elif [ $elapsed -ge 60 ]; then
    echo "$((elapsed / 60))m $((elapsed % 60))s"
  else
    echo "${elapsed}s"
  fi
}

# Function to check if process is running
is_running() {
  pgrep -f 'lint-fixer/run-v2.sh' >/dev/null 2>&1 || pgrep -f 'opencode run' >/dev/null 2>&1
}

# Main display loop
while true; do
  clear

  # Get current lint-fixes branch
  BRANCH=$(get_lint_branch)

  # Calculate stats
  IFS='|' read -r COMMITS INSERTIONS DELETIONS FILES <<< "$(calculate_stats "$BRANCH")"

  # Get results
  IFS='|' read -r PASSED FAILED AI_CALLS <<< "$(get_package_results)"

  # Get queue status
  IFS='|' read -r COMPLETED TOTAL <<< "$(get_queue_status)"

  # Get current activity
  ACTIVITY=$(get_current_activity)

  # Get current package
  CURRENT_PKG=$(get_current_package)

  # Get elapsed time
  ELAPSED=$(get_elapsed_time "$BRANCH")

  # Check if running
  if is_running; then
    STATUS="${GREEN}● RUNNING${RESET}"
  else
    STATUS="${RED}○ STOPPED${RESET}"
  fi

  # Display header
  echo -e "${BOLD}╔══════════════════════════════════════════════════════╗${RESET}"
  echo -e "${BOLD}║     Pulse Lint Fixer — Live Dashboard               ║${RESET}"
  echo -e "${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
  echo -e "${BOLD}║${RESET}  Status:      $STATUS"
  echo -e "${BOLD}║${RESET}  Model:       ${CYAN}MiniMax M2.5${RESET}"
  echo -e "${BOLD}║${RESET}  Branch:      ${YELLOW}${BRANCH:-none}${RESET}"
  echo -e "${BOLD}║${RESET}  Elapsed:     ${ELAPSED}"
  echo -e "${BOLD}╚══════════════════════════════════════════════════════╝${RESET}"
  echo ""

  # Current activity
  echo -e "${BOLD}Current Activity:${RESET}"
  echo -e "  $ACTIVITY"
  echo ""

  # Progress
  echo -e "${BOLD}Package Progress:${RESET}"
  if [ "${TOTAL:-0}" -gt 0 ]; then
    pct=$((COMPLETED * 100 / TOTAL))
    bar_width=40
    filled=$((pct * bar_width / 100))
    empty=$((bar_width - filled))

    echo -n "  ["
    for ((i=0; i<filled; i++)); do echo -n "█"; done
    for ((i=0; i<empty; i++)); do echo -n "░"; done
    echo -e "] ${pct}%"
    echo -e "  ${GREEN}${COMPLETED}${RESET} / ${TOTAL} packages completed"
    echo -e "  Currently: ${CYAN}${CURRENT_PKG}${RESET}"
  else
    echo -e "  ${YELLOW}Initializing...${RESET}"
  fi
  echo ""

  # Statistics
  echo -e "${BOLD}Statistics:${RESET}"
  echo -e "  ${GREEN}✓${RESET} Commits:      ${COMMITS}"
  echo -e "  ${GREEN}+${RESET} Insertions:   ${INSERTIONS}"
  echo -e "  ${RED}−${RESET} Deletions:    ${DELETIONS}"
  echo -e "  ${BLUE}📄${RESET} Files changed: ${FILES}"
  echo ""

  # AI Performance
  echo -e "${BOLD}AI Performance:${RESET}"
  success_rate=0
  PASSED="${PASSED:-0}"
  FAILED="${FAILED:-0}"
  AI_CALLS="${AI_CALLS:-0}"
  if [ $((PASSED + FAILED)) -gt 0 ]; then
    success_rate=$((PASSED * 100 / (PASSED + FAILED)))
  fi
  echo -e "  ${CYAN}🤖${RESET} AI calls:     ${AI_CALLS}"
  echo -e "  ${GREEN}✓${RESET} Tests passed: ${PASSED}"
  echo -e "  ${RED}✗${RESET} Tests failed: ${FAILED}"
  if [ "${AI_CALLS}" -gt 0 ]; then
    echo -e "  ${YELLOW}📊${RESET} Success rate: ${success_rate}%"
  fi
  echo ""

  # Recent activity (last 5 lines)
  echo -e "${BOLD}Recent Log:${RESET}"
  if [ -f "$LOG_FILE" ]; then
    tail -5 "$LOG_FILE" | sed 's/^/  /' | head -5
  else
    echo "  (no log file)"
  fi
  echo ""

  # Footer
  echo -e "${BOLD}Controls:${RESET}"
  echo -e "  ${CYAN}Ctrl+C${RESET}     Exit dashboard"
  echo -e "  ${CYAN}touch scripts/lint-fixer/.stop${RESET}  Stop fixer gracefully"
  echo -e "  ${CYAN}tail -f /tmp/lint-fixer.log${RESET}    Full log"
  echo ""
  echo -e "${YELLOW}Refreshing every 2 seconds...${RESET}"

  sleep 2
done
