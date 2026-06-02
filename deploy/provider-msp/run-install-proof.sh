#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  ./run-install-proof.sh --account-name "Example MSP" --owner-email owner@example.com [options] [-- extra install-proof flags]

Runs the provider-hosted MSP compose install proof on a Docker host:
  1. validates .env and docker-compose.yml
  2. optionally pulls the pinned Traefik and control-plane images
  3. starts Traefik so proof workspaces can attach isolated tenant networks
  4. runs provider-msp install-proof through the compose control-plane service
  5. starts the provider stack
  6. runs provider-msp status as a final operator check

Options:
  --account-name NAME          Provider MSP account display name
  --owner-email EMAIL          Provider owner email address
  --workspace-count COUNT      Proof client workspace count (default: 2)
  --install-type pve|pbs       Agent install command type to prove (default: pve)
  --target-path PATH           Tenant-local handoff target path
  --skip-compose-pull          Do not run docker compose pull before proof
  --skip-runtime-image-pull    Pass --skip-image-pull to install-proof
  --no-start                   Do not start the long-running provider stack after proof
  -h, --help                   Show this help

Environment equivalents:
  PROVIDER_MSP_ACCOUNT_NAME
  PROVIDER_MSP_OWNER_EMAIL
  PROVIDER_MSP_PROOF_WORKSPACE_COUNT
  PROVIDER_MSP_PROOF_INSTALL_TYPE
  PROVIDER_MSP_PROOF_TARGET_PATH
  PROVIDER_MSP_SKIP_COMPOSE_PULL=1
  PROVIDER_MSP_SKIP_RUNTIME_IMAGE_PULL=1
  PROVIDER_MSP_START_CONTROL_PLANE_AFTER_PROOF=0
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir"

account_name="${PROVIDER_MSP_ACCOUNT_NAME:-}"
owner_email="${PROVIDER_MSP_OWNER_EMAIL:-}"
workspace_count="${PROVIDER_MSP_PROOF_WORKSPACE_COUNT:-2}"
install_type="${PROVIDER_MSP_PROOF_INSTALL_TYPE:-pve}"
target_path="${PROVIDER_MSP_PROOF_TARGET_PATH:-/settings/infrastructure?add=linux-host}"
skip_compose_pull="${PROVIDER_MSP_SKIP_COMPOSE_PULL:-0}"
skip_runtime_image_pull="${PROVIDER_MSP_SKIP_RUNTIME_IMAGE_PULL:-0}"
start_after_proof="${PROVIDER_MSP_START_CONTROL_PLANE_AFTER_PROOF:-1}"
extra_install_args=()

while (($# > 0)); do
  case "$1" in
    --account-name)
      account_name="${2:-}"
      shift 2
      ;;
    --owner-email)
      owner_email="${2:-}"
      shift 2
      ;;
    --workspace-count)
      workspace_count="${2:-}"
      shift 2
      ;;
    --install-type)
      install_type="${2:-}"
      shift 2
      ;;
    --target-path)
      target_path="${2:-}"
      shift 2
      ;;
    --skip-compose-pull)
      skip_compose_pull=1
      shift
      ;;
    --skip-runtime-image-pull)
      skip_runtime_image_pull=1
      shift
      ;;
    --no-start)
      start_after_proof=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      extra_install_args+=("$@")
      break
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$account_name" ]]; then
  echo "--account-name or PROVIDER_MSP_ACCOUNT_NAME is required" >&2
  exit 2
fi
if [[ -z "$owner_email" ]]; then
  echo "--owner-email or PROVIDER_MSP_OWNER_EMAIL is required" >&2
  exit 2
fi
if [[ ! -f .env ]]; then
  echo "deploy/provider-msp/.env is required; copy .env.example to .env and fill in the provider values" >&2
  exit 2
fi

docker compose version >/dev/null
docker version >/dev/null
docker compose config --quiet

if [[ "$skip_compose_pull" != "1" ]]; then
  docker compose pull traefik control-plane
fi

docker compose up -d traefik

install_args=(
  provider-msp install-proof
  --account-name "$account_name"
  --owner-email "$owner_email"
  --workspace-count "$workspace_count"
  --install-type "$install_type"
  --target-path "$target_path"
)
if [[ "$skip_runtime_image_pull" == "1" ]]; then
  install_args+=(--skip-image-pull)
fi
if ((${#extra_install_args[@]} > 0)); then
  install_args+=("${extra_install_args[@]}")
fi

docker compose run --rm --no-deps control-plane "${install_args[@]}"

if [[ "$start_after_proof" != "0" ]]; then
  docker compose up -d traefik control-plane
  docker compose run --rm --no-deps control-plane provider-msp status
  docker compose ps
fi
