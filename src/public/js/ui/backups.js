PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let backupsSearchInput = null;
    let resetBackupsButton = null;
    let backupsTabContent = null;
    
    // Cache for expensive data transformations
    let dataCache = {
        lastStateHash: null,
        processedBackupData: null,
        guestBackupStatus: null
    };
    
    function _generateStateHash(vmsData, containersData, pbsDataArray, pveBackups) {
        // Simple hash based on data lengths and timestamps
        const vmCount = vmsData.length;
        const ctCount = containersData.length;
        const pbsCount = pbsDataArray.length;
        const pveTaskCount = pveBackups?.backupTasks?.length || 0;
        const pveStorageCount = pveBackups?.storageBackups?.length || 0;
        
        return `${vmCount}-${ctCount}-${pbsCount}-${pveTaskCount}-${pveStorageCount}`;
    }

    function _initMobileScrollIndicators() {
        const tableContainer = document.querySelector('#backups .table-container');
        const scrollHint = document.querySelector('#backups .scroll-hint');
        
        if (!tableContainer || !scrollHint) return;
        
        let scrollHintTimer;
        
        // Hide scroll hint after 5 seconds or on first scroll
        const hideScrollHint = () => {
            if (scrollHint) {
                scrollHint.style.display = 'none';
            }
        };
        
        scrollHintTimer = setTimeout(hideScrollHint, 5000);
        
        // Handle scroll events
        tableContainer.addEventListener('scroll', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
        
        // Also hide on table container click/touch
        tableContainer.addEventListener('touchstart', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
    }

    function init() {
        backupsSearchInput = document.getElementById('backups-search');
        resetBackupsButton = document.getElementById('reset-backups-filters-button');
        backupsTabContent = document.getElementById('backups');

        if (backupsSearchInput) {
            const debouncedUpdate = PulseApp.utils.debounce(() => updateBackupsTab(true), 300);
            backupsSearchInput.addEventListener('input', debouncedUpdate);
        } else {
            console.warn('Element #backups-search not found - backups text filtering disabled.');
        }

        if (resetBackupsButton) {
            resetBackupsButton.addEventListener('click', resetBackupsView);
        }

        if (backupsTabContent) {
            backupsTabContent.addEventListener('keydown', (event) => {
                if (event.key === 'Escape' && backupsTabContent.contains(document.activeElement)) {
                    resetBackupsView();
                }
            });
        }
        
        // Add event listeners for filter changes
        const filterElements = [
            'backups-filter-type-all', 'backups-filter-type-vm', 'backups-filter-type-ct',
            'backups-filter-backup-all', 'backups-filter-backup-pbs', 'backups-filter-backup-pve', 'backups-filter-backup-snapshots',
            'backups-filter-status-all', 'backups-filter-status-ok', 'backups-filter-status-stale', 'backups-filter-status-warning', 'backups-filter-status-none',
            'backups-filter-failures'
        ];
        
        filterElements.forEach(id => {
            const element = document.getElementById(id);
            if (element) {
                element.addEventListener('change', () => updateBackupsTab(true));
            }
        });
        
        // Global ESC key handler for backups tab
        document.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') {
                // Check if backups tab is currently active
                const backupsTab = document.querySelector('.tab[data-tab="backups"]');
                const isBackupsTabActive = backupsTab && backupsTab.classList.contains('active');
                
                if (isBackupsTabActive) {
                    // Check if there are any active filters to clear
                    const calendarFilter = PulseApp.state.get('calendarDateFilter');
                    const searchTerm = backupsSearchInput ? backupsSearchInput.value : '';
                    const typeFilter = PulseApp.state.get('backupsFilterGuestType');
                    const healthFilter = PulseApp.state.get('backupsFilterHealth');
                    const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType');
                    const failuresFilter = PulseApp.state.get('backupsFilterFailures');
                    
                    const hasActiveFilters = calendarFilter || 
                                           searchTerm || 
                                           (typeFilter && typeFilter !== 'all') || 
                                           (healthFilter && healthFilter !== 'all') ||
                                           (backupTypeFilter && backupTypeFilter !== 'all') ||
                                           failuresFilter;
                    
                    if (hasActiveFilters) {
                        event.preventDefault();
                        resetBackupsView();
                    }
                }
            }
        });
        
        // Initialize mobile scroll indicators
        if (window.innerWidth < 768) {
            _initMobileScrollIndicators();
        }
        
        // Initialize snapshot modal handlers
        _initSnapshotModal();
    }

    function calculateBackupSummary(backupStatusByGuest) {
        let totalGuests = backupStatusByGuest.length;
        let healthyCount = 0;
        let warningCount = 0;
        let errorCount = 0;
        let noneCount = 0;
        let totalPbsBackups = 0;
        let totalPveBackups = 0;
        let totalSnapshots = 0;

        backupStatusByGuest.forEach(guest => {
            
            switch (guest.backupHealthStatus) {
                case 'ok':
                case 'stale':
                    healthyCount++;
                    break;
                case 'old':
                    warningCount++;
                    break;
                case 'failed':
                    errorCount++;
                    break;
                case 'none':
                    noneCount++;
                    break;
            }
            
            totalPbsBackups += guest.pbsBackups || 0;
            totalPveBackups += guest.pveBackups || 0;
            totalSnapshots += guest.snapshotCount || 0;
        });

        return {
            totalGuests,
            healthyCount,
            warningCount,
            errorCount,
            noneCount,
            totalPbsBackups,
            totalPveBackups,
            totalSnapshots,
            healthyPercent: totalGuests > 0 ? (healthyCount / totalGuests) * 100 : 0
        };
    }

    
    function formatTimeAgo(milliseconds) {
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (days > 0) return `${days}d ago`;
        if (hours > 0) return `${hours}h ago`;
        if (minutes > 0) return `${minutes}m ago`;
        return 'Just now';
    }

    function createNodeBackupSummaryCard(nodeName, guestStatuses) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1';
        
        let healthyCount = 0;
        let warningCount = 0;
        let errorCount = 0;
        let noneCount = 0;
        let pbsTotal = 0;
        let pveTotal = 0;
        let snapshotTotal = 0;
        
        guestStatuses.forEach(guest => {
            switch (guest.backupHealthStatus) {
                case 'ok':
                case 'stale':
                    healthyCount++;
                    break;
                case 'old':
                    warningCount++;
                    break;
                case 'failed':
                    errorCount++;
                    break;
                case 'none':
                    noneCount++;
                    break;
            }
            pbsTotal += guest.pbsBackups || 0;
            pveTotal += guest.pveBackups || 0;
            snapshotTotal += guest.snapshotCount || 0;
        });
        
        const totalGuests = guestStatuses.length;
        const healthyPercent = totalGuests > 0 ? (healthyCount / totalGuests) * 100 : 0;
        
        // Sort guests by backup health (worst first for visibility)
        const sortedGuests = [...guestStatuses].sort((a, b) => {
            const priority = { 'failed': 0, 'none': 1, 'old': 2, 'stale': 3, 'ok': 4 };
            return priority[a.backupHealthStatus] - priority[b.backupHealthStatus];
        }); // Show all guests
        
        card.innerHTML = `
            <div class="flex justify-between items-center">
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate">${nodeName}</h3>
                <span class="text-xs text-gray-500 dark:text-gray-400">${totalGuests} guest${totalGuests > 1 ? 's' : ''}</span>
            </div>
            <div class="flex items-center gap-2 text-[10px] text-gray-600 dark:text-gray-400">
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-yellow-500 rounded-sm"></div>
                    <span>${snapshotTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-orange-500 rounded-sm"></div>
                    <span>${pveTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-purple-500 rounded-sm"></div>
                    <span>${pbsTotal}</span>
                </div>
            </div>
            ${sortedGuests.map(guest => {
                const statusColor = {
                    'ok': 'text-green-600 dark:text-green-400',
                    'stale': 'text-green-600 dark:text-green-400', 
                    'old': 'text-yellow-600 dark:text-yellow-400',
                    'failed': 'text-red-600 dark:text-red-400',
                    'none': 'text-gray-600 dark:text-gray-400'
                }[guest.backupHealthStatus] || 'text-gray-600 dark:text-gray-400';
                
                const statusIcon = {
                    'ok': '●',
                    'stale': '●',
                    'old': '●',
                    'failed': '●',
                    'none': '○'
                }[guest.backupHealthStatus] || '○';
                
                return `
                    <div class="text-[10px] text-gray-600 dark:text-gray-400 flex items-center gap-1">
                        <span class="${statusColor}">${statusIcon}</span>
                        <span class="truncate flex-1">${guest.guestName}</span>
                        <span class="text-[9px]">${guest.guestId}</span>
                    </div>
                `;
            }).join('')}
        `;
        
        return card;
    }

    function _extractBackupTypeFromVolid(volid, vmid) {
        // Extract type from volid format: vzdump-{type}-{vmid}-{timestamp}
        const volidMatch = volid.match(/vzdump-(qemu|lxc)-(\d+)-/);
        return volidMatch ? (volidMatch[1] === 'qemu' ? 'vm' : 'ct') : (vmid ? 'ct' : 'vm');
    }

    function _getInitialBackupData() {
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const initialDataReceived = PulseApp.state.get('initialDataReceived');
        
        // Check if we can use cached data
        const currentHash = _generateStateHash(vmsData, containersData, pbsDataArray, pveBackups);
        if (dataCache.lastStateHash === currentHash && dataCache.processedBackupData) {
            return dataCache.processedBackupData;
        }
        
        const allGuests = [...vmsData, ...containersData];

        // Combine PBS and PVE backup tasks
        const pbsBackupTasks = pbsDataArray.flatMap(pbs => {
            return (pbs.backupTasks?.recentTasks || []).map(task => ({
                ...task,
                guestId: task.id?.split('/')[1] || task.guestId || null,
                guestTypePbs: task.id?.split('/')[0] || task.guestType || null,
                pbsInstanceName: pbs.pbsInstanceName,
                source: 'pbs'
            }));
        });

        const pveBackupTasks = (pveBackups.backupTasks || []).map(task => ({
            ...task,
            guestId: task.guestId || task.vmid || null,
            guestTypePbs: task.guestType || task.type || null,
            startTime: task.starttime,
            source: 'pve'
        }));

        const allRecentBackupTasks = [...pbsBackupTasks, ...pveBackupTasks];
        
        
        // Debug failed tasks
        const failedTasks = allRecentBackupTasks.filter(task => task.status !== 'OK');

        // Combine PBS snapshots and PVE storage backups
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    backupType: snap['backup-type'],
                    backupVMID: snap['backup-id'],
                    source: 'pbs'
                }))
            )
        );

        const pveStorageBackups = (pveBackups.storageBackups || []).map(backup => ({
            'backup-time': backup.ctime,
            backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
            backupVMID: backup.vmid,
            vmid: backup.vmid, // Ensure vmid is preserved for filtering
            size: backup.size,
            protected: backup.protected,
            storage: backup.storage,
            volid: backup.volid,
            node: backup.node,
            endpointId: backup.endpointId,
            source: 'pve'
        }));

        const allSnapshots = [...pbsSnapshots, ...pveStorageBackups];

        // Pre-index data by guest ID and type for performance
        const tasksByGuest = new Map();
        const snapshotsByGuest = new Map();

        allRecentBackupTasks.forEach(task => {
            // Include node/endpointId in key to handle multiple clusters with same vmid
            const nodeKey = task.node ? `-${task.node}` : (task.endpointId ? `-${task.endpointId}` : '');
            const key = `${task.guestId}-${task.guestTypePbs}${nodeKey}`;
            if (!tasksByGuest.has(key)) tasksByGuest.set(key, []);
            tasksByGuest.get(key).push(task);
        });
        

        allSnapshots.forEach(snap => {
            // Different key strategies for PBS vs PVE:
            // PBS: centralized storage, use vmid-type (accessible by any guest with same vmid)
            // PVE: node-specific storage, use vmid-type-node
            let key;
            if (snap.source === 'pbs') {
                key = `${snap.backupVMID}-${snap.backupType}`;
            } else {
                // PVE backups are node-specific
                const nodeKey = snap.node ? `-${snap.node}` : (snap.endpointId ? `-${snap.endpointId}` : '');
                key = `${snap.backupVMID}-${snap.backupType}${nodeKey}`;
            }
            if (!snapshotsByGuest.has(key)) snapshotsByGuest.set(key, []);
            snapshotsByGuest.get(key).push(snap);
        });

        // Pre-calculate day boundaries for 7-day analysis
        const now = new Date();
        now.setHours(0, 0, 0, 0);
        const dayBoundaries = [];
        for (let i = 6; i >= 0; i--) {
            const dayStart = new Date(now);
            dayStart.setDate(now.getDate() - i);
            const dayEnd = new Date(dayStart);
            dayEnd.setDate(dayStart.getDate() + 1);
            dayBoundaries.push({
                start: Math.floor(dayStart.getTime() / 1000),
                end: Math.floor(dayEnd.getTime() / 1000)
            });
        }

        const threeDaysAgo = Math.floor(new Date(now).setDate(now.getDate() - 3) / 1000);
        const sevenDaysAgo = Math.floor(new Date(now).setDate(now.getDate() - 7) / 1000);

        const result = { 
            allGuests, 
            initialDataReceived, 
            tasksByGuest, 
            snapshotsByGuest, 
            dayBoundaries,
            threeDaysAgo,
            sevenDaysAgo
        };
        
        // Cache the processed data
        dataCache.lastStateHash = currentHash;
        dataCache.processedBackupData = result;
        
        return result;
    }

    function _determineGuestBackupStatus(guest, guestSnapshots, guestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo) {
        const guestId = String(guest.vmid);
        
        // Get guest snapshots from pveBackups - use node-aware filtering
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const allSnapshots = pveBackups.guestSnapshots || [];
        const guestSnapshotCount = allSnapshots
            .filter(snap => {
                // Match vmid
                if (parseInt(snap.vmid, 10) !== parseInt(guest.vmid, 10)) return false;
                
                // For VM/CT snapshots, match by node/endpoint if available
                if (guest.node && snap.node) {
                    return snap.node === guest.node;
                }
                if (guest.endpointId && snap.endpointId) {
                    return snap.endpointId === guest.endpointId;
                }
                
                // Fallback: include if no node info available
                return true;
            })
            .length;
        
        // Use pre-filtered data instead of filtering large arrays
        const totalBackups = guestSnapshots ? guestSnapshots.length : 0;
        const latestSnapshot = guestSnapshots && guestSnapshots.length > 0 
            ? guestSnapshots.reduce((latest, snap) => {
                return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
            }, null)
            : null;
        const latestSnapshotTime = latestSnapshot ? latestSnapshot['backup-time'] : null;

        const latestTask = guestTasks && guestTasks.length > 0
            ? guestTasks.reduce((latest, task) => {
               return (!latest || (task.startTime && task.startTime > latest.startTime)) ? task : latest;
            }, null)
            : null;

        let healthStatus = 'none';
        let displayTimestamp = latestSnapshotTime;

        if (latestTask) {
            displayTimestamp = latestTask.startTime;
            if (latestTask.status === 'OK') {
                if (latestTask.startTime >= threeDaysAgo) healthStatus = 'ok';
                else if (latestTask.startTime >= sevenDaysAgo) healthStatus = 'stale';
                else healthStatus = 'old';
            } else {
                healthStatus = 'failed';
            }
        } else if (latestSnapshotTime) {
             if (latestSnapshotTime >= threeDaysAgo) healthStatus = 'ok';
             else if (latestSnapshotTime >= sevenDaysAgo) healthStatus = 'stale';
             else healthStatus = 'old';
        } else {
            healthStatus = 'none';
            displayTimestamp = null;
        }

        // Calculate recent failures by analyzing actual PBS and PVE backup tasks
        let recentFailures = 0;
        let lastFailureTime = null;
        
        // Get recent failure window (7 days)
        const now = Math.floor(Date.now() / 1000);
        const sevenDaysAgoForFailures = now - (7 * 24 * 60 * 60);
        
        
        if (guestTasks && guestTasks.length > 0) {
            // Analyze actual backup tasks for failures in the last 7 days
            const recentFailedTasks = guestTasks.filter(task => {
                // Include tasks from the last 7 days that failed
                return task.startTime >= sevenDaysAgoForFailures && 
                       task.status !== 'OK' && 
                       task.status !== null && 
                       task.status !== undefined;
            });
            
            recentFailures = recentFailedTasks.length;
            
            
            // Find the most recent failure timestamp
            if (recentFailedTasks.length > 0) {
                const latestFailedTask = recentFailedTasks.reduce((latest, task) => {
                    return (task.startTime > latest.startTime) ? task : latest;
                });
                lastFailureTime = latestFailedTask.startTime;
            }
        }
        
        // If no task-based failures found but health status indicates failure, count as 1
        if (recentFailures === 0 && healthStatus === 'failed') {
            recentFailures = 1;
            lastFailureTime = displayTimestamp;
            
        }

        // Enhanced 7-day backup status calculation with backup type tracking
        const last7DaysBackupStatus = dayBoundaries.map((day, index) => {
            let backupTypes = new Set();
            let hasFailures = false;
            let activityDetails = [];

            // Check tasks for this day - using pre-filtered guest tasks
            if (guestTasks) {
                const failedTasksOnThisDay = guestTasks.filter(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status !== 'OK'
                );
                const successfulTasksOnThisDay = guestTasks.filter(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status === 'OK'
                );

                // Track successful backup types
                successfulTasksOnThisDay.forEach(task => {
                    const source = task.source === 'pbs' ? 'PBS' : 'PVE';
                    const location = task.source === 'pbs' ? task.pbsInstanceName : 'Local';
                    backupTypes.add(task.source);
                    activityDetails.push(`✓ ${source} backup${location ? ` (${location})` : ''}`);
                });

                // Track failed backup attempts
                failedTasksOnThisDay.forEach(task => {
                    const source = task.source === 'pbs' ? 'PBS' : 'PVE';
                    const location = task.source === 'pbs' ? task.pbsInstanceName : 'Local';
                    hasFailures = true;
                    activityDetails.push(`✗ ${source} backup failed${location ? ` (${location})` : ''}`);
                });
            }

            // Check for backup storage activity (snapshots/backups created)
            if (guestSnapshots) {
                const snapshotsOnThisDay = guestSnapshots.filter(
                    snap => snap['backup-time'] >= day.start && snap['backup-time'] < day.end
                );
                
                snapshotsOnThisDay.forEach(snap => {
                    if (snap.source === 'pbs') {
                        backupTypes.add('pbs');
                        activityDetails.push(`✓ PBS backup stored (${snap.pbsInstanceName})`);
                    } else if (snap.source === 'pve') {
                        backupTypes.add('pve');
                        activityDetails.push(`✓ PVE backup stored (${snap.storage || 'Local'})`);
                    }
                });
            }

            // Check for VM/CT snapshots on this day (if we have that data)
            const pveBackups = PulseApp.state.get('pveBackups') || {};
            const allSnapshots = pveBackups.guestSnapshots || [];
            const guestDaySnapshots = allSnapshots.filter(snap => {
                // Match vmid and time
                if (parseInt(snap.vmid, 10) !== parseInt(guestId, 10)) return false;
                if (!(snap.snaptime >= day.start && snap.snaptime < day.end)) return false;
                
                // Match by node/endpoint if available
                if (guest.node && snap.node) {
                    return snap.node === guest.node;
                }
                if (guest.endpointId && snap.endpointId) {
                    return snap.endpointId === guest.endpointId;
                }
                
                // Fallback: include if no node info available
                return true;
            });
            
            if (guestDaySnapshots.length > 0) {
                backupTypes.add('snapshot');
                activityDetails.push(`✓ ${guestDaySnapshots.length} VM/CT snapshot${guestDaySnapshots.length > 1 ? 's' : ''} created`);
            }

            // Create day label for tooltip
            const dayDate = new Date(day.start * 1000);
            const dayLabel = dayDate.toLocaleDateString('en-US', { 
                weekday: 'short', 
                month: 'short', 
                day: 'numeric' 
            });

            return {
                backupTypes: Array.from(backupTypes),
                hasFailures: hasFailures,
                details: activityDetails.length > 0 ? activityDetails.join('\n') : 'No backup activity',
                date: dayLabel
            };
        });

        // Calculate separate PBS and PVE backup information
        let pbsBackupCount = 0;
        let pbsBackupInfo = '';
        let pveBackupCount = 0; 
        let pveBackupInfo = '';
        
        if (guestSnapshots && guestSnapshots.length > 0) {
            // Separate PBS and PVE snapshots
            const pbsSnapshots = guestSnapshots.filter(s => s.source === 'pbs');
            const pveSnapshots = guestSnapshots.filter(s => s.source === 'pve');
            
            // Calculate PBS backup information
            if (pbsSnapshots.length > 0) {
                pbsBackupCount = pbsSnapshots.length;
                const pbsInstances = [...new Set(pbsSnapshots.map(s => s.pbsInstanceName).filter(Boolean))];
                const datastores = [...new Set(pbsSnapshots.map(s => s.datastoreName).filter(Boolean))];
                
                // Group backups by PBS instance for detailed info
                const backupsByPbs = {};
                pbsSnapshots.forEach(snap => {
                    if (snap.pbsInstanceName) {
                        if (!backupsByPbs[snap.pbsInstanceName]) {
                            backupsByPbs[snap.pbsInstanceName] = { count: 0, datastores: new Set() };
                        }
                        backupsByPbs[snap.pbsInstanceName].count++;
                        if (snap.datastoreName) {
                            backupsByPbs[snap.pbsInstanceName].datastores.add(snap.datastoreName);
                        }
                    }
                });
                
                if (pbsInstances.length === 1) {
                    pbsBackupInfo = `${pbsInstances[0]} (${datastores.join(', ')})`;
                } else if (pbsInstances.length > 1) {
                    const details = pbsInstances.map(pbs => {
                        const info = backupsByPbs[pbs];
                        const dsArray = Array.from(info.datastores);
                        return `${pbs}: ${info.count} on ${dsArray.join(', ')}`;
                    }).join(' | ');
                    pbsBackupInfo = details;
                }
            }
            
            // Calculate PVE backup information
            if (pveSnapshots.length > 0) {
                pveBackupCount = pveSnapshots.length;
                const storages = [...new Set(pveSnapshots.map(s => s.storage).filter(Boolean))];
                
                if (storages.length === 1) {
                    pveBackupInfo = storages[0];
                } else if (storages.length > 1) {
                    pveBackupInfo = storages.join(', ');
                } else {
                    pveBackupInfo = 'Local storage';
                }
            }
        }
        
        // Include task data for counts if no snapshots but tasks exist
        if (guestTasks && guestTasks.length > 0) {
            const pbsTasks = guestTasks.filter(t => t.source === 'pbs');
            const pveTasks = guestTasks.filter(t => t.source === 'pve');
            
            if (pbsBackupCount === 0 && pbsTasks.length > 0) {
                // No PBS snapshots but have PBS tasks - show as recent activity
                const pbsInstances = [...new Set(pbsTasks.map(t => t.pbsInstanceName).filter(Boolean))];
                if (pbsInstances.length > 0) {
                    pbsBackupInfo = `Recent activity on ${pbsInstances.join(', ')}`;
                }
            }
            
            if (pveBackupCount === 0 && pveTasks.length > 0) {
                // No PVE backups but have PVE tasks - show as recent activity
                pveBackupInfo = 'Recent activity';
            }
        }

        
        return {
            guestName: guest.name || `Guest ${guest.vmid}`,
            guestId: guest.vmid,
            guestType: guest.type === 'qemu' ? 'VM' : 'LXC',
            node: guest.node,
            guestPveStatus: guest.status,
            latestBackupTime: displayTimestamp,
            pbsBackups: pbsBackupCount,
            pbsBackupInfo: pbsBackupInfo,
            pveBackups: pveBackupCount,
            pveBackupInfo: pveBackupInfo,
            totalBackups: totalBackups,
            backupHealthStatus: healthStatus,
            last7DaysBackupStatus: last7DaysBackupStatus,
            snapshotCount: guestSnapshotCount,
            recentFailures: recentFailures,
            lastFailureTime: lastFailureTime,
            endpointId: guest.endpointId
        };
    }

    function _filterBackupData(backupStatusByGuest, backupsSearchInput) {
        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value.toLowerCase() : '';
        const backupsSearchTerms = currentBackupsSearchTerm.split(',').map(term => term.trim()).filter(term => term);
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');
        const backupsFilterBackupType = PulseApp.state.get('backupsFilterBackupType');
        const calendarDateFilter = PulseApp.state.get('calendarDateFilter');
        const showFailuresOnly = PulseApp.state.get('backupsFilterFailures') || false;

        // Debug logging for failures filter
        if (showFailuresOnly) {
            const guestsWithFailures = backupStatusByGuest.filter(item => item.recentFailures > 0);
        }

        return backupStatusByGuest.filter(item => {
            // Failures filter - show only guests with recent failures
            if (showFailuresOnly && item.recentFailures === 0) {
                return false;
            }

            const healthMatch = (backupsFilterHealth === 'all') ||
                                (backupsFilterHealth === 'ok' && (item.backupHealthStatus === 'ok' || item.backupHealthStatus === 'stale')) ||
                                (backupsFilterHealth === 'warning' && (item.backupHealthStatus === 'old')) ||
                                (backupsFilterHealth === 'none' && item.backupHealthStatus === 'none');
            if (!healthMatch) return false;

            const typeMatch = (backupsFilterGuestType === 'all') ||
                              (backupsFilterGuestType === 'vm' && item.guestType === 'VM') ||
                              (backupsFilterGuestType === 'lxc' && item.guestType === 'LXC');
            if (!typeMatch) return false;

            // Backup type filter - only show guests that have the specified backup type
            if (backupsFilterBackupType && backupsFilterBackupType !== 'all') {
                let hasBackupType = false;
                if (backupsFilterBackupType === 'pbs' && item.pbsBackups > 0) {
                    hasBackupType = true;
                } else if (backupsFilterBackupType === 'pve' && item.pveBackups > 0) {
                    hasBackupType = true;
                } else if (backupsFilterBackupType === 'snapshots' && item.snapshotCount > 0) {
                    hasBackupType = true;
                }
                if (!hasBackupType) return false;
            }

            // Calendar date filter - only show guests that had backups on the selected date
            if (calendarDateFilter && calendarDateFilter.guestIds && calendarDateFilter.guestIds.length > 0) {
                // Create unique key for this guest item
                const nodeIdentifier = item.node || item.endpointId || '';
                const itemUniqueKey = nodeIdentifier ? `${item.guestId}-${nodeIdentifier}` : item.guestId.toString();
                
                // Check if this guest's unique key or simple vmid is in the calendar filter
                const guestIdMatch = calendarDateFilter.guestIds.includes(itemUniqueKey) || 
                                   calendarDateFilter.guestIds.includes(item.guestId.toString());
                if (!guestIdMatch) return false;
            }

            if (backupsSearchTerms.length > 0) {
                return backupsSearchTerms.some(term =>
                    (item.guestName?.toLowerCase() || '').includes(term) ||
                    (item.node?.toLowerCase() || '').includes(term) ||
                    (item.guestId?.toString() || '').includes(term)
                );
            }
            return true;
        });
    }

    function createThresholdIndicator(guestStatus) {
        // Get current app state to check for custom thresholds
        const currentState = PulseApp.state.get();
        if (!currentState || !currentState.customThresholds) {
            return ''; // No custom thresholds data available
        }
        
        // Check if this guest has custom thresholds configured
        const hasCustomThresholds = currentState.customThresholds.some(config => 
            config.endpointId === guestStatus.endpointId && 
            config.nodeId === guestStatus.node && 
            config.vmid === guestStatus.guestId &&
            config.enabled
        );
        
        if (hasCustomThresholds) {
            return `
                <span class="inline-flex items-center justify-center w-3 h-3 text-xs font-bold text-white bg-blue-500 rounded-full" 
                      title="Custom alert thresholds configured">
                    T
                </span>
            `;
        }
        
        return '';
    }

    function _renderBackupTableRow(guestStatus) {
        const row = document.createElement('tr');
        row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        row.dataset.guestId = guestStatus.guestId;

        // Check if guest has custom thresholds
        const thresholdIndicator = createThresholdIndicator(guestStatus);

        const latestBackupFormatted = guestStatus.latestBackupTime
            ? PulseApp.utils.formatPbsTimestamp(guestStatus.latestBackupTime)
            : '<span class="text-gray-400">No backups found</span>';

        const typeIconClass = guestStatus.guestType === 'VM'
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;


        // Create PBS backup cell with visual indicator
        let pbsBackupCell = '';
        if (guestStatus.pbsBackups > 0) {
            const pbsIcon = '<span class="inline-block w-2 h-2 bg-purple-500 rounded-full mr-1" title="PBS Backup"></span>';
            pbsBackupCell = `<span class="text-purple-700 dark:text-purple-300" ${guestStatus.pbsBackupInfo ? `title="${guestStatus.pbsBackupInfo}"` : ''}>${pbsIcon}${guestStatus.pbsBackups}</span>`;
        } else {
            pbsBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create PVE backup cell with visual indicator  
        let pveBackupCell = '';
        if (guestStatus.pveBackups > 0) {
            const pveIcon = '<span class="inline-block w-2 h-2 bg-orange-500 rounded-full mr-1" title="PVE Backup"></span>';
            pveBackupCell = `<span class="text-orange-700 dark:text-orange-300" ${guestStatus.pveBackupInfo ? `title="${guestStatus.pveBackupInfo}"` : ''}>${pveIcon}${guestStatus.pveBackups}</span>`;
        } else {
            pveBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create snapshot button or count display
        let snapshotCell = '';
        if (guestStatus.snapshotCount > 0) {
            const snapshotIcon = '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full mr-1" title="VM/CT Snapshots"></span>';
            snapshotCell = `<button class="text-yellow-600 dark:text-yellow-400 hover:underline view-snapshots-btn" 
                data-vmid="${guestStatus.guestId}" 
                data-node="${guestStatus.node}"
                data-endpoint="${guestStatus.endpointId}"
                data-type="${guestStatus.guestType.toLowerCase()}">${snapshotIcon}${guestStatus.snapshotCount}</button>`;
        } else {
            snapshotCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create failures cell
        let failuresCell = '';
        if (guestStatus.recentFailures > 0) {
            const failureTime = guestStatus.lastFailureTime 
                ? PulseApp.utils.formatPbsTimestamp(guestStatus.lastFailureTime)
                : 'Unknown';
            failuresCell = `<span class="text-red-600 dark:text-red-400 font-medium" title="Last failure: ${failureTime}">${guestStatus.recentFailures}</span>`;
        } else {
            failuresCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        row.innerHTML = `
            <td class="sticky left-0 bg-white dark:bg-gray-800 z-10 p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 text-gray-900 dark:text-gray-100 border-r border-gray-300 dark:border-gray-600" title="${guestStatus.guestName}">
                <div class="flex items-center gap-1">
                    <span>${guestStatus.guestName}</span>
                    ${thresholdIndicator}
                </div>
            </td>
            <td class="p-1 px-2 text-gray-500 dark:text-gray-400">${guestStatus.guestId}</td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.node}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${latestBackupFormatted}</td>
            <td class="p-1 px-2 text-center">${snapshotCell}</td>
            <td class="p-1 px-2 text-center">${pveBackupCell}</td>
            <td class="p-1 px-2 text-center">${pbsBackupCell}</td>
            <td class="p-1 px-2 text-center">${failuresCell}</td>
        `;
        return row;
    }

    function _updateBackupStatusMessages(statusTextElement, visibleCount, backupsSearchInput) {
        if (!statusTextElement) return;

        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value : '';
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');

        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = currentBackupsSearchTerm ? ` | Filter: "${currentBackupsSearchTerm}"` : '';
        const typeFilterLabel = backupsFilterGuestType !== 'all' ? backupsFilterGuestType.toUpperCase() : '';
        const healthFilterLabel = backupsFilterHealth !== 'all' ? backupsFilterHealth.charAt(0).toUpperCase() + backupsFilterHealth.slice(1) : '';
        const otherFilters = [typeFilterLabel, healthFilterLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        const statusCountText = ` | Showing ${visibleCount} guests`;
        statusTextElement.textContent = statusBaseText + statusFilterText + statusCountText;
    }

    function _initTableCalendarClick() {
        const backupsTableBody = document.getElementById('backups-overview-tbody');
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        
        if (!backupsTableBody || !calendarContainer) return;
        
        // Get current filtered guest from state (persists across API updates)
        let currentFilteredGuest = PulseApp.state.get('currentFilteredGuest') || null;
        
        // Add click listeners to table rows
        const tableRows = backupsTableBody.querySelectorAll('tr[data-guest-id]');
        
        tableRows.forEach(row => {
            const guestId = row.dataset.guestId;
            
            // Add cursor pointer to indicate clickability
            row.style.cursor = 'pointer';
            
            // Restore visual indication if this row was previously selected
            if (currentFilteredGuest === guestId) {
                row.classList.add('bg-blue-50', 'dark:bg-blue-900/20');
                // Re-apply calendar filter on restore (not a user action)
                _filterCalendarToGuest(guestId, false);
            }
            
            row.addEventListener('click', () => {
                if (currentFilteredGuest === guestId) {
                    // Clicking the same row again resets the filter
                    _resetCalendarFilter();
                    currentFilteredGuest = null;
                    PulseApp.state.set('currentFilteredGuest', null);
                    // Remove visual indication
                    tableRows.forEach(r => r.classList.remove('bg-blue-50', 'dark:bg-blue-900/20'));
                } else {
                    // Filter to this guest
                    _filterCalendarToGuest(guestId);
                    currentFilteredGuest = guestId;
                    PulseApp.state.set('currentFilteredGuest', guestId);
                    // Add visual indication
                    tableRows.forEach(r => r.classList.remove('bg-blue-50', 'dark:bg-blue-900/20'));
                    row.classList.add('bg-blue-50', 'dark:bg-blue-900/20');
                }
            });
        });
        
        // If we had a filtered guest but the row no longer exists (e.g., due to filtering), clear the state
        if (currentFilteredGuest && !document.querySelector(`tr[data-guest-id="${currentFilteredGuest}"]`)) {
            PulseApp.state.set('currentFilteredGuest', null);
            _resetCalendarFilter();
        }
    }

    function _filterCalendarToGuest(guestId, isUserAction = true) {
        // Re-render the calendar with only this guest's data
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        if (!calendarContainer || !PulseApp.ui.calendarHeatmap) return;
        
        // Get the current backup data
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        // Get PBS snapshots
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    source: 'pbs'
                }))
            )
        );
        
        // Get PVE storage backups
        const pveStorageBackups = [];
        if (pveBackups?.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            pveBackups.storageBackups.forEach(backup => {
                pveStorageBackups.push({
                    'backup-time': backup.ctime,
                    backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
                    backupVMID: backup.vmid,
                    vmid: backup.vmid, // Ensure vmid is preserved for filtering
                    size: backup.size,
                    protected: backup.protected,
                    storage: backup.storage,
                    volid: backup.volid,
                    format: backup.format,
                    node: backup.node,
                    endpointId: backup.endpointId,
                    source: 'pve'
                });
            });
        }
        
        // Get VM snapshots
        const vmSnapshots = (pveBackups.guestSnapshots || []).map(snap => ({
            ...snap,
            source: 'vmSnapshots'
        }));
        
        // Get backup tasks
        const pbsBackupTasks = [];
        pbsDataArray.forEach(pbs => {
            if (pbs.backupTasks?.recentTasks && Array.isArray(pbs.backupTasks.recentTasks)) {
                pbs.backupTasks.recentTasks.forEach(task => {
                    pbsBackupTasks.push({
                        ...task,
                        pbsInstanceName: pbs.pbsInstanceName,
                        source: 'pbs'
                    });
                });
            }
        });
        
        const pveBackupTasks = [];
        if (Array.isArray(pveBackups?.backupTasks)) {
            pveBackups.backupTasks.forEach(task => {
                pveBackupTasks.push({
                    ...task,
                    source: 'pve'
                });
            });
        }
        
        const backupData = {
            pbsSnapshots: pbsSnapshots,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots,
            backupTasks: [...pbsBackupTasks, ...pveBackupTasks]
        };
        
        // Get detail card for callback
        const detailCardContainer = document.getElementById('backup-detail-card');
        let onDateSelect = null;
        
        if (detailCardContainer && PulseApp.ui.backupDetailCard) {
            // Find existing detail card or create callback
            const existingCard = detailCardContainer.querySelector('.bg-slate-800');
            if (existingCard) {
                onDateSelect = (dateData, instant = false) => {
                    PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, dateData, instant);
                };
                
                // Only auto-select today on initial filter, not on API updates
                // The calendar will handle the auto-selection
            }
        }
        
        // Reset filter when filtering to specific guest
        if (PulseApp.ui.calendarHeatmap.resetFilter) {
            PulseApp.ui.calendarHeatmap.resetFilter();
        }
        
        // Create filtered calendar for this specific guest
        const filteredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, guestId, [guestId], onDateSelect, isUserAction);
        calendarContainer.innerHTML = '';
        calendarContainer.appendChild(filteredCalendar);
    }
    
    function _resetCalendarFilter() {
        // Re-render the calendar with all filtered guests (respecting table filters)
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        if (!calendarContainer || !PulseApp.ui.calendarHeatmap) return;
        
        // Get current filtered backup status to determine which guests to show
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const { tasksByGuest, snapshotsByGuest, dayBoundaries, threeDaysAgo, sevenDaysAgo } = _getInitialBackupData();
        const backupStatusByGuest = allGuests.map(guest => {
            // Try both PBS (generic) and PVE (node-specific) keys
            const baseKey = `${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`;
            const nodeKey = guest.node ? `-${guest.node}` : (guest.endpointId ? `-${guest.endpointId}` : '');
            const nodeSpecificKey = `${baseKey}${nodeKey}`;
            
            // Get snapshots from both keys and combine them
            const pbsSnapshots = snapshotsByGuest.get(baseKey) || [];
            const pveSnapshots = snapshotsByGuest.get(nodeSpecificKey) || [];
            const allGuestSnapshots = [...pbsSnapshots, ...pveSnapshots];
            
            // Similar for tasks
            const pbsTasks = tasksByGuest.get(baseKey) || [];
            const pveTasks = tasksByGuest.get(nodeSpecificKey) || [];
            const allGuestTasks = [...pbsTasks, ...pveTasks];
            
            return _determineGuestBackupStatus(guest, allGuestSnapshots, allGuestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo);
        });
        const filteredBackupStatus = _filterBackupData(backupStatusByGuest, backupsSearchInput);
        
        // Get the current backup data
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        // Prepare backup data same as in updateBackupsTab
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    source: 'pbs'
                }))
            )
        );
        
        const pveStorageBackups = [];
        if (pveBackups?.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            pveBackups.storageBackups.forEach(backup => {
                pveStorageBackups.push({
                    'backup-time': backup.ctime,
                    backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
                    backupVMID: backup.vmid,
                    vmid: backup.vmid, // Ensure vmid is preserved for filtering
                    size: backup.size,
                    protected: backup.protected,
                    storage: backup.storage,
                    volid: backup.volid,
                    format: backup.format,
                    node: backup.node,
                    endpointId: backup.endpointId,
                    source: 'pve'
                });
            });
        }
        
        const vmSnapshots = (pveBackups.guestSnapshots || []).map(snap => ({
            ...snap,
            source: 'vmSnapshots'
        }));
        
        const pbsBackupTasks = [];
        pbsDataArray.forEach(pbs => {
            if (pbs.backupTasks?.recentTasks && Array.isArray(pbs.backupTasks.recentTasks)) {
                pbs.backupTasks.recentTasks.forEach(task => {
                    pbsBackupTasks.push({
                        ...task,
                        pbsInstanceName: pbs.pbsInstanceName,
                        source: 'pbs'
                    });
                });
            }
        });
        
        const pveBackupTasks = [];
        if (Array.isArray(pveBackups?.backupTasks)) {
            pveBackups.backupTasks.forEach(task => {
                pveBackupTasks.push({
                    ...task,
                    source: 'pve'
                });
            });
        }
        
        const backupData = {
            pbsSnapshots: pbsSnapshots,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots,
            backupTasks: [...pbsBackupTasks, ...pveBackupTasks]
        };
        
        // Create calendar respecting current table filters - use unique guest identifiers
        const filteredGuestIds = filteredBackupStatus.map(guest => {
            // Create unique identifier including node/endpoint to handle guests with same vmid on different nodes
            const nodeIdentifier = guest.node || guest.endpointId || '';
            return nodeIdentifier ? `${guest.guestId}-${nodeIdentifier}` : guest.guestId.toString();
        });
        // Get detail card for callback
        const detailCardContainer = document.getElementById('backup-detail-card');
        let onDateSelect = null;
        
        if (detailCardContainer && PulseApp.ui.backupDetailCard) {
            // Find existing detail card or create callback
            const existingCard = detailCardContainer.querySelector('.bg-slate-800');
            if (existingCard) {
                onDateSelect = (dateData, instant = false) => {
                    PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, dateData, instant);
                };
                
                // Clear the detail card when resetting filter
                PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, null, true);
            }
        }
        
        // Reset filter when restoring full calendar view  
        if (PulseApp.ui.calendarHeatmap.resetFilter) {
            PulseApp.ui.calendarHeatmap.resetFilter();
        }
        
        const restoredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, null, filteredGuestIds, onDateSelect);
        calendarContainer.innerHTML = '';
        calendarContainer.appendChild(restoredCalendar);
    }

    function checkGuestBackupForDate(backupData, guestId, dateKey) {
        const guestBackups = [];
        let hasBackups = false;
        
        // Get guest data for name lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(guestId, 10));
        
        if (!guest) return null;
        
        const guestInfo = {
            vmid: guestId,
            name: guest.name,
            type: guest.type === 'qemu' ? 'VM' : 'CT',
            types: [],
            backupCount: 0
        };
        
        // Check all backup sources for this guest on this date
        ['pbsSnapshots', 'pveBackups', 'vmSnapshots'].forEach(source => {
            if (!backupData[source]) return;
            
            const dayBackups = backupData[source].filter(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) return false;
                
                const date = new Date(timestamp * 1000);
                const utcDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
                const itemDateKey = utcDate.toISOString().split('T')[0];
                
                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                if (vmid != guestId || itemDateKey !== dateKey) return false;
                
                // For PBS backups (centralized), don't filter by node
                if (source === 'pbsSnapshots') return true;
                
                // For PVE backups and snapshots (node-specific), match node/endpoint
                const itemNode = item.node;
                const itemEndpoint = item.endpointId;
                
                // Match by node if available
                if (guest.node && itemNode) {
                    return itemNode === guest.node;
                }
                
                // Match by endpointId if available
                if (guest.endpointId && itemEndpoint) {
                    return itemEndpoint === guest.endpointId;
                }
                
                // If no node/endpoint info available, include it (fallback)
                return true;
            });
            
            if (dayBackups.length > 0) {
                hasBackups = true;
                guestInfo.types.push(source);
                guestInfo.backupCount += dayBackups.length;
            }
        });
        
        if (!hasBackups) return null;
        
        // Check for failures
        let hasFailures = false;
        if (backupData.backupTasks) {
            const dayTasks = backupData.backupTasks.filter(task => {
                if (!task.starttime) return false;
                
                // Match vmid
                const taskVmid = task.vmid || task.guestId;
                if (parseInt(taskVmid, 10) !== parseInt(guestId, 10)) return false;
                
                const date = new Date(task.starttime * 1000);
                const utcDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
                const taskDateKey = utcDate.toISOString().split('T')[0];
                
                if (taskDateKey !== dateKey || task.status === 'OK') return false;
                
                // For PBS tasks (centralized), don't filter by node
                if (task.source === 'pbs') return true;
                
                // For PVE tasks (node-specific), match node/endpoint
                const taskNode = task.node;
                const taskEndpoint = task.endpointId;
                
                // Match by node if available
                if (guest.node && taskNode) {
                    return taskNode === guest.node;
                }
                
                // Match by endpointId if available
                if (guest.endpointId && taskEndpoint) {
                    return taskEndpoint === guest.endpointId;
                }
                
                // If no node/endpoint info available, include it (fallback)
                return true;
            });
            
            hasFailures = dayTasks.length > 0;
        }
        
        guestBackups.push(guestInfo);
        
        // Count backup types for stats
        const stats = {
            totalGuests: 1,
            pbsCount: guestInfo.types.includes('pbsSnapshots') ? 1 : 0,
            pveCount: guestInfo.types.includes('pveBackups') ? 1 : 0,
            snapshotCount: guestInfo.types.includes('vmSnapshots') ? 1 : 0,
            failureCount: hasFailures ? 1 : 0
        };
        
        return {
            date: dateKey,
            backups: guestBackups,
            stats: stats
        };
    }

    function _dayHasGuestBackup(dateKey, guestId) {
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        const targetDate = new Date(dateKey);
        const startOfDay = new Date(targetDate.getFullYear(), targetDate.getMonth(), targetDate.getDate());
        const endOfDay = new Date(startOfDay.getTime() + 24 * 60 * 60 * 1000);
        const startTimestamp = Math.floor(startOfDay.getTime() / 1000);
        const endTimestamp = Math.floor(endOfDay.getTime() / 1000);
        
        // Check PBS snapshots
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).filter(snap => {
                    const vmid = snap['backup-id'];
                    const timestamp = snap['backup-time'];
                    return parseInt(vmid, 10) === parseInt(guestId, 10) && timestamp >= startTimestamp && timestamp < endTimestamp;
                })
            )
        );
        
        if (pbsSnapshots.length > 0) return true;
        
        // Check PVE storage backups
        if (pveBackups.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            const matchingBackups = pveBackups.storageBackups.filter(backup => {
                return parseInt(backup.vmid, 10) === parseInt(guestId, 10) && 
                       backup.ctime >= startTimestamp && 
                       backup.ctime < endTimestamp;
            });
            if (matchingBackups.length > 0) return true;
        }
        
        // Check VM snapshots
        const vmSnapshots = (pveBackups.guestSnapshots || []).filter(snap => {
            return parseInt(snap.vmid, 10) === parseInt(guestId, 10) &&
                   snap.snaptime >= startTimestamp &&
                   snap.snaptime < endTimestamp;
        });
        
        if (vmSnapshots.length > 0) return true;
        
        return false;
    }

    function _highlightTableRows(guestIds, highlight) {
        const backupsTableBody = document.getElementById('backups-overview-tbody');
        if (!backupsTableBody) return;
        
        guestIds.forEach(guestId => {
            const row = backupsTableBody.querySelector(`tr[data-guest-id="${guestId}"]`);
            if (row) {
                if (highlight) {
                    // Apply highlighting to non-sticky cells only to avoid layout shift
                    const cells = row.querySelectorAll('td:not(.sticky)');
                    cells.forEach(cell => {
                        cell.classList.add('bg-blue-50/50', 'dark:bg-blue-900/10');
                    });
                    // Add a subtle left border to the second cell (ID column)
                    const idCell = row.querySelector('td:nth-child(2)');
                    if (idCell) {
                        idCell.classList.add('border-l-2', 'border-l-blue-400', 'dark:border-l-blue-500');
                    }
                } else {
                    const cells = row.querySelectorAll('td:not(.sticky)');
                    cells.forEach(cell => {
                        cell.classList.remove('bg-blue-50/50', 'dark:bg-blue-900/10');
                    });
                    const idCell = row.querySelector('td:nth-child(2)');
                    if (idCell) {
                        idCell.classList.remove('border-l-2', 'border-l-blue-400', 'dark:border-l-blue-500');
                    }
                }
            }
        });
    }

    function updateBackupsTab(isUserAction = false) {
        const tableContainer = document.getElementById('backups-table-container');
        const tableBody = document.getElementById('backups-overview-tbody');
        const loadingMsg = document.getElementById('backups-loading-message');
        const noDataMsg = document.getElementById('backups-no-data-message');
        const statusTextElement = document.getElementById('backups-status-text');
        const pbsSummaryElement = document.getElementById('pbs-instances-summary');

        if (!tableContainer || !tableBody || !loadingMsg || !noDataMsg || !statusTextElement) {
            console.error("UI elements for Backups tab not found!");
            return;
        }
        
        // Find the scrollable container
        const scrollableContainer = PulseApp.utils.getScrollableParent(tableBody) || 
                                   tableContainer.closest('.overflow-x-auto') ||
                                   tableContainer;

        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;

        const { allGuests, initialDataReceived, tasksByGuest, snapshotsByGuest, dayBoundaries, threeDaysAgo, sevenDaysAgo } = _getInitialBackupData();

        if (!initialDataReceived) {
            loadingMsg.classList.remove('hidden');
            tableContainer.classList.add('hidden');
            noDataMsg.classList.add('hidden');
            return;
        }

        if (allGuests.length === 0) {
            loadingMsg.classList.add('hidden');
            tableContainer.classList.add('hidden');
            noDataMsg.textContent = "No Proxmox guests (VMs/Containers) found.";
            noDataMsg.classList.remove('hidden');
            _updateBackupStatusMessages(statusTextElement, 0, backupsSearchInput);
            return;
        }
        loadingMsg.classList.add('hidden');

        const backupStatusByGuest = allGuests.map(guest => {
            // Try both PBS (generic) and PVE (node-specific) keys
            const baseKey = `${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`;
            const nodeKey = guest.node ? `-${guest.node}` : (guest.endpointId ? `-${guest.endpointId}` : '');
            const nodeSpecificKey = `${baseKey}${nodeKey}`;
            
            // Get snapshots from both keys and combine them
            const pbsSnapshots = snapshotsByGuest.get(baseKey) || [];
            const pveSnapshots = snapshotsByGuest.get(nodeSpecificKey) || [];
            const allGuestSnapshots = [...pbsSnapshots, ...pveSnapshots];
            
            // Similar for tasks
            const pbsTasks = tasksByGuest.get(baseKey) || [];
            const pveTasks = tasksByGuest.get(nodeSpecificKey) || [];
            const allGuestTasks = [...pbsTasks, ...pveTasks];
            
            return _determineGuestBackupStatus(guest, allGuestSnapshots, allGuestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo);
        });
        const filteredBackupStatus = _filterBackupData(backupStatusByGuest, backupsSearchInput);

        // Prepare backup data for consolidated summary
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        // Get PBS snapshots
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    source: 'pbs'
                }))
            )
        );
        
        // Get PVE storage backups
        const pveStorageBackups = [];
        if (pveBackups?.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            pveBackups.storageBackups.forEach(backup => {
                pveStorageBackups.push({
                    'backup-time': backup.ctime,
                    backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
                    backupVMID: backup.vmid,
                    vmid: backup.vmid, // Ensure vmid is preserved for filtering
                    size: backup.size,
                    protected: backup.protected,
                    storage: backup.storage,
                    volid: backup.volid,
                    format: backup.format,
                    node: backup.node,
                    endpointId: backup.endpointId,
                    source: 'pve'
                });
            });
        }
        
        // Get VM snapshots
        const vmSnapshots = (pveBackups.guestSnapshots || []).map(snap => ({
            ...snap,
            source: 'vmSnapshots'
        }));
        
        const backupData = {
            pbsSnapshots: pbsSnapshots,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots
        };


        // Hide node backup cards - no longer needed with consolidated view
        const nodeBackupCards = document.getElementById('node-backup-cards');
        if (nodeBackupCards) {
            nodeBackupCards.classList.add('hidden');
        }

        // Display backup calendar visualization section
        const visualizationSection = document.getElementById('backup-visualization-section');
        const summaryCardsContainer = document.getElementById('backup-summary-cards-container');
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        
        if (visualizationSection && backupStatusByGuest.length > 0) {
            // Hide the summary cards container - we're using consolidated summary now
            if (summaryCardsContainer) {
                summaryCardsContainer.classList.add('hidden');
            }
            
            // Get backup tasks for calendar
            const pbsBackupTasks = [];
            pbsDataArray.forEach(pbs => {
                if (pbs.backupTasks?.recentTasks && Array.isArray(pbs.backupTasks.recentTasks)) {
                    pbs.backupTasks.recentTasks.forEach(task => {
                        pbsBackupTasks.push({
                            ...task,
                            pbsInstanceName: pbs.pbsInstanceName,
                            source: 'pbs'
                        });
                    });
                }
            });
            
            const pveBackupTasks = [];
            if (Array.isArray(pveBackups?.backupTasks)) {
                pveBackups.backupTasks.forEach(task => {
                    pveBackupTasks.push({
                        ...task,
                        source: 'pve'
                    });
                });
            }
            
            // Extend backupData with tasks for calendar
            const extendedBackupData = {
                ...backupData,
                backupTasks: [...pbsBackupTasks, ...pveBackupTasks]
            };
            
            // Create and display calendar heatmap with detail card
            if (calendarContainer && PulseApp.ui.calendarHeatmap && PulseApp.ui.backupDetailCard) {
                // Get filtered guest IDs for calendar filtering - use unique guest identifiers
                const filteredGuestIds = filteredBackupStatus.map(guest => {
                    // Create unique identifier including node/endpoint to handle guests with same vmid on different nodes
                    const nodeIdentifier = guest.node || guest.endpointId || '';
                    return nodeIdentifier ? `${guest.guestId}-${nodeIdentifier}` : guest.guestId.toString();
                });
                
                // Get detail card container
                const detailCardContainer = document.getElementById('backup-detail-card');
                let detailCard = detailCardContainer.querySelector('.bg-slate-800');
                
                // Find the detail card with the correct class
                detailCard = detailCardContainer.querySelector('.bg-white.dark\\:bg-gray-800');
                
                // Create detail card only if it doesn't exist
                if (detailCardContainer && !detailCard) {
                    detailCard = PulseApp.ui.backupDetailCard.createBackupDetailCard(null);
                    detailCardContainer.innerHTML = '';
                    detailCardContainer.appendChild(detailCard);
                }
                
                // Update detail card with filtered data if no calendar date is selected
                const calendarDateFilter = PulseApp.state.get('calendarDateFilter');
                if (!calendarDateFilter && isUserAction) {
                    if (filteredBackupStatus.length > 0) {
                        const multiDateData = _prepareMultiDateDetailData(filteredBackupStatus, extendedBackupData);
                        PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, multiDateData, false);
                    } else {
                        PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, null, false);
                    }
                }
                
                // Create calendar with date selection callback
                const onDateSelect = (dateData, instant = false) => {
                    if (detailCard && PulseApp.ui.backupDetailCard) {
                        if (dateData) {
                            // Apply current table filters to the selected date's data
                            const filteredDateBackups = dateData.backups.filter(backup => {
                                // Find the guest in filteredBackupStatus using unique identification
                                const guestInFiltered = filteredBackupStatus.find(g => {
                                    // First try exact vmid match for simple cases
                                    if (g.guestId.toString() === backup.vmid.toString()) {
                                        // If backup has node info, ensure it matches
                                        if (backup.node || backup.endpointId) {
                                            return (backup.node && g.node === backup.node) || 
                                                   (backup.endpointId && g.endpointId === backup.endpointId) ||
                                                   (!backup.node && !backup.endpointId);
                                        }
                                        return true;
                                    }
                                    return false;
                                });
                                return guestInFiltered !== undefined;
                            });
                            
                            // Create filtered date data
                            const filteredDateData = {
                                ...dateData,
                                backups: filteredDateBackups,
                                stats: {
                                    totalGuests: filteredDateBackups.length,
                                    pbsCount: filteredDateBackups.filter(b => b.types && b.types.includes('pbsSnapshots')).length,
                                    pveCount: filteredDateBackups.filter(b => b.types && b.types.includes('pveBackups')).length,
                                    snapshotCount: filteredDateBackups.filter(b => b.types && b.types.includes('vmSnapshots')).length,
                                    failureCount: dateData.stats.failureCount || 0
                                }
                            };
                            
                            // Force instant updates on API refreshes to prevent flashing
                            const forceInstant = !isUserAction || instant;
                            PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, filteredDateData, forceInstant);
                        } else {
                            // No date selected, show multi-date filtered data
                            if (filteredBackupStatus.length > 0) {
                                const multiDateData = _prepareMultiDateDetailData(filteredBackupStatus, extendedBackupData);
                                PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, multiDateData, !isUserAction || instant);
                            } else {
                                PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, null, !isUserAction || instant);
                            }
                        }
                    }
                    
                    // Update calendar date filter for table
                    if (dateData && dateData.backups && dateData.backups.length > 0) {
                        // Extract unique guest identifiers from the selected date's backup data
                        const guestIds = dateData.backups.map(backup => {
                            // Use unique key if available, fall back to vmid
                            if (backup.uniqueKey) {
                                return backup.uniqueKey;
                            }
                            // Create unique key from available data
                            const nodeIdentifier = backup.node || backup.endpointId || '';
                            return nodeIdentifier ? `${backup.vmid}-${nodeIdentifier}` : backup.vmid.toString();
                        });
                        PulseApp.state.set('calendarDateFilter', {
                            date: dateData.date,
                            guestIds: guestIds
                        });
                        
                        // Show visual indicator in table header
                        _updateCalendarFilterIndicator(dateData.date, guestIds.length);
                    } else {
                        // Clear filter if no data or deselected
                        PulseApp.state.set('calendarDateFilter', null);
                        _updateCalendarFilterIndicator(null, 0);
                    }
                    
                    // Refresh table to apply calendar filter (only if this is a user action to avoid loops)
                    if (!instant) {
                        updateBackupsTab(false); // false = not a user action for the table update
                    }
                };
                
                // Only recreate calendar if it doesn't exist or if this is a user action
                const existingCalendar = calendarContainer.querySelector('.calendar-heatmap-container');
                if (!existingCalendar || isUserAction) {
                    const calendarHeatmap = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(
                        extendedBackupData, 
                        null, 
                        filteredGuestIds, 
                        onDateSelect,
                        isUserAction
                    );
                    calendarContainer.innerHTML = '';
                    calendarContainer.appendChild(calendarHeatmap);
                } else {
                    // Update existing calendar data without recreation to prevent flashing
                    PulseApp.ui.calendarHeatmap.updateCalendarData(extendedBackupData, null, filteredGuestIds, onDateSelect);
                }
            }
            
            visualizationSection.classList.remove('hidden');
        } else if (visualizationSection) {
            visualizationSection.classList.add('hidden');
        }

        // Calculate PBS instances summary - only show if multiple PBS instances
        const pbsSummaryDismissed = PulseApp.state.get('pbsSummaryDismissed') || false;
        
        if (pbsSummaryElement) {
            if (pbsDataArray.length > 1 && !pbsSummaryDismissed) {
                const pbsSummary = pbsDataArray.map(pbs => {
                    const backupCount = (pbs.datastores || []).reduce((total, ds) => 
                        total + (ds.snapshots ? ds.snapshots.length : 0), 0);
                    return `${pbs.pbsInstanceName}: ${backupCount} backups`;
                }).join(' | ');
                
                pbsSummaryElement.innerHTML = `
                    <div class="flex justify-between items-center">
                        <div>
                            <strong>PBS Instances (${pbsDataArray.length}):</strong> ${pbsSummary}
                            <span class="text-gray-500 dark:text-gray-400 ml-2">• Showing aggregated backup data from all instances</span>
                        </div>
                        <button id="dismiss-pbs-summary" class="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 ml-4" title="Dismiss">
                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
                        </button>
                    </div>
                `;
                pbsSummaryElement.classList.remove('hidden');
                
                // Add dismiss handler
                const dismissBtn = document.getElementById('dismiss-pbs-summary');
                if (dismissBtn) {
                    dismissBtn.addEventListener('click', () => {
                        pbsSummaryElement.classList.add('hidden');
                        PulseApp.state.set('pbsSummaryDismissed', true);
                        PulseApp.state.saveFilterState();
                    });
                }
            } else {
                pbsSummaryElement.classList.add('hidden');
            }
        }

        const sortStateBackups = PulseApp.state.getSortState('backups');
        const sortedBackupStatus = PulseApp.utils.sortData(filteredBackupStatus, sortStateBackups.column, sortStateBackups.direction, 'backups');

        // Calculate dynamic column widths for responsive display
        if (sortedBackupStatus.length > 0) {
            let maxNameLength = 0;
            let maxNodeLength = 0;
            let maxPbsLength = 0;
            let maxDsLength = 0;
            
            sortedBackupStatus.forEach(status => {
                const nameLength = (status.guestName || '').length;
                const nodeLength = (status.node || '').length;
                const pbsLength = (status.pbsInstanceName || 'N/A').length;
                const dsLength = (status.datastoreName || 'N/A').length;
                
                if (nameLength > maxNameLength) maxNameLength = nameLength;
                if (nodeLength > maxNodeLength) maxNodeLength = nodeLength;
                if (pbsLength > maxPbsLength) maxPbsLength = pbsLength;
                if (dsLength > maxDsLength) maxDsLength = dsLength;
            });
            
            // Set CSS variables for column widths with responsive limits
            const nameColWidth = Math.min(Math.max(maxNameLength * 7 + 12, 80), 250);
            const nodeColWidth = Math.max(maxNodeLength * 7 + 12, 60);
            const pbsColWidth = Math.min(Math.max(maxPbsLength * 7 + 12, 80), 150);
            const dsColWidth = Math.min(Math.max(maxDsLength * 7 + 12, 80), 150);
            
            const htmlElement = document.documentElement;
            if (htmlElement) {
                htmlElement.style.setProperty('--backup-name-col-width', `${nameColWidth}px`);
                htmlElement.style.setProperty('--backup-node-col-width', `${nodeColWidth}px`);
                htmlElement.style.setProperty('--backup-pbs-col-width', `${pbsColWidth}px`);
                htmlElement.style.setProperty('--backup-ds-col-width', `${dsColWidth}px`);
            }
        }

        PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
            tableBody.innerHTML = '';
            if (sortedBackupStatus.length > 0) {
                sortedBackupStatus.forEach(guestStatus => {
                    const row = _renderBackupTableRow(guestStatus);
                    tableBody.appendChild(row);
                });
                noDataMsg.classList.add('hidden');
                tableContainer.classList.remove('hidden');
            } else {
            tableContainer.classList.add('hidden');
            let emptyMessage = "No backup information found for any guests.";
             if (backupStatusByGuest.length > 0 && filteredBackupStatus.length === 0) { // Data exists, but filters hide all
                const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value : '';
                const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');
                const typeFilterText = backupsFilterGuestType === 'all' ? '' : `Type: ${backupsFilterGuestType.toUpperCase()}`;
                const filtersApplied = [typeFilterText].filter(Boolean).join(', ');

                if (currentBackupsSearchTerm) {
                    emptyMessage = `No guests found matching search "${currentBackupsSearchTerm}"`;
                    if (filtersApplied) emptyMessage += ` and filters (${filtersApplied})`;
                } else if (filtersApplied) {
                    emptyMessage = `No guests found matching the selected filters (${filtersApplied}).`;
                } else {
                     emptyMessage = "No guests with backup information found matching current filters.";
                }
            }
            noDataMsg.textContent = emptyMessage;
            noDataMsg.classList.remove('hidden');
        }
        }); // End of preserveScrollPosition
        
        // Setup click filtering between table and calendar (only on user actions or initial load)
        if (isUserAction || !document.querySelector('#backups-overview-tbody tr[data-guest-id]')) {
            _initTableCalendarClick();
        }
        
        // Additional scroll position restoration for horizontal scrolling
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }

        const backupsSortColumn = sortStateBackups.column;
        const backupsHeader = document.querySelector(`#backups-overview-table th[data-sort="${backupsSortColumn}"]`);
        if (PulseApp.ui && PulseApp.ui.common) {
             PulseApp.ui.common.updateSortUI('backups-overview-table', backupsHeader);
        } else {
            console.warn('[Backups] PulseApp.ui.common not available for updateSortUI');
        }
        _updateBackupStatusMessages(statusTextElement, sortedBackupStatus.length, backupsSearchInput);
    }

    function resetBackupsView() {
        if (backupsSearchInput) backupsSearchInput.value = '';
        PulseApp.state.set('backupsSearchTerm', '');

        const backupTypeAllRadio = document.getElementById('backups-filter-type-all');
        if(backupTypeAllRadio) backupTypeAllRadio.checked = true;
        PulseApp.state.set('backupsFilterGuestType', 'all');

        const backupStatusAllRadio = document.getElementById('backups-filter-status-all');
        if(backupStatusAllRadio) backupStatusAllRadio.checked = true;
        PulseApp.state.set('backupsFilterHealth', 'all');

        const backupBackupTypeAllRadio = document.getElementById('backups-filter-backup-all');
        if(backupBackupTypeAllRadio) backupBackupTypeAllRadio.checked = true;
        PulseApp.state.set('backupsFilterBackupType', 'all');

        const failuresFilter = document.getElementById('backups-filter-failures');
        if(failuresFilter) failuresFilter.checked = false;
        PulseApp.state.set('backupsFilterFailures', false);

        PulseApp.state.setSortState('backups', 'latestBackupTime', 'desc');

        // Clear calendar filter selection
        PulseApp.state.set('currentFilteredGuest', null);
        
        // Clear calendar date filter
        PulseApp.state.set('calendarDateFilter', null);
        _updateCalendarFilterIndicator(null, 0);
        
        // Clear calendar selection if possible
        if (PulseApp.ui.calendarHeatmap && PulseApp.ui.calendarHeatmap.clearSelection) {
            PulseApp.ui.calendarHeatmap.clearSelection();
        }

        updateBackupsTab(true); // Mark as user action
        PulseApp.state.saveFilterState(); // Save reset state
    }

    function _initSnapshotModal() {
        const modal = document.getElementById('snapshot-modal');
        const modalClose = document.getElementById('snapshot-modal-close');
        const modalBody = document.getElementById('snapshot-modal-body');
        const modalTitle = document.getElementById('snapshot-modal-title');
        
        if (!modal || !modalClose || !modalBody) {
            console.warn('[Backups] Snapshot modal elements not found');
            return;
        }
        
        // Close modal on click outside or close button
        modalClose.addEventListener('click', () => {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        });
        
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.classList.add('hidden');
                modal.classList.remove('flex');
            }
        });
        
        // Handle snapshot button clicks
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('view-snapshots-btn')) {
                const vmid = e.target.dataset.vmid;
                const node = e.target.dataset.node;
                const endpoint = e.target.dataset.endpoint;
                const type = e.target.dataset.type;
                
                _showSnapshotModal(vmid, node, endpoint, type);
            }
        });
    }
    
    function _showSnapshotModal(vmid, node, endpoint, type) {
        const modal = document.getElementById('snapshot-modal');
        const modalBody = document.getElementById('snapshot-modal-body');
        const modalTitle = document.getElementById('snapshot-modal-title');
        
        if (!modal || !modalBody || !modalTitle) return;
        
        // Get guest info
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const guest = [...vmsData, ...containersData].find(g => g.vmid === vmid);
        const guestName = guest?.name || `Guest ${vmid}`;
        
        modalTitle.textContent = `Snapshots for ${guestName} (${type.toUpperCase()} ${vmid})`;
        modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Loading snapshots...</p>';
        
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        
        // Get snapshots from state
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const snapshots = (pveBackups.guestSnapshots || [])
            .filter(snap => parseInt(snap.vmid, 10) === parseInt(vmid, 10))
            .sort((a, b) => (b.snaptime || 0) - (a.snaptime || 0));
        
        if (snapshots.length === 0) {
            modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">No snapshots found for this guest.</p>';
            return;
        }
        
        // Build snapshot table
        let html = `
            <div class="overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead class="bg-gray-50 dark:bg-gray-700">
                        <tr>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Description</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">RAM</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
        `;
        
        snapshots.forEach(snap => {
            const created = snap.snaptime 
                ? new Date(snap.snaptime * 1000).toLocaleString()
                : 'Unknown';
            const hasRam = snap.vmstate ? 'Yes' : 'No';
            const description = snap.description || '-';
            
            html += `
                <tr>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">${snap.name}</td>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">${created}</td>
                    <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">${description}</td>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">${hasRam}</td>
                </tr>
            `;
        });
        
        html += `
                    </tbody>
                </table>
            </div>
        `;
        
        modalBody.innerHTML = html;
    }
    
    function _updateCalendarFilterIndicator(date, guestCount) {
        // Add visual indicator to show calendar date filter is active
        const statusText = document.getElementById('backups-status-text');
        if (!statusText) return;
        
        if (date && guestCount > 0) {
            const formattedDate = new Date(date).toLocaleDateString('en-US', { 
                month: 'short', 
                day: 'numeric',
                year: 'numeric'
            });
            
            // Add calendar filter indicator to status text
            const existingText = statusText.textContent.replace(/ \| Calendar: .+/, '');
            statusText.textContent = `${existingText} | Calendar: ${formattedDate} (${guestCount} guest${guestCount !== 1 ? 's' : ''})`;
            statusText.classList.add('text-blue-600', 'dark:text-blue-400');
        } else {
            // Remove calendar filter indicator
            const existingText = statusText.textContent.replace(/ \| Calendar: .+/, '');
            statusText.textContent = existingText;
            statusText.classList.remove('text-blue-600', 'dark:text-blue-400');
        }
    }
    
    function _prepareMultiDateDetailData(filteredBackupStatus, backupData) {
        // Prepare data for multi-date detail view
        const multiDateBackups = [];
        const stats = {
            totalGuests: filteredBackupStatus.length,
            totalBackups: 0,
            pbsCount: 0,
            pveCount: 0,
            snapshotCount: 0,
            failureCount: 0
        };
        
        // Get current backup type filter to determine which dates to include
        const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType') || 'all';
        
        // Get all backup data for filtered guests
        filteredBackupStatus.forEach(guestStatus => {
            const guestId = guestStatus.guestId.toString();
            const backupDates = [];
            
            // Check PBS snapshots (only if filter allows PBS backups)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'pbs') && backupData.pbsSnapshots) {
                const pbsDates = {};
                backupData.pbsSnapshots.forEach(snap => {
                    const snapId = snap['backup-id'] || snap.backupVMID;
                    if (parseInt(snapId, 10) === parseInt(guestId, 10)) {
                        const timestamp = snap['backup-time'];
                        if (timestamp) {
                            const date = new Date(timestamp * 1000);
                            const dateKey = date.toISOString().split('T')[0];
                            if (!pbsDates[dateKey]) {
                                pbsDates[dateKey] = { types: new Set(), count: 0 };
                            }
                            pbsDates[dateKey].types.add('pbsSnapshots');
                            pbsDates[dateKey].count++;
                            stats.totalBackups++;
                        }
                    }
                });
                Object.entries(pbsDates).forEach(([date, info]) => {
                    backupDates.push({ date, types: Array.from(info.types), count: info.count });
                });
            }
            
            // Check PVE backups (only if filter allows PVE backups)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'pve') && backupData.pveBackups) {
                const pveDates = {};
                backupData.pveBackups.forEach(backup => {
                    if (parseInt(backup.vmid, 10) === parseInt(guestId, 10)) {
                        const timestamp = backup['backup-time'] || backup.ctime;
                        if (timestamp) {
                            const date = new Date(timestamp * 1000);
                            const dateKey = date.toISOString().split('T')[0];
                            if (!pveDates[dateKey]) {
                                pveDates[dateKey] = { types: new Set(), count: 0 };
                            }
                            pveDates[dateKey].types.add('pveBackups');
                            pveDates[dateKey].count++;
                            stats.totalBackups++;
                        }
                    }
                });
                Object.entries(pveDates).forEach(([date, info]) => {
                    const existing = backupDates.find(d => d.date === date);
                    if (existing) {
                        existing.types = [...new Set([...existing.types, ...Array.from(info.types)])];
                        existing.count += info.count;
                    } else {
                        backupDates.push({ date, types: Array.from(info.types), count: info.count });
                    }
                });
            }
            
            // Check VM snapshots (only if filter allows snapshots)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'snapshots') && backupData.vmSnapshots) {
                const snapDates = {};
                backupData.vmSnapshots.forEach(snap => {
                    if (parseInt(snap.vmid, 10) === parseInt(guestId, 10)) {
                        const timestamp = snap.snaptime;
                        if (timestamp) {
                            const date = new Date(timestamp * 1000);
                            const dateKey = date.toISOString().split('T')[0];
                            if (!snapDates[dateKey]) {
                                snapDates[dateKey] = { types: new Set(), count: 0 };
                            }
                            snapDates[dateKey].types.add('vmSnapshots');
                            snapDates[dateKey].count++;
                            stats.totalBackups++;
                        }
                    }
                });
                Object.entries(snapDates).forEach(([date, info]) => {
                    const existing = backupDates.find(d => d.date === date);
                    if (existing) {
                        existing.types = [...new Set([...existing.types, ...Array.from(info.types)])];
                        existing.count += info.count;
                    } else {
                        backupDates.push({ date, types: Array.from(info.types), count: info.count });
                    }
                });
            }
            
            // Update stats
            if (guestStatus.pbsBackups > 0) stats.pbsCount++;
            if (guestStatus.pveBackups > 0) stats.pveCount++;
            if (guestStatus.snapshotCount > 0) stats.snapshotCount++;
            if (guestStatus.recentFailures > 0) stats.failureCount++;
            
            if (backupDates.length > 0) {
                multiDateBackups.push({
                    ...guestStatus,
                    backupDates: backupDates.sort((a, b) => b.date.localeCompare(a.date))
                });
            }
        });
        
        // Get current filter info
        const filterInfo = {
            search: backupsSearchInput ? backupsSearchInput.value : '',
            guestType: PulseApp.state.get('backupsFilterGuestType'),
            backupType: backupTypeFilter,
            healthStatus: PulseApp.state.get('backupsFilterHealth'),
            failuresOnly: PulseApp.state.get('backupsFilterFailures') || false
        };
        
        // Recalculate stats based on unique guests with backups
        // Use composite key (guestId-node) to handle guests with same VMID on different nodes
        const uniqueGuestsWithPBS = new Set();
        const uniqueGuestsWithPVE = new Set();
        const uniqueGuestsWithSnapshots = new Set();
        
        multiDateBackups.forEach(guest => {
            // Create unique identifier that includes both guest ID and node/endpoint
            const uniqueKey = `${guest.guestId}-${guest.node || guest.endpointId || 'unknown'}`;
            
            // Count guests based on whether they have backup dates of the filtered type
            // This ensures we count all guests that appear in the filtered view, not just recent activity
            if (backupTypeFilter === 'all') {
                // When showing all types, use the guest's overall backup counts
                if (guest.pbsBackups > 0) uniqueGuestsWithPBS.add(uniqueKey);
                if (guest.pveBackups > 0) uniqueGuestsWithPVE.add(uniqueKey);
                if (guest.snapshotCount > 0) uniqueGuestsWithSnapshots.add(uniqueKey);
            } else {
                // When filtering by specific type, count guests who have that type in their backup dates
                const hasFilteredBackupType = guest.backupDates.some(dateInfo => {
                    if (backupTypeFilter === 'pbs') return dateInfo.types.includes('pbsSnapshots');
                    if (backupTypeFilter === 'pve') return dateInfo.types.includes('pveBackups');
                    if (backupTypeFilter === 'snapshots') return dateInfo.types.includes('vmSnapshots');
                    return false;
                });
                
                if (hasFilteredBackupType) {
                    if (backupTypeFilter === 'pbs') uniqueGuestsWithPBS.add(uniqueKey);
                    if (backupTypeFilter === 'pve') uniqueGuestsWithPVE.add(uniqueKey);
                    if (backupTypeFilter === 'snapshots') uniqueGuestsWithSnapshots.add(uniqueKey);
                }
            }
        });
        
        stats.pbsCount = uniqueGuestsWithPBS.size;
        stats.pveCount = uniqueGuestsWithPVE.size;
        stats.snapshotCount = uniqueGuestsWithSnapshots.size;
        
        // Calculate unique dates for filtered view
        const uniqueDates = new Set();
        multiDateBackups.forEach(guest => {
            guest.backupDates.forEach(dateInfo => {
                uniqueDates.add(dateInfo.date);
            });
        });
        stats.uniqueDates = uniqueDates.size;
        
        return {
            isMultiDate: true,
            backups: multiDateBackups,
            stats: stats,
            filterInfo: filterInfo
        };
    }

    return {
        init,
        updateBackupsTab,
        resetBackupsView,
        _highlightTableRows
    };
})();
