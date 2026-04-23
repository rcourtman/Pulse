#!/usr/bin/env bash
set -euo pipefail

DOMAIN="${DOMAIN:-cloud.pulserelay.pro}"
PULSE_CLOUD_BASE_URL="${PULSE_CLOUD_BASE_URL:-https://${DOMAIN}}"
EXPECT_PUBLIC_SIGNUP_ENABLED="${EXPECT_PUBLIC_SIGNUP_ENABLED:-true}"
PUBLIC_SIGNUP_CLOSED_REDIRECT_URL="${PUBLIC_SIGNUP_CLOSED_REDIRECT_URL:-https://pulserelay.pro/}"
CHECK_MAGIC_LINK_VALID_PROBE="${CHECK_MAGIC_LINK_VALID_PROBE:-false}"
CURL_TIMEOUT="${CURL_TIMEOUT:-15}"

BASE_URL="${PULSE_CLOUD_BASE_URL%/}"
FAILURES=0

pass() {
  echo "[PASS] $*"
}

fail() {
  echo "[FAIL] $*"
  FAILURES=$((FAILURES + 1))
}

info() {
  echo "[INFO] $*"
}

http_code() {
  local method="$1"
  local path="$2"
  shift 2
  curl -sS --max-time "${CURL_TIMEOUT}" -o /dev/null -w "%{http_code}" -X "${method}" "$@" "${BASE_URL}${path}"
}

http_status_and_redirect() {
  local method="$1"
  local path="$2"
  shift 2
  curl -sS --max-time "${CURL_TIMEOUT}" -o /dev/null -w "%{http_code} %{redirect_url}" -X "${method}" "$@" "${BASE_URL}${path}"
}

expect_code() {
  local method="$1"
  local path="$2"
  local expected="$3"
  shift 3
  local code
  code="$(http_code "${method}" "${path}" "$@")"
  if [[ "${code}" == "${expected}" ]]; then
    pass "${method} ${path} returns ${expected}"
  else
    fail "${method} ${path} expected ${expected}, got ${code}"
  fi
}

expect_redirect() {
  local method="$1"
  local path="$2"
  local expected_code="$3"
  shift 3
  local code redirect_url
  read -r code redirect_url <<<"$(http_status_and_redirect "${method}" "${path}" "$@")"
  if [[ "${code}" == "${expected_code}" && "${redirect_url}" == "${PUBLIC_SIGNUP_CLOSED_REDIRECT_URL}" ]]; then
    pass "${method} ${path} returns ${expected_code} to ${PUBLIC_SIGNUP_CLOSED_REDIRECT_URL}"
  else
    fail "${method} ${path} expected ${expected_code} to ${PUBLIC_SIGNUP_CLOSED_REDIRECT_URL}, got ${code} to ${redirect_url:-<none>}"
  fi
}

check_shared_public_floor() {
  info "Checking shared public control-plane floor at ${BASE_URL}"
  expect_code GET /healthz 200
  expect_code GET /readyz 200
  expect_code POST /api/stripe/webhook 400
}

check_closed_signup_contract() {
  info "Checking intentionally closed public signup contract"
  expect_redirect GET /signup 302
  expect_redirect GET /cloud/signup 302
  expect_redirect GET /signup/complete 302
  expect_redirect GET /cloud/signup/complete 302
  expect_redirect GET /api/public/signup 302
  expect_redirect POST /api/public/signup 307 -H "Content-Type: application/json" --data '{"email":"not-an-email","org_name":"Acme"}'
  expect_code POST /api/public/magic-link/request 400 -H "Content-Type: application/json" --data '{"email":"not-an-email"}'
}

check_open_signup_contract() {
  info "Checking live public signup contract"
  expect_code GET /signup 200
  expect_code GET /cloud/signup 200
  expect_code GET /signup/complete 200
  expect_code GET /cloud/signup/complete 200
  expect_code GET /api/public/signup 405
  expect_code POST /api/public/signup 400 -H "Content-Type: application/json" --data '{"email":"not-an-email","org_name":"Acme"}'
  expect_code POST /api/public/magic-link/request 400 -H "Content-Type: application/json" --data '{"email":"not-an-email"}'

  if [[ "${CHECK_MAGIC_LINK_VALID_PROBE}" == "true" ]]; then
    expect_code POST /api/public/magic-link/request 200 -H "Content-Type: application/json" --data '{"email":"owner@example.com"}'
  else
    info "Skipping valid magic-link probe (CHECK_MAGIC_LINK_VALID_PROBE=${CHECK_MAGIC_LINK_VALID_PROBE})"
  fi
}

main() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required for Pulse Cloud public signup smoke" >&2
    exit 1
  fi

  local expected_state
  expected_state="$(echo "${EXPECT_PUBLIC_SIGNUP_ENABLED}" | tr '[:upper:]' '[:lower:]')"
  check_shared_public_floor

  case "${expected_state}" in
    true)
      check_open_signup_contract
      ;;
    false)
      check_closed_signup_contract
      ;;
    *)
      fail "EXPECT_PUBLIC_SIGNUP_ENABLED must be true or false (got '${EXPECT_PUBLIC_SIGNUP_ENABLED}')"
      ;;
  esac

  echo
  echo "Summary: failures=${FAILURES}"
  if (( FAILURES > 0 )); then
    exit 1
  fi
}

main "$@"
