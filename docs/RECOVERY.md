# Recovery

Pulse v6 includes a **provider-neutral recovery view** that aggregates backup, snapshot, and replication artifacts across all connected platforms into a single interface.

## Overview

Recovery answers two questions:

1. **"What is protected?"** → The **Protected Items** table shows a rollup per subject with its latest backup/snapshot status.
2. **"What happened?"** → The **Events** table shows individual recovery points (artifacts) with timestamps, outcomes, and sizes.

## Supported Providers

| Provider | Recovery Point Types |
|---|---|
| **Proxmox Backup Server (PBS)** | Full and incremental backups, sync jobs, verify tasks |
| **Proxmox VE (PVE)** | Local dump-style backups (`vzdump`) |
| **TrueNAS** | ZFS snapshots, replication tasks |
| **Kubernetes** | VolumeSnapshots, Velero backups (when available) |

## Concepts

### Subject (What Was Protected)

A subject is the thing being protected:

- A Proxmox VM or container
- A TrueNAS dataset (e.g., `tank/apps/postgres`)
- A Kubernetes PVC (e.g., `monitoring/prometheus-pvc`)

Subjects link to unified resources via `subjectResourceId` when possible.

### Recovery Point (An Artifact / Event)

A recovery point is a single concrete artifact:

- A PBS backup snapshot
- A local `vzdump` backup file
- A ZFS snapshot
- A replication run result

### Rollup (A Subject Summary)

A rollup groups recovery points for a subject to show:

- **Protection status** — is this subject actively protected?
- **Latest point** — when was the most recent successful backup/snapshot?
- **Health** — are there recent failures or warnings?

## Navigating Recovery

### Protected Items Tab

Shows one row per protected subject (or per subject + method when multiple backup methods exist). Key columns:

| Column | Description |
|---|---|
| Subject | The protected resource (VM name, dataset path, etc.) |
| Method | Backup method (PBS backup, local dump, ZFS snapshot, replication) |
| Last Point | Most recent recovery point timestamp |
| Outcome | Success / Warning / Failed |
| Source | Which provider created this point (pve, pbs, truenas, k8s) |

### Events Tab

Shows individual recovery points. Key columns:

| Column | Description |
|---|---|
| Time | When the point was created (started/completed) |
| Subject | What was backed up |
| Method | Kind + mode of the backup |
| Outcome | success / warning / failed / running |
| Size | Size of the artifact (when available) |
| Verified | Whether the backup has been verified (tri-state) |

### Filtering

Both tabs support:

- **Source filter** — show only points from a specific provider
- **Outcome filter** — show only failed, successful, or running points
- **Time range** — filter to a specific time window
- **Search** — full-text search across subjects and details

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/recovery/points` | List individual recovery points |
| `GET` | `/api/recovery/rollups` | List subject rollups (protected items) |
| `GET` | `/api/recovery/series` | Time-series data for recovery charts |
| `GET` | `/api/recovery/facets` | Available filter facets (providers, kinds, outcomes) |

### Query Parameters

All recovery endpoints support:

| Parameter | Description |
|---|---|
| `provider` | Filter by provider (`pve`, `pbs`, `truenas`, `k8s`) |
| `kind` | Filter by kind (`backup`, `snapshot`, `replication`) |
| `outcome` | Filter by outcome (`success`, `failed`, `warning`, `running`) |
| `since` | ISO 8601 timestamp — only points after this time |
| `until` | ISO 8601 timestamp — only points before this time |
| `subject` | Filter by subject reference |
| `limit` | Max results (default: 500) |

## Troubleshooting

### No recovery data showing

1. Verify at least one data source provides backup/snapshot data:
   - **PBS**: Ensure a PBS connection exists in Settings → Infrastructure.
   - **TrueNAS**: Ensure a TrueNAS connection exists in Settings → TrueNAS.
   - **PVE**: Local backups from PVE are included automatically.
2. Wait one polling cycle (~30 seconds) for data to appear.
3. Check the source filter — make sure you're not filtering to an empty source.

### PBS backups showing but not TrueNAS snapshots (or vice versa)

Check the **Source** filter on the Recovery page. Each provider surfaces its recovery points independently. Clear all filters to see everything.

### Recovery points showing as "failed"

Click the row to expand the details drawer, which shows the provider-specific error message. Common causes:

- **PBS**: Datastore unreachable, verification failed, prune job errors
- **TrueNAS**: Replication target unreachable, dataset locked, insufficient space
- **PVE**: Backup storage full, vzdump process error

## See Also

- [PBS Integration](PBS.md) — Proxmox Backup Server monitoring
- [TrueNAS Integration](TRUENAS.md) — TrueNAS snapshot and replication monitoring
- [Unified Resource Model](UNIFIED_RESOURCES.md) — how recovery integrates with the unified model
