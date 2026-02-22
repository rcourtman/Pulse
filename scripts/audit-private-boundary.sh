#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-report}"

if [[ "${MODE}" != "report" && "${MODE}" != "--enforce" && "${MODE}" != "--enforce-api-imports" && "${MODE}" != "--enforce-api-root-imports" && "${MODE}" != "--enforce-nonapi-imports" ]]; then
  echo "Usage: $0 [report|--enforce|--enforce-api-imports|--enforce-api-root-imports|--enforce-nonapi-imports]" >&2
  exit 2
fi

if ! command -v rg >/dev/null 2>&1; then
  echo "ripgrep (rg) is required" >&2
  exit 2
fi

PATTERN='^(internal/license/|internal/api/(license_|entitlement_|billing_|stripe_|hosted_|rbac_|audit_|reporting_|sso_|conversion_|router_routes_org_license\.go$|router_routes_hosted\.go$))'

ALL_MATCHES="$(rg --files internal/license internal/api 2>/dev/null | rg "${PATTERN}" || true)"
SHIM_EXCLUDES='^(internal/license/features\.go|internal/license/license\.go|internal/license/pubkey\.go|internal/license/persistence\.go|internal/license/subscription/state\.go|internal/license/subscription/transitions\.go|internal/license/conversion/upgrade_reasons\.go|internal/license/conversion/events\.go|internal/license/conversion/config\.go|internal/license/conversion/quality\.go|internal/license/conversion/metrics\.go|internal/license/conversion/store\.go|internal/license/conversion/recorder\.go|internal/license/metering/event\.go|internal/license/metering/aggregator\.go|internal/license/revocation/safety\.go|internal/license/revocation/crl\.go|internal/license/entitlements/types\.go|internal/license/entitlements/aliases\.go|internal/license/entitlements/source\.go|internal/license/entitlements/evaluator\.go|internal/license/entitlements/token_source\.go|internal/license/entitlements/database_source\.go|internal/license/entitlements/billing_store\.go)$'
ALL_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg -v "${SHIM_EXCLUDES}" || true)"
PROD_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg -v '_test\.go$' || true)"
TEST_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg '_test\.go$' || true)"

API_INTERNAL_LICENSE_IMPORTS="$(rg -n 'internal/license/' internal/api/*.go 2>/dev/null | rg -v '_test\.go' || true)"
API_INTERNAL_LICENSE_ROOT_IMPORTS="$(rg -n '"github.com/rcourtman/pulse-go-rewrite/internal/license"' internal/api/*.go 2>/dev/null | rg -v '_test\.go' || true)"
API_ROOT_IMPORT_ALLOWLIST='^$'
API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST="$(printf "%s\n" "${API_INTERNAL_LICENSE_ROOT_IMPORTS}" | rg -v "${API_ROOT_IMPORT_ALLOWLIST}" || true)"

NON_API_INTERNAL_LICENSE_ROOT_IMPORTS="$({
  rg -n '"github.com/rcourtman/pulse-go-rewrite/internal/license"' \
    --glob '*.go' \
    --glob '!**/*_test.go' \
    --glob '!internal/api/**' \
    --glob '!internal/license/**' \
    2>/dev/null || true
})"
NON_API_IMPORT_ALLOWLIST='^$'
NON_API_INTERNAL_LICENSE_IMPORTS="$(printf "%s\n" "${NON_API_INTERNAL_LICENSE_ROOT_IMPORTS}" | rg -v "${NON_API_IMPORT_ALLOWLIST}" || true)"

prod_count=0
test_count=0
if [[ -n "${PROD_MATCHES}" ]]; then
  prod_count="$(printf "%s\n" "${PROD_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${TEST_MATCHES}" ]]; then
  test_count="$(printf "%s\n" "${TEST_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

api_import_count=0
if [[ -n "${API_INTERNAL_LICENSE_IMPORTS}" ]]; then
  api_import_count="$(printf "%s\n" "${API_INTERNAL_LICENSE_IMPORTS}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

api_root_import_count=0
if [[ -n "${API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST}" ]]; then
  api_root_import_count="$(printf "%s\n" "${API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

non_api_import_count=0
if [[ -n "${NON_API_INTERNAL_LICENSE_IMPORTS}" ]]; then
  non_api_import_count="$(printf "%s\n" "${NON_API_INTERNAL_LICENSE_IMPORTS}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

echo "Private boundary audit"
echo "  Production files in paid domains: ${prod_count}"
echo "  Test files in paid domains: ${test_count}"
echo "  API files importing internal/license: ${api_import_count}"
echo "  API root imports of internal/license: ${api_root_import_count}"
echo "  Non-API runtime imports of internal/license: ${non_api_import_count}"

if [[ "${prod_count}" -gt 0 ]]; then
  echo
  echo "Production files currently in paid domains:"
  printf "%s\n" "${PROD_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${test_count}" -gt 0 ]]; then
  echo
  echo "Test files currently in paid domains:"
  printf "%s\n" "${TEST_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${api_import_count}" -gt 0 ]]; then
  echo
  echo "Non-test internal API imports of internal/license remain:"
  printf "%s\n" "${API_INTERNAL_LICENSE_IMPORTS}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${api_root_import_count}" -gt 0 ]]; then
  echo
  echo "API root imports of internal/license remain:"
  printf "%s\n" "${API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${non_api_import_count}" -gt 0 ]]; then
  echo
  echo "Non-API runtime imports of internal/license remain:"
  printf "%s\n" "${NON_API_INTERNAL_LICENSE_IMPORTS}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${MODE}" == "--enforce" && "${prod_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: production paid-domain files still live in public repo."
  exit 1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-imports") && "${api_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: non-test internal API files still import internal/license."
  exit 1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-root-imports") && "${api_root_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: API root imports of internal/license remain."
  exit 1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-nonapi-imports") && "${non_api_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: non-API runtime files import internal/license."
  exit 1
fi

exit 0
