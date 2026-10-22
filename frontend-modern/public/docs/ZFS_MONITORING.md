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
  **Overview** tab, in the **Storage** section of the technical details, which
  lists each mount point once the agent reports two or more. Without
  usable `zpool` output, the agent falls back to mounted-dataset counters;
  those are not necessarily whole-pool capacity.
* That capacity view and physical-disk SMART data do **not** provide
  `DEGRADED`/`FAULTED` pool-health alerts for an unregistered pool with no
  matching Proxmox storage row. Keep a separate ZFS health check for it.
* For capacity, health and device errors on a responsive Proxmox host, use the
  [bounded native status reader](#read-native-pool-status-safely) below. Prefer
  observations you already have; do not run it against an unresponsive host.

`zpool list` reports raw pool capacity; it can differ from the agent's usable
capacity estimate on RAIDZ. These commands do not change the pool or register
it as Proxmox storage.

## Read native pool status safely

Use this optional reader **on the affected Proxmox host**, not inside the Pulse
server container, and only with an account already authorised to read that
pool. It needs Bash, GNU `timeout`, `zpool` and the listed local utilities.
Replace `tank` with the existing pool name in the quoted assignment below;
do not add privileges or install tools just to complete a report. A read-only
command can still wait on storage. During a backup freeze/thaw or an
unresponsive-host incident, keep existing observations and follow
[Backup safety](VM_DISK_MONITORING.md#backup-safety), not a new status command.

```bash
# Proxmox host: one bounded, read-only ZFS status attempt
(
umask 077
for tool in timeout bash zpool head mktemp wc rm cat; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'Pool status unavailable: required tool %s is missing; no unbounded fallback.\n' "$tool" >&2
    exit 1
  }
done
if ! pulse_zfs_file=$(mktemp "${TMPDIR:-/tmp}/pulse-log.XXXXXX" 2>/dev/null); then
  printf 'Pool status unavailable: private temporary file could not be created.\n' >&2
  exit 1
fi
trap 'rm -- "$pulse_zfs_file"' EXIT
if timeout --signal=KILL 8s bash -o pipefail > "$pulse_zfs_file" 2>/dev/null <<'PULSE_ZFS_READ'
(
  set -e
  pool='tank'
  printf 'Pool capacity and health:\n'
  zpool list -o name,size,alloc,free,health "$pool"
  printf '\nPool device status:\n'
  zpool status "$pool"
) 2>&1 | head -c 262145
PULSE_ZFS_READ
then
  if ! pulse_zfs_bytes=$(wc -c < "$pulse_zfs_file" 2>/dev/null); then
    printf 'Pool status unavailable: result size could not be checked.\n' >&2
    exit 1
  fi
  if [ "$pulse_zfs_bytes" -gt 262144 ]; then
    printf 'Pool status unavailable: result exceeds 256 KiB; no partial result shown.\n' >&2
    exit 1
  fi
  cat -- "$pulse_zfs_file"
else
  pulse_zfs_status=$?
  printf 'Pool status unavailable (exit %s); no complete result. Stop collecting.\n' "$pulse_zfs_status" >&2
  exit "$pulse_zfs_status"
fi
)
```

The **eight-second deadline covers both commands and the byte limiter**, not
eight seconds for each command. At the deadline, GNU `timeout` sends KILL to
its collection group, including readers that ignore TERM; it does not stop
Pulse or change the pool. A kernel-blocked I/O operation may not exit promptly
even after that signal. If the attempt does not return, use the host's existing
recovery procedure; do not start another reader or assume the storage recovered.

Both commands must succeed and the combined result must fit within **256 KiB**
before any output is shown. A failed, oversized or timed-out read is
**unavailable**, not an empty pool or proof of health. The two reads are not an
atomic snapshot, and a successful `zpool` read does not establish Pulse API
collection, backup integrity or guest thaw. Keep separate ZFS health monitoring.

Output is **not sanitised**: it can contain pool names, device paths and serial
numbers. Review it locally; share only the relevant time, health/error counts
and manually redacted error with consistent private aliases, not the full
result. The owner-only temporary file is removed when the collection shell
exits; that cleanup does not erase terminal recordings or copies you retained.

## ⚙ Requirements

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
