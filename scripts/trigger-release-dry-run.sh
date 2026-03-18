#!/usr/bin/env bash
set -euo pipefail

# Helper script to trigger the governed non-publish release rehearsal.
# Usage: ./scripts/trigger-release-dry-run.sh 6.0.0

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Error: Version number required"
  echo "Usage: $0 <version>"
  echo "Example: $0 6.0.0"
  exit 1
fi

echo "Pre-flight checks for Release Dry Run v${VERSION}..."
echo ""

IS_PRERELEASE="false"
if [[ "$VERSION" =~ -rc\.[0-9]+$ ]] || [[ "$VERSION" =~ -alpha\.[0-9]+$ ]] || [[ "$VERSION" =~ -beta\.[0-9]+$ ]]; then
  IS_PRERELEASE="true"
fi

FILE_VERSION=$(cat VERSION | tr -d '\n')
if [ "$FILE_VERSION" != "$VERSION" ]; then
  echo "❌ VERSION file mismatch"
  echo ""
  echo "  VERSION file contains: ${FILE_VERSION}"
  echo "  Requested version:     ${VERSION}"
  echo ""
  echo "Fix: update VERSION to the governed rehearsal candidate before dispatching."
  exit 1
fi
echo "✓ VERSION file matches (${VERSION})"

if ! git diff-index --quiet HEAD --; then
  echo "❌ Working directory has uncommitted changes"
  echo ""
  echo "Fix: Commit or stash changes before running the rehearsal"
  echo ""
  git status --short
  exit 1
fi
echo "✓ Working directory is clean"

CURRENT_BRANCH=$(git branch --show-current)
REQUIRED_BRANCH=$(python3 scripts/release_control/control_plane.py --branch-for-version "$VERSION")
if [ "$CURRENT_BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "❌ Wrong release branch"
  echo ""
  echo "  Current branch:  ${CURRENT_BRANCH}"
  echo "  Required branch: ${REQUIRED_BRANCH}"
  echo ""
  exit 1
fi
echo "✓ On required branch (${REQUIRED_BRANCH})"

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

python3 scripts/check-workflow-dispatch-inputs.py \
  --workflow-path .github/workflows/release-dry-run.yml \
  --require version \
  --require promoted_from_tag \
  --require rollback_version \
  --require ga_date \
  --require v5_eos_date \
  --require hotfix_exception \
  --require hotfix_reason \
  --require note
echo "✓ Default-branch dry-run workflow contract matches governed inputs"

ROLLBACK_VERSION=""
PROMOTED_FROM_TAG=""
GA_DATE=""
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
    echo "❌ Error: promoted RC tag is required for stable rehearsals"
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
    read -r -p "v6 GA date to publish with GA (YYYY-MM-DD): " GA_DATE
    if [[ ! "$GA_DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
      echo "❌ Error: v6 GA date must be in YYYY-MM-DD form"
      exit 1
    fi

    echo ""
    read -r -p "v5 end-of-support date to publish with GA (YYYY-MM-DD): " V5_EOS_DATE
    if [[ ! "$V5_EOS_DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
      echo "❌ Error: v5 end-of-support date must be in YYYY-MM-DD form"
      exit 1
    fi
  fi
fi

echo ""
echo "Resolving governed promotion metadata..."
RESOLVER_ARGS=(
  --version "$VERSION"
  --rollback-version "$ROLLBACK_VERSION"
)
if [ -n "$PROMOTED_FROM_TAG" ]; then
  RESOLVER_ARGS+=(--promoted-from-tag "$PROMOTED_FROM_TAG")
fi
if [ -n "$GA_DATE" ]; then
  RESOLVER_ARGS+=(--ga-date "$GA_DATE")
fi
if [ -n "$V5_EOS_DATE" ]; then
  RESOLVER_ARGS+=(--v5-eos-date "$V5_EOS_DATE")
fi
if [ "$HOTFIX_EXCEPTION" = "true" ]; then
  RESOLVER_ARGS+=(--hotfix-exception --hotfix-reason "$HOTFIX_REASON")
fi
python3 scripts/release_control/resolve_release_promotion.py "${RESOLVER_ARGS[@]}" >/tmp/pulse-release-dry-run-metadata.out
cat /tmp/pulse-release-dry-run-metadata.out

echo ""
echo "Triggering Release Dry Run workflow..."
gh workflow run release-dry-run.yml \
  --ref "$CURRENT_BRANCH" \
  -f version="$VERSION" \
  -f promoted_from_tag="$PROMOTED_FROM_TAG" \
  -f rollback_version="$ROLLBACK_VERSION" \
  -f ga_date="$GA_DATE" \
  -f v5_eos_date="$V5_EOS_DATE" \
  -f hotfix_exception="$HOTFIX_EXCEPTION" \
  -f hotfix_reason="$HOTFIX_REASON" \
  -f note="Governed release rehearsal for ${VERSION}"

echo ""
echo "✓ Release Dry Run workflow triggered"
echo ""
echo "Monitor progress:"
echo "  gh run list --workflow=release-dry-run.yml --limit 1"
echo "  gh run watch <run-id>"
