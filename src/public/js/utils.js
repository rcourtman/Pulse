PulseApp.utils = (() => {
    function getUsageColor(percentage) {
        if (percentage >= 90) return 'red';
        if (percentage >= 75) return 'yellow';
        return 'green';
    }

    function createProgressTextBarHTML(percentage, text, color) {
        // Always use a neutral background regardless of the progress color
        const bgColorClass = 'bg-gray-200 dark:bg-gray-700';

        const progressColorClass = {
            red: 'bg-red-500/50 dark:bg-red-500/50',
            yellow: 'bg-yellow-500/50 dark:bg-yellow-500/50',
            green: 'bg-green-500/50 dark:bg-green-500/50'
        }[color] || 'bg-gray-500/50'; // Fallback progress color with opacity

        return `
            <div class="relative w-full h-3.5 rounded overflow-hidden ${bgColorClass}">
                <div class="absolute top-0 left-0 h-full ${progressColorClass}" style="width: ${percentage}%;"></div>
                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">${text}</span>
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
            let valA, valB;
            let effectiveColumn = column;

            if (column === 'netin') effectiveColumn = 'net_in_rate';
            else if (column === 'netout') effectiveColumn = 'net_out_rate';
            else if (column === 'diskread') effectiveColumn = 'disk_read_rate';
            else if (column === 'diskwrite') effectiveColumn = 'disk_write_rate';

            valA = a[effectiveColumn];
            valB = b[effectiveColumn];

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

    // Return the public API for this module
    return {
        sanitizeForId: (str) => str.replace(/[^a-zA-Z0-9-]/g, '-'),
        getUsageColor,
        createProgressTextBarHTML,
        formatBytes,
        formatSpeed,
        formatUptime,
        formatDuration,
        formatPbsTimestamp,
        getReadableThresholdName,
        formatThresholdValue,
        getReadableThresholdCriteria,
        sortData,
        renderTableBody
    };
})();

PulseApp.utils.formatTimeAgo = (timestamp) => {
    const now = Date.now();
    const seconds = Math.round((now - timestamp) / 1000);

    if (seconds < 5) {
        return 'just now';
    }

    let interval = Math.floor(seconds / 31536000); // years
    if (interval >= 1) {
        return interval + (interval === 1 ? ' year' : ' years') + ' ago';
    }
    interval = Math.floor(seconds / 2592000); // months
    if (interval >= 1) {
        return interval + (interval === 1 ? ' month' : ' months') + ' ago';
    }
    interval = Math.floor(seconds / 86400); // days
    if (interval >= 1) {
        return interval + (interval === 1 ? ' day' : ' days') + ' ago';
    }
    interval = Math.floor(seconds / 3600); // hours
    if (interval >= 1) {
        return interval + (interval === 1 ? ' hour' : ' hours') + ' ago';
    }
    interval = Math.floor(seconds / 60); // minutes
    if (interval >= 1) {
        return interval + (interval === 1 ? ' min' : ' mins') + ' ago';
    }
    if (seconds < 0) { // For timestamps slightly in the future due to clock sync
        return 'just now'; 
    }
    return Math.floor(seconds) + (seconds === 1 ? ' sec' : ' secs') + ' ago';
}; 