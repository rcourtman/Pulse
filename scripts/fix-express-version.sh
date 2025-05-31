#!/bin/bash

# Emergency fix script for Express version issue
# This script forces a clean reinstall of dependencies with the correct Express version

set -e

print() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

print "Emergency Express version fix for Pulse"
print "======================================"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    print "ERROR: This script must be run as root (use sudo)"
    exit 1
fi

# Stop the service
print "Stopping pulse-monitor service..."
systemctl stop pulse-monitor.service 2>/dev/null || true

# Navigate to Pulse directory
cd /opt/pulse || {
    print "ERROR: Could not find /opt/pulse directory"
    exit 1
}

# Backup current package.json if needed
if [ -f package.json ]; then
    cp package.json package.json.backup
    print "Backed up package.json"
fi

# Remove node_modules completely
print "Removing old node_modules directory..."
rm -rf node_modules

# Clear npm cache
print "Clearing npm cache..."
npm cache clean --force

# Ensure package.json has correct Express version
print "Checking Express version in package.json..."
if grep -q '"express": "\^4\.21' package.json; then
    print "Fixing Express version in package.json..."
    sed -i 's/"express": "\^4\.21\.[0-9]*"/"express": "4.19.2"/' package.json
fi

# Show current Express version in package.json
current_version=$(grep '"express":' package.json | sed 's/.*"express": "\([^"]*\)".*/\1/')
print "Express version in package.json: $current_version"

# Install dependencies using npm ci
print "Installing dependencies with npm ci..."
if npm ci --omit=dev; then
    print "SUCCESS: Dependencies installed with npm ci"
else
    print "WARNING: npm ci failed, trying npm install..."
    npm install --omit=dev || {
        print "ERROR: Failed to install dependencies"
        exit 1
    }
fi

# Verify Express version
installed_version=$(npm list express --depth=0 2>/dev/null | grep express@ | sed 's/.*express@//')
print "Express version installed: $installed_version"

if [[ "$installed_version" == "4.19.2" ]]; then
    print "SUCCESS: Correct Express version installed"
else
    print "WARNING: Express version mismatch. Expected 4.19.2, got $installed_version"
fi

# Set correct ownership
print "Setting correct ownership..."
chown -R pulse:pulse /opt/pulse

# Start the service
print "Starting pulse-monitor service..."
systemctl start pulse-monitor.service

# Wait a moment
sleep 3

# Check service status
if systemctl is-active --quiet pulse-monitor.service; then
    print "SUCCESS: pulse-monitor service is running!"
    systemctl status pulse-monitor.service --no-pager
else
    print "ERROR: Service failed to start. Checking logs..."
    journalctl -u pulse-monitor.service -n 20 --no-pager
fi

print "======================================"
print "Fix script completed"