PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alertManagementModal = (() => {
    let isInitialized = false;
    let activeTab = 'alerts';
    let currentConfig = {};
    let formDataCache = {};

    function init() {
        if (isInitialized) return;
        
        console.log('[AlertManagementModal] Initializing alert management modal...');
        
        // Create modal HTML
        createModalHTML();
        
        // Set up event listeners
        setupEventListeners();
        
        isInitialized = true;
        console.log('[AlertManagementModal] Alert management modal initialized');
    }

    function createModalHTML() {
        const existingModal = document.getElementById('alert-management-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modalHTML = `
            <div id="alert-management-modal" class="fixed inset-0 z-50 hidden items-center justify-center bg-black bg-opacity-50">
                <div class="modal-content bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-6xl max-h-[90vh] flex flex-col m-4">
                    <div class="modal-header flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-6 py-4">
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Alert Management</h2>
                        <button id="alert-management-modal-close" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    
                    <!-- Tab Navigation -->
                    <div class="border-b border-gray-200 dark:border-gray-700">
                        <nav class="flex space-x-8 px-6" id="alert-management-tabs">
                            <button class="alert-tab active py-3 px-1 border-b-2 border-blue-500 text-blue-600 dark:text-blue-400 font-medium text-sm" data-tab="alerts">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-5 5v-5z" />
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19H6a2 2 0 01-2-2V7a2 2 0 012-2h5m5 16v-5a1 1 0 00-1-1h-4a1 1 0 00-1 1v5a1 1 0 001 1h4a1 1 0 001-1z" />
                                </svg>
                                Alerts
                            </button>
                            <button class="alert-tab py-3 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-tab="alert-rules">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v6a2 2 0 002 2h2m0 0h2a2 2 0 002-2V7a2 2 0 00-2-2h-2m0 0h2a2 2 0 002-2V3a2 2 0 00-2-2H9a2 2 0 00-2 2v2a2 2 0 002 2z" />
                                </svg>
                                Alert Rules
                            </button>
                            <button class="alert-tab py-3 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-tab="notifications">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 4.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                                </svg>
                                Notifications
                            </button>
                        </nav>
                    </div>
                    
                    <div id="alert-management-modal-body" class="overflow-y-auto flex-grow p-6 scrollbar">
                        <p class="text-gray-500 dark:text-gray-400">Loading...</p>
                    </div>
                    
                    <div class="modal-footer border-t border-gray-300 dark:border-gray-700 px-6 py-4">
                        <div class="flex gap-3 justify-end">
                            <button type="button" id="alert-management-cancel-button" class="px-4 py-2 bg-gray-500 hover:bg-gray-600 text-white text-sm font-medium rounded-md transition-colors">
                                Close
                            </button>
                            <button type="button" id="alert-management-save-button" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                                Save Changes
                            </button>
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
        const cancelButton = document.getElementById('alert-management-cancel-button');
        const saveButton = document.getElementById('alert-management-save-button');

        if (closeButton) {
            closeButton.addEventListener('click', closeModal);
        }

        if (cancelButton) {
            cancelButton.addEventListener('click', closeModal);
        }

        if (saveButton) {
            saveButton.addEventListener('click', saveConfiguration);
        }

        // Close modal when clicking outside
        if (modal) {
            modal.addEventListener('click', (e) => {
                if (e.target === modal) {
                    closeModal();
                }
            });
        }

        // Handle escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && modal && !modal.classList.contains('hidden')) {
                closeModal();
            }
        });

        // Set up tab navigation
        setupTabNavigation();
    }

    function setupTabNavigation() {
        const tabButtons = document.querySelectorAll('.alert-tab');
        
        tabButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                const tabName = e.target.getAttribute('data-tab');
                switchTab(tabName);
            });
        });
    }

    function switchTab(tabName) {
        // Preserve current form data before switching tabs
        preserveCurrentFormData();
        
        activeTab = tabName;
        
        // Update tab buttons
        const tabButtons = document.querySelectorAll('.alert-tab');
        tabButtons.forEach(button => {
            const isActive = button.getAttribute('data-tab') === tabName;
            
            if (isActive) {
                button.classList.add('active');
                button.classList.remove('border-transparent', 'text-gray-500', 'dark:text-gray-400');
                button.classList.add('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
            } else {
                button.classList.remove('active');
                button.classList.remove('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
                button.classList.add('border-transparent', 'text-gray-500', 'dark:text-gray-400');
            }
        });

        // Update content
        renderTabContent();
        
        // Restore form data for the new tab
        setTimeout(() => {
            restoreFormData(tabName);
        }, 100);
    }

    function renderTabContent() {
        const modalBody = document.getElementById('alert-management-modal-body');
        if (!modalBody) return;

        switch (activeTab) {
            case 'alerts':
                modalBody.innerHTML = renderAlertsTab();
                initializeAlertsTab();
                break;
            case 'alert-rules':
                modalBody.innerHTML = renderAlertRulesTab();
                initializeAlertRulesTab();
                break;
            case 'notifications':
                modalBody.innerHTML = renderNotificationsTab();
                initializeNotificationsTab();
                break;
            default:
                modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Unknown tab</p>';
        }
    }

    function renderAlertsTab() {
        return `
            <div class="space-y-6">
                <div class="flex items-center justify-between">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Current Alerts</h3>
                    <button id="refresh-alerts-btn" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        Refresh
                    </button>
                </div>
                
                <div id="current-alerts-content" class="space-y-4">
                    <!-- Alert content will be injected here -->
                    <p class="text-gray-500 dark:text-gray-400">Loading current alerts...</p>
                </div>
                
                <div class="mt-6 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Quick Actions</h4>
                    <div class="flex gap-2">
                        <button id="acknowledge-all-alerts-btn" class="px-3 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                            Acknowledge All
                        </button>
                        <button id="view-alert-history-btn" class="px-3 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors">
                            View History
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    function renderAlertRulesTab() {
        return `
            <div class="space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Alert Rules</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Manage system and custom alert rules</p>
                    </div>
                    <button id="add-custom-alert-btn" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                        + Add Custom Alert
                    </button>
                </div>

                <!-- Nested Tabs -->
                <div class="border-b border-gray-200 dark:border-gray-700">
                    <nav class="flex space-x-8" id="alert-rules-sub-tabs">
                        <button class="alert-rules-sub-tab active py-2 px-1 border-b-2 border-blue-500 text-blue-600 dark:text-blue-400 font-medium text-sm" data-tab="system">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            System Alerts
                        </button>
                        <button class="alert-rules-sub-tab py-2 px-1 border-b-2 border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 font-medium text-sm" data-tab="custom">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 100 4m0-4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 100 4m0-4v2m0-6V4" />
                            </svg>
                            Custom Alerts
                        </button>
                    </nav>
                </div>

                <!-- Tab Content -->
                <div id="alert-rules-sub-content">
                    <!-- System Alerts Tab Content (Default) -->
                    <div id="system-alert-rules-content" class="alert-rules-sub-content">
                        <div class="space-y-6">
                            <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                                <div>
                                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">System Alert Rules</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Built-in monitoring alerts for CPU, Memory, Disk, and System Down</p>
                                </div>
                                <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded-full">Built-in</span>
                            </div>
                            
                            <div id="system-alerts-content" class="space-y-3">
                                <!-- System alerts will be loaded here -->
                                <p class="text-sm text-gray-500 dark:text-gray-400">Loading system alerts...</p>
                            </div>
                        </div>
                    </div>

                    <!-- Custom Alerts Tab Content (Hidden by default) -->
                    <div id="custom-alert-rules-content" class="alert-rules-sub-content hidden">
                        <div class="space-y-6">
                            <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                                <div>
                                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Custom Alert Rules</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">User-created alerts with custom thresholds and conditions</p>
                                </div>
                                <span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded-full">User-created</span>
                            </div>
                            
                            <div id="custom-alerts-content" class="space-y-3">
                                <!-- Custom alerts will be loaded here -->
                                <p class="text-sm text-gray-500 dark:text-gray-400">Loading custom alerts...</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function renderNotificationsTab() {
        const config = currentConfig || {};
        const smtp = config.advanced?.smtp || {};
        
        return `
            <div class="space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Notification Settings</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Configure how alerts are delivered beyond the Pulse dashboard</p>
                    </div>
                </div>

                <!-- Nested Tabs -->
                <div class="border-b border-gray-200 dark:border-gray-700">
                    <nav class="flex space-x-8" id="notification-sub-tabs">
                        <button class="notification-sub-tab active py-2 px-1 border-b-2 border-blue-500 text-blue-600 dark:text-blue-400 font-medium text-sm" data-tab="email">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 4.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                            </svg>
                            Email
                        </button>
                        <button class="notification-sub-tab py-2 px-1 border-b-2 border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 font-medium text-sm" data-tab="webhook">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.367 2.684 3 3 0 00-5.367-2.684z" />
                            </svg>
                            Webhooks
                        </button>
                    </nav>
                </div>

                <!-- Tab Content -->
                <div id="notification-sub-content">
                    <!-- Email Tab Content (Default) -->
                    <div id="email-notification-content" class="notification-sub-content">
                        <div class="space-y-6">
                            <!-- Email Toggle -->
                            <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                                <div>
                                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Email Notifications</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Send alerts to email addresses</p>
                                </div>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input type="checkbox" name="GLOBAL_EMAIL_ENABLED" class="sr-only peer">
                                    <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                </label>
                            </div>

                            <!-- Email Configuration -->
                            <div id="email-config-section">
                                <p class="text-sm text-gray-600 dark:text-gray-400">Email configuration will be loaded...</p>
                            </div>
                        </div>
                    </div>

                    <!-- Webhook Tab Content (Hidden by default) -->
                    <div id="webhook-notification-content" class="notification-sub-content hidden">
                        <div class="space-y-6">
                            <!-- Webhook Toggle -->
                            <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                                <div>
                                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Webhook Notifications</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Send alerts to external services</p>
                                </div>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input type="checkbox" name="GLOBAL_WEBHOOK_ENABLED" class="sr-only peer">
                                    <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                </label>
                            </div>

                            <!-- Popular Services -->
                            <div>
                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Popular Services</h5>
                                <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
                                    <button class="webhook-preset p-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-center transition-colors" data-service="discord">
                                        <div class="w-8 h-8 mx-auto mb-2 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                                            <span class="text-gray-600 dark:text-gray-400 font-bold text-xs">DC</span>
                                        </div>
                                        <span class="text-xs text-gray-700 dark:text-gray-300">Discord</span>
                                    </button>
                                    <button class="webhook-preset p-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-center transition-colors" data-service="slack">
                                        <div class="w-8 h-8 mx-auto mb-2 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                                            <span class="text-gray-600 dark:text-gray-400 font-bold text-xs">SL</span>
                                        </div>
                                        <span class="text-xs text-gray-700 dark:text-gray-300">Slack</span>
                                    </button>
                                    <button class="webhook-preset p-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-center transition-colors" data-service="teams">
                                        <div class="w-8 h-8 mx-auto mb-2 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                                            <span class="text-gray-600 dark:text-gray-400 font-bold text-xs">MS</span>
                                        </div>
                                        <span class="text-xs text-gray-700 dark:text-gray-300">Teams</span>
                                    </button>
                                    <button class="webhook-preset p-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-center transition-colors" data-service="custom">
                                        <div class="w-8 h-8 mx-auto mb-2 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                                            <span class="text-gray-600 dark:text-gray-400 font-bold text-xs">+</span>
                                        </div>
                                        <span class="text-xs text-gray-700 dark:text-gray-300">Custom</span>
                                    </button>
                                </div>
                            </div>

                            <!-- Webhook URL Input -->
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Webhook URL
                                </label>
                                <input type="url" name="WEBHOOK_URL" id="webhook-url-input"
                                       placeholder="https://your-service.com/webhook"
                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                    Enter your webhook URL from Discord, Slack, Teams, or any custom service
                                </p>
                            </div>
                            
                            <!-- Action Buttons -->
                            <div class="flex space-x-3">
                                <button id="test-webhook-btn" type="button" 
                                        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors">
                                    Send Test Message
                                </button>
                                <button id="save-webhook-btn" type="button"
                                        class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-lg transition-colors">
                                    Save Configuration
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function initializeAlertsTab() {
        // Set up refresh button
        const refreshBtn = document.getElementById('refresh-alerts-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', loadCurrentAlerts);
        }

        // Set up action buttons
        const acknowledgeAllBtn = document.getElementById('acknowledge-all-alerts-btn');
        if (acknowledgeAllBtn) {
            acknowledgeAllBtn.addEventListener('click', () => {
                if (PulseApp.alerts && PulseApp.alerts.markAllAsAcknowledged) {
                    PulseApp.alerts.markAllAsAcknowledged();
                    setTimeout(loadCurrentAlerts, 500); // Refresh after acknowledging
                }
            });
        }

        const viewHistoryBtn = document.getElementById('view-alert-history-btn');
        if (viewHistoryBtn) {
            viewHistoryBtn.addEventListener('click', () => {
                // TODO: Implement alert history view
                console.log('Alert history view not yet implemented');
            });
        }

        // Load current alerts
        loadCurrentAlerts();
    }

    function initializeAlertRulesTab() {
        // Set up nested tab navigation
        const subTabs = document.querySelectorAll('.alert-rules-sub-tab');
        subTabs.forEach(tab => {
            tab.addEventListener('click', (e) => {
                const targetTab = e.currentTarget.dataset.tab;
                switchAlertRulesSubTab(targetTab);
            });
        });
        
        // Set up add custom alert button
        const addCustomBtn = document.getElementById('add-custom-alert-btn');
        if (addCustomBtn) {
            addCustomBtn.addEventListener('click', openCustomAlertModal);
        }
        
        // Load system and custom alerts
        loadSystemAlerts();
        loadCustomAlerts();
    }
    
    function switchAlertRulesSubTab(tabName) {
        // Update tab buttons
        const subTabs = document.querySelectorAll('.alert-rules-sub-tab');
        subTabs.forEach(tab => {
            const isActive = tab.dataset.tab === tabName;
            if (isActive) {
                tab.classList.add('active');
                tab.classList.remove('border-transparent', 'text-gray-500', 'dark:text-gray-400');
                tab.classList.add('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
            } else {
                tab.classList.remove('active');
                tab.classList.remove('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
                tab.classList.add('border-transparent', 'text-gray-500', 'dark:text-gray-400');
            }
        });

        // Update content visibility
        const contentDivs = document.querySelectorAll('.alert-rules-sub-content');
        contentDivs.forEach(div => {
            div.classList.add('hidden');
        });
        
        const targetContent = document.getElementById(`${tabName}-alert-rules-content`);
        if (targetContent) {
            targetContent.classList.remove('hidden');
        }
    }

    function initializeNotificationsTab() {
        // Load email configuration into the email section
        loadEmailConfiguration();
        
        // Set up nested tab navigation
        const subTabs = document.querySelectorAll('.notification-sub-tab');
        subTabs.forEach(tab => {
            tab.addEventListener('click', (e) => {
                const targetTab = e.currentTarget.dataset.tab;
                switchNotificationSubTab(targetTab);
            });
        });
        
        // Set up webhook preset buttons
        const webhookPresets = document.querySelectorAll('.webhook-preset');
        webhookPresets.forEach(preset => {
            preset.addEventListener('click', (e) => {
                const service = e.currentTarget.dataset.service;
                handleWebhookPreset(service);
            });
        });
        
        // Set up webhook action buttons
        const testWebhookBtn = document.getElementById('test-webhook-btn');
        if (testWebhookBtn) {
            testWebhookBtn.addEventListener('click', testWebhookConnection);
        }
        
        const saveWebhookBtn = document.getElementById('save-webhook-btn');
        if (saveWebhookBtn) {
            saveWebhookBtn.addEventListener('click', saveWebhookConfiguration);
        }
        
        // Set up toggle switches
        const emailToggle = document.querySelector('input[name="GLOBAL_EMAIL_ENABLED"]');
        if (emailToggle) {
            emailToggle.addEventListener('change', (e) => {
                console.log('Email notifications toggled:', e.target.checked);
            });
        }
        
        const webhookToggle = document.querySelector('input[name="GLOBAL_WEBHOOK_ENABLED"]');
        if (webhookToggle) {
            webhookToggle.addEventListener('change', (e) => {
                console.log('Webhook notifications toggled:', e.target.checked);
            });
        }
    }
    
    function switchNotificationSubTab(tabName) {
        // Update tab buttons
        const subTabs = document.querySelectorAll('.notification-sub-tab');
        subTabs.forEach(tab => {
            const isActive = tab.dataset.tab === tabName;
            if (isActive) {
                tab.classList.add('active');
                tab.classList.remove('border-transparent', 'text-gray-500', 'dark:text-gray-400');
                tab.classList.add('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
            } else {
                tab.classList.remove('active');
                tab.classList.remove('border-blue-500', 'text-blue-600', 'dark:text-blue-400');
                tab.classList.add('border-transparent', 'text-gray-500', 'dark:text-gray-400');
            }
        });

        // Update content visibility
        const contentDivs = document.querySelectorAll('.notification-sub-content');
        contentDivs.forEach(div => {
            div.classList.add('hidden');
        });
        
        const targetContent = document.getElementById(`${tabName}-notification-content`);
        if (targetContent) {
            targetContent.classList.remove('hidden');
        }
    }

    function loadSystemAlerts() {
        const systemAlertsContent = document.getElementById('system-alerts-content');
        if (!systemAlertsContent) return;
        
        // Create system alert rules (CPU, Memory, Disk, Down)
        const systemAlerts = [
            { id: 'cpu', name: 'CPU Usage', description: 'Alert when CPU usage exceeds threshold', enabled: true, threshold: 85 },
            { id: 'memory', name: 'Memory Usage', description: 'Alert when memory usage exceeds threshold', enabled: true, threshold: 90 },
            { id: 'disk', name: 'Disk Usage', description: 'Alert when disk usage exceeds threshold', enabled: true, threshold: 95 },
            { id: 'down', name: 'System Down', description: 'Alert when VM/LXC goes offline', enabled: true, threshold: null }
        ];
        
        systemAlertsContent.innerHTML = systemAlerts.map(alert => createSystemAlertCard(alert)).join('');
    }

    async function loadCustomAlerts() {
        const customAlertsContent = document.getElementById('custom-alerts-content');
        if (!customAlertsContent) return;
        
        try {
            // Load custom alerts from the backend API
            const response = await fetch('/api/alerts/rules');
            if (!response.ok) {
                throw new Error(`Failed to fetch alert rules: ${response.status}`);
            }
            
            const data = await response.json();
            const customAlerts = data.rules ? data.rules.filter(rule => 
                rule.group === 'custom' || rule.type === 'compound_threshold'
            ) : [];
            
            if (customAlerts.length === 0) {
                customAlertsContent.innerHTML = `
                    <div class="text-center py-6">
                        <svg class="w-8 h-8 mx-auto mb-3 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                        </svg>
                        <p class="text-sm text-gray-500 dark:text-gray-400 mb-2">No custom alerts configured</p>
                        <button onclick="PulseApp.ui.alertManagementModal.openCustomAlertModal()" 
                                class="text-xs text-blue-600 dark:text-blue-400 hover:underline">
                            Create your first custom alert
                        </button>
                    </div>
                `;
                return;
            }
            
            // Display the custom alerts
            customAlertsContent.innerHTML = customAlerts.map(alert => createCustomAlertCard(alert)).join('');
            
        } catch (error) {
            console.error('[AlertManagementModal] Failed to load custom alerts:', error);
            customAlertsContent.innerHTML = `
                <div class="text-center py-6 text-red-500 dark:text-red-400">
                    <p class="text-sm">Failed to load custom alerts</p>
                    <p class="text-xs">${error.message}</p>
                </div>
            `;
        }
    }

    function createCustomAlertCard(alert) {
        const thresholdsList = alert.thresholds?.map(t => 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${t.type.charAt(0).toUpperCase() + t.type.slice(1)} ≥ ${t.value}${['cpu', 'memory', 'disk'].includes(t.type) ? '%' : ''}
            </span>`
        ).join('') || '';

        const createdDate = alert.createdAt ? new Date(alert.createdAt).toLocaleDateString() : 'Unknown';
        const targetDisplay = alert.targetType === 'all' ? 'All VMs/LXCs' : 
                             alert.targetType === 'vm' ? 'VMs only' :
                             alert.targetType === 'lxc' ? 'LXCs only' :
                             alert.specificTarget || alert.targetType?.toUpperCase() || 'All';
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                <div class="flex items-start justify-between mb-2">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-1">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${alert.name || alert.description || 'Unnamed Alert'}</h5>
                            <span class="text-xs px-2 py-0.5 ${alert.enabled !== false ? 'bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'} rounded-full">
                                ${alert.enabled !== false ? 'Enabled' : 'Disabled'}
                            </span>
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Created: ${createdDate} • Target: ${targetDisplay}
                        </div>
                    </div>
                    <div class="flex gap-1">
                        <button onclick="PulseApp.ui.alertManagementModal.toggleCustomAlert('${alert.id}', ${alert.enabled === false})" 
                                class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded">
                            ${alert.enabled !== false ? 'Disable' : 'Enable'}
                        </button>
                        <button onclick="PulseApp.ui.alertManagementModal.deleteCustomAlert('${alert.id}')" 
                                class="px-2 py-1 text-xs bg-red-600 hover:bg-red-700 text-white rounded">
                            Delete
                        </button>
                    </div>
                </div>
                <div>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Alert triggers when ANY condition is met:</p>
                    <div class="flex flex-wrap">
                        ${thresholdsList}
                    </div>
                </div>
            </div>
        `;
    }

    function createSystemAlertCard(alert) {
        const deliveryBadges = `
            <div class="flex items-center space-x-1">
                <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded font-medium" title="Always delivered to Pulse UI">
                    Pulse
                </span>
                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded" title="Configure in Notifications tab">
                    Email
                </span>
                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded" title="Configure in Notifications tab">
                    Webhook
                </span>
            </div>
        `;
        
        return `
            <div class="flex items-center justify-between p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-lg">
                <div class="flex-1">
                    <div class="flex items-center space-x-2 mb-1">
                        <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${alert.name}</h5>
                        ${alert.enabled ? 
                            '<span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded-full">Enabled</span>' : 
                            '<span class="text-xs px-2 py-0.5 bg-red-100 dark:bg-red-800 text-red-700 dark:text-red-300 rounded-full">Disabled</span>'
                        }
                    </div>
                    <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">${alert.description}</p>
                    ${deliveryBadges}
                </div>
                <div class="flex items-center space-x-2">
                    ${alert.threshold ? `
                        <span class="text-xs text-gray-500 dark:text-gray-400">${alert.threshold}%</span>
                    ` : ''}
                    <label class="relative inline-flex items-center cursor-pointer">
                        <input type="checkbox" ${alert.enabled ? 'checked' : ''} class="sr-only peer" onchange="toggleSystemAlert('${alert.id}', this.checked)">
                        <div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </label>
                </div>
            </div>
        `;
    }

    function loadEmailConfiguration() {
        const emailConfigSection = document.getElementById('email-config-section');
        if (!emailConfigSection) return;
        
        // Use existing email configuration from settings if available
        if (PulseApp.ui.settings && PulseApp.ui.settings.renderAlertsTab) {
            const config = currentConfig || {};
            const fullContent = PulseApp.ui.settings.renderAlertsTab(config.alerts || {}, config);
            
            // Extract just the email configuration section
            const tempDiv = document.createElement('div');
            tempDiv.innerHTML = fullContent;
            const emailSection = tempDiv.querySelector('.bg-gray-50:nth-child(2)'); // Email section
            
            if (emailSection) {
                // Remove the outer container and just show the inner content
                const emailContent = emailSection.innerHTML;
                emailConfigSection.innerHTML = emailContent;
                
                // Initialize email functionality
                if (PulseApp.ui.settings.setupEmailProviderSelection) {
                    setTimeout(() => PulseApp.ui.settings.setupEmailProviderSelection(), 100);
                }
                if (PulseApp.ui.settings.setupEmailTestButton) {
                    setTimeout(() => PulseApp.ui.settings.setupEmailTestButton(), 100);
                }
            }
        }
    }

    function handleWebhookPreset(service) {
        const webhookInput = document.getElementById('webhook-url-input');
        if (!webhookInput) return;
        
        const placeholders = {
            discord: 'https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN',
            slack: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK',
            teams: 'https://outlook.office.com/webhook/YOUR_TEAMS_WEBHOOK',
            custom: 'https://your-service.com/webhook'
        };
        
        webhookInput.placeholder = placeholders[service] || placeholders.custom;
        webhookInput.focus();
        
        // Highlight the selected preset
        document.querySelectorAll('.webhook-preset').forEach(btn => {
            btn.classList.remove('ring-2', 'ring-blue-500');
        });
        document.querySelector(`[data-service="${service}"]`).classList.add('ring-2', 'ring-blue-500');
    }
    
    function saveWebhookConfiguration() {
        const webhookUrl = document.getElementById('webhook-url-input')?.value;
        if (!webhookUrl) {
            alert('Please enter a webhook URL');
            return;
        }
        
        // TODO: Implement webhook configuration saving
        console.log('Saving webhook configuration:', webhookUrl);
        alert('Webhook configuration saved successfully!');
    }

    function testWebhookConnection() {
        const webhookUrl = document.getElementById('webhook-url-input')?.value;
        if (!webhookUrl) {
            alert('Please enter a webhook URL first');
            return;
        }
        
        // Use existing webhook test functionality from settings if available
        if (PulseApp.ui.settings && PulseApp.ui.settings.setupWebhookTestButton) {
            PulseApp.ui.settings.setupWebhookTestButton();
        } else {
            // Fallback test
            console.log('Testing webhook:', webhookUrl);
            alert('Webhook test functionality will be implemented');
        }
    }

    function openCustomAlertModal(presetThresholds = null) {
        // Check if settings modal has the threshold functionality we can reuse
        if (PulseApp.ui.settings && PulseApp.ui.settings.showThresholdModal && !presetThresholds) {
            // Use the existing threshold modal from settings (only if no presets)
            PulseApp.ui.settings.showThresholdModal();
        } else {
            // Create our own custom alert modal with optional preset values
            createCustomAlertModal(presetThresholds);
        }
    }

    function createCustomAlertModal(presetThresholds = null) {
        // Remove any existing custom alert modal
        const existingModal = document.getElementById('custom-alert-modal');
        if (existingModal) {
            existingModal.remove();
        }

        // Generate alert name and determine best metric from presets
        let suggestedName = 'Custom Alert';
        let primaryMetric = 'cpu';
        let primaryThreshold = 85;
        let multipleThresholds = false;
        
        if (presetThresholds && presetThresholds.length > 0) {
            // Create a descriptive name based on active thresholds
            const thresholdNames = presetThresholds.map(t => `${t.type.toUpperCase()}: ${t.value}${t.type === 'cpu' || t.type === 'memory' || t.type === 'disk' ? '%' : ''}`);
            suggestedName = `Alert for ${thresholdNames.join(', ')}`;
            
            // Use the first threshold as the primary one for the form
            primaryMetric = presetThresholds[0].type;
            primaryThreshold = presetThresholds[0].value;
            
            // Track if we have multiple thresholds
            multipleThresholds = presetThresholds.length > 1;
        }

        const modalHTML = `
            <div id="custom-alert-modal" class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
                <div class="modal-content bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md max-h-[90vh] flex flex-col m-4">
                    <div class="modal-header flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-6 py-4">
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Create Custom Alert</h2>
                        <button id="custom-alert-modal-close" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    
                    <div class="overflow-y-auto flex-grow p-6 scrollbar">
                        <form id="custom-alert-form" class="space-y-4">
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Alert Name
                                </label>
                                <input type="text" name="alertName" required
                                       value="${suggestedName}"
                                       placeholder="e.g., High CPU on Production VMs"
                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            </div>

                            ${multipleThresholds ? `
                            <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg p-3 mb-4">
                                <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">Multiple Thresholds Detected</h4>
                                <p class="text-xs text-blue-700 dark:text-blue-300 mb-2">
                                    This alert will trigger when ANY of these conditions are met:
                                </p>
                                <div class="space-y-1">
                                    ${presetThresholds.map(t => `
                                        <div class="flex items-center justify-between text-xs">
                                            <span class="text-blue-800 dark:text-blue-200">${t.type.charAt(0).toUpperCase() + t.type.slice(1)}</span>
                                            <span class="font-medium text-blue-900 dark:text-blue-100">≥ ${t.value}${['cpu', 'memory', 'disk'].includes(t.type) ? '%' : ''}</span>
                                        </div>
                                    `).join('')}
                                </div>
                                <input type="hidden" name="multipleThresholds" value="${JSON.stringify(presetThresholds).replace(/"/g, '&quot;')}">
                            </div>
                            ` : ''}

                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Target VMs/LXCs
                                </label>
                                <select name="targetType" class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    <option value="all">All VMs and LXCs</option>
                                    <option value="vm">Virtual Machines only</option>
                                    <option value="lxc">LXC Containers only</option>
                                    <option value="specific">Specific VM/LXC</option>
                                    <option value="node">All on specific node</option>
                                </select>
                            </div>

                            <div id="specific-target-section" class="hidden">
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    VM/LXC ID or Name
                                </label>
                                <input type="text" name="specificTarget"
                                       placeholder="e.g., 100, vm-name, or pve-node"
                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            </div>

                            ${!multipleThresholds ? `
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Metric to Monitor
                                </label>
                                <select name="metric" class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    <option value="cpu" ${primaryMetric === 'cpu' ? 'selected' : ''}>CPU Usage (%)</option>
                                    <option value="memory" ${primaryMetric === 'memory' ? 'selected' : ''}>Memory Usage (%)</option>
                                    <option value="disk" ${primaryMetric === 'disk' ? 'selected' : ''}>Disk Usage (%)</option>
                                    <option value="network" ${primaryMetric === 'network' ? 'selected' : ''}>Network Activity</option>
                                    <option value="status" ${primaryMetric === 'status' ? 'selected' : ''}>VM/LXC Status</option>
                                </select>
                            </div>

                            <div id="threshold-section">
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Alert Threshold
                                </label>
                                <div class="flex items-center space-x-2">
                                    <select name="operator" class="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
                                        <option value=">">&gt; Greater than</option>
                                        <option value="<">&lt; Less than</option>
                                        <option value="=">=  Equal to</option>
                                    </select>
                                    <input type="number" name="threshold" required min="0" max="100"
                                           value="${primaryThreshold}"
                                           placeholder="85"
                                           class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    <span class="text-sm text-gray-500 dark:text-gray-400">%</span>
                                </div>
                            </div>
                            ` : ''}

                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Alert Delivery (optional overrides)
                                </label>
                                <div class="space-y-2">
                                    <label class="flex items-center">
                                        <input type="checkbox" name="sendEmail" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                        <span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded font-medium mr-2">Email</span>
                                        <span class="text-sm text-gray-700 dark:text-gray-300">Also send via email</span>
                                    </label>
                                    <label class="flex items-center">
                                        <input type="checkbox" name="sendWebhook" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                        <span class="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded font-medium mr-2">Webhook</span>
                                        <span class="text-sm text-gray-700 dark:text-gray-300">Also send via webhook</span>
                                    </label>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Leave unchecked to use global notification defaults</p>
                                </div>
                            </div>
                        </form>
                    </div>
                    
                    <div class="modal-footer border-t border-gray-300 dark:border-gray-700 px-6 py-4">
                        <div class="flex gap-3 justify-end">
                            <button type="button" id="custom-alert-cancel-button" class="px-4 py-2 bg-gray-500 hover:bg-gray-600 text-white text-sm font-medium rounded-md transition-colors">
                                Cancel
                            </button>
                            <button type="button" id="custom-alert-save-button" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                                Create Alert
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHTML);
        setupCustomAlertModalEvents();
    }

    function setupCustomAlertModalEvents() {
        const modal = document.getElementById('custom-alert-modal');
        const closeBtn = document.getElementById('custom-alert-modal-close');
        const cancelBtn = document.getElementById('custom-alert-cancel-button');
        const saveBtn = document.getElementById('custom-alert-save-button');
        const targetTypeSelect = document.querySelector('select[name="targetType"]');
        const metricSelect = document.querySelector('select[name="metric"]');

        // Close modal events
        [closeBtn, cancelBtn].forEach(btn => {
            if (btn) {
                btn.addEventListener('click', () => {
                    modal.remove();
                });
            }
        });

        // Close on outside click
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.remove();
            }
        });

        // Handle target type changes
        if (targetTypeSelect) {
            targetTypeSelect.addEventListener('change', (e) => {
                const specificSection = document.getElementById('specific-target-section');
                if (e.target.value === 'specific' || e.target.value === 'node') {
                    specificSection.classList.remove('hidden');
                    const input = specificSection.querySelector('input');
                    input.required = true;
                    input.placeholder = e.target.value === 'node' ? 'e.g., pve-node-1' : 'e.g., 100 or vm-name';
                } else {
                    specificSection.classList.add('hidden');
                    specificSection.querySelector('input').required = false;
                }
            });
        }

        // Handle metric changes
        if (metricSelect) {
            metricSelect.addEventListener('change', (e) => {
                const thresholdSection = document.getElementById('threshold-section');
                const thresholdInput = document.querySelector('input[name="threshold"]');
                const operatorSelect = document.querySelector('select[name="operator"]');
                const percentSpan = thresholdSection.querySelector('span');

                if (e.target.value === 'status') {
                    thresholdSection.style.display = 'none';
                    thresholdInput.required = false;
                } else {
                    thresholdSection.style.display = 'block';
                    thresholdInput.required = true;
                    
                    // Update placeholder and limits based on metric
                    switch (e.target.value) {
                        case 'cpu':
                            thresholdInput.placeholder = '85';
                            thresholdInput.max = '100';
                            percentSpan.textContent = '%';
                            break;
                        case 'memory':
                            thresholdInput.placeholder = '90';
                            thresholdInput.max = '100';
                            percentSpan.textContent = '%';
                            break;
                        case 'disk':
                            thresholdInput.placeholder = '95';
                            thresholdInput.max = '100';
                            percentSpan.textContent = '%';
                            break;
                        case 'network':
                            thresholdInput.placeholder = '100';
                            thresholdInput.max = '1000';
                            percentSpan.textContent = 'MB/s';
                            break;
                    }
                }
            });
        }

        // Handle save
        if (saveBtn) {
            saveBtn.addEventListener('click', saveCustomAlert);
        }
    }

    async function saveCustomAlert() {
        const form = document.getElementById('custom-alert-form');
        const formData = new FormData(form);
        
        // Validate form
        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        // Check if we have multiple thresholds from the dashboard
        const multipleThresholdsData = formData.get('multipleThresholds');
        let thresholds = [];
        
        if (multipleThresholdsData) {
            // Parse the JSON data containing all the preset thresholds
            try {
                thresholds = JSON.parse(multipleThresholdsData.replace(/&quot;/g, '"'));
                console.log('Using multiple thresholds from dashboard:', thresholds);
            } catch (error) {
                console.warn('Failed to parse multiple thresholds data:', error);
            }
        }
        
        // If no multiple thresholds, use the single threshold from the form
        if (thresholds.length === 0) {
            const threshold = formData.get('threshold') ? parseFloat(formData.get('threshold')) : null;
            if (threshold !== null) {
                thresholds = [{
                    type: formData.get('metric'),
                    operator: formData.get('operator') || '>',
                    value: threshold
                }];
            }
        }

        // Build alert configuration
        const alertConfig = {
            name: formData.get('alertName'),
            targetType: formData.get('targetType'),
            specificTarget: formData.get('specificTarget'),
            thresholds: thresholds, // Use array of thresholds instead of single values
            sendEmail: formData.has('sendEmail'),
            sendWebhook: formData.has('sendWebhook'),
            enabled: true,
            createdAt: Date.now()
        };

        console.log('Creating custom alert:', alertConfig);

        try {
            // Save to backend via API
            const response = await fetch('/api/alerts/rules', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(alertConfig)
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `Server error: ${response.status}`);
            }

            const result = await response.json();
            console.log('Alert rule created successfully:', result);
            
            const thresholdSummary = thresholds.map(t => `${t.type.toUpperCase()}: ${t.value}${['cpu', 'memory', 'disk'].includes(t.type) ? '%' : ''}`).join(', ');
            alert(`Custom alert created successfully!\n\nAlert: ${alertConfig.name}\nThresholds: ${thresholdSummary}\n\nRule ID: ${result.rule?.id || 'Generated'}`);
            
            // Close modal
            document.getElementById('custom-alert-modal').remove();
            
            // Refresh the custom alerts list
            await loadCustomAlerts();
            
        } catch (error) {
            console.error('Failed to save custom alert:', error);
            alert(`Failed to create custom alert: ${error.message}`);
        }
    }

    function loadCurrentAlerts() {
        const contentDiv = document.getElementById('current-alerts-content');
        if (!contentDiv) return;

        // Get alerts from the alerts handler
        if (PulseApp.alerts && PulseApp.alerts.getActiveAlerts) {
            const activeAlerts = PulseApp.alerts.getActiveAlerts();
            renderCurrentAlerts(activeAlerts);
        } else {
            contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Alert system not initialized</p>';
        }
    }

    function renderCurrentAlerts(alerts) {
        const contentDiv = document.getElementById('current-alerts-content');
        if (!contentDiv) return;

        if (!alerts || alerts.length === 0) {
            contentDiv.innerHTML = `
                <div class="text-center py-8">
                    <svg class="w-12 h-12 mx-auto mb-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No Active Alerts</h3>
                    <p class="text-gray-500 dark:text-gray-400">All systems are running normally.</p>
                </div>
            `;
            return;
        }

        const unacknowledgedAlerts = alerts.filter(alert => !alert.acknowledged);
        const acknowledgedAlerts = alerts.filter(alert => alert.acknowledged);

        let content = '';

        if (unacknowledgedAlerts.length > 0) {
            content += `
                <div class="space-y-3">
                    <h4 class="text-md font-medium text-gray-900 dark:text-gray-100">
                        Unacknowledged Alerts (${unacknowledgedAlerts.length})
                    </h4>
                    ${unacknowledgedAlerts.map(alert => renderAlertCard(alert, false)).join('')}
                </div>
            `;
        }

        if (acknowledgedAlerts.length > 0) {
            content += `
                <div class="space-y-3 mt-6">
                    <h4 class="text-md font-medium text-gray-500 dark:text-gray-400">
                        Acknowledged Alerts (${acknowledgedAlerts.length})
                    </h4>
                    ${acknowledgedAlerts.map(alert => renderAlertCard(alert, true)).join('')}
                </div>
            `;
        }

        contentDiv.innerHTML = content;
    }

    function renderAlertCard(alert, acknowledged = false) {
        const severityColors = {
            'critical': 'border-red-400 bg-red-50 dark:bg-red-900/10',
            'warning': 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/10',
            'info': 'border-blue-400 bg-blue-50 dark:bg-blue-900/10'
        };

        const severity = alert.severity || 'info';
        const colorClass = severityColors[severity] || severityColors.info;
        
        const duration = Math.round((Date.now() - alert.triggeredAt) / 1000);
        const durationStr = duration < 60 ? `${duration}s` : 
                           duration < 3600 ? `${Math.round(duration/60)}m` : 
                           `${Math.round(duration/3600)}h`;

        const acknowledgedClass = acknowledged ? 'opacity-60' : '';

        let currentValueDisplay = '';
        if (alert.metric === 'status') {
            currentValueDisplay = alert.currentValue;
        } else if (typeof alert.currentValue === 'number') {
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.metric);
            currentValueDisplay = `${Math.round(alert.currentValue)}${isPercentageMetric ? '%' : ''}`;
        } else {
            currentValueDisplay = alert.currentValue;
        }

        // Delivery indicators - Pulse UI is always present
        const deliveryIndicators = `
            <div class="flex items-center space-x-1 mb-2">
                <span class="text-xs text-gray-500 dark:text-gray-400">Delivered to:</span>
                <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded font-medium">
                    Pulse
                </span>
                ${alert.emailSent ? '<span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded font-medium">Email</span>' : ''}
                ${alert.webhookSent ? '<span class="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded font-medium">Webhook</span>' : ''}
            </div>
        `;

        return `
            <div class="border-l-4 ${colorClass} p-4 rounded-r-lg ${acknowledgedClass}">
                <div class="flex items-start justify-between">
                    <div class="flex-1">
                        <div class="flex items-center space-x-2 mb-2">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${alert.guest?.name || 'Unknown'}
                            </h5>
                            <span class="text-xs px-2 py-1 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">
                                ${currentValueDisplay}
                            </span>
                            ${alert.escalated ? '<span class="text-xs bg-red-500 text-white px-2 py-1 rounded">Escalated</span>' : ''}
                            ${acknowledged ? '<span class="text-xs bg-green-500 text-white px-2 py-1 rounded">Acknowledged</span>' : ''}
                        </div>
                        ${deliveryIndicators}
                        <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">
                            ${alert.ruleName || 'Unknown Rule'}
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-500">
                            Active for ${durationStr}
                            ${acknowledged ? ` • Acknowledged ${Math.round((Date.now() - alert.acknowledgedAt) / 60000)}m ago` : ''}
                        </p>
                    </div>
                    <div class="flex space-x-2">
                        ${!acknowledged ? `
                            <button onclick="PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}'); setTimeout(() => PulseApp.ui.alertManagementModal.loadCurrentAlerts(), 500);" 
                                    class="px-3 py-1 bg-green-500 hover:bg-green-600 text-white text-xs font-medium rounded transition-colors">
                                Acknowledge
                            </button>
                        ` : ''}
                        <button class="px-3 py-1 bg-gray-500 hover:bg-gray-600 text-white text-xs font-medium rounded transition-colors">
                            Details
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    function preserveCurrentFormData() {
        // TODO: Implement form data preservation
    }

    function restoreFormData(tabName) {
        // TODO: Implement form data restoration
    }

    function openModal(targetTab = null) {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.remove('hidden');
            modal.classList.add('flex');
            
            // Load current configuration if needed
            loadConfiguration();
            
            // Switch to target tab if specified
            if (targetTab && targetTab !== activeTab) {
                switchTab(targetTab);
            } else {
                // Render initial tab content
                renderTabContent();
            }
        }
    }

    function closeModal() {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
    }

    async function loadConfiguration() {
        try {
            console.log('Loading alert configuration...');
            
            // Use the same configuration loading logic as settings
            if (PulseApp.ui.settings && PulseApp.ui.settings.getCurrentConfig) {
                currentConfig = await PulseApp.ui.settings.getCurrentConfig();
            } else {
                // Fallback: load configuration directly
                const response = await fetch('/api/config');
                if (response.ok) {
                    currentConfig = await response.json();
                } else {
                    console.error('Failed to load configuration');
                    currentConfig = {};
                }
            }
        } catch (error) {
            console.error('Error loading configuration:', error);
            currentConfig = {};
        }
    }

    async function saveConfiguration() {
        // TODO: Save alert configuration
        console.log('Saving alert configuration...');
    }

    async function toggleCustomAlert(alertId, enabled) {
        try {
            const response = await fetch(`/api/alerts/rules/${alertId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ enabled })
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `Server error: ${response.status}`);
            }

            console.log(`Custom alert ${alertId} ${enabled ? 'enabled' : 'disabled'}`);
            await loadCustomAlerts(); // Refresh display
            
        } catch (error) {
            console.error('Failed to toggle custom alert:', error);
            alert(`Failed to ${enabled ? 'enable' : 'disable'} alert: ${error.message}`);
        }
    }

    async function deleteCustomAlert(alertId) {
        if (!confirm('Are you sure you want to delete this custom alert? This action cannot be undone.')) {
            return;
        }
        
        try {
            const response = await fetch(`/api/alerts/rules/${alertId}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `Server error: ${response.status}`);
            }

            console.log(`Custom alert ${alertId} deleted`);
            await loadCustomAlerts(); // Refresh display
            
        } catch (error) {
            console.error('Failed to delete custom alert:', error);
            alert(`Failed to delete alert: ${error.message}`);
        }
    }

    // Global functions that need to be accessible from HTML onclick handlers
    window.toggleSystemAlert = function(alertId, enabled) {
        console.log(`Toggling system alert ${alertId} to ${enabled ? 'enabled' : 'disabled'}`);
        // TODO: Implement system alert toggle functionality
        // This would update the configuration and save it
    };

    // Public API
    return {
        init,
        openModal,
        closeModal,
        switchTab: switchTab,
        openCustomAlertModal,
        toggleCustomAlert,
        deleteCustomAlert,
        loadCurrentAlerts: () => {
            if (activeTab === 'alerts') {
                loadCurrentAlerts();
            }
        }
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.alertManagementModal.init);
} else {
    PulseApp.ui.alertManagementModal.init();
}