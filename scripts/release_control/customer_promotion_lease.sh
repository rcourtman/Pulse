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

  : "${TARGET_COMMITISH:?TARGET_COMMITISH is required}"
  : "${RELEASE_ID:?RELEASE_ID is required}"
  : "${SOURCE_RELEASE_RUN_ID:?SOURCE_RELEASE_RUN_ID is required}"
  : "${R2_PREFIX:?R2_PREFIX is required}"
  : "${ACTIVATION_OWNER_RUN_ID:?ACTIVATION_OWNER_RUN_ID is required}"
  : "${ACTIVATION_MARKER_SHA256:?ACTIVATION_MARKER_SHA256 is required}"
  if [[ ! "${TARGET_COMMITISH}" =~ ^[0-9a-f]{40}$ ]] || \
     [[ ! "${RELEASE_ID}" =~ ^[0-9]+$ ]] || \
     [[ ! "${SOURCE_RELEASE_RUN_ID}" =~ ^[0-9]+$ ]] || \
     [[ ! "${ACTIVATION_OWNER_RUN_ID}" =~ ^[0-9]+$ ]] || \
     [[ ! "${ACTIVATION_MARKER_SHA256}" =~ ^[0-9a-f]{64}$ ]]; then
    echo "::error::Customer-promotion owner identity is malformed."
    return 1
  fi

  export GIT_AUTHOR_NAME="${GITHUB_ACTOR}"
  export GIT_AUTHOR_EMAIL="${GITHUB_ACTOR_ID}+${GITHUB_ACTOR}@users.noreply.github.com"
  export GIT_COMMITTER_NAME="${GIT_AUTHOR_NAME}"
  export GIT_COMMITTER_EMAIL="${GIT_AUTHOR_EMAIL}"
  local lock_message="Release customer promotion lock run=${GITHUB_RUN_ID} attempt=${GITHUB_RUN_ATTEMPT} owner=${tag}"
  local owner_dir owner_record owner_asset_name owner_asset_sha256 owner_ref
  local owner_blob lock_index lock_tree lock_commit
  owner_dir="$(mktemp -d)"
  lock_index="$(mktemp)"
  owner_asset_name="release-convergence-owner-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}.json"
  owner_ref="refs/tags/${owner_asset_name%.json}"
  owner_record="${owner_dir}/${owner_asset_name}"
  jq -n \
    --arg tag "${tag}" \
    --arg target_commitish "${TARGET_COMMITISH}" \
    --arg release_id "${RELEASE_ID}" \
    --arg source_release_run_id "${SOURCE_RELEASE_RUN_ID}" \
    --arg r2_prefix "${R2_PREFIX}" \
    --arg activation_owner_run_id "${ACTIVATION_OWNER_RUN_ID}" \
    --arg activation_marker_sha256 "${ACTIVATION_MARKER_SHA256}" \
    --arg convergence_run_id "${GITHUB_RUN_ID}" \
    --arg owner_asset_name "${owner_asset_name}" \
    '{
      schema_version: 2,
      tag: $tag,
      target_commitish: $target_commitish,
      release_id: $release_id,
      source_release_run_id: $source_release_run_id,
      r2_prefix: $r2_prefix,
      activation_owner_run_id: $activation_owner_run_id,
      activation_marker_sha256: $activation_marker_sha256,
      convergence_run_id: $convergence_run_id,
      owner_asset_name: $owner_asset_name
    }' > "${owner_record}"
  owner_asset_sha256="$(sha256sum "${owner_record}" | awk '{print $1}')"

  # Store the owner record inside the lease commit itself. The exact lock SHA
  # therefore addresses both the active lease and immutable evidence for its
  # owner without trying to append an asset to an already-published release.
  owner_blob="$(git hash-object -w "${owner_record}")"
  GIT_INDEX_FILE="${lock_index}" git read-tree HEAD
  GIT_INDEX_FILE="${lock_index}" git update-index --add --cacheinfo \
    "100644,${owner_blob},${owner_asset_name}"
  lock_tree="$(GIT_INDEX_FILE="${lock_index}" git write-tree)"
  lock_commit="$(printf '%s\n' "${lock_message}" | git commit-tree "${lock_tree}" -p HEAD)"
  local deadline=$((SECONDS + timeout_seconds))

  while (( SECONDS < deadline )); do
    # The unique evidence tag retains the exact commit after the active lock ref
    # is released. Atomic creation prevents either ref from existing alone.
    if git push --atomic origin \
         "${lock_commit}:${LOCK_REF}" \
         "${lock_commit}:${owner_ref}" >/dev/null 2>&1; then
      {
        echo "lock_sha=${lock_commit}"
        echo "owner_asset_name=${owner_asset_name}"
        echo "owner_asset_sha256=${owner_asset_sha256}"
      } >> "${GITHUB_OUTPUT}"
      rm -rf -- "${owner_dir}"
      rm -f -- "${lock_index}"
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

  rm -rf -- "${owner_dir}"
  rm -f -- "${lock_index}"
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
