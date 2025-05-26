PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.common = (() => {
    let searchInput = null;
    let backupsSearchInput = null;

    function init() {
        searchInput = document.getElementById('dashboard-search');
        backupsSearchInput = document.getElementById('backups-search');

        setupTableSorting('main-table');
        setupTableSorting('backups-overview-table');

        _setupDashboardFilterListeners();
        _setupBackupFilterListeners();
        _setupResetButtonListeners();
        _setupGlobalKeydownListeners();
        applyInitialFilterUI();
        applyInitialSortUI();
    }

    function applyInitialFilterUI() {
        const groupByNode = PulseApp.state.get('groupByNode');
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');
        const backupsFilterHealth = PulseApp.state.get('backupsFilterHealth');
        const backupsFilterGuestType = PulseApp.state.get('backupsFilterGuestType');

        const groupRadio = document.getElementById(groupByNode ? 'group-grouped' : 'group-list');
        if (groupRadio) groupRadio.checked = true;
        const typeRadio = document.getElementById(`filter-${filterGuestType === 'ct' ? 'lxc' : filterGuestType}`);
        if (typeRadio) typeRadio.checked = true;
        const statusRadio = document.getElementById(`filter-status-${filterStatus}`);
        if (statusRadio) statusRadio.checked = true;
        const backupHealthRadio = document.getElementById(`backups-filter-status-${backupsFilterHealth}`);
        if (backupHealthRadio) backupHealthRadio.checked = true;
        const backupTypeRadio = document.getElementById(`backups-filter-type-${backupsFilterGuestType}`);
        if (backupTypeRadio) backupTypeRadio.checked = true;
    }

    function applyInitialSortUI() {
        const mainSortState = PulseApp.state.getSortState('main');
        const backupsSortState = PulseApp.state.getSortState('backups');

        const initialMainHeader = document.querySelector(`#main-table th[data-sort="${mainSortState.column}"]`);
        if (initialMainHeader) {
          updateSortUI('main-table', initialMainHeader);
        }

        const initialBackupsHeader = document.querySelector(`#backups-overview-table th[data-sort="${backupsSortState.column}"]`);
        if (initialBackupsHeader) {
          updateSortUI('backups-overview-table', initialBackupsHeader);
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
                }
            });
        });

        if (searchInput) {
            const debouncedUpdate = PulseApp.utils.debounce(function() {
                PulseApp.ui.dashboard.updateDashboardTable();
                if (PulseApp.ui.thresholds && typeof PulseApp.ui.thresholds.updateLogControlsVisibility === 'function') {
                    PulseApp.ui.thresholds.updateLogControlsVisibility();
                }
            }, 300);
            
            searchInput.addEventListener('input', debouncedUpdate);
        } else {
            console.warn('Element #dashboard-search not found - text filtering disabled.');
        }
    }

    function _setupBackupFilterListeners() {
        document.querySelectorAll('input[name="backups-type-filter"]').forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.checked) {
                    PulseApp.state.set('backupsFilterGuestType', this.value);
                    PulseApp.ui.backups.updateBackupsTab();
                    PulseApp.state.saveFilterState();
                }
            });
        });

        document.querySelectorAll('input[name="backups-status-filter"]').forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.checked) {
                    PulseApp.state.set('backupsFilterHealth', this.value);
                    PulseApp.ui.backups.updateBackupsTab();
                    PulseApp.state.saveFilterState();
                }
            });
        });
    }

    function _setupResetButtonListeners() {
        const resetButton = document.getElementById('reset-filters-button');
        if (resetButton) {
            resetButton.addEventListener('click', resetDashboardView);
        } else {
            console.warn('Reset button #reset-filters-button not found.');
        }
        // Note: Reset button for backups tab is handled within backups.js init
    }

    function _setupGlobalKeydownListeners() {
        document.addEventListener('keydown', function(event) {
            const activeElement = document.activeElement;
            const isSearchInputFocused = activeElement === searchInput || activeElement === backupsSearchInput;
            const isGeneralInputElement = !isSearchInputFocused && (activeElement.tagName === 'INPUT' || activeElement.tagName === 'TEXTAREA' || activeElement.isContentEditable);
            const isMainActive = document.getElementById('main')?.classList.contains('active');
            const isBackupsActive = document.getElementById('backups')?.classList.contains('active');


            if (event.key === 'Escape') {
                if (isBackupsActive && document.getElementById('backups')?.contains(activeElement)) {
                    PulseApp.ui.backups.resetBackupsView();
                } else if (isMainActive) { // Only reset dashboard if main tab is active and not in backup context
                     resetDashboardView();
                }
            } else if (isSearchInputFocused && event.key === 'Enter') {
              activeElement.blur();
              event.preventDefault();
            } else if (
                !isSearchInputFocused &&
                !isGeneralInputElement &&
                isMainActive && 
                !event.metaKey &&
                !event.ctrlKey &&
                !event.altKey &&
                event.key.length === 1 &&
                event.key !== ' '
            ) {
                if (searchInput) { // Focus dashboard search if main tab is active
                    searchInput.focus();
                }
            } else if (
                !isSearchInputFocused &&
                !isGeneralInputElement &&
                isBackupsActive &&
                 !event.metaKey &&
                !event.ctrlKey &&
                !event.altKey &&
                event.key.length === 1 &&
                event.key !== ' '
            ) {
                 if (backupsSearchInput) { // Focus backups search if backups tab is active
                    backupsSearchInput.focus();
                }
            }
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
         } else if (tableId.startsWith('backups-')) {
             derivedKey = 'backups';
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
          th.addEventListener('click', () => {
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
                case 'backups':
                    PulseApp.ui.backups.updateBackupsTab();
                    break;
                default:
                    console.error('Unknown table type for sorting update:', tableType);
            }

            updateSortUI(tableId, th);
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

        // Reset thresholds
        PulseApp.ui.thresholds.resetThresholds(); // This will also trigger a save

        // Update table and save states
        PulseApp.ui.dashboard.updateDashboardTable();
        PulseApp.state.saveFilterState(); // Thresholds are saved by its own reset
        // Sort state is not reset by this action intentionally
    }

    function generateNodeGroupHeaderCellHTML(text, colspan, cellTag = 'td') {
        const cellClasses = 'py-0.5 px-2 bg-gray-200 dark:bg-gray-700 subtle-stripes-light dark:subtle-stripes-dark text-left font-medium text-sm text-gray-700 dark:text-gray-300';
        return `<${cellTag} colspan="${colspan}" class="${cellClasses}">${text}</${cellTag}>`;
    }

    return {
        init,
        updateSortUI,
        setupTableSorting,
        resetDashboardView,
        generateNodeGroupHeaderCellHTML
    };
})();
