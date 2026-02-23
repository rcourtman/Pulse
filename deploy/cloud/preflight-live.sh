#!/usr/bin/env bash

set -euo pipefail

DOMAIN="${DOMAIN:-cloud.pulserelay.pro}"
SSH_TARGET="${SSH_TARGET:-root@pulse-cloud}"
ADMIN_KEY_FILE="${ADMIN_KEY_FILE:-${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/admin_key}"
BACKUP_MAX_AGE_SECONDS="${BACKUP_MAX_AGE_SECONDS:-172800}" # 48h
CHECK_TRIAL_SIGNUP_ROUTES="${CHECK_TRIAL_SIGNUP_ROUTES:-true}"
CHECK_PUBLIC_SIGNUP_ROUTES="${CHECK_PUBLIC_SIGNUP_ROUTES:-true}"
CHECK_STRIPE_CHECKOUT_PROBE="${CHECK_STRIPE_CHECKOUT_PROBE:-false}"
EXPECT_CP_ENV="${EXPECT_CP_ENV:-production}"
EXPECT_STRIPE_MODE="${EXPECT_STRIPE_MODE:-}"

FAILURES=0
WARNINGS=0

pass() {
  echo "[PASS] $*"
}

warn() {
  echo "[WARN] $*"
  WARNINGS=$((WARNINGS + 1))
}

fail() {
  echo "[FAIL] $*"
  FAILURES=$((FAILURES + 1))
}

info() {
  echo "[INFO] $*"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    fail "Missing required command: ${cmd}"
  fi
}

http_code() {
  local method="$1"
  local url="$2"
  shift 2
  curl -sS -o /dev/null -w "%{http_code}" -X "${method}" "$@" "${url}"
}

ssh_run() {
  local remote_cmd="$1"
  ssh -o BatchMode=yes -o ConnectTimeout=10 "${SSH_TARGET}" "${remote_cmd}"
}

check_local_http() {
  info "Running public endpoint checks against https://${DOMAIN}"

  local health_code ready_code webhook_code
  health_code="$(http_code GET "https://${DOMAIN}/healthz")"
  ready_code="$(http_code GET "https://${DOMAIN}/readyz")"
  webhook_code="$(http_code POST "https://${DOMAIN}/api/stripe/webhook")"

  if [[ "${health_code}" == "200" ]]; then
    pass "/healthz returns 200"
  else
    fail "/healthz expected 200, got ${health_code}"
  fi

  if [[ "${ready_code}" == "200" ]]; then
    pass "/readyz returns 200"
  else
    fail "/readyz expected 200, got ${ready_code}"
  fi

  if [[ "${webhook_code}" == "400" ]]; then
    pass "/api/stripe/webhook rejects unsigned requests (400)"
  else
    fail "/api/stripe/webhook unsigned request expected 400, got ${webhook_code}"
  fi
}

check_trial_signup_routes() {
  if [[ "${CHECK_TRIAL_SIGNUP_ROUTES}" != "true" ]]; then
    info "Skipping trial-signup route checks (CHECK_TRIAL_SIGNUP_ROUTES=${CHECK_TRIAL_SIGNUP_ROUTES})"
    return
  fi

  info "Checking trial-signup route contract"

  local start_url checkout_get_code checkout_post_code start_code
  start_url="https://${DOMAIN}/start-pro-trial?org_id=default&return_url=https%3A%2F%2Fexample.com%2Fauth%2Ftrial-activate"

  start_code="$(http_code GET "${start_url}")"
  checkout_get_code="$(http_code GET "https://${DOMAIN}/api/trial-signup/checkout")"
  checkout_post_code="$(http_code POST "https://${DOMAIN}/api/trial-signup/checkout" -H "Content-Type: application/x-www-form-urlencoded" --data "org_id=default&return_url=not-a-url")"

  if [[ "${start_code}" == "200" ]]; then
    pass "/start-pro-trial returns 200 with required query params"
  else
    fail "/start-pro-trial expected 200, got ${start_code}"
  fi
  if [[ "${checkout_get_code}" == "405" ]]; then
    pass "GET /api/trial-signup/checkout returns 405"
  else
    fail "GET /api/trial-signup/checkout expected 405, got ${checkout_get_code}"
  fi
  if [[ "${checkout_post_code}" == "400" ]]; then
    pass "POST /api/trial-signup/checkout rejects invalid input with 400"
  else
    fail "POST /api/trial-signup/checkout invalid input expected 400, got ${checkout_post_code}"
  fi
}

check_public_signup_routes() {
  if [[ "${CHECK_PUBLIC_SIGNUP_ROUTES}" != "true" ]]; then
    info "Skipping public signup route checks (CHECK_PUBLIC_SIGNUP_ROUTES=${CHECK_PUBLIC_SIGNUP_ROUTES})"
    return
  fi

  info "Checking public cloud-signup route contract"

  local signup_code cloud_signup_code complete_code api_get_code api_post_bad_code magic_bad_code magic_ok_code
  signup_code="$(http_code GET "https://${DOMAIN}/signup")"
  cloud_signup_code="$(http_code GET "https://${DOMAIN}/cloud/signup")"
  complete_code="$(http_code GET "https://${DOMAIN}/signup/complete")"
  api_get_code="$(http_code GET "https://${DOMAIN}/api/public/signup")"
  api_post_bad_code="$(http_code POST "https://${DOMAIN}/api/public/signup" -H "Content-Type: application/json" --data '{"email":"not-an-email","org_name":"Acme"}')"
  magic_bad_code="$(http_code POST "https://${DOMAIN}/api/public/magic-link/request" -H "Content-Type: application/json" --data '{"email":"not-an-email"}')"
  magic_ok_code="$(http_code POST "https://${DOMAIN}/api/public/magic-link/request" -H "Content-Type: application/json" --data '{"email":"owner@example.com"}')"

  if [[ "${signup_code}" == "200" ]]; then
    pass "/signup returns 200"
  else
    fail "/signup expected 200, got ${signup_code}"
  fi
  if [[ "${cloud_signup_code}" == "200" ]]; then
    pass "/cloud/signup returns 200"
  else
    fail "/cloud/signup expected 200, got ${cloud_signup_code}"
  fi
  if [[ "${complete_code}" == "200" ]]; then
    pass "/signup/complete returns 200"
  else
    fail "/signup/complete expected 200, got ${complete_code}"
  fi
  if [[ "${api_get_code}" == "405" ]]; then
    pass "GET /api/public/signup returns 405"
  else
    fail "GET /api/public/signup expected 405, got ${api_get_code}"
  fi
  if [[ "${api_post_bad_code}" == "400" ]]; then
    pass "POST /api/public/signup rejects invalid input with 400"
  else
    fail "POST /api/public/signup invalid input expected 400, got ${api_post_bad_code}"
  fi
  if [[ "${magic_bad_code}" == "400" ]]; then
    pass "POST /api/public/magic-link/request rejects invalid email with 400"
  else
    fail "POST /api/public/magic-link/request invalid email expected 400, got ${magic_bad_code}"
  fi
  if [[ "${magic_ok_code}" == "200" ]]; then
    pass "POST /api/public/magic-link/request returns 200 for valid payload"
  else
    fail "POST /api/public/magic-link/request valid payload expected 200, got ${magic_ok_code}"
  fi
}

check_stripe_checkout_probe() {
  if [[ "${CHECK_STRIPE_CHECKOUT_PROBE}" != "true" ]]; then
    info "Skipping Stripe checkout probe (CHECK_STRIPE_CHECKOUT_PROBE=${CHECK_STRIPE_CHECKOUT_PROBE})"
    return
  fi

  info "Running Stripe checkout probe via /api/public/signup"

  local tmp_file probe_email probe_org code
  tmp_file="$(mktemp)"
  probe_email="preflight+$(date -u +%s)-$RANDOM@example.com"
  probe_org="Pulse Cloud Checkout Probe"

  code="$(curl -sS -o "${tmp_file}" -w "%{http_code}" \
    -X POST "https://${DOMAIN}/api/public/signup" \
    -H "Content-Type: application/json" \
    --data "{\"email\":\"${probe_email}\",\"org_name\":\"${probe_org}\"}")"

  if [[ "${code}" != "201" ]]; then
    fail "Stripe checkout probe expected 201, got ${code} (body=$(cat "${tmp_file}"))"
    rm -f "${tmp_file}"
    return
  fi

  if grep -q '"checkout_url":"https://checkout\.stripe\.com/' "${tmp_file}"; then
    pass "Stripe checkout probe returned checkout_url"
  else
    fail "Stripe checkout probe response missing Stripe checkout_url (body=$(cat "${tmp_file}"))"
  fi
  rm -f "${tmp_file}"
}

check_status_endpoint() {
  if [[ ! -r "${ADMIN_KEY_FILE}" ]]; then
    fail "Admin key file not readable: ${ADMIN_KEY_FILE}"
    return
  fi

  local admin_key
  admin_key="$(tr -d '\r\n' <"${ADMIN_KEY_FILE}")"
  if [[ -z "${admin_key}" ]]; then
    fail "Admin key file is empty: ${ADMIN_KEY_FILE}"
    return
  fi

  local status_json
  if ! status_json="$(curl -fsS "https://${DOMAIN}/status" -H "X-Admin-Key: ${admin_key}")"; then
    fail "/status request failed with admin key"
    return
  fi

  if grep -q '"total_tenants"' <<<"${status_json}" && grep -q '"healthy"' <<<"${status_json}"; then
    pass "/status is reachable with admin key"
    info "Status payload: ${status_json}"
  else
    fail "/status payload missing expected fields"
  fi
}

check_ssh_connectivity() {
  if ssh_run "hostname >/dev/null 2>&1"; then
    pass "SSH connectivity to ${SSH_TARGET}"
  else
    fail "Unable to SSH to ${SSH_TARGET}"
  fi
}

check_remote_env_and_compose() {
  info "Checking remote compose/env guardrails"

  local env_file="/opt/pulse-cloud/.env"
  local compose_file="/opt/pulse-cloud/docker-compose.yml"

  if ssh_run "[ -f '${env_file}' ]"; then
    pass "Remote env file exists (${env_file})"
  else
    fail "Remote env file missing (${env_file})"
    return
  fi

  if ssh_run "[ -f '${compose_file}' ]"; then
    pass "Remote compose file exists (${compose_file})"
  else
    fail "Remote compose file missing (${compose_file})"
    return
  fi

  local required_env=(
    "DOMAIN"
    "ACME_EMAIL"
    "CP_ADMIN_KEY"
    "CP_PULSE_IMAGE"
    "CP_TRIAL_SIGNUP_PRICE_ID"
    "STRIPE_WEBHOOK_SECRET"
    "STRIPE_API_KEY"
    "RESEND_API_KEY"
    "CP_ENV"
    "CP_ALLOW_DOCKERLESS_PROVISIONING"
    "CP_REQUIRE_EMAIL_PROVIDER"
  )

  local key
  for key in "${required_env[@]}"; do
    if ssh_run "grep -qE '^${key}=' '${env_file}'"; then
      pass "Env key present: ${key}"
    else
      fail "Env key missing: ${key}"
    fi
  done

  local cp_env cp_dockerless cp_require_email stripe_api_key stripe_mode expected_cp_env expected_stripe_mode
  cp_env="$(ssh_run "grep -E '^CP_ENV=' '${env_file}' | head -n1 | cut -d= -f2-")"
  cp_dockerless="$(ssh_run "grep -E '^CP_ALLOW_DOCKERLESS_PROVISIONING=' '${env_file}' | head -n1 | cut -d= -f2-")"
  cp_require_email="$(ssh_run "grep -E '^CP_REQUIRE_EMAIL_PROVIDER=' '${env_file}' | head -n1 | cut -d= -f2-")"
  stripe_api_key="$(ssh_run "grep -E '^STRIPE_API_KEY=' '${env_file}' | head -n1 | cut -d= -f2-")"

  expected_cp_env="$(echo "${EXPECT_CP_ENV}" | tr '[:upper:]' '[:lower:]')"
  if [[ "${expected_cp_env}" != "production" && "${expected_cp_env}" != "staging" ]]; then
    fail "EXPECT_CP_ENV must be production or staging (got '${EXPECT_CP_ENV}')"
  elif [[ "${cp_env}" == "${expected_cp_env}" ]]; then
    pass "CP_ENV=${cp_env}"
  else
    fail "CP_ENV should be ${expected_cp_env} (got '${cp_env}')"
  fi
  [[ "${cp_dockerless}" == "false" ]] && pass "CP_ALLOW_DOCKERLESS_PROVISIONING=false" || fail "CP_ALLOW_DOCKERLESS_PROVISIONING should be false (got '${cp_dockerless}')"
  [[ "${cp_require_email}" == "true" ]] && pass "CP_REQUIRE_EMAIL_PROVIDER=true" || fail "CP_REQUIRE_EMAIL_PROVIDER should be true (got '${cp_require_email}')"

  expected_stripe_mode="$(echo "${EXPECT_STRIPE_MODE}" | tr '[:upper:]' '[:lower:]')"
  if [[ -z "${expected_stripe_mode}" ]]; then
    if [[ "${expected_cp_env}" == "staging" ]]; then
      expected_stripe_mode="test"
    else
      expected_stripe_mode="live"
    fi
  fi
  case "${expected_stripe_mode}" in
    test|live)
      ;;
    *)
      fail "EXPECT_STRIPE_MODE must be test or live (got '${EXPECT_STRIPE_MODE}')"
      expected_stripe_mode=""
      ;;
  esac
  stripe_mode="unknown"
  if [[ "${stripe_api_key}" == sk_test_* ]]; then
    stripe_mode="test"
  elif [[ "${stripe_api_key}" == sk_live_* ]]; then
    stripe_mode="live"
  fi
  if [[ -n "${expected_stripe_mode}" && "${stripe_mode}" == "${expected_stripe_mode}" ]]; then
    pass "STRIPE_API_KEY mode is ${stripe_mode}"
  elif [[ -n "${expected_stripe_mode}" ]]; then
    fail "STRIPE_API_KEY mode mismatch: expected ${expected_stripe_mode}, got ${stripe_mode}"
  fi

  local running_services
  running_services="$(ssh_run "docker compose -f '${compose_file}' ps --status running --services || true")"
  if grep -qx "traefik" <<<"${running_services}" && grep -qx "control-plane" <<<"${running_services}"; then
    pass "Required services are running (traefik, control-plane)"
  else
    fail "Missing running compose services. Running list: ${running_services:-<none>}"
  fi

  local image_refs floating_refs
  image_refs="$(ssh_run "cd /opt/pulse-cloud && docker compose -f '${compose_file}' config 2>/dev/null | awk '/^[[:space:]]*image:/ {print \$2}'")"
  if [[ -z "${image_refs}" ]]; then
    warn "Unable to resolve compose image refs via docker compose config"
  else
    floating_refs="$(grep -Ev '(@sha256:|^sha256:)' <<<"${image_refs}" || true)"
    if [[ -n "${floating_refs}" ]]; then
      warn "Compose uses non-digest image refs: ${floating_refs//$'\n'/, }"
    else
      pass "Compose images are digest-pinned"
    fi
  fi
}

check_remote_backup_state() {
  info "Checking backup and restore readiness"

  local backup_script="/opt/pulse-cloud/backup.sh"
  local restore_script="/opt/pulse-cloud/restore-drill.sh"
  local metrics_file="/var/lib/node_exporter/textfile_collector/pulse_cloud_backup.prom"

  ssh_run "[ -x '${backup_script}' ]" && pass "Backup script is executable (${backup_script})" || fail "Backup script missing/not executable (${backup_script})"
  ssh_run "[ -x '${restore_script}' ]" && pass "Restore drill script is executable (${restore_script})" || fail "Restore drill script missing/not executable (${restore_script})"

  if ssh_run "crontab -l 2>/dev/null | grep -q '/opt/pulse-cloud/backup.sh'"; then
    pass "Backup cron entry exists"
  else
    fail "Backup cron entry is missing"
  fi

  local latest_day
  latest_day="$(ssh_run "ls -1 /data/backups/daily 2>/dev/null | grep -E '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' | sort | tail -n1 || true")"
  if [[ -z "${latest_day}" ]]; then
    fail "No dated backup snapshots found in /data/backups/daily"
  else
    pass "Latest dated backup snapshot: ${latest_day}"
    local backup_mtime now age_seconds
    backup_mtime="$(ssh_run "stat -c %Y '/data/backups/daily/${latest_day}'")"
    now="$(date -u +%s)"
    age_seconds=$((now - backup_mtime))
    if (( age_seconds <= BACKUP_MAX_AGE_SECONDS )); then
      pass "Latest backup snapshot age is ${age_seconds}s (within ${BACKUP_MAX_AGE_SECONDS}s)"
    else
      fail "Latest backup snapshot age is ${age_seconds}s (exceeds ${BACKUP_MAX_AGE_SECONDS}s)"
    fi
  fi

  if ssh_run "[ -f '${metrics_file}' ]"; then
    pass "Backup metrics file exists (${metrics_file})"
    local backup_success
    backup_success="$(ssh_run "awk '/^pulse_cloud_backup_success / {print \$2}' '${metrics_file}' | tail -n1")"
    if [[ "${backup_success}" == "1" ]]; then
      pass "Backup metrics report success=1"
    else
      fail "Backup metrics success value is '${backup_success}' (expected 1)"
    fi
  else
    fail "Backup metrics file missing (${metrics_file})"
  fi
}

main() {
  require_cmd curl
  require_cmd ssh

  check_local_http
  check_trial_signup_routes
  check_public_signup_routes
  check_stripe_checkout_probe
  check_status_endpoint
  check_ssh_connectivity

  if (( FAILURES == 0 )); then
    check_remote_env_and_compose
    check_remote_backup_state
  fi

  echo
  echo "Summary: failures=${FAILURES}, warnings=${WARNINGS}"
  if (( FAILURES > 0 )); then
    exit 1
  fi
}

main "$@"
