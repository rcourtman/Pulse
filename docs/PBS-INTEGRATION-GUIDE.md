# PBS Integration Guide for Pulse

This guide consolidates all PBS (Proxmox Backup Server) integration knowledge for implementing backup functionality in Pulse.

## Table of Contents
1. [Critical Issues & Solutions](#critical-issues--solutions)
2. [PBS API Integration](#pbs-api-integration)
3. [Implementation Recommendations](#implementation-recommendations)
4. [UI/UX Considerations](#uiux-considerations)
5. [Testing & Validation](#testing--validation)

## Critical Issues & Solutions

### 1. VMID Collisions & Data Loss

**Problem**: Without namespaces, PBS silently overwrites backups when VMs/containers share the same VMID across different nodes.

**Example**: 
- Node1 has VM 100 (webserver)
- Node2 has VM 100 (database)
- PBS will only keep the most recent backup, silently discarding the other

**Solutions**:
- **PBS 2.2+**: Use namespaces (mandatory for multi-node setups)
- **PBS <2.2**: Implement VMID range allocation per node
- Always show warnings when collisions are detected

### 2. Node Information Extraction

**Container Backups**: Node info reliably available
```javascript
// Extract from client.log.blob
const match = log.match(/starting backup on host (\S+)/);
const nodeName = match ? match[1] : null;
```

**VM Backups**: Node info NOT available in backup metadata
- Must rely on external tracking or namespace organization

### 3. PBS Node Discovery Workaround

PBS `/nodes` endpoint returns 403 for API tokens, but accepts any node name:
```javascript
// Use dummy node name to get task data
const response = await pbsClient.get('/nodes/localhost/tasks?limit=1');
// Extract real node name from task data
const nodeName = task.node || task.upid.split(':')[1];
```

## PBS API Integration

### Key Endpoints

```javascript
// Get all backups with namespace grouping
GET /api/pbs/backups

// Get specific PBS instance data
GET /api/pbs/backups/:instanceId

// Get backups for specific guest
GET /api/pbs/backups/guest/:type/:id

// Get VMID collision analysis
GET /api/pbs/collisions
```

### Response Structure

```javascript
{
  summary: {
    totalSnapshots: 150,
    totalSize: "500 GB",
    namespaces: ["root", "node1", "node2"],
    uniqueVMs: 25,
    uniqueCTs: 30
  },
  namespaces: {
    "root": {
      snapshots: 50,
      size: "150 GB",
      vms: { count: 10, recent: [...] },
      cts: { count: 15, recent: [...] }
    }
  },
  collisions: [
    {
      vmid: 100,
      type: "vm",
      locations: ["node1/root", "node2/root"],
      severity: "critical"
    }
  ]
}
```

## Implementation Recommendations

### 1. Namespace Support Detection

```javascript
async function checkNamespaceSupport(pbsClient) {
  try {
    const response = await pbsClient.get('/admin/datastore/${datastore}/namespace');
    return { supported: true, version: '2.2+' };
  } catch (error) {
    if (error.response?.status === 404) {
      return { supported: false, version: '<2.2' };
    }
    throw error;
  }
}
```

### 2. Collision Detection

```javascript
function detectCollisions(backups) {
  const vmidMap = new Map();
  
  backups.forEach(backup => {
    const key = `${backup.backup_type}-${backup.backup_id}`;
    if (!vmidMap.has(key)) {
      vmidMap.set(key, []);
    }
    vmidMap.get(key).push({
      namespace: backup.namespace || 'root',
      node: backup.node || 'unknown',
      lastBackup: backup.backup_time
    });
  });
  
  return Array.from(vmidMap.entries())
    .filter(([_, locations]) => locations.length > 1)
    .map(([key, locations]) => ({
      vmid: key.split('-')[1],
      type: key.split('-')[0],
      locations,
      severity: locations.some(l => l.namespace === 'root') ? 'critical' : 'warning'
    }));
}
```

### 3. Type Detection Priority

```javascript
function determineGuestType(backup, guestData) {
  // 1. Use PBS backup-type (most reliable)
  if (backup.backup_type === 'vm') return 'vm';
  if (backup.backup_type === 'ct') return 'ct';
  
  // 2. Check running guest data
  if (guestData?.type) return guestData.type.toLowerCase();
  
  // 3. Fallback heuristics
  if (backup.files?.includes('qemu-server.conf')) return 'vm';
  if (backup.files?.includes('lxc.conf')) return 'ct';
  
  return 'unknown';
}
```

## UI/UX Considerations

### 1. Collision Warnings

Display clear warnings for VMID collisions:
- **Critical** (red): Same VMID in root namespace on different nodes
- **Warning** (yellow): Same VMID in different namespaces
- **Info** (blue): Namespace usage recommendations

### 2. Namespace Grouping

Group backups by namespace in the UI:
```
PBS Backups Summary
├── root (50 backups, 150 GB)
│   ├── VMs: 10 (most recent: vm/100, vm/101)
│   └── CTs: 15 (most recent: ct/200, ct/201)
├── node1 (25 backups, 75 GB)
└── node2 (25 backups, 75 GB)
```

### 3. Guest-Centric View

Allow filtering by specific guest across all namespaces:
- Show all backups for a VMID
- Highlight which namespace/node each backup belongs to
- Warn if backups exist in multiple locations

## Testing & Validation

### 1. Quick Test Commands

```bash
# Check PBS summary data
curl -s http://localhost:7655/api/pbs/backups | jq '.summary'

# Check for collisions
curl -s http://localhost:7655/api/pbs/backups | jq '.collisions'

# Validate specific guest
curl -s http://localhost:7655/api/pbs/backups/guest/vm/100 | jq '.'
```

### 2. Automated Validation

```javascript
// Test script to validate UI displays match API data
async function validatePBSDisplay() {
  const apiData = await fetch('/api/pbs/backups').then(r => r.json());
  const uiElements = {
    totalSnapshots: document.querySelector('#pbs-total-snapshots').textContent,
    totalSize: document.querySelector('#pbs-total-size').textContent,
    // ... other UI elements
  };
  
  // Compare and report discrepancies
  console.assert(apiData.summary.totalSnapshots == uiElements.totalSnapshots);
}
```

### 3. Edge Case Testing

- VMID reuse across nodes
- Missing guest data
- Mixed backup types for same VMID
- Namespace migrations
- PBS version upgrades

## Key Takeaways

1. **Namespaces are critical** for multi-node PBS deployments (PBS 2.2+)
2. **VMID collisions cause silent data loss** without namespaces
3. **Node information** is available for containers but not VMs
4. **Type detection** should prioritize PBS metadata over PVE data
5. **UI must clearly show** collision warnings and namespace organization

## References

- PBS REST API Documentation
- Proxmox VE Admin Guide (Backup section)
- PBS 2.2 Release Notes (Namespace feature)