#!/bin/bash

# Pulse for Proxmox VE - Installer
# https://github.com/rcourtman/Pulse

NODE_MAJOR_VERSION=20
PULSE_DIR="/opt/pulse"
OLD_PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse"
SERVICE_NAME="pulse-monitor.service"
REPO_BASE_URL="https://github.com/rcourtman/Pulse"

MODE_UPDATE=""
INSTALL_MODE=""
SPECIFIED_VERSION_TAG=""
TARGET_TAG=""

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --update) MODE_UPDATE="true"; shift ;;
        --version)
            if [[ -n "$2" ]] && [[ "$2" != --* ]]; then
                SPECIFIED_VERSION_TAG="$2"
                shift 2
            else
                echo "Error: --version requires a tag name (e.g., v3.16.0)" >&2
                exit 1
            fi
            ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
done

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

# Self-update check
self_update_check() {
    # Skip if running from pipe or no curl available
    if [ ! -t 0 ] || ! command -v curl &>/dev/null; then
        return 0
    fi
    
    print_info "Checking for installer updates..."
    
    local current_script="$0"
    local temp_script="/tmp/install-pulse-new.sh"
    local script_url="https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh"
    
    # Download latest version
    if curl -sL "$script_url" -o "$temp_script" 2>/dev/null; then
        # Compare with current script
        if ! diff -q "$current_script" "$temp_script" >/dev/null 2>&1; then
            print_warning "A newer version of the installer is available"
            read -p "Update installer and restart? [Y/n]: " update_confirm
            
            if [[ ! "$update_confirm" =~ ^[Nn]$ ]]; then
                print_info "Updating installer..."
                cp "$temp_script" "$current_script"
                chmod +x "$current_script"
                rm -f "$temp_script"
                
                print_success "Installer updated, restarting..."
                exec "$current_script" "$@"
            fi
        else
            print_success "Installer is up to date"
        fi
        rm -f "$temp_script"
    fi
}

# Print welcome banner
print_welcome() {
    echo ""
    echo -e "\033[1;34mPulse for Proxmox VE Installer\033[0m"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Get latest release tag from GitHub
get_latest_release_tag() {
    local api_url="https://api.github.com/repos/rcourtman/Pulse/releases/latest"
    local latest_tag
    
    latest_tag=$(curl -s "$api_url" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    
    if [ -z "$latest_tag" ] || [ "$latest_tag" = "null" ]; then
        return 1
    fi
    
    echo "$latest_tag"
}

# Check if a release exists
check_release_exists() {
    local tag=$1
    local api_url="https://api.github.com/repos/rcourtman/Pulse/releases/tags/$tag"
    
    curl -s --fail "$api_url" > /dev/null 2>&1
}

# Download and extract tarball
download_and_extract_tarball() {
    local version=$1
    local temp_dir="/tmp/pulse-install-$$"
    local tarball_url="${REPO_BASE_URL}/releases/download/${version}/pulse-${version}.tar.gz"
    
    print_info "Downloading Pulse ${version}..."
    
    # Create temporary directory
    mkdir -p "$temp_dir"
    cd "$temp_dir" || exit 1
    
    # Download tarball with progress
    if ! curl -L --progress-bar -o "pulse.tar.gz" "$tarball_url"; then
        print_error "Failed to download release"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Extract tarball
    print_info "Extracting files..."
    if ! tar -xzf "pulse.tar.gz"; then
        print_error "Failed to extract files"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Find extracted directory
    local extracted_dir=$(find . -maxdepth 1 -type d -name "pulse-*" | head -1)
    if [ -z "$extracted_dir" ]; then
        print_error "Invalid release archive"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Backup existing data if updating
    if [ -d "$PULSE_DIR/data" ]; then
        print_info "Backing up user data..."
        cp -r "$PULSE_DIR/data" "$temp_dir/data-backup"
    fi
    
    # Remove old installation
    if [ -d "$PULSE_DIR" ]; then
        rm -rf "$PULSE_DIR"
    fi
    
    # Move to installation directory
    mv "$extracted_dir" "$PULSE_DIR"
    
    # Restore data if it existed
    if [ -d "$temp_dir/data-backup" ]; then
        print_info "Restoring user data..."
        cp -r "$temp_dir/data-backup"/* "$PULSE_DIR/data/" 2>/dev/null || true
    fi
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    print_success "Installation files ready"
    return 0
}

# Get current installed version
get_current_version() {
    if [ -f "$PULSE_DIR/package.json" ]; then
        grep '"version"' "$PULSE_DIR/package.json" | head -1 | sed -E 's/.*"version": "([^"]+)".*/\1/'
    fi
}

# Check installation status
check_installation() {
    local pulse_exists=false
    local old_pulse_exists=false
    local service_exists=false
    
    [ -d "$PULSE_DIR" ] && pulse_exists=true
    [ -d "$OLD_PULSE_DIR" ] && old_pulse_exists=true
    systemctl list-unit-files | grep -q "^$SERVICE_NAME" && service_exists=true
    
    if [ "$old_pulse_exists" = true ]; then
        print_info "Old installation detected at $OLD_PULSE_DIR"
        INSTALL_MODE="migrate"
    elif [ "$pulse_exists" = true ] || [ "$service_exists" = true ]; then
        local current_version=$(get_current_version)
        if [ -n "$current_version" ]; then
            print_info "Pulse v$current_version is installed"
        else
            print_info "Pulse is installed"
        fi
        
        if [ -n "$MODE_UPDATE" ]; then
            INSTALL_MODE="update"
        else
            echo ""
            echo "Choose an action:"
            echo "  1) Update to latest version"
            echo "  2) Remove Pulse"
            echo "  3) Cancel"
            echo ""
            read -p "Your choice [1-3]: " choice
            
            case $choice in
                1) INSTALL_MODE="update" ;;
                2) INSTALL_MODE="remove" ;;
                3) print_info "Cancelled"; exit 0 ;;
                *) print_error "Invalid choice"; exit 1 ;;
            esac
        fi
    else
        INSTALL_MODE="install"
        print_info "Pulse is not installed"
    fi
}

# Install system dependencies
install_dependencies() {
    print_info "Checking system dependencies..."
    
    # Update package lists quietly
    apt-get update -qq || { print_error "Failed to update package lists"; exit 1; }
    
    # Check and install missing dependencies
    local deps="curl sudo gpg tar"
    local missing_deps=""
    
    for dep in $deps; do
        if ! command -v $dep &> /dev/null; then
            missing_deps="$missing_deps $dep"
        fi
    done
    
    if [ -n "$missing_deps" ]; then
        print_info "Installing:$missing_deps"
        apt-get install -y -qq $missing_deps || { print_error "Failed to install dependencies"; exit 1; }
    fi
    
    print_success "System dependencies ready"
}

# Setup Node.js
setup_node() {
    if command -v node &> /dev/null; then
        local current_version=$(node -v | sed 's/v//' | cut -d. -f1)
        if [ "$current_version" -ge "$NODE_MAJOR_VERSION" ]; then
            print_success "Node.js $(node -v) is installed"
            return 0
        fi
    fi
    
    print_info "Installing Node.js v$NODE_MAJOR_VERSION..."
    
    # Install NodeSource repository
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR_VERSION.x nodistro main" > /etc/apt/sources.list.d/nodesource.list
    
    apt-get update -qq || { print_error "Failed to update package lists"; exit 1; }
    apt-get install -y -qq nodejs || { print_error "Failed to install Node.js"; exit 1; }
    
    print_success "Node.js $(node -v) installed"
}

# Create pulse user
create_pulse_user() {
    if id "$PULSE_USER" &>/dev/null; then
        print_success "User '$PULSE_USER' exists"
    else
        print_info "Creating user '$PULSE_USER'..."
        useradd -r -s /bin/false -d /nonexistent -U "$PULSE_USER" || {
            print_error "Failed to create user"
            exit 1
        }
        print_success "User created"
    fi
}

# Set file permissions
set_permissions() {
    print_info "Setting permissions..."
    
    chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR"
    chmod -R 755 "$PULSE_DIR"
    
    # Ensure data directory exists
    mkdir -p "$PULSE_DIR/data"
    chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
    chmod 755 "$PULSE_DIR/data"
}

# Configure environment
configure_environment() {
    local env_file="$PULSE_DIR/.env"
    if [ ! -f "$env_file" ]; then
        cat > "$env_file" << EOF
NODE_ENV=production
PORT=3000
EOF
        chown "$PULSE_USER:$PULSE_USER" "$env_file"
        chmod 600 "$env_file"
    fi
}

# Setup systemd service
setup_systemd_service() {
    print_info "Configuring service..."
    
    cat > "/etc/systemd/system/$SERVICE_NAME" << EOF
[Unit]
Description=Pulse Monitor for Proxmox VE
After=network.target

[Service]
Type=simple
User=$PULSE_USER
WorkingDirectory=$PULSE_DIR
ExecStart=/usr/bin/node $PULSE_DIR/server/index.js
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-monitor
Environment="NODE_ENV=production"
Environment="PORT=3000"

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" &>/dev/null
    systemctl start "$SERVICE_NAME"
    
    # Wait a moment and check if service started
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "Service started"
    else
        print_error "Service failed to start"
        print_info "Check logs: journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
}

# Perform installation
perform_install() {
    print_info "Installing Pulse..."
    echo ""
    
    install_dependencies
    setup_node
    create_pulse_user
    
    # Determine target version
    if [ -n "$SPECIFIED_VERSION_TAG" ]; then
        TARGET_TAG="$SPECIFIED_VERSION_TAG"
        if ! check_release_exists "$TARGET_TAG"; then
            print_error "Version $TARGET_TAG does not exist"
            exit 1
        fi
    else
        TARGET_TAG=$(get_latest_release_tag)
        if [ -z "$TARGET_TAG" ]; then
            print_error "Could not determine latest version"
            exit 1
        fi
    fi
    
    download_and_extract_tarball "$TARGET_TAG" || exit 1
    set_permissions
    configure_environment
    setup_systemd_service
    
    echo ""
    print_success "Installation complete!"
}

# Perform update
perform_update() {
    # Stop service
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "Stopping service..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    # Determine target version
    if [ -n "$SPECIFIED_VERSION_TAG" ]; then
        TARGET_TAG="$SPECIFIED_VERSION_TAG"
        if ! check_release_exists "$TARGET_TAG"; then
            print_error "Version $TARGET_TAG does not exist"
            exit 1
        fi
    else
        TARGET_TAG=$(get_latest_release_tag)
        if [ -z "$TARGET_TAG" ]; then
            print_error "Could not determine latest version"
            exit 1
        fi
    fi
    
    local current_version=$(get_current_version)
    if [ "$current_version" = "${TARGET_TAG#v}" ]; then
        print_info "Already running latest version"
        systemctl start "$SERVICE_NAME"
        return 0
    fi
    
    print_info "Updating to $TARGET_TAG..."
    echo ""
    
    download_and_extract_tarball "$TARGET_TAG" || exit 1
    set_permissions
    
    # Restart service
    systemctl start "$SERVICE_NAME"
    
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo ""
        print_success "Update complete!"
    else
        print_error "Service failed to start after update"
        exit 1
    fi
}

# Remove Pulse
perform_remove() {
    print_warning "This will completely remove Pulse"
    read -p "Are you sure? [y/N]: " confirm
    
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        print_info "Cancelled"
        exit 0
    fi
    
    echo ""
    
    # Stop and disable service
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "Stopping service..."
        systemctl stop "$SERVICE_NAME"
    fi
    systemctl disable "$SERVICE_NAME" &>/dev/null || true
    rm -f "/etc/systemd/system/$SERVICE_NAME"
    systemctl daemon-reload
    
    # Remove directories
    [ -d "$PULSE_DIR" ] && rm -rf "$PULSE_DIR"
    [ -d "$OLD_PULSE_DIR" ] && rm -rf "$OLD_PULSE_DIR"
    
    print_success "Pulse has been removed"
}

# Migrate from old installation
perform_migration() {
    print_info "Migrating old installation..."
    
    # Stop old service
    if systemctl is-active --quiet "pulse-proxmox.service"; then
        systemctl stop "pulse-proxmox.service"
    fi
    systemctl disable "pulse-proxmox.service" &>/dev/null || true
    
    # Backup old data
    local backup_dir="/tmp/pulse-migration-$$"
    if [ -d "$OLD_PULSE_DIR/data" ]; then
        mkdir -p "$backup_dir"
        cp -r "$OLD_PULSE_DIR/data" "$backup_dir/"
    fi
    
    # Remove old installation
    rm -rf "$OLD_PULSE_DIR"
    rm -f "/etc/systemd/system/pulse-proxmox.service"
    
    # Perform fresh install
    INSTALL_MODE="install"
    perform_install
    
    # Restore old data
    if [ -d "$backup_dir/data" ]; then
        print_info "Restoring data..."
        cp -r "$backup_dir/data"/* "$PULSE_DIR/data/" 2>/dev/null || true
        chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
        rm -rf "$backup_dir"
    fi
}

# Show final instructions
show_final_instructions() {
    local ip=$(hostname -I | awk '{print $1}')
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "Access Pulse: \033[1;36mhttp://${ip}:3000\033[0m"
    echo ""
    echo "Useful commands:"
    echo "  • systemctl status $SERVICE_NAME"
    echo "  • journalctl -u $SERVICE_NAME -f"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Main execution
main() {
    check_root
    self_update_check
    print_welcome
    check_installation
    
    case "$INSTALL_MODE" in
        install)
            perform_install
            show_final_instructions
            ;;
        update)
            perform_update
            show_final_instructions
            ;;
        remove)
            perform_remove
            ;;
        migrate)
            perform_migration
            show_final_instructions
            ;;
        *)
            print_error "Invalid operation"
            exit 1
            ;;
    esac
}

# Run main function
main