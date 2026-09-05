# Scoped sharing principal — 5 September 2026

Main-based source: 2356e4300022a9e0502b97030b84fc902ca15281 plus
this additive fix. No release image changed or published.

## Diagnosis

Fresh official external evidence: https://playwright.dev/docs/auth distinguishes
cookie storage state from sessionStorage. This informs diagnostic method, not
new product demand. This work repairs an existing authenticated flow.

The source-built cookie-session baseline reproduced 5 passes, 1 failure and
1 skip. Scenario 6 failed on missing Accept; screenshot inspected and showed the
admin/owner-required warning. Relationship-only instrumentation showed:

| Property | Before switch | After switch |
| --- | --- | --- |
| Security status detail | privileged | authenticated |
| Presented username available | yes | no |
| Session hint matches fixture login | yes | yes |
| Source and target owner match fixture login | yes | yes |
| Source and target login membership | owner | owner |

No credentials or raw identities are retained here. The actual endpoint is
`/api/security/status` (not `/api/security-status`).

The scoped response correctly withholds instance-administrator configuration,
including `authUsername`. Settings incorrectly relied on that configuration
field to identify local users. The fix exposes `currentUsername` from the
already-validated authentication snapshot, only in authenticated responses, and
uses it for organisation panel identity. Explicitly empty principals cannot fall
back to the configured administrator. Older responses retain the legacy fallback.
No membership, role, token restriction or server-side permission check changed.

## Validation

- Focused API security-status and settings-enforcement tests passed. New test
  covers public omission, scoped ordinary local identity and scoped configured
  administrator identity while retaining denied instance-settings capability.
  It invokes the status mux with an explicit organisation context; it does not
  qualify tenant middleware. Browser coverage exercises the actual middleware.
- 40 focused frontend tests passed (registry, sharing panel, org utilities).
- 16 existing runner image/cleanup fixture checks passed; these do not prove
  signal-time credential or browser artifact cleanup.
- First browser invocation built successfully but could not load
  @playwright/test. Installed locked integration dependencies and reran.
- One intermediate fixed-image invocation was invalidated by an edit/restore of
  the running shell script and exited on a parsing error. The experimental runner
  changes were discarded; this is not a product failure or qualifying run.

## Cleanup scope and remaining work

Worker-finally cleanup remains insufficient: helpers.ts still uses the
checkout-shared `tests/integration/tmp/playwright-auth/shared-cookie-session.json`.
Runner signal traps tear down the stack but do not guarantee deletion of
invocation videos/reports or this cache. A future repair must stop/wait for child
writers before deleting owned output roots, isolate the shared cookie path per
invocation, and prove signal behaviour; merely adding rm to a trap can race
surviving writers. No cleanup repair is claimed in this commit.

Delayed outgoing admission, failed/superseded incoming admission and the requested
reconnect-recovery assertions remain separate unfinished qualification work.
This identity fix does not qualify those behaviours, stable release readiness,
or installed customer operation. Do not promote the quarantined diagnostic tier.

## Follow-on matrix finding

The first fixed run passed scenario 6 but reported 5 passes and 2 skips, not a
complete enabled matrix. Scenario 7's feature probe silently treated HTTP 401 as
an absent entitlement. Added a fail-closed assertion for this explicitly enabled
fixture, which reproduced the 401. Switching back to default before deleting
scenario 6's organisations did not resolve it; no causal claim is made for that
experiment. Scenario 7 also mixes browser token auth with ambient cookie-based
API requests; its retained change uses the existing cookie-session login instead.

Reproduction commands:

```sh
pulse-heavy-run -- bash tests/integration/scripts/run-tests.sh multi-tenant
bash scripts/ensure_test_assets.sh
go test ./internal/api -run 'TestSecurityStatus|TestSettingsCapabilitiesMatchRouteEnforcementWithoutRBAC' -count=1
npm --prefix frontend-modern test -- src/components/Settings/__tests__/settingsPanelRegistryContext.test.ts src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/utils/__tests__/orgUtils.test.ts
```

The focused Go selector covers 23 top-level tests. Local Go tests use the
repository's dummy embed asset helper, not a frontend release build. All browser
runs used source-built e2e_runtime and mock fixtures through pulse-heavy-run.

## Final browser result

Final source-built matrix: **6 passed, 1 skipped**, exit 0, 35.2 seconds.
Scenario 6 passed (accept control, accepted API status, intended viewer role);
scenario 7 passed scoped RBAC updates. Only the disabled-feature scenario skipped.
Before and after switching, currentUsername matches the fixture login and both
organisation owners; detail still changes from privileged to authenticated.

Project: pulse-e2e-1058aa3ef256a5f155d1dfe80934cd60

Loopback HTTP / agent / mock ports: 32820 / 32821 / 32819.

Server: sha256:6ce8b61b624214c3f34a4a6815ec1871b6d409e4d461d38b0079e10794a9cc18

Mock: sha256:7eadf2ea47737afbadb86ba7725081a8ce613a6903092b3af768bcc04144c049

After completion, label-scoped container/network/volume inventories were empty
for all seven invocations. Removed only their owned browser report/result roots
and this isolated checkout's shared cookie cache. This was manual post-run
cleanup, not proof of automatic interruption cleanup.

## Bounded preflight repair

The original commit lacked same-commit subsystem contracts, registry-recognised
proof paths and a matching browser receipt. No contract-neutral exemption used.
Documented the identity-only API addition and its six subsystem boundaries;
moved the scoped/public API proof into contract_test.go and updated the Settings
architecture proof (including its stale legacy-fallback source assertion).
78 focused architecture/registry tests and the migrated scoped API test pass.

Fresh desktop project pulse-e2e-c183e95ab1168905085b1c9996705dd8 and narrow project
pulse-e2e-76ec75fc0fee54b9aa57f9ad225f2652 each returned 6 passes and one expected
disabled-feature skip. Scenario 6 narrow proof uses 390x844, desktop uses
1280x720. Inspected pending/accepted screenshots from both. Narrow scrolled
tables are clipped: functional acceptance passed, visual polish is not claimed.
The committed browser receipt binds the unchanged runtime source hashes and
records these limits. External authentication guidance was reread from
https://playwright.dev/docs/auth; cookie artifacts remain sensitive even for
fixture accounts.

Repair validation: 23 focused API tests passed after moving the proof. Canonical
completion, browser receipt, governance-stage and staged-shape guards passed;
contract, control-plane, status and registry audits exited zero. These governance
results do not establish release readiness. Both repair invocation resource
inventories were empty, and owned browser roots/shared checkout cookie cache
were manually removed after screenshot inspection.
