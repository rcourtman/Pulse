#!/bin/bash

# Pulse for Proxmox VE - Installer
# https://github.com/rcourtman/Pulse

NODE_MAJOR_VERSION=20
PULSE_DIR="/opt/pulse"
OLD_PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse"
SERVICE_NAME="pulse.service"
REPO_BASE_URL="https://github.com/rcourtman/Pulse"
SCRIPT_NAME="install-pulse.sh"
SCRIPT_RAW_URL="https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh"
CURRENT_SCRIPT_COMMIT_SHA="3721e7709c01526f0dd4f70ef622d874ac71ee49"

MODE_UPDATE=""
INSTALL_MODE=""
SPECIFIED_VERSION_TAG=""
TARGET_TAG=""
INSTALLER_WAS_REEXECUTED=false

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --update) MODE_UPDATE="true"; shift ;;
        --installer-reexecuted)
            INSTALLER_WAS_REEXECUTED=true
            shift
            ;;
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
    # Skip if running from pipe, in update mode, or already reexecuted
    if [ ! -t 0 ] || [ -n "$MODE_UPDATE" ] || [ "$INSTALLER_WAS_REEXECUTED" = "true" ]; then
        return 0
    fi
    
    if ! command -v curl &>/dev/null; then
        print_warning "curl not found, skipping installer update check"
        return 0
    fi
    
    # Install jq if needed for robust SHA checking
    if ! command -v jq &>/dev/null; then
        print_info "Installing jq for installer update check..."
        if command -v apt-get &>/dev/null; then
            apt-get update -qq >/dev/null 2>&1
            if apt-get install -y -qq jq >/dev/null 2>&1; then
                print_success "jq installed"
            else
                print_warning "Could not install jq, falling back to simple update check"
                # Fall back to simple diff check
                simple_update_check
                return $?
            fi
        else
            print_warning "Could not install jq, falling back to simple update check"
            simple_update_check
            return $?
        fi
    fi
    
    print_info "Checking for installer updates (GitHub API)..."
    
    local owner="rcourtman"
    local repo="Pulse"
    local script_path="scripts/install-pulse.sh"
    local branch="main"
    local api_url="https://api.github.com/repos/${owner}/${repo}/commits?path=${script_path}&sha=${branch}&per_page=1"
    
    local latest_sha
    latest_sha=$(curl -sL -H "Accept: application/vnd.github.v3+json" "$api_url" | jq -r 'if type=="array" and length > 0 then .[0].sha else empty end')
    
    if [ -z "$latest_sha" ] || [ "$latest_sha" = "null" ]; then
        print_warning "Could not check for updates via API"
        return 0
    fi
    
    local current_sha="$CURRENT_SCRIPT_COMMIT_SHA"
    if [ -z "$current_sha" ]; then
        print_warning "Current version unknown, skipping update check"
        return 0
    fi
    
    if [ "$latest_sha" != "$current_sha" ]; then
        print_warning "New installer version available (${latest_sha:0:7})"
        read -p "Update installer and restart? [Y/n]: " confirm
        
        if [[ ! "$confirm" =~ ^[Nn]$ ]]; then
            print_info "Downloading new installer..."
            local temp_script="/tmp/${SCRIPT_NAME}.tmp"
            
            if ! curl -sL "$SCRIPT_RAW_URL" -o "$temp_script"; then
                print_error "Failed to download new installer"
                rm -f "$temp_script"
                return 1
            fi
            
            # Update the SHA in the downloaded script
            local updated_script="${temp_script}.updated"
            local sha_updated=false
            
            while IFS= read -r line || [[ -n "$line" ]]; do
                if [[ "$line" == CURRENT_SCRIPT_COMMIT_SHA=* ]]; then
                    echo "CURRENT_SCRIPT_COMMIT_SHA=\"$latest_sha\"" >> "$updated_script"
                    sha_updated=true
                else
                    echo "$line" >> "$updated_script"
                fi
            done < "$temp_script"
            
            if [ "$sha_updated" = false ]; then
                print_error "Failed to update version in new installer"
                rm -f "$temp_script" "$updated_script"
                return 1
            fi
            
            # Replace current script
            if ! mv "$updated_script" "$0"; then
                print_error "Failed to update installer"
                rm -f "$temp_script" "$updated_script"
                return 1
            fi
            
            chmod +x "$0"
            rm -f "$temp_script"
            
            print_success "Installer updated to ${latest_sha:0:7}"
            print_info "Restarting..."
            exec "$0" --installer-reexecuted "$@"
        fi
    else
        print_success "Installer is up to date"
    fi
}

# Simple fallback update check using diff
simple_update_check() {
    local temp_script="/tmp/install-pulse-new.sh"
    
    if curl -sL "$SCRIPT_RAW_URL" -o "$temp_script" 2>/dev/null; then
        if ! diff -q "$0" "$temp_script" >/dev/null 2>&1; then
            print_warning "New installer version might be available"
            read -p "Update installer and restart? [Y/n]: " confirm
            
            if [[ ! "$confirm" =~ ^[Nn]$ ]]; then
                cp "$temp_script" "$0"
                chmod +x "$0"
                rm -f "$temp_script"
                print_success "Installer updated"
                exec "$0" "$@"
            fi
        fi
        rm -f "$temp_script"
    fi
    return 0
}

# Print welcome banner
print_welcome() {
    echo ""
    echo -e "\033[1;34mPulse for Proxmox VE Installer\033[0m"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Backup and restore functions
create_backup() {
    local backup_dir="/tmp/pulse-backup-$(date +%Y%m%d-%H%M%S)"
    
    if [ ! -d "$PULSE_DIR" ]; then
        return 1
    fi
    
    print_info "Creating backup at $backup_dir..."
    mkdir -p "$backup_dir"
    
    # Backup user data directory (includes custom thresholds, acknowledgements, metrics history)
    if [ -d "$PULSE_DIR/data" ]; then
        cp -r "$PULSE_DIR/data" "$backup_dir/"
        local data_files=$(find "$PULSE_DIR/data" -type f | wc -l)
        print_success "Backed up user data ($data_files files)"
    fi
    
    # Backup configuration files (.env contains webhook settings, alert configs, etc.)
    if [ -f "$PULSE_DIR/.env" ]; then
        cp "$PULSE_DIR/.env" "$backup_dir/"
        print_success "Backed up .env configuration"
    else
        print_info "No .env file found (fresh installation)"
    fi
    
    # Backup config directory (frontend settings, update channel preference)
    if [ -d "$PULSE_DIR/config" ]; then
        cp -r "$PULSE_DIR/config" "$backup_dir/"
        print_success "Backed up config directory"
    else
        print_info "No config directory found"
    fi
    
    # Create backup info file
    cat > "$backup_dir/backup-info.txt" << EOF
Pulse Backup Created: $(date)
Original Installation: $PULSE_DIR
Pulse Version: $(get_current_version 2>/dev/null || echo "Unknown")

Backup Contents:
$(ls -la "$backup_dir")

Data Directory Contents:
$([ -d "$backup_dir/data" ] && find "$backup_dir/data" -type f -exec basename {} \; | sort || echo "No data directory")

Configuration Summary:
- Custom Thresholds: $([ -f "$backup_dir/data/custom-thresholds.json" ] && echo "Yes" || echo "No")
- Acknowledgements: $([ -f "$backup_dir/data/acknowledgements.json" ] && echo "Yes" || echo "No")
- Environment Config: $([ -f "$backup_dir/.env" ] && echo "Yes" || echo "No")
- Frontend Settings: $([ -d "$backup_dir/config" ] && echo "Yes" || echo "No")
EOF
    
    echo "$backup_dir"
    return 0
}

detect_backups() {
    find /tmp -maxdepth 1 -name "pulse-backup-*" -type d 2>/dev/null | sort -r
}

restore_from_backup() {
    local backup_dir="$1"
    
    if [ ! -d "$backup_dir" ]; then
        print_error "Backup directory does not exist: $backup_dir"
        return 1
    fi
    
    print_info "Restoring from backup: $backup_dir"
    
    # Ensure target directories exist
    mkdir -p "$PULSE_DIR"
    chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR"
    
    # Restore user data
    if [ -d "$backup_dir/data" ]; then
        print_info "Restoring user data..."
        # Ensure data directory exists
        mkdir -p "$PULSE_DIR/data"
        # Copy contents, preserving structure
        cp -r "$backup_dir/data/"* "$PULSE_DIR/data/" 2>/dev/null || true
        chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
        
        # Verify restoration
        local restored_files=$(find "$PULSE_DIR/data" -type f 2>/dev/null | wc -l)
        print_success "User data restored ($restored_files files)"
        
        # Log specific important files
        [ -f "$PULSE_DIR/data/custom-thresholds.json" ] && print_info "✓ Custom thresholds restored"
        [ -f "$PULSE_DIR/data/acknowledgements.json" ] && print_info "✓ Alert acknowledgements restored"
    fi
    
    # Restore .env file
    if [ -f "$backup_dir/.env" ]; then
        print_info "Restoring configuration..."
        cp "$backup_dir/.env" "$PULSE_DIR/"
        chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/.env"
        chmod 600 "$PULSE_DIR/.env"
        print_success "Configuration restored"
    fi
    
    # Restore config directory
    if [ -d "$backup_dir/config" ]; then
        print_info "Restoring config directory..."
        cp -r "$backup_dir/config" "$PULSE_DIR/"
        chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/config"
        print_success "Config directory restored"
    fi
    
    return 0
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
    
    # Backup existing data and configuration if updating
    if [ -d "$PULSE_DIR/data" ]; then
        print_info "Backing up user data..."
        cp -r "$PULSE_DIR/data" "$temp_dir/data-backup"
    fi
    
    # Backup .env file if it exists
    if [ -f "$PULSE_DIR/.env" ]; then
        print_info "Backing up configuration..."
        cp "$PULSE_DIR/.env" "$temp_dir/.env-backup"
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
    
    # Restore .env file if it was backed up
    if [ -f "$temp_dir/.env-backup" ]; then
        print_info "Restoring configuration..."
        cp "$temp_dir/.env-backup" "$PULSE_DIR/.env"
        chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/.env"
        chmod 600 "$PULSE_DIR/.env"
    fi
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    print_success "Installation files ready"
    return 0
}

# Get current installed version
get_current_version() {
    # Try to get version from the running service API first
    if systemctl is-active --quiet pulse.service 2>/dev/null; then
        local api_version
        api_version=$(curl -s -m 5 "http://localhost:7655/api/version" 2>/dev/null | grep -o '"version":"[^"]*"' | cut -d'"' -f4 2>/dev/null)
        if [ -n "$api_version" ] && [ "$api_version" != "null" ]; then
            echo "$api_version"
            return 0
        fi
    fi
    
    # Fallback to package.json if API is not available
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
    # Check for any pulse-related service
    (systemctl list-unit-files | grep -q "^$SERVICE_NAME" || \
     systemctl list-unit-files | grep -q "^pulse-monitor.service" || \
     systemctl list-unit-files | grep -q "^pulse-proxmox.service") && service_exists=true
    
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
        
        # Check for existing backups
        local backups=($(detect_backups))
        if [ ${#backups[@]} -gt 0 ]; then
            echo ""
            print_info "Found ${#backups[@]} backup(s) from previous installations:"
            for i in "${!backups[@]}"; do
                local backup_dir="${backups[$i]}"
                local backup_date=$(basename "$backup_dir" | sed 's/pulse-backup-//' | sed 's/-/ /' | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\) \([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)/\3\/\2\/\1 \4:\5:\6/')
                local backup_info=""
                if [ -f "$backup_dir/backup-info.txt" ]; then
                    backup_info=" - $(grep "Pulse Version:" "$backup_dir/backup-info.txt" 2>/dev/null | cut -d: -f2 | xargs)"
                fi
                echo "  $((i+1))) $(basename "$backup_dir") (Created: $backup_date$backup_info)"
            done
            echo "  $((${#backups[@]}+1))) Fresh install (no restore)"
            echo ""
            read -p "Choose backup to restore [1-$((${#backups[@]}+1))]: " backup_choice
            
            if [[ "$backup_choice" =~ ^[0-9]+$ ]] && [ "$backup_choice" -ge 1 ] && [ "$backup_choice" -le ${#backups[@]} ]; then
                RESTORE_BACKUP="${backups[$((backup_choice-1))]}"
                print_info "Will restore from: $(basename "$RESTORE_BACKUP")"
            elif [ "$backup_choice" -eq $((${#backups[@]}+1)) ]; then
                print_info "Will perform fresh installation"
            else
                print_error "Invalid choice"
                exit 1
            fi
        fi
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
    
    # Check for polkit separately since it's a package, not a command
    if ! command -v pkexec &> /dev/null; then
        missing_deps="$missing_deps policykit-1"
    fi
    
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
PORT=7655
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
Environment="PORT=7655"

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" &>/dev/null
    
    # Setup polkit rule for sudoless updates
    setup_polkit_rule
    
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

# Setup polkit rule for sudoless service management
setup_polkit_rule() {
    print_info "Setting up polkit rule for automatic updates..."
    
    # Note: polkit is required for sudoless updates in the web interface
    # Without this, users would need to manually restart the service after updates
    
    # Create polkit rules directory if it doesn't exist
    mkdir -p /etc/polkit-1/rules.d
    
    # Ensure correct permissions (polkit installation may set restrictive permissions)
    chown root:root /etc/polkit-1/rules.d
    chmod 755 /etc/polkit-1/rules.d
    
    # Create the polkit rule
    cat > /etc/polkit-1/rules.d/10-pulse-service.rules << 'EOF'
/* Allow pulse user to manage pulse.service without password */
polkit.addRule(function(action, subject) {
    if ((action.id == "org.freedesktop.systemd1.manage-units" ||
         action.id == "org.freedesktop.systemd1.manage-unit-files" ||
         action.id == "org.freedesktop.systemd1.reload-daemon") &&
        action.lookup("unit") == "pulse.service" &&
        subject.user == "pulse") {
        return polkit.Result.YES;
    }
    return polkit.Result.NOT_HANDLED;
});
EOF
    
    # Set correct permissions
    chmod 644 /etc/polkit-1/rules.d/10-pulse-service.rules
    
    # Restart polkit to apply changes (don't fail if polkit isn't running)
    if systemctl is-active --quiet polkit 2>/dev/null; then
        systemctl restart polkit 2>/dev/null || true
    fi
    
    print_success "Polkit rule configured for automatic updates"
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
    
    # Fix Express version if needed
    print_info "Checking dependencies..."
    cd "$PULSE_DIR"
    if grep -q '"express": "4.19.2"' package.json; then
        local current_express=$(npm list express --depth=0 2>/dev/null | grep express@ | sed 's/.*express@//' || echo "")
        if [[ "$current_express" != "4.19.2" ]]; then
            print_warning "Fixing Express version mismatch..."
            rm -rf node_modules
            npm install --omit=dev || {
                print_error "Failed to install dependencies"
                exit 1
            }
        fi
    fi
    
    set_permissions
    configure_environment
    setup_systemd_service
    
    # Restore backup if selected
    if [ -n "$RESTORE_BACKUP" ]; then
        echo ""
        restore_from_backup "$RESTORE_BACKUP"
    fi
    
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
        setup_systemd_service
        systemctl start "$SERVICE_NAME"
        return 0
    fi
    
    print_info "Updating to $TARGET_TAG..."
    echo ""
    
    download_and_extract_tarball "$TARGET_TAG" || exit 1
    
    # Fix Express version if needed
    print_info "Checking dependencies..."
    cd "$PULSE_DIR"
    if grep -q '"express": "4.19.2"' package.json; then
        local current_express=$(npm list express --depth=0 2>/dev/null | grep express@ | sed 's/.*express@//' || echo "")
        if [[ "$current_express" != "4.19.2" ]]; then
            print_warning "Fixing Express version mismatch..."
            rm -rf node_modules
            npm install --omit=dev || {
                print_error "Failed to install dependencies"
                exit 1
            }
        fi
    fi
    
    set_permissions
    setup_systemd_service
    
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
    
    # Offer backup option
    if [ -d "$PULSE_DIR" ] && ([ -d "$PULSE_DIR/data" ] || [ -f "$PULSE_DIR/.env" ] || [ -d "$PULSE_DIR/config" ]); then
        echo ""
        print_info "Would you like to backup your data and configuration before removal?"
        print_info "This includes: user data, .env settings, and config files"
        read -p "Create backup? [Y/n]: " backup_confirm
        
        if [[ ! "$backup_confirm" =~ ^[Nn]$ ]]; then
            backup_path=$(create_backup)
            if [ $? -eq 0 ]; then
                print_success "Backup created at: $backup_path"
                print_info "You can restore this backup during future installations"
            else
                print_warning "Backup failed, but continuing with removal"
            fi
        fi
        echo ""
    fi
    
    read -p "Are you sure you want to remove Pulse? [y/N]: " confirm
    
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
    
    # Kill any remaining Pulse processes to prevent port conflicts
    print_info "Ensuring all Pulse processes are stopped..."
    pkill -f "/opt/pulse/server/index.js" 2>/dev/null || true
    sleep 2
    pkill -9 -f "/opt/pulse/server/index.js" 2>/dev/null || true
    
    # Remove all pulse-related service files
    rm -f "/etc/systemd/system/$SERVICE_NAME"
    rm -f "/etc/systemd/system/pulse-monitor.service"
    rm -f "/etc/systemd/system/pulse-proxmox.service"
    systemctl daemon-reload
    
    # Remove directories
    [ -d "$PULSE_DIR" ] && rm -rf "$PULSE_DIR"
    [ -d "$OLD_PULSE_DIR" ] && rm -rf "$OLD_PULSE_DIR"
    
    print_success "Pulse has been removed"
}

# Migrate from old installation
perform_migration() {
    print_info "Migrating old installation..."
    
    # Stop old service (could be either pulse-proxmox.service or pulse-monitor.service)
    if systemctl is-active --quiet "pulse-proxmox.service"; then
        systemctl stop "pulse-proxmox.service"
    fi
    if systemctl is-active --quiet "pulse-monitor.service"; then
        systemctl stop "pulse-monitor.service"
    fi
    systemctl disable "pulse-proxmox.service" &>/dev/null || true
    systemctl disable "pulse-monitor.service" &>/dev/null || true
    
    # Backup old data
    local backup_dir="/tmp/pulse-migration-$$"
    mkdir -p "$backup_dir"
    
    if [ -d "$OLD_PULSE_DIR/data" ]; then
        cp -r "$OLD_PULSE_DIR/data" "$backup_dir/"
    fi
    
    # Backup old .env file if it exists
    if [ -f "$OLD_PULSE_DIR/.env" ]; then
        cp "$OLD_PULSE_DIR/.env" "$backup_dir/.env"
    fi
    
    # Remove old installation
    rm -rf "$OLD_PULSE_DIR"
    rm -f "/etc/systemd/system/pulse-proxmox.service"
    rm -f "/etc/systemd/system/pulse-monitor.service"
    
    # Perform fresh install
    INSTALL_MODE="install"
    perform_install
    
    # Restore old data
    if [ -d "$backup_dir/data" ]; then
        print_info "Restoring data..."
        cp -r "$backup_dir/data"/* "$PULSE_DIR/data/" 2>/dev/null || true
        chown -R "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/data"
    fi
    
    # Restore old .env file
    if [ -f "$backup_dir/.env" ]; then
        print_info "Restoring configuration..."
        cp "$backup_dir/.env" "$PULSE_DIR/.env"
        chown "$PULSE_USER:$PULSE_USER" "$PULSE_DIR/.env"
        chmod 600 "$PULSE_DIR/.env"
    fi
    
    rm -rf "$backup_dir"
}

# Show final instructions
show_final_instructions() {
    local ip=$(hostname -I | awk '{print $1}')
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "Access Pulse: \033[1;36mhttp://${ip}:7655\033[0m"
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