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
| `type` | `agent` (Proxmox node, host agent or other host-class parent; `docker-host` when it carries only Docker data), `vm`, `system-container`, `app-container`, `docker-service`, `storage`, `ceph`, `physical_disk`, `pbs`, `pmg`, `network-share`, `network-endpoint`, `pod`, `k8s-cluster`, `k8s-node`, `k8s-deployment`, plus further `docker-*` and `k8s-*` types |
| `status` | `online`, `warning`, `offline`, `unknown` (critical severity is reported separately as `critical` in the `health` object's verdict) |
| `sources` | Array of contributing data sources (e.g., `["proxmox", "agent"]`) |
| `sourceStatus` | Per-source health status |
| `metrics` | CPU, memory, disk, network (when available) |
| Platform extensions | `.kubernetes`, `.truenas`, `.docker`, etc. |

### Sources

A single resource can be reported by **multiple sources**. For example, a Proxmox node might have data from both the PVE API and a host agent:

```
sources: ["proxmox", "agent"]
sourceStatus:
  proxmox: { status: "online", lastSeen: "..." }
  agent: { status: "online", lastSeen: "..." }
```

The aggregate `status` is computed from all contributing sources.

### Data Sources

| Source | What it feeds |
|---|---|
| `proxmox` | Proxmox VE API — nodes, VMs, containers, storage |
| `pbs` | Proxmox Backup Server — datastores, backups, sync jobs |
| `pmg` | Proxmox Mail Gateway — mail stats, cluster health |
| `agent` | Unified agent — host metrics, temperatures, S.M.A.R.T. |
| `docker` | Docker/Podman — containers, images, networks |
| `kubernetes` | Kubernetes — clusters, nodes, pods, deployments |
| `truenas` | TrueNAS — system info, ZFS pools, datasets, snapshots, replication |

## Unified Navigation

The v6 UI keeps platform-shaped top-level navigation. The unified resource model sits behind it, but there are no separate Dashboard, Workloads, Storage or Recovery pages.

| Page | What it shows |
|---|---|
| **Proxmox**, **Docker**, **Kubernetes**, **TrueNAS**, **vSphere**, **Machines** | One page per platform, shown only when Pulse has resources for that platform |
| **Alerts** | Alerts across all platforms |
| **Patrol** | Patrol checks, shown only when AI is enabled |
| **Actions** | Approvals, ready work and recorded outcomes |
| **Settings** | Pulse configuration, shown only to users with settings access |

`/infrastructure` is not a page of its own. It redirects to the first available platform page, or to Settings (or Alerts when the user has no settings access) when no platform has resources.

### Unified Routes

The unified `/workloads`, `/storage` and `/recovery` routes that briefly shipped in earlier release candidates were reverted in `v6.0.0-rc.6` and do not exist. See [Migration Guide](MIGRATION_UNIFIED_NAV.md) for the platform-shaped equivalents.

## API

### Primary Endpoint

```
GET /api/resources
```

Returns all unified resources. Supports query parameters:

| Parameter | Description |
|---|---|
| `type` | Filter by resource type, comma-separated (`agent`, `vm`, `system-container`, `app-container`, `storage`, `pod`, etc.). Unknown tokens return HTTP 400. |
| `source` | Filter by data source (`proxmox`, `docker`, `kubernetes`, etc.) |
| `status` | Filter by status (`online`, `warning`, `offline`, `unknown`) |
| `q` | Case-insensitive substring match on resource name |
| `tags` | Filter by resource tag |

### Resource Details

Individual resource details are available via the unified state WebSocket connection, which pushes real-time updates to the frontend.

## Frontend Architecture

The frontend uses SolidJS reactive selectors to derive views from the unified store:

- `useResources()` — access the full unified resource list, plus derived views such as `infra()` (nodes, agents, docker-hosts), `workloads()` (VMs, containers, docker-containers), `byType()`, `filtered()` and `children()`

The hook hydrates from the WebSocket state when available and falls back to the REST API.

## See Also

- [Architecture](../ARCHITECTURE.md) — system architecture overview
- [Migration Guide](MIGRATION_UNIFIED_NAV.md) — upgrading from platform-specific navigation
- [API Reference](API.md) — full API documentation
- [TrueNAS Integration](TRUENAS.md) — TrueNAS-specific details
