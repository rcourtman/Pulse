#!/usr/bin/env bash
#
# Pulse Lint Fixer v3 - Autonomous Package-Level Fixing (Resumable + Parallel)
#
# P0 fixes in this version:
# - Uses OPENCODE variable everywhere (no hardcoded binary path)
# - Scans golangci-lint once and caches JSON/TSV for reuse
# - Persists .state.json with atomic writes for resume support
# - Runs independent packages in parallel with wait -n throttling
# - Uses isolated worktrees and pre-flight dirty checks to avoid reverting user WIP
# - Checks disk space before running
# - Adds structured error context (package/linter/warnings/timestamp/exit reason)

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd -P)"
SCRIPT_DIR="$REPO_DIR/scripts/lint-fixer"

MODEL="${MODEL:-opencode/minimax-m2.5-free}"
LINTERS="${LINTERS:-errcheck,dupl}"
PACKAGES="${PACKAGES:-}"    # Optional comma-separated package list
BRANCH=""
USE_WORKTREE=false

AIDER_TIMEOUT_SECS="${AIDER_TIMEOUT_SECS:-900}"
TEST_TIMEOUT_SECS="${TEST_TIMEOUT_SECS:-300}"     # P1: test timeout
MAX_PARALLEL="${MAX_PARALLEL:-4}"                 # P0: parallel package workers
MIN_FREE_MB="${MIN_FREE_MB:-500}"                 # P0: disk preflight
AI_FAILURE_CIRCUIT="${AI_FAILURE_CIRCUIT:-3}"     # P1: stop after N consecutive AI failures
AI_MAX_ATTEMPTS="${AI_MAX_ATTEMPTS:-3}"           # P2: retry transient AI failures
AI_RETRY_BASE_DELAY_SECS="${AI_RETRY_BASE_DELAY_SECS:-2}"
AI_RETRY_MAX_DELAY_SECS="${AI_RETRY_MAX_DELAY_SECS:-20}"
WORKER_LAUNCH_STAGGER_SECS="${WORKER_LAUNCH_STAGGER_SECS:-1}"  # P2: soften API burst
GIT_LOCK_RETRIES="${GIT_LOCK_RETRIES:-8}"         # P1: retry on index.lock

LOG_FILE="/tmp/lint-fixer-runner.log"
PROGRESS_FILE="$SCRIPT_DIR/PROGRESS.md"
STATE_FILE="$SCRIPT_DIR/.state.json"
SCAN_CACHE_FILE="$SCRIPT_DIR/.scan-cache-v3.json"
WARNINGS_TSV_FILE="$SCRIPT_DIR/.scan-cache-v3.tsv"

RUN_ID="$(date +%Y%m%d-%H%M%S)-$$"
WORKTREE_ROOT="$SCRIPT_DIR/.worktrees-v3-$RUN_ID"
RESULTS_DIR="$SCRIPT_DIR/.results-v3-$RUN_ID"
AI_TRACE_DIR="$SCRIPT_DIR/.ai-traces-v3-$RUN_ID"

mkdir -p "$SCRIPT_DIR"
mkdir -p "$WORKTREE_ROOT" "$RESULTS_DIR" "$AI_TRACE_DIR"
touch "$LOG_FILE"

# Parse args (kept compatible with v2)
while [[ $# -gt 0 ]]; do
  case "$1" in
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
  BRANCH="$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD)"
fi

# -----------------------------------------------------------------------------
# Logging / helpers
# -----------------------------------------------------------------------------

timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

log() {
  printf '[%s] %s\n' "$(timestamp)" "$*" | tee -a "$LOG_FILE"
}

map_exit_reason() {
  local code="${1:-0}"
  case "$code" in
    0)   echo "success" ;;
    1)   echo "generic failure (inspect trace/output)" ;;
    2)   echo "misuse of shell builtins / bad invocation" ;;
    124) echo "timeout" ;;
    126) echo "command found but not executable" ;;
    127) echo "command not found" ;;
    130) echo "interrupted (SIGINT)" ;;
    137) echo "killed (SIGKILL/OOM)" ;;
    *)   echo "exit-$code" ;;
  esac
}

log_error() {
  local package="$1"
  local linter="$2"
  local warning_count="$3"
  local message="$4"
  local exit_code="${5:-}"

  local suffix=""
  if [ -n "$exit_code" ]; then
    suffix=" (exit=$exit_code, reason=$(map_exit_reason "$exit_code"))"
  fi

  log "ERROR package=$package linter=$linter warnings=$warning_count ts=$(timestamp) $message$suffix"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_error "global" "global" "0" "Required command not found: $cmd" 127
    exit 1
  fi
}

run_with_timeout() {
  local timeout_secs="$1"
  shift

  if command -v timeout >/dev/null 2>&1; then
    timeout "$timeout_secs" "$@"
    return $?
  fi

  # Fallback compatible with systems without GNU timeout.
  "$@" &
  local cmd_pid=$!
  local start_time
  start_time="$(date +%s)"

  while kill -0 "$cmd_pid" 2>/dev/null; do
    if [ $(( $(date +%s) - start_time )) -ge "$timeout_secs" ]; then
      kill "$cmd_pid" 2>/dev/null || true
      sleep 2
      kill -9 "$cmd_pid" 2>/dev/null || true
      wait "$cmd_pid" 2>/dev/null || true
      return 124
    fi
    sleep 1
  done

  wait "$cmd_pid"
}

AI_LAST_TRACE_FILE=""
AI_LAST_FAILURE_KIND=""
AI_LAST_ATTEMPT=0

classify_ai_failure() {
  local trace_file="$1"
  local exit_code="$2"

  if [ "$exit_code" -eq 124 ]; then
    printf 'timeout'
    return
  fi

  if [ ! -s "$trace_file" ]; then
    printf 'no_output'
    return
  fi

  if grep -Eiq 'rate[ -]?limit|too many requests|status[=: ]*429|quota' "$trace_file"; then
    printf 'rate_limit'
    return
  fi

  if grep -Eiq 'timed? out|timeout|econnreset|econnrefused|enotfound|network|socket hang up|temporarily unavailable|status[=: ]*5[0-9]{2}' "$trace_file"; then
    printf 'transient_network'
    return
  fi

  if grep -Eiq 'unauthorized|forbidden|invalid api key|auth(entication)? failed|permission denied' "$trace_file"; then
    printf 'auth_or_permission'
    return
  fi

  if grep -Eiq 'model .* not found|unknown model|no such model' "$trace_file"; then
    printf 'model_config'
    return
  fi

  printf 'generic'
}

ai_failure_is_retryable() {
  local kind="$1"
  local exit_code="$2"

  case "$kind" in
    timeout|rate_limit|transient_network|no_output)
      return 0
      ;;
    generic)
      [ "$exit_code" -eq 1 ] && return 0 || return 1
      ;;
    *)
      return 1
      ;;
  esac
}

log_ai_trace_tail() {
  local pkg="$1"
  local linter="$2"
  local warning_count="$3"
  local attempt="$4"
  local trace_file="$5"

  if [ ! -s "$trace_file" ]; then
    log "package=internal/$pkg linter=$linter warnings=$warning_count AI trace empty attempt=$attempt trace=$trace_file"
    return
  fi

  printf '[%s] AI trace tail package=internal/%s linter=%s warnings=%s attempt=%s trace=%s\n' \
    "$(timestamp)" "$pkg" "$linter" "$warning_count" "$attempt" "$trace_file" >> "$LOG_FILE"
  tail -n 20 "$trace_file" >> "$LOG_FILE"
}

run_ai_with_retries() {
  local pkg="$1"
  local linter="$2"
  local warning_count="$3"
  local prompt="$4"

  local safe_pkg
  local attempt=1
  local delay="$AI_RETRY_BASE_DELAY_SECS"
  local trace_file
  local ai_exit
  local kind

  safe_pkg="${pkg//\//-}"

  AI_LAST_TRACE_FILE=""
  AI_LAST_FAILURE_KIND=""
  AI_LAST_ATTEMPT=0

  while [ "$attempt" -le "$AI_MAX_ATTEMPTS" ]; do
    trace_file="$AI_TRACE_DIR/${safe_pkg}-${linter}-attempt${attempt}.log"
    : > "$trace_file"

    log "package=internal/$pkg linter=$linter warnings=$warning_count calling AI model attempt=$attempt/$AI_MAX_ATTEMPTS"

    if run_with_timeout "$AIDER_TIMEOUT_SECS" "$OPENCODE" run --model "$MODEL" "$prompt" > "$trace_file" 2>&1; then
      AI_LAST_TRACE_FILE="$trace_file"
      AI_LAST_FAILURE_KIND=""
      AI_LAST_ATTEMPT="$attempt"
      log "package=internal/$pkg linter=$linter warnings=$warning_count AI call completed attempt=$attempt/$AI_MAX_ATTEMPTS trace=$trace_file"
      return 0
    fi

    ai_exit=$?
    kind="$(classify_ai_failure "$trace_file" "$ai_exit")"

    AI_LAST_TRACE_FILE="$trace_file"
    AI_LAST_FAILURE_KIND="$kind"
    AI_LAST_ATTEMPT="$attempt"

    log_error "internal/$pkg" "$linter" "$warning_count" "AI call failed attempt=$attempt/$AI_MAX_ATTEMPTS classify=$kind trace=$trace_file" "$ai_exit"
    log_ai_trace_tail "$pkg" "$linter" "$warning_count" "$attempt/$AI_MAX_ATTEMPTS" "$trace_file"

    if [ "$attempt" -ge "$AI_MAX_ATTEMPTS" ] || ! ai_failure_is_retryable "$kind" "$ai_exit"; then
      return "$ai_exit"
    fi

    log "package=internal/$pkg linter=$linter warnings=$warning_count retrying AI call in ${delay}s (classify=$kind)"
    sleep "$delay"

    if [ "$delay" -lt "$AI_RETRY_MAX_DELAY_SECS" ]; then
      delay=$((delay * 2))
      if [ "$delay" -gt "$AI_RETRY_MAX_DELAY_SECS" ]; then
        delay="$AI_RETRY_MAX_DELAY_SECS"
      fi
    fi

    attempt=$((attempt + 1))
  done

  return 1
}

# P1: retry git operations when index.lock conflicts happen.
run_git_with_lock_retry() {
  local attempt=1
  local delay=1
  local output
  local rc

  while [ "$attempt" -le "$GIT_LOCK_RETRIES" ]; do
    if output=$(git "$@" 2>&1); then
      [ -n "$output" ] && printf '%s\n' "$output" >> "$LOG_FILE"
      return 0
    fi

    rc=$?
    [ -n "$output" ] && printf '%s\n' "$output" >> "$LOG_FILE"

    if printf '%s' "$output" | grep -qi 'index\.lock'; then
      if [ "$attempt" -lt "$GIT_LOCK_RETRIES" ]; then
        log "git index.lock detected for: git $* (attempt $attempt/$GIT_LOCK_RETRIES), retrying in ${delay}s"
        sleep "$delay"
        attempt=$((attempt + 1))
        delay=$((delay * 2))
        continue
      fi
    fi

    return "$rc"
  done

  return 1
}

atomic_write_file() {
  local target="$1"
  local tmp
  tmp="$(mktemp "$SCRIPT_DIR/.tmp-atomic.XXXXXX")"
  cat > "$tmp"
  mv "$tmp" "$target"
}

check_disk_space() {
  local required_kb=$((MIN_FREE_MB * 1024))
  local available_kb

  available_kb="$(df -Pk "$REPO_DIR" | awk 'NR==2 {print $4}')"
  if [ -z "$available_kb" ]; then
    log_error "global" "global" "0" "Failed to read disk space" 1
    exit 1
  fi

  if [ "$available_kb" -lt "$required_kb" ]; then
    log_error "global" "global" "0" "Insufficient disk space: need ${MIN_FREE_MB}MB+, have $((available_kb / 1024))MB"
    exit 1
  fi

  log "Disk preflight OK: $((available_kb / 1024))MB available"
}

normalize_pkg_name() {
  local raw="$1"
  raw="${raw#./}"
  raw="${raw#internal/}"
  raw="${raw%/...}"
  raw="${raw%/}"
  printf '%s' "$raw"
}

# -----------------------------------------------------------------------------
# Tool resolution
# -----------------------------------------------------------------------------

if ! command -v golangci-lint >/dev/null 2>&1; then
  GOLANGCI_LINT="$HOME/go/bin/golangci-lint"
  if [ ! -f "$GOLANGCI_LINT" ]; then
    log_error "global" "global" "0" "golangci-lint not found" 127
    exit 1
  fi
else
  GOLANGCI_LINT="golangci-lint"
fi

if ! command -v opencode >/dev/null 2>&1; then
  OPENCODE="/opt/homebrew/bin/opencode"
  if [ ! -f "$OPENCODE" ]; then
    log_error "global" "global" "0" "opencode not found" 127
    exit 1
  fi
else
  OPENCODE="opencode"
fi

require_cmd jq

# Parse linter list once
IFS=',' read -r -a LINT_ARRAY <<< "$LINTERS"

# Parse requested package filter (optional)
REQUESTED_PACKAGES=()
if [ -n "$PACKAGES" ]; then
  IFS=',' read -r -a RAW_REQUESTED_PACKAGES <<< "$PACKAGES"
  for raw_pkg in "${RAW_REQUESTED_PACKAGES[@]}"; do
    raw_pkg="$(echo "$raw_pkg" | xargs)"
    [ -n "$raw_pkg" ] || continue
    REQUESTED_PACKAGES+=("$(normalize_pkg_name "$raw_pkg")")
  done
fi

package_is_requested() {
  local pkg="$1"
  local req

  if [ "${#REQUESTED_PACKAGES[@]}" -eq 0 ]; then
    return 0
  fi

  for req in "${REQUESTED_PACKAGES[@]}"; do
    if [ "$pkg" = "$req" ]; then
      return 0
    fi
  done

  return 1
}

# -----------------------------------------------------------------------------
# State file helpers (atomic updates)
# -----------------------------------------------------------------------------

init_state_file() {
  local now
  now="$(timestamp)"

  jq -n \
    --arg branch "$BRANCH" \
    --arg model "$MODEL" \
    --arg linters "$LINTERS" \
    --arg now "$now" \
    --arg scan_cache "$SCAN_CACHE_FILE" \
    '{
      version: 3,
      branch: $branch,
      model: $model,
      linters: $linters,
      started_at: $now,
      updated_at: $now,
      scan_cache: $scan_cache,
      totals: {
        commits: 0,
        packages_completed: 0
      },
      packages: {}
    }' | atomic_write_file "$STATE_FILE"
}

ensure_state_file() {
  if [ ! -f "$STATE_FILE" ]; then
    init_state_file
    log "Created new state file: $STATE_FILE"
    return
  fi

  if jq -e \
    --arg branch "$BRANCH" \
    --arg model "$MODEL" \
    --arg linters "$LINTERS" \
    '.version == 3 and .branch == $branch and .model == $model and .linters == $linters' \
    "$STATE_FILE" >/dev/null 2>&1; then
    log "Resuming from existing state file: $STATE_FILE"
    return
  fi

  local backup="$STATE_FILE.bak.$(date +%Y%m%d-%H%M%S)"
  mv "$STATE_FILE" "$backup"
  log "State file mismatch; backed up to: $backup"
  init_state_file
}

state_update() {
  local tmp
  tmp="$(mktemp "$SCRIPT_DIR/.state-tmp.XXXXXX")"
  jq "$@" "$STATE_FILE" > "$tmp"
  mv "$tmp" "$STATE_FILE"
}

state_get_linter_status() {
  local pkg="$1"
  local linter="$2"
  jq -r --arg pkg "$pkg" --arg linter "$linter" '.packages[$pkg].linters[$linter].status // ""' "$STATE_FILE"
}

state_get_package_status() {
  local pkg="$1"
  jq -r --arg pkg "$pkg" '.packages[$pkg].status // ""' "$STATE_FILE"
}

state_set_scan_cache() {
  local now
  now="$(timestamp)"

  state_update \
    --arg path "$SCAN_CACHE_FILE" \
    --arg now "$now" \
    '.scan_cache = $path | .updated_at = $now'
}

state_set_linter_status() {
  local pkg="$1"
  local linter="$2"
  local status="$3"
  local warning_count="$4"
  local commit_sha="$5"
  local reason="$6"

  local now
  now="$(timestamp)"

  state_update \
    --arg pkg "$pkg" \
    --arg linter "$linter" \
    --arg status "$status" \
    --arg warning_count "$warning_count" \
    --arg commit_sha "$commit_sha" \
    --arg reason "$reason" \
    --arg now "$now" \
    '
      .updated_at = $now
      | setpath(["packages", $pkg, "linters", $linter]; {
          status: $status,
          warning_count: ($warning_count | tonumber),
          commit: $commit_sha,
          reason: $reason,
          updated_at: $now
        })
      | setpath(["packages", $pkg, "status"]; (.packages[$pkg].status // "pending"))
      | setpath(["packages", $pkg, "reason"]; (.packages[$pkg].reason // ""))
    '
}

state_set_package_status() {
  local pkg="$1"
  local status="$2"
  local reason="$3"

  local now
  now="$(timestamp)"

  state_update \
    --arg pkg "$pkg" \
    --arg status "$status" \
    --arg reason "$reason" \
    --arg now "$now" \
    '
      .updated_at = $now
      | setpath(["packages", $pkg, "status"]; $status)
      | setpath(["packages", $pkg, "reason"]; $reason)
      | setpath(["packages", $pkg, "linters"]; (.packages[$pkg].linters // {}))
    '
}

state_increment_commits() {
  local now
  now="$(timestamp)"

  state_update --arg now "$now" '.totals.commits += 1 | .updated_at = $now'
}

state_increment_packages_completed_once() {
  local pkg="$1"
  local current_status
  current_status="$(state_get_package_status "$pkg")"

  if [ "$current_status" = "completed" ]; then
    return
  fi

  state_set_package_status "$pkg" "completed" ""

  local now
  now="$(timestamp)"
  state_update --arg now "$now" '.totals.packages_completed += 1 | .updated_at = $now'
}

update_package_completion_status() {
  local pkg="$1"
  local linter
  local status

  for linter in "${LINT_ARRAY[@]}"; do
    status="$(state_get_linter_status "$pkg" "$linter")"
    if [ "$status" != "completed" ]; then
      return
    fi
  done

  state_increment_packages_completed_once "$pkg"
}

write_progress_snapshot() {
  local tmp
  tmp="$(mktemp "$SCRIPT_DIR/.progress-tmp.XXXXXX")"

  {
    echo "# Lint Fixer Progress (v3)"
    echo ""
    echo "- Updated: $(timestamp)"
    echo "- Branch: $BRANCH"
    echo "- Commits: $(jq -r '.totals.commits // 0' "$STATE_FILE")"
    echo "- Packages completed: $(jq -r '.totals.packages_completed // 0' "$STATE_FILE")"
    echo ""
    echo "## Package Status"

    local pkg
    for pkg in "${PACKAGES_TO_FIX[@]:-}"; do
      [ -n "$pkg" ] || continue
      echo "- internal/$pkg: $(state_get_package_status "$pkg")"
    done
  } > "$tmp"

  mv "$tmp" "$PROGRESS_FILE"
}

# -----------------------------------------------------------------------------
# Lint scan cache (single scan, reused everywhere)
# -----------------------------------------------------------------------------

scan_lint_warnings_once() {
  local reuse_cache=false

  if [ -f "$SCAN_CACHE_FILE" ]; then
    if jq -e \
      --arg linters "$LINTERS" \
      --arg base_sha "$BASE_SHA" \
      '.metadata.linters == $linters and .metadata.base_sha == $base_sha and (.issues | type) == "array"' \
      "$SCAN_CACHE_FILE" >/dev/null 2>&1; then
      reuse_cache=true
    fi
  fi

  if [ "$reuse_cache" = true ]; then
    log "Reusing cached scan JSON: $SCAN_CACHE_FILE"
  else
    log "Running golangci-lint scan once for all configured linters..."

    local raw_tmp
    raw_tmp="$(mktemp "$SCRIPT_DIR/.scan-raw.XXXXXX")"

    local enable_args=()
    local linter
    for linter in "${LINT_ARRAY[@]}"; do
      enable_args+=(--enable "$linter")
    done

    if "$GOLANGCI_LINT" run \
      --disable-all \
      --issues-exit-code 0 \
      --out-format json \
      "${enable_args[@]}" \
      ./internal/... > "$raw_tmp" 2>> "$LOG_FILE"; then
      :
    else
      local scan_exit=$?
      log_error "global" "global" "0" "golangci-lint scan failed" "$scan_exit"
      rm -f "$raw_tmp"
      exit 1
    fi

    jq -n \
      --arg now "$(timestamp)" \
      --arg linters "$LINTERS" \
      --arg base_sha "$BASE_SHA" \
      --slurpfile raw "$raw_tmp" \
      '{
        metadata: {
          generated_at: $now,
          linters: $linters,
          base_sha: $base_sha,
          target: "./internal/..."
        },
        issues: ($raw[0].Issues // [])
      }' | atomic_write_file "$SCAN_CACHE_FILE"

    rm -f "$raw_tmp"
    log "Wrote scan cache: $SCAN_CACHE_FILE"
  fi

  state_set_scan_cache

  jq -r '
    .issues[]?
    | select(.Pos.Filename? | startswith("internal/"))
    | (try (.Pos.Filename | capture("^internal/(?<pkg>[^/]+)/").pkg) catch "") as $pkg
    | select($pkg != "")
    | [
        $pkg,
        .FromLinter,
        "\(.Pos.Filename):\(.Pos.Line):\(.Pos.Column): \(.Text) (\(.FromLinter))"
      ]
    | @tsv
  ' "$SCAN_CACHE_FILE" | atomic_write_file "$WARNINGS_TSV_FILE"

  log "Wrote warning index: $WARNINGS_TSV_FILE"
}

get_warning_count() {
  local pkg="$1"
  local linter="$2"
  awk -F '\t' -v p="$pkg" -v l="$linter" '$1 == p && $2 == l {c++} END {print c + 0}' "$WARNINGS_TSV_FILE"
}

dump_warnings_for_pkg_linter() {
  local pkg="$1"
  local linter="$2"
  local out_file="$3"
  awk -F '\t' -v p="$pkg" -v l="$linter" '$1 == p && $2 == l {print $3}' "$WARNINGS_TSV_FILE" > "$out_file"
}

collect_packages_to_fix() {
  PACKAGES_TO_FIX=()
  PACKAGE_COUNTS=()

  local counts_tmp
  counts_tmp="$(mktemp "$SCRIPT_DIR/.pkg-counts.XXXXXX")"

  awk -F '\t' 'NF >= 2 {count[$1]++} END {for (pkg in count) printf "%d\t%s\n", count[pkg], pkg}' "$WARNINGS_TSV_FILE" \
    | sort -n -k1,1 -k2,2 > "$counts_tmp"

  while IFS=$'\t' read -r count pkg; do
    [ -n "$pkg" ] || continue

    if ! package_is_requested "$pkg"; then
      continue
    fi

    PACKAGES_TO_FIX+=("$pkg")
    PACKAGE_COUNTS+=("$count")
  done < "$counts_tmp"

  rm -f "$counts_tmp"
}

# -----------------------------------------------------------------------------
# Prompt generation
# -----------------------------------------------------------------------------

build_prompt() {
  local pkg="$1"
  local linter="$2"
  local warning_count="$3"
  local warnings="$4"

  case "$linter" in
    errcheck)
      cat <<PROMPT_EOF
## Task: Fix all $warning_count errcheck warnings in internal/$pkg

**Why this matters:** Unchecked errors can cause silent failures, data corruption, or security issues in production. We need proper error handling to make Pulse reliable for users monitoring critical infrastructure.

**All errcheck warnings in this package:**
\`\`\`
$warnings
\`\`\`

**Your approach:**
You decide how to tackle these - all at once, group by file, whatever makes sense to you.

**Guidelines:**

For **test files** (\`*_test.go\`):
- If the error is truly irrelevant (e.g., \`defer file.Close()\` in test setup), use: \`_ = expr\`

For **production code**:
- If function returns \`error\`: propagate it
  - Good: \`return fmt.Errorf("failed to save: %w", err)\`
- If function doesn't return error: log and continue
  - Example: \`if err := conn.Close(); err != nil { log.Warn("close failed: %v", err) }\`
- For critical operations: ALWAYS handle errors

**Constraints:**
- Do NOT change function signatures
- Preserve all existing behavior
- Ensure tests pass

**Expected output:** All errors properly handled.
PROMPT_EOF
      ;;

    dupl)
      cat <<PROMPT_EOF
## Task: Eliminate all $warning_count code duplication warnings in internal/$pkg

**Why this matters:** Duplicated code creates maintenance burden. When we fix bugs or add features, we have to find and update multiple copies, which leads to inconsistencies.

**All dupl warnings in this package:**
\`\`\`
$warnings
\`\`\`

**Your approach:**
Figure out how to best eliminate these duplications - extract helpers, refactor shared logic, whatever makes sense to you.

**Guidelines:**
1. **Identify** duplicated logic blocks
2. **Extract** into well-named helper functions:
   - Use descriptive names
   - Keep helpers private unless they need export
   - Place near call sites
3. **Replace** duplicated code with helper calls

**Constraints:**
- Do NOT over-abstract
- Preserve exact behavior
- Ensure tests pass

**Expected output:** DRY code with clear, well-named helpers.
PROMPT_EOF
      ;;

    *)
      cat <<PROMPT_EOF
## Task: Fix all $warning_count $linter warnings in internal/$pkg

**All warnings:**
\`\`\`
$warnings
\`\`\`

**Instructions:**
Fix these warnings. Keep changes minimal and ensure tests pass.
PROMPT_EOF
      ;;
  esac
}

# -----------------------------------------------------------------------------
# Worker execution (runs in isolated per-package worktree)
# -----------------------------------------------------------------------------

emit_worker_result() {
  local result_file="$1"
  local pkg="$2"
  local linter="$3"
  local status="$4"
  local warning_count="$5"
  local worker_commit="$6"
  local reason="$7"
  local exit_code="$8"

  jq -nc \
    --arg package "$pkg" \
    --arg linter "$linter" \
    --arg status "$status" \
    --arg warning_count "$warning_count" \
    --arg worker_commit "$worker_commit" \
    --arg reason "$reason" \
    --arg exit_code "$exit_code" \
    --arg ts "$(timestamp)" \
    '{
      package: $package,
      linter: $linter,
      status: $status,
      warning_count: ($warning_count | tonumber),
      worker_commit: $worker_commit,
      reason: $reason,
      exit_code: ($exit_code | tonumber),
      timestamp: $ts
    }' >> "$result_file"
}

process_package_worker() {
  local pkg="$1"
  local worker_dir="$2"
  local result_file="$3"
  local pending_linter_csv="$4"

  local consecutive_ai_failures=0
  local linter

  cd "$worker_dir"
  : > "$result_file"

  IFS=',' read -r -a WORKER_LINTERS <<< "$pending_linter_csv"

  for linter in "${WORKER_LINTERS[@]}"; do
    local warnings_tmp
    local warnings
    local warning_count

    warning_count="$(get_warning_count "$pkg" "$linter")"

    if [ "$warning_count" -eq 0 ]; then
      log "package=internal/$pkg linter=$linter warnings=0 no warnings in cache"
      emit_worker_result "$result_file" "$pkg" "$linter" "completed" "0" "" "no_warnings" "0"
      continue
    fi

    warnings_tmp="$(mktemp "$worker_dir/.warnings.XXXXXX")"
    dump_warnings_for_pkg_linter "$pkg" "$linter" "$warnings_tmp"
    warnings="$(cat "$warnings_tmp")"
    rm -f "$warnings_tmp"

    local prompt
    prompt="$(build_prompt "$pkg" "$linter" "$warning_count" "$warnings")"

    if run_ai_with_retries "$pkg" "$linter" "$warning_count" "$prompt"; then
      consecutive_ai_failures=0
    else
      local ai_exit=$?
      local ai_reason="ai_failure"
      consecutive_ai_failures=$((consecutive_ai_failures + 1))
      if [ -n "${AI_LAST_FAILURE_KIND:-}" ]; then
        ai_reason="ai_${AI_LAST_FAILURE_KIND}"
      fi
      log_error "internal/$pkg" "$linter" "$warning_count" "AI call failed after $AI_LAST_ATTEMPT attempt(s) reason=$ai_reason trace=${AI_LAST_TRACE_FILE:-none}" "$ai_exit"

      # Safe revert in isolated worker: only this package in this throwaway worktree.
      git checkout -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true
      git clean -fd -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true

      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "$ai_reason" "$ai_exit"

      if [ "$consecutive_ai_failures" -ge "$AI_FAILURE_CIRCUIT" ]; then
        log_error "internal/$pkg" "$linter" "$warning_count" "Circuit breaker tripped after $AI_FAILURE_CIRCUIT consecutive AI failures" "$ai_exit"
        return 0
      fi

      continue
    fi

    if git diff --quiet HEAD -- "internal/$pkg"; then
      log_error "internal/$pkg" "$linter" "$warning_count" "AI produced no package-local changes" 1
      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "ai_no_changes" "1"
      continue
    fi

    log "package=internal/$pkg linter=$linter warnings=$warning_count running tests"
    if run_with_timeout "$TEST_TIMEOUT_SECS" go test "./internal/$pkg/..." >> "$LOG_FILE" 2>&1; then
      :
    else
      local test_exit=$?
      log_error "internal/$pkg" "$linter" "$warning_count" "go test failed" "$test_exit"

      git checkout -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true
      git clean -fd -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true

      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "test_failure" "$test_exit"
      continue
    fi

    local commit_msg
    commit_msg="fix($linter): internal/$pkg ($warning_count warnings)

Model: $MODEL
Context: Pulse infrastructure monitoring platform"

    if ! run_git_with_lock_retry add "internal/$pkg/"; then
      local add_exit=$?
      log_error "internal/$pkg" "$linter" "$warning_count" "git add failed" "$add_exit"
      git checkout -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true
      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "git_add_failed" "$add_exit"
      continue
    fi

    if git diff --cached --quiet; then
      log_error "internal/$pkg" "$linter" "$warning_count" "No staged changes after git add" 1
      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "no_staged_changes" "1"
      continue
    fi

    if run_git_with_lock_retry commit -m "$commit_msg"; then
      local worker_commit
      worker_commit="$(git rev-parse HEAD)"
      log "package=internal/$pkg linter=$linter warnings=$warning_count committed worker_sha=$worker_commit"
      emit_worker_result "$result_file" "$pkg" "$linter" "committed" "$warning_count" "$worker_commit" "worker_commit_ready" "0"
    else
      local commit_exit=$?
      log_error "internal/$pkg" "$linter" "$warning_count" "git commit failed" "$commit_exit"
      git reset --mixed HEAD >> "$LOG_FILE" 2>&1 || true
      git checkout -- "internal/$pkg" >> "$LOG_FILE" 2>&1 || true
      emit_worker_result "$result_file" "$pkg" "$linter" "failed" "$warning_count" "" "git_commit_failed" "$commit_exit"
      continue
    fi
  done

  return 0
}

# -----------------------------------------------------------------------------
# Job orchestration
# -----------------------------------------------------------------------------

JOB_PIDS=()
JOB_PKGS=()
JOB_RESULTS=()
JOB_WORKTREES=()
JOB_DONE_FILES=()
JOB_PENDING_LINTERS=()

supports_wait_n() {
  help wait 2>/dev/null | grep -q -- '-n'
}

build_pending_linter_csv_for_pkg() {
  local pkg="$1"
  local pending=()
  local linter
  local status

  for linter in "${LINT_ARRAY[@]}"; do
    status="$(state_get_linter_status "$pkg" "$linter")"
    if [ "$status" != "completed" ]; then
      pending+=("$linter")
    fi
  done

  local csv=""
  local idx
  for idx in "${!pending[@]}"; do
    if [ "$idx" -gt 0 ]; then
      csv+=","
    fi
    csv+="${pending[$idx]}"
  done

  printf '%s' "$csv"
}

launch_package_job() {
  local pkg="$1"
  local pending_csv="$2"

  local safe_pkg
  safe_pkg="${pkg//\//-}"

  local worker_dir="$WORKTREE_ROOT/$safe_pkg"
  local result_file="$RESULTS_DIR/$safe_pkg.ndjson"
  local done_file="$RESULTS_DIR/$safe_pkg.done"

  rm -rf "$worker_dir"

  if ! run_git_with_lock_retry worktree add --detach "$worker_dir" "$BASE_SHA"; then
    local wt_exit=$?
    log_error "internal/$pkg" "global" "0" "Failed to create worker worktree" "$wt_exit"
    state_set_package_status "$pkg" "failed" "worktree_create_failed"
    return 1
  fi

  (
    set +e
    process_package_worker "$pkg" "$worker_dir" "$result_file" "$pending_csv"
    worker_rc=$?
    echo "$worker_rc" > "$done_file"
    exit 0
  ) &

  local pid=$!

  JOB_PIDS+=("$pid")
  JOB_PKGS+=("$pkg")
  JOB_RESULTS+=("$result_file")
  JOB_WORKTREES+=("$worker_dir")
  JOB_DONE_FILES+=("$done_file")
  JOB_PENDING_LINTERS+=("$pending_csv")

  log "Started worker pid=$pid package=internal/$pkg pending_linters=$pending_csv"
}

cleanup_worker() {
  local worker_dir="$1"

  if [ -d "$worker_dir" ]; then
    run_git_with_lock_retry worktree remove --force "$worker_dir" >> "$LOG_FILE" 2>&1 || true
    rm -rf "$worker_dir" || true
  fi
}

integrate_worker_results() {
  local pkg="$1"
  local result_file="$2"

  if [ ! -f "$result_file" ]; then
    log_error "internal/$pkg" "global" "0" "Worker result file missing" 1
    state_set_package_status "$pkg" "failed" "missing_result_file"
    return
  fi

  while IFS= read -r line; do
    [ -n "$line" ] || continue

    local linter status warning_count worker_commit reason exit_code

    linter="$(printf '%s' "$line" | jq -r '.linter')"
    status="$(printf '%s' "$line" | jq -r '.status')"
    warning_count="$(printf '%s' "$line" | jq -r '.warning_count')"
    worker_commit="$(printf '%s' "$line" | jq -r '.worker_commit')"
    reason="$(printf '%s' "$line" | jq -r '.reason')"
    exit_code="$(printf '%s' "$line" | jq -r '.exit_code')"

    case "$status" in
      completed)
        state_set_linter_status "$pkg" "$linter" "completed" "$warning_count" "" "$reason"
        log "package=internal/$pkg linter=$linter warnings=$warning_count completed reason=$reason"
        ;;

      committed)
        # Integrate worker commit into the main branch and persist state immediately.
        if run_git_with_lock_retry cherry-pick "$worker_commit" >> "$LOG_FILE" 2>&1; then
          local main_commit
          main_commit="$(git rev-parse HEAD)"
          state_set_linter_status "$pkg" "$linter" "completed" "$warning_count" "$main_commit" "cherry_picked"
          state_increment_commits
          log "package=internal/$pkg linter=$linter warnings=$warning_count integrated commit=$main_commit"
        else
          local cp_exit=$?
          log_error "internal/$pkg" "$linter" "$warning_count" "cherry-pick failed for worker commit $worker_commit" "$cp_exit"
          git cherry-pick --abort >> "$LOG_FILE" 2>&1 || true
          state_set_linter_status "$pkg" "$linter" "failed" "$warning_count" "" "cherry_pick_failed"
        fi
        ;;

      failed)
        log_error "internal/$pkg" "$linter" "$warning_count" "Worker step failed: $reason" "$exit_code"
        state_set_linter_status "$pkg" "$linter" "failed" "$warning_count" "" "$reason"
        ;;

      *)
        log_error "internal/$pkg" "$linter" "$warning_count" "Unknown worker status: $status" 1
        state_set_linter_status "$pkg" "$linter" "failed" "$warning_count" "" "unknown_status"
        ;;
    esac
  done < "$result_file"

  update_package_completion_status "$pkg"
}

reap_finished_jobs() {
  local i
  local new_pids=()
  local new_pkgs=()
  local new_results=()
  local new_worktrees=()
  local new_done=()
  local new_pending_linters=()

  for i in "${!JOB_PIDS[@]}"; do
    local pid="${JOB_PIDS[$i]}"
    local pkg="${JOB_PKGS[$i]}"
    local result_file="${JOB_RESULTS[$i]}"
    local worker_dir="${JOB_WORKTREES[$i]}"
    local done_file="${JOB_DONE_FILES[$i]}"
    local pending_linters="${JOB_PENDING_LINTERS[$i]:-global}"

    if [ -f "$done_file" ] || ! kill -0 "$pid" 2>/dev/null; then
      wait "$pid" 2>/dev/null || true
      if [ -f "$result_file" ]; then
        integrate_worker_results "$pkg" "$result_file"
      else
        log_error "internal/$pkg" "${pending_linters:-global}" "0" "Worker exited without result file" 1
        state_set_package_status "$pkg" "failed" "worker_crash_no_result"
      fi
      cleanup_worker "$worker_dir"
      rm -f "$done_file"
      write_progress_snapshot
    else
      new_pids+=("$pid")
      new_pkgs+=("$pkg")
      new_results+=("$result_file")
      new_worktrees+=("$worker_dir")
      new_done+=("$done_file")
      new_pending_linters+=("$pending_linters")
    fi
  done

  JOB_PIDS=("${new_pids[@]}")
  JOB_PKGS=("${new_pkgs[@]}")
  JOB_RESULTS=("${new_results[@]}")
  JOB_WORKTREES=("${new_worktrees[@]}")
  JOB_DONE_FILES=("${new_done[@]}")
  JOB_PENDING_LINTERS=("${new_pending_linters[@]}")
}

wait_for_available_slot() {
  while [ "${#JOB_PIDS[@]}" -ge "$MAX_PARALLEL" ]; do
    if supports_wait_n; then
      wait -n || true
    else
      sleep 1
    fi
    reap_finished_jobs
  done
}

wait_for_all_jobs() {
  while [ "${#JOB_PIDS[@]}" -gt 0 ]; do
    if supports_wait_n; then
      wait -n || true
    else
      sleep 1
    fi
    reap_finished_jobs
  done
}

cleanup_all_workers() {
  local wt
  for wt in "${JOB_WORKTREES[@]:-}"; do
    [ -n "$wt" ] || continue
    cleanup_worker "$wt"
  done
  rm -rf "$WORKTREE_ROOT" "$RESULTS_DIR" || true
}

trap cleanup_all_workers EXIT

# -----------------------------------------------------------------------------
# Main flow
# -----------------------------------------------------------------------------

START_TIME_EPOCH="$(date +%s)"

{
  echo ""
  echo "╔══════════════════════════════════════════════════════╗"
  echo "║     Pulse Lint Fixer v3 (Parallel + Resume)         ║"
  echo "╠══════════════════════════════════════════════════════╣"
  printf "║  Model:       %-39s║\n" "$MODEL"
  printf "║  Linters:     %-39s║\n" "$LINTERS"
  printf "║  Branch:      %-39s║\n" "$BRANCH"
  printf "║  Max parallel:%-39s║\n" "$MAX_PARALLEL"
  printf "║  Started:     %-39s║\n" "$(date)"
  echo "╚══════════════════════════════════════════════════════╝"
  echo ""
} | tee -a "$LOG_FILE"

cd "$REPO_DIR"

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
  run_git_with_lock_retry checkout -b "$BRANCH" >> "$LOG_FILE" 2>&1 || run_git_with_lock_retry checkout "$BRANCH" >> "$LOG_FILE" 2>&1
  log "Working on branch: $BRANCH"
else
  log "Working on current branch: $BRANCH"
fi

BASE_SHA="$(git rev-parse HEAD)"
log "Base commit: $BASE_SHA"

check_disk_space
ensure_state_file

# Build (or reuse) scan cache once and reuse it for all package/linter work.
scan_lint_warnings_once
collect_packages_to_fix

if [ "${#PACKAGES_TO_FIX[@]}" -eq 0 ]; then
  log "No warnings found for requested scope"
  exit 0
fi

log "Found ${#PACKAGES_TO_FIX[@]} packages with warnings"
for idx in "${!PACKAGES_TO_FIX[@]}"; do
  log "  internal/${PACKAGES_TO_FIX[$idx]}: ${PACKAGE_COUNTS[$idx]} warnings"
done

write_progress_snapshot

# Launch package workers with throttling.
for pkg in "${PACKAGES_TO_FIX[@]}"; do
  # P0 safe revert logic: do not touch packages with existing local WIP.
  if [ -n "$(git status --porcelain -- "internal/$pkg")" ]; then
    log_error "internal/$pkg" "global" "0" "Pre-existing changes detected; skipping package to protect local WIP" 1
    state_set_package_status "$pkg" "skipped" "preexisting_changes"
    write_progress_snapshot
    continue
  fi

  pending_csv="$(build_pending_linter_csv_for_pkg "$pkg")"
  if [ -z "$pending_csv" ]; then
    log "Skipping internal/$pkg: all configured linters already completed in state"
    state_increment_packages_completed_once "$pkg"
    write_progress_snapshot
    continue
  fi

  wait_for_available_slot
  if [ "${#JOB_PIDS[@]}" -gt 0 ] && [ "$WORKER_LAUNCH_STAGGER_SECS" -gt 0 ]; then
    log "Staggering next worker launch by ${WORKER_LAUNCH_STAGGER_SECS}s to reduce API burst"
    sleep "$WORKER_LAUNCH_STAGGER_SECS"
  fi
  if ! launch_package_job "$pkg" "$pending_csv"; then
    write_progress_snapshot
    continue
  fi
done

wait_for_all_jobs
write_progress_snapshot

TOTAL_COMMITS="$(jq -r '.totals.commits // 0' "$STATE_FILE")"
PACKAGES_COMPLETED="$(jq -r '.totals.packages_completed // 0' "$STATE_FILE")"
ELAPSED_SECS=$(( $(date +%s) - START_TIME_EPOCH ))
ELAPSED_MIN=$((ELAPSED_SECS / 60))
ELAPSED_REM=$((ELAPSED_SECS % 60))

{
  echo ""
  echo "╔══════════════════════════════════════════════════════╗"
  echo "║  Lint Fixer v3 Complete                              ║"
  echo "╠══════════════════════════════════════════════════════╣"
  printf "║  Packages:  %-41s║\n" "$PACKAGES_COMPLETED / ${#PACKAGES_TO_FIX[@]}"
  printf "║  Commits:   %-41s║\n" "$TOTAL_COMMITS"
  printf "║  Duration:  %-41s║\n" "${ELAPSED_MIN}m ${ELAPSED_REM}s"
  echo "╚══════════════════════════════════════════════════════╝"
  echo ""
} | tee -a "$LOG_FILE"

log "State file: $STATE_FILE"
log "Scan cache: $SCAN_CACHE_FILE"
log "AI traces: $AI_TRACE_DIR"
log "Progress: $PROGRESS_FILE"
log "Full log: $LOG_FILE"
