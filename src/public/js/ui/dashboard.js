PulseApp.ui = PulseApp.ui || {};

const FILTER_ALL = 'all';
const FILTER_VM = 'vm';
const FILTER_LXC = 'lxc';

const GUEST_TYPE_VM = 'VM';
const GUEST_TYPE_CT = 'CT';

const STATUS_RUNNING = 'running';
const STATUS_STOPPED = 'stopped';

const METRIC_CPU = 'cpu';
const METRIC_MEMORY = 'memory';
const METRIC_DISK = 'disk';
const METRIC_DISK_READ = 'diskread';
const METRIC_DISK_WRITE = 'diskwrite';
const METRIC_NET_IN = 'netin';
const METRIC_NET_OUT = 'netout';

PulseApp.ui.dashboard = (() => {
    let searchInput = null;
    let guestMetricDragSnapshot = {}; // To store metrics during slider drag
    let tableBodyEl = null;
    let statusElementEl = null;

    function init() {
        searchInput = document.getElementById('dashboard-search');
        tableBodyEl = document.querySelector('#main-table tbody');
        statusElementEl = document.getElementById('dashboard-status-text');
    }

    function _calculateAverage(historyArray, key) {
        if (!historyArray || historyArray.length === 0) return null;
        const validEntries = historyArray.filter(entry => typeof entry[key] === 'number' && !isNaN(entry[key]));
        if (validEntries.length === 0) return null;
        const sum = validEntries.reduce((acc, curr) => acc + curr[key], 0);
        return sum / validEntries.length;
    }

    function _calculateAverageRate(historyArray, key) {
        if (!historyArray || historyArray.length < 2) return null;
        const validHistory = historyArray.filter(entry =>
            typeof entry.timestamp === 'number' && !isNaN(entry.timestamp) &&
            typeof entry[key] === 'number' && !isNaN(entry[key])
        );

        if (validHistory.length < 2) return null;

        const oldest = validHistory[0];
        const newest = validHistory[validHistory.length - 1];
        const valueDiff = newest[key] - oldest[key];
        const timeDiffSeconds = (newest.timestamp - oldest.timestamp) / 1000;

        if (timeDiffSeconds <= 0) return null;
        if (valueDiff < 0) return null;
        
        return valueDiff / timeDiffSeconds;
    }

    function _processSingleGuestData(guest) {
        let avgCpu = 0, avgMem = 0, avgDisk = 0;
        let avgDiskReadRate = null, avgDiskWriteRate = null, avgNetInRate = null, avgNetOutRate = null;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';
        let effectiveMemorySource = 'host';
        let currentMemForAvg = 0;
        let currentMemTotalForDisplay = guest.maxmem;

        const metricsData = PulseApp.state.get('metricsData') || [];
        const metrics = metricsData.find(m =>
            m.id === guest.vmid &&
            m.type === guest.type &&
            m.node === guest.node &&
            m.endpointId === guest.endpointId
        );
        const guestUniqueId = guest.id;

        const isDragging = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();
        const snapshot = guestMetricDragSnapshot[guestUniqueId];

        if (isDragging && snapshot) {
            avgDiskReadRate = snapshot.diskread;
            avgDiskWriteRate = snapshot.diskwrite;
            avgNetInRate = snapshot.netin;
            avgNetOutRate = snapshot.netout;

            if (guest.status === STATUS_RUNNING && metrics && metrics.current) {
                const currentDataPoint = { timestamp: Date.now(), ...metrics.current };
                PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];
                avgCpu = _calculateAverage(history, 'cpu') ?? 0;
                avgMem = _calculateAverage(history, 'mem') ?? 0;
                avgDisk = _calculateAverage(history, 'disk') ?? 0;
            } else {
                PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }
        } else {
            if (guest.status === STATUS_RUNNING && metrics && metrics.current) {
                let baseMemoryValue = metrics.current.mem;
                currentMemTotalForDisplay = guest.maxmem;
                effectiveMemorySource = 'host';

                if (metrics.current.guest_mem_actual_used_bytes !== undefined && metrics.current.guest_mem_actual_used_bytes !== null) {
                    baseMemoryValue = metrics.current.guest_mem_actual_used_bytes;
                    effectiveMemorySource = 'guest';
                    if (metrics.current.guest_mem_total_bytes !== undefined && metrics.current.guest_mem_total_bytes > 0) {
                        currentMemTotalForDisplay = metrics.current.guest_mem_total_bytes;
                    }
                }
                currentMemForAvg = baseMemoryValue;

                const currentDataPoint = {
                    timestamp: Date.now(),
                    ...metrics.current,
                    effective_mem: currentMemForAvg,
                    effective_mem_total: currentMemTotalForDisplay,
                    effective_mem_source: effectiveMemorySource
                };
                PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];

                avgCpu = _calculateAverage(history, 'cpu') ?? 0;
                avgMem = _calculateAverage(history, 'effective_mem') ?? 0;
                avgDisk = _calculateAverage(history, 'disk') ?? 0;
                avgDiskReadRate = _calculateAverageRate(history, 'diskread');
                avgDiskWriteRate = _calculateAverageRate(history, 'diskwrite');
                avgNetInRate = _calculateAverageRate(history, 'netin');
                avgNetOutRate = _calculateAverageRate(history, 'netout');
            } else {
                PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }
        }

        const historyForGuest = PulseApp.state.getDashboardHistory()[guestUniqueId];
        let finalMemTotalForPercent = guest.maxmem;
        let finalMemSourceForTooltip = 'host';

        if (historyForGuest && historyForGuest.length > 0) {
            const lastHistoryEntry = historyForGuest[historyForGuest.length - 1];
            if (lastHistoryEntry.effective_mem_total !== undefined && lastHistoryEntry.effective_mem_total > 0) {
                finalMemTotalForPercent = lastHistoryEntry.effective_mem_total;
            }
            if (lastHistoryEntry.effective_mem_source) {
                finalMemSourceForTooltip = lastHistoryEntry.effective_mem_source;
            }
        }

        avgMemoryPercent = (finalMemTotalForPercent > 0 && typeof avgMem === 'number') ? Math.round(avgMem / finalMemTotalForPercent * 100) : 'N/A';
        avgDiskPercent = (guest.maxdisk > 0 && typeof avgDisk === 'number') ? Math.round(avgDisk / guest.maxdisk * 100) : 'N/A';
        
        let rawHostReportedMem = null;
        if (guest.status === STATUS_RUNNING && metrics && metrics.current && metrics.current.mem !== undefined) {
            rawHostReportedMem = metrics.current.mem;
        }

        return {
            id: guest.vmid,
            uniqueId: guestUniqueId,
            vmid: guest.vmid,
            name: guest.name || `${guest.type === 'qemu' ? GUEST_TYPE_VM : GUEST_TYPE_CT} ${guest.vmid}`,
            node: guest.node,
            type: guest.type === 'qemu' ? GUEST_TYPE_VM : GUEST_TYPE_CT,
            status: guest.status,
            cpu: avgCpu,
            cpus: guest.cpus || 1,
            memory: avgMemoryPercent,
            memoryCurrent: avgMem,
            memoryTotal: finalMemTotalForPercent,
            memorySource: finalMemSourceForTooltip,
            rawHostMemory: rawHostReportedMem,
            disk: avgDiskPercent,
            diskCurrent: avgDisk,
            diskTotal: guest.maxdisk,
            uptime: guest.status === STATUS_RUNNING ? guest.uptime : 0,
            diskread: avgDiskReadRate,
            diskwrite: avgDiskWriteRate,
            netin: avgNetInRate,
            netout: avgNetOutRate
        };
    }

    function _setDashboardColumnWidths(dashboardData) {
        let maxNameLength = 0;
        let maxUptimeLength = 0;

        dashboardData.forEach(guest => {
            const uptimeFormatted = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.name.length > maxNameLength) maxNameLength = guest.name.length;
            if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;
        });

        const nameColWidth = Math.min(Math.max(maxNameLength * 8 + 16, 100), 300);
        const uptimeColWidth = Math.max(maxUptimeLength * 7 + 16, 80);
        const htmlElement = document.documentElement;
        if (htmlElement) {
            htmlElement.style.setProperty('--name-col-width', `${nameColWidth}px`);
            htmlElement.style.setProperty('--uptime-col-width', `${uptimeColWidth}px`);
        }
    }

    function refreshDashboardData() {
        PulseApp.state.set('dashboardData', []);
        let dashboardData = [];

        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];

        vmsData.forEach(vm => dashboardData.push(_processSingleGuestData(vm)));
        containersData.forEach(ct => dashboardData.push(_processSingleGuestData(ct)));
        
        PulseApp.state.set('dashboardData', dashboardData);
        _setDashboardColumnWidths(dashboardData);
    }

    function _filterDashboardData(dashboardData, searchInput, filterGuestType, filterStatus, thresholdState) {
        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];

        return dashboardData.filter(guest => {
            const typeMatch = filterGuestType === FILTER_ALL ||
                              (filterGuestType === FILTER_VM && guest.type === GUEST_TYPE_VM) ||
                              (filterGuestType === FILTER_LXC && guest.type === GUEST_TYPE_CT);
            const statusMatch = filterStatus === FILTER_ALL || guest.status === filterStatus;

            const searchMatch = textSearchTerms.length === 0 || textSearchTerms.some(term =>
                (guest.name && guest.name.toLowerCase().includes(term)) ||
                (guest.node && guest.node.toLowerCase().includes(term)) ||
                (guest.vmid && guest.vmid.toString().includes(term)) ||
                (guest.uniqueId && guest.uniqueId.toString().includes(term))
            );

            let thresholdsMet = true;
            for (const type in thresholdState) {
                const state = thresholdState[type];
                let guestValue;

                if (type === METRIC_CPU) guestValue = guest.cpu * 100;
                else if (type === METRIC_MEMORY) guestValue = guest.memory;
                else if (type === METRIC_DISK) guestValue = guest.disk;
                else if (type === METRIC_DISK_READ) guestValue = guest.diskread;
                else if (type === METRIC_DISK_WRITE) guestValue = guest.diskwrite;
                else if (type === METRIC_NET_IN) guestValue = guest.netin;
                else if (type === METRIC_NET_OUT) guestValue = guest.netout;
                else continue;

                if (state.value > 0) {
                    if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) {
                        thresholdsMet = false;
                        break;
                    }
                    if (!(guestValue >= state.value)) {
                        thresholdsMet = false;
                        break;
                    }
                }
            }
            return typeMatch && statusMatch && searchMatch && thresholdsMet;
        });
    }

    function _renderGroupedByNode(tableBody, sortedData, createRowFn) {
        const nodeGroups = {};
        let visibleNodes = new Set();
        let visibleCount = 0;

        sortedData.forEach(guest => {
            const nodeName = guest.node || 'Unknown Node';
            if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
            nodeGroups[nodeName].push(guest);
        });
        
        tableBody.innerHTML = '';

        Object.keys(nodeGroups).sort().forEach(nodeName => {
            visibleNodes.add(nodeName.toLowerCase());
            const nodeHeaderRow = document.createElement('tr');
            nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
            nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 11, 'td');
            tableBody.appendChild(nodeHeaderRow);

            nodeGroups[nodeName].forEach(guest => {
                const guestRow = createRowFn(guest);
                if (guestRow) {
                    tableBody.appendChild(guestRow);
                    visibleCount++;
                }
            });
        });
        return { visibleCount, visibleNodes };
    }

    function _updateDashboardStatusMessage(statusElement, visibleCount, visibleNodes, groupByNode, filterGuestType, filterStatus, searchInput, thresholdState) {
        if (!statusElement) return;
        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];
        
        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = textSearchTerms.length > 0 ? ` | Search: "${textSearchTerms.join(', ')}"` : '';
        const typeLabel = filterGuestType !== FILTER_ALL ? filterGuestType.toUpperCase() : '';
        const statusLabel = filterStatus !== FILTER_ALL ? filterStatus : '';
        const otherFilters = [typeLabel, statusLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        
        const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
        if (activeThresholds.length > 0) {
            const thresholdTexts = activeThresholds.map(([key, state]) => {
                return `${PulseApp.utils.getReadableThresholdName(key)}>=${PulseApp.utils.formatThresholdValue(key, state.value)}`;
            });
            statusFilterText += ` | Thresholds: ${thresholdTexts.join(', ')}`;
        }

        let statusCountText = ` | Showing ${visibleCount} guests`;
        if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
        statusElement.textContent = statusBaseText + statusFilterText + statusCountText;
    }


    function updateDashboardTable() {
        if (!tableBodyEl || !statusElementEl) {
            console.error('Dashboard table body or status element not found/initialized!');
            return;
        }

        refreshDashboardData();

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const thresholdState = PulseApp.state.getThresholdState();
        const groupByNode = PulseApp.state.get('groupByNode');

        const filteredData = _filterDashboardData(dashboardData, searchInput, filterGuestType, filterStatus, thresholdState);
        const sortStateMain = PulseApp.state.getSortState('main');
        const sortedData = PulseApp.utils.sortData(filteredData, sortStateMain.column, sortStateMain.direction, 'main');

        let visibleCount = 0;
        let visibleNodes = new Set();

        if (groupByNode) {
            const groupRenderResult = _renderGroupedByNode(tableBodyEl, sortedData, createGuestRow);
            visibleCount = groupRenderResult.visibleCount;
            visibleNodes = groupRenderResult.visibleNodes;
        } else {
            PulseApp.utils.renderTableBody(tableBodyEl, sortedData, createGuestRow, "No matching guests found.", 11);
            visibleCount = sortedData.length;
            sortedData.forEach(guest => visibleNodes.add((guest.node || 'Unknown Node').toLowerCase()));
        }

        if (visibleCount === 0 && tableBodyEl) {
            let filterDescription = [];
            if (filterGuestType !== FILTER_ALL) filterDescription.push(`Type: ${filterGuestType.toUpperCase()}`);
            if (filterStatus !== FILTER_ALL) filterDescription.push(`Status: ${filterStatus}`);
            const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];
            if (textSearchTerms.length > 0) filterDescription.push(`Search: "${textSearchTerms.join(', ')}"`);
            
            const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
            if (activeThresholds.length > 0) {
                const thresholdTexts = activeThresholds.map(([key, state]) => {
                    return `${PulseApp.utils.getReadableThresholdName(key)}>=${PulseApp.utils.formatThresholdValue(key, state.value)}`;
                });
                filterDescription.push(`Thresholds: ${thresholdTexts.join(', ')}`);
            }
            let message = "No guests match the current filters";
            if (filterDescription.length > 0) message += ` (${filterDescription.join('; ')})`;
            message += ".";
            tableBodyEl.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
        }
        
        _updateDashboardStatusMessage(statusElementEl, visibleCount, visibleNodes, groupByNode, filterGuestType, filterStatus, searchInput, thresholdState);

        const mainSortColumn = sortStateMain.column;
        const mainHeader = document.querySelector(`#main-table th[data-sort="${mainSortColumn}"]`);
        if (PulseApp.ui && PulseApp.ui.common) {
            if (mainHeader) {
                PulseApp.ui.common.updateSortUI('main-table', mainHeader);
            } else {
                console.warn(`Sort header for column '${mainSortColumn}' not found in main table.`);
            }
        } else {
            console.warn('[Dashboard] PulseApp.ui.common not available for updateSortUI');
        }
    }

    function _createCpuBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        const cpuPercent = Math.round(guest.cpu * 100);
        const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : ''}`;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent);
        return PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
    }

    function _createMemoryBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        const memoryPercent = guest.memory; // This is already a percentage
        let memoryTooltipText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)} (${memoryPercent}%)`;
        if (guest.type === GUEST_TYPE_VM && guest.memorySource === 'guest' && guest.rawHostMemory !== null && guest.rawHostMemory !== undefined) {
            memoryTooltipText += ` (Host: ${PulseApp.utils.formatBytes(guest.rawHostMemory)})`;
        }
        const memColorClass = PulseApp.utils.getUsageColor(memoryPercent);
        return PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);
    }

    function _createDiskBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        if (guest.type === GUEST_TYPE_CT) {
            const diskPercent = guest.disk; // This is already a percentage
            const diskTooltipText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
            const diskColorClass = PulseApp.utils.getUsageColor(diskPercent);
            return PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
        } else { // For VMs, show total disk size, not a progress bar
            if (guest.diskTotal) {
                return `<span class="text-xs text-gray-700 dark:text-gray-200 truncate">${PulseApp.utils.formatBytes(guest.diskTotal)}</span>`;
            }
            return '-';
        }
    }

    function createGuestRow(guest) {
        const row = document.createElement('tr');
        row.className = `border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 ${guest.status === STATUS_STOPPED ? 'opacity-60 grayscale' : ''}`;
        row.setAttribute('data-name', guest.name.toLowerCase());
        row.setAttribute('data-type', guest.type.toLowerCase());
        row.setAttribute('data-node', guest.node.toLowerCase());
        row.setAttribute('data-id', guest.id);

        const cpuBarHTML = _createCpuBarHtml(guest);
        const memoryBarHTML = _createMemoryBarHtml(guest);
        const diskBarHTML = _createDiskBarHtml(guest);

        const diskReadFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.diskread, 0) : '-';
        const diskWriteFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.diskwrite, 0) : '-';

        // Icons and colors
        const upArrow = '↑';
        const downArrow = '↓';

        let netInIcon = '';
        let netOutIcon = '';

        if (guest.status === STATUS_RUNNING) {
            const netInActive = guest.netin > 0;
            const netOutActive = guest.netout > 0;

            netInIcon = `<span class="text-xs mr-1 ${netInActive ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'}">${downArrow}</span>`;
            netOutIcon = `<span class="text-xs mr-1 ${netOutActive ? 'text-red-500' : 'text-gray-400 dark:text-gray-500'}">${upArrow}</span>`;
        } else {
            netInIcon = `<span class="text-xs mr-1 text-gray-400 dark:text-gray-500">${downArrow}</span>`;
            netOutIcon = `<span class="text-xs mr-1 text-gray-400 dark:text-gray-500">${upArrow}</span>`;
        }
        
        const netInFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.netin, 0) : '-';
        const netOutFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.netout, 0) : '-';

        const typeIconClass = guest.type === GUEST_TYPE_VM
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;

        let uptimeDisplay = '-';
        if (guest.status === STATUS_RUNNING) {
            uptimeDisplay = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.uptime < 3600) { // Less than 1 hour (3600 seconds)
                uptimeDisplay = `<span class="text-orange-500">${uptimeDisplay}</span>`;
            }
        }

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2">${guest.id}</td>
            <td class="p-1 px-2 whitespace-nowrap">${uptimeDisplay}</td>
            <td class="p-1 px-2">${cpuBarHTML}</td>
            <td class="p-1 px-2">${memoryBarHTML}</td>
            <td class="p-1 px-2">${diskBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap">${diskReadFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${diskWriteFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${netInIcon}${netInFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${netOutIcon}${netOutFormatted}</td>
        `;
        return row;
    }

    function snapshotGuestMetricsForDrag() {
        guestMetricDragSnapshot = {}; // Clear previous snapshot
        const currentDashboardData = PulseApp.state.get('dashboardData') || [];
        currentDashboardData.forEach(guest => {
            if (guest && guest.uniqueId) {
                guestMetricDragSnapshot[guest.uniqueId] = {
                    diskread: guest.diskread,
                    diskwrite: guest.diskwrite,
                    netin: guest.netin,
                    netout: guest.netout
                    // Optionally snapshot other metrics if they also show issues
                };
            }
        });
    }

    function clearGuestMetricSnapshots() {
        guestMetricDragSnapshot = {};
    }

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow,
        snapshotGuestMetricsForDrag, // Export snapshot function
        clearGuestMetricSnapshots    // Export clear function
    };
})();
