# Known RC Issue Closure For GA Duplicate Metrics Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The RC3 issue triage found `#1442` as an additional credible v6 blocker
candidate. The reporter saw the browser tab grow to roughly 10 GB and the
server logs repeatedly emitted:

- `UNIQUE constraint failed: metrics.resource_type, metrics.resource_id, metrics.metric_type, metrics.timestamp, metrics.tier`

The current v6 metrics schema already declares that tuple as unique through
`idx_metrics_unique`, but the raw write path still used a plain insert. Two
samples for the same resource, metric, timestamp, and tier could therefore
produce noisy write failures and leave the metrics writer retrying/logging a
condition that should be idempotent.

## Disposition

`#1442` is fixed in the canonical metrics persistence layer:

- `pkg/metrics/store.go` now writes buffered metrics with an `ON CONFLICT`
  upsert against the existing uniqueness contract.
- The duplicate sample keeps one row and the latest buffered value wins.
- `min_value` and `max_value` are cleared on raw duplicate writes, matching the
  raw metric shape where those aggregate columns are derived at query time.

This is the root fix for the v6 regression candidate because the duplicate
tuple is owned by the metrics store schema, not by any one caller or frontend
view.

## Additional RC3 Triage

The same pass reviewed recent v5 fixes and current issue/discussion candidates:

- `#1447`, `#1446`, `#1438`, and `#1437` already have matching fixes on
  `pulse/v6-release` for agent identity persistence, mdstat operation gating,
  linked host/guest filesystems, and snapshot continuity.
- `#1433` and `#1427` are legacy v5 dashboard-specific reports and do not map
  to the current v6 default surface as RC3 code blockers.
- `#1443` is real v6 product feedback on interface density, but it is not a
  narrow RC3 defect fix candidate for this slice.
- Discussion `#876` and `#1434` map back to installer and entitlement issues
  already covered by the earlier RC3 follow-up fixes.
- Discussion `#1448` is already covered by the PBS threshold fix from the RC3
  follow-up record.

## Proof

- `go test ./pkg/metrics -count=1`

## Outcome

The remaining credible duplicate metrics RC3 candidate is fixed with targeted
regression coverage. The `known-rc-issue-closure-for-ga` gate remains satisfied
for the current v6 release candidate.
