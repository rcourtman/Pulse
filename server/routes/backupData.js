/**
 * Backup Data API Routes
 * New API endpoints for the refactored backup system
 */

const express = require('express');
const router = express.Router();
const { processBackupDataWithCoordinator } = require('../dataFetcher');
const stateManager = require('../state');

/**
 * GET /api/backup-data
 * Get processed backup data with optional filters
 */
router.get('/', async (req, res) => {
    try {
        // Get current state
        const currentState = stateManager.getState();
        
        // Get filters from query parameters
        const filters = {
            namespaceFilter: req.query.namespace || 'all',
            backupTypeFilter: req.query.backupType || 'all'
        };
        
        // Extract data from state
        const vms = currentState.vms || [];
        const containers = currentState.containers || [];
        const pbsInstances = currentState.pbs || [];
        const pveBackupData = currentState.pveBackups || {
            storageBackups: [],
            guestSnapshots: []
        };
        
        // Process backup data with the new system
        const result = await processBackupDataWithCoordinator(
            vms,
            containers,
            pbsInstances,
            pveBackupData,
            filters
        );
        
        res.json({
            success: true,
            data: result,
            filters: filters,
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        console.error('[BackupDataAPI] Error processing backup data:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/backup-data/namespaces
 * Get available PBS namespaces
 */
router.get('/namespaces', async (req, res) => {
    try {
        // Get current state
        const currentState = stateManager.getState();
        
        // Get PBS instances to extract namespaces
        const pbsInstances = currentState.pbs || [];
        const namespaces = new Set(['root']); // Always include root
        
        pbsInstances.forEach(instance => {
            if (instance.datastores) {
                instance.datastores.forEach(datastore => {
                    if (datastore.snapshots) {
                        datastore.snapshots.forEach(snapshot => {
                            const namespace = snapshot.namespace || 'root';
                            namespaces.add(namespace === '' ? 'root' : namespace);
                        });
                    }
                });
            }
        });
        
        res.json({
            success: true,
            namespaces: Array.from(namespaces).sort()
        });
    } catch (error) {
        console.error('[BackupDataAPI] Error getting namespaces:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/backup-data/guest/:compositeKey
 * Get detailed backup information for a specific guest
 */
router.get('/guest/:compositeKey', async (req, res) => {
    try {
        const { compositeKey } = req.params;
        
        // Get current state
        const currentState = stateManager.getState();
        
        // Extract data from state
        const vms = currentState.vms || [];
        const containers = currentState.containers || [];
        const pbsInstances = currentState.pbs || [];
        const pveBackupData = currentState.pveBackups || {
            storageBackups: [],
            guestSnapshots: []
        };
        
        // Process all data to get guest details
        const result = await processBackupDataWithCoordinator(
            vms,
            containers,
            pbsInstances,
            pveBackupData
        );
        
        // Find the specific guest
        const guestData = result.backupStatusByGuest.find(g => g.compositeKey === compositeKey);
        
        if (!guestData) {
            return res.status(404).json({
                success: false,
                error: 'Guest not found'
            });
        }
        
        res.json({
            success: true,
            data: guestData
        });
    } catch (error) {
        console.error('[BackupDataAPI] Error getting guest backup data:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

/**
 * GET /api/backup-data/debug
 * Debug endpoint to check data structures
 */
router.get('/debug', async (req, res) => {
    try {
        const currentState = stateManager.getState();
        
        const debugInfo = {
            vmsCount: currentState.vms?.length || 0,
            containersCount: currentState.containers?.length || 0,
            pbsInstancesCount: currentState.pbs?.length || 0,
            sampleVm: currentState.vms?.[0] || null,
            sampleContainer: currentState.containers?.[0] || null,
            pbsStructure: null
        };
        
        // Check PBS structure
        if (currentState.pbs && currentState.pbs.length > 0) {
            const firstPbs = currentState.pbs[0];
            debugInfo.pbsStructure = {
                name: firstPbs.pbsInstanceName || firstPbs.name || 'unknown',
                datastoresCount: firstPbs.datastores?.length || 0,
                firstDatastore: null
            };
            
            if (firstPbs.datastores && firstPbs.datastores.length > 0) {
                const firstDs = firstPbs.datastores[0];
                debugInfo.pbsStructure.firstDatastore = {
                    name: firstDs.name,
                    snapshotsCount: firstDs.snapshots?.length || 0,
                    sampleSnapshot: firstDs.snapshots?.[0] || null
                };
            }
        }
        
        res.json({
            success: true,
            debug: debugInfo
        });
    } catch (error) {
        console.error('[BackupDataAPI] Debug error:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

module.exports = router;