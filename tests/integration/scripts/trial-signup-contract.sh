#!/usr/bin/env bash
set -euo pipefail

# Live trial-signup contract probe for self-hosted Pulse + control-plane.
# Intended to run inside the target environment (for example inside an LXC).

PULSE_BASE_URL="${PULSE_BASE_URL:-http://127.0.0.1:7655}"
PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}"
PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}"
TRIAL_ORG_ID="${TRIAL_ORG_ID:-default}"
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
entitlements_body="${tmp_dir}/entitlements.json"
start_body="${tmp_dir}/trial_start.json"

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

echo "[3/6] Check pre-trial entitlements"
entitlements_code="$(
  curl -sS -o "${entitlements_body}" -w '%{http_code}' \
    -b "${cookies_file}" \
    "${PULSE_BASE_URL}/api/license/entitlements"
)"
assert_code "200" "${entitlements_code}" "GET /api/license/entitlements"
trial_eligible="$(jq -r '.trial_eligible // empty' "${entitlements_body}")"
if [ "${trial_eligible}" != "true" ]; then
  echo "ERROR: expected trial_eligible=true before trial start"
  exit 1
fi

echo "[4/6] Verify local trial start succeeds (no credit card required)"
start_code="$(
  curl -sS -o "${start_body}" -w '%{http_code}' \
    -X POST \
    -b "${cookies_file}" \
    -H "X-CSRF-Token: ${csrf_token}" \
    "${PULSE_BASE_URL}/api/license/trial/start"
)"
assert_code "200" "${start_code}" "POST /api/license/trial/start"

start_sub_state="$(jq -r '.subscription_state // empty' "${start_body}")"
if [ "${start_sub_state}" != "trial" ]; then
  echo "ERROR: expected subscription_state=trial, got ${start_sub_state:-<empty>}"
  exit 1
fi

echo "[5/6] Verify post-trial entitlements reflect active trial"
post_entitlements_code="$(
  curl -sS -o "${entitlements_body}" -w '%{http_code}' \
    -b "${cookies_file}" \
    "${PULSE_BASE_URL}/api/license/entitlements"
)"
assert_code "200" "${post_entitlements_code}" "GET /api/license/entitlements (post-trial)"
post_trial_eligible="$(jq -r '.trial_eligible // empty' "${entitlements_body}")"
if [ "${post_trial_eligible}" != "false" ]; then
  echo "ERROR: expected trial_eligible=false after trial start"
  exit 1
fi

echo "[6/6] Verify second trial start is rejected (already used)"
second_start_code="$(
  curl -sS -o "${start_body}" -w '%{http_code}' \
    -X POST \
    -b "${cookies_file}" \
    -H "X-CSRF-Token: ${csrf_token}" \
    "${PULSE_BASE_URL}/api/license/trial/start"
)"
assert_code "409" "${second_start_code}" "POST /api/license/trial/start (second attempt)"

echo "PASS: trial signup contract validated"
echo "  login_code=${login_code}"
echo "  entitlements_before_code=${entitlements_code}"
echo "  trial_start_code=${start_code} (state=${start_sub_state})"
echo "  post_trial_entitlements_code=${post_entitlements_code}"
echo "  second_trial_start_code=${second_start_code}"
