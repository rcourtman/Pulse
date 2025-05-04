PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.dashboard = (() => {
    let searchInput = null;

    function init() {
        searchInput = document.getElementById('dashboard-search');
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
            const timeDiff = (newest.timestamp - oldest.timestamp) / 1000;

            if (timeDiff <= 0) return 0;
            return valueDiff / timeDiff;
        }

        const processGuest = (guest, type) => {
            let avgCpu = 0, avgMem = 0, avgDisk = 0;
            let avgDiskReadRate = 0, avgDiskWriteRate = 0, avgNetInRate = 0, avgNetOutRate = 0;
            let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';

            const metricsData = PulseApp.state.get('metricsData') || [];
            const metrics = metricsData.find(m =>
                m.id === guest.vmid &&
                m.type === guest.type &&
                m.node === guest.node &&
                m.endpointId === guest.endpointId
            );

            const guestUniqueId = guest.id;

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
                avgDiskReadRate = calculateAverageRate(history, 'diskread') ?? 0;
                avgDiskWriteRate = calculateAverageRate(history, 'diskwrite') ?? 0;
                avgNetInRate = calculateAverageRate(history, 'netin') ?? 0;
                avgNetOutRate = calculateAverageRate(history, 'netout') ?? 0;
                avgMemoryPercent = (guest.maxmem > 0) ? Math.round(avgMem / guest.maxmem * 100) : 'N/A';
                avgDiskPercent = (guest.maxdisk > 0) ? Math.round(avgDisk / guest.maxdisk * 100) : 'N/A';

            } else {
                 PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }

            const name = guest.name || `${guest.type === 'qemu' ? 'VM' : 'CT'} ${guest.vmid}`;
            const uptimeFormatted = PulseApp.utils.formatUptime(guest.uptime);
            if (name.length > maxNameLength) maxNameLength = name.length;
            if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;

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
                memoryCurrent: avgMem,
                memoryTotal: guest.maxmem,
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
        const tableBody = document.querySelector('#main-table tbody');
        const statusElement = document.getElementById('dashboard-status-text');
        if (!tableBody || !statusElement) {
            console.error('Dashboard table body or status element not found!');
            return;
        }

        refreshDashboardData();

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const thresholdState = PulseApp.state.getThresholdState();

        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];

        let filteredData = dashboardData.filter(guest => {
            const typeMatch = filterGuestType === 'all' || (filterGuestType === 'vm' && guest.type === 'VM') || (filterGuestType === 'lxc' && guest.type === 'CT');
            const statusMatch = filterStatus === 'all' || guest.status === filterStatus;

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

                if (type === 'cpu') guestValue = guest.cpu * 100;
                else if (type === 'memory') guestValue = guest.memory;
                else if (type === 'disk') guestValue = guest.disk;
                else if (type === 'diskread') guestValue = guest.diskread;
                else if (type === 'diskwrite') guestValue = guest.diskwrite;
                else if (type === 'netin') guestValue = guest.netin;
                else if (type === 'netout') guestValue = guest.netout;
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

        tableBody.innerHTML = '';
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

            Object.keys(nodeGroups).sort().forEach(nodeName => {
                visibleNodes.add(nodeName.toLowerCase());
                const nodeHeaderRow = document.createElement('tr');
                nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
                nodeHeaderRow.innerHTML = `<td colspan="11" class="px-2 py-1">
                    <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
                    ${nodeName}
                </td>`;
                tableBody.appendChild(nodeHeaderRow);

                nodeGroups[nodeName].forEach(guest => {
                    const guestRow = createGuestRow(guest);
                    if (guestRow) {
                        tableBody.appendChild(guestRow);
                        visibleCount++;
                    }
                });
            });
        } else {
            sortedData.forEach(guest => {
                const guestRow = createGuestRow(guest);
                 if (guestRow) {
                    tableBody.appendChild(guestRow);
                    visibleCount++;
                    visibleNodes.add((guest.node || 'Unknown Node').toLowerCase());
                }
            });
        }

        if (visibleCount === 0) {
            let filterDescription = [];
            if (filterGuestType !== 'all') filterDescription.push(`Type: ${filterGuestType.toUpperCase()}`);
            if (filterStatus !== 'all') filterDescription.push(`Status: ${filterStatus}`);
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

            tableBody.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
        }

        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = textSearchTerms.length > 0 ? ` | Search: "${textSearchTerms.join(', ')}"` : '';
        const typeLabel = filterGuestType !== 'all' ? filterGuestType.toUpperCase() : '';
        const statusLabel = filterStatus !== 'all' ? filterStatus : '';
        const otherFilters = [typeLabel, statusLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        let statusCountText = ` | Showing ${visibleCount} guests`;
        if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
        statusElement.textContent = statusBaseText + statusFilterText + statusCountText;

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
        row.className = `border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 ${guest.status === 'stopped' ? 'opacity-60 grayscale' : ''}`;
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

        if (guest.status === 'running') {
            const cpuPercent = Math.round(guest.cpu * 100);
            const memoryPercent = guest.memory;

            const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : ''}`;
            const memoryTooltipText = guest.memoryTotal ? `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)} (${memoryPercent}%)` : `${memoryPercent}%`;

            const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent);
            const memColorClass = PulseApp.utils.getUsageColor(memoryPercent);

            cpuBarHTML = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
            memoryBarHTML = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);

            if (guest.type === 'CT') {
                const diskPercent = guest.disk;
                const diskTooltipText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
                const diskColorClass = PulseApp.utils.getUsageColor(diskPercent);
                diskBarHTML = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
            } else if (guest.type === 'VM') {
                if (guest.diskTotal) {
                    const totalDiskFormatted = PulseApp.utils.formatBytes(guest.diskTotal);
                    diskBarHTML = `<span class="text-xs text-gray-700 dark:text-gray-200 truncate">${totalDiskFormatted}</span>`;
                } else {
                     diskBarHTML = '-';
                }
            } else {
                 diskBarHTML = '-';
            }

            diskReadFormatted = PulseApp.utils.formatSpeed(guest.diskread, 0);
            diskWriteFormatted = PulseApp.utils.formatSpeed(guest.diskwrite, 0);
            netInFormatted = PulseApp.utils.formatSpeed(guest.netin, 0);
            netOutFormatted = PulseApp.utils.formatSpeed(guest.netout, 0);
          }

          const typeIconClass = guest.type === 'VM'
              ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
              : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
          const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === 'VM' ? 'VM' : 'LXC'}</span>`;

          row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
            <td class="p-1 px-1 text-center">${typeIcon}</td>
            <td class="p-1 px-2 text-center">${guest.id}</td>
            <td class="p-1 px-2 whitespace-nowrap">${guest.status === 'stopped' ? '-' : PulseApp.utils.formatUptime(guest.uptime)}</td>
            <td class="p-1 px-2">${cpuBarHTML}</td>
            <td class="p-1 px-2">${memoryBarHTML}</td>
            <td class="p-1 px-2">${diskBarHTML}</td>
            <td class="p-1 px-2 text-right whitespace-nowrap">${diskReadFormatted}</td>
            <td class="p-1 px-2 text-right whitespace-nowrap">${diskWriteFormatted}</td>
            <td class="p-1 px-2 text-right whitespace-nowrap">${netInFormatted}</td>
            <td class="p-1 px-2 text-right whitespace-nowrap">${netOutFormatted}</td>
          `;

          return row;
      }

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow
    };
})(); 