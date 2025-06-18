// Per-Guest Alert System UI Module
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alerts = (() => {
    let alertsToggle = null;
    let globalAlertThresholds = null;
    let mainTable = null;
    let isAlertsMode = false;
    let guestAlertThresholds = {}; // Store per-guest thresholds
    let globalThresholds = {}; // Store global default thresholds

    function init() {
        // Get DOM elements
        alertsToggle = document.getElementById('toggle-alerts-checkbox');
        globalAlertThresholds = document.getElementById('global-alert-thresholds-row');
        mainTable = document.getElementById('main-table');

        if (!alertsToggle) {
            console.warn('Alerts toggle not found');
            return;
        }

        // Setup event listeners
        setupEventListeners();
        
        // Initialize global thresholds
        initializeGlobalThresholds();
        
        // Initialize state
        isAlertsMode = alertsToggle.checked;
        updateAlertsMode();
        
        // Load saved configuration
        loadSavedConfiguration();
    }

    function setupEventListeners() {
        // Alerts toggle
        if (alertsToggle) {
            alertsToggle.addEventListener('change', handleAlertsToggle);
        }


        // Make sure charts and thresholds toggles are mutually exclusive with alerts
        const chartsToggle = document.getElementById('toggle-charts-checkbox');
        const thresholdsToggle = document.getElementById('toggle-thresholds-checkbox');
        
        if (chartsToggle) {
            chartsToggle.addEventListener('change', () => {
                if (chartsToggle.checked && isAlertsMode) {
                    alertsToggle.checked = false;
                    handleAlertsToggle();
                }
            });
        }
        
        if (thresholdsToggle) {
            thresholdsToggle.addEventListener('change', () => {
                if (thresholdsToggle.checked && isAlertsMode) {
                    alertsToggle.checked = false;
                    handleAlertsToggle();
                }
            });
        }
    }

    function handleAlertsToggle() {
        isAlertsMode = alertsToggle.checked;
        
        // Disable other modes when alerts is enabled
        if (isAlertsMode) {
            const chartsToggle = document.getElementById('toggle-charts-checkbox');
            const thresholdsToggle = document.getElementById('toggle-thresholds-checkbox');
            
            if (chartsToggle && chartsToggle.checked) {
                chartsToggle.checked = false;
                chartsToggle.dispatchEvent(new Event('change'));
            }
            
            if (thresholdsToggle && thresholdsToggle.checked) {
                thresholdsToggle.checked = false;
                thresholdsToggle.dispatchEvent(new Event('change'));
            }
        }
        
        updateAlertsMode();
    }

    function updateAlertsMode() {
        if (!globalAlertThresholds || !mainTable) return;

        if (isAlertsMode) {
            // Show global thresholds row
            globalAlertThresholds.classList.remove('hidden');
            
            // Add alerts mode class to body for CSS targeting
            document.body.classList.add('alerts-mode');
            
            // Transform table to show threshold inputs instead of values
            transformTableToAlertsMode();
            
            // Apply initial row styling like threshold system
            updateRowStylingOnly();
            
        } else {
            // Hide global thresholds row
            globalAlertThresholds.classList.add('hidden');
            
            // Remove alerts mode class
            document.body.classList.remove('alerts-mode');
            
            // Restore normal table view
            restoreNormalTableMode();
            
            // Clear only alert-specific row styling
            clearAllAlertStyling();
        }
    }

    function initializeGlobalThresholds() {
        if (!globalAlertThresholds) return;
        
        // Set default global threshold values
        globalThresholds = {
            cpu: 80,
            memory: 85,
            disk: 90,
            diskread: '',
            diskwrite: '',
            netin: '',
            netout: ''
        };
        
        // Populate the table cells - exactly like threshold row
        const cpuCell = document.getElementById('global-cpu-cell');
        const memoryCell = document.getElementById('global-memory-cell');
        const diskCell = document.getElementById('global-disk-cell');
        
        if (cpuCell) {
            cpuCell.innerHTML = PulseApp.ui.thresholds.createThresholdSliderHtml(
                'global-alert-cpu', 0, 100, 5, globalThresholds.cpu
            );
        }
        
        if (memoryCell) {
            memoryCell.innerHTML = PulseApp.ui.thresholds.createThresholdSliderHtml(
                'global-alert-memory', 0, 100, 5, globalThresholds.memory
            );
        }
        
        if (diskCell) {
            diskCell.innerHTML = PulseApp.ui.thresholds.createThresholdSliderHtml(
                'global-alert-disk', 0, 100, 5, globalThresholds.disk
            );
        }
        
        // Create dropdowns for I/O metrics
        const ioOptions = [
            { value: '', label: 'No alert' },
            { value: '1048576', label: '> 1 MB/s' },
            { value: '10485760', label: '> 10 MB/s' },
            { value: '52428800', label: '> 50 MB/s' },
            { value: '104857600', label: '> 100 MB/s' }
        ];
        
        const diskreadCell = document.getElementById('global-diskread-cell');
        const diskwriteCell = document.getElementById('global-diskwrite-cell');
        const netinCell = document.getElementById('global-netin-cell');
        const netoutCell = document.getElementById('global-netout-cell');
        
        if (diskreadCell) {
            diskreadCell.innerHTML = PulseApp.ui.thresholds.createThresholdSelectHtml(
                'global-alert-diskread', ioOptions, globalThresholds.diskread
            );
        }
        
        if (diskwriteCell) {
            diskwriteCell.innerHTML = PulseApp.ui.thresholds.createThresholdSelectHtml(
                'global-alert-diskwrite', ioOptions, globalThresholds.diskwrite
            );
        }
        
        if (netinCell) {
            netinCell.innerHTML = PulseApp.ui.thresholds.createThresholdSelectHtml(
                'global-alert-netin', ioOptions, globalThresholds.netin
            );
        }
        
        if (netoutCell) {
            netoutCell.innerHTML = PulseApp.ui.thresholds.createThresholdSelectHtml(
                'global-alert-netout', ioOptions, globalThresholds.netout
            );
        }
        
        // Setup event listeners for global controls using threshold system pattern
        setupGlobalThresholdEventListeners();
    }

    // Optimized event setup: lightweight during drag, full update on release
    function setupAlertSliderEvents(sliderElement, metricType) {
        if (!sliderElement) return;
        
        // Lightweight updates during drag (styling only, like threshold system)
        sliderElement.addEventListener('input', (event) => {
            const value = event.target.value;
            globalThresholds[metricType] = value; // Update state immediately
            updateRowStylingOnly(); // Fast styling-only update
            PulseApp.tooltips.updateSliderTooltip(event.target);
        });
        
        // Full update on release (input values + save)
        sliderElement.addEventListener('change', (event) => {
            const value = event.target.value;
            updateGuestInputValues(metricType, value, true); // Update input values on release, skip tooltips
            autoSaveAlertConfig();
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

    function setupGlobalThresholdEventListeners() {
        // Setup sliders using custom alert event handling
        const cpuSlider = document.getElementById('global-alert-cpu');
        const memorySlider = document.getElementById('global-alert-memory');
        const diskSlider = document.getElementById('global-alert-disk');
        
        if (cpuSlider) {
            setupAlertSliderEvents(cpuSlider, 'cpu');
        }
        
        if (memorySlider) {
            setupAlertSliderEvents(memorySlider, 'memory');
        }
        
        if (diskSlider) {
            setupAlertSliderEvents(diskSlider, 'disk');
        }
        
        // Setup dropdowns
        const diskreadSelect = document.getElementById('global-alert-diskread');
        const diskwriteSelect = document.getElementById('global-alert-diskwrite');
        const netinSelect = document.getElementById('global-alert-netin');
        const netoutSelect = document.getElementById('global-alert-netout');
        
        if (diskreadSelect) {
            diskreadSelect.addEventListener('change', (e) => {
                updateGlobalThreshold('diskread', e.target.value);
            });
        }
        
        if (diskwriteSelect) {
            diskwriteSelect.addEventListener('change', (e) => {
                updateGlobalThreshold('diskwrite', e.target.value);
            });
        }
        
        if (netinSelect) {
            netinSelect.addEventListener('change', (e) => {
                updateGlobalThreshold('netin', e.target.value);
            });
        }
        
        if (netoutSelect) {
            netoutSelect.addEventListener('change', (e) => {
                updateGlobalThreshold('netout', e.target.value);
            });
        }
        
        // Setup reset button
        const resetButton = document.getElementById('reset-global-thresholds');
        if (resetButton) {
            resetButton.addEventListener('click', () => {
                resetGlobalThresholds();
            });
            
            // Set initial state (disabled since no custom values initially)
            resetButton.classList.add('opacity-50', 'cursor-not-allowed');
            resetButton.disabled = true;
        }
    }

    // Lightweight update function matching threshold system pattern
    function updateGlobalThreshold(metricType, newValue, shouldSave = true) {
        globalThresholds[metricType] = newValue;
        
        // Fast path: only update styling, not input values (like threshold system)
        updateRowStylingOnly();
        
        // Update reset button state when global thresholds change
        updateResetButtonVisibility(Object.keys(guestAlertThresholds).length > 0);
        
        if (shouldSave) {
            autoSaveAlertConfig();
        }
    }

    // Lightweight styling-only update (matches threshold system approach)
    function updateAlertRowStylingOnly() {
        if (!isAlertsMode) return;
        
        const tableBody = mainTable.querySelector('tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            const guestThresholds = guestAlertThresholds[guestId] || {};
            const hasAnyIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            // Simple opacity styling only (no input value updates during drag)
            if (!hasAnyIndividualSettings) {
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
            } else {
                row.style.opacity = '1';
                row.style.transition = 'opacity 0.1s ease-out';
                row.removeAttribute('data-alert-dimmed');
            }
        });
    }

    // Check if a guest would trigger alerts based on current threshold settings
    function checkGuestWouldTriggerAlerts(guestId, guestThresholds) {
        // Get guest data from the dashboard
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const guest = dashboardData.find(g => g.id === guestId);
        if (!guest) return false;
        
        // Only check running guests for alerts (stopped guests can't trigger metric alerts)
        if (guest.status !== 'running') return false;
        
        // Check each metric type
        const metricsToCheck = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
        
        for (const metricType of metricsToCheck) {
            // Get the effective threshold for this guest/metric
            let thresholdValue;
            if (guestThresholds[metricType] !== undefined) {
                // Guest has individual threshold
                thresholdValue = guestThresholds[metricType];
            } else {
                // Use global threshold
                thresholdValue = globalThresholds[metricType];
            }
            
            // Skip if no threshold is set for this metric
            if (!thresholdValue || thresholdValue === '' || thresholdValue === '0') continue;
            
            // Get guest's current value for this metric
            let guestValue;
            if (metricType === 'cpu') guestValue = guest.cpu;
            else if (metricType === 'memory') guestValue = guest.memory;
            else if (metricType === 'disk') guestValue = guest.disk;
            else if (metricType === 'diskread') guestValue = guest.diskread;
            else if (metricType === 'diskwrite') guestValue = guest.diskwrite;
            else if (metricType === 'netin') guestValue = guest.netin;
            else if (metricType === 'netout') guestValue = guest.netout;
            
            // Skip if guest value is not available
            if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) continue;
            
            // Check if guest value exceeds threshold (would trigger alert)
            if (guestValue >= parseFloat(thresholdValue)) {
                return true; // This guest would trigger an alert
            }
        }
        
        return false; // No alerts would be triggered for this guest
    }

    // Standalone row styling update with granular cell dimming
    function updateRowStylingOnly() {
        if (!isAlertsMode) return;
        
        const tableBody = mainTable.querySelector('tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        let hasAnyCustomValues = false;
        
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            const guestThresholds = guestAlertThresholds[guestId] || {};
            const hasIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            if (hasIndividualSettings) {
                hasAnyCustomValues = true;
            }
            
            // Check if this guest would trigger alerts based on current thresholds
            const wouldTriggerAlerts = checkGuestWouldTriggerAlerts(guestId, guestThresholds);
            
            // Apply row-level dimming: dim if guest WOULDN'T trigger alerts (like threshold system)
            if (!wouldTriggerAlerts) {
                if (!hasIndividualSettings) {
                    // Pure global settings - full row dimming
                    row.style.opacity = '0.4';
                    row.style.transition = 'opacity 0.1s ease-out';
                    row.setAttribute('data-alert-dimmed', 'true');
                    row.removeAttribute('data-alert-mixed');
                } else {
                    // Mixed settings - let cell styling handle granular dimming
                    row.style.opacity = '';
                    row.style.transition = '';
                    row.removeAttribute('data-alert-dimmed');
                    row.setAttribute('data-alert-mixed', 'true');
                }
            } else {
                // Would trigger alerts - keep visible
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
                if (hasIndividualSettings) {
                    row.setAttribute('data-alert-mixed', 'true');
                } else {
                    row.removeAttribute('data-alert-mixed');
                }
            }
            
            // Apply granular cell-level dimming/undimming for mixed rows
            if (hasIndividualSettings) {
                updateCellStyling(row, guestId, guestThresholds);
            }
        });
        
        // Update reset button visibility based on whether there are any custom values
        updateResetButtonVisibility(hasAnyCustomValues);
    }
    
    // Apply dimming to individual cells based on which metrics have custom values
    function updateCellStyling(row, guestId, guestThresholds) {
        const cells = row.querySelectorAll('td');
        if (cells.length < 11) return;
        
        
        // Map metric types to cell indices: name(0), type(1), id(2), uptime(3), cpu(4), memory(5), disk(6), diskread(7), diskwrite(8), netin(9), netout(10)
        const metricCells = {
            'cpu': cells[4],
            'memory': cells[5], 
            'disk': cells[6],
            'diskread': cells[7],
            'diskwrite': cells[8],
            'netin': cells[9],
            'netout': cells[10]
        };
        
        // Check if this row has mixed values
        const isMixedRow = row.hasAttribute('data-alert-mixed');
        
        if (isMixedRow) {
            // For mixed rows, apply cell-specific opacity
            Object.entries(metricCells).forEach(([metricType, cell]) => {
                if (!cell) return;
                
                const hasCustomValue = guestThresholds[metricType] !== undefined;
                
                if (hasCustomValue) {
                    // This specific metric has a custom value - full opacity
                    cell.style.opacity = '1';
                    cell.style.transition = 'opacity 0.1s ease-out';
                    cell.setAttribute('data-alert-custom', 'true');
                } else {
                    // This metric uses global value - dim it
                    cell.style.opacity = '0.4';
                    cell.style.transition = 'opacity 0.1s ease-out';
                    cell.removeAttribute('data-alert-custom');
                }
            });
            
            // Also dim non-metric cells in mixed rows
            for (let i = 0; i < 4; i++) { // name, type, id, uptime
                if (cells[i]) {
                    cells[i].style.opacity = '0.4';
                    cells[i].style.transition = 'opacity 0.1s ease-out';
                }
            }
        } else {
            // For non-mixed rows, clear all cell-specific styling
            cells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.removeAttribute('data-alert-custom');
            });
        }
    }
    
    function updateResetButtonVisibility(hasCustomValues) {
        const resetButton = document.getElementById('reset-global-thresholds');
        if (resetButton) {
            // Check if global thresholds have been changed from defaults
            const defaultThresholds = {
                cpu: 80,
                memory: 85,
                disk: 90,
                diskread: '',
                diskwrite: '',
                netin: '',
                netout: ''
            };
            
            const hasGlobalChanges = Object.keys(defaultThresholds).some(key => 
                globalThresholds[key] != defaultThresholds[key] // Use != for loose comparison
            );
            
            // Enable button if there are either custom guest values OR global changes
            if (hasCustomValues || hasGlobalChanges) {
                resetButton.classList.remove('opacity-50', 'cursor-not-allowed');
                resetButton.disabled = false;
            } else {
                resetButton.classList.add('opacity-50', 'cursor-not-allowed');
                resetButton.disabled = true;
            }
        }
    }

    function updateGuestInputValues(metricType, newValue, skipTooltips = false) {
        if (!isAlertsMode) return;
        
        const tableBody = mainTable.querySelector('tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            
            // Only update inputs for guests that don't have individual thresholds for this metric
            const guestThresholds = guestAlertThresholds[guestId] || {};
            const hasIndividualForThisMetric = guestThresholds[metricType] !== undefined;
            
            if (!hasIndividualForThisMetric) {
                const metricInput = row.querySelector(`[data-guest-id="${guestId}"][data-metric="${metricType}"] input, [data-guest-id="${guestId}"][data-metric="${metricType}"] select`);
                
                if (metricInput) {
                    metricInput.value = newValue;
                    
                    // Update tooltip for sliders (skip during reset operations)
                    if (metricInput.type === 'range' && !skipTooltips) {
                        PulseApp.tooltips.updateSliderTooltip(metricInput);
                    }
                }
            }
        });
    }

    function clearAllAlertStyling() {
        const tableBody = mainTable.querySelector('tbody');
        if (!tableBody) return;
        
        // Aggressively clear ALL alert-related styling from all rows and cells
        const rows = tableBody.querySelectorAll('tr[data-id]');
        rows.forEach(row => {
            // Disable transitions temporarily to prevent flash
            row.style.transition = 'none';
            
            // Clear ALL cell-level styling first
            const allCells = row.querySelectorAll('td');
            allCells.forEach(cell => {
                cell.style.transition = 'none';
                cell.style.opacity = '';
                cell.style.backgroundColor = '';
                cell.style.borderLeft = '';
                cell.removeAttribute('data-alert-custom');
            });
            
            // Then set row-level styling
            row.style.opacity = '0.4'; // Apply dimming immediately
            row.setAttribute('data-alert-dimmed', 'true'); // Mark as properly dimmed
            row.removeAttribute('data-alert-mixed');
            
            // Re-enable transitions after a brief delay
            setTimeout(() => {
                row.style.transition = 'opacity 0.1s ease-out';
                allCells.forEach(cell => {
                    cell.style.transition = '';
                });
            }, 10);
        });
    }

    function resetGlobalThresholds() {
        // Store old values for smooth updates
        const oldThresholds = { ...globalThresholds };
        
        // Reset to defaults
        globalThresholds = {
            cpu: 80,
            memory: 85,
            disk: 90,
            diskread: '',
            diskwrite: '',
            netin: '',
            netout: ''
        };
        
        // Update the UI controls
        const cpuSlider = document.getElementById('global-alert-cpu');
        const memorySlider = document.getElementById('global-alert-memory');
        const diskSlider = document.getElementById('global-alert-disk');
        const diskreadSelect = document.getElementById('global-alert-diskread');
        const diskwriteSelect = document.getElementById('global-alert-diskwrite');
        const netinSelect = document.getElementById('global-alert-netin');
        const netoutSelect = document.getElementById('global-alert-netout');
        
        if (cpuSlider) {
            cpuSlider.value = globalThresholds.cpu;
        }
        if (memorySlider) {
            memorySlider.value = globalThresholds.memory;
        }
        if (diskSlider) {
            diskSlider.value = globalThresholds.disk;
        }
        if (diskreadSelect) diskreadSelect.value = globalThresholds.diskread;
        if (diskwriteSelect) diskwriteSelect.value = globalThresholds.diskwrite;
        if (netinSelect) netinSelect.value = globalThresholds.netin;
        if (netoutSelect) netoutSelect.value = globalThresholds.netout;
        
        // Clear all individual guest thresholds
        guestAlertThresholds = {};
        
        // Update all guest rows smoothly
        if (isAlertsMode) {
            // First, immediately clean up all styling and apply final dimming to prevent flash
            clearAllAlertStyling();
            
            // Then update input values to match global thresholds (skip tooltips during reset)
            for (const metricType in globalThresholds) {
                updateGuestInputValues(metricType, globalThresholds[metricType], true);
            }
        }
        
        // Auto-save
        autoSaveAlertConfig();
        
        // Clear any tooltip positioning issues that might have occurred during reset
        PulseApp.tooltips.hideSliderTooltipImmediately();
        
        PulseApp.ui.toast?.success('Global thresholds reset to defaults');
    }

    function transformTableToAlertsMode() {
        // Get all guest rows
        const guestRows = mainTable.querySelectorAll('tbody tr[data-id]');
        
        guestRows.forEach(row => {
            const guestId = row.getAttribute('data-id'); // Use composite unique ID, not just vmid
            
            // Transform each metric cell using threshold system pattern
            transformMetricCell(row, 'cpu', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            transformMetricCell(row, 'memory', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            transformMetricCell(row, 'disk', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            
            const ioOptions = [
                { value: '', label: 'No alert' },
                { value: '1048576', label: '> 1 MB/s' },
                { value: '10485760', label: '> 10 MB/s' },
                { value: '52428800', label: '> 50 MB/s' },
                { value: '104857600', label: '> 100 MB/s' }
            ];
            
            transformMetricCell(row, 'netin', guestId, { type: 'select', options: ioOptions });
            transformMetricCell(row, 'netout', guestId, { type: 'select', options: ioOptions });
            transformMetricCell(row, 'diskread', guestId, { type: 'select', options: ioOptions });
            transformMetricCell(row, 'diskwrite', guestId, { type: 'select', options: ioOptions });
        });
        
        // Apply row styling after transformation
        updateRowStylingOnly();
    }

    function transformMetricCell(row, metricType, guestId, config) {
        // Map metric types to table column indices
        const columnIndices = {
            'cpu': 4, 'memory': 5, 'disk': 6, 'diskread': 7, 
            'diskwrite': 8, 'netin': 9, 'netout': 10
        };
        
        const columnIndex = columnIndices[metricType];
        if (columnIndex === undefined) return;
        
        const cells = row.querySelectorAll('td');
        const cell = cells[columnIndex];
        if (!cell) return;
        
        // Skip if cell already has alert input
        if (cell.querySelector('.alert-threshold-input')) {
            return;
        }
        
        // Store original content
        if (!cell.dataset.originalContent) {
            cell.dataset.originalContent = cell.innerHTML;
        }
        
        // Get current threshold value for this guest/metric
        const currentValue = (guestAlertThresholds[guestId] && guestAlertThresholds[guestId][metricType]) 
            || globalThresholds[metricType] || '';
        
        if (config.type === 'slider') {
            // Use threshold system's helper function
            const sliderId = `alert-slider-${guestId}-${metricType}`;
            const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(
                sliderId, config.min, config.max, config.step, currentValue || config.min
            );
            
            cell.innerHTML = `
                <div class="alert-threshold-input h-5 leading-5" data-guest-id="${guestId}" data-metric="${metricType}">
                    ${sliderHtml}
                </div>
            `;
            
            // Setup custom alert events using threshold system pattern
            const slider = cell.querySelector('input[type="range"]');
            if (slider) {
                // Update everything immediately during drag (like threshold system)
                slider.addEventListener('input', (event) => {
                    const value = event.target.value;
                    updateGuestThreshold(guestId, metricType, value, true); // Save immediately
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
                
                slider.addEventListener('mousedown', (event) => {
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                });
                
                slider.addEventListener('touchstart', (event) => {
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                    if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                        PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
                    }
                }, { passive: true });
            }
            
        } else if (config.type === 'select') {
            // Use threshold system's helper function
            const selectId = `alert-select-${guestId}-${metricType}`;
            const selectHtml = PulseApp.ui.thresholds.createThresholdSelectHtml(
                selectId, config.options, currentValue
            );
            
            cell.innerHTML = `
                <div class="alert-threshold-input h-5 leading-5" data-guest-id="${guestId}" data-metric="${metricType}">
                    ${selectHtml}
                </div>
            `;
            
            // Add event listener for select
            const select = cell.querySelector('select');
            select.addEventListener('change', (e) => {
                updateGuestThreshold(guestId, metricType, e.target.value);
            });
        }
    }

    function restoreNormalTableMode() {
        if (!mainTable) return;
        
        // Restore original cell content
        const modifiedCells = mainTable.querySelectorAll('[data-original-content]');
        modifiedCells.forEach(cell => {
            cell.innerHTML = cell.dataset.originalContent;
            delete cell.dataset.originalContent;
        });
    }

    function updateGuestThreshold(guestId, metricType, value, shouldSave = true) {
        
        // Initialize guest object if it doesn't exist
        if (!guestAlertThresholds[guestId]) {
            guestAlertThresholds[guestId] = {};
        }
        
        // Check if value matches global value (handle both string and numeric comparisons)
        const globalValue = globalThresholds[metricType] || '';
        const isMatchingGlobal = value === globalValue || 
                                 value == globalValue || 
                                 (value === '' && globalValue === '');
        
        
        if (isMatchingGlobal || value === '') {
            // Remove threshold if it matches global or is explicitly empty
            // Note: 0 is a valid threshold value, don't remove it just because it's 0
            delete guestAlertThresholds[guestId][metricType];
            
            // Remove guest object if no thresholds left
            if (Object.keys(guestAlertThresholds[guestId]).length === 0) {
                delete guestAlertThresholds[guestId];
            }
        } else {
            // Store individual threshold
            guestAlertThresholds[guestId][metricType] = value;
        }
        
        
        // Update row styling immediately using the same pattern as threshold system
        updateRowStylingOnly();
        
        // Update reset button state when guest thresholds change
        updateResetButtonVisibility(Object.keys(guestAlertThresholds).length > 0);
        
        // Only auto-save when explicitly requested (on release, not during drag)
        if (shouldSave) {
            autoSaveAlertConfig();
        }
    }

    async function autoSaveAlertConfig() {
        const alertConfig = {
            type: 'per_guest_thresholds',
            globalThresholds: globalThresholds,
            guestThresholds: guestAlertThresholds,
            notifications: {
                dashboard: true
            },
            enabled: true,
            lastUpdated: new Date().toISOString()
        };
        
        try {
            const response = await fetch('/api/alerts/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(alertConfig)
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                console.log('Alert configuration auto-saved');
            } else {
                console.warn('Failed to auto-save alert configuration:', result.error);
            }
            
        } catch (error) {
            console.warn('Error auto-saving alert configuration:', error);
        }
    }

    function clearAllAlerts() {
        guestAlertThresholds = {};
        
        if (isAlertsMode) {
            transformTableToAlertsMode();
        }
        
        autoSaveAlertConfig();
        PulseApp.ui.toast?.success('All alert thresholds cleared');
    }

    async function loadSavedConfiguration() {
        try {
            const response = await fetch('/api/alerts/config');
            const result = await response.json();
            
            if (response.ok && result.success && result.config) {
                const config = result.config;
                
                // Load global thresholds
                if (config.globalThresholds) {
                    globalThresholds = { ...globalThresholds, ...config.globalThresholds };
                    
                    // Update UI controls with loaded values
                    const cpuSlider = document.getElementById('global-alert-cpu');
                    const memorySlider = document.getElementById('global-alert-memory');
                    const diskSlider = document.getElementById('global-alert-disk');
                    const diskreadSelect = document.getElementById('global-alert-diskread');
                    const diskwriteSelect = document.getElementById('global-alert-diskwrite');
                    const netinSelect = document.getElementById('global-alert-netin');
                    const netoutSelect = document.getElementById('global-alert-netout');
                    
                    if (cpuSlider && globalThresholds.cpu) {
                        cpuSlider.value = globalThresholds.cpu;
                        PulseApp.tooltips.updateSliderTooltip(cpuSlider);
                    }
                    if (memorySlider && globalThresholds.memory) {
                        memorySlider.value = globalThresholds.memory;
                        PulseApp.tooltips.updateSliderTooltip(memorySlider);
                    }
                    if (diskSlider && globalThresholds.disk) {
                        diskSlider.value = globalThresholds.disk;
                        PulseApp.tooltips.updateSliderTooltip(diskSlider);
                    }
                    if (diskreadSelect && globalThresholds.diskread) diskreadSelect.value = globalThresholds.diskread;
                    if (diskwriteSelect && globalThresholds.diskwrite) diskwriteSelect.value = globalThresholds.diskwrite;
                    if (netinSelect && globalThresholds.netin) netinSelect.value = globalThresholds.netin;
                    if (netoutSelect && globalThresholds.netout) netoutSelect.value = globalThresholds.netout;
                }
                
                // Load guest thresholds
                if (config.guestThresholds) {
                    guestAlertThresholds = config.guestThresholds;
                }
                
                
                console.log('Alert configuration loaded successfully');
                
                // Update reset button state after loading configuration
                updateResetButtonVisibility(Object.keys(guestAlertThresholds).length > 0);
                
                // Update styling if in alerts mode
                if (isAlertsMode) {
                    updateRowStylingOnly();
                }
            }
        } catch (error) {
            console.warn('Failed to load alert configuration:', error);
        }
    }

    async function getActiveAlertsForGuest(endpointId, node, vmid) {
        try {
            // Get all active alerts from the server
            const response = await fetch('/api/alerts/active');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const data = await response.json();
            if (!data.success || !Array.isArray(data.alerts)) {
                return [];
            }
            
            // Filter alerts for this specific guest
            const guestAlerts = data.alerts.filter(alert => {
                return alert.guest && 
                       alert.guest.endpointId === endpointId && 
                       alert.guest.node === node && 
                       String(alert.guest.vmid) === String(vmid);
            });
            
            return guestAlerts;
        } catch (error) {
            console.error('Failed to get active alerts for guest:', error);
            return [];
        }
    }

    // Public API
    return {
        init,
        isAlertsMode: () => isAlertsMode,
        getGuestThresholds: () => guestAlertThresholds,
        getGlobalThresholds: () => globalThresholds,
        clearAllAlerts: clearAllAlerts,
        updateGuestThreshold: updateGuestThreshold,
        updateRowStylingOnly: updateRowStylingOnly,
        updateCellStyling: updateCellStyling,
        getActiveAlertsForGuest: getActiveAlertsForGuest
    };
})();