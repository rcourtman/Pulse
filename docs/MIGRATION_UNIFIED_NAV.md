# Migration Guide: Unified Navigation

This guide explains what changed in the unified navigation release and how to find the new locations for legacy pages.

## What Changed
- Navigation is now organized by **task** (Infrastructure, Workloads, Storage, Backups) instead of by platform.
- Legacy pages (Proxmox Overview, Hosts, Docker, Services, Kubernetes) redirect to unified views.
- Global search and keyboard shortcuts make navigation faster across all resources.
- Kubernetes is now split by intent:
  - **Infrastructure** shows Kubernetes clusters and nodes.
  - **Workloads** shows Kubernetes pods with the same filters/grouping as VMs and containers.

## Deprecation Window
- Legacy redirects are transitional and not permanent URL contracts.
- `/services` and `/kubernetes` remain available during the migration window only.
- To sunset all legacy aliases immediately, set `PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS=true`
  (or set `disableLegacyRouteRedirects` in `system.json`).
- Plan automation/bookmarks to use canonical routes now:
  - `/infrastructure?source=pmg`
  - `/workloads?type=k8s`

## Where Old Pages Moved

| Legacy Page | New Location |
|------------|--------------|
| Proxmox Overview | `/infrastructure` |
| Hosts | `/infrastructure` |
| Docker | `/workloads` (containers) + `/infrastructure` (hosts) |
| Proxmox Storage | `/storage` |
| Proxmox Backups | `/backups` |
| Proxmox Replication | `/replication` (also accessible via Backups if enabled) |
| Proxmox Ceph | `/ceph` (summary also visible in Storage) |
| Proxmox Mail Gateway | `/infrastructure?source=pmg` |
| Services | `/infrastructure?source=pmg` |
| Kubernetes | `/workloads?type=k8s` |

## New Features to Know

### Global Search
- Press `/` to focus search.
- Search by name, node, type, tags, or status.
- Results navigate directly to the relevant view.
 - Use `Cmd/Ctrl+K` for the command palette.

### Keyboard Shortcuts
- `g i` → Infrastructure
- `g w` → Workloads
- `g s` → Storage
- `g b` → Backups
- `g a` → Alerts
- `g t` → Settings
- `?` → Shortcut help

### Debug Drawer (Optional)
- Enable with localStorage key `pulse_debug_mode` for raw JSON in resource drawer.

## Tips
- If you used Docker and Hosts pages before, start with **Infrastructure** (hosts) and **Workloads** (containers).
- If you used the Kubernetes page before, use **Infrastructure** for cluster/node health and **Workloads** for pod-level operations.
- The new pages support unified filters, tags, and search across all sources.
