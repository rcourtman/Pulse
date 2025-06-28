const express = require('express');
const { detectVmidCollisions, analyzePbsConfiguration } = require('./pbsUtils');

const router = express.Router();

/**
 * Extract and process backup data from PBS instances
 * @param {Array} pbsInstances - Array of PBS instance data
 * @returns {Object} Processed backup data
 */
function processBackupData(pbsInstances) {
    const result = {
        instances: [],
        globalStats: {
            totalBackups: 0,
            totalSize: 0,
            totalGuests: new Set(),
            namespaces: new Set(),
            collisions: []
        }
    };
    
    pbsInstances.forEach(instance => {
        const instanceData = {
            id: instance.pbsEndpointId,
            name: instance.pbsInstanceName,
            status: instance.status,
            namespaces: {},
            datastores: [],
            collisions: [],
            stats: {
                totalBackups: 0,
                totalSize: 0,
                totalGuests: new Set()
            }
        };
        
        // Process datastores and snapshots
        if (instance.datastores) {
            instance.datastores.forEach(ds => {
                const datastoreInfo = {
                    name: ds.name,
                    usage: {
                        used: ds.used,
                        total: ds.total,
                        available: ds.available,
                        percentage: ds.total > 0 ? Math.round((ds.used / ds.total) * 100) : 0
                    },
                    backupCount: 0
                };
                
                if (ds.snapshots && Array.isArray(ds.snapshots)) {
                    datastoreInfo.backupCount = ds.snapshots.length;
                    
                    // Process snapshots by namespace
                    ds.snapshots.forEach(snap => {
                        const namespace = snap.namespace || 'root';
                        const guestId = `${snap['backup-type']}/${snap['backup-id']}`;
                        
                        // Initialize namespace if needed
                        if (!instanceData.namespaces[namespace]) {
                            instanceData.namespaces[namespace] = {
                                vms: {},
                                cts: {},
                                totalBackups: 0,
                                totalSize: 0,
                                oldestBackup: null,
                                newestBackup: null
                            };
                        }
                        
                        const nsData = instanceData.namespaces[namespace];
                        nsData.totalBackups++;
                        nsData.totalSize += snap.size || 0;
                        
                        // Track oldest/newest
                        const backupTime = snap['backup-time'];
                        if (!nsData.oldestBackup || backupTime < nsData.oldestBackup) {
                            nsData.oldestBackup = backupTime;
                        }
                        if (!nsData.newestBackup || backupTime > nsData.newestBackup) {
                            nsData.newestBackup = backupTime;
                        }
                        
                        // Group by guest type
                        const guestType = snap['backup-type'];
                        const guestList = guestType === 'vm' ? nsData.vms : nsData.cts;
                        
                        if (!guestList[snap['backup-id']]) {
                            guestList[snap['backup-id']] = {
                                id: snap['backup-id'],
                                backups: [],
                                totalSize: 0,
                                latestBackup: 0,
                                oldestBackup: backupTime
                            };
                        }
                        
                        const guestData = guestList[snap['backup-id']];
                        guestData.backups.push({
                            time: backupTime,
                            size: snap.size || 0,
                            verified: snap.verification ? true : false,
                            comment: snap.comment || ''
                        });
                        guestData.totalSize += snap.size || 0;
                        guestData.latestBackup = Math.max(guestData.latestBackup, backupTime);
                        
                        // Update instance stats
                        instanceData.stats.totalBackups++;
                        instanceData.stats.totalSize += snap.size || 0;
                        instanceData.stats.totalGuests.add(guestId);
                        
                        // Update global stats
                        result.globalStats.totalBackups++;
                        result.globalStats.totalSize += snap.size || 0;
                        result.globalStats.totalGuests.add(guestId);
                        result.globalStats.namespaces.add(namespace);
                    });
                    
                    // Detect collisions
                    if (ds.snapshots._collisionInfo) {
                        instanceData.collisions.push({
                            datastore: ds.name,
                            ...ds.snapshots._collisionInfo
                        });
                    }
                }
                
                instanceData.datastores.push(datastoreInfo);
            });
        }
        
        // Convert Sets to counts
        instanceData.stats.totalGuests = instanceData.stats.totalGuests.size;
        
        // Add collision analysis
        if (instance.vmidCollisions) {
            instanceData.collisionAnalysis = instance.vmidCollisions;
        }
        
        if (instance.configurationAnalysis) {
            instanceData.configurationAnalysis = instance.configurationAnalysis;
        }
        
        result.instances.push(instanceData);
    });
    
    // Convert global Sets to counts
    result.globalStats.totalGuests = result.globalStats.totalGuests.size;
    result.globalStats.namespaces = Array.from(result.globalStats.namespaces);
    
    return result;
}

/**
 * Get backup data for a specific guest across all PBS instances
 * @param {Array} pbsInstances - Array of PBS instance data
 * @param {string} guestType - 'vm' or 'ct'
 * @param {string} guestId - Guest ID
 * @returns {Object} Guest backup data
 */
function getGuestBackups(pbsInstances, guestType, guestId) {
    const result = {
        guestType,
        guestId,
        instances: [],
        totalBackups: 0,
        totalSize: 0,
        oldestBackup: null,
        newestBackup: null
    };
    
    pbsInstances.forEach(instance => {
        if (!instance.datastores) return;
        
        const instanceBackups = {
            instanceId: instance.pbsEndpointId,
            instanceName: instance.pbsInstanceName,
            namespaces: {}
        };
        
        instance.datastores.forEach(ds => {
            if (!ds.snapshots) return;
            
            ds.snapshots.forEach(snap => {
                if (snap['backup-type'] === guestType && snap['backup-id'] === guestId) {
                    const namespace = snap.namespace || 'root';
                    
                    if (!instanceBackups.namespaces[namespace]) {
                        instanceBackups.namespaces[namespace] = {
                            datastore: ds.name,
                            backups: []
                        };
                    }
                    
                    instanceBackups.namespaces[namespace].backups.push({
                        time: snap['backup-time'],
                        size: snap.size || 0,
                        verified: snap.verification ? true : false,
                        comment: snap.comment || ''
                    });
                    
                    // Update totals
                    result.totalBackups++;
                    result.totalSize += snap.size || 0;
                    
                    const backupTime = snap['backup-time'];
                    if (!result.oldestBackup || backupTime < result.oldestBackup) {
                        result.oldestBackup = backupTime;
                    }
                    if (!result.newestBackup || backupTime > result.newestBackup) {
                        result.newestBackup = backupTime;
                    }
                }
            });
        });
        
        if (Object.keys(instanceBackups.namespaces).length > 0) {
            result.instances.push(instanceBackups);
        }
    });
    
    return result;
}

// Routes

/**
 * GET /api/pbs/backups
 * Get all backup data with optional filtering
 */
router.get('/backups', (req, res) => {
    try {
        const stateManager = req.app.locals.stateManager;
        const currentState = stateManager.getState();
        const pbsData = currentState.pbs || [];
        
        if (pbsData.length === 0) {
            return res.json({
                success: true,
                data: {
                    instances: [],
                    globalStats: {
                        totalBackups: 0,
                        totalSize: 0,
                        totalGuests: 0,
                        namespaces: [],
                        collisions: []
                    }
                }
            });
        }
        
        const processedData = processBackupData(pbsData);
        
        res.json({
            success: true,
            data: processedData
        });
    } catch (error) {
        console.error('[PBS Backups API] Error:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/pbs/backups/:instanceId
 * Get backup data for a specific PBS instance
 */
router.get('/backups/:instanceId', (req, res) => {
    try {
        const { instanceId } = req.params;
        const stateManager = req.app.locals.stateManager;
        const currentState = stateManager.getState();
        const pbsData = currentState.pbs || [];
        
        const instance = pbsData.find(inst => inst.pbsEndpointId === instanceId);
        
        if (!instance) {
            return res.status(404).json({
                success: false,
                error: 'PBS instance not found'
            });
        }
        
        const processedData = processBackupData([instance]);
        
        res.json({
            success: true,
            data: processedData.instances[0]
        });
    } catch (error) {
        console.error('[PBS Backups API] Error:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/pbs/backups/guest/:type/:id
 * Get all backups for a specific guest
 */
router.get('/backups/guest/:type/:id', (req, res) => {
    try {
        const { type, id } = req.params;
        const stateManager = req.app.locals.stateManager;
        const currentState = stateManager.getState();
        const pbsData = currentState.pbs || [];
        
        if (!['vm', 'ct'].includes(type)) {
            return res.status(400).json({
                success: false,
                error: 'Invalid guest type. Must be "vm" or "ct"'
            });
        }
        
        const guestData = getGuestBackups(pbsData, type, id);
        
        res.json({
            success: true,
            data: guestData
        });
    } catch (error) {
        console.error('[PBS Backups API] Error:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/pbs/backups/date/:date
 * Get all backups for a specific date
 * @param date - Date in YYYY-MM-DD format
 */
router.get('/backups/date/:date', (req, res) => {
    try {
        const { date } = req.params;
        const stateManager = req.app.locals.stateManager;
        if (!stateManager) {
            return res.status(500).json({
                success: false,
                error: 'State manager not initialized'
            });
        }
        
        const currentState = stateManager.getState();
        const pbsData = currentState.pbs || [];
        
        // Parse the date in UTC
        const [year, month, day] = date.split('-').map(Number);
        if (!year || !month || !day) {
            return res.status(400).json({
                success: false,
                error: 'Invalid date format. Use YYYY-MM-DD'
            });
        }
        
        // Create UTC date range
        const startOfDay = new Date(Date.UTC(year, month - 1, day, 0, 0, 0, 0));
        const endOfDay = new Date(Date.UTC(year, month - 1, day, 23, 59, 59, 999));
        
        // Convert to Unix timestamps
        const startTimestamp = Math.floor(startOfDay.getTime() / 1000);
        const endTimestamp = Math.floor(endOfDay.getTime() / 1000);
        
        const result = {
            date: date,
            startTime: startOfDay.toISOString(),
            endTime: endOfDay.toISOString(),
            instances: [],
            summary: {
                totalBackups: 0,
                totalSize: 0,
                byType: {
                    vm: { count: 0, size: 0 },
                    ct: { count: 0, size: 0 }
                },
                byNamespace: {},
                byHour: Array(24).fill(0),
                guests: new Set()
            }
        };
        
        // Process each PBS instance
        pbsData.forEach(instance => {
            if (!instance.datastores) return;
            
            const instanceResult = {
                instanceId: instance.pbsEndpointId,
                instanceName: instance.pbsInstanceName,
                backups: []
            };
            
            instance.datastores.forEach(ds => {
                if (!ds.snapshots) return;
                
                ds.snapshots.forEach(snap => {
                    const backupTime = snap['backup-time'];
                    
                    // Check if backup is within the date range
                    if (backupTime >= startTimestamp && backupTime <= endTimestamp) {
                        const backupData = {
                            time: backupTime,
                            timeString: new Date(backupTime * 1000).toISOString(),
                            type: snap['backup-type'],
                            id: snap['backup-id'],
                            size: snap.size || 0,
                            datastore: ds.name,
                            namespace: snap.namespace || 'root',
                            verified: snap.verification ? true : false,
                            comment: snap.comment || '',
                            owner: snap.owner || '',
                            extractedNode: snap.extractedNode || null
                        };
                        
                        instanceResult.backups.push(backupData);
                        
                        // Update summary
                        result.summary.totalBackups++;
                        result.summary.totalSize += backupData.size;
                        result.summary.byType[backupData.type].count++;
                        result.summary.byType[backupData.type].size += backupData.size;
                        
                        // Track by namespace
                        if (!result.summary.byNamespace[backupData.namespace]) {
                            result.summary.byNamespace[backupData.namespace] = {
                                count: 0,
                                size: 0,
                                vms: new Set(),
                                cts: new Set()
                            };
                        }
                        result.summary.byNamespace[backupData.namespace].count++;
                        result.summary.byNamespace[backupData.namespace].size += backupData.size;
                        
                        if (backupData.type === 'vm') {
                            result.summary.byNamespace[backupData.namespace].vms.add(backupData.id);
                        } else {
                            result.summary.byNamespace[backupData.namespace].cts.add(backupData.id);
                        }
                        
                        // Track by hour
                        const backupHour = new Date(backupTime * 1000).getHours();
                        result.summary.byHour[backupHour]++;
                        
                        // Track unique guests
                        result.summary.guests.add(`${backupData.type}/${backupData.id}`);
                    }
                });
            });
            
            if (instanceResult.backups.length > 0) {
                // Sort backups by time
                instanceResult.backups.sort((a, b) => b.time - a.time);
                result.instances.push(instanceResult);
            }
        });
        
        // Convert Sets to counts
        result.summary.guests = result.summary.guests.size;
        Object.keys(result.summary.byNamespace).forEach(ns => {
            const nsData = result.summary.byNamespace[ns];
            nsData.vms = nsData.vms.size;
            nsData.cts = nsData.cts.size;
        });
        
        res.json({
            success: true,
            data: result
        });
    } catch (error) {
        console.error('[PBS Backups API] Error in date query:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/pbs/collisions
 * Get VMID collision analysis
 */
router.get('/collisions', (req, res) => {
    try {
        const stateManager = req.app.locals.stateManager;
        const currentState = stateManager.getState();
        const pbsData = currentState.pbs || [];
        
        const collisionAnalysis = {
            hasCollisions: false,
            totalCollisions: 0,
            criticalCollisions: 0,
            warningCollisions: 0,
            byInstance: []
        };
        
        pbsData.forEach(instance => {
            if (instance.vmidCollisions && instance.vmidCollisions.detected) {
                collisionAnalysis.hasCollisions = true;
                collisionAnalysis.totalCollisions += instance.vmidCollisions.totalCollisions;
                
                if (instance.vmidCollisions.severity === 'critical') {
                    collisionAnalysis.criticalCollisions += instance.vmidCollisions.totalCollisions;
                } else {
                    collisionAnalysis.warningCollisions += instance.vmidCollisions.totalCollisions;
                }
                
                collisionAnalysis.byInstance.push({
                    instanceId: instance.pbsEndpointId,
                    instanceName: instance.pbsInstanceName,
                    collisions: instance.vmidCollisions
                });
            }
        });
        
        res.json({
            success: true,
            data: collisionAnalysis
        });
    } catch (error) {
        console.error('[PBS Backups API] Error:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

module.exports = router;