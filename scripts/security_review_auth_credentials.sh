#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(unset CDPATH; cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname -- "$SCRIPT_DIR")"

cd "$REPO_ROOT"

printf 'Pulse authentication and credential review baseline\n'
printf 'Commit: %s\n' "$(git rev-parse HEAD)"
go version

printf '\nRunning cryptography, authentication, and configuration package tests\n'
go test ./internal/crypto ./pkg/auth ./internal/config -count=1

printf '\nVerifying the required API security regression inventory\n'
./scripts/ensure_test_assets.sh

# Keep this list exact rather than relying on a broad regex. Go treats a test
# command that matches no tests as successful, which would let a rename or
# deletion silently weaken the external-review baseline.
required_api_tests=(
  TestAPIOnlyModeRequiresToken
  TestAllowUnprotectedExportCannotOverrideAuthentication
  TestAllowUnprotectedExportIsExportOnly
  TestAuthenticatedEndpointsRequireToken
  TestBearerAPITokenScopesDenyReadWriteAndExecRoutes
  TestCheckAuth_QueryTokenRejectedWithoutWebSocketUpgrade
  TestConfigTransferAnonymousAuthenticatedModesDenyBeforeBodyRead
  TestConfigTransferEnvironmentOIDCAndSSOLoadFailureFailClosed
  TestConfigTransferNoAuthRejectsForwardedLoopback
  TestConfigTransferNoAuthUsesDirectLoopbackPolicy
  TestConfigTransferAuthorizedInstanceModesReachHandler
  TestConfigTransferSSOViewerDeniedBeforeBodyRead
  TestConfigTransferTenantSessionsRequireManagement
  TestConfigTransferTokenScopesAndOrganizationBinding
  TestDeniedConfigTransferDoesNotMutateOrReload
  TestHandleChangePassword_InvalidatesSessionsDocker
  TestHandleLogout_Post
  TestLimitedAPITokenCannotCreateBroaderToken
  TestNewOIDCHTTPClient_BlocksCrossOriginRedirects
  TestOIDCServiceAuthCodeURLIncludesPKCE
  TestProxyAuthNonAdminCannotEscalateWithToken
  TestRequireAuth_ProxyAuthInvalidSecretRejects
  TestRevokedAPITokenImmediatelyLosesAccess
  TestRouterCSRFBlocksCrossSiteProxyAuthMutation
  TestRouterCSRFEnforcedForSessionRequests
  TestSecurityStatusMatchesConfigTransferPolicy
  TestSecurityTokens_Create_RejectsScopeEscalationForTokenCaller
  TestSecurityTokens_DeleteFailsAuthImmediately
  TestSecurityTokens_ExpiredTokenRejectedAtHTTPLayer
  TestSecurityTokens_ListCreateDelete
  TestSecurityTokens_RotateRejectsScopeEscalation
  TestSessionStore_CreateAndValidate
  TestSessionStore_Load_MigratesLegacyFormat
  TestSessionStore_Persistence
  TestSessionStore_ValidateSession_Expired
  TestSSOOIDCCallbackProviderMismatchStillRejected
  TestTenantMiddleware_AuthorizationAllowed
  TestTenantMiddleware_AuthorizationDenied
  TestTenantMiddleware_DefaultOrgAuthorizationDenied
  TestTenantMiddleware_RejectsUnknownOrgBeforeLicense
  TestValidateSAMLRedirectTarget
)

printf -v api_test_alternation '|%s' "${required_api_tests[@]}"
api_test_pattern="^(${api_test_alternation:1})$"
api_test_inventory="$(go test ./internal/api -list "$api_test_pattern")"

for test_name in "${required_api_tests[@]}"; do
  if ! grep -Fxq "$test_name" <<<"$api_test_inventory"; then
    printf 'Required API security regression is missing: %s\n' "$test_name" >&2
    exit 1
  fi
done

printf 'Verified %d required API security regressions\n' "${#required_api_tests[@]}"

printf '\nRunning session, token, proxy, tenant, and configuration-transfer regressions\n'
go test ./internal/api -run "$api_test_pattern" -count=1

printf '\nChecking public documentation links and mirrors\n'
python3 scripts/check_public_docs.py

printf '\nAuthentication and credential review baseline passed\n'
