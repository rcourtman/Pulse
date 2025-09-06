# VM Disk Usage Monitoring

Pulse can show actual disk usage for VMs (just like containers) when the QEMU Guest Agent is installed and configured properly.

## What You See

**Without QEMU Guest Agent:**
- VMs show allocated disk size only (e.g., 32GB allocated)
- No visibility into actual disk usage inside the VM

**With QEMU Guest Agent:**
- VMs show real disk usage like containers do (e.g., 5.2GB used of 32GB)
- Accurate threshold alerts based on actual usage
- Better capacity planning with real data

## Requirements

### 1. Install QEMU Guest Agent in Your VMs

**Linux VMs:**
```bash
# Debian/Ubuntu
apt-get install qemu-guest-agent
systemctl enable --now qemu-guest-agent

# RHEL/Rocky/AlmaLinux
yum install qemu-guest-agent
systemctl enable --now qemu-guest-agent

# Alpine
apk add qemu-guest-agent
rc-update add qemu-guest-agent
rc-service qemu-guest-agent start
```

**Windows VMs:**
- Download virtio-win guest tools from: https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/
- Install the guest tools package which includes the QEMU Guest Agent
- The service starts automatically after installation

### 2. Enable Guest Agent in VM Options

In Proxmox web UI:
1. Select your VM
2. Go to **Options** → **QEMU Guest Agent**
3. Check **Enabled**
4. Start/restart the VM

Or via CLI:
```bash
qm set <vmid> --agent enabled=1
```

### 3. Verify Guest Agent is Working

Check if the agent is responding:
```bash
qm agent <vmid> ping
```

Get filesystem info (what Pulse uses):
```bash
qm agent <vmid> get-fsinfo
```

### 4. Pulse Permissions

Pulse needs the right permissions to query the guest agent:

**Proxmox 8 and below:** Requires `VM.Monitor` permission
**Proxmox 9+:** Requires `VM.GuestAgent.Audit` permission

When you run the Pulse setup script, it automatically detects your Proxmox version and sets the correct permissions. If setting up manually:

```bash
# Proxmox 9+
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor
# PVEAuditor includes VM.GuestAgent.Audit in PVE 9+

# Proxmox 8 and below
pveum role add PulseMonitor -privs VM.Monitor
pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
```

## Troubleshooting

### Quick Diagnostic Tool

Pulse includes a diagnostic script that can identify why a VM isn't showing disk usage:

```bash
# Run on your Proxmox host
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/test-vm-disk.sh | bash

# Or if Pulse is installed locally:
/opt/pulse/scripts/test-vm-disk.sh
```

Enter the VM ID when prompted. The script will check:
- VM running status
- Guest agent configuration  
- Guest agent runtime status
- Filesystem information
- API permissions

### Guest Agent Not Responding

**Check if agent is running inside VM:**
```bash
# Linux
systemctl status qemu-guest-agent

# Windows
Get-Service QEMU-GA
```

**Check VM configuration:**
```bash
# Should show "agent: 1"
qm config <vmid> | grep agent
```

**Check agent communication:**
```bash
# Should return without error
qm agent <vmid> ping
```

### Disk Usage Not Showing

If the agent is working but Pulse still shows allocated size:

1. **Check Pulse permissions** - Ensure the Pulse user has VM.Monitor (PVE 8) or VM.GuestAgent.Audit (PVE 9+)
2. **Check agent version** - Older agents might not support filesystem info
3. **Windows VMs** - Ensure virtio-win drivers are up to date
4. **Check Pulse logs** - Look for "GetVMFSInfo" errors

### Network Filesystems

The agent reports all mounted filesystems. Pulse automatically filters out:
- Network mounts (NFS, CIFS, SMB)
- Special filesystems (proc, sys, tmpfs, etc.)
- Bind mounts and overlays

Only local disk usage is counted toward the VM's total.

## Best Practices

1. **Install guest agent in VM templates** - New VMs will have it ready
2. **Monitor agent status** - Set up alerts if critical VMs lose agent connectivity
3. **Keep agents updated** - Update guest agents when updating VM operating systems
4. **Test after VM migrations** - Verify agent still works after moving VMs between nodes

## Platform-Specific Notes

### Cloud-Init Images
Most cloud images include qemu-guest-agent pre-installed but may need to be enabled:
```bash
systemctl enable --now qemu-guest-agent
```

### Docker/Kubernetes VMs
Container workloads can show high disk usage due to container layers. Consider:
- Using separate disks for container storage
- Monitoring container disk usage separately
- Setting appropriate thresholds for container hosts

### Database VMs
Databases often pre-allocate space. The guest agent shows actual usage, which might be less than what the database reports internally.

## Benefits

With QEMU Guest Agent disk monitoring:
- **Accurate alerts** - Alert on real usage, not allocated space
- **Better planning** - See actual growth trends
- **Prevent surprises** - Know when VMs are actually running out of space
- **Optimize storage** - Identify over-provisioned VMs