#!/bin/bash
# Script to change password in Pulse systemd configuration
# This needs to be run with sudo

OVERRIDE_FILE="/etc/systemd/system/pulse-backend.service.d/override.conf"
NEW_PASSWORD="$1"

if [ -z "$NEW_PASSWORD" ]; then
    echo "Usage: $0 <new_password_hash>"
    exit 1
fi

if [ ! -f "$OVERRIDE_FILE" ]; then
    echo "No override file found"
    exit 1
fi

# Create a backup
cp "$OVERRIDE_FILE" "$OVERRIDE_FILE.bak"

# Replace the password line
sed -i "s|Environment=\"PULSE_AUTH_PASS=.*\"|Environment=\"PULSE_AUTH_PASS=$NEW_PASSWORD\"|" "$OVERRIDE_FILE"

# Reload systemd configuration
systemctl daemon-reload

echo "Password changed successfully"