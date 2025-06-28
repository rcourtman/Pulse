# PBS Integration Robustness Improvements for Pulse

## Current State
Pulse currently handles PBS data well but has limitations in node information extraction that could affect users with complex multi-node setups.

## Recommended Improvements

### 1. Implement Reliable Node Extraction for CT Backups

```javascript
// Add to pbsUtils.js or dataFetcher.js
async function extractNodeFromClientLog(pbsClient, snapshot, datastoreName) {
    if (snapshot['backup-type'] !== 'ct') {
        return null; // Only reliable for CTs
    }
    
    try {
        const response = await pbsClient.get(
            `/admin/datastore/${datastoreName}/download-decoded`,
            {
                params: {
                    'backup-type': snapshot['backup-type'],
                    'backup-id': snapshot['backup-id'],
                    'backup-time': snapshot['backup-time'],
                    'file-name': 'client.log.blob'
                }
            }
        );
        
        // Extract node name from log
        const match = response.data.match(/INFO: Client name: (.+)/);
        return match ? match[1].trim() : null;
    } catch (error) {
        console.warn('Failed to extract node from client log:', error);
        return null;
    }
}
```

### 2. Enhanced Collision Detection Messages

```javascript
// Improve user messaging based on setup
function getCollisionMessage(collisions, hasNamespaces) {
    if (!hasNamespaces) {
        return {
            severity: 'critical',
            title: '⚠️ Critical: VMID Collisions Causing Data Loss',
            message: 'Without namespaces, newer backups are overwriting older ones. Previous backups may be inaccessible!',
            action: 'Enable PBS namespaces immediately or use separate datastores per node.'
        };
    } else {
        return {
            severity: 'warning',
            title: '⚠️ Warning: VMID Collisions Detected',
            message: 'Namespaces are protecting your data, but identical VMIDs across nodes complicate management.',
            action: 'Consider implementing VMID range allocation per node.'
        };
    }
}
```

### 3. PBS Version Detection

```javascript
// Detect PBS version to provide appropriate guidance
async function getPBSVersion(pbsClient) {
    try {
        const response = await pbsClient.get('/version');
        const version = response.data.version;
        const major = parseInt(version.split('.')[0]);
        const minor = parseInt(version.split('.')[1]);
        
        return {
            version,
            supportsNamespaces: major > 2 || (major === 2 && minor >= 2),
            features: {
                namespaces: major > 2 || (major === 2 && minor >= 2),
                syncJobs: major >= 2,
                tapeBackup: major > 2 || (major === 2 && minor >= 1)
            }
        };
    } catch (error) {
        return { version: 'unknown', supportsNamespaces: false };
    }
}
```

### 4. Backup Ownership Tracking

```javascript
// Track backup ownership changes over time
function analyzeBackupOwnership(snapshots) {
    const ownershipChanges = [];
    const snapshotsByVmid = {};
    
    // Group by VMID
    snapshots.forEach(snap => {
        const key = `${snap['backup-type']}/${snap['backup-id']}`;
        if (!snapshotsByVmid[key]) {
            snapshotsByVmid[key] = [];
        }
        snapshotsByVmid[key].push(snap);
    });
    
    // Analyze ownership changes
    Object.entries(snapshotsByVmid).forEach(([vmid, snaps]) => {
        snaps.sort((a, b) => a['backup-time'] - b['backup-time']);
        
        let lastOwner = null;
        snaps.forEach(snap => {
            const currentOwner = snap.owner || 'unknown';
            if (lastOwner && lastOwner !== currentOwner) {
                ownershipChanges.push({
                    vmid,
                    timestamp: snap['backup-time'],
                    previousOwner: lastOwner,
                    newOwner: currentOwner,
                    impact: 'Previous backups may be inaccessible'
                });
            }
            lastOwner = currentOwner;
        });
    });
    
    return ownershipChanges;
}
```

### 5. Smart VMID Allocation Suggestions

```javascript
// Suggest VMID ranges based on current usage
function suggestVMIDRanges(nodes, existingVMIDs) {
    const suggestions = {};
    const rangeSize = 1000;
    let nextRangeStart = 100;
    
    nodes.forEach(node => {
        // Find unused range
        while (existingVMIDs.some(id => id >= nextRangeStart && id < nextRangeStart + rangeSize)) {
            nextRangeStart += rangeSize;
        }
        
        suggestions[node] = {
            start: nextRangeStart,
            end: nextRangeStart + rangeSize - 1,
            conflictingVMIDs: existingVMIDs.filter(id => 
                id >= nextRangeStart && id < nextRangeStart + rangeSize
            )
        };
        
        nextRangeStart += rangeSize;
    });
    
    return suggestions;
}
```

### 6. Disaster Recovery Helper

```javascript
// Help users identify which backups belong to which node
function generateDisasterRecoveryMap(pbsData) {
    const recoveryMap = {
        byNode: {},
        byVMID: {},
        ambiguous: []
    };
    
    pbsData.forEach(instance => {
        instance.datastores.forEach(ds => {
            ds.snapshots.forEach(snap => {
                const vmid = `${snap['backup-type']}/${snap['backup-id']}`;
                const nodeInfo = extractNodeFromComment(snap.comment) || 
                               extractNodeFromMetadata(snap) || 
                               'unknown';
                
                if (nodeInfo === 'unknown') {
                    recoveryMap.ambiguous.push({
                        vmid,
                        snapshot: snap['backup-time'],
                        namespace: snap.namespace || 'root'
                    });
                } else {
                    if (!recoveryMap.byNode[nodeInfo]) {
                        recoveryMap.byNode[nodeInfo] = [];
                    }
                    recoveryMap.byNode[nodeInfo].push(vmid);
                    
                    if (!recoveryMap.byVMID[vmid]) {
                        recoveryMap.byVMID[vmid] = [];
                    }
                    recoveryMap.byVMID[vmid].push(nodeInfo);
                }
            });
        });
    });
    
    return recoveryMap;
}
```

## Implementation Priority

1. **High Priority**: PBS version detection and appropriate warnings
2. **High Priority**: Enhanced collision severity detection based on namespace usage
3. **Medium Priority**: Client log parsing for CT node extraction
4. **Medium Priority**: Disaster recovery mapping
5. **Low Priority**: VMID range suggestions

## User Experience Improvements

1. **Clear Actionable Warnings**: Not just "16 collisions detected" but "16 VMID collisions detected. Your data is safe due to namespace usage, but consider VMID range allocation for easier management."

2. **Setup Wizard**: For new PBS connections, detect the setup and provide immediate recommendations

3. **Recovery Assistant**: "Looking for VM 101? It has backups from 3 different nodes: pi, delly, minipc. Check namespace 'root' for all versions."

## Testing Scenarios

Pulse should be tested with:
- PBS without namespaces (critical scenario)
- PBS with partial namespace adoption
- PBS with complete namespace separation
- Mixed PBS versions in multi-instance setups
- Deleted guests with remaining backups
- VMID range conflicts
- Ownership transfer scenarios