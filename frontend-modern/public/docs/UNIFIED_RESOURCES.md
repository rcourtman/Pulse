# Unified Resource Model

Pulse v6 introduces a **unified resource model** that normalizes all monitored infrastructure (Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, Docker, host agents, Kubernetes, TrueNAS, VMware vSphere, and Availability checks) into a single, consistent data structure.

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
| `health` | Backend-owned canonical health envelope with a `verdict` (including `stale` and `critical`) and stable `reasons` (`code` such as `telemetry_stale` plus optional detail) |
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

### Proxmox VE 9.3 persistent identity

On Proxmox VE 9.3 and later, Pulse reads persistent guest identity (`uid` and
`rtime`) and cluster identity (`cluster-id` from `GET /cluster/status`, for
both clustered and standalone hosts):

- **Upgrade continuity and live migration:** Existing guests keep their
  pre-9.3 canonical ID when they receive their first `uid`, and live migrations
  between nodes in the same cluster stay pinned to that same canonical ID.
- **VMID reuse isolation:** When a VMID slot on the same Proxmox instance is
  reused by a different guest with a new `uid`, Pulse mints a distinct
  UID-based canonical ID so overrides, metadata, alerts, and sibling-node
  History do not leak from the deleted guest.
- **Cluster identity guard:** Differing `cluster-id` values prevent independent
  clusters or standalone nodes that share a cluster or node name from
  auto-merging during membership reconciliation or unified node matching.

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
| `vmware` | VMware vSphere: vCenter and ESXi hosts, VMs, datastores, networks |
| `availability` | Availability checks: website, service, device, and TLS certificate checks |

## Unified Navigation

The v6 UI keeps platform-shaped top-level navigation. The unified resource model sits behind it, but there are no separate Dashboard, Workloads, Storage or Recovery pages.

| Page | What it shows |
|---|---|
| **Proxmox**, **Docker**, **Kubernetes**, **TrueNAS**, **vSphere**, **Machines** | One page per platform, shown only when Pulse has resources for that platform |
| **Availability** | Website, service, device, and TLS certificate checks (`/availability`) |
| **Alerts** | Alerts across all platforms |
| **Patrol** | Patrol checks, shown only when AI is enabled |
| **Actions** | Approvals, ready work and recorded outcomes |
| **Settings** | Pulse configuration, shown only to users with settings access |

`/infrastructure` is not a page of its own. It redirects to the first available platform page, or to Settings (or Alerts when the user has no settings access) when no platform has resources.

### Unified Routes

The unified `/workloads`, `/storage` and `/recovery` routes that briefly shipped in earlier release candidates were reverted in `v6.0.0-rc.6` and do not exist. See [Migration Guide](MIGRATION_UNIFIED_NAV.md) for the platform-shaped equivalents.

### Stale Pulse Agent telemetry on Machines

On **Machines** (`/standalone/machines`), Pulse keeps a host's last known metrics visible when its Pulse Agent stops reporting, but marks the readings clearly so stale hosts do not appear healthy:

- **Status and filter classification:** When backend `health.verdict` is `stale`, `telemetry_stale` is recorded, `sourceStatus.agent` is impaired, `agent.stale` is true, or `lastSeen` is older than 5 minutes, the host leaves **Healthy**, joins **Degraded**, and shows an amber **Stale** status indicator. Explicit **Offline** and awaiting-first-report states remain distinct.
- **Table banner and row cue:** A table-level warning banner lists affected hosts and ages (`Telemetry is stale for <host> (<age>). CPU, memory, disk, and sensor readings shown are last known values until their agents report again.`). Each affected row displays an always-visible compact amber cue beneath the machine name (`Stale · <age>` or `Stale data` on desktop, and `Stale` on narrow screens).
- **Expanded details, search, and fallbacks:** Expanding a stale machine displays an amber warning notice (`Agent has stopped reporting.`, the last report age, `Showing last known readings.`, and `Last error:` when recorded). Machine search matches stale labels, detail text, and recorded source errors. Retained RAID summaries are labelled `RAID Arrays (last known)`, and retained temperatures stay muted as last-known values.

### Stale workload telemetry

Workload tables and drawers (VMs, system containers, and container workloads across Proxmox, VMware vSphere, Docker, and Kubernetes) project canonical resource staleness and primary provider `sourceStatus` directly onto each workload row:

- **Primary provider scope:** Staleness tracks the workload's primary hypervisor or container provider (`Proxmox`, `vCenter`, `Docker`, or `Kubernetes`). On hybrid workloads (such as a Proxmox VM with an optional Pulse agent), fresh hypervisor readings stay unflagged when only the linked agent goes silent. Agent-sourced fields track agent staleness separately.
- **Filter routing and status priority:** Stale running and stopped workloads leave **Running** and route into **Attention** so stale workloads do not appear healthy, while explicit `offline`, `error`, and `failed` resource states stay ahead of staleness. If a running workload also has a failed attached availability probe, the header indicator keeps the probe warning alongside the stale label (`Running, not responding to probe · <stale label>`).
- **Parent node headers and workload rows:** Only a confirmed offline parent node marks its guests **Node offline** and dims them. A stale or degraded parent node displays an amber **Stale data** badge in the node group header (`Node telemetry is stale. Workload readings below reflect the last poll received.`) while keeping last-known guest rows readable. Affected workload rows show an inline amber cue (`Stale · <age>` when report age is available, or `Stale data`) with hover detail while preserving last-known metrics.
- **Search and workload drawer:** Workload search matches stale labels, stale detail text (`Showing the last values received.`), and recorded source error text. Expanding a stale workload aligns the drawer header status dot to amber and shows an inline **Readings are stale** warning notice with the provider stale detail. Both clear reactively as soon as live telemetry resumes.

## API

### Primary Endpoint

```
GET /api/resources
```

Returns all unified resources. Supports query parameters:

| Parameter | Description |
|---|---|
| `type` | Filter by resource type, comma-separated (`agent`, `vm`, `system-container`, `app-container`, `storage`, `pod`, etc.). Unknown tokens return HTTP 400. |
| `source` | Filter by data source, comma-separated (`proxmox`, `pbs`, `pmg`, `agent`, `docker`, `kubernetes`, `truenas`, `vmware`, `availability`) |
| `excludeSource` | Exclude one or more data sources, comma-separated |
| `status` | Filter by status (`online`, `warning`, `offline`, `unknown`) |
| `parent` | Filter by parent resource ID |
| `cluster` | Filter by cluster name |
| `namespace` | Filter by Kubernetes namespace |
| `q` | Case-insensitive substring match on resource name |
| `tags` | Filter by resource tag |
| `page` | 1-based page number (default `1`) |
| `limit` | Page size (default `50`, maximum `100`) |
| `sort` | Sort field: `name` (default), `status`, `type`, or `lastSeen` |
| `order` | Sort direction (`asc` or `desc`, default `asc`) |

### Resource Details

`GET /api/resources` prunes heavy platform-specific fields (such as PMG domain statistics) to keep list responses compact. Fetch the full payload for one resource with `GET /api/resources/{id}` (URL-encoding the canonical resource ID). Real-time updates are also pushed to the frontend over the unified state WebSocket connection.

## Frontend Architecture

The frontend uses SolidJS reactive selectors to derive views from the unified store:

- `useResources()` — access the full unified resource list, plus derived views such as `infra()` (nodes, agents, docker-hosts), `workloads()` (VMs, containers, docker-containers), `byType()`, `filtered()` and `children()`

The hook hydrates from the WebSocket state when available and falls back to the REST API.

## See Also

- [Architecture](../ARCHITECTURE.md) — system architecture overview
- [Migration Guide](MIGRATION_UNIFIED_NAV.md) — upgrading from platform-specific navigation
- [API Reference](API.md) — full API documentation
- [TrueNAS Integration](TRUENAS.md) — TrueNAS-specific details
