#!/bin/bash
set -e

trim() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "$value"
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
#   curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- --uninstall

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

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "Error: This script must be run as root"
   echo ""
   echo "Please run the command with 'bash' instead of just piping to bash:"
   echo "  Install: curl -fsSL <URL>/install-docker-agent.sh | bash -s -- --url <URL>"
   echo "  Uninstall: curl -fsSL <URL>/install-docker-agent.sh | bash -s -- --uninstall"
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

  if [[ -f "$SERVICE_PATH" ]]; then
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

echo "==================================="
echo "Pulse Docker Agent Installer"
echo "==================================="
if [[ -n "$AGENT_PATH_NOTE" ]]; then
  echo "$AGENT_PATH_NOTE"
fi
echo "Primary Pulse URL: $PRIMARY_URL"
if [[ ${#TARGETS[@]} -gt 1 ]]; then
  echo "Additional Pulse targets: $(( ${#TARGETS[@]} - 1 ))"
fi
echo "Install path: $AGENT_PATH"
echo "Interval: $INTERVAL"
if [[ "$UNINSTALL" != true ]]; then
  echo "API token: (provided)"
  echo "Targets:"
  for spec in "${TARGETS[@]}"; do
    IFS='|' read -r target_url _ target_insecure <<< "$spec"
    if [[ "$target_insecure" == "true" ]]; then
      echo "  - $target_url (skip TLS verify)"
    else
      echo "  - $target_url"
    fi
  done
fi
echo ""

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
      echo "Warning: Unknown architecture '$ARCH'. Falling back to default agent binary."
      ;;
  esac
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Warning: Docker not found. The agent requires Docker to be installed."
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
    fi
fi

# Download agent binary
echo "Downloading Pulse Docker agent..."
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
    echo "Falling back to server default agent binary..."
else
    echo "Error: Failed to download agent binary"
    echo "Make sure the Pulse server is accessible at: $PRIMARY_URL"
    exit 1
fi

chmod +x "$AGENT_PATH"
echo "✓ Agent binary installed"

# Check if systemd is available
if ! command -v systemctl &> /dev/null || [ ! -d /etc/systemd/system ]; then
    echo ""
    echo "Systemd not detected - configuring for alternative init system..."

    # Check if this is Unraid (has /boot/config directory)
    if [ -d /boot/config ]; then
        echo "Unraid detected - creating startup script..."

        # Create go.d directory if it doesn't exist
        mkdir -p /boot/config/go.d

        # Create startup script
        STARTUP_SCRIPT="/boot/config/go.d/pulse-docker-agent.sh"
cat > "$STARTUP_SCRIPT" <<EOF
#!/bin/bash
# Pulse Docker Agent - Auto-start script
sleep 10  # Wait for Docker to be ready
PULSE_URL="$PRIMARY_URL" PULSE_TOKEN="$PRIMARY_TOKEN" PULSE_TARGETS="$JOINED_TARGETS" PULSE_INSECURE_SKIP_VERIFY="$PRIMARY_INSECURE" $AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL" > /var/log/pulse-docker-agent.log 2>&1 &
EOF

        chmod +x "$STARTUP_SCRIPT"
        echo "✓ Startup script created at $STARTUP_SCRIPT"

        # Start the agent now
        echo "Starting agent..."
        PULSE_URL="$PRIMARY_URL" PULSE_TOKEN="$PRIMARY_TOKEN" PULSE_TARGETS="$JOINED_TARGETS" PULSE_INSECURE_SKIP_VERIFY="$PRIMARY_INSECURE" $AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL" > /var/log/pulse-docker-agent.log 2>&1 &

        echo ""
        echo "==================================="
        echo "✓ Installation complete!"
        echo "==================================="
        echo ""
        echo "The agent is now running and will auto-start on reboot."
        echo "Your Docker host should appear in Pulse within 30 seconds."
        echo ""
        echo "Logs: /var/log/pulse-docker-agent.log"
        echo ""
        exit 0
    fi

    # For other non-systemd systems, provide manual instructions
    echo ""
    echo "The agent has been installed to: $AGENT_PATH"
    echo ""
    echo "To run manually:"
    echo "  PULSE_URL=$PRIMARY_URL PULSE_TOKEN=<api-token> \\"
    echo "  PULSE_TARGETS=\"https://pulse.example.com|<token>[;https://pulse-alt.example.com|<token2>]\" \\"
    echo "  $AGENT_PATH --interval $INTERVAL &"
    echo ""
    echo "To make it start automatically, add the above command to your system's startup scripts."
    echo ""
    exit 0
fi

# Create systemd service
echo "Creating systemd service..."
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
ExecStart=$AGENT_PATH --url "$PRIMARY_URL" --interval "$INTERVAL"
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF

echo "✓ Systemd service created"

# Reload systemd and start service
echo "Starting service..."
systemctl daemon-reload
systemctl enable pulse-docker-agent
systemctl start pulse-docker-agent

echo ""
echo "==================================="
echo "✓ Installation complete!"
echo "==================================="
echo ""
echo "The Pulse Docker agent is now running."
echo "Check status: systemctl status pulse-docker-agent"
echo "View logs: journalctl -u pulse-docker-agent -f"
echo ""
echo "Your Docker host should appear in Pulse within 30 seconds."
