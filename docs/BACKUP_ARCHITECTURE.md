# Backup Data Architecture & Pragmatic Solution

## Overview

Pulse's backup monitoring system faces architectural challenges due to diverse Proxmox environments. This document outlines complexities and provides a pragmatic, minimal solution approach.

## Backup Types & Behaviors

### 1. PVE Local Backups (vzdump)
- **Location**: Local storage on individual PVE nodes
- **Scope**: Node-specific, not visible to other nodes
- **Identification**: Node + vmid + timestamp
- **Migration Impact**: Backup stays on original node even if VM migrates

### 2. PVE Shared Storage Backups (vzdump to NFS/Ceph)
- **Location**: Shared storage visible to multiple nodes
- **Scope**: Cluster-wide visibility
- **Identification**: Multiple nodes see same backup file
- **Migration Impact**: Backup visible regardless of current VM location
- **Duplication Risk**: Same backup appears once per node that mounts the storage

### 3. PBS Remote Backups
- **Location**: Centralized PBS server
- **Scope**: Cross-cluster, can backup from multiple PVE clusters
- **Identification**: PBS datastore + backup-id + timestamp
- **Migration Impact**: Independent of VM location
- **Centralized**: Single source of truth per PBS server

### 4. VM/Container Snapshots
- **Location**: VM storage location (can be local or shared)
- **Scope**: Follows VM storage
- **Identification**: Node + vmid + snapshot name + timestamp
- **Migration Impact**: Snapshots follow VM if on shared storage

## Problem Analysis: Real vs. Theoretical

### Critical Issues (Must Fix)
1. **Timestamp Precision Loss** - Core user issue causing wrong age display
   - **Impact**: High - incorrect data shown to users daily
   - **Fix Complexity**: Low - simple data structure enhancement

### Medium Issues (Monitor & Fix If Needed)
2. **Shared Storage Duplication** - Inflated backup counts
   - **Impact**: Medium - cosmetic but confusing
   - **Frequency**: Only affects shared storage users

3. **VM Migration Tracking** - Some backups may be missed
   - **Impact**: Medium - historical data gaps
   - **Frequency**: Only when VMs migrate AND users drill into history

### Low Priority Issues (Address If Encountered)
4. **Cross-cluster vmid conflicts** - Data mixing between unrelated VMs
   - **Impact**: Low - rare edge case
   - **Frequency**: Only with multiple clusters + same vmids

5. **Guest name conflicts** - Cosmetic confusion
   - **Impact**: Low - doesn't affect functionality

## Pragmatic Solution: Enhanced Current System

### Principle: Minimal Disruption, Maximum Benefit

Instead of complete architectural overhaul, enhance existing structure to solve core issues:

### Current Structure (Problematic)
```javascript
guest: {
  guestId: 111,
  guestName: "debian",
  latestBackupTime: 1749344442, // Could be ANY backup type - PROBLEM
  backupDates: [
    { 
      date: "2025-06-07",           // Date only - loses time precision - PROBLEM
      types: ["pbsSnapshots", "pveBackups"], 
      count: 3 
    }
  ]
}
```

### Enhanced Structure (Solution)
```javascript
guest: {
  guestId: 111,
  guestName: "debian",
  
  // Keep existing for compatibility
  latestBackupTime: 1749344442,   // Overall latest (any type)
  
  // NEW: Type-specific latest times - SOLVES CORE ISSUE
  latestTimes: {
    pve: 1749300000,      // Latest PVE backup timestamp (17h ago)
    pbs: 1749344442,      // Latest PBS backup timestamp (11h ago)  
    snapshots: 1749330000 // Latest snapshot timestamp
  },
  
  // Enhanced existing structure
  backupDates: [
    { 
      date: "2025-06-07", 
      types: ["pveBackups"], 
      count: 1,
      // NEW: Preserve actual timestamps for precision
      timestamps: {
        pveBackups: [1749300000],     // Actual 8pm timestamp
        pbsSnapshots: [],
        vmSnapshots: []
      }
    },
    {
      date: "2025-06-08",
      types: ["pbsSnapshots"],
      count: 1, 
      timestamps: {
        pveBackups: [],
        pbsSnapshots: [1749344442],   // Actual PBS timestamp
        vmSnapshots: []
      }
    }
  ]
}
```

### Simple Age Calculation Fix
```javascript
function getFilteredAge(guest, backupTypeFilter) {
  const now = Date.now();
  let latestTimestamp;
  
  // Direct timestamp lookup - fast and accurate
  switch(backupTypeFilter) {
    case 'pve': 
      latestTimestamp = guest.latestTimes?.pve;
      break;
    case 'pbs': 
      latestTimestamp = guest.latestTimes?.pbs;
      break;
    case 'snapshots': 
      latestTimestamp = guest.latestTimes?.snapshots;
      break;
    case 'all':
      latestTimestamp = guest.latestBackupTime; // Overall latest
      break;
  }
  
  // Fallback to overall if type-specific not available
  latestTimestamp = latestTimestamp || guest.latestBackupTime;
  
  return latestTimestamp 
    ? (now - latestTimestamp * 1000) / (1000 * 60 * 60 * 24)
    : Infinity;
}
```

## Implementation Plan

### Phase 1: Fix Core Issue (1-2 days)
1. **Server-side Enhancement**:
   ```javascript
   // During guest processing, calculate type-specific latest times
   guest.latestTimes = {
     pve: findLatestBackupByType(guestBackups, 'pve'),
     pbs: findLatestBackupByType(guestBackups, 'pbs'),
     snapshots: findLatestBackupByType(guestBackups, 'snapshots')
   };
   ```

2. **Client-side Fix**:
   ```javascript
   // Use type-specific timestamps in age calculations
   const age = getFilteredAge(guest, activeFilter);
   ```

### Phase 2: Enhanced Precision (Optional - 1 day)
- Add timestamp arrays to backupDates for calendar precision
- Enhance daily drill-down to show exact times

### Phase 3: Address Other Issues (As Needed)
- **Shared Storage Deduplication**: Only if users report confusion
- **Migration Tracking**: Only if users report missing backups  
- **Identity Conflicts**: Only if actually encountered in production

## Benefits of This Approach

### ✅ Solves Your Immediate Problem
- PVE filter shows 17h (correct) instead of 36h (wrong)
- Accurate age calculations for all backup types
- No precision loss from date aggregation

### ✅ Minimal Risk
- Extends existing structure instead of replacing
- Backward compatible
- Small, focused changes
- Easy to test and verify

### ✅ Performance Benefits  
- Direct timestamp lookups (O(1))
- No complex nested object traversal
- Minimal server-side processing overhead
- Fast client-side age calculations

### ✅ Maintainability
- Simple to understand and debug
- Follows existing patterns
- Clear separation of concerns without over-engineering
- Easy to extend if needed

### ✅ Future-Proof
- Foundation for addressing other issues incrementally
- Doesn't prevent future enhancements
- Data structure supports additional metadata

## Result: Your Specific Fix

**Before (Broken)**:
```
Filter: PVE → Shows 36h age (wrong - using midnight of backup date)
Filter: PBS → Shows 11h age  
Filter: All → Shows 11h age
```

**After (Fixed)**:
```  
Filter: PVE → Shows 17h age (correct - using actual 8pm backup timestamp)
Filter: PBS → Shows 11h age (correct)
Filter: All → Shows 11h age (correct - most recent overall)
```

## Conclusion

**This pragmatic approach delivers 80% of the benefits with 20% of the complexity.** It solves the core user-facing issue while maintaining system simplicity and keeping the door open for future enhancements if they become necessary.

The other theoretical issues (duplication, migration, conflicts) can be addressed incrementally if and when they actually impact users in practice.