#!/usr/bin/env bash
set -euo pipefail

# Helper script to trigger a release with pre-flight validation
# Usage: ./scripts/trigger-release.sh 4.30.0

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Error: Version number required"
  echo "Usage: $0 <version>"
  echo "Example: $0 4.30.0"
  exit 1
fi

echo "Pre-flight checks for release v${VERSION}..."
echo ""

# Check 1: VERSION file matches
FILE_VERSION=$(cat VERSION | tr -d '\n')
if [ "$FILE_VERSION" != "$VERSION" ]; then
  echo "❌ VERSION file mismatch"
  echo ""
  echo "  VERSION file contains: ${FILE_VERSION}"
  echo "  Requested version:     ${VERSION}"
  echo ""
  echo "Fix: Update VERSION file and commit:"
  echo "  echo '${VERSION}' > VERSION"
  echo "  git add VERSION"
  echo "  git commit -m 'Prepare v${VERSION} release'"
  echo "  git push"
  echo ""
  exit 1
fi
echo "✓ VERSION file matches (${VERSION})"

# Check 2: Working directory is clean
if ! git diff-index --quiet HEAD --; then
  echo "❌ Working directory has uncommitted changes"
  echo ""
  echo "Fix: Commit or stash changes before releasing"
  echo ""
  git status --short
  exit 1
fi
echo "✓ Working directory is clean"

# Check 3: On main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "⚠️  Warning: Not on main branch (current: ${CURRENT_BRANCH})"
  echo ""
  read -p "Continue anyway? [y/N] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 1
  fi
else
  echo "✓ On main branch"
fi

# Check 4: Up to date with remote
git fetch origin --quiet
LOCAL=$(git rev-parse @)
REMOTE=$(git rev-parse @{u})

if [ "$LOCAL" != "$REMOTE" ]; then
  echo "⚠️  Warning: Local and remote branches have diverged"
  git status -sb
  echo ""
  read -p "Continue anyway? [y/N] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 1
  fi
else
  echo "✓ Up to date with remote"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Ready to release v${VERSION}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next: Provide release notes when prompted"
echo ""

# Check 5: Release notes file (optional)
NOTES_FILE="/tmp/release_notes_${VERSION}.md"
if [ -f "$NOTES_FILE" ]; then
  echo "Found release notes file: ${NOTES_FILE}"
  echo ""
  cat "$NOTES_FILE"
  echo ""
  read -p "Use these release notes? [Y/n] " -n 1 -r
  echo ""
  if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo "Release notes file ignored"
    NOTES_FILE=""
  fi
else
  echo "No release notes file found at ${NOTES_FILE}"
  echo ""
  echo "Create release notes manually or let the workflow prompt you."
  echo ""
  read -p "Continue without release notes file? [y/N] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 1
  fi
  NOTES_FILE=""
fi

# Trigger the workflow
echo ""
echo "Triggering release workflow..."
if [ -n "$NOTES_FILE" ]; then
  gh workflow run create-release.yml \
    -f version="${VERSION}" \
    -f release_notes="$(cat "$NOTES_FILE")"
else
  echo ""
  echo "❌ Error: Release notes are required"
  echo ""
  echo "Create ${NOTES_FILE} with your release notes, then run this script again."
  echo ""
  exit 1
fi

echo ""
echo "✓ Release workflow triggered"
echo ""
echo "Monitor progress:"
echo "  gh run list --workflow=create-release.yml --limit 1"
echo "  gh run watch <run-id>"
echo ""
