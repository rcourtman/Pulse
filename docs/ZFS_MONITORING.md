# 💾 ZFS Pool Monitoring

Pulse automatically detects and monitors ZFS pools on your Proxmox nodes.

> **TrueNAS users:** TrueNAS ZFS pool monitoring is handled separately via the TrueNAS integration. See [CONFIGURATION.md](CONFIGURATION.md#truenas) for setup. This page covers Proxmox-native ZFS pools.

## 🚀 Features

*   **Auto-Detection**: No configuration needed.
*   **Health Status**: Tracks `ONLINE`, `DEGRADED`, and `FAULTED` states.
*   **Error Tracking**: Monitors read, write, and checksum errors.
*   **Alerts**: Notifies you of degraded pools or failing devices.

## ⚙️ Requirements

The Pulse user needs `Sys.Audit` permission on `/nodes/{node}/disks` (included in the standard Pulse role).

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
1.  Check permissions: `pveum user permissions pulse-monitor@pve`.
2.  Verify pools exist: `zpool list`.
3.  Check logs: `journalctl -u pulse -n 200 | grep -i zfs`.
