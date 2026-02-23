#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INTEGRATION_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PVE_HOST="${PVE_HOST:-delly}"
PVE_USER="${PVE_USER:-root}"
PVE_TARGET="${PVE_USER}@${PVE_HOST}"
PVE_CTID="${PVE_CTID:-211}"
PVE_SNAPSHOT="${PVE_SNAPSHOT:-pre-eval-baseline}"

LOCAL_PULSE_PORT="${LOCAL_PULSE_PORT:-17655}"
LOCAL_CP_PORT="${LOCAL_CP_PORT:-18443}"

PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}"
PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}"
PULSE_E2E_BOOTSTRAP_TOKEN="${PULSE_E2E_BOOTSTRAP_TOKEN:-}"

MULTI_TENANT_SCENARIO="${MULTI_TENANT_SCENARIO:-multi-tenant}"
TRIAL_SCENARIO="${TRIAL_SCENARIO:-trial-signup}"
CLOUD_SCENARIOS="${CLOUD_SCENARIOS:-cloud-hosting,cloud-billing-lifecycle}"

PULSE_CP_ADMIN_KEY="${PULSE_CP_ADMIN_KEY:-}"
PULSE_E2E_STRIPE_API_KEY="${PULSE_E2E_STRIPE_API_KEY:-}"
PULSE_E2E_STRIPE_WEBHOOK_SECRET="${PULSE_E2E_STRIPE_WEBHOOK_SECRET:-}"
PULSE_E2E_CP_BINARY="${PULSE_E2E_CP_BINARY:-}"

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
    node ./scripts/run-evals.mjs --mode deterministic --scenario "${scenarios}"
  )
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

force_trial_expiry_on_container() {
  ssh_cmd "pct exec ${PVE_CTID} -- sh -lc '
set -eu
if [ ! -f /etc/pulse/billing.json ]; then
  echo \"missing /etc/pulse/billing.json\" >&2
  exit 1
fi
tmp=\$(mktemp)
jq '\''.subscription_state = \"trial\" | .trial_ends_at = ((now|floor)-7200) | .integrity = \"\"'\'' /etc/pulse/billing.json > \"\$tmp\"
mv \"\$tmp\" /etc/pulse/billing.json
systemctl restart pulse.service
'"
}

verify_trial_expired_via_api() {
  local tmp_dir
  tmp_dir="$(mktemp -d)"

  local cookies_file="${tmp_dir}/cookies.txt"
  local login_body="${tmp_dir}/login.json"
  local entitlements_body="${tmp_dir}/entitlements.json"

  local login_code
  login_code="$(
    curl -sS -o "${login_body}" -w '%{http_code}' \
      -c "${cookies_file}" \
      -H 'Content-Type: application/json' \
      --data "{\"username\":\"${PULSE_E2E_USERNAME}\",\"password\":\"${PULSE_E2E_PASSWORD}\"}" \
      "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/login"
  )"
  if [[ "${login_code}" != "200" ]]; then
    echo "ERROR: lifecycle login failed with HTTP ${login_code}" >&2
    exit 1
  fi

  local state=""
  local attempts=0
  while [[ "${attempts}" -lt 30 ]]; do
    attempts=$((attempts + 1))
    local code
    code="$(
      curl -sS -o "${entitlements_body}" -w '%{http_code}' \
        -b "${cookies_file}" \
        "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/license/entitlements"
    )"
    if [[ "${code}" == "200" ]]; then
      state="$(jq -r '.subscription_state // empty' "${entitlements_body}")"
      if [[ "${state}" == "expired" ]]; then
        break
      fi
    fi
    sleep 1
  done

  if [[ "${state}" != "expired" ]]; then
    echo "ERROR: expected subscription_state=expired after forced trial expiry" >&2
    cat "${entitlements_body}" >&2 || true
    exit 1
  fi

  local trial_eligible valid days_remaining has_autofix
  trial_eligible="$(jq -r 'if has("trial_eligible") then (.trial_eligible|tostring) else "" end' "${entitlements_body}")"
  valid="$(jq -r 'if has("valid") then (.valid|tostring) else "" end' "${entitlements_body}")"
  days_remaining="$(jq -r '.days_remaining // empty' "${entitlements_body}")"
  has_autofix="$(jq -r '(.capabilities // []) | index("ai_autofix") != null' "${entitlements_body}")"
  if [[ "${valid}" != "false" ]]; then
    echo "ERROR: expected valid=false after trial expiry, got ${valid}" >&2
    exit 1
  fi
  if [[ "${days_remaining}" != "0" ]]; then
    echo "ERROR: expected days_remaining=0 after trial expiry, got ${days_remaining}" >&2
    exit 1
  fi
  if [[ "${has_autofix}" != "false" ]]; then
    echo "ERROR: expected ai_autofix capability removed after trial expiry" >&2
    exit 1
  fi
  if [[ "${trial_eligible}" != "false" ]]; then
    echo "ERROR: expected trial_eligible=false after trial expiry, got ${trial_eligible}" >&2
    exit 1
  fi
  rm -rf "${tmp_dir}" >/dev/null 2>&1 || true
}

echo "[1/9] Discovering container IP"
discover_container_ip
echo "      CT ${PVE_CTID} IP: ${CT_IP}"

echo "[2/9] Rolling back to snapshot ${PVE_SNAPSHOT}"
rollback_and_start_services

echo "[3/9] Starting local SSH tunnel"
start_tunnel
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"

echo "[4/9] Loading control-plane secrets"
load_cloud_secrets

echo "[5/9] Seeding multi-tenant entitlement and running ${MULTI_TENANT_SCENARIO}"
seed_multi_tenant_entitlement
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
run_eval_scenarios "${MULTI_TENANT_SCENARIO}"

echo "[6/9] Restoring clean baseline and running ${TRIAL_SCENARIO}"
rollback_and_start_services
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"
run_eval_scenarios "${TRIAL_SCENARIO}"

echo "[7/9] Forcing trial expiry and validating downgrade contract"
force_trial_expiry_on_container
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
verify_trial_expired_via_api

echo "[8/9] Restoring clean baseline for cloud lifecycle"
rollback_and_start_services
enable_cp_dockerless_provisioning
wait_http_ready "http://127.0.0.1:${LOCAL_PULSE_PORT}/api/health"
wait_http_ready "http://127.0.0.1:${LOCAL_CP_PORT}/healthz"

echo "[9/9] Running ${CLOUD_SCENARIOS}"
run_eval_scenarios "${CLOUD_SCENARIOS}"

echo "All requested sandbox scenarios completed."
