# 💾 VM Disk Monitoring

Monitor actual disk usage inside your VMs using the QEMU Guest Agent.

## 🚀 Quick Start

1.  **Install Guest Agent**:
    *   **Linux**: `apt install qemu-guest-agent` (Debian/Ubuntu) or `yum install qemu-guest-agent` (RHEL).
    *   **Windows**: Install **virtio-win** drivers.
2.  **Enable in Proxmox**:
    *   VM Options → **QEMU Guest Agent** → Enabled.
    *   Restart the VM.
3.  **Verify**:
    *   Run `qm agent <vmid> ping` on the Proxmox host.
    *   Check Pulse dashboard for disk usage (e.g., "5.2GB used of 32GB").

## ⚙️ Requirements

*   **QEMU Guest Agent**: Must be installed and running inside the VM.
*   **Proxmox Permissions**: `VM.Monitor` (Proxmox 8) or `VM.GuestAgent.Audit` + `VM.GuestAgent.FileRead` (Proxmox 9+). Note: `PVEAuditor` is a built-in read-only role that cannot be modified — create a custom role instead.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **Disk shows "-"** | Hover over the dash for details. Common causes: Agent not running, disabled in config, or permission denied. |
| **Permission Denied** | Ensure your Proxmox token/user has `VM.GuestAgent.Audit` + `VM.GuestAgent.FileRead` (PVE 9+) or `VM.Monitor` (PVE 8). |
| **Rocky Linux / RHEL: memory or disk data missing** | The default qemu-guest-agent config may block file-read RPCs. Check `/etc/sysconfig/qemu-ga` and ensure `guest-file-open`, `guest-file-read`, and `guest-file-close` are not blocked, then restart the agent. See your distro's qemu-guest-agent docs for exact syntax. |
| **Agent Timeout** | Increase timeouts via env vars if network is slow: `GUEST_AGENT_FSINFO_TIMEOUT=10s`. |
| **Windows VMs** | Ensure the **QEMU Guest Agent** service is running in Windows Services. |

### Diagnostic Script
Run this on your Proxmox host to debug specific VMs:
```bash
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/test-vm-disk.sh | bash
```

## 📝 Notes

*   **Network Mounts**: NFS/SMB mounts are automatically excluded.
*   **Databases**: Usage reflects filesystem usage, which may differ from database-internal metrics.
*   **Containers**: LXC containers are monitored natively without the guest agent.
