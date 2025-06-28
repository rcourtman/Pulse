# PBS Implementation Plan

## Overview
Based on the PBS Node Information Report findings, this document outlines the necessary changes to Pulse to handle PBS VMID collisions and node information extraction.

## Critical Issues to Address

### 1. Silent Data Loss from VMID Collisions
**Severity**: CRITICAL
**Current State**: PBS silently overwrites backups when VMIDs collide across nodes
**Impact**: Users lose access to previous backups without warning

### 2. Node Information Not Available for VM Disk Backups
**Severity**: HIGH
**Current State**: VM backups with disks don't store source node information
**Impact**: Cannot identify which node a VM backup came from

## Implementation Plan

### Phase 1: Detection & Warnings (PRIORITY)

#### 1.1 VMID Collision Detection
**Files to modify**: 
- `server/dataFetcher.js` - Add collision detection logic
- `server/pbsUtils.js` - Add helper functions to detect duplicates

**Implementation**:
```javascript
// In fetchPbsDatastoreSnapshots, detect VMID collisions
function detectVmidCollisions(snapshots) {
  const vmidMap = new Map();
  const collisions = [];
  
  snapshots.forEach(snap => {
    const key = `${snap['backup-type']}/${snap['backup-id']}`;
    if (!vmidMap.has(key)) {
      vmidMap.set(key, []);
    }
    vmidMap.get(key).push(snap);
  });
  
  // Find collisions (same VMID from different sources)
  vmidMap.forEach((snaps, vmid) => {
    if (snaps.length > 1) {
      // Check if they have different comments (indicating different nodes)
      const uniqueComments = new Set(snaps.map(s => s.comment));
      if (uniqueComments.size > 1) {
        collisions.push({
          vmid,
          count: snaps.length,
          sources: Array.from(uniqueComments)
        });
      }
    }
  });
  
  return collisions;
}
```

#### 1.2 UI Warnings
**Files to modify**:
- `src/public/js/ui/pbs.js` - Add warning display for collisions

**Implementation**:
- Add warning banner when collisions detected
- Show which VMIDs are affected
- Recommend namespace usage

### Phase 2: Node Information Extraction

#### 2.1 Add Node Info Extraction for CT Backups
**Files to modify**:
- `server/dataFetcher.js` - Add client.log.blob fetching

**Implementation**:
```javascript
async function extractNodeInfoFromBackup(client, backup) {
  if (backup['backup-type'] === 'ct') {
    try {
      const response = await client.get('/admin/datastore/download-decoded', {
        params: {
          'backup-type': backup['backup-type'],
          'backup-id': backup['backup-id'],
          'backup-time': backup['backup-time'],
          'file-name': 'client.log.blob'
        }
      });
      
      // Extract "Client name:" from log
      const match = response.data.match(/Client name:\s*(\S+)/);
      return match ? match[1] : null;
    } catch (error) {
      console.warn('Failed to extract node info:', error.message);
      return null;
    }
  }
  return null; // VMs with disks don't have node info
}
```

#### 2.2 Display Node Info in UI
**Files to modify**:
- `src/public/js/ui/pbs.js` - Show node info when available

### Phase 3: Namespace Guidance

#### 3.1 Configuration UI Enhancement
**Files to modify**:
- `src/public/js/ui/configuration.js` - Add namespace configuration section

**Features**:
- Add namespace field to PBS configuration
- Show warning if namespaces not configured for multi-node setups
- Link to documentation about namespace importance

#### 3.2 Documentation
**Files to create**:
- `docs/PBS_NAMESPACE_GUIDE.md` - User guide for namespace setup

### Phase 4: Enhanced Diagnostics

#### 4.1 PBS Health Check Enhancement
**Files to modify**:
- `server/dataFetcher.js` - Enhance verification diagnostics

**Add checks for**:
- VMID collision detection
- Namespace configuration status
- Multi-node backup detection

## Testing Requirements

1. **Unit Tests**:
   - Test collision detection logic
   - Test node info extraction
   - Test namespace discovery enhancements

2. **Integration Tests**:
   - Test with multiple nodes backing up same VMIDs
   - Test with and without namespaces
   - Test CT vs VM backup handling

3. **Manual Testing**:
   - Verify warnings appear correctly
   - Test node info display
   - Verify namespace configuration UI

## Migration Guide for Users

1. **For existing users without namespaces**:
   - Display migration warning
   - Provide step-by-step namespace setup guide
   - Explain data loss risks

2. **For new users**:
   - Enforce namespace configuration during setup
   - Provide best practices documentation

## Performance Considerations

- Node info extraction adds API calls - implement caching
- Collision detection runs on every fetch - optimize algorithm
- Consider batch fetching for client.log.blob

## Security Considerations

- Ensure PBS token has permissions for download-decoded endpoint
- Handle 403 errors gracefully
- Don't expose sensitive log information in UI

## Rollout Plan

1. **Phase 1**: Deploy collision detection and warnings (1 week)
2. **Phase 2**: Add node info extraction (1 week)
3. **Phase 3**: Namespace configuration UI (1 week)
4. **Phase 4**: Enhanced diagnostics (1 week)

## Success Metrics

- Zero silent data loss from VMID collisions
- 100% of CT backups show source node
- Clear user understanding of namespace importance
- Reduced support tickets about "missing" backups