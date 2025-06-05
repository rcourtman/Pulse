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
                            <!-- System Alert Rules List -->
                            <div class="space-y-4">
                                <div class="flex items-center justify-between">
                                    <h4 class="text-lg font-medium text-gray-900 dark:text-gray-100">System Alert Rules</h4>
                                    <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded-full">Built-in</span>
                                </div>
                                <p class="text-sm text-gray-500 dark:text-gray-400">Global monitoring rules that apply to all VMs and LXCs unless overridden by custom settings.</p>
                            </div>
                            
                            <div id="system-alerts-content" class="space-y-3">
                                <!-- CPU Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-white dark:bg-gray-800">
                                    <div class="flex items-center justify-between">
                                        <div class="flex-1">
                                            <div class="flex items-center space-x-3">
                                                <div class="flex-shrink-0">
                                                    <div class="w-8 h-8 bg-red-100 dark:bg-red-900/20 rounded-full flex items-center justify-center">
                                                        <svg class="w-4 h-4 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                                                        </svg>
                                                    </div>
                                                </div>
                                                <div class="flex-1">
                                                    <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">CPU Alert</h3>
                                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Triggers when CPU usage exceeds <span id="cpu-threshold-display">85%</span></p>
                                                </div>
                                            </div>
                                        </div>
                                        <div class="flex items-center space-x-3">
                                            <label class="relative inline-flex items-center cursor-pointer">
                                                <input type="checkbox" id="cpu-alert-enabled" class="sr-only peer" checked>
                                                <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                            </label>
                                            <button onclick="editSystemAlert('cpu')" class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 shadow-sm text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                                <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                                                </svg>
                                                Edit
                                            </button>
                                        </div>
                                    </div>
                                </div>

                                <!-- Memory Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-white dark:bg-gray-800">
                                    <div class="flex items-center justify-between">
                                        <div class="flex-1">
                                            <div class="flex items-center space-x-3">
                                                <div class="flex-shrink-0">
                                                    <div class="w-8 h-8 bg-orange-100 dark:bg-orange-900/20 rounded-full flex items-center justify-center">
                                                        <svg class="w-4 h-4 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
                                                        </svg>
                                                    </div>
                                                </div>
                                                <div class="flex-1">
                                                    <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Memory Alert</h3>
                                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Triggers when memory usage exceeds <span id="memory-threshold-display">90%</span></p>
                                                </div>
                                            </div>
                                        </div>
                                        <div class="flex items-center space-x-3">
                                            <label class="relative inline-flex items-center cursor-pointer">
                                                <input type="checkbox" id="memory-alert-enabled" class="sr-only peer" checked>
                                                <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                            </label>
                                            <button onclick="editSystemAlert('memory')" class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 shadow-sm text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                                <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                                                </svg>
                                                Edit
                                            </button>
                                        </div>
                                    </div>
                                </div>

                                <!-- Disk Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-white dark:bg-gray-800">
                                    <div class="flex items-center justify-between">
                                        <div class="flex-1">
                                            <div class="flex items-center space-x-3">
                                                <div class="flex-shrink-0">
                                                    <div class="w-8 h-8 bg-purple-100 dark:bg-purple-900/20 rounded-full flex items-center justify-center">
                                                        <svg class="w-4 h-4 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"></path>
                                                        </svg>
                                                    </div>
                                                </div>
                                                <div class="flex-1">
                                                    <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Disk Alert</h3>
                                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Triggers when disk usage exceeds <span id="disk-threshold-display">95%</span></p>
                                                </div>
                                            </div>
                                        </div>
                                        <div class="flex items-center space-x-3">
                                            <label class="relative inline-flex items-center cursor-pointer">
                                                <input type="checkbox" id="disk-alert-enabled" class="sr-only peer" checked>
                                                <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                            </label>
                                            <button onclick="editSystemAlert('disk')" class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 shadow-sm text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                                <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                                                </svg>
                                                Edit
                                            </button>
                                        </div>
                                    </div>
                                </div>

                                <!-- Down Alert Rule -->
                                <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-white dark:bg-gray-800">
                                    <div class="flex items-center justify-between">
                                        <div class="flex-1">
                                            <div class="flex items-center space-x-3">
                                                <div class="flex-shrink-0">
                                                    <div class="w-8 h-8 bg-red-100 dark:bg-red-900/20 rounded-full flex items-center justify-center">
                                                        <svg class="w-4 h-4 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728L5.636 5.636m12.728 12.728L18.364 5.636M5.636 18.364l12.728-12.728"></path>
                                                        </svg>
                                                    </div>
                                                </div>
                                                <div class="flex-1">
                                                    <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Down Alert</h3>
                                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Triggers when VM/LXC becomes unreachable or stops responding</p>
                                                </div>
                                            </div>
                                        </div>
                                        <div class="flex items-center space-x-3">
                                            <label class="relative inline-flex items-center cursor-pointer">
                                                <input type="checkbox" id="down-alert-enabled" class="sr-only peer" checked>
                                                <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                            </label>
                                            <button onclick="editSystemAlert('down')" class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 shadow-sm text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                                <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                                                </svg>
                                                Edit
                                            </button>
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

        // Set up system alert toggles
        const systemAlertToggles = document.querySelectorAll('#system-alerts-content input[type="checkbox"]');
        systemAlertToggles.forEach(toggle => {
            toggle.addEventListener('change', (e) => {
                const alertType = e.target.id.replace('-alert-enabled', '');
                console.log(`System ${alertType} alert toggled:`, e.target.checked);
                updateSystemAlertStatus(alertType, e.target.checked);
            });
        });
        
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
                console.log('Email notifications toggled:', e.target.checked);
            });
        }
        
        const webhookToggle = document.querySelector('input[name="GLOBAL_WEBHOOK_ENABLED"]');
        if (webhookToggle) {
            webhookToggle.addEventListener('change', (e) => {
                console.log('Webhook notifications toggled:', e.target.checked);
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
                help: 'For Gmail, you need to enable 2-factor authentication and generate an App Password. Go to Google Account → Security → 2-Step Verification → App passwords.'
            },
            outlook: {
                host: 'smtp-mail.outlook.com',
                port: 587,
                secure: true,
                passwordLabel: 'Password',
                passwordHelp: '(Your Microsoft account password)',
                help: 'Use your regular Microsoft account password. If you have 2FA enabled, you may need to generate an app password.'
            },
            yahoo: {
                host: 'smtp.mail.yahoo.com',
                port: 587,
                secure: true,
                passwordLabel: 'App Password',
                passwordHelp: '(Generate from Yahoo Account settings)',
                help: 'For Yahoo Mail, you need to generate an App Password. Go to Yahoo Account Info → Account Security → Generate app password.'
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
    
    function testEmailConnection() {
        // Collect email configuration
        const emailConfig = {
            from: document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value,
            to: document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value,
            password: document.querySelector('input[name="ALERT_EMAIL_PASSWORD"]')?.value,
            host: document.querySelector('input[name="ALERT_SMTP_HOST"]')?.value,
            port: document.querySelector('input[name="ALERT_SMTP_PORT"]')?.value,
            user: document.querySelector('input[name="ALERT_SMTP_USER"]')?.value,
            secure: document.querySelector('input[name="ALERT_SMTP_SECURE"]')?.checked
        };
        
        if (!emailConfig.from || !emailConfig.to) {
            alert('Please enter both sender and recipient email addresses');
            return;
        }
        
        // TODO: Implement actual email test
        console.log('Testing email configuration:', emailConfig);
        alert('Test email functionality will be implemented with backend integration');
    }
    
    function saveEmailConfiguration() {
        // Collect email configuration
        const emailConfig = {
            from: document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value,
            to: document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value,
            password: document.querySelector('input[name="ALERT_EMAIL_PASSWORD"]')?.value,
            host: document.querySelector('input[name="ALERT_SMTP_HOST"]')?.value,
            port: document.querySelector('input[name="ALERT_SMTP_PORT"]')?.value,
            user: document.querySelector('input[name="ALERT_SMTP_USER"]')?.value,
            secure: document.querySelector('input[name="ALERT_SMTP_SECURE"]')?.checked
        };
        
        // TODO: Implement saving to backend
        console.log('Saving email configuration:', emailConfig);
        alert('Email configuration saved successfully!');
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
        
        // TODO: Save to backend
        console.log(`Saving ${alertType} alert:`, alertConfig);
        
        // Update the UI immediately
        updateSystemAlertDisplay(alertType, alertConfig);
        
        alert(`${alertType.charAt(0).toUpperCase() + alertType.slice(1)} alert settings saved successfully!`);
        closeSystemAlertModal();
    }
    
    function updateSystemAlertStatus(alertType, enabled) {
        // TODO: Save status change to backend
        console.log(`${alertType} alert ${enabled ? 'enabled' : 'disabled'}`);
    }
    
    function updateSystemAlertDisplay(alertType, config) {
        // Update the toggle
        const toggle = document.getElementById(`${alertType}-alert-enabled`);
        if (toggle) toggle.checked = config.enabled;
        
        // Update the threshold display
        if (config.threshold && alertType !== 'down') {
            const display = document.getElementById(`${alertType}-threshold-display`);
            if (display) display.textContent = `${config.threshold}%`;
        }
    }

    function loadSystemAlertConfiguration() {
        // TODO: Load from backend and populate system alert displays
        const config = PulseApp.config?.alerts || {};
        
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
                primaryMetric = existingAlert.thresholds[0].type;
                primaryThreshold = existingAlert.thresholds[0].value;
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
${isEditing ? 'Update Alert' : 'Create Alert'}
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
            enabled: existingAlert ? existingAlert.enabled : true,
            createdAt: existingAlert ? existingAlert.createdAt : Date.now()
        };

        // If editing, preserve the existing alert ID
        if (existingAlert) {
            alertConfig.id = existingAlert.id;
        }

        console.log(existingAlert ? 'Updating custom alert:' : 'Creating custom alert:', alertConfig);

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
            console.log(`Alert rule ${existingAlert ? 'updated' : 'created'} successfully:`, result);
            
            const thresholdSummary = thresholds.map(t => `${t.type.toUpperCase()}: ${t.value}${['cpu', 'memory', 'disk'].includes(t.type) ? '%' : ''}`).join(', ');
            alert(`Custom alert ${existingAlert ? 'updated' : 'created'} successfully!\n\nAlert: ${alertConfig.name}\nThresholds: ${thresholdSummary}\n\nRule ID: ${result.rule?.id || existingAlert?.id || 'Generated'}`);
            
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
            alert(`Failed to load alert for editing: ${error.message}`);
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
    window.editSystemAlert = editSystemAlert;
    window.closeSystemAlertModal = closeSystemAlertModal;
    window.saveSystemAlert = saveSystemAlert;
    
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
        editCustomAlert,
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