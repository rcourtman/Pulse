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
echo "  Source: $PROD_DIR"
echo "  Target: $DEV_DIR"
echo ""

# CRITICAL: Always sync production encryption key to dev
# This ensures dev can decrypt production's encrypted config files
if [ -f "$PROD_DIR/.encryption.key" ]; then
    if [ ! -f "$DEV_DIR/.encryption.key" ]; then
        cp -f "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"
        chmod 600 "$DEV_DIR/.encryption.key"
        echo "✓ Synced encryption key from production"
    else
        # Check if keys are different
        if ! cmp -s "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"; then
            echo "⚠ Dev encryption key differs from production - syncing production key"
            echo "  (This prevents decryption errors when loading production configs)"
            cp -f "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"
            chmod 600 "$DEV_DIR/.encryption.key"
            echo "✓ Synced encryption key from production"
        else
            echo "✓ Dev encryption key matches production"
        fi
    fi
fi

# Copy nodes configuration - WITH VALIDATION
if [ -f "$PROD_DIR/nodes.enc" ]; then
    # Check if production nodes.enc is valid (not corrupted)
    # Only sync if destination doesn't exist OR production file is newer OR dev copy is corrupted
    SHOULD_SYNC=false

    if [ ! -f "$DEV_DIR/nodes.enc" ]; then
        # Destination doesn't exist, safe to sync
        SHOULD_SYNC=true
        echo "  → Dev nodes.enc doesn't exist, will sync from production"
    else
        # Dev copy exists - validate it before deciding
        DEV_SIZE=$(stat -c %s "$DEV_DIR/nodes.enc" 2>/dev/null || echo 0)
        PROD_SIZE=$(stat -c %s "$PROD_DIR/nodes.enc" 2>/dev/null || echo 0)

        # Check if dev copy is suspiciously small (likely corrupted)
        if [ "$DEV_SIZE" -lt 100 ]; then
            echo "  → Dev nodes.enc is too small ($DEV_SIZE bytes), likely corrupted"
            SHOULD_SYNC=true
        # Check if files are different (always prefer production to avoid drift)
        elif ! cmp -s "$PROD_DIR/nodes.enc" "$DEV_DIR/nodes.enc"; then
            echo "  → Dev nodes.enc differs from production, syncing to avoid drift"
            echo "  → (Production: $PROD_SIZE bytes, Dev: $DEV_SIZE bytes)"
            SHOULD_SYNC=true
        else
            # Files are identical
            echo "  → Dev nodes.enc is identical to production, no sync needed"
        fi
    fi

    if [ "$SHOULD_SYNC" = true ]; then
        # Back up the old dev copy if it exists (for debugging)
        if [ -f "$DEV_DIR/nodes.enc" ]; then
            cp -f "$DEV_DIR/nodes.enc" "$DEV_DIR/nodes.enc.before-sync-$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
        fi
        cp -f "$PROD_DIR/nodes.enc" "$DEV_DIR/nodes.enc"
        chmod 600 "$DEV_DIR/nodes.enc"
        echo "✓ Synced nodes configuration from production"
    fi
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