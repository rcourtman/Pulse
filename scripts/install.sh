#!/usr/bin/env bash
#
# Pulse Unified Agent Installer
# Supports: Linux (systemd, OpenRC, SysV init), macOS (launchd), FreeBSD (rc.d), Synology DSM (6.x/7+), Unraid
#
# Usage:
#   curl -fsSL http://pulse/install.sh | bash -s -- --url http://pulse --token <token> [options]
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
#   --disk-exclude <pattern>  Exclude mount points matching pattern (repeatable)
#   --insecure          Skip TLS certificate verification
#   --enable-commands   Enable AI command execution on agent (disabled by default)
#   --uninstall         Remove the agent
#
# Auto-Detection:
#   The installer automatically detects Docker, Kubernetes, and Proxmox on the
#   target machine and enables monitoring for detected platforms. Use --disable-*
#   flags to skip specific platforms, or --enable-* to force enable even if not
#   detected.

set -euo pipefail

# Wrap entire script in a function to protect against partial download
# See: https://www.kicksecure.com/wiki/Dev/curl_bash_pipe
main() {

# --- Cleanup trap ---
TMP_FILES=()
# shellcheck disable=SC2317  # Invoked by trap, not directly
cleanup() {
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
UNINSTALL="false"
INSECURE="false"
AGENT_ID=""
HOSTNAME_OVERRIDE=""
ENABLE_COMMANDS="false"
ENROLL="false"
KUBECONFIG_PATH=""  # Path to kubeconfig file for Kubernetes monitoring
KUBE_INCLUDE_ALL_PODS="false"
KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
DISK_EXCLUDES=()  # Array for multiple --disk-exclude values
STATE_DIR="/var/lib/pulse-agent"  # Persistent state directory (overridden per platform)
CURL_CA_BUNDLE="" # Path to CA bundle for curl and agent TLS (sets SSL_CERT_FILE)
NON_INTERACTIVE="false"
TOKEN_FILE_PATH=""       # Path to file containing the token
OUTPUT_FORMAT="text"     # "text" (default) or "json"
PREFLIGHT_ONLY="false"

# Track if flags were explicitly set (to override auto-detection)
DOCKER_EXPLICIT="false"
KUBERNETES_EXPLICIT="false"
PROXMOX_EXPLICIT="false"

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
  --disk-exclude <path>   Exclude mount point (repeatable)
  --insecure              Skip TLS verification
  --cacert <path>         Custom CA certificate for TLS (used by curl and agent)
  --enable-commands       Enable AI command execution
  --enroll                Exchange bootstrap token for runtime token (deploy wizard)
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
        log_info "SELinux context restored"
    else
        # Fallback to chcon if restorecon isn't available
        if command -v chcon >/dev/null 2>&1; then
            log_info "Setting SELinux context for installed binary..."
            chcon -t bin_t "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
        fi
    fi
}

# --- Post-Start Health Verification ---
# After starting the agent service, poll its readiness endpoint to verify it
# actually started. The agent exposes /readyz on :9191 once modules are initialized.
verify_agent_started() {
    local health_url="http://127.0.0.1:9191/readyz"
    local max_iterations=8
    local interval=2
    local iteration=0
    local log_file="${TRUENAS_LOG_FILE:-$LOG_FILE}"

    log_info "Verifying agent started successfully..."

    # Brief pause to let the agent process spawn (especially for background starts like Unraid)
    sleep 2

    while [ $iteration -lt $max_iterations ]; do
        # Check the readiness endpoint first — this is the definitive signal
        if curl -sf --max-time 2 "$health_url" >/dev/null 2>&1; then
            log_info "Agent is running and healthy."
            return 0
        fi

        # If curl failed, check whether the process is still alive.
        # Use pgrep where available, fall back to ps + grep.
        local agent_running=false
        if command -v pgrep >/dev/null 2>&1; then
            # Use -x (exact match) if supported, otherwise fall back to -f
            pgrep -x "${BINARY_NAME}" >/dev/null 2>&1
            local pgrep_rc=$?
            if [ $pgrep_rc -eq 0 ]; then
                agent_running=true
            elif [ $pgrep_rc -ge 2 ]; then
                # Exit code >= 2 means bad option — -x not supported, try -f
                pgrep -f "${BINARY_NAME}" >/dev/null 2>&1 && agent_running=true
            fi
        else
            # shellcheck disable=SC2009
            # Use bracket trick ([p]ulse-agent) to prevent grep from matching itself
            local grep_pattern="[${BINARY_NAME:0:1}]${BINARY_NAME:1}"
            if ps -e -o comm= 2>/dev/null | grep -q "$grep_pattern" || ps aux 2>/dev/null | grep -q "$grep_pattern"; then
                agent_running=true
            fi
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
    return 1
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

detect_proxmox_type() {
    if [[ -d "/etc/proxmox-backup" ]] || command -v proxmox-backup-manager &>/dev/null; then
        echo "pbs"
        return 0
    fi
    if [[ -d "/etc/pve" ]] || command -v pveversion &>/dev/null; then
        echo "pve"
        return 0
    fi
    echo ""
    return 1
}

# Build exec args string for use in service files
# Returns via EXEC_ARGS variable
build_exec_args() {
    EXEC_ARGS="--url ${PULSE_URL} --token ${PULSE_TOKEN} --interval ${INTERVAL}"
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARGS="$EXEC_ARGS --enable-host"
    else
        EXEC_ARGS="$EXEC_ARGS --enable-host=false"
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-docker"; fi
    # Pass explicit false when Docker was explicitly disabled (prevents auto-detection)
    if [[ "$ENABLE_DOCKER" == "false" && "$DOCKER_EXPLICIT" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-docker=false"; fi
    if [[ "$ENABLE_KUBERNETES" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-kubernetes"; fi
    if [[ -n "$KUBECONFIG_PATH" ]]; then EXEC_ARGS="$EXEC_ARGS --kubeconfig ${KUBECONFIG_PATH}"; fi
    if [[ "$ENABLE_PROXMOX" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-proxmox"; fi
    if [[ -n "$PROXMOX_TYPE" ]]; then EXEC_ARGS="$EXEC_ARGS --proxmox-type ${PROXMOX_TYPE}"; fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --insecure"; fi
    if [[ "$ENABLE_COMMANDS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-commands"; fi
    if [[ "$ENROLL" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enroll"; fi
    if [[ "$KUBE_INCLUDE_ALL_PODS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --kube-include-all-pods"; fi
    if [[ "$KUBE_INCLUDE_ALL_DEPLOYMENTS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --kube-include-all-deployments"; fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARGS="$EXEC_ARGS --agent-id ${AGENT_ID}"; fi
    if [[ -n "$HOSTNAME_OVERRIDE" ]]; then EXEC_ARGS="$EXEC_ARGS --hostname ${HOSTNAME_OVERRIDE}"; fi
    if [[ -n "$STATE_DIR" ]]; then EXEC_ARGS="$EXEC_ARGS --state-dir ${STATE_DIR}"; fi
    # Add disk exclude patterns (use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u)
    for pattern in ${DISK_EXCLUDES[@]+"${DISK_EXCLUDES[@]}"}; do
        EXEC_ARGS="$EXEC_ARGS --disk-exclude '${pattern}'"
    done
}

# Build exec args as array for direct execution (proper quoting)
# Returns via EXEC_ARGS_ARRAY variable
build_exec_args_array() {
    EXEC_ARGS_ARRAY=(--url "$PULSE_URL" --token "$PULSE_TOKEN" --interval "$INTERVAL")
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARGS_ARRAY+=(--enable-host)
    else
        EXEC_ARGS_ARRAY+=(--enable-host=false)
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-docker); fi
    # Pass explicit false when Docker was explicitly disabled (prevents auto-detection)
    if [[ "$ENABLE_DOCKER" == "false" && "$DOCKER_EXPLICIT" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-docker=false); fi
    if [[ "$ENABLE_KUBERNETES" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-kubernetes); fi
    if [[ -n "$KUBECONFIG_PATH" ]]; then EXEC_ARGS_ARRAY+=(--kubeconfig "$KUBECONFIG_PATH"); fi
    if [[ "$ENABLE_PROXMOX" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-proxmox); fi
    if [[ -n "$PROXMOX_TYPE" ]]; then EXEC_ARGS_ARRAY+=(--proxmox-type "$PROXMOX_TYPE"); fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARGS_ARRAY+=(--insecure); fi
    if [[ "$ENABLE_COMMANDS" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-commands); fi
    if [[ "$ENROLL" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enroll); fi
    if [[ "$KUBE_INCLUDE_ALL_PODS" == "true" ]]; then EXEC_ARGS_ARRAY+=(--kube-include-all-pods); fi
    if [[ "$KUBE_INCLUDE_ALL_DEPLOYMENTS" == "true" ]]; then EXEC_ARGS_ARRAY+=(--kube-include-all-deployments); fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARGS_ARRAY+=(--agent-id "$AGENT_ID"); fi
    if [[ -n "$HOSTNAME_OVERRIDE" ]]; then EXEC_ARGS_ARRAY+=(--hostname "$HOSTNAME_OVERRIDE"); fi
    if [[ -n "$STATE_DIR" ]]; then EXEC_ARGS_ARRAY+=(--state-dir "$STATE_DIR"); fi
    # Add disk exclude patterns (use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u)
    for pattern in ${DISK_EXCLUDES[@]+"${DISK_EXCLUDES[@]}"}; do
        EXEC_ARGS_ARRAY+=(--disk-exclude "$pattern")
    done
}

# Save install script and connection details for offline uninstall
save_connection_info() {
    local state_dir="$1"
    mkdir -p "$state_dir"
    # Save connection details so uninstall can deregister without --url/--token.
    # Single-quote values to prevent shell interpretation on read-back.
    # PULSE_URL is validated as ^https?:// and PULSE_TOKEN as ^[a-fA-F0-9]+$,
    # so neither can contain single quotes — no inner escaping needed.
    cat > "${state_dir}/connection.env" <<CONNEOF
PULSE_URL='${PULSE_URL}'
PULSE_TOKEN='${PULSE_TOKEN}'
CONNEOF
    chmod 600 "${state_dir}/connection.env"
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
        --interval) INTERVAL="$2"; shift 2 ;;
        --enable-host) ENABLE_HOST="true"; shift ;;
        --disable-host) ENABLE_HOST="false"; shift ;;
        --enable-docker) ENABLE_DOCKER="true"; DOCKER_EXPLICIT="true"; shift ;;
        --disable-docker) ENABLE_DOCKER="false"; DOCKER_EXPLICIT="true"; shift ;;
        --enable-kubernetes) ENABLE_KUBERNETES="true"; KUBERNETES_EXPLICIT="true"; shift ;;
        --disable-kubernetes) ENABLE_KUBERNETES="false"; KUBERNETES_EXPLICIT="true"; shift ;;
        --kubeconfig) KUBECONFIG_PATH="$2"; KUBERNETES_EXPLICIT="true"; ENABLE_KUBERNETES="true"; shift 2 ;;
        --enable-proxmox) ENABLE_PROXMOX="true"; PROXMOX_EXPLICIT="true"; shift ;;
        --disable-proxmox) ENABLE_PROXMOX="false"; PROXMOX_EXPLICIT="true"; shift ;;
        --proxmox-type) PROXMOX_TYPE="$2"; shift 2 ;;
        --insecure) INSECURE="true"; shift ;;
        --cacert) CURL_CA_BUNDLE="$2"; shift 2 ;;
        --enable-commands) ENABLE_COMMANDS="true"; shift ;;
        --enroll) ENROLL="true"; shift ;;
        --uninstall) UNINSTALL="true"; shift ;;
        --agent-id) AGENT_ID="$2"; shift 2 ;;
        --hostname) HOSTNAME_OVERRIDE="$2"; shift 2 ;;
        --kube-include-all-pods) KUBE_INCLUDE_ALL_PODS="true"; shift ;;
        --kube-include-all-deployments) KUBE_INCLUDE_ALL_DEPLOYMENTS="true"; shift ;;
        --disk-exclude) DISK_EXCLUDES+=("$2"); shift 2 ;;
        --non-interactive) NON_INTERACTIVE="true"; shift ;;
        --token-file) TOKEN_FILE_PATH="$2"; shift 2 ;;
        --pulse-url) PULSE_URL="$2"; shift 2 ;;
        --output) OUTPUT_FORMAT="$2"; shift 2 ;;
        --preflight-only) PREFLIGHT_ONLY="true"; shift ;;
        *) fail "Unknown argument: $1" ;;
    esac
done

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

if [[ -n "$PROXMOX_TYPE" && "$PROXMOX_TYPE" != "pve" && "$PROXMOX_TYPE" != "pbs" ]]; then
    fail "Invalid --proxmox-type value: ${PROXMOX_TYPE} (expected 'pve' or 'pbs')"
fi

# --- Check Root ---
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root. Please use sudo." 
   exit 1
fi

# --- URL Normalization ---
# Strip trailing slashes from PULSE_URL to prevent double-slash URLs
# (e.g., http://host:7655//download/... which would match frontend routes)
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

if [[ "$ENABLE_PROXMOX" == "true" && -z "$PROXMOX_TYPE" ]]; then
    auto_type="$(detect_proxmox_type || true)"
    if [[ -n "$auto_type" ]]; then
        PROXMOX_TYPE="$auto_type"
        log_info "Proxmox mode detected: ${PROXMOX_TYPE}"
    fi
fi

# Summary of what will be monitored
log_info "Monitoring configuration:"
log_info "  Agent metrics: $ENABLE_HOST"
log_info "  Docker/Podman: $ENABLE_DOCKER"
log_info "  Kubernetes: $ENABLE_KUBERNETES"
log_info "  Proxmox: $ENABLE_PROXMOX"
if [[ "$ENABLE_PROXMOX" == "true" && -n "$PROXMOX_TYPE" ]]; then
    log_info "  Proxmox type: $PROXMOX_TYPE"
fi

# --- Uninstall Logic ---
if [[ "$UNINSTALL" == "true" ]]; then
    log_info "Uninstalling ${AGENT_NAME} and cleaning up legacy agents..."

    # Recover connection details from saved config if not provided on command line.
    # Parse key=value safely with grep+sed instead of source to avoid shell injection.
    if [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" ]]; then
        local conn_env=""
        if [[ -f /var/lib/pulse-agent/connection.env ]]; then
            conn_env="/var/lib/pulse-agent/connection.env"
        elif [[ -f /boot/config/plugins/pulse-agent/connection.env ]]; then
            conn_env="/boot/config/plugins/pulse-agent/connection.env"
        elif [[ -f "$TRUENAS_STATE_DIR/connection.env" ]]; then
            conn_env="$TRUENAS_STATE_DIR/connection.env"
        fi
        if [[ -n "$conn_env" ]]; then
            log_info "Recovering connection details from ${conn_env}..."
            # Strip surrounding single quotes from values written by save_connection_info.
            # Use || true to prevent set -e from aborting if keys are missing.
            if [[ -z "$PULSE_URL" ]]; then
                PULSE_URL=$(grep '^PULSE_URL=' "$conn_env" 2>/dev/null | head -1 | sed "s/^PULSE_URL=//; s/^'//; s/'$//" || true)
            fi
            if [[ -z "$PULSE_TOKEN" ]]; then
                PULSE_TOKEN=$(grep '^PULSE_TOKEN=' "$conn_env" 2>/dev/null | head -1 | sed "s/^PULSE_TOKEN=//; s/^'//; s/'$//" || true)
            fi
        fi
    fi

    # Try to notify the Pulse server about uninstallation if we have connection details
    # This ensures the agent record is removed and any linked PVE nodes are updated immediately.
    if [[ -n "$PULSE_URL" && -n "$PULSE_TOKEN" ]]; then
        # Try to recover agent ID if not provided.
        # Priority: agent-id file (canonical) > hostname API lookup (fallback)
        if [[ -z "$AGENT_ID" ]]; then
            # Primary: canonical agent-id file
            for aid_path in /var/lib/pulse-agent/agent-id /boot/config/plugins/pulse-agent/agent-id "$TRUENAS_STATE_DIR/agent-id"; do
                if [[ -f "$aid_path" ]]; then
                    AGENT_ID=$(cat "$aid_path")
                    log_info "Recovered agent ID from ${aid_path}"
                    break
                fi
            done
        fi
        if [[ -z "$AGENT_ID" ]]; then
            # API fallback: look up by hostname
            LOOKUP_HOSTNAME=$(hostname 2>/dev/null || true)
            if [[ -n "$LOOKUP_HOSTNAME" ]]; then
                LOOKUP_ARGS=(-fsSL --connect-timeout 5 -H "X-API-Token: ${PULSE_TOKEN}")
                if [[ "$INSECURE" == "true" ]]; then LOOKUP_ARGS+=(-k); fi
                if [[ -n "$CURL_CA_BUNDLE" ]]; then LOOKUP_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi
                LOOKUP_RESP=$(curl "${LOOKUP_ARGS[@]}" "${PULSE_URL}/api/agents/agent/lookup?hostname=${LOOKUP_HOSTNAME}" 2>/dev/null || true)
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
            CURL_ARGS=(-fsSL --connect-timeout 5 -X POST -H "Content-Type: application/json" -H "X-API-Token: ${PULSE_TOKEN}")
            if [[ "$INSECURE" == "true" ]]; then CURL_ARGS+=(-k); fi
            if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

            # Send unregistration request (ignore errors as we are uninstalling anyway)
            curl "${CURL_ARGS[@]}" -d "{\"agentId\": \"${AGENT_ID}\"}" "${PULSE_URL}/api/agents/agent/uninstall" >/dev/null 2>&1 || true
        fi
    fi

    # Kill any running agent processes first.
    # Use -x (exact process name match) to avoid killing THIS uninstall script,
    # whose command line path contains "pulse-agent" (e.g. /boot/config/plugins/pulse-agent/install.sh).
    pkill -x "pulse-agent" 2>/dev/null || true
    # Kill Unraid wrapper scripts — both current (start-pulse-agent.sh) and
    # legacy naming conventions.
    pkill -f "start-pulse-agent.sh" 2>/dev/null || true
    sleep 1

    # Systemd - unified agent
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop "${AGENT_NAME}" 2>/dev/null || true
        systemctl disable "${AGENT_NAME}" 2>/dev/null || true
        rm -f "/etc/systemd/system/${AGENT_NAME}.service"
        
        systemctl daemon-reload 2>/dev/null || true
    fi

    # Remove legacy binaries

    # Remove agent state directory (contains agent ID, proxmox registration state, etc.)
    rm -rf /var/lib/pulse-agent

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
            systemctl stop "${AGENT_NAME}" 2>/dev/null || true
            systemctl disable "${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/systemd/system/${AGENT_NAME}.service"
            systemctl daemon-reload 2>/dev/null || true
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
        # Stop running agents and their wrapper scripts.
        pkill -x "pulse-agent" 2>/dev/null || true
        pkill -f "start-pulse-agent.sh" 2>/dev/null || true
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

    # TrueNAS SCALE/CORE
    if [[ -d "$TRUENAS_STATE_DIR" ]] || [[ -f /etc/truenas-version ]] || [[ -f /etc/version ]]; then
        if [[ "$(uname -s)" == "Linux" ]]; then
            log_info "Removing TrueNAS SCALE installation..."
            systemctl stop "${AGENT_NAME}" 2>/dev/null || true
            systemctl disable "${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/systemd/system/${AGENT_NAME}.service"
            systemctl daemon-reload 2>/dev/null || true
        elif [[ "$(uname -s)" == "FreeBSD" ]]; then
            log_info "Removing TrueNAS CORE installation..."
            service "${AGENT_NAME}" stop 2>/dev/null || true
            rm -f "/usr/local/etc/rc.d/${AGENT_NAME}"
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
        rc-service "${AGENT_NAME}" stop 2>/dev/null || true
        rc-update del "${AGENT_NAME}" default 2>/dev/null || true
        rm -f "/etc/init.d/${AGENT_NAME}"
    fi

    # SysV init (legacy systems like Asustor, older Debian/RHEL, etc.)
    if [[ -f "/etc/init.d/${AGENT_NAME}" ]]; then
        "/etc/init.d/${AGENT_NAME}" stop 2>/dev/null || true
        # Remove using available tools
        if command -v update-rc.d >/dev/null 2>&1; then
            update-rc.d -f "${AGENT_NAME}" remove >/dev/null 2>&1 || true
        elif command -v chkconfig >/dev/null 2>&1; then
            chkconfig "${AGENT_NAME}" off >/dev/null 2>&1 || true
            chkconfig --del "${AGENT_NAME}" >/dev/null 2>&1 || true
        fi
        # Remove rc.d symlinks manually (in case tools weren't available)
        for RL in 0 1 2 3 4 5 6; do
            rm -f "/etc/rc${RL}.d/S99${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/rc${RL}.d/K01${AGENT_NAME}" 2>/dev/null || true
        done
        rm -f "/etc/init.d/${AGENT_NAME}"
        rm -f "/var/run/${AGENT_NAME}.pid"
    fi

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Uninstallation complete."
    exit 0
fi

# --- Validation ---
if [[ -z "$PULSE_URL" ]]; then
    fail "Missing required argument: --url (or --pulse-url)" "$EXIT_MISSING_ARGS"
fi
if [[ -z "$PULSE_TOKEN" ]]; then
    fail "Missing required argument: --token (or --token-file)" "$EXIT_MISSING_ARGS"
fi

# Validate URL format (basic check) - case-insensitive for http:// or https://
# Normalize to lowercase for the check
url_lower=$(echo "$PULSE_URL" | tr '[:upper:]' '[:lower:]')
if [[ ! "$url_lower" =~ ^https?:// ]]; then
    fail "Invalid URL format. Must start with http:// or https://"
fi

# Validate token format (should be hex string, typically 64 chars)
if [[ ! "$PULSE_TOKEN" =~ ^[a-fA-F0-9]+$ ]]; then
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
    json_event "preflight" "arch_ok" "Architecture: ${PF_OS}-${PF_ARCH}"

    # Check 2: Existing agent
    AGENT_STATUS="not_installed"
    if [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        AGENT_STATUS="already_installed"
    elif command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet "${AGENT_NAME}" 2>/dev/null; then
        AGENT_STATUS="already_installed"
    fi
    json_event "preflight" "$AGENT_STATUS" "Agent status: ${AGENT_STATUS}"

    # Check 3: Pulse URL reachability
    PREFLIGHT_EXIT="$EXIT_OK"
    if [[ -n "$PULSE_URL" ]]; then
        CURL_TEST_ARGS=(-sf --connect-timeout 5 -o /dev/null)
        if [[ "$INSECURE" == "true" ]]; then CURL_TEST_ARGS+=(-k); fi
        if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_TEST_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi
        if curl "${CURL_TEST_ARGS[@]}" "${PULSE_URL}/api/health"; then
            json_event "preflight" "pulse_reachable" "Pulse URL reachable"
        else
            json_event "preflight" "pulse_unreachable" "Pulse URL not reachable" "$EXIT_PREFLIGHT_FAILED"
            PREFLIGHT_EXIT="$EXIT_PREFLIGHT_FAILED"
        fi
    fi

    # Output summary
    if [[ "$PREFLIGHT_EXIT" -eq 0 ]]; then
        if [[ "$OUTPUT_FORMAT" == "json" ]]; then
            printf '{"phase":"preflight_complete","code":"ok","message":"Preflight checks passed","exitCode":0,"data":{"arch":"%s-%s","agent_status":"%s"}}\n' \
                "$PF_OS" "$PF_ARCH" "$AGENT_STATUS"
        else
            log_info "Preflight checks passed (arch: ${PF_OS}-${PF_ARCH}, agent: ${AGENT_STATUS})"
        fi
    else
        if [[ "$OUTPUT_FORMAT" == "json" ]]; then
            printf '{"phase":"preflight_complete","code":"failed","message":"Preflight checks failed","exitCode":%d,"data":{"arch":"%s-%s","agent_status":"%s"}}\n' \
                "$PREFLIGHT_EXIT" "$PF_OS" "$PF_ARCH" "$AGENT_STATUS"
        else
            log_error "Preflight checks failed (arch: ${PF_OS}-${PF_ARCH}, agent: ${AGENT_STATUS})"
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

DOWNLOAD_URL="${PULSE_URL}/download/${BINARY_NAME}?arch=${ARCH_PARAM}"
log_info "Downloading agent from ${DOWNLOAD_URL}..."

# Create temp file and register for cleanup
TMP_BIN=$(mktemp)
TMP_FILES+=("$TMP_BIN")

# Build curl arguments as array for proper quoting
CURL_ARGS=(-fsSL --connect-timeout 30 --max-time 300)
if [[ "$INSECURE" == "true" ]]; then CURL_ARGS+=(-k); fi
if [[ -n "$CURL_CA_BUNDLE" ]]; then CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

if ! curl "${CURL_ARGS[@]}" -o "$TMP_BIN" "$DOWNLOAD_URL"; then
    fail "Download failed. Check URL and connectivity." "$EXIT_DOWNLOAD_FAILED"
fi

# Verify downloaded binary
if [[ ! -s "$TMP_BIN" ]]; then
    fail "Downloaded file is empty." "$EXIT_DOWNLOAD_FAILED"
fi

# Check if it's a valid executable (ELF for Linux, Mach-O for macOS)
if [[ "$OS" == "linux" ]]; then
    if ! head -c 4 "$TMP_BIN" | grep -q "ELF"; then
        fail "Downloaded file is not a valid Linux executable." "$EXIT_DOWNLOAD_FAILED"
    fi
elif [[ "$OS" == "darwin" ]]; then
    # Mach-O magic: feedface (32-bit) or feedfacf (64-bit) or cafebabe (universal)
    MAGIC=$(xxd -p -l 4 "$TMP_BIN" 2>/dev/null || head -c 4 "$TMP_BIN" | od -A n -t x1 | tr -d ' ')
    if [[ ! "$MAGIC" =~ ^(cffaedfe|cefaedfe|cafebabe|feedface|feedfacf) ]]; then
        fail "Downloaded file is not a valid macOS executable." "$EXIT_DOWNLOAD_FAILED"
    fi
fi

# Checksum verification (when header is available)
CHECKSUM_URL="${PULSE_URL}/download/${BINARY_NAME}?arch=${ARCH_PARAM}"
TMP_HEADERS=$(mktemp)
TMP_FILES+=("$TMP_HEADERS")
HEADER_CURL_ARGS=(-fsSL --connect-timeout 10 --max-time 30 -D "$TMP_HEADERS" -o /dev/null)
if [[ "$INSECURE" == "true" ]]; then HEADER_CURL_ARGS+=(-k); fi
if [[ -n "$CURL_CA_BUNDLE" ]]; then HEADER_CURL_ARGS+=(--cacert "$CURL_CA_BUNDLE"); fi

EXPECTED_SHA=""
if curl "${HEADER_CURL_ARGS[@]}" "$CHECKSUM_URL" 2>/dev/null; then
    EXPECTED_SHA=$(grep -i '^X-Checksum-Sha256:' "$TMP_HEADERS" 2>/dev/null | tr -d '\r' | awk '{print $2}' || true)
fi

if [[ -n "$EXPECTED_SHA" ]]; then
    ACTUAL_SHA=$(sha256sum "$TMP_BIN" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_BIN" 2>/dev/null | awk '{print $1}')
    if [[ -n "$ACTUAL_SHA" && "$ACTUAL_SHA" != "$EXPECTED_SHA" ]]; then
        fail "Checksum verification failed (expected: ${EXPECTED_SHA:0:16}..., got: ${ACTUAL_SHA:0:16}...)" "$EXIT_CHECKSUM_FAILED"
    fi
    json_event "download" "checksum_ok" "Binary checksum verified"
    log_info "Binary checksum verified"
fi

chmod +x "$TMP_BIN"

# --- Upgrade Detection ---
# Check if pulse-agent is already installed and handle upgrade gracefully
EXISTING_VERSION=""
UPGRADE_MODE=false

if [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    EXISTING_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null | head -1 || echo "unknown")
    NEW_VERSION=$("$TMP_BIN" --version 2>/dev/null | head -1 || echo "unknown")
    
    if [[ -n "$EXISTING_VERSION" && "$EXISTING_VERSION" != "unknown" ]]; then
        UPGRADE_MODE=true
        log_info "Existing installation detected: $EXISTING_VERSION"
        log_info "Upgrading to: $NEW_VERSION"
        
        # Stop the existing agent service gracefully
        if command -v systemctl >/dev/null 2>&1; then
            if systemctl is-active --quiet "${AGENT_NAME}" 2>/dev/null; then
                log_info "Stopping existing ${AGENT_NAME} service..."
                systemctl stop "${AGENT_NAME}" 2>/dev/null || true
                sleep 2
            fi
        elif command -v rc-service >/dev/null 2>&1; then
            if rc-service "${AGENT_NAME}" status >/dev/null 2>&1; then
                log_info "Stopping existing ${AGENT_NAME} service..."
                rc-service "${AGENT_NAME}" stop 2>/dev/null || true
                sleep 2
            fi
        elif command -v service >/dev/null 2>&1; then
            if service "${AGENT_NAME}" status >/dev/null 2>&1; then
                log_info "Stopping existing ${AGENT_NAME} service..."
                service "${AGENT_NAME}" stop 2>/dev/null || true
                sleep 2
            fi
        fi
        
        # Also kill any running process in case it was started manually
        pkill -f "^${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
        sleep 1
    fi
elif command -v systemctl >/dev/null 2>&1 && systemctl is-enabled --quiet "${AGENT_NAME}" 2>/dev/null; then
    # Service exists but binary is missing - reinstall scenario
    log_info "Agent service exists but binary is missing. Reinstalling..."
    systemctl stop "${AGENT_NAME}" 2>/dev/null || true
fi

# Install Binary
log_info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

if [[ "$UPGRADE_MODE" == "true" ]]; then
    log_info "Binary upgraded successfully. Updating service configuration..."
fi

# --- Service Installation ---

# If Proxmox mode is enabled, clear the state files to ensure fresh registration
# This allows re-installation to re-create the Proxmox API tokens
if [[ "$ENABLE_PROXMOX" == "true" ]]; then
    log_info "Clearing Proxmox state for fresh registration..."
    rm -f /var/lib/pulse-agent/proxmox-registered 2>/dev/null || true
    rm -f /var/lib/pulse-agent/proxmox-pve-registered 2>/dev/null || true
    rm -f /var/lib/pulse-agent/proxmox-pbs-registered 2>/dev/null || true
fi

# 1. macOS (Launchd)
if [[ "$OS" == "darwin" ]]; then
    PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
    log_info "Configuring Launchd service at $PLIST..."

    # Build program arguments array
    PLIST_ARGS="        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>--url</string>
        <string>${PULSE_URL}</string>
        <string>--token</string>
        <string>${PULSE_TOKEN}</string>
        <string>--interval</string>
        <string>${INTERVAL}</string>"

    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-host</string>"
    else
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-host=false</string>"
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-docker</string>"
    fi
    if [[ "$ENABLE_KUBERNETES" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-kubernetes</string>"
    fi
    if [[ -n "$KUBECONFIG_PATH" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--kubeconfig</string>
        <string>${KUBECONFIG_PATH}</string>"
    fi
    if [[ "$KUBE_INCLUDE_ALL_PODS" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--kube-include-all-pods</string>"
    fi
    if [[ "$KUBE_INCLUDE_ALL_DEPLOYMENTS" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--kube-include-all-deployments</string>"
    fi
    if [[ "$INSECURE" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--insecure</string>"
    fi
    if [[ "$ENABLE_COMMANDS" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-commands</string>"
    fi
    if [[ "$ENROLL" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enroll</string>"
    fi
    if [[ -n "$AGENT_ID" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--agent-id</string>
        <string>${AGENT_ID}</string>"
    fi
    if [[ -n "$STATE_DIR" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--state-dir</string>
        <string>${STATE_DIR}</string>"
    fi
    # Add disk exclude patterns (use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u)
    for pattern in ${DISK_EXCLUDES[@]+"${DISK_EXCLUDES[@]}"}; do
        PLIST_ARGS="${PLIST_ARGS}
        <string>--disk-exclude</string>
        <string>${pattern}</string>"
    done

    PLIST_ENV=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        PLIST_ENV="
    <key>EnvironmentVariables</key>
    <dict>
        <key>${SSL_CERT_ENV_NAME}</key>
        <string>${SSL_CERT_ENV_VALUE}</string>
    </dict>"
    fi

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
    </array>${PLIST_ENV}
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
    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f $LOG_FILE"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 2. Synology DSM
# DSM 7+ uses systemd, DSM 6.x uses upstart
if [[ -d /usr/syno ]] && [[ -f /etc/VERSION ]]; then
    # Extract major version from /etc/VERSION
    DSM_MAJOR=$(grep 'majorversion=' /etc/VERSION | cut -d'"' -f2)
    log_info "Detected Synology DSM ${DSM_MAJOR}..."

    # Build command line args
    build_exec_args

    if [[ "$DSM_MAJOR" -ge 7 ]]; then
        # DSM 7+ uses systemd
        UNIT="/etc/systemd/system/${AGENT_NAME}.service"
        log_info "Configuring systemd service at $UNIT (DSM 7+)..."

        SYSTEMD_ENV=""
        if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
            SYSTEMD_ENV=$'\n'"Environment=${SSL_CERT_ENV_NAME}=${SSL_CERT_ENV_VALUE}"
        fi

        cat > "$UNIT" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS}${SYSTEMD_ENV}
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable "${AGENT_NAME}"
        systemctl restart "${AGENT_NAME}"
    else
        # DSM 6.x uses upstart
        CONF="/etc/init/${AGENT_NAME}.conf"
        log_info "Configuring Upstart service at $CONF (DSM 6.x)..."

        UPSTART_ENV=""
        if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
            UPSTART_ENV=$'\n'"env ${SSL_CERT_ENV_NAME}=${SSL_CERT_ENV_VALUE}"
        fi

        cat > "$CONF" <<EOF
description "Pulse Unified Agent"
author "Pulse"

start on syno.network.ready
stop on runlevel [06]

respawn
respawn limit 5 10${UPSTART_ENV}

exec ${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS} >> ${LOG_FILE} 2>&1
EOF
        initctl stop "${AGENT_NAME}" 2>/dev/null || true
        initctl start "${AGENT_NAME}"
    fi

    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f $LOG_FILE"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 3. Unraid (no init system - use /boot/config/go script)
# Detect Unraid by /etc/unraid-version (preferred) or /boot/config/go with unraid markers
if [[ -f /etc/unraid-version ]]; then
    log_info "Detected Unraid system..."

    # Unraid's /boot is FAT32 (no execute permission), so we store the binary there
    # for persistence but copy it to RAM disk (/usr/local/bin) for execution
    UNRAID_STORAGE_DIR="/boot/config/plugins/pulse-agent"
    UNRAID_STORED_BINARY="${UNRAID_STORAGE_DIR}/${BINARY_NAME}"
    RUNTIME_BINARY="${INSTALL_DIR}/${BINARY_NAME}"
    GO_SCRIPT="/boot/config/go"

    STATE_DIR="$UNRAID_STORAGE_DIR"
    mkdir -p "$UNRAID_STORAGE_DIR"

    # Copy binary to persistent storage (for survival across reboots)
    cp "${RUNTIME_BINARY}" "$UNRAID_STORED_BINARY"
    # Keep binary in /usr/local/bin (RAM disk) with execute permission for runtime
    chmod +x "${RUNTIME_BINARY}"

    log_info "Installed binary to ${UNRAID_STORED_BINARY} (persistent) and ${RUNTIME_BINARY} (runtime)..."

    # Build command line args (string for wrapper script, array for direct execution)
    build_exec_args
    build_exec_args_array

    # Kill any existing pulse agents.
    log_info "Stopping any existing pulse agents..."
    # Use process name matching to avoid killing unrelated processes
    pkill -f "^${RUNTIME_BINARY}" 2>/dev/null || true
    sleep 2

    # Create a wrapper script that will be called from /boot/config/go
    # This script copies from persistent storage to RAM disk on boot, then starts the agent
    EXPORT_SSL_CERT_FILE=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        EXPORT_SSL_CERT_FILE=$'\n'"export ${SSL_CERT_ENV_NAME}=\"${SSL_CERT_ENV_VALUE}\""
    fi

    WRAPPER_SCRIPT="${UNRAID_STORAGE_DIR}/start-pulse-agent.sh"
    cat > "$WRAPPER_SCRIPT" <<EOF
#!/bin/bash
# Pulse Agent startup script for Unraid
# Auto-generated by Pulse installer
# Includes watchdog loop to restart agent on failure

# Kill any existing pulse-agent processes
pkill -f "^${RUNTIME_BINARY}" 2>/dev/null || true
sleep 2

# Copy binary from persistent storage to RAM disk (needed after reboot)
cp "${UNRAID_STORED_BINARY}" "${RUNTIME_BINARY}"
chmod +x "${RUNTIME_BINARY}"${EXPORT_SSL_CERT_FILE}

# Watchdog loop: restart agent if it exits
# Uses exponential backoff to prevent rapid restart loops
RESTART_DELAY=5
MAX_RESTART_DELAY=60

while true; do
    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] Starting pulse-agent..." >> /var/log/${AGENT_NAME}.log
    ${RUNTIME_BINARY} ${EXEC_ARGS} >> /var/log/${AGENT_NAME}.log 2>&1
    EXIT_CODE=\$?
    
    echo "\$(date '+%Y-%m-%d %H:%M:%S') [watchdog] pulse-agent exited with code \$EXIT_CODE, restarting in \${RESTART_DELAY}s..." >> /var/log/${AGENT_NAME}.log
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

    save_connection_info "$UNRAID_STORAGE_DIR"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent is running."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f /var/log/${AGENT_NAME}.log"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    log_info "The agent will start automatically on boot."
    log_info "To check status: pgrep -a pulse-agent"
    log_info "To view logs: tail -f /var/log/${AGENT_NAME}.log"
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 4. TrueNAS SCALE/CORE (immutable root, uses systemd on SCALE and rc.d on CORE)
# TrueNAS can wipe service registration files on upgrades, so we store the service
# in /data and create an Init/Shutdown task to recreate the symlink on boot.
# Note: /data may have exec=off on some TrueNAS systems. We try multiple runtime locations.
if [[ "$TRUENAS" == true ]]; then
    log_info "Configuring TrueNAS SCALE/CORE installation..."
    STATE_DIR="$TRUENAS_STATE_DIR"

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
    # Kill any remaining pulse-agent processes (may be running from different paths)
    pkill -9 -f "pulse-agent" 2>/dev/null || true
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
    build_exec_args

    # Store service file in /data (persists across upgrades)
    TRUENAS_SERVICE_STORAGE="$TRUENAS_STATE_DIR/${AGENT_NAME}.service"

    if [[ "$(uname -s)" == "Linux" ]]; then
        TRUENAS_LOG_TARGET="$LOG_FILE"
        if [[ -n "$TRUENAS_LOG_FILE" ]]; then
            TRUENAS_LOG_TARGET="$TRUENAS_LOG_FILE"
        fi

        SYSTEMD_ENV=""
        if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
            SYSTEMD_ENV=$'\n'"Environment=${SSL_CERT_ENV_NAME}=${SSL_CERT_ENV_VALUE}"
        fi

        cat > "$TRUENAS_SERVICE_STORAGE" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${TRUENAS_RUNTIME_BINARY} ${EXEC_ARGS}${SYSTEMD_ENV}
Restart=always
RestartSec=5s
User=root
StandardOutput=append:${TRUENAS_LOG_TARGET}
StandardError=append:${TRUENAS_LOG_TARGET}

[Install]
WantedBy=multi-user.target
EOF
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        cat > "$TRUENAS_SERVICE_STORAGE" <<'RCEOF'
#!/bin/sh

# PROVIDE: pulse_agent
# REQUIRE: LOGIN NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pulse_agent"
rcvar="pulse_agent_enable"
pidfile="/var/run/${name}.pid"

command="RUNTIME_BINARY_PLACEHOLDER"
command_args="EXEC_ARGS_PLACEHOLDER"

start_cmd="${name}_start"
stop_cmd="${name}_stop"
status_cmd="${name}_status"

pulse_agent_start()
{
    if checkyesno ${rcvar}; then
        echo "Starting ${name}."
        SSL_CERT_FILE_PLACEHOLDER
        /usr/sbin/daemon -r -p ${pidfile} -f ${command} ${command_args}
    fi
}

pulse_agent_stop()
{
    if [ -f ${pidfile} ]; then
        echo "Stopping ${name}."
        kill $(cat ${pidfile}) 2>/dev/null
        rm -f ${pidfile}
    else
        echo "${name} is not running."
    fi
}

pulse_agent_status()
{
    if [ -f ${pidfile} ] && kill -0 $(cat ${pidfile}) 2>/dev/null; then
        echo "${name} is running as pid $(cat ${pidfile})."
    else
        echo "${name} is not running."
        return 1
    fi
}

load_rc_config $name
run_rc_command "$1"
RCEOF

        sed -i '' "s|RUNTIME_BINARY_PLACEHOLDER|${TRUENAS_RUNTIME_BINARY}|g" "$TRUENAS_SERVICE_STORAGE" 2>/dev/null || \
            sed -i "s|RUNTIME_BINARY_PLACEHOLDER|${TRUENAS_RUNTIME_BINARY}|g" "$TRUENAS_SERVICE_STORAGE"
        sed -i '' "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$TRUENAS_SERVICE_STORAGE" 2>/dev/null || \
            sed -i "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$TRUENAS_SERVICE_STORAGE"
        SSL_CERT_LINE=""
        if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
            SSL_CERT_LINE="export ${SSL_CERT_ENV_NAME}=\"${SSL_CERT_ENV_VALUE}\""
        fi
        sed -i '' "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$TRUENAS_SERVICE_STORAGE" 2>/dev/null || \
            sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$TRUENAS_SERVICE_STORAGE"

        chmod +x "$TRUENAS_SERVICE_STORAGE"
    fi

    # Store environment/config for reference
    cat > "$TRUENAS_ENV_FILE" <<EOF
# Pulse Agent configuration (for reference)
PULSE_URL=${PULSE_URL}
PULSE_TOKEN=${PULSE_TOKEN}
PULSE_INTERVAL=${INTERVAL}
PULSE_ENABLE_HOST=${ENABLE_HOST}
PULSE_ENABLE_DOCKER=${ENABLE_DOCKER}
PULSE_ENABLE_KUBERNETES=${ENABLE_KUBERNETES}
PULSE_KUBE_INCLUDE_ALL_PODS=${KUBE_INCLUDE_ALL_PODS}
PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS=${KUBE_INCLUDE_ALL_DEPLOYMENTS}
EOF
    chmod 600 "$TRUENAS_ENV_FILE"

    # Create bootstrap script that runs on boot
    # This script handles the runtime binary location and recreates the systemd/rc.d symlink
    if [[ "$(uname -s)" == "Linux" ]]; then
        cat > "$TRUENAS_BOOTSTRAP_SCRIPT" <<'BOOTSTRAP'
#!/bin/bash
# Pulse Agent Bootstrap for TrueNAS SCALE
# This script is called by TrueNAS Init/Shutdown task on boot.
# It ensures the binary is in an executable location and recreates the
# systemd symlink (which is wiped on TrueNAS upgrades).

set -e

SERVICE_NAME="pulse-agent"
STATE_DIR="STATE_DIR_PLACEHOLDER"
STORED_BINARY="${STATE_DIR}/pulse-agent"
RUNTIME_BINARY="RUNTIME_BINARY_PLACEHOLDER"
SERVICE_STORAGE="${STATE_DIR}/pulse-agent.service"
SYSTEMD_LINK="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ ! -f "$STORED_BINARY" ]]; then
    echo "ERROR: Binary not found at $STORED_BINARY"
    exit 1
fi

if [[ ! -f "$SERVICE_STORAGE" ]]; then
    echo "ERROR: Service file not found at $SERVICE_STORAGE"
    exit 1
fi

# If runtime binary is different from stored binary, copy it
if [[ "$RUNTIME_BINARY" != "$STORED_BINARY" ]]; then
    # Ensure parent directory exists (e.g., /root/bin)
    mkdir -p "$(dirname "$RUNTIME_BINARY")" 2>/dev/null || true
    cp "$STORED_BINARY" "$RUNTIME_BINARY"
    chmod +x "$RUNTIME_BINARY"
fi

# Create symlink (or update if exists)
ln -sf "$SERVICE_STORAGE" "$SYSTEMD_LINK"

# Reload and start
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" 2>/dev/null || true
systemctl restart "$SERVICE_NAME"

echo "Pulse agent started successfully"
BOOTSTRAP

    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        cat > "$TRUENAS_BOOTSTRAP_SCRIPT" <<'BOOTSTRAP'
#!/bin/bash
# Pulse Agent Bootstrap for TrueNAS CORE
# Called by TrueNAS Init/Shutdown task on boot.

set -e

SERVICE_NAME="pulse-agent"
STATE_DIR="STATE_DIR_PLACEHOLDER"
STORED_BINARY="${STATE_DIR}/pulse-agent"
RUNTIME_BINARY="RUNTIME_BINARY_PLACEHOLDER"
TRUENAS_SERVICE_STORAGE="${STATE_DIR}/pulse-agent.service"
RCSCRIPT_LINK="/usr/local/etc/rc.d/${SERVICE_NAME}"

if [[ ! -f "$STORED_BINARY" ]]; then
    echo "ERROR: Binary not found at $STORED_BINARY"
    exit 1
fi

if [[ ! -f "$TRUENAS_SERVICE_STORAGE" ]]; then
    echo "ERROR: Service file not found at $TRUENAS_SERVICE_STORAGE"
    exit 1
fi

if [[ "$RUNTIME_BINARY" != "$STORED_BINARY" ]]; then
    mkdir -p "$(dirname "$RUNTIME_BINARY")" 2>/dev/null || true
    cp "$STORED_BINARY" "$RUNTIME_BINARY"
    chmod +x "$RUNTIME_BINARY"
fi

ln -sf "$TRUENAS_SERVICE_STORAGE" "$RCSCRIPT_LINK"

if ! grep -q "pulse_agent_enable" /etc/rc.conf 2>/dev/null; then
    echo 'pulse_agent_enable="YES"' >> /etc/rc.conf
else
    sed -i '' 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf 2>/dev/null || \
        sed -i 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf
fi

service "${SERVICE_NAME}" stop 2>/dev/null || true
sleep 1
service "${SERVICE_NAME}" start 2>/dev/null || true

echo "Pulse agent started successfully"
BOOTSTRAP
    fi

    sed -i '' "s|STATE_DIR_PLACEHOLDER|${TRUENAS_STATE_DIR}|g" "$TRUENAS_BOOTSTRAP_SCRIPT" 2>/dev/null || \
        sed -i "s|STATE_DIR_PLACEHOLDER|${TRUENAS_STATE_DIR}|g" "$TRUENAS_BOOTSTRAP_SCRIPT"
    sed -i '' "s|RUNTIME_BINARY_PLACEHOLDER|${TRUENAS_RUNTIME_BINARY}|g" "$TRUENAS_BOOTSTRAP_SCRIPT" 2>/dev/null || \
        sed -i "s|RUNTIME_BINARY_PLACEHOLDER|${TRUENAS_RUNTIME_BINARY}|g" "$TRUENAS_BOOTSTRAP_SCRIPT"
    chmod +x "$TRUENAS_BOOTSTRAP_SCRIPT"

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
        systemctl daemon-reload
        systemctl enable "${AGENT_NAME}" 2>/dev/null || true
        systemctl restart "${AGENT_NAME}"
    elif [[ "$(uname -s)" == "FreeBSD" ]]; then
        if ! grep -q "pulse_agent_enable" /etc/rc.conf 2>/dev/null; then
            echo 'pulse_agent_enable="YES"' >> /etc/rc.conf
        else
            sed -i '' 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf 2>/dev/null || \
                sed -i 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf
        fi

        service "${AGENT_NAME}" stop 2>/dev/null || true
        sleep 1
        service "${AGENT_NAME}" start 2>/dev/null || true
    fi

    save_connection_info "$TRUENAS_STATE_DIR"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent is running."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
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
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 5. OpenRC (Alpine, Gentoo, Artix, etc.)
# Check for rc-service but make sure we're not on a systemd system that happens to have it
if command -v rc-service >/dev/null 2>&1 && [[ -d /etc/init.d ]] && ! command -v systemctl >/dev/null 2>&1; then
    INITSCRIPT="/etc/init.d/${AGENT_NAME}"
    log_info "Configuring OpenRC service at $INITSCRIPT..."

    # Build command line args
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
    SSL_CERT_LINE=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        SSL_CERT_LINE="export ${SSL_CERT_ENV_NAME}=\"${SSL_CERT_ENV_VALUE}\""
    fi
    sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$INITSCRIPT"

    chmod +x "$INITSCRIPT"
    rc-service "${AGENT_NAME}" stop 2>/dev/null || true
    rc-update add "${AGENT_NAME}" default 2>/dev/null || true
    rc-service "${AGENT_NAME}" start
    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f $LOG_FILE"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 5b. FreeBSD rc.d (OPNsense, pfSense, vanilla FreeBSD)
if [[ "$OS" == "freebsd" ]] || [[ -f /etc/rc.subr ]]; then
    RCSCRIPT="/usr/local/etc/rc.d/${AGENT_NAME}"
    log_info "Configuring FreeBSD rc.d service at $RCSCRIPT..."

    # Build command line args
    build_exec_args

    # Create FreeBSD rc.d script following FreeBSD conventions
    cat > "$RCSCRIPT" <<'RCEOF'
#!/bin/sh

# PROVIDE: pulse_agent
# REQUIRE: LOGIN NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pulse_agent"
rcvar="pulse_agent_enable"
pidfile="/var/run/${name}.pid"

# These placeholders are replaced by sed below
command="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
command_args="EXEC_ARGS_PLACEHOLDER"

start_cmd="${name}_start"
stop_cmd="${name}_stop"
status_cmd="${name}_status"

pulse_agent_start()
{
    if checkyesno ${rcvar}; then
        echo "Starting ${name}."
        SSL_CERT_FILE_PLACEHOLDER
        /usr/sbin/daemon -r -p ${pidfile} -f ${command} ${command_args}
    fi
}

pulse_agent_stop()
{
    if [ -f ${pidfile} ]; then
        echo "Stopping ${name}."
        kill $(cat ${pidfile}) 2>/dev/null
        rm -f ${pidfile}
    else
        echo "${name} is not running."
    fi
}

pulse_agent_status()
{
    if [ -f ${pidfile} ] && kill -0 $(cat ${pidfile}) 2>/dev/null; then
        echo "${name} is running as pid $(cat ${pidfile})."
    else
        echo "${name} is not running."
        return 1
    fi
}

load_rc_config $name
run_rc_command "$1"
RCEOF

    # Replace placeholders with actual values
    sed -i '' "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$RCSCRIPT" 2>/dev/null || \
        sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$RCSCRIPT"
    sed -i '' "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$RCSCRIPT" 2>/dev/null || \
        sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$RCSCRIPT"
    sed -i '' "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$RCSCRIPT" 2>/dev/null || \
        sed -i "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$RCSCRIPT"
    SSL_CERT_LINE=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        SSL_CERT_LINE="export ${SSL_CERT_ENV_NAME}=\"${SSL_CERT_ENV_VALUE}\""
    fi
    sed -i '' "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$RCSCRIPT" 2>/dev/null || \
        sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$RCSCRIPT"

    chmod +x "$RCSCRIPT"

    # Enable the service in rc.conf
    if ! grep -q "pulse_agent_enable" /etc/rc.conf 2>/dev/null; then
        echo 'pulse_agent_enable="YES"' >> /etc/rc.conf
    else
        sed -i '' 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf 2>/dev/null || \
            sed -i 's/pulse_agent_enable=.*/pulse_agent_enable="YES"/' /etc/rc.conf
    fi

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
    "$RCSCRIPT" stop 2>/dev/null || true
    sleep 1

    # Start the agent
    "$RCSCRIPT" start
    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f /var/log/messages"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    log_info "To check status: $RCSCRIPT status"
    log_info "To view logs: tail -f /var/log/messages"
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 5. Linux (Systemd)
if command -v systemctl >/dev/null 2>&1; then
    UNIT="/etc/systemd/system/${AGENT_NAME}.service"
    TOKEN_DIR="/var/lib/pulse-agent"
    TOKEN_FILE="${TOKEN_DIR}/token"
    log_info "Configuring Systemd service at $UNIT..."

    # Write token to secure file (not visible in ps or service file)
    mkdir -p "$TOKEN_DIR"
    echo -n "$PULSE_TOKEN" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
    chown root:root "$TOKEN_FILE"
    log_info "Token stored securely at $TOKEN_FILE (mode 600)"

    # Build command line args WITHOUT the token (token is read from file)
    EXEC_ARGS="--url ${PULSE_URL} --interval ${INTERVAL}"
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARGS="$EXEC_ARGS --enable-host"
    else
        EXEC_ARGS="$EXEC_ARGS --enable-host=false"
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-docker"; fi
    # Pass explicit false when Docker was explicitly disabled (prevents auto-detection)
    if [[ "$ENABLE_DOCKER" == "false" && "$DOCKER_EXPLICIT" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-docker=false"; fi
    if [[ "$ENABLE_KUBERNETES" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-kubernetes"; fi
    if [[ -n "$KUBECONFIG_PATH" ]]; then EXEC_ARGS="$EXEC_ARGS --kubeconfig ${KUBECONFIG_PATH}"; fi
    if [[ "$ENABLE_PROXMOX" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-proxmox"; fi
    if [[ -n "$PROXMOX_TYPE" ]]; then EXEC_ARGS="$EXEC_ARGS --proxmox-type ${PROXMOX_TYPE}"; fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --insecure"; fi
    if [[ "$ENABLE_COMMANDS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-commands"; fi
    if [[ "$ENROLL" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enroll"; fi
    if [[ "$KUBE_INCLUDE_ALL_PODS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --kube-include-all-pods"; fi
    if [[ "$KUBE_INCLUDE_ALL_DEPLOYMENTS" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --kube-include-all-deployments"; fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARGS="$EXEC_ARGS --agent-id ${AGENT_ID}"; fi
    if [[ -n "$HOSTNAME_OVERRIDE" ]]; then EXEC_ARGS="$EXEC_ARGS --hostname ${HOSTNAME_OVERRIDE}"; fi
    if [[ -n "$STATE_DIR" ]]; then EXEC_ARGS="$EXEC_ARGS --state-dir ${STATE_DIR}"; fi
    # Add disk exclude patterns (use ${arr[@]+"${arr[@]}"} for bash 3.2 compatibility with set -u)
    for pattern in ${DISK_EXCLUDES[@]+"${DISK_EXCLUDES[@]}"}; do
        EXEC_ARGS="$EXEC_ARGS --disk-exclude '${pattern}'"
    done

    SYSTEMD_ENV=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        SYSTEMD_ENV=$'\n'"Environment=${SSL_CERT_ENV_NAME}=${SSL_CERT_ENV_VALUE}"
    fi

    cat > "$UNIT" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS}${SYSTEMD_ENV}
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF
    # Restrict service file permissions (contains no secrets now, but good practice)
    chmod 644 "$UNIT"

    # Restore SELinux contexts (required for Fedora, RHEL, CentOS)
    restore_selinux_contexts

    systemctl daemon-reload
    systemctl enable "${AGENT_NAME}"
    systemctl restart "${AGENT_NAME}"
    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            log_info "Token file: $TOKEN_FILE (mode 600, root only)"
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: journalctl -u ${AGENT_NAME} --no-pager -n 20"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

# 6. SysV Init (legacy systems like Asustor, older Debian/RHEL, etc.)
# This is a fallback for systems that have /etc/init.d but no systemd/OpenRC
if [[ -d /etc/init.d ]] && [[ -w /etc/init.d ]]; then
    INITSCRIPT="/etc/init.d/${AGENT_NAME}"
    log_info "Configuring SysV init script at $INITSCRIPT..."

    # Build command line args
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
    SSL_CERT_LINE=""
    if [[ -n "$SSL_CERT_ENV_NAME" ]]; then
        SSL_CERT_LINE="export ${SSL_CERT_ENV_NAME}=\"${SSL_CERT_ENV_VALUE}\""
    fi
    sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SSL_CERT_LINE}|g" "$INITSCRIPT"

    chmod +x "$INITSCRIPT"

    # Try to enable on boot using available tools
    if command -v update-rc.d >/dev/null 2>&1; then
        # Debian-based systems
        update-rc.d "${AGENT_NAME}" defaults >/dev/null 2>&1 || true
        log_info "Enabled service with update-rc.d."
    elif command -v chkconfig >/dev/null 2>&1; then
        # RHEL-based systems
        chkconfig --add "${AGENT_NAME}" >/dev/null 2>&1 || true
        chkconfig "${AGENT_NAME}" on >/dev/null 2>&1 || true
        log_info "Enabled service with chkconfig."
    else
        # Manual symlink creation for systems without tools
        # Try to create rc.d symlinks manually
        for RL in 2 3 4 5; do
            if [[ -d "/etc/rc${RL}.d" ]]; then
                ln -sf "$INITSCRIPT" "/etc/rc${RL}.d/S99${AGENT_NAME}" 2>/dev/null || true
            fi
        done
        for RL in 0 1 6; do
            if [[ -d "/etc/rc${RL}.d" ]]; then
                ln -sf "$INITSCRIPT" "/etc/rc${RL}.d/K01${AGENT_NAME}" 2>/dev/null || true
            fi
        done
        log_info "Created rc.d symlinks manually."
    fi

    # Stop existing agent if running
    "$INITSCRIPT" stop 2>/dev/null || true
    sleep 1

    # Start the agent
    "$INITSCRIPT" start
    save_connection_info "/var/lib/pulse-agent"
    if verify_agent_started; then
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_info "Upgrade complete! Agent restarted with new configuration."
            json_event "complete" "updated" "Installation updated"
        else
            log_info "Installation complete! Agent is running."
            json_event "complete" "installed" "Installation installed"
        fi
    else
        if [[ "$UPGRADE_MODE" == "true" ]]; then
            log_warn "Upgrade complete, but the agent may not be running correctly."
            json_event "complete" "updated_unhealthy" "Agent updated but not responding"
        else
            log_warn "Installation complete, but the agent may not be running correctly."
            log_warn "Check logs: tail -f /var/log/${AGENT_NAME}.log"
            json_event "complete" "installed_unhealthy" "Agent installed but not responding"
        fi
    fi
    log_info "To check status: $INITSCRIPT status"
    log_info "To view logs: tail -f /var/log/${AGENT_NAME}.log"
    if [[ -n "$SAVED_INSTALL_SCRIPT" ]]; then log_info "To uninstall later: sudo bash ${SAVED_INSTALL_SCRIPT} --uninstall"; fi
    exit 0
fi

fail "Could not detect a supported service manager (systemd, OpenRC, FreeBSD rc.d, SysV init, launchd, or Unraid)."

}

# Call main function with all arguments
main "$@"
