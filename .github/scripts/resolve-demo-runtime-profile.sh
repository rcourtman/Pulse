#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: resolve-demo-runtime-profile.sh TAG_REF" >&2
  exit 2
fi

TAG_REF="$1"
git rev-parse -q --verify "${TAG_REF}^{commit}" >/dev/null || {
  echo "demo runtime profile ref does not resolve to a commit: ${TAG_REF}" >&2
  exit 1
}

# Reusable workflows execute from the caller revision, while the installed
# binary comes from TAG_REF. The large-estate profile is safe only when that
# exact runtime implements both required scaling boundaries.
if git grep -q 'mockEagerHistoryPVEGuestLimit' "${TAG_REF}" -- internal/monitoring/mock_metrics_history.go && \
   git grep -q 'UpdateMetricCohort' "${TAG_REF}" -- internal/mock/integration.go; then
  PROFILE="large-estate"
  MOCK_NODES=50
  MOCK_SEED_DURATION=48h
  MOCK_UPDATE_INTERVAL=2s
else
  PROFILE="legacy-bounded"
  MOCK_NODES="$({
    git show "${TAG_REF}:.github/workflows/update-demo-server.yml" 2>/dev/null || true
  } | sed -n -E 's/^[[:space:]]*set_env_value PULSE_MOCK_NODES ([0-9]+)[[:space:]]*$/\1/p' | tail -1)"
  if [[ ! "${MOCK_NODES}" =~ ^[1-9][0-9]*$ ]]; then
    MOCK_NODES=32
  fi
  MOCK_SEED_DURATION=6h
  MOCK_UPDATE_INTERVAL=10s
fi

printf 'profile=%s\n' "$PROFILE"
printf 'mock_nodes=%s\n' "$MOCK_NODES"
printf 'mock_seed_duration=%s\n' "$MOCK_SEED_DURATION"
printf 'mock_update_interval=%s\n' "$MOCK_UPDATE_INTERVAL"
