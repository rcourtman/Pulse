# Recovery Contract (Pulse v6)

This document defines the provider-neutral Recovery contract used by Pulse v6 to represent backup, snapshot, and replication artifacts across platforms (Proxmox, TrueNAS, Kubernetes, etc.).

Recovery is intentionally not “a Proxmox backups page”. It is a unified model built around a small set of universal concepts (subjects, points, rollups), with provider-specific detail kept out of the core columns and shown in details drawers/modals.

## Concepts

### Subject (What Was Protected)

A subject is the thing being protected, for example:

- Proxmox VM/CT
- TrueNAS dataset (`tank/apps/postgres`)
- Kubernetes PVC (`monitoring/prometheus-pvc`)
- Kubernetes cluster backup (Velero backup of a cluster)

Subjects should be stable, and where possible should link to a unified resource via `subjectResourceId`.

### Recovery Point (An Artifact / Event)

A recovery point is a single concrete artifact or event, such as:

- A backup snapshot in PBS
- A local dump-style backup
- A ZFS snapshot
- A snapshot replication run
- A VolumeSnapshot
- A Velero backup

Points are what the Recovery “Events/Artifacts” table lists.

### Rollup (A Subject Summary)

A rollup groups points for a single subject (and often a specific method/target) to answer:

- Is this subject protected?
- What was the most recent point?
- Are there failures/warnings recently?

Rollups are what the Recovery “Protected” table lists.

## Normalized Fields

### Identity and Grouping

- `provider`: normalized provider id (example: `pve`, `pbs`, `truenas`, `k8s`)
- `kind`: normalized artifact kind (example: `backup`, `snapshot`)
- `mode`: normalized mode (example: `local`, `remote`)
- `subjectResourceId`: (optional) unified resource id for the subject (preferred when available)
- `subjectRef`: provider-specific but stable reference for the subject (example: dataset path, VMID, PVC UID)
- `rollupId`: stable grouping key for rollups; should be deterministic from provider + subject + kind + mode (+ repository/target when needed)
- `id`: stable point id; should be deterministic from provider + subject + time (+ provider’s native id when available)

### Time

- `startedAt`: RFC3339 timestamp (optional)
- `completedAt`: RFC3339 timestamp (preferred when the point is finished)

For “last activity” and sorting, prefer `completedAt` when present, otherwise fall back to `startedAt`.

### Outcome / State

- `outcome`: `success` | `warning` | `failed` | `running`
- `message`: short human-readable summary (optional; provider specific)

### Optional Capability Fields

These fields are present when the provider can supply them:

- `sizeBytes`
- `verified`: tri-state (`true` | `false` | `null/unknown`)
- `encrypted`: tri-state (`true` | `false` | `null/unknown`)
- `immutable`: tri-state (`true` | `false` | `null/unknown`)
- `repositoryRef` / `targetRef`: where the artifact lives (example: PBS datastore, replication target)

The UI should not assume these exist across all providers. The Recovery filters and columns should adapt based on capability flags (see `/api/recovery/facets`).

## UI Semantics (Why Two Tables Exist)

- **Protected**: rollups view. One row per protected subject (or per subject + method/target when required). Optimized for answering “what is protected and is it healthy?”.
- **Events/Artifacts**: points view. Many rows per subject. Optimized for answering “what happened and when?”.

The “universal columns” should remain domain-level:

- Time
- Subject
- Method (kind + mode)
- Repository/Target (when applicable)
- Outcome
- Size (optional)
- Verification (tri-state; optional)
- Source (provider)
- Details (provider-specific summary; full JSON in drawer when debug mode enabled)

Avoid provider-specific columns (for example VMID/namespace/datastore) as first-class columns. When those details are useful, render them inside Subject/Repository/Details or in the details drawer.

## API Notes

The primary API surface is:

- `GET /api/recovery/points`
- `GET /api/recovery/rollups`
- `GET /api/recovery/series`
- `GET /api/recovery/facets`

See `docs/API.md` for query params and endpoints.

