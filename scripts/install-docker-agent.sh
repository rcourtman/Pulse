#!/bin/bash
set -e

# Pulse Docker Agent Installer/Uninstaller
# Install: curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- --url http://pulse.example.com --token <api-token>
# Uninstall: curl -fsSL http://pulse.example.com/install-docker-agent.sh | bash -s -- --uninstall

PULSE_URL=""
AGENT_PATH="/usr/local/bin/pulse-docker-agent"
SERVICE_PATH="/etc/systemd/system/pulse-docker-agent.service"
UNRAID_STARTUP="/boot/config/go.d/pulse-docker-agent.sh"
LOG_PATH="/var/log/pulse-docker-agent.log"
INTERVAL="30s"
UNINSTALL=false
TOKEN="${PULSE_TOKEN:-}"

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
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 --url <Pulse URL> --token <API token> [--interval 30s]"
      echo "       $0 --uninstall"
      exit 1
      ;;
  esac
done

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "Error: This script must be run as root"
   echo ""
   echo "Please run the command with 'bash' instead of just piping to bash:"
   echo "  Install: curl -fsSL <URL>/install-docker-agent.sh | bash -s -- --url <URL>"
   echo "  Uninstall: curl -fsSL <URL>/install-docker-agent.sh | bash -s -- --uninstall"
   exit 1
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

# Validate URL and token for install
if [[ "$UNINSTALL" != true ]]; then
  if [[ -z "$PULSE_URL" ]]; then
    echo "Error: --url parameter is required for installation"
    echo ""
    echo "Usage:"
    echo "  Install:   $0 --url http://pulse.example.com --token <api-token> [--interval 30s]"
    echo "  Uninstall: $0 --uninstall"
    exit 1
  fi

  if [[ -z "$TOKEN" ]]; then
    echo "Error: API token required. Provide via --token or PULSE_TOKEN environment variable."
    exit 1
  fi
fi

echo "==================================="
echo "Pulse Docker Agent Installer"
echo "==================================="
echo "Pulse URL: $PULSE_URL"
echo "Install path: $AGENT_PATH"
echo "Interval: $INTERVAL"
if [[ "$UNINSTALL" != true ]]; then
  echo "API token: (provided)"
fi
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Warning: Docker not found. The agent requires Docker to be installed."
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Download agent binary
echo "Downloading Pulse Docker agent..."
if command -v wget &> /dev/null; then
    wget -q --show-progress -O "$AGENT_PATH" "$PULSE_URL/download/pulse-docker-agent" || {
        echo "Error: Failed to download agent binary"
        echo "Make sure the Pulse server is accessible at: $PULSE_URL"
        exit 1
    }
elif command -v curl &> /dev/null; then
    curl -fL --progress-bar -o "$AGENT_PATH" "$PULSE_URL/download/pulse-docker-agent" || {
        echo "Error: Failed to download agent binary"
        echo "Make sure the Pulse server is accessible at: $PULSE_URL"
        exit 1
    }
else
    echo "Error: Neither wget nor curl found. Please install one of them."
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
PULSE_TOKEN="$TOKEN" $AGENT_PATH --url "$PULSE_URL" --interval "$INTERVAL" > /var/log/pulse-docker-agent.log 2>&1 &
EOF

        chmod +x "$STARTUP_SCRIPT"
        echo "✓ Startup script created at $STARTUP_SCRIPT"

        # Start the agent now
        echo "Starting agent..."
        PULSE_TOKEN="$TOKEN" $AGENT_PATH --url "$PULSE_URL" --interval "$INTERVAL" > /var/log/pulse-docker-agent.log 2>&1 &

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
    echo "  PULSE_TOKEN=<api-token> $AGENT_PATH --url $PULSE_URL --interval $INTERVAL &"
    echo ""
    echo "To make it start automatically, add the above command to your system's startup scripts."
    echo ""
    exit 0
fi

# Create systemd service
echo "Creating systemd service..."
cat > "$SERVICE_PATH" << EOF
[Unit]
Description=Pulse Docker Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
Environment="PULSE_TOKEN=$TOKEN"
ExecStart=$AGENT_PATH --url "$PULSE_URL" --interval "$INTERVAL"
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
