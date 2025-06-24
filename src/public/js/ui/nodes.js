PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.nodes = (() => {
    let currentNodesData = null; // Store current nodes data for resize handling

    function _createNodeCpuBarHtml(node, includeChart = false) {
        const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
        const cpuTooltipText = `${cpuPercent.toFixed(1)}%${node.maxcpu && node.maxcpu > 0 ? ` (${(node.cpu * node.maxcpu).toFixed(1)}/${node.maxcpu} cores)` : ''}`;
        const progressBar = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass, `${cpuPercent.toFixed(0)}%`);
        
        if (!includeChart) {
            return progressBar;
        }
        
        // Create both text and chart versions like dashboard does
        const nodeId = `node-${node.node}`;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(nodeId, 'cpu') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createNodeMemoryBarHtml(node, includeChart = false) {
        const memUsed = node.mem || 0;
        const memTotal = node.maxmem || 0;
        const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
        const memColorClass = PulseApp.utils.getUsageColor(memPercent, 'memory');
        const memTooltipText = `${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)} (${memPercent.toFixed(1)}%)`;
        const progressBar = PulseApp.utils.createProgressTextBarHTML(memPercent, memTooltipText, memColorClass, `${memPercent.toFixed(0)}%`);
        
        if (!includeChart) {
            return progressBar;
        }
        
        // Create both text and chart versions like dashboard does
        const nodeId = `node-${node.node}`;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(nodeId, 'memory') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createNodeDiskBarHtml(node, includeChart = false) {
        const diskUsed = node.disk || 0;
        const diskTotal = node.maxdisk || 0;
        const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0;
        const diskColorClass = PulseApp.utils.getUsageColor(diskPercent, 'disk');
        const diskTooltipText = `${PulseApp.utils.formatBytes(diskUsed)} / ${PulseApp.utils.formatBytes(diskTotal)} (${diskPercent.toFixed(1)}%)`;
        const progressBar = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass, `${diskPercent.toFixed(0)}%`);
        
        if (!includeChart) {
            return progressBar;
        }
        
        // Create both text and chart versions like dashboard does
        const nodeId = `node-${node.node}`;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(nodeId, 'disk') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
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
            <td class="p-1 px-2 min-w-[200px]">${cpuBarHTML}</td>
            <td class="p-1 px-2 min-w-[200px]">${memoryBarHTML}</td>
            <td class="p-1 px-2 min-w-[200px]">${diskBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap">${uptimeFormatted}</td>
            <td class="p-1 px-2 whitespace-nowrap">${normalizedLoadFormatted}</td>
        `;
        return row;
    }

    // Helper function to create node alert slider HTML
    function _createNodeAlertSliderHtml(nodeId, metricType, config) {
        // Get current threshold value for this node/metric from alerts system
        const nodeThresholds = PulseApp.ui.alerts?.getNodeThresholds?.() || {};
        const globalNodeThresholds = PulseApp.ui.alerts?.getGlobalNodeThresholds?.() || {};
        
        // Use individual node threshold if set, otherwise use global node threshold, otherwise use config min
        let currentValue = config.min;
        let isUsingGlobal = false;
        
        if (nodeThresholds[nodeId] && nodeThresholds[nodeId][metricType] !== undefined) {
            // Node has individual threshold set
            currentValue = nodeThresholds[nodeId][metricType];
        } else if (globalNodeThresholds[metricType] !== undefined && globalNodeThresholds[metricType] !== '') {
            // Use global node threshold as default
            currentValue = globalNodeThresholds[metricType];
            isUsingGlobal = true;
        } else {
            // Default values for node alerts
            currentValue = metricType === 'cpu' ? 90 : metricType === 'memory' ? 95 : 95; // Higher defaults for nodes
        }
        
        // Use the threshold system's helper function to create identical HTML
        const sliderId = `node-alert-slider-${nodeId}-${metricType}`;
        const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(
            sliderId, 
            config.min, 
            config.max, 
            config.step, 
            currentValue
        );
        
        return sliderHtml;
    }
    
    // Helper function to setup event listeners for node alert sliders
    function _setupNodeAlertEventListeners(container) {
        if (!container) return;
        
        // Setup event listeners for all node alert sliders in this container
        const nodeAlertSliders = container.querySelectorAll('.node-alert-slider input[type="range"]');
        nodeAlertSliders.forEach(slider => {
            const sliderContainer = slider.closest('.node-alert-slider');
            const nodeId = sliderContainer?.getAttribute('data-node-id');
            const metricType = sliderContainer?.getAttribute('data-metric');
            
            if (nodeId && metricType) {
                // Set up alert-specific events (save only on release, not during drag)
                slider.addEventListener('input', (event) => {
                    const value = event.target.value;
                    // Only update tooltip during drag, don't save
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
                
                slider.addEventListener('change', (event) => {
                    const value = event.target.value;
                    // Save on mouse release
                    PulseApp.ui.alerts.updateNodeThreshold(nodeId, metricType, value, true);
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
            }
        });
    }

    function createNodeSummaryCard(node) {
        const isOnline = node && node.uptime > 0;
        const statusText = isOnline ? 'online' : (node.status || 'unknown');
        const statusColor = isOnline ? 'bg-green-500' : 'bg-red-500';
        const statusDotColor = isOnline ? 'text-green-500' : 'text-red-500';

        // Check if charts mode is active
        const isChartsMode = document.getElementById('toggle-charts-checkbox')?.checked || false;
        const mainContainer = document.getElementById('main');
        const chartsEnabled = isChartsMode && mainContainer && mainContainer.classList.contains('charts-mode');
        
        // Check if alerts mode is active
        const isAlertsMode = document.getElementById('toggle-alerts-checkbox')?.checked || false;
        const nodeId = node.node || 'unknown';

        // Check if this node has any custom alert settings
        const nodeThresholds = PulseApp.ui.alerts?.getNodeThresholds?.() || {};
        const hasCustomSettings = nodeThresholds[nodeId] && Object.keys(nodeThresholds[nodeId]).length > 0;
        
        // Apply dimming if no custom settings in alert mode
        const shouldDim = isAlertsMode && !hasCustomSettings;

        // Create progress bars or alert sliders based on mode
        let cpuBarHTML, memoryBarHTML, diskBarHTML;
        
        if (isAlertsMode) {
            // In alert mode, show sliders instead of progress bars
            cpuBarHTML = _createNodeAlertSliderHtml(nodeId, 'cpu', { min: 0, max: 100, step: 5 });
            memoryBarHTML = _createNodeAlertSliderHtml(nodeId, 'memory', { min: 0, max: 100, step: 5 });
            diskBarHTML = _createNodeAlertSliderHtml(nodeId, 'disk', { min: 0, max: 100, step: 5 });
        } else {
            // Normal mode - show progress bars
            cpuBarHTML = _createNodeCpuBarHtml(node, chartsEnabled);
            memoryBarHTML = _createNodeMemoryBarHtml(node, chartsEnabled);
            diskBarHTML = _createNodeDiskBarHtml(node, chartsEnabled);
        }
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
        
        if (isAlertsMode) {
            card.setAttribute('data-node-id', nodeId);
            card.setAttribute('data-alert-dimmed', !hasCustomSettings);
            
            // Apply dimming using inline style for reliability
            if (shouldDim) {
                card.style.opacity = '0.4';
                card.style.transition = 'opacity 0.1s ease-out';
            }
        }

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
            <div class="text-[11px] text-gray-600 dark:text-gray-400 ${isAlertsMode ? 'node-alert-slider' : ''}" ${isAlertsMode ? `data-node-id="${nodeId}" data-metric="cpu"` : ''}>
                <span class="font-medium">CPU:</span>
                ${cpuBarHTML}
            </div>
            <div class="text-[11px] text-gray-600 dark:text-gray-400 ${isAlertsMode ? 'node-alert-slider' : ''}" ${isAlertsMode ? `data-node-id="${nodeId}" data-metric="memory"` : ''}>
                <span class="font-medium">Mem:</span>
                ${memoryBarHTML}
            </div>
            <div class="text-[11px] text-gray-600 dark:text-gray-400 ${isAlertsMode ? 'node-alert-slider' : ''}" ${isAlertsMode ? `data-node-id="${nodeId}" data-metric="disk"` : ''}>
                <span class="font-medium">Disk:</span>
                ${diskBarHTML}
            </div>
            <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400 pt-0.5">
                <span>Uptime: ${uptimeFormatted}</span>
                <span>Load: ${normalizedLoadFormatted}</span>
            </div>
        `;
        
        // Set up event listeners for alert sliders if in alert mode
        if (isAlertsMode) {
            requestAnimationFrame(() => {
                _setupNodeAlertEventListeners(card);
            });
        }
        
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
        
        // Sort nodes by name for consistent order in summary cards
        const sortedNodes = [...nodes].sort((a, b) => (a.node || '').localeCompare(b.node || ''));
        
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
            tbody.innerHTML = '<tr><td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found or data unavailable</td></tr>';
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
        updateNodeCharts
    };
})();
