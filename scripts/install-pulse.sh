#!/bin/bash


NODE_MAJOR_VERSION=20
PULSE_DIR="/opt/pulse-proxmox"
PULSE_USER="pulse"
SERVICE_NAME="pulse-monitor.service"
SCRIPT_NAME="install-pulse.sh"
LOG_FILE="/var/log/pulse_update.log"
SCRIPT_ABS_PATH=""
REPO_URL="https://github.com/rcourtman/Pulse.git"
SCRIPT_RAW_URL="https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh"
CURRENT_SCRIPT_COMMIT_SHA="5d60cc0"
INSTALLER_VERSION="0.0.0"

MODE_UPDATE=""
INSTALL_MODE=""  
SPECIFIED_VERSION_TAG=""
TARGET_TAG=""
SPECIFIED_BRANCH=""
TARGET_BRANCH=""
INSTALLER_WAS_REEXECUTED=false

if command -v readlink &> /dev/null && readlink -f "$0" &> /dev/null; then
    SCRIPT_ABS_PATH=$(readlink -f "$0")
else
     if [[ "$0" == /* ]]; then
        SCRIPT_ABS_PATH="$0"
     else
        SCRIPT_ABS_PATH="$(pwd)/$0"
     fi
     if [ ! -f "$SCRIPT_ABS_PATH" ]; then
          SCRIPT_ABS_PATH=""
     fi
fi

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
                echo "Error: --version requires a tag name [e.g., v3.2.3]" >&2
                exit 1
            fi
            ;;
        --branch)
            if [[ -n "$2" ]] && [[ "$2" != --* ]]; then
                SPECIFIED_BRANCH="$2"
                shift 2
            else
                echo "Error: --branch requires a branch name [e.g., feature/pve-backups]" >&2
                exit 1
            fi
            ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
done

# Validate that both --version and --branch aren't specified
if [[ -n "$SPECIFIED_VERSION_TAG" ]] && [[ -n "$SPECIFIED_BRANCH" ]]; then
    echo "Error: Cannot specify both --version and --branch. Please choose one." >&2
    exit 1
fi

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

print_dependency_info() {
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
    echo "See README.md for more details about Pulse: https://github.com/rcourtman/Pulse"
    echo ""
     local dep_text
     dep_text=$(cat << 'EOF'
-----------------------------------------------------
Required Dependencies:
Core Tools: curl, git, sudo, gpg, diffutils (via apt)
Pulse Runtime: Node.js & npm (via NodeSource)
Installer Self-Update: jq (via apt)
-----------------------------------------------------
EOF
     )
     print_info "$dep_text"

    local confirm_proceed

    read -p "Do you want to proceed with the installation/update? [Y/n]: " confirm_proceed
    if [[ "$confirm_proceed" =~ ^[Nn]$ ]]; then
        print_error "Operation aborted by user."
        exit 1
    fi
    print_info "Proceeding..."
}

self_update_check() {
    if [ ! -t 0 ] || [ -n "$MODE_UPDATE" ] || [ -z "$SCRIPT_ABS_PATH" ]; then
        return 0
    fi

    if ! command -v curl &> /dev/null; then
        print_warning "curl command not found, skipping installer self-update check."
        return 0
    fi
    if ! command -v jq &> /dev/null; then
        print_info "jq command not found, attempting to install it for self-update check..."
        if command -v apt-get &> /dev/null; then
            apt-get update -qq > /dev/null
            if apt-get install -y -qq jq > /dev/null; then
                print_success "jq installed successfully."
            else
                local jq_install_exit_code=$?
                print_warning "Failed to automatically install jq (Exit code: $jq_install_exit_code)."
                print_warning "Please install jq manually (e.g., sudo apt-get install jq) to enable installer updates."
                print_warning "Skipping installer self-update check for this run."
                return 0
            fi
        else
             print_warning "apt-get not found. Cannot automatically install jq."
             print_warning "Please install jq manually to enable installer updates."
             print_warning "Skipping installer self-update check for this run."
             return 0
        fi
    fi
    local git_ok=false
    if [ -d "$PULSE_DIR/.git" ] && command -v git &> /dev/null; then
        git_ok=true
    fi
    if [ "$git_ok" = false ]; then
        if [ -d "$PULSE_DIR/.git" ]; then
            print_warning "git command not found within $PULSE_DIR, skipping installer self-update check."
            return 0
        else
             print_info "Git not found or $PULSE_DIR not a repo yet, proceeding with API check."
        fi
    fi

    print_info "Checking for updates to the installer script itself (using GitHub API)..."

    local owner="rcourtman"
    local repo="Pulse"
    local script_relative_path="scripts/install-pulse.sh"
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

    local current_local_sha="$CURRENT_SCRIPT_COMMIT_SHA"
    if [ -z "$current_local_sha" ]; then
        print_warning "Could not find embedded CURRENT_SCRIPT_COMMIT_SHA in the script. Skipping self-update check."
        return 0
    fi

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

            print_info "[DEBUG] Updating embedded SHA in temp script to $latest_remote_sha..."
            local processed_temp_script="${temp_script}.processed"
            local line_updated=false
            while IFS= read -r line || [[ -n "$line" ]]; do
                if [[ "$line" == CURRENT_SCRIPT_COMMIT_SHA=* ]]; then
                    echo "CURRENT_SCRIPT_COMMIT_SHA=\"$latest_remote_sha\"" >> "$processed_temp_script"
                    line_updated=true
                else
                    echo "$line" >> "$processed_temp_script"
                fi
            done < "$temp_script"

            if [ "$line_updated" = false ]; then
                print_error "Failed to find and update CURRENT_SCRIPT_COMMIT_SHA line in downloaded script!"
                rm -f "$temp_script" "$processed_temp_script"
                return 1
            fi

            if ! chmod +x "$processed_temp_script"; then
                print_error "Failed to make processed temporary script executable."
                rm -f "$temp_script" "$processed_temp_script"
                return 1
            fi

            if ! mv "$processed_temp_script" "$SCRIPT_ABS_PATH"; then
                 print_error "Failed to replace the current script file with processed version."
                 rm -f "$temp_script" "$processed_temp_script"
                 return 1
            fi
            rm -f "$temp_script"

            print_success "Installer updated successfully to commit ${latest_remote_sha:0:7}."
            print_info "Re-executing with updated installer..."
            exec bash "$SCRIPT_ABS_PATH" --installer-reexecuted "$@"
            print_error "Failed to re-execute the updated script. Please re-run manually: sudo bash $SCRIPT_ABS_PATH"
            exit 1
        else
            print_info "Skipping installer update. Continuing with the current version."
            return 0
        fi
    else
        print_info "Installer script is up-to-date (Commit: $current_local_sha)"
        return 0
    fi
}

get_latest_remote_tag() {
    local latest_tag
    print_info "Fetching latest remote tags..." >&2
    if ! sudo -u "$PULSE_USER" git fetch origin --tags --force >/dev/null 2>&1; then
        print_warning "Could not fetch remote tags." >&2
        return 1
    fi
    latest_tag=$(sudo -u "$PULSE_USER" git tag -l 'v*' --sort='-version:refname' | head -n 1)
    if [ -z "$latest_tag" ]; then
        print_warning "Could not determine the latest remote release tag." >&2
        return 1
    fi
    echo "$latest_tag"
    return 0
}

check_remote_tag_exists() {
    local tag_to_check=$1
    if [ -z "$tag_to_check" ]; then return 1; fi
    print_info "Checking if remote tag '$tag_to_check' exists..."
    if sudo -u "$PULSE_USER" git ls-remote --tags origin "refs/tags/$tag_to_check" | grep -q "refs/tags/$tag_to_check$"; then
        print_info "Remote tag '$tag_to_check' found."
        return 0
    else
        print_error "Remote tag '$tag_to_check' not found."
        return 1
    fi
}

get_current_local_tag() {
    local current_tag
    current_tag=$(sudo -u "$PULSE_USER" git describe --tags --exact-match HEAD 2>/dev/null)
    if [ $? -ne 0 ]; then
         current_tag=$(sudo -u "$PULSE_USER" git describe --tags --abbrev=0 HEAD 2>/dev/null)
         if [ $? -ne 0 ]; then
             current_tag=""
         fi
    fi
    echo "$current_tag"
}

check_release_exists() {
    local tag_to_check=$1
    if [ -z "$tag_to_check" ]; then return 1; fi
    
    print_info "Checking if release '$tag_to_check' has available assets..."
    local release_url="https://api.github.com/repos/rcourtman/Pulse/releases/tags/$tag_to_check"
    
    if command -v curl >/dev/null 2>&1; then
        if curl -sSf "$release_url" >/dev/null 2>&1; then
            print_info "Release '$tag_to_check' found with assets."
            return 0
        fi
    fi
    
    print_warning "Release '$tag_to_check' not found or no assets available."
    return 1
}

download_and_extract_tarball() {
    local tag=$1
    local target_dir=$2
    
    if [ -z "$tag" ] || [ -z "$target_dir" ]; then
        print_error "download_and_extract_tarball: Missing required parameters."
        return 1
    fi
    
    local tarball_url="https://github.com/rcourtman/Pulse/releases/download/$tag/pulse-${tag#v}.tar.gz"
    local temp_tarball="/tmp/pulse-$tag.tar.gz"
    local temp_extract_dir="/tmp/pulse-extract-$$"
    
    print_info "Downloading Pulse $tag tarball..."
    print_info "Download URL: $tarball_url"
    
    if ! curl -sL "$tarball_url" -o "$temp_tarball"; then
        print_error "Failed to download tarball from $tarball_url"
        return 1
    fi
    
    # Show download info for debugging
    local file_size=$(stat -f%z "$temp_tarball" 2>/dev/null || stat -c%s "$temp_tarball" 2>/dev/null || echo "unknown")
    print_info "Downloaded file size: $file_size bytes"
    
    # Verify downloaded file is actually a gzip file
    local file_type=$(file "$temp_tarball")
    print_info "Downloaded file type: $file_type"
    
    if ! echo "$file_type" | grep -q "gzip compressed"; then
        print_error "Downloaded file is not a valid gzip archive!"
        print_error "File type: $file_type"
        print_error "First 200 bytes as hex:"
        head -c 200 "$temp_tarball" | xxd
        print_error "First 200 bytes as text:"
        head -c 200 "$temp_tarball" | cat -v
        print_error "URL attempted: $tarball_url"
        rm -f "$temp_tarball"
        return 1
    fi
    
    print_success "Tarball download verified as valid gzip archive"
    
    print_info "Extracting tarball..."
    mkdir -p "$temp_extract_dir"
    if ! tar -xzf "$temp_tarball" -C "$temp_extract_dir" --strip-components=1; then
        print_error "Failed to extract tarball"
        rm -f "$temp_tarball"
        rm -rf "$temp_extract_dir"
        return 1
    fi
    
    # Create target directory if it doesn't exist
    if [ ! -d "$target_dir" ]; then
        mkdir -p "$target_dir"
        chown "$PULSE_USER":"$PULSE_USER" "$target_dir"
    fi
    
    # Move extracted content to target directory
    print_info "Installing files to $target_dir..."
    # Enable dotglob to include hidden files like .env.example
    shopt -s dotglob
    if ! cp -r "$temp_extract_dir"/* "$target_dir/"; then
        print_error "Failed to copy files to target directory"
        shopt -u dotglob
        rm -f "$temp_tarball"
        rm -rf "$temp_extract_dir"
        return 1
    fi
    shopt -u dotglob
    
    # Set proper ownership
    chown -R "$PULSE_USER":"$PULSE_USER" "$target_dir"
    
    # Cleanup
    rm -f "$temp_tarball"
    rm -rf "$temp_extract_dir"
    
    print_success "Tarball extracted and installed successfully."
    return 0
}

perform_tarball_update() {
    if [ -z "$TARGET_TAG" ]; then
        print_error "Target version tag not determined. Cannot update via tarball."
        return 1
    fi
    
    print_info "Attempting to update Pulse to version $TARGET_TAG using tarball..."
    
    # Check if release exists before proceeding
    if ! check_release_exists "$TARGET_TAG"; then
        print_warning "Release tarball not available for $TARGET_TAG, falling back to git..."
        return 1
    fi
    
    # Create comprehensive backup of user data
    local user_data_backup=""
    if [ -d "$PULSE_DIR" ]; then
        cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; return 1; }
        if user_data_backup=$(backup_user_data); then
            print_success "User data backup created for tarball update"
        else
            print_warning "Failed to create user data backup, proceeding anyway..."
        fi
        cd ..
    fi
    
    local script_backup_path="/tmp/${SCRIPT_NAME}.bak"
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Backing up current installer script to $script_backup_path..."
        if cp "$SCRIPT_ABS_PATH" "$script_backup_path"; then
             print_success "Installer script backed up."
        else
             print_warning "Failed to back up installer script."
             script_backup_path=""
        fi
    else
        script_backup_path=""
    fi

    # Backup current installation directory (full backup as fallback)
    local backup_dir="/tmp/pulse-backup-$(date +%s)"
    if [ -d "$PULSE_DIR" ]; then
        print_info "Creating full installation backup as safety measure..."
        if ! cp -r "$PULSE_DIR" "$backup_dir"; then
            print_warning "Failed to backup current installation, continuing anyway..."
        fi
    fi
    
    # Download and extract new version
    if download_and_extract_tarball "$TARGET_TAG" "$PULSE_DIR"; then
        cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; return 1; }
        
        # Restore installer script if we backed it up
        if [ -n "$script_backup_path" ] && [ -f "$script_backup_path" ]; then
            print_info "Restoring potentially updated installer script from backup..."
            if cp "$script_backup_path" "$PULSE_DIR/scripts/$SCRIPT_NAME"; then
                print_success "Installer script restored."
            fi
            rm -f "$script_backup_path"
        fi
        
        # The tarball should already include built assets, but ensure permissions are correct
        print_info "Setting permissions for $PULSE_DIR..."
        chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR"
        chmod +x "$PULSE_DIR/scripts/$SCRIPT_NAME" 2>/dev/null || true
        
        # Check if node_modules exists in tarball, if not, install dependencies
        if [ ! -d "$PULSE_DIR/node_modules" ]; then
            print_info "Installing NPM dependencies in $PULSE_DIR (root project directory)..."
            if npm install --omit=dev --silent >/dev/null 2>&1; then
                print_success "NPM dependencies installed successfully in $PULSE_DIR."
            else
                print_error "Failed to install NPM dependencies in $PULSE_DIR."
                if [ -d "$backup_dir" ]; then
                    print_info "Restoring from backup..."
                    rm -rf "$PULSE_DIR"
                    mv "$backup_dir" "$PULSE_DIR"
                fi
                return 1
            fi
        else
            print_info "NPM dependencies already present in tarball."
        fi
        
        # Always check if CSS assets are valid after tarball extraction
        print_info "Verifying CSS assets in $PULSE_DIR..."
        
        # Check if output.css exists and is valid (not corrupted with HTML content)
        if [ ! -f "$PULSE_DIR/src/public/output.css" ]; then
            print_error "Critical: output.css file is missing after tarball extraction. This indicates a problem with the tarball or extraction process."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                print_success "CSS file recovered using direct extraction."
            else
                print_error "Failed to recover CSS file. Frontend may not display correctly."
            fi
        elif [ ! -s "$PULSE_DIR/src/public/output.css" ]; then
            print_error "Critical: output.css file is empty after tarball extraction."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                print_success "CSS file recovered using direct extraction."
            else
                print_error "Failed to recover CSS file. Frontend may not display correctly."
            fi
        elif head -1 "$PULSE_DIR/src/public/output.css" 2>/dev/null | grep -q "<!DOCTYPE\|<html\|<head"; then
            print_warning "output.css contains HTML instead of CSS - attempting to recover from tarball..."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                # Verify the recovery worked
                if [ -s "$PULSE_DIR/src/public/output.css" ] && ! head -1 "$PULSE_DIR/src/public/output.css" 2>/dev/null | grep -q "<!DOCTYPE\|<html\|<head"; then
                    print_success "CSS file recovered and verified."
                    chown "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR/src/public/output.css"
                else
                    print_error "CSS recovery failed. Frontend may not display correctly."
                fi
            else
                print_error "Failed to recover CSS file. Frontend may not display correctly."
            fi
        else
            print_success "CSS assets verified and ready."
        fi
        
        # Restore user data after successful tarball extraction
        if [ -n "$user_data_backup" ] && [ -d "$user_data_backup" ]; then
            if restore_user_data "$user_data_backup"; then
                print_success "User configuration restored after tarball update"
            else
                print_warning "Failed to restore some user configuration"
            fi
            rm -rf "$user_data_backup"
        fi
        
        # Cleanup backup if successful
        [ -d "$backup_dir" ] && rm -rf "$backup_dir"
        
        print_success "Tarball update completed successfully."
        return 0
    else
        print_error "Failed to download/extract tarball."
        
        # Restore backup if available
        if [ -d "$backup_dir" ]; then
            print_info "Restoring from full backup..."
            rm -rf "$PULSE_DIR"
            mv "$backup_dir" "$PULSE_DIR"
        fi
        
        # Cleanup failed backups
        [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
        [ -n "$user_data_backup" ] && rm -rf "$user_data_backup"
        return 1
    fi
}

# Emergency recovery function for when updates fail
emergency_recovery() {
    local backup_dir="$1"
    
    print_error "==================== EMERGENCY RECOVERY ===================="
    print_error "Update failed catastrophically. Attempting emergency recovery..."
    
    if [ -n "$backup_dir" ] && [ -d "$backup_dir" ]; then
        print_info "Restoring from emergency backup: $backup_dir"
        if [ -d "$PULSE_DIR" ]; then
            rm -rf "$PULSE_DIR.failed" 2>/dev/null
            mv "$PULSE_DIR" "$PULSE_DIR.failed" 2>/dev/null
        fi
        
        if mv "$backup_dir" "$PULSE_DIR"; then
            print_success "Emergency recovery successful!"
            print_info "Your original configuration has been restored."
            print_info "Failed installation moved to: $PULSE_DIR.failed"
            return 0
        else
            print_error "Emergency recovery failed!"
        fi
    fi
    
    print_error "============== MANUAL RECOVERY REQUIRED ================"
    print_error "Automatic recovery failed. You may need to:"
    print_error "1. Manually restore from backup if available"
    print_error "2. Re-run the installer: sudo bash install-pulse.sh"
    print_error "3. Reconfigure your .env file with Proxmox credentials"
    print_error "4. Contact support: https://github.com/rcourtman/Pulse/issues"
    print_error "======================================================="
    
    return 1
}

perform_tarball_install() {
    if [ -z "$TARGET_TAG" ]; then
        print_error "Target version tag not determined. Cannot install via tarball."
        return 1
    fi
    
    print_info "Installing Pulse $TARGET_TAG using tarball..."
    
    # Check if release exists before proceeding
    if ! check_release_exists "$TARGET_TAG"; then
        print_warning "Release tarball not available for $TARGET_TAG, falling back to git..."
        return 1
    fi
    
    # Download and extract
    if download_and_extract_tarball "$TARGET_TAG" "$PULSE_DIR"; then
        cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; return 1; }
        
        # The tarball should already include built assets, but check if we need to install dependencies
        if [ ! -d "$PULSE_DIR/node_modules" ]; then
            print_info "Installing NPM dependencies in $PULSE_DIR (root project directory)..."
            if npm install --omit=dev --silent >/dev/null 2>&1; then
                print_success "NPM dependencies installed successfully in $PULSE_DIR."
            else
                print_error "Failed to install NPM dependencies in $PULSE_DIR."
                return 1
            fi
        else
            print_info "NPM dependencies already present in tarball."
        fi
        
        # Always check if CSS assets are valid after tarball extraction
        print_info "Verifying CSS assets in $PULSE_DIR..."
        
        # Check if output.css exists and is valid (not corrupted with HTML content)
        if [ ! -f "$PULSE_DIR/src/public/output.css" ]; then
            print_error "Critical: output.css file is missing after tarball extraction. Attempting recovery..."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                print_success "CSS file recovered using direct extraction."
                chown "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR/src/public/output.css"
            else
                print_error "Failed to recover CSS file. Installation will continue but frontend may not display correctly."
            fi
        elif [ ! -s "$PULSE_DIR/src/public/output.css" ]; then
            print_error "Critical: output.css file is empty after tarball extraction. Attempting recovery..."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                print_success "CSS file recovered using direct extraction."
                chown "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR/src/public/output.css"
            else
                print_error "Failed to recover CSS file. Installation will continue but frontend may not display correctly."
            fi
        elif head -1 "$PULSE_DIR/src/public/output.css" 2>/dev/null | grep -q "<!DOCTYPE\|<html\|<head"; then
            print_warning "output.css contains HTML instead of CSS - attempting to recover from tarball..."
            # Try to re-extract just the CSS file as a fallback
            if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$TARGET_TAG/pulse-${TARGET_TAG#v}.tar.gz" | tar -xzf - --to-stdout "pulse-${TARGET_TAG#v}/src/public/output.css" > "$PULSE_DIR/src/public/output.css" 2>/dev/null; then
                # Verify the recovery worked
                if [ -s "$PULSE_DIR/src/public/output.css" ] && ! head -1 "$PULSE_DIR/src/public/output.css" 2>/dev/null | grep -q "<!DOCTYPE\|<html\|<head"; then
                    print_success "CSS file recovered and verified."
                    chown "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR/src/public/output.css"
                else
                    print_error "CSS recovery failed. Installation will continue but frontend may not display correctly."
                fi
            else
                print_error "Failed to recover CSS file. Installation will continue but frontend may not display correctly."
            fi
        else
            print_success "CSS assets verified and ready."
        fi
        
        print_success "Tarball installation completed successfully."
        return 0
    else
        print_error "Failed to install from tarball."
        return 1
    fi
}

prompt_for_branch_selection() {
    print_info "Fetching available feature branches..."
    local branches
    if [ -d "$PULSE_DIR" ]; then
        cd "$PULSE_DIR" || return 1
        branches=$(sudo -u "$PULSE_USER" git ls-remote --heads origin | { grep -E 'feature/|hotfix/|dev/' || true; } | sed 's|.*refs/heads/||' | sort)
        cd - >/dev/null
    else
        # For fresh installations, fetch from remote
        branches=$(git ls-remote --heads https://github.com/rcourtman/Pulse.git | { grep -E 'feature/|hotfix/|dev/' || true; } | sed 's|.*refs/heads/||' | sort)
    fi
    
    if [ -z "$branches" ]; then
        print_warning "No development branches found."
        return 1
    fi
    
    echo "Available development branches:"
    local i=1
    local branch_array=()
    while IFS= read -r branch; do
        echo "  $i) $branch"
        branch_array+=("$branch")
        i=$((i+1))
    done <<< "$branches"
    echo "  $i) Cancel"
    
    read -p "Select a branch to install [1-$i]: " choice
    
    if [ "$choice" = "$i" ]; then
        return 1
    elif [ "$choice" -ge 1 ] && [ "$choice" -lt "$i" ]; then
        SPECIFIED_BRANCH="${branch_array[$((choice-1))]}"
        return 0
    else
        print_error "Invalid choice."
        return 1
    fi
}

check_installation_status_and_determine_action() {
    if [ -n "$MODE_UPDATE" ]; then
        print_info "Running in non-interactive update mode..."
        INSTALL_MODE="update"
        if [ -n "$SPECIFIED_VERSION_TAG" ]; then
            if [ -d "$PULSE_DIR/.git" ]; then
                 cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
                 if check_remote_tag_exists "$SPECIFIED_VERSION_TAG"; then
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                 else
                     INSTALL_MODE="error"
                     cd ..; return
                 fi
                 cd ..
            else
                print_error "Cannot validate specified version tag: Pulse directory $PULSE_DIR not found."
                INSTALL_MODE="error"; return
            fi
        elif [ -n "$SPECIFIED_BRANCH" ]; then
            if [ -d "$PULSE_DIR/.git" ]; then
                 cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
                 print_info "Checking if remote branch '$SPECIFIED_BRANCH' exists..."
                 if sudo -u "$PULSE_USER" git ls-remote --heads origin "$SPECIFIED_BRANCH" | grep -q "$SPECIFIED_BRANCH" 2>/dev/null; then
                     TARGET_BRANCH="$SPECIFIED_BRANCH"
                     print_info "Will update to branch: $TARGET_BRANCH"
                 else
                     print_error "Remote branch '$SPECIFIED_BRANCH' not found."
                     INSTALL_MODE="error"
                     cd ..; return
                 fi
                 cd ..
            else
                print_error "Cannot validate specified branch: Pulse directory $PULSE_DIR not found."
                INSTALL_MODE="error"; return
            fi
        else
            if [ -d "$PULSE_DIR/.git" ]; then
                cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }
                TARGET_TAG=$(get_latest_remote_tag)
                if [ $? -ne 0 ] || [ -z "$TARGET_TAG" ]; then
                     print_error "Could not determine latest remote tag for update."
                     INSTALL_MODE="error"
                fi
                cd ..
            else
                 print_info "Pulse directory $PULSE_DIR not found. Switching to installation mode."
                 INSTALL_MODE="install"
                 # Determine latest tag for fresh installation
                 TARGET_TAG=$(git ls-remote --tags --sort='-version:refname' https://github.com/rcourtman/Pulse.git | grep 'refs/tags/v' | head -n 1 | sed 's/.*refs\/tags\///' | sed 's/\^{}.*//')
                 if [ -z "$TARGET_TAG" ]; then
                     print_error "Could not determine latest release tag for installation."
                     INSTALL_MODE="error"; return
                 fi
                 print_info "Will install latest version: $TARGET_TAG"
            fi
        fi
        return 0
    fi

    print_info "Checking Pulse installation at $PULSE_DIR..."
    if [ -d "$PULSE_DIR" ]; then
        # Check if this is a valid Pulse installation (either git or tarball)
        if [ -f "$PULSE_DIR/package.json" ] && grep -q '"name".*"pulse"' "$PULSE_DIR/package.json" 2>/dev/null; then
            print_info "Found valid Pulse installation"
            
            # Determine installation type
            local is_git_install=false
            if [ -d "$PULSE_DIR/.git" ]; then
                is_git_install=true
                print_info "Installation type: Git repository"
            else
                print_info "Installation type: Tarball installation"
            fi
        else
            print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse installation."
            print_error "Missing or invalid package.json file."
            print_error "Please remove this directory manually ($PULSE_DIR) or choose a different installation path and re-run the script."
            INSTALL_MODE="error"
            return
        fi
        
        # Handle git installations
        if [ "$is_git_install" = true ]; then
            cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR"; INSTALL_MODE="error"; return; }

            local current_tag
            current_tag=$(get_current_local_tag)

            local latest_tag
            latest_tag=$(get_latest_remote_tag)
            if [ $? -ne 0 ] || [ -z "$latest_tag" ]; then
                print_warning "Could not determine the latest remote release tag. Cannot check for updates reliably."
                INSTALL_MODE="update"
            else
                print_info "Current installed version tag: ${current_tag:-Not on a tag}"
                print_info "Latest available version tag: $latest_tag"

                # Check for specific version/branch requests first
                if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                     if check_remote_tag_exists "$SPECIFIED_VERSION_TAG"; then
                         TARGET_TAG="$SPECIFIED_VERSION_TAG"
                         print_info "Will target specified version: $TARGET_TAG"
                         INSTALL_MODE="update"
                     else
                         INSTALL_MODE="error"; cd ..; return
                     fi
                elif [ -n "$SPECIFIED_BRANCH" ]; then
                    print_info "Checking if remote branch '$SPECIFIED_BRANCH' exists..."
                    if sudo -u "$PULSE_USER" git ls-remote --heads origin "$SPECIFIED_BRANCH" | grep -q "$SPECIFIED_BRANCH" 2>/dev/null; then
                        TARGET_BRANCH="$SPECIFIED_BRANCH"
                        print_info "Will switch to branch: $TARGET_BRANCH"
                        INSTALL_MODE="update"
                    else
                        print_error "Remote branch '$SPECIFIED_BRANCH' not found."
                        INSTALL_MODE="error"; cd ..; return
                    fi
                elif [ -n "$current_tag" ] && [ "$current_tag" = "$latest_tag" ]; then
                    print_info "Pulse is already installed and up-to-date with the latest release $latest_tag."
                    INSTALL_MODE="uptodate"
                    TARGET_TAG="$latest_tag"
                else
                    print_warning "Pulse is installed, but an update to $latest_tag is available."
                    INSTALL_MODE="update"
                    TARGET_TAG="$latest_tag"
                fi
            fi

            if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                 if check_remote_tag_exists "$SPECIFIED_VERSION_TAG"; then
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                     print_info "Will target specified version: $TARGET_TAG"
                     INSTALL_MODE="update"
                 else
                     INSTALL_MODE="error"; cd ..; return
                 fi
            elif [ -n "$SPECIFIED_BRANCH" ]; then
                print_info "Checking if remote branch '$SPECIFIED_BRANCH' exists..."
                if sudo -u "$PULSE_USER" git ls-remote --heads origin "$SPECIFIED_BRANCH" | grep -q "$SPECIFIED_BRANCH" 2>/dev/null; then
                    TARGET_BRANCH="$SPECIFIED_BRANCH"
                    print_info "Will switch to branch: $TARGET_BRANCH"
                    INSTALL_MODE="update"
                else
                    print_error "Remote branch '$SPECIFIED_BRANCH' not found."
                    INSTALL_MODE="error"; cd ..; return
                fi
            else
                TARGET_TAG="$latest_tag"
            fi
            cd ..

            if [ "$INSTALL_MODE" = "uptodate" ]; then
                if [ -n "$MODE_UPDATE" ]; then
                    # In non-interactive mode, skip if up to date
                    print_info "Pulse is already up-to-date. Skipping update."
                    INSTALL_MODE="cancel"
                elif [ -n "$SPECIFIED_VERSION_TAG" ] && [ "$SPECIFIED_VERSION_TAG" != "$current_tag" ]; then
                     print_info "You requested version $SPECIFIED_VERSION_TAG, but $current_tag is installed."
                     echo -e "Choose an action:"
                     echo -e "  1) Manage automatic updates"
                     echo -e "  2) Re-install current version $current_tag"
                     echo -e "  3) Remove Pulse"
                     echo -e "  4) Cancel"
                     read -p "Enter your choice [1-4]: " user_choice
                     case $user_choice in
                         1) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                         2) INSTALL_MODE="update" ;;
                         3) INSTALL_MODE="remove" ;;
                         4) INSTALL_MODE="cancel" ;;
                         *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                     esac
                elif [ -n "$SPECIFIED_BRANCH" ]; then
                    if [ -n "$MODE_UPDATE" ]; then
                        # In non-interactive mode, proceed with branch update
                        INSTALL_MODE="update"
                    else
                        echo -e "Choose an action:"
                        echo -e "  1) Install branch $SPECIFIED_BRANCH (for testing)"
                        echo -e "  2) Remove Pulse"
                        echo -e "  3) Cancel"
                        echo -e "  4) Manage automatic updates"
                        read -p "Enter your choice [1-4]: " user_choice
                        case $user_choice in
                            1) INSTALL_MODE="update" ;;
                            2) INSTALL_MODE="remove" ;;
                            3) INSTALL_MODE="cancel" ;;
                            4) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                            *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                        esac
                    fi
                else
                    if [ -n "$MODE_UPDATE" ]; then
                        # In non-interactive mode, re-install if explicitly requested
                        INSTALL_MODE="update"
                    else
                        echo -e "Choose an action:"
                        echo -e "  1) Manage automatic updates"
                        echo -e "  2) Re-install current version $current_tag"
                        echo -e "  3) Test a feature branch"
                        echo -e "  4) Remove Pulse"
                        echo -e "  5) Cancel"
                        read -p "Enter your choice [1-5]: " user_choice
                        case $user_choice in
                            1) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                            2) INSTALL_MODE="update" ;;
                            3) 
                                if prompt_for_branch_selection; then
                                    INSTALL_MODE="update"
                                else
                                    INSTALL_MODE="cancel"
                                fi
                                ;;
                            4) INSTALL_MODE="remove" ;;
                            5) INSTALL_MODE="cancel" ;;
                            *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                        esac
                    fi
                fi
            elif [ "$INSTALL_MODE" = "update" ]; then
                 if [ -n "$MODE_UPDATE" ]; then
                     # In non-interactive mode, proceed with update
                     :  # Do nothing, keep INSTALL_MODE as "update"
                 elif [ -n "$SPECIFIED_VERSION_TAG" ]; then
                     echo "Choose an action:"
                     echo "  1) Install specified version $SPECIFIED_VERSION_TAG"
                     echo "  2) Remove Pulse"
                     echo "  3) Cancel"
                     echo "  4) Manage automatic updates"
                     read -p "Enter your choice [1-4]: " user_choice
                     case $user_choice in
                         1) INSTALL_MODE="update" ;;
                         2) INSTALL_MODE="remove" ;;
                         3) INSTALL_MODE="cancel" ;;
                         4) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                         *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                     esac
                 elif [ -n "$SPECIFIED_BRANCH" ]; then
                     echo "Choose an action:"
                     echo "  1) Install branch $SPECIFIED_BRANCH (for testing)"
                     echo "  2) Remove Pulse"
                     echo "  3) Cancel"
                     echo "  4) Manage automatic updates"
                     read -p "Enter your choice [1-4]: " user_choice
                     case $user_choice in
                         1) INSTALL_MODE="update" ;;
                         2) INSTALL_MODE="remove" ;;
                         3) INSTALL_MODE="cancel" ;;
                         4) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                         *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                     esac
                 else
                      echo "Choose an action:"
                      echo "  1) Update Pulse to the latest version $TARGET_TAG"
                      echo "  2) Test a feature branch"
                      echo "  3) Remove Pulse"
                      echo "  4) Cancel"
                      echo "  5) Manage automatic updates"
                      read -p "Enter your choice [1-5]: " user_choice
                      case $user_choice in
                          1) INSTALL_MODE="update" ;;
                          2) 
                              if prompt_for_branch_selection; then
                                  INSTALL_MODE="update"
                              else
                                  INSTALL_MODE="cancel"
                              fi
                              ;;
                          3) INSTALL_MODE="remove" ;;
                          4) INSTALL_MODE="cancel" ;;
                          5) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                          *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                      esac
                 fi
            fi

        else
            # Handle tarball installations
            print_info "Handling tarball installation..."
            
            # Get current version from package.json
            local current_version=""
            if [ -f "$PULSE_DIR/package.json" ]; then
                current_version=$(grep '"version"' "$PULSE_DIR/package.json" | sed 's/.*"version": *"\([^"]*\)".*/\1/')
                print_info "Current installed version: $current_version"
            fi
            
            # Determine latest version for comparison
            local latest_tag=""
            latest_tag=$(git ls-remote --tags --sort='-version:refname' https://github.com/rcourtman/Pulse.git | grep 'refs/tags/v' | head -n 1 | sed 's/.*refs\/tags\///' | sed 's/\^{}.*//')
            if [ -n "$latest_tag" ]; then
                local latest_version=$(echo "$latest_tag" | sed 's/^v//')
                print_info "Latest available version: $latest_version"
                
                # Compare versions
                if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                    TARGET_TAG="$SPECIFIED_VERSION_TAG"
                    print_info "Will target specified version: $TARGET_TAG"
                    INSTALL_MODE="update"
                elif [ -n "$SPECIFIED_BRANCH" ]; then
                    TARGET_BRANCH="$SPECIFIED_BRANCH"
                    print_info "Will switch to branch: $TARGET_BRANCH"
                    INSTALL_MODE="update"
                elif [ "$current_version" = "$latest_version" ]; then
                    print_info "Pulse is already up-to-date with the latest release $latest_tag."
                    INSTALL_MODE="uptodate"
                    TARGET_TAG="$latest_tag"
                else
                    print_warning "Pulse is installed, but an update to $latest_tag is available."
                    INSTALL_MODE="update"
                    TARGET_TAG="$latest_tag"
                fi
            else
                print_warning "Could not determine latest version. Will proceed with update."
                INSTALL_MODE="update"
            fi
            
            # Handle user interaction for tarball installations
            if [ "$INSTALL_MODE" = "uptodate" ]; then
                if [ -n "$MODE_UPDATE" ]; then
                    print_info "Pulse is already up-to-date. Skipping update."
                    INSTALL_MODE="cancel"
                else
                    echo "Choose an action:"
                    echo "  1) Manage automatic updates"
                    echo "  2) Re-install current version"
                    echo "  3) Test a feature branch"
                    echo "  4) Remove Pulse"
                    echo "  5) Cancel"
                    read -p "Enter your choice [1-5]: " user_choice
                    case $user_choice in
                        1) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                        2) INSTALL_MODE="update" ;;
                        3) 
                            if prompt_for_branch_selection; then
                                INSTALL_MODE="update"
                            else
                                INSTALL_MODE="cancel"
                            fi
                            ;;
                        4) INSTALL_MODE="remove" ;;
                        5) INSTALL_MODE="cancel" ;;
                        *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                    esac
                fi
            elif [ "$INSTALL_MODE" = "update" ]; then
                if [ -n "$MODE_UPDATE" ]; then
                    # In non-interactive mode, proceed with update
                    :  # Do nothing, keep INSTALL_MODE as "update"
                else
                    echo "Choose an action:"
                    echo "  1) Update Pulse to the latest version ${TARGET_TAG:-$latest_tag}"
                    echo "  2) Test a feature branch"
                    echo "  3) Remove Pulse"
                    echo "  4) Cancel"
                    echo "  5) Manage automatic updates"
                    read -p "Enter your choice [1-5]: " user_choice
                    case $user_choice in
                        1) INSTALL_MODE="update" ;;
                        2) 
                            if prompt_for_branch_selection; then
                                INSTALL_MODE="update"
                            else
                                INSTALL_MODE="cancel"
                            fi
                            ;;
                        3) INSTALL_MODE="remove" ;;
                        4) INSTALL_MODE="cancel" ;;
                        5) prompt_for_cron_setup; INSTALL_MODE="cancel" ;;
                        *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                    esac
                fi
            fi
        fi
    else
        print_info "Pulse is not currently installed."
        if [ -n "$SPECIFIED_VERSION_TAG" ]; then
            TARGET_TAG="$SPECIFIED_VERSION_TAG"
            print_info "Will attempt to install specified version: $TARGET_TAG"
            INSTALL_MODE="install"
        elif [ -n "$SPECIFIED_BRANCH" ]; then
            TARGET_BRANCH="$SPECIFIED_BRANCH"
            print_info "Will attempt to install from branch: $TARGET_BRANCH"
            INSTALL_MODE="install"
        else
            if [ -n "$MODE_UPDATE" ]; then
                # In non-interactive mode, install latest version
                INSTALL_MODE="install"
            else
                echo "Choose an action:"
                echo "  1) Install Pulse [latest version]"
                echo "  2) Test a feature branch"
                echo "  3) Cancel"
                read -p "Enter your choice [1-3]: " user_choice
                case $user_choice in
                    1) INSTALL_MODE="install" ;;
                    2) 
                        if prompt_for_branch_selection; then
                            INSTALL_MODE="install"
                        else
                            INSTALL_MODE="cancel"
                        fi
                        ;;
                    3) INSTALL_MODE="cancel" ;;
                    *) print_error "Invalid choice."; INSTALL_MODE="error" ;;
                esac
            fi
        fi
    fi
}

run_apt_update() {
    print_info "Updating package lists..."
    if apt-get update -qq > /dev/null; then
        print_success "Package lists updated."
    else
        print_error "Failed to update package lists."
        return 1
    fi
    return 0
}

run_apt_upgrade_system() {
    print_info "Upgrading system packages (this may take a while)..."
    if apt-get upgrade -y -qq > /dev/null; then
        print_success "System packages upgraded."
    else
        print_error "Failed to upgrade system packages."
        return 1
    fi
    return 0
}

install_dependencies() {
    print_info "Checking and installing necessary dependencies (git, curl, sudo, gpg, diffutils)..."
    local required_deps=("git" "curl" "sudo" "gpg" "diffutils")
    local missing_deps=()
    local dep_to_install

    for dep in "${required_deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            missing_deps+=("$dep")
        fi
    done

    if [ ${#missing_deps[@]} -gt 0 ]; then
        print_info "Missing dependencies: ${missing_deps[*]}. Attempting to install..."
        dep_to_install=$(IFS=","; echo "${missing_deps[*]}") # For print message
        if apt-get install -y -qq "${missing_deps[@]}" > /dev/null; then
            print_success "Successfully installed: ${dep_to_install}."
        else
            print_error "Failed to install: ${dep_to_install}."
            exit 1
        fi
    else
        print_info "All core dependencies are already installed."
    fi
    print_success "Dependency check complete."
}

setup_node() {
    print_info "Setting up Node.js repository [NodeSource]..."
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

    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not found. Please install it first [apt-get install curl]."
        exit 1
    fi

    KEYRING_DIR="/usr/share/keyrings"
    KEYRING_FILE="$KEYRING_DIR/nodesource.gpg"
    if [ ! -d "$KEYRING_DIR" ]; then
        mkdir -p "$KEYRING_DIR" || { print_error "Failed to create $KEYRING_DIR"; exit 1; }
    fi
    print_info "Downloading/refreshing NodeSource GPG key..."
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --yes --dearmor -o "$KEYRING_FILE"
    if [ $? -ne 0 ]; then
        print_error "Failed to download or process NodeSource GPG key."
        rm -f "$KEYRING_FILE"
        exit 1
    fi

    echo "deb [signed-by=$KEYRING_FILE] https://deb.nodesource.com/node_$NODE_MAJOR_VERSION.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list > /dev/null
    if [ $? -ne 0 ]; then
        print_error "Failed to add NodeSource repository to sources list."
        exit 1
    fi

    print_info "Updating package list after adding NodeSource repository..."
    if ! apt-get update > /dev/null; then
        print_error "Failed to update package list after adding NodeSource repository."
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
        rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
        exit 1
    fi
}

create_pulse_user() {
    print_info "Creating dedicated user '$PULSE_USER'..."
    if id "$PULSE_USER" &>/dev/null; then
        print_warning "User '$PULSE_USER' already exists. Skipping creation."
    else
        useradd -r -s /bin/false "$PULSE_USER"
        if [ $? -eq 0 ]; then
            print_success "User '$PULSE_USER' created successfully."
        else
            print_error "Failed to create user '$PULSE_USER'."
            exit 1
        fi
    fi
}


# Comprehensive backup of user configuration and data
backup_user_data() {
    local backup_base_dir="/tmp/pulse-config-backup-$(date +%s)"
    mkdir -p "$backup_base_dir" || {
        print_error "Failed to create backup directory $backup_base_dir"
        return 1
    }
    
    print_info "Creating comprehensive backup of user configuration..."
    
    # Files to always preserve
    local preserve_files=(
        ".env"
        "custom-config.json"
        "user-settings.json"
    )
    
    # Directories to preserve (if they exist and contain user data)
    local preserve_dirs=(
        "data"
        "logs"
        "backups"
        "custom"
    )
    
    local backup_success=false
    
    # Backup individual files
    for file in "${preserve_files[@]}"; do
        if [ -f "$PULSE_DIR/$file" ]; then
            local file_backup_dir="$backup_base_dir/$(dirname "$file")"
            mkdir -p "$file_backup_dir"
            if cp "$PULSE_DIR/$file" "$backup_base_dir/$file" 2>/dev/null; then
                print_success "Backed up: $file"
                backup_success=true
            else
                print_warning "Failed to backup: $file"
            fi
        fi
    done
    
    # Backup directories
    for dir in "${preserve_dirs[@]}"; do
        if [ -d "$PULSE_DIR/$dir" ] && [ "$(ls -A "$PULSE_DIR/$dir" 2>/dev/null)" ]; then
            if cp -r "$PULSE_DIR/$dir" "$backup_base_dir/$dir" 2>/dev/null; then
                print_success "Backed up directory: $dir"
                backup_success=true
            else
                print_warning "Failed to backup directory: $dir"
            fi
        fi
    done
    
    if [ "$backup_success" = true ]; then
        echo "$backup_base_dir"
        return 0
    else
        rm -rf "$backup_base_dir" 2>/dev/null
        print_warning "No user data found to backup or all backups failed"
        return 1
    fi
}

# Restore user configuration and data
restore_user_data() {
    local backup_dir="$1"
    
    if [ -z "$backup_dir" ] || [ ! -d "$backup_dir" ]; then
        print_warning "No backup directory provided or directory doesn't exist"
        return 1
    fi
    
    print_info "Restoring user configuration from backup..."
    
    # Restore files and directories
    if [ -d "$backup_dir" ]; then
        if cp -r "$backup_dir"/* "$PULSE_DIR/" 2>/dev/null; then
            # Fix ownership and permissions
            chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR"
            
            # Set secure permissions on sensitive files
            [ -f "$PULSE_DIR/.env" ] && chmod 600 "$PULSE_DIR/.env"
            
            print_success "User configuration restored successfully"
            return 0
        else
            print_error "Failed to restore user configuration"
            return 1
        fi
    fi
    
    return 1
}

perform_update() {
    if [ -z "$TARGET_TAG" ] && [ -z "$TARGET_BRANCH" ]; then
        print_error "Target version tag or branch not determined. Cannot update."
        return 1
    fi
    
    # For version tags, try tarball first, then fall back to git
    if [ -n "$TARGET_TAG" ]; then
        print_info "Attempting to update Pulse to version $TARGET_TAG..."
        
        # Try tarball update first - this is safer and preserves user data
        if perform_tarball_update; then
            return 0
        fi
        
        # Fall back to git update only if tarball fails
        print_warning "Tarball update failed, falling back to git update..."
        print_warning "⚠️  Git update may be more disruptive to user configuration"
    else
        print_info "Attempting to update Pulse to branch $TARGET_BRANCH..."
        print_warning "⚠️  Branch installations are for testing only and may be unstable!"
    fi
    
    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    # Create comprehensive backup of all user data before ANY destructive operations
    local user_data_backup=""
    if user_data_backup=$(backup_user_data); then
        print_success "User data backup created at: $user_data_backup"
    else
        print_warning "Failed to create comprehensive user data backup"
        print_warning "Proceeding with update but configuration may be lost"
    fi

    local script_backup_path="/tmp/${SCRIPT_NAME}.bak"
    if [ -n "$SCRIPT_ABS_PATH" ] && [ -f "$SCRIPT_ABS_PATH" ]; then
        print_info "Backing up current installer script to $script_backup_path..."
        if cp "$SCRIPT_ABS_PATH" "$script_backup_path"; then
             print_success "Installer script backed up."
        else
             print_warning "Failed to back up installer script. Update might revert it."
             script_backup_path=""
        fi
    else
        print_warning "Cannot determine current installer script path. Cannot back it up."
        script_backup_path=""
    fi

    git config --global --add safe.directory "$PULSE_DIR" > /dev/null 2>&1 || print_warning "Could not configure safe.directory for root user."

    # Check git status before destructive operations
    local has_local_changes=false
    if sudo -u "$PULSE_USER" git status --porcelain 2>/dev/null | grep -q .; then
        has_local_changes=true
        print_warning "Local changes detected in repository"
    fi

    print_info "Resetting local repository to discard potential changes [running as user $PULSE_USER]..."
    if ! sudo -u "$PULSE_USER" git reset --hard HEAD; then
        print_error "Failed to reset local repository. Aborting update."
        [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
        [ -n "$user_data_backup" ] && rm -rf "$user_data_backup"
        cd ..
        return 1
    fi
    
    # Only clean untracked files if absolutely necessary
    # Skip git clean entirely and handle conflicts more gracefully
    if [ "$has_local_changes" = true ]; then
        print_info "Checking for untracked files that might conflict..."
        local untracked_files=$(sudo -u "$PULSE_USER" git status --porcelain 2>/dev/null | grep '^??' | cut -c4-)
        
        if [ -n "$untracked_files" ]; then
            print_warning "Found untracked files that may conflict with update:"
            echo "$untracked_files" | while read -r file; do
                print_warning "  - $file"
            done
            
            # Instead of git clean, selectively handle conflicts
            print_info "Moving conflicting files to temporary backup..."
            local conflict_backup="/tmp/pulse-conflicts-$(date +%s)"
            mkdir -p "$conflict_backup"
            
            echo "$untracked_files" | while read -r file; do
                if [ -f "$file" ] || [ -d "$file" ]; then
                    local file_dir="$conflict_backup/$(dirname "$file")"
                    mkdir -p "$file_dir"
                    mv "$file" "$conflict_backup/$file" 2>/dev/null || true
                fi
            done
        fi
    fi

    print_info "Fetching latest changes from git [running as user $PULSE_USER]..."
    if [ -n "$TARGET_TAG" ]; then
        if ! sudo -u "$PULSE_USER" git fetch origin --tags --force; then
            print_error "Failed to fetch latest changes/tags from git."
            [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
            cd ..
            return 1
        fi
        
        if ! sudo -u "$PULSE_USER" git rev-parse "$TARGET_TAG" >/dev/null 2>&1; then
             print_error "Target tag '$TARGET_TAG' could not be found locally after fetch."
             print_error "It might have been deleted remotely or there was a fetch issue."
             [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
             cd ..
             return 1
        fi

        print_info "Checking out target version tag '$TARGET_TAG'..."
        if ! sudo -u "$PULSE_USER" git checkout -f "$TARGET_TAG"; then
            print_error "Failed to checkout tag '$TARGET_TAG'."
            [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
            cd ..
            return 1
        fi
        
        print_info "Verifying package.json was updated..."
        local package_version=$(node -p "require('./package.json').version" 2>/dev/null || echo "unknown")
        local expected_version=$(echo "$TARGET_TAG" | sed 's/^v//')
        if [ "$package_version" != "$expected_version" ] && [ "$package_version" != "unknown" ]; then
            print_warning "package.json version ($package_version) does not match expected version ($expected_version)"
            print_info "Forcing checkout to ensure all files are updated..."
            sudo -u "$PULSE_USER" git checkout -f "$TARGET_TAG" -- .
        fi
    else
        # Branch checkout
        if ! sudo -u "$PULSE_USER" git fetch origin "$TARGET_BRANCH"; then
            print_error "Failed to fetch branch '$TARGET_BRANCH' from git."
            [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
            cd ..
            return 1
        fi
        
        print_info "Checking out branch '$TARGET_BRANCH'..."
        if ! sudo -u "$PULSE_USER" git checkout -B "$TARGET_BRANCH" "origin/$TARGET_BRANCH"; then
            print_error "Failed to checkout branch '$TARGET_BRANCH'."
            [ -n "$script_backup_path" ] && rm -f "$script_backup_path"
            cd ..
            return 1
        fi
        
        print_info "Pulling latest changes from branch..."
        if ! sudo -u "$PULSE_USER" git pull origin "$TARGET_BRANCH"; then
            print_warning "Failed to pull latest changes, continuing with current state"
        fi
    fi

    local current_tag
    current_tag=$(sudo -u "$PULSE_USER" git describe --tags --exact-match HEAD 2>/dev/null)

    if [ -n "$script_backup_path" ] && [ -f "$script_backup_path" ]; then
        print_info "Restoring potentially updated installer script from backup..."
        if cp "$script_backup_path" "$SCRIPT_ABS_PATH"; then
             print_success "Installer script restored."
        else
             print_warning "Failed to restore installer script from backup."
             print_warning "The version in $SCRIPT_ABS_PATH might be outdated."
        fi
        rm -f "$script_backup_path"
    fi

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

    print_info "Installing NPM dependencies in $PULSE_DIR (root project directory)..."
    if npm install --unsafe-perm --silent; then
        print_success "NPM dependencies installed successfully in $PULSE_DIR."
    else
        print_error "Failed to install NPM dependencies in $PULSE_DIR. See npm output above."
        exit 1
    fi

    print_info "Building CSS assets in $PULSE_DIR..."
    if npm run build:css --silent; then
        print_success "CSS assets built successfully."
    else
        print_error "Failed to build CSS assets. See npm output above."
        print_warning "Continuing update, but frontend may not display correctly."
    fi

    # Restore user data BEFORE setting permissions
    if [ -n "$user_data_backup" ] && [ -d "$user_data_backup" ]; then
        if restore_user_data "$user_data_backup"; then
            print_success "User configuration and data restored successfully"
        else
            print_warning "Failed to restore some user configuration"
        fi
        # Cleanup backup after restoration
        rm -rf "$user_data_backup"
    fi

    set_permissions

    print_info "Ensuring systemd service $SERVICE_NAME is configured..."
    if ! setup_systemd_service true; then
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

    if [ -n "$TARGET_TAG" ]; then
        print_info "Performing post-update version verification..."
        local installed_version=$(node -p "require('./package.json').version" 2>/dev/null || echo "unknown")
        local expected_version=$(echo "$TARGET_TAG" | sed 's/^v//')
        
        if [ "$installed_version" = "$expected_version" ]; then
            print_success "Version verification passed: $installed_version"
        elif [ "$installed_version" = "unknown" ]; then
            print_warning "Could not verify installed version from package.json"
        else
            print_warning "Version mismatch detected!"
            print_warning "Expected: $expected_version, Found: $installed_version"
            print_warning "The application may not report the correct version."
        fi
        
        if [ -n "$current_tag" ]; then
          print_success "Pulse updated successfully to version $current_tag!"
        else
          print_success "Pulse updated successfully! [Could not confirm exact tag]"
        fi
        
        print_info "The application should now report version: $expected_version"
    else
        # Branch update
        local current_branch=$(sudo -u "$PULSE_USER" git rev-parse --abbrev-ref HEAD 2>/dev/null)
        local current_commit=$(sudo -u "$PULSE_USER" git rev-parse --short HEAD 2>/dev/null)
        
        print_success "Pulse updated successfully to branch: $current_branch!"
        if [ -n "$current_commit" ]; then
            print_info "Current commit: $current_commit"
        fi
        print_warning "⚠️  Running from branch - version reporting may show development version"
    fi
    return 0
}


perform_remove() {
    print_warning "This will stop and disable the Pulse service[s] and remove the installation directory ($PULSE_DIR)."
    local remove_confirm=""
    if [ -t 0 ]; then
      read -p "Are you sure you want to remove Pulse? [y/N]: " remove_confirm
      if [[ ! "$remove_confirm" =~ ^[Yy]$ ]]; then
          print_info "Removal cancelled."
          return 1
      fi
    else
        print_warning "Running non-interactively. Proceeding with removal..."
    fi

    local potential_services=("pulse-monitor.service" "pulse-proxmox.service")
    local service_removed=false

    for service_name in "${potential_services[@]}"; do
        local service_file_path="/etc/systemd/system/$service_name"
        if systemctl list-units --full -all | grep -q "$service_name" 2>/dev/null; then
            print_info "Stopping service $service_name..."
            systemctl stop "$service_name" > /dev/null 2>&1

            print_info "Disabling service $service_name..."
            systemctl disable "$service_name" > /dev/null 2>&1

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

    if [ "$service_removed" = true ]; then
        print_info "Reloading systemd daemon..."
        systemctl daemon-reload
    fi

    print_info "Removing Pulse installation directory $PULSE_DIR..."
    if rm -rf "$PULSE_DIR"; then
        print_success "Installation directory removed."
    else
        print_error "Failed to remove installation directory $PULSE_DIR. Please remove it manually."
        return 1
    fi

    print_success "Pulse removed successfully."
    return 0
}

install_npm_deps() {
    print_info "Installing npm dependencies..."
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
        return 1
    fi

    cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; return 1; }

    print_info "Installing root dependencies [including dev]..."
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
        cd ..
        return 1
    fi

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
        print_warning "Check ownership and permissions for $PULSE_DIR."
        return 1
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

    if [ -f "$env_path" ]; then
        print_warning "Configuration file $env_path already exists."
        if [ -t 0 ]; then
            read -p "Overwrite existing configuration? [y/N]: " overwrite_confirm
            if [[ ! "$overwrite_confirm" =~ ^[Yy]$ ]]; then
                print_info "Skipping environment configuration."
                return 0
            fi
            print_info "Proceeding to overwrite existing configuration..."
        else
            print_info "Running non-interactively, skipping environment configuration as file exists."
            return 0
        fi
    fi

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

        if [[ ! "$proxmox_host" =~ ^https?:// ]]; then
            print_warning "URL does not start with http:// or https://. Prepending https://."
            proxmox_host="https://$proxmox_host"
            print_info "Using Proxmox Host URL: $proxmox_host"
        fi

        echo ""
        print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
        print_info "or run the following commands on your Proxmox host shell:"
        echo "----------------------------------------------------------------------"
        echo "  # 1. Create user 'pulse-monitor' [enter password when prompted]:"
        echo "  pveum useradd pulse-monitor@pam -comment "API user for Pulse monitoring""
        echo '  ' 
        echo "  # 2. Create API token 'pulse' for user [COPY THE SECRET VALUE!]:"
        echo "  pveum user token add pulse-monitor@pam pulse --privsep=1"
        echo '  ' 
        echo "  # 3. Assign PVEAuditor role to user:"
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

        read -p "Allow self-signed certificates for Proxmox? [Y/n]: " allow_self_signed
        read -p "Port for Pulse server [leave blank for default 7655]: " pulse_port
    else
        print_warning "Running non-interactively. Cannot prompt for environment details."
        print_warning "Ensure $env_path is configured manually or exists from a previous run."
        if [ -f "$env_path" ]; then return 0; fi
    fi

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

    print_info "Creating $env_path from example..."
    if cp "$env_example_path" "$env_path"; then
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
    local update_mode=${1:-false}

    print_info "Setting up systemd service $SERVICE_NAME..."
    local service_file="/etc/systemd/system/$SERVICE_NAME"

    local node_path
    node_path=$(command -v node)
    if [ -z "$node_path" ]; then
        print_error "Could not find Node.js executable path. Cannot create service."
        return 1
    fi
    local npm_path
    npm_path=$(command -v npm)
     if [ -z "$npm_path" ]; then
        print_warning "Could not find npm executable path. Service might require adjustment."
        local node_dir
        node_dir=$(dirname "$node_path")
        npm_path="$node_dir/npm"
         if ! command -v "$npm_path" &> /dev/null; then
             print_error "Could not reliably find npm executable path. Cannot create service."
             return 1
         fi
         print_info "Found npm at $npm_path"
    fi

    print_info "Creating/Updating service file at $service_file..."
    cat << EOF > "$service_file"
[Unit]
Description=Pulse for Proxmox VE Monitoring Application
After=network.target

[Service]
Type=simple
User=$PULSE_USER
Group=$PULSE_USER
WorkingDirectory=$PULSE_DIR

EnvironmentFile=$PULSE_DIR/.env

ExecStart=$node_path $npm_path run start

Restart=on-failure
RestartSec=5


StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    if [ $? -ne 0 ]; then
        print_error "Failed to create systemd service file $service_file."
        return 1
    fi

    chmod 644 "$service_file"

    print_info "Reloading systemd daemon..."
    systemctl daemon-reload

    if [ "$update_mode" = true ]; then
        print_info "[Update mode: Skipping service enable/start]"
        return 0
    fi

    print_info "Enabling $SERVICE_NAME to start on boot..."
    if systemctl enable "$SERVICE_NAME" > /dev/null 2>&1; then
        print_success "Service enabled successfully."
    else
        print_error "Failed to enable systemd service."
        return 1
    fi

    print_info "Starting $SERVICE_NAME..."
    if systemctl start "$SERVICE_NAME"; then
        print_success "Service started successfully."
    else
        print_error "Failed to start systemd service."
        print_warning "Please check the service status using: systemctl status $SERVICE_NAME"
        print_warning "And check the logs using: journalctl -u $SERVICE_NAME"
        return 1
    fi
    return 0
}



disable_cron_update() {
    print_info "Disabling Pulse automatic update cron job..."
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]"

    current_cron=$(crontab -l -u root 2>/dev/null || true)

    local identifier_for_grep_check
    identifier_for_grep_check=$(echo "$cron_identifier" | sed 's/[]$.*^[]/\\&/g')
    if ! echo "$current_cron" | grep -q "^${identifier_for_grep_check}$"; then
        print_info "Pulse automatic update cron job not found. Nothing to disable."
        return 0
    fi

    local escaped_identifier_for_awk
    escaped_identifier_for_awk=$(echo "$cron_identifier" | sed 's/[][\\.*^$(){}|+?]/\\\\&/g')
    local awk_script='/__PATTERN__/ { skip=1; next } skip == 1 { skip=0; next } { print }'
    awk_script="${awk_script/__PATTERN__/^$escaped_identifier_for_awk$}"

    filtered_cron=$(echo "$current_cron" | awk "$awk_script")

    trimmed_filtered_cron=$(echo "$filtered_cron" | awk 'NF')
    if [ -z "$trimmed_filtered_cron" ]; then
        echo "" | crontab -u root -
    else
        echo "$trimmed_filtered_cron" | crontab -u root -
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
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]"

    if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
         print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
         return 1
    fi

    print_info "Choose update frequency [will update to the *latest release tag*]:"
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

    cron_command="$cron_schedule /usr/bin/bash $script_path --update >> $LOG_FILE 2>&1"

    print_info "Checking/Updating cron job for Pulse automatic updates..."
    current_cron=$(crontab -l -u root 2>/dev/null || true)

    local escaped_identifier_for_awk
    escaped_identifier_for_awk=$(echo "$cron_identifier" | sed 's/[][\\.*^$(){}|+?]/\\\\&/g')
    local awk_script='/__PATTERN__/ { skip=1; next } skip == 1 { skip=0; next } { print }'
    awk_script="${awk_script/__PATTERN__/^$escaped_identifier_for_awk$}"

    filtered_cron=$(echo "$current_cron" | awk "$awk_script")

    local new_cron_content

    trimmed_filtered_cron=$(echo "$filtered_cron" | awk 'NF')
    if [ -n "$trimmed_filtered_cron" ]; then
        new_cron_content=$(printf "%s\n%s\n%s" "$trimmed_filtered_cron" "$cron_identifier" "$cron_command")
    else
        new_cron_content=$(printf "%s\n%s" "$cron_identifier" "$cron_command")
    fi

    echo "$new_cron_content" | crontab -u root -
    if [ $? -eq 0 ]; then
        print_success "Cron job for automatic updates set successfully."
        print_info "Update logs will be written to $LOG_FILE"
        touch "$LOG_FILE" || print_warning "Could not touch log file $LOG_FILE"
        chown root:root "$LOG_FILE" || print_warning "Could not chown log file $LOG_FILE"
        chmod 644 "$LOG_FILE" || print_warning "Could not chmod log file $LOG_FILE"
    else
        print_error "Failed to update cron job. Please check crontab configuration manually."
        return 1
    fi

    return 0
}

prompt_for_cron_setup() {
    if [ -n "$MODE_UPDATE" ]; then
        return 0
    fi

    local cron_exists=false
    local cron_identifier="# Pulse-Auto-Update [$SCRIPT_NAME]"
    local identifier_for_grep_check
    identifier_for_grep_check=$(echo "$cron_identifier" | sed 's/[]$.*^[]/\\&/g')

    if crontab -l -u root 2>/dev/null | grep -q "^${identifier_for_grep_check}$"; then
        cron_exists=true
    fi

    echo ""

    if [ "$cron_exists" = true ]; then
        print_info "Automatic updates for Pulse appear to be currently ENABLED."
        print_info "[Cron job will update to the latest release tag when run]"
        echo -e "Choose an action:"
        echo -e "  1) Keep current schedule [Do nothing]"
        echo -e "  2) Disable automatic updates"
        echo -e "  3) Change update schedule"
        read -p "Enter your choice [1-3]: " cron_manage_choice

        case $cron_manage_choice in
            1) print_info "Keeping current automatic update schedule.";;
            2) disable_cron_update ;;
            3) setup_cron_update ;;
            *) print_warning "Invalid choice. No changes made to automatic updates.";;
        esac
    else
        print_info "Automatic updates for Pulse appear to be currently DISABLED."
        print_info "[Cron job would update to the latest release tag when run]"
        read -p "Do you want to set up automatic updates for Pulse? [Y/n]: " setup_cron_confirm
        if [[ ! "$setup_cron_confirm" =~ ^[Nn]$ ]]; then
            setup_cron_update
        else
            print_info "Skipping automatic update setup."
        fi
    fi
}

final_instructions() {
    local ip_address
    ip_address=$(hostname -I | awk '{print $1}')
    local port_value
    local env_path="$PULSE_DIR/.env"
    if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
        port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
    else
        port_value="7655"
    fi

    local final_tag=""
    local final_branch=""
    local final_commit=""
    
    if [ -d "$PULSE_DIR/.git" ]; then
        cd "$PULSE_DIR" || print_warning "Could not cd to $PULSE_DIR to get version info."
        final_tag=$(get_current_local_tag)
        if [ -z "$final_tag" ]; then
            final_branch=$(sudo -u "$PULSE_USER" git rev-parse --abbrev-ref HEAD 2>/dev/null)
            final_commit=$(sudo -u "$PULSE_USER" git rev-parse --short HEAD 2>/dev/null)
        fi
        cd ..
    fi

    echo ""
    print_success "Pulse for Proxmox VE installation/update complete!"
    if [ -n "$final_tag" ]; then
        print_success "Current version installed: $final_tag"
    elif [ -n "$final_branch" ]; then
        print_success "Running from branch: $final_branch"
        if [ -n "$final_commit" ]; then
            print_info "Current commit: $final_commit"
        fi
        print_warning "⚠️  This is a test/development branch - not for production use!"
    fi
    echo "-------------------------------------------------------------"
    print_info "You should be able to access the Pulse dashboard at:"
    if [ -n "$ip_address" ]; then
        echo "  http://$ip_address:$port_value"
    else
        echo "  http://<YOUR-LXC-IP>:$port_value"
        print_warning "Could not automatically determine the LXC IP address."
    fi
    echo ""
    print_info "The Pulse service $SERVICE_NAME is running and enabled on boot."
    print_info "To check the status: sudo systemctl status $SERVICE_NAME"
    print_info "To view logs: sudo journalctl -u $SERVICE_NAME -f"
    echo "-------------------------------------------------------------"
}


check_root

if [ -z "$MODE_UPDATE" ] && [ "$INSTALLER_WAS_REEXECUTED" != "true" ]; then
    print_dependency_info
fi

self_update_check || print_warning "Installer self-check failed, proceeding anyway..."

check_installation_status_and_determine_action

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
        print_error "Exiting due to error."
        exit 1
        ;;
    "install" | "update")

        print_info "Proceeding with install/update. Installing prerequisites..."
        if [ "$INSTALL_MODE" = "install" ]; then
            run_apt_update || exit 1
            run_apt_upgrade_system || exit 1
        elif [ "$INSTALL_MODE" = "update" ]; then
            run_apt_update || exit 1
        fi
        install_dependencies || exit 1
        setup_node || exit 1
        create_pulse_user || exit 1
        print_success "Prerequisites installed."

        if [ "$INSTALL_MODE" = "install" ]; then
            print_info "Starting installation..."

            if [ -n "$SPECIFIED_VERSION_TAG" ]; then
                 if ! git ls-remote --tags --exit-code origin "refs/tags/$SPECIFIED_VERSION_TAG"; then
                     print_error "Specified version tag '$SPECIFIED_VERSION_TAG' not found on remote repository."
                     exit 1
                 else
                     print_info "Specified version tag '$SPECIFIED_VERSION_TAG' confirmed."
                     TARGET_TAG="$SPECIFIED_VERSION_TAG"
                 fi
            fi

            # For branch installations, use git clone
            if [ -n "$TARGET_BRANCH" ]; then
                print_info "Cloning Pulse repository into $PULSE_DIR for branch installation..."
                if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
                    print_success "Repository cloned successfully."
                    cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR after clone."; exit 1; }
                    chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR" || print_warning "Failed initial chown after clone."
                    
                    print_info "Checking out branch '$TARGET_BRANCH'..."
                    if ! sudo -u "$PULSE_USER" git checkout -B "$TARGET_BRANCH" "origin/$TARGET_BRANCH"; then
                         print_error "Failed to checkout branch '$TARGET_BRANCH' after cloning."
                         cd ..; exit 1
                    fi
                    print_success "Checked out branch $TARGET_BRANCH."
                    print_warning "⚠️  Branch installation is for testing only and may be unstable!"
                else
                    print_error "Failed to clone repository."
                    exit 1
                fi
            else
                # For version tag installations, try tarball first
                if [ -z "$TARGET_TAG" ]; then
                    # Need to determine latest tag - use git ls-remote for this
                    print_info "Determining latest release version..."
                    latest_tag=$(git ls-remote --tags --sort='-version:refname' https://github.com/rcourtman/Pulse.git | grep 'refs/tags/v' | head -n 1 | sed 's/.*refs\/tags\///' | sed 's/\^{}.*//')
                    if [ -z "$latest_tag" ]; then
                         print_error "Could not determine latest release tag to install."
                         exit 1
                    fi
                    TARGET_TAG="$latest_tag"
                    print_info "Determined latest version tag: $TARGET_TAG"
                fi
                
                # Try tarball installation first
                TARBALL_INSTALL_SUCCESS=false
                if perform_tarball_install; then
                    print_info "Installation completed using tarball."
                    TARBALL_INSTALL_SUCCESS=true
                else
                    # Fall back to git clone
                    print_info "Tarball installation failed, falling back to git clone..."
                    
                    print_info "Cloning Pulse repository into $PULSE_DIR..."
                    if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
                        print_success "Repository cloned successfully."
                        cd "$PULSE_DIR" || { print_error "Failed to cd into $PULSE_DIR after clone."; exit 1; }
                        chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR" || print_warning "Failed initial chown after clone."
                        
                        print_info "Checking out target version tag '$TARGET_TAG'..."
                        if ! sudo -u "$PULSE_USER" git checkout "$TARGET_TAG"; then
                             print_error "Failed to checkout tag '$TARGET_TAG' after cloning."
                             cd ..; exit 1
                        fi
                        print_success "Checked out version $TARGET_TAG."
                    else
                        print_error "Failed to clone repository."
                        exit 1
                    fi
                fi
            fi
            cd ..

            # Skip npm install and CSS build if tarball installation was successful
            # (tarball should already include these)
            if [ "$TARBALL_INSTALL_SUCCESS" != "true" ]; then
                print_info "Installing NPM dependencies in $PULSE_DIR..."
                cd "$PULSE_DIR" || { print_error "Failed to cd to $PULSE_DIR before npm install"; exit 1; }
                if ! npm install --unsafe-perm --silent; then
                    print_error "Failed to install NPM dependencies. See output above."
                    exit 1
                else
                    print_success "NPM dependencies installed."
                fi
                
                print_info "Building CSS assets in $PULSE_DIR..."
                if ! npm run build:css --silent; then
                    print_error "Failed to build CSS assets. See output above."
                    exit 1
                else
                    print_success "CSS assets built."
                fi
            else
                # Ensure we're in the right directory for subsequent operations
                cd "$PULSE_DIR" || { print_error "Failed to cd to $PULSE_DIR"; exit 1; }
                print_info "Skipping npm install and CSS build (already included in tarball)."
            fi

            set_permissions || exit 1
            configure_environment || exit 1
            setup_systemd_service || exit 1
            final_instructions
            prompt_for_cron_setup

        else
            print_info "Starting update to version $TARGET_TAG..."
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