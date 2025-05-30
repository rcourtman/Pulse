PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.settings = (() => {
    let currentConfig = {};
    let isInitialized = false;

    function init() {
        if (isInitialized) return;
        
        console.log('[Settings] Initializing settings module...');
        
        // Set up modal event listeners
        const settingsButton = document.getElementById('settings-button');
        const modal = document.getElementById('settings-modal');
        const closeButton = document.getElementById('settings-modal-close');
        const cancelButton = document.getElementById('settings-cancel-button');
        const testButton = document.getElementById('settings-test-button');
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

        if (testButton) {
            testButton.addEventListener('click', testConnections);
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
        
        isInitialized = true;
        console.log('[Settings] Settings module initialized');
    }

    async function openModal() {
        console.log('[Settings] Opening settings modal...');
        
        const modal = document.getElementById('settings-modal');
        const body = document.getElementById('settings-modal-body');
        
        if (!modal || !body) return;
        
        // Show modal
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        
        // Load current configuration
        try {
            await loadCurrentConfig();
        } catch (error) {
            console.error('[Settings] Failed to load configuration:', error);
            // Initialize with empty config to prevent errors
            currentConfig = {};
        }
        
        // Always render the form, even if config loading failed
        renderConfigurationForm();
    }

    function closeModal() {
        console.log('[Settings] Closing settings modal...');
        
        const modal = document.getElementById('settings-modal');
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
    }

    async function loadCurrentConfig() {
        try {
            const response = await fetch('/api/config');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const configData = await response.json();
            currentConfig = configData || {}; // Ensure we never have null/undefined
            console.log('[Settings] Current config loaded:', currentConfig);
            
        } catch (error) {
            console.error('[Settings] Failed to load current configuration:', error);
            throw error;
        }
    }

    function renderConfigurationForm() {
        const body = document.getElementById('settings-modal-body');
        if (!body) return;

        // Ensure currentConfig has a safe default structure
        const safeConfig = currentConfig || {};
        const proxmox = safeConfig.proxmox || {};
        const pbs = safeConfig.pbs || {};
        const advanced = safeConfig.advanced || {};
        const alerts = advanced.alerts || {};

        const html = `
            <form id="settings-form" class="space-y-6">
                <!-- Proxmox VE Primary Endpoint -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <div class="mb-4">
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Primary Proxmox VE Server</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Main PVE server configuration (required)</p>
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

                <!-- Primary PBS Configuration -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <div class="mb-4">
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Primary Proxmox Backup Server</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Main PBS server configuration (optional)</p>
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

                <!-- Service Settings -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
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

                <!-- Alert Settings -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Alert Configuration</h3>
                    
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

                <!-- Status Messages -->
                <div id="settings-messages"></div>
            </form>
        `;

        body.innerHTML = html;
        
        // Render additional endpoints
        renderAdditionalEndpoints();
    }

    function renderAdditionalEndpoints() {
        // This will be populated based on what additional endpoints exist in config
        // For now, we'll support dynamic addition
    }

    function addPveEndpoint() {
        const container = document.getElementById('pve-endpoints-container');
        if (!container) return;

        // Hide empty state message when adding first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

        const index = container.children.length + 1; // Start from _2 (but adjust for hidden empty state)
        const endpointHtml = `
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-2 right-2 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <h4 class="text-md font-medium text-gray-800 dark:text-gray-200 mb-3">PVE Server #${index}</h4>
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

        // Hide empty state message when adding first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

        const index = container.children.length + 1; // Start from _2 (but adjust for hidden empty state)
        const endpointHtml = `
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-2 right-2 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <h4 class="text-md font-medium text-gray-800 dark:text-gray-200 mb-3">PBS Server #${index}</h4>
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
        const container = button.closest('#pve-endpoints-container, #pbs-endpoints-container');
        const endpointDiv = button.parentElement;
        
        // Remove the endpoint
        endpointDiv.remove();
        
        // Check if this was the last additional endpoint
        const remainingEndpoints = container.querySelectorAll('.border:not(.border-dashed)');
        if (remainingEndpoints.length === 0) {
            // Show empty state again
            const emptyState = container.querySelector('.border-dashed');
            if (emptyState) {
                emptyState.style.display = 'block';
            }
        }
    }

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

    function showMessage(message, type) {
        const container = document.getElementById('settings-messages');
        if (!container) return;

        const typeClasses = {
            error: 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800 text-red-700 dark:text-red-300',
            success: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-700 dark:text-green-300',
            info: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300'
        };

        const html = `
            <div class="border rounded-lg p-3 ${typeClasses[type] || typeClasses.info}">
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

    // Public API
    return {
        init,
        openModal,
        closeModal,
        addPveEndpoint,
        addPbsEndpoint,
        removeEndpoint,
        testConnections,
        saveConfiguration
    };
})();