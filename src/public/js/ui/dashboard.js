PulseApp.ui = PulseApp.ui || {};

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

    // Alert helper functions (defined early to be available everywhere)
    function _createAlertSliderHtml(guestId, metricType, config) {
        // Get current threshold value for this guest/metric from alerts system
        const guestThresholds = PulseApp.ui.alerts?.getGuestThresholds?.() || {};
        const globalThresholds = PulseApp.ui.alerts?.getGlobalThresholds?.() || {};
        
        // Use individual guest threshold if set, otherwise use global threshold, otherwise use config min
        let currentValue = config.min;
        let isUsingGlobal = false;
        
        if (guestThresholds[guestId] && guestThresholds[guestId][metricType] !== undefined) {
            // Guest has individual threshold set
            currentValue = guestThresholds[guestId][metricType];
        } else if (globalThresholds[metricType] !== undefined && globalThresholds[metricType] !== '') {
            // Use global threshold as default
            currentValue = globalThresholds[metricType];
            isUsingGlobal = true;
        }
        
        // Use the threshold system's helper function to create identical HTML
        const sliderId = `alert-slider-${guestId}-${metricType}`;
        const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(
            sliderId, 
            config.min, 
            config.max, 
            config.step, 
            currentValue
        );
        
        // Add alert-specific data attributes to the container and visual indicator for global values
        const globalIndicator = isUsingGlobal ? ' data-using-global="true"' : '';
        const containerClass = isUsingGlobal ? 'alert-threshold-input using-global' : 'alert-threshold-input';
        
        return `
            <div class="${containerClass} h-5 leading-5" data-guest-id="${guestId}" data-metric="${metricType}"${globalIndicator}>
                ${sliderHtml}
            </div>
        `;
    }

    function _createAlertDropdownHtml(guestId, metricType, options) {
        // Get current threshold value for this guest/metric from alerts system
        const guestThresholds = PulseApp.ui.alerts?.getGuestThresholds?.() || {};
        const globalThresholds = PulseApp.ui.alerts?.getGlobalThresholds?.() || {};
        
        // Use individual guest threshold if set, otherwise use global threshold, otherwise use empty
        let currentValue = '';
        let isUsingGlobal = false;
        
        if (guestThresholds[guestId] && guestThresholds[guestId][metricType] !== undefined) {
            // Guest has individual threshold set
            currentValue = guestThresholds[guestId][metricType];
        } else if (globalThresholds[metricType] !== undefined) {
            // Use global threshold as default
            currentValue = globalThresholds[metricType];
            isUsingGlobal = true;
        }
        
        // Use the threshold system's helper function to create identical HTML
        const selectId = `alert-select-${guestId}-${metricType}`;
        const selectHtml = PulseApp.ui.thresholds.createThresholdSelectHtml(
            selectId, 
            options, 
            currentValue
        );
        
        // Add alert-specific data attributes to the container and visual indicator for global values
        const globalIndicator = isUsingGlobal ? ' data-using-global="true"' : '';
        const containerClass = isUsingGlobal ? 'alert-threshold-input using-global' : 'alert-threshold-input';
        
        return `
            <div class="${containerClass} h-5 leading-5" data-guest-id="${guestId}" data-metric="${metricType}"${globalIndicator}>
                ${selectHtml}
            </div>
        `;
    }

    // Helper function to setup event listeners for alert sliders and dropdowns
    function _setupAlertEventListeners(container) {
        if (!container) return;
        
        // Setup event listeners for all alert sliders in this container
        const alertSliders = container.querySelectorAll('.alert-threshold-input input[type="range"]');
        alertSliders.forEach(slider => {
            const container = slider.closest('.alert-threshold-input');
            const guestId = container?.getAttribute('data-guest-id');
            const metricType = container?.getAttribute('data-metric');
            
            if (guestId && metricType) {
                // Set up alert-specific events (save only on release, not during drag)
                slider.addEventListener('input', (event) => {
                    const value = event.target.value;
                    // Update during drag but don't save
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, value, false);
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                    
                    // Remove global indicator visual styling
                    container.classList.remove('using-global');
                    container.removeAttribute('data-using-global');
                });
                
                // Save only on release
                slider.addEventListener('change', (event) => {
                    const value = event.target.value;
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, value, true);
                });
                
                slider.addEventListener('mousedown', (event) => {
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                });
                
                slider.addEventListener('touchstart', (event) => {
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                }, { passive: true });
            }
        });
        
        // Setup event listeners for all alert dropdowns in this container
        const alertSelects = container.querySelectorAll('.alert-threshold-input select');
        alertSelects.forEach(select => {
            const container = select.closest('.alert-threshold-input');
            const guestId = container?.getAttribute('data-guest-id');
            const metricType = container?.getAttribute('data-metric');
            
            if (guestId && metricType) {
                select.addEventListener('change', (e) => {
                    // When user changes value, it becomes an individual threshold (no longer global)
                    PulseApp.ui.alerts.updateGuestThreshold(guestId, metricType, e.target.value);
                    
                    // Remove global indicator visual styling
                    container.classList.remove('using-global');
                    container.removeAttribute('data-using-global');
                });
            }
        });
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
            searchInput = document.getElementById('dashboard-search');
            tableBodyEl = document.querySelector('#main-table tbody');
            statusElementEl = document.getElementById('dashboard-status-text');
            
            return tableBodyEl && statusElementEl;
        }
        
        // Try to find elements immediately
        if (!findElements()) {
            console.warn('[Dashboard] Critical elements not found on first attempt, retrying...');
            // Retry after a short delay in case DOM is still loading
            setTimeout(() => {
                if (!findElements()) {
                    console.error('[Dashboard] Critical elements still not found after retry. Dashboard may not function properly.');
                    console.error('[Dashboard] Missing elements:', {
                        tableBodyEl: !!tableBodyEl,
                        statusElementEl: !!statusElementEl
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
        
        // Initialize mobile scroll indicators
        if (window.innerWidth < 768) {
            _initMobileScrollIndicators();
        }
        
        // Initialize fixed table line
        _initTableFixedLine();
        
        // Add resize listener for progress bar text updates
        window.addEventListener('resize', PulseApp.utils.updateProgressBarTextsDebounced);

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

            // 2. If a modal is active (e.g., snapshot-modal prevents background interaction)
            const snapshotModal = document.getElementById('snapshot-modal');
            if (snapshotModal && !snapshotModal.classList.contains('hidden')) {
                return;
            }
            // Add similar checks for other modals if they exist and should block this behavior.

            if (searchInput) { // searchInput is the module-scoped variable
                // For printable characters (letters, numbers, symbols, space)
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
        // _setDashboardColumnWidths(dashboardData);
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
        const nodeGroups = {};
        let visibleNodes = new Set();
        let visibleCount = 0;

        sortedData.forEach(guest => {
            const nodeName = guest.node || 'Unknown Node';
            if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
            nodeGroups[nodeName].push(guest);
        });
        
        tableBody.innerHTML = '';

        Object.keys(nodeGroups).sort().forEach(nodeName => {
            visibleNodes.add(nodeName.toLowerCase());
            const nodeHeaderRow = document.createElement('tr');
            nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
            nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 11, 'td');
            tableBody.appendChild(nodeHeaderRow);

            nodeGroups[nodeName].forEach(guest => {
                const guestRow = createRowFn(guest);
                if (guestRow) {
                    tableBody.appendChild(guestRow);
                    visibleCount++;
                }
            });
        });
        return { visibleCount, visibleNodes };
    }

    // Incremental table update using DOM diffing
    function _updateTableIncremental(tableBody, sortedData, createRowFn, groupByNode) {
        const existingRows = new Map();
        const nodeHeaders = new Map();
        let visibleCount = 0;
        let visibleNodes = new Set();

        // Build maps of existing rows and node headers (optimized)
        const children = tableBody.children;
        for (let i = 0; i < children.length; i++) {
            const row = children[i];
            if (row.classList.contains('node-header')) {
                const nodeText = row.querySelector('td').textContent.trim();
                nodeHeaders.set(nodeText, row);
            } else {
                const guestId = row.getAttribute('data-id');
                if (guestId) {
                    existingRows.set(guestId, row);
                }
            }
        }

        if (groupByNode) {
            // Group data by node (optimized)
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
                if (!nodeHeader) {
                    // Create new node header
                    nodeHeader = PulseApp.ui.common.createTableRow({
                        classes: 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs border-b border-gray-300 dark:border-gray-600',
                        baseClasses: '' // Override base classes for node headers
                    });
                    nodeHeader.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 11, 'td');
                }
                
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
        while (tableBody.children.length > (groupByNode ? visibleCount + visibleNodes.size : visibleCount)) {
            tableBody.lastChild.remove();
        }

        return { visibleCount, visibleNodes };
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
        
        if (isAlertsMode) {
            // Apply alert-specific dimming with granular cell control
            const allGuestThresholds = PulseApp.ui.alerts?.getGuestThresholds?.() || {};
            const guestThresholds = allGuestThresholds[guest.id] || {};
            const hasIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            if (!hasIndividualSettings) {
                // Using only global alert values - dim the whole row
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
                // Ensure threshold dimming attribute is removed
                row.removeAttribute('data-dimmed');
            } else {
                // Has some individual settings - apply cell-specific dimming immediately
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
                row.setAttribute('data-alert-mixed', 'true'); // Mark as having mixed values
                // Ensure threshold dimming attribute is removed
                row.removeAttribute('data-dimmed');
                
                // Apply granular cell styling immediately to prevent flashing
                const cells = row.querySelectorAll('td');
                if (cells.length >= 11) {
                    // Map metric types to cell indices
                    const metricCells = {
                        'cpu': cells[4], 'memory': cells[5], 'disk': cells[6],
                        'diskread': cells[7], 'diskwrite': cells[8], 'netin': cells[9], 'netout': cells[10]
                    };
                    
                    // Apply cell-specific opacity immediately
                    Object.entries(metricCells).forEach(([metricType, cell]) => {
                        if (!cell) return;
                        const hasCustomValue = guestThresholds[metricType] !== undefined;
                        cell.style.opacity = hasCustomValue ? '1' : '0.4';
                        cell.style.transition = 'opacity 0.1s ease-out';
                        if (hasCustomValue) {
                            cell.setAttribute('data-alert-custom', 'true');
                        } else {
                            cell.removeAttribute('data-alert-custom');
                        }
                    });
                    
                    // Dim non-metric cells
                    for (let i = 0; i < 4; i++) {
                        if (cells[i]) {
                            cells[i].style.opacity = '0.4';
                            cells[i].style.transition = 'opacity 0.1s ease-out';
                        }
                    }
                }
            }
        } else if (guest.meetsThresholds === false) {
            // Apply threshold dimming (only when not in alerts mode)
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
            // Cell order: name(0), type(1), id(2), uptime(3), cpu(4), memory(5), disk(6), diskread(7), diskwrite(8), netin(9), netout(10)
            
            // Update name (cell 0) with full HTML structure including indicators
            const thresholdIndicator = createThresholdIndicator(guest);
            const alertIndicator = createAlertIndicator(guest);
            const nameHTML = `
                <div class="flex items-center gap-1">
                    <span>${guest.name}</span>
                    ${alertIndicator}
                    ${thresholdIndicator}
                </div>
            `;
            if (cells[0].innerHTML !== nameHTML) {
                cells[0].innerHTML = nameHTML;
                cells[0].title = guest.name;
            }
            
            // Ensure ID cell (2) has proper classes
            if (cells[2]) {
                cells[2].className = 'py-1 px-2 align-middle';
            }
            
            // Ensure uptime cell (3) has proper classes
            if (cells[3]) {
                cells[3].className = 'py-1 px-2 align-middle whitespace-nowrap overflow-hidden text-ellipsis';
            }

            // Update uptime (cell 3)
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

            // Update CPU (cell 4)
            const cpuCell = cells[4];
            const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
            if (isAlertsMode && cpuCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else {
                const newCpuHTML = _createCpuBarHtml(guest);
                if (cpuCell.innerHTML !== newCpuHTML) {
                    cpuCell.innerHTML = newCpuHTML;
                }
            }

            // Update Memory (cell 5)
            const memCell = cells[5];
            if (isAlertsMode && memCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else {
                const newMemHTML = _createMemoryBarHtml(guest);
                if (memCell.innerHTML !== newMemHTML) {
                    memCell.innerHTML = newMemHTML;
                }
            }

            // Update Disk (cell 6)
            const diskCell = cells[6];
            if (isAlertsMode && diskCell.querySelector('.alert-threshold-input')) {
                // Skip update if already has alert control to preserve event listeners
            } else {
                const newDiskHTML = _createDiskBarHtml(guest);
                if (diskCell.innerHTML !== newDiskHTML) {
                    diskCell.innerHTML = newDiskHTML;
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
                // Disk Read (cell 7)
                const diskReadCell = cells[7];
                let newDiskReadHTML;
                if (isAlertsMode) {
                    // Only regenerate if not already an alert control
                    if (!diskReadCell.querySelector('.alert-threshold-input')) {
                        newDiskReadHTML = _createAlertDropdownHtml(guest.id, 'diskread', [
                            { value: '', label: 'No alert' },
                            { value: '1048576', label: '> 1 MB/s' },
                            { value: '10485760', label: '> 10 MB/s' },
                            { value: '52428800', label: '> 50 MB/s' },
                            { value: '104857600', label: '> 100 MB/s' }
                        ]);
                        diskReadCell.innerHTML = newDiskReadHTML;
                    }
                } else {
                    const diskReadFormatted = PulseApp.utils.formatSpeedWithStyling(guest.diskread, 0);
                    newDiskReadHTML = PulseApp.charts ? 
                        `<div class="metric-text">${diskReadFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'diskread')}</div>` :
                        diskReadFormatted;
                    if (diskReadCell.innerHTML !== newDiskReadHTML) {
                        diskReadCell.innerHTML = newDiskReadHTML;
                    }
                }

                // Disk Write (cell 8)
                const diskWriteCell = cells[8];
                let newDiskWriteHTML;
                if (isAlertsMode) {
                    // Only regenerate if not already an alert control
                    if (!diskWriteCell.querySelector('.alert-threshold-input')) {
                        newDiskWriteHTML = _createAlertDropdownHtml(guest.id, 'diskwrite', [
                            { value: '', label: 'No alert' },
                            { value: '1048576', label: '> 1 MB/s' },
                            { value: '10485760', label: '> 10 MB/s' },
                            { value: '52428800', label: '> 50 MB/s' },
                            { value: '104857600', label: '> 100 MB/s' }
                        ]);
                        diskWriteCell.innerHTML = newDiskWriteHTML;
                    }
                } else {
                    const diskWriteFormatted = PulseApp.utils.formatSpeedWithStyling(guest.diskwrite, 0);
                    newDiskWriteHTML = PulseApp.charts ? 
                        `<div class="metric-text">${diskWriteFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'diskwrite')}</div>` :
                        diskWriteFormatted;
                    if (diskWriteCell.innerHTML !== newDiskWriteHTML) {
                        diskWriteCell.innerHTML = newDiskWriteHTML;
                    }
                }

                // Net In (cell 9)
                const netInCell = cells[9];
                let newNetInHTML;
                if (isAlertsMode) {
                    // Only regenerate if not already an alert control
                    if (!netInCell.querySelector('.alert-threshold-input')) {
                        newNetInHTML = _createAlertDropdownHtml(guest.id, 'netin', [
                            { value: '', label: 'No alert' },
                            { value: '1048576', label: '> 1 MB/s' },
                            { value: '10485760', label: '> 10 MB/s' },
                            { value: '52428800', label: '> 50 MB/s' },
                            { value: '104857600', label: '> 100 MB/s' }
                        ]);
                        netInCell.innerHTML = newNetInHTML;
                    }
                } else {
                    const netInFormatted = PulseApp.utils.formatSpeedWithStyling(guest.netin, 0);
                    newNetInHTML = PulseApp.charts ? 
                        `<div class="metric-text">${netInFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'netin')}</div>` :
                        netInFormatted;
                    if (netInCell.innerHTML !== newNetInHTML) {
                        netInCell.innerHTML = newNetInHTML;
                    }
                }

                // Net Out (cell 10)
                if (cells[10]) {
                    const netOutCell = cells[10];
                    let newNetOutHTML;
                    if (isAlertsMode) {
                        // Only regenerate if not already an alert control
                        if (!netOutCell.querySelector('.alert-threshold-input')) {
                            newNetOutHTML = _createAlertDropdownHtml(guest.id, 'netout', [
                                { value: '', label: 'No alert' },
                                { value: '1048576', label: '> 1 MB/s' },
                                { value: '10485760', label: '> 10 MB/s' },
                                { value: '52428800', label: '> 50 MB/s' },
                                { value: '104857600', label: '> 100 MB/s' }
                            ]);
                            netOutCell.innerHTML = newNetOutHTML;
                        }
                    } else {
                        const netOutFormatted = PulseApp.utils.formatSpeedWithStyling(guest.netout, 0);
                        newNetOutHTML = PulseApp.charts ? 
                            `<div class="metric-text">${netOutFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'netout')}</div>` :
                            netOutFormatted;
                        if (netOutCell.innerHTML !== newNetOutHTML) {
                            netOutCell.innerHTML = newNetOutHTML;
                        }
                    }
                }
            } else {
                // Set I/O cells to '-' if not running, or show alert dropdowns if in alerts mode
                [7, 8, 9, 10].forEach(index => {
                    if (cells[index]) {
                        let newHTML = '-';
                        if (isAlertsMode) {
                            const metricMap = { 7: 'diskread', 8: 'diskwrite', 9: 'netin', 10: 'netout' };
                            newHTML = _createAlertDropdownHtml(guest.id, metricMap[index], [
                                { value: '', label: 'No alert' },
                                { value: '1048576', label: '> 1 MB/s' },
                                { value: '10485760', label: '> 10 MB/s' },
                                { value: '52428800', label: '> 50 MB/s' },
                                { value: '104857600', label: '> 100 MB/s' }
                            ]);
                        }
                        if (cells[index].innerHTML !== newHTML) {
                            cells[index].innerHTML = newHTML;
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

    function updateDashboardTable() {
        // If elements aren't initialized yet, try to initialize them
        if (!tableBodyEl || !statusElementEl) {
            tableBodyEl = document.querySelector('#main-table tbody');
            statusElementEl = document.getElementById('dashboard-status-text');
            
            // If still not found, silently return (not an error during initial load)
            if (!tableBodyEl || !statusElementEl) {
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

        // Show loading skeleton if no data yet
        const currentData = PulseApp.state.get('dashboardData');
        if (!currentData || currentData.length === 0) {
            if (PulseApp.ui.loadingSkeletons && tableBodyEl) {
                PulseApp.ui.loadingSkeletons.showTableSkeleton(tableBodyEl.closest('table'), 5, 11);
            }
        }

        refreshDashboardData();

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const thresholdState = PulseApp.state.getThresholdState();
        const groupByNode = PulseApp.state.get('groupByNode');

        const filteredData = _filterDashboardData(dashboardData, searchInput, filterGuestType, filterStatus, thresholdState);
        const sortStateMain = PulseApp.state.getSortState('main');
        const sortedData = PulseApp.utils.sortData(filteredData, sortStateMain.column, sortStateMain.direction, 'main');

        let visibleCount = 0;
        let visibleNodes = new Set();

        // Check if we need a full rebuild (grouping mode changed or first render)
        const needsFullRebuild = previousGroupByNode !== groupByNode || previousTableData === null;

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
                // Set fixed height for virtual scroll container
                tableContainer.style.height = '600px';
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
        
        // Update progress bar texts based on available width
        requestAnimationFrame(() => {
            PulseApp.utils.updateProgressBarTexts();
        });
        
        // Additional scroll position restoration for both axes
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }
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
        const guestId = guest.uniqueId;
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
        const guestId = guest.uniqueId;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'memory') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createDiskBarHtml(guest) {
        // Check if alerts mode is active
        const isAlertsMode = PulseApp.ui.alerts?.isAlertsMode?.() || false;
        if (isAlertsMode && guest.type === GUEST_TYPE_CT) {
            // In alerts mode, always show alert controls for CTs regardless of running status
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
            const guestId = guest.uniqueId;
            const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'disk') : '';
            
            return `
                <div class="metric-text">${progressBar}</div>
                <div class="metric-chart">${chartHtml}</div>
            `;
        } else {
            if (guest.diskTotal) {
                return `<span class="text-xs text-gray-700 dark:text-gray-200 truncate">${PulseApp.utils.formatBytes(guest.diskTotal)}</span>`;
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
        const iconColor = 'bg-red-500';
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
        
        if (isAlertsMode) {
            // Apply alert-specific dimming with granular cell control
            const allGuestThresholds = PulseApp.ui.alerts?.getGuestThresholds?.() || {};
            const guestThresholds = allGuestThresholds[guest.id] || {};
            const hasIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            if (!hasIndividualSettings) {
                // Using only global alert values - dim the whole row
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
            } else {
                // Has some individual settings - mark as mixed for later cell styling
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
                row.setAttribute('data-alert-mixed', 'true'); // Mark as having mixed values
                
                // Note: Cell styling will be applied after row HTML is complete
                row.setAttribute('data-needs-cell-styling', JSON.stringify(guestThresholds));
            }
        } else if (guest.meetsThresholds === false) {
            // Apply threshold dimming (only when not in alerts mode)
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

        // Create sticky name column
        const nameContent = `
            <div class="flex items-center gap-1">
                <span>${guest.name}</span>
                ${alertIndicator}
                ${thresholdIndicator}
            </div>
        `;
        const stickyNameCell = PulseApp.ui.common.createStickyColumn(nameContent, { title: guest.name });
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
            
            // Apply cell styling if this row needs it (mixed values)
            if (row.hasAttribute('data-needs-cell-styling')) {
                const guestThresholds = JSON.parse(row.getAttribute('data-needs-cell-styling'));
                const cells = row.querySelectorAll('td');
                
                if (cells.length >= 11) {
                    // Map metric types to cell indices
                    const metricCells = {
                        'cpu': cells[4], 'memory': cells[5], 'disk': cells[6],
                        'diskread': cells[7], 'diskwrite': cells[8], 'netin': cells[9], 'netout': cells[10]
                    };
                    
                    // Apply cell-specific opacity immediately
                    Object.entries(metricCells).forEach(([metricType, cell]) => {
                        if (!cell) return;
                        const hasCustomValue = guestThresholds[metricType] !== undefined;
                        cell.style.opacity = hasCustomValue ? '1' : '0.4';
                        cell.style.transition = 'opacity 0.1s ease-out';
                        if (hasCustomValue) {
                            cell.setAttribute('data-alert-custom', 'true');
                        } else {
                            cell.removeAttribute('data-alert-custom');
                        }
                    });
                    
                    // Dim non-metric cells
                    for (let i = 0; i < 4; i++) {
                        if (cells[i]) {
                            cells[i].style.opacity = '0.4';
                            cells[i].style.transition = 'opacity 0.1s ease-out';
                        }
                    }
                }
                
                // Clean up the temporary attribute
                row.removeAttribute('data-needs-cell-styling');
            }
        }
        
        return row;
    }

    function snapshotGuestMetricsForDrag() {
        guestMetricDragSnapshot = {}; // Clear previous snapshot
        const currentDashboardData = PulseApp.state.get('dashboardData') || [];
        currentDashboardData.forEach(guest => {
            if (guest && guest.uniqueId) {
                guestMetricDragSnapshot[guest.uniqueId] = {
                    diskread: guest.diskread,
                    diskwrite: guest.diskwrite,
                    netin: guest.netin,
                    netout: guest.netout
                    // Optionally snapshot other metrics if they also show issues
                };
            }
        });
    }

    function clearGuestMetricSnapshots() {
        guestMetricDragSnapshot = {};
    }

    function toggleChartsMode() {
        const mainContainer = document.getElementById('main');
        const checkbox = document.getElementById('toggle-charts-checkbox');
        const label = checkbox ? checkbox.parentElement : null;
        
        if (checkbox && checkbox.checked) {
            // Switch to charts mode  
            mainContainer.classList.add('charts-mode');
            if (label) label.title = 'Toggle Metrics View';
            
            // Immediately render charts when switching to charts mode
            if (PulseApp.charts) {
                requestAnimationFrame(() => {
                    PulseApp.charts.updateAllCharts();
                });
            }
        } else {
            // Switch to metrics mode
            mainContainer.classList.remove('charts-mode');
            if (label) label.title = 'Toggle Charts View';
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
                            <h4 class="font-semibold text-gray-900 dark:text-gray-100 text-red-600 dark:text-red-400">
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

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow,
        snapshotGuestMetricsForDrag, // Export snapshot function
        clearGuestMetricSnapshots,    // Export clear function
        toggleChartsMode,
        toggleGuestAlertDetails,
        acknowledgeAlert
    };
})();
