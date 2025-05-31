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

        return {
            id: guest.vmid,
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
        _setDashboardColumnWidths(dashboardData);
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

            let thresholdsMet = true;
            for (const type in thresholdState) {
                const state = thresholdState[type];
                let guestValue;

                if (type === METRIC_CPU) guestValue = guest.cpu;
                else if (type === METRIC_MEMORY) guestValue = guest.memory;
                else if (type === METRIC_DISK) guestValue = guest.disk;
                else if (type === METRIC_DISK_READ) guestValue = guest.diskread;
                else if (type === METRIC_DISK_WRITE) guestValue = guest.diskwrite;
                else if (type === METRIC_NET_IN) guestValue = guest.netin;
                else if (type === METRIC_NET_OUT) guestValue = guest.netout;
                else continue;

                if (state.value > 0) {
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
            return typeMatch && statusMatch && searchMatch && thresholdsMet;
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

        // Build maps of existing rows and node headers
        Array.from(tableBody.children).forEach(row => {
            if (row.classList.contains('node-header')) {
                const nodeText = row.querySelector('td').textContent.trim();
                nodeHeaders.set(nodeText, row);
            } else {
                const guestId = row.getAttribute('data-id');
                if (guestId) {
                    existingRows.set(guestId, row);
                }
            }
        });

        if (groupByNode) {
            // Group data by node
            const nodeGroups = {};
            sortedData.forEach(guest => {
                const nodeName = guest.node || 'Unknown Node';
                if (!nodeGroups[nodeName]) nodeGroups[nodeName] = [];
                nodeGroups[nodeName].push(guest);
            });

            // Process each node group
            let currentIndex = 0;
            Object.keys(nodeGroups).sort().forEach(nodeName => {
                visibleNodes.add(nodeName.toLowerCase());
                
                // Handle node header
                let nodeHeader = nodeHeaders.get(nodeName);
                if (!nodeHeader) {
                    // Create new node header
                    nodeHeader = document.createElement('tr');
                    nodeHeader.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
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
        
        // Update class - same styling for both stopped and running
        row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        
        // Update specific cells that might have changed
        const cells = row.querySelectorAll('td');
        
        // Ensure name cell keeps sticky styling even after row class updates
        if (cells[0]) {
            cells[0].className = 'sticky left-0 bg-white dark:bg-gray-800 z-10 p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 border-r border-gray-300 dark:border-gray-600';
        }
        if (cells.length >= 10) {
            // Cell order: name(0), type(1), id(2), uptime(3), cpu(4), memory(5), disk(6), diskread(7), diskwrite(8), netin(9), netout(10)
            
            // Update name (cell 0) content only (styling handled above)
            if (cells[0].textContent !== guest.name) {
                cells[0].textContent = guest.name;
                cells[0].title = guest.name;
            }
            
            // Ensure ID cell (2) has proper classes
            if (cells[2]) {
                cells[2].className = 'p-1 px-2';
            }
            
            // Ensure uptime cell (3) has proper classes
            if (cells[3]) {
                cells[3].className = 'p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis';
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
            const newCpuHTML = _createCpuBarHtml(guest);
            if (cpuCell.innerHTML !== newCpuHTML) {
                cpuCell.innerHTML = newCpuHTML;
            }

            // Update Memory (cell 5)
            const memCell = cells[5];
            const newMemHTML = _createMemoryBarHtml(guest);
            if (memCell.innerHTML !== newMemHTML) {
                memCell.innerHTML = newMemHTML;
            }

            // Update Disk (cell 6)
            const diskCell = cells[6];
            const newDiskHTML = _createDiskBarHtml(guest);
            if (diskCell.innerHTML !== newDiskHTML) {
                diskCell.innerHTML = newDiskHTML;
            }

            // Ensure I/O cells (7-10) have proper classes
            [7, 8, 9, 10].forEach(index => {
                if (cells[index]) {
                    cells[index].className = 'p-1 px-2';
                }
            });

            // Update I/O cells (7-10) if running
            if (guest.status === STATUS_RUNNING) {
                // Disk Read (cell 7)
                const diskReadCell = cells[7];
                const diskReadFormatted = PulseApp.utils.formatSpeedWithStyling(guest.diskread, 0);
                const newDiskReadHTML = PulseApp.charts ? 
                    `<div class="metric-text">${diskReadFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'diskread')}</div>` :
                    diskReadFormatted;
                if (diskReadCell.innerHTML !== newDiskReadHTML) {
                    diskReadCell.innerHTML = newDiskReadHTML;
                }

                // Disk Write (cell 8)
                const diskWriteCell = cells[8];
                const diskWriteFormatted = PulseApp.utils.formatSpeedWithStyling(guest.diskwrite, 0);
                const newDiskWriteHTML = PulseApp.charts ? 
                    `<div class="metric-text">${diskWriteFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'diskwrite')}</div>` :
                    diskWriteFormatted;
                if (diskWriteCell.innerHTML !== newDiskWriteHTML) {
                    diskWriteCell.innerHTML = newDiskWriteHTML;
                }

                // Net In (cell 9)
                const netInCell = cells[9];
                const netInFormatted = PulseApp.utils.formatSpeedWithStyling(guest.netin, 0);
                const newNetInHTML = PulseApp.charts ? 
                    `<div class="metric-text">${netInFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'netin')}</div>` :
                    netInFormatted;
                if (netInCell.innerHTML !== newNetInHTML) {
                    netInCell.innerHTML = newNetInHTML;
                }

                // Net Out (cell 10)
                if (cells[10]) {
                    const netOutCell = cells[10];
                    const netOutFormatted = PulseApp.utils.formatSpeedWithStyling(guest.netout, 0);
                    const newNetOutHTML = PulseApp.charts ? 
                        `<div class="metric-text">${netOutFormatted}</div><div class="metric-chart">${PulseApp.charts.createSparklineHTML(guest.uniqueId, 'netout')}</div>` :
                        netOutFormatted;
                    if (netOutCell.innerHTML !== newNetOutHTML) {
                        netOutCell.innerHTML = newNetOutHTML;
                    }
                }
            } else {
                // Set I/O cells to '-' if not running
                [7, 8, 9, 10].forEach(index => {
                    if (cells[index] && cells[index].innerHTML !== '-') {
                        cells[index].innerHTML = '-';
                    }
                });
            }
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
        if (guest.status !== STATUS_RUNNING) return '-';
        const cpuPercent = Math.round(guest.cpu);
        const cpuTooltipText = `${cpuPercent}% ${guest.cpus ? `(${(guest.cpu * guest.cpus / 100).toFixed(1)}/${guest.cpus} cores)` : ''}`;
        const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass, `${cpuPercent}%`);
        
        // Create both text and chart versions
        const guestId = guest.uniqueId;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'cpu') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createMemoryBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        const memoryPercent = guest.memory;
        let memoryTooltipText = `${PulseApp.utils.formatBytes(guest.memoryCurrent)} / ${PulseApp.utils.formatBytes(guest.memoryTotal)} (${memoryPercent}%)`;
        if (guest.type === GUEST_TYPE_VM && guest.memorySource === 'guest' && guest.rawHostMemory !== null && guest.rawHostMemory !== undefined) {
            memoryTooltipText += ` (Host: ${PulseApp.utils.formatBytes(guest.rawHostMemory)})`;
        }
        const memColorClass = PulseApp.utils.getUsageColor(memoryPercent, 'memory');
        const progressBar = PulseApp.utils.createProgressTextBarHTML(memoryPercent, memoryTooltipText, memColorClass, `${memoryPercent}%`);
        
        // Create both text and chart versions
        const guestId = guest.uniqueId;
        const chartHtml = PulseApp.charts ? PulseApp.charts.createUsageChartHTML(guestId, 'memory') : '';
        
        return `
            <div class="metric-text">${progressBar}</div>
            <div class="metric-chart">${chartHtml}</div>
        `;
    }

    function _createDiskBarHtml(guest) {
        if (guest.status !== STATUS_RUNNING) return '-';
        if (guest.type === GUEST_TYPE_CT) {
            const diskPercent = guest.disk;
            const diskTooltipText = guest.diskTotal ? `${PulseApp.utils.formatBytes(guest.diskCurrent)} / ${PulseApp.utils.formatBytes(guest.diskTotal)} (${diskPercent}%)` : `${diskPercent}%`;
            const diskColorClass = PulseApp.utils.getUsageColor(diskPercent, 'disk');
            const progressBar = PulseApp.utils.createProgressTextBarHTML(diskPercent, diskTooltipText, diskColorClass, `${diskPercent}%`);
            
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

    function createGuestRow(guest) {
        const row = document.createElement('tr');
        row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        row.setAttribute('data-name', guest.name.toLowerCase());
        row.setAttribute('data-type', guest.type.toLowerCase());
        row.setAttribute('data-node', guest.node.toLowerCase());
        row.setAttribute('data-id', guest.id);

        // Check if guest has custom thresholds
        const thresholdIndicator = createThresholdIndicator(guest);

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
        
        if (guest.status === STATUS_RUNNING && PulseApp.charts) {
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

        row.innerHTML = `
            <td class="sticky left-0 bg-white dark:bg-gray-800 z-10 p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 border-r border-gray-300 dark:border-gray-600" title="${guest.name}">
                <div class="flex items-center gap-1">
                    <span>${guest.name}</span>
                    ${thresholdIndicator}
                </div>
            </td>
            <td class="p-1 px-2">${typeIcon}</td>
            <td class="p-1 px-2">${guest.id}</td>
            <td class="p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis">${uptimeDisplay}</td>
            <td class="p-1 px-2">${cpuBarHTML}</td>
            <td class="p-1 px-2">${memoryBarHTML}</td>
            <td class="p-1 px-2">${diskBarHTML}</td>
            <td class="p-1 px-2">${diskReadCell}</td>
            <td class="p-1 px-2">${diskWriteCell}</td>
            <td class="p-1 px-2">${netInCell}</td>
            <td class="p-1 px-2">${netOutCell}</td>
        `;
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

    return {
        init,
        refreshDashboardData,
        updateDashboardTable,
        createGuestRow,
        snapshotGuestMetricsForDrag, // Export snapshot function
        clearGuestMetricSnapshots,    // Export clear function
        toggleChartsMode
    };
})();
