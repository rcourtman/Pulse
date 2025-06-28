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
                        
                        // Update summary
                        const summary = calculateSummary();
                        const summaryElements = container.querySelectorAll('.text-xl.font-semibold');
                        if (summaryElements.length >= 2) {
                            summaryElements[0].textContent = summary.totalCount;
                            summaryElements[1].textContent = summary.uniqueGuests;
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

        container.innerHTML = `
            <!-- Summary -->
            <div class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="grid grid-cols-2 gap-4 text-sm">
                    <div>
                        <div class="text-gray-600 dark:text-gray-400">Active Snapshots</div>
                        <div class="text-xl font-semibold">${summary.totalCount}</div>
                    </div>
                    <div>
                        <div class="text-gray-600 dark:text-gray-400">Guests with Snapshots</div>
                        <div class="text-xl font-semibold">${summary.uniqueGuests}</div>
                    </div>
                </div>
            </div>

            <!-- Filters -->
            <div class="mb-3 p-2 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="flex flex-wrap items-center gap-3">
                    <input type="search" id="snapshot-search" placeholder="Search snapshots..." 
                        value="${currentFilters.searchTerm}"
                        class="flex-1 min-w-[200px] p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                    
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-gray-600 dark:text-gray-400 font-medium">Type:</span>
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
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
                <table class="w-full text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-700">
                        <tr class="text-left text-xs">
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('vmid')">
                                VMID <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('name')">
                                Guest Name <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('type')">
                                Type <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('snapname')">
                                Snapshot Name <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('node')">
                                Node <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600" onclick="PulseApp.ui.snapshots.sortBy('snaptime')">
                                Age <span class="sort-indicator"></span>
                            </th>
                            <th class="p-1 px-2">Description</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        ${renderSnapshotRows()}
                    </tbody>
                </table>
            </div>
        `;

        // Setup event listeners
        setupEventListeners();
    }

    function calculateSummary() {
        let totalCount = 0;
        const uniqueGuests = new Set();

        snapshotsData.forEach(snapshot => {
            totalCount++;
            uniqueGuests.add(`${snapshot.node}:${snapshot.vmid}`);
        });

        return { totalCount, uniqueGuests: uniqueGuests.size };
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

        return sorted.map(snapshot => {
            const age = getRelativeTime(snapshot.snaptime);
            const typeLabel = snapshot.type === 'qemu' ? 'VM' : 'LXC';
            
            return `
                <tr class="hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td class="p-1 px-2">${snapshot.vmid}</td>
                    <td class="p-1 px-2">${snapshot.name}</td>
                    <td class="p-1 px-2">
                        <span class="px-1.5 py-0.5 text-xs font-medium rounded ${
                            snapshot.type === 'qemu' 
                                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' 
                                : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                        }">${typeLabel}</span>
                    </td>
                    <td class="p-1 px-2">${snapshot.snapname}</td>
                    <td class="p-1 px-2">${snapshot.node}</td>
                    <td class="p-1 px-2 text-gray-600 dark:text-gray-400">${age}</td>
                    <td class="p-1 px-2 text-gray-600 dark:text-gray-400">${snapshot.description || '-'}</td>
                </tr>
            `;
        }).join('');
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

    function getRelativeTime(timestamp) {
        if (!timestamp) return 'Unknown';
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        
        if (diff < 60) return 'Just now';
        if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
        if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
        if (diff < 604800) return Math.floor(diff / 86400) + 'd ago';
        if (diff < 2592000) return Math.floor(diff / 604800) + 'w ago';
        return Math.floor(diff / 2592000) + 'mo ago';
    }

    function setupEventListeners() {
        // Search
        const searchInput = document.getElementById('snapshot-search');
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                currentFilters.searchTerm = e.target.value;
                const filtered = filterSnapshots();
                const tbody = document.querySelector('#snapshots-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderSnapshotRows();
                }
            });
        }
        
        // Auto-focus search on keypress (like main dashboard)
        document.addEventListener('keydown', (e) => {
            // Check if snapshots tab is active
            const snapshotsTab = document.getElementById('snapshots');
            if (!snapshotsTab || snapshotsTab.classList.contains('hidden')) return;
            
            // Don't interfere with input fields or special keys
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || 
                e.ctrlKey || e.metaKey || e.altKey || e.key === 'Tab' || e.key === 'Escape') {
                return;
            }
            
            // Focus search on alphanumeric keys
            if (e.key.length === 1 && /[a-zA-Z0-9]/.test(e.key)) {
                const searchInput = document.getElementById('snapshot-search');
                if (searchInput && document.activeElement !== searchInput) {
                    e.preventDefault();
                    searchInput.focus();
                    searchInput.value = searchInput.value + e.key;
                    searchInput.dispatchEvent(new Event('input'));
                }
            }
        });
        
        // Guest type radio buttons
        document.querySelectorAll('input[name="guest-type"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.guestType = e.target.value;
                const filtered = filterSnapshots();
                const tbody = document.querySelector('#snapshots-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderSnapshotRows();
                }
            });
        });
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
        
        // Update sort indicators
        document.querySelectorAll('th .sort-indicator').forEach(indicator => {
            indicator.textContent = '';
        });
        
        const activeHeader = document.querySelector(`th[onclick*="${field}"] .sort-indicator`);
        if (activeHeader) {
            activeHeader.textContent = currentSort.ascending ? '▲' : '▼';
        }
    }

    return {
        init,
        updateSnapshotsInfo,
        sortBy
    };
})();