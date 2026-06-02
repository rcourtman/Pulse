#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

usage() {
  cat <<'USAGE'
Usage:
  ./upgrade.sh [options]

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

if [[ ! -f .env ]]; then
  die "deploy/provider-msp/.env is required; copy .env.example to .env and fill in the provider values"
fi

provider_data_dir="$(env_value PULSE_PROVIDER_MSP_DATA_DIR .env)"
provider_data_dir="${provider_data_dir:-/data}"
tenant_runtime_image="$(env_value CP_PULSE_IMAGE .env)"
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

run_control provider-msp status
run_control "${preflight_args[@]}"

if truthy "${dry_run}"; then
  run_control tenant-runtime rollout --all --image "${tenant_runtime_image}" --dry-run
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
