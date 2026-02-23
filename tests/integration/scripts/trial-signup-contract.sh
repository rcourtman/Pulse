#!/usr/bin/env bash
set -euo pipefail

# Live trial-signup contract probe for self-hosted Pulse + control-plane.
# Intended to run inside the target environment (for example inside an LXC).

PULSE_BASE_URL="${PULSE_BASE_URL:-http://127.0.0.1:7655}"
PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}"
PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}"
TRIAL_SIGNUP_MARKER="${TRIAL_SIGNUP_MARKER:-Start your 14-day Pulse Pro trial}"
TRIAL_ORG_ID="${TRIAL_ORG_ID:-default}"
TRIAL_EMAIL="${TRIAL_EMAIL:-trial-e2e@example.com}"
TRIAL_NAME="${TRIAL_NAME:-Trial E2E}"
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
hosted_page="${tmp_dir}/hosted_signup.html"
checkout_headers="${tmp_dir}/checkout.headers"
checkout_body="${tmp_dir}/checkout.body"

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

echo "[4/6] Verify local trial start is blocked without hosted signup"
start_code="$(
  curl -sS -o "${start_body}" -w '%{http_code}' \
    -X POST \
    -b "${cookies_file}" \
    -H "X-CSRF-Token: ${csrf_token}" \
    "${PULSE_BASE_URL}/api/license/trial/start"
)"
assert_code "409" "${start_code}" "POST /api/license/trial/start"

start_error_code="$(jq -r '.code // empty' "${start_body}")"
if [ "${start_error_code}" != "trial_signup_required" ]; then
  echo "ERROR: expected code=trial_signup_required, got ${start_error_code:-<empty>}"
  exit 1
fi

action_url="$(jq -r '.details.action_url // empty' "${start_body}")"
if [ -z "${action_url}" ]; then
  echo "ERROR: trial start response missing details.action_url"
  exit 1
fi

cp_origin="$(printf '%s' "${action_url}" | sed -E 's#(https?://[^/]+).*#\1#')"
checkout_url="${cp_origin}/api/trial-signup/checkout"

echo "[5/6] Verify hosted signup page renders"
hosted_code="$(curl -sS -o "${hosted_page}" -w '%{http_code}' "${action_url}")"
assert_code "200" "${hosted_code}" "GET hosted signup page"
if ! grep -qi "${TRIAL_SIGNUP_MARKER}" "${hosted_page}"; then
  echo "ERROR: hosted signup page missing expected marker text"
  exit 1
fi

echo "[6/6] Verify checkout endpoint redirects to Stripe hosted checkout"
checkout_code="$(
  curl -sS -o "${checkout_body}" -D "${checkout_headers}" -w '%{http_code}' \
    -X POST \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "org_id=${TRIAL_ORG_ID}" \
    --data-urlencode "return_url=${PULSE_BASE_URL}/auth/trial-activate" \
    --data-urlencode "email=${TRIAL_EMAIL}" \
    --data-urlencode "name=${TRIAL_NAME}" \
    "${checkout_url}"
)"
assert_code "303" "${checkout_code}" "POST /api/trial-signup/checkout"

location_header="$(awk 'BEGIN { IGNORECASE=1 } /^Location:/ { sub("\r$", "", $2); print $2 }' "${checkout_headers}" | tail -n1)"
if [ -z "${location_header}" ]; then
  echo "ERROR: checkout response missing Location header"
  exit 1
fi
if [[ "${location_header}" != *"checkout.stripe.com"* ]]; then
  echo "ERROR: expected Stripe hosted checkout redirect, got: ${location_header}"
  exit 1
fi

echo "PASS: trial signup contract validated"
echo "  login_code=${login_code}"
echo "  entitlements_before_code=${entitlements_code}"
echo "  trial_start_code=${start_code} (code=${start_error_code})"
echo "  hosted_signup_page_code=${hosted_code}"
echo "  checkout_post_code=${checkout_code}"
echo "  checkout_redirect_location=${location_header}"
