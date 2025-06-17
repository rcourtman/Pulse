// Improved caching strategy for backups.js

// Enhanced cache structure with fine-grained caching
let dataCache = {
    // Global cache invalidation
    lastStateHash: null,
    processedBackupData: null,
    
    // Fine-grained guest data caching
    guestCache: new Map(), // Map<guestKey, {hash, data, timestamp}>
    
    // Component-specific caches
    tasksByGuestCache: new Map(),
    snapshotsByGuestCache: new Map(),
    
    // Cache metadata
    cacheHits: 0,
    cacheMisses: 0,
    lastCleanup: Date.now()
};

// More efficient state hash generation using WeakMap for object hashing
const objectHashCache = new WeakMap();

function _generateStateHash(vmsData, containersData, pbsDataArray, pveBackups, namespaceFilter, pbsInstanceFilter) {
    // Use a more sophisticated hashing approach
    const components = [];
    
    // Basic counts
    components.push(vmsData.length);
    components.push(containersData.length);
    components.push(pbsDataArray.length);
    components.push(pveBackups?.backupTasks?.length || 0);
    components.push(pveBackups?.storageBackups?.length || 0);
    
    // Filter states
    components.push(namespaceFilter || 'all');
    components.push(pbsInstanceFilter || 'all');
    
    // Sample data for change detection (first and last items + random sample)
    const sampleData = (data, sampleSize = 3) => {
        if (!data || data.length === 0) return '';
        const samples = [];
        samples.push(data[0]?.vmid || data[0]?.id || '');
        if (data.length > 1) {
            samples.push(data[data.length - 1]?.vmid || data[data.length - 1]?.id || '');
        }
        if (data.length > 2) {
            const midIndex = Math.floor(data.length / 2);
            samples.push(data[midIndex]?.vmid || data[midIndex]?.id || '');
        }
        return samples.join(',');
    };
    
    components.push(sampleData(vmsData));
    components.push(sampleData(containersData));
    
    // Include modification timestamps if available
    const getLatestTimestamp = (data) => {
        if (!data || data.length === 0) return 0;
        return Math.max(...data.map(item => item.uptime || item.ctime || 0));
    };
    
    components.push(getLatestTimestamp(pveBackups?.backupTasks));
    components.push(getLatestTimestamp(pveBackups?.storageBackups));
    
    // Create a more robust hash
    const hashString = components.join('|');
    return hashString + '|' + simpleHash(hashString);
}

// Simple string hash function for better collision detection
function simpleHash(str) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        const char = str.charCodeAt(i);
        hash = ((hash << 5) - hash) + char;
        hash = hash & hash; // Convert to 32-bit integer
    }
    return hash.toString(36);
}

// Generate cache key for individual guest
function generateGuestCacheKey(guest, namespaceFilter, pbsInstanceFilter) {
    return `${guest.vmid}-${guest.type}-${guest.node || 'any'}-${namespaceFilter}-${pbsInstanceFilter}`;
}

// Smart cache invalidation
function invalidateGuestCache(guestId, type = null) {
    // Invalidate all entries for a specific guest
    for (const [key, value] of dataCache.guestCache.entries()) {
        if (key.startsWith(`${guestId}-`)) {
            if (!type || key.includes(`-${type}-`)) {
                dataCache.guestCache.delete(key);
            }
        }
    }
}

// Periodic cache cleanup (remove stale entries)
function cleanupCache(maxAge = 5 * 60 * 1000) { // 5 minutes default
    const now = Date.now();
    
    // Only run cleanup every minute at most
    if (now - dataCache.lastCleanup < 60000) return;
    
    let cleaned = 0;
    for (const [key, value] of dataCache.guestCache.entries()) {
        if (now - value.timestamp > maxAge) {
            dataCache.guestCache.delete(key);
            cleaned++;
        }
    }
    
    dataCache.lastCleanup = now;
    if (cleaned > 0) {
        console.log(`Cache cleanup: removed ${cleaned} stale entries`);
    }
}

// Enhanced _processBackupData with fine-grained caching
function _processBackupData(vmsData, containersData, pbsDataArray, pveBackups) {
    const allGuestsUnfiltered = [...vmsData, ...containersData];
    const initialDataReceived = allGuestsUnfiltered.length > 0;
    
    // Get filters
    const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
    const pbsInstanceFilterValue = PulseApp.state.get('backupsFilterPbsInstance') || 'all';
    
    // Check global cache first
    const currentHash = _generateStateHash(vmsData, containersData, pbsDataArray, pveBackups, namespaceFilter, pbsInstanceFilterValue);
    if (dataCache.lastStateHash === currentHash && dataCache.processedBackupData) {
        dataCache.cacheHits++;
        return dataCache.processedBackupData;
    }
    
    dataCache.cacheMisses++;
    
    // Run periodic cleanup
    cleanupCache();
    
    // Process data with partial caching
    const processingStart = performance.now();
    
    // Filter PBS instances
    const filteredPbsDataArray = pbsInstanceFilterValue === 'all'
        ? pbsDataArray
        : pbsDataArray.filter((_, index) => index.toString() === pbsInstanceFilterValue);
    
    // Process PBS backup tasks with caching
    let pbsBackupTasks = [];
    const tasksCacheKey = `tasks-${namespaceFilter}-${pbsInstanceFilterValue}`;
    
    if (dataCache.tasksByGuestCache.has(tasksCacheKey)) {
        pbsBackupTasks = dataCache.tasksByGuestCache.get(tasksCacheKey);
    } else {
        pbsBackupTasks = filteredPbsDataArray.flatMap(pbs => {
            return (pbs.backupTasks?.recentTasks || []).map(task => ({
                ...task,
                guestId: task.id?.split('/')[1] || task.guestId || null,
                guestTypePbs: task.id?.split('/')[0] || task.guestType || null,
                pbsInstanceName: pbs.pbsInstanceName,
                source: 'pbs'
            }));
        });
        
        // Apply namespace filtering if needed
        if (namespaceFilter !== 'all') {
            const guestNodeCombosInNamespace = getGuestNodeCombosInNamespace(filteredPbsDataArray, namespaceFilter);
            pbsBackupTasks = pbsBackupTasks.filter(task => {
                const taskKey = `${task.guestId}-${task.node || 'unknown'}`;
                return guestNodeCombosInNamespace.has(taskKey);
            });
        }
        
        dataCache.tasksByGuestCache.set(tasksCacheKey, pbsBackupTasks);
    }
    
    // Filter guests with partial caching
    let allGuests;
    if (namespaceFilter !== 'all') {
        const guestNodeCombosInNamespace = getGuestNodeCombosInNamespace(filteredPbsDataArray, namespaceFilter);
        allGuests = allGuestsUnfiltered.filter(guest => {
            const guestKey = `${guest.vmid}-${guest.node}`;
            return guestNodeCombosInNamespace.has(guestKey);
        });
    } else {
        allGuests = allGuestsUnfiltered;
    }
    
    // Process snapshots with caching
    const snapshotsCacheKey = `snapshots-${namespaceFilter}-${pbsInstanceFilterValue}`;
    let allSnapshots;
    
    if (dataCache.snapshotsByGuestCache.has(snapshotsCacheKey)) {
        allSnapshots = dataCache.snapshotsByGuestCache.get(snapshotsCacheKey);
    } else {
        let pbsSnapshots = filteredPbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    backupType: snap['backup-type'],
                    backupVMID: snap['backup-id'],
                    namespace: snap.namespace || 'root',
                    source: 'pbs'
                }))
            )
        );
        
        if (namespaceFilter !== 'all') {
            pbsSnapshots = pbsSnapshots.filter(snap => snap.namespace === namespaceFilter);
        }
        
        const pveStorageBackups = (pveBackups.storageBackups || []).map(backup => ({
            'backup-time': backup.ctime,
            backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
            backupVMID: backup.vmid,
            vmid: backup.vmid,
            size: backup.size,
            protected: backup.protected,
            storage: backup.storage,
            volid: backup.volid,
            node: backup.node,
            endpointId: backup.endpointId,
            source: 'pve'
        }));
        
        allSnapshots = [...pbsSnapshots, ...pveStorageBackups];
        dataCache.snapshotsByGuestCache.set(snapshotsCacheKey, allSnapshots);
    }
    
    // Build indexed data structures
    const tasksByGuest = new Map();
    const snapshotsByGuest = new Map();
    const pveBackupTasks = [];
    
    // Index tasks
    [...pbsBackupTasks, ...pveBackupTasks].forEach(task => {
        const nodeKey = task.node ? `-${task.node}` : (task.endpointId ? `-${task.endpointId}` : '');
        const key = `${task.guestId}-${task.guestTypePbs}${nodeKey}`;
        if (!tasksByGuest.has(key)) tasksByGuest.set(key, []);
        tasksByGuest.get(key).push(task);
    });
    
    // Index snapshots
    allSnapshots.forEach(snap => {
        if (snap.source === 'pbs') {
            const namespace = snap.namespace || 'root';
            const key = `${snap.backupVMID}-${snap.backupType}-${snap.pbsInstanceName}-${namespace}`;
            if (!snapshotsByGuest.has(key)) snapshotsByGuest.set(key, []);
            snapshotsByGuest.get(key).push(snap);
        } else {
            // PVE handling (unchanged)
            const endpointKey = snap.endpointId ? `-${snap.endpointId}` : '';
            const endpointGenericKey = `${snap.backupVMID}-${snap.backupType}${endpointKey}`;
            const nodeKey = snap.node ? `-${snap.node}` : '';
            const fullNodeSpecificKey = `${snap.backupVMID}-${snap.backupType}${endpointKey}${nodeKey}`;
            
            if (!snapshotsByGuest.has(endpointGenericKey)) snapshotsByGuest.set(endpointGenericKey, []);
            snapshotsByGuest.get(endpointGenericKey).push(snap);
            
            if (!snapshotsByGuest.has(fullNodeSpecificKey)) snapshotsByGuest.set(fullNodeSpecificKey, []);
            snapshotsByGuest.get(fullNodeSpecificKey).push(snap);
        }
    });
    
    // Pre-calculate time boundaries
    const now = new Date();
    now.setHours(0, 0, 0, 0);
    const dayBoundaries = [];
    for (let i = 6; i >= 0; i--) {
        const dayStart = new Date(now);
        dayStart.setDate(now.getDate() - i);
        const dayEnd = new Date(dayStart);
        dayEnd.setDate(dayStart.getDate() + 1);
        dayBoundaries.push({
            start: Math.floor(dayStart.getTime() / 1000),
            end: Math.floor(dayEnd.getTime() / 1000)
        });
    }
    
    const threeDaysAgo = Math.floor(Date.now() / 1000) - (3 * 24 * 60 * 60);
    const sevenDaysAgo = Math.floor(Date.now() / 1000) - (7 * 24 * 60 * 60);
    
    const processingTime = performance.now() - processingStart;
    console.log(`Backup data processing took ${processingTime.toFixed(2)}ms (cache hits: ${dataCache.cacheHits}, misses: ${dataCache.cacheMisses})`);
    
    const result = {
        allGuests,
        initialDataReceived,
        tasksByGuest,
        snapshotsByGuest,
        dayBoundaries,
        threeDaysAgo,
        sevenDaysAgo
    };
    
    // Update global cache
    dataCache.lastStateHash = currentHash;
    dataCache.processedBackupData = result;
    
    return result;
}

// Helper function to extract guest-node combinations
function getGuestNodeCombosInNamespace(filteredPbsDataArray, namespaceFilter) {
    const guestNodeCombosInNamespace = new Set();
    filteredPbsDataArray.forEach(pbsInstance => {
        (pbsInstance.datastores || []).forEach(ds => {
            (ds.snapshots || []).forEach(snap => {
                const snapNamespace = snap.namespace || 'root';
                if (snapNamespace === namespaceFilter) {
                    const guestId = snap['backup-id'];
                    const comment = snap.comment || '';
                    const commentParts = comment.split(', ');
                    if (commentParts.length >= 3 && guestId) {
                        const sourceNode = commentParts[1];
                        guestNodeCombosInNamespace.add(`${guestId}-${sourceNode}`);
                    }
                }
            });
        });
    });
    return guestNodeCombosInNamespace;
}

// Enhanced guest backup status determination with caching
function _determineGuestBackupStatus(guest, guestSnapshots, guestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo) {
    const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
    const pbsInstanceFilter = PulseApp.state.get('backupsFilterPbsInstance') || 'all';
    const guestCacheKey = generateGuestCacheKey(guest, namespaceFilter, pbsInstanceFilter);
    
    // Check guest-specific cache
    if (dataCache.guestCache.has(guestCacheKey)) {
        const cached = dataCache.guestCache.get(guestCacheKey);
        // Use cached data if it's less than 30 seconds old
        if (Date.now() - cached.timestamp < 30000) {
            return cached.data;
        }
    }
    
    // Perform the actual status determination
    const result = _determineGuestBackupStatusUncached(guest, guestSnapshots, guestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo);
    
    // Cache the result
    dataCache.guestCache.set(guestCacheKey, {
        hash: guestCacheKey,
        data: result,
        timestamp: Date.now()
    });
    
    return result;
}

// Export cache statistics for monitoring
function getCacheStatistics() {
    return {
        cacheHits: dataCache.cacheHits,
        cacheMisses: dataCache.cacheMisses,
        hitRate: dataCache.cacheHits / (dataCache.cacheHits + dataCache.cacheMisses) || 0,
        guestCacheSize: dataCache.guestCache.size,
        tasksCacheSize: dataCache.tasksByGuestCache.size,
        snapshotsCacheSize: dataCache.snapshotsByGuestCache.size,
        lastCleanup: new Date(dataCache.lastCleanup).toISOString()
    };
}

// Reset cache (useful for testing or manual refresh)
function resetCache() {
    dataCache = {
        lastStateHash: null,
        processedBackupData: null,
        guestCache: new Map(),
        tasksByGuestCache: new Map(),
        snapshotsByGuestCache: new Map(),
        cacheHits: 0,
        cacheMisses: 0,
        lastCleanup: Date.now()
    };
    console.log('Backup data cache has been reset');
}

// Preload cache for specific guests (useful for pagination)
function preloadGuestCache(guests, namespaceFilter, pbsInstanceFilter) {
    const promises = guests.map(guest => {
        return new Promise((resolve) => {
            const cacheKey = generateGuestCacheKey(guest, namespaceFilter, pbsInstanceFilter);
            if (!dataCache.guestCache.has(cacheKey)) {
                // Simulate async loading - in real implementation this would call the actual determination function
                setTimeout(() => {
                    console.log(`Preloading cache for guest ${guest.vmid}`);
                    resolve();
                }, 0);
            } else {
                resolve();
            }
        });
    });
    
    return Promise.all(promises);
}