#!/bin/bash
# Script to remove authentication from Pulse systemd configuration
# This needs to be run with sudo

OVERRIDE_FILE="/etc/systemd/system/pulse-backend.service.d/override.conf"

if [ ! -f "$OVERRIDE_FILE" ]; then
    echo "No override file found, authentication already removed"
    exit 0
fi

# Remove all authentication-related environment variables from the override file
if grep -q "PULSE_AUTH_USER\|PULSE_AUTH_PASS\|PULSE_PASSWORD\|API_TOKEN" "$OVERRIDE_FILE"; then
    # Create a backup
    cp "$OVERRIDE_FILE" "$OVERRIDE_FILE.bak"
    
    # Remove the authentication lines but keep other settings
    grep -v "PULSE_AUTH_USER\|PULSE_AUTH_PASS\|PULSE_PASSWORD\|API_TOKEN" "$OVERRIDE_FILE" > "$OVERRIDE_FILE.tmp"
    mv "$OVERRIDE_FILE.tmp" "$OVERRIDE_FILE"
    
    # Reload systemd and restart the service
    systemctl daemon-reload
    systemctl restart pulse-backend
    
    echo "Authentication removed successfully"
else
    echo "No authentication configuration found in override file"
fi