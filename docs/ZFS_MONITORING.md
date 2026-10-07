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

Pool health and device status come from the Proxmox API and need `Sys.Audit`
on the affected node's disk scope. Use the **user, realm and token ID actually
configured for that Pulse connection**, not an example account. With privilege
separation enabled, effective access is the **intersection of user and token
permissions**; a working administrator session or a user-only permission
listing does not test Pulse's token.

If an existing API denial identifies missing access, have the Proxmox
administrator inspect both scoped ACLs and inherited permissions. Follow
[Check permissions](TROUBLESHOOTING.md#check-permissions-proxmox) for a necessary
access repair in a maintenance window. Do not grant a role across all nodes,
disable privilege separation, substitute an administrator token or rerun setup
just to diagnose absent ZFS data. A missing unregistered-pool row is not a
permission failure.

Dataset inventory additionally requires a Unified Agent on the Proxmox node
with read access to the local `zfs` command. If the command is unavailable,
Pulse falls back to the mounted ZFS datasets visible to the agent. Agent
capacity or dataset readings do not prove the API token can read pool health.

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

Start with the same **Proxmox installation, node, configured storage and
backing pool** in Pulse and Proxmox. Repeated storage or pool names on different
nodes are not one identity. Use the existing configuration and observations;
do not register a pool, rename storage or recreate a connection to make them
match.

- **No storage row:** check whether the pool backs configured Proxmox storage,
  using the existing **Datacenter → Storage** configuration and the affected
  node's storage view. An intentionally unregistered pool has no separate row;
  use [the capacity and separate health paths above](#unregistered-and-data-only-pools).
  A pool's existence alone does not establish that Pulse should list it.
- **Storage row present, pool health absent:** capacity/usage is a separate
  reading. Inspect the original collection error and time, if available.
  Only an actual access denial supports the [scoped permissions check above](#-requirements);
  a green connection badge or an administrator's successful native view does
  not establish that Pulse's token read the pool.
- **Pool health disagrees:** compare the same pool's native health and device
  errors with Pulse's reading and last successful collection time. A retained
  value is not proof of a fresh read; missing health is **unknown**, not
  `ONLINE`. Keep established native ZFS health checks meanwhile. Do not scrub,
  export/import, clear errors or replace a device just to investigate a display.

If server logs are needed, use the
[bounded Pulse log reader](TROUBLESHOOTING.md#inspect-notification-logs) for your
actual deployment and original time window. For a Pulse LXC, read inside that
container, not the Proxmox host; for Docker, use the container reader. Do not
pipe a journal read into `grep zfs`: a matching partial line can mask a failed
read, and an empty search does not prove collection succeeded. Do not restart
Pulse, enable Debug or run diagnostics merely to collect evidence.

For a report, retain the failing surface (row, capacity, pool health or dataset
inventory), time, collection path (API or agent), and relevant redacted error.
Use consistent aliases for private installation, node, storage, pool and device
names. Keep token secrets, full permission listings, raw logs and screenshots
with private paths or serial numbers out of the public thread. A working
capacity view does not resolve a pool-health failure.
