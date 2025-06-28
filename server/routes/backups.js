const express = require('express');
const router = express.Router();
const stateManager = require('../state');

// Get PVE backups
router.get('/backups/pve', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const backups = [];
        
        // Extract backups from pveBackups.storageBackups data
        if (currentState.pveBackups && currentState.pveBackups.storageBackups) {
            // storageBackups is an array
            if (Array.isArray(currentState.pveBackups.storageBackups)) {
                currentState.pveBackups.storageBackups.forEach(backup => {
                    backups.push({
                        node: backup.node || 'unknown',
                        storage: backup.storage || 'unknown',
                        volid: backup.volid,
                        vmid: backup.vmid,
                        ctime: backup.ctime,
                        format: backup.format,
                        size: backup.size || 0,
                        content: backup.content,
                        notes: backup.notes || '',
                        type: 'pve'
                    });
                });
            }
        }

        // Sort backups by timestamp (newest first)
        backups.sort((a, b) => (b.ctime || 0) - (a.ctime || 0));

        res.json({
            backups: backups,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error('Error fetching PVE backups:', error);
        res.status(500).json({ 
            error: 'Failed to fetch PVE backups',
            message: error.message 
        });
    }
});

// Get PBS backups (if configured)
router.get('/backups/pbs', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const backups = [];
        
        // Check if PBS is configured
        const pbsDataArray = currentState.pbs || [];
        if (pbsDataArray.length === 0) {
            return res.json({
                enabled: false,
                backups: [],
                timestamp: Date.now()
            });
        }

        // Extract PBS backup data from datastores
        pbsDataArray.forEach(pbsInstance => {
            if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
                pbsInstance.datastores.forEach(datastore => {
                    if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
                        datastore.snapshots.forEach(backup => {
                            let totalSize = 0;
                            if (backup.files && Array.isArray(backup.files)) {
                                totalSize = backup.files.reduce((sum, file) => sum + (file.size || 0), 0);
                            }
                            
                            backups.push({
                                server: pbsInstance.pbsInstanceName || 'PBS',
                                datastore: datastore.name,
                                namespace: backup.namespace || 'root',
                                vmid: backup['backup-id'],
                                ctime: backup['backup-time'],
                                size: totalSize,
                                type: backup['backup-type'],
                                verified: backup.verification && backup.verification.state === 'ok',
                                protected: backup.protected || false,
                                notes: backup.comment || '',
                                backupType: 'pbs'
                            });
                        });
                    }
                });
            }
        });

        // Sort backups by timestamp (newest first)
        backups.sort((a, b) => (b.ctime || 0) - (a.ctime || 0));

        res.json({
            enabled: true,
            backups: backups,
            namespaces: extractNamespaces(pbsDataArray),
            timestamp: Date.now()
        });
    } catch (error) {
        console.error('Error fetching PBS backups:', error);
        res.status(500).json({ 
            error: 'Failed to fetch PBS backups',
            message: error.message 
        });
    }
});

// Get unified view of all backups
router.get('/backups/unified', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const allBackups = [];
        const vmidCollisions = new Map();
        
        // Get PVE backups
        if (currentState.pveBackups && currentState.pveBackups.storageBackups) {
            // storageBackups is an array
            if (Array.isArray(currentState.pveBackups.storageBackups)) {
                currentState.pveBackups.storageBackups.forEach(backup => {
                    const backupData = {
                        node: backup.node || 'unknown',
                        storage: backup.storage || 'unknown',
                        volid: backup.volid,
                        vmid: backup.vmid,
                        ctime: backup.ctime,
                        format: backup.format,
                        size: backup.size || 0,
                        content: backup.content,
                        notes: backup.notes || '',
                        type: 'pve',
                        source: 'pve'
                    };
                    
                    allBackups.push(backupData);
                    
                    // Track VMIDs for collision detection
                    if (!vmidCollisions.has(backup.vmid)) {
                        vmidCollisions.set(backup.vmid, new Set());
                    }
                    vmidCollisions.get(backup.vmid).add(backup.node || 'unknown');
                });
            }
        }

        // Get PBS backups if configured
        const pbsDataArray = currentState.pbs || [];
        const pbsEnabled = pbsDataArray.length > 0;
        
        if (pbsEnabled) {
            pbsDataArray.forEach(pbsInstance => {
                if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
                    pbsInstance.datastores.forEach(datastore => {
                        if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
                            datastore.snapshots.forEach(backup => {
                                let totalSize = 0;
                                if (backup.files && Array.isArray(backup.files)) {
                                    totalSize = backup.files.reduce((sum, file) => sum + (file.size || 0), 0);
                                }
                                
                                const backupData = {
                                    server: pbsInstance.pbsInstanceName || 'PBS',
                                    datastore: datastore.name,
                                    namespace: backup.namespace || 'root',
                                    vmid: backup['backup-id'],
                                    ctime: backup['backup-time'],
                                    size: totalSize,
                                    type: backup['backup-type'],
                                    verified: backup.verification && backup.verification.state === 'ok',
                                    protected: backup.protected || false,
                                    notes: backup.comment || '',
                                    source: 'pbs'
                                };
                                
                                allBackups.push(backupData);
                            });
                        }
                    });
                }
            });
        }

        // Sort all backups by timestamp (newest first)
        allBackups.sort((a, b) => (b.ctime || 0) - (a.ctime || 0));

        // Identify VMIDs with collisions
        const collisions = [];
        vmidCollisions.forEach((nodes, vmid) => {
            if (nodes.size > 1) {
                collisions.push({
                    vmid: vmid,
                    nodes: Array.from(nodes)
                });
            }
        });

        res.json({
            backups: allBackups,
            pbs: {
                enabled: pbsEnabled,
                servers: pbsDataArray.map(pbs => pbs.name)
            },
            collisions: collisions,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error('Error fetching unified backups:', error);
        res.status(500).json({ 
            error: 'Failed to fetch unified backups',
            message: error.message 
        });
    }
});

function extractNamespaces(pbsDataArray) {
    const namespaces = new Set();
    pbsDataArray.forEach(pbsInstance => {
        if (pbsInstance.backups && Array.isArray(pbsInstance.backups)) {
            pbsInstance.backups.forEach(backup => {
                namespaces.add(backup.namespace || 'root');
            });
        }
    });
    return Array.from(namespaces);
}

module.exports = router;