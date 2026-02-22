# Unified Resource Model

Pulse v6 introduces a **unified resource model** that normalizes all monitored infrastructure — Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, Docker, host agents, Kubernetes, and TrueNAS — into a single, consistent data structure.

## Why Unified Resources?

In earlier versions, each platform had its own data model, API endpoints, and frontend pages. This created:

- Duplicate UI code for each platform
- Inconsistent filtering and search
- No cross-platform comparison
- Separate alert logic per platform

The unified model eliminates this by representing **every resource** as a single `Resource` struct with a common set of fields plus optional platform-specific extensions.

## Core Concepts

### Resource

Every monitored entity is a `Resource` with:

| Field | Description |
|---|---|
| `id` | Globally unique identifier |
| `name` | Display name |
| `type` | `host`, `vm`, `container`, `storage`, `pool`, `dataset`, `disk`, `service`, `cluster`, `pod`, `deployment` |
| `status` | `online`, `warning`, `critical`, `offline`, `unknown` |
| `sources` | Array of contributing data sources (e.g., `["pve", "agent"]`) |
| `sourceStatus` | Per-source health status |
| `metrics` | CPU, memory, disk, network (when available) |
| Platform extensions | `.kubernetes`, `.truenas`, `.docker`, etc. |

### Sources

A single resource can be reported by **multiple sources**. For example, a Proxmox node might have data from both the PVE API and a host agent:

```
sources: ["pve", "agent"]
sourceStatus:
  pve: { status: "online", lastSeen: "..." }
  agent: { status: "online", lastSeen: "..." }
```

The aggregate `status` is computed from all contributing sources.

### Data Sources

| Source | What it feeds |
|---|---|
| `pve` | Proxmox VE API — nodes, VMs, containers, storage |
| `pbs` | Proxmox Backup Server — datastores, backups, sync jobs |
| `pmg` | Proxmox Mail Gateway — mail stats, cluster health |
| `agent` | Unified agent — host metrics, temperatures, S.M.A.R.T. |
| `docker` | Docker/Podman — containers, images, networks |
| `kubernetes` | Kubernetes — clusters, nodes, pods, deployments |
| `truenas` | TrueNAS — system info, ZFS pools, datasets, snapshots, replication |

## Unified Navigation

The v6 UI organises pages by **task** instead of **platform**:

| Page | What it shows |
|---|---|
| **Dashboard** | Overview panels aggregating all sources |
| **Infrastructure** | All hosts: Proxmox nodes, Docker hosts, K8s nodes, TrueNAS systems, agent-only hosts |
| **Workloads** | All workloads: VMs, LXC containers, Docker containers, Kubernetes pods |
| **Storage** | All storage: Proxmox storage, PBS datastores, ZFS pools/datasets, Ceph |
| **Recovery** | All backup/snapshot artifacts: PBS backups, PVE local dumps, ZFS snapshots, replication |
| **Alerts** | Unified alert view across all platforms |

Every page supports **source filtering** — click a source badge to see only resources from that platform.

### Legacy Route Compatibility

Legacy URLs redirect automatically with toast notifications:

| Legacy Route | Redirects To |
|---|---|
| `/proxmox/overview` | `/infrastructure` |
| `/hosts` | `/infrastructure?source=agent` |
| `/docker` | `/workloads?source=docker` |
| `/kubernetes` | `/infrastructure?source=kubernetes` |
| `/services`, `/mail` | `/infrastructure?source=pmg` |

See [Migration Guide](MIGRATION_UNIFIED_NAV.md) for the full mapping.

## API

### Primary Endpoint

```
GET /api/resources
```

Returns all unified resources. Supports query parameters:

| Parameter | Description |
|---|---|
| `type` | Filter by resource type (`host`, `vm`, `container`, etc.) |
| `source` | Filter by data source (`pve`, `docker`, `kubernetes`, etc.) |
| `status` | Filter by status (`online`, `warning`, `critical`, `offline`) |
| `search` | Full-text search across name, ID, tags |

### Legacy Endpoint (Deprecated)

```
GET /api/v2/resources
```

Still available for backward compatibility but **deprecated**. Use `/api/resources` for new integrations.

### Resource Details

Individual resource details are available via the unified state WebSocket connection, which pushes real-time updates to the frontend.

## Frontend Architecture

The frontend uses SolidJS reactive selectors to derive views from the unified store:

- `useResources()` — access the full unified resource list
- `useInfrastructureResources()` — hosts filtered for the Infrastructure page
- `useWorkloadResources()` — VMs/containers/pods for the Workloads page
- `useStorageResources()` — storage pools/datasets for the Storage page
- `useRecoveryResources()` — backup/snapshot data for the Recovery page

These selectors read from a single SolidJS store that is updated in real-time via WebSocket.

## See Also

- [Architecture](../ARCHITECTURE.md) — system architecture overview
- [Migration Guide](MIGRATION_UNIFIED_NAV.md) — upgrading from platform-specific navigation
- [API Reference](API.md) — full API documentation
- [TrueNAS Integration](TRUENAS.md) — TrueNAS-specific details
