#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 15 ]; then
  echo "usage: recover-demo-runtime.sh EXPECTED_HOSTNAME LOCAL_BASE_URL EXPECTED_VERSION MOCK_NODES MOCK_VMS_PER_NODE MOCK_LXCS_PER_NODE MOCK_DOCKER_HOSTS MOCK_DOCKER_CONTAINERS MOCK_GENERIC_HOSTS MOCK_K8S_CLUSTERS MOCK_K8S_NODES MOCK_K8S_PODS MOCK_K8S_DEPLOYMENTS MOCK_SEED_DURATION MOCK_UPDATE_INTERVAL" >&2
  exit 2
fi

EXPECTED_HOSTNAME="$1"
LOCAL_BASE_URL="${2%/}"
EXPECTED_VERSION="${3#v}"
MOCK_NODES="$4"
MOCK_VMS_PER_NODE="$5"
MOCK_LXCS_PER_NODE="$6"
MOCK_DOCKER_HOSTS="$7"
MOCK_DOCKER_CONTAINERS="$8"
MOCK_GENERIC_HOSTS="$9"
MOCK_K8S_CLUSTERS="${10}"
MOCK_K8S_NODES="${11}"
MOCK_K8S_PODS="${12}"
MOCK_K8S_DEPLOYMENTS="${13}"
MOCK_SEED_DURATION="${14}"
MOCK_UPDATE_INTERVAL="${15}"
SERVICE_NAME="pulse"
RELAY_SERVICE_NAME="pulse-relay"
EXPECTED_BINARY="/opt/pulse/bin/pulse"
EXPECTED_UNIT="/etc/systemd/system/pulse.service"
MUTATED=false
BACKUP_DIR=""

for fixture_count in \
  "$MOCK_NODES" \
  "$MOCK_VMS_PER_NODE" \
  "$MOCK_LXCS_PER_NODE" \
  "$MOCK_DOCKER_HOSTS" \
  "$MOCK_DOCKER_CONTAINERS" \
  "$MOCK_GENERIC_HOSTS" \
  "$MOCK_K8S_CLUSTERS" \
  "$MOCK_K8S_NODES" \
  "$MOCK_K8S_PODS" \
  "$MOCK_K8S_DEPLOYMENTS"; do
  [[ "$fixture_count" =~ ^[1-9][0-9]*$ ]]
done
[[ "$MOCK_SEED_DURATION" =~ ^[1-9][0-9]*[smhd]$ ]]
[[ "$MOCK_UPDATE_INTERVAL" =~ ^[1-9][0-9]*[smh]$ ]]

log() {
  printf '[demo-recovery] %s\n' "$*" >&2
}

unit_hash() {
  sudo sha256sum "$EXPECTED_UNIT" | awk '{print $1}'
}

binary_hash() {
  sudo sha256sum "$EXPECTED_BINARY" | awk '{print $1}'
}

dropin_manifest_hash() {
  local paths path
  paths="$(systemctl show "$SERVICE_NAME" --property=DropInPaths --value)"
  if [ -z "$paths" ]; then
    printf '%s' 'none' | sha256sum | awk '{print $1}'
    return
  fi
  for path in $paths; do
    sudo test -f "$path"
    printf '%s\t' "$path"
    sudo sha256sum "$path" | awk '{print $1}'
  done | sort | sha256sum | awk '{print $1}'
}

runtime_config_hash() {
  local path
  for path in /etc/pulse/.env /etc/pulse/billing.json; do
    sudo test -f "$path"
    printf '%s\t' "$path"
    sudo sha256sum "$path" | awk '{print $1}'
  done | sha256sum | awk '{print $1}'
}

set_env_value() {
  local key="$1"
  local value="$2"
  local env_file="/etc/pulse/.env"
  if sudo grep -Eq "^[[:space:]]*${key}=" "$env_file"; then
    sudo sed -i "s|^[[:space:]]*${key}=.*|${key}=${value}|" "$env_file"
  else
    printf '\n%s=%s\n' "$key" "$value" | sudo tee -a "$env_file" >/dev/null
  fi
}

runtime_profile_matches() {
  sudo grep -Fxq "PULSE_MOCK_NODES=${MOCK_NODES}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_VMS_PER_NODE=${MOCK_VMS_PER_NODE}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_LXCS_PER_NODE=${MOCK_LXCS_PER_NODE}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_DOCKER_HOSTS=${MOCK_DOCKER_HOSTS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_DOCKER_CONTAINERS=${MOCK_DOCKER_CONTAINERS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_GENERIC_HOSTS=${MOCK_GENERIC_HOSTS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_K8S_CLUSTERS=${MOCK_K8S_CLUSTERS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_K8S_NODES=${MOCK_K8S_NODES}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_K8S_PODS=${MOCK_K8S_PODS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_K8S_DEPLOYMENTS=${MOCK_K8S_DEPLOYMENTS}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_SEED_METRICS_STORE=false" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_TRENDS_SEED_DURATION=${MOCK_SEED_DURATION}" /etc/pulse/.env &&
    sudo grep -Fxq "PULSE_MOCK_UPDATE_INTERVAL=${MOCK_UPDATE_INTERVAL}" /etc/pulse/.env
}

cleanup_config_backup() {
  [ -n "$BACKUP_DIR" ] || return 0
  sudo unlink "$BACKUP_DIR/runtime.env" 2>/dev/null || true
  sudo unlink "$BACKUP_DIR/billing.json" 2>/dev/null || true
  rmdir "$BACKUP_DIR" 2>/dev/null || true
  BACKUP_DIR=""
}

restore_runtime_config() {
  [ -n "$BACKUP_DIR" ] || return 0
  sudo cp --archive "$BACKUP_DIR/runtime.env" /etc/pulse/.env
  sudo cp --archive "$BACKUP_DIR/billing.json" /etc/pulse/billing.json
}

local_version() {
  curl --connect-timeout 3 --max-time 8 -fsS \
    "${LOCAL_BASE_URL}/api/version" | jq -er '.version'
}

local_healthy() {
  curl --connect-timeout 3 --max-time 8 -fsS \
    "${LOCAL_BASE_URL}/api/health" >/dev/null && local_version >/dev/null
}

emit_evidence() {
  local status="$1"
  local before_pid="$2"
  local after_pid="$3"
  local before_relay_pid="$4"
  local after_relay_pid="$5"
  local before_binary_sha="$6"
  local after_binary_sha="$7"
  local before_unit_sha="$8"
  local after_unit_sha="$9"
  local before_dropins_sha="${10}"
  local after_dropins_sha="${11}"
  local version="${12}"
  local before_config_sha="${13}"
  local after_config_sha="${14}"
  jq -n \
    --arg status "$status" \
    --argjson mutated "$MUTATED" \
    --arg hostname "$(hostname)" \
    --arg service "$SERVICE_NAME" \
    --arg before_pid "$before_pid" \
    --arg after_pid "$after_pid" \
    --arg relay_service "$RELAY_SERVICE_NAME" \
    --arg before_relay_pid "$before_relay_pid" \
    --arg after_relay_pid "$after_relay_pid" \
    --arg binary "$EXPECTED_BINARY" \
    --arg before_binary_sha256 "$before_binary_sha" \
    --arg after_binary_sha256 "$after_binary_sha" \
    --arg unit "$EXPECTED_UNIT" \
    --arg before_unit_sha256 "$before_unit_sha" \
    --arg after_unit_sha256 "$after_unit_sha" \
    --arg before_dropins_sha256 "$before_dropins_sha" \
    --arg after_dropins_sha256 "$after_dropins_sha" \
    --arg before_runtime_config_sha256 "$before_config_sha" \
    --arg after_runtime_config_sha256 "$after_config_sha" \
    --arg version "$version" \
    --arg expected_version "$EXPECTED_VERSION" \
    --arg mock_nodes "$MOCK_NODES" \
    --arg mock_vms_per_node "$MOCK_VMS_PER_NODE" \
    --arg mock_lxcs_per_node "$MOCK_LXCS_PER_NODE" \
    --arg mock_docker_hosts "$MOCK_DOCKER_HOSTS" \
    --arg mock_docker_containers "$MOCK_DOCKER_CONTAINERS" \
    --arg mock_generic_hosts "$MOCK_GENERIC_HOSTS" \
    --arg mock_k8s_clusters "$MOCK_K8S_CLUSTERS" \
    --arg mock_k8s_nodes "$MOCK_K8S_NODES" \
    --arg mock_k8s_pods "$MOCK_K8S_PODS" \
    --arg mock_k8s_deployments "$MOCK_K8S_DEPLOYMENTS" \
    --arg mock_seed_duration "$MOCK_SEED_DURATION" \
    --arg mock_update_interval "$MOCK_UPDATE_INTERVAL" \
    '{schema_version: 1, status: $status, mutated: $mutated,
      hostname: $hostname, service: $service,
      before_pid: $before_pid, after_pid: $after_pid,
      relay_service: $relay_service,
      before_relay_pid: $before_relay_pid,
      after_relay_pid: $after_relay_pid,
      binary: $binary,
      before_binary_sha256: $before_binary_sha256,
      after_binary_sha256: $after_binary_sha256,
      unit: $unit,
      before_unit_sha256: $before_unit_sha256,
      after_unit_sha256: $after_unit_sha256,
      before_dropins_sha256: $before_dropins_sha256,
      after_dropins_sha256: $after_dropins_sha256,
      before_runtime_config_sha256: $before_runtime_config_sha256,
      after_runtime_config_sha256: $after_runtime_config_sha256,
      version: $version, expected_version: $expected_version,
      runtime_profile: {mock_nodes: $mock_nodes,
        mock_vms_per_node: $mock_vms_per_node,
        mock_lxcs_per_node: $mock_lxcs_per_node,
        mock_docker_hosts: $mock_docker_hosts,
        mock_docker_containers: $mock_docker_containers,
        mock_generic_hosts: $mock_generic_hosts,
        mock_k8s_clusters: $mock_k8s_clusters,
        mock_k8s_nodes: $mock_k8s_nodes,
        mock_k8s_pods: $mock_k8s_pods,
        mock_k8s_deployments: $mock_k8s_deployments,
        mock_seed_duration: $mock_seed_duration,
        mock_update_interval: $mock_update_interval}}'
}

[ "$(hostname)" = "$EXPECTED_HOSTNAME" ] || {
  log "target hostname does not match the governed demo environment"
  exit 1
}
[ "$(systemctl show "$SERVICE_NAME" --property=FragmentPath --value)" = "$EXPECTED_UNIT" ]
[ "$(systemctl show "$SERVICE_NAME" --property=User --value)" = "pulse" ]
[ "$(systemctl show "$SERVICE_NAME" --property=Group --value)" = "pulse" ]
[ "$(systemctl is-enabled "$SERVICE_NAME")" = "enabled" ]
[ "$(systemctl is-active "$RELAY_SERVICE_NAME")" = "active" ]
systemctl show "$SERVICE_NAME" --property=ExecStart --value | grep -Fq "$EXPECTED_BINARY"
sudo test -x "$EXPECTED_BINARY"
sudo test -f "$EXPECTED_UNIT"

BEFORE_PID="$(systemctl show "$SERVICE_NAME" --property=MainPID --value)"
BEFORE_RELAY_PID="$(systemctl show "$RELAY_SERVICE_NAME" --property=MainPID --value)"
BEFORE_BINARY_SHA="$(binary_hash)"
BEFORE_UNIT_SHA="$(unit_hash)"
BEFORE_DROPINS_SHA="$(dropin_manifest_hash)"
BEFORE_CONFIG_SHA="$(runtime_config_hash)"

if local_healthy && runtime_profile_matches; then
  VERSION="$(local_version)"
  [ "${VERSION#v}" = "$EXPECTED_VERSION" ]
  emit_evidence healthy_noop "$BEFORE_PID" "$BEFORE_PID" \
    "$BEFORE_RELAY_PID" "$BEFORE_RELAY_PID" \
    "$BEFORE_BINARY_SHA" "$BEFORE_BINARY_SHA" \
    "$BEFORE_UNIT_SHA" "$BEFORE_UNIT_SHA" \
    "$BEFORE_DROPINS_SHA" "$BEFORE_DROPINS_SHA" "$VERSION" \
    "$BEFORE_CONFIG_SHA" "$BEFORE_CONFIG_SHA"
  exit 0
fi

rollback_on_error() {
  local rc=$?
  trap - ERR
  if [ "$MUTATED" = true ]; then
    log "recovery validation failed; compensating by stopping only pulse.service"
    sudo systemctl stop "$SERVICE_NAME" || true
    restore_runtime_config || true
  fi
  emit_evidence compensated_unavailable "$BEFORE_PID" \
    "$(systemctl show "$SERVICE_NAME" --property=MainPID --value || true)" \
    "$BEFORE_RELAY_PID" \
    "$(systemctl show "$RELAY_SERVICE_NAME" --property=MainPID --value || true)" \
    "$BEFORE_BINARY_SHA" "$(binary_hash || true)" \
    "$BEFORE_UNIT_SHA" "$(unit_hash || true)" \
    "$BEFORE_DROPINS_SHA" "$(dropin_manifest_hash || true)" "" \
    "$BEFORE_CONFIG_SHA" "$(runtime_config_hash || true)"
  cleanup_config_backup
  exit "$rc"
}
BACKUP_DIR="$(mktemp -d)"
sudo cp --archive /etc/pulse/.env "$BACKUP_DIR/runtime.env"
sudo cp --archive /etc/pulse/billing.json "$BACKUP_DIR/billing.json"
trap rollback_on_error ERR

log "applying the target-compatible demo runtime profile and restarting only pulse.service"
MUTATED=true
set_env_value PULSE_MOCK_NODES "$MOCK_NODES"
set_env_value PULSE_MOCK_VMS_PER_NODE "$MOCK_VMS_PER_NODE"
set_env_value PULSE_MOCK_LXCS_PER_NODE "$MOCK_LXCS_PER_NODE"
set_env_value PULSE_MOCK_DOCKER_HOSTS "$MOCK_DOCKER_HOSTS"
set_env_value PULSE_MOCK_DOCKER_CONTAINERS "$MOCK_DOCKER_CONTAINERS"
set_env_value PULSE_MOCK_GENERIC_HOSTS "$MOCK_GENERIC_HOSTS"
set_env_value PULSE_MOCK_K8S_CLUSTERS "$MOCK_K8S_CLUSTERS"
set_env_value PULSE_MOCK_K8S_NODES "$MOCK_K8S_NODES"
set_env_value PULSE_MOCK_K8S_PODS "$MOCK_K8S_PODS"
set_env_value PULSE_MOCK_K8S_DEPLOYMENTS "$MOCK_K8S_DEPLOYMENTS"
# Persistent mock-store backfill is reserved for the isolated local-dev data
# directory. The governed demo already has persistent runtime data, so a stale
# opt-in here can turn startup into an unbounded SQLite backfill.
set_env_value PULSE_MOCK_SEED_METRICS_STORE false
set_env_value PULSE_MOCK_TRENDS_SEED_DURATION "$MOCK_SEED_DURATION"
set_env_value PULSE_MOCK_UPDATE_INTERVAL "$MOCK_UPDATE_INTERVAL"
sudo chown pulse:pulse /etc/pulse/.env
sudo chmod 600 /etc/pulse/.env
sudo systemctl restart "$SERVICE_NAME"

for _ in $(seq 1 12); do
  if [ "$(systemctl is-active "$SERVICE_NAME" || true)" = "active" ] && local_healthy; then
    break
  fi
  sleep 5
done

[ "$(systemctl is-active "$SERVICE_NAME")" = "active" ]
[ "$(systemctl show "$SERVICE_NAME" --property=SubState --value)" = "running" ]
local_healthy
runtime_profile_matches

AFTER_PID="$(systemctl show "$SERVICE_NAME" --property=MainPID --value)"
AFTER_RELAY_PID="$(systemctl show "$RELAY_SERVICE_NAME" --property=MainPID --value)"
AFTER_BINARY_SHA="$(binary_hash)"
AFTER_UNIT_SHA="$(unit_hash)"
AFTER_DROPINS_SHA="$(dropin_manifest_hash)"
AFTER_CONFIG_SHA="$(runtime_config_hash)"
VERSION="$(local_version)"

[ "$AFTER_PID" -gt 0 ]
[ "$AFTER_PID" != "$BEFORE_PID" ]
[ "$AFTER_RELAY_PID" = "$BEFORE_RELAY_PID" ]
[ "$AFTER_BINARY_SHA" = "$BEFORE_BINARY_SHA" ]
[ "$AFTER_UNIT_SHA" = "$BEFORE_UNIT_SHA" ]
[ "$AFTER_DROPINS_SHA" = "$BEFORE_DROPINS_SHA" ]
[ "${VERSION#v}" = "$EXPECTED_VERSION" ]

trap - ERR
cleanup_config_backup
emit_evidence recovered "$BEFORE_PID" "$AFTER_PID" \
  "$BEFORE_RELAY_PID" "$AFTER_RELAY_PID" \
  "$BEFORE_BINARY_SHA" "$AFTER_BINARY_SHA" \
  "$BEFORE_UNIT_SHA" "$AFTER_UNIT_SHA" \
  "$BEFORE_DROPINS_SHA" "$AFTER_DROPINS_SHA" "$VERSION" \
  "$BEFORE_CONFIG_SHA" "$AFTER_CONFIG_SHA"
