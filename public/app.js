// Setup hot reload capability (No changes needed here unless it manipulates classes/styles)
(function setupHotReload() {
  // Check for connection to server and auto-refresh
  const socket = io();

  // Listen for hotReload event from server
  socket.on('hotReload', function() {
    console.log('Hot reload triggered, refreshing page...');
    window.location.reload();
  });

  // Fallback: Check for server disconnects/reconnects as trigger for reload
  let wasConnected = false;
  socket.on('connect', function() {
    console.log('Connected to server');
    if (wasConnected) {
      console.log('Reconnected - refreshing page');
      // Slight delay to ensure server is ready after reconnect
      setTimeout(() => window.location.reload(), 500);
    }
    wasConnected = true;
    // Note: Initial data request moved to DOMContentLoaded to ensure elements exist
  });

  // Optional: Log disconnects
  socket.on('disconnect', function(reason) {
    console.log('Disconnected from server:', reason);
    wasConnected = false; // Reset connection status
    // UI update for disconnect handled in DOMContentLoaded listener
  });

})();

document.addEventListener('DOMContentLoaded', function() {
  // Guard clauses to ensure essential elements exist before proceeding
  const themeToggle = document.getElementById('theme-toggle');
  const connectionStatus = document.getElementById('connection-status');
  const mainTableBody = document.querySelector('#main-table tbody');
  const tooltipElement = document.getElementById('custom-tooltip');
  const searchInput = document.getElementById('dashboard-search');
  const statusElement = document.getElementById('dashboard-status-text');
  const versionSpan = document.getElementById('app-version'); // Get version span

  if (!connectionStatus) {
      console.error('Critical element #connection-status not found!');
      return; // Stop execution if essential elements are missing
  }
  if (!mainTableBody) {
      console.error('Critical element #main-table tbody not found!');
      // Allow execution for other features, but log error
  }
   if (!tooltipElement) {
      console.warn('Element #custom-tooltip not found - tooltips will not work.');
  }

  const htmlElement = document.documentElement; // Target <html>

  // --- Theme Handling ---
  const prefersDarkScheme = window.matchMedia('(prefers-color-scheme: dark)');
  const savedTheme = localStorage.getItem('theme');

  function applyTheme(theme) {
    if (!themeToggle) return; // Guard against missing toggle
    if (theme === 'dark') {
      htmlElement.classList.add('dark');
      themeToggle.checked = true;
      localStorage.setItem('theme', 'dark');
    } else {
      htmlElement.classList.remove('dark');
      themeToggle.checked = false;
      localStorage.setItem('theme', 'light');
    }
  }

  // Apply initial theme only if toggle exists
  if (themeToggle) {
      applyTheme(savedTheme || (prefersDarkScheme.matches ? 'dark' : 'light'));

      themeToggle.addEventListener('change', function() {
        applyTheme(this.checked ? 'dark' : 'light');
      });
  } else {
      console.warn('Element #theme-toggle not found - theme switching disabled.');
      // Apply system preference without saving/using toggle state
      applyTheme(prefersDarkScheme.matches ? 'dark' : 'light');
  }

  // --- Tab Functionality ---
  const tabs = document.querySelectorAll('.tab');
  const tabContents = document.querySelectorAll('.tab-content');
  let showTab = 'main'; // Default visible tab

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const tabId = tab.getAttribute('data-tab');

      tabs.forEach(t => {
        t.classList.remove('active', 'bg-white', 'dark:bg-gray-800', 'border', 'border-gray-300', 'dark:border-gray-700', 'border-b-0', '-mb-px');
        t.classList.add('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');
      });
      tab.classList.add('active', 'bg-white', 'dark:bg-gray-800', 'border', 'border-gray-300', 'dark:border-gray-700', 'border-b-0', '-mb-px');
      tab.classList.remove('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');

      tabContents.forEach(content => {
        content.classList.remove('block');
        content.classList.add('hidden');
        if (content.id === tabId) {
          content.classList.remove('hidden');
          content.classList.add('block');
        }
      });

      showTab = tabId; // Update global state
      // Potentially trigger data refresh if needed for the specific tab
    });
  });

  // --- Data Storage and State ---
  let nodesData = [];
  let vmsData = [];
  let containersData = [];
  let metricsData = [];
  let dashboardData = [];
  const sortState = {
    nodes: { column: null, direction: 'asc' },
    vms: { column: null, direction: 'asc' },
    containers: { column: null, direction: 'asc' },
    main: { column: 'id', direction: 'asc' }
  };
  let groupByNode = true; // Default view
  let filterGuestType = 'all'; // Default filter
  const AVERAGING_WINDOW_SIZE = 5;
  const dashboardHistory = {};
  let filterStatus = 'all'; // New state variable for status filter

  // --- WebSocket Connection ---
  const socket = io();

  socket.on('connect', function() {
    console.log('[socket] Connected');
    connectionStatus.textContent = 'Connected';
    connectionStatus.classList.remove('disconnected', 'bg-gray-200', 'dark:bg-gray-700', 'text-gray-600', 'dark:text-gray-400', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300');
    connectionStatus.classList.add('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
    requestFullData(); // Request data once connected
  });

  socket.on('disconnect', function(reason) {
    console.log('[socket] Disconnected:', reason);
    connectionStatus.textContent = 'Disconnected';
    connectionStatus.classList.remove('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
    connectionStatus.classList.add('disconnected', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300');
  });

  // --- Sorting Logic ---
  function updateSortUI(tableId, clickedHeader) {
      const tableElement = document.getElementById(tableId);
      if (!tableElement) return; // Guard against missing table

      const table = tableId.split('-')[0];
      const headers = tableElement.querySelectorAll('th.sortable');
      const currentSort = sortState[table];

      headers.forEach(header => {
          header.classList.remove('bg-blue-50', 'dark:bg-blue-900/20');
          const arrow = header.querySelector('.sort-arrow');
          if (arrow) arrow.remove();

          if (header === clickedHeader && currentSort.column) { // Only highlight/arrow if a sort is active
              header.classList.add('bg-blue-50', 'dark:bg-blue-900/20');
              const arrowSpan = document.createElement('span');
              arrowSpan.className = 'sort-arrow ml-1';
              arrowSpan.textContent = currentSort.direction === 'asc' ? '▲' : '▼';
              header.appendChild(arrowSpan);
          }
      });
  }

  function setupTableSorting(tableId) {
      const tableElement = document.getElementById(tableId);
      if (!tableElement) {
          console.warn(`Table #${tableId} not found for sort setup.`);
          return;
      }
      const tableType = tableId.split('-')[0]; // e.g., 'nodes', 'main'

      tableElement.querySelectorAll('th.sortable').forEach(th => {
        th.addEventListener('click', () => {
          const column = th.getAttribute('data-sort');
          if (!column) return;

          if (sortState[tableType].column === column) {
            sortState[tableType].direction = sortState[tableType].direction === 'asc' ? 'desc' : 'asc';
          } else {
            sortState[tableType].column = column;
            sortState[tableType].direction = 'asc';
          }

          // Trigger the correct update function based on table type
          switch(tableType) {
              case 'nodes': updateNodesTable(nodesData); break;
              case 'vms': updateVmsTable(vmsData); break;
              case 'containers': updateContainersTable(containersData); break;
              case 'main': updateDashboardTable(); break;
              default: console.error('Unknown table type for sorting:', tableType);
          }

          updateSortUI(tableId, th);
        });
      });
  }

  // Setup sorting for all tables
  setupTableSorting('nodes-table');
  setupTableSorting('vms-table');
  setupTableSorting('containers-table');
  setupTableSorting('main-table');

  // --- Filtering Logic ---
  // Grouping Filter
  document.querySelectorAll('input[name="group-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        groupByNode = (this.value === 'grouped');
        updateDashboardTable();
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter
      }
    });
  });

  // Type Filter
  document.querySelectorAll('input[name="type-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        filterGuestType = this.value;
        updateDashboardTable();
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter
      }
    });
  });

  // Text Search Filter
  if (searchInput) {
      searchInput.addEventListener('input', function() {
          // Re-rendering the table applies the search filter within updateDashboardTable
          updateDashboardTable();
      });
  } else {
      console.warn('Element #dashboard-search not found - text filtering disabled.');
  }

  // Status Filter (NEW)
  document.querySelectorAll('input[name="status-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        filterStatus = this.value; // Update the status filter state
        updateDashboardTable(); // Re-render the table
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter if needed
      }
    });
  });

  // --- Data Sorting Function ---
  function sortData(data, column, direction, type) {
    if (!column || !data) return data || []; // Return empty array if data is null/undefined

    // Create a shallow copy to avoid modifying the original array
    const dataToSort = [...data];

    return dataToSort.sort((a, b) => {
      let valueA, valueB;

      // Use a helper to get comparable values, handling potential missing data
      const getValue = (item, col) => {
          if (!item) return type === 'string' ? '' : 0; // Default value based on expected type
          let val = item[col];
          // Handle specific column logic if needed
          if (type === 'main' && col === 'id') val = parseInt(item.vmid || item.id || 0);
          else if (type === 'nodes' && col === 'id') val = item.node;
          // ... other specific cases ...
          return val ?? (type === 'string' ? '' : 0); // Use default if null/undefined
      };

      valueA = getValue(a, column);
      valueB = getValue(b, column);

      // Determine type for comparison (simple check)
      const compareType = (typeof valueA === 'string' || typeof valueB === 'string') ? 'string' : 'number';

      // Comparison logic
      if (compareType === 'string') {
        valueA = String(valueA).toLowerCase();
        valueB = String(valueB).toLowerCase();
        return direction === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
      } else {
        // Ensure numeric comparison
        valueA = parseFloat(valueA) || 0;
        valueB = parseFloat(valueB) || 0;
        return direction === 'asc' ? valueA - valueB : valueB - valueA;
      }
    });
  }

  // --- Data Update/Display Functions ---
  function updateNodesTable(nodes, skipSorting = false) {
    const tbody = document.querySelector('#nodes-table tbody');
    if (!tbody) return; // Guard
    tbody.innerHTML = '';

    const dataToDisplay = skipSorting ? (nodes || []) : sortData(nodes, sortState.nodes.column, sortState.nodes.direction, 'nodes');

    if (dataToDisplay.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found</td></tr>';
      return;
    }

    dataToDisplay.forEach(node => {
      const row = document.createElement('tr');
      row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-700/60 transition-colors duration-150 ease-in-out';

      const statusColor = node.status === 'online' ? 'bg-green-500' : 'bg-red-500'; // Use red for non-online, consider gray/yellow for others if needed
      const cpuPercent = (node.cpu || 0) * 100;
      const memPercent = node.maxmem > 0 ? ((node.mem || 0) / node.maxmem) * 100 : 0;

      // Determine color based on percentage
      const getUsageColor = (percent) => {
        if (percent > 85) return 'bg-red-500';
        if (percent > 65) return 'bg-yellow-500';
        return 'bg-green-500'; // Default to green
      };

      const cpuColorClass = getUsageColor(cpuPercent);
      const memColorClass = getUsageColor(memPercent);

      row.innerHTML = `
        <td class="p-2 px-3 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">${node.node || 'N/A'}</td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
          <span class="flex items-center">
            <span class="h-2.5 w-2.5 rounded-full ${statusColor} mr-2"></span>
            ${node.status || 'N/A'}
          </span>
        </td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
          <div class="w-full bg-gray-200 dark:bg-gray-600 rounded-full h-2.5 group relative">
            <div class="${cpuColorClass} h-2.5 rounded-full" style="width: ${cpuPercent.toFixed(1)}%"></div>
            <span class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-1 hidden group-hover:block px-2 py-1 text-xs text-white bg-gray-900 rounded shadow-lg whitespace-nowrap">
              ${cpuPercent.toFixed(1)}%
            </span>
          </div>
        </td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
          <div class="w-full bg-gray-200 dark:bg-gray-600 rounded-full h-2.5 group relative">
            <div class="${memColorClass} h-2.5 rounded-full" style="width: ${memPercent.toFixed(1)}%"></div>
             <span class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-1 hidden group-hover:block px-2 py-1 text-xs text-white bg-gray-900 rounded shadow-lg whitespace-nowrap">
              ${formatBytes(node.mem)} / ${formatBytes(node.maxmem)} (${memPercent.toFixed(1)}%)
            </span>
          </div>
        </td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300 text-right">${formatBytes(node.maxmem)}</td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300 text-right">${formatUptime(node.uptime)}</td>
        <td class="p-2 px-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">${node.ip || 'N/A'}</td>
      `;
      tbody.appendChild(row);
    });

    // Re-enable tooltips if the library/method exists (assuming a simple CSS hover tooltip here)
    // This example uses group-hover, so no extra JS needed for *these* tooltips.
  }

  function updateVmsTable(vms, skipSorting = false) {
      const tbody = document.querySelector('#vms-table tbody');
      if (!tbody) return; // Guard
      tbody.innerHTML = '';

      const dataToDisplay = skipSorting ? (vms || []) : sortData(vms, sortState.vms.column, sortState.vms.direction, 'vms');

      if (dataToDisplay.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="p-4 text-center text-gray-500">No VMs found</td></tr>';
        return;
      }

      dataToDisplay.forEach(vm => {
        const row = document.createElement('tr');
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors';
        row.innerHTML = `
          <td class="p-1 px-2 whitespace-nowrap">${vm.vmid || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${vm.name || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${vm.node || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${vm.status || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${vm.cpus || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatBytes(vm.maxmem)}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatBytes(vm.maxdisk)}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatUptime(vm.uptime)}</td>
        `;
        tbody.appendChild(row);
      });
    }

    function updateContainersTable(containers, skipSorting = false) {
      const tbody = document.querySelector('#containers-table tbody');
      if (!tbody) return; // Guard
      tbody.innerHTML = '';

      const dataToDisplay = skipSorting ? (containers || []) : sortData(containers, sortState.containers.column, sortState.containers.direction, 'containers');

      if (dataToDisplay.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="p-4 text-center text-gray-500">No containers found</td></tr>';
        return;
      }

      dataToDisplay.forEach(ct => {
        const row = document.createElement('tr');
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors';
        row.innerHTML = `
          <td class="p-1 px-2 whitespace-nowrap">${ct.vmid || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${ct.name || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${ct.node || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${ct.status || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${ct.cpus || 'N/A'}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatBytes(ct.maxmem)}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatBytes(ct.maxdisk)}</td>
          <td class="p-1 px-2 whitespace-nowrap">${formatUptime(ct.uptime)}</td>
        `;
        tbody.appendChild(row);
      });
    }

  // --- Formatting Helpers ---
  function formatBytes(bytes) {
     if (bytes === undefined || bytes === null || isNaN(bytes)) return 'N/A';
     if (bytes <= 0) return '0 B'; // Handle 0 or negative
     const units = ['B', 'KB', 'MB', 'GB', 'TB'];
     const i = Math.floor(Math.log(bytes) / Math.log(1024));
     const unitIndex = Math.max(0, Math.min(i, units.length - 1));
     const value = bytes / Math.pow(1024, unitIndex);
     return `${parseFloat(value.toFixed(unitIndex === 0 ? 0 : 1))} ${units[unitIndex]}`;
  }
  function formatCpu(cpu) {
      if (cpu === undefined || cpu === null || isNaN(cpu)) return 'N/A';
      return `${(cpu * 100).toFixed(1)}%`;
  }
  function formatUptime(seconds) {
      if (seconds === undefined || seconds === null || isNaN(seconds) || seconds < 0) return 'N/A';
      if (seconds < 60) return '<1m';
      const d = Math.floor(seconds / 86400);
      const h = Math.floor((seconds % 86400) / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      let str = '';
      if (d > 0) str += `${d}d `;
      if (h > 0 || d > 0) str += `${h}h `; // Show 0h if days are present
      str += `${m}m`;
      return str.trim();
  }
  function formatSpeed(bytesPerSecond) {
      if (bytesPerSecond === undefined || bytesPerSecond === null || isNaN(bytesPerSecond)) return 'N/A';
      if (bytesPerSecond < 1) return '0 B/s';
      return `${formatBytes(bytesPerSecond)}/s`;
  }
  function formatBytesInt(bytes) {
      if (bytes === undefined || bytes === null || isNaN(bytes)) return 'N/A';
      if (bytes <= 0) return '0 B';
      const units = ['B', 'KB', 'MB', 'GB', 'TB'];
       const i = Math.floor(Math.log(bytes) / Math.log(1024));
      const unitIndex = Math.max(0, Math.min(i, units.length - 1));
      const value = bytes / Math.pow(1024, unitIndex);
      return `${Math.max(1, Math.round(value))} ${units[unitIndex]}`;
  }
  function formatCpuInt(cpu) {
      if (cpu === undefined || cpu === null || isNaN(cpu)) return 'N/A';
      return `${Math.round(cpu * 100)}%`;
  }
  function formatSpeedInt(bytesPerSecond) {
      if (bytesPerSecond === undefined || bytesPerSecond === null || isNaN(bytesPerSecond)) return 'N/A';
      if (bytesPerSecond < 1) return '0 B/s';
      return `${formatBytesInt(bytesPerSecond)}/s`;
  }

  // --- Dashboard Data Processing & Display ---
  function refreshDashboardData() {
    dashboardData = [];
    console.log('[refreshDashboardData] Starting refresh...');

    let maxNameLength = 0;
    let maxUptimeLength = 0;

    // Helper: Calculates average, returns null if invalid/insufficient data
    function calculateAverage(historyArray, key) {
      if (!historyArray || historyArray.length === 0) return null;
      const validEntries = historyArray.filter(entry => typeof entry[key] === 'number' && !isNaN(entry[key]));
      if (validEntries.length === 0) return null;
      const sum = validEntries.reduce((acc, curr) => acc + curr[key], 0);
      return sum / validEntries.length;
    }

    // Helper: Calculates rate, returns null if invalid/insufficient data
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

    // Process VMs and Containers
    const processGuest = (guest, type) => {
        const metrics = (metricsData || []).find(m => m.id === guest.vmid && m.type === type);
        // console.log(`[refreshDashboardData] Processing ${type} ${guest.vmid} (${guest.name}). Found metrics:`, metrics);

        let avgCpu = 0, avgMem = 0, avgDisk = 0;
        let avgDiskReadRate = 0, avgDiskWriteRate = 0, avgNetInRate = 0, avgNetOutRate = 0;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';

        if (guest.status === 'running' && metrics && metrics.current) {
            // Only process metrics history for running guests with current metrics
            console.log(`[dbg ${guest.vmid}] metrics.current:`, JSON.stringify(metrics.current)); // DEBUG: Log raw current metrics
            if (!dashboardHistory[guest.vmid]) dashboardHistory[guest.vmid] = [];
            const history = dashboardHistory[guest.vmid];
            const currentDataPoint = { timestamp: Date.now(), ...metrics.current };
            history.push(currentDataPoint);
            if (history.length > AVERAGING_WINDOW_SIZE) history.shift();

            avgCpu = calculateAverage(history, 'cpu') ?? 0;
            avgMem = calculateAverage(history, 'mem') ?? 0;
            avgDisk = calculateAverage(history, 'disk') ?? 0;
            avgDiskReadRate = calculateAverageRate(history, 'diskread') ?? 0;
            avgDiskWriteRate = calculateAverageRate(history, 'diskwrite') ?? 0;
            avgNetInRate = calculateAverageRate(history, 'netin') ?? 0;
            avgNetOutRate = calculateAverageRate(history, 'netout') ?? 0;
            avgMemoryPercent = (guest.maxmem > 0) ? Math.round(avgMem / guest.maxmem * 100) : 'N/A';
            avgDiskPercent = (guest.maxdisk > 0) ? Math.round(avgDisk / guest.maxdisk * 100) : 'N/A';
            console.log(`[dbg ${guest.vmid}] Rates (B/s): netin=${avgNetInRate?.toFixed(0)}, netout=${avgNetOutRate?.toFixed(0)}, diskread=${avgDiskReadRate?.toFixed(0)}, diskwrite=${avgDiskWriteRate?.toFixed(0)}`); // DEBUG: Log calculated rates
             // console.log(`[refreshDashboardData] ${type} ${guest.vmid} Calculated Percentages: Mem%=${avgMemoryPercent}, Disk%=${avgDiskPercent}`);
        } else if (guest.status === 'stopped') {
            // Clear history for stopped guests
            if (dashboardHistory[guest.vmid]) {
                delete dashboardHistory[guest.vmid];
            }
            // Metrics remain at default 0 / N/A for stopped guests
        }

        const name = guest.name || `${type === 'qemu' ? 'VM' : 'CT'} ${guest.vmid}`;
        const uptimeFormatted = formatUptime(guest.uptime);
        if (name.length > maxNameLength) maxNameLength = name.length;
        if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;

        // Always push the guest data, using defaults for stopped guests
        dashboardData.push({
            id: guest.vmid, name: name, node: guest.node,
            type: type === 'qemu' ? 'VM' : 'CT',
            status: guest.status,
            cpu: avgCpu,
            cpus: guest.cpus || 1,
            memory: avgMemoryPercent, // 'N/A' for stopped or if no maxmem
            memoryCurrent: avgMem,
            memoryTotal: guest.maxmem,
            disk: avgDiskPercent, // 'N/A' for stopped or if no maxdisk
            diskCurrent: avgDisk,
            diskTotal: guest.maxdisk,
            uptime: guest.status === 'running' ? guest.uptime : 0, // Sort stopped guests differently
            diskread: avgDiskReadRate,
            diskwrite: avgDiskWriteRate,
            netin: avgNetInRate,
            netout: avgNetOutRate
        });
    };

    (vmsData || []).forEach(vm => processGuest(vm, 'qemu'));
    (containersData || []).forEach(ct => processGuest(ct, 'lxc'));

    // Set Column Widths
    const nameColWidth = Math.min(Math.max(maxNameLength * 8 + 16, 100), 300);
    const uptimeColWidth = Math.max(maxUptimeLength * 7 + 16, 80);
    if (htmlElement) {
        htmlElement.style.setProperty('--name-col-width', `${nameColWidth}px`);
        htmlElement.style.setProperty('--uptime-col-width', `${uptimeColWidth}px`);
    }

    updateDashboardTable(); // Render the table
  }

  function updateDashboardTable() {
    if (!mainTableBody) return;
    mainTableBody.innerHTML = ''; // Clear

    const currentSearchTerm = searchInput ? searchInput.value.toLowerCase() : '';

    // Apply type filter
    const typeFilteredData = (dashboardData || []).filter(guest =>
      filterGuestType === 'all' || (guest.type && guest.type.toLowerCase() === filterGuestType)
    );

    // Apply status filter
    const statusFilteredData = typeFilteredData.filter(guest =>
      filterStatus === 'all' || guest.status === filterStatus
    );

    // Apply sorting
    const sortedData = sortData(statusFilteredData, sortState.main.column, sortState.main.direction, 'main');

    // Apply search filter
    const searchTerms = (searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : []);

    // Filter data based on search terms, type filter, and status filter
    let filteredData = sortedData.filter(item => {
      const typeMatch = (filterGuestType === 'all' || (item.type && item.type.toLowerCase() === filterGuestType));
      const statusMatch = (filterStatus === 'all' || item.status === filterStatus);
      const nameMatch = searchTerms.length === 0 || searchTerms.some(term =>
          (item.name?.toLowerCase() || '').includes(term) ||
          (item.node?.toLowerCase() || '').includes(term) || // Allow searching node name
          (item.id?.toString() || '').includes(term)        // Allow searching ID
      );
      return typeMatch && statusMatch && nameMatch; // Combine all filters
    });

    // Group data if needed
    const nodeGroups = {};
    if (groupByNode) {
      filteredData.forEach(guest => {
        if (!nodeGroups[guest.node]) nodeGroups[guest.node] = [];
        nodeGroups[guest.node].push(guest);
      });
    }

    // Render Rows
    let visibleCount = 0;
    let visibleNodes = new Set();

    if (groupByNode) {
      Object.keys(nodeGroups).sort().forEach(nodeName => {
        visibleNodes.add(nodeName.toLowerCase());
        const nodeHeaderRow = document.createElement('tr');
        nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
        nodeHeaderRow.innerHTML = `<td colspan="11" class="px-2 py-1">
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
          ${nodeName}
        </td>`;
        mainTableBody.appendChild(nodeHeaderRow);
        nodeGroups[nodeName].forEach(guest => {
          mainTableBody.appendChild(createGuestRow(guest));
          visibleCount++;
        });
      });
    } else {
      filteredData.forEach(guest => {
        mainTableBody.appendChild(createGuestRow(guest));
        visibleCount++;
        visibleNodes.add(guest.node.toLowerCase());
      });
    }

    // Handle empty table states
    if (visibleCount === 0) {
        const filterText = currentSearchTerm ? ` match filter "${currentSearchTerm}"` : '';
        const typeText = filterGuestType !== 'all' ? filterGuestType.toUpperCase() + 's' : 'guests';
        const statusText = filterStatus !== 'all' ? ` (${filterStatus})` : ''; // Add status to the message
        mainTableBody.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500">No ${typeText}${statusText}${filterText} found</td></tr>`;
    }

    // Update Status Text
    if (statusElement) {
        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = currentSearchTerm ? ` | Filter: "${currentSearchTerm}"` : '';
        let statusCountText = ` | Showing ${visibleCount}`;
        if (filterGuestType !== 'all') statusCountText += ` ${filterGuestType.toUpperCase()}s`;
        if (filterStatus !== 'all') statusCountText += ` (${filterStatus})`; // Add status to the count text
        statusCountText += ` guests`;
        if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
        statusElement.textContent = statusBaseText + statusFilterText + statusCountText;
    }
  }

  function createGuestRow(guest) {
      // console.log('[createGuestRow] Received guest data:', guest);
      const row = document.createElement('tr');
      // Add more prominent hover background, shadow, lift effect, and transition
      row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px ${guest.status === 'stopped' ? 'opacity-60' : ''}`;
      row.setAttribute('data-name', guest.name.toLowerCase());
      row.setAttribute('data-type', guest.type.toLowerCase());
      row.setAttribute('data-node', guest.node.toLowerCase());
      row.setAttribute('data-id', guest.id);

      const memoryPercent = guest.memory; // Already calculated, possibly 'N/A'
      const diskPercent = guest.disk;     // Already calculated, possibly 'N/A'
      const cpuPercent = Math.round(guest.cpu * 100);

      const cpuAbsolute = guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : '';
      const memoryAbsolute = guest.memoryTotal ? `(${formatBytesInt(guest.memoryCurrent)} / ${formatBytesInt(guest.memoryTotal)})` : '';
      const diskAbsolute = guest.diskTotal ? `(${formatBytesInt(guest.diskCurrent)} / ${formatBytesInt(guest.diskTotal)})` : '';

      const cpuUsageText = createUsageText(cpuPercent, cpuAbsolute);
      const memoryUsageText = createUsageText(memoryPercent, memoryAbsolute);
      const diskUsageText = createUsageText(diskPercent, diskAbsolute);

      const typeIconClass = guest.type === 'VM'
          ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-700'
          : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 border border-green-200 dark:border-green-700';
      const typeIcon = `<span class="type-icon inline-block w-5 h-5 leading-5 text-center rounded text-[10px] font-bold align-middle ${typeIconClass}">${guest.type}</span>`;

      row.innerHTML = `
        <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
        <td class="p-1 px-1 text-center">${typeIcon}</td>
        <td class="p-1 px-2 text-center">${guest.id}</td>
        <td class="p-1 px-2 whitespace-nowrap">${formatUptime(guest.uptime)}</td>
        <td class="p-1 px-2 text-center">${cpuUsageText}</td>
        <td class="p-1 px-2 text-center">${memoryUsageText}</td>
        <td class="p-1 px-2 text-center">${diskUsageText}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.diskread)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.diskwrite)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.netin)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.netout)}</td>
      `;
      return row;
  }

  function createUsageText(percentage, tooltipText = '') {
      // console.log(`[createUsageText] Received percentage: ${percentage}, tooltip: ${tooltipText}`);
      let colorClass = '';
      let displayPercentage = percentage;

      if (percentage === 'N/A' || isNaN(percentage)) {
         displayPercentage = 'N/A';
         colorClass = 'text-gray-400 dark:text-gray-500';
      } else {
         const numericPercentage = parseInt(percentage);
         displayPercentage = `${numericPercentage}%`;
         if (numericPercentage > 85) {
             colorClass = 'text-red-600 dark:text-red-400 font-medium';
         } else if (numericPercentage > 65) {
             colorClass = 'text-yellow-600 dark:text-yellow-400';
         } else {
             colorClass = 'text-green-600 dark:text-green-400';
         }
      }
      const safeTooltipText = tooltipText.replace(/"/g, '&quot;');
      return `<span class="${colorClass} metric-tooltip-trigger cursor-default" data-tooltip="${safeTooltipText}">${displayPercentage}</span>`;
  }

  // --- WebSocket Message Handling ---
  // Add a generic listener to catch *any* events from the server
  socket.onAny((eventName, ...args) => {
    console.log(`[socket.onAny] Received event: ${eventName}`, args);
  });

  // Listener for the 'rawData' event from the server
  socket.on('rawData', (jsonData) => {
    console.log('[socket.on("rawData")] Received data event');
    try {
        // Assuming server sends data as a JSON string
        const data = typeof jsonData === 'string' ? JSON.parse(jsonData) : jsonData;

        // Update global data stores
        nodesData = data.nodes || [];
        vmsData = data.vms || [];
        containersData = data.containers || [];
        metricsData = data.metrics || [];
        console.log('[socket.on("rawData")] Parsed data');

        // Update UI tables
        updateNodesTable(nodesData);
        updateVmsTable(vmsData);
        updateContainersTable(containersData);
        refreshDashboardData(); // Process and update the main dashboard
        console.log('[socket.on("rawData")] Processed data and updated UI');

    } catch (e) {
        console.error('Error processing received rawData:', e, jsonData);
    }
  });

  function requestFullData() {
      console.log("Requesting full data...");
      if (socket.connected) {
        socket.emit('requestData'); // Standard emit
      } else {
        console.warn("Socket not connected, cannot request full data.");
      }
  }

  // --- Tooltip Logic ---
  if (mainTableBody && tooltipElement) {
      mainTableBody.addEventListener('mouseover', (event) => {
          const target = event.target.closest('.metric-tooltip-trigger');
          if (target) {
              const tooltipText = target.getAttribute('data-tooltip');
              if (tooltipText) {
                  tooltipElement.textContent = tooltipText;
                  const offsetX = 10;
                  const offsetY = 15;
                  tooltipElement.style.left = `${event.pageX + offsetX}px`;
                  tooltipElement.style.top = `${event.pageY + offsetY}px`;
                  tooltipElement.classList.remove('hidden', 'opacity-0');
                  tooltipElement.classList.add('opacity-100');
              }
          }
      });
      mainTableBody.addEventListener('mouseout', (event) => {
          const target = event.target.closest('.metric-tooltip-trigger');
          if (target) {
              tooltipElement.classList.add('hidden', 'opacity-0');
              tooltipElement.classList.remove('opacity-100');
          }
      });
       mainTableBody.addEventListener('mousemove', (event) => {
           const target = event.target.closest('.metric-tooltip-trigger');
           if (!tooltipElement.classList.contains('hidden') && target) {
               // Update position while moving over the trigger
               const offsetX = 10;
               const offsetY = 15;
               tooltipElement.style.left = `${event.pageX + offsetX}px`;
               tooltipElement.style.top = `${event.pageY + offsetY}px`;
           } else if (!tooltipElement.classList.contains('hidden') && !target) {
              // Optional: hide if mouse moves off trigger onto non-trigger area
              // tooltipElement.classList.add('hidden', 'opacity-0');
              // tooltipElement.classList.remove('opacity-100');
           }
       });
  }

  // --- Reset Filters/Sort Listener ---
  document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape') {
      if(searchInput) searchInput.value = '';
      sortState.main = { column: 'id', direction: 'asc' };
      updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]'));
      const groupGroupedRadio = document.getElementById('group-grouped');
      if(groupGroupedRadio) groupGroupedRadio.checked = true;
      groupByNode = true;
      const filterAllRadio = document.getElementById('filter-all');
      if(filterAllRadio) filterAllRadio.checked = true;
      filterGuestType = 'all';
      filterStatus = 'all';
      updateDashboardTable();
    }
  });

  // --- Initial Setup Calls ---
  updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]'));
  // Data is requested on socket 'connect' event

  // --- Fetch and display version ---
  fetch('/api/version')
      .then(response => response.json())
      .then(data => {
          if (versionSpan && data.version) {
              versionSpan.textContent = data.version;
          }
      })
      .catch(error => {
          console.error('Error fetching version:', error);
          if (versionSpan) {
              versionSpan.textContent = 'error';
          }
      });
  // --- End fetch version ---

}); // End DOMContentLoaded