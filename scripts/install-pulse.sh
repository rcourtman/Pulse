#!/bin/bash

# Pulse for Proxmox VE - Simplified Installer
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
                echo "Error: --version requires a tag name [e.g., v3.2.3]" >&2
                exit 1
            fi
            ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
done

# Color output functions
print_info() {
    echo -e "\033[1;34m[INFO]\033[0m $1"
}

print_success() {
    echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

print_warning() {
    echo -e "\033[1;33m[WARNING]\033[0m $1"
}

print_error() {
    echo -e "\033[1;31m[ERROR]\033[0m $1" >&2
}

# Check if running as root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        print_error "This script must be run as root. Please use sudo."
        exit 1
    fi
}

# Print welcome banner
print_welcome() {
    echo -e "\033[1;34m"
    echo ' ____        _          '
    echo '|  _ \ _   _| |___  ___ '
    echo '| |_) | | | | / __|/ _ \'
    echo '|  __/| |_| | \__ \  __/'
    echo '|_|    \__,_|_|___/\___|'
    echo '                       '
    echo -e "\033[0m"
    echo ""
    echo -e "\033[1mWelcome to the Pulse for Proxmox VE Installer\033[0m"
    echo "This script will install Pulse, a lightweight monitoring application."
    echo "See README.md for more details: https://github.com/rcourtman/Pulse"
    echo ""
    print_info "Required Dependencies:"
    print_info "- Core Tools: curl, sudo, gpg, tar (via apt)"
    print_info "- Runtime: Node.js & npm (via NodeSource)"
    echo ""
    
    read -p "Do you want to proceed? [Y/n]: " confirm_proceed
    if [[ "$confirm_proceed" =~ ^[Nn]$ ]]; then
        print_error "Operation aborted by user."
        exit 1
    fi
    print_info "Proceeding..."
}

# Get latest release tag from GitHub
get_latest_release_tag() {
    local api_url="https://api.github.com/repos/rcourtman/Pulse/releases/latest"
    local latest_tag
    
    latest_tag=$(curl -s "$api_url" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    
    if [ -z "$latest_tag" ] || [ "$latest_tag" = "null" ]; then
        print_error "Could not determine latest release tag"
        return 1
    fi
    
    echo "$latest_tag"
}

# Check if a release exists
check_release_exists() {
    local tag=$1
    local api_url="https://api.github.com/repos/rcourtman/Pulse/releases/tags/$tag"
    
    if curl -s --fail "$api_url" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Download and extract tarball
download_and_extract_tarball() {
    local version=$1
    local temp_dir="/tmp/pulse-install-$$"
    local tarball_url="${REPO_BASE_URL}/archive/refs/tags/${version}.tar.gz"
    
    print_info "Downloading Pulse ${version}..."
    
    # Create temporary directory
    mkdir -p "$temp_dir"
    cd "$temp_dir" || exit 1
    
    # Download tarball
    if ! curl -L -o "pulse.tar.gz" "$tarball_url"; then
        print_error "Failed to download tarball from $tarball_url"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Extract tarball
    print_info "Extracting files..."
    if ! tar -xzf "pulse.tar.gz"; then
        print_error "Failed to extract tarball"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Find extracted directory (format: Pulse-{version without v})
    local extracted_dir=$(find . -maxdepth 1 -type d -name "Pulse-*" | head -1)
    if [ -z "$extracted_dir" ]; then
        print_error "Could not find extracted directory"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # Backup existing data if updating
    if [ -d "$PULSE_DIR/data" ]; then
        print_info "Backing up existing data..."
        cp -r "$PULSE_DIR/data" "$temp_dir/data-backup"
    fi
    
    # Remove old installation
    if [ -d "$PULSE_DIR" ]; then
        print_info "Removing old installation..."
        rm -rf "$PULSE_DIR"
    fi
    
    # Move extracted files to installation directory
    print_info "Installing files to $PULSE_DIR..."
    mv "$extracted_dir" "$PULSE_DIR"
    
    # Restore data if it existed
    if [ -d "$temp_dir/data-backup" ]; then
        print_info "Restoring data..."
        cp -r "$temp_dir/data-backup"/* "$PULSE_DIR/data/" 2>/dev/null || true
    fi
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    print_success "Files extracted successfully"
    return 0
}

# Get current installed version
get_current_version() {
    if [ -f "$PULSE_DIR/package.json" ]; then
        grep '"version"' "$PULSE_DIR/package.json" | head -1 | sed -E 's/.*"version": "([^"]+)".*/\1/'
    else
        echo ""
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
            print_info "Pulse v$current_version is currently installed"
        else
            print_info "Pulse is currently installed"
        fi
        
        if [ -n "$MODE_UPDATE" ]; then
            INSTALL_MODE="update"
        else
            echo ""
            echo "What would you like to do?"
            echo "  1) Update to latest version"
            echo "  2) Remove Pulse"
            echo "  3) Cancel"
            read -p "Enter your choice [1-3]: " choice
            
            case $choice in
                1) INSTALL_MODE="update" ;;
                2) INSTALL_MODE="remove" ;;
                3) print_info "Operation cancelled"; exit 0 ;;
                *) print_error "Invalid choice"; exit 1 ;;
            esac
        fi
    else
        INSTALL_MODE="install"
    fi
}

# Install system dependencies
install_dependencies() {
    print_info "Installing system dependencies..."
    
    apt-get update || { print_error "Failed to update package lists"; exit 1; }
    
    local deps="curl sudo gpg tar"
    for dep in $deps; do
        if ! command -v $dep &> /dev/null; then
            print_info "Installing $dep..."
            apt-get install -y $dep || { print_error "Failed to install $dep"; exit 1; }
        fi
    done
    
    print_success "System dependencies installed"
}

# Setup Node.js
setup_node() {
    if command -v node &> /dev/null; then
        local current_version=$(node -v | sed 's/v//' | cut -d. -f1)
        if [ "$current_version" -ge "$NODE_MAJOR_VERSION" ]; then
            print_info "Node.js v$(node -v) is already installed"
            return 0
        fi
    fi
    
    print_info "Installing Node.js v$NODE_MAJOR_VERSION..."
    
    # Install NodeSource repository
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR_VERSION.x nodistro main" > /etc/apt/sources.list.d/nodesource.list
    
    apt-get update || { print_error "Failed to update package lists"; exit 1; }
    apt-get install -y nodejs || { print_error "Failed to install Node.js"; exit 1; }
    
    print_success "Node.js $(node -v) installed"
}

# Create pulse user
create_pulse_user() {
    if id "$PULSE_USER" &>/dev/null; then
        print_info "User $PULSE_USER already exists"
    else
        print_info "Creating user $PULSE_USER..."
        useradd -r -s /bin/false -d /nonexistent -U "$PULSE_USER" || {
            print_error "Failed to create user $PULSE_USER"
            exit 1
        }
        print_success "User $PULSE_USER created"
    fi
}

# Install npm dependencies
install_npm_deps() {
    print_info "Installing NPM dependencies..."
    cd "$PULSE_DIR" || exit 1
    
    # Check if node_modules exists (from tarball)
    if [ -d "node_modules" ] && [ -f "package-lock.json" ]; then
        print_info "Dependencies already included in release"
        # Verify dependencies are working
        if npm list --depth=0 &>/dev/null; then
            print_success "NPM dependencies verified"
            # Check if CSS is already built
            if [ -f "src/public/index.css" ]; then
                print_info "CSS assets already built in release"
                return 0
            fi
        else
            print_warning "Dependencies need to be reinstalled"
            rm -rf node_modules
        fi
    fi
    
    # For tarball installations, we need to install ALL dependencies first to build CSS
    print_info "Installing all dependencies (including dev) for CSS build..."
    npm ci || npm install || {
        print_error "Failed to install NPM dependencies"
        exit 1
    }
    
    # Build CSS if needed
    if [ ! -f "src/public/output.css" ]; then
        print_info "Building CSS assets..."
        npm run build:css || {
            print_error "Failed to build CSS assets"
            exit 1
        }
        print_success "CSS assets built"
    else
        print_info "CSS assets already exist"
    fi
    
    # Now remove dev dependencies to save space
    print_info "Removing development dependencies..."
    npm prune --omit=dev || print_warning "Failed to prune dev dependencies"
    
    print_success "NPM dependencies installed and optimized"
}

# Set file permissions
set_permissions() {
    print_info "Setting file permissions..."
    
    chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR"
    chmod -R 755 "$PULSE_DIR"
    
    # Ensure data directory exists and has correct permissions
    mkdir -p "$PULSE_DIR/data"
    chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
    chmod 755 "$PULSE_DIR/data"
    
    print_success "Permissions set"
}

# Configure environment
configure_environment() {
    print_info "Configuring environment..."
    
    local env_file="$PULSE_DIR/.env"
    if [ ! -f "$env_file" ]; then
        cat > "$env_file" << EOF
NODE_ENV=production
PORT=3000
EOF
        chown "$PULSE_USER:$PULSE_USER" "$env_file"
        chmod 600 "$env_file"
    fi
    
    print_success "Environment configured"
}

# Setup systemd service
setup_systemd_service() {
    print_info "Setting up systemd service..."
    
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
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "Service started successfully"
    else
        print_error "Service failed to start. Check: journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
}

# Perform installation
perform_install() {
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
            print_error "Could not determine version to install"
            exit 1
        fi
    fi
    
    print_info "Installing Pulse $TARGET_TAG..."
    
    download_and_extract_tarball "$TARGET_TAG" || exit 1
    install_npm_deps
    set_permissions
    configure_environment
    setup_systemd_service
}

# Perform update
perform_update() {
    # Stop service
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "Stopping $SERVICE_NAME..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    # Backup user data
    local backup_dir="/tmp/pulse-backup-$$"
    if [ -d "$PULSE_DIR/data" ]; then
        print_info "Backing up user data..."
        mkdir -p "$backup_dir"
        cp -r "$PULSE_DIR/data" "$backup_dir/"
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
            print_error "Could not determine version to update to"
            exit 1
        fi
    fi
    
    local current_version=$(get_current_version)
    if [ "$current_version" = "${TARGET_TAG#v}" ]; then
        print_info "Already running version $TARGET_TAG"
        systemctl start "$SERVICE_NAME"
        return 0
    fi
    
    print_info "Updating from v$current_version to $TARGET_TAG..."
    
    download_and_extract_tarball "$TARGET_TAG" || exit 1
    
    # Restore user data
    if [ -d "$backup_dir/data" ]; then
        print_info "Restoring user data..."
        cp -r "$backup_dir/data"/* "$PULSE_DIR/data/" 2>/dev/null || true
        rm -rf "$backup_dir"
    fi
    
    install_npm_deps
    set_permissions
    
    # Restart service
    systemctl start "$SERVICE_NAME"
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "Update completed successfully"
    else
        print_error "Service failed to start after update"
        exit 1
    fi
}

# Remove Pulse
perform_remove() {
    print_warning "This will remove Pulse completely from your system"
    read -p "Are you sure? [y/N]: " confirm
    
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        print_info "Removal cancelled"
        exit 0
    fi
    
    # Stop and disable service
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
    fi
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "/etc/systemd/system/$SERVICE_NAME"
    systemctl daemon-reload
    
    # Remove directory
    if [ -d "$PULSE_DIR" ]; then
        rm -rf "$PULSE_DIR"
    fi
    
    # Remove old directory if exists
    if [ -d "$OLD_PULSE_DIR" ]; then
        rm -rf "$OLD_PULSE_DIR"
    fi
    
    print_success "Pulse has been removed"
}

# Migrate from old installation
perform_migration() {
    print_info "Migrating from $OLD_PULSE_DIR to $PULSE_DIR..."
    
    # Stop old service if running
    if systemctl is-active --quiet "pulse-proxmox.service"; then
        systemctl stop "pulse-proxmox.service"
    fi
    systemctl disable "pulse-proxmox.service" 2>/dev/null || true
    
    # Backup old data if exists
    local backup_dir="/tmp/pulse-migration-$$"
    if [ -d "$OLD_PULSE_DIR/data" ]; then
        print_info "Backing up old data..."
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
        print_info "Restoring old data..."
        cp -r "$backup_dir/data"/* "$PULSE_DIR/data/" 2>/dev/null || true
        chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
        rm -rf "$backup_dir"
    fi
}

# Final instructions
show_final_instructions() {
    echo ""
    print_success "Pulse installation completed!"
    echo ""
    print_info "Access Pulse at: http://$(hostname -I | awk '{print $1}'):3000"
    print_info "Service status: systemctl status $SERVICE_NAME"
    print_info "View logs: journalctl -u $SERVICE_NAME -f"
    echo ""
}

# Main execution
main() {
    check_root
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
            print_error "Invalid operation mode"
            exit 1
            ;;
    esac
}

# Run main function
main