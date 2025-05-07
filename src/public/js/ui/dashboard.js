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

    function refreshDashboardData() {
        PulseApp.state.set('dashboardData', []);
        let dashboardData = [];

        let maxNameLength = 0;
        let maxUptimeLength = 0;

        function calculateAverage(historyArray, key) {
            if (!historyArray || historyArray.length === 0) return null;
            const validEntries = historyArray.filter(entry => typeof entry[key] === 'number' && !isNaN(entry[key]));
            if (validEntries.length === 0) return null;
            const sum = validEntries.reduce((acc, curr) => acc + curr[key], 0);
            return sum / validEntries.length;
        }

        function calculateAverageRate(historyArray, key) {
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

            if (timeDiffSeconds <= 0) {
                return null;
            }

            if (valueDiff < 0) {
                return null;
            }
            
            return valueDiff / timeDiffSeconds;
        }

        const processGuest = (guest, type) => {
            let avgCpu = 0, avgMem = 0, avgDisk = 0;
            let avgDiskReadRate = null, avgDiskWriteRate = null, avgNetInRate = null, avgNetOutRate = null;
            let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';
            let effectiveMemorySource = 'host'; // 'host' or 'guest'
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

            // Check for drag state and apply snapshot if active
            const isDragging = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();
            const snapshot = guestMetricDragSnapshot[guestUniqueId];

            if (isDragging && snapshot) {
                avgDiskReadRate = snapshot.diskread;
                avgDiskWriteRate = snapshot.diskwrite;
                avgNetInRate = snapshot.netin;
                avgNetOutRate = snapshot.netout;

                // For other metrics, continue with live data or defaults if snapshot doesn't cover them
                // or if we decide to only snapshot I/O rates.
                // For now, let other metrics be calculated as usual even during drag.
                if (guest.status === 'running' && metrics && metrics.current) {
                    const currentDataPoint = {
                        timestamp: Date.now(),
                        ...metrics.current
                    };
                    PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                    const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];
                    avgCpu = calculateAverage(history, 'cpu') ?? 0;
                    avgMem = calculateAverage(history, 'mem') ?? 0;
                    avgDisk = calculateAverage(history, 'disk') ?? 0;
                    // I/O rates are already set from snapshot if dragging
                } else {
                    PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
                    // If not running, CPU/Mem/Disk also go to their defaults (0 or N/A)
                }

            } else {
                // Original logic if not dragging or no snapshot
                if (guest.status === 'running' && metrics && metrics.current) {
                    let baseMemoryValue = metrics.current.mem; // Default to host memory
                    currentMemTotalForDisplay = guest.maxmem; // Default to host allocated max memory
                    effectiveMemorySource = 'host';

                    // Check if guest agent memory data is available and valid
                    if (metrics.current.guest_mem_actual_used_bytes !== undefined && metrics.current.guest_mem_actual_used_bytes !== null) {
                        baseMemoryValue = metrics.current.guest_mem_actual_used_bytes;
                        effectiveMemorySource = 'guest';
                        // If guest provides its own total, use it, otherwise stick to guest.maxmem
                        if (metrics.current.guest_mem_total_bytes !== undefined && metrics.current.guest_mem_total_bytes > 0) {
                            currentMemTotalForDisplay = metrics.current.guest_mem_total_bytes;
                        }
                        // console.log(`VM ${guest.vmid}: Using GUEST memory: ${baseMemoryValue / (1024*1024)} MB / ${currentMemTotalForDisplay / (1024*1024)} MB`);
                    } else {
                        // console.log(`VM ${guest.vmid}: Using HOST memory: ${baseMemoryValue / (1024*1024)} MB / ${currentMemTotalForDisplay / (1024*1024)} MB`);
                    }
                    currentMemForAvg = baseMemoryValue;

                    const currentDataPoint = {
                        timestamp: Date.now(),
                        ...metrics.current,
                        effective_mem: currentMemForAvg, // Store the memory value actually used for averaging
                        effective_mem_total: currentMemTotalForDisplay, // Store total used for percentage calc
                        effective_mem_source: effectiveMemorySource // Store the source for tooltip
                    };
                    PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                    const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];

                    avgCpu = calculateAverage(history, 'cpu') ?? 0;
                    avgMem = calculateAverage(history, 'effective_mem') ?? 0; // Average the memory value we decided to use
                    avgDisk = calculateAverage(history, 'disk') ?? 0;
                    avgDiskReadRate = calculateAverageRate(history, 'diskread');
                    avgDiskWriteRate = calculateAverageRate(history, 'diskwrite');
                    avgNetInRate = calculateAverageRate(history, 'netin');
                    avgNetOutRate = calculateAverageRate(history, 'netout');
                } else {
                    PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
                    // Rates remain null (will be N/A), others default
                }
            }
            
            // Use the total memory that corresponds to the source of avgMem for percentage calculation
            // If history has entries, the last one should have effective_mem_total and effective_mem_source
            const historyForGuest = PulseApp.state.getDashboardHistory()[guestUniqueId];
            let finalMemTotalForPercent = guest.maxmem; // Fallback
            let finalMemSourceForTooltip = 'host'; // Fallback

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

            const name = guest.name || `${guest.type === 'qemu' ? 'VM' : 'CT'} ${guest.vmid}`;
            const uptimeFormatted = PulseApp.utils.formatUptime(guest.uptime);
            if (name.length > maxNameLength) maxNameLength = name.length;
            if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;

            let rawHostReportedMem = null;
            if (guest.status === 'running' && metrics && metrics.current && metrics.current.mem !== undefined) {
                rawHostReportedMem = metrics.current.mem;
            }

            dashboardData.push({
                id: guest.vmid,
                uniqueId: guestUniqueId,
                vmid: guest.vmid,
                name: name,
                node: guest.node,
                type: guest.type === 'qemu' ? 'VM' : 'CT',
                status: guest.status,
                cpu: avgCpu,
                cpus: guest.cpus || 1,
                memory: avgMemoryPercent,
                memoryCurrent: avgMem, // This is now potentially guest actual used or host used
                memoryTotal: finalMemTotalForPercent, // This is now potentially guest total or host allocated
                memorySource: finalMemSourceForTooltip, // Add source for tooltip generation
                rawHostMemory: rawHostReportedMem, // Store host raw memory for tooltip comparison
                disk: avgDiskPercent,
                diskCurrent: avgDisk,
                diskTotal: guest.maxdisk,
                uptime: guest.status === 'running' ? guest.uptime : 0,
                diskread: avgDiskReadRate,
                diskwrite: avgDiskWriteRate,
                netin: avgNetInRate,
                netout: avgNetOutRate
            });
        };

        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];

        vmsData.forEach(vm => processGuest(vm, 'qemu'));
        containersData.forEach(ct => processGuest(ct, 'lxc'));

        PulseApp.state.set('dashboardData', dashboardData);

        const nameColWidth = Math.min(Math.max(maxNameLength * 8 + 16, 100), 300);
        const uptimeColWidth = Math.max(maxUptimeLength * 7 + 16, 80);
        const htmlElement = document.documentElement;
        if (htmlElement) {
            htmlElement.style.setProperty('--name-col-width', `${nameColWidth}px`);
            htmlElement.style.setProperty('--uptime-col-width', `${uptimeColWidth}px`);
        }
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

        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];

        let filteredData = dashboardData.filter(guest => {
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

        const sortStateMain = PulseApp.state.getSortState('main');
        let sortedData = PulseApp.utils.sortData(filteredData, sortStateMain.column, sortStateMain.direction, 'main');

        let visibleCount = 0;
        let visibleNodes = new Set();
        const groupByNode = PulseApp.state.get('groupByNode');

        if (groupByNode) {
            const nodeGroups = {};
            sortedData.forEach(guest => {
                const nodeName = guest.node || 'Unknown Node';
                if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
                nodeGroups[nodeName].push(guest);
            });
            
            tableBodyEl.innerHTML = '';

            Object.keys(nodeGroups).sort().forEach(nodeName => {
                visibleNodes.add(nodeName.toLowerCase());
                const nodeHeaderRow = document.createElement('tr');
                nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
                nodeHeaderRow.innerHTML = `<td colspan="11" class="px-2 py-1">
                    <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
                    ${nodeName}
                </td>`;
                tableBodyEl.appendChild(nodeHeaderRow);

                nodeGroups[nodeName].forEach(guest => {
                    const guestRow = createGuestRow(guest);
                    if (guestRow) {
                        tableBodyEl.appendChild(guestRow);
                        visibleCount++;
                    }
                });
            });
        } else {
             PulseApp.utils.renderTableBody(tableBodyEl, sortedData, createGuestRow, "No matching guests found.", 11);
             visibleCount = sortedData.length;
             sortedData.forEach(guest => visibleNodes.add((guest.node || 'Unknown Node').toLowerCase()));
        }

        if (visibleCount === 0) {
            let filterDescription = [];
            if (filterGuestType !== FILTER_ALL) filterDescription.push(`Type: ${filterGuestType.toUpperCase()}`);
            if (filterStatus !== FILTER_ALL) filterDescription.push(`Status: ${filterStatus}`);
            if (textSearchTerms.length > 0) filterDescription.push(`Search: "${textSearchTerms.join(', ')}"`);
            const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
            if (activeThresholds.length > 0) {
                const thresholdTexts = activeThresholds.map(([key, state]) => {
                    return `${PulseApp.utils.getReadableThresholdName(key)}>=${PulseApp.utils.formatThresholdValue(key, state.value)}`;
                });
                filterDescription.push(`Thresholds: ${thresholdTexts.join(', ')}`);
            }

            let message = "No guests match the current filters";
            if (filterDescription.length > 0) {
                message += ` (${filterDescription.join('; ')})`;
            }
            message += ".";

            tableBodyEl.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
        }

        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = textSearchTerms.length > 0 ? ` | Search: "${textSearchTerms.join(', ')}"` : '';
        const typeLabel = filterGuestType !== FILTER_ALL ? filterGuestType.toUpperCase() : '';
        const statusLabel = filterStatus !== FILTER_ALL ? filterStatus : '';
        const otherFilters = [typeLabel, statusLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        let statusCountText = ` | Showing ${visibleCount} guests`;
        if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
        statusElementEl.textContent = statusBaseText + statusFilterText + statusCountText;

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

    function createGuestRow(guest) {
        const row = document.createElement('tr');
        row.className = `border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 ${guest.status === STATUS_STOPPED ? 'opacity-60 grayscale' : ''}`;
        row.setAttribute('data-name', guest.name.toLowerCase());
        row.setAttribute('data-type', guest.type.toLowerCase());
        row.setAttribute('data-node', guest.node.toLowerCase());
        row.setAttribute('data-id', guest.id);

        let cpuBarHTML = '-';
        let memoryBarHTML = '-';
        let diskBarHTML = '-';
        let diskReadFormatted = '-';
        let diskWriteFormatted = '-';
        let netInFormatted = '-';
        let netOutFormatted = '-';

        if (guest.status === STATUS_RUNNING) {
            const cpuPercent = Math.round(guest.cpu * 100);
            const memoryPercent = guest.memory;

            const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : ''}`;
            
            // Simplified memoryTooltipText
            let memoryTooltipText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)} (${memoryPercent}%)`;
            if (guest.type === GUEST_TYPE_VM && guest.memorySource === 'guest' && guest.rawHostMemory !== null && guest.rawHostMemory !== undefined) {
                // If guest source is used for the main display, append host's view for comparison.
                memoryTooltipText += ` (Host: ${PulseApp.utils.formatBytes(guest.rawHostMemory)})`;
            }

            const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent);
            const memColorClass = PulseApp.utils.getUsageColor(memoryPercent);

            cpuBarHTML = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
            memoryBarHTML = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);

            if (guest.type === GUEST_TYPE_CT) {
                const diskPercent = guest.disk;
                const diskTooltipText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
                const diskColorClass = PulseApp.utils.getUsageColor(diskPercent);
                diskBarHTML = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
            } else {
                if (guest.diskTotal) {
                    const totalDiskFormatted = PulseApp.utils.formatBytes(guest.diskTotal);
                    diskBarHTML = `<span class="text-xs text-gray-700 dark:text-gray-200 truncate">${totalDiskFormatted}</span>`;
                } else {
                     diskBarHTML = '-';
                }
            }

            diskReadFormatted = PulseApp.utils.formatSpeed(guest.diskread, 0);
            diskWriteFormatted = PulseApp.utils.formatSpeed(guest.diskwrite, 0);
            netInFormatted = PulseApp.utils.formatSpeed(guest.netin, 0);
            netOutFormatted = PulseApp.utils.formatSpeed(guest.netout, 0);
          }

          const typeIconClass = guest.type === GUEST_TYPE_VM
              ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
              : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
          const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;

          row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2">${guest.id}</td>
            <td class="p-1 px-2 whitespace-nowrap">${guest.status === STATUS_STOPPED ? '-' : PulseApp.utils.formatUptime(guest.uptime)}</td>
            <td class="p-1 px-2">${cpuBarHTML}</td>
            <td class="p-1 px-2">${memoryBarHTML}</td>
            <td class="p-1 px-2">${diskBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap">${diskReadFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${diskWriteFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${netInFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${netOutFormatted}</td>
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