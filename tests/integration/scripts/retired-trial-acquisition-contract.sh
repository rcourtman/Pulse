#!/usr/bin/env bash
set -euo pipefail

# Retired trial-acquisition contract probe for self-hosted Pulse.
# This verifies ordinary self-hosted v6 runtimes do not expose in-app trial
# acquisition or the retired trial-return callback.

PULSE_BASE_URL="${PULSE_BASE_URL:-http://127.0.0.1:7655}"
PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}"
PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-adminadminadmin}"
WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-60}"

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required"
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required"
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

cookies_file="${tmp_dir}/cookies.txt"
login_body="${tmp_dir}/login.json"
entitlements_before_body="${tmp_dir}/entitlements_before.json"
entitlements_after_body="${tmp_dir}/entitlements_after.json"
trial_start_body="${tmp_dir}/trial_start.json"
trial_activate_body="${tmp_dir}/trial_activate.txt"

wait_for_http() {
  local url="$1"
  local elapsed=0
  while [ "${elapsed}" -lt "${WAIT_TIMEOUT_SECONDS}" ]; do
    code="$(curl -sS -o /dev/null -w '%{http_code}' "${url}" || true)"
    if [ "${code}" != "000" ]; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  echo "ERROR: timed out waiting for ${url}"
  return 1
}

assert_code() {
  local expected="$1"
  local actual="$2"
  local context="$3"
  if [ "${actual}" != "${expected}" ]; then
    echo "ERROR: ${context} expected HTTP ${expected}, got ${actual}"
    exit 1
  fi
}

echo "[1/6] Waiting for Pulse API readiness at ${PULSE_BASE_URL}"
wait_for_http "${PULSE_BASE_URL}/api/login"

echo "[2/6] Login with configured test credentials"
login_code="$(
  curl -sS -o "${login_body}" -w '%{http_code}' \
    -c "${cookies_file}" \
    -H 'Content-Type: application/json' \
    --data "{\"username\":\"${PULSE_E2E_USERNAME}\",\"password\":\"${PULSE_E2E_PASSWORD}\"}" \
    "${PULSE_BASE_URL}/api/login"
)"
assert_code "200" "${login_code}" "POST /api/login"

csrf_token="$(awk '$6=="pulse_csrf" {print $7}' "${cookies_file}" | tail -n1)"
if [ -z "${csrf_token}" ]; then
  echo "ERROR: login did not return pulse_csrf cookie"
  exit 1
fi

echo "[3/6] Capture entitlements before retired route probe"
entitlements_before_code="$(
  curl -sS -o "${entitlements_before_body}" -w '%{http_code}' \
    -b "${cookies_file}" \
    "${PULSE_BASE_URL}/api/license/entitlements"
)"
assert_code "200" "${entitlements_before_code}" "GET /api/license/entitlements"

echo "[4/6] Verify ordinary self-hosted trial start route is not exposed"
trial_start_code="$(
  curl -sS -o "${trial_start_body}" -w '%{http_code}' \
    -X POST \
    -b "${cookies_file}" \
    -H "X-CSRF-Token: ${csrf_token}" \
    "${PULSE_BASE_URL}/api/license/trial/start"
)"
assert_code "404" "${trial_start_code}" "POST /api/license/trial/start"

if jq -e '.code == "trial_signup_required" or .code == "trial_rate_limited"' "${trial_start_body}" >/dev/null 2>&1; then
  echo "ERROR: retired route returned legacy trial acquisition payload"
  exit 1
fi

echo "[5/6] Verify retired self-hosted trial activation callback is not exposed"
trial_activate_code="$(
  curl -sS -o "${trial_activate_body}" -w '%{http_code}' \
    -b "${cookies_file}" \
    "${PULSE_BASE_URL}/auth/trial-activate?token=retired_probe"
)"
assert_code "404" "${trial_activate_code}" "GET /auth/trial-activate"

echo "[6/6] Verify entitlements remain unchanged"
entitlements_after_code="$(
  curl -sS -o "${entitlements_after_body}" -w '%{http_code}' \
    -b "${cookies_file}" \
    "${PULSE_BASE_URL}/api/license/entitlements"
)"
assert_code "200" "${entitlements_after_code}" "GET /api/license/entitlements (after retired route probe)"

before_summary="$(jq -c '{subscription_state, tier, trial_eligible, trial_days_remaining, valid, is_lifetime}' "${entitlements_before_body}")"
after_summary="$(jq -c '{subscription_state, tier, trial_eligible, trial_days_remaining, valid, is_lifetime}' "${entitlements_after_body}")"
if [ "${before_summary}" != "${after_summary}" ]; then
  echo "ERROR: retired route probe changed entitlement state"
  echo "before=${before_summary}"
  echo "after=${after_summary}"
  exit 1
fi

echo "PASS: self-hosted trial acquisition routes are retired"
echo "  login_code=${login_code}"
echo "  entitlements_before_code=${entitlements_before_code}"
echo "  trial_start_code=${trial_start_code}"
echo "  trial_activate_code=${trial_activate_code}"
echo "  entitlements_after_code=${entitlements_after_code}"
