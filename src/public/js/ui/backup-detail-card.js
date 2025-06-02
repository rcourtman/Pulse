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
            return getCompactOverview(backups, stats);
        }
        
        // Otherwise show detailed table view
        return getCompactDetailTable(backups, stats, filterInfo);
    }

    function getCompactOverview(backups, stats) {
        // Debug: Log what the detail card is receiving
        console.log('[Backup Health Debug] getCompactOverview received:', {
            backupsCount: backups.length,
            stats: stats
        });
        
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
                    <div class="flex items-center justify-between mb-1">
                        <h3 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">Backup Health</h3>
                        <span class="text-xs ${healthScore >= 80 ? 'text-green-600 dark:text-green-400' : healthScore >= 60 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'} font-bold">${healthScore}%</span>
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
                        <div class="grid grid-cols-3 gap-2 text-[10px]">
                            <div class="text-center">
                                <div class="font-semibold text-purple-600 dark:text-purple-400">${stats.pbsCount || 0}</div>
                                <div class="text-gray-500 dark:text-gray-400">PBS</div>
                            </div>
                            <div class="text-center">
                                <div class="font-semibold text-orange-600 dark:text-orange-400">${stats.pveCount || 0}</div>
                                <div class="text-gray-500 dark:text-gray-400">PVE</div>
                            </div>
                            <div class="text-center">
                                <div class="font-semibold text-yellow-600 dark:text-yellow-400">${stats.snapshotCount || 0}</div>
                                <div class="text-gray-500 dark:text-gray-400">Snapshots</div>
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
                    <div class="col-span-3 text-center">Types</div>
                    <div class="col-span-2 text-right">Count</div>
                    <div class="col-span-2 text-right">Age</div>
                </div>
                
                <!-- Scrollable Table Body -->
                <div class="flex-1 overflow-y-auto">
                    <div class="space-y-0.5">
                        ${sortedBackups.map(guest => {
                            const mostRecent = guest.backupDates.length > 0 
                                ? new Date(guest.backupDates[0].date)
                                : null;
                            const ageInDays = mostRecent 
                                ? (new Date() - mostRecent) / (1000 * 60 * 60 * 24)
                                : Infinity;
                            
                            return `
                                <div class="grid grid-cols-12 gap-1 px-1 py-0.5 text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded">
                                    <div class="col-span-5 flex items-center gap-1 min-w-0">
                                        <span class="text-[9px] font-medium ${guest.guestType === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.guestType}</span>
                                        <span class="font-mono text-gray-600 dark:text-gray-400">${guest.guestId}</span>
                                        <span class="truncate text-gray-700 dark:text-gray-300">${guest.guestName}</span>
                                    </div>
                                    <div class="col-span-3 flex items-center justify-center gap-1 text-[9px]">
                                        ${guest.pbsBackups > 0 ? '<span class="text-purple-600 dark:text-purple-400 font-medium">PBS</span>' : ''}
                                        ${guest.pveBackups > 0 ? '<span class="text-orange-600 dark:text-orange-400 font-medium">PVE</span>' : ''}
                                        ${guest.snapshotCount > 0 ? '<span class="text-yellow-600 dark:text-yellow-400 font-medium">SNAP</span>' : ''}
                                    </div>
                                    <div class="col-span-2 text-right text-gray-600 dark:text-gray-400">
                                        ${guest.pbsBackups + guest.pveBackups + guest.snapshotCount}
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
        const { date, backups, stats } = data;
        
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
                        ${stats.pbsCount > 0 ? `<span class="text-purple-600 dark:text-purple-400">PBS: ${stats.pbsCount}</span>` : ''}
                        ${stats.pveCount > 0 ? `<span class="text-orange-600 dark:text-orange-400">PVE: ${stats.pveCount}</span>` : ''}
                        ${stats.snapshotCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">Snap: ${stats.snapshotCount}</span>` : ''}
                    </div>
                </div>
                
                <!-- Guest List -->
                <div class="flex-1 overflow-y-auto">
                    <div class="space-y-0.5">
                        ${sortedBackups.map(backup => `
                            <div class="flex items-center justify-between px-1 py-0.5 text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded">
                                <div class="flex items-center gap-1 min-w-0">
                                    <span class="text-[9px] font-medium ${backup.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${backup.type}</span>
                                    <span class="font-mono text-gray-600 dark:text-gray-400">${backup.vmid}</span>
                                    <span class="truncate text-gray-700 dark:text-gray-300">${backup.name}</span>
                                </div>
                                <div class="flex items-center gap-2 ml-2">
                                    <div class="flex items-center gap-1 text-[9px]">
                                        ${backup.types.includes('pbsSnapshots') ? '<span class="text-purple-600 dark:text-purple-400 font-medium">PBS</span>' : ''}
                                        ${backup.types.includes('pveBackups') ? '<span class="text-orange-600 dark:text-orange-400 font-medium">PVE</span>' : ''}
                                        ${backup.types.includes('vmSnapshots') ? '<span class="text-yellow-600 dark:text-yellow-400 font-medium">SNAP</span>' : ''}
                                    </div>
                                    <span class="text-gray-600 dark:text-gray-400">${backup.backupCount}</span>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
            </div>
        `;
    }

    // Helper functions
    function formatAge(ageInDays) {
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
            
            if (!instant) {
                contentDiv.style.opacity = '0';
                setTimeout(() => {
                    contentDiv.innerHTML = newContent;
                    contentDiv.style.opacity = '1';
                }, 150);
            } else {
                contentDiv.innerHTML = newContent;
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