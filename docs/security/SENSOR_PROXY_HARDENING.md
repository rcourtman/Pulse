# 🛡️ Sensor Proxy Hardening

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> Use `pulse-agent --enable-proxmox` for temperature monitoring.
> This document is retained for existing installations during the migration window.

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
bash -c "$(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/pulse-proxy-rotate-keys.sh)"
```
*   **Dry Run**: Add `--dry-run`.
*   **Rollback**: Add `--rollback`.

## 🚨 Incident Response
If compromised:
1.  **Stop Proxy**: `systemctl stop pulse-sensor-proxy`.
2.  **Rotate Keys**: Remove old keys from nodes manually or use the rotation helper above.
3.  **Audit Logs**: Check `journalctl -u pulse-sensor-proxy`.
4.  **Reinstall**:
    ```bash
    curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | sudo bash
    ```
