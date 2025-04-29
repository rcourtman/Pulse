#!/bin/bash

# Pulse for Proxmox VE LXC Installation Script
# This script automates the installation and setup of Pulse within a Proxmox LXC container.

# --- Configuration ---
NODE_MAJOR_VERSION=20 # Specify the desired Node.js major version (e.g., 18, 20*)
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
        --update*) MODE_UPDATE=true ;;
        **) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

# --- Helper Functions ---
print_info() {
  echo -e "\033[1;34m[INFO]\033[0m $1"
}

print_success(*) {
  echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

print_warning(*) {
  echo -e "\033[1;33m[WARNING]\033[0m $1"
}

print_error(*) {
  echo -e "\033[1;31m[ERROR]\033[0m $1 >&2
}

check_root(*) {
  if [ "$(id -u*)" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo."
    exit 1
  fi
}

# --- Installation Status Check & Action Determination --- 
check_installation_status_and_determine_action(*) {
    # Check if running in non-interactive update mode FIRST
    if [ "$MODE_UPDATE" = true ]; then
        print_info "Running in non-interactive update mode..."
        INSTALL_MODE="update"
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
                local_head=$(sudo -u "$PULSE_USER" git rev-parse HEAD 2>/dev/null*)
                remote_main=$(sudo -u "$PULSE_USER" git rev-parse origin/main 2>/dev/null*)

                if [ "$local_head" = "$remote_main" ]; then
                    print_info "Pulse is already installed and up-to-date with the main branch."
                    INSTALL_MODE="uptodate"
                else
                    # Escape parentheses in the warning string
                    print_warning "Pulse is installed, but an update is available \(local: ${local_head:0:7}, remote: ${remote_main:0:7}*)."
                    # Set default mode to update if different
                    INSTALL_MODE="update"
                fi
            fi
            cd .. # Go back to original directory

            # Prompt based on status
            if [ "$INSTALL_MODE" = "uptodate" ]; then
                echo "Choose an action:"
                echo "  1*) Re-run installation/update process anyway"
                echo "  2*) Remove Pulse"
                echo "  3*) Cancel"
                read -p "Enter your choice (1-3*): " user_choice
                case $user_choice in
                    1*) INSTALL_MODE="update" ;; # Treat re-run as update
                    2*) INSTALL_MODE="remove" ;; 
                    3*) INSTALL_MODE="cancel" ;; 
                    **) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
                esac
            else # Covers case where fetch failed OR update is available
                 echo "Choose an action:"
                 echo "  1*) Update Pulse to the latest version" 
                 echo "  2*) Remove Pulse"
                 echo "  3*) Cancel"
                 read -p "Enter your choice (1-3*): " user_choice
                 case $user_choice in
                     1*) INSTALL_MODE="update" ;; 
                     2*) INSTALL_MODE="remove" ;; 
                     3*) INSTALL_MODE="cancel" ;; 
                     **) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
                 esac
            fi

        else
            # Directory exists but isn't a git repository
            print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse git repository."
            print_error "Please remove this directory manually ($PULSE_DIR*) or choose a different installation path and re-run the script."
            INSTALL_MODE="error"
        fi
    else
        # Directory doesn't exist - Offer Install/Cancel
        print_info "Pulse is not currently installed."
        echo "Choose an action:"
        echo "  1*) Install Pulse"
        echo "  2*) Cancel"
        read -p "Enter your choice (1-2*): " user_choice

        case $user_choice in
            1*) INSTALL_MODE="install" ;; 
            2*) INSTALL_MODE="cancel" ;; 
            **) print_error "Invalid choice."; INSTALL_MODE="error" ;; 
        esac
    fi
    # We don't return here, INSTALL_MODE is now set globally for the main logic
}

# --- System Setup Functions --- (apt_update_upgrade, install_dependencies, setup_node, create_pulse_user*)
apt_update_upgrade(*) {
    print_info "Updating package lists and upgrading packages..."
    if apt-get update > /dev/null && apt-get upgrade -y > /dev/null; then
        print_success "System packages updated and upgraded."
    else
        print_error "Failed to update/upgrade system packages."
        exit 1
    fi
}

install_dependencies(*) {
    print_info "Installing necessary dependencies (git, curl, sudo, gpg*)..."
    if apt-get install -y git curl sudo gpg > /dev/null; then
        print_success "Dependencies installed."
    else
        print_error "Failed to install dependencies."
        exit 1
    fi
}

setup_node(*) {
    print_info "Setting up Node.js repository (NodeSource*)..."
    # Check if Node.js is already installed and meets version requirement
    if command -v node &> /dev/null; then
        current_node_version=$(node -v 2>/dev/null*)
        current_major_version=$(echo "$current_node_version" | sed 's/v//' | cut -d. -f1*)
        if [[ -n "$current_major_version" ]] && [[ "$current_major_version" -ge "$NODE_MAJOR_VERSION" ]]; then
            print_info "Node.js version ${current_node_version} already installed and meets requirement (>= v${NODE_MAJOR_VERSION}.x*). Skipping setup."
            return 0
        else
            print_warning "Installed Node.js version ($current_node_version*) does not meet requirement (>= v${NODE_MAJOR_VERSION}.x*) or could not be determined. Proceeding with setup..."
        fi
    else
         print_info "Node.js not found. Proceeding with setup..."
    fi

    # Check if curl is installed before using it
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not found. Please install it first (apt-get install curl*)."
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
        print_info "Node version: $(node -v*)"
        print_info "npm version: $(npm -v*)"
    else
        print_error "Failed to install Node.js."
        # Attempt cleanup
        rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
        exit 1
    fi
}

create_pulse_user(*) {
    print_info "Creating dedicated user '$PULSE_USER'..."
    if id "$PULSE_USER" &>/dev/null; then
        print_warning "User '$PULSE_USER' already exists. Skipping creation."
    else
        # Create a system user with no login shell and no home directory (or specify one if needed*)
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

# --- Core Action Functions --- (perform_update, perform_remove*)
perform_update(*) {
    print_info "Attempting to update Pulse..."
    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    # Add safe directory config for root user, just in case
    git config --global --add safe.directory "$PULSE_DIR" > /dev/null 2>&1 || print_warning "Could not configure safe.directory for root user."

    print_info "Fetching latest changes from git (running as user $PULSE_USER*)..."
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
    latest_tag=$(sudo -u "$PULSE_USER" git describe --tags --abbrev=0 HEAD 2>/dev/null*)

    print_info "Cleaning repository (removing untracked files*)..."
    # Remove untracked files and directories to ensure a clean state
    # Use -f for files, -d for directories. Do NOT use -x which removes ignored files like .env
    if ! sudo -u "$PULSE_USER" git clean -fd; then
        print_warning "Failed to clean untracked files from the repository."
        # Continue anyway, as the core update (reset*) succeeded
    fi

    # Ensure the script itself remains executable after update
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Ensuring install script ($SCRIPT_ABS_PATH*) is executable..."
        if chmod +x "$SCRIPT_ABS_PATH"; then
            print_success "Install script executable permission set."
        else
            print_warning "Failed to set executable permission on install script."
            # Not necessarily fatal, continue update
        fi
    else
        print_warning "Could not find script path ($SCRIPT_ABS_PATH*) to ensure executable permission."
    fi

    print_info "Re-installing npm dependencies (root*)..."
    # Keep --omit=dev here, we'll install cli separately
    # Use npm ci for faster, cleaner installs based on lock file
    if ! npm ci --unsafe-perm > /dev/null 2>&1; then
        print_warning "Failed to install root npm dependencies using npm ci. Trying npm install..."
        # Fallback to npm install if ci fails (e.g., no lock file initially*)
        if ! npm install --unsafe-perm > /dev/null 2>&1; then
            print_warning "Fallback npm install also failed for root dependencies. Continuing..."
        else
            print_success "Root dependencies installed via fallback npm install."
        fi
    else
        print_success "Root dependencies updated using npm ci."
    fi

    print_info "Re-installing server dependencies..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
    # Use npm ci for faster, cleaner installs based on lock file
     if npm ci --unsafe-perm > /dev/null 2>&1; then
        print_success "Server dependencies updated using npm ci."
     else
         print_warning "Failed to install server dependencies using npm ci. Trying npm install..."
         # Fallback to npm install if ci fails
         if ! npm install --unsafe-perm > /dev/null 2>&1; then
             print_error "Fallback npm install also failed for server dependencies."
             exit 1 # Exit if server deps fail even with fallback
         else
            print_success "Server dependencies installed via fallback npm install."
         fi
    fi
    cd .. # Back to PULSE_DIR

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
    print_info "Ensuring systemd service ($SERVICE_NAME*) is configured..."
    if ! setup_systemd_service true; then # Pass true to indicate update mode (skip start/enable*)
        print_error "Failed to configure systemd service during update."
        # Decide if this should be fatal, likely yes as restart will fail
        return 1
    fi

    print_info "===> Attempting to restart Pulse service ($SERVICE_NAME*)..."
    systemctl restart "$SERVICE_NAME"
    local restart_exit_code=$?

    if [ $restart_exit_code -eq 0 ]; then
        print_success "Pulse service restart command finished successfully (Exit code: $restart_exit_code*)."
    else
        print_error "Pulse service restart command failed (Exit code: $restart_exit_code*)."
        print_info "Attempting to display the last 20 lines of the service log:"
        # Use --no-pager to prevent blocking in script
        journalctl -u "$SERVICE_NAME" -n 20 --no-pager
        print_warning "Please check the service status manually: sudo systemctl status $SERVICE_NAME"
        # Keep existing warning about journalctl as well, in case the direct output fails
        print_warning "And check logs: sudo journalctl -u $SERVICE_NAME"
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

perform_remove(*) {
    print_warning "This will stop and disable the Pulse service(s*) and remove the installation directory ($PULSE_DIR*)."
    # Allow non-interactive removal if called directly with logic before
    # For now, keep interactive confirmation here
    local remove_confirm=""
    if [ -t 0 ]; then # Check if stdin is a terminal for interactive prompt
      read -p "Are you sure you want to remove Pulse? (y/N*): " remove_confirm
      if [[ ! "$remove_confirm" =~ ^[Yy]$ ]]; then
          print_info "Removal cancelled."
          return 1 # Indicate cancellation
      fi
    else
        print_warning "Running non-interactively. Proceeding with removal..."
    fi

    # List of potential service names to remove
    local potential_services=("pulse-monitor.service" "pulse-proxmox.service"*)
    local service_removed=false

    for service_name in "${potential_services[@]}"; do
        local service_file_path="/etc/systemd/system/$service_name"
        if systemctl list-units --full -all | grep -q "$service_name"; then
            print_info "Stopping service ($service_name*)..."
            systemctl stop "$service_name" > /dev/null 2>&1 # Ignore errors if already stopped

            print_info "Disabling service ($service_name*)..."
            systemctl disable "$service_name" > /dev/null 2>&1 # Ignore errors if already disabled

            if [ -f "$service_file_path" ]; then
                print_info "Removing systemd service file ($service_file_path*)..."
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
                 print_info "Removing orphaned systemd service file ($service_file_path*)..."
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

    print_info "Removing Pulse installation directory ($PULSE_DIR*)..."
    if rm -rf "$PULSE_DIR"; then
        print_success "Installation directory removed."
    else
        print_error "Failed to remove installation directory $PULSE_DIR. Please remove it manually."
        return 1 # Indicate error
    fi

    print_success "Pulse removed successfully."
    return 0
}

# --- Installation Step Functions --- (install_npm_deps, set_permissions, configure_environment, setup_systemd_service*)
install_npm_deps(*) {
    print_info "Installing npm dependencies..."
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
        # This shouldn't happen if called in the right flow, but check anyway
        return 1
    fi

    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    print_info "Installing root dependencies (including dev*)..."
    # Use --unsafe-perm if running npm install as root, which might be necessary for some packages
    # REMOVED --omit=dev to ensure build tools like postcss/autoprefixer are present
    if npm install --unsafe-perm > /dev/null 2>&1; then
        print_success "Root dependencies installed."
    else
        print_error "Failed to install root npm dependencies."
        return 1
    fi

    print_info "Installing server dependencies (including dev*)..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
    # REMOVED --omit=dev
     if npm install --unsafe-perm > /dev/null 2>&1; then
        print_success "Server dependencies installed."
    else
        print_error "Failed to install server npm dependencies."
        cd .. # Go back before returning error
        return 1
    fi

    # Return to script execution directory or root, if needed
    cd ..
    return 0
}

set_permissions(*) {
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

# --- Helper function to safely read .env files ---
# Reads a .env file, ignoring comments and empty lines, populating an associative array.
# Usage: declare -A env_vars; read_env_file "path/to/.env" env_vars
read_env_file(*) {
    local env_file="$1"
    local -n arr="$2" # Use nameref for the associative array
    arr=(*) # Clear the array
    if [ -f "$env_file" ]; then
        while IFS= read -r line || [[ -n "$line" ]]; do
            # Trim leading/trailing whitespace
            line=$(echo "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'*)
            # Skip empty lines and comments
            if [[ -z "$line" || "$line" == \#* ]]; then
                continue
            fi
            # Simple split on the first '=', allows values to contain '='
            local key="${line%%=*}"
            local value="${line#*=}"
            # Trim potential quotes from value (basic handling*)
            value=$(echo "$value" | sed -e 's/^"//' -e 's/"$/' -e "s/^'//" -e "s/'$//"*)
            arr["$key"]="$value"
        done < "$env_file"
    fi
}

# --- Helper function to prompt with default ---
# Usage: prompt_with_default VARIABLE_NAME PROMPT_TEXT [CURRENT_VALUE]
prompt_with_default(*) {
    local var_name="$1"
    local prompt_text="$2"
    local current_value="$3" # Optional current value
    local user_input

    if [ -n "$current_value" ]; then
        read -p "$prompt_text [$current_value]: " user_input
        # If user pressed Enter without typing, use the current value
        if [ -z "$user_input" ]; then
            printf -v "$var_name" '%s' "$current_value"
        else
            printf -v "$var_name" '%s' "$user_input"
        fi
    else
        # No current value, standard prompt
        read -p "$prompt_text: " user_input
        printf -v "$var_name" '%s' "$user_input"
    fi
}

# --- Helper function to prompt for secret with default ---
# Usage: prompt_secret_with_default VARIABLE_NAME PROMPT_TEXT [HAS_CURRENT_VALUE]
prompt_secret_with_default(*) {
    local var_name="$1"
    local prompt_text="$2"
    local has_current_value=$3 # Boolean (true/false*) indicates if a value exists
    local user_input

    if [ "$has_current_value" = true ]; then
         # Ask if they want to change it first
        read -p "$prompt_text [current value hidden, press Enter to keep, or type 'c' to change]: " change_choice
        if [[ "$change_choice" =~ ^[Cc]$ ]]; then
            read -sp " -> Enter new secret: " user_input
            echo
            # Assign the new value
             printf -v "$var_name" '%s' "$user_input"
            # Set a flag or return value indicating it was changed (optional, handled by direct assignment here*)
        else
            # Indicate value should be kept - assign a special value or handle upstream
             printf -v "$var_name" '%s' "__KEEP_SECRET__" # Special marker
        fi

    else
        # No current value, standard secret prompt
        read -sp "$prompt_text: " user_input
        echo
        printf -v "$var_name" '%s' "$user_input"
    fi
}

configure_environment(*) {
    print_info "Configuring Pulse environment..."
    local env_example_path="$PULSE_DIR/.env.example"
    local env_path="$PULSE_DIR/.env"

    if [ ! -f "$env_example_path" ]; then
        print_error "Environment example file not found at $env_example_path. Cannot configure."
        return 1
    fi

    # Declare associative array to hold env vars
    declare -A current_env_vars
    local env_file_exists=false
    if [ -f "$env_path" ]; then
        env_file_exists=true
        read_env_file "$env_path" current_env_vars
    fi

    # Define the primary keys we need to configure
    local config_keys=("PROXMOX_HOST" "PROXMOX_TOKEN_ID" "PROXMOX_TOKEN_SECRET" "PROXMOX_ALLOW_SELF_SIGNED_CERTS" "PORT"*)

    # Variables to store collected values
    local proxmox_host=""
    local proxmox_token_id=""
    local proxmox_token_secret=""
    local allow_self_signed=""
    local pulse_port=""

    local needs_update=false # Flag to track if we need to write the file

    if [ -t 0 ]; then # Interactive mode
        if [ "$env_file_exists" = true ]; then
            print_warning "Configuration file $env_path already exists."
            echo "Review the current settings and update if necessary."
            needs_update=true # Assume update is needed if file exists interactively

            # --- Get Proxmox Host ---
            prompt_with_default proxmox_host " -> Proxmox Host URL" "${current_env_vars[PROXMOX_HOST]}"
            while [ -z "$proxmox_host" ]; do
                print_warning "Proxmox Host URL cannot be empty."
                prompt_with_default proxmox_host " -> Proxmox Host URL" "${current_env_vars[PROXMOX_HOST]}"
            done
            if [[ ! "$proxmox_host" =~ ^https?:// ]]; then
                 print_warning "URL does not start with http:// or https://. Prepending https://."
                 proxmox_host="https://$proxmox_host"
                 print_info "Using Proxmox Host URL: $proxmox_host"
            fi

             # --- Display Token Generation Info --- (Only if Token ID or Secret is missing/being changed*)
             if [ -z "${current_env_vars[PROXMOX_TOKEN_ID]}" ] || [ -z "${current_env_vars[PROXMOX_TOKEN_SECRET]}" ]; then
                 echo ""
                 print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
                 print_info "or run the following commands on your Proxmox host shell:"
                 echo "----------------------------------------------------------------------"
                 echo '  # 1. Create user 'pulse-monitor' (enter password when prompted*):'
                 echo "  pveum useradd pulse-monitor@pam -comment "API user for Pulse monitoring""
                 echo '  '
                 echo '  # 2. Create API token 'pulse' for user (COPY THE SECRET VALUE!*):'
                 echo "  pveum user token add pulse-monitor@pam pulse --privsep=1"
                 echo '  '
                 echo '  # 3. Assign PVEAuditor role to user:'
                 echo "  pveum acl modify / -user pulse-monitor@pam -role PVEAuditor"
                 echo "----------------------------------------------------------------------"
                 echo "After running the 'token add' command, copy the Token ID and Secret Value"
                 echo "and paste them below."
                 echo ""
             fi

             # --- Get Proxmox Token ID ---
             prompt_with_default proxmox_token_id " -> Proxmox API Token ID" "${current_env_vars[PROXMOX_TOKEN_ID]}"
             while [ -z "$proxmox_token_id" ]; do
                 print_warning "Proxmox Token ID cannot be empty."
                 prompt_with_default proxmox_token_id " -> Proxmox API Token ID" "${current_env_vars[PROXMOX_TOKEN_ID]}"
             done

            # --- Get Proxmox Token Secret ---
            local has_secret=false
            if [ -n "${current_env_vars[PROXMOX_TOKEN_SECRET]}" ]; then has_secret=true; fi
            prompt_secret_with_default proxmox_token_secret " -> Proxmox API Token Secret" "$has_secret"
            if [ "$proxmox_token_secret" = "__KEEP_SECRET__" ]; then
                 proxmox_token_secret="${current_env_vars[PROXMOX_TOKEN_SECRET]}" # Keep existing secret
            fi
            while [ -z "$proxmox_token_secret" ]; do
                 print_warning "Proxmox Token Secret cannot be empty."
                 # Re-prompt without default if it was empty
                 read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
                 echo
            done

            # --- Get Self-Signed Cert Preference ---
            local current_self_signed="${current_env_vars[PROXMOX_ALLOW_SELF_SIGNED_CERTS]}"
            local self_signed_prompt="Allow self-signed certificates? (y/N*)"
            local self_signed_default_display=""
            if [ "$current_self_signed" = "true" ]; then
                self_signed_prompt="Allow self-signed certificates? (Y/n*)"
                self_signed_default_display="Y"
             elif [ "$current_self_signed" = "false" ]; then
                 self_signed_default_display="n"
            fi
            read -p " -> $self_signed_prompt [$self_signed_default_display]: " allow_self_signed_input
            # Determine final value based on input or default
            if [ -z "$allow_self_signed_input" ]; then
                 # Use current value if input is empty
                 allow_self_signed="$current_self_signed"
            elif [[ "$allow_self_signed_input" =~ ^[Yy]$ ]]; then
                allow_self_signed="true"
            elif [[ "$allow_self_signed_input" =~ ^[Nn]$ ]]; then
                allow_self_signed="false"
            else
                print_warning "Invalid input. Keeping current value: $current_self_signed"
                allow_self_signed="$current_self_signed"
            fi
            # Handle case where current value was not 'true' or 'false'
            if [[ "$allow_self_signed" != "true" && "$allow_self_signed" != "false" ]]; then
                 print_warning "Invalid or missing current value for self-signed certs. Defaulting to 'true' (Yes*)."
                 allow_self_signed="true" # Default to true if invalid/missing
            fi

             # --- Get Pulse Port ---
             local current_port="${current_env_vars[PORT]:-7655}" # Default to 7655 if not set
             prompt_with_default pulse_port " -> Port for Pulse server" "$current_port"
             if [ -n "$pulse_port" ]; then
                if ! [[ "$pulse_port" =~ ^[0-9]+$ ]] || [ "$pulse_port" -lt 1 ] || [ "$pulse_port" -gt 65535 ]; then
                     print_warning "Invalid port number entered ($pulse_port*). Using current/default: $current_port."
                     pulse_port="$current_port"
                 fi
             else
                 # User hit enter, keep current/default
                 pulse_port="$current_port"
             fi

        else # Interactive mode, but .env doesn't exist
             print_info "No existing configuration file found. Please provide details:"
             needs_update=true # Will create the file

             # --- Gather Proxmox Details (Original Logic*) ---
             read -p " -> Proxmox Host URL (e.g., https://192.168.1.100:8006*): " proxmox_host
             while [ -z "$proxmox_host" ]; do
                 print_warning "Proxmox Host URL cannot be empty."
                 read -p " -> Proxmox Host URL (e.g., https://192.168.1.100:8006*): " proxmox_host
             done
             if [[ ! "$proxmox_host" =~ ^https?:// ]]; then
                  print_warning "URL does not start with http:// or https://. Prepending https://."
                  proxmox_host="https://$proxmox_host"
                  print_info "Using Proxmox Host URL: $proxmox_host"
             fi

             # --- Display Token Generation Info ---
             echo ""
             print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
             echo "or run the following commands on your Proxmox host shell:"
             echo "----------------------------------------------------------------------"
             echo '  # 1. Create user 'pulse-monitor'...'
             echo "  pveum useradd pulse-monitor@pam ..."
             echo '  '
             echo '  # 2. Create API token 'pulse'...'
             echo "  pveum user token add pulse-monitor@pam pulse ..."
             echo '  '
             echo '  # 3. Assign PVEAuditor role...'
             echo "  pveum acl modify / -user pulse-monitor@pam ..."
             echo "----------------------------------------------------------------------"
             echo "Paste the Token ID and Secret Value below."
             echo ""

             read -p " -> Proxmox API Token ID (e.g., user@pam!tokenid*): " proxmox_token_id
             while [ -z "$proxmox_token_id" ]; do
                 print_warning "Proxmox Token ID cannot be empty."
                 read -p " -> Proxmox API Token ID: " proxmox_token_id
             done

             read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
             echo
             while [ -z "$proxmox_token_secret" ]; do
                 print_warning "Proxmox Token Secret cannot be empty."
                 read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
                 echo
             done

             # --- Optional Settings ---
             local allow_self_signed_input=""
             read -p " -> Allow self-signed certificates for Proxmox? (Y/n*): " allow_self_signed_input
             if [[ "$allow_self_signed_input" =~ ^[Nn]$ ]]; then
                 allow_self_signed="false"
             else
                 allow_self_signed="true" # Default to true
             fi

             local pulse_port_input=""
             read -p " -> Port for Pulse server (leave blank for default 7655*): " pulse_port_input
             if [ -n "$pulse_port_input" ]; then
                if [[ "$pulse_port_input" =~ ^[0-9]+$ ]] && [ "$pulse_port_input" -ge 1 ] && [ "$pulse_port_input" -le 65535 ]; then
                     pulse_port="$pulse_port_input"
                else
                     print_warning "Invalid port number entered. Using default 7655."
                     pulse_port="7655"
                 fi
             else
                pulse_port="7655" # Default if blank
             fi
        fi

    elif [ "$env_file_exists" = false ]; then # Non-interactive and file doesn't exist
        print_warning "Running non-interactively and $env_path does not exist."
        print_warning "Attempting to create from example, but values will be defaults."
        print_warning "Please edit $env_path manually with your specific configuration."
        # Set default values so the file creation logic works
        proxmox_host=""
        proxmox_token_id=""
        proxmox_token_secret=""
        allow_self_signed="true" # Default
        pulse_port="7655" # Default
        needs_update=true
    else
        # Non-interactive and file exists - do nothing, skip update
        print_info "Running non-interactively, existing $env_path found. Skipping modifications."
        needs_update=false
    fi

    # --- Write .env file if changes were made or file is new ---
    if [ "$needs_update" = true ]; then
        print_info "Writing configuration to $env_path..."
        # Create a temporary file first
        local temp_env_path="$env_path.tmp"

        # Copy example to temp if the original doesn't exist, otherwise copy original
        # This preserves any extra variables the user might have added
        if [ "$env_file_exists" = true ]; then
            cp "$env_path" "$temp_env_path" || { print_error "Failed to copy existing .env to temp file."; return 1; }
        else
            cp "$env_example_path" "$temp_env_path" || { print_error "Failed to copy .env.example to temp file."; return 1; }
        fi

        # Use sed to update or add the specific variables we manage
        # Using a loop and pattern matching for robustness
        for key in "${config_keys[@]}"; do
            local value=""
            case $key in
                PROXMOX_HOST*) value="$proxmox_host" ;;
                PROXMOX_TOKEN_ID*) value="$proxmox_token_id" ;;
                PROXMOX_TOKEN_SECRET*) value="$proxmox_token_secret" ;;
                PROXMOX_ALLOW_SELF_SIGNED_CERTS*) value="$allow_self_signed" ;;
                PORT*) value="$pulse_port" ;;
            esac

            # Escape sed replacement characters in the value (especially / and &*)
            local escaped_value=$(sed -e 's/[\\/&]/\\\\&/g' <<< "$value"*)

            # Check if key exists and update, otherwise append
            # Use a different delimiter for sed like '#' to avoid clashes with URL slashes
            if grep -qE "^${key}=" "$temp_env_path"; then
                 # Use '#' as delimiter for sed
                 sed -i "s#^${key}=.*#${key}=${escaped_value}#" "$temp_env_path"
            else
                echo "${key}=${value}" >> "$temp_env_path"
            fi
        done

        # Replace original with temp file
        if mv "$temp_env_path" "$env_path"; then
            chown "$PULSE_USER":"$PULSE_USER" "$env_path"
            chmod 600 "$env_path"
            print_success "Environment file updated/created in $env_path."
        else
            print_error "Failed to replace $env_path with updated version."
            rm -f "$temp_env_path" # Clean up temp file
            return 1
        fi
    fi

    return 0
}

setup_systemd_service(*) {
    # Add optional argument to skip start/enable during updates
    local update_mode=${1:-false} # Default to false if no argument passed

    print_info "Setting up systemd service ($SERVICE_NAME*)..."
    local service_file="/etc/systemd/system/$SERVICE_NAME"

    # Find Node path (needed for systemd ExecStart*)
    local node_path
    node_path=$(command -v node*)
    if [ -z "$node_path" ]; then
        print_error "Could not find Node.js executable path. Cannot create service."
        return 1
    fi
    # Find npm path (needed for systemd ExecStart*)
    local npm_path
    npm_path=$(command -v npm*)
     if [ -z "$npm_path" ]; then
        print_warning "Could not find npm executable path. Service might require adjustment."
        # Try to find it relative to node? Often in ../lib/node_modules/npm/bin/npm-cli.js
        local node_dir
        node_dir=$(dirname "$node_path"*)
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

# Environment (optional, if needed, but .env should handle this*)
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
        print_info "(Update mode: Skipping service enable/start*)"
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
        print_error "Failed to start systemd service (Exit code: $start_exit_code*)."
        print_info "Attempting to display the last 20 lines of the service log:"
        # Use --no-pager to prevent blocking in script
        journalctl -u "$SERVICE_NAME" -n 20 --no-pager
        print_warning "Please check the service status using: systemctl status $SERVICE_NAME"
        # Keep existing warning about journalctl as well, in case the direct output fails
        print_warning "And check the logs using: journalctl -u $SERVICE_NAME"
        # Don't necessarily exit, user might be able to fix it.
        return 1 # Indicate start failure
    fi
    return 0
}

# --- Final Steps Functions --- (setup_cron_update, disable_cron_update, prompt_for_cron_setup, final_instructions*)

# Function to specifically disable the cron job
disable_cron_update(*) {
    print_info "Disabling Pulse automatic update cron job..."
    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME*)"
    local escaped_cron_identifier
    escaped_cron_identifier=$(sed 's/[/.*^$]/\\&/g' <<< "$cron_identifier"*)

    # Get current crontab content or empty string if none exists
    current_cron=$(crontab -l -u root 2>/dev/null || true*)

    # Check if the job actually exists before trying to remove
    if ! echo "$current_cron" | grep -q "^${escaped_cron_identifier}$"; then
        print_info "Pulse automatic update cron job not found. Nothing to disable."
        return 0
    fi

    # Use sed to remove the identifier line and the line immediately following it.
    filtered_cron=$(echo "$current_cron" | sed "/^${escaped_cron_identifier}$/{N;d;}"*)

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

setup_cron_update(*) {
    local cron_schedule=""
    local cron_command=""
    local script_path="$SCRIPT_ABS_PATH"
    local escaped_script_path # For grep/sed patterns
    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME*)" # Identifier comment
    local escaped_cron_identifier

    if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
         print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
         return 1
    fi
    # Escape necessary characters for sed pattern matching
    escaped_cron_identifier=$(sed 's/[/.*^$]/\\&/g' <<< "$cron_identifier"*)

    print_info "Choose update frequency:"
    echo "  1*) Daily"
    echo "  2*) Weekly"
    echo "  3*) Monthly"
    echo "  4*) Never (Cancel*)"
    read -p "Enter your choice (1-4*): " freq_choice

    case $freq_choice in
        1*) cron_schedule="@daily" ;;
        2*) cron_schedule="@weekly" ;;
        3*) cron_schedule="@monthly" ;;
        4*) print_info "Automatic updates setup cancelled."; return 0 ;;
        **) print_error "Invalid choice. Skipping auto-update setup."; return 1 ;;
    esac

    # Construct the cron command
    cron_command="$cron_schedule /usr/bin/bash $script_path --update >> $LOG_FILE 2>&1"

    # --- Improved Cron Job Handling ---
    print_info "Checking/Updating cron job for Pulse automatic updates..."
    # Get current crontab content or empty string if none exists
    current_cron=$(crontab -l -u root 2>/dev/null || true*)

    # Use sed to remove the identifier line and the line immediately following it.
    # The pattern looks for the exact identifier comment at the beginning of a line (^*).
    # If found, it reads the Next line (N*) and Deletes both (d*).
    filtered_cron=$(echo "$current_cron" | sed "/^${escaped_cron_identifier}$/{N;d;}"*)

    # Prepare the new crontab content
    # If filtered_cron is empty after removal, avoid leading newline. Otherwise, add newline before appending.
    if [ -z "$filtered_cron" ]; then
        new_cron_content=$(printf "%s\n%s" "$cron_identifier" "$cron_command"*)
    else
        new_cron_content=$(printf "%s\n%s\n%s" "$filtered_cron" "$cron_identifier" "$cron_command"*)
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

prompt_for_cron_setup(*) {
    # Only prompt if not running in update mode
    if [ "$MODE_UPDATE" = true ]; then
        return 0
    fi

    local cron_identifier="# Pulse-Auto-Update ($SCRIPT_NAME*)"
    local cron_exists=false
    # Check if cron job exists for root user
    if crontab -l -u root 2>/dev/null | grep -q "$cron_identifier"; then
        cron_exists=true
    fi

    echo "" # Add spacing

    if [ "$cron_exists" = true ]; then
        print_info "Automatic updates for Pulse appear to be currently ENABLED."
        echo "Choose an action:"
        echo "  1*) Change update schedule"
        echo "  2*) Disable automatic updates"
        echo "  3*) Keep current schedule (Do nothing*)"
        read -p "Enter your choice (1-3*): " cron_manage_choice

        case $cron_manage_choice in
            1*) setup_cron_update ;; # Call function to prompt for new schedule and update
            2*) disable_cron_update ;; # Call function to remove the job
            3*) print_info "Keeping current automatic update schedule.";; 
            **) print_warning "Invalid choice. No changes made to automatic updates.";; 
        esac
    else
        print_info "Automatic updates for Pulse appear to be currently DISABLED."
        read -p "Do you want to set up automatic updates for Pulse? (Y/n*): " setup_cron_confirm
        if [[ ! "$setup_cron_confirm" =~ ^[Nn]$ ]]; then # Proceed if not 'N' or 'n' (Default Yes*)
            setup_cron_update
        else
            print_info "Skipping automatic update setup."
        fi
    fi
}

final_instructions(*) {
    # Try to get the IP address of the container
    local ip_address
    ip_address=$(hostname -I | awk '{print $1}'*) # Get the first IP address
    local port_value
    # Read the port from the .env file, fallback to default if not found/readable
    local env_path="$PULSE_DIR/.env"
    if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
        port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2*)
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
    print_info "The Pulse service ($SERVICE_NAME*) is running and enabled on boot."
    print_info "To check the status: sudo systemctl status $SERVICE_NAME"
    print_info "To view logs: sudo journalctl -u $SERVICE_NAME -f"
    print_info "Configuration file: $PULSE_DIR/.env"
    echo "-------------------------------------------------------------"
}


# --- Main Execution --- Refactored
check_root

# Determine absolute path of the script early
if command -v readlink &> /dev/null && readlink -f "$0" &> /dev/null; then
    SCRIPT_ABS_PATH=$(readlink -f "$0"*)
else
    # Basic fallback if readlink -f is not available or fails
     if [[ "$0" == /* ]]; then
        SCRIPT_ABS_PATH="$0"
     else
        SCRIPT_ABS_PATH="$(pwd*)/$0"
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
    "remove"*)
        print_info "Proceeding with removal..."
        perform_remove
        exit $?
        ;;
    "cancel"*)
        print_info "Operation cancelled by user."
        exit 0
        ;;
    "error"*)
        # Error message already printed
        print_error "Exiting due to error."
        exit 1
        ;;
    "install" | "update"*)
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
            
            # --- Pre-Clone Check ---
            if [ -d "$PULSE_DIR" ] && [ ! -d "$PULSE_DIR/.git" ]; then
                print_error "Directory $PULSE_DIR exists but is not a git repository."
                print_error "Please remove it manually or choose a different installation path."
                exit 1
            fi
            # --- End Pre-Clone Check ---
            
            print_info "Cloning Pulse repository into $PULSE_DIR..."
            if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
                print_success "Repository cloned successfully."
            else
                print_error "Failed to clone repository."
                exit 1
            fi

            install_npm_deps || exit 1 # Installs root and server (NOW INCLUDES DEV*)
            
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
    **)
        print_error "Internal script error: Unknown INSTALL_MODE '$INSTALL_MODE'"
        exit 1
        ;;
esac

print_info "Script finished." 