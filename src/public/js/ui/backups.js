PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let backupsSearchInput = null;
    let resetBackupsButton = null;
    let backupsTabContent = null;

    function init() {
        backupsSearchInput = document.getElementById('backups-search');
        resetBackupsButton = document.getElementById('reset-backups-filters-button');
        backupsTabContent = document.getElementById('backups');

        if (backupsSearchInput) {
            const debouncedUpdate = debounce(updateBackupsTab, 300);
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
    }

    function _getInitialBackupData() {
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const initialDataReceived = PulseApp.state.get('initialDataReceived');
        const allGuests = [...vmsData, ...containersData];

        const allRecentBackupTasks = pbsDataArray.flatMap(pbs =>
            (pbs.backupTasks?.recentTasks || []).map(task => ({
                ...task,
                guestId: task.id?.split('/')[1] || null,
                guestTypePbs: task.id?.split('/')[0] || null,
                pbsInstanceName: pbs.pbsInstanceName
            }))
        );

        const allSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    backupType: snap['backup-type'],
                    backupVMID: snap['backup-id']
                }))
            )
        );

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

        // Optimized 7-day backup status calculation using pre-calculated boundaries
        const last7DaysBackupStatus = dayBoundaries.map(day => {
            let dailyStatus = 'none';

            // Check tasks for this day - using pre-filtered guest tasks
            if (guestTasks) {
                const failedTaskOnThisDay = guestTasks.find(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status !== 'OK'
                );
                const successfulTaskOnThisDay = guestTasks.find(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status === 'OK'
                );

                if (failedTaskOnThisDay) {
                    dailyStatus = 'failed';
                } else if (successfulTaskOnThisDay) {
                    dailyStatus = 'ok';
                } else if (guestSnapshots) {
                    // Check snapshots as fallback - using pre-filtered guest snapshots
                    const snapshotOnThisDay = guestSnapshots.some(
                        snap => snap['backup-time'] >= day.start && snap['backup-time'] < day.end
                    );
                    if (snapshotOnThisDay) {
                        dailyStatus = 'ok';
                    }
                }
            }

            return dailyStatus;
        });

        return {
            guestName: guest.name || `Guest ${guest.vmid}`,
            guestId: guest.vmid,
            guestType: guest.type === 'qemu' ? 'VM' : 'LXC',
            node: guest.node,
            guestPveStatus: guest.status,
            latestBackupTime: displayTimestamp,
            pbsInstanceName: latestSnapshot?.pbsInstanceName || latestTask?.pbsInstanceName || 'N/A',
            datastoreName: latestSnapshot?.datastoreName || 'N/A',
            totalBackups: totalBackups,
            backupHealthStatus: healthStatus,
            last7DaysBackupStatus: last7DaysBackupStatus
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
        row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px`;

        const latestBackupFormatted = guestStatus.latestBackupTime
            ? PulseApp.utils.formatPbsTimestamp(guestStatus.latestBackupTime)
            : '<span class="text-gray-400">No backups found</span>';

        const typeIconClass = guestStatus.guestType === 'VM'
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;

        // Generate 7-day backup dots
        let sevenDayDots = '<div class="flex space-x-0.5" title="Last 7 days backup history (oldest to newest)">';
        if (guestStatus.last7DaysBackupStatus && guestStatus.last7DaysBackupStatus.length === 7) {
            guestStatus.last7DaysBackupStatus.forEach(status => {
                let dotClass = 'bg-gray-300 dark:bg-gray-600'; // Default for 'none'
                let dotTitle = 'No backup';
                if (status === 'ok') {
                    dotClass = 'bg-green-500';
                    dotTitle = 'Successful backup';
                } else if (status === 'failed') {
                    dotClass = 'bg-red-500';
                    dotTitle = 'Failed backup';
                }
                sevenDayDots += `<span class="w-2 h-2 ${dotClass} rounded-full" title="${dotTitle}"></span>`;
            });
        } else {
            for (let i = 0; i < 7; i++) {
                 sevenDayDots += '<span class="w-2 h-2 bg-gray-200 dark:bg-gray-700 rounded-full" title="Data unavailable"></span>'; // Placeholder if data is missing
            }
        }
        sevenDayDots += '</div>';

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100" title="${guestStatus.guestName}">${guestStatus.guestName}</td>
            <td class="p-1 px-2 text-gray-500 dark:text-gray-400">${guestStatus.guestId}</td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.node}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${latestBackupFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.pbsInstanceName}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.datastoreName}</td>
            <td class="p-1 px-2 text-gray-500 dark:text-gray-400">${guestStatus.totalBackups}</td>
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


    function updateBackupsTab() {
        const tableContainer = document.getElementById('backups-table-container');
        const tableBody = document.getElementById('backups-overview-tbody');
        const loadingMsg = document.getElementById('backups-loading-message');
        const noDataMsg = document.getElementById('backups-no-data-message');
        const statusTextElement = document.getElementById('backups-status-text');

        if (!tableContainer || !tableBody || !loadingMsg || !noDataMsg || !statusTextElement) {
            console.error("UI elements for Backups tab not found!");
            return;
        }

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

        const sortStateBackups = PulseApp.state.getSortState('backups');
        const sortedBackupStatus = PulseApp.utils.sortData(filteredBackupStatus, sortStateBackups.column, sortStateBackups.direction, 'backups');

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

        updateBackupsTab();
        PulseApp.state.saveFilterState(); // Save reset state
    }

    return {
        init,
        updateBackupsTab,
        resetBackupsView
    };
})();
