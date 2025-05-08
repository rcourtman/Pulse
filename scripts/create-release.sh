#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

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

# --- Copy Application Files ---
echo "Copying application files to $STAGING_FULL_PATH..."

# Server files (excluding tests)
echo "Copying server files..."
rsync -av --progress server/ "$STAGING_FULL_PATH/server/" --exclude 'tests/'

# Public files (including built CSS and other assets)
echo "Copying public files..."
mkdir -p "$STAGING_FULL_PATH/src" # Ensure parent directory exists
rsync -av --progress src/public/ "$STAGING_FULL_PATH/src/public/"

# Root files
echo "Copying root files..."
cp package.json "$STAGING_FULL_PATH/"
cp package-lock.json "$STAGING_FULL_PATH/"
cp README.md "$STAGING_FULL_PATH/"
cp LICENSE "$STAGING_FULL_PATH/"
cp CHANGELOG.md "$STAGING_FULL_PATH/"
cp .env.example "$STAGING_FULL_PATH/.env.example" # Essential for user configuration

# Scripts (e.g., install-pulse.sh, if intended for end-user)
if [ -d "scripts" ]; then
  echo "Copying scripts..."
  mkdir -p "$STAGING_FULL_PATH/scripts/"
  if [ -f "scripts/install-pulse.sh" ]; then
    cp scripts/install-pulse.sh "$STAGING_FULL_PATH/scripts/"
  fi
  # Add other scripts if they are part of the release
fi

# Docs
if [ -d "docs" ]; then
  echo "Copying docs..."
  rsync -av --progress docs/ "$STAGING_FULL_PATH/docs/"
fi

# --- Install Production Dependencies ---
echo "Installing production dependencies in $STAGING_FULL_PATH..."
(cd "$STAGING_FULL_PATH" && npm install --omit=dev --ignore-scripts)
# --ignore-scripts prevents any package's own postinstall scripts from running during this build phase.
# If your production dependencies have essential postinstall scripts, you might remove --ignore-scripts.

# --- Create Tarball ---
echo "Creating tarball: $TARBALL_NAME..."
# Go into the parent of the directory to be tarred to avoid leading paths in tarball
(cd "$STAGING_PARENT_DIR" && tar -czf "../$TARBALL_NAME" "$RELEASE_DIR_NAME")

# --- Cleanup ---
echo "Cleaning up staging directory ($STAGING_PARENT_DIR)..."
rm -rf "$STAGING_PARENT_DIR"

echo ""
echo "----------------------------------------------------"
echo "Release tarball created: $TARBALL_NAME"
echo "----------------------------------------------------"
echo "To use the tarball:"
echo "1. Copy $TARBALL_NAME to the target server."
echo "2. Extract: tar -xzf $TARBALL_NAME"
echo "3. Navigate into the directory: cd $RELEASE_DIR_NAME"
echo "4. Copy .env.example to .env and configure it: cp .env.example .env"
echo "5. Start the application: npm start (or node server/index.js)"
echo "   (If scripts/install-pulse.sh is provided, consult it for specific setup steps)"
echo "----------------------------------------------------"
