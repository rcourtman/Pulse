PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.thresholds = (() => {
    let thresholdRow = null;
    let toggleThresholdsButton = null;
    let thresholdBadge = null;
    let sliders = {};
    let thresholdSelects = {};
    let isDraggingSlider = false;

    function init() {
        thresholdRow = document.getElementById('threshold-slider-row');
        toggleThresholdsButton = document.getElementById('toggle-thresholds-button');
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

        applyInitialThresholdUI();
        updateThresholdIndicator();
        updateThresholdRowVisibility();

        if (toggleThresholdsButton) {
            toggleThresholdsButton.addEventListener('click', () => {
                PulseApp.state.set('isThresholdRowVisible', !PulseApp.state.get('isThresholdRowVisible'));
                updateThresholdRowVisibility();
            });
        } else {
            console.warn('#toggle-thresholds-button not found.');
        }

        _setupSliderListeners();
        _setupSelectListeners();
        _setupDragEndListeners();
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
        // Listen globally for drag end events
        document.addEventListener('mouseup', _handleThresholdDragEnd);
        document.addEventListener('touchend', _handleThresholdDragEnd);
    }

    function updateThreshold(type, value) {
        PulseApp.state.setThresholdValue(type, value);

        if (PulseApp.ui && PulseApp.ui.dashboard) {
            PulseApp.ui.dashboard.updateDashboardTable();
        } else {
            console.warn('[Thresholds] PulseApp.ui.dashboard not available for updateDashboardTable');
        }
        updateThresholdIndicator();
    }

    function updateThresholdRowVisibility() {
        const isVisible = PulseApp.state.get('isThresholdRowVisible');
        if (thresholdRow) {
            thresholdRow.classList.toggle('hidden', !isVisible);
            if (toggleThresholdsButton) {
                toggleThresholdsButton.classList.toggle('bg-blue-100', isVisible);
                toggleThresholdsButton.classList.toggle('dark:bg-blue-800/50', isVisible);
                toggleThresholdsButton.classList.toggle('text-blue-700', isVisible);
                toggleThresholdsButton.classList.toggle('dark:text-blue-300', isVisible);
            }
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

    function updateThresholdIndicator() {
        if (!thresholdBadge) return;

        const thresholdState = PulseApp.state.getThresholdState();
        const activeCount = _updateThresholdHeaderStyles(thresholdState);

        if (activeCount > 0) {
            thresholdBadge.textContent = activeCount;
            thresholdBadge.classList.remove('hidden');
        } else {
            thresholdBadge.classList.add('hidden');
        }
    }

    function resetThresholds() {
        const thresholdState = PulseApp.state.getThresholdState();
        for (const type in thresholdState) {
             PulseApp.state.setThresholdValue(type, 0);
            if (sliders[type]) {
                const sliderElement = sliders[type];
                if (sliderElement) sliderElement.value = 0;
            } else if (thresholdSelects[type]) {
                const selectElement = thresholdSelects[type];
                if (selectElement) selectElement.value = 0;
            }
        }
        PulseApp.tooltips.hideSliderTooltip();
        PulseApp.state.set('isThresholdRowVisible', false);
        updateThresholdRowVisibility();
        updateThresholdIndicator();
    }

    // Getter for dashboard.js to check drag state
    function isThresholdDragInProgress() {
        return isDraggingSlider;
    }

    return {
        init,
        resetThresholds,
        isThresholdDragInProgress
    };
})();
