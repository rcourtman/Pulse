PulseApp.thresholds = PulseApp.thresholds || {};

PulseApp.thresholds.logging = (() => {
    let clearAllLogsButton = null;

    function init() {
        clearAllLogsButton = document.getElementById('clear-all-logs-button');

        const startLogButton = document.getElementById('start-log-button');
        if (startLogButton) {
            startLogButton.addEventListener('click', startThresholdLogging);
        } else {
            console.warn('#start-log-button not found.');
        }

        if (clearAllLogsButton) {
            clearAllLogsButton.addEventListener('click', clearAllFinishedLogs);
        } else {
            console.warn('#clear-all-logs-button not found.');
        }

        updateClearAllButtonVisibility(); // Initial check
    }

    function startThresholdLogging() {
        const thresholdState = PulseApp.state.getThresholdState();
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const searchInput = document.getElementById('dashboard-search');

        const snapshottedThresholds = {};
        let activeThresholdCount = 0;
        let criteriaDescThresholds = [];
        for (const type in thresholdState) {
            const value = thresholdState[type].value;
            if (value > 0) {
                snapshottedThresholds[type] = value;
                activeThresholdCount++;
                criteriaDescThresholds.push(`${PulseApp.utils.getReadableThresholdName(type)}>=${PulseApp.utils.formatThresholdValue(type, value)}`);
            }
        }

        const snapshottedFilterGuestType = filterGuestType;
        const snapshottedFilterStatus = filterStatus;
        let criteriaDescFilters = [];
        if (snapshottedFilterGuestType !== 'all') criteriaDescFilters.push(`Type: ${snapshottedFilterGuestType.toUpperCase()}`);
        if (snapshottedFilterStatus !== 'all') criteriaDescFilters.push(`Status: ${snapshottedFilterStatus}`);

        const snapshottedRawSearch = searchInput ? searchInput.value.toLowerCase() : '';
        const snapshottedSearchTerms = snapshottedRawSearch.split(',').map(term => term.trim()).filter(term => term);
        let criteriaDescSearch = snapshottedSearchTerms.length > 0 ? `Search: "${snapshottedSearchTerms.join(', ')}"` : null;

        if (activeThresholdCount === 0 && snapshottedSearchTerms.length === 0 && criteriaDescFilters.length === 0) {
            alert("Please set at least one threshold, enter search terms, or select a Type/Status filter before starting the log.");
            const toggleThresholdsButton = document.getElementById('toggle-thresholds-button');
            if (toggleThresholdsButton && !PulseApp.state.get('isThresholdRowVisible') && activeThresholdCount === 0) {
                toggleThresholdsButton.click();
            }
            return;
        }

        const sessionId = Date.now();
        const startTime = new Date();
        let fullCriteriaDesc = [criteriaDescFilters.join('; '), criteriaDescSearch, criteriaDescThresholds.length > 0 ? `Thresholds: ${criteriaDescThresholds.join(', ')}`: null].filter(Boolean).join('; ');
        const sessionTitle = `Log @ ${startTime.toLocaleTimeString()}${fullCriteriaDesc ? ` (${fullCriteriaDesc})` : ''}`;

        const logContentContainer = PulseApp.ui.tabs.addLogTab(sessionId, sessionTitle, fullCriteriaDesc);
        if (!logContentContainer) return; // Stop if tab/content creation failed

        const panel = document.createElement('div');
        panel.id = `log-session-panel-${sessionId}`;
        panel.className = 'log-session-panel border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 shadow-md';

        const header = document.createElement('div');
        header.className = 'log-session-header flex justify-between items-center p-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50 rounded-t';

        const titleSpan = document.createElement('span');
        titleSpan.className = 'text-sm font-medium text-gray-700 dark:text-gray-200 truncate';
        titleSpan.textContent = sessionTitle;
        titleSpan.title = sessionTitle;

        const panelCloseButton = document.createElement('button');
        panelCloseButton.className = 'p-1 rounded text-gray-500 hover:bg-red-100 dark:text-gray-400 dark:hover:bg-red-800/50';
        panelCloseButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
        panelCloseButton.title = 'Close Log Panel & Stop Log (if active)';

        panelCloseButton.onclick = (event) => {
            const panelToRemove = event.currentTarget.closest('.log-session-panel');
            if (panelToRemove) {
                const panelSessionIdStr = panelToRemove.id.replace('log-session-panel-', '');
                PulseApp.ui.tabs.removeLogTabAndContent(panelSessionIdStr);
                // Stop session is handled implicitly by removeLogTabAndContent calling the tab close handler
            }
        };

        header.appendChild(titleSpan);
        header.appendChild(panelCloseButton);

        const tableContainer = document.createElement('div');
        tableContainer.className = 'log-table-container p-2 max-h-96 overflow-y-auto';

        const table = document.createElement('table');
        table.id = `log-table-${sessionId}`;
        table.className = 'min-w-full text-xs border-collapse';
        table.innerHTML = `
            <thead class="sticky top-0 bg-gray-100 dark:bg-gray-700 z-10">
                <tr class="border-b border-gray-300 dark:border-gray-600">
                    <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Time</th>
                    <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Guest</th>
                    <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Node</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">CPU</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">Mem</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">Disk%</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">DRead</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">DWrite</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">NetIn</th>
                    <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">NetOut</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                 <tr class="initial-log-message">
                    <td colspan="10" class="p-2 text-center text-gray-500 dark:text-gray-400 italic">
                        Logging started continuously<span class="dot-animate">.</span><span class="dot-animate">.</span><span class="dot-animate">.</span>
                    </td>
                 </tr>
            </tbody>
        `;

        tableContainer.appendChild(table);
        panel.appendChild(header);
        panel.appendChild(tableContainer);
        logContentContainer.appendChild(panel); // Add the panel to the content area created by addLogTab

        PulseApp.state.addActiveLogSession(sessionId, {
            thresholds: snapshottedThresholds,
            startTime: startTime,
            entries: [],
            element: panel,
            searchTerms: snapshottedSearchTerms,
            filterGuestType: snapshottedFilterGuestType,
            filterStatus: snapshottedFilterStatus
        });

        console.log(`[Log Session ${sessionId}] Started. Thresholds:`, snapshottedThresholds);
        updateClearAllButtonVisibility(); // Might hide the button if only active logs exist
    }

    function stopThresholdLogging(sessionId, reason = 'manual') {
        const session = PulseApp.state.getActiveLogSession(sessionId);
        if (!session) {
            console.warn(`Attempted to stop non-existent log session: ${sessionId}`);
            return;
        }

        console.log(`[Log Session ${sessionId}] Stopping. Reason: ${reason}`);

        if (session.element) {
            const tableBody = session.element.querySelector(`#log-table-${sessionId} tbody`);
            if (tableBody) {
                const initialMsgRow = tableBody.querySelector('.initial-log-message');
                if (initialMsgRow) initialMsgRow.remove();

                const finalStatusRow = tableBody.insertRow(-1);
                finalStatusRow.className = 'final-log-message';
                const cell = finalStatusRow.insertCell(0);
                cell.colSpan = 10; // Adjusted colspan
                cell.className = 'p-1 px-2 text-center text-xs text-gray-500 dark:text-gray-400 italic border-t border-gray-200 dark:border-gray-700';
                const stopTime = new Date().toLocaleTimeString();
                if (reason === 'timer') {
                    cell.textContent = `Logging finished (timer expired) at ${stopTime}`;
                } else {
                    cell.textContent = `Logging stopped manually at ${stopTime}`;
                }
            }
        }

        PulseApp.state.removeActiveLogSession(sessionId);
        updateClearAllButtonVisibility(); // Show the button as this log is now finished
    }

    function addLogRow(sessionId, entry) {
        const session = PulseApp.state.getActiveLogSession(sessionId);
        if (!session || !session.element) return;

        const tableBody = session.element.querySelector(`#log-table-${sessionId} tbody`);
        if (!tableBody) return;

        const initialMsgRow = tableBody.querySelector('.initial-log-message');
        if (initialMsgRow) initialMsgRow.remove();

        const row = tableBody.insertRow(0);
        row.className = 'bg-yellow-50 dark:bg-yellow-900/20 animate-pulse-once';

        const cpuValueHTML = entry.activeThresholdKeys.includes('cpu') ? `<strong>${entry.cpuFormatted}</strong>` : entry.cpuFormatted;
        const memValueHTML = entry.activeThresholdKeys.includes('memory') ? `<strong>${entry.memFormatted}</strong>` : entry.memFormatted;
        const diskValueHTML = entry.activeThresholdKeys.includes('disk') ? `<strong>${entry.diskFormatted}</strong>` : entry.diskFormatted;
        const diskReadValueHTML = entry.activeThresholdKeys.includes('diskread') ? `<strong>${entry.diskReadFormatted}</strong>` : entry.diskReadFormatted;
        const diskWriteValueHTML = entry.activeThresholdKeys.includes('diskwrite') ? `<strong>${entry.diskWriteFormatted}</strong>` : entry.diskWriteFormatted;
        const netInValueHTML = entry.activeThresholdKeys.includes('netin') ? `<strong>${entry.netInFormatted}</strong>` : entry.netInFormatted;
        const netOutValueHTML = entry.activeThresholdKeys.includes('netout') ? `<strong>${entry.netOutFormatted}</strong>` : entry.netOutFormatted;

        const guestDisplayHTML = entry.guestMatchedSearch ? `<strong>${entry.guestName} (${entry.guestId})</strong>` : `${entry.guestName} (${entry.guestId})`;
        const nodeDisplayHTML = entry.nodeMatchedSearch ? `<strong>${entry.node}</strong>` : entry.node;

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap">${entry.timestamp.toLocaleTimeString()}</td>
            <td class="p-1 px-2 whitespace-nowrap" title="${entry.guestName}">${guestDisplayHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap">${nodeDisplayHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${cpuValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${memValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${diskValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${diskReadValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${diskWriteValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${netInValueHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-right">${netOutValueHTML}</td>
        `;

        tableBody.prepend(row);

         setTimeout(() => {
              row.classList.remove('bg-yellow-50', 'dark:bg-yellow-900/20', 'animate-pulse-once');
         }, 1500);

         PulseApp.state.addLogEntry(sessionId, entry);
    }

    function checkThresholdViolations() {
        const activeSessions = PulseApp.state.getAllActiveLogSessions();
        if (Object.keys(activeSessions).length === 0) {
            return;
        }

        const now = new Date();
        const dashboardData = PulseApp.state.get('dashboardData') || [];

        dashboardData.forEach(guest => {
            if (guest.status !== 'running') return;

            Object.keys(activeSessions).forEach(sessionId => {
                const session = activeSessions[sessionId];
                if (!session) return;

                const typeMatch = session.filterGuestType === 'all' ||
                                  (session.filterGuestType === 'vm' && guest.type === 'VM') ||
                                  (session.filterGuestType === 'lxc' && guest.type === 'CT');
                if (!typeMatch) return;

                const statusMatch = session.filterStatus === 'all' || guest.status === session.filterStatus;
                if (!statusMatch) return;

                const snapshottedSearch = session.searchTerms || [];
                let guestMatchedSearch = false;
                let nodeMatchedSearch = false;
                let overallSearchMatch = false;

                if (snapshottedSearch.length > 0) {
                    snapshottedSearch.forEach(term => {
                        if (!guestMatchedSearch && (
                            (guest.name && guest.name.toLowerCase().includes(term)) ||
                            (guest.vmid && guest.vmid.toString().includes(term)) ||
                            (guest.uniqueId && guest.uniqueId.toString().includes(term))
                        )) {
                            guestMatchedSearch = true;
                        }
                        if (!nodeMatchedSearch && (
                            (guest.node && guest.node.toLowerCase().includes(term))
                        )) {
                            nodeMatchedSearch = true;
                        }
                    });
                    overallSearchMatch = guestMatchedSearch || nodeMatchedSearch;
                    if (!overallSearchMatch) return;
                } else {
                    overallSearchMatch = true;
                }

                let meetsAllThresholds = true;
                let violationDetails = [];

                for (const type in session.thresholds) {
                    const thresholdValue = session.thresholds[type];
                    if (thresholdValue <= 0) continue;

                    let guestValue;
                    if (type === 'cpu') guestValue = guest.cpu * 100;
                    else if (type === 'memory') guestValue = guest.memory;
                    else if (type === 'disk') guestValue = guest.disk;
                    else if (type === 'diskread') guestValue = guest.diskread;
                    else if (type === 'diskwrite') guestValue = guest.diskwrite;
                    else if (type === 'netin') guestValue = guest.netin;
                    else if (type === 'netout') guestValue = guest.netout;
                    else continue;

                    if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) {
                        continue;
                    }

                    if (guestValue < thresholdValue) {
                        meetsAllThresholds = false;
                        break;
                    }
                }

                if (meetsAllThresholds) {
                    const metricsSnapshot = {
                        cpu: guest.cpu * 100,
                        mem: guest.memory,
                        disk: guest.disk,
                        diskRead: guest.diskread,
                        diskWrite: guest.diskwrite,
                        netIn: guest.netin,
                        netOut: guest.netout
                    };

                    const logEntry = {
                        timestamp: now,
                        guestId: guest.vmid,
                        guestName: guest.name,
                        node: guest.node,
                        cpuFormatted: PulseApp.utils.formatThresholdValue('cpu', metricsSnapshot.cpu),
                        memFormatted: PulseApp.utils.formatThresholdValue('memory', metricsSnapshot.mem),
                        diskFormatted: PulseApp.utils.formatThresholdValue('disk', metricsSnapshot.disk),
                        diskReadFormatted: PulseApp.utils.formatThresholdValue('diskread', metricsSnapshot.diskRead),
                        diskWriteFormatted: PulseApp.utils.formatThresholdValue('diskwrite', metricsSnapshot.diskWrite),
                        netInFormatted: PulseApp.utils.formatThresholdValue('netin', metricsSnapshot.netIn),
                        netOutFormatted: PulseApp.utils.formatThresholdValue('netout', metricsSnapshot.netOut),
                        metricsRaw: metricsSnapshot,
                        activeThresholdKeys: Object.keys(session.thresholds),
                        guestMatchedSearch: guestMatchedSearch,
                        nodeMatchedSearch: nodeMatchedSearch
                    };

                    const lastEntry = session.entries.length > 0 ? session.entries[0] : null;
                    if (!lastEntry ||
                        !(lastEntry.guestId === logEntry.guestId &&
                          (now.getTime() - lastEntry.timestamp.getTime()) < 1000)
                       )
                    {
                          addLogRow(sessionId, logEntry);
                    }
                }
            });
        });
    }

    function updateClearAllButtonVisibility() {
        if (!clearAllLogsButton) return;
        const nestedTabsContainer = document.querySelector('.nested-tabs');
        if (!nestedTabsContainer) return;

        let hasFinishedLogs = false;
        const activeSessions = PulseApp.state.getAllActiveLogSessions();
        nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').forEach(tab => {
            const sessionId = tab.dataset.nestedTab.replace('log-session-', '');
            if (!activeSessions[sessionId]) {
                hasFinishedLogs = true;
            }
        });

        clearAllLogsButton.classList.toggle('hidden', !hasFinishedLogs);
    }

    function clearAllFinishedLogs() {
        const nestedTabsContainer = document.querySelector('.nested-tabs');
        const nestedTabContentContainer = document.querySelector('#log-content-area');
        const logSessionArea = document.getElementById('log-session-area');
        if (!nestedTabsContainer || !nestedTabContentContainer || !logSessionArea) return;

        const activeSessions = PulseApp.state.getAllActiveLogSessions();
        const tabsToRemove = [];
        const contentsToRemove = [];

        nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').forEach(tab => {
            const sessionIdFull = tab.dataset.nestedTab;
            if (!activeSessions[sessionIdFull.replace('log-session-', '')]) {
                tabsToRemove.push(tab);
                const content = nestedTabContentContainer.querySelector(`#${sessionIdFull}`);
                if (content) contentsToRemove.push(content);
            }
        });

        tabsToRemove.forEach(tab => tab.remove());
        contentsToRemove.forEach(content => content.remove());

         if (nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').length === 0) {
            logSessionArea.classList.add('hidden');
         }

        if (clearAllLogsButton) clearAllLogsButton.classList.add('hidden');
    }

    return {
        init,
        stopThresholdLogging,
        checkThresholdViolations,
        updateClearAllButtonVisibility
    };
})(); 