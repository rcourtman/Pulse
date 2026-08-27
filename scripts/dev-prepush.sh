#!/usr/bin/env bash
#
# Fast local validation before pushing to main. CI takes 10+ minutes to
# deliver a verdict; this catches the common failure classes in a few:
#
#   1. canonical completion guard (contract/verification coupling, per commit)
#   2. guard/registry snapshot tests when the subsystem registry changed
#   3. mutation registry audits (fail closed on unclassified API routes)
#   4. compilation plus tests for the Go packages the outgoing commits touch
#   5. frontend type-check when frontend-modern changed
#
# Usage: scripts/dev-prepush.sh [base-ref]
#   base-ref defaults to origin/main. Run `git fetch origin` first for an
#   accurate range. Tests here run without -race for speed; CI still runs
#   the full -race suite across all packages.

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BASE="${1:-origin/main}"
FAILURES=0

step() { printf '\n=== %s ===\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILURES=$((FAILURES + 1)); }

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
  echo "Base ref $BASE not found; run git fetch origin first." >&2
  exit 2
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Note: working tree has uncommitted changes; only committed work is checked."
fi

AHEAD=$(git rev-list --count "$BASE"..HEAD)
if [ "$AHEAD" -eq 0 ]; then
  echo "No commits ahead of $BASE; nothing to check."
  exit 0
fi

CHANGED=$(git diff --name-only "$BASE"...HEAD)
CHANGED_GO=$(printf '%s\n' "$CHANGED" | grep -E '\.go$' || true)

step "Canonical completion guard (per commit, CI mode)"
while IFS= read -r commit; do
  if [ -z "$commit" ]; then
    continue
  fi
  if ! git rev-parse --verify --quiet "${commit}^" >/dev/null; then
    echo "Skipping root commit $commit (no parent to diff against)."
    continue
  fi

  # Match canonical-governance.yml exactly: the prepare-commit-msg hook
  # persists an intentional local bypass as a trailer, so recover that
  # reason when validating already-created commits before push.
  reason=$(git log -1 --format='%(trailers:key=Contract-Neutral,valueonly,separator=; )' "$commit" | tr '\n' ' ')
  echo "Checking $commit"
  if ! git diff-tree --no-commit-id --name-only -r "$commit" | \
    PULSE_ALLOW_CONTRACT_NEUTRAL_COMMIT="$reason" \
      python3 scripts/release_control/canonical_completion_guard.py \
        --files-from-stdin --diff-base "${commit}^"; then
    fail "canonical completion guard @ $commit"
  fi
done < <(git rev-list --reverse --no-merges "$BASE"..HEAD)

if printf '%s\n' "$CHANGED" | grep -q 'docs/release-control/v6/internal/subsystems/registry.json'; then
  step "Registry snapshot tests (registry.json changed)"
  python3 scripts/release_control/canonical_completion_guard_test.py || fail "guard snapshot test"
  python3 scripts/release_control/subsystem_lookup_test.py || fail "subsystem lookup snapshot test (slow, ~3m)"
fi

if [ -n "$CHANGED_GO" ]; then
  # internal/api embeds frontend-modern/dist; a stub keeps compilation working
  # in worktrees that never built the frontend. CI does the same for test jobs.
  if [ ! -f internal/api/frontend-modern/dist/index.html ]; then
    mkdir -p internal/api/frontend-modern/dist
    printf '<!doctype html><title>local embed stub</title>\n' > internal/api/frontend-modern/dist/index.html
    echo "Note: created frontend embed stub (dist was absent in this checkout)."
  fi

  step "Go build"
  go build ./... || fail "go build"

  step "Mutation registry audits"
  go test ./internal/mutationregistry ./internal/ai/tools -count=1 \
    -run 'Test(EveryRegisteredMutationHasDisposition|InfrastructureAPIRoutesResolveToRegistry|TransportCommandCatalogsResolveToRegistry|PatrolJobRegistrationResolvesToRegistry|RuntimeCandidateAuditNegativeFixtures|ActionRouteMethodAuthorityIsExactAndLookalikesFailClosed|NonAdmittingTransportMessagesCannotCarryDispatchAuthority|UnknownTransportLookalikeFailsClosed|RegisteredModelMutationSchemasResolveToClosedRegistry|RetiredMutationAliasesCannotShadowExtensions)' \
    || fail "mutation registry audits"

  step "Go tests for touched packages (no -race locally)"
  PKGS=$(printf '%s\n' "$CHANGED_GO" | xargs -n1 dirname | sort -u | sed 's|^|./|')
  echo "$PKGS"
  # shellcheck disable=SC2086
  go test -count=1 -timeout 15m $PKGS || fail "go tests for touched packages"
fi

if printf '%s\n' "$CHANGED" | grep -q '^frontend-modern/'; then
  step "Frontend type-check"
  if [ -d frontend-modern/node_modules ]; then
    (cd frontend-modern && npm run type-check) || fail "frontend type-check"
  else
    echo "Skipped: frontend-modern/node_modules missing (run npm ci there first)."
  fi
fi

step "Result"
if [ "$FAILURES" -gt 0 ]; then
  echo "$FAILURES check(s) failed; fix before pushing."
  exit 1
fi
echo "All pre-push checks passed."
