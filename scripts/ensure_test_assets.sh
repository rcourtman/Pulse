#!/bin/bash
# scripts/ensure_test_assets.sh
# Ensures that frontend assets exist for Go embed requirements during testing.
set -euo pipefail

ASSET_DIR="internal/api/frontend-modern/dist"
INDEX_FILE="$ASSET_DIR/index.html"

# If directory doesn't exist, create it
if [ ! -d "$ASSET_DIR" ]; then
    echo "Test assets missing. Creating dummy frontend assets in $ASSET_DIR..."
    mkdir -p "$ASSET_DIR"
fi

# If index.html doesn't exist, create a dummy one
if [ ! -f "$INDEX_FILE" ]; then
    echo "Creating dummy index.html for testing..."
    cat > "$INDEX_FILE" <<EOF
<!DOCTYPE html>
<html>
<head><title>Test Asset</title></head>
<body><h1>Pulse Test Asset</h1></body>
</html>
EOF
fi
