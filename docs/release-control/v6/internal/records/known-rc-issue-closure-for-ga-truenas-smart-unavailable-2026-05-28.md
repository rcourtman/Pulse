# Known RC Issue Closure For GA TrueNAS SMART Unavailable Record

- Date: `2026-05-28`
- Gate: `known-rc-issue-closure-for-ga`
- Lane: `L13`
- Issue: `https://github.com/rcourtman/Pulse/issues/1474`
- Result: `fixed-local-proof`

## Context

Issue `#1474` reports Pulse v6.0.0-rc.5 showing every TrueNAS passed-through
disk as `Replace Now - Disk reports health status FAILED` when TrueNAS cannot
read SMART data inside a Proxmox-hosted VM. The user-visible failure is not
that TrueNAS lacks SMART in this topology. The failure is that unavailable
SMART telemetry is projected as a replacement-required disk failure.

The canonical v6 fix belongs in TrueNAS ingestion and physical-disk projection,
not in the frontend. The frontend should keep treating real `FAILED` health or
critical disk-risk evidence as replacement-required.

## Disposition

The v6 TrueNAS disk path now separates native disk state from explicit SMART
health:

- REST `/disk` and RPC `disk.query` payloads parse `smart_status` into an
  explicit disk-health channel.
- `smart_status: null`, empty, unknown, unavailable, `N/A`, and equivalent
  unavailable values normalize to `UNKNOWN`.
- Explicit SMART failures still normalize to `FAILED` and produce a canonical
  disk-health risk.
- Native TrueNAS disk states such as `FAULTED`, `FAILED`, `OFFLINE`,
  `REMOVED`, and `UNAVAIL` still produce critical `truenas_disk_state` risk.
- Unknown or unavailable SMART telemetry no longer creates replacement-required
  physical-disk risk or incidents.

This keeps the root ownership in the provider and shared physical-disk contract
while preserving the frontend's existing replacement-warning semantics for real
failures.

## Proof

- `go test ./internal/truenas ./internal/unifiedresources ./internal/storagehealth`
- `npm --prefix frontend-modern test -- src/features/storageBackups/__tests__/diskPresentation.test.ts`

## Outcome

The v6 code path for the reported false replacement alert is fixed with local
automated proof. No public GitHub comment, issue retitle, label change, or issue
closure was made during this work. Public issue closure should wait for the
maintainer's normal issue hygiene or a current TrueNAS/proxmox topology retest.
