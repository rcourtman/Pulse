# Migration Guide: Unified Navigation

This guide explains what changed in the unified navigation release and how to find the new locations for legacy pages.

## What Changed
- Navigation is now organized by **task** (Infrastructure, Workloads, Storage, Backups, Services) instead of by platform.
- Legacy pages (Proxmox Overview, Hosts, Docker) redirect to unified views.
- Global search and keyboard shortcuts make navigation faster across all resources.

## Where Old Pages Moved

| Legacy Page | New Location |
|------------|--------------|
| Proxmox Overview | `/infrastructure` |
| Hosts | `/infrastructure` |
| Docker | `/workloads` (containers) + `/infrastructure` (hosts) |
| Proxmox Storage | `/storage` |
| Proxmox Backups | `/backups` |
| Proxmox Replication | `/backups` (Replication tab) |
| Proxmox Mail Gateway | `/services` |

## New Features to Know

### Global Search
- Press `/` or `Cmd/Ctrl+K` to open search.
- Search by name, node, type, tags, or status.
- Results navigate directly to the relevant view.

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
- The new pages support unified filters, tags, and search across all sources.
