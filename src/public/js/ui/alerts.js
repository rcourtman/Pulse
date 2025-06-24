// Per-Guest Alert System UI Module
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alerts = (() => {
    let alertsToggle = null;
    let globalAlertThresholds = null;
    let mainTable = null;
    let isAlertsMode = false;
    let guestAlertThresholds = {}; // Store per-guest thresholds
    let globalThresholds = {}; // Store global default thresholds
    let nodeAlertThresholds = {}; // Store per-node thresholds
    let globalNodeThresholds = {}; // Store global default node thresholds
    let alertLogic = 'or'; // Fixed to OR logic (dropdown removed)
    let alertDuration = 0; // Track alert duration in milliseconds (default instant for testing)
    let autoResolve = true; // Track whether alerts should auto-resolve

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
        console.log(`Alert duration changed to: ${alertDuration}ms`);
        
        // Changes will be saved when user clicks save button
    }

    async function handleEmailToggle(event) {
        const enabled = event.target.checked;
        
        // Show/hide email cooldown settings
        const cooldownSettings = document.getElementById('email-cooldown-settings');
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
            
            // Update only ALERT_EMAIL_ENABLED
            const updatedConfig = {
                ...configData,
                ALERT_EMAIL_ENABLED: enabled
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
            
            PulseApp.ui.toast?.success(`Alert emails ${enabled ? 'enabled' : 'disabled'}`);
        } catch (error) {
            console.error('Failed to toggle alert emails:', error);
            PulseApp.ui.toast?.error('Failed to update email setting');
            // Revert toggle on error
            event.target.checked = !enabled;
        }
    }

    async function handleWebhookToggle(event) {
        const enabled = event.target.checked;
        
        // Show/hide webhook cooldown settings
        const cooldownSettings = document.getElementById('webhook-cooldown-settings');
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
            
            // Update only ALERT_WEBHOOK_ENABLED
            const updatedConfig = {
                ...configData,
                ALERT_WEBHOOK_ENABLED: enabled
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
            
            PulseApp.ui.toast?.success(`Alert webhooks ${enabled ? 'enabled' : 'disabled'}`);
        } catch (error) {
            console.error('Failed to toggle alert webhooks:', error);
            PulseApp.ui.toast?.error('Failed to update webhook setting');
            // Revert toggle on error
            event.target.checked = !enabled;
        }
    }
    
    function handleAutoResolveToggle(event) {
        autoResolve = event.target.checked;
        console.log(`Alert auto-resolve ${autoResolve ? 'enabled' : 'disabled'}`);
        
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
            const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
            updateResetButtonVisibility(hasCustomGuests || hasCustomNodes);
            
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
            // This will also apply the correct row styling via updateRowStylingOnly()
            transformTableToAlertsMode();
            
            // Update node summary cards to show alert sliders
            if (PulseApp.ui.nodes && PulseApp.ui.nodes.updateNodeSummaryCards) {
                const nodesData = PulseApp.state?.get('nodesData') || [];
                PulseApp.ui.nodes.updateNodeSummaryCards(nodesData);
            }
            
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
            
            // Update node summary cards to remove alert sliders
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
        
        // Set default global node thresholds (higher than guest defaults)
        globalNodeThresholds = {
            cpu: 90,
            memory: 95,
            disk: 95
        };
        
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

    // Optimized event setup: update everything during drag for consistency
    function setupAlertSliderEvents(sliderElement, metricType) {
        if (!sliderElement) return;
        
        // Full updates during drag (like node sliders)
        sliderElement.addEventListener('input', (event) => {
            const value = event.target.value;
            globalThresholds[metricType] = value; // Update state immediately
            globalNodeThresholds[metricType] = value; // Also update node thresholds
            updateRowStylingOnly(); // Fast styling-only update
            updateAllNodeCardsStyling(); // Update node styling too
            // Don't update guest or node input values during drag - only on release
            PulseApp.tooltips.updateSliderTooltip(event.target);
        });
        
        // Also update on release for consistency
        sliderElement.addEventListener('change', (event) => {
            const value = event.target.value;
            updateGuestInputValues(metricType, value, true); // Update guest input values
            updateNodeInputValues(metricType, value, true); // Update node input values on release only
            // Changes will be saved when user clicks save button
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
        const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
        const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
        updateResetButtonVisibility(hasCustomGuests || hasCustomNodes);
        
        // Update save message
        updateAlertSaveMessage();
        
        // Changes will be saved when user clicks save button
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

    // Check if a node would trigger alerts based on current threshold settings
    function checkNodeWouldTriggerAlerts(nodeId, nodeThresholds) {
        // Get node data from the state
        const nodesData = PulseApp.state.get('nodesData') || [];
        const node = nodesData.find(n => n.node === nodeId);
        if (!node) return false;
        
        // Only check online nodes for alerts
        if (!node.uptime || node.uptime <= 0) return false;
        
        // Check each metric type
        const metricsToCheck = ['cpu', 'memory', 'disk'];
        
        for (const metricType of metricsToCheck) {
            // Get the effective threshold for this node/metric
            let thresholdValue;
            if (nodeThresholds[metricType] !== undefined) {
                // Node has individual threshold
                thresholdValue = nodeThresholds[metricType];
            } else {
                // Use global node threshold
                thresholdValue = globalNodeThresholds[metricType];
            }
            
            // Skip if no threshold is set for this metric
            if (thresholdValue === undefined || thresholdValue === null || thresholdValue === '') continue;
            
            // Get current value and check against threshold
            let currentValue = 0;
            let hasValue = false;
            
            if (metricType === 'cpu' && node.cpu !== undefined) {
                currentValue = node.cpu * 100; // Convert to percentage
                hasValue = true;
            } else if (metricType === 'memory' && node.mem !== undefined && node.maxmem) {
                currentValue = (node.mem / node.maxmem) * 100;
                hasValue = true;
            } else if (metricType === 'disk' && node.disk !== undefined && node.maxdisk) {
                currentValue = (node.disk / node.maxdisk) * 100;
                hasValue = true;
            }
            
            if (hasValue && currentValue > thresholdValue) {
                return true; // Would trigger alert
            }
        }
        
        return false;
    }

    // Check if a guest would trigger alerts based on current threshold settings
    function checkGuestWouldTriggerAlerts(guestId, guestThresholds) {
        // Get guest data from the dashboard
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const guest = dashboardData.find(g => g.id == guestId || g.vmid == guestId);
        if (!guest) return false;
        
        // Only check running guests for alerts (stopped guests can't trigger metric alerts)
        if (guest.status !== 'running') return false;
        
        // Check each metric type using selected logic (AND or OR)
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
            
            // Skip if no threshold is set for this metric (but 0 is a valid threshold)
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
            
            // New behavior: Light up rows that have custom alert settings
            if (hasIndividualSettings) {
                // Light up rows with custom settings
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
            } else {
                // Dim rows using only default settings
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
            }
            
            // Clear any cell-level styling
            const cells = row.querySelectorAll('td');
            cells.forEach(cell => {
                cell.style.opacity = '';
                cell.style.transition = '';
                cell.removeAttribute('data-alert-custom');
            });
            
            // Remove mixed row attributes (no longer used)
            row.removeAttribute('data-alert-mixed');
        });
        
        // Update reset button visibility based on whether there are any custom values (guest or node)
        const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
        updateResetButtonVisibility(hasAnyCustomValues || hasCustomNodes);
        
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
        const guestCustomCount = Object.keys(guestAlertThresholds).length;
        
        // Count nodes with custom alert settings
        const nodeCustomCount = Object.keys(nodeAlertThresholds).length;
        
        // Total custom count
        const totalCustomCount = guestCustomCount + nodeCustomCount;
        
        // Update badge
        alertsBadge.textContent = totalCustomCount;
        if (totalCustomCount > 0) {
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
        
        // Count how many nodes would trigger alerts
        let nodeTriggerCount = 0;
        const nodesData = PulseApp.state?.get('nodesData') || [];
        
        nodesData.forEach(node => {
            const nodeThresholds = nodeAlertThresholds[node.node] || {};
            if (checkNodeWouldTriggerAlerts(node.node, nodeThresholds)) {
                nodeTriggerCount++;
            }
        });
        
        // Update message
        const totalTriggerCount = guestTriggerCount + nodeTriggerCount;
        if (totalTriggerCount > 0) {
            let message = '';
            if (guestTriggerCount > 0 && nodeTriggerCount > 0) {
                message = `${guestTriggerCount} guest & ${nodeTriggerCount} node alert${totalTriggerCount !== 1 ? 's' : ''} will trigger`;
            } else if (guestTriggerCount > 0) {
                message = `${guestTriggerCount} guest alert${guestTriggerCount !== 1 ? 's' : ''} will trigger`;
            } else {
                message = `${nodeTriggerCount} node alert${nodeTriggerCount !== 1 ? 's' : ''} will trigger`;
            }
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
        
        // Check if global node thresholds have been changed from defaults
        const defaultNodeThresholds = {
            cpu: 90,
            memory: 95,
            disk: 95
        };
        
        const hasNodeGlobalChanges = Object.keys(defaultNodeThresholds).some(key => 
            globalNodeThresholds[key] != defaultNodeThresholds[key]
        );
        
        // Global row should never be dimmed - it's the master control
        if (globalThresholdsRow) {
            globalThresholdsRow.style.opacity = '';
            globalThresholdsRow.style.transition = '';
        }
        
        if (resetButton) {
            // Enable button if there are custom values OR any global changes
            if (hasCustomValues || hasGlobalChanges || hasNodeGlobalChanges) {
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
        const tableBody = mainTable.querySelector('tbody');
        if (!tableBody) return;
        
        const rows = tableBody.querySelectorAll('tr[data-id]');
        
        // Single pass: apply dimming based on custom settings
        rows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            const guestThresholds = guestAlertThresholds[guestId] || {};
            const hasIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            // Remove threshold attributes
            row.removeAttribute('data-dimmed');
            row.style.transition = '';
            
            // New behavior: Light up rows that have custom alert settings
            if (hasIndividualSettings) {
                // Light up rows with custom settings
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
            } else {
                // Dim rows using only default settings
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
            }
        });
    }

    function resetGlobalThresholds() {
        // Store old values for smooth updates
        const oldThresholds = { ...globalThresholds };
        
        // Clear all individual thresholds
        guestAlertThresholds = {};
        nodeAlertThresholds = {};
        
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
        
        // Reset node thresholds to match
        globalNodeThresholds = {
            cpu: 80,
            memory: 85,
            disk: 90
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
            // Hide any lingering tooltip that might have been triggered
            PulseApp.tooltips.hideSliderTooltipImmediately();
        }
        if (diskreadSelect) diskreadSelect.value = globalThresholds.diskread;
        if (diskwriteSelect) diskwriteSelect.value = globalThresholds.diskwrite;
        if (netinSelect) netinSelect.value = globalThresholds.netin;
        if (netoutSelect) netoutSelect.value = globalThresholds.netout;
        
        // Clear all individual guest thresholds
        guestAlertThresholds = {};
        
        // Clear all individual node thresholds
        nodeAlertThresholds = {};
        
        // Reset global node thresholds to defaults
        globalNodeThresholds = {
            cpu: 90,
            memory: 95,
            disk: 95
        };
        
        
        // Update all guest rows smoothly
        if (isAlertsMode) {
            // First update input values to match global thresholds (skip tooltips during reset)
            for (const metricType in globalThresholds) {
                updateGuestInputValues(metricType, globalThresholds[metricType], true);
            }
            
            // Then update row styling with new thresholds (no flash since inputs are already updated)
            updateRowStylingOnly();
            
            // Update all node cards to reflect reset
            updateAllNodeCardsStyling();
            
            // Update node summary cards to refresh sliders
            if (PulseApp.ui.nodes && PulseApp.ui.nodes.updateNodeSummaryCards) {
                const nodesData = PulseApp.state?.get('nodesData') || [];
                PulseApp.ui.nodes.updateNodeSummaryCards(nodesData);
            }
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
        
        // First pass: Apply immediate dimming to prevent flash
        guestRows.forEach(row => {
            const guestId = row.getAttribute('data-id');
            const guestThresholds = guestAlertThresholds[guestId] || {};
            const hasIndividualSettings = Object.keys(guestThresholds).length > 0;
            
            // Remove any threshold-specific attributes
            row.removeAttribute('data-dimmed');
            
            // New behavior: Light up rows that have custom alert settings
            if (hasIndividualSettings) {
                // Light up rows with custom settings
                row.style.opacity = '';
                row.style.transition = '';
                row.removeAttribute('data-alert-dimmed');
            } else {
                // Dim rows using only default settings
                row.style.opacity = '0.4';
                row.style.transition = 'opacity 0.1s ease-out';
                row.setAttribute('data-alert-dimmed', 'true');
            }
        });
        
        // Second pass: Transform cells
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
                // Update everything immediately during drag (like threshold system)
                slider.addEventListener('input', (event) => {
                    const value = event.target.value;
                    updateGuestThreshold(guestId, metricType, value, true); // Save immediately
                    PulseApp.tooltips.updateSliderTooltip(event.target);
                });
                
                // Also add change event for consistency with node sliders
                slider.addEventListener('change', (event) => {
                    const value = event.target.value;
                    updateGuestThreshold(guestId, metricType, value, true); // Save on release too
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
        const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
        const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
        updateResetButtonVisibility(hasCustomGuests || hasCustomNodes);
        
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
            globalNodeThresholds: globalNodeThresholds,
            nodeThresholds: nodeAlertThresholds,
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
        
        console.log('[saveAlertConfig] Saving alert config:', alertConfig);
        console.log('[saveAlertConfig] Node thresholds:', {
            globalNodeThresholds: alertConfig.globalNodeThresholds,
            nodeThresholds: alertConfig.nodeThresholds
        });
        
        try {
            const response = await fetch('/api/alerts/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(alertConfig)
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast?.success('Alert configuration saved');
                
                // Return to main dashboard view
                if (alertsToggle && alertsToggle.checked) {
                    alertsToggle.checked = false;
                    handleAlertsToggle();
                }
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
                    }
                    if (memorySlider && globalThresholds.memory) {
                        memorySlider.value = globalThresholds.memory;
                    }
                    if (diskSlider && globalThresholds.disk) {
                        diskSlider.value = globalThresholds.disk;
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
                
                // Load global node thresholds
                if (config.globalNodeThresholds) {
                    globalNodeThresholds = { ...globalNodeThresholds, ...config.globalNodeThresholds };
                }
                
                // Load node thresholds
                if (config.nodeThresholds) {
                    nodeAlertThresholds = config.nodeThresholds;
                }
                
                // Alert logic is now fixed to OR (dropdown removed)
                
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
                
                console.log('Alert configuration loaded successfully');
                
                // Update reset button state and global row dimming after loading configuration
                const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
            const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
            updateResetButtonVisibility(hasCustomGuests || hasCustomNodes);
                
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
            const emailEnabled = data.ALERT_EMAIL_ENABLED !== false && data.GLOBAL_EMAIL_ENABLED !== false;
            const webhookEnabled = data.ALERT_WEBHOOK_ENABLED !== false && data.GLOBAL_WEBHOOK_ENABLED !== false;
            
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

    function updateGlobalNodeThresholdUI() {
        // This function is no longer needed since we're using the main global sliders
        return;
        
        // Populate the node threshold cells
        const cpuCell = document.getElementById('global-node-cpu-cell');
        const memoryCell = document.getElementById('global-node-memory-cell');
        const diskCell = document.getElementById('global-node-disk-cell');
        
        console.log('[DEBUG] Found cells:', { cpuCell: !!cpuCell, memoryCell: !!memoryCell, diskCell: !!diskCell });
        
        // Set up event delegation on parent container first
        const parentContainer = cpuCell?.parentElement?.parentElement;
        if (parentContainer && !parentContainer._nodeEventsDelegated) {
            console.log('[DEBUG] Setting up event delegation on parent container:', parentContainer);
            
            // Test with capturing phase to catch events before they might be stopped
            parentContainer.addEventListener('input', (e) => {
                console.log('[DEBUG] ANY input event captured:', e.target.tagName, e.target.id);
                if (e.target.id === 'global-node-alert-cpu' || 
                    e.target.id === 'global-node-alert-memory' || 
                    e.target.id === 'global-node-alert-disk') {
                    console.log('[DEBUG] Delegated input event:', e.target.id, 'value:', e.target.value);
                    const metricType = e.target.id.replace('global-node-alert-', '');
                    globalNodeThresholds[metricType] = e.target.value;
                    updateAllNodeCardsStyling();
                    PulseApp.tooltips.updateSliderTooltip(e.target);
                }
            }, true); // Use capturing phase
            
            parentContainer.addEventListener('change', (e) => {
                console.log('[DEBUG] ANY change event captured:', e.target.tagName, e.target.id);
                if (e.target.id === 'global-node-alert-cpu' || 
                    e.target.id === 'global-node-alert-memory' || 
                    e.target.id === 'global-node-alert-disk') {
                    console.log('[DEBUG] Delegated change event:', e.target.id, 'value:', e.target.value);
                    const metricType = e.target.id.replace('global-node-alert-', '');
                    updateNodeInputValues(metricType, e.target.value, true);
                }
            }, true); // Use capturing phase
            
            // Also add click handler to test if ANY events work
            parentContainer.addEventListener('click', (e) => {
                console.log('[DEBUG] Click event on:', e.target.tagName, e.target.id);
            }, true);
            
            parentContainer._nodeEventsDelegated = true;
        }
        
        if (cpuCell) {
            const sliderId = 'global-node-alert-cpu';
            const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(sliderId, 0, 100, 5, globalNodeThresholds.cpu || 90);
            cpuCell.innerHTML = sliderHtml;
            
            // Add event listeners directly after creating the element
            setTimeout(() => {
                const slider = document.getElementById(sliderId);
                if (slider) {
                    slider.oninput = function(e) {
                        console.log('[DEBUG] CPU oninput:', e.target.value);
                        globalNodeThresholds.cpu = e.target.value;
                        updateAllNodeCardsStyling();
                        PulseApp.tooltips.updateSliderTooltip(e.target);
                    };
                    slider.onchange = function(e) {
                        console.log('[DEBUG] CPU onchange:', e.target.value);
                        updateNodeInputValues('cpu', e.target.value, true);
                    };
                }
            }, 0);
        }
        
        if (memoryCell) {
            const sliderId = 'global-node-alert-memory';
            const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(sliderId, 0, 100, 5, globalNodeThresholds.memory || 95);
            memoryCell.innerHTML = sliderHtml;
            
            // Add event listeners directly after creating the element
            setTimeout(() => {
                const slider = document.getElementById(sliderId);
                if (slider) {
                    slider.oninput = function(e) {
                        console.log('[DEBUG] Memory oninput:', e.target.value);
                        globalNodeThresholds.memory = e.target.value;
                        updateAllNodeCardsStyling();
                        PulseApp.tooltips.updateSliderTooltip(e.target);
                    };
                    slider.onchange = function(e) {
                        console.log('[DEBUG] Memory onchange:', e.target.value);
                        updateNodeInputValues('memory', e.target.value, true);
                    };
                }
            }, 0);
        }
        
        if (diskCell) {
            const sliderId = 'global-node-alert-disk';
            const sliderHtml = PulseApp.ui.thresholds.createThresholdSliderHtml(sliderId, 0, 100, 5, globalNodeThresholds.disk || 95);
            diskCell.innerHTML = sliderHtml;
            
            // Add event listeners directly after creating the element
            setTimeout(() => {
                const slider = document.getElementById(sliderId);
                if (slider) {
                    slider.oninput = function(e) {
                        console.log('[DEBUG] Disk oninput:', e.target.value);
                        globalNodeThresholds.disk = e.target.value;
                        updateAllNodeCardsStyling();
                        PulseApp.tooltips.updateSliderTooltip(e.target);
                    };
                    slider.onchange = function(e) {
                        console.log('[DEBUG] Disk onchange:', e.target.value);
                        updateNodeInputValues('disk', e.target.value, true);
                    };
                }
            }, 0);
        }
        
        globalNodeUIInitialized = true;
        
        // TEST: Add a button to manually test if we can update values
        if (!document.getElementById('test-node-sliders-btn')) {
            const testBtn = document.createElement('button');
            testBtn.id = 'test-node-sliders-btn';
            testBtn.textContent = 'TEST NODE SLIDERS';
            testBtn.className = 'ml-2 text-xs bg-red-500 text-white px-2 py-1 rounded';
            testBtn.onclick = () => {
                console.log('[DEBUG] Test button clicked');
                const cpu = document.getElementById('global-node-alert-cpu');
                const mem = document.getElementById('global-node-alert-memory');
                const disk = document.getElementById('global-node-alert-disk');
                console.log('[DEBUG] Found sliders:', { cpu: !!cpu, mem: !!mem, disk: !!disk });
                if (cpu) {
                    console.log('[DEBUG] CPU slider value before:', cpu.value);
                    cpu.value = 50;
                    console.log('[DEBUG] CPU slider value after:', cpu.value);
                    updateNodeInputValues('cpu', 50, true);
                }
            };
            const container = document.querySelector('h4.text-sm.font-medium.text-gray-700.dark\\:text-gray-300');
            if (container && container.textContent.includes('Default Node Alert Thresholds')) {
                container.appendChild(testBtn);
            }
        }
        
        // Setup reset button
        const resetButton = document.getElementById('reset-global-node-thresholds');
        if (resetButton && !resetButton._hasClickHandler) {
            resetButton.addEventListener('click', resetGlobalNodeThresholds);
            resetButton._hasClickHandler = true;
        }
    }
    
    // Setup event listeners for global node threshold sliders (using same pattern as guest sliders)
    function setupGlobalNodeThresholdEventListeners() {
        console.log('[DEBUG] setupGlobalNodeThresholdEventListeners called');
        
        // Setup sliders using custom alert event handling (same as guest sliders)
        const cpuSlider = document.getElementById('global-node-alert-cpu');
        const memorySlider = document.getElementById('global-node-alert-memory');
        const diskSlider = document.getElementById('global-node-alert-disk');
        
        if (cpuSlider) {
            console.log('[DEBUG] Setting up node CPU slider');
            setupNodeAlertSliderEvents(cpuSlider, 'cpu');
        }
        
        if (memorySlider) {
            console.log('[DEBUG] Setting up node Memory slider');
            setupNodeAlertSliderEvents(memorySlider, 'memory');
        }
        
        if (diskSlider) {
            console.log('[DEBUG] Setting up node Disk slider');
            setupNodeAlertSliderEvents(diskSlider, 'disk');
        }
    }
    
    // Node-specific alert slider event setup (similar to setupAlertSliderEvents but for nodes)
    function setupNodeAlertSliderEvents(sliderElement, metricType) {
        if (!sliderElement) return;
        
        console.log('[DEBUG] setupNodeAlertSliderEvents for', metricType);
        console.log('[DEBUG] Slider element:', sliderElement);
        console.log('[DEBUG] Slider ID:', sliderElement.id);
        
        // Test if we can add ANY event listener
        sliderElement.addEventListener('click', () => {
            console.log('[DEBUG] CLICK event on', metricType, 'slider!');
        });
        
        // Lightweight updates during drag (styling only, like guest sliders)
        const inputHandler = (event) => {
            console.log('[DEBUG] Node', metricType, 'input event, value:', event.target.value);
            const value = event.target.value;
            globalNodeThresholds[metricType] = value; // Update state immediately
            updateAllNodeCardsStyling(); // Fast styling-only update
            PulseApp.tooltips.updateSliderTooltip(event.target);
        };
        
        // Full update on release (input values)
        const changeHandler = (event) => {
            console.log('[DEBUG] Node', metricType, 'change event, value:', event.target.value);
            const value = event.target.value;
            updateNodeInputValues(metricType, value, true); // Update input values on release, skip tooltips
        };
        
        sliderElement.addEventListener('input', inputHandler);
        sliderElement.addEventListener('change', changeHandler);
        
        // Store handlers for debugging
        sliderElement._inputHandler = inputHandler;
        sliderElement._changeHandler = changeHandler;
        
        // Log all event listeners on this element
        console.log('[DEBUG] Event listeners attached. Testing with getEventListeners in console...');
        
        sliderElement.addEventListener('mousedown', (event) => {
            console.log('[DEBUG] Mousedown on', metricType);
            PulseApp.tooltips.updateSliderTooltip(event.target);
        });
        
        sliderElement.addEventListener('touchstart', (event) => {
            PulseApp.tooltips.updateSliderTooltip(event.target);
        }, { passive: true });
    }
    
    function resetGlobalNodeThresholds() {
        // Reset to default values
        globalNodeThresholds = {
            cpu: 90,
            memory: 95,
            disk: 95
        };
        
        // Reset the initialization flag to force recreation
        globalNodeUIInitialized = false;
        
        // Update UI
        updateGlobalNodeThresholdUI();
        
        // Update save message
        updateAlertSaveMessage();
    }

    // Node threshold management functions
    function updateNodeThreshold(nodeId, metricType, value, shouldUpdate = true) {
        if (!nodeAlertThresholds[nodeId]) {
            nodeAlertThresholds[nodeId] = {};
        }
        
        // Check if value matches global value (handle both string and numeric comparisons)
        const globalValue = globalNodeThresholds[metricType] || '';
        const isMatchingGlobal = value === globalValue || 
                                 value == globalValue || 
                                 (value === '' && globalValue === '');
        
        if (isMatchingGlobal || value === '') {
            // Remove threshold if it matches global or is explicitly empty
            delete nodeAlertThresholds[nodeId][metricType];
            
            // Remove node object if no thresholds left
            if (Object.keys(nodeAlertThresholds[nodeId]).length === 0) {
                delete nodeAlertThresholds[nodeId];
            }
        } else {
            // Store individual threshold
            nodeAlertThresholds[nodeId][metricType] = value;
        }
        
        // Update node card dimming
        updateNodeCardStyling(nodeId);
        
        if (shouldUpdate) {
            // Use a slight delay to ensure this happens after any other updates
            setTimeout(() => {
                updateAlertSaveMessage();
                updateAlertsCountBadge();
                // Check if we have any custom thresholds (guest or node) to update reset button
                const hasCustomGuests = Object.keys(guestAlertThresholds).length > 0;
                const hasCustomNodes = Object.keys(nodeAlertThresholds).length > 0;
                console.log('[updateNodeThreshold] Updating reset button visibility:', { hasCustomGuests, hasCustomNodes, nodeAlertThresholds });
                updateResetButtonVisibility(hasCustomGuests || hasCustomNodes);
            }, 50);
        }
    }
    
    function updateNodeCardStyling(nodeId) {
        // Find the node card element
        const nodeCard = document.querySelector(`[data-node-id="${nodeId}"]`);
        if (!nodeCard) return;
        
        // Check if node has any custom settings
        const hasCustomSettings = nodeAlertThresholds[nodeId] && Object.keys(nodeAlertThresholds[nodeId]).length > 0;
        
        // Update opacity using inline styles
        if (hasCustomSettings) {
            nodeCard.style.opacity = '1';
            nodeCard.style.transition = 'opacity 0.1s ease-out';
            nodeCard.setAttribute('data-alert-dimmed', 'false');
        } else {
            nodeCard.style.opacity = '0.4';
            nodeCard.style.transition = 'opacity 0.1s ease-out';
            nodeCard.setAttribute('data-alert-dimmed', 'true');
        }
    }
    
    function updateGlobalNodeThreshold(metricType, newValue, shouldUpdateInputs = false) {
        globalNodeThresholds[metricType] = newValue;
        
        // Only update input values when specifically requested (on change event)
        if (shouldUpdateInputs) {
            updateNodeInputValues(metricType, newValue);
        }
        
        // Always update node card styling since global defaults changed
        updateAllNodeCardsStyling();
        
        // Update save message
        updateAlertSaveMessage();
    }
    
    function updateNodeInputValues(metricType, newValue, skipTooltips = false) {
        if (!isAlertsMode) return;
        
        
        // Debug: Let's see what selectors might work
        // Find all node sliders for this metric
        const nodeSliders = document.querySelectorAll(`.node-alert-slider[data-metric="${metricType}"] input[type="range"]`);
        
        nodeSliders.forEach(slider => {
            const sliderContainer = slider.closest('.node-alert-slider');
            const nodeId = sliderContainer?.getAttribute('data-node-id');
            
            if (nodeId) {
                // Only update sliders for nodes that don't have individual thresholds for this metric
                const nodeThresholds = nodeAlertThresholds[nodeId] || {};
                const hasIndividualForThisMetric = nodeThresholds[metricType] !== undefined;
                
                if (!hasIndividualForThisMetric) {
                    slider.value = newValue;
                    
                    // Update tooltip for sliders (skip during reset operations)
                    if (!skipTooltips) {
                        PulseApp.tooltips.updateSliderTooltip(slider);
                    }
                }
            }
        });
    }
    
    function updateAllNodeCardsStyling() {
        // Update styling for all node cards
        const nodeCards = document.querySelectorAll('[data-node-id]');
        nodeCards.forEach(card => {
            const nodeId = card.getAttribute('data-node-id');
            if (nodeId) {
                updateNodeCardStyling(nodeId);
            }
        });
    }

    // Public API
    return {
        init,
        isAlertsMode: () => isAlertsMode,
        getGuestThresholds: () => guestAlertThresholds,
        getGlobalThresholds: () => globalThresholds,
        getNodeThresholds: () => nodeAlertThresholds,
        getGlobalNodeThresholds: () => globalNodeThresholds,
        clearAllAlerts: clearAllAlerts,
        updateGuestThreshold: updateGuestThreshold,
        updateNodeThreshold: updateNodeThreshold,
        updateGlobalNodeThreshold: updateGlobalNodeThreshold,
        updateNodeInputValues: updateNodeInputValues,
        updateRowStylingOnly: updateRowStylingOnly,
        getActiveAlertsForGuest: getActiveAlertsForGuest,
        loadSavedConfiguration: loadSavedConfiguration,
        updateNotificationStatus: updateNotificationStatus,
        checkGuestWouldTriggerAlerts: checkGuestWouldTriggerAlerts
    };
})();