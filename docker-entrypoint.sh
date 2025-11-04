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
        chown -R root:root /data /app /etc/pulse /opt/pulse
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
    chown -R pulse:pulse /data /app /etc/pulse /opt/pulse
    
    # Switch to pulse user
    exec su-exec pulse "$@"
else
    # Not running as root, just exec the command
    exec "$@"
fi
