const express = require('express');
const router = express.Router();
const stateManager = require('../state');

router.get('/snapshots', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const snapshots = [];
        
        // Create a map of VMIDs to VM names
        const vmNames = new Map();
        if (currentState.vms) {
            currentState.vms.forEach(vm => {
                vmNames.set(vm.vmid, vm.name);
            });
        }
        if (currentState.containers) {
            currentState.containers.forEach(ct => {
                vmNames.set(ct.vmid, ct.name);
            });
        }
        
        // Extract snapshots from pveBackups.guestSnapshots data
        if (currentState.pveBackups && currentState.pveBackups.guestSnapshots) {
            // guestSnapshots is an array
            if (Array.isArray(currentState.pveBackups.guestSnapshots)) {
                currentState.pveBackups.guestSnapshots.forEach(snapshot => {
                    snapshots.push({
                        node: snapshot.node || 'unknown',
                        vmid: snapshot.vmid,
                        name: vmNames.get(snapshot.vmid) || `VM ${snapshot.vmid}`,
                        type: snapshot.type || 'unknown',
                        snapname: snapshot.name,
                        snaptime: snapshot.snaptime,
                        description: snapshot.description || '',
                        size: snapshot.size || 0,
                        parent: snapshot.parent || null,
                        running: snapshot.vmstate || false
                    });
                });
            }
        }

        // Sort snapshots by timestamp (newest first)
        snapshots.sort((a, b) => (b.snaptime || 0) - (a.snaptime || 0));

        res.json({
            snapshots: snapshots,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error('Error fetching snapshots:', error);
        res.status(500).json({ 
            error: 'Failed to fetch snapshots',
            message: error.message 
        });
    }
});

// Take a new snapshot
router.post('/snapshots/:node/:vmid', (req, res) => {
    const { node, vmid } = req.params;
    const { snapname, description } = req.body;
    
    // This is a placeholder - in a real implementation, this would
    // trigger a PVE API call to create a snapshot
    res.status(501).json({
        error: 'Not implemented',
        message: 'Snapshot creation requires direct PVE API integration'
    });
});

// Delete a snapshot
router.delete('/snapshots/:node/:vmid/:snapname', (req, res) => {
    const { node, vmid, snapname } = req.params;
    
    // This is a placeholder - in a real implementation, this would
    // trigger a PVE API call to delete a snapshot
    res.status(501).json({
        error: 'Not implemented',
        message: 'Snapshot deletion requires direct PVE API integration'
    });
});

module.exports = router;