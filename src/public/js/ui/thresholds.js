PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.thresholds = (() => {
    let thresholdRow = null;
    let alertRuleRow = null;
    let toggleThresholdsButton = null;
    let thresholdBadge = null;
    let createAlertRuleBtn = null;
    let viewAlertRulesBtn = null;
    let activeThresholdsSummary = null;
    let sliders = {};
    let thresholdSelects = {};
    let isDraggingSlider = false;
    let updateThrottleTimer = null;
    let alertNameManuallyEdited = false;

    function init() {
        thresholdRow = document.getElementById('threshold-slider-row');
        alertRuleRow = document.getElementById('alert-rule-row');
        toggleThresholdsButton = document.getElementById('toggle-thresholds-checkbox');
        thresholdBadge = document.getElementById('threshold-count-badge');
        createAlertRuleBtn = document.getElementById('expand-alert-builder-btn'); // Updated to new ID
        viewAlertRulesBtn = document.getElementById('view-alert-rules-btn');
        activeThresholdsSummary = document.getElementById('active-thresholds-summary');

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
            toggleThresholdsButton.addEventListener('change', () => {
                PulseApp.state.set('isThresholdRowVisible', toggleThresholdsButton.checked);
                updateThresholdRowVisibility();
            });
        } else {
            console.warn('#toggle-thresholds-checkbox not found.');
        }


        _setupSliderListeners();
        _setupSelectListeners();
        _setupDragEndListeners();
        _setupAlertRuleListeners();
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

    function _setupAlertRuleListeners() {
        if (viewAlertRulesBtn) {
            viewAlertRulesBtn.addEventListener('click', _handleViewAlertRules);
        }

        // New integrated alert builder listeners (replaces legacy modal approach)
        _setupIntegratedAlertBuilder();
    }


    function _handleCreateAlertRule() {
        const thresholdState = PulseApp.state.getThresholdState();
        const activeThresholds = _getActiveThresholds(thresholdState);
        
        if (activeThresholds.length === 0) {
            PulseApp.ui.toast.warning('Please set at least one threshold to create an alert rule.');
            return;
        }

        // Use the new integrated alert builder instead of modal
        _expandAlertBuilder();
    }

    function _handleViewAlertRules() {
        // Open the Alert Management modal directly to the Alert Rules tab
        if (PulseApp.ui && PulseApp.ui.alertManagementModal && PulseApp.ui.alertManagementModal.openModal) {
            PulseApp.ui.alertManagementModal.openModal('alert-rules');
        } else {
            // Fallback to the old modal if alert management isn't available
            _showViewAlertRulesModal();
        }
    }

    function _showCreateAlertRuleModal(activeThresholds) {
        // Create a modal for alert rule creation
        const modalHtml = `
            <div id="alert-rule-modal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
                    <div class="p-6">
                        <div class="flex items-center justify-between mb-4">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                Create Alert Rule from Thresholds
                            </h3>
                            <button id="close-alert-modal" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                                <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                                </svg>
                            </button>
                        </div>
                        
                        <form id="alert-rule-form">
                            <div class="space-y-4">
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                        Rule Name
                                    </label>
                                    <input type="text" id="rule-name" required
                                           placeholder="e.g., High Resource Usage Alert"
                                           class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
                                </div>
                                
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                        Description
                                    </label>
                                    <textarea id="rule-description" rows="2"
                                              placeholder="Optional description of what this alert monitors"
                                              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"></textarea>
                                </div>
                                
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                        Alert Conditions
                                    </label>
                                    <div class="bg-gray-50 dark:bg-gray-700 rounded-md p-3">
                                        <p class="text-sm text-gray-600 dark:text-gray-400 mb-2">
                                            Alert will trigger when ANY guest meets ALL of these conditions:
                                        </p>
                                        <ul class="space-y-1">
                                            ${activeThresholds.map(threshold => `
                                                <li class="text-sm text-gray-800 dark:text-gray-200">
                                                    • ${_getThresholdDisplayName(threshold.type)} ≥ ${_formatThresholdValue(threshold)}
                                                </li>
                                            `).join('')}
                                        </ul>
                                    </div>
                                </div>
                                
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                                        Notification methods
                                    </label>
                                    <div class="grid grid-cols-1 gap-3">
                                        <!-- In-app Notifications Card -->
                                        <div class="notification-card border-2 border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4 cursor-pointer transition-all" onclick="PulseApp.ui.thresholds.toggleNotificationCard('local')">
                                            <div class="flex items-center justify-between">
                                                <div class="flex items-center space-x-3">
                                                    <div class="flex-shrink-0">
                                                        <div class="w-10 h-10 bg-blue-500 rounded-lg flex items-center justify-center">
                                                            <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-5 5v-5zm-2-8V7a1 1 0 00-1-1H5a1 1 0 00-1 1v3a1 1 0 001 1h7a1 1 0 001-1z"></path>
                                                            </svg>
                                                        </div>
                                                    </div>
                                                    <div class="flex-1">
                                                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">In-app Alerts</h3>
                                                        <p class="text-xs text-gray-600 dark:text-gray-400">Shows in Pulse dashboard notifications</p>
                                                        <div class="flex items-center mt-1">
                                                            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                                                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                                                </svg>
                                                                Ready
                                                            </span>
                                                        </div>
                                                    </div>
                                                </div>
                                                <div class="flex-shrink-0">
                                                    <input type="checkbox" id="notify-local" checked class="w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                                                </div>
                                            </div>
                                        </div>

                                        <!-- Email Notifications Card -->
                                        <div class="notification-card border-2 border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 rounded-lg p-4 cursor-pointer transition-all hover:border-gray-300 dark:hover:border-gray-500" onclick="PulseApp.ui.thresholds.toggleNotificationCard('email')">
                                            <div class="flex items-center justify-between">
                                                <div class="flex items-center space-x-3">
                                                    <div class="flex-shrink-0">
                                                        <div class="w-10 h-10 bg-gray-100 dark:bg-gray-600 rounded-lg flex items-center justify-center">
                                                            <svg class="w-5 h-5 text-gray-600 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 4.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"></path>
                                                            </svg>
                                                        </div>
                                                    </div>
                                                    <div class="flex-1">
                                                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Email (SMTP)</h3>
                                                        <p class="text-xs text-gray-600 dark:text-gray-400">Send alerts via email notifications</p>
                                                        <div class="flex items-center mt-1">
                                                            <span id="email-status" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                                                </svg>
                                                                Setup required
                                                            </span>
                                                        </div>
                                                    </div>
                                                </div>
                                                <div class="flex-shrink-0">
                                                    <input type="checkbox" id="notify-email" class="w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                                                </div>
                                            </div>
                                        </div>

                                        <!-- Discord Notifications Card -->
                                        <div class="notification-card border-2 border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 rounded-lg p-4 cursor-pointer transition-all hover:border-gray-300 dark:hover:border-gray-500" onclick="PulseApp.ui.thresholds.toggleNotificationCard('discord')">
                                            <div class="flex items-center justify-between">
                                                <div class="flex items-center space-x-3">
                                                    <div class="flex-shrink-0">
                                                        <div class="w-10 h-10 bg-indigo-500 rounded-lg flex items-center justify-center">
                                                            <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                                                                <path d="M20.317 4.37a19.791 19.791 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028 14.09 14.09 0 001.226-1.994.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
                                                            </svg>
                                                        </div>
                                                    </div>
                                                    <div class="flex-1">
                                                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Discord</h3>
                                                        <p class="text-xs text-gray-600 dark:text-gray-400">Send alerts to Discord channels</p>
                                                        <div class="flex items-center mt-1">
                                                            <span id="discord-status" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                                                </svg>
                                                                Setup required
                                                            </span>
                                                        </div>
                                                    </div>
                                                </div>
                                                <div class="flex-shrink-0">
                                                    <input type="checkbox" id="notify-discord" class="w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                                                </div>
                                            </div>
                                        </div>

                                        <!-- Slack Notifications Card -->
                                        <div class="notification-card border-2 border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 rounded-lg p-4 cursor-pointer transition-all hover:border-gray-300 dark:hover:border-gray-500" onclick="PulseApp.ui.thresholds.toggleNotificationCard('slack')">
                                            <div class="flex items-center justify-between">
                                                <div class="flex items-center space-x-3">
                                                    <div class="flex-shrink-0">
                                                        <div class="w-10 h-10 bg-green-500 rounded-lg flex items-center justify-center">
                                                            <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                                                                <path d="M5.042 15.165a2.528 2.528 0 01-2.52-2.523A2.528 2.528 0 015.042 10.12h2.52v2.522a2.528 2.528 0 01-2.52 2.523zM6.313 17.688a2.528 2.528 0 012.52-2.523 2.528 2.528 0 012.523 2.523v6.349A2.528 2.528 0 018.833 26.56a2.528 2.528 0 01-2.52-2.523v-6.349zM8.833 5.042a2.528 2.528 0 01-2.52-2.52A2.528 2.528 0 018.833 0a2.528 2.528 0 012.523 2.522v2.52H8.833zM11.356 6.313a2.528 2.528 0 012.523-2.52 2.528 2.528 0 012.523 2.52 2.528 2.528 0 01-2.523 2.523h-2.523V6.313zM18.956 8.833a2.528 2.528 0 012.523 2.52 2.528 2.528 0 01-2.523 2.523h-2.523V8.833h2.523zM17.688 11.356a2.528 2.528 0 012.523 2.523 2.528 2.528 0 01-2.523 2.523H11.34a2.528 2.528 0 01-2.523-2.523 2.528 2.528 0 012.523-2.523h6.348z"/>
                                                            </svg>
                                                        </div>
                                                    </div>
                                                    <div class="flex-1">
                                                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Slack</h3>
                                                        <p class="text-xs text-gray-600 dark:text-gray-400">Send alerts to Slack channels</p>
                                                        <div class="flex items-center mt-1">
                                                            <span id="slack-status" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                                                </svg>
                                                                Setup required
                                                            </span>
                                                        </div>
                                                    </div>
                                                </div>
                                                <div class="flex-shrink-0">
                                                    <input type="checkbox" id="notify-slack" class="w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                                                </div>
                                            </div>
                                        </div>

                                        <!-- Custom Webhook Card -->
                                        <div class="notification-card border-2 border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 rounded-lg p-4 cursor-pointer transition-all hover:border-gray-300 dark:hover:border-gray-500" onclick="PulseApp.ui.thresholds.toggleNotificationCard('webhook')">
                                            <div class="flex items-center justify-between">
                                                <div class="flex items-center space-x-3">
                                                    <div class="flex-shrink-0">
                                                        <div class="w-10 h-10 bg-gray-100 dark:bg-gray-600 rounded-lg flex items-center justify-center">
                                                            <svg class="w-5 h-5 text-gray-600 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                                                            </svg>
                                                        </div>
                                                    </div>
                                                    <div class="flex-1">
                                                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Custom Webhook</h3>
                                                        <p class="text-xs text-gray-600 dark:text-gray-400">Send to any custom API endpoint</p>
                                                        <div class="flex items-center mt-1">
                                                            <span id="webhook-status" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                                                </svg>
                                                                Setup required
                                                            </span>
                                                        </div>
                                                    </div>
                                                </div>
                                                <div class="flex-shrink-0">
                                                    <input type="checkbox" id="notify-webhook" class="w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-3 text-center">
                                        💡 Choose one or more notification methods
                                    </p>
                                </div>
                            </div>
                            
                            <div class="flex justify-end gap-3 mt-6">
                                <button type="button" id="cancel-alert-rule" 
                                        class="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-200 dark:bg-gray-600 rounded-md hover:bg-gray-300 dark:hover:bg-gray-500">
                                    Cancel
                                </button>
                                <button type="submit" 
                                        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md">
                                    Create Alert Rule
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        `;
        
        document.body.insertAdjacentHTML('beforeend', modalHtml);
        _setupAlertRuleModalListeners(activeThresholds);
        _initializeNotificationCards();
    }

    function _formatThresholdValue(threshold) {
        if (['cpu', 'memory', 'disk'].includes(threshold.type)) {
            return `${threshold.value}%`;
        } else {
            return _formatBytesThreshold(threshold.value);
        }
    }

    function _setupAlertRuleModalListeners(activeThresholds) {
        const modal = document.getElementById('alert-rule-modal');
        const closeBtn = document.getElementById('close-alert-modal');
        const cancelBtn = document.getElementById('cancel-alert-rule');
        const form = document.getElementById('alert-rule-form');

        const closeModal = () => {
            modal.remove();
        };

        closeBtn.addEventListener('click', closeModal);
        cancelBtn.addEventListener('click', closeModal);
        
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeModal();
        });

        form.addEventListener('submit', (e) => {
            e.preventDefault();
            _handleAlertRuleSubmit(activeThresholds);
            closeModal();
        });
    }

    async function _handleAlertRuleSubmit(activeThresholds) {
        const ruleName = document.getElementById('rule-name').value;
        const ruleDescription = document.getElementById('rule-description').value;
        
        // Collect notification preferences
        const notifyLocal = document.getElementById('notify-local').checked;
        const notifyEmail = document.getElementById('notify-email').checked;
        const notifyDiscord = document.getElementById('notify-discord').checked;
        const notifySlack = document.getElementById('notify-slack').checked;
        const notifyWebhook = document.getElementById('notify-webhook').checked;
        
        const notificationChannels = [];
        if (notifyLocal) notificationChannels.push('local');
        if (notifyEmail) notificationChannels.push('email');
        if (notifyDiscord) notificationChannels.push('discord');
        if (notifySlack) notificationChannels.push('slack');
        if (notifyWebhook) notificationChannels.push('webhook');
        
        if (notificationChannels.length === 0) {
            _showInlineMessage('Please select at least one notification method.', 'error');
            return;
        }

        const alertRule = {
            name: ruleName,
            description: ruleDescription,
            thresholds: activeThresholds,
            notificationChannels: notificationChannels,
            enabled: true
        };


        try {
            const response = await fetch('/api/alerts/rules', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(alertRule)
            });

            const result = await response.json();

            if (response.ok && result.success) {
                const notificationMethods = notificationChannels.map(channel => {
                    switch (channel) {
                        case 'local': return 'in-app alerts';
                        case 'email': return 'email notifications';
                        case 'discord': return 'Discord notifications';
                        case 'slack': return 'Slack notifications';
                        case 'webhook': return 'custom webhook';
                        default: return channel;
                    }
                }).join(', ');
                
                _showSuccessNotification(
                    `Alert rule "${ruleName}" created successfully!`,
                    `Monitoring ${activeThresholds.length} threshold${activeThresholds.length > 1 ? 's' : ''} with ${notificationMethods}`
                );
            } else {
                throw new Error(result.error || 'Failed to create alert rule');
            }
        } catch (error) {
            console.error('Error creating alert rule:', error);
            _showInlineMessage(`Failed to create alert rule: ${error.message}`, 'error');
        }
    }

    async function _showViewAlertRulesModal() {
        try {
            const response = await fetch('/api/alerts/compound-rules');
            const result = await response.json();

            if (!response.ok || !result.success) {
                throw new Error(result.error || 'Failed to fetch alert rules');
            }

            const rules = result.rules || [];
            _displayAlertRulesModal(rules);
        } catch (error) {
            console.error('Error fetching alert rules:', error);
            PulseApp.ui.toast.error(`Failed to fetch alert rules: ${error.message}`);
        }
    }

    function _displayAlertRulesModal(rules) {
        const modalHtml = `
            <div id="view-rules-modal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-y-auto">
                    <div class="p-6">
                        <div class="flex items-center justify-between mb-4">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                Dynamic Threshold Alert Rules
                            </h3>
                            <button id="close-rules-modal" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                                <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                                </svg>
                            </button>
                        </div>
                        
                        ${rules.length === 0 ? `
                            <div class="text-center py-8">
                                <svg class="h-12 w-12 text-gray-400 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"></path>
                                </svg>
                                <p class="text-gray-500 dark:text-gray-400 mb-2">No dynamic threshold rules created yet</p>
                                <p class="text-sm text-gray-400 dark:text-gray-500">Set some thresholds above and click "Create Alert Rule" to get started!</p>
                            </div>
                        ` : `
                            <div class="space-y-4">
                                ${rules.map(rule => _formatRuleCard(rule)).join('')}
                            </div>
                        `}
                    </div>
                </div>
            </div>
        `;
        
        document.body.insertAdjacentHTML('beforeend', modalHtml);
        _setupViewRulesModalListeners();
    }

    function _formatRuleCard(rule) {
        const thresholdsList = rule.thresholds.map(t => 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${_getThresholdDisplayName(t.type)} ≥ ${_formatThresholdValue(t)}
            </span>`
        ).join('');

        const createdDate = new Date(rule.createdAt).toLocaleDateString();
        const alertColor = 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';

        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-4">
                <div class="flex items-start justify-between mb-3">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-2">
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">${rule.name}</h4>
                            <span class="px-2 py-1 text-xs font-medium rounded ${rule.enabled ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}">
                                ${rule.enabled ? 'ENABLED' : 'DISABLED'}
                            </span>
                        </div>
                        ${rule.description ? `<p class="text-sm text-gray-600 dark:text-gray-400 mb-2">${rule.description}</p>` : ''}
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Created: ${createdDate} • ID: ${rule.id}
                        </div>
                    </div>
                    <div class="flex gap-2">
                        <button onclick="PulseApp.ui.thresholds.toggleRule('${rule.id}', ${!rule.enabled})" 
                                class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded">
                            ${rule.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button onclick="PulseApp.ui.thresholds.deleteRule('${rule.id}')" 
                                class="px-2 py-1 text-xs bg-red-600 hover:bg-red-700 text-white rounded">
                            Delete
                        </button>
                    </div>
                </div>
                <div>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Alert triggers when ANY guest meets ALL conditions:</p>
                    <div class="flex flex-wrap">
                        ${thresholdsList}
                    </div>
                </div>
            </div>
        `;
    }

    function _setupViewRulesModalListeners() {
        const modal = document.getElementById('view-rules-modal');
        const closeBtn = document.getElementById('close-rules-modal');

        const closeModal = () => {
            modal.remove();
        };

        closeBtn.addEventListener('click', closeModal);
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeModal();
        });
    }

    async function toggleRule(ruleId, enabled) {
        try {
            const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ enabled })
            });

            const result = await response.json();

            if (response.ok && result.success) {
                // Refresh the modal
                document.getElementById('view-rules-modal').remove();
                _showViewAlertRulesModal();
            } else {
                throw new Error(result.error || 'Failed to update rule');
            }
        } catch (error) {
            console.error('Error updating rule:', error);
            PulseApp.ui.toast.error(`Failed to update rule: ${error.message}`);
        }
    }

    async function deleteRule(ruleId) {
        PulseApp.ui.toast.confirm(
            'Are you sure you want to delete this alert rule? This action cannot be undone.',
            async () => {
                await _performDeleteRule(ruleId);
            }
        );
    }

    async function _performDeleteRule(ruleId) {

        try {
            const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                method: 'DELETE'
            });

            const result = await response.json();

            if (response.ok && result.success) {
                // Refresh the modal
                document.getElementById('view-rules-modal').remove();
                _showViewAlertRulesModal();
            } else {
                throw new Error(result.error || 'Failed to delete rule');
            }
        } catch (error) {
            console.error('Error deleting rule:', error);
            PulseApp.ui.toast.error(`Failed to delete rule: ${error.message}`);
        }
    }

    // Notification functions for better UX
    function _showInlineMessage(message, type = 'error') {
        // Create inline message in the modal
        const existingMessage = document.querySelector('.alert-inline-message');
        if (existingMessage) existingMessage.remove();
        
        const messageDiv = document.createElement('div');
        messageDiv.className = `alert-inline-message mt-4 p-3 rounded-md ${
            type === 'error' 
                ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300'
                : 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300'
        }`;
        messageDiv.innerHTML = `
            <div class="flex items-center">
                <svg class="h-4 w-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="${
                        type === 'error' 
                            ? 'M18 10a8 8 0 11-16 0 8 8 0 0116 0zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z'
                            : 'M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z'
                    }" clip-rule="evenodd"></path>
                </svg>
                <span class="text-sm">${message}</span>
            </div>
        `;
        
        const form = document.getElementById('alert-rule-form');
        if (form) {
            form.appendChild(messageDiv);
        }
    }

    function _showSuccessNotification(title, subtitle) {
        // Create a beautiful toast notification
        const toastId = `toast-${Date.now()}`;
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = 'fixed bottom-4 left-4 z-50 max-w-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg transform -translate-x-full transition-transform duration-300';
        toast.innerHTML = `
            <div class="p-4">
                <div class="flex items-start">
                    <div class="flex-shrink-0">
                        <svg class="h-6 w-6 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                    </div>
                    <div class="ml-3 flex-1">
                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">${title}</p>
                        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">${subtitle}</p>
                    </div>
                    <div class="ml-4 flex-shrink-0">
                        <button onclick="document.getElementById('${toastId}').remove()" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                            </svg>
                        </button>
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(toast);
        
        // Animate in
        requestAnimationFrame(() => {
            toast.classList.remove('-translate-x-full');
        });
        
        // Auto remove after 5 seconds
        setTimeout(() => {
            if (document.getElementById(toastId)) {
                toast.classList.add('-translate-x-full');
                setTimeout(() => toast.remove(), 300);
            }
        }, 5000);
        
        // Close any open modals
        const modal = document.getElementById('alert-rule-modal');
        if (modal) modal.remove();
    }

    // Global function to handle notification card interactions
    function toggleNotificationCard(type) {
        const card = document.querySelector(`.notification-card[onclick*="${type}"]`);
        const checkbox = document.getElementById(`notify-${type}`);
        
        if (!card || !checkbox) return;
        
        // Toggle checkbox state
        checkbox.checked = !checkbox.checked;
        
        // Update card visual state
        updateNotificationCardState(card, checkbox, type);
    }

    function updateNotificationCardState(card, checkbox, type) {
        if (checkbox.checked) {
            // Selected state
            card.classList.remove('border-gray-200', 'dark:border-gray-600', 'bg-white', 'dark:bg-gray-700');
            card.classList.add('border-blue-500', 'dark:border-blue-400', 'bg-blue-50', 'dark:bg-blue-900/30');
            
            // Add selection indicator
            if (!card.querySelector('.selection-indicator')) {
                const indicator = document.createElement('div');
                indicator.className = 'selection-indicator absolute -top-1 -right-1 w-5 h-5 bg-blue-500 rounded-full flex items-center justify-center';
                indicator.innerHTML = '<svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path></svg>';
                card.style.position = 'relative';
                card.appendChild(indicator);
            }
        } else {
            // Unselected state
            card.classList.remove('border-blue-500', 'dark:border-blue-400', 'bg-blue-50', 'dark:bg-blue-900/30');
            card.classList.add('border-gray-200', 'dark:border-gray-600', 'bg-white', 'dark:bg-gray-700');
            
            // Remove selection indicator
            const indicator = card.querySelector('.selection-indicator');
            if (indicator) {
                indicator.remove();
            }
        }
    }

    // Initialize notification card states when modal is created
    function _initializeNotificationCards() {
        // Check email and webhook setup status and update card states
        _checkNotificationSetupStatus();
        
        // Set up initial visual states for all cards
        ['local', 'email', 'discord', 'slack', 'webhook'].forEach(type => {
            const card = document.querySelector(`.notification-card[onclick*="${type}"]`);
            const checkbox = document.getElementById(`notify-${type}`);
            
            if (card && checkbox) {
                updateNotificationCardState(card, checkbox, type);
            }
        });
    }

    async function _checkNotificationSetupStatus() {
        try {
            // Check if SMTP and webhook are configured
            const response = await fetch('/api/config');
            const config = await response.json();
            
            const emailStatus = document.getElementById('email-status');
            const webhookStatus = document.getElementById('webhook-status');
            
            
            // Update email status - check for SMTP configuration in advanced.smtp
            if (emailStatus) {
                const hasEmailConfig = config.advanced && config.advanced.smtp && 
                    config.advanced.smtp.host && config.advanced.smtp.user;
                
                
                if (hasEmailConfig) {
                    emailStatus.innerHTML = `
                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                        </svg>
                        Ready
                    `;
                    emailStatus.className = 'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
                } else {
                }
            }
            
            // Get webhook URL for service detection
            const webhookUrl = config.advanced && config.advanced.webhook && config.advanced.webhook.url;
            
            // Update Discord status
            const discordStatus = document.getElementById('discord-status');
            if (discordStatus) {
                const isDiscordWebhook = webhookUrl && 
                    (webhookUrl.includes('discord.com/api/webhooks') || webhookUrl.includes('discordapp.com/api/webhooks'));
                
                if (isDiscordWebhook) {
                    discordStatus.innerHTML = `
                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                        </svg>
                        Ready
                    `;
                    discordStatus.className = 'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
                }
            }
            
            // Update Slack status
            const slackStatus = document.getElementById('slack-status');
            if (slackStatus) {
                const isSlackWebhook = webhookUrl && 
                    webhookUrl.includes('hooks.slack.com');
                
                if (isSlackWebhook) {
                    slackStatus.innerHTML = `
                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                        </svg>
                        Ready
                    `;
                    slackStatus.className = 'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
                }
            }
            
            // Update custom webhook status - check for non-Discord/Slack webhooks
            if (webhookStatus) {
                // Be smarter about what constitutes a "real" webhook setup
                // Exclude localhost URLs, test URLs, and common placeholder patterns
                const isRealWebhook = webhookUrl && 
                    webhookUrl.trim() !== '' &&
                    !webhookUrl.includes('localhost') &&
                    !webhookUrl.includes('127.0.0.1') &&
                    !webhookUrl.includes('example.com') &&
                    !webhookUrl.includes('test') &&
                    !webhookUrl.includes('placeholder') &&
                    (webhookUrl.startsWith('https://') || webhookUrl.startsWith('http://')) &&
                    webhookUrl.length > 10 && // Reasonable minimum URL length
                    !webhookUrl.includes('discord.com/api/webhooks') &&
                    !webhookUrl.includes('discordapp.com/api/webhooks') &&
                    !webhookUrl.includes('hooks.slack.com'); // Exclude Discord/Slack URLs for custom webhook
                
                
                if (isRealWebhook) {
                    webhookStatus.innerHTML = `
                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                        </svg>
                        Ready
                    `;
                    webhookStatus.className = 'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
                } else {
                }
            }
        } catch (error) {
            console.warn('[Thresholds] Could not check notification setup status:', error);
        }
    }

    function updateThreshold(type, value, immediate = false) {
        PulseApp.state.setThresholdValue(type, value);
        updateThresholdIndicator();

        // Always update dashboard immediately for live responsiveness
        updateDashboardFromThreshold();
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
        
        const rows = tableBody.querySelectorAll('tr:not(.node-header)');
        rows.forEach(row => {
            const guestName = row.getAttribute('data-name');
            if (!guestName) return;
            
            // Find guest data by name
            const dashboardData = PulseApp.state.get('dashboardData') || [];
            const guest = dashboardData.find(g => g.name.toLowerCase() === guestName);
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

    function updateThresholdRowVisibility() {
        const isVisible = PulseApp.state.get('isThresholdRowVisible');
        if (thresholdRow) {
            thresholdRow.classList.toggle('hidden', !isVisible);
            if (toggleThresholdsButton) {
                // Update checkbox state to match visibility
                toggleThresholdsButton.checked = isVisible;
            }
        }
        
        // Show alert rule row only when threshold row is visible
        if (alertRuleRow) {
            alertRuleRow.classList.toggle('hidden', !isVisible);
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
        const activeThresholds = _getActiveThresholds(thresholdState);

        if (activeCount > 0) {
            thresholdBadge.textContent = activeCount;
            thresholdBadge.classList.remove('hidden');
        } else {
            thresholdBadge.classList.add('hidden');
        }

        // Update alert rule button state and summary
        _updateAlertRuleUI(activeThresholds, activeCount);
        
        // Update reset button highlighting when thresholds change
        if (PulseApp.ui && PulseApp.ui.common && PulseApp.ui.common.updateResetButtonState) {
            PulseApp.ui.common.updateResetButtonState();
        }
    }

    function _getActiveThresholds(thresholdState) {
        const active = [];
        for (const type in thresholdState) {
            if (thresholdState[type].value > 0) {
                active.push({
                    type: type,
                    value: thresholdState[type].value
                });
            }
        }
        return active;
    }

    function _updateAlertRuleUI(activeThresholds, activeCount) {
        // Update legacy button state (for fallback)
        if (createAlertRuleBtn) {
            createAlertRuleBtn.disabled = activeCount === 0;
            if (activeCount === 0) {
                createAlertRuleBtn.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                createAlertRuleBtn.classList.remove('opacity-50', 'cursor-not-allowed');
            }
        }

        // Update integrated alert builder button state
        const expandBtn = document.getElementById('expand-alert-builder-btn');
        if (expandBtn) {
            expandBtn.disabled = activeCount === 0;
            if (activeCount === 0) {
                expandBtn.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                expandBtn.classList.remove('opacity-50', 'cursor-not-allowed');
            }
        }

        // Update summary text
        if (activeThresholdsSummary) {
            if (activeCount > 0) {
                const summaryText = _formatThresholdSummary(activeThresholds);
                activeThresholdsSummary.textContent = `(${summaryText})`;
            } else {
                activeThresholdsSummary.textContent = '(Set thresholds above to enable)';
            }
        }

        // Update active thresholds display in builder
        _updateActiveThresholdsDisplay(activeThresholds);
        
        // Update real-time preview
        _updateAlertPreview(activeThresholds);
        
        // Update alert name if builder is expanded and name hasn't been manually edited
        _updateAlertNameIfNeeded(activeThresholds);
    }

    function _formatThresholdSummary(activeThresholds) {
        const formatted = activeThresholds.map(threshold => {
            const name = _getThresholdDisplayName(threshold.type);
            let value;
            
            if (['cpu', 'memory', 'disk'].includes(threshold.type)) {
                value = `${threshold.value}%`;
            } else {
                value = _formatBytesThreshold(threshold.value);
            }
            
            return `${name} > ${value}`;
        });
        
        return formatted.join(', ');
    }

    function _getThresholdDisplayName(type) {
        const names = {
            'cpu': 'CPU',
            'memory': 'Memory',
            'disk': 'Disk',
            'diskread': 'Disk Read',
            'diskwrite': 'Disk Write',
            'netin': 'Net In',
            'netout': 'Net Out'
        };
        return names[type] || type;
    }

    function _formatBytesThreshold(bytes) {
        return PulseApp.utils.formatSpeed(bytes, 0);
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
        // Don't close threshold row - keep it open for user to set new thresholds
        // PulseApp.state.set('isThresholdRowVisible', false);
        // updateThresholdRowVisibility();
        updateThresholdIndicator();
        
        // Clear any dimming from rows
        clearAllRowDimming();
    }

    // Getter for dashboard.js to check drag state
    function isThresholdDragInProgress() {
        return isDraggingSlider;
    }

    // ========================================
    // INTEGRATED ALERT BUILDER FUNCTIONS
    // ========================================

    function _setupIntegratedAlertBuilder() {
        const expandBtn = document.getElementById('expand-alert-builder-btn');
        const collapseBtn = document.getElementById('collapse-alert-builder-btn');
        const cancelBtn = document.getElementById('integrated-cancel-btn');
        const form = document.getElementById('integrated-alert-form');

        if (expandBtn) {
            expandBtn.addEventListener('click', _expandAlertBuilder);
        }

        if (collapseBtn) {
            collapseBtn.addEventListener('click', _collapseAlertBuilder);
        }

        if (cancelBtn) {
            cancelBtn.addEventListener('click', _collapseAlertBuilder);
        }

        if (form) {
            form.addEventListener('submit', _handleIntegratedAlertSubmit);
        }

        // Setup simplified target selection
        _setupSimpleTargetSelection();

        // Add listener for condition logic changes to update preview
        const conditionLogicSelect = document.getElementById('integrated-condition-logic');
        if (conditionLogicSelect) {
            conditionLogicSelect.addEventListener('change', () => {
                const thresholdState = PulseApp.state.getThresholdState();
                const activeThresholds = _getActiveThresholds(thresholdState);
                _updateAlertPreview(activeThresholds);
            });
        }

        // Setup test button
        const testBtn = document.getElementById('integrated-test-alert-btn');

        if (testBtn) {
            testBtn.addEventListener('click', _testAlert);
        }

        // Track manual edits to alert name
        const alertNameInput = document.getElementById('integrated-alert-name');
        if (alertNameInput) {
            alertNameInput.addEventListener('input', () => {
                alertNameManuallyEdited = true;
            });
        }
    }

    function _expandAlertBuilder() {
        const collapsed = document.getElementById('alert-builder-collapsed');
        const expanded = document.getElementById('alert-builder-expanded');
        const alertNameInput = document.getElementById('integrated-alert-name');

        if (collapsed && expanded) {
            collapsed.classList.add('hidden');
            expanded.classList.remove('hidden');

            // Auto-generate alert name based on active thresholds
            const thresholdState = PulseApp.state.getThresholdState();
            const activeThresholds = _getActiveThresholds(thresholdState);
            _generateAlertName(activeThresholds);

            // Focus on name input
            if (alertNameInput) {
                setTimeout(() => alertNameInput.focus(), 100);
            }

            // Load VM/Node lists for target selection
            _loadTargetOptions();
        }
    }

    function _collapseAlertBuilder() {
        const collapsed = document.getElementById('alert-builder-collapsed');
        const expanded = document.getElementById('alert-builder-expanded');
        const form = document.getElementById('integrated-alert-form');

        if (collapsed && expanded) {
            expanded.classList.add('hidden');
            collapsed.classList.remove('hidden');

            // Reset form
            if (form) {
                form.reset();
                // Re-select default values
                const conditionLogicSelect = document.getElementById('integrated-condition-logic');
                const localNotification = form.querySelector('input[name="notifications"][value="local"]');
                const allRadio = form.querySelector('input[name="targetScope"][value="all"]');

                if (conditionLogicSelect) conditionLogicSelect.value = 'any';
                if (localNotification) localNotification.checked = true;
                if (allRadio) allRadio.checked = true;
            }

            // Reset manual edit tracking
            alertNameManuallyEdited = false;
        }
    }

    function _generateAlertName(activeThresholds) {
        const alertNameInput = document.getElementById('integrated-alert-name');
        if (!alertNameInput || activeThresholds.length === 0) return;

        let suggestedName = '';
        
        if (activeThresholds.length === 1) {
            const threshold = activeThresholds[0];
            const metricName = _getThresholdDisplayName(threshold.type);
            const value = _formatThresholdValue(threshold);
            suggestedName = `High ${metricName} Alert (>${value})`;
        } else {
            const metrics = activeThresholds.map(t => _getThresholdDisplayName(t.type)).join(' & ');
            suggestedName = `High Resource Usage Alert (${metrics})`;
        }

        alertNameInput.value = suggestedName;
    }

    function _updateAlertNameIfNeeded(activeThresholds) {
        // Only update if builder is expanded and name hasn't been manually edited
        const expanded = document.getElementById('alert-builder-expanded');
        const alertNameInput = document.getElementById('integrated-alert-name');
        
        if (!expanded || expanded.classList.contains('hidden') || alertNameManuallyEdited || !alertNameInput) {
            return;
        }

        // Generate new name based on current thresholds
        if (activeThresholds.length === 0) {
            alertNameInput.value = '';
            return;
        }

        let suggestedName = '';
        
        if (activeThresholds.length === 1) {
            const threshold = activeThresholds[0];
            const metricName = _getThresholdDisplayName(threshold.type);
            const value = _formatThresholdValue(threshold);
            suggestedName = `High ${metricName} Alert (>${value})`;
        } else {
            const metrics = activeThresholds.map(t => _getThresholdDisplayName(t.type)).join(' & ');
            suggestedName = `High Resource Usage Alert (${metrics})`;
        }

        alertNameInput.value = suggestedName;
    }

    function _updateActiveThresholdsDisplay(activeThresholds) {
        const display = document.getElementById('active-thresholds-display');
        if (!display) return;

        if (activeThresholds.length === 0) {
            display.innerHTML = '<p class="text-blue-700 dark:text-blue-300">Set thresholds above to configure alert conditions</p>';
            return;
        }

        const thresholdsList = activeThresholds.map(threshold => `
            <div class="flex items-center justify-between py-1">
                <span class="text-blue-800 dark:text-blue-200 font-medium">${_getThresholdDisplayName(threshold.type)}</span>
                <span class="text-blue-600 dark:text-blue-400">≥ ${_formatThresholdValue(threshold)}</span>
            </div>
        `).join('');

        display.innerHTML = thresholdsList;
    }

    function _updateAlertPreview(activeThresholds) {
        const previewCount = document.getElementById('preview-trigger-count');
        const previewList = document.getElementById('preview-vm-list');
        const alertPreviewCount = document.getElementById('alert-preview-count');
        const matchingVmsCount = document.getElementById('matching-vms-count');

        if (!previewCount || !previewList) return;

        if (activeThresholds.length === 0) {
            previewCount.textContent = '0';
            previewList.innerHTML = '<p>Set thresholds to see preview</p>';
            if (alertPreviewCount) alertPreviewCount.classList.add('hidden');
            return;
        }

        // Get current dashboard data
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const conditionLogic = document.getElementById('integrated-condition-logic')?.value || 'any';
        
        // Filter VMs that would trigger the alert
        const triggeringVMs = dashboardData.filter(vm => {
            if (!vm || typeof vm !== 'object') return false;

            const meetsConditions = activeThresholds.map(threshold => {
                let vmValue;
                switch (threshold.type) {
                    case 'cpu': vmValue = vm.cpu; break;
                    case 'memory': vmValue = vm.memory; break;
                    case 'disk': vmValue = vm.disk; break;
                    case 'diskread': vmValue = vm.diskread; break;
                    case 'diskwrite': vmValue = vm.diskwrite; break;
                    case 'netin': vmValue = vm.netin; break;
                    case 'netout': vmValue = vm.netout; break;
                    default: return false;
                }

                if (vmValue === undefined || vmValue === null || vmValue === 'N/A' || isNaN(vmValue)) {
                    return false;
                }

                return vmValue >= threshold.value;
            });

            // Apply condition logic (any vs all)
            return conditionLogic === 'any' ? 
                meetsConditions.some(condition => condition) : 
                meetsConditions.every(condition => condition);
        });

        const count = triggeringVMs.length;
        previewCount.textContent = count;

        if (matchingVmsCount) {
            matchingVmsCount.textContent = count;
        }

        if (alertPreviewCount) {
            if (count > 0) {
                alertPreviewCount.classList.remove('hidden');
            } else {
                alertPreviewCount.classList.add('hidden');
            }
        }

        // Update high usage count in target selection
        _updateHighUsageCount();

        if (count === 0) {
            previewList.innerHTML = '<p class="text-gray-500">No VMs currently meet these conditions</p>';
        } else {
            const vmNames = triggeringVMs.slice(0, 5).map(vm => 
                `<span class="inline-block bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 px-2 py-0.5 rounded text-xs mr-1 mb-1 font-medium">${vm.name || vm.id}</span>`
            ).join('');
            
            const extraCount = triggeringVMs.length - 5;
            const extra = extraCount > 0 ? `<span class="text-xs text-gray-500 dark:text-gray-400">+${extraCount} more</span>` : '';
            
            previewList.innerHTML = vmNames + extra;
        }
    }

    // ========================================
    // SIMPLIFIED TARGET SELECTION
    // ========================================

    function _setupSimpleTargetSelection() {
        const radioButtons = document.querySelectorAll('input[name="targetScope"]');
        const specificSelection = document.getElementById('specific-vm-selection');

        // Listen for radio button changes
        radioButtons.forEach(radio => {
            radio.addEventListener('change', () => {
                if (radio.value === 'specific' && radio.checked) {
                    specificSelection.classList.remove('hidden');
                    _loadVMCheckboxes();
                } else {
                    specificSelection.classList.add('hidden');
                }
                _updateTargetTypeInput();
            });
        });

        // Setup quick select buttons
        const quickSelectHighUsage = document.getElementById('quick-select-high-usage');
        const quickSelectAll = document.getElementById('quick-select-all');
        const quickSelectNone = document.getElementById('quick-select-none');

        if (quickSelectHighUsage) {
            quickSelectHighUsage.addEventListener('click', _selectHighUsageVMs);
        }

        if (quickSelectAll) {
            quickSelectAll.addEventListener('click', () => _toggleAllVMCheckboxes(true));
        }

        if (quickSelectNone) {
            quickSelectNone.addEventListener('click', () => _toggleAllVMCheckboxes(false));
        }
    }

    function _loadVMCheckboxes() {
        const grid = document.getElementById('vm-checkbox-grid');
        if (!grid) return;

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        grid.innerHTML = '';

        dashboardData.forEach(vm => {
            if (!vm || !(vm.name || vm.id)) return;

            const vmId = vm.id || vm.vmid;
            const vmName = vm.name || vmId;
            const vmType = vm.type?.toUpperCase() || 'VM';

            // Determine if this VM is currently high usage
            const isHighUsage = _isVMHighUsage(vm);
            const statusClass = isHighUsage ? 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800' : 
                                            'bg-white dark:bg-gray-700 border-gray-200 dark:border-gray-600';

            const checkboxDiv = document.createElement('div');
            checkboxDiv.className = `flex items-center gap-2 p-2 rounded border hover:bg-gray-50 dark:hover:bg-gray-600 ${statusClass}`;
            
            checkboxDiv.innerHTML = `
                <input type="checkbox" id="vm-${vmId}" value="${vmId}" class="vm-target-checkbox w-3 h-3 text-blue-600 border-gray-300 rounded focus:ring-blue-500">
                <label for="vm-${vmId}" class="flex-1 cursor-pointer text-xs">
                    <span class="font-medium text-gray-900 dark:text-gray-100">${vmName}</span>
                    <span class="text-gray-500 dark:text-gray-400 ml-1">(${vmType})</span>
                    ${isHighUsage ? '<span class="ml-1 text-xs text-yellow-600 dark:text-yellow-400">⚠ Threshold Met</span>' : ''}
                </label>
            `;

            grid.appendChild(checkboxDiv);

            // Add change listener
            const checkbox = checkboxDiv.querySelector('.vm-target-checkbox');
            checkbox.addEventListener('change', _updateVMSelection);
        });

        _updateHighUsageCount();
        _updateVMSelection();
    }

    function _isVMHighUsage(vm) {
        const thresholdState = PulseApp.state.getThresholdState();
        const activeThresholds = _getActiveThresholds(thresholdState);
        
        if (activeThresholds.length === 0) {
            // Fallback: consider high usage if any metric is > 80%
            return vm.cpu > 80 || vm.memory > 80 || vm.disk > 80;
        }

        // Check if VM meets current threshold conditions
        const conditionLogic = document.getElementById('integrated-condition-logic')?.value || 'any';
        const meetsConditions = activeThresholds.map(threshold => {
            let vmValue;
            switch (threshold.type) {
                case 'cpu': vmValue = vm.cpu; break;
                case 'memory': vmValue = vm.memory; break;
                case 'disk': vmValue = vm.disk; break;
                case 'diskread': vmValue = vm.diskread; break;
                case 'diskwrite': vmValue = vm.diskwrite; break;
                case 'netin': vmValue = vm.netin; break;
                case 'netout': vmValue = vm.netout; break;
                default: return false;
            }

            if (vmValue === undefined || vmValue === null || vmValue === 'N/A' || isNaN(vmValue)) {
                return false;
            }

            return vmValue >= threshold.value;
        });

        return conditionLogic === 'any' ? 
            meetsConditions.some(condition => condition) : 
            meetsConditions.every(condition => condition);
    }

    function _selectHighUsageVMs() {
        const checkboxes = document.querySelectorAll('.vm-target-checkbox');
        checkboxes.forEach(checkbox => {
            const vmId = checkbox.value;
            const dashboardData = PulseApp.state.get('dashboardData') || [];
            const vm = dashboardData.find(v => (v.id || v.vmid) == vmId);
            
            if (vm && _isVMHighUsage(vm)) {
                checkbox.checked = true;
            }
        });
        _updateVMSelection();
    }

    function _toggleAllVMCheckboxes(select) {
        const checkboxes = document.querySelectorAll('.vm-target-checkbox');
        checkboxes.forEach(checkbox => {
            checkbox.checked = select;
        });
        _updateVMSelection();
    }

    function _updateVMSelection() {
        const selectedVMs = Array.from(document.querySelectorAll('.vm-target-checkbox:checked')).map(cb => cb.value);
        const summary = document.getElementById('selection-summary');
        const selectedVMsInput = document.getElementById('selected-vms-input');

        // Update hidden input
        if (selectedVMsInput) {
            selectedVMsInput.value = JSON.stringify(selectedVMs);
        }

        // Update summary
        if (summary) {
            if (selectedVMs.length === 0) {
                summary.textContent = 'No VMs selected';
                summary.className = summary.className.replace('text-blue-700 dark:text-blue-300', 'text-gray-500 dark:text-gray-400');
            } else {
                summary.textContent = `${selectedVMs.length} VM${selectedVMs.length > 1 ? 's' : ''} selected for monitoring`;
                summary.className = summary.className.replace('text-gray-500 dark:text-gray-400', 'text-blue-700 dark:text-blue-300');
            }
        }
    }

    function _updateHighUsageCount() {
        const countSpan = document.getElementById('high-usage-count');
        if (!countSpan) return;

        const dashboardData = PulseApp.state.get('dashboardData') || [];
        const highUsageCount = dashboardData.filter(vm => vm && _isVMHighUsage(vm)).length;
        countSpan.textContent = highUsageCount;
    }

    function _updateTargetTypeInput() {
        const allRadio = document.querySelector('input[name="targetScope"][value="all"]');
        const targetTypeInput = document.getElementById('target-type-input');
        
        if (targetTypeInput) {
            targetTypeInput.value = allRadio?.checked ? 'all' : 'specific';
        }
    }

    async function _loadTargetOptions() {
        const specificTargetSelect = document.getElementById('integrated-specific-target');
        if (!specificTargetSelect) return;

        try {
            // Get current dashboard data for VMs and nodes
            const dashboardData = PulseApp.state.get('dashboardData') || [];
            const nodes = [...new Set(dashboardData.map(vm => vm.node).filter(Boolean))];

            // Clear and populate options
            specificTargetSelect.innerHTML = '<option value="">Select...</option>';

            // Add node options
            nodes.forEach(node => {
                const option = document.createElement('option');
                option.value = `node:${node}`;
                option.textContent = `Node: ${node}`;
                specificTargetSelect.appendChild(option);
            });

            // Add VM/LXC options
            dashboardData.forEach(vm => {
                if (vm.name || vm.id) {
                    const option = document.createElement('option');
                    option.value = `vm:${vm.id || vm.name}`;
                    option.textContent = `${vm.type?.toUpperCase() || 'VM'}: ${vm.name || vm.id}`;
                    specificTargetSelect.appendChild(option);
                }
            });
        } catch (error) {
            console.error('Error loading target options:', error);
        }
    }


    async function _handleIntegratedAlertSubmit(event) {
        event.preventDefault();
        
        const form = event.target;
        const formData = new FormData(form);
        const saveBtn = document.getElementById('integrated-save-alert-btn');

        // Get active thresholds
        const thresholdState = PulseApp.state.getThresholdState();
        const activeThresholds = _getActiveThresholds(thresholdState);

        if (activeThresholds.length === 0) {
            PulseApp.ui.toast.warning('Please set at least one threshold to create an alert rule.');
            return;
        }

        // Collect notification channels
        const notificationChannels = [];
        const notificationInputs = form.querySelectorAll('input[name="notifications"]:checked');
        notificationInputs.forEach(input => {
            notificationChannels.push(input.value);
        });

        if (notificationChannels.length === 0) {
            PulseApp.ui.toast.warning('Please select at least one notification method.');
            return;
        }

        // Transform thresholds to match AlertManager format
        const transformedThresholds = activeThresholds.map(threshold => ({
            metric: threshold.type,
            condition: 'greater_than_or_equal',
            threshold: threshold.value
        }));

        // Build alert rule object
        const alertRule = {
            name: formData.get('alertName'),
            description: formData.get('alertDescription') || '',
            thresholds: transformedThresholds,
            targetType: formData.get('targetType'),
            selectedVMs: formData.get('selectedVMs') || '[]',
            conditionLogic: formData.get('conditionLogic'),
            notificationChannels: notificationChannels,
            enabled: true
        };

        // Show loading state
        const originalText = saveBtn.textContent;
        saveBtn.disabled = true;
        saveBtn.textContent = 'Saving...';

        try {
            const response = await fetch('/api/alerts/rules', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(alertRule)
            });

            const result = await response.json();

            if (response.ok && result.success) {
                // Success feedback
                saveBtn.textContent = 'Saved!';
                saveBtn.classList.remove('bg-blue-600', 'hover:bg-blue-700');
                saveBtn.classList.add('bg-green-600', 'hover:bg-green-700');

                // Show success message
                const notificationMethods = notificationChannels.map(channel => {
                    switch (channel) {
                        case 'local': return 'dashboard alerts';
                        case 'email': return 'email notifications';
                        case 'webhook': return 'webhook notifications';
                        default: return channel;
                    }
                }).join(', ');

                PulseApp.ui.toast.success(
                    `Alert rule "${alertRule.name}" created successfully! Monitoring ${activeThresholds.length} threshold${activeThresholds.length > 1 ? 's' : ''} with ${notificationMethods}.`
                );

                // Collapse builder after short delay
                setTimeout(() => {
                    _collapseAlertBuilder();
                    
                    // Reset button state
                    saveBtn.disabled = false;
                    saveBtn.textContent = originalText;
                    saveBtn.classList.remove('bg-green-600', 'hover:bg-green-700');
                    saveBtn.classList.add('bg-blue-600', 'hover:bg-blue-700');
                }, 1500);

            } else {
                throw new Error(result.error || 'Failed to create alert rule');
            }
        } catch (error) {
            console.error('Error creating alert rule:', error);
            PulseApp.ui.toast.error(`Failed to create alert rule: ${error.message}`);
            
            // Reset button state
            saveBtn.disabled = false;
            saveBtn.textContent = originalText;
        }
    }


    async function _testAlert() {
        const alertNameInput = document.getElementById('integrated-alert-name');
        const alertDescriptionInput = document.getElementById('integrated-alert-description');
        const alertName = alertNameInput?.value || 'Test Alert';
        const alertDescription = alertDescriptionInput?.value || 'Brief description of what this alert monitors...';
        
        // Collect selected notification channels
        const form = document.getElementById('integrated-alert-form');
        const notificationChannels = [];
        const notificationInputs = form.querySelectorAll('input[name="notifications"]:checked');
        notificationInputs.forEach(input => {
            notificationChannels.push(input.value);
        });

        if (notificationChannels.length === 0) {
            PulseApp.ui.toast.warning('Please select at least one notification method to test.');
            return;
        }

        // Get current active thresholds for the test
        const thresholdState = PulseApp.state.getThresholdState();
        const activeThresholds = _getActiveThresholds(thresholdState);
        
        // Get target selection info
        const targetTypeInput = document.getElementById('target-type-input');
        const selectedVMsInput = document.getElementById('selected-vms-input');
        const targetType = targetTypeInput?.value || 'all';
        const selectedVMs = selectedVMsInput?.value || '[]';

        try {
            PulseApp.ui.toast.warning(`Testing alert "${alertName}"...`);
            
            const response = await fetch('/api/alerts/test', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    alertName: alertName,
                    alertDescription: alertDescription,
                    activeThresholds: activeThresholds,
                    notificationChannels: notificationChannels,
                    targetType: targetType,
                    selectedVMs: selectedVMs
                })
            });

            const result = await response.json();

            if (response.ok && result.success) {
                // Show detailed success message
                const successChannels = Object.entries(result.results)
                    .filter(([channel, res]) => res.success)
                    .map(([channel]) => {
                        switch (channel) {
                            case 'local': return 'dashboard alerts';
                            case 'email': return 'email';
                            case 'webhook': return 'webhook';
                            default: return channel;
                        }
                    });
                
                const failedChannels = Object.entries(result.results)
                    .filter(([channel, res]) => !res.success)
                    .map(([channel, res]) => `${channel}: ${res.error}`);

                if (failedChannels.length === 0) {
                    PulseApp.ui.toast.success(`Test alert sent successfully via ${successChannels.join(', ')}!`);
                } else {
                    PulseApp.ui.toast.warning(`Test partially successful. Working: ${successChannels.join(', ')}. Failed: ${failedChannels.join('; ')}`);
                }
            } else {
                throw new Error(result.error || result.message || 'Failed to send test alert');
            }
        } catch (error) {
            console.error('Error testing alert:', error);
            PulseApp.ui.toast.error(`Failed to test alert: ${error.message}`);
        }
    }

    return {
        init,
        resetThresholds,
        isThresholdDragInProgress,
        toggleRule,
        deleteRule,
        toggleNotificationCard
    };
})();
