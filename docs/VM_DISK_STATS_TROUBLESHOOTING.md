# VM Disk Stats Troubleshooting Guide

**Last Updated: September 1, 2025**  
**Tested on: Proxmox VE 9.0.6**  
**Status: Current Proxmox limitation (may be fixed in future versions)**

## Quick Summary

VM disk usage shows 0% or "-" in Pulse? This is a known Proxmox limitation, not a Pulse bug.

**The Problem**: On Proxmox 9, API tokens cannot access guest agent data (returns 401 Unauthorized), even with full permissions. This prevents Pulse from retrieving actual disk usage for VMs.

## Why VM Disk Stats Don't Work

### 1. How Proxmox Reports VM Disk Usage

Unlike containers (LXCs), Proxmox doesn't natively track VM disk usage. The standard API endpoints return:
- `disk: 0` - Always 0 for actual usage
- `maxdisk: <size>` - The allocated disk size

To get real disk usage, you must query the QEMU Guest Agent inside the VM.

### 2. The Proxmox 9 API Token Limitation

**Testing Results (September 1, 2025 on PVE 9.0.6)**:

| User | Auth Method | Guest Agent Access |
|------|-------------|-------------------|
| root@pam | API Token | ❌ 401 Unauthorized |
| root@pam | Password/Cookie | ✅ 200 OK |
| user@pam with PVEAuditor | API Token | ❌ 401 Unauthorized |
| user@pam with PVEAuditor | Password/Cookie | ✅ 200 OK |

**Key Finding**: API tokens cannot access `/nodes/{node}/qemu/{vmid}/agent/*` endpoints on Proxmox 9, regardless of user or permissions.

### 3. Version-Specific Behavior

#### Proxmox 8 and earlier
- API tokens work with `VM.Monitor` permission
- Guest agent data accessible via tokens

#### Proxmox 9+
- `VM.Monitor` permission removed
- Replaced with `VM.GuestAgent.Audit` (part of PVEAuditor role)
- **BUT**: API tokens still cannot access guest agent endpoints (Proxmox bug/limitation)

## What You'll See in Pulse

### Disk Display States

1. **Shows percentage** (e.g., "45%")
   - Everything working correctly
   - Guest agent installed and accessible

2. **Shows "-" with tooltip**
   - Hover to see specific reason:
   - "Permission denied. On Proxmox 9, API tokens cannot access guest agent data."
   - "Guest agent not running. Install qemu-guest-agent in the VM."
   - "Only special filesystems detected. Normal for Live ISOs."

3. **Shows 0%**
   - Old Pulse version or edge case
   - Usually means no guest agent

## Solutions

### Option 1: Use Password Authentication (Not Recommended)
Instead of API tokens, use username/password authentication. This works but is less secure and not recommended for production.

### Option 2: Accept the Limitation
- Container (LXC) disk stats work fine
- VM disk stats will show "-" with explanatory tooltip
- Wait for Proxmox to fix this upstream

### Option 3: Install Guest Agent (Partial Solution)
Even though API tokens can't access the data on PVE 9, installing guest agent helps on PVE 8 and prepares for when Proxmox fixes this:

#### Debian/Ubuntu VMs
```bash
apt update && apt install qemu-guest-agent
systemctl enable --now qemu-guest-agent
```

#### RHEL/Rocky/AlmaLinux VMs
```bash
yum install qemu-guest-agent
systemctl enable --now qemu-guest-agent
```

#### Alpine Linux VMs
```bash
apk add qemu-guest-agent
rc-update add qemu-guest-agent
rc-service qemu-guest-agent start
```

#### Windows VMs
Install VirtIO drivers which include the guest agent.

After installation, enable in Proxmox:
1. VM → Options → QEMU Guest Agent → Enable
2. Restart the VM

## Verification Commands

### Check if guest agent is accessible (run on Proxmox host)
```bash
# Check if agent is enabled in VM config
qm config <VMID> | grep agent

# Test agent directly (works as root on host)
qm agent <VMID> ping
qm agent <VMID> get-fsinfo

# Test via API with token (will fail on PVE 9)
curl -k -H "Authorization: PVEAPIToken=user@pam!token=<TOKEN>" \
  https://localhost:8006/api2/json/nodes/<NODE>/qemu/<VMID>/agent/get-fsinfo
```

## Related Issues

- **GitHub #348**: "After 4.7 Disk usage is at 0% (Proxmox 9 related)"
- **GitHub #367**: "Strange disk usage reporting on some VMs"
- **GitHub #71**: Initial report of VM disk showing 0%
- **Proxmox Bug #1373**: Feature request for native VM disk usage in Proxmox

## Technical Details

### Why This Happens

1. **Proxmox Design**: VMs are black boxes to the hypervisor. Unlike containers which share the host kernel, VMs run their own OS and Proxmox can't see inside without guest agent.

2. **Security Model Change**: Proxmox 9 tightened security around guest agent access, but went too far and blocked API tokens entirely.

3. **API Endpoints Affected**:
   - `/nodes/{node}/qemu/{vmid}/agent/get-fsinfo` - File system information
   - `/nodes/{node}/qemu/{vmid}/agent/*` - All guest agent endpoints

### What Pulse Does

1. Fetches VM data from `/nodes/{node}/qemu` or `/cluster/resources`
2. Sees `disk: 0` for VMs
3. Attempts to query guest agent at `/nodes/{node}/qemu/{vmid}/agent/get-fsinfo`
4. Gets 401 Unauthorized (on PVE 9 with tokens)
5. Sets disk usage to "-" with explanation in tooltip

### Special Cases

#### Live ISOs/Installation Media
VMs booted from ISOs show special filesystems only:
- `squashfs` - Compressed read-only filesystem
- `iso9660` - CD/DVD filesystem
- `tmpfs` - RAM-based temporary filesystem

These are filtered out as they don't represent actual disk usage.

#### Templates
VM templates always show 0% (they're not running).

#### Stopped VMs
Stopped VMs show 0% (guest agent not accessible when VM is off).

## Future Updates

This limitation is specific to Proxmox 9.0.x as of September 2025. Check back for updates:

- **Proxmox may fix this** in 9.1 or later versions
- **Pulse will automatically work** once Proxmox fixes the API
- **No Pulse update needed** - the fix needs to come from Proxmox

## Workaround for Critical Monitoring

If VM disk monitoring is critical for your environment:

1. Consider using a dedicated monitoring solution inside VMs (Prometheus node exporter, Telegraf, etc.)
2. Use Proxmox's built-in email alerts for disk space
3. Monitor at the storage level instead of per-VM

## Need Help?

- Check Pulse logs: `tail -f /var/log/pulse.log | grep -i "guest agent"`
- Verify permissions: `pveum user permissions <username>`
- Test manually: `qm agent <VMID> get-fsinfo` (as root on Proxmox host)

---

*This document describes the current state as of September 2025. The situation may improve in future Proxmox releases.*