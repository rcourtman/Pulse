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

// Get PBS storage info
router.get('/backups/pbs-storage', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const pbsDataArray = currentState.pbs || [];
        
        if (pbsDataArray.length === 0) {
            return res.json({ datastores: [] });
        }
        
        const datastores = [];
        pbsDataArray.forEach(pbsInstance => {
            if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
                pbsInstance.datastores.forEach(datastore => {
                    if (datastore.status) {
                        datastores.push({
                            name: datastore.name,
                            total: datastore.status.total || 0,
                            used: datastore.status.used || 0,
                            available: datastore.status.avail || 0,
                            percentage: Math.round((datastore.status.used / datastore.status.total) * 100) || 0
                        });
                    }
                });
            }
        });
        
        res.json({ datastores });
    } catch (error) {
        console.error('Error fetching PBS storage info:', error);
        res.status(500).json({ error: 'Failed to fetch PBS storage info' });
    }
});

// Get unified view of all backups
router.get('/backups/unified', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const allBackups = [];
        const vmidCollisions = new Map();
        
        // Get all guests (VMs and containers) for coverage calculation
        const allGuests = [...(currentState.vms || []), ...(currentState.containers || [])];
        const guestMap = new Map(); // Map of vmid -> guest info
        
        
        // Build guest map for quick lookup
        allGuests.forEach(guest => {
            // Store by VMID, tracking all nodes where this VMID exists
            if (!guestMap.has(guest.vmid)) {
                guestMap.set(guest.vmid, []);
            }
            guestMap.get(guest.vmid).push({
                vmid: guest.vmid,
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT',
                node: guest.node,
                status: guest.status,
                endpointId: guest.endpointId
            });
        });
        
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
                                    source: 'pbs',
                                    deduplicationFactor: datastore.deduplicationFactor || null
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

        // Get PBS deduplication info if PBS is enabled
        let pbsStorageInfo = null;
        if (pbsEnabled && pbsDataArray.length > 0) {
            let totalUsed = 0;
            let totalAvailable = 0;
            let totalCapacity = 0;
            let avgDedupFactor = 0;
            let datastoreCount = 0;
            
            pbsDataArray.forEach(pbsInstance => {
                if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
                    pbsInstance.datastores.forEach(datastore => {
                        if (datastore.used !== undefined && datastore.total !== undefined) {
                            totalUsed += datastore.used;
                            totalAvailable += datastore.available || 0;
                            totalCapacity += datastore.total;
                            if (datastore.deduplicationFactor) {
                                avgDedupFactor += datastore.deduplicationFactor;
                                datastoreCount++;
                            }
                        }
                    });
                }
            });
            
            if (totalUsed > 0 || datastoreCount > 0) {
                avgDedupFactor = datastoreCount > 0 ? avgDedupFactor / datastoreCount : 1;
                pbsStorageInfo = {
                    actualUsed: totalUsed,
                    totalCapacity: totalCapacity,
                    available: totalAvailable,
                    deduplicationFactor: avgDedupFactor
                };
            }
        }
        
        // Calculate backup coverage
        const now = Date.now() / 1000; // Current time in seconds
        const oneDayAgo = now - (24 * 60 * 60);
        const twoDaysAgo = now - (48 * 60 * 60);
        const sevenDaysAgo = now - (7 * 24 * 60 * 60);
        
        const guestsWithBackups = new Map(); // vmid -> latest backup time
        
        // Find latest backup for each guest
        allBackups.forEach(backup => {
            const vmid = parseInt(backup.vmid, 10);
            const backupTime = backup.ctime;
            
            if (!isNaN(vmid) && (!guestsWithBackups.has(vmid) || backupTime > guestsWithBackups.get(vmid))) {
                guestsWithBackups.set(vmid, backupTime);
            }
        });
        
        
        // Calculate coverage statistics
        let totalGuests = 0;
        let backedUp24h = 0;
        let backedUp48h = 0;
        let backedUp7d = 0;
        let neverBackedUp = 0;
        const missingBackups = [];
        
        guestMap.forEach((guestInstances, vmid) => {
            // Count each unique VMID once
            totalGuests++;
            
            const lastBackupTime = guestsWithBackups.get(vmid);
            const guestInfo = guestInstances[0]; // Use first instance for display info
            
            if (lastBackupTime) {
                if (lastBackupTime >= oneDayAgo) backedUp24h++;
                if (lastBackupTime >= twoDaysAgo) backedUp48h++;
                if (lastBackupTime >= sevenDaysAgo) backedUp7d++;
                
                // If no backup in last 24h, add to missing list
                if (lastBackupTime < oneDayAgo) {
                    const daysSinceBackup = Math.floor((now - lastBackupTime) / (24 * 60 * 60));
                    missingBackups.push({
                        vmid: vmid,
                        name: guestInfo.name || `Guest ${vmid}`,
                        type: guestInfo.type,
                        nodes: guestInstances.map(g => g.node),
                        lastBackup: lastBackupTime,
                        daysSinceBackup: daysSinceBackup,
                        status: guestInfo.status
                    });
                }
            } else {
                neverBackedUp++;
                missingBackups.push({
                    vmid: vmid,
                    name: guestInfo.name || `Guest ${vmid}`,
                    type: guestInfo.type,
                    nodes: guestInstances.map(g => g.node),
                    lastBackup: null,
                    daysSinceBackup: null,
                    status: guestInfo.status
                });
            }
        });
        
        // Sort missing backups by days since backup (never backed up first)
        missingBackups.sort((a, b) => {
            if (a.lastBackup === null) return -1;
            if (b.lastBackup === null) return 1;
            return b.daysSinceBackup - a.daysSinceBackup;
        });
        
        const coverage = {
            totalGuests: totalGuests,
            backedUp24h: backedUp24h,
            backedUp48h: backedUp48h,
            backedUp7d: backedUp7d,
            neverBackedUp: neverBackedUp,
            percentage24h: totalGuests > 0 ? Math.round((backedUp24h / totalGuests) * 100) : 0,
            percentage48h: totalGuests > 0 ? Math.round((backedUp48h / totalGuests) * 100) : 0,
            percentage7d: totalGuests > 0 ? Math.round((backedUp7d / totalGuests) * 100) : 0,
            missingBackups: missingBackups
        };
        
        res.json({
            backups: allBackups,
            pbs: {
                enabled: pbsEnabled,
                servers: pbsDataArray.map(pbs => pbs.pbsInstanceName || pbs.name || 'PBS'),
                storageInfo: pbsStorageInfo
            },
            collisions: collisions,
            coverage: coverage,
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