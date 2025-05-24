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

        // Initialize chart system
        if (PulseApp.charts) {
            PulseApp.charts.startChartUpdates();
        }

        // Initialize charts toggle
        const chartsToggleButton = document.getElementById('toggle-charts-button');
        if (chartsToggleButton) {
            chartsToggleButton.addEventListener('click', toggleChartsMode);
        }

        document.addEventListener('keydown', (event) => {
            // Handle Escape for resetting filters
            if (event.key === 'Escape') {
                const resetButton = document.getElementById('reset-filters-button');
                if (resetButton) {
                    resetButton.click(); // This will clear search and update table
                }
                return; // Done with Escape key
            }

            // General conditions to ignore this global listener:
            // 1. If already typing in an input, textarea, or select element.
            const targetElement = event.target;
            const targetTagName = targetElement.tagName;
            if (targetTagName === 'INPUT' || targetTagName === 'TEXTAREA' || targetTagName === 'SELECT') {
                return;
            }

            // 2. If a modal is active (e.g., snapshot-modal prevents background interaction)
            const snapshotModal = document.getElementById('snapshot-modal');
            if (snapshotModal && !snapshotModal.classList.contains('hidden')) {
                return;
            }
            // Add similar checks for other modals if they exist and should block this behavior.

            if (searchInput) { // searchInput is the module-scoped variable
                // For printable characters (letters, numbers, symbols, space)
                // Check !event.ctrlKey && !event.metaKey to avoid conflict with browser/OS shortcuts.
                if (event.key.length === 1 && !event.ctrlKey && !event.metaKey) {
                    if (document.activeElement !== searchInput) {
                        searchInput.focus();
                        event.preventDefault(); // Prevent default action (e.g., page scroll, find dialog)
                        searchInput.value += event.key; // Append the typed character
                        searchInput.dispatchEvent(new Event('input', { bubbles: true, cancelable: true })); // Trigger update
                    }
                    // If searchInput is already focused, browser handles the typing.
                } else if (event.key === 'Backspace' && !event.ctrlKey && !event.metaKey && !event.altKey) {
                    // For Backspace, if search not focused, focus it. Prevent default (e.g., browser back).
                    if (document.activeElement !== searchInput) {
                        searchInput.focus();
                        event.preventDefault();
                    }
                    // If searchInput is already focused, browser handles Backspace.
                }
            }
        });
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
                const currentDataPoint = { 
                    timestamp: Date.now(), 
                    ...metrics.current,
                    // Convert CPU to percentage for consistency
                    cpu: (metrics.current.cpu || 0) * 100
                };
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
                    cpu: (metrics.current.cpu || 0) * 100,
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

        // More aggressive space optimization
        const nameColWidth = Math.min(Math.max(maxNameLength * 7 + 12, 80), 250);
        const uptimeColWidth = Math.max(maxUptimeLength * 6.5 + 8, 40);
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

                if (type === METRIC_CPU) guestValue = guest.cpu;
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

        // Update charts immediately after table is rendered, but only if in charts mode
        const mainContainer = document.getElementById('main');
        if (PulseApp.charts && visibleCount > 0 && mainContainer && mainContainer.classList.contains('charts-mode')) {
            // Use requestAnimationFrame to ensure DOM is fully updated
            requestAnimationFrame(() => {
                PulseApp.charts.updateAllCharts();
            });
        }
    }

    function _createCpuBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        const cpuPercent = Math.round(guest.cpu);
        const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus / 100).toFixed(1)}/${guest.cpus} cores)` : ''}`;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
        
        // Create both text and chart versions
        const guestId = guest.uniqueId;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'cpu') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createMemoryBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        const memoryPercent = guest.memory;
        let memoryTooltipText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)} (${memoryPercent}%)`;
        if (guest.type === GUEST_TYPE_VM && guest.memorySource === 'guest' && guest.rawHostMemory !== null && guest.rawHostMemory !== undefined) {
            memoryTooltipText += ` (Host: ${PulseApp.utils.formatBytes(guest.rawHostMemory)})`;
        }
        const memColorClass = PulseApp.utils.getUsageColor(memoryPercent, 'memory');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);
        
        // Create both text and chart versions
        const guestId = guest.uniqueId;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'memory') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createDiskBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        if (guest.type === GUEST_TYPE_CT) {
            const diskPercent = guest.disk;
            const diskTooltipText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
            const diskColorClass = PulseApp.utils.getUsageColor(diskPercent, 'disk');
            const progressBar = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
            
            // Create both text and chart versions
            const guestId = guest.uniqueId;
            const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'disk') : '';
            
            return `
                <div class="metric-text">${progressBar}</div>
                <div class="metric-chart">${chartHtml}</div>
            `;
        } else {
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

        const diskReadFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.diskread, 0) : '-';
        const diskWriteFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.diskwrite, 0) : '-';
        const netInFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.netin, 0) : '-';
        const netOutFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.netout, 0) : '-';

        // Create I/O cells with both text and chart versions
        const guestId = guest.uniqueId;
        
        let diskReadCell, diskWriteCell, netInCell, netOutCell;
        
        if (guest.status === STATUS_RUNNING && PulseApp.charts) {
            // Text versions - clean, no arrows
            const diskReadText = diskReadFormatted;
            const diskWriteText = diskWriteFormatted;
            const netInText = netInFormatted;
            const netOutText = netOutFormatted;
            
            // Chart versions - clean, no arrows
            const diskReadChart = PulseApp.charts.createSparklineHTML(guestId, 'diskread');
            const diskWriteChart = PulseApp.charts.createSparklineHTML(guestId, 'diskwrite');
            const netInChart = PulseApp.charts.createSparklineHTML(guestId, 'netin');
            const netOutChart = PulseApp.charts.createSparklineHTML(guestId, 'netout');
            
            diskReadCell = `<div class="metric-text">${diskReadText}</div><div class="metric-chart">${diskReadChart}</div>`;
            diskWriteCell = `<div class="metric-text">${diskWriteText}</div><div class="metric-chart">${diskWriteChart}</div>`;
            netInCell = `<div class="metric-text">${netInText}</div><div class="metric-chart">${netInChart}</div>`;
            netOutCell = `<div class="metric-text">${netOutText}</div><div class="metric-chart">${netOutChart}</div>`;
        } else {
            // Fallback to text only for stopped guests - no arrows
            diskReadCell = diskReadFormatted;
            diskWriteCell = diskWriteFormatted;
            netInCell = netInFormatted;
            netOutCell = netOutFormatted;
        }

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
            <td class="p-1 px-2">${diskReadCell}</td>
            <td class="p-1 px-2">${diskWriteCell}</td>
            <td class="p-1 px-2">${netInCell}</td>
            <td class="p-1 px-2">${netOutCell}</td>
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

    function toggleChartsMode() {
        const mainContainer = document.getElementById('main');
        const button = document.getElementById('toggle-charts-button');
        
        if (mainContainer.classList.contains('charts-mode')) {
            // Switch to metrics mode
            mainContainer.classList.remove('charts-mode');
            button.title = 'Toggle Charts View';
        } else {
            // Switch to charts mode  
            mainContainer.classList.add('charts-mode');
            button.title = 'Toggle Metrics View';
            
            // Immediately render charts when switching to charts mode
            if (PulseApp.charts) {
                requestAnimationFrame(() => {
                    PulseApp.charts.updateAllCharts();
                });
            }
        }
    }

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow,
        snapshotGuestMetricsForDrag, // Export snapshot function
        clearGuestMetricSnapshots,    // Export clear function
        toggleChartsMode
    };
})();
