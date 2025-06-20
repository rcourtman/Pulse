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
        const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
        const oneMonthAgo = new Date(now.getFullYear(), now.getMonth() - 1, now.getDate());
        
        // Detect backup schedule pattern
        let backupSchedule = 'unknown';
        let scheduleThreshold = 7; // days for "recent" coverage
        const backupDates = [];
        
        // Collect all backup dates for schedule detection
        backups.forEach(guest => {
            if (guest.backupDates && guest.backupDates.length > 0) {
                guest.backupDates.forEach(bd => {
                    if (bd.date) {
                        const timestamp = bd.latestTimestamp || new Date(bd.date).getTime() / 1000;
                        if (timestamp) backupDates.push(timestamp);
                    }
                });
            }
        });
        
        // Sort backup dates
        backupDates.sort((a, b) => b - a);
        
        // Detect schedule pattern from recent backups
        if (backupDates.length >= 7) {
            const recentDates = backupDates.slice(0, 14); // Look at last 2 weeks
            const gaps = [];
            for (let i = 1; i < recentDates.length; i++) {
                const gap = (recentDates[i-1] - recentDates[i]) / (24 * 60 * 60); // days
                if (gap < 30) gaps.push(gap); // Ignore monthly+ gaps
            }
            
            if (gaps.length > 0) {
                const avgGap = gaps.reduce((a, b) => a + b, 0) / gaps.length;
                if (avgGap <= 1.5) {
                    backupSchedule = 'daily';
                    scheduleThreshold = 2; // 2 days for daily backups
                } else if (avgGap <= 8) {
                    backupSchedule = 'weekly';
                    scheduleThreshold = 10; // 10 days for weekly backups
                } else {
                    backupSchedule = 'monthly';
                    scheduleThreshold = 35; // 35 days for monthly backups
                }
            }
        }
        
        // Adjust age thresholds based on schedule
        const criticalAge = scheduleThreshold * 2; // 2x the schedule
        const warningAge = scheduleThreshold; // 1x the schedule
        
        // Group guests by backup age
        const guestsByAge = {
            good: [],      // < 24h
            ok: [],        // 1-scheduleThreshold days
            warning: [],   // scheduleThreshold-criticalAge days
            critical: [],  // > criticalAge days
            none: []       // no backups
        };
        
        // Track namespace distribution and monthly backups
        const namespaceStats = new Map();
        const pbsBackupsByNamespace = new Map();
        let backupsThisMonth = 0;
        let lastBackupTime = 0;
        let recentFailures = 0;
        let backupHours = []; // Track backup hours for window detection
        let daysWithBackups = new Set(); // Track unique days with backups
        
        // Analyze each guest
        backups.forEach(guest => {
            let mostRecentBackup = null;
            
            // Count backups this month and track backup times
            if (guest.backupDates && guest.backupDates.length > 0) {
                guest.backupDates.forEach(bd => {
                    const timestamp = bd.latestTimestamp || (bd.date ? new Date(bd.date).getTime() / 1000 : 0);
                    if (timestamp) {
                        const backupDate = new Date(timestamp * 1000);
                        if (backupDate >= startOfMonth) {
                            backupsThisMonth += bd.count || 1;
                            daysWithBackups.add(backupDate.toDateString());
                        }
                        // Track backup hours for window detection
                        if (backupDate >= oneMonthAgo) {
                            backupHours.push(backupDate.getHours());
                        }
                        // Track most recent backup across all guests
                        if (timestamp > lastBackupTime) {
                            lastBackupTime = timestamp;
                        }
                    }
                });
            }
            
            // Count recent failures
            if (guest.recentFailures > 0) {
                recentFailures += guest.recentFailures;
            }
            
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
            
            // Extract namespace information from pbsBackupInfo if available
            let namespaces = [];
            if (guest.pbsBackupInfo && guest.pbsBackups > 0) {
                // Parse namespace info from pbsBackupInfo string
                const nsMatch = guest.pbsBackupInfo.match(/\[(.*?)\]/g);
                if (nsMatch) {
                    nsMatch.forEach(match => {
                        const nsContent = match.slice(1, -1); // Remove brackets
                        const nsList = nsContent.split(',').map(ns => ns.trim());
                        namespaces.push(...nsList);
                    });
                }
                
                // Track namespace statistics
                namespaces.forEach(ns => {
                    namespaceStats.set(ns, (namespaceStats.get(ns) || 0) + 1);
                    if (!pbsBackupsByNamespace.has(ns)) {
                        pbsBackupsByNamespace.set(ns, 0);
                    }
                    pbsBackupsByNamespace.set(ns, pbsBackupsByNamespace.get(ns) + guest.pbsBackups);
                });
            }
            
            const guestData = {
                id: guest.guestId,
                name: guest.guestName,
                type: guest.guestType,
                pbsCount: guest.pbsBackups || 0,
                pveCount: guest.pveBackups || 0,
                snapCount: guest.snapshotCount || 0,
                mostRecentBackupType: guest.mostRecentBackupType,
                failures: guest.recentFailures || 0,
                lastBackup: mostRecentBackup,
                namespaces: namespaces
            };
            
            if (!mostRecentBackup) {
                guestsByAge.none.push(guestData);
            } else {
                const ageInDays = (now - mostRecentBackup) / (1000 * 60 * 60 * 24);
                guestData.ageInDays = ageInDays;
                
                if (ageInDays < 1) {
                    guestsByAge.good.push(guestData);
                } else if (ageInDays <= scheduleThreshold) {
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
        const recentCoverage = guestsByAge.good.length + guestsByAge.ok.length;
        
        // Calculate daily average
        const daysInMonth = daysWithBackups.size || 1;
        const dailyAverage = Math.round(backupsThisMonth / daysInMonth);
        
        // Detect backup window
        let backupWindow = 'Unknown';
        if (backupHours.length > 0) {
            // Find most common hour
            const hourCounts = {};
            backupHours.forEach(hour => {
                hourCounts[hour] = (hourCounts[hour] || 0) + 1;
            });
            const sortedHours = Object.entries(hourCounts).sort((a, b) => b[1] - a[1]);
            const primaryHour = parseInt(sortedHours[0][0]);
            
            // Check if there's a clear window
            let consecutiveHours = [primaryHour];
            sortedHours.forEach(([hour, count]) => {
                const h = parseInt(hour);
                if (count > hourCounts[primaryHour] * 0.3 && // At least 30% of primary hour
                    (Math.abs(h - primaryHour) <= 2 || Math.abs(h - primaryHour) >= 22)) { // Within 2 hours
                    consecutiveHours.push(h);
                }
            });
            
            const minHour = Math.min(...consecutiveHours);
            const maxHour = Math.max(...consecutiveHours);
            
            // Format time window
            const formatHour = (h) => {
                const period = h >= 12 ? 'PM' : 'AM';
                const hour12 = h > 12 ? h - 12 : (h === 0 ? 12 : h);
                return `${hour12}${period}`;
            };
            
            if (maxHour - minHour <= 3 || maxHour - minHour >= 21) {
                backupWindow = `${formatHour(minHour)}-${formatHour((maxHour + 1) % 24)}`;
            } else {
                backupWindow = `~${formatHour(primaryHour)}`;
            }
        }
        
        // Check if all namespaces are the same
        const allSameNamespace = namespaceStats.size <= 1;
        
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
                    ${healthScore < 100 ? `
                    <div class="grid grid-cols-4 gap-2 text-[10px]">
                        <div class="text-center">
                            <div class="font-semibold text-green-600 dark:text-green-400">${guestsByAge.good.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">&lt;24h</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-blue-600 dark:text-blue-400">${guestsByAge.ok.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">&lt;${Math.ceil(scheduleThreshold)}d</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-orange-600 dark:text-orange-400">${guestsByAge.warning.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">${Math.ceil(scheduleThreshold)}-${Math.ceil(criticalAge)}d</div>
                        </div>
                        <div class="text-center">
                            <div class="font-semibold text-red-600 dark:text-red-400">${guestsByAge.critical.length + guestsByAge.none.length}</div>
                            <div class="text-gray-500 dark:text-gray-400">${guestsByAge.none.length > 0 ? 'Old/None' : `>${Math.ceil(criticalAge)}d`}</div>
                        </div>
                    </div>
                    ` : ''}
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
                                            ${guest.namespaces && guest.namespaces.length > 0 ? 
                                                `<span class="text-[8px] text-purple-600 dark:text-purple-400">[${guest.namespaces.join(', ')}]</span>` : 
                                                ''
                                            }
                                        </div>
                                        <div class="flex items-center gap-2 ml-2">
                                            <div class="flex items-center gap-1 text-[9px]">
                                                ${guest.mostRecentBackupType === 'pbs' ? '<span class="text-purple-600 dark:text-purple-400 font-medium">PBS</span>' : ''}
                                                ${guest.mostRecentBackupType === 'pve' ? '<span class="text-orange-600 dark:text-orange-400 font-medium">PVE</span>' : ''}
                                                ${guest.mostRecentBackupType === 'snapshot' ? '<span class="text-yellow-600 dark:text-yellow-400 font-medium">SNAP</span>' : ''}
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
                    
                    <!-- Healthy Status Message or Guest List -->
                    ${healthScore === 100 ? `
                        <div class="flex items-center justify-center py-8 text-center">
                            <div>
                                <svg class="w-12 h-12 text-green-500 dark:text-green-400 mx-auto mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                </svg>
                                <p class="text-sm font-medium text-green-700 dark:text-green-400">All ${stats.totalGuests} guests backed up successfully</p>
                                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">${backupSchedule.charAt(0).toUpperCase() + backupSchedule.slice(1)} schedule • ${backupWindow}</p>
                            </div>
                        </div>
                    ` : guestsByAge.good.length > 0 ? `
                        <div class="mb-2">
                            <div class="flex items-center justify-between mb-1">
                                <h4 class="text-[10px] font-semibold text-green-700 dark:text-green-400 uppercase tracking-wider">Recent (${guestsByAge.good.length})</h4>
                                ${guestsByAge.good.length > 5 ? `
                                    <button onclick="const list = this.parentElement.parentElement.querySelector('.space-y-0\\\\.5'); const hidden = list.querySelector('.hidden'); if(hidden) { hidden.classList.remove('hidden'); this.textContent = 'Show Less'; } else { list.querySelector('div:last-child').classList.add('hidden'); this.textContent = 'Show All'; }" 
                                        class="text-[9px] text-blue-600 dark:text-blue-400 hover:underline">
                                        Show All
                                    </button>
                                ` : ''}
                            </div>
                            <div class="space-y-0.5">
                                ${guestsByAge.good.slice(0, 5).map(guest => `
                                    <div class="flex items-center justify-between px-1 py-0.5 rounded text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30">
                                        <div class="flex items-center gap-1 flex-1 min-w-0">
                                            <span class="text-[9px] font-medium ${guest.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.type}</span>
                                            <span class="font-mono text-gray-600 dark:text-gray-400">${guest.id}</span>
                                            <span class="truncate text-gray-700 dark:text-gray-300">${guest.name}</span>
                                        </div>
                                        <div class="flex items-center gap-2 ml-2">
                                            <span class="text-green-600 dark:text-green-400 font-medium">${formatAge(guest.ageInDays)}</span>
                                        </div>
                                    </div>
                                `).join('')}
                                ${guestsByAge.good.length > 5 ? `
                                    <div class="hidden">
                                        ${guestsByAge.good.slice(5).map(guest => `
                                            <div class="flex items-center justify-between px-1 py-0.5 rounded text-[11px] hover:bg-gray-50 dark:hover:bg-gray-700/30">
                                                <div class="flex items-center gap-1 flex-1 min-w-0">
                                                    <span class="text-[9px] font-medium ${guest.type === 'VM' ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${guest.type}</span>
                                                    <span class="font-mono text-gray-600 dark:text-gray-400">${guest.id}</span>
                                                    <span class="truncate text-gray-700 dark:text-gray-300">${guest.name}</span>
                                                </div>
                                                <div class="flex items-center gap-2 ml-2">
                                                    <span class="text-green-600 dark:text-green-400 font-medium">${formatAge(guest.ageInDays)}</span>
                                                </div>
                                            </div>
                                        `).join('')}
                                    </div>
                                ` : ''}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Summary Stats -->
                    <div class="mt-3 pt-2 border-t border-gray-200 dark:border-gray-700">
                        <div class="space-y-2">
                            <!-- Schedule and Last Backup -->
                            <div class="text-[9px] text-gray-600 dark:text-gray-400">
                                <div class="flex justify-between items-center mb-1">
                                    <span>Schedule:</span>
                                    <span class="font-medium ${backupSchedule === 'daily' ? 'text-green-600 dark:text-green-400' : backupSchedule === 'weekly' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-600 dark:text-gray-400'}">\n                                        ${backupSchedule.charAt(0).toUpperCase() + backupSchedule.slice(1)} backups
                                    </span>
                                </div>
                                <div class="flex justify-between items-center">
                                    <span>Last Backup:</span>
                                    <span class="font-medium">${lastBackupTime ? formatTimeAgo(lastBackupTime) : 'Never'}</span>
                                </div>
                            </div>
                            
                            <!-- Key Metrics -->
                            <div class="grid grid-cols-3 gap-2 text-[10px]">
                                <div class="text-center">
                                    <div class="font-semibold text-blue-600 dark:text-blue-400">~${dailyAverage}</div>
                                    <div class="text-gray-500 dark:text-gray-400">Per Day</div>
                                </div>
                                <div class="text-center">
                                    <div class="font-semibold text-purple-600 dark:text-purple-400">${backupWindow}</div>
                                    <div class="text-gray-500 dark:text-gray-400">Window</div>
                                </div>
                                <div class="text-center">
                                    <div class="font-semibold ${recentCoverage === stats.totalGuests ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'}">${recentCoverage}/${stats.totalGuests}</div>
                                    <div class="text-gray-500 dark:text-gray-400">Coverage</div>
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
                            
                            <!-- Coverage and Failures -->
                            <div class="text-[9px] text-gray-600 dark:text-gray-400">
                                <div class="flex justify-between items-center mb-1">
                                    <span>Coverage:</span>
                                    <span>${stats.totalGuests - guestsByAge.none.length}/${stats.totalGuests} guests protected</span>
                                </div>
                                ${recentFailures > 0 ? `
                                <div class="flex justify-between items-center">
                                    <span>Recent Failures:</span>
                                    <span class="text-red-600 dark:text-red-400 font-medium">${recentFailures} tasks failed</span>
                                </div>` : ''}
                            </div>
                            
                            <!-- Namespace Distribution -->
                            ${namespaceStats.size > 1 ? `
                                <div class="text-[9px] text-gray-600 dark:text-gray-400">
                                    <div class="flex items-center justify-between mb-1">
                                        <span class="font-medium">Namespaces:</span>
                                    </div>
                                    <div class="flex gap-1 flex-wrap">
                                        ${Array.from(namespaceStats.entries())
                                            .sort((a, b) => b[1] - a[1])
                                            .map(([ns, count]) => {
                                                const percentage = Math.round((count / stats.totalGuests) * 100);
                                                return `
                                                    <div class="flex items-center gap-1 px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/30 rounded">
                                                        <span class="text-purple-700 dark:text-purple-300 font-medium">${ns}</span>
                                                        <span class="text-purple-600 dark:text-purple-400">${count}</span>
                                                    </div>
                                                `;
                                            }).join('')}
                                    </div>
                                </div>
                            ` : ''}
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
        }
        
        if (ageInDays === Infinity) return 'Never';
        if (ageInDays < 1) return `${Math.floor(ageInDays * 24)}h`;
        if (ageInDays < 7) return `${Math.floor(ageInDays)}d`;
        if (ageInDays < 30) return `${Math.floor(ageInDays / 7)}w`;
        return `${Math.floor(ageInDays / 30)}mo`;
    }
    
    function formatTimeAgo(timestamp) {
        const now = Math.floor(Date.now() / 1000);
        const diff = now - timestamp;
        
        if (diff < 60) return 'Just now';
        if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)} hours ago`;
        if (diff < 604800) return `${Math.floor(diff / 86400)} days ago`;
        if (diff < 2592000) return `${Math.floor(diff / 604800)} weeks ago`;
        return `${Math.floor(diff / 2592000)} months ago`;
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