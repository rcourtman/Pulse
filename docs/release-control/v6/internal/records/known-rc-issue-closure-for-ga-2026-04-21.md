# Known RC Issue Closure For GA Record

- Date: `2026-04-21`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `pass`

## Candidate Issue Disposition

1. `#1435` (`[Bug]: LXC command installing v6.0.0-rc.2`)
   - Fixed on `pulse/v6-release` by `4711d1116` (`Fix fresh Proxmox LXC installs defaulting to RC`).
   - Verification:
     - `go test ./scripts/installtests`
   - Result:
     - the stable install path now stays on the stable line instead of defaulting fresh Proxmox LXC installs to the prerelease tag.

2. `#1409` (`No limit devices for self-hosted / homelab`)
   - Fixed on the v6 line by the uncapped self-hosted cap scrub and expired-entitlement continuity fixes:
     - `943389827` (`Scrub stale monitored-system caps on self-hosted uncapped tiers`)
     - `770cceae5` (`Fix self-hosted community entitlements reporting expired state`)
   - Verification:
     - `go test ./pkg/licensing ./internal/api`
     - `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npm test -- src/components/Settings/__tests__/ProLicensePanel.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/pricingHandoff.test.ts`
   - Result:
     - self-hosted Community / Relay / Pro no longer carry the stale rc.2 monitored-system cap posture into the GA candidate.

3. `#1429` (`missing docker info including updates`)
   - Treated as an umbrella trust report rather than one atomic bug. The user-visible failures admitted in the thread were decomposed and covered on the current candidate:
     - stale self-hosted cap copy: covered by `943389827` and `770cceae5`
     - unavailable compare-plans / Pulse Account handoff: covered by `429f12dec` (`Recover unavailable Pulse Account handoffs`)
     - confusing empty trend state: covered by `9de093725` (`Clarify dashboard workload and trend states`)
     - v5-to-v6 wayfinding gap: covered by the current guided welcome / migration surfaces already on `pulse/v6-release`
   - Verification:
     - `go test ./pkg/licensing ./internal/api`
     - `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npm test -- src/components/Settings/__tests__/ProLicensePanel.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/pricingHandoff.test.ts src/features/dashboardOverview/__tests__/TrendCharts.test.tsx`
     - `cd /Volumes/Development/pulse/repos/pulse/tests/integration && PULSE_E2E_SKIP_DOCKER=1 PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm test -- tests/55-self-hosted-upgrade-return.spec.ts --project=chromium`
   - Result:
     - the GA candidate no longer knowingly carries the specific cap, handoff, or trend-state failures raised during the RC2 discussion.
     - the remaining GitHub thread hygiene is reporter-confirmation / maintainer-triage work, not an admitted GA product blocker.

4. `#1430` (`Width of the Name column`)
   - Fixed by consolidating the workload table sizing contract into the canonical dashboard column model and removing the legacy global CSS width rules that caused Firefox to expand the table to multi-million-pixel width.
   - Verification:
     - `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npx vitest run src/components/Dashboard/__tests__/GuestRow.test.tsx src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
     - `cd /Volumes/Development/pulse/repos/pulse/tests/integration && PULSE_E2E_SKIP_DOCKER=1 PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm test -- tests/59-workloads-column-layout.spec.ts --project=chromium`
     - managed browser Firefox proof on `http://127.0.0.1:5173/workloads` showed:
       - `wrapperClientWidth=1320`
       - `wrapperScrollWidth=1320`
       - `tableScrollWidth=1320`
       - `name` header width `200`
   - Result:
     - Firefox no longer blows the workloads table out horizontally; the current desktop table fits the shell with the bounded `Name` width contract intact.

5. `#1432` (`Dashbord filter`)
   - Already satisfied on the current candidate by the dashboard/workloads status filter path.
   - Verification:
     - `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npm test -- src/components/Dashboard/__tests__/DashboardFilter.test.tsx src/components/Dashboard/__tests__/workloadSelectors.test.ts`
   - Result:
     - the candidate already supports filtering the workloads slice by status (`All`, `Running`, `Degraded`, `Stopped`), so there is no missing offline-filter blocker to carry into GA.

6. `#1436` (`Better disk i/o reads for LXC containers`)
   - Fixed by merging prefetched LXC `status/current` counters into both container polling paths before rate calculation and reusing the same status snapshot for metadata enrichment.
   - Verification:
     - `go test ./internal/monitoring -run 'TestMergeContainerRuntimeCounters_PrefersHigherStatusCounters|TestBuildContainerFromClusterResource_UsesContainerStatusCountersForRates|TestBuildContainerFromClusterResource_PreservesProxmoxPool|TestEnrichContainerMetadata_DetectsOCIForStoppedContainer|TestMonitor_EnrichContainerMetadata_Extra'`
   - Result:
     - the GA candidate no longer depends solely on the lower-fidelity container list counters when current LXC runtime counters are available from the status endpoint.

## Outcome

- The RC-era issue set admitted into the v6 GA bar on `2026-04-21` is now covered by the candidate.
- Some GitHub issues may remain open until public maintainer triage catches up with the current release line, but the GA candidate no longer knowingly ships those RC-era failures unchanged.
- `known-rc-issue-closure-for-ga` is therefore satisfied for the candidate itself.
