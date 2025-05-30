#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

# This script creates a release tarball for Pulse
# Note: COPYFILE_DISABLE=1 is used when creating the tarball to prevent
# macOS extended attributes from being included, which would cause
# "Ignoring unknown extended header keyword" warnings on extraction

# --- Configuration ---
# Attempt to get version from package.json
PACKAGE_VERSION=$(node -p "require('./package.json').version")
# Suggest a release version by stripping common pre-release suffixes like -dev.X or -alpha.X etc.
SUGGESTED_RELEASE_VERSION=$(echo "$PACKAGE_VERSION" | sed -E 's/-(dev|alpha|beta|rc|pre)[-.0-9]*$//')

# --- User Input for Version ---
echo "Current version in package.json: $PACKAGE_VERSION"
read -p "Enter release version (default: $SUGGESTED_RELEASE_VERSION): " USER_VERSION
RELEASE_VERSION=${USER_VERSION:-$SUGGESTED_RELEASE_VERSION}

if [[ -z "$RELEASE_VERSION" ]]; then
  echo "Error: Release version cannot be empty."
  exit 1
fi
echo "Creating release for version: v$RELEASE_VERSION"

# --- Definitions ---
APP_NAME="pulse" # Or derive from package.json if preferred
RELEASE_DIR_NAME="${APP_NAME}-v${RELEASE_VERSION}"
STAGING_PARENT_DIR="pulse-release-staging" # Temporary parent for the release content
STAGING_FULL_PATH="$STAGING_PARENT_DIR/$RELEASE_DIR_NAME"
TARBALL_NAME="${RELEASE_DIR_NAME}.tar.gz"

# --- Cleanup Previous Attempts ---
echo "Cleaning up previous attempts..."
rm -rf "$STAGING_PARENT_DIR"
rm -f "$TARBALL_NAME"
mkdir -p "$STAGING_FULL_PATH"

# --- Build Step ---
echo "Building CSS..."
npm run build:css
if [ ! -f "src/public/output.css" ]; then
    echo "Error: src/public/output.css not found after build. Aborting."
    exit 1
fi

# --- Strip Extended Attributes First ---
echo "Stripping macOS extended attributes from source files..."
find . -type f \( -name "*.js" -o -name "*.json" -o -name "*.md" -o -name "*.css" -o -name "*.html" -o -name "*.sh" \) -exec xattr -c {} \; 2>/dev/null || true

# --- Copy Application Files ---
echo "Copying application files to $STAGING_FULL_PATH..."

# Server files (excluding tests)
echo "Copying server files..."
COPYFILE_DISABLE=1 rsync -av --progress server/ "$STAGING_FULL_PATH/server/" --exclude 'tests/'

# Source files (including built CSS, Tailwind config, and public assets)
echo "Copying source files..."
mkdir -p "$STAGING_FULL_PATH/src" # Ensure parent directory exists
COPYFILE_DISABLE=1 rsync -av --progress src/public/ "$STAGING_FULL_PATH/src/public/"

# Copy CSS build files and config
COPYFILE_DISABLE=1 cp src/index.css "$STAGING_FULL_PATH/src/" 2>/dev/null || echo "Warning: src/index.css not found"
COPYFILE_DISABLE=1 cp src/tailwind.config.js "$STAGING_FULL_PATH/src/" 2>/dev/null || echo "Warning: src/tailwind.config.js not found"
COPYFILE_DISABLE=1 cp src/postcss.config.js "$STAGING_FULL_PATH/src/" 2>/dev/null || echo "Warning: src/postcss.config.js not found"

# Root files
echo "Copying root files..."
COPYFILE_DISABLE=1 cp package.json "$STAGING_FULL_PATH/"
COPYFILE_DISABLE=1 cp package-lock.json "$STAGING_FULL_PATH/"
COPYFILE_DISABLE=1 cp README.md "$STAGING_FULL_PATH/"
COPYFILE_DISABLE=1 cp LICENSE "$STAGING_FULL_PATH/"
COPYFILE_DISABLE=1 cp CHANGELOG.md "$STAGING_FULL_PATH/"
# .env.example no longer needed - configuration is now done via web interface

# Scripts (e.g., install-pulse.sh, if intended for end-user)
if [ -d "scripts" ]; then
  echo "Copying scripts..."
  mkdir -p "$STAGING_FULL_PATH/scripts/"
  if [ -f "scripts/install-pulse.sh" ]; then
    COPYFILE_DISABLE=1 cp scripts/install-pulse.sh "$STAGING_FULL_PATH/scripts/"
  fi
  # Add other scripts if they are part of the release
fi

# Docs
if [ -d "docs" ]; then
  echo "Copying docs..."
  COPYFILE_DISABLE=1 rsync -av --progress docs/ "$STAGING_FULL_PATH/docs/"
fi

# --- Install Production Dependencies ---
echo "Installing production dependencies in $STAGING_FULL_PATH..."
(cd "$STAGING_FULL_PATH" && npm install --omit=dev --ignore-scripts)
# --ignore-scripts prevents any package's own postinstall scripts from running during this build phase.
# If your production dependencies have essential postinstall scripts, you might remove --ignore-scripts.

# --- Verify Essential Files ---
echo "Verifying essential files for tarball installation..."
MISSING_FILES=""
[ ! -f "$STAGING_FULL_PATH/package.json" ] && MISSING_FILES="$MISSING_FILES package.json"
# .env.example no longer required - web-based configuration
[ ! -f "$STAGING_FULL_PATH/server/index.js" ] && MISSING_FILES="$MISSING_FILES server/index.js"
[ ! -f "$STAGING_FULL_PATH/src/public/output.css" ] && MISSING_FILES="$MISSING_FILES src/public/output.css"
[ ! -d "$STAGING_FULL_PATH/node_modules" ] && MISSING_FILES="$MISSING_FILES node_modules/"

if [ -n "$MISSING_FILES" ]; then
    echo "Error: Missing essential files for tarball installation:$MISSING_FILES"
    echo "The install script expects these files to be present in the tarball."
    exit 1
fi
echo "✅ All essential files verified for tarball installation."

# --- Final Extended Attributes Cleanup ---
echo "Final cleanup: Stripping extended attributes from staging directory..."
find "$STAGING_PARENT_DIR" -type f -exec xattr -c {} \; 2>/dev/null || true

# --- Create Tarball ---
echo "Creating tarball: $TARBALL_NAME..."

# Detect and use GNU tar if available (preferred on macOS to avoid extended attributes)
TAR_CMD="tar"
if command -v gtar &> /dev/null; then
    TAR_CMD="gtar"
    echo "Using GNU tar to avoid macOS extended attributes"
elif tar --version 2>&1 | grep -q "GNU tar"; then
    TAR_CMD="tar"
    echo "Using GNU tar"
else
    TAR_CMD="tar"
    echo "Using system tar with COPYFILE_DISABLE=1"
fi

# Go into the parent of the directory to be tarred to avoid leading paths in tarball
if [ "$TAR_CMD" = "gtar" ]; then
    # GNU tar doesn't need COPYFILE_DISABLE and handles extended attributes properly
    (cd "$STAGING_PARENT_DIR" && "$TAR_CMD" -czf "../$TARBALL_NAME" "$RELEASE_DIR_NAME")
else
    # BSD tar (macOS) needs COPYFILE_DISABLE=1 to prevent extended attributes
    (cd "$STAGING_PARENT_DIR" && COPYFILE_DISABLE=1 "$TAR_CMD" -czf "../$TARBALL_NAME" "$RELEASE_DIR_NAME")
fi

# --- Cleanup ---
echo "Cleaning up staging directory ($STAGING_PARENT_DIR)..."
rm -rf "$STAGING_PARENT_DIR"

echo ""
echo "----------------------------------------------------"
echo "Release tarball created: $TARBALL_NAME"
echo "----------------------------------------------------"
echo "📦 This tarball includes:"
echo "   ✅ Pre-built CSS assets"
echo "   ✅ Production npm dependencies"
echo "   ✅ All server and client files"
echo "   ✅ Installation scripts"
echo ""
echo "🚀 Installation options:"
echo "1. RECOMMENDED: Use the install script (faster, automated):"
echo "   curl -sLO https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh"
echo "   chmod +x install-pulse.sh"
echo "   sudo ./install-pulse.sh"
echo "   (The script will automatically use this tarball for faster installation)"
echo ""
echo "2. Manual installation:"
echo "   - Copy $TARBALL_NAME to target server"
echo "   - Extract: tar -xzf $TARBALL_NAME"
echo "   - Navigate: cd $RELEASE_DIR_NAME"
echo "   - Start: npm start"
echo "   - Configure via web interface at http://localhost:7655"
echo "----------------------------------------------------"
