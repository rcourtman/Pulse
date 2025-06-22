// Backup data aggregation utilities
PulseApp.utils = PulseApp.utils || {};

PulseApp.utils.backupDataAggregator = (() => {
    
    // Aggregate backup entries by date for calendar view
    function aggregateByDate(backupEntries) {
        const dateMap = new Map(); // date -> { guests: Set, stats: {...} }
        
        backupEntries.forEach(entry => {
            entry.backups.forEach(backup => {
                const timestamp = backup.timestamp || backup['backup-time'] || backup.ctime || backup.snaptime;
                if (!timestamp) return;
                
                const date = new Date(timestamp * 1000);
                const dateKey = date.toISOString().split('T')[0]; // YYYY-MM-DD
                
                if (!dateMap.has(dateKey)) {
                    dateMap.set(dateKey, {
                        guests: new Set(),
                        backupCount: 0,
                        pbsCount: 0,
                        pveCount: 0,
                        snapshotCount: 0,
                        hasFailures: false
                    });
                }
                
                const dayData = dateMap.get(dateKey);
                dayData.guests.add(entry.uniqueKey);
                dayData.backupCount++;
                
                // Count by type
                if (backup.source === 'pbs') dayData.pbsCount++;
                else if (backup.source === 'pve') dayData.pveCount++;
                else if (backup.source === 'snapshot') dayData.snapshotCount++;
                
                // Check for failures
                if (entry.hasFailures) dayData.hasFailures = true;
            });
        });
        
        // Convert to final format
        const result = {};
        dateMap.forEach((data, date) => {
            result[date] = {
                guestCount: data.guests.size,
                backupCount: data.backupCount,
                pbsCount: data.pbsCount,
                pveCount: data.pveCount,
                snapshotCount: data.snapshotCount,
                hasFailures: data.hasFailures
            };
        });
        
        return result;
    }
    
    // Get backup entries for a specific date
    function getEntriesForDate(backupEntries, dateStr) {
        const result = [];
        
        backupEntries.forEach(entry => {
            const hasBackupOnDate = entry.backups.some(backup => {
                const timestamp = backup.timestamp || backup['backup-time'] || backup.ctime || backup.snaptime;
                if (!timestamp) return false;
                
                const date = new Date(timestamp * 1000);
                const backupDateStr = date.toISOString().split('T')[0];
                return backupDateStr === dateStr;
            });
            
            if (hasBackupOnDate) {
                result.push(entry);
            }
        });
        
        return result;
    }
    
    // Group entries by namespace for display
    function groupByNamespace(backupEntries) {
        const namespaceMap = new Map();
        
        backupEntries.forEach(entry => {
            const ns = entry.namespace;
            if (!namespaceMap.has(ns)) {
                namespaceMap.set(ns, []);
            }
            namespaceMap.get(ns).push(entry);
        });
        
        // Sort namespaces with root first
        const sortedNamespaces = Array.from(namespaceMap.keys()).sort((a, b) => {
            if (a === 'root') return -1;
            if (b === 'root') return 1;
            return a.localeCompare(b);
        });
        
        const result = [];
        sortedNamespaces.forEach(ns => {
            result.push({
                namespace: ns,
                entries: namespaceMap.get(ns)
            });
        });
        
        return result;
    }
    
    // Calculate statistics for backup entries
    function calculateStats(backupEntries) {
        const stats = {
            totalGuests: new Set(),
            totalEntries: backupEntries.length,
            pbsGuests: new Set(),
            pveGuests: new Set(),
            snapshotGuests: new Set(),
            healthyGuests: new Set(),
            warningGuests: new Set(),
            criticalGuests: new Set(),
            failedGuests: new Set(),
            namespaces: new Set(),
            totalBackups: 0,
            backupsToday: 0,
            backupsThisWeek: 0,
            backupsThisMonth: 0
        };
        
        const now = Date.now() / 1000;
        const today = new Date().setHours(0, 0, 0, 0) / 1000;
        const weekAgo = today - (7 * 86400);
        const monthAgo = today - (30 * 86400);
        
        backupEntries.forEach(entry => {
            stats.totalGuests.add(entry.uniqueKey);
            stats.namespaces.add(entry.namespace);
            
            // Count by type
            if (entry.pbsCount > 0) stats.pbsGuests.add(entry.uniqueKey);
            if (entry.pveCount > 0) stats.pveGuests.add(entry.uniqueKey);
            if (entry.snapshotCount > 0) stats.snapshotGuests.add(entry.uniqueKey);
            
            // Health status
            const age = entry.backupAge;
            if (age <= 1) stats.healthyGuests.add(entry.uniqueKey);
            else if (age <= 7) stats.warningGuests.add(entry.uniqueKey);
            else stats.criticalGuests.add(entry.uniqueKey);
            
            if (entry.hasFailures) stats.failedGuests.add(entry.uniqueKey);
            
            // Count backups by time period
            entry.backups.forEach(backup => {
                const timestamp = backup.timestamp || backup['backup-time'] || backup.ctime || backup.snaptime;
                if (timestamp) {
                    stats.totalBackups++;
                    if (timestamp >= today) stats.backupsToday++;
                    if (timestamp >= weekAgo) stats.backupsThisWeek++;
                    if (timestamp >= monthAgo) stats.backupsThisMonth++;
                }
            });
        });
        
        // Convert sets to counts
        return {
            uniqueGuests: stats.totalGuests.size,
            totalEntries: stats.totalEntries,
            pbsGuests: stats.pbsGuests.size,
            pveGuests: stats.pveGuests.size,
            snapshotGuests: stats.snapshotGuests.size,
            healthyGuests: stats.healthyGuests.size,
            warningGuests: stats.warningGuests.size,
            criticalGuests: stats.criticalGuests.size,
            failedGuests: stats.failedGuests.size,
            namespaceCount: stats.namespaces.size,
            totalBackups: stats.totalBackups,
            backupsToday: stats.backupsToday,
            backupsThisWeek: stats.backupsThisWeek,
            backupsThisMonth: stats.backupsThisMonth
        };
    }
    
    // Public API
    return {
        aggregateByDate,
        getEntriesForDate,
        groupByNamespace,
        calculateStats
    };
})();