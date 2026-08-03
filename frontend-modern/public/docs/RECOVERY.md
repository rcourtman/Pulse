# Recovery

Pulse v6 includes a **provider-neutral recovery view** that aggregates backup, snapshot, and replication artifacts across all connected platforms into a single interface.

## Overview

Recovery is event-first and answers two questions:

1. **"What happened?"** → The **Recovery events** table shows individual recovery points (artifacts) with timestamps, outcomes, and sizes.
2. **"What can I actually recover?"** → **Protection coverage** shows the canonical posture for each resource: protected, attention, unprotected, or unknown.

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

### Protection Posture (A Trust Decision)

A protection posture combines subject-linked recovery points with the latest
provider collection evidence. It deliberately keeps four operator-facing
states:

- **Protected** — a qualifying current recovery point is linked to the resource
  and complete provider evidence does not invalidate the claim.
- **Attention** — evidence exists, but it is stale, failing, incomplete, or
  unverified when verification is expected.
- **Unprotected** — complete evidence confirms that no qualifying protection
  exists.
- **Unknown** — identity, permissions, provider history, or collection
  completeness cannot support a stronger claim.

A backup or snapshot artifact may still be shown while posture is unknown.
Artifacts answer what Pulse found; posture answers what Pulse can safely claim.
Snapshot presence alone is never presented as independent recovery.

## Navigating Recovery

### Recovery Events

Shows individual recovery points. Key columns:

| Column | Description |
|---|---|
| Time | When the point was created (started/completed) |
| Subject | What was backed up |
| Method | Kind + mode of the backup |
| Outcome | success / warning / failed / running |
| Size | Size of the artifact (when available) |
| Verified | Whether the backup has been verified (tri-state) |

### Protection Coverage

The Proxmox **Backups → Coverage** view shows one row per workload. The default
table stays compact; expanding a row reveals the plain-language posture reason,
provider evidence quality, and individual restore artifacts.

| Column | Description |
|---|---|
| Item | The protected resource (VM name, dataset path, etc.) |
| Item Type | Canonical resource category |
| Posture | Protected, attention, unprotected, or unknown |
| Restore | Most recent successful recovery point timestamp |
| Provider columns | Latest PBS, PVE, or guest-snapshot artifact where available |

### Filtering

Both workspaces support:

- **Platform filter** — show only points from a specific platform
- **Outcome filter** — show only failed, successful, or running points
- **Time range** — filter to a specific time window
- **Search** — full-text search across items and details

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/recovery/points` | List individual recovery points |
| `GET` | `/api/recovery/rollups` | List subject rollups (protection coverage) |
| `GET` | `/api/recovery/postures` | List canonical per-resource protection postures |
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

`/api/recovery/postures` has a deliberately bounded table contract. Supply one
or more repeated `resourceId` parameters for a resource or batch lookup (at
most 200), or omit them for a paged list. It also accepts `state`, `page`, and
`limit`; `state=attention` returns the actionable attention list. The response
includes the posture policy and provider evidence states so clients do not
re-derive trust from raw artifacts.

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

### Current backups show an unknown posture

Expand the workload row and inspect the limiting evidence. Pulse uses unknown
when the current poll cannot prove provider-history completeness, permission
scope, or subject identity. Fix the reported collection or access gap and wait
for the next provider poll; Pulse does not promote retained backup artifacts to
protected while that uncertainty remains.

## See Also

- [PBS Integration](PBS.md) — Proxmox Backup Server monitoring
- [TrueNAS Integration](TRUENAS.md) — TrueNAS snapshot and replication monitoring
- [Unified Resource Model](UNIFIED_RESOURCES.md) — how recovery integrates with the unified model
