PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.nodes = (() => {

    function _createNodeCpuBarHtml(node) {
        const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent);
        const cpuTooltipText = `${cpuPercent.toFixed(1)}%${node.maxcpu && node.maxcpu > 0 ? ` (${(node.cpu * node.maxcpu).toFixed(1)}/${node.maxcpu} cores)` : ''}`;
        return PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
    }

    function _createNodeMemoryBarHtml(node) {
        const memUsed = node.mem || 0;
        const memTotal = node.maxmem || 0;
        const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
        const memColorClass = PulseApp.utils.getUsageColor(memPercent);
        const memTooltipText = `${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)} (${memPercent.toFixed(1)}%)`;
        return PulseApp.utils.createProgressTextBarHTML(memPercent, memTooltipText, memColorClass);
    }

    function _createNodeDiskBarHtml(node) {
        const diskUsed = node.disk || 0;
        const diskTotal = node.maxdisk || 0;
        const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0;
        const diskColorClass = PulseApp.utils.getUsageColor(diskPercent);
        const diskTooltipText = `${PulseApp.utils.formatBytes(diskUsed)} / ${PulseApp.utils.formatBytes(diskTotal)} (${diskPercent.toFixed(1)}%)`;
        return PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
    }

    // Create a dedicated function for rendering a single node row
    function createNodeRow(node) {
        const row = document.createElement('tr');
        row.className = 'transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px';

        const isOnline = node && node.uptime > 0;
        const statusText = isOnline ? 'online' : (node.status || 'unknown');
        const statusColor = isOnline
            ? 'bg-green-500 dark:bg-green-400'
            : 'bg-red-500 dark:bg-red-400';

        const cpuBarHTML = _createNodeCpuBarHtml(node);
        const memoryBarHTML = _createNodeMemoryBarHtml(node);
        const diskBarHTML = _createNodeDiskBarHtml(node);

        const uptimeFormatted = PulseApp.utils.formatUptime(node.uptime || 0);
        let normalizedLoadFormatted = 'N/A';
        if (node.loadavg && node.loadavg.length > 0 && node.maxcpu && node.maxcpu > 0) {
            const load1m = parseFloat(node.loadavg[0]);
            if (!isNaN(load1m)) {
                const normalizedLoad = load1m / node.maxcpu;
                normalizedLoadFormatted = normalizedLoad.toFixed(2);
            } else {
                console.warn(`[createNodeRow] Node '${node.node}' has non-numeric loadavg[0]:`, node.loadavg[0]);
            }
        } else if (node.loadavg && node.maxcpu <= 0) {
             console.warn(`[createNodeRow] Node '${node.node}' has invalid maxcpu (${node.maxcpu}) for load normalization.`);
        }

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap">
              <span class="flex items-center">
                <span class="h-2.5 w-2.5 rounded-full ${statusColor} mr-2 flex-shrink-0"></span>
                <span class="capitalize">${statusText}</span>
              </span>
            </td>
            <td class="p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100" title="${node.node || 'N/A'}">${node.node || 'N/A'}</td>
            <td class="p-1 px-2 min-w-[200px]">${cpuBarHTML}</td>
            <td class="p-1 px-2 min-w-[200px]">${memoryBarHTML}</td>
            <td class="p-1 px-2 min-w-[200px]">${diskBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap">${uptimeFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${normalizedLoadFormatted}</td>
        `;
        return row;
    }

    function updateNodesTable(nodes) {
        const tbody = document.getElementById('nodes-table-body');
        if (!tbody) {
            console.error('Critical element #nodes-table-body not found for nodes table update!');
            return;
        }
        // tbody.innerHTML = ''; // Handled by renderTableBody

        const sortStateNodes = PulseApp.state.getSortState('nodes');
        const dataToDisplay = PulseApp.utils.sortData(nodes, sortStateNodes.column, sortStateNodes.direction, 'nodes');

        // if (dataToDisplay.length === 0) { // Handled by renderTableBody
        //     tbody.innerHTML = '<tr><td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found or data unavailable</td></tr>';
        //     return;
        // }

        // Use the utility function
        PulseApp.utils.renderTableBody(tbody, dataToDisplay, createNodeRow, "No nodes found or data unavailable", 7);
        
        // REMOVED: dataToDisplay.forEach(node => { ... tbody.appendChild(row); });
    }

    return {
        updateNodesTable
    };
})();
