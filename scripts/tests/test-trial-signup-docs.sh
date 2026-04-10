#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNBOOK="${ROOT_DIR}/docs/operations/TRIAL_E2E_LXC_SNAPSHOT_RUNBOOK.md"

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

main() {
  local output
  output="$(
    awk '
      /^## Contract Probe Script$/ { capture=1 }
      capture { print }
      /^## Full Sandbox E2E \(Playwright\)$/ { exit }
    ' "${RUNBOOK}"
  )"

  assert_contains "runbook references hosted trial probe script" "${output}" "tests/integration/scripts/trial-signup-contract.sh"
  assert_contains "runbook documents initial hosted-signup redirect" "${output}" "returns \`409\` with \`trial_signup_required\`"
  assert_contains "runbook documents hosted-signup retry burst" "${output}" "hosted-signup retry burst"
  assert_contains "runbook documents retry-after backoff metadata" "${output}" "\`Retry-After\` backoff metadata"
  assert_contains "runbook documents limiter transition output" "${output}" "retry_limiter_attempt=..."
  assert_contains "runbook documents final trial-rate-limited output" "${output}" "final_trial_start_code=429"
  assert_not_contains "runbook no longer hardcodes second-attempt rejection" "${output}" "Second immediate trial start is rejected with \`429\`"

  if (( failures > 0 )); then
    echo "trial-signup docs smoke tests failed: ${failures}" >&2
    exit 1
  fi

  echo "trial-signup docs smoke tests passed."
}

main "$@"
