PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pve = (() => {
    // State
    let isInitialized = false;
    let pveData = {
        backups: []
    };
    
    let filters = {
        searchTerm: '',
        node: 'all',
        storage: 'all',
        guestType: 'all',
        selectedDate: null
    };
    
    let currentSort = {
        field: 'ctime',
        ascending: false
    };
    
    // Initialize
    function init() {
        if (isInitialized) return;
        isInitialized = true;
        updatePVEInfo();
    }
    
    // Fetch and update PVE backup data
    function updatePVEInfo() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        // Only show loading state on initial load
        if (!isInitialized || pveData.backups.length === 0) {
            container.innerHTML = `
                <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                    Loading PVE backups...
                </div>
            `;
        }

        fetch('/api/backups/pve')
            .then(r => r.json())
            .then(data => {
                const newBackups = data.backups || [];
                
                // Check if data has actually changed
                const backupsChanged = JSON.stringify(newBackups) !== JSON.stringify(pveData.backups);
                
                if (backupsChanged || !isInitialized) {
                    pveData.backups = newBackups;
                    renderPVEUI();
                }
            })
            .catch(error => {
                console.error('Error fetching PVE backups:', error);
                // Only show error if we don't have data already
                if (pveData.backups.length === 0) {
                    container.innerHTML = `
                        <div class="p-8 text-center">
                            <div class="text-red-500 dark:text-red-400">
                                Failed to load PVE backups: ${error.message}
                            </div>
                            <button onclick="PulseApp.ui.pve.updatePVEInfo()" class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
                                Retry
                            </button>
                        </div>
                    `;
                }
            });
    }
    
    // Main render function
    function renderPVEUI() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        // Save scroll position before update
        const scrollContainer = container.querySelector('.overflow-x-auto');
        const savedScrollLeft = scrollContainer ? scrollContainer.scrollLeft : 0;
        const savedScrollTop = scrollContainer ? scrollContainer.scrollTop : 0;

        container.innerHTML = renderPVEContent();

        // Restore scroll position after update
        const newScrollContainer = container.querySelector('.overflow-x-auto');
        if (newScrollContainer && (savedScrollLeft > 0 || savedScrollTop > 0)) {
            newScrollContainer.scrollLeft = savedScrollLeft;
            newScrollContainer.scrollTop = savedScrollTop;
        }

        // Setup event listeners
        setupEventListeners();
        updateResetButtonState();
    }
    
    // Render PVE content
    function renderPVEContent() {
        const uniqueNodes = getUniqueValues('node');
        const uniqueStorages = getUniqueValues('storage');
        
        return `
            ${renderBackupSummary()}

            <!-- PVE Filters -->
            <div class="mb-3 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-sm">
                <div class="flex flex-row flex-wrap items-center gap-2 sm:gap-3">
                    <div class="filter-controls-wrapper flex items-center gap-2 flex-1 min-w-[180px] sm:min-w-[240px]">
                        <input type="search" id="pve-search" placeholder="Search by VMID or notes..." 
                            value="${filters.searchTerm}"
                            class="flex-1 p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                        <button id="reset-pve-button" title="Reset Filters & Sort (Esc)" class="flex items-center justify-center p-1 h-7 w-7 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none transition-colors flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>
                        </button>
                    </div>
                    
                    <!-- Node Filter -->
                    ${uniqueNodes.length > 1 ? `
                        <div class="flex items-center gap-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Node:</span>
                            <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                                <input type="radio" id="pve-node-all" name="pve-node" value="all" class="hidden peer/all" ${filters.node === 'all' ? 'checked' : ''}>
                                <label for="pve-node-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                                
                                ${uniqueNodes.map((node, idx) => `
                                    <input type="radio" id="pve-node-${idx}" name="pve-node" value="${node}" class="hidden peer/node${idx}" ${filters.node === node ? 'checked' : ''}>
                                    <label for="pve-node-${idx}" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/node${idx}:bg-gray-100 dark:peer-checked/node${idx}:bg-gray-700 peer-checked/node${idx}:text-blue-600 dark:peer-checked/node${idx}:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">${node}</label>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Storage Filter -->
                    ${uniqueStorages.length > 1 ? `
                        <div class="flex flex-wrap items-center gap-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Storage:</span>
                            <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                                <input type="radio" id="pve-storage-all" name="pve-storage" value="all" class="hidden peer/all" ${filters.storage === 'all' ? 'checked' : ''}>
                                <label for="pve-storage-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                                
                                ${uniqueStorages.map((storage, idx) => `
                                    <input type="radio" id="pve-storage-${idx}" name="pve-storage" value="${storage}" class="hidden peer/storage${idx}" ${filters.storage === storage ? 'checked' : ''}>
                                    <label for="pve-storage-${idx}" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/storage${idx}:bg-gray-100 dark:peer-checked/storage${idx}:bg-gray-700 peer-checked/storage${idx}:text-blue-600 dark:peer-checked/storage${idx}:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">${storage}</label>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Type Filter -->
                    <div class="flex flex-wrap items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Type:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="pve-type-all" name="pve-type" value="all" class="hidden peer/all" ${filters.guestType === 'all' ? 'checked' : ''}>
                            <label for="pve-type-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="pve-type-vm" name="pve-type" value="vm" class="hidden peer/vm" ${filters.guestType === 'vm' ? 'checked' : ''}>
                            <label for="pve-type-vm" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/vm:bg-gray-100 dark:peer-checked/vm:bg-gray-700 peer-checked/vm:text-blue-600 dark:peer-checked/vm:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">VMs</label>
                            
                            <input type="radio" id="pve-type-lxc" name="pve-type" value="lxc" class="hidden peer/lxc" ${filters.guestType === 'lxc' ? 'checked' : ''}>
                            <label for="pve-type-lxc" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/lxc:bg-gray-100 dark:peer-checked/lxc:bg-gray-700 peer-checked/lxc:text-blue-600 dark:peer-checked/lxc:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">LXCs</label>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Backups Table -->
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
                <table class="w-full text-xs sm:text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-800">
                        <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                            ${renderTableHeader()}
                        </tr>
                    </thead>
                    <tbody>
                        ${renderTableRows(filterAndSortBackups())}
                    </tbody>
                </table>
            </div>
        `;
    }
    
    // Render backup summary
    function renderBackupSummary() {
        const backups = pveData.backups || [];
        
        if (backups.length === 0) {
            return '';
        }
        
        // Calculate total size and age statistics
        let totalSize = 0;
        let oldestBackup = null;
        let newestBackup = null;
        
        backups.forEach(backup => {
            totalSize += backup.size || 0;
            
            if (!oldestBackup || backup.ctime < oldestBackup.ctime) {
                oldestBackup = backup;
            }
            if (!newestBackup || backup.ctime > newestBackup.ctime) {
                newestBackup = backup;
            }
        });
        
        const now = Date.now() / 1000;
        const oldestAge = oldestBackup ? Math.floor((now - oldestBackup.ctime) / (24 * 60 * 60)) : 0;
        const newestAge = newestBackup ? Math.floor((now - newestBackup.ctime) / (24 * 60 * 60)) : 0;
        
        return `
            <!-- Backup Summary -->
            <div class="mb-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-sm">
                <div class="p-2">
                    <div class="flex items-center justify-between">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">PVE Backup Summary</h3>
                        <div class="text-sm text-gray-600 dark:text-gray-400">
                            <span class="font-semibold text-gray-800 dark:text-gray-100">${backups.length}</span> backups
                            <span class="mx-1">•</span>
                            <span class="font-semibold ${getSizeColorClass(totalSize)}">${formatBytes(totalSize).text}</span>
                            <span class="mx-1">•</span>
                            Newest: <span class="font-semibold ${newestAge === 0 ? 'text-green-600 dark:text-green-400' : newestAge <= 3 ? 'text-blue-600 dark:text-blue-400' : newestAge <= 7 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'}">${newestAge}d ago</span>
                            <span class="mx-1">•</span>
                            Oldest: <span class="font-semibold ${oldestAge <= 30 ? 'text-green-600 dark:text-green-400' : oldestAge <= 90 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'}">${oldestAge}d ago</span>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }
    
    // Render table header
    function renderTableHeader() {
        const headers = [
            { field: 'status', label: 'Status', width: 'w-12', center: true },
            { field: 'vmid', label: 'VMID', width: 'w-16' },
            { field: 'notes', label: 'Name/Notes', width: '' },
            { field: 'type', label: 'Type', width: 'w-16' },
            { field: 'node', label: 'Node', width: 'w-24' },
            { field: 'storage', label: 'Storage', width: 'w-24' },
            { field: 'size', label: 'Size', width: 'w-24' }
        ];
        
        return headers.map(header => {
            const isActive = currentSort.field === header.field;
            const sortIcon = isActive ? (currentSort.ascending ? '↑' : '↓') : '';
            const sortable = header.field !== 'status' && header.field !== 'notes';
            
            return `
                <th class="${sortable ? 'sortable' : ''} p-1 px-2 whitespace-nowrap ${header.center ? 'text-center' : ''}" 
                    ${sortable ? `onclick="PulseApp.ui.pve.sortTable('${header.field}')"` : ''}>
                    ${header.label} ${sortIcon}
                </th>
            `;
        }).join('');
    }
    
    // Render table rows grouped by date
    function renderTableRows(filteredBackups) {
        if (filteredBackups.length === 0) {
            return `
                <tr>
                    <td colspan="7" class="p-4 text-center text-gray-500 dark:text-gray-400">
                        No backups found
                    </td>
                </tr>
            `;
        }
        
        // Group backups by date
        const groupedBackups = {};
        filteredBackups.forEach(backup => {
            const dateKey = formatDateKey(backup.ctime);
            if (!groupedBackups[dateKey]) {
                groupedBackups[dateKey] = [];
            }
            groupedBackups[dateKey].push(backup);
        });
        
        // Sort dates (newest first)
        const sortedDates = Object.keys(groupedBackups).sort().reverse();
        
        let html = '';
        sortedDates.forEach(dateKey => {
            const backups = groupedBackups[dateKey];
            const displayDate = formatDateDisplay(dateKey);
            
            // Add date header
            html += `
                <tr class="bg-gray-50 dark:bg-gray-700/50">
                    <td colspan="7" class="p-1 px-2 text-xs font-medium text-gray-600 dark:text-gray-400">
                        ${displayDate} (${backups.length} backup${backups.length > 1 ? 's' : ''})
                    </td>
                </tr>
            `;
            
            // Add backups for this date
            backups.forEach(backup => {
                const size = formatBytes(backup.size || 0);
                const typeLabel = backup.guestType || 'Unknown';
                
                html += `
                    <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                        <td class="p-1 px-2 whitespace-nowrap text-center">
                            <span class="inline-flex items-center justify-center w-4 h-4">
                                <svg class="w-3 h-3 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path>
                                </svg>
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap font-medium">${backup.vmid}</td>
                        <td class="p-1 px-2 max-w-[200px] truncate" title="${backup.notes || ''}">
                            ${backup.notes || '-'}
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap">
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${typeLabel === 'VM' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'}">
                                ${typeLabel}
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap">${backup.node}</td>
                        <td class="p-1 px-2 whitespace-nowrap">${backup.storage}</td>
                        <td class="p-1 px-2 whitespace-nowrap">${size.text}</td>
                    </tr>
                `;
            });
        });
        
        return html;
    }
    
    // Filter and sort backups
    function filterAndSortBackups() {
        let filtered = [...pveData.backups];
        
        // Apply filters
        if (filters.searchTerm) {
            const search = filters.searchTerm.toLowerCase();
            filtered = filtered.filter(backup => 
                backup.vmid.toString().includes(search) ||
                (backup.notes && backup.notes.toLowerCase().includes(search))
            );
        }
        
        if (filters.node !== 'all') {
            filtered = filtered.filter(backup => backup.node === filters.node);
        }
        
        if (filters.storage !== 'all') {
            filtered = filtered.filter(backup => backup.storage === filters.storage);
        }
        
        if (filters.guestType !== 'all') {
            filtered = filtered.filter(backup => {
                const type = backup.guestType || 'Unknown';
                if (filters.guestType === 'vm') return type === 'VM';
                if (filters.guestType === 'lxc') return type === 'LXC';
                return false;
            });
        }
        
        // Apply sorting
        filtered.sort((a, b) => {
            let aVal = a[currentSort.field];
            let bVal = b[currentSort.field];
            
            // Handle numeric fields
            if (currentSort.field === 'vmid' || currentSort.field === 'size' || currentSort.field === 'ctime') {
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            }
            
            if (aVal < bVal) return currentSort.ascending ? -1 : 1;
            if (aVal > bVal) return currentSort.ascending ? 1 : -1;
            return 0;
        });
        
        return filtered;
    }
    
    // Event listeners
    function setupEventListeners() {
        const searchInput = document.getElementById('pve-search');
        const resetButton = document.getElementById('reset-pve-button');
        
        // Search input
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                filters.searchTerm = e.target.value;
                updateTable();
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
        
        // Radio button filters
        document.querySelectorAll('input[name="pve-node"], input[name="pve-storage"], input[name="pve-type"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                const filterName = e.target.name.replace('pve-', '');
                if (filterName === 'node') {
                    filters.node = e.target.value;
                } else if (filterName === 'storage') {
                    filters.storage = e.target.value;
                } else if (filterName === 'type') {
                    filters.guestType = e.target.value;
                }
                updateTable();
                updateResetButtonState();
            });
        });
        
        // Set up keyboard navigation for auto-focus search
        setupKeyboardNavigation();
    }
    
    // Setup keyboard navigation to auto-focus search
    function setupKeyboardNavigation() {
        // Remove any existing listener to avoid duplicates
        if (window.pveKeyboardHandler) {
            document.removeEventListener('keydown', window.pveKeyboardHandler);
        }
        
        // Define the handler
        window.pveKeyboardHandler = (event) => {
            // Only handle if backups (PVE) tab is active
            const activeTab = document.querySelector('.tab.active');
            if (!activeTab || activeTab.getAttribute('data-tab') !== 'backups') {
                return;
            }
            
            const searchInput = document.getElementById('pve-search');
            if (!searchInput) return;
            
            // Handle Escape for resetting filters
            if (event.key === 'Escape') {
                const resetButton = document.getElementById('reset-pve-button');
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
        document.addEventListener('keydown', window.pveKeyboardHandler);
    }
    
    // Update table without full re-render
    function updateTable() {
        const tbody = document.querySelector('#backups-content tbody');
        if (tbody) {
            tbody.innerHTML = renderTableRows(filterAndSortBackups());
        }
    }
    
    // Sort table
    function sortTable(field) {
        if (currentSort.field === field) {
            currentSort.ascending = !currentSort.ascending;
        } else {
            currentSort.field = field;
            currentSort.ascending = false;
        }
        renderPVEUI();
    }
    
    // Reset filters and sort
    function resetFiltersAndSort() {
        const searchInput = document.getElementById('pve-search');
        if (searchInput) {
            searchInput.value = '';
        }
        
        // Reset filters
        filters = {
            searchTerm: '',
            node: 'all',
            storage: 'all',
            guestType: 'all',
            selectedDate: null
        };
        
        // Reset sort
        currentSort.field = 'ctime';
        currentSort.ascending = false;
        
        // Re-render the UI to reset all radio buttons
        renderPVEUI();
    }
    
    // Update reset button state
    function updateResetButtonState() {
        const hasFilters = hasActiveFilters();
        const resetButton = document.getElementById('reset-pve-button');
        
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
        const isDefaultSort = currentSort.field === 'ctime' && !currentSort.ascending;
        
        return filters.searchTerm !== '' ||
               filters.node !== 'all' ||
               filters.storage !== 'all' ||
               filters.guestType !== 'all' ||
               !isDefaultSort;
    }
    
    // Helper functions
    function getUniqueValues(field) {
        const values = new Set();
        pveData.backups.forEach(backup => {
            if (backup[field]) values.add(backup[field]);
        });
        return Array.from(values).sort();
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
    
    function formatBytes(bytes) {
        if (bytes === 0) return { value: 0, unit: 'B', text: '0 B' };
        
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        const value = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
        
        return {
            value: value,
            unit: sizes[i],
            text: `${value} ${sizes[i]}`
        };
    }
    
    // Get color class based on backup size
    function getSizeColorClass(sizeInBytes) {
        const gb = sizeInBytes / (1024 * 1024 * 1024);
        
        if (gb < 50) {
            // Less than 50 GB - green
            return 'text-green-600 dark:text-green-400';
        } else if (gb < 200) {
            // 50-200 GB - blue
            return 'text-blue-600 dark:text-blue-400';
        } else if (gb < 500) {
            // 200-500 GB - yellow
            return 'text-yellow-600 dark:text-yellow-400';
        } else {
            // 500+ GB - red
            return 'text-red-600 dark:text-red-400';
        }
    }
    
    function formatTimeAgo(timestamp) {
        if (!timestamp) return 'Never';
        
        const now = Date.now();
        const backupTime = timestamp * 1000; // Convert to milliseconds
        const diffMs = now - backupTime;
        const diffSeconds = Math.floor(diffMs / 1000);
        const diffMinutes = Math.floor(diffSeconds / 60);
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);
        const diffWeeks = Math.floor(diffDays / 7);
        const diffMonths = Math.floor(diffDays / 30);
        const diffYears = Math.floor(diffDays / 365);
        
        if (diffMinutes < 1) {
            return 'Just now';
        } else if (diffMinutes === 1) {
            return '1m ago';
        } else if (diffMinutes < 60) {
            return `${diffMinutes}m ago`;
        } else if (diffHours === 1) {
            return '1h ago';
        } else if (diffHours < 24) {
            return `${diffHours}h ago`;
        } else if (diffDays === 1) {
            return '1d ago';
        } else if (diffDays < 7) {
            return `${diffDays}d ago`;
        } else if (diffWeeks === 1) {
            return '1w ago';
        } else if (diffWeeks < 4) {
            return `${diffWeeks}w ago`;
        } else if (diffMonths === 1) {
            return '1mo ago';
        } else if (diffMonths < 12) {
            return `${diffMonths}mo ago`;
        } else if (diffYears === 1) {
            return '1y ago';
        } else {
            return `${diffYears}y ago`;
        }
    }
    
    // Public API
    return {
        init,
        updatePVEInfo,
        sortTable
    };
})();