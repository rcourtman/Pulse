PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.nodes = (() => {
    let currentNodesData = null; // Store current nodes data for resize handling

    function _createNodeCpuBarHtml(node, includeChart = false) {
        return PulseApp.utils.createMetricBarHtml(node, 'node', 'cpu', includeChart);
    }

    function _createNodeMemoryBarHtml(node, includeChart = false) {
        return PulseApp.utils.createMetricBarHtml(node, 'node', 'memory', includeChart);
    }

    function _createNodeDiskBarHtml(node, includeChart = false) {
        return PulseApp.utils.createMetricBarHtml(node, 'node', 'disk', includeChart);
    }

    // Create a dedicated function for rendering a single node row
    function createNodeRow(node) {
        const row = document.createElement('tr');
        row.className = 'transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px';
        row.setAttribute('data-node-id', node.node); // Add node ID for alert styling

        const isOnline = node && node.uptime > 0;
        const statusText = isOnline ? 'online' : (node.status || 'unknown');
        const statusColor = isOnline
            ? 'bg-green-500 dark:bg-green-400'
            : 'bg-red-500 dark:bg-red-400';

        const isChartsMode = document.getElementById('toggle-charts-checkbox')?.checked || false;
        const mainContainer = document.getElementById('main');
        const chartsEnabled = isChartsMode && mainContainer && mainContainer.classList.contains('charts-mode');
        
        // Create content with charts when in charts mode
        const cpuContent = _createNodeCpuBarHtml(node, chartsEnabled);
        const memoryContent = _createNodeMemoryBarHtml(node, chartsEnabled);
        const diskContent = _createNodeDiskBarHtml(node, chartsEnabled);

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
            <td class="p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 text-gray-900 dark:text-gray-100" title="${node.displayName || node.node || 'N/A'}">${node.displayName || node.node || 'N/A'}</td>
            <td class="p-1 px-2 min-w-[200px]">${cpuContent}</td>
            <td class="p-1 px-2 min-w-[200px]">${memoryContent}</td>
            <td class="p-1 px-2 min-w-[200px]">${diskContent}</td>
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

        // Node summary cards don't show charts - charts are shown in node group rows
        const cpuBarHTML = _createNodeCpuBarHtml(node, false);
        const memoryBarHTML = _createNodeMemoryBarHtml(node, false);
        const diskBarHTML = _createNodeDiskBarHtml(node, false);
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
        // Node summary cards don't participate in alerts mode

        // Check if we can make the node name clickable
        const hostUrl = PulseApp.utils.getHostUrl(node.displayName || node.node);
        let nodeNameContent = node.displayName || node.node || 'N/A';
        
        if (hostUrl) {
            nodeNameContent = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="text-gray-800 dark:text-gray-200 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer" title="Open ${node.displayName || node.node} web interface">${node.displayName || node.node || 'N/A'}</a>`;
        }


        card.innerHTML = `
            <div class="flex justify-between items-center">
                <h3 class="text-sm font-semibold truncate">${nodeNameContent}</h3>
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
        // Node summary cards no longer have alert mode interactions
        
        return card;
    }

    function updateNodeSummaryCards(nodes) {
        const container = document.getElementById('node-summary-cards-container');
        if (!container) {
            console.error('Critical element #node-summary-cards-container not found for node summary cards update!');
            return;
        }
        
        // Store nodes data for resize handling
        if (nodes) {
            currentNodesData = nodes;
        }
        
        // Show loading skeletons if no data yet
        if (!nodes || nodes.length === 0) {
            if (PulseApp.ui.loadingSkeletons) {
                PulseApp.ui.loadingSkeletons.showNodeCardsSkeleton(container, 3);
                return;
            }
            container.innerHTML = '<p class="text-sm text-gray-500 dark:text-gray-400">No node data available for summary.</p>';
            return;
        }
        
        // Find the scrollable container
        const scrollableContainer = PulseApp.utils.getScrollableParent(container) || 
                                   container.closest('.overflow-x-auto') ||
                                   container.parentElement;
        
        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;
        
        // Sort nodes by name - for now using alphabetical since these nodes don't have numbers
        const sortedNodes = [...nodes].sort((a, b) => {
            const aName = a.node || '';
            const bName = b.node || '';
            return aName.localeCompare(bName);
        });
        
        PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
            container.innerHTML = ''; // Clear previous content

        const numNodes = nodes.length;
        const isMobile = window.innerWidth < 640; // sm breakpoint

        if (isMobile) {
            // Stack cards vertically on mobile with condensed layout
            const stackDiv = document.createElement('div');
            stackDiv.className = 'flex flex-col gap-2';

            sortedNodes.forEach(node => {
                const cardElement = createCondensedNodeCard(node);
                stackDiv.appendChild(cardElement);
            });
            container.appendChild(stackDiv);
            
        } else {
            // Use grid layout for desktop
            // Helper function to determine optimal columns to avoid a single orphan
            function calculateOptimalColumns(numItems, defaultCols) {
                if (numItems <= 0) return defaultCols;
                if (defaultCols <= 1) return 1;
                if (numItems <= defaultCols) return numItems;
                
                if (numItems % defaultCols === 1) {
                    if (defaultCols === 2) {
                        return defaultCols;
                    }
                    return Math.max(1, defaultCols - 1);
                }
                return defaultCols;
            }

            const smCols = calculateOptimalColumns(numNodes, 2);
            const mdCols = calculateOptimalColumns(numNodes, 3);
            const lgCols = calculateOptimalColumns(numNodes, 4);
            const xlCols = calculateOptimalColumns(numNodes, 4);
            
            const gridDiv = document.createElement('div');
            gridDiv.className = `grid grid-cols-1 sm:grid-cols-${smCols} md:grid-cols-${mdCols} lg:grid-cols-${lgCols} xl:grid-cols-${xlCols} gap-3`;

            sortedNodes.forEach(node => {
                const cardElement = createNodeSummaryCard(node);
                gridDiv.appendChild(cardElement);
            });
            container.appendChild(gridDiv);
        }
        }); // End of preserveScrollPosition
        
        // Update node charts if in charts mode
        const mainContainer = document.getElementById('main');
        if (PulseApp.charts && mainContainer && mainContainer.classList.contains('charts-mode')) {
            // Use requestAnimationFrame to ensure DOM is fully updated
            requestAnimationFrame(() => {
                updateNodeCharts(sortedNodes);
            });
        }
        
        // Additional scroll position restoration for both axes
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }
    }

    function createCondensedNodeCard(node) {
        const isOnline = node && node.uptime > 0;
        const statusDotColor = isOnline ? 'text-green-500' : 'text-red-500';

        const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
        const memUsed = node.mem || 0;
        const memTotal = node.maxmem || 0;
        const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
        const diskUsed = node.disk || 0;
        const diskTotal = node.maxdisk || 0;
        const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0;

        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-sm rounded-lg p-2 border border-gray-200 dark:border-gray-700';

        // Check if we can make the node name clickable
        const hostUrl = PulseApp.utils.getHostUrl(node.displayName || node.node);
        let nodeNameContent = node.displayName || node.node || 'Unknown';
        
        if (hostUrl) {
            nodeNameContent = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="text-current hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer" title="Open ${node.displayName || node.node} web interface">${node.displayName || node.node || 'Unknown'}</a>`;
        }

        card.innerHTML = `
            <div class="flex items-center justify-between gap-2">
                <div class="flex items-center min-w-0">
                    <span class="h-2 w-2 rounded-full ${statusDotColor} mr-1.5 flex-shrink-0"></span>
                    <h3 class="font-semibold text-xs truncate">${nodeNameContent}</h3>
                </div>
                <div class="flex items-center gap-3 text-[10px] text-gray-600 dark:text-gray-400">
                    <span class="flex items-center gap-1">
                        <span class="font-medium">CPU</span>
                        <span class="font-bold ${PulseApp.utils.getUsageColor(cpuPercent, 'cpu')}">${cpuPercent.toFixed(0)}%</span>
                    </span>
                    <span class="flex items-center gap-1">
                        <span class="font-medium">MEM</span>
                        <span class="font-bold ${PulseApp.utils.getUsageColor(memPercent, 'memory')}">${memPercent.toFixed(0)}%</span>
                    </span>
                    <span class="flex items-center gap-1">
                        <span class="font-medium">DISK</span>
                        <span class="font-bold ${PulseApp.utils.getUsageColor(diskPercent, 'disk')}">${diskPercent.toFixed(0)}%</span>
                    </span>
                </div>
            </div>
        `;
        return card;
    }

    function updateNodesTable(nodes) {
        const tbody = document.getElementById('nodes-table-body');
        if (!tbody) {
            // Node table doesn't exist in current UI - nodes are displayed as summary cards instead
            return;
        }
        
        // Find the scrollable container
        const scrollableContainer = PulseApp.utils.getScrollableParent(tbody) || 
                                   tbody.closest('.overflow-x-auto') ||
                                   tbody.parentElement;
        
        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;
        
        PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
            tbody.innerHTML = '';

        if (!nodes || nodes.length === 0) {
            PulseApp.utils.showEmptyState(tbody, 'No nodes found or data unavailable', 7, 'p-4');
            return;
        }

        // Group nodes by clusterIdentifier
        const clusters = nodes.reduce((acc, node) => {
            const key = node.clusterIdentifier || 'Unknown Cluster';
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
                const endpointType = (nodesInCluster && nodesInCluster.length > 0 && nodesInCluster[0].endpointType) 
                                     ? nodesInCluster[0].endpointType 
                                     : 'standalone';
                
                const iconSvg = endpointType === 'cluster'
                    ? PulseApp.ui.common.NODE_GROUP_CLUSTER_ICON_SVG
                    : PulseApp.ui.common.NODE_GROUP_STANDALONE_ICON_SVG;

                const clusterHeaderRow = document.createElement('tr');
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
        }); // End of preserveScrollPosition
        
        // Update node charts if in charts mode
        const mainContainer = document.getElementById('main');
        if (PulseApp.charts && mainContainer && mainContainer.classList.contains('charts-mode')) {
            // Use requestAnimationFrame to ensure DOM is fully updated
            requestAnimationFrame(() => {
                updateNodeCharts(nodes);
            });
        }
        
        // Additional scroll position restoration for both axes
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }
    }

    let resizeTimeout;
    let resizeHandler;

    function init() {
        // Add resize listener for responsive behavior
        resizeHandler = () => {
            clearTimeout(resizeTimeout);
            resizeTimeout = setTimeout(() => {
                // Re-render cards if we have data
                if (currentNodesData) {
                    updateNodeSummaryCards(currentNodesData);
                }
            }, 250); // Debounce resize events
        };
        window.addEventListener('resize', resizeHandler);
    }

    function cleanup() {
        if (resizeHandler) {
            window.removeEventListener('resize', resizeHandler);
            resizeHandler = null;
        }
        if (resizeTimeout) {
            clearTimeout(resizeTimeout);
            resizeTimeout = null;
        }
    }

    function updateNodeCharts(nodes) {
        if (!nodes || !PulseApp.charts) return;
        
        // Simply trigger the chart render for each node
        // The charts module will handle fetching and rendering the actual data
        nodes.forEach(node => {
            const nodeId = `node-${node.node}`;
            PulseApp.charts.renderNodeCharts(nodeId);
        });
    }

    return {
        init,
        cleanup,
        updateNodesTable,
        updateNodeSummaryCards,
        updateNodeCharts,
        // Expose the bar creation functions for use in dashboard
        _createNodeCpuBarHtml,
        _createNodeMemoryBarHtml,
        _createNodeDiskBarHtml
    };
})();
