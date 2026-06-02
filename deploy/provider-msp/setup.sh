#!/usr/bin/env bash
# Pulse Provider MSP - first-time host setup
# Run on a fresh Ubuntu 24.04 host as root.

set -euo pipefail
IFS=$'\n\t'

PULSE_PROVIDER_MSP_INSTALL_DIR="${PULSE_PROVIDER_MSP_INSTALL_DIR:-/opt/pulse-provider-msp}"
PULSE_PROVIDER_MSP_DATA_DIR="${PULSE_PROVIDER_MSP_DATA_DIR:-/data}"
PULSE_PROVIDER_MSP_DOCKER_NETWORK="${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}"
PULSE_PROVIDER_MSP_DOCKER_SOCKET="${PULSE_PROVIDER_MSP_DOCKER_SOCKET:-/var/run/docker.sock}"
PULSE_PROVIDER_MSP_HOST_ROOT="${PULSE_PROVIDER_MSP_HOST_ROOT:-/}"
PULSE_PROVIDER_MSP_DOCKER_DATA_DIR="${PULSE_PROVIDER_MSP_DOCKER_DATA_DIR:-/var/lib/docker}"
PULSE_PROVIDER_MSP_BUNDLE_URL="${PULSE_PROVIDER_MSP_BUNDLE_URL:-}"
PULSE_PROVIDER_MSP_EXPECT_ENV="${PULSE_PROVIDER_MSP_EXPECT_ENV:-production}"
PULSE_PROVIDER_MSP_SKIP_PULL="${PULSE_PROVIDER_MSP_SKIP_PULL:-0}"
PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF="${PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF:-auto}"
PULSE_PROVIDER_MSP_ACCOUNT_NAME="${PULSE_PROVIDER_MSP_ACCOUNT_NAME:-}"
PULSE_PROVIDER_MSP_OWNER_EMAIL="${PULSE_PROVIDER_MSP_OWNER_EMAIL:-}"

log() {
  echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"
}

die() {
  echo "error: $*" >&2
  exit 1
}

have() { command -v "$1" >/dev/null 2>&1; }

need_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    die "run as root (or: sudo -E bash setup.sh)"
  fi
}

apt_install() {
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get install -y --no-install-recommends "$@"
}

install_docker_ce() {
  if have docker && docker --version >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    log "docker and compose already installed"
    return 0
  fi

  log "installing Docker CE and compose plugin"
  apt_install ca-certificates curl gnupg lsb-release

  install -m 0755 -d /etc/apt/keyrings
  if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
  fi

  local arch codename
  arch="$(dpkg --print-architecture)"
  codename="$(. /etc/os-release && echo "${VERSION_CODENAME}")"

  cat >/etc/apt/sources.list.d/docker.list <<EOF
deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${codename} stable
EOF

  apt-get update -y
  apt-get install -y --no-install-recommends \
    docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

  systemctl enable --now docker
}

install_ops_tools() {
  log "installing ops tools"
  apt_install jq rsync sqlite3 rclone s3cmd
}

create_data_dirs() {
  local data_dir
  data_dir="$(provider_data_dir)"
  log "creating provider MSP data directories under ${data_dir}"
  install -d -m 0700 "${data_dir}"
  install -d -m 0700 "${data_dir}/tenants"
  install -d -m 0700 "${data_dir}/control-plane"
  install -d -m 0700 "${data_dir}/backups"
  install -d -m 0700 "${data_dir}/backups/provider-msp"
}

ensure_docker_network() {
  local network
  network="$(provider_docker_network)"
  log "ensuring Docker network ${network} exists"
  if ! docker network inspect "${network}" >/dev/null 2>&1; then
    docker network create "${network}" >/dev/null
  fi
}

script_dir_best_effort() {
  if [[ -n "${BASH_SOURCE[0]:-}" && -e "${BASH_SOURCE[0]}" ]]; then
    (cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
    return 0
  fi
  return 1
}

install_deploy_bundle() {
  log "installing provider MSP deploy bundle to ${PULSE_PROVIDER_MSP_INSTALL_DIR}"
  install -d -m 0755 "${PULSE_PROVIDER_MSP_INSTALL_DIR}"

  local src_dir=""
  if src_dir="$(script_dir_best_effort)"; then
    :
  else
    src_dir=""
  fi

  local required=(
    "docker-compose.yml"
    "traefik.yml"
    "traefik-dynamic.yml"
    ".env.example"
    "run-install-proof.sh"
    "upgrade.sh"
  )

  if [[ -n "${src_dir}" ]]; then
    local f
    for f in "${required[@]}"; do
      [[ -f "${src_dir}/${f}" ]] || src_dir=""
    done
  fi

  if [[ -z "${src_dir}" && -n "${PULSE_PROVIDER_MSP_BUNDLE_URL}" ]]; then
    log "deploy bundle not found locally; downloading PULSE_PROVIDER_MSP_BUNDLE_URL"
    local tmp
    tmp="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap "rm -rf \"${tmp}\"" EXIT

    curl -fsSL "${PULSE_PROVIDER_MSP_BUNDLE_URL}" -o "${tmp}/bundle.tgz"
    tar -xzf "${tmp}/bundle.tgz" -C "${tmp}"

    local cand ok f
    while IFS= read -r cand; do
      [[ -n "${cand}" ]] || continue
      local d
      d="$(dirname "${cand}")"
      ok="1"
      for f in "${required[@]}"; do
        [[ -f "${d}/${f}" ]] || ok="0"
      done
      if [[ "${ok}" == "1" ]]; then
        src_dir="${d}"
        break
      fi
    done < <(find "${tmp}" -type f -name docker-compose.yml -print 2>/dev/null || true)
  fi

  if [[ -z "${src_dir}" ]]; then
    cat >&2 <<'EOF'
error: missing deploy bundle files next to setup.sh.

This script needs these files present on disk:
  - docker-compose.yml
  - traefik.yml
  - traefik-dynamic.yml
  - .env.example
  - run-install-proof.sh
  - upgrade.sh

Run it from deploy/provider-msp/, or set PULSE_PROVIDER_MSP_BUNDLE_URL to a
tar.gz containing those files.
EOF
    exit 1
  fi

  install -m 0644 "${src_dir}/docker-compose.yml" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/docker-compose.yml"
  install -m 0644 "${src_dir}/traefik.yml" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/traefik.yml"
  install -m 0644 "${src_dir}/traefik-dynamic.yml" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/traefik-dynamic.yml"
  install -m 0644 "${src_dir}/.env.example" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env.example"
  install -m 0755 "${src_dir}/run-install-proof.sh" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/run-install-proof.sh"
  install -m 0755 "${src_dir}/upgrade.sh" "${PULSE_PROVIDER_MSP_INSTALL_DIR}/upgrade.sh"
}

env_value() {
  local key="$1"
  local env_path="${2:-${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env}"
  local value
  value="$(grep -E "^${key}=" "${env_path}" | tail -n 1 | cut -d= -f2- || true)"
  value="${value%\"}"; value="${value#\"}"
  value="${value%\'}"; value="${value#\'}"
  echo "${value}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

provider_data_dir() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local configured=""
  if [[ -f "${env_path}" ]]; then
    configured="$(env_value PULSE_PROVIDER_MSP_DATA_DIR "${env_path}")"
  fi
  echo "${configured:-${PULSE_PROVIDER_MSP_DATA_DIR}}"
}

provider_docker_network() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local configured=""
  if [[ -f "${env_path}" ]]; then
    configured="$(env_value PULSE_PROVIDER_MSP_DOCKER_NETWORK "${env_path}")"
  fi
  echo "${configured:-${PULSE_PROVIDER_MSP_DOCKER_NETWORK}}"
}

truthy() {
  case "$(echo "$1" | tr '[:upper:]' '[:lower:]')" in
    true|1|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

falsy() {
  case "$(echo "$1" | tr '[:upper:]' '[:lower:]')" in
    false|0|no|off) return 0 ;;
    *) return 1 ;;
  esac
}

ensure_env_file() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  if [[ -f "${env_path}" ]]; then
    chmod 0600 "${env_path}" || true
    return 0
  fi

  log "no ${env_path}; creating from .env.example"
  cp -n "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env.example" "${env_path}"
  chmod 0600 "${env_path}"

  cat <<EOF

Created ${env_path} from .env.example.

Edit it now and set required values:
  - DOMAIN
  - ACME_EMAIL
  - CF_DNS_API_TOKEN
  - TRAEFIK_IMAGE (digest pinned)
  - CONTROL_PLANE_IMAGE (digest pinned)
  - CP_PULSE_IMAGE (digest pinned)
  - CP_ADMIN_KEY
  - PULSE_PROVIDER_MSP_DATA_DIR
  - PULSE_PROVIDER_MSP_DOCKER_NETWORK
  - PULSE_PROVIDER_MSP_DOCKER_SOCKET
  - CP_PROVIDER_MSP_LICENSE_FILE
  - CP_TRIAL_ACTIVATION_PRIVATE_KEY

EOF

  if [[ -t 0 ]]; then
    read -r -p "Press Enter to continue after editing ${env_path}..." _
  else
    die "non-interactive run: edit ${env_path} then re-run setup.sh"
  fi
}

validate_env_file() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"

  local expected_env cp_env
  expected_env="$(echo "${PULSE_PROVIDER_MSP_EXPECT_ENV}" | tr '[:upper:]' '[:lower:]' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  case "${expected_env}" in
    production|staging) ;;
    *) die "PULSE_PROVIDER_MSP_EXPECT_ENV must be production or staging (got '${PULSE_PROVIDER_MSP_EXPECT_ENV}')" ;;
  esac

  local missing=()
  local k v
  for k in DOMAIN ACME_EMAIL CF_DNS_API_TOKEN CP_ENV TRAEFIK_IMAGE CONTROL_PLANE_IMAGE CP_ADMIN_KEY CP_PULSE_IMAGE PULSE_PROVIDER_MSP_DATA_DIR PULSE_PROVIDER_MSP_DOCKER_NETWORK PULSE_PROVIDER_MSP_DOCKER_SOCKET PULSE_PROVIDER_MSP_HOST_ROOT PULSE_PROVIDER_MSP_DOCKER_DATA_DIR CP_PROVIDER_MSP_LICENSE_FILE CP_TRIAL_ACTIVATION_PRIVATE_KEY CP_TENANT_MEMORY_LIMIT CP_ALLOW_DOCKERLESS_PROVISIONING CP_STORAGE_GUARDRAILS_ENABLED CP_STORAGE_MIN_ROOT_AVAILABLE CP_STORAGE_MIN_DATA_AVAILABLE CP_STORAGE_MIN_DOCKER_AVAILABLE CP_STORAGE_MAX_DOCKER_BUILD_CACHE CP_PROOF_TENANT_MAX_AGE CP_PROOF_TENANT_MATCHERS CP_REQUIRE_EMAIL_PROVIDER PULSE_EMAIL_FROM PULSE_EMAIL_REPLY_TO; do
    v="$(env_value "${k}" "${env_path}")"
    if [[ -z "${v}" ]]; then
      missing+=("${k}")
    fi
  done
  if [[ "${#missing[@]}" -ne 0 ]]; then
    die "missing required values in ${env_path}: ${missing[*]}"
  fi

  cp_env="$(env_value CP_ENV "${env_path}" | tr '[:upper:]' '[:lower:]')"
  if [[ "${cp_env}" != "${expected_env}" ]]; then
    die "CP_ENV must be '${expected_env}' for this setup run (got '${cp_env}')"
  fi

  local path_var path_value
  for path_var in PULSE_PROVIDER_MSP_DATA_DIR PULSE_PROVIDER_MSP_DOCKER_SOCKET PULSE_PROVIDER_MSP_HOST_ROOT PULSE_PROVIDER_MSP_DOCKER_DATA_DIR; do
    path_value="$(env_value "${path_var}" "${env_path}")"
    if [[ "${path_value}" != /* ]]; then
      die "${path_var} must be an absolute path"
    fi
  done
  if [[ ! -S "$(env_value PULSE_PROVIDER_MSP_DOCKER_SOCKET "${env_path}")" ]]; then
    die "PULSE_PROVIDER_MSP_DOCKER_SOCKET must point to a reachable Docker socket"
  fi
  if [[ ! -d "$(env_value PULSE_PROVIDER_MSP_HOST_ROOT "${env_path}")" ]]; then
    die "PULSE_PROVIDER_MSP_HOST_ROOT must point to an existing directory"
  fi

  local image_ref
  for k in TRAEFIK_IMAGE CONTROL_PLANE_IMAGE CP_PULSE_IMAGE; do
    image_ref="$(env_value "${k}" "${env_path}")"
    if [[ "${image_ref}" != *@sha256:* || "${image_ref}" == *"<pin>"* ]]; then
      die "${k} must be an immutable digest ref (expected ...@sha256:...)"
    fi
  done

  local forbidden forbidden_value
  for forbidden in STRIPE_API_KEY STRIPE_WEBHOOK_SECRET CP_TRIAL_SIGNUP_PRICE_ID CP_PUBLIC_CLOUD_SIGNUP_ENABLED CP_MSP_STARTER_PRICE_ID CP_MSP_GROWTH_PRICE_ID CP_MSP_SCALE_PRICE_ID; do
    forbidden_value="$(env_value "${forbidden}" "${env_path}")"
    if [[ -n "${forbidden_value}" ]]; then
      die "${forbidden} must not be configured in provider-hosted MSP mode"
    fi
  done

  if ! falsy "$(env_value CP_ALLOW_DOCKERLESS_PROVISIONING "${env_path}")"; then
    die "CP_ALLOW_DOCKERLESS_PROVISIONING must be false for provider-hosted MSP deploys"
  fi
  if ! truthy "$(env_value CP_STORAGE_GUARDRAILS_ENABLED "${env_path}")"; then
    die "CP_STORAGE_GUARDRAILS_ENABLED must be true for provider-hosted MSP deploys"
  fi
  if [[ ! -d "$(env_value PULSE_PROVIDER_MSP_DOCKER_DATA_DIR "${env_path}")" ]]; then
    die "PULSE_PROVIDER_MSP_DOCKER_DATA_DIR must point to the host Docker data directory"
  fi

  local proof_matchers required_matcher
  proof_matchers="$(env_value CP_PROOF_TENANT_MATCHERS "${env_path}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  for required_matcher in proof canary rehearsal msp_prod ownerseed owner_seed; do
    if [[ ",${proof_matchers}," != *",${required_matcher},"* ]]; then
      die "CP_PROOF_TENANT_MATCHERS must include '${required_matcher}'"
    fi
  done

  local require_email
  require_email="$(env_value CP_REQUIRE_EMAIL_PROVIDER "${env_path}")"
  if ! truthy "${require_email}" && ! falsy "${require_email}"; then
    die "CP_REQUIRE_EMAIL_PROVIDER must be an explicit boolean value"
  fi
  if truthy "${require_email}" && [[ -z "$(env_value RESEND_API_KEY "${env_path}")" ]]; then
    die "RESEND_API_KEY is required when CP_REQUIRE_EMAIL_PROVIDER=true"
  fi

  local license_file
  license_file="$(env_value CP_PROVIDER_MSP_LICENSE_FILE "${env_path}")"
  if [[ "${license_file}" != /* ]]; then
    license_file="${PULSE_PROVIDER_MSP_INSTALL_DIR}/${license_file}"
  fi
  [[ -f "${license_file}" ]] || die "CP_PROVIDER_MSP_LICENSE_FILE does not exist: ${license_file}"
}

validate_compose_config() {
  log "validating compose config"
  (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose config --quiet)
}

pull_provider_images() {
  if truthy "${PULSE_PROVIDER_MSP_SKIP_PULL}"; then
    log "skipping image pull because PULSE_PROVIDER_MSP_SKIP_PULL=${PULSE_PROVIDER_MSP_SKIP_PULL}"
    return 0
  fi
  log "pulling provider MSP images"
  (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose pull traefik control-plane)
}

run_install_proof_if_requested() {
  local mode
  mode="$(echo "${PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF}" | tr '[:upper:]' '[:lower:]')"
  case "${mode}" in
    auto|true|1|yes|on|false|0|no|off) ;;
    *) die "PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF must be auto, true, or false" ;;
  esac

  if falsy "${mode}"; then
    return 0
  fi
  if [[ -z "${PULSE_PROVIDER_MSP_ACCOUNT_NAME}" || -z "${PULSE_PROVIDER_MSP_OWNER_EMAIL}" ]]; then
    if truthy "${mode}"; then
      die "PULSE_PROVIDER_MSP_ACCOUNT_NAME and PULSE_PROVIDER_MSP_OWNER_EMAIL are required when PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF=true"
    fi
    return 0
  fi

  log "running provider MSP install proof"
  (
    cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}"
    PROVIDER_MSP_ACCOUNT_NAME="${PULSE_PROVIDER_MSP_ACCOUNT_NAME}" \
      PROVIDER_MSP_OWNER_EMAIL="${PULSE_PROVIDER_MSP_OWNER_EMAIL}" \
      ./run-install-proof.sh
  )
}

print_summary() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local domain data_dir network
  domain="$(env_value DOMAIN "${env_path}")"
  data_dir="$(provider_data_dir)"
  network="$(provider_docker_network)"

  cat <<EOF

Pulse Provider MSP setup prepared.

Paths:
  - Deploy dir: ${PULSE_PROVIDER_MSP_INSTALL_DIR}
  - Data dir:   ${data_dir}
  - Network:    ${network}

Proof:
  cd ${PULSE_PROVIDER_MSP_INSTALL_DIR}
  ./run-install-proof.sh --account-name "Example MSP" --owner-email owner@example.com

Portal:
  https://${domain}/

EOF
}

main() {
  need_root

  log "starting provider MSP first-time setup"
  apt_install apt-transport-https
  install_docker_ce
  install_ops_tools
  install_deploy_bundle
  ensure_env_file
  validate_env_file
  create_data_dirs
  ensure_docker_network
  validate_compose_config
  pull_provider_images
  run_install_proof_if_requested
  print_summary
}

main "$@"
