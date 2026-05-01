# Known RC Issue Closure For GA Ceph Monitors Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

Recent discussion triage found discussion `#1290` as a remaining technical
candidate. The reporter's `v5.1.28` screenshot was inspected and showed the
Ceph page reporting:

- Managers: `3`
- Monitors: `0`
- OSDs: `9 / 9`
- PGs: `129`

That shape points to a partial Ceph status decode rather than a missing Ceph
cluster: the same status payload was rich enough to count managers and OSDs,
but the monitor count fell through to zero.

## Disposition

The current v6 Proxmox Ceph decoder had the same compatibility risk because
`pkg/proxmox/ceph.go` only accepted monitor totals through
`monmap.num_mons`. Some Ceph/Proxmox status payloads expose the concrete
monitor list under `monmap.mons` or `monmap.monitors` instead.

The fix is in the shared Proxmox Ceph compatibility layer:

- `pkg/proxmox/ceph.go` now normalizes monitor totals from
  `monmap.num_mons`, legacy `monmap.numMons`, `monmap.mons`, or
  `monmap.monitors`.
- The Ceph page and unified resource projection continue to consume the
  canonical `numMons` model field; no frontend or page-local workaround is
  needed.
- `docs/release-control/v6/internal/subsystems/monitoring.md` now records this
  monitor-count compatibility contract beside the existing Ceph manager standby
  payload rule.

## Proof

- `go test ./pkg/proxmox -run 'TestGetCephStatus|TestGetCephDF' -count=1`
- `go test ./internal/monitoring -run 'TestBuildCephClusterModel|TestCountCephMonitorDaemons|TestCountCephManagerDaemons|TestPollCephCluster' -count=1`

## Outcome

The v6 candidate no longer depends on one narrow Proxmox Ceph monitor-count
field and should not regress the reported Ceph monitor count symptom when users
move from v5 to v6.
