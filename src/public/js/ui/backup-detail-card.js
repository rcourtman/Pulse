PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backupDetailCard = (() => {
    let pendingTimeout = null;
    function createBackupDetailCard(data) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700 h-full flex flex-col';
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
                <div class="flex flex-col items-center justify-center h-full text-center px-4 py-6">
                    <svg class="w-10 h-10 text-gray-400 dark:text-gray-500 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" 
                            d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                    </svg>
                    <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Loading backup data...</h3>
                    <p class="text-xs text-gray-400 dark:text-gray-500">
                        Summary will appear here
                    </p>
                </div>
            `;
        } else {
            return `
                <div class="flex flex-col items-center justify-center h-full text-center px-4 py-6">
                    <svg class="w-10 h-10 text-gray-400 dark:text-gray-500 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" 
                            d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">No matching backups</h3>
                    <p class="text-xs text-gray-400 dark:text-gray-500">
                        No backups found for the selected filters
                    </p>
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
        
        // Check if any filters are active (excluding 'all' values)
        const hasActiveFilters = filterInfo && (
            filterInfo.search ||
            (filterInfo.guestType && filterInfo.guestType !== 'all') ||
            (filterInfo.backupType && filterInfo.backupType !== 'all') ||
            (filterInfo.healthStatus && filterInfo.healthStatus !== 'all')
        );
        
        // If no filters active, show summary view
        if (!hasActiveFilters) {
            return getOverviewSummary(backups, stats);
        }
        
        // Otherwise show detailed view
        // Group backups by date
        const backupsByDate = {};
        backups.forEach(guest => {
            guest.backupDates.forEach(dateInfo => {
                const dateKey = dateInfo.date;
                if (!backupsByDate[dateKey]) {
                    backupsByDate[dateKey] = [];
                }
                backupsByDate[dateKey].push({
                    ...guest,
                    backupTypes: dateInfo.types,
                    backupCount: dateInfo.count
                });
            });
        });

        // Sort dates in descending order
        const sortedDates = Object.keys(backupsByDate).sort((a, b) => b.localeCompare(a));

        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-3 flex-shrink-0">
                    <div class="flex items-center justify-between mb-2">
                        <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">Filtered Backup Details</h3>
                        <div class="text-xs text-gray-500 dark:text-gray-400">${stats.totalBackups} total ${getBackupTypeLabel(filterInfo?.backupType)}</div>
                    </div>
                    <div class="flex flex-wrap gap-2 text-xs">
                        ${getCompactFilterSummary(filterInfo)}
                    </div>
                    <div class="flex flex-wrap gap-2 text-xs mt-2">
                        ${getCompactStatsSummary(stats, filterInfo)}
                    </div>
                </div>
                
                <!-- Scrollable Content -->
                <div class="flex-1 overflow-y-auto pr-1 scrollbar-thin scrollbar-track-gray-100 dark:scrollbar-track-gray-700 scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600 min-h-0">
                    ${sortedDates.length === 0 ? getNoBackupsMessage() : sortedDates.map(date => `
                        <div class="mb-3">
                            <h4 class="text-sm font-medium text-gray-600 dark:text-gray-400 mb-2 sticky top-0 bg-white dark:bg-gray-800 py-1 border-b border-gray-100 dark:border-gray-700/50">${formatCompactDate(date)}</h4>
                            ${getCompactGuestTable(backupsByDate[date], filterInfo)}
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }

    function getSingleDateContent(data) {
        const { date, backups, stats } = data;
        const formattedDate = formatCompactDate(date);
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-3 flex-shrink-0">
                    <div class="flex items-center justify-between mb-2">
                        <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">${formattedDate}</h3>
                        <div class="text-xs text-gray-500 dark:text-gray-400">${stats.totalGuests} guests</div>
                    </div>
                    <div class="flex flex-wrap gap-2 text-xs">
                        ${getCompactStatsSummary(stats)}
                    </div>
                </div>
                
                <!-- Scrollable Content -->
                <div class="flex-1 overflow-y-auto pr-1 scrollbar-thin scrollbar-track-gray-100 dark:scrollbar-track-gray-700 scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600 min-h-0">
                    ${backups.length === 0 ? getNoBackupsMessage() : getCompactGuestTable(backups)}
                </div>
            </div>
        `;
    }

    function formatDate(dateStr) {
        const date = new Date(dateStr);
        const options = { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' };
        return date.toLocaleDateString('en-US', options);
    }
    
    function formatCompactDate(dateStr) {
        const date = new Date(dateStr);
        const options = { month: 'short', day: 'numeric', year: 'numeric' };
        return date.toLocaleDateString('en-US', options);
    }
    
    function getBackupTypeLabel(backupType) {
        switch(backupType) {
            case 'pbs': return 'PBS backups';
            case 'pve': return 'PVE backups'; 
            case 'snapshots': return 'snapshots';
            default: return 'backups';
        }
    }
    
    function getCompactFilterSummary(filterInfo) {
        if (!filterInfo) return '';
        
        const items = [];
        
        if (filterInfo.search) {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Search: <span class="font-medium text-gray-700 dark:text-gray-300">"${filterInfo.search}"</span></span>`);
        }
        
        if (filterInfo.guestType && filterInfo.guestType !== 'all') {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Type: <span class="font-medium text-gray-700 dark:text-gray-300">${filterInfo.guestType.toUpperCase()}</span></span>`);
        }
        
        if (filterInfo.backupType && filterInfo.backupType !== 'all') {
            const typeLabels = { pbs: 'PBS', pve: 'PVE', snapshots: 'Snapshots' };
            items.push(`<span class="text-gray-500 dark:text-gray-400">Type: <span class="font-medium text-gray-700 dark:text-gray-300">${typeLabels[filterInfo.backupType]}</span></span>`);
        }
        
        if (filterInfo.healthStatus && filterInfo.healthStatus !== 'all') {
            const statusLabels = { ok: 'OK', stale: 'Stale', warning: 'Warning', none: 'None' };
            items.push(`<span class="text-gray-500 dark:text-gray-400">Status: <span class="font-medium text-gray-700 dark:text-gray-300">${statusLabels[filterInfo.healthStatus] || filterInfo.healthStatus}</span></span>`);
        }
        
        
        return items.join('<span class="text-gray-400 dark:text-gray-500 mx-1">•</span>');
    }
    
    function getCompactStatsSummary(stats, filterInfo = null) {
        const items = [];
        
        // Check if filtering by specific backup type
        const backupTypeFilter = filterInfo?.backupType;
        const isFiltering = backupTypeFilter && backupTypeFilter !== 'all';
        
        if (isFiltering) {
            // When filtering by specific type, show more useful metrics
            const guestCount = stats.pbsCount + stats.pveCount + stats.snapshotCount;
            items.push(`<span class="text-gray-600 dark:text-gray-400">${guestCount} guests</span>`);
            
            if (stats.uniqueDates) {
                items.push(`<span class="text-gray-600 dark:text-gray-400">${stats.uniqueDates} days</span>`);
            }
            
            if (guestCount > 0) {
                const avgPerGuest = (stats.totalBackups / guestCount).toFixed(1);
                items.push(`<span class="text-gray-600 dark:text-gray-400">${avgPerGuest} avg/guest</span>`);
            }
        } else {
            // When showing all types, show the original breakdown
            if (stats.pbsCount > 0) {
                items.push(`<span class="text-purple-600 dark:text-purple-400">PBS: ${stats.pbsCount}</span>`);
            }
            
            if (stats.pveCount > 0) {
                items.push(`<span class="text-orange-600 dark:text-orange-400">PVE: ${stats.pveCount}</span>`);
            }
            
            if (stats.snapshotCount > 0) {
                items.push(`<span class="text-yellow-600 dark:text-yellow-400">Snapshots: ${stats.snapshotCount}</span>`);
            }
        }
        
        if (stats.failureCount > 0) {
            items.push(`<span class="text-red-600 dark:text-red-400">Failed: ${stats.failureCount}</span>`);
        }
        
        return items.join('<span class="text-gray-400 dark:text-gray-500 mx-1">•</span>');
    }
    
    function getOverviewSummary(backups, stats) {
        // Calculate summary statistics
        const dateStats = {};
        let oldestBackup = null;
        let newestBackup = null;
        const guestsByStatus = { healthy: 0, warning: 0, failed: 0, none: 0 };
        const backupsByType = { pbs: 0, pve: 0, snapshots: 0 };
        
        backups.forEach(guest => {
            // Count by health status
            if (guest.recentFailures > 0) {
                guestsByStatus.failed++;
            } else if (guest.backupHealthStatus === 'ok' || guest.backupHealthStatus === 'stale') {
                guestsByStatus.healthy++;
            } else if (guest.backupHealthStatus === 'old') {
                guestsByStatus.warning++;
            } else if (guest.backupHealthStatus === 'none') {
                guestsByStatus.none++;
            }
            
            // Count by backup type
            if (guest.pbsBackups > 0) backupsByType.pbs++;
            if (guest.pveBackups > 0) backupsByType.pve++;
            if (guest.snapshotCount > 0) backupsByType.snapshots++;
            
            // Track dates
            guest.backupDates.forEach(dateInfo => {
                const date = new Date(dateInfo.date);
                if (!oldestBackup || date < oldestBackup) oldestBackup = date;
                if (!newestBackup || date > newestBackup) newestBackup = date;
                
                // Count backups per day
                if (!dateStats[dateInfo.date]) {
                    dateStats[dateInfo.date] = { count: 0, guests: new Set() };
                }
                dateStats[dateInfo.date].count += dateInfo.count;
                dateStats[dateInfo.date].guests.add(guest.guestId);
            });
        });
        
        // Calculate date range
        const dateRange = oldestBackup && newestBackup ? 
            Math.ceil((newestBackup - oldestBackup) / (1000 * 60 * 60 * 24)) + 1 : 0;
        
        // Find busiest day
        let busiestDay = null;
        let maxBackups = 0;
        Object.entries(dateStats).forEach(([date, stats]) => {
            if (stats.count > maxBackups) {
                maxBackups = stats.count;
                busiestDay = date;
            }
        });
        
        // Calculate overall health percentage
        const totalGuests = stats.totalGuests;
        const healthyGuests = guestsByStatus.healthy;
        const healthPercentage = totalGuests > 0 ? Math.round((healthyGuests / totalGuests) * 100) : 0;
        
        // Determine health status color and text
        let healthColor, healthStatus;
        if (healthPercentage >= 80) {
            healthColor = 'text-green-600 dark:text-green-400';
            healthStatus = 'Excellent';
        } else if (healthPercentage >= 60) {
            healthColor = 'text-yellow-600 dark:text-yellow-400'; 
            healthStatus = 'Good';
        } else {
            healthColor = 'text-red-600 dark:text-red-400';
            healthStatus = 'Needs Attention';
        }
        
        // Format last backup time
        const lastBackupText = newestBackup ? 
            `${formatTimeAgo(newestBackup)}` : 'No recent backups';
            
        function formatTimeAgo(date) {
            const now = new Date();
            const diffMs = now - date;
            const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
            const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
            
            if (diffDays === 0) {
                if (diffHours === 0) return 'Less than an hour ago';
                return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
            } else if (diffDays === 1) {
                return 'Yesterday';
            } else if (diffDays < 7) {
                return `${diffDays} days ago`;
            } else {
                return formatCompactDate(date.toISOString());
            }
        }
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-3 flex-shrink-0">
                    <div class="flex items-center justify-between mb-2">
                        <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">Backup Overview</h3>
                        <div class="text-xs text-gray-500 dark:text-gray-400">${stats.totalGuests} guests</div>
                    </div>
                    
                    <!-- Overall Health Status -->
                    <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-3 mb-3">
                        <div class="flex items-center justify-between mb-2">
                            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">Overall Health</span>
                            <span class="${healthColor} text-sm font-semibold">${healthPercentage}%</span>
                        </div>
                        <div class="flex items-center justify-between text-xs">
                            <span class="${healthColor} font-medium">${healthStatus}</span>
                            <span class="text-gray-500 dark:text-gray-400">Last backup: ${lastBackupText}</span>
                        </div>
                    </div>
                    
                    <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
                        Apply filters above to see detailed backup information
                    </p>
                </div>
                
                <!-- Summary Stats -->
                <div class="flex-1 overflow-y-auto pr-1 space-y-3">
                    <!-- Date Range -->
                    <div class="bg-gray-50 dark:bg-gray-700/30 rounded p-3">
                        <h4 class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Date Range</h4>
                        <div class="space-y-1 text-xs">
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Period:</span>
                                <span class="text-gray-700 dark:text-gray-300">${dateRange} days</span>
                            </div>
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Latest:</span>
                                <span class="text-gray-700 dark:text-gray-300">${newestBackup ? formatCompactDate(newestBackup.toISOString()) : 'N/A'}</span>
                            </div>
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Oldest:</span>
                                <span class="text-gray-700 dark:text-gray-300">${oldestBackup ? formatCompactDate(oldestBackup.toISOString()) : 'N/A'}</span>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Backup Statistics -->
                    <div class="bg-gray-50 dark:bg-gray-700/30 rounded p-3">
                        <h4 class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Backup Statistics</h4>
                        <div class="space-y-1 text-xs">
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Total Backups:</span>
                                <span class="text-gray-700 dark:text-gray-300 font-medium">${stats.totalBackups}</span>
                            </div>
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Avg per Guest:</span>
                                <span class="text-gray-700 dark:text-gray-300">${stats.totalGuests > 0 ? (stats.totalBackups / stats.totalGuests).toFixed(1) : 0}</span>
                            </div>
                            <div class="flex justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Busiest Day:</span>
                                <span class="text-gray-700 dark:text-gray-300">${busiestDay ? `${formatCompactDate(busiestDay)} (${maxBackups})` : 'N/A'}</span>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Guest Health -->
                    <div class="bg-gray-50 dark:bg-gray-700/30 rounded p-3">
                        <h4 class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Guest Health</h4>
                        <div class="space-y-1">
                            ${guestsByStatus.healthy > 0 ? `
                                <div class="flex items-center justify-between text-xs">
                                    <span class="text-green-600 dark:text-green-400">● Healthy</span>
                                    <span class="text-gray-700 dark:text-gray-300">${guestsByStatus.healthy} guests</span>
                                </div>
                            ` : ''}
                            ${guestsByStatus.warning > 0 ? `
                                <div class="flex items-center justify-between text-xs">
                                    <span class="text-yellow-600 dark:text-yellow-400">● Warning</span>
                                    <span class="text-gray-700 dark:text-gray-300">${guestsByStatus.warning} guests</span>
                                </div>
                            ` : ''}
                            ${guestsByStatus.failed > 0 ? `
                                <div class="flex items-center justify-between text-xs">
                                    <span class="text-red-600 dark:text-red-400">● Failed</span>
                                    <span class="text-gray-700 dark:text-gray-300">${guestsByStatus.failed} guests</span>
                                </div>
                            ` : ''}
                            ${guestsByStatus.none > 0 ? `
                                <div class="flex items-center justify-between text-xs">
                                    <span class="text-gray-600 dark:text-gray-400">○ No Backups</span>
                                    <span class="text-gray-700 dark:text-gray-300">${guestsByStatus.none} guests</span>
                                </div>
                            ` : ''}
                        </div>
                    </div>
                    
                    <!-- Backup Types -->
                    <div class="bg-gray-50 dark:bg-gray-700/30 rounded p-3">
                        <h4 class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Backup Coverage</h4>
                        <div class="space-y-1 text-xs">
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">PBS Backups:</span>
                                <span class="text-purple-600 dark:text-purple-400">${backupsByType.pbs} guests</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">PVE Backups:</span>
                                <span class="text-orange-600 dark:text-orange-400">${backupsByType.pve} guests</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Snapshots:</span>
                                <span class="text-yellow-600 dark:text-yellow-400">${backupsByType.snapshots} guests</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function getFilterSummary(filterInfo) {
        if (!filterInfo) return '';
        
        const items = [];
        
        if (filterInfo.search) {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Search: <span class="font-medium text-gray-700 dark:text-gray-300">"${filterInfo.search}"</span></span>`);
        }
        
        if (filterInfo.guestType && filterInfo.guestType !== 'all') {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Type: <span class="font-medium text-gray-700 dark:text-gray-300">${filterInfo.guestType.toUpperCase()}</span></span>`);
        }
        
        if (filterInfo.backupType && filterInfo.backupType !== 'all') {
            const typeLabels = { pbs: 'PBS', pve: 'PVE', snapshots: 'Snapshots' };
            items.push(`<span class="text-gray-500 dark:text-gray-400">Backup: <span class="font-medium text-gray-700 dark:text-gray-300">${typeLabels[filterInfo.backupType] || filterInfo.backupType}</span></span>`);
        }
        
        if (filterInfo.healthStatus && filterInfo.healthStatus !== 'all') {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Status: <span class="font-medium text-gray-700 dark:text-gray-300">${filterInfo.healthStatus.charAt(0).toUpperCase() + filterInfo.healthStatus.slice(1)}</span></span>`);
        }
        
        
        return items.join('<span class="text-gray-400 dark:text-gray-500 mx-1">•</span>');
    }

    function getStatsSummary(stats) {
        const items = [];
        
        items.push(`<span class="text-gray-500 dark:text-gray-400">Guests: <span class="font-medium text-gray-700 dark:text-gray-300">${stats.totalGuests}</span></span>`);
        
        if (stats.totalBackups) {
            items.push(`<span class="text-gray-500 dark:text-gray-400">Backups: <span class="font-medium text-gray-700 dark:text-gray-300">${stats.totalBackups}</span></span>`);
        }
        
        if (stats.pbsCount > 0) {
            items.push(`<span class="text-green-600 dark:text-green-400">PBS: ${stats.pbsCount}</span>`);
        }
        
        if (stats.pveCount > 0) {
            items.push(`<span class="text-yellow-600 dark:text-yellow-400">PVE: ${stats.pveCount}</span>`);
        }
        
        if (stats.snapshotCount > 0) {
            items.push(`<span class="text-blue-600 dark:text-blue-400">Snap: ${stats.snapshotCount}</span>`);
        }
        
        if (stats.failureCount > 0) {
            items.push(`<span class="text-red-600 dark:text-red-400">Failed: ${stats.failureCount}</span>`);
        }
        
        return items.join('<span class="text-gray-400 dark:text-gray-500 mx-1">•</span>');
    }

    function getNoBackupsMessage() {
        return `
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                <p class="text-sm">No backups found matching the current filters</p>
            </div>
        `;
    }

    function getGuestTable(backups) {
        // Sort by type then by vmid
        const sortedBackups = [...backups].sort((a, b) => {
            if (a.type !== b.type) return a.type === 'VM' ? -1 : 1;
            return parseInt(a.vmid) - parseInt(b.vmid);
        });
        
        return `
            <table class="w-full text-xs">
                <thead class="sticky top-0 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                    <tr>
                        <th class="text-left py-1 px-1 text-gray-500 dark:text-gray-400 font-medium">Type</th>
                        <th class="text-left py-1 px-1 text-gray-500 dark:text-gray-400 font-medium">ID</th>
                        <th class="text-left py-1 px-1 text-gray-500 dark:text-gray-400 font-medium">Name</th>
                        <th class="text-center py-1 px-1 text-gray-500 dark:text-gray-400 font-medium">Backups</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                    ${sortedBackups.map(guest => {
                        const types = guest.types || guest.backupTypes || [];
                        const typeIndicators = [];
                        
                        if (types.includes('pbsSnapshots') || guest.pbsBackups > 0) typeIndicators.push('<span class="text-purple-600 dark:text-purple-400" title="PBS">PBS</span>');
                        if (types.includes('pveBackups') || guest.pveBackups > 0) typeIndicators.push('<span class="text-orange-600 dark:text-orange-400" title="PVE">PVE</span>');
                        if (types.includes('vmSnapshots') || guest.snapshotCount > 0) typeIndicators.push('<span class="text-yellow-600 dark:text-yellow-400" title="Snapshot">Snapshots</span>');
                        
                        const hasFailure = guest.hasFailure || guest.recentFailures > 0;
                        const guestType = guest.type || guest.guestType;
                        const guestName = guest.name || guest.guestName;
                        const guestId = guest.vmid || guest.guestId;
                        
                        return `
                            <tr class="${hasFailure ? 'bg-red-50 dark:bg-red-900/10' : ''}">
                                <td class="py-1 px-1">
                                    <span class="${guestType === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guestType}</span>
                                </td>
                                <td class="py-1 px-1 text-gray-600 dark:text-gray-400">${guestId}</td>
                                <td class="py-1 px-1 text-gray-800 dark:text-gray-200 truncate max-w-[200px]" title="${guestName}">${guestName}</td>
                                <td class="py-1 px-1 text-center">
                                    ${typeIndicators.join(' ')}
                                    ${guest.backupCount && guest.backupCount > 1 ? `<span class="text-gray-500 dark:text-gray-400 ml-1">(${guest.backupCount})</span>` : ''}
                                    ${hasFailure ? '<span class="text-red-600 dark:text-red-400 ml-1">!</span>' : ''}
                                </td>
                            </tr>
                        `;
                    }).join('')}
                </tbody>
            </table>
        `;
    }
    
    function getCompactGuestTable(backups, filterInfo = null) {
        // Sort by type then by vmid
        const sortedBackups = [...backups].sort((a, b) => {
            if (a.type !== b.type) return a.type === 'VM' ? -1 : 1;
            return parseInt(a.vmid) - parseInt(b.vmid);
        });
        
        return `
            <div class="space-y-0.5 text-xs">
                ${sortedBackups.map(guest => {
                    const types = guest.types || guest.backupTypes || [];
                    const typeIndicators = [];
                    
                    // Only show backup types that match the current filter (if any)
                    const backupTypeFilter = filterInfo?.backupType;
                    const showAllTypes = !backupTypeFilter || backupTypeFilter === 'all';
                    
                    if (showAllTypes) {
                        // When showing all types, show all backup types the guest has
                        if (types.includes('pbsSnapshots') || guest.pbsBackups > 0) {
                            typeIndicators.push('<span class="text-purple-600 dark:text-purple-400" title="PBS Backup">PBS</span>');
                        }
                        if (types.includes('pveBackups') || guest.pveBackups > 0) {
                            typeIndicators.push('<span class="text-orange-600 dark:text-orange-400" title="PVE Backup">PVE</span>');
                        }
                        if (types.includes('vmSnapshots') || guest.snapshotCount > 0) {
                            typeIndicators.push('<span class="text-yellow-600 dark:text-yellow-400" title="Snapshot">Snapshots</span>');
                        }
                    } else {
                        // When filtering by specific type, only show that type if it exists in the current date's types
                        if (backupTypeFilter === 'pbs' && types.includes('pbsSnapshots')) {
                            typeIndicators.push('<span class="text-purple-600 dark:text-purple-400" title="PBS Backup">PBS</span>');
                        }
                        if (backupTypeFilter === 'pve' && types.includes('pveBackups')) {
                            typeIndicators.push('<span class="text-orange-600 dark:text-orange-400" title="PVE Backup">PVE</span>');
                        }
                        if (backupTypeFilter === 'snapshots' && types.includes('vmSnapshots')) {
                            typeIndicators.push('<span class="text-yellow-600 dark:text-yellow-400" title="Snapshot">Snapshots</span>');
                        }
                    }
                    
                    const hasFailure = guest.hasFailure || guest.recentFailures > 0;
                    const guestType = guest.type || guest.guestType;
                    const guestName = guest.name || guest.guestName;
                    const guestId = guest.vmid || guest.guestId;
                    return `
                        <div class="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50 dark:hover:bg-gray-700/50 ${hasFailure ? 'bg-red-50 dark:bg-red-900/10' : ''}">
                            <span class="${guestType === 'VM' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'} px-1.5 py-0.5 rounded text-[10px] font-medium w-10 text-center">${guestType}</span>
                            <span class="text-gray-500 dark:text-gray-400 w-10 text-right">${guestId}</span>
                            <span class="text-gray-700 dark:text-gray-300 flex-1 truncate" title="${guestName}">${guestName}</span>
                            <div class="text-right flex-shrink-0">
                                <div class="flex items-center gap-1 flex-wrap justify-end">
                                    ${typeIndicators.length > 0 ? typeIndicators.map(indicator => `<span class="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-[10px]">${indicator}</span>`).join('') : ''}
                                </div>
                                ${hasFailure ? '<div class="text-red-600 dark:text-red-400 text-[10px] font-medium mt-0.5">FAIL</div>' : ''}
                            </div>
                        </div>
                    `;
                }).join('')}
            </div>
        `;
    }

    function updateBackupDetailCard(card, data, instant = false) {
        // Clear any pending timeout to prevent race conditions
        if (pendingTimeout) {
            clearTimeout(pendingTimeout);
            pendingTimeout = null;
        }
        
        const content = card.querySelector('.backup-detail-content');
        if (content) {
            if (instant) {
                // Instant update for restoring selection
                content.innerHTML = !data ? getEmptyState(false) : getDetailContent(data);
            } else {
                // For empty states, use instant update to avoid stutter
                if (!data) {
                    content.style.opacity = '1';
                    content.style.transition = '';
                    content.innerHTML = getEmptyState(false);
                    return;
                }
                
                // Add fade transition for user clicks with actual data
                content.style.opacity = '0';
                content.style.transition = 'opacity 0.2s ease-in-out';
                
                pendingTimeout = setTimeout(() => {
                    content.innerHTML = getDetailContent(data);
                    content.style.opacity = '1';
                    pendingTimeout = null;
                }, 200);
            }
        }
    }
    
    return {
        createBackupDetailCard,
        updateBackupDetailCard
    };
})();