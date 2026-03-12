# API Token Scope And Assignment Record

- Date: `2026-03-12`
- Gate: `api-token-scope-and-assignment`
- Environment:
  - Managed local backend: `http://127.0.0.1:61530`
  - Multi-tenant entitlement profile: `multi-tenant`
  - Authenticated user under test: `admin`

## Automated Proof Baseline

- `go test ./internal/api -run 'Test(APIToken|SecurityTokens|SystemSettings|MultiTenant)' -count=1`
- `go test ./internal/api -run 'TestNormalizeRequestedScopesCanonicalizesLegacyHostAgentAliases|TestHostAgentEndpointsAcceptLegacyHostAgentReportScopeAlias|TestContract_APITokenScopeAliasNormalization' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/APITokenManager.test.tsx src/utils/__tests__/apiClient.org.test.ts src/utils/__tests__/apiTokenPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/13-api-token-scope.spec.ts --project=chromium`
- Result: pass

## Manual Exercise

1. Logged into the managed local backend as `admin` through `POST /api/login` and used the issued session plus CSRF cookies for token-management requests.
2. Created an owner-bound API token with `settings:read` and confirmed the create response bound `ownerUserId=admin`.
3. Used that token to read `GET /api/system/settings` successfully.
4. Confirmed the same token was denied on:
   - `POST /api/security/tokens` with `requiredScope=settings:write`
   - `POST /api/ai/execute/stream` with `requiredScope=ai:execute`
   - `PATCH /api/agents/agent/host-1/config` with `requiredScope=agent:manage`
5. Revoked the owner-bound token through `DELETE /api/security/tokens/{id}` and confirmed the stale bearer token immediately failed on `GET /api/system/settings` with `401`.
6. Created two orgs:
   - `manual-token-org-a-1773352558099-998718`
   - `manual-token-org-b-1773352558099-245536`
7. Created an org-bound token while scoped to org A and confirmed the create response still bound `ownerUserId=admin`.
8. Confirmed that org-bound token could read `GET /api/orgs/{orgA}/members` with `200` but failed against `GET /api/orgs/{orgB}/members` with `403` and `Token is not authorized for this organization`.
9. Created a token using legacy scope `host-agent:report` and confirmed the stored scope canonicalized to `agent:report`.
10. Used that legacy-report token against both:
    - `POST /api/agents/agent/report`
    - `POST /api/agents/host/report`
    Both requests reached the handler and failed only on intentionally invalid JSON with `400`, not on scope authorization.
11. Created a token using legacy scope `host-agent:config:read` and confirmed the stored scope canonicalized to `agent:config:read`.
12. Used that legacy-config token against both:
    - `GET /api/agents/agent/host-1/config`
    - `GET /api/agents/host/host-1/config`
    Both requests passed scope authorization and failed only because the synthetic host had not registered yet, returning `404 agent_not_found`, not `403`.
13. Deleted the temporary org-bound and legacy-alias tokens and removed both temporary orgs after the exercise.

## Outcome

- Session-created API tokens stayed bound to the authenticated user identity.
- Org-bound tokens stayed confined to the issuing org.
- Read, mutate, and exec scope enforcement returned the expected `missing_scope` failures with the canonical required scope names.
- Revocation invalidated bearer tokens immediately.
- Legacy persisted `host-agent:*` scope aliases canonicalized to the intended v6 `agent:*` scopes and passed the canonical report/config scope gates.

