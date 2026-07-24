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

3. `#1421` (`[Bug]: QNAP shown as separate Host and Docker agents, even though only one agent is installed`)
   - Fixed on the current `pulse/v6-release` candidate by tightening the canonical hostname-equivalence contract used by top-level monitored-system grouping, monitored-system replacement selectors, and Docker host re-identification.
   - Verification:
     - `go test ./internal/unifiedresources -run 'TestHostnamesEquivalent|TestResolveTopLevelSystemsTopLevelSourceMatrix|TestProjectMonitoredSystemCandidateReplacementMatchesShortAndFQDNHostnames'`
     - `go test ./internal/unifiedresources`
     - `go test ./internal/monitoring -run 'TestFindMatchingDockerHost'`
   - Result:
     - the current candidate no longer splits one QNAP box into separate Host and Docker monitored-system rows just because one path reports `qnap` while another reports `qnap.local`.

4. `#1429` (`missing docker info including updates`)
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

5. `#1430` (`Width of the Name column`)
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

6. `#1432` (`Dashbord filter`)
   - Already satisfied on the current candidate by the dashboard/workloads status filter path.
   - Verification:
     - `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npm test -- src/components/Dashboard/__tests__/DashboardFilter.test.tsx src/components/Dashboard/__tests__/workloadSelectors.test.ts`
   - Result:
     - the candidate already supports filtering the workloads slice by status (`All`, `Running`, `Degraded`, `Stopped`), so there is no missing offline-filter blocker to carry into GA.

7. `#1433` (`[Bug]:  v5.1.28 - Want All Proxmox Nodes Displayed on the DashBoard even if they are not Powered On`)
   - The original candidate proof covered a static offline fixture and the
     browser row, but did not exercise a healthy partial `/nodes` poll replacing
     an earlier full cluster inventory. That gap allowed current main to remove
     an unpowered member before the browser received the next snapshot.
   - Superseded disposition:
     - see
       `known-rc-issue-closure-for-ga-proxmox-offline-membership-2026-07-24.md`
       for the provider-membership reconciliation fix, removal rule, restart
       proof, same-name-cluster isolation, downstream history/count proof, and
       desktop/mobile browser coverage.
   - Result:
     - the earlier static proof is retained as historical evidence but is no
       longer treated as sufficient lifecycle closure for `#1433`.

8. `#1436` (`Better disk i/o reads for LXC containers`)
   - Fixed by merging prefetched LXC `status/current` counters into both container polling paths before rate calculation and reusing the same status snapshot for metadata enrichment.
   - Verification:
     - `go test ./internal/monitoring -run 'TestMergeContainerRuntimeCounters_PrefersHigherStatusCounters|TestBuildContainerFromClusterResource_UsesContainerStatusCountersForRates|TestBuildContainerFromClusterResource_PreservesProxmoxPool|TestEnrichContainerMetadata_DetectsOCIForStoppedContainer|TestMonitor_EnrichContainerMetadata_Extra'`
   - Result:
     - the GA candidate no longer depends solely on the lower-fidelity container list counters when current LXC runtime counters are available from the status endpoint.

9. `#1319` (`Proxmox clusters can show incorrect VM RAM and intermittently lose guest disk or network metrics`)
   - Retested against the current `pulse/v6-release` candidate after reviewing the reporter screenshots that showed:
     - Pulse and Proxmox both overstating VM memory versus the in-guest reading
     - guest disk inventory disappearing intermittently
     - guest network interfaces appearing late or dropping out temporarily
   - The current v6 candidate already carries the canonical protections for that remaining issue scope:
     - VM memory trust characterization prefers guest-agent / RRD-derived availability over inflated low-trust Proxmox status samples and preserves the last good sample when a healthy guest briefly falls back to a low-trust full-usage reading
     - guest disk continuity carries forward the last good disk inventory when guest-agent filesystem calls transiently return no data
     - guest metadata continuity preserves the last known guest network/interface payload when the guest agent is temporarily unavailable or returns an empty interface list
   - Verification:
     - `go test ./internal/monitoring -run 'TestHandleClusterVMResourceMemoryTrustCharacterization|TestPollVMsWithNodesMemoryTrustCharacterization|TestStabilizeGuestLowTrustMemoryUsesHealthyGuestAgentEvidence|TestFetchGuestAgentMetadataPreservesFreshCacheWhenAgentTemporarilyUnavailable|TestGuestDiskTrustCharacterizationCarriesForwardRecentSnapshot'`
   - Result:
     - the current v6 candidate no longer knowingly lacks protection for the specific RC-era RAM inflation, disappearing guest disk, or disappearing guest interface symptoms tracked in `#1319`.
     - the GitHub issue may still remain open for reporter confirmation on a real v6 build, but it is no longer an unexamined RC3 blocker on the candidate itself.

10. `#1420` (`[Bug]: Pulse Agent does not auto-update on Qnap Platforms`)
   - Fixed on the current `pulse/v6-release` candidate by restoring the canonical QNAP persistence contract across both installer bootstrap and self-update:
     - `scripts/install.sh` again provisions a QNAP-specific persistent state directory and wrapper on the writable data volume instead of assuming `/usr/local/bin` survives reboot
     - `internal/agentupdate/update.go` now updates that persisted QNAP binary copy after a successful self-update so the next reboot keeps the same version the live runtime just installed
   - Verification:
     - `bash -n scripts/install.sh`
     - `go test ./internal/agentupdate`
     - `go test ./scripts/installtests`
     - `go test ./cmd/pulse-agent`
   - Result:
     - the current v6 candidate no longer knowingly carries the QNAP auto-update regression where the running agent could update in place but reboot back to the older persisted binary.

11. `#1422` (`[Bug]: agent does not start on QNAP boot, fails due to hard drive encyption?`)
   - Fixed on the current `pulse/v6-release` candidate by restoring the canonical QNAP boot path in a safer shape than the old implementation:
     - the installer now writes the QNAP `autorun.sh` entry as a deferred bootstrap on the flash-backed config partition
     - that bootstrap waits for the persistent data-volume wrapper to become available before launching it, instead of trying to execute the encrypted-volume wrapper directly during early boot
     - uninstall now removes the same QNAP `autorun.sh` block and persistent state directory through the installer-owned lifecycle path
   - Verification:
     - `bash -n scripts/install.sh`
     - `go test ./scripts/installtests`
     - `go test ./cmd/pulse-agent`
   - Result:
     - the current v6 candidate no longer knowingly depends on the old early-boot QNAP startup path that could fail before the encrypted data volume was available.

## Outcome

- The RC-era issue set admitted into the v6 GA bar on `2026-04-21` is now covered by the candidate.
- Some GitHub issues may remain open until public maintainer triage catches up with the current release line, but the GA candidate no longer knowingly ships those RC-era failures unchanged.
- `known-rc-issue-closure-for-ga` is therefore satisfied for the candidate itself.
