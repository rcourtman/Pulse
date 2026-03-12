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

IS_PRERELEASE="false"
if [[ "$VERSION" =~ -rc\.[0-9]+$ ]] || [[ "$VERSION" =~ -alpha\.[0-9]+$ ]] || [[ "$VERSION" =~ -beta\.[0-9]+$ ]]; then
  IS_PRERELEASE="true"
fi

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

# Check 3: On required branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$IS_PRERELEASE" = "true" ]; then
  REQUIRED_BRANCH="pulse/v6"
else
  REQUIRED_BRANCH="main"
fi

if [ "$CURRENT_BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "❌ Wrong release branch"
  echo ""
  echo "  Current branch:  ${CURRENT_BRANCH}"
  echo "  Required branch: ${REQUIRED_BRANCH}"
  echo ""
  exit 1
fi
echo "✓ On required branch (${REQUIRED_BRANCH})"

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
  read -p "Generate release notes automatically? [Y/n] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Nn]$ ]]; then
      echo "Generating release notes..."
      # Try to find previous tag for better context
      PREV_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
      
      if ./scripts/generate-release-notes.sh "$VERSION" "$PREV_TAG" > "$NOTES_FILE"; then
          echo "Release notes generated at ${NOTES_FILE}"
          echo ""
          # Show first few lines
          head -n 20 "$NOTES_FILE"
          echo "... (truncated)"
          echo ""
          read -p "Use these release notes? [Y/n] " -n 1 -r
          echo ""
          if [[ $REPLY =~ ^[Nn]$ ]]; then
              echo "Release notes rejected."
              rm "$NOTES_FILE"
              NOTES_FILE=""
          fi
      else
          echo "Failed to generate release notes."
          NOTES_FILE=""
      fi
  else
      NOTES_FILE=""
  fi
fi

if [ -z "$NOTES_FILE" ]; then
    echo "❌ Error: Release notes are required"
    echo ""
    echo "Create ${NOTES_FILE} manually, then run this script again."
    echo ""
    exit 1
fi

ROLLBACK_VERSION=""
PROMOTED_FROM_TAG=""
V5_EOS_DATE=""
HOTFIX_EXCEPTION="false"
HOTFIX_REASON=""

echo ""
read -r -p "Rollback stable version (for example 5.1.14 or v5.1.14): " ROLLBACK_VERSION
if [ -z "$ROLLBACK_VERSION" ]; then
    echo "❌ Error: rollback version is required"
    exit 1
fi

if [ "$IS_PRERELEASE" != "true" ]; then
    DEFAULT_PROMOTED_FROM_TAG=$(git tag -l "v${VERSION}-rc.*" --sort=-version:refname | head -1 || true)
    echo ""
    if [ -n "$DEFAULT_PROMOTED_FROM_TAG" ]; then
      read -r -p "Promoted RC tag [${DEFAULT_PROMOTED_FROM_TAG}]: " PROMOTED_FROM_TAG
      PROMOTED_FROM_TAG="${PROMOTED_FROM_TAG:-$DEFAULT_PROMOTED_FROM_TAG}"
    else
      read -r -p "Promoted RC tag (for example v${VERSION}-rc.2): " PROMOTED_FROM_TAG
    fi
    if [ -z "$PROMOTED_FROM_TAG" ]; then
      echo "❌ Error: promoted RC tag is required for stable releases"
      exit 1
    fi

    echo ""
    read -r -p "Hotfix exception to bypass 72-hour RC soak? [y/N] " HOTFIX_REPLY
    if [[ "$HOTFIX_REPLY" =~ ^[Yy]$ ]]; then
      HOTFIX_EXCEPTION="true"
      read -r -p "Hotfix reason: " HOTFIX_REASON
      if [ -z "$HOTFIX_REASON" ]; then
        echo "❌ Error: hotfix reason is required when bypassing RC soak"
        exit 1
      fi
    fi

    if [ "$VERSION" = "6.0.0" ]; then
      echo ""
      read -r -p "v5 end-of-support date to publish with GA (YYYY-MM-DD): " V5_EOS_DATE
      if [[ ! "$V5_EOS_DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
        echo "❌ Error: v5 end-of-support date must be in YYYY-MM-DD form"
        exit 1
      fi
    fi
fi

# Trigger the workflow
echo ""
echo "Triggering release workflow..."
if [ -n "$NOTES_FILE" ]; then
  gh workflow run create-release.yml \
    -f version="${VERSION}" \
    -f release_notes="$(cat "$NOTES_FILE")" \
    -f rollback_version="${ROLLBACK_VERSION}" \
    -f promoted_from_tag="${PROMOTED_FROM_TAG}" \
    -f v5_eos_date="${V5_EOS_DATE}" \
    -f hotfix_exception="${HOTFIX_EXCEPTION}" \
    -f hotfix_reason="${HOTFIX_REASON}"
else
  # This should be unreachable due to check above, but kept for safety
  echo "❌ Error: Release notes are required"
  exit 1
fi

echo ""
echo "✓ Release workflow triggered"
echo ""
echo "Monitor progress:"
echo "  gh run list --workflow=create-release.yml --limit 1"
echo "  gh run watch <run-id>"
echo ""
