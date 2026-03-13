# Multi-Tenant Runtime Isolation and Coherence Record

- Date: `2026-03-13`
- Gate: `multi-tenant-runtime-isolation-and-coherence`
- Assertion: `RA12`
- Environment:
  - Managed local backend (seeded auth, then relaunched with proxy auth): `http://127.0.0.1:59221`
  - Entitlement profile: `multi-tenant`
  - Proxy-auth rehearsal identities:
    - `admin`
    - `alice`
    - `bob`
  - Seeded default-org live agent: `default-host.local`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers|TestMultiTenantStateProvider|TestMultiTenantAPITokenRemainsScopedToIssuingOrg' -count=1`
- `go test ./internal/monitoring -run 'TestMultiTenantMonitor' -count=1`
- `go test ./tests/migration -run 'TestV5DataDir_MultiTenantMigration' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts`
- `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/03-multi-tenant.spec.ts --project=chromium`
- Result: pass

## Manual Exercise

1. Started a clean managed local backend with `PULSE_MULTI_TENANT_ENABLED=true`, completed quick security setup, then relaunched the same data directory with proxy auth enabled so distinct users could be exercised over the live HTTP surface.
2. Seeded one live default-org unified agent report for `default-host.local` through `POST /api/agents/agent/report` as `admin`.
3. As `alice`, created org `mtgate-a-1773391667`.
4. As `bob`, created org `mtgate-b-1773391667`.
5. As `alice`, created org `mtgate-shared-1773391667`.
6. As `alice`, added `bob` to the shared org as `viewer`.
7. Listed `GET /api/orgs` as both users and confirmed membership-filtered visibility:
   - `alice` saw `default`, `mtgate-a-1773391667`, and `mtgate-shared-1773391667`
   - `bob` saw `default`, `mtgate-b-1773391667`, and `mtgate-shared-1773391667`
8. As `bob` while still `viewer`, attempted `PUT /api/orgs/mtgate-shared-1773391667` and received `403` with `Admin role required for this organization`.
9. As `alice`, promoted `bob` to `admin` in `mtgate-shared-1773391667`.
10. As `bob`, repeated `PUT /api/orgs/mtgate-shared-1773391667` and confirmed it succeeded with `200`.
11. As `alice`, demoted `bob` back to `viewer`.
12. As `bob`, repeated `PUT /api/orgs/mtgate-shared-1773391667` and confirmed it immediately failed again with `403`.
13. Verified tenant-scoped runtime isolation directly through the live lookup path:
    - `GET /api/agents/agent/lookup?hostname=default-host.local` in the default org returned `200` with the seeded live agent.
    - The same lookup with `X-Pulse-Org-ID: mtgate-a-1773391667` returned `404 agent_not_found`, confirming non-default orgs did not fall back to default-org runtime state.
14. As `alice`, created a cross-org share from `mtgate-a-1773391667` into `mtgate-b-1773391667` with `accessRole=editor`.
15. As `bob`, listed `GET /api/orgs/mtgate-b-1773391667/shares/incoming` and confirmed the incoming share preserved:
    - `sourceOrgId=mtgate-a-1773391667`
    - `accessRole=editor`
16. As `bob`, attempted `GET /api/orgs/mtgate-a-1773391667/shares` and confirmed the source-org share list remained blocked with `403` and `User is not a member of the organization`.
17. Deleted the temporary share and all temporary orgs after the rehearsal.

## Outcome

- Multi-tenant org visibility stayed scoped to actual membership.
- Membership role changes immediately changed the allowed write surface for the shared organization.
- Tenant-scoped runtime lookup failed closed for a non-default org instead of falling back to default-org live agent state.
- Cross-org sharing preserved the intended access role and did not widen source-org visibility.
- The live HTTP surface enforced tenant isolation consistently across org creation, membership, role changes, runtime lookup, and share visibility.

## Lifecycle Regression Revalidation

1. After the same-day rehearsal exposed shutdown-time alert-history save errors during org deletion, I patched tenant removal so it cancels the tenant runtime, waits for the monitor loop to exit, and only then flushes tenant state and removes the org directory.
2. Re-ran the automated proof surfaces that own this boundary:
   - `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers|TestMultiTenantStateProvider|TestMultiTenantAPITokenRemainsScopedToIssuingOrg|TestRBACLifecycle' -count=1`
   - `go test ./internal/monitoring -run 'TestMultiTenantMonitor' -count=1`
   - `go test ./tests/migration -run 'TestV5DataDir_MultiTenantMigration' -count=1`
3. Re-ran the managed-runtime deletion path on a fresh local backend at `http://127.0.0.1:59231`:
   - created `mtfix-live-1773396128` as `alice`
   - forced tenant monitor initialization through `GET /api/alerts/config` with `X-Pulse-Org-ID: mtfix-live-1773396128`
   - deleted the org with `DELETE /api/orgs/mtfix-live-1773396128`
4. Verified the live server log showed the corrected shutdown order for the initialized tenant:
   - `stopping and removing tenant monitor`
   - `monitoring loop stopped`
   - `stopping monitor`
   - `monitor stopped`
5. Verified the previous shutdown fault did not recur:
   - no `Failed to save alert history on shutdown`
   - no missing `alerts/alert-history.json.tmp*` write errors during tenant deletion

## Gate Decision

- `multi-tenant-runtime-isolation-and-coherence` is now satisfied at the required `managed-runtime-exercise` tier.
- The earlier org-deletion cleanup fault was reproduced, fixed, and revalidated on the live managed-runtime surface before closing the gate.

## Notes

- The browser-level multi-tenant suite passed separately on a fresh managed local backend, including CRUD, cross-org token isolation, self-role denial, cross-org share handling, and scoped permission updates.
- The runtime-state rehearsal intentionally used the direct agent lookup route because it reflects live host-agent inventory immediately, while the generic resource list on a fresh backend can remain empty until unrelated polling state is populated.
