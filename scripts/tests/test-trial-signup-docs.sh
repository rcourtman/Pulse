#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNBOOK="${ROOT_DIR}/docs/operations/TRIAL_E2E_LXC_SNAPSHOT_RUNBOOK.md"
PRICING_DOC="${ROOT_DIR}/docs/architecture/v6-pricing-and-tiering.md"
UPGRADE_DOC="${ROOT_DIR}/docs/UPGRADE_v6.md"
INTEGRATION_README="${ROOT_DIR}/tests/integration/README.md"
EVAL_TASK_DOC="${ROOT_DIR}/tests/integration/evals/tasks/trial-signup.md"
EVAL_SCENARIOS_DOC="${ROOT_DIR}/tests/integration/evals/scenarios.json"
SOURCE_OF_TRUTH_DOC="${ROOT_DIR}/docs/release-control/v6/internal/SOURCE_OF_TRUTH.md"
MIGRATION_AUDIT_DOC="${ROOT_DIR}/docs/release-control/v6/internal/V5_TO_V6_COMMERCIAL_MIGRATION_AUDIT_2026-03-07.md"

TRIAL_SIGNUP_REFERENCE_PATTERN='(/api/license/trial/start|trial_signup_required|trial_rate_limited)'

OPERATOR_DOCS=(
  "${RUNBOOK}"
  "${PRICING_DOC}"
  "${UPGRADE_DOC}"
  "${INTEGRATION_README}"
  "${EVAL_TASK_DOC}"
  "${EVAL_SCENARIOS_DOC}"
)

TRACKED_REFERENCE_DOCS=()

FORBIDDEN_PATTERNS=(
  "1 trial initiation attempt per org per 24 hours"
  "1 initiation attempt per org per 24h"
  "A second immediate initiation attempt is rate limited."
  "Second immediate trial start is rejected with \`429\`"
)

failures=0

record_failure() {
  echo "[FAIL] $1" >&2
  ((failures += 1))
}

record_pass() {
  echo "[PASS] $1"
}

discover_tracked_reference_docs() {
  local discovered_doc

  TRACKED_REFERENCE_DOCS=()
  while IFS= read -r discovered_doc; do
    TRACKED_REFERENCE_DOCS+=("${ROOT_DIR}/${discovered_doc}")
  done < <(
    git -C "${ROOT_DIR}" grep -lE "${TRIAL_SIGNUP_REFERENCE_PATTERN}" -- docs tests/integration
  )

  if (( ${#TRACKED_REFERENCE_DOCS[@]} == 0 )); then
    record_failure "discovered tracked trial-start reference files"
    echo "Expected tracked trial-start reference docs/tests from git grep discovery." >&2
  else
    record_pass "discovered tracked trial-start reference files (${#TRACKED_REFERENCE_DOCS[@]})"
  fi
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

assert_doc_discovered() {
  local label="$1"
  local doc="$2"
  local candidate
  for candidate in "${TRACKED_REFERENCE_DOCS[@]}"; do
    if [[ "${candidate}" == "${doc}" ]]; then
      record_pass "${label}"
      return
    fi
  done
  record_failure "${label}"
  echo "Expected discovered reference set to include: ${doc#${ROOT_DIR}/}" >&2
}

assert_forbidden_patterns_absent() {
  local doc file_content needle label

  for doc in "${TRACKED_REFERENCE_DOCS[@]}"; do
    file_content="$(cat "${doc}")"
    for needle in "${FORBIDDEN_PATTERNS[@]}"; do
      label="${doc#${ROOT_DIR}/} excludes stale pattern: ${needle}"
      assert_not_contains "${label}" "${file_content}" "${needle}"
    done
  done
}

main() {
  local runbook_output pricing_output upgrade_output integration_output eval_task_output
  local eval_scenarios_output
  local source_of_truth_output migration_audit_output
  local operator_doc

  discover_tracked_reference_docs

  assert_doc_discovered "source of truth is in tracked reference discovery" "${SOURCE_OF_TRUTH_DOC}"
  assert_doc_discovered "migration audit is in tracked reference discovery" "${MIGRATION_AUDIT_DOC}"
  for operator_doc in "${OPERATOR_DOCS[@]}"; do
    assert_doc_discovered "${operator_doc#${ROOT_DIR}/} is in tracked reference discovery" "${operator_doc}"
  done

  runbook_output="$(
    awk '
      /^## Contract Probe Script$/ { capture=1 }
      capture { print }
      /^## Full Sandbox E2E \(Playwright\)$/ { exit }
    ' "${RUNBOOK}"
  )"
  pricing_output="$(
    sed -n '368,386p' "${PRICING_DOC}"
  )"
  upgrade_output="$(
    sed -n '112,128p' "${UPGRADE_DOC}"
  )"
  integration_output="$(
    sed -n '28,40p' "${INTEGRATION_README}"
  )"
  eval_task_output="$(
    sed -n '1,24p' "${EVAL_TASK_DOC}"
  )"
  eval_scenarios_output="$(
    sed -n '24,34p' "${EVAL_SCENARIOS_DOC}"
  )"
  source_of_truth_output="$(
    sed -n '412,420p' "${SOURCE_OF_TRUTH_DOC}"
  )"
  migration_audit_output="$(
    sed -n '10,22p' "${MIGRATION_AUDIT_DOC}"
  )"

  assert_contains "runbook references hosted trial probe script" "${runbook_output}" "tests/integration/scripts/trial-signup-contract.sh"
  assert_contains "runbook documents initial hosted-signup redirect" "${runbook_output}" "returns \`409\` with \`trial_signup_required\`"
  assert_contains "runbook documents hosted-signup retry burst" "${runbook_output}" "hosted-signup retry burst"
  assert_contains "runbook documents retry-after backoff metadata" "${runbook_output}" "\`Retry-After\` backoff metadata"
  assert_contains "runbook documents limiter transition output" "${runbook_output}" "retry_limiter_attempt=..."
  assert_contains "runbook documents final trial-rate-limited output" "${runbook_output}" "final_trial_start_code=429"
  assert_not_contains "runbook no longer hardcodes second-attempt rejection" "${runbook_output}" "Second immediate trial start is rejected with \`429\`"

  assert_contains "pricing doc documents retry burst" "${pricing_output}" "retry burst"
  assert_contains "pricing doc documents trial-rate-limited response" "${pricing_output}" "\`429 trial_rate_limited\`"
  assert_contains "pricing doc documents retry-after metadata" "${pricing_output}" "\`Retry-After\`"
  assert_not_contains "pricing doc no longer claims 24 hour limiter" "${pricing_output}" "24 hours"

  assert_contains "upgrade guide keeps hosted-signup-only wording" "${upgrade_output}" "initiates hosted signup rather than minting a local trial directly"
  assert_contains "upgrade guide documents retry burst" "${upgrade_output}" "retry burst"
  assert_contains "upgrade guide documents trial-rate-limited response" "${upgrade_output}" "\`429 trial_rate_limited\`"
  assert_contains "upgrade guide documents retry-after metadata" "${upgrade_output}" "\`Retry-After\`"

  assert_contains "integration readme documents trial-start route" "${integration_output}" "POST /api/license/trial/start"
  assert_contains "integration readme documents hosted-signup redirect code" "${integration_output}" "\`409 trial_signup_required\`"
  assert_contains "integration readme documents canonical trial-rate-limited response" "${integration_output}" "\`429 trial_rate_limited\`"
  assert_contains "integration readme documents reused-instance retry-after branch" "${integration_output}" "Retry-After"
  assert_contains "integration readme documents hosted-signup retry burst" "${integration_output}" "hosted-signup retry-burst contract"

  assert_contains "eval task documents retry-burst contract" "${eval_task_output}" "retry-burst contract"
  assert_contains "eval task documents canonical trial-rate-limited response" "${eval_task_output}" "\`429 trial_rate_limited\`"
  assert_contains "eval task documents retry-after metadata" "${eval_task_output}" "\`Retry-After\`"

  assert_contains "eval scenario documents trial-start route" "${eval_scenarios_output}" "POST /api/license/trial/start"
  assert_contains "eval scenario documents hosted-signup redirect code" "${eval_scenarios_output}" "409 trial_signup_required"
  assert_contains "eval scenario documents canonical trial-rate-limited response" "${eval_scenarios_output}" "429 trial_rate_limited"
  assert_contains "eval scenario documents retry-after metadata" "${eval_scenarios_output}" "Retry-After"

  assert_contains "source of truth keeps hosted-signup-only contract" "${source_of_truth_output}" "must initiate hosted signup only"
  assert_contains "source of truth forbids local trial minting" "${source_of_truth_output}" "must not mint local trial"

  assert_contains "migration audit keeps hosted-signup-only contract" "${migration_audit_output}" "must initiate hosted signup only"
  assert_contains "migration audit forbids local trial minting" "${migration_audit_output}" "may only redeem signed hosted trial activation tokens"

  assert_forbidden_patterns_absent

  if (( failures > 0 )); then
    echo "trial-signup docs smoke tests failed: ${failures}" >&2
    exit 1
  fi

  echo "trial-signup docs smoke tests passed."
}

main "$@"
