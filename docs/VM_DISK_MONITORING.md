# VM Disk Usage Monitoring

Pulse can show actual disk usage for VMs (just like containers) when the QEMU Guest Agent is installed and configured properly.

## Quick Summary

**Without QEMU Guest Agent:**
- VMs show "-" for disk usage (no data available)
- Cannot monitor actual disk usage inside the VM

**With QEMU Guest Agent:**
- VMs show real disk usage like containers do (e.g., "5.2GB used of 32GB / 16%")
- Accurate threshold alerts based on actual usage
- Better capacity planning with real data

## How It Works

Proxmox doesn't track VM disk usage natively (unlike containers which share the host kernel). To get real disk usage from VMs:

1. Proxmox API returns `disk=0` and `maxdisk=<allocated_size>` (this is normal)
2. Pulse automatically queries the QEMU Guest Agent API to get filesystem info
3. Guest agent reports all mounted filesystems from inside the VM
4. Pulse aggregates the data (filtering out special filesystems) and displays it

**Important**: This works with both API tokens and password authentication. API tokens work fine for guest agent queries when permissions are set correctly.

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

**Proxmox VE 8 and below:**
- Requires `VM.Monitor` for guest agent access
- `Sys.Audit` adds Ceph/cluster metrics and is applied when available
- Pulse setup script creates a `PulseMonitor` role with these privileges automatically

**Proxmox VE 9+:**
- Requires `VM.GuestAgent.Audit` for guest agent access
- `Sys.Audit` remains recommended for Ceph/cluster metrics
- Pulse setup script applies both via the `PulseMonitor` role (even if `PVEAuditor` lacks them)

**Both API tokens and passwords work** - tokens do NOT have any limitation accessing guest agent data.

When you run the Pulse setup script, it automatically detects your Proxmox version and sets the correct permissions. If setting up manually:

```bash
# Shared read-only access
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor

# Extra privileges for guest metrics and Ceph
EXTRA_PRIVS=()

# Sys.Audit (Ceph, cluster status)
if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
  EXTRA_PRIVS+=(Sys.Audit)
else
  if pveum role add PulseTmpSysAudit -privs Sys.Audit 2>/dev/null; then
    EXTRA_PRIVS+=(Sys.Audit)
    pveum role delete PulseTmpSysAudit 2>/dev/null
  fi
fi

# VM guest agent / monitor privileges
VM_PRIV=""
if pveum role list 2>/dev/null | grep -q "VM.Monitor"; then
  VM_PRIV="VM.Monitor"
elif pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
  VM_PRIV="VM.GuestAgent.Audit"
else
  if pveum role add PulseTmpVMMonitor -privs VM.Monitor 2>/dev/null; then
    VM_PRIV="VM.Monitor"
    pveum role delete PulseTmpVMMonitor 2>/dev/null
  elif pveum role add PulseTmpGuestAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
    VM_PRIV="VM.GuestAgent.Audit"
    pveum role delete PulseTmpGuestAudit 2>/dev/null
  fi
fi

if [ -n "$VM_PRIV" ]; then
  EXTRA_PRIVS+=("$VM_PRIV")
fi

if [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then
  PRIV_STRING="${EXTRA_PRIVS[*]}"
  pveum role delete PulseMonitor 2>/dev/null
  pveum role add PulseMonitor -privs "$PRIV_STRING"
  pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
fi
```

## Troubleshooting

### Quick Diagnostic Tool

Pulse includes a diagnostic script that can identify why a VM isn't showing disk usage:

```bash
# Run on your Proxmox host (latest version from GitHub)
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/test-vm-disk.sh | bash

# Or use the bundled copy installed with Pulse
/opt/pulse/scripts/test-vm-disk.sh
```

Enter the VM ID when prompted. The script will check:
- VM running status
- Guest agent configuration
- Guest agent runtime status
- Filesystem information
- API permissions

### Understanding Disk Display States

**Shows percentage** (e.g., "45%")
- Everything working correctly
- Guest agent installed and accessible

**Shows "-" with hover tooltip**
- Hover to see the specific reason
- Common reasons:
  - "Guest agent not running" - Agent not installed or service not started
  - "Guest agent disabled" - Not enabled in VM config
  - "Permission denied" - Token/user lacks required permissions
  - "Agent timeout" - Agent installed but not responding
  - "No filesystems" - Agent returned no usable filesystem data

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

### Permission Denied Errors

If you see "permission denied" in Pulse logs when querying guest agent:

1. **Verify token/user permissions:**
   ```bash
   pveum user permissions pulse-monitor@pam
   ```

2. **For Proxmox 9+:** Ensure user has the `VM.GuestAgent.Audit` privilege (PulseMonitor role handles this)

3. **For Proxmox 8:** Ensure user has the `VM.Monitor` privilege (PulseMonitor role handles this)

4. **All versions:** Confirm `Sys.Audit` is present for Ceph metrics when applicable

5. **Re-run setup script** if you added the node before Pulse v4.7 (old scripts didn't add VM.Monitor/guest agent privileges)

### Disk Usage Still Not Showing

If the agent is working but Pulse still shows "-":

1. **Check Pulse logs** for specific error messages:
   ```bash
   # Docker
   docker logs pulse | grep -i "guest agent\|fsinfo"

   # Systemd
   journalctl -u pulse -f | grep -i "guest agent\|fsinfo"
   ```

2. **Test guest agent manually** from Proxmox host:
   ```bash
   qm agent <vmid> get-fsinfo
   ```
   If this works but Pulse doesn't show data, check Pulse permissions and logs

3. **Check agent version** - Older agents might not support filesystem info

4. **Windows VMs** - Ensure virtio-win drivers are up to date

### Network Filesystems

The agent reports all mounted filesystems. Pulse automatically filters out:
- Network mounts (NFS, CIFS, SMB)
- Special filesystems (proc, sys, tmpfs, devtmpfs, etc.)
- Special Windows partitions ("System Reserved")
- Bind mounts and overlays
- Read-only appliance or optical images (squashfs, erofs, iso9660, CDFS, UDF, cramfs, romfs, fuse.cdfs)

Only local disk usage is counted toward the VM's total.

## Best Practices

1. **Install guest agent in VM templates** - New VMs will have it ready
2. **Monitor agent status** - Set up alerts if critical VMs lose agent connectivity
3. **Keep agents updated** - Update guest agents when updating VM operating systems
4. **Test after VM migrations** - Verify agent still works after moving VMs between nodes
5. **Check logs regularly** - Monitor Pulse logs for guest agent errors

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
- **Consistent monitoring** - VMs and containers use the same metrics
