# Organization User Scope And RBAC Record

- Date: `2026-03-12`
- Gate: `organization-user-scope-and-rbac`
- Environment:
  - Managed local backend: `http://127.0.0.1:51688`
  - Multi-tenant entitlement profile: `multi-tenant`
  - Live distinct users exercised through proxy-auth headers:
    - `alice`
    - `bob`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers' -count=1`
- `go test ./internal/monitoring -run 'TestMultiTenantMonitor'`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/03-multi-tenant.spec.ts --project=chromium`
- Result: pass (`6 passed`, `1 skipped` for the multi-tenant-disabled kill-switch scenario)

## Manual Exercise

1. Seeded a clean managed local backend with the canonical quick security setup, then relaunched the same data directory with proxy-auth enabled so the live HTTP surface could exercise distinct users `alice` and `bob`.
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

## Outcome

- Users saw only orgs they belonged to.
- Membership role changes immediately changed the allowed write surface for that org.
- Self role modification stayed blocked with the canonical `self_modification_denied` error.
- Cross-org sharing preserved the intended access role and did not expose source-org management to the target-org user.
- The live HTTP surface enforced least privilege consistently across org membership, role change, and cross-org sharing behavior.

