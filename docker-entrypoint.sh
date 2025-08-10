#!/bin/sh
set -e

# Default UID/GID if not provided
PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Only adjust permissions if running as root
if [ "$(id -u)" = "0" ]; then
    echo "Starting with UID: $PUID, GID: $PGID"
    
    # If PUID is 0 (root), don't create a new user, just run as root
    if [ "$PUID" = "0" ]; then
        echo "Running as root user"
        # Fix ownership to root
        chown -R root:root /data /app /etc/pulse
        exec "$@"
    fi
    
    # Create group if it doesn't exist
    if ! getent group pulse >/dev/null 2>&1; then
        addgroup -g "$PGID" pulse
    else
        # Modify existing group GID if different
        current_gid=$(getent group pulse | cut -d: -f3)
        if [ "$current_gid" != "$PGID" ]; then
            delgroup pulse 2>/dev/null || true
            addgroup -g "$PGID" pulse
        fi
    fi
    
    # Create user if it doesn't exist
    if ! id -u pulse >/dev/null 2>&1; then
        adduser -D -u "$PUID" -G pulse pulse
    else
        # Modify existing user UID if different
        current_uid=$(id -u pulse)
        if [ "$current_uid" != "$PUID" ]; then
            deluser pulse 2>/dev/null || true
            adduser -D -u "$PUID" -G pulse pulse
        fi
    fi
    
    # Fix ownership of data directory
    chown -R pulse:pulse /data /app /etc/pulse
    
    # Switch to pulse user
    exec su-exec pulse "$@"
else
    # Not running as root, just exec the command
    exec "$@"
fi