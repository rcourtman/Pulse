# PBS Testing Utilities & Scripts

This document consolidates all PBS testing scripts and validation utilities for the backup system implementation.

## PBS API Test Script

```javascript
// server/pbsBackupsApi.js - PBS Backups REST API
const express = require('express');
const router = express.Router();

// Main endpoint: GET /api/pbs/backups
router.get('/backups', async (req, res) => {
    try {
        const pbsDataArray = req.app.locals.stateManager?.getState('pbsDataArray') || [];
        const processedData = processPBSBackups(pbsDataArray);
        res.json(processedData);
    } catch (error) {
        console.error('[PBS Backups API] Error:', error);
        res.status(500).json({ error: 'Failed to process PBS backup data' });
    }
});

// Get backups for specific PBS instance
router.get('/backups/:instanceId', async (req, res) => {
    const { instanceId } = req.params;
    // Implementation here
});

// Get backups for specific guest
router.get('/backups/guest/:type/:id', async (req, res) => {
    const { type, id } = req.params;
    // Implementation here
});
```

## Collision Detection Test

```javascript
#!/usr/bin/env node
// test-collision-detection.js

const testData = {
    pbsSnapshots: [
        { "backup-id": "100", "backup-type": "vm", namespace: "root", "backup-time": 1234567890 },
        { "backup-id": "100", "backup-type": "vm", namespace: "node2", "backup-time": 1234567891 },
        { "backup-id": "200", "backup-type": "ct", namespace: "root", "backup-time": 1234567892 }
    ]
};

function detectVMIDCollisions(snapshots) {
    const vmidMap = new Map();
    
    snapshots.forEach(snapshot => {
        const key = `${snapshot['backup-type']}-${snapshot['backup-id']}`;
        if (!vmidMap.has(key)) {
            vmidMap.set(key, []);
        }
        vmidMap.get(key).push({
            namespace: snapshot.namespace || 'root',
            backupTime: snapshot['backup-time']
        });
    });
    
    const collisions = [];
    vmidMap.forEach((locations, key) => {
        if (locations.length > 1) {
            const [type, vmid] = key.split('-');
            collisions.push({
                vmid,
                type,
                locations: locations.map(l => l.namespace),
                count: locations.length
            });
        }
    });
    
    return collisions;
}

console.log('Detected collisions:', detectVMIDCollisions(testData.pbsSnapshots));
```

## PBS Node Extraction Tests

```bash
#!/bin/bash
# test-pbs-node-extraction.sh

echo "Testing PBS node information extraction..."

# Test 1: Container backup log parsing
echo -e "\n1. Testing container backup log:"
cat > /tmp/test-client.log << 'EOF'
starting backup on host proxmox1
backup mode: snapshot
backup type: lxc
EOF

if grep -o "starting backup on host \S\+" /tmp/test-client.log | awk '{print $5}'; then
    echo "✓ Successfully extracted node name from container backup"
else
    echo "✗ Failed to extract node name"
fi

# Test 2: Task UPID parsing
echo -e "\n2. Testing UPID parsing:"
UPID="UPID:proxmox2:00001234:00ABCDEF:5F123456:backup:101:root@pam:"
NODE=$(echo $UPID | cut -d: -f2)
echo "Extracted node from UPID: $NODE"

# Test 3: PBS API node discovery workaround
echo -e "\n3. Testing PBS node discovery workaround:"
echo "curl -s https://pbs.example.com:8007/api2/json/nodes/localhost/tasks?limit=1"
echo "Expected: 200 OK with task data containing real node name"
```

## PBS UI Validation Script

```bash
#!/bin/bash
# validate-pbs-ui-api.sh

API_URL="http://localhost:7655/api/pbs/backups"

echo "PBS UI/API Validation Test"
echo "========================="

# Fetch API data
API_DATA=$(curl -s $API_URL)

# Extract summary values
TOTAL_SNAPSHOTS=$(echo $API_DATA | jq -r '.summary.totalSnapshots')
TOTAL_SIZE=$(echo $API_DATA | jq -r '.summary.totalSize')
NAMESPACES=$(echo $API_DATA | jq -r '.summary.namespaces | length')

echo "API Summary Data:"
echo "- Total Snapshots: $TOTAL_SNAPSHOTS"
echo "- Total Size: $TOTAL_SIZE"
echo "- Namespaces: $NAMESPACES"

# Check for collisions
COLLISIONS=$(echo $API_DATA | jq '.collisions | length')
if [ "$COLLISIONS" -gt 0 ]; then
    echo -e "\n⚠️  VMID Collisions Detected: $COLLISIONS"
    echo $API_DATA | jq '.collisions'
fi

# Validate namespace grouping
echo -e "\nNamespace Summary:"
echo $API_DATA | jq -r '.namespaces | to_entries[] | "- \(.key): \(.value.snapshots) snapshots, \(.value.size)"'

# Test guest-specific endpoint
TEST_VMID="100"
echo -e "\nTesting guest endpoint for VM $TEST_VMID:"
curl -s "$API_URL/guest/vm/$TEST_VMID" | jq '.'
```

## Continuous Validation Loop

```bash
#!/bin/bash
# continuous-validation.sh

while true; do
    clear
    echo "PBS Data Validation - $(date)"
    echo "================================"
    
    # Run validation
    ./validate-pbs-ui-api.sh
    
    # Check UI elements (requires browser console)
    cat << 'EOF'
    
    Run in browser console:
    -----------------------
    // Check if UI matches API
    const apiData = await fetch('/api/pbs/backups').then(r => r.json());
    const uiSnapshots = document.querySelector('[data-pbs-total]')?.textContent;
    console.log('API:', apiData.summary.totalSnapshots, 'UI:', uiSnapshots);
    console.assert(apiData.summary.totalSnapshots == uiSnapshots, 'Mismatch!');
EOF
    
    sleep 30
done
```

## Test Scenarios Checklist

### Basic Functionality
- [ ] PBS connection and authentication
- [ ] Snapshot enumeration
- [ ] Size calculation
- [ ] Namespace detection

### Collision Detection
- [ ] Same VMID across nodes (critical)
- [ ] Same VMID across namespaces (warning)
- [ ] Mixed VM/CT with same ID
- [ ] Namespace migration scenarios

### Node Information
- [ ] Container backup node extraction
- [ ] VM backup node handling
- [ ] Task-based node discovery
- [ ] Missing node information fallback

### UI Display
- [ ] Summary cards match API data
- [ ] Namespace grouping displays correctly
- [ ] Collision warnings appear
- [ ] Guest type icons are correct
- [ ] Backup counts are accurate

### Edge Cases
- [ ] PBS instance offline
- [ ] Empty namespaces
- [ ] Huge backup counts (>1000)
- [ ] Special characters in namespace names
- [ ] VMID reuse after guest deletion

## Performance Testing

```javascript
// Measure API response time
async function measurePBSAPIPerformance() {
    const iterations = 10;
    const times = [];
    
    for (let i = 0; i < iterations; i++) {
        const start = Date.now();
        await fetch('/api/pbs/backups');
        const duration = Date.now() - start;
        times.push(duration);
    }
    
    console.log('PBS API Performance:');
    console.log('- Average:', times.reduce((a,b) => a+b) / times.length, 'ms');
    console.log('- Min:', Math.min(...times), 'ms');
    console.log('- Max:', Math.max(...times), 'ms');
}
```

## Debugging Utilities

```javascript
// Debug namespace processing
function debugNamespaceData(pbsData) {
    console.group('PBS Namespace Debug');
    
    const namespaces = new Set();
    const vmidsByNamespace = new Map();
    
    pbsData.forEach(instance => {
        instance.pbsSnapshots?.forEach(snapshot => {
            const ns = snapshot.namespace || 'root';
            namespaces.add(ns);
            
            if (!vmidsByNamespace.has(ns)) {
                vmidsByNamespace.set(ns, new Set());
            }
            vmidsByNamespace.get(ns).add(snapshot['backup-id']);
        });
    });
    
    console.log('Namespaces found:', Array.from(namespaces));
    console.log('VMIDs per namespace:');
    vmidsByNamespace.forEach((vmids, ns) => {
        console.log(`  ${ns}: ${vmids.size} unique VMIDs`);
    });
    
    console.groupEnd();
}
```

## Mock Data Generator

```javascript
// Generate test PBS data for development
function generateMockPBSData(config = {}) {
    const {
        instances = 1,
        namespacesPerInstance = 3,
        backupsPerNamespace = 50,
        collisionRate = 0.1
    } = config;
    
    const mockData = [];
    const usedVMIDs = [];
    
    for (let i = 0; i < instances; i++) {
        const instance = {
            name: `pbs${i + 1}`,
            status: 'ok',
            pbsSnapshots: []
        };
        
        for (let n = 0; n < namespacesPerInstance; n++) {
            const namespace = n === 0 ? 'root' : `node${n}`;
            
            for (let b = 0; b < backupsPerNamespace; b++) {
                let vmid;
                if (Math.random() < collisionRate && usedVMIDs.length > 0) {
                    // Create collision
                    vmid = usedVMIDs[Math.floor(Math.random() * usedVMIDs.length)];
                } else {
                    // New VMID
                    vmid = 100 + Math.floor(Math.random() * 900);
                    usedVMIDs.push(vmid);
                }
                
                instance.pbsSnapshots.push({
                    'backup-id': String(vmid),
                    'backup-type': Math.random() > 0.5 ? 'vm' : 'ct',
                    'backup-time': Date.now() / 1000 - Math.random() * 30 * 86400,
                    namespace,
                    size: Math.floor(Math.random() * 10737418240) // 0-10GB
                });
            }
        }
        
        mockData.push(instance);
    }
    
    return mockData;
}
```