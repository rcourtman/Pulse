# Generated file. Do not edit.
# Bundled on: 2025-11-25T08:34:41Z
# Manifest: scripts/bundle.manifest

# === Begin: scripts/lib/common.sh ===
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
# === End: scripts/lib/common.sh ===

# === Begin: scripts/lib/systemd.sh ===
#!/usr/bin/env bash
#
# Systemd management helpers for Pulse shell scripts.

set -euo pipefail

# Ensure the common library is available when sourced directly.
if ! declare -F common::log_info >/dev/null 2>&1; then
  # shellcheck disable=SC1091
  source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
fi

declare -g SYSTEMD_TIMEOUT_SEC="${SYSTEMD_TIMEOUT_SEC:-20}"

# systemd::safe_systemctl args...
# Executes systemctl with sensible defaults and optional timeout guards.
systemd::safe_systemctl() {
  if ! command -v systemctl >/dev/null 2>&1; then
    common::fail "systemctl not available on this host"
  fi

  local -a cmd
  systemd::__build_cmd cmd "$@"
  systemd::__wrap_timeout cmd

  local label="systemctl $*"
  common::run --label "${label}" -- "${cmd[@]}"
}

# systemd::detect_service_name [service...]
# Returns the first existing service name from the provided list.
systemd::detect_service_name() {
  local -a candidates=("$@")
  if ((${#candidates[@]} == 0)); then
    candidates=(pulse.service pulse-backend.service pulse-docker-agent.service pulse-hot-dev.service)
  fi

  local name
  for name in "${candidates[@]}"; do
    if systemd::service_exists "${name}"; then
      printf '%s\n' "$(systemd::__normalize_unit "${name}")"
      return 0
    fi
  done
  return 1
}

# systemd::service_exists service
# Returns success if the given service unit exists.
systemd::service_exists() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || return 1

  if command -v systemctl >/dev/null 2>&1 && ! common::is_dry_run; then
    local -a cmd
    systemd::__build_cmd cmd "list-unit-files" "${unit}"
    systemd::__wrap_timeout cmd
    local output=""
    if output="$("${cmd[@]}" 2>/dev/null)"; then
      [[ "${output}" =~ ^${unit}[[:space:]] ]] && return 0
    fi
  fi

  local paths=(
    "/etc/systemd/system/${unit}"
    "/lib/systemd/system/${unit}"
    "/usr/lib/systemd/system/${unit}"
  )
  local path
  for path in "${paths[@]}"; do
    if [[ -f "${path}" ]]; then
      return 0
    fi
  done
  return 1
}

# systemd::is_active service
# Checks whether the given service is active.
systemd::is_active() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || return 1

  if common::is_dry_run; then
    return 1
  fi

  if ! command -v systemctl >/dev/null 2>&1; then
    return 1
  fi

  local -a cmd
  systemd::__build_cmd cmd "is-active" "--quiet" "${unit}"
  systemd::__wrap_timeout cmd
  if "${cmd[@]}" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

# systemd::create_service /path/to/unit.service
# Reads unit file content from stdin and writes it to the supplied path.
systemd::create_service() {
  local target="${1:-}"
  local mode="${2:-0644}"
  if [[ -z "${target}" ]]; then
    common::fail "systemd::create_service requires a target path"
  fi

  local content
  content="$(cat)"

  if common::is_dry_run; then
    common::log_info "[dry-run] Would write systemd unit ${target}"
    common::log_debug "${content}"
    return 0
  fi

  mkdir -p "$(dirname "${target}")"
  printf '%s' "${content}" > "${target}"
  chmod "${mode}" "${target}"
  common::log_info "Wrote unit file: ${target}"
}

# systemd::enable_and_start service
# Reloads systemd, enables, and starts the given service.
systemd::enable_and_start() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || common::fail "Invalid systemd unit name"

  systemd::safe_systemctl daemon-reload
  systemd::safe_systemctl enable "${unit}"
  systemd::safe_systemctl start "${unit}"
}

# systemd::restart service
# Safely restarts the given service.
systemd::restart() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || common::fail "Invalid systemd unit name"
  systemd::safe_systemctl restart "${unit}"
}

# Internal: build systemctl command array.
systemd::__build_cmd() {
  local -n ref=$1
  shift
  ref=("systemctl" "--no-ask-password" "--no-pager")
  ref+=("$@")
}

# Internal: wrap command with timeout if necessary.
systemd::__wrap_timeout() {
  local -n ref=$1
  if ! command -v timeout >/dev/null 2>&1; then
    return
  fi
  if systemd::__should_timeout; then
    local -a wrapped=("timeout" "${SYSTEMD_TIMEOUT_SEC}s")
    wrapped+=("${ref[@]}")
    ref=("${wrapped[@]}")
  fi
}

# Internal: determine if we are in a container environment.
systemd::__should_timeout() {
  if [[ -f /run/systemd/container ]]; then
    return 0
  fi
  if command -v systemd-detect-virt >/dev/null 2>&1; then
    if systemd-detect-virt --quiet --container; then
      return 0
    fi
  fi
  return 1
}

# Internal: normalize service names to include .service suffix.
systemd::__normalize_unit() {
  local unit="${1:-}"
  if [[ -z "${unit}" ]]; then
    return 1
  fi
  if [[ "${unit}" != *.service ]]; then
    unit="${unit}.service"
  fi
  printf '%s\n' "${unit}"
}
# === End: scripts/lib/systemd.sh ===

# === Begin: scripts/lib/http.sh ===
#!/usr/bin/env bash
#
# HTTP helpers for Pulse shell scripts (downloads, API calls, GitHub queries).

set -euo pipefail

# Ensure common library is loaded when sourced directly.
if ! declare -F common::log_info >/dev/null 2>&1; then
  # shellcheck disable=SC1091
  source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
fi

declare -g HTTP_DEFAULT_RETRIES="${HTTP_DEFAULT_RETRIES:-3}"
declare -g HTTP_DEFAULT_BACKOFF="${HTTP_DEFAULT_BACKOFF:-1 3 5}"

# http::detect_download_tool
# Emits the preferred download tool (curl/wget) or fails if neither exist.
http::detect_download_tool() {
  if command -v curl >/dev/null 2>&1; then
    printf 'curl\n'
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    printf 'wget\n'
    return 0
  fi
  return 1
}

# http::download --url URL --output PATH [--insecure] [--quiet] [--header "Name: value"]
#               [--retries N] [--backoff "1 3 5"]
# Downloads the specified URL to PATH using curl or wget.
http::download() {
  local url=""
  local output=""
  local insecure=false
  local quiet=false
  local retries="${HTTP_DEFAULT_RETRIES}"
  local backoff="${HTTP_DEFAULT_BACKOFF}"
  local -a headers=()

  while (($#)); do
    case "$1" in
      --url)
        url="$2"
        shift 2
        ;;
      --output)
        output="$2"
        shift 2
        ;;
      --insecure)
        insecure=true
        shift
        ;;
      --quiet)
        quiet=true
        shift
        ;;
      --header)
        headers+=("$2")
        shift 2
        ;;
      --retries)
        retries="$2"
        shift 2
        ;;
      --backoff)
        backoff="$2"
        shift 2
        ;;
      *)
        common::log_warn "Unknown flag for http::download: $1"
        shift
        ;;
    esac
  done

  if [[ -z "${url}" || -z "${output}" ]]; then
    common::fail "http::download requires --url and --output"
  fi

  if common::is_dry_run; then
    common::log_info "[dry-run] Download ${url} -> ${output}"
    return 0
  fi

  local tool
  if ! tool="$(http::detect_download_tool)"; then
    common::fail "No download tool available (install curl or wget)"
  fi

  mkdir -p "$(dirname "${output}")"

  local -a cmd
  if [[ "${tool}" == "curl" ]]; then
    cmd=(curl -fL --connect-timeout 15)
    if [[ "${quiet}" == true ]]; then
      cmd+=(-sS)
    else
      cmd+=(--progress-bar)
    fi
    if [[ "${insecure}" == true ]]; then
      cmd+=(-k)
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(-H "${header}")
    done
    cmd+=(-o "${output}" "${url}")
  else
    cmd=(wget --tries=3)
    if [[ "${quiet}" == true ]]; then
      cmd+=(-q)
    else
      cmd+=(--progress=bar:force)
    fi
    if [[ "${insecure}" == true ]]; then
      cmd+=(--no-check-certificate)
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(--header="${header}")
    done
    cmd+=(-O "${output}" "${url}")
  fi

  local -a run_args=(--label "download ${url}")
  [[ -n "${retries}" ]] && run_args+=(--retries "${retries}")
  [[ -n "${backoff}" ]] && run_args+=(--backoff "${backoff}")

  common::run "${run_args[@]}" -- "${cmd[@]}"
}

# http::api_call --url URL [--method METHOD] [--token TOKEN] [--bearer TOKEN]
#                [--body DATA] [--header "Name: value"] [--insecure]
# Performs an API request and prints the response body.
http::api_call() {
  local url=""
  local method="GET"
  local token=""
  local bearer=""
  local body=""
  local insecure=false
  local -a headers=()

  while (($#)); do
    case "$1" in
      --url)
        url="$2"
        shift 2
        ;;
      --method)
        method="$2"
        shift 2
        ;;
      --token)
        token="$2"
        shift 2
        ;;
      --bearer)
        bearer="$2"
        shift 2
        ;;
      --body)
        body="$2"
        shift 2
        ;;
      --header)
        headers+=("$2")
        shift 2
        ;;
      --insecure)
        insecure=true
        shift
        ;;
      *)
        common::log_warn "Unknown flag for http::api_call: $1"
        shift
        ;;
    esac
  done

  if [[ -z "${url}" ]]; then
    common::fail "http::api_call requires --url"
  fi

  if common::is_dry_run; then
    common::log_info "[dry-run] API ${method} ${url}"
    return 0
  fi

  local tool
  if ! tool="$(http::detect_download_tool)"; then
    common::fail "No HTTP client available (install curl or wget)"
  fi

  local -a cmd
  if [[ "${tool}" == "curl" ]]; then
    cmd=(curl -fsSL)
    [[ "${insecure}" == true ]] && cmd+=(-k)
    [[ -n "${method}" ]] && cmd+=(-X "${method}")
    if [[ -n "${body}" ]]; then
      cmd+=(-d "${body}")
    fi
    if [[ -n "${token}" ]]; then
      headers+=("X-API-Token: ${token}")
    fi
    if [[ -n "${bearer}" ]]; then
      headers+=("Authorization: Bearer ${bearer}")
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(-H "${header}")
    done
    cmd+=("${url}")
  else
    cmd=(wget -qO-)
    [[ "${insecure}" == true ]] && cmd+=(--no-check-certificate)
    [[ -n "${method}" ]] && cmd+=(--method="${method}")
    if [[ -n "${body}" ]]; then
      cmd+=(--body-data="${body}")
    fi
    if [[ -n "${token}" ]]; then
      cmd+=(--header="X-API-Token: ${token}")
    fi
    if [[ -n "${bearer}" ]]; then
      cmd+=(--header="Authorization: Bearer ${bearer}")
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(--header="${header}")
    done
    cmd+=("${url}")
  fi

  common::run_capture --label "api ${method} ${url}" -- "${cmd[@]}"
}

# http::get_github_latest_release owner/repo
# Echoes the latest release tag for a GitHub repository.
http::get_github_latest_release() {
  local repo="${1:-}"
  if [[ -z "${repo}" ]]; then
    common::fail "http::get_github_latest_release requires owner/repo argument"
  fi

  local response
  response="$(http::api_call --url "https://api.github.com/repos/${repo}/releases/latest" --header "Accept: application/vnd.github+json" 2>/dev/null || true)"
  if [[ -z "${response}" ]]; then
    return 1
  fi

  if [[ "${response}" =~ \"tag_name\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  if [[ "${response}" =~ \"name\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  common::log_warn "Unable to parse GitHub release tag for ${repo}"
  return 1
}

# http::parse_bool value
# Parses truthy/falsy strings and prints canonical true/false.
http::parse_bool() {
  local input="${1:-}"
  local lowered="${input,,}"
  case "${lowered}" in
    1|true|yes|y|on)
      printf 'true\n'
      return 0
      ;;
    0|false|no|n|off|"")
      printf 'false\n'
      return 0
      ;;
  esac
  return 1
}
# === End: scripts/lib/http.sh ===

# === Begin: scripts/install-docker-agent-v2.sh ===
#!/usr/bin/env bash
#
# Pulse Docker Agent Installer/Uninstaller (v2)
# Refactored to leverage shared script libraries.

set -euo pipefail

LIB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${LIB_ROOT}/lib"
if [[ -f "${LIB_DIR}/common.sh" ]]; then
  # shellcheck disable=SC1090
  source "${LIB_DIR}/common.sh"
  # shellcheck disable=SC1090
  source "${LIB_DIR}/systemd.sh"
  # shellcheck disable=SC1090
  source "${LIB_DIR}/http.sh"
fi

common::init "$@"

log_info() {
    common::log_info "$1"
}

log_warn() {
    common::log_warn "$1"
}

log_error() {
    common::log_error "$1"
}

log_success() {
    common::log_info "[ OK ] $1"
}

trim() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "$value"
}

determine_agent_identifier() {
    local agent_id=""

    if command -v docker &> /dev/null; then
        agent_id=$(docker info --format '{{.ID}}' 2>/dev/null | head -n1 | tr -d '[:space:]')
    fi

    if [[ -z "$agent_id" ]] && [[ -r /etc/machine-id ]]; then
        agent_id=$(tr -d '[:space:]' < /etc/machine-id)
    fi

    if [[ -z "$agent_id" ]]; then
        agent_id=$(hostname 2>/dev/null | tr -d '[:space:]')
    fi

    printf '%s' "$agent_id"
}

log_header() {
    printf '\n== %s ==\n' "$1"
}

quote_shell_arg() {
    local value="$1"
    value=${value//\'/\'\\\'\'}
    printf "'%s'" "$value"
}

parse_bool() {
    local result
    if result="$(http::parse_bool "${1:-}")"; then
        PARSED_BOOL="$result"
        return 0
    fi
    return 1
}

parse_target_spec() {
    local spec="$1"
    local raw_url raw_token raw_insecure

    IFS='|' read -r raw_url raw_token raw_insecure <<< "$spec"
    raw_url=$(trim "$raw_url")
    raw_token=$(trim "$raw_token")
    raw_insecure=$(trim "$raw_insecure")

    if [[ -z "$raw_url" || -z "$raw_token" ]]; then
        echo "Error: invalid target spec \"$spec\". Expected format url|token[|insecure]." >&2
        return 1
    fi

    PARSED_TARGET_URL="${raw_url%/}"
    PARSED_TARGET_TOKEN="$raw_token"

    if [[ -n "$raw_insecure" ]]; then
        if ! parse_bool "$raw_insecure"; then
            echo "Error: invalid insecure flag \"$raw_insecure\" in target spec \"$spec\"." >&2
            return 1
        fi
        PARSED_TARGET_INSECURE="$PARSED_BOOL"
    else
        PARSED_TARGET_INSECURE="false"
    fi

    return 0
}

split_targets_from_env() {
    local value="$1"
    if [[ -z "$value" ]]; then
        return 0
    fi

    value="${value//$'\n'/;}"
    IFS=';' read -ra __env_targets <<< "$value"
    for entry in "${__env_targets[@]}"; do
        local trimmed
        trimmed=$(trim "$entry")
        if [[ -n "$trimmed" ]]; then
            printf '%s\n' "$trimmed"
        fi
    done
}

# Early runtime detection to hand off Podman installs to the container-aware script.
ORIGINAL_ARGS=("$@")
DETECTED_RUNTIME="${PULSE_RUNTIME:-}"
if [[ -z "$DETECTED_RUNTIME" ]]; then
    idx=0
    total_args=${#ORIGINAL_ARGS[@]}
    while [[ $idx -lt $total_args ]]; do
        arg="${ORIGINAL_ARGS[$idx]}"
        case "$arg" in
            --runtime)
                if (( idx + 1 < total_args )); then
                    DETECTED_RUNTIME="${ORIGINAL_ARGS[$((idx + 1))]}"
                fi
                ((idx += 2))
                continue
                ;;
            --runtime=*)
                DETECTED_RUNTIME="${arg#--runtime=}"
                ;;
        esac
        ((idx += 1))
    done
    unset total_args
fi

if [[ -n "$DETECTED_RUNTIME" ]]; then
    runtime_lower=$(printf '%s' "$DETECTED_RUNTIME" | tr '[:upper:]' '[:lower:]')
    if [[ "$runtime_lower" == "podman" ]]; then
        SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
        if [[ -f "${SCRIPT_DIR}/install-container-agent.sh" ]]; then
            exec "${SCRIPT_DIR}/install-container-agent.sh" "${ORIGINAL_ARGS[@]}"
        fi
        common::log_error "Podman runtime requested but install-container-agent.sh not found."
        exit 1
    fi
fi

extract_targets_from_service() {
    local file="$1"
    [[ ! -f "$file" ]] && return

    local line value

    # Prefer explicit multi-target configuration if present
    line=$(grep -m1 'PULSE_TARGETS=' "$file" 2>/dev/null || true)
    if [[ -n "$line" ]]; then
        value=$(printf '%s\n' "$line" | sed -n 's/.*PULSE_TARGETS=\([^"]*\).*/\1/p')
        if [[ -n "$value" ]]; then
            IFS=';' read -ra __service_targets <<< "$value"
            for entry in "${__service_targets[@]}"; do
                entry=$(trim "$entry")
                if [[ -n "$entry" ]]; then
                    printf '%s\n' "$entry"
                fi
            done
        fi
        return
    fi

    local url=""
    local token=""
    local insecure="false"

    line=$(grep -m1 'PULSE_URL=' "$file" 2>/dev/null || true)
    if [[ -n "$line" ]]; then
        value="${line#*PULSE_URL=}"
        value="${value%\"*}"
        url=$(trim "$value")
    fi

    line=$(grep -m1 'PULSE_TOKEN=' "$file" 2>/dev/null || true)
    if [[ -n "$line" ]]; then
        value="${line#*PULSE_TOKEN=}"
        value="${value%\"*}"
        token=$(trim "$value")
    fi

    line=$(grep -m1 'PULSE_INSECURE_SKIP_VERIFY=' "$file" 2>/dev/null || true)
    if [[ -n "$line" ]]; then
        value="${line#*PULSE_INSECURE_SKIP_VERIFY=}"
        value="${value%\"*}"
        if parse_bool "$value"; then
            insecure="$PARSED_BOOL"
        fi
    fi

    local exec_line
    exec_line=$(grep -m1 '^ExecStart=' "$file" 2>/dev/null || true)
    if [[ -n "$exec_line" ]]; then
        if [[ -z "$url" ]]; then
            if [[ "$exec_line" =~ --url[[:space:]]+\"([^\"]+)\" ]]; then
                url="${BASH_REMATCH[1]}"
            elif [[ "$exec_line" =~ --url[[:space:]]+([^[:space:]]+) ]]; then
                url="${BASH_REMATCH[1]}"
            fi
        fi
        if [[ -z "$token" ]]; then
            if [[ "$exec_line" =~ --token[[:space:]]+\"([^\"]+)\" ]]; then
                token="${BASH_REMATCH[1]}"
            elif [[ "$exec_line" =~ --token[[:space:]]+([^[:space:]]+) ]]; then
                token="${BASH_REMATCH[1]}"
            fi
        fi
        if [[ "$insecure" != "true" && "$exec_line" == *"--insecure"* ]]; then
            insecure="true"
        fi
    fi

    url=$(trim "$url")
    token=$(trim "$token")

    if [[ -n "$url" && -n "$token" ]]; then
        printf '%s|%s|%s\n' "$url" "$token" "$insecure"
    fi
}

detect_agent_path_from_service() {
    if [[ -n "$SERVICE_PATH" && -f "$SERVICE_PATH" ]]; then
        local exec_line
        exec_line=$(grep -m1 '^ExecStart=' "$SERVICE_PATH" 2>/dev/null || true)
        if [[ -n "$exec_line" ]]; then
            local value="${exec_line#ExecStart=}"
            value=$(trim "$value")
            if [[ -n "$value" ]]; then
                printf '%s' "${value%%[[:space:]]*}"
                return
            fi
        fi
    fi
}

detect_agent_path_from_unraid() {
    if [[ -n "$UNRAID_STARTUP" && -f "$UNRAID_STARTUP" ]]; then
        local match
        match=$(grep -m1 -o '/[^[:space:]]*pulse-docker-agent' "$UNRAID_STARTUP" 2>/dev/null || true)
        if [[ -n "$match" ]]; then
            printf '%s' "$match"
            return
        fi
    fi
}

detect_existing_agent_path() {
    local path

    path=$(detect_agent_path_from_service)
    if [[ -n "$path" ]]; then
        printf '%s' "$path"
        return
    fi

    path=$(detect_agent_path_from_unraid)
    if [[ -n "$path" ]]; then
        printf '%s' "$path"
        return
    fi

    if command -v pulse-docker-agent >/dev/null 2>&1; then
        path=$(command -v pulse-docker-agent)
        if [[ -n "$path" ]]; then
            printf '%s' "$path"
            return
        fi
    fi
}

ensure_agent_path_writable() {
    local file_path="$1"
    local dir="${file_path%/*}"

    if [[ -z "$dir" || "$file_path" != /* ]]; then
        return 1
    fi

    if [[ ! -d "$dir" ]]; then
        if ! mkdir -p "$dir" 2>/dev/null; then
            return 1
        fi
    fi

    local test_file="$dir/.pulse-agent-write-test-$$"
    if ! touch "$test_file" 2>/dev/null; then
        return 1
    fi
    rm -f "$test_file" 2>/dev/null || true
    return 0
}

select_agent_path_for_install() {
    local candidates=()
    declare -A seen=()
    local selected=""
    local default_attempted="false"
    local default_failed="false"

    if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
        candidates+=("$AGENT_PATH_OVERRIDE")
    fi

    if [[ -n "$EXISTING_AGENT_PATH" ]]; then
        candidates+=("$EXISTING_AGENT_PATH")
    fi

    candidates+=("$DEFAULT_AGENT_PATH")
    for fallback in "${AGENT_FALLBACK_PATHS[@]}"; do
        candidates+=("$fallback")
    done

    for candidate in "${candidates[@]}"; do
        candidate=$(trim "$candidate")
        if [[ -z "$candidate" || "$candidate" != /* ]]; then
            continue
        fi
        if [[ -n "${seen[$candidate]:-}" ]]; then
            continue
        fi
        seen["$candidate"]=1

        if [[ "$candidate" == "$DEFAULT_AGENT_PATH" ]]; then
            default_attempted="true"
        fi

        if ensure_agent_path_writable "$candidate"; then
            selected="$candidate"
            if [[ "$candidate" == "$DEFAULT_AGENT_PATH" ]]; then
                DEFAULT_AGENT_PATH_WRITABLE="true"
            else
                if [[ "$default_attempted" == "true" && "$default_failed" == "true" && "$OVERRIDE_SPECIFIED" == "false" ]]; then
                    AGENT_PATH_NOTE="Note: Detected that $DEFAULT_AGENT_PATH is not writable. Using fallback path: $candidate"
                fi
            fi
            break
        else
            if [[ "$candidate" == "$DEFAULT_AGENT_PATH" ]]; then
                default_failed="true"
                DEFAULT_AGENT_PATH_WRITABLE="false"
            fi
        fi
    done

    if [[ -z "$selected" ]]; then
        echo "Error: Could not find a writable location for the agent binary." >&2
        if [[ "$OVERRIDE_SPECIFIED" == "true" ]]; then
            echo "Provided agent path: $AGENT_PATH_OVERRIDE" >&2
        fi
        exit 1
    fi

    printf '%s' "$selected"
}

resolve_agent_path_for_uninstall() {
    if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
        printf '%s' "$AGENT_PATH_OVERRIDE"
        return
    fi

    local existing_path
    existing_path=$(detect_existing_agent_path)
    if [[ -n "$existing_path" ]]; then
        printf '%s' "$existing_path"
        return
    fi

    printf '%s' "$DEFAULT_AGENT_PATH"
}

# Pulse Docker Agent Installer/Uninstaller
# Install (single target):
#   curl -fSL http://pulse.example.com/install-docker-agent.sh -o /tmp/pulse-install-docker-agent.sh && \
#     sudo bash /tmp/pulse-install-docker-agent.sh --url http://pulse.example.com --token <api-token> && \
#     rm -f /tmp/pulse-install-docker-agent.sh
# Install (multi-target fan-out):
#   curl -fSL http://pulse.example.com/install-docker-agent.sh -o /tmp/pulse-install-docker-agent.sh && \
#     sudo bash /tmp/pulse-install-docker-agent.sh -- \
#       --target https://pulse.example.com|<api-token> \
#       --target https://pulse-dr.example.com|<api-token> && \
#     rm -f /tmp/pulse-install-docker-agent.sh
# Uninstall:
#   curl -fSL http://pulse.example.com/install-docker-agent.sh -o /tmp/pulse-install-docker-agent.sh && \
#     sudo bash /tmp/pulse-install-docker-agent.sh --uninstall && \
#     rm -f /tmp/pulse-install-docker-agent.sh

PULSE_URL=""
DEFAULT_AGENT_PATH="/usr/local/bin/pulse-docker-agent"
AGENT_FALLBACK_PATHS=(
    "/opt/pulse/bin/pulse-docker-agent"
    "/opt/bin/pulse-docker-agent"
    "/var/lib/pulse/bin/pulse-docker-agent"
)
AGENT_PATH_OVERRIDE="${PULSE_AGENT_PATH:-}"
OVERRIDE_SPECIFIED="false"
if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
    OVERRIDE_SPECIFIED="true"
fi
AGENT_PATH_NOTE=""
DEFAULT_AGENT_PATH_WRITABLE="unknown"
EXISTING_AGENT_PATH=""
AGENT_PATH=""
SERVICE_PATH="/etc/systemd/system/pulse-docker-agent.service"
UNRAID_STARTUP="/boot/config/go.d/pulse-docker-agent.sh"
LOG_PATH="/var/log/pulse-docker-agent.log"
INTERVAL="30s"
UNINSTALL=false
TOKEN="${PULSE_TOKEN:-}"
DOWNLOAD_ARCH=""
TARGET_SPECS=()
PULSE_TARGETS_ENV="${PULSE_TARGETS:-}"
DEFAULT_INSECURE="$(trim "${PULSE_INSECURE_SKIP_VERIFY:-}")"
PRIMARY_URL=""
PRIMARY_TOKEN=""
PRIMARY_INSECURE="false"
JOINED_TARGETS=""
ORIGINAL_ARGS=("$@")

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --url)
      PULSE_URL="$2"
      shift 2
      ;;
    --interval)
      INTERVAL="$2"
      shift 2
      ;;
    --uninstall)
      UNINSTALL=true
      shift
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --target)
      TARGET_SPECS+=("$2")
      shift 2
      ;;
    --agent-path)
      AGENT_PATH_OVERRIDE="$2"
      OVERRIDE_SPECIFIED="true"
      shift 2
      ;;
    --dry-run)
      common::set_dry_run true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 --url <Pulse URL> --token <API token> [--interval 30s]"
      echo "       $0 --agent-path /custom/path/pulse-docker-agent"
      echo "       $0 --uninstall"
      exit 1
      ;;
  esac
done

# Normalize PULSE_URL - strip trailing slashes to prevent double-slash issues
PULSE_URL="${PULSE_URL%/}"

if ! common::is_dry_run; then
    common::ensure_root --allow-sudo --args "${ORIGINAL_ARGS[@]}"
else
    common::log_debug "Skipping privilege escalation due to dry-run mode"
fi

AGENT_PATH_OVERRIDE=$(trim "$AGENT_PATH_OVERRIDE")
if [[ -z "$AGENT_PATH_OVERRIDE" ]]; then
    OVERRIDE_SPECIFIED="false"
fi

if [[ -n "$AGENT_PATH_OVERRIDE" && "$AGENT_PATH_OVERRIDE" != /* ]]; then
    echo "Error: --agent-path must be an absolute path." >&2
    exit 1
fi

EXISTING_AGENT_PATH=$(detect_existing_agent_path)

if [[ "$UNINSTALL" = true ]]; then
    AGENT_PATH=$(resolve_agent_path_for_uninstall)
else
    AGENT_PATH=$(select_agent_path_for_install)
fi

# Handle uninstall
if [ "$UNINSTALL" = true ]; then
    echo "==================================="
    echo "Pulse Docker Agent Uninstaller"
    echo "==================================="
    echo ""

    # Stop and disable systemd service
    if command -v systemctl &> /dev/null && [ -f "$SERVICE_PATH" ]; then
        echo "Stopping systemd service..."
        systemctl stop pulse-docker-agent 2>/dev/null || true
        systemctl disable pulse-docker-agent 2>/dev/null || true
        rm -f "$SERVICE_PATH"
        systemctl daemon-reload
        echo "✓ Systemd service removed"
    fi

    # Stop running agent process
    if pgrep -f pulse-docker-agent > /dev/null; then
        echo "Stopping agent process..."
        pkill -f pulse-docker-agent || true
        sleep 1
        echo "✓ Agent process stopped"
    fi

    # Remove binary
    if [ -f "$AGENT_PATH" ]; then
        rm -f "$AGENT_PATH"
        echo "✓ Agent binary removed"
    fi

    # Remove Unraid startup script
    if [ -f "$UNRAID_STARTUP" ]; then
        rm -f "$UNRAID_STARTUP"
        echo "✓ Unraid startup script removed"
    fi

    # Remove log file
    if [ -f "$LOG_PATH" ]; then
        rm -f "$LOG_PATH"
        echo "✓ Log file removed"
    fi

    echo ""
    echo "==================================="
    echo "✓ Uninstall complete!"
    echo "==================================="
    echo ""
    echo "The Pulse Docker agent has been removed from this system."
    echo ""
    exit 0
fi

# Validate target configuration for install
if [[ "$UNINSTALL" != true ]]; then
  declare -a RAW_TARGETS=()

  if [[ ${#TARGET_SPECS[@]} -gt 0 ]]; then
    RAW_TARGETS+=("${TARGET_SPECS[@]}")
  fi

  if [[ -n "$PULSE_TARGETS_ENV" ]]; then
    mapfile -t ENV_TARGETS < <(split_targets_from_env "$PULSE_TARGETS_ENV")
    if [[ ${#ENV_TARGETS[@]} -gt 0 ]]; then
      RAW_TARGETS+=("${ENV_TARGETS[@]}")
    fi
  fi

  TOKEN=$(trim "$TOKEN")
  PULSE_URL=$(trim "$PULSE_URL")

  if [[ ${#RAW_TARGETS[@]} -eq 0 ]]; then
    if [[ -z "$PULSE_URL" || -z "$TOKEN" ]]; then
      echo "Error: Provide --target / PULSE_TARGETS or legacy --url and --token values."
      echo ""
      echo "Usage:"
      echo "  Install:   $0 --target https://pulse.example.com|<api-token> [--target ...] [--interval 30s]"
      echo "  Legacy:    $0 --url http://pulse.example.com --token <api-token> [--interval 30s]"
      echo "  Uninstall: $0 --uninstall"
      exit 1
    fi

    if [[ -n "$DEFAULT_INSECURE" ]]; then
      if ! parse_bool "$DEFAULT_INSECURE"; then
        echo "Error: invalid PULSE_INSECURE_SKIP_VERIFY value \"$DEFAULT_INSECURE\"." >&2
        exit 1
      fi
      PRIMARY_INSECURE="$PARSED_BOOL"
    else
      PRIMARY_INSECURE="false"
    fi

    RAW_TARGETS+=("${PULSE_URL%/}|$TOKEN|$PRIMARY_INSECURE")
  fi

  if [[ -f "$SERVICE_PATH" && ${#RAW_TARGETS[@]} -eq 0 ]]; then
    mapfile -t EXISTING_TARGETS < <(extract_targets_from_service "$SERVICE_PATH")
    if [[ ${#EXISTING_TARGETS[@]} -gt 0 ]]; then
      RAW_TARGETS+=("${EXISTING_TARGETS[@]}")
    fi
  fi

  declare -A SEEN_TARGETS=()
  TARGETS=()

  for spec in "${RAW_TARGETS[@]}"; do
    if ! parse_target_spec "$spec"; then
      exit 1
    fi

    local_normalized="${PARSED_TARGET_URL}|${PARSED_TARGET_TOKEN}|${PARSED_TARGET_INSECURE}"

    if [[ -z "$PRIMARY_URL" ]]; then
      PRIMARY_URL="$PARSED_TARGET_URL"
      PRIMARY_TOKEN="$PARSED_TARGET_TOKEN"
      PRIMARY_INSECURE="$PARSED_TARGET_INSECURE"
    fi

    if [[ -n "${SEEN_TARGETS[$local_normalized]:-}" ]]; then
      continue
    fi

    SEEN_TARGETS[$local_normalized]=1
    TARGETS+=("$local_normalized")
  done

  if [[ ${#TARGETS[@]} -eq 0 ]]; then
    echo "Error: no valid Pulse targets provided." >&2
    exit 1
  fi

  JOINED_TARGETS=$(printf "%s;" "${TARGETS[@]}")
  JOINED_TARGETS="${JOINED_TARGETS%;}"

  # Backwards compatibility for older agent versions
  PULSE_URL="$PRIMARY_URL"
  TOKEN="$PRIMARY_TOKEN"
fi

log_header "Pulse Docker Agent Installer"
if [[ "$UNINSTALL" != true ]]; then
  AGENT_IDENTIFIER=$(determine_agent_identifier)
else
  AGENT_IDENTIFIER=""
fi
if [[ -n "$AGENT_PATH_NOTE" ]]; then
  log_warn "$AGENT_PATH_NOTE"
fi
log_info "Primary Pulse URL : $PRIMARY_URL"
if [[ ${#TARGETS[@]} -gt 1 ]]; then
  log_info "Additional targets : $(( ${#TARGETS[@]} - 1 ))"
fi
log_info "Install path      : $AGENT_PATH"
log_info "Log directory     : /var/log/pulse-docker-agent"
log_info "Reporting interval: $INTERVAL"
if [[ "$UNINSTALL" != true ]]; then
  log_info "API token         : provided"
  if [[ -n "$AGENT_IDENTIFIER" ]]; then
    log_info "Docker host ID    : $AGENT_IDENTIFIER"
  fi
  log_info "Targets:" 
  for spec in "${TARGETS[@]}"; do
    IFS='|' read -r target_url _ target_insecure <<< "$spec"
    if [[ "$target_insecure" == "true" ]]; then
      log_info "  • $target_url (skip TLS verify)"
    else
      log_info "  • $target_url"
    fi
  done
fi
printf '\n'

# Detect architecture for download
if [[ "$UNINSTALL" != true ]]; then
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64|amd64)
      DOWNLOAD_ARCH="linux-amd64"
      ;;
    aarch64|arm64)
      DOWNLOAD_ARCH="linux-arm64"
      ;;
    armv7l|armhf|armv7)
      DOWNLOAD_ARCH="linux-armv7"
      ;;
    *)
      DOWNLOAD_ARCH=""
      log_warn "Unknown architecture '$ARCH'. Falling back to default agent binary."
      ;;
  esac
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    log_warn 'Docker not found. The agent requires Docker to be installed.'
    if common::is_dry_run; then
        log_warn 'Dry-run mode: skipping Docker enforcement.'
    else
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

if [[ "$UNINSTALL" != true ]] && command -v systemctl >/dev/null 2>&1; then
    if systemd::service_exists "pulse-docker-agent"; then
        if systemd::is_active "pulse-docker-agent"; then
            systemd::safe_systemctl stop "pulse-docker-agent"
        fi
    fi
fi

# Download agent binary
log_info "Downloading agent binary"
DOWNLOAD_URL_BASE="$PRIMARY_URL/download/pulse-docker-agent"
DOWNLOAD_URL="$DOWNLOAD_URL_BASE"
if [[ -n "$DOWNLOAD_ARCH" ]]; then
    DOWNLOAD_URL="$DOWNLOAD_URL?arch=$DOWNLOAD_ARCH"
fi

download_agent_binary() {
    local primary_url="$1"
    local fallback_url="$2"
    local -a download_args=(--url "${primary_url}" --output "${AGENT_PATH}" --retries 3 --backoff "1 3 5")
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        download_args+=(--insecure)
    fi

    if http::download "${download_args[@]}"; then
        AGENT_DOWNLOAD_SOURCE="${primary_url}"
        return 0
    fi

    if ! common::is_dry_run; then
        rm -f "$AGENT_PATH" 2>/dev/null || true
    fi

    if [[ "${fallback_url}" != "${primary_url}" && -n "${fallback_url}" ]]; then
        log_info 'Falling back to server default agent binary'
        download_args=(--url "${fallback_url}" --output "${AGENT_PATH}" --retries 3 --backoff "1 3 5")
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            download_args+=(--insecure)
        fi
        if http::download "${download_args[@]}"; then
            AGENT_DOWNLOAD_SOURCE="${fallback_url}"
            return 0
        fi

        if ! common::is_dry_run; then
            rm -f "$AGENT_PATH" 2>/dev/null || true
        fi
    fi

    return 1
}

unset AGENT_DOWNLOAD_SOURCE

if download_agent_binary "$DOWNLOAD_URL" "$DOWNLOAD_URL_BASE"; then
    :
else
    log_warn 'Failed to download agent binary'
    log_warn "Ensure the Pulse server is reachable at $PRIMARY_URL"
    exit 1
fi

fetch_checksum_header() {
    local url="$1"
    local header=""

    if command -v curl &> /dev/null; then
        local curl_args=(-fsSI "$url")
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            curl_args=(-k "${curl_args[@]}")
        fi
        header=$(curl "${curl_args[@]}" 2>/dev/null || true)
    elif command -v wget &> /dev/null; then
        local tmp
        tmp=$(mktemp)
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            wget --spider --no-check-certificate --server-response "$url" >/dev/null 2>"$tmp" || true
        else
            wget --spider --server-response "$url" >/dev/null 2>"$tmp" || true
        fi
        header=$(cat "$tmp" 2>/dev/null || true)
        rm -f "$tmp"
    fi

    if [[ -z "$header" ]]; then
        return 1
    fi

    local checksum_line
    checksum_line=$(printf '%s\n' "$header" | awk 'BEGIN{IGNORECASE=1} /^ *X-Checksum-Sha256:/{print $0; exit}')
    if [[ -z "$checksum_line" ]]; then
        return 1
    fi

    local value
    value=$(printf '%s\n' "$checksum_line" | awk -F':' '{sub(/^[[:space:]]*/,"",$2); sub(/[[:space:]]*$/,"",$2); print $2}')
    value=$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')
    # Extra trim to be safe
    value=$(trim "$value")

    if [[ -z "$value" ]]; then
        return 1
    fi

    FETCHED_CHECKSUM="$value"
    return 0
}

calculate_sha256() {
    local file="$1"
    local hash=""

    if command -v sha256sum &> /dev/null; then
        hash=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        hash=$(shasum -a 256 "$file" | awk '{print $1}')
    fi

    if [[ -z "$hash" ]]; then
        return 1
    fi

    CALCULATED_CHECKSUM=$(printf '%s' "$hash" | tr '[:upper:]' '[:lower:]')
    return 0
}

verify_agent_checksum() {
    local url="$1"

    if common::is_dry_run; then
        log_info '[dry-run] Skipping checksum verification'
        return 0
    fi

    if ! fetch_checksum_header "$url"; then
        log_warn 'Agent download did not include X-Checksum-Sha256 header; skipping verification'
        return 0
    fi

    if ! calculate_sha256 "$AGENT_PATH"; then
        log_warn 'Unable to calculate sha256 checksum locally; skipping verification'
        return 0
    fi

    local clean_fetched
    clean_fetched=$(echo "$FETCHED_CHECKSUM" | tr -d '[:space:]')
    local clean_calculated
    clean_calculated=$(echo "$CALCULATED_CHECKSUM" | tr -d '[:space:]')

    if [[ "$clean_fetched" != "$clean_calculated" ]]; then
        rm -f "$AGENT_PATH"
        log_error "Checksum mismatch."
        log_error "  Expected: '$clean_fetched' (from header)"
        log_error "  Actual:   '$clean_calculated' (calculated)"
        return 1
    fi

    log_success 'Checksum verified for agent binary'
    unset FETCHED_CHECKSUM CALCULATED_CHECKSUM
    return 0
}

if [[ -n "${AGENT_DOWNLOAD_SOURCE:-}" ]]; then
    if ! verify_agent_checksum "$AGENT_DOWNLOAD_SOURCE"; then
        log_error 'Agent download failed checksum verification'
        exit 1
    fi
fi

if ! common::is_dry_run; then
    chmod 0755 "$AGENT_PATH"
fi
log_success "Agent binary installed"

allow_reenroll_if_needed() {
    local host_id="$1"
    if [[ -z "$host_id" || -z "$PRIMARY_TOKEN" || -z "$PRIMARY_URL" ]]; then
        return 0
    fi

    local endpoint="$PRIMARY_URL/api/agents/docker/hosts/${host_id}/allow-reenroll"
    local -a api_args=(--url "${endpoint}" --method POST --token "${PRIMARY_TOKEN}")
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        api_args+=(--insecure)
    fi

    if http::api_call "${api_args[@]}" >/dev/null 2>&1; then
        log_success "Cleared any previous stop block for host"
    else
        log_warn "Unable to confirm removal block clearance (continuing)"
    fi

    return 0
}

allow_reenroll_if_needed "$AGENT_IDENTIFIER"

# Check if systemd is available
if ! command -v systemctl &> /dev/null || [ ! -d /etc/systemd/system ]; then
    printf '\n%s\n' '-- Systemd not detected; configuring alternative startup --'

    # Check if this is Unraid (has /boot/config directory)
    if [ -d /boot/config ]; then
        log_info 'Detected Unraid environment'

        mkdir -p /boot/config/go.d
        STARTUP_SCRIPT="/boot/config/go.d/pulse-docker-agent.sh"
cat > "$STARTUP_SCRIPT" <<EOF
#!/bin/bash
# Pulse Docker Agent - Auto-start script
sleep 10  # Wait for Docker to be ready
PULSE_URL="$PRIMARY_URL" PULSE_TOKEN="$PRIMARY_TOKEN" PULSE_TARGETS="$JOINED_TARGETS" PULSE_INSECURE_SKIP_VERIFY="$PRIMARY_INSECURE" $AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"$NO_AUTO_UPDATE_FLAG > /var/log/pulse-docker-agent.log 2>&1 &
EOF

        chmod +x "$STARTUP_SCRIPT"
        log_success "Created startup script: $STARTUP_SCRIPT"

        log_info 'Starting agent'
        PULSE_URL="$PRIMARY_URL" PULSE_TOKEN="$PRIMARY_TOKEN" PULSE_TARGETS="$JOINED_TARGETS" PULSE_INSECURE_SKIP_VERIFY="$PRIMARY_INSECURE" $AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"$NO_AUTO_UPDATE_FLAG > /var/log/pulse-docker-agent.log 2>&1 &

        log_header 'Installation complete'
        log_info 'Agent started via Unraid go.d hook'
        log_info 'Log file             : /var/log/pulse-docker-agent.log'
        log_info 'Host visible in Pulse: ~30 seconds'
        exit 0
    fi

    log_info 'Manual startup environment detected'
    log_info "Binary location      : $AGENT_PATH"
    log_info 'Start manually with  :'
    printf '  PULSE_URL=%s PULSE_TOKEN=<api-token> \\n' "$PRIMARY_URL"
    printf '  PULSE_TARGETS="%s" \\n' "https://pulse.example.com|<token>[;https://pulse-alt.example.com|<token2>]"
    printf '  %s --interval %s &
' "$AGENT_PATH" "$INTERVAL"
    log_info 'Add the same command to your init system to start automatically.'
    exit 0

fi


# Check if server is in development mode
NO_AUTO_UPDATE_FLAG=""
if http::detect_download_tool >/dev/null 2>&1; then
    SERVER_INFO_URL="$PRIMARY_URL/api/server/info"
    IS_DEV="false"

    SERVER_INFO=""
    declare -a __server_info_args=(--url "$SERVER_INFO_URL")
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        __server_info_args+=(--insecure)
    fi
    SERVER_INFO="$(http::api_call "${__server_info_args[@]}" 2>/dev/null || true)"
    unset __server_info_args

if [[ -n "$SERVER_INFO" ]] && echo "$SERVER_INFO" | grep -q '"isDevelopment"[[:space:]]*:[[:space:]]*true'; then
    IS_DEV="true"
    NO_AUTO_UPDATE_FLAG=" --no-auto-update"
    log_info 'Development server detected – auto-update disabled'
fi

if [[ -n "$NO_AUTO_UPDATE_FLAG" ]]; then
    if ! "$AGENT_PATH" --help 2>&1 | grep -q -- '--no-auto-update'; then
        log_warn 'Agent binary lacks --no-auto-update flag; keeping auto-update enabled'
        NO_AUTO_UPDATE_FLAG=""
    fi
fi
fi

# Create systemd service
log_header 'Configuring systemd service'
SYSTEMD_ENV_TARGETS_LINE=""
if [[ -n "$JOINED_TARGETS" ]]; then
SYSTEMD_ENV_TARGETS_LINE="Environment=\"PULSE_TARGETS=$JOINED_TARGETS\""
fi
SYSTEMD_ENV_URL_LINE="Environment=\"PULSE_URL=$PRIMARY_URL\""
SYSTEMD_ENV_TOKEN_LINE="Environment=\"PULSE_TOKEN=$PRIMARY_TOKEN\""
SYSTEMD_ENV_INSECURE_LINE="Environment=\"PULSE_INSECURE_SKIP_VERIFY=$PRIMARY_INSECURE\""
systemd::create_service "$SERVICE_PATH" <<EOF
[Unit]
Description=Pulse Docker Agent
After=network-online.target docker.socket docker.service
Wants=network-online.target docker.socket

[Service]
Type=simple
$SYSTEMD_ENV_URL_LINE
$SYSTEMD_ENV_TOKEN_LINE
$SYSTEMD_ENV_TARGETS_LINE
$SYSTEMD_ENV_INSECURE_LINE
ExecStart=$AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"$NO_AUTO_UPDATE_FLAG
Restart=on-failure
RestartSec=5s
User=root
ProtectSystem=full
ProtectHome=read-only
ProtectControlGroups=yes
ProtectKernelModules=yes
ProtectKernelTunables=yes
ProtectKernelLogs=yes
UMask=0077
NoNewPrivileges=yes
RestrictSUIDSGID=yes
RestrictRealtime=yes
PrivateTmp=yes
MemoryDenyWriteExecute=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
ReadWritePaths=/var/run/docker.sock
ProtectHostname=yes
ProtectClock=yes

[Install]
WantedBy=multi-user.target
EOF

log_success "Wrote unit file: $SERVICE_PATH"

# Reload systemd and start service
log_info 'Starting service'
systemd::enable_and_start "pulse-docker-agent"

log_header 'Installation complete'
log_info 'Agent service enabled and started'
log_info 'Check status          : systemctl status pulse-docker-agent'
log_info 'Follow logs           : journalctl -u pulse-docker-agent -f'
log_info 'Host visible in Pulse : ~30 seconds'
# === End: scripts/install-docker-agent-v2.sh ===

