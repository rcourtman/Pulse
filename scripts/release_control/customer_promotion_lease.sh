#!/usr/bin/env bash
set -euo pipefail

LOCK_REF="refs/heads/release-customer-promotion-lock"
COMMAND="${1:-}"

require_runtime() {
  : "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
  : "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"
  : "${GITHUB_RUN_ATTEMPT:?GITHUB_RUN_ATTEMPT is required}"
  : "${GITHUB_ACTOR:?GITHUB_ACTOR is required}"
  : "${GITHUB_ACTOR_ID:?GITHUB_ACTOR_ID is required}"
  : "${GH_TOKEN:?GH_TOKEN is required}"
}

acquire() {
  local tag="${2:-}"
  local timeout_seconds="${CUSTOMER_PROMOTION_LEASE_TIMEOUT_SECONDS:-21000}"
  if [ -z "${tag}" ]; then
    echo "::error::A lease owner label is required."
    return 1
  fi
  if [[ ! "${timeout_seconds}" =~ ^[0-9]+$ ]] || [ "${timeout_seconds}" -lt 1 ]; then
    echo "::error::CUSTOMER_PROMOTION_LEASE_TIMEOUT_SECONDS must be a positive integer."
    return 1
  fi

  export GIT_AUTHOR_NAME="${GITHUB_ACTOR}"
  export GIT_AUTHOR_EMAIL="${GITHUB_ACTOR_ID}+${GITHUB_ACTOR}@users.noreply.github.com"
  export GIT_COMMITTER_NAME="${GIT_AUTHOR_NAME}"
  export GIT_COMMITTER_EMAIL="${GIT_AUTHOR_EMAIL}"
  local lock_message="Release customer promotion lock run=${GITHUB_RUN_ID} attempt=${GITHUB_RUN_ATTEMPT} owner=${tag}"
  local lock_commit
  lock_commit="$(printf '%s\n' "${lock_message}" | git commit-tree "$(git rev-parse 'HEAD^{tree}')" -p HEAD)"
  local deadline=$((SECONDS + timeout_seconds))

  while (( SECONDS < deadline )); do
    if git push origin "${lock_commit}:${LOCK_REF}" >/dev/null 2>&1; then
      echo "lock_sha=${lock_commit}" >> "${GITHUB_OUTPUT}"
      echo "[OK] Acquired global customer-promotion lease ${lock_commit}."
      return 0
    fi

    local observed_sha
    observed_sha="$(git ls-remote origin "${LOCK_REF}" | awk '{print $1}')"
    if [[ ! "${observed_sha}" =~ ^[0-9a-f]{40}$ ]]; then
      # GitHub's Actions token can fast-forward this lease ref but may reject
      # creating it with git push. Seed the absent ref at the checked-out
      # release-control commit so every contender still has to win the normal
      # fast-forward push of its owner commit. A concurrent creator returning
      # 422 is harmless; the next loop observes whichever contender won.
      local bootstrap_sha
      bootstrap_sha="$(git rev-parse HEAD)"
      if jq -n \
           --arg ref "${LOCK_REF}" \
           --arg sha "${bootstrap_sha}" \
           '{ref: $ref, sha: $sha}' | \
         gh api \
           --method POST \
           "repos/${GITHUB_REPOSITORY}/git/refs" \
           --input - >/dev/null 2>&1; then
        echo "Bootstrapped absent customer-promotion lease ref at ${bootstrap_sha}."
      else
        echo "Customer-promotion lease changed while acquiring; retrying."
      fi
      sleep 15
      continue
    fi
    git fetch --no-tags origin "${observed_sha}" >/dev/null 2>&1
    local owner_message owner_run_id owner_attempt owner_status
    owner_message="$(git show -s --format=%B "${observed_sha}")"
    owner_run_id="$(sed -n -E 's/.* run=([0-9]+).*/\1/p' <<<"${owner_message}" | head -1)"
    owner_attempt="$(sed -n -E 's/.* attempt=([0-9]+).*/\1/p' <<<"${owner_message}" | head -1)"
    local reclaim=false
    if [ "${owner_run_id}" = "${GITHUB_RUN_ID}" ] && [ "${owner_attempt}" != "${GITHUB_RUN_ATTEMPT}" ]; then
      reclaim=true
    elif [[ "${owner_run_id}" =~ ^[0-9]+$ ]]; then
      owner_status="$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${owner_run_id}" --jq '.status' 2>/dev/null || true)"
      if [ "${owner_status}" = "completed" ]; then
        reclaim=true
      fi
    fi

    if [ "${reclaim}" = "true" ]; then
      echo "Reclaiming stale customer-promotion lease ${observed_sha}."
      git push \
        --force-with-lease="${LOCK_REF}:${observed_sha}" \
        origin ":${LOCK_REF}" >/dev/null 2>&1 || true
      continue
    fi
    echo "Global customer-promotion lease is held by run ${owner_run_id:-unknown}; waiting."
    sleep 30
  done

  echo "::error::Timed out acquiring the global customer-promotion lease."
  return 1
}

release() {
  local lock_sha="${2:-}"
  if [[ ! "${lock_sha}" =~ ^[0-9a-f]{40}$ ]]; then
    echo "::error::A valid owned lease SHA is required."
    return 1
  fi
  local observed_sha
  observed_sha="$(git ls-remote origin "${LOCK_REF}" | awk '{print $1}')"
  if [ -z "${observed_sha}" ]; then
    echo "Customer-promotion lease is already absent."
    return 0
  fi
  if [ "${observed_sha}" != "${lock_sha}" ]; then
    echo "::error::Refusing to release customer-promotion lease ${observed_sha}; this run owns ${lock_sha}."
    return 1
  fi
  git push \
    --force-with-lease="${LOCK_REF}:${lock_sha}" \
    origin ":${LOCK_REF}"
  echo "[OK] Released global customer-promotion lease ${lock_sha}."
}

require_runtime
case "${COMMAND}" in
  acquire)
    acquire "$@"
    ;;
  release)
    release "$@"
    ;;
  *)
    echo "Usage: $0 {acquire <owner-label>|release <lock-sha>}" >&2
    exit 2
    ;;
esac
