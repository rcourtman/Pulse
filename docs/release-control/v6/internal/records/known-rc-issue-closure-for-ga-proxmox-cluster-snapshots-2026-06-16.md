# Known RC Issue Closure For GA Proxmox Cluster Snapshots Record

- Date: `2026-06-16`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1437`
- Result: `fixed-local-proof`

## Context

Issue `#1437` remained open after the final v5 maintenance release because a
reporter still saw Proxmox guest snapshots on a standalone node but not on a
Proxmox cluster. The reporter plans to retest on the first stable v6 release
instead of exporting more v5 logs.

That makes the v6 cluster snapshot path part of the known-issue GA floor even
though the original v5 polling starvation fix was already addressed.

## Disposition

The v6 PVE backup/snapshot poller now keeps backup inventory reads on the
canonical `unifiedresources.ReadState` contract while refreshing the canonical
resource store from current monitor state when the store-backed read-state has
not yet observed the PVE instance's freshly collected guests.

This prevents the detached backup/snapshot poll from missing clustered guests
that were collected earlier in the same monitor cycle but had not yet reached
the store-backed read-state view.

## Proof

- `go test ./internal/monitoring -run 'TestMonitorPollGuestSnapshots|TestMonitor_PollGuestSnapshots|TestMonitor_PollPVEBackupsAndSnapshots|TestMonitorPollStorageBackupsWithNodes|TestMonitorCalculateBackupOperationTimeout'`

## Outcome

Clustered PVE guest snapshot polling no longer depends on a stale resource
store tick before it calls the Proxmox guest snapshot APIs. The reporter can
retest `#1437` against v6 stable without v6 knowingly carrying the cluster
snapshot discovery risk forward from v5.
