# Backup Type Handling Documentation

## Overview
This document explains how Pulse handles guest type detection across different backup sources, particularly addressing the complexity of PBS backups that may not have direct guest information.

## The Problem
- PBS backups contain authoritative type information ('vm' or 'ct') in their data
- Guest data from Proxmox uses 'qemu' for VMs and 'lxc' for containers  
- VMID reuse across different nodes/clusters can cause incorrect type detection
- PBS backups might match the wrong guest when looking up by VMID alone

## The Solution: backupEntityType

### Single Source of Truth
We introduced `backupEntityType` as the definitive field for determining whether a backup is for a VM or Container:
- Values: 'vm' or 'ct' (following PBS convention)
- Set at the server level when processing backup data
- Used consistently across all UI components

### Type Detection Priority
1. **PBS Backups**: Use the `backup-type` field from PBS data (most authoritative)
2. **PVE/Snapshots**: Convert guest type ('qemu' → 'vm', 'lxc' → 'ct')
3. **Fallback**: Default to VM if type cannot be determined

### Implementation Details

#### Server-side (dataFetcher.js)
```javascript
// Determine the definitive backup entity type
let backupEntityType = null;
if (guest.pbsBackups.length > 0) {
    // Use the backup type from the first PBS backup
    backupEntityType = guest.pbsBackups[0].backupType; // 'vm' or 'ct'
} else {
    // Fallback to guest type conversion
    backupEntityType = guest.type === 'qemu' ? 'vm' : 'ct';
}
```

#### Client-side (calendar-heatmap.js)
```javascript
// Determine the definitive backup type
let backupEntityType = null;
if (isPBSBackup && dateData[dateKey].backupType) {
    // For PBS backups, use the backup-type from PBS data
    backupEntityType = dateData[dateKey].backupType; // 'vm' or 'ct'
} else if (guestInfo.type) {
    // For PVE/snapshots, convert guest type to backup type format
    backupEntityType = guestInfo.type === 'qemu' ? 'vm' : 'ct';
}
```

#### Display Logic (backup-detail-card.js)
```javascript
// Use backupEntityType for display
const displayType = backup.backupEntityType === 'ct' ? 'CT' : 
                   backup.backupEntityType === 'vm' ? 'VM' : 
                   'Unknown';
```

## Benefits
1. **Accuracy**: PBS backup types are always correct, even without guest matching
2. **Consistency**: One field used throughout the application
3. **Robustness**: Handles VMID reuse and missing guest data gracefully
4. **Simplicity**: Clear fallback chain for type detection

## Edge Cases Handled
- PBS backups without matching guests
- VMID reuse across different nodes
- Mixed backup types for the same VMID
- Missing or corrupted type data

## Testing Checklist
- [ ] PBS backups show correct VM/CT type
- [ ] PVE backups show correct VM/CT type  
- [ ] Snapshots show correct VM/CT type
- [ ] Filters correctly filter by VM/Container type
- [ ] Calendar displays correct types
- [ ] Detail card shows correct types
- [ ] Table view shows correct types