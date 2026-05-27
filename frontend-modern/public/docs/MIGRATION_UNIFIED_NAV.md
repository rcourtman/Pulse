# Migration Guide: Unified Navigation

> **This migration has been reverted as of `v6.0.0-rc.6`.** Pulse v6 ships
> with the platform-shaped top-level navigation existing v5 operators
> already know (Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Standalone,
> plus Alerts, Patrol, and Settings). The unified
> `/infrastructure` / `/workloads` / `/storage` / `/recovery` layout that
> briefly shipped across `rc.1` through `rc.5` is retired. The unified
> resource model and `/api/resources` contract remain on the backend.
>
> If you have automation, bookmarks, or runbooks targeting the unified
> routes, point them at the platform-shaped equivalents instead:
>
> - `/infrastructure?source=pmg` → `/proxmox/mail-gateway` (or the
>   relevant Proxmox sub-page)
> - `/workloads?type=k8s` → `/kubernetes`
> - `/storage` → the platform-shaped Storage views on each platform page
> - `/recovery` → the platform-shaped Recovery / Backups views on each
>   platform page (the underlying `/api/recovery/*` contract is unchanged)
>
> The keyboard shortcuts follow the platform-shaped shape:
>
> - `g p` Proxmox, `g d` Docker, `g k` Kubernetes, `g n` TrueNAS,
>   `g v` vSphere, `g s` Standalone
> - `g a` Alerts, `g r` Patrol, `g t` Settings, `?` shortcuts help
>
> The "Classic shortcuts" bar referenced in the historical content below
> is not part of the shipped product; ignore that aid.
>
> The rest of this document is preserved as historical context for the
> `rc.1` through `rc.5` line.

## Historical context (rc.1 through rc.5 only)

The content below describes the unified-navigation migration that
shipped across `rc.1` through `rc.5` and was reverted in `rc.6`. It is
kept as a historical record so links from earlier release notes still
resolve. None of it describes the shipped product as of `rc.6` or later.

### What Changed (historical)
- Navigation was organized by **task** (Infrastructure, Workloads, Storage, Recovery) instead of by platform.
- Legacy pages (Proxmox Overview, Hosts, Docker, Services, Kubernetes) were replaced by unified views.
- Global search and keyboard shortcuts make navigation faster across all resources.
- Kubernetes was split by intent:
  - **Infrastructure** showed Kubernetes clusters and nodes.
  - **Workloads** showed Kubernetes pods with the same filters/grouping as VMs and containers.

### Why This Change (historical)
- A unified resource model enables one inventory and one search across platforms.
- Filters, drawers, and workflows stay consistent, instead of being re-implemented per platform page.
- New integrations can be added without expanding the top-level navigation indefinitely.

### Legacy Aliases and Redirects (historical)
- Legacy aliases have been fully removed; update bookmarks and runbooks to canonical routes.
- Optional migration aid: enable the **Classic shortcuts** bar in the main navigation (Settings → System → General).
- Plan automation/bookmarks to use canonical routes now:
  - `/infrastructure?source=pmg`
  - `/workloads?type=k8s`

### Where Old Pages Moved (historical)

| Legacy Page | New Location (rc.1-rc.5) |
|------------|--------------|
| Proxmox Overview | `/infrastructure` |
| Hosts | `/infrastructure` |
| Docker | `/workloads` (containers) + `/infrastructure` (hosts) |
| Proxmox Storage | `/storage` |
| Proxmox Backups | `/recovery` |
| Proxmox Replication | `/recovery?view=events&mode=remote` |
| Proxmox Ceph | `/ceph` (summary also visible in Storage) |
| Proxmox Mail Gateway | `/infrastructure?source=pmg` |
| Services | `/infrastructure?source=pmg` |
| Kubernetes | `/workloads?type=k8s` |

### New Features to Know (historical)

#### Global Search
- Press `/` to focus search.
- Search by name, node, type, tags, or status.
- Results navigate directly to the relevant view.
- Use `Cmd/Ctrl+K` for the command palette.

#### Keyboard Shortcuts (historical, rc.1-rc.5 shape)
- `g i` → Infrastructure
- `g w` → Workloads
- `g s` → Storage
- `g b` → Recovery
- `g a` → Alerts
- `g t` → Settings
- `?` → Shortcut help

#### Debug Drawer (Optional)
- Enable with localStorage key `pulse_debug_mode` for raw JSON in resource drawer.

### Tips (historical)
- If you used Docker and Hosts pages before, start with **Infrastructure** (hosts) and **Workloads** (containers).
- If you used the Kubernetes page before, use **Infrastructure** for cluster/node health and **Workloads** for pod-level operations.
- The new pages support unified filters, tags, and search across all sources.
