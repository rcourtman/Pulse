#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INTEGRATION_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Require explicit sandbox targeting so private lab topology never becomes a
# committed default.
PVE_HOST="${PVE_HOST:?set PVE_HOST to the disposable PVE host}"
PVE_USER="${PVE_USER:-root}"
PVE_TARGET="${PVE_USER}@${PVE_HOST}"
PVE_CTID="${PVE_CTID:?set PVE_CTID to the disposable test container ID}"
PVE_SNAPSHOT="${PVE_SNAPSHOT:-pre-eval-baseline}"

LOCAL_PULSE_PORT="${LOCAL_PULSE_PORT:-17655}"
LOCAL_CP_PORT="${LOCAL_CP_PORT:-18443}"

PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}"
PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}"
PULSE_E2E_BOOTSTRAP_TOKEN="${PULSE_E2E_BOOTSTRAP_TOKEN:-}"

MULTI_TENANT_SCENARIO="${MULTI_TENANT_SCENARIO:-multi-tenant}"
TRIAL_SCENARIO="${TRIAL_SCENARIO:-trial-signup}"
CLOUD_SCENARIOS="${CLOUD_SCENARIOS:-cloud-hosting,cloud-billing-lifecycle}"
TRUENAS_SCENARIO="${TRUENAS_SCENARIO:-truenas-node-add}"
RELAY_SCENARIO="${RELAY_SCENARIO:-relay-pairing}"

PULSE_CP_ADMIN_KEY="${PULSE_CP_ADMIN_KEY:-}"
PULSE_E2E_STRIPE_API_KEY="${PULSE_E2E_STRIPE_API_KEY:-}"
PULSE_E2E_STRIPE_WEBHOOK_SECRET="${PULSE_E2E_STRIPE_WEBHOOK_SECRET:-}"
PULSE_E2E_CP_BINARY="${PULSE_E2E_CP_BINARY:-}"

# Infrastructure journey env vars (TrueNAS + relay)
PULSE_E2E_TRUENAS_HOST="${PULSE_E2E_TRUENAS_HOST:-}"
PULSE_E2E_TRUENAS_API_KEY="${PULSE_E2E_TRUENAS_API_KEY:-}"
PULSE_E2E_RELAY_HOST="${PULSE_E2E_RELAY_HOST:-}"

CT_IP=""
TUNNEL_PID=""
TUNNEL_LOG=""

ssh_cmd() {
  ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new "${PVE_TARGET}" "$@"
}

cleanup() {
  if [[ -n "${TUNNEL_PID}" ]] && kill -0 "${TUNNEL_PID}" >/dev/null 2>&1; then
    kill "${TUNNEL_PID}" >/dev/null 2>&1 || true
    wait "${TUNNEL_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${TUNNEL_LOG}" && -f "${TUNNEL_LOG}" ]]; then
    rm -f "${TUNNEL_LOG}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

discover_container_ip() {
  CT_IP="$(ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'hostname -I | tr -s \" \" \"\\n\" | sed -n \"1p\"'")"
  CT_IP="$(echo "${CT_IP}" | tr -d '[:space:]')"
  if [[ -z "${CT_IP}" ]]; then
    echo "ERROR: failed to resolve container IP for CT ${PVE_CTID}" >&2
    exit 1
  fi
}

rollback_and_start_services() {
  ssh_cmd "pct rollback ${PVE_CTID} ${PVE_SNAPSHOT} --start 1 >/dev/null"
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'systemctl start pulse.service pulse-control-plane.service && systemctl is-active pulse.service pulse-control-plane.service >/dev/null'"
  sync_control_plane_binary
}

sync_control_plane_binary() {
  if [[ -z "${PULSE_E2E_CP_BINARY}" ]]; then
    return 0
  fi
  if [[ ! -f "${PULSE_E2E_CP_BINARY}" ]]; then
    echo "ERROR: PULSE_E2E_CP_BINARY does not exist: ${PULSE_E2E_CP_BINARY}" >&2
    exit 1
  fi

  local remote_tmp="/tmp/pulse-control-plane-${PVE_CTID}-$$"
  scp -q -o BatchMode=yes -o StrictHostKeyChecking=accept-new "${PULSE_E2E_CP_BINARY}" "${PVE_TARGET}:${remote_tmp}"
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'systemctl stop pulse-control-plane.service'"
  ssh_cmd "pct push ${PVE_CTID} ${remote_tmp} /opt/pulse-test/bin/pulse-control-plane --perms 0755 >/dev/null"
  ssh_cmd "rm -f ${remote_tmp}"
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'systemctl start pulse-control-plane.service && systemctl is-active pulse-control-plane.service >/dev/null'"
}

enable_cp_dockerless_provisioning() {
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc '
set -eu
env_file=/opt/pulse-cloud-test/.env
if [ ! -f \"\$env_file\" ]; then
  echo \"missing \$env_file\" >&2
  exit 1
fi
if grep -q \"^CP_ALLOW_DOCKERLESS_PROVISIONING=\" \"\$env_file\"; then
  sed -i '\''s/^CP_ALLOW_DOCKERLESS_PROVISIONING=.*/CP_ALLOW_DOCKERLESS_PROVISIONING=true/'\'' \"\$env_file\"
else
  printf \"\\nCP_ALLOW_DOCKERLESS_PROVISIONING=true\\n\" >> \"\$env_file\"
fi
systemctl restart pulse-control-plane.service
systemctl is-active pulse-control-plane.service >/dev/null
'"
}

wait_http_ready() {
  local url="$1"
  local max_attempts="${2:-90}"
  local attempt=0
  while (( attempt < max_attempts )); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "ERROR: timed out waiting for ${url}" >&2
  return 1
}

start_tunnel() {
  TUNNEL_LOG="$(mktemp -t pulse-lxc-tunnel.XXXXXX.log)"
  ssh \
    -o BatchMode=yes \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=20 \
    -o ServerAliveCountMax=3 \
    -N \
    -L "${LOCAL_PULSE_PORT}:${CT_IP}:7655" \
    -L "${LOCAL_CP_PORT}:${CT_IP}:8443" \
    "${PVE_TARGET}" 2>"${TUNNEL_LOG}" &
  TUNNEL_PID=$!
}

seed_multi_tenant_entitlement() {
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'cat > /etc/pulse/billing.json <<\"JSON\"
{\"capabilities\":[\"multi_tenant\",\"update_alerts\",\"sso\",\"ai_patrol\"],\"limits\":{},\"meters_enabled\":[],\"plan_version\":\"enterprise_eval\",\"subscription_state\":\"active\",\"integrity\":\"\"}
JSON
systemctl restart pulse.service
'"
}

run_eval_scenarios() {
  local scenarios="$1"
  (
    cd "${INTEGRATION_ROOT}"
    PULSE_E2E_SKIP_DOCKER=1 \
    PULSE_BASE_URL="http://127.0.0.1:${LOCAL_PULSE_PORT}" \
    PULSE_CLOUD_BASE_URL="http://127.0.0.1:${LOCAL_CP_PORT}" \
    PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME}" \
    PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD}" \
    PULSE_E2E_BOOTSTRAP_TOKEN="${PULSE_E2E_BOOTSTRAP_TOKEN}" \
    PULSE_CP_ADMIN_KEY="${PULSE_CP_ADMIN_KEY}" \
    PULSE_E2E_STRIPE_API_KEY="${PULSE_E2E_STRIPE_API_KEY}" \
    PULSE_E2E_STRIPE_WEBHOOK_SECRET="${PULSE_E2E_STRIPE_WEBHOOK_SECRET}" \
    PULSE_E2E_LXC_SSH_TARGET="${PVE_TARGET}" \
    PULSE_E2E_LXC_CTID="${PVE_CTID}" \
    PULSE_E2E_TRUENAS_HOST="${PULSE_E2E_TRUENAS_HOST}" \
    PULSE_E2E_TRUENAS_API_KEY="${PULSE_E2E_TRUENAS_API_KEY}" \
    PULSE_E2E_RELAY_HOST="${PULSE_E2E_RELAY_HOST}" \
    node ./scripts/run-evals.mjs --mode deterministic --scenario "${scenarios}"
  )
}

seed_infra_entitlement() {
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc 'cat > /etc/pulse/billing.json <<\"JSON\"
{\"capabilities\":[\"relay\",\"update_alerts\",\"advanced_reporting\",\"audit_logging\",\"ai_patrol\"],\"limits\":{},\"meters_enabled\":[],\"plan_version\":\"pro_eval\",\"subscription_state\":\"active\",\"integrity\":\"\"}
JSON
systemctl restart pulse.service
'"
}

load_cloud_secrets() {
  local raw
  raw="$(ssh_cmd "pct exec ${PVE_CTID} -- sh -lc '. /opt/pulse-cloud-test/.env >/dev/null 2>&1 || true; printf \"%s\\n%s\\n%s\\n\" \"\$CP_ADMIN_KEY\" \"\$STRIPE_API_KEY\" \"\$STRIPE_WEBHOOK_SECRET\"'")"
  local line1 line2 line3
  line1="$(printf '%s\n' "${raw}" | sed -n '1p' | tr -d '\r')"
  line2="$(printf '%s\n' "${raw}" | sed -n '2p' | tr -d '\r')"
  line3="$(printf '%s\n' "${raw}" | sed -n '3p' | tr -d '\r')"

  if [[ -z "${PULSE_CP_ADMIN_KEY}" ]]; then
    PULSE_CP_ADMIN_KEY="${line1}"
  fi
  if [[ -z "${PULSE_E2E_STRIPE_API_KEY}" ]]; then
    PULSE_E2E_STRIPE_API_KEY="${line2}"
  fi
  if [[ -z "${PULSE_E2E_STRIPE_WEBHOOK_SECRET}" ]]; then
    PULSE_E2E_STRIPE_WEBHOOK_SECRET="${line3}"
  fi

  if [[ -z "${PULSE_CP_ADMIN_KEY}" || -z "${PULSE_E2E_STRIPE_API_KEY}" || -z "${PULSE_E2E_STRIPE_WEBHOOK_SECRET}" ]]; then
    echo "ERROR: failed to resolve CP admin/Stripe secrets from /opt/pulse-cloud-test/.env" >&2
    exit 1
  fi
}

TOTAL_STEPS=8
INFRA_JOURNEYS_AVAILABLE=false
# TrueNAS requires both host AND API key; relay requires host.
# At least one fully-configured journey must be present to add infra steps.
TRUENAS_READY=false
RELAY_READY=false
if [[ -n "${PULSE_E2E_TRUENAS_HOST}" && -n "${PULSE_E2E_TRUENAS_API_KEY}" ]]; then
  TRUENAS_READY=true
fi
if [[ -n "${PULSE_E2E_RELAY_HOST}" ]]; then
  RELAY_READY=true
fi
if [[ "${TRUENAS_READY}" == "true" || "${RELAY_READY}" == "true" ]]; then
  INFRA_JOURNEYS_AVAILABLE=true
  TOTAL_STEPS=10
fi

echo "[1/${TOTAL_STEPS}] Discovering container IP"
discover_container_ip
echo "      CT ${PVE_CTID} IP: ${CT_IP}"

echo "[2/${TOTAL_STEPS}] Rolling back to snapshot ${PVE_SNAPSHOT}"
rollback_and_start_services

echo "[3/${TOTAL_STEPS}] Starting local SSH tunnel"
start_tunnel
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"

echo "[4/${TOTAL_STEPS}] Loading control-plane secrets"
load_cloud_secrets

echo "[5/${TOTAL_STEPS}] Seeding multi-tenant entitlement and running ${MULTI_TENANT_SCENARIO}"
seed_multi_tenant_entitlement
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
run_eval_scenarios "${MULTI_TENANT_SCENARIO}"

echo "[6/${TOTAL_STEPS}] Restoring clean baseline and running ${TRIAL_SCENARIO}"
rollback_and_start_services
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"
run_eval_scenarios "${TRIAL_SCENARIO}"

echo "[7/${TOTAL_STEPS}] Restoring clean baseline for cloud lifecycle"
rollback_and_start_services
enable_cp_dockerless_provisioning
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"

echo "[8/${TOTAL_STEPS}] Running ${CLOUD_SCENARIOS}"
run_eval_scenarios "${CLOUD_SCENARIOS}"

# --- Infrastructure journeys (TrueNAS + relay) ---
# These require additional env vars (PULSE_E2E_TRUENAS_HOST, PULSE_E2E_RELAY_HOST).
# When not configured, the Playwright specs skip gracefully via test.skip() guards.

if [[ "${INFRA_JOURNEYS_AVAILABLE}" == "true" ]]; then
  INFRA_SCENARIOS=""
  if [[ "${TRUENAS_READY}" == "true" ]]; then
    INFRA_SCENARIOS="${TRUENAS_SCENARIO}"
  fi
  if [[ "${RELAY_READY}" == "true" ]]; then
    INFRA_SCENARIOS="${INFRA_SCENARIOS:+${INFRA_SCENARIOS},}${RELAY_SCENARIO}"
  fi

  echo "[10/${TOTAL_STEPS}] Restoring clean baseline for infrastructure journeys"
  echo "      TrueNAS ready: ${TRUENAS_READY}, Relay ready: ${RELAY_READY}"
  rollback_and_start_services
  seed_infra_entitlement
  wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"

  echo "[11/${TOTAL_STEPS}] Running infrastructure journeys (${INFRA_SCENARIOS})"
  run_eval_scenarios "${INFRA_SCENARIOS}"
fi

echo "All requested sandbox scenarios completed."
