/**
 * Backup Processors - Separate processing pipelines for each backup type
 * This module handles the processing and matching of backups to guests
 */

const { PBSBackup, PVEBackup, Snapshot, Guest } = require('../models/backupModels');

/**
 * PBS Backup Processor
 * Handles processing of Proxmox Backup Server backups
 */
class PBSBackupProcessor {
    constructor() {
        this.backups = [];
        this.backupsByNamespace = new Map();
    }
    
    /**
     * Process raw PBS data into PBSBackup objects
     */
    processRawData(pbsInstances) {
        this.backups = [];
        this.backupsByNamespace.clear();
        
        console.log(`[PBSProcessor] Processing ${pbsInstances.length} PBS instances`);
        
        pbsInstances.forEach(pbsInstance => {
            if (!pbsInstance.datastores) return;
            
            pbsInstance.datastores.forEach(datastore => {
                if (!datastore.snapshots) return;
                
                console.log(`[PBSProcessor] Processing datastore ${datastore.name} with ${datastore.snapshots.length} snapshots`);
                
                datastore.snapshots.forEach(snapshot => {
                    const backup = new PBSBackup({
                        ...snapshot,
                        pbsInstanceName: pbsInstance.pbsInstanceName,
                        datastoreName: datastore.name
                    });
                    
                    this.backups.push(backup);
                    
                    // Index by namespace
                    const namespace = backup.namespace;
                    if (!this.backupsByNamespace.has(namespace)) {
                        this.backupsByNamespace.set(namespace, []);
                    }
                    this.backupsByNamespace.get(namespace).push(backup);
                    
                    // Log namespace assignment
                    const displayNamespace = namespace || 'root';
                    console.log(`[PBSProcessor] Backup ${backup.backupType}/${backup.backupId} assigned to namespace '${displayNamespace}' (raw namespace: '${snapshot.namespace || '(empty)'}')`);
                });
            });
        });
        
        // Log namespace summary
        console.log(`[PBSProcessor] Namespace summary:`);
        this.backupsByNamespace.forEach((backups, namespace) => {
            const displayNamespace = namespace || 'root';
            console.log(`[PBSProcessor]   - Namespace '${displayNamespace}': ${backups.length} backups`);
            // Log first few backups in each namespace
            backups.slice(0, 3).forEach(backup => {
                console.log(`[PBSProcessor]     -> ${backup.backupType}/${backup.backupId} owner: ${backup.owner}`);
            });
        });
        
        return this.backups;
    }
    
    /**
     * Get backups filtered by namespace
     */
    getBackupsByNamespace(namespace) {
        if (namespace === 'all') {
            console.log(`[PBSProcessor] getBackupsByNamespace: returning all ${this.backups.length} backups`);
            return this.backups;
        }
        
        // Handle both 'root' and empty string as root namespace
        let lookupNamespace = namespace;
        if (namespace === 'root') {
            lookupNamespace = '';
        }
        const backupsInNamespace = this.backupsByNamespace.get(lookupNamespace) || [];
        
        const displayNamespace = lookupNamespace || 'root';
        console.log(`[PBSProcessor] getBackupsByNamespace: requested='${namespace}', lookup='${lookupNamespace}', display='${displayNamespace}', found ${backupsInNamespace.length} backups`);
        
        // If specific namespace requested, log details
        if (normalizedNamespace === 'pimox') {
            console.log(`[PBSProcessor] Details for 'pimox' namespace:`);
            if (backupsInNamespace.length === 0) {
                console.log(`[PBSProcessor]   - No backups found in 'pimox' namespace`);
                console.log(`[PBSProcessor]   - Available namespaces: ${Array.from(this.backupsByNamespace.keys()).join(', ')}`);
            } else {
                backupsInNamespace.forEach(backup => {
                    console.log(`[PBSProcessor]   - ${backup.backupType}/${backup.backupId} owner: ${backup.owner} ownerToken: ${backup.ownerToken}`);
                });
            }
        }
        
        return backupsInNamespace;
    }
    
    /**
     * Get all available namespaces
     */
    getAvailableNamespaces() {
        const namespaces = Array.from(this.backupsByNamespace.keys());
        return namespaces.sort();
    }
    
    /**
     * Match PBS backups to guests
     */
    matchBackupsToGuests(guests, namespaceFilter = 'all') {
        const filteredBackups = this.getBackupsByNamespace(namespaceFilter);
        
        console.log(`[PBSProcessor] Matching ${filteredBackups.length} backups to ${guests.length} guests (namespace filter: '${namespaceFilter}')`);
        
        let matchedCount = 0;
        let unmatchedCount = 0;
        
        filteredBackups.forEach(backup => {
            // Try to find matching guest
            let matchFound = false;
            for (const guest of guests) {
                if (backup.matchesGuest(guest)) {
                    console.log(`[PBSProcessor] Matched backup ${backup.backupType}/${backup.backupId} (namespace: ${backup.namespace}, owner: ${backup.owner}) to guest ${guest.compositeKey}`);
                    guest.addBackupIfMatches(backup);
                    matchFound = true;
                    matchedCount++;
                    break; // Only match to first guest (in case of duplicates)
                }
            }
            
            if (!matchFound) {
                unmatchedCount++;
                console.log(`[PBSProcessor] Unmatched backup: ${backup.backupType}/${backup.backupId} in namespace '${backup.namespace}' with owner '${backup.owner}' (ownerToken: '${backup.ownerToken}')`);
            }
        });
        
        console.log(`[PBSProcessor] Matching summary: ${matchedCount} matched, ${unmatchedCount} unmatched`);
    }
}

/**
 * PVE Backup Processor
 * Handles processing of Proxmox VE storage backups
 */
class PVEBackupProcessor {
    constructor() {
        this.backups = [];
    }
    
    /**
     * Process raw PVE storage backup data
     */
    processRawData(storageBackups) {
        this.backups = storageBackups.map(backup => new PVEBackup(backup));
        return this.backups;
    }
    
    /**
     * Match PVE backups to guests
     * PVE backups don't have namespaces, so they're always included
     */
    matchBackupsToGuests(guests) {
        this.backups.forEach(backup => {
            const matched = guests.some(guest => guest.addBackupIfMatches(backup));
            
            if (!matched) {
                console.log(`[PVEProcessor] Unmatched backup: ${backup.filename} for VMID ${backup.vmid} on node ${backup.node}`);
            }
        });
    }
}

/**
 * Snapshot Processor
 * Handles processing of VM/CT snapshots
 */
class SnapshotProcessor {
    constructor() {
        this.snapshots = [];
    }
    
    /**
     * Process raw snapshot data
     */
    processRawData(guestSnapshots) {
        this.snapshots = guestSnapshots.map(snapshot => new Snapshot(snapshot));
        return this.snapshots;
    }
    
    /**
     * Match snapshots to guests
     * Snapshots are always tied to specific guests
     */
    matchSnapshotsToGuests(guests) {
        this.snapshots.forEach(snapshot => {
            const matched = guests.some(guest => guest.addBackupIfMatches(snapshot));
            
            if (!matched) {
                console.log(`[SnapshotProcessor] Unmatched snapshot: ${snapshot.name} for VMID ${snapshot.vmid} on node ${snapshot.node}`);
            }
        });
    }
}

/**
 * Guest Processor
 * Handles creation and management of Guest objects
 */
class GuestProcessor {
    constructor() {
        this.guests = [];
        this.guestsByCompositeKey = new Map();
    }
    
    /**
     * Process raw VM/Container data into Guest objects
     */
    processRawData(vms, containers) {
        this.guests = [];
        this.guestsByCompositeKey.clear();
        
        // Process VMs
        vms.forEach(vm => {
            const guest = new Guest({
                ...vm,
                type: 'qemu'
            });
            this.guests.push(guest);
            this.guestsByCompositeKey.set(guest.compositeKey, guest);
        });
        
        // Process Containers
        containers.forEach(ct => {
            const guest = new Guest({
                ...ct,
                type: 'lxc'
            });
            this.guests.push(guest);
            this.guestsByCompositeKey.set(guest.compositeKey, guest);
        });
        
        return this.guests;
    }
    
    /**
     * Get guest by composite key
     */
    getGuestByKey(compositeKey) {
        return this.guestsByCompositeKey.get(compositeKey);
    }
    
    /**
     * Filter guests by namespace
     * Only include guests that have PBS backups in the specified namespace
     */
    filterGuestsByNamespace(namespace) {
        if (namespace === 'all') {
            console.log(`[GuestProcessor] filterGuestsByNamespace: returning all ${this.guests.length} guests`);
            return this.guests;
        }
        
        // Handle both 'root' and empty string as root namespace
        let lookupNamespace = namespace;
        if (namespace === 'root') {
            lookupNamespace = '';
        }
        const displayNamespace = lookupNamespace || 'root';
        console.log(`[GuestProcessor] filterGuestsByNamespace: filtering for namespace '${namespace}' (lookup: '${lookupNamespace}', display: '${displayNamespace}')`);
        
        const filteredGuests = this.guests.filter(guest => {
            const guestNamespaces = guest.getNamespaces();
            const hasNamespace = guestNamespaces.includes(lookupNamespace);
            
            if (lookupNamespace === 'pimox' && guestNamespaces.length > 0) {
                console.log(`[GuestProcessor]   - Guest ${guest.compositeKey} has namespaces: [${guestNamespaces.join(', ')}] - matches: ${hasNamespace}`);
            }
            
            return hasNamespace;
        });
        
        console.log(`[GuestProcessor] filterGuestsByNamespace: found ${filteredGuests.length} guests with namespace '${displayNamespace}'`);
        
        return filteredGuests;
    }
}

/**
 * Main Backup Processing Coordinator
 * Orchestrates all backup processors
 */
class BackupProcessingCoordinator {
    constructor() {
        this.guestProcessor = new GuestProcessor();
        this.pbsProcessor = new PBSBackupProcessor();
        this.pveProcessor = new PVEBackupProcessor();
        this.snapshotProcessor = new SnapshotProcessor();
    }
    
    /**
     * Process all backup data and match to guests
     */
    processAllBackupData(data, filters = {}) {
        const { namespaceFilter = 'all', backupTypeFilter = 'all' } = filters;
        
        // 1. Process guests first
        const allGuests = this.guestProcessor.processRawData(data.vms, data.containers);
        console.log(`[Coordinator] Processed ${allGuests.length} guests`);
        
        // Debug: Log sample guest
        if (allGuests.length > 0) {
            console.log(`[Coordinator] Sample guest:`, {
                vmid: allGuests[0].vmid,
                name: allGuests[0].name,
                node: allGuests[0].node,
                endpointId: allGuests[0].endpointId,
                compositeKey: allGuests[0].compositeKey
            });
        }
        
        // 2. Process each backup type
        if (backupTypeFilter === 'all' || backupTypeFilter === 'pbs') {
            const pbsBackups = this.pbsProcessor.processRawData(data.pbsInstances || []);
            console.log(`[Coordinator] Processed ${pbsBackups.length} PBS backups`);
            
            // Match PBS backups to guests (with namespace filter)
            this.pbsProcessor.matchBackupsToGuests(allGuests, namespaceFilter);
        }
        
        if (backupTypeFilter === 'all' || backupTypeFilter === 'pve') {
            const pveBackups = this.pveProcessor.processRawData(data.storageBackups || []);
            console.log(`[Coordinator] Processed ${pveBackups.length} PVE backups`);
            
            // Match PVE backups to guests (no namespace filter)
            this.pveProcessor.matchBackupsToGuests(allGuests);
        }
        
        if (backupTypeFilter === 'all' || backupTypeFilter === 'snapshots') {
            const snapshots = this.snapshotProcessor.processRawData(data.guestSnapshots || []);
            console.log(`[Coordinator] Processed ${snapshots.length} snapshots`);
            
            // Match snapshots to guests
            this.snapshotProcessor.matchSnapshotsToGuests(allGuests);
        }
        
        // 3. Filter guests based on namespace (only if PBS is included)
        let filteredGuests = allGuests;
        if (namespaceFilter !== 'all' && (backupTypeFilter === 'all' || backupTypeFilter === 'pbs')) {
            console.log(`[Coordinator] Applying namespace filter '${namespaceFilter}' (backup type filter: '${backupTypeFilter}')`);
            filteredGuests = this.guestProcessor.filterGuestsByNamespace(namespaceFilter);
            console.log(`[Coordinator] Filtered to ${filteredGuests.length} guests in namespace '${namespaceFilter}'`);
            
            // For debugging pimox namespace
            if (namespaceFilter === 'pimox' && filteredGuests.length === 0) {
                console.log(`[Coordinator] WARNING: No guests found for 'pimox' namespace!`);
                
                // Check if any PBS backups exist in pimox namespace
                const pimoxBackups = this.pbsProcessor.getBackupsByNamespace('pimox');
                console.log(`[Coordinator] PBS backups in 'pimox' namespace: ${pimoxBackups.length}`);
                
                if (pimoxBackups.length > 0) {
                    console.log(`[Coordinator] Sample pimox backups:`);
                    pimoxBackups.slice(0, 5).forEach(backup => {
                        console.log(`[Coordinator]   - ${backup.backupType}/${backup.backupId} owner: ${backup.owner} ownerToken: ${backup.ownerToken}`);
                        
                        // Check if any guest matches this VMID
                        const matchingGuests = allGuests.filter(g => String(g.vmid) === String(backup.backupId));
                        console.log(`[Coordinator]     -> Found ${matchingGuests.length} guests with VMID ${backup.backupId}`);
                        matchingGuests.forEach(g => {
                            console.log(`[Coordinator]        - ${g.compositeKey} (endpointId: ${g.endpointId})`);
                        });
                    });
                }
            }
        }
        
        // 4. Return processed data
        return {
            guests: filteredGuests,
            availableNamespaces: this.pbsProcessor.getAvailableNamespaces(),
            stats: {
                totalGuests: allGuests.length,
                filteredGuests: filteredGuests.length,
                totalPBSBackups: this.pbsProcessor.backups.length,
                totalPVEBackups: this.pveProcessor.backups.length,
                totalSnapshots: this.snapshotProcessor.snapshots.length
            }
        };
    }
    
    /**
     * Get backup summary for a specific guest
     */
    getGuestBackupSummary(compositeKey) {
        const guest = this.guestProcessor.getGuestByKey(compositeKey);
        if (!guest) return null;
        
        return {
            guest: {
                compositeKey: guest.compositeKey,
                vmid: guest.vmid,
                name: guest.name,
                type: guest.type,
                node: guest.node,
                endpointId: guest.endpointId
            },
            backups: {
                pbs: guest.pbsBackups.map(b => ({
                    backupTime: b.backupTime,
                    size: b.size,
                    namespace: b.namespace,
                    protected: b.protected,
                    comment: b.comment
                })),
                pve: guest.pveBackups.map(b => ({
                    ctime: b.ctime,
                    size: b.size,
                    storage: b.storage,
                    filename: b.filename,
                    protected: b.protected
                })),
                snapshots: guest.snapshots.map(s => ({
                    name: s.name,
                    snaptime: s.snaptime,
                    description: s.description,
                    parent: s.parent
                }))
            },
            counts: guest.getBackupCounts(),
            latestBackupTime: guest.getLatestBackupTime(),
            namespaces: guest.getNamespaces()
        };
    }
}

module.exports = {
    PBSBackupProcessor,
    PVEBackupProcessor,
    SnapshotProcessor,
    GuestProcessor,
    BackupProcessingCoordinator
};