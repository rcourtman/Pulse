# PMG Backup Detection - Issue #359 Resolution

## Problem
PMG (Proxmox Mail Gateway) host configuration backups with VMID=0 were being incorrectly displayed as "LXC" instead of "Host" type.

## Root Causes Identified

1. **PBS stores PMG backups as 'ct' type** - PBS (Proxmox Backup Server) stores PMG backups with `backupType: "ct"` and `vmid: "0"` (string)
2. **Type checking inconsistency** - Some code paths weren't checking for VMID=0
3. **String vs Number VMID** - API returns VMID as string from PBS but as number from storage

## Solution Implemented

### 1. Robust VMID=0 Detection
```javascript
const isVmidZero = backup.vmid === '0' || backup.vmid === 0 || parseInt(String(backup.vmid)) === 0;
```

### 2. Multiple Detection Points
- PBS backups (`state.pbsBackups`)
- Storage backups (`state.pveBackups.storageBackups`)
- Both check VMID=0 BEFORE checking backup type

### 3. Debug Mode
Users can enable debug logging:
```javascript
localStorage.setItem('debug-pmg', 'true');
```
Then check console for `[PMG Debug]` messages.

## Testing

### Test Script
Created `/opt/pulse/test-pmg-backups.js` which tests:
- PBS backups with string VMID "0"
- PBS backups with numeric VMID 0
- Storage backups with type "host"
- Storage backups with type "lxc" but VMID=0
- All test cases pass ✓

### Test Results
```
✓ PBS PMG backup (ct type with VMID 0) → Host
✓ PBS PMG backup (ct type with numeric VMID 0) → Host
✓ Storage PMG backup (host type) → Host
✓ Storage PMG backup (lxc type with VMID 0) → Host
✓ Regular LXC backup → LXC
```

## How PMG Backups Are Stored

### In PBS
```json
{
  "backup-type": "ct",
  "backup-id": "0",
  "backup-time": 1234567890,
  "comment": "PMG host configuration backup"
}
```

### In PVE Storage
```json
{
  "type": "host",  // or sometimes "lxc"
  "vmid": 0,
  "volid": "local:backup/pmgbackup-...",
  "notes": "PMG host config backup"
}
```

## User Instructions for Debugging

If PMG backups still show as LXC:

1. **Enable debug mode**:
   ```javascript
   // In browser console
   localStorage.setItem('debug-pmg', 'true');
   location.reload();
   ```

2. **Check console** for `[PMG Debug]` messages

3. **Share the debug output** including:
   - The vmid value and type
   - The backupType or type field
   - The volid if available

4. **Disable debug mode**:
   ```javascript
   localStorage.removeItem('debug-pmg');
   ```

## Files Modified

1. `/opt/pulse/frontend-modern/src/components/Backups/UnifiedBackups.tsx`
   - Lines 203-228: PBS backup type detection
   - Lines 279-303: Storage backup type detection
   - Added debug logging

2. `/opt/pulse/internal/monitoring/monitor.go`
   - Lines 2591, 2603: Backend detection for "pmgbackup" and VMID=0

## Verification Checklist

- [x] PBS backups with VMID="0" show as Host
- [x] PBS backups with VMID=0 show as Host  
- [x] Storage backups with type="host" show as Host
- [x] Storage backups with VMID=0 show as Host (regardless of type)
- [x] Regular LXC backups (VMID≠0) show as LXC
- [x] Debug mode provides useful information
- [x] No regression for VM/LXC detection

## Known Edge Cases

1. **Old PMG versions** might not use VMID=0
2. **Custom backup names** without "pmgbackup" in volid
3. **PBS namespace** differences

## Monitoring

To verify in production:
1. Check for backups with VMID=0 in the UI
2. Verify they show "Host" badge (orange color)
3. Enable debug mode if issues persist