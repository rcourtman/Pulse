PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backupDetailCard = (() => {
    let pendingTimeout = null;
    
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
        
        // Check if any filters are active (excluding 'all' values)
        const hasActiveFilters = filterInfo && (
            filterInfo.search ||
            (filterInfo.guestType && filterInfo.guestType !== 'all') ||
            (filterInfo.backupType && filterInfo.backupType !== 'all') ||
            (filterInfo.healthStatus && filterInfo.healthStatus !== 'all')
        );
        
        
        // If no filters active, show summary view
        if (!hasActiveFilters) {
            return getCompactOverview(backups, stats, filterInfo);
        }
        
        // Otherwise show detailed table view
        return getCompactDetailTable(backups, stats, filterInfo);
    }

    function getCompactOverview(backups, stats, filterInfo) {
        
        // Calculate critical metrics
        const now = new Date();
        const criticalAge = 14; // days
        const warningAge = 7; // days
        
        // Group guests by backup age
        const guestsByAge = {
            good: [],      // < 24h
            ok: [],        // 1-7 days
            warning: [],   // 7-14 days
            critical: [],  // > 14 days
            none: []       // no backups
        };
        
        
        // Analyze each guest
        backups.forEach(guest => {
            let mostRecentBackup = null;
            
            // Check if we have filter info to determine which backup types to consider
            const activeBackupFilter = filterInfo?.backupType;
            
            
            // If a specific backup type filter is active, use type-specific latest times
            if (activeBackupFilter && activeBackupFilter !== 'all') {
                let latestTimestamp = null;
                
                // Use direct timestamp lookup from latestTimes
                if (guest.latestTimes) {
                    switch(activeBackupFilter) {
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
                    mostRecentBackup = new Date(latestTimestamp * 1000);
                }
                
            } else {
                // Use overall latest backup time for 'all' filter or when no filter is active
                
                // Find most recent backup - only use if it's a valid timestamp
                if (guest.latestBackupTime && guest.latestBackupTime > 0) {
                    // latestBackupTime is a Unix timestamp from the main backup status
                    mostRecentBackup = new Date(guest.latestBackupTime * 1000);
                } else if (guest.backupDates && guest.backupDates.length > 0) {
                    // Fallback to backupDates array - find the most recent actual backup
                    const validDates = guest.backupDates
                        .filter(bd => bd.date && new Date(bd.date).getTime() > 0)
                        .map(bd => new Date(bd.date))
                        .sort((a, b) => b - a);
                    if (validDates.length > 0) {
                        mostRecentBackup = validDates[0];
                    }
                }
            }
            
            const guestData = {
                id: guest.guestId,
                name: guest.guestName,
                type: guest.guestType,
                pbsCount: guest.pbsBackups || 0,
                pveCount: guest.pveBackups || 0,
                snapCount: guest.snapshotCount || 0,
                failures: guest.recentFailures || 0,
                lastBackup: mostRecentBackup
            };
            
            if (!mostRecentBackup) {
                guestsByAge.none.push(guestData);
            } else {
                const ageInDays = (now - mostRecentBackup) / (1000 * 60 * 60 * 24);
                guestData.ageInDays = ageInDays;
                
                if (ageInDays < 1) {
                    guestsByAge.good.push(guestData);
                } else if (ageInDays <= warningAge) {
                    guestsByAge.ok.push(guestData);
                } else if (ageInDays <= criticalAge) {
                    guestsByAge.warning.push(guestData);
                } else {
                    guestsByAge.critical.push(guestData);
                }
            }
        });
        
        // Sort each group
        Object.keys(guestsByAge).forEach(key => {
            if (key !== 'none') {
                guestsByAge[key].sort((a, b) => (a.ageInDays || 0) - (b.ageInDays || 0));
            }
        });
        
        // Calculate summary stats
        const totalIssues = guestsByAge.critical.length + guestsByAge.warning.length + guestsByAge.none.length;
        const healthScore = Math.round(((stats.totalGuests - totalIssues) / stats.totalGuests) * 100) || 0;
        
        return `
            <div class="flex flex-col h-full">
                <!-- Compact Header with Key Metrics -->
                <div class="mb-2 pb-2 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between mb-2">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">Backup Health</h3>
                        <div class="text-right">
                            <div class="text-lg font-bold ${healthScore >= 80 ? 'text-green-600 dark:text-green-400' : healthScore >= 60 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'}">${healthScore}%</div>
                            <div class="text-[9px] text-gray-500 dark:text-gray-400">${stats.totalGuests - totalIssues}/${stats.totalGuests} healthy</div>
                        </div>
                    </div>
                    <div class="grid grid-cols-4 gap-2 text-[10px]">
                        <div class="text-center">
                            <div class="font-semibold text-green-600 dark:text-green-400">${guestsByAge.good.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">&lt;24h</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-blue-600 dark:text-blue-400">${guestsByAge.ok.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">1-7d</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-orange-600 dark:text-orange-400">${guestsByAge.warning.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">7-14d</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-red-600 dark:text-red-400">${guestsByAge.critical.length + guestsByAge.none.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">&gt;14d</div>
                        </div>
                    </div>
                </div>
                
                <!-- Scrollable Guest List -->
                <div class="flex-1 overflow-y-auto">
                    ${totalIssues > 0 ? `
                        <!-- Critical/Warning Guests -->
                        <div class="mb-2">
                            <h4 class="text-[10px] font-semibold text-red-700 dark:text-red-400 uppercase tracking-wider mb-1">Needs Attention (${totalIssues})</h4>
                            <div class="space-y-0.5">
                                ${[...guestsByAge.none, ...guestsByAge.critical, ...guestsByAge.warning].map(guest => `
                                    <div class="flex items-center justify-between px-1 py-0.5 rounded text-[11px] bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30">
                                        <div class="flex items-center gap-1 flex-1 min-w-0">
                                            <span class="text-[9px] font-medium ${guest.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.type}</span>
                                            <span class="font-mono text-gray-600 dark:text-gray-400">${guest.id}</span>
                                            <span class="truncate text-gray-700 dark:text-gray-300">${guest.name}</span>
                                        </div>
                                        <div class="flex items-center gap-2 ml-2">
                                            <div class="flex items-center gap-1 text-[9px]">
                                                ${guest.pbsCount > 0 ? '<span class="text-purple-600 dark:text-purple-400 font-medium">PBS</span>' : ''}
                                                ${guest.pveCount > 0 ? '<span class="text-orange-600 dark:text-orange-400 font-medium">PVE</span>' : ''}
                                                ${guest.snapCount > 0 ? '<span class="text-yellow-600 dark:text-yellow-400 font-medium">SNAP</span>' : ''}
                                            </div>
                                            ${guest.failures > 0 ? `<span class="text-red-600 dark:text-red-400">⚠ ${guest.failures}</span>` : ''}
                                            <span class="${guest.lastBackup ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'} font-medium">
                                                ${guest.lastBackup ? formatAge(guest.ageInDays) : 'Never'}
                                            </span>
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Recent Backups -->
                    ${guestsByAge.good.length > 0 ? `
                        <div class="mb-2">
                            <h4 class="text-[10px] font-semibold text-green-700 dark:text-green-400 uppercase tracking-wider mb-1">Recent (${guestsByAge.good.length})</h4>
                            <div class="space-y-0.5">
                                ${guestsByAge.good.map(guest => `
                                    <div class="flex items-center justify-between px-1 py-0.5 rounded text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30">
                                        <div class="flex items-center gap-1 flex-1 min-w-0">
                                            <span class="text-[9px] font-medium ${guest.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.type}</span>
                                            <span class="font-mono text-gray-600 dark:text-gray-400">${guest.id}</span>
                                            <span class="truncate text-gray-700 dark:text-gray-300">${guest.name}</span>
                                        </div>
                                        <div class="flex items-center gap-2 ml-2">
                                            <div class="flex items-center gap-1 text-[9px]">
                                                ${guest.pbsCount > 0 ? '<span class="text-purple-600 dark:text-purple-400 font-medium">PBS</span>' : ''}
                                                ${guest.pveCount > 0 ? '<span class="text-orange-600 dark:text-orange-400 font-medium">PVE</span>' : ''}
                                                ${guest.snapCount > 0 ? '<span class="text-yellow-600 dark:text-yellow-400 font-medium">SNAP</span>' : ''}
                                            </div>
                                            <span class="text-green-600 dark:text-green-400 font-medium">${formatAge(guest.ageInDays)}</span>
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Summary Stats -->
                    <div class="mt-3 pt-2 border-t border-gray-200 dark:border-gray-700">
                        <div class="space-y-2">
                            <!-- Total Backups and Coverage -->
                            <div class="grid grid-cols-2 gap-3 text-[10px]">
                                <div class="text-center">
                                    <div class="font-semibold text-blue-600 dark:text-blue-400">${stats.totalBackups || 0}</div>
                                    <div class="text-gray-500 dark:text-gray-400">Total Backups</div>
                                </div>
                                <div class="text-center">
                                    <div class="font-semibold text-green-600 dark:text-green-400">${guestsByAge.good.length + guestsByAge.ok.length}</div>
                                    <div class="text-gray-500 dark:text-gray-400">Recent Coverage</div>
                                </div>
                            </div>
                            
                            <!-- Backup Type Distribution -->
                            <div class="text-[9px] text-gray-600 dark:text-gray-400">
                                <div class="flex justify-between items-center">
                                    <span>Backup Types:</span>
                                    <div class="flex gap-2">
                                        ${stats.pbsCount > 0 ? `<span class="text-purple-600 dark:text-purple-400">${stats.pbsCount} PBS</span>` : ''}
                                        ${stats.pveCount > 0 ? `<span class="text-orange-600 dark:text-orange-400">${stats.pveCount} PVE</span>` : ''}
                                        ${stats.snapshotCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">${stats.snapshotCount} Snap</span>` : ''}
                                    </div>
                                </div>
                            </div>
                            
                            <!-- Coverage Detail -->
                            <div class="text-[9px] text-gray-600 dark:text-gray-400">
                                <div class="flex justify-between items-center">
                                    <span>Coverage:</span>
                                    <span>${stats.totalGuests - guestsByAge.none.length}/${stats.totalGuests} guests protected</span>
                                </div>
                            </div>
                        </div>
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
                                if (guest.latestBackupTime && guest.latestBackupTime > 0) {
                                    // latestBackupTime is a Unix timestamp from the main backup status
                                    mostRecent = new Date(guest.latestBackupTime * 1000);
                                } else if (guest.backupDates && guest.backupDates.length > 0) {
                                    // Fallback to filtered backup dates
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
                                    </div>
                                    <div class="col-span-3 flex items-center gap-1 text-[9px]">
                                        ${filteredBackupData.typeLabels}
                                    </div>
                                    <div class="col-span-2 text-right text-gray-600 dark:text-gray-400">
                                        
                                    </div>
                                    <div class="col-span-2 text-right font-medium ${getAgeColor(ageInDays)}">
                                        ${formatAge(ageInDays)}
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
        const sortedBackups = [...backups].sort((a, b) => 
            (a.name || a.vmid).localeCompare(b.name || b.vmid)
        );
        
        return `
            <div class="flex flex-col h-full">
                <!-- Header -->
                <div class="mb-2 pb-1 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                            ${formatCompactDate(date)}
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
                        ${sortedBackups.map(backup => {
                            // Get filtered backup types and counts based on active filter
                            const filteredBackupData = getFilteredSingleDateBackupData(backup, filterInfo);
                            
                            return `
                                <div class="flex items-center justify-between px-1 py-0.5 text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded">
                                    <div class="flex items-center gap-1 min-w-0">
                                        <span class="text-[9px] font-medium ${backup.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${backup.type}</span>
                                        <span class="font-mono text-gray-600 dark:text-gray-400">${backup.vmid}</span>
                                        <span class="truncate text-gray-700 dark:text-gray-300">${backup.name}</span>
                                    </div>
                                    <div class="flex items-center gap-2 ml-2">
                                        <div class="flex items-center gap-1 text-[9px]">
                                            ${filteredBackupData.typeLabels}
                                        </div>
                                    </div>
                                </div>
                            `;
                        }).join('')}
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
    function formatAge(ageInDays) {
        // Debug for unusual age calculations  
        if (ageInDays > 0.5 || Math.floor(ageInDays * 24) > 10) {
            console.log(`[Debug] formatAge called with unusual value:`, {
                ageInDays: ageInDays,
                result: ageInDays < 1 ? `${Math.floor(ageInDays * 24)}h` : `${Math.floor(ageInDays)}d`
            });
        }
        
        if (ageInDays === Infinity) return 'Never';
        if (ageInDays < 1) return `${Math.floor(ageInDays * 24)}h`;
        if (ageInDays < 7) return `${Math.floor(ageInDays)}d`;
        if (ageInDays < 30) return `${Math.floor(ageInDays / 7)}w`;
        return `${Math.floor(ageInDays / 30)}mo`;
    }

    function getAgeColor(ageInDays) {
        if (ageInDays <= 1) return 'text-green-600 dark:text-green-400';
        if (ageInDays <= 3) return 'text-blue-600 dark:text-blue-400';
        if (ageInDays <= 7) return 'text-yellow-600 dark:text-yellow-400';
        if (ageInDays <= 14) return 'text-orange-600 dark:text-orange-400';
        return 'text-red-600 dark:text-red-400';
    }

    function formatCompactDate(dateStr) {
        const date = new Date(dateStr);
        const month = date.toLocaleDateString('en-US', { month: 'short' });
        const day = date.getDate();
        return `${month} ${day}`;
    }

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
        return parts.length > 0 ? parts.join(' / ') : 'Filtered Results';
    }

    function getBackupTypeLabel(type) {
        switch(type) {
            case 'pbs': return 'PBS backups';
            case 'pve': return 'PVE backups';
            case 'snapshots': return 'snapshots';
            default: return 'backups';
        }
    }

    function updateBackupDetailCard(card, data, instant = false) {
        if (!card) return;
        
        const contentDiv = card.querySelector('.backup-detail-content');
        if (!contentDiv) return;
        
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
            pendingTimeout = setTimeout(updateContent, 100);
        } else {
            updateContent();
        }
    }

    return {
        createBackupDetailCard,
        updateBackupDetailCard
    };
})();