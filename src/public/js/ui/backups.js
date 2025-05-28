PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let backupsSearchInput = null;
    let resetBackupsButton = null;
    let backupsTabContent = null;

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
            const debouncedUpdate = PulseApp.utils.debounce(updateBackupsTab, 300);
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

    function createConsolidatedBackupSummary(summary, backupData, backupStatusByGuest) {
        // Calculate additional stats for consolidated view
        const stats = PulseApp.ui.backupSummaryCards ? 
            PulseApp.ui.backupSummaryCards.calculateBackupStatistics(backupData) : 
            { lastBackup: { time: null }, coverage: { daily: 0, weekly: 0, monthly: 0 }, protected: { count: 0 } };
        
        // For backup health, reverse the color logic - higher percentage should be green
        const healthColorClass = summary.healthyPercent >= 80 ? 'text-green-600' : 
                                 summary.healthyPercent >= 60 ? 'text-yellow-600' : 'text-red-600';
        const progressColor = summary.healthyPercent >= 80 ? 'green' : 
                             summary.healthyPercent >= 60 ? 'yellow' : 'red';
        
        // Format last backup time
        const lastBackupText = stats.lastBackup.time ? 
            formatTimeAgo(Date.now() - stats.lastBackup.time) : 
            'No backups';
        const lastBackupClass = stats.lastBackup.time ? 'text-gray-900 dark:text-gray-100' : 'text-red-600 dark:text-red-400';
        
        // Determine coverage status
        let coverageStatus = 'Good';
        let coverageClass = 'text-green-600 dark:text-green-400';
        if (stats.coverage.daily === 0) {
            coverageStatus = 'Poor';
            coverageClass = 'text-red-600 dark:text-red-400';
        } else if (stats.coverage.weekly < 3) {
            coverageStatus = 'Fair';
            coverageClass = 'text-yellow-600 dark:text-yellow-400';
        }
        
        return `
            <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 mb-4">
                <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
                    <!-- Overview Section -->
                    <div class="lg:col-span-1">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Backup Overview</h3>
                        <div class="space-y-2 text-sm">
                            <div class="flex justify-between items-center">
                                <span class="text-gray-500 dark:text-gray-400">Total Guests:</span>
                                <span class="font-medium text-gray-900 dark:text-gray-100">${summary.totalGuests}</span>
                            </div>
                            <div class="flex justify-between items-center">
                                <span class="text-gray-500 dark:text-gray-400">Last Backup:</span>
                                <span class="font-medium ${lastBackupClass}">${lastBackupText}</span>
                            </div>
                            <div class="flex justify-between items-center">
                                <span class="text-gray-500 dark:text-gray-400">Coverage:</span>
                                <span class="font-medium ${coverageClass}">${coverageStatus}</span>
                            </div>
                            <div class="flex justify-between items-center">
                                <span class="text-gray-500 dark:text-gray-400">Protected:</span>
                                <span class="font-medium text-gray-900 dark:text-gray-100">${stats.protected.count}</span>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Backup Types Section -->
                    <div class="lg:col-span-1">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Backup Types</h3>
                        <div class="space-y-2">
                            <div class="flex items-center justify-between">
                                <div class="flex items-center gap-2">
                                    <span class="inline-block w-2 h-2 bg-blue-500 rounded-full"></span>
                                    <span class="text-sm text-gray-600 dark:text-gray-400">Snapshots</span>
                                </div>
                                <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${summary.totalSnapshots}</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <div class="flex items-center gap-2">
                                    <span class="inline-block w-2 h-2 bg-yellow-500 rounded-full"></span>
                                    <span class="text-sm text-gray-600 dark:text-gray-400">PVE Backups</span>
                                </div>
                                <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${summary.totalPveBackups}</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <div class="flex items-center gap-2">
                                    <span class="inline-block w-2 h-2 bg-green-500 rounded-full"></span>
                                    <span class="text-sm text-gray-600 dark:text-gray-400">PBS Backups</span>
                                </div>
                                <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${summary.totalPbsBackups}</span>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Health Status Section -->
                    <div class="lg:col-span-1">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Health Status</h3>
                        <div class="space-y-2">
                            <div class="flex items-center justify-between">
                                <span class="text-sm text-gray-600 dark:text-gray-400">Overall Health</span>
                                <span class="text-sm font-medium ${healthColorClass}">${summary.healthyPercent.toFixed(0)}%</span>
                            </div>
                            ${PulseApp.utils.createProgressTextBarHTML(summary.healthyPercent, '', progressColor, '')}
                            <div class="grid grid-cols-2 gap-2 text-xs mt-2">
                                ${summary.healthyCount > 0 ? `<span class="text-green-600 dark:text-green-400">● ${summary.healthyCount} healthy</span>` : ''}
                                ${summary.warningCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">● ${summary.warningCount} warning</span>` : ''}
                                ${summary.errorCount > 0 ? `<span class="text-red-600 dark:text-red-400">● ${summary.errorCount} failed</span>` : ''}
                                ${summary.noneCount > 0 ? `<span class="text-gray-600 dark:text-gray-400">● ${summary.noneCount} none</span>` : ''}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
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
                    <div class="w-2 h-2 bg-blue-500 rounded-sm"></div>
                    <span>${snapshotTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-yellow-500 rounded-sm"></div>
                    <span>${pveTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-green-500 rounded-sm"></div>
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

    function _getInitialBackupData() {
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const initialDataReceived = PulseApp.state.get('initialDataReceived');
        const allGuests = [...vmsData, ...containersData];

        // Combine PBS and PVE backup tasks
        const pbsBackupTasks = pbsDataArray.flatMap(pbs =>
            (pbs.backupTasks?.recentTasks || []).map(task => ({
                ...task,
                guestId: task.id?.split('/')[1] || null,
                guestTypePbs: task.id?.split('/')[0] || null,
                pbsInstanceName: pbs.pbsInstanceName,
                source: 'pbs'
            }))
        );

        const pveBackupTasks = (pveBackups.backupTasks || []).map(task => ({
            ...task,
            guestId: task.guestId,
            guestTypePbs: task.guestType,
            startTime: task.starttime,
            source: 'pve'
        }));

        const allRecentBackupTasks = [...pbsBackupTasks, ...pveBackupTasks];

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
            backupType: backup.vmid ? 'vm' : 'ct', // Guess based on context
            backupVMID: backup.vmid,
            size: backup.size,
            protected: backup.protected,
            storage: backup.storage,
            source: 'pve'
        }));

        const allSnapshots = [...pbsSnapshots, ...pveStorageBackups];

        // Pre-index data by guest ID and type for performance
        const tasksByGuest = new Map();
        const snapshotsByGuest = new Map();

        allRecentBackupTasks.forEach(task => {
            const key = `${task.guestId}-${task.guestTypePbs}`;
            if (!tasksByGuest.has(key)) tasksByGuest.set(key, []);
            tasksByGuest.get(key).push(task);
        });

        allSnapshots.forEach(snap => {
            const key = `${snap.backupVMID}-${snap.backupType}`;
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

        return { 
            allGuests, 
            initialDataReceived, 
            tasksByGuest, 
            snapshotsByGuest, 
            dayBoundaries,
            threeDaysAgo,
            sevenDaysAgo
        };
    }

    function _determineGuestBackupStatus(guest, guestSnapshots, guestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo) {
        const guestId = String(guest.vmid);
        
        // Get guest snapshots from pveBackups
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const allSnapshots = pveBackups.guestSnapshots || [];
        const guestSnapshotCount = allSnapshots
            .filter(snap => parseInt(snap.vmid, 10) === parseInt(guest.vmid, 10))
            .length;
        
        // Debug disabled
        
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
            const guestDaySnapshots = allSnapshots.filter(snap => 
                parseInt(snap.vmid, 10) === parseInt(guestId, 10) &&
                snap.snaptime >= day.start && snap.snaptime < day.end
            );
            
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
            endpointId: guest.endpointId
        };
    }

    function _filterBackupData(backupStatusByGuest, backupsSearchInput) {
        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value.toLowerCase() : '';
        const backupsSearchTerms = currentBackupsSearchTerm.split(',').map(term => term.trim()).filter(term => term);
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');

        return backupStatusByGuest.filter(item => {
            const healthMatch = (backupsFilterHealth === 'all') ||
                                (backupsFilterHealth === 'ok' && (item.backupHealthStatus === 'ok' || item.backupHealthStatus === 'stale')) ||
                                (backupsFilterHealth === 'warning' && (item.backupHealthStatus === 'old')) ||
                                (backupsFilterHealth === 'error' && item.backupHealthStatus === 'failed') ||
                                (backupsFilterHealth === 'none' && item.backupHealthStatus === 'none');
            if (!healthMatch) return false;

            const typeMatch = (backupsFilterGuestType === 'all') ||
                              (backupsFilterGuestType === 'vm' && item.guestType === 'VM') ||
                              (backupsFilterGuestType === 'lxc' && item.guestType === 'LXC');
            if (!typeMatch) return false;

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

    function _renderBackupTableRow(guestStatus) {
        const row = document.createElement('tr');
        row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        row.dataset.guestId = guestStatus.guestId;

        const latestBackupFormatted = guestStatus.latestBackupTime
            ? PulseApp.utils.formatPbsTimestamp(guestStatus.latestBackupTime)
            : '<span class="text-gray-400">No backups found</span>';

        const typeIconClass = guestStatus.guestType === 'VM'
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;

        // Generate 7-day backup indicators as horizontal bars
        let sevenDayDots = '<div class="flex space-x-0.5">';
        if (guestStatus.last7DaysBackupStatus && guestStatus.last7DaysBackupStatus.length === 7) {
            guestStatus.last7DaysBackupStatus.forEach(dayInfo => {
                let barContent = '';
                let barTitle = `${dayInfo.date}: ${dayInfo.details}`;
                
                if (dayInfo.backupTypes && dayInfo.backupTypes.length > 0) {
                    // Sort types by priority: snapshot -> pve -> pbs
                    const sortedTypes = dayInfo.backupTypes.sort((a, b) => {
                        const order = { 'snapshot': 0, 'pve': 1, 'pbs': 2 };
                        return order[a] - order[b];
                    });
                    
                    const typeClasses = {
                        'snapshot': 'bg-blue-500',     // Blue for snapshots (local)
                        'pve': 'bg-yellow-500',        // Yellow for PVE (cluster) 
                        'pbs': 'bg-green-500'          // Green for PBS (remote)
                    };
                    
                    // Use dots for multiple backup types to avoid overlap
                    if (sortedTypes.length === 1) {
                        // Single backup type - solid square
                        const type = sortedTypes[0];
                        const colorClass = typeClasses[type];
                        const failureBorder = dayInfo.hasFailures ? 'border border-red-500' : '';
                        barContent = `<div class="w-2 h-2 ${colorClass} ${failureBorder} rounded-sm" title="${barTitle}"></div>`;
                    } else {
                        // Multiple backup types - stacked dots
                        const failureBorder = dayInfo.hasFailures ? 'border border-red-500' : '';
                        
                        barContent = `<div class="flex flex-col w-2 h-2 ${failureBorder} rounded-sm" title="${barTitle}">`;
                        
                        if (sortedTypes.length === 2) {
                            // Two types - split top/bottom
                            sortedTypes.forEach((type, index) => {
                                const colorClass = typeClasses[type];
                                barContent += `<div class="flex-1 ${colorClass} ${index === 0 ? 'rounded-t-sm' : 'rounded-b-sm'}"></div>`;
                            });
                        } else {
                            // Three types - show as striped pattern
                            const primaryType = sortedTypes[0]; // Show most important type
                            const colorClass = typeClasses[primaryType];
                            barContent = `<div class="w-2 h-2 ${colorClass} ${failureBorder} rounded-sm relative" title="${barTitle}">`;
                            barContent += `<div class="absolute inset-0 bg-gradient-to-r from-transparent via-white via-50% to-transparent opacity-30 rounded-sm"></div>`;
                            barContent += '</div>';
                        }
                        
                        if (sortedTypes.length === 2) {
                            barContent += '</div>';
                        }
                    }
                } else {
                    // No backup activity
                    barContent = `<div class="w-2 h-2 bg-gray-300 dark:bg-gray-600 rounded-sm" title="${barTitle}"></div>`;
                }
                
                sevenDayDots += barContent;
            });
        } else {
            for (let i = 0; i < 7; i++) {
                 sevenDayDots += '<div class="w-2 h-2 bg-gray-200 dark:bg-gray-700 rounded-sm" title="Data unavailable"></div>';
            }
        }
        sevenDayDots += '</div>';

        // Create PBS backup cell with visual indicator
        let pbsBackupCell = '';
        if (guestStatus.pbsBackups > 0) {
            const pbsIcon = '<span class="inline-block w-2 h-2 bg-green-500 rounded-full mr-1" title="PBS Backup"></span>';
            pbsBackupCell = `<span class="text-green-700 dark:text-green-300" ${guestStatus.pbsBackupInfo ? `title="${guestStatus.pbsBackupInfo}"` : ''}>${pbsIcon}${guestStatus.pbsBackups}</span>`;
        } else {
            pbsBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create PVE backup cell with visual indicator  
        let pveBackupCell = '';
        if (guestStatus.pveBackups > 0) {
            const pveIcon = '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full mr-1" title="PVE Backup"></span>';
            pveBackupCell = `<span class="text-yellow-700 dark:text-yellow-300" ${guestStatus.pveBackupInfo ? `title="${guestStatus.pveBackupInfo}"` : ''}>${pveIcon}${guestStatus.pveBackups}</span>`;
        } else {
            pveBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create snapshot button or count display
        let snapshotCell = '';
        if (guestStatus.snapshotCount > 0) {
            const snapshotIcon = '<span class="inline-block w-2 h-2 bg-blue-500 rounded-full mr-1" title="VM/CT Snapshot"></span>';
            snapshotCell = `<button class="text-blue-600 dark:text-blue-400 hover:underline view-snapshots-btn" 
                data-vmid="${guestStatus.guestId}" 
                data-node="${guestStatus.node}"
                data-endpoint="${guestStatus.endpointId}"
                data-type="${guestStatus.guestType.toLowerCase()}">${snapshotIcon}${guestStatus.snapshotCount}</button>`;
        } else {
            snapshotCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        row.innerHTML = `
            <td class="sticky left-0 bg-white dark:bg-gray-800 z-10 p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 text-gray-900 dark:text-gray-100" title="${guestStatus.guestName}">${guestStatus.guestName}</td>
            <td class="p-1 px-2 text-gray-500 dark:text-gray-400">${guestStatus.guestId}</td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.node}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${latestBackupFormatted}</td>
            <td class="p-1 px-2 text-center">${pbsBackupCell}</td>
            <td class="p-1 px-2 text-center">${pveBackupCell}</td>
            <td class="p-1 px-2 text-center">${snapshotCell}</td>
            <td class="p-1 px-2 whitespace-nowrap">${sevenDayDots}</td>
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
                // Re-apply calendar filter on restore
                _filterCalendarToGuest(guestId);
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

    function _filterCalendarToGuest(guestId) {
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
        if (pveBackups?.storageBackups) {
            Object.entries(pveBackups.storageBackups).forEach(([nodeName, nodeData]) => {
                if (nodeData && typeof nodeData === 'object') {
                    Object.entries(nodeData).forEach(([storage, backups]) => {
                        if (Array.isArray(backups)) {
                            backups.forEach(backup => {
                                pveStorageBackups.push({
                                    ...backup,
                                    node: nodeName,
                                    storage: storage,
                                    source: 'pve'
                                });
                            });
                        }
                    });
                }
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
        
        // Create filtered calendar for this specific guest
        const filteredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, guestId, [guestId]);
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
        const backupStatusByGuest = allGuests.map(guest => _determineGuestBackupStatus(guest, snapshotsByGuest.get(`${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`) || [], tasksByGuest.get(`${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`) || [], dayBoundaries, threeDaysAgo, sevenDaysAgo));
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
        if (pveBackups?.storageBackups) {
            Object.entries(pveBackups.storageBackups).forEach(([nodeName, nodeData]) => {
                if (nodeData && typeof nodeData === 'object') {
                    Object.entries(nodeData).forEach(([storage, backups]) => {
                        if (Array.isArray(backups)) {
                            backups.forEach(backup => {
                                pveStorageBackups.push({
                                    ...backup,
                                    node: nodeName,
                                    storage: storage,
                                    source: 'pve'
                                });
                            });
                        }
                    });
                }
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
        
        // Create calendar respecting current table filters
        const filteredGuestIds = filteredBackupStatus.map(guest => guest.guestId.toString());
        const restoredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, null, filteredGuestIds);
        calendarContainer.innerHTML = '';
        calendarContainer.appendChild(restoredCalendar);
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
                    return vmid == guestId && timestamp >= startTimestamp && timestamp < endTimestamp;
                })
            )
        );
        
        if (pbsSnapshots.length > 0) return true;
        
        // Check PVE storage backups
        if (pveBackups.storageBackups) {
            for (const [nodeName, nodeData] of Object.entries(pveBackups.storageBackups)) {
                if (nodeData && typeof nodeData === 'object') {
                    for (const [storage, backups] of Object.entries(nodeData)) {
                        if (Array.isArray(backups)) {
                            const matchingBackups = backups.filter(backup => {
                                return backup.vmid == guestId && 
                                       backup.ctime >= startTimestamp && 
                                       backup.ctime < endTimestamp;
                            });
                            if (matchingBackups.length > 0) return true;
                        }
                    }
                }
            }
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

    function updateBackupsTab() {
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

        const backupStatusByGuest = allGuests.map(guest => _determineGuestBackupStatus(guest, snapshotsByGuest.get(`${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`) || [], tasksByGuest.get(`${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`) || [], dayBoundaries, threeDaysAgo, sevenDaysAgo));
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
        if (pveBackups?.storageBackups) {
            Object.entries(pveBackups.storageBackups).forEach(([nodeName, nodeData]) => {
                if (nodeData && typeof nodeData === 'object') {
                    Object.entries(nodeData).forEach(([storage, backups]) => {
                        if (Array.isArray(backups)) {
                            backups.forEach(backup => {
                                pveStorageBackups.push({
                                    ...backup,
                                    node: nodeName,
                                    storage: storage,
                                    source: 'pve'
                                });
                            });
                        }
                    });
                }
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

        // Calculate and display consolidated backup summary
        const backupSummary = calculateBackupSummary(backupStatusByGuest);
        const backupSummaryContainer = document.getElementById('backup-summary-container');
        if (backupSummaryContainer && backupStatusByGuest.length > 0) {
            backupSummaryContainer.innerHTML = createConsolidatedBackupSummary(backupSummary, backupData, backupStatusByGuest);
            backupSummaryContainer.classList.remove('hidden');
        } else if (backupSummaryContainer) {
            backupSummaryContainer.classList.add('hidden');
        }

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
            
            // Create and display calendar heatmap
            if (calendarContainer && PulseApp.ui.calendarHeatmap) {
                // Get filtered guest IDs for calendar filtering
                const filteredGuestIds = filteredBackupStatus.map(guest => guest.guestId.toString());
                const calendarHeatmap = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(extendedBackupData, null, filteredGuestIds);
                calendarContainer.innerHTML = '';
                calendarContainer.appendChild(calendarHeatmap);
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
        
        // Setup click filtering between table and calendar
        _initTableCalendarClick();
        
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
        console.log('Resetting backups view...');
        if (backupsSearchInput) backupsSearchInput.value = '';
        PulseApp.state.set('backupsSearchTerm', '');

        const backupTypeAllRadio = document.getElementById('backups-filter-type-all');
        if(backupTypeAllRadio) backupTypeAllRadio.checked = true;
        PulseApp.state.set('backupsFilterGuestType', 'all');

        const backupStatusAllRadio = document.getElementById('backups-filter-status-all');
        if(backupStatusAllRadio) backupStatusAllRadio.checked = true;
        PulseApp.state.set('backupsFilterHealth', 'all');

        PulseApp.state.setSortState('backups', 'latestBackupTime', 'desc');

        // Clear calendar filter selection
        PulseApp.state.set('currentFilteredGuest', null);

        updateBackupsTab();
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

    return {
        init,
        updateBackupsTab,
        resetBackupsView,
        _highlightTableRows
    };
})();
