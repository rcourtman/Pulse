PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.thresholds = (() => {
    let thresholdRow = null;
    let toggleThresholdsButton = null;
    let thresholdBadge = null;
    let sliders = {};
    let thresholdSelects = {};
    let startLogButton = null;
    let isDraggingSlider = false;

    function init() {
        thresholdRow = document.getElementById('threshold-slider-row');
        toggleThresholdsButton = document.getElementById('toggle-thresholds-button');
        thresholdBadge = document.getElementById('threshold-count-badge');
        startLogButton = document.getElementById('start-log-button');

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
        updateLogControlsVisibility();

        if (toggleThresholdsButton) {
            toggleThresholdsButton.addEventListener('click', () => {
                PulseApp.state.set('isThresholdRowVisible', !PulseApp.state.get('isThresholdRowVisible'));
                updateThresholdRowVisibility();
            });
        } else {
            console.warn('#toggle-thresholds-button not found.');
        }

        setupThresholdListeners();
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

    function setupThresholdListeners() {
        for (const type in sliders) {
            if (sliders[type]) {
                const sliderElement = sliders[type];
                sliderElement.addEventListener('input', (event) => {
                    const value = event.target.value;
                    updateThreshold(type, value);
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });

                const showTooltip = (event) => PulseApp.tooltips.updateSliderTooltip(event.target);
                sliderElement.addEventListener('mousedown', (event) => {
                    showTooltip(event);
                    isDraggingSlider = true;
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                });
                document.addEventListener('mouseup', () => {
                    if (isDraggingSlider) {
                        isDraggingSlider = false;
                        if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.clearGuestMetricSnapshots) {
                            PulseApp.ui.dashboard.clearGuestMetricSnapshots();
                        }
                    }
                });
                sliderElement.addEventListener('touchstart', (event) => {
                    showTooltip(event);
                    isDraggingSlider = true;
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                }, { passive: true });
                document.addEventListener('touchend', () => {
                    if (isDraggingSlider) {
                        isDraggingSlider = false;
                        if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.clearGuestMetricSnapshots) {
                            PulseApp.ui.dashboard.clearGuestMetricSnapshots();
                        }
                    }
                });
            } else {
                console.warn(`Slider element not found for type: ${type}`);
            }
        }

        for (const type in thresholdSelects) {
            const selectElement = thresholdSelects[type];
            if (selectElement) {
                selectElement.addEventListener('change', (event) => {
                    const value = event.target.value;
                    updateThreshold(type, value);
                });
            }
        }
    }

    function updateThreshold(type, value) {
        PulseApp.state.setThresholdValue(type, value);

        if (PulseApp.ui && PulseApp.ui.dashboard) {
            PulseApp.ui.dashboard.updateDashboardTable();
        } else {
            console.warn('[Thresholds] PulseApp.ui.dashboard not available for updateDashboardTable');
        }
        updateThresholdIndicator();
        updateLogControlsVisibility();
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

    function updateThresholdIndicator() {
        if (!thresholdBadge) return;

        const mainTableHeader = document.querySelector('#main-table thead');
        if (!mainTableHeader) return;

        const thresholdState = PulseApp.state.getThresholdState();
        let activeCount = 0;
        for (const type in thresholdState) {
            const defaultColorClasses = ['text-gray-600', 'dark:text-gray-300'];
            const activeColorClasses = ['text-blue-600', 'dark:text-blue-400'];
            const headerCell = mainTableHeader.querySelector(`th[data-sort="${type}"]`);

            if (thresholdState[type].value > 0) {
                activeCount++;
                if (headerCell) {
                    headerCell.classList.add('threshold-active-header');
                    headerCell.classList.remove(...defaultColorClasses);
                    headerCell.classList.add(...activeColorClasses);
                }
            } else {
                if (headerCell) {
                    headerCell.classList.remove('threshold-active-header');
                    headerCell.classList.remove(...activeColorClasses);
                    headerCell.classList.add(...defaultColorClasses);
                }
            }
        }

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
        updateLogControlsVisibility(); // Ensure log button visibility updates
    }

    function updateLogControlsVisibility() {
        if (!startLogButton) return;

        let isAnyFilterActive = false;
        const thresholdState = PulseApp.state.getThresholdState();
        const searchInput = document.getElementById('dashboard-search');
        const filterGuestType = PulseApp.state.get('filterGuestType');
        const filterStatus = PulseApp.state.get('filterStatus');

        for (const type in thresholdState) {
            if (thresholdState[type].value > 0) {
                isAnyFilterActive = true;
                break;
            }
        }

        if (!isAnyFilterActive && searchInput && searchInput.value.trim() !== '') {
            isAnyFilterActive = true;
        }

        if (!isAnyFilterActive && filterGuestType !== 'all') {
            isAnyFilterActive = true;
        }

        if (!isAnyFilterActive && filterStatus !== 'all') {
            isAnyFilterActive = true;
        }

        startLogButton.classList.toggle('hidden', !isAnyFilterActive);
    }

    // Getter for dashboard.js to check drag state
    function isThresholdDragInProgress() {
        return isDraggingSlider;
    }

    return {
        init,
        resetThresholds,
        updateLogControlsVisibility,
        isThresholdDragInProgress
    };
})(); 