#!/usr/bin/env bash
# Pulse Cloud Platform — First-Time Setup
# Run on a fresh DigitalOcean droplet (Ubuntu 24.04)
# Usage: curl -sL <url> | bash  OR  bash setup.sh

set -euo pipefail
IFS=$'\n\t'

PULSE_CLOUD_INSTALL_DIR="${PULSE_CLOUD_INSTALL_DIR:-/opt/pulse-cloud}"
PULSE_CLOUD_DATA_DIR="${PULSE_CLOUD_DATA_DIR:-/data}"
PULSE_CLOUD_DOCKER_NETWORK="${PULSE_CLOUD_DOCKER_NETWORK:-pulse-cloud}"
PULSE_CLOUD_BUNDLE_URL="${PULSE_CLOUD_BUNDLE_URL:-}"

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
  if have docker && docker --version >/dev/null 2>&1; then
    log "docker already installed"
    return 0
  fi

  log "installing Docker CE + compose plugin (official Docker repo)"
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
  log "installing ops tools (jq, rsync, sqlite3, rclone, s3cmd)"
  apt_install jq rsync sqlite3 rclone s3cmd
}

create_data_dirs() {
  log "creating data directories under ${PULSE_CLOUD_DATA_DIR}"
  install -d -m 0700 "${PULSE_CLOUD_DATA_DIR}"
  install -d -m 0700 "${PULSE_CLOUD_DATA_DIR}/tenants"
  install -d -m 0700 "${PULSE_CLOUD_DATA_DIR}/control-plane"
  install -d -m 0700 "${PULSE_CLOUD_DATA_DIR}/traefik"
  install -d -m 0700 "${PULSE_CLOUD_DATA_DIR}/backups"
}

ensure_docker_network() {
  log "ensuring Docker network ${PULSE_CLOUD_DOCKER_NETWORK} exists"
  if ! docker network inspect "${PULSE_CLOUD_DOCKER_NETWORK}" >/dev/null 2>&1; then
    docker network create "${PULSE_CLOUD_DOCKER_NETWORK}" >/dev/null
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
  log "installing deploy bundle to ${PULSE_CLOUD_INSTALL_DIR}"
  install -d -m 0755 "${PULSE_CLOUD_INSTALL_DIR}"

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
    "Dockerfile.control-plane"
    ".env.example"
  )

  if [[ -n "${src_dir}" ]]; then
    for f in "${required[@]}"; do
      [[ -f "${src_dir}/${f}" ]] || src_dir=""
    done
  fi

  # If setup.sh was piped via stdin (or run without the deploy bundle present),
  # allow fetching a tar.gz bundle that contains deploy/cloud/*.
  if [[ -z "${src_dir}" && -n "${PULSE_CLOUD_BUNDLE_URL}" ]]; then
    log "deploy bundle not found locally; downloading PULSE_CLOUD_BUNDLE_URL"

    local tmp
    tmp="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap "rm -rf \"${tmp}\"" EXIT

    curl -fsSL "${PULSE_CLOUD_BUNDLE_URL}" -o "${tmp}/bundle.tgz"
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
  - Dockerfile.control-plane
  - .env.example

Run it from a clone of the repo (deploy/cloud/), or set PULSE_CLOUD_BUNDLE_URL to a
tar.gz that contains those files (for curl | bash style installs).
EOF
    exit 1
  fi

  install -m 0644 "${src_dir}/docker-compose.yml" "${PULSE_CLOUD_INSTALL_DIR}/docker-compose.yml"
  install -m 0644 "${src_dir}/traefik.yml" "${PULSE_CLOUD_INSTALL_DIR}/traefik.yml"
  install -m 0644 "${src_dir}/traefik-dynamic.yml" "${PULSE_CLOUD_INSTALL_DIR}/traefik-dynamic.yml"
  install -m 0644 "${src_dir}/Dockerfile.control-plane" "${PULSE_CLOUD_INSTALL_DIR}/Dockerfile.control-plane"
  install -m 0644 "${src_dir}/.env.example" "${PULSE_CLOUD_INSTALL_DIR}/.env.example"

  # Install operational scripts alongside compose so cron/jobs can reference stable paths.
  if [[ -f "${src_dir}/backup.sh" ]]; then
    install -m 0755 "${src_dir}/backup.sh" "${PULSE_CLOUD_INSTALL_DIR}/backup.sh"
  fi
  if [[ -f "${src_dir}/rollout.sh" ]]; then
    install -m 0755 "${src_dir}/rollout.sh" "${PULSE_CLOUD_INSTALL_DIR}/rollout.sh"
  fi
  if [[ -f "${src_dir}/RUNBOOK.md" ]]; then
    install -m 0644 "${src_dir}/RUNBOOK.md" "${PULSE_CLOUD_INSTALL_DIR}/RUNBOOK.md"
  fi
}

ensure_env_file() {
  local env_path="${PULSE_CLOUD_INSTALL_DIR}/.env"
  if [[ -f "${env_path}" ]]; then
    chmod 0600 "${env_path}" || true
    return 0
  fi

  log "no ${env_path}; creating from .env.example"
  if [[ -f "${PULSE_CLOUD_INSTALL_DIR}/.env.example" ]]; then
    cp -n "${PULSE_CLOUD_INSTALL_DIR}/.env.example" "${env_path}"
    chmod 0600 "${env_path}"
  else
    die "missing ${PULSE_CLOUD_INSTALL_DIR}/.env.example; cannot create ${env_path}"
  fi

  cat <<EOF

Created ${env_path} from .env.example.

Edit it now and set required values:
  - DOMAIN
  - ACME_EMAIL
  - CF_API_EMAIL
  - CF_API_KEY
  - CP_ADMIN_KEY
  - STRIPE_WEBHOOK_SECRET
  - STRIPE_API_KEY

EOF

  if [[ -t 0 ]]; then
    read -r -p "Press Enter to continue after editing ${env_path}..." _
  else
    die "non-interactive run: edit ${env_path} then re-run setup.sh"
  fi
}

validate_env_file() {
  local env_path="${PULSE_CLOUD_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"

  local missing=()
  local k v
  for k in DOMAIN ACME_EMAIL CF_API_EMAIL CF_API_KEY CP_ADMIN_KEY STRIPE_WEBHOOK_SECRET STRIPE_API_KEY; do
    v="$(grep -E "^${k}=" "${env_path}" | tail -n 1 | cut -d= -f2- || true)"
    # Trim surrounding quotes and whitespace.
    v="${v%\"}"; v="${v#\"}"
    v="${v%\'}"; v="${v#\'}"
    v="$(echo "${v}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    if [[ -z "${v}" ]]; then
      missing+=("${k}")
    fi
  done

  if [[ "${#missing[@]}" -ne 0 ]]; then
    die "missing required values in ${env_path}: ${missing[*]}"
  fi
}

compose_up() {
  log "pulling images"
  (cd "${PULSE_CLOUD_INSTALL_DIR}" && docker compose pull)

  log "starting services"
  (cd "${PULSE_CLOUD_INSTALL_DIR}" && docker compose up -d)
}

verify_health() {
  local env_path="${PULSE_CLOUD_INSTALL_DIR}/.env"
  local domain
  domain="$(grep -E '^DOMAIN=' "${env_path}" | tail -n 1 | cut -d= -f2-)"
  domain="${domain%\"}"; domain="${domain#\"}"
  domain="${domain%\'}"; domain="${domain#\'}"

  log "verifying Traefik is listening on :443"
  if ! ss -lnt | awk '{print $4}' | grep -qE '(:443)$'; then
    die "port 443 not listening (check: docker compose ps; docker logs traefik)"
  fi

  log "verifying control plane health via Traefik routing (this may take a minute if ACME is issuing certs)"
  local url="https://${domain}/healthz"
  local ready="https://${domain}/readyz"
  local status="https://${domain}/status"

  # Use --resolve to avoid depending on public DNS being set up yet.
  local resolve=(--resolve "${domain}:443:127.0.0.1")

  local i
  for i in {1..30}; do
    if curl -fsS -k "${resolve[@]}" "${url}" >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done

  curl -fsS -k "${resolve[@]}" "${url}" >/dev/null
  curl -fsS -k "${resolve[@]}" "${ready}" >/dev/null
  curl -fsS -k "${resolve[@]}" "${status}" >/dev/null
}

print_summary() {
  local env_path="${PULSE_CLOUD_INSTALL_DIR}/.env"
  local domain
  domain="$(grep -E '^DOMAIN=' "${env_path}" | tail -n 1 | cut -d= -f2-)"
  domain="${domain%\"}"; domain="${domain#\"}"
  domain="${domain%\'}"; domain="${domain#\'}"

  cat <<EOF

Pulse Cloud setup complete.

Installed:
  - Docker: $(docker --version | sed 's/,.*//')
  - Docker Compose: $(docker compose version | head -n 1)

Paths:
  - Deploy dir: ${PULSE_CLOUD_INSTALL_DIR}
  - Data dir:   ${PULSE_CLOUD_DATA_DIR}

Health:
  - Control plane: https://${domain}/healthz
  - Status:        https://${domain}/status

Next steps:
  1) DNS:
     - A record: ${domain} -> <droplet public IP>
     - Wildcard A record: *.${domain} -> <droplet public IP>
  2) Stripe:
     - Create webhook endpoint: https://${domain}/api/stripe/webhook
     - Subscribe to: checkout.session.completed, customer.subscription.updated,
       customer.subscription.deleted, invoice.payment_failed
  3) Backups:
     - Configure rclone or s3cmd (see RUNBOOK.md), then add cron:
       0 3 * * * ${PULSE_CLOUD_INSTALL_DIR}/backup.sh

EOF
}

main() {
  need_root

  log "starting first-time setup"
  apt_install apt-transport-https
  install_docker_ce
  install_ops_tools
  create_data_dirs
  ensure_docker_network
  install_deploy_bundle
  ensure_env_file
  validate_env_file
  compose_up
  verify_health
  print_summary
}

main "$@"
