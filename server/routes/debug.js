
const express = require('express');
const stateManager = require('../state');
const fs = require('fs');
const path = require('path');

const router = express.Router();

// Debug endpoint to check nodes data
router.get('/nodes', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const nodes = currentState.nodes || [];
        res.json({
            nodeCount: nodes.length,
            nodes: nodes.map(node => ({
                node: node.node,
                name: node.name,
                displayName: node.displayName,
                clusterIdentifier: node.clusterIdentifier,
                endpointType: node.endpointType,
                endpointId: node.endpointId,
                status: node.status
            }))
        });
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Debug endpoint for namespace filter testing
router.get('/namespace-filter-test', (req, res) => {
    const namespace = req.query.namespace || 'root';
    const pbsData = stateManager.get('pbsData') || [];
    const result = {
        requestedNamespace: namespace,
        pbsInstances: [],
        totalSnapshotsBeforeFilter: 0,
        totalSnapshotsAfterFilter: 0,
        snapshotsByVMID: {}
    };
    
    pbsData.forEach((pbs, index) => {
        let instanceSnapshots = 0;
        let instanceFilteredSnapshots = 0;
        
        if (pbs.datastores) {
            pbs.datastores.forEach(ds => {
                if (ds.snapshots) {
                    ds.snapshots.forEach(snap => {
                        instanceSnapshots++;
                        const snapNamespace = snap.namespace || 'root';
                        
                        if (snapNamespace === namespace) {
                            instanceFilteredSnapshots++;
                            const vmid = snap['backup-id'];
                            if (!result.snapshotsByVMID[vmid]) {
                                result.snapshotsByVMID[vmid] = {
                                    count: 0,
                                    snapshots: []
                                };
                            }
                            result.snapshotsByVMID[vmid].count++;
                            result.snapshotsByVMID[vmid].snapshots.push({
                                namespace: snap.namespace,
                                type: snap['backup-type'],
                                time: snap['backup-time'],
                                owner: snap.owner
                            });
                        }
                    });
                }
            });
        }
        
        result.pbsInstances.push({
            name: pbs.pbsInstanceName,
            totalSnapshots: instanceSnapshots,
            filteredSnapshots: instanceFilteredSnapshots
        });
        result.totalSnapshotsBeforeFilter += instanceSnapshots;
        result.totalSnapshotsAfterFilter += instanceFilteredSnapshots;
    });
    
    res.json(result);
});

// Debug endpoint to test namespace filter
router.get('/test-namespace-filter', (req, res) => {
    const namespace = req.query.namespace || 'root';
    
    // Get current state
    const pbsData = stateManager.get('pbsData') || [];
    const vms = stateManager.get('vms') || [];
    const containers = stateManager.get('containers') || [];
    
    // Process the data as the frontend would
    let pbsSnapshots = [];
    pbsData.forEach(pbs => {
        if (pbs.datastores) {
            pbs.datastores.forEach(ds => {
                if (ds.snapshots) {
                    ds.snapshots.forEach(snap => {
                        pbsSnapshots.push({
                            ...snap,
                            pbsInstanceName: pbs.pbsInstanceName,
                            source: 'pbs',
                            'backup-time': snap['backup-time'],
                            backupVMID: snap['backup-id'],
                            backupType: snap['backup-type'],
                            namespace: snap.namespace || 'root'
                        });
                    });
                }
            });
        }
    });
    
    // Filter by namespace
    const originalCount = pbsSnapshots.length;
    if (namespace !== 'all') {
        pbsSnapshots = pbsSnapshots.filter(snap => {
            const snapNamespace = snap.namespace || 'root';
            return snapNamespace === namespace;
        });
    }
    
    // Build snapshotsByGuest map
    const snapshotsByGuest = new Map();
    const allGuests = [...vms, ...containers];
    
    pbsSnapshots.forEach(snap => {
        // Simulate the key building logic
        const owner = snap.owner || '';
        let endpointSuffix = '-unknown';
        
        if (owner && owner.includes('!')) {
            const ownerToken = owner.split('!')[1].toLowerCase();
            const matchingGuest = allGuests.find(g => {
                if (!g.nodeDisplayName || !g.endpointId || g.endpointId === 'primary') return false;
                const clusterName = g.nodeDisplayName.split(' - ')[0].toLowerCase();
                return clusterName === ownerToken;
            });
            
            if (matchingGuest) {
                endpointSuffix = `-${matchingGuest.endpointId}`;
            } else {
                endpointSuffix = `-primary-${ownerToken}`;
            }
        }
        
        const snapNamespace = snap.namespace || 'root';
        const key = `${snap.backupVMID}-${snap.backupType}-${snap.pbsInstanceName}-${snapNamespace}${endpointSuffix}`;
        
        if (!snapshotsByGuest.has(key)) {
            snapshotsByGuest.set(key, []);
        }
        snapshotsByGuest.get(key).push(snap);
    });
    
    // Show what keys were created
    const keysByVMID = {};
    Array.from(snapshotsByGuest.keys()).forEach(key => {
        const vmid = key.split('-')[0];
        if (!keysByVMID[vmid]) {
            keysByVMID[vmid] = [];
        }
        keysByVMID[vmid].push({
            key,
            count: snapshotsByGuest.get(key).length
        });
    });
    
    res.json({
        namespace,
        originalCount,
        filteredCount: pbsSnapshots.length,
        snapshotKeys: Array.from(snapshotsByGuest.keys()),
        keysByVMID,
        sampleSnapshots: pbsSnapshots.slice(0, 5).map(s => ({
            vmid: s.backupVMID,
            type: s.backupType,
            namespace: s.namespace,
            owner: s.owner
        }))
    });
});

// Debug endpoint for PBS namespace data
router.get('/pbs-namespaces', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const pbsDataArray = currentState.pbs || [];
        
        const namespaceInfo = pbsDataArray.map(pbs => ({
            name: pbs.pbsInstanceName,
            datastores: (pbs.datastores || []).map(ds => ({
                name: ds.name,
                snapshots: (ds.snapshots || []).map(snap => ({
                    id: snap['backup-id'],
                    namespace: snap.namespace,
                    type: snap['backup-type']
                })).slice(0, 5) // First 5 snapshots only
            }))
        }));
        
        // Collect all unique namespaces
        const allNamespaces = new Set();
        pbsDataArray.forEach(pbs => {
            (pbs.datastores || []).forEach(ds => {
                (ds.snapshots || []).forEach(snap => {
                    allNamespaces.add(snap.namespace || 'root');
                });
            });
        });
        
        res.json({
            namespaces: Array.from(allNamespaces).sort(),
            pbsInstances: namespaceInfo,
            summary: {
                totalPbsInstances: pbsDataArray.length,
                totalNamespaces: allNamespaces.size
            }
        });
    } catch (error) {
        console.error('[API] Error in PBS namespace debug endpoint:', error);
        res.status(500).json({ error: 'Internal server error' });
    }
});

// Debug endpoint for snapshot timestamps
router.get('/snapshots', (req, res) => {
    try {
        const currentState = stateManager.getState();
        const guestSnapshots = currentState.pveBackups?.guestSnapshots || [];
        const now = Math.floor(Date.now() / 1000);
        
        const snapshotDebug = guestSnapshots.map(snap => {
            const ageSeconds = now - snap.snaptime;
            const ageHours = ageSeconds / 3600;
            const ageDays = ageHours / 24;
            
            return {
                vmid: snap.vmid,
                name: snap.name,
                node: snap.node,
                type: snap.type,
                snaptime: snap.snaptime,
                originalSnaptime: snap.originalSnaptime,
                timestampIssue: snap.timestampIssue,
                date: new Date(snap.snaptime * 1000).toISOString(),
                ageHours: ageHours.toFixed(2),
                ageDays: ageDays.toFixed(2),
                willShowAsJustNow: ageDays < 0.042,
                currentTime: now,
                currentDate: new Date().toISOString()
            };
        });
        
        snapshotDebug.sort((a, b) => parseFloat(a.ageHours) - parseFloat(b.ageHours));
        
        const recentSnapshots = snapshotDebug.filter(s => s.willShowAsJustNow);
        const summary = {
            totalSnapshots: snapshotDebug.length,
            recentSnapshots: recentSnapshots.length,
            guestsWithRecentSnapshots: new Set(recentSnapshots.map(s => s.vmid)).size,
            currentTime: now,
            currentDate: new Date().toISOString()
        };
        
        res.json({
            summary,
            recentSnapshots,
            allSnapshots: snapshotDebug
        });
    } catch (error) {
        console.error('[API] Error in snapshot debug endpoint:', error);
        res.status(500).json({ error: 'Internal server error' });
    }
});

// Automated namespace test endpoint
router.post('/namespace-automated-test', (req, res) => {
    try {
        const debugData = req.body;
        console.log('[AUTO DEBUG] Received automated test results');
        
        // Analyze the test results
        const analysis = {
            timestamp: new Date().toISOString(),
            summary: {
                totalTests: debugData.tests?.length || 0,
                passed: debugData.tests?.filter(t => t.success).length || 0,
                failed: debugData.tests?.filter(t => !t.success).length || 0
            },
            issues: []
        };
        
        // Check for common issues
        if (debugData.domState?.table?.rowCount === 0 && debugData.filterState?.namespace === 'root') {
            analysis.issues.push({
                type: 'NO_ROWS_IN_ROOT_NAMESPACE',
                details: 'Root namespace selected but no rows displayed',
                filterState: debugData.filterState,
                tableState: debugData.domState.table
            });
        }
        
        // Check PBS data
        if (debugData.backupData?.pbsAnalysis) {
            const rootNamespaceData = [];
            debugData.backupData.pbsAnalysis.forEach(pbs => {
                pbs.datastores.forEach(ds => {
                    if (ds.namespaces.includes('root')) {
                        rootNamespaceData.push({
                            pbsInstance: pbs.name,
                            datastore: ds.name,
                            snapshotsInRoot: ds.snapshotCount
                        });
                    }
                });
            });
            analysis.rootNamespaceData = rootNamespaceData;
        }
        
        // Extract key namespace logs
        const namespaceLogs = debugData.consoleCapture?.filter(log => 
            log.args.some(arg => 
                typeof arg === 'string' && arg.includes('[DEBUG NAMESPACE]')
            )
        ) || [];
        
        analysis.keyLogs = namespaceLogs.slice(-20).map(log => ({
            time: log.time,
            message: log.args.join(' ')
        }));
        
        // Check for specific patterns
        const vmidsInNamespace = namespaceLogs.find(log => 
            log.args.some(arg => arg.includes('vmidsInNamespace:'))
        );
        
        if (vmidsInNamespace) {
            analysis.vmidsFound = vmidsInNamespace.args.join(' ');
        }
        
        // Save to file for inspection
        const debugFilePath = path.join(__dirname, '../debug', 'namespace-test-results.json');
        fs.writeFileSync(debugFilePath, JSON.stringify({
            ...debugData,
            analysis
        }, null, 2));
        
        console.log('[AUTO DEBUG] Analysis complete:', analysis.summary);
        console.log('[AUTO DEBUG] Issues found:', analysis.issues.length);
        
        res.json({
            success: true,
            analysis,
            debugFile: debugFilePath
        });
        
    } catch (error) {
        console.error('[AUTO DEBUG] Error processing test results:', error);
        res.status(500).json({ 
            success: false, 
            error: error.message 
        });
    }
});

module.exports = router;
