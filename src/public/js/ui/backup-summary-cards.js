PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backupSummaryCards = (() => {
    
    function calculateBackupStatistics(backupData, guestId, guestNode, guestEndpointId) {
        const now = Date.now();
        const stats = {
            lastBackup: { time: null, type: null, status: 'none' },
            coverage: { daily: 0, weekly: 0, monthly: 0 },
            protected: { count: 0, oldestDate: null, coverage: 0 },
            health: { score: 0, issues: [] }
        };

        // Find most recent backup across all types
        const allBackups = [];
        
        ['pbsSnapshots', 'pveBackups', 'vmSnapshots'].forEach(type => {
            if (!backupData[type]) return;
            
            const items = guestId 
                ? backupData[type].filter(item => {
                    // Match vmid
                    const itemVmid = item.vmid || item['backup-id'] || item.backupVMID;
                    if (parseInt(itemVmid, 10) !== parseInt(guestId, 10)) return false;
                    
                    // For PBS backups (centralized), don't filter by node
                    if (type === 'pbsSnapshots') return true;
                    
                    // For PVE backups and snapshots (node-specific), match node/endpoint
                    const itemNode = item.node;
                    const itemEndpoint = item.endpointId;
                    
                    // Match by node if available
                    if (guestNode && itemNode) {
                        return itemNode === guestNode;
                    }
                    
                    // Match by endpointId if available
                    if (guestEndpointId && itemEndpoint) {
                        return itemEndpoint === guestEndpointId;
                    }
                    
                    return true;
                })
                : backupData[type];
            
            items.forEach(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (timestamp) {
                    allBackups.push({
                        time: timestamp * 1000,
                        type: type,
                        protected: item.protected || false,
                        verification: item.verification
                    });
                }
            });
        });

        // Sort by time descending
        allBackups.sort((a, b) => b.time - a.time);

        // Last backup info
        if (allBackups.length > 0) {
            const last = allBackups[0];
            stats.lastBackup = {
                time: last.time,
                type: last.type,
                status: last.verification?.state === 'failed' ? 'failed' : 'success',
                age: now - last.time
            };
        }

        const oneDayAgo = now - 24 * 60 * 60 * 1000;
        const oneWeekAgo = now - 7 * 24 * 60 * 60 * 1000;
        const oneMonthAgo = now - 30 * 24 * 60 * 60 * 1000;
        
        allBackups.forEach(backup => {
            if (backup.time >= oneDayAgo) stats.coverage.daily++;
            if (backup.time >= oneWeekAgo) stats.coverage.weekly++;
            if (backup.time >= oneMonthAgo) stats.coverage.monthly++;
        });

        // Protected backups analysis
        const protectedBackups = allBackups.filter(b => b.protected);
        stats.protected.count = protectedBackups.length;
        if (protectedBackups.length > 0) {
            const oldestProtected = protectedBackups[protectedBackups.length - 1];
            stats.protected.oldestDate = oldestProtected.time;
            stats.protected.coverage = Math.floor((now - oldestProtected.time) / (24 * 60 * 60 * 1000));
        }

        return stats;
    }

    return {
        calculateBackupStatistics
    };
})();