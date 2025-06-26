PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let backupsSearchInput = null;
    let resetBackupsButton = null;
    let backupsTabContent = null;
    let namespaceFilter = null;
    let pbsInstanceFilter = null;
    let lastUserUpdateTime = 0; // Track when user last triggered an update
    let isProcessingDateSelection = false; // Prevent re-entrancy during date selection
    let guestToValidatedSnapshots = null; // Store validated snapshots mapping
    
    // Enhanced cache for expensive data transformations
    let dataCache = {
        lastStateHash: null,
        processedBackupData: null,
        guestBackupStatus: null,
        // Fine-grained caching
        guestCache: new Map(), // Maps guestId to cached data with TTL
        tasksByGuestCache: new Map(),
        snapshotsByGuestCache: new Map(),
        lastCleanup: Date.now(),
        cacheStats: { hits: 0, misses: 0 }
    };
    
    // Cache TTL in milliseconds
    const CACHE_TTL = 30000; // 30 seconds for guest data
    const CACHE_CLEANUP_INTERVAL = 300000; // 5 minutes
    
    // DOM element cache to avoid repeated queries
    const domCache = {
        tableBody: null,
        noDataMsg: null,
        tableContainer: null,
        scrollableContainer: null,
        visualizationSection: null,
        calendarContainer: null,
        statusTextElement: null,
        pbsSummaryElement: null,
        detailCardContainer: null,
        loadingIndicator: null
    };
    
    // Row tracking for incremental updates
    const rowTracker = new Map(); // Maps guestId to row element
    
    // API backup data cache
    let apiBackupData = null;
    let apiDataTimestamp = 0;
    const API_CACHE_TTL = 5000; // 5 seconds
    
    // Fetch backup data from the new API
    async function fetchBackupDataFromAPI() {
        try {
            // Check cache
            if (apiBackupData && (Date.now() - apiDataTimestamp < API_CACHE_TTL)) {
                return apiBackupData;
            }
            
            // Get current filters
            const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
            const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType') || 'all';
            
            // Build query parameters
            const params = new URLSearchParams();
            if (namespaceFilter !== 'all') params.append('namespace', namespaceFilter);
            if (backupTypeFilter !== 'all') params.append('backupType', backupTypeFilter);
            
            const response = await fetch(`/api/backup-data?${params}`);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const result = await response.json();
            if (result.success && result.data) {
                apiBackupData = result.data;
                apiDataTimestamp = Date.now();
                return result.data;
            }
            
            throw new Error('Invalid API response');
        } catch (error) {
            console.error('Error fetching backup data from API:', error);
            return null;
        }
    }
    
    // Render backups table from API data
    function _renderBackupsFromAPIData(apiData, tableContainer, tableBody, noDataMsg, statusTextElement, loadingMsg, scrollableContainer, currentScrollLeft, currentScrollTop) {
        const backupStatusByGuest = apiData.backupStatusByGuest || [];
        
        // Hide loading message and show table
        loadingMsg.classList.add('hidden');
        
        if (backupStatusByGuest.length === 0) {
            tableContainer.classList.add('hidden');
            noDataMsg.textContent = "No backups found for any guests.";
            noDataMsg.classList.remove('hidden');
            _updateBackupStatusMessages(statusTextElement, 0, backupsSearchInput);
            return;
        }
        
        // Get search filter value
        const searchValue = backupsSearchInput?.value?.toLowerCase() || '';
        
        // Filter guests based on search
        let filteredGuests = backupStatusByGuest;
        if (searchValue) {
            const searchTerms = searchValue.split(',').map(t => t.trim()).filter(t => t);
            if (searchTerms.length > 0) {
                filteredGuests = backupStatusByGuest.filter(guest => {
                    return searchTerms.some(term => 
                        guest.guestName?.toLowerCase().includes(term) ||
                        guest.guestId?.toString().includes(term) ||
                        guest.node?.toLowerCase().includes(term)
                    );
                });
            }
        }
        
        // Show table
        tableContainer.classList.remove('hidden');
        noDataMsg.classList.add('hidden');
        
        // Create document fragment for better performance
        const fragment = document.createDocumentFragment();
        
        // Render each guest row
        filteredGuests.forEach(guestStatus => {
            const row = _renderBackupTableRow(guestStatus);
            fragment.appendChild(row);
        });
        
        // Clear existing rows and append new ones
        tableBody.innerHTML = '';
        tableBody.appendChild(fragment);
        
        // Update status messages
        _updateBackupStatusMessages(statusTextElement, filteredGuests.length, backupsSearchInput);
        
        // Restore scroll position
        scrollableContainer.scrollLeft = currentScrollLeft;
        scrollableContainer.scrollTop = currentScrollTop;
        
        // Update visualization section
        const visualizationSection = document.getElementById('backup-visualization-section');
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        const detailCardContainer = document.getElementById('backup-detail-card');
        
        if (visualizationSection && backupStatusByGuest.length > 0) {
            visualizationSection.classList.remove('hidden');
            
            
            
            // Hide node backup cards if present
            const nodeBackupCards = document.getElementById('node-backup-cards');
            if (nodeBackupCards) {
                nodeBackupCards.classList.add('hidden');
            }
            
            // Hide summary cards container
            const summaryCardsContainer = document.getElementById('backup-summary-cards-container');
            if (summaryCardsContainer) {
                summaryCardsContainer.classList.add('hidden');
            }
            
            // Create calendar if it exists
            if (calendarContainer && PulseApp.ui.calendarHeatmap) {
                // Get guests with backups for calendar
                const guestsWithBackups = backupStatusByGuest.filter(guest => guest.totalBackups > 0);
                const guestIds = guestsWithBackups.map(guest => {
                    const nodeIdentifier = guest.node || guest.endpointId || '';
                    return nodeIdentifier ? `${guest.guestId}-${nodeIdentifier}` : guest.guestId.toString();
                });
                
                // Create backup data structure for calendar
                const backupData = {
                    pbsSnapshots: apiData.pbsSnapshots || [],
                    pveBackups: [], // TODO: Add if needed
                    vmSnapshots: [], // TODO: Add if needed
                    backupTasks: apiData.backupTasks || [],
                    guestToValidatedSnapshots: new Map() // TODO: Build if needed
                };
                
                // Initialize detail card if needed
                if (detailCardContainer && PulseApp.ui.backupDetailCard) {
                    let detailCard = detailCardContainer.querySelector('.bg-white.dark\\:bg-gray-800');
                    if (!detailCard) {
                        detailCard = PulseApp.ui.backupDetailCard.createBackupDetailCard(null);
                        while (detailCardContainer.firstChild) {
                            detailCardContainer.removeChild(detailCardContainer.firstChild);
                        }
                        detailCardContainer.appendChild(detailCard);
                    }
                }
                
                // Create date selection callback for detail card
                const onDateSelect = (selectedDate, instantUpdate) => {
                    if (!detailCardContainer || !PulseApp.ui.backupDetailCard) return;
                    
                    let detailCard = detailCardContainer.querySelector('.bg-white.dark\\:bg-gray-800');
                    if (detailCard && selectedDate) {
                        // Update detail card with selected date data
                        // For now, just pass the raw data - the detail card component will handle filtering
                        const dateData = {
                            selectedDate,
                            guests: backupStatusByGuest,
                            backupData
                        };
                        PulseApp.ui.backupDetailCard.updateDetailCard(detailCard, dateData);
                    }
                };
                
                // Create/update calendar
                const calendarHeatmap = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(
                    backupData,
                    null, // selectedDate
                    guestIds, // filteredGuestIds
                    onDateSelect, // date selection callback
                    false // preserveSelection
                );
                
                // Append calendar to container
                calendarContainer.innerHTML = '';
                calendarContainer.appendChild(calendarHeatmap);
            }
        } else if (visualizationSection) {
            visualizationSection.classList.add('hidden');
        }
    }
    
    
    // Helper function to parse namespace filter value
    function _parseNamespaceFilter(namespaceFilter) {
        if (!namespaceFilter || namespaceFilter === 'all') {
            return { targetPbsIndex: null, targetNamespace: 'all' };
        }
        
        // Check if filter includes PBS instance prefix (format: "index:namespace")
        if (namespaceFilter.includes(':')) {
            const parts = namespaceFilter.split(':');
            return {
                targetPbsIndex: parseInt(parts[0], 10),
                targetNamespace: parts[1]
            };
        }
        
        // Simple namespace without PBS prefix
        return {
            targetPbsIndex: null,
            targetNamespace: namespaceFilter
        };
    }
    
    // Helper function to determine which PBS instances to use based on filters
    function _getPbsInstancesToUse(pbsDataArray, namespaceFilter, pbsInstanceFilterValue) {
        const { targetPbsIndex } = _parseNamespaceFilter(namespaceFilter);
        
        if (targetPbsIndex !== null) {
            // Namespace filter includes specific PBS instance (e.g., "0:namespace")
            return pbsDataArray.filter((_, index) => index === targetPbsIndex);
        } else if (pbsInstanceFilterValue !== 'all') {
            // Separate PBS instance filter is active
            return pbsDataArray.filter((_, index) => index.toString() === pbsInstanceFilterValue);
        } else {
            // No specific PBS filtering - use all instances
            return pbsDataArray;
        }
    }
    
    // Initialize DOM cache
    function _initDomCache() {
        domCache.tableBody = document.getElementById('backups-overview-tbody');
        domCache.noDataMsg = document.getElementById('backups-no-data-message');
        domCache.tableContainer = document.getElementById('backups-table-container');
        domCache.scrollableContainer = document.querySelector('#backups .overflow-x-auto');
        domCache.visualizationSection = document.getElementById('backup-visualization-section');
        domCache.calendarContainer = document.getElementById('backup-calendar-heatmap');
        domCache.statusTextElement = document.getElementById('backups-status-text');
        domCache.pbsSummaryElement = document.getElementById('pbs-instances-summary');
        domCache.detailCardContainer = document.getElementById('backup-detail-card');
        domCache.loadingIndicator = document.getElementById('backups-loading-message');
    }
    
    function _generateStateHash(vmsData, containersData, pbsDataArray, pveBackups, namespaceFilter, pbsInstanceFilter) {
        // Enhanced hash generation with data sampling
        const vmCount = vmsData.length;
        const ctCount = containersData.length;
        const pbsCount = pbsDataArray.length;
        const pveTaskCount = pveBackups?.backupTasks?.length || 0;
        const pveStorageCount = pveBackups?.storageBackups?.length || 0;
        
        // Sample data for better change detection
        const vmSample = vmsData.length > 0 ? `${vmsData[0]?.id}-${vmsData[vmsData.length-1]?.id}` : '';
        const ctSample = containersData.length > 0 ? `${containersData[0]?.id}-${containersData[containersData.length-1]?.id}` : '';
        
        // Include timestamps for better cache invalidation
        const latestBackupTime = Math.max(
            ...pbsDataArray.flatMap(pbs => 
                (pbs.datastores || []).flatMap(ds => 
                    (ds.snapshots || []).map(s => s.timestamp || 0)
                )
            ),
            0
        );
        
        return `${vmCount}-${ctCount}-${pbsCount}-${pveTaskCount}-${pveStorageCount}-${namespaceFilter || 'all'}-${pbsInstanceFilter || 'all'}-${vmSample}-${ctSample}-${latestBackupTime}`;
    }
    
    // Clean up expired cache entries
    function _cleanupCache() {
        const now = Date.now();
        if (now - dataCache.lastCleanup < CACHE_CLEANUP_INTERVAL) return;
        
        // Clean up guest cache
        for (const [guestId, data] of dataCache.guestCache) {
            if (now - data.timestamp > CACHE_TTL) {
                dataCache.guestCache.delete(guestId);
            }
        }
        
        // Clean up component caches
        for (const [key, data] of dataCache.tasksByGuestCache) {
            if (now - data.timestamp > CACHE_TTL) {
                dataCache.tasksByGuestCache.delete(key);
            }
        }
        
        for (const [key, data] of dataCache.snapshotsByGuestCache) {
            if (now - data.timestamp > CACHE_TTL) {
                dataCache.snapshotsByGuestCache.delete(key);
            }
        }
        
        dataCache.lastCleanup = now;
    }

    function _initTableFixedLine() {
        // No longer needed - using CSS border styling instead
    }

    function init() {
        // Initialize DOM cache first
        _initDomCache();
        
        backupsSearchInput = document.getElementById('backups-search');
        resetBackupsButton = document.getElementById('reset-backups-filters-button');
        backupsTabContent = document.getElementById('backups');

        if (backupsSearchInput) {
            const debouncedUpdate = PulseApp.utils.debounce(() => {
                // Clear API cache when search changes
                apiBackupData = null;
                apiDataTimestamp = 0;
                updateBackupsTab(true);
            }, 300);
            backupsSearchInput.addEventListener('input', debouncedUpdate);
        } else {
            console.warn('Element #backups-search not found - backups text filtering disabled.');
        }

        if (resetBackupsButton) {
            resetBackupsButton.addEventListener('click', resetBackupsView);
        }

        if (backupsTabContent) {
            backupsTabContent.addEventListener('keydown', (event) => {
                if (event.key === 'Escape' && backupsTabContent.contains(document.activeElement)) {
                    resetBackupsView();
                }
            });
        }
        
        // Add event listeners for filter changes
        const filterElements = [
            'backups-filter-type-all', 'backups-filter-type-vm', 'backups-filter-type-ct',
            'backups-filter-backup-all', 'backups-filter-backup-pbs', 'backups-filter-backup-pve', 'backups-filter-backup-snapshots',
            'backups-filter-status-all', 'backups-filter-status-ok', 'backups-filter-status-stale', 'backups-filter-status-warning', 'backups-filter-status-none',
            'backups-filter-failures'
        ];
        
        filterElements.forEach(id => {
            const element = document.getElementById(id);
            if (element) {
                element.addEventListener('change', () => {
                    // Update state based on which filter was changed
                    if (id.includes('filter-type-')) {
                        // Guest type filter
                        if (element.checked) {
                            const type = id.replace('backups-filter-type-', '');
                            PulseApp.state.set('backupsFilterGuestType', type);
                        }
                    } else if (id.includes('filter-backup-')) {
                        // Backup type filter
                        if (element.checked) {
                            const backupType = id.replace('backups-filter-backup-', '');
                            PulseApp.state.set('backupsFilterBackupType', backupType);
                            // Update PBS-specific filter visibility when backup type changes
                            _updateNamespaceOptions();
                            _updatePbsInstanceOptions();
                        }
                    } else if (id.includes('filter-status-')) {
                        // Health status filter
                        if (element.checked) {
                            const status = id.replace('backups-filter-status-', '');
                            PulseApp.state.set('backupsFilterHealth', status);
                        }
                    } else if (id === 'backups-filter-failures') {
                        // Failures checkbox
                        PulseApp.state.set('backupsFilterFailures', element.checked);
                    }
                    
                    updateBackupsTab(true);
                });
            }
        });
        
        // Global ESC key handler for backups tab
        document.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') {
                // Check if backups tab is currently active
                const backupsTab = document.querySelector('.tab[data-tab="backups"]');
                const isBackupsTabActive = backupsTab && backupsTab.classList.contains('active');
                
                if (isBackupsTabActive) {
                    // Check if there are any active filters to clear
                    const calendarFilter = PulseApp.state.get('calendarDateFilter');
                    const searchTerm = backupsSearchInput ? backupsSearchInput.value : '';
                    const typeFilter = PulseApp.state.get('backupsFilterGuestType');
                    const healthFilter = PulseApp.state.get('backupsFilterHealth');
                    const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType');
                    const failuresFilter = PulseApp.state.get('backupsFilterFailures');
                    
                    const hasActiveFilters = calendarFilter || 
                                           searchTerm || 
                                           (typeFilter && typeFilter !== 'all') || 
                                           (healthFilter && healthFilter !== 'all') ||
                                           (backupTypeFilter && backupTypeFilter !== 'all') ||
                                           failuresFilter;
                    
                    if (hasActiveFilters) {
                        event.preventDefault();
                        resetBackupsView();
                    }
                }
            }
        });
        
        // Initialize mobile scroll indicators
        if (window.innerWidth < 768) {
            PulseApp.utils.initMobileScrollIndicators('#backups');
        }
        
        // Initialize snapshot modal handlers
        _initSnapshotModal();
        
        // Initialize namespace filter
        _initNamespaceFilter();
        
        // Initialize PBS instance filter
        _initPbsInstanceFilter();
    }

    function calculateBackupSummary(backupStatusByGuest) {
        let totalGuests = backupStatusByGuest.length;
        let healthyCount = 0;
        let warningCount = 0;
        let errorCount = 0;
        let noneCount = 0;
        let totalPbsBackups = 0;
        let totalPveBackups = 0;
        let totalSnapshots = 0;

        backupStatusByGuest.forEach(guest => {
            
            switch (guest.backupHealthStatus) {
                case 'ok':
                case 'stale':
                    healthyCount++;
                    break;
                case 'old':
                    warningCount++;
                    break;
                case 'failed':
                    errorCount++;
                    break;
                case 'none':
                    noneCount++;
                    break;
            }
            
            totalPbsBackups += guest.pbsBackups || 0;
            totalPveBackups += guest.pveBackups || 0;
            totalSnapshots += guest.snapshotCount || 0;
            
        });

        return {
            totalGuests,
            healthyCount,
            warningCount,
            errorCount,
            noneCount,
            totalPbsBackups,
            totalPveBackups,
            totalSnapshots,
            healthyPercent: totalGuests > 0 ? (healthyCount / totalGuests) * 100 : 0
        };
    }

    function createNodeBackupSummaryCard(nodeName, guestStatuses) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1';
        
        let healthyCount = 0;
        let warningCount = 0;
        let errorCount = 0;
        let noneCount = 0;
        let pbsTotal = 0;
        let pveTotal = 0;
        let snapshotTotal = 0;
        
        guestStatuses.forEach(guest => {
            switch (guest.backupHealthStatus) {
                case 'ok':
                case 'stale':
                    healthyCount++;
                    break;
                case 'old':
                    warningCount++;
                    break;
                case 'failed':
                    errorCount++;
                    break;
                case 'none':
                    noneCount++;
                    break;
            }
            pbsTotal += guest.pbsBackups || 0;
            pveTotal += guest.pveBackups || 0;
            snapshotTotal += guest.snapshotCount || 0;
        });
        
        const totalGuests = guestStatuses.length;
        const healthyPercent = totalGuests > 0 ? (healthyCount / totalGuests) * 100 : 0;
        
        // Sort guests by backup health (worst first for visibility)
        const sortedGuests = [...guestStatuses].sort((a, b) => {
            const priority = { 'failed': 0, 'none': 1, 'old': 2, 'stale': 3, 'ok': 4 };
            return priority[a.backupHealthStatus] - priority[b.backupHealthStatus];
        }); // Show all guests
        
        card.innerHTML = `
            <div class="flex justify-between items-center">
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate">${nodeName}</h3>
                <span class="text-xs text-gray-500 dark:text-gray-400">${totalGuests} guest${totalGuests > 1 ? 's' : ''}</span>
            </div>
            <div class="flex items-center gap-2 text-[10px] text-gray-600 dark:text-gray-400">
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-yellow-500 rounded-sm"></div>
                    <span>${snapshotTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-orange-500 rounded-sm"></div>
                    <span>${pveTotal}</span>
                </div>
                <div class="flex items-center gap-1">
                    <div class="w-2 h-2 bg-purple-500 rounded-sm"></div>
                    <span>${pbsTotal}</span>
                </div>
            </div>
            ${sortedGuests.map(guest => {
                const statusColor = PulseApp.utils.getBackupStatusColor(guest.backupHealthStatus);
                
                const statusIcon = {
                    'ok': '●',
                    'stale': '●',
                    'old': '●',
                    'failed': '●',
                    'none': '○'
                }[guest.backupHealthStatus] || '○';
                
                return `
                    <div class="text-[10px] text-gray-600 dark:text-gray-400 flex items-center gap-1">
                        <span class="${statusColor}">${statusIcon}</span>
                        <span class="truncate flex-1">${guest.guestName}</span>
                        <span class="text-[9px]">${guest.guestId}</span>
                    </div>
                `;
            }).join('')}
        `;
        
        return card;
    }

    function _extractBackupTypeFromVolid(volid, vmid) {
        // Extract type from volid format: vzdump-{type}-{vmid}-{timestamp}
        const volidMatch = volid.match(/vzdump-(qemu|lxc)-(\d+)-/);
        if (volidMatch) {
            return volidMatch[1] === 'qemu' ? 'vm' : 'ct';
        }
        
        // Fallback: try to determine from guest data if available
        // Look up the guest in current data to determine actual type
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        
        const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(vmid, 10));
        if (guest) {
            return guest.type === 'qemu' ? 'vm' : 'ct';
        }
        
        // Final fallback: assume VM if no match found
        console.warn('[Backups] Could not determine backup type from volid:', volid, 'vmid:', vmid);
        return 'vm';
    }

    // REMOVED: _getInitialBackupData function (379 lines)

    function _determineGuestBackupStatus(guest, guestSnapshots, guestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo) {
        const guestId = String(guest.vmid);
        
        // Get guest snapshots from pveBackups - use node-aware filtering
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const allSnapshots = pveBackups.guestSnapshots || [];
        const guestSnapshotCount = allSnapshots
            .filter(snap => {
                // Match vmid
                if (parseInt(snap.vmid, 10) !== parseInt(guest.vmid, 10)) return false;
                
                // For VM/CT snapshots, match by node/endpoint if available
                if (guest.node && snap.node) {
                    return snap.node === guest.node;
                }
                if (guest.endpointId && snap.endpointId) {
                    return snap.endpointId === guest.endpointId;
                }
                
                // Fallback: include if no node info available
                return true;
            })
            .length;
        
        // Use pre-filtered data instead of filtering large arrays
        const totalBackups = guestSnapshots ? guestSnapshots.length : 0;
        const latestSnapshot = guestSnapshots && guestSnapshots.length > 0 
            ? guestSnapshots.reduce((latest, snap) => {
                return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
            }, null)
            : null;
        const latestSnapshotTime = latestSnapshot ? latestSnapshot['backup-time'] : null;

        const latestTask = guestTasks && guestTasks.length > 0
            ? guestTasks.reduce((latest, task) => {
               return (!latest || (task.startTime && task.startTime > latest.startTime)) ? task : latest;
            }, null)
            : null;

        let healthStatus = 'none';
        let displayTimestamp = null;

        // Only use actual backup snapshots for timestamp, not just tasks
        // Tasks might be failed attempts or other operations
        if (latestSnapshotTime) {
            displayTimestamp = latestSnapshotTime;
            if (latestSnapshotTime >= threeDaysAgo) healthStatus = 'ok';
            else if (latestSnapshotTime >= sevenDaysAgo) healthStatus = 'stale';
            else healthStatus = 'old';
        } else if (latestTask && latestTask.status === 'OK') {
            // Only use task timestamp if there are no snapshots AND the task succeeded
            displayTimestamp = latestTask.startTime;
            if (latestTask.startTime >= threeDaysAgo) healthStatus = 'ok';
            else if (latestTask.startTime >= sevenDaysAgo) healthStatus = 'stale';
            else healthStatus = 'old';
        } else if (latestTask && latestTask.status !== 'OK') {
            // Failed task with no snapshots
            healthStatus = 'failed';
            displayTimestamp = null; // No successful backup timestamp
        } else {
            // No snapshots and no tasks
            healthStatus = 'none';
            displayTimestamp = null;
        }

        // Calculate recent failures by analyzing actual PBS and PVE backup tasks
        let recentFailures = 0;
        let lastFailureTime = null;
        
        // Get recent failure window (7 days)
        const now = Math.floor(Date.now() / 1000);
        const sevenDaysAgoForFailures = now - (7 * 24 * 60 * 60);

        if (guestTasks && guestTasks.length > 0) {
            // Analyze actual backup tasks for failures in the last 7 days
            const recentFailedTasks = guestTasks.filter(task => {
                // Include tasks from the last 7 days that failed
                return task.startTime >= sevenDaysAgoForFailures && 
                       task.status !== 'OK' && 
                       task.status !== null && 
                       task.status !== undefined;
            });
            
            recentFailures = recentFailedTasks.length;

            // Find the most recent failure timestamp
            if (recentFailedTasks.length > 0) {
                const latestFailedTask = recentFailedTasks.reduce((latest, task) => {
                    return (task.startTime > latest.startTime) ? task : latest;
                });
                lastFailureTime = latestFailedTask.startTime;
            }
        }
        
        // If no task-based failures found but health status indicates failure, count as 1
        if (recentFailures === 0 && healthStatus === 'failed') {
            recentFailures = 1;
            lastFailureTime = displayTimestamp;
            
        }

        // Enhanced 7-day backup status calculation with backup type tracking
        const last7DaysBackupStatus = dayBoundaries.map((day, index) => {
            let backupTypes = new Set();
            let hasFailures = false;
            let activityDetails = [];

            // Check tasks for this day - using pre-filtered guest tasks
            if (guestTasks) {
                const failedTasksOnThisDay = guestTasks.filter(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status !== 'OK'
                );
                const successfulTasksOnThisDay = guestTasks.filter(task => 
                    task.startTime >= day.start && task.startTime < day.end && task.status === 'OK'
                );

                // Track successful backup types
                successfulTasksOnThisDay.forEach(task => {
                    const source = task.source === 'pbs' ? 'PBS' : 'PVE';
                    const location = task.source === 'pbs' ? task.pbsInstanceName : 'Local';
                    backupTypes.add(task.source);
                    activityDetails.push(`✓ ${source} backup${location ? ` (${location})` : ''}`);
                });

                // Track failed backup attempts
                failedTasksOnThisDay.forEach(task => {
                    const source = task.source === 'pbs' ? 'PBS' : 'PVE';
                    const location = task.source === 'pbs' ? task.pbsInstanceName : 'Local';
                    hasFailures = true;
                    activityDetails.push(`✗ ${source} backup failed${location ? ` (${location})` : ''}`);
                });
            }

            // Check for backup storage activity (snapshots/backups created)
            if (guestSnapshots) {
                const snapshotsOnThisDay = guestSnapshots.filter(
                    snap => snap['backup-time'] >= day.start && snap['backup-time'] < day.end
                );
                
                snapshotsOnThisDay.forEach(snap => {
                    if (snap.source === 'pbs') {
                        backupTypes.add('pbs');
                        activityDetails.push(`✓ PBS backup stored (${snap.pbsInstanceName})`);
                    } else if (snap.source === 'pve') {
                        backupTypes.add('pve');
                        activityDetails.push(`✓ PVE backup stored (${snap.storage || 'Local'})`);
                    }
                });
            }

            // Check for VM/CT snapshots on this day (if we have that data)
            const pveBackups = PulseApp.state.get('pveBackups') || {};
            const allSnapshots = pveBackups.guestSnapshots || [];
            const guestDaySnapshots = allSnapshots.filter(snap => {
                // Match vmid and time
                if (parseInt(snap.vmid, 10) !== parseInt(guestId, 10)) return false;
                if (!(snap.snaptime >= day.start && snap.snaptime < day.end)) return false;
                
                // Match by node/endpoint if available
                if (guest.node && snap.node) {
                    return snap.node === guest.node;
                }
                if (guest.endpointId && snap.endpointId) {
                    return snap.endpointId === guest.endpointId;
                }
                
                // Fallback: include if no node info available
                return true;
            });
            
            if (guestDaySnapshots.length > 0) {
                backupTypes.add('snapshot');
                activityDetails.push(`✓ ${guestDaySnapshots.length} VM/CT snapshot${guestDaySnapshots.length > 1 ? 's' : ''} created`);
            }

            // Create day label for tooltip
            const dayDate = new Date(day.start * 1000);
            const dayLabel = dayDate.toLocaleDateString('en-US', { 
                weekday: 'short', 
                month: 'short', 
                day: 'numeric' 
            });

            return {
                backupTypes: Array.from(backupTypes),
                hasFailures: hasFailures,
                details: activityDetails.length > 0 ? activityDetails.join('\n') : 'No backup activity',
                date: dayLabel
            };
        });

        // Calculate separate PBS and PVE backup information
        let pbsBackupCount = 0;
        let pbsBackupInfo = '';
        let pveBackupCount = 0; 
        let pveBackupInfo = '';
        let pbsBackupAmbiguous = false; // Track if PBS backups can't be reliably attributed
        
        if (guestSnapshots && guestSnapshots.length > 0) {
            // Separate PBS and PVE snapshots
            const pbsSnapshots = guestSnapshots.filter(s => s.source === 'pbs');
            const pveSnapshots = guestSnapshots.filter(s => s.source === 'pve');
            
            // Calculate PBS backup information
            if (pbsSnapshots.length > 0) {
                pbsBackupCount = pbsSnapshots.length;
                const pbsInstances = [...new Set(pbsSnapshots.map(s => s.pbsInstanceName).filter(Boolean))];
                const datastores = [...new Set(pbsSnapshots.map(s => s.datastoreName).filter(Boolean))];
                
                // Since we're now properly filtering by owner/endpoint, we only need to check
                // if any snapshots have missing owner information
                const ambiguousSnapshots = pbsSnapshots.filter(snap => {
                    const owner = snap.owner || '';
                    return !owner || !owner.includes('!');
                });
                
                // Mark as ambiguous only if we have snapshots without owner info
                if (ambiguousSnapshots.length > 0) {
                    pbsBackupAmbiguous = true;
                }
                
                // Group backups by PBS instance and namespace for detailed info
                const backupsByPbs = {};
                pbsSnapshots.forEach(snap => {
                    if (snap.pbsInstanceName) {
                        if (!backupsByPbs[snap.pbsInstanceName]) {
                            backupsByPbs[snap.pbsInstanceName] = { count: 0, datastores: new Set(), namespaces: new Set() };
                        }
                        backupsByPbs[snap.pbsInstanceName].count++;
                        if (snap.datastoreName) {
                            backupsByPbs[snap.pbsInstanceName].datastores.add(snap.datastoreName);
                        }
                        if (snap.namespace) {
                            backupsByPbs[snap.pbsInstanceName].namespaces.add(snap.namespace);
                        }
                    }
                });
                
                if (pbsInstances.length === 1) {
                    const info = backupsByPbs[pbsInstances[0]];
                    const nsArray = Array.from(info.namespaces || []);
                    
                    // Create namespace breakdown for display
                    const namespaceBreakdown = {};
                    pbsSnapshots.forEach(snap => {
                        const ns = snap.namespace || 'root';
                        namespaceBreakdown[ns] = (namespaceBreakdown[ns] || 0) + 1;
                    });
                    
                    // Format namespace info with counts
                    let nsInfo = '';
                    const actualNamespaces = Object.keys(namespaceBreakdown);
                    if (actualNamespaces.length > 1) {
                        // This guest actually has backups in multiple namespaces
                        const nsDetails = Object.entries(namespaceBreakdown)
                            .map(([ns, count]) => `${ns === 'root' ? 'root' : ns}:${count}`)
                            .join(', ');
                        nsInfo = ` (${nsDetails})`;
                    } else if (actualNamespaces.length === 1 && actualNamespaces[0] !== 'root') {
                        // Single non-root namespace
                        nsInfo = ` (${actualNamespaces[0]})`;
                    }
                    
                    pbsBackupInfo = `${pbsInstances[0]} (${datastores.join(', ')})${nsInfo}`;
                } else if (pbsInstances.length > 1) {
                    const details = pbsInstances.map(pbs => {
                        const info = backupsByPbs[pbs];
                        const dsArray = Array.from(info.datastores);
                        const nsArray = Array.from(info.namespaces || []);
                        const nsInfo = nsArray.length > 0 ? ` [${nsArray.map(ns => ns === 'root' ? 'root' : ns).join(',')}]` : '';
                        return `${pbs}: ${info.count} on ${dsArray.join(', ')}${nsInfo}`;
                    }).join(' | ');
                    pbsBackupInfo = details;
                }
            }
            
            // Calculate PVE backup information
            if (pveSnapshots.length > 0) {
                pveBackupCount = pveSnapshots.length;
                const storages = [...new Set(pveSnapshots.map(s => s.storage).filter(Boolean))];
                
                if (storages.length === 1) {
                    pveBackupInfo = storages[0];
                } else if (storages.length > 1) {
                    pveBackupInfo = storages.join(', ');
                } else {
                    pveBackupInfo = 'Local storage';
                }
            }
        }
        
        // Include task data for counts if no snapshots but tasks exist
        if (guestTasks && guestTasks.length > 0) {
            const pbsTasks = guestTasks.filter(t => t.source === 'pbs');
            const pveTasks = guestTasks.filter(t => t.source === 'pve');
            
            if (pbsBackupCount === 0 && pbsTasks.length > 0) {
                // No PBS snapshots but have PBS tasks - show as recent activity
                const pbsInstances = [...new Set(pbsTasks.map(t => t.pbsInstanceName).filter(Boolean))];
                if (pbsInstances.length > 0) {
                    pbsBackupInfo = `Recent activity on ${pbsInstances.join(', ')}`;
                }
            }
            
            if (pveBackupCount === 0 && pveTasks.length > 0) {
                // No PVE backups but have PVE tasks - show as recent activity
                pveBackupInfo = 'Recent activity';
            }
        }

        // Calculate type-specific latest backup times
        const latestTimes = {};
        
        if (guestSnapshots && guestSnapshots.length > 0) {
            // Separate snapshots by type
            const pbsSnapshots = guestSnapshots.filter(snap => snap.source === 'pbs' || snap.pbsInstance);
            const pveSnapshots = guestSnapshots.filter(snap => snap.source === 'pve' || (!snap.source && !snap.pbsInstance));
            
            // Calculate latest PBS backup time
            if (pbsSnapshots.length > 0) {
                const latestPbs = pbsSnapshots.reduce((latest, snap) => {
                    return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
                }, null);
                latestTimes.pbs = latestPbs ? latestPbs['backup-time'] : null;
            }
            
            // Calculate latest PVE backup time  
            if (pveSnapshots.length > 0) {
                const latestPve = pveSnapshots.reduce((latest, snap) => {
                    return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
                }, null);
                latestTimes.pve = latestPve ? latestPve['backup-time'] : null;
            }
        }
        
        // VM/Container snapshots are tracked separately
        if (guestSnapshotCount > 0) {
            // Get latest snapshot time from VM/CT snapshots
            const vmSnapshots = allSnapshots.filter(snap => {
                if (parseInt(snap.vmid, 10) !== parseInt(guest.vmid, 10)) return false;
                if (guest.node && snap.node) return snap.node === guest.node;
                if (guest.endpointId && snap.endpointId) return snap.endpointId === guest.endpointId;
                return true;
            });
            
            if (vmSnapshots.length > 0) {
                const latestVmSnapshot = vmSnapshots.reduce((latest, snap) => {
                    const snapTime = snap.snaptime;
                    const latestTime = latest ? latest.snaptime : 0;
                    return (snapTime && snapTime > latestTime) ? snap : latest;
                }, null);
                latestTimes.snapshots = latestVmSnapshot ? latestVmSnapshot.snaptime : null;
            }
        }
        
        // Determine the most recent backup type based on latest times
        let mostRecentBackupType = null;
        let mostRecentTime = 0;
        
        if (latestTimes.pbs && latestTimes.pbs > mostRecentTime) {
            mostRecentTime = latestTimes.pbs;
            mostRecentBackupType = 'pbs';
        }
        if (latestTimes.pve && latestTimes.pve > mostRecentTime) {
            mostRecentTime = latestTimes.pve;
            mostRecentBackupType = 'pve';
        }
        if (latestTimes.snapshots && latestTimes.snapshots > mostRecentTime) {
            mostRecentTime = latestTimes.snapshots;
            mostRecentBackupType = 'snapshot';
        }
        
        return {
            guestName: guest.name || `Guest ${guest.vmid}`,
            guestId: guest.vmid,
            guestType: guest.type === 'qemu' ? 'VM' : 'LXC',
            node: guest.node,
            guestPveStatus: guest.status,
            latestBackupTime: displayTimestamp,
            latestTimes: latestTimes, // NEW: Type-specific latest times
            mostRecentBackupType: mostRecentBackupType, // NEW: Most recent backup type
            pbsBackups: pbsBackupCount,
            pbsBackupInfo: pbsBackupInfo,
            pbsBackupAmbiguous: pbsBackupAmbiguous, // NEW: Track if PBS backups are ambiguous
            pveBackups: pveBackupCount,
            pveBackupInfo: pveBackupInfo,
            totalBackups: totalBackups,
            backupHealthStatus: healthStatus,
            last7DaysBackupStatus: last7DaysBackupStatus,
            snapshotCount: guestSnapshotCount,
            recentFailures: recentFailures,
            lastFailureTime: lastFailureTime,
            endpointId: guest.endpointId
        };
    }

    function _filterBackupData(backupStatusByGuest, backupsSearchInput) {
        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value.toLowerCase() : '';
        const backupsSearchTerms = currentBackupsSearchTerm.split(',').map(term => term.trim()).filter(term => term);
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');
        const backupsFilterBackupType = PulseApp.state.get('backupsFilterBackupType');
        const calendarDateFilter = PulseApp.state.get('calendarDateFilter');
        const showFailuresOnly = PulseApp.state.get('backupsFilterFailures') || false;

        // Debug logging for failures filter
        if (showFailuresOnly) {
            const guestsWithFailures = backupStatusByGuest.filter(item => item.recentFailures > 0);
        }

        return backupStatusByGuest.filter(item => {
            // Failures filter - show only guests with recent failures
            if (showFailuresOnly && item.recentFailures === 0) {
                return false;
            }

            const healthMatch = (backupsFilterHealth === 'all') ||
                                (backupsFilterHealth === 'ok' && item.backupHealthStatus === 'ok') ||
                                (backupsFilterHealth === 'stale' && item.backupHealthStatus === 'stale') ||
                                (backupsFilterHealth === 'warning' && (item.backupHealthStatus === 'old' || item.backupHealthStatus === 'failed')) ||
                                (backupsFilterHealth === 'none' && item.backupHealthStatus === 'none');
            if (!healthMatch) return false;

            const typeMatch = (backupsFilterGuestType === 'all') ||
                              (backupsFilterGuestType === 'vm' && item.guestType === 'VM') ||
                              (backupsFilterGuestType === 'lxc' && item.guestType === 'LXC');
            if (!typeMatch) return false;

            // Backup type filter - only show guests that have the specified backup type
            if (backupsFilterBackupType && backupsFilterBackupType !== 'all') {
                let hasBackupType = false;
                if (backupsFilterBackupType === 'pbs' && item.pbsBackups > 0) {
                    hasBackupType = true;
                } else if (backupsFilterBackupType === 'pve' && item.pveBackups > 0) {
                    hasBackupType = true;
                } else if (backupsFilterBackupType === 'snapshots' && item.snapshotCount > 0) {
                    hasBackupType = true;
                }
                if (!hasBackupType) return false;
            }

            // Calendar date selection should NOT filter the table
            // It only affects the detail card/filtered summary

            if (backupsSearchTerms.length > 0) {
                return backupsSearchTerms.some(term =>
                    (item.guestName?.toLowerCase() || '').includes(term) ||
                    (item.node?.toLowerCase() || '').includes(term) ||
                    (item.guestId?.toString() || '').includes(term)
                );
            }
            return true;
        });
    }

    function createThresholdIndicator(guestStatus) {
        // Get current app state to check for custom thresholds
        const currentState = PulseApp.state.get();
        if (!currentState || !currentState.customThresholds) {
            return ''; // No custom thresholds data available
        }
        
        // Check if this guest has custom thresholds configured
        const hasCustomThresholds = currentState.customThresholds.some(config => 
            config.endpointId === guestStatus.endpointId && 
            config.nodeId === guestStatus.node && 
            config.vmid === guestStatus.guestId &&
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

    function _renderBackupTableRow(guestStatus) {
        const row = PulseApp.ui.common.createTableRow();
        row.dataset.guestId = guestStatus.guestId;
        row.id = `backup-row-${guestStatus.guestId}`; // Add ID for row tracking

        // Check if guest has custom thresholds
        const thresholdIndicator = createThresholdIndicator(guestStatus);

        const latestBackupFormatted = guestStatus.latestBackupTime
            ? PulseApp.utils.formatPbsTimestampRelative(guestStatus.latestBackupTime)
            : '<span class="text-gray-400">No backups found</span>';

        const typeIconClass = guestStatus.guestType === 'VM'
            ? 'vm-icon bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 font-medium'
            : 'ct-icon bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 font-medium';
        const typeIcon = `<span class="type-icon inline-block rounded text-xs align-middle ${typeIconClass}">${guestStatus.guestType}</span>`;

        // Create PBS backup cell with visual indicator
        let pbsBackupCell = '';
        if (guestStatus.pbsBackups > 0) {
            const pbsIcon = '<span class="inline-block w-2 h-2 bg-purple-500 rounded-full mr-1" title="PBS Backup"></span>';
            const ambiguityIcon = guestStatus.pbsBackupAmbiguous 
                ? '<span class="inline-block ml-1 text-yellow-500 dark:text-yellow-400" title="⚠️ These PBS backups may belong to another guest with the same VMID">⚠️</span>' 
                : '';
            const pbsText = `${pbsIcon}${guestStatus.pbsBackups}${ambiguityIcon}`;
            const tooltip = guestStatus.pbsBackupInfo || '';
            const fullTooltip = guestStatus.pbsBackupAmbiguous 
                ? `${tooltip} (Warning: Backups may belong to another guest with VMID ${guestStatus.guestId})`
                : tooltip;
            pbsBackupCell = `<span class="text-purple-700 dark:text-purple-300" ${fullTooltip ? `title="${fullTooltip}"` : ''}>${pbsText}</span>`;
        } else {
            pbsBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create PVE backup cell with visual indicator  
        let pveBackupCell = '';
        if (guestStatus.pveBackups > 0) {
            const pveIcon = '<span class="inline-block w-2 h-2 bg-orange-500 rounded-full mr-1" title="PVE Backup"></span>';
            pveBackupCell = `<span class="text-orange-700 dark:text-orange-300" ${guestStatus.pveBackupInfo ? `title="${guestStatus.pveBackupInfo}"` : ''}>${pveIcon}${guestStatus.pveBackups}</span>`;
        } else {
            pveBackupCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create snapshot button or count display
        let snapshotCell = '';
        if (guestStatus.snapshotCount > 0) {
            const snapshotIcon = '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full mr-1" title="VM/CT Snapshots"></span>';
            snapshotCell = `<button class="text-yellow-600 dark:text-yellow-400 hover:underline view-snapshots-btn" 
                data-vmid="${guestStatus.guestId}" 
                data-node="${guestStatus.node}"
                data-endpoint="${guestStatus.endpointId}"
                data-type="${guestStatus.guestType.toLowerCase()}">${snapshotIcon}${guestStatus.snapshotCount}</button>`;
        } else {
            snapshotCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create failures cell
        let failuresCell = '';
        if (guestStatus.recentFailures > 0) {
            const failureTime = guestStatus.lastFailureTime 
                ? PulseApp.utils.formatPbsTimestamp(guestStatus.lastFailureTime)
                : 'Unknown';
            failuresCell = `<span class="text-red-600 dark:text-red-400 font-medium" title="Last failure: ${failureTime}">${guestStatus.recentFailures}</span>`;
        } else {
            failuresCell = '<span class="text-gray-400 dark:text-gray-500">0</span>';
        }

        // Create namespace cell
        const namespaceCell = guestStatus.pbsNamespaceText || '-';

        // Create sticky guest name column
        const guestNameContent = `
            <div class="flex items-center gap-1">
                <span>${guestStatus.guestName}</span>
                ${thresholdIndicator}
            </div>
        `;
        const stickyGuestCell = PulseApp.ui.common.createStickyColumn(guestNameContent, {
            title: guestStatus.guestName,
            additionalClasses: 'text-gray-900 dark:text-gray-100',
            padding: 'p-1 px-2'
        });
        row.appendChild(stickyGuestCell);
        
        // Create regular cells
        row.appendChild(PulseApp.ui.common.createTableCell(guestStatus.guestId, 'p-1 px-2 text-gray-500 dark:text-gray-400'));
        row.appendChild(PulseApp.ui.common.createTableCell(typeIcon, 'p-1 px-2'));
        row.appendChild(PulseApp.ui.common.createTableCell(guestStatus.node, 'p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400'));
        row.appendChild(PulseApp.ui.common.createTableCell(namespaceCell, 'p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400'));
        row.appendChild(PulseApp.ui.common.createTableCell(latestBackupFormatted, 'p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400'));
        row.appendChild(PulseApp.ui.common.createTableCell(snapshotCell, 'p-1 px-2 text-center'));
        row.appendChild(PulseApp.ui.common.createTableCell(pveBackupCell, 'p-1 px-2 text-center'));
        row.appendChild(PulseApp.ui.common.createTableCell(pbsBackupCell, 'p-1 px-2 text-center'));
        row.appendChild(PulseApp.ui.common.createTableCell(failuresCell, 'p-1 px-2 text-center'));
        return row;
    }

    function _updateBackupStatusMessages(statusTextElement, visibleCount, backupsSearchInput) {
        if (!statusTextElement) return;

        const currentBackupsSearchTerm = backupsSearchInput ? backupsSearchInput.value : '';
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');

        const statusBaseText = `Updated: ${new Date().toLocaleTimeString()}`;
        let statusFilterText = currentBackupsSearchTerm ? ` | Filter: "${currentBackupsSearchTerm}"` : '';
        const typeFilterLabel = backupsFilterGuestType !== 'all' ? backupsFilterGuestType.toUpperCase() : '';
        const healthFilterLabel = backupsFilterHealth !== 'all' ? backupsFilterHealth.charAt(0).toUpperCase() + backupsFilterHealth.slice(1) : '';
        const otherFilters = [typeFilterLabel, healthFilterLabel].filter(Boolean).join('/');
        if (otherFilters) {
            statusFilterText += ` | ${otherFilters}`;
        }
        const statusCountText = ` | Showing ${visibleCount} guests`;
        statusTextElement.textContent = statusBaseText + statusFilterText + statusCountText;
    }

    function _initTableCalendarClick() {
        const backupsTableBody = document.getElementById('backups-overview-tbody');
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        
        if (!backupsTableBody || !calendarContainer) return;
        
        // Get current filtered guest from state (persists across API updates)
        let currentFilteredGuest = PulseApp.state.get('currentFilteredGuest') || null;
        
        // Add click listeners to table rows
        const tableRows = backupsTableBody.querySelectorAll('tr[data-guest-id]');
        
        tableRows.forEach(row => {
            const guestId = row.dataset.guestId;
            
            // Add cursor pointer to indicate clickability
            row.style.cursor = 'pointer';
            
            // Restore visual indication if this row was previously selected
            if (currentFilteredGuest === guestId) {
                row.classList.add('bg-blue-50', 'dark:bg-blue-900/20');
                // Re-apply calendar filter on restore (not a user action)
                _filterCalendarToGuest(guestId, false);
            }
            
            row.addEventListener('click', () => {
                if (currentFilteredGuest === guestId) {
                    // Clicking the same row again resets the filter
                    _resetCalendarFilter();
                    currentFilteredGuest = null;
                    PulseApp.state.set('currentFilteredGuest', null);
                    // Remove visual indication
                    tableRows.forEach(r => r.classList.remove('bg-blue-50', 'dark:bg-blue-900/20'));
                } else {
                    // Filter to this guest
                    _filterCalendarToGuest(guestId);
                    currentFilteredGuest = guestId;
                    PulseApp.state.set('currentFilteredGuest', guestId);
                    // Add visual indication
                    tableRows.forEach(r => r.classList.remove('bg-blue-50', 'dark:bg-blue-900/20'));
                    row.classList.add('bg-blue-50', 'dark:bg-blue-900/20');
                }
            });
        });
        
        // If we had a filtered guest but the row no longer exists (e.g., due to filtering), clear the state
        if (currentFilteredGuest && !document.querySelector(`tr[data-guest-id="${currentFilteredGuest}"]`)) {
            PulseApp.state.set('currentFilteredGuest', null);
            _resetCalendarFilter();
        }
    }

    function _filterCalendarToGuest(guestId, isUserAction = true) {
        // Re-render the calendar with only this guest's data
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        if (!calendarContainer || !PulseApp.ui.calendarHeatmap) return;
        
        // Get the current backup data
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        // Apply filters to determine which PBS instances to use
        const pbsInstanceFilterValue = PulseApp.state.get('backupsFilterPbsInstance') || 'all';
        const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
        const pbsInstancesToUse = _getPbsInstancesToUse(pbsDataArray, namespaceFilter, pbsInstanceFilterValue);
        
        // Get PBS snapshots
        const pbsSnapshots = pbsInstancesToUse.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    source: 'pbs'
                }))
            )
        );
        
        // Get PVE storage backups
        const pveStorageBackups = [];
        if (pveBackups?.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            pveBackups.storageBackups.forEach(backup => {
                pveStorageBackups.push({
                    'backup-time': backup.ctime,
                    backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
                    backupVMID: backup.vmid,
                    vmid: backup.vmid, // Ensure vmid is preserved for filtering
                    size: backup.size,
                    protected: backup.protected,
                    storage: backup.storage,
                    volid: backup.volid,
                    format: backup.format,
                    node: backup.node,
                    endpointId: backup.endpointId,
                    source: 'pve'
                });
            });
        }
        
        // Get VM snapshots
        const vmSnapshots = (pveBackups.guestSnapshots || []).map(snap => ({
            ...snap,
            source: 'vmSnapshots'
        }));
        
        // Get backup tasks
        const pbsBackupTasks = [];
        pbsInstancesToUse.forEach(pbs => {
            if (pbs.backupTasks?.recentTasks && Array.isArray(pbs.backupTasks.recentTasks)) {
                pbs.backupTasks.recentTasks.forEach(task => {
                    pbsBackupTasks.push({
                        ...task,
                        pbsInstanceName: pbs.pbsInstanceName,
                        source: 'pbs'
                    });
                });
            }
        });
        
        const pveBackupTasks = [];
        if (Array.isArray(pveBackups?.backupTasks)) {
            pveBackups.backupTasks.forEach(task => {
                pveBackupTasks.push({
                    ...task,
                    source: 'pve'
                });
            });
        }
        
        const backupData = {
            pbsSnapshots: pbsSnapshots,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots,
            backupTasks: [...pbsBackupTasks, ...pveBackupTasks]
        };
        
        // Get detail card for callback
        const detailCardContainer = document.getElementById('backup-detail-card');
        let onDateSelect = null;
        
        if (detailCardContainer && PulseApp.ui.backupDetailCard) {
            // Find existing detail card or create callback
            const existingCard = detailCardContainer.querySelector('.bg-slate-800');
            if (existingCard) {
                onDateSelect = (dateData, instant = false) => {
                    PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, dateData, instant);
                };
                
                // Only auto-select today on initial filter, not on API updates
                // The calendar will handle the auto-selection
            }
        }
        
        // Reset filter when filtering to specific guest
        if (PulseApp.ui.calendarHeatmap.resetFilter) {
            PulseApp.ui.calendarHeatmap.resetFilter();
        }
        
        // Create filtered calendar for this specific guest
        const filteredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, guestId, [guestId], onDateSelect, isUserAction);
        // Replace children instead of using innerHTML to avoid flash
        while (calendarContainer.firstChild) {
            calendarContainer.removeChild(calendarContainer.firstChild);
        }
        calendarContainer.appendChild(filteredCalendar);
    }
    
    function _resetCalendarFilter() {
        // Re-render the calendar with all filtered guests (respecting table filters)
        const calendarContainer = document.getElementById('backup-calendar-heatmap');
        if (!calendarContainer || !PulseApp.ui.calendarHeatmap) return;
        
        // Get current filtered backup status to determine which guests to show
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        
        // Apply filters to determine which PBS instances to use
        const pbsInstanceFilterValue = PulseApp.state.get('backupsFilterPbsInstance') || 'all';
        const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
        const pbsInstancesToUse = _getPbsInstancesToUse(pbsDataArray, namespaceFilter, pbsInstanceFilterValue);
        
        const allGuests = [...vmsData, ...containersData];
        const { tasksByGuest, snapshotsByGuest, dayBoundaries, threeDaysAgo, sevenDaysAgo } = _getInitialBackupData();
        
        // Debug: log all VMID 102 guests before processing
        const vmid102Guests = allGuests.filter(g => g.vmid === 102 || g.vmid === '102');
        if (vmid102Guests.length > 0) {

        }
        
        const backupStatusByGuest = allGuests.map(guest => {
            // Try PBS (generic), PVE (endpoint-generic), and PVE (fully-specific) keys
            const baseKey = `${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`;
            const endpointKey = guest.endpointId ? `-${guest.endpointId}` : '';
            const nodeKey = guest.node ? `-${guest.node}` : '';
            const endpointGenericKey = `${baseKey}${endpointKey}`;
            const fullSpecificKey = `${baseKey}${endpointKey}${nodeKey}`;
            
            // Get snapshots from all keys and combine them
            // For PBS, we need to check all PBS instances AND namespaces to find backups for this guest
            // PBS backups use owner tokens to identify the source node, so we need to include endpoint suffix
            const pbsSnapshots = [];
            
            // Determine the endpoint suffixes to check for this guest
            const guestEndpointSuffixes = [];
            if (guest.endpointId && guest.endpointId !== 'primary') {
                // Secondary endpoint - only check specific endpoint
                guestEndpointSuffixes.push(`-${guest.endpointId}`);
            } else {
                // Primary endpoint - check both node-specific and generic primary keys
                if (guest.node) {
                    // Check node-specific key first (for node-specific backups)
                    guestEndpointSuffixes.push(`-primary-${guest.node.toLowerCase()}`);
                }
                // Always also check generic primary key (for cluster-wide backups)
                guestEndpointSuffixes.push('-primary');
            }

            pbsInstancesToUse.forEach(pbsInstance => {
                
                // Get namespaces to check based on filter
                const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
                const namespaces = new Set();
                
                // Debug the filter state
                
                if (namespaceFilter === 'all') {
                    // When no filter, check all namespaces from PBS instance
                    namespaces.add('root'); // Always include root
                    if (pbsInstance.datastores) {
                        pbsInstance.datastores.forEach(ds => {
                            if (ds.snapshots) {
                                ds.snapshots.forEach(snap => {
                                    if (snap.namespace) {
                                        namespaces.add(snap.namespace);
                                    }
                                });
                            }
                        });
                    }
                } else {
                    // When filter is active, only check the selected namespace
                    namespaces.add(namespaceFilter);
                }

                // Check namespace keys for this guest with all possible endpoint suffixes
                namespaces.forEach(namespace => {
                    // Try each possible endpoint suffix for this guest
                    guestEndpointSuffixes.forEach(endpointSuffix => {
                        const pbsKey = `${baseKey}-${pbsInstance.pbsInstanceName}-${namespace}${endpointSuffix}`;
                        let snapshots = snapshotsByGuest.get(pbsKey) || [];

                        // STRICT VALIDATION: Filter snapshots to ensure they truly belong to this guest
                        snapshots = snapshots.filter(snap => {
                        // 1. Verify VMID matches exactly
                        if (String(snap.backupVMID) !== String(guest.vmid)) {
                            
                            return false;
                        }
                        
                        // 2. Verify type matches (vm vs ct)
                        const expectedType = guest.type === 'qemu' ? 'vm' : 'ct';
                        if (snap.backupType !== expectedType) {
                            
                            return false;
                        }
                        
                        // 3. Verify owner/node matches if we have that information
                        if (snap.owner && snap.owner.includes('!')) {
                            const ownerNode = snap.owner.split('!')[1].toLowerCase();
                            
                            // For primary endpoints, check node name
                            if (guest.endpointId === 'primary' || !guest.endpointId) {
                                // Skip node validation if the owner token was already determined to be non-node-specific
                                // (i.e., the key was created with generic '-primary' suffix)
                                // This is handled by the key matching logic above
                                if (guest.node && ownerNode !== guest.node.toLowerCase() && 
                                    ownerNode !== 'primary' && ownerNode !== 'backup') {
                                    // Check if this ownerNode is a known node name in the cluster
                                    const isKnownNode = PulseApp.state.get('vms')?.some(vm => 
                                        vm.node && vm.node.toLowerCase() === ownerNode
                                    ) || PulseApp.state.get('containers')?.some(ct => 
                                        ct.node && ct.node.toLowerCase() === ownerNode
                                    );
                                    
                                    if (isKnownNode) {
                                        // This is a real node name, so enforce strict matching
                                        
                                        return false;
                                    }
                                    // Otherwise, it's likely a PBS server name and was already handled by key matching
                                }
                            } else {
                                // For secondary endpoints, verify it matches the endpoint
                                // This requires checking if the owner matches the expected endpoint pattern
                                // For now, we'll trust the endpoint suffix matching in the key
                            }
                        }
                        
                        // 4. Additional validation based on backup name pattern
                        // PBS backups often include guest name in the backup-id
                        if (snap['backup-id']) {
                            const backupId = snap['backup-id'];
                            // Check if backup-id contains wrong guest type indicator
                            if (guest.type === 'qemu' && backupId.includes('/ct/')) {
                                
                                return false;
                            }
                            if (guest.type === 'lxc' && backupId.includes('/vm/')) {
                                
                                return false;
                            }
                        }
                        
                        // 5. Enhanced comment validation for VMID collisions
                        if (snap.comment) {
                            const commentLower = snap.comment.toLowerCase();
                            const guestNameLower = guest.name.toLowerCase();
                            
                            // Special handling for known VMID collisions
                            if (guest.vmid === 102) {
                                // List of known container names that use VMID 102
                                const containerNames = ['docker', 'pi-', 'pulse', 'homepage', 'debian', 'pbs', 'jellyfin', 'pihole', 'influxdb', 'socat'];
                                const isContainer = containerNames.some(name => guestNameLower.includes(name));
                                const isWindows = guestNameLower.includes('windows') || guestNameLower.includes('win11');
                                
                                // Check if comment indicates wrong guest type
                                if (isWindows && containerNames.some(name => commentLower.includes(name))) {
                                    
                                    return false;
                                }
                                if (isContainer && (commentLower.includes('windows') || commentLower.includes('win11'))) {
                                    
                                    return false;
                                }
                                
                                // Also check against the actual guest name
                                if (!isWindows && !isContainer) {
                                    // For other guests with VMID 102, do strict name matching
                                    if (!commentLower.includes(guestNameLower) && !guestNameLower.includes(commentLower.split(' ')[0])) {
                                        
                                        return false;
                                    }
                                }
                            }
                        }
                        
                        return true;
                        });
                        
                        // Add validated snapshots to the PBS snapshots array
                        if (snapshots.length > 0) {
                            pbsSnapshots.push(...snapshots);
                        }
                        
                        // CRITICAL: Check if we're accidentally looking at wrong type
                        if (guest.vmid === 102 && namespace === 'root') {
                            // Check if there are any cross-type keys
                            const wrongTypeKey = guest.type === 'qemu' ? 
                                `102-ct-${pbsInstance.pbsInstanceName}-${namespace}${endpointSuffix}` :
                                `102-vm-${pbsInstance.pbsInstanceName}-${namespace}${endpointSuffix}`;
                            
                            const wrongTypeSnapshots = snapshotsByGuest.get(wrongTypeKey) || [];
                            if (wrongTypeSnapshots.length > 0) {
                                console.error(`[PBS CRITICAL] Found ${wrongTypeSnapshots.length} backups with wrong type for ${guest.name}:`, {
                                    correctType: guest.type === 'qemu' ? 'vm' : 'ct',
                                    wrongTypeKey,
                                    wrongTypeBackup: {
                                        owner: wrongTypeSnapshots[0].owner,
                                        comment: wrongTypeSnapshots[0].comment?.substring(0, 50)
                                    }
                                });
                            }
                        }
                    });
                    
                    // Also check for unknown endpoint (backups without owner info)
                    const unknownKey = `${baseKey}-${pbsInstance.pbsInstanceName}-${namespace}-unknown`;
                    const unknownSnapshots = snapshotsByGuest.get(unknownKey) || [];
                    
                    // Don't add unknown snapshots - they're too ambiguous and cause cross-contamination
                    // If a guest has no properly attributed backups, it should show as having none
                    // rather than showing potentially incorrect backups from other nodes
                    
                    // Log if we're skipping unknown snapshots
                    if (unknownSnapshots.length > 0 && pbsSnapshots.length === 0) {
                        
                    }
                });
            });
            
            // Try multiple key variations for PVE backup matching to handle edge cases
            let pveEndpointSnapshots = snapshotsByGuest.get(endpointGenericKey) || [];
            let pveSpecificSnapshots = snapshotsByGuest.get(fullSpecificKey) || [];
            
            // REMOVED DANGEROUS FALLBACK: This was causing cross-type contamination
            // DO NOT try alternative guest types - a VM should never see CT backups and vice versa
            // This was the root cause of Windows11 VM showing pi-docker container backups
            
            // Deduplicate PVE snapshots by volid to avoid counting the same backup multiple times
            const pveSnapshotsMap = new Map();
            [...pveEndpointSnapshots, ...pveSpecificSnapshots].forEach(snap => {
                if (snap.volid) {
                    pveSnapshotsMap.set(snap.volid, snap);
                }
            });
            const uniquePveSnapshots = Array.from(pveSnapshotsMap.values());
            
            // FINAL SAFETY CHECK: Validate all snapshots before using them
            const validatedPbsSnapshots = pbsSnapshots.filter(snap => {
                // Verify this snapshot truly belongs to this guest
                const snapVmid = String(snap.backupVMID || snap.vmid || snap['backup-id']);
                const expectedType = guest.type === 'qemu' ? 'vm' : 'ct';
                
                if (snapVmid !== String(guest.vmid)) {
                    if (guest.vmid === 102) {
                        
                    }
                    return false;
                }
                
                if (snap.backupType && snap.backupType !== expectedType) {
                    if (guest.vmid === 102) {
                        
                    }
                    return false;
                }
                
                // Additional owner check for PBS snapshots
                if (snap.owner && guest.node) {
                    const ownerParts = snap.owner.split('!');
                    if (ownerParts.length > 1) {
                        const ownerToken = ownerParts[1].toLowerCase();
                        
                    }
                }
                
                // Sanity check: backup time shouldn't be in the future
                const backupTime = snap['backup-time'];
                if (backupTime) {
                    const now = Math.floor(Date.now() / 1000);
                    const oneHourFuture = now + 3600; // Allow 1 hour for clock drift
                    if (backupTime > oneHourFuture) {
                        if (guest.vmid === 102) {
                            
                        }
                        return false;
                    }
                }
                
                return true;
            });
            
            // Log validation results for VMID 102
            if (guest.vmid === 102) {

                // Log what we're keeping
                if (validatedPbsSnapshots.length > 0) {
                    const latest = validatedPbsSnapshots.reduce((latest, snap) => {
                        return (!latest || (snap['backup-time'] && snap['backup-time'] > latest['backup-time'])) ? snap : latest;
                    }, null);

                }
            }
            
            const allGuestSnapshots = [...validatedPbsSnapshots, ...uniquePveSnapshots];
            
            // Debug log for root namespace
            if (namespaceFilter === 'root') {
                
            }
            
            // Similar for tasks
            const pbsTasks = tasksByGuest.get(baseKey) || [];
            const pveEndpointTasks = tasksByGuest.get(endpointGenericKey) || [];
            const pveSpecificTasks = tasksByGuest.get(fullSpecificKey) || [];
            const allGuestTasks = [...pbsTasks, ...pveEndpointTasks, ...pveSpecificTasks];
            
            const status = _determineGuestBackupStatus(guest, allGuestSnapshots, allGuestTasks, dayBoundaries, threeDaysAgo, sevenDaysAgo);
            
            // Debug log for root namespace
            if (namespaceFilter === 'root' && allGuestSnapshots.length > 0) {
                
            }
            
            return status;
        });
        
        const filteredBackupStatus = _filterBackupData(backupStatusByGuest, backupsSearchInput);

        // Get the current backup data
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        // Prepare backup data same as in updateBackupsTab
        const pbsSnapshots = pbsInstancesToUse.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).map(snap => ({
                    ...snap,
                    pbsInstanceName: pbsInstance.pbsInstanceName,
                    datastoreName: ds.name,
                    source: 'pbs'
                }))
            )
        );
        
        const pveStorageBackups = [];
        if (pveBackups?.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            pveBackups.storageBackups.forEach(backup => {
                pveStorageBackups.push({
                    'backup-time': backup.ctime,
                    backupType: _extractBackupTypeFromVolid(backup.volid, backup.vmid),
                    backupVMID: backup.vmid,
                    vmid: backup.vmid, // Ensure vmid is preserved for filtering
                    size: backup.size,
                    protected: backup.protected,
                    storage: backup.storage,
                    volid: backup.volid,
                    format: backup.format,
                    node: backup.node,
                    endpointId: backup.endpointId,
                    source: 'pve'
                });
            });
        }
        
        const vmSnapshots = (pveBackups.guestSnapshots || []).map(snap => ({
            ...snap,
            source: 'vmSnapshots'
        }));
        
        const pbsBackupTasks = [];
        pbsInstancesToUse.forEach(pbs => {
            if (pbs.backupTasks?.recentTasks && Array.isArray(pbs.backupTasks.recentTasks)) {
                pbs.backupTasks.recentTasks.forEach(task => {
                    pbsBackupTasks.push({
                        ...task,
                        pbsInstanceName: pbs.pbsInstanceName,
                        source: 'pbs'
                    });
                });
            }
        });
        
        const pveBackupTasks = [];
        if (Array.isArray(pveBackups?.backupTasks)) {
            pveBackups.backupTasks.forEach(task => {
                pveBackupTasks.push({
                    ...task,
                    source: 'pve'
                });
            });
        }
        
        // Create raw backup data (unfiltered)
        const rawBackupData = {
            pbsSnapshots: pbsSnapshots,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots,
            backupTasks: [...pbsBackupTasks, ...pveBackupTasks]
        };
        
        // Create filtered backup data that only contains validated snapshots for each guest
        // This prevents cross-contamination in the detail card
        const filteredPbsSnapshots = [];
        guestToValidatedSnapshots = new Map(); // Store in module scope
        
        // Build a map of guest -> validated snapshots from backupStatusByGuest
        backupStatusByGuest.forEach(guestStatus => {
            const guestKey = `${guestStatus.guestId}-${guestStatus.node || 'unknown'}`;
            const validatedSnapshots = [];
            
            // Find this guest in the original data
            const guest = allGuests.find(g => 
                g.vmid === guestStatus.guestId && 
                g.node === guestStatus.node
            );
            
            if (guest) {
                // Get the validated PBS snapshots for this guest
                // These have already been through strict validation
                const baseKey = `${guest.vmid}-${guest.type === 'qemu' ? 'vm' : 'ct'}`;
                const guestEndpointSuffix = guest.endpointId && guest.endpointId !== 'primary' ?
                    `-${guest.endpointId}` :
                    guest.node ? `-primary-${guest.node.toLowerCase()}` : '-primary';
                
                // Collect validated snapshots from all PBS instances
                pbsInstancesToUse.forEach(pbsInstance => {
                    const namespaces = new Set(['root']);
                    pbsInstance.datastores?.forEach(ds => {
                        ds.snapshots?.forEach(snap => {
                            if (snap.namespace) namespaces.add(snap.namespace);
                        });
                    });
                    
                    namespaces.forEach(namespace => {
                        const pbsKey = `${baseKey}-${pbsInstance.pbsInstanceName}-${namespace}${guestEndpointSuffix}`;
                        const snapshots = snapshotsByGuest.get(pbsKey) || [];
                        
                        // Apply same validation as in guest lookup
                        const validated = snapshots.filter(snap => {
                            const snapVmid = String(snap.backupVMID || snap.vmid || snap['backup-id']);
                            const expectedType = guest.type === 'qemu' ? 'vm' : 'ct';
                            
                            // Must match VMID and type
                            if (snapVmid !== String(guest.vmid)) return false;
                            if (snap.backupType && snap.backupType !== expectedType) return false;
                            
                            // Must match node for primary endpoints
                            if (snap.owner && guest.node && (!guest.endpointId || guest.endpointId === 'primary')) {
                                const ownerParts = snap.owner.split('!');
                                if (ownerParts.length > 1) {
                                    const ownerNode = ownerParts[1].toLowerCase();
                                    if (ownerNode !== guest.node.toLowerCase()) return false;
                                }
                            }
                            
                            return true;
                        });
                        
                        validatedSnapshots.push(...validated);
                    });
                });
            }
            
            guestToValidatedSnapshots.set(guestKey, validatedSnapshots);
            filteredPbsSnapshots.push(...validatedSnapshots);
        });
        
        // Remove duplicates from filteredPbsSnapshots
        const uniqueFilteredPbs = Array.from(new Map(
            filteredPbsSnapshots.map(snap => [`${snap.owner}-${snap['backup-time']}-${snap.backupVMID}`, snap])
        ).values());
        
        // Log filtering results for VMID 102
        const vmid102FilteredGuests = backupStatusByGuest.filter(g => g.guestId === 102);
        if (vmid102FilteredGuests.length > 0) {
            
            vmid102FilteredGuests.forEach(guest => {
                const guestKey = `${guest.guestId}-${guest.node || 'unknown'}`;
                const validatedSnaps = guestToValidatedSnapshots.get(guestKey) || [];
                
            });

            const vmid102Filtered = uniqueFilteredPbs.filter(s => s.backupVMID === '102' || s.backupVMID === 102);
            
        }
        
        // Create properly filtered backup data
        const backupData = {
            pbsSnapshots: uniqueFilteredPbs,
            pveBackups: pveStorageBackups,
            vmSnapshots: vmSnapshots,
            backupTasks: [...pbsBackupTasks, ...pveBackupTasks],
            // Include mapping for detail card to use
            guestToValidatedSnapshots: guestToValidatedSnapshots
        };
        
        // Create calendar respecting current table filters - use unique guest identifiers
        // Apply namespace filtering if active
        let guestsForIds = filteredBackupStatus;
        if (namespaceFilter !== 'all') {
            guestsForIds = filteredBackupStatus.filter(guestStatus => {
                // Only include guests that have backups in the selected namespace
                return guestStatus.totalBackups > 0;
            });
        }
        
        const filteredGuestIds = guestsForIds.map(guest => {
            // Create unique identifier including node/endpoint to handle guests with same vmid on different nodes
            const nodeIdentifier = guest.node || guest.endpointId || '';
            return nodeIdentifier ? `${guest.guestId}-${nodeIdentifier}` : guest.guestId.toString();
        });
        // Get detail card for callback
        const detailCardContainer = document.getElementById('backup-detail-card');
        let onDateSelect = null;
        
        if (detailCardContainer && PulseApp.ui.backupDetailCard) {
            // Find existing detail card or create callback
            const existingCard = detailCardContainer.querySelector('.bg-slate-800');
            if (existingCard) {
                onDateSelect = (dateData, instant = false) => {
                    PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, dateData, instant);
                };
                
                // Clear the detail card when resetting filter
                PulseApp.ui.backupDetailCard.updateBackupDetailCard(existingCard, null, true);
            }
        }
        
        // Reset filter when restoring full calendar view  
        if (PulseApp.ui.calendarHeatmap.resetFilter) {
            PulseApp.ui.calendarHeatmap.resetFilter();
        }
        
        const restoredCalendar = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(backupData, null, filteredGuestIds, onDateSelect);
        // Replace children instead of using innerHTML to avoid flash
        while (calendarContainer.firstChild) {
            calendarContainer.removeChild(calendarContainer.firstChild);
        }
        calendarContainer.appendChild(restoredCalendar);
    }

    function checkGuestBackupForDate(backupData, guestId, dateKey) {
        const guestBackups = [];
        let hasBackups = false;
        
        // Get guest data for name lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(guestId, 10));
        
        if (!guest) return null;
        
        const guestInfo = {
            vmid: guestId,
            name: guest.name,
            type: guest.type === 'qemu' ? 'VM' : 'CT',
            types: [],
            backupCount: 0,
            node: guest.node,
            endpointId: guest.endpointId,
            // Create unique key to match table filtering logic
            uniqueKey: (guest.node || guest.endpointId) ? 
                `${guestId}-${guest.node || guest.endpointId}` : 
                guestId.toString()
        };
        
        // Check all backup sources for this guest on this date
        ['pbsSnapshots', 'pveBackups', 'vmSnapshots'].forEach(source => {
            if (!backupData[source]) return;
            
            const dayBackups = backupData[source].filter(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) return false;
                
                const date = new Date(timestamp * 1000);
                const utcDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
                const itemDateKey = utcDate.toISOString().split('T')[0];
                
                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                if (vmid != guestId || itemDateKey !== dateKey) return false;
                
                // For PBS backups (centralized), don't filter by node
                if (source === 'pbsSnapshots') return true;
                
                // For PVE backups and snapshots (node-specific), match node/endpoint
                const itemNode = item.node;
                const itemEndpoint = item.endpointId;
                
                // Match by node if available
                if (guest.node && itemNode) {
                    return itemNode === guest.node;
                }
                
                // Match by endpointId if available
                if (guest.endpointId && itemEndpoint) {
                    return itemEndpoint === guest.endpointId;
                }
                
                // If no node/endpoint info available, include it (fallback)
                return true;
            });
            
            if (dayBackups.length > 0) {
                hasBackups = true;
                guestInfo.types.push(source);
                guestInfo.backupCount += dayBackups.length;
            }
        });
        
        if (!hasBackups) return null;
        
        // Check for failures
        let hasFailures = false;
        if (backupData.backupTasks) {
            const dayTasks = backupData.backupTasks.filter(task => {
                if (!task.starttime) return false;
                
                // Match vmid
                const taskVmid = task.vmid || task.guestId;
                if (parseInt(taskVmid, 10) !== parseInt(guestId, 10)) return false;
                
                const date = new Date(task.starttime * 1000);
                const utcDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
                const taskDateKey = utcDate.toISOString().split('T')[0];
                
                if (taskDateKey !== dateKey || task.status === 'OK') return false;
                
                // For PBS tasks (centralized), don't filter by node
                if (task.source === 'pbs') return true;
                
                // For PVE tasks (node-specific), match node/endpoint
                const taskNode = task.node;
                const taskEndpoint = task.endpointId;
                
                // Match by node if available
                if (guest.node && taskNode) {
                    return taskNode === guest.node;
                }
                
                // Match by endpointId if available
                if (guest.endpointId && taskEndpoint) {
                    return taskEndpoint === guest.endpointId;
                }
                
                // If no node/endpoint info available, include it (fallback)
                return true;
            });
            
            hasFailures = dayTasks.length > 0;
        }
        
        guestBackups.push(guestInfo);
        
        // Count backup types for stats
        const stats = {
            totalGuests: 1,
            pbsCount: guestInfo.types.includes('pbsSnapshots') ? 1 : 0,
            pveCount: guestInfo.types.includes('pveBackups') ? 1 : 0,
            snapshotCount: guestInfo.types.includes('vmSnapshots') ? 1 : 0,
            failureCount: hasFailures ? 1 : 0
        };
        
        return {
            date: dateKey,
            backups: guestBackups,
            stats: stats
        };
    }

    function _dayHasGuestBackup(dateKey, guestId) {
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        
        const targetDate = new Date(dateKey);
        const startOfDay = new Date(targetDate.getFullYear(), targetDate.getMonth(), targetDate.getDate());
        const endOfDay = new Date(startOfDay.getTime() + 24 * 60 * 60 * 1000);
        const startTimestamp = Math.floor(startOfDay.getTime() / 1000);
        const endTimestamp = Math.floor(endOfDay.getTime() / 1000);
        
        // Check PBS snapshots
        const pbsSnapshots = pbsDataArray.flatMap(pbsInstance =>
            (pbsInstance.datastores || []).flatMap(ds =>
                (ds.snapshots || []).filter(snap => {
                    const vmid = snap['backup-id'];
                    const timestamp = snap['backup-time'];
                    return parseInt(vmid, 10) === parseInt(guestId, 10) && timestamp >= startTimestamp && timestamp < endTimestamp;
                })
            )
        );
        
        if (pbsSnapshots.length > 0) return true;
        
        // Check PVE storage backups
        if (pveBackups.storageBackups && Array.isArray(pveBackups.storageBackups)) {
            const matchingBackups = pveBackups.storageBackups.filter(backup => {
                return parseInt(backup.vmid, 10) === parseInt(guestId, 10) && 
                       backup.ctime >= startTimestamp && 
                       backup.ctime < endTimestamp;
            });
            if (matchingBackups.length > 0) return true;
        }
        
        // Check VM snapshots
        const vmSnapshots = (pveBackups.guestSnapshots || []).filter(snap => {
            return parseInt(snap.vmid, 10) === parseInt(guestId, 10) &&
                   snap.snaptime >= startTimestamp &&
                   snap.snaptime < endTimestamp;
        });
        
        if (vmSnapshots.length > 0) return true;
        
        return false;
    }

    function _highlightTableRows(guestIds, highlight) {
        const backupsTableBody = document.getElementById('backups-overview-tbody');
        if (!backupsTableBody) return;
        
        guestIds.forEach(guestId => {
            const row = backupsTableBody.querySelector(`tr[data-guest-id="${guestId}"]`);
            if (row) {
                if (highlight) {
                    // Apply highlighting to non-sticky cells only to avoid layout shift
                    const cells = row.querySelectorAll('td:not(.sticky)');
                    cells.forEach(cell => {
                        cell.classList.add('bg-blue-50/50', 'dark:bg-blue-900/10');
                    });
                    // Add a subtle left border to the second cell (ID column)
                    const idCell = row.querySelector('td:nth-child(2)');
                    if (idCell) {
                        idCell.classList.add('border-l-2', 'border-l-blue-400', 'dark:border-l-blue-500');
                    }
                } else {
                    const cells = row.querySelectorAll('td:not(.sticky)');
                    cells.forEach(cell => {
                        cell.classList.remove('bg-blue-50/50', 'dark:bg-blue-900/10');
                    });
                    const idCell = row.querySelector('td:nth-child(2)');
                    if (idCell) {
                        idCell.classList.remove('border-l-2', 'border-l-blue-400', 'dark:border-l-blue-500');
                    }
                }
            }
        });
    }

    async function updateBackupsTab(isUserAction = false) {
        // Prevent socket updates too close to user actions
        if (!isUserAction && (Date.now() - lastUserUpdateTime < 1000)) {
            // Skip this update if it's within 1 second of a user action
            return;
        }
        
        // Prevent updates while processing date selection to avoid overwriting
        if (isProcessingDateSelection && !isUserAction) {
            return;
        }
        
        if (isUserAction) {
            lastUserUpdateTime = Date.now();
        }

        // Ensure DOM cache is initialized
        if (!domCache.tableBody) {
            _initDomCache();
        }
        
        // Use cached DOM elements
        const { tableContainer, tableBody, noDataMsg, statusTextElement, pbsSummaryElement, scrollableContainer } = domCache;
        const loadingMsg = document.getElementById('backups-loading-message'); // Not in cache yet

        if (!tableContainer || !tableBody || !loadingMsg || !noDataMsg || !statusTextElement) {
            console.error("UI elements for Backups tab not found!");
            return;
        }

        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;

        // Try to fetch data from the new API first
        const apiData = await fetchBackupDataFromAPI();
        if (apiData && apiData.backupStatusByGuest) {
            // Use API data if available
            _renderBackupsFromAPIData(apiData, tableContainer, tableBody, noDataMsg, statusTextElement, loadingMsg, scrollableContainer, currentScrollLeft, currentScrollTop);
            return;
        }

        // Show error if API fails
        loadingMsg.classList.add('hidden');
        tableContainer.classList.add('hidden');
        noDataMsg.textContent = "Failed to load backup data. Please refresh the page.";
        noDataMsg.classList.remove('hidden');
        return;
    }

    function resetBackupsView() {
        if (backupsSearchInput) backupsSearchInput.value = '';
        PulseApp.state.set('backupsSearchTerm', '');

        const backupTypeAllRadio = document.getElementById('backups-filter-type-all');
        if(backupTypeAllRadio) backupTypeAllRadio.checked = true;
        PulseApp.state.set('backupsFilterGuestType', 'all');

        const backupStatusAllRadio = document.getElementById('backups-filter-status-all');
        if(backupStatusAllRadio) backupStatusAllRadio.checked = true;
        PulseApp.state.set('backupsFilterHealth', 'all');

        const backupBackupTypeAllRadio = document.getElementById('backups-filter-backup-all');
        if(backupBackupTypeAllRadio) backupBackupTypeAllRadio.checked = true;
        PulseApp.state.set('backupsFilterBackupType', 'all');

        const failuresFilter = document.getElementById('backups-filter-failures');
        if(failuresFilter) failuresFilter.checked = false;
        PulseApp.state.set('backupsFilterFailures', false);
        
        // Reset PBS instance filter
        if (pbsInstanceFilter) pbsInstanceFilter.value = 'all';
        PulseApp.state.set('backupsFilterPbsInstance', 'all');
        
        // Reset namespace filter
        if (namespaceFilter) namespaceFilter.value = 'all';
        PulseApp.state.set('backupsFilterNamespace', 'all');

        PulseApp.state.setSortState('backups', 'latestBackupTime', 'desc');

        // Clear calendar filter selection
        PulseApp.state.set('currentFilteredGuest', null);
        
        // Clear calendar date filter
        PulseApp.state.set('calendarDateFilter', null);
        
        // Clear calendar selection if possible
        if (PulseApp.ui.calendarHeatmap && PulseApp.ui.calendarHeatmap.clearSelection) {
            PulseApp.ui.calendarHeatmap.clearSelection();
        }

        updateBackupsTab(true); // Mark as user action
        PulseApp.state.saveFilterState(); // Save reset state
    }

    function _initSnapshotModal() {
        const modal = document.getElementById('snapshot-modal');
        const modalClose = document.getElementById('snapshot-modal-close');
        const modalBody = document.getElementById('snapshot-modal-body');
        const modalTitle = document.getElementById('snapshot-modal-title');
        
        if (!modal || !modalClose || !modalBody) {
            console.warn('[Backups] Snapshot modal elements not found');
            return;
        }
        
        // Close modal on click outside or close button
        modalClose.addEventListener('click', () => {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        });
        
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.classList.add('hidden');
                modal.classList.remove('flex');
            }
        });
        
        // Handle snapshot button clicks
        document.addEventListener('click', (e) => {
            const button = e.target.closest('.view-snapshots-btn');
            if (button) {
                const vmid = button.dataset.vmid;
                const node = button.dataset.node;
                const endpoint = button.dataset.endpoint;
                const type = button.dataset.type;
                
                _showSnapshotModal(vmid, node, endpoint, type);
            }
        });
    }
    
    function _showSnapshotModal(vmid, node, endpoint, type) {
        const modal = document.getElementById('snapshot-modal');
        const modalBody = document.getElementById('snapshot-modal-body');
        const modalTitle = document.getElementById('snapshot-modal-title');
        
        if (!modal || !modalBody || !modalTitle) return;
        
        // Get guest info
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const guest = [...vmsData, ...containersData].find(g => g.vmid === vmid);
        const guestName = guest?.name || `Guest ${vmid}`;
        
        modalTitle.textContent = `Snapshots for ${guestName} (${type.toUpperCase()} ${vmid})`;
        modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Loading snapshots...</p>';
        
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        
        // Get snapshots from state
        const pveBackups = PulseApp.state.get('pveBackups') || {};
        const snapshots = (pveBackups.guestSnapshots || [])
            .filter(snap => parseInt(snap.vmid, 10) === parseInt(vmid, 10))
            .sort((a, b) => (b.snaptime || 0) - (a.snaptime || 0));
        
        if (snapshots.length === 0) {
            modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">No snapshots found for this guest.</p>';
            return;
        }
        
        // Build snapshot table
        let html = `
            <div class="overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead class="bg-gray-50 dark:bg-gray-700">
                        <tr>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Description</th>
                            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">RAM</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
        `;
        
        snapshots.forEach(snap => {
            const created = snap.snaptime 
                ? new Date(snap.snaptime * 1000).toLocaleString()
                : 'Unknown';
            const hasRam = snap.vmstate ? 'Yes' : 'No';
            const description = snap.description || '-';
            
            html += `
                <tr>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">${snap.name}</td>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">${created}</td>
                    <td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">${description}</td>
                    <td class="px-4 py-2 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">${hasRam}</td>
                </tr>
            `;
        });
        
        html += `
                    </tbody>
                </table>
            </div>
        `;
        
        modalBody.innerHTML = html;
    }

    function _guestHasBackupOnDate(guestStatus, dateString, backupData) {
        const guestId = guestStatus.guestId.toString();
        const types = [];
        let count = 0;
        
        // Get current namespace filter
        const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
        
        // Check PBS snapshots - use validated snapshots only
        if (guestStatus.pbsBackups > 0 && backupData.pbsSnapshots) {
            backupData.pbsSnapshots.forEach(snap => {
                const snapId = snap['backup-id'] || snap.backupVMID;
                
                // Verify VMID matches
                if (String(snapId) !== String(guestStatus.guestId)) {
                    return;
                }
                
                // Verify type matches
                const snapType = snap.backupType || snap['backup-type'] || '';
                const expectedType = guestStatus.guestType === 'VM' ? 'vm' : 'ct';
                if (snapType !== expectedType) {
                    return;
                }
                
                // Apply namespace filter for PBS snapshots
                if (namespaceFilter !== 'all') {
                    const { targetPbsIndex, targetNamespace } = _parseNamespaceFilter(namespaceFilter);
                    
                    // Check PBS instance if specified
                    if (targetPbsIndex !== null) {
                        const snapPbsInstance = snap.pbsInstanceName || '';
                        const pbsIndex = backupData.pbsDataArray?.findIndex(pbs => pbs.pbsInstanceName === snapPbsInstance) || -1;
                        if (pbsIndex !== targetPbsIndex) {
                            return;
                        }
                    }
                    
                    const snapNamespace = snap.namespace || 'root';
                    const normalizedSnapNamespace = (!snapNamespace || snapNamespace === '' || snapNamespace === 'root') ? 'root' : snapNamespace;
                    const normalizedFilterNamespace = (!targetNamespace || targetNamespace === '' || targetNamespace === 'root') ? 'root' : targetNamespace;
                    if (normalizedSnapNamespace !== normalizedFilterNamespace) {
                        // Skip this snapshot as it doesn't match the namespace filter
                        return;
                    }
                }
                
                // CRITICAL: Verify node/owner matches to prevent cross-contamination
                if (snap.owner && snap.owner.includes('!')) {
                    const ownerNode = snap.owner.split('!')[1].toLowerCase();
                    
                    // For primary endpoints, check node name
                    if (guestStatus.endpointId === 'primary' || !guestStatus.endpointId) {
                        if (guestStatus.node && ownerNode !== guestStatus.node.toLowerCase()) {
                            // This snapshot is from a different node, skip it
                            return;
                        }
                    }
                }
                
                // Additional validation for VMID 102 to prevent Windows/Container mix-up
                if (guestStatus.guestId === 102) {
                    const guestNameLower = guestStatus.guestName.toLowerCase();
                    const isWindows = guestNameLower.includes('windows') || guestNameLower.includes('win11');
                    
                    // Check comment for mismatches
                    if (snap.comment) {
                        const commentLower = snap.comment.toLowerCase();
                        const containerNames = ['docker', 'pi-', 'pulse', 'homepage', 'debian', 'pbs', 'jellyfin', 'pihole', 'influxdb', 'socat'];
                        
                        if (isWindows && containerNames.some(name => commentLower.includes(name))) {
                            // Windows guest but snapshot has container comment
                            return;
                        }
                        if (!isWindows && (commentLower.includes('windows') || commentLower.includes('win11'))) {
                            // Container guest but snapshot has Windows comment
                            return;
                        }
                    }
                    
                    // Check backup-id pattern
                    if (snap['backup-id']) {
                        const backupId = snap['backup-id'];
                        if (isWindows && expectedType === 'vm' && backupId.includes('/ct/')) {
                            // Windows VM but backup-id indicates container
                            return;
                        }
                        if (!isWindows && expectedType === 'ct' && backupId.includes('/vm/')) {
                            // Container but backup-id indicates VM
                            return;
                        }
                    }
                }
                
                const timestamp = snap['backup-time'];
                if (timestamp) {
                    const snapDate = new Date(timestamp * 1000);
                    const snapDateStr = snapDate.toISOString().split('T')[0];
                    if (snapDateStr === dateString) {
                        if (!types.includes('pbsSnapshots')) types.push('pbsSnapshots');
                        count++;
                    }
                }
            });
        }
        
        // Check PVE backups
        if (guestStatus.pveBackups > 0 && backupData.pveBackups) {
            backupData.pveBackups.forEach(backup => {
                if (parseInt(backup.vmid, 10) === parseInt(guestId, 10)) {
                    const timestamp = backup.ctime || backup['backup-time'];
                    if (timestamp) {
                        const backupDate = new Date(timestamp * 1000);
                        const backupDateStr = backupDate.toISOString().split('T')[0];
                        if (backupDateStr === dateString) {
                            if (!types.includes('pveBackups')) types.push('pveBackups');
                            count++;
                        }
                    }
                }
            });
        }
        
        // Check VM snapshots
        if (guestStatus.snapshotCount > 0 && backupData.vmSnapshots) {
            backupData.vmSnapshots.forEach(snap => {
                if (parseInt(snap.vmid, 10) === parseInt(guestId, 10)) {
                    const timestamp = snap.snaptime;
                    if (timestamp) {
                        const snapDate = new Date(timestamp * 1000);
                        const snapDateStr = snapDate.toISOString().split('T')[0];
                        if (snapDateStr === dateString) {
                            if (!types.includes('vmSnapshots')) types.push('vmSnapshots');
                            count++;
                        }
                    }
                }
            });
        }
        
        return count > 0 ? { types, count } : null;
    }

    function _prepareMultiDateDetailData(filteredBackupStatus, backupData) {
        // Prepare data for multi-date detail view
        const multiDateBackups = [];
        
        // Get current filters to ensure stats are context-aware
        const namespaceFilter = PulseApp.state.get('backupsFilterNamespace') || 'all';
        
        // IMPORTANT: When namespace filter is 'all', the same guest might appear multiple times
        // in filteredBackupStatus if it has backups in different namespaces. We need to count
        // unique guests properly based on the current filter context.
        const stats = {
            totalGuests: filteredBackupStatus.length, // This will be recalculated if needed
            totalBackups: 0,
            pbsCount: 0,
            pveCount: 0,
            snapshotCount: 0,
            failureCount: 0
        };
        
        // Get current backup type filter to determine which dates to include
        const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType') || 'all';
        
        // Get all backup data for filtered guests
        filteredBackupStatus.forEach(guestStatus => {
            const guestId = guestStatus.guestId.toString();
            const backupDates = [];
            
            // Log when we process VMID 102 guests
            if (guestId === '102') {
                
            }
            
            // Check PBS snapshots (only if filter allows PBS backups)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'pbs') && backupData.pbsSnapshots) {
                const pbsDates = {};
                
                // CRITICAL: Use validated snapshots only to prevent cross-contamination
                const guestKey = `${guestStatus.guestId}-${guestStatus.node || 'unknown'}`;
                
                // Since guestToValidatedSnapshots is in a different scope, we need to filter here
                const validatedSnapshots = [];
                
                // Filter PBS snapshots for this specific guest
                backupData.pbsSnapshots.forEach(snap => {
                    const snapId = snap['backup-id'] || snap.backupVMID;
                    
                    // Log only if this snapshot belongs to the current guest
                    if ((snapId === '102' || snapId === 102) && String(snapId) === String(guestStatus.guestId)) {
                        
                    }
                    
                    // 1. Verify VMID matches exactly
                    if (String(snapId) !== String(guestStatus.guestId)) {
                        return;
                    }
                    
                    // 2. Verify type matches (vm vs ct)
                    const snapType = snap.backupType || snap['backup-type'] || '';
                    const expectedType = guestStatus.guestType === 'VM' ? 'vm' : 'ct';
                    if (snapType !== expectedType) {
                        
                        return;
                    }
                    
                    // 3. Validate owner field against guest's node/endpoint
                    const owner = snap.owner || '';
                    if (owner && owner.includes('!')) {
                        const ownerToken = owner.split('!')[1].toLowerCase();
                        
                        // For primary endpoint guests, validate against node name
                        if (!guestStatus.endpointId || guestStatus.endpointId === 'primary') {
                            if (guestStatus.node && ownerToken !== guestStatus.node.toLowerCase()) {
                                
                                return;
                            }
                        } else {
                            // For secondary endpoint guests, validate against endpoint cluster name
                            // The nodeDisplayName format is "ClusterName - NodeName (endpointId)"
                            if (guestStatus.nodeDisplayName) {
                                const clusterName = guestStatus.nodeDisplayName.split(' - ')[0].toLowerCase();
                                if (ownerToken !== clusterName) {
                                    
                                    return;
                                }
                            }
                        }
                    } else {
                        // If no owner field, we can't validate - log warning but allow for now
                        
                    }
                    
                    // Log successful validation for debugging (only for the guest being processed)
                    if ((snapId === '102' || snapId === 102) && String(snapId) === String(guestStatus.guestId)) {
                        
                    }
                    
                    // Passed all validation
                    validatedSnapshots.push(snap);
                });

                // Only use validated snapshots for this specific guest
                validatedSnapshots.forEach(snap => {
                    const timestamp = snap['backup-time'];
                    if (timestamp) {
                        const date = new Date(timestamp * 1000);
                        const dateKey = date.toISOString().split('T')[0];
                        if (!pbsDates[dateKey]) {
                            pbsDates[dateKey] = { types: new Set(), count: 0, latestTimestamp: timestamp };
                        } else {
                            // Keep the latest timestamp for this date
                            if (timestamp > pbsDates[dateKey].latestTimestamp) {
                                pbsDates[dateKey].latestTimestamp = timestamp;
                            }
                        }
                        pbsDates[dateKey].types.add('pbsSnapshots');
                        pbsDates[dateKey].count++;
                        stats.totalBackups++;
                    }
                });
                
                Object.entries(pbsDates).forEach(([date, info]) => {
                    backupDates.push({ date, types: Array.from(info.types), count: info.count, latestTimestamp: info.latestTimestamp });
                });
            }
            
            // Check PVE backups (only if filter allows PVE backups)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'pve') && backupData.pveBackups) {
                const pveDates = {};
                backupData.pveBackups.forEach(backup => {
                    if (parseInt(backup.vmid, 10) === parseInt(guestId, 10)) {
                        const timestamp = backup['backup-time'] || backup.ctime;
                        if (timestamp) {
                            const date = new Date(timestamp * 1000);
                            const dateKey = date.toISOString().split('T')[0];
                            if (!pveDates[dateKey]) {
                                pveDates[dateKey] = { types: new Set(), count: 0, latestTimestamp: timestamp };
                            } else {
                                // Keep the latest timestamp for this date
                                if (timestamp > pveDates[dateKey].latestTimestamp) {
                                    pveDates[dateKey].latestTimestamp = timestamp;
                                }
                            }
                            pveDates[dateKey].types.add('pveBackups');
                            pveDates[dateKey].count++;
                            stats.totalBackups++;
                        }
                    }
                });
                Object.entries(pveDates).forEach(([date, info]) => {
                    const existing = backupDates.find(d => d.date === date);
                    if (existing) {
                        existing.types = [...new Set([...existing.types, ...Array.from(info.types)])];
                        existing.count += info.count;
                        // Keep the latest timestamp across all backup types for this date
                        if (info.latestTimestamp > existing.latestTimestamp) {
                            existing.latestTimestamp = info.latestTimestamp;
                        }
                    } else {
                        backupDates.push({ date, types: Array.from(info.types), count: info.count, latestTimestamp: info.latestTimestamp });
                    }
                });
            }
            
            // Check VM snapshots (only if filter allows snapshots)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'snapshots') && backupData.vmSnapshots) {
                const snapDates = {};
                backupData.vmSnapshots.forEach(snap => {
                    if (parseInt(snap.vmid, 10) === parseInt(guestId, 10)) {
                        const timestamp = snap.snaptime;
                        if (timestamp) {
                            const date = new Date(timestamp * 1000);
                            const dateKey = date.toISOString().split('T')[0];
                            if (!snapDates[dateKey]) {
                                snapDates[dateKey] = { types: new Set(), count: 0, latestTimestamp: timestamp };
                            } else {
                                // Keep the latest timestamp for this date
                                if (timestamp > snapDates[dateKey].latestTimestamp) {
                                    snapDates[dateKey].latestTimestamp = timestamp;
                                }
                            }
                            snapDates[dateKey].types.add('vmSnapshots');
                            snapDates[dateKey].count++;
                            stats.totalBackups++;
                        }
                    }
                });
                Object.entries(snapDates).forEach(([date, info]) => {
                    const existing = backupDates.find(d => d.date === date);
                    if (existing) {
                        existing.types = [...new Set([...existing.types, ...Array.from(info.types)])];
                        existing.count += info.count;
                        // Keep the latest timestamp across all backup types for this date
                        if (info.latestTimestamp > existing.latestTimestamp) {
                            existing.latestTimestamp = info.latestTimestamp;
                        }
                    } else {
                        backupDates.push({ date, types: Array.from(info.types), count: info.count, latestTimestamp: info.latestTimestamp });
                    }
                });
            }
            
            // Update stats
            if (guestStatus.pbsBackups > 0) stats.pbsCount++;
            if (guestStatus.pveBackups > 0) stats.pveCount++;
            if (guestStatus.snapshotCount > 0) stats.snapshotCount++;
            if (guestStatus.recentFailures > 0) stats.failureCount++;
            
            if (backupDates.length > 0) {
                multiDateBackups.push({
                    ...guestStatus,
                    backupDates: backupDates.sort((a, b) => b.latestTimestamp - a.latestTimestamp)
                });
            } else {
                // Include ALL guests in multiDateBackups, even those without backups
                // This ensures totalGuests matches the filtered guest count
                multiDateBackups.push({
                    ...guestStatus,
                    backupDates: []
                });
            }
        });
        
        // Sort by most recent backup to see which ones will appear in "Recent"
        const sortedByRecent = multiDateBackups
            .filter(g => g.backupDates && g.backupDates.length > 0)
            .sort((a, b) => {
                const latestTimestampA = a.backupDates[0].latestTimestamp;
                const latestTimestampB = b.backupDates[0].latestTimestamp;
                return latestTimestampB - latestTimestampA;
            });

        // Get current filter info
        const filterInfo = {
            search: backupsSearchInput ? backupsSearchInput.value : '',
            guestType: PulseApp.state.get('backupsFilterGuestType'),
            backupType: backupTypeFilter,
            healthStatus: PulseApp.state.get('backupsFilterHealth'),
            failuresOnly: PulseApp.state.get('backupsFilterFailures') || false,
            namespace: PulseApp.state.get('backupsFilterNamespace') || 'all',
            pbsInstance: pbsInstanceFilter ? pbsInstanceFilter.value : 'all'
        };
        

        // Recalculate stats based on unique guests with backups
        // Use composite key (guestId-node) to handle guests with same VMID on different nodes
        const uniqueGuestsWithPBS = new Set();
        const uniqueGuestsWithPVE = new Set();
        const uniqueGuestsWithSnapshots = new Set();
        
        multiDateBackups.forEach(guest => {
            // Create unique identifier that includes both guest ID and node/endpoint
            const uniqueKey = `${guest.guestId}-${guest.node || guest.endpointId || 'unknown'}`;
            
            // Count guests based on whether they have backup dates of the filtered type
            // This ensures we count all guests that appear in the filtered view, not just recent activity
            if (backupTypeFilter === 'all') {
                // When showing all types, use the guest's overall backup counts
                if (guest.pbsBackups > 0) uniqueGuestsWithPBS.add(uniqueKey);
                if (guest.pveBackups > 0) uniqueGuestsWithPVE.add(uniqueKey);
                if (guest.snapshotCount > 0) uniqueGuestsWithSnapshots.add(uniqueKey);
            } else {
                // When filtering by specific type, count guests who have that type in their backup dates
                // OR if they have no backup dates but have backups of that type (important for namespace filtering)
                const hasFilteredBackupType = guest.backupDates && guest.backupDates.length > 0 ?
                    guest.backupDates.some(dateInfo => {
                        if (backupTypeFilter === 'pbs') return dateInfo.types.includes('pbsSnapshots');
                        if (backupTypeFilter === 'pve') return dateInfo.types.includes('pveBackups');
                        if (backupTypeFilter === 'snapshots') return dateInfo.types.includes('vmSnapshots');
                        return false;
                    }) :
                    // Fallback to guest backup counts if no dates available
                    (backupTypeFilter === 'pbs' && guest.pbsBackups > 0) ||
                    (backupTypeFilter === 'pve' && guest.pveBackups > 0) ||
                    (backupTypeFilter === 'snapshots' && guest.snapshotCount > 0);
                
                if (hasFilteredBackupType) {
                    if (backupTypeFilter === 'pbs') uniqueGuestsWithPBS.add(uniqueKey);
                    if (backupTypeFilter === 'pve') uniqueGuestsWithPVE.add(uniqueKey);
                    if (backupTypeFilter === 'snapshots') uniqueGuestsWithSnapshots.add(uniqueKey);
                }
            }
        });
        
        stats.pbsCount = uniqueGuestsWithPBS.size;
        stats.pveCount = uniqueGuestsWithPVE.size;
        stats.snapshotCount = uniqueGuestsWithSnapshots.size;
        
        
        // Recalculate total guests based on actual unique guests in the filtered view
        // This is important when namespace filter is 'all' as the same guest might appear
        // multiple times in different namespaces in the original filteredBackupStatus
        stats.totalGuests = multiDateBackups.length;
        
        // Debug: Check for guests with multiple backup types
        const guestsWithMultipleTypes = [];
        const allUniqueGuests = new Set();
        
        multiDateBackups.forEach(guest => {
            const uniqueKey = `${guest.guestId}-${guest.node || guest.endpointId || 'unknown'}`;
            allUniqueGuests.add(uniqueKey);
            
            let backupTypeCount = 0;
            if (guest.pbsBackups > 0) backupTypeCount++;
            if (guest.pveBackups > 0) backupTypeCount++;
            if (guest.snapshotCount > 0) backupTypeCount++;
            
            if (backupTypeCount > 1) {
                guestsWithMultipleTypes.push({
                    uniqueKey,
                    guestName: guest.guestName,
                    pbsBackups: guest.pbsBackups,
                    pveBackups: guest.pveBackups,
                    snapshotCount: guest.snapshotCount,
                    backupTypeCount
                });
            }
        });
        
        
        // Calculate unique dates for filtered view
        const uniqueDates = new Set();
        multiDateBackups.forEach(guest => {
            guest.backupDates.forEach(dateInfo => {
                uniqueDates.add(dateInfo.date);
            });
        });
        stats.uniqueDates = uniqueDates.size;
        
        return {
            isMultiDate: true,
            backups: multiDateBackups,
            stats: stats,
            filterInfo: filterInfo
        };
    }

    function _initNamespaceFilter() {
        namespaceFilter = document.getElementById('backups-filter-namespace');
        const filterGroup = document.getElementById('pbs-namespace-filter-group');
        
        if (!namespaceFilter || !filterGroup) return;
        
        // Create debounced update function to prevent rapid updates
        const debouncedNamespaceUpdate = PulseApp.utils.debounce(() => {
            PulseApp.state.set('backupsFilterNamespace', namespaceFilter.value);
            // Clear API cache when namespace changes
            apiBackupData = null;
            apiDataTimestamp = 0;
            // Clear calendar cache when namespace changes
            if (PulseApp.ui.calendarHeatmap && PulseApp.ui.calendarHeatmap.clearCache) {
                PulseApp.ui.calendarHeatmap.clearCache();
            }
            updateBackupsTab(true);
        }, 300); // 300ms delay
        
        // Add change listener
        namespaceFilter.addEventListener('change', debouncedNamespaceUpdate);
        
        // Update namespace options when PBS data changes
        // Note: Using state change tracking instead of eventBus
        const originalPbsData = PulseApp.state.get('pbsDataArray');
        let lastPbsDataLength = originalPbsData ? originalPbsData.length : 0;
        
        // Check for PBS data changes periodically
        const checkPbsDataChanges = () => {
            const currentPbsData = PulseApp.state.get('pbsDataArray');
            const currentLength = currentPbsData ? currentPbsData.length : 0;
            if (currentLength !== lastPbsDataLength) {
                lastPbsDataLength = currentLength;
                _updateNamespaceOptions();
            }
        };
        
        // Check for changes every 2 seconds
        setInterval(checkPbsDataChanges, 2000);
        
        // Initial update
        _updateNamespaceOptions();
    }
    
    function _updateNamespaceOptions() {
        if (!namespaceFilter) return;
        
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const selectedPbsInstance = pbsInstanceFilter ? pbsInstanceFilter.value : 'all';
        
        // Collect namespaces grouped by PBS instance
        const namespacesByInstance = new Map();
        
        // Collect namespaces from PBS data based on selected instance
        const instancesToCheck = selectedPbsInstance === 'all' 
            ? pbsDataArray 
            : pbsDataArray.filter((_, index) => index.toString() === selectedPbsInstance);
            
        instancesToCheck.forEach((pbsInstance, globalIndex) => {
            const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${pbsDataArray.indexOf(pbsInstance) + 1}`;
            const instanceNamespaces = new Set();
            
            if (pbsInstance.datastores) {
                pbsInstance.datastores.forEach(ds => {
                    if (ds.snapshots) {
                        ds.snapshots.forEach(snap => {
                            // Normalize namespace - treat '', null, undefined as 'root'
                            const namespace = snap.namespace || '';
                            const normalizedNamespace = (!namespace || namespace === '') ? 'root' : namespace;
                            instanceNamespaces.add(normalizedNamespace);
                        });
                    }
                });
            }
            
            if (instanceNamespaces.size > 0) {
                namespacesByInstance.set(instanceName, {
                    index: pbsDataArray.indexOf(pbsInstance),
                    namespaces: Array.from(instanceNamespaces).sort((a, b) => {
                        if (a === 'root') return -1;
                        if (b === 'root') return 1;
                        return a.localeCompare(b);
                    })
                });
            }
        });
        
        // Update options
        const currentValue = namespaceFilter.value || 'all';
        namespaceFilter.innerHTML = '<option value="all">All Namespaces</option>';
        
        // If multiple PBS instances, group namespaces by instance
        if (namespacesByInstance.size > 1 && selectedPbsInstance === 'all') {
            // Add grouped options
            namespacesByInstance.forEach((instanceData, instanceName) => {
                // Create optgroup for this PBS instance
                const optgroup = document.createElement('optgroup');
                optgroup.label = instanceName;
                
                instanceData.namespaces.forEach(ns => {
                    const option = document.createElement('option');
                    // Value includes instance index and namespace for unique identification
                    option.value = `${instanceData.index}:${ns}`;
                    option.textContent = ns === 'root' ? 'Root Namespace' : ns;
                    optgroup.appendChild(option);
                });
                
                namespaceFilter.appendChild(optgroup);
            });
        } else {
            // Single PBS instance or specific instance selected - flat list
            const allNamespaces = new Set();
            namespacesByInstance.forEach(instanceData => {
                instanceData.namespaces.forEach(ns => allNamespaces.add(ns));
            });
            
            const sortedNamespaces = Array.from(allNamespaces).sort((a, b) => {
                if (a === 'root') return -1;
                if (b === 'root') return 1;
                return a.localeCompare(b);
            });
            
            sortedNamespaces.forEach(ns => {
                const option = document.createElement('option');
                option.value = ns;
                option.textContent = ns === 'root' ? 'Root Namespace' : `${ns} Namespace`;
                namespaceFilter.appendChild(option);
            });
        }
        
        // Restore selection if it still exists
        const hasOption = Array.from(namespaceFilter.options).some(opt => opt.value === currentValue);
        if (hasOption) {
            namespaceFilter.value = currentValue;
        } else {
            namespaceFilter.value = 'all';
        }
        
        // Show/hide filter based on:
        // 1. PBS backup type being selected
        // 2. PBS being available
        // 3. Having multiple namespaces
        const filterGroup = document.getElementById('pbs-namespace-filter-group');
        if (filterGroup) {
            const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType') || 'all';
            const isPBSFilterActive = backupTypeFilter === 'pbs';
            
            // Calculate total unique namespaces across all instances
            const allNamespacesSet = new Set();
            namespacesByInstance.forEach(instanceData => {
                instanceData.namespaces.forEach(ns => allNamespacesSet.add(ns));
            });
            
            const hasMultipleNamespaces = allNamespacesSet.size > 1;
            const hasPBS = instancesToCheck.length > 0;
            
            // Only show namespace filter when PBS type is selected AND conditions are met
            const shouldShowFilter = isPBSFilterActive && hasPBS && hasMultipleNamespaces;
            filterGroup.style.display = shouldShowFilter ? '' : 'none';
            
            // Reset namespace filter to 'all' when hiding it
            if (!shouldShowFilter && namespaceFilter && namespaceFilter.value !== 'all') {
                namespaceFilter.value = 'all';
                PulseApp.state.set('backupsFilterNamespace', 'all');
            }
        }
    }
    
    function _initPbsInstanceFilter() {
        pbsInstanceFilter = document.getElementById('backups-filter-pbs-instance');
        const filterGroup = document.getElementById('pbs-instance-filter-group');
        
        if (!pbsInstanceFilter || !filterGroup) return;
        
        // Create debounced update function to prevent rapid updates
        const debouncedPbsInstanceUpdate = PulseApp.utils.debounce(() => {
            PulseApp.state.set('backupsFilterPbsInstance', pbsInstanceFilter.value);
            // Update namespace options based on selected instance
            _updateNamespaceOptions();
            updateBackupsTab(true);
        }, 300); // 300ms delay
        
        // Add change listener
        pbsInstanceFilter.addEventListener('change', debouncedPbsInstanceUpdate);
        
        // Update PBS instance options when PBS data changes
        const originalPbsData = PulseApp.state.get('pbsDataArray');
        let lastPbsDataLength = originalPbsData ? originalPbsData.length : 0;
        
        // Check for PBS data changes periodically
        const checkPbsDataChanges = () => {
            const currentPbsData = PulseApp.state.get('pbsDataArray');
            const currentLength = currentPbsData ? currentPbsData.length : 0;
            if (currentLength !== lastPbsDataLength) {
                lastPbsDataLength = currentLength;
                _updatePbsInstanceOptions();
            }
        };
        
        // Check for changes every 2 seconds
        setInterval(checkPbsDataChanges, 2000);
        
        // Initial update
        _updatePbsInstanceOptions();
    }
    
    function _updatePbsInstanceOptions() {
        if (!pbsInstanceFilter) return;
        
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        
        // Update options
        const currentValue = pbsInstanceFilter.value || 'all';
        pbsInstanceFilter.innerHTML = '<option value="all">All PBS Instances</option>';
        
        pbsDataArray.forEach((pbsInstance, index) => {
            const option = document.createElement('option');
            option.value = index.toString();
            option.textContent = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
            pbsInstanceFilter.appendChild(option);
        });
        
        // Restore selection
        pbsInstanceFilter.value = currentValue;
        
        // Show/hide filter based on:
        // 1. PBS backup type being selected  
        // 2. Having multiple PBS instances
        const filterGroup = document.getElementById('pbs-instance-filter-group');
        if (filterGroup) {
            const backupTypeFilter = PulseApp.state.get('backupsFilterBackupType') || 'all';
            const isPBSFilterActive = backupTypeFilter === 'pbs';
            const hasMultiplePBS = pbsDataArray.length > 1;
            
            // Only show PBS instance filter when PBS type is selected AND multiple instances exist
            const shouldShowFilter = isPBSFilterActive && hasMultiplePBS;
            filterGroup.style.display = shouldShowFilter ? '' : 'none';
            
            // Reset PBS instance filter to 'all' when hiding it
            if (!shouldShowFilter && pbsInstanceFilter && pbsInstanceFilter.value !== 'all') {
                pbsInstanceFilter.value = 'all';
                PulseApp.state.set('backupsFilterPbsInstance', 'all');
            }
        }
    }

    // Enhanced debug function for console
    window.debugBackups = function(vmid) {
        const state = PulseApp.state;
        const pveBackups = state.get('pveBackups');
        const pbsData = state.get('pbsData');

        if (pveBackups && pveBackups.storageBackups) {
            const backups = pveBackups.storageBackups;

            // Look for June 7th
            const june7 = backups.filter(b => {
                const d = new Date(b.ctime * 1000);
                return d.getMonth() === 5 && d.getDate() === 7;
            });
            
            if (june7.length > 0) {
                june7.forEach(b => {
                    const d = new Date(b.ctime * 1000);
                });
            }
            
            // Recent backups
            backups.sort((a,b) => b.ctime - a.ctime).slice(0,3).forEach(b => {
                const d = new Date(b.ctime * 1000);
            });
        }
        
        // Check calendar
        const currentMonth = document.querySelector('.calendar-month-container h3');

        const june7Cell = document.querySelector('[data-date="2024-06-07"]');
        if (june7Cell) {
            // June 7 cell found
        } else {
            // June 7 cell not in current view
        }
        
        return 'Debug complete - check console output';
    };
    
    return {
        init,
        updateBackupsTab,
        resetBackupsView,
        _highlightTableRows
    };
})();
