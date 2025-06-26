PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backupDetailCard = (() => {
    let pendingTimeout = null;
    let expandedClusters = new Set(); // Track which clusters are expanded
    let lastDataHash = null; // Track data changes to prevent unnecessary updates
    
    function createBackupDetailCard(data) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 rounded-lg p-2 border border-gray-200 dark:border-gray-700 h-full flex flex-col';
        card.style.height = '100%';
        card.innerHTML = `
            <div class="backup-detail-content h-full overflow-hidden flex flex-col">
                ${!data ? getEmptyState(true) : getDetailContent(data)}
            </div>
        `;
        
        return card;
    }

    function getEmptyState(isLoading = true) {
        if (isLoading) {
            return `
                <div class="flex flex-col items-center justify-center h-full text-center px-2 py-4">
                    <div class="animate-pulse">
                        <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-32 mb-2"></div>
                        <div class="h-3 bg-gray-200 dark:bg-gray-700 rounded w-24 mx-auto"></div>
                    </div>
                </div>
            `;
        } else {
            return `
                <div class="flex flex-col items-center justify-center h-full text-center px-2 py-4">
                    <svg class="w-6 h-6 text-gray-400 dark:text-gray-500 mb-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" 
                            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                    </svg>
                    <p class="text-xs text-gray-500 dark:text-gray-400">No backup data</p>
                </div>
            `;
        }
    }

    function getDetailContent(data) {
        // Handle both single-date and multi-date data
        if (data.isMultiDate) {
            return getMultiDateContent(data);
        } else {
            return getSingleDateContent(data);
        }
    }

    function getMultiDateContent(data) {
        const { backups, stats, filterInfo } = data;
        
        // For the refactored table, always show a simple summary
        return getSimpleBackupSummary(backups, stats, filterInfo, data);
    }

    function getSimpleBackupSummary(backups, stats, filterInfo, additionalData) {
        if (!backups || backups.length === 0) {
            return getEmptyState(false);
        }
        
        // Calculate actionable insights from the filtered backups
        const backupTypeFilter = filterInfo?.backupType || 'all';
        const now = Date.now() / 1000; // Current time in seconds
        
        // Process snapshot data if provided
        const snapshotsByGuest = {};
        if (additionalData?.vmSnapshots) {
            additionalData.vmSnapshots.forEach(snapshot => {
                const vmid = String(snapshot.vmid);
                if (!snapshotsByGuest[vmid]) {
                    snapshotsByGuest[vmid] = [];
                }
                snapshotsByGuest[vmid].push(snapshot);
            });
        }
        
        // Health tracking
        const healthAlerts = [];
        const guestsNeedingAttention = [];
        let oldestBackupTime = Infinity;
        let newestBackupTime = 0;
        let oldestBackupGuest = null;
        let newestBackupGuest = null;
        
        // Activity tracking  
        const last24h = now - (24 * 60 * 60);
        const last7d = now - (7 * 24 * 60 * 60);
        const last30d = now - (30 * 24 * 60 * 60);
        let backupsLast24h = 0;
        let backupsLast7d = 0;
        let backupsLast30d = 0;
        
        // Size tracking
        let totalSize = 0;
        let largestBackup = null;
        let largestBackupSize = 0;
        
        // Failed tasks
        let failedTasks = 0;
        let failedTasksGuests = [];
        
        // Backup patterns
        const backupFrequencies = [];
        const orphanedGuests = []; // Guests with backups but in stopped/paused state
        const inconsistentBackups = []; // Guests with mixed backup types (some failing)
        let totalBackupCount = 0;
        
        // Time patterns
        const hourCounts = new Array(24).fill(0); // Track which hours backups occur
        const daysSinceBackup = [];
        
        backups.forEach(guest => {
            // Skip guests that don't match the current filter
            if (backupTypeFilter === 'snapshots' && (!guest.snapshotCount || guest.snapshotCount === 0)) {
                return; // Skip guests without snapshots when filtering by snapshots
            } else if (backupTypeFilter === 'pbs' && (!guest.pbsBackups || guest.pbsBackups === 0)) {
                return; // Skip guests without PBS backups when filtering by PBS
            } else if (backupTypeFilter === 'pve' && (!guest.pveBackups || guest.pveBackups === 0)) {
                return; // Skip guests without PVE backups when filtering by PVE
            }
            
            // Check backup health and track guests needing attention
            if (guest.backupHealthStatus === 'none') {
                guestsNeedingAttention.push({
                    name: guest.guestName,
                    id: guest.guestId,
                    reason: 'No backups'
                });
            } else if (guest.backupHealthStatus === 'old' || guest.backupHealthStatus === 'failed') {
                const lastBackup = guest.latestBackupTime;
                const daysSince = lastBackup ? Math.floor((now - lastBackup) / (24 * 60 * 60)) : null;
                guestsNeedingAttention.push({
                    name: guest.guestName,
                    id: guest.guestId,
                    reason: guest.backupHealthStatus === 'failed' ? 'Failed backup' : `${daysSince}+ days old`
                });
            }
            
            // Track newest and oldest backups/snapshots based on filter
            let relevantTime = null;
            
            // Determine which timestamp to use based on filter
            if (backupTypeFilter === 'snapshots') {
                // For snapshots, get the latest snapshot time for this guest
                const guestId = String(guest.guestId);
                const guestSnapshots = snapshotsByGuest[guestId] || [];
                if (guestSnapshots.length > 0) {
                    // Find the most recent snapshot
                    relevantTime = Math.max(...guestSnapshots.map(s => s.snaptime || 0));
                } else if (guest.latestSnapshotTime) {
                    // Use latestSnapshotTime from guest data if available
                    relevantTime = guest.latestSnapshotTime;
                }
                // Don't fall back to backup times when filtering by snapshots
            } else if (backupTypeFilter === 'all') {
                // For 'all', use the most recent of any type
                const guestSnapshots = snapshotsByGuest[String(guest.guestId)] || [];
                const latestSnapshotTime = guestSnapshots.length > 0 ? 
                    Math.max(...guestSnapshots.map(s => s.snaptime || 0)) : 0;
                
                relevantTime = Math.max(
                    guest.latestBackupTime || 0,
                    latestSnapshotTime
                ) || null;
            } else if (backupTypeFilter === 'pbs') {
                // For PBS filter, use PBS-specific backup time
                relevantTime = guest.lastPbsBackupTime || guest.latestBackupTime;
            } else if (backupTypeFilter === 'pve') {
                // For PVE filter, use PVE-specific backup time
                relevantTime = guest.lastPveBackupTime || guest.latestBackupTime;
            } else {
                // Default to general backup time
                relevantTime = guest.latestBackupTime;
            }
            
            if (relevantTime) {
                // For snapshot filter, only count if this guest actually has snapshots
                if (backupTypeFilter === 'snapshots' && guest.snapshotCount === 0) {
                    relevantTime = null;
                }
            }
            
            if (relevantTime) {
                if (relevantTime > newestBackupTime) {
                    newestBackupTime = relevantTime;
                    newestBackupGuest = guest;
                }
                if (relevantTime < oldestBackupTime) {
                    oldestBackupTime = relevantTime;
                    oldestBackupGuest = guest;
                }
                
                // Count recent activity
                if (relevantTime > last24h) backupsLast24h++;
                if (relevantTime > last7d) backupsLast7d++;
                if (relevantTime > last30d) backupsLast30d++;
            }
            
            // Track failed tasks
            if (guest.lastBackupTask && guest.lastBackupTask.endtime > last24h && 
                guest.lastBackupTask.exitstatus && guest.lastBackupTask.exitstatus !== 'OK') {
                failedTasks++;
                failedTasksGuests.push(guest.guestName);
            }
            
            // Track backup patterns
            if (guest.latestBackupTime) {
                const daysSince = Math.floor((now - guest.latestBackupTime) / (24 * 60 * 60));
                daysSinceBackup.push(daysSince);
                
                // Track hour of backup
                const backupDate = new Date(guest.latestBackupTime * 1000);
                hourCounts[backupDate.getHours()]++;
            }
            
            // Track total backup count
            totalBackupCount += (guest.pbsBackups || 0) + (guest.pveBackups || 0);
            
            // Check for orphaned backups (stopped/paused VMs with backups)
            if (guest.status && (guest.status === 'stopped' || guest.status === 'paused') && 
                guest.latestBackupTime && guest.latestBackupTime > last30d) {
                orphanedGuests.push({
                    name: guest.guestName,
                    status: guest.status,
                    lastBackup: formatRelativeTime(guest.latestBackupTime)
                });
            }
            
            // Check for inconsistent backup patterns
            if (guest.pbsBackups > 0 && guest.pveBackups > 0) {
                // Guest has multiple backup types - could indicate migration or misconfiguration
                inconsistentBackups.push({
                    name: guest.guestName,
                    types: `${guest.pbsBackups} PBS, ${guest.pveBackups} PVE`
                });
            }
            
            // Track sizes if available
            if (guest.totalBackupSize) {
                totalSize += guest.totalBackupSize;
                if (guest.totalBackupSize > largestBackupSize) {
                    largestBackupSize = guest.totalBackupSize;
                    largestBackup = guest;
                }
            }
        });
        
        // Calculate intelligent insights
        const insights = [];
        
        // Backup frequency insight
        if (daysSinceBackup.length > 0) {
            const avgDaysSince = daysSinceBackup.reduce((a, b) => a + b, 0) / daysSinceBackup.length;
            if (avgDaysSince > 7) {
                insights.push({
                    type: 'warning',
                    text: `Backup frequency low: avg ${Math.round(avgDaysSince)} days between backups`
                });
            }
        }
        
        // Find most active backup hour
        const maxHour = hourCounts.indexOf(Math.max(...hourCounts));
        const maxCount = Math.max(...hourCounts);
        if (maxCount > backups.length * 0.3) { // If >30% backups happen in same hour
            insights.push({
                type: 'info',
                text: `Peak backup time: ${maxHour}:00 (${maxCount} backups)`
            });
        }
        
        // Orphaned backups insight
        if (orphanedGuests.length > 0) {
            insights.push({
                type: 'info',
                text: `${orphanedGuests.length} stopped/paused ${orphanedGuests.length === 1 ? 'VM has' : 'VMs have'} recent backups`,
                detail: orphanedGuests[0].name
            });
        }
        
        // Mixed backup types insight
        if (inconsistentBackups.length > 0) {
            insights.push({
                type: 'warning',
                text: `${inconsistentBackups.length} ${inconsistentBackups.length === 1 ? 'guest has' : 'guests have'} mixed backup types`,
                detail: inconsistentBackups[0].name
            });
        }
        
        // Backup efficiency insight
        if (totalBackupCount > 0 && backups.length > 0) {
            const avgBackupsPerGuest = totalBackupCount / backups.length;
            if (avgBackupsPerGuest > 20) {
                insights.push({
                    type: 'info',
                    text: `High retention: avg ${Math.round(avgBackupsPerGuest)} backups per guest`
                });
            }
        }
        
        // Generate filter label
        const filterLabel = getFilterLabel(filterInfo);
        const hasFilters = filterInfo && (
            filterInfo.search ||
            (filterInfo.guestType && filterInfo.guestType !== 'all') ||
            (filterInfo.backupType && filterInfo.backupType !== 'all') ||
            (filterInfo.healthStatus && filterInfo.healthStatus !== 'all') ||
            (filterInfo.namespace && filterInfo.namespace !== 'all') ||
            (filterInfo.pbsInstance && filterInfo.pbsInstance !== 'all')
        );
        
        // Format time helpers
        const formatRelativeTime = (timestamp) => {
            if (!timestamp) return 'Never';
            const seconds = now - timestamp;
            if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
            if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
            if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
            return `${Math.floor(seconds / 604800)}w ago`;
        };
        
        const formatSize = (bytes) => {
            if (!bytes) return '0 B';
            const units = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(1024));
            return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
        };
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-2 pb-1 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                            ${hasFilters ? filterLabel : 'Backup Summary'}
                        </h3>
                        <span class="text-xs text-gray-500 dark:text-gray-400">${backups.length} guests</span>
                    </div>
                </div>
                
                <!-- Actionable Insights -->
                <div class="flex-grow overflow-y-auto">
                    <!-- Health Alerts -->
                    ${guestsNeedingAttention.length > 0 ? `
                        <div class="mb-3 p-2 bg-red-50 dark:bg-red-900/20 rounded-md border border-red-200 dark:border-red-800">
                            <h4 class="text-xs font-medium text-red-700 dark:text-red-300 mb-1">
                                ⚠️ ${guestsNeedingAttention.length} ${guestsNeedingAttention.length === 1 ? 'guest needs' : 'guests need'} attention
                            </h4>
                            <div class="space-y-0.5">
                                ${guestsNeedingAttention.slice(0, 3).map(guest => `
                                    <div class="text-xs text-red-600 dark:text-red-400">
                                        <span class="font-medium">${guest.name || guest.id}</span>: ${guest.reason}
                                    </div>
                                `).join('')}
                                ${guestsNeedingAttention.length > 3 ? `
                                    <div class="text-xs text-red-600 dark:text-red-400 italic">
                                        +${guestsNeedingAttention.length - 3} more...
                                    </div>
                                ` : ''}
                            </div>
                        </div>
                    ` : `
                        <div class="mb-3 p-2 bg-green-50 dark:bg-green-900/20 rounded-md border border-green-200 dark:border-green-800">
                            <div class="text-xs text-green-700 dark:text-green-300">
                                ✓ All guests backed up successfully
                            </div>
                        </div>
                    `}
                    
                    <!-- Recent Activity -->
                    <div class="mb-3">
                        <h4 class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">Recent Activity</h4>
                        <div class="space-y-1">
                            ${newestBackupGuest ? `
                                <div class="flex items-center justify-between">
                                    <span class="text-xs text-gray-600 dark:text-gray-400">
                                        Last ${backupTypeFilter === 'snapshots' ? 'snapshot' : 
                                               backupTypeFilter === 'pbs' ? 'PBS backup' : 
                                               backupTypeFilter === 'pve' ? 'PVE backup' : 'backup'}
                                    </span>
                                    <span class="text-xs font-medium text-gray-800 dark:text-gray-200">
                                        ${formatRelativeTime(newestBackupTime)} 
                                        <span class="text-gray-500">(${newestBackupGuest.guestName})</span>
                                    </span>
                                </div>
                            ` : ''}
                            ${oldestBackupGuest && oldestBackupTime !== newestBackupTime ? `
                                <div class="flex items-center justify-between">
                                    <span class="text-xs text-gray-600 dark:text-gray-400">
                                        Least recent ${backupTypeFilter === 'snapshots' ? 'snapshot' : 
                                                      backupTypeFilter === 'pbs' ? 'PBS' : 
                                                      backupTypeFilter === 'pve' ? 'PVE' : ''}
                                    </span>
                                    <span class="text-xs font-medium text-gray-800 dark:text-gray-200">
                                        ${formatRelativeTime(oldestBackupTime)}
                                        <span class="text-gray-500">(${oldestBackupGuest.guestName})</span>
                                    </span>
                                </div>
                            ` : ''}
                            <div class="flex items-center justify-between">
                                <span class="text-xs text-gray-600 dark:text-gray-400">Activity</span>
                                <span class="text-xs font-medium text-gray-800 dark:text-gray-200">
                                    24h: ${backupsLast24h} | 7d: ${backupsLast7d} | 30d: ${backupsLast30d}
                                </span>
                            </div>
                            ${failedTasks > 0 ? `
                                <div class="flex items-center justify-between">
                                    <span class="text-xs text-gray-600 dark:text-gray-400">Failed (24h)</span>
                                    <span class="text-xs font-medium text-red-600 dark:text-red-400">${failedTasks} tasks</span>
                                </div>
                            ` : ''}
                        </div>
                    </div>
                    
                    <!-- Intelligent Insights -->
                    ${insights.length > 0 ? `
                        <div class="mb-3">
                            <h4 class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">Insights</h4>
                            <div class="space-y-1">
                                ${insights.slice(0, 3).map(insight => `
                                    <div class="text-xs ${
                                        insight.type === 'warning' ? 'text-yellow-600 dark:text-yellow-400' :
                                        insight.type === 'error' ? 'text-red-600 dark:text-red-400' :
                                        'text-blue-600 dark:text-blue-400'
                                    }">
                                        ${insight.type === 'warning' ? '⚡' : 
                                          insight.type === 'error' ? '❌' : '💡'} 
                                        ${insight.text}
                                        ${insight.detail ? `<span class="text-gray-500 dark:text-gray-400">(${insight.detail})</span>` : ''}
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Storage Insights (if available) -->
                    ${totalSize > 0 ? `
                        <div class="mb-3">
                            <h4 class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">Storage</h4>
                            <div class="space-y-1">
                                <div class="flex items-center justify-between">
                                    <span class="text-xs text-gray-600 dark:text-gray-400">Total size</span>
                                    <span class="text-xs font-medium text-gray-800 dark:text-gray-200">${formatSize(totalSize)}</span>
                                </div>
                                ${largestBackup ? `
                                    <div class="flex items-center justify-between">
                                        <span class="text-xs text-gray-600 dark:text-gray-400">Largest</span>
                                        <span class="text-xs font-medium text-gray-800 dark:text-gray-200">
                                            ${formatSize(largestBackupSize)}
                                            <span class="text-gray-500">(${largestBackup.guestName})</span>
                                        </span>
                                    </div>
                                ` : ''}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Filter Summary -->
                    ${hasFilters && backupTypeFilter !== 'all' ? `
                        <div class="mt-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                            <div class="text-xs text-gray-500 dark:text-gray-400 italic">
                                Showing ${backupTypeFilter === 'pbs' ? 'PBS' : backupTypeFilter === 'pve' ? 'PVE' : 'snapshot'} data only
                            </div>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }

    function getCompactOverview(backups, stats, filterInfo) {
        // Group by node/infrastructure first
        const nodeGroups = {};
        const groupByNode = PulseApp.state.get('groupByNode') ?? true;
        
        backups.forEach(guest => {
            const nodeName = guest.node || 'Unknown';
            
            
            if (!nodeGroups[nodeName]) {
                nodeGroups[nodeName] = {
                    guests: [],
                    totalGuests: 0,
                    healthy: 0,
                    lessThan24h: 0,
                    oneToSevenDays: 0,
                    sevenToFourteenDays: 0,
                    moreThanFourteenDays: 0,
                    noBackup: 0,
                    pbsCount: 0,
                    pveCount: 0,
                    snapCount: 0
                };
            }
            
            const group = nodeGroups[nodeName];
            group.totalGuests++;
            
            // Get the latest backup time for this guest
            let latestTime = 0;
            let backupType = '';
            
            if (guest.latestTimes) {
                // Check PBS backups
                if (guest.latestTimes.pbs && guest.latestTimes.pbs > 0) {
                    if (guest.latestTimes.pbs > latestTime) {
                        latestTime = guest.latestTimes.pbs;
                        backupType = 'PBS';
                    }
                }
                // Check PVE backups
                if (guest.latestTimes.pve && guest.latestTimes.pve > 0) {
                    if (guest.latestTimes.pve > latestTime) {
                        latestTime = guest.latestTimes.pve;
                        backupType = 'PVE';
                    }
                }
                // Check snapshots
                if (guest.latestTimes.snapshot && guest.latestTimes.snapshot > 0) {
                    if (guest.latestTimes.snapshot > latestTime) {
                        latestTime = guest.latestTimes.snapshot;
                        backupType = 'Snap';
                    }
                }
            } else if (guest.latestBackupTime) {
                latestTime = guest.latestBackupTime;
                // Determine backup type from guest data
                if (guest.pbsBackups > 0) backupType = 'PBS';
                else if (guest.pveBackups > 0) backupType = 'PVE';
                else if (guest.snapshotCount > 0) backupType = 'Snap';
            }
            
            // Track backup types
            if (guest.pbsBackups > 0) group.pbsCount++;
            if (guest.pveBackups > 0) group.pveCount++;
            if (guest.snapshotCount > 0) group.snapCount++;
            
            if (latestTime > 0) {
                group.healthy++;
                
                // Calculate age
                const now = Date.now() / 1000;
                const age = now - latestTime;
                const days = age / 86400;
                
                if (days < 1) group.lessThan24h++;
                else if (days <= 7) group.oneToSevenDays++;
                else if (days <= 14) group.sevenToFourteenDays++;
                else group.moreThanFourteenDays++;
                
                // Create unique guest key to prevent duplicates
                const guestKey = `${guest.guestId || guest.vmid}-${guest.node}`;
                
                const existingGuestIndex = group.guests.findIndex(g => `${g.id}-${g.node}` === guestKey);
                if (existingGuestIndex >= 0) {
                    // Update existing guest if newer backup
                    if (latestTime > group.guests[existingGuestIndex].time) {
                        group.guests[existingGuestIndex] = {
                            type: guest.guestType || 'Unknown',
                            id: guest.guestId || guest.vmid,
                            name: guest.guestName || guest.name || 'Unknown',
                            backupType: backupType,
                            time: latestTime,
                            node: guest.node
                        };
                    }
                } else {
                    group.guests.push({
                        type: guest.guestType || 'Unknown',
                        id: guest.guestId || guest.vmid,
                        name: guest.guestName || guest.name || 'Unknown',
                        backupType: backupType,
                        time: latestTime,
                        node: guest.node
                    });
                }
            } else {
                group.noBackup++;
            }
        });
        
        // Sort guests by time within each group
        Object.values(nodeGroups).forEach(group => {
            group.guests.sort((a, b) => b.time - a.time);
        });
        
        
        // Format age for display
        const formatAge = (timestamp) => {
            const now = Date.now() / 1000;
            const age = now - timestamp;
            const hours = Math.floor(age / 3600);
            const days = Math.floor(hours / 24);
            
            if (days > 0) return `${days}d`;
            else if (hours > 0) return `${hours}h`;
            else return 'Now';
        };
        
        // Single unified summary view - groupByNode only affects the table, not the summary
        const sortedNodes = Object.entries(nodeGroups).sort((a, b) => a[0].localeCompare(b[0]));
        
        // Collect all guests but maintain their node information
        let allGuests = [];
        let totalByAge = {
            lessThan24h: 0,
            oneToSevenDays: 0,
            sevenToFourteenDays: 0,
            moreThanFourteenDays: 0,
            noBackup: 0
        };
        
        sortedNodes.forEach(([nodeName, group]) => {
            group.guests.forEach(guest => {
                allGuests.push({...guest, nodeName});
            });
            totalByAge.lessThan24h += group.lessThan24h;
            totalByAge.oneToSevenDays += group.oneToSevenDays;
            totalByAge.sevenToFourteenDays += group.sevenToFourteenDays;
            totalByAge.moreThanFourteenDays += group.moreThanFourteenDays;
            totalByAge.noBackup += group.noBackup;
        });
        
        // Sort all guests by time
        allGuests.sort((a, b) => b.time - a.time);
        
        return `
            <div class="h-full flex flex-col text-xs">
                <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Backup Health Summary</h3>
                
                <!-- Time buckets summary -->
                <div class="grid grid-cols-4 gap-1 mb-2">
                    <div class="text-center p-1 bg-gray-50 dark:bg-gray-800 rounded">
                        <div class="text-sm font-bold text-green-600">${totalByAge.lessThan24h}</div>
                        <div class="text-[10px] text-gray-500">&lt;24h</div>
                    </div>
                    <div class="text-center p-1 bg-gray-50 dark:bg-gray-800 rounded">
                        <div class="text-sm font-bold text-yellow-600">${totalByAge.oneToSevenDays}</div>
                        <div class="text-[10px] text-gray-500">1-7d</div>
                    </div>
                    <div class="text-center p-1 bg-gray-50 dark:bg-gray-800 rounded">
                        <div class="text-sm font-bold text-orange-600">${totalByAge.sevenToFourteenDays}</div>
                        <div class="text-[10px] text-gray-500">7-14d</div>
                    </div>
                    <div class="text-center p-1 bg-gray-50 dark:bg-gray-800 rounded">
                        <div class="text-sm font-bold text-red-600">${totalByAge.moreThanFourteenDays}</div>
                        <div class="text-[10px] text-gray-500">&gt;14d</div>
                    </div>
                </div>
                
                <!-- Node breakdown -->
                <div class="flex-1 overflow-y-auto">
                    <div class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Backup Status by Node</div>
                    <div class="space-y-2">
                        ${sortedNodes.map(([nodeName, group]) => {
                            const healthPercentage = group.totalGuests > 0 ? Math.round((group.healthy / group.totalGuests) * 100) : 0;
                            // Show all backups for this node
                            const nodeRecentBackups = group.guests;
                            
                            return `
                                <div class="bg-gray-50 dark:bg-gray-800 rounded p-2">
                                    <div class="flex items-center justify-between mb-1">
                                        <span class="font-medium text-gray-700 dark:text-gray-300">${nodeName}</span>
                                        <div class="flex items-center gap-2">
                                            <div class="flex items-center gap-1 text-[10px]">
                                                ${group.pbsCount > 0 ? `<span class="text-purple-600">${group.pbsCount} PBS</span>` : ''}
                                                ${group.pveCount > 0 ? `<span class="text-orange-600">${group.pveCount} PVE</span>` : ''}
                                                ${group.snapCount > 0 ? `<span class="text-yellow-600">${group.snapCount} Snap</span>` : ''}
                                            </div>
                                            <span class="text-[10px] font-bold ${healthPercentage >= 80 ? 'text-green-600' : healthPercentage >= 60 ? 'text-yellow-600' : 'text-red-600'}">${healthPercentage}%</span>
                                        </div>
                                    </div>
                                    
                                    
                                    <!-- Most recent backups for this node -->
                                    ${nodeRecentBackups.length > 0 ? `
                                        <div class="space-y-0.5 text-[10px]">
                                            ${nodeRecentBackups.map(guest => `
                                                <div class="flex items-center justify-between">
                                                    <div class="flex items-center gap-1 min-w-0">
                                                        <span class="${guest.type === 'VM' ? 'text-blue-600' : 'text-green-600'}">${guest.type}</span>
                                                        <span class="text-gray-500">${guest.id}</span>
                                                        <span class="text-gray-700 dark:text-gray-300 truncate">${guest.name}</span>
                                                    </div>
                                                    <div class="flex items-center gap-1">
                                                        <span class="text-[9px] px-1 rounded ${
                                                            guest.backupType === 'PBS' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' :
                                                            guest.backupType === 'PVE' ? 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300' :
                                                            'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300'
                                                        }">${guest.backupType}</span>
                                                        <span class="text-gray-400">${formatAge(guest.time)}</span>
                                                    </div>
                                                </div>
                                            `).join('')}
                                        </div>
                                    ` : ''}
                                </div>
                            `;
                        }).join('')}
                    </div>
                </div>
                
            </div>
        `;
    }

    function getCompactDetailTable(backups, stats, filterInfo) {
        // Sort backups by most recent backup date
        const sortedBackups = [...backups].sort((a, b) => {
            const aDate = a.backupDates.length > 0 ? new Date(a.backupDates[0].date) : new Date(0);
            const bDate = b.backupDates.length > 0 ? new Date(b.backupDates[0].date) : new Date(0);
            return bDate - aDate;
        });
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-2 pb-1 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                            ${getFilterLabel(filterInfo)}
                        </h3>
                        <span class="text-xs text-gray-500 dark:text-gray-400">${stats.totalGuests} guests</span>
                    </div>
                </div>
                
                <!-- Table Header -->
                <div class="grid grid-cols-12 gap-1 px-1 pb-1 text-[10px] font-semibold text-gray-600 dark:text-gray-400 uppercase">
                    <div class="col-span-5">Guest</div>
                    <div class="col-span-3">Types</div>
                    <div class="col-span-2 text-right">Count</div>
                    <div class="col-span-2 text-right">Age</div>
                </div>
                
                <!-- Scrollable Table Body -->
                <div class="flex-1 overflow-y-auto">
                    <div class="space-y-0.5">
                        ${sortedBackups.map(guest => {
                            // Calculate age based on filtered backup data when specific backup type is selected
                            let mostRecent = null;
                            const now = new Date();
                            const backupTypeFilter = filterInfo?.backupType;
                            
                            // Use type-specific latest times when a specific backup type is selected
                            if (backupTypeFilter && backupTypeFilter !== 'all') {
                                let latestTimestamp = null;
                                
                                // Use direct timestamp lookup from latestTimes
                                if (guest.latestTimes) {
                                    switch(backupTypeFilter) {
                                        case 'pve':
                                            latestTimestamp = guest.latestTimes.pve;
                                            break;
                                        case 'pbs':
                                            latestTimestamp = guest.latestTimes.pbs;
                                            break;
                                        case 'snapshots':
                                            latestTimestamp = guest.latestTimes.snapshots;
                                            break;
                                    }
                                }
                                
                                // Convert timestamp to Date object
                                if (latestTimestamp) {
                                    mostRecent = new Date(latestTimestamp * 1000);
                                }
                                
                            } else {
                                // Use overall backup age when no filter is active
                                // IMPORTANT: Calculate the actual most recent backup across all types
                                
                                let latestTimestamp = null;
                                
                                // First check latestTimes object for the most recent across all backup types
                                if (guest.latestTimes) {
                                    const times = [
                                        guest.latestTimes.pbs,
                                        guest.latestTimes.pve,
                                        guest.latestTimes.snapshots
                                    ].filter(t => t && t > 0);
                                    
                                    if (times.length > 0) {
                                        latestTimestamp = Math.max(...times);
                                    }
                                }
                                
                                // If we found a timestamp from latestTimes, use it
                                if (latestTimestamp) {
                                    mostRecent = new Date(latestTimestamp * 1000);
                                } else if (guest.latestBackupTime && guest.latestBackupTime > 0) {
                                    // Fallback to latestBackupTime if latestTimes is not available
                                    mostRecent = new Date(guest.latestBackupTime * 1000);
                                } else if (guest.backupDates && guest.backupDates.length > 0) {
                                    // Final fallback to backupDates array
                                    mostRecent = new Date(guest.backupDates[0].date);
                                }
                            }
                            
                            const ageInDays = mostRecent 
                                ? (now - mostRecent) / (1000 * 60 * 60 * 24)
                                : Infinity;
                            
                            // Get filtered backup types and counts based on active filter
                            const filteredBackupData = getFilteredBackupData(guest, filterInfo);
                            
                            return `
                                <div class="grid grid-cols-12 gap-1 px-1 py-0.5 text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded">
                                    <div class="col-span-5 flex items-center gap-1 min-w-0">
                                        <span class="text-[9px] font-medium ${guest.guestType === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.guestType}</span>
                                        <span class="font-mono text-gray-600 dark:text-gray-400">${guest.guestId}</span>
                                        <span class="truncate text-gray-700 dark:text-gray-300">${guest.guestName}</span>
                                        ${guest.node ? `<span class="text-[8px] text-gray-500 dark:text-gray-400">@${guest.node}</span>` : ''}
                                    </div>
                                    <div class="col-span-3 flex items-center gap-1 text-[9px]">
                                        ${filteredBackupData.typeLabels}
                                    </div>
                                    <div class="col-span-2 text-right text-gray-600 dark:text-gray-400">
                                        
                                    </div>
                                    <div class="col-span-2 text-right font-medium ${PulseApp.utils.backup.getAgeColor(ageInDays)}">
                                        ${PulseApp.utils.backup.formatAge(ageInDays)}
                                    </div>
                                </div>
                            `;
                        }).join('')}
                    </div>
                </div>
            </div>
        `;
    }

    function getSingleDateContent(data) {
        const { date, backups, stats, filterInfo } = data;
        
        if (!backups || backups.length === 0) {
            return getEmptyState(false);
        }
        
        // Sort by guest name
        const sortedBackups = [...backups].sort((a, b) => {
            return (a.name || `VM ${a.vmid}`).localeCompare(b.name || `VM ${b.vmid}`);
        });
        
        // Generate simple list of backups for the selected date
        const backupList = sortedBackups.map(backup => {
            // Get backup type labels based on the types array from calendar
            const types = Array.isArray(backup.types) ? backup.types : [];
            const typeLabels = [];
            
            if (types.includes('pbsSnapshots')) {
                typeLabels.push('<span class="px-1 py-0.5 rounded text-[8px] bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-medium">PBS</span>');
            }
            if (types.includes('pveBackups')) {
                typeLabels.push('<span class="px-1 py-0.5 rounded text-[8px] bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-medium">PVE</span>');
            }
            if (types.includes('vmSnapshots')) {
                typeLabels.push('<span class="px-1 py-0.5 rounded text-[8px] bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-medium">SNAP</span>');
            }
            
            const guestType = backup.type === 'qemu' ? 'VM' : (backup.type === 'lxc' ? 'LXC' : backup.type);
            
            return `
                <div class="flex items-center justify-between px-1 py-0.5 text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded">
                    <div class="flex items-center gap-1 min-w-0">
                        <span class="text-[9px] font-medium ${guestType === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guestType}</span>
                        <span class="font-mono text-gray-600 dark:text-gray-400">${backup.vmid}</span>
                        <span class="truncate text-gray-700 dark:text-gray-300">${backup.name || `Guest ${backup.vmid}`}</span>
                        ${backup.node ? `<span class="text-[9px] text-gray-500 dark:text-gray-400">(${backup.node})</span>` : ''}
                    </div>
                    <div class="flex items-center gap-2 ml-2">
                        <div class="flex items-center gap-1 text-[9px]">
                            ${typeLabels.join(' ')}
                            ${backup.backupCount > 1 ? `<span class="text-gray-500 dark:text-gray-400">${backup.backupCount}</span>` : ''}
                        </div>
                    </div>
                </div>
            `;
        }).join('');
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-2 pb-1 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                            ${PulseApp.utils.backup.formatCompactDate(date)}
                        </h3>
                        <span class="text-xs text-gray-500 dark:text-gray-400">${stats.totalGuests} guests</span>
                    </div>
                    <div class="flex items-center gap-3 mt-1 text-[10px]">
                        ${getFilteredStatsDisplay(stats, filterInfo)}
                    </div>
                </div>
                
                <!-- Guest List -->
                <div class="flex-1 overflow-y-auto">
                    <div class="space-y-0.5">
                        ${backupList}
                    </div>
                </div>
            </div>
        `;
    }

    // Get filtered backup data based on active filter
    function getFilteredBackupData(guest, filterInfo) {
        if (!guest) return { typeLabels: '', totalCount: 0 };
        
        const backupType = filterInfo?.backupType;
        
        // If no specific backup type filter is active, show all types
        if (!backupType || backupType === 'all') {
            const typeLabels = [
                guest.pbsBackups > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-medium">PBS</span>' : '',
                guest.pveBackups > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-medium">PVE</span>' : '',
                guest.snapshotCount > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-medium">SNAP</span>' : ''
            ].filter(label => label).join(' ');
            
            return {
                typeLabels,
                totalCount: guest.pbsBackups + guest.pveBackups + guest.snapshotCount
            };
        }
        
        // Show only the filtered backup type
        switch (backupType) {
            case 'pbs':
                return {
                    typeLabels: guest.pbsBackups > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-medium">PBS</span>' : '',
                    totalCount: guest.pbsBackups
                };
            case 'pve':
                return {
                    typeLabels: guest.pveBackups > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-medium">PVE</span>' : '',
                    totalCount: guest.pveBackups
                };
            case 'snapshots':
                return {
                    typeLabels: guest.snapshotCount > 0 ? '<span class="px-1 py-0.5 rounded text-[8px] bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-medium">SNAP</span>' : '',
                    totalCount: guest.snapshotCount
                };
            default:
                return {
                    typeLabels: '',
                    totalCount: 0
                };
        }
    }

    // Get filtered backup data for single date view based on active filter
    function getFilteredSingleDateBackupData(backup, filterInfo) {
        if (!backup) return { typeLabels: '', backupCount: 0 };
        
        const backupType = filterInfo?.backupType;
        
        // If no specific backup type filter is active, show all types
        if (!backupType || backupType === 'all') {
            const typeLabels = [
                backup.types.includes('pbsSnapshots') ? '<span class="px-1 py-0.5 rounded text-[8px] bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-medium">PBS</span>' : '',
                backup.types.includes('pveBackups') ? '<span class="px-1 py-0.5 rounded text-[8px] bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-medium">PVE</span>' : '',
                backup.types.includes('vmSnapshots') ? '<span class="px-1 py-0.5 rounded text-[8px] bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-medium">SNAP</span>' : ''
            ].filter(label => label).join(' ');
            
            return {
                typeLabels,
                backupCount: backup.backupCount
            };
        }
        
        // Show only the filtered backup type
        const typeMapping = {
            'pbs': 'pbsSnapshots',
            'pve': 'pveBackups', 
            'snapshots': 'vmSnapshots'
        };
        
        const targetType = typeMapping[backupType];
        if (backup.types.includes(targetType)) {
            switch (backupType) {
                case 'pbs':
                    return {
                        typeLabels: '<span class="px-1 py-0.5 rounded text-[8px] bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-medium">PBS</span>',
                        backupCount: backup.backupCount // Note: This shows total count, which may include other types
                    };
                case 'pve':
                    return {
                        typeLabels: '<span class="px-1 py-0.5 rounded text-[8px] bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-medium">PVE</span>',
                        backupCount: backup.backupCount
                    };
                case 'snapshots':
                    return {
                        typeLabels: '<span class="px-1 py-0.5 rounded text-[8px] bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-medium">SNAP</span>',
                        backupCount: backup.backupCount
                    };
            }
        }
        
        return {
            typeLabels: '',
            backupCount: 0
        };
    }

    // Get filtered stats display for single date view
    function getFilteredStatsDisplay(stats, filterInfo) {
        if (!stats) return '';
        
        const backupType = filterInfo?.backupType;
        
        // If no specific backup type filter is active, show all stats
        if (!backupType || backupType === 'all') {
            return [
                stats.pbsCount > 0 ? `<span class="text-purple-600 dark:text-purple-400">PBS: ${stats.pbsCount}</span>` : '',
                stats.pveCount > 0 ? `<span class="text-orange-600 dark:text-orange-400">PVE: ${stats.pveCount}</span>` : '',
                stats.snapshotCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">Snap: ${stats.snapshotCount}</span>` : ''
            ].filter(stat => stat).join('');
        }
        
        // Show only the filtered backup type stat
        switch (backupType) {
            case 'pbs':
                return stats.pbsCount > 0 ? `<span class="text-purple-600 dark:text-purple-400">PBS: ${stats.pbsCount}</span>` : '';
            case 'pve':
                return stats.pveCount > 0 ? `<span class="text-orange-600 dark:text-orange-400">PVE: ${stats.pveCount}</span>` : '';
            case 'snapshots':
                return stats.snapshotCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">Snap: ${stats.snapshotCount}</span>` : '';
            default:
                return '';
        }
    }

    // Helper functions
    function getFilterLabel(filterInfo) {
        const parts = [];
        if (filterInfo.backupType && filterInfo.backupType !== 'all') {
            parts.push(filterInfo.backupType.toUpperCase());
        }
        if (filterInfo.guestType && filterInfo.guestType !== 'all') {
            parts.push(filterInfo.guestType === 'vm' ? 'VMs' : 'LXCs');
        }
        if (filterInfo.healthStatus && filterInfo.healthStatus !== 'all') {
            parts.push(filterInfo.healthStatus);
        }
        if (filterInfo.namespace && filterInfo.namespace !== 'all') {
            parts.push(`NS: ${filterInfo.namespace}`);
        }
        if (filterInfo.pbsInstance && filterInfo.pbsInstance !== 'all') {
            // Try to get PBS instance name from state
            const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
            const pbsInstance = pbsDataArray[parseInt(filterInfo.pbsInstance)];
            if (pbsInstance && pbsInstance.name) {
                parts.push(`PBS: ${pbsInstance.name}`);
            } else {
                parts.push(`PBS Instance ${filterInfo.pbsInstance}`);
            }
        }
        return parts.length > 0 ? parts.join(' / ') : 'Filtered Results';
    }

    function updateBackupDetailCard(card, data, instant = false) {
        if (!card) return;
        
        const contentDiv = card.querySelector('.backup-detail-content');
        if (!contentDiv) return;
        
        // Create a simple hash of the data to detect changes
        const dataHash = data ? JSON.stringify({
            totalGuests: data.stats?.totalGuests,
            healthyGuests: data.stats?.healthyGuests,
            backupCount: data.backups?.length,
            filterInfo: data.filterInfo
        }) : 'empty';
        
        // Skip update if data hasn't changed and it's not a user action
        if (!instant && dataHash === lastDataHash) {
            return;
        }
        
        lastDataHash = dataHash;
        
        // Cancel any pending timeout
        if (pendingTimeout) {
            clearTimeout(pendingTimeout);
            pendingTimeout = null;
        }
        
        const updateContent = () => {
            const newContent = !data ? getEmptyState(false) : getDetailContent(data);
            
            // Find scrollable container and preserve scroll position
            const scrollableContainer = contentDiv.querySelector('.overflow-y-auto');
            const scrollTop = scrollableContainer ? scrollableContainer.scrollTop : 0;
            
            if (!instant) {
                // For API updates, use a longer debounce to prevent flashing
                contentDiv.style.opacity = '0';
                setTimeout(() => {
                    contentDiv.innerHTML = newContent;
                    contentDiv.style.opacity = '1';
                    
                    // Restore scroll position
                    requestAnimationFrame(() => {
                        const newScrollableContainer = contentDiv.querySelector('.overflow-y-auto');
                        if (newScrollableContainer && scrollTop > 0) {
                            newScrollableContainer.scrollTop = scrollTop;
                        }
                    });
                }, 150);
            } else {
                // For user actions, update immediately
                contentDiv.innerHTML = newContent;
                
                // Restore scroll position for instant updates
                requestAnimationFrame(() => {
                    const newScrollableContainer = contentDiv.querySelector('.overflow-y-auto');
                    if (newScrollableContainer && scrollTop > 0) {
                        newScrollableContainer.scrollTop = scrollTop;
                    }
                });
            }
        };
        
        if (!instant) {
            // Use longer debounce for API updates to prevent dropdowns from closing
            pendingTimeout = setTimeout(updateContent, 500);
        } else {
            updateContent();
        }
    }

    return {
        createBackupDetailCard,
        updateBackupDetailCard
    };
})();