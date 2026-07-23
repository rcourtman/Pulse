#!/usr/bin/env bash
set -euo pipefail

MODE="publish"
VERSION=""
MOBILE_RELEASE_DECISION=""
MOBILE_RELEASE_EVIDENCE=""
HOTFIX_REASON=""
UNSIGNED_WINDOWS_REASON=""

usage() {
  cat <<'EOF'
Usage: scripts/trigger-stable-patch.sh [--dry-run] [options] [version]

Dispatches exactly one governed workflow. The default release workflow builds
and validates an immutable candidate before publication. Use --dry-run only
when a no-public-release rehearsal is required.

Options:
  --dry-run                         Dispatch Release Dry Run only.
  --mobile-release-decision VALUE   Override the inferred mobile decision.
  --mobile-release-evidence VALUE   Evidence for a mobile compatibility decision.
  --emergency-hotfix-reason VALUE   Bypass an RC-required risk with an explicit reason.
  --unsigned-windows-exception-reason VALUE
                                    Use an approved version-bound unsigned Windows exception.
  -h, --help                        Show this help.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      MODE="dry-run"
      shift
      ;;
    --mobile-release-decision)
      MOBILE_RELEASE_DECISION="${2:?--mobile-release-decision requires a value}"
      shift 2
      ;;
    --mobile-release-evidence)
      MOBILE_RELEASE_EVIDENCE="${2:?--mobile-release-evidence requires a value}"
      shift 2
      ;;
    --emergency-hotfix-reason)
      HOTFIX_REASON="${2:?--emergency-hotfix-reason requires a value}"
      shift 2
      ;;
    --unsigned-windows-exception-reason)
      UNSIGNED_WINDOWS_REASON="${2:?--unsigned-windows-exception-reason requires a value}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [ -n "$VERSION" ]; then
        echo "Only one version may be supplied." >&2
        exit 2
      fi
      VERSION="$1"
      shift
      ;;
  esac
done

VERSION="${VERSION:-$(tr -d '\n' < VERSION)}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[1-9][0-9]*$ ]]; then
  echo "Stable patch version required, got: ${VERSION}" >&2
  exit 1
fi

FILE_VERSION="$(tr -d '\n' < VERSION)"
if [ "$FILE_VERSION" != "$VERSION" ]; then
  echo "VERSION contains ${FILE_VERSION}; requested ${VERSION}." >&2
  exit 1
fi

if [ -n "$(git status --porcelain=v1)" ]; then
  echo "The release worktree must be clean." >&2
  git status --short
  exit 1
fi

CURRENT_BRANCH="$(git branch --show-current)"
REQUIRED_BRANCH="$(python3 scripts/release_control/control_plane.py --branch-for-version "$VERSION")"
if [ "$CURRENT_BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "Version ${VERSION} must be released from ${REQUIRED_BRANCH}, not ${CURRENT_BRANCH}." >&2
  exit 1
fi

git fetch --quiet --prune origin "$REQUIRED_BRANCH" --tags
LOCAL_SHA="$(git rev-parse HEAD)"
REMOTE_SHA="$(git rev-parse "origin/${REQUIRED_BRANCH}")"
if [ "$LOCAL_SHA" != "$REMOTE_SHA" ]; then
  echo "The exact release commit must already be pushed to origin/${REQUIRED_BRANCH}." >&2
  exit 1
fi

NOTES_FILE="docs/releases/RELEASE_NOTES_v${VERSION}.md"
if [ ! -s "$NOTES_FILE" ]; then
  echo "Canonical release notes are required at ${NOTES_FILE}." >&2
  exit 1
fi

RESOLVER_ARGS=(
  --version "$VERSION"
  --derive-rollback-latest-stable
  --release-notes-file "$NOTES_FILE"
)
HOTFIX_EXCEPTION="false"
if [ -n "$HOTFIX_REASON" ]; then
  HOTFIX_EXCEPTION="true"
  RESOLVER_ARGS+=(--hotfix-exception --hotfix-reason "$HOTFIX_REASON")
fi
UNSIGNED_WINDOWS_EXCEPTION="false"
if [ -n "$UNSIGNED_WINDOWS_REASON" ]; then
  UNSIGNED_WINDOWS_EXCEPTION="true"
  RESOLVER_ARGS+=(
    --unsigned-windows-exception
    --unsigned-windows-reason "$UNSIGNED_WINDOWS_REASON"
  )
fi

PROMOTION_METADATA="$(python3 scripts/release_control/resolve_release_promotion.py "${RESOLVER_ARGS[@]}")"
ROLLBACK_TAG="$(awk -F= '$1 == "rollback_tag" {print $2}' <<<"$PROMOTION_METADATA")"
if [ -z "$ROLLBACK_TAG" ]; then
  echo "Release preflight did not resolve a rollback tag." >&2
  exit 1
fi

MOBILE_IMPACT_PATHS="$({
  git diff --name-only "${ROLLBACK_TAG}..HEAD" | rg '^(internal/(relay|mobile)/|pkg/relay/|internal/api/(cloud_handoff|magic_link|mobile)|tests/integration/.*(mobile|relay)|scripts/.*mobile)'
} || true)"
if [ -z "$MOBILE_RELEASE_DECISION" ]; then
  if [ -n "$MOBILE_IMPACT_PATHS" ]; then
    echo "Mobile-facing paths changed since ${ROLLBACK_TAG}:" >&2
    printf '%s\n' "$MOBILE_IMPACT_PATHS" >&2
    echo "Supply --mobile-release-decision and --mobile-release-evidence after completing the governed mobile check." >&2
    exit 1
  fi
  MOBILE_RELEASE_DECISION="no-mobile-impact"
  MOBILE_RELEASE_EVIDENCE="No mobile-facing paths changed between ${ROLLBACK_TAG} and ${LOCAL_SHA}."
fi

python3 scripts/release_control/mobile_release_gate.py \
  --version "$VERSION" \
  --decision "$MOBILE_RELEASE_DECISION" \
  --evidence "$MOBILE_RELEASE_EVIDENCE"

if [ "$MODE" = "dry-run" ]; then
  WORKFLOW="release-dry-run.yml"
  python3 scripts/check-workflow-dispatch-inputs.py \
    --workflow-path .github/workflows/release-dry-run.yml \
    --branch "$CURRENT_BRANCH" \
    --require version \
    --require promoted_from_tag \
    --require rollback_version \
    --require ga_date \
    --require v5_eos_date \
    --require hotfix_exception \
    --require hotfix_reason \
    --require unsigned_windows_exception \
    --require unsigned_windows_reason \
    --require note \
    --require mobile_release_decision \
    --require mobile_release_evidence

  gh workflow run "$WORKFLOW" \
    --ref "$CURRENT_BRANCH" \
    -f version="$VERSION" \
    -f promoted_from_tag="" \
    -f rollback_version="$ROLLBACK_TAG" \
    -f ga_date="" \
    -f v5_eos_date="" \
    -f hotfix_exception="$HOTFIX_EXCEPTION" \
    -f hotfix_reason="$HOTFIX_REASON" \
    -f unsigned_windows_exception="$UNSIGNED_WINDOWS_EXCEPTION" \
    -f unsigned_windows_reason="$UNSIGNED_WINDOWS_REASON" \
    -f note="Stable patch preflight for ${VERSION} at ${LOCAL_SHA}" \
    -f mobile_release_decision="$MOBILE_RELEASE_DECISION" \
    -f mobile_release_evidence="$MOBILE_RELEASE_EVIDENCE"
else
  python3 scripts/release_control/render_release_body.py \
    --version "$VERSION" \
    --validate-notes-file "$NOTES_FILE"

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

  jq -n \
    --arg version "$VERSION" \
    --rawfile release_notes "$NOTES_FILE" \
    --arg promoted_from_tag "" \
    --arg rollback_version "$ROLLBACK_TAG" \
    --arg ga_date "" \
    --arg v5_eos_date "" \
    --arg hotfix_exception "$HOTFIX_EXCEPTION" \
    --arg hotfix_reason "$HOTFIX_REASON" \
    --arg unsigned_windows_exception "$UNSIGNED_WINDOWS_EXCEPTION" \
    --arg unsigned_windows_reason "$UNSIGNED_WINDOWS_REASON" \
    --arg draft_only "false" \
    --arg mobile_release_decision "$MOBILE_RELEASE_DECISION" \
    --arg mobile_release_evidence "$MOBILE_RELEASE_EVIDENCE" \
    '{
      version: $version,
      release_notes: $release_notes,
      promoted_from_tag: $promoted_from_tag,
      rollback_version: $rollback_version,
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

fi

echo "Dispatched ${WORKFLOW:-create-release.yml} for v${VERSION} at ${LOCAL_SHA}."
