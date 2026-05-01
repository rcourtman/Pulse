# Known RC Issue Closure For GA Metric Threshold Coloring Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The v5 maintenance delta audit found that `#1358` was not fully carried into
v6. Pulse v5 had been fixed so metric color bands respected configured alert
thresholds, but the v6 Workloads, Infrastructure, and Storage display helpers
still colored CPU, memory, and disk bars from static display constants.

That meant an operator could lower, raise, or disable alert thresholds in the
alert configuration and still see v6 runtime metric bars use the old hard-coded
color bands.

## Disposition

The v6 candidate now resolves display metric colors through the alert
configuration path:

- `frontend-modern/src/utils/metricThresholds.ts` resolves display thresholds
  from alert defaults, hysteresis trigger/clear pairs, disabled thresholds, and
  resource overrides.
- `frontend-modern/src/stores/alertsActivation.ts` exposes that resolver to
  frontend consumers through the active alert configuration.
- Workloads route guest rows and drawers through guest/Docker scope identity
  candidates before coloring CPU, memory, and disk bars.
- Infrastructure and Storage consumers pass alert-backed threshold props into
  their existing metric bar models instead of selecting thresholds locally.
- The legacy static `METRIC_THRESHOLDS` path remains only as fallback display
  behavior for callers that do not have alert configuration in scope.

## Proof

- `npm --prefix frontend-modern run type-check`
- From `frontend-modern`: `npm exec vitest run src/utils/__tests__/metricThresholds.test.ts src/components/Workloads/MetricBar.test.tsx src/components/Workloads/__tests__/EnhancedCPUBar.test.tsx src/components/Workloads/__tests__/GuestRow.test.tsx src/components/Workloads/__tests__/WorkloadsSurface.performance.contract.test.tsx src/components/Workloads/__tests__/WorkloadsSurface.k8s.test.tsx src/components/shared/__tests__/InfrastructureSummaryTable.test.tsx src/components/Storage/__tests__/Storage.test.tsx src/components/Storage/__tests__/useEnhancedStorageBarModel.test.ts`
- Browser proof on `http://127.0.0.1:5173/workloads`: page loaded after reload,
  Workloads table rendered with metric bars, backend/live data showed connected,
  and browser console error count was `0`.
- Local threshold sanity check:
  `curl -s http://127.0.0.1:7655/api/alerts/config | jq '{activationState, hysteresisMargin, guestDefaults: .guestDefaults, dockerDefaults: .dockerDefaults}'`
  showed active guest thresholds and Docker CPU/memory/disk triggers set to
  `0`, matching the neutral high-CPU Docker row observed in-browser.

## Outcome

The v6 candidate no longer knowingly regresses v5 `#1358`. Runtime metric
colors now respect configured alert thresholds, including disabled Docker
thresholds, while keeping static display constants as a compatibility fallback
for threshold-unaware callers.
