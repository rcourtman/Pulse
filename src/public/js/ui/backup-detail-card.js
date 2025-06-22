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
        
        // Check if any filters are active (excluding 'all' values)
        const hasActiveFilters = filterInfo && (
            filterInfo.search ||
            (filterInfo.guestType && filterInfo.guestType !== 'all') ||
            (filterInfo.backupType && filterInfo.backupType !== 'all') ||
            (filterInfo.healthStatus && filterInfo.healthStatus !== 'all') ||
            (filterInfo.namespace && filterInfo.namespace !== 'all') ||
            (filterInfo.pbsInstance && filterInfo.pbsInstance !== 'all')
        );
        // If no filters active, show summary view
        if (!hasActiveFilters) {
            return getCompactOverview(backups, stats, filterInfo);
        }
        
        // Otherwise show detailed table view
        return getCompactDetailTable(backups, stats, filterInfo);
    }

    function getCompactOverview(backups, stats, filterInfo) {
        // Get grouping preference
        const groupByNode = PulseApp.state.get('groupByNode') ?? true;
        
        // First deduplicate guests when viewing "all namespaces"
        const guestMap = new Map();
        backups.forEach(guest => {
            const guestKey = `${guest.guestId || guest.vmid}-${guest.node}`;
            
            if (!guestMap.has(guestKey)) {
                guestMap.set(guestKey, guest);
            } else {
                const existingGuest = guestMap.get(guestKey);
                
                // Compare backup times to keep the entry with the most recent backup
                let existingTime = 0;
                let currentTime = 0;
                
                if (existingGuest.latestTimes) {
                    const times = Object.values(existingGuest.latestTimes).filter(t => t && t > 0);
                    if (times.length > 0) existingTime = Math.max(...times);
                } else if (existingGuest.latestBackupTime) {
                    existingTime = existingGuest.latestBackupTime;
                }
                
                if (guest.latestTimes) {
                    const times = Object.values(guest.latestTimes).filter(t => t && t > 0);
                    if (times.length > 0) currentTime = Math.max(...times);
                } else if (guest.latestBackupTime) {
                    currentTime = guest.latestBackupTime;
                }
                
                if (currentTime > existingTime) {
                    guestMap.set(guestKey, guest);
                }
            }
        });
        
        // Get nodes data to determine infrastructure
        const nodesData = PulseApp.state.get('nodesData') || [];
        const nodeGroups = new Map();
        
        // Process deduplicated guests and group by node
        guestMap.forEach(guest => {
            const nodeName = guest.node || 'Unknown';
            
            if (!nodeGroups.has(nodeName)) {
                // Find node info
                const nodeInfo = nodesData.find(n => (n.node || n.name) === nodeName);
                nodeGroups.set(nodeName, {
                    cluster: nodeInfo?.clusterIdentifier || 'Standalone',
                    type: nodeInfo?.endpointType || 'unknown',
                    guests: [],
                    healthy: 0,
                    outdated: 0,
                    unprotected: 0,
                    pbsBackups: 0,
                    pveBackups: 0,
                    snapshots: 0
                });
            }
            
            const nodeGroup = nodeGroups.get(nodeName);
            nodeGroup.guests.push(guest);
            
            // Track backup methods
            if (guest.pbsBackups > 0) nodeGroup.pbsBackups++;
            if (guest.pveBackups > 0) nodeGroup.pveBackups++;
            if (guest.snapshotCount > 0) nodeGroup.snapshots++;
            
            // Categorize by backup status
            const now = Date.now() / 1000;
            let mostRecent = 0;
            
            if (guest.latestTimes) {
                const times = Object.values(guest.latestTimes).filter(t => t && t > 0);
                if (times.length > 0) mostRecent = Math.max(...times);
            } else if (guest.latestBackupTime) {
                mostRecent = guest.latestBackupTime;
            }
            
            const hasBackup = guest.pbsBackups > 0 || guest.pveBackups > 0 || guest.snapshotCount > 0;
            const ageInDays = mostRecent ? (now - mostRecent) / 86400 : Infinity;
            
            if (!hasBackup) {
                nodeGroup.unprotected++;
            } else if (ageInDays <= 7) {
                nodeGroup.healthy++;
            } else {
                nodeGroup.outdated++;
            }
        });
        
        // Sort nodes by name
        const sortedNodes = Array.from(nodeGroups.entries()).sort((a, b) => a[0].localeCompare(b[0]));
        
        if (!groupByNode) {
            // Flat view - combine all into one summary
            let totalHealthy = 0;
            let totalOutdated = 0;
            let totalUnprotected = 0;
            
            nodeGroups.forEach(group => {
                totalHealthy += group.healthy;
                totalOutdated += group.outdated;
                totalUnprotected += group.unprotected;
            });
            
            const totalGuests = totalHealthy + totalOutdated + totalUnprotected;
            const overallHealth = totalGuests > 0 ? Math.round((totalHealthy / totalGuests) * 100) : 0;
            
            return `
                <div class="flex flex-col h-full">
                    <!-- Single Overview -->
                    <div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
                        <div class="text-center mb-3">
                            <div class="text-3xl font-bold ${overallHealth >= 80 ? 'text-green-600' : overallHealth >= 60 ? 'text-yellow-600' : 'text-red-600'}">
                                ${overallHealth}%
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                                ${totalHealthy}/${totalGuests} guests with current backups
                            </div>
                        </div>
                        
                        <div class="grid grid-cols-3 gap-3 text-center">
                            <div class="bg-white dark:bg-gray-900 rounded-lg p-3">
                                <div class="text-2xl font-bold text-green-600">${totalHealthy}</div>
                                <div class="text-xs text-gray-500">Current</div>
                                <div class="text-[9px] text-gray-400">≤7 days</div>
                            </div>
                            <div class="bg-white dark:bg-gray-900 rounded-lg p-3">
                                <div class="text-2xl font-bold text-yellow-600">${totalOutdated}</div>
                                <div class="text-xs text-gray-500">Outdated</div>
                                <div class="text-[9px] text-gray-400">>7 days</div>
                            </div>
                            <div class="bg-white dark:bg-gray-900 rounded-lg p-3">
                                <div class="text-2xl font-bold text-red-600">${totalUnprotected}</div>
                                <div class="text-xs text-gray-500">No Backup</div>
                                <div class="text-[9px] text-gray-400">unprotected</div>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        } else {
            // Grouped view - show each node separately
            return `
                <div class="flex flex-col h-full overflow-y-auto">
                    ${sortedNodes.map(([nodeName, group]) => {
                        const nodeTotal = group.healthy + group.outdated + group.unprotected;
                        const nodeHealth = nodeTotal > 0 ? Math.round((group.healthy / nodeTotal) * 100) : 0;
                        const healthColor = nodeHealth >= 80 ? 'green' : nodeHealth >= 60 ? 'yellow' : 'red';
                        
                        // Create backup method badges
                        const backupMethods = [];
                        if (group.pbsBackups > 0) backupMethods.push(`<span class="text-purple-600">PBS</span>`);
                        if (group.pveBackups > 0) backupMethods.push(`<span class="text-orange-600">PVE</span>`);
                        if (group.snapshots > 0) backupMethods.push(`<span class="text-yellow-600">Snap</span>`);
                        
                        return `
                            <div class="mb-3 last:mb-0">
                                <!-- Node Header -->
                                <div class="bg-gray-100 dark:bg-gray-700/80 rounded-t-lg px-3 py-2">
                                    <div class="flex items-center justify-between">
                                        <div class="flex items-center gap-2">
                                            <span class="font-semibold text-sm text-gray-700 dark:text-gray-300">${nodeName}</span>
                                            <span class="text-xs text-gray-500">
                                                ${nodeTotal} guest${nodeTotal !== 1 ? 's' : ''}
                                                ${backupMethods.length > 0 ? `• ${backupMethods.join(' ')}` : ''}
                                            </span>
                                        </div>
                                        <div class="flex items-center gap-2">
                                            <div class="text-lg font-bold text-${healthColor}-600">${nodeHealth}%</div>
                                        </div>
                                    </div>
                                </div>
                                
                                <!-- Node Stats -->
                                <div class="bg-white dark:bg-gray-800 rounded-b-lg border border-t-0 border-gray-200 dark:border-gray-700 p-3">
                                    <div class="flex items-center gap-2 mb-2">
                                        <div class="flex-1 bg-gray-200 dark:bg-gray-700 rounded-full h-3 overflow-hidden">
                                            <div class="h-full bg-${healthColor}-500 transition-all" style="width: ${nodeHealth}%"></div>
                                        </div>
                                    </div>
                                    <div class="grid grid-cols-3 gap-2 text-center text-xs">
                                        <div>
                                            <span class="font-semibold text-green-600">${group.healthy}</span>
                                            <span class="text-gray-500 ml-1">current</span>
                                        </div>
                                        <div>
                                            <span class="font-semibold text-yellow-600">${group.outdated}</span>
                                            <span class="text-gray-500 ml-1">outdated</span>
                                        </div>
                                        <div>
                                            <span class="font-semibold text-red-600">${group.unprotected}</span>
                                            <span class="text-gray-500 ml-1">none</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        `;
                    }).join('')}
                </div>
            `;
        }
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
        
        
        // Sort by namespace first (if "all" namespaces selected), then by guest name
        const namespaceFilter = data.namespaceFilter || 'all';
        const sortedBackups = [...backups].sort((a, b) => {
            if (namespaceFilter === 'all') {
                // Sort by namespace path hierarchically
                const aNamespace = a.namespace || 'root';
                const bNamespace = b.namespace || 'root';
                
                // Root always comes first
                if (aNamespace === 'root' && bNamespace !== 'root') return -1;
                if (bNamespace === 'root' && aNamespace !== 'root') return 1;
                
                // For nested namespaces, sort by full path
                const namespaceCompare = aNamespace.localeCompare(bNamespace);
                if (namespaceCompare !== 0) return namespaceCompare;
            }
            // Then sort by name
            return (a.name || a.vmid).localeCompare(b.name || b.vmid);
        });
        
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
                        ${(() => {
                            let currentNamespace = null;
                            return sortedBackups.map(backup => {
                                // Get filtered backup types and counts based on active filter
                                const filteredBackupData = getFilteredSingleDateBackupData(backup, filterInfo);
                                
                                // Check if we need a namespace header
                                const backupNamespace = backup.namespace || 'root';
                                let namespaceHeader = '';
                                
                                if (namespaceFilter === 'all' && backupNamespace !== currentNamespace) {
                                    currentNamespace = backupNamespace;
                                    
                                    // Calculate nesting level and format namespace path
                                    const namespaceParts = currentNamespace.split('/');
                                    const nestingLevel = namespaceParts.length - 1;
                                    const displayName = namespaceParts[namespaceParts.length - 1];
                                    const parentPath = namespaceParts.slice(0, -1).join('/');
                                    
                                    namespaceHeader = `
                                        <div class="px-1 py-1 mt-2 ${currentNamespace !== sortedBackups[0].namespace ? 'border-t border-gray-200 dark:border-gray-700' : ''}">
                                            <div class="flex items-center" style="padding-left: ${nestingLevel * 12}px">
                                                ${nestingLevel > 0 ? '<span class="text-[10px] text-gray-400 dark:text-gray-500 mr-1">└─</span>' : ''}
                                                <span class="text-[10px] font-semibold text-purple-700 dark:text-purple-400 uppercase">
                                                    ${displayName}
                                                </span>
                                                ${parentPath ? `<span class="text-[9px] text-gray-500 dark:text-gray-400 ml-1">(in ${parentPath})</span>` : ''}
                                            </div>
                                        </div>
                                    `;
                                }
                                
                                return namespaceHeader + `
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
                            }).join('');
                        })()}
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