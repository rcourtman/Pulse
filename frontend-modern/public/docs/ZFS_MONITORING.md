# 💾 ZFS Pool Monitoring

Pulse adds ZFS pool health to **configured Proxmox storage** on your nodes.
The Proxmox → Storage list is not an inventory of every local ZFS pool.

> **TrueNAS users:** TrueNAS ZFS pool monitoring is handled separately via the TrueNAS integration. See [CONFIGURATION.md](CONFIGURATION.md#truenas) for setup. This page covers Proxmox-native ZFS pools.

## 🚀 Features

*   **Auto-Detection**: Matches configured storage to its backing ZFS pool, including directory storage on a ZFS dataset.
*   **Health Status**: Tracks `ONLINE`, `DEGRADED`, and `FAULTED` states.
*   **Error Tracking**: Monitors read, write, and checksum errors.
*   **Dataset Inventory**: With a Unified Agent on the node, expanded pool details list ZFS filesystems and zvols with used, available, referenced, and mountpoint information.
*   **Alerts**: Notifies you of degraded pools or failing devices.

## Unregistered and data-only pools

Pulse reads `/nodes/{node}/disks/zfs` and the pool-detail endpoint to enrich
Proxmox storage rows. A pool that is not configured in `/etc/pve/storage.cfg`
does not get its own row, even if it is `ONLINE` in the ZFS API, its physical
disks are visible, and the API account has the required permissions. A pool
used only through datasets and LXC bind mounts can therefore be absent from
Proxmox → Storage without a collection error.

Do not register a pool as Proxmox storage or change its datasets solely to make
it appear in Pulse. For a pool you intentionally keep unregistered:

* A **Unified Agent on the Proxmox host**, with a locally mounted ZFS dataset
  and access to `zpool`, can report capacity and usage in the node's
  **Overview details → Disks** panel, under the selected mount point. Without
  usable `zpool` output, the agent falls back to mounted-dataset counters;
  those are not necessarily whole-pool capacity.
* That capacity view and physical-disk SMART data do **not** provide
  `DEGRADED`/`FAULTED` pool-health alerts for an unregistered pool with no
  matching Proxmox storage row. Keep a separate ZFS health check for it.
* On the Proxmox host, these read-only commands show pool capacity, health and
  device errors. Replace `tank` with your pool name:

  ```bash
  zpool list -o name,size,alloc,free,health tank
  zpool status tank
  ```

`zpool list` reports raw pool capacity; it can differ from the agent's usable
capacity estimate on RAIDZ. These commands do not change the pool or register
it as Proxmox storage.

## ⚙️ Requirements

The Pulse user needs `Sys.Audit` permission on `/nodes/{node}/disks` (included in the standard Pulse role).

Pool health and device status come from the Proxmox API. Dataset inventory additionally requires a Unified Agent on the Proxmox node with read access to the local `zfs` command. If the command is unavailable, Pulse falls back to the mounted ZFS datasets visible to the agent.

```bash
# Grant permission manually if needed
pveum acl modify /nodes -user pulse-monitor@pve -role PVEAuditor
```

## 🔧 Configuration

ZFS monitoring is **enabled by default**. To disable it:

```bash
# Add to /etc/pulse/.env (systemd/LXC) or /data/.env (Docker/Kubernetes)
PULSE_DISABLE_ZFS_MONITORING=true
```

## 🚨 Alerts

| Severity | Condition |
| :--- | :--- |
| **Warning** | Pool `DEGRADED` or any read/write/checksum errors. |
| **Critical** | Pool `FAULTED` or `UNAVAIL`. |

## 🔍 Troubleshooting

**No ZFS Data?**
1.  Check whether the pool backs configured Proxmox storage: `pvesm status`.
    If it is intentionally unregistered, use the capacity and health paths above.
2.  Verify pools exist: `zpool list`.
3.  For a missing configured storage row, check permissions:
    `pveum user permissions pulse-monitor@pve`.
4.  Check logs: `journalctl -u pulse -n 200 | grep -i zfs`.
    A missing unregistered-pool row alone is not evidence that more API
    permissions are needed.
