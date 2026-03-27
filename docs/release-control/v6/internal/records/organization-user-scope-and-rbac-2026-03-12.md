# Organization User Scope and RBAC Record

- Date: `2026-03-12`
- Gate: `organization-user-scope-and-rbac`
- Environment:
  - Managed local backend runs:
    - `http://127.0.0.1:51688`
    - `http://127.0.0.1:8766`
  - Entitlement profile: `multi-tenant`
  - Live distinct users exercised through proxy-auth headers:
    - `alice`
    - `bob`
    - `admin-user`
    - `viewer-user`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers' -count=1`
- `go test ./internal/monitoring -run 'TestMultiTenantMonitor' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/03-multi-tenant.spec.ts --project=chromium`
- Result: pass

## Manual Exercise A: Org Membership and Management Isolation

1. Seeded a clean managed local backend with the canonical quick security setup, then relaunched the same data directory with proxy auth enabled so the live HTTP surface could exercise distinct users `alice` and `bob`.
2. As `alice`, created org `manual-org-a-1773353229067-469132`.
3. As `bob`, created org `manual-org-b-1773353229067-199801`.
4. As `alice`, created shared org `manual-org-shared-1773353229067-962894`.
5. As `alice`, added `bob` to the shared org as `viewer`.
6. Listed `GET /api/orgs` as both users and confirmed membership-filtered visibility:
   - `alice` saw `default`, `manual-org-a-1773353229067-469132`, and `manual-org-shared-1773353229067-962894`
   - `bob` saw `default`, `manual-org-b-1773353229067-199801`, and `manual-org-shared-1773353229067-962894`
7. As `bob` while still `viewer`, attempted `PUT /api/orgs/{shared}` and received `403` with `Admin role required for this organization`.
8. As `alice`, promoted `bob` to `admin` via `POST /api/orgs/{shared}/members`.
9. As `bob`, repeated `PUT /api/orgs/{shared}` and confirmed it succeeded with `200`.
10. As `alice`, demoted `bob` back to `viewer`.
11. As `bob`, repeated `PUT /api/orgs/{shared}` and confirmed it immediately failed again with `403`.
12. As `alice`, attempted `PUT /api/admin/users/alice/roles` on her own account and confirmed the request failed closed with `403` and `code=self_modification_denied`.
13. As `alice`, created a cross-org share from org A into org B with `accessRole=editor`.
14. As `bob`, listed `GET /api/orgs/{orgB}/shares/incoming` and confirmed the incoming share preserved `accessRole=editor`.
15. As `bob`, attempted `GET /api/orgs/{orgA}/shares` and confirmed the source-org share list remained blocked with `403`.
16. Deleted the temporary share and all three temporary orgs after the rehearsal.

## Manual Exercise B: Scoped RBAC Permission Isolation

1. Created three organizations as `admin-user`:
   - `rbacscope20260312`
   - `rbachidden20260312`
   - `rbacsource20260312`
2. Added `viewer-user` to `rbacscope20260312` as an organization `viewer`.
3. Confirmed `viewer-user` could only see:
   - `default`
   - `rbacscope20260312`
   and could not see `rbachidden20260312` or `rbacsource20260312`.
4. Assigned scoped RBAC role `viewer` to `viewer-user` in `rbacscope20260312` and confirmed effective permissions were:
   - `read` on `*` in `rbacscope20260312`
   - empty in `rbachidden20260312`
5. Promoted the same scoped RBAC role to `admin` in `rbacscope20260312` and confirmed effective permissions changed to:
   - `admin` on `*` in `rbacscope20260312`
   - still empty in `rbachidden20260312`
6. Confirmed self-role mutation failed closed in both relevant ways:
   - `viewer-user` attempting to mutate their own roles was blocked before mutation with `403 Admin privileges required`
   - `admin-user` attempting to mutate their own roles was blocked with `403 self_modification_denied`
7. Created a cross-org share from `rbacsource20260312` to `rbacscope20260312` with `accessRole=editor`.
8. Confirmed `viewer-user` did not see that incoming share while their organization membership in `rbacscope20260312` remained `viewer`.
9. Promoted `viewer-user` organization membership in `rbacscope20260312` to `editor`.
10. Confirmed the same incoming share became visible only after that membership promotion, with the intended `editor` access role preserved.

## Outcome

- Organization visibility stayed scoped to membership.
- Membership role changes immediately changed the allowed write surface for the shared organization.
- Scoped RBAC permissions changed in the intended organization only and did not leak into a second organization.
- Self-role escalation failed closed.
- Cross-org shares preserved the intended access role and did not expose source-org management to the target-org user.
- The live HTTP surface enforced least privilege consistently across org membership, role change, scoped RBAC assignment, self-role mutation, and cross-org sharing behavior.

## Notes

- The default organization remains visible by design; the critical least-privilege check is that non-member custom organizations stayed hidden.
- The live managed-backend exercise was captured from the same API surfaces that power the Settings organization, roles, assignments, and sharing flows.
