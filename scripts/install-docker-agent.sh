#!/bin/bash
set -e

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

log_info() {
    printf '[INFO] %s\n' "$1"
}

log_success() {
    printf '[ OK ] %s\n' "$1"
}

log_warn() {
    printf '[WARN] %s\n' "$1" >&2
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
    local value
    value=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
    case "$value" in
        1|true|yes|y|on)
            PARSED_BOOL="true"
            return 0
            ;;
        0|false|no|n|off|"")
            PARSED_BOOL="false"
            return 0
            ;;
        *)
            return 1
            ;;
    esac
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
        if [[ -n "${seen[$candidate]}" ]]; then
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
#   curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- --url http://pulse.example.com --token <api-token>
# Install (multi-target fan-out):
#   curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- \
#     --target https://pulse.example.com|<api-token> \
#     --target https://pulse-dr.example.com|<api-token>
# Uninstall:
#   curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- --uninstall [--purge]

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
PURGE=false
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
    --purge)
      PURGE=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 --url <Pulse URL> --token <API token> [--interval 30s]"
      echo "       $0 --agent-path /custom/path/pulse-docker-agent"
      echo "       $0 --uninstall [--purge]"
      exit 1
      ;;
  esac
done

# Validate purge usage
if [[ "$PURGE" = true && "$UNINSTALL" != true ]]; then
    log_warn "--purge is only valid together with --uninstall; ignoring"
    PURGE=false
fi

# Normalize PULSE_URL - strip trailing slashes to prevent double-slash issues
PULSE_URL="${PULSE_URL%/}"

ORIGINAL_ARGS_STRING=""
if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
    __quoted_original_args=()
    for __arg in "${ORIGINAL_ARGS[@]}"; do
        __quoted_original_args+=("$(quote_shell_arg "$__arg")")
    done
    ORIGINAL_ARGS_STRING="${__quoted_original_args[*]}"
fi
unset __quoted_original_args __arg

ORIGINAL_ARGS_ESCAPED=""
if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
    __command_args=()
    for __arg in "${ORIGINAL_ARGS[@]}"; do
        __command_args+=("$(printf '%q' "$__arg")")
    done
    ORIGINAL_ARGS_ESCAPED="${__command_args[*]}"
fi
unset __command_args __arg

INSTALLER_URL_HINT="${PULSE_INSTALLER_URL_HINT:-}"
if [[ -z "$INSTALLER_URL_HINT" && -n "$PULSE_URL" ]]; then
    INSTALLER_URL_HINT="${PULSE_URL%/}/install-docker-agent.sh"
fi
if [[ -z "$INSTALLER_URL_HINT" && ${#TARGET_SPECS[@]} -gt 0 ]]; then
    if parse_target_spec "${TARGET_SPECS[0]}" >/dev/null 2>&1; then
        if [[ -n "$PARSED_TARGET_URL" ]]; then
            INSTALLER_URL_HINT="${PARSED_TARGET_URL%/}/install-docker-agent.sh"
        fi
    fi
fi

if [[ -z "$INSTALLER_URL_HINT" && -f "$SERVICE_PATH" ]]; then
    mapfile -t __service_targets_for_hint < <(extract_targets_from_service "$SERVICE_PATH")
    if [[ ${#__service_targets_for_hint[@]} -gt 0 ]]; then
        if parse_target_spec "${__service_targets_for_hint[0]}" >/dev/null 2>&1; then
            if [[ -n "$PARSED_TARGET_URL" ]]; then
                INSTALLER_URL_HINT="${PARSED_TARGET_URL%/}/install-docker-agent.sh"
            fi
        fi
    fi
    unset __service_targets_for_hint
fi

if [[ -z "$INSTALLER_URL_HINT" && -n "${PPID:-}" ]]; then
    PARENT_CMD=$(ps -o command= -p "$PPID" 2>/dev/null || true)
    if [[ -n "$PARENT_CMD" ]]; then
        if [[ "$PARENT_CMD" =~ (https?://[^[:space:]\"\'\|]+/install-docker-agent\.sh) ]]; then
            INSTALLER_URL_HINT="${BASH_REMATCH[1]}"
        elif [[ "$PARENT_CMD" =~ (https?://[^[:space:]\|]+install-docker-agent\.sh) ]]; then
            INSTALLER_URL_HINT="${BASH_REMATCH[1]}"
        fi
        if [[ -n "$INSTALLER_URL_HINT" ]]; then
            INSTALLER_URL_HINT="${INSTALLER_URL_HINT#\"}"
            INSTALLER_URL_HINT="${INSTALLER_URL_HINT#\'}"
            INSTALLER_URL_HINT="${INSTALLER_URL_HINT%\"}"
            INSTALLER_URL_HINT="${INSTALLER_URL_HINT%\'}"
        fi
    fi
    unset PARENT_CMD
fi

PIPELINE_COMMAND=""
PIPELINE_COMMAND_SUDO_C=""
PIPELINE_COMMAND_INNER=""
if [[ -n "$INSTALLER_URL_HINT" ]]; then
    INSTALLER_URL_QUOTED=$(quote_shell_arg "$INSTALLER_URL_HINT")
    PIPELINE_COMMAND="curl -fsSL ${INSTALLER_URL_QUOTED} | sudo bash -s"
    if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
        PIPELINE_COMMAND+=" -- ${ORIGINAL_ARGS_STRING}"
    fi
    INSTALLER_URL_ESCAPED=$(printf '%q' "$INSTALLER_URL_HINT")
    PIPELINE_COMMAND_INNER="curl -fsSL ${INSTALLER_URL_ESCAPED} | bash -s"
    if [[ -n "$ORIGINAL_ARGS_ESCAPED" ]]; then
        PIPELINE_COMMAND_INNER+=" -- ${ORIGINAL_ARGS_ESCAPED}"
    fi
    PIPELINE_COMMAND_SUDO_C="sudo bash -c $(quote_shell_arg "$PIPELINE_COMMAND_INNER")"
    unset INSTALLER_URL_ESCAPED INSTALLER_URL_QUOTED
fi

SCRIPT_SOURCE_HINT=""
if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
    __script_source_candidate="${BASH_SOURCE[0]}"
    if [[ -n "$__script_source_candidate" && -f "$__script_source_candidate" ]]; then
        if command -v realpath >/dev/null 2>&1; then
            __resolved_source=$(realpath "$__script_source_candidate" 2>/dev/null || true)
        elif command -v readlink >/dev/null 2>&1; then
            __resolved_source=$(readlink -f "$__script_source_candidate" 2>/dev/null || true)
        else
            __resolved_source=""
        fi
        if [[ -z "$__resolved_source" ]]; then
            if [[ "$__script_source_candidate" == /* ]]; then
                __resolved_source="$__script_source_candidate"
            else
                __resolved_source="$(pwd)/$__script_source_candidate"
            fi
        fi
        SCRIPT_SOURCE_HINT="$__resolved_source"
    fi
fi
unset __script_source_candidate __resolved_source

LOCAL_COMMAND=""
if [[ -n "$SCRIPT_SOURCE_HINT" ]]; then
    LOCAL_COMMAND="sudo bash $(quote_shell_arg "$SCRIPT_SOURCE_HINT")"
    if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
        LOCAL_COMMAND+=" ${ORIGINAL_ARGS_STRING}"
    fi
fi

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   AUTO_SUDO_ATTEMPTED="false"
   AUTO_SUDO_EXIT_CODE=0
   if [[ -z "${PULSE_AUTO_SUDO_ATTEMPTED:-}" ]]; then
       PULSE_AUTO_SUDO_ATTEMPTED=1
       export PULSE_AUTO_SUDO_ATTEMPTED
       if command -v sudo >/dev/null 2>&1; then
           if [[ -n "$SCRIPT_SOURCE_HINT" ]]; then
               AUTO_SUDO_ATTEMPTED="true"
               echo "Requesting sudo to continue installation..."
               if sudo bash "$SCRIPT_SOURCE_HINT" "${ORIGINAL_ARGS[@]}"; then
                   exit 0
               else
                   AUTO_SUDO_EXIT_CODE=$?
               fi
           elif [[ -n "$PIPELINE_COMMAND_INNER" ]]; then
               AUTO_SUDO_ATTEMPTED="true"
               echo "Requesting sudo to continue installation..."
               if sudo bash -c "$PIPELINE_COMMAND_INNER"; then
                   exit 0
               else
                   AUTO_SUDO_EXIT_CODE=$?
               fi
           fi
       fi
   fi

   if [[ "$AUTO_SUDO_ATTEMPTED" == "true" ]]; then
       echo "WARN: Automatic sudo elevation failed (exit code ${AUTO_SUDO_EXIT_CODE})."
       echo ""
       if [[ -n "$PIPELINE_COMMAND" ]]; then
           echo "Retry manually with sudo:"
           echo "  $PIPELINE_COMMAND"
           if [[ -n "$PIPELINE_COMMAND_SUDO_C" ]]; then
               echo ""
           fi
       fi
       if [[ -n "$PIPELINE_COMMAND_SUDO_C" ]]; then
           echo "Or run the entire pipeline under sudo:"
           echo "  $PIPELINE_COMMAND_SUDO_C"
           if [[ -n "$LOCAL_COMMAND" ]]; then
               echo ""
           fi
       fi
       if [[ -n "$LOCAL_COMMAND" ]]; then
           echo "If you downloaded the script locally:"
           echo "  $LOCAL_COMMAND"
           echo ""
       fi
       if [[ -z "$PIPELINE_COMMAND" && -z "$LOCAL_COMMAND" ]]; then
           echo "Please re-run the installer with elevated privileges, for example:"
           if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
               echo "  curl -fsSL <URL>/install-docker-agent.sh | sudo bash -s -- ${ORIGINAL_ARGS_STRING}"
               FALLBACK_PIPELINE_INNER="curl -fsSL <URL>/install-docker-agent.sh | bash -s -- ${ORIGINAL_ARGS_ESCAPED}"
               echo "  sudo bash -c $(quote_shell_arg "$FALLBACK_PIPELINE_INNER")"
               unset FALLBACK_PIPELINE_INNER
           else
               echo "  curl -fsSL <URL>/install-docker-agent.sh | sudo bash -s"
               echo "  sudo bash -c 'curl -fsSL <URL>/install-docker-agent.sh | bash -s'"
           fi
       fi
       exit "${AUTO_SUDO_EXIT_CODE:-1}"
   fi

   echo "Error: This script must be run as root"
   echo ""
   if [[ -n "$PIPELINE_COMMAND" ]]; then
       echo "Re-run with sudo using the same arguments:"
       echo "  $PIPELINE_COMMAND"
       if [[ -n "$PIPELINE_COMMAND_SUDO_C" ]]; then
           echo ""
       fi
   fi
   if [[ -n "$PIPELINE_COMMAND_SUDO_C" ]]; then
       echo "Or run the entire pipeline under sudo:"
       echo "  $PIPELINE_COMMAND_SUDO_C"
       if [[ -n "$LOCAL_COMMAND" ]]; then
           echo ""
       fi
   fi
   if [[ -n "$LOCAL_COMMAND" ]]; then
       echo "If you downloaded the script locally:"
       echo "  $LOCAL_COMMAND"
       echo ""
   fi
   if [[ -z "$PIPELINE_COMMAND" && -z "$LOCAL_COMMAND" ]]; then
       echo "Please re-run the installer with elevated privileges, for example:"
       if [[ ${#ORIGINAL_ARGS[@]} -gt 0 ]]; then
           echo "  curl -fsSL <URL>/install-docker-agent.sh | sudo bash -s -- ${ORIGINAL_ARGS_STRING}"
           FALLBACK_PIPELINE_INNER="curl -fsSL <URL>/install-docker-agent.sh | bash -s -- ${ORIGINAL_ARGS_ESCAPED}"
           echo "  sudo bash -c $(quote_shell_arg "$FALLBACK_PIPELINE_INNER")"
           unset FALLBACK_PIPELINE_INNER
       else
           echo "  curl -fsSL <URL>/install-docker-agent.sh | sudo bash -s"
           echo "  sudo bash -c 'curl -fsSL <URL>/install-docker-agent.sh | bash -s'"
       fi
   fi
   exit 1
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
    log_header "Pulse Docker Agent Uninstaller"

    if command -v systemctl &> /dev/null; then
        log_info "Stopping pulse-docker-agent service"
        systemctl stop pulse-docker-agent 2>/dev/null || true
        log_info "Disabling pulse-docker-agent service"
        systemctl disable pulse-docker-agent 2>/dev/null || true
        if [ -f "$SERVICE_PATH" ]; then
            rm -f "$SERVICE_PATH"
            log_success "Removed unit file: $SERVICE_PATH"
        else
            log_info "Unit file not present at $SERVICE_PATH"
        fi
        systemctl daemon-reload 2>/dev/null || true
    else
        log_warn "systemctl not found; skipping service disable"
    fi

    if pgrep -f pulse-docker-agent > /dev/null 2>&1; then
        log_info "Stopping running agent processes"
        pkill -f pulse-docker-agent 2>/dev/null || true
        sleep 1
    fi

    if [ -f "$AGENT_PATH" ]; then
        rm -f "$AGENT_PATH"
        log_success "Removed agent binary: $AGENT_PATH"
    else
        log_info "Agent binary not found at $AGENT_PATH"
    fi

    if [ -f "$UNRAID_STARTUP" ]; then
        rm -f "$UNRAID_STARTUP"
        log_success "Removed Unraid startup script: $UNRAID_STARTUP"
    fi

    if [ "$PURGE" = true ]; then
        if [ -f "$LOG_PATH" ]; then
            rm -f "$LOG_PATH"
            log_success "Removed agent log file: $LOG_PATH"
        else
            log_info "Agent log file already absent: $LOG_PATH"
        fi
    elif [ -f "$LOG_PATH" ]; then
        log_info "Preserving agent log file at $LOG_PATH (use --purge to remove)"
    fi

    log_success "Uninstall complete"
    log_info "The Pulse Docker agent has been removed from this system."
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

    if [[ -n "${SEEN_TARGETS[$local_normalized]}" ]]; then
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
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

if [[ "$UNINSTALL" != true ]]; then
    if command -v systemctl &> /dev/null; then
        if systemctl list-unit-files pulse-docker-agent.service &> /dev/null; then
            if systemctl is-active --quiet pulse-docker-agent; then
                systemctl stop pulse-docker-agent
            fi
        fi
    else
        if pgrep -f pulse-docker-agent > /dev/null 2>&1; then
            log_info "Stopping running agent process"
            pkill -f pulse-docker-agent 2>/dev/null || true
            sleep 1
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

if ! command -v wget &> /dev/null && ! command -v curl &> /dev/null; then
    echo "Error: Neither wget nor curl found. Please install one of them."
    exit 1
fi

WGET_IS_BUSYBOX="false"
if command -v wget &> /dev/null; then
    if wget --help 2>&1 | grep -qi "busybox"; then
        WGET_IS_BUSYBOX="true"
    fi
fi

download_agent_from_url() {
    local url="$1"
    local wget_success="false"

    if command -v wget &> /dev/null; then
        local wget_args=(-O "$AGENT_PATH" "$url")
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            wget_args=(--no-check-certificate "${wget_args[@]}")
        fi
        if [[ "$WGET_IS_BUSYBOX" == "true" ]]; then
            wget_args=(-q "${wget_args[@]}")
        else
            wget_args=(-q --show-progress "${wget_args[@]}")
        fi

        if wget "${wget_args[@]}"; then
            wget_success="true"
        else
            rm -f "$AGENT_PATH"
        fi
    fi

    if [[ "$wget_success" == "true" ]]; then
        return 0
    fi

    if command -v curl &> /dev/null; then
        local curl_args=(-fL --progress-bar -o "$AGENT_PATH" "$url")
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            curl_args=(-k "${curl_args[@]}")
        fi
        if curl "${curl_args[@]}"; then
            return 0
        fi
        rm -f "$AGENT_PATH"
    fi

    return 1
}

if download_agent_from_url "$DOWNLOAD_URL"; then
    :
elif [[ "$DOWNLOAD_URL" != "$DOWNLOAD_URL_BASE" ]] && download_agent_from_url "$DOWNLOAD_URL_BASE"; then
    log_info 'Falling back to server default agent binary'
else
    log_warn 'Failed to download agent binary'
    log_warn "Ensure the Pulse server is reachable at $PRIMARY_URL"
    exit 1
fi

chmod +x "$AGENT_PATH"
log_success "Agent binary installed"

allow_reenroll_if_needed() {
    local host_id="$1"
    if [[ -z "$host_id" || -z "$PRIMARY_TOKEN" || -z "$PRIMARY_URL" ]]; then
        return 0
    fi

    local endpoint="$PRIMARY_URL/api/agents/docker/hosts/${host_id}/allow-reenroll"
    local success="false"

    if command -v curl &> /dev/null; then
        local curl_args=(-fsSL -X POST -H "X-API-Token: $PRIMARY_TOKEN" "$endpoint")
        if [[ "$PRIMARY_INSECURE" == "true" ]]; then
            curl_args=(-k "${curl_args[@]}")
        fi
        if curl "${curl_args[@]}" >/dev/null 2>&1; then
            success="true"
        fi
    fi

    if [[ "$success" != "true" ]]; then
        if command -v wget &> /dev/null; then
            local wget_args=(--method=POST --header="X-API-Token: $PRIMARY_TOKEN" -q -O /dev/null "$endpoint")
            if [[ "$PRIMARY_INSECURE" == "true" ]]; then
                wget_args=(--no-check-certificate "${wget_args[@]}")
            fi
            if wget "${wget_args[@]}" >/dev/null 2>&1; then
                success="true"
            fi
        fi
    fi

    if [[ "$success" == "true" ]]; then
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
if command -v curl &> /dev/null || command -v wget &> /dev/null; then
    SERVER_INFO_URL="$PRIMARY_URL/api/server/info"
    IS_DEV="false"

    if command -v curl &> /dev/null; then
        SERVER_INFO=$(curl -fsSL "$SERVER_INFO_URL" 2>/dev/null || echo "")
    elif command -v wget &> /dev/null; then
        SERVER_INFO=$(wget -qO- "$SERVER_INFO_URL" 2>/dev/null || echo "")
    fi

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
cat > "$SERVICE_PATH" << EOF
[Unit]
Description=Pulse Docker Agent
After=network-online.target docker.service
Wants=network-online.target

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

[Install]
WantedBy=multi-user.target
EOF

log_success "Wrote unit file: $SERVICE_PATH"

# Reload systemd and start service
log_info 'Starting service'
systemctl daemon-reload
systemctl enable pulse-docker-agent
systemctl start pulse-docker-agent

log_header 'Installation complete'
log_info 'Agent service enabled and started'
log_info 'Check status          : systemctl status pulse-docker-agent'
log_info 'Follow logs           : journalctl -u pulse-docker-agent -f'
log_info 'Host visible in Pulse : ~30 seconds'
