# ZFS Pool Monitoring

Pulse v4.15.0+ includes automatic ZFS pool health monitoring for Proxmox VE nodes.

## Features

- **Automatic Detection**: Detects ZFS storage and monitors associated pools
- **Health Status**: Monitors pool state (ONLINE, DEGRADED, FAULTED)
- **Error Tracking**: Tracks read, write, and checksum errors
- **Device Monitoring**: Monitors individual devices within pools
- **Alert Generation**: Creates alerts for degraded pools and device errors
- **Frontend Display**: Shows ZFS issues inline with storage information

## Requirements

### Proxmox Permissions
The Pulse user needs `Sys.Audit` permission on `/nodes/{node}/disks` to access ZFS information:

```bash
# Grant permission for ZFS monitoring (already included in standard Pulse role)
pveum acl modify /nodes -user pulse-monitor@pam -role PVEAuditor
```

### API Endpoints Used
- `/nodes/{node}/disks/zfs` - Lists ZFS pools
- `/nodes/{node}/disks/zfs/{pool}` - Gets detailed pool status

## Configuration

ZFS monitoring is **enabled by default** in Pulse v4.15.0+.

### Disabling ZFS Monitoring
If you want to disable ZFS monitoring (e.g., for performance reasons):

```bash
# Add to /opt/pulse/.env or environment
PULSE_DISABLE_ZFS_MONITORING=true
```

## Alert Types

### Pool State Alerts
- **Warning**: Pool is DEGRADED
- **Critical**: Pool is FAULTED or UNAVAIL

### Error Alerts
- **Warning**: Any read/write/checksum errors detected
- Alerts include error counts and affected devices

### Device Alerts
- **Warning**: Device has errors but is ONLINE
- **Critical**: Device is FAULTED or UNAVAIL

## Frontend Display

ZFS issues appear in the Storage tab:
- Yellow warning bar for degraded pools
- Red error counts for devices with issues
- Detailed device status for troubleshooting

## Performance Impact

- Adds 2 API calls per node with ZFS storage
- Typically adds <1 second to polling cycle
- Only queries nodes that have ZFS storage

## Troubleshooting

### No ZFS Data Appearing
1. Check permissions: `pveum user permissions pulse-monitor@pam`
2. Verify ZFS pools exist: `zpool list`
3. Check logs: `grep ZFS /opt/pulse/pulse.log`

### Permission Denied Errors
Grant the required permission:
```bash
pveum acl modify /nodes -user pulse-monitor@pam -role PVEAuditor
```

### High API Load
Disable ZFS monitoring if not needed:
```bash
echo "PULSE_DISABLE_ZFS_MONITORING=true" >> /opt/pulse/.env
systemctl restart pulse-backend
```

## Example Alert

```
Alert: ZFS pool 'rpool' is DEGRADED
Node: pve1
Pool: rpool
State: DEGRADED
Errors: 12 read, 0 write, 3 checksum
Device sdb2: DEGRADED with 12 read errors
```

This helps administrators identify failing drives before complete failure occurs.