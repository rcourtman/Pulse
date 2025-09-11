#!/usr/bin/env bash

# Script to build and install Pulse from main branch
# This allows testing latest changes without creating a release

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   print_error "This script must be run as root"
   exit 1
fi

print_info "Building Pulse from main branch..."

# Install dependencies
print_info "Installing build dependencies..."
apt-get update >/dev/null 2>&1
apt-get install -y git golang-go make nodejs npm >/dev/null 2>&1 || {
    print_error "Failed to install dependencies"
    exit 1
}

# Create temp directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

print_info "Cloning repository..."
git clone --depth 1 https://github.com/rcourtman/Pulse.git >/dev/null 2>&1 || {
    print_error "Failed to clone repository"
    exit 1
}

cd Pulse

print_info "Building frontend..."
cd frontend-modern
npm ci >/dev/null 2>&1 || {
    print_error "Failed to install frontend dependencies"
    exit 1
}
npm run build >/dev/null 2>&1 || {
    print_error "Failed to build frontend"
    exit 1
}
cd ..

print_info "Building backend..."
make build >/dev/null 2>&1 || {
    print_error "Failed to build backend"
    exit 1
}

# Detect service name
SERVICE_NAME="pulse"
if systemctl list-unit-files --no-legend | grep -q "^pulse-backend.service"; then
    SERVICE_NAME="pulse-backend"
fi

# Stop service if running
if systemctl is-active --quiet "$SERVICE_NAME"; then
    print_info "Stopping $SERVICE_NAME service..."
    systemctl stop "$SERVICE_NAME"
fi

# Backup existing binary
if [[ -f /opt/pulse/bin/pulse ]]; then
    print_info "Backing up existing binary..."
    cp /opt/pulse/bin/pulse /opt/pulse/bin/pulse.backup
fi

# Install new binary
print_info "Installing new binary..."
mkdir -p /opt/pulse/bin
cp bin/pulse /opt/pulse/bin/pulse
chmod +x /opt/pulse/bin/pulse

# Update VERSION file to show it's from main
echo "main-$(git rev-parse --short HEAD)" > /opt/pulse/VERSION

# Start service
print_info "Starting $SERVICE_NAME service..."
systemctl start "$SERVICE_NAME" || {
    print_warning "Failed to start service automatically. You may need to start it manually."
}

# Cleanup
cd /
rm -rf "$TEMP_DIR"

print_success "Successfully installed Pulse from main branch!"
print_info "Version: main-$(cd /opt/pulse && git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
print_info "Access Pulse at: http://$(hostname -I | awk '{print $1}'):7655"