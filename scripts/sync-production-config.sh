#!/bin/bash
# Sync production config to dev environment
# This ensures dev mode has access to production nodes when mock is disabled

set -euo pipefail

PROD_DIR="/etc/pulse"
DEV_DIR="/opt/pulse/tmp/dev-config"

# Ensure dev config directory exists
mkdir -p "$DEV_DIR"
chmod 700 "$DEV_DIR"

# Copy essential production config files to dev
# Skip session/csrf/alert files which are runtime-specific
echo "Syncing production config to dev environment..."

# Copy encryption key if it exists
if [ -f "$PROD_DIR/.encryption.key" ]; then
    cp -f "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"
    chmod 600 "$DEV_DIR/.encryption.key"
    echo "✓ Synced encryption key"
fi

# Copy nodes configuration
if [ -f "$PROD_DIR/nodes.enc" ]; then
    cp -f "$PROD_DIR/nodes.enc" "$DEV_DIR/nodes.enc"
    chmod 600 "$DEV_DIR/nodes.enc"
    echo "✓ Synced nodes configuration"
elif [ -f "$PROD_DIR/nodes.json" ]; then
    cp -f "$PROD_DIR/nodes.json" "$DEV_DIR/nodes.json"
    chmod 600 "$DEV_DIR/nodes.json"
    echo "✓ Synced nodes configuration (unencrypted)"
fi

# Copy system settings (but keep dev-specific log level)
if [ -f "$PROD_DIR/system.json" ]; then
    cp -f "$PROD_DIR/system.json" "$DEV_DIR/system.json"
    echo "✓ Synced system settings"
fi

# Copy guest metadata if it exists
if [ -f "$PROD_DIR/guest_metadata.json" ]; then
    cp -f "$PROD_DIR/guest_metadata.json" "$DEV_DIR/guest_metadata.json"
    echo "✓ Synced guest metadata"
fi

# Copy email config if it exists
if [ -f "$PROD_DIR/email.enc" ]; then
    cp -f "$PROD_DIR/email.enc" "$DEV_DIR/email.enc"
    chmod 600 "$DEV_DIR/email.enc"
    echo "✓ Synced email configuration"
fi

# Copy webhook config if it exists
if [ -f "$PROD_DIR/webhooks.enc" ]; then
    cp -f "$PROD_DIR/webhooks.enc" "$DEV_DIR/webhooks.enc"
    chmod 600 "$DEV_DIR/webhooks.enc"
    echo "✓ Synced webhook configuration"
fi

# Initialize empty runtime files if they don't exist
touch "$DEV_DIR/sessions.json" "$DEV_DIR/csrf_tokens.json" 2>/dev/null || true
echo "[]" > "$DEV_DIR/sessions.json" 2>/dev/null || true
echo "[]" > "$DEV_DIR/csrf_tokens.json" 2>/dev/null || true
chmod 600 "$DEV_DIR/sessions.json" "$DEV_DIR/csrf_tokens.json" 2>/dev/null || true

# Create alerts directory if it doesn't exist
mkdir -p "$DEV_DIR/alerts" 2>/dev/null || true

echo ""
echo "✓ Production config synced to dev environment"
echo "  Source: $PROD_DIR"
echo "  Target: $DEV_DIR"