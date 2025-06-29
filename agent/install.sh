#!/bin/bash

# Pulse PBS Agent Installation Script

set -e

AGENT_DIR="/opt/pulse-agent"
CONFIG_DIR="/etc/pulse-agent"
SERVICE_FILE="/etc/systemd/system/pulse-agent.service"

echo "Installing Pulse PBS Agent..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "Node.js is required but not installed. Please install Node.js 14 or later."
    exit 1
fi

# Create directories
echo "Creating directories..."
mkdir -p "$AGENT_DIR"
mkdir -p "$CONFIG_DIR"

# Copy agent files
echo "Copying agent files..."
cp pulse-agent.js "$AGENT_DIR/"
cp package.json "$AGENT_DIR/"
chmod +x "$AGENT_DIR/pulse-agent.js"

# Install dependencies
echo "Installing dependencies..."
cd "$AGENT_DIR"
npm install --production

# Create user if it doesn't exist
if ! id -u pulse &>/dev/null; then
    echo "Creating pulse user..."
    useradd -r -s /bin/false -d /nonexistent -c "Pulse Agent" pulse
fi

# Set ownership
chown -R pulse:pulse "$AGENT_DIR"

# Install systemd service
echo "Installing systemd service..."
cp "$(dirname "$0")/pulse-agent.service" "$SERVICE_FILE"
systemctl daemon-reload

# Copy example config if no config exists
if [ ! -f "$CONFIG_DIR/pulse-agent.env" ]; then
    echo "Creating configuration file..."
    cp "$(dirname "$0")/pulse-agent.env.example" "$CONFIG_DIR/pulse-agent.env"
    chmod 600 "$CONFIG_DIR/pulse-agent.env"
    echo ""
    echo "IMPORTANT: Edit $CONFIG_DIR/pulse-agent.env with your configuration"
else
    echo "Configuration file already exists at $CONFIG_DIR/pulse-agent.env"
fi

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit $CONFIG_DIR/pulse-agent.env with your Pulse server details and PBS API token"
echo "2. Start the agent: systemctl start pulse-agent"
echo "3. Enable auto-start: systemctl enable pulse-agent"
echo "4. Check status: systemctl status pulse-agent"
echo "5. View logs: journalctl -u pulse-agent -f"