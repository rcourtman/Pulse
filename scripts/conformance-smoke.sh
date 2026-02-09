#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

declare -a PERM_NAMES=()
declare -a PERM_RESULTS=()
PASS_COUNT=0
FAIL_COUNT=0

run_permutation() {
  local name="$1"
  shift

  local idx=$(( ${#PERM_NAMES[@]} + 1 ))
  local failed=0

  echo
  echo "=== Permutation ${idx}: ${name} ==="

  local cmd
  for cmd in "$@"; do
    local exit_code=0
    echo "+ ${cmd}"
    set +e
    bash -lc "${cmd}"
    exit_code=$?
    set -e

    echo "  exit code: ${exit_code}"
    if [[ "${exit_code}" -ne 0 ]]; then
      failed=1
    fi
  done

  PERM_NAMES+=("${name}")
  if [[ "${failed}" -eq 0 ]]; then
    PERM_RESULTS+=("PASS")
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    PERM_RESULTS+=("FAIL")
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

run_permutation \
  "Default Mode" \
  "go build ./..." \
  "go test ./internal/api/... -run \"TestRouterRouteInventory|TestResourceV2|TestTenantMiddleware_Enforcement_Permanent|TestWebSocketIsolation_Permanent|TestStateIsolation_Permanent|TestResourceIsolation_Permanent\" -count=1"

run_permutation \
  "Monitoring Subsystem" \
  "go test ./internal/monitoring/... -run \"TrueNAS|Tenant|Alert|Isolation\" -count=1"

run_permutation \
  "Multi-tenant + Isolation Aggregate" \
  "go test ./internal/api/... -run \"Tenant|Isolation|RBAC|OrgHandlers|SuspendGate\" -count=1" \
  "go test ./internal/websocket/... -run \"TenantIsolation|AlertBroadcast\" -count=1"

run_permutation \
  "License + Hosted Mode Gates" \
  "go test ./internal/api/... -run \"TestRequireLicenseFeature|TestLicenseGatedEmptyResponse|TestNoInline402Responses|TestHostedSignup|TestOrgLifecycle\" -count=1" \
  "go test ./internal/license/... -run \"ContractParity\" -count=1"

run_permutation \
  "Frontend Baseline" \
  "frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json" \
  "cd frontend-modern && npx vitest run"

run_permutation \
  "Unified Resources + TrueNAS Contract" \
  "go test ./internal/unifiedresources/... -count=1" \
  "go test ./internal/truenas/... -count=1"

echo
echo "=== Conformance Smoke Summary ==="
printf "%-3s | %-42s | %s\n" "ID" "Permutation" "Result"
printf -- "----+--------------------------------------------+--------\n"

for i in "${!PERM_NAMES[@]}"; do
  id=$((i + 1))
  printf "%-3s | %-42s | %s\n" "${id}" "${PERM_NAMES[$i]}" "${PERM_RESULTS[$i]}"
done

echo
echo "Total Pass: ${PASS_COUNT}"
echo "Total Fail: ${FAIL_COUNT}"

if [[ "${FAIL_COUNT}" -gt 0 ]]; then
  exit 1
fi

exit 0
