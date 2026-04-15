#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INTEGRATION_ROOT="${REPO_ROOT}/tests/integration"

HOSTED_SCENARIOS="${PULSE_E2E_CLOUD_SMOKE_SCENARIOS:-cloud-hosting,cloud-billing-lifecycle}"
PULSE_CLOUD_BASE_URL="${PULSE_CLOUD_BASE_URL:-}"
PULSE_CLOUD_SSH_TARGET="${PULSE_CLOUD_SSH_TARGET:-}"
PULSE_CP_ADMIN_KEY="${PULSE_CP_ADMIN_KEY:-}"
PULSE_E2E_STRIPE_API_KEY="${PULSE_E2E_STRIPE_API_KEY:-}"
PULSE_E2E_STRIPE_WEBHOOK_SECRET="${PULSE_E2E_STRIPE_WEBHOOK_SECRET:-}"
PULSE_E2E_HOSTED_TENANT_ID="${PULSE_E2E_HOSTED_TENANT_ID:-}"
PULSE_E2E_HOSTED_MOBILE_EMAIL="${PULSE_E2E_HOSTED_MOBILE_EMAIL:-}"
PULSE_E2E_HOSTED_MOBILE_POLL_INTERVAL_MS="${PULSE_E2E_HOSTED_MOBILE_POLL_INTERVAL_MS:-}"
PULSE_E2E_HOSTED_MOBILE_POLL_TIMEOUT_MS="${PULSE_E2E_HOSTED_MOBILE_POLL_TIMEOUT_MS:-}"

require_command() {
  local command_name="$1"
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "${command_name} is required for hosted staging smoke" >&2
    exit 1
  fi
}

require_env() {
  local var_name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    echo "${var_name} is required for hosted staging smoke" >&2
    exit 1
  fi
}

require_command curl
require_command go
require_command node
require_command npx
require_command scp
require_command ssh

if [[ ! -x "${INTEGRATION_ROOT}/node_modules/.bin/playwright" ]]; then
  echo "tests/integration dependencies are missing; run 'cd ${INTEGRATION_ROOT} && npm ci'" >&2
  exit 1
fi

require_env "PULSE_CLOUD_BASE_URL" "${PULSE_CLOUD_BASE_URL}"
require_env "PULSE_CLOUD_SSH_TARGET" "${PULSE_CLOUD_SSH_TARGET}"
require_env "PULSE_CP_ADMIN_KEY" "${PULSE_CP_ADMIN_KEY}"
require_env "PULSE_E2E_STRIPE_API_KEY" "${PULSE_E2E_STRIPE_API_KEY}"
require_env "PULSE_E2E_STRIPE_WEBHOOK_SECRET" "${PULSE_E2E_STRIPE_WEBHOOK_SECRET}"
require_env "PULSE_E2E_HOSTED_TENANT_ID" "${PULSE_E2E_HOSTED_TENANT_ID}"

echo "[1/2] Running hosted signup and billing smoke against ${PULSE_CLOUD_BASE_URL}"
(
  cd "${INTEGRATION_ROOT}"
  PULSE_E2E_SKIP_DOCKER=1 \
  PULSE_BASE_URL="${PULSE_BASE_URL:-${PULSE_CLOUD_BASE_URL}}" \
  PULSE_CLOUD_BASE_URL="${PULSE_CLOUD_BASE_URL}" \
  PULSE_CP_ADMIN_KEY="${PULSE_CP_ADMIN_KEY}" \
  PULSE_E2E_STRIPE_API_KEY="${PULSE_E2E_STRIPE_API_KEY}" \
  PULSE_E2E_STRIPE_WEBHOOK_SECRET="${PULSE_E2E_STRIPE_WEBHOOK_SECRET}" \
  node ./scripts/run-evals.mjs --mode deterministic --scenario "${HOSTED_SCENARIOS}"
)

echo "[2/2] Running hosted mobile onboarding smoke against tenant ${PULSE_E2E_HOSTED_TENANT_ID}"
bootstrap_args=(
  "${INTEGRATION_ROOT}/scripts/bootstrap-hosted-mobile-onboarding.mjs"
  --tenant-id "${PULSE_E2E_HOSTED_TENANT_ID}"
  --cloud-host "${PULSE_CLOUD_SSH_TARGET}"
  --control-plane-url "${PULSE_CLOUD_BASE_URL}"
)

if [[ -n "${PULSE_E2E_HOSTED_MOBILE_EMAIL}" ]]; then
  bootstrap_args+=(--email "${PULSE_E2E_HOSTED_MOBILE_EMAIL}")
fi

if [[ -n "${PULSE_E2E_HOSTED_MOBILE_POLL_INTERVAL_MS}" ]]; then
  bootstrap_args+=(--poll-interval-ms "${PULSE_E2E_HOSTED_MOBILE_POLL_INTERVAL_MS}")
fi

if [[ -n "${PULSE_E2E_HOSTED_MOBILE_POLL_TIMEOUT_MS}" ]]; then
  bootstrap_args+=(--poll-timeout-ms "${PULSE_E2E_HOSTED_MOBILE_POLL_TIMEOUT_MS}")
fi

node "${bootstrap_args[@]}"
