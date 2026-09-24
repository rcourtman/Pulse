#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

usage() {
  cat <<'USAGE'
Usage:
  ./upgrade.sh [options]

To move to a new release, download and verify its provider bundle exactly as
for a fresh install, then run ./upgrade.sh from the extracted bundle directory
(sudo -E bash ./upgrade.sh). It installs the bundle's files into the existing
install (PULSE_PROVIDER_MSP_INSTALL_DIR, default /opt/pulse-provider-msp),
keeps .env and a pre-upgrade copy of it, re-pins CONTROL_PLANE_IMAGE and
CP_PULSE_IMAGE to that release, and then runs the flow below. Run from the
install directory, it applies the image pins already in .env.

Runs the provider-hosted MSP pre-upgrade and upgrade flow:
  1. validates .env and docker-compose.yml
  2. checks provider status and install preflight
  3. creates and verifies a fresh provider MSP backup
  4. dry-runs restore into a separate target data directory
  5. pulls and starts the provider Traefik, Docker socket proxy, and control-plane services
  6. prints the tenant runtime rollout plan for CP_PULSE_IMAGE
  7. optionally rolls all tenant runtimes onto CP_PULSE_IMAGE

Options:
  --dry-run                    Print the non-mutating upgrade plan only
  --rollout-tenants            Execute tenant-runtime rollout --all after provider services are updated
  --prune-previous             Remove preserved pre-rollout tenant containers after successful tenant rollout
  --skip-compose-pull          Do not run docker compose pull for provider services
  --skip-runtime-image-pull    Pass --skip-image-pull to provider-msp preflight
  --backup-output PATH         Write the fresh backup archive to PATH
  --restore-target DIR         Restore dry-run target data dir (default: <data-dir>/upgrade-restore-drill)
  --run-id ID                  Operator-visible tenant reconcile run id
  --health-timeout DURATION    Tenant rollout health timeout (default: 90s)
  --keep-image-pins            From a bundle: install its files but keep the image pins in .env
  -h, --help                   Show this help

Environment equivalents:
  PROVIDER_MSP_UPGRADE_DRY_RUN=1
  PROVIDER_MSP_UPGRADE_ROLLOUT_TENANTS=1
  PROVIDER_MSP_UPGRADE_PRUNE_PREVIOUS=1
  PROVIDER_MSP_SKIP_COMPOSE_PULL=1
  PROVIDER_MSP_SKIP_RUNTIME_IMAGE_PULL=1
  PROVIDER_MSP_UPGRADE_BACKUP_OUTPUT=/data/backups/provider-msp/pre-upgrade.tar.gz
  PROVIDER_MSP_UPGRADE_RESTORE_TARGET=/data/upgrade-restore-drill
  PROVIDER_MSP_UPGRADE_RUN_ID=provider-msp-upgrade-20260602T120000Z
  PROVIDER_MSP_UPGRADE_HEALTH_TIMEOUT=90s
USAGE
}

die() {
  echo "error: $*" >&2
  exit 1
}

truthy() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    true|1|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

env_value() {
  local key="$1"
  local env_path="${2:-.env}"
  local value
  if [[ ! -f "${env_path}" ]]; then
    return 0
  fi
  value="$(grep -E "^${key}=" "${env_path}" | tail -n 1 | cut -d= -f2- || true)"
  value="${value%\"}"; value="${value#\"}"
  value="${value%\'}"; value="${value#\'}"
  echo "${value}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

extract_field() {
  local key="$1"
  awk -F= -v key="${key}" '$1 == key { print substr($0, length($1) + 2); exit }'
}

run_control() {
  docker compose run --rm --no-deps control-plane "$@"
}

set_env_value() {
  local key="$1" value="$2" env_path="$3" tmp
  tmp="$(mktemp)"
  if grep -q -E "^${key}=" "${env_path}"; then
    awk -v key="${key}" -v value="${value}" 'BEGIN{done=0} $0 ~ "^" key "=" && done==0 { print key "=" value; done=1; next } { print }' "${env_path}" >"${tmp}"
  else
    cat "${env_path}" >"${tmp}"
    printf '%s=%s\n' "${key}" "${value}" >>"${tmp}"
  fi
  cat "${tmp}" >"${env_path}"
  rm -f "${tmp}"
}

# resolve_image_digest turns a tag into an immutable digest ref, as setup.sh
# does, reading the registry without pulling the image.
resolve_image_digest() {
  local ref="$1" manifest_json digest
  manifest_json="$(docker buildx imagetools inspect "${ref}" --format '{{json .Manifest}}' 2>/dev/null || true)"
  digest="$(printf '%s' "${manifest_json}" | jq -r 'if type == "object" then .digest // empty else empty end' 2>/dev/null || true)"
  if [[ "${digest}" != sha256:* ]]; then
    digest="$(docker buildx imagetools inspect "${ref}" 2>/dev/null | awk '$1 == "Digest:" {print $2; exit}' || true)"
  fi
  [[ "${digest}" == sha256:* ]] || return 1
  printf '%s@%s\n' "${ref%:*}" "${digest}"
}

# The files setup.sh installs from a bundle. A bundle upgrade installs the same
# set, so an upgraded install matches a fresh one.
bundle_install_files=(docker-compose.yml traefik.yml traefik-dynamic.yml .env.example run-install-proof.sh upgrade.sh)

# A bundle upgrade brings an existing install up to the release bundle this
# script was extracted with. Setup resolves the image pins in .env to digests
# once, so an upgrade used to re-pull only the release the provider first
# installed; moving on meant finding four digests by hand. Nothing on disk
# changes until the platform has been checked and backed up: those steps run
# the new release's control plane as a one-off through a shell override of the
# pins (compose prefers the shell over .env), which also carries fixes to the
# checks themselves. Only then are the bundle files installed and .env
# re-pinned, just before the new images start.
bundle_dir=""
bundle_version=""
bundle_pin_keys=()
bundle_pin_values=()

plan_bundle_upgrade() {
  local install_dir="$1" env_path key target current resolved
  bundle_dir="$2"
  bundle_version="$(tr -d '[:space:]' <"${bundle_dir}/VERSION")"
  env_path="${install_dir}/.env"
  [[ -f "${env_path}" ]] || die "no provider MSP install at ${install_dir} (missing .env); run ./setup.sh for a fresh install"
  [[ -w "${install_dir}" && -w "${env_path}" ]] || die "cannot write ${install_dir}; run: sudo -E bash ./upgrade.sh"
  echo "provider_msp_upgrade_bundle_version=${bundle_version}"
  echo "provider_msp_upgrade_install_dir=${install_dir}"
  if truthy "${keep_image_pins}"; then
    echo "provider_msp_upgrade_image_pins=kept"
    return 0
  fi
  for key in CONTROL_PLANE_IMAGE CP_PULSE_IMAGE; do
    target="$(env_value "${key}" "${bundle_dir}/.env.example")"
    [[ -n "${target}" ]] || die "bundle .env.example has no ${key}; download the release bundle, not the source tree"
    current="$(env_value "${key}" "${env_path}")"
    if [[ "${target}" == *@sha256:* ]]; then
      resolved="${target}"
    elif ! resolved="$(resolve_image_digest "${target}")"; then
      die "could not resolve a digest for ${target}; check this host can reach the registry"
    fi
    echo "provider_msp_upgrade_image_pin ${key} current=${current} target=${resolved}"
    bundle_pin_keys+=("${key}")
    bundle_pin_values+=("${resolved}")
    export "${key}=${resolved}"
  done
}

apply_bundle_upgrade() {
  local install_dir="$1" env_path stamp file i
  env_path="${install_dir}/.env"
  stamp="$(date -u +'%Y%m%dT%H%M%SZ')"
  cp -p "${env_path}" "${env_path}.pre-upgrade-${stamp}"
  echo "provider_msp_upgrade_env_backup=${env_path}.pre-upgrade-${stamp}"
  for file in "${bundle_install_files[@]}"; do
    [[ -f "${bundle_dir}/${file}" ]] || die "bundle is missing ${file}"
    case "${file}" in
      *.sh) install -m 0755 "${bundle_dir}/${file}" "${install_dir}/${file}" ;;
      *) install -m 0644 "${bundle_dir}/${file}" "${install_dir}/${file}" ;;
    esac
  done
  for ((i = 0; i < ${#bundle_pin_keys[@]}; i++)); do
    set_env_value "${bundle_pin_keys[$i]}" "${bundle_pin_values[$i]}" "${env_path}"
  done
  echo "provider_msp_upgrade_bundle_installed=true"
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${script_dir}"

dry_run="${PROVIDER_MSP_UPGRADE_DRY_RUN:-0}"
rollout_tenants="${PROVIDER_MSP_UPGRADE_ROLLOUT_TENANTS:-0}"
prune_previous="${PROVIDER_MSP_UPGRADE_PRUNE_PREVIOUS:-0}"
skip_compose_pull="${PROVIDER_MSP_SKIP_COMPOSE_PULL:-0}"
skip_runtime_image_pull="${PROVIDER_MSP_SKIP_RUNTIME_IMAGE_PULL:-0}"
backup_output="${PROVIDER_MSP_UPGRADE_BACKUP_OUTPUT:-}"
restore_target="${PROVIDER_MSP_UPGRADE_RESTORE_TARGET:-}"
run_id="${PROVIDER_MSP_UPGRADE_RUN_ID:-provider-msp-upgrade-$(date -u +'%Y%m%dT%H%M%SZ')}"
health_timeout="${PROVIDER_MSP_UPGRADE_HEALTH_TIMEOUT:-90s}"
keep_image_pins="${PROVIDER_MSP_UPGRADE_KEEP_IMAGE_PINS:-0}"
while (($# > 0)); do
  case "$1" in
    --dry-run)
      dry_run=1
      shift
      ;;
    --rollout-tenants)
      rollout_tenants=1
      shift
      ;;
    --prune-previous)
      prune_previous=1
      shift
      ;;
    --skip-compose-pull)
      skip_compose_pull=1
      shift
      ;;
    --skip-runtime-image-pull)
      skip_runtime_image_pull=1
      shift
      ;;
    --backup-output)
      backup_output="${2:-}"
      shift 2
      ;;
    --restore-target)
      restore_target="${2:-}"
      shift 2
      ;;
    --run-id)
      run_id="${2:-}"
      shift 2
      ;;
    --health-timeout)
      health_timeout="${2:-}"
      shift 2
      ;;
    --keep-image-pins)
      keep_image_pins=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

# Run from an extracted release bundle (it carries VERSION and no .env), work
# on the existing install and move it to that release.
install_dir="${script_dir}"
if [[ -f "${script_dir}/VERSION" && ! -f "${script_dir}/.env" ]]; then
  install_dir="${PULSE_PROVIDER_MSP_INSTALL_DIR:-/opt/pulse-provider-msp}"
  plan_bundle_upgrade "${install_dir}" "${script_dir}"
  cd "${install_dir}"
fi

if [[ ! -f .env ]]; then
  die "deploy/provider-msp/.env is required; copy .env.example to .env and fill in the provider values"
fi

provider_data_dir="$(env_value PULSE_PROVIDER_MSP_DATA_DIR .env)"
provider_data_dir="${provider_data_dir:-/data}"
tenant_runtime_image="${CP_PULSE_IMAGE:-$(env_value CP_PULSE_IMAGE .env)}"
if [[ -z "${tenant_runtime_image}" ]]; then
  die "CP_PULSE_IMAGE is required"
fi
if [[ -z "${restore_target}" ]]; then
  restore_target="${provider_data_dir%/}/upgrade-restore-drill"
fi
if [[ "${restore_target%/}" == "${provider_data_dir%/}" ]]; then
  die "--restore-target must not point at the live provider data dir (${provider_data_dir})"
fi

docker compose version >/dev/null
docker version >/dev/null
docker compose config --quiet

preflight_args=(provider-msp preflight)
if truthy "${skip_runtime_image_pull}" || truthy "${dry_run}"; then
  preflight_args+=(--skip-image-pull)
fi

echo "provider_msp_upgrade_dry_run=$(truthy "${dry_run}" && echo true || echo false)"
echo "provider_msp_upgrade_run_id=${run_id}"
echo "provider_msp_upgrade_restore_target=${restore_target}"
echo "provider_msp_upgrade_tenant_runtime_image=${tenant_runtime_image}"

# Status checks that the tenant runtime image is already on this host, and runs
# before anything else would pull it, so a new CP_PULSE_IMAGE failed the gate on
# every upgrade. Pulling changes nothing that is running.
if ! truthy "${skip_runtime_image_pull}"; then
  docker pull "${tenant_runtime_image}" >/dev/null
fi

run_control provider-msp status
run_control "${preflight_args[@]}"

if truthy "${dry_run}"; then
  run_control tenant-runtime rollout --all --image "${tenant_runtime_image}" --dry-run
  if [[ -n "${bundle_dir}" ]]; then
    echo "provider_msp_upgrade_bundle_installed=false"
  fi
  echo "tenant_runtime_rollout_applied=false"
  echo "provider_msp_upgrade_plan_ok=true"
  exit 0
fi

backup_args=(provider-msp backup create)
if [[ -n "${backup_output}" ]]; then
  backup_args+=(--output "${backup_output}")
fi

backup_output_text="$(run_control "${backup_args[@]}")"
printf '%s\n' "${backup_output_text}"
archive_path="$(printf '%s\n' "${backup_output_text}" | extract_field archive_path)"
if [[ -z "${archive_path}" ]]; then
  die "provider-msp backup create did not print archive_path"
fi

run_control provider-msp backup verify "${archive_path}"
run_control provider-msp backup restore "${archive_path}" --target-data-dir "${restore_target}" --dry-run
run_control provider-msp status --require-backup

# The platform is checked and backed up; now install the bundle and re-pin.
if [[ -n "${bundle_dir}" ]]; then
  apply_bundle_upgrade "${install_dir}"
fi

if ! truthy "${skip_compose_pull}"; then
  docker compose pull traefik docker-socket-proxy control-plane
fi
docker compose up -d traefik docker-socket-proxy control-plane
run_control provider-msp status --require-backup
run_control tenant-runtime rollout --all --image "${tenant_runtime_image}" --dry-run

if truthy "${rollout_tenants}"; then
  reconcile_args=(
    tenant-runtime rollout
    --all
    --image "${tenant_runtime_image}"
    --run-id "${run_id}"
    --health-timeout "${health_timeout}"
  )
  if truthy "${prune_previous}"; then
    reconcile_args+=(--prune-previous)
  fi
  run_control "${reconcile_args[@]}"
  run_control provider-msp status --require-backup
  echo "tenant_runtime_rollout_applied=true"
else
  echo "tenant_runtime_rollout_applied=false"
  echo "tenant_runtime_rollout_next_command=docker compose run --rm --no-deps control-plane tenant-runtime rollout --all --image ${tenant_runtime_image} --run-id ${run_id} --health-timeout ${health_timeout}"
fi

docker compose ps

echo "backup_path=${archive_path}"
echo "restore_target_data_dir=${restore_target}"
echo "provider_msp_upgrade_ok=true"
