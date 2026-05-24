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
*   **Proxmox Permissions**: `VM.GuestAgent.Audit` plus `VM.GuestAgent.FileRead` when available, with `VM.Monitor` only as a legacy fallback on older Proxmox 8 systems.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **Disk shows "-"** | Hover over the dash for details. Common causes: Agent not running, disabled in config, or permission denied. |
| **Permission Denied** | Ensure your Proxmox token/user has `VM.GuestAgent.Audit` plus `VM.GuestAgent.FileRead` where available, or `VM.Monitor` on older Proxmox 8 systems. |
| **Agent Timeout** | Increase the guest-agent filesystem timeout if network or guest responsiveness is slow: `GUEST_AGENT_FSINFO_TIMEOUT=30s`. |
| **Windows VMs** | Ensure the **QEMU Guest Agent** service is running in Windows Services. |

### Large Proxmox estates

If guest disk values only populate for the first part of a large VM list, tune the server poll budget as well as the guest-agent timeout:

- `GUEST_AGENT_FSINFO_TIMEOUT=30s`
- `GUEST_AGENT_VM_BUDGET=45s`
- `GUEST_AGENT_VM_MAX_CONCURRENT=8`
- `MAX_POLL_TIMEOUT=10m`

`GUEST_AGENT_FSINFO_TIMEOUT` controls each guest-agent filesystem call. `GUEST_AGENT_VM_BUDGET` is the per-VM guest-agent work budget. `GUEST_AGENT_VM_MAX_CONCURRENT` caps how many VM guest-agent jobs Pulse runs in parallel, and `MAX_POLL_TIMEOUT` gives large clusters enough total poll time to finish a full cycle.

### Diagnostic Script
Run this on your Proxmox host to debug specific VMs:
```bash
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/release/5.1/scripts/test-vm-disk.sh | bash
```

## 📝 Notes

*   **Network Mounts**: NFS/SMB mounts are automatically excluded.
*   **Databases**: Usage reflects filesystem usage, which may differ from database-internal metrics.
*   **Containers**: LXC containers are monitored natively without the guest agent.
