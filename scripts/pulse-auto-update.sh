#!/bin/bash

# Pulse Automatic Update Script
# This script checks for and installs stable Pulse updates
# It is designed to be run by systemd timer

set -euo pipefail

# Configuration
GITHUB_REPO="rcourtman/Pulse"
INSTALL_DIR="/opt/pulse"
CONFIG_DIR="/etc/pulse"
LOG_TAG="pulse-auto-update"
MAX_LOG_SIZE=10485760  # 10MB

# Logging function
log() {
    local level=$1
    shift
    logger -t "$LOG_TAG" -p "user.$level" "$@"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $@"
}

# Check if auto-updates are enabled
check_auto_updates_enabled() {
    # Check system.json for autoUpdateEnabled flag (note: no 's' - matches Go struct)
    if [[ -f "$CONFIG_DIR/system.json" ]]; then
        local enabled=$(cat "$CONFIG_DIR/system.json" 2>/dev/null | grep -o '"autoUpdateEnabled"[[:space:]]*:[[:space:]]*true' || true)
        if [[ -z "$enabled" ]]; then
            log info "Auto-updates disabled in configuration"
            exit 0
        fi
    fi
    
    # Also check if timer is enabled (belt and suspenders)
    if ! systemctl is-enabled --quiet pulse-update.timer 2>/dev/null; then
        log info "Auto-update timer is disabled"
        exit 0
    fi
}

# Get current version
get_current_version() {
    local version=""
    
    # Try to get version from binary
    if [[ -f "$INSTALL_DIR/bin/pulse" ]]; then
        version=$("$INSTALL_DIR/bin/pulse" --version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.]+)?' | head -1 || true)
    elif [[ -f "$INSTALL_DIR/pulse" ]]; then
        version=$("$INSTALL_DIR/pulse" --version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.]+)?' | head -1 || true)
    fi
    
    # Fallback to VERSION file
    if [[ -z "$version" ]] && [[ -f "$INSTALL_DIR/VERSION" ]]; then
        version=$(cat "$INSTALL_DIR/VERSION" 2>/dev/null | tr -d '\n' || true)
    fi
    
    echo "${version:-unknown}"
}

# Get latest stable release from GitHub
get_latest_stable_version() {
    local latest_version=""
    
    # Get latest stable release (not pre-releases)
    latest_version=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/' || true)
    
    # Check if we got rate limited or failed
    if [[ -z "$latest_version" ]] || [[ "$latest_version" == *"rate limit"* ]]; then
        # Try direct GitHub latest URL as fallback
        latest_version=$(curl -sI "https://github.com/$GITHUB_REPO/releases/latest" | \
            grep -i '^location:' | \
            sed -E 's|.*tag/([^[:space:]]+).*|\1|' | \
            tr -d '\r' || true)
    fi
    
    echo "${latest_version:-}"
}

# Compare versions (returns 0 if v1 > v2, 1 if v1 <= v2)
version_greater_than() {
    local v1="${1#v}"  # Remove 'v' prefix
    local v2="${2#v}"
    
    # Don't update if current version is unknown
    if [[ "$2" == "unknown" ]]; then
        return 1
    fi
    
    # Split versions into parts
    IFS='.' read -ra V1_PARTS <<< "${v1%%-*}"  # Remove pre-release suffix
    IFS='.' read -ra V2_PARTS <<< "${v2%%-*}"
    
    # Compare major.minor.patch
    for i in 0 1 2; do
        local p1="${V1_PARTS[$i]:-0}"
        local p2="${V2_PARTS[$i]:-0}"
        if [[ "$p1" -gt "$p2" ]]; then
            return 0
        elif [[ "$p1" -lt "$p2" ]]; then
            return 1
        fi
    done
    
    # Versions are equal in major.minor.patch
    # Now check pre-release suffixes
    local has_suffix1=false
    local has_suffix2=false
    
    [[ "$v1" == *-* ]] && has_suffix1=true
    [[ "$v2" == *-* ]] && has_suffix2=true
    
    # Stable (no suffix) > pre-release (has suffix)
    if [[ "$has_suffix1" == "false" ]] && [[ "$has_suffix2" == "true" ]]; then
        return 0  # v1 (stable) > v2 (pre-release)
    elif [[ "$has_suffix1" == "true" ]] && [[ "$has_suffix2" == "false" ]]; then
        return 1  # v1 (pre-release) < v2 (stable)
    elif [[ "$has_suffix1" == "true" ]] && [[ "$has_suffix2" == "true" ]]; then
        # Both are pre-releases, compare suffixes lexicographically
        local suffix1="${v1#*-}"
        local suffix2="${v2#*-}"
        if [[ "$suffix1" > "$suffix2" ]]; then
            return 0
        fi
    fi
    
    return 1  # v1 <= v2
}

# Detect service name (could be pulse or pulse-backend)
detect_service_name() {
    if systemctl list-unit-files --no-legend | grep -q "^pulse-backend.service"; then
        echo "pulse-backend"
    elif systemctl list-unit-files --no-legend | grep -q "^pulse.service"; then
        echo "pulse"
    else
        echo "pulse"  # Default
    fi
}

# Perform the update
perform_update() {
    local new_version=$1
    local service_name=$(detect_service_name)
    
    log info "Starting update to $new_version"
    
    # Create backup of current installation
    local backup_dir="/tmp/pulse-backup-$(date +%Y%m%d-%H%M%S)"
    log info "Creating backup in $backup_dir"
    mkdir -p "$backup_dir"
    
    # Backup binary
    if [[ -f "$INSTALL_DIR/bin/pulse" ]]; then
        cp -a "$INSTALL_DIR/bin/pulse" "$backup_dir/" || true
    elif [[ -f "$INSTALL_DIR/pulse" ]]; then
        cp -a "$INSTALL_DIR/pulse" "$backup_dir/" || true
    fi
    
    # Backup VERSION file
    if [[ -f "$INSTALL_DIR/VERSION" ]]; then
        cp -a "$INSTALL_DIR/VERSION" "$backup_dir/" || true
    fi
    
    # Download update using install script (safest method)
    log info "Downloading and installing update"
    
    # Run install script with specific version
    if curl -sSL "https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh" | \
       bash -s -- --version "$new_version" 2>&1 | \
       while IFS= read -r line; do
           log info "installer: $line"
       done; then
        
        log info "Update successfully installed"
        
        # Verify new version
        local installed_version=$(get_current_version)
        if [[ "$installed_version" == "$new_version" ]]; then
            log info "Version verified: $installed_version"
            
            # Clean up backup
            rm -rf "$backup_dir"
            
            return 0
        else
            log error "Version mismatch after update. Expected: $new_version, Got: $installed_version"
            
            # Restore from backup
            log info "Restoring from backup"
            if [[ -f "$backup_dir/pulse" ]]; then
                if [[ -f "$INSTALL_DIR/bin/pulse" ]]; then
                    cp -f "$backup_dir/pulse" "$INSTALL_DIR/bin/pulse"
                else
                    cp -f "$backup_dir/pulse" "$INSTALL_DIR/pulse"
                fi
            fi
            if [[ -f "$backup_dir/VERSION" ]]; then
                cp -f "$backup_dir/VERSION" "$INSTALL_DIR/VERSION"
            fi
            
            # Restart service with old version
            systemctl restart "$service_name" || true
            
            # Clean up backup
            rm -rf "$backup_dir"
            
            return 1
        fi
    else
        log error "Update installation failed"
        
        # Restore from backup
        log info "Restoring from backup"
        if [[ -f "$backup_dir/pulse" ]]; then
            if [[ -f "$INSTALL_DIR/bin/pulse" ]]; then
                cp -f "$backup_dir/pulse" "$INSTALL_DIR/bin/pulse"
            else
                cp -f "$backup_dir/pulse" "$INSTALL_DIR/pulse"
            fi
        fi
        if [[ -f "$backup_dir/VERSION" ]]; then
            cp -f "$backup_dir/VERSION" "$INSTALL_DIR/VERSION"
        fi
        
        # Clean up backup
        rm -rf "$backup_dir"
        
        return 1
    fi
}

# Main update check
main() {
    log info "Starting Pulse auto-update check"
    
    # Check if auto-updates are enabled
    check_auto_updates_enabled
    
    # Check if we're in Docker (updates not supported)
    if [[ -f /.dockerenv ]] || grep -q docker /proc/1/cgroup 2>/dev/null; then
        log info "Docker environment detected, skipping auto-update"
        exit 0
    fi
    
    # Get current version
    local current_version=$(get_current_version)
    log info "Current version: $current_version"
    
    if [[ "$current_version" == "unknown" ]]; then
        log error "Could not determine current version, skipping update"
        exit 1
    fi
    
    # Get latest stable version
    local latest_version=$(get_latest_stable_version)
    
    if [[ -z "$latest_version" ]]; then
        log error "Could not determine latest version from GitHub"
        exit 1
    fi
    
    log info "Latest stable version: $latest_version"
    
    # Compare versions
    if version_greater_than "$latest_version" "$current_version"; then
        log info "New version available: $latest_version (current: $current_version)"
        
        # Perform update
        if perform_update "$latest_version"; then
            log info "Update completed successfully to $latest_version"
            
            # Send notification if webhooks are configured
            if [[ -f "$CONFIG_DIR/webhooks.enc" ]] || [[ -f "$CONFIG_DIR/webhooks.json" ]]; then
                # Create a simple notification via the Pulse API
                curl -s -X POST "http://localhost:7655/api/internal/notification" \
                    -H "Content-Type: application/json" \
                    -d "{\"type\":\"update\",\"message\":\"Pulse automatically updated from $current_version to $latest_version\"}" \
                    2>/dev/null || true
            fi
        else
            log error "Update failed"
            exit 1
        fi
    else
        log info "Already running latest version"
    fi
    
    log info "Auto-update check completed"
}

# Run main function
main "$@"