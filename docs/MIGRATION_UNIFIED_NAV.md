# Migration Guide: Unified Navigation

This guide explains what changed in unified navigation and where legacy pages moved in v6.

## What Changed
- Navigation is now organized by **task** (Infrastructure, Workloads, Storage, Recovery) instead of by platform.
- Legacy pages (Proxmox Overview, Hosts, Docker, Services, Kubernetes) were replaced by unified views.
- Global search and keyboard shortcuts make navigation faster across all resources.
- Kubernetes is now split by intent:
  - **Infrastructure** shows Kubernetes clusters and nodes.
  - **Workloads** shows Kubernetes pods with the same filters/grouping as VMs and containers.

## Why This Change
- A unified resource model enables one inventory and one search across platforms.
- Filters, drawers, and workflows stay consistent, instead of being re-implemented per platform page.
- New integrations can be added without expanding the top-level navigation indefinitely.

## Legacy Aliases and Redirects
- Legacy aliases have been fully removed; update bookmarks and runbooks to canonical routes.
- Optional migration aid: enable the **Classic shortcuts** bar in the main navigation (Settings → System → General).
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
| Proxmox Backups | `/recovery` |
| Proxmox Replication | `/recovery?view=events&mode=remote` |
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
- `g b` → Recovery
- `g a` → Alerts
- `g t` → Settings
- `?` → Shortcut help

### Debug Drawer (Optional)
- Enable with localStorage key `pulse_debug_mode` for raw JSON in resource drawer.

## Tips
- If you used Docker and Hosts pages before, start with **Infrastructure** (hosts) and **Workloads** (containers).
- If you used the Kubernetes page before, use **Infrastructure** for cluster/node health and **Workloads** for pod-level operations.
- The new pages support unified filters, tags, and search across all sources.
