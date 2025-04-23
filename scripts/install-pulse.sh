#!/bin/bash

# Pulse for Proxmox VE LXC Installation Script
# This script automates the installation and setup of Pulse within a Proxmox LXC container.

# --- Configuration ---
NODE_MAJOR_VERSION=20 # Specify the desired Node.js major version (e.g., 18, 20)
PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse" # Dedicated user to run Pulse
SERVICE_NAME="pulse-proxmox.service"
SCRIPT_NAME="install-pulse.sh" # Used for cron job identification
LOG_FILE="/var/log/pulse_update.log" # Log file for cron updates
SCRIPT_ABS_PATH="" # Store absolute path of the script here
CRON_IDENTIFIER="# Pulse-Auto-Update ($SCRIPT_NAME)" # Identifier comment for cron job

# --- Flags ---
MODE_UPDATE=false # Flag to run in non-interactive update mode

# --- Argument Parsing ---
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --update) MODE_UPDATE=true ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

# --- Helper Functions ---
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

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo."
    exit 1
  fi
}

apt_update_upgrade() {
    print_info "Updating package lists and upgrading packages..."
    if apt-get update > /dev/null && apt-get upgrade -y > /dev/null; then
        print_success "System packages updated and upgraded."
    else
        print_error "Failed to update/upgrade system packages."
        exit 1
    fi
}

install_dependencies() {
    print_info "Installing necessary dependencies (git, curl, sudo, gpg)..."
    if apt-get install -y git curl sudo gpg > /dev/null; then
        print_success "Dependencies installed."
    else
        print_error "Failed to install dependencies."
        exit 1
    fi
}

setup_node() {
    print_info "Setting up Node.js repository (NodeSource)..."
    # Check if Node.js is already installed and meets version requirement (optional, for robustness)
    # node_version=$(node -v 2>/dev/null | sed 's/v//' | cut -d. -f1)
    # if [[ "$node_version" -ge "$NODE_MAJOR_VERSION" ]]; then
    #     print_info "Node.js version ${node_version} already installed and meets requirement (>=${NODE_MAJOR_VERSION}). Skipping setup."
    #     return 0
    # fi

    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not found. Please install it first."
        exit 1
    fi

    # Add NodeSource repository GPG key and setup script
    KEYRING_DIR="/usr/share/keyrings"
    KEYRING_FILE="$KEYRING_DIR/nodesource.gpg"
    if [ ! -d "$KEYRING_DIR" ]; then
        mkdir -p "$KEYRING_DIR" || { print_error "Failed to create $KEYRING_DIR"; exit 1; }
    fi
    print_info "Downloading/refreshing NodeSource GPG key..."
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --yes --dearmor -o "$KEYRING_FILE"
    if [ $? -ne 0 ]; then
        print_error "Failed to download or process NodeSource GPG key."
        rm -f "$KEYRING_FILE" # Clean up partial file
        exit 1
    fi

    # Add the repository configuration
    echo "deb [signed-by=$KEYRING_FILE] https://deb.nodesource.com/node_$NODE_MAJOR_VERSION.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list > /dev/null
    if [ $? -ne 0 ]; then
        print_error "Failed to add NodeSource repository to sources list."
        exit 1
    fi

    print_info "Updating package list after adding NodeSource repository..."
    if ! apt-get update > /dev/null; then
        print_error "Failed to update package list after adding NodeSource repository."
        # Attempt cleanup
        rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
        exit 1
    fi

    print_info "Installing Node.js ${NODE_MAJOR_VERSION}.x..."
    if apt-get install nodejs -y > /dev/null; then
        print_success "Node.js ${NODE_MAJOR_VERSION}.x installed successfully."
        print_info "Node version: $(node -v)"
        print_info "npm version: $(npm -v)"
    else
        print_error "Failed to install Node.js."
        # Attempt cleanup
        rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
        exit 1
    fi
}

create_pulse_user() {
    print_info "Creating dedicated user '$PULSE_USER'..."
    if id "$PULSE_USER" &>/dev/null; then
        print_warning "User '$PULSE_USER' already exists. Skipping creation."
    else
        # Create a system user with no login shell and no home directory (or specify one if needed)
        useradd -r -s /bin/false "$PULSE_USER"
        if [ $? -eq 0 ]; then
            print_success "User '$PULSE_USER' created successfully."
        else
            print_error "Failed to create user '$PULSE_USER'."
            # Decide if this is fatal. Maybe allow script to continue if user exists?
            # For now, exiting as it might indicate a problem.
            exit 1
        fi
    fi
}

# --- Function to perform update ---
perform_update() {
    print_info "Attempting to update Pulse..."
    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    # Add safe directory config for root user, in case it's needed for stash/other ops
    # Might not be strictly necessary if only pulse user runs git, but adds robustness
    git config --global --add safe.directory "$PULSE_DIR" > /dev/null 2>&1 || print_warning "Could not configure safe.directory for root user."

    print_info "Ensuring clean repository state before update..."
    # Reset any potential lingering conflicts or local changes as the pulse user
    if ! sudo -u "$PULSE_USER" git reset --hard HEAD > /dev/null 2>&1; then
        print_warning "Failed to reset repository HEAD. Update might fail."
        # Continue anyway, maybe stash/pull will work
    fi

    print_info "Stashing potential local changes (e.g., generated files)..."
    # Stash changes as the pulse user to avoid ownership issues with the stash itself
    # This is mainly for files not tracked or generated artifacts; conflicts were reset above.
    if ! sudo -u "$PULSE_USER" git stash push -m "Auto-stash before update"; then
        print_warning "Failed to stash local changes. Update might fail if conflicts exist."
        # Decide if this should be fatal or just a warning
    fi

    print_info "Fetching latest changes from git (running as user $PULSE_USER)..."
    # Run git pull as the pulse user to avoid ownership issues
    if ! sudo -u "$PULSE_USER" git pull origin main; then
        print_error "Failed to pull latest changes from git."
        # Attempt to restore stashed changes on failure
        sudo -u "$PULSE_USER" git stash pop > /dev/null 2>&1 || true # Ignore pop errors if stash failed/empty
        cd ..
        return 1
    fi

    # Attempt to pop stashed changes after successful pull
    # This might cause conflicts if the stashed changes conflict with pulled changes
    # Alternatively, could just drop the stash: git stash drop
    print_info "Attempting to restore stashed changes..."
    if ! sudo -u "$PULSE_USER" git stash pop > /dev/null 2>&1; then
        print_warning "Could not automatically restore stashed changes. Manual check might be needed if you had local modifications."
    else
        print_success "Stashed changes restored (if any)."
    fi

    print_info "Re-installing npm dependencies (root)..."
    if ! npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
        print_warning "Failed to install root npm dependencies during update. Continuing..."
        # Decide if this is fatal or just a warning
    else
        print_success "Root dependencies updated."
    fi

    print_info "Re-installing server dependencies..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
     if ! npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
        print_warning "Failed to install server npm dependencies during update. Continuing..."
        # Decide if this is fatal or just a warning
    else
        print_success "Server dependencies updated."
    fi
    cd .. # Back to PULSE_DIR

    set_permissions # Ensure permissions are correct after update

    print_info "Restarting Pulse service ($SERVICE_NAME)..."
    if systemctl restart "$SERVICE_NAME"; then
        print_success "Pulse service restarted."
    else
        print_error "Failed to restart Pulse service. Check service status manually: systemctl status $SERVICE_NAME"
        return 1
    fi

    print_success "Pulse updated successfully!"
    return 0
}

# --- Function to perform removal ---
perform_remove() {
    print_warning "This will stop and disable the Pulse service and remove the installation directory ($PULSE_DIR)."
    read -p "Are you sure you want to remove Pulse? (y/N): " remove_confirm
    if [[ ! "$remove_confirm" =~ ^[Yy]$ ]]; then
        print_info "Removal cancelled."
        return 1 # Indicate cancellation
    fi

    print_info "Stopping Pulse service ($SERVICE_NAME)..."
    systemctl stop "$SERVICE_NAME" > /dev/null 2>&1 # Ignore errors if already stopped

    print_info "Disabling Pulse service ($SERVICE_NAME)..."
    systemctl disable "$SERVICE_NAME" > /dev/null 2>&1 # Ignore errors if already disabled

    local service_file_path="/etc/systemd/system/$SERVICE_NAME"
    print_info "Removing systemd service file ($service_file_path)..."
    if [ -f "$service_file_path" ]; then
        rm -f "$service_file_path"
        if [ $? -eq 0 ]; then
            print_success "Service file removed."
            # Reload systemd daemon
            systemctl daemon-reload
        else
            print_warning "Failed to remove service file $service_file_path. Please remove it manually."
        fi
    else
        print_warning "Service file $service_file_path not found."
    fi

    print_info "Removing Pulse installation directory ($PULSE_DIR)..."
    if rm -rf "$PULSE_DIR"; then
        print_success "Installation directory removed."
    else
        print_error "Failed to remove installation directory $PULSE_DIR. Please remove it manually."
        return 1 # Indicate error
    fi

    print_success "Pulse removed successfully."
    return 0
}

clone_repository() {
    print_info "Checking Pulse installation directory $PULSE_DIR..."
    if [ -d "$PULSE_DIR" ]; then
        # Check if it's a git repository
        if [ -d "$PULSE_DIR/.git" ]; then
            # If --update flag is set, run update directly and exit
            if [ "$MODE_UPDATE" = true ]; then
                print_info "Running in non-interactive update mode..."
                if perform_update; then
                    print_success "Non-interactive update completed successfully."
                    exit 0
                else
                    print_error "Non-interactive update failed."
                    exit 1
                fi
            fi

            # Otherwise, show interactive menu
            print_warning "Pulse seems to be already installed in $PULSE_DIR."
            echo "Choose an action:"
            echo "  1) Update Pulse to the latest version"
            echo "  2) Remove Pulse"
            echo "  3) Cancel installation"
            read -p "Enter your choice (1-3): " user_choice

            case $user_choice in
                1) 
                    if perform_update; then
                        # Update was successful, allow script to continue to prompt for cron etc.
                        print_success "Interactive update completed. Continuing script..."
                    else
                        # Update failed, exit script with error
                        print_error "Interactive update failed."
                        exit 1
                    fi
                    ;;
                2)
                    if perform_remove; then
                        # Removal was successful, exit script cleanly
                        exit 0
                    else
                        # Removal failed or cancelled, exit script with error/cancel code
                        exit 1
                    fi
                    ;;
                3)
                    print_info "Installation cancelled by user."
                    exit 0
                    ;;
                *)
                    print_error "Invalid choice. Exiting."
                    exit 1
                    ;;
            esac
        else
            # Directory exists but doesn't seem to be a git repository
            print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse git repository."
            print_error "Please remove this directory manually or choose a different installation path and re-run the script."
            exit 1
        fi
    else
        # Directory doesn't exist, proceed with cloning
        print_info "Cloning Pulse repository into $PULSE_DIR..."
        # Clone the main branch. Consider adding --depth 1 for faster clone if history isn't needed.
        if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
             print_success "Repository cloned successfully."
        else
            print_error "Failed to clone repository."
            exit 1
        fi
    fi
}

install_npm_deps() {
    print_info "Installing npm dependencies..."
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
        exit 1
    fi

    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; exit 1; }

    print_info "Installing root dependencies..."
    # Use --unsafe-perm if running npm install as root, which might be necessary for some packages
    # Use --omit=dev to skip development dependencies
    if npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
        print_success "Root dependencies installed."
    else
        print_error "Failed to install root npm dependencies."
        exit 1
    fi

    print_info "Installing server dependencies..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; exit 1; }
     if npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
        print_success "Server dependencies installed."
    else
        print_error "Failed to install server npm dependencies."
        exit 1
    fi

    # Return to script execution directory or root, if needed
    cd ..
}

set_permissions() {
    print_info "Setting permissions for $PULSE_DIR..."
    if chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR"; then
        print_success "Permissions set correctly."
    else
        print_error "Failed to set permissions for $PULSE_DIR."
        # This might not be fatal, but could cause runtime issues. Log warning?
        print_warning "Check ownership and permissions for $PULSE_DIR."
    fi
}

configure_environment() {
    print_info "Configuring Pulse environment..."
    local env_example_path="$PULSE_DIR/server/.env.example"
    local env_path="$PULSE_DIR/server/.env"

    if [ ! -f "$env_example_path" ]; then
        print_error "Environment example file not found at $env_example_path. Cannot configure."
        exit 1
    fi

    # Check if .env already exists
    if [ -f "$env_path" ]; then
        print_warning "Configuration file $env_path already exists."
        read -p "Overwrite existing configuration? (y/N): " overwrite_confirm
        if [[ ! "$overwrite_confirm" =~ ^[Yy]$ ]]; then
            print_info "Skipping environment configuration."
            return 0 # Exit the function successfully without configuring
        fi
        print_info "Proceeding to overwrite existing configuration..."
    fi

    # --- Gather Proxmox Details ---
    echo "Please provide your Proxmox connection details:"
    read -p " -> Proxmox Host URL (e.g., https://192.168.1.100:8006): " proxmox_host
    while [ -z "$proxmox_host" ]; do
        print_warning "Proxmox Host URL cannot be empty."
        read -p " -> Proxmox Host URL (e.g., https://192.168.1.100:8006): " proxmox_host
    done

    # --- Display Token Generation Info ---
    echo ""
    print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
    print_info "or run the following commands on your Proxmox host shell:"
    echo "----------------------------------------------------------------------"
    echo '  # 1. Create user 'pulse-monitor' (enter password when prompted):'
    echo "  pveum useradd pulse-monitor@pam -comment \"API user for Pulse monitoring\""
    echo '  '
    echo '  # 2. Create API token 'pulse' for user (COPY THE SECRET VALUE!):'
    echo "  pveum user token add pulse-monitor@pam pulse --privsep=1"
    echo '  '
    echo '  # 3. Assign PVEAuditor role to user:'
    echo "  pveum acl modify / -user pulse-monitor@pam -role PVEAuditor"
    echo "----------------------------------------------------------------------"
    echo "After running the 'token add' command, copy the Token ID and Secret Value"
    echo "and paste them below."
    echo ""
    # --- End Token Generation Info ---

    read -p " -> Proxmox API Token ID (e.g., user@pam!tokenid): " proxmox_token_id
     while [ -z "$proxmox_token_id" ]; do
        print_warning "Proxmox Token ID cannot be empty."
        read -p " -> Proxmox API Token ID (e.g., user@pam!tokenid): " proxmox_token_id
    done

    # Use -s for silent input for the secret
    read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
    echo # Add a newline after secret input
     while [ -z "$proxmox_token_secret" ]; do
        print_warning "Proxmox Token Secret cannot be empty."
        read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
        echo
    done

    # --- Optional Settings ---
    read -p "Allow self-signed certificates for Proxmox? (y/N): " allow_self_signed
    local self_signed_value="false"
    if [[ "$allow_self_signed" =~ ^[Yy]$ ]]; then
        self_signed_value="true"
    fi

    read -p "Port for Pulse server (leave blank for default 7655): " pulse_port
    local port_value="7655"
    if [ -n "$pulse_port" ]; then
        # Basic validation: check if it's a number (optional)
        if [[ "$pulse_port" =~ ^[0-9]+$ ]] && [ "$pulse_port" -ge 1 ] && [ "$pulse_port" -le 65535 ]; then
            port_value="$pulse_port"
        else
            print_warning "Invalid port number entered. Using default 7655."
        fi
    fi

    # --- Create .env file ---
    print_info "Creating $env_path from example..."
    if cp "$env_example_path" "$env_path"; then
        # Use sed to replace placeholders. Use a different delimiter for sed if URLs contain slashes
        sed -i "s|^PROXMOX_HOST=.*|PROXMOX_HOST=$proxmox_host|" "$env_path"
        sed -i "s|^PROXMOX_TOKEN_ID=.*|PROXMOX_TOKEN_ID=$proxmox_token_id|" "$env_path"
        sed -i "s|^PROXMOX_TOKEN_SECRET=.*|PROXMOX_TOKEN_SECRET=$proxmox_token_secret|" "$env_path"
        sed -i "s|^PROXMOX_ALLOW_SELF_SIGNED_CERTS=.*|PROXMOX_ALLOW_SELF_SIGNED_CERTS=$self_signed_value|" "$env_path"
        sed -i "s|^PORT=.*|PORT=$port_value|" "$env_path"

        # Set ownership
        chown "$PULSE_USER":"$PULSE_USER" "$env_path"
        chmod 600 "$env_path" # Restrict permissions for security

        print_success "Environment configured successfully in $env_path."
    else
        print_error "Failed to copy $env_example_path to $env_path."
        exit 1
    fi
}

setup_systemd_service() {
    print_info "Setting up systemd service ($SERVICE_NAME)..."
    local service_file="/etc/systemd/system/$SERVICE_NAME"

    # Find Node path (needed for systemd ExecStart)
    local node_path
    node_path=$(command -v node)
    if [ -z "$node_path" ]; then
        print_error "Could not find Node.js executable path. Cannot create service."
        exit 1
    fi
    # Find npm path (needed for systemd ExecStart)
    local npm_path
    npm_path=$(command -v npm)
     if [ -z "$npm_path" ]; then
        print_warning "Could not find npm executable path. Service might require adjustment."
        # Try to find it relative to node? Often in ../lib/node_modules/npm/bin/npm-cli.js
        local node_dir
        node_dir=$(dirname "$node_path")
        # This is a common structure, adjust if needed
        npm_path="$node_dir/npm" # Adjust if npm is installed elsewhere
         if ! command -v "$npm_path" &> /dev/null; then
             print_error "Could not reliably find npm executable path. Cannot create service."
             exit 1
         fi
         print_info "Found npm at $npm_path"
    fi

    print_info "Creating service file at $service_file..."
    # Use a HEREDOC to write the service file contents
    cat << EOF > "$service_file"
[Unit]
Description=Pulse for Proxmox VE Monitoring Application
After=network.target

[Service]
Type=simple
User=$PULSE_USER
Group=$PULSE_USER
WorkingDirectory=$PULSE_DIR

# Start command using absolute paths found earlier
ExecStart=$node_path $npm_path run start

# Restart policy
Restart=on-failure
RestartSec=5

# Environment (optional, if needed, but .env should handle this)
# Environment="NODE_ENV=production"

# Standard output/error logging
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    if [ $? -ne 0 ]; then
        print_error "Failed to create systemd service file $service_file."
        exit 1
    fi

    # Set permissions for the service file
    chmod 644 "$service_file"

    print_info "Reloading systemd daemon..."
    systemctl daemon-reload

    print_info "Enabling $SERVICE_NAME to start on boot..."
    if systemctl enable "$SERVICE_NAME" > /dev/null 2>&1; then
        print_success "Service enabled successfully."
    else
        print_error "Failed to enable systemd service."
        # Attempt cleanup of service file?
        # rm -f "$service_file"
        exit 1
    fi

    print_info "Starting $SERVICE_NAME..."
    if systemctl start "$SERVICE_NAME"; then
        print_success "Service started successfully."
    else
        print_error "Failed to start systemd service."
        print_warning "Please check the service status using: systemctl status $SERVICE_NAME"
        print_warning "And check the logs using: journalctl -u $SERVICE_NAME"
        # Don't necessarily exit, user might be able to fix it.
    fi
}

# --- Function to remove the automatic update cron job ---
remove_cron_update() {
    print_info "Removing existing automatic update cron job..."
    # Use grep -v to filter out the identifier and the line above it (the schedule)
    # This is safer than just removing the identifier line if the schedule is separate
    (crontab -l -u root 2>/dev/null | grep -v -e "$CRON_IDENTIFIER" -e ".*$SCRIPT_NAME --update.*") | crontab -u root -
    if [ $? -ne 0 ]; then
        print_error "Failed to remove existing cron job. You may need to remove it manually (crontab -e -u root)."
        return 1
    fi
    print_success "Automatic update cron job removed."
    return 0
}

# --- Function to set up automatic updates via cron ---
setup_cron_update() {
    local cron_schedule=""
    local cron_command=""
    # Use the pre-calculated absolute path
    local script_path="$SCRIPT_ABS_PATH"

    if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
         print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
         return 1
    fi
    # Escape path for use in sed/grep (already handled by pre-calculation? Check this)
    # Re-calculate escaped path just in case
    local escaped_script_path
    escaped_script_path=$(sed 's/[&/\\]/\\\&/g' <<< "$script_path")


    print_info "Choose update frequency:"
    echo "  1) Daily"
    echo "  2) Weekly"
    echo "  3) Monthly"
    # No cancel option here, cancellation is handled before calling this function
    read -p "Enter your choice (1-3): " freq_choice

    case $freq_choice in
        1) cron_schedule="@daily" ;;
        2) cron_schedule="@weekly" ;;
        3) cron_schedule="@monthly" ;;
        *) print_error "Invalid choice. Aborting auto-update setup."; return 1 ;;
    esac

    # Construct the cron command
    # Runs as root, uses absolute path to bash and the script, redirects output
    cron_command="$cron_schedule /usr/bin/bash $script_path --update >> $LOG_FILE 2>&1"
    # Use the globally defined identifier

    # Remove potentially existing job first (ensure clean state before adding)
    print_info "Removing any previous Pulse cron schedule..."
     # Use grep -v to filter out the identifier and the line above it (the schedule)
    (crontab -l -u root 2>/dev/null | grep -v -e "$CRON_IDENTIFIER" -e ".*$SCRIPT_NAME --update.*") | crontab -u root -
     if [ $? -ne 0 ]; then
         print_warning "Could not remove existing cron job (maybe none existed). Proceeding..."
         # Continue even if removal fails, maybe crontab was empty
     fi

    # Add the new cron job
    print_info "Adding cron job with schedule: $cron_schedule"
    # Add identifier comment and the command
    (crontab -l -u root 2>/dev/null; echo "$CRON_IDENTIFIER"; echo "$cron_command") | crontab -u root -
    if [ $? -eq 0 ]; then
        print_success "Cron job for automatic updates configured successfully."
        print_info "Update logs will be written to $LOG_FILE"
        # Ensure log file exists and is writable (optional, cron might create it)
        touch "$LOG_FILE" || print_warning "Could not touch log file $LOG_FILE"
        chmod 644 "$LOG_FILE" || print_warning "Could not chmod log file $LOG_FILE"
    else
        print_error "Failed to add cron job. Please check crontab configuration."
        return 1
    fi
    return 0
}

# --- Function to prompt user about setting up cron ---
prompt_for_cron_setup() {
    # Only prompt if not running in update mode
    if [ "$MODE_UPDATE" = true ]; then
        return 0
    fi

    echo "" # Add spacing

    # Check for existing cron job
    local existing_schedule
    # Get the line *before* the identifier comment, which should be the schedule
    existing_schedule=$(crontab -l -u root 2>/dev/null | grep "$CRON_IDENTIFIER" -B 1 | head -n 1 | awk '{print $1}')

    if [ -n "$existing_schedule" ]; then
        # Attempt to normalize schedule display
        local display_schedule="$existing_schedule"
        case "$existing_schedule" in
            "@daily") display_schedule="Daily" ;;
            "@weekly") display_schedule="Weekly" ;;
            "@monthly") display_schedule="Monthly" ;;
            # Add more cases if needed for specific cron times
        esac

        print_info "Automatic updates are currently configured to run: $display_schedule"
        read -p "Do you want to change the schedule, remove it, or keep it? (change/remove/keep): " change_cron_confirm
        case "$change_cron_confirm" in
            [Cc]|[Cc]hange)
                setup_cron_update
                ;;
            [Rr]|[Rr]emove)
                remove_cron_update
                ;;
            [Kk]|[Kk]eep)
                print_info "Keeping existing automatic update schedule."
                ;;
            *)
                print_info "Invalid choice. Keeping existing automatic update schedule."
                ;;
        esac
    else
        # No existing job found
        read -p "Do you want to set up automatic updates for Pulse? (y/N): " setup_cron_confirm
        if [[ "$setup_cron_confirm" =~ ^[Yy]$ ]]; then
            setup_cron_update
        else
            print_info "Skipping automatic update setup."
        fi
    fi
}

final_instructions() {
    # Try to get the IP address of the container
    local ip_address
    ip_address=$(hostname -I | awk '{print $1}') # Get the first IP address
    local port_value
    # Read the port from the .env file, fallback to default if not found/readable
    local env_path="$PULSE_DIR/server/.env"
    if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
        port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
    else
        port_value="7655" # Default port
    fi

    echo ""
    print_success "Pulse for Proxmox VE installation and setup complete!"
    echo "-------------------------------------------------------------"
    print_info "You should be able to access the Pulse dashboard at:"
    if [ -n "$ip_address" ]; then
        echo "  http://$ip_address:$port_value"
    else
        echo "  http://<YOUR-LXC-IP>:$port_value" 
        print_warning "Could not automatically determine the LXC IP address."
    fi
    echo ""
    print_info "The Pulse service ($SERVICE_NAME) is running and enabled on boot."
    print_info "To check the status: sudo systemctl status $SERVICE_NAME"
    print_info "To view logs: sudo journalctl -u $SERVICE_NAME -f"
    print_info "Configuration file: $PULSE_DIR/server/.env"
    echo "-------------------------------------------------------------"
}

# --- Main Execution ---
check_root

# Determine absolute path of the script early
if command -v readlink &> /dev/null && readlink -f "$0" &> /dev/null; then
    SCRIPT_ABS_PATH=$(readlink -f "$0")
else
    # Basic fallback if readlink -f is not available or fails
     if [[ "$0" == /* ]]; then
        SCRIPT_ABS_PATH="$0"
     else
        SCRIPT_ABS_PATH="$(pwd)/$0"
     fi
     # Verify the fallback path
     if [ ! -f "$SCRIPT_ABS_PATH" ]; then
          print_warning "Warning: Could not reliably determine absolute script path using fallback."
          SCRIPT_ABS_PATH="" # Clear path if unsure
     fi
fi

# Execute main functions
apt_update_upgrade
install_dependencies
setup_node
create_pulse_user
clone_repository # This function now handles the update flag or interactive menu

# The following steps are skipped if clone_repository exits due to update/remove/cancel
install_npm_deps
set_permissions
configure_environment # Might be skipped if user chooses not to overwrite existing .env
setup_systemd_service
final_instructions

# Ask about setting up cron job at the end
prompt_for_cron_setup

print_info "Script finished."

# Remove placeholder lines
# echo ""
# print_success "Script execution will continue..." # Placeholder 