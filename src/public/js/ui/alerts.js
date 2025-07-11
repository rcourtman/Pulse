// Per-Guest Alert System UI Module
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alerts = (() => {
    let alertsToggle = null;
    let globalAlertThresholds = null;
    let mainTable = null;
    let isAlertsMode = false;
    let guestAlertThresholds = {}; // Store per-guest thresholds
    let globalThresholds = {}; // Store global default thresholds
    let alertLogic = 'or'; // Fixed to OR logic (dropdown removed)
    let alertDuration = 0; // Track alert duration in milliseconds (default instant for testing)
    let autoResolve = true; // Track whether alerts should auto-resolve
    let isSliderDragging = false; // Track if any slider is being dragged

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
        
        // Load notification status
        updateNotificationStatus();
        
        // Set default auto-resolve state
        const autoResolveToggle = document.getElementById('alert-auto-resolve-toggle');
        if (autoResolveToggle && autoResolve !== undefined) {
            autoResolveToggle.checked = autoResolve;
        }
        
        // Add global mouse/touch up handlers to clear dragging state
        document.addEventListener('mouseup', () => {
            isSliderDragging = false;
        });
        document.addEventListener('touchend', () => {
            isSliderDragging = false;
        });
    }

    function setupEventListeners() {
        // Alerts toggle
        if (alertsToggle) {
            alertsToggle.addEventListener('change', handleAlertsToggle);
        }
        
        
        // Alert duration selector
        const alertDurationSelect = document.getElementById('alert-duration-select');
        if (alertDurationSelect) {
            alertDurationSelect.addEventListener('change', handleAlertDurationChange);
        }

        // Email toggle
        const emailToggle = document.getElementById('alert-email-toggle');
        if (emailToggle) {
            emailToggle.addEventListener('change', handleEmailToggle);
        }

        // Webhook toggle
        const webhookToggle = document.getElementById('alert-webhook-toggle');
        if (webhookToggle) {
            webhookToggle.addEventListener('change', handleWebhookToggle);
        }
        
        // Save button
        const saveButton = document.getElementById('save-alert-config');
        if (saveButton) {
            saveButton.addEventListener('click', saveAlertConfig);
        }
        
        // Test email button
        const testEmailButton = document.getElementById('test-email-alert');
        if (testEmailButton) {
            testEmailButton.addEventListener('click', sendTestEmail);
        }
        
        // Test webhook button
        const testWebhookButton = document.getElementById('test-webhook-alert');
        if (testWebhookButton) {
            testWebhookButton.addEventListener('click', sendTestWebhook);
        }
        
        // Auto-resolve toggle
        const autoResolveToggle = document.getElementById('alert-auto-resolve-toggle');
        if (autoResolveToggle) {
            autoResolveToggle.addEventListener('change', handleAutoResolveToggle);
        }

        // Make sure charts toggle is mutually exclusive with alerts
        const chartsToggle = document.getElementById('toggle-charts-checkbox');
        
        if (chartsToggle) {
            chartsToggle.addEventListener('change', () => {
                if (chartsToggle.checked && isAlertsMode) {
                    alertsToggle.checked = false;
                    handleAlertsToggle();
                }
            });
        }
    }

    function handleAlertsToggle() {
        isAlertsMode = alertsToggle.checked;
        
        // Add class to disable transitions during switch
        document.body.classList.add('switching-modes');
        
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
        
        // Clear threshold styling only when EXITING alerts mode
        if (!isAlertsMode && PulseApp.ui.thresholds) {
            if (PulseApp.ui.thresholds.clearAllStyling) {
                PulseApp.ui.thresholds.clearAllStyling();
            } else if (PulseApp.ui.thresholds.clearAllRowDimming) {
                PulseApp.ui.thresholds.clearAllRowDimming();
            }
        }
        
        // Hide any lingering tooltips when entering alerts mode
        if (isAlertsMode && PulseApp.tooltips) {
            if (PulseApp.tooltips.hideTooltip) {
                PulseApp.tooltips.hideTooltip();
            }
            if (PulseApp.tooltips.hideSliderTooltipImmediately) {
                PulseApp.tooltips.hideSliderTooltipImmediately();
            }
        }
        
        
        // Remove class after mode switch is complete
        requestAnimationFrame(() => {
            requestAnimationFrame(() => {
                document.body.classList.remove('switching-modes');
            });
        });
    }
    
    
    function handleAlertDurationChange(event) {
        alertDuration = parseInt(event.target.value);
        
        // Changes will be saved when user clicks save button
    }

    async function handleNotificationToggle(event, type) {
        const enabled = event.target.checked;
        const isEmail = type === 'email';
        
        // Show/hide cooldown settings
        const cooldownSettingsId = isEmail ? 'email-cooldown-settings' : 'webhook-cooldown-settings';
        const cooldownSettings = document.getElementById(cooldownSettingsId);
        if (cooldownSettings) {
            cooldownSettings.style.display = enabled ? 'block' : 'none';
        }
        
        try {
            // Get current config
            const configResponse = await fetch('/api/config');
            if (!configResponse.ok) {
                throw new Error('Failed to get current config');
            }
            const configData = await configResponse.json();
            
            // Update the appropriate config field
            const configField = isEmail ? 'ALERT_EMAIL_ENABLED' : 'ALERT_WEBHOOK_ENABLED';
            const updatedConfig = {
                ...configData,
                [configField]: enabled
            };
            
            // Save updated config
            const saveResponse = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updatedConfig)
            });
            
            const saveResult = await saveResponse.json();
            
            if (!saveResult.success) {
                throw new Error('Failed to save config');
            }
            
            const typeLabel = isEmail ? 'emails' : 'webhooks';
            PulseApp.ui.toast?.success(`Alert ${typeLabel} ${enabled ? 'enabled' : 'disabled'}`);
        } catch (error) {
            const typeLabel = isEmail ? 'email' : 'webhook';
            console.error(`Failed to toggle alert ${typeLabel}s:`, error);
            PulseApp.ui.toast?.error(`Failed to update ${typeLabel} setting`);
            // Revert toggle on error
            event.target.checked = !enabled;
        }
    }

    function handleEmailToggle(event) {
        return handleNotificationToggle(event, 'email');
    }

    function handleWebhookToggle(event) {
        return handleNotificationToggle(event, 'webhook');
    }
    
    function handleAutoResolveToggle(event) {
        autoResolve = event.target.checked;
        
        // Changes will be saved when user clicks save button
    }

    function updateAlertsMode() {
        if (!globalAlertThresholds || !mainTable) return;
        
        // Get the alert mode controls container
        const alertModeControls = document.getElementById('alert-mode-controls');
        // Get the main threshold row
        const thresholdRow = document.getElementById('threshold-slider-row');

        if (isAlertsMode) {
            // Hide the main threshold row
            if (thresholdRow) {
                thresholdRow.classList.add('hidden');
            }
            
            // Show global thresholds row
            globalAlertThresholds.classList.remove('hidden');
            
            // Show alert mode controls
            if (alertModeControls) {
                alertModeControls.classList.remove('hidden');
            }
            
            // Apply initial dimming to global row if using defaults
            const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
            updateResetButtonVisibility(hasCustomGuests);
            
            // Update notification status when showing alerts mode
            updateNotificationStatus();
            
            // Update save message
            updateAlertSaveMessage();
            
            // Update cooldown settings visibility based on toggle states
            const emailToggle = document.getElementById('alert-email-toggle');
            const emailCooldownSettings = document.getElementById('email-cooldown-settings');
            if (emailToggle && emailCooldownSettings) {
                emailCooldownSettings.style.display = emailToggle.checked ? 'block' : 'none';
            }
            
            const webhookToggle = document.getElementById('alert-webhook-toggle');
            const webhookCooldownSettings = document.getElementById('webhook-cooldown-settings');
            if (webhookToggle && webhookCooldownSettings) {
                webhookCooldownSettings.style.display = webhookToggle.checked ? 'block' : 'none';
            }
            
            // Add alerts mode class to body for CSS targeting
            document.body.classList.add('alerts-mode');
            
            // Transform table to show threshold inputs
            transformTableToAlertsMode();
            
        } else {
            // Don't show the main threshold row - let the threshold module control its visibility
            
            // Hide global thresholds row
            globalAlertThresholds.classList.add('hidden');
            
            // Hide alert mode controls
            if (alertModeControls) {
                alertModeControls.classList.add('hidden');
            }
            
            // Remove alerts mode class
            document.body.classList.remove('alerts-mode');
            
            // Restore normal table view
            restoreNormalTableMode();
            
            // Clear only alert-specific row styling
            clearAllAlertStyling();
            
            // Update node summary cards
            if (PulseApp.ui.nodes && PulseApp.ui.nodes.updateNodeSummaryCards) {
                const nodesData = PulseApp.state?.get('nodesData') || [];
                PulseApp.ui.nodes.updateNodeSummaryCards(nodesData);
            }
            
            // Hide alerts count badge
            updateAlertsCountBadge();
            
            // Don't re-apply threshold filtering when exiting alerts mode
            // Thresholds should only be applied when the threshold toggle is explicitly checked
        }
    }

    function initializeGlobalThresholds() {
        if (!globalAlertThresholds) return;
        
        // Only set defaults if not already initialized
        if (!globalThresholds || Object.keys(globalThresholds).length === 0) {
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
        }
        
        
        // Populate the table cells - exactly like threshold row
        const cpuCell = document.getElementById('global-cpu-cell');
        const memoryCell = document.getElementById('global-memory-cell');
        const diskCell = document.getElementById('global-disk-cell');
        
        if (cpuCell) {
            cpuCell.innerHTML = `<div class="alert-threshold-input">${
                PulseApp.ui.thresholds.createThresholdSliderHtml(
                    'global-alert-cpu', 0, 100, 5, globalThresholds.cpu
                )
            }</div>`;
        }
        
        if (memoryCell) {
            memoryCell.innerHTML = `<div class="alert-threshold-input">${
                PulseApp.ui.thresholds.createThresholdSliderHtml(
                    'global-alert-memory', 0, 100, 5, globalThresholds.memory
                )
            }</div>`;
        }
        
        if (diskCell) {
            diskCell.innerHTML = `<div class="alert-threshold-input">${
                PulseApp.ui.thresholds.createThresholdSliderHtml(
                    'global-alert-disk', 0, 100, 5, globalThresholds.disk
                )
            }</div>`;
        }
        
        // Create dropdowns for I/O metrics
        const ioOptions = PulseApp.utils.IO_ALERT_OPTIONS;
        
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

    // Optimized event setup: only update tooltip during drag, defer all DOM updates until release
    function setupAlertSliderEvents(sliderElement, metricType) {
        if (!sliderElement) return;
        
        // Only update tooltip during drag, don't update any DOM
        sliderElement.addEventListener('input', (event) => {
            PulseApp.tooltips.updateSliderTooltip(event.target);
        });
        
        // Update everything on release
        sliderElement.addEventListener('change', (event) => {
            isSliderDragging = false; // Clear dragging state
            const value = event.target.value;
            globalThresholds[metricType] = value; // Update state
            
            // Update the slider's own styling based on whether it's default or custom
            const defaultValue = metricType === 'cpu' ? 80 : metricType === 'memory' ? 85 : metricType === 'disk' ? 90 : '';
            if (value != defaultValue && value !== '') {
                sliderElement.classList.add('custom-threshold');
            } else {
                sliderElement.classList.remove('custom-threshold');
            }
            
            // Update guest input values
            updateGuestInputValues(metricType, value, true);
            // Update row styling
            updateRowStylingOnly();
            PulseApp.tooltips.updateSliderTooltip(event.target);
            // Changes will be saved when user clicks save button
        });
        
        sliderElement.addEventListener('mousedown', (event) => {
            isSliderDragging = true; // Set dragging state
            PulseApp.tooltips.updateSliderTooltip(event.target);
            if (PulseApp.ui.dashboard && PulseApp.ui.dashboard.snapshotGuestMetricsForDrag) {
                PulseApp.ui.dashboard.snapshotGuestMetricsForDrag();
            }
        });
        
        sliderElement.addEventListener('touchstart', (event) => {
            isSliderDragging = true; // Set dragging state
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
            PulseApp.ui.thresholds.updateSliderVisual(cpuSlider);
        }
        
        if (memorySlider) {
            setupAlertSliderEvents(memorySlider, 'memory');
            PulseApp.ui.thresholds.updateSliderVisual(memorySlider);
        }
        
        if (diskSlider) {
            setupAlertSliderEvents(diskSlider, 'disk');
            PulseApp.ui.thresholds.updateSliderVisual(diskSlider);
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
            
            resetButton.classList.add('opacity-50', 'cursor-not-allowed');
            resetButton.disabled = true;
        }
        
    }

    // Lightweight update function matching threshold system pattern
    function updateGlobalThreshold(metricType, newValue, shouldSave = true) {
        globalThresholds[metricType] = newValue;
        
        // Update the global slider styling based on whether it's default or custom
        const globalSlider = document.getElementById(`global-alert-${metricType}`);
        if (globalSlider) {
            const defaultValue = metricType === 'cpu' ? 80 : metricType === 'memory' ? 85 : metricType === 'disk' ? 90 : '';
            // Use != for type-coercing comparison since slider values are strings
            if (newValue != defaultValue && newValue !== '') {
                globalSlider.classList.add('custom-threshold');
            } else {
                globalSlider.classList.remove('custom-threshold');
            }
        }
        
        updateRowStylingOnly();
        
        // Update reset button state when global thresholds change
        const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
        updateResetButtonVisibility(hasCustomGuests);
        
        // Update save message
        updateAlertSaveMessage();
        
        // Changes will be saved when user clicks save button
    }

    // This function is no longer needed - redirecting to main styling update
    function updateAlertRowStylingOnly() {
        updateRowStylingOnly();
    }

    // Check if a node would trigger alerts based on current threshold settings

    // Check if a guest would trigger alerts based on current threshold settings
    function checkGuestWouldTriggerAlerts(guestId, guestThresholds) {
        // Get guest data from the dashboard
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const guest = dashboardData.find(g => g.id == guestId || g.vmid == guestId);
        if (!guest) return false;
        
        if (guest.status !== 'running') return false;
        
        const metricsToCheck = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
        let hasAnyThresholds = false;
        let exceededThresholds = 0;
        let totalThresholds = 0;
        
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
            
            if (thresholdValue === undefined || thresholdValue === null || thresholdValue === '') continue;
            
            hasAnyThresholds = true;
            totalThresholds++;
            
            // Get guest's current value for this metric
            let guestValue;
            if (metricType === 'cpu') {
                guestValue = guest.cpu;
            } else if (metricType === 'memory') {
                // Since we don't have raw mem/maxmem values in frontend, use the pre-calculated value
                // but adjust for rounding differences with backend
                guestValue = guest.memory;
                
                // If the value is exactly at the threshold when rounded, check if it would be below when not rounded
                // This accounts for cases like 44.7% that rounds to 45% but is actually below the 45% threshold
                if (guestValue === parseFloat(thresholdValue)) {
                    // Assume worst case: the actual value could be up to 0.5% lower
                    // This ensures frontend matches backend behavior
                    guestValue = guestValue - 0.01;
                }
                
            } else if (metricType === 'disk') {
                // Same issue as memory - use pre-calculated value with rounding adjustment
                guestValue = guest.disk;
                
                // If the value is exactly at the threshold when rounded, adjust for potential rounding
                if (guestValue === parseFloat(thresholdValue)) {
                    guestValue = guestValue - 0.01;
                }
            } else if (metricType === 'diskread') {
                guestValue = guest.diskread;
            } else if (metricType === 'diskwrite') {
                guestValue = guest.diskwrite;
            } else if (metricType === 'netin') {
                guestValue = guest.netin;
            } else if (metricType === 'netout') {
                guestValue = guest.netout;
            }
            
            // Skip if guest value is not available
            if (guestValue === undefined || guestValue === null || guestValue === 'N/A' || isNaN(guestValue)) continue;
            
            // Check if guest value exceeds threshold
            if (guestValue >= parseFloat(thresholdValue)) {
                exceededThresholds++;
                
                // For OR logic, return true immediately if any threshold is exceeded
                if (alertLogic === 'or') {
                    return true;
                }
            } else if (alertLogic === 'and') {
                // For AND logic, return false immediately if any threshold is not exceeded
                return false;
            }
        }
        
        // Return result based on logic mode
        if (alertLogic === 'and') {
            // Return true only if we have thresholds and ALL of them are exceeded
            return hasAnyThresholds && exceededThresholds === totalThresholds;
        } else {
            // OR logic - already returned true above if any threshold was exceeded
            return false;
        }
    }

    // Update slider styling to indicate custom vs global thresholds
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
            
            // Update slider styling for each metric
            const metricTypes = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
            metricTypes.forEach(metricType => {
                const hasCustomThreshold = guestThresholds[metricType] !== undefined;
                
                let globalIsCustom = false;
                if (metricType === 'cpu' || metricType === 'memory' || metricType === 'disk') {
                    const defaultValue = metricType === 'cpu' ? 80 : metricType === 'memory' ? 85 : 90;
                    // Use != for type coercion since values might be strings
                    globalIsCustom = globalThresholds[metricType] != defaultValue;
                } else {
                    // For I/O metrics, any non-empty value is custom
                    globalIsCustom = globalThresholds[metricType] !== '' && globalThresholds[metricType] !== undefined;
                }
                
                const sliderContainer = row.querySelector(`[data-guest-id="${guestId}"][data-metric="${metricType}"]`);
                if (sliderContainer) {
                    const slider = sliderContainer.querySelector('input[type="range"]');
                    if (slider) {
                        // Slider is blue if it has a custom value OR if it's using a custom global value
                        if (hasCustomThreshold || (!hasCustomThreshold && globalIsCustom)) {
                            slider.classList.add('custom-threshold');
                        } else {
                            slider.classList.remove('custom-threshold');
                        }
                    }
                }
            });
        });
        
        // Update reset button visibility based on whether there are any custom values
        updateResetButtonVisibility(hasAnyCustomValues);
        
        // Update alerts count badge
        updateAlertsCountBadge();
        
        // Update save message
        updateAlertSaveMessage();
    }
    
    function updateAlertsCountBadge() {
        const alertsBadge = document.getElementById('alerts-count-badge');
        if (!alertsBadge || !isAlertsMode) {
            if (alertsBadge) {
                alertsBadge.classList.add('hidden');
            }
            return;
        }
        
        // Count guests with custom alert settings
        const customCount = Object.keys(guestAlertThresholds).length;
        
        // Update badge
        alertsBadge.textContent = customCount;
        if (customCount > 0) {
            alertsBadge.classList.remove('hidden');
        } else {
            alertsBadge.classList.add('hidden');
        }
    }
    
    function updateAlertSaveMessage() {
        const saveMessage = document.getElementById('alert-save-message');
        if (!saveMessage || !isAlertsMode) return;
        
        // Count how many guests would trigger alerts
        let guestTriggerCount = 0;
        const dashboardData = PulseApp.state?.get('dashboardData') || [];
        
        dashboardData.forEach(guest => {
            const guestThresholds = guestAlertThresholds[guest.id] || {};
            if (checkGuestWouldTriggerAlerts(guest.id, guestThresholds)) {
                guestTriggerCount++;
            }
        });
        
        
        // Update message
        if (guestTriggerCount > 0) {
            let message = '';
            message = `${guestTriggerCount} guest alert${guestTriggerCount !== 1 ? 's' : ''} will trigger`;
            saveMessage.textContent = message;
            saveMessage.classList.remove('text-green-600', 'dark:text-green-400');
            saveMessage.classList.add('text-amber-600', 'dark:text-amber-400');
        } else {
            saveMessage.textContent = 'No alerts will trigger';
            saveMessage.classList.remove('text-amber-600', 'dark:text-amber-400');
            saveMessage.classList.add('text-green-600', 'dark:text-green-400');
        }
    }
    
    
    function updateResetButtonVisibility(hasCustomValues) {
        const resetButton = document.getElementById('reset-global-thresholds');
        const globalThresholdsRow = document.getElementById('global-alert-thresholds-row');
        
        // Check if global guest thresholds have been changed from defaults
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
        
        // Global row should never be dimmed - it's the master control
        if (globalThresholdsRow) {
            globalThresholdsRow.style.opacity = '';
            globalThresholdsRow.style.transition = '';
        }
        
        if (resetButton) {
            // Enable button if there are custom values OR any global changes
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
        
        // Clear ALL alert-related styling from all rows and cells
        const rows = tableBody.querySelectorAll('tr[data-id]');
        rows.forEach(row => {
            // Clear row-level styling immediately
            row.style.opacity = '';
            row.style.transition = '';
            row.removeAttribute('data-alert-dimmed');
            row.removeAttribute('data-alert-mixed');
            
            // Clear ALL cell-level styling
            const allCells = row.querySelectorAll('td');
            allCells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.style.backgroundColor = '';
                cell.style.borderLeft = '';
                cell.removeAttribute('data-alert-custom');
            });
        });
    }
    
    function applyAlertDimmingFast() {
        // This function is no longer needed as we're using slider styling instead of row dimming
        updateRowStylingOnly();
    }

    function resetGlobalThresholds() {
        // Store old values for smooth updates
        const oldThresholds = { ...globalThresholds };
        
        // Clear all individual thresholds
        guestAlertThresholds = {};
        
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
            PulseApp.ui.thresholds.updateSliderVisual(cpuSlider);
        }
        if (memorySlider) {
            memorySlider.value = globalThresholds.memory;
            PulseApp.ui.thresholds.updateSliderVisual(memorySlider);
        }
        if (diskSlider) {
            diskSlider.value = globalThresholds.disk;
            PulseApp.ui.thresholds.updateSliderVisual(diskSlider);
            // Hide any lingering tooltip that might have been triggered
            PulseApp.tooltips.hideSliderTooltipImmediately();
        }
        if (diskreadSelect) diskreadSelect.value = globalThresholds.diskread;
        if (diskwriteSelect) diskwriteSelect.value = globalThresholds.diskwrite;
        if (netinSelect) netinSelect.value = globalThresholds.netin;
        if (netoutSelect) netoutSelect.value = globalThresholds.netout;
        
        // Update all guest rows smoothly
        if (isAlertsMode) {
            for (const metricType in globalThresholds) {
                updateGuestInputValues(metricType, globalThresholds[metricType], true);
            }
            
            updateRowStylingOnly();
        }
        
        // Clear any tooltip positioning issues that might have occurred during reset
        PulseApp.tooltips.hideSliderTooltipImmediately();
        
        // Update reset button - should be disabled after reset since everything is cleared
        updateResetButtonVisibility(false);
        
        // Update save message
        updateAlertSaveMessage();
        
        PulseApp.ui.toast?.success('Global thresholds reset to defaults - click Save to apply');
    }

    function transformTableToAlertsMode() {
        // Get all guest rows
        const guestRows = mainTable.querySelectorAll('tbody tr[data-id]');
        
        // First pass: Clean up any previous styling
        guestRows.forEach(row => {
            // Remove any threshold-specific attributes
            row.removeAttribute('data-dimmed');
            row.removeAttribute('data-alert-dimmed');
            row.style.opacity = '';
            row.style.transition = '';
        });
        
        // Second pass: Transform cells
        guestRows.forEach(row => {
            const guestId = row.getAttribute('data-id'); // Use composite unique ID, not just vmid
            
            // Transform each metric cell using threshold system pattern
            transformMetricCell(row, 'cpu', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            transformMetricCell(row, 'memory', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            transformMetricCell(row, 'disk', guestId, { type: 'slider', min: 0, max: 100, step: 5 });
            
            transformMetricCell(row, 'netin', guestId, { type: 'select', options: PulseApp.utils.IO_ALERT_OPTIONS });
            transformMetricCell(row, 'netout', guestId, { type: 'select', options: PulseApp.utils.IO_ALERT_OPTIONS });
            transformMetricCell(row, 'diskread', guestId, { type: 'select', options: PulseApp.utils.IO_ALERT_OPTIONS });
            transformMetricCell(row, 'diskwrite', guestId, { type: 'select', options: PulseApp.utils.IO_ALERT_OPTIONS });
        });
        
        // Apply final row styling after transformation
        updateRowStylingOnly();
        
        // No need for special fixes - the table structure remains intact
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
                <div class="alert-threshold-input" data-guest-id="${guestId}" data-metric="${metricType}">
                    ${sliderHtml}
                </div>
            `;
            
            // Setup custom alert events using threshold system pattern
            const slider = cell.querySelector('input[type="range"]');
            if (slider) {
                // Apply custom-threshold class if this guest has a custom value for this metric
                const hasCustomValue = guestAlertThresholds[guestId] && guestAlertThresholds[guestId][metricType] !== undefined;
                if (hasCustomValue) {
                    slider.classList.add('custom-threshold');
                }
                
                // Only update tooltip during drag, defer all updates until release
                slider.addEventListener('input', (event) => {
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
                
                // Update everything on release
                slider.addEventListener('change', (event) => {
                    const value = event.target.value;
                    updateGuestThreshold(guestId, metricType, value, true);
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
            
            // For dropdowns, we need to preserve the data attributes for event handling
            cell.innerHTML = selectHtml;
            // Add data attributes to the select element itself
            const select = cell.querySelector('select');
            if (select) {
                select.setAttribute('data-guest-id', guestId);
                select.setAttribute('data-metric', metricType);
                select.addEventListener('change', (e) => {
                    updateGuestThreshold(guestId, metricType, e.target.value);
                });
            }
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
        
        // Force update charts if charts mode is active
        const chartsToggle = document.getElementById('toggle-charts-checkbox');
        if (chartsToggle && chartsToggle.checked && PulseApp.charts) {
            // Give DOM time to settle then update charts
            requestAnimationFrame(() => {
                PulseApp.charts.updateAllCharts();
            });
        }
    }

    function updateGuestThreshold(guestId, metricType, value, shouldSave = true) {
        
        // Initialize guest object if it doesn't exist
        if (!guestAlertThresholds[guestId]) {
            guestAlertThresholds[guestId] = {};
        }
        
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
        const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
        updateResetButtonVisibility(hasCustomGuests);
        
        // Update save message
        updateAlertSaveMessage();
        
        // Changes will be saved when user clicks save button
    }

    async function saveAlertConfig() {
        // Get current toggle states
        const emailToggle = document.getElementById('alert-email-toggle');
        const webhookToggle = document.getElementById('alert-webhook-toggle');
        
        // Get email cooldown settings
        const cooldownMinutes = document.getElementById('alert-cooldown-minutes');
        const debounceMinutes = document.getElementById('alert-debounce-minutes');
        const maxEmailsPerHour = document.getElementById('alert-max-emails-hour');
        
        // Get webhook cooldown settings
        const webhookCooldownMinutes = document.getElementById('webhook-cooldown-minutes');
        const webhookDebounceMinutes = document.getElementById('webhook-debounce-minutes');
        const webhookMaxCallsPerHour = document.getElementById('webhook-max-calls-hour');
        
        const alertConfig = {
            type: 'per_guest_thresholds',
            globalThresholds: globalThresholds,
            guestThresholds: guestAlertThresholds,
            alertLogic: alertLogic,
            duration: alertDuration,
            autoResolve: autoResolve,
            notifications: {
                dashboard: true,
                email: emailToggle ? emailToggle.checked : false,
                webhook: webhookToggle ? webhookToggle.checked : false
            },
            emailCooldowns: {
                cooldownMinutes: cooldownMinutes ? parseInt(cooldownMinutes.value) : 15,
                debounceMinutes: debounceMinutes ? parseInt(debounceMinutes.value) : 2,
                maxEmailsPerHour: maxEmailsPerHour ? parseInt(maxEmailsPerHour.value) : 4
            },
            webhookCooldowns: {
                cooldownMinutes: webhookCooldownMinutes ? parseInt(webhookCooldownMinutes.value) : 5,
                debounceMinutes: webhookDebounceMinutes ? parseInt(webhookDebounceMinutes.value) : 1,
                maxCallsPerHour: webhookMaxCallsPerHour ? parseInt(webhookMaxCallsPerHour.value) : 10
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
                // Show success message
                PulseApp.ui.toast?.success('Alert configuration saved');
                
                // Clear any unsaved changes indicator
                const saveButton = document.getElementById('save-alert-config');
                if (saveButton) {
                    saveButton.textContent = 'Save Changes';
                    saveButton.classList.remove('animate-pulse');
                }
                
                // Update save message to indicate saved state
                updateAlertSaveMessage();
            } else {
                PulseApp.ui.toast?.error('Failed to save alert configuration');
                console.warn('Failed to save alert configuration:', result.error);
            }
            
        } catch (error) {
            PulseApp.ui.toast?.error('Error saving alert configuration');
            console.error('Error saving alert configuration:', error);
        }
    }

    function clearAllAlerts() {
        guestAlertThresholds = {};
        
        if (isAlertsMode) {
            transformTableToAlertsMode();
            updateAlertSaveMessage();
        }
        
        PulseApp.ui.toast?.success('All alert thresholds cleared - click Save to apply');
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
                        if (globalThresholds.cpu != 80) {
                            cpuSlider.classList.add('custom-threshold');
                        }
                    }
                    if (memorySlider && globalThresholds.memory) {
                        memorySlider.value = globalThresholds.memory;
                        if (globalThresholds.memory != 85) {
                            memorySlider.classList.add('custom-threshold');
                        }
                    }
                    if (diskSlider && globalThresholds.disk) {
                        diskSlider.value = globalThresholds.disk;
                        if (globalThresholds.disk != 90) {
                            diskSlider.classList.add('custom-threshold');
                        }
                    }
                    
                    // Hide any tooltips that might have been triggered during value setting
                    PulseApp.tooltips.hideSliderTooltipImmediately();
                    if (diskreadSelect && globalThresholds.diskread) diskreadSelect.value = globalThresholds.diskread;
                    if (diskwriteSelect && globalThresholds.diskwrite) diskwriteSelect.value = globalThresholds.diskwrite;
                    if (netinSelect && globalThresholds.netin) netinSelect.value = globalThresholds.netin;
                    if (netoutSelect && globalThresholds.netout) netoutSelect.value = globalThresholds.netout;
                }
                
                // Load guest thresholds
                if (config.guestThresholds) {
                    guestAlertThresholds = config.guestThresholds;
                }
                
                
                // Load alert duration
                if (config.duration !== undefined) {
                    alertDuration = config.duration;
                    const alertDurationSelect = document.getElementById('alert-duration-select');
                    if (alertDurationSelect) {
                        alertDurationSelect.value = alertDuration.toString();
                    }
                }
                
                // Load auto-resolve setting
                if (config.autoResolve !== undefined) {
                    autoResolve = config.autoResolve;
                    const autoResolveToggle = document.getElementById('alert-auto-resolve-toggle');
                    if (autoResolveToggle) {
                        autoResolveToggle.checked = autoResolve;
                    }
                }
                
                // Load email cooldown settings
                if (config.emailCooldowns) {
                    const cooldownMinutes = document.getElementById('alert-cooldown-minutes');
                    const debounceMinutes = document.getElementById('alert-debounce-minutes');
                    const maxEmailsPerHour = document.getElementById('alert-max-emails-hour');
                    
                    if (cooldownMinutes && config.emailCooldowns.cooldownMinutes !== undefined) {
                        cooldownMinutes.value = config.emailCooldowns.cooldownMinutes;
                    }
                    if (debounceMinutes && config.emailCooldowns.debounceMinutes !== undefined) {
                        debounceMinutes.value = config.emailCooldowns.debounceMinutes;
                    }
                    if (maxEmailsPerHour && config.emailCooldowns.maxEmailsPerHour !== undefined) {
                        maxEmailsPerHour.value = config.emailCooldowns.maxEmailsPerHour;
                    }
                }
                
                // Load webhook cooldown settings
                if (config.webhookCooldowns) {
                    const webhookCooldownMinutes = document.getElementById('webhook-cooldown-minutes');
                    const webhookDebounceMinutes = document.getElementById('webhook-debounce-minutes');
                    const webhookMaxCallsPerHour = document.getElementById('webhook-max-calls-hour');
                    
                    if (webhookCooldownMinutes && config.webhookCooldowns.cooldownMinutes !== undefined) {
                        webhookCooldownMinutes.value = config.webhookCooldowns.cooldownMinutes;
                    }
                    if (webhookDebounceMinutes && config.webhookCooldowns.debounceMinutes !== undefined) {
                        webhookDebounceMinutes.value = config.webhookCooldowns.debounceMinutes;
                    }
                    if (webhookMaxCallsPerHour && config.webhookCooldowns.maxCallsPerHour !== undefined) {
                        webhookMaxCallsPerHour.value = config.webhookCooldowns.maxCallsPerHour;
                    }
                }
                
                // Show/hide email cooldown settings based on email toggle state
                const emailToggle = document.getElementById('alert-email-toggle');
                const emailCooldownSettings = document.getElementById('email-cooldown-settings');
                if (emailToggle && emailCooldownSettings) {
                    emailCooldownSettings.style.display = emailToggle.checked ? 'block' : 'none';
                }
                
                // Show/hide webhook cooldown settings based on webhook toggle state
                const webhookToggle = document.getElementById('alert-webhook-toggle');
                const webhookCooldownSettings = document.getElementById('webhook-cooldown-settings');
                if (webhookToggle && webhookCooldownSettings) {
                    webhookCooldownSettings.style.display = webhookToggle.checked ? 'block' : 'none';
                }
                
                
                // Update reset button state and global row dimming after loading configuration
                const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
                updateResetButtonVisibility(hasCustomGuests);
                
                // Update styling if in alerts mode
                if (isAlertsMode) {
                    updateRowStylingOnly();
                    updateAlertSaveMessage();
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

    async function updateNotificationStatus() {
        try {
            // Fetch current configuration to get notification status
            const response = await fetch('/api/config');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const data = await response.json();
            
            // Extract notification status from config
            const emailEnabled = data.ALERT_EMAIL_ENABLED !== false && (data.SMTP_HOST || data.ALERT_FROM_EMAIL);
            const webhookEnabled = data.ALERT_WEBHOOK_ENABLED !== false && !!data.WEBHOOK_URL;
            
            // Update toggle switches
            const emailToggle = document.getElementById('alert-email-toggle');
            const webhookToggle = document.getElementById('alert-webhook-toggle');
            
            if (emailToggle) {
                emailToggle.checked = emailEnabled;
            }
            
            if (webhookToggle) {
                webhookToggle.checked = webhookEnabled;
            }
        } catch (error) {
            console.error('Failed to update notification status:', error);
        }
    }

    async function sendTestEmail() {
        const testButton = document.getElementById('test-email-alert');
        const originalText = testButton?.textContent || 'Test Email';
        
        if (testButton) {
            testButton.disabled = true;
            testButton.textContent = 'Sending...';
        }
        
        try {
            const response = await fetch('/api/alerts/test-email', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast?.success('Test email sent successfully');
            } else {
                PulseApp.ui.toast?.error(result.error || 'Failed to send test email');
            }
        } catch (error) {
            console.error('Error sending test email:', error);
            PulseApp.ui.toast?.error('Error sending test email');
        } finally {
            if (testButton) {
                testButton.disabled = false;
                testButton.textContent = originalText;
            }
        }
    }
    
    async function sendTestWebhook() {
        const testButton = document.getElementById('test-webhook-alert');
        const originalText = testButton?.textContent || 'Test Webhook';
        
        if (testButton) {
            testButton.disabled = true;
            testButton.textContent = 'Sending...';
        }
        
        try {
            const response = await fetch('/api/alerts/test-webhook', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast?.success('Test webhook sent successfully');
            } else {
                PulseApp.ui.toast?.error(result.error || 'Failed to send test webhook');
            }
        } catch (error) {
            console.error('Error sending test webhook:', error);
            PulseApp.ui.toast?.error('Error sending test webhook');
        } finally {
            if (testButton) {
                testButton.disabled = false;
                testButton.textContent = originalText;
            }
        }
    }




    // Public API
    return {
        init,
        isAlertsMode: () => isAlertsMode,
        isSliderDragging: () => isSliderDragging,
        setSliderDragging: (dragging) => { isSliderDragging = dragging; },
        getGuestThresholds: () => guestAlertThresholds,
        getGlobalThresholds: () => globalThresholds,
        clearAllAlerts: clearAllAlerts,
        updateGuestThreshold: updateGuestThreshold,
        updateRowStylingOnly: updateRowStylingOnly,
        getActiveAlertsForGuest: getActiveAlertsForGuest,
        loadSavedConfiguration: loadSavedConfiguration,
        updateNotificationStatus: updateNotificationStatus,
        checkGuestWouldTriggerAlerts: checkGuestWouldTriggerAlerts
    };
})();