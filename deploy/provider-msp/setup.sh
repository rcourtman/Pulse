#!/usr/bin/env bash
# Pulse Provider MSP - first-time host setup
# Run on a fresh Ubuntu 24.04 host as root.

set -euo pipefail
IFS=$'\n\t'

PULSE_PROVIDER_MSP_INSTALL_DIR="${PULSE_PROVIDER_MSP_INSTALL_DIR:-/opt/pulse-provider-msp}"
PULSE_PROVIDER_MSP_DATA_DIR="${PULSE_PROVIDER_MSP_DATA_DIR:-/data}"
PULSE_PROVIDER_MSP_DOCKER_NETWORK="${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}"
PULSE_PROVIDER_MSP_DOCKER_SUBNET="${PULSE_PROVIDER_MSP_DOCKER_SUBNET:-172.30.0.0/24}"
PULSE_PROVIDER_MSP_DOCKER_SOCKET="${PULSE_PROVIDER_MSP_DOCKER_SOCKET:-/var/run/docker.sock}"
PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR="${PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR:-/var/lib/pulse-provider-msp/spacecheck/root}"
PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR="${PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR:-/var/lib/docker/.pulse-provider-msp-spacecheck}"
PULSE_PROVIDER_MSP_BUNDLE_URL="${PULSE_PROVIDER_MSP_BUNDLE_URL:-}"
PULSE_PROVIDER_MSP_EXPECT_ENV="${PULSE_PROVIDER_MSP_EXPECT_ENV:-production}"
PULSE_PROVIDER_MSP_SKIP_PULL="${PULSE_PROVIDER_MSP_SKIP_PULL:-0}"
PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF="${PULSE_PROVIDER_MSP_RUN_INSTALL_PROOF:-auto}"
PULSE_PROVIDER_MSP_ACCOUNT_NAME="${PULSE_PROVIDER_MSP_ACCOUNT_NAME:-}"
PULSE_PROVIDER_MSP_OWNER_EMAIL="${PULSE_PROVIDER_MSP_OWNER_EMAIL:-}"
PULSE_PROVIDER_MSP_LICENSE_URL="${PULSE_PROVIDER_MSP_LICENSE_URL:-https://license.pulserelay.pro}"
PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE="${PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE:-0}"
PULSE_PROVIDER_MSP_EVAL_EMAIL="${PULSE_PROVIDER_MSP_EVAL_EMAIL:-}"
PULSE_PROVIDER_MSP_SIGNUP_SOURCE="${PULSE_PROVIDER_MSP_SIGNUP_SOURCE:-provider_msp_setup}"
# The values only a provider can supply. Seeded into a new .env; an interactive
# run asks for any left blank before installing anything.
PULSE_PROVIDER_MSP_DOMAIN="${PULSE_PROVIDER_MSP_DOMAIN:-}"
PULSE_PROVIDER_MSP_ACME_EMAIL="${PULSE_PROVIDER_MSP_ACME_EMAIL:-}"
PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN="${PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN:-}"
# http issues one certificate per client over port 80 and needs no DNS token.
PULSE_PROVIDER_MSP_ACME_CHALLENGE="${PULSE_PROVIDER_MSP_ACME_CHALLENGE:-}"
# 1 builds a shared trial address from this host's public IP instead of a domain.
PULSE_PROVIDER_MSP_TRIAL_ADDRESS="${PULSE_PROVIDER_MSP_TRIAL_ADDRESS:-0}"
PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS="${PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS:-180}"

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
  apt_install jq openssl rsync sqlite3 rclone s3cmd
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

  local root_spacecheck docker_spacecheck
  root_spacecheck="$(provider_root_spacecheck_dir)"
  docker_spacecheck="$(provider_docker_spacecheck_dir)"
  log "creating storage space-check marker directories"
  install -d -m 0700 "${root_spacecheck}"
  install -d -m 0700 "${docker_spacecheck}"
}

ensure_docker_network() {
  local network subnet existing_subnets
  network="$(provider_docker_network)"
  subnet="$(provider_docker_subnet)"
  log "checking Docker network ${network}"
  if ! docker network inspect "${network}" >/dev/null 2>&1; then
    log "Docker network ${network} will be created by compose with subnet ${subnet}"
    return 0
  fi

  existing_subnets="$(docker network inspect -f '{{range .IPAM.Config}}{{println .Subnet}}{{end}}' "${network}" 2>/dev/null | tr '\n' ',' | sed 's/,$//' || true)"
  if [[ ",${existing_subnets}," != *",${subnet},"* ]]; then
    die "Docker network ${network} exists with subnet(s) ${existing_subnets:-<none>}; expected ${subnet} so CP_TRUSTED_PROXY_CIDRS can trust Traefik without trusting every peer"
  fi
}

block_container_metadata_service() {
  if ! have iptables; then
    log "iptables not found; skipping container metadata-service block"
    return 0
  fi
  log "ensuring containers cannot reach cloud metadata service"
  iptables -N DOCKER-USER 2>/dev/null || true
  if ! iptables -C DOCKER-USER -d 169.254.169.254/32 -j REJECT >/dev/null 2>&1; then
    iptables -I DOCKER-USER -d 169.254.169.254/32 -j REJECT
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
  # Traefik's static configuration lives in docker-compose.yml; an installed
  # traefik.yml would only mislead whoever edits it.
  rm -f "${PULSE_PROVIDER_MSP_INSTALL_DIR}/traefik.yml"
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

provider_docker_subnet() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local configured=""
  if [[ -f "${env_path}" ]]; then
    configured="$(env_value PULSE_PROVIDER_MSP_DOCKER_SUBNET "${env_path}")"
  fi
  echo "${configured:-${PULSE_PROVIDER_MSP_DOCKER_SUBNET}}"
}

provider_root_spacecheck_dir() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local configured=""
  if [[ -f "${env_path}" ]]; then
    configured="$(env_value PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR "${env_path}")"
  fi
  echo "${configured:-${PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR}}"
}

provider_docker_spacecheck_dir() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local configured=""
  if [[ -f "${env_path}" ]]; then
    configured="$(env_value PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR "${env_path}")"
  fi
  echo "${configured:-${PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR}}"
}

set_env_value() {
  local key="$1"
  local value="$2"
  local env_path="${3:-${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env}"
  local tmp
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

# default_image_ref maps each image variable to the tag its digest is resolved
# from when the operator has not pinned one by hand.
default_image_ref() {
  case "$1" in
    TRAEFIK_IMAGE) echo "traefik:v3" ;;
    DOCKER_SOCKET_PROXY_IMAGE) echo "tecnativa/docker-socket-proxy:latest" ;;
    CONTROL_PLANE_IMAGE) echo "ghcr.io/rcourtman/pulse-control-plane:latest" ;;
    CP_PULSE_IMAGE) echo "ghcr.io/rcourtman/pulse:latest" ;;
    *) return 1 ;;
  esac
}

# resolve_image_digest turns a tag into an immutable digest ref. Uses buildx
# imagetools, which the Docker install above provides, and which reads the
# registry without pulling the image.
resolve_image_digest() {
  local ref="$1" manifest_json digest
  manifest_json="$(docker buildx imagetools inspect "${ref}" --format '{{json .Manifest}}' 2>/dev/null || true)"
  digest="$(printf '%s' "${manifest_json}" | jq -r 'if type == "object" then .digest // empty else empty end' 2>/dev/null || true)"
  if [[ "${digest}" != sha256:* ]]; then
    # Ubuntu's packaged Buildx and Docker's plugin have differed in which
    # fields their Go template exposes. Keep a human-output fallback so an
    # already-working Docker install is not rejected only because its Buildx
    # formatter is older or distro-patched.
    digest="$(docker buildx imagetools inspect "${ref}" 2>/dev/null | awk '$1 == "Digest:" {print $2; exit}' || true)"
  fi
  [[ "${digest}" == sha256:* ]] || return 1
  printf '%s@%s\n' "${ref%:*}" "${digest}"
}

# ensure_image_pins resolves every tag-based image reference to an immutable
# digest. Operators may provide an exact release tag or a digest directly;
# blank and legacy <pin> values use the bundle defaults.
#
# The bundle used to ship four unfillable "@sha256:<pin>" placeholders that
# setup.sh then refused to run without, so the only way to obtain them was to
# ask us. All four images are publicly readable, so there was never anything
# to hand out; it just meant nobody could start without a conversation first.
#
# Still resolved to an immutable digest, not left on a tag, so a later tag
# mutation cannot silently change what a provider is running.
ensure_image_pins() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"

  local key current ref resolved
  for key in TRAEFIK_IMAGE DOCKER_SOCKET_PROXY_IMAGE CONTROL_PLANE_IMAGE CP_PULSE_IMAGE; do
    current="$(env_value "${key}" "${env_path}")"
    if [[ "${current}" == *@sha256:* && "${current}" != *"<pin>"* ]]; then
      continue
    fi
    if [[ -n "${current}" && "${current}" != *"<pin>"* ]]; then
      ref="${current}"
    else
      ref="$(default_image_ref "${key}")" || die "no default image ref for ${key}"
    fi
    log "resolving ${key} digest from ${ref}"
    if ! resolved="$(resolve_image_digest "${ref}")"; then
      die "could not resolve a digest for ${ref}
Set ${key} in ${env_path} by hand, or check this host can reach the registry."
    fi
    log "  ${resolved}"
    set_env_value "${key}" "${resolved}" "${env_path}"
  done
}

# ensure_eval_license self-issues a capped evaluation license when the operator
# has not supplied one.
#
# An unlicensed control plane starts, but release-build client runtimes only
# trust entitlement leases chained to a Pulse-signed license, so its client
# workspaces would run without the capabilities being evaluated. Requesting one
# by email put a human round-trip in front of the first screen; this asks the
# license server directly with the public half of the key generated above.
#
# The private key never leaves this host. Failure is a warning, not a stop:
# the portal can start unlicensed, but a running portal is not proof that client
# runtimes have MSP capabilities. An air-gapped operator can skip the request.
ensure_eval_license() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"

  if [[ -n "$(env_value CP_PROVIDER_MSP_LICENSE_FILE "${env_path}")" ]]; then
    return 0
  fi

  local eval_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/provider-msp-eval-license.jwt"
  if [[ -s "${eval_path}" ]]; then
    log "reusing existing evaluation license ${eval_path}"
    set_env_value CP_PROVIDER_MSP_LICENSE_FILE "./provider-msp-eval-license.jwt" "${env_path}"
    return 0
  fi

  if truthy "${PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE}"; then
    log "PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE set: staying unlicensed"
    log "  client MSP capabilities are not active until a license is installed"
    return 0
  fi

  have curl || die "curl is required to request an evaluation license"

  # Tolerate failure rather than abort: ensure_generated_secrets has already
  # created the key, so this only trips on something unexpected, and an
  # evaluation licence is never worth failing an otherwise good install.
  #
  # Tested with `if !` rather than `|| true` inside the substitution: the
  # derive helper calls die, and `exit` in a subshell terminates it outright
  # instead of yielding a status `||` could catch, so setup.sh would abort.
  local public_key=""
  if ! public_key="$(derive_lease_signing_public_key 2>/dev/null)"; then
    public_key=""
  fi
  if [[ -z "${public_key}" ]]; then
    log "warning: could not derive the lease signing public key; staying unlicensed"
    return 0
  fi

  log "requesting a 2-client evaluation license"
  local body response http_status curl_exit token=""
  body="$(jq -cn \
    --arg public_key "${public_key}" \
    --arg email "${PULSE_PROVIDER_MSP_EVAL_EMAIL}" \
    --arg signup_source "${PULSE_PROVIDER_MSP_SIGNUP_SOURCE}" \
    '{entitlement_signing_public_key: $public_key, setup_stage: "images_ready"}
      + (if $email == "" then {} else {email: $email} end)
      + (if $signup_source == "" then {} else {signup_source: $signup_source} end)')"
  # Keep transport exit, HTTP status and body separate. --fail discards the
  # distinction between a request rejection and a connection failure. The
  # final curl-owned line is metadata, even when the JSON body has newlines.
  # Neither arbitrary response/error text nor the configured URL is logged.
  if response="$(curl -sS --max-time 20 --write-out $'\n%{http_code}' \
      -H 'Content-Type: application/json' \
      -d "${body}" \
      "${PULSE_PROVIDER_MSP_LICENSE_URL%/}/v1/provider-msp/eval-license" 2>/dev/null)"; then
    curl_exit=0
  else
    curl_exit=$?
  fi
  http_status="${response##*$'\n'}"
  response="${response%$'\n'*}"

  if [[ "${curl_exit}" != 0 ]]; then
    log "warning: evaluation license request failed (curl exit ${curl_exit})"
    case "${curl_exit}" in
      6) log "  check this host's DNS resolution before retrying" ;;
      7|28) log "  check this host's connection to the license server before retrying" ;;
      35|51|60) log "  check the license endpoint and certificate trust before retrying; keep TLS verification enabled" ;;
      *) log "  check the configured license endpoint and host connectivity before retrying" ;;
    esac
  elif [[ ! "${http_status}" =~ ^[1-5][0-9]{2}$ ]]; then
    log "warning: license server returned an unexpected HTTP status"
  elif [[ "${http_status}" == 429 ]]; then
    log "warning: evaluation license request was rate limited (HTTP 429)"
    log "  wait before a deliberate retry"
  elif [[ "${http_status}" == 4* ]]; then
    log "warning: license server rejected the evaluation request (HTTP ${http_status})"
    log "  Check the request details before retrying, including the configured endpoint and public signing key."
  elif [[ "${http_status}" == 5* ]]; then
    log "warning: evaluation license service is temporarily unavailable (HTTP ${http_status})"
    log "  try again later; no automatic retry was attempted"
  elif [[ "${http_status}" != 2* ]]; then
    log "warning: license server returned an unexpected HTTP status (HTTP ${http_status})"
    log "  check the configured license endpoint before retrying"
  else
    # A successful HTTP exchange is not an issued licence. Admit exactly one
    # JSON object and a nonempty string, never a number/object or extra document.
    token="$(printf '%s' "${response}" | jq -ers '
      if length == 1 and (.[0] | type == "object") then
        .[0].license | select(type == "string" and length > 0 and (test("[[:space:][:cntrl:]]") | not))
      else empty end' 2>/dev/null || true)"
    if [[ -z "${token}" ]]; then
      log "warning: license server response contained no valid license"
    fi
  fi
  if [[ -z "${token}" ]]; then
    log "  continuing unlicensed; client MSP capabilities are not active"
    log "  resolve the failure before deliberately re-running setup.sh, or set"
    log "  CP_PROVIDER_MSP_LICENSE_FILE to an existing license bound to this host's signing key."
    log "  Keep saved clients and signing keys; do not recreate them to retry issuance."
    return 0
  fi

  printf '%s' "${token}" >"${eval_path}"
  chmod 0600 "${eval_path}"
  set_env_value CP_PROVIDER_MSP_LICENSE_FILE "./provider-msp-eval-license.jwt" "${env_path}"

  local expires
  expires="$(printf '%s' "${response}" | jq -r '.expires_at | select(type == "string")' 2>/dev/null || true)"
  # Preserve useful expiry information without echoing arbitrary server data.
  if [[ ! "${expires}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$ ]]; then
    expires=""
  fi
  log "evaluation license installed: 2 client workspaces${expires:+, expires ${expires}}"
  log "  Plan in your provider portal shows whether a paid upgrade is available"
  log "  keep within two clients until Plan confirms a higher active limit"
}

ensure_generated_secrets() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"
  have openssl || die "openssl is required to generate provider MSP secrets"

  if [[ -z "$(env_value CP_ADMIN_KEY "${env_path}")" ]]; then
    log "generating CP_ADMIN_KEY"
    set_env_value CP_ADMIN_KEY "$(openssl rand -hex 32)" "${env_path}"
  fi
  if [[ -z "$(env_value CP_ENTITLEMENT_SIGNING_PRIVATE_KEY "${env_path}")" ]]; then
    log "generating CP_ENTITLEMENT_SIGNING_PRIVATE_KEY"
    set_env_value CP_ENTITLEMENT_SIGNING_PRIVATE_KEY "$(openssl rand -base64 32 | tr -d '\n')" "${env_path}"
  fi
  chmod 0600 "${env_path}"
}

# derive_lease_signing_public_key prints the base64 Ed25519 public key for
# CP_ENTITLEMENT_SIGNING_PRIVATE_KEY. The provider MSP license must bind this
# exact key (entitlement_signing_public_key) or the control plane will refuse
# to start; include it when requesting your license. The private key never
# leaves this host.
derive_lease_signing_public_key() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  [[ -f "${env_path}" ]] || die "missing ${env_path}"
  have openssl || die "openssl is required to derive the lease signing public key"

  local key_b64 key_len tmp_der
  key_b64="$(env_value CP_ENTITLEMENT_SIGNING_PRIVATE_KEY "${env_path}")"
  [[ -n "${key_b64}" ]] || die "CP_ENTITLEMENT_SIGNING_PRIVATE_KEY is not set; run setup.sh first"
  key_len="$(printf '%s' "${key_b64}" | base64 -d 2>/dev/null | wc -c | tr -d ' ')"
  case "${key_len}" in
    64)
      # 64-byte Ed25519 private key: the public key is the trailing 32 bytes.
      printf '%s' "${key_b64}" | base64 -d | tail -c 32 | base64 | tr -d '\n'
      ;;
    32)
      # 32-byte seed: wrap in a PKCS#8 DER envelope and let openssl derive
      # the public key (raw key = trailing 32 bytes of the SPKI DER).
      tmp_der="$(mktemp)"
      {
        printf '\x30\x2e\x02\x01\x00\x30\x05\x06\x03\x2b\x65\x70\x04\x22\x04\x20'
        printf '%s' "${key_b64}" | base64 -d
      } >"${tmp_der}"
      openssl pkey -inform DER -in "${tmp_der}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n'
      rm -f "${tmp_der}"
      ;;
    *)
      die "CP_ENTITLEMENT_SIGNING_PRIVATE_KEY must decode to a 32-byte seed or 64-byte Ed25519 key (got ${key_len} bytes)"
      ;;
  esac
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

# Values reach .env unquoted, so each is held to characters that compose and
# set_env_value read literally: no whitespace, quotes, $, # or backslashes.
valid_first_run_value() {
  local key="$1" value="$2"
  local label='[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?'
  local domain_re="^(${label}\\.)+[A-Za-z]{2,63}$"
  local email_re="^[A-Za-z0-9._%+-]+@(${label}\\.)+[A-Za-z]{2,63}$"
  local token_re='^[A-Za-z0-9_.+/=-]{20,255}$'
  case "${key}" in
    DOMAIN) [[ "${value}" =~ ${domain_re} ]] ;;
    ACME_EMAIL) [[ "${value}" =~ ${email_re} ]] ;;
    CF_DNS_API_TOKEN) [[ "${value}" =~ ${token_re} ]] ;;
    ACME_CHALLENGE) [[ "${value}" == dns || "${value}" == http ]] ;;
    *) return 1 ;;
  esac
}

# Routable from the internet: not private, loopback, link-local, carrier-grade
# NAT, documentation-free multicast or reserved space.
public_ipv4() {
  local ip="$1" a b c d
  [[ "${ip}" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})$ ]] || return 1
  a="${BASH_REMATCH[1]}"; b="${BASH_REMATCH[2]}"; c="${BASH_REMATCH[3]}"; d="${BASH_REMATCH[4]}"
  (( 10#${a} <= 255 && 10#${b} <= 255 && 10#${c} <= 255 && 10#${d} <= 255 )) || return 1
  (( 10#${a} == 0 || 10#${a} == 10 || 10#${a} == 127 || 10#${a} >= 224 )) && return 1
  (( 10#${a} == 100 && 10#${b} >= 64 && 10#${b} <= 127 )) && return 1
  (( 10#${a} == 169 && 10#${b} == 254 )) && return 1
  (( 10#${a} == 172 && 10#${b} >= 16 && 10#${b} <= 31 )) && return 1
  (( 10#${a} == 192 && 10#${b} == 168 )) && return 1
  return 0
}

# A cloud server's own interface often holds a private address behind 1:1 NAT,
# so ask from outside. A wrong answer only yields a name Let's Encrypt cannot
# reach, which the readiness report then explains.
detect_public_ipv4() {
  local url ip
  have curl || return 1
  for url in https://api.ipify.org https://checkip.amazonaws.com https://ifconfig.me/ip; do
    ip="$(curl -4 -fsS --max-time 8 "${url}" 2>/dev/null | tr -d '[:space:]' || true)"
    if public_ipv4 "${ip}"; then
      printf '%s\n' "${ip}"
      return 0
    fi
  done
  return 1
}

# sslip.io and nip.io both answer any name that embeds an IP with that IP. They
# are not on the Public Suffix List, so all their users share one Let's Encrypt
# limit on new certificates; check_portal_readiness falls back between them.
trial_address() {
  printf 'pulse.%s.%s\n' "${1//./-}" "$2"
}

use_trial_address() {
  local ip
  if ! ip="$(detect_public_ipv4)"; then
    die "could not find a public IPv4 address for this host. A trial address needs a
server the internet can reach on port 80. Use a domain you control instead, or
run setup on a host with a public IP."
  fi
  PULSE_PROVIDER_MSP_DOMAIN="$(trial_address "${ip}" sslip.io)"
  PULSE_PROVIDER_MSP_ACME_CHALLENGE=http
  PULSE_PROVIDER_MSP_TRIAL_ADDRESS=1
}

# Runs before Docker is installed, so the long unattended part of setup never
# stops halfway for a file edit. An existing .env is never prompted for.
collect_first_run_values() {
  [[ -f "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env" ]] && return 0

  if [[ -t 0 ]]; then
    cat <<'EOF'

Pulse Provider MSP needs an address for its portal and clients.

  Your own domain: enter it below. Use this for anything you will keep.
  No domain: press Enter for a trial address built from this server's public IP.
    A trial address is for evaluation only. Moving to your own domain later
    means reinstalling every agent.

EOF
    if [[ -z "${PULSE_PROVIDER_MSP_DOMAIN}" ]] && ! truthy "${PULSE_PROVIDER_MSP_TRIAL_ADDRESS}"; then
      read -r -p "Domain, e.g. pulse.example.com (Enter for a trial address): " PULSE_PROVIDER_MSP_DOMAIN
    fi
    if [[ -z "${PULSE_PROVIDER_MSP_DOMAIN}" ]]; then
      PULSE_PROVIDER_MSP_TRIAL_ADDRESS=1
    fi
    if [[ -z "${PULSE_PROVIDER_MSP_ACME_EMAIL}" ]]; then
      read -r -p "Email for certificate notices: " PULSE_PROVIDER_MSP_ACME_EMAIL
    fi
    if ! truthy "${PULSE_PROVIDER_MSP_TRIAL_ADDRESS}" \
      && [[ -z "${PULSE_PROVIDER_MSP_ACME_CHALLENGE}" && -z "${PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN}" ]]; then
      read -r -s -p "Cloudflare API token with Zone Read and DNS Edit, hidden (Enter to use port 80 instead): " PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN
      echo
      [[ -n "${PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN}" ]] || PULSE_PROVIDER_MSP_ACME_CHALLENGE=http
    fi
  fi

  if truthy "${PULSE_PROVIDER_MSP_TRIAL_ADDRESS}"; then
    use_trial_address
  fi

  local key var value
  for key in DOMAIN ACME_EMAIL CF_DNS_API_TOKEN ACME_CHALLENGE; do
    var="PULSE_PROVIDER_MSP_${key}"
    value="${!var}"
    if [[ -n "${value}" ]] && ! valid_first_run_value "${key}" "${value}"; then
      die "${key} is not valid; check it and run setup.sh again"
    fi
  done

  if truthy "${PULSE_PROVIDER_MSP_TRIAL_ADDRESS}"; then
    cat <<EOF

Trial address: ${PULSE_PROVIDER_MSP_DOMAIN}
No DNS records or token needed. Inbound TCP ports 80 and 443 must be open in
your cloud provider's firewall.

EOF
  elif [[ -n "${PULSE_PROVIDER_MSP_DOMAIN}" ]]; then
    cat <<EOF

Before the certificate is issued, point two DNS records at this host's public
IP (A records; on Cloudflare, proxy status DNS only):
  ${PULSE_PROVIDER_MSP_DOMAIN}      the portal
  *.${PULSE_PROVIDER_MSP_DOMAIN}    one address per client

EOF
    if [[ "${PULSE_PROVIDER_MSP_ACME_CHALLENGE}" == http ]]; then
      cat <<EOF
Each client gets its own certificate over port 80, so inbound TCP ports 80 and
443 must stay open to the internet.

EOF
    fi
  fi
}

# HTTP-01 replaces the two DNS-01 settings on both Traefik resolvers and takes
# the wildcard off the portal certificate. See docker-compose.yml.
write_http_challenge_settings() {
  local env_path="$1"
  set_env_value ACME_CHALLENGE_SETTING_1 "httpchallenge=true" "${env_path}"
  set_env_value ACME_CHALLENGE_SETTING_2 "httpchallenge.entrypoint=web" "${env_path}"
  set_env_value PORTAL_CERTIFICATE_SANS_LABEL "pulse.provider-msp.client-certificates=per-name" "${env_path}"
  set_env_value CP_PER_CLIENT_CERTIFICATES "true" "${env_path}"
}

per_client_certificates() {
  truthy "$(env_value CP_PER_CLIENT_CERTIFICATES "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env")"
}

trial_address_install() {
  truthy "$(env_value PROVIDER_MSP_TRIAL_ADDRESS "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env")"
}

ensure_env_file() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  if [[ -f "${env_path}" ]]; then
    chmod 0600 "${env_path}" || true
    return 0
  fi

  log "no ${env_path}; creating from .env.example"
  (umask 077 && cp -n "${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env.example" "${env_path}")
  chmod 0600 "${env_path}"

  FIRST_RUN_ENV_CREATED=1
  local key var missing=0
  for key in DOMAIN ACME_EMAIL CF_DNS_API_TOKEN; do
    var="PULSE_PROVIDER_MSP_${key}"
    if [[ -n "${!var}" ]]; then
      set_env_value "${key}" "${!var}" "${env_path}"
    elif [[ "${key}" != CF_DNS_API_TOKEN || "${PULSE_PROVIDER_MSP_ACME_CHALLENGE}" != http ]]; then
      missing=1
    fi
  done
  if [[ "${PULSE_PROVIDER_MSP_ACME_CHALLENGE}" == http ]]; then
    write_http_challenge_settings "${env_path}"
  fi
  if truthy "${PULSE_PROVIDER_MSP_TRIAL_ADDRESS}"; then
    set_env_value PROVIDER_MSP_TRIAL_ADDRESS "true" "${env_path}"
  fi
  if [[ "${missing}" -eq 0 ]]; then
    log "wrote the first-run values to ${env_path}"
    return 0
  fi

  cat <<EOF

Created ${env_path} from .env.example.

Edit it now and set the three values only you can supply:
  - DOMAIN (client workspaces are served at https://<client-id>.DOMAIN)
  - ACME_EMAIL
  - CF_DNS_API_TOKEN (with the default ACME_DNS_PROVIDER=cloudflare; for any
    other Traefik dnsChallenge provider, set ACME_DNS_PROVIDER and put that
    provider's credential variables in dns-credentials.env)

Everything else has a working default. setup.sh resolves the image pins to
digests and generates CP_ADMIN_KEY and CP_ENTITLEMENT_SIGNING_PRIVATE_KEY while
they are blank.

EOF

  if [[ -t 0 ]]; then
    read -r -p "Press Enter to continue after editing ${env_path}..." _
  else
    die "non-interactive run: edit ${env_path} then re-run setup.sh"
  fi
}

# Traefik is the only container that needs DNS-01 credentials, and it must not
# receive the operator .env (that holds CP_ADMIN_KEY and the entitlement
# signing private key). Non-Cloudflare providers put their credential
# variables here; compose injects the file into the traefik container alone.
ensure_dns_credentials_file() {
  local creds_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/dns-credentials.env"
  if [[ -f "${creds_path}" ]]; then
    chmod 0600 "${creds_path}" || true
    return 0
  fi
  cat > "${creds_path}" <<'EOF'
# Credential variables for the ACME DNS-01 provider, injected only into the
# traefik container. With the default ACME_DNS_PROVIDER=cloudflare this file
# stays empty; CF_DNS_API_TOKEN in .env is passed through directly. For any
# other provider, set ACME_DNS_PROVIDER in .env to the Traefik dnsChallenge
# provider name and put that provider's variables here, e.g. for route53:
#   AWS_ACCESS_KEY_ID=...
#   AWS_SECRET_ACCESS_KEY=...
#   AWS_REGION=...
EOF
  chmod 0600 "${creds_path}"
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
  for k in DOMAIN ACME_EMAIL CP_ENV TRAEFIK_IMAGE DOCKER_SOCKET_PROXY_IMAGE CONTROL_PLANE_IMAGE CP_ADMIN_KEY CP_PULSE_IMAGE PULSE_PROVIDER_MSP_DATA_DIR PULSE_PROVIDER_MSP_DOCKER_NETWORK PULSE_PROVIDER_MSP_DOCKER_SUBNET PULSE_PROVIDER_MSP_DOCKER_SOCKET PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR CP_TRUSTED_PROXY_CIDRS CP_ENTITLEMENT_SIGNING_PRIVATE_KEY CP_TENANT_MEMORY_LIMIT CP_ALLOW_DOCKERLESS_PROVISIONING CP_STORAGE_GUARDRAILS_ENABLED CP_STORAGE_MIN_ROOT_AVAILABLE CP_STORAGE_MIN_DATA_AVAILABLE CP_STORAGE_MIN_DOCKER_AVAILABLE CP_STORAGE_MAX_DOCKER_BUILD_CACHE CP_PROOF_TENANT_MAX_AGE CP_PROOF_TENANT_MATCHERS CP_REQUIRE_EMAIL_PROVIDER PULSE_EMAIL_FROM PULSE_EMAIL_REPLY_TO; do
    v="$(env_value "${k}" "${env_path}")"
    if [[ -z "${v}" ]]; then
      missing+=("${k}")
    fi
  done
  if [[ "${#missing[@]}" -ne 0 ]]; then
    die "missing required values in ${env_path}: ${missing[*]}"
  fi

  local dns_provider creds_path
  dns_provider="$(env_value ACME_DNS_PROVIDER "${env_path}")"
  dns_provider="${dns_provider:-cloudflare}"
  if per_client_certificates; then
    log "certificates: one per client over port 80, no DNS token"
  elif [[ "${dns_provider}" == "cloudflare" ]]; then
    if [[ -z "$(env_value CF_DNS_API_TOKEN "${env_path}")" ]]; then
      die "CF_DNS_API_TOKEN is required with the default ACME_DNS_PROVIDER=cloudflare; for another provider set ACME_DNS_PROVIDER to a Traefik dnsChallenge provider name and put its credential variables in dns-credentials.env"
    fi
  else
    creds_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/dns-credentials.env"
    if [[ ! -f "${creds_path}" ]] || ! grep -Eq '^[A-Za-z_][A-Za-z0-9_]*=.+' "${creds_path}"; then
      die "ACME_DNS_PROVIDER=${dns_provider}: put that provider's credential variables in ${creds_path} (see Traefik's dnsChallenge provider table for the variable names)"
    fi
  fi

  cp_env="$(env_value CP_ENV "${env_path}" | tr '[:upper:]' '[:lower:]')"
  if [[ "${cp_env}" != "${expected_env}" ]]; then
    die "CP_ENV must be '${expected_env}' for this setup run (got '${cp_env}')"
  fi

  local path_var path_value
  for path_var in PULSE_PROVIDER_MSP_DATA_DIR PULSE_PROVIDER_MSP_DOCKER_SOCKET PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR; do
    path_value="$(env_value "${path_var}" "${env_path}")"
    if [[ "${path_value}" != /* ]]; then
      die "${path_var} must be an absolute path"
    fi
  done
  if [[ ! -S "$(env_value PULSE_PROVIDER_MSP_DOCKER_SOCKET "${env_path}")" ]]; then
    die "PULSE_PROVIDER_MSP_DOCKER_SOCKET must point to a reachable Docker socket"
  fi

  local image_ref
  for k in TRAEFIK_IMAGE DOCKER_SOCKET_PROXY_IMAGE CONTROL_PLANE_IMAGE CP_PULSE_IMAGE; do
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

  local admin_key trial_key trusted_cidrs docker_subnet
  admin_key="$(env_value CP_ADMIN_KEY "${env_path}")"
  if [[ "${#admin_key}" -lt 32 ]]; then
    die "CP_ADMIN_KEY must be at least 32 characters"
  fi
  trial_key="$(env_value CP_ENTITLEMENT_SIGNING_PRIVATE_KEY "${env_path}")"
  if ! printf '%s' "${trial_key}" | base64 -d >/dev/null 2>&1; then
    die "CP_ENTITLEMENT_SIGNING_PRIVATE_KEY must be valid base64"
  fi
  docker_subnet="$(env_value PULSE_PROVIDER_MSP_DOCKER_SUBNET "${env_path}")"
  trusted_cidrs="$(env_value CP_TRUSTED_PROXY_CIDRS "${env_path}" | tr -d '[:space:]')"
  if [[ ",${trusted_cidrs}," != *",${docker_subnet},"* ]]; then
    die "CP_TRUSTED_PROXY_CIDRS must include PULSE_PROVIDER_MSP_DOCKER_SUBNET (${docker_subnet})"
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

  # An empty CP_PROVIDER_MSP_LICENSE_FILE is evaluation mode, not a mistake.
  #
  # This used to be mandatory, which meant nobody could start the stack, create
  # a client workspace, or see the portal until they had emailed for a licence
  # and waited for a human to mint one. That put a round-trip with us in front
  # of the first screen, and an isolation guarantee is the one claim a provider
  # cannot evaluate from a screenshot.
  #
  # Unlicensed runs on msp_eval (2 client workspaces). Paid caps come from a
  # valid licence, never from local configuration.
  local license_file
  license_file="$(env_value CP_PROVIDER_MSP_LICENSE_FILE "${env_path}")"
  if [[ -z "${license_file}" ]]; then
    log "no CP_PROVIDER_MSP_LICENSE_FILE set: evaluation mode, 2 client workspaces"
    return 0
  fi
  if [[ "${license_file}" != /* ]]; then
    license_file="${PULSE_PROVIDER_MSP_INSTALL_DIR}/${license_file}"
  fi
  if [[ ! -f "${license_file}" ]]; then
    die "CP_PROVIDER_MSP_LICENSE_FILE is set but does not exist: ${license_file}
Leave it blank to run in evaluation mode (2 client workspaces), or place an
already issued license at that path. For a custom license, print the platform's
lease signing public key with ./setup.sh --print-lease-signing-public-key;
the license must bind that key or the control plane will refuse to start."
  fi
}

validate_compose_config() {
  log "validating compose config"
  # The same config supports both an installed licence and unlicensed
  # evaluation, before and after a best-effort evaluation request.
  (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose config --quiet)
}

pull_provider_images() {
  if truthy "${PULSE_PROVIDER_MSP_SKIP_PULL}"; then
    log "skipping image pull because PULSE_PROVIDER_MSP_SKIP_PULL=${PULSE_PROVIDER_MSP_SKIP_PULL}"
    return 0
  fi
  log "pulling provider MSP images"
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local key image_ref
  for key in TRAEFIK_IMAGE DOCKER_SOCKET_PROXY_IMAGE CONTROL_PLANE_IMAGE CP_PULSE_IMAGE; do
    image_ref="$(env_value "${key}" "${env_path}")"
    [[ "${image_ref}" == *@sha256:* ]] || die "${key} is not digest-pinned before image pull"
    docker pull "${image_ref}"
  done
}

start_provider_services() {
  # Leave the platform running. The next steps setup prints (bootstrap, then
  # the portal sign-in link) only work against a running control plane; a
  # setup that stopped at "prepared" handed a first-time provider a sign-in
  # link that answered 404 until something else happened to start it.
  log "starting provider MSP services"
  (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose up -d traefik docker-socket-proxy control-plane)

  local attempt
  for attempt in $(seq 1 30); do
    if (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose ps --services --status running 2>/dev/null) | grep -qx control-plane; then
      return 0
    fi
    sleep 2
  done
  die "control plane did not reach running state; inspect: cd ${PULSE_PROVIDER_MSP_INSTALL_DIR} && docker compose logs control-plane"
}

# What a provider must fix when Traefik reports an ACME failure, in their terms.
explain_acme_failure() {
  case "$1" in
    *invalidContact*)
      echo "Let's Encrypt refused ACME_EMAIL. Use a real address; example.com and similar are refused." ;;
    *"too many certificates"*sslip.io*|*"too many certificates"*nip.io*)
      echo "Let's Encrypt's weekly limit for trial addresses is used up. Everyone using sslip.io or nip.io shares it. It refills gradually, so retry later, or use your own domain." ;;
    *"too many failed authorizations"*|*"failed authorization"*)
      echo "Let's Encrypt paused this name after repeated failed checks; it allows 5 an hour. Fix the cause, wait up to an hour, then retry." ;;
    *"Timeout during connect"*|*"Connection refused"*|*"Connection reset"*|*"acme:error:connection"*)
      echo "Let's Encrypt could not reach port 80 on this server. Open inbound TCP ports 80 and 443 in your cloud provider's firewall. Docker publishes them past ufw, so that firewall is the usual cause." ;;
    *"Invalid response from"*|*"acme:error:unauthorized"*)
      echo "Let's Encrypt reached port 80, but something other than this server answered. Check that the name points here and that no proxy or CDN sits in front." ;;
    *"looking up A "*|*"looking up AAAA "*)
      echo "Let's Encrypt could not look up this name. Check its A records point at this server." ;;
    *rateLimited*)
      echo "Let's Encrypt is rate limiting this domain. Wait for the time in the message, then retry." ;;
    *[Cc]loudflare*)
      echo "Cloudflare refused the DNS change. CF_DNS_API_TOKEN needs Zone / Zone / Read and Zone / DNS / Edit on the zone that holds DOMAIN." ;;
    *NXDOMAIN*|*"time limit exceeded"*|*propagat*)
      echo "The DNS challenge record was not visible in time. Check that DOMAIN is in the zone the token can edit." ;;
    *)
      echo "Read the Traefik message below." ;;
  esac
}

# s_client normally prints the peer's certificate even when it is expired,
# untrusted or for another host. Require verification before reading its SANs,
# then match the whole name, not a different domain's prefix.
portal_certificate_verified() {
  local domain="$1" required_san="$2" certificate certificate_sans
  certificate="$(timeout 10 openssl s_client -connect 127.0.0.1:443 -servername "${domain}" \
    -verify_return_error -verify_hostname "${domain}" </dev/null 2>/dev/null)" \
    && certificate_sans="$(printf '%s\n' "${certificate}" | openssl x509 -noout -ext subjectAltName 2>/dev/null)" \
    && printf '%s\n' "${certificate_sans}" | tr ',' '\n' \
      | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | grep -qxF "${required_san}"
}

latest_acme_error() {
  (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose logs --no-color "$@" traefik 2>/dev/null) \
    | grep -F "Unable to obtain ACME certificate" | tail -n 1 \
    | sed -e 's/\x1b\[[0-9;]*m//g' -e 's/.*error=//' -e 's/ ACME CA=.*//' -e 's/^"//' -e 's/"$//' \
    | cut -c1-400 || true
}

# When a shared service's weekly limit is used up, new orders succeed only as it
# refills, and Traefik does not retry a refused order on its own. Keep asking,
# then try the other service. Renaming is safe only on a first run, before any
# client or agent has used the name.
TRIAL_CERTIFICATE_ATTEMPTS="${TRIAL_CERTIFICATE_ATTEMPTS:-6}"
TRIAL_CERTIFICATE_RETRY_SECONDS="${TRIAL_CERTIFICATE_RETRY_SECONDS:-30}"
FIRST_RUN_ENV_CREATED=0
retry_trial_certificate() {
  local env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  local attempt started service ip_label current=sslip.io other=nip.io
  if [[ "${domain}" == *.nip.io ]]; then
    current=nip.io
    other=sslip.io
  fi
  for service in "${current}" "${other}"; do
    if [[ "${service}" != "${current}" ]]; then
      [[ "${FIRST_RUN_ENV_CREATED}" -eq 1 ]] || return 0
      ip_label="${domain#pulse.}"
      ip_label="${ip_label%%.*}"
      domain="pulse.${ip_label}.${service}"
      required_san="DNS:${domain}"
      log "trying the same trial address on ${service}: ${domain}"
      set_env_value DOMAIN "${domain}" "${env_path}"
    fi
    for (( attempt = 1; attempt <= TRIAL_CERTIFICATE_ATTEMPTS; attempt++ )); do
      started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      log "shared trial limit reached; asking again (${service}, attempt ${attempt} of ${TRIAL_CERTIFICATE_ATTEMPTS})"
      (cd "${PULSE_PROVIDER_MSP_INSTALL_DIR}" && docker compose up -d --force-recreate traefik control-plane >/dev/null 2>&1) || true
      sleep "${TRIAL_CERTIFICATE_RETRY_SECONDS}"
      if portal_certificate_verified "${domain}" "${required_san}"; then
        PORTAL_CERTIFICATE_READY=1
        acme_error=""
        return 0
      fi
      acme_error="$(latest_acme_error --since "${started}")"
      [[ "${acme_error}" == *"too many certificates"* || -z "${acme_error}" ]] || return 0
    done
  done
}

# A running stack is not a usable one: until its certificate exists, Traefik
# refuses TLS for that name (sniStrict), so browsers and agents see only a
# handshake error. Wait for it, check the two DNS names, and say what to fix.
# Setup still finishes either way.
PORTAL_CERTIFICATE_READY=0
PORTAL_READINESS_REPORT=""
check_portal_readiness() {
  local env_path domain waited acme_error="" reason name missing=() missing_list
  local required_san certificate_scope
  env_path="${PULSE_PROVIDER_MSP_INSTALL_DIR}/.env"
  domain="$(env_value DOMAIN "${env_path}")"
  [[ "${PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS}" =~ ^[0-9]+$ ]] \
    || die "PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS must be a whole number of seconds"

  # HTTP-01 cannot issue a wildcard: clients get their own certificates later.
  if per_client_certificates; then
    required_san="DNS:${domain}"
    certificate_scope="${domain}"
  else
    required_san="DNS:*.${domain}"
    certificate_scope="${domain} and *.${domain}"
  fi

  log "waiting up to ${PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS}s for the certificate for ${certificate_scope}"
  for (( waited = 0; waited <= PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS; waited += 5 )); do
    if portal_certificate_verified "${domain}" "${required_san}"; then
      PORTAL_CERTIFICATE_READY=1
      break
    fi
    acme_error="$(latest_acme_error)"
    [[ -n "${acme_error}" ]] && break
    sleep 5
  done

  if [[ "${PORTAL_CERTIFICATE_READY}" -ne 1 && "${acme_error}" == *"too many certificates"* ]] && trial_address_install; then
    retry_trial_certificate
    certificate_scope="${domain}"
  fi

  for name in "${domain}" "pulse-dns-check.${domain}"; do
    timeout 10 getent ahostsv4 "${name}" >/dev/null 2>&1 || missing+=("${name}")
  done

  if [[ "${PORTAL_CERTIFICATE_READY}" -eq 1 && "${#missing[@]}" -eq 0 ]]; then
    PORTAL_READINESS_REPORT="Certificate: verified on this host for ${certificate_scope}."
    if per_client_certificates; then
      PORTAL_READINESS_REPORT+=$'\n'"Each client gets its own certificate from Let's Encrypt when you add it."
    fi
  else
    PORTAL_READINESS_REPORT="NOT READY YET:"
    if [[ "${PORTAL_CERTIFICATE_READY}" -ne 1 ]]; then
      if [[ -n "${acme_error}" ]]; then
        reason="$(explain_acme_failure "${acme_error}")"
      else
        reason="No trusted, current certificate covering ${certificate_scope} after ${PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS}s. Check Traefik's ACME logs and this host's clock; issuance may still be pending."
      fi
      PORTAL_READINESS_REPORT+=$'\n'"  - No verified TLS certificate yet; browsers and agents may refuse the connection."
      PORTAL_READINESS_REPORT+=$'\n'"    ${reason}"
      [[ -n "${acme_error}" ]] && PORTAL_READINESS_REPORT+=$'\n'"    Traefik said: ${acme_error}"
    fi
    if [[ "${#missing[@]}" -ne 0 ]]; then
      # setup.sh sets IFS to newline and tab, so join the names by hand.
      missing_list="$(printf '%s, ' "${missing[@]}")"
      if trial_address_install; then
        PORTAL_READINESS_REPORT+=$'\n'"  - This host cannot resolve ${missing_list%, }. Its DNS resolver may block shared trial"
        PORTAL_READINESS_REPORT+=$'\n'"    address services. Browsers and agents need to resolve them too."
      else
        PORTAL_READINESS_REPORT+=$'\n'"  - This host cannot resolve ${missing_list%, }. Browsers and agents need A records"
        PORTAL_READINESS_REPORT+=$'\n'"    for ${domain} and *.${domain} pointing here (on Cloudflare, DNS only)."
      fi
    fi
    PORTAL_READINESS_REPORT+=$'\n'"  Correct any wrong value with: sudo nano ${env_path}"
    PORTAL_READINESS_REPORT+=$'\n'"  then retry with: cd ${PULSE_PROVIDER_MSP_INSTALL_DIR} && sudo docker compose up -d --force-recreate traefik control-plane"
  fi
  printf '\n%s\n\n' "${PORTAL_READINESS_REPORT}"
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
  local domain data_dir network trial_note=""
  domain="$(env_value DOMAIN "${env_path}")"
  data_dir="$(provider_data_dir)"
  network="$(provider_docker_network)"
  if trial_address_install; then
    trial_note="TRIAL ONLY: ${domain} is a shared trial address for evaluating Pulse.
Do not build on it. It stops working if this server's IP changes, new clients
depend on a Let's Encrypt limit shared with everyone using these services, and
moving to your own domain later means reinstalling every agent.

"
  fi

  cat <<EOF

Pulse Provider MSP is running.

${trial_note}${PORTAL_READINESS_REPORT}

Paths:
  - Deploy dir: ${PULSE_PROVIDER_MSP_INSTALL_DIR}
  - Data dir:   ${data_dir}
  - Network:    ${network}

Next step: create your operator account (prints your portal sign-in link):
  cd ${PULSE_PROVIDER_MSP_INSTALL_DIR}
  sudo docker compose run --rm control-plane provider-msp bootstrap \\
    --account-name "Example MSP" --owner-email owner@example.com

Then open the link, choose Add client, and follow Install agents.

Prove the platform before the first real client:
  sudo ./run-install-proof.sh --account-name "Example MSP" --owner-email owner@example.com

Portal (after bootstrap):
  https://${domain}/portal

Plan: check Plan in the portal for this deployment's licence status and whether
a paid upgrade is available. A successfully issued evaluation covers two clients;
a running portal alone does not confirm active client MSP capabilities. Keep
within two clients until Plan confirms a higher active limit. A paid plan exposes
Manage billing for changes or cancellation.

Day 2: portal sessions last 7 days. Re-run the bootstrap command above any
time to print a fresh owner sign-in link, or use
  sudo docker compose run --rm control-plane provider-msp portal-link --email you@example.com
for any invited teammate. Set RESEND_API_KEY in .env to enable emailed
sign-in links instead.

EOF
}

main() {
  need_root

  if [[ "${1:-}" == "--print-lease-signing-public-key" ]]; then
    ensure_env_file
    ensure_generated_secrets
    derive_lease_signing_public_key
    printf '\n'
    exit 0
  fi

  log "starting provider MSP first-time setup"
  collect_first_run_values
  apt_install apt-transport-https
  install_docker_ce
  install_ops_tools
  install_deploy_bundle
  ensure_env_file
  ensure_dns_credentials_file
  ensure_generated_secrets
  # After install_docker_ce, which provides the buildx used to read the
  # registry, and before validation, which requires the pins to be set.
  ensure_image_pins
  validate_env_file
  create_data_dirs
  ensure_docker_network
  block_container_metadata_service
  validate_compose_config
  pull_provider_images
  # Issue the evaluation only after the host is configured and the immutable
  # images are reachable. This makes an issued evaluation a useful activation
  # signal rather than a record created before setup can succeed.
  ensure_eval_license
  validate_compose_config
  start_provider_services
  check_portal_readiness
  run_install_proof_if_requested
  print_summary
}

main "$@"
