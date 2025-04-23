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
  const themeToggleButton = document.getElementById('theme-toggle-button');
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

  // Function to apply the theme (no longer needs to set checkbox state)
  function applyTheme(theme) {
    if (theme === 'dark') {
      htmlElement.classList.add('dark');
      localStorage.setItem('theme', 'dark');
    } else {
      htmlElement.classList.remove('dark');
      localStorage.setItem('theme', 'light');
    }
  }

  // Determine initial theme
  const initialTheme = savedTheme || (prefersDarkScheme.matches ? 'dark' : 'light');
  applyTheme(initialTheme);

  // Add event listener to the button
  if (themeToggleButton) {
    themeToggleButton.addEventListener('click', function() {
      // Toggle theme based on current state
      const currentIsDark = htmlElement.classList.contains('dark');
      applyTheme(currentIsDark ? 'light' : 'dark');
    });
  } else {
    console.warn('Element #theme-toggle-button not found - theme switching disabled.');
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
  // Load saved sort state from localStorage or use defaults
  const savedSortState = JSON.parse(localStorage.getItem('pulseSortState')) || {};
  const sortState = {
    nodes: { column: null, direction: 'asc', ...savedSortState.nodes },
    vms: { column: null, direction: 'asc', ...savedSortState.vms },
    containers: { column: null, direction: 'asc', ...savedSortState.containers },
    main: { column: 'id', direction: 'asc', ...savedSortState.main }
  };
  let groupByNode = true; // Default view
  let filterGuestType = 'all'; // Default filter
  const AVERAGING_WINDOW_SIZE = 5;
  const dashboardHistory = {}; // Re-add this line
  let filterStatus = 'all'; // New state variable for status filter
  let initialDataReceived = false; // Flag to control initial rendering
  let storageData = {}; // Add state for storage data

  // --- Global Helper for Text Progress Bar ---
  const createProgressTextBarHTML = (percent, text, colorClass) => {
      const numericPercent = isNaN(parseInt(percent)) ? 0 : parseInt(percent);
      const textColorClass = 'text-gray-700 dark:text-gray-200';

       return `
        <!-- Outer container with lighter background - REMOVED title attribute -->
        <div 
            class=\"w-full rounded h-4 relative overflow-hidden bg-gray-100 dark:bg-gray-700/50\" 
        >
          <!-- Inner div represents the actual progress, with opacity -->
          <div 
            class=\"absolute top-0 left-0 h-full rounded ${colorClass} opacity-50\" 
            style="width: ${numericPercent}%;"
          >
          </div>
          <!-- Text is centered within the outer container -->
          <span class="absolute inset-0 flex items-center justify-center text-xs font-medium ${textColorClass} px-1 truncate">
            ${text}
          </span>
        </div>
      `;
  };
  // --- End Global Helper ---

  // --- Global Helper for Usage Color ---
  const getUsageColor = (percent) => {
      if (isNaN(percent) || percent === 'N/A') return 'bg-gray-400 dark:bg-gray-600';
      const numericPercentage = parseInt(percent);
      // Using thresholds consistent across tables now
      if (numericPercentage > 85) return 'bg-red-500'; 
      if (numericPercentage > 70) return 'bg-yellow-500';
      return 'bg-green-500';
  };
  // --- End Global Helper ---

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

          // Save updated sort state to localStorage
          localStorage.setItem('pulseSortState', JSON.stringify(sortState));

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
          
          // Special Handling for Percentage Columns (Main and Nodes tables)
          if ((type === 'main' || type === 'nodes') && (col === 'cpu' || col === 'memory' || col === 'disk')) {
            // Treat N/A string as -1 for sorting purposes
            if (val === 'N/A') return -1;
            // Convert numeric percentage values (or numeric strings) to numbers
            const numericVal = parseFloat(val);
            return isNaN(numericVal) ? 0 : numericVal; // Default to 0 if parsing fails unexpectedly
          }
          // --- End Special Handling ---

          // Handle specific column logic if needed
          if (type === 'main' && col === 'id') val = parseInt(item.vmid || item.id || 0);
          else if (type === 'nodes' && col === 'id') val = item.node;
          // ... other specific cases ...
          
          // Fallback for other types or columns
          return val ?? (type === 'string' ? '' : 0); // Use default if null/undefined
      };

      valueA = getValue(a, column);
      valueB = getValue(b, column);

      // Determine type for comparison (Now should favor number for percentage columns)
      const compareType = (typeof valueA === 'number' && typeof valueB === 'number') ? 'number' : 'string';

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
    // Corrected selector to target the tbody directly by its ID
    const tbody = document.getElementById('nodes-table-body');
    if (!tbody) {
      console.error('Critical element #nodes-table-body not found for nodes table update!');
      return; 
    }
    tbody.innerHTML = ''; // Clear existing content

    const dataToDisplay = skipSorting ? (nodes || []) : sortData(nodes, sortState.nodes.column, sortState.nodes.direction, 'nodes');

    if (dataToDisplay.length === 0) {
      // Corrected colspan to match the actual number of columns (5)
      tbody.innerHTML = '<tr><td colspan="5" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found or data unavailable</td></tr>'; 
      return;
    }

    dataToDisplay.forEach(node => {
      const row = document.createElement('tr');
      // Use same hover/transition classes as main dashboard rows
      row.className = 'transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px'; 

      // --- Determine Status (Inferring 'online' if we have data from /status endpoint) ---
      // Proxmox API /nodes/{node}/status usually only returns data for online nodes.
      // A more robust check might involve looking at node.uptime or specific error fallbacks from the backend.
      const isOnline = node && node.uptime > 0; // Simple inference based on uptime
      const statusText = isOnline ? 'online' : (node.status || 'unknown'); // Use synthesized status if available, else unknown
      const statusColor = isOnline 
        ? 'bg-green-500 dark:bg-green-400' 
        : 'bg-red-500 dark:bg-red-400'; // Red for inferred offline/unknown

      // Calculate percentages safely using the correct data structure
      const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
      // Use node.memory object
      const memUsed = node.memory?.used || 0;
      const memTotal = node.memory?.total || 0;
      const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
      // Use node.rootfs object for disk
      const diskUsed = node.rootfs?.used || 0;
      const diskTotal = node.rootfs?.total || 0;
      const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0; 

      // Get color classes for bars
      const cpuColorClass = getUsageColor(cpuPercent);
      const memColorClass = getUsageColor(memPercent);
      const diskColorClass = getUsageColor(diskPercent);

      // Create tooltips and bar HTML using correct fields
      const cpuTooltipText = `${cpuPercent.toFixed(1)}%`;
      const memTooltipText = `${formatBytes(memUsed)} / ${formatBytes(memTotal)} (${memPercent.toFixed(1)}%)`;
      const diskTooltipText = `${formatBytes(diskUsed)} / ${formatBytes(diskTotal)} (${diskPercent.toFixed(1)}%)`;
      
      const cpuBarHTML = createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
      const memoryBarHTML = createProgressTextBarHTML(memPercent, memTooltipText, memColorClass);
      const diskBarHTML = createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);

      // Correctly generate the 5 columns matching the updated header order
      // Use styling consistent with main dashboard (p-1 px-2, etc.)
      row.innerHTML = `
        <td class=\"p-1 px-2 whitespace-nowrap\">
          <span class=\"flex items-center\">
            <span class=\"h-2.5 w-2.5 rounded-full ${statusColor} mr-2 flex-shrink-0\"></span>
            <span class=\"capitalize\">${statusText}</span>
          </span>
        </td>
        <td class=\"p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100\" title=\"${node.node || 'N/A'}\">${node.node || 'N/A'}</td>
        <!-- Add metric-tooltip-trigger and data-tooltip for custom tooltip -->
        <td class=\"p-1 px-2 text-right\">${cpuBarHTML}</td>
        <td class=\"p-1 px-2 text-right\">${memoryBarHTML}</td>
        <td class=\"p-1 px-2 text-right\">${diskBarHTML}</td>
      `;
      
      tbody.appendChild(row);
    });
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

  // --- Storage Data Display Function ---
  function updateStorageInfo(storage) {
    const contentDiv = document.getElementById('storage-info-content');
    if (!contentDiv) return;
    contentDiv.innerHTML = ''; // Clear previous content
    // Remove container styling, as it's now handled by the parent div in HTML
    contentDiv.className = ''; 

    // Check for global error first
    if (storage && storage.globalError) {
        // Error message styling - remove card styles, just use text/padding
        contentDiv.innerHTML = `<p class="p-4 text-red-700 dark:text-red-300">Error: ${storage.globalError}</p>`;
        return;
    }

    if (!storage || Object.keys(storage).length === 0) {
      // Empty message styling - remove card styles, just use text/padding
      contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No storage data available or failed to load for any node.</p>';
      return;
    }

    // --- Helper function for Storage Icons ---
    function getStorageTypeIcon(type) {
        // Simple icons using Tailwind/SVG - can be expanded
        switch(type) {
            case 'dir': 
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-yellow-600 dark:text-yellow-400"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>'; // Folder
            case 'lvm':
            case 'lvmthin':
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-purple-600 dark:text-purple-400"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>'; // Database (representing logical volume)
            case 'zfs':
            case 'zfspool':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-red-600 dark:text-red-400"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>'; // Activity (representing ZFS complexity/features)
            case 'nfs':
            case 'cifs':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-blue-600 dark:text-blue-400"><path d="M16 17l5-5-5-5"></path><path d="M8 17l-5-5 5-5"></path></svg>'; // Share-2
            case 'cephfs':
            case 'rbd':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-indigo-600 dark:text-indigo-400"><path d="M18 8h1a4 4 0 0 1 0 8h-1"></path><path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z"></path><line x1="6" y1="1" x2="6" y2="4"></line><line x1="10" y1="1" x2="10" y2="4"></line><line x1="14" y1="1" x2="14" y2="4"></line></svg>'; // Server (representing distributed storage)
            default:
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-gray-500"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>'; // HelpCircle (unknown)
        }
    }
    // --- End Helper ---

    // --- Updated Helper for Content Badge Details (Class + Tooltip) ---
    function getContentBadgeDetails(contentType) {
        let details = {
            badgeClass: 'bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300', // Default style
            tooltip: `Content type: ${contentType}` // Default tooltip
        };

        switch(contentType) {
            case 'iso': 
                details.badgeClass = 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300';
                details.tooltip = 'ISO images (e.g., for OS installation)';
                break;
            case 'vztmpl':
                details.badgeClass = 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300';
                details.tooltip = 'Container templates';
                break;
            case 'backup':
                details.badgeClass = 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300';
                details.tooltip = 'VM/Container backup files (vzdump)';
                break;
            case 'images':
                details.badgeClass = 'bg-teal-100 dark:bg-teal-900/50 text-teal-700 dark:text-teal-300';
                details.tooltip = 'VM disk images (qcow2, raw, etc.)';
                break;
            case 'rootdir':
                 details.badgeClass = 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300';
                 details.tooltip = 'Storage for container root filesystems';
                 break;
             case 'snippets':
                 details.badgeClass = 'bg-pink-100 dark:bg-pink-900/50 text-pink-700 dark:text-pink-300';
                 details.tooltip = 'Snippet files (e.g., cloud-init configs)';
                 break;
            // Add more cases as needed
        }
        return details;
    }
    // --- End Helper ---

    // --- Helper: Sort storage array --- 
    function sortNodeStorageData(storageArray) {
        if (!storageArray || !Array.isArray(storageArray)) return [];
        // Create a shallow copy to avoid modifying the original
        const sortedArray = [...storageArray];
        sortedArray.sort((a, b) => {
            const nameA = String(a.storage || '').toLowerCase();
            const nameB = String(b.storage || '').toLowerCase();
            return nameA.localeCompare(nameB);
        });
        return sortedArray;
    }
    // --- End Helper ---

    // Create ONE table for all nodes
    const table = document.createElement('table');
    table.className = 'w-full text-sm border-collapse table-fixed';

    const thead = document.createElement('thead');
    // Define widths for most columns, leave Usage column without width
    thead.innerHTML = `
        <tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 sticky top-0 z-10"> 
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-3/12">Storage</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-2/12">Content</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-1/12">Type</th>
          <th class="text-center p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-[80px]">Shared</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-[150px]">Usage</th>
          <th class="text-right p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-1/12">Avail</th>
          <th class="text-right p-2 px-3 font-semibold text-gray-700 dark:text-gray-300 w-1/12">Total</th>
        </tr>
      `;
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';

    // --- Sort nodes alphabetically before processing ---
    const sortedNodeNames = Object.keys(storage).sort((a, b) => a.localeCompare(b));
    // --- End Node Sorting ---

    // Iterate through the *sorted* node names
    sortedNodeNames.forEach(nodeName => {
      const nodeStorageData = storage[nodeName]; 

      // Add Node Header Row - Colspan needs to match data columns (7)
      const nodeHeaderRow = document.createElement('tr');
      nodeHeaderRow.className = 'bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs node-storage-header'; 
      nodeHeaderRow.innerHTML = `
        <td colspan="7" class="p-1.5 px-3">
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
          Node: ${nodeName}
        </td>`;
      tbody.appendChild(nodeHeaderRow);

      // Handle errors or empty data for this specific node
      if (nodeStorageData.error) {
        const errorRow = document.createElement('tr');
        errorRow.innerHTML = `<td colspan="7" class="p-2 px-3 text-sm text-red-600 dark:text-red-400 italic">Error loading storage: ${nodeStorageData.error}</td>`;
        tbody.appendChild(errorRow);
        return; // Skip to next node
      }

      if (!Array.isArray(nodeStorageData) || nodeStorageData.length === 0) {
        const noDataRow = document.createElement('tr');
        noDataRow.innerHTML = `<td colspan="7" class="p-2 px-3 text-sm text-gray-500 dark:text-gray-400 italic">No storage configured or found for this node.</td>`;
        tbody.appendChild(noDataRow);
        return; // Skip to next node
      }

      // Sort storage data within this node
      const sortedNodeStorageData = sortNodeStorageData(nodeStorageData);
      
      // Add Storage Data Rows for this node using the sorted storage data
      sortedNodeStorageData.forEach(store => { 
        const row = document.createElement('tr');
        const isDisabled = store.enabled === 0 || store.active === 0;
        row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700/60 hover:shadow-md hover:-translate-y-px ${isDisabled ? 'opacity-50 grayscale-[50%]' : ''}`;
        
        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const usageTooltipText = `${formatBytes(store.used)} / ${formatBytes(store.total)} (${usagePercent.toFixed(1)}%)`;
        
        const usageColorClass = getUsageColor(usagePercent);
        const sharedIconTooltip = store.shared === 1 ? 'Shared across cluster' : 'Local to node';
        const sharedIcon = store.shared === 1 ? `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block text-green-600 dark:text-green-400"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>` 
            : `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block text-gray-400 dark:text-gray-500 opacity-50"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>`;
        
        const contentTypes = (store.content || '').split(',').map(ct => ct.trim()).filter(ct => ct);
        contentTypes.sort(); 
        const contentBadges = contentTypes.map(ct => {
            const details = getContentBadgeDetails(ct); // Use the updated helper
            // Re-add data-tooltip with the purpose, add trigger class and cursor
            return `<span data-tooltip="${details.tooltip}" class="storage-tooltip-trigger inline-block ${details.badgeClass} rounded px-1.5 py-0.5 text-xs font-medium mr-1 mb-1 cursor-default">${ct}</span>`;
        }).join('');

        const usageBarHTML = createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass);

        row.innerHTML = `
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-900 dark:text-gray-100 font-medium">${store.storage || 'N/A'}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs">${contentBadges || '-'}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-600 dark:text-gray-300">${store.type || 'N/A'}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-center storage-tooltip-trigger cursor-default" data-tooltip="${sharedIconTooltip}">${sharedIcon}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-600 dark:text-gray-300">${usageBarHTML}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-600 dark:text-gray-300 text-right">${formatBytes(store.avail)}</td>
            <td class="p-2 px-3 py-1 whitespace-nowrap text-gray-600 dark:text-gray-300 text-right">${formatBytes(store.total)}</td>
        `;
        tbody.appendChild(row);
      });
    }); // End looping through nodes

    table.appendChild(thead);
    table.appendChild(tbody);
    contentDiv.appendChild(table);

    // --- Tooltip Listener Setup (Moved outside updateStorageInfo) ---
    if (tooltipElement) { 
        const storageTbody = table.querySelector('tbody'); // Get the tbody we just created
        if (storageTbody) {
           // Remove these listeners from here
        } // End if storageTbody
    } // End if tooltipElement
    // --- End Tooltip Listener Setup ---

  }

  // --- Consolidated Tooltip Logic (Attached to Document Body) ---
  if (tooltipElement) { 
      // Set faster duration (already done, kept for clarity)
      tooltipElement.classList.remove('duration-100'); 
      tooltipElement.classList.add('duration-50');

      document.body.addEventListener('mouseover', (event) => {
          // Look for either trigger class
          const target = event.target.closest('.metric-tooltip-trigger, .storage-tooltip-trigger'); 
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

      document.body.addEventListener('mouseout', (event) => {
          // Look for either trigger class
          const target = event.target.closest('.metric-tooltip-trigger, .storage-tooltip-trigger');
          if (target) {
              tooltipElement.classList.add('hidden', 'opacity-0');
              tooltipElement.classList.remove('opacity-100');
          }
      });

       document.body.addEventListener('mousemove', (event) => {
           // Look for either trigger class
           const target = event.target.closest('.metric-tooltip-trigger, .storage-tooltip-trigger');
           if (!tooltipElement.classList.contains('hidden') && target) {
               const offsetX = 10;
               const offsetY = 15;
               tooltipElement.style.left = `${event.pageX + offsetX}px`;
               tooltipElement.style.top = `${event.pageY + offsetY}px`;
           } else if (!tooltipElement.classList.contains('hidden') && !target) {
               // Optional: hide if mouse moves off trigger onto non-trigger area
               // This might be less desirable with body-level listener, could hide unexpectedly.
           }
       });

  } else {
      console.warn('Tooltip element not found, custom tooltips disabled.');
  }
  // --- End Consolidated Tooltip Logic ---

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

      const cpuPercent = Math.round(guest.cpu * 100);
      const memoryPercent = guest.memory; 
      const diskPercent = guest.disk;

      const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : ''}`;
      const memoryTooltipText = guest.memoryTotal ? `${formatBytesInt(guest.memoryCurrent)} / ${formatBytesInt(guest.memoryTotal)} (${memoryPercent}%)` : `${memoryPercent}%`;
      const diskTooltipText = guest.diskTotal ? `${formatBytesInt(guest.diskCurrent)} / ${formatBytesInt(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
      
      const cpuColorClass = getUsageColor(cpuPercent);
      const memColorClass = getUsageColor(memoryPercent);
      const diskColorClass = getUsageColor(diskPercent);
      
      const cpuBarHTML = createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
      const memoryBarHTML = createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);
      const diskBarHTML = createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);

      const typeIconClass = guest.type === 'VM'
          ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium' 
          : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
      const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type}</span>`;

      row.innerHTML = `
        <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
        <td class="p-1 px-1 text-center">${typeIcon}</td>
        <td class="p-1 px-2 text-center">${guest.id}</td>
        <td class="p-1 px-2 whitespace-nowrap">${formatUptime(guest.uptime)}</td>
        <td class="p-1 px-2">${cpuBarHTML}</td>
        <td class="p-1 px-2">${memoryBarHTML}</td>
        <td class="p-1 px-2">${diskBarHTML}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.diskread)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.diskwrite)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.netin)}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${formatSpeedInt(guest.netout)}</td>
      `;
      return row;
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
        console.log('[socket.on("rawData")] Parsed data and updated stores');

        // Set flag after first successful data parse
        if (!initialDataReceived) {
          initialDataReceived = true;
          console.log('[socket.on("rawData")] Initial data received, enabling UI updates.');
          // Optional: Trigger immediate first render instead of waiting for interval
          // updateAllUITables(); 
        }

        // --- REMOVED UI update calls from here ---
        // updateNodesTable(nodesData);
        // updateVmsTable(vmsData);
        // updateContainersTable(containersData);
        // refreshDashboardData(); 
        // console.log('[socket.on("rawData")] Processed data and updated UI');

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

  // --- Function to Reset Dashboard Filters/Sort ---
  function resetDashboardView() {
      console.log('Resetting dashboard view...');
      if (searchInput) searchInput.value = '';
      sortState.main = { column: 'id', direction: 'asc' }; 
      updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]'));
      
      const groupGroupedRadio = document.getElementById('group-grouped');
      if(groupGroupedRadio) groupGroupedRadio.checked = true;
      groupByNode = true;
      
      const filterAllRadio = document.getElementById('filter-all');
      if(filterAllRadio) filterAllRadio.checked = true;
      filterGuestType = 'all';
      
      const statusAllRadio = document.getElementById('filter-status-all');
      if(statusAllRadio) statusAllRadio.checked = true;
      filterStatus = 'all';
      
      updateDashboardTable();
      if (searchInput) searchInput.blur(); // Blur search input after reset
  }

  // --- Reset Filters/Sort Listener ---
  document.addEventListener('keydown', function(event) {
    const activeElement = document.activeElement;
    const isSearchInputFocused = activeElement === searchInput;
    const isGeneralInputElement = !isSearchInputFocused && (activeElement.tagName === 'INPUT' || activeElement.tagName === 'TEXTAREA' || activeElement.isContentEditable);

    if (event.key === 'Escape') {
        resetDashboardView(); // Call the reset function
    } else if (isSearchInputFocused && event.key === 'Enter') {
      searchInput.blur();
      event.preventDefault(); // Prevent any default Enter key behavior

    } else if (
        !isSearchInputFocused &&
        !isGeneralInputElement && // Don't hijack typing in other inputs
        !event.metaKey &&   // Ignore Cmd key (Mac)
        !event.ctrlKey &&  // Ignore Ctrl key
        !event.altKey &&   // Ignore Alt key
        event.key.length === 1 && // Check if it's a character key
        event.key !== ' ' // Ignore spacebar unless focusing is desired
    ) {
        if (searchInput) {
            searchInput.focus();
            // We let the subsequent keypress event populate the input
        }
    }
  });

  // Add listener for the new Reset button
  const resetButton = document.getElementById('reset-filters-button');
  if (resetButton) {
      resetButton.addEventListener('click', resetDashboardView); // Call the same reset function
  } else {
      console.warn('Reset button #reset-filters-button not found.');
  }

  // --- Initial Setup Calls ---
  updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]'));
  // Data is requested on socket 'connect' event

  // --- Frontend Render Interval ---
  function updateAllUITables() {
    // Update UI tables using the currently stored data
    updateNodesTable(nodesData);
    updateVmsTable(vmsData);
    updateContainersTable(containersData);
    refreshDashboardData(); // Process and update the main dashboard
    updateStorageInfo(storageData); // Update storage info tab
  }

  // Add a separate fetch for storage data, maybe less frequent?
  async function fetchStorageData() {
    try {
      const response = await fetch('/api/storage');
      // Removed the response.ok check here, as we want to parse the JSON even for 500 errors
      // to check for the globalError property.
      storageData = await response.json();
      // console.log('[Storage Fetch] Fetched storage data:', storageData);
      
      // If the server responded with an error status but *didn't* include our globalError JSON,
      // synthesize an error state for the UI.
      if (!response.ok && !storageData.globalError) {
          console.error('Error fetching storage data: Server returned status', response.status, 'but no globalError field.');
          storageData = { globalError: `Failed to load storage data (Status: ${response.status})` };
      } 
    } catch (error) {
      console.error('Error fetching or parsing storage data:', error);
      // Network error or JSON parsing error
      storageData = { globalError: 'Failed to load storage data due to a network or parsing error.' }; 
    }
    // We don't call updateStorageInfo here anymore, the main interval handles it.
  }

  setInterval(() => {
    if (initialDataReceived) {
      // console.log('[UI Interval] Updating UI tables...');
      updateAllUITables();
    }
  }, 2000); // Update UI every 2 seconds

  // Fetch storage data periodically (e.g., every 10 seconds)
  setInterval(fetchStorageData, 10000);
  fetchStorageData(); // Initial fetch on load

  // --- End Frontend Render Interval ---

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