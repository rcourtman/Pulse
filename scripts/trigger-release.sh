#!/usr/bin/env bash
set -euo pipefail

# Helper script to trigger a release with pre-flight validation
# Usage: ./scripts/trigger-release.sh 4.30.0 [release-notes-file]

VERSION="${1:-}"
NOTES_FILE_ARG="${2:-}"

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

# Check 4: Up to date with remote
git fetch origin --quiet
LOCAL=$(git rev-parse @)
REMOTE=$(git rev-parse @{u})

if [ "$LOCAL" != "$REMOTE" ]; then
  echo "❌ Local branch is not fully pushed to origin"
  echo ""
  echo "  Local ref:  ${LOCAL}"
  echo "  Remote ref: ${REMOTE}"
  echo ""
  echo "Release automation executes the selected remote ref, not local-only governance state."
  echo "Push ${CURRENT_BRANCH} to origin before triggering the publish workflow."
  git status -sb
  exit 1
else
  echo "✓ Up to date with remote"
fi

python3 scripts/check-workflow-dispatch-inputs.py \
  --workflow-path .github/workflows/create-release.yml \
  --branch "$CURRENT_BRANCH" \
  --require version \
  --require release_notes \
  --require promoted_from_tag \
  --require rollback_version \
  --require ga_date \
  --require v5_eos_date \
  --require hotfix_exception \
  --require hotfix_reason \
  --require unsigned_windows_exception \
  --require unsigned_windows_reason \
  --require draft_only \
  --require mobile_release_decision \
  --require mobile_release_evidence
echo "✓ Remote release-branch publish workflow contract matches governed inputs"

MOBILE_RELEASE_DECISION=""
MOBILE_RELEASE_EVIDENCE=""
MOBILE_REPO="../pulse-mobile"

echo ""
python3 scripts/release_control/mobile_release_gate.py --mobile-repo "$MOBILE_REPO" --summary-only || true
echo ""
echo "Mobile release decision:"
echo "  no-mobile-impact                 Server release has no mobile/relay/onboarding compatibility impact."
echo "  existing-mobile-build-compatible Existing TestFlight/Play candidate is proven compatible."
echo "  mobile-candidate-uploaded        A new mobile candidate has already been uploaded."
echo "  mobile-candidate-required        A mobile candidate is required; stop this release dispatch."
echo ""
read -r -p "Mobile release decision: " MOBILE_RELEASE_DECISION
read -r -p "Mobile release evidence or note: " MOBILE_RELEASE_EVIDENCE
python3 scripts/release_control/mobile_release_gate.py \
  --version "$VERSION" \
  --decision "$MOBILE_RELEASE_DECISION" \
  --evidence "$MOBILE_RELEASE_EVIDENCE"
echo "✓ Mobile release decision recorded"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Ready to release v${VERSION}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next: Provide release notes when prompted"
echo ""

# Check 5: Release notes file
NOTES_FILE="${NOTES_FILE_ARG:-/tmp/release_notes_${VERSION}.md}"
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

python3 scripts/release_control/render_release_body.py \
  --version "$VERSION" \
  --validate-notes-file "$NOTES_FILE"
echo "✓ Release-note Markdown structure validated"

ROLLBACK_VERSION=""
ROLLBACK_COMMAND=""
PROMOTED_FROM_TAG=""
GA_DATE=""
V5_EOS_DATE=""
HOTFIX_EXCEPTION="false"
HOTFIX_REASON=""
UNSIGNED_WINDOWS_EXCEPTION="false"
UNSIGNED_WINDOWS_REASON=""

echo ""
read -r -p "Rollback stable version (for example 5.1.14 or v5.1.14): " ROLLBACK_VERSION
if [ -z "$ROLLBACK_VERSION" ]; then
    echo "❌ Error: rollback version is required"
    exit 1
fi

if [[ "$ROLLBACK_VERSION" == v* ]]; then
    ROLLBACK_COMMAND="./scripts/install.sh --version ${ROLLBACK_VERSION}"
else
    ROLLBACK_COMMAND="./scripts/install.sh --version v${ROLLBACK_VERSION}"
fi

echo ""
echo "Derived rollback command: ${ROLLBACK_COMMAND}"

if [ "$IS_PRERELEASE" != "true" ]; then
    echo ""
    read -r -p "Hotfix exception to bypass 72-hour prerelease soak? [y/N] " HOTFIX_REPLY
    if [[ "$HOTFIX_REPLY" =~ ^[Yy]$ ]]; then
      HOTFIX_EXCEPTION="true"
      read -r -p "Hotfix reason: " HOTFIX_REASON
      if [ -z "$HOTFIX_REASON" ]; then
        echo "❌ Error: hotfix reason is required when bypassing prerelease soak"
        exit 1
      fi
    fi

    DEFAULT_PROMOTED_FROM_TAG=$(git tag -l "v${VERSION}-rc.*" --sort=-version:refname | head -1 || true)
    echo ""
    if [ -n "$DEFAULT_PROMOTED_FROM_TAG" ]; then
      if [ "$HOTFIX_EXCEPTION" = "true" ]; then
        read -r -p "Promoted prerelease tag, or blank for hotfix with no RC lineage [${DEFAULT_PROMOTED_FROM_TAG}]: " PROMOTED_FROM_TAG
      else
        read -r -p "Promoted prerelease tag [${DEFAULT_PROMOTED_FROM_TAG}]: " PROMOTED_FROM_TAG
        PROMOTED_FROM_TAG="${PROMOTED_FROM_TAG:-$DEFAULT_PROMOTED_FROM_TAG}"
      fi
    else
      read -r -p "Promoted prerelease tag (for example v${VERSION}-rc.2; blank only for approved hotfix): " PROMOTED_FROM_TAG
    fi
    if [ -z "$PROMOTED_FROM_TAG" ] && [ "$HOTFIX_EXCEPTION" != "true" ]; then
      echo "❌ Error: promoted prerelease tag is required for stable releases unless this is an approved hotfix"
      exit 1
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

    if [ "$VERSION" = "6.1.0" ]; then
      echo ""
      read -r -p "Use the recorded v6.1.0 unsigned Windows exception? [y/N] " UNSIGNED_WINDOWS_REPLY
      if [[ "$UNSIGNED_WINDOWS_REPLY" =~ ^[Yy]$ ]]; then
        UNSIGNED_WINDOWS_EXCEPTION="true"
        read -r -p "Unsigned Windows exception reason: " UNSIGNED_WINDOWS_REASON
        if [ -z "$UNSIGNED_WINDOWS_REASON" ]; then
          echo "❌ Error: an owner reason is required for the unsigned Windows exception"
          exit 1
        fi
      fi
    fi
fi

# Trigger the workflow
echo ""
echo "Resolving governed promotion metadata..."
RESOLVER_ARGS=(
  --version "$VERSION"
  --rollback-version "$ROLLBACK_VERSION"
  --hotfix-reason "$HOTFIX_REASON"
  --release-notes-file "$NOTES_FILE"
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
  RESOLVER_ARGS+=(--hotfix-exception)
fi
if [ "$UNSIGNED_WINDOWS_EXCEPTION" = "true" ]; then
  RESOLVER_ARGS+=(
    --unsigned-windows-exception
    --unsigned-windows-reason "$UNSIGNED_WINDOWS_REASON"
  )
fi
python3 scripts/release_control/resolve_release_promotion.py "${RESOLVER_ARGS[@]}" >/tmp/pulse-release-metadata.out
cat /tmp/pulse-release-metadata.out

echo ""
echo "Triggering release workflow..."
if [ -n "$NOTES_FILE" ]; then
  jq -n \
    --arg version "$VERSION" \
    --rawfile release_notes "$NOTES_FILE" \
    --arg rollback_version "$ROLLBACK_VERSION" \
    --arg promoted_from_tag "$PROMOTED_FROM_TAG" \
    --arg ga_date "$GA_DATE" \
    --arg v5_eos_date "$V5_EOS_DATE" \
    --argjson hotfix_exception "$HOTFIX_EXCEPTION" \
    --arg hotfix_reason "$HOTFIX_REASON" \
    --argjson unsigned_windows_exception "$UNSIGNED_WINDOWS_EXCEPTION" \
    --arg unsigned_windows_reason "$UNSIGNED_WINDOWS_REASON" \
    --argjson draft_only false \
    --arg mobile_release_decision "$MOBILE_RELEASE_DECISION" \
    --arg mobile_release_evidence "$MOBILE_RELEASE_EVIDENCE" \
    '{
      version: $version,
      release_notes: $release_notes,
      rollback_version: $rollback_version,
      promoted_from_tag: $promoted_from_tag,
      ga_date: $ga_date,
      v5_eos_date: $v5_eos_date,
      hotfix_exception: $hotfix_exception,
      hotfix_reason: $hotfix_reason,
      unsigned_windows_exception: $unsigned_windows_exception,
      unsigned_windows_reason: $unsigned_windows_reason,
      draft_only: $draft_only,
      mobile_release_decision: $mobile_release_decision,
      mobile_release_evidence: $mobile_release_evidence
    }' |
    gh workflow run create-release.yml --ref "$CURRENT_BRANCH" --json
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
