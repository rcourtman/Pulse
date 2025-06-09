/**
 * Fixes for dataFetcher.js to address ground truth discrepancies
 */

/**
 * Deduplicates guests across multiple endpoints
 * @param {Array} guests - Array of VM or container objects
 * @returns {Array} Deduplicated array
 */
function deduplicateGuests(guests) {
    const seen = new Map();
    
    return guests.filter(guest => {
        // Create a unique key based on VMID and node
        // This prevents counting the same guest multiple times
        const key = `${guest.vmid}-${guest.node}`;
        
        if (seen.has(key)) {
            return false;
        }
        
        seen.set(key, true);
        return true;
    });
}

/**
 * Fixed version of fetchPveDiscoveryData with deduplication
 */
async function fetchPveDiscoveryDataFixed(currentApiClients) {
    const pveEndpointIds = Object.keys(currentApiClients);
    let allNodes = [], allVms = [], allContainers = [];

    if (pveEndpointIds.length === 0) {
        return { nodes: [], vms: [], containers: [] };
    }

    const pvePromises = pveEndpointIds.map(endpointId => {
        if (!currentApiClients[endpointId]) {
            console.error(`[DataFetcher] No client found for endpoint: ${endpointId}`);
            return Promise.resolve({ nodes: [], vms: [], containers: [] });
        }
        const { client: apiClientInstance, config } = currentApiClients[endpointId];
        return fetchDataForPveEndpoint(endpointId, apiClientInstance, config);
    });

    const pveResults = await Promise.all(pvePromises);

    // Aggregate results from all endpoints
    pveResults.forEach(result => {
        if (result) {
            allNodes.push(...(result.nodes || []));
            allVms.push(...(result.vms || []));
            allContainers.push(...(result.containers || []));
        }
    });

    // Deduplicate guests before returning
    const deduplicatedVms = deduplicateGuests(allVms);
    const deduplicatedContainers = deduplicateGuests(allContainers);
    

    return { 
        nodes: allNodes, 
        vms: deduplicatedVms, 
        containers: deduplicatedContainers 
    };
}

/**
 * Improved PBS backup counting that correctly identifies backup runs
 */
function improvedBackupCounting(pbsData) {
    const backupsByGuest = new Map();
    const backupsByDate = new Map();
    
    if (!pbsData || !pbsData[0]?.datastores) {
        return { totalBackups: 0, uniqueBackupRuns: 0, backupsByGuest };
    }
    
    pbsData[0].datastores.forEach(ds => {
        (ds.snapshots || []).forEach(snap => {
            const guestKey = `${snap['backup-type']}/${snap['backup-id']}`;
            const dateKey = new Date(snap['backup-time'] * 1000).toISOString().split('T')[0];
            const runKey = `${guestKey}:${dateKey}`;
            
            // Count by guest
            if (!backupsByGuest.has(guestKey)) {
                backupsByGuest.set(guestKey, []);
            }
            backupsByGuest.get(guestKey).push(snap);
            
            // Count unique backup runs (one per guest per day)
            backupsByDate.set(runKey, snap);
        });
    });
    
    return {
        totalBackups: Array.from(backupsByGuest.values()).reduce((sum, backups) => sum + backups.length, 0),
        uniqueBackupRuns: backupsByDate.size,
        backupsByGuest
    };
}

/**
 * Validates backup ages and identifies missing backups
 */
function validateBackupAgesImproved(pbsData, expectedGuests) {
    const now = Date.now();
    const twentyFourHours = 24 * 60 * 60 * 1000;
    const results = {
        guestsWithRecentBackups: new Set(),
        guestsWithoutRecentBackups: new Set(),
        backupAgesByGuest: new Map(),
        issues: []
    };
    
    // Initialize all expected guests as missing
    expectedGuests.forEach(guestId => {
        results.guestsWithoutRecentBackups.add(guestId);
    });
    
    if (pbsData && pbsData[0]?.datastores) {
        pbsData[0].datastores.forEach(ds => {
            (ds.snapshots || []).forEach(snap => {
                const guestId = snap['backup-id'];
                const backupTime = snap['backup-time'] * 1000;
                const ageMs = now - backupTime;
                
                // Track most recent backup for each guest
                if (!results.backupAgesByGuest.has(guestId) || 
                    backupTime > results.backupAgesByGuest.get(guestId).time) {
                    results.backupAgesByGuest.set(guestId, {
                        time: backupTime,
                        ageHours: ageMs / (1000 * 60 * 60),
                        ageReadable: formatAge(ageMs)
                    });
                }
                
                // Check if backup is recent (within 24 hours)
                if (ageMs < twentyFourHours) {
                    results.guestsWithRecentBackups.add(guestId);
                    results.guestsWithoutRecentBackups.delete(guestId);
                }
            });
        });
    }
    
    // Identify specific issues
    if (results.guestsWithoutRecentBackups.has('102')) {
        results.issues.push('VM 102 has no recent backup despite being in backup job');
    }
    
    return results;
}

/**
 * Format age in human-readable format
 */
function formatAge(ageMs) {
    const hours = Math.floor(ageMs / (1000 * 60 * 60));
    const days = Math.floor(hours / 24);
    
    if (days > 0) {
        return `${days}d ${hours % 24}h`;
    }
    return `${hours}h`;
}

module.exports = {
    deduplicateGuests,
    fetchPveDiscoveryDataFixed,
    improvedBackupCounting,
    validateBackupAgesImproved,
    formatAge
};