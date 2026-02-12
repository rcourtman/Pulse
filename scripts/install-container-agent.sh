#!/usr/bin/env bash
# Pulse Container Runtime Agent installer
set -euo pipefail

# -----------------------------------------------------------------------------
# Logging helpers (borrowed from legacy installer for consistent UX)
# -----------------------------------------------------------------------------

log_info() {
  printf '[INFO] %s\n' "$1"
}

log_warn() {
  printf '[WARN] %s\n' "$1" >&2
}

log_error() {
  printf '[ERROR] %s\n' "$1" >&2
}

log_success() {
  printf '[ OK ] %s\n' "$1"
}

log_header() {
  printf '\n== %s ==\n' "$1"
}

# -----------------------------------------------------------------------------
# Utility helpers
# -----------------------------------------------------------------------------

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

to_lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

parse_bool() {
  local value
  value="$(to_lower "${1:-}")"
  case "$value" in
    1|true|yes|y|on)
      PARSED_BOOL="true"
      return 0
      ;;
    0|false|no|n|off|"")
      PARSED_BOOL="false"
      return 0
      ;;
  esac
  return 1
}

# -----------------------------------------------------------------------------
# Target parsing helpers (reused from legacy installer)
# -----------------------------------------------------------------------------

declare -a TARGET_SPECS=()
declare -a TARGETS=()
declare -A SEEN_TARGETS

parse_target_spec() {
  local spec="$1"
  local raw_url raw_token raw_insecure

  IFS='|' read -r raw_url raw_token raw_insecure <<< "$spec"
  raw_url=$(trim "$raw_url")
  raw_token=$(trim "$raw_token")
  raw_insecure=$(trim "$raw_insecure")

  if [[ -z "$raw_url" || -z "$raw_token" ]]; then
    log_error "Invalid target spec \"$spec\". Expected format url|token[|insecure]."
    return 1
  fi

  PARSED_TARGET_URL="${raw_url%/}"
  PARSED_TARGET_TOKEN="$raw_token"

  if [[ -n "$raw_insecure" ]]; then
    if parse_bool "$raw_insecure"; then
      PARSED_TARGET_INSECURE="$PARSED_BOOL"
    else
      log_error "Invalid insecure flag \"$raw_insecure\" in target spec \"$spec\"."
      return 1
    fi
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
    local trimmed_entry
    trimmed_entry=$(trim "$entry")
    if [[ -n "$trimmed_entry" ]]; then
      TARGET_SPECS+=("$trimmed_entry")
    fi
  done
}

# -----------------------------------------------------------------------------
# Default paths + globals
# -----------------------------------------------------------------------------

DEFAULT_ROOTFUL_AGENT="/usr/local/bin/pulse-docker-agent"
DEFAULT_ROOTLESS_AGENT="$HOME/.local/bin/pulse-docker-agent"
ROOTFUL_ENV_FILE="/etc/pulse/pulse-docker-agent.env"
ROOTFUL_SERVICE="/etc/systemd/system/pulse-docker-agent.service"
ROOTFUL_LOG="/var/log/pulse-docker-agent.log"
ROOTLESS_ENV_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/pulse/pulse-docker-agent.env"
ROOTLESS_SERVICE="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/pulse-docker-agent.service"
ROOTLESS_LOG="${XDG_STATE_HOME:-$HOME/.local/state}/pulse-docker-agent/pulse-docker-agent.log"

INTERVAL="30s"
UNINSTALL="false"
PURGE="false"
RUNTIME="podman"
RUNTIME_MODE="auto"
ROOTLESS="auto"
CONTAINER_SOCKET_INPUT=""
CONTAINER_SOCKET_PATH=""
CONTAINER_SOCKET_URI=""
PULSE_URL=""
TOKEN=""
PRIMARY_URL=""
PRIMARY_TOKEN=""
PRIMARY_INSECURE="false"
JOINED_TARGETS=""
INSTALLER_URL_HINT=""
NO_AUTO_UPDATE_FLAG=""
DOWNLOAD_ARCH=""
AGENT_PATH_OVERRIDE=""
AGENT_PATH=""
KUBE_INCLUDE_ALL_PODS="false"
KUBE_INCLUDE_ALL_DEPLOYMENTS="false"

PULSE_TARGETS_ENV="${PULSE_TARGETS:-}"
PULSE_RUNTIME_ENV="$(trim "${PULSE_RUNTIME:-}")"
PULSE_RUNTIME_SOCKET_ENV="$(trim "${PULSE_CONTAINER_SOCKET:-${CONTAINER_HOST:-}}")"
PULSE_ROOTLESS_ENV="$(trim "${PULSE_RUNTIME_ROOTLESS:-}")"

ORIGINAL_ARGS=("$@")

# -----------------------------------------------------------------------------
# Argument parsing
# -----------------------------------------------------------------------------

usage() {
  cat <<'EOF'
Pulse Container Agent Installer

Usage:
  install-container-agent.sh [options]

Options:
  --url <url>             Primary Pulse server URL.
  --token <token>         API token for the primary server (legacy mode).
  --target <spec>         Fan-out target spec (url|token[|insecure]); repeatable.
  --interval <duration>   Reporting interval (default 30s).
  --runtime <name>        Container runtime (podman|docker|auto). Default podman.
  --container-socket <s>  Container runtime socket path or unix:// URI.
  --rootless              Force rootless install (user service).
  --system                Force system-wide install (requires root).
  --agent-path <path>     Override binary installation path.
  --kube-include-all-pods Include all non-succeeded pods.
  --kube-include-all-deployments Include all deployments.
  --uninstall             Remove existing installation.
  --purge                 Remove config files when uninstalling.
  --help                  Show this help message.
EOF
}

if [[ $# -eq 0 ]]; then
  : # allow interactive prompts when desired
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --url)
      PULSE_URL="$2"
      shift 2
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --target)
      TARGET_SPECS+=("$2")
      shift 2
      ;;
    --interval)
      INTERVAL="$2"
      shift 2
      ;;
    --runtime)
      RUNTIME="$2"
      shift 2
      ;;
    --runtime=*)
      RUNTIME="${1#--runtime=}"
      shift
      ;;
    --container-socket|--socket)
      CONTAINER_SOCKET_INPUT="$2"
      shift 2
      ;;
    --container-socket=*|--socket=*)
      CONTAINER_SOCKET_INPUT="${1#*=}"
      shift
      ;;
    --rootless)
      ROOTLESS="true"
      shift
      ;;
    --system)
      ROOTLESS="false"
      shift
      ;;
    --agent-path)
      AGENT_PATH_OVERRIDE="$2"
      shift 2
      ;;
    --agent-path=*)
      AGENT_PATH_OVERRIDE="${1#*=}"
      shift
      ;;
    --kube-include-all-pods)
      KUBE_INCLUDE_ALL_PODS="true"
      shift
      ;;
    --kube-include-all-deployments)
      KUBE_INCLUDE_ALL_DEPLOYMENTS="true"
      shift
      ;;
    --uninstall)
      UNINSTALL="true"
      shift
      ;;
    --purge)
      PURGE="true"
      shift
      ;;
    *)
      log_error "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

# -----------------------------------------------------------------------------
# Normalise runtime + mode
# -----------------------------------------------------------------------------

if [[ -n "$PULSE_RUNTIME_ENV" ]]; then
  RUNTIME="$PULSE_RUNTIME_ENV"
fi
RUNTIME="$(to_lower "$(trim "$RUNTIME")")"
if [[ -z "$RUNTIME" || "$RUNTIME" == "auto" ]]; then
  # Prefer docker if explicitly available (compat with legacy usage),
  # otherwise fall back to podman.
  if command -v docker >/dev/null 2>&1; then
    RUNTIME="docker"
  elif command -v podman >/dev/null 2>&1; then
    RUNTIME="podman"
  else
    RUNTIME="podman"
  fi
fi

if [[ "$RUNTIME" == "docker" ]]; then
  # Chain to legacy installer for Docker runtime.
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -f "${SCRIPT_DIR}/install-docker-agent.sh" ]]; then
    exec "${SCRIPT_DIR}/install-docker-agent.sh" "${ORIGINAL_ARGS[@]}"
  fi
  log_error "Docker runtime requested but install-docker-agent.sh not found."
  exit 1
fi

if [[ "$RUNTIME" != "podman" ]]; then
  log_error "Unsupported runtime \"$RUNTIME\". Supported: podman, docker."
  exit 1
fi

# Determine rootless/system preference
if [[ -n "$PULSE_ROOTLESS_ENV" ]]; then
  if parse_bool "$PULSE_ROOTLESS_ENV"; then
    ROOTLESS="$PARSED_BOOL"
  fi
fi

if [[ "$ROOTLESS" == "auto" ]]; then
  if [[ "$EUID" -ne 0 ]]; then
    ROOTLESS="true"
  else
    ROOTLESS="false"
  fi
else
  ROOTLESS="$(to_lower "$ROOTLESS")"
  case "$ROOTLESS" in
    true|false)
      ;;
    *)
      if parse_bool "$ROOTLESS"; then
        ROOTLESS="$PARSED_BOOL"
      else
        ROOTLESS="false"
      fi
      ;;
  esac
fi

# Runtime socket
if [[ -n "$PULSE_RUNTIME_SOCKET_ENV" && -z "$CONTAINER_SOCKET_INPUT" ]]; then
  CONTAINER_SOCKET_INPUT="$PULSE_RUNTIME_SOCKET_ENV"
fi

normalize_socket() {
  local input="$1"
  if [[ "$input" == unix://* ]]; then
    CONTAINER_SOCKET_URI="$input"
    CONTAINER_SOCKET_PATH="${input#unix://}"
  else
    CONTAINER_SOCKET_PATH="$input"
    CONTAINER_SOCKET_URI="unix://$CONTAINER_SOCKET_PATH"
  fi

  if [[ "$CONTAINER_SOCKET_PATH" != /* ]]; then
    log_error "Socket path must be absolute: $CONTAINER_SOCKET_PATH"
    exit 1
  fi
}

if [[ -n "$CONTAINER_SOCKET_INPUT" ]]; then
  normalize_socket "$CONTAINER_SOCKET_INPUT"
else
  if [[ "$ROOTLESS" == "true" ]]; then
    CONTAINER_SOCKET_PATH="/run/user/$UID/podman/podman.sock"
  else
    CONTAINER_SOCKET_PATH="/run/podman/podman.sock"
  fi
  CONTAINER_SOCKET_URI="unix://$CONTAINER_SOCKET_PATH"
fi

# -----------------------------------------------------------------------------
# Common validation helpers
# -----------------------------------------------------------------------------

ensure_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_error "Required command '$cmd' not found in PATH."
    exit 1
  fi
}

ensure_podman_socket() {
  if [[ ! -S "$CONTAINER_SOCKET_PATH" ]]; then
    if [[ "$ROOTLESS" == "true" ]]; then
      log_info "Podman socket not detected at $CONTAINER_SOCKET_PATH"
      log_info "Attempting to enable podman.socket for the current user"
      if command -v systemctl >/dev/null 2>&1; then
        systemctl --user enable --now podman.socket >/dev/null 2>&1 || true
      fi
    else
      log_info "Podman socket not detected at $CONTAINER_SOCKET_PATH"
      log_info "Attempting to enable system podman.socket"
      if command -v systemctl >/dev/null 2>&1; then
        systemctl enable --now podman.socket >/dev/null 2>&1 || true
      fi
    fi
  fi

  if [[ ! -S "$CONTAINER_SOCKET_PATH" ]]; then
    log_warn "Podman API socket not found at $CONTAINER_SOCKET_PATH."
    log_warn "Start it manually with 'podman system service --time=0' or enable podman.socket."
  fi
}

# -----------------------------------------------------------------------------
# Target + token normalisation
# -----------------------------------------------------------------------------

PULSE_URL="$(trim "$PULSE_URL")"
PULSE_URL="${PULSE_URL%/}"
TOKEN="$(trim "$TOKEN")"

split_targets_from_env "$PULSE_TARGETS_ENV"

if [[ -n "$PULSE_URL" && -n "$TOKEN" ]]; then
  TARGET_SPECS+=("${PULSE_URL}|${TOKEN}")
fi

if [[ -n "$PULSE_URL" && -z "$TOKEN" && ${#TARGET_SPECS[@]} -eq 0 ]]; then
  log_error "--token or at least one --target is required when --url is provided."
  exit 1
fi

if [[ ${#TARGET_SPECS[@]} -eq 0 ]]; then
  log_error "No Pulse targets specified. Use --target or --url + --token."
  exit 1
fi

for spec in "${TARGET_SPECS[@]}"; do
  if ! parse_target_spec "$spec"; then
    exit 1
  fi

  local_normalized="${PARSED_TARGET_URL}|${PARSED_TARGET_TOKEN}|${PARSED_TARGET_INSECURE}"
  if [[ -n "${SEEN_TARGETS[$local_normalized]+x}" ]]; then
    continue
  fi
  SEEN_TARGETS[$local_normalized]=1
  TARGETS+=("$local_normalized")
done

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  log_error "No valid Pulse targets after normalisation."
  exit 1
fi

PRIMARY_URL="$(printf '%s' "${TARGETS[0]}" | awk -F'|' '{print $1}')"
PRIMARY_TOKEN="$(printf '%s' "${TARGETS[0]}" | awk -F'|' '{print $2}')"
PRIMARY_INSECURE="$(printf '%s' "${TARGETS[0]}" | awk -F'|' '{print $3}')"

JOINED_TARGETS=$(printf "%s;" "${TARGETS[@]}")
JOINED_TARGETS="${JOINED_TARGETS%;}"

# -----------------------------------------------------------------------------
# Download helpers (shared for both modes)
# -----------------------------------------------------------------------------

determine_arch() {
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)
      DOWNLOAD_ARCH="linux-amd64"
      ;;
    aarch64|arm64)
      DOWNLOAD_ARCH="linux-arm64"
      ;;
    armv7l|armhf|armv7)
      DOWNLOAD_ARCH="linux-armv7"
      ;;
    armv6l)
      DOWNLOAD_ARCH="linux-armv6"
      ;;
    i386|i686)
      DOWNLOAD_ARCH="linux-386"
      ;;
    *)
      DOWNLOAD_ARCH=""
      ;;
  esac
}

download_agent() {
  ensure_command uname
  determine_arch

  if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
    log_error "Neither curl nor wget found in PATH."
    exit 1
  fi

  local download_url="$PRIMARY_URL/download/pulse-docker-agent"
  if [[ -n "$DOWNLOAD_ARCH" ]]; then
    download_url="$download_url?arch=$DOWNLOAD_ARCH"
  fi

  log_info "Fetching agent binary from $download_url"

  local tmp
  tmp=$(mktemp "${AGENT_PATH}.download.XXXXXX")

  local fetched="false"
  local expected_checksum=""
  if command -v wget >/dev/null 2>&1; then
    local wget_headers
    wget_headers=$(mktemp "${AGENT_PATH}.headers.XXXXXX")
    local wget_args=(--server-response -q -O "$tmp" "$download_url")
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
      wget_args=(--no-check-certificate "${wget_args[@]}")
    fi
    if wget "${wget_args[@]}" 2>"$wget_headers"; then
      fetched="true"
      expected_checksum=$(awk 'BEGIN{IGNORECASE=1} /^[[:space:]]*x-checksum-sha256:/ {gsub(/\r/,"",$2); value=$2} END {print value}' "$wget_headers")
    else
      rm -f "$tmp"
    fi
    rm -f "$wget_headers"
  fi

  if [[ "$fetched" != "true" ]]; then
    if command -v curl >/dev/null 2>&1; then
      local curl_headers
      curl_headers=$(mktemp "${AGENT_PATH}.headers.XXXXXX")
      local curl_args=(-fL --progress-bar -D "$curl_headers" -o "$tmp" "$download_url")
      if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        curl_args=(-k "${curl_args[@]}")
      fi
      if curl "${curl_args[@]}"; then
        fetched="true"
        expected_checksum=$(awk 'BEGIN{IGNORECASE=1} /^x-checksum-sha256:/ {gsub(/\r/,"",$2); value=$2} END {print value}' "$curl_headers")
      else
        rm -f "$tmp"
      fi
      rm -f "$curl_headers"
    fi
  fi

  if [[ "$fetched" != "true" ]]; then
    log_error "Failed to download agent binary."
    exit 1
  fi

  # Checksum verification
  if [[ -z "$expected_checksum" ]]; then
    local checksum_url="$PRIMARY_URL/download/pulse-docker-agent.sha256"
    if [[ -n "$DOWNLOAD_ARCH" ]]; then
      checksum_url="$checksum_url?arch=$DOWNLOAD_ARCH"
    fi

    # Fallback for older releases that publish checksum files instead of headers.
    if command -v curl >/dev/null 2>&1; then
      local curl_args=(-fsSL "$checksum_url")
      if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        curl_args=(-k "${curl_args[@]}")
      fi
      expected_checksum=$(curl "${curl_args[@]}" 2>/dev/null || true)
    elif command -v wget >/dev/null 2>&1; then
      local wget_args=(-qO- "$checksum_url")
      if [[ "$PRIMARY_INSECURE" == "true" ]]; then
        wget_args=(--no-check-certificate "${wget_args[@]}")
      fi
      expected_checksum=$(wget "${wget_args[@]}" 2>/dev/null || true)
    fi
  fi

  if [[ -n "$expected_checksum" ]]; then
    log_info "Verifying checksum..."
    local actual_checksum=""
    if command -v sha256sum >/dev/null 2>&1; then
      actual_checksum=$(sha256sum "$tmp" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      actual_checksum=$(shasum -a 256 "$tmp" | awk '{print $1}')
    fi

    if [[ -n "$actual_checksum" ]]; then
      # Support both "hash" and "hash  filename" checksum formats.
      local clean_expected
      clean_expected=$(printf '%s\n' "$expected_checksum" | awk 'NF {print $1; exit}' | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')
      local clean_actual
      clean_actual=$(printf '%s\n' "$actual_checksum" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

      if [[ "$clean_expected" != "$clean_actual" ]]; then
        rm -f "$tmp"
        log_error "Checksum mismatch."
        log_error "  Expected: '$clean_expected'"
        log_error "  Actual:   '$clean_actual'"
        exit 1
      fi
      log_success "Checksum verified"
    else
      log_warn "Unable to calculate local checksum (sha256sum/shasum not found). Skipping verification."
    fi
  else
    log_warn "No checksum metadata returned by server. Skipping verification."
  fi

  mv "$tmp" "$AGENT_PATH"
  chmod 0755 "$AGENT_PATH"
  log_success "Agent installed at $AGENT_PATH"
}

# -----------------------------------------------------------------------------
# Rootful (system) installation helpers
# -----------------------------------------------------------------------------

SERVICE_USER="pulse-docker"
SERVICE_GROUP="pulse-docker"
SERVICE_HOME="/var/lib/pulse-docker-agent"
SERVICE_USER_CREATED="false"
SERVICE_USER_ACTUAL="$SERVICE_USER"
SERVICE_GROUP_ACTUAL="$SERVICE_GROUP"
SYSTEMD_SUPPLEMENTARY_GROUPS_LINE=""

ensure_service_user() {
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    SERVICE_USER_ACTUAL="$SERVICE_USER"
    if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
      SERVICE_GROUP_ACTUAL="$SERVICE_GROUP"
    else
      SERVICE_GROUP_ACTUAL="$(id -gn "$SERVICE_USER")"
    fi
    return
  fi

  if ! command -v useradd >/dev/null 2>&1; then
    log_warn "useradd not available; running service as root"
    SERVICE_USER_ACTUAL="root"
    SERVICE_GROUP_ACTUAL="root"
    return
  fi

  if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    groupadd --system "$SERVICE_GROUP" >/dev/null 2>&1 || true
  fi
  useradd --system --home "$SERVICE_HOME" --shell /usr/sbin/nologin --gid "$SERVICE_GROUP" "$SERVICE_USER" >/dev/null 2>&1 || true
  SERVICE_USER_ACTUAL="$SERVICE_USER"
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    SERVICE_GROUP_ACTUAL="$SERVICE_GROUP"
  else
    SERVICE_GROUP_ACTUAL="$(id -gn "$SERVICE_USER")"
  fi
  SERVICE_USER_CREATED="true"
}

ensure_service_home() {
  if [[ "$SERVICE_USER_ACTUAL" == "root" ]]; then
    return
  fi
  if [[ ! -d "$SERVICE_HOME" ]]; then
    mkdir -p "$SERVICE_HOME"
  fi
  chown "$SERVICE_USER_ACTUAL":"$SERVICE_GROUP_ACTUAL" "$SERVICE_HOME" >/dev/null 2>&1 || true
  chmod 750 "$SERVICE_HOME" >/dev/null 2>&1 || true
}

ensure_podman_group_membership() {
  SYSTEMD_SUPPLEMENTARY_GROUPS_LINE=""
  if [[ "$SERVICE_USER_ACTUAL" == "root" ]]; then
    return
  fi

  local group="podman"
  if ! getent group "$group" >/dev/null 2>&1; then
    log_info "Creating '$group' group for socket access"
    groupadd --system "$group" >/dev/null 2>&1 || {
      log_warn "Failed to create group '$group'; ensure $SERVICE_USER_ACTUAL can access $CONTAINER_SOCKET_PATH"
      return
    }
    # Change socket ownership to the new group if it exists
    if [[ -S "$CONTAINER_SOCKET_PATH" ]]; then
      chgrp "$group" "$CONTAINER_SOCKET_PATH" >/dev/null 2>&1 || log_warn "Failed to change socket group; adjust permissions manually"
    fi
  fi

  if ! id -nG "$SERVICE_USER_ACTUAL" | tr ' ' '\n' | grep -Fxq "$group"; then
    if command -v usermod >/dev/null 2>&1; then
      usermod -a -G "$group" "$SERVICE_USER_ACTUAL" >/dev/null 2>&1 || log_warn "Failed to add $SERVICE_USER_ACTUAL to $group; adjust socket permissions manually."
    else
      log_warn "Unable to modify group memberships; ensure $SERVICE_USER_ACTUAL can access $CONTAINER_SOCKET_PATH."
    fi
  fi

  if id -nG "$SERVICE_USER_ACTUAL" | tr ' ' '\n' | grep -Fxq "$group"; then
    SYSTEMD_SUPPLEMENTARY_GROUPS_LINE="SupplementaryGroups=$group"
  else
    log_warn "Service user $SERVICE_USER_ACTUAL is not in $group; socket access may fail."
  fi
}

write_rootful_env() {
  mkdir -p "$(dirname "$ROOTFUL_ENV_FILE")"
  local tmp
  tmp=$(mktemp "${ROOTFUL_ENV_FILE}.XXXXXX")
  chmod 600 "$tmp"

  {
    printf 'PULSE_URL=%q\n' "$PRIMARY_URL"
    printf 'PULSE_TOKEN=%q\n' "$PRIMARY_TOKEN"
    printf 'PULSE_TARGETS=%q\n' "$JOINED_TARGETS"
    printf 'PULSE_INTERVAL=%q\n' "$INTERVAL"
    printf 'PULSE_RUNTIME=podman\n'
    printf 'CONTAINER_HOST=%q\n' "$CONTAINER_SOCKET_URI"
    printf 'DOCKER_HOST=%q\n' "$CONTAINER_SOCKET_URI"
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
      printf 'PULSE_INSECURE_SKIP_VERIFY=true\n'
    fi
    printf 'PULSE_KUBE_INCLUDE_ALL_PODS=%q\n' "$KUBE_INCLUDE_ALL_PODS"
    printf 'PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS=%q\n' "$KUBE_INCLUDE_ALL_DEPLOYMENTS"
  } > "$tmp"

  mv "$tmp" "$ROOTFUL_ENV_FILE"
  chmod 600 "$ROOTFUL_ENV_FILE"
  log_success "Environment file written to $ROOTFUL_ENV_FILE"
}

install_rootful() {
  if [[ "$EUID" -ne 0 ]]; then
    if command -v sudo >/dev/null 2>&1; then
      exec sudo -- "$0" "${ORIGINAL_ARGS[@]}" --system
    fi
    log_error "Root privileges required for system installation."
    exit 1
  fi

  ensure_command podman
  ensure_podman_socket

  if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
    AGENT_PATH=$(trim "$AGENT_PATH_OVERRIDE")
    if [[ ! "$AGENT_PATH" =~ ^/ ]]; then
      log_error "--agent-path must be absolute."
      exit 1
    fi
  else
    AGENT_PATH="$DEFAULT_ROOTFUL_AGENT"
  fi

  mkdir -p "$(dirname "$AGENT_PATH")"

  download_agent

  ensure_service_user
  ensure_service_home
  ensure_podman_group_membership
  write_rootful_env

  mkdir -p "$(dirname "$ROOTFUL_LOG")"
  if [[ ! -f "$ROOTFUL_LOG" ]]; then
    touch "$ROOTFUL_LOG"
  fi
  chown "$SERVICE_USER_ACTUAL":"$SERVICE_GROUP_ACTUAL" "$ROOTFUL_LOG" >/dev/null 2>&1 || true

  cat > "$ROOTFUL_SERVICE" <<EOF
[Unit]
Description=Pulse Container Agent (Podman)
After=network-online.target podman.socket
Wants=network-online.target podman.socket

[Service]
Type=simple
EnvironmentFile=-$ROOTFUL_ENV_FILE
ExecStart=$AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"
Restart=on-failure
RestartSec=5s
User=$SERVICE_USER_ACTUAL
Group=$SERVICE_GROUP_ACTUAL
$SYSTEMD_SUPPLEMENTARY_GROUPS_LINE
NoNewPrivileges=yes
ProtectSystem=full
ProtectHome=read-only
RestrictSUIDSGID=yes
RestrictRealtime=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
ReadWritePaths=$CONTAINER_SOCKET_PATH
ProtectClock=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes

[Install]
WantedBy=multi-user.target
EOF

  log_success "Systemd unit written to $ROOTFUL_SERVICE"

  systemctl daemon-reload
  systemctl enable pulse-docker-agent
  systemctl restart pulse-docker-agent

  log_header "Installation complete"
  log_info "Runtime socket   : $CONTAINER_SOCKET_PATH"
  log_info "Service user     : $SERVICE_USER_ACTUAL"
  log_info "Agent binary     : $AGENT_PATH"
  log_info "Service status   : systemctl status pulse-docker-agent"
  log_info "Logs             : journalctl -u pulse-docker-agent -f"
}

uninstall_rootful() {
  if [[ "$EUID" -ne 0 ]]; then
    if command -v sudo >/dev/null 2>&1; then
      exec sudo -- "$0" "${ORIGINAL_ARGS[@]}" --system --uninstall
    fi
    log_error "Root privileges required to uninstall system service."
    exit 1
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop pulse-docker-agent 2>/dev/null || true
    systemctl disable pulse-docker-agent 2>/dev/null || true
  fi

  rm -f "$ROOTFUL_SERVICE"
  rm -f "$ROOTFUL_ENV_FILE"
  if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
    AGENT_PATH="$AGENT_PATH_OVERRIDE"
  else
    AGENT_PATH="$DEFAULT_ROOTFUL_AGENT"
  fi
  rm -f "$AGENT_PATH"
  systemctl daemon-reload 2>/dev/null || true

  log_success "System installation removed"
}

# -----------------------------------------------------------------------------
# Rootless helpers
# -----------------------------------------------------------------------------

rootless_env_dir="$(dirname "$ROOTLESS_ENV_FILE")"
rootless_service_dir="$(dirname "$ROOTLESS_SERVICE")"
rootless_log_dir="$(dirname "$ROOTLESS_LOG")"

write_rootless_env() {
  mkdir -p "$rootless_env_dir"
  local tmp
  tmp=$(mktemp "${ROOTLESS_ENV_FILE}.XXXXXX")
  chmod 600 "$tmp"

  {
    printf 'PULSE_URL=%q\n' "$PRIMARY_URL"
    printf 'PULSE_TOKEN=%q\n' "$PRIMARY_TOKEN"
    printf 'PULSE_TARGETS=%q\n' "$JOINED_TARGETS"
    printf 'PULSE_INTERVAL=%q\n' "$INTERVAL"
    printf 'PULSE_RUNTIME=podman\n'
    printf 'CONTAINER_HOST=%q\n' "$CONTAINER_SOCKET_URI"
    printf 'DOCKER_HOST=%q\n' "$CONTAINER_SOCKET_URI"
    if [[ "$PRIMARY_INSECURE" == "true" ]]; then
      printf 'PULSE_INSECURE_SKIP_VERIFY=true\n'
    fi
    printf 'PULSE_KUBE_INCLUDE_ALL_PODS=%q\n' "$KUBE_INCLUDE_ALL_PODS"
    printf 'PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS=%q\n' "$KUBE_INCLUDE_ALL_DEPLOYMENTS"
  } > "$tmp"

  mv "$tmp" "$ROOTLESS_ENV_FILE"
  chmod 600 "$ROOTLESS_ENV_FILE"
  log_success "Environment file written to $ROOTLESS_ENV_FILE"
}

install_rootless() {
  ensure_command podman
  ensure_podman_socket

  if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
    AGENT_PATH=$(trim "$AGENT_PATH_OVERRIDE")
    if [[ ! "$AGENT_PATH" =~ ^/ ]]; then
      log_error "--agent-path must be absolute."
      exit 1
    fi
  else
    AGENT_PATH="$DEFAULT_ROOTLESS_AGENT"
  fi

  mkdir -p "$(dirname "$AGENT_PATH")"
  download_agent

  write_rootless_env

  mkdir -p "$rootless_log_dir"
  touch "$ROOTLESS_LOG"

  mkdir -p "$rootless_service_dir"
  cat > "$ROOTLESS_SERVICE" <<EOF
[Unit]
Description=Pulse Container Agent (Podman rootless)
After=podman.socket
Requires=podman.socket

[Service]
Type=simple
EnvironmentFile=-$ROOTLESS_ENV_FILE
ExecStart=$AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"
Restart=on-failure
RestartSec=5s
ProtectSystem=full
ProtectHome=read-only
RestrictSUIDSGID=yes
RestrictRealtime=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
ReadWritePaths=$CONTAINER_SOCKET_PATH
Environment=STATE_DIR=$rootless_log_dir
Environment=LOG_FILE=$ROOTLESS_LOG

[Install]
WantedBy=default.target
EOF

  log_success "User service unit written to $ROOTLESS_SERVICE"

  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user daemon-reload
    systemctl --user enable --now podman.socket >/dev/null 2>&1 || true
    systemctl --user enable --now pulse-docker-agent
  else
    log_warn "systemctl not available; start the agent manually:"
    log_info "  env \$(grep -v '^#' $ROOTLESS_ENV_FILE | xargs) $AGENT_PATH --runtime podman --interval $INTERVAL"
    return
  fi

  if command -v loginctl >/dev/null 2>&1; then
    if ! loginctl show-user "$USER" | grep -q 'Linger=yes'; then
      log_info "Enable lingering to keep the service alive after logout:"
      log_info "  sudo loginctl enable-linger $USER"
    fi
  fi

  log_header "Rootless installation complete"
  log_info "Agent binary     : $AGENT_PATH"
  log_info "Runtime socket   : $CONTAINER_SOCKET_PATH"
  log_info "Service status   : systemctl --user status pulse-docker-agent"
  log_info "Logs             : journalctl --user -u pulse-docker-agent -f"
}

uninstall_rootless() {
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user stop pulse-docker-agent 2>/dev/null || true
    systemctl --user disable pulse-docker-agent 2>/dev/null || true
  fi

  rm -f "$ROOTLESS_SERVICE"
  rm -f "$ROOTLESS_ENV_FILE"
  if [[ -n "$AGENT_PATH_OVERRIDE" ]]; then
    AGENT_PATH=$(trim "$AGENT_PATH_OVERRIDE")
  else
    AGENT_PATH="$DEFAULT_ROOTLESS_AGENT"
  fi
  rm -f "$AGENT_PATH"

  log_success "Rootless installation removed"
}

# -----------------------------------------------------------------------------
# Entry point
# -----------------------------------------------------------------------------

if [[ "$UNINSTALL" == "true" ]]; then
  if [[ "$ROOTLESS" == "true" ]]; then
    uninstall_rootless
  else
    uninstall_rootful
  fi
  if [[ "$PURGE" == "true" ]]; then
    rm -f "$ROOTFUL_LOG" "$ROOTLESS_LOG" 2>/dev/null || true
  fi
  exit 0
fi

log_header "Pulse Podman Agent Installer"
log_info "Primary Pulse URL : $PRIMARY_URL"
log_info "Runtime            : Podman ($([[ "$ROOTLESS" == "true" ]] && printf 'rootless' || printf 'system'))"
log_info "Runtime socket     : $CONTAINER_SOCKET_PATH"
log_info "Reporting interval : $INTERVAL"

if [[ "$ROOTLESS" == "true" ]]; then
  install_rootless
else
  install_rootful
fi
