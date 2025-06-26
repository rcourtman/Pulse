// Unified backup filtering pipeline
PulseApp.utils = PulseApp.utils || {};

PulseApp.utils.backupFilterPipeline = (() => {
    
    // Define the backup data structure that all components will use
    // Each backup entry represents a unique guest-namespace combination
    class BackupEntry {
        constructor(guest, namespace = 'root') {
            this.vmid = String(guest.vmid);
            this.name = guest.name;
            this.type = guest.type === 'qemu' ? 'VM' : 'CT';
            this.node = guest.node;
            this.endpointId = guest.endpointId;
            this.uniqueKey = `${this.vmid}-${this.node || this.endpointId || 'unknown'}`;
            
            // Namespace info
            this.namespace = namespace;
            this.namespaceKey = `${this.uniqueKey}-${namespace}`;
            
            // Backup counts by type
            this.pbsCount = 0;
            this.pveCount = 0;
            this.snapshotCount = 0;
            
            // Backup details
            this.backups = [];
            this.latestBackupTime = null;
            this.oldestBackupTime = null;
            
            // Status
            this.hasFailures = false;
            this.recentFailures = 0;
        }
        
        get totalBackups() {
            return this.pbsCount + this.pveCount + this.snapshotCount;
        }
        
        get backupAge() {
            if (!this.latestBackupTime) return Infinity;
            return (Date.now() / 1000 - this.latestBackupTime) / 86400; // days
        }
        
        get types() {
            const types = [];
            if (this.pbsCount > 0) types.push('pbsSnapshots');
            if (this.pveCount > 0) types.push('pveBackups');
            if (this.snapshotCount > 0) types.push('vmSnapshots');
            return types;
        }
        
        addBackup(backup) {
            this.backups.push(backup);
            
            // Update counts
            if (backup.source === 'pbs') this.pbsCount++;
            else if (backup.source === 'pve') this.pveCount++;
            else if (backup.source === 'snapshot') this.snapshotCount++;
            
            // Update times
            const timestamp = PulseApp.utils.getBackupTimestamp(backup);
            if (timestamp) {
                if (!this.latestBackupTime || timestamp > this.latestBackupTime) {
                    this.latestBackupTime = timestamp;
                }
                if (!this.oldestBackupTime || timestamp < this.oldestBackupTime) {
                    this.oldestBackupTime = timestamp;
                }
            }
        }
    }
    
    // Main filtering pipeline
    function applyFilters(rawData, filters) {
        const {
            search = '',
            guestType = 'all',
            backupType = 'all',
            namespace = 'all',
            pbsInstance = 'all',
            healthStatus = 'all',
            failuresOnly = false
        } = filters;
        
        // Step 1: Process raw data into BackupEntry objects
        const entries = processRawData(rawData, namespace, pbsInstance);
        
        // Step 2: Apply filters in order of efficiency
        let filtered = entries;
        
        if (backupType !== 'all') {
            filtered = filtered.filter(entry => {
                switch (backupType) {
                    case 'pbs': return entry.pbsCount > 0;
                    case 'pve': return entry.pveCount > 0;
                    case 'snapshots': return entry.snapshotCount > 0;
                    default: return true;
                }
            });
        }
        
        
        // Filter by guest type
        if (guestType !== 'all') {
            const targetType = guestType === 'vm' ? 'VM' : 'CT';
            filtered = filtered.filter(entry => entry.type === targetType);
        }
        
        // Filter by health status
        if (healthStatus !== 'all') {
            filtered = filtered.filter(entry => {
                const age = entry.backupAge;
                switch (healthStatus) {
                    case 'good': return age <= 1;
                    case 'warning': return age > 1 && age <= 7;
                    case 'critical': return age > 7;
                    case 'none': return entry.totalBackups === 0;
                    default: return true;
                }
            });
        }
        
        // Filter by failures
        if (failuresOnly) {
            filtered = filtered.filter(entry => entry.hasFailures || entry.recentFailures > 0);
        }
        
        if (search) {
            const searchTerms = search.toLowerCase().split(',').map(s => s.trim());
            filtered = filtered.filter(entry => {
                const searchTarget = `${entry.vmid} ${entry.name} ${entry.node || ''} ${entry.namespace}`.toLowerCase();
                return searchTerms.some(term => searchTarget.includes(term));
            });
        }
        
        return filtered;
    }
    
    // Process raw data into unified structure
    function processRawData(rawData, namespaceFilter, pbsInstanceFilter) {
        const { guests, pbsSnapshots, pveBackups, vmSnapshots, backupTasks } = rawData;
        const entriesMap = new Map(); // namespaceKey -> BackupEntry
        
        // Process each guest
        guests.forEach(guest => {
            // Determine which namespaces this guest has backups in
            const namespaces = new Set();
            
            // Check PBS snapshots for namespaces
            if (pbsSnapshots) {
                pbsSnapshots.forEach(snap => {
                    const snapVmid = String(snap['backup-id'] || snap.backupVMID || snap.vmid);
                    if (snapVmid === String(guest.vmid)) {
                        // Verify this snapshot belongs to this specific guest instance
                        if (isSnapshotForGuest(snap, guest)) {
                            const ns = snap.namespace || 'root';
                            namespaces.add(ns);
                        }
                    }
                });
            }
            
            // If no PBS backups, but has other backups, add to root namespace
            if (namespaces.size === 0 && (hasGuestPveBackups(guest, pveBackups) || hasGuestSnapshots(guest, vmSnapshots))) {
                namespaces.add('root');
            }
            
            // Skip if namespace filter is active and guest has no backups in that namespace
            if (namespaceFilter !== 'all' && !namespaces.has(namespaceFilter)) {
                return;
            }
            
            // Create entries for each namespace
            const namespacesToProcess = namespaceFilter === 'all' ? Array.from(namespaces) : [namespaceFilter];
            
            namespacesToProcess.forEach(ns => {
                const entry = new BackupEntry(guest, ns);
                
                // Add PBS backups for this namespace
                if (pbsSnapshots) {
                    pbsSnapshots.forEach(snap => {
                        const snapVmid = String(snap['backup-id'] || snap.backupVMID || snap.vmid);
                        if (snapVmid === String(guest.vmid) && 
                            (snap.namespace || 'root') === ns &&
                            isSnapshotForGuest(snap, guest)) {
                            entry.addBackup({ ...snap, source: 'pbs' });
                        }
                    });
                }
                
                if (pveBackups) {
                    pveBackups.forEach(backup => {
                        if (isBackupForGuest(backup, guest)) {
                            entry.addBackup({ ...backup, source: 'pve' });
                        }
                    });
                }
                
                if (vmSnapshots) {
                    vmSnapshots.forEach(snap => {
                        if (isSnapshotForGuest(snap, guest)) {
                            entry.addBackup({ ...snap, source: 'snapshot' });
                        }
                    });
                }
                
                // Process backup tasks for failure info
                if (backupTasks) {
                    backupTasks.forEach(task => {
                        if (isTaskForGuest(task, guest) && task.status === 'failed') {
                            entry.hasFailures = true;
                            entry.recentFailures++;
                        }
                    });
                }
                
                // Only add entry if it has backups or we're showing all
                if (entry.totalBackups > 0 || namespaceFilter === 'all') {
                    entriesMap.set(entry.namespaceKey, entry);
                }
            });
        });
        
        return Array.from(entriesMap.values());
    }
    
    // Helper functions
    function isSnapshotForGuest(snapshot, guest) {
        // PBS snapshot validation
        if (snapshot.owner) {
            const ownerParts = snapshot.owner.split('!');
            if (ownerParts.length > 1 && guest.node) {
                const ownerNode = ownerParts[1].toLowerCase();
                if (ownerNode !== guest.node.toLowerCase()) {
                    return false;
                }
            }
        }
        
        // Type validation
        const snapType = snapshot.backupType || snapshot['backup-type'] || '';
        const expectedType = guest.type === 'qemu' ? 'vm' : 'ct';
        if (snapType && snapType !== expectedType) {
            return false;
        }
        
        return true;
    }
    
    function isBackupForGuest(backup, guest) {
        const backupVmid = String(backup.vmid || backup['backup-id']);
        return backupVmid === String(guest.vmid) && 
               (!backup.node || backup.node === guest.node);
    }
    
    function isTaskForGuest(task, guest) {
        const taskVmid = String(task.vmid || task.guestId);
        return taskVmid === String(guest.vmid) &&
               (!task.node || task.node === guest.node);
    }
    
    function hasGuestPveBackups(guest, pveBackups) {
        if (!pveBackups) return false;
        return pveBackups.some(backup => isBackupForGuest(backup, guest));
    }
    
    function hasGuestSnapshots(guest, vmSnapshots) {
        if (!vmSnapshots) return false;
        return vmSnapshots.some(snap => isSnapshotForGuest(snap, guest));
    }
    
    // Public API
    return {
        applyFilters,
        BackupEntry
    };
})();