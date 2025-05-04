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
            backupsSearchInput.addEventListener('input', updateBackupsTab);
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

        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const initialDataReceived = PulseApp.state.get('initialDataReceived');

        const allGuests = [...vmsData, ...containersData];

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
            return;
        }

        loadingMsg.classList.add('hidden');

        const backupStatusByGuest = [];
        const now = Math.floor(Date.now() / 1000);
        const sevenDaysAgo = now - (7 * 24 * 60 * 60);
        const threeDaysAgo = now - (3 * 24 * 60 * 60);

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

        allGuests.forEach(guest => {
            const guestId = String(guest.vmid);
            const guestTypePve = guest.type === 'qemu' ? 'vm' : 'ct';

            const guestSnapshots = allSnapshots.filter(snap =>
                String(snap.backupVMID) === guestId && snap.backupType === guestTypePve
            );
            const totalBackups = guestSnapshots.length;
            const latestSnapshot = guestSnapshots.reduce((latest, snap) => {
                return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
            }, null);
            const latestSnapshotTime = latestSnapshot ? latestSnapshot['backup-time'] : null;

            const guestTasks = allRecentBackupTasks.filter(task =>
                task.guestId === guestId && task.guestTypePbs === guestTypePve
            );
            const latestTask = guestTasks.reduce((latest, task) => {
               return (!latest || (task.startTime && task.startTime > latest.startTime)) ? task : latest;
            }, null);

            let healthStatus = 'none';
            let displayTimestamp = latestSnapshotTime;

            if (latestTask) {
                displayTimestamp = latestTask.startTime;
                if (latestTask.status === 'OK') {
                    if (latestTask.startTime >= threeDaysAgo) {
                        healthStatus = 'ok';
                    } else if (latestTask.startTime >= sevenDaysAgo) {
                        healthStatus = 'stale';
                    } else {
                        healthStatus = 'old';
                    }
                } else {
                    healthStatus = 'failed';
                }
            } else if (latestSnapshotTime) {
                 if (latestSnapshotTime >= threeDaysAgo) {
                     healthStatus = 'ok';
                 } else if (latestSnapshotTime >= sevenDaysAgo) {
                     healthStatus = 'stale';
                 } else {
                     healthStatus = 'old';
                 }
            } else {
                healthStatus = 'none';
                displayTimestamp = null;
            }

            backupStatusByGuest.push({
                guestName: guest.name || `Guest ${guest.vmid}`,
                guestId: guest.vmid,
                guestType: guest.type === 'qemu' ? 'VM' : 'LXC',
                node: guest.node,
                guestPveStatus: guest.status,
                latestBackupTime: displayTimestamp,
                pbsInstanceName: latestSnapshot?.pbsInstanceName || latestTask?.pbsInstanceName || 'N/A',
                datastoreName: latestSnapshot?.datastoreName || 'N/A',
                totalBackups: totalBackups,
                backupHealthStatus: healthStatus
            });
        });

        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value.toLowerCase() : '';
        const backupsSearchTerms = currentBackupsSearchTerm.split(',').map(term => term.trim()).filter(term => term);
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');

        const filteredBackupStatus = backupStatusByGuest.filter(item => {
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
                const nameMatch = backupsSearchTerms.some(term =>
                    (item.guestName?.toLowerCase() || '').includes(term) ||
                    (item.node?.toLowerCase() || '').includes(term) ||
                    (item.guestId?.toString() || '').includes(term)
                );
                if (!nameMatch) return false;
            }

            return true;
        });

        const sortStateBackups = PulseApp.state.getSortState('backups');
        const sortedBackupStatus = PulseApp.utils.sortData(filteredBackupStatus, sortStateBackups.column, sortStateBackups.direction, 'backups');

        tableBody.innerHTML = '';
        let visibleCount = 0;
        if (sortedBackupStatus.length > 0) {
            sortedBackupStatus.forEach(guestStatus => {
                const row = tableBody.insertRow();
                row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px ${guestStatus.guestPveStatus === 'stopped' ? 'opacity-60 grayscale' : ''}`;

                const latestBackupFormatted = guestStatus.latestBackupTime
                    ? PulseApp.utils.formatPbsTimestamp(guestStatus.latestBackupTime)
                    : '<span class="text-gray-400">No backups found</span>';

                let healthIndicator = '';
                switch (guestStatus.backupHealthStatus) {
                    case 'ok':
                        healthIndicator = '<span class="text-green-600 dark:text-green-400" title="OK">●</span>';
                        break;
                    case 'stale':
                        healthIndicator = '<span class="text-yellow-600 dark:text-yellow-400" title="Stale">●</span>';
                        break;
                    case 'failed':
                        healthIndicator = '<span class="text-red-600 dark:text-red-400 font-bold" title="Failed">✖</span>';
                        break;
                    case 'old':
                         healthIndicator = '<span class="text-orange-600 dark:text-orange-400" title="Old">●</span>';
                         break;
                    case 'none':
                        healthIndicator = '<span class="text-gray-400 dark:text-gray-500" title="None">-</span>';
                        break;
                }

                const typeIconClass = guestStatus.guestType === 'VM'
                    ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
                    : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
                const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;

                row.innerHTML = `
                    <td class="p-1 px-2 whitespace-nowrap text-center">${healthIndicator}</td>
                    <td class="p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100" title="${guestStatus.guestName}">${guestStatus.guestName}</td>
                    <td class="p-1 px-2 text-center text-gray-500 dark:text-gray-400">${guestStatus.guestId}</td>
                    <td class="p-1 px-2 text-center">${typeIcon}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.node}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${latestBackupFormatted}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.pbsInstanceName}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.datastoreName}</td>
                    <td class="p-1 px-2 text-center text-gray-500 dark:text-gray-400">${guestStatus.totalBackups}</td>
                `;
                visibleCount++;
            });

            loadingMsg.classList.add('hidden');
            noDataMsg.classList.add('hidden');
            tableContainer.classList.remove('hidden');
        } else {
            loadingMsg.classList.add('hidden');
            tableContainer.classList.add('hidden');
            let emptyMessage = "No backup information found for any guests.";
            if (backupStatusByGuest.length === 0) {
                if (allGuests.length === 0) {
                     emptyMessage = "No Proxmox guests (VMs/Containers) found.";
                } else {
                    emptyMessage = "No backup information found for any guests.";
                }
            }
            else if (filteredBackupStatus.length === 0) {
                 const typeFilterText = backupsFilterGuestType === 'all' ? '' : `Type: ${backupsFilterGuestType.toUpperCase()}`;
                 const filtersApplied = [typeFilterText].filter(Boolean).join(', ');

                 if (filtersApplied) {
                   emptyMessage = `No guests found matching the selected filters (${filtersApplied}).`;
                 } else {
                   emptyMessage = "No guests with backup information found.";
                 }
            }
            if (filteredBackupStatus.length === 0 && backupsSearchTerms.length > 0) {
               emptyMessage = `No guests found matching search "${currentBackupsSearchTerm}".`;
               if (filtersApplied) {
                   emptyMessage += ` and filters (${filtersApplied})`;
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

        if (statusTextElement) {
            const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
            let statusFilterText = currentBackupsSearchTerm ? ` | Filter: "${currentBackupsSearchTerm}"` : '';
            const typeFilterLabel = backupsFilterGuestType !== 'all' ? backupsFilterGuestType.toUpperCase() : '';
            const healthFilterLabel = backupsFilterHealth !== 'all' ? backupsFilterHealth.charAt(0).toUpperCase() + backupsFilterHealth.slice(1) : '';
            const otherFilters = [typeFilterLabel, healthFilterLabel].filter(Boolean).join('/');
            if (otherFilters) {
                statusFilterText += ` | ${otherFilters}`;
            }
            let statusCountText = ` | Showing ${visibleCount} guests`;
            statusTextElement.textContent = statusBaseText + statusFilterText + statusCountText;
        }
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