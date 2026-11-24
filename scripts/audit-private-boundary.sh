#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-report}"
PAID_SURFACE_ALLOWLIST_FILE="scripts/repo-boundary-paid-surface.allowlist"
PRIVATE_IMPL_BASELINE_FILE="scripts/repo-boundary-private-implementation.baseline"

if [[ "${MODE}" != "report" && "${MODE}" != "--enforce" && "${MODE}" != "--enforce-api-imports" && "${MODE}" != "--enforce-api-root-imports" && "${MODE}" != "--enforce-nonapi-imports" && "${MODE}" != "--enforce-api-pkg-licensing-imports" && "${MODE}" != "--enforce-paid-surface-allowlist" ]]; then
  echo "Usage: $0 [report|--enforce|--enforce-api-imports|--enforce-api-root-imports|--enforce-nonapi-imports|--enforce-api-pkg-licensing-imports|--enforce-paid-surface-allowlist]" >&2
  exit 2
fi

if ! inside="$(git rev-parse --is-inside-work-tree 2>/dev/null)" ||
   [[ "$inside" != true ]] ||
   ! prefix="$(git rev-parse --show-prefix 2>/dev/null)" ||
   [[ -n "$prefix" ]]; then
  echo "Run the boundary audit from the Git repository root" >&2
  exit 2
fi

if ! git_diagnostic="$(mktemp)"; then
  echo "Boundary audit could not prepare Git diagnostics." >&2
  exit 2
fi
trap 'rm -- "$git_diagnostic"' EXIT

# git grep reports a quiet exit 1 for no matches. Any diagnostic means the
# inventory may be incomplete, even when grep also returned some matches.
read_git_inventory() {
  local phase="$1" result status=0
  shift
  result="$(git "$@" 2>"$git_diagnostic")" || status=$?
  if [[ ! -s "$git_diagnostic" ]]; then
    if [[ "$status" == 0 ]]; then
      printf '%s\n' "$result"
      return 0
    fi
    if [[ "$1" == grep && "$status" == 1 && -z "$result" ]]; then
      return 0
    fi
  fi
  printf 'Boundary audit could not %s (git %s exit %s).\n' "$phase" "$1" "$status" >&2
  return 2
}

# Ignore rules address workspace-only files, not files already in the index.
# Read tracked content separately: --no-index with excludes can skip it.
read_git_content() {
  local phase="$1" tracked workspace
  shift
  tracked="$(read_git_inventory "$phase in tracked files" grep "$@")" || return 2
  workspace="$(read_git_inventory "$phase in non-ignored workspace files" grep --no-index --exclude-standard "$@")" || return 2
  printf '%s\n%s\n' "$tracked" "$workspace" | sed '/^$/d' | sort -u
}

filter_matches() {
  local status=0
  grep "$@" || status=$?
  if [[ "$status" != 0 && "$status" != 1 ]]; then
    printf 'Boundary audit could not filter its inventory (grep exit %s).\n' "$status" >&2
    return 2
  fi
  return 0
}

read_policy_inventory() {
  local result
  if ! result="$(sed 's/#.*$//' "$1" | sed '/^$/d' | sort -u)"; then
    printf 'Boundary audit could not read policy inventory: %s.\n' "$1" >&2
    return 2
  fi
  printf '%s\n' "$result"
}

NAME_PATTERN='^(internal/license/|internal/api/(license_|entitlement_|billing_|stripe_|hosted_|rbac_|audit_|reporting_|sso_|conversion_|router_routes_org_license\.go$|router_routes_hosted\.go$|router_routes_licensing\.go$|router_routes_cloud\.go$))'
SEMANTIC_API_PATTERN='/api/license|/api/upgrade-metrics|/api/webhooks/stripe|/api/public/signup|/api/public/magic-link|/api/hosted/|/api/admin/orgs/\{id\}/billing-state|/api/security/sso|/api/admin/rbac|/api/audit|"github\.com/rcourtman/pulse-go-rewrite/pkg/licensing'

# Git and grep are already present on the checkout runner. Include untracked,
# non-ignored files as before, without installing a search tool from apt mirrors.
NAME_MATCHES="$(read_git_inventory 'list candidate paths' ls-files --cached --others --exclude-standard -- internal/license internal/api | filter_matches -E "${NAME_PATTERN}")"
SEMANTIC_API_MATCHES="$(read_git_content 'scan API route paths' -l -E -e "${SEMANTIC_API_PATTERN}" -- ':(glob)internal/api/*.go' ':(exclude,glob)internal/api/*_test.go')"
ALL_MATCHES="$(printf "%s\n%s\n" "${NAME_MATCHES}" "${SEMANTIC_API_MATCHES}" | sed '/^$/d' | sort -u)"
SHIM_EXCLUDES='^(internal/license/features\.go|internal/license/license\.go|internal/license/pubkey\.go|internal/license/persistence\.go|internal/license/subscription/state\.go|internal/license/subscription/transitions\.go|internal/license/conversion/upgrade_reasons\.go|internal/license/conversion/events\.go|internal/license/conversion/config\.go|internal/license/conversion/quality\.go|internal/license/conversion/metrics\.go|internal/license/conversion/store\.go|internal/license/conversion/recorder\.go|internal/license/metering/event\.go|internal/license/metering/aggregator\.go|internal/license/entitlements/types\.go|internal/license/entitlements/aliases\.go|internal/license/entitlements/source\.go|internal/license/entitlements/evaluator\.go|internal/license/entitlements/token_source\.go|internal/license/entitlements/database_source\.go|internal/license/entitlements/billing_store\.go)$'
ALL_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | filter_matches -Ev "${SHIM_EXCLUDES}")"
PROD_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | filter_matches -Ev '_test\.go$')"
TEST_MATCHES="$(printf "%s\n" "${ALL_MATCHES}" | filter_matches -E '_test\.go$')"

PAID_SURFACE_ALLOWLIST="$(read_policy_inventory "${PAID_SURFACE_ALLOWLIST_FILE}")"
PAID_SURFACE_ALLOWLIST_REGEX='^$'
if [[ -n "${PAID_SURFACE_ALLOWLIST}" ]]; then
	PAID_SURFACE_ALLOWLIST_REGEX="^($(printf "%s\n" "${PAID_SURFACE_ALLOWLIST}" | sed 's/[.[\*^$()+?{|]/\\&/g' | sed 's#/#\\/#g' | paste -sd'|' -))$"
fi

PROD_PAID_SURFACE_MATCHES="$(printf "%s\n" "${PROD_MATCHES}" | filter_matches -E "${PAID_SURFACE_ALLOWLIST_REGEX}")"
PROD_PRIVATE_IMPL_MATCHES="$(printf "%s\n" "${PROD_MATCHES}" | filter_matches -Ev "${PAID_SURFACE_ALLOWLIST_REGEX}")"

PAID_SURFACE_MISSING_FILES="$(printf "%s\n" "${PAID_SURFACE_ALLOWLIST}" | while IFS= read -r file; do
	[[ -z "${file}" ]] && continue
	if [[ ! -f "${file}" ]]; then
		printf "%s\n" "${file}"
	fi
done)"

PRIVATE_IMPL_BASELINE="$(read_policy_inventory "${PRIVATE_IMPL_BASELINE_FILE}")"
PRIVATE_IMPL_BASELINE_REGEX='^$'
if [[ -n "${PRIVATE_IMPL_BASELINE}" ]]; then
	PRIVATE_IMPL_BASELINE_REGEX="^($(printf "%s\n" "${PRIVATE_IMPL_BASELINE}" | sed 's/[.[\*^$()+?{|]/\\&/g' | sed 's#/#\\/#g' | paste -sd'|' -))$"
fi

PROD_BASELINED_PRIVATE_IMPL_MATCHES="$(printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | filter_matches -E "${PRIVATE_IMPL_BASELINE_REGEX}")"
PROD_NEW_PRIVATE_IMPL_MATCHES="$(printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | filter_matches -Ev "${PRIVATE_IMPL_BASELINE_REGEX}")"
PRIVATE_IMPL_STALE_BASELINE_ENTRIES="$(printf "%s\n" "${PRIVATE_IMPL_BASELINE}" | while IFS= read -r file; do
	[[ -z "${file}" ]] && continue
	if ! printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | grep -Fxq -- "${file}"; then
		printf "%s\n" "${file}"
	fi
done)"

API_INTERNAL_LICENSE_IMPORTS="$(read_git_content 'scan API internal licence imports' -n -E -e 'internal/license/' -- ':(glob)internal/api/*.go' | filter_matches -Ev '_test\.go')"
API_INTERNAL_LICENSE_ROOT_IMPORTS="$(read_git_content 'scan API root licence imports' -n -E -e '"github.com/rcourtman/pulse-go-rewrite/internal/license"' -- ':(glob)internal/api/*.go' | filter_matches -Ev '_test\.go')"
API_ROOT_IMPORT_ALLOWLIST='^$'
API_INTERNAL_LICENSE_ROOT_IMPORTS_OUTSIDE_ALLOWLIST="$(printf "%s\n" "${API_INTERNAL_LICENSE_ROOT_IMPORTS}" | filter_matches -Ev "${API_ROOT_IMPORT_ALLOWLIST}")"
API_PKG_LICENSING_IMPORTS="$(read_git_content 'scan API licensing bridge imports' -n -E -e '"github.com/rcourtman/pulse-go-rewrite/pkg/licensing' -- ':(glob)internal/api/*.go' | filter_matches -Ev '_test\.go')"
API_PKG_LICENSING_IMPORT_FILES="$(printf "%s\n" "${API_PKG_LICENSING_IMPORTS}" | cut -d: -f1 | sed '/^$/d' | sort -u)"
API_PKG_LICENSING_IMPORT_ALLOWLIST='^internal/api/licensing_bridge\.go$'
API_PKG_LICENSING_IMPORT_FILES_OUTSIDE_ALLOWLIST="$(printf "%s\n" "${API_PKG_LICENSING_IMPORT_FILES}" | filter_matches -Ev "${API_PKG_LICENSING_IMPORT_ALLOWLIST}")"

NON_API_INTERNAL_LICENSE_ROOT_IMPORTS="$(
  read_git_content 'scan non-API licence imports' -n -E \
    -e '"github.com/rcourtman/pulse-go-rewrite/internal/license"' -- \
    ':(glob)**/*.go' \
    ':(exclude,glob)**/*_test.go' \
    ':(exclude)internal/api' \
    ':(exclude)internal/license'
)"
NON_API_IMPORT_ALLOWLIST='^$'
NON_API_INTERNAL_LICENSE_IMPORTS="$(printf "%s\n" "${NON_API_INTERNAL_LICENSE_ROOT_IMPORTS}" | filter_matches -Ev "${NON_API_IMPORT_ALLOWLIST}")"

prod_count=0
prod_private_impl_count=0
prod_baselined_private_impl_count=0
prod_new_private_impl_count=0
prod_paid_surface_count=0
test_count=0
if [[ -n "${PROD_MATCHES}" ]]; then
  prod_count="$(printf "%s\n" "${PROD_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${PROD_PRIVATE_IMPL_MATCHES}" ]]; then
  prod_private_impl_count="$(printf "%s\n" "${PROD_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${PROD_BASELINED_PRIVATE_IMPL_MATCHES}" ]]; then
  prod_baselined_private_impl_count="$(printf "%s\n" "${PROD_BASELINED_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
fi
if [[ -n "${PROD_NEW_PRIVATE_IMPL_MATCHES}" ]]; then
  prod_new_private_impl_count="$(printf "%s\n" "${PROD_NEW_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | wc -l | tr -d ' ')"
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

private_impl_stale_baseline_count=0
if [[ -n "${PRIVATE_IMPL_STALE_BASELINE_ENTRIES}" ]]; then
  private_impl_stale_baseline_count="$(printf "%s\n" "${PRIVATE_IMPL_STALE_BASELINE_ENTRIES}" | sed '/^$/d' | wc -l | tr -d ' ')"
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
echo "  Production files in paid domains (baselined private implementation leakage): ${prod_baselined_private_impl_count}"
echo "  Production files in paid domains (new private implementation leakage): ${prod_new_private_impl_count}"
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

if [[ "${prod_baselined_private_impl_count}" -gt 0 ]]; then
	echo
	echo "Baselined production paid-domain implementation files:"
	printf "%s\n" "${PROD_BASELINED_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${prod_new_private_impl_count}" -gt 0 ]]; then
	echo
	echo "New production files in paid domains outside paid-surface allowlist:"
	printf "%s\n" "${PROD_NEW_PRIVATE_IMPL_MATCHES}" | sed '/^$/d' | sed 's/^/  - /'
fi

if [[ "${private_impl_stale_baseline_count}" -gt 0 ]]; then
	echo
	echo "Private-implementation baseline entries that no longer match:"
	printf "%s\n" "${PRIVATE_IMPL_STALE_BASELINE_ENTRIES}" | sed '/^$/d' | sed 's/^/  - /'
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

failed=0
if [[ "${MODE}" == "--enforce" && "${prod_new_private_impl_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: new production paid-domain implementation files are not baselined."
	failed=1
fi

if [[ "${MODE}" == "--enforce" && "${private_impl_stale_baseline_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: shrink the private-implementation baseline after removing or reclassifying files."
	failed=1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-paid-surface-allowlist") && "${paid_surface_missing_file_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: paid-surface allowlist contains paths that no longer exist."
	failed=1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-imports") && "${api_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: non-test internal API files still import internal/license."
  failed=1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-root-imports") && "${api_root_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: API root imports of internal/license remain."
  failed=1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-nonapi-imports") && "${non_api_import_count}" -gt 0 ]]; then
  echo
  echo "Boundary enforcement failed: non-API runtime files import internal/license."
  failed=1
fi

if [[ ("${MODE}" == "--enforce" || "${MODE}" == "--enforce-api-pkg-licensing-imports") && "${api_pkg_licensing_import_outside_allowlist_count}" -gt 0 ]]; then
	echo
	echo "Boundary enforcement failed: API files outside bridge allowlist still import pkg/licensing."
	failed=1
fi

exit "$failed"
