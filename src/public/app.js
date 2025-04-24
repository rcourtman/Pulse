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
  // ---> CHANGE: pbsData becomes pbsDataArray
  // let pbsData = {}; // Add state for PBS data
  let pbsDataArray = []; // Holds data for multiple PBS instances
  // <--- END CHANGE
  // Load saved sort state from localStorage or use defaults
  const savedSortState = JSON.parse(localStorage.getItem('pulseSortState')) || {};
  const sortState = {
    nodes: { column: null, direction: 'asc', ...(savedSortState.nodes || {}) },
    main: { column: 'id', direction: 'asc', ...(savedSortState.main || {}) },
    // Add default sort states for PBS task tables
    pbsBackup: { column: 'startTime', direction: 'desc', ...(savedSortState.pbsBackup || {}) },
    pbsVerify: { column: 'startTime', direction: 'desc', ...(savedSortState.pbsVerify || {}) },
    pbsSync: { column: 'startTime', direction: 'desc', ...(savedSortState.pbsSync || {}) },
    pbsPruneGc: { column: 'startTime', direction: 'desc', ...(savedSortState.pbsPruneGc || {}) }
  };
  let groupByNode = true; // Default view
  let filterGuestType = 'all'; // Default filter
  const AVERAGING_WINDOW_SIZE = 5;
  const dashboardHistory = {}; // Re-add this line
  let filterStatus = 'all'; // New state variable for status filter
  let initialDataReceived = false; // Flag to control initial rendering
  let storageData = {}; // Add state for storage data
  // ---> REMOVE: pbsConfigured flag is less relevant now
  // let pbsConfigured = false; // Flag to track if PBS is configured
  // <--- END REMOVE

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

      // ---> CHANGE: Get sort state key from tableId <---
      // const table = tableId.split('-')[0]; // Old way, assumes simple pattern
      let tableKey;
      if (tableId.startsWith('pbs-')) {
          // Extract the specific PBS key (e.g., 'pbsBackup', 'pbsVerify')
           // Match pbs-recent-<type>-tasks-table-<instanceId>
          const match = tableId.match(/pbs-recent-(backup|verify|sync|prunegc)-tasks-table-/);
          if (match && match[1]) {
              // Construct key like 'pbsBackup', 'pbsVerify', etc.
              tableKey = `pbs${match[1].charAt(0).toUpperCase() + match[1].slice(1)}`; 
          } else {
              console.error(`[updateSortUI] Could not extract PBS key from tableId: ${tableId}`);
              return; // Cannot proceed without key
          }
      } else if (tableId.startsWith('nodes-')) {
          tableKey = 'nodes';
      } else if (tableId.startsWith('main-')) {
          tableKey = 'main';
      } else {
           console.error(`[updateSortUI] Unknown table type for tableId: ${tableId}`);
           return; // Cannot proceed without key
      }
      // console.log(`[updateSortUI] Determined tableKey: '${tableKey}' for tableId: '${tableId}'`); // Optional Debug log
      // ---> END CHANGE <---

      const headers = tableElement.querySelectorAll('th.sortable');
      // ---> CHANGE: Use the derived tableKey <---
      const currentSort = sortState[tableKey]; 
      // ---> END CHANGE <---

      // ---> ADD GUARD: Check if currentSort is defined before accessing properties
      if (!currentSort) {
          console.error(`[updateSortUI] No sort state found for key: '${tableKey}' (derived from tableId: ${tableId})`);
          console.log('[updateSortUI] Current sortState:', JSON.stringify(sortState)); // Log the state for debugging
          return; // Cannot proceed if sort state doesn't exist
      }
      // ---> END GUARD <---

      headers.forEach(header => {
          header.classList.remove('bg-blue-50', 'dark:bg-blue-900/20');
          const arrow = header.querySelector('.sort-arrow');
          if (arrow) arrow.remove();

          // The error occurred here because currentSort was undefined
          // Now currentSort is guaranteed to exist due to the guard above.
          if (header === clickedHeader && currentSort.column) { // Only highlight/arrow if a sort is active for this key
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

          // Save updated sort state to localStorage (Only relevant keys)
          const stateToSave = {
            nodes: sortState.nodes,
            main: sortState.main
            // Note: PBS sort state is not saved to localStorage currently
          };
          localStorage.setItem('pulseSortState', JSON.stringify(stateToSave));

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
  // setupTableSorting('vms-table'); // Removed - Table doesn't exist in base HTML
  // setupTableSorting('containers-table'); // Removed - Table doesn't exist in base HTML
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

  // --- NEW PBS Task Sorting Function ---
  function sortPbsTasks(data, column, direction) {
      if (!column || !data) return data || [];
      const dataToSort = [...data];

      return dataToSort.sort((a, b) => {
          let valueA = a ? a[column] : null;
          let valueB = b ? b[column] : null;

          // Handle specific types for comparison
          if (column === 'startTime' || column === 'endTime' || column === 'duration') {
              valueA = Number(valueA) || 0;
              valueB = Number(valueB) || 0;
              return direction === 'asc' ? valueA - valueB : valueB - valueA;
          } else if (column === 'id') { // Guest ID/Job ID/Datastore
              valueA = String(valueA || '').toLowerCase();
              valueB = String(valueB || '').toLowerCase();
              return direction === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
          } else if (column === 'status') {
              // Simple status sort (OK > running > Error/Other > null/undefined)
              const getStatusValue = (status) => {
                  if (status === 'OK') return 0;
                  if (status === 'running') return 1;
                  if (!status) return 3;
                  return 2; // Other error statuses
              };
              valueA = getStatusValue(a?.status);
              valueB = getStatusValue(b?.status);
              return direction === 'asc' ? valueA - valueB : valueB - valueA;
          } else {
              // Default string comparison
              valueA = String(valueA || '').toLowerCase();
              valueB = String(valueB || '').toLowerCase();
              return direction === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
          }
      });
  }
  // --- END NEW PBS Task Sorting Function ---

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
      // ---> Use correct top-level properties for memory and disk <---
      const memUsed = node.mem || 0;       // Use node.mem
      const memTotal = node.maxmem || 0;     // Use node.maxmem
      const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
      // ---> Use node.rootfs object for disk
      const diskUsed = node.disk || 0;       // Use node.disk
      const diskTotal = node.maxdisk || 0;   // Use node.maxdisk
      const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0; 
      // ---> END MODIFICATION <---

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
    contentDiv.className = ''; 

    // Check for global error first
    if (storage && storage.globalError) {
        // Error message styling - remove card styles, just use text/padding
        contentDiv.innerHTML = `<p class="p-4 text-red-700 dark:text-red-300">Error: ${storage.globalError}</p>`;
        return;
    }

    // ---> REFINED CHECK for empty or all-error state <---
    const nodeKeys = storage ? Object.keys(storage) : [];
    const hasValidNodeData = nodeKeys.length > 0 && nodeKeys.some(key => Array.isArray(storage[key]));
    const allNodesAreErrors = nodeKeys.length > 0 && nodeKeys.every(key => storage[key] && storage[key].error);

    if (nodeKeys.length === 0) {
      // Truly empty or null/undefined
      contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No storage data received from server.</p>';
      return;
    } else if (allNodesAreErrors) {
      // All nodes reported fetch errors
      contentDiv.innerHTML = '<p class="text-red-600 dark:text-red-400 p-4 text-center">Failed to load storage data for all nodes. Check server logs.</p>';
      return;
    } else if (!hasValidNodeData) {
      // Contains keys, but none have valid array data (maybe unexpected format?)
      contentDiv.innerHTML = '<p class="text-yellow-600 dark:text-yellow-400 p-4 text-center">Received unexpected storage data format from server.</p>';
      return;
    }
    // ---> END REFINED CHECK <---

    // If we reach here, there's at least one node with potentially valid (even if empty) storage data array.

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

      // ---> REMOVED: Special handling for diskread debugging <---
      // if (key === 'diskread') { ... debug logic ... }
      // ---> END REMOVED SECTION <---

      if (!historyArray || historyArray.length < 2) return null;
      // ---> Keep filtering logic <---
      const validHistory = historyArray.filter(entry =>
          typeof entry.timestamp === 'number' && !isNaN(entry.timestamp) &&
          typeof entry[key] === 'number' && !isNaN(entry[key])
      );
      // ---> END SECTION <---

      // ---> REMOVED: Redundant log (was same as above) <---
      // console.log(`[calculateAverageRate - ${key}] Valid history (${validHistory.length} entries):`, validHistory.map(e => ({ t: new Date(e.timestamp).toLocaleTimeString(), v: e[key] })));
      // ---> END REMOVED SECTION <---

      if (validHistory.length < 2) return null;
      const oldest = validHistory[0];
      const newest = validHistory[validHistory.length - 1];
      const valueDiff = newest[key] - oldest[key];
      const timeDiff = (newest.timestamp - oldest.timestamp) / 1000;

      // ---> REMOVED: Duplicate history log <---
      // if (key === 'diskread' || key === 'diskwrite') { ... log sample history ... }
      // ---> END REMOVED SECTION <---

      // ---> Keep log for rate calculation details <---
      console.log(`[calculateAverageRate - ${key}] oldest=${oldest[key]}, newest=${newest[key]}, timeDiff=${timeDiff.toFixed(2)}s, valueDiff=${valueDiff}, rate=${timeDiff > 0 ? (valueDiff / timeDiff).toFixed(0) : 'N/A'}`);
      // ---> END SECTION <---
      if (timeDiff <= 0) return 0;
      return valueDiff / timeDiff;
    }

    // Process VMs and Containers
    const processGuest = (guest, type) => {
        // console.log(`[refreshDashboardData] Processing ${type} ${guest.vmid} (${guest.name}). Found metrics:`, metrics);
        let avgCpu = 0, avgMem = 0, avgDisk = 0;
        let avgDiskReadRate = 0, avgDiskWriteRate = 0, avgNetInRate = 0, avgNetOutRate = 0;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';

        if (guest.status === 'running') { // Only process running guests

            // ---> Keep Ensure history exists check <---
            if (!dashboardHistory[guest.vmid] || !Array.isArray(dashboardHistory[guest.vmid])) {
                // console.log(`[processGuest - ${guest.vmid}] Initializing/Resetting history array.`); // REMOVED THIS LOG
                dashboardHistory[guest.vmid] = [];
            }
            // ---> END SECTION <---

            const history = dashboardHistory[guest.vmid];
            const metrics = (metricsData || []).find(m => m.id === guest.vmid && m.type === type);

            if (metrics && metrics.current) {
                // console.log(`[dbg ${guest.vmid}] metrics.current:`, JSON.stringify(metrics.current));

                // ---> REVERTED: Back to spread syntax <---
                const currentDataPoint = { 
                    timestamp: Date.now(), 
                    ...metrics.current // Use spread syntax
                };
                // ---> END REVERTED SECTION <---

                // ---> REMOVED: Debug log for deep copy <---
                // console.log(`[processGuest - ${guest.vmid}] Created currentDataPoint (deep copy): ...`);
                // ---> END REMOVED SECTION <---

                history.push(currentDataPoint);
                if (history.length > AVERAGING_WINDOW_SIZE) history.shift();

                // Calculate averages (rest of the block remains the same)
                avgCpu = calculateAverage(history, 'cpu') ?? 0;
                avgMem = calculateAverage(history, 'mem') ?? 0;
                avgDisk = calculateAverage(history, 'disk') ?? 0;
                avgDiskReadRate = calculateAverageRate(history, 'diskread') ?? 0;
                avgDiskWriteRate = calculateAverageRate(history, 'diskwrite') ?? 0;
                // ---> REMOVED: Redundant log (already have calc log) <---
                // console.log(`[processGuest - ${guest.vmid}] Calculated avgDiskReadRate: ${avgDiskReadRate}`);
                // ---> END REMOVED SECTION <---
                avgNetInRate = calculateAverageRate(history, 'netin') ?? 0;
                avgNetOutRate = calculateAverageRate(history, 'netout') ?? 0;
                // ---> REMOVED: Redundant log (already have calc log) <---
                // console.log(`[processGuest - ${guest.vmid}] History before rate calc:`, ...);
                // ---> END REMOVED SECTION <---
                avgMemoryPercent = (guest.maxmem > 0) ? Math.round(avgMem / guest.maxmem * 100) : 'N/A';
                avgDiskPercent = (guest.maxdisk > 0) ? Math.round(avgDisk / guest.maxdisk * 100) : 'N/A';

            } // End if metrics && metrics.current

        } else { // Guest is stopped or unknown
             // Clear history for non-running guests
             if (dashboardHistory[guest.vmid]) {
                 console.log(`[processGuest - ${guest.vmid}] Clearing history for non-running guest.`);
                 delete dashboardHistory[guest.vmid];
             }
             // Metrics remain at default 0 / N/A
        }

        // Update dashboardData entry (rest of processGuest remains the same)
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
    // console.log(`[socket.onAny] Received event: ${eventName}`, args); // DEBUG: Keep commented
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
        // ---> REMOVED incorrect update to storageData <---
        // storageData = data.storage || {}; // DO NOT UPDATE storage here
        // ---> END REMOVAL <---

        // ---> MODIFIED: Only update metrics if present in data <---
        if (data.hasOwnProperty('metrics')) {
             metricsData = data.metrics || [];
             if (data.metrics && data.metrics.length > 0) {
                 console.log(`[socket.on("rawData")] Updated metricsData (${metricsData.length} entries).`);
             }
        } else {
            // If metrics key is missing (e.g., from discovery cycle), DO NOT update metricsData
             // ---> REMOVED DEBUG_METRICS check <---
            // console.log('[socket.on("rawData")] Metrics key missing, preserving existing metricsData.');
        }
        // ---> END MODIFICATION <---

        // ---> CHANGE: Update pbsDataArray
        if (data.hasOwnProperty('pbs')) {
            // Expecting an array now
            pbsDataArray = Array.isArray(data.pbs) ? data.pbs : []; 
            console.log(`[socket.on("rawData")] Updated pbsDataArray with ${pbsDataArray.length} instance(s).`);
        } else {
            // If pbs key is missing, preserve the existing array
            console.log('[socket.on("rawData")] PBS key missing in rawData, preserving existing pbsDataArray.');
        }
        // <--- END CHANGE

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

  // ---> CHANGE: Handle initial PBS status array
  socket.on('pbsInitialStatus', (pbsStatusArray) => {
      console.log('[socket] Received pbsInitialStatus array:', pbsStatusArray);
      if (Array.isArray(pbsStatusArray)) {
          // Update the global pbsDataArray with these initial statuses
          // This ensures the UI shows something before the first full discovery
          pbsDataArray = pbsStatusArray.map(statusInfo => ({ 
              ...statusInfo, // includes pbsEndpointId, pbsInstanceName, status
              // Add default empty structures for other fields expected by updatePbsInfo
              backupTasks: { recentTasks: [], summary: {} },
              datastores: [],
              verificationTasks: { summary: {} },
              syncTasks: { summary: {} },
              pruneTasks: { summary: {} },
              nodeName: null // Node name isn't known yet
          }));
          // Trigger an immediate partial UI update for PBS status
          updatePbsInfo(pbsDataArray);
      } else {
          console.warn('[socket] Received non-array data for pbsInitialStatus:', pbsStatusArray);
      }
  });
  // ---> END CHANGE

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
  // Update the UI to reflect the currently loaded sort state for the main table
  const initialMainSortColumn = sortState.main.column;
  const initialMainHeader = document.querySelector(`#main-table th[data-sort="${initialMainSortColumn}"]`);
  if (initialMainHeader) {
    updateSortUI('main-table', initialMainHeader);
  } else {
      // Fallback or log if the saved column header isn't found (e.g., after code changes)
      console.warn(`Initial sort header for column '${initialMainSortColumn}' not found in main table.`);
      updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]')); // Fallback to ID visual
  }
  // Data is requested on socket 'connect' event

  // --- Frontend Render Interval ---
  function updateAllUITables() {
    // Update UI tables using the currently stored data
    updateNodesTable(nodesData);
    // updateVmsTable(vmsData); // No separate VM table
    // updateContainersTable(containersData); // No separate CT table
    refreshDashboardData(); // Process and update the main dashboard
    updateStorageInfo(storageData); // Update storage info tab
    // ---> CHANGE: Pass pbsDataArray
    // updatePbsInfo(pbsData);
    updatePbsInfo(pbsDataArray);
    // <--- END CHANGE
  }

  // Add a separate fetch for storage data, maybe less frequent?
  async function fetchStorageData() {
    try {
      const response = await fetch('/api/storage');
      // Check if the response was successful (status code 200-299)
      if (!response.ok) {
          // Try to parse error json from server if possible, otherwise use status text
          let serverErrorMsg = `Server responded with status: ${response.status} ${response.statusText}`;
          try {
              const errorJson = await response.json();
              if (errorJson && errorJson.globalError) {
                  serverErrorMsg = errorJson.globalError;
              } else if (errorJson) {
                   serverErrorMsg += ` | Body: ${JSON.stringify(errorJson)}`;
              }
          } catch (parseError) {
              // Ignore parsing error if response is not JSON
          }
          throw new Error(serverErrorMsg); // Throw error to be caught below
      }

      // Only parse and update global storageData if response.ok
      const fetchedData = await response.json();
      // ---> MODIFIED: Only update global state on success <---
      // ---> REMOVED: Log successfully fetched data <---
      // console.log('[Storage Fetch] Successfully fetched and parsed data:', fetchedData);
      // ---> END REMOVED SECTION <---
      storageData = fetchedData; // Update the global variable
      // console.log('[Storage Fetch] Successfully updated storageData.'); // Optional debug log
      // ---> END MODIFICATION <---

    } catch (error) { // Catches fetch errors (network) and the thrown error above
      // ---> MODIFIED: Improved error logging (No change to logic here, just logging) <---
      let finalErrorMessage = 'Failed to load storage data due to an unknown error.';
      if (error instanceof TypeError) {
        // Likely a network error (failed fetch)
        finalErrorMessage = `Failed to load storage data due to a network error: ${error.message}`;
        console.error('Network error during storage fetch:', error);
      } else {
        // Likely an error thrown from the !response.ok block or JSON parsing error
        finalErrorMessage = error.message;
        console.error('Error processing storage response:', error);
      }
      // ---> MODIFIED: Don't update global storageData on error <---
      // storageData = { globalError: finalErrorMessage }; 
      // Just log the error, don't update the global state which would clear the UI
      console.error(`Storage fetch failed, preserving previous data. Error: ${finalErrorMessage}`);
      // ---> END MODIFICATION <---
    }
    // Update UI in the main interval now
    // updateStorageInfo(storageData);
  }

  setInterval(() => {
    if (initialDataReceived) {
      // console.log('[UI Interval] Updating UI tables...');
      updateAllUITables();
    }
  }, 2000); // Update UI every 2 seconds (was 2500)

  // Fetch storage data periodically (e.g., every 10 seconds)
  setInterval(fetchStorageData, 30000); // Changed to 30 seconds
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

  // --- Helper Functions for PBS Tab ---
  const formatPbsTimestamp = (ts) => {
      if (!ts) return 'Never';
      try {
          // Use Intl for locale-aware, shorter format
          return new Intl.DateTimeFormat(undefined, { 
              year: 'numeric', month: 'numeric', day: 'numeric',
              hour: 'numeric', minute: 'numeric'
          }).format(new Date(ts * 1000));
      } catch (e) {
          console.warn("Error formatting PBS timestamp:", ts, e);
          return 'Invalid Date';
      }
  };

  const formatDuration = (seconds) => {
      if (seconds === null || seconds === undefined || seconds < 0) return 'N/A';
      if (seconds === 0) return '0s';
      const d = Math.floor(seconds / (3600*24));
      const h = Math.floor(seconds % (3600*24) / 3600);
      const m = Math.floor(seconds % 3600 / 60);
      const s = Math.floor(seconds % 60);
      
      let parts = [];
      if (d > 0) parts.push(d + 'd');
      if (h > 0) parts.push(h + 'h');
      if (m > 0) parts.push(m + 'm');
      if (s > 0 || parts.length === 0) parts.push(s + 's'); // Show seconds if duration is < 1m
      
      return parts.slice(0, 2).join(''); // Show max 2 units (e.g., 1d2h, 3h5m, 45m10s, 3s)
  };

  const getPbsStatusIcon = (status) => {
      if (status === 'OK') {
          return '<span class="text-green-500 dark:text-green-400" title="OK">✓</span>'; // Checkmark
      } else if (status === 'running') {
           // Simple spinner (could be replaced with SVG)
          return '<span class="inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500" title="Running"></span>'; 
      } else if (status) {
          return `<span class="text-red-500 dark:text-red-400 font-bold" title="${status}">✗</span>`; // Cross mark for errors
      } else {
          return '<span class="text-gray-400" title="Unknown">?</span>';
      }
  };

  const getPbsGcStatusText = (gcStatus) => {
    if (!gcStatus || gcStatus === 'unknown') return '<span class="text-xs text-gray-400">Unknown</span>';
    // Basic coloring, can be enhanced
    let colorClass = 'text-gray-600 dark:text-gray-400';
    if (gcStatus.includes('error') || gcStatus.includes('failed')) {
        colorClass = 'text-red-500 dark:text-red-400';
    } else if (gcStatus === 'OK') {
        colorClass = 'text-green-500 dark:text-green-400';
    }
    return `<span class="text-xs ${colorClass}">${gcStatus}</span>`;
  };

  // --- Function to Update Specific Task Summary Card ---
  function updatePbsTaskSummaryCard(prefix, summaryData) {
    const okEl = document.getElementById(`pbs-${prefix}-ok`);
    const failedEl = document.getElementById(`pbs-${prefix}-failed`);
    const totalEl = document.getElementById(`pbs-${prefix}-total`);
    const lastOkEl = document.getElementById(`pbs-${prefix}-last-ok`);
    const lastFailedEl = document.getElementById(`pbs-${prefix}-last-failed`);

    if (!okEl || !failedEl || !totalEl || !lastOkEl || !lastFailedEl) {
      console.warn(`UI elements for PBS task summary '${prefix}' not found.`);
      return;
    }

    if (summaryData && summaryData.summary) {
      const summary = summaryData.summary;
      okEl.textContent = summary.ok ?? '-';
      failedEl.textContent = summary.failed ?? '-';
      totalEl.textContent = summary.total ?? '-';
      lastOkEl.textContent = formatPbsTimestamp(summary.lastOk);
      lastFailedEl.textContent = formatPbsTimestamp(summary.lastFailed);

      // Add warning style if failures exist
      failedEl.classList.toggle('font-bold', (summary.failed ?? 0) > 0);

    } else {
      // Clear fields if data is missing
      okEl.textContent = '-';
      failedEl.textContent = '-';
      totalEl.textContent = '-';
      lastOkEl.textContent = '-';
      lastFailedEl.textContent = '-';
      failedEl.classList.remove('font-bold');
    }
  }

  // --- NEW Function to Populate a PBS Task Table ---
  function populatePbsTaskTable(tbodyId, tasks, sortColumn, sortDirection) {
    const tbody = document.getElementById(tbodyId);
    if (!tbody) {
        console.error(`PBS Task Table Body #${tbodyId} not found!`);
        return;
    }
    tbody.innerHTML = ''; // Clear previous content

    const sortedTasks = sortPbsTasks(tasks, sortColumn, sortDirection);

    if (!sortedTasks || sortedTasks.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">No recent tasks found (last 7 days).</td></tr>`;
        return;
    }

    sortedTasks.forEach(task => {
        const row = document.createElement('tr');
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50';
        row.innerHTML = `
            <td class="px-4 py-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${task.id || 'N/A'}</td>
            <td class="px-4 py-2 text-center">${getPbsStatusIcon(task.status)}</td>
            <td class="px-4 py-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${formatPbsTimestamp(task.startTime)}</td>
            <td class="px-4 py-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${formatDuration(task.duration)}</td>
            <td class="px-4 py-2 whitespace-nowrap text-xs text-gray-400 dark:text-gray-500 font-mono truncate" title="${task.upid || 'N/A'}">${task.upid || 'N/A'}</td>
        `;
        tbody.appendChild(row);
    });
  }
  // --- END NEW Function ---

  // --- Updated Function: Update PBS Info Section (Upsert Logic) ---
  function updatePbsInfo(pbsArray) {
    const container = document.getElementById('pbs-instances-container');
    if (!container) {
        console.error("PBS container element #pbs-instances-container not found!");
        return;
    }

    console.log('[updatePbsInfo] Processing PBS array:', pbsArray); // Keep this log

    const currentInstanceIds = new Set(); // Keep track of instances processed in this run

    // Helper functions defined here or accessible in scope
    const createSummaryCard = (type, title, summaryData) => {
        const card = document.createElement('div');
        card.className = 'border border-gray-200 dark:border-gray-700 rounded p-3 bg-gray-100/50 dark:bg-gray-700/50';
        const summary = summaryData?.summary || {};
        const ok = summary.ok ?? '-';
        const failed = summary.failed ?? '-';
        const total = summary.total ?? '-';
        const lastOk = formatPbsTimestamp(summary.lastOk);
        const lastFailed = formatPbsTimestamp(summary.lastFailed);
        const failedStyle = (failed > 0) ? 'font-bold text-red-600 dark:text-red-400' : 'text-red-600 dark:text-red-400 font-semibold';
        card.innerHTML = `
            <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">${title} (7d)</h4>
            <div class="space-y-1 text-sm">
                <div><span class="font-medium text-gray-800 dark:text-gray-200">OK:</span> <span class="ml-1 text-green-600 dark:text-green-400 font-semibold">${ok}</span></div>
                <div><span class="font-medium text-gray-800 dark:text-gray-200">Failed:</span> <span class="ml-1 ${failedStyle}">${failed}</span></div>
                <div><span class="font-medium text-gray-800 dark:text-gray-200">Total:</span> <span class="ml-1 text-gray-700 dark:text-gray-300 font-semibold">${total}</span></div>
                <div><span class="font-medium text-gray-800 dark:text-gray-200">Last OK:</span> <span class="ml-1 text-gray-600 dark:text-gray-400 text-xs">${lastOk}</span></div>
                <div><span class="font-medium text-gray-800 dark:text-gray-200">Last Fail:</span> <span class="ml-1 text-gray-600 dark:text-gray-400 text-xs">${lastFailed}</span></div>
            </div>`;
        return card; // Return the element
    };

     const createTaskTableHTML = (tableId, title, idColumnHeader) => `
        <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Recent ${title} Tasks</h4>
        <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded max-h-60 overflow-y-auto">
            <table id="${tableId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                <thead class="text-xs text-gray-700 uppercase bg-gray-100 dark:bg-gray-700/50 sticky top-0">
                    <tr>
                        <th scope="col" class="sortable px-4 py-2 text-left font-semibold cursor-pointer select-none" data-sort="id">${idColumnHeader}</th>
                        <th scope="col" class="sortable px-4 py-2 text-center font-semibold cursor-pointer select-none" data-sort="status">Status</th>
                        <th scope="col" class="sortable px-4 py-2 text-left font-semibold cursor-pointer select-none" data-sort="startTime">Start Time</th>
                        <th scope="col" class="sortable px-4 py-2 text-left font-semibold cursor-pointer select-none" data-sort="duration">Duration</th>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">UPID</th>
                    </tr>
                </thead>
                <tbody id="${tableId.replace('-table-', '-tbody-')}" class="divide-y divide-gray-200 dark:divide-gray-700">
                     <!-- Populated by JS -->
                </tbody>
            </table>
        </div>`;

     const setupPbsSortListener = (tableId, sortStateKey, pbsInstance) => { // Pass pbsInstance here
        const tableElement = document.getElementById(tableId);
        if (!tableElement) {
            console.error(`PBS Task Table #${tableId} not found for sort listener setup!`);
            return;
        }

        tableElement.querySelectorAll('th.sortable').forEach(th => {
            if (th.dataset.listenerAttached === 'true') return;

            th.addEventListener('click', () => {
                const column = th.getAttribute('data-sort');
                if (!column) return;

                const currentSort = sortState[sortStateKey];
                if (currentSort.column === column) {
                    currentSort.direction = currentSort.direction === 'asc' ? 'desc' : 'asc';
                } else {
                    currentSort.column = column;
                    currentSort.direction = 'asc';
                }
                // Get the correct task list based on sortStateKey
                 let tasks;
                 switch (sortStateKey) {
                     case 'pbsBackup': tasks = pbsInstance.backupTasks?.recentTasks; break;
                     case 'pbsVerify': tasks = pbsInstance.verificationTasks?.recentTasks; break;
                     case 'pbsSync': tasks = pbsInstance.syncTasks?.recentTasks; break;
                     case 'pbsPruneGc': tasks = pbsInstance.pruneTasks?.recentTasks; break;
                     default: tasks = [];
                 }

                const tbodyId = tableId.replace('-table-', '-tbody-');
                populatePbsTaskTable(tbodyId, tasks, currentSort.column, currentSort.direction);
                updateSortUI(tableId, th, sortStateKey);
            });
            th.dataset.listenerAttached = 'true';
        });

        const currentSortConfig = sortState[sortStateKey];
         if (!currentSortConfig || !currentSortConfig.column) {
             console.warn(`Initial sort column not found for state key '${sortStateKey}'. Skipping initial sort UI update for table ${tableId}.`);
             return;
         }
        const initialHeader = tableElement.querySelector(`th[data-sort="${currentSortConfig.column}"]`);
        if (initialHeader) updateSortUI(tableId, initialHeader, sortStateKey);
    };


    // --- Create/Update content for each PBS instance ---
    pbsArray.forEach((pbsInstance, index) => {
        const instanceId = pbsInstance.pbsEndpointId || `instance-${index}`;
        const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
        const instanceElementId = `pbs-instance-${instanceId}`;
        currentInstanceIds.add(instanceElementId); // Track this ID

        let instanceWrapper = document.getElementById(instanceElementId);
        let detailsContainer, dsTableBody, statusElement;

        // Determine Status Text and Detail Visibility
        let statusText = 'Loading...';
        let showDetails = false;
        let statusColorClass = 'text-gray-600 dark:text-gray-400';
        switch (pbsInstance.status) {
            case 'configured':
                statusText = `Configured (${pbsInstance.nodeName || '...'}), attempting connection...`;
                statusColorClass = 'text-gray-600 dark:text-gray-400';
                break;
            case 'ok':
                statusText = `Status: OK (${pbsInstance.nodeName || 'Unknown Node'})`;
                statusColorClass = 'text-green-600 dark:text-green-400';
                showDetails = true;
                break;
            case 'error':
                statusText = `Error connecting (${pbsInstance.errorMessage || 'Check Pulse logs.'})`; // Try to show specific error
                statusColorClass = 'text-red-600 dark:text-red-400';
                break;
            // 'unconfigured' is handled by the empty array check earlier
            default:
                statusText = `Status: ${pbsInstance.status || 'Unknown'}`;
                break;
        }

        if (instanceWrapper) {
            // --- Instance Exists: Update ---
            statusElement = instanceWrapper.querySelector(`#pbs-status-${instanceId}`);
            detailsContainer = instanceWrapper.querySelector(`#pbs-details-${instanceId}`);

            // Update status text and color
            if (statusElement) {
                statusElement.textContent = statusText;
                statusElement.className = `text-sm ${statusColorClass}`; // Update classes
            }

            if (detailsContainer) {
                 // Update Datastore Table Body
                 dsTableBody = detailsContainer.querySelector(`#pbs-ds-tbody-${instanceId}`);
                 if (dsTableBody) {
                     dsTableBody.innerHTML = ''; // Clear existing rows
                     if (showDetails && pbsInstance.datastores) {
                         if (pbsInstance.datastores.length === 0) {
                             dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">No PBS datastores found or accessible.</td></tr>`;
                         } else {
                             pbsInstance.datastores.forEach(ds => {
                                const totalBytes = ds.total || 0;
                                const usedBytes = ds.used || 0;
                                const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
                                const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
                                const usageColor = getUsageColor(usagePercent);
                                const usageText = totalBytes > 0 ? `${usagePercent}% (${formatBytes(usedBytes)} of ${formatBytes(totalBytes)})` : 'N/A';
                                const gcStatusHtml = getPbsGcStatusText(ds.gcStatus);
                                const row = document.createElement('tr');
                                row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50';
                                row.innerHTML = `<td class="px-4 py-2 whitespace-nowrap">${ds.name || 'N/A'}</td> <td class="px-4 py-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${ds.path || 'N/A'}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${formatBytes(usedBytes)}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${formatBytes(availableBytes)}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${totalBytes > 0 ? formatBytes(totalBytes) : 'N/A'}</td> <td class="px-4 py-2 text-center min-w-[150px]">${totalBytes > 0 ? createProgressTextBarHTML(usagePercent, usageText, usageColor) : '-'}</td> <td class="px-4 py-2 text-center whitespace-nowrap">${gcStatusHtml}</td>`;
                                dsTableBody.appendChild(row);
                             });
                         }
                    } else { // Not OK status, show status text in table
                         dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    }
                 }

                 // Update Task Summary Cards
                 const summariesSection = detailsContainer.querySelector(`#pbs-summaries-section-${instanceId}`);
                 if (summariesSection) {
                     summariesSection.innerHTML = ''; // Clear existing cards
                     summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
                     summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
                     summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
                     summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
                 }

                 // Update Task Tables
                if (showDetails) {
                    populatePbsTaskTable(`pbs-recent-backup-tasks-tbody-${instanceId}`, pbsInstance.backupTasks?.recentTasks, sortState.pbsBackup.column, sortState.pbsBackup.direction);
                    populatePbsTaskTable(`pbs-recent-verify-tasks-tbody-${instanceId}`, pbsInstance.verificationTasks?.recentTasks, sortState.pbsVerify.column, sortState.pbsVerify.direction);
                    populatePbsTaskTable(`pbs-recent-sync-tasks-tbody-${instanceId}`, pbsInstance.syncTasks?.recentTasks, sortState.pbsSync.column, sortState.pbsSync.direction);
                    populatePbsTaskTable(`pbs-recent-prunegc-tasks-tbody-${instanceId}`, pbsInstance.pruneTasks?.recentTasks, sortState.pbsPruneGc.column, sortState.pbsPruneGc.direction);
                    // Re-apply/Ensure sort listeners exist
                    setupPbsSortListener(`pbs-recent-backup-tasks-table-${instanceId}`, 'pbsBackup', pbsInstance);
                    setupPbsSortListener(`pbs-recent-verify-tasks-table-${instanceId}`, 'pbsVerify', pbsInstance);
                    setupPbsSortListener(`pbs-recent-sync-tasks-table-${instanceId}`, 'pbsSync', pbsInstance);
                    setupPbsSortListener(`pbs-recent-prunegc-tasks-table-${instanceId}`, 'pbsPruneGc', pbsInstance);
                } else {
                     // If details are hidden, ensure tables show the status message
                    const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`);
                    if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`);
                    if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`);
                    if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`);
                    if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                }

                // Set visibility of the details container
                detailsContainer.classList.toggle('hidden', !showDetails);
            } // end if(detailsContainer)

        } else {
            // --- Instance Doesn't Exist: Create ---
            instanceWrapper = document.createElement('div');
            instanceWrapper.className = 'pbs-instance-section border border-gray-200 dark:border-gray-700 rounded p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30';
            instanceWrapper.id = instanceElementId;

            // Header
            const headerDiv = document.createElement('div');
            headerDiv.className = 'flex justify-between items-center mb-3';
            const instanceTitle = document.createElement('h3');
            instanceTitle.className = 'text-lg font-semibold text-gray-800 dark:text-gray-200';
            instanceTitle.textContent = instanceName;
            statusElement = document.createElement('div');
            statusElement.className = `text-sm ${statusColorClass}`;
            statusElement.id = `pbs-status-${instanceId}`;
            statusElement.textContent = statusText;
            headerDiv.appendChild(instanceTitle);
            headerDiv.appendChild(statusElement);
            instanceWrapper.appendChild(headerDiv);

            // Details Container
            detailsContainer = document.createElement('div');
            detailsContainer.className = `pbs-instance-details space-y-4 ${showDetails ? '' : 'hidden'}`;
            detailsContainer.id = `pbs-details-${instanceId}`;

            // Datastores Section
            const dsSection = document.createElement('div');
            dsSection.id = `pbs-ds-section-${instanceId}`;
            dsSection.innerHTML = `
                <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Datastores</h4>
                <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
                    <table id="pbs-ds-table-${instanceId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                         <thead class="text-xs text-gray-700 uppercase bg-gray-100 dark:bg-gray-700/50"> <tr> <th scope="col" class="px-4 py-2 text-left font-semibold">Name</th> <th scope="col" class="px-4 py-2 text-left font-semibold">Path</th> <th scope="col" class="px-4 py-2 text-right font-semibold">Used</th> <th scope="col" class="px-4 py-2 text-right font-semibold">Available</th> <th scope="col" class="px-4 py-2 text-right font-semibold">Total</th> <th scope="col" class="px-4 py-2 text-center font-semibold">Usage</th> <th scope="col" class="px-4 py-2 text-center font-semibold">GC Status</th> </tr> </thead>
                        <tbody id="pbs-ds-tbody-${instanceId}" class="divide-y divide-gray-200 dark:divide-gray-700"></tbody>
                    </table>
                </div>`;
            detailsContainer.appendChild(dsSection);

            // Task Summaries Section
            const summariesSection = document.createElement('div');
            summariesSection.id = `pbs-summaries-section-${instanceId}`;
            summariesSection.className = 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4';
            summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
            summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
            summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
            summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
            detailsContainer.appendChild(summariesSection);

            // Create Task Sections
             const recentBackupTasksSection = document.createElement('div');
             recentBackupTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-backup-tasks-table-${instanceId}`, 'Backup', 'Guest');
             detailsContainer.appendChild(recentBackupTasksSection);
             const recentVerifyTasksSection = document.createElement('div');
             recentVerifyTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-verify-tasks-table-${instanceId}`, 'Verification', 'Guest/Group');
             detailsContainer.appendChild(recentVerifyTasksSection);
             const recentSyncTasksSection = document.createElement('div');
             recentSyncTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-sync-tasks-table-${instanceId}`, 'Sync', 'Job ID');
             detailsContainer.appendChild(recentSyncTasksSection);
             const recentPruneGcTasksSection = document.createElement('div');
             recentPruneGcTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-prunegc-tasks-table-${instanceId}`, 'Prune/GC', 'Datastore/Group');
             detailsContainer.appendChild(recentPruneGcTasksSection);

            instanceWrapper.appendChild(detailsContainer);
            container.appendChild(instanceWrapper); // Append new instance to DOM

            // Populate tables and attach listeners AFTER appending
            dsTableBody = instanceWrapper.querySelector(`#pbs-ds-tbody-${instanceId}`); // Find the newly created body
             if (dsTableBody) {
                 if (showDetails && pbsInstance.datastores) {
                      if (pbsInstance.datastores.length === 0) { dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">No PBS datastores found or accessible.</td></tr>`; }
                      else { 
                          // Populate rows
                          pbsInstance.datastores.forEach(ds => { 
                            const totalBytes = ds.total || 0; 
                            const usedBytes = ds.used || 0; 
                            const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0); 
                            const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0; 
                            const usageColor = getUsageColor(usagePercent); 
                            const usageText = totalBytes > 0 ? `${usagePercent}% (${formatBytes(usedBytes)} of ${formatBytes(totalBytes)})` : 'N/A'; 
                            const gcStatusHtml = getPbsGcStatusText(ds.gcStatus); 
                            const row = document.createElement('tr'); 
                            row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50'; 
                            row.innerHTML = `<td class="px-4 py-2 whitespace-nowrap">${ds.name || 'N/A'}</td> <td class="px-4 py-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${ds.path || 'N/A'}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${formatBytes(usedBytes)}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${formatBytes(availableBytes)}</td> <td class="px-4 py-2 text-right whitespace-nowrap">${totalBytes > 0 ? formatBytes(totalBytes) : 'N/A'}</td> <td class="px-4 py-2 text-center min-w-[150px]">${totalBytes > 0 ? createProgressTextBarHTML(usagePercent, usageText, usageColor) : '-'}</td> <td class="px-4 py-2 text-center whitespace-nowrap">${gcStatusHtml}</td>`; 
                            dsTableBody.appendChild(row);
                          });
                      }
                 } else { dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`; }
             }

             if (showDetails) {
                populatePbsTaskTable(`pbs-recent-backup-tasks-tbody-${instanceId}`, pbsInstance.backupTasks?.recentTasks, sortState.pbsBackup.column, sortState.pbsBackup.direction);
                populatePbsTaskTable(`pbs-recent-verify-tasks-tbody-${instanceId}`, pbsInstance.verificationTasks?.recentTasks, sortState.pbsVerify.column, sortState.pbsVerify.direction);
                populatePbsTaskTable(`pbs-recent-sync-tasks-tbody-${instanceId}`, pbsInstance.syncTasks?.recentTasks, sortState.pbsSync.column, sortState.pbsSync.direction);
                populatePbsTaskTable(`pbs-recent-prunegc-tasks-tbody-${instanceId}`, pbsInstance.pruneTasks?.recentTasks, sortState.pbsPruneGc.column, sortState.pbsPruneGc.direction);

                setupPbsSortListener(`pbs-recent-backup-tasks-table-${instanceId}`, 'pbsBackup', pbsInstance);
                setupPbsSortListener(`pbs-recent-verify-tasks-table-${instanceId}`, 'pbsVerify', pbsInstance);
                setupPbsSortListener(`pbs-recent-sync-tasks-table-${instanceId}`, 'pbsSync', pbsInstance);
                setupPbsSortListener(`pbs-recent-prunegc-tasks-table-${instanceId}`, 'pbsPruneGc', pbsInstance);
             } else {
                 // Ensure initial status message is present if details start hidden
                 const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`); if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                 const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`); if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                 const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`); if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                 const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`); if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
             }

        } // End Upsert Logic

    }); // End forEach pbsInstance

    // --- Remove Orphaned Instances ---
    container.querySelectorAll('.pbs-instance-section').forEach(el => {
        if (!currentInstanceIds.has(el.id)) {
            console.log(`Removing orphaned PBS instance element: ${el.id}`);
            el.remove();
        }
    });

  }
  // --- End Update PBS Info Function ---

}); // End DOMContentLoaded