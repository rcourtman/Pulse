# Infrastructure Default Landing Dashboard Retirement

Date: 2026-04-29
Lane: First-session post-RC polish
Follow-up: `first-session-post-rc-polish`
Assertion: `infrastructure-default-landing-surface`

## Outcome

Pulse v6 now treats Infrastructure as the authenticated landing surface. The old Dashboard route is retired instead of preserved as a compatibility wallboard: primary navigation no longer includes Dashboard, root runtime routing sends authenticated operators to Infrastructure, and first-session/setup completion handoffs now point into Infrastructure or Add infrastructure.

The Dashboard-specific overview stack is also removed rather than rebranded locally. Workload table code that still belongs to the product is owned under `frontend-modern/src/components/Workloads/`, and the deleted Dashboard overview API (`/api/resources/dashboard-summary`) is absent from the router, handler inventory, client API, and route tests.

## Proof

- `npm --prefix frontend-modern run type-check`
  - Result: pass.
- `npm --prefix frontend-modern run lint`
  - Result: pass; ESLint, theme audit, and canonical platform audit completed.
- `npm --prefix frontend-modern test -- src/components/Workloads src/pages/__tests__/Workloads.helpers.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts src/utils/__tests__/workloadEmptyStatePresentation.test.ts src/utils/__tests__/workloadGuestPresentation.test.ts src/__tests__/App.architecture.test.ts src/__tests__/AppLayout.test.tsx src/routing/__tests__/navigation.test.ts`
  - Result: pass, 36 files and 563 tests.
- `go test ./internal/api -run 'Test(RouterRouteInventory|RouteInventoryContractCoversAllRouteModules|RouterFrontendRouteInventory|ResourceList|ResourceGet|ResourceAndStorageResponses|ResourceContract|ApplyResource|ComputeResource|UnsupportedResourceType|ParseResource|SecurityStatus|SecurityTokens|RouterCSRF|AdminEndpoints|SSO|OIDC|SAML|MagicLink|HandlePublicMagicLink|EstablishOIDCSession|EstablishSAMLSession|BuildSSOOIDCCallbackURL|SanitizeOIDCReturnTo|RedirectOIDC|RedirectSAML|Contract_Resource|Contract_Security|Contract_ResetFirstRunSecurity)' -count=1`
  - Result: pass.
- `python3 scripts/release_control/contract_audit.py --pretty`
  - Result: pass.
- `python3 scripts/release_control/status_audit.py --pretty`
  - Result: pass.
- `python3 scripts/release_control/registry_audit.py --pretty`
  - Result: pass.
- `python3 scripts/release_control/canonical_completion_guard.py`
  - Result: pass.
- `python3 scripts/release_control/canonical_completion_guard_test.py`
  - Result: pass, 136 tests.
- `python3 scripts/release_control/subsystem_lookup_test.py`
  - Result: pass, 181 tests.
- Browser proof on the local v6 app:
  - Authenticated `/` redirected to `/infrastructure`.
  - `/infrastructure` rendered the Infrastructure heading with no Dashboard primary-nav tab.
  - `/dashboard` rendered the Not Found page with no Workloads surface and no Dashboard primary-nav tab.

## Verification Residual

- `go test ./internal/api -count=1`
  - Result: fail outside this slice. `TestSLO_WorkloadCharts` exceeded its strict local p95 SLO under package load (`p95=92.83075ms` against a 90ms target), and `TestSLO_WorkloadsSummaryCharts` exceeded the same local p95 SLO (`p95=135.267ms` against a 90ms target). The focused API route/resource/auth contract tests above are green.
