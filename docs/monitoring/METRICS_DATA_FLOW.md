# Metrics Data Flow (Bars, Trends and History)

A chart that is still visible does not tell you that new readings are arriving.
If a value stops updating or a chart is empty, identify the view before checking
polling or storage. Keep the same connection, resource and metric in each view.

| Where you see it | Where the readings come from |
| --- | --- |
| **Bars** and live status rows across platform tables | The latest resource state delivered to the page, not a query for stored points. When a provider or agent stops updating, cached rows across Proxmox (including Ceph and replication), Docker (including images, storage, networks, Compose, Swarm, and Active Alerts), TrueNAS (including protection and Health Alerts), Kubernetes (including inventory, record drawers, and Active Alerts), VMware (including Health Signals and Activity), and Physical Disks (Storage tables, **Physical Disks & SMART** cards, linked pool and guest physical disks, and disk drawers) show an amber **Stale data** cue (or amber **Stale** verdict for clear stale disks) and expanded-drawer notice while preserving worse Offline, Degraded, Replace Now, Needs Attention, or alert severity states. Stale physical disk temperatures and SSD life stay muted as last known without triggering live **Running Hot**, and temperature or SMART history charts defer until live reporting recovers. **Storage Pools** also switch clear stale pools to amber **Stale** (leaving **Healthy**, joining **Needs attention** and warning counts, and sorting ahead of fresh healthy pools), while preserving worse explicit pool states (Faulted, Offline, Critical, Failed, Unavailable, Degraded, Warning) and alert headlines with an inline amber **Stale data** cue on both desktop and phones while keeping retained usage bars visible. **Proxmox Backups** (`/proxmox/backups`) adds a page-level stale telemetry banner naming stale PVE or PBS sources, amber **Stale data** header cues on **Backup health** and **Backup jobs**, and scoped stale cues and drawer notices on affected **Coverage** rows, **By date** restore points, and **Backup servers** while ignoring optional stale host agents when provider polling itself is healthy. **Machines** (`/standalone/machines`) and **Standalone VMs** (`/standalone/vms`) surface amber **Stale data** (or `Stale · <age>`) cues and drawer notices when Pulse Agent telemetry stops updating while preserving offline, degraded, and alert states. **Workloads** (`/workloads`) routes stale running Proxmox, vSphere, Docker, and Kubernetes workloads out of **Running** into **Attention**, displays inline **Stale** cues in rows and node group headers while retaining last-known readings, and shows an amber **Readings are stale** notice in the guest drawer without flagging fresh hypervisor telemetry when only an optional linked agent is stale. Keep any stale, last-known, or unavailable indication with the value. |
| **Trends** in a workload or Proxmox node table | A separate server chart read. Workload summaries use `/api/charts/workloads` and node summaries use `/api/charts/infrastructure`. Some row previews read history for just that resource. The browser can retain an earlier result for the same selection while it refreshes. |
| **Overview** charts in a resource drawer (or **History** on older layouts) | A read of `/api/metrics-store/history` for the selected resource and range. Check the chart's source label and observation time. A current or recent-only fallback is not stored coverage, and an empty range is not measured zero. |

The summary reads use recent server samples when they cover the requested
window. They can supplement shallow coverage with stored readings and, for
supported resources, native provider history. There is no fixed two-hour
boundary between recent and stored samples. The workload and node summary
sparklines do not build a browser history by sampling the live dashboard.
The resource drawer has its own range and coverage. No paid plan is needed
for metric history. See [how to read a History panel](../METRICS_HISTORY.md#read-a-history-panel)
and [what retention keeps](../METRICS_HISTORY.md#retention-and-accessible-range).

## If the views disagree

- **Bars change but Trends or the drawer do not.** Start with the affected
  history view. Compare the selected range, the most recent observation time
  if shown, and whether the chart says it could not refresh. This is not by
  itself evidence that the provider or agent stopped reporting. For example,
  a newly changed node disk bar and an older stored disk point can both be
  correct for their own times.
- **Several views stop together.** A visible line or a working Pulse Health
  response cannot settle this. Compare the affected connection's
  [recorded poll results](../api/SCHEDULER_HEALTH.md#compare-two-poll-snapshots)
  across its normal interval. Check agent reports and alert evaluation
  separately. A poll for one connection or a point for one resource does not
  prove that the other paths advanced.

Use the readings already available. Do not restart Pulse, clear history or
change polling or retention just to make the views agree. If you report a
disagreement, keep the view, metric, range, last visible point time and any
source or error label together. Use consistent private aliases and don't
post raw API responses or network captures.
