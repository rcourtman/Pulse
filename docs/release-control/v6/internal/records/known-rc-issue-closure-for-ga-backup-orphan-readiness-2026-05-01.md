# Known RC Issue Closure For GA Backup Orphan Readiness Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The v5 maintenance delta audit found that `#1352` had not been carried into the
v6 backup-alert path. Pulse v5 learned not to run backup orphan detection before
Proxmox template inventory was ready, because backup polling can race ahead of
guest and template discovery during startup.

The v6 runtime no longer evaluates raw storage backup arrays directly; it
evaluates recovery rollups. That made the v5 patch non-cherry-pickable, but the
same failure mode still applied: an old PVE backup whose VMID was not yet in
the current guest/template inventory could be marked as an orphaned backup-age
alert.

## Disposition

The v6 candidate now carries an inventory-aware backup alert boundary:

- `internal/alerts/alerts.go` keeps the existing `CheckBackups` API for direct
  callers and adds `CheckBackupsWithInventory` for monitoring-owned runtime
  evaluation.
- PVE orphaned backup alerts now require per-instance, per-guest-type inventory
  readiness before unresolved PVE backup subjects can alert.
- Known Proxmox template VM/container subjects are carried as backup-valid
  subjects and skipped from orphaned backup-age alert creation.
- Monitoring records Proxmox template subjects from both the efficient
  `cluster/resources` poll and the traditional VM/container poll fallback, and
  passes that scoped inventory into backup alert evaluation.
- PBS/PMG rollup behavior remains unchanged, so external backup-only subjects
  can still alert when no matching local guest exists.

## Proof

- `go test ./internal/alerts -run 'TestCheckBackups(SkipsPVEOrphanUntilInventoryReady|CreatesPVEOrphanWhenInventoryReady|SkipsKnownPVETemplateBackupSubject|SkipsOrphanedWhenDisabled|HandlesPbsOnlyGuests|VMIDCollision)' -count=1`
- `go test ./internal/monitoring -run 'TestPVEBackupTemplateInventoryScopeFromClusterResources|TestBuildGuestLookupsFromReadState' -count=1`
- `go test ./internal/alerts -count=1`
- `go test ./internal/monitoring -count=1`

## Outcome

The v6 recovery-rollup alert path no longer knowingly regresses v5 `#1352`.
PVE backup orphan alerts wait for the owning Proxmox inventory signal, and
template backups do not become false orphaned backup alerts just because
templates are excluded from normal workload resources.
