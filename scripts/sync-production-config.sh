#!/bin/bash
# Sync production config to dev environment
# This ensures dev mode has access to production nodes when mock is disabled

set -euo pipefail

PROD_DIR="/etc/pulse"
DEV_DIR=${DEV_DIR:-"/opt/pulse/tmp/dev-config"}

# Ensure dev config directory exists
mkdir -p "$DEV_DIR"
chmod 700 "$DEV_DIR"

# If production config is missing, skip sync and let dev config stand alone.
if [ ! -d "$PROD_DIR" ]; then
    echo "⚠ Production config directory not found: $PROD_DIR"
    echo "✓ Skipping production sync; using dev-only config in $DEV_DIR"
    exit 0
fi

# Copy essential production config files to dev
# Skip session/csrf/alert files which are runtime-specific
echo "Syncing production config to dev environment..."
echo "  Source: $PROD_DIR"
echo "  Target: $DEV_DIR"
echo ""

# Track whether we have the production encryption key available
HAVE_PROD_KEY=false

# CRITICAL: Always sync production encryption key to dev when it exists
# First, check if the key is missing but a backup exists - auto-restore it
if [ ! -f "$PROD_DIR/.encryption.key" ]; then
    # Look for backup keys in production directory
    BACKUP_KEY=$(find "$PROD_DIR" -maxdepth 1 -name '.encryption.key.bak*' -type f 2>/dev/null | head -1)
    if [ -n "$BACKUP_KEY" ] && [ -f "$BACKUP_KEY" ]; then
        echo ""
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo "!! ENCRYPTION KEY WAS MISSING - AUTO-RESTORING FROM BACKUP !!"
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo "!! Backup used: $BACKUP_KEY"
        echo "!! "
        echo "!! To find out what deleted the key, run:"
        echo "!!   sudo journalctl -u encryption-key-watcher -n 100"
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo ""
        cp -f "$BACKUP_KEY" "$PROD_DIR/.encryption.key"
        chmod 600 "$PROD_DIR/.encryption.key"
        # Ensure proper ownership (may need root, so try but don't fail)
        chown pulse:pulse "$PROD_DIR/.encryption.key" 2>/dev/null || true
        echo "✓ Restored encryption key from backup"
    fi
fi

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
    HAVE_PROD_KEY=true
else
    echo "⚠ Production encryption key not found. Using dev-only key."
    if [ ! -f "$DEV_DIR/.encryption.key" ]; then
        # Generate a dev-only encryption key so backend can start
        openssl rand -base64 32 > "$DEV_DIR/.encryption.key"
        chmod 600 "$DEV_DIR/.encryption.key"
        echo "✓ Generated dev encryption key at $DEV_DIR/.encryption.key"

        # Remove encrypted artifacts that rely on the missing/old key
        find "$DEV_DIR" -maxdepth 1 -type f -name 'nodes.enc*' -exec rm -f {} \;
        rm -f "$DEV_DIR/email.enc" "$DEV_DIR/webhooks.enc"
        echo "✓ Cleared encrypted artifacts (new key generated)"
    else
        echo "✓ Reusing existing dev encryption key"
    fi
fi

# Copy nodes configuration - WITH VALIDATION
if [ "$HAVE_PROD_KEY" = true ] && [ -f "$PROD_DIR/nodes.enc" ]; then
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

# If we had to clear encrypted nodes, ensure we start from a clean slate
if [ "$HAVE_PROD_KEY" = false ]; then
    rm -f "$DEV_DIR/nodes.json"
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

# Copy email config if it exists and we have the key
if [ "$HAVE_PROD_KEY" = true ] && [ -f "$PROD_DIR/email.enc" ]; then
    cp -f "$PROD_DIR/email.enc" "$DEV_DIR/email.enc"
    chmod 600 "$DEV_DIR/email.enc"
    echo "✓ Synced email configuration"
fi

# Copy webhook config if it exists and we have the key
if [ "$HAVE_PROD_KEY" = true ] && [ -f "$PROD_DIR/webhooks.enc" ]; then
    cp -f "$PROD_DIR/webhooks.enc" "$DEV_DIR/webhooks.enc"
    chmod 600 "$DEV_DIR/webhooks.enc"
    echo "✓ Synced webhook configuration"
fi

# Copy API tokens (needed for agents to authenticate in dev mode)
if [ -f "$PROD_DIR/api_tokens.json" ]; then
    cp -f "$PROD_DIR/api_tokens.json" "$DEV_DIR/api_tokens.json"
    chmod 600 "$DEV_DIR/api_tokens.json"
    echo "✓ Synced API tokens (for agent authentication)"
fi

# Copy AI config if it exists and we have the key
if [ "$HAVE_PROD_KEY" = true ] && [ -f "$PROD_DIR/ai.enc" ]; then
    cp -f "$PROD_DIR/ai.enc" "$DEV_DIR/ai.enc"
    chmod 600 "$DEV_DIR/ai.enc"
    echo "✓ Synced AI configuration"
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
