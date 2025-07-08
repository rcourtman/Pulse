PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.common = (() => {
    let searchInput = null;

    function init() {
        searchInput = document.getElementById('dashboard-search');

        setupTableSorting('main-table');

        _setupDashboardFilterListeners();
        _setupResetButtonListeners();
        _setupGlobalKeydownListeners();
        _setupTabSwitchListeners();
        applyInitialFilterUI();
        applyInitialSortUI();
        
        // Initialize reset button state
        setTimeout(() => updateResetButtonState(), 100); // Small delay to ensure all UI is initialized
    }

    function applyInitialFilterUI() {
        const groupByNode = PulseApp.state.get('groupByNode');
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');

        const groupRadio = document.getElementById(groupByNode ? 'group-grouped' : 'group-list');
        if (groupRadio) groupRadio.checked = true;
        const typeRadio = document.getElementById(`filter-${filterGuestType === 'ct' ? 'lxc' : filterGuestType}`);
        if (typeRadio) typeRadio.checked = true;
        const statusRadio = document.getElementById(`filter-status-${filterStatus}`);
        if (statusRadio) statusRadio.checked = true;
    }

    function applyInitialSortUI() {
        const mainSortState = PulseApp.state.getSortState('main');

        const initialMainHeader = document.querySelector(`#main-table th[data-sort="${mainSortState.column}"]`);
        if (initialMainHeader) {
          updateSortUI('main-table', initialMainHeader);
        }

    }

    function _setupDashboardFilterListeners() {
        document.querySelectorAll('input[name="group-filter"]').forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.checked) {
                    PulseApp.state.set('groupByNode', this.value === 'grouped');
                    PulseApp.ui.dashboard.updateDashboardTable();
                    if (searchInput) searchInput.dispatchEvent(new Event('input'));
                    PulseApp.state.saveFilterState();
                    updateResetButtonState();
                    
                }
            });
        });

        document.querySelectorAll('input[name="type-filter"]').forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.checked) {
                    PulseApp.state.set('filterGuestType', this.value);
                    PulseApp.ui.dashboard.updateDashboardTable();
                    if (searchInput) searchInput.dispatchEvent(new Event('input'));
                    PulseApp.state.saveFilterState();
                    if (PulseApp.ui.thresholds && typeof PulseApp.ui.thresholds.updateLogControlsVisibility === 'function') {
                        PulseApp.ui.thresholds.updateLogControlsVisibility();
                    }
                    updateResetButtonState();
                }
            });
        });

        document.querySelectorAll('input[name="status-filter"]').forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.checked) {
                    PulseApp.state.set('filterStatus', this.value);
                    PulseApp.ui.dashboard.updateDashboardTable();
                    if (searchInput) searchInput.dispatchEvent(new Event('input'));
                    PulseApp.state.saveFilterState();
                    if (PulseApp.ui.thresholds && typeof PulseApp.ui.thresholds.updateLogControlsVisibility === 'function') {
                        PulseApp.ui.thresholds.updateLogControlsVisibility();
                    }
                    updateResetButtonState();
                }
            });
        });

        if (searchInput) {
            const debouncedUpdate = PulseApp.utils.debounce(function() {
                PulseApp.ui.dashboard.updateDashboardTable();
                if (PulseApp.ui.thresholds && typeof PulseApp.ui.thresholds.updateLogControlsVisibility === 'function') {
                    PulseApp.ui.thresholds.updateLogControlsVisibility();
                }
                updateResetButtonState();
            }, 300);
            
            searchInput.addEventListener('input', debouncedUpdate);
        } else {
            console.warn('Element #dashboard-search not found - text filtering disabled.');
        }
    }


    function _setupResetButtonListeners() {
        const resetButton = document.getElementById('reset-filters-button');
        if (resetButton) {
            resetButton.addEventListener('click', resetDashboardView);
        } else {
            console.warn('Reset button #reset-filters-button not found.');
        }
    }

    function _setupGlobalKeydownListeners() {
        document.addEventListener('keydown', function(event) {
            const activeElement = document.activeElement;
            const isSearchInputFocused = activeElement === searchInput;
            const isGeneralInputElement = !isSearchInputFocused && (
                activeElement.tagName === 'INPUT' || 
                activeElement.tagName === 'TEXTAREA' || 
                activeElement.isContentEditable
            );

            // Handle Escape key
            if (event.key === 'Escape') {
                handleEscapeKey();
            } 
            // Handle Enter key in search inputs
            else if (isSearchInputFocused && event.key === 'Enter') {
                activeElement.blur();
                event.preventDefault();
            } 
            // Handle typing characters for auto-focus search inputs
            else if (
                !isSearchInputFocused &&
                !isGeneralInputElement &&
                !event.metaKey &&
                !event.ctrlKey &&
                !event.altKey &&
                !event.shiftKey &&
                event.key.length === 1 &&
                event.key !== ' ' &&
                /[a-zA-Z0-9]/.test(event.key) // Only alphanumeric characters
            ) {
                const mainTab = document.getElementById('main');
                
                if (mainTab && !mainTab.classList.contains('hidden') && searchInput) {
                    // Main tab is visible
                    searchInput.focus();
                    searchInput.value = event.key; // Set the typed character immediately
                    event.preventDefault(); // Prevent the character from being typed twice
                }
            }
        });
    }

    function _setupTabSwitchListeners() {
        // Listen for tab clicks to clear search inputs when switching tabs
        document.querySelectorAll('.tab[data-tab]').forEach(tab => {
            tab.addEventListener('click', function() {
                const targetTab = this.getAttribute('data-tab');
                
                // Clear search input when switching tabs
                if (targetTab !== 'main' && searchInput && searchInput.value) {
                    // Switching away from main tab, clear main search if it has content
                    searchInput.value = '';
                    // Trigger dashboard update to clear filtered results
                    if (PulseApp.ui && PulseApp.ui.dashboard) {
                        PulseApp.ui.dashboard.updateDashboardTable();
                    }
                }
            });
        });
    }

    function updateSortUI(tableId, clickedHeader, explicitKey = null) {
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
         } else {
             derivedKey = null;
         }

        const tableKey = explicitKey || derivedKey;
        if (!tableKey) {
            console.error(`[updateSortUI] Could not determine sort key for tableId: ${tableId}`);
            return;
        }

        const currentSort = PulseApp.state.getSortState(tableKey);
        if (!currentSort) {
            console.error(`[updateSortUI] No sort state found for key: '${tableKey}'`);
            return;
        }

        const headers = tableElement.querySelectorAll('th.sortable');
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
        const tableTypeMatch = tableId.match(/^([a-zA-Z]+)-/);
        if (!tableTypeMatch) {
            console.warn(`Could not determine table type from ID: ${tableId}`);
            return;
        }
        const tableType = tableTypeMatch[1];

        tableElement.querySelectorAll('th.sortable').forEach(th => {
          // Make sortable headers keyboard accessible
          th.setAttribute('tabindex', '0');
          th.setAttribute('role', 'button');
          th.setAttribute('aria-label', `Sort by ${th.textContent.trim()}`);
          
          
          const handleSort = () => {
            const column = th.getAttribute('data-sort');
            if (!column) return;

            const currentSortState = PulseApp.state.getSortState(tableType);
            let newDirection = 'asc';
            if (currentSortState && currentSortState.column === column) {
                newDirection = currentSortState.direction === 'asc' ? 'desc' : 'asc';
            }

            PulseApp.state.setSortState(tableType, column, newDirection);

            switch(tableType) {
                case 'main':
                    PulseApp.ui.dashboard.updateDashboardTable();
                    break;
                default:
                    console.error('Unknown table type for sorting update:', tableType);
            }

            updateSortUI(tableId, th);
          };
          
          th.addEventListener('click', handleSort);
          th.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleSort();
            }
          });
        });
    }

    function resetDashboardView() {
        // Reset search
        if (searchInput) searchInput.value = '';

        // Reset filters to defaults
        PulseApp.state.set('groupByNode', true);
        document.getElementById('group-grouped').checked = true;
        PulseApp.state.set('filterGuestType', 'all');
        document.getElementById('filter-all').checked = true;
        PulseApp.state.set('filterStatus', 'all');
        document.getElementById('filter-status-all').checked = true;

        // Note: Thresholds are now handled by their own dedicated reset button

        // Update table and save states
        PulseApp.ui.dashboard.updateDashboardTable();
        PulseApp.state.saveFilterState();
        // Sort state is not reset by this action intentionally
        
        // Update reset button highlighting
        updateResetButtonState();
    }
    
    function hasActiveFilters() {
        // Check search input
        if (searchInput && searchInput.value.trim() !== '') return true;
        
        // Check filters
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const groupByNode = PulseApp.state.get('groupByNode');
        
        if (filterGuestType !== 'all' || filterStatus !== 'all' || groupByNode !== true) return true;
        
        // Note: Thresholds are no longer included - they have their own reset button
        
        return false;
    }
    
    function updateResetButtonState() {
        const resetButton = document.getElementById('reset-filters-button');
        if (!resetButton) return;
        
        const hasActiveStates = hasActiveFilters();
        
        if (hasActiveStates) {
            resetButton.className = 'flex items-center justify-center p-1 h-11 w-11 sm:h-7 sm:w-7 text-xs border border-blue-400 dark:border-blue-500 rounded bg-blue-50/50 dark:bg-blue-900/10 text-blue-600 dark:text-blue-400 hover:bg-blue-100/50 dark:hover:bg-blue-900/20 focus:outline-none focus:ring-2 focus:ring-blue-400 transition-colors flex-shrink-0';
        } else {
            // Default state - button is inactive
            resetButton.className = 'flex items-center justify-center p-1 h-11 w-11 sm:h-7 sm:w-7 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none transition-colors flex-shrink-0';
        }
    }

    function generateNodeGroupHeaderCellHTML(text, colspan, cellTag = 'td') {
        const baseClasses = 'px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400';
        
        // Check if we can make this node name clickable
        const hostUrl = PulseApp.utils.getHostUrl(text);
        let nodeContent = text;
        
        if (hostUrl) {
            nodeContent = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer" title="Open ${text} web interface">${text}</a>`;
        }
        
        // Always create individual cells so first one can be sticky
        // Match the exact structure from backups tab that works
        let html = `<${cellTag} class="sticky left-0 z-10 ${baseClasses} bg-gray-50 dark:bg-gray-700/50">${nodeContent}</${cellTag}>`;
        // Add empty cells for remaining columns
        for (let i = 1; i < colspan; i++) {
            html += `<${cellTag} class="${baseClasses}"></${cellTag}>`;
        }
        return html;
    }

    function addTableFixedLine(containerSelector, columnWidthVar) {
        // No longer needed - using CSS border styling instead
    }
    
    function createTableRow(options = {}) {
        const {
            classes = '',
            baseClasses = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700',
            isSpecialRow = false,
            specialBgClass = '',
            specialHoverClass = ''
        } = options;
        
        const row = document.createElement('tr');
        
        if (isSpecialRow && specialBgClass) {
            row.className = `${baseClasses} ${specialBgClass} ${specialHoverClass} ${classes}`.trim();
        } else {
            row.className = `${baseClasses} ${classes}`.trim();
        }
        
        return row;
    }
    
    function createStickyColumn(content, options = {}) {
        const {
            tag = 'td',
            title = '',
            additionalClasses = '',
            padding = 'py-1 px-2',
            includeTextColor = true
        } = options;
        
        const element = document.createElement(tag);
        // Add grey text color for td elements unless already has text color or disabled
        const textColorClass = includeTextColor && tag === 'td' && !additionalClasses.includes('text-') ? 'text-gray-700 dark:text-gray-300' : '';
        element.className = `sticky left-0 z-10 ${padding} align-middle whitespace-nowrap overflow-hidden text-ellipsis max-w-0 ${textColorClass} ${additionalClasses}`.trim();
        
        if (title) {
            element.title = title;
        }
        
        if (typeof content === 'string') {
            element.innerHTML = content;
        } else {
            element.appendChild(content);
        }
        
        return element;
    }
    
    function createTableCell(content, classes = 'py-1 px-2 align-middle', includeTextColor = true) {
        const cell = document.createElement('td');
        // Add default grey text color unless explicitly disabled
        const textColorClass = includeTextColor && !classes.includes('text-') ? 'text-gray-700 dark:text-gray-300' : '';
        cell.className = `${classes} ${textColorClass}`.trim();
        cell.innerHTML = content;
        return cell;
    }

    function handleEscapeKey() {
        // Determine which tab is active
        const activeTabContent = document.querySelector('.tab-content:not(.hidden)');
        if (!activeTabContent) return;
        
        const activeTabId = activeTabContent.id;
        
        switch (activeTabId) {
            case 'main':
                // Dashboard tab
                resetDashboardView();
                break;
                
            case 'storage':
                // Storage tab - reset sorting
                if (PulseApp.ui.storage && PulseApp.ui.storage.resetSort) {
                    PulseApp.ui.storage.resetSort();
                }
                break;
                
            case 'backups':
                // Backups tab - reset search, filters, and sorting
                if (PulseApp.ui.backups && PulseApp.ui.backups.resetFiltersAndSort) {
                    PulseApp.ui.backups.resetFiltersAndSort();
                }
                break;
                
            case 'snapshots':
                // Snapshots tab - reset search, filters, and sorting
                if (PulseApp.ui.snapshots && PulseApp.ui.snapshots.resetFiltersAndSort) {
                    PulseApp.ui.snapshots.resetFiltersAndSort();
                }
                break;
                
            case 'pbs':
                // PBS tab - reset time range and selections
                if (PulseApp.ui.pbs && PulseApp.ui.pbs.resetToDefaults) {
                    PulseApp.ui.pbs.resetToDefaults();
                }
                break;
                
            case 'nodes':
                // Nodes tab - reset sorting if available
                if (PulseApp.ui.nodes && PulseApp.ui.nodes.resetSort) {
                    PulseApp.ui.nodes.resetSort();
                }
                break;
                
            case 'settings':
                // Settings tab - nothing to reset
                break;
        }
    }

    return {
        init,
        updateSortUI,
        setupTableSorting,
        resetDashboardView,
        generateNodeGroupHeaderCellHTML,
        updateResetButtonState,
        hasActiveFilters,
        createTableRow,
        createStickyColumn,
        createTableCell
    };
})();
