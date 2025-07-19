PulseApp.utils = (() => {
    // Debounce function to limit function calls
    function debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    function getUsageColor(percentage, metric = 'generic') {
        // Progress bars use traditional green/yellow/red with metric-specific thresholds
        if (metric === 'cpu') {
            // CPU: show color for significant usage
            if (percentage >= 90) return 'red';
            if (percentage >= 80) return 'yellow';
            return 'green'; // Healthy green for normal CPU usage
        } else if (metric === 'memory') {
            // Memory: be more conservative due to critical nature
            if (percentage >= 85) return 'red';
            if (percentage >= 75) return 'yellow';
            return 'green'; // Healthy green for normal memory usage
        } else if (metric === 'disk') {
            // Disk: can run higher before concerning
            if (percentage >= 90) return 'red';
            if (percentage >= 80) return 'yellow';
            return 'green'; // Healthy green for normal disk usage
        } else {
            if (percentage >= 90) return 'red';
            if (percentage >= 75) return 'yellow';
            return 'green'; // Keep green for non-dashboard usage
        }
    }

    function createProgressTextBarHTML(percentage, text, color, simpleText = null) {
        // Always use a neutral background regardless of the progress color
        const bgColorClass = 'bg-gray-200 dark:bg-gray-600';

        const progressColorClass = {
            red: 'bg-red-500/60 dark:bg-red-500/50',
            yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
            green: 'bg-green-500/60 dark:bg-green-500/50'
        }[color] || 'bg-gray-500/60 dark:bg-gray-500/50'; // Fallback progress color with opacity

        return `
            <div class="relative w-full h-3.5 rounded overflow-hidden ${bgColorClass}">
                <div class="absolute top-0 left-0 h-full ${progressColorClass}" style="width: ${percentage}%;"></div>
                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                    <span class="truncate px-1">${text}</span>
                </span>
            </div>
        `;
    }
    
    function getBackupTimestamp(backup) {
        return backup.timestamp || backup['backup-time'] || backup.ctime || backup.snaptime;
    }
    
    function toDateKey(date) {
        if (!date) return null;
        const d = date instanceof Date ? date : new Date(date * 1000);
        return d.toISOString().split('T')[0];
    }

    function formatBytes(bytes, decimals = 1, k = 1024, compact = false) {
        if (bytes === 0 || bytes === null || bytes === undefined) return compact ? '0B' : '0 B';
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = compact 
            ? ['B', 'K', 'M', 'G', 'T', 'P', 'E', 'Z', 'Y']
            : ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        const value = parseFloat((bytes / Math.pow(k, i)).toFixed(dm));
        return compact ? `${value}${sizes[i]}` : `${value} ${sizes[i]}`;
    }

    function formatBytesCompact(bytes, decimals = 1, k = 1024) {
        return formatBytes(bytes, decimals, k, true);
    }

    function formatSpeed(bytesPerSecond, decimals = 1) {
        if (bytesPerSecond === null || bytesPerSecond === undefined) return 'N/A';
        if (bytesPerSecond < 1) return '0 B/s';
        return formatBytes(bytesPerSecond, decimals) + '/s';
    }

    function formatSpeedWithStyling(bytesPerSecond, decimals = 1) {
        if (bytesPerSecond === null || bytesPerSecond === undefined) return 'N/A';
        
        let formattedSpeed;
        if (bytesPerSecond < 1) {
            formattedSpeed = '0 B/s';
        } else {
            formattedSpeed = formatBytes(bytesPerSecond, decimals) + '/s';
        }
        
        // Use same absolute thresholds as chart logic
        const mbps = bytesPerSecond / (1024 * 1024);
        const isDarkMode = document.documentElement.classList.contains('dark');
        
        let textClass = '';
        if (mbps < 1) {
            // Not noteworthy - use theme-adaptive dim gray
            textClass = isDarkMode ? 'text-gray-400' : 'text-gray-300';
        } else {
            // Noteworthy - use normal text color
            textClass = 'text-gray-800 dark:text-gray-200';
        }
        
        return `<span class="${textClass}">${formattedSpeed}</span>`;
    }

    function formatUptime(seconds) {
        if (seconds === null || seconds === undefined || seconds < 0) return 'N/A';
        if (seconds < 60) return `${Math.floor(seconds)}s`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
        const days = Math.floor(seconds / 86400);
        return `${days}d`;
    }

    function formatDuration(seconds) {
        if (seconds === null || seconds === undefined || seconds < 0) return 'N/A';
        if (seconds < 1) return `< 1s`;
        if (seconds < 60) return `${Math.round(seconds)}s`;
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = Math.round(seconds % 60);
        if (minutes < 60) {
            return `${minutes}m ${remainingSeconds}s`;
        }
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        return `${hours}h ${remainingMinutes}m`;
    }

    function formatPbsTimestamp(timestamp) {
        if (!timestamp) return 'N/A';
        try {
            const date = new Date(timestamp * 1000);
            const now = new Date();
            const isToday = date.toDateString() === now.toDateString();
            const timeOptions = { hour: '2-digit', minute: '2-digit' };
            const dateOptions = { month: 'short', day: 'numeric' };

            if (isToday) {
                return `Today ${date.toLocaleTimeString([], timeOptions)}`;
            } else {
                return `${date.toLocaleDateString([], dateOptions)} ${date.toLocaleTimeString([], timeOptions)}`;
            }
        } catch (e) {
            console.error("Error formatting PBS timestamp:", timestamp, e);
            return 'Invalid Date';
        }
    }

    function formatPbsTimestampRelative(timestamp) {
        if (!timestamp) return 'N/A';
        try {
            const date = new Date(timestamp * 1000);
            const now = new Date();
            const diffMs = now - date;
            const diffSeconds = Math.floor(diffMs / 1000);
            const diffMinutes = Math.floor(diffSeconds / 60);
            const diffHours = Math.floor(diffMinutes / 60);
            const diffDays = Math.floor(diffHours / 24);

            if (diffSeconds < 60) {
                return 'just now';
            } else if (diffMinutes < 60) {
                return `${diffMinutes} minute${diffMinutes !== 1 ? 's' : ''} ago`;
            } else if (diffHours < 24) {
                return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`;
            } else if (diffDays < 7) {
                return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`;
            } else if (diffDays < 30) {
                const diffWeeks = Math.floor(diffDays / 7);
                return `${diffWeeks} week${diffWeeks !== 1 ? 's' : ''} ago`;
            } else if (diffDays < 365) {
                const diffMonths = Math.floor(diffDays / 30);
                return `${diffMonths} month${diffMonths !== 1 ? 's' : ''} ago`;
            } else {
                const diffYears = Math.floor(diffDays / 365);
                return `${diffYears} year${diffYears !== 1 ? 's' : ''} ago`;
            }
        } catch (e) {
            console.error("Error formatting PBS timestamp:", timestamp, e);
            return 'Invalid Date';
        }
    }




    function sortData(data, column, direction, tableType = 'main') {
        if (!column) return data;

        const sortStates = PulseApp.state.getSortState(tableType);
        const effectiveDirection = direction || sortStates.direction;

        return [...data].sort((a, b) => {
            // Map 'id' column to 'vmid' property for guest table
            const propertyName = column === 'id' ? 'vmid' : column;
            
            let valA = a[propertyName];
            let valB = b[propertyName];

            if (column === 'id' || column === 'vmid' || column === 'guestId') {
                valA = parseInt(valA, 10);
                valB = parseInt(valB, 10);
            }
             else if (column === 'name' || column === 'node' || column === 'guestName' || column === 'guestType' || column === 'pbsInstanceName' || column === 'datastoreName') {
                
                valA = String(valA || '').toLowerCase();
                valB = String(valB || '').toLowerCase();
            }
            else if (['cpu', 'memory', 'disk', 'maxcpu', 'maxmem', 'maxdisk', 'uptime', 'loadavg', 'loadnorm', 'totalBackups'].includes(column)) {
                valA = parseFloat(valA || 0);
                valB = parseFloat(valB || 0);
            }
            else if (['diskread', 'diskwrite', 'netin', 'netout'].includes(column)) {
                valA = parseFloat(valA || 0);
                valB = parseFloat(valB || 0);
            }
             else if (column === 'latestBackupTime') {
                valA = parseInt(valA || 0, 10);
                valB = parseInt(valB || 0, 10);
            }
            else if (column === 'backupHealthStatus') {
                 const healthOrder = { 'failed': 0, 'old': 1, 'stale': 2, 'ok': 3, 'none': 4 };
                 valA = healthOrder[valA] ?? 99;
                 valB = healthOrder[valB] ?? 99;
            }

            let comparison = 0;
            if (valA < valB) {
                comparison = -1;
            } else if (valA > valB) {
                comparison = 1;
            }

            return effectiveDirection === 'desc' ? (comparison * -1) : comparison;
        });
    }

    function renderTableBody(tbodyElement, data, rowRendererFn, noDataMessage = 'No data available', colspan = 100) {
        if (!tbodyElement) {
            console.error('Table body element not provided for rendering!');
            return;
        }
        tbodyElement.innerHTML = ''; // Clear existing content

        if (!data || data.length === 0) {
            tbodyElement.innerHTML = `<tr><td colspan="${colspan}" class="p-4 text-center text-gray-500 dark:text-gray-400">${noDataMessage}</td></tr>`;
            return;
        }

        data.forEach(item => {
            const row = rowRendererFn(item); // Call the specific renderer for this table type
            if (row instanceof HTMLElement) {
                tbodyElement.appendChild(row);
            } else {
                 // Only log warning if rowRendererFn didn't explicitly return null/undefined
                 if (row !== null && row !== undefined) {
                    console.warn('Row renderer function did not return a valid HTML element for item:', item);
                 }
            }
        });
    }

    // Function to check and update progress bar text based on available width
    // DISABLED: Progress bars now always show full text
    function updateProgressBarTexts() {
        // No-op - functionality disabled
    }
    
    // Debounced version for resize events

    // Scroll position preservation for table updates
    function preserveScrollPosition(element, updateFn) {
        if (!element) {
            updateFn();
            return;
        }

        // Save current scroll positions
        const scrollLeft = element.scrollLeft;
        const scrollTop = element.scrollTop;

        // Execute the update function
        updateFn();

        // Restore scroll positions after DOM updates are complete
        requestAnimationFrame(() => {
            element.scrollLeft = scrollLeft;
            element.scrollTop = scrollTop;
        });
    }

    // Get the scrollable parent of a table
    function getScrollableParent(element) {
        if (!element) return null;
        
        let parent = element.parentElement;
        while (parent) {
            const style = window.getComputedStyle(parent);
            if (style.overflowX === 'auto' || style.overflowX === 'scroll' ||
                style.overflowY === 'auto' || style.overflowY === 'scroll') {
                return parent;
            }
            parent = parent.parentElement;
        }
        return null;
    }

    // Get URL for a host based on endpoint configuration and API data
    function getHostUrl(nodeName) {
        const endpoints = PulseApp.state.get('endpoints') || [];
        const pbsConfigs = PulseApp.state.get('pbsConfigs') || [];
        const nodesData = PulseApp.state.get('nodesData') || [];
        
        // Helper function to construct the complete URL
        function constructUrl(host, port, defaultPort = '8006') {
            if (!host) return null;
            
            // If host already includes protocol, return as-is
            if (host.startsWith('http://') || host.startsWith('https://')) {
                return host;
            }
            
            // Use the provided port or default
            const finalPort = port || defaultPort;
            return `https://${host}:${finalPort}`;
        }
        
        // First check PBS configs for exact name match
        for (const config of pbsConfigs) {
            if (config.name === nodeName) {
                return constructUrl(config.host, config.port, '8007'); // PBS default port
            }
        }
        
        // For Proxmox nodes, we need to find which endpoint this node belongs to
        // by looking at the nodes data from the API
        
        // This needs to be checked first because the UI shows displayName in headers
        const nodeByDisplayName = nodesData.find(node => node.displayName === nodeName);
        if (nodeByDisplayName && nodeByDisplayName.endpointId) {
            const endpoint = endpoints.find(ep => ep.id === nodeByDisplayName.endpointId);
            if (endpoint) {
                return constructUrl(endpoint.host, endpoint.port);
            }
        }
        
        // Then find the node in the API data by actual node name
        const nodeInfo = nodesData.find(node => node.node === nodeName);
        
        if (nodeInfo && nodeInfo.endpointId) {
            // Find the endpoint that matches this endpointId
            const endpoint = endpoints.find(ep => ep.id === nodeInfo.endpointId);
            if (endpoint) {
                return constructUrl(endpoint.host, endpoint.port);
            }
        }
        
        // Fallback: try direct name matches with endpoints
        for (const endpoint of endpoints) {
            if (endpoint.name === nodeName) {
                return constructUrl(endpoint.host, endpoint.port);
            }
        }
        
        // Additional fallback: for standalone nodes, check if the endpoint name matches
        // and there's only one node for that endpoint
        for (const endpoint of endpoints) {
            const endpointNodes = nodesData.filter(node => node.endpointId === endpoint.id);
            if (endpointNodes.length === 1 && endpoint.name === nodeName) {
                return constructUrl(endpoint.host, endpoint.port);
            }
        }
        
        return null;
    }

    // Button state management utilities
    function setButtonLoading(button, loadingText = 'Loading...') {
        if (!button) return null;
        
        const originalState = {
            text: button.textContent,
            disabled: button.disabled,
            classList: button.className
        };
        
        button.textContent = loadingText;
        button.disabled = true;
        button.classList.add('opacity-50', 'cursor-not-allowed');
        
        return originalState;
    }

    function resetButton(button, originalState) {
        if (!button || !originalState) return;
        
        button.textContent = originalState.text;
        button.disabled = originalState.disabled;
        button.className = originalState.classList;
    }

    // Date formatting utilities
    function formatDate(date) {
        if (!date) return '';
        const d = date instanceof Date ? date : new Date(date);
        return d.toLocaleDateString();
    }

    function formatDateTime(date) {
        if (!date) return '';
        const d = date instanceof Date ? date : new Date(date);
        return d.toLocaleString();
    }


    // Format time ago for backup age display
    function formatTimeAgo(daysAgo) {
        if (daysAgo === 0) return 'Today';
        if (daysAgo === 1) return 'Yesterday';
        if (daysAgo < 7) return `${Math.floor(daysAgo)} days ago`;
        if (daysAgo < 30) return `${Math.floor(daysAgo / 7)} weeks ago`;
        if (daysAgo < 365) return `${Math.floor(daysAgo / 30)} months ago`;
        return `${Math.floor(daysAgo / 365)} years ago`;
    }

    // Initialize mobile scroll indicators for tables
    function initMobileScrollIndicators(containerSelector) {
        const containers = containerSelector instanceof HTMLElement 
            ? [containerSelector] 
            : document.querySelectorAll(containerSelector);
        
        containers.forEach(container => {
            if (!container) return;
            
            // Add touch indicator on mobile
            const scrollHint = document.createElement('div');
            scrollHint.className = 'scroll-hint';
            scrollHint.innerHTML = '← Scroll for more →';
            scrollHint.style.cssText = `
                position: absolute;
                bottom: 10px;
                left: 50%;
                transform: translateX(-50%);
                background: rgba(0, 0, 0, 0.7);
                color: white;
                padding: 4px 12px;
                border-radius: 4px;
                font-size: 12px;
                pointer-events: none;
                transition: opacity 0.3s;
                z-index: 10;
            `;
            container.style.position = 'relative';
            container.appendChild(scrollHint);

            // Hide on scroll or touch
            let hideTimeout;
            const hideHint = () => {
                scrollHint.style.opacity = '0';
                clearTimeout(hideTimeout);
            };

            container.addEventListener('scroll', hideHint);
            container.addEventListener('touchstart', hideHint);

            // Auto-hide after 5 seconds
            hideTimeout = setTimeout(hideHint, 5000);
        });
    }

    // Get color for backup status
    function getBackupStatusColor(status) {
        const statusColorMap = {
            'ok': 'text-green-600 dark:text-green-500',
            'stale': 'text-yellow-600 dark:text-yellow-500',
            'old': 'text-orange-600 dark:text-orange-500',
            'failed': 'text-red-600 dark:text-red-500',
            'none': 'text-gray-400 dark:text-gray-500'
        };
        return statusColorMap[status] || 'text-gray-400 dark:text-gray-500';
    }

    // Create health badge HTML
    function createHealthBadgeHTML(status, tooltip = null) {
        const colorClass = getBackupStatusColor(status);
        const tooltipAttr = tooltip ? `title="${tooltip}"` : '';
        const statusDisplay = {
            'ok': 'OK',
            'stale': 'Stale',
            'old': 'Old',
            'failed': 'Failed',
            'none': 'None'
        }[status] || status.toUpperCase();
        
        return `<span class="px-2 py-0.5 text-xs font-medium rounded-full bg-gray-100 dark:bg-gray-700 ${colorClass}" ${tooltipAttr}>${statusDisplay}</span>`;
    }

    // Get readable threshold name for display
    function getReadableThresholdName(type) {
        const names = {
            'cpu': 'CPU',
            'memory': 'Memory', 
            'disk': 'Disk',
            'diskread': 'Disk Read',
            'diskwrite': 'Disk Write',
            'netin': 'Net In',
            'netout': 'Net Out'
        };
        return names[type] || type;
    }

    // Format threshold value with appropriate units
    function formatThresholdValue(type, value) {
        if (['cpu', 'memory', 'disk'].includes(type)) {
            return `${value}%`;
        } else {
            const mb = value / (1024 * 1024);
            if (mb >= 100) return `${Math.round(mb)}MB/s`;
            if (mb >= 10) return `${Math.round(mb)}MB/s`;
            return `${Math.round(mb * 10) / 10}MB/s`;
        }
    }
    
    // Unified metric display helpers
    function createMetricBarHtml(entity, entityType, metricType, includeChart = false) {
        // Get metric value and configuration based on type
        let value, maxValue, tooltipExtra, displayValue;
        
        switch (metricType) {
            case 'cpu':
                value = entity.cpu ? (entity.cpu * 100) : 0;
                maxValue = entity.maxcpu;
                tooltipExtra = maxValue && maxValue > 0 ? ` (${(entity.cpu * maxValue).toFixed(1)}/${maxValue} cores)` : '';
                displayValue = `${value.toFixed(0)}%`;
                break;
            case 'memory':
                const memUsage = entity.mem && entity.maxmem ? (entity.mem / entity.maxmem * 100) : 0;
                value = memUsage;
                tooltipExtra = entity.mem && entity.maxmem ? ` (${formatBytes(entity.mem)}/${formatBytes(entity.maxmem)})` : '';
                displayValue = `${value.toFixed(0)}%`;
                break;
            case 'disk':
                const diskUsage = entity.disk && entity.maxdisk ? (entity.disk / entity.maxdisk * 100) : 0;
                value = diskUsage;
                tooltipExtra = entity.disk && entity.maxdisk ? ` (${formatBytes(entity.disk)}/${formatBytes(entity.maxdisk)})` : '';
                displayValue = `${value.toFixed(0)}%`;
                break;
            default:
                value = 0;
                tooltipExtra = '';
                displayValue = '0%';
        }
        
        const colorClass = getUsageColor(value, metricType);
        const tooltipText = `${value.toFixed(1)}%${tooltipExtra}`;
        const progressBar = createProgressTextBarHTML(value, tooltipText, colorClass, displayValue);
        
        if (!includeChart) {
            return progressBar;
        }
        
        // Create both text and chart versions
        const entityId = entityType === 'node' ? `node-${entity.node}` : entity.vmid || entity.id;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(entityId, metricType) : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }
    
    // Alert threshold control helpers
    function createAlertSliderHtml(entityId, entityType, metricType, config) {
        // entityType can be 'guest', 'node', etc.
        const thresholds = {
            guest: PulseApp.ui.alerts?.getGuestThresholds?.() || {},
            node: PulseApp.ui.alerts?.getNodeThresholds?.() || {},
            global: PulseApp.ui.alerts?.getGlobalThresholds?.() || {}
        };
        
        // Determine current value based on entity type
        let currentValue = config.min;
        let isUsingGlobal = false;
        
        if (entityType === 'guest' && thresholds.guest[entityId] && thresholds.guest[entityId][metricType] !== undefined) {
            currentValue = thresholds.guest[entityId][metricType];
        } else if (entityType === 'node' && thresholds.node[entityId] && thresholds.node[entityId][metricType] !== undefined) {
            currentValue = thresholds.node[entityId][metricType];
        } else if (thresholds.global[metricType] !== undefined && thresholds.global[metricType] !== '') {
            currentValue = thresholds.global[metricType];
            isUsingGlobal = true;
        }
        
        const sliderId = `alert-slider-${entityId}-${metricType}`;
        const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(
            sliderId, 
            config.min, 
            config.max, 
            config.step, 
            currentValue
        );
        
        const containerClass = isUsingGlobal ? 'alert-threshold-input using-global' : 'alert-threshold-input';
        const dataAttrs = `data-${entityType}-id="${entityId}" data-metric="${metricType}"`;
        
        return `
            <div class="${containerClass} flex items-center" ${dataAttrs}>
                ${sliderHtml}
            </div>
        `;
    }
    
    function createAlertDropdownHtml(entityId, entityType, metricType, options) {
        const thresholds = {
            guest: PulseApp.ui.alerts?.getGuestThresholds?.() || {},
            node: PulseApp.ui.alerts?.getNodeThresholds?.() || {},
            global: PulseApp.ui.alerts?.getGlobalThresholds?.() || {}
        };
        
        let currentValue = '';
        let isUsingGlobal = false;
        
        if (entityType === 'guest' && thresholds.guest[entityId] && thresholds.guest[entityId][metricType] !== undefined) {
            currentValue = thresholds.guest[entityId][metricType];
        } else if (entityType === 'node' && thresholds.node[entityId] && thresholds.node[entityId][metricType] !== undefined) {
            currentValue = thresholds.node[entityId][metricType];
        } else if (thresholds.global[metricType] !== undefined) {
            currentValue = thresholds.global[metricType];
            isUsingGlobal = true;
        }
        
        
        const selectId = `alert-select-${entityId}-${metricType}`;
        const selectHtml = PulseApp.ui.thresholds.createThresholdSelectHtml(
            selectId, 
            options, 
            currentValue.toString()  // Ensure it's a string for comparison
        );
        
        return selectHtml;
    }
    
    // Helper for showing empty state messages in tables
    function showEmptyState(container, message, columns = 1, additionalClasses = '') {
        const html = `
            <tr>
                <td colspan="${columns}" class="text-center py-8 text-gray-500 dark:text-gray-400 ${additionalClasses}">
                    ${message}
                </td>
            </tr>
        `;
        
        if (container instanceof HTMLElement) {
            container.innerHTML = html;
        }
        return html;
    }
    
    // Helper for updating guest I/O metric cells
    function updateGuestIOMetric(cell, guest, metricName, isAlertsMode) {
        if (isAlertsMode) {
            const existingSelect = cell.querySelector('select');
            if (!existingSelect) {
                cell.innerHTML = createAlertDropdownHtml(guest.id, 'guest', metricName, IO_ALERT_OPTIONS);
                // Alert event listeners should be set up by the caller
            }
        } else {
            const formatted = formatSpeedWithStyling(guest[metricName], 0);
            const chartContainer = cell.querySelector(`#chart-${guest.id}-${metricName}`);
            const metricText = cell.querySelector('.metric-text');
            
            if (chartContainer && metricText) {
                metricText.innerHTML = formatted;
            } else {
                cell.innerHTML = PulseApp.charts ? 
                    `<div class="metric-text">${formatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.id, metricName)}</div>` :
                    formatted;
            }
        }
    }
    
    // DOM Cache utility class
    class DOMCache {
        constructor() {
            this.cache = new Map();
            this.observers = new Map();
        }

        get(selector, forceRefresh = false) {
            if (!forceRefresh && this.cache.has(selector)) {
                const element = this.cache.get(selector);
                if (document.contains(element)) {
                    return element;
                }
                this.cache.delete(selector);
            }

            const element = document.querySelector(selector);
            if (element) {
                this.cache.set(selector, element);
            }
            return element;
        }

        getAll(selector, forceRefresh = false) {
            const key = `all:${selector}`;
            if (!forceRefresh && this.cache.has(key)) {
                const elements = this.cache.get(key);
                if (elements.length > 0 && document.contains(elements[0])) {
                    return elements;
                }
                this.cache.delete(key);
            }

            const elements = Array.from(document.querySelectorAll(selector));
            this.cache.set(key, elements);
            return elements;
        }

        set(key, element) {
            if (element && document.contains(element)) {
                this.cache.set(key, element);
            }
        }

        clear(selector = null) {
            if (selector) {
                this.cache.delete(selector);
                this.cache.delete(`all:${selector}`);
            } else {
                this.cache.clear();
            }
        }

        invalidate() {
            // Remove entries for elements no longer in DOM
            for (const [key, value] of this.cache.entries()) {
                if (key.startsWith('all:')) {
                    if (!value.length || !document.contains(value[0])) {
                        this.cache.delete(key);
                    }
                } else if (!document.contains(value)) {
                    this.cache.delete(key);
                }
            }
        }
    }
    
    
    // Storage helper for localStorage/sessionStorage
    const StorageHelper = {
        // localStorage methods
        get(key, defaultValue = null) {
            try {
                const item = localStorage.getItem(key);
                return item ? JSON.parse(item) : defaultValue;
            } catch (e) {
                console.error(`Error reading ${key} from localStorage:`, e);
                return defaultValue;
            }
        },
        
        set(key, value) {
            try {
                localStorage.setItem(key, JSON.stringify(value));
                return true;
            } catch (e) {
                console.error(`Error writing ${key} to localStorage:`, e);
                return false;
            }
        },
        
        remove(key) {
            try {
                localStorage.removeItem(key);
                return true;
            } catch (e) {
                console.error(`Error removing ${key} from localStorage:`, e);
                return false;
            }
        },
        
        clear() {
            try {
                localStorage.clear();
                return true;
            } catch (e) {
                console.error('Error clearing localStorage:', e);
                return false;
            }
        },
        
        // sessionStorage methods
        session: {
            get(key, defaultValue = null) {
                try {
                    const item = sessionStorage.getItem(key);
                    return item ? JSON.parse(item) : defaultValue;
                } catch (e) {
                    console.error(`Error reading ${key} from sessionStorage:`, e);
                    return defaultValue;
                }
            },
            
            set(key, value) {
                try {
                    sessionStorage.setItem(key, JSON.stringify(value));
                    return true;
                } catch (e) {
                    console.error(`Error writing ${key} to sessionStorage:`, e);
                    return false;
                }
            },
            
            remove(key) {
                try {
                    sessionStorage.removeItem(key);
                    return true;
                } catch (e) {
                    console.error(`Error removing ${key} from sessionStorage:`, e);
                    return false;
                }
            },
            
            clear() {
                try {
                    sessionStorage.clear();
                    return true;
                } catch (e) {
                    console.error('Error clearing sessionStorage:', e);
                    return false;
                }
            }
        }
    };
    
    // Simple logger utility
    const createLogger = (module) => ({
        log: (message, ...args) => console.log(`[${module}] ${message}`, ...args),
        warn: (message, ...args) => console.warn(`[${module}] ${message}`, ...args),
        error: (message, ...args) => console.error(`[${module}] ${message}`, ...args),
        debug: (message, ...args) => console.debug(`[${module}] ${message}`, ...args)
    });
    
    
    // Common constants for alert thresholds
    const IO_ALERT_OPTIONS = [
        { value: '', label: 'No alert' },
        { value: '1048576', label: '> 1 MB/s' },
        { value: '10485760', label: '> 10 MB/s' },
        { value: '52428800', label: '> 50 MB/s' },
        { value: '104857600', label: '> 100 MB/s' }
    ];
    
    // Common CSS class constants for consistency
    const CSS_CLASSES = {
        // Text colors with dark mode variants
        TEXT_GRAY_500_DARK_400: 'text-gray-500 dark:text-gray-400',
        TEXT_GRAY_600_DARK_400: 'text-gray-600 dark:text-gray-400',
        TEXT_GRAY_700_DARK_300: 'text-gray-700 dark:text-gray-300',
        TEXT_RED_600_DARK_400: 'text-red-600 dark:text-red-400',
        TEXT_GREEN_500_DARK_400: 'text-green-500 dark:text-green-400',
        TEXT_GREEN_600_DARK_500: 'text-green-600 dark:text-green-500',
        TEXT_BLUE_600_DARK_400: 'text-blue-600 dark:text-blue-400',
        TEXT_YELLOW_600_DARK_400: 'text-yellow-600 dark:text-yellow-400',
        TEXT_YELLOW_600_DARK_500: 'text-yellow-600 dark:text-yellow-500',
        TEXT_ORANGE_600_DARK_500: 'text-orange-600 dark:text-orange-500',
        
        // Background colors
        BG_GRAY_50_DARK_700_50: 'bg-gray-50 dark:bg-gray-700/50',
        BG_GRAY_100_DARK_700: 'bg-gray-100 dark:bg-gray-700',
        
        // Component presets
        TABLE_HEADER: 'px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider',
        SECTION_HEADER: 'font-medium text-gray-700 dark:text-gray-300 mb-1',
        SUMMARY_CARD: 'p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-center border border-gray-200 dark:border-gray-600',
        
        // Status indicators
        STATUS_DOT_YELLOW: 'w-2 h-2 bg-yellow-500 rounded-sm',
        STATUS_DOT_ORANGE: 'w-2 h-2 bg-orange-500 rounded-sm',
        STATUS_DOT_PURPLE: 'w-2 h-2 bg-purple-500 rounded-sm',
        STATUS_DOT_GREEN: 'w-2 h-2 bg-green-500 rounded-sm',
        STATUS_DOT_RED: 'w-2 h-2 bg-red-500 rounded-sm',
        
        // Common utilities
        LOADING_SPINNER: 'inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500',
        TRUNCATE: 'truncate',
        FLEX_CENTER_GAP_1: 'flex items-center gap-1',
        
        // Text sizes
        TEXT_XS: 'text-xs',
        TEXT_SM: 'text-sm',
        TEXT_BASE: 'text-base',
        
        // Font weights
        FONT_MEDIUM: 'font-medium',
        FONT_SEMIBOLD: 'font-semibold'
    };


    // Return the public API for this module
    return {
        sanitizeForId: (str) => str.replace(/[^a-zA-Z0-9-]/g, '-'),
        getUsageColor,
        createProgressTextBarHTML,
        getBackupTimestamp,
        toDateKey,
        formatBytes,
        formatBytesCompact,
        formatSpeed,
        formatSpeedWithStyling,
        formatUptime,
        formatDuration,
        formatPbsTimestamp,
        formatPbsTimestampRelative,
        sortData,
        renderTableBody,
        debounce,
        updateProgressBarTexts,
        preserveScrollPosition,
        getScrollableParent,
        getHostUrl,
        // Button state management
        setButtonLoading,
        resetButton,
        // Date formatting
        formatDate,
        formatDateTime,
        formatTimeAgo,
        // UI utilities
        initMobileScrollIndicators,
        getBackupStatusColor,
        createHealthBadgeHTML,
        // Threshold utilities
        getReadableThresholdName,
        formatThresholdValue,
        // Metric display helpers
        createMetricBarHtml,
        updateGuestIOMetric,
        showEmptyState,
        // Alert control helpers
        createAlertSliderHtml,
        createAlertDropdownHtml,
        // Utilities
        DOMCache,
        StorageHelper,
        createLogger,
        // Constants
        IO_ALERT_OPTIONS,
        CSS_CLASSES
    };
})(); // Test hot reload
