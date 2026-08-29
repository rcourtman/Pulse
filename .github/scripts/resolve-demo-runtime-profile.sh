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
# exact runtime implements both scaling boundaries and carries an explicit
# startup-readiness marker earned by the complete governed profile. Source
# structure alone is not proof that health becomes responsive at that scale.
if git grep -q 'mockEagerHistoryPVEGuestLimit' "${TAG_REF}" -- internal/monitoring/mock_metrics_history.go && \
   git grep -q 'UpdateMetricCohort' "${TAG_REF}" -- internal/mock/integration.go && \
   git grep -q 'mockLargeEstateStartupReady' "${TAG_REF}" -- internal/monitoring/mock_metrics_history.go; then
  PROFILE="large-estate"
  MOCK_NODES=50
  MOCK_VMS_PER_NODE=10
  MOCK_LXCS_PER_NODE=8
  MOCK_DOCKER_HOSTS=5
  MOCK_DOCKER_CONTAINERS=14
  MOCK_GENERIC_HOSTS=4
  MOCK_K8S_CLUSTERS=3
  MOCK_K8S_NODES=5
  MOCK_K8S_PODS=40
  MOCK_K8S_DEPLOYMENTS=14
  MOCK_SEED_DURATION=48h
  MOCK_UPDATE_INTERVAL=2s
else
  PROFILE="legacy-bounded"
  # Older runtimes update every fixture on every tick and eagerly retain every
  # history series. Keep their public-demo payload deliberately compact; the
  # tagged workflow's former 32-node default still produced a 4 MB state
  # bootstrap and could leave real browsers parked on the loading shell.
  MOCK_NODES=8
  MOCK_VMS_PER_NODE=6
  MOCK_LXCS_PER_NODE=4
  MOCK_DOCKER_HOSTS=2
  MOCK_DOCKER_CONTAINERS=8
  MOCK_GENERIC_HOSTS=2
  MOCK_K8S_CLUSTERS=1
  MOCK_K8S_NODES=3
  MOCK_K8S_PODS=12
  MOCK_K8S_DEPLOYMENTS=4
  MOCK_SEED_DURATION=2h
  MOCK_UPDATE_INTERVAL=15s
fi

printf 'profile=%s\n' "$PROFILE"
printf 'mock_nodes=%s\n' "$MOCK_NODES"
printf 'mock_vms_per_node=%s\n' "$MOCK_VMS_PER_NODE"
printf 'mock_lxcs_per_node=%s\n' "$MOCK_LXCS_PER_NODE"
printf 'mock_docker_hosts=%s\n' "$MOCK_DOCKER_HOSTS"
printf 'mock_docker_containers=%s\n' "$MOCK_DOCKER_CONTAINERS"
printf 'mock_generic_hosts=%s\n' "$MOCK_GENERIC_HOSTS"
printf 'mock_k8s_clusters=%s\n' "$MOCK_K8S_CLUSTERS"
printf 'mock_k8s_nodes=%s\n' "$MOCK_K8S_NODES"
printf 'mock_k8s_pods=%s\n' "$MOCK_K8S_PODS"
printf 'mock_k8s_deployments=%s\n' "$MOCK_K8S_DEPLOYMENTS"
printf 'mock_seed_duration=%s\n' "$MOCK_SEED_DURATION"
printf 'mock_update_interval=%s\n' "$MOCK_UPDATE_INTERVAL"
