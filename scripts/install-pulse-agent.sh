#!/bin/bash

# Pulse PBS Agent - Installer
# https://github.com/rcourtman/Pulse
#
# This script installs the Pulse agent on a PBS server to enable push-mode monitoring
# Run this on each PBS server that needs to push metrics to Pulse

AGENT_VERSION="1.0.0"
AGENT_DIR="/opt/pulse-agent"
CONFIG_DIR="/etc/pulse-agent"
SERVICE_NAME="pulse-agent.service"
GITHUB_BASE_URL="https://raw.githubusercontent.com/rcourtman/Pulse/main"

# Set safer defaults
set -u  # Exit on undefined variables

# Color output functions
print_info() {
    echo -e "\033[0;36m➜\033[0m $1"
}

print_success() {
    echo -e "\033[0;32m✓\033[0m $1"
}

print_warning() {
    echo -e "\033[0;33m⚠\033[0m $1"
}

print_error() {
    echo -e "\033[0;31m✗\033[0m $1" >&2
}

# Check if running as root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# Print welcome message
print_welcome() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "        Pulse PBS Agent Installer v${AGENT_VERSION}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "This will install the Pulse agent for push-mode"
    echo "monitoring of isolated PBS servers."
    echo ""
}

# Check if this is a PBS server
check_pbs() {
    if ! command -v proxmox-backup-manager &>/dev/null; then
        print_warning "This doesn't appear to be a PBS server"
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        print_success "PBS detected: $(proxmox-backup-manager version 2>/dev/null || echo 'version unknown')"
    fi
}

# Check network connectivity
check_network() {
    print_info "Checking network connectivity..."
    if curl -fsSL --connect-timeout 5 https://github.com >/dev/null 2>&1; then
        print_success "Network connectivity OK"
    else
        print_error "Cannot reach GitHub. Check your internet connection."
        exit 1
    fi
}

# Check prerequisites
check_prerequisites() {
    local missing_deps=()
    
    # Check for Node.js
    if ! command -v node &>/dev/null; then
        missing_deps+=("nodejs")
    else
        local node_version=$(node -v 2>/dev/null | sed 's/v//')
        local major_version=$(echo "$node_version" | cut -d. -f1)
        if [ -n "$major_version" ] && [ "$major_version" -lt 14 ]; then
            print_warning "Node.js version $node_version is installed, but version 14+ is recommended"
        else
            print_success "Node.js $node_version detected"
        fi
    fi
    
    # Check for npm
    if ! command -v npm &>/dev/null; then
        missing_deps+=("npm")
    fi
    
    # Check for curl
    if ! command -v curl &>/dev/null; then
        missing_deps+=("curl")
    fi
    
    # Install missing dependencies
    if [ ${#missing_deps[@]} -gt 0 ]; then
        print_info "Installing dependencies: ${missing_deps[*]}"
        
        # Update package list
        apt-get update -qq >/dev/null 2>&1
        
        # Install Node.js if needed
        if [[ " ${missing_deps[@]} " =~ " nodejs " ]]; then
            print_info "Installing Node.js..."
            curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null 2>&1
            apt-get install -y nodejs >/dev/null 2>&1
        fi
        
        # Install other dependencies
        for dep in "${missing_deps[@]}"; do
            if [ "$dep" != "nodejs" ]; then
                apt-get install -y "$dep" >/dev/null 2>&1
            fi
        done
        
        print_success "Dependencies installed"
    fi
}

# Download agent files
download_agent() {
    print_info "Downloading Pulse agent..."
    
    # Create directories
    mkdir -p "$AGENT_DIR"
    mkdir -p "$CONFIG_DIR"
    
    # Download files
    local files=(
        "agent/pulse-agent.js"
        "agent/package.json"
    )
    
    for file in "${files[@]}"; do
        local filename=$(basename "$file")
        local temp_file="/tmp/${filename}.$$"
        
        # Download to temp file first
        if curl -fsSL "$GITHUB_BASE_URL/$file" -o "$temp_file" 2>/dev/null; then
            # Verify the download
            if [ -s "$temp_file" ]; then
                mv "$temp_file" "$AGENT_DIR/$filename"
                print_success "Downloaded $filename"
            else
                print_error "Downloaded file is empty: $filename"
                rm -f "$temp_file"
                exit 1
            fi
        else
            print_error "Failed to download $filename"
            rm -f "$temp_file"
            exit 1
        fi
    done
    
    # Make agent executable
    chmod +x "$AGENT_DIR/pulse-agent.js"
}

# Install dependencies
install_dependencies() {
    print_info "Installing Node.js dependencies..."
    cd "$AGENT_DIR" || exit 1
    
    # Check if package.json exists
    if [ ! -f "package.json" ]; then
        print_error "package.json not found in $AGENT_DIR"
        exit 1
    fi
    
    # Run npm install with error output
    local npm_output
    npm_output=$(npm install --production 2>&1)
    local npm_status=$?
    
    if [ $npm_status -eq 0 ]; then
        print_success "Dependencies installed"
    else
        print_error "Failed to install dependencies:"
        echo "$npm_output" | tail -10
        exit 1
    fi
}

# Create user
create_user() {
    if ! id -u pulse &>/dev/null; then
        print_info "Creating pulse user..."
        useradd -r -s /bin/false -d /nonexistent -c "Pulse Agent" pulse
        print_success "User created"
    else
        print_success "User 'pulse' already exists"
    fi
    
    # Set ownership
    chown -R pulse:pulse "$AGENT_DIR"
}

# Create systemd service
create_service() {
    print_info "Creating systemd service..."
    
    cat > "/etc/systemd/system/$SERVICE_NAME" << 'EOF'
[Unit]
Description=Pulse PBS Monitoring Agent
After=network.target

[Service]
Type=simple
User=pulse
WorkingDirectory=/opt/pulse-agent
ExecStart=/usr/bin/node /opt/pulse-agent/pulse-agent.js
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/pulse-agent

# Environment variables
EnvironmentFile=-/etc/pulse-agent/pulse-agent.env

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    print_success "Service created"
}

# Create configuration
create_config() {
    if [ ! -f "$CONFIG_DIR/pulse-agent.env" ]; then
        print_info "Creating configuration template..."
        
        cat > "$CONFIG_DIR/pulse-agent.env" << 'EOF'
# Pulse PBS Agent Configuration
# 
# IMPORTANT: You must configure these settings before starting the agent

# Required: Pulse server URL
PULSE_SERVER_URL=https://pulse.example.com

# Required: API key for authentication
# Generate this in Pulse settings or set PULSE_PUSH_API_KEY on the Pulse server
PULSE_API_KEY=your-api-key-here

# Required: PBS API token
# Create with: pvesh create /access/users/pulse@pbs/token/monitoring --privsep=0
PBS_API_TOKEN=pulse@pbs!monitoring:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Optional: PBS API URL (default: https://localhost:8007)
# PBS_API_URL=https://localhost:8007

# Optional: PBS server fingerprint (if not set, certificate verification is disabled)
# PBS_FINGERPRINT=XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX

# Optional: Push interval in seconds (default: 30)
# PUSH_INTERVAL=30

# Optional: Agent ID (default: hostname)
# AGENT_ID=$(hostname)
EOF
        
        chmod 600 "$CONFIG_DIR/pulse-agent.env"
        print_success "Configuration template created"
    else
        print_warning "Configuration already exists"
    fi
}

# Show PBS token creation help
show_pbs_token_help() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "         PBS API Token Creation"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "To create a PBS API token for the agent, run:"
    echo ""
    echo -e "\033[1;36m  pvesh create /access/users/pulse@pbs/token/monitoring --privsep=0\033[0m"
    echo ""
    echo "Save the generated token - you'll need it for the configuration."
    echo ""
    read -p "Press Enter to continue..."
}

# Show final instructions
show_final_instructions() {
    local hostname=$(hostname)
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "        Installation Complete!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Edit the configuration file:"
    echo -e "   \033[1;36mnano $CONFIG_DIR/pulse-agent.env\033[0m"
    echo ""
    echo "2. Set your Pulse server URL and API key"
    echo ""
    echo "3. Add your PBS API token (see instructions above)"
    echo ""
    echo "4. Start the agent:"
    echo -e "   \033[1;36msystemctl start $SERVICE_NAME\033[0m"
    echo -e "   \033[1;36msystemctl enable $SERVICE_NAME\033[0m"
    echo ""
    echo "5. Check agent status:"
    echo -e "   \033[1;36msystemctl status $SERVICE_NAME\033[0m"
    echo -e "   \033[1;36mjournalctl -u $SERVICE_NAME -f\033[0m"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Check for existing installation
check_existing_installation() {
    if [ -d "$AGENT_DIR" ] || [ -f "/etc/systemd/system/$SERVICE_NAME" ]; then
        print_warning "Existing Pulse agent installation detected"
        echo ""
        echo "Options:"
        echo "1) Upgrade existing installation"
        echo "2) Remove and reinstall"
        echo "3) Cancel"
        echo ""
        read -p "Choose an option [1-3]: " -n 1 -r choice
        echo ""
        
        case "$choice" in
            1)
                print_info "Upgrading existing installation..."
                # Stop service before upgrade
                systemctl stop $SERVICE_NAME 2>/dev/null || true
                ;;
            2)
                uninstall
                ;;
            3)
                exit 0
                ;;
            *)
                print_error "Invalid choice"
                exit 1
                ;;
        esac
    fi
}

# Main installation
main() {
    check_root
    print_welcome
    check_existing_installation
    check_pbs
    check_network
    check_prerequisites
    download_agent
    install_dependencies
    create_user
    create_service
    create_config
    show_pbs_token_help
    show_final_instructions
}

# Uninstall function
uninstall() {
    print_warning "This will remove the Pulse agent"
    read -p "Are you sure? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
    
    # Stop and disable service
    systemctl stop $SERVICE_NAME 2>/dev/null
    systemctl disable $SERVICE_NAME 2>/dev/null
    
    # Remove files
    rm -f "/etc/systemd/system/$SERVICE_NAME"
    rm -rf "$AGENT_DIR"
    
    # Keep config for potential reinstall
    print_info "Configuration preserved in $CONFIG_DIR"
    print_success "Pulse agent removed"
}

# Parse arguments
case "${1:-}" in
    --uninstall|--remove)
        check_root
        uninstall
        ;;
    --help|-h)
        echo "Usage: $0 [--uninstall]"
        echo ""
        echo "Options:"
        echo "  --uninstall    Remove the Pulse agent"
        echo "  --help         Show this help message"
        exit 0
        ;;
    *)
        main
        ;;
esac