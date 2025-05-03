(function setupHotReload() {
  const socket = io();

  socket.on('hotReload', function() {
    window.location.reload();
  });

  let wasConnected = false;
  socket.on('connect', function() {
    if (wasConnected) {
      console.log('Reconnected - refreshing page');
      setTimeout(() => window.location.reload(), 500);
    }
    wasConnected = true;
  });

  socket.on('disconnect', function(reason) {
    wasConnected = false; // Reset connection status
  });

})();

const sanitizeForId = (str) => str.replace(/[^a-zA-Z0-9-]/g, '-');

document.addEventListener('DOMContentLoaded', function() {
  const themeToggleButton = document.getElementById('theme-toggle-button');
  const connectionStatus = document.getElementById('connection-status');
  const mainTableBody = document.querySelector('#main-table tbody');
  const tooltipElement = document.getElementById('custom-tooltip');
  const searchInput = document.getElementById('dashboard-search');
  const backupsSearchInput = document.getElementById('backups-search'); // Added
  const statusElement = document.getElementById('dashboard-status-text');
  const backupsStatusElement = document.getElementById('backups-status-text'); // Added
  const versionSpan = document.getElementById('app-version'); // Get version span

  if (!connectionStatus) {
      console.error('Critical element #connection-status not found!');
      return; // Stop execution if essential elements are missing
  }
  if (!mainTableBody) {
      console.error('Critical element #main-table tbody not found!');
  }
   if (!tooltipElement) {
      console.warn('Element #custom-tooltip not found - tooltips will not work.');
  }
  const sliderValueTooltip = document.getElementById('slider-value-tooltip'); // Get the new tooltip element
  if (!sliderValueTooltip) {
      console.warn('Element #slider-value-tooltip not found - slider values will not display on drag.');
  }

  const htmlElement = document.documentElement; // Target <html>

  const prefersDarkScheme = window.matchMedia('(prefers-color-scheme: dark)');
  const savedTheme = localStorage.getItem('theme');

  function applyTheme(theme) {
    if (theme === 'dark') {
      htmlElement.classList.add('dark');
      localStorage.setItem('theme', 'dark');
    } else {
      htmlElement.classList.remove('dark');
      localStorage.setItem('theme', 'light');
    }
  }

  const initialTheme = savedTheme || (prefersDarkScheme.matches ? 'dark' : 'light');
  applyTheme(initialTheme);

  if (themeToggleButton) {
    themeToggleButton.addEventListener('click', function() {
      const currentIsDark = htmlElement.classList.contains('dark');
      applyTheme(currentIsDark ? 'light' : 'dark');
    });
  } else {
    console.warn('Element #theme-toggle-button not found - theme switching disabled.');
  }

  const tabs = document.querySelectorAll('.tab');
  const tabContents = document.querySelectorAll('.tab-content');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      // --- Deactivate all top-level tabs and content --- 
      tabs.forEach(t => {
        t.classList.remove('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700', 'text-gray-900', 'dark:text-white');
        t.classList.add('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');
      });
      tabContents.forEach(content => content.classList.add('hidden'));

      // --- Activate clicked top-level tab and content --- 
      tab.classList.add('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700', 'text-gray-900', 'dark:text-white', '-mb-px');
      tab.classList.remove('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');
      
      const tabId = tab.getAttribute('data-tab');
      const activeContent = document.getElementById(tabId);
      if (activeContent) {
        activeContent.classList.remove('hidden');

        // --- Handle nested tabs within 'main' --- 
        if (tabId === 'main') {
            // Ensure the 'Dashboard' nested tab is active by default when switching TO 'main'
            activateNestedTab('nested-tab-dashboard'); 
            updateDashboardTable(); 
        }
        // --- End handle nested tabs ---

        if (tabId === 'backups') {
            updateBackupsTab(); // Call the update function when tab is selected
        }
      }
    });
  });

  // --- Nested Tab Logic (Main Dashboard) ---
  const nestedTabsContainer = document.querySelector('.nested-tabs'); // Container for tabs
  const nestedTabContentContainer = document.querySelector('#log-content-area'); // NEW: Container for log panels
  const logSessionArea = document.getElementById('log-session-area'); // NEW: Wrapper for log tabs & content
  // --- END Nested Tab Logic ---

  function activateNestedTab(targetId) {
      // Deactivate all nested tabs (only within the log section's tab container)
      if (nestedTabsContainer) {
          nestedTabsContainer.querySelectorAll('.nested-tab').forEach(nt => {
              nt.classList.remove('active', 'text-blue-600', 'border-blue-600'); // Reset styles
              nt.classList.add('text-gray-500', 'hover:text-gray-700', 'dark:text-gray-400', 'dark:hover:text-gray-200', 'border-transparent', 'hover:border-gray-300', 'dark:hover:border-gray-600');
          });
      }
      // Hide all nested tab content (only within the log content area)
      if (nestedTabContentContainer) {
          nestedTabContentContainer.querySelectorAll('.log-session-panel-container').forEach(panelContainer => { // CORRECT SELECTOR
              panelContainer.classList.add('hidden');
          });
      }

      // Activate the target tab (in the log section's tab container)
      const targetTab = nestedTabsContainer?.querySelector(`.nested-tab[data-nested-tab="${targetId}"]`);
      if (targetTab) {
          targetTab.classList.add('active', 'text-blue-600', 'border-blue-600'); // Apply active styles
          targetTab.classList.remove('text-gray-500', 'hover:text-gray-700', 'dark:text-gray-400', 'dark:hover:text-gray-200', 'border-transparent', 'hover:border-gray-300', 'dark:hover:border-gray-600');
      }

      // Show the target content (within the log content area)
      // const targetContent = nestedTabContentContainer?.querySelector(`#${targetId}`); // OLD: Looked for #log-session-id
      const targetPanelContainer = nestedTabContentContainer?.querySelector(`#${targetId}`); // Still look for the #log-session-id container
      if (targetPanelContainer) {
          targetPanelContainer.classList.remove('hidden');
          // Make sure the parent area is visible too
          if (logSessionArea) logSessionArea.classList.remove('hidden');
      }
      // Update dashboard if switching back to it - NO LONGER NEEDED HERE
      // if (targetId === 'nested-tab-dashboard') {
      //     updateDashboardTable();
      // }
  }

  // Add initial listeners only to static nested tabs (Dashboard) - REMOVED as there are no static tabs
  /*
  if (nestedTabsContainer) {
      nestedTabsContainer.querySelectorAll('.nested-tab:not([data-nested-tab^="log-session-"])').forEach(nestedTab => {
          nestedTab.addEventListener('click', () => {
              const nestedTabId = nestedTab.getAttribute('data-nested-tab');
              if (nestedTabId) { // Check if ID exists
                  activateNestedTab(nestedTabId);
              }
          });
      });
  }
  */

  let nodesData = [];
  let vmsData = [];
  let containersData = [];
  let metricsData = [];
  let dashboardData = [];
  let pbsDataArray = []; 
  const savedSortState = JSON.parse(localStorage.getItem('pulseSortState')) || {};
  const sortState = {
    nodes: { column: null, direction: 'asc', ...(savedSortState.nodes || {}) },
    main: { column: 'id', direction: 'asc', ...(savedSortState.main || {}) },
    backups: { column: 'latestBackupTime', direction: 'desc', ...(savedSortState.backups || {}) }
  };

  const savedFilterState = JSON.parse(localStorage.getItem('pulseFilterState')) || {};
  let groupByNode = savedFilterState.groupByNode ?? true; // Default view
  let filterGuestType = savedFilterState.filterGuestType || 'all'; 
  let filterStatus = savedFilterState.filterStatus || 'all'; // New state variable for status filter
  let backupsFilterHealth = savedFilterState.backupsFilterHealth || 'all'; // 'ok', 'warning', 'error', 'none'
  let backupsFilterGuestType = savedFilterState.backupsFilterGuestType || 'all'; // 'vm', 'ct', 'all'

  let backupsSearchTerm = ''; // Added: State for backup search
  let isThresholdRowVisible = false; // State for threshold row visibility

  // --- Updated Threshold State Structure ---
  let thresholdState = {
    cpu: { value: 0 }, // Keep sliders as simple value
    memory: { value: 0 },
    disk: { value: 0 },
    diskread: { value: 0 },
    diskwrite:{ value: 0 },
    netin:    { value: 0 },
    netout:   { value: 0 }
  };

  // --- Threshold Logging State ---
  let isThresholdLoggingActive = false; // Keep for potential UI state, but use activeLogSessions for logic
  let activeLogSessions = {}; // Object to hold multiple logging sessions { sessionId: { thresholds: {}, startTime: Date, durationMs: number, timerId: number, entries: [], element: node } }
  let thresholdLogEntries = [];
  let activeLoggingThresholds = null; // Will store the thresholds active at logging start
  // --- END Threshold Logging State ---

  // Load saved threshold state from localStorage
  const savedThresholdState = JSON.parse(localStorage.getItem('pulseThresholdState')) || {};
  // Merge saved state, ensuring structure is maintained
  for (const type in thresholdState) {
    if (savedThresholdState.hasOwnProperty(type)) {
        // Ensure the loaded type exists in our default structure
        if (thresholdState[type].hasOwnProperty('operator')) {
            // For text inputs, load operator and input string
            thresholdState[type].operator = savedThresholdState[type]?.operator || '>=';
            thresholdState[type].input = savedThresholdState[type]?.input || '';
            // We will parse the loaded input later when inputs are initialized
        } else {
            // For sliders, load the numeric value
            thresholdState[type].value = savedThresholdState[type]?.value || 0;
        }
    }
  }

  const AVERAGING_WINDOW_SIZE = 5;
  const dashboardHistory = {}; // Re-add this line
  let initialDataReceived = false; // Flag to control initial rendering
  let storageData = {}; // Add state for storage data

  const loadingOverlay = document.getElementById('loading-overlay');

  const INITIAL_PBS_TASK_LIMIT = 5;

  function saveFilterState() {
      const stateToSave = {
          groupByNode,
          filterGuestType,
          filterStatus,
          backupsFilterHealth,
          backupsFilterGuestType // ---> ADDED: Save backup type filter state <---
      };
      localStorage.setItem('pulseFilterState', JSON.stringify(stateToSave));
      // Save threshold state separately
      localStorage.setItem('pulseThresholdState', JSON.stringify(thresholdState));
  }

  function applyInitialFilterUI() {
      const groupRadio = document.getElementById(groupByNode ? 'group-grouped' : 'group-list');
      if (groupRadio) groupRadio.checked = true;
      const typeRadio = document.getElementById(`filter-${filterGuestType === 'ct' ? 'lxc' : filterGuestType}`);
      if (typeRadio) typeRadio.checked = true;
      const statusRadio = document.getElementById(`filter-status-${filterStatus}`);
      if (statusRadio) statusRadio.checked = true;
      const backupHealthRadio = document.getElementById(`backups-filter-status-${backupsFilterHealth}`);
      if (backupHealthRadio) backupHealthRadio.checked = true;
      const backupTypeRadio = document.getElementById(`backups-filter-type-${backupsFilterGuestType}`);
      if (backupTypeRadio) backupTypeRadio.checked = true;

      // --- Apply Initial Threshold UI ---
      for (const type in thresholdState) {
        if (sliders[type]) { // Slider
          const sliderElement = document.getElementById(`threshold-slider-${type}`);
          if (sliderElement) sliderElement.value = thresholdState[type].value;
        } else if (thresholdSelects[type]) { // Dropdown Select
          const inputElement = document.getElementById(`threshold-input-${type}`);
          const selectElement = document.getElementById(`threshold-select-${type}`);
          if (selectElement) selectElement.value = thresholdState[type].value;
        }
      }
      // --- END Apply Initial Threshold UI ---
  }

  // --- Define Threshold Element References BEFORE they are used ---
  const sliders = {
    cpu: document.getElementById('threshold-slider-cpu'),
    memory: document.getElementById('threshold-slider-memory'),
    disk: document.getElementById('threshold-slider-disk'),
  };
  const thresholdSelects = {
    diskread: document.getElementById('threshold-select-diskread'),
    diskwrite: document.getElementById('threshold-select-diskwrite'),
    netin: document.getElementById('threshold-select-netin'),
    netout: document.getElementById('threshold-select-netout'),
  };
  // --- END Threshold Element References ---

  // --- Threshold Indicator Logic ---
  const thresholdBadge = document.getElementById('threshold-count-badge');

  function updateThresholdIndicator() {
    if (!thresholdBadge) return;

    const mainTableHeader = document.querySelector('#main-table thead');
    const mainTableBody = document.querySelector('#main-table tbody'); // Need this to get default row classes later
    if (!mainTableHeader || !mainTableBody) return; // Guard if table elements not found

    let activeCount = 0;
    for (const type in thresholdState) {
      // Define default and active color classes
      const defaultColorClasses = ['text-gray-600', 'dark:text-gray-300'];
      const activeColorClasses = ['text-blue-600', 'dark:text-blue-400'];

      // Find the corresponding header cell
      const headerCell = mainTableHeader.querySelector(`th[data-sort="${type}"]`);

      if (thresholdState[type].value > 0) {
        activeCount++;
        // Add class to header
        if (headerCell) {
          headerCell.classList.add('threshold-active-header');
          // Remove default colors and add active colors
          headerCell.classList.remove(...defaultColorClasses);
          headerCell.classList.add(...activeColorClasses);
        }
      } else {
        // Remove class from header
        if (headerCell) {
          headerCell.classList.remove('threshold-active-header');
          // Remove active colors and add default colors
          // Note: Headers inherit base color from TR, but explicitly adding ensures override
          headerCell.classList.remove(...activeColorClasses);
          headerCell.classList.add(...defaultColorClasses);
        }
      }
    }

    if (activeCount > 0) {
      thresholdBadge.textContent = activeCount;
      thresholdBadge.classList.remove('hidden');
    } else {
      thresholdBadge.classList.add('hidden');
    }
  }
  // --- END Threshold Indicator Logic ---

  applyInitialFilterUI();
  updateThresholdIndicator(); // Set initial indicator state

  // Get references to threshold row and toggle button
  const thresholdRow = document.getElementById('threshold-slider-row');
  const toggleThresholdsButton = document.getElementById('toggle-thresholds-button');
  // const thresholdLogControlsRow = document.getElementById('threshold-log-controls-row'); // REMOVED

  // const logControlsArea = document.getElementById('log-controls-area'); // REMOVED - Controls moved back to main bar

  // --- Threshold Log Elements ---
  const toggleLogModeButton = document.getElementById('toggle-log-mode-button');
  // const logDurationInput = document.getElementById('log-duration-input'); // Old Input
  const logDurationSelect = document.getElementById('log-duration-select'); // New Select
  // const logSessionsContainer = document.getElementById('log-sessions-container'); // Added
  const logModeButtonText = toggleLogModeButton?.querySelector('span'); // Get inner span for text change
  // const logContainer = document.getElementById('threshold-log-container'); // REMOVED
  // const logTableBody = document.getElementById('threshold-log-tbody'); // REMOVED
  // const logStatusText = document.getElementById('threshold-log-status-text'); // REMOVED
  const mainTableContainer = document.getElementById('main'); // Assuming 'main' is the ID of the main tab content
  // --- END Threshold Log Elements ---

  // Function to update threshold row visibility based on state
  function updateThresholdRowVisibility() {
      // if (thresholdRow && logControlsArea) { // Old check
      if (thresholdRow) { // Only toggle the slider row now
          thresholdRow.classList.toggle('hidden', !isThresholdRowVisible);
          // logControlsArea.classList.toggle('hidden', !isThresholdRowVisible); // REMOVED
          // Optional: Update button appearance (e.g., add active class)
          if (toggleThresholdsButton) {
              toggleThresholdsButton.classList.toggle('bg-blue-100', isThresholdRowVisible);
              toggleThresholdsButton.classList.toggle('dark:bg-blue-800/50', isThresholdRowVisible);
              toggleThresholdsButton.classList.toggle('text-blue-700', isThresholdRowVisible);
              toggleThresholdsButton.classList.toggle('dark:text-blue-300', isThresholdRowVisible);
          }
      }
  }

  // Set initial visibility on load
  updateThresholdRowVisibility(); 

  // Add listener to toggle button
  if (toggleThresholdsButton) {
      toggleThresholdsButton.addEventListener('click', () => {
          isThresholdRowVisible = !isThresholdRowVisible;
          updateThresholdRowVisibility();
      });
  } else {
      console.warn('#toggle-thresholds-button not found.');
  }

  // --- BEGIN Threshold Slider Setup ---
  // --- BEGIN Threshold Text Input Setup ---
  // --- END Threshold Text Input Setup ---

  // Function to update threshold state and trigger table update
  function updateThreshold(type, value) {
    // All inputs now just update a simple value
    thresholdState[type].value = parseInt(value) || 0; // Ensure it's a number, default to 0
 
    updateDashboardTable();
    saveFilterState(); // Save state whenever a threshold changes
    updateThresholdIndicator(); // Update badge count
  }

  // Function to get formatted display text for a threshold type and value
  function getThresholdDisplayText(type, value) {
      const numericValue = parseInt(value);
      let displayText = '';
      if (type === 'uptime') {
        displayText = formatUptime(numericValue); 
      } else if (type === 'diskio' || type === 'netio') {
        displayText = formatSpeed(numericValue); 
      } else { // cpu, memory, disk
        displayText = `${numericValue}%`;
      }
      return displayText;
  }

  // Function to update and show the slider value tooltip
  function updateSliderTooltip(sliderElement) {
      if (!sliderValueTooltip || !sliderElement) return; // Guard clause

      const type = sliderElement.id.replace('threshold-slider-', ''); // Extract type from slider ID
      // Ensure we only process sliders here
      if (!sliders[type]) return;

      const numericValue = parseInt(sliderElement.value);
      let displayText = `${numericValue}%`; // Only sliders left are percentages

      const rect = sliderElement.getBoundingClientRect();
      const min = parseFloat(sliderElement.min);
      const max = parseFloat(sliderElement.max);
      const value = parseFloat(sliderElement.value);
      
      const percent = (max > min) ? (value - min) / (max - min) : 0;
      
      // Estimate thumb's visual center X position 
      // This calculation might need fine-tuning depending on exact browser/CSS for the thumb
      const thumbWidthEstimate = 16; // Approximate width of the slider thumb
      let thumbX = rect.left + (percent * (rect.width - thumbWidthEstimate)) + (thumbWidthEstimate / 2);
      
      // Update tooltip content first to get its actual width for centering
      sliderValueTooltip.textContent = displayText;
      sliderValueTooltip.classList.remove('hidden'); // Make visible before getting rect
      const tooltipRect = sliderValueTooltip.getBoundingClientRect(); 
      
      // Center tooltip horizontally over thumbX
      const posX = thumbX - (tooltipRect.width / 2);
      // Position tooltip above slider with a small gap
      const posY = rect.top - tooltipRect.height - 5; 

      sliderValueTooltip.style.left = `${posX}px`;
      sliderValueTooltip.style.top = `${posY}px`;
  }

  // Function to hide the slider tooltip
  function hideSliderTooltip() {
      if (sliderValueTooltip) {
          sliderValueTooltip.classList.add('hidden');
      }
  }

  // Add event listeners to sliders
  for (const type in sliders) {
    if (sliders[type]) {
      const sliderElement = sliders[type];

      // Update state & tooltip position WHILE dragging
      sliderElement.addEventListener('input', (event) => {
        const value = event.target.value;
        updateThreshold(type, value); // Update state & potentially filter table (if thresholdsMet check is used)
        updateSliderTooltip(event.target); // Update tooltip position & text
      });

      // Show tooltip on interaction start
      const showTooltip = (event) => {
          updateSliderTooltip(event.target);
      };
      sliderElement.addEventListener('mousedown', showTooltip);
      sliderElement.addEventListener('touchstart', showTooltip, { passive: true });

    } else {
      console.warn(`Slider element not found for type: ${type}`);
    }
  }

  // --- Add Event Listeners for Dropdown Selects ---
  for (const type in thresholdSelects) {
      const selectElement = thresholdSelects[type];
      if (selectElement) {
          selectElement.addEventListener('change', (event) => {
              const value = event.target.value;
              updateThreshold(type, value);
          });
      }
  }
  // --- END Select Listeners ---

  // Hide the slider tooltip on interaction end (mouseup/touchend anywhere)
  document.addEventListener('mouseup', hideSliderTooltip);
  document.addEventListener('touchend', hideSliderTooltip);
  // --- END Threshold Slider Setup ---

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
          <!-- Text is centered within the outer container - Add whitespace-nowrap -->
          <span class="absolute inset-0 flex items-center justify-center text-xs font-medium ${textColorClass} px-1 truncate whitespace-nowrap">
            ${text}
          </span>
        </div>
      `;
  };

  const getUsageColor = (percent) => {
      if (isNaN(percent) || percent === 'N/A') return 'bg-gray-400 dark:bg-gray-600';
      const numericPercentage = parseInt(percent);
      if (numericPercentage > 85) return 'bg-red-500'; 
      if (numericPercentage > 70) return 'bg-yellow-500';
      return 'bg-green-500';
  };

  const socket = io();

  socket.on('connect', function() {
    connectionStatus.textContent = 'Connected';
    connectionStatus.classList.remove('disconnected', 'bg-gray-200', 'dark:bg-gray-700', 'text-gray-600', 'dark:text-gray-400', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300');
    connectionStatus.classList.add('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
    

    requestFullData(); // Request data once connected
  });

  socket.on('disconnect', function(reason) {
    connectionStatus.textContent = 'Disconnected';
    connectionStatus.classList.remove('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
    connectionStatus.classList.add('disconnected', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300'); // Use distinct disconnected styling
    wasConnected = false; // Ensure hot reload logic knows state

    if (loadingOverlay) {
      const loadingText = loadingOverlay.querySelector('p'); 
      if (loadingText) {
          loadingText.textContent = 'Connection lost.';
          
      }
      loadingOverlay.style.display = 'flex'; 
    }
  });

  function updateSortUI(tableId, clickedHeader, explicitKey = null) { // Accept explicitKey
      const tableElement = document.getElementById(tableId);
      if (!tableElement) return;

      let derivedKey;
       if (tableId.startsWith('pbs-')) {
           const match = tableId.match(/pbs-recent-(backup|verify|sync|prunegc)-tasks-table-/);
           derivedKey = match && match[1] ? `pbs${match[1].charAt(0).toUpperCase() + match[1].slice(1)}` : null;
       } else if (tableId.startsWith('nodes-')) {
           derivedKey = 'nodes';
       } else if (tableId.startsWith('main-')) {
           derivedKey = 'main';
       } else if (tableId.startsWith('backups-')) {
           derivedKey = 'backups';
       } else {
           derivedKey = null;
       }

      const tableKey = explicitKey || derivedKey; // Use explicitKey if provided, otherwise fallback to derived (for non-PBS calls)
      if (!tableKey) {
          console.error(`[updateSortUI] Could not determine sort key for tableId: ${tableId}`);
          return;
      }

      const currentSort = sortState[tableKey];


      if (!currentSort) {
          console.error(`[updateSortUI] No sort state found for finalKey: '${tableKey}'`);
          return;
      }

      const headers = tableElement.querySelectorAll('th.sortable'); // Moved back up

      headers.forEach(header => {
          header.classList.remove('bg-blue-50', 'dark:bg-blue-900/20');
          const arrow = header.querySelector('.sort-arrow');
          if (arrow) arrow.remove();

          if (header === clickedHeader && currentSort.column) {
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

          const stateToSave = {
            nodes: sortState.nodes,
            main: sortState.main,
            backups: sortState.backups
          };
          localStorage.setItem('pulseSortState', JSON.stringify(stateToSave));

          switch(tableType) {
              case 'nodes': updateNodesTable(nodesData); break;
              case 'vms': updateVmsTable(vmsData); break;
              case 'containers': updateContainersTable(containersData); break;
              case 'main': updateDashboardTable(); break;
              case 'backups': updateBackupsTab(); break; 
              default: console.error('Unknown table type for sorting:', tableType);
          }

          updateSortUI(tableId, th);
        });
      });
  }

  setupTableSorting('nodes-table');
  setupTableSorting('main-table');
  setupTableSorting('backups-overview-table'); // Setup sorting for the backups table

  document.querySelectorAll('input[name="group-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        groupByNode = (this.value === 'grouped');
        updateDashboardTable();
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter
        saveFilterState(); // ---> ADDED: Save state <---
      }
    });
  });

  document.querySelectorAll('input[name="type-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        filterGuestType = this.value; // Use the restored state variable
        updateDashboardTable(); // Update the main dashboard table
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter
        saveFilterState(); // ---> ADDED: Save state <---
      }
    });
  });

  if (searchInput) {
      searchInput.addEventListener('input', function() {
          updateDashboardTable();
      });
  } else {
      console.warn('Element #dashboard-search not found - text filtering disabled.');
  }

  if (backupsSearchInput) {
      backupsSearchInput.addEventListener('input', function() {
          updateBackupsTab(); // Re-render the backups table to apply the search
      });
  } else {
      console.warn('Element #backups-search not found - backups text filtering disabled.');
  }

  document.querySelectorAll('input[name="status-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        filterStatus = this.value; // Update the status filter state
        updateDashboardTable(); // Re-render the table
        if (searchInput) searchInput.dispatchEvent(new Event('input')); // Re-apply text filter if needed
        saveFilterState(); // ---> ADDED: Save state <---
      }
    });
  });

  document.querySelectorAll('input[name="backups-type-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        backupsFilterGuestType = this.value;
        updateBackupsTab(); // Only update the backups tab
        saveFilterState(); // Save state
      }
    });
  });

  document.querySelectorAll('input[name="backups-status-filter"]').forEach(radio => {
    radio.addEventListener('change', function() {
      if (this.checked) {
        backupsFilterHealth = this.value; // Update the health filter state
        updateBackupsTab(); // Only update the backups tab
        saveFilterState(); // ---> ADDED: Save state <---
      }
    });
  });

  const resetBackupsButton = document.getElementById('reset-backups-filters-button');
  const backupsTabContent = document.getElementById('backups'); // Declare ONCE here

  if (resetBackupsButton) {
      resetBackupsButton.addEventListener('click', resetBackupsView);
  }

  if (backupsTabContent) { // Reuse for keydown listener
      backupsTabContent.addEventListener('keydown', (event) => {
          if (event.key === 'Escape') {
              if (backupsTabContent.contains(document.activeElement)) {
                  resetBackupsView();
              }
          }
      });
  }

  function sortData(data, column, direction, type) {
    if (!column || !data) return data || []; // Return empty array if data is null/undefined

    const dataToSort = [...data];

    return dataToSort.sort((a, b) => {
      let valueA, valueB;

      const getValue = (item, col) => {
          if (!item) return type === 'string' ? '' : (col === 'latestBackupTime' ? null : 0); // Handle backups time
          let val = item[col];

          if ((type === 'main' || type === 'nodes') && (col === 'cpu' || col === 'memory' || col === 'disk')) {
            if (val === 'N/A') return -1;
            const numericVal = parseFloat(val);
            return isNaN(numericVal) ? 0 : numericVal; // Default to 0 if parsing fails unexpectedly
          }
          if (type === 'main' && col === 'id') val = parseInt(item.vmid || item.id || 0);
          else if (type === 'nodes' && col === 'id') val = item.node;
          else if (type === 'backups') {
              if (col === 'backupHealthStatus') {
                  switch (item[col]) {
                      case 'failed': return 1;
                      case 'old': return 2;
                      case 'stale': return 3;
                      case 'none': return 4;
                      case 'ok': return 5;
                      default: return 0; // Should not happen
                  }
              }
              if (col === 'guestId' || col === 'totalBackups') val = parseInt(item[col] || 0);
              if (col === 'latestBackupTime') val = item[col]; // Keep as timestamp (number or null)
          }

          return val ?? (type === 'string' ? '' : (col === 'latestBackupTime' ? null : 0)); // Use default if null/undefined
      };

      if (type === 'nodes') {
          if (column === 'uptime') {
              valueA = a ? a.uptime || 0 : 0;
              valueB = b ? b.uptime || 0 : 0;
          } else if (column === 'loadnorm') { // <-- Changed from 'loadavg'
              const normA = (a && a.loadavg && a.loadavg.length > 0 && a.maxcpu > 0) 
                  ? (parseFloat(a.loadavg[0]) / a.maxcpu) 
                  : -1;
              const normB = (b && b.loadavg && b.loadavg.length > 0 && b.maxcpu > 0) 
                  ? (parseFloat(b.loadavg[0]) / b.maxcpu) 
                  : -1;
              valueA = isNaN(normA) ? -1 : normA;
              valueB = isNaN(normB) ? -1 : normB;
          } else {
              valueA = getValue(a, column);
              valueB = getValue(b, column);
          }
      } else {
        valueA = getValue(a, column);
        valueB = getValue(b, column);
      }

      const compareType = (typeof valueA === 'number' && typeof valueB === 'number') || (column === 'latestBackupTime' && (typeof valueA === 'number' || valueA === null) && (typeof valueB === 'number' || valueB === null)) 
          ? 'number' 
          : 'string';

      if (compareType === 'string') {
        valueA = String(valueA ?? '').toLowerCase(); // Handle potential null/undefined
        valueB = String(valueB ?? '').toLowerCase(); // Handle potential null/undefined
        return direction === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
      } else { // Numeric comparison (including timestamps)
        if (valueA === null && valueB === null) return 0;
        if (valueA === null) return direction === 'asc' ? -1 : 1;
        if (valueB === null) return direction === 'asc' ? 1 : -1;

        valueA = parseFloat(valueA) || 0;
        valueB = parseFloat(valueB) || 0;
        return direction === 'asc' ? valueA - valueB : valueB - valueA;
      }
    });
  }

  /*
  function sortPbsTasks(data, column, direction) {
  }
  */

  function updateNodesTable(nodes, skipSorting = false) {
    const tbody = document.getElementById('nodes-table-body');
    if (!tbody) {
      console.error('Critical element #nodes-table-body not found for nodes table update!');
      return; 
    }
    tbody.innerHTML = ''; // Clear existing content

    const dataToDisplay = skipSorting ? (nodes || []) : sortData(nodes, sortState.nodes.column, sortState.nodes.direction, 'nodes');

    if (dataToDisplay.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">No nodes found or data unavailable</td></tr>'; 
      return;
    }

    dataToDisplay.forEach(node => {
      const row = document.createElement('tr');
      row.className = 'transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px'; 


      const isOnline = node && node.uptime > 0; // Simple inference based on uptime
      const statusText = isOnline ? 'online' : (node.status || 'unknown'); // Use synthesized status if available, else unknown
      const statusColor = isOnline 
        ? 'bg-green-500 dark:bg-green-400' 
        : 'bg-red-500 dark:bg-red-400'; // Red for inferred offline/unknown

      const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
      const memUsed = node.mem || 0;       // Use node.mem
      const memTotal = node.maxmem || 0;     // Use node.maxmem
      const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
      const diskUsed = node.disk || 0;       // Use node.disk
      const diskTotal = node.maxdisk || 0;   // Use node.maxdisk
      const diskPercent = (diskUsed && diskTotal > 0) ? (diskUsed / diskTotal * 100) : 0; 

      const cpuColorClass = getUsageColor(cpuPercent);
      const memColorClass = getUsageColor(memPercent);
      const diskColorClass = getUsageColor(diskPercent);

      const cpuTooltipText = `${cpuPercent.toFixed(1)}%${node.maxcpu && node.maxcpu > 0 ? ` (${(node.cpu * node.maxcpu).toFixed(1)}/${node.maxcpu} cores)` : ''}`;
      const memTooltipText = `${formatBytes(memUsed)} / ${formatBytes(memTotal)} (${memPercent.toFixed(1)}%)`;
      const diskTooltipText = `${formatBytes(diskUsed)} / ${formatBytes(diskTotal)} (${diskPercent.toFixed(1)}%)`;
      
      const cpuBarHTML = createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
      const memoryBarHTML = createProgressTextBarHTML(memPercent, memTooltipText, memColorClass);
      const diskBarHTML = createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);

      const uptimeFormatted = formatUptime(node.uptime || 0);
      let normalizedLoadFormatted = 'N/A';
      if (node.loadavg && node.loadavg.length > 0 && node.maxcpu && node.maxcpu > 0) {
          const load1m = parseFloat(node.loadavg[0]);
          if (!isNaN(load1m)) {
              const normalizedLoad = load1m / node.maxcpu;
              normalizedLoadFormatted = normalizedLoad.toFixed(2);
          } else {
              console.warn(`[updateNodesTable] Node '${node.node}' has non-numeric loadavg[0]:`, node.loadavg[0]);
          }
      } else if (node.loadavg && node.maxcpu <= 0) {
           console.warn(`[updateNodesTable] Node '${node.node}' has invalid maxcpu (${node.maxcpu}) for load normalization.`);
      } // Implicit else: loadavg missing or empty, keep 'N/A'

      row.innerHTML = `
        <td class="p-1 px-2 whitespace-nowrap">
          <span class="flex items-center">
            <span class="h-2.5 w-2.5 rounded-full ${statusColor} mr-2 flex-shrink-0"></span>
            <span class="capitalize">${statusText}</span>
          </span>
        </td>
        <td class="p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100" title="${node.node || 'N/A'}">${node.node || 'N/A'}</td>
        <!-- Add min-width to progress bar columns -->
        <td class="p-1 px-2 text-right min-w-[200px]">${cpuBarHTML}</td>
        <td class="p-1 px-2 text-right min-w-[200px]">${memoryBarHTML}</td>
        <td class="p-1 px-2 text-right min-w-[200px]">${diskBarHTML}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${uptimeFormatted}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${normalizedLoadFormatted}</td>
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

  function formatBytes(bytes) {
/* 
     if (bytes === undefined || bytes === null || isNaN(bytes)) return 'N/A_debug';
     return `DEBUG_RAW_${bytes}`;
*/ 
     if (bytes === undefined || bytes === null || isNaN(bytes)) return 'N/A';
     if (bytes <= 0) return '0 B'; // Handle 0 or negative
     const units = ['B', 'KB', 'MB', 'GB', 'TB'];
     const i = Math.floor(Math.log(bytes) / Math.log(1024));
     const unitIndex = Math.max(0, Math.min(i, units.length - 1));
     const value = bytes / Math.pow(1024, unitIndex);
     let decimals = 0;
     if (unitIndex === 1 || unitIndex === 2) { // KB or MB
       decimals = 1;
     } else if (unitIndex >= 3) { // GB or TB
       decimals = 2;
     }
     return `${value.toFixed(decimals)} ${units[unitIndex]}`;
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
      if (bytesPerSecond === 0) return '0 B/s'; // Show 0 only if exactly 0
      if (bytesPerSecond < 1) return '<1 B/s'; // Show <1 for small positive rates
      return `${formatBytesInt(bytesPerSecond)}/s`;
  }

  function updateStorageInfo(storage) {
    const contentDiv = document.getElementById('storage-info-content');
    if (!contentDiv) return;
    contentDiv.innerHTML = ''; // Clear previous content
    contentDiv.className = ''; 

    if (storage && storage.globalError) {
        contentDiv.innerHTML = `<p class="p-4 text-red-700 dark:text-red-300">Error: ${storage.globalError}</p>`;
        return;
    }

    const nodeKeys = storage ? Object.keys(storage) : [];
    const hasValidNodeData = nodeKeys.length > 0 && nodeKeys.some(key => Array.isArray(storage[key]));
    const allNodesAreErrors = nodeKeys.length > 0 && nodeKeys.every(key => storage[key] && storage[key].error);

    if (nodeKeys.length === 0) {
      contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No storage data received from server.</p>';
      return;
    } else if (allNodesAreErrors) {
      contentDiv.innerHTML = '<p class="text-red-600 dark:text-red-400 p-4 text-center">Failed to load storage data for all nodes. Check server logs.</p>';
      return;
    } else if (!hasValidNodeData) {
      contentDiv.innerHTML = '<p class="text-yellow-600 dark:text-yellow-400 p-4 text-center">Received unexpected storage data format from server.</p>';
      return;
    }


    function getStorageTypeIcon(type) {
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
        }
        return details;
    }

    function sortNodeStorageData(storageArray) {
        if (!storageArray || !Array.isArray(storageArray)) return [];
        const sortedArray = [...storageArray];
        sortedArray.sort((a, b) => {
            const nameA = String(a.storage || '').toLowerCase();
            const nameB = String(b.storage || '').toLowerCase();
            return nameA.localeCompare(nameB);
        });
        return sortedArray;
    }

    const table = document.createElement('table');
    table.className = 'w-full text-sm border-collapse table-auto min-w-full';

    const thead = document.createElement('thead');
    thead.innerHTML = `
        <tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 sticky top-0 z-10"> 
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Storage</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Content</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Type</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Shared</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Usage</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Avail</th>
          <th class="text-left p-2 px-3 font-semibold text-gray-700 dark:text-gray-300">Total</th>
        </tr>
      `;
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';

    const sortedNodeNames = Object.keys(storage).sort((a, b) => a.localeCompare(b));

    sortedNodeNames.forEach(nodeName => {
      const nodeStorageData = storage[nodeName]; 

      const nodeHeaderRow = document.createElement('tr');
      nodeHeaderRow.className = 'bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs node-storage-header'; 
      nodeHeaderRow.innerHTML = `
        <td colspan="7" class="p-1.5 px-3">
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
          Node: ${nodeName}
        </td>`;
      tbody.appendChild(nodeHeaderRow);

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

      const sortedNodeStorageData = sortNodeStorageData(nodeStorageData);
      
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
            return `<span data-tooltip="${details.tooltip}" class="storage-tooltip-trigger inline-block ${details.badgeClass} rounded px-1.5 py-0.5 text-xs font-medium mr-1 cursor-default">${ct}</span>`;
        }).join('');

        const usageBarHTML = createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass);

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap text-gray-900 dark:text-gray-100 font-medium">${store.storage || 'N/A'}</td>
            <!-- Add flex items-center for vertical alignment -->
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs flex items-center">${contentBadges || '-'}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${store.type || 'N/A'}</td>
            <td class="p-1 px-2 whitespace-nowrap text-center storage-tooltip-trigger cursor-default" data-tooltip="${sharedIconTooltip}">${sharedIcon}</td>
            <!-- Remove min-width to allow auto-sizing -->
            <!-- Remove whitespace-nowrap from parent TD -->
            <!-- RE-ADD min-width -->
            <td class="p-1 px-2 text-gray-600 dark:text-gray-300 min-w-[250px]">${usageBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-right">${formatBytes(store.avail)}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-right">${formatBytes(store.total)}</td>
        `;
        tbody.appendChild(row);
      });
    }); // End looping through nodes

    table.appendChild(thead);
    table.appendChild(tbody);
    contentDiv.appendChild(table);

    if (tooltipElement) { 
        const storageTbody = table.querySelector('tbody'); // Get the tbody we just created
        if (storageTbody) {
        } // End if storageTbody
    } // End if tooltipElement

  }

  if (tooltipElement) { 
      tooltipElement.classList.remove('duration-100'); 
      tooltipElement.classList.add('duration-50');

      document.body.addEventListener('mouseover', (event) => {
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
          const target = event.target.closest('.metric-tooltip-trigger, .storage-tooltip-trigger');
          if (target) {
              tooltipElement.classList.add('hidden', 'opacity-0');
              tooltipElement.classList.remove('opacity-100');
          }
      });

       document.body.addEventListener('mousemove', (event) => {
           const target = event.target.closest('.metric-tooltip-trigger, .storage-tooltip-trigger');
           if (!tooltipElement.classList.contains('hidden') && target) {
               const offsetX = 10;
               const offsetY = 15;
               tooltipElement.style.left = `${event.pageX + offsetX}px`;
               tooltipElement.style.top = `${event.pageY + offsetY}px`;
           } else if (!tooltipElement.classList.contains('hidden') && !target) {
           }
       });

  } else {
      console.warn('Tooltip element not found, custom tooltips disabled.');
  }

  function refreshDashboardData() {
    dashboardData = [];

    let maxNameLength = 0;
    let maxUptimeLength = 0;

    function calculateAverage(historyArray, key) {
      if (!historyArray || historyArray.length === 0) return null;
      const validEntries = historyArray.filter(entry => typeof entry[key] === 'number' && !isNaN(entry[key]));
      if (validEntries.length === 0) return null;
      const sum = validEntries.reduce((acc, curr) => acc + curr[key], 0);
      return sum / validEntries.length;
    }

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

    const processGuest = (guest, type) => {

        let avgCpu = 0, avgMem = 0, avgDisk = 0;
        let avgDiskReadRate = 0, avgDiskWriteRate = 0, avgNetInRate = 0, avgNetOutRate = 0;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';

        const metrics = (metricsData || []).find(m => 
            m.id === guest.vmid && 
            m.type === guest.type &&
            m.node === guest.node && 
            m.endpointId === guest.endpointId
        ); 

        if (guest.status === 'running' && metrics && metrics.current) { 
            if (!dashboardHistory[guest.id] || !Array.isArray(dashboardHistory[guest.id])) { // Use guest.id (unique combo) as key
               dashboardHistory[guest.id] = [];
            }
            
            const history = dashboardHistory[guest.id]; // Use unique guest.id

            const currentDataPoint = { 
                timestamp: Date.now(), 
                ...metrics.current 
            };
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

        } else { // Guest is stopped, unknown, or metrics not found for it
             if (dashboardHistory[guest.id]) { 
                 delete dashboardHistory[guest.id];
             }
        }

        const name = guest.name || `${guest.type === 'qemu' ? 'VM' : 'CT'} ${guest.vmid}`;
        const uptimeFormatted = formatUptime(guest.uptime);
        if (name.length > maxNameLength) maxNameLength = name.length;
        if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;

        dashboardData.push({
            id: guest.vmid, // Display ID
            uniqueId: guest.id, // Unique ID for history key
            vmid: guest.vmid, 
            name: name, 
            node: guest.node, 
            type: guest.type === 'qemu' ? 'VM' : 'CT', 
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

    const nameColWidth = Math.min(Math.max(maxNameLength * 8 + 16, 100), 300);
    const uptimeColWidth = Math.max(maxUptimeLength * 7 + 16, 80);
    if (htmlElement) {
        htmlElement.style.setProperty('--name-col-width', `${nameColWidth}px`);
        htmlElement.style.setProperty('--uptime-col-width', `${uptimeColWidth}px`);
    }

    // updateDashboardTable(); // Render the table // REMOVED THIS LINE TO FIX RECURSION
  }

  function updateDashboardTable() {
    const tableBody = document.querySelector('#main-table tbody');
    const statusElement = document.getElementById('dashboard-status-text');
    if (!tableBody || !statusElement) {
        console.error('Dashboard table body or status element not found!');
        return;
    }

    refreshDashboardData(); // Ensure dashboardData is up-to-date with latest averages

    // --- Define threshold prefixes used for search term filtering ---
    // const thresholdPrefixes = ['cpu>=', 'memory>=', 'disk>=', 'uptime>=', 'diskread>=', 'diskwrite>=', 'netin>=', 'netout>=']; // REMOVED

    // Get raw search terms, split by comma
    const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];

    // Separate search terms from threshold filter tags
    // const textSearchTerms = rawSearchTerms.filter(term => !thresholdPrefixes.some(prefix => term.startsWith(prefix))); // REMOVED separation
    // Note: We don't actually need to *use* the threshold tags parsed from the search bar for filtering,
    // because the filtering happens based on the actual thresholdState object below.

    let filteredData = dashboardData.filter(guest => {
        // 1. Type Filter
        const typeMatch = filterGuestType === 'all' || (filterGuestType === 'vm' && guest.type === 'VM') || (filterGuestType === 'lxc' && guest.type === 'CT');
        // 2. Status Filter
        const statusMatch = filterStatus === 'all' || guest.status === filterStatus;
        
        // 3. Search Filter (using ONLY text terms, ignore threshold tags)
        const searchMatch = textSearchTerms.length === 0 || textSearchTerms.some(term => 
            (guest.name && guest.name.toLowerCase().includes(term)) || 
            (guest.node && guest.node.toLowerCase().includes(term)) ||
            (guest.vmid && guest.vmid.toString().includes(term)) ||
            (guest.id && guest.id.toString().includes(term)) // Assuming guest.id is the uniqueId
        );
        
        // 4. Threshold Filtering (uses thresholdState, independent of search bar tags)
        let thresholdsMet = true;
        for (const type in thresholdState) {
            const state = thresholdState[type];
            let guestValue;

            if (type === 'cpu') guestValue = guest.cpu * 100; // % value
            else if (type === 'memory') guestValue = guest.memory; // % value
            else if (type === 'disk') guestValue = guest.disk; // % value
            else if (type === 'diskread') guestValue = guest.diskread; // bytes/s
            else if (type === 'diskwrite') guestValue = guest.diskwrite; // bytes/s
            else if (type === 'netin') guestValue = guest.netin; // bytes/s
            else if (type === 'netout') guestValue = guest.netout; // bytes/s
            else continue; // Should not happen

            // Simplified check: Only filter if threshold value is > 0
            if (state.value > 0) {
                // Skip check if guest value is invalid/unavailable (e.g., stopped VM)
                if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) {
                    thresholdsMet = false; // Guest cannot meet a specific threshold if value is missing
                    break;
                }

                // All filters are now effectively '>='
                if (!(guestValue >= state.value)) {
                    thresholdsMet = false;
                    break;
                }
            }
        }
        // --- END Updated Threshold Filtering ---

        return typeMatch && statusMatch && searchMatch && thresholdsMet;
    });

    // Apply sorting
    let sortedData = sortData(filteredData, sortState.main.column, sortState.main.direction, 'main');

    // --- BEGIN Restored Rendering Logic ---
    tableBody.innerHTML = ''; // Clear previous content
    let visibleCount = 0;
    let visibleNodes = new Set();

    if (groupByNode) {
        const nodeGroups = {};
        // Group sorted data by node
        sortedData.forEach(guest => {
            const nodeName = guest.node || 'Unknown Node'; // Handle guests potentially without a node
            if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
            nodeGroups[nodeName].push(guest);
        });

        // Sort node names and render groups
        Object.keys(nodeGroups).sort().forEach(nodeName => {
            visibleNodes.add(nodeName.toLowerCase());
            const nodeHeaderRow = document.createElement('tr');
            nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
            // Ensure colspan matches the number of columns in the main table head (11)
            nodeHeaderRow.innerHTML = `<td colspan="11" class="px-2 py-1">
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
                ${nodeName}
            </td>`;
            tableBody.appendChild(nodeHeaderRow);

            // Render guests within this node group
            nodeGroups[nodeName].forEach(guest => {
                const guestRow = createGuestRow(guest); // Use the existing function
                if (guestRow) { // Check if createGuestRow returned a valid row
                    tableBody.appendChild(guestRow);
                    visibleCount++;
                }
            });
        });
    } else { // Render as a flat list if not grouping
        sortedData.forEach(guest => {
            const guestRow = createGuestRow(guest); // Use the existing function
             if (guestRow) { // Check if createGuestRow returned a valid row
                tableBody.appendChild(guestRow);
                visibleCount++;
                visibleNodes.add((guest.node || 'Unknown Node').toLowerCase());
            }
        });
    }

    // Handle case where no guests match filters
    if (visibleCount === 0) {
        // Generate filter description string
        let filterDescription = [];
        if (filterGuestType !== 'all') filterDescription.push(`Type: ${filterGuestType.toUpperCase()}`);
        if (filterStatus !== 'all') filterDescription.push(`Status: ${filterStatus}`);
        if (textSearchTerms.length > 0) filterDescription.push(`Search: "${textSearchTerms.join(', ')}"`);
        const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
        if (activeThresholds.length > 0) {
            // Generate threshold description without relying on getThresholdFilterTag
            const thresholdTexts = activeThresholds.map(([key, state]) => {
                return `${getReadableThresholdName(key)}>=${formatThresholdValue(key, state.value)}`;
            }); 
            filterDescription.push(`Thresholds: ${thresholdTexts.join(', ')}`);
        }

        let message = "No guests match the current filters";
        if (filterDescription.length > 0) {
            message += ` (${filterDescription.join('; ')})`;
        }
        message += ".";

        tableBody.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
    }

    // Update the status text below the table
    const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
    let statusFilterText = textSearchTerms.length > 0 ? ` | Search: "${textSearchTerms.join(', ')}"` : '';
    const typeLabel = filterGuestType !== 'all' ? filterGuestType.toUpperCase() : '';
    const statusLabel = filterStatus !== 'all' ? filterStatus : '';
    const otherFilters = [typeLabel, statusLabel].filter(Boolean).join('/');
    if (otherFilters) {
        statusFilterText += ` | ${otherFilters}`;
    }
    let statusCountText = ` | Showing ${visibleCount} guests`;
    if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
    statusElement.textContent = statusBaseText + statusFilterText + statusCountText;

    // Update sort UI indicator for the main table
    const mainSortColumn = sortState.main.column;
    const mainHeader = document.querySelector(`#main-table th[data-sort="${mainSortColumn}"]`);
    if (mainHeader) {
        updateSortUI('main-table', mainHeader);
    } else {
        // Fallback or error handling if header not found
        console.warn(`Sort header for column '${mainSortColumn}' not found in main table.`);
    }
    // --- END Restored Rendering Logic ---
  }

  function createGuestRow(guest) {
      const row = document.createElement('tr');
      row.className = `border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 ${guest.status === 'stopped' ? 'opacity-60 grayscale' : ''}`;
      row.setAttribute('data-name', guest.name.toLowerCase());
      row.setAttribute('data-type', guest.type.toLowerCase());
      row.setAttribute('data-node', guest.node.toLowerCase());
      row.setAttribute('data-id', guest.id);

      let cpuBarHTML = '-';
      let memoryBarHTML = '-';
      let diskBarHTML = '-';
      let diskReadFormatted = '-';
      let diskWriteFormatted = '-';
      let netInFormatted = '-';
      let netOutFormatted = '-';

      if (guest.status === 'running') {
        const cpuPercent = Math.round(guest.cpu * 100);
        const memoryPercent = guest.memory;

        const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus).toFixed(1)}/${guest.cpus} cores)` : ''}`;
        const memoryTooltipText = guest.memoryTotal ? `${formatBytesInt(guest.memoryCurrent)} / ${formatBytesInt(guest.memoryTotal)} (${memoryPercent}%)` : `${memoryPercent}%`;

        const cpuColorClass = getUsageColor(cpuPercent);
        const memColorClass = getUsageColor(memoryPercent);

        cpuBarHTML = createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
        memoryBarHTML = createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass);
        
        if (guest.type === 'CT') { // Corrected check: Use 'CT' for containers
            const diskPercent = guest.disk; // Use metric disk usage for LXC
            const diskTooltipText = guest.diskTotal ? `${formatBytesInt(guest.diskCurrent)} / ${formatBytesInt(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
            const diskColorClass = getUsageColor(diskPercent);
            diskBarHTML = createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass);
        } else if (guest.type === 'VM') { 
            if (guest.diskTotal) { // Check if total disk exists and is non-zero
                const totalDiskFormatted = formatBytesInt(guest.diskTotal);
                diskBarHTML = `<span class=\"text-xs text-gray-700 dark:text-gray-200 truncate\">${totalDiskFormatted}</span>`; 
            } else {
                 diskBarHTML = '-'; // Fallback if no total disk reported by API
            }
        } else {
             diskBarHTML = '-'; // Fallback if type unknown
        }

        diskReadFormatted = formatSpeedInt(guest.diskread);
        diskWriteFormatted = formatSpeedInt(guest.diskwrite);
        netInFormatted = formatSpeedInt(guest.netin);
        netOutFormatted = formatSpeedInt(guest.netout);
      }

      const typeIconClass = guest.type === 'VM'
          ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
          : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
      const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === 'VM' ? 'VM' : 'LXC'}</span>`;

      row.innerHTML = `
        <td class="p-1 px-2 whitespace-nowrap truncate" title="${guest.name}">${guest.name}</td>
        <td class="p-1 px-1 text-center">${typeIcon}</td>
        <td class="p-1 px-2 text-center">${guest.id}</td>
        <td class="p-1 px-2 whitespace-nowrap">${guest.status === 'stopped' ? '-' : formatUptime(guest.uptime)}</td>
        <td class="p-1 px-2">${cpuBarHTML}</td>
        <td class="p-1 px-2">${memoryBarHTML}</td>
        <td class="p-1 px-2">${diskBarHTML}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${diskReadFormatted}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${diskWriteFormatted}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${netInFormatted}</td>
        <td class="p-1 px-2 text-right whitespace-nowrap">${netOutFormatted}</td>
      `;

      return row;
  }

  function getHeaderTextForCell(td) {
    try {
        const cellIndex = Array.from(td.parentNode.children).indexOf(td);
        const headerRow = document.querySelector('#main-table thead tr');
        if (!headerRow) {
            console.warn("Header row not found for #main-table");
            return 'Unknown Header';
        }
        const headerCell = headerRow.children[cellIndex];
        if (!headerCell) {
            console.warn(`Header cell not found at index ${cellIndex}`);
            return 'Unknown Header';
        }
        return headerCell.textContent || headerCell.innerText || 'Unknown Header';
    } catch (error) {
        console.error("Error getting header text for cell:", error, td);
        return 'Error Header';
    }
  }

  socket.onAny((eventName, ...args) => {
  });

  socket.on('rawData', (jsonData) => {
    try {
        const data = typeof jsonData === 'string' ? JSON.parse(jsonData) : jsonData;

        nodesData = data.nodes || [];
        vmsData = data.vms || [];
        containersData = data.containers || [];

        if (data.hasOwnProperty('metrics')) {
             metricsData = data.metrics || [];
             if (data.metrics && data.metrics.length > 0) {
             }
        } else {
        }

        if (data.hasOwnProperty('pbs')) {
            pbsDataArray = Array.isArray(data.pbs) ? data.pbs : []; 
        } else {
        }

        updateTabAvailability(); 


        if (!initialDataReceived) {
          initialDataReceived = true;

        }


    } catch (e) {
        console.error('Error processing received rawData:', e, jsonData);
    }
  });

  socket.on('pbsInitialStatus', (pbsStatusArray) => {
      if (Array.isArray(pbsStatusArray)) {
          pbsDataArray = pbsStatusArray.map(statusInfo => ({ 
              ...statusInfo, // includes pbsEndpointId, pbsInstanceName, status
              backupTasks: { recentTasks: [], summary: {} },
              datastores: [],
              verificationTasks: { summary: {} },
              syncTasks: { summary: {} },
              pruneTasks: { summary: {} },
              nodeName: null // Node name isn't known yet
          }));
          updatePbsInfo(pbsDataArray);
          updateTabAvailability(); 
      } else {
          console.warn('[socket] Received non-array data for pbsInitialStatus:', pbsStatusArray);
      }
  });

  function requestFullData() {
    console.log('Requesting full data reload from server...');
    if (loadingOverlay) {
      const loadingText = loadingOverlay.querySelector('p'); // Changed selector from #loading-text to p
      if (loadingText) {
          loadingText.textContent = 'Connected. Reloading data...';
      }
      loadingOverlay.style.display = 'flex'; // Or \'block\', ensure it matches initial display style
    }
    socket.emit('requestData'); // Ensure this uses the correct event name
  }

  function resetThresholds() {
      for (const type in thresholdState) {
          if (sliders[type]) { // Slider
             // If logging is active while resetting, stop it.
             if (isThresholdLoggingActive && toggleLogModeButton) {
                 toggleLogModeButton.click(); // Simulate click to stop logging cleanly
             }

              thresholdState[type].value = 0;
              const sliderElement = sliders[type];
              if (sliderElement) sliderElement.value = 0;
          } else if (thresholdSelects[type]) { // Dropdown Select
              thresholdState[type].value = 0;
              const selectElement = thresholdSelects[type];
              if (selectElement) selectElement.value = 0; // Set to the 'Any' option value
          }

      } // End for loop
      hideSliderTooltip(); // Make sure tooltip is hidden after reset
      // Ensure threshold row is hidden on reset
      isThresholdRowVisible = false;
      updateThresholdRowVisibility();
      // --- END Reset Thresholds Logic (within this func) ---
      updateThresholdIndicator(); // Update badge after reset
  }

  // Reset the main dashboard view
  function resetDashboardView() {
      if (searchInput) searchInput.value = '';
      sortState.main = { column: 'id', direction: 'asc' }; 
      updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]'));
      
      const groupGroupedRadio = document.getElementById('group-grouped');
      if(groupGroupedRadio) groupGroupedRadio.checked = true;
      groupByNode = true;
      
      const filterAllRadio = document.getElementById('filter-all');
      if(filterAllRadio) filterAllRadio.checked = true;
      filterGuestType = 'all'; // Reset type filter state
      
      const statusAllRadio = document.getElementById('filter-status-all');
      if(statusAllRadio) statusAllRadio.checked = true;
      filterStatus = 'all';
      
      // --- BEGIN Reset Thresholds ---
      resetThresholds(); // Call the dedicated reset function
      // --- END Reset Thresholds Section ---

      updateDashboardTable(); // Update table AFTER resetting everything
      if (searchInput) searchInput.blur(); // Blur search input after reset


      const backupStatusAllRadio = document.getElementById('backups-filter-status-all');
      if(backupStatusAllRadio) backupStatusAllRadio.checked = true;
      backupsFilterHealth = 'all';

      const typeAllRadio = document.getElementById('filter-all');
      if(typeAllRadio) typeAllRadio.checked = true;
      filterGuestType = 'all';

      saveFilterState(); // ---> ADDED: Save reset state <---
  }

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
        }
    }
  });

  const resetButton = document.getElementById('reset-filters-button');
  if (resetButton) {
      resetButton.addEventListener('click', resetDashboardView); // Call the same reset function
  } else {
      console.warn('Reset button #reset-filters-button not found.');
  }

  const initialMainSortColumn = sortState.main.column;
  const initialMainHeader = document.querySelector(`#main-table th[data-sort="${initialMainSortColumn}"]`);
  if (initialMainHeader) {
    updateSortUI('main-table', initialMainHeader);
  } else {
      console.warn(`Initial sort header for column '${initialMainSortColumn}' not found in main table.`);
      updateSortUI('main-table', document.querySelector('#main-table th[data-sort="id"]')); // Fallback to ID visual
  }

  function updateAllUITables() {
    updateNodesTable(nodesData);
    refreshDashboardData(); // Process data and update dashboardData array
    updateDashboardTable(); // ADDED: Re-render the main table with refreshed data
    updateStorageInfo(storageData); // Update storage info tab
    updatePbsInfo(pbsDataArray);
    updateBackupsTab(); 

    if (loadingOverlay && loadingOverlay.style.display !== 'none') {
        if (socket.connected) { // ADDED: Check socket connection status
            console.log('[UI Update] Hiding loading overlay.'); // Add log for confirmation
            loadingOverlay.style.display = 'none';
        } else {
        }
    }

    initialDataReceived = true; // Keep this flag for other logic (like the interval trigger)
  }

  async function fetchStorageData() {
    try {
      const response = await fetch('/api/storage');
      if (!response.ok) {
          let serverErrorMsg = `Server responded with status: ${response.status} ${response.statusText}`;
          try {
              const errorJson = await response.json();
              if (errorJson && errorJson.globalError) {
                  serverErrorMsg = errorJson.globalError;
              } else if (errorJson) {
                   serverErrorMsg += ` | Body: ${JSON.stringify(errorJson)}`;
              }
          } catch (parseError) {
          }
          throw new Error(serverErrorMsg); // Throw error to be caught below
      }

      const fetchedData = await response.json();
      storageData = fetchedData; // Update the global variable

    } catch (error) { // Catches fetch errors (network) and the thrown error above
      let finalErrorMessage = 'Failed to load storage data due to an unknown error.';
      if (error instanceof TypeError) {
        finalErrorMessage = `Failed to load storage data due to a network error: ${error.message}`;
        console.error('Network error during storage fetch:', error);
      } else {
        finalErrorMessage = error.message;
        console.error('Error processing storage response:', error);
      }
      console.error(`Storage fetch failed, preserving previous data. Error: ${finalErrorMessage}`);
    }
  }

  setInterval(() => {
    if (initialDataReceived) {
      updateAllUITables();
      checkThresholdViolations(); // Add check after UI update
    }
  }, 2000); // Update UI every 2 seconds (was 2500)

  setInterval(fetchStorageData, 30000); // Changed to 30 seconds
  fetchStorageData(); // Initial fetch on load


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

  const formatPbsTimestamp = (ts) => {
      if (!ts) return 'Never';
      try {
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
      if (s > 0 || parts.length === 0) parts.push(s + 's'); 
      
      return parts.slice(0, 2).join(''); 
  };

  const getPbsStatusIcon = (status) => {
      if (status === 'OK') {
          return '<span class="text-green-500 dark:text-green-400" title="OK">✓</span>'; 
      } else if (status === 'running') {
          return '<span class="inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500" title="Running"></span>'; 
      } else if (status) {
          return `<span class="text-red-500 dark:text-red-400 font-bold" title="${status}">✗</span>`; 
      } else {
          return '<span class="text-gray-400" title="Unknown">?</span>';
      }
  };

  const getPbsGcStatusText = (gcStatus) => {
    if (!gcStatus || gcStatus === 'unknown' || gcStatus === 'N/A') { 
      return '<span class="text-xs text-gray-400">-</span>'; // Changed from "Unknown"
    }
    let colorClass = 'text-gray-600 dark:text-gray-400';
    if (gcStatus.includes('error') || gcStatus.includes('failed')) {
        colorClass = 'text-red-500 dark:text-red-400';
    } else if (gcStatus === 'OK') {
        colorClass = 'text-green-500 dark:text-green-400';
    }
    return `<span class="text-xs ${colorClass}">${gcStatus}</span>`;
  };

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

      failedEl.classList.toggle('font-bold', (summary.failed ?? 0) > 0);

    } else {
      okEl.textContent = '-';
      failedEl.textContent = '-';
      totalEl.textContent = '-';
      lastOkEl.textContent = '-';
      lastFailedEl.textContent = '-';
      failedEl.classList.remove('font-bold');
    }
  }

  const parsePbsTaskTarget = (task) => {
    const workerId = task.worker_id || task.id || ''; // e.g., guests:ct/103/681078F1 or hosts:host/bkp-2024-05-15 or guests::ct/103
    const taskType = task.worker_type || task.type || ''; // e.g., backup, verify, prune, garbage_collection

    let displayTarget = workerId;

    if (taskType === 'backup' || taskType === 'verify') {
      const parts = workerId.split(':');
      if (parts.length >= 2) {
        const targetPart = parts[1]; // e.g., ct/103/681078F1 or host/bkp-2024-05-15
        const targetSubParts = targetPart.split('/');
        if (targetSubParts.length >= 2) {
            const guestType = targetSubParts[0]; // ct or vm or host
            const guestId = targetSubParts[1]; // 103 or bkp-2024-05-15
            displayTarget = `${guestType}/${guestId}`;
        }
      }
    } else if (taskType === 'prune' || taskType === 'garbage_collection') {
        const parts = workerId.split('::'); // Try double colon first for prune groups
        if (parts.length === 2) {
            displayTarget = `Prune ${parts[0]} (${parts[1]})`; // e.g., Prune guests (ct/103)
        } else {
            const singleColonParts = workerId.split(':'); // Fallback for GC datastore
             if (singleColonParts.length === 1 && workerId !== '') { // GC often just has datastore name
                 displayTarget = `GC ${workerId}`;
             } else if (singleColonParts.length >= 2) {
                 displayTarget = `Prune ${singleColonParts[0]} (${singleColonParts[1]})` // Fallback prune format?
             }
        }
    } else if (taskType === 'sync') {
        displayTarget = `Sync Job: ${workerId}`;
    }


    return displayTarget; // Return the parsed or original string
  };

  function populatePbsTaskTable(parentSectionElement, fullTasksArray) {
    if (!parentSectionElement) {
        console.warn('[PBS UI] Parent element not found for task table');
        return;
    }
    const tableBody = parentSectionElement.querySelector('tbody');
    const showMoreButton = parentSectionElement.querySelector('.pbs-show-more');
    const noTasksMessage = parentSectionElement.querySelector('.pbs-no-tasks');

    if (!tableBody) {
        console.warn('[PBS UI] Table body not found within', parentSectionElement);
        return; // Exit if table body doesn't exist
    }

    tableBody.innerHTML = '';

    const tasks = fullTasksArray || []; // Ensure tasks is an array
    let displayedTasks = tasks.slice(0, INITIAL_PBS_TASK_LIMIT);

    if (tasks.length === 0) {
        if (noTasksMessage) noTasksMessage.classList.remove('hidden');
        if (showMoreButton) showMoreButton.classList.add('hidden');
    } else {
        if (noTasksMessage) noTasksMessage.classList.add('hidden');

        displayedTasks.forEach(task => {
            const target = parsePbsTaskTarget(task); // Use the new parsing function
            const statusIcon = getPbsStatusIcon(task.status);
            const startTime = task.startTime ? formatPbsTimestamp(task.startTime) : 'N/A';
            const duration = task.duration !== null ? formatDuration(task.duration) : 'N/A';
            const upid = task.upid || 'N/A';
            const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

            const row = document.createElement('tr');
            row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150 ease-in-out';
            row.innerHTML = `
                <td class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300">${target}</td>
                <td class="px-4 py-2 text-sm text-center">${statusIcon}</td>
                <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${startTime}</td>
                <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${duration}</td>
                <td class="px-4 py-2 text-xs font-mono text-gray-400 dark:text-gray-500 truncate" title="${upid}">${shortUpid}</td>
            `; // Display shortened UPID, full UPID in title
            tableBody.appendChild(row);
        });

        if (showMoreButton) {
            if (tasks.length > INITIAL_PBS_TASK_LIMIT) {
                showMoreButton.classList.remove('hidden');
                const remainingCount = tasks.length - INITIAL_PBS_TASK_LIMIT;
                showMoreButton.textContent = `Show More (${remainingCount} older)`;
                if (!showMoreButton.dataset.handlerAttached) {
                    showMoreButton.addEventListener('click', () => {
                        tasks.slice(INITIAL_PBS_TASK_LIMIT).forEach(task => {
                           const target = parsePbsTaskTarget(task); // Use the new parsing function
                           const statusIcon = getPbsStatusIcon(task.status);
                           const startTime = task.startTime ? formatPbsTimestamp(task.startTime) : 'N/A';
                           const duration = task.duration !== null ? formatDuration(task.duration) : 'N/A';
                           const upid = task.upid || 'N/A';
                           const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

                           const row = document.createElement('tr');
                           row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150 ease-in-out';
                           row.innerHTML = `
                               <td class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300">${target}</td>
                               <td class="px-4 py-2 text-sm text-center">${statusIcon}</td>
                               <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${startTime}</td>
                               <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${duration}</td>
                               <td class="px-4 py-2 text-xs font-mono text-gray-400 dark:text-gray-500 truncate" title="${upid}">${shortUpid}</td>
                           `; // Display shortened UPID, full UPID in title
                            tableBody.appendChild(row);
                        });
                        showMoreButton.classList.add('hidden'); // Hide after showing all
                    });
                     showMoreButton.dataset.handlerAttached = 'true'; // Mark handler as attached
                }
            } else {
                showMoreButton.classList.add('hidden');
            }
        }
    }
}

  function updatePbsInfo(pbsArray) {
    const container = document.getElementById('pbs-instances-container');
    if (!container) {
        console.error("PBS container element #pbs-instances-container not found!");
        return;
    }

    if (!pbsArray || pbsArray.length === 0) {
        container.innerHTML = ''; // Clear previous content
        const placeholder = document.createElement('p');
        placeholder.className = 'text-gray-500 dark:text-gray-400 p-4 text-center text-sm';
        placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
        container.appendChild(placeholder);
        return; // Stop processing for this tab
    }

    const loadingMessage = document.getElementById('pbs-loading-message');
    if (loadingMessage) {
        loadingMessage.remove();
    }
    
    const notConfiguredBanner = container.querySelector('.pbs-not-configured-banner');
    if (notConfiguredBanner) {
        notConfiguredBanner.remove();
    }


    const currentInstanceIds = new Set(); 

    const createSummaryCard = (type, title, summaryData) => {
        const card = document.createElement('div');
        const summary = summaryData?.summary || {};
        const ok = summary.ok ?? '-';
        const failed = summary.failed ?? '-';
        const total = summary.total ?? '-';
        const lastOk = formatPbsTimestamp(summary.lastOk);
        const lastFailed = formatPbsTimestamp(summary.lastFailed);
        const failedStyle = (failed > 0) ? 'font-bold text-red-600 dark:text-red-400' : 'text-red-600 dark:text-red-400 font-semibold';
        
        const highlightClass = (failed > 0) ? 'border-l-4 border-red-500 dark:border-red-400' : 'border-l-4 border-transparent'; // Use transparent border normally
        card.className = `border border-gray-200 dark:border-gray-700 rounded p-3 bg-gray-100/50 dark:bg-gray-700/50 ${highlightClass}`;

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

     const createTaskTableHTML = (tableId, title, idColumnHeader) => {
        const tbodyId = tableId.replace('-table-', '-tbody-');
        const toggleButtonContainerId = tableId.replace('-table', '-toggle-container');
        return `
        <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Recent ${title} Tasks</h4>
        <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
            <table id="${tableId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                <thead class="text-xs text-gray-700 dark:text-gray-300 uppercase bg-gray-100 dark:bg-gray-700/50 sticky top-0"> 
                    <tr>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">${idColumnHeader}</th>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">Status</th>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">Start Time</th>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">Duration</th>
                        <th scope="col" class="px-4 py-2 text-left font-semibold">UPID</th>
                    </tr>
                </thead>
                <tbody id="${tbodyId}" class="pbs-task-tbody divide-y divide-gray-200 dark:divide-gray-700">
                     <!-- Populated by JS -->
                </tbody>
            </table>
        </div>
        <div id="${toggleButtonContainerId}" class="pbs-toggle-button-container pt-1 text-right"></div>
        `;
    };

    /*
     const setupPbsSortListener = (tableId, sortStateKey, pbsInstance) => { 
     };
    */

    pbsArray.forEach((pbsInstance, index) => {
      const rawInstanceId = pbsInstance.pbsEndpointId || `instance-${index}`;
      const instanceId = sanitizeForId(rawInstanceId); // Use sanitized ID for elements
      const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
      const instanceElementId = `pbs-instance-${instanceId}`;
      currentInstanceIds.add(instanceElementId); 

      let instanceWrapper = document.getElementById(instanceElementId);
      let detailsContainer, dsTableBody, instanceTitleElement; 

      let statusText = 'Loading...';
      let showDetails = false;
      let statusColorClass = 'text-gray-600 dark:text-gray-400';
      switch (pbsInstance.status) {
          case 'configured':
              statusText = `Configured, attempting connection...`; // Simplified
              statusColorClass = 'text-gray-600 dark:text-gray-400';
              break;
          case 'ok':
              statusText = `Status: OK`; // Simplified
              statusColorClass = 'text-green-600 dark:text-green-400';
              showDetails = true;
              break;
          case 'error':
              statusText = `Error: ${pbsInstance.errorMessage || 'Connection failed'}`; // Simplified
              statusColorClass = 'text-red-600 dark:text-red-400';
              break;
          default:
              statusText = `Status: ${pbsInstance.status || 'Unknown'}`;
              break;
      }

      let overallHealth = 'ok'; // Assume ok initially
      let healthTitle = 'OK';
      if (pbsInstance.status === 'error') {
          overallHealth = 'error';
          healthTitle = `Error: ${pbsInstance.errorMessage || 'Connection failed'}`;
      } else if (pbsInstance.status !== 'ok') {
          overallHealth = 'warning'; // Configured or unknown status
          healthTitle = 'Connecting or unknown status';
      } else {
          const highUsageDatastore = (pbsInstance.datastores || []).find(ds => {
              const totalBytes = ds.total || 0;
              const usedBytes = ds.used || 0;
              const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
              return usagePercent > 85; // Warning threshold
          });
          if (highUsageDatastore) {
              overallHealth = 'warning';
              healthTitle = `Warning: Datastore ${highUsageDatastore.name} usage high (${Math.round((highUsageDatastore.used / highUsageDatastore.total) * 100)}%)`;
          }

          if (overallHealth !== 'error') {
              const hasFailures = [
                  pbsInstance.backupTasks,
                  pbsInstance.verificationTasks,
                  pbsInstance.syncTasks,
                  pbsInstance.pruneTasks
              ].some(taskGroup => (taskGroup?.summary?.failed ?? 0) > 0);

              if (hasFailures) {
                  overallHealth = 'error'; // Treat any failure in the last 7 days as an error state for the badge
                  healthTitle = 'Error: One or more recent tasks failed';
              }
          }
      }

      const createHealthBadgeHTML = (health, title) => {
          let colorClass = 'bg-gray-400 dark:bg-gray-500'; // Default/unknown
          if (health === 'ok') colorClass = 'bg-green-500';
          else if (health === 'warning') colorClass = 'bg-yellow-500';
          else if (health === 'error') colorClass = 'bg-red-500';
          return `<span title="${title}" class="inline-block w-3 h-3 ${colorClass} rounded-full mr-2 flex-shrink-0"></span>`;
      };

      if (instanceWrapper) {
          detailsContainer = instanceWrapper.querySelector(`#pbs-details-${instanceId}`);
          instanceTitleElement = instanceWrapper.querySelector('h3'); // Find the existing h3

          if (instanceTitleElement) {
              instanceTitleElement.innerHTML = `${createHealthBadgeHTML(overallHealth, healthTitle)}${instanceName}`;
          }

          if (detailsContainer) {
               dsTableBody = detailsContainer.querySelector(`#pbs-ds-tbody-${instanceId}`);
               if (dsTableBody) {
                   dsTableBody.innerHTML = ''; 
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
                  } else { 
                       dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                  }
               }

               const summariesSection = detailsContainer.querySelector(`#pbs-summaries-section-${instanceId}`);
               if (summariesSection) {
                 summariesSection.innerHTML = ''; // Clear existing cards
                 summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
                 summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
                 summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
                 summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
               }

              if (showDetails) {
                  const backupSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="backup"]');
                  if (backupSection) populatePbsTaskTable(backupSection, pbsInstance.backupTasks?.recentTasks);
                  
                  const verifySection = detailsContainer.querySelector('.pbs-task-section[data-task-type="verify"]');
                  if (verifySection) populatePbsTaskTable(verifySection, pbsInstance.verificationTasks?.recentTasks);
                  
                  const syncSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="sync"]');
                  if (syncSection) populatePbsTaskTable(syncSection, pbsInstance.syncTasks?.recentTasks);
                  
                  const pruneGcSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="prunegc"]');
                  if (pruneGcSection) populatePbsTaskTable(pruneGcSection, pbsInstance.pruneTasks?.recentTasks);

              } else {
                  const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`);
                  if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                  const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`);
                  if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                  const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`);
                  if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                  const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`);
                  if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
              }

              detailsContainer.classList.toggle('hidden', !showDetails);
          } // End if(detailsContainer)

      } else {
          instanceWrapper = document.createElement('div');
          instanceWrapper.className = 'pbs-instance-section border border-gray-200 dark:border-gray-700 rounded p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30';
          instanceWrapper.id = instanceElementId;

          const headerDiv = document.createElement('div');
          headerDiv.className = 'flex justify-between items-center mb-3';
          instanceTitleElement = document.createElement('h3'); // Assign to instanceTitleElement
          instanceTitleElement.className = 'text-lg font-semibold text-gray-800 dark:text-gray-200 flex items-center'; // Corrected: Use instanceTitleElement
          instanceTitleElement.innerHTML = `${createHealthBadgeHTML(overallHealth, healthTitle)}${instanceName}`;
          headerDiv.appendChild(instanceTitleElement);
          instanceWrapper.appendChild(headerDiv);

          detailsContainer = document.createElement('div');
          detailsContainer.className = `pbs-instance-details space-y-4 ${showDetails ? '' : 'hidden'}`;
          detailsContainer.id = `pbs-details-${instanceId}`;

          const dsSection = document.createElement('div');
          dsSection.id = `pbs-ds-section-${instanceId}`;
          dsSection.innerHTML = `
              <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Datastores</h4>
              <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
                  <table id="pbs-ds-table-${instanceId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                       <thead class="text-xs text-gray-700 dark:text-gray-300 uppercase bg-gray-100 dark:bg-gray-700/50"> 
                           <tr> 
                               <th scope="col" class="px-4 py-2 text-left font-semibold dark:text-gray-300">Name</th> 
                               <th scope="col" class="px-4 py-2 text-left font-semibold dark:text-gray-300">Path</th> 
                               <th scope="col" class="px-4 py-2 text-right font-semibold dark:text-gray-300">Used</th> 
                               <th scope="col" class="px-4 py-2 text-right font-semibold dark:text-gray-300">Available</th> 
                               <th scope="col" class="px-4 py-2 text-right font-semibold dark:text-gray-300">Total</th> 
                               <th scope="col" class="px-4 py-2 text-center font-semibold dark:text-gray-300">Usage</th> 
                               <th scope="col" class="px-4 py-2 text-center font-semibold dark:text-gray-300">GC Status</th> 
                           </tr> 
                       </thead>
                      <tbody id="pbs-ds-tbody-${instanceId}" class="divide-y divide-gray-200 dark:divide-gray-700"></tbody>
                  </table>
              </div>`;
          detailsContainer.appendChild(dsSection);

          const summariesSection = document.createElement('div');
          summariesSection.id = `pbs-summaries-section-${instanceId}`;
          summariesSection.className = 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4';
          summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
          summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
          summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
          summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
          detailsContainer.appendChild(summariesSection);

           const recentBackupTasksSection = document.createElement('div');
           recentBackupTasksSection.className = 'pbs-task-section'; // Add class
           recentBackupTasksSection.dataset.taskType = 'backup'; // Add data attribute
           recentBackupTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-backup-tasks-table-${instanceId}`, 'Backup', 'Guest');
           detailsContainer.appendChild(recentBackupTasksSection);
           
           const recentVerifyTasksSection = document.createElement('div');
           recentVerifyTasksSection.className = 'pbs-task-section'; // Add class
           recentVerifyTasksSection.dataset.taskType = 'verify'; // Add data attribute
           recentVerifyTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-verify-tasks-table-${instanceId}`, 'Verification', 'Guest/Group');
           detailsContainer.appendChild(recentVerifyTasksSection);
           
           const recentSyncTasksSection = document.createElement('div');
           recentSyncTasksSection.className = 'pbs-task-section'; // Add class
           recentSyncTasksSection.dataset.taskType = 'sync'; // Add data attribute
           recentSyncTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-sync-tasks-table-${instanceId}`, 'Sync', 'Job ID');
           detailsContainer.appendChild(recentSyncTasksSection);
           
           const recentPruneGcTasksSection = document.createElement('div');
           recentPruneGcTasksSection.className = 'pbs-task-section'; // Add class
           recentPruneGcTasksSection.dataset.taskType = 'prunegc'; // Add data attribute
           recentPruneGcTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-prunegc-tasks-table-${instanceId}`, 'Prune/GC', 'Datastore/Group');
           detailsContainer.appendChild(recentPruneGcTasksSection);

          instanceWrapper.appendChild(detailsContainer);
          container.appendChild(instanceWrapper); // Append new instance to DOM

          dsTableBody = instanceWrapper.querySelector(`#pbs-ds-tbody-${instanceId}`); // Find the newly created body
           if (dsTableBody) {
               if (showDetails && pbsInstance.datastores) {
                    if (pbsInstance.datastores.length === 0) { dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">No PBS datastores found or accessible.</td></tr>`; }
                    else { 
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
              const backupSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="backup"]');
              if (backupSection) populatePbsTaskTable(backupSection, pbsInstance.backupTasks?.recentTasks);
              
              const verifySection = detailsContainer.querySelector('.pbs-task-section[data-task-type="verify"]');
              if (verifySection) populatePbsTaskTable(verifySection, pbsInstance.verificationTasks?.recentTasks);
              
              const syncSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="sync"]');
              if (syncSection) populatePbsTaskTable(syncSection, pbsInstance.syncTasks?.recentTasks);
              
              const pruneGcSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="prunegc"]');
              if (pruneGcSection) populatePbsTaskTable(pruneGcSection, pbsInstance.pruneTasks?.recentTasks);

          } else {
              const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`);
              if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
              const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`);
              if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
              const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`);
              if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
              const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`);
              if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
          }

      } // End Upsert Logic

  }); // End forEach pbsInstance

  container.querySelectorAll('.pbs-instance-section').forEach(el => {
      if (!currentInstanceIds.has(el.id)) {
          el.remove();
      }
  });

}

  const pbsInstancesContainer = document.getElementById('pbs-instances-container');
  if (pbsInstancesContainer) {
      pbsInstancesContainer.addEventListener('click', (event) => {
          const button = event.target.closest('.pbs-toggle-button');
          if (!button) return; // Click wasn't on a toggle button

          const targetTbodyId = button.dataset.targetTbodyId;
          const tbody = document.getElementById(targetTbodyId);
          if (!tbody) {
              console.error("Target tbody not found for toggle button:", targetTbodyId);
              return;
          }

          const parentSection = tbody.closest('.pbs-task-section');
          if (!parentSection) {
              console.error("Parent task section (.pbs-task-section) not found for tbody:", targetTbodyId);
              return;
          }

          const isCurrentlyExpanded = tbody.dataset.isExpanded === 'true';
          const fullTasks = JSON.parse(tbody.dataset.fullTasks || '[]');

          tbody.dataset.isExpanded = isCurrentlyExpanded ? 'false' : 'true';

          populatePbsTaskTable(parentSection, fullTasks);
      });
  } else {
      console.warn("PBS instances container not found, toggle functionality will not work.");
  }

  function updateBackupsTab() {
      const tableContainer = document.getElementById('backups-table-container');
      const tableBody = document.getElementById('backups-overview-tbody');
      const loadingMsg = document.getElementById('backups-loading-message');
      const noDataMsg = document.getElementById('backups-no-data-message');
      const statusTextElement = document.getElementById('backups-status-text'); 

      if (!tableContainer || !tableBody || !loadingMsg || !noDataMsg || !statusTextElement) { 
          console.error("UI elements for Backups tab not found!");
          return;
      }


      const allGuests = [
          ...(vmsData || []), 
          ...(containersData || [])
      ];

      if (!initialDataReceived || !pbsDataArray) { 
          loadingMsg.classList.remove('hidden');
          tableContainer.classList.add('hidden');
          noDataMsg.classList.add('hidden');
          return;
      }

      if (allGuests.length === 0) {
          loadingMsg.classList.add('hidden');
          tableContainer.classList.add('hidden');
          noDataMsg.textContent = "No Proxmox guests (VMs/Containers) found.";
          noDataMsg.classList.remove('hidden');
          return;
      }

      loadingMsg.classList.add('hidden');

      const backupStatusByGuest = [];
      const now = Math.floor(Date.now() / 1000);
      const sevenDaysAgo = now - (7 * 24 * 60 * 60);
      const threeDaysAgo = now - (3 * 24 * 60 * 60);

      const allRecentBackupTasks = (pbsDataArray || []).flatMap(pbs =>
          (pbs.backupTasks?.recentTasks || []).map(task => ({ // Ensure recentTasks exists
              ...task,
              guestId: task.id?.split('/')[1] || null,
              guestTypePbs: task.id?.split('/')[0] || null, // vm or ct
              pbsInstanceName: pbs.pbsInstanceName // Add instance name for reference
          }))
      );

      const allSnapshots = (pbsDataArray || []).flatMap(pbsInstance =>
          (pbsInstance.datastores || []).flatMap(ds =>
              (ds.snapshots || []).map(snap => ({
                  ...snap,
                  pbsInstanceName: pbsInstance.pbsInstanceName,
                  datastoreName: ds.name,
                  backupType: snap['backup-type'],
                  backupVMID: snap['backup-id']
              }))
          )
      );

      allGuests.forEach(guest => {
          const guestId = String(guest.vmid);
          const guestTypePve = guest.type === 'qemu' ? 'vm' : 'ct';

          const guestSnapshots = allSnapshots.filter(snap =>
              String(snap.backupVMID) === guestId && snap.backupType === guestTypePve
          );
          const totalBackups = guestSnapshots.length;
          const latestSnapshot = guestSnapshots.reduce((latest, snap) => {
              return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
          }, null);
          const latestSnapshotTime = latestSnapshot ? latestSnapshot['backup-time'] : null;

          const guestTasks = allRecentBackupTasks.filter(task =>
              task.guestId === guestId && task.guestTypePbs === guestTypePve
          );
          const latestTask = guestTasks.reduce((latest, task) => {
             return (!latest || (task.startTime && task.startTime > latest.startTime)) ? task : latest;
          }, null);

          let healthStatus = 'none';
          let displayTimestamp = latestSnapshotTime; // Default to snapshot time

          if (latestTask) {
              displayTimestamp = latestTask.startTime; // Prefer task start time if available
              if (latestTask.status === 'OK') {
                  if (latestTask.startTime >= threeDaysAgo) {
                      healthStatus = 'ok';
                  } else if (latestTask.startTime >= sevenDaysAgo) {
                      healthStatus = 'stale'; // Successful but 3-7 days old
                  } else {
                      healthStatus = 'old'; // Successful but > 7 days old
                  }
              } else {
                  healthStatus = 'failed';
              }
          } else if (latestSnapshotTime) {
               if (latestSnapshotTime >= threeDaysAgo) {
                   healthStatus = 'ok'; // Treat as OK if snapshot is recent, even without recent task info
               } else if (latestSnapshotTime >= sevenDaysAgo) {
                   healthStatus = 'stale'; // Keep as stale if snapshot is 3-7 days old
               } else {
                   healthStatus = 'old'; // Treat as old if snapshot > 7d, no task
               }
          } else {
              healthStatus = 'none';
              displayTimestamp = null; // Ensure no timestamp shown
          }

          backupStatusByGuest.push({
              guestName: guest.name || `Guest ${guest.vmid}`,
              guestId: guest.vmid,
              guestType: guest.type === 'qemu' ? 'VM' : 'LXC',
              node: guest.node,
              guestPveStatus: guest.status, // Add the PVE status here
              latestBackupTime: displayTimestamp, // Use determined display timestamp
              pbsInstanceName: latestSnapshot?.pbsInstanceName || latestTask?.pbsInstanceName || 'N/A',
              datastoreName: latestSnapshot?.datastoreName || 'N/A', // Only snapshots know datastore
              totalBackups: totalBackups,
              backupHealthStatus: healthStatus // Store the calculated health
          });
      });


      const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value.toLowerCase() : '';
      const backupsSearchTerms = currentBackupsSearchTerm.split(',').map(term => term.trim()).filter(term => term);
      
      const filteredBackupStatus = backupStatusByGuest.filter(item => {
          const healthMatch = (backupsFilterHealth === 'all') ||
                              (backupsFilterHealth === 'ok' && (item.backupHealthStatus === 'ok' || item.backupHealthStatus === 'stale')) || // OK includes Stale for filtering
                              (backupsFilterHealth === 'warning' && (item.backupHealthStatus === 'old')) || // Warning = Old
                              (backupsFilterHealth === 'error' && item.backupHealthStatus === 'failed') || // Error = Failed
                              (backupsFilterHealth === 'none' && item.backupHealthStatus === 'none');
          if (!healthMatch) return false;

          const typeMatch = (backupsFilterGuestType === 'all') ||
                            (backupsFilterGuestType === 'vm' && item.guestType === 'VM') ||
                            (backupsFilterGuestType === 'lxc' && item.guestType === 'LXC'); // Use 'lxc' to match radio value and data type
          if (!typeMatch) return false;
          
          if (backupsSearchTerms.length > 0) {
              const nameMatch = backupsSearchTerms.some(term =>
                  (item.guestName?.toLowerCase() || '').includes(term) ||
                  (item.node?.toLowerCase() || '').includes(term) ||
                  (item.guestId?.toString() || '').includes(term)
              );
              if (!nameMatch) return false; // Early exit if search doesn't match
          }

          return true;
      });

      const sortedBackupStatus = sortData(filteredBackupStatus, sortState.backups.column, sortState.backups.direction, 'backups');

      tableBody.innerHTML = ''; // Clear previous content
      let visibleCount = 0; // Added counter
      if (sortedBackupStatus.length > 0) { 
          sortedBackupStatus.forEach(guestStatus => { 
              const row = tableBody.insertRow();
              row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700 hover:shadow-md hover:-translate-y-px ${guestStatus.guestPveStatus === 'stopped' ? 'opacity-60 grayscale' : ''}`;
              
              const latestBackupFormatted = guestStatus.latestBackupTime 
                  ? formatPbsTimestamp(guestStatus.latestBackupTime) 
                  : '<span class="text-gray-400">No backups found</span>';

              let healthIndicator = '';
              switch (guestStatus.backupHealthStatus) {
                  case 'ok':
                      healthIndicator = '<span class="text-green-600 dark:text-green-400" title="OK">●</span>';
                      break;
                  case 'stale':
                      healthIndicator = '<span class="text-yellow-600 dark:text-yellow-400" title="Stale">●</span>';
                      break;
                  case 'failed':
                      healthIndicator = '<span class="text-red-600 dark:text-red-400 font-bold" title="Failed">✖</span>';
                      break;
                  case 'old':
                       healthIndicator = '<span class="text-orange-600 dark:text-orange-400" title="Old">●</span>';
                       break;
                  case 'none':
                      healthIndicator = '<span class="text-gray-400 dark:text-gray-500" title="None">-</span>';
                      break;
              }

              const typeIconClass = guestStatus.guestType === 'VM'
                  ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium' 
                  : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
              const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;

              row.innerHTML = `
                  <td class="p-1 px-2 whitespace-nowrap text-center">${healthIndicator}</td> <!-- Health Status -->
                  <td class="p-1 px-2 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100" title="${guestStatus.guestName}">${guestStatus.guestName}</td>
                  <td class="p-1 px-2 text-center text-gray-500 dark:text-gray-400">${guestStatus.guestId}</td>
                  <td class="p-1 px-2 text-center">${typeIcon}</td> <!-- Use icon -->
                  <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.node}</td>
                  <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${latestBackupFormatted}</td> 
                  <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.pbsInstanceName}</td>
                  <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${guestStatus.datastoreName}</td>
                  <td class="p-1 px-2 text-center text-gray-500 dark:text-gray-400">${guestStatus.totalBackups}</td>
              `;
              visibleCount++; // Increment counter
          });

          loadingMsg.classList.add('hidden');
          noDataMsg.classList.add('hidden');
          tableContainer.classList.remove('hidden'); // Show the table
      } else {
          loadingMsg.classList.add('hidden');
          tableContainer.classList.add('hidden');
          let emptyMessage = "No backup information found for any guests.";
          if (backupStatusByGuest.length === 0) { // Check original data length before filtering
              if (allGuests.length === 0) {
                   emptyMessage = "No Proxmox guests (VMs/Containers) found.";
              } else {
                  emptyMessage = "No backup information found for any guests.";
              }
          } 
          else if (filteredBackupStatus.length === 0) { // Check filtered data length
               const typeFilterText = backupsFilterGuestType === 'all' ? '' : `Type: ${backupsFilterGuestType.toUpperCase()}`;
               const filtersApplied = [typeFilterText].filter(Boolean).join(', '); // Only include type filter
               
               if (filtersApplied) {
                 emptyMessage = `No guests found matching the selected filters (${filtersApplied}).`;
               } else {
                 emptyMessage = "No guests with backup information found."; 
               }
          }
          if (filteredBackupStatus.length === 0 && backupsSearchTerms.length > 0) {
             emptyMessage = `No guests found matching search "${currentBackupsSearchTerm}".`;
             if (filtersApplied) {
                 emptyMessage += ` and filters (${filtersApplied})`;
             }
          }
          noDataMsg.textContent = emptyMessage;
          noDataMsg.classList.remove('hidden');
      }
       const backupsSortColumn = sortState.backups.column;
       const backupsHeader = document.querySelector(`#backups-overview-table th[data-sort="${backupsSortColumn}"]`);
       updateSortUI('backups-overview-table', backupsHeader);
       if (statusTextElement) {
           const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
           let statusFilterText = currentBackupsSearchTerm ? ` | Filter: "${currentBackupsSearchTerm}"` : '';
           const typeFilterLabel = backupsFilterGuestType !== 'all' ? backupsFilterGuestType.toUpperCase() : '';
           const healthFilterLabel = backupsFilterHealth !== 'all' ? backupsFilterHealth.charAt(0).toUpperCase() + backupsFilterHealth.slice(1) : ''; // Capitalize health status
           const otherFilters = [typeFilterLabel, healthFilterLabel].filter(Boolean).join('/');
           if (otherFilters) {
               statusFilterText += ` | ${otherFilters}`;
           }
           let statusCountText = ` | Showing ${visibleCount} guests`;
           statusTextElement.textContent = statusBaseText + statusFilterText + statusCountText;
       }
  }

  function updateTabAvailability() {
      const pbsTab = document.querySelector('.tab[data-tab="pbs"]');
      const backupsTab = document.querySelector('.tab[data-tab="backups"]');

      if (!pbsTab || !backupsTab) {
          console.warn("PBS or Backups tab element not found for availability update.");
          return;
      }

      const isPbsAvailable = pbsDataArray && pbsDataArray.length > 0 && pbsDataArray.some(pbs => pbs.status === 'ok');

      const disabledClasses = ['opacity-50', 'cursor-not-allowed', 'pointer-events-none'];
      const enabledClasses = ['hover:bg-gray-200', 'dark:hover:bg-gray-700', 'cursor-pointer']; // Ensure hover/pointer restored

      [pbsTab, backupsTab].forEach(tab => {
          if (!isPbsAvailable) {
              tab.classList.add(...disabledClasses);
              tab.classList.remove(...enabledClasses);
              tab.setAttribute('title', 'Requires PBS integration to be configured and connected.');
              tab.classList.remove('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700');
              tab.classList.add('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent');

          } else {
              tab.classList.remove(...disabledClasses);
              tab.classList.add(...enabledClasses);
              tab.removeAttribute('title');
          }
      });
  }

  function resetBackupsView() {
      console.log('Resetting backups view...');
      if (backupsSearchInput) backupsSearchInput.value = '';
      backupsSearchTerm = ''; // Clear state variable if you were using one
      
      const backupTypeAllRadio = document.getElementById('backups-filter-type-all');
      if(backupTypeAllRadio) backupTypeAllRadio.checked = true;
      backupsFilterGuestType = 'all';

      const backupStatusAllRadio = document.getElementById('backups-filter-status-all');
      if(backupStatusAllRadio) backupStatusAllRadio.checked = true;
      backupsFilterHealth = 'all';

      sortState.backups = { column: 'latestBackupTime', direction: 'desc' }; // Default sort
      const initialBackupsHeader = document.querySelector('#backups-overview-table th[data-sort="latestBackupTime"]');
      updateSortUI('backups-overview-table', initialBackupsHeader);

      updateBackupsTab();
      saveFilterState();
  }

  if (resetBackupsButton) {
      resetBackupsButton.addEventListener('click', resetBackupsView);
  }

  if (backupsTabContent) { // Reuse for keydown listener
      backupsTabContent.addEventListener('keydown', (event) => {
          if (event.key === 'Escape') {
              if (backupsTabContent.contains(document.activeElement)) {
                  resetBackupsView();
              }
          }
      });
  }



  const typeAllRadio = document.getElementById('filter-all');
  if(typeAllRadio) typeAllRadio.checked = true;
  filterGuestType = 'all'; // Keep resetting the main dashboard type filter here


  updateDashboardTable(); // Trigger dashboard update
  if (searchInput) searchInput.blur(); // Blur search input after reset
  saveFilterState(); // Save potentially changed main dashboard state

// --- START Threshold Logging Functions ---

  function startThresholdLogging() {
    /* // REMOVED Check: Allow multiple logs
    if (Object.keys(activeLogSessions).length > 0) {
        console.warn("Threshold logging already active. Stop the current session first.");
        // Optionally provide user feedback here (e.g., flash the button)
        return;
    }
    */

    // --- Get Duration --- 
    const durationMinutes = parseInt(logDurationSelect.value); // Use select value
    if (isNaN(durationMinutes) || durationMinutes <= 0) {
        alert("Please select a valid duration."); 
        logDurationSelect.focus();
        return;
    }
    const durationMs = durationMinutes * 60 * 1000;
    // --- End Get Duration --- 

    // --- Snapshot current filter state --- 
    const snapshottedThresholds = {};
    let activeThresholdCount = 0;
    let criteriaDescThresholds = [];
    for (const type in thresholdState) {
        const value = thresholdState[type].value;
        if (value > 0) {
            snapshottedThresholds[type] = value;
            activeThresholdCount++;
            criteriaDescThresholds.push(`${getReadableThresholdName(type)}>=${formatThresholdValue(type, value)}`);
        }
    }

    // --- Snapshot filters ---
    const snapshottedFilterGuestType = filterGuestType; 
    const snapshottedFilterStatus = filterStatus;
    let criteriaDescFilters = [];
    if (snapshottedFilterGuestType !== 'all') criteriaDescFilters.push(`Type: ${snapshottedFilterGuestType.toUpperCase()}`);
    if (snapshottedFilterStatus !== 'all') criteriaDescFilters.push(`Status: ${snapshottedFilterStatus}`);
    // --- End snapshot filters ---

    const snapshottedRawSearch = searchInput ? searchInput.value.toLowerCase() : '';
    const snapshottedSearchTerms = snapshottedRawSearch.split(',').map(term => term.trim()).filter(term => term);
    let criteriaDescSearch = snapshottedSearchTerms.length > 0 ? `Search: "${snapshottedSearchTerms.join(', ')}"` : null;
    // --- End Snapshot ---

    // --- Check if *any* criteria is set (Thresholds OR Search OR Filters) --- 
    if (activeThresholdCount === 0 && snapshottedSearchTerms.length === 0 && criteriaDescFilters.length === 0) { 
        alert("Please set at least one threshold, enter search terms, or select a Type/Status filter before starting the log."); // Updated alert message
        if (toggleThresholdsButton && !isThresholdRowVisible && activeThresholdCount === 0) { // Only show thresholds if none are set
            toggleThresholdsButton.click(); 
        }
        return;
    }

    const sessionId = Date.now(); // Use timestamp as unique ID
    const startTime = new Date();
    // Build session title including search terms and thresholds
    let fullCriteriaDesc = [criteriaDescFilters.join('; '), criteriaDescSearch, criteriaDescThresholds.length > 0 ? `Thresholds: ${criteriaDescThresholds.join(', ')}`: null].filter(Boolean).join('; ');
    const sessionTitle = `Log @ ${startTime.toLocaleTimeString()}${fullCriteriaDesc ? ` (${fullCriteriaDesc})` : ''}`;
    // const shortLogTitle = generateShortLogTitle(criteriaDescThresholds, criteriaDescFilters, criteriaDescSearch); // REMOVED call

    // Create the session panel HTML
    const panel = document.createElement('div');
    panel.id = `log-session-panel-${sessionId}`; // Add prefix to panel ID
    panel.className = 'log-session-panel border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 shadow-md';

    const header = document.createElement('div');
    header.className = 'log-session-header flex justify-between items-center p-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50 rounded-t';

    const titleSpan = document.createElement('span');
    titleSpan.className = 'text-sm font-medium text-gray-700 dark:text-gray-200 truncate';
    titleSpan.textContent = sessionTitle;
    titleSpan.title = sessionTitle; // Full title on hover

    const panelCloseButton = document.createElement('button'); // Renamed variable
    panelCloseButton.className = 'p-1 rounded text-gray-500 hover:bg-red-100 dark:text-gray-400 dark:hover:bg-red-800/50';
    panelCloseButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
    panelCloseButton.title = 'Close Log Panel & Stop Log (if active)'; // Updated title

    panelCloseButton.onclick = (event) => { // Updated logic
        const panelToRemove = event.currentTarget.closest('.log-session-panel');
        if (panelToRemove) {
            const panelSessionIdStr = panelToRemove.id.replace('log-session-panel-', '');

            // Stop the log session if it's active
            if (activeLogSessions[panelSessionIdStr]) {
                stopThresholdLogging(panelSessionIdStr, 'manual');
            }

            const tabToRemove = nestedTabsContainer.querySelector(`.nested-tab[data-nested-tab="log-session-${panelSessionIdStr}"]`);
            const contentToRemove = nestedTabContentContainer.querySelector(`#log-session-${panelSessionIdStr}`); // Target the content div

            if (tabToRemove) tabToRemove.remove();
            if (contentToRemove) contentToRemove.remove(); // Remove the content div (which contains the panel)

            updateClearAllButtonVisibility(); // Update clear button state after removal

            // If the closed tab was active, switch back to the main dashboard
            if (tabToRemove && tabToRemove.classList.contains('active')) {
                activateNestedTab('nested-tab-dashboard');
            }
        }
    };

    header.appendChild(titleSpan);
    header.appendChild(panelCloseButton); // Append renamed button

    const tableContainer = document.createElement('div');
    tableContainer.className = 'log-table-container p-2 max-h-96 overflow-y-auto'; // Limit height and enable scroll

    const table = document.createElement('table');
    table.id = `log-table-${sessionId}`;
    table.className = 'min-w-full text-xs border-collapse';
    table.innerHTML = `
        <thead class="sticky top-0 bg-gray-100 dark:bg-gray-700 z-10">
            <tr class="border-b border-gray-300 dark:border-gray-600">
                <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Time</th>
                <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Guest</th>
                <th class="p-1 px-2 text-left font-semibold text-gray-600 dark:text-gray-300">Node</th>
                <!-- New Metric Headers -->
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">CPU</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">Mem</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">Disk%</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">DRead</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">DWrite</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">NetIn</th>
                <th class="p-1 px-2 text-right font-semibold text-gray-600 dark:text-gray-300">NetOut</th>
            </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <!-- Log entries added here -->
             <tr class="initial-log-message">
                <td colspan="10" class="p-2 text-center text-gray-500 dark:text-gray-400 italic"> <!-- Adjusted colspan -->
                    Logging started for ${durationMinutes} min<span class="dot-animate">.</span><span class="dot-animate">.</span><span class="dot-animate">.</span>
                </td>
             </tr>
        </tbody>
    `;

    tableContainer.appendChild(table);
    panel.appendChild(header);
    panel.appendChild(tableContainer);
    // logSessionsContainer.appendChild(panel); // Original line
    // logSessionsContainer.prepend(panel); // Insert new panel at the top

    // --- START Dynamic Tab and Content Creation (Restored) ---
    const nestedTabId = `log-session-${sessionId}`;

    // Create Nested Tab Element
    const newTab = document.createElement('div');
    newTab.className = 'nested-tab px-3 py-1.5 cursor-pointer text-sm font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 border-b-2 border-transparent hover:border-gray-300 dark:hover:border-gray-600 flex items-center gap-1';
    newTab.dataset.nestedTab = nestedTabId;
    newTab.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block align-middle"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/></svg>
        <span class="log-tab-title" title="${fullCriteriaDesc || 'Log started ' + startTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}">${startTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
        <span class="log-timer-display ml-1.5 text-xs text-gray-500 dark:text-gray-400" data-session-id="${sessionId}">(--:--)</span>
    `;
    newTab.addEventListener('click', () => activateNestedTab(nestedTabId));

    // --- Add Rename Functionality ---
    const tabTitleSpan = newTab.querySelector('.log-tab-title'); // Renamed variable
    if (tabTitleSpan) {
        tabTitleSpan.addEventListener('dblclick', () => {
            tabTitleSpan.classList.add('hidden'); // Hide the span

            const input = document.createElement('input');
            input.type = 'text';
            input.value = tabTitleSpan.textContent;
            input.className = 'log-tab-rename-input flex-grow p-0 px-1 h-5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none'; // Basic styling
            input.style.maxWidth = '150px'; // Prevent excessive width

            const finalizeRename = () => {
                const newName = input.value.trim();
                if (newName) {
                    tabTitleSpan.textContent = newName;
                }
                input.remove(); // Remove the input
                tabTitleSpan.classList.remove('hidden'); // Show the span again
            };

            const cancelRename = () => {
                input.remove();
                tabTitleSpan.classList.remove('hidden');
            };

            input.addEventListener('blur', finalizeRename);
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    finalizeRename();
                } else if (e.key === 'Escape') {
                    cancelRename();
                }
            });

            // Insert input before the timer span, or at the end if timer doesn't exist
            const timerSpan = newTab.querySelector('.log-timer-display');
            if (timerSpan) {
                newTab.insertBefore(input, timerSpan);
            } else {
                 newTab.appendChild(input);
            }
            input.focus();
            input.select();
        });
    }
    // --- End Rename Functionality ---

    // --- Add Close Button to Tab --- 
    const tabCloseButton = document.createElement('button');
    tabCloseButton.className = 'ml-auto pl-2 text-gray-400 hover:text-red-600 dark:hover:text-red-400 opacity-50 hover:opacity-100 transition-opacity';
    tabCloseButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
    tabCloseButton.title = 'Close & Stop Log';
    tabCloseButton.onclick = (event) => {
        event.stopPropagation(); // Prevent tab activation when clicking close
        const tabElement = event.currentTarget.closest('.nested-tab');
        if (!tabElement) return;

        const tabSessionId = tabElement.dataset.nestedTab; // e.g., "log-session-1678886400000"
        const tabSessionIdStr = tabSessionId.replace('log-session-', '');

        // Stop the log session if it's active
        if (activeLogSessions[tabSessionIdStr]) {
            stopThresholdLogging(tabSessionIdStr, 'manual');
        }

        const contentToRemove = nestedTabContentContainer.querySelector(`#${tabSessionId}`);

        tabElement.remove();
        if (contentToRemove) contentToRemove.remove();

        updateClearAllButtonVisibility();

        // If the closed tab was active, switch back to the main dashboard
        if (tabElement.classList.contains('active')) {
            activateNestedTab('nested-tab-dashboard');
        }
    };
    newTab.appendChild(tabCloseButton);
    // --- END Add Close Button to Tab --- 

    // Create Nested Tab Content Element (This is the container for the panel)
    const newContent = document.createElement('div');
    newContent.id = nestedTabId; // ID is log-session-<timestamp>
    // newContent.className = 'nested-tab-content p-3 hidden'; // OLD class
    newContent.className = 'log-session-panel-container hidden'; // NEW class, start hidden
    newContent.appendChild(panel); // Add the log panel to the content area

    // Append to DOM
    if (nestedTabsContainer && nestedTabContentContainer && logSessionArea) {
        nestedTabsContainer.appendChild(newTab);
        nestedTabContentContainer.appendChild(newContent); // Append content container to #log-content-area
        logSessionArea.classList.remove('hidden'); // Show the whole log area
    } else {
        console.error("Log tab/content container or session area not found!");
    }

    // Activate the new tab
    activateNestedTab(nestedTabId);
    // --- END Dynamic Tab and Content Creation (Restored) ---

    // --- Append panel to the new container --- 
    // if (logSessionsContainer) { // REMOVED
    //     logSessionsContainer.prepend(panel); // Add the panel to the dedicated container // REMOVED
    // } else { // REMOVED
    //     console.error("#log-sessions-container not found!"); // REMOVED
    // } // REMOVED
    // --- END Append panel ---

    // Start the timer to automatically stop the session
    const timerId = setTimeout(() => {
        // Pass 'timer' reason when stopped by timeout
        stopThresholdLogging(sessionId, 'timer'); 
    }, durationMs);

    // Store session info
    activeLogSessions[sessionId] = {
        thresholds: snapshottedThresholds,
        startTime: startTime,
        durationMs: durationMs,
        timerId: timerId,
        entries: [],
        element: panel, // Reference to the DOM element for easy removal
        searchTerms: snapshottedSearchTerms, // Store snapshotted search terms
        filterGuestType: snapshottedFilterGuestType, // <<< ADDED
        filterStatus: snapshottedFilterStatus       // <<< ADDED
    };

    // Update UI
    // isThresholdLoggingActive = true; // REMOVED
    // logDurationSelect.disabled = true; // REMOVED - Keep select enabled

    console.log(`[Log Session ${sessionId}] Started. Duration: ${durationMinutes}m. Thresholds:`, snapshottedThresholds);
  }

  function stopThresholdLogging(sessionId, reason = 'manual') { // Added reason parameter
      const session = activeLogSessions[sessionId];
      if (!session) {
          console.warn(`Attempted to stop non-existent log session: ${sessionId}`);
          return;
      }

      console.log(`[Log Session ${sessionId}] Stopping. Reason: ${reason}`);

      clearTimeout(session.timerId); // Clear the auto-stop timer regardless of reason

      // --- Modifications for Persistence ---
      if (session.element) {
          // 1. Disable the close button on the panel
          /* // REMOVED THIS BLOCK - We want the 'x' to always remove the panel
          const closeButton = session.element.querySelector('.log-session-header button');
          if (closeButton) {
              closeButton.disabled = true;
              closeButton.classList.add('opacity-50', 'cursor-not-allowed');
              closeButton.onclick = null; // Remove listener
          }
          */

          // 2. Add final status message to the table body (Less critical now, but keep for timer expiry case)
          const tableBody = session.element.querySelector(`#log-table-${sessionId} tbody`);
          if (tableBody) {
              // Remove the initial 'Logging started...' message if it's still there
              const initialMsgRow = tableBody.querySelector('.initial-log-message');
              if (initialMsgRow) initialMsgRow.remove();

              const finalStatusRow = tableBody.insertRow(-1); // Insert at the end
              finalStatusRow.className = 'final-log-message';
              const cell = finalStatusRow.insertCell(0);
              cell.colSpan = 6; // Span all columns
              cell.className = 'p-1 px-2 text-center text-xs text-gray-500 dark:text-gray-400 italic border-t border-gray-200 dark:border-gray-700';
              const stopTime = new Date().toLocaleTimeString();
              if (reason === 'timer') {
                  cell.textContent = `Logging finished (timer expired) at ${stopTime}`;
              } else {
                  cell.textContent = `Logging stopped manually at ${stopTime}`;
              }
          }
          // Remove panel removal - Keep panel visible after stop
          // session.element.remove(); // << REMOVED THIS LINE
      }
      // --- End Modifications --- 

      // Remove from active sessions AFTER updating its UI elements
      delete activeLogSessions[sessionId]; 

      // Update main UI only if this was the last active session - REMOVED Global State Updates
      /* // REMOVED Block
      if (Object.keys(activeLogSessions).length === 0) {
          isThresholdLoggingActive = false;
          logDurationSelect.disabled = false; // Enable select
      }
      */
      updateClearAllButtonVisibility(); // Update clear button state after stopping
  }

  function addLogRow(sessionId, entry) {
      const session = activeLogSessions[sessionId];
      if (!session || !session.element) return;

      const tableBody = session.element.querySelector(`#log-table-${sessionId} tbody`);
      if (!tableBody) return;

      // Remove initial message if present
      const initialMsgRow = tableBody.querySelector('.initial-log-message');
      if (initialMsgRow) initialMsgRow.remove();

      const row = tableBody.insertRow(0); // Insert at the top
      row.className = 'bg-yellow-50 dark:bg-yellow-900/20 animate-pulse-once'; // Highlight new rows briefly

      // --- Apply Bold Styling Based on Active Thresholds ---
      const cpuValueHTML = entry.activeThresholdKeys.includes('cpu') ? `<strong>${entry.cpuFormatted}</strong>` : entry.cpuFormatted;
      const memValueHTML = entry.activeThresholdKeys.includes('memory') ? `<strong>${entry.memFormatted}</strong>` : entry.memFormatted;
      const diskValueHTML = entry.activeThresholdKeys.includes('disk') ? `<strong>${entry.diskFormatted}</strong>` : entry.diskFormatted;
      const diskReadValueHTML = entry.activeThresholdKeys.includes('diskread') ? `<strong>${entry.diskReadFormatted}</strong>` : entry.diskReadFormatted;
      const diskWriteValueHTML = entry.activeThresholdKeys.includes('diskwrite') ? `<strong>${entry.diskWriteFormatted}</strong>` : entry.diskWriteFormatted;
      const netInValueHTML = entry.activeThresholdKeys.includes('netin') ? `<strong>${entry.netInFormatted}</strong>` : entry.netInFormatted;
      const netOutValueHTML = entry.activeThresholdKeys.includes('netout') ? `<strong>${entry.netOutFormatted}</strong>` : entry.netOutFormatted;
      // --- End Apply Bold Styling ---

      // --- Apply Bold Styling for Search Matches ---
      const guestDisplayHTML = entry.guestMatchedSearch ? `<strong>${entry.guestName} (${entry.guestId})</strong>` : `${entry.guestName} (${entry.guestId})`;
      const nodeDisplayHTML = entry.nodeMatchedSearch ? `<strong>${entry.node}</strong>` : entry.node;
      // --- End Apply Bold Styling for Search Matches ---


      // Populate cells with comprehensive metric data
      console.log(`[addLogRow] About to add row for ${entry.guestName} (${entry.guestId}) to session ${sessionId}`); // DEBUG
      row.innerHTML = `
          <td class="p-1 px-2 whitespace-nowrap">${entry.timestamp.toLocaleTimeString()}</td>
          <td class="p-1 px-2 whitespace-nowrap" title="${entry.guestName}">${guestDisplayHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap">${nodeDisplayHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${cpuValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${memValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${diskValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${diskReadValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${diskWriteValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${netInValueHTML}</td>
          <td class="p-1 px-2 whitespace-nowrap text-right">${netOutValueHTML}</td>
      `;

      tableBody.prepend(row); // Prepend the row

      // --- DEBUG: Check if row is in DOM and potentially visible ---
      const addedRow = tableBody.firstChild; // Get the row we just prepended
      if (addedRow === row) {
          console.log(`[addLogRow] Row for ${entry.guestName} successfully prepended to session ${sessionId}. Computed display: ${window.getComputedStyle(addedRow).display}`);
      } else {
          console.error(`[addLogRow] Failed to prepend or find row for ${entry.guestName} in session ${sessionId}!`);
      }
      // --- END DEBUG ---

      // Remove the pulse animation class after it finishes
      // setTimeout(() => { // Commented out pulse for debugging clarity
       setTimeout(() => {
            row.classList.remove('bg-yellow-50', 'dark:bg-yellow-900/20', 'animate-pulse-once');
       }, 1500); // Match animation duration in CSS if defined, otherwise estimate

       session.entries.push(entry); // Add to session data
  }


function checkThresholdViolations() {
      if (Object.keys(activeLogSessions).length === 0) {
          return; // No active logging sessions
    }

    const now = new Date();

      // --- Get current UI filter state --- 
      const currentFilterGuestType = filterGuestType;
      const currentFilterStatus = filterStatus;
      // const rawSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : []; // REMOVED
      // const currentTextSearchTerms = rawSearchTerms.filter(term => !thresholdPrefixes.some(prefix => term.startsWith(prefix))); // REMOVED
      // --- End Get current UI filter state --- 

      // We assume refreshDashboardData() has been called recently by the main interval,
      // so dashboardData is up-to-date.

    dashboardData.forEach(guest => {
          // --- REVERT: Remove checks for current UI filters --- 
          /* // REMOVED Block
          // --- Apply Dashboard Filters BEFORE checking thresholds ---
          // Must be running
          if (guest.status !== 'running') return; 

          // Type Filter
          const typeMatch = currentFilterGuestType === 'all' || (currentFilterGuestType === 'vm' && guest.type === 'VM') || (currentFilterGuestType === 'lxc' && guest.type === 'CT');
          if (!typeMatch) return; // Skip if type doesn't match UI filter

          // Status Filter (Redundant if only checking running, but keep for consistency?)
          const statusMatch = currentFilterStatus === 'all' || guest.status === currentFilterStatus;
          if (!statusMatch) return; // Skip if status doesn't match UI filter

          // Text Search Filter
          const searchMatch = currentTextSearchTerms.length === 0 || currentTextSearchTerms.some(term => 
                (guest.name && guest.name.toLowerCase().includes(term)) || 
                (guest.node && guest.node.toLowerCase().includes(term)) ||
                (guest.vmid && guest.vmid.toString().includes(term)) ||
                (guest.id && guest.id.toString().includes(term))
            );
          if (!searchMatch) return; // Skip if text search doesn't match
          // --- End Apply Dashboard Filters --- 
          */

          // --- Keep only the check for running status --- 
        if (guest.status !== 'running') return; // Only log for running guests
          // --- End Keep only check --- 

          Object.keys(activeLogSessions).forEach(sessionId => {
              const session = activeLogSessions[sessionId];
              if (!session) return; // Should not happen, but safety check

              // --- Apply Snapshotted Type/Status Filters FIRST --- <<< Added
              const sessionFilterGuestType = session.filterGuestType;
              const sessionFilterStatus = session.filterStatus;

              const typeMatch = sessionFilterGuestType === 'all' ||
                                (sessionFilterGuestType === 'vm' && guest.type === 'VM') ||
                                (sessionFilterGuestType === 'lxc' && guest.type === 'CT');
              if (!typeMatch) return; // Skip guest if type doesn't match session filter

              const statusMatch = sessionFilterStatus === 'all' || guest.status === sessionFilterStatus;
              if (!statusMatch) return; // Skip guest if status doesn't match session filter
              // --- End Apply Snapshotted Type/Status Filters ---

              // --- Apply Snapshotted Text Search Filter --- <<< Renamed section
              const snapshottedSearch = session.searchTerms || []; // Get snapshotted terms
              let guestMatchedSearch = false; // Flag specifically for guest match
              let nodeMatchedSearch = false;  // Flag specifically for node match
              let overallSearchMatch = false; // Flag to check if ANY part matched for logging

              if (snapshottedSearch.length > 0) {
                  snapshottedSearch.forEach(term => {
                      if (!guestMatchedSearch && (
                          (guest.name && guest.name.toLowerCase().includes(term)) ||
                          (guest.vmid && guest.vmid.toString().includes(term)) ||
                          (guest.id && guest.id.toString().includes(term)) // Assuming guest.id is the uniqueId
                      )) {
                          guestMatchedSearch = true;
                      }
                      if (!nodeMatchedSearch && (
                          (guest.node && guest.node.toLowerCase().includes(term))
                      )) {
                          nodeMatchedSearch = true;
                      }
                  });

                  overallSearchMatch = guestMatchedSearch || nodeMatchedSearch;

                  // If search terms exist, but neither guest nor node matched, skip guest.
                  if (!overallSearchMatch) return; 
              } else {
                  // If no search terms, consider it a match for logging purposes
                  overallSearchMatch = true; 
              }
              // --- End Apply Snapshotted Text Search ---

              // --- Now check thresholds (if any were snapshotted) ---
              let meetsAllThresholds = true; // Start assuming guest meets all thresholds
              // let shouldLogEntry = false; // Old OR logic flag
              let violationDetails = []; // Still useful to store which thresholds were active for context

              for (const type in session.thresholds) {
                  const thresholdValue = session.thresholds[type]; // Snapshotted value
            if (thresholdValue <= 0) continue; // Skip inactive thresholds from the snapshot

            let guestValue;
                  let valueFormatted, limitFormatted;

                  // Get the current guest value for the specific metric type
            if (type === 'cpu') guestValue = guest.cpu * 100;
            else if (type === 'memory') guestValue = guest.memory;
            else if (type === 'disk') guestValue = guest.disk;
            else if (type === 'diskread') guestValue = guest.diskread;
            else if (type === 'diskwrite') guestValue = guest.diskwrite;
            else if (type === 'netin') guestValue = guest.netin;
            else if (type === 'netout') guestValue = guest.netout;
                  else continue; // Unknown type in snapshot

                  // Skip check if guest value is invalid/unavailable
                  if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) {
                      continue;
                  }

                  // Check for violation (guest value >= threshold limit)
                  // if (guestValue >= thresholdValue) { // Old OR check
                  if (guestValue < thresholdValue) { // Check if guest FAILS this threshold
                      // valueFormatted = formatThresholdValue(type, guestValue);
                      // limitFormatted = formatThresholdValue(type, thresholdValue);
                      // shouldLogEntry = true; // Old OR logic flag
                      // violationDetails.push({ type: getReadableThresholdName(type), value: formatThresholdValue(type, guestValue), limit: formatThresholdValue(type, thresholdValue) });
                      meetsAllThresholds = false; // Guest failed at least one threshold
                      // --- DEBUG LOG --- 
                      console.log(`[Log Check FAIL] Guest: ${guest.name}, Metric: ${type}, Value: ${guestValue}, Threshold: ${thresholdValue}`);
                      // --- END DEBUG LOG ---
                      break; // No need to check further thresholds for this guest
                  } else {
                      // --- DEBUG LOG --- 
                      console.log(`[Log Check PASS] Guest: ${guest.name}, Metric: ${type}, Value: ${guestValue}, Threshold: ${thresholdValue}`);
                      // --- END DEBUG LOG ---
                      // Optional: Store passed threshold details for context if needed
                      // violationDetails.push({ type: getReadableThresholdName(type), value: formatThresholdValue(type, guestValue), limit: formatThresholdValue(type, thresholdValue) });
                  }
              } // End loop through session thresholds

              // If the guest met ALL thresholds checked
              // if (shouldLogEntry) { // Old OR check
              if (meetsAllThresholds) { 
                    // Gather all current metrics for the guest
                    const metricsSnapshot = {
                        cpu: guest.cpu * 100,
                        mem: guest.memory,
                        disk: guest.disk,
                        diskRead: guest.diskread,
                        diskWrite: guest.diskwrite,
                        netIn: guest.netin,
                        netOut: guest.netout
                    };

                    // Create the comprehensive log entry
                    const logEntry = {
                        timestamp: now,
                        guestId: guest.vmid,
                        guestName: guest.name,
                        node: guest.node,
                        // Formatted values for display
                        cpuFormatted: formatThresholdValue('cpu', metricsSnapshot.cpu),
                        memFormatted: formatThresholdValue('memory', metricsSnapshot.mem),
                        diskFormatted: formatThresholdValue('disk', metricsSnapshot.disk),
                        diskReadFormatted: formatThresholdValue('diskread', metricsSnapshot.diskRead),
                        diskWriteFormatted: formatThresholdValue('diskwrite', metricsSnapshot.diskWrite),
                        netInFormatted: formatThresholdValue('netin', metricsSnapshot.netIn),
                        netOutFormatted: formatThresholdValue('netout', metricsSnapshot.netOut),
                        // Raw values for potential debouncing/comparison
                        metricsRaw: metricsSnapshot, 
                        activeThresholdKeys: Object.keys(session.thresholds), // Pass the keys for highlighting
                        guestMatchedSearch: guestMatchedSearch, // <<< ADDED specific flag
                        nodeMatchedSearch: nodeMatchedSearch   // <<< ADDED specific flag
                        // violations: violationDetails // Removed for now, as only logging passes
                    };

                    // Avoid logging the exact same violation repeatedly very quickly (debouncing)
                    const lastEntry = session.entries.length > 0 ? session.entries[0] : null; // Check the most recent entry (inserted at top)
                    if (!lastEntry ||
                        !(lastEntry.guestId === logEntry.guestId &&
                          (now.getTime() - lastEntry.timestamp.getTime()) < 5000) // Simple time-based debounce per guest
                       )
                    {
                          addLogRow(sessionId, logEntry);
                    }
              }
          }); // End loop through active sessions
      }); // End loop through guests
  }

  // Add listener for the Start/Stop Log button
  if (toggleLogModeButton) {
      toggleLogModeButton.addEventListener('click', () => {
          if (isThresholdLoggingActive) {
              // Find the active session ID (assuming only one for now)
              const activeSessionId = Object.keys(activeLogSessions)[0];
              if (activeSessionId) {
                  // Explicitly pass 'manual' when clicking the main button
                  stopThresholdLogging(activeSessionId, 'manual'); 
              }
          } else {
              startThresholdLogging();
          }
      });
  } else {
      console.warn('#toggle-log-mode-button not found.');
  }

  // Add listener for the NEW Start Log button
  const startLogButton = document.getElementById('start-log-button');
  if (startLogButton) {
      startLogButton.addEventListener('click', () => {
          startThresholdLogging(); // Only starts logging
      });
  } else {
      console.warn('#start-log-button not found.');
  }

  // Add function to manage Clear All button visibility
  const clearAllLogsButton = document.getElementById('clear-all-logs-button'); // Get reference later

  function updateClearAllButtonVisibility() {
    if (!clearAllLogsButton || !nestedTabsContainer) return; // Check nestedTabsContainer

    // Check if there are any log tabs that correspond to finished sessions
    let hasFinishedLogs = false;
    nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').forEach(tab => {
        const sessionId = tab.dataset.nestedTab.replace('log-session-', '');
        if (!activeLogSessions[sessionId]) { // If session ID not in active sessions, it's finished
            hasFinishedLogs = true;
        }
    });

    if (hasFinishedLogs) {
        clearAllLogsButton.classList.remove('hidden');
    } else {
        clearAllLogsButton.classList.add('hidden');
    }
  }

  // Add listener for the Clear All button
  if (clearAllLogsButton) {
      clearAllLogsButton.addEventListener('click', () => {
          if (nestedTabsContainer && nestedTabContentContainer && logSessionArea) { // Check logSessionArea exists
              const tabsToRemove = [];
              const contentsToRemove = [];
              // Find finished log tabs and content
              nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').forEach(tab => {
                  const sessionIdFull = tab.dataset.nestedTab; // Use the full ID from data attribute
                  if (!activeLogSessions[sessionIdFull.replace('log-session-', '')]) { // Check if session is NOT active
                      tabsToRemove.push(tab);
                      const content = nestedTabContentContainer.querySelector(`#${sessionIdFull}`); // Find content by ID
                      if (content) contentsToRemove.push(content);
                  }
              });

              // Remove them from DOM
              tabsToRemove.forEach(tab => tab.remove());
              contentsToRemove.forEach(content => content.remove());

              // Hide the entire log area if no tabs are left after clearing
               if (nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').length === 0) {
                  logSessionArea.classList.add('hidden');
               }
          }
          // Hide the button itself after clearing
          clearAllLogsButton.classList.add('hidden');
      });
  } else {
      console.warn('#clear-all-logs-button not found.');
  }

// --- END Threshold Logging Functions ---
 
// --- START Reset Functions ---
// Define standalone resetThresholds function
function resetThresholds() {
    /* // REMOVED: Don't stop active log on view reset
    // If logging is active while resetting thresholds, stop it.
    if (isThresholdLoggingActive) {
        const activeSessionId = Object.keys(activeLogSessions)[0];
        if (activeSessionId) {
            stopThresholdLogging(activeSessionId, 'manual'); // Stop manually to preserve logs
        }
    }
    */

    for (const type in thresholdState) {
        if (sliders[type]) { // Slider
            thresholdState[type].value = 0;
            const sliderElement = sliders[type];
            if (sliderElement) sliderElement.value = 0;
        } else if (thresholdSelects[type]) { // Dropdown Select
            thresholdState[type].value = 0;
            const selectElement = thresholdSelects[type];
            if (selectElement) selectElement.value = 0; // Set to the 'Any' option value
        }
    } // End for loop
    hideSliderTooltip(); // Make sure tooltip is hidden after reset
    // Ensure threshold row is hidden on reset
    isThresholdRowVisible = false;
    updateThresholdRowVisibility();
    updateThresholdIndicator(); // Update badge after reset
}

// Reset the main dashboard view
  // ... existing code ...
  // --- END Reset Functions ---

  // --- Helper functions moved outside DOMContentLoaded ---

  // NEW Helper function to generate a concise title for log tabs
  function generateShortLogTitle(thresholds, filters, search, maxLen = 25) {
      let parts = [];

      // 1. Thresholds (simplified)
      if (thresholds.length > 0) {
          const simpleThresholds = thresholds.map(t => {
              // Extract key parts like 'CPU>=80%' or 'Disk Read>=1 MB/s'
              const match = t.match(/^([\w\s]+)[>=<]+(.*)$/);
              if (match) {
                  const key = match[1].replace(' %',''); // Remove space+%
                  const value = match[2].replace(/%|\s.*$/g, ''); // Remove % and units like /s
                  return `${key}>${value}`;
              }
              return t; // Fallback
          });
          parts.push(simpleThresholds.join(' ')); // Use space separator
      }

      // 2. Search
      if (search) {
          // Extract search term like "Search: 'term'"
           const termMatch = search.match(/^Search: "(.+)"$/);
           if (termMatch) {
                // parts.push(`S:'${termMatch[1].substring(0, 10)}'`); // Old format
                parts.push(`'${termMatch[1].substring(0, 10)}'`); // New format: just the term in quotes
           }
      }

      // 3. Filters
      if (filters.length > 0) {
          const simpleFilters = filters.map(f => {
              // Extract key parts like 'Type: VM' -> 'VM'
              const parts = f.split(': ');
              return parts.length > 1 ? parts[1] : f;
          });
           parts.push(simpleFilters.join('/'));
      }

      let shortTitle = parts.join(' '); // Use space separator

      if (shortTitle.length > maxLen) {
          return shortTitle.substring(0, maxLen - 1) + '…'; // Truncate
      } else if (shortTitle.length === 0) {
          return 'Log'; // Fallback if no criteria
      }

      return shortTitle;
  }

  function getReadableThresholdName(type) {
      switch(type) {
          case 'cpu': return 'CPU';
          case 'memory': return 'Memory %';
          case 'disk': return 'Disk %'; // More specific for VM % usage
          case 'diskread': return 'Disk Read';
          case 'diskwrite': return 'Disk Write';
          case 'netin': return 'Net In';
          case 'netout': return 'Net Out';
          default: return type.charAt(0).toUpperCase() + type.slice(1); // Capitalize
      }
  }

  function formatThresholdValue(type, value) {
      if (value === null || value === undefined || isNaN(value)) return 'N/A';
      if (type === 'cpu' || type === 'memory' || type === 'disk') return `${Math.round(value)}%`; // Round percentages for display
      if (type === 'diskread' || type === 'diskwrite' || type === 'netin' || type === 'netout') return formatSpeedInt(value); // Use integer speed format
      return String(value);
  }

  // Add CSS for the pulse animation (if not already defined elsewhere)
  // Example: Add this to your CSS file or a <style> block in index.html
  /*
  @keyframes pulse-once {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  .animate-pulse-once {
    animation: pulse-once 1.5s ease-in-out;
  }
  */

  // --- START Search Bar Threshold Integration ---
  function getThresholdFilterTag(type, value) {
    if (value <= 0) return null; // No tag for inactive thresholds

    let operator = '>='; // Default operator
    let displayValue;

    // Format value based on type for the tag
    if (type === 'cpu' || type === 'memory' || type === 'disk') {
        displayValue = `${parseInt(value)}`; // Just the number for percentage
    } else if (type === 'uptime') {
        displayValue = `${parseInt(value)}`; // Use seconds for uptime
    } else if (['diskread', 'diskwrite', 'netin', 'netout'].includes(type)) {
        displayValue = `${parseInt(value)}`; // Use bytes/sec
    } else {
        return null; // Unknown type
    }
    return `${type}${operator}${displayValue}`;
  }

  function updateSearchTermWithThreshold(type, value) {
    if (!searchInput) return; // Only works if search input exists

    const newTag = getThresholdFilterTag(type, value);
    const currentSearchText = searchInput.value || '';
    let searchTerms = currentSearchText.split(',').map(term => term.trim()).filter(term => term);

    // Remove any existing tag for this type
    const typePrefix = `${type}>=`;
    searchTerms = searchTerms.filter(term => !term.startsWith(typePrefix));

    // Add the new tag if the threshold is active
    if (newTag) {
        searchTerms.push(newTag);
    }

    const updatedSearchText = searchTerms.join(', ');
    if (searchInput.value !== updatedSearchText) {
        searchInput.value = updatedSearchText;
        // Dispatch input event to trigger table update based on search
        searchInput.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }
  // --- END Search Bar Threshold Integration ---

  // --- Global Timer Update Logic ---
  function formatRemainingTime(ms) {
      if (ms <= 0) return "00:00";
      const totalSeconds = Math.max(0, Math.floor(ms / 1000));
      const minutes = Math.floor(totalSeconds / 60);
      const seconds = totalSeconds % 60;
      return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
  }

  function updateAllLogTimers() {
      const now = Date.now();
      // Select all visible timer spans first
      document.querySelectorAll('.log-timer-display').forEach(timerSpan => {
          const sessionId = timerSpan.dataset.sessionId;
          const session = activeLogSessions[sessionId];

          if (session) {
              const endTime = session.startTime.getTime() + session.durationMs;
              const remainingMs = endTime - now;
              timerSpan.textContent = formatRemainingTime(remainingMs);

              // Optional: Change style when finished
              if (remainingMs <= 0) {
                  timerSpan.classList.add('text-red-500', 'dark:text-red-400');
              }

          } else {
              // If session is no longer active but span exists, ensure it shows 00:00
              // This might happen briefly if stopThresholdLogging hasn't updated it yet
              if (timerSpan.textContent !== "00:00") {
                 timerSpan.textContent = "00:00";
                 timerSpan.classList.add('text-red-500', 'dark:text-red-400');
              }
          }
      });
  }

  // Start the global timer interval
  setInterval(updateAllLogTimers, 1000); // Update every second
  // --- END Global Timer Update Logic ---

}); // End DOMContentLoaded
