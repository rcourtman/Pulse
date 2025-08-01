PulseApp.ui = PulseApp.ui || {};

// Create logger instance for dashboard
const logger = PulseApp.utils.createLogger('Dashboard');
// Create DOM cache instance
const domCache = new PulseApp.utils.DOMCache();

const FILTER_ALL = 'all';
const FILTER_VM = 'vm';
const FILTER_LXC = 'lxc';

const GUEST_TYPE_VM = 'VM';
const GUEST_TYPE_CT = 'CT';

const STATUS_RUNNING = 'running';
const STATUS_STOPPED = 'stopped';

const METRIC_CPU = 'cpu';
const METRIC_MEMORY = 'memory';
const METRIC_DISK = 'disk';
const METRIC_DISK_READ = 'diskread';
const METRIC_DISK_WRITE = 'diskwrite';
const METRIC_NET_IN = 'netin';
const METRIC_NET_OUT = 'netout';

PulseApp.ui.dashboard = (() => {
    let searchInput = null;
    let guestMetricDragSnapshot = {}; // To store metrics during slider drag
    let tableBodyEl = null;
    let statusElementEl = null;
    let virtualScroller = null;
    const VIRTUAL_SCROLL_THRESHOLD = 100; // Use virtual scrolling for >100 items

    function _createAlertSliderHtml(guestId, metricType, config) {
        return PulseApp.utils.createAlertSliderHtml(guestId, 'guest', metricType, config);
    }

    function _createAlertDropdownHtml(guestId, metricType, options) {
        return PulseApp.utils.createAlertDropdownHtml(guestId, 'guest', metricType, options);
    }
    

    // Helper function to check if guest has any backup in last 24 hours
    function hasRecentBackup(vmid) {
        const now = Date.now() / 1000; // Current timestamp in seconds
        const twentyFourHoursAgo = now - (24 * 60 * 60);
        
        // Check PBS backups
        const pbsData = PulseApp.state.get('pbsDataArray') || [];
        for (const pbsInstance of pbsData) {
            if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
                for (const datastore of pbsInstance.datastores) {
                    if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
                        for (const backup of datastore.snapshots) {
                            // Check if this backup is for our VM/container
                            if (backup['backup-id'] === String(vmid)) {
                                const backupTime = backup['backup-time'] || 0;
                                if (backupTime >= twentyFourHoursAgo) {
                                    return true;
                                }
                            }
                        }
                    }
                }
            }
        }
        
        // Check PVE local backups
        const pveBackups = PulseApp.state.get('pveBackups');
        if (pveBackups && pveBackups.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            for (const backup of pveBackups.storageBackups) {
                if (backup.vmid === vmid || backup.vmid === String(vmid)) {
                    const backupTime = backup.ctime || 0;
                    if (backupTime >= twentyFourHoursAgo) {
                        return true;
                    }
                }
            }
        }
        
        return false;
    }

    // Helper function to setup event listeners for alert sliders and dropdowns
    function _setupAlertEventListeners(container) {
        if (!container) return;
        
        // Setup event listeners for all alert number inputs in this container
        const alertInputs = container.querySelectorAll('.alert-threshold-input input[type="number"]');
        alertInputs.forEach(input => {
            const container = input.closest('.alert-threshold-input');
            const guestId = container?.getAttribute('data-guest-id');
            const metricType = container?.getAttribute('data-metric');
            
            if (guestId && metricType) {
                // Auto-select all text on focus for easy replacement
                input.addEventListener('focus', (event) => {
                    event.target.select();
                });
                
                // Update on input change
                input.addEventListener('input', (event) => {
                    let value = parseInt(event.target.value) || 0;
                    // Clamp value to valid range
                    value = Math.max(0, Math.min(100, value));
                    event.target.value = value;
                    // Update threshold and remove global indicator
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, value.toString(), true);
                });
                
                // Handle Enter key to blur input
                input.addEventListener('keydown', (event) => {
                    if (event.key === 'Enter') {
                        event.target.blur();
                    }
                });
                
                // Handle blur to ensure valid value
                input.addEventListener('blur', (event) => {
                    let value = parseInt(event.target.value) || 0;
                    value = Math.max(0, Math.min(100, value));
                    event.target.value = value;
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, value.toString(), true);
                });
            }
        });
        
        // Setup event listeners for all alert select dropdowns in this container
        const alertSelects = container.querySelectorAll('.alert-threshold-input select');
        alertSelects.forEach(select => {
            const container = select.closest('.alert-threshold-input');
            const guestId = container?.getAttribute('data-guest-id');
            const metricType = container?.getAttribute('data-metric');
            
            if (guestId && metricType) {
                // Only add event listener if one doesn't already exist
                if (!select.hasAttribute('data-listener-attached')) {
                    select.setAttribute('data-listener-attached', 'true');
                    
                    // Prevent dropdown from closing when clicking on it
                    select.addEventListener('mousedown', (event) => {
                        event.stopPropagation();
                    });
                    
                    select.addEventListener('mouseup', (event) => {
                        event.stopPropagation();
                    });
                    
                    select.addEventListener('click', (event) => {
                        event.stopPropagation();
                    });
                    
                    select.addEventListener('focus', (event) => {
                        event.stopPropagation();
                    });
                    
                    select.addEventListener('change', (event) => {
                        event.stopPropagation();
                        PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, event.target.value, true);
                    });
                }
            }
        });
        
        // Setup event listeners for stepper buttons
        const stepperButtons = container.querySelectorAll('.alert-threshold-input .stepper-button');
        stepperButtons.forEach(button => {
            button.addEventListener('click', (event) => {
                const targetId = button.getAttribute('data-stepper-target');
                const action = button.getAttribute('data-stepper-action');
                const input = document.getElementById(targetId);
                
                if (!input) return;
                
                const container = input.closest('.alert-threshold-input');
                const guestId = container?.getAttribute('data-guest-id');
                const metricType = container?.getAttribute('data-metric');
                
                if (!guestId || !metricType) return;
                
                // The stepper logic is handled by thresholds.js global handler
                // Just need to update the guest threshold after the value changes
                setTimeout(() => {
                    const value = input.value;
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, value, true);
                    
                    // Remove global indicator visual styling
                    container.classList.remove('using-global');
                    container.removeAttribute('data-using-global');
                }, 0);
            });
        });
        
        // Setup event listeners for all alert dropdowns in this container
        const alertDropdowns = container.querySelectorAll('.alert-threshold-input select');
        alertDropdowns.forEach(select => {
            const container = select.closest('.alert-threshold-input');
            const guestId = container?.getAttribute('data-guest-id');
            const metricType = container?.getAttribute('data-metric');
            
            if (guestId && metricType) {
                select.addEventListener('change', (e) => {
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, e.target.value);
                    
                    // Remove global indicator visual styling
                    container.classList.remove('using-global');
                    container.removeAttribute('data-using-global');
                });
            }
        });
        
        // Status alert dropdowns are already handled by the alertSelects loop above
        // No need for separate event listeners
    }

    function _initMobileScrollIndicators() {
        const tableContainer = document.querySelector('.table-container');
        const scrollHint = document.getElementById('scroll-hint');
        
        if (!tableContainer || !scrollHint) return;
        
        let scrollHintTimer;
        
        // Hide scroll hint after 5 seconds or on first scroll
        const hideScrollHint = () => {
            if (scrollHint) {
                scrollHint.style.display = 'none';
            }
        };
        
        scrollHintTimer = setTimeout(hideScrollHint, 5000);
        
        // Handle scroll events
        tableContainer.addEventListener('scroll', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
        
        // Also hide on table container click/touch
        tableContainer.addEventListener('touchstart', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
    }

    function _initTableFixedLine() {
        // No longer needed - using CSS border styling instead
    }

    function init() {
        // Attempt to find elements, with fallback retry mechanism
        function findElements() {
            searchInput = domCache.get('#dashboard-search');
            tableBodyEl = domCache.get('#main-table tbody');
            // statusElementEl no longer needed - guest count moved to node rows
            
            return tableBodyEl;
        }
        
        // Try to find elements immediately
        if (!findElements()) {
            console.warn('[Dashboard] Critical elements not found on first attempt, retrying...');
            // Retry after a short delay in case DOM is still loading
            setTimeout(() => {
                if (!findElements()) {
                    logger.error('Critical elements still not found after retry. Dashboard may not function properly.');
                    logger.error('Missing elements:', {
                        tableBodyEl: !!tableBodyEl
                    });
                }
            }, 100);
        }

        // Initialize chart system
        if (PulseApp.charts) {
            PulseApp.charts.startChartUpdates();
        }

        // Initialize charts toggle
        const chartsToggleCheckbox = document.getElementById('toggle-charts-checkbox');
        if (chartsToggleCheckbox) {
            chartsToggleCheckbox.addEventListener('change', toggleChartsMode);
        }
        
        // Time range dropdown event listener
        const timeRangeSelect = document.getElementById('time-range-select');
        if (timeRangeSelect) {
            timeRangeSelect.addEventListener('change', handleTimeRangeChange);
        }
        
        // Initialize mobile scroll indicators
        if (window.innerWidth < 768) {
            _initMobileScrollIndicators();
        }
        
        // Initialize fixed table line
        _initTableFixedLine();
        
        // Resize listener for progress bar text updates - DISABLED

        document.addEventListener('keydown', (event) => {
            // Handle Escape for resetting filters
            if (event.key === 'Escape') {
                const resetButton = document.getElementById('reset-filters-button');
                if (resetButton) {
                    resetButton.click(); // This will clear search and update table
                }
                return; // Done with Escape key
            }

            // General conditions to ignore this global listener:
            // 1. If already typing in an input, textarea, or select element.
            const targetElement = event.target;
            const targetTagName = targetElement.tagName;
            if (targetTagName === 'INPUT' || targetTagName === 'TEXTAREA' || targetTagName === 'SELECT') {
                return;
            }

            const snapshotModal = document.getElementById('snapshot-modal');
            if (snapshotModal && !snapshotModal.classList.contains('hidden')) {
                return;
            }
            // Add similar checks for other modals if they exist and should block this behavior.

            if (searchInput) { // searchInput is the module-scoped variable
                // Check !event.ctrlKey && !event.metaKey to avoid conflict with browser/OS shortcuts.
                if (event.key.length === 1 && !event.ctrlKey && !event.metaKey) {
                    if (document.activeElement !== searchInput) {
                        searchInput.focus();
                        event.preventDefault(); // Prevent default action (e.g., page scroll, find dialog)
                        searchInput.value += event.key; // Append the typed character
                        searchInput.dispatchEvent(new Event('input', { bubbles: true, cancelable: true })); // Trigger update
                    }
                    // If searchInput is already focused, browser handles the typing.
                } else if (event.key === 'Backspace' && !event.ctrlKey && !event.metaKey && !event.altKey) {
                    // For Backspace, if search not focused, focus it. Prevent default (e.g., browser back).
                    if (document.activeElement !== searchInput) {
                        searchInput.focus();
                        event.preventDefault();
                    }
                    // If searchInput is already focused, browser handles Backspace.
                }
            }
        });
    }

    function _calculateAverage(historyArray, key) {
        if (!historyArray || historyArray.length === 0) return null;
        const validEntries = historyArray.filter(entry => typeof entry[key] === 'number' && !isNaN(entry[key]));
        if (validEntries.length === 0) return null;
        const sum = validEntries.reduce((acc, curr) => acc + curr[key], 0);
        return sum / validEntries.length;
    }

    function _calculateAverageRate(historyArray, key) {
        if (!historyArray || historyArray.length < 2) return null;
        const validHistory = historyArray.filter(entry =>
            typeof entry.timestamp === 'number' && !isNaN(entry.timestamp) &&
            typeof entry[key] === 'number' && !isNaN(entry[key])
        );

        if (validHistory.length < 2) return null;

        const oldest = validHistory[0];
        const newest = validHistory[validHistory.length - 1];
        const valueDiff = newest[key] - oldest[key];
        const timeDiffSeconds = (newest.timestamp - oldest.timestamp) / 1000;

        if (timeDiffSeconds <= 0) return null;
        if (valueDiff < 0) return null;
        
        return valueDiff / timeDiffSeconds;
    }

    function _processSingleGuestData(guest) {
        let avgCpu = 0, avgMem = 0, avgDisk = 0;
        let avgDiskReadRate = null, avgDiskWriteRate = null, avgNetInRate = null, avgNetOutRate = null;
        let avgMemoryPercent = 'N/A', avgDiskPercent = 'N/A';
        let effectiveMemorySource = 'host';
        let currentMemForAvg = 0;
        let currentMemTotalForDisplay = guest.maxmem;

        const metricsData = PulseApp.state.get('metricsData') || [];
        const metrics = metricsData.find(m =>
            m.id === guest.vmid &&
            m.type === guest.type &&
            m.node === guest.node &&
            m.endpointId === guest.endpointId
        );
        const guestUniqueId = guest.id;
        

        const isDragging = PulseApp.ui.thresholds && PulseApp.ui.thresholds.isThresholdDragInProgress && PulseApp.ui.thresholds.isThresholdDragInProgress();
        const snapshot = guestMetricDragSnapshot[guestUniqueId];

        if (isDragging && snapshot) {
            avgDiskReadRate = snapshot.diskread;
            avgDiskWriteRate = snapshot.diskwrite;
            avgNetInRate = snapshot.netin;
            avgNetOutRate = snapshot.netout;

            if (guest.status === STATUS_RUNNING && metrics && metrics.current) {
                const currentDataPoint = { 
                    timestamp: Date.now(), 
                    ...metrics.current,
                    // Convert CPU to percentage for consistency
                    cpu: (metrics.current.cpu || 0) * 100
                };
                PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];
                avgCpu = _calculateAverage(history, 'cpu') ?? 0;
                avgMem = _calculateAverage(history, 'mem') ?? 0;
                avgDisk = _calculateAverage(history, 'disk') ?? 0;
            } else {
                PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }
        } else {
            if (guest.status === STATUS_RUNNING && metrics && metrics.current) {
                let baseMemoryValue = metrics.current.mem;
                currentMemTotalForDisplay = guest.maxmem;
                effectiveMemorySource = 'host';

                if (metrics.current.guest_mem_actual_used_bytes !== undefined && metrics.current.guest_mem_actual_used_bytes !== null) {
                    baseMemoryValue = metrics.current.guest_mem_actual_used_bytes;
                    effectiveMemorySource = 'guest';
                    if (metrics.current.guest_mem_total_bytes !== undefined && metrics.current.guest_mem_total_bytes > 0) {
                        currentMemTotalForDisplay = metrics.current.guest_mem_total_bytes;
                    }
                }
                currentMemForAvg = baseMemoryValue;

                const currentDataPoint = {
                    timestamp: Date.now(),
                    ...metrics.current,
                    cpu: (metrics.current.cpu || 0) * 100,
                    effective_mem: currentMemForAvg,
                    effective_mem_total: currentMemTotalForDisplay,
                    effective_mem_source: effectiveMemorySource
                };
                PulseApp.state.updateDashboardHistory(guestUniqueId, currentDataPoint);
                const history = PulseApp.state.getDashboardHistory()[guestUniqueId] || [];

                avgCpu = _calculateAverage(history, 'cpu') ?? 0;
                avgMem = _calculateAverage(history, 'effective_mem') ?? 0;
                avgDisk = _calculateAverage(history, 'disk') ?? 0;
                avgDiskReadRate = _calculateAverageRate(history, 'diskread');
                avgDiskWriteRate = _calculateAverageRate(history, 'diskwrite');
                avgNetInRate = _calculateAverageRate(history, 'netin');
                avgNetOutRate = _calculateAverageRate(history, 'netout');
            } else {
                PulseApp.state.clearDashboardHistoryEntry(guestUniqueId);
            }
        }

        const historyForGuest = PulseApp.state.getDashboardHistory()[guestUniqueId];
        let finalMemTotalForPercent = guest.maxmem;
        let finalMemSourceForTooltip = 'host';

        if (historyForGuest && historyForGuest.length > 0) {
            const lastHistoryEntry = historyForGuest[historyForGuest.length - 1];
            if (lastHistoryEntry.effective_mem_total !== undefined && lastHistoryEntry.effective_mem_total > 0) {
                finalMemTotalForPercent = lastHistoryEntry.effective_mem_total;
            }
            if (lastHistoryEntry.effective_mem_source) {
                finalMemSourceForTooltip = lastHistoryEntry.effective_mem_source;
            }
        }

        avgMemoryPercent = (finalMemTotalForPercent > 0 && typeof avgMem === 'number') ? Math.round(avgMem / finalMemTotalForPercent * 100) : 'N/A';
        avgDiskPercent = (guest.maxdisk > 0 && typeof avgDisk === 'number') ? Math.round(avgDisk / guest.maxdisk * 100) : 'N/A';
        
        let rawHostReportedMem = null;
        if (guest.status === STATUS_RUNNING && metrics && metrics.current && metrics.current.mem !== undefined) {
            rawHostReportedMem = metrics.current.mem;
        }

        const returnObj = {
            id: guestUniqueId,
            uniqueId: guestUniqueId,
            vmid: guest.vmid,
            name: guest.name || `${guest.type === 'qemu' ? GUEST_TYPE_VM : GUEST_TYPE_CT} ${guest.vmid}`,
            node: guest.nodeDisplayName || guest.node, // Use display name if available
            type: guest.type === 'qemu' ? GUEST_TYPE_VM : GUEST_TYPE_CT,
            status: guest.status,
            cpu: avgCpu,
            cpus: guest.cpus || 1,
            memory: avgMemoryPercent,
            memoryCurrent: avgMem,
            memoryTotal: finalMemTotalForPercent,
            memorySource: finalMemSourceForTooltip,
            rawHostMemory: rawHostReportedMem,
            disk: avgDiskPercent,
            diskCurrent: avgDisk,
            diskTotal: guest.maxdisk,
            uptime: guest.status === STATUS_RUNNING ? guest.uptime : 0,
            diskread: avgDiskReadRate,
            diskwrite: avgDiskWriteRate,
            netin: avgNetInRate,
            netout: avgNetOutRate
        };
        
        return returnObj;
    }

    function _setDashboardColumnWidths(dashboardData) {
        let maxNameLength = 0;
        let maxUptimeLength = 0;

        dashboardData.forEach(guest => {
            const uptimeFormatted = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.name.length > maxNameLength) maxNameLength = guest.name.length;
            if (uptimeFormatted.length > maxUptimeLength) maxUptimeLength = uptimeFormatted.length;
        });

        // More aggressive space optimization
        const nameColWidth = Math.min(Math.max(maxNameLength * 7 + 12, 80), 250);
        const uptimeColWidth = Math.max(maxUptimeLength * 6.5 + 8, 40);
        const htmlElement = document.documentElement;
        if (htmlElement) {
            htmlElement.style.setProperty('--name-col-width', `${nameColWidth}px`);
            htmlElement.style.setProperty('--uptime-col-width', `${uptimeColWidth}px`);
        }
    }

    function refreshDashboardData() {
        PulseApp.state.set('dashboardData', []);
        let dashboardData = [];

        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];

        vmsData.forEach(vm => dashboardData.push(_processSingleGuestData(vm)));
        containersData.forEach(ct => dashboardData.push(_processSingleGuestData(ct)));
        
        PulseApp.state.set('dashboardData', dashboardData);
        // Disabled dynamic width calculation to prevent column shifting
    }

    function _filterDashboardData(dashboardData, searchInput, filterGuestType, filterStatus, thresholdState) {
        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];

        return dashboardData.filter(guest => {
            const typeMatch = filterGuestType === FILTER_ALL ||
                              (filterGuestType === FILTER_VM && guest.type === GUEST_TYPE_VM) ||
                              (filterGuestType === FILTER_LXC && guest.type === GUEST_TYPE_CT);
            const statusMatch = filterStatus === FILTER_ALL || guest.status === filterStatus;

            const searchMatch = textSearchTerms.length === 0 || textSearchTerms.some(term =>
                (guest.name && guest.name.toLowerCase().includes(term)) ||
                (guest.node && guest.node.toLowerCase().includes(term)) ||
                (guest.vmid && guest.vmid.toString().includes(term)) ||
                (guest.uniqueId && guest.uniqueId.toString().includes(term))
            );

            // Check if any thresholds are active
            const hasActiveThresholds = Object.values(thresholdState).some(state => state && state.value > 0);
            
            // For threshold filtering, we'll add a property to track if thresholds are met
            // but don't filter out - we'll use this for styling instead
            let thresholdsMet = true;
            
            if (hasActiveThresholds) {
                for (const type in thresholdState) {
                    const state = thresholdState[type];
                    if (!state || state.value <= 0) continue;
                    
                    let guestValue;

                    if (type === METRIC_CPU) guestValue = guest.cpu;
                    else if (type === METRIC_MEMORY) guestValue = guest.memory;
                    else if (type === METRIC_DISK) guestValue = guest.disk;
                    else if (type === METRIC_DISK_READ) guestValue = guest.diskread;
                    else if (type === METRIC_DISK_WRITE) guestValue = guest.diskwrite;
                    else if (type === METRIC_NET_IN) guestValue = guest.netin;
                    else if (type === METRIC_NET_OUT) guestValue = guest.netout;
                    else continue;


                    if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) {
                        thresholdsMet = false;
                        break;
                    }
                    if (!(guestValue >= state.value)) {
                        thresholdsMet = false;
                        break;
                    }
                }
            }
            
            // Add threshold status to guest data for styling
            // Only set to false if we have active thresholds and guest doesn't meet them
            guest.meetsThresholds = hasActiveThresholds ? thresholdsMet : true;
            
            // Only filter out based on type, status and search - not thresholds
            return typeMatch && statusMatch && searchMatch;
        });
    }

    function _renderGroupedByNode(tableBody, sortedData, createRowFn) {
        const fnStartTime = performance.now();
        
        const nodeGroups = {};
        let visibleNodes = new Set();
        let visibleCount = 0;

        sortedData.forEach(guest => {
            const nodeName = guest.node || 'Unknown Node';
            if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
            nodeGroups[nodeName].push(guest);
        });
        
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        
        // Process nodes in order
        const sortedNodeNames = Object.keys(nodeGroups).sort();
        
        // Build all rows in a fragment first
        const fragment = document.createDocumentFragment();
        
        // Add node headers and their guests together
        sortedNodeNames.forEach(nodeName => {
            visibleNodes.add(nodeName.toLowerCase());
            
            // Create node header row
            const nodeHeaderRow = document.createElement('tr');
            nodeHeaderRow.className = 'node-header bg-gray-50 dark:bg-gray-700/50 font-semibold text-gray-700 dark:text-gray-300 text-xs';
            nodeHeaderRow.setAttribute('data-node-name', nodeName);
            
            // Check alert state
            const wouldTriggerNodeAlerts = alertStateCache.get(`node-${nodeName}`) || false;
            if (wouldTriggerNodeAlerts) {
                nodeHeaderRow.setAttribute('data-should-alert', 'true');
            }
            
            if (isAlertsMode) {
                nodeHeaderRow.innerHTML = _createNodeAlertRow(nodeName);
            } else {
                nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 11, 'td', wouldTriggerNodeAlerts);
            }
            
            fragment.appendChild(nodeHeaderRow);
            
            // Immediately add guests for this node
            nodeGroups[nodeName].forEach(guest => {
                const wouldTriggerGuestAlerts = alertStateCache.get(`guest-${guest.id}`) || false;
                
                const guestRow = createRowFn(guest);
                if (guestRow) {
                    fragment.appendChild(guestRow);
                    visibleCount++;
                }
            });
        });
        
        // Second pass: Replace entire table content at once
        const domUpdateStartTime = performance.now();
        
        // Clear table and append all rows at once
        tableBody.innerHTML = '';
        
        // Log what we're about to append
        const allRows = fragment.querySelectorAll('tr');
        let nodeCount = 0, guestCount = 0;
        allRows.forEach(row => {
            if (row.classList.contains('node-header')) {
                if (row.getAttribute('data-should-alert') === 'true') {
                    nodeCount++;
                }
            } else if (row.getAttribute('data-would-trigger-alert') === 'true') {
                guestCount++;
            }
        });
        
        tableBody.appendChild(fragment);
        
        // Check what actually got rendered
        const renderedAlertNodes = tableBody.querySelectorAll('tr.node-header[data-should-alert="true"]').length;
        const renderedAlertGuests = tableBody.querySelectorAll('tr[data-would-trigger-alert="true"]').length;
        
        return { visibleCount, visibleNodes };
    }

    // Incremental table update using DOM diffing
    function _updateTableIncremental(tableBody, sortedData, createRowFn, groupByNode) {
        const existingRows = new Map();
        const nodeHeaders = new Map();
        let visibleCount = 0;
        let visibleNodes = new Set();

        const children = tableBody.children;
        for (let i = 0; i < children.length; i++) {
            const row = children[i];
            if (row.classList.contains('node-header')) {
                // Try to find node name from different sources depending on mode
                let nodeName = null;
                
                // In alerts mode, look for the link or button with data-node
                const nodeButton = row.querySelector('button[data-node]');
                if (nodeButton) {
                    nodeName = nodeButton.getAttribute('data-node');
                } else {
                    // In normal mode or as fallback, get text from first td
                    const firstTd = row.querySelector('td');
                    if (firstTd) {
                        // Extract just the text, ignoring any child elements
                        nodeName = firstTd.textContent.split('\n')[0].trim();
                    }
                }
                
                if (nodeName) {
                    nodeHeaders.set(nodeName, row);
                }
            } else {
                const guestId = row.getAttribute('data-id');
                if (guestId) {
                    existingRows.set(guestId, row);
                }
            }
        }

        if (groupByNode) {
            const nodeGroups = {};
            for (let i = 0; i < sortedData.length; i++) {
                const guest = sortedData[i];
                const nodeName = guest.node || 'Unknown Node';
                if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
                nodeGroups[nodeName].push(guest);
            }

            // Process each node group
            let currentIndex = 0;
            Object.keys(nodeGroups).sort().forEach(nodeName => {
                visibleNodes.add(nodeName.toLowerCase());
                
                // Handle node header
                let nodeHeader = nodeHeaders.get(nodeName);
                const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
                
                if (!nodeHeader) {
                    // Create new node header
                    nodeHeader = PulseApp.ui.common.createTableRow({
                        classes: 'node-header bg-gray-50 dark:bg-gray-700/50',
                        baseClasses: '' // Override base classes for node headers
                    });
                }
                
                // Update node header based on mode
                _updateNodeRow(nodeHeader, nodeName, isAlertsMode);
                
                // Move or insert node header at correct position
                if (tableBody.children[currentIndex] !== nodeHeader) {
                    tableBody.insertBefore(nodeHeader, tableBody.children[currentIndex] || null);
                }
                currentIndex++;

                // Process guests in this node group
                nodeGroups[nodeName].forEach(guest => {
                    let guestRow = existingRows.get(guest.id);
                    if (guestRow) {
                        // Update existing row
                        _updateGuestRow(guestRow, guest);
                        existingRows.delete(guest.id);
                    } else {
                        // Create new row
                        guestRow = createRowFn(guest);
                    }
                    
                    if (guestRow) {
                        // Move or insert at correct position
                        if (tableBody.children[currentIndex] !== guestRow) {
                            tableBody.insertBefore(guestRow, tableBody.children[currentIndex] || null);
                        }
                        currentIndex++;
                        visibleCount++;
                    }
                });
            });

            // Remove unused node headers
            nodeHeaders.forEach((header, nodeName) => {
                if (!nodeGroups[nodeName] && header.parentNode) {
                    header.remove();
                }
            });
        } else {
            // Non-grouped update
            sortedData.forEach((guest, index) => {
                visibleNodes.add((guest.node || 'Unknown Node').toLowerCase());
                let guestRow = existingRows.get(guest.id);
                
                if (guestRow) {
                    // Update existing row
                    _updateGuestRow(guestRow, guest);
                    existingRows.delete(guest.id);
                } else {
                    // Create new row
                    guestRow = createRowFn(guest);
                }
                
                if (guestRow) {
                    // Move or insert at correct position
                    if (tableBody.children[index] !== guestRow) {
                        tableBody.insertBefore(guestRow, tableBody.children[index] || null);
                    }
                    visibleCount++;
                }
            });
        }

        // Remove any remaining unused rows
        existingRows.forEach(row => {
            if (row.parentNode) {
                row.remove();
            }
        });

        // Remove extra rows at the end
        const expectedRowCount = groupByNode ? visibleCount + visibleNodes.size : visibleCount;
        while (tableBody.children.length > expectedRowCount) {
            tableBody.lastChild.remove();
        }

        return { visibleCount, visibleNodes };
    }

    // Update an existing node row based on mode
    function _updateNodeRow(row, nodeName, isAlertsMode) {
        // Always update the row content to ensure yellow borders are applied
        // Use cached alert state for performance
        const wouldTriggerAlerts = alertStateCache.get(`node-${nodeName}`) || false;
        
        if (isAlertsMode) {
            const currentIsAlerts = row.querySelector('input[type="range"]') !== null;
            
            if (!currentIsAlerts) {
                // Switching to alerts mode
                row.innerHTML = _createNodeAlertRow(nodeName);
            } else {
                // Already in alerts mode - nothing to update for nodes
                
                // Update the yellow border on the first cell using CSS class
                const firstCell = row.querySelector('td:first-child');
                if (firstCell) {
                    if (wouldTriggerAlerts) {
                        firstCell.classList.add('alert-indicator');
                    } else {
                        firstCell.classList.remove('alert-indicator');
                    }
                }
            }
        } else {
            // Non-alerts mode
            row.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 11, 'td', wouldTriggerAlerts);
        }
    }

    // Update an existing guest row with new data
    function _updateGuestRow(row, guest) {
        // Update data attributes
        row.setAttribute('data-name', guest.name.toLowerCase());
        row.setAttribute('data-type', guest.type.toLowerCase());
        row.setAttribute('data-node', guest.node.toLowerCase());
        
        // Update class - apply dimming based on active mode
        const baseClasses = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        row.className = baseClasses;
        
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        
        // Check if guest would trigger alerts regardless of mode
        // This ensures the yellow border persists after leaving alerts mode
        // Don't set the attribute here during alerts mode - let updateAlertBorders handle it
        if (!isAlertsMode) {
            // Use cached value for performance when not in alerts mode
            const wouldTriggerAlerts = alertStateCache.get(`guest-${guest.id}`) || false;
            
            if (wouldTriggerAlerts) {
                row.setAttribute('data-would-trigger-alert', 'true');
            } else {
                row.removeAttribute('data-would-trigger-alert');
            }
        }
        // In alerts mode, never set alert attributes here - only updateAlertBorders should handle them
        
        if (isAlertsMode) {
            // Remove all dimming attributes when in alerts mode
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-dimmed');
            row.removeAttribute('data-alert-mixed');
            
            // Remove any cell-specific styling as well
            const cells = row.querySelectorAll('td');
            cells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.removeAttribute('data-alert-custom');
            });
        } else if (guest.meetsThresholds === false && document.getElementById('toggle-thresholds-checkbox')?.checked) {
            row.style.opacity = '0.4';
            row.style.transition = 'opacity 0.2s ease-in-out';
            row.setAttribute('data-dimmed', 'true');
            // Ensure alert dimming attribute is removed
            row.removeAttribute('data-alert-dimmed');
        } else {
            // No dimming needed
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-dimmed');
            row.removeAttribute('data-alert-dimmed');
        }
        
        // Update specific cells that might have changed
        const cells = row.querySelectorAll('td');
        
        // Ensure name cell keeps sticky styling even after row class updates
        if (cells[0]) {
            // Preserve the existing content while updating classes
            const content = cells[0].innerHTML;
            const title = cells[0].title;
            const newCell = PulseApp.ui.common.createStickyColumn(content, { title });
            cells[0].className = newCell.className;
        }
        if (cells.length >= 10) {
            
            // Update name (cell 0) with full HTML structure including indicators
            const thresholdIndicator = createThresholdIndicator(guest);
            const alertIndicator = createAlertIndicator(guest);
            const hasSecureBackup = hasRecentBackup(guest.vmid);
            const secureBackupIndicator = hasSecureBackup
                ? `<span style="color: #10b981; margin-right: 6px;" title="Backup detected within the last 24 hours">
                    <svg style="width: 14px; height: 14px; display: inline-block; vertical-align: middle;" fill="currentColor" viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
                        <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                    </svg>
                  </span>`
                : '';
            const nameHTML = `
                <div class="flex items-center gap-1">
                    ${secureBackupIndicator}
                    <span>${guest.name}</span>
                    ${alertIndicator}
                    ${thresholdIndicator}
                </div>
            `;
            // Always update to ensure secure backup indicator is shown
            cells[0].innerHTML = nameHTML;
            cells[0].title = guest.name;
            
            // Ensure ID cell (2) has proper classes
            if (cells[2]) {
                cells[2].className = 'py-1 px-2 align-middle';
            }
            
            const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
            
            // Ensure uptime cell (3) has proper classes
            if (cells[3]) {
                // Don't use overflow-hidden in alerts mode as it can hide dropdowns
                cells[3].className = isAlertsMode 
                    ? 'py-1 px-2 align-middle whitespace-nowrap'
                    : 'py-1 px-2 align-middle whitespace-nowrap overflow-hidden text-ellipsis';
            }

            const uptimeCell = cells[3];
            let newUptimeHTML = '-';
            if (guest.status === STATUS_RUNNING) {
                const formattedUptime = PulseApp.utils.formatUptime(guest.uptime);
                if (guest.uptime < 3600) { // Less than 1 hour
                    newUptimeHTML = `<span class="text-orange-500">${formattedUptime}</span>`;
                } else {
                    newUptimeHTML = formattedUptime;
                }
            }
            if (uptimeCell.innerHTML !== newUptimeHTML) {
                uptimeCell.innerHTML = newUptimeHTML;
            }

            const cpuCell = cells[4];
            if (isAlertsMode && cpuCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else if (guest.status === STATUS_RUNNING) {
                // Check if we already have the chart structure
                const existingChartContainer = cpuCell.querySelector(`#chart-${guest.id}-cpu`);
                const existingMetricText = cpuCell.querySelector('.metric-text');
                
                if (existingChartContainer && existingMetricText) {
                    // Update only the progress bar, preserve the chart container
                    const cpuPercent = Math.round(guest.cpu);
                    const cpuFullText = guest.cpus ? `${(guest.cpu * guest.cpus / 100).toFixed(1)}/${guest.cpus} cores` : `${cpuPercent}%`;
                    const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
                    const progressBar = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuFullText, cpuColorClass);
                    existingMetricText.innerHTML = progressBar;
                } else {
                    // Create the initial structure
                    const newCpuHTML = _createCpuBarHtml(guest);
                    cpuCell.innerHTML = newCpuHTML;
                }
            } else {
                // Not running or alerts mode without existing control
                const newCpuHTML = _createCpuBarHtml(guest);
                if (cpuCell.innerHTML !== newCpuHTML) {
                    cpuCell.innerHTML = newCpuHTML;
                }
            }

            const memCell = cells[5];
            if (isAlertsMode && memCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else if (guest.status === STATUS_RUNNING) {
                // Check if we already have the chart structure
                const existingChartContainer = memCell.querySelector(`#chart-${guest.id}-memory`);
                const existingMetricText = memCell.querySelector('.metric-text');
                
                if (existingChartContainer && existingMetricText) {
                    // Update only the progress bar, preserve the chart container
                    const memoryPercent = guest.memory;
                    const memoryFullText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)}`;
                    const memColorClass = PulseApp.utils.getUsageColor(memoryPercent, 'memory');
                    const progressBar = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryFullText, memColorClass);
                    existingMetricText.innerHTML = progressBar;
                } else {
                    // Create the initial structure
                    const newMemHTML = _createMemoryBarHtml(guest);
                    memCell.innerHTML = newMemHTML;
                }
            } else {
                // Not running or alerts mode without existing control
                const newMemHTML = _createMemoryBarHtml(guest);
                if (memCell.innerHTML !== newMemHTML) {
                    memCell.innerHTML = newMemHTML;
                }
            }

            const diskCell = cells[6];
            if (isAlertsMode && diskCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else if (!isAlertsMode && guest.status === STATUS_RUNNING && guest.type === GUEST_TYPE_CT) {
                // Check if we already have the chart structure
                const existingChartContainer = diskCell.querySelector(`#chart-${guest.id}-disk`);
                const existingMetricText = diskCell.querySelector('.metric-text');
                
                if (existingChartContainer && existingMetricText) {
                    // Update only the progress bar, preserve the chart container
                    const diskPercent = guest.disk;
                    const diskFullText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)}` : `${diskPercent}%`;
                    const diskColorClass = PulseApp.utils.getUsageColor(diskPercent, 'disk');
                    const progressBar = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskFullText, diskColorClass);
                    existingMetricText.innerHTML = progressBar;
                } else {
                    // Create the initial structure
                    const newDiskHTML = _createDiskBarHtml(guest);
                    diskCell.innerHTML = newDiskHTML;
                }
            } else {
                // Not running, not CT, or needs alert control
                const newDiskHTML = _createDiskBarHtml(guest);
                if (diskCell.innerHTML !== newDiskHTML) {
                    diskCell.innerHTML = newDiskHTML;
                    // Setup event listeners if in alerts mode
                    if (isAlertsMode) {
                        _setupAlertEventListeners(diskCell);
                    }
                }
            }

            // Ensure I/O cells (7-10) have proper classes
            [7, 8, 9, 10].forEach(index => {
                if (cells[index]) {
                    cells[index].className = 'py-1 px-2 align-middle';
                }
            });

            // isAlertsMode already declared above

            // Update I/O cells (7-10) if running
            if (guest.status === STATUS_RUNNING) {
                const diskReadCell = cells[7];
                let newDiskReadHTML;
                PulseApp.utils.updateGuestIOMetric(diskReadCell, guest, 'diskread', isAlertsMode);
                if (isAlertsMode && !diskReadCell.querySelector('select')) {
                    _setupAlertEventListeners(diskReadCell);
                }

                const diskWriteCell = cells[8];
                PulseApp.utils.updateGuestIOMetric(diskWriteCell, guest, 'diskwrite', isAlertsMode);
                if (isAlertsMode && !diskWriteCell.querySelector('select')) {
                    _setupAlertEventListeners(diskWriteCell);
                }

                const netInCell = cells[9];
                PulseApp.utils.updateGuestIOMetric(netInCell, guest, 'netin', isAlertsMode);
                if (isAlertsMode && !netInCell.querySelector('select')) {
                    _setupAlertEventListeners(netInCell);
                }

                if (cells[10]) {
                    const netOutCell = cells[10];
                    PulseApp.utils.updateGuestIOMetric(netOutCell, guest, 'netout', isAlertsMode);
                    if (isAlertsMode && !netOutCell.querySelector('select')) {
                        _setupAlertEventListeners(netOutCell);
                    }
                }
            } else {
                // Set I/O cells to '-' if not running, or show alert dropdowns if in alerts mode
                [7, 8, 9, 10].forEach(index => {
                    if (cells[index]) {
                        if (isAlertsMode) {
                            // Check if we already have a select element
                            const existingSelect = cells[index].querySelector('select');
                            if (!existingSelect) {
                                // Create new dropdown
                                const metricMap = { 7: 'diskread', 8: 'diskwrite', 9: 'netin', 10: 'netout' };
                                const newHTML = _createAlertDropdownHtml(guest.id, metricMap[index], PulseApp.utils.IO_ALERT_OPTIONS);
                                cells[index].innerHTML = newHTML;
                                // Re-setup event listeners for the new dropdown
                                _setupAlertEventListeners(cells[index]);
                            }
                            // If select exists, leave it alone - don't recreate
                        } else {
                            // Not in alerts mode, show '-'
                            if (cells[index].innerHTML !== '-') {
                                cells[index].innerHTML = '-';
                            }
                        }
                    }
                });
            }
        }
        
        // Reapply alert styling if in alerts mode
        if (PulseApp.ui.alerts?.isAlertsMode?.()) {
            // Trigger unified row-level styling update based on alert thresholds
            PulseApp.ui.alerts.updateRowStylingOnly?.();
        }
    }

    function _updateDashboardStatusMessage(statusElement, visibleCount, visibleNodes, groupByNode, filterGuestType, filterStatus, searchInput, thresholdState) {
        if (!statusElement) return;
        const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];
        
        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = textSearchTerms.length > 0 ? ` | Search: "${textSearchTerms.join(', ')}"` : '';
        const typeLabel = filterGuestType !== FILTER_ALL ? filterGuestType.toUpperCase() : '';
        const statusLabel = filterStatus !== FILTER_ALL ? filterStatus : '';
        const otherFilters = [typeLabel, statusLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        
        const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
        if (activeThresholds.length > 0) {
            const thresholdTexts = activeThresholds.map(([key, state]) => {
                return `${PulseApp.utils.getReadableThresholdName(key)}>=${PulseApp.utils.formatThresholdValue(key, state.value)}`;
            });
            statusFilterText += ` | Thresholds: ${thresholdTexts.join(', ')}`;
        }

        let statusCountText = ` | Showing ${visibleCount} guests`;
        if (groupByNode && visibleNodes.size > 0) statusCountText += ` across ${visibleNodes.size} nodes`;
        statusElement.textContent = statusBaseText + statusFilterText + statusCountText;
    }


    // Cache for previous table data to enable DOM diffing
    let previousTableData = null;
    let previousGroupByNode = null;

    // Cache for alert states to avoid recalculating during render
    let alertStateCache = new Map();
    
    function updateDashboardTable() {
        // Skip updates if syncing sliders or dragging global alert slider
        if (PulseApp.ui.alerts) {
            if ((PulseApp.ui.alerts.isSyncingSliders && PulseApp.ui.alerts.isSyncingSliders()) ||
                (PulseApp.ui.alerts.isDraggingGlobalSlider && PulseApp.ui.alerts.isDraggingGlobalSlider())) {
                return;
            }
        }
        
        // Skip updates if user is actively editing an input or select in alerts mode
        if (PulseApp.ui.alerts?.isAlertsMode?.() && document.activeElement && 
            ((document.activeElement.tagName === 'INPUT' && 
              document.activeElement.id && document.activeElement.id.includes('alert-')) ||
             document.activeElement.tagName === 'SELECT')) {
            return;
        }
        
        // If elements aren't initialized yet, try to initialize them
        if (!tableBodyEl) {
            tableBodyEl = document.querySelector('#main-table tbody');
            
            if (!tableBodyEl) {
                return;
            }
        }

        // Find the scrollable container
        const scrollableContainer = PulseApp.utils.getScrollableParent(tableBodyEl) || 
                                   document.querySelector('.table-container') ||
                                   tableBodyEl.closest('.overflow-x-auto');

        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;
        
        // Check if we're in alerts mode
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        
        // Preserve focus information if in alerts mode
        let focusedElement = null;
        let focusedId = null;
        let focusedValue = null;
        let focusedSelectionStart = null;
        let focusedSelectionEnd = null;
        
        if (isAlertsMode && document.activeElement && document.activeElement.tagName === 'INPUT') {
            focusedElement = document.activeElement;
            focusedId = focusedElement.id;
            focusedValue = focusedElement.value;
            if (focusedElement.type === 'number') {
                focusedSelectionStart = focusedElement.selectionStart;
                focusedSelectionEnd = focusedElement.selectionEnd;
            }
        }

        // Show loading skeleton if no data yet
        const currentData = PulseApp.state.get('dashboardData');
        if (!currentData || currentData.length === 0) {
            if (PulseApp.ui.loadingSkeletons && tableBodyEl) {
                PulseApp.ui.loadingSkeletons.showTableSkeleton(tableBodyEl.closest('table'), 5, 11);
            }
        }

        // Skip data refresh if we're in alerts mode and just updating thresholds
        // This prevents data from changing while adjusting alert thresholds
        const skipDataRefresh = isAlertsMode && currentData && currentData.length > 0;
        
        if (!skipDataRefresh) {
            refreshDashboardData();
        }

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const thresholdState = PulseApp.state.getThresholdState();
        const groupByNode = PulseApp.state.get('groupByNode');

        const filteredData = _filterDashboardData(dashboardData, searchInput, filterGuestType, filterStatus, thresholdState);
        const sortStateMain = PulseApp.state.getSortState('main');
        
        // In alerts mode, always sort by ID to prevent row position changes
        let sortedData;
        if (isAlertsMode) {
            sortedData = [...filteredData].sort((a, b) => a.id.localeCompare(b.id));
        } else {
            sortedData = PulseApp.utils.sortData(filteredData, sortStateMain.column, sortStateMain.direction, 'main');
        }
        
        // Pre-calculate all alert states to avoid staggered updates
        alertStateCache.clear();
        const startTime = performance.now();
        
        if (PulseApp.ui.alerts && PulseApp.ui.alerts.checkGuestWouldTriggerAlerts) {
            // Calculate guest alert states
            sortedData.forEach(guest => {
                const guestThresholds = PulseApp.ui.alerts.getGuestThresholds()[guest.id] || {};
                const wouldTrigger = PulseApp.ui.alerts.checkGuestWouldTriggerAlerts(guest.vmid, guestThresholds);
                alertStateCache.set(`guest-${guest.id}`, wouldTrigger);
            });
            
            // Calculate node alert states (always calculate, not just when grouped)
            if (PulseApp.ui.alerts.checkNodeWouldTriggerAlerts) {
                const nodeStartTime = performance.now();
                const nodesData = PulseApp.state.get('nodesData') || [];
                nodesData.forEach(node => {
                    const wouldTrigger = PulseApp.ui.alerts.checkNodeWouldTriggerAlerts(node.node);
                    alertStateCache.set(`node-${node.node}`, wouldTrigger);
                });
            }
        }

        let visibleCount = 0;
        let visibleNodes = new Set();

        // Check if we're switching between alerts mode and normal mode
        const previousIsAlertsMode = tableBodyEl?.querySelector('.node-header input[type="range"]') !== null;
        const modeChanged = previousIsAlertsMode !== isAlertsMode;
        
        // Don't do full rebuild if we're dragging the global alert slider
        const isDraggingAlertSlider = PulseApp.ui.alerts && PulseApp.ui.alerts.isDraggingGlobalSlider && PulseApp.ui.alerts.isDraggingGlobalSlider();
        const needsFullRebuild = !isDraggingAlertSlider && (previousGroupByNode !== groupByNode || previousTableData === null || (modeChanged && groupByNode));

        // Destroy existing virtual scroller if switching modes or data size changes significantly
        if (virtualScroller && (groupByNode || sortedData.length <= VIRTUAL_SCROLL_THRESHOLD)) {
            virtualScroller.destroy();
            virtualScroller = null;
            // Restore normal table structure
            const tableContainer = document.querySelector('.table-container');
            if (tableContainer) {
                tableContainer.style.height = '';
                tableContainer.innerHTML = '<table id="main-table" class="w-full min-w-[800px] table-auto text-xs sm:text-sm" role="table" aria-label="Virtual machines and containers"><tbody></tbody></table>';
                tableBodyEl = document.querySelector('#main-table tbody');
            }
        }

        // Use virtual scrolling for large datasets when not grouped
        if (!groupByNode && sortedData.length > VIRTUAL_SCROLL_THRESHOLD && PulseApp.virtualScroll) {
            const tableContainer = document.querySelector('.table-container');
            if (tableContainer && !virtualScroller) {
                // For virtual scrolling, use a much larger viewport or full viewport
                // This maintains performance while showing more content
                tableContainer.style.height = '90vh';
                virtualScroller = PulseApp.virtualScroll.createVirtualScroller(
                    tableContainer,
                    sortedData,
                    (guest) => {
                        const row = createGuestRow(guest);
                        // Remove hover effects for virtual rows
                        if (row) {
                            row.style.borderBottom = '1px solid rgb(229 231 235)';
                            row.classList.remove('hover:bg-gray-50', 'dark:hover:bg-gray-700/50');
                        }
                        return row;
                    }
                );
            } else if (virtualScroller) {
                // Preserve scroll position during virtual scroller updates
                const containerScrollTop = tableContainer.scrollTop;
                const containerScrollLeft = tableContainer.scrollLeft;
                
                virtualScroller.updateItems(sortedData);
                
                // Restore scroll position for virtual scroller
                if (containerScrollTop > 0 || containerScrollLeft > 0) {
                    requestAnimationFrame(() => {
                        tableContainer.scrollTop = containerScrollTop;
                        tableContainer.scrollLeft = containerScrollLeft;
                    });
                }
            }
            visibleCount = sortedData.length;
            sortedData.forEach(guest => visibleNodes.add((guest.node || 'Unknown Node').toLowerCase()));
        } else if (needsFullRebuild) {
            // Full rebuild for normal rendering with scroll preservation
            const renderStartTime = performance.now();
            
            PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
                if (groupByNode) {
                    const groupRenderResult = _renderGroupedByNode(tableBodyEl, sortedData, createGuestRow);
                    visibleCount = groupRenderResult.visibleCount;
                    visibleNodes = groupRenderResult.visibleNodes;
                } else {
                    PulseApp.utils.renderTableBody(tableBodyEl, sortedData, createGuestRow, "No matching guests found.", 11);
                    visibleCount = sortedData.length;
                    sortedData.forEach(guest => visibleNodes.add((guest.node || 'Unknown Node').toLowerCase()));
                }
            });
            previousGroupByNode = groupByNode;
            
        } else {
            // Incremental update using DOM diffing with scroll preservation
            PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
                const result = _updateTableIncremental(tableBodyEl, sortedData, createGuestRow, groupByNode);
                visibleCount = result.visibleCount;
                visibleNodes = result.visibleNodes;
            });
            
        }

        previousTableData = sortedData;

        if (visibleCount === 0 && tableBodyEl) {
            PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
                const textSearchTerms = searchInput ? searchInput.value.toLowerCase().split(',').map(term => term.trim()).filter(term => term) : [];
                const activeThresholds = Object.entries(thresholdState).filter(([_, state]) => state.value > 0);
                const thresholdTexts = activeThresholds.map(([key, state]) => {
                    return `${PulseApp.utils.getReadableThresholdName(key)}>=${PulseApp.utils.formatThresholdValue(key, state.value)}`;
                });
                
                const hasFilters = filterGuestType !== FILTER_ALL || filterStatus !== FILTER_ALL || textSearchTerms.length > 0 || activeThresholds.length > 0;
                
                if (PulseApp.ui.emptyStates) {
                    const context = {
                        filterType: filterGuestType,
                        filterStatus: filterStatus,
                        searchTerms: textSearchTerms,
                        thresholds: thresholdTexts
                    };
                    
                    const emptyType = hasFilters ? 'no-results' : 'no-guests';
                    tableBodyEl.innerHTML = PulseApp.ui.emptyStates.createTableEmptyState(emptyType, context, 11);
                } else {
                    // Fallback to simple message
                    let message = hasFilters ? "No guests match the current filters." : "No guests found.";
                    tableBodyEl.innerHTML = `<tr><td colspan="11" class="p-4 text-center text-gray-500 dark:text-gray-400">${message}</td></tr>`;
                }
            });
        }
        
        _updateDashboardStatusMessage(statusElementEl, visibleCount, visibleNodes, groupByNode, filterGuestType, filterStatus, searchInput, thresholdState);

        const mainSortColumn = sortStateMain.column;
        const mainHeader = document.querySelector(`#main-table th[data-sort="${mainSortColumn}"]`);
        if (PulseApp.ui && PulseApp.ui.common) {
            if (mainHeader) {
                PulseApp.ui.common.updateSortUI('main-table', mainHeader);
            } else {
                console.warn(`Sort header for column '${mainSortColumn}' not found in main table.`);
            }
        } else {
            console.warn('[Dashboard] PulseApp.ui.common not available for updateSortUI');
        }

        
        // Update charts immediately after table is rendered, but only if in charts mode
        const mainContainer = document.getElementById('main');
        if (PulseApp.charts && visibleCount > 0 && mainContainer && mainContainer.classList.contains('charts-mode')) {
            // Use requestAnimationFrame to ensure DOM is fully updated
            requestAnimationFrame(() => {
                PulseApp.charts.updateAllCharts();
            });
        }
        
        // Update progress bar texts based on available width - DISABLED
        
        // Additional scroll position restoration for both axes
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }
        
        // Restore focus if we had a focused element in alerts mode
        if (focusedId && isAlertsMode) {
            requestAnimationFrame(() => {
                const newElement = document.getElementById(focusedId);
                if (newElement && newElement.tagName === 'INPUT') {
                    newElement.focus();
                    // Restore the value if it was being edited
                    if (focusedValue !== null) {
                        newElement.value = focusedValue;
                    }
                    // For number inputs, restore selection
                    if (newElement.type === 'number' && focusedSelectionStart !== null) {
                        try {
                            newElement.setSelectionRange(focusedSelectionStart, focusedSelectionEnd);
                        } catch (e) {
                            // Some browsers don't support selection on number inputs
                            newElement.select();
                        }
                    }
                }
            });
        }
        
        // Re-add node threshold rows if in alerts mode
        if (PulseApp.ui.alerts && PulseApp.ui.alerts.isAlertsMode && PulseApp.ui.alerts.isAlertsMode()) {
            // Node list updates are now handled by the dashboard table update itself
            // when in alerts mode and grouped by node
            
            // Update alert borders after dashboard update
            if (PulseApp.ui.alerts.updateAlertBorders) {
                PulseApp.ui.alerts.updateAlertBorders();
            }
            
            // Reapply dimming classes to ensure proper styling after dashboard update
            if (PulseApp.ui.alerts.reapplyDimmingClasses) {
                PulseApp.ui.alerts.reapplyDimmingClasses();
            }
        }
        
        // Reapply threshold dimming if thresholds are active
        const thresholdsToggle = document.getElementById('toggle-thresholds-checkbox');
        if (thresholdsToggle && thresholdsToggle.checked && PulseApp.ui.thresholds) {
            // Use applyThresholdDimmingNow to immediately re-apply threshold styling
            if (PulseApp.ui.thresholds.applyThresholdDimmingNow) {
                PulseApp.ui.thresholds.applyThresholdDimmingNow();
            }
        }
    }

    function _createNodeAlertRow(nodeName) {
        // In alerts mode, just show the node name without any sliders
        const hostUrl = PulseApp.utils.getHostUrl(nodeName);
        let nodeLink = nodeName;
        if (hostUrl) {
            nodeLink = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer" title="Open ${nodeName} web interface">${nodeName}</a>`;
        }
        
        // Match the exact styling from generateNodeGroupHeaderCellHTML
        const baseClasses = 'px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400';
        
        // Create HTML matching the normal mode structure
        let html = `<td class="sticky left-0 ${baseClasses} bg-gray-50 dark:bg-gray-700/50">${nodeLink}</td>`;
        
        // Empty cells for all columns (11 total columns)
        html += `<td class="${baseClasses}"></td>`; // Type
        html += `<td class="${baseClasses}"></td>`; // VMID
        html += `<td class="${baseClasses}"></td>`; // Uptime
        html += `<td class="${baseClasses}"></td>`; // CPU
        html += `<td class="${baseClasses}"></td>`; // Memory
        html += `<td class="${baseClasses}"></td>`; // Disk
        html += `<td class="${baseClasses}"></td>`; // Disk Read
        html += `<td class="${baseClasses}"></td>`; // Disk Write
        html += `<td class="${baseClasses}"></td>`; // Net In
        html += `<td class="${baseClasses}"></td>`; // Net Out
        
        return html;
    }

    function _createCpuBarHtml(guest) {
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        if (isAlertsMode) {
            // In alerts mode, always show alert controls regardless of running status
            return _createAlertSliderHtml(guest.id, 'cpu', {
                min: 0,
                max: 100,
                step: 5,
                unit: '%'
            });
        }
        
        // Normal mode: only show metrics for running guests
        if (guest.status !== STATUS_RUNNING) return '-';
        
        const cpuPercent = Math.round(guest.cpu);
        const cpuFullText = guest.cpus ? `${(guest.cpu * guest.cpus / 100).toFixed(1)}/${guest.cpus} cores` : `${cpuPercent}%`;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuFullText, cpuColorClass);
        
        // Create both text and chart versions
        const guestId = guest.id;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'cpu') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createMemoryBarHtml(guest) {
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        if (isAlertsMode) {
            // In alerts mode, always show alert controls regardless of running status
            return _createAlertSliderHtml(guest.id, 'memory', {
                min: 0,
                max: 100,
                step: 5,
                unit: '%'
            });
        }
        
        // Normal mode: only show metrics for running guests
        if (guest.status !== STATUS_RUNNING) return '-';
        
        const memoryPercent = guest.memory;
        const memoryFullText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)}`;
        const memColorClass = PulseApp.utils.getUsageColor(memoryPercent, 'memory');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryFullText, memColorClass);
        
        // Create both text and chart versions
        const guestId = guest.id;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'memory') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createNodeGroupDataRow(node) {
        const row = document.createElement('tr');
        row.className = 'node-header bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700';
        row.setAttribute('data-node-id', node.node);
        
        const isOnline = node && node.uptime > 0;
        const statusText = isOnline ? 'online' : (node.status || 'unknown');
        const statusColor = isOnline ? 'text-green-500' : 'text-red-500';
        
        // Calculate guest counts for this node
        const dashboardData = PulseApp.state?.get('dashboardData') || [];
        const nodeGuests = dashboardData.filter(guest => guest.node === node.node);
        const runningGuests = nodeGuests.filter(guest => guest.status === 'running').length;
        const stoppedGuests = nodeGuests.filter(guest => guest.status === 'stopped').length;
        const totalGuests = nodeGuests.length;
        
        // Check if charts mode is active
        const isChartsMode = document.getElementById('toggle-charts-checkbox')?.checked || false;
        const mainContainer = document.getElementById('main');
        const chartsEnabled = isChartsMode && mainContainer && mainContainer.classList.contains('charts-mode');
        
        // Calculate percentages
        const cpuPercent = node.cpu ? (node.cpu * 100) : 0;
        const memPercent = (node.mem && node.maxmem > 0) ? (node.mem / node.maxmem * 100) : 0;
        const diskPercent = (node.disk && node.maxdisk > 0) ? (node.disk / node.maxdisk * 100) : 0;
        
        // Format values
        const cpuText = `${cpuPercent.toFixed(0)}%`;
        const memText = `${PulseApp.utils.formatBytes(node.mem || 0)} / ${PulseApp.utils.formatBytes(node.maxmem || 0)}`;
        const diskText = `${PulseApp.utils.formatBytes(node.disk || 0)} / ${PulseApp.utils.formatBytes(node.maxdisk || 0)}`;
        
        // Create progress bars with optional charts
        let cpuContent, memContent, diskContent;
        
        if (chartsEnabled && PulseApp.ui.nodes) {
            // Use the same functions from nodes.js to create content with charts
            cpuContent = PulseApp.ui.nodes._createNodeCpuBarHtml ? 
                PulseApp.ui.nodes._createNodeCpuBarHtml(node, true) :
                PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuText, PulseApp.utils.getUsageColor(cpuPercent, 'cpu'));
            
            memContent = PulseApp.ui.nodes._createNodeMemoryBarHtml ? 
                PulseApp.ui.nodes._createNodeMemoryBarHtml(node, true) :
                PulseApp.utils.createProgressTextBarHTML(memPercent, memText, PulseApp.utils.getUsageColor(memPercent, 'memory'));
            
            diskContent = PulseApp.ui.nodes._createNodeDiskBarHtml ? 
                PulseApp.ui.nodes._createNodeDiskBarHtml(node, true) :
                PulseApp.utils.createProgressTextBarHTML(diskPercent, diskText, PulseApp.utils.getUsageColor(diskPercent, 'disk'));
        } else {
            // Just progress bars without charts
            cpuContent = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuText, PulseApp.utils.getUsageColor(cpuPercent, 'cpu'));
            memContent = PulseApp.utils.createProgressTextBarHTML(memPercent, memText, PulseApp.utils.getUsageColor(memPercent, 'memory'));
            diskContent = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskText, PulseApp.utils.getUsageColor(diskPercent, 'disk'));
        }
        
        // Check if we can make the node name clickable
        const hostUrl = PulseApp.utils.getHostUrl(node.displayName || node.node);
        let nodeNameContent = node.displayName || node.node || 'Unknown';
        
        if (hostUrl) {
            nodeNameContent = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="text-gray-700 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150" title="Open ${node.displayName || node.node} web interface">${node.displayName || node.node || 'Unknown'}</a>`;
        }
        
        row.innerHTML = `
            <td class="sticky left-0 bg-gray-50 dark:bg-gray-800 z-10 py-1 px-2 font-medium text-gray-700 dark:text-gray-300">
                <div class="flex items-center gap-2">
                    <span class="h-2 w-2 rounded-full ${statusColor}"></span>
                    <div>
                        ${nodeNameContent}
                        <div class="text-xs text-gray-500 dark:text-gray-400 font-normal">
                            ${totalGuests} guest${totalGuests !== 1 ? 's' : ''} (${runningGuests} running, ${stoppedGuests} stopped)
                        </div>
                    </div>
                </div>
            </td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">NODE</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">-</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">${PulseApp.utils.formatUptime(node.uptime || 0)}</td>
            <td class="py-1 px-2 min-w-[160px]">${cpuContent}</td>
            <td class="py-1 px-2 min-w-[160px]">${memContent}</td>
            <td class="py-1 px-2 min-w-[160px]">${diskContent}</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">-</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">-</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">-</td>
            <td class="py-1 px-2 text-gray-700 dark:text-gray-300">-</td>
        `;
        
        return row;
    }

    function _createDiskBarHtml(guest) {
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        if (isAlertsMode) {
            // In alerts mode, show alert controls for all guests regardless of running status
            return _createAlertSliderHtml(guest.id, 'disk', {
                min: 0,
                max: 100,
                step: 5,
                unit: '%'
            });
        }
        
        // Normal mode: only show metrics for running guests
        if (guest.status !== STATUS_RUNNING) return '-';
        
        if (guest.type === GUEST_TYPE_CT) {
            const diskPercent = guest.disk;
            const diskFullText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)}` : `${diskPercent}%`;
            const diskColorClass = PulseApp.utils.getUsageColor(diskPercent, 'disk');
            const progressBar = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskFullText, diskColorClass);
            
            // Create both text and chart versions
            const guestId = guest.id;
            const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'disk') : '';
            
            return `
                <div class="metric-text">${progressBar}</div>
                <div class="metric-chart">${chartHtml}</div>
            `;
        } else {
            if (guest.diskTotal) {
                return `<span class="text-xs text-gray-700 dark:text-gray-300 truncate">${PulseApp.utils.formatBytes(guest.diskTotal)}</span>`;
            }
            return '-';
        }
    }

    function createThresholdIndicator(guest) {
        // Get current app state to check for custom thresholds
        const currentState = PulseApp.state.get();
        if (!currentState || !currentState.customThresholds) {
            return ''; // No custom thresholds data available
        }
        
        // Check if this guest has custom thresholds configured  
        // Note: We only check endpointId and vmid to support VM migration within clusters
        const hasCustomThresholds = currentState.customThresholds.some(config => 
            config.endpointId === guest.endpointId && 
            config.vmid === guest.id &&
            config.enabled
        );
        
        if (hasCustomThresholds) {
            return `
                <span class="inline-flex items-center justify-center w-3 h-3 text-xs font-bold text-white bg-blue-500 rounded-full" 
                      title="Custom alert thresholds configured">
                    T
                </span>
            `;
        }
        
        return '';
    }

    function createAlertIndicator(guest) {
        // Get active alerts for this guest
        const activeAlerts = PulseApp.alerts?.getActiveAlertsForGuest?.(guest.endpointId, guest.node, guest.id) || [];
        
        if (activeAlerts.length === 0) {
            return '';
        }
        
        // Simple alert indicator without severity levels
        const iconColor = 'bg-amber-500';
        const alertDetails = `${activeAlerts.length} alert${activeAlerts.length > 1 ? 's' : ''}`;
        
        const totalText = activeAlerts.length > 1 ? ` (${activeAlerts.length} total)` : '';
        
        return `
            <span class="inline-flex items-center justify-center w-3 h-3 text-xs font-bold text-white ${iconColor} rounded-full cursor-pointer alert-indicator" 
                  title="${alertDetails}${totalText} - Click to view details"
                  data-guest-id="${guest.endpointId}-${guest.node}-${guest.id}"
                  onclick="PulseApp.ui.dashboard.toggleGuestAlertDetails('${guest.endpointId}', '${guest.node}', '${guest.id}')">
                !
            </span>
        `;
    }

    function createGuestRow(guest) {
        const row = PulseApp.ui.common.createTableRow();
        
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        
        // Check if guest would trigger alerts (for all modes, not just alerts mode)
        // Don't set the attribute here during alerts mode - let updateAlertBorders handle it
        if (!isAlertsMode) {
            // Use cached value for performance when not in alerts mode
            const wouldTriggerAlerts = alertStateCache.get(`guest-${guest.id}`) || false;
            
            if (wouldTriggerAlerts) {
                // This will be applied to the first cell after it's created
                row.setAttribute('data-would-trigger-alert', 'true');
            }
        }
        // In alerts mode, never set alert attributes here - only updateAlertBorders should handle them
        
        if (isAlertsMode) {
            // Remove all dimming in alerts mode
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-dimmed');
        } else if (guest.meetsThresholds === false && document.getElementById('toggle-thresholds-checkbox')?.checked) {
            row.style.opacity = '0.4';
            row.style.transition = 'opacity 0.2s ease-in-out';
            row.setAttribute('data-dimmed', 'true');
        } else {
            // No dimming needed
            row.style.opacity = '';
            row.style.transition = '';
        }
        
        row.setAttribute('data-name', guest.name.toLowerCase());
        row.setAttribute('data-type', guest.type.toLowerCase());
        row.setAttribute('data-node', guest.node.toLowerCase());
        row.setAttribute('data-id', guest.id);

        // Check if guest has custom thresholds and alerts
        const thresholdIndicator = createThresholdIndicator(guest);
        const alertIndicator = createAlertIndicator(guest);

        const cpuBarHTML = _createCpuBarHtml(guest);
        const memoryBarHTML = _createMemoryBarHtml(guest);
        const diskBarHTML = _createDiskBarHtml(guest);

        const diskReadFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.diskread, 0) : '-';
        const diskWriteFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.diskwrite, 0) : '-';
        const netInFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.netin, 0) : '-';
        const netOutFormatted = guest.status === STATUS_RUNNING ? PulseApp.utils.formatSpeedWithStyling(guest.netout, 0) : '-';

        // Create I/O cells with both text and chart versions
        const guestId = guest.uniqueId;
        
        let diskReadCell, diskWriteCell, netInCell, netOutCell;
        
        if (isAlertsMode) {
            // Create alert dropdowns for I/O cells
            diskReadCell = _createAlertDropdownHtml(guest.id, 'diskread', [
                { value: '', label: 'No alert' },
                { value: '1048576', label: '> 1 MB/s' },
                { value: '10485760', label: '> 10 MB/s' },
                { value: '52428800', label: '> 50 MB/s' },
                { value: '104857600', label: '> 100 MB/s' }
            ]);
            diskWriteCell = _createAlertDropdownHtml(guest.id, 'diskwrite', [
                { value: '', label: 'No alert' },
                { value: '1048576', label: '> 1 MB/s' },
                { value: '10485760', label: '> 10 MB/s' },
                { value: '52428800', label: '> 50 MB/s' },
                { value: '104857600', label: '> 100 MB/s' }
            ]);
            netInCell = _createAlertDropdownHtml(guest.id, 'netin', [
                { value: '', label: 'No alert' },
                { value: '1048576', label: '> 1 MB/s' },
                { value: '10485760', label: '> 10 MB/s' },
                { value: '52428800', label: '> 50 MB/s' },
                { value: '104857600', label: '> 100 MB/s' }
            ]);
            netOutCell = _createAlertDropdownHtml(guest.id, 'netout', [
                { value: '', label: 'No alert' },
                { value: '1048576', label: '> 1 MB/s' },
                { value: '10485760', label: '> 10 MB/s' },
                { value: '52428800', label: '> 50 MB/s' },
                { value: '104857600', label: '> 100 MB/s' }
            ]);
        } else if (guest.status === STATUS_RUNNING && PulseApp.charts) {
            // Text versions - clean, no arrows
            const diskReadText = diskReadFormatted;
            const diskWriteText = diskWriteFormatted;
            const netInText = netInFormatted;
            const netOutText = netOutFormatted;
            
            // Chart versions - clean, no arrows
            const diskReadChart = PulseApp.charts.createSparklineHTML(guestId, 'diskread');
            const diskWriteChart = PulseApp.charts.createSparklineHTML(guestId, 'diskwrite');
            const netInChart = PulseApp.charts.createSparklineHTML(guestId, 'netin');
            const netOutChart = PulseApp.charts.createSparklineHTML(guestId, 'netout');
            
            diskReadCell = `<div class="metric-text">${diskReadText}</div><div class="metric-chart">${diskReadChart}</div>`;
            diskWriteCell = `<div class="metric-text">${diskWriteText}</div><div class="metric-chart">${diskWriteChart}</div>`;
            netInCell = `<div class="metric-text">${netInText}</div><div class="metric-chart">${netInChart}</div>`;
            netOutCell = `<div class="metric-text">${netOutText}</div><div class="metric-chart">${netOutChart}</div>`;
        } else {
            // Fallback to text only for stopped guests - no arrows
            diskReadCell = diskReadFormatted;
            diskWriteCell = diskWriteFormatted;
            netInCell = netInFormatted;
            netOutCell = netOutFormatted;
        }

        const typeIconClass = guest.type === GUEST_TYPE_VM
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guest.type === GUEST_TYPE_VM ? GUEST_TYPE_VM : 'LXC'}</span>`;

        let uptimeDisplay = '-';
        if (guest.status === STATUS_RUNNING) {
            uptimeDisplay = PulseApp.utils.formatUptime(guest.uptime);
            if (guest.uptime < 3600) { // Less than 1 hour (3600 seconds)
                uptimeDisplay = `<span class="text-orange-500">${uptimeDisplay}</span>`;
            }
        }

        // Create secure backup indicator
        const hasSecureBackup = hasRecentBackup(guest.vmid);
        const secureBackupIndicator = hasSecureBackup
            ? `<span style="color: #10b981; margin-right: 6px;" title="Backup within 24 hours">
                <svg style="width: 14px; height: 14px; display: inline-block; vertical-align: middle;" fill="currentColor" viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                </svg>
              </span>`
            : '';

        // Create sticky name column
        const nameContent = `
            <div class="flex items-center gap-1">
                ${secureBackupIndicator}
                <span>${guest.name}</span>
                ${alertIndicator}
                ${thresholdIndicator}
            </div>
        `;
        // Create sticky name cell
        const stickyNameCell = PulseApp.ui.common.createStickyColumn(nameContent, { 
            title: guest.name
        });
        
        row.appendChild(stickyNameCell);
        
        // Create regular cells
        row.appendChild(PulseApp.ui.common.createTableCell(typeIcon));
        row.appendChild(PulseApp.ui.common.createTableCell(guest.vmid));
        row.appendChild(PulseApp.ui.common.createTableCell(uptimeDisplay, 'py-1 px-2 align-middle whitespace-nowrap overflow-hidden text-ellipsis'));
        row.appendChild(PulseApp.ui.common.createTableCell(cpuBarHTML));
        row.appendChild(PulseApp.ui.common.createTableCell(memoryBarHTML));
        row.appendChild(PulseApp.ui.common.createTableCell(diskBarHTML));
        row.appendChild(PulseApp.ui.common.createTableCell(diskReadCell));
        row.appendChild(PulseApp.ui.common.createTableCell(diskWriteCell));
        row.appendChild(PulseApp.ui.common.createTableCell(netInCell));
        row.appendChild(PulseApp.ui.common.createTableCell(netOutCell));
        
        // Setup event listeners for alert sliders and dropdowns
        if (isAlertsMode) {
            _setupAlertEventListeners(row);
        }
        
        return row;
    }

    function snapshotGuestMetricsForDrag() {
        guestMetricDragSnapshot = {}; // Clear previous snapshot
        const currentDashboardData = PulseApp.state.get('dashboardData') || [];
        currentDashboardData.forEach(guest => {
            if (guest && guest.id) {
                // Snapshot ALL metrics for alert calculations
                guestMetricDragSnapshot[guest.id] = {
                    cpu: guest.cpu,
                    memory: guest.memory,
                    disk: guest.disk,
                    diskread: guest.diskread,
                    diskwrite: guest.diskwrite,
                    netin: guest.netin,
                    netout: guest.netout,
                    status: guest.status
                };
            }
        });
    }

    function clearGuestMetricSnapshots() {
        guestMetricDragSnapshot = {};
    }

    function handleTimeRangeChange() {
        const timeRangeSelect = document.getElementById('time-range-select');
        if (!timeRangeSelect) return;
        
        const selectedMinutes = parseInt(timeRangeSelect.value);
        
        // Update the chart data with new time range
        if (PulseApp.charts) {
            // Clear existing charts immediately for instant feedback
            document.querySelectorAll('.usage-chart-container svg, .sparkline-container svg').forEach(svg => {
                svg.style.opacity = '0.3';
            });
            
            // Force a fresh fetch with the new time range
            PulseApp.charts.fetchChartData().then((result) => {
                // Only update if we got valid data (not aborted)
                if (result) {
                    // Update all charts with the new data
                    PulseApp.charts.updateAllCharts(true); // Use immediate mode for faster updates
                    
                    // Restore chart opacity
                    document.querySelectorAll('.usage-chart-container svg, .sparkline-container svg').forEach(svg => {
                        svg.style.opacity = '';
                    });
                }
            }).catch(error => {
                // Only log non-abort errors
                if (error.name !== 'AbortError') {
                    console.error('[Dashboard] Failed to fetch chart data:', error);
                }
            });
        }
    }

    function toggleChartsMode() {
        const mainContainer = document.getElementById('main');
        const checkbox = document.getElementById('toggle-charts-checkbox');
        const label = checkbox ? checkbox.parentElement : null;
        const timeRangeContainer = document.getElementById('time-range-dropdown-container');
        
        if (checkbox && checkbox.checked) {
            // Switch to charts mode  
            mainContainer.classList.add('charts-mode');
            if (label) label.title = 'Toggle Metrics View';
            
            // Show charts controls
            if (PulseApp.ui.chartsControls) {
                PulseApp.ui.chartsControls.showChartsControls();
                // Auto-select the largest available time window when charts are toggled on
                if (PulseApp.charts && PulseApp.charts.getLargestAvailableTimeRange) {
                    const largestRange = PulseApp.charts.getLargestAvailableTimeRange();
                    PulseApp.ui.chartsControls.setTimeRange(largestRange);
                }
            }
            
            
            // Turn off thresholds toggle and hide its elements
            const thresholdsToggle = document.getElementById('toggle-thresholds-checkbox');
            if (thresholdsToggle && thresholdsToggle.checked) {
                thresholdsToggle.checked = false;
                thresholdsToggle.dispatchEvent(new Event('change'));
                
                // Wait for threshold change to complete before proceeding
                setTimeout(() => {
                    // Double-check that all styling is cleared after threshold toggle
                    if (PulseApp.ui.thresholds && PulseApp.ui.thresholds.clearAllStyling) {
                        PulseApp.ui.thresholds.clearAllStyling();
                    }
                }, 0);
            }
            
            // Clear threshold styling when entering charts mode
            if (PulseApp.ui.thresholds) {
                if (PulseApp.ui.thresholds.clearAllRowDimming) {
                    PulseApp.ui.thresholds.clearAllRowDimming();
                }
                // Use the more comprehensive clearAllStyling if available
                if (PulseApp.ui.thresholds.clearAllStyling) {
                    PulseApp.ui.thresholds.clearAllStyling();
                }
            }
            
            // Hide any lingering tooltips from thresholds
            if (PulseApp.tooltips) {
                if (PulseApp.tooltips.hideTooltip) {
                    PulseApp.tooltips.hideTooltip();
                }
                if (PulseApp.tooltips.hideSliderTooltipImmediately) {
                    PulseApp.tooltips.hideSliderTooltipImmediately();
                }
            }
            
            // Check if we're coming from alerts mode which modifies the DOM
            const wasInAlertsMode = document.querySelector('[data-original-content]');
            
            if (wasInAlertsMode) {
                // Coming from alerts mode - restore original content and clear charts
                const mainTable = document.getElementById('main-table');
                if (mainTable) {
                    const modifiedCells = mainTable.querySelectorAll('[data-original-content]');
                    modifiedCells.forEach(cell => {
                        cell.innerHTML = cell.dataset.originalContent;
                        delete cell.dataset.originalContent;
                    });
                    
                    // Clear all chart containers to force recreation
                    mainTable.querySelectorAll('.usage-chart-container, .sparkline-container').forEach(container => {
                        container.innerHTML = '';
                    });
                }
                
                // Now hide text and show charts
                document.querySelectorAll('.metric-text').forEach(el => el.style.display = 'none');
                document.querySelectorAll('.metric-chart').forEach(el => el.style.display = 'block');
            } else {
                // Just need to show charts - much faster
                document.querySelectorAll('.metric-text').forEach(el => el.style.display = 'none');
                document.querySelectorAll('.metric-chart').forEach(el => el.style.display = 'block');
            }
            
            // Ensure all rows and cells are fully visible when in charts mode
            const tableBody = document.querySelector('#main-table tbody');
            if (tableBody) {
                const rows = tableBody.querySelectorAll('tr[data-id]');
                rows.forEach(row => {
                    row.style.opacity = '';
                    row.style.transition = '';
                    row.removeAttribute('data-dimmed');
                    
                    // Also clear cell-level opacity
                    const cells = row.querySelectorAll('td');
                    cells.forEach(cell => {
                        cell.style.opacity = '';
                        cell.style.transition = '';
                    });
                });
                
                // Also ensure chart containers and SVGs are not dimmed
                tableBody.querySelectorAll('.usage-chart-container, .sparkline-container').forEach(container => {
                    container.style.opacity = '';
                    const svg = container.querySelector('svg');
                    if (svg) {
                        svg.style.opacity = '';
                    }
                });
                
                // Force a reflow to ensure styles are applied
                void tableBody.offsetHeight;
            }
            
            // Start fetching chart data if needed and update charts
            if (PulseApp.charts) {
                // Small delay to ensure DOM is ready after mode switch
                setTimeout(() => {
                    // Clear all dimming one more time after DOM is ready
                    const clearAllDimming = () => {
                        const tableBody = document.querySelector('#main-table tbody');
                        if (tableBody) {
                            // Clear row dimming
                            tableBody.querySelectorAll('tr[data-id]').forEach(row => {
                                row.style.opacity = '';
                                row.style.transition = '';
                                row.removeAttribute('data-dimmed');
                                row.removeAttribute('data-threshold-hidden');
                            });
                            
                            // Clear all cell dimming
                            tableBody.querySelectorAll('td').forEach(cell => {
                                cell.style.opacity = '';
                                cell.style.transition = '';
                            });
                            
                            // Clear chart container dimming - be more specific
                            tableBody.querySelectorAll('.usage-chart-container, .sparkline-container').forEach(container => {
                                container.style.setProperty('opacity', '1', 'important');
                                container.style.transition = '';
                                // Also check parent elements
                                let parent = container.parentElement;
                                while (parent && parent !== tableBody) {
                                    if (parent.style.opacity) {
                                        parent.style.opacity = '';
                                    }
                                    parent = parent.parentElement;
                                }
                            });
                            
                            // Clear metric-chart divs
                            tableBody.querySelectorAll('.metric-chart').forEach(chart => {
                                chart.style.setProperty('opacity', '1', 'important');
                                chart.style.transition = '';
                            });
                            
                            // Clear SVG dimming
                            tableBody.querySelectorAll('svg').forEach(svg => {
                                svg.style.setProperty('opacity', '1', 'important');
                                svg.style.transition = '';
                            });
                        }
                        
                        // Ensure tooltip containers are not affected by dimming
                        const customTooltip = document.getElementById('custom-tooltip');
                        const sliderTooltip = document.getElementById('slider-value-tooltip');
                        if (customTooltip) {
                            // Remove any inherited opacity but don't force visibility
                            customTooltip.style.removeProperty('opacity');
                            // Ensure the tooltip element itself can show at full opacity when needed
                            const parent = customTooltip.parentElement;
                            if (parent && parent.style.opacity) {
                                parent.style.opacity = '';
                            }
                        }
                        if (sliderTooltip) {
                            sliderTooltip.style.removeProperty('opacity');
                            const parent = sliderTooltip.parentElement;
                            if (parent && parent.style.opacity) {
                                parent.style.opacity = '';
                            }
                        }
                    };
                    
                    clearAllDimming();
                    
                    // First try to update with existing data
                    PulseApp.charts.updateAllCharts(true);
                    
                    // Clear dimming again after chart update
                    setTimeout(clearAllDimming, 10);
                    
                    // Then fetch fresh data if needed
                    if (PulseApp.charts.getChartData) {
                        PulseApp.charts.getChartData().then(() => {
                            // Update again with fresh data
                            PulseApp.charts.updateAllCharts(true);
                            // And clear dimming one final time
                            setTimeout(clearAllDimming, 10);
                            
                            // Remove the !important overrides after a delay
                            setTimeout(() => {
                                const tableBody = document.querySelector('#main-table tbody');
                                if (tableBody) {
                                    tableBody.querySelectorAll('.usage-chart-container, .sparkline-container, .metric-chart, svg').forEach(el => {
                                        el.style.opacity = '1';  // Keep opacity 1 but remove !important
                                    });
                                }
                            }, 500);
                        });
                    }
                }, 50); // 50ms delay to ensure DOM updates are complete
            }
            
            // Refresh node summary cards to show charts
            if (PulseApp.ui.nodes && PulseApp.ui.nodes.updateNodeSummaryCards) {
                const nodesData = PulseApp.state.get('nodesData');
                if (nodesData) {
                    PulseApp.ui.nodes.updateNodeSummaryCards(nodesData);
                }
            }
        } else {
            // Switch to metrics mode
            mainContainer.classList.remove('charts-mode');
            if (label) label.title = 'Toggle Charts View';
            
            
            // Hide charts controls
            if (PulseApp.ui.chartsControls) {
                PulseApp.ui.chartsControls.hideChartsControls();
            }
            
            // Remove inline styles to let CSS classes take over
            document.querySelectorAll('.metric-text').forEach(el => {
                el.style.display = '';  // Remove inline style
            });
            document.querySelectorAll('.metric-chart').forEach(el => {
                el.style.display = '';  // Remove inline style
            });
            
            // Re-apply threshold styling if thresholds are active
            const thresholdsToggle = document.getElementById('toggle-thresholds-checkbox');
            if (thresholdsToggle && thresholdsToggle.checked && PulseApp.ui.thresholds) {
                // Use applyThresholdDimmingNow to immediately re-apply threshold styling
                if (PulseApp.ui.thresholds.applyThresholdDimmingNow) {
                    PulseApp.ui.thresholds.applyThresholdDimmingNow();
                }
            }
            
            // Refresh node summary cards to hide charts
            if (PulseApp.ui.nodes && PulseApp.ui.nodes.updateNodeSummaryCards) {
                const nodesData = PulseApp.state.get('nodesData');
                if (nodesData) {
                    PulseApp.ui.nodes.updateNodeSummaryCards(nodesData);
                }
            }
        }
    }

    // Alert details expansion functionality
    function toggleGuestAlertDetails(endpointId, node, vmid) {
        const guestId = `${endpointId}-${node}-${vmid}`;
        const existingRow = document.querySelector(`tr[data-id="${vmid}"] + tr.alert-details-row`);
        
        if (existingRow) {
            // Collapse existing alert details
            existingRow.remove();
            return;
        }
        
        // Find the guest row
        const guestRow = document.querySelector(`tr[data-id="${vmid}"]`);
        if (!guestRow) return;
        
        // Get alerts for this guest
        const activeAlerts = PulseApp.alerts?.getActiveAlertsForGuest?.(endpointId, node, vmid) || [];
        
        if (activeAlerts.length === 0) {
            return;
        }
        
        // Create expanded alert details row
        const alertDetailsRow = document.createElement('tr');
        alertDetailsRow.className = 'alert-details-row bg-orange-50 dark:bg-orange-900/20 border-b border-orange-200 dark:border-orange-700';
        
        const alertsHTML = activeAlerts.map(alert => {
            const startTime = new Date(alert.startTime).toLocaleString();
            const duration = alert.startTime ? formatAlertDuration(Date.now() - alert.startTime) : 'Unknown';
            
            return `
                <div class="p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="flex items-start justify-between mb-2">
                        <div class="flex-1">
                            <h4 class="font-semibold text-gray-900 dark:text-gray-100 text-amber-600 dark:text-amber-400">
                                ${alert.name || 'Unknown Alert'}
                            </h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                ${alert.description || 'No description available'}
                            </p>
                        </div>
                        <div class="flex gap-2 ml-4">
                            ${!alert.acknowledged ? `
                                <button onclick="PulseApp.ui.dashboard.acknowledgeAlert('${alert.id}')" 
                                        class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-xs rounded transition-colors">
                                    Acknowledge
                                </button>
                            ` : `
                                <span class="px-3 py-1 bg-gray-500 text-white text-xs rounded">
                                    Acknowledged
                                </span>
                            `}
                        </div>
                    </div>
                    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
                        <div>
                            <span class="font-medium text-gray-700 dark:text-gray-300">Metric:</span>
                            <span class="text-gray-600 dark:text-gray-400">${alert.metricType || alert.metric || 'Unknown'}</span>
                        </div>
                        <div>
                            <span class="font-medium text-gray-700 dark:text-gray-300">Current Value:</span>
                            <span class="text-gray-600 dark:text-gray-400">${formatAlertValue(alert.currentValue, alert.metricType)}</span>
                        </div>
                        <div>
                            <span class="font-medium text-gray-700 dark:text-gray-300">Threshold:</span>
                            <span class="text-gray-600 dark:text-gray-400">${formatAlertValue(alert.effectiveThreshold || alert.threshold, alert.metricType)}</span>
                        </div>
                        <div>
                            <span class="font-medium text-gray-700 dark:text-gray-300">Started:</span>
                            <span class="text-gray-600 dark:text-gray-400">${startTime}</span>
                        </div>
                        <div>
                            <span class="font-medium text-gray-700 dark:text-gray-300">Duration:</span>
                            <span class="text-gray-600 dark:text-gray-400">${duration}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
        
        alertDetailsRow.innerHTML = `
            <td colspan="11" class="p-4">
                <div class="space-y-3">
                    <div class="flex items-center justify-between mb-3">
                        <h3 class="text-lg font-semibold text-orange-900 dark:text-orange-100">
                            Alert Details for ${guestRow.querySelector('span').textContent}
                        </h3>
                        <button onclick="PulseApp.ui.dashboard.toggleGuestAlertDetails('${endpointId}', '${node}', '${vmid}')" 
                                class="px-3 py-1 bg-gray-500 hover:bg-gray-600 text-white text-sm rounded transition-colors">
                            Collapse
                        </button>
                    </div>
                    ${alertsHTML}
                </div>
            </td>
        `;
        
        // Insert after the guest row
        guestRow.parentNode.insertBefore(alertDetailsRow, guestRow.nextSibling);
    }
    
    function acknowledgeAlert(alertId) {
        if (!alertId) return;
        
        // Call the alert management system to acknowledge
        if (PulseApp.alerts?.acknowledgeAlert) {
            PulseApp.alerts.acknowledgeAlert(alertId).then(() => {
                // Refresh the table to update alert indicators
                refreshDashboardData();
            }).catch(error => {
                console.error('Failed to acknowledge alert:', error);
                if (PulseApp.ui.toast) {
                    PulseApp.ui.toast.error('Failed to acknowledge alert');
                }
            });
        }
    }
    
    function formatAlertDuration(milliseconds) {
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        
        if (days > 0) return `${days}d ${hours % 24}h`;
        if (hours > 0) return `${hours}h ${minutes % 60}m`;
        if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
        return `${seconds}s`;
    }
    

    function formatAlertValue(value, metricType) {
        if (value === null || value === undefined) return 'N/A';
        
        switch (metricType) {
            case 'cpu':
            case 'memory':
            case 'disk':
                return `${Math.round(value)}%`;
            case 'diskread':
            case 'diskwrite':
            case 'netin':
            case 'netout':
                return PulseApp.utils?.formatBytes ? PulseApp.utils.formatBytes(value) + '/s' : `${value} B/s`;
            default:
                return String(value);
        }
    }

    function clearAlertCache() {
        alertStateCache.clear();
    }
    
    function updateAlertCache(key, value) {
        alertStateCache.set(key, value);
    }
    
    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow,
        snapshotGuestMetricsForDrag, // Export snapshot function
        clearGuestMetricSnapshots,    // Export clear function
        getGuestMetricSnapshot: () => guestMetricDragSnapshot, // Export getter for snapshot
        clearAlertCache,              // Export cache management functions
        updateAlertCache,
        toggleChartsMode,
        toggleGuestAlertDetails,
        acknowledgeAlert
    };
})();
