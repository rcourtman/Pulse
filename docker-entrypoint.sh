#!/bin/sh
set -e

# Default UID/GID if not provided
PUID=${PUID:-1000}
PGID=${PGID:-1000}
PULSE_IMMUTABLE_OWNERSHIP_PATHS=${PULSE_IMMUTABLE_OWNERSHIP_PATHS:-}

is_immutable_ownership_path() {
    target="$1"
    [ -n "$target" ] || return 1

    case ":$PULSE_IMMUTABLE_OWNERSHIP_PATHS:" in
        *:"$target":*)
            return 0
            ;;
    esac

    return 1
}

chown_tree_skipping_immutable_paths() {
    owner="$1"
    root="$2"

    if [ ! -e "$root" ]; then
        return 0
    fi

    if [ -z "$PULSE_IMMUTABLE_OWNERSHIP_PATHS" ]; then
        chown -R "$owner" "$root"
        return 0
    fi

    find "$root" -mindepth 1 | while IFS= read -r path; do
        if is_immutable_ownership_path "$path"; then
            echo "Skipping immutable ownership path: $path"
            continue
        fi
        chown "$owner" "$path"
    done

    chown "$owner" "$root"
}

# Only adjust permissions if running as root
if [ "$(id -u)" = "0" ]; then
    echo "Starting with UID: $PUID, GID: $PGID"
    
    # If PUID is 0 (root), don't create a new user, just run as root
    if [ "$PUID" = "0" ]; then
        echo "Running as root user"
        # Fix ownership to root
        chown -R root:root /data /app /opt/pulse
        chown_tree_skipping_immutable_paths root:root /etc/pulse
        exec "$@"
    fi
    
    # Check if we need to modify the user/group
    current_uid=$(id -u pulse 2>/dev/null || echo "")
    current_gid=$(getent group pulse 2>/dev/null | cut -d: -f3 || echo "")
    
    # If user/group don't match, recreate them
    if [ "$current_uid" != "$PUID" ] || [ "$current_gid" != "$PGID" ]; then
        # Remove existing user and group
        deluser pulse 2>/dev/null || true
        delgroup pulse 2>/dev/null || true
        
        # Create new group and user with desired IDs
        addgroup -g "$PGID" pulse
        adduser -D -u "$PUID" -G pulse pulse
    fi
    
    # Fix ownership of data directory
    chown -R pulse:pulse /data /app /opt/pulse
    chown_tree_skipping_immutable_paths pulse:pulse /etc/pulse
    
    # Switch to pulse user
    exec su-exec pulse "$@"
else
    # Not running as root, just exec the command
    exec "$@"
fi
