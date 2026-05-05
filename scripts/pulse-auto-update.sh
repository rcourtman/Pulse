#!/bin/bash

# Pulse Automatic Update Script
# This script checks for and installs stable Pulse updates
# It is designed to be run by systemd timer

set -euo pipefail

# Configuration
GITHUB_REPO="${GITHUB_REPO:-rcourtman/Pulse}"
SERVICE_NAME="${PULSE_SERVICE_NAME:-pulse}"
INSTALL_DIR="${PULSE_INSTALL_DIR:-/opt/pulse}"
CONFIG_DIR="${PULSE_CONFIG_DIR:-/etc/pulse}"
UPDATE_TIMER_UNIT="${PULSE_UPDATE_TIMER_UNIT:-${SERVICE_NAME}-update.timer}"
LOG_TAG="${PULSE_AUTO_UPDATE_LOG_TAG:-${SERVICE_NAME}-auto-update}"
MAX_LOG_SIZE=10485760  # 10MB
INSTALL_SIGNATURE_IDENTITY="pulse-installer"
INSTALL_SIGNATURE_NAMESPACE="pulse-install"
PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"

# Logging function
log() {
    local level=$1
    shift
    logger -t "$LOG_TAG" -p "user.$level" "$@"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $@"
}

release_signature_key_available() {
    [[ -n "${PINNED_RELEASE_SSH_PUBLIC_KEY:-}" ]]
}

require_release_signature_verifier() {
    if ! release_signature_key_available; then
        log error "Pinned release signature key is not configured"
        return 1
    fi
    if ! command -v ssh-keygen >/dev/null 2>&1; then
        log error "ssh-keygen is required to verify signed Pulse release assets"
        return 1
    fi
}

verify_release_signature() {
    local target_path="$1"
    local signature_path="$2"
    local context="${3:-downloaded file}"
    local allowed_signers=""

    if ! require_release_signature_verifier; then
        return 1
    fi

    allowed_signers=$(mktemp /tmp/pulse-release-signers.XXXXXX)
    printf '%s %s\n' "$INSTALL_SIGNATURE_IDENTITY" "$PINNED_RELEASE_SSH_PUBLIC_KEY" > "$allowed_signers"

    if ! ssh-keygen -Y verify \
        -f "$allowed_signers" \
        -I "$INSTALL_SIGNATURE_IDENTITY" \
        -n "$INSTALL_SIGNATURE_NAMESPACE" \
        -s "$signature_path" < "$target_path" >/dev/null 2>&1; then
        rm -f "$allowed_signers"
        log error "Cryptographic signature verification failed for ${context}"
        return 1
    fi

    rm -f "$allowed_signers"
    return 0
}

# Check if auto-updates are enabled
check_auto_updates_enabled() {
    # Check system.json for autoUpdateEnabled flag (note: no 's' - matches Go struct)
    if [[ -f "$CONFIG_DIR/system.json" ]]; then
        local enabled=$(cat "$CONFIG_DIR/system.json" 2>/dev/null | grep -o '"autoUpdateEnabled"[[:space:]]*:[[:space:]]*true' || true)
        local channel=$(cat "$CONFIG_DIR/system.json" 2>/dev/null | grep -o '"updateChannel"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/' || true)
        if [[ -z "$enabled" ]]; then
            log info "Auto-updates disabled in configuration"
            exit 0
        fi
        if [[ "$channel" == "rc" ]]; then
            log info "Prerelease channel detected; unattended auto-updates run only on stable"
            exit 0
        fi
    fi
    
    # Also check if timer is enabled (belt and suspenders)
    if ! systemctl is-enabled --quiet "$UPDATE_TIMER_UNIT" 2>/dev/null; then
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

# Determine whether a tag is a semver pre-release.
#
# Per semver 2.0.0, any identifier after the patch version introduced by a
# hyphen (e.g. `v6.0.0-rc.2`, `v5.1.28-beta.1`, `v5.1.28-nightly`) is a
# pre-release identifier. The unattended updater must refuse these on the
# stable channel regardless of what GitHub's `/releases/latest` endpoint
# returns, because that endpoint has, in the wild, briefly surfaced tags
# that were later corrected to prerelease=true (see the 2026-04-16 incident
# that bumped demo.pulserelay.pro from 5.1.27 to 6.0.0-rc.2).
#
# Returns 0 if the tag looks like a prerelease, 1 otherwise. Empty or
# obviously-malformed input is treated as prerelease (fail-closed).
is_prerelease_tag() {
    local tag="${1:-}"
    if [[ -z "$tag" ]]; then
        return 0
    fi
    # Must look like semver: v?MAJOR.MINOR.PATCH with optional suffix.
    if [[ ! "$tag" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
        return 0
    fi
    # Any hyphen after the patch component is a prerelease identifier.
    if [[ "$tag" == *-* ]]; then
        return 0
    fi
    return 1
}

# Get latest stable release from GitHub
get_latest_stable_version() {
    local latest_version=""
    local release_json=""
    local is_prerelease_flag=""

    # Get latest stable release (not pre-releases). `/releases/latest`
    # already skips prereleases on GitHub's side, but we still parse and
    # enforce the `prerelease` flag ourselves as a second line of defense.
    release_json=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" || true)

    if [[ -n "$release_json" ]] && [[ "$release_json" != *"rate limit"* ]]; then
        latest_version=$(echo "$release_json" | \
            grep '"tag_name":' | \
            head -1 | \
            sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/' || true)
        is_prerelease_flag=$(echo "$release_json" | \
            grep '"prerelease":' | \
            head -1 | \
            sed -E 's/.*"prerelease":[[:space:]]*(true|false).*/\1/' || true)

        # Refuse if the API explicitly flags this release as a prerelease.
        if [[ "$is_prerelease_flag" == "true" ]]; then
            log error "GitHub /releases/latest returned a prerelease ($latest_version); refusing on stable channel"
            echo ""
            return 0
        fi
    fi

    # Check if we got rate limited or failed
    if [[ -z "$latest_version" ]] || [[ "$latest_version" == *"rate limit"* ]]; then
        # Try direct GitHub latest URL as fallback
        latest_version=$(curl -sI "https://github.com/$GITHUB_REPO/releases/latest" | \
            grep -i '^location:' | \
            sed -E 's|.*tag/([^[:space:]]+).*|\1|' | \
            tr -d '\r' || true)
    fi

    # Final belt-and-braces: never hand back a prerelease-shaped tag even
    # if an upstream path told us it was stable. The channel-pinning policy
    # lives in the Go server (EffectiveAutoUpdateEnabled gates this timer on
    # stable only); refusing prerelease tag shapes here ensures the unattended
    # script cannot cross the major-version boundary on a corrupted API reply.
    if [[ -n "$latest_version" ]] && is_prerelease_tag "$latest_version"; then
        log error "GitHub returned prerelease-shaped tag ($latest_version) as latest; refusing on stable channel"
        echo ""
        return 0
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
    if [[ -n "${PULSE_SERVICE_NAME:-}" ]]; then
        echo "$SERVICE_NAME"
    elif systemctl list-unit-files --no-legend | grep -q "^pulse-backend.service"; then
        echo "pulse-backend"
    elif systemctl list-unit-files --no-legend | grep -q "^pulse.service"; then
        echo "pulse"
    else
        echo "pulse"  # Default
    fi
}

resolve_install_script_url() {
    local target_version=$1
    printf 'https://github.com/%s/releases/download/%s/install.sh\n' "$GITHUB_REPO" "$target_version"
}

# Perform the update
perform_update() {
    local new_version=$1
    local service_name=$(detect_service_name)
    local installer_tmp=""
    local signature_tmp=""

    # Refuse to install a prerelease via the unattended updater. The stable
    # channel must never cross onto a tag like v6.0.0-rc.2, even if every
    # caller above this point thought it was safe.
    if is_prerelease_tag "$new_version"; then
        log error "Refusing to install prerelease version $new_version via unattended updater"
        return 1
    fi

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
    local install_script_url
    install_script_url=$(resolve_install_script_url "$new_version")
    local install_signature_url="${install_script_url}.sshsig"
    
    # Run install script with specific version
    local marker_file="$INSTALL_DIR/BUILD_FROM_SOURCE"
    local -a installer_args=(--version "$new_version")
    if [[ -f "$marker_file" ]]; then
        local branch
        branch=$(tr -d '\r\n' <"$marker_file" 2>/dev/null || true)
        if [[ -n "$branch" ]]; then
            installer_args=(--source "$branch" "${installer_args[@]}")
        fi
    fi

    installer_tmp=$(mktemp /tmp/pulse-update-installer.XXXXXX)
    signature_tmp=$(mktemp /tmp/pulse-update-installer.sig.XXXXXX)
    trap 'rm -f "$installer_tmp" "$signature_tmp"' RETURN

    if ! curl -fsSL "$install_script_url" -o "$installer_tmp"; then
        log error "Failed to download installer from $install_script_url"
        return 1
    fi
    if ! curl -fsSL "$install_signature_url" -o "$signature_tmp"; then
        log error "Failed to download installer signature from $install_signature_url"
        return 1
    fi
    if ! verify_release_signature "$installer_tmp" "$signature_tmp" "downloaded Pulse installer"; then
        return 1
    fi
    log info "Installer signature verified"

    if env \
           "PULSE_SERVICE_NAME=$service_name" \
           "PULSE_INSTALL_DIR=$INSTALL_DIR" \
           "PULSE_CONFIG_DIR=$CONFIG_DIR" \
           bash "$installer_tmp" "${installer_args[@]}" 2>&1 | \
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
