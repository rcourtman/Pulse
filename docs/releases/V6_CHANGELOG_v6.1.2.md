# Pulse v6.1.2

_This changelog describes stable `v6.1.2` compared with stable `v6.1.1`._

## Changed

- Proxmox and PBS monitoring now uses fresher ownership boundaries for cluster,
  guest, backup, disk, Ceph, and connection-health state.
- TrueNAS uses its supported JSON-RPC transport and retains authoritative pool,
  replication, disk-topology, and SMART evidence.
- Metrics persistence reduces SQLite write amplification, while polling cadence
  changes apply immediately to live monitors.
- Evidence-based pool-health alerts and canonical web-interface links are
  available across the unified resource surfaces.
- Patrol readiness checks are representative of the configured local-provider
  runtime.

## Fixed

- Unified Agent installs on constrained QNAP and Unraid roots preflight free
  space and keep agent/watchdog logging bounded (#1617).
- Saved backup and PBS polling intervals take effect without a service restart
  (#1619).
- Operator-state payloads keep capability lists iterable, and stopped
  containers retain intentionally-offline and never-auto-remediate controls
  (#1621, #1622).
- Proxmox Quincy and Squid Ceph schemas report correct monitor and manager
  counts (#1626).
- Scheduled multi-guest `vzdump` jobs contribute stable per-guest backup status.
- Proxmox membership, sampling, registration, disk identity, and workload
  refresh remain coherent across clusters.
- TrueNAS health, replication, boot-pool, threshold, and disk evidence no
  longer falls back to stale interpretations.
- Availability, discovery suppression, RBAC recovery, diagnostic redaction,
  and organization-switch refresh paths fail safely and consistently.

## Release Metadata

- Version: `v6.1.2`
- Previous stable: `v6.1.1`
- Rollback target: `v6.1.1`
- Rollback command: `./scripts/install.sh --version v6.1.1`
- Promotion path: stable patch hotfix from `main`, using the integrated
  exact-SHA candidate and definitive release verdict
- Windows signing decision: the release owner approved a `v6.1.2`-only
  unsigned-Windows exception while SignPath company verification is still
  processing; Windows may show **Unknown Publisher**, while exact-SHA,
  checksum, detached-signature, manifest, and published-digest controls remain
  mandatory
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is required
