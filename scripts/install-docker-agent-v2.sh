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
    value=$(printf '%s\n' "$checksum_line" | awk -F':' '{sub(/^[[:space:]]*/,"",$2); print $2}')
    value=$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')
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

    if [[ "$FETCHED_CHECKSUM" != "$CALCULATED_CHECKSUM" ]]; then
        rm -f "$AGENT_PATH"
        log_error "Checksum mismatch. Expected $FETCHED_CHECKSUM but downloaded $CALCULATED_CHECKSUM"
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
