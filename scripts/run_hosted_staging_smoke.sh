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

resolve_hosted_tenant_id() {
  if [[ -n "${PULSE_E2E_HOSTED_TENANT_ID}" ]]; then
    return 0
  fi

  local tenants_payload
  if ! tenants_payload="$(
    curl -fsS \
      -H "X-Admin-Key: ${PULSE_CP_ADMIN_KEY}" \
      -H 'Accept: application/json' \
      "${PULSE_CLOUD_BASE_URL%/}/admin/tenants?state=active"
  )"; then
    echo "Failed to list active hosted tenants from ${PULSE_CLOUD_BASE_URL}" >&2
    exit 1
  fi

  if ! PULSE_E2E_HOSTED_TENANT_ID="$(
    printf '%s' "${tenants_payload}" | node -e '
      let input = "";
      process.stdin.setEncoding("utf8");
      process.stdin.on("data", (chunk) => {
        input += chunk;
      });
      process.stdin.on("end", () => {
        const parsed = JSON.parse(input);
        const tenants = Array.isArray(parsed?.tenants) ? parsed.tenants : [];
        const selected = tenants.find((entry) => typeof entry?.id === "string" && entry.id.trim() !== "");
        if (!selected) {
          process.exit(2);
        }
        process.stdout.write(selected.id.trim());
      });
    '
  )"; then
    echo "No active hosted tenant found; set PULSE_E2E_HOSTED_TENANT_ID explicitly." >&2
    exit 1
  fi

  echo "Auto-selected active hosted tenant ${PULSE_E2E_HOSTED_TENANT_ID}"
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

resolve_hosted_tenant_id

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
