#!/bin/bash
# Script Version: a29105e
# Testing Self-Update Again: Another small change.

# Pulse for Proxmox VE LXC Installation Script
# This script automates the installation and setup of Pulse within a Proxmox LXC container.

# --- Configuration ---
NODE_MAJOR_VERSION=20 # Specify the desired Node.js major version [e.g., 18, 20]
PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse" # Dedicated user to run Pulse
SERVICE_NAME="pulse-monitor.service"
SCRIPT_NAME="install-pulse.sh" # Used for cron job identification
LOG_FILE="/var/log/pulse_update.log" # Log file for cron updates
SCRIPT_ABS_PATH="" # Store absolute path of the script here
REPO_URL="https://github.com/rcourtman/Pulse.git"
SCRIPT_RAW_URL="https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh"
CURRENT_SCRIPT_COMMIT_SHA="5c9d9525e2ba5eafd2ec6b8a55ec49e2e4cd52de" # Keeping HEAD version
# Version of the script being run (will be replaced by self-update mechanism)
INSTALLER_VERSION="0.0.0" # Placeholder

# --- Flags & Variables ---
MODE_UPDATE="" # Flag to run in non-interactive update mode (empty string means false)
INSTALL_MODE=""   # Stores the determined action: install, update, remove, cancel, error
SPECIFIED_VERSION_TAG="" # Stores tag specified via --version
TARGET_TAG="" # Stores the final tag to be installed/updated
INSTALLER_WAS_REEXECUTED=false # ---> NEW: Flag to track re-execution

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
          # Cannot reliably get path, self-update won't work safely
          SCRIPT_ABS_PATH=""
     fi
fi

# --- Argument Parsing ---
# Consume arguments until --version is found, then expect its value
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --update) MODE_UPDATE="true"; shift ;; # Set to non-empty value
        --installer-reexecuted)
             INSTALLER_WAS_REEXECUTED=true
             shift # Consume the flag
             ;;
        --version)
            if [[ -n "$2" ]] && [[ "$2" != --* ]]; then
                SPECIFIED_VERSION_TAG="$2"
                shift 2 # Consume --version and its value
            else
                echo "Error: --version requires a tag name [e.g., v3.2.3]" >&2
                exit 1
            fi
            ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
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
  echo -e "\033[1;31m[ERROR]\033[0m $1 >&2"
}

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo."
    exit 1
  fi
}

# ---> NEW: Function to inform about dependencies < ---
print_dependency_info() {
    # --- ASCII Banner (from stash) ---
    echo -e "\033[1;34m" # Start color blue
    echo ' ____        _          '
    echo '|  _ \ _   _| |___  ___ '
    echo '| |_) | | | | / __|/ _ \'
    echo '|  __/| |_| | \__ \  __/'
    echo '|_|    \__,_|_|___/\___|'
    echo '                       '
    echo -e "\033[0m" # End color
    echo "" # Keep blank line for spacing
    echo -e "\033[1mWelcome to the Pulse for Proxmox VE Installer\033[0m"
    echo "This script will install Pulse, a lightweight monitoring application."
    echo "See README.md for more details about Pulse: https://github.com/rcourtman/Pulse"
    echo ""
     # --- Display Dependencies (Method from stash, content from HEAD) ---
     local dep_text
     dep_text=$(cat << 'EOF'
-----------------------------------------------------
Required Dependencies:

  - Core Tools:
    * curl, git, sudo, gpg, diffutils
    (Will be installed via apt-get if missing)

  - Pulse Runtime:
    * Node.js & npm
    (Will be installed via NodeSource if missing)

  - Installer Self-Update:
    * jq
    (Will attempt apt-get install if missing)

-----------------------------------------------------
EOF
     )
     print_info "$dep_text"
     # --- End Display Dependencies ---

    # --- NEW: Confirmation Prompt (Default Yes) ---
    local confirm_proceed

    # Prompt defaults to Yes [Y/n]
    read -p "Do you want to proceed with the installation/update? [Y/n]: " confirm_proceed
    # Exit only if user explicitly enters 'n' or 'N'
    if [[ "$confirm_proceed" =~ ^[Nn]$ ]]; then
        print_error "Operation aborted by user."
        exit 1 # Use exit code 1 for aborted operations
    fi
    print_info "Proceeding..."
    # --- END Confirmation Prompt ---
}
# --- END NEW FUNCTION ---

# --- Self-Update Function (GitHub API + jq + git) ---
self_update_check() {
    # Only run check if interactive and script path was found
    if [ ! -t 0 ] || [ -n "$MODE_UPDATE" ] || [ -z "$SCRIPT_ABS_PATH" ]; then # Check if MODE_UPDATE is non-empty
        return 0
    fi

    # --- Dependencies Check ---
    if ! command -v curl &> /dev/null; then
        print_warning "curl command not found, skipping installer self-update check."
        return 0
    fi
    if ! command -v jq &> /dev/null; then
        print_info "jq command not found, attempting to install it for self-update check..."
        # Attempt to install jq quietly. Requires apt-get.
        if command -v apt-get &> /dev/null; then
            apt-get update -qq > /dev/null # Update package lists quietly
            if apt-get install -y -qq jq > /dev/null; then
                print_success "jq installed successfully."
            else
                local jq_install_exit_code=$?
                print_warning "Failed to automatically install jq (Exit code: $jq_install_exit_code)."
                print_warning "Please install jq manually (e.g., sudo apt-get install jq) to enable installer updates."
                print_warning "Skipping installer self-update check for this run."
                return 0 # Skip self-update if install failed
            fi
        else
             print_warning "apt-get not found. Cannot automatically install jq."
             print_warning "Please install jq manually to enable installer updates."
             print_warning "Skipping installer self-update check for this run."
             return 0 # Skip self-update if apt-get not found
        fi
    fi
    # Git is not strictly needed for the SHA comparison, but keep check for now
    local git_ok=false
    if [ -d "$PULSE_DIR/.git" ] && command -v git &> /dev/null; then
        git_ok=true
    fi
    if [ "$git_ok" = false ]; then
        # If we are *inside* the cloned repo, git is needed for other parts later.
        # If not yet cloned, it's okay to not have it for this check.
        # We rely on the absolute script path existing for updates.
        if [ -d "$PULSE_DIR/.git" ]; then
            print_warning "git command not found within $PULSE_DIR, skipping installer self-update check."
            return 0
        else
             print_info "Git not found or $PULSE_DIR not a repo yet, proceeding with API check."
        fi
    fi
    # --- END Dependencies Check ---

    print_info "Checking for updates to the installer script itself (using GitHub API)..."

    # --- Get Latest Commit SHA from GitHub API ---
    local owner="rcourtman"
    local repo="Pulse"
    local script_relative_path="scripts/install-pulse.sh" # Relative path within repo
    local branch="main"
    local api_url="https://api.github.com/repos/${owner}/${repo}/commits?path=${script_relative_path}&sha=${branch}&per_page=1"
    local latest_remote_sha

    latest_remote_sha=$(curl -sL -H "Accept: application/vnd.github.v3+json" "$api_url" | jq -r 'if type=="array" and length > 0 then .[0].sha else empty end')
    local curl_exit_code=$?

    if [ $curl_exit_code -ne 0 ]; then
        print_warning "curl command failed when checking GitHub API for updates (Exit code: $curl_exit_code). Skipping self-update check."
        return 0
    fi
    if [ -z "$latest_remote_sha" ] || [ "$latest_remote_sha" = "null" ]; then
        print_warning "Could not determine the latest commit SHA from GitHub API. Skipping self-update check."
        return 0
    fi
    # --- END Get Latest Commit SHA ---

    # --- Use Embedded SHA for Local Version ---
    # The currently running script's commit SHA is hardcoded at the top
    local current_local_sha="$CURRENT_SCRIPT_COMMIT_SHA"
    if [ -z "$current_local_sha" ]; then
        print_warning "Could not find embedded CURRENT_SCRIPT_COMMIT_SHA in the script. Skipping self-update check."
        return 0
    fi
    # --- END Use Embedded SHA ---

    # --- Compare SHAs ---
    if [ "$latest_remote_sha" != "$current_local_sha" ]; then
        print_warning "A newer version of the installation script (Remote: ${latest_remote_sha:0:7}, Local: ${current_local_sha:0:7}) is available."
        read -p "Do you want to update the installer and re-run? [Y/n]: " update_confirm
        if [[ ! "$update_confirm" =~ ^[Nn]$ ]]; then
            print_info "Updating installer script..."
            local temp_script="/tmp/${SCRIPT_NAME}.tmp"
            if ! curl -sL "$SCRIPT_RAW_URL" -o "$temp_script"; then
                 print_error "Failed to download the latest installer script from $SCRIPT_RAW_URL."
                 rm -f "$temp_script"
                 return 1
            fi

            # ---> NEW: Update embedded SHA in the downloaded file BEFORE fixing endings/moving < ---
            print_info "[DEBUG] Updating embedded SHA in temp script to $latest_remote_sha..."
            # ---> START REPLACEMENT LOGIC (Robust alternative to sed)
            local processed_temp_script="${temp_script}.processed"
            local line_updated=false
            while IFS= read -r line || [[ -n "$line" ]]; do
                # Check if the line starts with the variable we want to replace
                if [[ "$line" == CURRENT_SCRIPT_COMMIT_SHA=* ]]; then
                    echo "CURRENT_SCRIPT_COMMIT_SHA=\"$latest_remote_sha\"" >> "$processed_temp_script"
                    line_updated=true
                # Optional: Fix line endings (though curl might handle this)
                # elif [[ "$line" == *$'\r' ]]; then
                #    echo "${line%$'\r'}" >> "$processed_temp_script" 
                else
                    # Write other lines as-is
                    echo "$line" >> "$processed_temp_script"
                fi
            done < "$temp_script"

            if [ "$line_updated" = false ]; then
                print_error "Failed to find and update CURRENT_SCRIPT_COMMIT_SHA line in downloaded script!"
                rm -f "$temp_script" "$processed_temp_script"
                return 1
            fi
            # ---> END REPLACEMENT LOGIC <---

            # ---> NOW work with processed_temp_script <---
            if ! chmod +x "$processed_temp_script"; then
                print_error "Failed to make processed temporary script executable."
                rm -f "$temp_script" "$processed_temp_script"
                return 1
            fi

            # --- Move the PROCESSED temp file into place ---
            if ! mv "$processed_temp_script" "$SCRIPT_ABS_PATH"; then
                 print_error "Failed to replace the current script file with processed version."
                 rm -f "$temp_script" "$processed_temp_script" # Clean up both temps
                 return 1
            fi
            rm -f "$temp_script" # Clean up original temp file

            print_success "Installer updated successfully to commit ${latest_remote_sha:0:7}."
            print_info "Re-executing with updated installer..."
            # ---> MODIFY exec to pass flag < ---
            exec bash "$SCRIPT_ABS_PATH" --installer-reexecuted "$@"
            print_error "Failed to re-execute the updated script. Please re-run manually: sudo bash $SCRIPT_ABS_PATH"
            exit 1
        else
            print_info "Skipping installer update. Continuing with the current version."
            return 0
        fi
    else
        print_info "Installer script is up-to-date (Commit: $current_local_sha)" # Use info level
        return 0
    fi
}

# --- Git Helper Functions ---
# Fetches remote tags and returns the latest semantic version tag [vX.Y.Z]
get_latest_remote_tag() {
    local latest_tag
    print_info "Fetching latest remote tags..." >&2 # Redirect info message to stderr
    # Use sudo only if not already root [though check_root ensures we are]
    if ! sudo -u "$PULSE_USER" git fetch origin --tags --force >/dev/null 2>&1; then
        print_warning "Could not fetch remote tags." >&2 # Also redirect warning
        return 1
    fi
    # Sort tags semantically [version sort] and get the latest 'v*' tag
    latest_tag=$(sudo -u "$PULSE_USER" git tag -l 'v*' --sort='-version:refname' | head -n 1)
    if [ -z "$latest_tag" ]; then
        print_warning "Could not determine the latest remote release tag." >&2 # Also redirect warning
        return 1
    fi
    echo "$latest_tag" # This is the intended stdout
    return 0
}

# Checks if a specific tag exists remotely
check_remote_tag_exists() {
    local tag_to_check=$1
    if [ -z "$tag_to_check" ]; then return 1; fi
    print_info "Checking if remote tag '$tag_to_check' exists..."
    # Use ls-remote to check efficiently
    if sudo -u "$PULSE_USER" git ls-remote --tags origin "refs/tags/$tag_to_check" | grep -q "refs/tags/$tag_to_check$"; then
        print_info "Remote tag '$tag_to_check' found."
        return 0
    else
        print_error "Remote tag '$tag_to_check' not found."
        return 1
    fi
}

# Gets the tag currently checked out
get_current_local_tag() {
    local current_tag
    # describe might fail if not exactly on a tag, try rev-parse first on HEAD
    # then try describe. If HEAD isn't tagged, describe --tags might give nearest ancestor.
    current_tag=$(sudo -u "$PULSE_USER" git describe --tags --exact-match HEAD 2>/dev/null)
    if [ $? -ne 0 ]; then
         # Not exactly on a tag, maybe try nearest? Or just report empty?
         # For comparison, maybe nearest is okay, but indicates "not on latest release"
         current_tag=$(sudo -u "$PULSE_USER" git describe --tags --abbrev=0 HEAD 2>/dev/null)
         # If even that fails, return empty
         if [ $? -ne 0 ]; then
             current_tag=""
         fi
    fi
    echo "$current_tag"
}


# --- Installation Status Check & Action Determination ---
check_installation_status_and_determine_action() {
    # Check if running in non-interactive update mode FIRST
    if [ -n "$MODE_UPDATE" ]; then # Check if MODE_UPDATE is non-empty
        print_info "Running in non-interactive update mode..."
        INSTALL_MODE="update"
        # Determine target tag [latest unless specified]
        if [ -n "$SPECIFIED_VERSION_TAG" ]; then
            # Need to check if repo exists to run git commands
            if [ -d "$PULSE_DIR/.git" ]; then
                 cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
                 if check_remote_tag_exists "$SPECIFIED_VERSION_TAG"; then
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                 else
                     INSTALL_MODE="error" # Tag not found
                     cd ..; return
                 fi
                 cd ..
            else
                print_error "Cannot validate specified version tag: Pulse directory $PULSE_DIR not found."
                INSTALL_MODE="error"; return
            fi
        else
            # Need to check if repo exists to run git commands
            if [ -d "$PULSE_DIR/.git" ]; then
                cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
                TARGET_TAG=$(get_latest_remote_tag)
                if [ $? -ne 0 ] || [ -z "$TARGET_TAG" ]; then
                     print_error "Could not determine latest remote tag for update."
                     INSTALL_MODE="error"
                fi
                cd ..
            else
                 print_error "Cannot determine latest version tag: Pulse directory $PULSE_DIR not found."
                 INSTALL_MODE="error"; return
            fi
        fi
        # If error occurred determining tag, INSTALL_MODE will be set to error
        return 0 # Continue to main update logic if no error
    fi

    print_info "Checking Pulse installation at $PULSE_DIR..."
    if [ -d "$PULSE_DIR" ]; then
        # Directory exists
        if [ -d "$PULSE_DIR/.git" ]; then
            # It's a git repository
            cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }

            local current_tag
            current_tag=$(get_current_local_tag) # Get tag of current HEAD

            local latest_tag
            latest_tag=$(get_latest_remote_tag) # Get latest remote tag [also fetches]
            if [ $? -ne 0 ] || [ -z "$latest_tag" ]; then
                print_warning "Could not determine the latest remote release tag. Cannot check for updates reliably."
                INSTALL_MODE="update" # Offer update anyway
            else
                print_info "Current installed version tag: ${current_tag:-Not on a tag}"
                print_info "Latest available version tag: $latest_tag"

                if [ -n "$current_tag" ] && [ "$current_tag" = "$latest_tag" ]; then
                    print_info "Pulse is already installed and up-to-date with the latest release $latest_tag."
                    INSTALL_MODE="uptodate"
                else
                    print_warning "Pulse is installed, but an update to $latest_tag is available."
                    INSTALL_MODE="update"
                fi
            fi

            # Determine target tag for potential install/update
            if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                 if check_remote_tag_exists "$SPECIFIED_VERSION_TAG"; then
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                     print_info "Will target specified version: $TARGET_TAG"
                     INSTALL_MODE="update" # Force update mode if specific version requested
                 else
                     INSTALL_MODE="error"; cd ..; return # Specified tag not found
                 fi
            else
                TARGET_TAG="$latest_tag" # Default to latest determined tag
            fi
            cd .. # Go back to original directory

            # Prompt based on status
            if [ "$INSTALL_MODE" = "uptodate" ]; then
                # If user specified a version different from current, offer update
                if [ -n "$SPECIFIED_VERSION_TAG" ] && [ "$SPECIFIED_VERSION_TAG" != "$current_tag" ]; then
                     print_info "You requested version $SPECIFIED_VERSION_TAG, but $current_tag is installed."
                     echo -e "Choose an action:"
                     echo -e "  1) Manage automatic updates"
                     echo -e "  2) Re-install current version $current_tag"
                     echo -e "  3) Remove Pulse"
                     echo -e "  4) Cancel"
                     read -p "Enter your choice [1-4]: " user_choice # Updated range
                     case $user_choice in
                         1) prompt_for_cron_setup; INSTALL_MODE="cancel" ;; # Call cron setup and then cancel main flow
                         2) INSTALL_MODE="update" ;; # Treat re-run as update
                         3) INSTALL_MODE="remove" ;;
                         4) INSTALL_MODE="cancel" ;;
                         *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                     esac
                else # Up to date and no specific version requested OR specified matches current
                    echo -e "Choose an action:"
                    echo -e "  1) Manage automatic updates"
                    echo -e "  2) Re-install current version $current_tag"
                    echo -e "  3) Remove Pulse"
                    echo -e "  4) Cancel"
                    read -p "Enter your choice [1-4]: " user_choice # Updated range
                    case $user_choice in
                        1) prompt_for_cron_setup; INSTALL_MODE="cancel" ;; # Call cron setup and then cancel main flow
                        2) INSTALL_MODE="update" ;; # Treat re-run as update
                        3) INSTALL_MODE="remove" ;;
                        4) INSTALL_MODE="cancel" ;;
                        *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                    esac
                fi
            elif [ "$INSTALL_MODE" = "update" ]; then # Update available or fetch failed
                 if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                     echo "Choose an action:"
                     echo "  1) Install specified version $SPECIFIED_VERSION_TAG"
                     echo "  2) Remove Pulse"
                     echo "  3) Cancel"
                     echo "  4) Manage automatic updates"
                     read -p "Enter your choice [1-4]: " user_choice # Updated range
                     case $user_choice in
                         1) INSTALL_MODE="update" ;;
                         2) INSTALL_MODE="remove" ;;
                         3) INSTALL_MODE="cancel" ;;
                         4) prompt_for_cron_setup; INSTALL_MODE="cancel" ;; # Call cron setup and then cancel main flow
                         *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                     esac
                 else # Defaulting to latest tag
                      echo "Choose an action:"
                      echo "  1) Update Pulse to the latest version $TARGET_TAG"
                      echo "  2) Remove Pulse"
                      echo "  3) Cancel"
                      echo "  4) Manage automatic updates"
                      read -p "Enter your choice [1-4]: " user_choice # Updated range
                      case $user_choice in
                          1) INSTALL_MODE="update" ;;
                          2) INSTALL_MODE="remove" ;;
                          3) INSTALL_MODE="cancel" ;;
                          4) prompt_for_cron_setup; INSTALL_MODE="cancel" ;; # Call cron setup and then cancel main flow
                          *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                      esac
                 fi
            fi

        else
            # Directory exists but isn't a git repository
            print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse git repository."
            print_error "Please remove this directory manually \($PULSE_DIR\) or choose a different installation path and re-run the script."
            INSTALL_MODE="error"
        fi
    else
        # Directory doesn't exist - Offer Install/Cancel
        print_info "Pulse is not currently installed."
        # Determine target tag for potential install
        if [ -n "$SPECIFIED_VERSION_TAG" ]; then
            # Need to check tag existence BEFORE offering install
            # Clone temporarily to check? No, use ls-remote. Need git installed first...
            # Assume git is installed by dependencies step later, check tag existence THEN.
            # For now, just store it.
            TARGET_TAG="$SPECIFIED_VERSION_TAG"
            print_info "Will attempt to install specified version: $TARGET_TAG"
            INSTALL_MODE="install"
        else
            # Need git to determine latest. We'll do this later in the install step.
            # Set mode first.
            echo "Choose an action:"
            echo "  1) Install Pulse [latest version]"
            echo "  2) Cancel"
            read -p "Enter your choice [1-2]: " user_choice
            case $user_choice in
                1) INSTALL_MODE="install" ;; # Target tag determined later
                2) INSTALL_MODE="cancel" ;;
                *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
            esac
        fi
    fi
    # We don't return here, INSTALL_MODE is now set globally for the main logic
}

# --- System Setup Functions --- [apt_update_upgrade, install_dependencies, setup_node, create_pulse_user]
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
    print_info "Installing necessary dependencies [git, curl, sudo, gpg, diffutils]..."
    # Added diffutils previously, keeping it
    if apt-get install -y git curl sudo gpg diffutils > /dev/null; then
        print_success "Dependencies installed."
    else
        print_error "Failed to install dependencies."
        exit 1
    fi
}

setup_node() {
    print_info "Setting up Node.js repository [NodeSource]..."
    # Check if Node.js is already installed and meets version requirement
    if command -v node &> /dev/null; then
        current_node_version=$(node -v 2>/dev/null)
        current_major_version=$(echo "$current_node_version" | sed 's/v//' | cut -d. -f1)
        if [[ -n "$current_major_version" ]] && [[ "$current_major_version" -ge "$NODE_MAJOR_VERSION" ]]; then
            print_info "Node.js version ${current_node_version} already installed and meets requirement >= v${NODE_MAJOR_VERSION}.x. Skipping setup."
            return 0
        else
            print_warning "Installed Node.js version $current_node_version does not meet requirement >= v${NODE_MAJOR_VERSION}.x or could not be determined. Proceeding with setup..."
        fi
    else
         print_info "Node.js not found. Proceeding with setup..."
    fi

    # Check if curl is installed before using it
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not found. Please install it first [apt-get install curl]."
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
        # Create a system user with no login shell and no home directory [or specify one if needed]
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


# --- Core Action Functions --- [perform_update, perform_remove]
perform_update() {
    # TARGET_TAG should be set by check_installation_status_and_determine_action
    if [ -z "$TARGET_TAG" ]; then
        print_error "Target version tag not determined. Cannot update."
        return 1
    fi
    print_info "Attempting to update Pulse to version $TARGET_TAG..."
    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    # ---> NEW: Backup the current installer script < ---
    local script_backup_path="/tmp/${SCRIPT_NAME}.bak"
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Backing up current installer script to $script_backup_path..."
        if cp "$SCRIPT_ABS_PATH" "$script_backup_path"; then
             print_success "Installer script backed up."
        else
             print_warning "Failed to back up installer script. Update might revert it."
             # Clear backup path so we don't try to restore/remove it later
             script_backup_path=""
        fi
    else
        print_warning "Cannot determine current installer script path. Cannot back it up."
        script_backup_path=""
    fi
    # --- END NEW STEP ---

    # Add safe directory config for root user, just in case
    git config --global --add safe.directory "$PULSE_DIR" > /dev/null 2>&1 || print_warning "Could not configure safe.directory for root user."

    # ---> Force reset local changes before fetching <--- 
    print_info "Resetting local repository to discard potential changes [running as user $PULSE_USER]..."
    # This WILL revert the installer script in the workdir if it was updated by self_update
    if ! sudo -u "$PULSE_USER" git reset --hard HEAD; then
        print_error "Failed to reset local repository. Aborting update."
        # Clean up backup if it exists
        [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
        cd ..
        return 1
    fi
    # ---> End modification <--- 

    print_info "Fetching latest changes and tags from git [running as user $PULSE_USER]..."
    # Ensure tags are fetched
    if ! sudo -u "$PULSE_USER" git fetch origin --tags --force; then
        print_error "Failed to fetch latest changes/tags from git."
        [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
        cd ..
        return 1
    fi

    # Check if target tag actually exists after fetching
    if ! sudo -u "$PULSE_USER" git rev-parse "$TARGET_TAG" >/dev/null 2>&1; then
         print_error "Target tag '$TARGET_TAG' could not be found locally after fetch."
         print_error "It might have been deleted remotely or there was a fetch issue."
         [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
         cd ..
         return 1
    fi

    print_info "Checking out target version tag '$TARGET_TAG'..."
    # Checkout the specific tag. This will result in a detached HEAD state.
    if ! sudo -u "$PULSE_USER" git checkout "$TARGET_TAG"; then
        print_error "Failed to checkout tag '$TARGET_TAG'."
        [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
        cd ..
        return 1
    fi

    # Get the tag again now that we've checked it out
    local current_tag
    current_tag=$(sudo -u "$PULSE_USER" git describe --tags --exact-match HEAD 2>/dev/null) # Should now match TARGET_TAG

    print_info "Cleaning repository [removing untracked files]..."
    # Remove untracked files and directories to ensure a clean state
    # Use -f for files, -d for directories. Do NOT use -x which removes ignored files like .env
    if ! sudo -u "$PULSE_USER" git clean -fd; then
        print_warning "Failed to clean untracked files from the repository."
        # Continue anyway, as the core update [checkout] succeeded
    fi

    # ---> NEW: Restore the installer script if it was backed up < ---
    if [ -n "$script_backup_path" ] && [ -f "$script_backup_path" ]; then
        print_info "Restoring potentially updated installer script from backup..."
        if cp "$script_backup_path" "$SCRIPT_ABS_PATH"; then
             print_success "Installer script restored."
        else
             print_warning "Failed to restore installer script from backup."
             print_warning "The version in $SCRIPT_ABS_PATH might be outdated."
        fi
        # Clean up the backup file now
        rm -f "$script_backup_path"
    fi
    # --- END NEW STEP ---

    # Ensure the script itself remains executable after update/restore
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Ensuring install script $SCRIPT_ABS_PATH is executable..."
        if chmod +x "$SCRIPT_ABS_PATH"; then
            print_success "Install script executable permission set."
        else
            print_warning "Failed to set executable permission on install script."
        fi
    else
        print_warning "Could not find script path $SCRIPT_ABS_PATH to ensure executable permission."
    fi

    print_info "Re-installing npm dependencies [root]..."
    if ! npm install --unsafe-perm > /dev/null 2>&1; then
        print_warning "Failed to install root npm dependencies during update. Continuing..."
    else
        print_success "Root dependencies updated."
    fi

    print_info "Re-installing server dependencies..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
     if npm install --unsafe-perm > /dev/null 2>&1; then
        print_success "Server dependencies updated."
    else
        print_error "Failed to install server npm dependencies."
        exit 1 # Exit if server deps fail
    fi
    cd .. # Back to PULSE_DIR

    # Build CSS after dependencies
    print_info "Building CSS assets..."
    if ! npm run build:css > /dev/null 2>&1; then
        print_error "Failed to build CSS assets during update."
        print_warning "Continuing update despite CSS build failure."
    else
        print_success "CSS assets built."
    fi

    set_permissions # Ensure permissions are correct after update and build

    # Ensure the systemd service is configured correctly before restarting
    print_info "Ensuring systemd service $SERVICE_NAME is configured..."
    if ! setup_systemd_service true; then # Pass true to indicate update mode [skip start/enable]
        print_error "Failed to configure systemd service during update."
        return 1
    fi

    print_info "===> Attempting to restart Pulse service $SERVICE_NAME..."
    systemctl restart "$SERVICE_NAME"
    local restart_exit_code=$?

    if [ $restart_exit_code -eq 0 ]; then
        print_success "Pulse service restart command finished successfully [Exit code: $restart_exit_code]."
    else
        print_error "Pulse service restart command failed [Exit code: $restart_exit_code]."
        print_warning "Please check the service status manually: sudo systemctl status $SERVICE_NAME"
        print_warning "And check logs: sudo journalctl -u $SERVICE_NAME"
        return 1
    fi

    # Use the actual checked-out tag in the success message
    if [ -n "$current_tag" ]; then
      print_success "Pulse updated successfully to version $current_tag!"
    else
      print_success "Pulse updated successfully! [Could not confirm exact tag]"
    fi
    return 0
}


perform_remove() {
    print_warning "This will stop and disable the Pulse service[s] and remove the installation directory ($PULSE_DIR)."
    # Allow non-interactive removal if called directly with logic before
    # For now, keep interactive confirmation here
    local remove_confirm=""
    if [ -t 0 ]; then # Check if stdin is a terminal for interactive prompt
      read -p "Are you sure you want to remove Pulse? [y/N]: " remove_confirm
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
            print_info "Stopping service $service_name..."
            systemctl stop "$service_name" > /dev/null 2>&1 # Ignore errors if already stopped

            print_info "Disabling service $service_name..."
            systemctl disable "$service_name" > /dev/null 2>&1 # Ignore errors if already disabled

            if [ -f "$service_file_path" ]; then
                print_info "Removing systemd service file $service_file_path..."
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
                 print_info "Removing orphaned systemd service file $service_file_path..."
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

    print_info "Removing Pulse installation directory $PULSE_DIR..."
    if rm -rf "$PULSE_DIR"; then
        print_success "Installation directory removed."
    else
        print_error "Failed to remove installation directory $PULSE_DIR. Please remove it manually."
        return 1 # Indicate error
    fi

    print_success "Pulse removed successfully."
    return 0
}

# --- Installation Step Functions --- [install_npm_deps, set_permissions, configure_environment, setup_systemd_service]
install_npm_deps() {
    print_info "Installing npm dependencies..."
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
        # This shouldn't happen if called in the right flow, but check anyway
        return 1
    fi

    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    print_info "Installing root dependencies [including dev]..."
    # Use --unsafe-perm if running npm install as root, which might be necessary for some packages
    # Includes dev deps needed for build tools like postcss/autoprefixer
    if npm install --unsafe-perm > /dev/null 2>&1; then
        print_success "Root dependencies installed."
    else
        print_error "Failed to install root npm dependencies."
        return 1
    fi

    print_info "Installing server dependencies [including dev]..."
    cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; cd ..; return 1; }
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
    print_info "Configuring Pulse environment..."
    local env_example_path="$PULSE_DIR/.env.example"
    local env_path="$PULSE_DIR/.env"

    if [ ! -f "$env_example_path" ]; then
        print_error "Environment example file not found at $env_example_path. Cannot configure."
        return 1
    fi

    # Check if .env already exists
    if [ -f "$env_path" ]; then
        print_warning "Configuration file $env_path already exists."
        # Only prompt if interactive
        if [ -t 0 ]; then
            read -p "Overwrite existing configuration? [y/N]: " overwrite_confirm
            if [[ ! "$overwrite_confirm" =~ ^[Yy]$ ]]; then
                print_info "Skipping environment configuration."
                return 0 # Exit the function successfully without configuring
            fi
            print_info "Proceeding to overwrite existing configuration..."
        else
            print_info "Running non-interactively, skipping environment configuration as file exists."
            return 0
        fi
    fi

    # --- Gather Proxmox Details [Only if interactive] ---
    local proxmox_host=""
    local proxmox_token_id=""
    local proxmox_token_secret=""
    local allow_self_signed="y"
    local pulse_port=""

    if [ -t 0 ]; then
        echo "Please provide your Proxmox connection details:"
        read -p " -> Proxmox Host URL [e.g., https://192.168.1.100:8006]: " proxmox_host
        while [ -z "$proxmox_host" ]; do
            print_warning "Proxmox Host URL cannot be empty."
            read -p " -> Proxmox Host URL [e.g., https://192.168.1.100:8006]: " proxmox_host
        done

        # Validate and potentially prepend https://
        if [[ ! "$proxmox_host" =~ ^https?:// ]]; then
            print_warning "URL does not start with http:// or https://. Prepending https://."
            proxmox_host="https://$proxmox_host"
            print_info "Using Proxmox Host URL: $proxmox_host"
        fi

        # --- Display Token Generation Info ---
        echo ""
        print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
        print_info "or run the following commands on your Proxmox host shell:"
        echo "----------------------------------------------------------------------"
        echo '  # 1. Create user ''pulse-monitor'' [enter password when prompted]:'
        echo "  pveum useradd pulse-monitor@pam -comment "API user for Pulse monitoring""
        echo '  '
        echo '  # 2. Create API token ''pulse'' for user [COPY THE SECRET VALUE!]:'
        echo "  pveum user token add pulse-monitor@pam pulse --privsep=1"
        echo '  '
        echo '  # 3. Assign PVEAuditor role to user:'
        echo "  pveum acl modify / -user pulse-monitor@pam -role PVEAuditor"
        echo "----------------------------------------------------------------------"
        echo "After running the 'token add' command, copy the Token ID and Secret Value"
        echo "and paste them below."
        echo ""

        read -p " -> Proxmox API Token ID [e.g., user@pam!tokenid]: " proxmox_token_id
        while [ -z "$proxmox_token_id" ]; do
            print_warning "Proxmox Token ID cannot be empty."
            read -p " -> Proxmox API Token ID [e.g., user@pam!tokenid]: " proxmox_token_id
        done

        read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
        echo
        while [ -z "$proxmox_token_secret" ]; do
            print_warning "Proxmox Token Secret cannot be empty."
            read -sp " -> Proxmox API Token Secret: " proxmox_token_secret
            echo
        done

        # --- Optional Settings ---
        read -p "Allow self-signed certificates for Proxmox? [Y/n]: " allow_self_signed
        read -p "Port for Pulse server [leave blank for default 7655]: " pulse_port
    else
        print_warning "Running non-interactively. Cannot prompt for environment details."
        print_warning "Ensure $env_path is configured manually or exists from a previous run."
        # Use defaults or skip creation if running non-interactively?
        # For now, just skip the interactive part and proceed to copy/sed if file doesn't exist
        # or if overwrite was forced [though non-interactive won't force]
        if [ -f "$env_path" ]; then return 0; fi # Skip if file exists and non-interactive
    fi

    # Determine values [use defaults if not set interactively]
    local self_signed_value="true"
    if [[ "$allow_self_signed" =~ ^[Nn]$ ]]; then self_signed_value="false"; fi

    local port_value="7655"
    if [ -n "$pulse_port" ]; then
        if [[ "$pulse_port" =~ ^[0-9]+$ ]] && [ "$pulse_port" -ge 1 ] && [ "$pulse_port" -le 65535 ]; then
            port_value="$pulse_port"
        else
            print_warning "Invalid port number entered. Using default 7655."
        fi
    fi

    # --- Create .env file --- [Handle case where variables might be empty if non-interactive]
    print_info "Creating $env_path from example..."
    if cp "$env_example_path" "$env_path"; then
        # Only run sed if variables were set [i.e., interactive mode ran]
        if [ -n "$proxmox_host" ]; then
             sed -i "s|^PROXMOX_HOST=.*|PROXMOX_HOST=$proxmox_host|" "$env_path"
             sed -i "s|^PROXMOX_TOKEN_ID=.*|PROXMOX_TOKEN_ID=$proxmox_token_id|" "$env_path"
             sed -i "s|^PROXMOX_TOKEN_SECRET=.*|PROXMOX_TOKEN_SECRET=$proxmox_token_secret|" "$env_path"
             sed -i "s|^PROXMOX_ALLOW_SELF_SIGNED_CERTS=.*|PROXMOX_ALLOW_SELF_SIGNED_CERTS=$self_signed_value|" "$env_path"
             sed -i "s|^PORT=.*|PORT=$port_value|" "$env_path"
        else
            print_warning "Skipping variable substitution in .env as no values were provided [non-interactive?]."
            print_warning "Please edit $env_path manually."
        fi

        # Set ownership & permissions regardless
        chown "$PULSE_USER":"$PULSE_USER" "$env_path"
        chmod 600 "$env_path"

        print_success "Environment file created/updated in $env_path."
        return 0
    else
        print_error "Failed to copy $env_example_path to $env_path."
        return 1
    fi
}

setup_systemd_service() {
    # Add optional argument to skip start/enable during updates
    local update_mode=${1:-false} # Default to false if no argument passed

    print_info "Setting up systemd service $SERVICE_NAME..."
    local service_file="/etc/systemd/system/$SERVICE_NAME"

    # Find Node path [needed for systemd ExecStart]
    local node_path
    node_path=$(command -v node)
    if [ -z "$node_path" ]; then
        print_error "Could not find Node.js executable path. Cannot create service."
        return 1
    fi
    # Find npm path [needed for systemd ExecStart]
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

# Environment [optional, if needed, but .env should handle this]
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
        print_info "[Update mode: Skipping service enable/start]"
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
    if systemctl start "$SERVICE_NAME"; then
        print_success "Service started successfully."
    else
        print_error "Failed to start systemd service."
        print_warning "Please check the service status using: systemctl status $SERVICE_NAME"
        print_warning "And check the logs using: journalctl -u $SERVICE_NAME"
        # Don't necessarily exit, user might be able to fix it.
        return 1 # Indicate start failure
    fi
    return 0
}


# --- Final Steps Functions --- [setup_cron_update, disable_cron_update, prompt_for_cron_setup, final_instructions]
# [No changes needed in disable_cron_update or setup_cron_update logic, but prompt needs adjusting]

# Function to specifically disable the cron job
disable_cron_update() {
    print_info "Disabling Pulse automatic update cron job..."
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]" # Use the variable

    # Get current crontab content or empty string if none exists
    current_cron=$(crontab -l -u root 2>/dev/null || true)

    # Check if the job actually exists before trying to remove
    # Escape only for grep's basic regex needs here
    local identifier_for_grep_check
    identifier_for_grep_check=$(echo "$cron_identifier" | sed 's/[]$.*^[]/\\&/g') # Escape regex metachars for grep
    if ! echo "$current_cron" | grep -q "^${identifier_for_grep_check}$"; then
        print_info "Pulse automatic update cron job not found. Nothing to disable."
        return 0
    fi

    # --- Use awk to remove existing blocks ---
    # Escape the identifier for awk pattern matching (ensure literal match)
    local escaped_identifier_for_awk
    # Escape for extended regex used by awk
    escaped_identifier_for_awk=$(echo "$cron_identifier" | sed 's/[][\\.*^$(){}|+?]/\\\\&/g')
    local awk_script='/__PATTERN__/ { skip=1; next } skip == 1 { skip=0; next } { print }'
    # Replace placeholder with escaped identifier, using ^ to anchor at start of line
    awk_script="${awk_script/__PATTERN__/^$escaped_identifier_for_awk$}"

    # Filter the crontab, preserving other lines
    filtered_cron=$(echo "$current_cron" | awk "$awk_script")
    # --- End awk filtering ---

    # Load the modified crontab content
    # Trim potential leading/trailing whitespace/newlines from filtered_cron
    trimmed_filtered_cron=$(echo "$filtered_cron" | awk 'NF') # Get non-empty lines
    if [ -z "$trimmed_filtered_cron" ]; then
        echo "" | crontab -u root - # Load empty crontab
    else
        echo "$trimmed_filtered_cron" | crontab -u root - # Load filtered content
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
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]" # Use the variable

    if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
         print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
         return 1
    fi

    print_info "Choose update frequency [will update to the *latest release tag*]:" # Clarified update target
    echo "  1) Daily"
    echo "  2) Weekly"
    echo "  3) Monthly"
    echo "  4) Never [Cancel]"
    read -p "Enter your choice [1-4]: " freq_choice

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

    # --- Use awk to remove existing blocks ---
    # Escape the identifier for awk pattern matching (ensure literal match)
    local escaped_identifier_for_awk
    # Escape for extended regex used by awk
    escaped_identifier_for_awk=$(echo "$cron_identifier" | sed 's/[][\\.*^$(){}|+?]/\\\\&/g')
    local awk_script='/__PATTERN__/ { skip=1; next } skip == 1 { skip=0; next } { print }'
    # Replace placeholder with escaped identifier, using ^ to anchor at start of line
    awk_script="${awk_script/__PATTERN__/^$escaped_identifier_for_awk$}"

    # Filter the crontab, preserving other lines
    filtered_cron=$(echo "$current_cron" | awk "$awk_script")
    # --- End awk filtering ---

    # Prepare the new crontab content
    local new_cron_content

    # Trim potential leading/trailing whitespace/newlines from filtered_cron
    trimmed_filtered_cron=$(echo "$filtered_cron" | awk 'NF') # Get non-empty lines
    if [ -n "$trimmed_filtered_cron" ]; then
        # Append the new block with a preceding newline
        new_cron_content=$(printf "%s\n%s\n%s" "$trimmed_filtered_cron" "$cron_identifier" "$cron_command")
    else
        # Filtered cron was empty, just add the new block
        new_cron_content=$(printf "%s\n%s" "$cron_identifier" "$cron_command")
    fi

    # Load the new crontab content
    echo "$new_cron_content" | crontab -u root -
    if [ $? -eq 0 ]; then
        print_success "Cron job for automatic updates set successfully."
        print_info "Update logs will be written to $LOG_FILE"
        # Ensure log file exists and is writable
        touch "$LOG_FILE" || print_warning "Could not touch log file $LOG_FILE"
        chown root:root "$LOG_FILE" || print_warning "Could not chown log file $LOG_FILE"
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
    if [ -n "$MODE_UPDATE" ]; then # Check if MODE_UPDATE is non-empty
        return 0
    fi

    local cron_exists=false
    # Define the exact pattern to search for, escaping regex characters for grep
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]" # Use the variable
    local identifier_for_grep_check
    # Escape only basic regex metachars for standard grep -q
    identifier_for_grep_check=$(echo "$cron_identifier" | sed 's/[]$.*^[]/\\&/g')

    # Check if cron job exists for root user, anchored to start of line
    # Use grep -q for silent check based on exit status
    if crontab -l -u root 2>/dev/null | grep -q "^${identifier_for_grep_check}$"; then
        cron_exists=true
    fi

    echo "" # Add spacing

    if [ "$cron_exists" = true ]; then
        print_info "Automatic updates for Pulse appear to be currently ENABLED."
        print_info "[Cron job will update to the latest release tag when run]" # Clarification
        echo -e "Choose an action:"
        echo -e "  1) Keep current schedule [Do nothing]"
        echo -e "  2) Disable automatic updates"
        echo -e "  3) Change update schedule"
        read -p "Enter your choice [1-3]: " cron_manage_choice

        case $cron_manage_choice in
            1) print_info "Keeping current automatic update schedule.";;
            2) disable_cron_update ;; # Call function to remove the job
            3) setup_cron_update ;; # Call function to prompt for new schedule and update
            *) print_warning "Invalid choice. No changes made to automatic updates.";;
        esac
    else
        print_info "Automatic updates for Pulse appear to be currently DISABLED."
        print_info "[Cron job would update to the latest release tag when run]" # Clarification
        read -p "Do you want to set up automatic updates for Pulse? [Y/n]: " setup_cron_confirm
        if [[ ! "$setup_cron_confirm" =~ ^[Nn]$ ]]; then # Proceed if not 'N' or 'n' [Default Yes]
            setup_cron_update
        else
            print_info "Skipping automatic update setup."
        fi
    fi
}

final_instructions() {
    # Try to get the IP address of the container
    local ip_address
    ip_address=$(hostname -I | awk '{print $1}') # <--- Line 1155 (UNCOMMENTED)
    local port_value
    # Read the port from the .env file, fallback to default if not found/readable
    local env_path="$PULSE_DIR/.env"
    if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
        port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
    else
        port_value="7655" # Default port
    fi

    # Get current checked out tag for final message
    local final_tag=""
    if [ -d "$PULSE_DIR/.git" ]; then
        cd "$PULSE_DIR" || print_warning "Could not cd to $PULSE_DIR to get final tag."
        final_tag=$(get_current_local_tag)
        cd .. # Go back
    fi

    echo ""
    print_success "Pulse for Proxmox VE installation/update complete!"
    if [ -n "$final_tag" ]; then
        print_success "Current version installed: $final_tag"
    fi
    echo "-------------------------------------------------------------"
    print_info "You should be able to access the Pulse dashboard at:"
    if [ -n "$ip_address" ]; then # <--- UNCOMMENTED
        echo "  http://$ip_address:$port_value" # <--- UNCOMMENTED
    else # <--- UNCOMMENTED
        echo "  http://<YOUR-LXC-IP>:$port_value"
        print_warning "Could not automatically determine the LXC IP address."
    fi # <--- UNCOMMENTED
    echo ""
    print_info "The Pulse service $SERVICE_NAME is running and enabled on boot."
    print_info "To check the status: sudo systemctl status $SERVICE_NAME"
    print_info "To view logs: sudo journalctl -u $SERVICE_NAME -f"
    echo "-------------------------------------------------------------"
}


# --- Main Execution --- Refactored
check_root

# ---> MODIFIED: Check flag instead of env var < ---
if [ -z "$MODE_UPDATE" ] && [ "$INSTALLER_WAS_REEXECUTED" != "true" ]; then
    print_dependency_info
fi

# Check for self-update - runs regardless of whether info was printed
self_update_check || print_warning "Installer self-check failed, proceeding anyway..."

# Check installation status and determine user's desired action first
# This also determines TARGET_TAG
check_installation_status_and_determine_action

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
        # ---> Self-update check moved to the beginning <---

        # Only install dependencies if installing or updating
        print_info "Proceeding with install/update. Installing prerequisites..."
        apt_update_upgrade || exit 1
        install_dependencies || exit 1 # Installs git, curl, diffutils
        setup_node || exit 1
        create_pulse_user || exit 1
        print_success "Prerequisites installed."

        # Now perform the specific action
        if [ "$INSTALL_MODE" = "install" ]; then
            print_info "Starting installation..."
            # --- Installation specific steps ---

            # If TARGET_TAG was specified, check it exists BEFORE cloning
            if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                 # Check tag existence remotely using ls-remote [requires git]
                 if ! git ls-remote --tags --exit-code origin "refs/tags/$SPECIFIED_VERSION_TAG"; then
                     print_error "Specified version tag '$SPECIFIED_VERSION_TAG' not found on remote repository."
                     exit 1
                 else
                     print_info "Specified version tag '$SPECIFIED_VERSION_TAG' confirmed."
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                 fi
            fi

            print_info "Cloning Pulse repository into $PULSE_DIR..."
            # Clone shallowly initially? No, need history for tags.
            if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
                print_success "Repository cloned successfully."
                cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR after clone."; exit 1; }
                # Set ownership early so subsequent git commands run as pulse user
                chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR" || print_warning "Failed initial chown after clone."
            else
                print_error "Failed to clone repository."
                exit 1
            fi

            # Determine TARGET_TAG if not specified [find latest tag]
            if [ -z "$TARGET_TAG" ]; then
                TARGET_TAG=$(get_latest_remote_tag) # Already fetched during check if update, fetch here if install
                if [ $? -ne 0 ] || [ -z "$TARGET_TAG" ]; then
                     print_error "Could not determine latest release tag to install."
                     cd ..; exit 1
                fi
                print_info "Determined latest version tag: $TARGET_TAG"
            fi

            # Checkout the target tag
            print_info "Checking out target version tag '$TARGET_TAG'..."
            if ! sudo -u "$PULSE_USER" git checkout "$TARGET_TAG"; then
                 print_error "Failed to checkout tag '$TARGET_TAG' after cloning."
                 cd ..; exit 1
            fi
            print_success "Checked out version $TARGET_TAG."
            cd .. # Back to original dir before dependency install

            install_npm_deps || exit 1 # Installs root and server [NOW INCLUDES DEV]

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
            cd ..

            set_permissions || exit 1 # Set permissions AFTER building css
            configure_environment || exit 1 # Prompt user for details
            setup_systemd_service || exit 1 # Create, enable, start service
            final_instructions
            prompt_for_cron_setup # Ask about cron on fresh install

        else # Update mode [TARGET_TAG determined during check]
            print_info "Starting update to version $TARGET_TAG..."
            # --- Update specific steps ---
            if perform_update; then # perform_update now uses TARGET_TAG
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