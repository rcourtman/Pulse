# Known RC Issue Closure For GA RC3 Follow-Up Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The earlier `2026-05-01` RC3 maintenance audit left four follow-up candidates:
`#1441`, `#1452`, discussion `#1448`, and `#1435`.

## Disposition

1. `#1441` server offline shown as online:
   - Fixed in the v6 connection-system grouping model by preserving the most
     severe member state when node and unified-resource snapshots describe the
     same Proxmox system member.
   - Regression proof:
     - `go test ./internal/api -run 'TestMergeConnectionSystemMembersKeepsMostSevereState|TestBuildConnectionSystems_ClusterMemberAgentsAttachToOwningProxmoxSystem'`
     - `npm --prefix frontend-modern test -- src/components/Settings/__tests__/useConnectionsLedger.test.ts`
   - Browser proof:
     - Settings connection rollup was exercised in the dev build with a forced
       `/api/connections` response. The `homelab` system and the `minipc`
       member both rendered `Unreachable`.

2. `#1452` graph tooltip overlaps graph:
   - Fixed in the shared history-chart tooltip layout by preferring side
     placement beside the hovered point, clamping the tooltip inside the chart,
     and only falling back to top/bottom placement on cramped charts.
   - Regression proof:
     - `npm --prefix frontend-modern test -- src/components/shared/__tests__/HistoryChart.test.tsx`
   - Browser proof:
     - The storage Capacity Trend tooltip rendered beside the hover line instead
       of covering the hovered graph point.

3. Discussion `#1448` PBS Alert Thresholds potentially broken:
   - Screenshots show a PBS memory threshold configured as `Off` while an email
     notification still reports `90.8% of 85%`.
   - Fixed in alert config reevaluation: canonical metric alerts whose metadata
     says `resourceType: PBS` now resolve against PBS thresholds even when the
     alert `Instance` field contains the PBS server name rather than the literal
     string `PBS`.
   - Regression proof:
     - `go test ./internal/alerts -run 'TestReevaluateActiveAlertsUsesPBSResourceTypeMetadata|TestCheckPBSComprehensive' -count=1`

4. `#1435` release sequencing:
   - Not a v6 RC code blocker. The stable installer/latest-release sequencing
     issue remains a v5 publication/backfill concern for the next stable asset,
     but it does not represent a known defect carried by the v6 candidate.

## Additional Proof

- `go test ./internal/api -count=1`
- `npm --prefix frontend-modern test -- src/components/shared/__tests__/HistoryChart.test.tsx src/components/Settings/__tests__/useConnectionsLedger.test.ts`
- `./node_modules/.bin/prettier --check src/components/shared/historyChartModel.ts src/components/shared/__tests__/HistoryChart.test.tsx src/components/Settings/__tests__/useConnectionsLedger.test.ts`
- `git diff --check`

## Outcome

The RC3 follow-up candidates that affect the v6 candidate are now fixed with
targeted regression coverage and browser proof for the customer-facing
surfaces. The `known-rc-issue-closure-for-ga` gate is satisfied for the current
v6 release candidate.
