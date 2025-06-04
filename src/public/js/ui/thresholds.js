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

    function init() {
        thresholdRow = document.getElementById('threshold-slider-row');
        alertRuleRow = document.getElementById('alert-rule-row');
        toggleThresholdsButton = document.getElementById('toggle-thresholds-checkbox');
        thresholdBadge = document.getElementById('threshold-count-badge');
        createAlertRuleBtn = document.getElementById('create-alert-rule-btn');
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
        if (createAlertRuleBtn) {
            createAlertRuleBtn.addEventListener('click', _handleCreateAlertRule);
        }

        if (viewAlertRulesBtn) {
            viewAlertRulesBtn.addEventListener('click', _handleViewAlertRules);
        }
    }

    function _handleCreateAlertRule() {
        const thresholdState = PulseApp.state.getThresholdState();
        const activeThresholds = _getActiveThresholds(thresholdState);
        
        if (activeThresholds.length === 0) {
            alert('Please set at least one threshold to create an alert rule.');
            return;
        }

        // Show modal for creating alert rule
        _showCreateAlertRuleModal(activeThresholds);
    }

    function _handleViewAlertRules() {
        // Show modal for viewing/managing existing alert rules
        _showViewAlertRulesModal();
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
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                        Alert Severity
                                    </label>
                                    <select id="rule-severity" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
                                        <option value="warning">Warning</option>
                                        <option value="critical">Critical</option>
                                    </select>
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
        const ruleSeverity = document.getElementById('rule-severity').value;

        const alertRule = {
            name: ruleName,
            description: ruleDescription,
            severity: ruleSeverity,
            thresholds: activeThresholds,
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
                alert(`✅ Alert rule "${ruleName}" created successfully!\n\nIt will monitor for guests meeting ALL of these conditions:\n${activeThresholds.map(t => `• ${_getThresholdDisplayName(t.type)} ≥ ${_formatThresholdValue(t)}`).join('\n')}\n\nYou'll receive email notifications when any guest meets these criteria.`);
            } else {
                throw new Error(result.error || 'Failed to create alert rule');
            }
        } catch (error) {
            console.error('Error creating alert rule:', error);
            alert(`❌ Failed to create alert rule: ${error.message}`);
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
            alert(`❌ Failed to fetch alert rules: ${error.message}`);
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
        const severityColor = rule.severity === 'critical' ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';

        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-4">
                <div class="flex items-start justify-between mb-3">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-2">
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">${rule.name}</h4>
                            <span class="px-2 py-1 text-xs font-medium rounded ${severityColor}">
                                ${rule.severity.toUpperCase()}
                            </span>
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
            alert(`❌ Failed to update rule: ${error.message}`);
        }
    }

    async function deleteRule(ruleId) {
        if (!confirm('Are you sure you want to delete this alert rule? This action cannot be undone.')) {
            return;
        }

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
            alert(`❌ Failed to delete rule: ${error.message}`);
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
        // Update button state
        if (createAlertRuleBtn) {
            createAlertRuleBtn.disabled = activeCount === 0;
            if (activeCount === 0) {
                createAlertRuleBtn.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                createAlertRuleBtn.classList.remove('opacity-50', 'cursor-not-allowed');
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
        const mb = bytes / (1024 * 1024);
        if (mb >= 100) return `${Math.round(mb)}MB/s`;
        if (mb >= 10) return `${Math.round(mb)}MB/s`;
        return `${Math.round(mb * 10) / 10}MB/s`;
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
        isThresholdDragInProgress,
        toggleRule,
        deleteRule
    };
})();
