PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    // State management - single source of truth
    const backupsState = {
        // Filter state
        filters: {
            guestType: 'all',      // all, vm, lxc
            healthStatus: 'all',   // all, ok, stale, warning, none
            backupType: 'all',     // all, pbs, pve, snapshots
            showFailures: false,
            searchTerm: '',
            groupByNode: true,
            namespace: 'all',
            pbsInstance: 'all'
        },
        
        // Data state
        data: {
            raw: null,           // Raw API response
            filtered: null,      // Filtered data
            lastFetch: 0,        // Timestamp
            loading: false,      // Loading state
            error: null         // Error state
        }
    };
    
    // DOM element references
    let domElements = {
        tableBody: null,
        tableContainer: null,
        noDataMsg: null,
        loadingMsg: null,
        statusText: null,
        searchInput: null,
        resetButton: null,
        scrollableContainer: null,
        visualizationSection: null,
        calendarContainer: null,
        detailCardContainer: null,
        namespaceFilter: null,
        pbsInstanceFilter: null
    };
    
    // Debounce timer for API fetches
    let fetchDebounceTimer = null;
    
    // Initialize the backups tab
    function init() {
        _initDOMElements();
        _initEventListeners();
        _loadSavedFilters();
        updateBackupsTab();
    }
    
    // Initialize DOM element references
    function _initDOMElements() {
        domElements.tableBody = document.getElementById('backups-overview-tbody');
        domElements.tableContainer = document.getElementById('backups-table-container');
        domElements.noDataMsg = document.getElementById('no-backup-data-message');
        domElements.loadingMsg = document.getElementById('backup-loading-message');
        domElements.statusText = document.getElementById('backup-status-text');
        domElements.searchInput = document.getElementById('backups-search');
        domElements.resetButton = document.getElementById('reset-backups-filters-button');
        domElements.scrollableContainer = document.querySelector('#backups-table-container .overflow-x-auto');
        domElements.visualizationSection = document.getElementById('backup-visualization-section');
        domElements.calendarContainer = document.getElementById('backup-calendar-heatmap');
        domElements.detailCardContainer = document.getElementById('backup-detail-card');
        domElements.namespaceFilter = document.getElementById('namespace-filter');
        domElements.pbsInstanceFilter = document.getElementById('pbs-instance-filter');
    }
    
    // Initialize all event listeners
    function _initEventListeners() {
        // Guest type filter (VM/Container/All)
        document.querySelectorAll('input[name="backups-type-filter"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                if (e.target.checked) {
                    handleFilterChange('guestType', e.target.value);
                }
            });
        });
        
        // Health status filter
        document.querySelectorAll('input[name="backups-status-filter"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                if (e.target.checked) {
                    handleFilterChange('healthStatus', e.target.value);
                }
            });
        });
        
        // Backup type filter (PBS/PVE/Snapshots)
        document.querySelectorAll('input[name="backups-backup-filter"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                if (e.target.checked) {
                    handleFilterChange('backupType', e.target.value);
                }
            });
        });
        
        // Group by node filter
        document.querySelectorAll('input[name="backups-group-filter"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                if (e.target.checked) {
                    handleFilterChange('groupByNode', e.target.value === 'grouped');
                }
            });
        });
        
        // Failures checkbox
        const failuresCheckbox = document.getElementById('backups-filter-failures');
        if (failuresCheckbox) {
            failuresCheckbox.addEventListener('change', (e) => {
                handleFilterChange('showFailures', e.target.checked);
            });
        }
        
        // Search input with debouncing
        if (domElements.searchInput) {
            domElements.searchInput.addEventListener('input', (e) => {
                handleFilterChange('searchTerm', e.target.value);
            });
        }
        
        // Namespace filter
        if (domElements.namespaceFilter) {
            domElements.namespaceFilter.addEventListener('change', (e) => {
                handleFilterChange('namespace', e.target.value);
            });
        }
        
        // PBS instance filter
        if (domElements.pbsInstanceFilter) {
            domElements.pbsInstanceFilter.addEventListener('change', (e) => {
                handleFilterChange('pbsInstance', e.target.value);
            });
        }
        
        // Reset button
        if (domElements.resetButton) {
            domElements.resetButton.addEventListener('click', resetFilters);
        }
    }
    
    // Load saved filter preferences
    function _loadSavedFilters() {
        // Load from PulseApp.state
        backupsState.filters.guestType = PulseApp.state.get('backupsFilterGuestType') || 'all';
        backupsState.filters.healthStatus = PulseApp.state.get('backupsFilterHealth') || 'all';
        backupsState.filters.backupType = PulseApp.state.get('backupsFilterBackupType') || 'all';
        backupsState.filters.showFailures = PulseApp.state.get('backupsFilterFailures') || false;
        backupsState.filters.searchTerm = PulseApp.state.get('backupsSearchTerm') || '';
        backupsState.filters.groupByNode = PulseApp.state.get('groupByNode') !== false;
        backupsState.filters.namespace = PulseApp.state.get('backupsFilterNamespace') || 'all';
        backupsState.filters.pbsInstance = PulseApp.state.get('backupsFilterPbsInstance') || 'all';
        
        // Update UI to match loaded filters
        _updateFilterUI();
    }
    
    // Update UI elements to match current filter state
    function _updateFilterUI() {
        // Uncheck all radios in each group first, then check the correct one
        
        // Guest type radio - handle the special case for container (lxc -> ct)
        document.querySelectorAll('input[name="backups-type-filter"]').forEach(r => r.checked = false);
        const guestTypeValue = backupsState.filters.guestType === 'lxc' ? 'ct' : backupsState.filters.guestType;
        const guestTypeRadio = document.getElementById(`backups-filter-type-${guestTypeValue}`);
        if (guestTypeRadio) guestTypeRadio.checked = true;
        
        // Health status radio
        document.querySelectorAll('input[name="backups-status-filter"]').forEach(r => r.checked = false);
        const healthRadio = document.getElementById(`backups-filter-status-${backupsState.filters.healthStatus}`);
        if (healthRadio) healthRadio.checked = true;
        
        // Backup type radio
        document.querySelectorAll('input[name="backups-backup-filter"]').forEach(r => r.checked = false);
        const backupTypeRadio = document.getElementById(`backups-filter-backup-${backupsState.filters.backupType}`);
        if (backupTypeRadio) backupTypeRadio.checked = true;
        
        // Group by node radio
        document.querySelectorAll('input[name="backups-group-filter"]').forEach(r => r.checked = false);
        const groupRadio = document.getElementById(backupsState.filters.groupByNode ? 'backups-group-grouped' : 'backups-group-list');
        if (groupRadio) groupRadio.checked = true;
        
        // Failures checkbox
        const failuresCheckbox = document.getElementById('backups-filter-failures');
        if (failuresCheckbox) failuresCheckbox.checked = backupsState.filters.showFailures;
        
        // Search input
        if (domElements.searchInput) domElements.searchInput.value = backupsState.filters.searchTerm;
        
        // Dropdowns
        if (domElements.namespaceFilter) domElements.namespaceFilter.value = backupsState.filters.namespace;
        if (domElements.pbsInstanceFilter) domElements.pbsInstanceFilter.value = backupsState.filters.pbsInstance;
    }
    
    // Handle filter changes
    function handleFilterChange(filterType, value) {
        // Update state
        backupsState.filters[filterType] = value;
        
        // Save to PulseApp.state
        switch(filterType) {
            case 'guestType':
                PulseApp.state.set('backupsFilterGuestType', value);
                break;
            case 'healthStatus':
                PulseApp.state.set('backupsFilterHealth', value);
                break;
            case 'backupType':
                PulseApp.state.set('backupsFilterBackupType', value);
                break;
            case 'showFailures':
                PulseApp.state.set('backupsFilterFailures', value);
                break;
            case 'searchTerm':
                PulseApp.state.set('backupsSearchTerm', value);
                break;
            case 'groupByNode':
                PulseApp.state.set('groupByNode', value);
                break;
            case 'namespace':
                PulseApp.state.set('backupsFilterNamespace', value);
                break;
            case 'pbsInstance':
                PulseApp.state.set('backupsFilterPbsInstance', value);
                break;
        }
        
        // Save filter state
        PulseApp.state.saveFilterState();
        
        // Apply filters immediately to existing data
        if (backupsState.data.raw) {
            backupsState.data.filtered = applyFilters(backupsState.data.raw.backupStatusByGuest);
            // Re-enrich with snapshot times
            enrichGuestDataWithSnapshotTimes(backupsState.data.filtered, backupsState.data.raw);
            renderTable(backupsState.data.filtered);
        }
        
        // Update reset button state
        updateResetButtonState();
        
        // Debounce API fetch for search, otherwise fetch immediately
        if (filterType === 'searchTerm') {
            clearTimeout(fetchDebounceTimer);
            fetchDebounceTimer = setTimeout(() => {
                fetchBackupData();
            }, 300);
        } else {
            // For other filters, fetch immediately
            fetchBackupData();
        }
    }
    
    // Sort data based on column and direction
    function sortData(data, column, direction) {
        if (!data || !column) return data;
        
        return [...data].sort((a, b) => {
            let aVal = a[column];
            let bVal = b[column];
            
            // Handle special cases
            if (column === 'guestId') {
                // Sort IDs numerically
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            } else if (column === 'latestBackupTime') {
                // Sort by timestamp (already numeric)
                aVal = aVal || 0;
                bVal = bVal || 0;
            } else if (column === 'guestName' || column === 'node') {
                // Case-insensitive string sort
                aVal = (aVal || '').toLowerCase();
                bVal = (bVal || '').toLowerCase();
            } else if (typeof aVal === 'string') {
                // Generic string sort
                aVal = aVal.toLowerCase();
                bVal = bVal.toLowerCase();
            }
            
            // Compare values
            if (aVal < bVal) return direction === 'asc' ? -1 : 1;
            if (aVal > bVal) return direction === 'asc' ? 1 : -1;
            return 0;
        });
    }
    
    // Enrich guest data with backup times from API data
    function enrichGuestDataWithSnapshotTimes(filteredGuests, apiData) {
        if (!filteredGuests || !apiData) return;
        
        // Build maps for each backup type
        const snapshotTimeMap = new Map();
        const pveBackupTimeMap = new Map();
        const pbsBackupTimeMap = new Map();
        
        // Process VM snapshots
        if (apiData.vmSnapshots) {
            apiData.vmSnapshots.forEach(snapshot => {
                const vmid = String(snapshot.vmid);
                const snapTime = snapshot.snaptime || 0;
                
                // Keep track of the latest snapshot time for each VM
                const currentLatest = snapshotTimeMap.get(vmid) || 0;
                if (snapTime > currentLatest) {
                    snapshotTimeMap.set(vmid, snapTime);
                }
            });
        }
        
        // Process PVE backups
        if (apiData.pveBackups) {
            apiData.pveBackups.forEach(backup => {
                const vmid = String(backup.vmid);
                const backupTime = backup.ctime || 0;
                
                // Keep track of the latest PVE backup time for each VM
                const currentLatest = pveBackupTimeMap.get(vmid) || 0;
                if (backupTime > currentLatest) {
                    pveBackupTimeMap.set(vmid, backupTime);
                }
            });
        }
        
        // Process PBS snapshots
        if (apiData.pbsSnapshots) {
            apiData.pbsSnapshots.forEach(snapshot => {
                const vmid = String(snapshot['backup-id']);
                const backupTime = snapshot['backup-time'] || 0;
                
                // Keep track of the latest PBS backup time for each VM
                const currentLatest = pbsBackupTimeMap.get(vmid) || 0;
                if (backupTime > currentLatest) {
                    pbsBackupTimeMap.set(vmid, backupTime);
                }
            });
        }
        
        // Add backup times to guest objects
        filteredGuests.forEach(guest => {
            const vmid = String(guest.guestId);
            
            const latestSnapTime = snapshotTimeMap.get(vmid);
            if (latestSnapTime) {
                guest.latestSnapshotTime = latestSnapTime;
            }
            
            const latestPveTime = pveBackupTimeMap.get(vmid);
            if (latestPveTime) {
                guest.lastPveBackupTime = latestPveTime;
            }
            
            const latestPbsTime = pbsBackupTimeMap.get(vmid);
            if (latestPbsTime) {
                guest.lastPbsBackupTime = latestPbsTime;
            }
        });
    }
    
    // Apply all filters to the data
    function applyFilters(rawData) {
        if (!rawData || !Array.isArray(rawData)) return [];
        
        return rawData.filter(guest => {
            // Guest type filter
            if (backupsState.filters.guestType !== 'all') {
                const guestType = guest.guestType?.toUpperCase();
                if (backupsState.filters.guestType === 'vm' && guestType !== 'VM') return false;
                if (backupsState.filters.guestType === 'lxc' && guestType !== 'LXC') return false;
            }
            
            // Health status filter
            if (backupsState.filters.healthStatus !== 'all') {
                const healthMatch = 
                    (backupsState.filters.healthStatus === 'ok' && guest.backupHealthStatus === 'ok') ||
                    (backupsState.filters.healthStatus === 'stale' && guest.backupHealthStatus === 'stale') ||
                    (backupsState.filters.healthStatus === 'warning' && (guest.backupHealthStatus === 'old' || guest.backupHealthStatus === 'failed')) ||
                    (backupsState.filters.healthStatus === 'none' && guest.backupHealthStatus === 'none');
                if (!healthMatch) return false;
            }
            
            // Backup type filter
            if (backupsState.filters.backupType !== 'all') {
                const hasBackupType = 
                    (backupsState.filters.backupType === 'pbs' && guest.pbsBackups > 0) ||
                    (backupsState.filters.backupType === 'pve' && guest.pveBackups > 0) ||
                    (backupsState.filters.backupType === 'snapshots' && guest.snapshotCount > 0);
                if (!hasBackupType) return false;
            }
            
            // Failures filter
            if (backupsState.filters.showFailures && guest.recentFailures === 0) {
                return false;
            }
            
            // Search filter
            if (backupsState.filters.searchTerm) {
                const searchTerms = backupsState.filters.searchTerm.toLowerCase().split(',').map(t => t.trim()).filter(t => t);
                if (searchTerms.length > 0) {
                    const matchesSearch = searchTerms.some(term =>
                        guest.guestName?.toLowerCase().includes(term) ||
                        guest.guestId?.toString().includes(term) ||
                        guest.node?.toLowerCase().includes(term)
                    );
                    if (!matchesSearch) return false;
                }
            }
            
            return true;
        });
    }
    
    // Fetch backup data from API
    async function fetchBackupData() {
        try {
            // Build query parameters
            const params = new URLSearchParams();
            if (backupsState.filters.namespace !== 'all') {
                params.append('namespace', backupsState.filters.namespace);
            }
            if (backupsState.filters.pbsInstance !== 'all') {
                params.append('pbsInstance', backupsState.filters.pbsInstance);
            }
            
            const response = await fetch(`/api/backup-data?${params}`);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const result = await response.json();
            
            if (result.success && result.data) {
                backupsState.data.raw = result.data;
                backupsState.data.lastFetch = Date.now();
                backupsState.data.error = null;
                
                // Apply filters and render
                backupsState.data.filtered = applyFilters(result.data.backupStatusByGuest);
                // Enrich filtered data with snapshot times
                enrichGuestDataWithSnapshotTimes(backupsState.data.filtered, result.data);
                renderTable(backupsState.data.filtered);
                
                // Update visualization if needed
                updateVisualization(result.data);
                
                // Update filter options
                updateFilterOptions(result.data);
            } else {
                throw new Error('Invalid API response');
            }
        } catch (error) {
            console.error('Error fetching backup data:', error);
            backupsState.data.error = error;
            showError(error.message);
        }
    }
    
    // Render the table
    function renderTable(filteredData) {
        if (!domElements.tableBody) {
            console.error('No table body element found');
            return;
        }
        
        // Apply sorting if active
        const sortState = PulseApp.state.getSortState('backups');
        if (sortState && sortState.column) {
            filteredData = sortData(filteredData, sortState.column, sortState.direction);
        }
        
        // Hide loading, show table
        if (domElements.loadingMsg) domElements.loadingMsg.classList.add('hidden');
        if (domElements.tableContainer) domElements.tableContainer.classList.remove('hidden');
        
        if (!filteredData || filteredData.length === 0) {
            // Show no data message
            if (domElements.tableContainer) domElements.tableContainer.classList.add('hidden');
            if (domElements.noDataMsg) {
                domElements.noDataMsg.textContent = getNoDataMessage();
                domElements.noDataMsg.classList.remove('hidden');
            }
            updateStatusText(0);
            return;
        }
        
        // Hide no data message
        if (domElements.noDataMsg) domElements.noDataMsg.classList.add('hidden');
        
        // Create fragment for performance
        const fragment = document.createDocumentFragment();
        
        if (backupsState.filters.groupByNode) {
            // Group by node
            const nodeGroups = {};
            filteredData.forEach(guest => {
                const node = guest.node || 'Unknown Node';
                if (!nodeGroups[node]) nodeGroups[node] = [];
                nodeGroups[node].push(guest);
            });
            
            // Render each node group
            Object.keys(nodeGroups).sort().forEach(node => {
                // Create node header
                const headerRow = document.createElement('tr');
                headerRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
                headerRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(node, 11, 'td');
                fragment.appendChild(headerRow);
                
                // Render guests in this node
                nodeGroups[node].forEach(guest => {
                    const row = createGuestRow(guest);
                    if (row) fragment.appendChild(row);
                });
            });
        } else {
            // Flat list
            filteredData.forEach(guest => {
                const row = createGuestRow(guest);
                if (row) fragment.appendChild(row);
            });
        }
        
        // Update DOM in one operation
        domElements.tableBody.innerHTML = '';
        domElements.tableBody.appendChild(fragment);
        
        // Update status text
        updateStatusText(filteredData.length);
        
        // Update PBS instances summary
        updatePBSInstancesSummary();
        
        // Update visualization to match filtered data
        if (backupsState.data.raw) {
            updateVisualization(backupsState.data.raw);
        }
    }
    
    // Get contextual latest backup text based on active filter
    function getContextualLatestBackupText(guest) {
        const backupTypeFilter = backupsState.filters.backupType;
        const now = Date.now() / 1000; // Current time in seconds
        
        // Helper to format relative time
        const formatRelativeTime = (timestamp) => {
            if (!timestamp || timestamp === 0) return 'Never';
            const deltaSeconds = now - timestamp;
            if (deltaSeconds < 60) return 'Just now';
            if (deltaSeconds < 3600) return `${Math.floor(deltaSeconds / 60)} minutes ago`;
            if (deltaSeconds < 86400) return `${Math.floor(deltaSeconds / 3600)} hours ago`;
            return `${Math.floor(deltaSeconds / 86400)} days ago`;
        };
        
        // When filtering by specific backup type, show that type's latest time
        if (backupTypeFilter === 'snapshots' && guest.snapshotCount > 0) {
            // Use snapshot time if available
            if (guest.latestSnapshotTime) {
                return formatRelativeTime(guest.latestSnapshotTime);
            }
            // Fallback if we have snapshots but no time data
            return guest.snapshotCount > 0 ? 'Has snapshots' : 'No snapshots';
        } else if (backupTypeFilter === 'pbs' && guest.pbsBackups > 0) {
            // Show PBS backup time
            if (guest.lastPbsBackupTime) {
                return formatRelativeTime(guest.lastPbsBackupTime);
            }
            return guest.pbsBackups > 0 ? 'Has PBS backups' : 'No PBS backups';
        } else if (backupTypeFilter === 'pve' && guest.pveBackups > 0) {
            // Show PVE backup time
            if (guest.lastPveBackupTime) {
                return formatRelativeTime(guest.lastPveBackupTime);
            }
            return guest.pveBackups > 0 ? 'Has PVE backups' : 'No PVE backups';
        } else {
            // For 'all' filter or when no specific data, use the provided text
            return guest.lastBackupText || 'Never';
        }
    }
    
    // Create a guest row
    function createGuestRow(guest) {
        const row = document.createElement('tr');
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700 border-b border-gray-200 dark:border-gray-700 last:border-b-0';
        row.setAttribute('data-guest-id', guest.guestId);
        row.setAttribute('data-node', guest.node || '');
        
        // Create cells properly to avoid XSS
        const cells = [
            // Guest Name
            createTextCell(guest.guestName || 'Unknown', 'px-3 py-1 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100'),
            
            // ID
            createTextCell(guest.guestId, 'px-3 py-1 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100'),
            
            // Type
            createCell(() => {
                const icon = document.createElement('i');
                icon.className = `${guest.guestType === 'VM' ? 'fas fa-desktop' : 'fas fa-cube'} ${guest.guestType === 'VM' ? 'text-blue-500' : 'text-green-500'}`;
                icon.title = guest.guestType;
                return icon;
            }, 'px-3 py-1 whitespace-nowrap text-sm text-center'),
            
            // Node
            createTextCell(guest.node || 'Unknown', 'px-3 py-1 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100'),
            
            // PBS Namespace
            createTextCell(guest.pbsNamespaceText || guest.pbsNamespace || '-', 'px-3 py-1 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100'),
            
            // Latest Backup - now contextual based on filter
            createTextCell(getContextualLatestBackupText(guest), 'px-3 py-1 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100'),
            
            // Snapshots
            createCell(() => {
                if (guest.snapshotCount > 0) {
                    const span = document.createElement('span');
                    span.className = 'text-yellow-600 dark:text-yellow-400';
                    span.textContent = guest.snapshotCount;
                    return span;
                }
                return document.createTextNode('-');
            }, 'px-3 py-1 whitespace-nowrap text-sm text-center'),
            
            // PVE Backups
            createCell(() => {
                if (guest.pveBackups > 0) {
                    const span = document.createElement('span');
                    span.className = 'text-orange-600 dark:text-orange-400';
                    span.textContent = guest.pveBackups;
                    return span;
                }
                return document.createTextNode('-');
            }, 'px-3 py-1 whitespace-nowrap text-sm text-center'),
            
            // PBS Backups
            createCell(() => {
                if (guest.pbsBackups > 0) {
                    const span = document.createElement('span');
                    span.className = 'text-purple-600 dark:text-purple-400';
                    span.textContent = guest.pbsBackups;
                    return span;
                }
                return document.createTextNode('-');
            }, 'px-3 py-1 whitespace-nowrap text-sm text-center'),
            
            // Recent Failures
            createCell(() => {
                const span = document.createElement('span');
                span.innerHTML = getRecentFailuresBadge(guest.recentFailures);
                return span.firstChild || document.createTextNode('-');
            }, 'px-3 py-1 whitespace-nowrap text-sm')
        ];
        
        cells.forEach(cell => row.appendChild(cell));
        return row;
    }
    
    // Helper to create text cell
    function createTextCell(text, className) {
        const td = document.createElement('td');
        td.className = className;
        td.textContent = text;
        return td;
    }
    
    // Helper to create cell with custom content
    function createCell(contentFn, className) {
        const td = document.createElement('td');
        td.className = className;
        const content = contentFn();
        if (content) td.appendChild(content);
        return td;
    }
    
    // Get health status badge HTML
    function getHealthStatusBadge(status) {
        const badges = {
            'ok': '<span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">OK</span>',
            'stale': '<span class="px-2 py-1 text-xs rounded-full bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">Stale</span>',
            'old': '<span class="px-2 py-1 text-xs rounded-full bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200">Old</span>',
            'failed': '<span class="px-2 py-1 text-xs rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">Failed</span>',
            'none': '<span class="px-2 py-1 text-xs rounded-full bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300">None</span>'
        };
        return badges[status] || badges['none'];
    }
    
    // Get recent failures badge
    function getRecentFailuresBadge(failures) {
        if (failures === 0) return '-';
        return `<span class="px-2 py-1 text-xs rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">${failures}</span>`;
    }
    
    // Update status text
    function updateStatusText(count) {
        if (!domElements.statusText) return;
        
        const hasFilters = 
            backupsState.filters.guestType !== 'all' ||
            backupsState.filters.healthStatus !== 'all' ||
            backupsState.filters.backupType !== 'all' ||
            backupsState.filters.showFailures ||
            backupsState.filters.searchTerm ||
            backupsState.filters.namespace !== 'all' ||
            backupsState.filters.pbsInstance !== 'all';
        
        const totalCount = backupsState.data.raw?.backupStatusByGuest?.length || 0;
        
        if (hasFilters && totalCount > 0) {
            domElements.statusText.textContent = `Showing ${count} of ${totalCount} guests`;
        } else {
            domElements.statusText.textContent = `${count} guest${count !== 1 ? 's' : ''}`;
        }
    }
    
    // Get appropriate no data message
    function getNoDataMessage() {
        if (backupsState.data.error) {
            return 'Error loading backup data. Please refresh.';
        }
        
        const hasFilters = 
            backupsState.filters.guestType !== 'all' ||
            backupsState.filters.healthStatus !== 'all' ||
            backupsState.filters.backupType !== 'all' ||
            backupsState.filters.showFailures ||
            backupsState.filters.searchTerm;
        
        if (hasFilters) {
            return 'No backups found matching your filters.';
        }
        
        return 'No backup data available.';
    }
    
    // Show error message
    function showError(message) {
        if (domElements.loadingMsg) domElements.loadingMsg.classList.add('hidden');
        if (domElements.tableContainer) domElements.tableContainer.classList.add('hidden');
        if (domElements.noDataMsg) {
            domElements.noDataMsg.textContent = message || 'An error occurred loading backup data.';
            domElements.noDataMsg.classList.remove('hidden');
        }
    }
    
    // Show loading message
    function showLoading() {
        if (domElements.loadingMsg) domElements.loadingMsg.classList.remove('hidden');
        if (domElements.tableContainer) domElements.tableContainer.classList.add('hidden');
        if (domElements.noDataMsg) domElements.noDataMsg.classList.add('hidden');
    }
    
    // Reset all filters
    function resetFilters() {
        // Reset state
        backupsState.filters = {
            guestType: 'all',
            healthStatus: 'all',
            backupType: 'all',
            showFailures: false,
            searchTerm: '',
            groupByNode: true,
            namespace: 'all',
            pbsInstance: 'all'
        };
        
        // Update UI
        _updateFilterUI();
        
        // Save state
        PulseApp.state.set('backupsFilterGuestType', 'all');
        PulseApp.state.set('backupsFilterHealth', 'all');
        PulseApp.state.set('backupsFilterBackupType', 'all');
        PulseApp.state.set('backupsFilterFailures', false);
        PulseApp.state.set('backupsSearchTerm', '');
        PulseApp.state.set('groupByNode', true);
        PulseApp.state.set('backupsFilterNamespace', 'all');
        PulseApp.state.set('backupsFilterPbsInstance', 'all');
        PulseApp.state.saveFilterState();
        
        // Update table
        if (backupsState.data.raw) {
            backupsState.data.filtered = applyFilters(backupsState.data.raw.backupStatusByGuest);
            // Re-enrich with snapshot times
            enrichGuestDataWithSnapshotTimes(backupsState.data.filtered, backupsState.data.raw);
            renderTable(backupsState.data.filtered);
        }
        
        // Fetch fresh data
        fetchBackupData();
        
        // Update reset button
        updateResetButtonState();
    }
    
    // Update reset button state
    function updateResetButtonState() {
        if (!domElements.resetButton) return;
        
        const hasActiveFilters = 
            backupsState.filters.guestType !== 'all' ||
            backupsState.filters.healthStatus !== 'all' ||
            backupsState.filters.backupType !== 'all' ||
            backupsState.filters.showFailures ||
            backupsState.filters.searchTerm ||
            backupsState.filters.namespace !== 'all' ||
            backupsState.filters.pbsInstance !== 'all' ||
            !backupsState.filters.groupByNode;
        
        domElements.resetButton.disabled = !hasActiveFilters;
        
        if (hasActiveFilters) {
            domElements.resetButton.classList.remove('opacity-50', 'cursor-not-allowed');
        } else {
            domElements.resetButton.classList.add('opacity-50', 'cursor-not-allowed');
        }
    }
    
    // Update PBS instances summary
    function updatePBSInstancesSummary() {
        const summaryElement = document.getElementById('pbs-instances-summary');
        if (!summaryElement || !backupsState.data.filtered) return;
        
        // Calculate stats from filtered data
        const stats = {
            totalGuests: backupsState.data.filtered.length,
            totalPBSBackups: 0,
            totalPVEBackups: 0,
            totalSnapshots: 0,
            pbsInstances: new Map()
        };
        
        backupsState.data.filtered.forEach(guest => {
            // Only count backups based on active filter
            const backupTypeFilter = backupsState.filters.backupType;
            
            if (backupTypeFilter === 'all' || backupTypeFilter === 'pbs') {
                stats.totalPBSBackups += guest.pbsBackups || 0;
            }
            if (backupTypeFilter === 'all' || backupTypeFilter === 'pve') {
                stats.totalPVEBackups += guest.pveBackups || 0;
            }
            if (backupTypeFilter === 'all' || backupTypeFilter === 'snapshots') {
                stats.totalSnapshots += guest.snapshotCount || 0;
            }
            
            // Count PBS backups by instance (if we have that data and PBS is not filtered out)
            if ((backupTypeFilter === 'all' || backupTypeFilter === 'pbs') && 
                guest._pbsBackups && Array.isArray(guest._pbsBackups)) {
                guest._pbsBackups.forEach(backup => {
                    const instance = backup.pbsInstance || 'Unknown';
                    stats.pbsInstances.set(instance, (stats.pbsInstances.get(instance) || 0) + 1);
                });
            }
        });
        
        // Clear the summary - information is already shown in table and summary card
        summaryElement.innerHTML = '';
    }
    
    // Update visualization section (calendar, etc)
    function updateVisualization(apiData) {
        if (!domElements.visualizationSection) {
            return;
        }
        
        // Always show visualization section
        domElements.visualizationSection.classList.remove('hidden');
        
        // Update calendar if available
        if (domElements.calendarContainer && PulseApp.ui.calendarHeatmap) {
            const filteredData = backupsState.data.filtered || [];
            const guestIds = filteredData.map(guest => {
                const nodeIdentifier = guest.node || guest.endpointId || '';
                return nodeIdentifier ? `${guest.guestId}-${nodeIdentifier}` : guest.guestId.toString();
            });
            
            // Filter backup data to only include backups for filtered guests
            const filteredGuestIds = new Set(filteredData.map(g => String(g.guestId)));
            
            const backupData = {
                pbsSnapshots: (apiData.pbsSnapshots || []).filter(snap => 
                    filteredGuestIds.has(String(snap['backup-id']))
                ),
                pveBackups: (apiData.pveBackups || []).filter(backup => 
                    filteredGuestIds.has(String(backup.vmid))
                ),
                vmSnapshots: (apiData.vmSnapshots || []).filter(snapshot => 
                    filteredGuestIds.has(String(snapshot.vmid))
                ),
                backupTasks: (apiData.backupTasks || []).filter(task => 
                    task.vmid && filteredGuestIds.has(String(task.vmid))
                ),
                guestToValidatedSnapshots: new Map(),
                // Pass the filtered guest data to calendar for proper name lookup
                guests: backupsState.data.filtered
            };
            
            const calendarHeatmap = PulseApp.ui.calendarHeatmap.createCalendarHeatmap(
                backupData,
                null,
                guestIds,
                (selectedDateData, instant) => {
                    // Update detail card when a date is selected
                    if (domElements.detailCardContainer && PulseApp.ui.backupDetailCard) {
                        const detailCard = domElements.detailCardContainer.querySelector('.bg-white.dark\\:bg-gray-800');
                        if (detailCard) {
                            if (selectedDateData) {
                                // Show backups for selected date
                                PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, selectedDateData, instant || true);
                            } else {
                                // Go back to showing summary for all filtered backups
                                const filteredData = backupsState.data.filtered || [];
                                const summaryData = {
                                    backups: filteredData,
                                    stats: {
                                        totalGuests: filteredData.length,
                                        healthyGuests: filteredData.filter(g => g.backupHealthStatus === 'ok').length
                                    },
                                    filterInfo: {
                                        guestType: backupsState.filters.guestType,
                                        backupType: backupsState.filters.backupType,
                                        healthStatus: backupsState.filters.healthStatus,
                                        namespace: backupsState.filters.namespace,
                                        pbsInstance: backupsState.filters.pbsInstance,
                                        search: backupsState.filters.searchTerm
                                    },
                                    isMultiDate: true,
                                    // Include snapshot data for time calculations
                                    vmSnapshots: apiData.vmSnapshots || []
                                };
                                PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, summaryData, true);
                            }
                        }
                    }
                },
                false
            );
            
            domElements.calendarContainer.innerHTML = '';
            domElements.calendarContainer.appendChild(calendarHeatmap);
        }
        
        // Initialize detail card if needed
        if (domElements.detailCardContainer && PulseApp.ui.backupDetailCard) {
            let detailCard = domElements.detailCardContainer.querySelector('.bg-white.dark\\:bg-gray-800');
            if (!detailCard) {
                // Create the detail card with initial data
                detailCard = PulseApp.ui.backupDetailCard.createBackupDetailCard(null);
                // Remove loading animation
                while (domElements.detailCardContainer.firstChild) {
                    domElements.detailCardContainer.removeChild(domElements.detailCardContainer.firstChild);
                }
                domElements.detailCardContainer.appendChild(detailCard);
                
                // Reconstruct backup data for detail card
                const filteredData = backupsState.data.filtered || [];
                const filteredGuestIds = new Set(filteredData.map(g => String(g.guestId)));
                const detailBackupData = {
                    pbsSnapshots: (apiData.pbsSnapshots || []).filter(snap => 
                        filteredGuestIds.has(String(snap['backup-id']))
                    ),
                    pveBackups: (apiData.pveBackups || []).filter(backup => 
                        filteredGuestIds.has(String(backup.vmid))
                    ),
                    vmSnapshots: (apiData.vmSnapshots || []).filter(snapshot => 
                        filteredGuestIds.has(String(snapshot.vmid))
                    ),
                    backupTasks: (apiData.backupTasks || []).filter(task => 
                        task.vmid && filteredGuestIds.has(String(task.vmid))
                    ),
                    guestToValidatedSnapshots: new Map()
                };
                
                // Show summary for all filtered backups
                const summaryData = {
                    backups: filteredData,
                    stats: {
                        totalGuests: filteredData.length,
                        healthyGuests: filteredData.filter(g => g.backupHealthStatus === 'ok').length
                    },
                    filterInfo: {
                        guestType: backupsState.filters.guestType,
                        backupType: backupsState.filters.backupType,
                        healthStatus: backupsState.filters.healthStatus,
                        namespace: backupsState.filters.namespace,
                        pbsInstance: backupsState.filters.pbsInstance,
                        search: backupsState.filters.searchTerm
                    },
                    isMultiDate: true,
                    // Include snapshot data for time calculations
                    vmSnapshots: apiData.vmSnapshots || []
                };
                PulseApp.ui.backupDetailCard.updateBackupDetailCard(detailCard, summaryData);
            }
        }
    }
    
    // Update filter options based on available data
    function updateFilterOptions(apiData) {
        // Update namespace dropdown
        if (domElements.namespaceFilter && apiData.availableNamespaces) {
            const currentValue = domElements.namespaceFilter.value;
            domElements.namespaceFilter.innerHTML = '<option value="all">All Namespaces</option>';
            
            apiData.availableNamespaces.forEach(ns => {
                const option = document.createElement('option');
                option.value = ns;
                option.textContent = ns === 'root' ? 'Root Namespace' : ns;
                domElements.namespaceFilter.appendChild(option);
            });
            
            domElements.namespaceFilter.value = currentValue;
        }
        
        // Update PBS instance dropdown
        if (domElements.pbsInstanceFilter && apiData.availablePbsInstances) {
            const currentValue = domElements.pbsInstanceFilter.value;
            domElements.pbsInstanceFilter.innerHTML = '<option value="all">All PBS Instances</option>';
            
            apiData.availablePbsInstances.forEach(instance => {
                const option = document.createElement('option');
                option.value = instance;
                option.textContent = instance;
                domElements.pbsInstanceFilter.appendChild(option);
            });
            
            domElements.pbsInstanceFilter.value = currentValue;
        }
    }
    
    // Main update function (called from outside)
    async function updateBackupsTab(isUserAction = false, sortOnly = false) {
        // If sort only, just re-render existing data
        if (sortOnly && backupsState.data.filtered) {
            renderTable(backupsState.data.filtered);
            return;
        }
        
        // If not user action and we have recent data, skip update
        if (!isUserAction && backupsState.data.lastFetch && (Date.now() - backupsState.data.lastFetch < 5000)) {
            return;
        }
        
        // Show loading only if no data
        if (!backupsState.data.raw) {
            showLoading();
        }
        
        // Fetch and render
        await fetchBackupData();
    }
    
    // Public API
    return {
        init,
        updateBackupsTab,
        resetBackupsView: resetFilters
    };
})();