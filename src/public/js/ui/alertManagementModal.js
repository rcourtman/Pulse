PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alertManagementModal = (() => {
    let isInitialized = false;
    let activeTab = 'alerts';
    let currentConfig = {};
    let formDataCache = {};

    function init() {
        if (isInitialized) return;
        
        
        // Create modal HTML
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
            <div id="alert-management-modal" class="fixed inset-0 z-50 hidden items-start justify-center bg-black bg-opacity-50 pt-8">
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
                                Alerts
                            </button>
                            <button class="alert-tab py-3 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-tab="alert-rules">
                                Alert Rules
                            </button>
                            <button class="alert-tab py-3 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-tab="notifications">
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

        // Show/hide the save button based on the active tab
        // Only show save button on notifications tab where there might be unsaved changes
        const saveButton = document.getElementById('alert-management-save-button');
        if (saveButton) {
            if (tabName === 'notifications') {
                saveButton.style.display = 'block';
            } else {
                saveButton.style.display = 'none';
            }
        }

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
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Global monitoring rules that apply to all VMs and LXCs unless overridden by custom settings</p>
                                </div>
                                <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded-full">Built-in</span>
                            </div>
                            
                            <div id="system-alerts-content" class="space-y-3">
                                <!-- CPU Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                                    <div class="flex items-start justify-between mb-2">
                                        <div class="flex-1">
                                            <div class="flex items-center gap-2 mb-1">
                                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">CPU Alert</h5>
                                                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-full" id="cpu-status-badge">Loading...</span>
                                            </div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                                                Built-in system rule • Target: All VMs/LXCs
                                            </div>
                                        </div>
                                        <div class="flex gap-1">
                                            <button onclick="editSystemAlert('cpu')" class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                                                Edit
                                            </button>
                                            <button onclick="toggleSystemAlert('cpu', false)" class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded" id="cpu-toggle-btn">
                                                Disable
                                            </button>
                                        </div>
                                    </div>
                                    <div>
                                        <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Triggers when CPU usage exceeds <span id="cpu-threshold-display">85%</span></p>
                                        <div class="flex flex-wrap">
                                            <span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">CPU ≥ <span id="cpu-threshold-badge">85</span>%</span>
                                        </div>
                                    </div>
                                </div>

                                <!-- Memory Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                                    <div class="flex items-start justify-between mb-2">
                                        <div class="flex-1">
                                            <div class="flex items-center gap-2 mb-1">
                                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Memory Alert</h5>
                                                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-full" id="memory-status-badge">Loading...</span>
                                            </div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                                                Built-in system rule • Target: All VMs/LXCs
                                            </div>
                                        </div>
                                        <div class="flex gap-1">
                                            <button onclick="editSystemAlert('memory')" class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                                                Edit
                                            </button>
                                            <button onclick="toggleSystemAlert('memory', false)" class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded" id="memory-toggle-btn">
                                                Disable
                                            </button>
                                        </div>
                                    </div>
                                    <div>
                                        <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Triggers when memory usage exceeds <span id="memory-threshold-display">90%</span></p>
                                        <div class="flex flex-wrap">
                                            <span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">Memory ≥ <span id="memory-threshold-badge">90</span>%</span>
                                        </div>
                                    </div>
                                </div>

                                <!-- Disk Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                                    <div class="flex items-start justify-between mb-2">
                                        <div class="flex-1">
                                            <div class="flex items-center gap-2 mb-1">
                                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Disk Alert</h5>
                                                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-full" id="disk-status-badge">Loading...</span>
                                            </div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                                                Built-in system rule • Target: All VMs/LXCs
                                            </div>
                                        </div>
                                        <div class="flex gap-1">
                                            <button onclick="editSystemAlert('disk')" class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                                                Edit
                                            </button>
                                            <button onclick="toggleSystemAlert('disk', false)" class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded" id="disk-toggle-btn">
                                                Disable
                                            </button>
                                        </div>
                                    </div>
                                    <div>
                                        <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Triggers when disk usage exceeds <span id="disk-threshold-display">95%</span></p>
                                        <div class="flex flex-wrap">
                                            <span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">Disk ≥ <span id="disk-threshold-badge">95</span>%</span>
                                        </div>
                                    </div>
                                </div>

                                <!-- Down Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                                    <div class="flex items-start justify-between mb-2">
                                        <div class="flex-1">
                                            <div class="flex items-center gap-2 mb-1">
                                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Down Alert</h5>
                                                <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-full" id="down-status-badge">Loading...</span>
                                            </div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                                                Built-in system rule • Target: All VMs/LXCs
                                            </div>
                                        </div>
                                        <div class="flex gap-1">
                                            <button onclick="editSystemAlert('down')" class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                                                Edit
                                            </button>
                                            <button onclick="toggleSystemAlert('down', false)" class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded" id="down-toggle-btn">
                                                Disable
                                            </button>
                                        </div>
                                    </div>
                                    <div>
                                        <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">Triggers when VM/LXC becomes unreachable or stops responding</p>
                                        <div class="flex flex-wrap">
                                            <span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">System Down Alert</span>
                                        </div>
                                    </div>
                                </div>
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
                            <div id="email-config-section" class="space-y-6">
                                <!-- Email Provider Selection -->
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                                        Choose your email provider
                                    </label>
                                    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                                        <button type="button" class="email-provider-btn p-3 border-2 border-gray-200 dark:border-gray-600 rounded-lg text-center hover:border-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-all" data-provider="gmail">
                                            <div class="text-sm font-medium text-gray-700 dark:text-gray-300">Gmail</div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400">Google</div>
                                        </button>
                                        <button type="button" class="email-provider-btn p-3 border-2 border-gray-200 dark:border-gray-600 rounded-lg text-center hover:border-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-all" data-provider="outlook">
                                            <div class="text-sm font-medium text-gray-700 dark:text-gray-300">Outlook</div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400">Microsoft</div>
                                        </button>
                                        <button type="button" class="email-provider-btn p-3 border-2 border-gray-200 dark:border-gray-600 rounded-lg text-center hover:border-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-all" data-provider="yahoo">
                                            <div class="text-sm font-medium text-gray-700 dark:text-gray-300">Yahoo</div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400">Yahoo Mail</div>
                                        </button>
                                        <button type="button" class="email-provider-btn p-3 border-2 border-gray-200 dark:border-gray-600 rounded-lg text-center hover:border-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-all" data-provider="custom">
                                            <div class="text-sm font-medium text-gray-700 dark:text-gray-300">Other</div>
                                            <div class="text-xs text-gray-500 dark:text-gray-400">Custom SMTP</div>
                                        </button>
                                    </div>
                                    <div id="email-provider-help" class="text-xs text-gray-500 dark:text-gray-400 p-3 bg-gray-50 dark:bg-gray-800 rounded-md hidden">
                                        <!-- Provider-specific help will be shown here -->
                                    </div>
                                    
                                    <!-- App Password Help -->
                                    <div id="app-password-help" class="hidden bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg p-4">
                                        <div class="flex items-start gap-3">
                                            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                            </svg>
                                            <div class="flex-1">
                                                <h5 class="text-sm font-medium text-blue-800 dark:text-blue-200 mb-1" id="app-password-title">App Password Required</h5>
                                                <p class="text-sm text-blue-700 dark:text-blue-300 mb-3" id="app-password-description">
                                                    You need to create an app password for Pulse to send emails.
                                                </p>
                                                <a id="app-password-link" href="#" target="_blank" rel="noopener noreferrer" 
                                                   class="inline-flex items-center px-3 py-2 text-sm font-medium text-blue-600 dark:text-blue-400 bg-white dark:bg-gray-800 border border-blue-200 dark:border-blue-600 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900/50 transition-colors">
                                                    <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-1M14 6h6m0 0v6m0-6L10 16"/>
                                                    </svg>
                                                    Create App Password
                                                </a>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Basic Email Configuration -->
                                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                            Your Email Address
                                            <span class="text-gray-500 text-xs ml-1">(for sending alerts)</span>
                                        </label>
                                        <input type="email" id="email-from-input" name="ALERT_FROM_EMAIL"
                                               placeholder="your-email@gmail.com"
                                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                            Alert Recipients
                                            <span class="text-gray-500 text-xs ml-1">(who gets notified)</span>
                                        </label>
                                        <input type="text" name="ALERT_TO_EMAIL"
                                               placeholder="admin@example.com, tech@example.com"
                                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    </div>
                                </div>

                                <!-- Password/App Password -->
                                <div>
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                        <span id="password-label">Password</span>
                                        <span class="text-gray-500 text-xs ml-1" id="password-help">(for email authentication)</span>
                                    </label>
                                    <input type="password" name="ALERT_EMAIL_PASSWORD"
                                           placeholder="Enter your email password or app password"
                                           class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                </div>

                                <!-- Advanced SMTP Settings (Collapsible) -->
                                <div>
                                    <button type="button" id="toggle-advanced-smtp" class="flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100">
                                        <svg class="w-4 h-4 mr-2 transform transition-transform" id="advanced-smtp-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
                                        </svg>
                                        Advanced SMTP Settings
                                    </button>
                                    <div id="advanced-smtp-settings" class="hidden mt-3 p-4 border border-gray-200 dark:border-gray-600 rounded-lg bg-gray-50 dark:bg-gray-800">
                                        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                                            <div>
                                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">SMTP Host</label>
                                                <input type="text" name="ALERT_SMTP_HOST" 
                                                       placeholder="smtp.gmail.com"
                                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                            </div>
                                            <div>
                                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">SMTP Port</label>
                                                <input type="number" name="ALERT_SMTP_PORT" 
                                                       placeholder="587"
                                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                            </div>
                                            <div>
                                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                                                <input type="text" name="ALERT_SMTP_USER" 
                                                       placeholder="your-email@gmail.com"
                                                       class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                            </div>
                                        </div>
                                        <div class="mt-4">
                                            <label class="flex items-center">
                                                <input type="checkbox" name="ALERT_SMTP_SECURE" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                                <span class="text-sm text-gray-700 dark:text-gray-300">Use SSL/TLS encryption</span>
                                            </label>
                                        </div>
                                    </div>
                                </div>

                                <!-- Test Email Button -->
                                <div class="flex space-x-3">
                                    <button type="button" id="test-email-btn" class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-lg transition-colors">
                                        <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 4.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
                                        </svg>
                                        Send Test Email
                                    </button>
                                    <button type="button" id="save-email-config-btn" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors">
                                        Save Email Configuration
                                    </button>
                                </div>
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
            // Initially hide the button since system tab is active by default
            addCustomBtn.classList.add('hidden');
        }

        // Set up system alert toggles
        const systemAlertToggles = document.querySelectorAll('#system-alerts-content input[type="checkbox"]');
        systemAlertToggles.forEach(toggle => {
            toggle.addEventListener('change', (e) => {
                const alertType = e.target.id.replace('-alert-enabled', '');
                updateSystemAlertStatus(alertType, e.target.checked);
            });
        });
        
        // Load system and custom alerts
        // Ensure we have the latest configuration before loading alerts
        loadConfiguration().then(() => {
            loadSystemAlerts();
            loadCustomAlerts();
        });
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

        // Toggle Add Custom Alert button visibility
        const addCustomAlertBtn = document.getElementById('add-custom-alert-btn');
        if (addCustomAlertBtn) {
            if (tabName === 'custom') {
                addCustomAlertBtn.classList.remove('hidden');
            } else {
                addCustomAlertBtn.classList.add('hidden');
            }
        }
    }

    function initializeNotificationsTab() {
        // Set up nested tab navigation
        const subTabs = document.querySelectorAll('.notification-sub-tab');
        subTabs.forEach(tab => {
            tab.addEventListener('click', (e) => {
                const targetTab = e.currentTarget.dataset.tab;
                switchNotificationSubTab(targetTab);
            });
        });

        // Set up email provider selection
        const emailProviderBtns = document.querySelectorAll('.email-provider-btn');
        emailProviderBtns.forEach(btn => {
            btn.addEventListener('click', (e) => {
                const provider = e.currentTarget.dataset.provider;
                handleEmailProviderSelection(provider);
            });
        });

        // Set up auto-detection when email address is entered
        const emailFromInput = document.getElementById('email-from-input');
        if (emailFromInput) {
            emailFromInput.addEventListener('blur', () => {
                autoDetectEmailProvider(emailFromInput.value);
            });
        }

        // Set up advanced SMTP toggle
        const advancedToggle = document.getElementById('toggle-advanced-smtp');
        if (advancedToggle) {
            advancedToggle.addEventListener('click', toggleAdvancedSMTPSettings);
        }

        // Set up email test and save buttons
        const testEmailBtn = document.getElementById('test-email-btn');
        if (testEmailBtn) {
            testEmailBtn.addEventListener('click', testEmailConnection);
        }

        const saveEmailBtn = document.getElementById('save-email-config-btn');
        if (saveEmailBtn) {
            saveEmailBtn.addEventListener('click', saveEmailConfiguration);
        }

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
            });
        }
        
        const webhookToggle = document.querySelector('input[name="GLOBAL_WEBHOOK_ENABLED"]');
        if (webhookToggle) {
            webhookToggle.addEventListener('change', (e) => {
            });
        }

        // Load existing email configuration
        loadEmailConfiguration();
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
        // System alerts are now hardcoded in HTML, just load their configuration
        loadSystemAlertConfiguration();
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
                ${t.metric.charAt(0).toUpperCase() + t.metric.slice(1)} ≥ ${t.threshold}${['cpu', 'memory', 'disk'].includes(t.metric) ? '%' : ''}
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
                        <button onclick="PulseApp.ui.alertManagementModal.editCustomAlert('${alert.id}')" 
                                class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                            Edit
                        </button>
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
                    <div class="flex flex-wrap mb-3">
                        ${thresholdsList}
                    </div>
                    <div class="text-xs text-gray-600 dark:text-gray-400">
                        <strong>Notifications:</strong>
                        <div class="flex items-center gap-1 mt-1">
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300">
                                Pulse UI
                            </span>
                            ${alert.sendEmail ? '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300">Email</span>' : ''}
                            ${alert.sendWebhook ? '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300">Webhook</span>' : ''}
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function createSystemAlertCard(alert) {
        const thresholdBadge = alert.threshold ? 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${alert.name.split(' ')[0]} ≥ ${alert.threshold}%
            </span>` : '';
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                <div class="flex items-start justify-between mb-2">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-1">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${alert.name}</h5>
                            <span class="text-xs px-2 py-0.5 ${alert.enabled ? 'bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'} rounded-full">
                                ${alert.enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Built-in system rule • Target: All VMs/LXCs
                        </div>
                    </div>
                    <div class="flex gap-1">
                        ${alert.threshold ? `
                            <button onclick="editSystemAlert('${alert.id}')" 
                                    class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded">
                                Edit
                            </button>
                        ` : ''}
                        <button onclick="toggleSystemAlert('${alert.id}', ${!alert.enabled})" 
                                class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded">
                            ${alert.enabled ? 'Disable' : 'Enable'}
                        </button>
                    </div>
                </div>
                <div>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">${alert.description}</p>
                    <div class="flex flex-wrap">
                        ${thresholdBadge}
                    </div>
                </div>
            </div>
        `;
    }

    function loadEmailConfiguration() {
        const emailConfigSection = document.getElementById('email-config-section');
        if (!emailConfigSection) return;
        
        // Get email configuration from currentConfig
        const config = currentConfig || {};
        const smtp = config.advanced?.smtp || {};
        
        // Populate email fields with values from configuration
        const emailFromInput = document.querySelector('input[name="ALERT_FROM_EMAIL"]');
        if (emailFromInput && smtp.from) {
            emailFromInput.value = smtp.from;
        }
        
        const emailToInput = document.querySelector('input[name="ALERT_TO_EMAIL"]');
        if (emailToInput && smtp.to) {
            emailToInput.value = smtp.to;
        }
        
        const smtpHostInput = document.querySelector('input[name="ALERT_SMTP_HOST"]');
        if (smtpHostInput && smtp.host) {
            smtpHostInput.value = smtp.host;
        }
        
        const smtpPortInput = document.querySelector('input[name="ALERT_SMTP_PORT"]');
        if (smtpPortInput && smtp.port) {
            smtpPortInput.value = smtp.port;
        }
        
        const smtpUserInput = document.querySelector('input[name="ALERT_SMTP_USER"]');
        if (smtpUserInput && smtp.user) {
            smtpUserInput.value = smtp.user;
        }
        
        const smtpSecureInput = document.querySelector('input[name="ALERT_SMTP_SECURE"]');
        if (smtpSecureInput && smtp.secure !== undefined) {
            smtpSecureInput.checked = smtp.secure;
        }
        
        const smtpPassInput = document.querySelector('input[name="ALERT_EMAIL_PASSWORD"]');
        if (smtpPassInput && smtp.pass) {
            smtpPassInput.value = smtp.pass;
        }
        
        // Auto-detect email provider if from email is set
        if (emailFromInput && emailFromInput.value) {
            autoDetectEmailProvider(emailFromInput.value);
        }
        
        // Use existing email configuration from settings if available
        if (PulseApp.ui.settings && PulseApp.ui.settings.renderAlertsTab) {
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
            PulseApp.ui.toast.warning('Please enter a webhook URL');
            return;
        }
        
        // TODO: Implement webhook configuration saving
        PulseApp.ui.toast.success('Webhook configuration saved successfully!');
    }

    function handleEmailProviderSelection(provider) {
        // Remove active state from all buttons
        document.querySelectorAll('.email-provider-btn').forEach(btn => {
            btn.classList.remove('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
        });
        
        // Add active state to selected button
        const selectedBtn = document.querySelector(`[data-provider="${provider}"]`);
        selectedBtn.classList.add('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
        
        // Update form fields based on provider
        const helpDiv = document.getElementById('email-provider-help');
        const passwordLabel = document.getElementById('password-label');
        const passwordHelp = document.getElementById('password-help');
        const hostInput = document.querySelector('input[name="ALERT_SMTP_HOST"]');
        const portInput = document.querySelector('input[name="ALERT_SMTP_PORT"]');
        const secureCheckbox = document.querySelector('input[name="ALERT_SMTP_SECURE"]');
        
        const providers = {
            gmail: {
                host: 'smtp.gmail.com',
                port: 587,
                secure: true,
                passwordLabel: 'App Password',
                passwordHelp: '(Generate from Google Account settings)',
                help: 'For Gmail, you need to enable 2-factor authentication and generate an App Password. Go to Google Account → Security → 2-Step Verification → App passwords.',
                appPasswordUrl: 'https://myaccount.google.com/apppasswords',
                appPasswordTitle: 'Gmail App Password Required',
                appPasswordDesc: 'Gmail requires an app password for third-party applications. Your regular password won\'t work.'
            },
            outlook: {
                host: 'smtp-mail.outlook.com',
                port: 587,
                secure: true,
                passwordLabel: 'Password',
                passwordHelp: '(Your Microsoft account password)',
                help: 'Use your regular Microsoft account password. If you have 2FA enabled, you may need to generate an app password.',
                appPasswordUrl: 'https://account.microsoft.com/security/app-passwords',
                appPasswordTitle: 'Outlook App Password (If 2FA Enabled)',
                appPasswordDesc: 'If you have 2-factor authentication enabled, you\'ll need an app password. Otherwise, use your regular password.'
            },
            yahoo: {
                host: 'smtp.mail.yahoo.com',
                port: 587,
                secure: true,
                passwordLabel: 'App Password',
                passwordHelp: '(Generate from Yahoo Account settings)',
                help: 'For Yahoo Mail, you need to generate an App Password. Go to Yahoo Account Info → Account Security → Generate app password.',
                appPasswordUrl: 'https://login.yahoo.com/account/security/app-passwords',
                appPasswordTitle: 'Yahoo App Password Required',
                appPasswordDesc: 'Yahoo Mail requires an app password for third-party applications.'
            },
            custom: {
                host: '',
                port: 587,
                secure: true,
                passwordLabel: 'Password',
                passwordHelp: '(SMTP authentication password)',
                help: 'Enter the SMTP settings provided by your email provider. Check their documentation for the correct host, port, and security settings.'
            }
        };
        
        const config = providers[provider];
        if (config) {
            if (hostInput) hostInput.value = config.host;
            if (portInput) portInput.value = config.port;
            if (secureCheckbox) secureCheckbox.checked = config.secure;
            if (passwordLabel) passwordLabel.textContent = config.passwordLabel;
            if (passwordHelp) passwordHelp.textContent = config.passwordHelp;
            
            if (helpDiv) {
                helpDiv.textContent = config.help;
                helpDiv.classList.remove('hidden');
            }

            // Show app password help for providers that need it
            showAppPasswordHelp(config);
            
            // Auto-fill username with from email if available
            const fromEmailInput = document.getElementById('email-from-input');
            const usernameInput = document.querySelector('input[name="ALERT_SMTP_USER"]');
            if (fromEmailInput && usernameInput && fromEmailInput.value && provider !== 'custom') {
                usernameInput.value = fromEmailInput.value;
            }
            
            // Auto-expand advanced settings to show what was configured
            const advancedSettings = document.getElementById('advanced-smtp-settings');
            const toggleBtn = document.getElementById('toggle-advanced-smtp');
            if (advancedSettings && advancedSettings.classList.contains('hidden')) {
                toggleAdvancedSMTPSettings();
            }
        } else {
            // Hide app password help for custom provider
            hideAppPasswordHelp();
        }
    }
    
    function showAppPasswordHelp(config) {
        const helpDiv = document.getElementById('app-password-help');
        const titleEl = document.getElementById('app-password-title');
        const descEl = document.getElementById('app-password-description');
        const linkEl = document.getElementById('app-password-link');

        if (config.appPasswordUrl && helpDiv && titleEl && descEl && linkEl) {
            helpDiv.classList.remove('hidden');
            titleEl.textContent = config.appPasswordTitle;
            descEl.textContent = config.appPasswordDesc;
            linkEl.href = config.appPasswordUrl;
        }
    }

    function hideAppPasswordHelp() {
        const helpDiv = document.getElementById('app-password-help');
        if (helpDiv) {
            helpDiv.classList.add('hidden');
        }
    }

    function autoDetectEmailProvider(email) {
        if (!email || !email.includes('@')) return;
        
        const domain = email.split('@')[1].toLowerCase();
        const providers = {
            'gmail.com': 'gmail',
            'googlemail.com': 'gmail',
            'outlook.com': 'outlook',
            'hotmail.com': 'outlook',
            'live.com': 'outlook',
            'msn.com': 'outlook',
            'yahoo.com': 'yahoo',
            'yahoo.co.uk': 'yahoo',
            'yahoo.ca': 'yahoo',
            'ymail.com': 'yahoo'
        };

        const provider = providers[domain];
        if (provider) {
            handleEmailProviderSelection(provider);
        }
    }

    function toggleAdvancedSMTPSettings() {
        const settingsDiv = document.getElementById('advanced-smtp-settings');
        const icon = document.getElementById('advanced-smtp-icon');
        
        if (settingsDiv.classList.contains('hidden')) {
            settingsDiv.classList.remove('hidden');
            icon.style.transform = 'rotate(90deg)';
        } else {
            settingsDiv.classList.add('hidden');
            icon.style.transform = 'rotate(0deg)';
        }
    }
    
    async function testEmailConnection() {
        // Collect email configuration
        const emailConfig = {
            from: document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value,
            to: document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value,
            pass: document.querySelector('input[name="ALERT_EMAIL_PASSWORD"]')?.value, // Backend expects 'pass' not 'password'
            host: document.querySelector('input[name="ALERT_SMTP_HOST"]')?.value,
            port: document.querySelector('input[name="ALERT_SMTP_PORT"]')?.value,
            user: document.querySelector('input[name="ALERT_SMTP_USER"]')?.value,
            secure: document.querySelector('input[name="ALERT_SMTP_SECURE"]')?.checked
        };
        
        if (!emailConfig.from || !emailConfig.to) {
            PulseApp.ui.toast.warning('Please enter both sender and recipient email addresses');
            return;
        }
        
        // Send test email via API
        try {
            const response = await fetch('/api/test-email', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(emailConfig)
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast.success('Test email sent successfully! Please check your inbox.');
            } else {
                PulseApp.ui.toast.error('Test email failed: ' + (result.error || 'Unknown error'));
            }
        } catch (error) {
            console.error('[Email Test] Error:', error);
            PulseApp.ui.toast.error('Error sending test email: ' + error.message);
        }
    }
    
    async function saveEmailConfiguration() {
        // This function is called by the specific save button in the email section
        // It will trigger the main saveConfiguration function
        await saveConfiguration();
        PulseApp.ui.toast.success('Email configuration saved successfully!');
    }

    function editSystemAlert(alertType) {
        // Create and show modal for editing system alert
        const modal = document.createElement('div');
        modal.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50';
        
        const config = PulseApp.config?.alerts?.[alertType] || {};
        const defaultThresholds = { cpu: 85, memory: 90, disk: 95 };
        const currentThreshold = config.threshold || defaultThresholds[alertType] || 85;
        const isEnabled = config.enabled !== false;
        
        modal.innerHTML = `
            <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
                <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Edit ${alertType.charAt(0).toUpperCase() + alertType.slice(1)} Alert</h3>
                </div>
                <div class="px-6 py-4 space-y-4">
                    <div>
                        <label class="flex items-center">
                            <input type="checkbox" id="system-alert-enabled" ${isEnabled ? 'checked' : ''} class="mr-3 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">Enable ${alertType} alerts</span>
                        </label>
                    </div>
                    ${alertType !== 'down' ? `
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Alert Threshold (%)
                            </label>
                            <input type="number" id="system-alert-threshold" value="${currentThreshold}" min="50" max="100" 
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Alert when ${alertType} usage exceeds this percentage</p>
                        </div>
                    ` : ''}
                </div>
                <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end space-x-3">
                    <button onclick="closeSystemAlertModal()" class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600">
                        Cancel
                    </button>
                    <button onclick="saveSystemAlert('${alertType}')" class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md">
                        Save
                    </button>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
        window.currentSystemAlertModal = modal;
    }
    
    function closeSystemAlertModal() {
        if (window.currentSystemAlertModal) {
            document.body.removeChild(window.currentSystemAlertModal);
            window.currentSystemAlertModal = null;
        }
    }
    
    function saveSystemAlert(alertType) {
        const enabled = document.getElementById('system-alert-enabled')?.checked;
        const thresholdInput = document.getElementById('system-alert-threshold');
        const threshold = thresholdInput ? parseInt(thresholdInput.value) : null;
        
        const alertConfig = { enabled };
        if (threshold && alertType !== 'down') {
            alertConfig.threshold = threshold;
        }
        
        // Save to backend
        const configUpdate = {};
        configUpdate[`ALERT_${alertType.toUpperCase()}_ENABLED`] = enabled ? 'true' : 'false';
        if (threshold && alertType !== 'down') {
            configUpdate[`ALERT_${alertType.toUpperCase()}_THRESHOLD`] = threshold.toString();
        }
        
        // Save via config API
        fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(configUpdate)
        }).then(response => response.json()).then(result => {
            if (!result.success) {
                console.error('Failed to save alert configuration:', result.error);
                PulseApp.ui.toast.error('Failed to save alert configuration');
            }
        }).catch(error => {
            console.error('Error saving alert configuration:', error);
            PulseApp.ui.toast.error('Failed to save alert configuration');
        });
        
        // Update the UI immediately
        updateSystemAlertDisplay(alertType, alertConfig);
        
        PulseApp.ui.toast.success(`${alertType.charAt(0).toUpperCase() + alertType.slice(1)} alert settings saved successfully!`);
        closeSystemAlertModal();
    }
    
    function updateSystemAlertStatus(alertType, enabled) {
        
        // Save status change to backend
        const configUpdate = {};
        configUpdate[`ALERT_${alertType.toUpperCase()}_ENABLED`] = enabled ? 'true' : 'false';
        
        fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(configUpdate)
        }).then(response => response.json()).then(result => {
            if (result.success) {
                PulseApp.ui.toast.success(`${alertType.charAt(0).toUpperCase() + alertType.slice(1)} alert ${enabled ? 'enabled' : 'disabled'} successfully`);
                // Reload configuration to ensure UI reflects the saved state
                loadConfiguration().then(() => {
                    // Update the display with the fresh config
                    const alertConfig = currentConfig?.advanced?.alerts?.[alertType] || {};
                    updateSystemAlertDisplay(alertType, { enabled: alertConfig.enabled !== false });
                    
                    // If we just enabled an alert, trigger immediate evaluation of current state
                    if (enabled) {
                        triggerImmediateAlertEvaluation();
                    }
                });
            } else {
                console.error('[DEBUG] Failed to save alert status:', result.error);
                PulseApp.ui.toast.error('Failed to save alert status');
            }
        }).catch(error => {
            console.error('[DEBUG] Error saving alert status:', error);
            PulseApp.ui.toast.error('Failed to save alert status');
        });
    }

    function triggerImmediateAlertEvaluation() {
        
        PulseApp.ui.toast.alert('Alert enabled - checking for existing conditions...');
        
        // Call the API endpoint to trigger immediate evaluation
        fetch('/api/alerts/evaluate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                // Refresh the main page alerts display
                setTimeout(() => {
                    if (typeof window.updateAlertsDisplay === 'function') {
                        window.updateAlertsDisplay();
                    }
                }, 1000);
            } else {
                console.error('[DEBUG] Failed to trigger alert evaluation:', data.error);
            }
        })
        .catch(error => {
            console.error('[DEBUG] Error calling alert evaluation endpoint:', error);
        });
    }
    
    function updateSystemAlertDisplay(alertType, config) {
        
        // Update the status badge
        const statusBadge = document.getElementById(`${alertType}-status-badge`);
        if (statusBadge) {
            const newText = config.enabled ? 'Enabled' : 'Disabled';
            const newClassName = `text-xs px-2 py-0.5 rounded-full ${
                config.enabled 
                    ? 'bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300' 
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
            }`;
            
            statusBadge.textContent = newText;
            statusBadge.className = newClassName;
        } else {
        }
        
        // Update the toggle button text and onclick
        const toggleBtn = document.getElementById(`${alertType}-toggle-btn`);
        if (toggleBtn) {
            const newText = config.enabled ? 'Disable' : 'Enable';
            toggleBtn.textContent = newText;
            toggleBtn.onclick = () => toggleSystemAlert(alertType, !config.enabled);
        } else {
        }
        
        // Update the threshold display
        if (config.threshold && alertType !== 'down') {
            const display = document.getElementById(`${alertType}-threshold-display`);
            if (display) {
                display.textContent = `${config.threshold}%`;
            } else {
            }
        }
    }

    function loadSystemAlertConfiguration() {
        // Load from current configuration and populate system alert displays
        
        const config = currentConfig?.advanced?.alerts || {};
        
        // Update system alert displays
        ['cpu', 'memory', 'disk', 'down'].forEach(alertType => {
            const alertConfig = config[alertType] || {};
            const enabled = alertConfig.enabled !== false;
            const defaultThresholds = { cpu: 85, memory: 90, disk: 95 };
            const threshold = alertConfig.threshold || defaultThresholds[alertType];
            
            
            updateSystemAlertDisplay(alertType, { enabled, threshold });
        });
    }

    function testWebhookConnection() {
        const webhookUrl = document.getElementById('webhook-url-input')?.value;
        if (!webhookUrl) {
            PulseApp.ui.toast.warning('Please enter a webhook URL first');
            return;
        }
        
        // Use existing webhook test functionality from settings if available
        if (PulseApp.ui.settings && PulseApp.ui.settings.setupWebhookTestButton) {
            PulseApp.ui.settings.setupWebhookTestButton();
        } else {
            // Fallback test
            PulseApp.ui.toast.info('Webhook test functionality will be implemented');
        }
    }

    function openCustomAlertModal(presetThresholds = null, existingAlert = null) {
        // Check if settings modal has the threshold functionality we can reuse
        if (PulseApp.ui.settings && PulseApp.ui.settings.showThresholdModal && !presetThresholds && !existingAlert) {
            // Use the existing threshold modal from settings (only if no presets and not editing)
            PulseApp.ui.settings.showThresholdModal();
        } else {
            // Create our own custom alert modal with optional preset values or existing alert data
            createCustomAlertModal(presetThresholds, existingAlert);
        }
    }

    function createCustomAlertModal(presetThresholds = null, existingAlert = null) {
        // Remove any existing custom alert modal
        const existingModal = document.getElementById('custom-alert-modal');
        if (existingModal) {
            existingModal.remove();
        }

        // Generate alert name and determine best metric from presets or existing alert
        let suggestedName = 'Custom Alert';
        let primaryMetric = 'cpu';
        let primaryThreshold = 85;
        let multipleThresholds = false;
        let isEditing = false;
        
        if (existingAlert) {
            // Pre-fill form with existing alert data
            suggestedName = existingAlert.name || existingAlert.alert || 'Custom Alert';
            if (existingAlert.thresholds && existingAlert.thresholds.length > 0) {
                primaryMetric = existingAlert.thresholds[0].metric || existingAlert.thresholds[0].type;
                primaryThreshold = existingAlert.thresholds[0].threshold || existingAlert.thresholds[0].value;
                multipleThresholds = existingAlert.thresholds.length > 1;
            }
            isEditing = true;
        } else if (presetThresholds && presetThresholds.length > 0) {
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
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">${isEditing ? 'Edit Custom Alert' : 'Create Custom Alert'}</h2>
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

                            ${multipleThresholds ? `<input type="hidden" name="multipleThresholds" value="${JSON.stringify(presetThresholds).replace(/"/g, '&quot;')}">` : ''}

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
                                <select name="specificTarget" id="specificTargetSelect"
                                        class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    <option value="">Select a VM or LXC...</option>
                                </select>
                            </div>

                            <div id="threshold-section">
                                <div class="bg-blue-50 dark:bg-blue-900/20 border-2 border-blue-300 dark:border-blue-600 rounded-lg p-4">
                                    <div class="flex items-center gap-2 mb-3">
                                        <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zM3 10a1 1 0 011-1h6a1 1 0 110 2H4a1 1 0 01-1-1zM3 16a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z" clip-rule="evenodd"></path>
                                        </svg>
                                        <h4 class="text-sm font-semibold text-blue-900 dark:text-blue-100">Alert Thresholds</h4>
                                    </div>
                                    <p class="text-xs text-blue-700 dark:text-blue-300 mb-3">
                                        Set thresholds for the metrics you want to monitor. Alert will trigger when <strong>ANY</strong> of these conditions are met. Leave thresholds at 0 to disable.
                                    </p>
                                    
                                    <div class="space-y-4">
                                        <!-- CPU Threshold -->
                                        <div class="threshold-input-group">
                                            <div class="flex items-center justify-between mb-1">
                                                <label class="text-xs font-medium text-blue-800 dark:text-blue-200">CPU Usage</label>
                                                <span id="cpu-threshold-value" class="text-xs text-blue-600 dark:text-blue-300">0%</span>
                                            </div>
                                            <input type="range" name="cpuThreshold" min="0" max="100" step="1" value="0" 
                                                   class="w-full h-2 bg-gray-200 dark:bg-gray-600 rounded-lg appearance-none cursor-pointer range-slider">
                                        </div>
                                        
                                        <!-- Memory Threshold -->
                                        <div class="threshold-input-group">
                                            <div class="flex items-center justify-between mb-1">
                                                <label class="text-xs font-medium text-blue-800 dark:text-blue-200">Memory Usage</label>
                                                <span id="memory-threshold-value" class="text-xs text-blue-600 dark:text-blue-300">0%</span>
                                            </div>
                                            <input type="range" name="memoryThreshold" min="0" max="100" step="1" value="0" 
                                                   class="w-full h-2 bg-gray-200 dark:bg-gray-600 rounded-lg appearance-none cursor-pointer range-slider">
                                        </div>
                                        
                                        <!-- Disk Threshold -->
                                        <div class="threshold-input-group">
                                            <div class="flex items-center justify-between mb-1">
                                                <label class="text-xs font-medium text-blue-800 dark:text-blue-200">Disk Usage</label>
                                                <span id="disk-threshold-value" class="text-xs text-blue-600 dark:text-blue-300">0%</span>
                                            </div>
                                            <input type="range" name="diskThreshold" min="0" max="100" step="1" value="0" 
                                                   class="w-full h-2 bg-gray-200 dark:bg-gray-600 rounded-lg appearance-none cursor-pointer range-slider">
                                        </div>
                                        
                                        <!-- Network Activity Threshold -->
                                        <div class="threshold-input-group">
                                            <label class="block text-xs font-medium text-blue-800 dark:text-blue-200 mb-1">Network Activity</label>
                                            <select name="networkThreshold" class="w-full px-2 py-1 text-xs border border-blue-300 dark:border-blue-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
                                                <option value="0">Disabled</option>
                                                <option value="1048576">&gt; 1 MB/s</option>
                                                <option value="10485760">&gt; 10 MB/s</option>
                                                <option value="52428800">&gt; 50 MB/s</option>
                                                <option value="104857600">&gt; 100 MB/s</option>
                                            </select>
                                        </div>
                                        
                                        <!-- VM/LXC Status Monitoring -->
                                        <div class="threshold-input-group">
                                            <label class="flex items-center">
                                                <input type="checkbox" name="statusMonitoring" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                                <span class="text-xs font-medium text-blue-800 dark:text-blue-200">Monitor VM/LXC Status</span>
                                            </label>
                                            <p class="text-xs text-blue-600 dark:text-blue-300 mt-1 ml-6">Alert when VM/LXC stops or becomes unavailable</p>
                                        </div>
                                        
                                        <div id="threshold-preview" class="mt-3 p-2 bg-white dark:bg-gray-800 border border-blue-200 dark:border-blue-600 rounded text-xs">
                                            <div class="text-blue-700 dark:text-blue-300 font-medium mb-1">Active Thresholds:</div>
                                            <div id="threshold-preview-list" class="text-gray-600 dark:text-gray-400">No thresholds set</div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Duration (how long condition must persist)
                                </label>
                                <div class="flex gap-2">
                                    <input type="number" name="durationValue" min="1" max="60" step="1" value="5" required
                                           class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                    <select name="durationUnit" class="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                        <option value="seconds">seconds</option>
                                        <option value="minutes" selected>minutes</option>
                                    </select>
                                </div>
                                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                    Alert will only trigger if the condition persists for this duration. Prevents false alarms from brief spikes.
                                    Range: 5 seconds to 60 minutes.
                                </p>
                            </div>

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
${isEditing ? 'Update Alert' : 'Create Alert'}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHTML);
        
        // Pre-populate form if editing existing alert
        if (existingAlert) {
            setTimeout(() => {
                const targetTypeSelect = document.querySelector('select[name="targetType"]');
                const specificSection = document.getElementById('specific-target-section');
                const specificSelect = document.getElementById('specificTargetSelect');
                
                if (targetTypeSelect && existingAlert.targetType) {
                    targetTypeSelect.value = existingAlert.targetType;
                    
                    // Show and populate specific target section if needed
                    if ((existingAlert.targetType === 'specific' || existingAlert.targetType === 'node') && existingAlert.specificTarget) {
                        specificSection.classList.remove('hidden');
                        specificSelect.required = true;
                        
                        // Populate dropdown first
                        populateTargetDropdown(existingAlert.targetType);
                        
                        // Then set the selected value
                        setTimeout(() => {
                            specificSelect.value = existingAlert.specificTarget;
                        }, 10);
                    }
                }
            }, 10);
        }
        
        setupCustomAlertModalEvents(existingAlert, presetThresholds);
    }

    // Function to populate target dropdown with VM/LXC options
    function populateTargetDropdown(targetType) {
        const specificSelect = document.getElementById('specificTargetSelect');
        if (!specificSelect) return;

        // Clear existing options except the first one
        while (specificSelect.children.length > 1) {
            specificSelect.removeChild(specificSelect.lastChild);
        }

        if (targetType === 'node') {
            // For node selection, get unique node names
            const nodesData = PulseApp.state.get('nodesData') || [];
            const uniqueNodes = [...new Set(nodesData.map(node => node.node))];
            
            uniqueNodes.forEach(nodeName => {
                const option = document.createElement('option');
                option.value = nodeName;
                option.textContent = nodeName;
                specificSelect.appendChild(option);
            });
        } else if (targetType === 'specific') {
            // For specific VM/LXC selection, get all VMs and containers
            const dashboardData = PulseApp.state.get('dashboardData') || [];
            
            // Sort by type first (VMs then LXCs), then by vmid
            const sortedData = dashboardData.sort((a, b) => {
                if (a.type !== b.type) {
                    return a.type === 'qemu' ? -1 : 1; // VMs first
                }
                return a.vmid - b.vmid;
            });

            sortedData.forEach(item => {
                const option = document.createElement('option');
                // Use vmid as value since that's what the backend expects
                option.value = item.vmid;
                
                // Create descriptive label: "VM 100: web-server (node1)" or "CT 101: database (node2)"
                const typeLabel = item.type === 'qemu' ? 'VM' : 'CT';
                const nameText = item.name || `${item.type}-${item.vmid}`;
                const nodeText = item.nodeDisplayName || item.node || '';
                
                option.textContent = `${typeLabel} ${item.vmid}: ${nameText}${nodeText ? ` (${nodeText})` : ''}`;
                specificSelect.appendChild(option);
            });
        }
    }

    function setupCustomAlertModalEvents(existingAlert = null, presetThresholds = null) {
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
                const specificSelect = document.getElementById('specificTargetSelect');
                
                if (e.target.value === 'specific' || e.target.value === 'node') {
                    specificSection.classList.remove('hidden');
                    specificSelect.required = true;
                    
                    // Populate dropdown based on target type
                    populateTargetDropdown(e.target.value);
                } else {
                    specificSection.classList.add('hidden');
                    specificSelect.required = false;
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


        // Handle multiple threshold slider inputs
        const rangeSliders = document.querySelectorAll('.range-slider');
        rangeSliders.forEach(slider => {
            slider.addEventListener('input', (e) => {
                updateSliderValueDisplay(e.target);
                updateThresholdPreview();
            });
        });

        // Handle threshold select inputs and status monitoring checkbox  
        const thresholdSelects = document.querySelectorAll('select[name$="Threshold"]');
        thresholdSelects.forEach(select => {
            select.addEventListener('change', () => {
                updateThresholdPreview();
            });
        });
        
        // Handle status monitoring checkbox
        const statusCheckbox = document.querySelector('input[name="statusMonitoring"]');
        if (statusCheckbox) {
            statusCheckbox.addEventListener('change', () => {
                updateThresholdPreview();
            });
        }

        // Initialize slider value displays
        rangeSliders.forEach(slider => {
            updateSliderValueDisplay(slider);
        });
        
        // Pre-populate thresholds (handle both preset thresholds from dashboard and existing alerts)
        const thresholdsToPopulate = existingAlert ? existingAlert.thresholds : presetThresholds;
        
        if (thresholdsToPopulate && thresholdsToPopulate.length > 0) {
            thresholdsToPopulate.forEach(threshold => {
                // Support both old format (type/value) and new format (metric/threshold)
                const metric = threshold.metric || threshold.type;
                const value = threshold.threshold || threshold.value;
                
                if (metric === 'cpu' && value > 0) {
                    const cpuSlider = document.querySelector('input[name="cpuThreshold"]');
                    if (cpuSlider) {
                        cpuSlider.value = value;
                        updateSliderValueDisplay(cpuSlider);
                    }
                } else if (metric === 'memory' && value > 0) {
                    const memorySlider = document.querySelector('input[name="memoryThreshold"]');
                    if (memorySlider) {
                        memorySlider.value = value;
                        updateSliderValueDisplay(memorySlider);
                    }
                } else if (metric === 'disk' && value > 0) {
                    const diskSlider = document.querySelector('input[name="diskThreshold"]');
                    if (diskSlider) {
                        diskSlider.value = value;
                        updateSliderValueDisplay(diskSlider);
                    }
                } else if (metric === 'network' && value > 0) {
                    const networkSelect = document.querySelector('select[name="networkThreshold"]');
                    if (networkSelect) {
                        networkSelect.value = value;
                    }
                } else if (metric === 'status') {
                    const statusCheckbox = document.querySelector('input[name="statusMonitoring"]');
                    if (statusCheckbox) {
                        statusCheckbox.checked = true;
                    }
                }
            });
        }
        
        // Initialize threshold preview
        updateThresholdPreview();

        // Pre-populate notification checkboxes for existing alerts
        if (existingAlert) {
            const emailCheckbox = document.querySelector('input[name="sendEmail"]');
            const webhookCheckbox = document.querySelector('input[name="sendWebhook"]');
            
            if (emailCheckbox && typeof existingAlert.sendEmail === 'boolean') {
                emailCheckbox.checked = existingAlert.sendEmail;
            }
            
            if (webhookCheckbox && typeof existingAlert.sendWebhook === 'boolean') {
                webhookCheckbox.checked = existingAlert.sendWebhook;
            }
            
            // Pre-populate duration fields for existing alerts
            if (existingAlert.duration) {
                const durationValueInput = document.querySelector('input[name="durationValue"]');
                const durationUnitSelect = document.querySelector('select[name="durationUnit"]');
                
                if (durationValueInput && durationUnitSelect) {
                    // Convert duration from milliseconds to appropriate unit
                    const durationMs = existingAlert.duration;
                    if (durationMs >= 60000 && durationMs % 60000 === 0) {
                        // Use minutes if duration is in whole minutes
                        durationValueInput.value = durationMs / 60000;
                        durationUnitSelect.value = 'minutes';
                    } else {
                        // Use seconds
                        durationValueInput.value = durationMs / 1000;
                        durationUnitSelect.value = 'seconds';
                    }
                }
            }
        }

        // Handle duration unit changes to update input constraints
        const durationUnitSelect = document.querySelector('select[name="durationUnit"]');
        const durationValueInput = document.querySelector('input[name="durationValue"]');
        if (durationUnitSelect && durationValueInput) {
            durationUnitSelect.addEventListener('change', (e) => {
                if (e.target.value === 'minutes') {
                    durationValueInput.min = '1';
                    durationValueInput.max = '60';
                    durationValueInput.step = '1';
                    // Convert seconds to minutes if current value is in seconds range
                    const currentValue = parseInt(durationValueInput.value);
                    if (currentValue >= 60) {
                        durationValueInput.value = Math.round(currentValue / 60);
                    }
                } else {
                    durationValueInput.min = '5';
                    durationValueInput.max = '3600';
                    durationValueInput.step = '5';
                    // Convert minutes to seconds if current value is small
                    const currentValue = parseInt(durationValueInput.value);
                    if (currentValue <= 60) {
                        durationValueInput.value = currentValue * 60;
                    }
                }
            });
        }

        // Handle save
        if (saveBtn) {
            saveBtn.addEventListener('click', () => saveCustomAlert(existingAlert));
        }
    }

    async function saveCustomAlert(existingAlert = null) {
        const form = document.getElementById('custom-alert-form');
        const formData = new FormData(form);
        
        // Validate form
        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        // Check if we have multiple thresholds from the dashboard (preset mode)
        const multipleThresholdsData = formData.get('multipleThresholds');
        let thresholds = [];
        
        if (multipleThresholdsData) {
            // Parse the JSON data containing all the preset thresholds
            try {
                thresholds = JSON.parse(multipleThresholdsData.replace(/&quot;/g, '"'));
            } catch (error) {
                console.warn('Failed to parse multiple thresholds data:', error);
            }
        } else {
            // Collect thresholds from the simplified form
            const cpuThreshold = parseFloat(formData.get('cpuThreshold')) || 0;
            const memoryThreshold = parseFloat(formData.get('memoryThreshold')) || 0;
            const diskThreshold = parseFloat(formData.get('diskThreshold')) || 0;
            const networkThreshold = parseFloat(formData.get('networkThreshold')) || 0;
            const statusMonitoring = formData.has('statusMonitoring');
            
            // Build thresholds array (only include non-zero values and enabled options)
            if (cpuThreshold > 0) {
                thresholds.push({metric: 'cpu', condition: 'greater_than', threshold: cpuThreshold});
            }
            if (memoryThreshold > 0) {
                thresholds.push({metric: 'memory', condition: 'greater_than', threshold: memoryThreshold});
            }
            if (diskThreshold > 0) {
                thresholds.push({metric: 'disk', condition: 'greater_than', threshold: diskThreshold});
            }
            if (networkThreshold > 0) {
                thresholds.push({metric: 'network', condition: 'greater_than', threshold: networkThreshold});
            }
            if (statusMonitoring) {
                thresholds.push({metric: 'status', condition: 'equals', threshold: 'stopped'});
            }
            
        }

        // Validate that we have at least one threshold
        if (thresholds.length === 0) {
            PulseApp.ui.toast.warning('Please set at least one threshold to create an alert.');
            return;
        }

        // Collect duration and convert to milliseconds
        const durationValue = parseInt(formData.get('durationValue')) || 5; // default 5 minutes
        const durationUnit = formData.get('durationUnit') || 'minutes';
        const durationMs = durationUnit === 'minutes' ? durationValue * 60 * 1000 : durationValue * 1000;
        
        // Validate duration range (5 seconds to 60 minutes)
        if (durationMs < 5000 || durationMs > 3600000) {
            PulseApp.ui.toast.warning('Duration must be between 5 seconds and 60 minutes.');
            return;
        }

        // Build alert configuration
        const alertConfig = {
            name: formData.get('alertName'),
            targetType: formData.get('targetType'),
            specificTarget: formData.get('specificTarget'),
            thresholds: thresholds, // Use array of thresholds instead of single values
            duration: durationMs, // Duration in milliseconds
            sendEmail: formData.has('sendEmail'),
            sendWebhook: formData.has('sendWebhook'),
            enabled: existingAlert ? existingAlert.enabled : true,
            createdAt: existingAlert ? existingAlert.createdAt : Date.now()
        };

        // If editing, preserve the existing alert ID
        if (existingAlert) {
            alertConfig.id = existingAlert.id;
        }


        try {
            // Save to backend via API
            const url = existingAlert ? `/api/alerts/rules/${existingAlert.id}` : '/api/alerts/rules';
            const method = existingAlert ? 'PUT' : 'POST';
            
            const response = await fetch(url, {
                method: method,
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
            
            const thresholdSummary = thresholds.map(t => `${t.metric.toUpperCase()}: ${t.threshold}${['cpu', 'memory', 'disk'].includes(t.metric) ? '%' : ''}`).join(', ');
            PulseApp.ui.toast.success(`Custom alert ${existingAlert ? 'updated' : 'created'} successfully! Alert: ${alertConfig.name}`);
            
            // Close modal
            document.getElementById('custom-alert-modal').remove();
            
            // Refresh the custom alerts list
            await loadCustomAlerts();
            
        } catch (error) {
            console.error('Failed to save custom alert:', error);
            PulseApp.ui.toast.error(`Failed to create custom alert: ${error.message}`);
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
        } else if (typeof alert.currentValue === 'object' && alert.currentValue !== null) {
            // Handle compound threshold alerts (multiple metrics)
            const values = [];
            for (const [metric, value] of Object.entries(alert.currentValue)) {
                const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(metric);
                const formattedValue = typeof value === 'number' 
                    ? `${Math.round(value)}${isPercentageMetric ? '%' : ''}`
                    : value;
                values.push(`${metric}: ${formattedValue}`);
            }
            currentValueDisplay = values.join(', ');
        } else {
            currentValueDisplay = alert.currentValue || '';
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

        // Enhanced information display
        const alertType = alert.rule?.metric === 'compound' || alert.rule?.type === 'compound_threshold' ? 'Compound Alert' : 
                         alert.rule?.metric ? `${alert.rule.metric.toUpperCase()} Alert` : 'System Alert';
        
        const severityBadgeClass = alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200' :
                                  alert.severity === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200' :
                                  'bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200';
        
        const thresholdInfo = (() => {
            if (alert.rule?.metric === 'compound' || alert.rule?.type === 'compound_threshold') {
                if (alert.rule?.thresholds && Array.isArray(alert.rule.thresholds)) {
                    const thresholds = alert.rule.thresholds.map(t => {
                        const isPercentage = ['cpu', 'memory', 'disk'].includes(t.metric);
                        return `${t.metric.toUpperCase()} ≥ ${t.threshold}${isPercentage ? '%' : ''}`;
                    });
                    return thresholds.join(' AND ');
                }
                return 'Multiple thresholds';
            } else {
                const threshold = alert.effectiveThreshold !== undefined ? alert.effectiveThreshold : alert.rule?.threshold;
                if (threshold !== undefined && threshold !== null) {
                    const metric = alert.rule?.metric;
                    const isPercentage = ['cpu', 'memory', 'disk'].includes(metric);
                    return `Threshold: ${threshold}${isPercentage ? '%' : ''}`;
                }
                return '';
            }
        })();

        return `
            <div class="border-l-4 ${colorClass} p-4 rounded-r-lg ${acknowledgedClass}">
                <div class="flex items-start justify-between">
                    <div class="flex-1">
                        <div class="flex items-center space-x-2 mb-2">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${alert.guest?.name || 'Unknown'}
                            </h5>
                            <span class="text-xs px-2 py-1 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">
                                ${alert.guest?.type || 'unknown'} ${alert.guest?.vmid || ''}
                            </span>
                            <span class="text-xs px-2 py-1 rounded-full ${severityBadgeClass}">
                                ${alert.severity || 'info'}
                            </span>
                            ${alert.escalated ? '<span class="text-xs bg-red-500 text-white px-2 py-1 rounded">Escalated</span>' : ''}
                            ${acknowledged ? '<span class="text-xs bg-green-500 text-white px-2 py-1 rounded">Acknowledged</span>' : ''}
                        </div>
                        ${deliveryIndicators}
                        <div class="space-y-1">
                            <p class="text-sm text-gray-900 dark:text-gray-100 font-medium">
                                ${alertType}: ${currentValueDisplay}
                            </p>
                            ${thresholdInfo ? `<p class="text-xs text-gray-600 dark:text-gray-400">${thresholdInfo}</p>` : ''}
                            <p class="text-xs text-gray-500 dark:text-gray-500">
                                ${alert.guest?.node ? `Node: ${alert.guest.node} • ` : ''}Active for ${durationStr}
                                ${acknowledged ? ` • Acknowledged ${Math.round((Date.now() - alert.acknowledgedAt) / 60000)}m ago` : ''}
                            </p>
                        </div>
                    </div>
                    <div class="flex space-x-2">
                        ${!acknowledged ? `
                            <button onclick="PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}'); setTimeout(() => PulseApp.ui.alertManagementModal.loadCurrentAlerts(), 500);" 
                                    class="px-3 py-1 bg-green-500 hover:bg-green-600 text-white text-xs font-medium rounded transition-colors">
                                Acknowledge
                            </button>
                        ` : ''}
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
            
            // Hide save button initially (only show on notifications tab)
            const saveButton = document.getElementById('alert-management-save-button');
            if (saveButton) {
                saveButton.style.display = 'none';
            }
            
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
            
            // Always load configuration directly from API to ensure fresh data
            const response = await fetch('/api/config');
            if (response.ok) {
                currentConfig = await response.json();
            } else {
                console.error('[DEBUG] Failed to load configuration - response not ok');
                currentConfig = {};
            }
            
        } catch (error) {
            console.error('[DEBUG] Error loading configuration:', error);
            currentConfig = {};
        }
    }

    async function saveConfiguration() {
        // The save configuration button in the alert management modal should 
        // save any pending changes. Since individual system alerts are edited 
        // in their own modals and saved immediately, and custom alerts have 
        // their own save buttons, this function mainly handles email/webhook 
        // configuration that might be pending.
        
        const saveButton = document.getElementById('alert-management-save-button');
        if (!saveButton) return;

        const originalText = saveButton.textContent;
        saveButton.disabled = true;
        saveButton.textContent = 'Saving...';

        try {
            let configToSave = {};
            let hasChanges = false;

            // Check for email configuration changes
            const emailConfigSection = document.getElementById('email-config-section');
            if (emailConfigSection) {
                const smtpHost = document.querySelector('input[name="ALERT_SMTP_HOST"]')?.value;
                const smtpPort = document.querySelector('input[name="ALERT_SMTP_PORT"]')?.value;
                const smtpUser = document.querySelector('input[name="ALERT_SMTP_USER"]')?.value;
                const smtpPass = document.querySelector('input[name="ALERT_EMAIL_PASSWORD"]')?.value;
                const fromEmail = document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value;
                const toEmail = document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value;
                const smtpSecure = document.querySelector('input[name="ALERT_SMTP_SECURE"]')?.checked;

                if (smtpHost) { configToSave.SMTP_HOST = smtpHost; hasChanges = true; }
                if (smtpPort) { configToSave.SMTP_PORT = smtpPort; hasChanges = true; }
                if (smtpUser) { configToSave.SMTP_USER = smtpUser; hasChanges = true; }
                if (smtpPass) { configToSave.SMTP_PASS = smtpPass; hasChanges = true; }
                if (fromEmail) { configToSave.ALERT_FROM_EMAIL = fromEmail; hasChanges = true; }
                if (toEmail) { configToSave.ALERT_TO_EMAIL = toEmail; hasChanges = true; }
                configToSave.SMTP_SECURE = smtpSecure ? 'true' : 'false';
                hasChanges = true;
            }

            // Check for webhook configuration changes
            const webhookForm = document.getElementById('webhook-config-form');
            if (webhookForm) {
                const webhookUrl = webhookForm.querySelector('input[name="webhook_url"]')?.value;
                const webhookEnabled = webhookForm.querySelector('input[name="webhook_enabled"]')?.checked;

                if (webhookUrl !== undefined) { configToSave.WEBHOOK_URL = webhookUrl; hasChanges = true; }
                configToSave.WEBHOOK_ENABLED = webhookEnabled ? 'true' : 'false';
                hasChanges = true;
            }

            if (!hasChanges) {
                PulseApp.ui.toast.alert('No changes to save');
                return;
            }

            // Save configuration via API
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(configToSave)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast.success('Configuration saved successfully');
                await loadConfiguration(); // Reload to reflect changes
            } else {
                throw new Error(result.error || 'Failed to save configuration');
            }

        } catch (error) {
            console.error('Error saving configuration:', error);
            PulseApp.ui.toast.error('Failed to save configuration: ' + error.message);
        } finally {
            saveButton.disabled = false;
            saveButton.textContent = originalText;
        }
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

            await loadCustomAlerts(); // Refresh display
            
        } catch (error) {
            console.error('Failed to toggle custom alert:', error);
            PulseApp.ui.toast.error(`Failed to ${enabled ? 'enable' : 'disable'} alert: ${error.message}`);
        }
    }

    async function editCustomAlert(alertId) {
        try {
            // First, fetch the current alert data
            const response = await fetch('/api/alerts/rules');
            if (!response.ok) {
                throw new Error(`Failed to fetch alert rules: ${response.status}`);
            }
            
            const data = await response.json();
            const alert = data.rules ? data.rules.find(rule => rule.id === alertId) : null;
            
            if (!alert) {
                throw new Error('Alert not found');
            }
            
            
            // Open the custom alert modal with pre-filled data
            openCustomAlertModal(alert.thresholds || [], alert);
            
        } catch (error) {
            console.error('Failed to load alert for editing:', error);
            PulseApp.ui.toast.error(`Failed to load alert for editing: ${error.message}`);
        }
    }

    async function deleteCustomAlert(alertId) {
        await _performDeleteCustomAlert(alertId);
    }

    function showAlertDetails(alertId) {
        // Find the alert in the current alerts list
        const alertsResponse = document.querySelector('#alert-management-modal-body');
        if (!alertsResponse) return;

        // Get the alert data from the current state
        try {
            const alerts = PulseApp.alerts.getActiveAlerts();
            const alert = alerts.find(a => a.id === alertId);
            
            if (alert) {
                displayAlertDetailsModal(alert);
            } else {
                // Fallback: try to get alert from server
                fetch(`/api/alerts`)
                    .then(response => response.json())
                    .then(data => {
                        const alert = data.active ? data.active.find(a => a.id === alertId) : null;
                        if (alert) {
                            displayAlertDetailsModal(alert);
                        } else {
                            PulseApp.ui.toast.error('Alert not found');
                        }
                    })
                    .catch(error => {
                        console.error('Failed to fetch alert details:', error);
                        PulseApp.ui.toast.error('Failed to load alert details');
                    });
            }
        } catch (error) {
            console.error('Error accessing alerts:', error);
            PulseApp.ui.toast.error('Failed to access alert data');
        }
    }

    function displayAlertDetailsModal(alert) {
        // Create alert details modal
        const modalHTML = `
            <div id="alert-details-modal" class="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50" style="z-index: 9999;">
                <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[80vh] flex flex-col m-4">
                    <div class="flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-6 py-4">
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Alert Details</h2>
                        <button onclick="document.getElementById('alert-details-modal').remove();" 
                                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    
                    <div class="overflow-y-auto flex-grow p-6">
                        <div class="space-y-4">
                            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                                <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Alert Information</h3>
                                <div class="grid grid-cols-2 gap-4 text-sm">
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Rule:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.rule?.name || alert.ruleName || 'Unknown'}</span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Severity:</span>
                                        <span class="ml-2 px-2 py-1 rounded text-xs font-medium 
                                            ${alert.severity === 'critical' ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' :
                                              alert.severity === 'warning' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' :
                                              'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'}">
                                            ${alert.severity || 'info'}
                                        </span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Status:</span>
                                        <span class="ml-2 px-2 py-1 rounded text-xs font-medium 
                                            ${alert.acknowledged ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' :
                                              'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'}">
                                            ${alert.acknowledged ? 'Acknowledged' : 'Active'}
                                        </span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Duration:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">
                                            ${alert.triggeredAt ? Math.round((Date.now() - alert.triggeredAt) / 60000) + ' minutes' : 'Unknown'}
                                        </span>
                                    </div>
                                </div>
                            </div>

                            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                                <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Affected Resource</h3>
                                <div class="grid grid-cols-2 gap-4 text-sm">
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Name:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.guest?.name || 'Unknown'}</span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Type:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.guest?.type || 'Unknown'}</span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Node:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.guest?.node || 'Unknown'}</span>
                                    </div>
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">VM/CT ID:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.guest?.vmid || 'Unknown'}</span>
                                    </div>
                                </div>
                            </div>

                            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                                <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Metric Details</h3>
                                <div class="space-y-2 text-sm">
                                    <div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">Alert Type:</span>
                                        <span class="text-gray-900 dark:text-gray-100 ml-2">
                                            ${(() => {
                                                const metric = alert.rule?.metric || alert.metric;
                                                if (metric === 'compound' || alert.rule?.type === 'compound_threshold') {
                                                    return 'Compound Threshold Alert';
                                                }
                                                return metric ? metric.charAt(0).toUpperCase() + metric.slice(1) + ' Alert' : 'Single Metric Alert';
                                            })()}
                                        </span>
                                    </div>
                                    ${(() => {
                                        const metric = alert.rule?.metric || alert.metric;
                                        if (metric === 'compound' || alert.rule?.type === 'compound_threshold') {
                                            // Handle compound threshold alerts
                                            let currentValuesHtml = '';
                                            let thresholdsHtml = '';
                                            
                                            if (typeof alert.currentValue === 'object' && alert.currentValue !== null) {
                                                const values = Object.entries(alert.currentValue).map(([key, value]) => {
                                                    const isPercentage = ['cpu', 'memory', 'disk'].includes(key);
                                                    return key.toUpperCase() + ': ' + (typeof value === 'number' ? Math.round(value) : value) + (isPercentage ? '%' : '');
                                                });
                                                currentValuesHtml = values.join(', ');
                                            }
                                            
                                            if (alert.rule?.thresholds && Array.isArray(alert.rule.thresholds)) {
                                                const thresholds = alert.rule.thresholds.map(t => {
                                                    const isPercentage = ['cpu', 'memory', 'disk'].includes(t.metric);
                                                    return t.metric.toUpperCase() + ' ' + t.condition.replace('_', ' ') + ' ' + t.threshold + (isPercentage ? '%' : '');
                                                });
                                                thresholdsHtml = thresholds.join(' AND ');
                                            } else if (typeof alert.effectiveThreshold === 'object' && alert.effectiveThreshold !== null) {
                                                thresholdsHtml = JSON.stringify(alert.effectiveThreshold);
                                            }
                                            
                                            return '<div>' +
                                                '<span class="font-medium text-gray-700 dark:text-gray-300">Current Values:</span>' +
                                                '<span class="text-gray-900 dark:text-gray-100 ml-2">' + (currentValuesHtml || 'N/A') + '</span>' +
                                                '</div>' +
                                                '<div>' +
                                                '<span class="font-medium text-gray-700 dark:text-gray-300">Threshold Conditions:</span>' +
                                                '<span class="text-gray-900 dark:text-gray-100 ml-2">' + (thresholdsHtml || 'N/A') + '</span>' +
                                                '</div>';
                                        } else {
                                            // Handle single metric alerts
                                            const currentValue = typeof alert.currentValue === 'number' ? 
                                                (['cpu', 'memory', 'disk'].includes(metric) ? Math.round(alert.currentValue) + '%' : alert.currentValue) : 
                                                (alert.currentValue || 'N/A');
                                            
                                            const threshold = alert.effectiveThreshold !== undefined ? alert.effectiveThreshold : alert.rule?.threshold || alert.threshold;
                                            const thresholdDisplay = threshold !== undefined && threshold !== null ? 
                                                (typeof threshold === 'number' && ['cpu', 'memory', 'disk'].includes(metric) ? threshold + '%' : threshold) : 
                                                'N/A';
                                            
                                            return '<div>' +
                                                '<span class="font-medium text-gray-700 dark:text-gray-300">Metric:</span>' +
                                                '<span class="text-gray-900 dark:text-gray-100 ml-2">' + (metric || 'Unknown') + '</span>' +
                                                '</div>' +
                                                '<div>' +
                                                '<span class="font-medium text-gray-700 dark:text-gray-300">Current Value:</span>' +
                                                '<span class="text-gray-900 dark:text-gray-100 ml-2">' + currentValue + '</span>' +
                                                '</div>' +
                                                '<div>' +
                                                '<span class="font-medium text-gray-700 dark:text-gray-300">Threshold:</span>' +
                                                '<span class="text-gray-900 dark:text-gray-100 ml-2">' + thresholdDisplay + '</span>' +
                                                '</div>';
                                        }
                                    })()}
                                    ${alert.rule?.description ? `
                                        <div>
                                            <span class="font-medium text-gray-700 dark:text-gray-300">Description:</span>
                                            <p class="text-gray-900 dark:text-gray-100 mt-1">${alert.rule.description}</p>
                                        </div>
                                    ` : ''}
                                </div>
                            </div>

                            ${alert.acknowledged ? `
                                <div class="bg-green-50 dark:bg-green-900/20 rounded-lg p-4">
                                    <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Acknowledgment</h3>
                                    <div class="text-sm">
                                        <div>
                                            <span class="font-medium text-gray-700 dark:text-gray-300">Acknowledged by:</span>
                                            <span class="text-gray-900 dark:text-gray-100 ml-2">${alert.acknowledgedBy || 'System'}</span>
                                        </div>
                                        <div>
                                            <span class="font-medium text-gray-700 dark:text-gray-300">Time:</span>
                                            <span class="text-gray-900 dark:text-gray-100 ml-2">
                                                ${alert.acknowledgedAt ? new Date(alert.acknowledgedAt).toLocaleString() : 'Unknown'}
                                            </span>
                                        </div>
                                        ${alert.acknowledgeNote ? `
                                            <div>
                                                <span class="font-medium text-gray-700 dark:text-gray-300">Note:</span>
                                                <p class="text-gray-900 dark:text-gray-100 mt-1">${alert.acknowledgeNote}</p>
                                            </div>
                                        ` : ''}
                                    </div>
                                </div>
                            ` : ''}
                        </div>
                    </div>
                    
                    <div class="border-t border-gray-300 dark:border-gray-700 px-6 py-4">
                        <div class="flex gap-3 justify-end">
                            ${!alert.acknowledged ? `
                                <button onclick="PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}'); setTimeout(() => { document.getElementById('alert-details-modal').remove(); PulseApp.ui.alertManagementModal.loadCurrentAlerts(); }, 500);" 
                                        class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                                    Acknowledge Alert
                                </button>
                            ` : ''}
                            <button onclick="document.getElementById('alert-details-modal').remove();" 
                                    class="px-4 py-2 bg-gray-500 hover:bg-gray-600 text-white text-sm font-medium rounded-md transition-colors">
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        // Remove any existing details modal
        const existingModal = document.getElementById('alert-details-modal');
        if (existingModal) {
            existingModal.remove();
        }

        // Add the modal to the page
        document.body.insertAdjacentHTML('beforeend', modalHTML);
    }

    async function _performDeleteCustomAlert(alertId) {
        try {
            const response = await fetch(`/api/alerts/rules/${alertId}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `Server error: ${response.status}`);
            }

            PulseApp.ui.toast.success('Custom alert deleted successfully');
            await loadCustomAlerts(); // Refresh display
            
        } catch (error) {
            console.error('Failed to delete custom alert:', error);
            PulseApp.ui.toast.error(`Failed to delete alert: ${error.message}`);
        }
    }


    // Global functions that need to be accessible from HTML onclick handlers
    window.editSystemAlert = editSystemAlert;
    window.closeSystemAlertModal = closeSystemAlertModal;
    window.saveSystemAlert = saveSystemAlert;
    
    window.toggleSystemAlert = function(alertId, enabled) {
        
        // Update the status badge
        const statusBadge = document.getElementById(`${alertId}-status-badge`);
        if (statusBadge) {
            statusBadge.textContent = enabled ? 'Enabled' : 'Disabled';
            statusBadge.className = `text-xs px-2 py-0.5 rounded-full ${
                enabled 
                    ? 'bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300' 
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
            }`;
        }
        
        // Update the toggle button
        const toggleBtn = document.getElementById(`${alertId}-toggle-btn`);
        if (toggleBtn) {
            toggleBtn.textContent = enabled ? 'Disable' : 'Enable';
            toggleBtn.onclick = () => toggleSystemAlert(alertId, !enabled);
        }
        
        // Save the configuration to backend
        updateSystemAlertStatus(alertId, enabled);
    };

    // Helper function to update slider value displays
    function updateSliderValueDisplay(slider) {
        const value = slider.value;
        const name = slider.name;
        
        // Find the corresponding value display element
        let displayElement;
        if (name === 'cpuThreshold') {
            displayElement = document.getElementById('cpu-threshold-value');
        } else if (name === 'memoryThreshold') {
            displayElement = document.getElementById('memory-threshold-value');
        } else if (name === 'diskThreshold') {
            displayElement = document.getElementById('disk-threshold-value');
        }
        
        if (displayElement) {
            if (value > 0) {
                displayElement.textContent = `${value}%`;
                displayElement.classList.add('font-bold');
            } else {
                displayElement.textContent = '0%';
                displayElement.classList.remove('font-bold');
            }
        }
    }

    // Helper function to update the threshold preview
    function updateThresholdPreview() {
        const previewList = document.getElementById('threshold-preview-list');
        if (!previewList) return;
        
        const thresholds = [];
        
        // Check all threshold inputs
        const cpuValue = parseInt(document.querySelector('input[name="cpuThreshold"]')?.value) || 0;
        const memoryValue = parseInt(document.querySelector('input[name="memoryThreshold"]')?.value) || 0;
        const diskValue = parseInt(document.querySelector('input[name="diskThreshold"]')?.value) || 0;
        const networkValue = parseInt(document.querySelector('select[name="networkThreshold"]')?.value) || 0;
        const statusMonitoring = document.querySelector('input[name="statusMonitoring"]')?.checked || false;
        
        // Add active thresholds to the list
        if (cpuValue > 0) thresholds.push(`CPU ≥ ${cpuValue}%`);
        if (memoryValue > 0) thresholds.push(`Memory ≥ ${memoryValue}%`);
        if (diskValue > 0) thresholds.push(`Disk ≥ ${diskValue}%`);
        if (networkValue > 0) thresholds.push(`Network ≥ ${_formatBytesForDisplay(networkValue)}`);
        if (statusMonitoring) thresholds.push(`VM/LXC Status Changes`);
        
        // Update the preview display
        if (thresholds.length > 0) {
            previewList.innerHTML = thresholds.join(' • ');
            previewList.classList.remove('text-gray-600', 'dark:text-gray-400');
            previewList.classList.add('text-blue-700', 'dark:text-blue-300', 'font-medium');
        } else {
            previewList.textContent = 'No thresholds set';
            previewList.classList.remove('text-blue-700', 'dark:text-blue-300', 'font-medium');
            previewList.classList.add('text-gray-600', 'dark:text-gray-400');
        }
    }

    // Helper function to format bytes for display
    function _formatBytesForDisplay(bytes) {
        const mb = bytes / (1024 * 1024);
        if (mb >= 100) return `${Math.round(mb)}MB/s`;
        if (mb >= 10) return `${Math.round(mb)}MB/s`;
        return `${Math.round(mb * 10) / 10}MB/s`;
    }

    // Public API
    return {
        init,
        openModal,
        closeModal,
        switchTab: switchTab,
        openCustomAlertModal,
        editCustomAlert,
        toggleCustomAlert,
        deleteCustomAlert,
        showAlertDetails,
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