# 🛡️ Sensor Proxy Hardening

> **⚠️ Deprecated:** The sensor-proxy is deprecated in favor of the unified Pulse agent.
> For new installations, use `install.sh --enable-proxmox` instead.
> See [TEMPERATURE_MONITORING.md](/docs/security/TEMPERATURE_MONITORING.md).

The `pulse-sensor-proxy` runs on the host to securely collect temperatures, keeping SSH keys out of containers.

## 🏗️ Architecture
*   **Host**: Runs `pulse-sensor-proxy` (unprivileged user).
*   **Container**: Connects via Unix socket (`/run/pulse-sensor-proxy/pulse-sensor-proxy.sock`).
*   **Auth**: Uses `SO_PEERCRED` to verify container UID/PID.

## 🔒 Host Hardening

### Service Account
Runs as `pulse-sensor-proxy` (no shell, no home).
```bash
id pulse-sensor-proxy # uid=XXX(pulse-sensor-proxy)
```

### Systemd Security
The service unit uses:
*   `User=pulse-sensor-proxy`
*   `NoNewPrivileges=true`
*   `ProtectSystem=strict`
*   `PrivateTmp=true`

### File Permissions
| Path | Owner | Mode |
| :--- | :--- | :--- |
| `/var/lib/pulse-sensor-proxy/` | `pulse-sensor-proxy` | `0750` |
| `/var/lib/pulse-sensor-proxy/ssh/` | `pulse-sensor-proxy` | `0700` |
| `/run/pulse-sensor-proxy/` | `pulse-sensor-proxy` | `0775` |

## 📦 LXC Configuration
Required for the container to access the proxy socket.

**`/etc/pve/lxc/<VMID>.conf`**:
```ini
unprivileged: 1
lxc.apparmor.profile: generated
lxc.mount.entry: /run/pulse-sensor-proxy mnt/pulse-proxy none bind,create=dir 0 0
```

## 🔑 Key Management
SSH keys are restricted to `sensors -j` only.

**Rotation**:
```bash
/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh
```
*   **Dry Run**: Add `--dry-run`.
*   **Rollback**: Add `--rollback`.

## 🚨 Incident Response
If compromised:
1.  **Stop Proxy**: `systemctl stop pulse-sensor-proxy`.
2.  **Rotate Keys**: Remove old keys from nodes manually or use `pulse-sensor-proxy-rotate-keys.sh`.
3.  **Audit Logs**: Check `journalctl -u pulse-sensor-proxy`.
4.  **Reinstall**: Run `/opt/pulse/scripts/install-sensor-proxy.sh`.
