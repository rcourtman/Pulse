#!/usr/bin/env bash
#
# Shared common utilities for Pulse shell scripts.
# Provides logging, privilege escalation, command execution helpers, and cleanup management.

set -o errexit
set -o nounset
set -o pipefail

shopt -s extglob

declare -gA COMMON__LOG_LEVELS=(
  [debug]=10
  [info]=20
  [warn]=30
  [error]=40
)

declare -g COMMON__SCRIPT_PATH=""
declare -g COMMON__SCRIPT_DIR=""
declare -g COMMON__ORIGINAL_ARGS=()

declare -g COMMON__LOG_LEVEL="info"
declare -g COMMON__LOG_LEVEL_NUM=20
declare -g COMMON__DEBUG=false
declare -g COMMON__COLOR_ENABLED=true

declare -g COMMON__DRY_RUN=false

declare -ag COMMON__CLEANUP_DESCRIPTIONS=()
declare -ag COMMON__CLEANUP_COMMANDS=()
declare -g COMMON__CLEANUP_REGISTERED=false

# shellcheck disable=SC2034
declare -g COMMON__ANSI_RESET=""
# shellcheck disable=SC2034
declare -g COMMON__ANSI_INFO=""
# shellcheck disable=SC2034
declare -g COMMON__ANSI_WARN=""
# shellcheck disable=SC2034
declare -g COMMON__ANSI_ERROR=""
# shellcheck disable=SC2034
declare -g COMMON__ANSI_DEBUG=""

# common::init
# Initializes the common library. Call at the top of every script.
# Sets script metadata, log level, color handling, traps, and stores original args.
common::init() {
  COMMON__SCRIPT_PATH="$(common::__resolve_script_path)"
  COMMON__SCRIPT_DIR="$(dirname "${COMMON__SCRIPT_PATH}")"
  COMMON__ORIGINAL_ARGS=("$@")

  common::__configure_color
  common::__configure_log_level

  if [[ "${COMMON__DEBUG}" == true ]]; then
    common::log_debug "Debug mode enabled"
  fi

  if [[ "${COMMON__CLEANUP_REGISTERED}" == false ]]; then
    trap common::cleanup_run EXIT
    COMMON__CLEANUP_REGISTERED=true
  fi
}

# common::log_info "message"
# Logs informational messages. Printed to stdout.
common::log_info() {
  common::__log "info" "$@"
}

# common::log_warn "message"
# Logs warning messages. Printed to stderr.
common::log_warn() {
  common::__log "warn" "$@"
}

# common::log_error "message"
# Logs error messages. Printed to stderr.
common::log_error() {
  common::__log "error" "$@"
}

# common::log_debug "message"
# Logs debug messages when log level is debug. Printed to stderr.
common::log_debug() {
  common::__log "debug" "$@"
}

# common::fail "message" [--code N]
# Logs an error message and exits with provided code (default 1).
common::fail() {
  local exit_code=1
  local message_parts=()
  while (($#)); do
    case "$1" in
      --code)
        shift
        exit_code="${1:-1}"
        ;;
      *)
        message_parts+=("$1")
        ;;
    esac
    shift || break
  done
  local message="${message_parts[*]}"
  common::log_error "${message}"

  if common::is_interactive; then
    echo ""
    if [[ -t 0 ]]; then
        read -p "Press Enter to exit..."
    elif [[ -e /dev/tty ]]; then
        read -p "Press Enter to exit..." < /dev/tty
    fi
  fi

  exit "${exit_code}"
}

# common::require_command cmd1 [cmd2 ...]
# Verifies that required commands exist. Fails if any are missing.
common::require_command() {
  local missing=()
  local cmd
  for cmd in "$@"; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      missing+=("${cmd}")
    fi
  done

  if ((${#missing[@]} > 0)); then
    common::fail "Missing required command(s): ${missing[*]}"
  fi
}

# common::is_interactive
# Returns success when running in an interactive TTY or PULSE_FORCE_INTERACTIVE=1.
common::is_interactive() {
  if [[ "${PULSE_FORCE_INTERACTIVE:-0}" == "1" ]]; then
    return 0
  fi
  [[ -t 0 && -t 1 ]]
}

# common::ensure_root [--allow-sudo] [--args "${COMMON__ORIGINAL_ARGS[@]}"]
# Ensures the script is running with root privileges. Optionally re-execs with sudo.
common::ensure_root() {
  local allow_sudo=false
  local reexec_args=()

  while (($#)); do
    case "$1" in
      --allow-sudo)
        allow_sudo=true
        ;;
      --args)
        shift
        reexec_args=("$@")
        break
        ;;
      *)
        common::log_warn "Unknown argument to common::ensure_root: $1"
        ;;
    esac
    shift || break
  done

  if [[ "${EUID}" -eq 0 ]]; then
    return 0
  fi

  if [[ "${allow_sudo}" == true ]]; then
    if common::is_interactive; then
      common::log_info "Escalating privileges with sudo..."
      common::sudo_exec "${COMMON__SCRIPT_PATH}" "${reexec_args[@]}"
      exit 0
    fi
    common::fail "Root privileges required; rerun with sudo or as root."
  fi

  common::fail "Root privileges required."
}

# common::sudo_exec command [args...]
# Executes a command via sudo, providing user guidance on failure.
common::sudo_exec() {
  local sudo_cmd="${PULSE_SUDO_CMD:-sudo}"
  if command -v "${sudo_cmd}" >/dev/null 2>&1; then
    exec "${sudo_cmd}" -- "$@"
  fi

  cat <<'EOF' >&2
Unable to escalate privileges automatically because sudo is unavailable.
Please install sudo or rerun this script as root.
EOF
  exit 1
}

# common::run [--label desc] [--retries N] [--backoff "1 2 4"] command [args...]
# Executes a command, respecting dry-run mode and retry policies.
common::run() {
  local label=""
  local retries=1
  local backoff=()
  local -a cmd=()

  while (($#)); do
    case "$1" in
      --label)
        label="$2"
        shift 2
        continue
        ;;
      --retries)
        retries="$2"
        shift 2
        continue
        ;;
      --backoff)
        read -r -a backoff <<<"$2"
        shift 2
        continue
        ;;
      --)
        shift
        break
        ;;
      -*)
        common::log_warn "Unknown flag for common::run: $1"
        shift
        continue
        ;;
      *)
        break
        ;;
    esac
  done

  cmd=("$@")
  [[ -z "${label}" ]] && label="${cmd[*]}"

  if [[ "${COMMON__DRY_RUN}" == true ]]; then
    common::log_info "[dry-run] ${label}"
    return 0
  fi

  local attempt=1
  local exit_code=0

  while (( attempt <= retries )); do
    if "${cmd[@]}"; then
      return 0
    fi

    exit_code=$?
    if (( attempt == retries )); then
      common::log_error "Command failed (${exit_code}): ${label}"
      return "${exit_code}"
    fi

    local sleep_time="${backoff[$((attempt - 1))]:-1}"
    common::log_warn "Command failed (${exit_code}): ${label} — retrying in ${sleep_time}s (attempt ${attempt}/${retries})"
    sleep "${sleep_time}"
    ((attempt++))
  done

  return "${exit_code}"
}

# common::run_capture [--label desc] command [args...]
# Executes a command and captures stdout. Respects dry-run mode.
common::run_capture() {
  local label=""

  while (($#)); do
    case "$1" in
      --label)
        label="$2"
        shift 2
        continue
        ;;
      --)
        shift
        break
        ;;
      -*)
        common::log_warn "Unknown flag for common::run_capture: $1"
        shift
        continue
        ;;
      *)
        break
        ;;
    esac
  done

  local -a cmd=("$@")
  [[ -z "${label}" ]] && label="${cmd[*]}"

  if [[ "${COMMON__DRY_RUN}" == true ]]; then
    common::log_info "[dry-run] ${label}"
    echo ""
    return 0
  fi

  "${cmd[@]}"
}

# common::temp_dir <var> [--prefix name]
# Creates a temporary directory tracked for cleanup and assigns it to <var>.
common::temp_dir() {
  local var_name=""
  local prefix="pulse-"

  if (($#)) && [[ $1 != --* ]]; then
    var_name="$1"
    shift
  fi

  while (($#)); do
    case "$1" in
      --prefix)
        prefix="$2"
        shift 2
        continue
        ;;
      *)
        common::log_warn "Unknown argument to common::temp_dir: $1"
        shift
        continue
        ;;
    esac
  done

  local dir
  dir="$(mktemp -d -t "${prefix}XXXXXX")"
  if (( "${BASH_SUBSHELL:-0}" > 0 )); then
    common::log_warn "common::temp_dir invoked in subshell; cleanup handlers will not be registered automatically for ${dir}"
  else
    common::cleanup_push "Remove temp dir ${dir}" "rm -rf ${dir@Q}"
  fi

  if [[ -n "${var_name}" ]]; then
    printf -v "${var_name}" '%s' "${dir}"
  else
    printf '%s\n' "${dir}"
  fi
}

# common::cleanup_push "description" "command"
# Adds a cleanup handler executed in LIFO order on exit.
common::cleanup_push() {
  local description="${1:-}"
  local command="${2:-}"

  if [[ -z "${command}" ]]; then
    common::log_warn "Ignoring cleanup handler without command"
    return
  fi

  COMMON__CLEANUP_DESCRIPTIONS+=("${description}")
  COMMON__CLEANUP_COMMANDS+=("${command}")
}

# common::cleanup_run
# Executes registered cleanup handlers. Called automatically via EXIT trap.
common::cleanup_run() {
  local idx=$(( ${#COMMON__CLEANUP_COMMANDS[@]} - 1 ))
  while (( idx >= 0 )); do
    local command="${COMMON__CLEANUP_COMMANDS[$idx]}"
    local description="${COMMON__CLEANUP_DESCRIPTIONS[$idx]}"
    if [[ -n "${description}" ]]; then
      common::log_debug "Running cleanup: ${description}"
    fi
    eval "${command}"
    ((idx--))
  done

  COMMON__CLEANUP_COMMANDS=()
  COMMON__CLEANUP_DESCRIPTIONS=()
}

# common::set_dry_run true|false
# Enables or disables global dry-run mode.
common::set_dry_run() {
  local flag="${1:-false}"
  case "${flag}" in
    true|1|yes)
      COMMON__DRY_RUN=true
      ;;
    false|0|no)
      COMMON__DRY_RUN=false
      ;;
    *)
      common::log_warn "Unknown dry-run flag: ${flag}"
      COMMON__DRY_RUN=true
      ;;
  esac
}

# common::is_dry_run
# Returns success when dry-run mode is active.
common::is_dry_run() {
  [[ "${COMMON__DRY_RUN}" == true ]]
}

# Internal helper: configure color output.
common::__configure_color() {
  if [[ "${PULSE_NO_COLOR:-0}" == "1" || ! -t 1 ]]; then
    COMMON__COLOR_ENABLED=false
  fi

  if [[ "${COMMON__COLOR_ENABLED}" == true ]]; then
    COMMON__ANSI_RESET=$'\033[0m'
    COMMON__ANSI_INFO=$'\033[1;34m'
    COMMON__ANSI_WARN=$'\033[1;33m'
    COMMON__ANSI_ERROR=$'\033[1;31m'
    COMMON__ANSI_DEBUG=$'\033[1;35m'
  else
    COMMON__ANSI_RESET=""
    COMMON__ANSI_INFO=""
    COMMON__ANSI_WARN=""
    COMMON__ANSI_ERROR=""
    COMMON__ANSI_DEBUG=""
  fi
}

# Internal helper: set log level and debug flag.
common::__configure_log_level() {
  if [[ "${PULSE_DEBUG:-0}" == "1" ]]; then
    COMMON__DEBUG=true
    COMMON__LOG_LEVEL="debug"
  else
    COMMON__LOG_LEVEL="${PULSE_LOG_LEVEL:-info}"
  fi

  COMMON__LOG_LEVEL="${COMMON__LOG_LEVEL,,}"
  COMMON__LOG_LEVEL_NUM="${COMMON__LOG_LEVELS[${COMMON__LOG_LEVEL}]:-20}"
}

# Internal helper: generic logger.
common::__log() {
  local level="$1"
  shift
  local message="$*"

  local level_num="${COMMON__LOG_LEVELS[${level}]:-20}"
  if (( level_num < COMMON__LOG_LEVEL_NUM )); then
    return 0
  fi

  local timestamp
  timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local color=""

  case "${level}" in
    info) color="${COMMON__ANSI_INFO}" ;;
    warn) color="${COMMON__ANSI_WARN}" ;;
    error) color="${COMMON__ANSI_ERROR}" ;;
    debug) color="${COMMON__ANSI_DEBUG}" ;;
  esac

  local formatted="[$timestamp] [${level^^}] ${message}"
  if [[ "${level}" == "info" ]]; then
    printf '%s%s%s\n' "${color}" "${formatted}" "${COMMON__ANSI_RESET}"
  else
    printf '%s%s%s\n' "${color}" "${formatted}" "${COMMON__ANSI_RESET}" >&2
  fi
}

# Internal helper: determine absolute script path.
common::__resolve_script_path() {
  local source="${BASH_SOURCE[1]:-${BASH_SOURCE[0]}}"
  if [[ -z "${source}" ]]; then
    pwd
    return
  fi

  if [[ "${source}" == /* ]]; then
    printf '%s\n' "${source}"
    return
  fi

  printf '%s\n' "$(cd "$(dirname "${source}")" && pwd)/$(basename "${source}")"
}
