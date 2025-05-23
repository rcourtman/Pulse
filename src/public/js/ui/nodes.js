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

    function createNodeSummaryCard(node) {
        const isOnline = node && node.uptime > 0;
        const statusText = isOnline ? 'online' : (node.status || 'unknown');
        const statusColor = isOnline ? 'bg-green-500' : 'bg-red-500';
        const statusDotColor = isOnline ? 'text-green-500' : 'text-red-500';

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
            }
        }

        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1';

        card.innerHTML = `
            <div class="flex justify-between items-center">
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate" title="${node.node || 'N/A'}">${node.node || 'N/A'}</h3>
                <div class="flex items-center">
                    <span class="h-2.5 w-2.5 rounded-full ${statusColor} mr-1.5 flex-shrink-0"></span>
                    <span class="text-xs capitalize text-gray-600 dark:text-gray-400">${statusText}</span>
                </div>
            </div>
            <div class="text-[11px] text-gray-600 dark:text-gray-400">
                <span class="font-medium">CPU:</span>
                ${cpuBarHTML}
            </div>
            <div class="text-[11px] text-gray-600 dark:text-gray-400">
                <span class="font-medium">Mem:</span>
                ${memoryBarHTML}
            </div>
            <div class="text-[11px] text-gray-600 dark:text-gray-400">
                <span class="font-medium">Disk:</span>
                ${diskBarHTML}
            </div>
            <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400 pt-0.5">
                <span>Uptime: ${uptimeFormatted}</span>
                <span>Load: ${normalizedLoadFormatted}</span>
            </div>
        `;
        return card;
    }

    function updateNodeSummaryCards(nodes) {
        const container = document.getElementById('node-summary-cards-container');
        if (!container) {
            console.error('Critical element #node-summary-cards-container not found for node summary cards update!');
            return;
        }
        container.innerHTML = ''; // Clear previous content

        if (!nodes || nodes.length === 0) {
            container.innerHTML = '<p class="text-sm text-gray-500 dark:text-gray-400">No node data available for summary.</p>';
            return;
        }

        const numNodes = nodes.length;

        // Helper function to determine optimal columns to avoid a single orphan
        function calculateOptimalColumns(numItems, defaultCols) {
            if (numItems <= 0) return defaultCols; // No items, use default or let it be empty
            if (defaultCols <= 1) return 1; // Cannot reduce further, or already 1
            // If numItems is less than or equal to defaultCols, use numItems as the column count.
            if (numItems <= defaultCols) return numItems; 
            
            // Now numItems > defaultCols (multiple rows expected)
            // Avoid a single orphan, but don't reduce to 1 column if defaultCols is 2.
            if (numItems % defaultCols === 1) {
                if (defaultCols === 2) {
                    // If default is 2 cols (sm), and we have an odd number of items (e.g., 3, 5),
                    // use 2 columns to avoid stacking. (Results in 2 side-by-side, then 1 or more).
                    return defaultCols; 
                }
                // For other defaultCols (>=3), reducing by 1 to avoid an orphan is fine.
                return Math.max(1, defaultCols - 1); // Ensure at least 1 column
            }
            return defaultCols;
        }

        const smCols = calculateOptimalColumns(numNodes, 2);
        const mdCols = calculateOptimalColumns(numNodes, 3);
        const lgCols = calculateOptimalColumns(numNodes, 4);
        const xlCols = calculateOptimalColumns(numNodes, 4); // Based on previous preference
        
        const gridDiv = document.createElement('div');
        gridDiv.className = `grid grid-cols-1 sm:grid-cols-${smCols} md:grid-cols-${mdCols} lg:grid-cols-${lgCols} xl:grid-cols-${xlCols} gap-3`;

        // Sort nodes by name for consistent order in summary cards
        const sortedNodes = [...nodes].sort((a, b) => (a.node || '').localeCompare(b.node || ''));

        sortedNodes.forEach(node => {
            const cardElement = createNodeSummaryCard(node);
            gridDiv.appendChild(cardElement);
        });
        container.appendChild(gridDiv);
    }

    function updateNodesTable(nodes) {
        const tbody = document.getElementById('nodes-table-body');
        if (!tbody) {
            console.error('Critical element #nodes-table-body not found for nodes table update!');
            return;
        }
        tbody.innerHTML = ''; // Clear previous content

        if (!nodes || nodes.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found or data unavailable</td></tr>';
            return;
        }

        // Group nodes by clusterIdentifier
        const clusters = nodes.reduce((acc, node) => {
            const key = node.clusterIdentifier || 'Unknown Cluster'; // Fallback for safety
            if (!acc[key]) {
                acc[key] = [];
            }
            acc[key].push(node);
            return acc;
        }, {});

        const sortStateNodes = PulseApp.state.getSortState('nodes');

        // Iterate over each cluster group
        for (const clusterIdentifier in clusters) {
            if (clusters.hasOwnProperty(clusterIdentifier)) {
                const nodesInCluster = clusters[clusterIdentifier];
                // All nodes in this group should have the same endpointType, so pick from the first.
                // Default to 'standalone' if nodesInCluster is empty or endpointType is missing for some reason.
                const endpointType = (nodesInCluster && nodesInCluster.length > 0 && nodesInCluster[0].endpointType) 
                                     ? nodesInCluster[0].endpointType 
                                     : 'standalone';
                
                const iconSvg = endpointType === 'cluster'
                    ? PulseApp.ui.common.NODE_GROUP_CLUSTER_ICON_SVG
                    : PulseApp.ui.common.NODE_GROUP_STANDALONE_ICON_SVG;

                const clusterHeaderRow = document.createElement('tr');
                // Applying base background, then overlaying with stripe pattern classes
                clusterHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(clusterIdentifier, 7, 'th');
                tbody.appendChild(clusterHeaderRow);

                // Sort nodes within this cluster group
                const sortedNodesInCluster = PulseApp.utils.sortData(nodesInCluster, sortStateNodes.column, sortStateNodes.direction, 'nodes');

                sortedNodesInCluster.forEach(node => {
                    const nodeRow = createNodeRow(node);
                    tbody.appendChild(nodeRow);
                });
            }
        }
    }

    return {
        updateNodesTable,
        updateNodeSummaryCards
    };
})();
