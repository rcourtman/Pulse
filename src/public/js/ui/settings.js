PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.settings = (() => {
    let currentConfig = {};
    let isInitialized = false;
    let activeTab = 'proxmox';

    function init() {
        if (isInitialized) return;
        
        console.log('[Settings] Initializing settings module...');
        
        // Set up modal event listeners
        const settingsButton = document.getElementById('settings-button');
        const modal = document.getElementById('settings-modal');
        const closeButton = document.getElementById('settings-modal-close');
        const cancelButton = document.getElementById('settings-cancel-button');
        const saveButton = document.getElementById('settings-save-button');

        if (settingsButton) {
            settingsButton.addEventListener('click', openModal);
        }

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
            if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
                closeModal();
            }
        });

        // Set up tab navigation
        setupTabNavigation();

        isInitialized = true;
        console.log('[Settings] Settings module initialized');
    }

    function setupTabNavigation() {
        const tabButtons = document.querySelectorAll('.settings-tab');
        
        tabButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                const tabName = e.target.getAttribute('data-tab');
                switchTab(tabName);
            });
        });
    }

    function switchTab(tabName) {
        activeTab = tabName;
        
        // Update tab buttons
        const tabButtons = document.querySelectorAll('.settings-tab');
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
    }

    async function openModal() {
        console.log('[Settings] Opening modal...');
        
        const modal = document.getElementById('settings-modal');
        if (!modal) return;

        // Show the modal
        modal.classList.remove('hidden');
        modal.classList.add('flex');

        // Load current configuration
        await loadConfiguration();
        
        // Reset to first tab
        switchTab('proxmox');
    }

    function closeModal() {
        console.log('[Settings] Closing modal...');
        
        const modal = document.getElementById('settings-modal');
        if (!modal) return;

        modal.classList.add('hidden');
        modal.classList.remove('flex');
    }

    async function loadConfiguration() {
        try {
            const response = await fetch('/api/config');
            const data = await response.json();
            
            if (response.ok) {
                currentConfig = data;
                console.log('[Settings] Configuration loaded:', currentConfig);
                renderTabContent();
            } else {
                console.error('[Settings] Failed to load configuration:', data.error);
                showMessage('Failed to load configuration', 'error');
            }
        } catch (error) {
            console.error('[Settings] Error loading configuration:', error);
            showMessage('Failed to load configuration: ' + error.message, 'error');
        }
    }

    function renderTabContent() {
        const container = document.getElementById('settings-modal-body');
        if (!container) return;

        // Ensure currentConfig has a safe default structure
        const safeConfig = currentConfig || {};
        const proxmox = safeConfig.proxmox || {};
        const pbs = safeConfig.pbs || {};
        const advanced = safeConfig.advanced || {};
        const alerts = advanced.alerts || {};

        let content = '';

        switch (activeTab) {
            case 'proxmox':
                content = renderProxmoxTab(proxmox, safeConfig);
                break;
            case 'pbs':
                content = renderPBSTab(pbs, safeConfig);
                break;
            case 'alerts':
                content = renderAlertsTab(alerts);
                break;
            case 'system':
                content = renderSystemTab(advanced, safeConfig);
                break;
        }

        container.innerHTML = `<form id="settings-form" class="space-y-6">${content}</form>`;
        
        // Load existing additional endpoints for Proxmox and PBS tabs
        if (activeTab === 'proxmox') {
            loadExistingPveEndpoints();
        } else if (activeTab === 'pbs') {
            loadExistingPbsEndpoints();
        } else if (activeTab === 'system') {
            // Auto-check for latest version when system tab is opened
            checkLatestVersion();
        } else if (activeTab === 'alerts') {
            // Load threshold configurations when alerts tab is opened
            loadThresholdConfigurations();
        }
    }

    function renderProxmoxTab(proxmox, config) {
        return `
            <!-- Primary Proxmox VE Configuration -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                <div class="mb-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Primary Proxmox VE Server</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                        Main PVE server configuration (required) 
                        <a href="https://github.com/rcourtman/Pulse#creating-a-proxmox-api-token" target="_blank" rel="noopener noreferrer" 
                           class="text-blue-600 dark:text-blue-400 hover:underline ml-1">
                            📚 Need help creating API tokens?
                        </a>
                    </p>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Host Address <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_HOST" required
                               value="${proxmox.host || ''}"
                               placeholder="https://proxmox.example.com:8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT"
                               value="${proxmox.port || ''}"
                               placeholder="8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Node Name
                        </label>
                        <input type="text" name="PROXMOX_NODE_NAME"
                               value="${proxmox.nodeName || ''}"
                               placeholder="Display name (optional)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token ID <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_TOKEN_ID" required
                               value="${proxmox.tokenId || ''}"
                               placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token Secret
                        </label>
                        <input type="password" name="PROXMOX_TOKEN_SECRET"
                               placeholder="Leave blank to keep current"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Enabled</label>
                        <input type="checkbox" name="PROXMOX_ENABLED" ${proxmox.enabled !== false ? 'checked' : ''}
                               class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                    </div>
                </div>
            </div>

            <!-- Additional Proxmox VE Endpoints -->
            <div id="additional-pve-endpoints">
                <div class="flex justify-between items-center mb-4">
                    <div>
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Additional Proxmox VE Servers</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Add more PVE servers beyond the primary one above</p>
                    </div>
                    <button type="button" onclick="PulseApp.ui.settings.addPveEndpoint()" 
                            class="flex items-center gap-2 px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                        </svg>
                        Add Another PVE Server
                    </button>
                </div>
                <div id="pve-endpoints-container" class="space-y-4">
                    <div class="text-center py-8 text-gray-500 dark:text-gray-400 italic border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg">
                        No additional PVE servers configured.<br>
                        <span class="text-sm">Click "Add Another PVE Server" to add more.</span>
                    </div>
                </div>
            </div>
            
            <!-- Test Connections -->
            <div class="flex justify-end mt-6">
                <button type="button" onclick="PulseApp.ui.settings.testConnections()" 
                        class="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors">
                    Test PVE Connections
                </button>
            </div>
        `;
    }

    function renderPBSTab(pbs, config) {
        return `
            <!-- Primary PBS Configuration -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                <div class="mb-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Primary Proxmox Backup Server</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                        Main PBS server configuration (optional)
                        <a href="https://github.com/rcourtman/Pulse#creating-a-proxmox-backup-server-api-token" target="_blank" rel="noopener noreferrer" 
                           class="text-blue-600 dark:text-blue-400 hover:underline ml-1">
                            📚 Need help creating PBS API tokens?
                        </a>
                    </p>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host Address</label>
                        <input type="text" name="PBS_HOST"
                               value="${pbs.host || ''}"
                               placeholder="https://pbs.example.com:8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PBS_PORT"
                               value="${pbs.port || ''}"
                               placeholder="8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Node Name
                        </label>
                        <input type="text" name="PBS_NODE_NAME"
                               value="${pbs.nodeName || ''}"
                               placeholder="PBS internal hostname"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PBS_TOKEN_ID"
                               value="${pbs.tokenId || ''}"
                               placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token Secret</label>
                        <input type="password" name="PBS_TOKEN_SECRET"
                               placeholder="Leave blank to keep current"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                </div>
            </div>

            <!-- Additional PBS Endpoints -->
            <div id="additional-pbs-endpoints">
                <div class="flex justify-between items-center mb-4">
                    <div>
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Additional PBS Servers</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Add more PBS servers beyond the primary one above</p>
                    </div>
                    <button type="button" onclick="PulseApp.ui.settings.addPbsEndpoint()" 
                            class="flex items-center gap-2 px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-sm rounded-md">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                        </svg>
                        Add Another PBS Server
                    </button>
                </div>
                <div id="pbs-endpoints-container" class="space-y-4">
                    <div class="text-center py-8 text-gray-500 dark:text-gray-400 italic border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg">
                        No additional PBS servers configured.<br>
                        <span class="text-sm">Click "Add Another PBS Server" to add more.</span>
                    </div>
                </div>
            </div>
            
            <!-- Test Connections -->
            <div class="flex justify-end mt-6">
                <button type="button" onclick="PulseApp.ui.settings.testConnections()" 
                        class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                    Test PBS Connections
                </button>
            </div>
        `;
    }

    function renderAlertsTab(alerts) {
        return `
            <div class="space-y-6">
                <!-- Global Alert Settings -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Global Alert Settings</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">These settings apply to all VMs and LXCs unless overridden by custom thresholds below.</p>
                    
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
                        <label class="flex items-center">
                            <input type="checkbox" name="ALERT_CPU_ENABLED" ${alerts.cpu?.enabled !== false ? 'checked' : ''}
                                   class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">CPU Alerts</span>
                        </label>
                        <label class="flex items-center">
                            <input type="checkbox" name="ALERT_MEMORY_ENABLED" ${alerts.memory?.enabled !== false ? 'checked' : ''}
                                   class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">Memory Alerts</span>
                        </label>
                        <label class="flex items-center">
                            <input type="checkbox" name="ALERT_DISK_ENABLED" ${alerts.disk?.enabled !== false ? 'checked' : ''}
                                   class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">Disk Alerts</span>
                        </label>
                        <label class="flex items-center">
                            <input type="checkbox" name="ALERT_DOWN_ENABLED" ${alerts.down?.enabled !== false ? 'checked' : ''}
                                   class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">Down Alerts</span>
                        </label>
                    </div>

                    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                CPU Threshold (%)
                            </label>
                            <input type="number" name="ALERT_CPU_THRESHOLD"
                                   value="${alerts.cpu?.threshold || ''}"
                                   placeholder="85 (default)"
                                   min="50" max="100"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Memory Threshold (%)
                            </label>
                            <input type="number" name="ALERT_MEMORY_THRESHOLD"
                                   value="${alerts.memory?.threshold || ''}"
                                   placeholder="90 (default)"
                                   min="50" max="100"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Disk Threshold (%)
                            </label>
                            <input type="number" name="ALERT_DISK_THRESHOLD"
                                   value="${alerts.disk?.threshold || ''}"
                                   placeholder="95 (default)"
                                   min="50" max="100"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                    </div>
                </div>

                <!-- Custom Per-VM/LXC Thresholds Section -->
                <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-blue-800 dark:text-blue-200">Custom Per-VM/LXC Thresholds</h3>
                            <p class="mt-1 text-sm text-blue-700 dark:text-blue-300">Configure custom alert thresholds for individual VMs/LXCs based on their specific resource requirements and usage patterns.</p>
                        </div>
                    </div>
                </div>

                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between">
                            <div>
                                <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Custom Threshold Configurations</h3>
                                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">VMs and containers with custom alert thresholds</p>
                            </div>
                            <button type="button" id="add-threshold-btn" class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path>
                                </svg>
                                Add Custom Threshold
                            </button>
                        </div>
                    </div>
                    
                    <div id="thresholds-loading" class="px-6 py-8 text-center">
                        <div class="inline-flex items-center">
                            <svg class="animate-spin -ml-1 mr-3 h-5 w-5 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            <span class="text-gray-600 dark:text-gray-400">Loading threshold configurations...</span>
                        </div>
                    </div>
                    
                    <div id="thresholds-container" class="hidden">
                        <div id="thresholds-list" class="divide-y divide-gray-200 dark:divide-gray-700">
                            <!-- Threshold configurations will be loaded here -->
                        </div>
                        
                        <div id="thresholds-empty" class="hidden px-6 py-8 text-center">
                            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 00-2-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v4"></path>
                            </svg>
                            <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No custom thresholds configured</h3>
                            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Get started by adding a custom threshold configuration for a VM or container.</p>
                        </div>
                    </div>
                </div>

                <div class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4">
                    <div class="space-y-2">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Examples:</h4>
                        <ul class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                            <li>• <strong>Storage/NAS VMs:</strong> Set memory warning to 95% and critical to 99% (high memory usage from disk caching is normal)</li>
                            <li>• <strong>Application Servers:</strong> Set CPU warning to 70% and critical to 85% for better performance monitoring</li>
                            <li>• <strong>Development VMs:</strong> Set disk warning to 75% and critical to 90% for early space alerts</li>
                        </ul>
                    </div>
                </div>
            </div>
        `;
    }

    function renderSystemTab(advanced, config) {
        const currentTheme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
        
        return `
            <!-- Appearance Settings -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Appearance</h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Theme
                        </label>
                        <select name="THEME_PREFERENCE" onchange="PulseApp.ui.settings.changeTheme(this.value)"
                                class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            <option value="auto" ${currentTheme === 'auto' ? 'selected' : ''}>Auto (System)</option>
                            <option value="light" ${currentTheme === 'light' ? 'selected' : ''}>Light</option>
                            <option value="dark" ${currentTheme === 'dark' ? 'selected' : ''}>Dark</option>
                        </select>
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Choose your preferred color scheme</p>
                    </div>
                </div>
            </div>

            <!-- Service Settings -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Service Settings</h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Metric Update Interval (ms)
                        </label>
                        <input type="number" name="PULSE_METRIC_INTERVAL_MS"
                               value="${advanced.metricInterval || ''}"
                               placeholder="2000 (default)"
                               min="1000" max="60000"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">How often to fetch VM/Container metrics</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Discovery Interval (ms)
                        </label>
                        <input type="number" name="PULSE_DISCOVERY_INTERVAL_MS"
                               value="${advanced.discoveryInterval || ''}"
                               placeholder="30000 (default)"
                               min="5000" max="300000"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">How often to discover nodes and VMs</p>
                    </div>
                </div>
            </div>

            <!-- Update Management -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Software Updates</h3>
                
                <!-- Auto-update Setting -->
                <div class="mb-6 p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="flex items-center justify-between">
                        <div>
                            <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-200">Automatic Updates</h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                Automatically check for and install updates when available
                            </p>
                        </div>
                        <label class="flex items-center">
                            <input type="checkbox" name="AUTO_UPDATE_ENABLED" ${advanced.autoUpdate?.enabled !== false ? 'checked' : ''}
                                   class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Enable</span>
                        </label>
                    </div>
                    <div class="mt-3 grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Check Interval (hours)
                            </label>
                            <input type="number" name="AUTO_UPDATE_CHECK_INTERVAL"
                                   value="${advanced.autoUpdate?.checkInterval || ''}"
                                   placeholder="24 (default)"
                                   min="1" max="168"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Update Time
                            </label>
                            <input type="time" name="AUTO_UPDATE_TIME"
                                   value="${advanced.autoUpdate?.time || ''}"
                                   placeholder="02:00"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Preferred time for automatic updates</p>
                        </div>
                    </div>
                </div>
                
                <div id="update-status" class="mb-4">
                    <div class="flex items-center justify-between">
                        <div>
                            <p class="text-sm text-gray-700 dark:text-gray-300">
                                Current Version: <span id="current-version" class="font-mono font-semibold">${currentConfig.version || 'Unknown'}</span>
                            </p>
                            <p class="text-sm text-gray-700 dark:text-gray-300 mt-1">
                                Latest Version: <span id="latest-version" class="font-mono font-semibold text-gray-500 dark:text-gray-400">Checking...</span>
                            </p>
                            <p id="version-status" class="text-sm mt-1"></p>
                        </div>
                        <button type="button" onclick="PulseApp.ui.settings.checkForUpdates()" 
                                id="check-updates-button"
                                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md flex items-center gap-2">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                            Check for Updates
                        </button>
                    </div>
                </div>
                
                <!-- Update Details (hidden by default) -->
                <div id="update-details" class="hidden">
                    <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
                        <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4 mb-4">
                            <h4 class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                                Update Available: <span id="update-version"></span>
                            </h4>
                            <div id="update-release-notes" class="text-sm text-gray-700 dark:text-gray-300 prose prose-sm max-w-none"></div>
                        </div>
                        
                        <div class="flex items-center justify-between">
                            <p class="text-sm text-gray-600 dark:text-gray-400">
                                Published: <span id="update-published"></span>
                            </p>
                            <button type="button" onclick="PulseApp.ui.settings.applyUpdate()" 
                                    id="apply-update-button"
                                    class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm rounded-md flex items-center gap-2">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                                </svg>
                                Apply Update
                            </button>
                        </div>
                    </div>
                </div>
                
                <!-- Update Progress (hidden by default) -->
                <div id="update-progress" class="hidden">
                    <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
                        <div class="mb-4">
                            <p class="text-sm font-medium text-gray-700 dark:text-gray-300" id="update-progress-text">Preparing update...</p>
                        </div>
                        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
                            <div id="update-progress-bar" class="bg-blue-600 h-2.5 rounded-full transition-all duration-300" style="width: 0%"></div>
                        </div>
                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                            Do not close this window or refresh the page during the update process.
                        </p>
                    </div>
                </div>
            </div>
        `;
    }

    function renderThresholdsTab() {
        return `
            <div class="space-y-6">
                <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-blue-800 dark:text-blue-200">Custom Alert Thresholds</h3>
                            <p class="mt-1 text-sm text-blue-700 dark:text-blue-300">Configure custom alert thresholds for individual VMs/LXCs based on their specific resource requirements and usage patterns.</p>
                        </div>
                    </div>
                </div>

                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between">
                            <div>
                                <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Custom Threshold Configurations</h3>
                                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">VMs and containers with custom alert thresholds</p>
                            </div>
                            <button type="button" id="add-threshold-btn" class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
                                <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path>
                                </svg>
                                Add Custom Threshold
                            </button>
                        </div>
                    </div>
                    
                    <div id="thresholds-loading" class="px-6 py-8 text-center">
                        <div class="inline-flex items-center">
                            <svg class="animate-spin -ml-1 mr-3 h-5 w-5 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            <span class="text-gray-600 dark:text-gray-400">Loading threshold configurations...</span>
                        </div>
                    </div>
                    
                    <div id="thresholds-container" class="hidden">
                        <div id="thresholds-list" class="divide-y divide-gray-200 dark:divide-gray-700">
                            <!-- Threshold configurations will be loaded here -->
                        </div>
                        
                        <div id="thresholds-empty" class="hidden px-6 py-8 text-center">
                            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 00-2-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v4"></path>
                            </svg>
                            <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No custom thresholds configured</h3>
                            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Get started by adding a custom threshold configuration for a VM or container.</p>
                        </div>
                    </div>
                </div>

                <div class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4">
                    <div class="space-y-2">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Examples:</h4>
                        <ul class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                            <li>• <strong>Storage/NAS VMs:</strong> Set memory warning to 95% and critical to 99% (high memory usage from disk caching is normal)</li>
                            <li>• <strong>Application Servers:</strong> Set CPU warning to 70% and critical to 85% for better performance monitoring</li>
                            <li>• <strong>Development VMs:</strong> Set disk warning to 75% and critical to 90% for early space alerts</li>
                        </ul>
                    </div>
                </div>
            </div>
        `;
    }

    // Additional endpoint management functions
    function addPveEndpoint() {
        const container = document.getElementById('pve-endpoints-container');
        if (!container) return;

        // Hide empty state if this is the first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

        // Count only actual endpoint divs (not the empty state)
        const existingEndpoints = container.querySelectorAll('.border:not(.border-dashed)');
        const index = existingEndpoints.length + 2; // Start from _2 for additional endpoints
        const endpointHtml = `
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-4 right-4 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <h4 class="text-lg font-medium text-gray-800 dark:text-gray-200 mb-4">PVE Server #${index}</h4>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host Address</label>
                        <input type="text" name="PROXMOX_HOST_${index}" placeholder="https://pve${index}.example.com:8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT_${index}" placeholder="8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Node Name</label>
                        <input type="text" name="PROXMOX_NODE_NAME_${index}" placeholder="PVE Server ${index}"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PROXMOX_TOKEN_ID_${index}" placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token Secret</label>
                        <input type="password" name="PROXMOX_TOKEN_SECRET_${index}" placeholder="Enter token secret"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="flex items-center mt-6">
                            <input type="checkbox" name="PROXMOX_ENABLED_${index}" checked
                                   class="mr-2 h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                            <span class="text-sm text-gray-700 dark:text-gray-300">Enabled</span>
                        </label>
                    </div>
                </div>
            </div>
        `;

        container.insertAdjacentHTML('beforeend', endpointHtml);
    }

    function addPbsEndpoint() {
        const container = document.getElementById('pbs-endpoints-container');
        if (!container) return;

        // Hide empty state if this is the first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

        // Count only actual endpoint divs (not the empty state)
        const existingEndpoints = container.querySelectorAll('.border:not(.border-dashed)');
        const index = existingEndpoints.length + 2; // Start from _2 for additional endpoints
        const endpointHtml = `
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-4 right-4 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <h4 class="text-lg font-medium text-gray-800 dark:text-gray-200 mb-4">PBS Server #${index}</h4>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host Address</label>
                        <input type="text" name="PBS_HOST_${index}" placeholder="https://pbs${index}.example.com:8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PBS_PORT_${index}" placeholder="8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Node Name</label>
                        <input type="text" name="PBS_NODE_NAME_${index}" placeholder="PBS internal hostname"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PBS_TOKEN_ID_${index}" placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token Secret</label>
                        <input type="password" name="PBS_TOKEN_SECRET_${index}" placeholder="Enter token secret"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                </div>
            </div>
        `;

        container.insertAdjacentHTML('beforeend', endpointHtml);
    }

    function removeEndpoint(button) {
        const endpointDiv = button.closest('.border');
        if (endpointDiv) {
            const container = endpointDiv.parentElement;
            endpointDiv.remove();
            
            // Show empty state if no endpoints remain
            const remainingEndpoints = container.querySelectorAll('.border:not(.border-dashed)');
            if (remainingEndpoints.length === 0) {
                const emptyState = container.querySelector('.border-dashed');
                if (emptyState) {
                    emptyState.style.display = 'block';
                }
            }
        }
    }

    function loadExistingPveEndpoints() {
        const config = currentConfig || {};
        const container = document.getElementById('pve-endpoints-container');
        if (!container) return;

        // Load additional PVE endpoints from config
        for (let i = 2; i <= 10; i++) {
            if (config[`PROXMOX_HOST_${i}`]) {
                addPveEndpoint();
                // Populate the newly added endpoint with data
                const newEndpoint = container.lastElementChild;
                const inputs = newEndpoint.querySelectorAll('input');
                inputs.forEach(input => {
                    const configKey = input.name;
                    if (config[configKey]) {
                        if (input.type === 'checkbox') {
                            input.checked = config[configKey] !== 'false';
                        } else {
                            input.value = config[configKey];
                        }
                    }
                });
            }
        }
    }

    function loadExistingPbsEndpoints() {
        const config = currentConfig || {};
        const container = document.getElementById('pbs-endpoints-container');
        if (!container) return;

        // Load additional PBS endpoints from config
        for (let i = 2; i <= 10; i++) {
            if (config[`PBS_HOST_${i}`]) {
                addPbsEndpoint();
                // Populate the newly added endpoint with data
                const newEndpoint = container.lastElementChild;
                const inputs = newEndpoint.querySelectorAll('input');
                inputs.forEach(input => {
                    const configKey = input.name;
                    if (config[configKey]) {
                        if (input.type === 'checkbox') {
                            input.checked = config[configKey] !== 'false';
                        } else {
                            input.value = config[configKey];
                        }
                    }
                });
            }
        }
    }

    // Rest of the functions (testConnections, saveConfiguration, etc.)
    async function testConnections() {
        showMessage('Testing connections...', 'info');
        
        const config = collectFormData();
        
        try {
            const response = await fetch('/api/config/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                showMessage('All connections tested successfully!', 'success');
            } else {
                showMessage(result.error || 'Connection test failed', 'error');
            }
        } catch (error) {
            showMessage('Failed to test connections: ' + error.message, 'error');
        }
    }

    async function saveConfiguration() {
        const saveButton = document.getElementById('settings-save-button');
        if (!saveButton) return;

        const originalText = saveButton.textContent;
        saveButton.disabled = true;
        saveButton.textContent = 'Saving...';

        try {
            const config = collectFormData();
            
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                showMessage('Configuration saved successfully!', 'success');
                setTimeout(() => {
                    closeModal();
                }, 1500);
            } else {
                showMessage(result.error || 'Failed to save configuration', 'error');
            }
        } catch (error) {
            showMessage('Failed to save configuration: ' + error.message, 'error');
        } finally {
            saveButton.disabled = false;
            saveButton.textContent = originalText;
        }
    }

    function collectFormData() {
        const form = document.getElementById('settings-form');
        if (!form) return {};

        const formData = new FormData(form);
        const config = {};

        // Build the config object from form data
        for (const [name, value] of formData.entries()) {
            if (value.trim() === '') continue; // Skip empty values

            // Handle checkbox values
            if (form.querySelector(`[name="${name}"]`).type === 'checkbox') {
                config[name] = 'true';
            } else {
                config[name] = value;
            }
        }

        // Also handle unchecked checkboxes
        const checkboxes = form.querySelectorAll('input[type="checkbox"]');
        checkboxes.forEach(checkbox => {
            if (!checkbox.checked && !config[checkbox.name]) {
                config[checkbox.name] = 'false';
            }
        });

        return config;
    }

    function showMessage(message, type = 'info') {
        // Find or create message container within the current tab
        let container = document.getElementById('settings-message');
        if (!container) {
            container = document.createElement('div');
            container.id = 'settings-message';
            container.className = 'mb-4';
            
            const form = document.getElementById('settings-form');
            if (form) {
                form.insertBefore(container, form.firstChild);
            }
        }

        const typeClasses = {
            error: 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800 text-red-700 dark:text-red-300',
            success: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-700 dark:text-green-300',
            info: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300'
        };

        const html = `
            <div class="border rounded-lg p-4 ${typeClasses[type] || typeClasses.info}">
                ${message}
            </div>
        `;

        container.innerHTML = html;

        // Clear message after 5 seconds for non-error messages
        if (type !== 'error') {
            setTimeout(() => {
                container.innerHTML = '';
            }, 5000);
        }
    }

    // Check for latest version from GitHub releases
    async function checkLatestVersion() {
        const latestVersionElement = document.getElementById('latest-version');
        const versionStatusElement = document.getElementById('version-status');
        
        if (!latestVersionElement) return;
        
        try {
            latestVersionElement.textContent = 'Checking...';
            latestVersionElement.className = 'font-mono font-semibold text-gray-500 dark:text-gray-400';
            
            const response = await fetch('https://api.github.com/repos/rcourtman/Pulse/releases/latest');
            const data = await response.json();
            
            if (response.ok && data.tag_name) {
                const latestVersion = data.tag_name.replace(/^v/, ''); // Remove 'v' prefix if present
                const currentVersion = currentConfig.version || 'Unknown';
                
                latestVersionElement.textContent = latestVersion;
                
                // Compare versions
                if (currentVersion !== 'Unknown' && compareVersions(currentVersion, latestVersion) < 0) {
                    // Update available
                    latestVersionElement.className = 'font-mono font-semibold text-green-600 dark:text-green-400';
                    versionStatusElement.innerHTML = '<span class="text-green-600 dark:text-green-400">📦 Update available!</span>';
                    
                    // Show update details
                    showUpdateDetails(data);
                } else if (currentVersion !== 'Unknown' && compareVersions(currentVersion, latestVersion) >= 0) {
                    // Up to date
                    latestVersionElement.className = 'font-mono font-semibold text-gray-700 dark:text-gray-300';
                    versionStatusElement.innerHTML = '<span class="text-green-600 dark:text-green-400">✅ Up to date</span>';
                } else {
                    // Unknown current version
                    latestVersionElement.className = 'font-mono font-semibold text-gray-700 dark:text-gray-300';
                    versionStatusElement.innerHTML = '<span class="text-gray-500 dark:text-gray-400">Unable to compare versions</span>';
                }
            } else {
                throw new Error('Failed to fetch release data');
            }
        } catch (error) {
            console.error('Error checking for updates:', error);
            latestVersionElement.textContent = 'Error';
            latestVersionElement.className = 'font-mono font-semibold text-red-500';
            versionStatusElement.innerHTML = '<span class="text-red-500">Failed to check for updates</span>';
        }
    }
    
    // Simple version comparison (assumes semver format)
    function compareVersions(version1, version2) {
        const v1parts = version1.split('.').map(Number);
        const v2parts = version2.split('.').map(Number);
        
        for (let i = 0; i < Math.max(v1parts.length, v2parts.length); i++) {
            const v1part = v1parts[i] || 0;
            const v2part = v2parts[i] || 0;
            
            if (v1part < v2part) return -1;
            if (v1part > v2part) return 1;
        }
        
        return 0;
    }
    
    // Show update details in the update section
    function showUpdateDetails(releaseData) {
        const updateDetails = document.getElementById('update-details');
        if (!updateDetails) return;
        
        const updateVersion = document.getElementById('update-version');
        const updateReleaseNotes = document.getElementById('update-release-notes');
        const updatePublished = document.getElementById('update-published');
        
        if (updateVersion) {
            updateVersion.textContent = releaseData.tag_name;
        }
        
        if (updateReleaseNotes && releaseData.body) {
            // Convert markdown to basic HTML (simple implementation)
            const htmlContent = releaseData.body
                .replace(/### (.*)/g, '<h4 class="font-semibold mt-3 mb-1">$1</h4>')
                .replace(/## (.*)/g, '<h3 class="font-semibold text-lg mt-3 mb-2">$1</h3>')
                .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
                .replace(/\*(.*?)\*/g, '<em>$1</em>')
                .replace(/- (.*)/g, '<li class="ml-4">• $1</li>')
                .replace(/\n/g, '<br>');
            
            updateReleaseNotes.innerHTML = htmlContent;
        }
        
        if (updatePublished && releaseData.published_at) {
            const publishedDate = new Date(releaseData.published_at).toLocaleDateString();
            updatePublished.textContent = publishedDate;
        }
        
        updateDetails.classList.remove('hidden');
    }

    // Update management functions
    async function checkForUpdates() {
        await checkLatestVersion();
        showMessage('Update check completed', 'info');
    }

    async function applyUpdate() {
        showMessage('Update functionality not implemented yet', 'info');
        // Implementation would go here
    }

    // Theme management function
    function changeTheme(theme) {
        const htmlElement = document.documentElement;
        
        if (theme === 'auto') {
            // Use system preference
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            if (prefersDark) {
                htmlElement.classList.add('dark');
            } else {
                htmlElement.classList.remove('dark');
            }
            localStorage.setItem('theme', 'auto');
            
            // Set up listener for system theme changes
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
                if (localStorage.getItem('theme') === 'auto') {
                    if (e.matches) {
                        htmlElement.classList.add('dark');
                    } else {
                        htmlElement.classList.remove('dark');
                    }
                }
            });
        } else if (theme === 'dark') {
            htmlElement.classList.add('dark');
            localStorage.setItem('theme', 'dark');
        } else {
            htmlElement.classList.remove('dark');
            localStorage.setItem('theme', 'light');
        }
    }

    // Threshold management functions
    async function loadThresholdConfigurations() {
        const loadingEl = document.getElementById('thresholds-loading');
        const containerEl = document.getElementById('thresholds-container');
        const listEl = document.getElementById('thresholds-list');
        const emptyEl = document.getElementById('thresholds-empty');
        
        if (!loadingEl || !containerEl || !listEl || !emptyEl) return;
        
        try {
            loadingEl.classList.remove('hidden');
            containerEl.classList.add('hidden');
            
            const response = await fetch('/api/thresholds');
            const result = await response.json();
            
            if (result.success) {
                const thresholds = result.data || [];
                
                loadingEl.classList.add('hidden');
                containerEl.classList.remove('hidden');
                
                if (thresholds.length === 0) {
                    listEl.innerHTML = '';
                    emptyEl.classList.remove('hidden');
                } else {
                    emptyEl.classList.add('hidden');
                    renderThresholdsList(thresholds);
                }
                
                // Set up event handlers
                setupThresholdEventHandlers();
            } else {
                throw new Error(result.error || 'Failed to load thresholds');
            }
        } catch (error) {
            console.error('Error loading threshold configurations:', error);
            loadingEl.innerHTML = `
                <div class="text-center text-red-600 dark:text-red-400">
                    <p>Failed to load threshold configurations</p>
                    <p class="text-sm">${error.message}</p>
                </div>
            `;
        }
    }
    
    function renderThresholdsList(thresholds) {
        const listEl = document.getElementById('thresholds-list');
        if (!listEl) return;
        
        listEl.innerHTML = thresholds.map(config => `
            <div class="threshold-config-item px-6 py-4" data-endpoint="${config.endpointId}" data-node="${config.nodeId}" data-vmid="${config.vmid}">
                <div class="flex items-center justify-between">
                    <div class="flex items-center space-x-3">
                        <div class="flex-shrink-0">
                            <div class="w-3 h-3 rounded-full ${config.enabled ? 'bg-green-400' : 'bg-gray-400'}"></div>
                        </div>
                        <div>
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                ${config.endpointId}:${config.vmid}
                            </div>
                            <div class="text-sm text-gray-500 dark:text-gray-400">
                                ${renderThresholdSummary(config.thresholds)} • Node: ${config.nodeId}
                            </div>
                        </div>
                    </div>
                    <div class="flex items-center space-x-2">
                        <button type="button" class="edit-threshold-btn text-blue-600 hover:text-blue-500 text-sm font-medium">
                            Edit
                        </button>
                        <button type="button" class="toggle-threshold-btn text-${config.enabled ? 'red' : 'green'}-600 hover:text-${config.enabled ? 'red' : 'green'}-500 text-sm font-medium">
                            ${config.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button type="button" class="delete-threshold-btn text-red-600 hover:text-red-500 text-sm font-medium">
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        `).join('');
    }
    
    function renderThresholdSummary(thresholds) {
        const parts = [];
        if (thresholds.cpu) parts.push(`CPU: ${thresholds.cpu.warning}%/${thresholds.cpu.critical}%`);
        if (thresholds.memory) parts.push(`Memory: ${thresholds.memory.warning}%/${thresholds.memory.critical}%`);
        if (thresholds.disk) parts.push(`Disk: ${thresholds.disk.warning}%/${thresholds.disk.critical}%`);
        return parts.join(' • ');
    }
    
    function setupThresholdEventHandlers() {
        // Add threshold button
        const addBtn = document.getElementById('add-threshold-btn');
        if (addBtn) {
            addBtn.addEventListener('click', (e) => {
                e.preventDefault();
                showThresholdModal();
            });
        }
        
        // Edit, toggle, delete buttons
        document.querySelectorAll('.edit-threshold-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const item = e.target.closest('.threshold-config-item');
                if (item) {
                    editThresholdConfiguration(
                        item.dataset.endpoint,
                        item.dataset.node,
                        item.dataset.vmid
                    );
                }
            });
        });
        
        document.querySelectorAll('.toggle-threshold-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const item = e.target.closest('.threshold-config-item');
                if (item) {
                    toggleThresholdConfiguration(
                        item.dataset.endpoint,
                        item.dataset.node,
                        item.dataset.vmid
                    );
                }
            });
        });
        
        document.querySelectorAll('.delete-threshold-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const item = e.target.closest('.threshold-config-item');
                if (item) {
                    deleteThresholdConfiguration(
                        item.dataset.endpoint,
                        item.dataset.node,
                        item.dataset.vmid
                    );
                }
            });
        });
    }
    
    function showThresholdModal(endpointId = '', nodeId = '', vmid = '', existingThresholds = null) {
        // Get current VMs/LXCs from app state
        const currentState = PulseApp.state.get();
        const allGuests = [];
        
        // Check if we have dashboard data (which contains VMs/LXCs)
        const dashboardData = PulseApp.state.get('dashboardData') || [];
        
        // Add all guests from dashboard data
        dashboardData.forEach(guest => {
            if (guest && guest.name && guest.id) {
                allGuests.push({
                    name: guest.name,
                    id: guest.id,
                    type: guest.type === 'qemu' ? 'VM' : 'LXC',
                    endpointId: guest.endpointId || 'primary',
                    node: guest.node
                });
            }
        });
        
        // Fallback: try to get from state.vms and state.containers if dashboard data is not available
        if (allGuests.length === 0 && currentState) {
            if (currentState.vms && Array.isArray(currentState.vms)) {
                allGuests.push(...currentState.vms.map(vm => ({...vm, type: 'VM'})));
            }
            if (currentState.containers && Array.isArray(currentState.containers)) {
                allGuests.push(...currentState.containers.map(ct => ({...ct, type: 'LXC'})));
            }
        }
        
        // Create modal HTML with dropdown
        const modalHTML = `
            <div id="threshold-modal" class="fixed inset-0 z-50 overflow-y-auto">
                <div class="flex items-end justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
                    <div class="fixed inset-0 transition-opacity" aria-hidden="true">
                        <div class="absolute inset-0 bg-gray-500 opacity-75"></div>
                    </div>
                    <div class="inline-block align-bottom bg-white dark:bg-gray-800 rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
                        <form id="threshold-form">
                            <div class="bg-white dark:bg-gray-800 px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
                                <div class="sm:flex sm:items-start">
                                    <div class="w-full">
                                        <h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-gray-100 mb-4">
                                            ${existingThresholds ? 'Edit' : 'Add'} Custom Threshold Configuration
                                        </h3>
                                        
                                        <div class="space-y-4">
                                            <div>
                                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Select VM/LXC</label>
                                                <select id="guest-selector" class="mt-1 block w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 dark:bg-gray-700 dark:text-gray-100" required>
                                                    <option value="">Choose a VM or LXC...</option>
                                                    ${allGuests.length > 0 ? 
                                                        allGuests.map(guest => `
                                                            <option value="${guest.endpointId}:${guest.id}" 
                                                                    ${(guest.endpointId === endpointId && guest.id === vmid) ? 'selected' : ''}>
                                                                ${guest.name} (${guest.id})
                                                            </option>
                                                        `).join('') 
                                                        : '<option value="" disabled>No VMs or LXCs found. Please wait for data to load...</option>'
                                                    }
                                                </select>
                                                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Select the VM or LXC you want to configure custom alert thresholds for</p>
                                            </div>
                                            
                                            <!-- Hidden fields to store the parsed values -->
                                            <input type="hidden" id="threshold-endpoint" value="${endpointId}">
                                            <input type="hidden" id="threshold-node" value="${nodeId}">
                                            <input type="hidden" id="threshold-vmid" value="${vmid}">
                                            
                                            <div class="space-y-4">
                                                ${renderThresholdInputs('cpu', existingThresholds?.cpu)}
                                                ${renderThresholdInputs('memory', existingThresholds?.memory)}
                                                ${renderThresholdInputs('disk', existingThresholds?.disk)}
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="bg-gray-50 dark:bg-gray-700 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
                                <button type="submit" class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-blue-600 text-base font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 sm:ml-3 sm:w-auto sm:text-sm">
                                    ${existingThresholds ? 'Update' : 'Create'}
                                </button>
                                <button type="button" id="threshold-modal-cancel" class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 dark:border-gray-600 shadow-sm px-4 py-2 bg-white dark:bg-gray-600 text-base font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-500 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm">
                                    Cancel
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        `;
        
        // Add modal to body
        document.body.insertAdjacentHTML('beforeend', modalHTML);
        
        // Set up event handlers
        const modal = document.getElementById('threshold-modal');
        const form = document.getElementById('threshold-form');
        const cancelBtn = document.getElementById('threshold-modal-cancel');
        
        cancelBtn.addEventListener('click', () => {
            modal.remove();
        });
        
        // Set up guest selector dropdown handler
        const guestSelector = document.getElementById('guest-selector');
        const endpointField = document.getElementById('threshold-endpoint');
        const nodeField = document.getElementById('threshold-node');
        const vmidField = document.getElementById('threshold-vmid');
        
        // If editing existing threshold, select the correct guest in dropdown
        if (endpointId && vmid) {
            const selectValue = `${endpointId}:${vmid}`;
            guestSelector.value = selectValue;
            
            // Trigger change event to populate hidden fields properly
            const changeEvent = new Event('change');
            guestSelector.dispatchEvent(changeEvent);
        }
        
        guestSelector.addEventListener('change', (e) => {
            if (e.target.value) {
                const [selectedEndpoint, selectedVmid] = e.target.value.split(':');
                endpointField.value = selectedEndpoint;
                vmidField.value = selectedVmid;
                
                // Find the current node for this VM (for display purposes)
                const selectedGuest = allGuests.find(g => g.endpointId === selectedEndpoint && g.id === selectedVmid);
                nodeField.value = selectedGuest ? selectedGuest.node : '';
                
                // Debug logging
                console.log('[Settings] Guest selector changed:', {
                    selectedEndpoint,
                    selectedVmid,
                    selectedGuest,
                    nodeValue: selectedGuest ? selectedGuest.node : 'MISSING'
                });
            } else {
                endpointField.value = '';
                nodeField.value = '';
                vmidField.value = '';
            }
        });

        // Set up checkbox event handlers to enable/disable threshold inputs
        ['cpu', 'memory', 'disk'].forEach(metric => {
            const checkbox = document.getElementById(`${metric}-enabled`);
            const container = checkbox.closest('.border').querySelector('.grid');
            
            checkbox.addEventListener('change', (e) => {
                if (e.target.checked) {
                    container.style.opacity = '1';
                    container.style.pointerEvents = 'auto';
                } else {
                    container.style.opacity = '0.5';
                    container.style.pointerEvents = 'none';
                }
            });
        });

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(form);
            const thresholds = {};
            
            // Collect threshold data
            ['cpu', 'memory', 'disk'].forEach(metric => {
                const warningEl = document.getElementById(`${metric}-warning`);
                const criticalEl = document.getElementById(`${metric}-critical`);
                const enabledEl = document.getElementById(`${metric}-enabled`);
                
                if (enabledEl && enabledEl.checked && warningEl.value && criticalEl.value) {
                    thresholds[metric] = {
                        warning: parseInt(warningEl.value),
                        critical: parseInt(criticalEl.value)
                    };
                }
            });
            
            const endpointId = document.getElementById('threshold-endpoint').value;
            const nodeId = document.getElementById('threshold-node').value;
            const vmid = document.getElementById('threshold-vmid').value;
            
            // Validate required fields
            if (!endpointId || !nodeId || !vmid) {
                alert('Please select a VM/LXC from the dropdown first.');
                return;
            }
            
            console.log('[Settings] Saving thresholds for:', { endpointId, nodeId, vmid });
            
            try {
                const method = existingThresholds ? 'PUT' : 'POST';
                const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`, {
                    method,
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ thresholds })
                });
                
                const result = await response.json();
                
                if (result.success) {
                    modal.remove();
                    await loadThresholdConfigurations(); // Reload the list
                } else {
                    alert('Failed to save threshold configuration: ' + result.error);
                }
            } catch (error) {
                alert('Error saving threshold configuration: ' + error.message);
            }
        });
    }
    
    function renderThresholdInputs(metric, existing) {
        const enabled = existing ? true : false;
        const warning = existing?.warning || '';
        const critical = existing?.critical || '';
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-4">
                <div class="flex items-center justify-between mb-3">
                    <label class="text-sm font-medium text-gray-900 dark:text-gray-100 capitalize">${metric} Thresholds</label>
                    <input type="checkbox" id="${metric}-enabled" ${enabled ? 'checked' : ''} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
                </div>
                <div class="grid grid-cols-2 gap-3" ${enabled ? '' : 'style="opacity: 0.5; pointer-events: none;"'}>
                    <div>
                        <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Warning (%)</label>
                        <input type="number" id="${metric}-warning" value="${warning}" min="0" max="100" class="w-full text-sm border border-gray-300 dark:border-gray-600 rounded px-2 py-1 dark:bg-gray-700 dark:text-gray-100">
                    </div>
                    <div>
                        <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Critical (%)</label>
                        <input type="number" id="${metric}-critical" value="${critical}" min="0" max="100" class="w-full text-sm border border-gray-300 dark:border-gray-600 rounded px-2 py-1 dark:bg-gray-700 dark:text-gray-100">
                    </div>
                </div>
            </div>
        `;
    }
    
    async function editThresholdConfiguration(endpointId, nodeId, vmid) {
        try {
            const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`);
            const result = await response.json();
            
            if (result.success) {
                showThresholdModal(endpointId, nodeId, vmid, result.data.thresholds);
            } else {
                alert('Failed to load threshold configuration: ' + result.error);
            }
        } catch (error) {
            alert('Error loading threshold configuration: ' + error.message);
        }
    }
    
    async function toggleThresholdConfiguration(endpointId, nodeId, vmid) {
        try {
            // Get current state
            const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`);
            const result = await response.json();
            
            if (result.success) {
                const newEnabledState = !result.data.enabled;
                
                const toggleResponse = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}/toggle`, {
                    method: 'PATCH',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ enabled: newEnabledState })
                });
                
                const toggleResult = await toggleResponse.json();
                
                if (toggleResult.success) {
                    await loadThresholdConfigurations(); // Reload the list
                } else {
                    alert('Failed to toggle threshold configuration: ' + toggleResult.error);
                }
            }
        } catch (error) {
            alert('Error toggling threshold configuration: ' + error.message);
        }
    }
    
    async function deleteThresholdConfiguration(endpointId, nodeId, vmid) {
        if (!confirm(`Are you sure you want to delete the custom threshold configuration for ${endpointId}:${nodeId}:${vmid}?`)) {
            return;
        }
        
        try {
            const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`, {
                method: 'DELETE'
            });
            
            const result = await response.json();
            
            if (result.success) {
                await loadThresholdConfigurations(); // Reload the list
            } else {
                alert('Failed to delete threshold configuration: ' + result.error);
            }
        } catch (error) {
            alert('Error deleting threshold configuration: ' + error.message);
        }
    }

    // Public API
    return {
        init,
        openModal,
        closeModal,
        addPveEndpoint,
        addPbsEndpoint,
        removeEndpoint,
        testConnections,
        checkForUpdates,
        applyUpdate,
        changeTheme
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.settings.init);
} else {
    PulseApp.ui.settings.init();
}