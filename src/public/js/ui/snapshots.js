PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.snapshots = (() => {
    let isInitialized = false;
    let currentFilters = {
        searchTerm: '',
        guestType: 'all' // 'all', 'vm', 'lxc'
    };
    let currentSort = {
        field: 'snaptime',
        ascending: false
    };
    let snapshotsData = [];

    function init() {
        if (isInitialized) return;
        isInitialized = true;
        
        updateSnapshotsInfo();
    }

    function updateSnapshotsInfo() {
        const container = document.getElementById('snapshots-content');
        if (!container) return;

        // Only show loading state on initial load
        if (!isInitialized || snapshotsData.length === 0) {
            container.innerHTML = `
                <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                    Loading snapshots...
                </div>
            `;
        }

        // Fetch snapshots data
        fetch('/api/snapshots')
            .then(response => response.json())
            .then(data => {
                const newData = data.snapshots || [];
                
                // Check if data has actually changed
                const hasChanged = JSON.stringify(newData) !== JSON.stringify(snapshotsData);
                
                if (hasChanged) {
                    snapshotsData = newData;
                    
                    // Only render full UI on first load
                    if (container.querySelector('.overflow-x-auto')) {
                        // Update only the table body
                        const tbody = container.querySelector('tbody');
                        if (tbody) {
                            tbody.innerHTML = renderSnapshotRows();
                        }
                        
                        // Update summary cards
                        const summary = calculateSummary();
                        const summaryContainer = container.querySelector('.grid');
                        if (summaryContainer) {
                            summaryContainer.innerHTML = renderSummaryCards(summary);
                        }
                    } else {
                        // First load - render full UI
                        renderSnapshotsUI();
                    }
                }
            })
            .catch(error => {
                console.error('Error fetching snapshots:', error);
                // Only show error if we don't have data already
                if (snapshotsData.length === 0) {
                    container.innerHTML = `
                        <div class="p-8 text-center">
                            <div class="text-red-500 dark:text-red-400">
                                Failed to load snapshots data: ${error.message}
                            </div>
                            <button onclick="PulseApp.ui.snapshots.updateSnapshotsInfo()" class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
                                Retry
                            </button>
                        </div>
                    `;
                }
            });
    }

    function renderSnapshotsUI() {
        const container = document.getElementById('snapshots-content');
        if (!container) return;

        const summary = calculateSummary();

        const nodeCount = Object.keys(summary.nodeStats).length;
        const gridCols = nodeCount === 1 ? 'grid-cols-1' : 
                        nodeCount === 2 ? 'grid-cols-1 sm:grid-cols-2' : 
                        nodeCount === 3 ? 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3' : 
                        'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4';
        
        container.innerHTML = `
            <!-- Snapshot Summary Cards -->
            <div class="mb-3">
                <div class="grid ${gridCols} gap-3">
                    ${renderSummaryCards(summary)}
                </div>
            </div>

            <!-- Filters -->
            <div class="mb-3 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-sm">
                <div class="flex flex-row flex-wrap items-center gap-2 sm:gap-3">
                    <div class="filter-controls-wrapper flex items-center gap-2 flex-1 min-w-[180px] sm:min-w-[240px]">
                        <input type="search" id="snapshot-search" placeholder="Search snapshots..." 
                            value="${currentFilters.searchTerm}"
                            class="flex-1 p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                        <button id="reset-snapshots-button" title="Reset Filters & Sort (Esc)" class="flex items-center justify-center p-1 h-7 w-7 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none transition-colors flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>
                        </button>
                    </div>
                    
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Type:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="type-all" name="guest-type" value="all" class="hidden peer/all" ${currentFilters.guestType === 'all' ? 'checked' : ''}>
                            <label for="type-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="type-vm" name="guest-type" value="vm" class="hidden peer/vm" ${currentFilters.guestType === 'vm' ? 'checked' : ''}>
                            <label for="type-vm" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/vm:bg-gray-100 dark:peer-checked/vm:bg-gray-700 peer-checked/vm:text-blue-600 dark:peer-checked/vm:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">VM</label>
                            
                            <input type="radio" id="type-lxc" name="guest-type" value="lxc" class="hidden peer/lxc" ${currentFilters.guestType === 'lxc' ? 'checked' : ''}>
                            <label for="type-lxc" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/lxc:bg-gray-100 dark:peer-checked/lxc:bg-gray-700 peer-checked/lxc:text-blue-600 dark:peer-checked/lxc:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">LXC</label>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Snapshots Table -->
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
                <table class="w-full text-xs sm:text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-800">
                        <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                            <th class="p-1 px-2 whitespace-nowrap text-center">Status</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="vmid">VMID</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="name">Name</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="type">Type</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="node">Node</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="snapname">Snapshot</th>
                            <th class="p-1 px-2 whitespace-nowrap">Description</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${renderSnapshotRows()}
                    </tbody>
                </table>
            </div>
        `;

        // Setup event listeners
        setupEventListeners();
        updateResetButtonState();
        
        // Setup sortable headers
        const table = document.querySelector('#snapshots-content table');
        if (table) {
            table.id = 'snapshots-table';
            table.querySelectorAll('th.sortable').forEach(th => {
                th.style.cursor = 'pointer';
                th.addEventListener('click', () => {
                    const field = th.getAttribute('data-sort');
                    if (field) sortBy(field);
                });
            });
        }
    }

    function calculateSummary() {
        let totalCount = 0;
        const uniqueGuests = new Set();
        const nodeStats = {};
        const ageDistribution = { day: 0, week: 0, month: 0, older: 0 };
        let oldestSnapshot = null;
        let newestSnapshot = null;
        let totalSize = 0;
        const now = Date.now() / 1000;

        snapshotsData.forEach(snapshot => {
            totalCount++;
            uniqueGuests.add(`${snapshot.node}:${snapshot.vmid}`);
            
            // Node statistics
            if (!nodeStats[snapshot.node]) {
                nodeStats[snapshot.node] = {
                    count: 0,
                    vmCount: 0,
                    lxcCount: 0,
                    guests: new Set()
                };
            }
            nodeStats[snapshot.node].count++;
            nodeStats[snapshot.node].guests.add(snapshot.vmid);
            if (snapshot.type === 'qemu') {
                nodeStats[snapshot.node].vmCount++;
            } else {
                nodeStats[snapshot.node].lxcCount++;
            }
            
            // Age distribution
            const age = now - (snapshot.snaptime || 0);
            if (age < 86400) ageDistribution.day++;
            else if (age < 604800) ageDistribution.week++;
            else if (age < 2592000) ageDistribution.month++;
            else ageDistribution.older++;
            
            // Track oldest and newest
            if (!oldestSnapshot || (snapshot.snaptime && snapshot.snaptime < oldestSnapshot.snaptime)) {
                oldestSnapshot = snapshot;
            }
            if (!newestSnapshot || (snapshot.snaptime && snapshot.snaptime > newestSnapshot.snaptime)) {
                newestSnapshot = snapshot;
            }
            
            // Size (if available)
            if (snapshot.size) totalSize += snapshot.size;
        });

        // Convert node stats guests sets to counts
        Object.keys(nodeStats).forEach(node => {
            nodeStats[node].uniqueGuests = nodeStats[node].guests.size;
            delete nodeStats[node].guests;
        });

        return { 
            totalCount, 
            uniqueGuests: uniqueGuests.size,
            nodeStats,
            ageDistribution,
            oldestSnapshot,
            newestSnapshot,
            totalSize
        };
    }

    function renderSummaryCards(summary) {
        const nodeNames = Object.keys(summary.nodeStats).sort();
        
        if (nodeNames.length === 0) {
            return '';
        }
        
        return nodeNames.map(nodeName => {
            const node = summary.nodeStats[nodeName];
            const now = Date.now() / 1000;
            
            // Find newest and oldest snapshots for this node
            let nodeNewest = null;
            let nodeOldest = null;
            snapshotsData.forEach(snapshot => {
                if (snapshot.node === nodeName) {
                    if (!nodeNewest || snapshot.snaptime > nodeNewest.snaptime) {
                        nodeNewest = snapshot;
                    }
                    if (!nodeOldest || snapshot.snaptime < nodeOldest.snaptime) {
                        nodeOldest = snapshot;
                    }
                }
            });
            
            // Format age for newest
            let newestText = 'Never';
            let newestColorClass = 'text-gray-500 dark:text-gray-400';
            if (nodeNewest && nodeNewest.snaptime) {
                const age = now - nodeNewest.snaptime;
                if (age < 3600) {
                    newestText = Math.floor(age / 60) + 'm ago';
                    newestColorClass = 'text-green-600 dark:text-green-400';
                } else if (age < 86400) {
                    newestText = Math.floor(age / 3600) + 'h ago';
                    newestColorClass = age < 43200 ? 'text-green-600 dark:text-green-400' : 'text-blue-600 dark:text-blue-400';
                } else {
                    newestText = Math.floor(age / 86400) + 'd ago';
                    newestColorClass = age < 172800 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400';
                }
            }
            
            // Count critical/warning based on age
            let oldCount = 0;
            let veryOldCount = 0;
            snapshotsData.forEach(snapshot => {
                if (snapshot.node === nodeName && snapshot.snaptime) {
                    const age = now - snapshot.snaptime;
                    if (age > 2592000) veryOldCount++; // > 30 days
                    else if (age > 604800) oldCount++; // > 7 days
                }
            });
            
            return `
                <div class="bg-white dark:bg-gray-800 shadow-md rounded-lg p-3 border border-gray-200 dark:border-gray-700">
                    <div class="flex justify-between items-center mb-2">
                        <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">${nodeName}</h3>
                        <div class="flex items-center gap-3">
                            ${veryOldCount > 0 ? `<span class="text-xs font-medium text-red-600 dark:text-red-400" title="${veryOldCount} snapshots older than 30 days">● ${veryOldCount}</span>` : ''}
                            ${oldCount > 0 ? `<span class="text-xs font-medium text-yellow-600 dark:text-yellow-400" title="${oldCount} snapshots 7-30 days old">● ${oldCount}</span>` : ''}
                        </div>
                    </div>
                    <div class="space-y-1 text-sm">
                        <div class="flex justify-between">
                            <div class="flex gap-2">
                                <span class="text-gray-500 dark:text-gray-500">Total:</span>
                                <span class="font-semibold text-gray-800 dark:text-gray-200">${node.count}</span>
                            </div>
                            <div class="flex gap-2">
                                <span class="text-gray-500 dark:text-gray-500">Guests:</span>
                                <span class="font-semibold text-gray-800 dark:text-gray-200">${node.uniqueGuests}</span>
                            </div>
                        </div>
                        ${(node.vmCount > 0 || node.lxcCount > 0) ? `
                        <div class="flex justify-between">
                            ${node.vmCount > 0 ? `
                            <div class="flex gap-2">
                                <span class="text-gray-500 dark:text-gray-500">VMs:</span>
                                <span class="font-semibold text-blue-600 dark:text-blue-400">${node.vmCount}</span>
                            </div>` : '<div></div>'}
                            ${node.lxcCount > 0 ? `
                            <div class="flex gap-2">
                                <span class="text-gray-500 dark:text-gray-500">LXCs:</span>
                                <span class="font-semibold text-purple-600 dark:text-purple-400">${node.lxcCount}</span>
                            </div>` : '<div></div>'}
                        </div>` : ''}
                        <div class="flex gap-2 pt-1 border-t border-gray-200 dark:border-gray-700">
                            <span class="text-gray-500 dark:text-gray-500">Latest:</span>
                            <span class="font-semibold ${newestColorClass}">${newestText}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    }
    
    function renderSnapshotRows() {
        const filtered = filterSnapshots();
        const sorted = sortSnapshots(filtered);
        
        if (sorted.length === 0) {
            return `
                <tr>
                    <td colspan="7" class="px-3 py-8 text-center text-gray-500 dark:text-gray-400">
                        No snapshots found
                    </td>
                </tr>
            `;
        }

        // Group snapshots by date
        const groupedSnapshots = {};
        sorted.forEach(snapshot => {
            const dateKey = formatDateKey(snapshot.snaptime);
            if (!groupedSnapshots[dateKey]) {
                groupedSnapshots[dateKey] = [];
            }
            groupedSnapshots[dateKey].push(snapshot);
        });
        
        // Sort dates (newest first)
        const sortedDates = Object.keys(groupedSnapshots).sort().reverse();
        
        let html = '';
        sortedDates.forEach(dateKey => {
            const snapshots = groupedSnapshots[dateKey];
            const displayDate = formatDateDisplay(dateKey);
            
            // Add date header
            html += `
                <tr class="bg-gray-50 dark:bg-gray-700/50">
                    <td colspan="7" class="p-1 px-2 text-xs font-medium text-gray-600 dark:text-gray-400">
                        ${displayDate} (${snapshots.length} snapshot${snapshots.length > 1 ? 's' : ''})
                    </td>
                </tr>
            `;
            
            // Add snapshots for this date
            snapshots.forEach(snapshot => {
                const typeLabel = snapshot.type === 'qemu' ? 'VM' : 'LXC';
                
                html += `
                    <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                        <td class="p-1 px-2 whitespace-nowrap text-center">
                            <span class="inline-flex items-center justify-center w-4 h-4">
                                <svg class="w-3 h-3 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path>
                                </svg>
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap font-medium">${snapshot.vmid}</td>
                        <td class="p-1 px-2 max-w-[200px] truncate" title="${snapshot.name || ''}">
                            ${snapshot.name || '-'}
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap">
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${typeLabel === 'VM' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'}">
                                ${typeLabel}
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap">${snapshot.node}</td>
                        <td class="p-1 px-2 max-w-[150px] truncate" title="${snapshot.snapname || ''}">
                            ${snapshot.snapname || '-'}
                        </td>
                        <td class="p-1 px-2 max-w-[200px] truncate" title="${snapshot.description || ''}">
                            ${snapshot.description || '-'}
                        </td>
                    </tr>
                `;
            });
        });
        
        return html;
    }

    function filterSnapshots() {
        return snapshotsData.filter(snapshot => {
            // Guest type filter
            if (currentFilters.guestType !== 'all') {
                if (currentFilters.guestType === 'vm' && snapshot.type !== 'qemu') return false;
                if (currentFilters.guestType === 'lxc' && snapshot.type !== 'lxc') return false;
            }

            // Search filter
            if (currentFilters.searchTerm) {
                const search = currentFilters.searchTerm.toLowerCase();
                const nameMatch = snapshot.name?.toLowerCase().includes(search);
                const vmidMatch = snapshot.vmid?.toString().includes(search);
                const snapNameMatch = snapshot.snapname?.toLowerCase().includes(search);
                const nodeMatch = snapshot.node?.toLowerCase().includes(search);
                const descMatch = snapshot.description?.toLowerCase().includes(search);
                
                if (!nameMatch && !vmidMatch && !snapNameMatch && !nodeMatch && !descMatch) return false;
            }

            return true;
        });
    }

    function formatBytes(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    // Format date for grouping (YYYY-MM-DD)
    function formatDateKey(timestamp) {
        const date = new Date(timestamp * 1000);
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
    }
    
    // Format date for display
    function formatDateDisplay(dateKey) {
        const [year, month, day] = dateKey.split('-');
        const date = new Date(year, month - 1, day);
        
        // Check if it's today or yesterday
        const today = new Date();
        const yesterday = new Date(today);
        yesterday.setDate(yesterday.getDate() - 1);
        
        if (dateKey === formatDateKey(today.getTime() / 1000)) {
            return 'Today';
        } else if (dateKey === formatDateKey(yesterday.getTime() / 1000)) {
            return 'Yesterday';
        }
        
        // Otherwise return formatted date
        return date.toLocaleDateString(undefined, { 
            weekday: 'short',
            day: 'numeric',
            month: 'short',
            year: 'numeric'
        });
    }

    function getRelativeTime(timestamp) {
        if (!timestamp) return 'Unknown';
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        
        if (diff < 60) return 'Just\u00A0now';
        if (diff < 3600) return Math.floor(diff / 60) + 'm\u00A0ago';
        if (diff < 86400) return Math.floor(diff / 3600) + 'h\u00A0ago';
        if (diff < 604800) return Math.floor(diff / 86400) + 'd\u00A0ago';
        if (diff < 2592000) return Math.floor(diff / 604800) + 'w\u00A0ago';
        return Math.floor(diff / 2592000) + 'mo\u00A0ago';
    }

    function setupEventListeners() {
        // Search
        const searchInput = document.getElementById('snapshot-search');
        const resetButton = document.getElementById('reset-snapshots-button');
        
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                currentFilters.searchTerm = e.target.value;
                const filtered = filterSnapshots();
                const tbody = document.querySelector('#snapshots-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderSnapshotRows();
                }
                updateResetButtonState();
            });
            
            // ESC key handler
            searchInput.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    resetFiltersAndSort();
                }
            });
        }
        
        // Reset button
        if (resetButton) {
            resetButton.addEventListener('click', resetFiltersAndSort);
        }
        
        // Set up keyboard navigation for auto-focus search
        setupKeyboardNavigation();
        
        // Guest type radio buttons
        document.querySelectorAll('input[name="guest-type"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.guestType = e.target.value;
                const filtered = filterSnapshots();
                const tbody = document.querySelector('#snapshots-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderSnapshotRows();
                }
                updateResetButtonState();
            });
        });
    }
    
    // Setup keyboard navigation to auto-focus search
    function setupKeyboardNavigation() {
        // Remove any existing listener to avoid duplicates
        if (window.snapshotsKeyboardHandler) {
            document.removeEventListener('keydown', window.snapshotsKeyboardHandler);
        }
        
        // Define the handler
        window.snapshotsKeyboardHandler = (event) => {
            // Only handle if snapshots tab is active
            const activeTab = document.querySelector('.tab.active');
            if (!activeTab || activeTab.getAttribute('data-tab') !== 'snapshots') {
                return;
            }
            
            const searchInput = document.getElementById('snapshot-search');
            if (!searchInput) return;
            
            // Handle Escape for resetting filters
            if (event.key === 'Escape') {
                const resetButton = document.getElementById('reset-snapshots-button');
                if (resetButton) {
                    resetButton.click();
                }
                return;
            }
            
            // Ignore if already typing in an input, textarea, or select
            const targetElement = event.target;
            const targetTagName = targetElement.tagName;
            if (targetTagName === 'INPUT' || targetTagName === 'TEXTAREA' || targetTagName === 'SELECT') {
                return;
            }
            
            // Ignore if any modal is open
            const modals = document.querySelectorAll('.modal:not(.hidden)');
            if (modals.length > 0) {
                return;
            }
            
            // For single character keys (letters, numbers, etc.)
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
                if (document.activeElement !== searchInput) {
                    searchInput.focus();
                    event.preventDefault();
                    searchInput.value += event.key;
                    searchInput.dispatchEvent(new Event('input', { bubbles: true, cancelable: true }));
                }
            } 
            // For Backspace
            else if (event.key === 'Backspace' && !event.ctrlKey && !event.metaKey && !event.altKey) {
                if (document.activeElement !== searchInput) {
                    searchInput.focus();
                    event.preventDefault();
                }
            }
        };
        
        // Add the listener
        document.addEventListener('keydown', window.snapshotsKeyboardHandler);
    }
    
    // Update reset button state
    function updateResetButtonState() {
        const hasFilters = hasActiveFilters();
        const resetButton = document.getElementById('reset-snapshots-button');
        
        if (resetButton) {
            if (hasFilters) {
                resetButton.classList.remove('opacity-50', 'cursor-not-allowed');
                resetButton.classList.add('hover:bg-gray-100', 'dark:hover:bg-gray-700');
                resetButton.disabled = false;
            } else {
                resetButton.classList.add('opacity-50', 'cursor-not-allowed');
                resetButton.classList.remove('hover:bg-gray-100', 'dark:hover:bg-gray-700');
                resetButton.disabled = true;
            }
        }
    }
    
    // Check if any filters are active
    function hasActiveFilters() {
        const isDefaultSort = currentSort.field === 'snaptime' && !currentSort.ascending;
        
        return currentFilters.searchTerm !== '' ||
               currentFilters.guestType !== 'all' ||
               !isDefaultSort;
    }

    function sortSnapshots(snapshots) {
        return snapshots.sort((a, b) => {
            let aVal = a[currentSort.field];
            let bVal = b[currentSort.field];
            
            // Handle numeric fields
            if (currentSort.field === 'vmid') {
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            } else if (currentSort.field === 'snaptime') {
                aVal = aVal || 0;
                bVal = bVal || 0;
            } else {
                // String fields
                aVal = (aVal || '').toString().toLowerCase();
                bVal = (bVal || '').toString().toLowerCase();
            }
            
            if (aVal < bVal) return currentSort.ascending ? -1 : 1;
            if (aVal > bVal) return currentSort.ascending ? 1 : -1;
            return 0;
        });
    }
    
    function sortBy(field) {
        if (currentSort.field === field) {
            currentSort.ascending = !currentSort.ascending;
        } else {
            currentSort.field = field;
            currentSort.ascending = true;
        }
        
        const tbody = document.querySelector('#snapshots-content tbody');
        if (tbody) {
            tbody.innerHTML = renderSnapshotRows();
        }
        
        // Update sort UI using common function
        const table = document.querySelector('#snapshots-content table');
        if (table && PulseApp.ui.common) {
            const clickedHeader = table.querySelector(`th[data-sort="${field}"]`);
            if (clickedHeader) {
                // Create a temporary table element with ID for common.js compatibility
                table.id = 'snapshots-table';
                PulseApp.state.setSortState('snapshots', field, currentSort.ascending ? 'asc' : 'desc');
                PulseApp.ui.common.updateSortUI('snapshots-table', clickedHeader, 'snapshots');
            }
        }
        
        // Update reset button state  
        updateResetButtonState();
    }

    function resetFiltersAndSort() {
        // Reset search input
        const searchInput = document.getElementById('snapshot-search');
        if (searchInput) {
            searchInput.value = '';
            currentFilters.searchTerm = '';
        }
        
        // Reset guest type filter to 'all'
        const typeAllRadio = document.getElementById('type-all');
        if (typeAllRadio) {
            typeAllRadio.checked = true;
            currentFilters.guestType = 'all';
        }
        
        // Reset sort to default (snapshot time descending)
        currentSort.field = 'snaptime';
        currentSort.ascending = false;
        
        // Update the table with reset filters and sort
        const tbody = document.querySelector('#snapshots-content tbody');
        if (tbody) {
            tbody.innerHTML = renderSnapshotRows();
        }
        
        // Update sort UI
        PulseApp.state.setSortState('snapshots', 'snaptime', 'desc');
        const snaptimeHeader = document.querySelector('#snapshots-table th[data-sort="snaptime"]');
        if (snaptimeHeader) {
            PulseApp.ui.common.updateSortUI('snapshots-table', snaptimeHeader, 'snapshots');
        }
    }

    return {
        init,
        updateSnapshotsInfo,
        sortBy,
        resetFiltersAndSort
    };
})();