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

// Health Status Constants and Colors
const HEALTH_CRITICAL = 'critical';
const HEALTH_WARNING = 'warning';
const HEALTH_TRIGGERED = 'triggered';
const HEALTH_NORMAL = 'normal';

const healthStatusColors = {
    [HEALTH_CRITICAL]: 'bg-red-500',
    [HEALTH_WARNING]: 'bg-yellow-500',
    [HEALTH_TRIGGERED]: 'bg-orange-500',
    [HEALTH_NORMAL]: 'bg-green-500',
};
const healthDotHTML = (status) => `<span class="inline-block w-2 h-2 rounded-full mr-2 ${healthStatusColors[status] || 'bg-gray-300'}" title="Health: ${status.charAt(0).toUpperCase() + status.slice(1)}"></span>`;

// Helper function to get styled HTML for standard rate metrics
function getStandardRateValueHTML(guest, metricConfig, thresholdState) {
    const rawValue = guest[metricConfig.guestProperty];
    let formattedValue = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(rawValue, 0) : '-';

    if (guest.status === STATUS_RUNNING && metricConfig.isRate) {
        const metricThreshold = thresholdState[metricConfig.thresholdKey];
        if (metricThreshold && metricThreshold.value > 0 && rawValue !== null && rawValue !== undefined && rawValue >= metricThreshold.value) {
            return `<span class="text-orange-500 dark:text-orange-400">${formattedValue}</span>`;
        }
    }
    return formattedValue;
}

// +PulseDB Frontend: Metric configurations for graph view
const frontendMetricConfigs = [
    { apiMetricName: 'cpu_usage_percent', displayName: 'CPU', thresholdKey: 'cpu', guestProperty: 'cpu', isPercentage: true },
    { apiMetricName: 'memory_usage_percent', displayName: 'Memory', thresholdKey: 'memory', guestProperty: 'memory', isPercentage: true },
    { apiMetricName: 'disk_usage_percent', displayName: 'Disk', thresholdKey: 'disk', guestProperty: 'disk', isPercentage: true },
    { apiMetricName: 'disk_read_bytes_per_sec', displayName: 'Disk Read', thresholdKey: 'diskread', guestProperty: 'disk_read_rate', isRate: true },
    { apiMetricName: 'disk_write_bytes_per_sec', displayName: 'Disk Write', thresholdKey: 'diskwrite', guestProperty: 'disk_write_rate', isRate: true },
    { apiMetricName: 'net_in_bytes_per_sec', displayName: 'Net In', thresholdKey: 'netin', guestProperty: 'net_in_rate', isRate: true },
    { apiMetricName: 'net_out_bytes_per_sec', displayName: 'Net Out', thresholdKey: 'netout', guestProperty: 'net_out_rate', isRate: true }
];

PulseApp.ui.dashboard = (() => {
    let searchInput = null;
    let tableBodyEl = null;
    let statusElementEl = null;
    let graphViewActive = false; // Added for graph view toggle
    let graphViewToggleButton = null; // Added for graph view toggle

    const graphCellResizeObservers = new Map(); // Map to store ResizeObserver instances for graph cells

    // Helper to cleanup observer for a cell
    function cleanupResizeObserver(cellElement) {
        if (graphCellResizeObservers.has(cellElement)) {
            graphCellResizeObservers.get(cellElement).disconnect();
            graphCellResizeObservers.delete(cellElement);
        }
    }

    function init() {
        searchInput = document.getElementById('dashboard-search');
        tableBodyEl = document.querySelector('#main-table tbody');
        statusElementEl = document.getElementById('dashboard-status-text');
        graphViewToggleButton = document.getElementById('toggle-graph-view-button'); // Added for graph view toggle

        // Added for graph view toggle
        if (graphViewToggleButton) {
            graphViewToggleButton.addEventListener('click', () => {
                graphViewActive = !graphViewActive;
                // Add a class to the button when active for styling
                graphViewToggleButton.classList.toggle('bg-blue-100', graphViewActive);
                graphViewToggleButton.classList.toggle('dark:bg-blue-800/50', graphViewActive);
                updateDashboardTable(); 
            });
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
        let diskReadRate = null, diskWriteRate = null, netInRate = null, netOutRate = null;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';
        let effectiveMemorySource = 'host';
        let currentMemForAvg = 0;
        let currentMemTotalForDisplay = guest.maxmem;
        let finalMemTotalForPercent = guest.maxmem; // Declare and initialize here

        const metricsData = PulseApp.state.get('metricsData') || [];
        const metrics = metricsData.find(m =>
            m.id === guest.vmid &&
            m.type === guest.type &&
            m.node === guest.node &&
            m.endpointId === guest.endpointId
        );
        const guestUniqueId = guest.id;

        const isDragging = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();
        // const snapshot = guestMetricDragSnapshot[guestUniqueId]; // Snapshot logic will be removed for now

        // Removed the if (isDragging && snapshot) block. Calculations will always run.
        // History updates will be conditional on isDragging.

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
                timestamp: Date.now(), // Or metrics.current.timestamp if available and more accurate
                ...metrics.current,
                effective_mem: currentMemForAvg,
                effective_mem_total: currentMemTotalForDisplay,
                effective_mem_source: effectiveMemorySource
            };
            
            if (!isDragging) { // Only update history if not dragging
                PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
            }
            const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];

            avgCpu = _calculateAverage(history, 'cpu') ?? 0;
            avgMem = _calculateAverage(history, 'effective_mem') ?? 0;
            avgDisk = _calculateAverage(history, 'disk') ?? 0;

            // Prioritize pre-calculated rates if available from backend/state
            if (metrics.current.disk_read_bytes_per_sec !== undefined) {
                diskReadRate = metrics.current.disk_read_bytes_per_sec;
            } else if (metrics.current.diskread !== undefined) { // Fallback to calculating from cumulative
                diskReadRate = _calculateAverageRate(history, 'diskread');
            }

            if (metrics.current.disk_write_bytes_per_sec !== undefined) {
                diskWriteRate = metrics.current.disk_write_bytes_per_sec;
            } else if (metrics.current.diskwrite !== undefined) { // Fallback
                diskWriteRate = _calculateAverageRate(history, 'diskwrite');
            }

            if (metrics.current.net_in_bytes_per_sec !== undefined) {
                netInRate = metrics.current.net_in_bytes_per_sec;
            } else if (metrics.current.netin !== undefined) { // Fallback
                netInRate = _calculateAverageRate(history, 'netin');
            }

            if (metrics.current.net_out_bytes_per_sec !== undefined) {
                netOutRate = metrics.current.net_out_bytes_per_sec;
            } else if (metrics.current.netout !== undefined) { // Fallback
                netOutRate = _calculateAverageRate(history, 'netout');
            }
        } else {
            if (!isDragging) { // Only clear history if not dragging
                PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }
        }

        const historyForGuest = PulseApp.state.getDashboardHistory()[guestUniqueId]; // Re-fetch history in case it was cleared

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

        // Determine Health Status
        let healthStatus = HEALTH_NORMAL;
        if (guest.status === STATUS_RUNNING) {
            const cpuColor = PulseApp.utils.getUsageColor(avgCpu * 100); // avgCpu is 0-1 range
            if (cpuColor === 'red') healthStatus = HEALTH_CRITICAL;
            else if (cpuColor === 'yellow' && healthStatus !== HEALTH_CRITICAL) healthStatus = HEALTH_WARNING;

            if (avgMemoryPercent !== 'N/A') {
                const memColor = PulseApp.utils.getUsageColor(avgMemoryPercent);
                if (memColor === 'red') healthStatus = HEALTH_CRITICAL;
                else if (memColor === 'yellow' && healthStatus !== HEALTH_CRITICAL) healthStatus = HEALTH_WARNING;
            }

            if (guest.type === GUEST_TYPE_CT && avgDiskPercent !== 'N/A') {
                const diskColor = PulseApp.utils.getUsageColor(avgDiskPercent);
                if (diskColor === 'red') healthStatus = HEALTH_CRITICAL;
                else if (diskColor === 'yellow' && healthStatus !== HEALTH_CRITICAL) healthStatus = HEALTH_WARNING;
            }

            // Check rate-based thresholds only if not already critical or warning
            if (healthStatus !== HEALTH_CRITICAL && healthStatus !== HEALTH_WARNING) {
                const thresholdState = PulseApp.state.getThresholdState();
                for (const config of frontendMetricConfigs) {
                    if (config.isRate) {
                        const metricThreshold = thresholdState[config.thresholdKey];
                        if (metricThreshold && metricThreshold.value > 0) {
                            const guestValue = guest[config.guestProperty]; // This is already set on guest object
                            if (guestValue !== null && guestValue !== undefined && guestValue >= metricThreshold.value) {
                                healthStatus = HEALTH_TRIGGERED;
                                break; // One triggered threshold is enough
                            }
                        }
                    }
                }
            }
        } else {
            healthStatus = 'stopped'; // Or some other neutral status for stopped guests
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
            disk_read_rate: diskReadRate,
            disk_write_rate: diskWriteRate,
            net_in_rate: netInRate,
            net_out_rate: netOutRate,
            healthStatus: healthStatus,
            // Store values needed for snapshot stability if they were calculated and not direct guest props
            memory_percent: avgMemoryPercent, 
            disk_percent: avgDiskPercent,
            memory_total: finalMemTotalForPercent // Store the memory total used for percentage
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
                else if (type === METRIC_DISK_READ) guestValue = guest.disk_read_rate;
                else if (type === METRIC_DISK_WRITE) guestValue = guest.disk_write_rate;
                else if (type === METRIC_NET_IN) guestValue = guest.net_in_rate;
                else if (type === METRIC_NET_OUT) guestValue = guest.net_out_rate;
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
        // Ensure secondary sort by name if primary sort is not name, for stable ordering
        const sortedData = PulseApp.utils.sortData(filteredData, sortStateMain.column, sortStateMain.direction, 'main', guest => guest.name.toLowerCase());

        let visibleCount = 0;
        const visibleNodes = new Set();
        
        // Map existing DOM rows by uniqueId for efficient updates/removals
        const existingDomRows = new Map();
        tableBodyEl.querySelectorAll('tr[data-unique-id]').forEach(row => {
            existingDomRows.set(row.dataset.uniqueId, row);
        });

        // Map existing node header rows by node name
        const existingNodeHeaders = new Map();
        if (groupByNode) {
            tableBodyEl.querySelectorAll('tr.node-header[data-node-name]').forEach(row => {
                existingNodeHeaders.set(row.dataset.nodeName, row);
            });
        }

        // BEFORE clearing the table, disconnect all existing observers to prevent leaks
        tableBodyEl.querySelectorAll('td').forEach(cell => {
            cleanupResizeObserver(cell);
        });

        const rowsToKeepOrAdd = new Set();
        const nodeHeadersToKeepOrAdd = new Set();
        const fragment = document.createDocumentFragment(); // To build the new table body contents

        let scrollableElement = null;
        let scrollTop = 0;
        let scrollLeft = 0;
        let originalOverflowY = ''; // To store original overflow-y style

        // Check tableBodyEl itself first (less common for tbody to be the scroller, but good to check)
        // const tbodyStyle = window.getComputedStyle(tableBodyEl);
        // if (tbodyStyle.overflowY === 'auto' || tbodyStyle.overflowY === 'scroll') {
        //     scrollableElement = tableBodyEl;
        // }
        // Based on HTML, tableBodyEl (tbody) is not the scroller. Start with parent.

        // Traverse upwards to find a scrollable ancestor for vertical scroll
        let verticalScroller = tableBodyEl.parentElement; // Starts at <table>
        while (verticalScroller && verticalScroller.tagName !== 'BODY' && verticalScroller.tagName !== 'HTML') {
            const style = window.getComputedStyle(verticalScroller);
            if (style.overflowY === 'auto' || style.overflowY === 'scroll') {
                scrollableElement = verticalScroller; // This should find #dashboard-content
                break;
            }
            verticalScroller = verticalScroller.parentElement;
        }

        if (scrollableElement) {
            scrollTop = scrollableElement.scrollTop;
            const horizontalScroller = document.getElementById('main-table-container');
            scrollLeft = horizontalScroller ? horizontalScroller.scrollLeft : (scrollableElement.scrollLeft || 0);
            originalOverflowY = scrollableElement.style.overflowY; // Save original overflowY
            scrollableElement.style.overflowY = 'hidden'; // Temporarily hide scrollbar
        } else {
            // If no specific element, use the document's scrolling element
            scrollableElement = document.scrollingElement || document.documentElement;
            scrollTop = scrollableElement.scrollTop;
            scrollLeft = window.pageXOffset !== undefined ? window.pageXOffset : scrollableElement.scrollLeft;
            // For documentElement/body, overflow changes can be more complex/risky, typically handled by browser.
            // We might not want to change overflowY for the whole document.
            // However, our scrollableElement logic above should find #dashboard-content before this else.
        }
        
        let originalMinHeight = '';
        let scrollerHadMinHeightAtt = false;
        if (scrollableElement && scrollableElement !== (document.scrollingElement || document.documentElement)) {
            scrollerHadMinHeightAtt = scrollableElement.hasAttribute('style') && scrollableElement.style.minHeight !== '';
            originalMinHeight = scrollableElement.style.minHeight;
            scrollableElement.style.minHeight = `${scrollableElement.scrollHeight}px`;
        }

        if (groupByNode) {
            const nodeGroups = {};
            sortedData.forEach(guest => {
                const nodeName = guest.node || 'Unknown Node';
                if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
                nodeGroups[nodeName].push(guest);
            });

            Object.keys(nodeGroups).sort().forEach(nodeName => {
                visibleNodes.add(nodeName.toLowerCase());
                let nodeHeaderRow = existingNodeHeaders.get(nodeName);
                if (!nodeHeaderRow) {
                    nodeHeaderRow = document.createElement('tr');
                    nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
                    nodeHeaderRow.dataset.nodeName = nodeName; // For tracking
                    // Determine colspan based on active view (graph view has more columns)
                    const colspan = graphViewActive ? (4 + frontendMetricConfigs.length) : 11;
                    nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, colspan, 'td');
                }
                fragment.appendChild(nodeHeaderRow);
                nodeHeadersToKeepOrAdd.add(nodeName);

                nodeGroups[nodeName].forEach(guest => {
                    let guestRow = existingDomRows.get(guest.uniqueId);
                    if (guestRow) {
                        updateGuestRow(guestRow, guest); // Update existing row in place
                    } else {
                        guestRow = createGuestRow(guest); // Create new row
                    }
                    fragment.appendChild(guestRow);
                    rowsToKeepOrAdd.add(guest.uniqueId);
                    visibleCount++;
                });
            });
        } else {
            sortedData.forEach(guest => {
                visibleNodes.add((guest.node || 'Unknown Node').toLowerCase());
                let guestRow = existingDomRows.get(guest.uniqueId);
                if (guestRow) {
                    updateGuestRow(guestRow, guest);
                } else {
                    guestRow = createGuestRow(guest);
                }
                fragment.appendChild(guestRow);
                rowsToKeepOrAdd.add(guest.uniqueId);
                visibleCount++;
            });
        }

        // Remove old rows that are no longer needed
        existingDomRows.forEach((row, uniqueId) => {
            if (!rowsToKeepOrAdd.has(uniqueId)) {
                row.remove();
            }
        });

        if (groupByNode) {
            existingNodeHeaders.forEach((row, nodeName) => {
                if (!nodeHeadersToKeepOrAdd.has(nodeName)) {
                    row.remove();
                }
            });
        }
        
        tableBodyEl.innerHTML = '';
        tableBodyEl.appendChild(fragment);

        requestAnimationFrame(() => {
            if (scrollableElement && scrollableElement !== (document.scrollingElement || document.documentElement)) {
                // Restore original minHeight carefully
                if (scrollerHadMinHeightAtt) {
                    scrollableElement.style.minHeight = originalMinHeight;
                } else {
                    scrollableElement.style.removeProperty('min-height');
                }
                scrollableElement.style.overflowY = originalOverflowY; // Restore original overflowY
            }

            if (scrollableElement === (document.scrollingElement || document.documentElement)) {
                 window.scrollTo(scrollLeft, scrollTop);
            } else if (scrollableElement) {
                scrollableElement.scrollTop = scrollTop;
                // Restore horizontal scroll to #main-table-container if it exists, or to the vertical scroller
                const horizontalScroller = document.getElementById('main-table-container');
                if (horizontalScroller) {
                    horizontalScroller.scrollLeft = scrollLeft;
                } else {
                    scrollableElement.scrollLeft = scrollLeft;
                }
            }
        });

        if (visibleCount === 0 && tableBodyEl.children.length === 0) { // Check if fragment was empty
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
            const colspan = graphViewActive ? (4 + frontendMetricConfigs.length) : 11;
            tableBodyEl.innerHTML = `<tr><td colspan="${colspan}" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
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

    // Updated createMiniGraphSVG to handle empty values array better for initial render
    // and to optionally update an existing SVG instance.
    const createMiniGraphSVG = (values = [], width = 100, height = 20, viewBoxWidth = 100, viewBoxHeight = 20, existingSvgInstance = null, metricNameForTooltip = '', appearanceDetails = null) => {
        const svgNS = "http://www.w3.org/2000/svg";
        let svg;
        let activePointIndicator;
        let interactionOverlay;
        let yMaxLabel, yMinLabel;

        let oldLineElement = null;
        let oldSingleDotElement = null;
        let oldAreaElement = null; // Added for area fill

        const yLabelAreaWidth = 35; 
        const graphRightPadding = 1;  
        const labelFontSize = 8;
        // const labelTextYOffset = labelFontSize / 2 -1; // Not used

        // Color mapping for sparklines based on getUsageColor result
        const sparklineColorMapping = {
            red: { stroke: 'rgba(239, 68, 68, 1)', fillOpacity: '0.3', strokeWidth: '1.5' },     // Tailwind red-500
            yellow: { stroke: 'rgba(245, 158, 11, 1)', fillOpacity: '0.25', strokeWidth: '1.5' }, // Tailwind yellow-500
            green: { stroke: 'currentColor', fillOpacity: '0.1', strokeWidth: '1' }         // Default/current color
        };
        const defaultSparklineLook = { stroke: 'currentColor', fillOpacity: '0.1', strokeWidth: '1' };
        const triggeredThresholdSparklineLook = { stroke: 'rgba(251, 146, 60, 1)', fillOpacity: '0.3', strokeWidth: '1.5' }; // Orange-500 for triggered rate thresholds

        // Determine actual look based on appearanceDetails or default logic
        let actualSparklineLook = defaultSparklineLook;
        let activeThresholdInfo = null; // For tooltip enhancement

        if (appearanceDetails) {
            if (appearanceDetails.look) {
                actualSparklineLook = appearanceDetails.look;
            }
            if (appearanceDetails.thresholdInfo) {
                activeThresholdInfo = appearanceDetails.thresholdInfo;
            }
        } else if (metricNameForTooltip.includes('_percent') || metricNameForTooltip.includes('cpu')) {
            // Ensure dataPointsToRender is available here. It's calculated a bit lower.
            // This logic needs to be after dataPointsToRender is finalized.
            // For now, this part will be handled after dataPointsToRender is defined.
        }

        // Calculate scales and data-dependent parameters based on current values
        const dataPoints = values.map(d => ({ timestamp: d.timestamp * 1000, value: d.value }));

        let minTimestamp, maxTimestamp;
        if (dataPoints.length > 1) {
            minTimestamp = Math.min(...dataPoints.map(d => d.timestamp));
            maxTimestamp = Math.max(...dataPoints.map(d => d.timestamp));
        } else { 
            minTimestamp = (dataPoints[0]?.timestamp || Date.now()) - 30000;
            maxTimestamp = (dataPoints[0]?.timestamp || Date.now()) + 30000;
        }
        if (dataPoints.length > 1 && (maxTimestamp - minTimestamp) < 60000) {
            const center = (minTimestamp + maxTimestamp) / 2;
            minTimestamp = center - 30000;
            maxTimestamp = center + 30000;
        }

        const dataDurationSeconds = (maxTimestamp - minTimestamp) / 1000;
        const effectiveViewBoxStartTime = maxTimestamp - (Math.min(dataDurationSeconds, 3600) * 1000);

        // Downsampling logic
        let dataPointsToRender = dataPoints;
        const bucketSeconds = PulseApp.config.GRAPH_BUCKET_SECONDS;

        if (bucketSeconds && bucketSeconds > 0 && dataPoints.length > (dataDurationSeconds / bucketSeconds) * 0.8) { // Apply if bucketing makes sense
            const bucketMs = bucketSeconds * 1000;
            const downsampledMap = new Map(); // Use a map to store one point per bucket key

            // Determine the overall start and end for bucketing from the data itself
            // Optimized min/max calculation
            let actualMinDataTimestamp = Infinity;
            let actualMaxDataTimestamp = -Infinity;
            if (dataPoints.length > 0) {
                for (const p of dataPoints) {
                    if (p.timestamp < actualMinDataTimestamp) actualMinDataTimestamp = p.timestamp;
                    if (p.timestamp > actualMaxDataTimestamp) actualMaxDataTimestamp = p.timestamp;
                }
            } else {
                // Fallback if dataPoints is empty, though this case is less likely here
                // as dataPoints comes from API data. Ensure these align with how they were used before.
                actualMinDataTimestamp = effectiveViewBoxStartTime; 
                actualMaxDataTimestamp = maxTimestamp; 
            }

            for (const point of dataPoints) {
                // Ensure we only process points within the effective display window for bucketing
                if (point.timestamp < effectiveViewBoxStartTime || point.timestamp > maxTimestamp) { // maxTimestamp is the true end of our data window
                    continue; 
                }
                // Align bucket key to absolute time intervals (e.g., start of minute, 10s interval, etc.)
                const bucketKey = Math.floor(point.timestamp / bucketMs) * bucketMs;
                
                const existingPointInBucket = downsampledMap.get(bucketKey);
                if (!existingPointInBucket || point.value > existingPointInBucket.value) {
                    // If no point for this bucket yet, or if current point has a higher value (peak)
                    downsampledMap.set(bucketKey, point);
                }
            }
            
            let downsampled = Array.from(downsampledMap.values());

            // Ensure the very last data point from the original set is included if it's more recent
            const lastOriginalPoint = dataPoints.length > 0 ? dataPoints[dataPoints.length - 1] : null;
            if (lastOriginalPoint) {
                const lastBucketKey = Math.floor(lastOriginalPoint.timestamp / bucketMs) * bucketMs;
                const lastPointInDownsampled = downsampledMap.get(lastBucketKey);
                // If last original point isn't the one chosen for its bucket (e.g. not the peak), or if it's in a newer bucket not yet fully processed.
                // Or simply, if the absolute last point isn't in the downsampled set, add it.
                if (!downsampled.find(p => p.timestamp === lastOriginalPoint.timestamp)) {
                    // To avoid duplicate timestamps if the last point was already a peak, remove any existing point with the same timestamp first
                    downsampled = downsampled.filter(p => p.timestamp !== lastOriginalPoint.timestamp);
                    downsampled.push(lastOriginalPoint);
                }
            }

            if (downsampled.length > 0) {
                dataPointsToRender = downsampled.sort((a, b) => a.timestamp - b.timestamp);
            } else if (dataPoints.length > 0) {
                // Fallback if downsampling resulted in nothing (e.g. very sparse data over a short period)
                dataPointsToRender = [dataPoints[dataPoints.length-1]];
            }
        } 
        // Fallback for no dataPoints initially, or if downsampling cleared everything (should be rare with above fallback)
        if (dataPointsToRender.length === 0 && dataPoints.length > 0) {
             dataPointsToRender = [dataPoints[dataPoints.length-1]];
        } else if (dataPointsToRender.length === 0 && dataPoints.length === 0) {
             // Truly no data, will be handled by the existing array length check for drawing
        }

        // Moved appearance logic here, after dataPointsToRender is finalized
        if (!appearanceDetails && (metricNameForTooltip.includes('_percent') || metricNameForTooltip.includes('cpu'))) {
            if (dataPointsToRender.length > 0) {
                const latestValue = dataPointsToRender[dataPointsToRender.length - 1].value;
                const colorKey = PulseApp.utils.getUsageColor(latestValue);
                actualSparklineLook = sparklineColorMapping[colorKey] || defaultSparklineLook;
            }
        }

        const yDataMin = 0; 
        // Scale Y-axis based on the actual points to be rendered
        const yDataMax = dataPointsToRender.length > 0 ? Math.max(...dataPointsToRender.map(d => d.value), 0) : 0;
        const yVisualPadding = yDataMax > 0 ? yDataMax * 0.1 : (viewBoxHeight > 1 ? 1 : 0);
        
        const graphPlotAreaXStart = yLabelAreaWidth;
        const graphPlotAreaWidth = viewBoxWidth - yLabelAreaWidth - graphRightPadding;

        const xScale = (timestamp) => {
            const timeRange = maxTimestamp - effectiveViewBoxStartTime;
            if (timeRange === 0) return graphPlotAreaXStart + graphPlotAreaWidth / 2; 
            return graphPlotAreaXStart + (((timestamp - effectiveViewBoxStartTime) / timeRange) * graphPlotAreaWidth);
        };
        const yScale = (value) => {
            if (yDataMax === yDataMin) return viewBoxHeight / 2; 
            const yRange = (yDataMax + yVisualPadding) - yDataMin;
            if (yRange === 0) return viewBoxHeight / 2;
            return viewBoxHeight - ((value - yDataMin) / yRange) * viewBoxHeight;
        };

        // Define event handlers within this scope to capture current scales/data
        const mouseEnterHandler = () => { /* Optional: specific mouse enter logic */ };
        
        const mouseMoveHandler = (event) => {
            const tooltipElement = document.getElementById('graph-tooltip'); // Get fresh reference
            if (!tooltipElement || dataPointsToRender.length === 0 || graphPlotAreaWidth <=0) { // Use dataPointsToRender
                if(activePointIndicator) activePointIndicator.style.display = 'none';
                if(tooltipElement) tooltipElement.style.display = 'none';
                return;
            }

            const svgRect = svg.getBoundingClientRect();
            let mouseXInViewBoxCoords = 0;
            if (svgRect.width > 0) {
                mouseXInViewBoxCoords = ((event.clientX - svgRect.left) / svgRect.width) * viewBoxWidth;
            }
            
            let mouseXInPlotArea_viewBox = mouseXInViewBoxCoords - graphPlotAreaXStart;
            mouseXInPlotArea_viewBox = Math.max(0, Math.min(mouseXInPlotArea_viewBox, graphPlotAreaWidth));

            const timeRange = maxTimestamp - effectiveViewBoxStartTime;
            let closestPoint;

            if (dataPointsToRender.length === 1) { // Use dataPointsToRender
                closestPoint = dataPointsToRender[0];
            } else if (timeRange === 0) { 
                closestPoint = dataPointsToRender[dataPointsToRender.length -1] || dataPointsToRender[0]; // Use dataPointsToRender
            } else {
                const hoverTimestamp = (mouseXInPlotArea_viewBox / graphPlotAreaWidth) * timeRange + effectiveViewBoxStartTime;
                closestPoint = dataPointsToRender.reduce((prev, curr) => { // Use dataPointsToRender
                    return (Math.abs(curr.timestamp - hoverTimestamp) < Math.abs(prev.timestamp - hoverTimestamp) ? curr : prev);
                });
            }
             if (!closestPoint) { 
                if(activePointIndicator) activePointIndicator.style.display = 'none';
                if(tooltipElement) tooltipElement.style.display = 'none';
                return;
            }

            if (!activePointIndicator) { // Ensure activePointIndicator is defined if it was missing
                 activePointIndicator = svg.querySelector('.active-point-indicator');
                 if (!activePointIndicator) { // Still missing, create it (should ideally be created once)
                    activePointIndicator = document.createElementNS(svgNS, "circle");
                    activePointIndicator.classList.add('active-point-indicator');
                    activePointIndicator.setAttribute("r", "2.5");
                    activePointIndicator.setAttribute("fill", "blue");
                    activePointIndicator.setAttribute("stroke", "white");
                    activePointIndicator.setAttribute("stroke-width", "1");
                    activePointIndicator.style.pointerEvents = "none";
                    svg.appendChild(activePointIndicator); // Append if newly created
                 }
            }
            
            activePointIndicator.setAttribute("cx", xScale(closestPoint.timestamp).toFixed(2));
            activePointIndicator.setAttribute("cy", yScale(closestPoint.value).toFixed(2));
            activePointIndicator.style.display = "block";

            let formattedValue = PulseApp.utils.formatSpeed(closestPoint.value, 2);
            if (metricNameForTooltip.includes('_percent') || metricNameForTooltip.includes('cpu')) {
                formattedValue = `${closestPoint.value.toFixed(1)}%`;
            } else if (metricNameForTooltip.includes('memory')) {
                formattedValue = PulseApp.utils.formatBytes(closestPoint.value, 2);
            }

            const timeAgo = PulseApp.utils.formatTimeAgo(closestPoint.timestamp);
            tooltipElement.textContent = `${formattedValue} (${timeAgo})`;

            if (activeThresholdInfo && activeThresholdInfo.key && activeThresholdInfo.value) {
                const readableName = PulseApp.utils.getReadableThresholdName(activeThresholdInfo.key);
                const formattedThresholdValue = PulseApp.utils.formatThresholdValue(activeThresholdInfo.key, activeThresholdInfo.value);
                tooltipElement.textContent += ` (Threshold: ${readableName} >= ${formattedThresholdValue})`;
            }

            tooltipElement.style.left = `${event.pageX + 10}px`;
            tooltipElement.style.top = `${event.pageY + 10}px`;
            tooltipElement.style.display = 'block';
        };

        const mouseLeaveHandler = () => {
            const tooltipElement = document.getElementById('graph-tooltip'); // Get fresh reference
            if (tooltipElement) tooltipElement.style.display = 'none';
            if (activePointIndicator) activePointIndicator.style.display = "none";
        };


        if (existingSvgInstance) {
            svg = existingSvgInstance;
            oldLineElement = svg.querySelector('.graph-line');
            oldSingleDotElement = svg.querySelector('.single-point-dot');
            oldAreaElement = svg.querySelector('.graph-area'); // Added for area fill
            activePointIndicator = svg.querySelector('.active-point-indicator');
            interactionOverlay = svg.querySelector('.interaction-overlay');
            yMaxLabel = svg.querySelector('.y-max-label');
            yMinLabel = svg.querySelector('.y-min-label');

            svg.setAttribute("width", "100%");
            svg.setAttribute("height", height.toString());
            svg.setAttribute("viewBox", `0 0 ${viewBoxWidth} ${viewBoxHeight}`);
        } else {
            svg = document.createElementNS(svgNS, "svg");
            svg.setAttribute("width", "100%");
            svg.setAttribute("height", height.toString());
            svg.setAttribute("viewBox", `0 0 ${viewBoxWidth} ${viewBoxHeight}`);
            svg.setAttribute("preserveAspectRatio", "xMinYMid meet");
            svg.classList.add("mini-graph-svg");
        }

        if (!Array.isArray(values) || dataPointsToRender.length === 0 || graphPlotAreaWidth <=0 ) { // Check dataPointsToRender
            oldLineElement?.remove();
            oldSingleDotElement?.remove();
            oldAreaElement?.remove(); // Added for area fill
            if(activePointIndicator) activePointIndicator.style.display = 'none';
            if(yMaxLabel) yMaxLabel.textContent = ''; // Clear Y-axis max label
            if(yMinLabel) yMinLabel.textContent = ''; // Clear Y-axis min label
            // If there's an interaction overlay, clear its listeners if we are clearing the graph
            if (interactionOverlay && interactionOverlay._mouseMoveHandler) {
                interactionOverlay.removeEventListener('mousemove', interactionOverlay._mouseMoveHandler);
                interactionOverlay.removeEventListener('mouseleave', interactionOverlay._mouseLeaveHandler);
                interactionOverlay.removeEventListener('mouseenter', interactionOverlay._mouseEnterHandler);
                interactionOverlay._mouseMoveHandler = null; // Clear stored handlers
                interactionOverlay._mouseLeaveHandler = null;
                interactionOverlay._mouseEnterHandler = null;
            }
            return svg; 
        }
        
        // ... (y-axis label creation/update logic remains here, using current scales)
        const formatYValue = (value, metricName) => {
            let formatted = '0';
            if (metricName.includes('_percent') || metricName.includes('cpu')) {
                formatted = `${value.toFixed(0)}%`;
            } else if (metricName.includes('memory')) {
                formatted = PulseApp.utils.formatBytes(value, 0, true);
            } else { 
                formatted = PulseApp.utils.formatSpeed(value, 0, true);
            }
            return formatted;
        };

        if (!yMaxLabel) {
            yMaxLabel = document.createElementNS(svgNS, "text");
            yMaxLabel.classList.add('y-max-label');
            yMaxLabel.setAttribute("x", "1"); 
            yMaxLabel.setAttribute("y", (labelFontSize).toString());
            yMaxLabel.setAttribute("font-size", `${labelFontSize}px`);
            yMaxLabel.setAttribute("fill", "currentColor");
            yMaxLabel.setAttribute("text-anchor", "start"); 
            yMaxLabel.style.opacity = "0.7";
            svg.appendChild(yMaxLabel);
        }
        yMaxLabel.textContent = formatYValue(yDataMax, metricNameForTooltip);

        if (!yMinLabel) {
            yMinLabel = document.createElementNS(svgNS, "text");
            yMinLabel.classList.add('y-min-label');
            yMinLabel.setAttribute("x", "1"); 
            yMinLabel.setAttribute("y", (viewBoxHeight - 2).toString());
            yMinLabel.setAttribute("font-size", `${labelFontSize}px`);
            yMinLabel.setAttribute("fill", "currentColor");
            yMinLabel.setAttribute("text-anchor", "start"); 
            yMinLabel.style.opacity = "0.7";
            svg.appendChild(yMinLabel);
        }
        yMinLabel.textContent = formatYValue(yDataMin, metricNameForTooltip);
        
        // --- Draw or Update Line/Dot --- (using current scales and dataPointsToRender)
        if (dataPointsToRender.length === 1) { // Use dataPointsToRender
            const point = dataPointsToRender[0];
            const newCx = xScale(point.timestamp).toFixed(2);
            const newCy = yScale(point.value).toFixed(2);
            const newR = "1.5";

            if (oldSingleDotElement && 
                oldSingleDotElement.getAttribute('cx') === newCx && 
                oldSingleDotElement.getAttribute('cy') === newCy &&
                oldSingleDotElement.getAttribute('r') === newR) {
                // Dot is the same
            } else {
                oldSingleDotElement?.remove();
                const circle = document.createElementNS(svgNS, "circle");
                circle.classList.add('single-point-dot');
                circle.setAttribute("cx", newCx);
                circle.setAttribute("cy", newCy);
                circle.setAttribute("r", newR); 
                circle.setAttribute("fill", "currentColor");
                svg.appendChild(circle);
            }
            oldLineElement?.remove(); 
            oldAreaElement?.remove(); // Remove area if switching to single dot
        } else { 
            const linePathData = "M" + dataPointsToRender.map(p => `${xScale(p.timestamp).toFixed(2)},${yScale(p.value).toFixed(2)}`).join(" L ");

            // Area path
            const areaPathData = linePathData + 
                               ` L ${xScale(dataPointsToRender[dataPointsToRender.length - 1].timestamp).toFixed(2)},${yScale(yDataMin).toFixed(2)}` +
                               ` L ${xScale(dataPointsToRender[0].timestamp).toFixed(2)},${yScale(yDataMin).toFixed(2)} Z`;

            if (oldAreaElement && oldAreaElement.getAttribute('d') === areaPathData && oldAreaElement.getAttribute('fill') === actualSparklineLook.stroke && oldAreaElement.getAttribute('fill-opacity') === actualSparklineLook.fillOpacity) {
                // Area is the same
            } else {
                oldAreaElement?.remove();
                const areaPath = document.createElementNS(svgNS, "path");
                areaPath.classList.add('graph-area');
                areaPath.setAttribute("d", areaPathData);
                areaPath.setAttribute("fill", actualSparklineLook.stroke);
                areaPath.setAttribute("fill-opacity", actualSparklineLook.fillOpacity); 
                areaPath.style.pointerEvents = "none"; 
                svg.appendChild(areaPath); 
            }

            if (oldLineElement && oldLineElement.getAttribute('d') === linePathData && oldLineElement.getAttribute('stroke') === actualSparklineLook.stroke && oldLineElement.getAttribute('stroke-width') === actualSparklineLook.strokeWidth) {
                // Line is the same
            } else {
                oldLineElement?.remove();
                const line = document.createElementNS(svgNS, "path");
                line.classList.add('graph-line');
                line.setAttribute("d", linePathData);
                line.setAttribute("stroke", actualSparklineLook.stroke);
                line.setAttribute("stroke-width", actualSparklineLook.strokeWidth);
                line.setAttribute("fill", "none");
                line.setAttribute("vector-effect", "non-scaling-stroke");
                svg.appendChild(line); 
            }
            oldSingleDotElement?.remove(); 
        }

        // Ensure activePointIndicator exists (it's created on first need within mouseMoveHandler if completely new)
        if (!activePointIndicator && svg.querySelector('.active-point-indicator')) { // Check if it was created by another instance before overlay
            activePointIndicator = svg.querySelector('.active-point-indicator');
        } else if (!activePointIndicator) { // Still not there, create it hidden
            activePointIndicator = document.createElementNS(svgNS, "circle");
            activePointIndicator.classList.add('active-point-indicator');
            activePointIndicator.setAttribute("r", "2.5");
            activePointIndicator.setAttribute("fill", "blue");
            activePointIndicator.setAttribute("stroke", "white");
            activePointIndicator.setAttribute("stroke-width", "1");
            activePointIndicator.style.display = "none";
            activePointIndicator.style.pointerEvents = "none";
            svg.appendChild(activePointIndicator);
        }


        // Manage interaction overlay and its listeners
        if (!interactionOverlay) {
            interactionOverlay = document.createElementNS(svgNS, "rect");
            interactionOverlay.classList.add('interaction-overlay');
            interactionOverlay.setAttribute("fill", "transparent");
            interactionOverlay.style.cursor = "crosshair";
            svg.appendChild(interactionOverlay); 
        }
        
        // Always set/update dimensions
        interactionOverlay.setAttribute("width", viewBoxWidth.toString()); 
        interactionOverlay.setAttribute("height", viewBoxHeight.toString());

        // Remove previously stored listeners, if any
        if (interactionOverlay._mouseMoveHandler) {
            interactionOverlay.removeEventListener('mousemove', interactionOverlay._mouseMoveHandler);
        }
        if (interactionOverlay._mouseLeaveHandler) {
            interactionOverlay.removeEventListener('mouseleave', interactionOverlay._mouseLeaveHandler);
        }
        if (interactionOverlay._mouseEnterHandler) {
            interactionOverlay.removeEventListener('mouseenter', interactionOverlay._mouseEnterHandler);
        }

        // Add the new handlers (which close over the current scales and data)
        interactionOverlay.addEventListener('mousemove', mouseMoveHandler);
        interactionOverlay.addEventListener('mouseleave', mouseLeaveHandler);
        interactionOverlay.addEventListener('mouseenter', mouseEnterHandler);

        // Store references to the new handlers for future removal
        interactionOverlay._mouseMoveHandler = mouseMoveHandler;
        interactionOverlay._mouseLeaveHandler = mouseLeaveHandler;
        interactionOverlay._mouseEnterHandler = mouseEnterHandler;
        
        return svg;
    };

    // +PulseDB Frontend: New function to fetch data and render a single graph
    async function fetchAndRenderMetricGraph(guestUniqueId, metricName, targetCellElement, guestStatus, appearanceDetails = null, isThresholdSliderDrag = false) {
        PulseApp.state.graphDataCache = PulseApp.state.graphDataCache || {};
        const cacheKey = `${guestUniqueId}-${metricName}`;

        if (!guestUniqueId || !metricName || !targetCellElement) {
            console.warn('[Pulse UI] fetchAndRenderMetricGraph called with missing parameters.');
            if (targetCellElement) targetCellElement.textContent = 'Params Missing';
            return;
        }

        const graphHeight = 20;
        const placeholderDivHTML = '<div class="h-[22px] w-full border border-gray-300 dark:border-gray-600 rounded my-1 flex items-center justify-center text-xs text-gray-400">-</div>';

        const setupObserverAndRender = (dataForGraph) => {
            requestAnimationFrame(() => {
                const currentCellWidth = targetCellElement.clientWidth; // Read true width, no fallback here
                let svgElement = targetCellElement.querySelector('svg.mini-graph-svg');

                if (currentCellWidth === 0) {
                    // Width is 0, hide SVG or show placeholder. ResizeObserver will do the first render.
                    if (svgElement) {
                        svgElement.style.visibility = 'hidden';
                    } else if (targetCellElement.innerHTML !== placeholderDivHTML) {
                        cleanupResizeObserver(targetCellElement); // Clean before changing innerHTML
                        targetCellElement.innerHTML = placeholderDivHTML;
                    }
                } else {
                    // Width is > 0, render now.
                    // Fallback to 70px only if currentCellWidth was >0 but somehow falsy (unlikely for clientWidth)
                    const viewBoxWidth = currentCellWidth || 70; 
                    if (!svgElement || targetCellElement.firstChild?.tagName !== 'SVG') {
                        cleanupResizeObserver(targetCellElement); // Clean before changing innerHTML
                        svgElement = createMiniGraphSVG([], viewBoxWidth, graphHeight, viewBoxWidth, graphHeight, null, metricName, appearanceDetails);
                        targetCellElement.innerHTML = '';
                        targetCellElement.appendChild(svgElement);
                    }
                    createMiniGraphSVG(dataForGraph, viewBoxWidth, graphHeight, viewBoxWidth, graphHeight, svgElement, metricName, appearanceDetails);
                    if (dataForGraph && dataForGraph.length > 0) {
                        svgElement.style.visibility = 'visible';
                    } else {
                        svgElement.style.visibility = 'hidden';
                    }
                }

                // Ensure ResizeObserver is active
                if (window.ResizeObserver) {
                    cleanupResizeObserver(targetCellElement); // Remove old one first
                    const observer = new ResizeObserver(entries => {
                        for (let entry of entries) {
                            requestAnimationFrame(() => { // Use rAF inside observer callback
                                const newWidth = entry.target.clientWidth;
                                let liveSvgElement = entry.target.querySelector('svg.mini-graph-svg');
                                const liveData = PulseApp.state.graphDataCache[cacheKey]; // Get current data

                                if (newWidth === 0) {
                                    if (liveSvgElement) liveSvgElement.style.visibility = 'hidden';
                                    // Optionally, if cell is empty and svg was hidden, restore placeholder explicitly
                                    // if (!entry.target.firstChild || entry.target.firstChild.tagName !== 'SVG') {
                                    //    if(entry.target.innerHTML !== placeholderDivHTML) entry.target.innerHTML = placeholderDivHTML;
                                    // }
                                    return; // Don't render if width is 0
                                }
                                
                                // Fallback to 70px if newWidth was >0 but somehow falsy
                                const newViewBoxWidth = newWidth || 70;

                                if (!liveSvgElement || entry.target.firstChild?.tagName !== 'SVG') {
                                    liveSvgElement = createMiniGraphSVG([], newViewBoxWidth, graphHeight, newViewBoxWidth, graphHeight, null, metricName, appearanceDetails);
                                    entry.target.innerHTML = ''; // Clear placeholder/old content
                                    entry.target.appendChild(liveSvgElement);
                                }
                                
                                createMiniGraphSVG(liveData, newViewBoxWidth, graphHeight, newViewBoxWidth, graphHeight, liveSvgElement, metricName, appearanceDetails);
                                
                                if (liveData && liveData.length > 0) {
                                    liveSvgElement.style.visibility = 'visible';
                                } else {
                                    liveSvgElement.style.visibility = 'hidden';
                                }
                            });
                        }
                    });
                    observer.observe(targetCellElement);
                    graphCellResizeObservers.set(targetCellElement, observer);
                }
            });
        };

        if (guestStatus !== STATUS_RUNNING) {
            cleanupResizeObserver(targetCellElement);
            if (targetCellElement.innerHTML !== placeholderDivHTML) {
                targetCellElement.innerHTML = placeholderDivHTML;
            }
            delete PulseApp.state.graphDataCache[cacheKey];
            return;
        }

        if (isThresholdSliderDrag) {
            const cachedData = PulseApp.state.graphDataCache[cacheKey];
            // Even for drag, if there's no data, we might show a placeholder.
            // The main concern is avoiding draw with bad width.
            if (cachedData) {
                 setupObserverAndRender(cachedData);
            } else {
                // No cached data during drag: show placeholder & ensure observer is cleaned up
                cleanupResizeObserver(targetCellElement); 
                if (targetCellElement.innerHTML !== placeholderDivHTML) {
                    targetCellElement.innerHTML = placeholderDivHTML;
                }
            }
            return;
        }

        // NOT DRAGGING: Set initial placeholder, then fetch, then setup observer and render.
        if (targetCellElement.innerHTML !== placeholderDivHTML && !targetCellElement.querySelector('svg.mini-graph-svg')) {
            cleanupResizeObserver(targetCellElement); // Clean up if putting placeholder before fetch
            targetCellElement.innerHTML = placeholderDivHTML;
        }

        try {
            const response = await fetch(`/api/history/${guestUniqueId}/${metricName}?duration=3600`);
            let fetchedData = null;

            if (response.ok) {
                fetchedData = await response.json();
                PulseApp.state.graphDataCache[cacheKey] = fetchedData;
            } else {
                cleanupResizeObserver(targetCellElement);
                let errorText = `Error ${response.status}`;
                try {
                    const errorData = await response.json();
                    errorText = errorData.error || errorData.message || errorText;
                } catch (e) { /* Ignore */ }
                targetCellElement.innerHTML = `<span class="text-xs italic text-red-400">${errorText}</span>`;
                delete PulseApp.state.graphDataCache[cacheKey];
                return;
            }
            setupObserverAndRender(fetchedData);

        } catch (error) {
            console.error(`Fetch failed for ${metricName} on ${guestUniqueId}:`, error);
            cleanupResizeObserver(targetCellElement);
            requestAnimationFrame(() => { // Ensure DOM update for error is also safe
                targetCellElement.innerHTML = '<span class="text-xs italic text-red-500">Fetch Failed</span>';
            });
            delete PulseApp.state.graphDataCache[cacheKey];
        }
    }

    // NEW function to update an existing guest row
    function updateGuestRow(row, guest) {
        const isThresholdDrag = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();

        const nameCell = row.cells[0]; // Define nameCell once here

        // Health status dot update (always needed)
        const currentDotSpan = nameCell.querySelector('span.inline-block.w-2.h-2.rounded-full');
        const newDotHTML = healthDotHTML(guest.healthStatus);
        const newDotFragment = document.createRange().createContextualFragment(newDotHTML).firstChild;
        
        if (!currentDotSpan || currentDotSpan.className !== newDotFragment.className || currentDotSpan.title !== newDotFragment.title) {
            if (currentDotSpan) currentDotSpan.replaceWith(newDotFragment);
            else nameCell.insertBefore(newDotFragment, nameCell.firstChild);
        }

        if (isThresholdDrag) {
            // --- Targeted updates during threshold drag ---
            if (graphViewActive) {
                frontendMetricConfigs.forEach((config, index) => {
                    const metricCell = row.cells[4 + index];
                    if (metricCell) {
                        let sparklineAppearanceDetails = null;
                        if (config.isRate) {
                            const thresholdState = PulseApp.state.getThresholdState();
                            const metricThreshold = thresholdState[config.thresholdKey];
                            if (metricThreshold && metricThreshold.value > 0) {
                                let guestValue = guest[config.guestProperty];
                                if (guestValue !== null && guestValue !== undefined && guestValue >= metricThreshold.value) {
                                    sparklineAppearanceDetails = {
                                        look: triggeredThresholdSparklineLook,
                                        thresholdInfo: { value: metricThreshold.value, key: config.thresholdKey }
                                    };
                                }
                            }
                        }
                        if (guest.status === STATUS_RUNNING) {
                            if (guest.type === GUEST_TYPE_VM && config.apiMetricName === 'disk_usage_percent') {
                                if (!metricCell.querySelector('svg.mini-graph-svg')) {
                                    const placeholderDiskVM = '<div class="h-[22px] w-full flex items-center justify-center text-xs text-gray-400">-</div>';
                                    if (metricCell.innerHTML !== placeholderDiskVM) metricCell.innerHTML = placeholderDiskVM;
                                }
                            } else {
                                fetchAndRenderMetricGraph(guest.uniqueId, config.apiMetricName, metricCell, guest.status, sparklineAppearanceDetails, true /*isThresholdDrag*/);
                            }
                        } else {
                             fetchAndRenderMetricGraph(guest.uniqueId, config.apiMetricName, metricCell, guest.status, sparklineAppearanceDetails, true /*isThresholdDrag*/);
                        }
                    }
                });
            } else {
                // Standard view: Only update rate metric text colors
                const thresholdState = PulseApp.state.getThresholdState();
                const rateMetricCells = {
                    diskread: row.cells[7],
                    diskwrite: row.cells[8],
                    netin: row.cells[9],
                    netout: row.cells[10]
                };
                frontendMetricConfigs.forEach(config => {
                    if (config.isRate && rateMetricCells[config.thresholdKey]) {
                        const cell = rateMetricCells[config.thresholdKey];
                        let iconHTML = '';
                        if (config.thresholdKey === 'netin' || config.thresholdKey === 'netout') {
                            // const iconSpan = cell.querySelector('span.text-xs.mr-1');
                            // if (iconSpan) iconHTML = iconSpan.outerHTML; // Old logic: only preserved existing
                            
                            const downArrow = '↓';
                            const upArrow = '↑';
                            let arrow = '';
                            let colorClass = 'text-gray-400 dark:text-gray-500'; // Default to gray

                            if (guest.status === STATUS_RUNNING) {
                                if (config.thresholdKey === 'netin') {
                                    arrow = downArrow;
                                    if (guest.net_in_rate > 0) {
                                        colorClass = 'text-green-500';
                                    }
                                } else { // netout
                                    arrow = upArrow;
                                    if (guest.net_out_rate > 0) {
                                        colorClass = 'text-red-500';
                                    }
                                }
                            } else { // Not running
                                arrow = (config.thresholdKey === 'netin') ? downArrow : upArrow;
                            }
                            iconHTML = `<span class="text-xs mr-1 ${colorClass}">${arrow}</span>`;

                        }
                        const newStyledValueHTML = getStandardRateValueHTML(guest, config, thresholdState);
                        const finalHTML = iconHTML + newStyledValueHTML;
                        if (cell.innerHTML !== finalHTML) {
                            cell.innerHTML = finalHTML;
                        }
                    }
                });
            }
            return; // End updateGuestRow early for drag
        }

        // --- Full update (not dragging) ---
        // Status class on row itself
        if (row.dataset.status !== guest.status) {
            let baseRowClass = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50';
            if (graphViewActive) {
                baseRowClass += ' guest-row';
            }
            row.className = baseRowClass;
            if (guest.status === STATUS_STOPPED) {
                row.classList.add('opacity-60', 'grayscale');
            } else {
                row.classList.remove('opacity-60', 'grayscale');
            }
            row.dataset.status = guest.status;
        }
        
        // Name cell (index 0) - Text part (dot is handled above)
        const nameTextNode = Array.from(nameCell.childNodes).find(node => node.nodeType === Node.TEXT_NODE && node.textContent.trim() !== '');
        const guestNameText = guest.name || '';
        if (!nameTextNode || nameTextNode.textContent !== guestNameText) {
            if (nameTextNode) {
                nameTextNode.textContent = guestNameText;
            } else {
                // Ensure dot exists before appending text, or append after the existing dot
                const dot = nameCell.querySelector('span.inline-block.w-2.h-2.rounded-full');
                if (dot && dot.nextSibling) {
                    nameCell.insertBefore(document.createTextNode(guestNameText), dot.nextSibling);
                } else if (dot) {
                    nameCell.appendChild(document.createTextNode(guestNameText));
                } else { // Should not happen if dot logic is correct
                    nameCell.appendChild(document.createTextNode(guestNameText)); 
                }
            }
        }
        if (nameCell.title !== guest.name) nameCell.title = guest.name; // Keep title in sync for the whole cell

        // Type cell (index 1)
        const typeCell = row.cells[1];
        const typeIconClass = guest.type === GUEST_TYPE_VM
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const newTypeHtml = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;
        if (typeCell.innerHTML !== newTypeHtml) {
            typeCell.innerHTML = newTypeHtml;
        }

        // ID cell (index 2)
        const idCell = row.cells[2];
        if (idCell.textContent !== guest.id.toString()) {
            idCell.textContent = guest.id;
        }

        // Uptime cell (index 3)
        const uptimeCell = row.cells[3];
        let newUptimeHtml = '-';
        if (guest.status === STATUS_RUNNING) {
            newUptimeHtml = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.uptime < 3600) { 
                newUptimeHtml = `<span class="text-orange-500">${newUptimeHtml}</span>`;
            }
        }
        if (uptimeCell.innerHTML !== newUptimeHtml) {
            uptimeCell.innerHTML = newUptimeHtml;
        }

        if (graphViewActive) {
            // Metric graph cells (starting from index 4) - full update, not just appearance
            frontendMetricConfigs.forEach((config, index) => {
                const metricCell = row.cells[4 + index];
                if (metricCell) { 
                    let sparklineAppearanceDetails = null; 
                    if (config.isRate) {
                        const thresholdState = PulseApp.state.getThresholdState();
                        const metricThreshold = thresholdState[config.thresholdKey];
                        if (metricThreshold && metricThreshold.value > 0) {
                            let guestValue = guest[config.guestProperty];
                            if (guestValue !== null && guestValue !== undefined && guestValue >= metricThreshold.value) {
                                sparklineAppearanceDetails = {
                                    look: triggeredThresholdSparklineLook,
                                    thresholdInfo: { value: metricThreshold.value, key: config.thresholdKey }
                                };
                            }
                        }
                    }
                    if (guest.status === STATUS_RUNNING) {
                        if (guest.type === GUEST_TYPE_VM && config.apiMetricName === 'disk_usage_percent') {
                            const placeholderDiskVM = '<div class="h-[22px] w-full flex items-center justify-center text-xs text-gray-400">-</div>';
                            if (metricCell.innerHTML !== placeholderDiskVM) metricCell.innerHTML = placeholderDiskVM;
                        } else {
                            fetchAndRenderMetricGraph(guest.uniqueId, config.apiMetricName, metricCell, guest.status, sparklineAppearanceDetails, false /*isThresholdDrag=false*/);
                        }
                    } else {
                        fetchAndRenderMetricGraph(guest.uniqueId, config.apiMetricName, metricCell, guest.status, sparklineAppearanceDetails, false /*isThresholdDrag=false*/);
                    }
                }
            });
        } else {
            // Standard view: CPU, Memory, Disk bars, and text stats (cells 4-10) - full update
            const cpuCell = row.cells[4];
            const newCpuBarHTML = _createCpuBarHtml(guest);
            if (cpuCell.innerHTML !== newCpuBarHTML) cpuCell.innerHTML = newCpuBarHTML;

            const memoryCell = row.cells[5];
            const newMemoryBarHTML = _createMemoryBarHtml(guest);
            if (memoryCell.innerHTML !== newMemoryBarHTML) memoryCell.innerHTML = newMemoryBarHTML;

            const diskCell = row.cells[6];
            const newDiskBarHTML = _createDiskBarHtml(guest);
            if (diskCell.innerHTML !== newDiskBarHTML) diskCell.innerHTML = newDiskBarHTML;

            const thresholdState = PulseApp.state.getThresholdState();
            const rateMetricCells = {
                diskread: row.cells[7],
                diskwrite: row.cells[8],
                netin: row.cells[9],
                netout: row.cells[10]
            };
            frontendMetricConfigs.forEach(config => {
                if (config.isRate && rateMetricCells[config.thresholdKey]) {
                    const cell = rateMetricCells[config.thresholdKey];
                    let iconHTML = '';
                    if (config.thresholdKey === 'netin' || config.thresholdKey === 'netout') {
                        // const iconSpan = cell.querySelector('span.text-xs.mr-1');
                        // if (iconSpan) iconHTML = iconSpan.outerHTML; // Old logic: only preserved existing
                        
                        const downArrow = '↓';
                        const upArrow = '↑';
                        let arrow = '';
                        let colorClass = 'text-gray-400 dark:text-gray-500'; // Default to gray

                        if (guest.status === STATUS_RUNNING) {
                            if (config.thresholdKey === 'netin') {
                                arrow = downArrow;
                                if (guest.net_in_rate > 0) {
                                    colorClass = 'text-green-500';
                                }
                            } else { // netout
                                arrow = upArrow;
                                if (guest.net_out_rate > 0) {
                                    colorClass = 'text-red-500';
                                }
                            }
                        } else { // Not running
                            arrow = (config.thresholdKey === 'netin') ? downArrow : upArrow;
                        }
                        iconHTML = `<span class="text-xs mr-1 ${colorClass}">${arrow}</span>`;

                    }
                    const newStyledValueHTML = getStandardRateValueHTML(guest, config, thresholdState);
                    const finalHTML = iconHTML + newStyledValueHTML;
                    if (cell.innerHTML !== finalHTML) {
                        cell.innerHTML = finalHTML;
                    }
                }
            });
        }
    }

    function createGuestRow(guest) {
        // Standard view row creation (non-graph view)
        if (!graphViewActive) {
            const row = document.createElement('tr');
            row.className = `border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 ${guest.status === STATUS_STOPPED ? 'opacity-60 grayscale' : ''}`;
            row.setAttribute('data-name', guest.name.toLowerCase());
            row.setAttribute('data-type', guest.type.toLowerCase());
            row.setAttribute('data-node', guest.node.toLowerCase());
            row.setAttribute('data-id', guest.id); // original id
            row.dataset.uniqueId = guest.uniqueId; // Add uniqueId for precise DOM targeting

            const cpuBarHTML = _createCpuBarHtml(guest);
            const memoryBarHTML = _createMemoryBarHtml(guest);
            const diskBarHTML = _createDiskBarHtml(guest);

            const thresholdState = PulseApp.state.getThresholdState(); // Get once for all rate metrics

            const diskReadConfig = frontendMetricConfigs.find(c => c.thresholdKey === 'diskread');
            const diskReadHTML = diskReadConfig ? getStandardRateValueHTML(guest, diskReadConfig, thresholdState) : (guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.disk_read_rate, 0) : '-');

            const diskWriteConfig = frontendMetricConfigs.find(c => c.thresholdKey === 'diskwrite');
            const diskWriteHTML = diskWriteConfig ? getStandardRateValueHTML(guest, diskWriteConfig, thresholdState) : (guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.disk_write_rate, 0) : '-');

            const upArrow = '↑';
            const downArrow = '↓';
            let netInIcon = '';
            let netOutIcon = '';

            if (guest.status === STATUS_RUNNING) {
                const netInActive = guest.net_in_rate > 0;
                const netOutActive = guest.net_out_rate > 0;
                netInIcon = `<span class="text-xs mr-1 ${netInActive ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'}">${downArrow}</span>`;
                netOutIcon = `<span class="text-xs mr-1 ${netOutActive ? 'text-red-500' : 'text-gray-400 dark:text-gray-500'}">${upArrow}</span>`;
            } else {
                netInIcon = `<span class="text-xs mr-1 text-gray-400 dark:text-gray-500">${downArrow}</span>`;
                netOutIcon = `<span class="text-xs mr-1 text-gray-400 dark:text-gray-500">${upArrow}</span>`;
            }
            
            const netInConfig = frontendMetricConfigs.find(c => c.thresholdKey === 'netin');
            const netInFormattedHTML = netInConfig ? getStandardRateValueHTML(guest, netInConfig, thresholdState) : (guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.net_in_rate, 0) : '-');
            
            const netOutConfig = frontendMetricConfigs.find(c => c.thresholdKey === 'netout');
            const netOutFormattedHTML = netOutConfig ? getStandardRateValueHTML(guest, netOutConfig, thresholdState) : (guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeed(guest.net_out_rate, 0) : '-');

            const typeIconClass = guest.type === GUEST_TYPE_VM
                ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
                : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
            const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;

            let uptimeDisplay = '-';
            if (guest.status === STATUS_RUNNING) {
                uptimeDisplay = PulseApp.utils.formatUptime(guest.uptime);
                if (guest.uptime < 3600) { 
                    uptimeDisplay = `<span class="text-orange-500">${uptimeDisplay}</span>`;
                }
            }

            row.innerHTML = `
                <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${healthDotHTML(guest.healthStatus)}${guest.name}</td>
                <td class="p-1 px-2">${typeIcon}</td>
                <td class="p-1 px-2">${guest.id}</td>
                <td class="p-1 px-2 whitespace-nowrap">${uptimeDisplay}</td>
                <td class="p-1 px-2">${cpuBarHTML}</td>
                <td class="p-1 px-2">${memoryBarHTML}</td>
                <td class="p-1 px-2">${diskBarHTML}</td>
                <td class="p-1 px-2 whitespace-nowrap">${diskReadHTML}</td>
                <td class="p-1 px-2 whitespace-nowrap">${diskWriteHTML}</td>
                <td class="p-1 px-2 whitespace-nowrap">${netInIcon}${netInFormattedHTML}</td>
                <td class="p-1 px-2 whitespace-nowrap">${netOutIcon}${netOutFormattedHTML}</td>
            `;
            return row;
        }

        // Graph view row creation
        const row = document.createElement('tr');
        row.className = 'guest-row border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50';
        if (guest.status === STATUS_STOPPED) {
            row.classList.add('opacity-60', 'grayscale');
        }
        row.dataset.vmid = guest.vmid; // Retain for compatibility if anything uses it
        row.dataset.node = guest.node;
        row.dataset.type = guest.type;
        row.dataset.name = guest.name;
        row.dataset.status = guest.status;
        row.dataset.uniqueId = guest.uniqueId; // Crucial for finding and updating this row

        // Name, Type, ID, Uptime cells
        const nameCell = document.createElement('td');
        nameCell.className = 'p-1 px-2 whitespace-nowrap truncate';
        nameCell.title = guest.name;
        nameCell.textContent = guest.name;
        row.appendChild(nameCell);

        const typeCell = document.createElement('td');
        typeCell.className = 'p-1 px-2';
        const typeIconClassGraph = guest.type === GUEST_TYPE_VM
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        typeCell.innerHTML = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClassGraph}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;
        row.appendChild(typeCell);

        const idCell = document.createElement('td');
        idCell.className = 'p-1 px-2';
        idCell.textContent = guest.id;
        row.appendChild(idCell);

        const uptimeCell = document.createElement('td');
        uptimeCell.className = 'p-1 px-2 whitespace-nowrap';
        if (guest.status === STATUS_RUNNING) {
            let uptimeDisplayGraph = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.uptime < 3600) { 
                uptimeDisplayGraph = `<span class="text-orange-500">${uptimeDisplayGraph}</span>`;
            }
            uptimeCell.innerHTML = uptimeDisplayGraph;
        } else {
            uptimeCell.textContent = '-';
        }
        row.appendChild(uptimeCell);

        const isThresholdDragForCreate = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();
        // Metric graph cells
        frontendMetricConfigs.forEach(config => {
            const metricCell = document.createElement('td');
            metricCell.className = 'p-1 px-2 align-middle text-center'; // Match class from original
            // Add a data-metric attribute to help identify the cell if needed later
            metricCell.dataset.metricName = config.apiMetricName; 

            let sparklineAppearanceDetails = null;
            if (config.isRate) {
                const thresholdState = PulseApp.state.getThresholdState();
                const metricThreshold = thresholdState[config.thresholdKey];
                if (metricThreshold && metricThreshold.value > 0) {
                    let guestValue = guest[config.guestProperty];
                    if (guestValue !== null && guestValue !== undefined && guestValue >= metricThreshold.value) {
                        sparklineAppearanceDetails = {
                            look: triggeredThresholdSparklineLook,
                            thresholdInfo: { value: metricThreshold.value, key: config.thresholdKey }
                        };
                    }
                }
            }

            if (guest.status === STATUS_RUNNING) {
                if (guest.type === GUEST_TYPE_VM && config.apiMetricName === 'disk_usage_percent') {
                    const placeholderDiskVM = '<div class="h-[22px] w-full flex items-center justify-center text-xs text-gray-400">-</div>';
                    if (metricCell.innerHTML !== placeholderDiskVM) metricCell.innerHTML = placeholderDiskVM;
                } else {
                    fetchAndRenderMetricGraph(guest.uniqueId, config.apiMetricName, metricCell, guest.status, sparklineAppearanceDetails, isThresholdDragForCreate);
                }
            } else {
                metricCell.innerHTML = '<div class="h-[22px] w-full border border-gray-300 dark:border-gray-600 rounded my-1 flex items-center justify-center text-xs text-gray-400">-</div>';
                // metricCell.pulseGraphData = null; // This line is removed
            }
            row.appendChild(metricCell);
        });
        return row;
    }

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow
    };
})();
