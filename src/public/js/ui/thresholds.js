PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.thresholds = (() => {
    let thresholdRow = null;
    let thresholdBadge = null;
    let sliders = {};
    let thresholdSelects = {};
    let isDraggingSlider = false;

    function init() {
        thresholdRow = document.getElementById('threshold-slider-row');
        thresholdBadge = document.getElementById('threshold-count-badge');
        const thresholdSettingsPanel = document.getElementById('threshold-filter-controls');

        sliders = {
            cpu: document.getElementById('threshold-slider-cpu'),
            memory: document.getElementById('threshold-slider-memory'),
            disk: document.getElementById('threshold-slider-disk'),
        };
        thresholdSelects = {
            diskread: document.getElementById('threshold-select-diskread'),
            diskwrite: document.getElementById('threshold-select-diskwrite'),
            netin: document.getElementById('threshold-select-netin'),
            netout: document.getElementById('threshold-select-netout'),
        };

        // Initially hide the threshold row, badge and settings panel
        if (thresholdRow) {
            thresholdRow.classList.add('hidden');
        }
        if (thresholdBadge) {
            thresholdBadge.classList.add('hidden');
        }
        if (thresholdSettingsPanel) {
            thresholdSettingsPanel.classList.add('hidden');
        }

        applyInitialThresholdUI();
        // Don't call updateThresholdIndicator on init - it might show elements
        
        // Don't apply threshold dimming on init - wait for toggle to be checked

        _setupSliderListeners();
        _setupSelectListeners();
        _setupDragEndListeners();
        _setupResetButtonListener();
        _setupHideModeListener();
        _setupThresholdToggleListener();
    }

    function applyInitialThresholdUI() {
        const thresholdState = PulseApp.state.getThresholdState();
        for (const type in thresholdState) {
            if (sliders[type]) {
                const sliderElement = sliders[type];
                if (sliderElement) sliderElement.value = thresholdState[type].value;
            } else if (thresholdSelects[type]) {
                const selectElement = thresholdSelects[type];
                if (selectElement) selectElement.value = thresholdState[type].value;
            }
        }
    }

    function _handleThresholdDragStart(event) {
        PulseApp.tooltips.updateSliderTooltip(event.target);
        isDraggingSlider = true;
        if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
            PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
        }
    }

    function _handleThresholdDragEnd() {
        if (isDraggingSlider) {
            isDraggingSlider = false;
            if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.clearGuestMetricSnapshots) {
                PulseApp.ui.dashboard.clearGuestMetricSnapshots();
            }
        }
    }

    function _setupSliderListeners() {
        for (const type in sliders) {
            const sliderElement = sliders[type];
            if (sliderElement) {
                sliderElement.addEventListener('input', (event) => {
                    const value = event.target.value;
                    updateThreshold(type, value);
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
                sliderElement.addEventListener('mousedown', _handleThresholdDragStart);
                sliderElement.addEventListener('touchstart', _handleThresholdDragStart, { passive: true });
            } else {
                console.warn(`Slider element not found for type: ${type}`);
            }
        }
    }

    function _setupSelectListeners() {
        for (const type in thresholdSelects) {
            const selectElement = thresholdSelects[type];
            if (selectElement) {
                selectElement.addEventListener('change', (event) => {
                    const value = event.target.value;
                    updateThreshold(type, value);
                });
            } else {
                console.warn(`Select element not found for type: ${type}`);
            }
        }
    }

    function _setupDragEndListeners() {
        document.addEventListener('mouseup', _handleThresholdDragEnd);
        document.addEventListener('touchend', _handleThresholdDragEnd);
    }

    function _setupResetButtonListener() {
        const resetButton = document.getElementById('reset-thresholds');
        if (resetButton) {
            resetButton.addEventListener('click', resetThresholds);
        }
    }

    function _setupHideModeListener() {
        const hideModeCheckbox = document.getElementById('threshold-hide-mode');
        if (hideModeCheckbox) {
            // Set initial state from saved preference
            hideModeCheckbox.checked = PulseApp.state.get('thresholdHideMode') || false;
            
            hideModeCheckbox.addEventListener('change', (event) => {
                PulseApp.state.set('thresholdHideMode', event.target.checked);
                // Immediately update the display
                updateDashboardFromThreshold();
            });
        }
    }

    function _setupThresholdToggleListener() {
        const thresholdToggle = document.getElementById('toggle-thresholds-checkbox');
        if (thresholdToggle) {
            thresholdToggle.addEventListener('change', (event) => {
                const isChecked = event.target.checked;
                
                if (isChecked) {
                    // Clear any existing styling first
                    clearAllStyling();
                    
                    // Show threshold row
                    if (thresholdRow) thresholdRow.classList.remove('hidden');
                    
                    // Check if there are active thresholds to show settings panel
                    const thresholdState = PulseApp.state.getThresholdState();
                    const hasActiveThresholds = Object.values(thresholdState).some(t => t && t.value > 0);
                    const thresholdSettingsPanel = document.getElementById('threshold-filter-controls');
                    if (thresholdSettingsPanel && hasActiveThresholds) {
                        thresholdSettingsPanel.classList.remove('hidden');
                    }
                    
                    // Disable other modes (mutually exclusive)
                    const chartsToggle = document.getElementById('toggle-charts-checkbox');
                    const alertsToggle = document.getElementById('toggle-alerts-checkbox');
                    if (chartsToggle && chartsToggle.checked) {
                        chartsToggle.checked = false;
                        chartsToggle.dispatchEvent(new Event('change'));
                    }
                    if (alertsToggle && alertsToggle.checked) {
                        alertsToggle.checked = false;
                        alertsToggle.dispatchEvent(new Event('change'));
                    }
                    
                    // Apply thresholds if any are active
                    if (hasActiveThresholds) {
                        updateRowStylingOnly(thresholdState);
                    }
                } else {
                    // Hide threshold row and settings
                    if (thresholdRow) thresholdRow.classList.add('hidden');
                    const thresholdSettingsPanel = document.getElementById('threshold-filter-controls');
                    if (thresholdSettingsPanel) thresholdSettingsPanel.classList.add('hidden');
                    
                    // Clear all threshold styling when toggle is turned off
                    clearAllStyling();
            
            // Also clear any alert-specific styling
            const rows = document.querySelectorAll('#main-table tbody tr[data-id]');
            rows.forEach(row => {
                row.removeAttribute('data-alert-dimmed');
                row.removeAttribute('data-alert-mixed');
            });
                    
                    // Reset threshold header styles to default
                    resetThresholdHeaderStyles();
                    
                    // Hide any lingering tooltips
                    if (PulseApp.tooltips) {
                        if (PulseApp.tooltips.hideTooltip) {
                            PulseApp.tooltips.hideTooltip();
                        }
                        if (PulseApp.tooltips.hideSliderTooltipImmediately) {
                            PulseApp.tooltips.hideSliderTooltipImmediately();
                        }
                    }
                }
            });
        }
    }


    function updateThreshold(type, value, immediate = false) {
        PulseApp.state.setThresholdValue(type, value);
        updateThresholdIndicator();

        // Always update dashboard immediately for live responsiveness
        updateDashboardFromThreshold();
        
        // Update reset button highlighting
        if (PulseApp.ui.common && PulseApp.ui.common.updateResetButtonState) {
            PulseApp.ui.common.updateResetButtonState();
        }
    }

    function updateDashboardFromThreshold() {
        // Only apply thresholds if the toggle is checked
        const thresholdToggle = document.getElementById('toggle-thresholds-checkbox');
        if (!thresholdToggle || !thresholdToggle.checked) {
            clearAllRowDimming();
            return;
        }
        
        // Fast path: just update row styling without full table rebuild
        const thresholdState = PulseApp.state.getThresholdState();
        const hasActiveThresholds = Object.values(thresholdState).some(state => state && state.value > 0);
        
        if (hasActiveThresholds) {
            updateRowStylingOnly(thresholdState);
        } else {
            // No thresholds active, remove all dimming
            clearAllRowDimming();
        }
    }
    
    function updateRowStylingOnly(thresholdState) {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        let hiddenCount = 0;
        
        // First, show all rows to reset the state
        rows.forEach(row => {
            row.style.display = '';
        });
        
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            if (!guestId) return;
            
            // Clear any existing alert styling first
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-alert-mixed');
            const cells = row.querySelectorAll('td');
            cells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.removeAttribute('data-alert-custom');
            });
            
            // Find guest data by id
            const guest = dashboardData.find(g => g.id === guestId);
            if (!guest) return;
            
            // Check if guest meets thresholds
            let thresholdsMet = true;
            for (const type in thresholdState) {
                const state = thresholdState[type];
                if (!state || state.value <= 0) continue;
                
                let guestValue;
                if (type === 'cpu') guestValue = guest.cpu;
                else if (type === 'memory') guestValue = guest.memory;
                else if (type === 'disk') guestValue = guest.disk;
                else if (type === 'diskread') guestValue = guest.diskread;
                else if (type === 'diskwrite') guestValue = guest.diskwrite;
                else if (type === 'netin') guestValue = guest.netin;
                else if (type === 'netout') guestValue = guest.netout;
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
            
            // Apply dimming or hiding based on mode
            const hideMode = PulseApp.state.get('thresholdHideMode');
            if (!thresholdsMet) {
                if (hideMode) {
                    row.style.display = 'none';
                    row.setAttribute('data-threshold-hidden', 'true');
                    hiddenCount++;
                } else {
                    row.style.opacity = '0.4';
                    row.style.transition = 'opacity 0.1s ease-out';
                    row.setAttribute('data-dimmed', 'true');
                }
            } else {
                row.style.display = '';
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-dimmed');
                row.removeAttribute('data-threshold-hidden');
            }
        });
        
        // Handle node headers when in hide mode
        const hideMode = PulseApp.state.get('thresholdHideMode');
        if (hideMode) {
            updateNodeHeaderVisibility();
        }
        
        // Update the status message to reflect hidden guests
        if (hideMode && hiddenCount > 0) {
            updateStatusMessageForHiddenGuests(hiddenCount);
        }
    }
    
    function clearAllRowDimming() {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        // Clear ALL rows with data-id attribute (guest rows)
        const rows = tableBody.querySelectorAll('tr[data-id]');
        rows.forEach(row => {
            row.style.display = '';
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-dimmed');
            row.removeAttribute('data-threshold-hidden');
        });
        
        // Also show all node headers
        const nodeHeaders = tableBody.querySelectorAll('tr.node-header');
        nodeHeaders.forEach(header => {
            header.style.display = '';
        });
    }
    
    function clearAllStyling() {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        // Clear ALL rows - both threshold and alert dimming
        const rows = tableBody.querySelectorAll('tr[data-id]');
        rows.forEach(row => {
            // Clear row-level styling
            row.style.display = '';
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-dimmed');
            row.removeAttribute('data-threshold-hidden');
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-alert-mixed');
            
            // Clear cell-level styling that alerts might have applied
            const cells = row.querySelectorAll('td');
            cells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.removeAttribute('data-alert-custom');
            });
        });
    }
    
    function applyThresholdDimmingFast(thresholdState) {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        let hiddenCount = 0;
        
        // First, show all rows to reset the state
        rows.forEach(row => {
            row.style.display = '';
        });
        
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            if (!guestId) return;
            
            // Clear alert styling inline
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-alert-mixed');
            
            const guest = dashboardData.find(g => g.id === guestId);
            if (!guest) return;
            
            // Check thresholds
            let thresholdsMet = true;
            for (const type in thresholdState) {
                const state = thresholdState[type];
                if (!state || state.value <= 0) continue;
                
                let guestValue;
                if (type === 'cpu') guestValue = guest.cpu;
                else if (type === 'memory') guestValue = guest.memory;
                else if (type === 'disk') guestValue = guest.disk;
                else if (type === 'diskread') guestValue = guest.diskread;
                else if (type === 'diskwrite') guestValue = guest.diskwrite;
                else if (type === 'netin') guestValue = guest.netin;
                else if (type === 'netout') guestValue = guest.netout;
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
            
            // Apply threshold dimming or hiding immediately
            const hideMode = PulseApp.state.get('thresholdHideMode');
            if (!thresholdsMet) {
                if (hideMode) {
                    row.style.display = 'none';
                    row.setAttribute('data-threshold-hidden', 'true');
                    hiddenCount++;
                } else {
                    row.style.opacity = '0.4';
                    row.setAttribute('data-dimmed', 'true');
                }
            } else {
                row.style.display = '';
                row.style.opacity = '';
                row.removeAttribute('data-dimmed');
                row.removeAttribute('data-threshold-hidden');
            }
        });
        
        // Handle node headers when in hide mode
        const hideMode = PulseApp.state.get('thresholdHideMode');
        if (hideMode) {
            updateNodeHeaderVisibility();
        }
        
        // Update the status message to reflect hidden guests
        if (hideMode && hiddenCount > 0) {
            updateStatusMessageForHiddenGuests(hiddenCount);
        }
    }


    function _updateThresholdHeaderStyles(thresholdState) {
        const mainTableHeader = document.querySelector('#main-table thead');
        if (!mainTableHeader) return 0; // Return 0 active count if header not found

        let activeCount = 0;
        const defaultColorClasses = ['text-gray-600', 'dark:text-gray-300'];
        const activeColorClasses = ['text-blue-600', 'dark:text-blue-400'];

        for (const type in thresholdState) {
            const headerCell = mainTableHeader.querySelector(`th[data-sort="${type}"]`);
            if (!headerCell) continue; // Skip if header cell for this type doesn't exist

            if (thresholdState[type].value > 0) {
                activeCount++;
                headerCell.classList.add('threshold-active-header');
                headerCell.classList.remove(...defaultColorClasses);
                headerCell.classList.add(...activeColorClasses);
            } else {
                headerCell.classList.remove('threshold-active-header');
                headerCell.classList.remove(...activeColorClasses);
                headerCell.classList.add(...defaultColorClasses);
            }
        }
        return activeCount;
    }
    
    function resetThresholdHeaderStyles() {
        const mainTableHeader = document.querySelector('#main-table thead');
        if (!mainTableHeader) return;

        const defaultColorClasses = ['text-gray-600', 'dark:text-gray-300'];
        const activeColorClasses = ['text-blue-600', 'dark:text-blue-400'];
        const thresholdTypes = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];

        for (const type of thresholdTypes) {
            const headerCell = mainTableHeader.querySelector(`th[data-sort="${type}"]`);
            if (!headerCell) continue;

            headerCell.classList.remove('threshold-active-header');
            headerCell.classList.remove(...activeColorClasses);
            headerCell.classList.add(...defaultColorClasses);
        }
    }

    function updateThresholdIndicator() {
        const thresholdToggle = document.getElementById('toggle-thresholds-checkbox');
        const thresholdState = PulseApp.state.getThresholdState();
        
        // Only update header styles and count if threshold toggle is checked
        let activeCount = 0;
        if (thresholdToggle && thresholdToggle.checked) {
            activeCount = _updateThresholdHeaderStyles(thresholdState);
        }
        
        const activeThresholds = _getActiveThresholds(thresholdState);

        // Update badge only if threshold toggle is checked
        if (thresholdBadge) {
            if (thresholdToggle && thresholdToggle.checked && activeCount > 0) {
                thresholdBadge.textContent = activeCount;
                thresholdBadge.classList.remove('hidden');
            } else {
                thresholdBadge.classList.add('hidden');
            }
        }

        // Update threshold row dimming only if thresholds toggle is checked
        if (thresholdToggle && thresholdToggle.checked && thresholdRow) {
            if (activeCount > 0) {
                thresholdRow.classList.remove('dimmed');
            } else {
                thresholdRow.classList.add('dimmed');
            }
        }

        // Show/hide settings panel based on active thresholds only if toggle is checked
        const thresholdSettingsPanel = document.getElementById('threshold-filter-controls');
        if (thresholdToggle && thresholdToggle.checked && thresholdSettingsPanel) {
            if (activeCount > 0) {
                thresholdSettingsPanel.classList.remove('hidden');
            } else {
                thresholdSettingsPanel.classList.add('hidden');
            }
        }
    }

    function _getActiveThresholds(thresholdState) {
        const activeThresholds = {};
        for (const type in thresholdState) {
            if (thresholdState[type] && thresholdState[type].value > 0) {
                activeThresholds[type] = thresholdState[type];
            }
        }
        return activeThresholds;
    }

    function resetThresholds() {
        // Reset state
        PulseApp.state.resetThresholds();
        
        // Reset hide mode checkbox
        const hideModeCheckbox = document.getElementById('threshold-hide-mode');
        if (hideModeCheckbox) {
            hideModeCheckbox.checked = false;
            PulseApp.state.set('thresholdHideMode', false);
        }
        
        // Reset UI
        for (const type in sliders) {
            if (sliders[type]) {
                sliders[type].value = 0;
            }
        }
        for (const type in thresholdSelects) {
            if (thresholdSelects[type]) {
                thresholdSelects[type].value = '0';
            }
        }
        
        // Update UI
        updateThresholdIndicator();
        clearAllRowDimming();
        
        // Update reset button highlighting
        if (PulseApp.ui.common && PulseApp.ui.common.updateResetButtonState) {
            PulseApp.ui.common.updateResetButtonState();
        }
        
        PulseApp.ui.toast?.success('Thresholds reset');
    }

    function isThresholdDragInProgress() {
        return isDraggingSlider;
    }

    // Helper function to create threshold slider HTML (used by alerts system)
    function createThresholdSliderHtml(id, min, max, step, value, additionalClasses = '') {
        return `
            <div class="flex flex-row items-center gap-1 ${additionalClasses}">
                <input type="range" 
                       id="${id}"
                       min="${min}" 
                       max="${max}" 
                       step="${step}" 
                       value="${value || min}"
                       class="custom-slider w-full h-3 rounded-full bg-gray-300 dark:bg-gray-600 appearance-none cursor-pointer flex-grow">
            </div>
        `;
    }

    // Helper function to create threshold select HTML (used by alerts system)
    function createThresholdSelectHtml(id, options, value, additionalClasses = '') {
        const optionsHtml = options.map(option => 
            `<option value="${option.value}" ${option.value === value ? 'selected' : ''}>${option.label}</option>`
        ).join('');
        
        return `
            <select id="${id}" 
                    class="threshold-select px-1 py-0 h-5 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-[10px] w-full focus:outline-none focus:ring-1 focus:ring-blue-500 ${additionalClasses}">
                ${optionsHtml}
            </select>
        `;
    }

    // Helper function to setup threshold slider events on any slider element
    function setupThresholdSliderEvents(sliderElement, onChangeCallback) {
        if (!sliderElement) return;
        
        sliderElement.addEventListener('input', (event) => {
            const value = event.target.value;
            if (onChangeCallback) onChangeCallback(value, event.target);
            PulseApp.tooltips.updateSliderTooltip(event.target);
        });
        
        sliderElement.addEventListener('mousedown', (event) => {
            PulseApp.tooltips.updateSliderTooltip(event.target);
            if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
            }
        });
        
        sliderElement.addEventListener('touchstart', (event) => {
            PulseApp.tooltips.updateSliderTooltip(event.target);
            if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
            }
        }, { passive: true });
    }

    function applyThresholdDimmingNow() {
        const thresholdState = PulseApp.state.getThresholdState();
        const hasActiveThresholds = Object.values(thresholdState).some(t => t && t.value > 0);
        if (hasActiveThresholds) {
            applyThresholdDimmingFast(thresholdState);
        }
    }

    function updateStatusMessageForHiddenGuests(hiddenCount) {
        const statusElement = document.getElementById('dashboard-status-text');
        if (!statusElement) return;
        
        const currentText = statusElement.textContent;
        const hideMode = PulseApp.state.get('thresholdHideMode');
        
        if (hideMode && hiddenCount > 0) {
            // Add hidden count info to the status text if not already present
            if (!currentText.includes('hidden')) {
                const totalRows = document.querySelectorAll('#main-table tbody tr[data-id]').length;
                const visibleRows = totalRows - hiddenCount;
                
                // Update the showing count to reflect visible rows
                const updatedText = currentText.replace(
                    /Showing (\d+) guests/,
                    `Showing ${visibleRows} guests (${hiddenCount} hidden)`
                );
                statusElement.textContent = updatedText;
            }
        }
    }

    function updateNodeHeaderVisibility() {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        const nodeHeaders = tableBody.querySelectorAll('tr.node-header');
        
        nodeHeaders.forEach(header => {
            let hasVisibleGuests = false;
            let sibling = header.nextElementSibling;
            
            // Check all following rows until we hit another node header or end of table
            while (sibling && !sibling.classList.contains('node-header')) {
                if (sibling.getAttribute('data-id') && sibling.style.display !== 'none') {
                    hasVisibleGuests = true;
                    break;
                }
                sibling = sibling.nextElementSibling;
            }
            
            // Hide or show the node header based on whether it has visible guests
            if (!hasVisibleGuests) {
                header.style.display = 'none';
            } else {
                header.style.display = '';
            }
        });
    }

    return {
        init,
        resetThresholds,
        isThresholdDragInProgress,
        applyThresholdDimmingNow,
        clearAllRowDimming,
        clearAllStyling,
        // Expose helper functions for alerts system
        createThresholdSliderHtml,
        createThresholdSelectHtml,
        setupThresholdSliderEvents
    };
})();