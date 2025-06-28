PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let isInitialized = false;
    let currentFilters = {
        searchTerm: '',
        backupType: 'all', // 'all', 'pve', 'pbs'
        node: 'all'
    };
    let currentSort = {
        field: 'ctime',
        ascending: false
    };
    let backupsData = {
        unified: [],
        pbsEnabled: false
    };

    function init() {
        if (isInitialized) return;
        isInitialized = true;
        
        updateBackupsInfo();
    }

    function updateBackupsInfo() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        // Only show loading state on initial load
        if (!isInitialized || backupsData.unified.length === 0) {
            container.innerHTML = `
                <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                    Loading backups...
                </div>
            `;
        }

        // Fetch unified backups data
        fetch('/api/backups/unified')
            .then(response => response.json())
            .then(data => {
                const newBackups = data.backups || [];
                const newPbsEnabled = data.pbs?.enabled || false;
                
                // Check if data has actually changed
                const hasChanged = JSON.stringify(newBackups) !== JSON.stringify(backupsData.unified) ||
                                 newPbsEnabled !== backupsData.pbsEnabled;
                
                if (hasChanged) {
                    backupsData.unified = newBackups;
                    backupsData.pbsEnabled = newPbsEnabled;
                    
                    // Only render full UI on first load
                    if (container.querySelector('.overflow-x-auto')) {
                        // Update only the table body
                        const tbody = container.querySelector('tbody');
                        if (tbody) {
                            tbody.innerHTML = renderBackupRows();
                        }
                        
                        // Update summary
                        const summary = calculateSummary();
                        const summaryElements = container.querySelectorAll('.text-xl.font-semibold');
                        if (summaryElements.length >= 3) {
                            summaryElements[0].textContent = summary.total;
                            summaryElements[1].textContent = summary.pve;
                            if (backupsData.pbsEnabled && summaryElements.length >= 4) {
                                summaryElements[2].textContent = summary.pbs;
                                summaryElements[3].textContent = formatBytes(summary.totalSize);
                            } else {
                                summaryElements[2].textContent = formatBytes(summary.totalSize);
                            }
                        }
                    } else {
                        // First load - render full UI
                        renderBackupsUI();
                    }
                }
            })
            .catch(error => {
                console.error('Error fetching backups:', error);
                // Only show error if we don't have data already
                if (backupsData.unified.length === 0) {
                    container.innerHTML = `
                        <div class="p-8 text-center">
                            <div class="text-red-500 dark:text-red-400">
                                Failed to load backups data: ${error.message}
                            </div>
                            <button onclick="PulseApp.ui.backups.updateBackupsInfo()" class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
                                Retry
                            </button>
                        </div>
                    `;
                }
            });
    }

    function renderBackupsUI() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        const summary = calculateSummary();
        const uniqueNodes = getUniqueNodes();

        container.innerHTML = `
            <!-- Summary -->
            <div class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                        <div class="text-gray-500 dark:text-gray-400">Total Backups</div>
                        <div class="text-xl font-semibold">${summary.total}</div>
                    </div>
                    <div>
                        <div class="text-gray-500 dark:text-gray-400">PVE Backups</div>
                        <div class="text-xl font-semibold text-orange-600">${summary.pve}</div>
                    </div>
                    ${backupsData.pbsEnabled ? `
                        <div>
                            <div class="text-gray-500 dark:text-gray-400">PBS Backups</div>
                            <div class="text-xl font-semibold text-purple-600">${summary.pbs}</div>
                        </div>
                    ` : ''}
                    <div>
                        <div class="text-gray-500 dark:text-gray-400">Total Size</div>
                        <div class="text-xl font-semibold">${formatBytes(summary.totalSize)}</div>
                    </div>
                </div>
            </div>

            <!-- Filters -->
            <div class="mb-3 p-2 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="flex flex-wrap items-center gap-3">
                    <input type="search" id="backup-search" placeholder="Search by VMID or notes..." 
                        value="${currentFilters.searchTerm}"
                        class="flex-1 min-w-[200px] p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                    
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Type:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="backup-type-all" name="backup-type" value="all" class="hidden peer/all" ${currentFilters.backupType === 'all' ? 'checked' : ''}>
                            <label for="backup-type-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="backup-type-pve" name="backup-type" value="pve" class="hidden peer/pve" ${currentFilters.backupType === 'pve' ? 'checked' : ''}>
                            <label for="backup-type-pve" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/pve:bg-gray-100 dark:peer-checked/pve:bg-gray-700 peer-checked/pve:text-blue-600 dark:peer-checked/pve:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">PVE</label>
                            
                            ${backupsData.pbsEnabled ? `
                                <input type="radio" id="backup-type-pbs" name="backup-type" value="pbs" class="hidden peer/pbs" ${currentFilters.backupType === 'pbs' ? 'checked' : ''}>
                                <label for="backup-type-pbs" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/pbs:bg-gray-100 dark:peer-checked/pbs:bg-gray-700 peer-checked/pbs:text-blue-600 dark:peer-checked/pbs:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">PBS</label>
                            ` : ''}
                        </div>
                    </div>
                    
                    ${uniqueNodes.length > 1 ? `
                        <div class="flex items-center gap-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Node:</span>
                            <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                                <input type="radio" id="node-all" name="backup-node" value="all" class="hidden peer/all" ${currentFilters.node === 'all' ? 'checked' : ''}>
                                <label for="node-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                                
                                ${uniqueNodes.map((node, idx) => `
                                    <input type="radio" id="node-${node}" name="backup-node" value="${node}" class="hidden peer/${node}" ${currentFilters.node === node ? 'checked' : ''}>
                                    <label for="node-${node}" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/${node}:bg-gray-100 dark:peer-checked/${node}:bg-gray-700 peer-checked/${node}:text-blue-600 dark:peer-checked/${node}:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">${node}</label>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                </div>
            </div>

            <!-- Backups Table -->
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
                <table class="w-full text-xs sm:text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-800">
                        <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="vmid">VMID</th>
                            <th class="p-1 px-2 whitespace-nowrap">Name/Notes</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="type">Type</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="node">Node</th>
                            <th class="p-1 px-2 whitespace-nowrap">Storage</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="size">Size</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="ctime">Age</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="source">Source</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${renderBackupRows()}
                    </tbody>
                </table>
            </div>
        `;

        // Setup event listeners
        setupEventListeners();
        
        // Setup sortable headers
        const table = document.querySelector('#backups-content table');
        if (table) {
            table.id = 'backups-table';
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
        let total = 0;
        let pve = 0;
        let pbs = 0;
        let totalSize = 0;

        backupsData.unified.forEach(backup => {
            total++;
            totalSize += backup.size || 0;
            if (backup.source === 'pve') {
                pve++;
            } else {
                pbs++;
            }
        });

        return { total, pve, pbs, totalSize };
    }

    function getUniqueNodes() {
        const nodes = new Set();
        backupsData.unified.forEach(backup => {
            if (backup.node) nodes.add(backup.node);
        });
        return Array.from(nodes).sort();
    }

    function renderBackupRows() {
        const filtered = filterBackups();
        const sorted = sortBackups(filtered);
        
        if (sorted.length === 0) {
            return `
                <tr>
                    <td colspan="8" class="px-3 py-8 text-center text-gray-500 dark:text-gray-400">
                        No backups found
                    </td>
                </tr>
            `;
        }

        return sorted.map(backup => {
            const age = getRelativeTime(backup.ctime);
            const typeLabel = backup.type === 'vm' ? 'VM' : 'LXC';
            const sourceColor = backup.source === 'pve' ? 'orange' : 'purple';
            
            return `
                <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td class="p-1 px-2 align-middle">${backup.vmid}</td>
                    <td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400">
                        <div class="max-w-[120px] sm:max-w-[200px] lg:max-w-[300px] truncate" title="${backup.notes || ''}">
                            ${backup.notes || '-'}
                        </div>
                    </td>
                    <td class="p-1 px-2 align-middle">
                        <span class="px-1.5 py-0.5 text-xs font-medium rounded ${
                            backup.type === 'vm' 
                                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' 
                                : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                        }">${typeLabel}</span>
                    </td>
                    <td class="p-1 px-2 align-middle">${backup.node || '-'}</td>
                    <td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400">${backup.storage || backup.datastore || '-'}</td>
                    <td class="p-1 px-2 align-middle whitespace-nowrap">${formatBytes(backup.size)}</td>
                    <td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400 whitespace-nowrap">${age}</td>
                    <td class="p-1 px-2 align-middle">
                        <span class="inline-flex items-center gap-1">
                            <span class="inline-block w-2 h-2 rounded-full bg-${sourceColor}-500"></span>
                            <span class="text-xs">${backup.source.toUpperCase()}</span>
                        </span>
                    </td>
                </tr>
            `;
        }).join('');
    }

    function filterBackups() {
        return backupsData.unified.filter(backup => {
            // Type filter
            if (currentFilters.backupType !== 'all') {
                if (backup.source !== currentFilters.backupType) return false;
            }
            
            // Node filter
            if (currentFilters.node !== 'all') {
                if (backup.node !== currentFilters.node) return false;
            }
            
            // Search filter
            if (currentFilters.searchTerm) {
                const search = currentFilters.searchTerm.toLowerCase();
                const vmidMatch = backup.vmid?.toString().includes(search);
                const notesMatch = backup.notes?.toLowerCase().includes(search);
                const nodeMatch = backup.node?.toLowerCase().includes(search);
                const storageMatch = (backup.storage || backup.datastore || '').toLowerCase().includes(search);
                if (!vmidMatch && !notesMatch && !nodeMatch && !storageMatch) return false;
            }
            
            return true;
        });
    }

    function formatBytes(bytes) {
        if (!bytes || bytes === 0) return '0\u00A0B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        const value = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
        // Use non-breaking space to prevent wrapping between number and unit
        return value + '\u00A0' + sizes[i];
    }

    function getRelativeTime(timestamp) {
        if (!timestamp) return 'Unknown';
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        
        if (diff < 60) return 'Just\u00A0now';
        if (diff < 3600) return Math.floor(diff / 60) + 'm\u00A0ago';
        if (diff < 86400) return Math.floor(diff / 3600) + 'h\u00A0ago';
        if (diff < 604800) return Math.floor(diff / 86400) + 'd\u00A0ago';
        return new Date(timestamp * 1000).toLocaleDateString();
    }

    function setupEventListeners() {
        // Search
        const searchInput = document.getElementById('backup-search');
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                currentFilters.searchTerm = e.target.value;
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
            });
        }
        
        // Auto-focus search on keypress (like main dashboard)
        document.addEventListener('keydown', (e) => {
            // Check if backups tab is active
            const backupsTab = document.getElementById('backups');
            if (!backupsTab || backupsTab.classList.contains('hidden')) return;
            
            // Don't interfere with input fields or special keys
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || 
                e.ctrlKey || e.metaKey || e.altKey || e.key === 'Tab' || e.key === 'Escape') {
                return;
            }
            
            // Focus search on alphanumeric keys
            if (e.key.length === 1 && /[a-zA-Z0-9]/.test(e.key)) {
                const searchInput = document.getElementById('backup-search');
                if (searchInput && document.activeElement !== searchInput) {
                    e.preventDefault();
                    searchInput.focus();
                    searchInput.value = searchInput.value + e.key;
                    searchInput.dispatchEvent(new Event('input'));
                }
            }
        });
        
        // Type filter radio buttons
        document.querySelectorAll('input[name="backup-type"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.backupType = e.target.value;
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
            });
        });
        
        // Node filter radio buttons
        document.querySelectorAll('input[name="backup-node"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.node = e.target.value;
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
            });
        });
    }

    function sortBackups(backups) {
        return backups.sort((a, b) => {
            let aVal = a[currentSort.field];
            let bVal = b[currentSort.field];
            
            // Handle numeric fields
            if (currentSort.field === 'vmid') {
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            } else if (currentSort.field === 'ctime' || currentSort.field === 'size') {
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
        
        const tbody = document.querySelector('#backups-content tbody');
        if (tbody) {
            tbody.innerHTML = renderBackupRows();
        }
        
        // Update sort UI using common function
        const table = document.querySelector('#backups-content table');
        if (table && PulseApp.ui.common) {
            const clickedHeader = table.querySelector(`th[data-sort="${field}"]`);
            if (clickedHeader) {
                // Create a temporary table element with ID for common.js compatibility
                table.id = 'backups-table';
                PulseApp.state.setSortState('backups', field, currentSort.ascending ? 'asc' : 'desc');
                PulseApp.ui.common.updateSortUI('backups-table', clickedHeader, 'backups');
            }
        }
    }

    return {
        init,
        updateBackupsInfo,
        sortBy
    };
})();