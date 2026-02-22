#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-report}"
PAID_SURFACE_ALLOWLIST_FILE="scripts/repo-boundary-paid-surface.allowlist"

if [[ "${MODE}" != "report" && "${MODE}" != "--enforce" && "${MODE}" != "--enforce-api-imports" && "${MODE}" != "--enforce-api-root-imports" && "${MODE}" != "--enforce-nonapi-imports" && "${MODE}" != "--enforce-api-pkg-licensing-imports" && "${MODE}" != "--enforce-paid-surface-allowlist" ]]; then
  echo "Usage: $0 [report|--enforce|--enforce-api-imports|--enforce-api-root-imports|--enforce-nonapi-imports|--enforce-api-pkg-licensing-imports|--enforce-paid-surface-allowlist]" >&2
  exit 2
fi

if ! command -v rg >/dev/null 2>&1; then
  echo "ripgrep (rg) is required" >&2
  exit 2
fi

NAME_PATTERN='^(internal/license/|internal/api/(license_|entitlement_|billing_|stripe_|hosted_|rbac_|audit_|reporting_|sso_|conversion_|router_routes_org_license\.go$|router_routes_hosted\.go$|router_routes_licensing\.go$|router_routes_cloud\.go$))'
SEMANTIC_API_PATTERN='/api/license|/api/conversion|/api/upgrade-metrics|/api/webhooks/stripe|/api/public/signup|/api/public/magic-link|/api/hosted/|/api/admin/orgs/\{id\}/billing-state|/api/security/sso|/api/admin/rbac|/api/audit|"github\.com/rcourtman/pulse-go-rewrite/pkg/licensing'

NAME_MATCHES="$(rg --files internal/license internal/api 2>/dev/null | rg "${NAME_PATTERN}" || true)"
SEMANTIC_API_MATCHES="$(rg -l --glob 'internal/api/*.go' --glob '!internal/api/*_test.go' "${SEMANTIC_API_PATTERN}" internal/api 2>/dev/null || true)"
ALL_MATCHES="$(printf "%s\n%s\n" "${NAME_MATCHES}" "${SEMANTIC_API_MATCHES}" | sed '/^$/d' | sort -u || true)"
SHIM_EXCLUDES='^(internal/license/features\.go|internal/license/license\.go|internal/license/pubkey\.go|internal/license/persistence\.go|internal/license/subscription/state\.go|internal/license/subscription/transitions\.go|internal/license/conversion/upgrade_reasons\.go|internal/license/conversion/events\.go|internal/license/conversion/config\.go|internal/license/conversion/quality\.go|internal/license/conversion/metrics\.go|internal/license/conversion/store\.go|internal/license/conversion/recorder\.go|internal/license/metering/event\.go|internal/license/metering/aggregator\.go|internal/license/revocation/safety\.go|internal/license/revocation/crl\.go|internal/license/entitlements/types\.go|internal/license/entitlements/aliases\.go|internal/license/entitlements/source\.go|internal/license/entitlements/evaluator\.go|internal/license/entitlements/token_source\.go|internal/license/entitlements/database_source\.go|internal/license/entitlements/billing_store\.go)$'
ALL_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg -v "${SHIM_EXCLUDES}" || true)"
PROD_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg -v '_test\.go$' || true)"
TEST_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | rg '_test\.go$' || true)"

PAID_SURFACE_ALLOWLIST="$(cat "${PAID_SURFACE_ALLOWLIST_FILE}" 2>/dev/null | sed 's/#.*$//' | sed '/^$/d' | sort -u || true)"
PAID_SURFACE_ALLOWLIST_REGEX='^$'
if [[ -n "${PAID_SURFACE_ALLOWLIST}" ]]; then
	PAID_SURFACE_ALLOWLIST_REGEX="$(printf "%s\n" "${PAID_SURFACE_ALLOWLIST}" | sed 's/[.[\*^$()+?{|]/\\&/g' | sed 's#/#\\/#g' | paste -sd'|' -)"
fi

PROD_PAID_SURFACE_MATCHES="$(printf "%s\n" "${PROD_MATCHES}" | rg "${PAID_SURFACE_ALLOWLIST_REGEX}" || true)"
PROD_PRIVATE_IMPL_MATCHES="$(printf "%s\n" "${PROD_MATCHES}" | rg -v "${PAID_SURFACE_ALLOWLIST_REGEX}" || true)"

PAID_SURFACE_MISSING_FILES="$(printf "%s\n" "${PAID_SURFACE_ALLOWLIST}" | while IFS= read -r file; do
	[[ -z "${file}" ]] && continue
	if [[ ! -f "${file}" ]]; then
		printf "%s\n" "${file}"
	fi
done)"

API_INTERNAL_LICENSE_IMPORTS="$(rg -n 'internal/license/' internal/api/*.go 2>/dev/null | rg -v '_test\.go' || true)"
API_INTERNAL_LICENSE_ROOT_IMPORTS="$(rg -n '"github.com/rcourtman/pulse-go-rewrite/internal/license"' internal/api/*.go 2>/dev/null | rg -v '_test\.go' || true)"
API_ROOT_IMPORT_ALLOWLIST='^$'
API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST="$(printf "%s\n" "${API_INTERNAL_LICENSE_ROOT_IMPORTS}" | rg -v "${API_ROOT_IMPORT_ALLOWLIST}" || true)"
API_PKG_LICENSING_IMPORTS="$(rg -n '"github.com/rcourtman/pulse-go-rewrite/pkg/licensing' internal/api/*.go 2>/dev/null | rg -v '_test\.go' || true)"
API_PKG_LICENSING_IMPORT_FILES="$(printf "%s\n" "${API_PKG_LICENSING_IMPORTS}" | cut -d: -f1 | sed '/^$/d' | sort -u || true)"
API_PKG_LICENSING_IMPORT_ALLOWLIST='^internal/api/licensing_bridge\.go$'
API_PKG_LICENSING_IMPORT_FILES_OUTSIDE_ALLOWLIST="$(printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES}" | rg -v "${API_PKG_LICENSING_IMPORT_ALLOWLIST}" || true)"

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
prod_private_impl_count=0
prod_paid_surface_count=0
test_count=0
if [[ -n "${PROD_MATCHES}" ]]; then
  prod_count="$(printf "%s\n" "${PROD_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${PROD_PRIVATE_IMPL_MATCHES}" ]]; then
  prod_private_impl_count="$(printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${PROD_PAID_SURFACE_MATCHES}" ]]; then
  prod_paid_surface_count="$(printf "%s\n" "${PROD_PAID_SURFACE_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${TEST_MATCHES}" ]]; then
  test_count="$(printf "%s\n" "${TEST_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

paid_surface_missing_file_count=0
if [[ -n "${PAID_SURFACE_MISSING_FILES}" ]]; then
  paid_surface_missing_file_count="$(printf "%s\n" "${PAID_SURFACE_MISSING_FILES}" | sed '/^$/d' | wc -l | tr -d ' ')"
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

api_pkg_licensing_import_file_count=0
if [[ -n "${API_PKG_LICENSING_IMPORT_FILES}" ]]; then
	api_pkg_licensing_import_file_count="$(printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

api_pkg_licensing_import_outside_allowlist_count=0
if [[ -n "${API_PKG_LICENSING_IMPORT_FILES_OUTSIDE_ALLOWLIST}" ]]; then
	api_pkg_licensing_import_outside_allowlist_count="$(printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES_OUTSIDE_ALLOWLIST}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi

echo "Private boundary audit"
echo "  Production files in paid domains: ${prod_count}"
echo "  Production files in paid domains (private implementation leakage): ${prod_private_impl_count}"
echo "  Production files in paid domains (allowlisted paid surface adapters): ${prod_paid_surface_count}"
echo "  Test files in paid domains: ${test_count}"
echo "  API files importing internal/license: ${api_import_count}"
echo "  API root imports of internal/license: ${api_root_import_count}"
echo "  Non-API runtime imports of internal/license: ${non_api_import_count}"
echo "  API files importing pkg/licensing: ${api_pkg_licensing_import_file_count}"
echo "  API files importing pkg/licensing outside bridge: ${api_pkg_licensing_import_outside_allowlist_count}"

if [[ "${prod_count}" -gt 0 ]]; then
	echo
	echo "Production files currently in paid domains:"
	printf "%s\n" "${PROD_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${prod_private_impl_count}" -gt 0 ]]; then
	echo
	echo "Production files in paid domains outside paid-surface allowlist:"
	printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${prod_paid_surface_count}" -gt 0 ]]; then
	echo
	echo "Allowlisted paid-surface adapter files:"
	printf "%s\n" "${PROD_PAID_SURFACE_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${paid_surface_missing_file_count}" -gt 0 ]]; then
	echo
	echo "Paid-surface allowlist entries that do not exist:"
	printf "%s\n" "${PAID_SURFACE_MISSING_FILES}" | sed '/^$/d' | sed 's/^/  - /'
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

if [[ "${api_pkg_licensing_import_file_count}" -gt 0 ]]; then
	echo
	echo "API files importing pkg/licensing:"
	printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${api_pkg_licensing_import_outside_allowlist_count}" -gt 0 ]]; then
	echo
	echo "API files importing pkg/licensing outside bridge allowlist:"
	printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES_OUTSIDE_ALLOWLIST}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${MODE}" == "--enforce" && "${prod_private_impl_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: production paid-domain implementation files still live in public repo."
	exit 1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-paid-surface-allowlist") && "${paid_surface_missing_file_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: paid-surface allowlist contains paths that no longer exist."
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

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-pkg-licensing-imports") && "${api_pkg_licensing_import_outside_allowlist_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: API files outside bridge allowlist still import pkg/licensing."
	exit 1
fi

exit 0
