#!/usr/bin/env bash
# Pulse Cloud — Controlled Image Rollout
# Usage: ./rollout.sh <new-image-digest>
#
# Examples:
#   ./rollout.sh ghcr.io/rcourtman/pulse@sha256:...
#   ./rollout.sh sha256:...   (uses CP_PULSE_IMAGE from /opt/pulse-cloud/.env as the repo)

set -euo pipefail
IFS=$'\n\t'

DOCKER_NETWORK="${PULSE_DOCKER_NETWORK:-pulse-cloud}"
DATA_DIR="${PULSE_DATA_DIR:-/data}"
TENANTS_DIR="${PULSE_TENANTS_DIR:-${DATA_DIR}/tenants}"
SNAPSHOT_ROOT="${PULSE_SNAPSHOT_ROOT:-${DATA_DIR}/backups/rollout}"
CONTROL_ENV="${PULSE_CONTROL_ENV:-/opt/pulse-cloud/.env}"
CONTAINER_PORT="${PULSE_TENANT_PORT:-7655}"
HEALTH_PATH="${PULSE_TENANT_HEALTH_PATH:-/api/health}"
STOP_TIMEOUT_SECS="${PULSE_STOP_TIMEOUT_SECS:-30}"

PAUSE_AFTER_CANARY="${PULSE_PAUSE_AFTER_CANARY:-1}"

log() {
  echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"
}

die() {
  echo "error: $*" >&2
  exit 1
}

have() { command -v "$1" >/dev/null 2>&1; }

usage() {
  cat <<EOF
Usage:
  $0 <new-image-digest>

Accepted formats:
  - Full image ref with digest: ghcr.io/rcourtman/pulse@sha256:...
  - Digest only: sha256:... (repo inferred from CP_PULSE_IMAGE in ${CONTROL_ENV})

Environment:
  PULSE_DOCKER_NETWORK          (default: ${DOCKER_NETWORK})
  PULSE_DATA_DIR                (default: ${DATA_DIR})
  PULSE_TENANTS_DIR             (default: ${TENANTS_DIR})
  PULSE_SNAPSHOT_ROOT           (default: ${SNAPSHOT_ROOT})
  PULSE_CONTROL_ENV             (default: ${CONTROL_ENV})
  PULSE_TENANT_PORT             (default: ${CONTAINER_PORT})
  PULSE_TENANT_HEALTH_PATH      (default: ${HEALTH_PATH})
  PULSE_STOP_TIMEOUT_SECS       (default: ${STOP_TIMEOUT_SECS})
  PULSE_PAUSE_AFTER_CANARY      (default: ${PAUSE_AFTER_CANARY})
EOF
}

read_env_value() {
  local file="$1"
  local key="$2"
  [[ -f "${file}" ]] || return 1
  local v
  v="$(grep -E "^${key}=" "${file}" | tail -n 1 | cut -d= -f2- || true)"
  v="${v%\"}"; v="${v#\"}"
  v="${v%\'}"; v="${v#\'}"
  echo "${v}"
}

image_repo_from_ref() {
  # Strip @sha256:... if present, then strip :tag (best-effort).
  local ref="$1"
  ref="${ref%@sha256:*}"
  # If there is a :tag, drop it (but keep : in registry host:port).
  # Heuristic: drop the last ":<something>" only if there's a "/" after it exists.
  if [[ "${ref}" == */*:* ]]; then
    ref="${ref%:*}"
  fi
  echo "${ref}"
}

resolve_new_image_ref() {
  local input="$1"
  if [[ "${input}" == *@sha256:* ]]; then
    echo "${input}"
    return 0
  fi
  if [[ "${input}" == sha256:* ]]; then
    local base
    base="$(read_env_value "${CONTROL_ENV}" "CP_PULSE_IMAGE" || true)"
    [[ -n "${base}" ]] || die "digest-only input requires CP_PULSE_IMAGE in ${CONTROL_ENV}"
    local repo
    repo="$(image_repo_from_ref "${base}")"
    echo "${repo}@${input}"
    return 0
  fi
  die "unsupported image argument: ${input} (expected <repo>@sha256:... or sha256:...)"
}

list_tenant_containers() {
  docker ps -a --filter "label=pulse.managed=true" --format '{{.Names}}' | sort
}

tenant_id_for_container() {
  local name="$1"
  docker inspect -f '{{ index .Config.Labels "pulse.tenant.id" }}' "${name}" 2>/dev/null || true
}

container_ip_on_network() {
  local name="$1"
  docker inspect -f "{{(index .NetworkSettings.Networks \"${DOCKER_NETWORK}\").IPAddress}}" "${name}" 2>/dev/null || true
}

wait_for_health() {
  local name="$1"
  local ip
  ip="$(container_ip_on_network "${name}")"
  [[ -n "${ip}" ]] || return 1

  local url="http://${ip}:${CONTAINER_PORT}${HEALTH_PATH}"
  local i
  for i in {1..30}; do
    if curl -fsS --max-time 5 "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

snapshot_tenant_dir() {
  local tenant_id="$1"
  local snap_dir="$2"

  local src="${TENANTS_DIR}/${tenant_id}"
  [[ -d "${src}" ]] || die "missing tenant data dir: ${src}"

  mkdir -p "${snap_dir}"
  rsync -a --delete --numeric-ids "${src}/" "${snap_dir}/"
}

restore_tenant_dir() {
  local tenant_id="$1"
  local snap_dir="$2"
  local dst="${TENANTS_DIR}/${tenant_id}"
  [[ -d "${snap_dir}" ]] || die "missing snapshot dir: ${snap_dir}"
  mkdir -p "${dst}"
  rsync -a --delete --numeric-ids "${snap_dir}/" "${dst}/"
}

create_container_like() {
  # Create a new container from a template container, but with a different image ref.
  # Args: <template_container_name> <new_container_name> <new_image_ref>
  local tmpl="$1"
  local new_name="$2"
  local new_image="$3"

  # jq is required here for correctness (labels/env/mounts/resources).
  local inspect_json
  inspect_json="$(docker inspect "${tmpl}")"

  local restart_policy
  restart_policy="$(echo "${inspect_json}" | jq -r '.[0].HostConfig.RestartPolicy.Name // "unless-stopped"')"

  local memory cpu_shares
  memory="$(echo "${inspect_json}" | jq -r '.[0].HostConfig.Resources.Memory // 0')"
  cpu_shares="$(echo "${inspect_json}" | jq -r '.[0].HostConfig.Resources.CPUShares // 0')"

  local networks
  networks="$(echo "${inspect_json}" | jq -r '.[0].NetworkSettings.Networks | keys[]' 2>/dev/null || true)"
  local primary_network="${DOCKER_NETWORK}"
  if [[ -n "${networks}" ]]; then
    primary_network="$(echo "${networks}" | head -n 1)"
  fi

  local args=()
  args+=(docker create --name "${new_name}")
  args+=(--restart "${restart_policy}")
  args+=(--network "${primary_network}")

  if [[ "${memory}" != "0" ]]; then
    args+=(--memory "${memory}")
  fi
  if [[ "${cpu_shares}" != "0" ]]; then
    args+=(--cpu-shares "${cpu_shares}")
  fi

  # Labels
  while IFS= read -r kv; do
    [[ -n "${kv}" ]] || continue
    args+=(--label "${kv}")
  done < <(echo "${inspect_json}" | jq -r '.[0].Config.Labels // {} | to_entries[] | "\(.key)=\(.value)"')

  # Environment
  while IFS= read -r e; do
    [[ -n "${e}" ]] || continue
    args+=(--env "${e}")
  done < <(echo "${inspect_json}" | jq -r '.[0].Config.Env[]?')

  # Mounts (bind + volume)
  while IFS= read -r m; do
    [[ -n "${m}" ]] || continue
    args+=(--mount "${m}")
  done < <(
    echo "${inspect_json}" | jq -r '
      .[0].Mounts[]? |
      if .Type == "bind" then
        "type=bind,src=\(.Source),dst=\(.Destination),readonly=\((.RW|not)|tostring)"
      elif .Type == "volume" then
        # Volume name is in .Name; Destination is the mountpoint.
        "type=volume,src=\(.Name),dst=\(.Destination),readonly=\((.RW|not)|tostring)"
      else
        empty
      end
    '
  )

  # Use image defaults for entrypoint/cmd. The tenant containers are created by the control plane
  # without explicit Cmd/Entrypoint overrides; attempting to round-trip those safely in bash is
  # error-prone and not required for this rollout flow.
  args+=("${new_image}")

  "${args[@]}" >/dev/null

  # Connect to any additional networks the template was attached to.
  if [[ -n "${networks}" ]]; then
    while IFS= read -r n; do
      [[ -n "${n}" ]] || continue
      [[ "${n}" == "${primary_network}" ]] && continue
      docker network connect "${n}" "${new_name}" >/dev/null 2>&1 || true
    done <<<"${networks}"
  fi
}

main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 2
  fi

  have docker || die "missing docker"
  have jq || die "missing jq (install it; setup.sh installs it)"
  have rsync || die "missing rsync (install it; setup.sh installs it)"
  have curl || die "missing curl"

  local new_image
  new_image="$(resolve_new_image_ref "$1")"

  log "pre-pulling image: ${new_image}"
  docker pull "${new_image}" >/dev/null

  local run_id
  run_id="$(date -u +'%Y%m%dT%H%M%SZ')"
  local run_dir="${SNAPSHOT_ROOT}/${run_id}"
  mkdir -p "${run_dir}"
  chmod 0700 "${run_dir}" || true

  local containers
  containers="$(list_tenant_containers)"
  [[ -n "${containers}" ]] || die "no tenant containers found (label pulse.managed=true)"

  log "starting rollout (network=${DOCKER_NETWORK}, tenants_dir=${TENANTS_DIR})"
  log "canary: first tenant in lexicographic order; pause_after_canary=${PAUSE_AFTER_CANARY}"

  local idx=0
  while IFS= read -r name; do
    [[ -n "${name}" ]] || continue
    idx="$((idx + 1))"

    local tenant_id
    tenant_id="$(tenant_id_for_container "${name}")"
    [[ -n "${tenant_id}" ]] || die "failed to resolve tenant id for container: ${name}"

    local data_dir="${TENANTS_DIR}/${tenant_id}"
    local snap_dir="${run_dir}/${tenant_id}"
    local old_name="${name}.pre-rollout-${run_id}"

    log "tenant ${idx}: ${tenant_id} (container=${name})"

    log "stopping ${name} (timeout=${STOP_TIMEOUT_SECS}s)"
    docker stop -t "${STOP_TIMEOUT_SECS}" "${name}" >/dev/null 2>&1 || true

    log "snapshotting ${data_dir} -> ${snap_dir}"
    snapshot_tenant_dir "${tenant_id}" "${snap_dir}"

    log "renaming old container: ${name} -> ${old_name}"
    docker rename "${name}" "${old_name}"

    log "creating new container with image ${new_image}"
    if ! create_container_like "${old_name}" "${name}" "${new_image}"; then
      log "create failed; rolling back ${tenant_id}"
      docker rename "${old_name}" "${name}" || true
      docker start "${name}" >/dev/null 2>&1 || true
      die "failed to create new container for ${tenant_id}"
    fi

    log "starting new container: ${name}"
    docker start "${name}" >/dev/null

    log "health check: ${tenant_id}"
    if ! wait_for_health "${name}"; then
      log "health check failed; rolling back ${tenant_id}"

      docker stop -t "${STOP_TIMEOUT_SECS}" "${name}" >/dev/null 2>&1 || true
      docker rm "${name}" >/dev/null 2>&1 || true

      log "restoring snapshot -> ${data_dir}"
      restore_tenant_dir "${tenant_id}" "${snap_dir}"

      log "restoring old container name and starting it"
      docker rename "${old_name}" "${name}" >/dev/null
      docker start "${name}" >/dev/null 2>&1 || true

      die "rollout halted on tenant ${tenant_id}"
    fi

    log "tenant ${tenant_id}: OK"

    # Canary pause: after first tenant succeeds, require explicit confirmation (TTY only).
    if [[ "${idx}" -eq 1 && "${PAUSE_AFTER_CANARY}" != "0" && -t 0 ]]; then
      read -r -p "Canary succeeded (${tenant_id}). Continue rollout? [y/N] " ans
      if [[ "${ans}" != "y" && "${ans}" != "Y" ]]; then
        die "rollout stopped after canary by operator"
      fi
    fi

    # Old container is retained for investigation; remove it only after full success.
  done <<<"${containers}"

  log "rollout complete. snapshots: ${run_dir}"
  log "note: pre-rollout containers were retained with suffix .pre-rollout-${run_id}"
}

main "$@"
