#!/bin/bash

# Pulse for Proxmox VE LXC Installation Script
# This script automates the installation and setup of Pulse within a Proxmox LXC container.

# --- Configuration ---
NODE_MAJOR_VERSION=20 # Specify the desired Node.js major version (e.g., 18, 20)
PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse" # Dedicated user to run Pulse
SERVICE_NAME="pulse-monitor.service"
SCRIPT_NAME="install-pulse.sh" # Used for cron job identification
LOG_FILE="/var/log/pulse_update.log" # Log file for cron updates
SCRIPT_ABS_PATH="" # Store absolute path of the script here

# --- Flags ---
MODE_UPDATE=false # Flag to run in non-interactive update mode
INSTALL_MODE=""   # Stores the determined action: install, update, remove, cancel, error

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
  echo -e "\033[1;31m[ERROR]\033[0m $1 >&2
}

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo."
    exit 1
  fi
}

# --- Installation Status Check & Action Determination --- 
check_installation_status_and_determine_action() {
    # Add trace for debugging
    set -x

    # Check if running in non-interactive update mode FIRST
    if [ "$MODE_UPDATE" = true ]; then
        print_info "Running in non-interactive update mode..."
        INSTALL_MODE="update"
        set +x # Turn off trace before returning
        return 0 # Continue to main update logic
    fi

    print_info "Checking Pulse installation at $PULSE_DIR..."
    if [ -d "$PULSE_DIR" ]; then
        # Directory exists
        if [ -d "$PULSE_DIR/.git" ]; then
            # It's a git repository - Check if up-to-date
            cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
            print_info "Fetching remote information..."
            # Fetch quietly as the user might not need to see this yet
            if ! sudo -u "$PULSE_USER" git fetch origin > /dev/null 2>&1; then
                 print_warning "Could not fetch remote information to check for updates. Proceeding anyway..."
                 # Reset INSTALL_MODE, it will be set below
                 INSTALL_MODE=""
            else
                # Compare local HEAD with remote main
                local_head=$(sudo -u "$PULSE_USER" git rev-parse HEAD 2>/dev/null)
                remote_main=$(sudo -u "$PULSE_USER" git rev-parse origin/main 2>/dev/null)

                if [ "$local_head" = "$remote_main" ]; then
                    print_info "Pulse is already installed and up-to-date with the main branch."
                    INSTALL_MODE="uptodate"
                else
                    # Escape parentheses in the warning string
                    print_warning "Pulse is installed, but an update is available \(local: ${local_head:0:7}, remote: ${remote_main:0:7}\)."
                    # Set default mode to update if different
                    INSTALL_MODE="update"
                fi
            fi
            cd .. # Go back to original directory

            # Prompt based on status
            if [ "$INSTALL_MODE" = "uptodate" ]; then
                echo "Choose an action:"
                # echo "  1) Re-run installation/update process anyway" # <-- Commented out for debugging
                echo "  2) Remove Pulse"
                echo "  3) Cancel"
                read -p "Enter your choice (1-3): " user_choice
                case $user_choice in
                    1) INSTALL_MODE="update" ;; # Treat re-run as update
                    2) INSTALL_MODE="remove" ;; 
                    3) INSTALL_MODE="cancel" ;; 
                    *) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
                esac
            else # Covers case where fetch failed OR update is available
                 echo "Choose an action:"
                 echo "  1) Update Pulse to the latest version" 
                 echo "  2) Remove Pulse"
                 echo "  3) Cancel"
                 read -p "Enter your choice (1-3): " user_choice
                 case $user_choice in
                     1) INSTALL_MODE="update" ;; 
                     2) INSTALL_MODE="remove" ;; 
                     3) INSTALL_MODE="cancel" ;; 
                     *) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
                 esac
            fi

        else
            # Directory exists but isn't a git repository
            print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse git repository."
            print_error "Please remove this directory manually ($PULSE_DIR) or choose a different installation path and re-run the script."
            INSTALL_MODE="error"
        fi
    else
        # Directory doesn't exist - Offer Install/Cancel
        print_info "Pulse is not currently installed."
        echo "Choose an action:"
        echo "  1) Install Pulse"
        echo "  2) Cancel"
        read -p "Enter your choice (1-2): " user_choice

        case $user_choice in
            1) INSTALL_MODE="install" ;; 
            2) INSTALL_MODE="cancel" ;; 
            *) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
        esac
    fi
    # We don't return here, INSTALL_MODE is now set globally for the main logic

    # Ensure trace is turned off before exiting function
    set +x
}

# --- System Setup Functions --- (apt_update_upgrade, install_dependencies, setup_node, create_pulse_user)
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
    # Check if Node.js is already installed and meets version requirement
    if command -v node &> /dev/null; then
        current_node_version=$(node -v 2>/dev/null)
        current_major_version=$(echo "$current_node_version" | sed 's/v//' | cut -d. -f1)
        if [[ -n "$current_major_version" ]] && [[ "$current_major_version" -ge "$NODE_MAJOR_VERSION" ]]; then
            print_info "Node.js version ${current_node_version} already installed and meets requirement (>= v${NODE_MAJOR_VERSION}.x). Skipping setup."
            return 0
        else
            print_warning "Installed Node.js version ($current_node_version) does not meet requirement (>= v${NODE_MAJOR_VERSION}.x) or could not be determined. Proceeding with setup..."
        fi
    else
         print_info "Node.js not found. Proceeding with setup..."
    fi

    # Check if curl is installed before using it
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not found. Please install it first (apt-get install curl)."
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

# --- Core Action Functions --- (perform_update, perform_remove)
perform_update() {
    print_info "Attempting to update Pulse..."
    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    # Add safe directory config for root user, just in case
    git config --global --add safe.directory "$PULSE_DIR" > /dev/null 2>&1 || print_warning "Could not configure safe.directory for root user."

    print_info "Fetching latest changes from git (running as user $PULSE_USER)..."
    # Add --force to allow overwriting local tags if they conflict with remote after amends/force-pushes
    if ! sudo -u "$PULSE_USER" git fetch origin --tags --force; then # Ensure tags are fetched
        print_error "Failed to fetch latest changes from git."
        cd ..
        return 1
    fi

    print_info "Resetting local repository to match remote 'main' branch..."
    # Reset local main to exactly match the fetched origin/main, discarding local changes/commits
    if ! sudo -u "$PULSE_USER" git reset --hard origin/main; then
        print_error "Failed to reset local repository to origin/main."
        cd ..
        return 1
    fi

    # Get the latest tag pointing to the current HEAD
    local latest_tag
    latest_tag=$(sudo -u "$PULSE_USER" git describe --tags --abbrev=0 HEAD 2>/dev/null)

    print_info "Cleaning repository (removing untracked files)..."
    # Remove untracked files and directories to ensure a clean state
    # Use -f for files, -d for directories. Do NOT use -x which removes ignored files like .env
    if ! sudo -u "$PULSE_USER" git clean -fd; then
        print_warning "Failed to clean untracked files from the repository."
        # Continue anyway, as the core update (reset) succeeded
    fi

    # Ensure the script itself remains executable after update
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Ensuring install script ($SCRIPT_ABS_PATH) is executable..."
        if chmod +x "$SCRIPT_ABS_PATH"; then
            print_success "Install script executable permission set."
        else
            print_warning "Failed to set executable permission on install script."
            # Not necessarily fatal, continue update
        fi
    else
        print_warning "Could not find script path ($SCRIPT_ABS_PATH) to ensure executable permission."
    fi

    print_info "Re-installing dependencies using lock files..."
    # Call the updated function which now uses 'npm ci' by default
    if ! install_npm_deps; then 
        print_error "Failed to install dependencies during update."
        # Maybe don't exit immediately? Allow CSS build etc? For now, return error.
        return 1
    fi

    # Build CSS after dependencies
    print_info "Building CSS assets..."
    if ! npm run build:css > /dev/null 2>&1; then
        print_error "Failed to build CSS assets during update."
        # Potentially return 1 here if CSS build failure is critical
        # For now, just warn and continue
        print_warning "Continuing update despite CSS build failure."
    else
        print_success "CSS assets built."
    fi

    set_permissions # Ensure permissions are correct after update and build

    # Ensure the systemd service is configured correctly before restarting
    print_info "Ensuring systemd service ($SERVICE_NAME) is configured..."
    if ! setup_systemd_service true; then # Pass true to indicate update mode (skip start/enable)
        print_error "Failed to configure systemd service during update."
        # Decide if this should be fatal, likely yes as restart will fail
        return 1
    fi

    print_info "===> Attempting to restart Pulse service ($SERVICE_NAME)..."
    systemctl restart "$SERVICE_NAME"
    local restart_exit_code=$?

    if [ $restart_exit_code -eq 0 ]; then
        print_success "Pulse service restart command finished successfully (Exit code: $restart_exit_code)."
    else
        print_error "Pulse service restart command failed (Exit code: $restart_exit_code)."
        print_warning "Please check the service status manually: sudo systemctl status $SERVICE_NAME"
        print_warning "Attempting to display last 10 lines of log output:"
        # Use timeout to prevent hanging if journalctl fails
        timeout 5s journalctl -u "$SERVICE_NAME" --no-pager --lines=10 --quiet || print_error "Could not retrieve logs from journalctl."
        return 1
    fi

    # Use the detected tag in the success message
    if [ -n "$latest_tag" ]; then
      print_success "Pulse updated successfully to version $latest_tag!"
    else
      print_success "Pulse updated successfully!" # Fallback if tag detection failed
    fi
    return 0
}

perform_remove() {
    print_warning "This will stop and disable the Pulse service(s) and remove the installation directory ($PULSE_DIR)."
    # Allow non-interactive removal if called directly with logic before
    # For now, keep interactive confirmation here
    local remove_confirm=""
    if [ -t 0 ]; then # Check if stdin is a terminal for interactive prompt
      read -p "Are you sure you want to remove Pulse? (y/N): " remove_confirm
      if [[ ! "$remove_confirm" =~ ^[Yy]$ ]]; then
          print_info "Removal cancelled."
          return 1 # Indicate cancellation
      fi
    else
        print_warning "Running non-interactively. Proceeding with removal..."
    fi

    # List of potential service names to remove
    local potential_services=("pulse-monitor.service" "pulse-proxmox.service")
    local service_removed=false

    for service_name in "${potential_services[@]}"; do
        local service_file_path="/etc/systemd/system/$service_name"
        if systemctl list-units --full -all | grep -q "$service_name"; then
            print_info "Stopping service ($service_name)..."
            systemctl stop "$service_name" > /dev/null 2>&1 # Ignore errors if already stopped

            print_info "Disabling service ($service_name)..."
            systemctl disable "$service_name" > /dev/null 2>&1 # Ignore errors if already disabled

            if [ -f "$service_file_path" ]; then
                print_info "Removing systemd service file ($service_file_path)..."
                rm -f "$service_file_path"
                if [ $? -eq 0 ]; then
                    print_success "Service file $service_file_path removed."
                    service_removed=true
                else
                    print_warning "Failed to remove service file $service_file_path. Please remove it manually."
                fi
            else
                 print_info "Service file $service_file_path not found, skipping removal."
            fi
        else
             print_info "Service $service_name not found, skipping stop/disable."
             # Also check if the file exists even if service isn't loaded
             if [ -f "$service_file_path" ]; then
                 print_info "Removing orphaned systemd service file ($service_file_path)..."
                 rm -f "$service_file_path"
                 if [ $? -eq 0 ]; then
                     print_success "Orphaned service file $service_file_path removed."
                     service_removed=true
                 else
                     print_warning "Failed to remove orphaned service file $service_file_path. Please remove it manually."
                 fi
             fi
        fi
    done

    # Reload systemd daemon if any service file was removed
    if [ "$service_removed" = true ]; then
        print_info "Reloading systemd daemon..."
        systemctl daemon-reload
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

# --- Installation Step Functions --- (install_npm_deps, set_permissions, configure_environment, setup_systemd_service)
install_npm_deps() {
    print_info "Installing npm dependencies using 'npm ci'..."
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
        return 1
    fi

    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    print_info "Installing root dependencies (from lock file)..."
    # Use --unsafe-perm if running as root
    if npm ci --unsafe-perm > /dev/null 2>&1; then
        print_success "Root dependencies installed."
    else
        print_error "Failed to install root npm dependencies using 'npm ci'. Ensure package-lock.json is up-to-date."
        print_warning "Attempting fallback with 'npm install'..."
        # Fallback to install if ci fails (e.g., no lock file)
        if npm install --unsafe-perm > /dev/null 2>&1; then
             print_success "Root dependencies installed using fallback 'npm install'."
        else
             print_error "Fallback 'npm install' also failed for root dependencies."
             return 1
        fi
    fi

    print_info "Installing server dependencies (from lock file)..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
    # Check if server package-lock.json exists
    if [ ! -f "package-lock.json" ]; then
         print_warning "Server package-lock.json not found. Running 'npm install' instead of 'npm ci'."
         if npm install --unsafe-perm > /dev/null 2>&1; then
            print_success "Server dependencies installed using 'npm install'."
         else
            print_error "Failed to install server dependencies using 'npm install'."
            cd ..
            return 1
         fi
    elif npm ci --unsafe-perm > /dev/null 2>&1; then
        print_success "Server dependencies installed."
    else
        print_error "Failed to install server npm dependencies using 'npm ci'. Ensure server/package-lock.json is up-to-date."
        print_warning "Attempting fallback with 'npm install'..."
         if npm install --unsafe-perm > /dev/null 2>&1; then
             print_success "Server dependencies installed using fallback 'npm install'."
        else
             print_error "Fallback 'npm install' also failed for server dependencies."
             cd .. # Go back before returning error
             return 1
         fi
    fi

    # Return to script execution directory or root, if needed
    cd ..
    return 0
}

set_permissions() {
    print_info "Setting permissions for $PULSE_DIR..."
    if chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR"; then
        print_success "Permissions set correctly."
        return 0
    else
        print_error "Failed to set permissions for $PULSE_DIR."
        # This might not be fatal, but could cause runtime issues. Log warning?
        print_warning "Check ownership and permissions for $PULSE_DIR."
        return 1 # Indicate potential problem
    fi
}

configure_environment() {
    print_info "Configuring Pulse environment (.env)..."
    local env_example_path="$PULSE_DIR/.env.example"
    local env_path="$PULSE_DIR/.env"

    if [ ! -f "$env_example_path" ]; then
        print_error "Environment example file not found at $env_example_path. Cannot configure."
        return 1
    fi

    # --- Read existing values if .env exists --- 
    local current_host=""
    local current_token_id=""
    local current_token_secret=""
    local current_self_signed="true"
    local current_port="7655"
    local env_exists=false

    if [ -f "$env_path" ]; then
        env_exists=true
        print_info "Reading existing configuration from $env_path..."
        # Source the file in a subshell to avoid polluting current env, handle potential errors
        # Use grep and cut for robustness against file content issues
        current_host=$(grep '^PROXMOX_HOST=' "$env_path" | cut -d'=' -f2- || echo "")
        current_token_id=$(grep '^PROXMOX_TOKEN_ID=' "$env_path" | cut -d'=' -f2- || echo "")
        # Don't read/display token secret for security
        current_self_signed=$(grep '^PROXMOX_ALLOW_SELF_SIGNED_CERTS=' "$env_path" | cut -d'=' -f2- || echo "true")
        current_port=$(grep '^PORT=' "$env_path" | cut -d'=' -f2- || echo "7655")
        print_info "Current Host: $current_host"
        print_info "Current Token ID: $current_token_id"
        print_info "Current Allow Self-Signed: $current_self_signed"
        print_info "Current Port: $current_port"
    fi

    # --- Decide whether to configure --- 
    local should_configure=false
    if [ "$env_exists" = true ]; then
        if [ -t 0 ]; then # Only prompt if interactive
            read -p "Modify existing configuration? (y/N): " modify_confirm
            if [[ "$modify_confirm" =~ ^[Yy]$ ]]; then
                should_configure=true
                print_info "Proceeding to modify configuration..."
            else
                print_info "Keeping existing environment configuration."
                # Ensure correct ownership/permissions even if not modified
                chown "$PULSE_USER":"$PULSE_USER" "$env_path"
                chmod 600 "$env_path"
                return 0 
            fi
        else
            print_info "Running non-interactively, keeping existing environment configuration."
            return 0
        fi
    else
        # .env doesn't exist, proceed with configuration
        should_configure=true
    fi

    # --- Gather Proxmox Details (Interactive Only) --- 
    local proxmox_host="$current_host"
    local proxmox_token_id="$current_token_id"
    local proxmox_token_secret="" # Always prompt for secret
    local allow_self_signed="$current_self_signed"
    local pulse_port="$current_port"

    if [ "$should_configure" = true ]; then
        if [ -t 0 ]; then # Interactive prompts
            echo "Please provide/confirm your Proxmox connection details:"
            read -p " -> Proxmox Host URL [Current: $current_host]: " input_host
            # Only update if user provided input
            if [ -n "$input_host" ]; then proxmox_host="$input_host"; fi
            # Simple validation/prepend https
            if [[ -n "$proxmox_host" ]] && [[ ! "$proxmox_host" =~ ^https?:// ]]; then
                print_warning "URL does not start with http:// or https://. Prepending https://."
                proxmox_host="https://$proxmox_host"
                print_info "Using Proxmox Host URL: $proxmox_host"
            elif [ -z "$proxmox_host" ]; then
                 while [ -z "$proxmox_host" ]; do
                    print_error "Proxmox Host URL cannot be empty."
                    read -p " -> Proxmox Host URL (e.g., https://192.168.1.100:8006): " proxmox_host
                 done
                 # Re-validate after getting non-empty value
                 if [[ ! "$proxmox_host" =~ ^https?:// ]]; then 
                      print_warning "Prepending https://."
                      proxmox_host="https://$proxmox_host"
                 fi
            fi
            
            # Display token info (no change needed here)
            # ... (token info display code as before) ...
            echo ""
            print_info "You need a Proxmox API Token..."
            # ... (rest of token info display code) ...
            echo ""

            read -p " -> Proxmox API Token ID [Current: $current_token_id]: " input_token_id
            if [ -n "$input_token_id" ]; then proxmox_token_id="$input_token_id"; fi
             while [ -z "$proxmox_token_id" ]; do
                print_error "Proxmox Token ID cannot be empty."
                read -p " -> Proxmox API Token ID (e.g., user@pam!tokenid): " proxmox_token_id
            done

            # Always ask for secret, don't show current
            read -sp " -> Proxmox API Token Secret [Current: ******]: " proxmox_token_secret
            echo
             while [ -z "$proxmox_token_secret" ]; do
                print_error "Proxmox Token Secret cannot be empty."
                read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
                echo
            done

            # Optional Settings with defaults shown
            local self_signed_prompt_val="Y"
            if [ "$current_self_signed" = "false" ]; then self_signed_prompt_val="n"; fi
            read -p " -> Allow self-signed certificates for Proxmox? [Current: $self_signed_prompt_val] (Y/n): " input_self_signed
            if [[ "$input_self_signed" =~ ^[Nn]$ ]]; then allow_self_signed="false"; else allow_self_signed="true"; fi 

            read -p " -> Port for Pulse server [Current: $current_port]: " input_port
            if [ -n "$input_port" ]; then
                if [[ "$input_port" =~ ^[0-9]+$ ]] && [ "$input_port" -ge 1 ] && [ "$input_port" -le 65535 ]; then
                    pulse_port="$input_port"
                else
                    print_warning "Invalid port number entered. Keeping current value: $current_port."
                    pulse_port="$current_port" # Revert to current if invalid
                fi
            fi

        else # Non-interactive 
            print_warning "Running non-interactively. Cannot prompt for environment details."
            if [ "$env_exists" = false ]; then
                 print_warning "Creating .env from example, but it will need manual configuration."
                 # Copy example but don't substitute variables
                 cp "$env_example_path" "$env_path"
                 chown "$PULSE_USER":"$PULSE_USER" "$env_path"
                 chmod 600 "$env_path"
                 print_success "Empty environment file created at $env_path. PLEASE EDIT MANUALLY."
                 return 1 # Indicate manual intervention needed
            else
                 # Should have already returned if file existed and non-interactive
                 print_warning "Keeping existing configuration (non-interactive)."
                 return 0
            fi
        fi
    fi # End if should_configure

    # --- Create/Update .env file --- 
    print_info "Writing configuration to $env_path..."
    # Create file from scratch or overwrite existing
    # Use printf for better control over format and prevent injection
    printf "%s\n" "# Proxmox VE Connection Details" > "$env_path"
    printf "PROXMOX_HOST=%s\n" "$proxmox_host" >> "$env_path"
    printf "PROXMOX_TOKEN_ID=%s\n" "$proxmox_token_id" >> "$env_path"
    printf "PROXMOX_TOKEN_SECRET=%s\n" "$proxmox_token_secret" >> "$env_path"
    printf "PROXMOX_ALLOW_SELF_SIGNED_CERTS=%s\n" "$allow_self_signed" >> "$env_path"
    printf "\n%s\n" "# Pulse Server Settings" >> "$env_path"
    printf "PORT=%s\n" "$pulse_port" >> "$env_path"
    # Add other variables from .env.example if they exist
    # This requires parsing .env.example - more complex, skip for now 
    # Example: Append rest of example, commenting them out?
    # grep -vE '^PROXMOX_|^PORT=|^#' "$env_example_path" | sed 's/^/# /' >> "$env_path"

    chown "$PULSE_USER":"$PULSE_USER" "$env_path"
    chmod 600 "$env_path"
    print_success "Environment file updated in $env_path."
    return 0
}

setup_systemd_service() {
    # Add optional argument to skip start/enable during updates
    local update_mode=${1:-false} # Default to false if no argument passed

    print_info "Setting up systemd service ($SERVICE_NAME)..."
    local service_file="/etc/systemd/system/$SERVICE_NAME"

    # Find Node path (needed for systemd ExecStart)
    local node_path
    node_path=$(command -v node)
    if [ -z "$node_path" ]; then
        print_error "Could not find Node.js executable path. Cannot create service."
        return 1
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
             return 1
         fi
         print_info "Found npm at $npm_path"
    fi

    print_info "Creating/Updating service file at $service_file..."
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

# Load environment variables from .env file in the working directory
# Using explicit path instead of %W due to potential expansion issues
EnvironmentFile=$PULSE_DIR/.env

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
        return 1
    fi

    # Set permissions for the service file
    chmod 644 "$service_file"

    print_info "Reloading systemd daemon..."
    systemctl daemon-reload

    # Skip enable/start if in update mode
    if [ "$update_mode" = true ]; then
        print_info "(Update mode: Skipping service enable/start)"
        return 0
    fi

    print_info "Enabling $SERVICE_NAME to start on boot..."
    if systemctl enable "$SERVICE_NAME" > /dev/null 2>&1; then
        print_success "Service enabled successfully."
    else
        print_error "Failed to enable systemd service."
        # Attempt cleanup of service file?
        # rm -f "$service_file"
        return 1
    fi

    print_info "Starting $SERVICE_NAME..."
    systemctl start "$SERVICE_NAME"
    local start_exit_code=$?
    if [ $start_exit_code -eq 0 ]; then
        print_success "Service started successfully."
    else
        print_error "Failed to start systemd service (Exit code: $start_exit_code)."
        print_warning "Please check the service status using: sudo systemctl status $SERVICE_NAME"
        print_warning "Attempting to display last 10 lines of log output:"
        # Use timeout to prevent hanging if journalctl fails
        timeout 5s journalctl -u "$SERVICE_NAME" --no-pager --lines=10 --quiet || print_error "Could not retrieve logs from journalctl."
        # Don't necessarily exit, user might be able to fix it.
        return 1 # Indicate start failure
    fi
    return 0
}

# --- Final Steps Functions --- (setup_cron_update, disable_cron_update, prompt_for_cron_setup, final_instructions)

# Function to specifically disable the cron job
disable_cron_update() {
    print_info "Disabling Pulse automatic update cron job..."
    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME)"
    local escaped_cron_identifier
    escaped_cron_identifier=$(sed 's/[/.*^$]/\\&/g' <<< "$cron_identifier")

    # Get current crontab content or empty string if none exists
    current_cron=$(crontab -l -u root 2>/dev/null || true)

    # Check if the job actually exists before trying to remove
    if ! echo "$current_cron" | grep -q "^${escaped_cron_identifier}$"; then
        print_info "Pulse automatic update cron job not found. Nothing to disable."
        return 0
    fi

    # Use sed to remove the identifier line and the line immediately following it.
    filtered_cron=$(echo "$current_cron" | sed "/^${escaped_cron_identifier}$/{N;d;}")

    # Load the modified crontab content
    # Handle case where removing the job leaves the crontab empty
    if [ -z "$filtered_cron" ]; then
        echo "" | crontab -u root -
    else
        echo "$filtered_cron" | crontab -u root -
    fi

    if [ $? -eq 0 ]; then
        print_success "Cron job for automatic updates disabled successfully."
    else
        print_error "Failed to disable cron job. Please check crontab configuration manually."
        return 1
    fi
    return 0
}

setup_cron_update() {
    local cron_schedule=""
    local cron_command=""
    local script_path="$SCRIPT_ABS_PATH"
    local escaped_script_path # For grep/sed patterns
    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME)" # Identifier comment
    local escaped_cron_identifier

    if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
         print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
         return 1
    fi
    # Escape necessary characters for sed pattern matching
    escaped_cron_identifier=$(sed 's/[/.*^$]/\\&/g' <<< "$cron_identifier")

    print_info "Choose update frequency:"
    echo "  1) Daily"
    echo "  2) Weekly"
    echo "  3) Monthly"
    echo "  4) Never (Cancel)"
    read -p "Enter your choice (1-4): " freq_choice

    case $freq_choice in
        1) cron_schedule="@daily" ;;
        2) cron_schedule="@weekly" ;;
        3) cron_schedule="@monthly" ;;
        4) print_info "Automatic updates setup cancelled."; return 0 ;;
        *) print_error "Invalid choice. Skipping auto-update setup."; return 1 ;;
    esac

    # Construct the cron command
    cron_command="$cron_schedule /usr/bin/bash $script_path --update >> $LOG_FILE 2>&1"

    # --- Improved Cron Job Handling ---
    print_info "Checking/Updating cron job for Pulse automatic updates..."
    # Get current crontab content or empty string if none exists
    current_cron=$(crontab -l -u root 2>/dev/null || true)

    # Use sed to remove the identifier line and the line immediately following it.
    # The pattern looks for the exact identifier comment at the beginning of a line (^).
    # If found, it reads the Next line (N) and Deletes both (d).
    filtered_cron=$(echo "$current_cron" | sed "/^${escaped_cron_identifier}$/{N;d;}")

    # Prepare the new crontab content
    # If filtered_cron is empty after removal, avoid leading newline. Otherwise, add newline before appending.
    if [ -z "$filtered_cron" ]; then
        new_cron_content=$(printf "%s\n%s" "$cron_identifier" "$cron_command")
    else
        new_cron_content=$(printf "%s\n%s\n%s" "$filtered_cron" "$cron_identifier" "$cron_command")
    fi

    # Load the new crontab content
    echo "$new_cron_content" | crontab -u root -
    if [ $? -eq 0 ]; then
        print_success "Cron job for automatic updates set successfully."
        print_info "Update logs will be written to $LOG_FILE"
        # Ensure log file exists and is writable
        touch "$LOG_FILE" || print_warning "Could not touch log file $LOG_FILE"
        chown root:root "$LOG_FILE" || print_warning "Could not chown log file $LOG_FILE to root"
        chmod 644 "$LOG_FILE" || print_warning "Could not chmod log file $LOG_FILE"
    else
        print_error "Failed to update cron job. Please check crontab configuration manually."
        return 1
    fi
    # --- End Improved Cron Job Handling ---

    return 0
}

prompt_for_cron_setup() {
    # Only prompt if not running in update mode
    if [ "$MODE_UPDATE" = true ]; then
        return 0
    fi

    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME)"
    local cron_exists=false
    # Check if cron job exists for root user
    if crontab -l -u root 2>/dev/null | grep -q "$cron_identifier"; then
        cron_exists=true
    fi

    echo "" # Add spacing

    if [ "$cron_exists" = true ]; then
        print_info "Automatic updates for Pulse appear to be currently ENABLED."
        echo "Choose an action:"
        echo "  1) Change update schedule"
        echo "  2) Disable automatic updates"
        echo "  3) Keep current schedule (Do nothing)"
        read -p "Enter your choice (1-3): " cron_manage_choice

        case $cron_manage_choice in
            1) setup_cron_update ;; # Call function to prompt for new schedule and update
            2) disable_cron_update ;; # Call function to remove the job
            3) print_info "Keeping current automatic update schedule.";; 
            *) print_warning "Invalid choice. No changes made to automatic updates.";; 
        esac
    else
        print_info "Automatic updates for Pulse appear to be currently DISABLED."
        read -p "Do you want to set up automatic updates for Pulse? (Y/n): " setup_cron_confirm
        if [[ ! "$setup_cron_confirm" =~ ^[Nn]$ ]]; then # Proceed if not 'N' or 'n' (Default Yes)
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
    local env_path="$PULSE_DIR/.env"
    if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
        port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
    else
        port_value="7655" # Default port
    fi

    echo ""
    print_success "Pulse for Proxmox VE installation/update complete!"
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
    print_info "Configuration file: $PULSE_DIR/.env"
    echo "-------------------------------------------------------------"
}


# --- Main Execution --- Refactored
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

# Check installation status and determine user's desired action first
check_installation_status_and_determine_action # Sets the INSTALL_MODE variable

# --- Execute Action Based on INSTALL_MODE --- 
case "$INSTALL_MODE" in
    "remove")
        print_info "Proceeding with removal..."
        perform_remove
        exit $?
        ;;
    "cancel")
        print_info "Operation cancelled by user."
        exit 0
        ;;
    "error")
        # Error message already printed
        print_error "Exiting due to error."
        exit 1
        ;;
    "install" | "update")
        # Only install dependencies if installing or updating
        print_info "Proceeding with install/update. Installing prerequisites..."
        apt_update_upgrade || exit 1
        install_dependencies || exit 1
        setup_node || exit 1
        create_pulse_user || exit 1
        print_success "Prerequisites installed."

        # Now perform the specific action
        if [ "$INSTALL_MODE" = "install" ]; then
            print_info "Starting installation..."
            # --- Installation specific steps --- 
            
            # Check if target directory exists BEFORE cloning
            if [ -e "$PULSE_DIR" ]; then
                print_error "Target directory or file $PULSE_DIR already exists. Cannot clone."
                print_error "If a previous installation failed, please remove it first: sudo rm -rf $PULSE_DIR"
                exit 1
            fi
            
            print_info "Cloning Pulse repository into $PULSE_DIR..."
            if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
                print_success "Repository cloned successfully."
            else
                print_error "Failed to clone repository."
                exit 1
            fi

            install_npm_deps || exit 1 # Installs root and server (NOW INCLUDES DEV)
            
            # Build CSS after dependencies
            print_info "Building CSS assets..."
            cd "$PULSE_DIR" || { print_error "Failed to cd to $PULSE_DIR before building CSS"; exit 1; }
            if ! npm run build:css > /dev/null 2>&1; then
                print_error "Failed to build CSS assets."
                exit 1
            else
                print_success "CSS assets built."
            fi
            # Now cd back if needed, though subsequent steps might need PULSE_DIR
            # cd .. 

            set_permissions || exit 1 # Set permissions AFTER building css
            configure_environment || exit 1 # Prompt user for details
            setup_systemd_service || exit 1 # Create, enable, start service
            final_instructions
            prompt_for_cron_setup # Ask about cron on fresh install

        else # Update mode
            print_info "Starting update..."
            # --- Update specific steps --- 
            if perform_update; then
                print_success "Pulse update completed successfully."
                final_instructions
                prompt_for_cron_setup
            else
                print_error "Pulse update failed."
                exit 1
            fi
        fi
        ;;
    *)
        print_error "Internal script error: Unknown INSTALL_MODE '$INSTALL_MODE'"
        exit 1
        ;;
esac

print_info "Script finished." 