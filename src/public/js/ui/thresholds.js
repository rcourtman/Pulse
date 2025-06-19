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

        // Set initial dimmed state for filter row
        if (thresholdRow) {
            thresholdRow.classList.add('dimmed');
        }

        applyInitialThresholdUI();
        updateThresholdIndicator();
        
        // Apply initial threshold dimming - thresholds are always active now
        const thresholdState = PulseApp.state.getThresholdState();
        const hasActiveThresholds = Object.values(thresholdState).some(t => t && t.value > 0);
        if (hasActiveThresholds) {
            updateRowStylingOnly(thresholdState);
        }

        _setupSliderListeners();
        _setupSelectListeners();
        _setupDragEndListeners();
        _setupResetButtonListener();
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
            
            // Apply dimming directly to row
            if (!thresholdsMet) {
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-dimmed', 'true');
            } else {
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-dimmed');
            }
        });
    }
    
    function clearAllRowDimming() {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-dimmed]');
        rows.forEach(row => {
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-dimmed');
        });
    }
    
    function clearAllStyling() {
        const tableBody = document.querySelector('#main-table tbody');
        if (!tableBody) return;
        
        // Clear ALL rows - both threshold and alert dimming
        const rows = tableBody.querySelectorAll('tr[data-id]');
        rows.forEach(row => {
            // Clear row-level styling
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-dimmed');
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
            
            // Apply threshold dimming immediately
            if (!thresholdsMet) {
                row.style.opacity = '0.4';
                row.setAttribute('data-dimmed', 'true');
            } else {
                row.style.opacity = '';
                row.removeAttribute('data-dimmed');
            }
        });
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

    function updateThresholdIndicator() {
        const thresholdState = PulseApp.state.getThresholdState();
        const activeCount = _updateThresholdHeaderStyles(thresholdState);
        const activeThresholds = _getActiveThresholds(thresholdState);

        // Update badge
        if (thresholdBadge) {
            if (activeCount > 0) {
                thresholdBadge.textContent = activeCount;
                thresholdBadge.classList.remove('hidden');
            } else {
                thresholdBadge.classList.add('hidden');
            }
        }

        // Dim filter row when no filters are active
        if (thresholdRow) {
            if (activeCount > 0) {
                thresholdRow.classList.remove('dimmed');
            } else {
                thresholdRow.classList.add('dimmed');
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
        
        // Reset UI
        for (const type in sliders) {
            if (sliders[type]) {
                sliders[type].value = 0;
            }
        }
        for (const type in thresholdSelects) {
            if (thresholdSelects[type]) {
                thresholdSelects[type].value = '';
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

    return {
        init,
        resetThresholds,
        isThresholdDragInProgress,
        applyThresholdDimmingNow,
        // Expose helper functions for alerts system
        createThresholdSliderHtml,
        createThresholdSelectHtml,
        setupThresholdSliderEvents
    };
})();