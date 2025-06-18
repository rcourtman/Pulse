PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alertManagementModal = (() => {
    let isInitialized = false;
    let activeAlerts = [];
    let refreshInterval = null;

    function init() {
        if (isInitialized) return;
        
        // Create simplified modal HTML
        createModalHTML();
        
        // Set up event listeners
        setupEventListeners();
        
        isInitialized = true;
    }

    function createModalHTML() {
        const existingModal = document.getElementById('alert-management-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modalHTML = `
            <div id="alert-management-modal" class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 hidden">
                <div class="relative top-20 mx-auto p-5 border w-11/12 max-w-4xl shadow-lg rounded-md bg-white dark:bg-gray-800">
                    <!-- Header -->
                    <div class="flex items-center justify-between pb-4 border-b border-gray-200 dark:border-gray-700">
                        <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Alert Monitor</h3>
                        <button id="alert-management-modal-close" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                            </svg>
                        </button>
                    </div>

                    <!-- Content -->
                    <div class="py-4">
                        <!-- Alert Summary -->
                        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
                            <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
                                <div class="flex items-center justify-between">
                                    <div>
                                        <p class="text-sm font-medium text-red-600 dark:text-red-400">Critical Alerts</p>
                                        <p class="text-2xl font-bold text-red-900 dark:text-red-100" id="critical-alerts-count">0</p>
                                    </div>
                                    <svg class="w-8 h-8 text-red-500" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                                    </svg>
                                </div>
                            </div>
                            
                            <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
                                <div class="flex items-center justify-between">
                                    <div>
                                        <p class="text-sm font-medium text-yellow-600 dark:text-yellow-400">Warning Alerts</p>
                                        <p class="text-2xl font-bold text-yellow-900 dark:text-yellow-100" id="warning-alerts-count">0</p>
                                    </div>
                                    <svg class="w-8 h-8 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                                    </svg>
                                </div>
                            </div>
                            
                            <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                                <div class="flex items-center justify-between">
                                    <div>
                                        <p class="text-sm font-medium text-blue-600 dark:text-blue-400">Total Active</p>
                                        <p class="text-2xl font-bold text-blue-900 dark:text-blue-100" id="total-alerts-count">0</p>
                                    </div>
                                    <svg class="w-8 h-8 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                                        <path d="M10 2L3 7v11h14V7l-7-5z"/>
                                    </svg>
                                </div>
                            </div>
                        </div>

                        <!-- Alert List -->
                        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                            <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                                <h4 class="text-lg font-medium text-gray-900 dark:text-gray-100">Active Alerts</h4>
                                <div class="flex gap-2">
                                    <button id="refresh-alerts-btn" class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors">
                                        Refresh
                                    </button>
                                    <button id="acknowledge-all-btn" class="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white text-sm rounded transition-colors">
                                        Acknowledge All
                                    </button>
                                </div>
                            </div>
                            <div id="alerts-container" class="p-4 max-h-96 overflow-y-auto">
                                <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                                    Loading alerts...
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHTML);
    }

    function setupEventListeners() {
        const modal = document.getElementById('alert-management-modal');
        const closeButton = document.getElementById('alert-management-modal-close');
        const refreshButton = document.getElementById('refresh-alerts-btn');
        const acknowledgeAllButton = document.getElementById('acknowledge-all-btn');

        if (closeButton) {
            closeButton.addEventListener('click', closeModal);
        }

        if (refreshButton) {
            refreshButton.addEventListener('click', loadActiveAlerts);
        }

        if (acknowledgeAllButton) {
            acknowledgeAllButton.addEventListener('click', acknowledgeAllAlerts);
        }

        // Close on outside click
        if (modal) {
            modal.addEventListener('click', (e) => {
                if (e.target === modal) {
                    closeModal();
                }
            });
        }

        // Close on escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
                closeModal();
            }
        });
    }

    function openModal() {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.remove('hidden');
            loadActiveAlerts();
            
            // Start auto-refresh
            if (refreshInterval) {
                clearInterval(refreshInterval);
            }
            refreshInterval = setInterval(loadActiveAlerts, 10000); // Refresh every 10 seconds
        }
    }

    function closeModal() {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.add('hidden');
            
            // Stop auto-refresh
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        }
    }

    async function loadActiveAlerts() {
        try {
            const response = await fetch('/api/alerts/active');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const data = await response.json();
            if (data.success && Array.isArray(data.alerts)) {
                activeAlerts = data.alerts;
                renderAlerts();
                updateSummary();
            } else {
                throw new Error('Invalid response format');
            }
        } catch (error) {
            console.error('Failed to load active alerts:', error);
            const container = document.getElementById('alerts-container');
            if (container) {
                container.innerHTML = `
                    <div class="text-center py-8 text-red-500 dark:text-red-400">
                        Failed to load alerts: ${error.message}
                    </div>
                `;
            }
        }
    }

    function renderAlerts() {
        const container = document.getElementById('alerts-container');
        if (!container) return;

        if (activeAlerts.length === 0) {
            container.innerHTML = `
                <div class="text-center py-8 text-green-600 dark:text-green-400">
                    <svg class="w-12 h-12 mx-auto mb-2" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                    </svg>
                    <p class="text-lg font-medium">No active alerts</p>
                    <p class="text-sm">All systems are operating normally</p>
                </div>
            `;
            return;
        }

        const alertsHTML = activeAlerts.map(alert => {
            const severity = alert.severity || 'warning'; // Default to warning for status alerts
            const severityColors = {
                critical: 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20',
                warning: 'border-yellow-200 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900/20',
                info: 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20'
            };
            const severityTextColors = {
                critical: 'text-red-600 dark:text-red-400',
                warning: 'text-yellow-600 dark:text-yellow-400',
                info: 'text-blue-600 dark:text-blue-400'
            };

            const startTime = new Date(alert.triggeredAt || alert.startTime).toLocaleString();
            const duration = alert.triggeredAt ? formatDuration(Date.now() - alert.triggeredAt) : 
                           alert.startTime ? formatDuration(Date.now() - alert.startTime) : 'Unknown';
            const guestName = alert.guest?.name || 'Unknown VM';

            return `
                <div class="border ${severityColors[severity] || severityColors.info} rounded-lg p-4 mb-3">
                    <div class="flex items-start justify-between">
                        <div class="flex-1">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${severityTextColors[severity] || severityTextColors.info} bg-white dark:bg-gray-800 border">
                                    ${severity.toUpperCase()}
                                </span>
                                <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${guestName}</span>
                            </div>
                            <h4 class="font-semibold text-gray-900 dark:text-gray-100 mb-1">
                                ${alert.ruleName || alert.name || 'Unknown Alert'}
                            </h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mb-2">
                                ${alert.description || 'No description available'}
                            </p>
                            <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                                <div>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">Metric:</span>
                                    <span class="text-gray-600 dark:text-gray-400">${alert.metric || alert.metricType || 'Unknown'}</span>
                                </div>
                                <div>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">Current:</span>
                                    <span class="text-gray-600 dark:text-gray-400">${formatAlertValue(alert.currentValue, alert.metric || alert.metricType)}</span>
                                </div>
                                <div>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">Threshold:</span>
                                    <span class="text-gray-600 dark:text-gray-400">${formatAlertValue(alert.threshold, alert.metric || alert.metricType)}</span>
                                </div>
                                <div>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">Duration:</span>
                                    <span class="text-gray-600 dark:text-gray-400">${duration}</span>
                                </div>
                            </div>
                        </div>
                        <div class="ml-4">
                            ${!alert.acknowledged ? `
                                <button onclick="PulseApp.ui.alertManagementModal.acknowledgeAlert('${alert.id}')" 
                                        class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-xs rounded transition-colors">
                                    Acknowledge
                                </button>
                            ` : `
                                <span class="px-3 py-1 bg-gray-500 text-white text-xs rounded">
                                    Acknowledged
                                </span>
                            `}
                        </div>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = alertsHTML;
    }

    function updateSummary() {
        const criticalCount = activeAlerts.filter(a => a.severity === 'critical').length;
        const warningCount = activeAlerts.filter(a => a.severity === 'warning').length;
        const totalCount = activeAlerts.length;

        const criticalElement = document.getElementById('critical-alerts-count');
        const warningElement = document.getElementById('warning-alerts-count');
        const totalElement = document.getElementById('total-alerts-count');

        if (criticalElement) criticalElement.textContent = criticalCount;
        if (warningElement) warningElement.textContent = warningCount;
        if (totalElement) totalElement.textContent = totalCount;
    }

    async function acknowledgeAlert(alertId) {
        try {
            const response = await fetch(`/api/alerts/${alertId}/acknowledge`, {
                method: 'POST'
            });

            if (response.ok) {
                await loadActiveAlerts(); // Refresh the list
                if (PulseApp.ui.toast) {
                    PulseApp.ui.toast.success('Alert acknowledged');
                }
            } else {
                throw new Error('Failed to acknowledge alert');
            }
        } catch (error) {
            console.error('Failed to acknowledge alert:', error);
            if (PulseApp.ui.toast) {
                PulseApp.ui.toast.error('Failed to acknowledge alert');
            }
        }
    }

    async function acknowledgeAllAlerts() {
        const unacknowledgedAlerts = activeAlerts.filter(a => !a.acknowledged);
        
        if (unacknowledgedAlerts.length === 0) {
            if (PulseApp.ui.toast) {
                PulseApp.ui.toast.info('No alerts to acknowledge');
            }
            return;
        }

        if (!confirm(`Acknowledge ${unacknowledgedAlerts.length} alert(s)?`)) {
            return;
        }

        try {
            const promises = unacknowledgedAlerts.map(alert => 
                fetch(`/api/alerts/${alert.id}/acknowledge`, { method: 'POST' })
            );

            await Promise.all(promises);
            await loadActiveAlerts(); // Refresh the list
            
            if (PulseApp.ui.toast) {
                PulseApp.ui.toast.success(`Acknowledged ${unacknowledgedAlerts.length} alert(s)`);
            }
        } catch (error) {
            console.error('Failed to acknowledge all alerts:', error);
            if (PulseApp.ui.toast) {
                PulseApp.ui.toast.error('Failed to acknowledge all alerts');
            }
        }
    }

    function formatDuration(milliseconds) {
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        
        if (days > 0) return `${days}d ${hours % 24}h`;
        if (hours > 0) return `${hours}h ${minutes % 60}m`;
        if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
        return `${seconds}s`;
    }
    
    function formatAlertValue(value, metricType) {
        if (value === null || value === undefined) return 'N/A';
        
        switch (metricType) {
            case 'cpu':
            case 'memory':
            case 'disk':
                return `${Math.round(value)}%`;
            case 'diskread':
            case 'diskwrite':
            case 'netin':
            case 'netout':
                return PulseApp.utils?.formatBytes ? PulseApp.utils.formatBytes(value) + '/s' : `${value} B/s`;
            default:
                return String(value);
        }
    }

    return {
        init,
        openModal,
        closeModal,
        acknowledgeAlert
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.alertManagementModal.init);
} else {
    PulseApp.ui.alertManagementModal.init();
}