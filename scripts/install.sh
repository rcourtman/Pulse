#!/usr/bin/env bash
#
# Pulse Unified Agent Installer
# Supports: Linux (systemd, OpenRC, SysV init), macOS (launchd), FreeBSD (rc.d), Synology DSM (6.x/7+), Unraid, QNAP QTS/QuTS hero, TrueNAS
#
# Usage:
#   curl -fsSL http://pulse/install.sh | bash -s -- --url http://pulse --token <token> [options]
#   curl -fsSL http://pulse/install.sh | bash -s -- --update --url http://pulse [options]
#
# Options:
#   --enable-host       Enable host metrics (default: true)
#   --enable-docker     Force enable Docker monitoring (default: auto-detect)
#   --disable-docker    Disable Docker monitoring even if detected
#   --enable-kubernetes Force enable Kubernetes monitoring (default: auto-detect)
#   --kubeconfig <path> Path to kubeconfig file (auto-detected if not specified)
#   --disable-kubernetes Disable Kubernetes monitoring even if detected
#   --kube-include-all-pods Include all non-succeeded pods (default: false)
#   --kube-include-all-deployments Include all deployments (default: false)
#   --enable-proxmox    Force enable Proxmox integration (default: auto-detect)
#   --disable-proxmox   Disable Proxmox integration even if detected
#   --interval <dur>    Reporting interval (default: 30s)
#   --agent-id <id>     Custom agent identifier (default: auto-generated)
#   --disk-exclude <pattern>  Exclude device names/paths or mount points (repeatable)
#   --disk-include <pattern>  Include a device or mount point despite automatic filtering (repeatable)
#   --insecure          Skip TLS certificate verification
#   --server-fingerprint <sha256> Pin the Pulse server leaf certificate
#   --observers-file <path> Report to additional observer Pulse instances
#   --enable-commands   Enable Pulse command execution on agent (disabled by default; required for Patrol actions and Proxmox LXC Docker inventory)
#   --least-privilege   Run the agent as the dedicated 'pulse-agent' system user instead of root (Linux systemd only; SMART, Proxmox LXC filesystems, and command execution need root or an explicit grant)
#   --enable-privileged-helper Install the typed root helper for an explicitly selected least-privilege Linux systemd profile
#   --enable-action-runner Install the separately credentialed typed remediation runner (requires the typed-helper profile)
#   --disable-action-runner Disable and remove the action runner while leaving monitoring active
#   --uninstall-action-runner Remove only the action runner while leaving monitoring active
#   --action-token-file <path> Read the separate action-runner credential from a file
#   --grant-smart       With --least-privilege: allow SMART collection through an exact-command sudoers grant for smartctl
#   --grant-pct         With --least-privilege: allow Proxmox LXC filesystem capacity through a sudoers grant restricted to 'pct list' and 'pct df'
#   --health-addr <addr> Health/metrics listener address (default: 127.0.0.1:9191, use "" to disable)
#   --safe-profile-inspect Report the effective collector privilege profile and migration differences without changing the host
#   --safe-profile-apply Explicitly migrate an existing Linux systemd collector to the typed-helper monitoring-only profile
#   --safe-profile-rollback Restore the collector/helper snapshot retained by the last successful safe-profile migration
#   --update            Update an existing agent using saved connection state
#   --uninstall         Remove the agent
#
# Auto-Detection:
#   The installer automatically detects Docker, Kubernetes, and Proxmox on the
#   target machine and enables monitoring for detected platforms. Proxmox auto
#   mode keeps the runtime unpinned so the agent can register every detected
#   PVE / PBS service on that host. Use --disable-* flags to skip specific
#   platforms, or --enable-* to force enable even if not detected.

set -euo pipefail

# Wrap entire script in a function to protect against partial download
# See: https://www.kicksecure.com/wiki/Dev/curl_bash_pipe
main() {

# --- Cleanup trap ---
TMP_FILES=()
# shellcheck disable=SC2317  # Invoked by trap, not directly
cleanup() {
    if [[ "${SAFE_PROFILE_TRANSACTION_ACTIVE:-false}" == "true" &&
          "${SAFE_PROFILE_TRANSACTION_COMMITTED:-false}" != "true" &&
          -n "${SAFE_PROFILE_TRANSACTION_DIR:-}" ]] &&
       declare -F safe_profile_restore_transaction >/dev/null 2>&1; then
        log_error "Safe-profile migration did not commit; restoring the previous collector/helper profile."
        safe_profile_restore_transaction "$SAFE_PROFILE_TRANSACTION_DIR" "automatic-failure" ||
            log_error "Automatic safe-profile rollback failed; run --safe-profile-rollback before retrying."
    fi
    # Use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u
    for f in ${TMP_FILES[@]+"${TMP_FILES[@]}"}; do
        rm -f "$f" 2>/dev/null || true
    done
}
trap cleanup EXIT

# --- Configuration ---
AGENT_NAME="pulse-agent"
BINARY_NAME="pulse-agent"
INSTALL_DIR="/usr/local/bin"
LOG_FILE="/var/log/${AGENT_NAME}.log"

# TrueNAS SCALE configuration (immutable root filesystem)
TRUENAS=false
TRUENAS_STATE_DIR="/data/pulse-agent"
TRUENAS_LOG_DIR="$TRUENAS_STATE_DIR/logs"
TRUENAS_LOG_FILE=""    # Set during TrueNAS detection
TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
TRUENAS_ENV_FILE="$TRUENAS_STATE_DIR/pulse-agent.env"

# Defaults
PULSE_URL=""
PULSE_TOKEN=""
INTERVAL="30s"
ENABLE_HOST="true"
ENABLE_DOCKER=""  # Empty means "auto-detect"
ENABLE_KUBERNETES=""  # Empty means "auto-detect"
ENABLE_PROXMOX=""  # Empty means "auto-detect"
PROXMOX_TYPE=""
UPDATE_ONLY="false"
RETARGET_ONLY="false"
UNINSTALL="false"
INSECURE="false"
INSECURE_EXPLICIT="false"
SERVER_FINGERPRINT="${PULSE_SERVER_FINGERPRINT:-}"
OBSERVERS_FILE="${PULSE_OBSERVERS_FILE:-}"
AGENT_ID=""
HOSTNAME_OVERRIDE=""
REPORT_IP=""
ENABLE_COMMANDS="false"
COMMAND_AUTHORITY="${PULSE_COMMAND_AUTHORITY:-}"
COMMAND_AUTHORITY_SOURCE=""
if [[ -n "$COMMAND_AUTHORITY" ]]; then
    COMMAND_AUTHORITY_SOURCE="explicit"
fi
HEALTH_ADDR="${PULSE_HEALTH_ADDR:-}"
HEALTH_ADDR_SET="false"
if [[ -n "${PULSE_HEALTH_ADDR+x}" ]]; then
    HEALTH_ADDR_SET="true"
fi
ENROLL="false"
KUBECONFIG_PATH=""  # Path to kubeconfig file for Kubernetes monitoring
KUBE_INCLUDE_ALL_PODS="false"
KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
DISK_EXCLUDES=()  # Array for multiple --disk-exclude values
DISK_INCLUDES=()  # Array for multiple --disk-include values
AGENT_LOG_FILE="" # When set, pass --log-file so the agent's rotating log writer engages (set per platform)
DEFAULT_STATE_DIR="/var/lib/pulse-agent"
STATE_DIR="$DEFAULT_STATE_DIR"  # Persistent state directory (overridden per platform)
STATE_DIR_SOURCE="default"      # default, explicit, recovered, or platform
CURL_CA_BUNDLE="${PULSE_CACERT:-}" # Path to CA bundle for curl and agent TLS (sets SSL_CERT_FILE)
NON_INTERACTIVE="false"
TOKEN_FILE_PATH=""       # Path to file containing the token
RUNTIME_TOKEN_FILE=""    # Secure token file passed to the installed service
RUNTIME_TOKEN_CHANGED="false"
OUTPUT_FORMAT="text"     # "text" (default) or "json"
PREFLIGHT_ONLY="false"
INSTALL_SIGNATURE_NAMESPACE="pulse-install"
INSTALL_SIGNATURE_IDENTITY="pulse-installer"
PINNED_INSTALLER_SSH_PUBLIC_KEY="__PULSE_INSTALLER_SSH_PUBLIC_KEY__"

ROOTLESS_RUNTIME_KIND=""
ROOTLESS_RUNTIME_SOCKET_PATH=""
ROOTLESS_RUNTIME_SOCKET_URI=""
ROOTLESS_RUNTIME_XDG_DIR=""

# Least-privilege profile: run the service as a dedicated system user instead
# of root, with optional exact-command sudo helpers for the two collectors
# that genuinely need elevation (smartctl, pct list/df). Linux systemd only.
LEAST_PRIVILEGE="false"
GRANT_SMART="false"
GRANT_PCT="false"
SERVICE_USER="root"
LEAST_PRIVILEGE_USER="pulse-agent"
PRIVILEGE_HELPER_DIR="/usr/local/lib/pulse-agent"
PRIVILEGE_SUDOERS_FILE="/etc/sudoers.d/pulse-agent"
PRIVILEGED_HELPER_ENABLED="false"
PRIVILEGED_HELPER_EXPLICIT="false"
PRIVILEGED_HELPER_NAME="pulse-agent-helper"
PRIVILEGED_HELPER_BINARY_NAME="pulse-agent-helper"
PRIVILEGED_HELPER_BINARY_PATH="${PRIVILEGE_HELPER_DIR}/${PRIVILEGED_HELPER_BINARY_NAME}"
PRIVILEGED_HELPER_SERVICE_UNIT="/etc/systemd/system/${PRIVILEGED_HELPER_NAME}.service"
PRIVILEGED_HELPER_SOCKET_UNIT="/etc/systemd/system/${PRIVILEGED_HELPER_NAME}.socket"
PRIVILEGED_HELPER_SOCKET_DIR="/run/pulse-agent"
PRIVILEGED_HELPER_SOCKET_PATH="${PRIVILEGED_HELPER_SOCKET_DIR}/helper.sock"
PRIVILEGED_HELPER_CREDENTIAL_DIR="/etc/pulse-agent"
PRIVILEGED_HELPER_STATE_DIR="/var/lib/pulse-agent-helper"
PRIVILEGED_HELPER_UPDATE_STAGING_DIR="${PRIVILEGED_HELPER_STATE_DIR}/update-staging"
PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR="/var/lib/pulse-agent/update-quarantine"
TMP_HELPER_BIN=""

# Typed remediation is a separate root service and credential lifecycle. It is
# never inferred from the collector profile or from the collector token.
ACTION_RUNNER_ENABLED="false"
ACTION_RUNNER_EXPLICIT="false"
UNINSTALL_ACTION_RUNNER="false"
ACTION_RUNNER_NAME="pulse-agent-runner"
ACTION_RUNNER_BINARY_NAME="pulse-agent-runner"
ACTION_RUNNER_BINARY_PATH="${PRIVILEGE_HELPER_DIR}/${ACTION_RUNNER_BINARY_NAME}"
ACTION_RUNNER_SERVICE_UNIT="/etc/systemd/system/${ACTION_RUNNER_NAME}.service"
ACTION_RUNNER_CONFIG_DIR="/etc/pulse-agent-runner"
ACTION_RUNNER_ENV_FILE="${ACTION_RUNNER_CONFIG_DIR}/runner.env"
ACTION_RUNNER_TOKEN_FILE="${ACTION_RUNNER_CONFIG_DIR}/token"
ACTION_RUNNER_STATE_DIR="/var/lib/pulse-agent-runner"
ACTION_RUNNER_HEALTH_FILE="${ACTION_RUNNER_STATE_DIR}/health.json"
ACTION_RUNNER_ACTIVATION_NONCE=""
ACTION_TOKEN=""
ACTION_TOKEN_FILE_PATH=""
TMP_ACTION_RUNNER_BIN=""

# Explicit safe-profile migration lifecycle. Ordinary --update deliberately
# leaves these unset and therefore never enters a migration transaction.
SAFE_PROFILE_ACTION=""
SAFE_PROFILE_STATE_DIR="/var/lib/pulse-agent-profile"
SAFE_PROFILE_CURRENT_FILE="${SAFE_PROFILE_STATE_DIR}/current.env"
SAFE_PROFILE_COLLECTOR_UNIT="/etc/systemd/system/${AGENT_NAME}.service"
SAFE_PROFILE_TRANSACTION_DIR=""
SAFE_PROFILE_TRANSACTION_ACTIVE="false"
SAFE_PROFILE_TRANSACTION_COMMITTED="false"
SAFE_PROFILE_PRIOR_REGISTRATION_LAST_SEEN=""
AGENT_REGISTRATION_LAST_SEEN=""

SYSTEMD_ENV_LINES=""
SHELL_EXPORT_LINES=""
PLIST_ENV_ENTRIES=""
PLIST_ENV_BLOCK=""
UPSTART_ENV_LINES=""
SED_EXPORT_LINES=""
APPLIED_SERVICE_ENV_KEYS="|"

# Track if flags were explicitly set (to override auto-detection)
DOCKER_EXPLICIT="false"
KUBERNETES_EXPLICIT="false"
PROXMOX_EXPLICIT="false"
HOST_EXPLICIT="false"
INTERVAL_EXPLICIT="false"

# --- Helper Functions ---
log_info() {
    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        printf "[INFO] %s\n" "$(redact_token "$1")"
    else
        printf "[INFO] %s\n" "$1"
    fi
}
log_warn() {
    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        printf "[WARN] %s\n" "$(redact_token "$1")"
    else
        printf "[WARN] %s\n" "$1"
    fi
}
log_error() {
    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        printf "[ERROR] %s\n" "$(redact_token "$1")"
    else
        printf "[ERROR] %s\n" "$1"
    fi
}

# Feed the API token to curl through a private config file. Passing a header
# value with -H would expose the token in the transient curl process argv.
curl_with_pulse_token() {
    local config_file=""
    local curl_rc=0

    if [[ -z "$PULSE_TOKEN" ]]; then
        curl "$@"
        return $?
    fi
    case "$PULSE_TOKEN" in
        *$'\r'*|*$'\n'*) return 2 ;;
    esac

    config_file=$(mktemp)
    TMP_FILES+=("$config_file")
    chmod 600 "$config_file"
    printf 'header = "X-API-Token: %s"\n' "$PULSE_TOKEN" > "$config_file"
    if curl --config "$config_file" "$@"; then
        curl_rc=0
    else
        curl_rc=$?
    fi
    rm -f "$config_file"
    return "$curl_rc"
}
url_encode() {
    local input="$1"
    local output=""
    local i c encoded
    local old_lc_all="${LC_ALL-}"
    LC_ALL=C
    for ((i=0; i<${#input}; i++)); do
        c="${input:i:1}"
        case "$c" in
            [a-zA-Z0-9.~_-])
                output+="$c"
                ;;
            *)
                printf -v encoded '%%%02X' "'$c"
                output+="$encoded"
                ;;
        esac
    done
    if [[ -n "${old_lc_all}" ]]; then
        LC_ALL="$old_lc_all"
    else
        unset LC_ALL
    fi
    printf '%s' "$output"
}
final_response_header_value() {
    local headers_path="$1"
    local header_name="$2"

    awk -v header_name="$header_name" '
        BEGIN {
            prefix = tolower(header_name) ":"
            value = ""
        }
        /^HTTP\// {
            value = ""
            next
        }
        {
            line = $0
            sub(/\r$/, "", line)
            if (substr(tolower(line), 1, length(prefix)) == prefix) {
                value = substr(line, length(prefix) + 1)
                sub(/^[[:space:]]+/, "", value)
            }
        }
        END {
            print value
        }
    ' "$headers_path" 2>/dev/null
}
fail() {
    local code="${2:-1}"
    if [[ "$OUTPUT_FORMAT" == "json" ]]; then
        printf '{"phase":"error","code":"install_failed","message":"%s","exitCode":%d}\n' \
            "$(echo "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g' | tr -d '\n\r')" "$code"
    else
        log_error "$1"
    fi
    if [[ "$NON_INTERACTIVE" != "true" ]]; then
        if [[ -t 0 ]]; then
            read -r -p "Press Enter to exit..."
        elif [[ -e /dev/tty ]]; then
            read -r -p "Press Enter to exit..." < /dev/tty
        fi
    fi
    exit "$code"
}

# Stable exit codes by failure class
EXIT_OK=0
EXIT_GENERAL=1
EXIT_UNSUPPORTED_ARCH=10
EXIT_DOWNLOAD_FAILED=11
EXIT_CHECKSUM_FAILED=12
EXIT_SERVICE_START_FAILED=13
EXIT_PREFLIGHT_FAILED=14
EXIT_ALREADY_INSTALLED=15    # Not a failure — used with --preflight-only
EXIT_MISSING_ARGS=16
EXIT_SIGNATURE_FAILED=17
EXIT_AUTH_REJECTED=18

json_event() {
    # Usage: json_event <phase> <code> <message> [exitCode]
    if [[ "$OUTPUT_FORMAT" == "json" ]]; then
        local exit_code="${4:-0}"
        printf '{"phase":"%s","code":"%s","message":"%s","exitCode":%d}\n' \
            "$1" "$2" "$(echo "$3" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g' | tr -d '\n\r')" "$exit_code"
    fi
}

redact_token() {
    # Replace token values with redacted placeholder in log output
    local msg="$1"
    if [[ -n "$PULSE_TOKEN" ]]; then
        msg="${msg//$PULSE_TOKEN/[REDACTED]}"
    fi
    if [[ -n "$TOKEN_FILE_PATH" ]]; then
        msg="${msg//$TOKEN_FILE_PATH/[token-file]}"
    fi
    echo "$msg"
}

# Minimum free space to stage (download to temp) and install the agent binary.
# The 6.x agent binary is ~34MiB; keep margin for growth. On appliances with a
# RAM-backed root (QNAP, Unraid) temp and install dir share one small filesystem.
AGENT_MIN_TEMP_FREE_BYTES=$((48 * 1024 * 1024))
AGENT_MIN_INSTALL_FREE_BYTES=$((48 * 1024 * 1024))

bytes_to_human() {
    local bytes="${1:-0}"

    if [[ ! "$bytes" =~ ^[0-9]+$ ]]; then
        printf '%s\n' "$bytes"
        return 0
    fi

    local units=("B" "KB" "MB" "GB" "TB")
    local value="$bytes"
    local unit_index=0

    while (( value >= 1024 && unit_index < ${#units[@]} - 1 )); do
        value=$((value / 1024))
        ((unit_index += 1))
    done

    printf '%s%s\n' "$value" "${units[$unit_index]}"
}

nearest_existing_dir() {
    local path="$1"

    while [[ -n "$path" && "$path" != "/" && ! -d "$path" ]]; do
        path=$(dirname "$path")
    done

    printf '%s\n' "${path:-/}"
}

get_available_bytes_for_path() {
    local path="$1"
    local available_kb=""

    available_kb=$(df -Pk "$path" 2>/dev/null | awk 'NR==2 {print $4}')
    if [[ ! "$available_kb" =~ ^[0-9]+$ ]]; then
        return 1
    fi

    printf '%s\n' $((available_kb * 1024))
}

get_filesystem_device_for_path() {
    local path="$1"
    local filesystem=""

    filesystem=$(df -Pk "$path" 2>/dev/null | awk 'NR==2 {print $1}')
    if [[ -z "$filesystem" ]]; then
        return 1
    fi

    printf '%s\n' "$filesystem"
}

ensure_agent_disk_headroom() {
    local temp_path="${1:-${TMPDIR:-/tmp}}"
    local install_path="${2:-$INSTALL_DIR}"
    local temp_fs=""
    local install_fs=""
    local temp_free_bytes=""
    local install_free_bytes=""
    local combined_required_bytes=$((AGENT_MIN_TEMP_FREE_BYTES + AGENT_MIN_INSTALL_FREE_BYTES))

    temp_path=$(nearest_existing_dir "$temp_path")
    install_path=$(nearest_existing_dir "$install_path")

    temp_fs=$(get_filesystem_device_for_path "$temp_path" 2>/dev/null || true)
    install_fs=$(get_filesystem_device_for_path "$install_path" 2>/dev/null || true)
    temp_free_bytes=$(get_available_bytes_for_path "$temp_path" 2>/dev/null || true)
    install_free_bytes=$(get_available_bytes_for_path "$install_path" 2>/dev/null || true)

    if [[ -z "$temp_free_bytes" || -z "$install_free_bytes" ]]; then
        log_warn "Could not determine available disk space for the install preflight; continuing anyway"
        return 0
    fi

    if [[ -n "$temp_fs" && "$temp_fs" == "$install_fs" ]]; then
        if (( temp_free_bytes < combined_required_bytes )); then
            log_error "Not enough free disk space to stage and install the Pulse agent"
            log_info "The same filesystem backs $temp_path and $install_path"
            log_info "Available: $(bytes_to_human "$temp_free_bytes"), required: $(bytes_to_human "$combined_required_bytes")"
            log_info "If this filesystem is a small RAM-backed root (common on QNAP/Unraid), set TMPDIR to a directory on a data volume before re-running, e.g. TMPDIR=/share/CACHEDEV1_DATA/tmp"
            return 1
        fi
        return 0
    fi

    if (( temp_free_bytes < AGENT_MIN_TEMP_FREE_BYTES )); then
        log_error "Not enough free disk space in $temp_path to stage the Pulse agent download"
        log_info "Available: $(bytes_to_human "$temp_free_bytes"), required: $(bytes_to_human "$AGENT_MIN_TEMP_FREE_BYTES")"
        log_info "Free disk space under $temp_path, or set TMPDIR to a directory with more space, and retry"
        return 1
    fi

    if (( install_free_bytes < AGENT_MIN_INSTALL_FREE_BYTES )); then
        log_error "Not enough free disk space in $install_path to install the Pulse agent"
        log_info "Available: $(bytes_to_human "$install_free_bytes"), required: $(bytes_to_human "$AGENT_MIN_INSTALL_FREE_BYTES")"
        log_info "Free disk space under $install_path and retry"
        return 1
    fi

    return 0
}

has_pinned_installer_signature_key() {
    [[ -n "$PINNED_INSTALLER_SSH_PUBLIC_KEY" && "$PINNED_INSTALLER_SSH_PUBLIC_KEY" != "__PULSE_INSTALLER_SSH_PUBLIC_KEY__" ]]
}

decode_base64_to_file() {
    local encoded="$1"
    local output="$2"

    if command -v base64 >/dev/null 2>&1; then
        if printf '%s' "$encoded" | base64 --decode > "$output" 2>/dev/null; then
            return 0
        fi
        if printf '%s' "$encoded" | base64 -d > "$output" 2>/dev/null; then
            return 0
        fi
        if printf '%s' "$encoded" | base64 -D > "$output" 2>/dev/null; then
            return 0
        fi
    fi

    fail "Base64 decoder is required to verify signed Pulse downloads." "$EXIT_SIGNATURE_FAILED"
}

verify_download_signature() {
    local target_path="$1"
    local signature_header="$2"

    if ! has_pinned_installer_signature_key; then
        return 0
    fi
    if [[ -z "$signature_header" ]]; then
        fail "Server did not provide SSH signature metadata; refusing signed install." "$EXIT_SIGNATURE_FAILED"
    fi
    if ! command -v ssh-keygen >/dev/null 2>&1; then
        fail "ssh-keygen is required to verify signed Pulse downloads." "$EXIT_SIGNATURE_FAILED"
    fi

    local allowed_signers signature_file
    allowed_signers=$(mktemp)
    signature_file=$(mktemp)
    TMP_FILES+=("$allowed_signers" "$signature_file")

    printf '%s %s\n' "$INSTALL_SIGNATURE_IDENTITY" "$PINNED_INSTALLER_SSH_PUBLIC_KEY" > "$allowed_signers"
    decode_base64_to_file "$signature_header" "$signature_file"

    if ! ssh-keygen -Y verify \
        -f "$allowed_signers" \
        -I "$INSTALL_SIGNATURE_IDENTITY" \
        -n "$INSTALL_SIGNATURE_NAMESPACE" \
        -s "$signature_file" < "$target_path" >/dev/null 2>&1; then
        fail "Cryptographic signature verification failed for the downloaded agent binary." "$EXIT_SIGNATURE_FAILED"
    fi

    json_event "download" "signature_ok" "Binary signature verified"
    log_info "Binary signature verified"
}

show_help() {
    cat <<EOF
Pulse Unified Agent Installer

Usage:
  install.sh [options]

Options:
  --url <url>             Pulse server URL (e.g. http://pulse:7655)
  --token <token>         Pulse API token
  --interval <duration>   Reporting interval (default: 30s)
  --enable-host           Enable host metrics (default: true)
  --disable-host          Disable host metrics
  --enable-docker         Force enable Docker monitoring
  --enable-kubernetes     Force enable Kubernetes monitoring
  --kubeconfig <path>     Path to kubeconfig file
  --kube-include-all-pods Include all non-succeeded pods
  --kube-include-all-deployments Include all deployments
  --enable-proxmox        Force enable Proxmox integration
  --agent-id <id>         Custom agent identifier
  --hostname <name>       Override hostname reported to Pulse
  --report-ip <ip>        IP address to report to Pulse (for multi-NIC systems)
  --state-dir <path>      Override persistent state directory
  --disk-exclude <pattern> Exclude device names/paths or mount points (repeatable)
  --disk-include <pattern> Include a device or mount point despite automatic filtering (repeatable)
  --insecure              Skip TLS verification (auto-enabled for http:// URLs)
  --cacert <path>         Custom CA certificate for TLS (used by curl and agent)
  --server-fingerprint <sha256> Pin the Pulse server leaf certificate for agent connections
  --observers-file <path> Absolute path to private JSON config for report-only observer Pulse destinations
  --enable-commands       Enable Pulse command execution (disabled by default; required for Patrol actions and Proxmox LXC Docker inventory)
  --command-authority <profile> Local command ceiling: monitoring-only, command-capable, or legacy
  --least-privilege       Run the agent as the 'pulse-agent' system user instead of root (Linux systemd only)
  --enable-privileged-helper Install the typed root helper (requires --least-privilege on standard Linux systemd)
  --disable-privileged-helper Remove/disable an existing typed helper profile during this install
  --enable-action-runner  Install the separate typed remediation service (requires --least-privilege and --enable-privileged-helper)
  --disable-action-runner Remove the action runner during this install; monitoring remains active
  --uninstall-action-runner Remove only the action runner and exit; monitoring remains active
  --action-token-file <path> Read the separate action credential from a private file (required on first enable; never placed in argv)
  --grant-smart           With --least-privilege: exact-command sudoers grant so SMART collection keeps working
  --grant-pct             With --least-privilege: sudoers grant restricted to 'pct list'/'pct df' so Proxmox LXC filesystem capacity keeps working
  --health-addr <addr>    Health/metrics listener address (default: 127.0.0.1:9191; use "" to disable)
  --safe-profile-inspect  Read-only report of current authority, providers, platform support, and calculated migration differences
  --safe-profile-apply    Explicitly migrate an existing Linux systemd collector to typed-helper monitoring-only (never implied by --update)
  --safe-profile-rollback Restore the prior collector/helper snapshot from the last committed safe-profile migration
  --enroll                Exchange bootstrap token for runtime token (deploy wizard)
  --update                Update an existing agent using saved connection state
  --retarget              Point an existing agent at --url using saved identity and token
  --uninstall             Remove the agent
  --non-interactive       Skip TTY prompts (for automated/scripted installs)
  --token-file <path>     Read token from file (alternative to --token)
  --pulse-url <url>       Alias for --url
  --preflight-only        Run preflight checks and exit (no install)
  --output <format>       Output format: text (default) or json
  --help, -h              Show this help

EOF
}

# --- SELinux Context Restoration ---
# On SELinux-enforcing systems (Fedora, RHEL, CentOS), binaries in non-standard
# locations need proper security contexts for systemd to execute them.
restore_selinux_contexts() {
    # Check if SELinux is available and enforcing
    if ! command -v getenforce >/dev/null 2>&1; then
        return 0  # SELinux not installed
    fi

    if [[ "$(getenforce 2>/dev/null)" != "Enforcing" ]]; then
        return 0  # SELinux not enforcing
    fi

    # restorecon is the proper way to fix SELinux contexts
    if command -v restorecon >/dev/null 2>&1; then
        log_info "Restoring SELinux contexts for installed binaries..."
        restorecon -v "${INSTALL_DIR}/${BINARY_NAME}" >/dev/null 2>&1 || true
        if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
            restorecon -v "$PRIVILEGED_HELPER_BINARY_PATH" >/dev/null 2>&1 || true
        fi
        if [[ "$ACTION_RUNNER_ENABLED" == "true" ]]; then
            restorecon -v "$ACTION_RUNNER_BINARY_PATH" >/dev/null 2>&1 || true
        fi
        log_info "SELinux context restored"
    else
        # Fallback to chcon if restorecon isn't available
        if command -v chcon >/dev/null 2>&1; then
            log_info "Setting SELinux context for installed binary..."
            chcon -t bin_t "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
            if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
                chcon -t bin_t "$PRIVILEGED_HELPER_BINARY_PATH" 2>/dev/null || true
            fi
            if [[ "$ACTION_RUNNER_ENABLED" == "true" ]]; then
                chcon -t bin_t "$ACTION_RUNNER_BINARY_PATH" 2>/dev/null || true
            fi
        fi
    fi
}

# --- Post-Start Health Verification ---
# After starting the agent service, poll its readiness endpoint to verify it
# actually started. The agent exposes /readyz on the configured health address
# once modules are initialized. The default is 127.0.0.1:9191.
# warn_agent_token_rejected surfaces the actionable recovery path when the
# server rejects the agent's token. Keeps the message in one place so both
# health-check paths report it identically.
warn_agent_token_rejected() {
    log_warn "Pulse rejected this agent's API token or required reporting scope (HTTP 401/403). The saved credential cannot authenticate this agent on the server, which usually means Pulse was restored/reinstalled or upgraded (for example v5 -> v6) without carrying the token across."
    log_warn "Re-run the full agent install command from the Pulse UI to mint a fresh token. The agent will keep reporting 401/403 until the credential is replaced."
}

# Resolve the newly downloaded/installed pulse-agent command surface used for
# authenticated installer lifecycle operations. Safe-profile preflight runs
# after download but before replacement, so it must prefer the new binary over
# an older installed agent that does not yet expose these commands.
collector_lifecycle_binary() {
    if [[ -n "${COLLECTOR_LIFECYCLE_BINARY_PATH:-}" && -x "$COLLECTOR_LIFECYCLE_BINARY_PATH" ]]; then
        printf '%s\n' "$COLLECTOR_LIFECYCLE_BINARY_PATH"
        return 0
    fi
    if [[ -n "${TMP_BIN:-}" && -x "$TMP_BIN" ]]; then
        printf '%s\n' "$TMP_BIN"
        return 0
    fi
    if [[ -x "${INSTALL_DIR%/}/${BINARY_NAME}" ]]; then
        printf '%s\n' "${INSTALL_DIR%/}/${BINARY_NAME}"
        return 0
    fi
    return 1
}

resolve_safe_profile_hostname() {
    local resolved_hostname="${HOSTNAME_OVERRIDE:-}"
    if [[ -z "$resolved_hostname" ]]; then
        resolved_hostname=$(hostname -f 2>/dev/null || true)
    fi
    if [[ -z "$resolved_hostname" ]]; then
        resolved_hostname=$(hostname 2>/dev/null || true)
    fi
    resolved_hostname=$(printf '%s' "$resolved_hostname" | tr '[:upper:]' '[:lower:]')
    resolved_hostname="${resolved_hostname%.}"
    [[ ${#resolved_hostname} -ge 1 && ${#resolved_hostname} -le 253 &&
       "$resolved_hostname" =~ ^[a-z0-9][a-z0-9._:-]*$ ]] || return 1
    HOSTNAME_OVERRIDE="$resolved_hostname"
}

# Select the bearer actually used by the collector without copying it through
# argv. Enrolled runtime state wins over the bootstrap token. PULSE_TOKEN-only
# installs get a root-only temporary file that is removed after each command.
prepare_collector_lifecycle_token_file() {
    local candidate=""
    local temp_token=""

    COLLECTOR_LIFECYCLE_TOKEN_FILE=""
    COLLECTOR_LIFECYCLE_TEMP_TOKEN_FILE=""
    for candidate in \
        "${STATE_DIR%/}/runtime.token" \
        "${RUNTIME_TOKEN_FILE:-}" \
        "${STATE_DIR%/}/token"; do
        [[ -n "$candidate" ]] || continue
        if [[ -s "$candidate" && -f "$candidate" && ! -L "$candidate" ]]; then
            COLLECTOR_LIFECYCLE_TOKEN_FILE="$candidate"
            return 0
        fi
    done

    [[ -n "$PULSE_TOKEN" && "$PULSE_TOKEN" != *$'\r'* && "$PULSE_TOKEN" != *$'\n'* ]] || return 1
    temp_token=$(mktemp) || return 1
    chmod 0600 "$temp_token" || { rm -f "$temp_token"; return 1; }
    if ! printf '%s' "$PULSE_TOKEN" > "$temp_token"; then
        rm -f "$temp_token"
        return 1
    fi
    COLLECTOR_LIFECYCLE_TOKEN_FILE="$temp_token"
    COLLECTOR_LIFECYCLE_TEMP_TOKEN_FILE="$temp_token"
    return 0
}

run_collector_lifecycle_command() {
    local command_name="$1"
    shift
    local lifecycle_binary=""
    local collector_uid=""
    local lifecycle_rc=1
    local -a lifecycle_args

    lifecycle_binary=$(collector_lifecycle_binary) || return 1
    prepare_collector_lifecycle_token_file || return 1
    lifecycle_args=("$command_name" --url "$PULSE_URL" --token-file "$COLLECTOR_LIFECYCLE_TOKEN_FILE")
    collector_uid=$(id -u "$LEAST_PRIVILEGE_USER" 2>/dev/null || true)
    if [[ "$collector_uid" =~ ^[0-9]+$ ]]; then
        lifecycle_args+=(--token-owner-uid "$collector_uid")
    fi
    [[ -n "$CURL_CA_BUNDLE" ]] && lifecycle_args+=(--cacert "$CURL_CA_BUNDLE")
    [[ -n "$SERVER_FINGERPRINT" ]] && lifecycle_args+=(--server-fingerprint "$SERVER_FINGERPRINT")
    lifecycle_args+=("$@")

    if "$lifecycle_binary" "${lifecycle_args[@]}"; then
        lifecycle_rc=0
    else
        lifecycle_rc=$?
    fi
    if [[ -n "$COLLECTOR_LIFECYCLE_TEMP_TOKEN_FILE" ]]; then
        rm -f "$COLLECTOR_LIFECYCLE_TEMP_TOKEN_FILE"
    fi
    COLLECTOR_LIFECYCLE_TOKEN_FILE=""
    COLLECTOR_LIFECYCLE_TEMP_TOKEN_FILE=""
    return "$lifecycle_rc"
}

# verify_agent_server_registration returns:
#   0 - the server confirmed this agent's registration
#   1 - registration not confirmed yet (transient: agent not reported, network)
#   2 - the server rejected the credential (401, or 403 other than a stale
#       hostname ownership match - actionable, permanent)
verify_agent_server_registration() {
    local required_previous_last_seen="${1:-}"
    local lookup_id="${AGENT_ID}"
    local lookup_hostname="${HOSTNAME_OVERRIDE}"
    local lookup_last_seen=""
    local lookup_rc=1
    local -a lookup_args=(collector-verify-registration)

    if [[ -z "$PULSE_URL" ]]; then
        return 1
    fi
    if [[ -z "$lookup_id" ]] && declare -F recover_agent_id_from_state_file >/dev/null 2>&1; then
        lookup_id=$(recover_agent_id_from_state_file || true)
        if [[ -n "$lookup_id" ]]; then
            AGENT_ID="$lookup_id"
        fi
    fi
    if [[ -z "$lookup_id" && -z "$lookup_hostname" ]]; then
        lookup_hostname=$(hostname 2>/dev/null || true)
    fi
    if [[ -n "$lookup_id" ]]; then
        lookup_args+=(--agent-id "$lookup_id")
    fi
    if [[ -n "$lookup_hostname" ]]; then
        lookup_args+=(--hostname "$lookup_hostname")
    fi
    if [[ ${#lookup_args[@]} -eq 1 ]]; then
        return 1
    fi
    [[ -n "$required_previous_last_seen" ]] && lookup_args+=(--previous-last-seen "$required_previous_last_seen")
    AGENT_REGISTRATION_LAST_SEEN=""
    if lookup_last_seen=$(run_collector_lifecycle_command "${lookup_args[@]}" 2>/dev/null); then
        lookup_rc=0
    else
        lookup_rc=$?
    fi
    if [[ $lookup_rc -eq 0 && -n "$lookup_last_seen" ]]; then
        AGENT_REGISTRATION_LAST_SEEN="$lookup_last_seen"
        return 0
    fi
    [[ $lookup_rc -eq 2 ]] && return 2
    return 1
}

# verify_agent_server_registration_with_retry polls the server-side lookup for
# a short window before declaring registration unconfirmed. The local /readyz
# endpoint flips before the agent's first report cycle completes, so a single
# immediate lookup routinely misses a perfectly healthy registration (#1644).
# Return codes mirror verify_agent_server_registration.
verify_agent_server_registration_with_retry() {
    local required_previous_last_seen="${1:-}"
    local max_attempts=10
    local interval=3
    local attempt=0
    local reg_rc=1

    if [[ -z "$PULSE_URL" ]]; then
        return 1
    fi

    while [ $attempt -lt $max_attempts ]; do
        verify_agent_server_registration "$required_previous_last_seen"
        reg_rc=$?
        # 0 = confirmed; 2 = token rejected, which is definitive and will not
        # change with more polling.
        if [[ $reg_rc -eq 0 || $reg_rc -eq 2 ]]; then
            return $reg_rc
        fi
        attempt=$((attempt + 1))
        if [ $attempt -lt $max_attempts ]; then
            sleep $interval
        fi
    done
    return 1
}

resolve_agent_health_url() {
    if [[ "$HEALTH_ADDR_SET" == "true" && -z "$HEALTH_ADDR" ]]; then
        return 1
    fi

    local addr="${HEALTH_ADDR:-127.0.0.1:9191}"
    local ipv6_any_prefix="[::]:"
    case "$addr" in
        :*) addr="127.0.0.1${addr}" ;;
        0.0.0.0:*) addr="127.0.0.1:${addr#0.0.0.0:}" ;;
    esac
    if [[ "$addr" == "$ipv6_any_prefix"* ]]; then
        addr="[::1]:${addr#$ipv6_any_prefix}"
    fi

    printf 'http://%s/readyz\n' "$addr"
}

agent_process_running() {
    if command -v pgrep >/dev/null 2>&1; then
        # Use -x (exact match) if supported, otherwise fall back to -f.
        pgrep -x "${BINARY_NAME}" >/dev/null 2>&1
        local pgrep_rc=$?
        if [ $pgrep_rc -eq 0 ]; then
            return 0
        elif [ $pgrep_rc -ge 2 ]; then
            pgrep -f "${BINARY_NAME}" >/dev/null 2>&1 && return 0
        fi
    else
        # shellcheck disable=SC2009
        # Use bracket trick ([p]ulse-agent) to prevent grep from matching itself.
        local grep_pattern="[${BINARY_NAME:0:1}]${BINARY_NAME:1}"
        if ps -e -o comm= 2>/dev/null | grep -q "$grep_pattern" || ps aux 2>/dev/null | grep -q "$grep_pattern"; then
            return 0
        fi
    fi

    return 1
}

verify_agent_started() {
    local health_url=""
    local max_iterations=8
    local interval=2
    local iteration=0
    local log_file="${AGENT_LOG_FILE:-${TRUENAS_LOG_FILE:-$LOG_FILE}}"

    log_info "Verifying agent started successfully..."

    # Brief pause to let the agent process spawn (especially for background starts like Unraid)
    sleep 2

    health_url="$(resolve_agent_health_url || true)"
    if [[ -z "$health_url" ]]; then
        while [ $iteration -lt $max_iterations ]; do
            if agent_process_running; then
                verify_agent_server_registration_with_retry
                local reg_rc=$?
                if [[ $reg_rc -eq 0 ]]; then
                    log_info "Agent process is running and registered with Pulse."
                elif [[ $reg_rc -eq 2 ]]; then
                    warn_agent_token_rejected
                    return 2
                else
                    log_warn "Agent process is running, but server registration was not confirmed yet."
                fi
                return 0
            fi
            sleep $interval
            iteration=$((iteration + 1))
        done

        log_warn "Agent process is not running!"
        if [ -f "$log_file" ]; then
            log_warn "Last log lines:"
            tail -5 "$log_file" 2>/dev/null | while IFS= read -r line; do log_warn "  $line"; done
        fi
        return 1
    fi

    while [ $iteration -lt $max_iterations ]; do
        # Check the readiness endpoint first — this is the definitive signal
        if curl -sf --max-time 2 "$health_url" >/dev/null 2>&1; then
            verify_agent_server_registration_with_retry
            local reg_rc=$?
            if [[ $reg_rc -eq 0 ]]; then
                log_info "Agent is running, healthy, and registered with Pulse."
            elif [[ $reg_rc -eq 2 ]]; then
                warn_agent_token_rejected
                return 2
            else
                log_warn "Agent local health is ready, but server registration was not confirmed yet."
            fi
            return 0
        fi

        # If curl failed, check whether the process is still alive.
        # Use pgrep where available, fall back to ps + grep.
        local agent_running=false
        if agent_process_running; then
            agent_running=true
        fi

        if [ "$agent_running" = "false" ] && [ $iteration -ge 3 ]; then
            # Only treat missing process as failure after ~8s — on Unraid the wrapper
            # script takes several seconds before the actual binary launches.
            log_warn "Agent process is not running!"
            # Show last few log lines for diagnostics
            if [ -f "$log_file" ]; then
                log_warn "Last log lines:"
                tail -5 "$log_file" 2>/dev/null | while IFS= read -r line; do log_warn "  $line"; done
            fi
            return 1
        fi

        sleep $interval
        iteration=$((iteration + 1))
    done

    # Timed out — process alive but not ready
    log_warn "Agent process is running but did not become ready within ~$((max_iterations * interval + 2))s."
    log_warn "It may still be initializing. Check logs: tail -f $log_file"
    return 1
}

stop_existing_agent_service() {
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet "${AGENT_NAME}" 2>/dev/null; then
            log_info "Stopping existing ${AGENT_NAME} service..."
            systemctl stop "${AGENT_NAME}" 2>/dev/null || true
            sleep 2
            return 0
        fi
    elif command -v rc-service >/dev/null 2>&1; then
        if rc-service "${AGENT_NAME}" status >/dev/null 2>&1; then
            log_info "Stopping existing ${AGENT_NAME} service..."
            rc-service "${AGENT_NAME}" stop 2>/dev/null || true
            sleep 2
            return 0
        fi
    elif command -v service >/dev/null 2>&1; then
        if service "${AGENT_NAME}" status >/dev/null 2>&1; then
            log_info "Stopping existing ${AGENT_NAME} service..."
            service "${AGENT_NAME}" stop 2>/dev/null || true
            sleep 2
            return 0
        fi
    fi

    return 1
}

restart_systemd_agent_service() {
    systemctl daemon-reload
    systemctl enable "${AGENT_NAME}" 2>/dev/null || true
    systemctl restart "${AGENT_NAME}"
}

restart_openrc_agent_service() {
    rc-service "${AGENT_NAME}" stop 2>/dev/null || true
    rc-update add "${AGENT_NAME}" default 2>/dev/null || true
    rc-service "${AGENT_NAME}" start
}

restart_service_command_agent() {
    service "${AGENT_NAME}" stop 2>/dev/null || true
    sleep 1
    service "${AGENT_NAME}" start 2>/dev/null || true
}

restart_sysv_agent_service() {
    local initscript="$1"
    "$initscript" stop 2>/dev/null || true
    sleep 1
    "$initscript" start
}

teardown_systemd_agent_service() {
    local unit_path="${1:-/etc/systemd/system/${AGENT_NAME}.service}"
    systemctl stop "${AGENT_NAME}" 2>/dev/null || true
    systemctl disable "${AGENT_NAME}" 2>/dev/null || true
    rm -f "$unit_path"
    systemctl daemon-reload 2>/dev/null || true
}

teardown_privileged_helper_service() {
    if ! command -v systemctl >/dev/null 2>&1; then
        return 0
    fi
    systemctl stop "${PRIVILEGED_HELPER_NAME}.socket" 2>/dev/null || true
    systemctl stop "${PRIVILEGED_HELPER_NAME}.service" 2>/dev/null || true
    systemctl disable "${PRIVILEGED_HELPER_NAME}.socket" 2>/dev/null || true
    rm -f "$PRIVILEGED_HELPER_SOCKET_UNIT" "$PRIVILEGED_HELPER_SERVICE_UNIT"
    rm -f "$PRIVILEGED_HELPER_SOCKET_PATH"
    rm -rf "$PRIVILEGED_HELPER_CREDENTIAL_DIR"
    systemctl daemon-reload 2>/dev/null || true
    systemctl reset-failed "${PRIVILEGED_HELPER_NAME}.service" 2>/dev/null || true
}

read_action_runner_env_value() {
    local key="$1"
    local value=""
    [[ "$key" =~ ^[A-Z0-9_]+$ ]] || return 1
    [[ -f "$ACTION_RUNNER_ENV_FILE" && ! -L "$ACTION_RUNNER_ENV_FILE" ]] || return 1
    value=$(sed -n "s/^${key}=\"\(.*\)\"$/\1/p" "$ACTION_RUNNER_ENV_FILE" | tail -1)
    # Values emitted by this installer escape backslashes and quotes. The
    # revoke contract only needs URL, hostname, and an absolute state path;
    # refuse an escaped value instead of evaluating shell syntax to recover it.
    [[ -n "$value" && "$value" != *'\'* && "$value" != *$'\r'* && "$value" != *$'\n'* ]] || return 1
    printf '%s\n' "$value"
}

revoke_action_runner_credential() {
    local runner_url=""
    local runner_hostname=""
    local runner_agent_id_direct=""
    local runner_agent_id_file=""
    local runner_agent_id=""
	local runner_token_file=""
	local runner_server_fingerprint=""
	local runner_ca_file=""
	local runner_insecure=""
	local lifecycle_binary=""
	local collector_uid=""
	local -a identity_args
	local -a revoke_args

    runner_url=$(read_action_runner_env_value "PULSE_URL" || true)
    runner_hostname=$(read_action_runner_env_value "PULSE_AGENT_RUNNER_HOSTNAME" || true)
    runner_agent_id_direct=$(read_action_runner_env_value "PULSE_AGENT_RUNNER_AGENT_ID" || true)
    runner_agent_id_file=$(read_action_runner_env_value "PULSE_AGENT_RUNNER_AGENT_ID_FILE" || true)
    runner_token_file=$(read_action_runner_env_value "PULSE_AGENT_RUNNER_TOKEN_FILE" || true)
	runner_server_fingerprint=$(read_action_runner_env_value "PULSE_SERVER_FINGERPRINT" || true)
	runner_ca_file=$(read_action_runner_env_value "SSL_CERT_FILE" || true)
	runner_insecure=$(read_action_runner_env_value "PULSE_INSECURE" || true)
	action_runner_url_transport_allowed "$runner_url" || return 1
	[[ -n "$runner_hostname" ]] || return 1
	[[ -x "$ACTION_RUNNER_BINARY_PATH" ]] || return 1
    [[ "$runner_token_file" == /* && "$runner_token_file" != *'/../'* &&
       -f "$runner_token_file" && ! -L "$runner_token_file" ]] || return 1
    if [[ -n "$runner_agent_id_direct" ]]; then
        runner_agent_id="$runner_agent_id_direct"
    else
        [[ "$runner_agent_id_file" == /* && "$runner_agent_id_file" != *'/../'* &&
           -f "$runner_agent_id_file" && ! -L "$runner_agent_id_file" ]] || return 1
		lifecycle_binary=$(collector_lifecycle_binary) || return 1
		identity_args=(collector-read-agent-id --agent-id-file "$runner_agent_id_file")
		collector_uid=$(id -u "$LEAST_PRIVILEGE_USER" 2>/dev/null || true)
		if [[ "$collector_uid" =~ ^[0-9]+$ ]]; then
			identity_args+=(--token-owner-uid "$collector_uid")
		fi
		runner_agent_id=$("$lifecycle_binary" "${identity_args[@]}" 2>/dev/null || true)
    fi
    (( ${#runner_agent_id} >= 1 && ${#runner_agent_id} <= 128 &&
       ${#runner_hostname} >= 1 && ${#runner_hostname} <= 253 )) || return 1
    [[ "$runner_agent_id" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]*$ &&
       "$runner_hostname" =~ ^[A-Za-z0-9._:-]+$ ]] || return 1
	revoke_args=(revoke-credential --url "$runner_url" --token-file "$runner_token_file" --agent-id "$runner_agent_id" --hostname "$runner_hostname")
	[[ -n "$runner_ca_file" ]] && revoke_args+=(--cacert "$runner_ca_file")
	[[ -n "$runner_server_fingerprint" ]] && revoke_args+=(--server-fingerprint "$runner_server_fingerprint")
	if action_runner_url_uses_loopback_http "$runner_url" && [[ "$runner_insecure" == "true" ]]; then
		revoke_args+=(--insecure-loopback)
	fi
	"$ACTION_RUNNER_BINARY_PATH" "${revoke_args[@]}"
}

reduce_safe_profile_collector_authority() {
    [[ -n "$AGENT_ID" && ${#AGENT_ID} -le 256 && "$AGENT_ID" =~ ^[A-Za-z0-9._:-]+$ ]] || return 1
    [[ -n "$HOSTNAME_OVERRIDE" && ${#HOSTNAME_OVERRIDE} -le 253 && "$HOSTNAME_OVERRIDE" =~ ^[A-Za-z0-9._:-]+$ ]] || return 1
    if run_collector_lifecycle_command collector-reduce-authority \
        --agent-id "$AGENT_ID" --hostname "$HOSTNAME_OVERRIDE" >/dev/null 2>&1; then
        log_info "Durably removed execution and cross-host management scopes from the collector credential before migration."
        return 0
    fi
    return 1
}

teardown_action_runner_service() {
	local had_runner_artifact="false"
	if [[ -e "$ACTION_RUNNER_BINARY_PATH" || -e "$ACTION_RUNNER_SERVICE_UNIT" ||
	      -e "$ACTION_RUNNER_CONFIG_DIR" || -e "$ACTION_RUNNER_STATE_DIR" ]]; then
		had_runner_artifact="true"
	fi
	if command -v systemctl >/dev/null 2>&1; then
		systemctl stop "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
		systemctl disable "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
	fi
	if [[ "$had_runner_artifact" == "true" ]]; then
		if revoke_action_runner_credential; then
			log_info "Revoked the action-runner credential before removing local runner recovery material."
		else
			log_error "Could not confirm action-runner credential revocation. The runner is stopped and disabled; every local artifact was retained for a safe retry or manual server-side revoke."
			fail "Action runner removal requires a successful credential revocation; retry with the exact root-only credential, or revoke it in Pulse before manual cleanup" "$EXIT_GENERAL"
		fi
	fi
	rm -f "$ACTION_RUNNER_SERVICE_UNIT"
	if command -v systemctl >/dev/null 2>&1; then
		systemctl daemon-reload 2>/dev/null || true
		systemctl reset-failed "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
	fi
	rm -f "$ACTION_RUNNER_BINARY_PATH"
	rm -rf "$ACTION_RUNNER_CONFIG_DIR" "$ACTION_RUNNER_STATE_DIR"
}

teardown_openrc_agent_service() {
    local init_path="${1:-/etc/init.d/${AGENT_NAME}}"
    rc-service "${AGENT_NAME}" stop 2>/dev/null || true
    rc-update del "${AGENT_NAME}" default 2>/dev/null || true
    rm -f "$init_path"
}

teardown_freebsd_agent_service() {
    local service_path="${1:-/usr/local/etc/rc.d/${AGENT_NAME}}"

    # Stop the rc.d supervisor before removing the executable. Killing only the
    # child is insufficient because daemon(8) immediately restarts it.
    if [[ -x "$service_path" ]]; then
        "$service_path" stop 2>/dev/null || true
    elif command -v service >/dev/null 2>&1; then
        service "${AGENT_NAME}" stop 2>/dev/null || true
    fi

    if command -v sysrc >/dev/null 2>&1; then
        sysrc -x pulse_agent_enable >/dev/null 2>&1 || true
    else
        for rc_config in /etc/rc.conf /etc/rc.conf.local; do
            if [[ -f "$rc_config" ]]; then
                sed -i '' '/^[[:space:]]*pulse_agent_enable[[:space:]]*=/d' "$rc_config" 2>/dev/null || \
                    sed -i '/^[[:space:]]*pulse_agent_enable[[:space:]]*=/d' "$rc_config" 2>/dev/null || true
            fi
        done
    fi

    rm -f "$service_path"
    rm -f /usr/local/etc/rc.d/pulse_agent.sh
    rm -f /var/run/pulse_agent.pid /var/run/pulse_agent.child.pid
}

teardown_sysv_agent_service() {
    local init_path="${1:-/etc/init.d/${AGENT_NAME}}"
    "$init_path" stop 2>/dev/null || true
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d -f "${AGENT_NAME}" remove >/dev/null 2>&1 || true
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig "${AGENT_NAME}" off >/dev/null 2>&1 || true
        chkconfig --del "${AGENT_NAME}" >/dev/null 2>&1 || true
    fi
    for RL in 0 1 2 3 4 5 6; do
        rm -f "/etc/rc${RL}.d/S99${AGENT_NAME}" 2>/dev/null || true
        rm -f "/etc/rc${RL}.d/K01${AGENT_NAME}" 2>/dev/null || true
    done
    rm -f "$init_path"
    rm -f "/var/run/${AGENT_NAME}.pid"
}

enable_sysv_agent_service() {
    local init_path="${1:-/etc/init.d/${AGENT_NAME}}"
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d "${AGENT_NAME}" defaults >/dev/null 2>&1 || true
        log_info "Enabled service with update-rc.d."
        return 0
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add "${AGENT_NAME}" >/dev/null 2>&1 || true
        chkconfig "${AGENT_NAME}" on >/dev/null 2>&1 || true
        log_info "Enabled service with chkconfig."
        return 0
    fi

    for RL in 2 3 4 5; do
        if [[ -d "/etc/rc${RL}.d" ]]; then
            ln -sf "$init_path" "/etc/rc${RL}.d/S99${AGENT_NAME}" 2>/dev/null || true
        fi
    done
    for RL in 0 1 6; do
        if [[ -d "/etc/rc${RL}.d" ]]; then
            ln -sf "$init_path" "/etc/rc${RL}.d/K01${AGENT_NAME}" 2>/dev/null || true
        fi
    done
    log_info "Created rc.d symlinks manually."
}

write_truenas_bootstrap_script() {
    local platform="$1"
    local service_link=""
    local service_management_functions=""

    if [[ "$platform" == "Linux" ]]; then
        service_link="/etc/systemd/system/${AGENT_NAME}.service"
        service_management_functions=$(cat <<'EOF'
start_agent_service() {
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" 2>/dev/null || true
    systemctl restart "$SERVICE_NAME"
}
EOF
)
    else
        service_link="/usr/local/etc/rc.d/${AGENT_NAME}"
        service_management_functions="$(freebsd_enable_snippet)

start_agent_service() {
    ensure_freebsd_agent_enabled
    service \"\${SERVICE_NAME}\" stop 2>/dev/null || true
    sleep 1
    service \"\${SERVICE_NAME}\" start 2>/dev/null || true
}"
    fi

    cat > "$TRUENAS_BOOTSTRAP_SCRIPT" <<BOOTSTRAP
#!/bin/bash
# Pulse Agent Bootstrap for TrueNAS
# Called by TrueNAS Init/Shutdown task on boot.

set -e

SERVICE_NAME="${AGENT_NAME}"
STATE_DIR="${TRUENAS_STATE_DIR}"
STORED_BINARY="\${STATE_DIR}/pulse-agent"
RUNTIME_BINARY="${TRUENAS_RUNTIME_BINARY}"
SERVICE_STORAGE="\${STATE_DIR}/pulse-agent.service"
SERVICE_LINK="${service_link}"

require_bootstrap_file() {
    local path="\$1"
    local label="\$2"
    if [[ ! -f "\$path" ]]; then
        echo "ERROR: \$label not found at \$path"
        exit 1
    fi
}

sync_runtime_binary() {
    if [[ "\$RUNTIME_BINARY" == "\$STORED_BINARY" ]]; then
        return 0
    fi

    mkdir -p "\$(dirname "\$RUNTIME_BINARY")" 2>/dev/null || true
    cp "\$STORED_BINARY" "\$RUNTIME_BINARY"
    chmod +x "\$RUNTIME_BINARY"
}

link_service_artifact() {
    ln -sf "\$SERVICE_STORAGE" "\$SERVICE_LINK"
}

${service_management_functions}

require_bootstrap_file "\$STORED_BINARY" "Binary"
require_bootstrap_file "\$SERVICE_STORAGE" "Service file"
sync_runtime_binary
link_service_artifact
start_agent_service

echo "Pulse agent started successfully"
BOOTSTRAP

    chmod +x "$TRUENAS_BOOTSTRAP_SCRIPT"
}

freebsd_enable_snippet() {
    cat <<'EOF'
apply_freebsd_agent_enablement() {
    if ! grep -q "pulse_agent_enable" /etc/rc.conf 2>/dev/null; then
        echo 'pulse_agent_enable="YES"' >> /etc/rc.conf
    else
        sed -i '' 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf 2>/dev/null || \
            sed -i 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf
    fi
}
EOF
}

ensure_freebsd_agent_enabled() {
    eval "$(freebsd_enable_snippet)"
    apply_freebsd_agent_enablement
}

render_privileged_helper_socket_unit() {
    local unit_path="$1"

    cat > "$unit_path" <<EOF
[Unit]
Description=Pulse Agent typed privileged helper socket

[Socket]
ListenStream=${PRIVILEGED_HELPER_SOCKET_PATH}
SocketUser=root
SocketGroup=${LEAST_PRIVILEGE_USER}
SocketMode=0660
DirectoryMode=0755
RemoveOnStop=true

[Install]
WantedBy=sockets.target
EOF
}

render_privileged_helper_service_unit() {
    local unit_path="$1"
    local helper_binary="$2"

    cat > "$unit_path" <<EOF
[Unit]
Description=Pulse Agent typed privileged helper
Requires=${PRIVILEGED_HELPER_NAME}.socket
After=${PRIVILEGED_HELPER_NAME}.socket

[Service]
Type=simple
ExecStart=${helper_binary}
User=root
Group=root
UMask=0077
NoNewPrivileges=true
PrivateNetwork=true
RestrictAddressFamilies=AF_UNIX
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
LockPersonality=true
RestrictSUIDSGID=true
SystemCallArchitectures=native
TasksMax=64
LimitNOFILE=256
MemoryMax=256M
ReadOnlyPaths=${PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR}
ReadWritePaths=${PRIVILEGED_HELPER_STATE_DIR} /usr/local/bin
EOF
}

render_action_runner_service_unit() {
    local unit_path="$1"
    local runner_binary="$2"

    cat > "$unit_path" <<EOF
[Unit]
Description=Pulse Agent typed action runner
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=${runner_binary}
EnvironmentFile=${ACTION_RUNNER_ENV_FILE}
User=root
Group=root
UMask=0077
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=false
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
LockPersonality=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
SystemCallArchitectures=native
ReadWritePaths=${ACTION_RUNNER_STATE_DIR}

[Install]
WantedBy=multi-user.target
EOF
}

write_action_runner_config() {
    local runner_hostname="$HOSTNAME_OVERRIDE"
    local runner_agent_id=""
    if [[ -L "$ACTION_RUNNER_CONFIG_DIR" || -L "$ACTION_RUNNER_STATE_DIR" ||
          -L "$ACTION_RUNNER_TOKEN_FILE" || -L "$ACTION_RUNNER_ENV_FILE" ]]; then
        fail "Refusing unsafe symlink in the action-runner config or state boundary" "$EXIT_GENERAL"
    fi
    install -d -o root -g root -m 0700 "$ACTION_RUNNER_CONFIG_DIR"
    install -d -o root -g root -m 0700 "$ACTION_RUNNER_STATE_DIR"
	action_runner_url_transport_allowed "$PULSE_URL" ||
		fail "Action runner requires HTTPS/WSS; plaintext HTTP/WS is allowed only for loopback local use" "$EXIT_MISSING_ARGS"
	if [[ "$INSECURE" == "true" && "$PULSE_URL" =~ ^[Hh][Tt][Tt][Pp][Ss]:// &&
	      -z "$CURL_CA_BUNDLE" && -z "$SERVER_FINGERPRINT" ]]; then
		fail "Action runner refuses generic insecure HTTPS; configure a trusted CA bundle or exact server fingerprint" "$EXIT_MISSING_ARGS"
	fi

    if [[ -n "$ACTION_TOKEN" ]]; then
        printf '%s\n' "$ACTION_TOKEN" > "$ACTION_RUNNER_TOKEN_FILE"
    elif [[ ! -s "$ACTION_RUNNER_TOKEN_FILE" ]]; then
        fail "--enable-action-runner requires a separate --action-token-file on first install" "$EXIT_MISSING_ARGS"
    fi
    chown root:root "$ACTION_RUNNER_TOKEN_FILE"
    chmod 0600 "$ACTION_RUNNER_TOKEN_FILE"
    ACTION_TOKEN=""

    if [[ -z "$runner_hostname" ]]; then
        runner_hostname=$(hostname 2>/dev/null || true)
    fi
	[[ -n "$runner_hostname" && "$runner_hostname" != *$'\r'* && "$runner_hostname" != *$'\n'* ]] ||
		fail "Action runner requires a canonical hostname" "$EXIT_MISSING_ARGS"
	[[ "$ACTION_RUNNER_ACTIVATION_NONCE" =~ ^[a-f0-9]{64}$ ]] ||
		fail "Action runner requires a fresh activation nonce" "$EXIT_GENERAL"
	runner_agent_id=$(resolve_action_runner_agent_id) ||
		fail "Action runner requires a safely resolved canonical collector identity" "$EXIT_MISSING_ARGS"
	AGENT_ID="$runner_agent_id"

    : > "$ACTION_RUNNER_ENV_FILE"
    chmod 0600 "$ACTION_RUNNER_ENV_FILE"
    write_action_runner_env_value "PULSE_URL" "$PULSE_URL"
    write_action_runner_env_value "PULSE_AGENT_RUNNER_TOKEN_FILE" "$ACTION_RUNNER_TOKEN_FILE"
    write_action_runner_env_value "PULSE_AGENT_RUNNER_STATE_DIR" "$ACTION_RUNNER_STATE_DIR"
	write_action_runner_env_value "PULSE_AGENT_RUNNER_HEALTH_FILE" "$ACTION_RUNNER_HEALTH_FILE"
	write_action_runner_env_value "PULSE_AGENT_RUNNER_ACTIVATION_NONCE" "$ACTION_RUNNER_ACTIVATION_NONCE"
	write_action_runner_env_value "PULSE_AGENT_RUNNER_AGENT_ID" "$runner_agent_id"
    write_action_runner_env_value "PULSE_AGENT_RUNNER_HOSTNAME" "$runner_hostname"
    if [[ -n "$SERVER_FINGERPRINT" ]]; then
        write_action_runner_env_value "PULSE_SERVER_FINGERPRINT" "$SERVER_FINGERPRINT"
    fi
    if [[ -n "$CURL_CA_BUNDLE" ]]; then
        write_action_runner_env_value "SSL_CERT_FILE" "$CURL_CA_BUNDLE"
    fi
    if [[ "$INSECURE" == "true" ]] && action_runner_url_uses_loopback_http "$PULSE_URL"; then
        write_action_runner_env_value "PULSE_INSECURE" "true"
    fi
    chown root:root "$ACTION_RUNNER_ENV_FILE"
}

generate_action_runner_activation_nonce() {
	local nonce=""
	nonce=$(od -An -N32 -tx1 /dev/urandom 2>/dev/null | tr -d '[:space:]' || true)
	[[ "$nonce" =~ ^[a-f0-9]{64}$ ]] || return 1
	printf '%s\n' "$nonce"
}

action_runner_health_matches_activation() {
	local expected_agent_id="$1"
	local expected_nonce="$2"
	local health_agent_id=""
	local health_activation_nonce=""
	local health_owner=""
	local health_mode=""

	[[ -n "$expected_agent_id" && "$expected_nonce" =~ ^[a-f0-9]{64}$ ]] || return 1
	[[ -f "$ACTION_RUNNER_HEALTH_FILE" && ! -L "$ACTION_RUNNER_HEALTH_FILE" ]] || return 1
	health_owner=$(stat -c '%u' "$ACTION_RUNNER_HEALTH_FILE" 2>/dev/null || true)
	health_mode=$(stat -c '%a' "$ACTION_RUNNER_HEALTH_FILE" 2>/dev/null || true)
	health_agent_id=$(sed -n 's/.*"host_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ACTION_RUNNER_HEALTH_FILE" | head -1)
	health_activation_nonce=$(sed -n 's/.*"activation_nonce"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ACTION_RUNNER_HEALTH_FILE" | head -1)
	[[ "$health_owner" == "0" && "$health_mode" =~ ^(400|600)$ ]] || return 1
	grep -Eq '"registered"[[:space:]]*:[[:space:]]*true([[:space:],}]|$)' "$ACTION_RUNNER_HEALTH_FILE" 2>/dev/null || return 1
	grep -Eq '"activated"[[:space:]]*:[[:space:]]*true([[:space:],}]|$)' "$ACTION_RUNNER_HEALTH_FILE" 2>/dev/null || return 1
	[[ "$health_agent_id" == "$expected_agent_id" && "$health_activation_nonce" == "$expected_nonce" ]]
}

# Print pending or active for the exact installed runner credential. Failure is
# intentionally indeterminate: callers must not restore a predecessor because
# an unreachable server may already have committed and revoked it.
action_runner_url_uses_loopback_http() {
	local raw_url="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
	local authority=""
	local host=""
	local octet=""
	local -a octets

	[[ "$raw_url" =~ ^http://[^[:space:]]+$ ]] || return 1
	authority="${raw_url#http://}"
	authority="${authority%%/*}"
	[[ -n "$authority" && "$authority" != *'@'* ]] || return 1
	if [[ "$authority" == \[* ]]; then
		host="${authority#\[}"
		host="${host%%\]*}"
		[[ "$authority" == "[${host}]" || "$authority" == "[${host}]:"* ]] || return 1
		[[ "$host" == "::1" ]]
		return
	fi
	[[ "$authority" != *:*:* ]] || return 1
	host="${authority%%:*}"
	host="${host%.}"
	if [[ "$host" == "localhost" ]]; then
		return 0
	fi
	[[ "$host" =~ ^127\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]] || return 1
	IFS='.' read -r -a octets <<< "$host"
	for octet in "${octets[@]}"; do
		(( 10#$octet <= 255 )) || return 1
	done
	return 0
}

action_runner_url_transport_allowed() {
	local raw_url="$1"
	local lower_url="$(printf '%s' "$raw_url" | tr '[:upper:]' '[:lower:]')"
	if [[ "$lower_url" =~ ^https://[^[:space:]]+$ && "$lower_url" != *'@'* ]]; then
		return 0
	fi
	action_runner_url_uses_loopback_http "$raw_url"
}

# Atomically cancel the exact pending replacement. Only the runner command's
# zero exit (server HTTP 204) authorizes predecessor restore. The bearer is
# written to a private temporary file and never appears in argv or curl.
cancel_pending_action_runner_credential() {
	local expected_agent_id="$1"
	local expected_hostname="$2"
	local replacement_token="$3"
	local token_tmp=""
	local old_umask=""
	local -a cancel_args

	[[ -x "$ACTION_RUNNER_BINARY_PATH" && -d "$ACTION_RUNNER_CONFIG_DIR" && ! -L "$ACTION_RUNNER_CONFIG_DIR" ]] || return 1
	[[ -n "$expected_agent_id" && -n "$expected_hostname" && -n "$replacement_token" ]] || return 1
	action_runner_url_transport_allowed "$PULSE_URL" || return 1
	old_umask=$(umask)
	umask 077
	token_tmp=$(mktemp "${ACTION_RUNNER_CONFIG_DIR%/}/.cancel-token.XXXXXX") || {
		umask "$old_umask"
		return 1
	}
	if ! printf '%s\n' "$replacement_token" > "$token_tmp" ||
	   ! chown root:root "$token_tmp" || ! chmod 0600 "$token_tmp" || ! sync -f "$token_tmp"; then
		rm -f "$token_tmp"
		umask "$old_umask"
		return 1
	fi
	umask "$old_umask"
	cancel_args=(cancel-pending-credential --url "$PULSE_URL" --token-file "$token_tmp")
	[[ -n "${CURL_CA_BUNDLE:-}" ]] && cancel_args+=(--cacert "$CURL_CA_BUNDLE")
	[[ -n "${SERVER_FINGERPRINT:-}" ]] && cancel_args+=(--server-fingerprint "$SERVER_FINGERPRINT")
	action_runner_url_uses_loopback_http "$PULSE_URL" && cancel_args+=(--insecure-loopback)
	if "$ACTION_RUNNER_BINARY_PATH" "${cancel_args[@]}"; then
		rm -f "$token_tmp"
		return 0
	fi
	rm -f "$token_tmp"
	return 1
}

persist_action_runner_replacement_token() {
	local replacement_token="$1"
	local token_dir="$(dirname "$ACTION_RUNNER_TOKEN_FILE")"
	local token_tmp=""
	local old_umask=""
	local token_owner=""
	local token_mode=""

	[[ -n "$replacement_token" && "$replacement_token" != *$'\r'* && "$replacement_token" != *$'\n'* ]] || return 1
	[[ "$ACTION_RUNNER_TOKEN_FILE" == "${ACTION_RUNNER_CONFIG_DIR%/}/"* &&
	   -d "$token_dir" && ! -L "$token_dir" ]] || return 1
	[[ ! -e "$ACTION_RUNNER_TOKEN_FILE" || ( -f "$ACTION_RUNNER_TOKEN_FILE" && ! -L "$ACTION_RUNNER_TOKEN_FILE" ) ]] || return 1
	old_umask=$(umask)
	umask 077
	token_tmp=$(mktemp "${token_dir}/.replacement-token.XXXXXX") || {
		umask "$old_umask"
		return 1
	}
	if ! printf '%s\n' "$replacement_token" > "$token_tmp" ||
	   ! chown root:root "$token_tmp" ||
	   ! chmod 0600 "$token_tmp" ||
	   ! sync -f "$token_tmp" ||
	   ! mv -f "$token_tmp" "$ACTION_RUNNER_TOKEN_FILE"; then
		rm -f "$token_tmp"
		umask "$old_umask"
		return 1
	fi
	token_tmp=""
	if ! sync -f "$token_dir"; then
		umask "$old_umask"
		return 1
	fi
	umask "$old_umask"
	token_owner=$(stat -c '%u' "$ACTION_RUNNER_TOKEN_FILE" 2>/dev/null || true)
	token_mode=$(stat -c '%a' "$ACTION_RUNNER_TOKEN_FILE" 2>/dev/null || true)
	[[ "$token_owner" == "0" && "$token_mode" == "600" &&
	   -f "$ACTION_RUNNER_TOKEN_FILE" && ! -L "$ACTION_RUNNER_TOKEN_FILE" ]]
}

write_action_runner_env_value() {
    local key="$1"
    local value="$2"
    if [[ "$value" == *$'\r'* || "$value" == *$'\n'* ]]; then
        fail "Refusing newline in action-runner environment value for ${key}" "$EXIT_MISSING_ARGS"
    fi
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf '%s="%s"\n' "$key" "$value" >> "$ACTION_RUNNER_ENV_FILE"
}

resolve_action_runner_agent_id() {
    local agent_id="${AGENT_ID:-}"
    local persisted_agent_id=""
    local lifecycle_binary=""
    local collector_uid=""
    local -a identity_args

    if [[ -z "$agent_id" && -f "${ACTION_RUNNER_ENV_FILE:-}" && ! -L "${ACTION_RUNNER_ENV_FILE:-}" ]]; then
        agent_id=$(read_action_runner_env_value "PULSE_AGENT_RUNNER_AGENT_ID" 2>/dev/null || true)
    fi
    if [[ -z "$agent_id" ]]; then
        lifecycle_binary=$(collector_lifecycle_binary) || return 1
        identity_args=(collector-read-agent-id --agent-id-file "${STATE_DIR%/}/agent-id")
        collector_uid=$(id -u "$LEAST_PRIVILEGE_USER" 2>/dev/null || true)
        if [[ "$collector_uid" =~ ^[0-9]+$ ]]; then
            identity_args+=(--token-owner-uid "$collector_uid")
        fi
        persisted_agent_id=$("$lifecycle_binary" "${identity_args[@]}" 2>/dev/null || true)
        agent_id="$persisted_agent_id"
    fi
    [[ ${#agent_id} -ge 1 && ${#agent_id} -le 128 && "$agent_id" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]*$ ]] || return 1
    printf '%s\n' "$agent_id"
}

provision_action_runner() {
    local backup_suffix=".pulse-install-backup.$$"
    local path=""
    local had_binary="false"
    local had_unit="false"
    local had_state_dir="false"
    local had_config_dir="false"
	local runner_active="false"
	local apply_succeeded="false"
	local activation_nonce=""
	local credential_replacement_requested="false"
	local replacement_action_token=""
	local expected_agent_id=""
	local expected_hostname="$HOSTNAME_OVERRIDE"

	if [[ -n "$ACTION_TOKEN" ]]; then
		credential_replacement_requested="true"
		replacement_action_token="$ACTION_TOKEN"
	fi
	if [[ -z "$expected_hostname" ]]; then
		expected_hostname=$(hostname 2>/dev/null || true)
	fi

    if [[ -d "$ACTION_RUNNER_STATE_DIR" ]]; then
        had_state_dir="true"
    fi
    if [[ -d "$ACTION_RUNNER_CONFIG_DIR" ]]; then
        had_config_dir="true"
    fi
	for path in "$ACTION_RUNNER_BINARY_PATH" "$ACTION_RUNNER_SERVICE_UNIT" "$ACTION_RUNNER_ENV_FILE" "$ACTION_RUNNER_TOKEN_FILE"; do
        if [[ -e "$path" ]]; then
            cp -a "$path" "${path}${backup_suffix}"
            case "$path" in
                "$ACTION_RUNNER_BINARY_PATH") had_binary="true" ;;
                "$ACTION_RUNNER_SERVICE_UNIT") had_unit="true" ;;
            esac
        fi
	done

	activation_nonce=$(generate_action_runner_activation_nonce) ||
		fail "Could not generate an action-runner activation nonce" "$EXIT_GENERAL"
	ACTION_RUNNER_ACTIVATION_NONCE="$activation_nonce"
	if (
		set -e
		rm -f "$ACTION_RUNNER_HEALTH_FILE"
        mkdir -p "$PRIVILEGE_HELPER_DIR"
        install -o root -g root -m 0755 "$TMP_ACTION_RUNNER_BIN" "${ACTION_RUNNER_BINARY_PATH}.new"
        mv "${ACTION_RUNNER_BINARY_PATH}.new" "$ACTION_RUNNER_BINARY_PATH"
        restore_selinux_contexts
        write_action_runner_config
        render_action_runner_service_unit "${ACTION_RUNNER_SERVICE_UNIT}.new" "$ACTION_RUNNER_BINARY_PATH"
        chown root:root "${ACTION_RUNNER_SERVICE_UNIT}.new"
        chmod 0644 "${ACTION_RUNNER_SERVICE_UNIT}.new"
        mv "${ACTION_RUNNER_SERVICE_UNIT}.new" "$ACTION_RUNNER_SERVICE_UNIT"
        systemctl daemon-reload
        action_runner_verify_effective_target
        systemctl enable "${ACTION_RUNNER_NAME}.service"
        systemctl restart "${ACTION_RUNNER_NAME}.service"
    ); then
        apply_succeeded="true"
    fi
    ACTION_TOKEN=""

    if [[ "$apply_succeeded" == "true" ]]; then
        local attempt=0
		while [[ "$attempt" -lt 30 ]]; do
			local expected_agent_id=""
			expected_agent_id=$(resolve_action_runner_agent_id || true)
			if systemctl is-active --quiet "${ACTION_RUNNER_NAME}.service" &&
			   action_runner_health_matches_activation "$expected_agent_id" "$activation_nonce"; then
                runner_active="true"
                break
            fi
            attempt=$((attempt + 1))
            sleep 1
        done
    fi

	if [[ "$runner_active" != "true" ]]; then
		systemctl stop "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
		if [[ "$credential_replacement_requested" == "true" ]]; then
			expected_agent_id=$(resolve_action_runner_agent_id || true)
			if ! cancel_pending_action_runner_credential "$expected_agent_id" "$expected_hostname" "$replacement_action_token"; then
				log_error "The server did not durably confirm cancellation of the pending action-runner credential. The predecessor cannot be restored because activation may already be committed."
				if ! persist_action_runner_replacement_token "$replacement_action_token"; then
					replacement_action_token=""
					log_error "Could not durably persist the exact replacement action-runner credential. The runner remains stopped, the predecessor was not restored, and action-runner re-enrollment is required."
					ACTION_RUNNER_ACTIVATION_NONCE=""
					fail "Action runner credential recovery requires re-enrollment; no predecessor credential was restored" "$EXIT_GENERAL"
				fi
				log_error "The exact replacement credential and runtime were retained durably; repair is required."
				replacement_action_token=""
				rm -f "${ACTION_RUNNER_BINARY_PATH}.new" "${ACTION_RUNNER_SERVICE_UNIT}.new"
				rm -f \
					"${ACTION_RUNNER_BINARY_PATH}${backup_suffix}" \
					"${ACTION_RUNNER_SERVICE_UNIT}${backup_suffix}" \
					"${ACTION_RUNNER_ENV_FILE}${backup_suffix}" \
					"${ACTION_RUNNER_TOKEN_FILE}${backup_suffix}"
				systemctl daemon-reload 2>/dev/null || true
				systemctl enable --now "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
				ACTION_RUNNER_ACTIVATION_NONCE=""
				fail "Action runner activation requires repair; the new credential and runtime were retained and the previous credential was not restored" "$EXIT_GENERAL"
			fi
			replacement_action_token=""
		fi
		log_error "New action runner did not become healthy before server activation committed; rolling back runner-only files while leaving monitoring active."
		systemctl disable "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
        rm -f "${ACTION_RUNNER_BINARY_PATH}.new" "${ACTION_RUNNER_SERVICE_UNIT}.new"
		rm -f "$ACTION_RUNNER_HEALTH_FILE"
		for path in "$ACTION_RUNNER_BINARY_PATH" "$ACTION_RUNNER_SERVICE_UNIT" "$ACTION_RUNNER_ENV_FILE" "$ACTION_RUNNER_TOKEN_FILE"; do
            rm -f "$path"
            if [[ -e "${path}${backup_suffix}" ]]; then
                mv "${path}${backup_suffix}" "$path"
            fi
        done
        if [[ "$had_state_dir" != "true" ]]; then
            rm -rf "$ACTION_RUNNER_STATE_DIR"
        fi
        if [[ "$had_config_dir" != "true" ]]; then
            rm -rf "$ACTION_RUNNER_CONFIG_DIR"
        fi
        systemctl daemon-reload 2>/dev/null || true
        if [[ "$had_unit" == "true" && "$had_binary" == "true" ]]; then
            systemctl enable --now "${ACTION_RUNNER_NAME}.service" 2>/dev/null || true
		fi
		ACTION_RUNNER_ACTIVATION_NONCE=""
		fail "Action runner activation failed and its previous installation was restored; collector monitoring was not stopped or removed" "$EXIT_GENERAL"
    fi

	replacement_action_token=""
    rm -f \
        "${ACTION_RUNNER_BINARY_PATH}${backup_suffix}" \
		"${ACTION_RUNNER_SERVICE_UNIT}${backup_suffix}" \
		"${ACTION_RUNNER_ENV_FILE}${backup_suffix}" \
		"${ACTION_RUNNER_TOKEN_FILE}${backup_suffix}"
	ACTION_RUNNER_ACTIVATION_NONCE=""
    TMP_ACTION_RUNNER_BIN=""
    log_info "Typed action runner enabled as a separate root service with its own credential; collector monitoring remains independently active."
}

# --- Safe collector profile migration transaction ---
# These operations intentionally cover only the collector and typed helper.
# The independently installed action runner has its own lifecycle and is never
# snapshotted, stopped, rewritten, or restored here.
safe_profile_platform_supported() {
    [[ "$(uname -s 2>/dev/null || true)" == "Linux" ]] || return 1
    command -v systemctl >/dev/null 2>&1 || return 1
    [[ ! -d /usr/syno ]] || return 1
    [[ ! -f /etc/unraid-version ]] || return 1
    [[ ! -d /boot/config/plugins ]] || return 1
    [[ ! -x /sbin/getcfg ]] || return 1
    [[ ! -f /etc/truenas-version ]] || return 1
    [[ ! -d /data/ix-applications ]] || return 1
    [[ ! -d /etc/ix-apps.d ]] || return 1
    [[ ! -d /etc/ix.rc.d ]] || return 1
    if declare -F is_truenas >/dev/null 2>&1 && is_truenas; then
        return 1
    fi
    return 0
}

safe_profile_detect_current_profile() {
    local unit_path="$SAFE_PROFILE_COLLECTOR_UNIT"
    if [[ ! -f "$unit_path" ]]; then
        printf 'absent\n'
    elif grep -q "^User=${LEAST_PRIVILEGE_USER}$" "$unit_path" 2>/dev/null &&
         grep -q 'PULSE_AGENT_HELPER_SOCKET=' "$unit_path" 2>/dev/null; then
        printf 'typed-helper-monitoring-only\n'
    elif grep -q "^User=${LEAST_PRIVILEGE_USER}$" "$unit_path" 2>/dev/null; then
        printf 'legacy-least-privilege\n'
    elif grep -Eq -- '(^|[[:space:]])--enable-commands([[:space:]]|$)' "$unit_path" 2>/dev/null; then
        printf 'legacy-root-command-capable\n'
    else
        printf 'legacy-root-monitoring\n'
    fi
}

safe_profile_unit_property() {
    local property="$1"
    local unit_path="$SAFE_PROFILE_COLLECTOR_UNIT"
    local value=""
    value=$(systemctl show "${AGENT_NAME}.service" --property "$property" --value 2>/dev/null || true)
    if [[ -n "$value" ]]; then
        printf '%s\n' "$value"
        return 0
    fi
    case "$property" in
        User) sed -n 's/^User=//p' "$unit_path" 2>/dev/null | tail -1 ;;
        AmbientCapabilities) sed -n 's/^AmbientCapabilities=//p' "$unit_path" 2>/dev/null | tail -1 ;;
    esac
}

systemd_effective_unit_property() {
    local unit_name="$1"
    local property="$2"
    systemctl show "$unit_name" --property "$property" --value 2>/dev/null
}

systemd_effective_unit_unoverridden() {
    local unit_name="$1"
    local expected_fragment="$2"
    local fragment_path=""
    local drop_in_paths=""
    fragment_path=$(systemd_effective_unit_property "$unit_name" FragmentPath) || return 1
    drop_in_paths=$(systemd_effective_unit_property "$unit_name" DropInPaths) || return 1
    [[ "$fragment_path" == "$expected_fragment" && -z "$drop_in_paths" ]]
}

systemd_effective_exec_argv() {
    local unit_name="$1"
    local exec_start=""
    local argv=""
    exec_start=$(systemd_effective_unit_property "$unit_name" ExecStart) || return 1
    [[ "$exec_start" == *"argv[]="* ]] || return 1
    argv="${exec_start#*argv[]=}"
    argv="${argv%% ;*}"
    printf '%s\n' "$argv"
}

systemd_effective_exec_exact() {
    local unit_name="$1"
    local expected_binary="$2"
    local argv=""
    argv=$(systemd_effective_exec_argv "$unit_name") || return 1
    [[ "$argv" == "$expected_binary" ]]
}

systemd_effective_words_equal() {
    local unit_name="$1"
    local property="$2"
    shift 2
    local actual=""
    local expected=""
    actual=$(systemd_effective_unit_property "$unit_name" "$property") || return 1
    actual=$(printf '%s\n' "$actual" | tr '[:space:]' '\n' | sed '/^$/d' | LC_ALL=C sort -u | tr '\n' ' ' | sed 's/[[:space:]]*$//')
    expected=$(printf '%s\n' "$@" | LC_ALL=C sort -u | tr '\n' ' ' | sed 's/[[:space:]]*$//')
    [[ "$actual" == "$expected" ]]
}

systemd_effective_common_hardening() {
    local unit_name="$1"
    [[ "$(systemd_effective_unit_property "$unit_name" UMask)" == "0077" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" NoNewPrivileges)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" PrivateTmp)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" PrivateDevices)" == "no" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" ProtectKernelTunables)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" ProtectKernelModules)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" ProtectControlGroups)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" LockPersonality)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" RestrictSUIDSGID)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$unit_name" SystemCallArchitectures)" == "native" ]] || return 1
}

safe_profile_verify_helper_effective_target() {
    local helper_service="${PRIVILEGED_HELPER_NAME}.service"
    local helper_socket="${PRIVILEGED_HELPER_NAME}.socket"
    local listen=""
    local environment_files=""

    systemd_effective_unit_unoverridden "$helper_service" "$PRIVILEGED_HELPER_SERVICE_UNIT" || return 1
    systemd_effective_unit_unoverridden "$helper_socket" "$PRIVILEGED_HELPER_SOCKET_UNIT" || return 1
    systemd_effective_exec_exact "$helper_service" "$PRIVILEGED_HELPER_BINARY_PATH" || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" User)" == "root" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" Group)" == "root" ]] || return 1
    [[ -z "$(systemd_effective_unit_property "$helper_service" AmbientCapabilities)" ]] || return 1
    systemd_effective_common_hardening "$helper_service" || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" PrivateNetwork)" == "yes" ]] || return 1
    systemd_effective_words_equal "$helper_service" RestrictAddressFamilies AF_UNIX || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" ProtectSystem)" == "strict" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" ProtectHome)" == "yes" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" TasksMax)" == "64" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" LimitNOFILE)" == "256" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_service" MemoryMax)" == "268435456" ]] || return 1
    [[ -z "$(systemd_effective_unit_property "$helper_service" Environment)" ]] || return 1
    environment_files=$(systemd_effective_unit_property "$helper_service" EnvironmentFiles) || return 1
    [[ -z "$environment_files" ]] || return 1
    systemd_effective_words_equal "$helper_service" ReadOnlyPaths "$PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR" || return 1
    systemd_effective_words_equal "$helper_service" ReadWritePaths "$PRIVILEGED_HELPER_STATE_DIR" /usr/local/bin || return 1

    [[ "$(systemd_effective_unit_property "$helper_socket" SocketUser)" == "root" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_socket" SocketGroup)" == "$LEAST_PRIVILEGE_USER" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_socket" SocketMode)" == "0660" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_socket" DirectoryMode)" == "0755" ]] || return 1
    [[ "$(systemd_effective_unit_property "$helper_socket" RemoveOnStop)" == "yes" ]] || return 1
    listen=$(systemd_effective_unit_property "$helper_socket" Listen) || return 1
    [[ "$listen" == *"${PRIVILEGED_HELPER_SOCKET_PATH}"* && "$listen" == *"Stream"* ]] || return 1
}

action_runner_verify_effective_target() {
    local runner_service="${ACTION_RUNNER_NAME}.service"
    local environment_files=""

    systemd_effective_unit_unoverridden "$runner_service" "$ACTION_RUNNER_SERVICE_UNIT" || return 1
    systemd_effective_exec_exact "$runner_service" "$ACTION_RUNNER_BINARY_PATH" || return 1
    [[ "$(systemd_effective_unit_property "$runner_service" User)" == "root" ]] || return 1
    [[ "$(systemd_effective_unit_property "$runner_service" Group)" == "root" ]] || return 1
    [[ -z "$(systemd_effective_unit_property "$runner_service" AmbientCapabilities)" ]] || return 1
    systemd_effective_common_hardening "$runner_service" || return 1
    [[ "$(systemd_effective_unit_property "$runner_service" PrivateNetwork)" == "no" ]] || return 1
    systemd_effective_words_equal "$runner_service" RestrictAddressFamilies AF_UNIX AF_INET AF_INET6 || return 1
    [[ "$(systemd_effective_unit_property "$runner_service" ProtectSystem)" == "no" ]] || return 1
    [[ "$(systemd_effective_unit_property "$runner_service" ProtectHome)" == "yes" ]] || return 1
    systemd_effective_words_equal "$runner_service" ReadWritePaths "$ACTION_RUNNER_STATE_DIR" || return 1
    environment_files=$(systemd_effective_unit_property "$runner_service" EnvironmentFiles) || return 1
    [[ "$environment_files" == "$ACTION_RUNNER_ENV_FILE (ignore_errors=no)" ]]
}

safe_profile_effective_unit_unoverridden() {
    systemd_effective_unit_unoverridden "${AGENT_NAME}.service" "$SAFE_PROFILE_COLLECTOR_UNIT"
}

safe_profile_verify_effective_target() {
    local unit_user=""
    local ambient=""
    local exec_argv=""
    local environment=""

    safe_profile_effective_unit_unoverridden || return 1
    unit_user=$(safe_profile_unit_property User)
    ambient=$(safe_profile_unit_property AmbientCapabilities)
    exec_argv=$(systemd_effective_exec_argv "${AGENT_NAME}.service") || return 1
    environment=$(safe_profile_unit_property Environment)
    [[ "$unit_user" == "$LEAST_PRIVILEGE_USER" ]] || return 1
    [[ -z "$ambient" ]] || return 1
    systemd_effective_common_hardening "${AGENT_NAME}.service" || return 1
    [[ "$exec_argv" == "${INSTALL_DIR}/${BINARY_NAME}" || "$exec_argv" == "${INSTALL_DIR}/${BINARY_NAME} "* ]] || return 1
    [[ "$exec_argv" != *"--enable-commands"* ]] || return 1
    [[ "$environment" == *"PULSE_AGENT_HELPER_SOCKET=${PRIVILEGED_HELPER_SOCKET_PATH}"* ]] || return 1
    safe_profile_verify_helper_effective_target || return 1
    if [[ -e "$ACTION_RUNNER_SERVICE_UNIT" ]]; then
        action_runner_verify_effective_target || return 1
    fi
}

safe_profile_inspect() {
    local unit_path="$SAFE_PROFILE_COLLECTOR_UNIT"
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    local supported="false"
    local profile=""
    local unit_user="root"
    local groups="unavailable"
    local ambient="none"
    local binary_owner="missing"
    local binary_mode="missing"
    local host="false" docker="false" kubernetes="false" proxmox="false"
    local helper="false" commands="false" runner="false"
    local fragment_path="unavailable" drop_in_paths="unavailable" unit_unoverridden="false"

    if safe_profile_platform_supported; then supported="true"; fi
    profile=$(safe_profile_detect_current_profile)
    if [[ -f "$unit_path" ]]; then
        fragment_path=$(safe_profile_unit_property FragmentPath)
        fragment_path="${fragment_path:-unavailable}"
        drop_in_paths=$(safe_profile_unit_property DropInPaths)
        drop_in_paths="${drop_in_paths:-none}"
        if safe_profile_effective_unit_unoverridden; then unit_unoverridden="true"; fi
        unit_user=$(safe_profile_unit_property User)
        unit_user="${unit_user:-root}"
        ambient=$(safe_profile_unit_property AmbientCapabilities)
        ambient="${ambient:-none}"
        if id "$unit_user" >/dev/null 2>&1; then
            groups=$(id -nG "$unit_user" 2>/dev/null || printf 'unavailable')
        fi
        grep -Eq -- '(^|[[:space:]])--disable-host([[:space:]]|$)' "$unit_path" 2>/dev/null || host="true"
        grep -Eq -- '(^|[[:space:]])--enable-docker([[:space:]]|$)' "$unit_path" 2>/dev/null && docker="true"
        grep -Eq -- '(^|[[:space:]])--enable-kubernetes([[:space:]]|$)' "$unit_path" 2>/dev/null && kubernetes="true"
        grep -Eq -- '(^|[[:space:]])--enable-proxmox([[:space:]]|$)' "$unit_path" 2>/dev/null && proxmox="true"
        grep -Eq -- '(^|[[:space:]])--enable-commands([[:space:]]|$)' "$unit_path" 2>/dev/null && commands="true"
        grep -q 'PULSE_AGENT_HELPER_SOCKET=' "$unit_path" 2>/dev/null && helper="true"
    fi
    if [[ -e "$binary_path" && ! -L "$binary_path" ]]; then
        binary_owner=$(stat -c '%U:%G' "$binary_path" 2>/dev/null || printf 'unknown')
        binary_mode=$(stat -c '%a' "$binary_path" 2>/dev/null || printf 'unknown')
    fi
    if [[ -f "$ACTION_RUNNER_SERVICE_UNIT" ]]; then runner="true"; fi

    printf '%s\n' \
        "Pulse safe-profile migration inspection (read-only)" \
        "platform_supported=${supported}" \
        "current_profile=${profile}" \
        "unit_fragment_path=${fragment_path}" \
        "unit_drop_in_paths=${drop_in_paths}" \
        "unit_unoverridden=${unit_unoverridden}" \
        "unit_user=${unit_user}" \
        "unit_groups=${groups}" \
        "ambient_capabilities=${ambient}" \
        "collector_binary_owner=${binary_owner}" \
        "collector_binary_mode=${binary_mode}" \
        "provider_host=${host}" \
        "provider_docker=${docker}" \
        "provider_kubernetes=${kubernetes}" \
        "provider_proxmox=${proxmox}" \
        "typed_helper=${helper}" \
        "collector_commands=${commands}" \
        "action_runner_independent=${runner}" \
        "target_profile=typed-helper-monitoring-only" \
        "target_unit_user=${LEAST_PRIVILEGE_USER}" \
        "target_groups=no-rootful-docker-group" \
        "target_ambient_capabilities=none" \
        "target_binary_owner=root:root" \
        "target_commands=false" \
        "target_smart=typed-helper" \
        "target_proxmox_filesystems=typed-helper" \
        "target_action_runner=unchanged"
    if [[ "$docker" == "true" ]]; then
        printf '%s\n' "degraded_docker=rootful daemon access is removed unless an independently usable rootless socket is configured"
    else
        printf '%s\n' "degraded_docker=none (Docker provider is not enabled in the current unit)"
    fi
    if [[ "$commands" == "true" ]]; then
        printf '%s\n' "degraded_actions=collector command authority is removed; remediation requires the separately enrolled action runner"
    else
        printf '%s\n' "degraded_actions=none (collector command authority is already disabled)"
    fi
    if [[ "$supported" != "true" ]]; then
        log_error "Safe-profile migration is unsupported on this platform; no root or broader-privilege fallback is available."
        return 1
    fi
}

safe_profile_snapshot_entry() {
    local source_path="$1"
    local snapshot_name="$2"
    local manifest_key="$3"
    local manifest_file="${SAFE_PROFILE_TRANSACTION_DIR}/manifest.env"
    if [[ -L "$source_path" ]]; then
        fail "Refusing safe-profile migration across symlinked ${source_path}" "$EXIT_GENERAL"
    fi
    if [[ -e "$source_path" ]]; then
        [[ -f "$source_path" ]] ||
            fail "Refusing non-regular safe-profile migration artifact: ${source_path}" "$EXIT_GENERAL"
        cp -a "$source_path" "${SAFE_PROFILE_TRANSACTION_DIR}/${snapshot_name}"
        printf '%s=true\n' "$manifest_key" >> "$manifest_file"
    else
        printf '%s=false\n' "$manifest_key" >> "$manifest_file"
    fi
}

safe_profile_manifest_value() {
    local manifest_file="$1"
    local key="$2"
    sed -n "s/^${key}=//p" "$manifest_file" 2>/dev/null | tail -1
}

safe_profile_snapshot_state_metadata() {
    local metadata_file="${SAFE_PROFILE_TRANSACTION_DIR}/state-metadata.bin"
    local uid="" gid="" mode=""

    [[ -d "$STATE_DIR" && ! -L "$STATE_DIR" ]] ||
        fail "Refusing unsafe safe-profile state directory: ${STATE_DIR}" "$EXIT_GENERAL"
    : > "$metadata_file"
    chmod 0600 "$metadata_file"
    # Only the state root is installer-owned metadata. Descendants become
    # collector-writable during the safe profile and therefore must never be
    # replayed later as root-owned pathname operations.
    uid=$(stat -c '%u' "$STATE_DIR") || return 1
    gid=$(stat -c '%g' "$STATE_DIR") || return 1
    mode=$(stat -c '%a' "$STATE_DIR") || return 1
    printf '.\0%s\0%s\0%s\0directory\0' "$uid" "$gid" "$mode" > "$metadata_file"
}

safe_profile_restore_state_metadata() {
    local transaction_dir="$1"
    local snapshot_state_dir="$2"
    local metadata_file="${transaction_dir}/state-metadata.bin"
    local relative_path="" uid="" gid="" mode="" entry_type="" destination=""

    [[ -f "$metadata_file" && ! -L "$metadata_file" ]] || return 1
    while IFS= read -r -d '' relative_path &&
          IFS= read -r -d '' uid &&
          IFS= read -r -d '' gid &&
          IFS= read -r -d '' mode &&
          IFS= read -r -d '' entry_type; do
        # Format-v2 snapshots created before the hardened rollback recorded the
        # complete mutable tree. Ignore those descendant records: replaying
        # collector-controlled pathnames as root is unsafe. The state root
        # cannot be replaced by the service account because its parent is root.
        [[ "$relative_path" == "." ]] || continue
        destination="$snapshot_state_dir"
        [[ -d "$destination" && ! -L "$destination" && "$entry_type" == "directory" ]] || return 1
        chown "${uid}:${gid}" "$destination" || return 1
        chmod "$mode" "$destination" || return 1
    done < "$metadata_file"
}

safe_profile_begin_transaction() {
    local unit_path="$SAFE_PROFILE_COLLECTOR_UNIT"
    local prior_profile=""
    local transaction_id=""
    local manifest_file=""
    local docker_member="false"

    [[ -x "${INSTALL_DIR}/${BINARY_NAME}" && -f "$unit_path" ]] ||
        fail "--safe-profile-apply requires an existing Linux systemd Pulse collector installation" "$EXIT_MISSING_ARGS"
    safe_profile_effective_unit_unoverridden ||
        fail "Refusing safe-profile migration while the collector has a different effective FragmentPath or any systemd drop-in override; consolidate the effective unit first" "$EXIT_GENERAL"
    [[ ! -L "$SAFE_PROFILE_STATE_DIR" ]] ||
        fail "Refusing symlinked safe-profile transaction directory: ${SAFE_PROFILE_STATE_DIR}" "$EXIT_GENERAL"
    mkdir -p "$SAFE_PROFILE_STATE_DIR"
    chmod 0700 "$SAFE_PROFILE_STATE_DIR"
    transaction_id="transaction-$(date -u +%Y%m%dT%H%M%SZ)-$$"
    SAFE_PROFILE_TRANSACTION_DIR="${SAFE_PROFILE_STATE_DIR}/${transaction_id}"
    mkdir "$SAFE_PROFILE_TRANSACTION_DIR"
    chmod 0700 "$SAFE_PROFILE_TRANSACTION_DIR"
    manifest_file="${SAFE_PROFILE_TRANSACTION_DIR}/manifest.env"
    : > "$manifest_file"
    chmod 0600 "$manifest_file"

    prior_profile=$(safe_profile_detect_current_profile)
    printf '%s\n' \
        "FORMAT_VERSION=2" \
        "PRIOR_PROFILE=${prior_profile}" \
        "TARGET_PROFILE=typed-helper-monitoring-only" \
        "STATE_DIR=${STATE_DIR}" \
        "COLLECTOR_ACTIVE=$(systemctl is-active --quiet "${AGENT_NAME}.service" 2>/dev/null && printf true || printf false)" \
        "COLLECTOR_ENABLED=$(systemctl is-enabled --quiet "${AGENT_NAME}.service" 2>/dev/null && printf true || printf false)" \
        "HELPER_ACTIVE=$(systemctl is-active --quiet "${PRIVILEGED_HELPER_NAME}.socket" 2>/dev/null && printf true || printf false)" \
        "HELPER_ENABLED=$(systemctl is-enabled --quiet "${PRIVILEGED_HELPER_NAME}.socket" 2>/dev/null && printf true || printf false)" >> "$manifest_file"
    if id -nG "$LEAST_PRIVILEGE_USER" 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
        docker_member="true"
    fi
    printf 'DOCKER_MEMBER=%s\n' "$docker_member" >> "$manifest_file"
    if [[ -L "$PRIVILEGED_HELPER_CREDENTIAL_DIR" ]]; then
        fail "Refusing symlinked safe-profile credential directory: ${PRIVILEGED_HELPER_CREDENTIAL_DIR}" "$EXIT_GENERAL"
    elif [[ -d "$PRIVILEGED_HELPER_CREDENTIAL_DIR" ]]; then
        printf '%s\n' \
            "PROTECTED_DIR=true" \
            "PROTECTED_DIR_UID=$(stat -c '%u' "$PRIVILEGED_HELPER_CREDENTIAL_DIR")" \
            "PROTECTED_DIR_GID=$(stat -c '%g' "$PRIVILEGED_HELPER_CREDENTIAL_DIR")" \
            "PROTECTED_DIR_MODE=$(stat -c '%a' "$PRIVILEGED_HELPER_CREDENTIAL_DIR")" >> "$manifest_file"
    else
        printf 'PROTECTED_DIR=false\n' >> "$manifest_file"
    fi

    safe_profile_snapshot_entry "${INSTALL_DIR}/${BINARY_NAME}" collector-binary COLLECTOR_BINARY
    safe_profile_snapshot_entry "$unit_path" collector-unit COLLECTOR_UNIT
    safe_profile_snapshot_entry "$PRIVILEGED_HELPER_BINARY_PATH" helper-binary HELPER_BINARY
    safe_profile_snapshot_entry "$PRIVILEGED_HELPER_SERVICE_UNIT" helper-service-unit HELPER_SERVICE_UNIT
    safe_profile_snapshot_entry "$PRIVILEGED_HELPER_SOCKET_UNIT" helper-socket-unit HELPER_SOCKET_UNIT
    safe_profile_snapshot_entry "$PRIVILEGE_SUDOERS_FILE" legacy-sudoers LEGACY_SUDOERS
    safe_profile_snapshot_entry "${PRIVILEGE_HELPER_DIR}/smartctl" legacy-smartctl-wrapper LEGACY_SMARTCTL_WRAPPER
    safe_profile_snapshot_entry "${PRIVILEGE_HELPER_DIR}/pct" legacy-pct-wrapper LEGACY_PCT_WRAPPER
    safe_profile_snapshot_entry "${STATE_DIR%/}/token" state-token STATE_TOKEN
    safe_profile_snapshot_entry "${STATE_DIR%/}/runtime.token" runtime-token RUNTIME_TOKEN
    safe_profile_snapshot_entry "${STATE_DIR%/}/agent-id" agent-id AGENT_ID_FILE
    safe_profile_snapshot_entry "${STATE_DIR%/}/connection.env" connection-env CONNECTION_ENV
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-registered" proxmox-registered PROXMOX_REGISTERED
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-pve-registered" proxmox-pve-registered PROXMOX_PVE_REGISTERED
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-pbs-registered" proxmox-pbs-registered PROXMOX_PBS_REGISTERED
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-pve-registration-blocked" proxmox-pve-registration-blocked PROXMOX_PVE_REGISTRATION_BLOCKED
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-pbs-registration-blocked" proxmox-pbs-registration-blocked PROXMOX_PBS_REGISTRATION_BLOCKED
    safe_profile_snapshot_entry "${STATE_DIR%/}/proxmox-detected-types" proxmox-detected-types PROXMOX_DETECTED_TYPES
    safe_profile_snapshot_entry "${PRIVILEGED_HELPER_CREDENTIAL_DIR}/token" protected-token PROTECTED_TOKEN
    safe_profile_snapshot_state_metadata

    SAFE_PROFILE_TRANSACTION_ACTIVE="true"
    SAFE_PROFILE_TRANSACTION_COMMITTED="false"
    log_info "Snapshotted ${prior_profile} collector/helper profile before migration."
}

safe_profile_restore_entry() {
    local transaction_dir="$1"
    local snapshot_name="$2"
    local destination="$3"
    local manifest_key="$4"
    local present=""
    present=$(safe_profile_manifest_value "${transaction_dir}/manifest.env" "$manifest_key")
    rm -f "$destination"
    if [[ "$present" == "true" ]]; then
        mkdir -p "$(dirname "$destination")"
        cp -a "${transaction_dir}/${snapshot_name}" "$destination"
    fi
}

safe_profile_remove_collector_command_authority() {
    local unit_path="$1"
    local rewritten=""

    [[ -f "$unit_path" && ! -L "$unit_path" ]] || return 1
    rewritten=$(mktemp)
    chmod 0600 "$rewritten"
    if ! sed -E 's/(^|[[:space:]])--enable-commands([[:space:]]|$)/\1\2/g' "$unit_path" > "$rewritten"; then
        rm -f "$rewritten"
        return 1
    fi
    # Preserve the restored unit's root-controlled inode metadata while making
    # the irreversible server-side scope reduction explicit in local config.
    if ! cat "$rewritten" > "$unit_path"; then
        rm -f "$rewritten"
        return 1
    fi
    rm -f "$rewritten"
    ! grep -Eq -- '(^|[[:space:]])--enable-commands([[:space:]]|$)' "$unit_path"
}

safe_profile_restore_transaction() {
    local transaction_dir="$1"
    local reason="${2:-explicit}"
    local manifest_file="${transaction_dir}/manifest.env"
    local snapshot_state_dir=""
    local prior_profile=""
    local docker_member="false"
    local current_tmp=""
    local format_version=""

    SAFE_PROFILE_TRANSACTION_ACTIVE="false"
    case "$transaction_dir" in
        "${SAFE_PROFILE_STATE_DIR}"/transaction-*) ;;
        *) log_error "Refusing untrusted safe-profile transaction path: ${transaction_dir}"; return 1 ;;
    esac
    [[ -d "$transaction_dir" && ! -L "$transaction_dir" && -f "$manifest_file" && ! -L "$manifest_file" ]] || return 1
    format_version=$(safe_profile_manifest_value "$manifest_file" FORMAT_VERSION)
    [[ "$format_version" == "1" || "$format_version" == "2" ]] || return 1
    snapshot_state_dir=$(safe_profile_manifest_value "$manifest_file" STATE_DIR)
    [[ -n "$snapshot_state_dir" && "$snapshot_state_dir" == /* && "$snapshot_state_dir" != "/" ]] || return 1
    prior_profile=$(safe_profile_manifest_value "$manifest_file" PRIOR_PROFILE)

    systemctl stop "${AGENT_NAME}.service" 2>/dev/null || true
    systemctl stop "${PRIVILEGED_HELPER_NAME}.socket" 2>/dev/null || true
    systemctl stop "${PRIVILEGED_HELPER_NAME}.service" 2>/dev/null || true
    safe_profile_restore_entry "$transaction_dir" collector-binary "${INSTALL_DIR}/${BINARY_NAME}" COLLECTOR_BINARY
    safe_profile_restore_entry "$transaction_dir" collector-unit "$SAFE_PROFILE_COLLECTOR_UNIT" COLLECTOR_UNIT
    # The server-side execution-scope reduction deliberately survives both
    # explicit and automatic local rollback. Never recreate a command-capable
    # collector configuration around that monitoring-only credential.
    safe_profile_remove_collector_command_authority "$SAFE_PROFILE_COLLECTOR_UNIT" || return 1
    safe_profile_restore_entry "$transaction_dir" helper-binary "$PRIVILEGED_HELPER_BINARY_PATH" HELPER_BINARY
    safe_profile_restore_entry "$transaction_dir" helper-service-unit "$PRIVILEGED_HELPER_SERVICE_UNIT" HELPER_SERVICE_UNIT
    safe_profile_restore_entry "$transaction_dir" helper-socket-unit "$PRIVILEGED_HELPER_SOCKET_UNIT" HELPER_SOCKET_UNIT
    safe_profile_restore_entry "$transaction_dir" legacy-sudoers "$PRIVILEGE_SUDOERS_FILE" LEGACY_SUDOERS
    safe_profile_restore_entry "$transaction_dir" legacy-smartctl-wrapper "${PRIVILEGE_HELPER_DIR}/smartctl" LEGACY_SMARTCTL_WRAPPER
    safe_profile_restore_entry "$transaction_dir" legacy-pct-wrapper "${PRIVILEGE_HELPER_DIR}/pct" LEGACY_PCT_WRAPPER
    safe_profile_restore_entry "$transaction_dir" state-token "${snapshot_state_dir%/}/token" STATE_TOKEN
    safe_profile_restore_entry "$transaction_dir" runtime-token "${snapshot_state_dir%/}/runtime.token" RUNTIME_TOKEN
    safe_profile_restore_entry "$transaction_dir" agent-id "${snapshot_state_dir%/}/agent-id" AGENT_ID_FILE
    safe_profile_restore_entry "$transaction_dir" connection-env "${snapshot_state_dir%/}/connection.env" CONNECTION_ENV
    if [[ "$format_version" == "2" ]]; then
        safe_profile_restore_entry "$transaction_dir" proxmox-registered "${snapshot_state_dir%/}/proxmox-registered" PROXMOX_REGISTERED
        safe_profile_restore_entry "$transaction_dir" proxmox-pve-registered "${snapshot_state_dir%/}/proxmox-pve-registered" PROXMOX_PVE_REGISTERED
        safe_profile_restore_entry "$transaction_dir" proxmox-pbs-registered "${snapshot_state_dir%/}/proxmox-pbs-registered" PROXMOX_PBS_REGISTERED
        safe_profile_restore_entry "$transaction_dir" proxmox-pve-registration-blocked "${snapshot_state_dir%/}/proxmox-pve-registration-blocked" PROXMOX_PVE_REGISTRATION_BLOCKED
        safe_profile_restore_entry "$transaction_dir" proxmox-pbs-registration-blocked "${snapshot_state_dir%/}/proxmox-pbs-registration-blocked" PROXMOX_PBS_REGISTRATION_BLOCKED
        safe_profile_restore_entry "$transaction_dir" proxmox-detected-types "${snapshot_state_dir%/}/proxmox-detected-types" PROXMOX_DETECTED_TYPES
    fi
    safe_profile_restore_entry "$transaction_dir" protected-token "${PRIVILEGED_HELPER_CREDENTIAL_DIR}/token" PROTECTED_TOKEN
    if [[ "$(safe_profile_manifest_value "$manifest_file" PROTECTED_DIR)" == "true" ]]; then
        chown "$(safe_profile_manifest_value "$manifest_file" PROTECTED_DIR_UID):$(safe_profile_manifest_value "$manifest_file" PROTECTED_DIR_GID)" "$PRIVILEGED_HELPER_CREDENTIAL_DIR" || return 1
        chmod "$(safe_profile_manifest_value "$manifest_file" PROTECTED_DIR_MODE)" "$PRIVILEGED_HELPER_CREDENTIAL_DIR" || return 1
    else
        rmdir "$PRIVILEGED_HELPER_CREDENTIAL_DIR" 2>/dev/null || true
    fi
    if [[ "$format_version" == "2" ]]; then
        safe_profile_restore_state_metadata "$transaction_dir" "$snapshot_state_dir" || return 1
    fi
    rm -f "$PRIVILEGED_HELPER_SOCKET_PATH"

    docker_member=$(safe_profile_manifest_value "$manifest_file" DOCKER_MEMBER)
    if getent group docker >/dev/null 2>&1 && id "$LEAST_PRIVILEGE_USER" >/dev/null 2>&1; then
        if [[ "$docker_member" == "true" ]]; then
            gpasswd -a "$LEAST_PRIVILEGE_USER" docker >/dev/null 2>&1 || return 1
        else
            gpasswd -d "$LEAST_PRIVILEGE_USER" docker >/dev/null 2>&1 || true
        fi
    fi

    systemctl daemon-reload 2>/dev/null || return 1
    if [[ "$(safe_profile_manifest_value "$manifest_file" HELPER_ENABLED)" == "true" ]]; then
        systemctl enable "${PRIVILEGED_HELPER_NAME}.socket" >/dev/null 2>&1 || return 1
    else
        systemctl disable "${PRIVILEGED_HELPER_NAME}.socket" >/dev/null 2>&1 || true
    fi
    if [[ "$(safe_profile_manifest_value "$manifest_file" HELPER_ACTIVE)" == "true" ]]; then
        systemctl start "${PRIVILEGED_HELPER_NAME}.socket" >/dev/null 2>&1 || return 1
    fi
    if [[ "$(safe_profile_manifest_value "$manifest_file" COLLECTOR_ENABLED)" == "true" ]]; then
        systemctl enable "${AGENT_NAME}.service" >/dev/null 2>&1 || return 1
    else
        systemctl disable "${AGENT_NAME}.service" >/dev/null 2>&1 || true
    fi
    if [[ "$(safe_profile_manifest_value "$manifest_file" COLLECTOR_ACTIVE)" == "true" ]]; then
        systemctl start "${AGENT_NAME}.service" >/dev/null 2>&1 || return 1
    fi

    mkdir -p "$SAFE_PROFILE_STATE_DIR"
    chmod 0700 "$SAFE_PROFILE_STATE_DIR"
    current_tmp=$(mktemp "${SAFE_PROFILE_STATE_DIR}/.current.XXXXXX")
    chmod 0600 "$current_tmp"
    printf '%s\n' \
        "FORMAT_VERSION=1" \
        "CURRENT_PROFILE=${prior_profile}" \
        "PREVIOUS_PROFILE=typed-helper-monitoring-only" \
        "LAST_ROLLBACK_TRANSACTION=${transaction_dir}" \
        "ROLLBACK_REASON=${reason}" > "$current_tmp"
    mv "$current_tmp" "$SAFE_PROFILE_CURRENT_FILE"
    SAFE_PROFILE_TRANSACTION_COMMITTED="false"
    log_info "Restored collector/helper profile ${prior_profile}; the action runner was left unchanged."
}

safe_profile_remove_legacy_authority() {
    rm -f "$PRIVILEGE_SUDOERS_FILE"
    rm -f "${PRIVILEGE_HELPER_DIR}/smartctl" "${PRIVILEGE_HELPER_DIR}/pct"
    if getent group docker >/dev/null 2>&1 && id "$LEAST_PRIVILEGE_USER" >/dev/null 2>&1; then
        gpasswd -d "$LEAST_PRIVILEGE_USER" docker >/dev/null 2>&1 || true
    fi
}

safe_profile_probe_helper_protocol() {
    local request_id="installer-health-$$"
    local request=""
    local request_length=0
    local response_file=""
    local response_size=0
    local response_length=0
    local response_body=""
    local header_bytes=""
    local header_one=0 header_two=0 header_three=0 header_four=0

    command -v runuser >/dev/null 2>&1 || return 1
    id "$LEAST_PRIVILEGE_USER" >/dev/null 2>&1 || return 1
    [[ -S "$PRIVILEGED_HELPER_SOCKET_PATH" ]] || return 1
    request="{\"protocolVersion\":1,\"requestId\":\"${request_id}\",\"operation\":\"helper.health\",\"operationVersion\":1,\"deadlineMillis\":2000,\"payload\":{}}"
    request_length=${#request}
    [[ $request_length -gt 0 && $request_length -le 65536 ]] || return 1
    response_file=$(mktemp "${SAFE_PROFILE_TRANSACTION_DIR}/.helper-health.XXXXXX") || return 1
    chmod 0600 "$response_file"
    if ! {
        printf "\\$(printf '%03o' $((request_length / 16777216 % 256)))"
        printf "\\$(printf '%03o' $((request_length / 65536 % 256)))"
        printf "\\$(printf '%03o' $((request_length / 256 % 256)))"
        printf "\\$(printf '%03o' $((request_length % 256)))"
        printf '%s' "$request"
    } | runuser -u "$LEAST_PRIVILEGE_USER" -- curl -sS --max-time 5 \
        --unix-socket "$PRIVILEGED_HELPER_SOCKET_PATH" --upload-file - telnet://localhost > "$response_file"; then
        rm -f "$response_file"
        return 1
    fi
    response_size=$(wc -c < "$response_file" | tr -d ' ')
    [[ "$response_size" =~ ^[0-9]+$ && $response_size -ge 5 ]] || { rm -f "$response_file"; return 1; }
    header_bytes=$(od -An -tu1 -N4 "$response_file")
    read -r header_one header_two header_three header_four <<< "$header_bytes"
    response_length=$((header_one * 16777216 + header_two * 65536 + header_three * 256 + header_four))
    [[ $response_length -gt 0 && $response_length -le 1048576 && $response_size -eq $((response_length + 4)) ]] || {
        rm -f "$response_file"
        return 1
    }
    response_body=$(dd if="$response_file" bs=1 skip=4 count="$response_length" 2>/dev/null)
    rm -f "$response_file"
    printf '%s' "$response_body" | grep -q '"protocolVersion"[[:space:]]*:[[:space:]]*1' || return 1
    printf '%s' "$response_body" | grep -q "\"requestId\"[[:space:]]*:[[:space:]]*\"${request_id}\"" || return 1
    printf '%s' "$response_body" | grep -q '"operation"[[:space:]]*:[[:space:]]*"helper.health"' || return 1
    printf '%s' "$response_body" | grep -q '"operationVersion"[[:space:]]*:[[:space:]]*1' || return 1
    printf '%s' "$response_body" | grep -q '"success"[[:space:]]*:[[:space:]]*true' || return 1
    printf '%s' "$response_body" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"' || return 1
}

safe_profile_verify_declared_health() {
    local health_url=""
    local attempt=0
    local local_health_ready="false"
    health_url=$(resolve_agent_health_url || true)
    [[ -n "$health_url" ]] || return 1

    # systemd can report the new units active before the collector readiness
    # endpoint and socket-activated helper have finished starting. Give that
    # local floor a bounded window, then perform the (separately retried)
    # authoritative server-registration proof exactly once.
    for ((attempt = 1; attempt <= 30; attempt++)); do
        if curl -sf --max-time 2 "$health_url" >/dev/null 2>&1 &&
           systemctl is-active --quiet "${AGENT_NAME}.service" &&
           systemctl is-active --quiet "${PRIVILEGED_HELPER_NAME}.socket" &&
           safe_profile_verify_effective_target &&
           safe_profile_probe_helper_protocol; then
            local_health_ready="true"
            break
        fi
        sleep 1
    done
    [[ "$local_health_ready" == "true" ]] || return 1
    [[ -n "$SAFE_PROFILE_PRIOR_REGISTRATION_LAST_SEEN" ]] || return 1
    verify_agent_server_registration_with_retry "$SAFE_PROFILE_PRIOR_REGISTRATION_LAST_SEEN"
}

safe_profile_commit_transaction() {
    local current_tmp=""
    local prior_profile=""
    [[ "$SAFE_PROFILE_TRANSACTION_ACTIVE" == "true" && -n "$SAFE_PROFILE_TRANSACTION_DIR" ]] || return 1
    prior_profile=$(safe_profile_manifest_value "${SAFE_PROFILE_TRANSACTION_DIR}/manifest.env" PRIOR_PROFILE)
    current_tmp=$(mktemp "${SAFE_PROFILE_STATE_DIR}/.current.XXXXXX")
    chmod 0600 "$current_tmp"
    printf '%s\n' \
        "FORMAT_VERSION=1" \
        "PRIOR_PROFILE=${prior_profile}" \
        "CURRENT_PROFILE=typed-helper-monitoring-only" \
        "TRANSACTION_DIR=${SAFE_PROFILE_TRANSACTION_DIR}" \
        "COMMITTED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$current_tmp"
    mv "$current_tmp" "$SAFE_PROFILE_CURRENT_FILE"
    SAFE_PROFILE_TRANSACTION_COMMITTED="true"
    SAFE_PROFILE_TRANSACTION_ACTIVE="false"
    log_info "Committed typed-helper monitoring-only profile; rollback snapshot retained at ${SAFE_PROFILE_TRANSACTION_DIR}."
}

safe_profile_rollback_last() {
    local transaction_dir=""
    [[ -f "$SAFE_PROFILE_CURRENT_FILE" && ! -L "$SAFE_PROFILE_CURRENT_FILE" ]] ||
        fail "No committed safe-profile migration is available to roll back" "$EXIT_MISSING_ARGS"
    transaction_dir=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
    [[ -n "$transaction_dir" ]] ||
        fail "The current profile record has no active migration snapshot to roll back" "$EXIT_MISSING_ARGS"
    safe_profile_restore_transaction "$transaction_dir" "explicit-operator-rollback" ||
        fail "Safe-profile rollback failed closed; the action runner was not changed" "$EXIT_GENERAL"
}

render_systemd_agent_unit() {
	local unit_path="$1"
	local exec_path="$2"
	local exec_args="$3"
	local after_targets="$4"
    local wants_targets="$5"
    local run_as_user="$6"
    local log_target="$7"
    local env_line=""
    local wants_line=""
	local user_line=""
	local log_lines=""
	local no_new_privileges="true"
	local restrict_suidsgid="true"

	env_line="$SYSTEMD_ENV_LINES"
	if [[ -n "$wants_targets" ]]; then
		wants_line=$'\n'"Wants=${wants_targets}"
	fi
    if [[ -n "$run_as_user" ]]; then
        user_line=$'\n'"User=${run_as_user}"
    fi
	if [[ -n "$log_target" ]]; then
		log_lines=$'\n'"StandardOutput=append:${log_target}"$'\n'"StandardError=append:${log_target}"
	fi
	local ambient_line=""
	if systemd_agent_requires_lxc_attach; then
		no_new_privileges="false"
		restrict_suidsgid="false"
	fi
	if [[ "$LEAST_PRIVILEGE" == "true" ]] && [[ "$GRANT_SMART" == "true" || "$GRANT_PCT" == "true" ]]; then
		# The scoped sudo helpers are the profile's only privilege path, and
		# NoNewPrivileges blocks sudo outright ("no new privileges flag is
		# set"). Proven on a live systemd host: with NNP on, every helper call
		# fails and SMART/pct silently disappear. A grant therefore relaxes
		# NNP; a grantless least-privilege install keeps it.
		no_new_privileges="false"
	fi
	if systemd_agent_may_attach_lxc; then
		# lxc-attach into an unprivileged guest writes /proc/<pid>/uid_map,
		# which needs CAP_SETUID in the parent user namespace. NoNewPrivileges
		# drops CAP_SETUID from the effective set and also stops lxc-attach
		# falling back to the setuid newuidmap/newgidmap helpers, so the probe
		# dies with "write_id_mapping: Operation not permitted" and Docker in
		# every unprivileged LXC stays invisible.
		#
		# The grant exists only for an explicitly command-capable PVE install.
		# A monitoring-only unit must not carry dormant privilege in case a
		# remote setting later requests command execution.
		ambient_line=$'\n'"AmbientCapabilities=CAP_SETUID CAP_SETGID"
	fi

	local hardening_lines
	hardening_lines="NoNewPrivileges=${no_new_privileges}
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
LockPersonality=true
RestrictSUIDSGID=${restrict_suidsgid}
SystemCallArchitectures=native${ambient_line}"
	if [[ -d /usr/syno ]]; then
		# Synology DSM ships a heavily patched, old systemd whose kernels
		# cannot apply these sandbox directives; NoNewPrivileges alone kills
		# the service with status=227/NO_NEW_PRIVILEGES before exec.
		hardening_lines="# Sandbox hardening omitted: Synology DSM systemd cannot apply it."
	fi

	cat > "$unit_path" <<EOF
[Unit]
Description=Pulse Unified Agent
After=${after_targets}${wants_line}
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${exec_path} ${exec_args}${env_line}
Restart=always
RestartSec=5s${user_line}${log_lines}
UMask=0077
${hardening_lines}

[Install]
WantedBy=multi-user.target
EOF
}

systemd_agent_requires_lxc_attach() {
	if [[ "$ENABLE_COMMANDS" != "true" ]]; then
		return 1
	fi
	systemd_agent_may_attach_lxc
}

# Whether this explicitly command-capable host needs lxc-attach. Remote
# configuration cannot promote a monitoring-only process into a command
# runtime, so the unit must not provision dormant attach capabilities.
systemd_agent_may_attach_lxc() {
	if [[ "$ENABLE_COMMANDS" != "true" ]]; then
		return 1
	fi
	# The least-privilege profile never attaches into guests: pct exec and
	# command execution stay root-profile features, so it does not get the
	# CAP_SETUID/CAP_SETGID ambient grant either.
	if [[ "$LEAST_PRIVILEGE" == "true" ]]; then
		return 1
	fi
	if [[ "$ENABLE_PROXMOX" != "true" ]]; then
		return 1
	fi
	case "${PROXMOX_TYPE:-}" in
		""|pve|all)
			return 0
			;;
		*)
			return 1
			;;
	esac
}

render_freebsd_rc_agent_script() {
    local script_path="$1"
    local exec_path="$2"
    local exec_args="$3"
    local service_env_lines="$SHELL_EXPORT_LINES"

    cat > "$script_path" <<EOF
#!/bin/sh

# PROVIDE: pulse_agent
# REQUIRE: LOGIN NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pulse_agent"
rcvar="pulse_agent_enable"
pidfile="/var/run/\${name}.pid"
child_pidfile="/var/run/\${name}.child.pid"

command="${exec_path}"
command_args="${exec_args}"

start_cmd="\${name}_start"
stop_cmd="\${name}_stop"
status_cmd="\${name}_status"

pulse_agent_pid_command()
{
    ps -o command= -p "\$1" 2>/dev/null | sed 's/^[[:space:]]*//'
}

pulse_agent_supervisor_pid()
{
    agent_pid="\$1"
    agent_command=\$(pulse_agent_pid_command "\${agent_pid}")
    case "\${agent_command}" in
        daemon:*)
            echo "\${agent_pid}"
            return 0
            ;;
    esac

    parent_pid=\$(ps -o ppid= -p "\${agent_pid}" 2>/dev/null | tr -d '[:space:]')
    if [ -z "\${parent_pid}" ] || [ "\${parent_pid}" = "1" ]; then
        return 1
    fi

    parent_command=\$(pulse_agent_pid_command "\${parent_pid}")
    case "\${parent_command}" in
        daemon:*)
            echo "\${parent_pid}"
            return 0
            ;;
    esac

    return 1
}

pulse_agent_start()
{
    if checkyesno \${rcvar}; then
        if [ -f \${pidfile} ]; then
            existing_pid=\$(cat \${pidfile} 2>/dev/null)
            if [ -n "\${existing_pid}" ] && kill -0 "\${existing_pid}" 2>/dev/null; then
                echo "\${name} is already running as pid \${existing_pid}."
                return 0
            fi
        fi

        rm -f \${pidfile} \${child_pidfile}
        echo "Starting \${name}."
        ${service_env_lines}
        /usr/sbin/daemon -r -P \${pidfile} -p \${child_pidfile} -f "\${command}" \${command_args}
    fi
}

pulse_agent_stop()
{
    supervisor_pid=""
    child_pid=""
    stopped=0

    if [ -f \${child_pidfile} ]; then
        child_pid=\$(cat \${child_pidfile} 2>/dev/null)
    fi

    if [ -f \${pidfile} ]; then
        primary_pid=\$(cat \${pidfile} 2>/dev/null)
        if [ -n "\${primary_pid}" ] && kill -0 "\${primary_pid}" 2>/dev/null; then
            detected_supervisor=\$(pulse_agent_supervisor_pid "\${primary_pid}" 2>/dev/null || true)
            if [ -n "\${detected_supervisor}" ]; then
                supervisor_pid="\${detected_supervisor}"
                if [ "\${detected_supervisor}" != "\${primary_pid}" ] && [ -z "\${child_pid}" ]; then
                    child_pid="\${primary_pid}"
                fi
            else
                supervisor_pid="\${primary_pid}"
            fi
        fi
    fi

    if [ -n "\${supervisor_pid}" ] && kill -0 "\${supervisor_pid}" 2>/dev/null; then
        echo "Stopping \${name} supervisor."
        kill "\${supervisor_pid}" 2>/dev/null || true
        sleep 1
        if kill -0 "\${supervisor_pid}" 2>/dev/null; then
            kill -KILL "\${supervisor_pid}" 2>/dev/null || true
        fi
        stopped=1
    fi

    if [ -n "\${child_pid}" ] && kill -0 "\${child_pid}" 2>/dev/null; then
        echo "Stopping \${name} child."
        kill "\${child_pid}" 2>/dev/null || true
        sleep 1
        if kill -0 "\${child_pid}" 2>/dev/null; then
            kill -KILL "\${child_pid}" 2>/dev/null || true
        fi
        stopped=1
    fi

    rm -f \${pidfile} \${child_pidfile}

    if [ "\${stopped}" -eq 0 ]; then
        echo "\${name} is not running."
    fi
}

pulse_agent_status()
{
    if [ -f \${pidfile} ]; then
        primary_pid=\$(cat \${pidfile} 2>/dev/null)
        if [ -n "\${primary_pid}" ] && kill -0 "\${primary_pid}" 2>/dev/null; then
            child_status=""
            if [ -f \${child_pidfile} ]; then
                child_pid=\$(cat \${child_pidfile} 2>/dev/null)
                if [ -n "\${child_pid}" ] && kill -0 "\${child_pid}" 2>/dev/null; then
                    child_status=" with child pid \${child_pid}"
                fi
            fi
            echo "\${name} is running as supervisor pid \${primary_pid}\${child_status}."
            return 0
        fi
    fi

    if [ -f \${child_pidfile} ]; then
        child_pid=\$(cat \${child_pidfile} 2>/dev/null)
        if [ -n "\${child_pid}" ] && kill -0 "\${child_pid}" 2>/dev/null; then
            echo "\${name} is running as child pid \${child_pid}."
            return 0
        fi
    fi

    if [ -f \${pidfile} ]; then
        legacy_pid=\$(cat \${pidfile} 2>/dev/null)
        legacy_supervisor=\$(pulse_agent_supervisor_pid "\${legacy_pid}" 2>/dev/null || true)
        if [ -n "\${legacy_supervisor}" ] && kill -0 "\${legacy_supervisor}" 2>/dev/null; then
            echo "\${name} is running as legacy child pid \${legacy_pid} supervised by pid \${legacy_supervisor}."
            return 0
        fi
    fi

    echo "\${name} is not running."
    return 1
}

load_rc_config \$name
run_rc_command "\$1"
EOF

    chmod +x "$script_path"
}

# report_proxmox_registration_outcome surfaces the agent's Proxmox
# registration result in installer output. The agent records a denied
# registration grant in a proxmox-<type>-registration-blocked marker file so
# the failure is not buried in its journal (#1644).
#
# A host with both PVE and PBS installed registers each product separately, so
# the outcome is reported per product. The agent publishes the products it
# detected in proxmox-detected-types; with that list the installer waits for an
# outcome from every product before deciding, instead of letting whichever one
# lands first speak for the whole install. Agents that predate the list keep the
# old first-outcome-wins timing.
report_proxmox_registration_outcome() {
    local state_dir="$1"
    local max_iterations=15
    local interval=2
    local iteration=0
    local detected_file=""
    local awaited_types=""
    local detected_types=""
    local types_known="false"
    local line=""
    local pending=""
    local saw_outcome="false"
    local blocked_any="false"
    local unconfirmed_any="false"
    local ptype=""

    if [[ "$ENABLE_PROXMOX" != "true" || -z "$state_dir" ]]; then
        return 0
    fi

    detected_file="${state_dir}/proxmox-detected-types"
    awaited_types="pve pbs"

    log_info "Waiting for Proxmox registration result..."
    while [ $iteration -lt $max_iterations ]; do
        # Re-read each pass: the agent writes the list once it has probed the
        # host, which can be after the first poll. Only known product names are
        # accepted, so the marker can never inject a path or a glob into the
        # state-file lookups below.
        if [ -f "$detected_file" ]; then
            detected_types=""
            while IFS= read -r line || [[ -n "$line" ]]; do
                case "${line%$'\r'}" in
                    pve|pbs) detected_types="${detected_types}${line%$'\r'} " ;;
                esac
            done < "$detected_file"
            if [[ -n "$detected_types" ]]; then
                awaited_types="$detected_types"
                types_known="true"
            fi
        fi

        pending=""
        saw_outcome="false"
        for ptype in $awaited_types; do
            if [ -f "${state_dir}/proxmox-${ptype}-registered" ] || [ -f "${state_dir}/proxmox-${ptype}-registration-blocked" ]; then
                saw_outcome="true"
            else
                pending="${pending}${ptype} "
            fi
        done

        if [[ -z "$pending" ]]; then
            break
        fi
        if [[ "$types_known" != "true" && "$saw_outcome" == "true" ]]; then
            break
        fi

        sleep $interval
        iteration=$((iteration + 1))
    done

    for ptype in $awaited_types; do
        if [ -f "${state_dir}/proxmox-${ptype}-registration-blocked" ]; then
            blocked_any="true"
            log_error "Proxmox ${ptype} registration failed:"
            while IFS= read -r line; do log_error "  $line"; done < "${state_dir}/proxmox-${ptype}-registration-blocked"
        elif [ -f "${state_dir}/proxmox-${ptype}-registered" ]; then
            log_info "Proxmox ${ptype} node registered with Pulse."
        elif [[ "$types_known" == "true" ]]; then
            unconfirmed_any="true"
            log_warn "Proxmox ${ptype} registration was not confirmed within ~$((max_iterations * interval))s. The agent keeps retrying in the background."
        fi
    done

    if [[ "$types_known" != "true" && "$saw_outcome" != "true" ]]; then
        unconfirmed_any="true"
        log_warn "Proxmox registration was not confirmed within ~$((max_iterations * interval))s. The agent keeps retrying in the background."
    fi

    if [[ "$unconfirmed_any" == "true" ]]; then
        log_warn "Check the agent logs for Proxmox registration status if the node does not appear in Pulse."
    fi

    if [[ "$blocked_any" == "true" ]]; then
        return 1
    fi
    return 0
}

complete_installation_flow() {
    local state_dir="$1"
    local install_success_message="$2"
    local upgrade_success_message="$3"
    local unhealthy_log_hint="$4"

    local verification_rc=0

    save_connection_info "$state_dir"
    verify_agent_started || verification_rc=$?
    if [[ $verification_rc -eq 0 ]]; then
        report_proxmox_registration_outcome "$state_dir" || true
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "$upgrade_success_message"
            json_event "complete" "updated" "Installation updated"
        else
            log_info "$install_success_message"
            json_event "complete" "installed" "Installation installed"
        fi
    elif [[ $verification_rc -eq 2 ]]; then
        log_error "Pulse Agent authentication failed. The local service is running, but Pulse rejected its credential; installation is not complete. Generate a fresh scoped agent credential in Pulse and run the repair command again."
        json_event "complete" "auth_rejected" "Pulse rejected the agent credential" "$EXIT_AUTH_REJECTED"
        exit "$EXIT_AUTH_REJECTED"
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            if [[ -n "$unhealthy_log_hint" ]]; then
                log_warn "Check logs: $unhealthy_log_hint"
            fi
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi

    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then
        log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"
    fi
}

portable_sed_in_place() {
    local expr="$1"
    local target="$2"

    sed -i '' "$expr" "$target" 2>/dev/null || sed -i "$expr" "$target" 2>/dev/null || true
}

select_platform_state_dir() {
    local platform_default="$1"

    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        STATE_DIR="$platform_default"
        STATE_DIR_SOURCE="platform"
    fi
}

discover_state_dir_from_saved_installer() {
    local script_path="${1:-$0}"
    local script_dir=""

    if [[ "${STATE_DIR_SOURCE:-default}" != "default" || ! -f "$script_path" ]]; then
        return 1
    fi
    script_dir=$(cd "$(dirname "$script_path")" 2>/dev/null && pwd -P) || return 1
    if [[ -f "$script_dir/connection.env" ]]; then
        STATE_DIR="$script_dir"
        STATE_DIR_SOURCE="recovered"
        return 0
    fi
    return 1
}

remove_agent_state_dir() {
    local state_dir="${1:-$STATE_DIR}"

    if [[ -z "$state_dir" || "$state_dir" != /* || "$state_dir" == "/" ||
          "$state_dir" == *$'\r'* || "$state_dir" == *$'\n'* ]]; then
        log_warn "Refusing to remove invalid agent state directory: ${state_dir:-<empty>}"
        return 1
    fi
    rm -rf -- "$state_dir"
}

detect_qnap_data_volume() {
    local qnap_vol=""
    local candidate=""

    if command -v getcfg >/dev/null 2>&1; then
        qnap_vol=$(getcfg SHARE_DEF defVolMP -f /etc/config/def_share.info 2>/dev/null || echo "")
        qnap_vol="${qnap_vol%/}"
        if [[ -n "$qnap_vol" ]] && [[ -d "$qnap_vol" ]] && [[ -w "$qnap_vol" ]]; then
            printf '%s\n' "$qnap_vol"
            return 0
        fi
    fi

    for candidate in /share/CACHEDEV1_DATA /share/CACHEDEV2_DATA /share/MD0_DATA /share/HDA_DATA; do
        if [[ -d "$candidate" ]] && [[ -w "$candidate" ]]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done

    return 1
}

find_qnap_state_dir() {
    local candidate=""

    if [[ -n "$STATE_DIR" && "${STATE_DIR_SOURCE:-default}" != "default" ]]; then
        printf '%s\n' "$STATE_DIR"
        return 0
    fi
    if [[ -n "$STATE_DIR" ]] && [[ "$STATE_DIR" != "/var/lib/pulse-agent" ]] && \
       { [[ -d "$STATE_DIR" ]] || [[ -f "$STATE_DIR/connection.env" ]] || [[ -f "$STATE_DIR/agent-id" ]]; }; then
        printf '%s\n' "$STATE_DIR"
        return 0
    fi

    candidate=$(detect_qnap_data_volume || true)
    if [[ -n "$candidate" ]]; then
        printf '%s\n' "${candidate}/.pulse-agent"
        return 0
    fi

    for candidate in /share/CACHEDEV1_DATA/.pulse-agent /share/CACHEDEV2_DATA/.pulse-agent /share/MD0_DATA/.pulse-agent /share/HDA_DATA/.pulse-agent; do
        if [[ -d "$candidate" ]] || [[ -f "$candidate/connection.env" ]] || [[ -f "$candidate/agent-id" ]]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done

    return 1
}

remove_qnap_autorun_block() {
    local autorun_path="$1"

    portable_sed_in_place '/^# Pulse Agent bootstrap begin$/,/^# Pulse Agent bootstrap end$/d' "$autorun_path"
    portable_sed_in_place '/^# Pulse Agent$/d' "$autorun_path"
    portable_sed_in_place '/start-pulse-agent\.sh/d' "$autorun_path"
}

write_qnap_wrapper_script() {
    local wrapper_script="$1"
    local runtime_binary="$2"
    local stored_binary="$3"
    local log_dir="$4"
    local state_dir="$5"
    local service_env_lines="$SHELL_EXPORT_LINES"

    cat > "$wrapper_script" <<EOF
#!/bin/sh
# Pulse Agent startup script for QNAP
# Auto-generated by Pulse installer.

WATCHDOG_LOG="${log_dir}/${AGENT_NAME}-watchdog.log"
WATCHDOG_PIDFILE="${state_dir}/${AGENT_NAME}.watchdog.pid"
AGENT_PIDFILE="${state_dir}/${AGENT_NAME}.pid"
LOCK_DIR="${state_dir}/${AGENT_NAME}.watchdog.lock"
CURRENT_AGENT_PID=""

wait_for_file() {
    path="\$1"
    while [ ! -f "\$path" ]; do
        sleep 5
    done
}

# The watchdog log lives on the data volume but has no rotation, so cap it.
trim_watchdog_log() {
    [ -f "\$WATCHDOG_LOG" ] || return 0
    _size=\$(wc -c < "\$WATCHDOG_LOG" 2>/dev/null | tr -d ' \t')
    case "\$_size" in ''|*[!0-9]*) return 0 ;; esac
    if [ "\$_size" -gt 5242880 ]; then
        tail -c 1048576 "\$WATCHDOG_LOG" > "\${WATCHDOG_LOG}.tmp" 2>/dev/null && mv "\${WATCHDOG_LOG}.tmp" "\$WATCHDOG_LOG"
    fi
}

pid_is_live() {
    _pid="\$1"
    case "\$_pid" in ''|*[!0-9]*) return 1 ;; esac
    kill -0 "\$_pid" 2>/dev/null
}

cleanup_watchdog() {
    trap - EXIT INT TERM HUP

    if pid_is_live "\$CURRENT_AGENT_PID"; then
        kill "\$CURRENT_AGENT_PID" 2>/dev/null || true
        wait "\$CURRENT_AGENT_PID" 2>/dev/null || true
    fi

    if [ -f "\$AGENT_PIDFILE" ] && [ "\$(cat "\$AGENT_PIDFILE" 2>/dev/null)" = "\$CURRENT_AGENT_PID" ]; then
        rm -f "\$AGENT_PIDFILE"
    fi

    if [ -f "\$WATCHDOG_PIDFILE" ] && [ "\$(cat "\$WATCHDOG_PIDFILE" 2>/dev/null)" = "\$\$" ]; then
        rm -f "\$WATCHDOG_PIDFILE"
        rmdir "\$LOCK_DIR" 2>/dev/null || true
    fi
}

shutdown_watchdog() {
    cleanup_watchdog
    exit 0
}

acquire_watchdog_lock() {
    _waited=0
    while ! mkdir "\$LOCK_DIR" 2>/dev/null; do
        _owner=\$(cat "\$WATCHDOG_PIDFILE" 2>/dev/null || true)
        if pid_is_live "\$_owner"; then
            echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] Another QNAP watchdog is already running (pid \$_owner); exiting." >> "\$WATCHDOG_LOG"
            return 1
        fi

        # Give a new owner time to publish its PID before deciding the lock is
        # stale. This closes the small race between mkdir and the PID write.
        if [ "\$_waited" -lt 2 ]; then
            sleep 1
            _waited=\$((_waited + 1))
            continue
        fi

        # The owner no longer exists. Remove only the stale singleton metadata,
        # then race safely with any other starter for a fresh atomic mkdir.
        rm -f "\$WATCHDOG_PIDFILE"
        rmdir "\$LOCK_DIR" 2>/dev/null || true
        _waited=0
    done

    echo "\$\$" > "\$WATCHDOG_PIDFILE"
    return 0
}

mkdir -p "${state_dir}" "${log_dir}" 2>/dev/null || true
if ! acquire_watchdog_lock; then
    exit 0
fi
trap cleanup_watchdog EXIT
trap shutdown_watchdog INT TERM HUP

wait_for_file "${stored_binary}"

# The singleton owner may replace an orphan left by an older installer.
pkill -x "pulse-agent" 2>/dev/null || true
sleep 2

# When the runtime binary lives on the data volume it IS the stored binary;
# only a split layout needs the boot-time copy back onto the root.
if [ "${stored_binary}" != "${runtime_binary}" ]; then
    mkdir -p "$(dirname "$runtime_binary")" 2>/dev/null || true
    cp "${stored_binary}" "${runtime_binary}"
fi
chmod +x "${runtime_binary}"${service_env_lines}

# Watchdog loop: restart agent if it exits.
RESTART_DELAY=5
MAX_RESTART_DELAY=60

while true; do
    trim_watchdog_log
    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] Starting pulse-agent (agent log: ${log_dir}/${AGENT_NAME}.log)..." >> "\$WATCHDOG_LOG"
    # The agent writes its own rotating log via --log-file; discard the stdout
    # mirror so nothing accumulates on the RAM-backed root filesystem.
    ${runtime_binary} ${EXEC_ARGS} > /dev/null 2>> "\$WATCHDOG_LOG" &
    CURRENT_AGENT_PID=\$!
    echo "\$CURRENT_AGENT_PID" > "\$AGENT_PIDFILE"
    wait "\$CURRENT_AGENT_PID"
    EXIT_CODE=\$?
    rm -f "\$AGENT_PIDFILE"
    CURRENT_AGENT_PID=""

    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] pulse-agent exited with code \$EXIT_CODE, restarting in \${RESTART_DELAY}s..." >> "\$WATCHDOG_LOG"
    sleep \$RESTART_DELAY

    RESTART_DELAY=\$((RESTART_DELAY * 2))
    if [ \$RESTART_DELAY -gt \$MAX_RESTART_DELAY ]; then
        RESTART_DELAY=\$MAX_RESTART_DELAY
    fi
done
EOF

    chmod +x "$wrapper_script"
}

append_qnap_autorun_block() {
    local autorun_path="$1"
    local wrapper_script="$2"
    local state_dir="$3"

    remove_qnap_autorun_block "$autorun_path"
    if [[ ! -f "$autorun_path" ]]; then
        echo "#!/bin/sh" > "$autorun_path"
    fi

    cat >> "$autorun_path" <<EOF

# Pulse Agent bootstrap begin
(
    _pulse_wrapper="${wrapper_script}"
    _pulse_log="/var/log/${AGENT_NAME}.log"
    _pulse_waited=0
    _pulse_wait_max=1800
    while [ ! -x "\$_pulse_wrapper" ] && [ "\$_pulse_waited" -lt "\$_pulse_wait_max" ]; do
        sleep 2
        _pulse_waited=\$((_pulse_waited + 2))
    done
    if [ -x "\$_pulse_wrapper" ]; then
        [ "\$_pulse_waited" -gt 0 ] && echo "\$(date '+%Y-%m-%d %H:%M:%S') [pulse-agent-autorun] data volume available after \${_pulse_waited}s" >> "\$_pulse_log"
        "\$_pulse_wrapper" >> "\$_pulse_log" 2>&1
    else
        echo "\$(date '+%Y-%m-%d %H:%M:%S') [pulse-agent-autorun] timed out after \${_pulse_wait_max}s waiting for \$_pulse_wrapper" >> "\$_pulse_log"
    fi
) >> /var/log/${AGENT_NAME}.log 2>&1 &
# Pulse Agent bootstrap end

EOF

    chmod +x "$autorun_path"
}

# --- Auto-Detection Functions ---
detect_docker() {
    # Check if Docker is available and accessible
    if command -v docker &>/dev/null; then
        # Try to connect to Docker daemon
        if docker info &>/dev/null 2>&1; then
            return 0
        else
            log_warn "Docker binary found ($(command -v docker)) but 'docker info' failed. Is the daemon running?"
        fi
    fi
    # Also check for Podman (Docker-compatible)
    if command -v podman &>/dev/null; then
        if podman info &>/dev/null 2>&1; then
            return 0
        else
            log_warn "Podman binary found but 'podman info' failed."
        fi
    fi
    if discover_rootless_container_runtime; then
        return 0
    fi
    return 1
}

discover_single_socket_match() {
    local pattern="$1"
    local matches=()
    local candidate=""

    for candidate in $pattern; do
        [[ -S "$candidate" ]] || continue
        matches+=("$candidate")
    done

    case "${#matches[@]}" in
        0)
            return 1
            ;;
        1)
            printf '%s\n' "${matches[0]}"
            return 0
            ;;
        *)
            printf '%s\n' "__AMBIGUOUS__"
            return 0
            ;;
    esac
}

system_docker_runtime_is_active() {
    # True when a rootful/system Docker daemon is actually answering. Checked
    # before any rootless socket discovery (issue #1647): a transient rootless
    # Podman API socket (socket-activated for a login session) must never
    # outrank a working system Docker, or the agent service gets pinned to a
    # socket that disappears when the session ends.
    local socket_path="${PULSE_SYSTEM_DOCKER_SOCKET:-/var/run/docker.sock}"

    if command -v docker &>/dev/null; then
        # Strip DOCKER_HOST/CONTAINER_HOST so only the default system daemon counts.
        if (unset DOCKER_HOST CONTAINER_HOST; docker info &>/dev/null); then
            return 0
        fi
    fi

    if [[ -S "$socket_path" ]]; then
        if command -v curl &>/dev/null; then
            if curl -sf --max-time 3 --unix-socket "$socket_path" http://localhost/_ping &>/dev/null; then
                return 0
            fi
            return 1
        fi
        # Socket exists but no way to probe it; assume the daemon owns it.
        return 0
    fi

    return 1
}

discover_rootless_container_runtime() {
    local docker_match=""
    local podman_match=""
    local rootless_root="${PULSE_ROOTLESS_RUNTIME_ROOT:-/run/user}"

    ROOTLESS_RUNTIME_KIND=""
    ROOTLESS_RUNTIME_SOCKET_PATH=""
    ROOTLESS_RUNTIME_SOCKET_URI=""
    ROOTLESS_RUNTIME_XDG_DIR=""

    if [[ "$(uname -s)" != "Linux" ]]; then
        return 1
    fi

    if system_docker_runtime_is_active; then
        # A live rootful Docker outranks any rootless socket (issue #1647).
        return 1
    fi

    docker_match=$(discover_single_socket_match "${rootless_root}/*/docker.sock" || true)
    podman_match=$(discover_single_socket_match "${rootless_root}/*/podman/podman.sock" || true)

    if [[ "$docker_match" == "__AMBIGUOUS__" ]]; then
        log_warn "Multiple rootless Docker sockets found under /run/user; not auto-selecting one."
    elif [[ -n "$docker_match" ]]; then
        ROOTLESS_RUNTIME_KIND="docker"
        ROOTLESS_RUNTIME_SOCKET_PATH="$docker_match"
        ROOTLESS_RUNTIME_SOCKET_URI="unix://${docker_match}"
        ROOTLESS_RUNTIME_XDG_DIR="$(dirname "$docker_match")"
        return 0
    fi

    if [[ "$podman_match" == "__AMBIGUOUS__" ]]; then
        log_warn "Multiple rootless Podman sockets found under /run/user; not auto-selecting one."
    elif [[ -n "$podman_match" ]]; then
        ROOTLESS_RUNTIME_KIND="podman"
        ROOTLESS_RUNTIME_SOCKET_PATH="$podman_match"
        ROOTLESS_RUNTIME_SOCKET_URI="unix://${podman_match}"
        ROOTLESS_RUNTIME_XDG_DIR="${podman_match%/podman/podman.sock}"
        return 0
    fi

    return 1
}

safe_profile_selected_rootless_runtime_usable() {
    local collector_uid=""
    local socket_uid=""
    [[ -n "$ROOTLESS_RUNTIME_KIND" && -S "$ROOTLESS_RUNTIME_SOCKET_PATH" ]] || return 1
    collector_uid=$(id -u "$LEAST_PRIVILEGE_USER" 2>/dev/null || true)
    socket_uid=$(stat -c '%u' "$ROOTLESS_RUNTIME_SOCKET_PATH" 2>/dev/null || true)
    [[ -n "$collector_uid" && "$socket_uid" == "$collector_uid" ]] || return 1
    command -v runuser >/dev/null 2>&1 || return 1
    runuser -u "$LEAST_PRIVILEGE_USER" -- test -r "$ROOTLESS_RUNTIME_SOCKET_PATH" || return 1
    runuser -u "$LEAST_PRIVILEGE_USER" -- test -w "$ROOTLESS_RUNTIME_SOCKET_PATH" || return 1
}

safe_profile_apply_docker_degradation() {
    [[ "$SAFE_PROFILE_ACTION" == "apply" && "$ENABLE_DOCKER" == "true" ]] || return 0
    if safe_profile_selected_rootless_runtime_usable; then
        log_info "Safe-profile migration preserved container monitoring through the collector-owned ${ROOTLESS_RUNTIME_KIND} socket: ${ROOTLESS_RUNTIME_SOCKET_PATH}"
        return 0
    fi
    if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
        log_warn "Safe-profile migration preserved rootful container inventory through the typed helper in summary-only mode. Container stats, images, storage, Swarm, update checks, and lifecycle actions remain unavailable without a collector-owned rootless socket."
        return 0
    fi
    ENABLE_DOCKER="false"
    DOCKER_EXPLICIT="true"
    log_warn "Safe-profile migration disabled rootful Docker monitoring: neither a usable collector-owned rootless runtime nor typed helper inventory is available."
}

detect_kubernetes() {
    # If user already specified a kubeconfig path, just verify it exists
    if [[ -n "$KUBECONFIG_PATH" ]]; then
        if [[ -f "$KUBECONFIG_PATH" ]]; then
            return 0
        else
            log_warn "Specified kubeconfig not found: $KUBECONFIG_PATH"
            return 1
        fi
    fi

    # Check for kubectl and cluster access
    if command -v kubectl &>/dev/null; then
        # Try to connect to cluster (quick timeout)
        if timeout 3 kubectl cluster-info &>/dev/null 2>&1; then
            # kubectl works, try to find the kubeconfig it's using
            if [[ -n "${KUBECONFIG:-}" ]] && [[ -f "${KUBECONFIG:-}" ]]; then
                KUBECONFIG_PATH="${KUBECONFIG}"
            elif [[ -f "${HOME}/.kube/config" ]]; then
                KUBECONFIG_PATH="${HOME}/.kube/config"
            fi
            return 0
        fi
    fi

    # Search for kubeconfig in common locations
    # Priority: /etc/kubernetes/admin.conf (standard k8s), then user home directories
    local search_paths=(
        "/etc/kubernetes/admin.conf"
        "/root/.kube/config"
    )
    
    # Add all user home directories
    for user_home in /home/*; do
        if [[ -d "$user_home/.kube" ]]; then
            search_paths+=("$user_home/.kube/config")
        fi
    done
    
    for kconfig in "${search_paths[@]}"; do
        if [[ -f "$kconfig" ]]; then
            KUBECONFIG_PATH="$kconfig"
            log_info "Found kubeconfig at: $KUBECONFIG_PATH"
            return 0
        fi
    done

    # Check if running inside a Kubernetes pod (in-cluster config)
    if [[ -f "/var/run/secrets/kubernetes.io/serviceaccount/token" ]]; then
        # In-cluster config doesn't need a kubeconfig file
        return 0
    fi
    return 1
}

detect_proxmox() {
    # Check for Proxmox VE
    if [[ -d "/etc/pve" ]]; then
        return 0
    fi
    # Check for Proxmox Backup Server
    if [[ -d "/etc/proxmox-backup" ]]; then
        return 0
    fi
    # Check for pveversion command
    if command -v pveversion &>/dev/null; then
        return 0
    fi
    # Check for proxmox-backup-manager command
    if command -v proxmox-backup-manager &>/dev/null; then
        return 0
    fi
    return 1
}

pulse_url_uses_plain_http() {
    local url_lower
    url_lower=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
    [[ "$url_lower" =~ ^http:// ]]
}

auto_enable_insecure_for_plain_http_url() {
    if [[ "$INSECURE" == "true" ]]; then
        return 0
    fi
    if ! pulse_url_uses_plain_http "$PULSE_URL"; then
        return 0
    fi
    INSECURE="true"
    log_info "Plain HTTP Pulse URL detected; enabling insecure mode for installer downloads and persisted agent update checks."
}

verify_pinned_server_certificate() {
    if [[ -z "$SERVER_FINGERPRINT" ]]; then
        return 0
    fi
    if pulse_url_uses_plain_http "$PULSE_URL"; then
        fail "--server-fingerprint requires an https:// Pulse URL." "$EXIT_PREFLIGHT_FAILED"
    fi
    if ! command -v openssl >/dev/null 2>&1; then
        fail "--server-fingerprint requires openssl so the installer can verify the server certificate before downloading." "$EXIT_PREFLIGHT_FAILED"
    fi

    local normalized=""
    local authority=""
    local host=""
    local target=""
    local actual=""
    normalized=$(printf '%s' "$SERVER_FINGERPRINT" | tr -d ':[:space:]' | tr '[:upper:]' '[:lower:]')
    if [[ ! "$normalized" =~ ^[a-f0-9]{64}$ ]]; then
        fail "Invalid --server-fingerprint value. Expected a SHA-256 certificate fingerprint (64 hexadecimal characters)." "$EXIT_PREFLIGHT_FAILED"
    fi

    authority="${PULSE_URL#https://}"
    authority="${authority%%/*}"
    if [[ "$authority" == \[*\]* ]]; then
        host="${authority#\[}"
        host="${host%%\]*}"
        target="$authority"
        if [[ "$authority" != *"]:"* ]]; then target="${authority}:443"; fi
    else
        host="${authority%%:*}"
        target="$authority"
        if [[ "$authority" != *:* ]]; then target="${authority}:443"; fi
    fi

    actual=$(openssl s_client -connect "$target" -servername "$host" </dev/null 2>/dev/null \
        | openssl x509 -outform DER 2>/dev/null \
        | openssl dgst -sha256 2>/dev/null \
        | awk '{print tolower($NF)}')
    if [[ -z "$actual" || "$actual" != "$normalized" ]]; then
        fail "Pulse server certificate fingerprint mismatch. Expected ${normalized}, got ${actual:-unavailable}." "$EXIT_PREFLIGHT_FAILED"
    fi

    SERVER_FINGERPRINT="$normalized"
    # curl must accept the self-signed chain after the explicit pin check. The
    # downloaded installer and binary still require their own signatures.
    INSECURE="true"
    log_info "Pulse server certificate fingerprint verified."
}

build_exec_arg_items() {
    local include_token="${1:-true}"

    EXEC_ARG_ITEMS=(--url "$PULSE_URL" --interval "$INTERVAL")
    if [[ "$include_token" == "true" && -n "$PULSE_TOKEN" ]]; then
        if [[ -n "$RUNTIME_TOKEN_FILE" ]]; then
            EXEC_ARG_ITEMS+=(--token-file "$RUNTIME_TOKEN_FILE")
        else
            fail "Internal installer error: runtime token file was not prepared before service rendering." "$EXIT_GENERAL"
        fi
    fi
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARG_ITEMS+=(--enable-host)
    else
        EXEC_ARG_ITEMS+=(--enable-host=false)
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARG_ITEMS+=(--enable-docker); fi
    # Pass explicit false when Docker was explicitly disabled (prevents auto-detection)
    if [[ "$ENABLE_DOCKER" == "false" && "$DOCKER_EXPLICIT" == "true" ]]; then EXEC_ARG_ITEMS+=(--enable-docker=false); fi
    if [[ "$ENABLE_KUBERNETES" == "true" ]]; then EXEC_ARG_ITEMS+=(--enable-kubernetes); fi
    if [[ -n "$KUBECONFIG_PATH" ]]; then EXEC_ARG_ITEMS+=(--kubeconfig "$KUBECONFIG_PATH"); fi
    if [[ "$ENABLE_PROXMOX" == "true" ]]; then EXEC_ARG_ITEMS+=(--enable-proxmox); fi
    if [[ -n "$PROXMOX_TYPE" ]]; then EXEC_ARG_ITEMS+=(--proxmox-type "$PROXMOX_TYPE"); fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARG_ITEMS+=(--insecure); fi
    if [[ -n "$SERVER_FINGERPRINT" ]]; then EXEC_ARG_ITEMS+=(--server-fingerprint "$SERVER_FINGERPRINT"); fi
    if [[ -n "$OBSERVERS_FILE" ]]; then EXEC_ARG_ITEMS+=(--observers-file "$OBSERVERS_FILE"); fi
    if [[ "$ENABLE_COMMANDS" == "true" ]]; then EXEC_ARG_ITEMS+=(--enable-commands); fi
    EXEC_ARG_ITEMS+=(--command-authority "${COMMAND_AUTHORITY:-legacy}")
    if [[ "$HEALTH_ADDR_SET" == "true" ]]; then EXEC_ARG_ITEMS+=(--health-addr "$HEALTH_ADDR"); fi
    if [[ "$ENROLL" == "true" ]]; then EXEC_ARG_ITEMS+=(--enroll); fi
    if [[ "$KUBE_INCLUDE_ALL_PODS" == "true" ]]; then EXEC_ARG_ITEMS+=(--kube-include-all-pods); fi
    if [[ "$KUBE_INCLUDE_ALL_DEPLOYMENTS" == "true" ]]; then EXEC_ARG_ITEMS+=(--kube-include-all-deployments); fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARG_ITEMS+=(--agent-id "$AGENT_ID"); fi
    if [[ -n "$HOSTNAME_OVERRIDE" ]]; then EXEC_ARG_ITEMS+=(--hostname "$HOSTNAME_OVERRIDE"); fi
    if [[ -n "$REPORT_IP" ]]; then EXEC_ARG_ITEMS+=(--report-ip "$REPORT_IP"); fi
    if [[ -n "$STATE_DIR" ]]; then EXEC_ARG_ITEMS+=(--state-dir "$STATE_DIR"); fi
    if [[ -n "${AGENT_LOG_FILE:-}" ]]; then EXEC_ARG_ITEMS+=(--log-file "$AGENT_LOG_FILE"); fi
    # Add disk exclude patterns (use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u)
    for pattern in ${DISK_EXCLUDES[@]+"${DISK_EXCLUDES[@]}"}; do
        EXEC_ARG_ITEMS+=(--disk-exclude "$pattern")
    done
    for pattern in ${DISK_INCLUDES[@]+"${DISK_INCLUDES[@]}"}; do
        EXEC_ARG_ITEMS+=(--disk-include "$pattern")
    done
}

join_exec_arg_items() {
    local joined=""
    local arg=""
    local quoted=""

    for arg in ${EXEC_ARG_ITEMS[@]+"${EXEC_ARG_ITEMS[@]}"}; do
        printf -v quoted '%q' "$arg"
        if [[ -n "$joined" ]]; then
            joined="$joined "
        fi
        joined="${joined}${quoted}"
    done

    EXEC_ARGS="$joined"
}

# Build exec args string for use in service files
# Returns via EXEC_ARGS variable
build_exec_args() {
    build_exec_arg_items "true"
    join_exec_arg_items
}

build_exec_args_without_token() {
    build_exec_arg_items "false"
    join_exec_arg_items
}

# Build exec args as array for direct execution (proper quoting)
# Returns via EXEC_ARGS_ARRAY variable
build_exec_args_array() {
    build_exec_arg_items "true"
    EXEC_ARGS_ARRAY=("${EXEC_ARG_ITEMS[@]}")
}

xml_escape() {
    local value="$1"
    value="${value//&/&amp;}"
    value="${value//</&lt;}"
    value="${value//>/&gt;}"
    printf '%s' "$value"
}

append_plist_arg() {
    local arg="$1"
    PLIST_ARGS="${PLIST_ARGS}
        <string>$(xml_escape "$arg")</string>"
}

build_plist_program_arguments() {
    local executable="$1"
    PLIST_ARGS=""
    append_plist_arg "$executable"
    build_exec_arg_items "true"

    local arg=""
    for arg in ${EXEC_ARG_ITEMS[@]+"${EXEC_ARG_ITEMS[@]}"}; do
        append_plist_arg "$arg"
    done
}

ensure_runtime_token_file() {
    local state_dir="${1:-$STATE_DIR}"
    local token_file="${state_dir}/token"
    local previous_token=""
    local old_umask=""
    local token_tmp=""

    RUNTIME_TOKEN_FILE=""
    RUNTIME_TOKEN_CHANGED="false"
    if [[ -z "$PULSE_TOKEN" ]]; then
        rm -f "$token_file" 2>/dev/null || true
        log_info "No API token provided; installer will configure token-optional agent runtime."
        return 0
    fi

    old_umask=$(umask)
    umask 077
    mkdir -p "$state_dir"
    chmod 700 "$state_dir"
    if [[ -f "$token_file" && ! -L "$token_file" ]]; then
        previous_token=$(cat "$token_file" 2>/dev/null || true)
    fi
    if [[ "$previous_token" != "$PULSE_TOKEN" ]]; then
        RUNTIME_TOKEN_CHANGED="true"
    fi
    token_tmp=$(mktemp "${state_dir}/.token.XXXXXX")
    TMP_FILES+=("$token_tmp")
    if ! printf '%s' "$PULSE_TOKEN" > "$token_tmp"; then
        umask "$old_umask"
        fail "Failed to write runtime token file: $token_file" "$EXIT_GENERAL"
    fi
    chmod 600 "$token_tmp"
    if [[ "$(id -u 2>/dev/null || echo 1)" == "0" ]]; then
        chown root:root "$token_tmp" 2>/dev/null || true
    fi
    if ! mv -f "$token_tmp" "$token_file"; then
        umask "$old_umask"
        fail "Failed to install runtime token file: $token_file" "$EXIT_GENERAL"
    fi
    umask "$old_umask"
    RUNTIME_TOKEN_FILE="$token_file"
    # A changed bootstrap token is explicit re-enrollment intent. Preserve the
    # runtime token across ordinary restarts and tokenless updates, but do not
    # let a stale runtime token shadow fresh enrollment credentials.
    if [[ "${ENROLL:-false}" == "true" && "$RUNTIME_TOKEN_CHANGED" == "true" ]]; then
        rm -f "${state_dir}/runtime.token" 2>/dev/null || true
    fi
    log_info "Token stored securely at $token_file (mode 600)"
}

clear_proxmox_state_if_needed() {
    if [[ "$ENABLE_PROXMOX" != "true" ]]; then
        return 0
    fi
    log_info "Clearing Proxmox state for fresh registration..."
    rm -f "${STATE_DIR}/proxmox-registered" 2>/dev/null || true
    rm -f "${STATE_DIR}/proxmox-pve-registered" 2>/dev/null || true
    rm -f "${STATE_DIR}/proxmox-pbs-registered" 2>/dev/null || true
    rm -f "${STATE_DIR}/proxmox-pve-registration-blocked" 2>/dev/null || true
    rm -f "${STATE_DIR}/proxmox-pbs-registration-blocked" 2>/dev/null || true
    rm -f "${STATE_DIR}/proxmox-detected-types" 2>/dev/null || true
}

write_connection_state_value() {
    local file="$1"
    local key="$2"
    local value="$3"

    if [[ -z "$value" ]]; then
        return 0
    fi

    printf "%s='%s'\n" "$key" "$value" >> "$file"
}

read_connection_state_value() {
    local file="$1"
    local key="$2"

    if [[ ! -f "$file" ]]; then
        return 0
    fi

    awk -F= -v key="$key" '
        $1 == key {
            value = substr($0, index($0, "=") + 1)
            sub(/^'\''/, "", value)
            sub(/'\''$/, "", value)
            print value
            exit
        }
    ' "$file" 2>/dev/null || true
}

recover_token_from_default_agent_token_file() {
    local token_path=""
    local recovered_token=""

    if [[ -n "$PULSE_TOKEN" ]]; then
        return 0
    fi

    # v5.1.x Linux services could omit --token and --token-file because the
    # Go agent read this default file itself.
    local token_paths=("${STATE_DIR%/}/token")
    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        token_paths+=("${DEFAULT_STATE_DIR:-/var/lib/pulse-agent}/token" "$TRUENAS_STATE_DIR/token")
    fi
    for token_path in "${token_paths[@]}"; do
        [[ -n "$token_path" && -f "$token_path" ]] || continue
        recovered_token=$(cat "$token_path" 2>/dev/null || true)
        if [[ -n "$recovered_token" ]]; then
            PULSE_TOKEN="$recovered_token"
            return 0
        fi
    done

    return 1
}

recover_connection_state() {
    local file="$1"
    local saved_state_dir=""

    saved_state_dir=$(read_connection_state_value "$file" "PULSE_STATE_DIR")
    if [[ -n "$saved_state_dir" && "$saved_state_dir" == /* && "$saved_state_dir" != "/" &&
          "$saved_state_dir" != *$'\r'* && "$saved_state_dir" != *$'\n'* &&
          "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        STATE_DIR="$saved_state_dir"
        STATE_DIR_SOURCE="recovered"
    fi

    if [[ -z "$PULSE_URL" ]]; then
        PULSE_URL=$(read_connection_state_value "$file" "PULSE_URL")
    fi
    if [[ -z "$PULSE_TOKEN" ]]; then
        PULSE_TOKEN=$(read_connection_state_value "$file" "PULSE_TOKEN")
    fi
    if [[ -z "$PULSE_TOKEN" ]]; then
        local saved_token_file=""
        saved_token_file=$(read_connection_state_value "$file" "PULSE_TOKEN_FILE")
        if [[ -n "$saved_token_file" && -f "$saved_token_file" ]]; then
            PULSE_TOKEN=$(cat "$saved_token_file")
        fi
    fi
    if [[ -z "$PULSE_TOKEN" && -n "$PULSE_URL" ]]; then
        recover_token_from_default_agent_token_file || true
    fi
    if [[ -z "$AGENT_ID" ]]; then
        AGENT_ID=$(read_connection_state_value "$file" "PULSE_AGENT_ID")
    fi
    if [[ -z "$HOSTNAME_OVERRIDE" ]]; then
        HOSTNAME_OVERRIDE=$(read_connection_state_value "$file" "PULSE_HOSTNAME")
    fi
    if [[ -z "$REPORT_IP" ]]; then
        REPORT_IP=$(read_connection_state_value "$file" "PULSE_REPORT_IP")
    fi
    if [[ "${RETARGET_ONLY:-false}" != "true" && "$INSECURE" != "true" ]]; then
        local saved_insecure=""
        saved_insecure=$(read_connection_state_value "$file" "PULSE_INSECURE_SKIP_VERIFY")
        if [[ "$saved_insecure" == "true" ]]; then
            INSECURE="true"
        fi
    fi
    if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$SERVER_FINGERPRINT" ]]; then
        SERVER_FINGERPRINT=$(read_connection_state_value "$file" "PULSE_SERVER_FINGERPRINT")
    fi
    if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$CURL_CA_BUNDLE" ]]; then
        CURL_CA_BUNDLE=$(read_connection_state_value "$file" "PULSE_CACERT")
    fi
}

strip_recovered_arg_quotes() {
    local value="$1"

    case "$value" in
        \"*\") value="${value#\"}"; value="${value%\"}" ;;
        \'*\') value="${value#\'}"; value="${value%\'}" ;;
    esac

    printf '%s\n' "$value"
}

normalize_recovered_agent_arg_key() {
    local key="$1"

    key="${key#--}"
    key="${key#-}"
    printf '%s\n' "$key"
}

apply_recovered_agent_arg_value() {
    local key="$1"
    local value="$2"

    key=$(normalize_recovered_agent_arg_key "$key")
    value=$(strip_recovered_arg_quotes "$value")

    case "$key" in
        url|pulse-url)
            if [[ -z "$PULSE_URL" ]]; then PULSE_URL="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        token)
            if [[ -z "$PULSE_TOKEN" ]]; then PULSE_TOKEN="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        token-file)
            if [[ -z "$PULSE_TOKEN" && -n "$value" && -f "$value" ]]; then
                PULSE_TOKEN=$(cat "$value")
            fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        interval)
            if [[ "$INTERVAL_EXPLICIT" != "true" && -n "$value" ]]; then INTERVAL="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        agent-id)
            if [[ -z "$AGENT_ID" ]]; then AGENT_ID="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        hostname)
            if [[ -z "$HOSTNAME_OVERRIDE" ]]; then HOSTNAME_OVERRIDE="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        report-ip)
            if [[ -z "$REPORT_IP" ]]; then REPORT_IP="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        cacert)
            if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$CURL_CA_BUNDLE" ]]; then CURL_CA_BUNDLE="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        server-fingerprint)
            if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$SERVER_FINGERPRINT" ]]; then SERVER_FINGERPRINT="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        observers-file)
            if [[ -z "$OBSERVERS_FILE" ]]; then OBSERVERS_FILE="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        command-authority)
            if [[ -z "$COMMAND_AUTHORITY_SOURCE" ]]; then
                COMMAND_AUTHORITY="$value"
                COMMAND_AUTHORITY_SOURCE="recovered"
            fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        health-addr)
            if [[ "$HEALTH_ADDR_SET" != "true" ]]; then
                HEALTH_ADDR="$value"
                HEALTH_ADDR_SET="true"
            fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        state-dir)
            if [[ -n "$value" && "$value" == /* && "$value" != "/" &&
                  "$value" != *$'\r'* && "$value" != *$'\n'* &&
                  "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
                STATE_DIR="$value"
                STATE_DIR_SOURCE="recovered"
            fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        kubeconfig)
            if [[ "$KUBERNETES_EXPLICIT" != "true" ]]; then
                KUBECONFIG_PATH="$value"
                KUBERNETES_EXPLICIT="true"
                ENABLE_KUBERNETES="true"
            fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        proxmox-type)
            if [[ -z "$PROXMOX_TYPE" ]]; then PROXMOX_TYPE="$value"; fi
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        disk-exclude)
            DISK_EXCLUDES+=("$value")
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
        disk-include)
            DISK_INCLUDES+=("$value")
            RECOVERED_AGENT_ARG_STATE="true"
            ;;
    esac
}

recovered_connection_state_ready() {
    [[ -n "$PULSE_URL" && -n "$PULSE_TOKEN" ]]
}

update_connection_state_incomplete() {
    [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" || -z "$AGENT_ID" || -z "$HOSTNAME_OVERRIDE" || -z "$CURL_CA_BUNDLE" || -z "$SERVER_FINGERPRINT" || "$INSECURE" != "true" ]]
}

resolve_command_authority_profile() {
    if [[ -z "$COMMAND_AUTHORITY" ]]; then
        if [[ "$ENABLE_COMMANDS" == "true" ]]; then
            COMMAND_AUTHORITY="command-capable"
        elif [[ "$UPDATE_ONLY" == "true" ]]; then
            # Services installed before this marker existed may have been
            # remotely promoted. Preserve that explicitly during update; fresh
            # monitoring installs are locked below.
            COMMAND_AUTHORITY="legacy"
        else
            COMMAND_AUTHORITY="monitoring-only"
        fi
    fi

    case "$COMMAND_AUTHORITY" in
        monitoring-only|command-capable|legacy)
            ;;
        *)
            fail "Invalid --command-authority value: ${COMMAND_AUTHORITY} (expected monitoring-only, command-capable, or legacy)" "$EXIT_MISSING_ARGS"
            ;;
    esac

    if [[ "$COMMAND_AUTHORITY" == "monitoring-only" && "$ENABLE_COMMANDS" == "true" ]]; then
        fail "--enable-commands conflicts with --command-authority monitoring-only" "$EXIT_MISSING_ARGS"
    fi
}

recover_connection_state_from_arg_stream() {
    local arg=""
    local pending_key=""
    local key=""
    local value=""

    RECOVERED_AGENT_ARG_STATE="false"

    while IFS= read -r arg; do
        if [[ -n "$pending_key" ]]; then
            apply_recovered_agent_arg_value "$pending_key" "$arg"
            pending_key=""
            continue
        fi

        case "$arg" in
            --url|--pulse-url|--token|--token-file|--interval|--agent-id|--hostname|--report-ip|--cacert|--server-fingerprint|--observers-file|--command-authority|--health-addr|--state-dir|--kubeconfig|--proxmox-type|--disk-exclude|--disk-include|-url|-pulse-url|-token|-token-file|-interval|-agent-id|-hostname|-report-ip|-cacert|-server-fingerprint|-observers-file|-command-authority|-health-addr|-state-dir|-kubeconfig|-proxmox-type|-disk-exclude|-disk-include)
                pending_key=$(normalize_recovered_agent_arg_key "$arg")
                ;;
            --url=*|--pulse-url=*|--token=*|--token-file=*|--interval=*|--agent-id=*|--hostname=*|--report-ip=*|--cacert=*|--server-fingerprint=*|--observers-file=*|--command-authority=*|--health-addr=*|--state-dir=*|--kubeconfig=*|--proxmox-type=*|--disk-exclude=*|--disk-include=*|-url=*|-pulse-url=*|-token=*|-token-file=*|-interval=*|-agent-id=*|-hostname=*|-report-ip=*|-cacert=*|-server-fingerprint=*|-observers-file=*|-command-authority=*|-health-addr=*|-state-dir=*|-kubeconfig=*|-proxmox-type=*|-disk-exclude=*|-disk-include=*)
                key="${arg%%=*}"
                value="${arg#*=}"
                apply_recovered_agent_arg_value "$key" "$value"
                ;;
            --enable-host|-enable-host|--enable-host=true|-enable-host=true)
                if [[ "$HOST_EXPLICIT" != "true" ]]; then ENABLE_HOST="true"; fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-host=false|-enable-host=false|--disable-host|-disable-host)
                if [[ "$HOST_EXPLICIT" != "true" ]]; then ENABLE_HOST="false"; fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-docker|-enable-docker|--enable-docker=true|-enable-docker=true)
                if [[ "$DOCKER_EXPLICIT" != "true" ]]; then
                    ENABLE_DOCKER="true"
                    DOCKER_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-docker=false|-enable-docker=false|--disable-docker|-disable-docker)
                if [[ "$DOCKER_EXPLICIT" != "true" ]]; then
                    ENABLE_DOCKER="false"
                    DOCKER_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-kubernetes|-enable-kubernetes|--enable-kubernetes=true|-enable-kubernetes=true)
                if [[ "$KUBERNETES_EXPLICIT" != "true" ]]; then
                    ENABLE_KUBERNETES="true"
                    KUBERNETES_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-kubernetes=false|-enable-kubernetes=false|--disable-kubernetes|-disable-kubernetes)
                if [[ "$KUBERNETES_EXPLICIT" != "true" ]]; then
                    ENABLE_KUBERNETES="false"
                    KUBERNETES_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-proxmox|-enable-proxmox|--enable-proxmox=true|-enable-proxmox=true)
                if [[ "$PROXMOX_EXPLICIT" != "true" ]]; then
                    ENABLE_PROXMOX="true"
                    PROXMOX_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-proxmox=false|-enable-proxmox=false|--disable-proxmox|-disable-proxmox)
                if [[ "$PROXMOX_EXPLICIT" != "true" ]]; then
                    ENABLE_PROXMOX="false"
                    PROXMOX_EXPLICIT="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --insecure|-insecure)
                if [[ "${RETARGET_ONLY:-false}" != "true" || "${INSECURE_EXPLICIT:-false}" == "true" ]]; then
                    INSECURE="true"
                fi
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enable-commands|-enable-commands)
                ENABLE_COMMANDS="true"
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --enroll|-enroll)
                ENROLL="true"
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --kube-include-all-pods|-kube-include-all-pods)
                KUBE_INCLUDE_ALL_PODS="true"
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
            --kube-include-all-deployments|-kube-include-all-deployments)
                KUBE_INCLUDE_ALL_DEPLOYMENTS="true"
                RECOVERED_AGENT_ARG_STATE="true"
                ;;
        esac
    done

    if [[ "$RECOVERED_AGENT_ARG_STATE" == "true" && -z "$PULSE_TOKEN" && -n "$PULSE_URL" ]]; then
        recover_token_from_default_agent_token_file || true
    fi

    [[ "$RECOVERED_AGENT_ARG_STATE" == "true" ]] && recovered_connection_state_ready
}

recover_connection_state_from_env_stream() {
    local env_line=""
    local value=""

    RECOVERED_AGENT_ENV_STATE="false"

    while IFS= read -r env_line; do
        case "$env_line" in
            PULSE_URL=*|PULSE_AGENT_URL=*|PULSE_AGENT_CONNECT_URL=*)
                value="${env_line#*=}"
                if [[ -z "$PULSE_URL" ]]; then PULSE_URL="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_TOKEN=*)
                value="${env_line#*=}"
                if [[ -z "$PULSE_TOKEN" ]]; then PULSE_TOKEN="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_TOKEN_FILE=*)
                value="${env_line#*=}"
                if [[ -z "$PULSE_TOKEN" && -n "$value" && -f "$value" ]]; then
                    PULSE_TOKEN=$(cat "$value")
                fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_STATE_DIR=*)
                value="${env_line#*=}"
                if [[ -n "$value" && "$value" == /* && "$value" != "/" &&
                      "$value" != *$'\r'* && "$value" != *$'\n'* &&
                      "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
                    STATE_DIR="$value"
                    STATE_DIR_SOURCE="recovered"
                fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_AGENT_ID=*)
                value="${env_line#*=}"
                if [[ -z "$AGENT_ID" ]]; then AGENT_ID="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_HOSTNAME=*)
                value="${env_line#*=}"
                if [[ -z "$HOSTNAME_OVERRIDE" ]]; then HOSTNAME_OVERRIDE="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_REPORT_IP=*)
                value="${env_line#*=}"
                if [[ -z "$REPORT_IP" ]]; then REPORT_IP="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_INSECURE_SKIP_VERIFY=true)
                if [[ "${RETARGET_ONLY:-false}" != "true" || "${INSECURE_EXPLICIT:-false}" == "true" ]]; then
                    INSECURE="true"
                fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_CACERT=*)
                value="${env_line#*=}"
                if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$CURL_CA_BUNDLE" ]]; then CURL_CA_BUNDLE="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
            PULSE_SERVER_FINGERPRINT=*)
                value="${env_line#*=}"
                if [[ "${RETARGET_ONLY:-false}" != "true" && -z "$SERVER_FINGERPRINT" ]]; then SERVER_FINGERPRINT="$value"; fi
                RECOVERED_AGENT_ENV_STATE="true"
                ;;
        esac
    done

    if [[ "$RECOVERED_AGENT_ENV_STATE" == "true" && -z "$PULSE_TOKEN" && -n "$PULSE_URL" ]]; then
        recover_token_from_default_agent_token_file || true
    fi

    [[ "$RECOVERED_AGENT_ENV_STATE" == "true" ]] && recovered_connection_state_ready
}

collect_running_agent_pids() {
    local path=""
    local pid=""

    if command -v pgrep >/dev/null 2>&1; then
        {
            pgrep -x "$BINARY_NAME" 2>/dev/null || true
            pgrep -f "/${BINARY_NAME}( |$)" 2>/dev/null || true
        } | awk '!seen[$0]++'
        return 0
    fi

    for path in /proc/[0-9]*/cmdline; do
        [[ -r "$path" ]] || continue
        if tr '\0' '\n' < "$path" 2>/dev/null | grep -qx ".*/${BINARY_NAME}"; then
            pid="${path%/cmdline}"
            printf '%s\n' "${pid##*/}"
        fi
    done
}

split_recovered_shell_words() {
    local payload="$1"
    local word=""
    local quote=""
    local char=""
    local escaped="false"
    local in_word="false"
    local index=0

    for ((index = 0; index < ${#payload}; index++)); do
        char="${payload:index:1}"

        if [[ "$escaped" == "true" ]]; then
            word+="$char"
            in_word="true"
            escaped="false"
            continue
        fi

        if [[ "$char" == "\\" && "$quote" != "'" ]]; then
            escaped="true"
            in_word="true"
            continue
        fi

        if [[ -n "$quote" ]]; then
            if [[ "$char" == "$quote" ]]; then
                quote=""
            else
                word+="$char"
            fi
            in_word="true"
            continue
        fi

        case "$char" in
            "'"|'"')
                quote="$char"
                in_word="true"
                ;;
            ' '|$'\t'|$'\n')
                if [[ "$in_word" == "true" ]]; then
                    printf '%s\n' "$word"
                    word=""
                    in_word="false"
                fi
                ;;
            *)
                word+="$char"
                in_word="true"
                ;;
        esac
    done

    if [[ "$escaped" == "true" ]]; then
        word+="\\"
    fi
    if [[ "$in_word" == "true" ]]; then
        printf '%s\n' "$word"
    fi

    [[ -z "$quote" ]]
}

running_agent_arg_stream() {
    local pid="$1"
    local proc_root="${2:-/proc}"
    local payload=""

    if [[ -r "${proc_root}/${pid}/cmdline" ]]; then
        tr '\0' '\n' < "${proc_root}/${pid}/cmdline" 2>/dev/null
        return 0
    fi

    command -v ps >/dev/null 2>&1 || return 1
    payload=$(ps -ww -o command= -p "$pid" 2>/dev/null || true)
    [[ -n "$payload" ]] || return 1
    split_recovered_shell_words "$payload"
}

running_agent_env_stream() {
    local pid="$1"
    local proc_root="${2:-/proc}"

    if [[ -r "${proc_root}/${pid}/environ" ]]; then
        tr '\0' '\n' < "${proc_root}/${pid}/environ" 2>/dev/null
        return 0
    fi

    # FreeBSD exposes process environment through procstat instead of procfs.
    command -v procstat >/dev/null 2>&1 || return 1
    procstat -e "$pid" 2>/dev/null | awk '
        NR > 1 {
            $1 = ""
            $2 = ""
            sub(/^[[:space:]]+/, "")
            print
        }
    ' | tr ' ' '\n'
}

recover_connection_state_from_running_agent() {
    local pid=""
    local recovered="false"

    while IFS= read -r pid; do
        [[ -n "$pid" && "$pid" != "$$" ]] || continue
        recovered="false"

        if recover_connection_state_from_arg_stream < <(running_agent_arg_stream "$pid"); then
            recovered="true"
        fi
        if recover_connection_state_from_env_stream < <(running_agent_env_stream "$pid"); then
            recovered="true"
        fi
        if [[ "$recovered" == "true" ]] && recovered_connection_state_ready; then
            return 0
        fi
    done < <(collect_running_agent_pids)

    return 1
}

recover_connection_state_from_systemd_unit() {
    local unit_path=""
    local candidate=""
    local line=""
    local payload=""
    local recovered="false"

    if command -v systemctl >/dev/null 2>&1; then
        unit_path=$(systemctl show -p FragmentPath --value "$AGENT_NAME" 2>/dev/null || true)
    fi

    for candidate in "$unit_path" "/etc/systemd/system/${AGENT_NAME}.service" "/lib/systemd/system/${AGENT_NAME}.service" "/usr/lib/systemd/system/${AGENT_NAME}.service"; do
        [[ -n "$candidate" && -f "$candidate" ]] || continue
        recovered="false"
        while IFS= read -r line; do
            case "$line" in
                ExecStart=*)
                    payload="${line#ExecStart=}"
                    if recover_connection_state_from_arg_stream < <(split_recovered_shell_words "$payload"); then
                        recovered="true"
                    fi
                    ;;
                Environment=*)
                    payload="${line#Environment=}"
                    if recover_connection_state_from_env_stream < <(printf '%s\n' "$payload" | tr ' ' '\n' | sed 's/^"//; s/"$//'); then
                        recovered="true"
                    fi
                    ;;
            esac
        done < "$candidate"
        if [[ "$recovered" == "true" ]] && recovered_connection_state_ready; then
            return 0
        fi
    done

    return 1
}

launchd_agent_arg_stream() {
    local plist_path="${1:-/Library/LaunchDaemons/com.pulse.agent.plist}"

    [[ -f "$plist_path" ]] || return 1
    awk '
        /<key>ProgramArguments<\/key>/ { in_program_args = 1; next }
        in_program_args && /<\/array>/ { exit }
        in_program_args && /<string>/ {
            value = $0
            sub(/^.*<string>/, "", value)
            sub(/<\/string>.*$/, "", value)
            gsub(/&amp;/, "\\&", value)
            gsub(/&lt;/, "<", value)
            gsub(/&gt;/, ">", value)
            gsub(/&quot;/, "\"", value)
            gsub(/&apos;/, "\047", value)
            print value
        }
    ' "$plist_path"
}

recover_connection_state_from_launchd_plist() {
    local plist_path="${1:-/Library/LaunchDaemons/com.pulse.agent.plist}"

    if recover_connection_state_from_arg_stream < <(launchd_agent_arg_stream "$plist_path"); then
        return 0
    fi
    return 1
}

recover_connection_state_from_service_scripts() {
    local candidate=""
    local line=""
    local payload=""
    local assignment=""
    local key=""
    local value=""
    local recovered="false"
    local candidates=(
        "/usr/local/etc/rc.d/${AGENT_NAME}"
        "${TRUENAS_STATE_DIR}/pulse-agent.service"
        "/etc/init.d/${AGENT_NAME}"
        "/etc/init/${AGENT_NAME}.conf"
        "/boot/config/plugins/pulse-agent/start-pulse-agent.sh"
    )

    if (( $# > 0 )); then
        candidates=("$@")
    fi

    for candidate in "${candidates[@]}"; do
        [[ -n "$candidate" && -f "$candidate" ]] || continue
        recovered="false"

        while IFS= read -r line; do
            line="${line#"${line%%[![:space:]]*}"}"
            case "$line" in
                command_args=*)
                    payload=$(strip_recovered_arg_quotes "${line#command_args=}")
                    if recover_connection_state_from_arg_stream < <(split_recovered_shell_words "$payload"); then
                        recovered="true"
                    fi
                    ;;
                exec\ *)
                    payload="${line#exec }"
                    if recover_connection_state_from_arg_stream < <(split_recovered_shell_words "$payload"); then
                        recovered="true"
                    fi
                    ;;
                export\ PULSE_*=*|PULSE_*=*)
                    assignment="${line#export }"
                    key="${assignment%%=*}"
                    value=$(strip_recovered_arg_quotes "${assignment#*=}")
                    if recover_connection_state_from_env_stream < <(printf '%s=%s\n' "$key" "$value"); then
                        recovered="true"
                    fi
                    ;;
            esac
        done < "$candidate"

        if [[ "$recovered" == "true" ]] && recovered_connection_state_ready; then
            return 0
        fi
    done

    return 1
}

recover_connection_state_from_existing_agent() {
    if recover_connection_state_from_running_agent; then
        log_info "Recovered connection details from the running Pulse Agent process."
        return 0
    fi
    if recover_connection_state_from_systemd_unit; then
        log_info "Recovered connection details from the existing Pulse Agent service."
        return 0
    fi
    if recover_connection_state_from_launchd_plist; then
        log_info "Recovered connection details from the existing Pulse Agent launchd service."
        return 0
    fi
    if recover_connection_state_from_service_scripts; then
        log_info "Recovered connection details from the existing Pulse Agent service script."
        return 0
    fi

    return 1
}

find_connection_state_file() {
    local conn_env=""
    local qnap_state_dir=""
    local conn_paths=("${STATE_DIR%/}/connection.env")

    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        conn_paths+=("${DEFAULT_STATE_DIR:-/var/lib/pulse-agent}/connection.env" /boot/config/plugins/pulse-agent/connection.env "$TRUENAS_STATE_DIR/connection.env")
    fi
    for conn_env in "${conn_paths[@]}"; do
        if [[ -f "$conn_env" ]]; then
            printf '%s\n' "$conn_env"
            return 0
        fi
    done

    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        qnap_state_dir=$(find_qnap_state_dir || true)
        if [[ -n "$qnap_state_dir" ]] && [[ -f "$qnap_state_dir/connection.env" ]]; then
            printf '%s\n' "$qnap_state_dir/connection.env"
            return 0
        fi
    fi

    return 1
}

recover_agent_id_from_state_file() {
    local aid_path=""
    local qnap_state_dir=""
    local aid_paths=("${STATE_DIR%/}/agent-id")

    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        aid_paths+=("${DEFAULT_STATE_DIR:-/var/lib/pulse-agent}/agent-id" /boot/config/plugins/pulse-agent/agent-id "$TRUENAS_STATE_DIR/agent-id")
    fi

    if [[ "${STATE_DIR_SOURCE:-default}" == "default" ]]; then
        qnap_state_dir=$(find_qnap_state_dir || true)
        if [[ -n "$qnap_state_dir" ]]; then
            aid_paths+=("$qnap_state_dir/agent-id")
        fi
    fi

    for aid_path in "${aid_paths[@]}"; do
        if [[ -f "$aid_path" ]]; then
            cat "$aid_path"
            return 0
        fi
    done

    return 1
}

# Save install script and connection details for offline uninstall
save_connection_info() {
    local state_dir="$1"
    local conn_env="${state_dir}/connection.env"
    local conn_tmp=""
    local old_umask=""
    old_umask=$(umask)
    umask 077
    mkdir -p "$state_dir"
    chmod 700 "$state_dir"
    # Save connection details so uninstall can deregister without --url/--token.
    # Single-quote values to prevent shell interpretation on read-back.
    # Legacy connection files may contain PULSE_TOKEN, but new installs persist
    # only the protected token file path.
    conn_tmp=$(mktemp "${state_dir}/.connection.env.XXXXXX")
    TMP_FILES+=("$conn_tmp")
    write_connection_state_value "$conn_tmp" "PULSE_STATE_DIR" "$state_dir"
    write_connection_state_value "$conn_tmp" "PULSE_URL" "$PULSE_URL"
    write_connection_state_value "$conn_tmp" "PULSE_TOKEN_FILE" "$RUNTIME_TOKEN_FILE"
    write_connection_state_value "$conn_tmp" "PULSE_AGENT_ID" "$AGENT_ID"
    write_connection_state_value "$conn_tmp" "PULSE_HOSTNAME" "$HOSTNAME_OVERRIDE"
    write_connection_state_value "$conn_tmp" "PULSE_REPORT_IP" "$REPORT_IP"
    if [[ "$INSECURE" == "true" ]]; then
        write_connection_state_value "$conn_tmp" "PULSE_INSECURE_SKIP_VERIFY" "true"
    fi
    write_connection_state_value "$conn_tmp" "PULSE_SERVER_FINGERPRINT" "$SERVER_FINGERPRINT"
    write_connection_state_value "$conn_tmp" "PULSE_CACERT" "$CURL_CA_BUNDLE"
    chmod 600 "$conn_tmp"
    mv -f "$conn_tmp" "$conn_env"
    umask "$old_umask"
    # Save a copy of this install script for offline uninstall.
    # When run via "curl | bash", $0 is /dev/stdin — not a usable file.
    # Try local copy first, then download a fresh copy from the server.
    local saved=false
    if [[ -f "$0" && "$0" != "/dev/stdin" && "$0" != "bash" && "$0" != "-bash" ]]; then
        if cp "$0" "${state_dir}/install.sh" 2>/dev/null; then
            saved=true
        fi
    fi
    if [[ "$saved" != "true" ]]; then
        # Download from the server (we know it's reachable — we just installed from it)
        local dl_args=(-fsSL --connect-timeout 10 --max-time 30)
        if [[ "$INSECURE" == "true" ]]; then dl_args+=(-k); fi
        if [[ -n "$CURL_CA_BUNDLE" ]]; then dl_args+=(--cacert "$CURL_CA_BUNDLE"); fi
        curl "${dl_args[@]}" -o "${state_dir}/install.sh" "${PULSE_URL}/install.sh" 2>/dev/null || true
    fi
    if [[ -f "${state_dir}/install.sh" ]]; then
        chmod +x "${state_dir}/install.sh"
        SAVED_INSTALL_SCRIPT="${state_dir}/install.sh"
    else
        SAVED_INSTALL_SCRIPT=""
    fi
}

# --- Parse Arguments ---
while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h) show_help; exit 0 ;;
        --url) PULSE_URL="$2"; shift 2 ;;
        --token) PULSE_TOKEN="$2"; shift 2 ;;
        --interval) INTERVAL="$2"; INTERVAL_EXPLICIT="true"; shift 2 ;;
        --enable-host) ENABLE_HOST="true"; HOST_EXPLICIT="true"; shift ;;
        --enable-host=true) ENABLE_HOST="true"; HOST_EXPLICIT="true"; shift ;;
        --enable-host=false) ENABLE_HOST="false"; HOST_EXPLICIT="true"; shift ;;
        --disable-host) ENABLE_HOST="false"; HOST_EXPLICIT="true"; shift ;;
        --enable-docker) ENABLE_DOCKER="true"; DOCKER_EXPLICIT="true"; shift ;;
        --enable-docker=true) ENABLE_DOCKER="true"; DOCKER_EXPLICIT="true"; shift ;;
        --enable-docker=false) ENABLE_DOCKER="false"; DOCKER_EXPLICIT="true"; shift ;;
        --disable-docker) ENABLE_DOCKER="false"; DOCKER_EXPLICIT="true"; shift ;;
        --enable-kubernetes) ENABLE_KUBERNETES="true"; KUBERNETES_EXPLICIT="true"; shift ;;
        --enable-kubernetes=true) ENABLE_KUBERNETES="true"; KUBERNETES_EXPLICIT="true"; shift ;;
        --enable-kubernetes=false) ENABLE_KUBERNETES="false"; KUBERNETES_EXPLICIT="true"; shift ;;
        --disable-kubernetes) ENABLE_KUBERNETES="false"; KUBERNETES_EXPLICIT="true"; shift ;;
        --kubeconfig) KUBECONFIG_PATH="$2"; KUBERNETES_EXPLICIT="true"; ENABLE_KUBERNETES="true"; shift 2 ;;
        --enable-proxmox) ENABLE_PROXMOX="true"; PROXMOX_EXPLICIT="true"; shift ;;
        --enable-proxmox=true) ENABLE_PROXMOX="true"; PROXMOX_EXPLICIT="true"; shift ;;
        --enable-proxmox=false) ENABLE_PROXMOX="false"; PROXMOX_EXPLICIT="true"; shift ;;
        --disable-proxmox) ENABLE_PROXMOX="false"; PROXMOX_EXPLICIT="true"; shift ;;
        --proxmox-type) PROXMOX_TYPE="$2"; shift 2 ;;
        --insecure) INSECURE="true"; INSECURE_EXPLICIT="true"; shift ;;
        --cacert) CURL_CA_BUNDLE="$2"; shift 2 ;;
        --server-fingerprint) SERVER_FINGERPRINT="$2"; shift 2 ;;
        --observers-file) OBSERVERS_FILE="$2"; shift 2 ;;
        --enable-commands) ENABLE_COMMANDS="true"; shift ;;
        --command-authority) COMMAND_AUTHORITY="$2"; COMMAND_AUTHORITY_SOURCE="explicit"; shift 2 ;;
        --least-privilege) LEAST_PRIVILEGE="true"; shift ;;
        --enable-privileged-helper) PRIVILEGED_HELPER_ENABLED="true"; PRIVILEGED_HELPER_EXPLICIT="true"; shift ;;
        --disable-privileged-helper) PRIVILEGED_HELPER_ENABLED="false"; PRIVILEGED_HELPER_EXPLICIT="true"; shift ;;
        --enable-action-runner) ACTION_RUNNER_ENABLED="true"; ACTION_RUNNER_EXPLICIT="true"; shift ;;
        --disable-action-runner) ACTION_RUNNER_ENABLED="false"; ACTION_RUNNER_EXPLICIT="true"; shift ;;
        --uninstall-action-runner) UNINSTALL_ACTION_RUNNER="true"; shift ;;
        --action-token-file) ACTION_TOKEN_FILE_PATH="$2"; shift 2 ;;
        --grant-smart) GRANT_SMART="true"; shift ;;
        --grant-pct) GRANT_PCT="true"; shift ;;
        --health-addr) HEALTH_ADDR="$2"; HEALTH_ADDR_SET="true"; shift 2 ;;
        --safe-profile-inspect) SAFE_PROFILE_ACTION="inspect"; shift ;;
        --safe-profile-apply) SAFE_PROFILE_ACTION="apply"; shift ;;
        --safe-profile-rollback) SAFE_PROFILE_ACTION="rollback"; shift ;;
        --enroll) ENROLL="true"; shift ;;
        --update) UPDATE_ONLY="true"; shift ;;
        --retarget) RETARGET_ONLY="true"; UPDATE_ONLY="true"; shift ;;
        --uninstall) UNINSTALL="true"; shift ;;
        --agent-id) AGENT_ID="$2"; shift 2 ;;
        --hostname) HOSTNAME_OVERRIDE="$2"; shift 2 ;;
        --report-ip) REPORT_IP="$2"; shift 2 ;;
        --state-dir) STATE_DIR="$2"; STATE_DIR_SOURCE="explicit"; shift 2 ;;
        --kube-include-all-pods) KUBE_INCLUDE_ALL_PODS="true"; shift ;;
        --kube-include-all-deployments) KUBE_INCLUDE_ALL_DEPLOYMENTS="true"; shift ;;
        --disk-exclude) DISK_EXCLUDES+=("$2"); shift 2 ;;
        --disk-include) DISK_INCLUDES+=("$2"); shift 2 ;;
        --non-interactive) NON_INTERACTIVE="true"; shift ;;
        --token-file) TOKEN_FILE_PATH="$2"; shift 2 ;;
        --pulse-url) PULSE_URL="$2"; shift 2 ;;
        --output) OUTPUT_FORMAT="$2"; shift 2 ;;
        --preflight-only) PREFLIGHT_ONLY="true"; shift ;;
        *) fail "Unknown argument: $1" ;;
    esac
done

if [[ "$RETARGET_ONLY" == "true" && -z "$PULSE_URL" ]]; then
    fail "--retarget requires the new Pulse endpoint in --url" "$EXIT_MISSING_ARGS"
fi
if [[ "$RETARGET_ONLY" == "true" && "$UNINSTALL" == "true" ]]; then
    fail "--retarget cannot be combined with --uninstall" "$EXIT_MISSING_ARGS"
fi

case "$SAFE_PROFILE_ACTION" in
    "") ;;
    inspect)
        if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" || "$UNINSTALL_ACTION_RUNNER" == "true" ]]; then
            fail "--safe-profile-inspect is a standalone read-only action" "$EXIT_MISSING_ARGS"
        fi
        ;;
    apply)
        if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" || "$UNINSTALL_ACTION_RUNNER" == "true" ||
              "$ACTION_RUNNER_EXPLICIT" == "true" ]]; then
            fail "--safe-profile-apply is an explicit collector/helper transaction and cannot be combined with another lifecycle action" "$EXIT_MISSING_ARGS"
        fi
        UPDATE_ONLY="true"
        LEAST_PRIVILEGE="true"
        PRIVILEGED_HELPER_ENABLED="true"
        PRIVILEGED_HELPER_EXPLICIT="true"
        ENABLE_COMMANDS="false"
        COMMAND_AUTHORITY="monitoring-only"
        COMMAND_AUTHORITY_SOURCE="explicit"
        GRANT_SMART="false"
        GRANT_PCT="false"
        ;;
    rollback)
        if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" || "$UNINSTALL_ACTION_RUNNER" == "true" ||
              "$ACTION_RUNNER_EXPLICIT" == "true" ]]; then
            fail "--safe-profile-rollback is a standalone collector/helper lifecycle action" "$EXIT_MISSING_ARGS"
        fi
        ;;
    *) fail "Internal safe-profile action is invalid" "$EXIT_GENERAL" ;;
esac

discover_state_dir_from_saved_installer "$0" || true

if [[ -z "$STATE_DIR" || "$STATE_DIR" != /* || "$STATE_DIR" == "/" ||
      "$STATE_DIR" == *$'\r'* || "$STATE_DIR" == *$'\n'* ]]; then
    fail "--state-dir must be an absolute, non-root path." "$EXIT_MISSING_ARGS"
fi

if [[ -n "$OBSERVERS_FILE" ]]; then
    if [[ "$OBSERVERS_FILE" != /* ]]; then
        fail "Observer config path must be absolute: ${OBSERVERS_FILE}" "$EXIT_MISSING_ARGS"
    fi
    if [[ ! -f "$OBSERVERS_FILE" || -L "$OBSERVERS_FILE" ]]; then
        fail "Observer config must be a regular non-symlink file: ${OBSERVERS_FILE}" "$EXIT_MISSING_ARGS"
    fi
fi

# Read token from file if --token-file was provided
if [[ -n "$TOKEN_FILE_PATH" ]]; then
    if [[ ! -f "$TOKEN_FILE_PATH" ]]; then
        fail "Token file not found: ${TOKEN_FILE_PATH}" "$EXIT_MISSING_ARGS"
    fi
    PULSE_TOKEN=$(cat "$TOKEN_FILE_PATH")
    if [[ -z "$PULSE_TOKEN" ]]; then
        fail "Token file is empty: ${TOKEN_FILE_PATH}" "$EXIT_MISSING_ARGS"
    fi
    # Clean up token file after reading in non-interactive mode (deploy bootstrap tokens are one-time use)
    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        rm -f "$TOKEN_FILE_PATH" 2>/dev/null || true
    fi
fi

if [[ -n "$ACTION_TOKEN_FILE_PATH" ]]; then
    if [[ ! -f "$ACTION_TOKEN_FILE_PATH" || -L "$ACTION_TOKEN_FILE_PATH" ]]; then
        fail "Action token file must be a regular non-symlink file: ${ACTION_TOKEN_FILE_PATH}" "$EXIT_MISSING_ARGS"
    fi
    ACTION_TOKEN_MODE=$(stat -c '%a' "$ACTION_TOKEN_FILE_PATH" 2>/dev/null || true)
    if [[ ! "$ACTION_TOKEN_MODE" =~ ^[0-7]{3,4}$ ]] || (( (8#$ACTION_TOKEN_MODE & 8#077) != 0 )); then
        fail "Action token file must be inaccessible to group and other users (mode 0600 or stricter)" "$EXIT_MISSING_ARGS"
    fi
    ACTION_TOKEN_SIZE=$(wc -c < "$ACTION_TOKEN_FILE_PATH" 2>/dev/null | tr -d ' ' || true)
    if [[ ! "$ACTION_TOKEN_SIZE" =~ ^[0-9]+$ || "$ACTION_TOKEN_SIZE" -lt 1 || "$ACTION_TOKEN_SIZE" -gt 4096 ]]; then
        fail "Action token file must contain between 1 and 4096 bytes" "$EXIT_MISSING_ARGS"
    fi
    ACTION_TOKEN=$(cat "$ACTION_TOKEN_FILE_PATH")
    if [[ -z "$ACTION_TOKEN" || "$ACTION_TOKEN" == *$'\r'* || "$ACTION_TOKEN" == *$'\n'* ]]; then
        fail "Action token file must contain one non-empty token value" "$EXIT_MISSING_ARGS"
    fi
    ACTION_TOKEN_FILE_PATH=""
fi

if [[ -n "$PROXMOX_TYPE" && "$PROXMOX_TYPE" != "pve" && "$PROXMOX_TYPE" != "pbs" ]]; then
    fail "Invalid --proxmox-type value: ${PROXMOX_TYPE} (expected 'pve' or 'pbs')"
fi

if [[ "$GRANT_SMART" == "true" || "$GRANT_PCT" == "true" ]] && [[ "$LEAST_PRIVILEGE" != "true" ]]; then
    fail "--grant-smart and --grant-pct only apply with --least-privilege (a root agent needs no grants)" "$EXIT_MISSING_ARGS"
fi
if [[ "$LEAST_PRIVILEGE" == "true" && "$ENABLE_COMMANDS" == "true" ]]; then
    fail "--least-privilege and --enable-commands are mutually exclusive: governed command execution requires the root profile" "$EXIT_MISSING_ARGS"
fi
if [[ "$PRIVILEGED_HELPER_EXPLICIT" != "true" ]] &&
   { [[ -f "$PRIVILEGED_HELPER_SOCKET_UNIT" ]] || [[ -f "$PRIVILEGED_HELPER_SERVICE_UNIT" ]]; }; then
    PRIVILEGED_HELPER_ENABLED="true"
    LEAST_PRIVILEGE="true"
    log_info "Preserving existing typed privileged-helper profile"
fi
if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" && "$LEAST_PRIVILEGE" != "true" ]]; then
    fail "--enable-privileged-helper requires --least-privilege" "$EXIT_MISSING_ARGS"
fi
if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" && ( "$GRANT_SMART" == "true" || "$GRANT_PCT" == "true" ) ]]; then
    fail "--enable-privileged-helper cannot be combined with legacy --grant-smart/--grant-pct sudo paths" "$EXIT_MISSING_ARGS"
fi
if [[ "$ACTION_RUNNER_EXPLICIT" != "true" && "$UNINSTALL" != "true" &&
      "$SAFE_PROFILE_ACTION" != "apply" &&
      "$UNINSTALL_ACTION_RUNNER" != "true" && -f "$ACTION_RUNNER_SERVICE_UNIT" ]]; then
    ACTION_RUNNER_ENABLED="true"
    log_info "Preserving existing separately enabled action-runner profile"
fi
if [[ "$ACTION_RUNNER_ENABLED" == "true" &&
      ( "$LEAST_PRIVILEGE" != "true" || "$PRIVILEGED_HELPER_ENABLED" != "true" ) ]]; then
    fail "--enable-action-runner requires the safe --least-privilege --enable-privileged-helper collector profile" "$EXIT_MISSING_ARGS"
fi
if [[ "$ACTION_RUNNER_ENABLED" == "true" && "$ENABLE_COMMANDS" == "true" ]]; then
    fail "--enable-action-runner cannot be combined with legacy collector --enable-commands" "$EXIT_MISSING_ARGS"
fi
if [[ -n "$ACTION_TOKEN" && "$ACTION_RUNNER_ENABLED" != "true" ]]; then
    fail "--action-token-file requires --enable-action-runner (or an existing preserved runner profile)" "$EXIT_MISSING_ARGS"
fi
if [[ "$ACTION_RUNNER_ENABLED" == "true" && "$PREFLIGHT_ONLY" != "true" &&
      -z "$ACTION_TOKEN" && ! -s "$ACTION_RUNNER_TOKEN_FILE" ]]; then
    fail "--enable-action-runner requires a separate --action-token-file on first install" "$EXIT_MISSING_ARGS"
fi
if [[ -n "$ACTION_TOKEN" && -n "$PULSE_TOKEN" && "$ACTION_TOKEN" == "$PULSE_TOKEN" ]]; then
    fail "The action runner must use a separate credential from the collector token" "$EXIT_MISSING_ARGS"
fi

# --- Check Root ---
if [[ $EUID -ne 0 && "$PREFLIGHT_ONLY" != "true" ]]; then
    if [[ "$SAFE_PROFILE_ACTION" != "inspect" ]]; then
        echo "This script must be run as root. Please use sudo."
        exit 1
    fi
fi

if [[ "$SAFE_PROFILE_ACTION" == "inspect" ]]; then
    safe_profile_inspect
    exit $?
fi
if [[ "$SAFE_PROFILE_ACTION" == "rollback" ]]; then
    safe_profile_platform_supported ||
        fail "Safe-profile rollback is supported only on standard Linux systemd hosts; no broader-privilege fallback was applied" "$EXIT_MISSING_ARGS"
    safe_profile_rollback_last
    exit 0
fi

if [[ "$UNINSTALL_ACTION_RUNNER" == "true" ]]; then
    if [[ "$UNINSTALL" == "true" || "$UPDATE_ONLY" == "true" || "$ACTION_RUNNER_ENABLED" == "true" ]]; then
        fail "--uninstall-action-runner is a standalone runner-only lifecycle action" "$EXIT_MISSING_ARGS"
    fi
    teardown_action_runner_service
    log_info "Pulse action runner removed. Collector monitoring was left installed and running."
    exit 0
fi

# --- URL Normalization ---
# Strip trailing slashes from PULSE_URL to prevent double-slash URLs
# (e.g., http://host:7655//download/... which would match frontend routes)
if [[ -n "$PULSE_URL" ]]; then
    PULSE_URL="${PULSE_URL%/}"
fi

# --- Installed Lifecycle State Recovery ---
# An explicit state directory is authoritative. Without one, inspect the active
# process/service first so a custom installation wins over stale default-path
# artifacts, then merge its canonical connection.env and agent-id state.
if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" ]]; then
    if [[ "$STATE_DIR_SOURCE" != "explicit" ]]; then
        recover_connection_state_from_existing_agent || true
    fi

    local lifecycle_conn_env=""
    lifecycle_conn_env=$(find_connection_state_file || true)
    if [[ -n "$lifecycle_conn_env" ]]; then
        log_info "Recovering connection details from ${lifecycle_conn_env}..."
        recover_connection_state "$lifecycle_conn_env"
    fi

    if update_connection_state_incomplete; then
        recover_connection_state_from_existing_agent || true
    fi

    if [[ -z "$AGENT_ID" ]]; then
        AGENT_ID=$(recover_agent_id_from_state_file || true)
        if [[ -n "$AGENT_ID" ]]; then
            log_info "Recovered agent ID from persisted agent-id state."
        fi
    fi

    if [[ "$UPDATE_ONLY" == "true" && ( -z "$PULSE_URL" || -z "$PULSE_TOKEN" ) ]]; then
        fail "No existing Pulse Agent connection state found. Use the install command instead." "$EXIT_MISSING_ARGS"
    fi
fi

if [[ -n "$ACTION_TOKEN" && -n "$PULSE_TOKEN" && "$ACTION_TOKEN" == "$PULSE_TOKEN" ]]; then
    fail "The action runner must use a separate credential from the collector token" "$EXIT_MISSING_ARGS"
fi

if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    # Recovered legacy service arguments may include command authority. The
    # explicit migration always lowers the collector ceiling and never carries
    # command execution or sudo grants into the safe profile.
    ENABLE_COMMANDS="false"
    COMMAND_AUTHORITY="monitoring-only"
    COMMAND_AUTHORITY_SOURCE="explicit"
    GRANT_SMART="false"
    GRANT_PCT="false"
fi

resolve_command_authority_profile

if [[ -z "$STATE_DIR" || "$STATE_DIR" != /* || "$STATE_DIR" == "/" ||
      "$STATE_DIR" == *$'\r'* || "$STATE_DIR" == *$'\n'* ]]; then
    fail "Recovered Pulse Agent state directory is invalid." "$EXIT_MISSING_ARGS"
fi

if [[ "$STATE_DIR_SOURCE" == "explicit" || "$STATE_DIR_SOURCE" == "recovered" ]]; then
    TRUENAS_STATE_DIR="$STATE_DIR"
    TRUENAS_LOG_DIR="$TRUENAS_STATE_DIR/logs"
    TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
    TRUENAS_ENV_FILE="$TRUENAS_STATE_DIR/pulse-agent.env"
fi

if [[ -n "$PULSE_URL" ]]; then
    PULSE_URL="${PULSE_URL%/}"
fi

# --- CA Certificate Validation ---
# --cacert must point to a PEM file (matches curl --cacert behaviour).
# The same path is passed to the agent process via SSL_CERT_FILE so that
# Go's crypto/x509 trusts the custom CA at runtime.
SSL_CERT_ENV_NAME=""
SSL_CERT_ENV_VALUE=""
if [[ -n "$CURL_CA_BUNDLE" ]]; then
    if [[ -f "$CURL_CA_BUNDLE" ]]; then
        SSL_CERT_ENV_NAME="SSL_CERT_FILE"
        SSL_CERT_ENV_VALUE="$CURL_CA_BUNDLE"
        log_info "CA certificate: ${CURL_CA_BUNDLE} (will set SSL_CERT_FILE for agent)"
    elif [[ -d "$CURL_CA_BUNDLE" ]]; then
        fail "--cacert requires a PEM file, not a directory. Try: --cacert ${CURL_CA_BUNDLE}/<cert-name>.pem"
    else
        fail "--cacert path does not exist: ${CURL_CA_BUNDLE}"
    fi
fi

service_env_has_key() {
    local env_key="$1"
    case "$APPLIED_SERVICE_ENV_KEYS" in
        *"|${env_key}|"*) return 0 ;;
        *) return 1 ;;
    esac
}

shell_export_value() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//\$/\\$}"
    value="${value//\`/\\\`}"
    printf '"%s"' "$value"
}

append_service_env() {
    local env_key="$1"
    local env_val="$2"
    local shell_value=""
    local plist_key=""
    local plist_value=""

    if [[ -z "$env_key" ]] || service_env_has_key "$env_key"; then
        return
    fi

    shell_value="$(shell_export_value "$env_val")"
    SYSTEMD_ENV_LINES+=$'\n'"Environment=\"${env_key}=${env_val}\""
    SHELL_EXPORT_LINES+=$'\n'"export ${env_key}=${shell_value}"
    UPSTART_ENV_LINES+=$'\n'"env ${env_key}=${env_val}"
    if [[ -n "$SED_EXPORT_LINES" ]]; then
        SED_EXPORT_LINES+="; "
    fi
    SED_EXPORT_LINES+="export ${env_key}=${shell_value}"

    plist_key="$(xml_escape "$env_key")"
    plist_value="$(xml_escape "$env_val")"
    PLIST_ENV_ENTRIES+="
        <key>${plist_key}</key>
        <string>${plist_value}</string>"
    APPLIED_SERVICE_ENV_KEYS+="${env_key}|"
}

finalize_plist_env_block() {
    PLIST_ENV_BLOCK=""
    if [[ -n "$PLIST_ENV_ENTRIES" ]]; then
        PLIST_ENV_BLOCK="
    <key>EnvironmentVariables</key>
    <dict>${PLIST_ENV_ENTRIES}
    </dict>"
    fi
}

if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
    append_service_env "$SSL_CERT_ENV_NAME" "$SSL_CERT_ENV_VALUE"
fi

# --- Platform Auto-Detection ---
# Only auto-detect if flags weren't explicitly set
log_info "Detecting available platforms..."

if [[ "$DOCKER_EXPLICIT" != "true" ]]; then
    if detect_docker; then
        log_info "Docker/Podman detected - enabling container monitoring"
        log_info "  (use --disable-docker to skip)"
        ENABLE_DOCKER="true"
    else
        ENABLE_DOCKER="false"
    fi
fi

if [[ "$KUBERNETES_EXPLICIT" != "true" ]]; then
    if detect_kubernetes; then
        log_info "Kubernetes detected - enabling cluster monitoring"
        log_info "  (use --disable-kubernetes to skip)"
        ENABLE_KUBERNETES="true"
    else
        ENABLE_KUBERNETES="false"
    fi
fi

if [[ "$PROXMOX_EXPLICIT" != "true" ]]; then
    if detect_proxmox; then
        log_info "Proxmox detected - enabling Proxmox integration"
        log_info "  (use --disable-proxmox to skip)"
        ENABLE_PROXMOX="true"
    else
        ENABLE_PROXMOX="false"
    fi
fi

# Summary of what will be monitored
log_info "Monitoring configuration:"
log_info "  Agent metrics: $ENABLE_HOST"
log_info "  Docker/Podman: $ENABLE_DOCKER"
log_info "  Kubernetes: $ENABLE_KUBERNETES"
log_info "  Proxmox: $ENABLE_PROXMOX"
log_info "  Pulse command execution: $ENABLE_COMMANDS"
if [[ "$ENABLE_PROXMOX" == "true" ]]; then
    if [[ -n "$PROXMOX_TYPE" ]]; then
        log_info "  Proxmox type: $PROXMOX_TYPE"
    else
        log_info "  Proxmox type: auto-detect all installed services"
    fi
fi
if [[ "$ENABLE_COMMANDS" == "true" ]]; then
    log_info "    Accepts Pulse-scoped command requests on this agent."
    log_info "    On Proxmox nodes this is required for opted-in LXC Docker inventory via pct exec."
    log_info "    The Pulse server must also be started with PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true."
else
    log_info "    Command execution is off; enable only when Patrol actions or Proxmox LXC Docker inventory are needed."
fi

# discover_rootless_container_runtime returns non-zero when a rootful Docker
# daemon is live, so an explicit --enable-docker with working system Docker
# never gets the rootless podman socket pinned into the service environment.
if [[ "$ENABLE_DOCKER" == "true" ]] && discover_rootless_container_runtime; then
    if [[ "$ROOTLESS_RUNTIME_KIND" == "docker" ]]; then
        if ! service_env_has_key "DOCKER_HOST"; then
            append_service_env "DOCKER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"
            if ! service_env_has_key "XDG_RUNTIME_DIR"; then
                append_service_env "XDG_RUNTIME_DIR" "$ROOTLESS_RUNTIME_XDG_DIR"
            fi
            log_info "Using rootless Docker socket for agent service: ${ROOTLESS_RUNTIME_SOCKET_PATH}"
        fi
    elif [[ "$ROOTLESS_RUNTIME_KIND" == "podman" ]]; then
        if ! service_env_has_key "CONTAINER_HOST" && ! service_env_has_key "PODMAN_HOST"; then
            append_service_env "PULSE_DOCKER_RUNTIME" "podman"
            append_service_env "CONTAINER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"
            append_service_env "PODMAN_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"
            if ! service_env_has_key "XDG_RUNTIME_DIR"; then
                append_service_env "XDG_RUNTIME_DIR" "$ROOTLESS_RUNTIME_XDG_DIR"
            fi
            log_info "Using rootless Podman socket for agent service: ${ROOTLESS_RUNTIME_SOCKET_PATH}"
        fi
    fi
fi

safe_profile_apply_docker_degradation

finalize_plist_env_block

# --- Uninstall Logic ---
if [[ "$UNINSTALL" == "true" ]]; then
    log_info "Uninstalling ${AGENT_NAME} and cleaning up legacy agents..."
    local qnap_state_dir=""

    # Try to notify the Pulse server about uninstallation if we have connection details
    # This ensures the agent record is removed and any linked PVE nodes are updated immediately.
    if [[ -n "$PULSE_URL" ]]; then
        # Try to recover agent ID if not provided.
        # Priority: agent-id file (canonical) > hostname API lookup (fallback)
        if [[ -z "$AGENT_ID" ]]; then
            local aid_path=""
            local aid_paths=("${STATE_DIR%/}/agent-id")
            if [[ "$STATE_DIR_SOURCE" == "default" ]]; then
                aid_paths+=("$DEFAULT_STATE_DIR/agent-id" /boot/config/plugins/pulse-agent/agent-id "$TRUENAS_STATE_DIR/agent-id")
            fi
            qnap_state_dir=$(find_qnap_state_dir || true)
            if [[ -n "$qnap_state_dir" ]]; then
                aid_paths+=("$qnap_state_dir/agent-id")
            fi

            # Primary: canonical agent-id file
            for aid_path in "${aid_paths[@]}"; do
                if [[ -f "$aid_path" ]]; then
                    AGENT_ID=$(cat "$aid_path")
                    log_info "Recovered agent ID from ${aid_path}"
                    break
                fi
            done
        fi
        if [[ -z "$AGENT_ID" ]]; then
            # API fallback: prefer explicit hostname continuity from the caller,
            # otherwise fall back to the local hostname.
            LOOKUP_HOSTNAME="$HOSTNAME_OVERRIDE"
            if [[ -z "$LOOKUP_HOSTNAME" ]]; then
                LOOKUP_HOSTNAME=$(hostname 2>/dev/null || true)
            fi
            if [[ -n "$LOOKUP_HOSTNAME" ]]; then
                LOOKUP_ARGS=(-fsSL --connect-timeout 5)
                if [[ "$INSECURE" == "true" ]]; then LOOKUP_ARGS+=(-k); fi
                if [[ -n "$CURL_CA_BUNDLE" ]]; then LOOKUP_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi
                LOOKUP_HOSTNAME_ESCAPED=$(url_encode "$LOOKUP_HOSTNAME")
                LOOKUP_RESP=$(curl_with_pulse_token "${LOOKUP_ARGS[@]}" "${PULSE_URL}/api/agents/agent/lookup?hostname=${LOOKUP_HOSTNAME_ESCAPED}" 2>/dev/null || true)
                if [[ -n "$LOOKUP_RESP" ]]; then
                    # Extract .agent.id from JSON (portable, no jq dependency)
                    AGENT_ID=$(echo "$LOOKUP_RESP" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"//; s/"$//' || true)
                    if [[ -n "$AGENT_ID" ]]; then
                        log_info "Recovered agent ID via server lookup: ${AGENT_ID}"
                    fi
                fi
            fi
        fi

        if [[ -n "$AGENT_ID" ]]; then
            log_info "Notifying Pulse server to unregister agent ID: ${AGENT_ID}..."
            CURL_ARGS=(-fsSL --connect-timeout 5 -X POST -H "Content-Type: application/json")
            if [[ "$INSECURE" == "true" ]]; then CURL_ARGS+=(-k); fi
            if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

            # Send unregistration request (ignore errors as we are uninstalling anyway)
            curl_with_pulse_token "${CURL_ARGS[@]}" -d "{\"agentId\": \"${AGENT_ID}\"}" "${PULSE_URL}/api/agents/agent/uninstall" >/dev/null 2>&1 || true
        fi
    fi

    # Kill wrapper scripts first: they are watchdogs, so stopping the agent
    # while its wrapper still loops only races the respawn. No leading path
    # separator here, unlike the install paths, so a wrapper invoked by a
    # relative path or left at an older location is still caught. The dot is
    # escaped and the far end bounded so an editor session or a .bak copy of
    # the wrapper is not swept up with it.
    pkill -f "start-pulse-agent\.sh([[:space:]]|$)" 2>/dev/null || true
    # Then the agent itself.
    # Use -x (exact process name match) to avoid killing THIS uninstall script,
    # whose command line path contains "pulse-agent" (e.g. /boot/config/plugins/pulse-agent/install.sh).
    pkill -x "pulse-agent" 2>/dev/null || true
    sleep 1

    # Systemd - unified agent
    if command -v systemctl >/dev/null 2>&1; then
        teardown_action_runner_service
        teardown_privileged_helper_service
        teardown_systemd_agent_service
    fi

    # Remove legacy binaries

    # Remove agent state directory (contains agent ID, proxmox registration state, etc.)
    remove_agent_state_dir "$STATE_DIR"

    # Remove least-privilege helper artifacts. The pulse-agent system user is
    # deliberately left behind: deleting accounts can orphan files elsewhere,
    # and an inert nologin system user is harmless.
    rm -f "$PRIVILEGE_SUDOERS_FILE"
    rm -rf "$PRIVILEGE_HELPER_DIR"

    # Remove log files
    rm -f /var/log/pulse-agent.log

    # Launchd (macOS)
    if [[ "$(uname -s)" == "Darwin" ]]; then
        # Unified agent
        PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
        launchctl unload "$PLIST" 2>/dev/null || true
        rm -f "$PLIST"
        
    fi

    # Synology DSM (handles both DSM 7+ systemd and DSM 6.x upstart)
    if [[ -d /usr/syno ]]; then
        # DSM 7+ uses systemd
        if [[ -f "/etc/systemd/system/${AGENT_NAME}.service" ]]; then
            teardown_systemd_agent_service
        fi
        # DSM 6.x uses upstart
        if [[ -f "/etc/init/${AGENT_NAME}.conf" ]]; then
            initctl stop "${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/init/${AGENT_NAME}.conf"
        fi
    fi

    # Unraid
    if [[ -f /etc/unraid-version ]] || [[ -d /boot/config/plugins/pulse-agent ]]; then
        log_info "Removing Unraid installation..."
        # Stop the wrapper watchdogs before the agents they supervise, and keep
        # the match bounded so a .bak copy or an editor session on the wrapper
        # is not swept up.
        pkill -f "start-pulse-agent\.sh([[:space:]]|$)" 2>/dev/null || true
        pkill -x "pulse-agent" 2>/dev/null || true
        sleep 1
        
        # Remove from /boot/config/go - all pulse-related entries
        GO_SCRIPT="/boot/config/go"
        if [[ -f "$GO_SCRIPT" ]]; then
            # Remove unified agent entries (line-by-line, not range-based,
            # to avoid consuming adjacent non-pulse entries when no trailing
            # blank line separates them).
            sed -i '/^# Pulse Agent$/d' "$GO_SCRIPT" 2>/dev/null || true
            sed -i '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || true
        fi
        
        # Remove installation directories
        rm -rf /boot/config/plugins/pulse-agent
        rm -rf /boot/config/pulse  # Legacy pulse directory
        
        # Remove binaries from RAM disk
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        
        # Remove log directory
        rm -rf /var/log/pulse
    fi

    # QNAP QTS/QuTS hero
    qnap_state_dir=$(find_qnap_state_dir || true)
    if [[ -n "$qnap_state_dir" ]] || [[ -f /sbin/getcfg ]] || [[ -f /etc/config/qpkg.conf ]]; then
        log_info "Removing QNAP installation..."
        if [[ -x /etc/init.d/init_disk.sh ]]; then
            if /etc/init.d/init_disk.sh mount_flash_config 2>/dev/null && [[ -d /tmp/nasconfig_tmp ]]; then
                AUTORUN_PATH="/tmp/nasconfig_tmp/autorun.sh"
                if [[ -f "$AUTORUN_PATH" ]]; then
                    remove_qnap_autorun_block "$AUTORUN_PATH"
                fi
                /etc/init.d/init_disk.sh umount_flash_config 2>/dev/null || true
            else
                /etc/init.d/init_disk.sh umount_flash_config 2>/dev/null || true
                log_warn "Could not mount QNAP flash config to remove autorun.sh entry."
            fi
        fi
        if [[ -n "$qnap_state_dir" ]]; then
            rm -rf "$qnap_state_dir"
        fi
    fi

    # TrueNAS SCALE/CORE
    if [[ -d "$TRUENAS_STATE_DIR" ]] || [[ -f /etc/truenas-version ]] || [[ -f /etc/version ]]; then
        if [[ "$(uname -s)" == "Linux" ]]; then
            log_info "Removing TrueNAS SCALE installation..."
            teardown_systemd_agent_service
        elif [[ "$(uname -s)" == "FreeBSD" ]]; then
            log_info "Removing TrueNAS CORE installation..."
            teardown_freebsd_agent_service "/usr/local/etc/rc.d/${AGENT_NAME}"
        fi
        # Remove Init/Shutdown task
        if command -v midclt >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
            TASK_ID=$(midclt call initshutdownscript.query '[["script","=","'"$TRUENAS_BOOTSTRAP_SCRIPT"'"]]' 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null || echo "")
            if [[ -n "$TASK_ID" ]]; then
                midclt call initshutdownscript.delete "$TASK_ID" >/dev/null 2>&1 || log_warn "Failed to remove Init/Shutdown task (id $TASK_ID)"
            fi
        fi
        # Remove state directory
        rm -rf "$TRUENAS_STATE_DIR"
    fi

    # OpenRC (Alpine, Gentoo, Artix, etc.)
    if command -v rc-service >/dev/null 2>&1; then
        teardown_openrc_agent_service
    fi

    # Vanilla FreeBSD, OPNsense, and pfSense all use the rc.d service rendered
    # by this installer. This must run even when no TrueNAS marker is present.
    if [[ "$(uname -s)" == "FreeBSD" ]]; then
        log_info "Removing FreeBSD rc.d installation..."
        teardown_freebsd_agent_service "/usr/local/etc/rc.d/${AGENT_NAME}"
    fi

    # SysV init (legacy systems like Asustor, older Debian/RHEL, etc.)
    if [[ -f "/etc/init.d/${AGENT_NAME}" ]]; then
        teardown_sysv_agent_service "/etc/init.d/${AGENT_NAME}"
    fi

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Uninstallation complete."
    exit 0
fi

# --- Validation ---
if [[ -z "$PULSE_URL" ]]; then
    fail "Missing required argument: --url (or --pulse-url)" "$EXIT_MISSING_ARGS"
fi
# Validate URL format (basic check) - case-insensitive for http:// or https://
# Normalize to lowercase for the check
url_lower=$(echo "$PULSE_URL" | tr '[:upper:]' '[:lower:]')
if [[ ! "$url_lower" =~ ^https?:// ]]; then
    fail "Invalid URL format. Must start with http:// or https://"
fi

auto_enable_insecure_for_plain_http_url
verify_pinned_server_certificate

# Validate token format when present (should be hex string, typically 64 chars)
if [[ -n "$PULSE_TOKEN" && ! "$PULSE_TOKEN" =~ ^[a-fA-F0-9]+$ ]]; then
    fail "Invalid token format. Token should be a hexadecimal string."
fi

# Validate interval format
if [[ ! "$INTERVAL" =~ ^[0-9]+[smh]?$ ]]; then
    fail "Invalid interval format. Use format like '30s', '5m', or '1h'."
fi

# --- TrueNAS SCALE/CORE Detection ---
# TrueNAS SCALE/CORE often have immutable root filesystems; /usr/local/bin may be read-only.
# We store everything in /data which persists across reboots and upgrades.
is_truenas() {
    if [[ -f /etc/truenas-version ]]; then
        return 0
    fi
    if [[ -f /etc/version ]] && grep -qi "truenas" /etc/version 2>/dev/null; then
        return 0
    fi
    if [[ -d /data/ix-applications ]] || [[ -d /etc/ix-apps.d ]] || [[ -d /etc/ix.rc.d ]]; then
        return 0
    fi
    # Fallback: check if hostname contains "truenas" (common default hostname)
    if hostname 2>/dev/null | grep -qi "truenas"; then
        return 0
    fi
    return 1
}

# Check if we can write to /usr/local/bin (catches immutable filesystems like TrueNAS)
is_install_dir_writable() {
    local test_file="${INSTALL_DIR}/.pulse-write-test-$$"
    if touch "$test_file" 2>/dev/null; then
        rm -f "$test_file" 2>/dev/null
        return 0
    fi
    return 1
}

# The least-privilege profile is supported only on standard Linux systemd
# hosts: appliance platforms (TrueNAS, Synology, QNAP, Unraid) and non-systemd
# init systems keep the root profile because their service managers, mounts,
# or vendor tooling assume it. Failing here is deliberate — a flag that
# silently falls back to root would defeat its purpose.
if [[ "$LEAST_PRIVILEGE" == "true" && "$UNINSTALL" != "true" ]]; then
    if [[ "$(uname -s)" != "Linux" ]] || ! command -v systemctl >/dev/null 2>&1 ||
        is_truenas || [[ -d /usr/syno ]] || [[ -f /etc/unraid-version ]] ||
        [[ -d /boot/config/plugins ]] || [[ -x /sbin/getcfg ]]; then
        fail "--least-privilege is supported only on standard Linux systemd hosts. This platform keeps the root profile; see docs/AGENT_SECURITY.md for the per-platform privilege model." "$EXIT_MISSING_ARGS"
    fi
fi

if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" && "$UNINSTALL" != "true" ]]; then
    if [[ "$(uname -s)" != "Linux" ]] || ! command -v systemctl >/dev/null 2>&1 ||
        is_truenas || [[ -d /usr/syno ]] || [[ -f /etc/unraid-version ]] ||
        [[ -d /boot/config/plugins ]] || [[ -x /sbin/getcfg ]]; then
        fail "--enable-privileged-helper is supported only on standard Linux systemd hosts; no broader-privilege fallback was applied" "$EXIT_MISSING_ARGS"
    fi
fi

if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    safe_profile_platform_supported ||
        fail "Safe-profile migration is supported only on standard Linux systemd hosts; no broader-privilege fallback was applied" "$EXIT_MISSING_ARGS"
    resolve_safe_profile_hostname ||
        fail "Safe-profile migration could not resolve a canonical local hostname" "$EXIT_MISSING_ARGS"
fi

# Create the dedicated service account for the least-privilege profile and
# hand it the mutable state directory. The legacy least-privilege profile also
# owns its agent binary so its in-process updater keeps working. The typed
# helper profile instead keeps both executables root-owned and disables direct
# collector activation until the helper owns that operation.
provision_least_privilege_user() {
    if ! id -u "$LEAST_PRIVILEGE_USER" >/dev/null 2>&1; then
        if ! command -v useradd >/dev/null 2>&1; then
            fail "--least-privilege needs useradd to create the ${LEAST_PRIVILEGE_USER} system user" "$EXIT_MISSING_ARGS"
        fi
        local nologin_shell="/usr/sbin/nologin"
        if [[ ! -x "$nologin_shell" ]]; then
            nologin_shell="/sbin/nologin"
        fi
        if [[ ! -x "$nologin_shell" ]]; then
            nologin_shell="/bin/false"
        fi
        useradd --system --user-group --home-dir "$STATE_DIR" --no-create-home \
            --shell "$nologin_shell" "$LEAST_PRIVILEGE_USER" ||
            fail "Failed to create the ${LEAST_PRIVILEGE_USER} system user" "$EXIT_MISSING_ARGS"
        log_info "Created system user ${LEAST_PRIVILEGE_USER}"
    fi

    # Docker/Podman socket reads need group membership, not root. Auto-detect
    # mirrors the module default: only an explicit --disable-docker skips it.
    if [[ "$PRIVILEGED_HELPER_ENABLED" != "true" && "$ENABLE_DOCKER" != "false" ]] && getent group docker >/dev/null 2>&1; then
        usermod -aG docker "$LEAST_PRIVILEGE_USER" 2>/dev/null || true
    fi

    chown -R "${LEAST_PRIVILEGE_USER}:${LEAST_PRIVILEGE_USER}" "$STATE_DIR" 2>/dev/null || true
    if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
        chown root:root "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
        chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"
    else
        chown "${LEAST_PRIVILEGE_USER}:${LEAST_PRIVILEGE_USER}" "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
    fi
}

# Keep the installer token in a root-owned directory outside the service
# account's write authority while leaving runtime state mutable by the
# collector. The enrolled runtime token remains collector-owned because the
# agent rotates it; it is monitoring-only and carries no execution scope.
protect_typed_profile_credentials() {
    local legacy_token="${STATE_DIR}/token"

    if [[ -L "$PRIVILEGED_HELPER_CREDENTIAL_DIR" || ! -d "$PRIVILEGED_HELPER_CREDENTIAL_DIR" ]]; then
        fail "Refusing unsafe typed-profile credential directory: ${PRIVILEGED_HELPER_CREDENTIAL_DIR}" "$EXIT_GENERAL"
    fi
    chown "root:${LEAST_PRIVILEGE_USER}" "$PRIVILEGED_HELPER_CREDENTIAL_DIR" ||
        fail "Failed to protect typed-profile credential directory ownership" "$EXIT_GENERAL"
    chmod 0750 "$PRIVILEGED_HELPER_CREDENTIAL_DIR" ||
        fail "Failed to protect typed-profile credential directory mode" "$EXIT_GENERAL"

    if [[ -L "$RUNTIME_TOKEN_FILE" || ! -f "$RUNTIME_TOKEN_FILE" ]]; then
        fail "Refusing unsafe typed-profile credential path: ${RUNTIME_TOKEN_FILE}" "$EXIT_GENERAL"
    fi
    chown "root:${LEAST_PRIVILEGE_USER}" "$RUNTIME_TOKEN_FILE" ||
        fail "Failed to protect typed-profile credential ownership: ${RUNTIME_TOKEN_FILE}" "$EXIT_GENERAL"
    chmod 0640 "$RUNTIME_TOKEN_FILE" ||
        fail "Failed to protect typed-profile credential mode: ${RUNTIME_TOKEN_FILE}" "$EXIT_GENERAL"

    if [[ "$legacy_token" != "$RUNTIME_TOKEN_FILE" ]]; then
        rm -f "$legacy_token"
    fi
}

# Write one privilege helper: a root-owned wrapper that execs the real binary
# through sudo -n, plus the sudoers rule that makes exactly that invocation
# possible. The agent is pointed at the wrapper via an env override that only
# accepts absolute paths.
write_privilege_helper() {
    local helper_name="$1"
    local real_path="$2"
    local sudoers_spec="$3"
    local helper_path="${PRIVILEGE_HELPER_DIR}/${helper_name}"

    mkdir -p "$PRIVILEGE_HELPER_DIR"
    chmod 755 "$PRIVILEGE_HELPER_DIR"
    cat > "$helper_path" <<HELPER
#!/bin/sh
# Pulse least-privilege helper: runs ${real_path} through the exact-command
# sudoers grant in ${PRIVILEGE_SUDOERS_FILE}. Managed by install.sh.
exec sudo -n ${real_path} "\$@"
HELPER
    chown root:root "$helper_path"
    chmod 755 "$helper_path"

    PRIVILEGE_SUDOERS_CONTENT+="${LEAST_PRIVILEGE_USER} ALL=(root) NOPASSWD: ${sudoers_spec}"$'\n'
}

# Install the requested sudo grants and point the agent at the wrappers.
provision_privilege_helpers() {
    if [[ "$GRANT_SMART" != "true" && "$GRANT_PCT" != "true" ]]; then
        return 0
    fi
    if ! command -v sudo >/dev/null 2>&1; then
        fail "--grant-smart/--grant-pct need sudo installed on this host" "$EXIT_MISSING_ARGS"
    fi

    PRIVILEGE_SUDOERS_CONTENT="# Pulse least-privilege agent grants. Managed by install.sh."$'\n'

    if [[ "$GRANT_SMART" == "true" ]]; then
        local smartctl_path
        smartctl_path="$(command -v smartctl 2>/dev/null || true)"
        if [[ -z "$smartctl_path" ]]; then
            fail "--grant-smart requires smartctl (smartmontools) on this host" "$EXIT_MISSING_ARGS"
        fi
        write_privilege_helper "smartctl" "$smartctl_path" "$smartctl_path"
        append_service_env "PULSE_SMARTCTL_PATH" "${PRIVILEGE_HELPER_DIR}/smartctl"
    fi

    if [[ "$GRANT_PCT" == "true" ]]; then
        local pct_path
        pct_path="$(command -v pct 2>/dev/null || true)"
        if [[ -z "$pct_path" ]]; then
            fail "--grant-pct requires the Proxmox pct tool on this host" "$EXIT_MISSING_ARGS"
        fi
        # Restricted to the two read-only queries the collector issues. This
        # deliberately does NOT cover pct exec, start, stop, or enter.
        write_privilege_helper "pct" "$pct_path" "${pct_path} list, ${pct_path} df *"
        append_service_env "PULSE_PCT_PATH" "${PRIVILEGE_HELPER_DIR}/pct"
    fi

    local sudoers_tmp
    sudoers_tmp="$(mktemp)"
    printf '%s' "$PRIVILEGE_SUDOERS_CONTENT" > "$sudoers_tmp"
    if command -v visudo >/dev/null 2>&1; then
        if ! visudo -cf "$sudoers_tmp" >/dev/null 2>&1; then
            rm -f "$sudoers_tmp"
            fail "Generated sudoers rules failed visudo validation; not installing them" "$EXIT_MISSING_ARGS"
        fi
    fi
    install -o root -g root -m 0440 "$sudoers_tmp" "$PRIVILEGE_SUDOERS_FILE"
    rm -f "$sudoers_tmp"
    log_info "Installed scoped sudoers grants at ${PRIVILEGE_SUDOERS_FILE}"
}

verify_privileged_helper_socket() {
    local helper_gid socket_owner socket_mode

    helper_gid=$(id -g "$LEAST_PRIVILEGE_USER")
    if [[ ! -S "$PRIVILEGED_HELPER_SOCKET_PATH" ]]; then
        fail "Typed privileged helper socket was not created at ${PRIVILEGED_HELPER_SOCKET_PATH}" "$EXIT_GENERAL"
    fi
    socket_owner=$(stat -c '%u:%g' "$PRIVILEGED_HELPER_SOCKET_PATH" 2>/dev/null || true)
    socket_mode=$(stat -c '%a' "$PRIVILEGED_HELPER_SOCKET_PATH" 2>/dev/null || true)
    if [[ "$socket_owner" != "0:${helper_gid}" || "$socket_mode" != "660" ]]; then
        fail "Typed privileged helper socket has unsafe ownership or mode (${socket_owner:-unknown} ${socket_mode:-unknown}); expected root:${LEAST_PRIVILEGE_USER} 0660" "$EXIT_GENERAL"
    fi
}

provision_typed_privileged_helper() {
    # The collector may write only into its quarantine. The root helper may
    # read that tree, but activation and rollback state live in a separate
    # root-only boundary. The helper's protocol fixes the sole activation
    # target at /usr/local/bin/pulse-agent.
    install -d -o "$LEAST_PRIVILEGE_USER" -g "$LEAST_PRIVILEGE_USER" -m 0700 \
        "$PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR"
    install -d -o root -g root -m 0700 \
        "$PRIVILEGED_HELPER_STATE_DIR" "$PRIVILEGED_HELPER_UPDATE_STAGING_DIR"
    if [[ -L "${INSTALL_DIR}/${BINARY_NAME}" || ! -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        fail "Typed privileged-helper updates require a regular installed agent binary" "$EXIT_GENERAL"
    fi
    if [[ "${INSTALL_DIR}/${BINARY_NAME}" != "/usr/local/bin/pulse-agent" ]]; then
        fail "Typed privileged-helper updates require the fixed /usr/local/bin/pulse-agent target" "$EXIT_GENERAL"
    fi
    chown root:root "${INSTALL_DIR}/${BINARY_NAME}"
    chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"

    render_privileged_helper_socket_unit "$PRIVILEGED_HELPER_SOCKET_UNIT"
    render_privileged_helper_service_unit "$PRIVILEGED_HELPER_SERVICE_UNIT" "$PRIVILEGED_HELPER_BINARY_PATH"
    chown root:root "$PRIVILEGED_HELPER_SOCKET_UNIT" "$PRIVILEGED_HELPER_SERVICE_UNIT"
    chmod 0644 "$PRIVILEGED_HELPER_SOCKET_UNIT" "$PRIVILEGED_HELPER_SERVICE_UNIT"
    append_service_env "PULSE_AGENT_HELPER_SOCKET" "$PRIVILEGED_HELPER_SOCKET_PATH"

    systemctl daemon-reload
    if ! safe_profile_verify_helper_effective_target; then
        fail "Refusing typed-helper activation because the effective helper service or socket differs from the installer-owned safe profile" "$EXIT_GENERAL"
    fi
    if ! systemctl enable --now "${PRIVILEGED_HELPER_NAME}.socket"; then
        fail "Failed to enable the typed privileged helper socket" "$EXIT_GENERAL"
    fi
    verify_privileged_helper_socket
    log_info "Typed privileged helper socket active at ${PRIVILEGED_HELPER_SOCKET_PATH} (root:${LEAST_PRIVILEGE_USER} 0660)"
}

if [[ "$(uname -s)" == "Linux" ]] && is_truenas; then
    TRUENAS=true
    INSTALL_DIR="$TRUENAS_STATE_DIR"
    TRUENAS_LOG_FILE="$TRUENAS_LOG_DIR/${AGENT_NAME}.log"
    log_info "TrueNAS SCALE detected (immutable root). Using $TRUENAS_STATE_DIR for installation."
elif [[ "$(uname -s)" == "Linux" ]] && [[ -d /data ]] && ! is_install_dir_writable; then
    TRUENAS=true
    INSTALL_DIR="$TRUENAS_STATE_DIR"
    TRUENAS_LOG_FILE="$TRUENAS_LOG_DIR/${AGENT_NAME}.log"
    log_info "Immutable filesystem detected (read-only /usr/local/bin). Using $TRUENAS_STATE_DIR for installation."
elif [[ "$(uname -s)" == "FreeBSD" ]] && is_truenas; then
    TRUENAS=true
    INSTALL_DIR="$TRUENAS_STATE_DIR"
    log_info "TrueNAS CORE detected (immutable root). Using $TRUENAS_STATE_DIR for installation."
elif [[ "$(uname -s)" == "FreeBSD" ]] && [[ -d /data ]] && ! is_install_dir_writable; then
    TRUENAS=true
    INSTALL_DIR="$TRUENAS_STATE_DIR"
    log_info "Immutable filesystem detected (read-only /usr/local/bin). Using $TRUENAS_STATE_DIR for installation."
fi

# QNAP QTS/QuTS hero: the root filesystem is a small RAM-backed volume that is
# rebuilt on every boot, so staging to /tmp and installing to /usr/local/bin
# can both fail on space and never persist anyway (issue #1617). QNAP's own
# QPKG packages execute from the data volume, so stage, install, and run the
# agent from there.
if [[ "$(uname -s)" == "Linux" ]] && { [[ -f /sbin/getcfg ]] || [[ -f /etc/config/qpkg.conf ]]; }; then
    QNAP_EARLY_VOL=$(detect_qnap_data_volume || true)
    if [[ -n "$QNAP_EARLY_VOL" ]]; then
        INSTALL_DIR="${QNAP_EARLY_VOL}/.pulse-agent"
        if [[ -z "${TMPDIR:-}" ]]; then
            QNAP_STAGING_TMPDIR="${QNAP_EARLY_VOL}/.pulse-agent/tmp"
            if mkdir -p "$QNAP_STAGING_TMPDIR" 2>/dev/null; then
                export TMPDIR="$QNAP_STAGING_TMPDIR"
            fi
        fi
        log_info "QNAP detected (RAM-backed root). Staging and installing under ${INSTALL_DIR}."
    fi
fi

# --- Preflight-Only Mode ---
if [[ "$PREFLIGHT_ONLY" == "true" ]]; then
    json_event "preflight" "checking" "Running preflight checks"

    # Check 1: Architecture
    PF_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    PF_ARCH=$(uname -m)
    case "$PF_ARCH" in
        x86_64|amd64) PF_ARCH="amd64" ;;
        aarch64|arm64) PF_ARCH="arm64" ;;
        armv7l|armhf) PF_ARCH="armv7" ;;
        armv6l) PF_ARCH="armv6" ;;
        i386|i686) PF_ARCH="386" ;;
        *) fail "Unsupported architecture: $PF_ARCH" "$EXIT_UNSUPPORTED_ARCH" ;;
    esac
    PF_ARCH_PARAM="${PF_OS}-${PF_ARCH}"
    json_event "preflight" "arch_ok" "Architecture: ${PF_ARCH_PARAM}"

    # Check 2: Existing agent
    AGENT_STATUS="not_installed"
    if [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        AGENT_STATUS="already_installed"
    elif command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet "${AGENT_NAME}" 2>/dev/null; then
        AGENT_STATUS="already_installed"
    fi
    json_event "preflight" "$AGENT_STATUS" "Agent status: ${AGENT_STATUS}"
    PREFLIGHT_EXIT="$EXIT_OK"

    # Check 3: Disk headroom for staging and installing the agent binary
    if ensure_agent_disk_headroom "${TMPDIR:-/tmp}" "$INSTALL_DIR"; then
        json_event "preflight" "disk_ok" "Sufficient disk space for agent install"
    else
        json_event "preflight" "disk_low" "Not enough free disk space to stage and install the agent" "$EXIT_PREFLIGHT_FAILED"
        PREFLIGHT_EXIT="$EXIT_PREFLIGHT_FAILED"
    fi

    # Check 4: Pulse URL reachability and agent binary availability
    if [[ -n "$PULSE_URL" ]]; then
        CURL_TEST_ARGS=(-sfL --connect-timeout 5 -o /dev/null)
        if [[ "$INSECURE" == "true" ]]; then CURL_TEST_ARGS+=(-k); fi
        if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_TEST_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi
        if curl "${CURL_TEST_ARGS[@]}" "${PULSE_URL}/api/health"; then
            json_event "preflight" "pulse_reachable" "Pulse URL reachable"
        else
            json_event "preflight" "pulse_unreachable" "Pulse URL not reachable" "$EXIT_PREFLIGHT_FAILED"
            PREFLIGHT_EXIT="$EXIT_PREFLIGHT_FAILED"
        fi

        PREFLIGHT_HEADERS=$(mktemp)
        TMP_FILES+=("$PREFLIGHT_HEADERS")
        CURL_DOWNLOAD_CHECK_ARGS=(-fsSIL --connect-timeout 5 --max-time 30 -D "$PREFLIGHT_HEADERS" -o /dev/null)
        if [[ "$INSECURE" == "true" ]]; then CURL_DOWNLOAD_CHECK_ARGS+=(-k); fi
        if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_DOWNLOAD_CHECK_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

        DOWNLOAD_CHECK_URL="${PULSE_URL}/download/${BINARY_NAME}?arch=${PF_ARCH_PARAM}"
        if curl "${CURL_DOWNLOAD_CHECK_ARGS[@]}" "$DOWNLOAD_CHECK_URL"; then
            PREFLIGHT_EXPECTED_SHA=$(final_response_header_value "$PREFLIGHT_HEADERS" "X-Checksum-Sha256" || true)
            if [[ -n "$PREFLIGHT_EXPECTED_SHA" ]]; then
                json_event "preflight" "agent_download_available" "Agent binary available for ${PF_ARCH_PARAM}"
            else
                json_event "preflight" "agent_download_checksum_missing" "Agent binary download did not include checksum header" "$EXIT_CHECKSUM_FAILED"
                PREFLIGHT_EXIT="$EXIT_CHECKSUM_FAILED"
            fi
        else
            json_event "preflight" "agent_download_unavailable" "Agent binary unavailable for ${PF_ARCH_PARAM}" "$EXIT_DOWNLOAD_FAILED"
            PREFLIGHT_EXIT="$EXIT_DOWNLOAD_FAILED"
        fi

        if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
            : > "$PREFLIGHT_HEADERS"
            HELPER_DOWNLOAD_CHECK_URL="${PULSE_URL}/download/${PRIVILEGED_HELPER_BINARY_NAME}?arch=${PF_ARCH_PARAM}"
            if curl "${CURL_DOWNLOAD_CHECK_ARGS[@]}" "$HELPER_DOWNLOAD_CHECK_URL"; then
                PREFLIGHT_HELPER_SHA=$(final_response_header_value "$PREFLIGHT_HEADERS" "X-Checksum-Sha256" || true)
                if [[ -n "$PREFLIGHT_HELPER_SHA" ]]; then
                    json_event "preflight" "helper_download_available" "Typed helper binary available for ${PF_ARCH_PARAM}"
                else
                    json_event "preflight" "helper_download_checksum_missing" "Typed helper download did not include checksum header" "$EXIT_CHECKSUM_FAILED"
                    PREFLIGHT_EXIT="$EXIT_CHECKSUM_FAILED"
                fi
            else
                json_event "preflight" "helper_download_unavailable" "Typed helper binary unavailable for ${PF_ARCH_PARAM}" "$EXIT_DOWNLOAD_FAILED"
                PREFLIGHT_EXIT="$EXIT_DOWNLOAD_FAILED"
            fi
        fi
        if [[ "$ACTION_RUNNER_ENABLED" == "true" ]]; then
            : > "$PREFLIGHT_HEADERS"
            RUNNER_DOWNLOAD_CHECK_URL="${PULSE_URL}/download/${ACTION_RUNNER_BINARY_NAME}?arch=${PF_ARCH_PARAM}"
            if curl "${CURL_DOWNLOAD_CHECK_ARGS[@]}" "$RUNNER_DOWNLOAD_CHECK_URL"; then
                PREFLIGHT_RUNNER_SHA=$(final_response_header_value "$PREFLIGHT_HEADERS" "X-Checksum-Sha256" || true)
                if [[ -n "$PREFLIGHT_RUNNER_SHA" ]]; then
                    json_event "preflight" "runner_download_available" "Typed action runner binary available for ${PF_ARCH_PARAM}"
                else
                    json_event "preflight" "runner_download_checksum_missing" "Typed action runner download did not include checksum header" "$EXIT_CHECKSUM_FAILED"
                    PREFLIGHT_EXIT="$EXIT_CHECKSUM_FAILED"
                fi
            else
                json_event "preflight" "runner_download_unavailable" "Typed action runner binary unavailable for ${PF_ARCH_PARAM}" "$EXIT_DOWNLOAD_FAILED"
                PREFLIGHT_EXIT="$EXIT_DOWNLOAD_FAILED"
            fi
        fi
    fi

    # Output summary
    if [[ "$PREFLIGHT_EXIT" -eq 0 ]]; then
        if [[ "$OUTPUT_FORMAT" == "json" ]]; then
            printf '{"phase":"preflight_complete","code":"ok","message":"Preflight checks passed","exitCode":0,"data":{"arch":"%s-%s","agent_status":"%s"}}\n' \
                "$PF_OS" "$PF_ARCH" "$AGENT_STATUS"
        else
            log_info "Preflight checks passed (arch: ${PF_ARCH_PARAM}, agent: ${AGENT_STATUS})"
        fi
    else
        if [[ "$OUTPUT_FORMAT" == "json" ]]; then
            printf '{"phase":"preflight_complete","code":"failed","message":"Preflight checks failed","exitCode":%d,"data":{"arch":"%s-%s","agent_status":"%s"}}\n' \
                "$PREFLIGHT_EXIT" "$PF_OS" "$PF_ARCH" "$AGENT_STATUS"
        else
            log_error "Preflight checks failed (arch: ${PF_ARCH_PARAM}, agent: ${AGENT_STATUS})"
        fi
    fi
    exit "$PREFLIGHT_EXIT"
fi

# --- Download ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armhf) ARCH="armv7" ;;
    armv6l) ARCH="armv6" ;;
    i386|i686) ARCH="386" ;;
    *) fail "Unsupported architecture: $ARCH" "$EXIT_UNSUPPORTED_ARCH" ;;
esac

# Construct arch param in format expected by download endpoint (e.g., linux-amd64)
ARCH_PARAM="${OS}-${ARCH}"

# Fail before downloading if the temp and install filesystems cannot hold the
# staged plus installed binary (mktemp below honours TMPDIR).
if ! ensure_agent_disk_headroom "${TMPDIR:-/tmp}" "$INSTALL_DIR"; then
    fail "Not enough free disk space to install the Pulse agent" "$EXIT_PREFLIGHT_FAILED"
fi

# Create temp file and register for cleanup
TMP_BIN=$(mktemp)
TMP_FILES+=("$TMP_BIN")
TMP_HEADERS=$(mktemp)
TMP_FILES+=("$TMP_HEADERS")

# Build curl arguments as array for proper quoting
CURL_ARGS=(-fsSL --connect-timeout 30 --max-time 300 -D "$TMP_HEADERS" -o "$TMP_BIN")
if [[ "$INSECURE" == "true" ]]; then CURL_ARGS+=(-k); fi
if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

SERVER_VERSION=""
VERSION_CURL_ARGS=(-fsSL --connect-timeout 10 --max-time 30)
if [[ "$INSECURE" == "true" ]]; then VERSION_CURL_ARGS+=(-k); fi
if [[ -n "$CURL_CA_BUNDLE" ]]; then VERSION_CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi
if server_version_json="$(curl "${VERSION_CURL_ARGS[@]}" "${PULSE_URL}/api/version" 2>/dev/null)"; then
    SERVER_VERSION="$(printf '%s' "$server_version_json" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
fi

DOWNLOAD_QUERY="arch=${ARCH_PARAM}"
if [[ -n "$SERVER_VERSION" ]]; then
    DOWNLOAD_QUERY="${DOWNLOAD_QUERY}&serverVersion=${SERVER_VERSION}"
    log_info "Pulse server version: ${SERVER_VERSION}"
fi

DOWNLOAD_URL="${PULSE_URL}/download/${BINARY_NAME}?${DOWNLOAD_QUERY}"
log_info "Downloading agent from ${DOWNLOAD_URL}..."

if ! curl "${CURL_ARGS[@]}" "$DOWNLOAD_URL"; then
    fail "Download failed. Check URL and connectivity." "$EXIT_DOWNLOAD_FAILED"
fi

# Verify downloaded binary
if [[ ! -s "$TMP_BIN" ]]; then
    fail "Downloaded file is empty." "$EXIT_DOWNLOAD_FAILED"
fi

# Check if it's a valid executable (ELF for Linux/FreeBSD, Mach-O for macOS).
# NAS shells (QNAP, some Synology setups) ship without od/hexdump/xxd; fall
# back through whichever exists and skip the sniff when none do rather than
# failing a good download — the SHA-256 verification below still guards
# integrity, this check only catches error pages saved as binaries.
read_magic_hex() {
    if command -v od >/dev/null 2>&1; then
        od -An -tx1 -N4 "$1" 2>/dev/null | tr -d ' \n'
    elif command -v hexdump >/dev/null 2>&1; then
        hexdump -v -e '1/1 "%02x"' -n 4 "$1" 2>/dev/null
    elif command -v xxd >/dev/null 2>&1; then
        xxd -p -l 4 "$1" 2>/dev/null | tr -d ' \n'
    else
        return 1
    fi
}

download_verified_privileged_helper() {
    local helper_headers helper_url helper_magic helper_expected_sha helper_signature helper_actual_sha
    local -a helper_curl_args

    TMP_HELPER_BIN=$(mktemp)
    helper_headers=$(mktemp)
    TMP_FILES+=("$TMP_HELPER_BIN" "$helper_headers")
    helper_curl_args=(-fsSL --connect-timeout 30 --max-time 300 -D "$helper_headers" -o "$TMP_HELPER_BIN")
    if [[ "$INSECURE" == "true" ]]; then helper_curl_args+=(-k); fi
    if [[ -n "$CURL_CA_BUNDLE" ]]; then helper_curl_args+=(--cacert "$CURL_CA_BUNDLE"); fi

    helper_url="${PULSE_URL}/download/${PRIVILEGED_HELPER_BINARY_NAME}?${DOWNLOAD_QUERY}"
    log_info "Downloading typed privileged helper from ${helper_url}..."
    if ! curl "${helper_curl_args[@]}" "$helper_url"; then
        fail "Typed privileged helper download failed; the safe profile was not installed." "$EXIT_DOWNLOAD_FAILED"
    fi
    if [[ ! -s "$TMP_HELPER_BIN" ]]; then
        fail "Downloaded typed privileged helper is empty; the safe profile was not installed." "$EXIT_DOWNLOAD_FAILED"
    fi
    if helper_magic=$(read_magic_hex "$TMP_HELPER_BIN"); then
        if [[ "$helper_magic" != "7f454c46" ]]; then
            fail "Downloaded typed privileged helper is not a Linux ELF executable." "$EXIT_DOWNLOAD_FAILED"
        fi
    else
        log_warn "No od/hexdump/xxd available to inspect the typed helper; relying on checksum verification."
    fi

    helper_expected_sha=$(final_response_header_value "$helper_headers" "X-Checksum-Sha256" || true)
    helper_signature=$(final_response_header_value "$helper_headers" "X-Signature-SSHSIG" || true)
    if [[ -z "$helper_expected_sha" ]]; then
        fail "Typed privileged helper download omitted its checksum; refusing install." "$EXIT_CHECKSUM_FAILED"
    fi
    if has_pinned_installer_signature_key && [[ -z "$helper_signature" ]]; then
        fail "Typed privileged helper download omitted its signature; refusing install." "$EXIT_SIGNATURE_FAILED"
    fi
    helper_actual_sha=$(sha256sum "$TMP_HELPER_BIN" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_HELPER_BIN" 2>/dev/null | awk '{print $1}')
    if [[ -z "$helper_actual_sha" ]]; then
        fail "Could not compute typed privileged helper checksum." "$EXIT_CHECKSUM_FAILED"
    fi
    if [[ "$helper_actual_sha" != "$helper_expected_sha" ]]; then
        fail "Typed privileged helper checksum verification failed." "$EXIT_CHECKSUM_FAILED"
    fi
    verify_download_signature "$TMP_HELPER_BIN" "$helper_signature"
    chmod 0755 "$TMP_HELPER_BIN"
    log_info "Typed privileged helper binary verified"
}

download_verified_action_runner() {
    local runner_headers runner_url runner_magic runner_expected_sha runner_signature runner_actual_sha
    local -a runner_curl_args

    TMP_ACTION_RUNNER_BIN=$(mktemp)
    runner_headers=$(mktemp)
    TMP_FILES+=("$TMP_ACTION_RUNNER_BIN" "$runner_headers")
    runner_curl_args=(-fsSL --connect-timeout 30 --max-time 300 -D "$runner_headers" -o "$TMP_ACTION_RUNNER_BIN")
    if [[ "$INSECURE" == "true" ]]; then runner_curl_args+=(-k); fi
    if [[ -n "$CURL_CA_BUNDLE" ]]; then runner_curl_args+=(--cacert "$CURL_CA_BUNDLE"); fi

    runner_url="${PULSE_URL}/download/${ACTION_RUNNER_BINARY_NAME}?${DOWNLOAD_QUERY}"
    log_info "Downloading typed action runner from ${runner_url}..."
    if ! curl "${runner_curl_args[@]}" "$runner_url"; then
        fail "Typed action runner download failed; the existing runner and collector were not changed." "$EXIT_DOWNLOAD_FAILED"
    fi
    if [[ ! -s "$TMP_ACTION_RUNNER_BIN" ]]; then
        fail "Downloaded typed action runner is empty; the existing runner and collector were not changed." "$EXIT_DOWNLOAD_FAILED"
    fi
    if runner_magic=$(read_magic_hex "$TMP_ACTION_RUNNER_BIN"); then
        if [[ "$runner_magic" != "7f454c46" ]]; then
            fail "Downloaded typed action runner is not a Linux ELF executable." "$EXIT_DOWNLOAD_FAILED"
        fi
    else
        log_warn "No od/hexdump/xxd available to inspect the typed action runner; relying on checksum verification."
    fi
    runner_expected_sha=$(final_response_header_value "$runner_headers" "X-Checksum-Sha256" || true)
    runner_signature=$(final_response_header_value "$runner_headers" "X-Signature-SSHSIG" || true)
    if [[ -z "$runner_expected_sha" ]]; then
        fail "Typed action runner download omitted its checksum; refusing install." "$EXIT_CHECKSUM_FAILED"
    fi
    if has_pinned_installer_signature_key && [[ -z "$runner_signature" ]]; then
        fail "Typed action runner download omitted its signature; refusing install." "$EXIT_SIGNATURE_FAILED"
    fi
    runner_actual_sha=$(sha256sum "$TMP_ACTION_RUNNER_BIN" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_ACTION_RUNNER_BIN" 2>/dev/null | awk '{print $1}')
    if [[ -z "$runner_actual_sha" || "$runner_actual_sha" != "$runner_expected_sha" ]]; then
        fail "Typed action runner checksum verification failed." "$EXIT_CHECKSUM_FAILED"
    fi
    verify_download_signature "$TMP_ACTION_RUNNER_BIN" "$runner_signature"
    chmod 0755 "$TMP_ACTION_RUNNER_BIN"
    log_info "Typed action runner binary verified"
}
if [[ "$OS" == "linux" || "$OS" == "freebsd" ]]; then
    if MAGIC=$(read_magic_hex "$TMP_BIN"); then
        if [[ "$MAGIC" != "7f454c46" ]]; then
            fail "Downloaded file is not a valid ${OS} ELF executable." "$EXIT_DOWNLOAD_FAILED"
        fi
    else
        log_warn "No od/hexdump/xxd available to sniff the binary header; relying on checksum verification."
    fi
elif [[ "$OS" == "darwin" ]]; then
    # Mach-O magic: feedface (32-bit) or feedfacf (64-bit) or cafebabe (universal)
    MAGIC=$(xxd -p -l 4 "$TMP_BIN" 2>/dev/null || head -c 4 "$TMP_BIN" | od -A n -t x1 | tr -d ' ')
    if [[ ! "$MAGIC" =~ ^(cffaedfe|cefaedfe|cafebabe|feedface|feedfacf) ]]; then
        fail "Downloaded file is not a valid macOS executable." "$EXIT_DOWNLOAD_FAILED"
    fi
fi

# Release metadata verification
EXPECTED_SHA=""
SSH_SIGNATURE_HEADER=""
EXPECTED_SHA=$(final_response_header_value "$TMP_HEADERS" "X-Checksum-Sha256" || true)
SSH_SIGNATURE_HEADER=$(final_response_header_value "$TMP_HEADERS" "X-Signature-SSHSIG" || true)

if [[ -z "$EXPECTED_SHA" ]]; then
    fail "Server did not provide checksum header; refusing install." "$EXIT_CHECKSUM_FAILED"
fi

if has_pinned_installer_signature_key && [[ -z "$SSH_SIGNATURE_HEADER" ]]; then
    fail "Server did not provide SSH signature header; refusing signed install." "$EXIT_SIGNATURE_FAILED"
fi

ACTUAL_SHA=$(sha256sum "$TMP_BIN" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_BIN" 2>/dev/null | awk '{print $1}')
if [[ -z "$ACTUAL_SHA" ]]; then
    fail "Could not compute binary checksum." "$EXIT_CHECKSUM_FAILED"
fi
if [[ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]]; then
    fail "Checksum verification failed (expected: ${EXPECTED_SHA:0:16}..., got: ${ACTUAL_SHA:0:16}...)" "$EXIT_CHECKSUM_FAILED"
fi
json_event "download" "checksum_ok" "Binary checksum verified"
log_info "Binary checksum verified"

verify_download_signature "$TMP_BIN" "$SSH_SIGNATURE_HEADER"

if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
    download_verified_privileged_helper
fi
if [[ "$ACTION_RUNNER_ENABLED" == "true" ]]; then
    download_verified_action_runner
fi

chmod 0755 "$TMP_BIN"
if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    # The irreversible server-side reduction must run only through the staged
    # binary whose checksum/signature have just been verified. The installed
    # predecessor may not expose the authenticated lifecycle commands yet.
    # Snapshot first, then reduce, before stopping or replacing any local
    # runtime so a failed reduction leaves the legacy install untouched.
    safe_profile_begin_transaction
    SAFE_PROFILE_STAGED_COLLECTOR="${INSTALL_DIR}/.${BINARY_NAME}.safe-profile-new.$$"
    TMP_FILES+=("$SAFE_PROFILE_STAGED_COLLECTOR")
    install -o root -g root -m 0755 "$TMP_BIN" "$SAFE_PROFILE_STAGED_COLLECTOR" ||
        fail "Safe-profile migration could not stage the verified collector on the installation filesystem" "$EXIT_GENERAL"
    COLLECTOR_LIFECYCLE_BINARY_PATH="$SAFE_PROFILE_STAGED_COLLECTOR"
    reduce_safe_profile_collector_authority ||
        fail "Safe-profile migration could not durably remove command and cross-host management authority from the existing collector credential; no privilege change was retained" "$EXIT_AUTH_REJECTED"
fi
VERSION_PROBE_BINARY="$TMP_BIN"
if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    VERSION_PROBE_BINARY="$SAFE_PROFILE_STAGED_COLLECTOR"
fi
NEW_VERSION=$("$VERSION_PROBE_BINARY" --version 2>/dev/null | head -1 || echo "unknown")

# Compare versions with any leading "v" stripped so the agent binary's "v6.0.4"
# and the server /api/version "6.0.4" are treated as equal. Only a genuine
# version difference (e.g. 6.0.3 vs 6.0.4) should raise the mismatch warning.
#
# Semver build metadata is stripped for the same reason. A server built from a
# working tree reports "6.2.0-rc.8+git.46.g98a638e00.dirty" while the agent it
# serves carries the release identity "v6.2.0-rc.8"; those are the same release,
# and comparing them raw made this warning fire on every correct development
# install. A warning that fires when nothing is wrong is worse than no warning,
# because it trains the reader to skip the one time it is real. The prerelease
# suffix is deliberately kept: 6.2.0-rc.8 and 6.2.0 are genuinely different.
NEW_VERSION_NORMALIZED="${NEW_VERSION#v}"
NEW_VERSION_NORMALIZED="${NEW_VERSION_NORMALIZED%%+*}"
SERVER_VERSION_NORMALIZED="${SERVER_VERSION#v}"
SERVER_VERSION_NORMALIZED="${SERVER_VERSION_NORMALIZED%%+*}"

if [[ -n "$SERVER_VERSION" && -n "$NEW_VERSION" && "$NEW_VERSION" != "unknown" && "$NEW_VERSION_NORMALIZED" != "$SERVER_VERSION_NORMALIZED" ]]; then
    log_warn "Downloaded agent version (${NEW_VERSION}) does not match Pulse server version (${SERVER_VERSION}). Check that Pulse is upgraded and that any reverse proxy is not serving a stale cached binary."
fi

# --- Upgrade Detection ---
# Check if pulse-agent is already installed and handle upgrade gracefully
EXISTING_VERSION=""
UPGRADE_MODE=false

if [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    EXISTING_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null | head -1 || echo "unknown")

    if [[ -n "$EXISTING_VERSION" && "$EXISTING_VERSION" != "unknown" ]]; then
        UPGRADE_MODE=true
        log_info "Existing installation detected: $EXISTING_VERSION"
        log_info "Upgrading to: $NEW_VERSION"
        
        # Stop the existing agent service gracefully through the installer-owned helper.
        stop_existing_agent_service || true
        
        # Also kill any running process in case it was started manually.
        # The trailing boundary matters: pkill -f matches the whole command
        # line and "^" only anchors the start, so an unbounded pattern also
        # matches a co-installed agent whose binary name merely starts with
        # this one (pulse-agent matching pulse-agent-prod).
        pkill -f "^${INSTALL_DIR}/${BINARY_NAME}([[:space:]]|$)" 2>/dev/null || true
        sleep 1
    fi
elif command -v systemctl >/dev/null 2>&1 && systemctl is-enabled --quiet "${AGENT_NAME}" 2>/dev/null; then
    # Service exists but binary is missing - reinstall scenario
    if [[ "$UPDATE_ONLY" == "true" ]]; then
        fail "No existing Pulse Agent binary found to update. Use the install command instead." "$EXIT_MISSING_ARGS"
    fi
    log_info "Agent service exists but binary is missing. Reinstalling..."
    systemctl stop "${AGENT_NAME}" 2>/dev/null || true
fi

if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    # Freeze the legacy collector before recording its server-side freshness
    # marker. The replacement must advance this exact registration row after
    # activation; an old row that merely still exists cannot commit migration.
    stop_existing_agent_service || true
    pkill -f "^${INSTALL_DIR}/${BINARY_NAME}([[:space:]]|$)" 2>/dev/null || true
    AGENT_REGISTRATION_LAST_SEEN=""
    if ! verify_agent_server_registration_with_retry || [[ -z "$AGENT_REGISTRATION_LAST_SEEN" ]]; then
        fail "Safe-profile migration could not capture the stopped collector's server registration freshness marker; restoring the previous profile" "$EXIT_GENERAL"
    fi
    SAFE_PROFILE_PRIOR_REGISTRATION_LAST_SEEN="$AGENT_REGISTRATION_LAST_SEEN"
fi

if [[ "$UPDATE_ONLY" == "true" && "$UPGRADE_MODE" != "true" ]]; then
    fail "No existing Pulse Agent installation found to update. Use the install command instead." "$EXIT_MISSING_ARGS"
fi

# Install Binary
log_info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
    [[ -x "$SAFE_PROFILE_STAGED_COLLECTOR" && -f "$SAFE_PROFILE_STAGED_COLLECTOR" ]] ||
        fail "Verified safe-profile collector staging artifact is unavailable" "$EXIT_GENERAL"
    mv "$SAFE_PROFILE_STAGED_COLLECTOR" "${INSTALL_DIR}/${BINARY_NAME}"
    COLLECTOR_LIFECYCLE_BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"
else
    mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"
fi

if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
    mkdir -p "$PRIVILEGE_HELPER_DIR"
    chown root:root "$PRIVILEGE_HELPER_DIR"
    chmod 0755 "$PRIVILEGE_HELPER_DIR"
    if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
        SAFE_PROFILE_STAGED_HELPER="${PRIVILEGED_HELPER_BINARY_PATH}.safe-profile-new.$$"
        TMP_FILES+=("$SAFE_PROFILE_STAGED_HELPER")
        install -o root -g root -m 0755 "$TMP_HELPER_BIN" "$SAFE_PROFILE_STAGED_HELPER"
        mv "$SAFE_PROFILE_STAGED_HELPER" "$PRIVILEGED_HELPER_BINARY_PATH"
    else
        mv "$TMP_HELPER_BIN" "$PRIVILEGED_HELPER_BINARY_PATH"
    fi
    TMP_HELPER_BIN=""
    chown root:root "$PRIVILEGED_HELPER_BINARY_PATH"
    chmod 0755 "$PRIVILEGED_HELPER_BINARY_PATH"
fi

if [[ "$UPGRADE_MODE" == "true" ]]; then
    log_info "Binary upgraded successfully. Updating service configuration..."
fi

# --- Service Installation ---

# 1. macOS (Launchd)
if [[ "$OS" == "darwin" ]]; then
    PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
    log_info "Configuring Launchd service at $PLIST..."
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed

    build_plist_program_arguments "${INSTALL_DIR}/${BINARY_NAME}"

    cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pulse.agent</string>
    <key>ProgramArguments</key>
    <array>
${PLIST_ARGS}
    </array>${PLIST_ENV_BLOCK}
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_FILE}</string>
    <key>StandardErrorPath</key>
    <string>${LOG_FILE}</string>
</dict>
</plist>
EOF
    chmod 644 "$PLIST"
    launchctl unload "$PLIST" 2>/dev/null || true
    launchctl load -w "$PLIST"
    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f $LOG_FILE"
    exit 0
fi

# 2. Synology DSM
# DSM 7+ uses systemd, DSM 6.x uses upstart
if [[ -d /usr/syno ]] && [[ -f /etc/VERSION ]]; then
    # Extract major version from /etc/VERSION
    DSM_MAJOR=$(grep 'majorversion=' /etc/VERSION | cut -d'"' -f2)
    log_info "Detected Synology DSM ${DSM_MAJOR}..."

    # Build command line args
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    if [[ "$DSM_MAJOR" -ge 7 ]]; then
        # DSM 7+ uses systemd
        UNIT="/etc/systemd/system/${AGENT_NAME}.service"
        log_info "Configuring systemd service at $UNIT (DSM 7+)..."

        render_systemd_agent_unit "$UNIT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}" "network.target" "" "" ""
        restart_systemd_agent_service
    else
        # DSM 6.x uses upstart
        CONF="/etc/init/${AGENT_NAME}.conf"
        log_info "Configuring Upstart service at $CONF (DSM 6.x)..."

        cat > "$CONF" <<EOF
description "Pulse Unified Agent"
author "Pulse"

start on syno.network.ready
stop on runlevel [06]

respawn
respawn limit 5 10${UPSTART_ENV_LINES}

exec ${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS} >> ${LOG_FILE} 2>&1
EOF
        initctl stop "${AGENT_NAME}" 2>/dev/null || true
        initctl start "${AGENT_NAME}"
    fi

    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f $LOG_FILE"
    exit 0
fi

# 3. Unraid (no init system - use /boot/config/go script)
# Detect Unraid by /etc/unraid-version (preferred) or /boot/config/go with unraid markers
if [[ -f /etc/unraid-version ]]; then
    log_info "Detected Unraid system..."

    # Unraid's /boot is FAT32 (no execute permission), so we store the binary there
    # for persistence but copy it to RAM disk (/usr/local/bin) for execution
    select_platform_state_dir "/boot/config/plugins/pulse-agent"
    UNRAID_STORAGE_DIR="$STATE_DIR"
    UNRAID_STORED_BINARY="${UNRAID_STORAGE_DIR}/${BINARY_NAME}"
    RUNTIME_BINARY="${INSTALL_DIR}/${BINARY_NAME}"
    GO_SCRIPT="/boot/config/go"

    mkdir -p "$UNRAID_STORAGE_DIR"

    # Copy binary to persistent storage (for survival across reboots)
    cp "${RUNTIME_BINARY}" "$UNRAID_STORED_BINARY"
    # Keep binary in /usr/local/bin (RAM disk) with execute permission for runtime
    chmod +x "${RUNTIME_BINARY}"

    log_info "Installed binary to ${UNRAID_STORED_BINARY} (persistent) and ${RUNTIME_BINARY} (runtime)..."

    # Unraid's /var/log is a small tmpfs and /boot is flash (unsuitable for
    # logs), so use the agent's rotating writer to cap log growth (issue #1617).
    # A subdirectory is required: the rotating writer chmods its log directory.
    UNRAID_LOG_DIR="/var/log/${AGENT_NAME}"
    AGENT_LOG_FILE="${UNRAID_LOG_DIR}/${AGENT_NAME}.log"

    # Build command line args (string for wrapper script, array for direct execution)
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args
    build_exec_args_array

    # Kill any existing pulse agents.
    log_info "Stopping any existing pulse agents..."
    # Stop the supervisor before the agent it supervises. The wrapper is a
    # watchdog loop, so killing the agent while its wrapper is still running
    # only races the respawn, and leaving that wrapper alive alongside the one
    # started at the end of this install leaves two loops competing to own the
    # same agent id. Matching on the trailing path segment catches a wrapper
    # left behind at an older storage location, while the escaped dot and the
    # trailing boundary keep a co-installed agent's wrapper
    # (start-pulse-agent-prod.sh) out of the match.
    pkill -f "/start-pulse-agent\.sh([[:space:]]|$)" 2>/dev/null || true
    # Use process name matching to avoid killing unrelated processes. The
    # trailing boundary keeps a co-installed agent whose binary name starts
    # with this one (pulse-agent vs pulse-agent-prod) out of the match.
    pkill -f "^${RUNTIME_BINARY}([[:space:]]|$)" 2>/dev/null || true
    sleep 2

    # Create a wrapper script that will be called from /boot/config/go
    # This script copies from persistent storage to RAM disk on boot, then starts the agent
    EXPORT_SERVICE_ENV="$SHELL_EXPORT_LINES"

    WRAPPER_SCRIPT="${UNRAID_STORAGE_DIR}/start-pulse-agent.sh"
    cat > "$WRAPPER_SCRIPT" <<EOF
#!/bin/bash
# Pulse Agent startup script for Unraid
# Auto-generated by Pulse installer
# Includes watchdog loop to restart agent on failure

WATCHDOG_LOG="/var/log/${AGENT_NAME}-watchdog.log"

# The watchdog log has no rotation, so cap it (it lives on a small tmpfs).
trim_watchdog_log() {
    [ -f "\$WATCHDOG_LOG" ] || return 0
    _size=\$(wc -c < "\$WATCHDOG_LOG" 2>/dev/null | tr -d ' \t')
    case "\$_size" in ''|*[!0-9]*) return 0 ;; esac
    if [ "\$_size" -gt 5242880 ]; then
        tail -c 1048576 "\$WATCHDOG_LOG" > "\${WATCHDOG_LOG}.tmp" 2>/dev/null && mv "\${WATCHDOG_LOG}.tmp" "\$WATCHDOG_LOG"
    fi
}

# Kill any existing pulse-agent processes.
# The trailing boundary is required: pkill -f matches the whole command line
# and "^" only anchors the start, so without it this also kills a co-installed
# agent whose binary name starts with this one (pulse-agent vs
# pulse-agent-prod), which on a host running both takes down the other agent
# every time this wrapper restarts.
pkill -f "^${RUNTIME_BINARY}([[:space:]]|\$)" 2>/dev/null || true
sleep 2

# Copy binary from persistent storage to RAM disk (needed after reboot)
cp "${UNRAID_STORED_BINARY}" "${RUNTIME_BINARY}"
chmod +x "${RUNTIME_BINARY}"${EXPORT_SERVICE_ENV}

# Watchdog loop: restart agent if it exits
# Uses exponential backoff to prevent rapid restart loops
RESTART_DELAY=5
MAX_RESTART_DELAY=60

while true; do
    trim_watchdog_log
    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] Starting pulse-agent (agent log: ${AGENT_LOG_FILE})..." >> "\$WATCHDOG_LOG"
    # The agent writes its own rotating log via --log-file; discard the stdout
    # mirror so unrotated output cannot fill the tmpfs-backed /var/log.
    ${RUNTIME_BINARY} ${EXEC_ARGS} > /dev/null 2>> "\$WATCHDOG_LOG"
    EXIT_CODE=\$?

    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] pulse-agent exited with code \$EXIT_CODE, restarting in \${RESTART_DELAY}s..." >> "\$WATCHDOG_LOG"
    sleep \$RESTART_DELAY

    # Exponential backoff (cap at MAX_RESTART_DELAY)
    RESTART_DELAY=\$((RESTART_DELAY * 2))
    if [ \$RESTART_DELAY -gt \$MAX_RESTART_DELAY ]; then
        RESTART_DELAY=\$MAX_RESTART_DELAY
    fi
done
EOF


    # Add to /boot/config/go if not already present
    GO_MARKER="# Pulse Agent"
    if [[ -f "$GO_SCRIPT" ]]; then
        # Remove any existing Pulse agent entries (line-by-line, not range-based)
        sed -i "/^${GO_MARKER}$/d" "$GO_SCRIPT" 2>/dev/null || true
        sed -i '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || true
    else
        # Create go script if it doesn't exist
        echo "#!/bin/bash" > "$GO_SCRIPT"
        chmod +x "$GO_SCRIPT"
    fi

    # Append startup entry (use bash explicitly since /boot is FAT32 and doesn't support execute bits)
    cat >> "$GO_SCRIPT" <<EOF

${GO_MARKER}
bash ${WRAPPER_SCRIPT}

EOF

    log_info "Added startup entry to ${GO_SCRIPT}..."

    # Start the agent now using the wrapper script (includes watchdog)
    # Use shell backgrounding instead of nohup for broader compatibility (QNAP, etc.)
    log_info "Starting agent with watchdog..."
    bash "${WRAPPER_SCRIPT}" >> "/var/log/${AGENT_NAME}.log" 2>&1 &
    disown 2>/dev/null || true  # Disown if available to prevent SIGHUP

    complete_installation_flow "$UNRAID_STORAGE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f ${AGENT_LOG_FILE}"
    log_info "The agent will start automatically on boot."
    log_info "To check status: pgrep -a pulse-agent"
    log_info "To view logs: tail -f ${AGENT_LOG_FILE}"
    log_info "Watchdog log: /var/log/${AGENT_NAME}-watchdog.log"
    exit 0
fi

# 3b. QNAP QTS/QuTS hero (ephemeral boot config; autorun.sh executes before the
# encrypted data volume is always ready, so boot persistence must wait for the
# canonical persistent wrapper on the data volume).
if [[ -f /sbin/getcfg ]] || [[ -f /etc/config/qpkg.conf ]]; then
    log_info "Detected QNAP QTS/QuTS hero system..."

    QNAP_VOL=$(detect_qnap_data_volume || true)
    if [[ -z "$QNAP_VOL" ]]; then
        fail "Could not find a writable QNAP data volume. Is a storage volume configured?"
    fi

    select_platform_state_dir "${QNAP_VOL}/.pulse-agent"
    QNAP_STORED_BINARY="${STATE_DIR}/${BINARY_NAME}"
    RUNTIME_BINARY="${INSTALL_DIR}/${BINARY_NAME}"
    WRAPPER_SCRIPT="${STATE_DIR}/start-pulse-agent.sh"

    mkdir -p "$STATE_DIR"

    # Copy binary to persistent storage and keep the runtime copy executable.
    # With the data-volume install dir these are the same file; the copy only
    # applies when a custom STATE_DIR separates them.
    if [[ "$RUNTIME_BINARY" != "$QNAP_STORED_BINARY" ]]; then
        cp "${RUNTIME_BINARY}" "$QNAP_STORED_BINARY"
    fi
    chmod +x "$QNAP_STORED_BINARY"
    chmod +x "$RUNTIME_BINARY"

    # A pre-relocation install left its runtime copy on the RAM-backed root;
    # reclaim that space now that the agent runs from the data volume.
    if [[ "$RUNTIME_BINARY" != "/usr/local/bin/${BINARY_NAME}" ]]; then
        rm -f "/usr/local/bin/${BINARY_NAME}"
    fi

    log_info "Installed binary to ${QNAP_STORED_BINARY} (persistent) and ${RUNTIME_BINARY} (runtime)..."

    # Log to the data volume with the agent's rotating writer; the RAM-backed
    # root (/var/log) must not accumulate agent output (issue #1617).
    QNAP_LOG_DIR="${STATE_DIR}/logs"
    AGENT_LOG_FILE="${QNAP_LOG_DIR}/${AGENT_NAME}.log"

    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    log_info "Stopping any existing pulse agents..."
    # Supervisor before the agent it supervises: the wrapper is a watchdog, so
    # stopping the agent first only races the respawn. The dot is escaped and
    # the far end bounded so an editor session or a .bak copy of the wrapper is
    # not swept up with it.
    pkill -f "/start-pulse-agent\.sh([[:space:]]|$)" 2>/dev/null || true
    pkill -x "pulse-agent" 2>/dev/null || true
    sleep 2

    write_qnap_wrapper_script "$WRAPPER_SCRIPT" "$RUNTIME_BINARY" "$QNAP_STORED_BINARY" "$QNAP_LOG_DIR" "$STATE_DIR"

    AUTORUN_CONFIGURED=false
    if [[ -x /etc/init.d/init_disk.sh ]]; then
        if /etc/init.d/init_disk.sh mount_flash_config 2>/dev/null && [[ -d /tmp/nasconfig_tmp ]]; then
            AUTORUN_PATH="/tmp/nasconfig_tmp/autorun.sh"
            append_qnap_autorun_block "$AUTORUN_PATH" "$WRAPPER_SCRIPT" "$STATE_DIR"
            /etc/init.d/init_disk.sh umount_flash_config 2>/dev/null || true
            AUTORUN_CONFIGURED=true
            log_info "Configured autorun.sh with a deferred Pulse Agent bootstrap."
        else
            /etc/init.d/init_disk.sh umount_flash_config 2>/dev/null || true
        fi
    fi

    if [[ "$AUTORUN_CONFIGURED" != true ]]; then
        log_warn "Could not configure autorun.sh automatically."
        log_warn "To persist across reboots, add a block to autorun.sh that waits for ${WRAPPER_SCRIPT} and then launches it."
        log_warn "See: https://wiki.qnap.com/wiki/Running_Your_Own_Application_at_Startup"
    fi

    log_info "Starting agent with QNAP watchdog..."
    sh "${WRAPPER_SCRIPT}" >> "/var/log/${AGENT_NAME}.log" 2>&1 &
    disown 2>/dev/null || true

    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f ${AGENT_LOG_FILE}"
    log_info "Persistent state: $STATE_DIR"
    if [[ "$AUTORUN_CONFIGURED" == true ]]; then
        log_info "The agent will start automatically after the QNAP data volume becomes available."
        log_info "IMPORTANT: Ensure 'Run user defined startup processes (autorun.sh)' is enabled"
        log_info "  in QNAP Control Panel > Hardware > General."
    fi
    log_info "To check status: pgrep -a pulse-agent"
    log_info "To view logs: tail -f ${AGENT_LOG_FILE}"
    log_info "Watchdog log: ${QNAP_LOG_DIR}/${AGENT_NAME}-watchdog.log"
    exit 0
fi

# 4. TrueNAS SCALE/CORE (immutable root, uses systemd on SCALE and rc.d on CORE)
# TrueNAS can wipe service registration files on upgrades, so we store the service
# in /data and create an Init/Shutdown task to recreate the symlink on boot.
# Note: /data may have exec=off on some TrueNAS systems. We try multiple runtime locations.
if [[ "$TRUENAS" == true ]]; then
    log_info "Configuring TrueNAS SCALE/CORE installation..."
    select_platform_state_dir "$TRUENAS_STATE_DIR"
    TRUENAS_STATE_DIR="$STATE_DIR"
    TRUENAS_LOG_DIR="$TRUENAS_STATE_DIR/logs"
    TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
    TRUENAS_ENV_FILE="$TRUENAS_STATE_DIR/pulse-agent.env"

    # Stop any existing agent before we modify binaries
    # The runtime binary may be in /root/bin or /var/tmp, not just INSTALL_DIR
    if [[ "$(uname -s)" == "Linux" ]]; then
        if systemctl is-active --quiet "${AGENT_NAME}" 2>/dev/null; then
            log_info "Stopping existing ${AGENT_NAME} service..."
            systemctl stop "${AGENT_NAME}" 2>/dev/null || true
            sleep 2
        fi
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        if service "${AGENT_NAME}" status >/dev/null 2>&1; then
            log_info "Stopping existing ${AGENT_NAME} service..."
            service "${AGENT_NAME}" stop 2>/dev/null || true
            sleep 2
        fi
    fi
    # Kill any remaining pulse-agent processes (may be running from different
    # paths). -x matches the process name exactly, which keeps the
    # path-agnostic intent while excluding a co-installed agent whose name
    # merely starts with this one (pulse-agent-prod).
    pkill -9 -x "${BINARY_NAME}" 2>/dev/null || true
    sleep 1
    # Remove old runtime binaries that may be "text file busy"
    rm -f /root/bin/pulse-agent 2>/dev/null || true
    rm -f /var/tmp/pulse-agent 2>/dev/null || true

    # Create directories
    mkdir -p "$TRUENAS_STATE_DIR"
    mkdir -p "$TRUENAS_LOG_DIR"

    TRUENAS_STORED_BINARY="$TRUENAS_STATE_DIR/${BINARY_NAME}"

    # Move binary to persistent storage location
    if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]] && [[ "$INSTALL_DIR" == "$TRUENAS_STATE_DIR" ]]; then
        # Binary already in the right place from earlier mv
        :
    else
        mv "${INSTALL_DIR}/${BINARY_NAME}" "$TRUENAS_STORED_BINARY"
    fi
    chmod +x "$TRUENAS_STORED_BINARY"

    # Determine runtime binary location - try executing from /data first
    # TrueNAS SCALE 24.04+ has read-only /usr/local/bin, so we need alternatives
    TRUENAS_RUNTIME_BINARY=""

    # Test if /data allows execution (no noexec mount option)
    if "$TRUENAS_STORED_BINARY" --version >/dev/null 2>&1; then
        log_info "Binary can execute from /data - using direct execution."
        TRUENAS_RUNTIME_BINARY="$TRUENAS_STORED_BINARY"
    else
        # /data has noexec, need to copy to an executable location
        # Try locations in order of preference
        for RUNTIME_DIR in "/usr/local/bin" "/root/bin" "/var/tmp"; do
            if [[ "$RUNTIME_DIR" == "/root/bin" ]]; then
                mkdir -p "$RUNTIME_DIR" 2>/dev/null || continue
            fi

            # Test if we can write and execute from this location
            TEST_FILE="${RUNTIME_DIR}/.pulse-exec-test-$$"
            if cp "$TRUENAS_STORED_BINARY" "$TEST_FILE" 2>/dev/null && \
               chmod +x "$TEST_FILE" 2>/dev/null && \
               "$TEST_FILE" --version >/dev/null 2>&1; then
                rm -f "$TEST_FILE"
                TRUENAS_RUNTIME_BINARY="${RUNTIME_DIR}/${BINARY_NAME}"
                log_info "Using ${RUNTIME_DIR} for binary execution."
                break
            fi
            rm -f "$TEST_FILE" 2>/dev/null
        done
    fi

    if [[ -z "$TRUENAS_RUNTIME_BINARY" ]]; then
        log_error "Could not find a writable location that allows execution."
        log_error "Tried: /data (noexec), /usr/local/bin (read-only), /root/bin, /var/tmp"
        exit 1
    fi

    # Copy to runtime location if different from storage location
    if [[ "$TRUENAS_RUNTIME_BINARY" != "$TRUENAS_STORED_BINARY" ]]; then
        cp "$TRUENAS_STORED_BINARY" "$TRUENAS_RUNTIME_BINARY"
        chmod +x "$TRUENAS_RUNTIME_BINARY"
    fi

    # Build command line args
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    # Store service file in /data (persists across upgrades)
    TRUENAS_SERVICE_STORAGE="$TRUENAS_STATE_DIR/${AGENT_NAME}.service"

    if [[ "$(uname -s)" == "Linux" ]]; then
        TRUENAS_LOG_TARGET="$LOG_FILE"
        if [[ -n "$TRUENAS_LOG_FILE" ]]; then
            TRUENAS_LOG_TARGET="$TRUENAS_LOG_FILE"
        fi

        render_systemd_agent_unit "$TRUENAS_SERVICE_STORAGE" "${TRUENAS_RUNTIME_BINARY}" "${EXEC_ARGS}" "network-online.target docker.service" "network-online.target" "root" "${TRUENAS_LOG_TARGET}"
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        render_freebsd_rc_agent_script "$TRUENAS_SERVICE_STORAGE" "${TRUENAS_RUNTIME_BINARY}" "${EXEC_ARGS}"
    fi

    # Store environment/config for reference
    cat > "$TRUENAS_ENV_FILE" <<EOF
# Pulse Agent configuration (for reference)
PULSE_STATE_DIR=${STATE_DIR}
PULSE_URL=${PULSE_URL}
PULSE_TOKEN_FILE=${RUNTIME_TOKEN_FILE}
PULSE_INTERVAL=${INTERVAL}
PULSE_ENABLE_HOST=${ENABLE_HOST}
PULSE_ENABLE_DOCKER=${ENABLE_DOCKER}
PULSE_ENABLE_KUBERNETES=${ENABLE_KUBERNETES}
PULSE_KUBE_INCLUDE_ALL_PODS=${KUBE_INCLUDE_ALL_PODS}
PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS=${KUBE_INCLUDE_ALL_DEPLOYMENTS}
PULSE_SERVER_FINGERPRINT=${SERVER_FINGERPRINT}
EOF
    chmod 600 "$TRUENAS_ENV_FILE"

    # Create bootstrap script that runs on boot
    # This script handles the runtime binary location and recreates the service symlink.
    write_truenas_bootstrap_script "$(uname -s)"

    # Create systemd/rc.d symlink now
    if [[ "$(uname -s)" == "Linux" ]]; then
        SYSTEMD_LINK="/etc/systemd/system/${AGENT_NAME}.service"
        ln -sf "$TRUENAS_SERVICE_STORAGE" "$SYSTEMD_LINK"
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        RCSCRIPT_LINK="/usr/local/etc/rc.d/${AGENT_NAME}"
        ln -sf "$TRUENAS_SERVICE_STORAGE" "$RCSCRIPT_LINK"
    fi

    # Register Init/Shutdown task using midclt
    if command -v midclt >/dev/null 2>&1; then
        log_info "Registering TrueNAS Init/Shutdown task..."

        # Check if task already exists
        EXISTING_TASK=$(midclt call initshutdownscript.query '[["script","=","'"$TRUENAS_BOOTSTRAP_SCRIPT"'"]]' 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null || echo "")

        if [[ -n "$EXISTING_TASK" ]]; then
            log_info "Init/Shutdown task already exists (id $EXISTING_TASK), updating..."
            midclt call initshutdownscript.update "$EXISTING_TASK" '{"type":"SCRIPT","script":"'"$TRUENAS_BOOTSTRAP_SCRIPT"'","when":"POSTINIT","enabled":true,"timeout":30,"comment":"Pulse Agent Bootstrap"}' >/dev/null 2>&1 || true
        else
            midclt call initshutdownscript.create '{"type":"SCRIPT","script":"'"$TRUENAS_BOOTSTRAP_SCRIPT"'","when":"POSTINIT","enabled":true,"timeout":30,"comment":"Pulse Agent Bootstrap"}' >/dev/null 2>&1 || log_warn "Failed to create Init/Shutdown task. Please add it manually in TrueNAS UI."
        fi
    else
        log_warn "midclt not available. Please create an Init/Shutdown task manually in TrueNAS UI:"
        log_warn "  Type: Script"
        log_warn "  Script: $TRUENAS_BOOTSTRAP_SCRIPT"
        log_warn "  When: Post Init"
    fi

    # Enable and start service
    if [[ "$(uname -s)" == "Linux" ]]; then
        restart_systemd_agent_service
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        ensure_freebsd_agent_enabled
        restart_service_command_agent
    fi

    complete_installation_flow "$TRUENAS_STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." ""
    log_info "Binary: $TRUENAS_STORED_BINARY (persistent)"
    log_info "Runtime: $TRUENAS_RUNTIME_BINARY (for execution)"
    if [[ "$(uname -s)" == "Linux" ]]; then
        log_info "Service: $TRUENAS_SERVICE_STORAGE (symlinked to systemd)"
        log_info "Logs: tail -f ${TRUENAS_LOG_FILE}"
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        log_info "Service: $TRUENAS_SERVICE_STORAGE (symlinked to rc.d)"
        log_info "Logs: tail -f /var/log/messages"
    fi
    log_info ""
    log_info "The Init/Shutdown task ensures the agent survives TrueNAS upgrades."
    exit 0
fi

# 5. OpenRC (Alpine, Gentoo, Artix, etc.)
# Check for rc-service but make sure we're not on a systemd system that happens to have it
if command -v rc-service >/dev/null 2>&1 && [[ -d /etc/init.d ]] && ! command -v systemctl >/dev/null 2>&1; then
    INITSCRIPT="/etc/init.d/${AGENT_NAME}"
    log_info "Configuring OpenRC service at $INITSCRIPT..."

    # Build command line args
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    # Create OpenRC init script following Alpine best practices
    # Using command_background=yes with pidfile for proper daemon management
    cat > "$INITSCRIPT" <<'INITEOF'
#!/sbin/openrc-run
# Pulse Unified Agent OpenRC init script

name="pulse-agent"
description="Pulse Unified Agent"

command="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
command_args="EXEC_ARGS_PLACEHOLDER"
SSL_CERT_FILE_PLACEHOLDER
command_background="yes"
command_user="root"

pidfile="/run/${RC_SVCNAME}.pid"
output_log="/var/log/pulse-agent.log"
error_log="/var/log/pulse-agent.log"

# Ensure log file exists
start_pre() {
    touch "$output_log"
}

depend() {
    need net
    use docker
}
INITEOF

    # Replace placeholders with actual values
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$INITSCRIPT"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$INITSCRIPT"
    sed -i "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$INITSCRIPT"
    sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SED_EXPORT_LINES}|g" "$INITSCRIPT"

    chmod +x "$INITSCRIPT"
    restart_openrc_agent_service
    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f $LOG_FILE"
    exit 0
fi

# 5b. FreeBSD rc.d (OPNsense, pfSense, vanilla FreeBSD)
if [[ "$OS" == "freebsd" ]] || [[ -f /etc/rc.subr ]]; then
    RCSCRIPT="/usr/local/etc/rc.d/${AGENT_NAME}"
    log_info "Configuring FreeBSD rc.d service at $RCSCRIPT..."

    # Build command line args
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    render_freebsd_rc_agent_script "$RCSCRIPT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}"

    # Enable the service in rc.conf
    ensure_freebsd_agent_enabled

    # pfSense does not use the standard FreeBSD rc.d boot system.
    # Scripts in /usr/local/etc/rc.d/ must end in .sh to run at boot.
    # Create a .sh wrapper that invokes the rc.d script on boot.
    if [ -f /usr/local/sbin/pfSsh.php ] || ([ -f /etc/platform ] && grep -qi pfsense /etc/platform 2>/dev/null); then
        BOOT_WRAPPER="/usr/local/etc/rc.d/pulse_agent.sh"
        log_info "Detected pfSense — creating boot wrapper at $BOOT_WRAPPER..."
        cat > "$BOOT_WRAPPER" <<'BOOTEOF'
#!/bin/sh
# pfSense boot wrapper for pulse-agent
# pfSense requires .sh extension for scripts to run at boot
/usr/local/etc/rc.d/pulse-agent start
BOOTEOF
        chmod +x "$BOOT_WRAPPER"
    fi

    # Stop existing agent if running
    restart_sysv_agent_service "$RCSCRIPT"
    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f /var/log/messages"
    log_info "To check status: $RCSCRIPT status"
    log_info "To view logs: tail -f /var/log/messages"
    exit 0
fi

# 5. Linux (Systemd)
if command -v systemctl >/dev/null 2>&1; then
    UNIT="/etc/systemd/system/${AGENT_NAME}.service"
    TOKEN_DIR="$STATE_DIR"
    TOKEN_FILE="${TOKEN_DIR}/token"
    log_info "Configuring Systemd service at $UNIT..."

    # A least-privilege install must survive updates that do not repeat the
    # flags: recover the profile and its grants from the existing unit before
    # rendering a replacement, so an --update never silently reverts the
    # service to root.
    if [[ -f "$UNIT" ]]; then
        if [[ "$LEAST_PRIVILEGE" != "true" ]] && grep -q "^User=${LEAST_PRIVILEGE_USER}\$" "$UNIT"; then
            if [[ "$ENABLE_COMMANDS" == "true" ]]; then
                fail "This agent runs the least-privilege profile; --enable-commands requires reinstalling the root profile first" "$EXIT_MISSING_ARGS"
            fi
            LEAST_PRIVILEGE="true"
            log_info "Preserving existing least-privilege profile (User=${LEAST_PRIVILEGE_USER})"
        fi
        if [[ "$LEAST_PRIVILEGE" == "true" && "$SAFE_PROFILE_ACTION" != "apply" ]]; then
            if [[ "$GRANT_SMART" != "true" ]] && grep -q "PULSE_SMARTCTL_PATH=${PRIVILEGE_HELPER_DIR}/" "$UNIT"; then
                GRANT_SMART="true"
            fi
            if [[ "$GRANT_PCT" != "true" ]] && grep -q "PULSE_PCT_PATH=${PRIVILEGE_HELPER_DIR}/" "$UNIT"; then
                GRANT_PCT="true"
            fi
        fi
    fi

    if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
        ensure_runtime_token_file "$PRIVILEGED_HELPER_CREDENTIAL_DIR"
    else
        ensure_runtime_token_file "$STATE_DIR"
    fi
    clear_proxmox_state_if_needed

    if [[ "$PRIVILEGED_HELPER_EXPLICIT" == "true" && "$PRIVILEGED_HELPER_ENABLED" != "true" ]]; then
        teardown_privileged_helper_service
        rm -f "$PRIVILEGED_HELPER_BINARY_PATH"
    fi

    if [[ "$LEAST_PRIVILEGE" == "true" ]]; then
        SERVICE_USER="$LEAST_PRIVILEGE_USER"
        provision_least_privilege_user
        if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
            protect_typed_profile_credentials
            provision_typed_privileged_helper
            if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
                safe_profile_remove_legacy_authority
            fi
            log_info "Typed-helper collector profile: service runs as ${SERVICE_USER}; binaries and credential files remain root-owned while mutable state remains ${SERVICE_USER}-owned."
        else
            provision_privilege_helpers
            log_info "Least-privilege profile: service runs as ${SERVICE_USER}. SMART $( [[ "$GRANT_SMART" == "true" ]] && echo "via scoped sudo helper" || echo "unavailable without --grant-smart" ); Proxmox LXC filesystems $( [[ "$GRANT_PCT" == "true" ]] && echo "via scoped sudo helper" || echo "unavailable without --grant-pct" )."
        fi
    fi

    # Build command line args with --token-file instead of the raw token.
    build_exec_args

    render_systemd_agent_unit "$UNIT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}" "network-online.target docker.service" "network-online.target" "$SERVICE_USER" ""
    # Restrict service file permissions (contains no secrets now, but good practice)
    chmod 644 "$UNIT"

    # Restore SELinux contexts (required for Fedora, RHEL, CentOS)
    restore_selinux_contexts

    restart_systemd_agent_service
    if [[ "$SAFE_PROFILE_ACTION" == "apply" ]]; then
        if ! safe_profile_verify_declared_health; then
            fail "Safe-profile collector did not satisfy local readiness, helper availability, and server registration; restoring the previous profile" "$EXIT_GENERAL"
        fi
        safe_profile_commit_transaction ||
            fail "Safe-profile health passed but its atomic profile record could not be committed; restoring the previous profile" "$EXIT_GENERAL"
    fi
    if [[ "$ACTION_RUNNER_EXPLICIT" == "true" && "$ACTION_RUNNER_ENABLED" != "true" ]]; then
        teardown_action_runner_service
        log_info "Typed action runner removed; collector monitoring remains active."
    elif [[ "$ACTION_RUNNER_ENABLED" == "true" ]]; then
        provision_action_runner
    fi
    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "journalctl -u ${AGENT_NAME} --no-pager -n 20"
    if [[ "$UPGRADE_MODE" != "true" && -n "$PULSE_TOKEN" ]]; then
        if [[ "$PRIVILEGED_HELPER_ENABLED" == "true" ]]; then
            log_info "Token file: ${RUNTIME_TOKEN_FILE} (mode 640, root:${LEAST_PRIVILEGE_USER})"
        else
            log_info "Token file: ${RUNTIME_TOKEN_FILE} (mode 600, root only)"
        fi
    fi
    exit 0
fi

# 6. SysV Init (legacy systems like Asustor, older Debian/RHEL, etc.)
# This is a fallback for systems that have /etc/init.d but no systemd/OpenRC
if [[ -d /etc/init.d ]] && [[ -w /etc/init.d ]]; then
    INITSCRIPT="/etc/init.d/${AGENT_NAME}"
    log_info "Configuring SysV init script at $INITSCRIPT..."

    # Build command line args
    ensure_runtime_token_file "$STATE_DIR"
    clear_proxmox_state_if_needed
    build_exec_args

    # Create SysV init script following LSB conventions
    cat > "$INITSCRIPT" <<'INITEOF'
#!/bin/sh
### BEGIN INIT INFO
# Provides:          pulse-agent
# Required-Start:    $network $remote_fs
# Required-Stop:     $network $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Pulse Unified Agent
# Description:       Pulse monitoring agent for host metrics, Docker, and Kubernetes
### END INIT INFO

# Pulse Unified Agent SysV init script

NAME="pulse-agent"
DAEMON="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
DAEMON_ARGS="EXEC_ARGS_PLACEHOLDER"
PIDFILE="/var/run/${NAME}.pid"
LOGFILE="/var/log/${NAME}.log"
SSL_CERT_FILE_PLACEHOLDER

# Exit if the binary is not installed
[ -x "$DAEMON" ] || exit 0

do_start() {
    if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
        echo "$NAME is already running."
        return 1
    fi
    echo "Starting $NAME..."
    # Start daemon in background, redirect output to log file
    # Use shell backgrounding instead of nohup for broader compatibility (QNAP, etc.)
    $DAEMON $DAEMON_ARGS >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 1
    if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
        echo "$NAME started."
        return 0
    else
        echo "Failed to start $NAME."
        rm -f "$PIDFILE"
        return 1
    fi
}

do_stop() {
    if [ ! -f "$PIDFILE" ]; then
        echo "$NAME is not running (no PID file)."
        return 0
    fi
    PID=$(cat "$PIDFILE")
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "$NAME is not running (stale PID file)."
        rm -f "$PIDFILE"
        return 0
    fi
    echo "Stopping $NAME..."
    kill "$PID"
    # Wait for process to stop
    for i in 1 2 3 4 5; do
        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
        sleep 1
    done
    # Force kill if still running
    if kill -0 "$PID" 2>/dev/null; then
        echo "Force killing $NAME..."
        kill -9 "$PID" 2>/dev/null || true
    fi
    rm -f "$PIDFILE"
    echo "$NAME stopped."
    return 0
}

do_status() {
    if [ -f "$PIDFILE" ]; then
        PID=$(cat "$PIDFILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo "$NAME is running (PID $PID)."
            return 0
        else
            echo "$NAME is not running (stale PID file)."
            return 1
        fi
    else
        echo "$NAME is not running."
        return 3
    fi
}

case "$1" in
    start)
        do_start
        ;;
    stop)
        do_stop
        ;;
    restart|reload|force-reload)
        do_stop
        sleep 1
        do_start
        ;;
    status)
        do_status
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}" >&2
        exit 3
        ;;
esac

exit $?
INITEOF

    # Replace placeholders with actual values
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$INITSCRIPT"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$INITSCRIPT"
    sed -i "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$INITSCRIPT"
    sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SED_EXPORT_LINES}|g" "$INITSCRIPT"

    chmod +x "$INITSCRIPT"

    enable_sysv_agent_service "$INITSCRIPT"

    # Stop existing agent if running
    "$INITSCRIPT" stop 2>/dev/null || true
    sleep 1

    # Start the agent
    "$INITSCRIPT" start
    complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f /var/log/${AGENT_NAME}.log"
    log_info "To check status: $INITSCRIPT status"
    log_info "To view logs: tail -f /var/log/${AGENT_NAME}.log"
    exit 0
fi

fail "Could not detect a supported service manager (systemd, OpenRC, FreeBSD rc.d, SysV init, launchd, or Unraid)."

}

# Call main function with all arguments
main "$@"
