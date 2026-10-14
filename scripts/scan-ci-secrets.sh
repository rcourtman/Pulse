#!/usr/bin/env bash
# Scan the delivered history, including merge-resolution bytes. Keep gitleaks'
# pinned detector, redaction, exact fingerprints and failure exits unchanged.
set -euo pipefail

expected_head="${1:-}"
base="${2:-}"
if [[ ! "$expected_head" =~ ^[0-9a-f]{40}$ ]]; then
  echo "::error::Secret scan requires an exact expected checkout SHA." >&2
  exit 1
fi
head=$(git rev-parse --verify 'HEAD^{commit}')
if [[ "$head" != "$expected_head" ]]; then
  echo "::error::Secret scan checkout does not match the expected source." >&2
  exit 1
fi
if [[ "$(git rev-parse --is-shallow-repository)" != false ]]; then
  echo "::error::Secret scan requires complete checkout history (fetch-depth: 0)." >&2
  exit 1
fi

# workflow_dispatch, new branches, missing objects and force-pushes do not
# establish an incremental range. Never silently narrow those cases to HEAD^.
revision="$head"
mode="complete checkout history"
if [[ "$base" =~ ^[0-9a-f]{40}$ && "$base" != 0000000000000000000000000000000000000000 && "$base" != "$head" ]] \
  && git cat-file -e "$base^{commit}" 2>/dev/null; then
  if git merge-base --is-ancestor "$base" "$head"; then
    revision="$base..$head"
    mode="delivered commit range"
  else
    status=$?
    if [[ "$status" != 1 ]]; then
      echo "::error::Secret scan could not establish checkout ancestry." >&2
      exit "$status"
    fi
  fi
fi

commits=$(git rev-list --count "$revision" --)
if [[ ! "$commits" =~ ^[1-9][0-9]*$ ]]; then
  echo "::error::Secret scan did not establish a nonempty history." >&2
  exit 1
fi
printf 'Secret scan scope: %s; %s reachable commits; head=%s\n' "$mode" "$commits" "$head"

# This changes merge *diffs*, not traversal: side-branch commits remain in the
# range, including credentials removed before the final tree. The first-parent
# merge diff also covers a resolution authored only in the merge itself.
gitleaks detect \
  --source . \
  --log-opts="--full-history --diff-merges=first-parent --no-ext-diff --no-textconv --no-renames $revision --" \
  --no-banner \
  --redact \
  --exit-code 2 \
  --report-format sarif \
  --report-path results.sarif
