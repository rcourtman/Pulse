#!/bin/bash
# Setup script for creating test release files

set -e

echo "Setting up test update server..."

# Create files directory for mock releases
mkdir -p files

# Function to create a fake release
create_test_release() {
    VERSION=$1
    echo "Creating test release v${VERSION}..."
    
    # Create a temporary directory for the release
    TEMP_DIR=$(mktemp -d)
    
    # Copy the current pulse binary (or create a dummy)
    if [ -f /opt/pulse/pulse ]; then
        cp /opt/pulse/pulse "$TEMP_DIR/pulse"
    else
        echo '#!/bin/bash' > "$TEMP_DIR/pulse"
        echo 'echo "Mock Pulse v'$VERSION'"' >> "$TEMP_DIR/pulse"
        chmod +x "$TEMP_DIR/pulse"
    fi
    
    # Copy frontend files
    if [ -d /opt/pulse/frontend-modern ]; then
        cp -r /opt/pulse/frontend-modern "$TEMP_DIR/"
    else
        mkdir -p "$TEMP_DIR/frontend-modern/dist"
        echo "<h1>Mock Pulse v${VERSION}</h1>" > "$TEMP_DIR/frontend-modern/dist/index.html"
    fi
    
    # Create VERSION file
    echo "$VERSION" > "$TEMP_DIR/VERSION"
    
    # Create the tarball
    cd "$TEMP_DIR"
    tar -czf "/opt/pulse/test-update-server/files/pulse-v${VERSION}-linux-amd64.tar.gz" .
    tar -czf "/opt/pulse/test-update-server/files/pulse-v${VERSION}-linux-arm64.tar.gz" .
    cd /opt/pulse/test-update-server
    
    # Cleanup
    rm -rf "$TEMP_DIR"
    
    echo "Created test release files/pulse-v${VERSION}-linux-amd64.tar.gz"
}

# Create a test release with version 4.0.99
create_test_release "4.0.99"

echo "Test server setup complete!"
echo ""
echo "To start the mock server:"
echo "  python3 server.py 8888"
echo ""
echo "To test against this server, modify the Pulse code to use:"
echo "  http://localhost:8888/repos/rcourtman/Pulse/releases/latest"
echo "instead of the real GitHub API"