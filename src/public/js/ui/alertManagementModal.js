PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.alertManagementModal = (() => {
    let isInitialized = false;
    let activeTab = 'alerts';
    let currentConfig = {};
    let formDataCache = {};
    let isLoading = false;
    let refreshInterval = null;

    function init() {
        if (isInitialized) return;
        
        // Create modal HTML
        createModalHTML();
        
        // Set up event listeners
        setupEventListeners();
        
        // Expose global functions after they are defined
        exposeGlobalFunctions();
        
        // Set up real-time updates for alert management modal
        if (window.socket) {
            // Auto-refresh when alerts change
            window.socket.on('alert', () => {
                try {
                    if (isModalOpen()) {
                        coordinatedRefresh();
                    }
                } catch (error) {
                    console.error('[Alert Modal] Error handling alert event:', error);
                }
            });
            
            window.socket.on('alertResolved', () => {
                try {
                    if (isModalOpen()) {
                        coordinatedRefresh();
                    }
                } catch (error) {
                    console.error('[Alert Modal] Error handling alertResolved event:', error);
                }
            });
            
            window.socket.on('alertAcknowledged', () => {
                try {
                    if (isModalOpen()) {
                        coordinatedRefresh();
                    }
                } catch (error) {
                    console.error('[Alert Modal] Error handling alertAcknowledged event:', error);
                }
            });
            
            window.socket.on('alertEscalated', () => {
                try {
                    if (isModalOpen()) {
                        coordinatedRefresh();
                    }
                } catch (error) {
                    console.error('[Alert Modal] Error handling alertEscalated event:', error);
                }
            });
        }

        // Set up periodic refresh to update durations every 2 seconds (for near real-time updates)
        refreshInterval = setInterval(() => {
            if (isModalOpen()) {
                coordinatedRefresh();
            }
        }, 2000);
        
        isInitialized = true;
    }
    

    function exposeGlobalFunctions() {
        // Make all onclick handler functions globally accessible immediately
        // This prevents timing issues where HTML is rendered before functions are exposed
        const functions = {
            deleteCustomAlert,
            openCustomAlertModal,
            showAlertDetails,
            toggleAlert,
            addWebhookEndpoint,
            removeWebhookEndpoint,
            handleEmailProviderSelection,
            switchTab,
            // Unified alert rule functions
            openAlertRuleModal,
            editAlertRule,
            toggleAlertRule,
            deleteAlertRule
        };
        
        Object.entries(functions).forEach(([name, func]) => {
            window[name] = func;
        });
    }

    function coordinatedRefresh() {
        // Prevent concurrent refreshes
        if (isLoading) return;
        
        isLoading = true;
        try {
            loadCurrentAlerts();
            loadSystemAlerts();
            loadCustomAlerts();
        } catch (error) {
            console.error('[Alert Modal] Error during coordinated refresh:', error);
        } finally {
            isLoading = false;
        }
    }


    function openModal() {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.remove('hidden');
            modal.classList.add('flex');
            // Default to monitor tab when opening
            switchTab('monitor');
            
            // Restart the refresh interval if it was cleaned up
            if (!refreshInterval) {
                refreshInterval = setInterval(() => {
                    if (isModalOpen()) {
                        coordinatedRefresh();
                    }
                }, 2000);
            }
        }
    }

    function cleanup() {
        // Clear the refresh interval to prevent memory leaks
        if (refreshInterval) {
            clearInterval(refreshInterval);
            refreshInterval = null;
        }
        
        // Remove socket event listeners to prevent memory leaks
        if (window.socket) {
            window.socket.off('alert');
            window.socket.off('alertResolved');
            window.socket.off('alertAcknowledged');
            window.socket.off('alertEscalated');
        }
        
        // Reset state
        isLoading = false;
    }

    function closeModal() {
        const modal = document.getElementById('alert-management-modal');
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
        
        // Clean up resources to prevent memory leaks
        cleanup();
    }

    function createModalHTML() {
        const existingModal = document.getElementById('alert-management-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modalHTML = `
            <div id="alert-management-modal" class="fixed inset-0 z-50 hidden items-start justify-center bg-black bg-opacity-50 pt-4 sm:pt-8">
                <div class="modal-content bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-6xl max-h-[95vh] sm:max-h-[90vh] flex flex-col m-2 sm:m-4">
                    <div class="modal-header flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-4 sm:px-6 py-3 sm:py-4">
                        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Alert Management</h2>
                        <button id="alert-management-modal-close" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    
                    <!-- Tab Navigation -->
                    <div class="border-b border-gray-200 dark:border-gray-700">
                        <nav class="flex space-x-4 sm:space-x-8 px-4 sm:px-6 overflow-x-auto" id="alert-management-tabs">
                            <button class="alert-tab active py-3 px-2 sm:px-1 border-b-2 border-blue-500 text-blue-600 dark:text-blue-400 font-medium text-sm whitespace-nowrap" data-tab="monitor">
                                <svg class="w-4 h-4 inline mr-1 sm:mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
                                </svg>
                                <span class="hidden sm:inline">Monitor</span>
                                <span class="sm:hidden">Mon</span>
                            </button>
                            <button class="alert-tab py-3 px-2 sm:px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm whitespace-nowrap" data-tab="configure">
                                <svg class="w-4 h-4 inline mr-1 sm:mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                                </svg>
                                <span class="hidden sm:inline">Configure</span>
                                <span class="sm:hidden">Config</span>
                            </button>
                        </nav>
                    </div>
                    
                    <div id="alert-management-modal-body" class="overflow-y-auto flex-grow p-4 sm:p-6 scrollbar">
                        <p class="text-gray-500 dark:text-gray-400">Loading...</p>
                    </div>
                    
                    <div class="modal-footer border-t border-gray-300 dark:border-gray-700 px-4 sm:px-6 py-3 sm:py-4">
                        <div class="flex gap-2 sm:gap-3 justify-end">
                            <button type="button" id="alert-management-cancel-button" class="px-3 sm:px-4 py-2 bg-gray-500 hover:bg-gray-600 text-white text-sm font-medium rounded-md transition-colors">
                                Close
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

        if (closeButton) {
            closeButton.addEventListener('click', closeModal);
        }

        if (cancelButton) {
            cancelButton.addEventListener('click', closeModal);
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
                e.preventDefault();
                const tabName = e.currentTarget.getAttribute('data-tab');
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

    // Utility function to create consistent section banners

    function renderTabContent() {
        const modalBody = document.getElementById('alert-management-modal-body');
        if (!modalBody) return;

        switch (activeTab) {
            case 'monitor':
                modalBody.innerHTML = renderMonitorTab();
                initializeMonitorTab();
                break;
            case 'configure':
                try {
                    const configureTabContent = renderConfigureTab();
                    modalBody.innerHTML = configureTabContent;
                    initializeConfigureTab();
                    // Load configuration for the configure tab
                    if (currentConfig && Object.keys(currentConfig).length > 0) {
                        setTimeout(async () => {
                            loadEmailConfiguration();
                            await loadGlobalToggles();
                        }, 50);
                    } else {
                        loadConfiguration().then(async () => {
                            // Add a small delay to ensure DOM is ready
                            setTimeout(async () => {
                                loadEmailConfiguration();
                                await loadGlobalToggles();
                            }, 100);
                        });
                    }
                } catch (error) {
                    console.error('[Alert Modal] Error rendering configure tab:', error);
                    modalBody.innerHTML = '<p class="text-red-500">Error loading configure tab: ' + error.message + '</p>';
                }
                break;
            default:
                modalBody.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Unknown tab</p>';
        }
    }

    function renderMonitorTab() {
        return `
            <div class="space-y-6">
                <!-- Alert Summary Cards -->
                <div class="grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4">
                    <div class="bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg p-4">
                        <div class="flex items-center justify-between">
                            <div>
                                <p class="text-sm font-medium text-orange-600 dark:text-orange-400">Active Alerts</p>
                                <p class="text-2xl font-bold text-orange-900 dark:text-orange-100" id="active-alerts-count">0</p>
                            </div>
                            <svg class="w-8 h-8 text-orange-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                            </svg>
                        </div>
                    </div>
                    
                    <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4">
                        <div class="flex items-center justify-between">
                            <div>
                                <p class="text-sm font-medium text-green-600 dark:text-green-400">Acknowledged</p>
                                <p class="text-2xl font-bold text-green-900 dark:text-green-100" id="acknowledged-alerts-count">0</p>
                            </div>
                            <svg class="w-8 h-8 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                            </svg>
                        </div>
                    </div>
                    
                    <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                        <div class="flex items-center justify-between">
                            <div>
                                <p class="text-sm font-medium text-blue-600 dark:text-blue-400">Alert Rules</p>
                                <p class="text-2xl font-bold text-blue-900 dark:text-blue-100" id="total-rules-count">0</p>
                            </div>
                            <svg class="w-8 h-8 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z" clip-rule="evenodd"/>
                            </svg>
                        </div>
                    </div>
                    
                    <div class="bg-gray-50 dark:bg-gray-900/20 border border-gray-200 dark:border-gray-800 rounded-lg p-4">
                        <div class="flex items-center justify-between">
                            <div>
                                <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Monitoring</p>
                                <p class="text-xl font-bold text-gray-900 dark:text-gray-100" id="monitoring-status">Active</p>
                            </div>
                            <svg class="w-8 h-8 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                            </svg>
                        </div>
                    </div>
                </div>

                <!-- Active Alerts Section -->
                <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center space-x-3">
                            <svg class="w-5 h-5 text-red-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                            </svg>
                            <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Active Alerts</h3>
                            <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200" id="active-alerts-badge">0</span>
                        </div>
                        <div class="flex items-center space-x-1 sm:space-x-2">
                            <button id="refresh-alerts-btn" class="px-2 sm:px-3 py-1 text-xs sm:text-sm bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded transition-colors">
                                <svg class="w-3 h-3 sm:w-4 sm:h-4 inline mr-0 sm:mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                                </svg>
                                <span class="hidden sm:inline">Refresh</span>
                            </button>
                            <button id="acknowledge-all-alerts-btn" class="px-2 sm:px-3 py-1 text-xs sm:text-sm bg-green-600 hover:bg-green-700 text-white rounded transition-colors">
                                <svg class="w-3 h-3 sm:w-4 sm:h-4 inline mr-0 sm:mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                </svg>
                                <span class="hidden sm:inline">Acknowledge All</span>
                                <span class="sm:hidden">Ack All</span>
                            </button>
                        </div>
                    </div>
                    <div id="active-alerts-list" class="divide-y divide-gray-200 dark:divide-gray-700">
                        <p class="p-6 text-gray-500 dark:text-gray-400">Loading active alerts...</p>
                    </div>
                </div>

                <!-- Recent Activity Section -->
                <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center space-x-3">
                            <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
                            </svg>
                            <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Recent Activity</h3>
                            <span class="text-sm text-gray-500 dark:text-gray-400">(Last 24 hours)</span>
                        </div>
                    </div>
                    <div id="recent-activity-list" class="divide-y divide-gray-200 dark:divide-gray-700 max-h-64 overflow-y-auto">
                        <p class="p-6 text-gray-500 dark:text-gray-400">Loading recent activity...</p>
                    </div>
                </div>
            </div>
        `;
    }

    function renderConfigureTab() {
        return `
            <div class="space-y-6">
                <!-- Unified Alert Rules Section -->
                <div>
                    <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center space-x-3">
                            <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"/>
                            </svg>
                            <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Alert Rules</h3>
                        </div>
                        <button onclick="openAlertRuleModal()" class="px-3 sm:px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-xs sm:text-sm font-medium rounded-md transition-colors">
                            <svg class="w-3 h-3 sm:w-4 sm:h-4 inline mr-1 sm:mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
                            </svg>
                            <span class="hidden sm:inline">Create Rule</span>
                            <span class="sm:hidden">Create</span>
                        </button>
                    </div>
                    
                    <!-- Unified Alert Rules List -->
                    <div id="alert-rules-list" class="space-y-3">
                        <p class="text-gray-500 dark:text-gray-400">Loading alert rules...</p>
                    </div>
                </div>

                <!-- Notification Settings Section -->
                <div>
                    <div class="flex items-center space-x-3 mb-4">
                        <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-5 5v-5zM4 5h5L4 0v5zm7 12l4-4m-4 4l4-4"/>
                        </svg>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Notification Settings</h3>
                    </div>
                    
                    <!-- Global Controls -->
                    <div class="p-4 border border-gray-200 dark:border-gray-700 rounded-lg bg-blue-50 dark:bg-blue-900/20 mb-4">
                        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                            <div>
                                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Global Notification Controls</h4>
                                <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">Master switches for all alert notifications</p>
                            </div>
                            <div class="flex flex-wrap items-center gap-6">
                                <label class="flex items-center gap-3">
                                    <span class="text-sm text-gray-700 dark:text-gray-300">Email Notifications</span>
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input type="checkbox" id="global-email-toggle" class="sr-only peer">
                                        <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                    </label>
                                </label>
                                <label class="flex items-center gap-3">
                                    <span class="text-sm text-gray-700 dark:text-gray-300">Webhook Notifications</span>
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input type="checkbox" id="global-webhook-toggle" class="sr-only peer">
                                        <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                    </label>
                                </label>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Email Configuration -->
                    <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 space-y-4" id="email-config-section">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Email Configuration</h4>
                        
                        <div class="grid grid-cols-1 gap-3">
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">From Email</label>
                                <input type="email" name="ALERT_FROM_EMAIL" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="alerts@yourdomain.com">
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">To Email</label>
                                <input type="email" name="ALERT_TO_EMAIL" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="admin@yourdomain.com">
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SMTP Server</label>
                                <input type="text" name="SMTP_HOST" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="smtp.gmail.com">
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SMTP Port</label>
                                <input type="number" name="SMTP_PORT" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="587" value="587">
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Username</label>
                                <input type="text" name="SMTP_USER" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="your.email@gmail.com">
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Password</label>
                                <input type="password" name="SMTP_PASS" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="Enter password">
                            </div>
                        </div>
                        
                        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                            <label class="flex items-center">
                                <input type="checkbox" name="SMTP_SECURE" class="h-4 w-4 text-blue-600 border-gray-300 rounded">
                                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Use SSL/TLS encryption</span>
                            </label>
                            <div class="flex gap-3">
                                <button id="test-email-btn" class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                                    Test Email
                                </button>
                                <button id="save-email-config-btn" class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                                    Save Configuration
                                </button>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Webhook Configuration -->
                    <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 mt-4" id="webhook-config-section">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Webhook Configuration</h4>
                        <div class="space-y-3">
                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Webhook URL</label>
                                <input type="url" name="WEBHOOK_URL" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="https://hooks.slack.com/services/...">
                            </div>
                            <div class="flex justify-end pt-3 border-t border-gray-200 dark:border-gray-700">
                                <div class="flex gap-3">
                                    <button id="test-webhook-btn" class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                                        Test Webhook
                                    </button>
                                    <button id="save-webhook-btn" class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                                        Save Configuration
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function initializeMonitorTab() {
        // Load current alerts and populate the monitor tab
        loadCurrentAlerts();
        loadRecentActivity();
        updateAlertSummary();
        
        // Set up button event listeners
        const refreshBtn = document.getElementById('refresh-alerts-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => {
                coordinatedRefresh();
                loadRecentActivity();
                updateAlertSummary();
            });
        }
        
        const acknowledgeAllBtn = document.getElementById('acknowledge-all-alerts-btn');
        if (acknowledgeAllBtn) {
            acknowledgeAllBtn.addEventListener('click', () => {
                if (PulseApp.alerts && PulseApp.alerts.markAllAsAcknowledged) {
                    PulseApp.alerts.markAllAsAcknowledged();
                }
            });
        }
    }

    function initializeConfigureTab() {
        // Load all alert rules (unified)
        loadAllAlertRules();
        
        // Set up event listeners for notifications
        setupNotificationToggles();
        setupEmailTestButton();
        
        // Create rule button now uses onclick attribute directly
    }

    async function loadAllAlertRules() {
        const alertRulesList = document.getElementById('alert-rules-list');
        if (!alertRulesList) return;

        try {
            // Load all alert rules from the API
            const response = await fetch('/api/alerts/rules');
            const data = await response.json();
            const rules = data.rules || [];

            if (rules.length === 0) {
                alertRulesList.innerHTML = `
                    <div class="text-center py-8">
                        <svg class="w-12 h-12 text-gray-400 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                        <p class="text-gray-500 dark:text-gray-400 mb-4">No alert rules configured</p>
                        <button onclick="openAlertRuleModal()" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md">
                            Create First Rule
                        </button>
                    </div>
                `;
                return;
            }

            // Sort rules by group then by name
            rules.sort((a, b) => {
                const groupA = a.group || 'zz_custom';
                const groupB = b.group || 'zz_custom';
                if (groupA !== groupB) {
                    return groupA.localeCompare(groupB);
                }
                return (a.name || '').localeCompare(b.name || '');
            });

            alertRulesList.innerHTML = rules.map(rule => renderUnifiedAlertRule(rule)).join('');

        } catch (error) {
            console.error('Error loading alert rules:', error);
            alertRulesList.innerHTML = `
                <div class="text-center py-8 text-red-500">
                    <p>Error loading alert rules</p>
                </div>
            `;
        }
    }

    function renderUnifiedAlertRule(rule) {
        const isCompoundRule = rule.thresholds && Array.isArray(rule.thresholds) && rule.thresholds.length > 0;
        
        // Generate threshold display
        const thresholdDisplay = (() => {
            if (rule.thresholds && Array.isArray(rule.thresholds)) {
                // Custom compound rule
                const thresholds = rule.thresholds.map(t => {
                    const unit = ['cpu', 'memory', 'disk'].includes(t.metric) ? '%' : '';
                    return `${t.metric.toUpperCase()} ${t.condition === 'greater_than' ? '>' : t.condition === 'less_than' ? '<' : '≥'} ${t.threshold}${unit}`;
                });
                return thresholds.join(' AND ');
            } else if (rule.threshold !== undefined) {
                // Simple rule with single threshold
                const unit = ['cpu', 'memory', 'disk'].includes(rule.metric) ? '%' : '';
                const condition = rule.condition === 'greater_than' ? '>' : rule.condition === 'less_than' ? '<' : '≥';
                return `${rule.metric ? rule.metric.toUpperCase() : 'VALUE'} ${condition} ${rule.threshold}${unit}`;
            }
            return '';
        })();
        
        // Generate duration display
        const durationDisplay = (() => {
            if (rule.duration) {
                const minutes = Math.round(rule.duration / 60000);
                return minutes === 1 ? '1 min' : `${minutes} mins`;
            }
            return '';
        })();
        
        return `
            <div class="bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
                <div class="flex flex-col gap-3">
                    <!-- Header with title and toggle -->
                    <div class="flex items-start justify-between gap-3">
                        <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100 flex-1">${rule.name || rule.id}</h5>
                        <label class="relative inline-flex items-center cursor-pointer flex-shrink-0">
                            <input type="checkbox" ${rule.enabled !== false ? 'checked' : ''} class="sr-only peer" onchange="toggleAlertRule('${rule.id}', this.checked)">
                            <div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                        </label>
                    </div>
                    
                    <!-- Condition section -->
                    ${thresholdDisplay ? `
                        <div class="bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-600 p-2">
                            <div class="flex flex-col gap-1">
                                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">Condition:</span>
                                <div class="flex flex-wrap items-center gap-2">
                                    <code class="text-xs px-2 py-1 bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 rounded font-mono">${thresholdDisplay}</code>
                                    ${durationDisplay ? `<span class="text-xs text-gray-500 dark:text-gray-400">for ${durationDisplay}</span>` : ''}
                                </div>
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Description -->
                    <p class="text-xs text-gray-500 dark:text-gray-400 leading-relaxed">${rule.description || getAlertRuleDescription(rule)}</p>
                    
                    <!-- Action buttons -->
                    <div class="flex items-center gap-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                        <button onclick="editAlertRule('${rule.id}')" class="px-3 py-1.5 text-xs bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded transition-colors flex items-center gap-1">
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                            </svg>
                            Edit
                        </button>
                        <button onclick="deleteAlertRule('${rule.id}')" class="px-3 py-1.5 text-xs bg-red-50 hover:bg-red-100 dark:bg-red-900/30 dark:hover:bg-red-900/50 text-red-600 dark:text-red-400 rounded transition-colors flex items-center gap-1">
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                            </svg>
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    function getAlertRuleDescription(rule) {
        if (rule.description) return rule.description;
        
        // Generate description for system rules
        switch (rule.id) {
            case 'cpu':
                return 'Monitors CPU usage across all VMs and containers';
            case 'memory':
                return 'Monitors memory usage across all VMs and containers';
            case 'disk':
                return 'Monitors disk space usage across all VMs and containers';
            case 'down':
                return 'Monitors VM/container availability and uptime';
            default:
                if (rule.thresholds && rule.thresholds.length > 0) {
                    return `Custom rule with ${rule.thresholds.length} threshold${rule.thresholds.length > 1 ? 's' : ''}`;
                }
                return 'Custom alert rule';
        }
    }

    // Unified alert rule functions
    function openAlertRuleModal(existingRule = null) {
        // For now, redirect to the existing custom alert modal
        // TODO: Create a unified modal that handles all rules
        openCustomAlertModal([], existingRule);
    }

    async function editAlertRule(ruleId) {
        try {
            // Fetch the current alert rule data from API
            const response = await fetch('/api/alerts/rules');
            if (!response.ok) {
                throw new Error(`Failed to fetch alert rules: ${response.status}`);
            }
            
            const data = await response.json();
            const rule = data.rules ? data.rules.find(r => r.id === ruleId) : null;
            
            if (!rule) {
                throw new Error('Alert rule not found');
            }
            
            // Convert single-metric rules to compound threshold format for unified editing
            let thresholds = [];
            let alertData = rule;
            
            if (rule.metric && rule.threshold !== undefined && !rule.thresholds) {
                // Convert simple single-metric rule to compound threshold format
                thresholds = [{
                    metric: rule.metric,
                    condition: rule.condition || 'greater_than',
                    threshold: rule.threshold
                }];
                
                // Create alertData object that openCustomAlertModal can understand
                alertData = {
                    ...rule,
                    type: 'compound_threshold',
                    thresholds: thresholds
                };
            } else {
                // Multi-threshold rule - use existing thresholds
                thresholds = rule.thresholds || [];
            }
            
            // Open the unified custom alert modal with pre-filled data
            openCustomAlertModal(thresholds, alertData);
            
        } catch (error) {
            console.error('Failed to load alert rule for editing:', error);
            PulseApp.ui.toast.error('Failed to load alert rule for editing');
        }
    }

    async function toggleAlertRule(ruleId, enabled) {
        // All rules use the same unified API endpoint now
        try {
            const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ enabled })
            });

            if (!response.ok) {
                throw new Error(`Failed to toggle alert rule: ${response.status}`);
            }

            PulseApp.ui.toast.success(`Alert rule ${enabled ? 'enabled' : 'disabled'}`);
            return true;
        } catch (error) {
            console.error('Failed to toggle alert rule:', error);
            PulseApp.ui.toast.error('Failed to toggle alert rule');
            return false;
        }
    }

    async function deleteAlertRule(ruleId) {
        if (confirm('Are you sure you want to delete this alert rule? This action cannot be undone.')) {
            try {
                const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    const errorData = await response.json().catch(() => ({}));
                    throw new Error(errorData.error || `Server error: ${response.status}`);
                }

                PulseApp.ui.toast.success('Alert rule deleted successfully');
                await loadAllAlertRules(); // Refresh the list
                
            } catch (error) {
                console.error('Failed to delete alert rule:', error);
                PulseApp.ui.toast.error('Failed to delete alert rule');
            }
        }
    }

    function updateAlertSummary() {
        // Get current alerts and update summary cards
        if (PulseApp.alerts && PulseApp.alerts.getActiveAlerts) {
            const alerts = PulseApp.alerts.getActiveAlerts();
            const acknowledgedAlerts = alerts.filter(alert => alert.acknowledged);
            const activeAlerts = alerts.filter(alert => !alert.acknowledged);
            
            // Update the count displays with null checks
            const activeAlertsEl = document.getElementById('active-alerts-count');
            const acknowledgedAlertsEl = document.getElementById('acknowledged-alerts-count');
            const badgeEl = document.getElementById('active-alerts-badge');
            
            if (activeAlertsEl) activeAlertsEl.textContent = activeAlerts.length;
            if (acknowledgedAlertsEl) acknowledgedAlertsEl.textContent = acknowledgedAlerts.length;
            if (badgeEl) badgeEl.textContent = activeAlerts.length; // Badge shows only active alerts
            
            console.log(`[Alert Summary] Updated counts: Active=${activeAlerts.length}, Acknowledged=${acknowledgedAlerts.length}, Total=${alerts.length}`);
        } else {
            console.log('[Alert Summary] PulseApp.alerts not available or getCurrentAlerts not found');
        }
        
        // Update rules count
        updateRulesCount();
    }
    
    async function updateRulesCount() {
        try {
            const response = await fetch('/api/alerts/rules');
            if (response.ok) {
                const data = await response.json();
                const rulesCount = data.rules ? data.rules.length : 0;
                const totalRulesEl = document.getElementById('total-rules-count');
                if (totalRulesEl) totalRulesEl.textContent = rulesCount;
            }
        } catch (error) {
            console.error('Failed to load rules count:', error);
        }
    }

    async function loadRecentActivity() {
        const activityList = document.getElementById('recent-activity-list');
        if (!activityList) return;

        try {
            // Load recent alert history (last 24 hours)
            const response = await fetch('/api/alerts/history?limit=20');
            if (!response.ok) throw new Error('Failed to fetch recent activity');
            
            const data = await response.json();
            const recentAlerts = data.history || [];
            
            // Filter to last 24 hours
            const oneDayAgo = Date.now() - (24 * 60 * 60 * 1000);
            const recent = recentAlerts.filter(alert => 
                (alert.triggeredAt || alert.resolvedAt || alert.acknowledgedAt) >= oneDayAgo
            );
            
            if (recent.length === 0) {
                activityList.innerHTML = `
                    <div class="p-6 text-center">
                        <svg class="w-12 h-12 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                        <p class="text-gray-500 dark:text-gray-400">No recent activity</p>
                        <p class="text-sm text-gray-400 dark:text-gray-500 mt-1">This is a good thing!</p>
                    </div>
                `;
                return;
            }
            
            // Render recent activity items
            const content = recent.map(alert => renderActivityItem(alert)).join('');
            activityList.innerHTML = content;
            
        } catch (error) {
            console.error('Error loading recent activity:', error);
            activityList.innerHTML = `
                <div class="p-6 text-center">
                    <svg class="w-12 h-12 mx-auto mb-4 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                    </svg>
                    <p class="text-red-500 dark:text-red-400">Error loading recent activity</p>
                </div>
            `;
        }
    }

    function renderActivityItem(alert) {
        const time = new Date(alert.triggeredAt || alert.resolvedAt || alert.acknowledgedAt);
        const timeAgo = getTimeAgo(time);
        
        let statusIcon = '';
        let statusText = '';
        
        if (alert.resolvedAt) {
            statusIcon = '<svg class="w-4 h-4 text-green-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>';
            statusText = 'Resolved';
        } else if (alert.acknowledgedAt) {
            statusIcon = '<svg class="w-4 h-4 text-blue-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/></svg>';
            statusText = 'Acknowledged';
        } else {
            statusIcon = '<svg class="w-4 h-4 text-red-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/></svg>';
            statusText = 'Triggered';
        }
        
        return `
            <div class="p-4 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                <div class="flex items-center justify-between">
                    <div class="flex items-center space-x-3">
                        ${statusIcon}
                        <div>
                            <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${alert.ruleName || 'Unknown Rule'}
                            </p>
                            <p class="text-xs text-gray-500 dark:text-gray-400">
                                ${alert.guest?.name || 'Unknown'} on ${alert.guest?.node || 'Unknown'}
                            </p>
                        </div>
                    </div>
                    <div class="flex items-center space-x-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400">${timeAgo}</span>
                    </div>
                </div>
            </div>
        `;
    }

    function getTimeAgo(date) {
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);
        
        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        return `${diffDays}d ago`;
    }

    function loadSystemRules() {
        const systemRulesList = document.getElementById('system-rules-list');
        if (!systemRulesList) return;
        
        renderSystemRulesForConfigure();
    }

    function loadCustomRules() {
        const customRulesList = document.getElementById('custom-rules-list');
        if (!customRulesList) return;
        
        renderCustomRulesForConfigure();
    }

    function renderSystemRulesForConfigure() {
        const systemRulesList = document.getElementById('system-rules-list');
        if (!systemRulesList) return;

        // System rules with current state from environment variables
        const systemRules = [
            { 
                id: 'cpu', 
                name: 'CPU Usage', 
                description: 'Monitor CPU usage across VMs/LXCs', 
                enabled: currentConfig?.ALERT_CPU_ENABLED !== 'false'
            },
            { 
                id: 'memory', 
                name: 'Memory Usage', 
                description: 'Monitor memory usage across VMs/LXCs', 
                enabled: currentConfig?.ALERT_MEMORY_ENABLED !== 'false'
            },
            { 
                id: 'disk', 
                name: 'Disk Usage', 
                description: 'Monitor disk space across VMs/LXCs', 
                enabled: currentConfig?.ALERT_DISK_ENABLED === 'true'
            },
            { 
                id: 'down', 
                name: 'VM/LXC Down', 
                description: 'Alert when VMs or LXCs go offline', 
                enabled: currentConfig?.ALERT_DOWN_ENABLED === 'true'
            }
        ];

        const rulesHTML = systemRules.map(rule => `
            <div class="flex items-center justify-between p-3 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-md">
                <div class="flex items-center space-x-3">
                    <div class="flex-shrink-0">
                        <div class="w-2 h-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}"></div>
                    </div>
                    <div>
                        <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${rule.name}</h5>
                        <p class="text-xs text-gray-500 dark:text-gray-400">${rule.description}</p>
                    </div>
                </div>
                <div class="flex items-center space-x-2">
                    <label class="relative inline-flex items-center cursor-pointer">
                        <input type="checkbox" ${rule.enabled ? 'checked' : ''} class="sr-only peer" onchange="toggleSystemAlert('${rule.id}', this.checked)">
                        <div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </label>
                    <button onclick="editAlertRule('${rule.id}')" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                        </svg>
                    </button>
                </div>
            </div>
        `).join('');

        systemRulesList.innerHTML = rulesHTML;
    }

    function renderCustomRulesForConfigure() {
        const customRulesList = document.getElementById('custom-rules-list');
        if (!customRulesList) return;

        // Get custom alert rules from the existing system
        fetch('/api/alerts/rules?group=custom')
            .then(response => response.json())
            .then(data => {
                const customRules = data.rules || [];
                
                if (customRules.length === 0) {
                    customRulesList.innerHTML = `
                        <div class="text-center py-6">
                            <svg class="w-12 h-12 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
                            </svg>
                            <p class="text-gray-500 dark:text-gray-400 mb-3">No custom rules yet</p>
                            <button onclick="openCustomAlertModal()" class="text-blue-600 hover:text-blue-700 dark:text-blue-400 text-sm font-medium">
                                Create your first custom rule
                            </button>
                        </div>
                    `;
                    return;
                }

                const rulesHTML = customRules.map(rule => `
                    <div class="flex items-center justify-between p-3 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-md">
                        <div class="flex items-center space-x-3">
                            <div class="flex-shrink-0">
                                <div class="w-2 h-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}"></div>
                            </div>
                            <div>
                                <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${rule.name || 'Unnamed Rule'}</h5>
                                <p class="text-xs text-gray-500 dark:text-gray-400">${rule.description || 'No description'}</p>
                                <div class="flex items-center space-x-2 mt-1">
                                    ${getActiveAlertsCount(rule.id) > 0 ? `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">${getActiveAlertsCount(rule.id)} active</span>` : ''}
                                </div>
                            </div>
                        </div>
                        <div class="flex items-center space-x-2">
                            <label class="relative inline-flex items-center cursor-pointer">
                                <input type="checkbox" ${rule.enabled ? 'checked' : ''} class="sr-only peer" onchange="toggleCustomAlert('${rule.id}', this.checked)">
                                <div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                            </label>
                            <button onclick="editCustomAlert('${rule.id}')" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
                                </svg>
                            </button>
                            <button onclick="deleteCustomAlert('${rule.id}')" class="p-1 text-red-400 hover:text-red-600">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                `).join('');

                customRulesList.innerHTML = rulesHTML;
            })
            .catch(error => {
                console.error('Error loading custom rules:', error);
                customRulesList.innerHTML = `
                    <div class="p-4 text-center">
                        <svg class="w-8 h-8 mx-auto mb-2 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                        <p class="text-red-500 dark:text-red-400 text-sm">Error loading custom rules</p>
                    </div>
                `;
            });
    }


    function getActiveAlertsCount(ruleId) {
        if (PulseApp.alerts && PulseApp.alerts.getCurrentAlerts) {
            const alerts = PulseApp.alerts.getCurrentAlerts();
            return alerts.filter(alert => alert.ruleId === ruleId).length;
        }
        return 0;
    }

    function setupNotificationToggles() {
        const emailToggle = document.getElementById('global-email-toggle');
        const webhookToggle = document.getElementById('global-webhook-toggle');
        
        if (emailToggle) {
            emailToggle.addEventListener('change', async (e) => {
                await handleGlobalEmailToggle(e.target.checked);
            });
        }
        
        if (webhookToggle) {
            webhookToggle.addEventListener('change', async (e) => {
                await handleGlobalWebhookToggle(e.target.checked);
            });
        }
    }
    
    async function handleGlobalEmailToggle(enabled) {
        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ALERT_EMAIL_ENABLED: enabled ? 'true' : 'false' })
            });
            
            if (!response.ok) {
                throw new Error('Failed to update email notification setting');
            }
            
            // Update currentConfig
            if (currentConfig) {
                currentConfig.ALERT_EMAIL_ENABLED = enabled ? 'true' : 'false';
            }
            
            // Show visual feedback
            const toggleEl = document.getElementById('global-email-toggle');
            if (toggleEl) {
                // Briefly flash the toggle to indicate success
                const parent = toggleEl.parentElement;
                parent.classList.add('ring-2', 'ring-green-500');
                setTimeout(() => parent.classList.remove('ring-2', 'ring-green-500'), 1000);
            }
            
        } catch (error) {
            console.error('Error updating email toggle:', error);
            // Revert the toggle on error
            const toggleEl = document.getElementById('global-email-toggle');
            if (toggleEl) {
                toggleEl.checked = !enabled;
            }
            PulseApp.ui.toast.error('Failed to update email notifications');
        }
    }
    
    async function handleGlobalWebhookToggle(enabled) {
        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ALERT_WEBHOOK_ENABLED: enabled ? 'true' : 'false' })
            });
            
            if (!response.ok) {
                throw new Error('Failed to update webhook notification setting');
            }
            
            // Update currentConfig
            if (currentConfig) {
                currentConfig.ALERT_WEBHOOK_ENABLED = enabled ? 'true' : 'false';
            }
            
            // Show visual feedback
            const toggleEl = document.getElementById('global-webhook-toggle');
            if (toggleEl) {
                // Briefly flash the toggle to indicate success
                const parent = toggleEl.parentElement;
                parent.classList.add('ring-2', 'ring-green-500');
                setTimeout(() => parent.classList.remove('ring-2', 'ring-green-500'), 1000);
            }
            
        } catch (error) {
            console.error('Error updating webhook toggle:', error);
            // Revert the toggle on error
            const toggleEl = document.getElementById('global-webhook-toggle');
            if (toggleEl) {
                toggleEl.checked = !enabled;
            }
            PulseApp.ui.toast.error('Failed to update webhook notifications');
        }
    }

    function setupEmailTestButton() {
        const testBtn = document.getElementById('test-email-btn');
        if (testBtn) {
            testBtn.addEventListener('click', testEmailConfiguration);
        }
    }
    
    async function testEmailConfiguration() {
        const testBtn = document.getElementById('test-email-btn');
        const originalText = testBtn.textContent;
        
        try {
            testBtn.disabled = true;
            testBtn.textContent = 'Testing...';
            
            const response = await fetch('/api/alerts/test-email', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });
            
            const result = await response.json();
            
            if (result.success) {
                testBtn.textContent = 'Test Sent!';
                testBtn.classList.remove('bg-gray-200', 'hover:bg-gray-300');
                testBtn.classList.add('bg-green-500', 'text-white');
                PulseApp.ui.toast.success('Test email sent successfully! Check your inbox.');
            } else {
                testBtn.textContent = 'Test Failed';
                testBtn.classList.remove('bg-gray-200', 'hover:bg-gray-300');
                testBtn.classList.add('bg-red-500', 'text-white');
                PulseApp.ui.toast.error(result.error || 'Failed to send test email');
            }
            
            // Reset button after 3 seconds
            setTimeout(() => {
                testBtn.textContent = originalText;
                testBtn.classList.remove('bg-green-500', 'bg-red-500', 'text-white');
                testBtn.classList.add('bg-gray-200', 'hover:bg-gray-300');
                testBtn.disabled = false;
            }, 3000);
            
        } catch (error) {
            console.error('[Test Email] Error:', error);
            testBtn.textContent = originalText;
            testBtn.disabled = false;
            PulseApp.ui.toast.error('Failed to test email configuration');
        }
    }

    function handleGlobalEmailToggle(enabled) {
        console.log(`[Email Toggle] Setting GLOBAL_EMAIL_ENABLED to: ${enabled}`);
        
        // Save the setting
        fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ GLOBAL_EMAIL_ENABLED: enabled ? 'true' : 'false' })
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                console.log(`[Email Toggle] Successfully ${enabled ? 'enabled' : 'disabled'} global email notifications`);
                PulseApp.ui.toast.success(`Email notifications ${enabled ? 'enabled' : 'disabled'}`);
                updateEmailConfigVisibility(enabled);
                // Update currentConfig immediately
                if (currentConfig) {
                    currentConfig.GLOBAL_EMAIL_ENABLED = enabled ? 'true' : 'false';
                }
            } else {
                console.error('[Email Toggle] Failed to save setting');
                PulseApp.ui.toast.error('Failed to save email setting');
            }
        })
        .catch(error => {
            console.error('[Email Toggle] Error saving setting:', error);
            PulseApp.ui.toast.error('Failed to save email setting');
        });
    }

    function handleGlobalWebhookToggle(enabled) {
        console.log(`[Webhook Toggle] Setting GLOBAL_WEBHOOK_ENABLED to: ${enabled}`);
        
        // Save the setting
        fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ GLOBAL_WEBHOOK_ENABLED: enabled ? 'true' : 'false' })
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                console.log(`[Webhook Toggle] Successfully ${enabled ? 'enabled' : 'disabled'} global webhook notifications`);
                PulseApp.ui.toast.success(`Webhook notifications ${enabled ? 'enabled' : 'disabled'}`);
                updateWebhookConfigVisibility(enabled);
                // Update currentConfig immediately
                if (currentConfig) {
                    currentConfig.GLOBAL_WEBHOOK_ENABLED = enabled ? 'true' : 'false';
                }
            } else {
                console.error('[Webhook Toggle] Failed to save setting');
                PulseApp.ui.toast.error('Failed to save webhook setting');
            }
        })
        .catch(error => {
            console.error('[Webhook Toggle] Error saving setting:', error);
            PulseApp.ui.toast.error('Failed to save webhook setting');
        });
    }

    function updateEmailConfigVisibility(enabled) {
        const emailSection = document.getElementById('email-config-section');
        if (emailSection) {
            emailSection.style.opacity = enabled ? '1' : '0.5';
            // Only disable input fields, not test/save buttons
            const inputs = emailSection.querySelectorAll('input[name*="SMTP"], input[name*="ALERT_"]');
            inputs.forEach(input => {
                input.disabled = !enabled;
            });
        }
    }

    function updateWebhookConfigVisibility(enabled) {
        const webhookSection = document.getElementById('webhook-config-section');
        if (webhookSection) {
            webhookSection.style.opacity = enabled ? '1' : '0.5';
            // Only disable input fields, not test/save buttons
            const inputs = webhookSection.querySelectorAll('input[name*="WEBHOOK"]');
            inputs.forEach(input => {
                input.disabled = !enabled;
            });
        }
    }

    async function loadGlobalToggles() {
        // Load and set the current state of global toggles
        const emailToggle = document.getElementById('global-email-toggle');
        const webhookToggle = document.getElementById('global-webhook-toggle');
        
        // Ensure we have fresh config
        if (!currentConfig || Object.keys(currentConfig).length === 0) {
            await loadConfiguration();
        }
        
        console.log('[Global Toggles] Loading with config:', {
            ALERT_EMAIL_ENABLED: currentConfig.ALERT_EMAIL_ENABLED,
            ALERT_WEBHOOK_ENABLED: currentConfig.ALERT_WEBHOOK_ENABLED
        });
        
        if (emailToggle && currentConfig) {
            const emailEnabled = currentConfig.ALERT_EMAIL_ENABLED === 'true';
            console.log('[Global Toggles] Setting email toggle to:', emailEnabled);
            emailToggle.checked = emailEnabled;
            updateEmailConfigVisibility(emailEnabled);
        }
        
        if (webhookToggle && currentConfig) {
            const webhookEnabled = currentConfig.ALERT_WEBHOOK_ENABLED === 'true';
            console.log('[Global Toggles] Setting webhook toggle to:', webhookEnabled);
            webhookToggle.checked = webhookEnabled;
            updateWebhookConfigVisibility(webhookEnabled);
        }
    }

    function renderAlertsTab() {
        return `
            <div class="space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Alerts</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">View and manage active alerts and alert history</p>
                    </div>
                    <button id="refresh-alerts-btn" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        Refresh
                    </button>
                </div>

                <!-- Sub-tabs for Alerts -->
                <div class="border-b border-gray-200 dark:border-gray-700">
                    <nav class="flex space-x-8" id="alerts-sub-tabs">
                        <button class="alerts-sub-tab active py-2 px-1 border-b-2 border-blue-500 text-blue-600 dark:text-blue-400 font-medium text-sm" data-tab="current">
                            Current Alerts
                        </button>
                        <button class="alerts-sub-tab py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-tab="history">
                            Alert History
                        </button>
                    </nav>
                </div>

                <!-- Current Alerts Content -->
                <div id="current-alerts-content" class="space-y-4">
                    <div id="current-alerts-list" class="space-y-4">
                        <p class="text-gray-500 dark:text-gray-400">Loading current alerts...</p>
                    </div>
                </div>

                <!-- Alert History Content -->
                <div id="alert-history-content" class="space-y-4 hidden">
                    <div id="alert-history-list" class="space-y-4">
                        <p class="text-gray-500 dark:text-gray-400">Loading alert history...</p>
                    </div>
                </div>
                
                <div class="mt-6 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Quick Actions</h4>
                    <div class="flex gap-2">
                        <button id="acknowledge-all-alerts-btn" class="px-3 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                            Acknowledge All
                        </button>
                        <button id="clear-history-btn" class="px-3 py-2 bg-red-600 hover:bg-red-700 text-white text-sm font-medium rounded-md transition-colors hidden">
                            Clear History
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
                            
                            <div id="system-alerts-content" class="space-y-3">
                                <!-- System alerts will be loaded dynamically here -->
                                <p class="text-sm text-gray-500 dark:text-gray-400">Loading system alerts...</p>
                            </div>
                        </div>
                    </div>

                    <!-- Custom Alerts Tab Content (Hidden by default) -->
                    <div id="custom-alert-rules-content" class="alert-rules-sub-content hidden">
                        <div class="space-y-6">
                            
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
                            <!-- Primary Email Configuration -->
                            <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                                <div class="flex items-center justify-between mb-4">
                                    <div>
                                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Email Notifications</h3>
                                        <p class="text-sm text-gray-600 dark:text-gray-400">Primary email configuration for alert delivery</p>
                                    </div>
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input type="checkbox" id="email-enabled" class="sr-only peer">
                                        <div class="w-11 h-6 bg-gray-300 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-gray-400 after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-500 peer-checked:after:bg-white dark:after:bg-gray-200"></div>
                                    </label>
                                </div>
                                
                                <!-- Email Provider Quick Setup -->
                                <div class="mb-4">
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Email Provider</label>
                                    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
                                        <button type="button" onclick="PulseApp.ui.alertManagementModal.handleEmailProviderSelection('gmail')" 
                                                class="email-provider-btn px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 text-gray-700 dark:text-gray-300">
                                            Gmail
                                        </button>
                                        <button type="button" onclick="PulseApp.ui.alertManagementModal.handleEmailProviderSelection('outlook')" 
                                                class="email-provider-btn px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 text-gray-700 dark:text-gray-300">
                                            Outlook
                                        </button>
                                        <button type="button" onclick="PulseApp.ui.alertManagementModal.handleEmailProviderSelection('yahoo')" 
                                                class="email-provider-btn px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 text-gray-700 dark:text-gray-300">
                                            Yahoo
                                        </button>
                                        <button type="button" onclick="PulseApp.ui.alertManagementModal.handleEmailProviderSelection('custom')" 
                                                class="email-provider-btn px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 text-gray-700 dark:text-gray-300">
                                            Custom
                                        </button>
                                    </div>
                                </div>

                                <div id="primary-email-config" class="grid grid-cols-1 gap-4">
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">From Email</label>
                                        <input type="email" id="email-from-input" name="ALERT_FROM_EMAIL" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="alerts@yourcompany.com">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">To Email</label>
                                        <input type="email" id="email-to" name="ALERT_TO_EMAIL" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="admin@yourcompany.com">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SMTP Server</label>
                                        <input type="text" id="email-smtp" name="SMTP_HOST" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="smtp.gmail.com">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Port</label>
                                        <input type="number" id="email-port" name="SMTP_PORT" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="587" value="587">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                            <span id="smtp-username-label">Username</span>
                                        </label>
                                        <input type="text" id="email-username" name="SMTP_USER" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="your.email@gmail.com">
                                        <p id="smtp-username-help" class="text-xs text-gray-500 dark:text-gray-400 mt-1">(Usually your email address)</p>
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                            <span id="smtp-password-label">Password</span>
                                        </label>
                                        <input type="password" id="email-password" name="SMTP_PASS" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="Enter password">
                                        <p id="smtp-password-help" class="text-xs text-gray-500 dark:text-gray-400 mt-1">(Your email password)</p>
                                    </div>
                                </div>

                                <!-- App Password Help -->
                                <div id="app-password-help" class="mt-4 p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg hidden">
                                    <div class="flex items-start">
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-yellow-600 dark:text-yellow-400 mr-3 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.962-.833-2.732 0L4.082 18.5c-.77.833.192 2.5 1.732 2.5z" />
                                        </svg>
                                        <div>
                                            <h4 id="app-password-title" class="text-sm font-medium text-yellow-800 dark:text-yellow-200">App Password Required</h4>
                                            <p id="app-password-description" class="text-sm text-yellow-700 dark:text-yellow-300 mt-1">Gmail requires an app password for third-party applications.</p>
                                            <a id="app-password-link" href="#" target="_blank" class="text-sm text-yellow-600 dark:text-yellow-400 underline hover:text-yellow-800 dark:hover:text-yellow-200 mt-2 inline-block">
                                                Generate App Password →
                                            </a>
                                        </div>
                                    </div>
                                </div>

                                <!-- Advanced SMTP Settings -->
                                <div class="mt-4">
                                    <button type="button" id="toggle-advanced-smtp" class="flex items-center text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200">
                                        <svg id="advanced-smtp-icon" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-2 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                        </svg>
                                        Advanced Settings
                                    </button>
                                    
                                    <div id="advanced-smtp-settings" class="mt-3 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg hidden">
                                        <div class="space-y-4">
                                            <div class="flex items-center">
                                                <input type="checkbox" id="smtp-secure" name="SMTP_SECURE" class="h-4 w-4 text-blue-600 border-gray-300 dark:border-gray-600 rounded">
                                                <label for="smtp-secure" class="ml-2 text-sm text-gray-700 dark:text-gray-300">Use SSL/TLS encryption</label>
                                            </div>
                                            <div id="smtp-provider-help" class="text-sm text-gray-600 dark:text-gray-400 hidden">
                                                Configure your email provider settings manually.
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Test Email Button -->
                                <div class="mt-4 flex gap-3">
                                    <button type="button" id="test-email-btn" 
                                            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                                        Test Email
                                    </button>
                                    <button type="button" id="save-email-config-btn" 
                                            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                                        Save Configuration
                                    </button>
                                </div>
                            </div>


                        </div>
                    </div>

                    <!-- Webhook Tab Content (Hidden by default) -->
                    <div id="webhook-notification-content" class="notification-sub-content hidden">
                        <div class="space-y-6">
                            <!-- Primary Webhook Configuration -->
                            <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                                <div class="flex items-center justify-between mb-4">
                                    <div>
                                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Webhook Notifications</h3>
                                        <p class="text-sm text-gray-600 dark:text-gray-400">Primary webhook configuration for external service alerts</p>
                                    </div>
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input type="checkbox" id="webhook-enabled" class="sr-only peer">
                                        <div class="w-11 h-6 bg-gray-300 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-gray-400 after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-500 peer-checked:after:bg-white dark:after:bg-gray-200"></div>
                                    </label>
                                </div>
                                
                                <div id="primary-webhook-config" class="grid grid-cols-1 gap-4">
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Webhook URL</label>
                                        <input type="url" id="webhook-url" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="https://discord.com/api/webhooks/...">
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Service Type</label>
                                        <select id="webhook-service" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
                                            <option value="discord">Discord</option>
                                            <option value="slack">Slack</option>
                                            <option value="teams">Microsoft Teams</option>
                                            <option value="custom">Custom</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Name</label>
                                        <input type="text" id="webhook-name" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="Production Alerts">
                                    </div>
                                </div>
                            </div>

                            <!-- Additional Webhooks -->
                            <div id="additional-webhooks">
                                <div class="flex justify-between items-center mb-4">
                                    <div>
                                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Additional Webhooks</h3>
                                        <p class="text-sm text-gray-600 dark:text-gray-400">Add more webhook endpoints beyond the primary one above</p>
                                    </div>
                                    <button type="button" onclick="PulseApp.ui.alertManagementModal.addWebhookEndpoint()" 
                                            class="flex items-center gap-2 px-3 py-1 bg-purple-600 hover:bg-purple-700 text-white text-sm rounded-md">
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                                        </svg>
                                        Add Webhook
                                    </button>
                                </div>
                                <div id="additional-webhook-list" class="space-y-2">
                                    <div class="text-center py-8 text-gray-500 dark:text-gray-400 italic border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg">
                                        No additional webhooks configured.<br>
                                        <span class="text-sm">Click "Add Webhook" to add more.</span>
                                    </div>
                                </div>
                            </div>

                        </div>
                    </div>
                </div>
            </div>
        `;
    }


    function initializeAlertsTab() {
        // Set up sub-tab navigation
        const subTabs = document.querySelectorAll('.alerts-sub-tab');
        subTabs.forEach(tab => {
            tab.addEventListener('click', (e) => {
                const targetTab = e.currentTarget.dataset.tab;
                switchAlertsSubTab(targetTab);
            });
        });

        // Set up refresh button
        const refreshBtn = document.getElementById('refresh-alerts-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => {
                const activeSubTab = document.querySelector('.alerts-sub-tab.active')?.dataset.tab || 'current';
                if (activeSubTab === 'current') {
                    loadCurrentAlerts();
                } else if (activeSubTab === 'history') {
                    loadAlertHistory();
                }
            });
        }

        // Set up action buttons
        const acknowledgeAllBtn = document.getElementById('acknowledge-all-alerts-btn');
        if (acknowledgeAllBtn) {
            acknowledgeAllBtn.addEventListener('click', () => {
                if (PulseApp.alerts && PulseApp.alerts.markAllAsAcknowledged) {
                    PulseApp.alerts.markAllAsAcknowledged();
                    // Socket events will handle the real-time updates
                }
            });
        }

        const clearHistoryBtn = document.getElementById('clear-history-btn');
        if (clearHistoryBtn) {
            clearHistoryBtn.addEventListener('click', clearAlertHistory);
        }


        // Load current alerts by default
        loadCurrentAlerts();
    }

    function switchAlertsSubTab(tabName) {
        // Update tab buttons
        const subTabs = document.querySelectorAll('.alerts-sub-tab');
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

        // Show/hide content sections
        const currentContent = document.getElementById('current-alerts-content');
        const historyContent = document.getElementById('alert-history-content');
        const acknowledgeAllBtn = document.getElementById('acknowledge-all-alerts-btn');
        const clearHistoryBtn = document.getElementById('clear-history-btn');

        if (tabName === 'current') {
            currentContent?.classList.remove('hidden');
            historyContent?.classList.add('hidden');
            acknowledgeAllBtn?.classList.remove('hidden');
            clearHistoryBtn?.classList.add('hidden');
            loadCurrentAlerts();
        } else if (tabName === 'history') {
            currentContent?.classList.add('hidden');
            historyContent?.classList.remove('hidden');
            acknowledgeAllBtn?.classList.add('hidden');
            clearHistoryBtn?.classList.remove('hidden');
            loadAlertHistory();
        }
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

        // System alert toggles are now handled via the unified alert system
        
        // Load system and custom alerts
        // Ensure we have the latest configuration before loading alerts
        loadConfiguration().then(() => {
            loadSystemAlerts();
            loadCustomAlerts();
            // Also reload email configuration after config is loaded
            if (activeTab === 'notifications') {
                loadEmailConfiguration();
            }
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

        // Email provider selection is handled via inline onclick handlers in HTML

        // Set up auto-detection when email address is entered
        const emailFromInput = document.getElementById('email-from-input');
        if (emailFromInput) {
            emailFromInput.addEventListener('blur', () => {
                autoDetectEmailProvider(emailFromInput.value);
                
                // Auto-fill username with email address for common providers
                const usernameInput = document.querySelector('input[name="SMTP_USER"]');
                if (usernameInput && emailFromInput.value && emailFromInput.value.includes('@')) {
                    const domain = emailFromInput.value.split('@')[1].toLowerCase();
                    const commonProviders = ['gmail.com', 'googlemail.com', 'outlook.com', 'hotmail.com', 'live.com', 'msn.com', 'yahoo.com', 'yahoo.co.uk', 'yahoo.ca', 'ymail.com'];
                    if (commonProviders.includes(domain)) {
                        usernameInput.value = emailFromInput.value;
                    }
                }
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
        const emailToggle = document.getElementById('email-enabled');
        if (emailToggle) {
            emailToggle.addEventListener('change', async (e) => {
                const enabled = e.target.checked;
                console.log('[Email Toggle] Setting GLOBAL_EMAIL_ENABLED to:', enabled);
                
                try {
                    const response = await fetch('/api/config', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            GLOBAL_EMAIL_ENABLED: enabled ? 'true' : 'false'
                        })
                    });
                    
                    if (response.ok) {
                        console.log('[Email Toggle] Successfully saved global email setting');
                        PulseApp.ui.toast.success(`Email notifications ${enabled ? 'enabled' : 'disabled'}`);
                    } else {
                        console.error('[Email Toggle] Failed to save setting');
                        PulseApp.ui.toast.error('Failed to save email setting');
                        // Revert toggle on failure
                        e.target.checked = !enabled;
                    }
                } catch (error) {
                    console.error('[Email Toggle] Error saving setting:', error);
                    PulseApp.ui.toast.error('Failed to save email setting');
                    // Revert toggle on failure
                    e.target.checked = !enabled;
                }
            });
        }
        
        const webhookToggle = document.getElementById('webhook-enabled');
        if (webhookToggle) {
            webhookToggle.addEventListener('change', async (e) => {
                const enabled = e.target.checked;
                console.log('[Webhook Toggle] Setting GLOBAL_WEBHOOK_ENABLED to:', enabled);
                
                try {
                    const response = await fetch('/api/config', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            GLOBAL_WEBHOOK_ENABLED: enabled ? 'true' : 'false'
                        })
                    });
                    
                    if (response.ok) {
                        console.log('[Webhook Toggle] Successfully saved global webhook setting');
                        PulseApp.ui.toast.success(`Webhook notifications ${enabled ? 'enabled' : 'disabled'}`);
                    } else {
                        console.error('[Webhook Toggle] Failed to save setting');
                        PulseApp.ui.toast.error('Failed to save webhook setting');
                        // Revert toggle on failure
                        e.target.checked = !enabled;
                    }
                } catch (error) {
                    console.error('[Webhook Toggle] Error saving setting:', error);
                    PulseApp.ui.toast.error('Failed to save webhook setting');
                    // Revert toggle on failure
                    e.target.checked = !enabled;
                }
            });
        }

        // Load existing email configuration
        loadEmailConfiguration();
    }

    // Helper function to check if modal is open
    function isModalOpen() {
        const modal = document.getElementById('alert-management-modal');
        return modal && !modal.classList.contains('hidden');
    }

    // Helper function to count active alerts for a rule
    function getActiveAlertCountForRule(ruleId) {
        if (!PulseApp.alerts || !PulseApp.alerts.getActiveAlerts) {
            return 0;
        }
        const activeAlerts = PulseApp.alerts.getActiveAlerts();
        return activeAlerts.filter(alert => alert.ruleId === ruleId).length;
    }

    // Helper function to format current value for display
    function formatCurrentValue(currentValue, alertType) {
        if (!currentValue) {
            return 'N/A';
        }
        
        // For compound threshold alerts, currentValue is an object like {cpu: 6.6}
        if (alertType === 'compound_threshold' && typeof currentValue === 'object') {
            const values = [];
            for (const [metric, value] of Object.entries(currentValue)) {
                const isPercentage = ['cpu', 'memory', 'disk'].includes(metric);
                const formattedValue = typeof value === 'number' ? Math.round(value * 10) / 10 : value;
                const unit = isPercentage ? '%' : '';
                values.push(`${metric.toUpperCase()}: ${formattedValue}${unit}`);
            }
            return values.join(', ');
        }
        
        // For single metric alerts, return as-is
        return currentValue.toString();
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
        
        // Define system alerts with their configuration using actual rule IDs from AlertManager
        const systemAlerts = [
            {
                id: 'cpu_usage_warning',
                name: 'CPU Alert',
                description: 'Triggers when CPU usage exceeds threshold',
                threshold: currentConfig?.advanced?.alerts?.cpu?.threshold || 85,
                enabled: currentConfig?.advanced?.alerts?.cpu?.enabled !== false
            },
            {
                id: 'memory_usage_warning', 
                name: 'Memory Alert',
                description: 'Triggers when memory usage exceeds threshold',
                threshold: currentConfig?.advanced?.alerts?.memory?.threshold || 90,
                enabled: currentConfig?.advanced?.alerts?.memory?.enabled !== false
            },
            {
                id: 'disk_space_warning',
                name: 'Disk Alert', 
                description: 'Triggers when disk usage exceeds threshold',
                threshold: currentConfig?.advanced?.alerts?.disk?.threshold || 95,
                enabled: currentConfig?.advanced?.alerts?.disk?.enabled !== false
            },
            {
                id: 'disk_space_critical',
                name: 'Critical Disk Alert',
                description: 'Triggers when disk usage reaches critical levels',
                threshold: 95,
                enabled: currentConfig?.advanced?.alerts?.disk?.enabled !== false
            },
            {
                id: 'guest_down',
                name: 'Down Alert',
                description: 'Triggers when VM/LXC becomes unreachable or stops responding',
                enabled: currentConfig?.advanced?.alerts?.down?.enabled !== false
            }
        ];
        
        // Generate HTML for all system alerts using unified renderer
        systemAlertsContent.innerHTML = systemAlerts.map(alert => createAlertCard({...alert, type: 'system'})).join('');
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
            
            // Display the custom alerts using unified renderer
            customAlertsContent.innerHTML = customAlerts.map(alert => createAlertCard({...alert, type: 'custom'})).join('');
            
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

    // Unified alert card renderer for all alerts
    function createAlertCard(alert) {
        const isEnabled = alert.enabled !== false;
        
        // Format metadata line
        const metadataLine = alert.group === 'system_performance' || alert.group === 'storage_alerts' || alert.group === 'availability_alerts'
            ? 'System alert rule • Target: All VMs/LXCs'
            : `Created: ${alert.createdAt ? new Date(alert.createdAt).toLocaleDateString() : 'Unknown'} • Target: ${getTargetDisplay(alert)}`;
        
        // Generate action buttons - all rules use same button style now
        const actionButtons = getSystemAlertButtons(alert);
        
        // Generate alert content (conditions/description)
        const alertContent = getSystemAlertContent(alert);
            
        // Get active alert count for this rule
        const activeCount = getActiveAlertCountForRule(alert.id);
        const countBadge = activeCount > 0 
            ? `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200">${activeCount}</span>`
            : '';
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800 ${!isEnabled ? 'opacity-50' : ''}">
                <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between mb-2">
                    <div class="flex-1 mb-2 sm:mb-0">
                        <div class="flex flex-wrap items-center gap-2 mb-1">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">${alert.name || alert.description || 'Unnamed Alert'}</h5>
                            ${countBadge}
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            ${metadataLine}
                        </div>
                    </div>
                    <div class="flex gap-1 flex-wrap">
                        ${actionButtons}
                    </div>
                </div>
                <div>
                    ${alertContent}
                </div>
            </div>
        `;
    }
    
    function getTargetDisplay(alert) {
        return alert.targetType === 'all' ? 'All VMs/LXCs' :
               alert.targetType === 'vm' ? 'VMs only' :
               alert.targetType === 'lxc' ? 'LXCs only' :
               alert.specificTarget || alert.targetType?.toUpperCase() || 'All';
    }
    
    function getSystemAlertButtons(alert) {
        const editButton = alert.threshold ? `
            <button onclick="editAlertRule('${alert.id}')" 
                    class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded whitespace-nowrap">
                Edit
            </button>
        ` : '';
        
        return `
            ${editButton}
            <button onclick="PulseApp.ui.alertManagementModal.toggleAlert('${alert.id}', 'system', ${!alert.enabled})" 
                    class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded whitespace-nowrap">
                ${alert.enabled ? 'Disable' : 'Enable'}
            </button>
        `;
    }
    
    function getCustomAlertButtons(alert) {
        return `
            <button onclick="PulseApp.ui.alertManagementModal.editCustomAlert('${alert.id}')" 
                    class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded whitespace-nowrap">
                Edit
            </button>
            <button onclick="PulseApp.ui.alertManagementModal.toggleAlert('${alert.id}', 'custom', ${alert.enabled === false})" 
                    class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded whitespace-nowrap">
                ${alert.enabled !== false ? 'Disable' : 'Enable'}
            </button>
            <button onclick="PulseApp.ui.alertManagementModal.deleteCustomAlert('${alert.id}')" 
                    class="px-2 py-1 text-xs bg-red-600 hover:bg-red-700 text-white rounded whitespace-nowrap">
                Delete
            </button>
        `;
    }
    
    function getSystemAlertContent(alert) {
        const thresholdBadge = alert.threshold ? 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${alert.name.split(' ')[0]} ≥ ${alert.threshold}%
            </span>` : '';
        
        // Check current global notification settings
        const emailEnabled = currentConfig?.GLOBAL_EMAIL_ENABLED === 'true' || currentConfig?.GLOBAL_EMAIL_ENABLED === true;
        const webhookEnabled = currentConfig?.GLOBAL_WEBHOOK_ENABLED === 'true' || currentConfig?.GLOBAL_WEBHOOK_ENABLED === true;
        
        return `
            <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">${alert.description}</p>
            ${thresholdBadge}
            <div class="text-xs text-gray-600 dark:text-gray-400 mt-2">
                <strong>Notifications:</strong>
                <div class="flex items-center gap-1 mt-1">
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300">
                        Pulse UI
                    </span>
                    ${emailEnabled ? '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300">Email</span>' : ''}
                    ${webhookEnabled ? '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300">Webhook</span>' : ''}
                </div>
            </div>
        `;
    }
    
    function getCustomAlertContent(alert) {
        const thresholdsList = alert.thresholds?.map(t => 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${t.metric.charAt(0).toUpperCase() + t.metric.slice(1)} ≥ ${t.threshold}${['cpu', 'memory', 'disk'].includes(t.metric) ? '%' : ''}
            </span>`
        ).join('') || '';
        
        return `
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
                    ${(() => {
                        const globalEmailEnabled = currentConfig?.GLOBAL_EMAIL_ENABLED === 'true' || currentConfig?.GLOBAL_EMAIL_ENABLED === true;
                        const ruleEmailEnabled = alert.sendEmail === true;
                        const emailActive = globalEmailEnabled && ruleEmailEnabled;
                        
                        if (globalEmailEnabled && ruleEmailEnabled) {
                            return '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300">Email</span>';
                        } else {
                            return '';
                        }
                    })()}
                    ${(() => {
                        const globalWebhookEnabled = currentConfig?.GLOBAL_WEBHOOK_ENABLED === 'true' || currentConfig?.GLOBAL_WEBHOOK_ENABLED === true;
                        const ruleWebhookEnabled = alert.sendWebhook === true;
                        const webhookActive = globalWebhookEnabled && ruleWebhookEnabled;
                        
                        if (globalWebhookEnabled && ruleWebhookEnabled) {
                            return '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300">Webhook</span>';
                        } else {
                            return '';
                        }
                    })()}
                </div>
            </div>
        `;
    }

    // Legacy wrapper - now uses unified system
    function createCustomAlertCard(alert) {
        return createAlertCard({...alert, type: 'custom'});
    }

    // Legacy wrapper - now uses unified system
    function createSystemAlertCard(alert) {
        return createAlertCard({...alert, type: 'system'});
    }
    
    function updateNotificationCheckboxes() {
        const emailLabel = document.getElementById('email-notification-label');
        const webhookLabel = document.getElementById('webhook-notification-label');
        const helpText = document.getElementById('notification-help-text');
        
        if (!emailLabel || !webhookLabel) return;
        
        const emailCheckbox = emailLabel.querySelector('input[name="sendEmail"]');
        const webhookCheckbox = webhookLabel.querySelector('input[name="sendWebhook"]');
        const emailBadge = emailLabel.querySelector('span:first-of-type');
        const webhookBadge = webhookLabel.querySelector('span:first-of-type');
        const emailText = emailLabel.querySelector('span:last-of-type');
        const webhookText = webhookLabel.querySelector('span:last-of-type');
        
        const globalEmailEnabled = currentConfig?.GLOBAL_EMAIL_ENABLED === 'true' || currentConfig?.GLOBAL_EMAIL_ENABLED === true;
        const globalWebhookEnabled = currentConfig?.GLOBAL_WEBHOOK_ENABLED === 'true' || currentConfig?.GLOBAL_WEBHOOK_ENABLED === true;
        
        // Update email checkbox
        if (!globalEmailEnabled) {
            emailLabel.classList.add('opacity-50');
            emailCheckbox.disabled = true;
            emailBadge.className = 'text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 rounded font-medium mr-2';
            emailText.className = 'text-sm text-gray-500 dark:text-gray-400';
        } else {
            emailLabel.classList.remove('opacity-50');
            emailCheckbox.disabled = false;
            emailBadge.className = 'text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded font-medium mr-2';
            emailText.className = 'text-sm text-gray-700 dark:text-gray-300';
        }
        
        // Update webhook checkbox
        if (!globalWebhookEnabled) {
            webhookLabel.classList.add('opacity-50');
            webhookCheckbox.disabled = true;
            webhookBadge.className = 'text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 rounded font-medium mr-2';
            webhookText.className = 'text-sm text-gray-500 dark:text-gray-400';
        } else {
            webhookLabel.classList.remove('opacity-50');
            webhookCheckbox.disabled = false;
            webhookBadge.className = 'text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded font-medium mr-2';
            webhookText.className = 'text-sm text-gray-700 dark:text-gray-300';
        }
        
        // Update help text
        if (!globalEmailEnabled && !globalWebhookEnabled) {
            helpText.textContent = 'Global notification settings must be enabled first';
            helpText.className = 'text-xs text-red-500 dark:text-red-400';
        } else if (!globalEmailEnabled || !globalWebhookEnabled) {
            helpText.textContent = 'Only enabled global notification methods will work';
            helpText.className = 'text-xs text-yellow-600 dark:text-yellow-400';
        } else {
            helpText.textContent = 'Notifications will be sent when alerts trigger';
            helpText.className = 'text-xs text-gray-500 dark:text-gray-400';
        }
    }

    function loadEmailConfiguration() {
        console.log('[Email Config] loadEmailConfiguration called');
        
        // Check if we're in the configure tab (different structure)
        const isConfigureTab = document.getElementById('email-config-section');
        if (!isConfigureTab) {
            console.log('[Email Config] Not in configure tab, looking for primary-email-config');
            const emailConfigSection = document.getElementById('primary-email-config');
            if (!emailConfigSection) {
                console.log('[Email Config] primary-email-config not found either, returning');
                return;
            }
        } else {
            console.log('[Email Config] In configure tab');
        }
        
        // Get email configuration from currentConfig
        const config = currentConfig || {};
        const smtp = config.advanced?.smtp || {};
        
        console.log('[Email Config] Loading email configuration:', smtp);
        console.log('[Email Config] Full config:', config);
        console.log('[Email Config] Direct config keys:', {
            SMTP_HOST: config.SMTP_HOST,
            SMTP_PORT: config.SMTP_PORT,
            SMTP_USER: config.SMTP_USER,
            ALERT_FROM_EMAIL: config.ALERT_FROM_EMAIL,
            ALERT_TO_EMAIL: config.ALERT_TO_EMAIL,
            GLOBAL_EMAIL_ENABLED: config.GLOBAL_EMAIL_ENABLED
        });
        
        // Set toggle states based on current config
        const emailToggle = document.getElementById('email-enabled') || document.getElementById('global-email-toggle');
        if (emailToggle) {
            const emailEnabled = config.GLOBAL_EMAIL_ENABLED === 'true' || config.GLOBAL_EMAIL_ENABLED === true;
            emailToggle.checked = emailEnabled;
            console.log('[Email Config] Email toggle set to:', emailEnabled);
        } else {
            console.log('[Email Config] Email toggle not found (looking for email-enabled or global-email-toggle)');
        }
        
        const webhookToggle = document.getElementById('webhook-enabled') || document.getElementById('global-webhook-toggle');
        if (webhookToggle) {
            const webhookEnabled = config.GLOBAL_WEBHOOK_ENABLED === 'true' || config.GLOBAL_WEBHOOK_ENABLED === true;
            webhookToggle.checked = webhookEnabled;
        }
        
        // Populate email fields with values from configuration
        // Check both nested (smtp) and direct config locations
        const emailFromInput = document.querySelector('input[name="ALERT_FROM_EMAIL"]');
        const fromValue = config.ALERT_FROM_EMAIL || smtp.from;
        console.log('[Email Config] From input found:', !!emailFromInput, 'Value to set:', fromValue);
        if (emailFromInput && fromValue) {
            emailFromInput.value = fromValue;
        }
        
        const emailToInput = document.querySelector('input[name="ALERT_TO_EMAIL"]');
        const toValue = config.ALERT_TO_EMAIL || smtp.to;
        console.log('[Email Config] To input found:', !!emailToInput, 'Value to set:', toValue);
        if (emailToInput && toValue) {
            emailToInput.value = toValue;
        }
        
        // Populate SMTP fields
        const smtpHostInput = document.querySelector('input[name="SMTP_HOST"]');
        const hostValue = config.SMTP_HOST || smtp.host;
        console.log('[Email Config] Host input found:', !!smtpHostInput, 'Value to set:', hostValue);
        if (smtpHostInput && hostValue) {
            smtpHostInput.value = hostValue;
        }
        
        const smtpPortInput = document.querySelector('input[name="SMTP_PORT"]');
        const portValue = config.SMTP_PORT || smtp.port;
        console.log('[Email Config] Port input found:', !!smtpPortInput, 'Value to set:', portValue);
        if (smtpPortInput && portValue) {
            smtpPortInput.value = portValue;
        }
        
        const smtpUserInput = document.querySelector('input[name="SMTP_USER"]');
        const userValue = config.SMTP_USER || smtp.user;
        console.log('[Email Config] User input found:', !!smtpUserInput, 'Value to set:', userValue);
        if (smtpUserInput && userValue) {
            smtpUserInput.value = userValue;
        }
        
        const smtpSecureCheckbox = document.querySelector('input[name="SMTP_SECURE"]');
        const secureValue = config.SMTP_SECURE || smtp.secure;
        if (smtpSecureCheckbox) {
            smtpSecureCheckbox.checked = secureValue === true || secureValue === 'true';
        }
        
        // Note: SMTP_PASS is not returned by the API for security reasons
        // Show a placeholder to indicate it's already set
        const smtpPassInput = document.querySelector('input[name="SMTP_PASS"]');
        const hasSmtpConfig = hostValue || smtp.host;
        if (smtpPassInput && hasSmtpConfig) {
            smtpPassInput.placeholder = '••••••••• (configured)';
        }
        
        // Populate webhook configuration
        const webhookUrlInput = document.querySelector('input[name="WEBHOOK_URL"]');
        if (webhookUrlInput && config.WEBHOOK_URL) {
            webhookUrlInput.value = config.WEBHOOK_URL;
        }
    }

    function markEmailConfigAsSaved() {
        // Just show a brief success indicator on the save button itself
        const saveBtn = document.getElementById('save-email-config-btn');
        if (saveBtn) {
            const originalText = saveBtn.textContent;
            const originalClass = saveBtn.className;
            
            saveBtn.className = 'px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-md transition-colors';
            saveBtn.innerHTML = `
                <svg class="w-4 h-4 inline mr-2" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                </svg>
                Saved
            `;
            
            // Reset after 2 seconds
            setTimeout(() => {
                saveBtn.textContent = originalText;
                saveBtn.className = originalClass;
            }, 2000);
        }
    }

    function markEmailTestAsSuccessful() {
        const testBtn = document.getElementById('test-email-btn');
        if (testBtn) {
            const originalText = testBtn.textContent;
            testBtn.className = 'px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-md transition-colors';
            testBtn.innerHTML = `
                <svg class="w-4 h-4 inline mr-2" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                </svg>
                Email Test Successful
            `;
            
            // Reset after 3 seconds
            setTimeout(() => {
                testBtn.textContent = originalText;
                testBtn.className = 'px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors';
            }, 3000);
        }
    }

    function updateSSLHelp(provider, config) {
        const sslLabel = document.querySelector('label[for="smtp-secure"]');
        const sslCheckbox = document.querySelector('input[name="SMTP_SECURE"]');
        const advancedSettings = document.getElementById('advanced-smtp-settings');
        
        if (sslLabel && sslCheckbox) {
            // Update label text based on provider
            if (provider === 'gmail') {
                sslLabel.innerHTML = 'Use SSL/TLS encryption <span class="text-xs text-gray-500">(Gmail uses STARTTLS - leave unchecked)</span>';
            } else if (provider === 'outlook' || provider === 'yahoo') {
                sslLabel.innerHTML = 'Use SSL/TLS encryption <span class="text-xs text-gray-500">(recommended for port 465)</span>';
            } else {
                sslLabel.textContent = 'Use SSL/TLS encryption';
            }
        }
    }

    function updatePasswordPlaceholder(provider) {
        const smtpPassInput = document.querySelector('input[name="SMTP_PASS"]');
        if (!smtpPassInput) return;
        
        // Get current SMTP configuration
        const config = currentConfig || {};
        const smtp = config.advanced?.smtp || {};
        
        // Check if current provider matches the saved configuration
        const providerConfigs = {
            gmail: { host: 'smtp.gmail.com', port: '587' },
            outlook: { host: 'smtp-mail.outlook.com', port: '587' },
            yahoo: { host: 'smtp.mail.yahoo.com', port: '587' },
            custom: null // Custom doesn't have a specific config to match
        };
        
        const providerConfig = providerConfigs[provider];
        
        // For custom, check if there's SMTP config that's NOT a known provider
        const hasMatchingConfig = provider === 'custom' 
            ? (smtp.host && smtp.user && 
               smtp.host !== 'smtp.gmail.com' && 
               smtp.host !== 'smtp-mail.outlook.com' && 
               smtp.host !== 'smtp.mail.yahoo.com') // Custom config that's not a known provider
            : (smtp.host === providerConfig?.host && smtp.port === providerConfig?.port); // Exact match for specific providers
        
        
        // Check if we're switching away from a configured provider
        const wasPreviouslyConfigured = smtpPassInput.placeholder.includes('Password saved');
        
        if (hasMatchingConfig) {
            smtpPassInput.placeholder = 'Password saved (click to change)';
            // Don't clear the value for configured providers
        } else {
            smtpPassInput.placeholder = provider === 'gmail' ? 'Enter app password' : 'Enter password';
            // Only clear value if we were previously showing a configured provider
            if (wasPreviouslyConfigured) {
                smtpPassInput.value = '';
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
        
        if (!provider) {
            console.error('[ERROR] Provider is undefined in handleEmailProviderSelection');
            return;
        }
        
        // Remove active state from all buttons
        document.querySelectorAll('.email-provider-btn').forEach(btn => {
            btn.classList.remove('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
        });
        
        // Add active state to selected button - find by text content since we don't have data-provider
        const buttons = document.querySelectorAll('.email-provider-btn');
        buttons.forEach(btn => {
            if (btn.textContent && btn.textContent.trim().toLowerCase() === provider.toLowerCase()) {
                btn.classList.add('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
            }
        });
        
        // Update form fields based on provider
        const passwordLabel = document.getElementById('smtp-password-label');
        const passwordHelp = document.getElementById('smtp-password-help');
        const usernameLabel = document.getElementById('smtp-username-label');
        const usernameHelp = document.getElementById('smtp-username-help');
        const hostInput = document.querySelector('input[name="SMTP_HOST"]');
        const portInput = document.querySelector('input[name="SMTP_PORT"]');
        const secureCheckbox = document.querySelector('input[name="SMTP_SECURE"]');
        
        const providers = {
            gmail: {
                host: 'smtp.gmail.com',
                port: 587,
                secure: false, // Gmail uses STARTTLS on port 587, not SSL/TLS
                passwordLabel: 'App Password',
                passwordHelp: '(Required - generate from Google Account settings)',
                help: 'Gmail requires 2-factor authentication and an App Password. Regular passwords won\'t work for third-party apps.',
                appPasswordUrl: 'https://myaccount.google.com/apppasswords',
                appPasswordTitle: 'Gmail App Password Required',
                appPasswordDesc: 'Gmail requires an app password for third-party applications. Enable 2FA first, then generate an app password from your Google Account security settings.'
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
            if (hostInput) {
                hostInput.value = config.host;
                // Update placeholder for custom provider
                if (provider === 'custom') {
                    hostInput.placeholder = 'smtp.yourprovider.com';
                } else {
                    hostInput.placeholder = config.host || 'smtp.gmail.com';
                }
            }
            if (portInput) portInput.value = config.port;
            if (secureCheckbox) secureCheckbox.checked = config.secure;
            if (passwordLabel) passwordLabel.textContent = config.passwordLabel;
            if (passwordHelp) passwordHelp.textContent = config.passwordHelp;
            
            // Update username label and help based on provider
            if (usernameLabel) {
                usernameLabel.textContent = provider === 'custom' ? 'Username' : 'Email Address';
            }
            if (usernameHelp) {
                const usernameHelpText = provider === 'custom' 
                    ? '(SMTP authentication username)'
                    : '(Your email address)';
                usernameHelp.textContent = usernameHelpText;
            }

            // Show app password help for providers that need it
            showAppPasswordHelp(config);
            
            // Add provider-specific SSL/TLS explanation
            updateSSLHelp(provider, config);
            
            // Update password placeholder based on provider
            updatePasswordPlaceholder(provider);
            
            // Update username placeholder and auto-fill
            const usernameInput = document.querySelector('input[name="SMTP_USER"]');
            const fromEmailInput = document.getElementById('email-from-input');
            
            if (usernameInput) {
                // Update placeholder based on provider
                const placeholders = {
                    gmail: 'your.email@gmail.com',
                    outlook: 'your.email@outlook.com',
                    yahoo: 'your.email@yahoo.com',
                    custom: 'username'
                };
                usernameInput.placeholder = placeholders[provider] || placeholders.custom;
                
                // Auto-fill username with from email if available
                if (fromEmailInput && fromEmailInput.value && provider !== 'custom') {
                    usernameInput.value = fromEmailInput.value;
                }
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
            pass: document.querySelector('input[name="SMTP_PASS"]')?.value, // Backend expects 'pass' not 'password'
            host: document.querySelector('input[name="SMTP_HOST"]')?.value,
            port: document.querySelector('input[name="SMTP_PORT"]')?.value,
            user: document.querySelector('input[name="SMTP_USER"]')?.value,
            secure: document.querySelector('input[name="SMTP_SECURE"]')?.checked
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
                markEmailTestAsSuccessful();
            } else {
                PulseApp.ui.toast.error('Test email failed: ' + (result.error || 'Unknown error'));
            }
        } catch (error) {
            console.error('[Email Test] Error:', error);
            PulseApp.ui.toast.error('Error sending test email: ' + error.message);
        }
    }
    
    async function saveEmailConfiguration() {
        const saveBtn = document.getElementById('save-email-config-btn');
        if (!saveBtn) return;
        
        const originalText = saveBtn.textContent;
        saveBtn.disabled = true;
        saveBtn.textContent = 'Saving...';
        
        try {
            // Validate SMTP configuration
            const validationErrors = validateSMTPConfiguration();
            if (validationErrors.length > 0) {
                const errorMessage = 'Please fix the following errors:\n\n• ' + validationErrors.join('\n• ');
                alert(errorMessage);
                saveBtn.textContent = originalText;
                saveBtn.disabled = false;
                return;
            }
            
            // Collect email configuration
            const configToSave = {};
            const smtpHost = document.querySelector('input[name="SMTP_HOST"]')?.value;
            const smtpPort = document.querySelector('input[name="SMTP_PORT"]')?.value;
            const smtpUser = document.querySelector('input[name="SMTP_USER"]')?.value;
            const smtpPass = document.querySelector('input[name="SMTP_PASS"]')?.value;
            const fromEmail = document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value;
            const toEmail = document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value;
            const smtpSecure = document.querySelector('input[name="SMTP_SECURE"]')?.checked;
            const emailEnabled = (document.getElementById('email-enabled') || document.getElementById('global-email-toggle'))?.checked;
            
            // Only include values that are set
            if (smtpHost) configToSave.SMTP_HOST = smtpHost;
            if (smtpPort) configToSave.SMTP_PORT = smtpPort;
            if (smtpUser) configToSave.SMTP_USER = smtpUser;
            if (smtpPass) configToSave.SMTP_PASS = smtpPass;
            if (fromEmail) configToSave.ALERT_FROM_EMAIL = fromEmail;
            if (toEmail) configToSave.ALERT_TO_EMAIL = toEmail;
            configToSave.SMTP_SECURE = smtpSecure ? 'true' : 'false';
            configToSave.GLOBAL_EMAIL_ENABLED = emailEnabled ? 'true' : 'false';
            
            // Save via API
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(configToSave)
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast.success('Email configuration saved successfully!');
                await loadConfiguration(); // Reload to reflect changes
                
                // Update button to show success
                saveBtn.textContent = 'Saved!';
                saveBtn.classList.remove('bg-blue-600', 'hover:bg-blue-700');
                saveBtn.classList.add('bg-green-600');
                
                setTimeout(() => {
                    saveBtn.textContent = originalText;
                    saveBtn.classList.remove('bg-green-600');
                    saveBtn.classList.add('bg-blue-600', 'hover:bg-blue-700');
                    saveBtn.disabled = false;
                }, 2000);
            } else {
                throw new Error(result.error || 'Failed to save configuration');
            }
        } catch (error) {
            console.error('Error saving email configuration:', error);
            PulseApp.ui.toast.error('Failed to save email configuration');
            saveBtn.textContent = originalText;
            saveBtn.disabled = false;
        }
    }
    
    async function saveWebhookConfiguration() {
        const saveBtn = document.getElementById('save-webhook-btn');
        if (!saveBtn) return;
        
        const originalText = saveBtn.textContent;
        saveBtn.disabled = true;
        saveBtn.textContent = 'Saving...';
        
        try {
            // Collect webhook configuration
            const configToSave = {};
            const webhookUrl = document.querySelector('input[name="WEBHOOK_URL"]')?.value;
            const webhookEnabled = (document.getElementById('webhook-enabled') || document.getElementById('global-webhook-toggle'))?.checked;
            
            configToSave.WEBHOOK_URL = webhookUrl || '';
            configToSave.GLOBAL_WEBHOOK_ENABLED = webhookEnabled ? 'true' : 'false';
            
            // Save via API
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(configToSave)
            });
            
            const result = await response.json();
            
            if (response.ok && result.success) {
                PulseApp.ui.toast.success('Webhook configuration saved successfully!');
                await loadConfiguration(); // Reload to reflect changes
                
                // Update button to show success
                saveBtn.textContent = 'Saved!';
                saveBtn.classList.remove('bg-blue-600', 'hover:bg-blue-700');
                saveBtn.classList.add('bg-green-600');
                
                setTimeout(() => {
                    saveBtn.textContent = originalText;
                    saveBtn.classList.remove('bg-green-600');
                    saveBtn.classList.add('bg-blue-600', 'hover:bg-blue-700');
                    saveBtn.disabled = false;
                }, 2000);
            } else {
                throw new Error(result.error || 'Failed to save configuration');
            }
        } catch (error) {
            console.error('Error saving webhook configuration:', error);
            PulseApp.ui.toast.error('Failed to save webhook configuration');
            saveBtn.textContent = originalText;
            saveBtn.disabled = false;
        }
    }

    // Old system alert functions removed - now using unified editAlertRule
    
    // Old system alert status and display functions removed - now using unified approach

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
            <div id="custom-alert-modal" class="fixed inset-0 z-50 flex items-start sm:items-center justify-center bg-black bg-opacity-50 pt-4 sm:pt-0">
                <div class="modal-content bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md max-h-[95vh] sm:max-h-[90vh] flex flex-col m-2 sm:m-4">
                    <div class="modal-header flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-4 sm:px-6 py-3 sm:py-4">
                        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">${isEditing ? 'Edit Custom Alert' : 'Create Custom Alert'}</h2>
                        <button id="custom-alert-modal-close" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    
                    <div class="overflow-y-auto flex-grow p-4 sm:p-6 scrollbar">
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
                                    Alert Notifications
                                </label>
                                <div class="space-y-2">
                                    <label class="flex items-center" id="email-notification-label">
                                        <input type="checkbox" name="sendEmail" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                        <span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded font-medium mr-2">Email</span>
                                        <span class="text-sm text-gray-700 dark:text-gray-300">Send email notifications</span>
                                    </label>
                                    <label class="flex items-center" id="webhook-notification-label">
                                        <input type="checkbox" name="sendWebhook" class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                                        <span class="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded font-medium mr-2">Webhook</span>
                                        <span class="text-sm text-gray-700 dark:text-gray-300">Send webhook notifications</span>
                                    </label>
                                    <p class="text-xs text-gray-500 dark:text-gray-400" id="notification-help-text">Note: Global notification settings must also be enabled for these to work</p>
                                </div>
                            </div>

                            <div>
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                    Alert Resolution Behavior
                                </label>
                                <div class="space-y-2">
                                    <label class="flex items-start">
                                        <input type="checkbox" name="autoResolve" checked class="mr-3 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded mt-0.5">
                                        <div>
                                            <span class="text-sm text-gray-700 dark:text-gray-300 font-medium">Auto-resolve when condition clears</span>
                                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                                When checked, alerts automatically resolve when the condition is no longer met (e.g., CPU drops below threshold).
                                                When unchecked, alerts remain active until manually acknowledged, creating a permanent record.
                                            </p>
                                        </div>
                                    </label>
                                </div>
                            </div>
                        </form>
                    </div>
                    
                    <div class="modal-footer border-t border-gray-300 dark:border-gray-700 px-4 sm:px-6 py-3 sm:py-4">
                        <div class="flex gap-2 sm:gap-3 justify-end">
                            <button type="button" id="custom-alert-cancel-button" class="px-3 sm:px-4 py-2 bg-gray-500 hover:bg-gray-600 text-white text-sm font-medium rounded-md transition-colors">
                                Cancel
                            </button>
                            <button type="button" id="custom-alert-save-button" class="px-3 sm:px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
${isEditing ? 'Update Alert' : 'Create Alert'}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHTML);
        
        // Update notification checkboxes based on global settings
        updateNotificationCheckboxes();
        
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
            
            // Pre-populate auto-resolve checkbox
            const autoResolveCheckbox = document.querySelector('input[name="autoResolve"]');
            if (autoResolveCheckbox && typeof existingAlert.autoResolve === 'boolean') {
                autoResolveCheckbox.checked = existingAlert.autoResolve;
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
        
        // Always collect thresholds from current form state (ignore preset data when editing)
        // This ensures user changes to sliders are respected
        {
            // Collect thresholds from the simplified form
            const cpuThreshold = parseFloat(formData.get('cpuThreshold')) || 0;
            const memoryThreshold = parseFloat(formData.get('memoryThreshold')) || 0;
            const diskThreshold = parseFloat(formData.get('diskThreshold')) || 0;
            const networkThreshold = parseFloat(formData.get('networkThreshold')) || 0;
            const statusMonitoring = formData.has('statusMonitoring');
            
            // Build thresholds array - ONLY include actively configured thresholds
            // Clear any existing thresholds and rebuild from scratch
            thresholds = [];
            
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
            autoResolve: formData.has('autoResolve'), // User-configurable auto-resolve
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
            
            // Refresh all alert rules lists
            await loadCustomAlerts();
            await loadAllAlertRules();
            
        } catch (error) {
            console.error('Failed to save custom alert:', error);
            PulseApp.ui.toast.error(`Failed to create custom alert: ${error.message}`);
        }
    }

    function loadCurrentAlerts() {
        // Try both old and new alert list containers for compatibility
        let contentDiv = document.getElementById('active-alerts-list');
        if (!contentDiv) {
            contentDiv = document.getElementById('current-alerts-list');
        }
        if (!contentDiv) return;

        // Get alerts from the alerts handler
        if (PulseApp.alerts && PulseApp.alerts.getActiveAlerts) {
            const activeAlerts = PulseApp.alerts.getActiveAlerts();
            renderCurrentAlerts(activeAlerts, contentDiv);
            
            // Update alert summary if we're in monitor tab
            if (activeTab === 'monitor') {
                updateAlertSummary();
            }
        } else {
            contentDiv.innerHTML = '<p class="p-6 text-gray-500 dark:text-gray-400">Alert system not initialized</p>';
        }
    }

    function renderCurrentAlerts(alerts, contentDiv = null) {
        if (!contentDiv) {
            contentDiv = document.getElementById('active-alerts-list') || document.getElementById('current-alerts-list');
        }
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
        const colorClass = 'border-red-400 bg-red-50 dark:bg-red-900/10';
        
        // Use the duration from the API if available, otherwise calculate it
        const duration = alert.duration ? Math.round(alert.duration / 1000) : 
                        Math.max(0, Math.round((Date.now() - (alert.triggeredAt || Date.now())) / 1000));
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
        const isCompoundAlert = alert.type === 'compound_threshold' || 
                               alert.rule?.type === 'compound_threshold' || 
                               alert.group === 'compound_threshold' ||
                               alert.rule?.metric === 'compound';
                               
        const alertType = isCompoundAlert ? 'Custom Alert' : 
                         alert.rule?.metric ? `${alert.rule.metric.toUpperCase()} Alert` : 
                         'System Alert';
        
        
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
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800 ${acknowledgedClass}">
                <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between mb-2">
                    <div class="flex-1 mb-2 sm:mb-0">
                        <div class="flex flex-wrap items-center gap-2 mb-1">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${alert.guest?.name || 'Unknown'}
                            </h5>
                            <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded-full">
                                ${alert.guest?.type || 'unknown'} ${alert.guest?.vmid || ''}
                            </span>
                            ${alert.escalated ? '<span class="text-xs bg-red-500 text-white px-2 py-0.5 rounded-full">Escalated</span>' : ''}
                            ${acknowledged ? '<span class="text-xs bg-green-500 text-white px-2 py-0.5 rounded-full">Acknowledged</span>' : ''}
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            ${alert.guest?.node ? `Node: ${alert.guest.node} • ` : ''}Active for ${durationStr}
                            ${acknowledged ? ` • Acknowledged ${Math.round((Date.now() - alert.acknowledgedAt) / 60000)}m ago` : ''}
                        </div>
                    </div>
                    <div class="flex gap-1 flex-wrap">
                        ${!acknowledged ? `
                            <button onclick="PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}');" 
                                    class="px-2 py-1 text-xs bg-green-600 hover:bg-green-700 text-white rounded whitespace-nowrap transition-all"
                                    data-alert-id="${alert.id}">
                                Acknowledge
                            </button>
                        ` : ''}
                    </div>
                </div>
                <div>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">
                        <span class="font-medium">${alert.ruleName || alertType}:</span> ${currentValueDisplay}
                    </p>
                    ${isCompoundAlert && alert.thresholds ? `
                        <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Rule: ${alert.thresholds.map(t => `${t.metric.toUpperCase()} ≥ ${t.threshold}${['cpu', 'memory', 'disk'].includes(t.metric) ? '%' : ''}`).join(' AND ')}
                        </p>
                    ` : ''}
                    ${thresholdInfo ? `
                        <div class="flex flex-wrap mb-2">
                            <span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                                ${thresholdInfo}
                            </span>
                        </div>
                    ` : ''}
                    ${alert.description ? `
                        <p class="text-xs text-gray-600 dark:text-gray-400 mb-2 italic">${alert.description}</p>
                    ` : ''}
                    <div class="flex justify-between items-end">
                        <div>
                            <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Notifications:</p>
                            <div class="flex gap-1">
                                <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">Pulse UI</span>
                                ${alert.emailSent ? '<span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded">Email</span>' : ''}
                                ${alert.webhookSent ? '<span class="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded">Webhook</span>' : ''}
                            </div>
                        </div>
                        <div class="text-right">
                            <p class="text-xs text-gray-500 dark:text-gray-400">
                                Triggered: ${new Date(alert.triggeredAt).toLocaleTimeString()}
                            </p>
                            ${alert.escalated ? `
                                <p class="text-xs text-orange-600 dark:text-orange-400 font-medium">
                                    Escalated
                                </p>
                            ` : ''}
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    async function loadAlertHistory() {
        const contentDiv = document.getElementById('alert-history-list');
        if (!contentDiv) return;

        try {
            // Show loading state
            contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Loading alert history...</p>';
            
            // Fetch actual alert history from the backend
            const response = await fetch('/api/alerts/history?limit=100');
            if (!response.ok) {
                throw new Error('Failed to fetch alert history');
            }
            
            const data = await response.json();
            const historicalAlerts = data.history || [];
            
            renderAlertHistory(historicalAlerts);
            
        } catch (error) {
            console.error('Error loading alert history:', error);
            contentDiv.innerHTML = `
                <div class="text-center py-8">
                    <svg class="w-12 h-12 mx-auto mb-4 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">Error Loading History</h3>
                    <p class="text-red-500 dark:text-red-400">Failed to load alert history: ${error.message}</p>
                </div>
            `;
        }
    }

    function renderAlertHistory(alerts) {
        const contentDiv = document.getElementById('alert-history-list');
        if (!contentDiv) return;

        if (!alerts || alerts.length === 0) {
            contentDiv.innerHTML = `
                <div class="text-center py-8">
                    <svg class="w-12 h-12 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                    <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No Alert History</h3>
                    <p class="text-gray-500 dark:text-gray-400">No historical alerts found.</p>
                </div>
            `;
            return;
        }

        // Group alerts by date
        const groupedAlerts = groupAlertsByDate(alerts);
        
        let content = '';
        Object.entries(groupedAlerts).forEach(([date, dayAlerts]) => {
            content += `
                <div class="space-y-3">
                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 border-b border-gray-200 dark:border-gray-700 pb-2">
                        ${date} (${dayAlerts.length} alert${dayAlerts.length !== 1 ? 's' : ''})
                    </h4>
                    <div class="space-y-2">
                        ${dayAlerts.map(alert => renderHistoricalAlertCard(alert)).join('')}
                    </div>
                </div>
            `;
        });

        contentDiv.innerHTML = content;
    }

    function renderHistoricalAlertCard(alert) {
        const duration = alert.resolvedAt ? 
            Math.round((alert.resolvedAt - alert.triggeredAt) / 1000) : 
            Math.round((Date.now() - alert.triggeredAt) / 1000);
        
        const durationStr = duration < 60 ? `${duration}s` : 
                           duration < 3600 ? `${Math.round(duration/60)}m` : 
                           `${Math.round(duration/3600)}h`;

        const resolutionBadge = alert.resolvedAt ? 
            `<span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded-full">
                Resolved
            </span>` :
            `<span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-full">
                Auto-cleared
            </span>`;


        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-800">
                <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between mb-2">
                    <div class="flex-1 mb-2 sm:mb-0">
                        <div class="flex flex-wrap items-center gap-2 mb-1">
                            <h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${alert.guest?.name || 'Unknown'}
                            </h5>
                            <span class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded-full">
                                ${alert.guest?.type || 'unknown'} ${alert.guest?.vmid || ''}
                            </span>
                            ${resolutionBadge}
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            ${new Date(alert.triggeredAt).toLocaleString()} • Duration: ${durationStr}
                            ${alert.guest?.node ? ` • Node: ${alert.guest.node}` : ''}
                        </div>
                    </div>
                    <div class="flex gap-1 flex-wrap">
                        <button onclick="PulseApp.ui.alertManagementModal.showAlertDetails('${alert.id}')" 
                                class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded whitespace-nowrap">
                            Details
                        </button>
                    </div>
                </div>
                <div>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mb-2">
                        ${alert.rule?.metric?.toUpperCase() || 'System'} Alert: ${alert.peakValue || alert.currentValue || 'N/A'}
                    </p>
                    <div>
                        <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Notifications:</p>
                        <div class="flex gap-1">
                            <span class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">Pulse UI</span>
                            ${alert.emailSent ? '<span class="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-800 text-green-700 dark:text-green-300 rounded">Email</span>' : ''}
                            ${alert.webhookSent ? '<span class="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-800 text-purple-700 dark:text-purple-300 rounded">Webhook</span>' : ''}
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    function groupAlertsByDate(alerts) {
        const groups = {};
        
        alerts.forEach(alert => {
            const date = new Date(alert.triggeredAt).toLocaleDateString();
            if (!groups[date]) {
                groups[date] = [];
            }
            groups[date].push(alert);
        });

        // Sort groups by date (newest first)
        const sortedGroups = {};
        Object.keys(groups)
            .sort((a, b) => new Date(b) - new Date(a))
            .forEach(date => {
                // Sort alerts within each day by time (newest first)
                groups[date].sort((a, b) => b.triggeredAt - a.triggeredAt);
                sortedGroups[date] = groups[date];
            });

        return sortedGroups;
    }

    function generateMockAlertHistory() {
        const now = Date.now();
        const oneDay = 24 * 60 * 60 * 1000;
        
        return [
            {
                id: 'hist-1',
                guest: { name: 'prod-web-01', type: 'vm', vmid: '101', node: 'pve-node-1' },
                rule: { metric: 'cpu' },
                triggeredAt: now - (2 * oneDay),
                resolvedAt: now - (2 * oneDay) + (30 * 60 * 1000),
                peakValue: '87%',
                emailSent: true,
                webhookSent: false
            },
            {
                id: 'hist-2',
                guest: { name: 'db-primary', type: 'vm', vmid: '102', node: 'pve-node-2' },
                rule: { metric: 'memory' },
                triggeredAt: now - (3 * oneDay),
                resolvedAt: now - (3 * oneDay) + (2 * 60 * 60 * 1000),
                peakValue: '94%',
                emailSent: true,
                webhookSent: true
            },
            {
                id: 'hist-3',
                guest: { name: 'backup-server', type: 'vm', vmid: '103', node: 'pve-node-1' },
                rule: { metric: 'disk' },
                triggeredAt: now - (7 * oneDay),
                resolvedAt: now - (7 * oneDay) + (4 * 60 * 60 * 1000),
                peakValue: '82%',
                emailSent: false,
                webhookSent: false
            }
        ];
    }

    function clearAlertHistory() {
        if (confirm('Are you sure you want to clear all alert history? This action cannot be undone.')) {
            if (PulseApp.alerts && PulseApp.alerts.clearHistory) {
                PulseApp.alerts.clearHistory();
            }
            loadAlertHistory(); // Refresh the display
        }
    }

    function preserveCurrentFormData() {
        // TODO: Implement form data preservation if needed
        // Currently just a stub to prevent errors
    }

    function restoreFormData(tabName) {
        // TODO: Implement form data restoration if needed
        // Currently just a stub to prevent errors
    }

    async function loadConfiguration() {
        try {
            // Always load configuration directly from API to ensure fresh data
            const response = await fetch('/api/config');
            if (response.ok) {
                currentConfig = await response.json();
            } else {
                console.error('Failed to load configuration - response not ok');
                currentConfig = {};
            }
        } catch (error) {
            console.error('Error loading configuration:', error);
            currentConfig = {};
        }
    }

    function validateSMTPConfiguration() {
        const errors = [];
        
        // Get SMTP configuration values
        const smtpHost = document.querySelector('input[name="SMTP_HOST"]')?.value?.trim();
        const smtpPort = document.querySelector('input[name="SMTP_PORT"]')?.value?.trim();
        const smtpUser = document.querySelector('input[name="SMTP_USER"]')?.value?.trim();
        const smtpPass = document.querySelector('input[name="SMTP_PASS"]')?.value?.trim();
        const fromEmail = document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value?.trim();
        const toEmail = document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value?.trim();
        
        // Check if there's already a saved SMTP configuration
        const existingSmtpConfig = currentConfig?.advanced?.smtp;
        const hasExistingConfig = existingSmtpConfig?.host && existingSmtpConfig?.user;
        
        // If all fields are empty and we have an existing config, skip validation
        // This handles the case where the form hasn't been populated yet
        const allFieldsEmpty = !smtpHost && !smtpPort && !smtpUser && !smtpPass && !fromEmail && !toEmail;
        if (allFieldsEmpty && hasExistingConfig) {
            return errors; // No errors, using existing config
        }
        
        // Validate required fields if any SMTP field is filled (partial configuration check)
        const hasAnySMTPField = smtpHost || smtpPort || smtpUser || smtpPass || fromEmail || toEmail;
        
        if (hasAnySMTPField) {
            if (!smtpHost) {
                errors.push('SMTP Host is required when configuring email');
            }
            
            if (!smtpPort) {
                errors.push('SMTP Port is required when configuring email');
            } else {
                const port = parseInt(smtpPort);
                if (isNaN(port) || port < 1 || port > 65535) {
                    errors.push('SMTP Port must be a valid port number (1-65535)');
                }
            }
            
            if (!smtpUser) {
                errors.push('SMTP Username is required when configuring email');
            }
            
            // Only require password for new configurations or if user is trying to change it
            if (!smtpPass && !hasExistingConfig) {
                errors.push('SMTP Password is required when configuring email');
            }
            
            if (!fromEmail) {
                errors.push('From Email address is required when configuring email');
            } else if (!isValidEmail(fromEmail)) {
                errors.push('From Email must be a valid email address');
            }
            
            if (!toEmail) {
                errors.push('To Email address is required when configuring email');
            } else if (!isValidEmail(toEmail)) {
                errors.push('To Email must be a valid email address');
            }
        }
        
        return errors;
    }
    
    function isValidEmail(email) {
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return emailRegex.test(email);
    }

    async function saveConfiguration() {
        // The save configuration button in the alert management modal should 
        // save any pending changes. Since individual system alerts are edited 
        // in their own modals and saved immediately, and custom alerts have 
        // their own save buttons, this function mainly handles email/webhook 
        // configuration that might be pending.
        
        const saveButton = document.getElementById('alert-management-save-button');
        if (!saveButton) {
            return;
        }

        const originalText = saveButton.textContent;
        saveButton.disabled = true;
        saveButton.textContent = 'Saving...';

        try {
            let configToSave = {};
            let hasChanges = false;
            
            // First check if we have any changes to save
            const emailConfigSection = document.getElementById('primary-email-config');
            let hasEmailChanges = false;
            
            if (emailConfigSection) {
                const smtpHost = document.querySelector('input[name="SMTP_HOST"]')?.value;
                const smtpPort = document.querySelector('input[name="SMTP_PORT"]')?.value;
                const smtpUser = document.querySelector('input[name="SMTP_USER"]')?.value;
                const smtpPass = document.querySelector('input[name="SMTP_PASS"]')?.value;
                const fromEmail = document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value;
                const toEmail = document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value;
                
                // Check if any email field has a value (indicating user wants to save email config)
                hasEmailChanges = smtpHost || smtpPort || smtpUser || smtpPass || fromEmail || toEmail;
            }
            
            // Only validate SMTP configuration if there are email changes
            if (hasEmailChanges) {
                const validationErrors = validateSMTPConfiguration();
                if (validationErrors.length > 0) {
                    const errorMessage = 'Please fix the following errors:\n\n• ' + validationErrors.join('\n• ');
                    alert(errorMessage);
                    
                    // Re-enable save button since validation failed
                    saveButton.disabled = false;
                    saveButton.textContent = originalText;
                    return;
                }
            }

            // Process email configuration changes
            if (emailConfigSection && hasEmailChanges) {
                const smtpHost = document.querySelector('input[name="SMTP_HOST"]')?.value;
                const smtpPort = document.querySelector('input[name="SMTP_PORT"]')?.value;
                const smtpUser = document.querySelector('input[name="SMTP_USER"]')?.value;
                const smtpPass = document.querySelector('input[name="SMTP_PASS"]')?.value;
                const fromEmail = document.querySelector('input[name="ALERT_FROM_EMAIL"]')?.value;
                const toEmail = document.querySelector('input[name="ALERT_TO_EMAIL"]')?.value;
                const smtpSecure = document.querySelector('input[name="SMTP_SECURE"]')?.checked;

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
                console.log('No changes to save');
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
                console.log('Configuration saved successfully');
                
                // Add visual feedback to show config is saved
                markEmailConfigAsSaved();
                
                await loadConfiguration(); // Reload to reflect changes
            } else {
                throw new Error(result.error || 'Failed to save configuration');
            }

        } catch (error) {
            console.error('Error saving configuration:', error);
        } finally {
            saveButton.disabled = false;
            saveButton.textContent = originalText;
        }
    }

    // Missing functions that were referenced in the public API
    async function toggleAlert(alertId, alertType, enabled) {
        if (alertType === 'system') {
            return toggleSystemAlert(alertId, enabled);
        } else if (alertType === 'custom') {
            return toggleCustomAlert(alertId, enabled);
        }
    }

    async function toggleSystemAlert(alertId, enabled) {
        // System alerts are now handled via the unified alert system
        // Use the same logic as custom alerts
        return toggleCustomAlert(alertId, enabled);
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
        }
    }

    // editCustomAlert function removed - now using unified editAlertRule

    async function deleteCustomAlert(alertId) {
        try {
            const response = await fetch(`/api/alerts/rules/${alertId}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `Server error: ${response.status}`);
            }

            await loadCustomAlerts(); // Refresh display
            
        } catch (error) {
            console.error('Failed to delete custom alert:', error);
        }
    }

    function showAlertDetails(alertId) {
        // Find the alert in the current alerts list
        try {
            const alerts = PulseApp.alerts ? PulseApp.alerts.getActiveAlerts() : [];
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
                            console.error('Alert not found');
                        }
                    })
                    .catch(error => {
                        console.error('Failed to fetch alert details:', error);
                    });
            }
        } catch (error) {
            console.error('Error accessing alerts:', error);
        }
    }

    function displayAlertDetailsModal(alert) {
        // Create alert details modal
        const modalHTML = `
            <div id="alert-details-modal" class="fixed inset-0 flex items-start sm:items-center justify-center bg-black bg-opacity-50 pt-4 sm:pt-0" style="z-index: 9999;">
                <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[95vh] sm:max-h-[80vh] flex flex-col m-2 sm:m-4">
                    <div class="flex justify-between items-center border-b border-gray-300 dark:border-gray-700 px-4 sm:px-6 py-3 sm:py-4">
                        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Alert Details</h2>
                        <button onclick="document.getElementById('alert-details-modal').remove();" 
                                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                            <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                    <div class="flex-1 overflow-y-auto p-4 sm:p-6">
                        <div class="space-y-4">
                            <div>
                                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Alert Rule</h3>
                                <p class="text-lg font-semibold text-gray-900 dark:text-gray-100">${alert.ruleName || alert.name}</p>
                            </div>
                            <div>
                                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Guest System</h3>
                                <p class="text-gray-900 dark:text-gray-100">${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid}) on ${alert.guest.node}</p>
                            </div>
                            <div>
                                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Current Value</h3>
                                <p class="text-gray-900 dark:text-gray-100">${formatCurrentValue(alert.currentValue, alert.type)}</p>
                            </div>
                            <div>
                                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Triggered At</h3>
                                <p class="text-gray-900 dark:text-gray-100">${new Date(alert.triggeredAt).toLocaleString()}</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        document.body.insertAdjacentHTML('beforeend', modalHTML);
    }

    function addWebhookEndpoint() {
        const listContainer = document.getElementById('additional-webhook-list');
        if (!listContainer) return;
        
        // Hide empty state if it exists
        const emptyState = listContainer.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }
        
        const webhookId = 'webhook-endpoint-' + Date.now();
        const webhookHtml = `
            <div class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg" id="${webhookId}">
                <div class="flex-1 space-y-2 sm:space-y-0 sm:grid sm:grid-cols-3 sm:gap-3">
                    <div class="sm:col-span-2">
                        <input type="url" class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="https://discord.com/api/webhooks/...">
                    </div>
                    <div>
                        <input type="text" class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" placeholder="Name (e.g., Dev Team)">
                    </div>
                </div>
                <button onclick="PulseApp.ui.alertManagementModal.removeWebhookEndpoint('${webhookId}')" 
                        class="px-2 py-1 text-xs bg-red-600 hover:bg-red-700 text-white rounded">
                    Remove
                </button>
            </div>
        `;
        
        listContainer.insertAdjacentHTML('beforeend', webhookHtml);
    }
    
    function removeWebhookEndpoint(webhookId) {
        const webhookElement = document.getElementById(webhookId);
        if (webhookElement) {
            webhookElement.remove();
            
            // Check if there are any remaining webhooks
            const listContainer = document.getElementById('additional-webhook-list');
            const remainingWebhooks = listContainer.querySelectorAll('[id^="webhook-endpoint-"]');
            
            if (remainingWebhooks.length === 0) {
                // Show empty state
                const emptyState = listContainer.querySelector('.border-dashed');
                if (emptyState) {
                    emptyState.style.display = 'block';
                }
            }
        }
    }

    // Update slider value display for custom alert modal
    function updateSliderValueDisplay(slider) {
        if (!slider) return;
        
        const value = slider.value;
        const name = slider.name;
        
        // Map slider names to their display element IDs
        const displayIds = {
            'cpuThreshold': 'cpu-threshold-value',
            'memoryThreshold': 'memory-threshold-value', 
            'diskThreshold': 'disk-threshold-value'
        };
        
        const displayElement = document.getElementById(displayIds[name]);
        if (displayElement) {
            displayElement.textContent = `${value}%`;
        }
    }

    // Update threshold preview for custom alert modal
    function updateThresholdPreview() {
        const previewList = document.getElementById('threshold-preview-list');
        if (!previewList) return;
        
        const activeThresholds = [];
        
        // Check CPU threshold
        const cpuSlider = document.querySelector('input[name="cpuThreshold"]');
        if (cpuSlider && parseInt(cpuSlider.value) > 0) {
            activeThresholds.push(`CPU > ${cpuSlider.value}%`);
        }
        
        // Check Memory threshold
        const memorySlider = document.querySelector('input[name="memoryThreshold"]');
        if (memorySlider && parseInt(memorySlider.value) > 0) {
            activeThresholds.push(`Memory > ${memorySlider.value}%`);
        }
        
        // Check Disk threshold
        const diskSlider = document.querySelector('input[name="diskThreshold"]');
        if (diskSlider && parseInt(diskSlider.value) > 0) {
            activeThresholds.push(`Disk > ${diskSlider.value}%`);
        }
        
        // Check Network threshold
        const networkSelect = document.querySelector('select[name="networkThreshold"]');
        if (networkSelect && parseInt(networkSelect.value) > 0) {
            const networkValue = parseInt(networkSelect.value);
            const networkMB = networkValue / (1024 * 1024);
            activeThresholds.push(`Network > ${networkMB} MB/s`);
        }
        
        // Check Status monitoring
        const statusCheckbox = document.querySelector('input[name="statusMonitoring"]');
        if (statusCheckbox && statusCheckbox.checked) {
            activeThresholds.push('VM/LXC Status monitoring');
        }
        
        // Update display
        if (activeThresholds.length > 0) {
            previewList.innerHTML = activeThresholds.join('<br>');
        } else {
            previewList.textContent = 'No thresholds set';
        }
    }

    // Public API
    return {
        init,
        openModal,
        closeModal,
        switchTab: switchTab,
        openCustomAlertModal,
        editAlertRule,
        toggleCustomAlert,
        toggleAlert,
        toggleAlertRule,
        deleteCustomAlert,
        showAlertDetails,
        addWebhookEndpoint,
        removeWebhookEndpoint,
        handleEmailProviderSelection,
        loadCurrentAlerts: () => {
            if (activeTab === 'monitor' || activeTab === 'alerts') {
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

// Global functions are now exposed in the init() function to prevent timing issues