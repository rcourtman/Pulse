#!/bin/bash
# Pulse wrapper script - auto-detects architecture and runs the correct binary

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        BINARY="pulse-linux-amd64"
        ;;
    aarch64)
        BINARY="pulse-linux-arm64"
        ;;
    armv7l)
        BINARY="pulse-linux-armv7"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# Check if the binary exists
if [ ! -f "$SCRIPT_DIR/$BINARY" ]; then
    echo "Error: Binary $BINARY not found in $SCRIPT_DIR" >&2
    exit 1
fi

# Make the correct binary executable (in case permissions were lost)
chmod +x "$SCRIPT_DIR/$BINARY" 2>/dev/null || true

# On first run, clean up other architecture binaries to save space
if [ -f "$SCRIPT_DIR/.first-run-cleanup" ]; then
    echo "Cleaning up unused architecture binaries..."
    for bin in "$SCRIPT_DIR"/pulse-linux-*; do
        if [ -f "$bin" ] && [ "$(basename "$bin")" != "$BINARY" ]; then
            rm -f "$bin"
        fi
    done
    rm -f "$SCRIPT_DIR/.first-run-cleanup"
fi

# Execute the correct binary with all arguments
exec "$SCRIPT_DIR/$BINARY" "$@"