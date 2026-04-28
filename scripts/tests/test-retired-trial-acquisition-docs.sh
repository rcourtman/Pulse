#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNBOOK="${ROOT_DIR}/docs/operations/RETIRED_TRIAL_ACQUISITION_LXC_SNAPSHOT_RUNBOOK.md"
UPGRADE_DOC="${ROOT_DIR}/docs/UPGRADE_v6.md"
INTEGRATION_README="${ROOT_DIR}/tests/integration/README.md"
EVAL_TASK_DOC="${ROOT_DIR}/tests/integration/evals/tasks/retired-trial-acquisition.md"
EVAL_SCENARIOS_DOC="${ROOT_DIR}/tests/integration/evals/scenarios.json"
SOURCE_OF_TRUTH_DOC="${ROOT_DIR}/docs/release-control/v6/internal/SOURCE_OF_TRUTH.md"
HIGH_RISK_MATRIX_DOC="${ROOT_DIR}/docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md"
API_CONTRACTS_DOC="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/api-contracts.md"
CLOUD_PAID_DOC="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/cloud-paid.md"
AGENT_LIFECYCLE_DOC="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/agent-lifecycle.md"
STORAGE_RECOVERY_DOC="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/storage-recovery.md"

FORBIDDEN_ACTIVE_PATTERNS=(
  "trial_signup_required"
  "trial_rate_limited"
  "hosted-signup retry burst"
  "hosted-signup retry-burst"
  "Start hosted Pro trial initiation"
)

RETIRED_CONTRACT_DOCS=(
  "${RUNBOOK}"
  "${INTEGRATION_README}"
  "${EVAL_TASK_DOC}"
  "${EVAL_SCENARIOS_DOC}"
  "${SOURCE_OF_TRUTH_DOC}"
  "${HIGH_RISK_MATRIX_DOC}"
  "${API_CONTRACTS_DOC}"
  "${CLOUD_PAID_DOC}"
  "${AGENT_LIFECYCLE_DOC}"
  "${STORAGE_RECOVERY_DOC}"
)

failures=0

record_failure() {
  echo "[FAIL] $1" >&2
  ((failures += 1))
}

record_pass() {
  echo "[PASS] $1"
}

assert_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    record_pass "${label}"
  else
    record_failure "${label}"
    echo "Expected to find: ${needle}" >&2
  fi
}

assert_not_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    record_failure "${label}"
    echo "Did not expect to find: ${needle}" >&2
  else
    record_pass "${label}"
  fi
}

assert_file_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"
  assert_contains "${label}" "$(cat "${file}")" "${needle}"
}

assert_file_not_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"
  assert_not_contains "${label}" "$(cat "${file}")" "${needle}"
}

main() {
  local doc pattern

  for doc in "${RETIRED_CONTRACT_DOCS[@]}"; do
    assert_file_contains "${doc#${ROOT_DIR}/} documents retired trial-start route" "${doc}" "POST /api/license/trial/start"
    assert_file_contains "${doc#${ROOT_DIR}/} documents HTTP 404 retired route behavior" "${doc}" "404"
  done

  assert_file_contains "upgrade guide documents no general in-app trial" "${UPGRADE_DOC}" "does not expose a general in-app trial"
  assert_file_contains "runbook documents unchanged entitlements" "${RUNBOOK}" "Entitlements remain unchanged"
  assert_file_contains "integration README documents no trial CTAs" "${INTEGRATION_README}" "trial CTAs"
  assert_file_contains "eval task documents no legacy acquisition payloads" "${EVAL_TASK_DOC}" "legacy"

  for doc in "${RETIRED_CONTRACT_DOCS[@]}"; do
    for pattern in "${FORBIDDEN_ACTIVE_PATTERNS[@]}"; do
      assert_file_not_contains "${doc#${ROOT_DIR}/} excludes old active trial-start wording: ${pattern}" "${doc}" "${pattern}"
    done
  done

  if (( failures > 0 )); then
    echo "retired trial acquisition docs smoke tests failed: ${failures}" >&2
    exit 1
  fi

  echo "retired trial acquisition docs smoke tests passed."
}

main "$@"
