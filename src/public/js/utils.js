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
            // Generic/legacy fallback (for other uses like storage, etc.)
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

        // Use simpleText for narrow screens if provided, otherwise just show percentage
        const mobileText = simpleText || `${percentage}%`;
        
        // Generate a unique ID for this progress bar to handle dynamic text switching
        const uniqueId = `pb-${Math.random().toString(36).substr(2, 9)}`;

        return `
            <div class="relative w-full h-3.5 rounded overflow-hidden ${bgColorClass}">
                <div class="absolute top-0 left-0 h-full ${progressColorClass}" style="width: ${percentage}%;"></div>
                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                    <span class="progress-text-full truncate px-1" data-progress-id="${uniqueId}" data-full-text="${text.replace(/"/g, '&quot;')}" data-simple-text="${mobileText.replace(/"/g, '&quot;')}">${text}</span>
                </span>
            </div>
        `;
    }

    function formatBytes(bytes, decimals = 1, k = 1024) {
        if (bytes === 0 || bytes === null || bytes === undefined) return '0 B';
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
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

    function getReadableThresholdName(type) {
        const names = {
            cpu: 'CPU',
            memory: 'Memory',
            disk: 'Disk Usage',
            diskread: 'Disk Read',
            diskwrite: 'Disk Write',
            netin: 'Net In',
            netout: 'Net Out'
        };
        return names[type] || type;
    }

    function formatThresholdValue(type, value) {
        const numericValue = Number(value);
        if (isNaN(numericValue)) return 'N/A';

        if (['cpu', 'memory', 'disk'].includes(type)) {
            return `${Math.round(numericValue)}%`;
        }
        if (['diskread', 'diskwrite', 'netin', 'netout'].includes(type)) {
            return formatSpeed(numericValue, 0);
        }
        return String(value); // Fallback
    }

    function getReadableThresholdCriteria(type, value) {
        const operatorMap = {
             diskread: '>=',
             diskwrite: '>=',
             netin: '>=',
             netout: '>='
         };
        const operator = operatorMap[type] || '>=';
        const displayValue = formatThresholdValue(type, value);
        return `${type}${operator}${displayValue}`;
    }

    function sortData(data, column, direction, tableType = 'main') {
        if (!column) return data;

        const sortStates = PulseApp.state.getSortState(tableType);
        const effectiveDirection = direction || sortStates.direction;

        return [...data].sort((a, b) => {
            let valA = a[column];
            let valB = b[column];

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
    function updateProgressBarTexts() {
        const progressTexts = document.querySelectorAll('.progress-text-full');
        
        progressTexts.forEach(span => {
            const fullText = span.getAttribute('data-full-text');
            const simpleText = span.getAttribute('data-simple-text');
            
            if (!fullText || !simpleText) return;
            
            // Check if the current text is overflowing
            const parent = span.parentElement;
            if (parent) {
                // Temporarily set to full text to measure
                span.textContent = fullText;
                
                // Check if text is truncated (scrollWidth > clientWidth)
                if (span.scrollWidth > parent.clientWidth - 8) { // 8px for padding
                    span.textContent = simpleText;
                } else {
                    span.textContent = fullText;
                }
            }
        });
    }
    
    // Debounced version for resize events
    const updateProgressBarTextsDebounced = debounce(updateProgressBarTexts, 100);

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
        
        // First check PBS configs for exact name match
        for (const config of pbsConfigs) {
            if (config.name === nodeName) {
                return config.host;
            }
        }
        
        // For Proxmox nodes, we need to find which endpoint this node belongs to
        // by looking at the nodes data from the API
        
        // First check if nodeName matches a displayName (for nodes with custom names)
        // This needs to be checked first because the UI shows displayName in headers
        const nodeByDisplayName = nodesData.find(node => node.displayName === nodeName);
        if (nodeByDisplayName && nodeByDisplayName.endpointId) {
            const endpoint = endpoints.find(ep => ep.id === nodeByDisplayName.endpointId);
            if (endpoint) {
                return endpoint.host;
            }
        }
        
        // Then find the node in the API data by actual node name
        const nodeInfo = nodesData.find(node => node.node === nodeName);
        
        if (nodeInfo && nodeInfo.endpointId) {
            // Find the endpoint that matches this endpointId
            const endpoint = endpoints.find(ep => ep.id === nodeInfo.endpointId);
            if (endpoint) {
                return endpoint.host;
            }
        }
        
        // Fallback: try direct name matches with endpoints
        for (const endpoint of endpoints) {
            if (endpoint.name === nodeName) {
                return endpoint.host;
            }
        }
        
        // Additional fallback: for standalone nodes, check if the endpoint name matches
        // and there's only one node for that endpoint
        for (const endpoint of endpoints) {
            const endpointNodes = nodesData.filter(node => node.endpointId === endpoint.id);
            if (endpointNodes.length === 1 && endpoint.name === nodeName) {
                return endpoint.host;
            }
        }
        
        return null;
    }

    // Return the public API for this module
    return {
        sanitizeForId: (str) => str.replace(/[^a-zA-Z0-9-]/g, '-'),
        getUsageColor,
        createProgressTextBarHTML,
        formatBytes,
        formatSpeed,
        formatSpeedWithStyling,
        formatUptime,
        formatDuration,
        formatPbsTimestamp,
        getReadableThresholdName,
        formatThresholdValue,
        getReadableThresholdCriteria,
        sortData,
        renderTableBody,
        debounce,
        updateProgressBarTexts,
        updateProgressBarTextsDebounced,
        preserveScrollPosition,
        getScrollableParent,
        getHostUrl
    };
})(); 