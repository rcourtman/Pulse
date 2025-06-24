#!/bin/bash
set -e

# This script creates an optimized release tarball for Pulse

# --- Configuration ---
PACKAGE_VERSION=$(node -p "require('./package.json').version")
SUGGESTED_RELEASE_VERSION=$(echo "$PACKAGE_VERSION" | sed -E 's/-(dev|alpha|beta|rc|pre)[-.0-9]*$//')

# --- User Input for Version ---
echo "Current version in package.json: $PACKAGE_VERSION"
echo "Suggested stable version: $SUGGESTED_RELEASE_VERSION"
echo ""
echo "Pre-release example: ${SUGGESTED_RELEASE_VERSION}-rc.1"
echo ""
read -p "Enter release version (default: $SUGGESTED_RELEASE_VERSION): " USER_VERSION
RELEASE_VERSION=${USER_VERSION:-$SUGGESTED_RELEASE_VERSION}

if [[ -z "$RELEASE_VERSION" ]]; then
  echo "Error: Release version cannot be empty."
  exit 1
fi
echo "Creating release for version: v$RELEASE_VERSION"

# --- Definitions ---
APP_NAME="pulse"
RELEASE_DIR_NAME="${APP_NAME}-v${RELEASE_VERSION}"
STAGING_DIR=".release-staging"
STAGING_FULL_PATH="$STAGING_DIR/$RELEASE_DIR_NAME"
TARBALL_NAME="${RELEASE_DIR_NAME}.tar.gz"

# --- Cleanup Previous Attempts ---
echo "Cleaning up previous attempts..."
rm -rf "$STAGING_DIR"
rm -f "$TARBALL_NAME"
mkdir -p "$STAGING_FULL_PATH"

# --- Build Step ---
echo "Building CSS..."
npm run build:css
if [ ! -f "src/public/output.css" ]; then
    echo "Error: src/public/output.css not found after build. Aborting."
    exit 1
fi

# --- Create optimized file list ---
echo "Creating release structure..."

# Create directories
mkdir -p "$STAGING_FULL_PATH"/{server,src/public,scripts}

# Copy essential files only
echo "Copying essential files..."

# Server files (exclude tests and development files)
rsync -a --progress \
  --exclude='*.test.js' \
  --exclude='*.spec.js' \
  --exclude='tests/' \
  --exclude='__tests__/' \
  --exclude='.eslintrc*' \
  server/ "$STAGING_FULL_PATH/server/"

# Built CSS and public assets only
rsync -a --progress src/public/ "$STAGING_FULL_PATH/src/public/"

# Root files (only what's needed for production)
cp package.json package-lock.json "$STAGING_FULL_PATH/"
[ -f LICENSE ] && cp LICENSE "$STAGING_FULL_PATH/"
[ -f README.md ] && cp README.md "$STAGING_FULL_PATH/"
[ -f CHANGELOG.md ] && cp CHANGELOG.md "$STAGING_FULL_PATH/"

# Install script only
[ -f scripts/install-pulse.sh ] && cp scripts/install-pulse.sh "$STAGING_FULL_PATH/scripts/"

# --- Install Production Dependencies ---
echo "Installing production dependencies..."
cd "$STAGING_FULL_PATH"
npm ci --omit=dev --no-audit --no-fund
cd - > /dev/null

# --- Remove unnecessary files from node_modules ---
echo "Optimizing node_modules..."
find "$STAGING_FULL_PATH/node_modules" \( \
  -name "*.md" -o \
  -name "*.markdown" -o \
  -name "*.yml" -o \
  -name "*.yaml" -o \
  -name ".npmignore" -o \
  -name ".gitignore" -o \
  -name ".eslintrc*" -o \
  -name ".prettierrc*" -o \
  -name "*.test.js" -o \
  -name "*.spec.js" -o \
  -name "test" -o \
  -name "tests" -o \
  -name "__tests__" -o \
  -name "example" -o \
  -name "examples" -o \
  -name ".github" -o \
  -name ".vscode" \
\) -type f -delete 2>/dev/null || true

find "$STAGING_FULL_PATH/node_modules" \( \
  -name "test" -o \
  -name "tests" -o \
  -name "__tests__" -o \
  -name "example" -o \
  -name "examples" -o \
  -name ".github" -o \
  -name "docs" \
\) -type d -exec rm -rf {} + 2>/dev/null || true

# --- Verify Essential Files ---
echo "Verifying essential files..."
MISSING_FILES=""
[ ! -f "$STAGING_FULL_PATH/package.json" ] && MISSING_FILES="$MISSING_FILES package.json"
[ ! -f "$STAGING_FULL_PATH/server/index.js" ] && MISSING_FILES="$MISSING_FILES server/index.js"
[ ! -f "$STAGING_FULL_PATH/src/public/output.css" ] && MISSING_FILES="$MISSING_FILES src/public/output.css"
[ ! -d "$STAGING_FULL_PATH/node_modules" ] && MISSING_FILES="$MISSING_FILES node_modules/"

if [ -n "$MISSING_FILES" ]; then
    echo "Error: Missing essential files:$MISSING_FILES"
    exit 1
fi
echo "✅ All essential files verified."

# --- Create Tarball ---
echo "Creating tarball: $TARBALL_NAME..."

# Strip extended attributes on macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    find "$STAGING_DIR" -type f -exec xattr -c {} \; 2>/dev/null || true
fi

# Use GNU tar if available
TAR_CMD="tar"
if command -v gtar &> /dev/null; then
    TAR_CMD="gtar"
fi

# Create tarball with consistent permissions
(cd "$STAGING_DIR" && COPYFILE_DISABLE=1 "$TAR_CMD" \
  --owner=0 --group=0 \
  --mode='u+rwX,go+rX,go-w' \
  -czf "../$TARBALL_NAME" "$RELEASE_DIR_NAME")

# --- Calculate sizes ---
STAGING_SIZE=$(du -sh "$STAGING_FULL_PATH" | cut -f1)
TARBALL_SIZE=$(du -sh "$TARBALL_NAME" | cut -f1)

# --- Cleanup ---
echo "Cleaning up staging directory..."
rm -rf "$STAGING_DIR"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Release tarball created: $TARBALL_NAME"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Size information:"
echo "   • Uncompressed: $STAGING_SIZE"
echo "   • Compressed: $TARBALL_SIZE"
echo ""
echo "📦 This tarball includes:"
echo "   ✅ Pre-built CSS assets"
echo "   ✅ Production npm dependencies (optimized)"
echo "   ✅ Server files (no tests/dev files)"
echo "   ✅ Installation script"
echo ""
echo "🚀 Ready for GitHub release upload"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"