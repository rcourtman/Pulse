PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.settings = (() => {
    let currentConfig = {};
    let isInitialized = false;
    let activeTab = 'proxmox';
    let latestReleaseData = null; // Store the latest release data
    let updateCache = new Map(); // Cache update check results to reduce API calls
    let updateCheckTimeout = null; // Debounce rapid channel changes
    let formDataCache = {}; // Store form data between tab switches

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
        // Preserve current form data before switching tabs
        preserveCurrentFormData();
        
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
        
        // Restore form data for the new tab
        restoreFormData();
    }

    async function openModal() {
        await openModalWithTab('proxmox');
    }
    
    async function openModalWithTab(tabName) {
        
        const modal = document.getElementById('settings-modal');
        if (!modal) return;

        // Show the modal
        modal.classList.remove('hidden');
        modal.classList.add('flex');

        // Load current configuration
        await loadConfiguration();
        
        // Switch to requested tab
        switchTab(tabName);
    }

    function closeModal() {
        // Preserve current form data before closing
        preserveCurrentFormData();
        
        const modal = document.getElementById('settings-modal');
        if (!modal) return;

        modal.classList.add('hidden');
        modal.classList.remove('flex');
        
        // Clear form data cache since modal is being closed
        formDataCache = {};
    }

    async function loadConfiguration() {
        try {
            const response = await fetch('/api/config');
            const data = await response.json();
            
            if (response.ok) {
                currentConfig = data;
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
            case 'system':
                content = renderSystemTab(advanced, safeConfig);
                break;
            case 'diagnostics':
                content = renderDiagnosticsTab();
                break;
        }

        container.innerHTML = `<form id="settings-form" class="space-y-6">${content}</form>`;
        
        // Restore form data after content is rendered
        setTimeout(() => {
            restoreFormData();
        }, 0);
        
        // Load existing additional endpoints for Proxmox and PBS tabs
        if (activeTab === 'proxmox') {
            loadExistingPveEndpoints();
        } else if (activeTab === 'pbs') {
            loadExistingPbsEndpoints();
        } else if (activeTab === 'system') {
            // Auto-check for latest version when system tab is opened
            checkLatestVersion();
            // Initialize update channel warning visibility
            setTimeout(() => {
                const channelSelect = document.querySelector('select[name="UPDATE_CHANNEL"]');
                if (channelSelect) {
                    onUpdateChannelChange(channelSelect.value);
                }
            }, 0);
        } else if (activeTab === 'alerts') {
            // Load threshold configurations when alerts tab is opened
            loadThresholdConfigurations();
        }
    }

    function renderProxmoxTab(proxmox, config) {
        // Handle both structured config (proxmox object) and flat config (direct env vars)
        let host = proxmox?.host || config.PROXMOX_HOST || '';
        let port = proxmox?.port || config.PROXMOX_PORT || '';
        const tokenId = proxmox?.tokenId || config.PROXMOX_TOKEN_ID || '';
        const nodeName = proxmox?.nodeName || config.PROXMOX_NODE_NAME || '';
        const enabled = proxmox?.enabled !== undefined ? proxmox.enabled : (config.PROXMOX_ENABLED !== 'false');
        
        // Clean the host value if it contains protocol or port, and extract port if needed
        if (host) {
            const originalHost = host;
            // Remove protocol (http:// or https://)
            host = host.replace(/^https?:\/\//, '');
            // Extract port if it's included in the host (e.g., "proxmox.lan:8006")
            const portMatch = host.match(/^([^:]+)(:(\d+))?$/);
            if (portMatch) {
                host = portMatch[1];
                // Use extracted port if no explicit port was set
                if (portMatch[3] && !port) {
                    port = portMatch[3];
                }
            }
        }
        
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
                               value="${host}"
                               placeholder="proxmox.example.com"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT"
                               value="${port}"
                               placeholder="8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox VE web interface port</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Node Name
                        </label>
                        <input type="text" name="PROXMOX_NODE_NAME"
                               value="${nodeName}"
                               placeholder="Display name (optional)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token ID <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_TOKEN_ID" required
                               value="${tokenId}"
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
                        <input type="checkbox" name="PROXMOX_ENABLED" ${enabled ? 'checked' : ''}
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
        // Handle both structured config (pbs object) and flat config (direct env vars)
        let host = pbs?.host || config.PBS_HOST || '';
        let port = pbs?.port || config.PBS_PORT || '';
        const tokenId = pbs?.tokenId || config.PBS_TOKEN_ID || '';
        const nodeName = pbs?.nodeName || config.PBS_NODE_NAME || '';
        
        // Clean the host value if it contains protocol or port, and extract port if needed
        if (host) {
            const originalHost = host;
            // Remove protocol (http:// or https://)
            host = host.replace(/^https?:\/\//, '');
            // Extract port if it's included in the host (e.g., "192.168.0.16:8007")
            const portMatch = host.match(/^([^:]+)(:(\d+))?$/);
            if (portMatch) {
                host = portMatch[1];
                // Use extracted port if no explicit port was set
                if (portMatch[3] && !port) {
                    port = portMatch[3];
                }
            }
        }
        
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
                               value="${host}"
                               placeholder="pbs.example.com"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PBS_PORT"
                               value="${port}"
                               placeholder="8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox Backup Server web interface port</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Node Name
                        </label>
                        <input type="text" name="PBS_NODE_NAME"
                               value="${nodeName}"
                               placeholder="PBS internal hostname"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PBS_TOKEN_ID"
                               value="${tokenId}"
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
                
                <!-- Update Channel Preference -->
                <div class="mb-6 p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="mb-4">
                        <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">Update Channel</h4>
                        <p class="text-sm text-gray-600 dark:text-gray-400 mb-3">
                            Choose which types of updates to receive
                        </p>
                        <select name="UPDATE_CHANNEL" onchange="PulseApp.ui.settings.onUpdateChannelChange(this.value)"
                                class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                            <option value="stable" ${(advanced.updateChannel || 'stable') === 'stable' ? 'selected' : ''}>
                                Stable - Production releases (recommended)
                            </option>
                            <option value="rc" ${(advanced.updateChannel || 'stable') === 'rc' ? 'selected' : ''}>
                                Release Candidate - Test fixes and new features
                            </option>
                        </select>
                        <div id="update-channel-description" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                            <strong>Stable:</strong> Thoroughly tested releases for production use
                        </div>
                        <div id="rc-warning" class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg hidden">
                            <div class="flex items-start gap-2">
                                <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                                </svg>
                                <div>
                                    <h5 class="text-sm font-semibold text-amber-800 dark:text-amber-200">Release Candidate Warning</h5>
                                    <p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
                                        RC versions are pre-release software for testing fixes and new features. 
                                        They may contain bugs. Only select this if you want to help test and provide feedback.
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                
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
                                <span id="latest-version-label">Latest Version</span>: <span id="latest-version" class="font-mono font-semibold text-gray-500 dark:text-gray-400">Checking...</span>
                            </p>
                            <p id="update-channel-info" class="text-sm text-gray-500 dark:text-gray-400 mt-1"></p>
                            <div id="channel-mismatch-warning" class="hidden mt-3 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                                <div class="flex items-start gap-2">
                                    <svg class="w-4 h-4 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/>
                                    </svg>
                                    <div>
                                        <h5 class="text-sm font-semibold text-blue-800 dark:text-blue-200">Channel Recommendation</h5>
                                        <p id="channel-mismatch-message" class="text-sm text-blue-700 dark:text-blue-300 mt-1"></p>
                                        <div class="mt-2 space-x-3">
                                            <button type="button" id="switch-to-stable-btn" class="text-sm text-green-600 dark:text-green-400 hover:text-green-800 dark:hover:text-green-200 underline font-medium" onclick="PulseApp.ui.settings.proceedWithStableSwitch()">
                                                Switch to stable release
                                            </button>
                                            <button type="button" id="cancel-switch-btn" class="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline" onclick="PulseApp.ui.settings.switchToChannel('rc')">
                                                Cancel (stay on RC)
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            </div>
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
                            <div class="flex items-center justify-between mb-3">
                                <h4 class="text-sm font-semibold text-blue-800 dark:text-blue-200">
                                    Update Available: <span id="update-version"></span>
                                </h4>
                                <span id="update-published-badge" class="text-xs text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-800 px-2 py-1 rounded"></span>
                            </div>
                            
                            <!-- Compact Changes Summary -->
                            <div id="changes-summary" class="mb-3">
                                <div id="changes-loading" class="flex items-center gap-2 text-gray-500 dark:text-gray-400 text-sm">
                                    <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    Loading changes...
                                </div>
                                <div id="changes-summary-text" class="hidden text-sm text-gray-700 dark:text-gray-300"></div>
                                <div id="changes-error" class="hidden text-red-600 dark:text-red-400 text-sm"></div>
                            </div>
                            
                            <!-- Expandable Details -->
                            <div class="border-t border-blue-200 dark:border-blue-700 pt-3">
                                <button type="button" id="toggle-details-btn" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 flex items-center gap-1">
                                    <span>Show details</span>
                                    <svg id="details-chevron" class="w-4 h-4 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                                    </svg>
                                </button>
                                
                                <div id="update-details-expanded" class="hidden mt-3 space-y-3">
                                    <div id="detailed-changes" class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded p-3">
                                        <h5 class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-2">Commit Details</h5>
                                        <div id="changes-list" class="space-y-2 max-h-64 overflow-y-auto"></div>
                                    </div>
                                    
                                    <div id="release-notes-section" class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded p-3">
                                        <h5 class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-2">Release Notes</h5>
                                        <div id="update-release-notes" class="text-sm text-gray-700 dark:text-gray-300 prose prose-sm max-w-none max-h-48 overflow-y-auto"></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <div class="flex justify-end">
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

    function renderDiagnosticsTab() {
        return `
            <div class="space-y-6">
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-2">System Diagnostics</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
                        Generate a comprehensive diagnostic report to help troubleshoot issues with your Pulse configuration.
                    </p>
                    
                    <div class="flex items-center gap-4 mb-4">
                        <button type="button" id="runDiagnostics" onclick="PulseApp.ui.settings.runDiagnostics()" 
                                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                            Run Diagnostics
                        </button>
                        <button type="button" id="copyReport" onclick="PulseApp.ui.settings.copyDiagnosticReport()" 
                                style="display: none;" title="Safe to share - all sensitive data has been sanitized"
                                class="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors flex items-center gap-2">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path>
                            </svg>
                            Copy Safe Report
                        </button>
                        <button type="button" id="downloadReport" onclick="PulseApp.ui.settings.downloadDiagnosticReport()" 
                                style="display: none;" title="Safe to share - all sensitive data has been sanitized"
                                class="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors flex items-center gap-2">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                            </svg>
                            Download Safe Report
                        </button>
                    </div>
                    
                    <div id="diagnostics-status" class="hidden mb-4 p-3 rounded-lg text-sm font-medium"></div>
                    
                    <div id="diagnostics-results" style="display: none;" class="space-y-4 mt-6">
                        <!-- Results will be populated here -->
                    </div>
                </div>
                
                <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
                    <div class="flex">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">Privacy Notice</h3>
                            <div class="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
                                <p>The diagnostic report displays real hostnames, IPs, and other potentially sensitive information for troubleshooting purposes.</p>
                                <p class="mt-1 font-semibold">When you copy or download the report, all sensitive data is automatically sanitized for safe sharing.</p>
                            </div>
                        </div>
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
        
        // Find the next available index by checking existing endpoints
        let index = 2;
        const usedIndexes = new Set();
        existingEndpoints.forEach(endpoint => {
            const header = endpoint.querySelector('h4');
            if (header) {
                const match = header.textContent.match(/#(\d+)/);
                if (match) {
                    usedIndexes.add(parseInt(match[1]));
                }
            }
        });
        
        // Find the first unused index starting from 2
        while (usedIndexes.has(index)) {
            index++;
        }
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
                        <input type="text" name="PROXMOX_HOST_${index}" placeholder="pve${index}.example.com"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT_${index}" placeholder="8006"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox VE web interface port</p>
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
        
        // Find the next available index by checking existing endpoints
        let index = 2;
        const usedIndexes = new Set();
        existingEndpoints.forEach(endpoint => {
            const header = endpoint.querySelector('h4');
            if (header) {
                const match = header.textContent.match(/#(\d+)/);
                if (match) {
                    usedIndexes.add(parseInt(match[1]));
                }
            }
        });
        
        // Find the first unused index starting from 2
        while (usedIndexes.has(index)) {
            index++;
        }
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
                        <input type="text" name="PBS_HOST_${index}" placeholder="pbs${index}.example.com"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PBS_PORT_${index}" placeholder="8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox Backup Server web interface port</p>
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
        // Find all PROXMOX_HOST_N keys in the config
        const pveHostKeys = Object.keys(config)
            .filter(key => key.match(/^PROXMOX_HOST_\d+$/))
            .map(key => {
                const match = key.match(/^PROXMOX_HOST_(\d+)$/);
                return match ? parseInt(match[1]) : null;
            })
            .filter(num => num !== null && num > 1) // Exclude primary endpoint (no suffix)
            .sort((a, b) => a - b);
        
        for (const i of pveHostKeys) {
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
                            let value = config[configKey];
                            
                            // Clean host values that contain protocol or port
                            if (configKey.includes('PROXMOX_HOST_') && value) {
                                // Remove protocol (http:// or https://)
                                value = value.replace(/^https?:\/\//, '');
                                // Extract port if it's included in the host
                                const portMatch = value.match(/^([^:]+)(:(\d+))?$/);
                                if (portMatch) {
                                    value = portMatch[1];
                                    // If we extracted a port and there's no explicit port config, set it
                                    if (portMatch[3]) {
                                        const portKey = configKey.replace('HOST_', 'PORT_');
                                        const portInput = newEndpoint.querySelector(`[name="${portKey}"]`);
                                        if (portInput && !config[portKey]) {
                                            portInput.value = portMatch[3];
                                        }
                                    }
                                }
                            }
                            
                            input.value = value;
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
        // Find all PBS_HOST_N keys in the config
        const pbsHostKeys = Object.keys(config)
            .filter(key => key.match(/^PBS_HOST_\d+$/))
            .map(key => {
                const match = key.match(/^PBS_HOST_(\d+)$/);
                return match ? parseInt(match[1]) : null;
            })
            .filter(num => num !== null && num > 1) // Exclude primary endpoint (no suffix)
            .sort((a, b) => a - b);
        
        for (const i of pbsHostKeys) {
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
                            let value = config[configKey];
                            
                            // Clean host values that contain protocol or port
                            if (configKey.includes('PBS_HOST_') && value) {
                                // Remove protocol (http:// or https://)
                                value = value.replace(/^https?:\/\//, '');
                                // Extract port if it's included in the host
                                const portMatch = value.match(/^([^:]+)(:(\d+))?$/);
                                if (portMatch) {
                                    value = portMatch[1];
                                    // If we extracted a port and there's no explicit port config, set it
                                    if (portMatch[3]) {
                                        const portKey = configKey.replace('HOST_', 'PORT_');
                                        const portInput = newEndpoint.querySelector(`[name="${portKey}"]`);
                                        if (portInput && !config[portKey]) {
                                            portInput.value = portMatch[3];
                                        }
                                    }
                                }
                            }
                            
                            input.value = value;
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
            // Preserve current tab data before collecting all data
            preserveCurrentFormData();
            const config = collectAllTabsData();
            
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                showSuccessToast('Configuration Saved', 'Your settings have been applied successfully');
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
                // Clean PBS_HOST entries - remove protocol and port if included
                if (name.startsWith('PBS_HOST')) {
                    let cleanHost = value.trim();
                    // Remove protocol (http:// or https://)
                    cleanHost = cleanHost.replace(/^https?:\/\//, '');
                    // Remove port if it's included in the host (e.g., "192.168.1.1:8007")
                    const portMatch = cleanHost.match(/^([^:]+)(:\d+)?$/);
                    if (portMatch) {
                        cleanHost = portMatch[1];
                    }
                    config[name] = cleanHost;
                } else {
                    config[name] = value;
                }
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

    function collectAllTabsData() {
        // Start with current form data and add cached data from other tabs
        const currentFormConfig = collectFormData();
        const allConfig = { ...currentFormConfig };

        // Merge data from all cached tabs
        Object.values(formDataCache).forEach(tabData => {
            Object.entries(tabData).forEach(([name, value]) => {
                // Only use cached value if not already set from current form
                if (!allConfig.hasOwnProperty(name)) {
                    if (typeof value === 'boolean') {
                        allConfig[name] = value ? 'true' : 'false';
                    } else {
                        allConfig[name] = value;
                    }
                }
            });
        });

        return allConfig;
    }

    function preserveCurrentFormData() {
        const form = document.getElementById('settings-form');
        if (!form) return;

        const formData = new FormData(form);
        const currentTabData = {};

        // Store all form field values
        for (const [name, value] of formData.entries()) {
            const field = form.querySelector(`[name="${name}"]`);
            if (field) {
                if (field.type === 'checkbox') {
                    currentTabData[name] = field.checked;
                } else {
                    currentTabData[name] = value;
                }
            }
        }

        // Also store unchecked checkboxes
        const checkboxes = form.querySelectorAll('input[type="checkbox"]');
        checkboxes.forEach(checkbox => {
            if (!checkbox.checked && !currentTabData.hasOwnProperty(checkbox.name)) {
                currentTabData[checkbox.name] = false;
            }
        });

        // Only cache data if it contains meaningful configuration
        if (hasSignificantConfiguration(currentTabData, activeTab)) {
            formDataCache[activeTab] = currentTabData;
            } else {
            // Clear cache for this tab if no significant data
            delete formDataCache[activeTab];
            console.log(`[Settings] No significant data to preserve for tab '${activeTab}', cleared cache`);
        }
    }

    function hasSignificantConfiguration(data, tabName) {
        // Check if the data contains any non-empty, meaningful values
        if (tabName === 'proxmox') {
            // For Proxmox tab, require at least a host to be considered significant
            return !!(data.PROXMOX_HOST && data.PROXMOX_HOST.trim()) ||
                   !!(data.PROXMOX_HOST_2 && data.PROXMOX_HOST_2.trim()) ||
                   !!(data.PROXMOX_HOST_3 && data.PROXMOX_HOST_3.trim());
        } else if (tabName === 'pbs') {
            // For PBS tab, require at least a host to be considered significant
            return !!(data.PBS_HOST && data.PBS_HOST.trim()) ||
                   !!(data.PBS_HOST_2 && data.PBS_HOST_2.trim()) ||
                   !!(data.PBS_HOST_3 && data.PBS_HOST_3.trim());
        } else if (tabName === 'alerts') {
            // For alerts tab, any threshold value or email/webhook config is significant
            return Object.keys(data).some(key => {
                const value = data[key];
                if (typeof value === 'string' && value.trim()) return true;
                if (typeof value === 'boolean' && value) return true;
                if (typeof value === 'number' && value > 0) return true;
                return false;
            });
        }
        
        // For other tabs, check if any field has a non-empty value
        return Object.values(data).some(value => {
            if (typeof value === 'string' && value.trim()) return true;
            if (typeof value === 'boolean' && value) return true;
            if (typeof value === 'number' && value !== 0) return true;
            return false;
        });
    }

    function restoreFormData() {
        const form = document.getElementById('settings-form');
        if (!form) return;

        const savedData = formDataCache[activeTab];
        if (!savedData) return;


        // Restore form field values
        Object.entries(savedData).forEach(([name, value]) => {
            const field = form.querySelector(`[name="${name}"]`);
            if (field) {
                if (field.type === 'checkbox') {
                    field.checked = value === true || value === 'true';
                } else {
                    field.value = value || '';
                }
            }
        });
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

    function showSuccessToast(title, subtitle) {
        const toastId = `toast-${Date.now()}`;
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = 'fixed top-4 right-4 z-50 max-w-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg transform translate-x-full transition-transform duration-300';
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
            toast.classList.remove('translate-x-full');
        });
        
        // Auto remove after 5 seconds
        setTimeout(() => {
            if (document.getElementById(toastId)) {
                toast.classList.add('translate-x-full');
                setTimeout(() => toast.remove(), 300);
            }
        }, 5000);
    }

    // Check for latest version from GitHub releases
    // @param {string} channelOverride - Optional channel to check instead of saved config
    async function checkLatestVersion(channelOverride = null) {
        const latestVersionElement = document.getElementById('latest-version');
        const latestVersionLabelElement = document.getElementById('latest-version-label');
        const versionStatusElement = document.getElementById('version-status');
        const updateChannelInfoElement = document.getElementById('update-channel-info');
        const channelMismatchWarning = document.getElementById('channel-mismatch-warning');
        
        if (!latestVersionElement) return;
        
        try {
            latestVersionElement.textContent = 'Checking...';
            latestVersionElement.className = 'font-mono font-semibold text-gray-500 dark:text-gray-400';
            
            if (updateChannelInfoElement) {
                updateChannelInfoElement.textContent = '';
            }
            
            if (channelMismatchWarning) {
                channelMismatchWarning.classList.add('hidden');
            }
            
            // Check cache first to reduce API calls
            const cacheKey = channelOverride || 'default';
            const cacheExpiry = 5 * 60 * 1000; // 5 minutes
            const cachedResult = updateCache.get(cacheKey);
            
            let data;
            if (cachedResult && (Date.now() - cachedResult.timestamp) < cacheExpiry) {
                console.log(`[Settings] Using cached result for channel: ${cacheKey}`);
                data = cachedResult.data;
            } else {
                // Use the server's update check API with optional channel override
                const url = channelOverride ? `/api/updates/check?channel=${channelOverride}` : '/api/updates/check';
                const response = await fetch(url);
                
                if (!response.ok) {
                    throw new Error(`Server error: ${response.status} ${response.statusText}`);
                }
                
                data = await response.json();
                
                // Cache the result
                updateCache.set(cacheKey, {
                    data: data,
                    timestamp: Date.now()
                });
            }
            
            // Add preview indicator if using channel override
            const isPreview = !!channelOverride;
            
            if (data && data.latestVersion) {
                const latestVersion = data.latestVersion;
                const currentVersion = data.currentVersion || currentConfig.version || 'Unknown';
                // Parse channel from descriptive text (e.g., "RC releases only" -> "rc")
                const rawChannel = data.updateChannel || 'stable';
                const rawChannelLower = rawChannel.toLowerCase();
                const updateChannel = (rawChannelLower.includes('rc') || rawChannelLower.includes('release candidate') || rawChannelLower.includes('alpha') || rawChannelLower.includes('beta')) ? 'rc' : 'stable';
                
                latestVersionElement.textContent = latestVersion;
                
                // Update stored config version if server provided it
                if (data.currentVersion) {
                    currentConfig.version = data.currentVersion;
                }
                
                // Update the label to be channel-specific
                if (latestVersionLabelElement) {
                    if (updateChannel === 'rc') {
                        latestVersionLabelElement.textContent = 'Latest RC Version';
                    } else {
                        latestVersionLabelElement.textContent = 'Latest Stable Version';
                    }
                }
                
                // Display update channel information with more context
                if (updateChannelInfoElement) {
                    const channelName = updateChannel === 'rc' ? 'Release Candidate (RC)' : 'Stable';
                    const previewText = isPreview ? ' (Preview)' : '';
                    updateChannelInfoElement.textContent = `Update channel: ${channelName}${previewText}`;
                    
                    // Add preview styling
                    if (isPreview) {
                        updateChannelInfoElement.classList.add('text-blue-600', 'dark:text-blue-400', 'font-medium');
                    } else {
                        updateChannelInfoElement.classList.remove('text-blue-600', 'dark:text-blue-400', 'font-medium');
                    }
                }
                
                // Check for channel mismatch and show recommendations
                const currentVersionLower = currentVersion.toLowerCase();
                const isCurrentRC = currentVersionLower.includes('-rc') || currentVersionLower.includes('-alpha') || currentVersionLower.includes('-beta');
                const shouldShowRecommendation = (!isCurrentRC && updateChannel === 'rc');
                
                console.log('Channel debug:', { currentVersion, updateChannel, isCurrentRC, shouldShowRecommendation, serverResponse: data });
                
                if (shouldShowRecommendation && channelMismatchWarning) {
                    const messageElement = document.getElementById('channel-mismatch-message');
                    if (messageElement) {
                        messageElement.textContent = `You're running a stable version (${currentVersion}) but checking for RC releases. Consider switching to the stable channel for production use.`;
                    }
                    channelMismatchWarning.classList.remove('hidden');
                }
                
                // Check if this is a "downgrade" scenario (RC to stable)
                const isDowngradeToStable = isCurrentRC && updateChannel === 'stable' && 
                    currentVersion !== latestVersion;
                
                if (data.updateAvailable || isDowngradeToStable) {
                    // Update available (or downgrade to stable)
                    latestVersionElement.className = 'font-mono font-semibold text-green-600 dark:text-green-400';
                    
                    let updateText;
                    if (isDowngradeToStable) {
                        updateText = '📦 Switch to stable release available';
                    } else {
                        updateText = updateChannel === 'rc' ? '📦 RC Update available!' : '📦 Update available!';
                    }
                    
                    versionStatusElement.innerHTML = `<span class="text-green-600 dark:text-green-400">${updateText}</span>`;
                    
                    // Convert server response to match GitHub API format for showUpdateDetails
                    const releaseData = {
                        tag_name: 'v' + latestVersion,
                        body: data.releaseNotes,
                        published_at: data.publishedAt,
                        html_url: data.releaseUrl,
                        assets: data.assets,
                        isDocker: data.isDocker // Store Docker status
                    };
                    
                    // Show update details
                    showUpdateDetails(releaseData);
                    
                    // Show Docker warning if applicable
                    if (data.isDocker) {
                        versionStatusElement.innerHTML += '<br><span class="text-amber-600 dark:text-amber-400 text-xs">Note: Docker deployments require manual update</span>';
                    }
                } else {
                    // Up to date - hide any update details
                    hideUpdateDetails();
                    
                    latestVersionElement.className = 'font-mono font-semibold text-gray-700 dark:text-gray-300';
                    
                    // Check if we should show "up to date" or "no updates" for RC on stable
                    if (isCurrentRC && updateChannel === 'stable' && currentVersion === latestVersion) {
                        // Same version on both channels
                        const upToDateText = '✅ Up to date (same as stable)';
                        versionStatusElement.innerHTML = `<span class="text-green-600 dark:text-green-400">${upToDateText}</span>`;
                    } else if (isCurrentRC && updateChannel === 'stable') {
                        // RC version is different from stable - should have been caught above as "downgrade"
                        const upToDateText = '⚠️ No newer stable (running RC)';
                        versionStatusElement.innerHTML = `<span class="text-amber-600 dark:text-amber-400">${upToDateText}</span>`;
                    } else {
                        const upToDateText = updateChannel === 'rc' ? '✅ Up to date (RC channel)' : '✅ Up to date';
                        versionStatusElement.innerHTML = `<span class="text-green-600 dark:text-green-400">${upToDateText}</span>`;
                    }
                }
            } else {
                throw new Error('Invalid response data - missing version information');
            }
        } catch (error) {
            console.error('Error checking for updates:', error);
            
            // Hide update details on error
            hideUpdateDetails();
            
            latestVersionElement.textContent = 'Error';
            latestVersionElement.className = 'font-mono font-semibold text-red-500';
            
            // Provide more specific error messages
            let errorMessage = 'Failed to check for updates';
            if (error.message.includes('500')) {
                errorMessage = 'Server error - please try again later';
            } else if (error.message.includes('403') || error.message.includes('429')) {
                errorMessage = 'Rate limited - please wait a moment';
            } else if (error.message.includes('Failed to fetch')) {
                errorMessage = 'Network error - check connection';
            }
            
            versionStatusElement.innerHTML = `<span class="text-red-500">${errorMessage}</span>`;
            
            // If it's a rate limit issue, suggest using cached data if available
            if (error.message.includes('403') || error.message.includes('429')) {
                const cacheKey = channelOverride || 'default';
                const staleCache = updateCache.get(cacheKey);
                if (staleCache) {
                    console.log('[Settings] Using stale cache due to rate limiting');
                    // Recursively call with cached data
                    setTimeout(() => {
                        const cachedData = staleCache.data;
                        // Process cached data (simplified version)
                        latestVersionElement.textContent = cachedData.latestVersion || 'Unknown';
                        latestVersionElement.className = 'font-mono font-semibold text-gray-700 dark:text-gray-300';
                        versionStatusElement.innerHTML = '<span class="text-amber-600 dark:text-amber-400">⚠️ Cached data (rate limited)</span>';
                    }, 100);
                }
            }
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
        
        // Store the release data for use in applyUpdate
        latestReleaseData = releaseData;
        
        const updateVersion = document.getElementById('update-version');
        const updatePublishedBadge = document.getElementById('update-published-badge');
        const updateReleaseNotes = document.getElementById('update-release-notes');
        
        if (updateVersion) {
            updateVersion.textContent = releaseData.tag_name;
        }
        
        if (updatePublishedBadge && releaseData.published_at) {
            const publishedDate = new Date(releaseData.published_at).toLocaleDateString();
            updatePublishedBadge.textContent = publishedDate;
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
        
        // Set up toggle functionality
        setupDetailsToggle();
        
        updateDetails.classList.remove('hidden');
        
        // Load commit differences for summary
        loadVersionChanges(releaseData.tag_name);
    }
    
    // Set up the toggle functionality for details expansion
    function setupDetailsToggle() {
        const toggleBtn = document.getElementById('toggle-details-btn');
        const detailsExpanded = document.getElementById('update-details-expanded');
        const chevron = document.getElementById('details-chevron');
        
        if (toggleBtn && detailsExpanded && chevron) {
            toggleBtn.onclick = () => {
                const isExpanded = !detailsExpanded.classList.contains('hidden');
                
                if (isExpanded) {
                    detailsExpanded.classList.add('hidden');
                    chevron.style.transform = 'rotate(0deg)';
                    toggleBtn.querySelector('span').textContent = 'Show details';
                } else {
                    detailsExpanded.classList.remove('hidden');
                    chevron.style.transform = 'rotate(180deg)';
                    toggleBtn.querySelector('span').textContent = 'Hide details';
                }
            };
        }
    }
    
    // Load and display commit differences between versions
    async function loadVersionChanges(targetVersion) {
        const currentVersion = currentConfig.version || 'Unknown';
        const changesLoading = document.getElementById('changes-loading');
        const changesSummaryText = document.getElementById('changes-summary-text');
        const changesList = document.getElementById('changes-list');
        const changesError = document.getElementById('changes-error');
        
        if (currentVersion === 'Unknown') return;
        
        // Determine if we're showing changes between RC and stable or between versions
        const currentVersionLower = currentVersion.toLowerCase();
        const isCurrentRC = currentVersionLower.includes('-rc') || currentVersionLower.includes('-alpha') || currentVersionLower.includes('-beta');
        const isTargetRC = targetVersion.toLowerCase().includes('-rc') || targetVersion.toLowerCase().includes('-alpha') || targetVersion.toLowerCase().includes('-beta');
        
        // Show loading state
        changesLoading.classList.remove('hidden');
        changesSummaryText.classList.add('hidden');
        changesError.classList.add('hidden');
        
        try {
            // GitHub API to compare between versions
            const baseVersion = isCurrentRC && !isTargetRC ? targetVersion : currentVersion;
            const headVersion = isCurrentRC && !isTargetRC ? currentVersion : targetVersion;
            
            // Strip any existing 'v' prefix to avoid double prefixes
            const cleanBaseVersion = baseVersion.replace(/^v/, '');
            const cleanHeadVersion = headVersion.replace(/^v/, '');
            
            const compareUrl = `https://api.github.com/repos/rcourtman/Pulse/compare/v${cleanBaseVersion}...v${cleanHeadVersion}`;
            
            console.log(`[Settings] Fetching version comparison: ${compareUrl}`);
            
            const response = await fetch(compareUrl);
            if (!response.ok) {
                if (response.status === 404) {
                    throw new Error(`Version comparison not found. One of the versions (${cleanBaseVersion} or ${cleanHeadVersion}) may not exist.`);
                } else if (response.status === 403) {
                    throw new Error('GitHub API rate limit exceeded. Please try again later.');
                } else {
                    throw new Error(`Failed to fetch version comparison: ${response.status} ${response.statusText}`);
                }
            }
            
            const compareData = await response.json();
            
            // Hide loading and show results
            changesLoading.classList.add('hidden');
            
            if (compareData.commits && compareData.commits.length > 0) {
                // Create compact summary
                createChangesSummary(compareData.commits, isCurrentRC && !isTargetRC);
                changesSummaryText.classList.remove('hidden');
                
                // Populate detailed view for expandable section
                displayCommitChanges(compareData.commits, isCurrentRC && !isTargetRC);
            } else {
                changesError.textContent = 'No commits found between these versions.';
                changesError.classList.remove('hidden');
            }
            
        } catch (error) {
            console.error('[Settings] Error fetching version changes:', error);
            changesLoading.classList.add('hidden');
            changesError.textContent = `Could not load changes: ${error.message}`;
            changesError.classList.remove('hidden');
        }
    }
    
    // Create a compact summary of changes for the main view
    function createChangesSummary(commits, isRollback = false) {
        const changesSummaryText = document.getElementById('changes-summary-text');
        if (!changesSummaryText) return;
        
        // Categorize commits
        const categories = {
            features: commits.filter(c => c.commit.message.toLowerCase().startsWith('feat')),
            fixes: commits.filter(c => c.commit.message.toLowerCase().startsWith('fix')),
            chores: commits.filter(c => c.commit.message.toLowerCase().startsWith('chore')),
            docs: commits.filter(c => c.commit.message.toLowerCase().startsWith('docs'))
        };
        
        const totalCommits = commits.length;
        const summaryParts = [];
        
        if (categories.features.length > 0) {
            summaryParts.push(`<span class="text-green-600 dark:text-green-400">✨ ${categories.features.length} new feature${categories.features.length === 1 ? '' : 's'}</span>`);
        }
        
        if (categories.fixes.length > 0) {
            summaryParts.push(`<span class="text-red-600 dark:text-red-400">🐛 ${categories.fixes.length} bug fix${categories.fixes.length === 1 ? '' : 'es'}</span>`);
        }
        
        if (categories.chores.length > 0) {
            summaryParts.push(`<span class="text-gray-600 dark:text-gray-400">🔧 ${categories.chores.length} maintenance</span>`);
        }
        
        if (categories.docs.length > 0) {
            summaryParts.push(`<span class="text-purple-600 dark:text-purple-400">📚 ${categories.docs.length} documentation</span>`);
        }
        
        let summaryHtml = '';
        
        if (isRollback) {
            summaryHtml = `<span class="text-amber-600 dark:text-amber-400">⚠️ Switching to stable will remove ${totalCommits} changes:</span>`;
        } else {
            summaryHtml = `<span class="text-green-600 dark:text-green-400">✅ This update includes ${totalCommits} changes:</span>`;
        }
        
        if (summaryParts.length > 0) {
            summaryHtml += ` ${summaryParts.join(', ')}`;
        }
        
        changesSummaryText.innerHTML = summaryHtml;
    }
    
    // Display commit changes in a nice format
    function displayCommitChanges(commits, isRollback = false) {
        const changesList = document.getElementById('changes-list');
        if (!changesList) return;
        
        // Limit to most recent 15 commits to avoid overwhelming the UI
        const limitedCommits = commits.slice(0, 15);
        const hasMore = commits.length > 15;
        
        const commitsHtml = limitedCommits.map(commit => {
            const shortSha = commit.sha.substring(0, 7);
            const commitUrl = commit.html_url;
            const message = commit.commit.message.split('\n')[0]; // First line only
            const author = commit.commit.author.name;
            const date = new Date(commit.commit.author.date).toLocaleDateString();
            
            // Simple commit type detection
            let icon = '📝';
            let iconClass = 'text-blue-600 dark:text-blue-400';
            
            if (message.toLowerCase().startsWith('feat')) {
                icon = '✨';
                iconClass = 'text-green-600 dark:text-green-400';
            } else if (message.toLowerCase().startsWith('fix')) {
                icon = '🐛';
                iconClass = 'text-red-600 dark:text-red-400';
            } else if (message.toLowerCase().startsWith('chore')) {
                icon = '🔧';
                iconClass = 'text-gray-600 dark:text-gray-400';
            } else if (message.toLowerCase().startsWith('docs')) {
                icon = '📚';
                iconClass = 'text-purple-600 dark:text-purple-400';
            }
            
            return `
                <div class="flex items-start gap-3 p-2 rounded border border-gray-200 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700">
                    <span class="${iconClass} text-lg">${icon}</span>
                    <div class="flex-1 min-w-0">
                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">${message}</p>
                        <div class="flex items-center gap-2 mt-1 text-xs text-gray-500 dark:text-gray-400">
                            <a href="${commitUrl}" target="_blank" class="font-mono hover:text-blue-600 dark:hover:text-blue-400 hover:underline">${shortSha}</a>
                            <span>•</span>
                            <span>${author}</span>
                            <span>•</span>
                            <span>${date}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
        
        let summaryText = '';
        if (isRollback) {
            summaryText = `<p class="text-sm text-amber-600 dark:text-amber-400 mb-3">⚠️ Switching to stable will remove these ${limitedCommits.length} changes${hasMore ? ` (showing ${limitedCommits.length} of ${commits.length})` : ''}:</p>`;
        } else {
            summaryText = `<p class="text-sm text-green-600 dark:text-green-400 mb-3">✅ This update includes ${limitedCommits.length} new changes${hasMore ? ` (showing ${limitedCommits.length} of ${commits.length})` : ''}:</p>`;
        }
        
        changesList.innerHTML = summaryText + commitsHtml;
        
        if (hasMore) {
            changesList.innerHTML += `
                <div class="text-center mt-3">
                    <a href="https://github.com/rcourtman/Pulse/compare/v${currentConfig.version}...v${latestReleaseData?.tag_name || 'latest'}" 
                       target="_blank" 
                       class="text-sm text-blue-600 dark:text-blue-400 hover:underline">
                        View all ${commits.length} changes on GitHub →
                    </a>
                </div>
            `;
        }
    }
    
    // Hide update details card
    function hideUpdateDetails() {
        const updateDetails = document.getElementById('update-details');
        if (updateDetails) {
            updateDetails.classList.add('hidden');
        }
        
        // Reset expanded details state
        const detailsExpanded = document.getElementById('update-details-expanded');
        const chevron = document.getElementById('details-chevron');
        const toggleBtn = document.getElementById('toggle-details-btn');
        
        if (detailsExpanded) {
            detailsExpanded.classList.add('hidden');
        }
        if (chevron) {
            chevron.style.transform = 'rotate(0deg)';
        }
        if (toggleBtn) {
            const span = toggleBtn.querySelector('span');
            if (span) span.textContent = 'Show details';
        }
        
        // Clear the stored release data
        latestReleaseData = null;
    }

    // Update management functions
    async function checkForUpdates() {
        await checkLatestVersion();
        showMessage('Update check completed', 'info');
    }

    async function applyUpdate() {
        console.log('[Settings] applyUpdate called, latestReleaseData:', latestReleaseData);
        
        if (!latestReleaseData || !latestReleaseData.assets || latestReleaseData.assets.length === 0) {
            console.error('[Settings] No release data or assets:', { latestReleaseData, hasAssets: !!latestReleaseData?.assets, assetCount: latestReleaseData?.assets?.length });
            showMessage('No update information available. Please check for updates first.', 'error');
            return;
        }
        
        // Find the tarball asset
        const tarballAsset = latestReleaseData.assets.find(asset => 
            asset.name.endsWith('.tar.gz') && asset.name.includes('pulse')
        );
        
        console.log('[Settings] Looking for tarball asset in:', latestReleaseData.assets.map(a => ({ name: a.name, downloadUrl: a.downloadUrl, browser_download_url: a.browser_download_url })));
        console.log('[Settings] Found tarball asset:', tarballAsset);
        
        if (!tarballAsset) {
            console.error('Available assets:', latestReleaseData.assets.map(a => a.name));
            showMessage('No update package found in the release. Expected a .tar.gz file containing "pulse".', 'error');
            return;
        }
        
        if (!tarballAsset.downloadUrl && !tarballAsset.browser_download_url) {
            console.error('Tarball asset missing download URL:', tarballAsset);
            showMessage('Update package is missing download URL', 'error');
            return;
        }
        
        // Check if running in Docker
        if (latestReleaseData.isDocker) {
            showMessage('Automatic updates are not supported in Docker. Please pull the latest Docker image.', 'warning');
            return;
        }
        
        // Confirm update
        if (!confirm(`Update to version ${latestReleaseData.tag_name}?\n\nThe application will restart automatically after the update is applied.`)) {
            return;
        }
        
        try {
            // Hide update details and show progress
            const updateDetails = document.getElementById('update-available');
            const updateProgress = document.getElementById('update-progress');
            const progressBar = document.getElementById('update-progress-bar');
            const progressText = document.getElementById('update-progress-text');
            
            if (updateDetails) updateDetails.classList.add('hidden');
            if (updateProgress) updateProgress.classList.remove('hidden');
            
            // Start the update
            const updateResponse = await fetch('/api/updates/apply', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    downloadUrl: tarballAsset.downloadUrl || tarballAsset.browser_download_url
                })
            });
            
            if (!updateResponse.ok) {
                const error = await updateResponse.json();
                throw new Error(error.error || 'Failed to start update');
            }
            
            // Listen for progress updates via WebSocket
            if (window.socket) {
                window.socket.on('updateProgress', (data) => {
                    if (progressBar) {
                        progressBar.style.width = `${data.progress}%`;
                    }
                    if (progressText) {
                        const phaseText = {
                            'download': 'Downloading update...',
                            'backup': 'Backing up current installation...',
                            'extract': 'Extracting update files...',
                            'apply': 'Applying update...',
                            'restarting': 'Update complete! Restarting service...'
                        };
                        progressText.textContent = phaseText[data.phase] || 'Processing...';
                    }
                });
                
                window.socket.on('updateComplete', () => {
                    if (progressText) {
                        progressText.textContent = 'Update complete! Restarting...';
                    }
                    showMessage('Update applied successfully. The application will restart momentarily.', 'success');
                });
                
                window.socket.on('updateError', (data) => {
                    showMessage(`Update failed: ${data.error}`, 'error');
                    if (updateDetails) updateDetails.classList.remove('hidden');
                    if (updateProgress) updateProgress.classList.add('hidden');
                });
            }
            
            showMessage('Update started. Please wait...', 'info');
            
        } catch (error) {
            console.error('Error applying update:', error);
            showMessage(`Failed to apply update: ${error.message}`, 'error');
            
            // Restore UI state
            const updateDetails = document.getElementById('update-available');
            const updateProgress = document.getElementById('update-progress');
            if (updateDetails) updateDetails.classList.remove('hidden');
            if (updateProgress) updateProgress.classList.add('hidden');
        }
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
                
                // If we can't find the guest or it doesn't have a node, use a placeholder
                // The server will handle finding the actual node
                if (selectedGuest && selectedGuest.node) {
                    nodeField.value = selectedGuest.node;
                } else {
                    // Use a placeholder value that will pass validation
                    // The server can determine the actual node from endpointId and vmid
                    nodeField.value = 'auto-detect';
                }
                
                // Debug logging
                console.log('[Settings] Guest selector changed:', {
                    selectedEndpoint,
                    selectedVmid,
                    selectedGuest,
                    nodeValue: nodeField.value,
                    allGuestsCount: allGuests.length
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

    // Email test functionality
    // Email provider configurations
    const emailProviders = {
        gmail: {
            name: 'Gmail',
            host: 'smtp.gmail.com',
            port: 587,
            secure: false,  // Use STARTTLS with port 587 (not SSL with port 465)
            requireTLS: true,
            domains: ['gmail.com', 'googlemail.com']
        },
        outlook: {
            name: 'Outlook',
            host: 'smtp-mail.outlook.com',
            port: 587,
            secure: false,  // Use STARTTLS
            requireTLS: true,
            domains: ['outlook.com', 'hotmail.com', 'live.com', 'msn.com']
        },
        yahoo: {
            name: 'Yahoo',
            host: 'smtp.mail.yahoo.com',
            port: 587,
            secure: false,  // Use STARTTLS
            requireTLS: true,
            domains: ['yahoo.com', 'yahoo.co.uk', 'yahoo.ca', 'ymail.com']
        },
        custom: {
            name: 'Custom',
            host: '',
            port: 587,
            secure: false,
            requireTLS: false,
            domains: []
        }
    };

    function setupEmailProviderSelection() {
        const providerButtons = document.querySelectorAll('.email-provider-btn');
        const emailFromInput = document.getElementById('email-from-input');
        const passwordLabel = document.getElementById('password-label');
        const passwordHelp = document.getElementById('password-help');
        const providerHelp = document.getElementById('provider-help');
        const advancedToggle = document.getElementById('toggle-advanced-email');
        const advancedSettings = document.getElementById('advanced-email-settings');
        const advancedIcon = document.getElementById('advanced-email-icon');
        
        // Set up provider button clicks
        providerButtons.forEach(btn => {
            btn.addEventListener('click', () => {
                selectProvider(btn.getAttribute('data-provider'));
            });
        });

        // Set up email input change detection
        if (emailFromInput) {
            emailFromInput.addEventListener('blur', () => {
                autoDetectProvider(emailFromInput.value);
            });
        }

        // Set up advanced settings toggle
        if (advancedToggle) {
            advancedToggle.addEventListener('click', () => {
                const isHidden = advancedSettings.classList.contains('hidden');
                if (isHidden) {
                    advancedSettings.classList.remove('hidden');
                    advancedIcon.style.transform = 'rotate(90deg)';
                } else {
                    advancedSettings.classList.add('hidden');
                    advancedIcon.style.transform = 'rotate(0deg)';
                }
            });
        }

        function selectProvider(providerKey) {
            const provider = emailProviders[providerKey];
            if (!provider) return;

            // Update button styles
            providerButtons.forEach(btn => {
                btn.classList.remove('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
                btn.classList.add('border-gray-200', 'dark:border-gray-600');
            });
            
            const selectedBtn = document.querySelector(`[data-provider="${providerKey}"]`);
            if (selectedBtn) {
                selectedBtn.classList.remove('border-gray-200', 'dark:border-gray-600');
                selectedBtn.classList.add('border-blue-500', 'bg-blue-50', 'dark:bg-blue-900/20');
            }

            // Update form fields
            const form = document.getElementById('settings-form');
            if (form) {
                const hostInput = form.querySelector('[name="SMTP_HOST"]');
                const portInput = form.querySelector('[name="SMTP_PORT"]');
                const userInput = form.querySelector('[name="SMTP_USER"]');
                const secureInput = form.querySelector('[name="SMTP_SECURE"]');

                if (hostInput) hostInput.value = provider.host;
                if (portInput) portInput.value = provider.port;
                if (secureInput) secureInput.checked = provider.secure;
                
                // Set username to match from email if available
                const fromEmail = form.querySelector('[name="ALERT_FROM_EMAIL"]').value;
                if (userInput && fromEmail && providerKey !== 'custom') {
                    userInput.value = fromEmail;
                }
            }

            // Update password label and help text
            updatePasswordLabels(providerKey);

            // Show/hide provider-specific help
            showProviderHelp(providerKey);

            // Show advanced settings for custom provider
            if (providerKey === 'custom' && advancedSettings.classList.contains('hidden')) {
                advancedToggle.click();
            }
        }

        function autoDetectProvider(email) {
            if (!email || !email.includes('@')) return;
            
            const domain = email.split('@')[1].toLowerCase();
            
            for (const [key, provider] of Object.entries(emailProviders)) {
                if (provider.domains.includes(domain)) {
                    selectProvider(key);
                    return;
                }
            }
            
            // If no provider detected, select custom
            selectProvider('custom');
        }

        function updatePasswordLabels(providerKey) {
            if (!passwordLabel || !passwordHelp) return;

            switch (providerKey) {
                case 'gmail':
                    passwordLabel.textContent = 'App Password';
                    passwordHelp.textContent = 'You need a 16-character App Password from Google (not your regular password)';
                    break;
                case 'yahoo':
                    passwordLabel.textContent = 'App Password';
                    passwordHelp.textContent = 'You need an App Password from Yahoo (not your regular password)';
                    break;
                case 'outlook':
                    passwordLabel.textContent = 'Password';
                    passwordHelp.textContent = 'Use your regular email password (or app password if 2FA is enabled)';
                    break;
                default:
                    passwordLabel.textContent = 'Password';
                    passwordHelp.textContent = 'Enter your email password or app password';
            }
        }

        function showProviderHelp(providerKey) {
            if (!providerHelp) return;

            // Hide all help sections
            const helpSections = providerHelp.querySelectorAll('[id$="-help"]');
            helpSections.forEach(section => section.classList.add('hidden'));

            // Show the relevant help section
            const helpSection = document.getElementById(`${providerKey}-help`);
            if (helpSection) {
                providerHelp.classList.remove('hidden');
                helpSection.classList.remove('hidden');
            } else {
                providerHelp.classList.add('hidden');
            }
        }

        // Auto-detect provider on page load if email is already filled
        if (emailFromInput && emailFromInput.value) {
            autoDetectProvider(emailFromInput.value);
        }
    }

    function setupEmailTestButton() {
        const testBtn = document.getElementById('test-email-btn');
        if (testBtn) {
            testBtn.addEventListener('click', async () => {
                const originalText = testBtn.textContent;
                testBtn.textContent = 'Sending...';
                testBtn.disabled = true;
                
                try {
                    // Get email settings from form
                    const form = document.getElementById('settings-form');
                    const formData = new FormData(form);
                    
                    const emailConfig = {
                        host: formData.get('SMTP_HOST'),
                        port: formData.get('SMTP_PORT'),
                        user: formData.get('SMTP_USER'),
                        pass: formData.get('SMTP_PASS'),
                        from: formData.get('ALERT_FROM_EMAIL'),
                        to: formData.get('ALERT_TO_EMAIL'),
                        secure: formData.get('SMTP_SECURE') === 'on'
                    };
                    
                    const response = await fetch('/api/test-email', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify(emailConfig)
                    });
                    
                    const result = await response.json();
                    
                    if (result.success) {
                        testBtn.textContent = '✓ Sent!';
                        testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    } else {
                        testBtn.textContent = '✗ Failed';
                        testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                        alert('Test email failed: ' + result.error);
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    }
                } catch (error) {
                    testBtn.textContent = '✗ Error';
                    testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                    alert('Error sending test email: ' + error.message);
                    setTimeout(() => {
                        testBtn.textContent = originalText;
                        testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                    }, 3000);
                } finally {
                    testBtn.disabled = false;
                }
            });
        }
    }

    // Webhook test functionality
    function setupWebhookTestButton() {
        const testBtn = document.getElementById('test-webhook-btn');
        if (testBtn) {
            testBtn.addEventListener('click', async () => {
                const originalText = testBtn.textContent;
                testBtn.textContent = 'Sending...';
                testBtn.disabled = true;
                
                try {
                    // Get webhook settings from form
                    const form = document.getElementById('settings-form');
                    const formData = new FormData(form);
                    
                    const webhookConfig = {
                        url: formData.get('WEBHOOK_URL'),
                        enabled: formData.get('WEBHOOK_ENABLED') === 'on'
                    };
                    
                    const response = await fetch('/api/test-webhook', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify(webhookConfig)
                    });
                    
                    const result = await response.json();
                    
                    if (result.success) {
                        testBtn.textContent = '✓ Sent!';
                        testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-purple-600 hover:bg-purple-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    } else {
                        testBtn.textContent = '✗ Failed';
                        testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                        alert('Test webhook failed: ' + result.error);
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-purple-600 hover:bg-purple-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    }
                } catch (error) {
                    testBtn.textContent = '✗ Error';
                    testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                    alert('Error sending test webhook: ' + error.message);
                    setTimeout(() => {
                        testBtn.textContent = originalText;
                        testBtn.className = 'px-3 py-1 bg-purple-600 hover:bg-purple-700 text-white text-xs font-medium rounded transition-colors';
                    }, 3000);
                } finally {
                    testBtn.disabled = false;
                }
            });
        }
    }

    // Diagnostics functions
    let diagnosticData = null;
    
    async function runDiagnostics() {
        const statusEl = document.getElementById('diagnostics-status');
        const resultsEl = document.getElementById('diagnostics-results');
        const runButton = document.getElementById('runDiagnostics');
        const copyButton = document.getElementById('copyReport');
        const downloadButton = document.getElementById('downloadReport');
        
        // Reset UI
        statusEl.className = 'block mb-4 p-3 rounded-lg text-sm font-medium';
        statusEl.classList.add('bg-blue-50', 'dark:bg-blue-900/20', 'text-blue-700', 'dark:text-blue-300');
        statusEl.textContent = 'Running diagnostics...';
        statusEl.style.display = 'block';
        resultsEl.style.display = 'none';
        runButton.disabled = true;
        
        try {
            const response = await fetch('/api/diagnostics');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            diagnosticData = await response.json();
            
            // Show success status
            statusEl.className = 'block mb-4 p-3 rounded-lg text-sm font-medium';
            if (diagnosticData.summary?.hasIssues) {
                statusEl.classList.add('bg-yellow-50', 'dark:bg-yellow-900/20', 'text-yellow-700', 'dark:text-yellow-300');
                statusEl.textContent = `Diagnostics complete. Found ${diagnosticData.summary.criticalIssues} critical issues and ${diagnosticData.summary.warnings} warnings.`;
            } else {
                statusEl.classList.add('bg-green-50', 'dark:bg-green-900/20', 'text-green-700', 'dark:text-green-300');
                statusEl.textContent = 'Diagnostics complete. No critical issues found!';
            }
            
            // Display results
            displayDiagnosticResults(diagnosticData);
            resultsEl.style.display = 'block';
            
            // Show action buttons
            copyButton.style.display = 'inline-flex';
            downloadButton.style.display = 'inline-flex';
            
        } catch (error) {
            statusEl.className = 'block mb-4 p-3 rounded-lg text-sm font-medium';
            statusEl.classList.add('bg-red-50', 'dark:bg-red-900/20', 'text-red-700', 'dark:text-red-300');
            statusEl.textContent = `Error running diagnostics: ${error.message}`;
        } finally {
            runButton.disabled = false;
        }
    }
    
    function displayDiagnosticResults(data) {
        const resultsEl = document.getElementById('diagnostics-results');
        
        let html = '';
        
        // Recommendations section
        if (data.recommendations && data.recommendations.length > 0) {
            html += createDiagnosticSection('Recommendations', renderRecommendations(data.recommendations), true);
        }
        
        // Configuration section
        if (data.configuration) {
            html += createDiagnosticSection('Configuration', renderConfiguration(data.configuration));
        }
        
        // Permissions section
        if (data.permissions) {
            html += createDiagnosticSection('API Token Permissions', renderPermissions(data.permissions));
        }
        
        // System Information section
        if (data.state || data.version) {
            html += createDiagnosticSection('System Information', renderSystemInfo(data));
        }
        
        resultsEl.innerHTML = html;
        
        // Add click handlers for collapsible sections
        resultsEl.querySelectorAll('.diagnostic-section-header').forEach(header => {
            header.addEventListener('click', () => {
                const section = header.parentElement;
                const content = section.querySelector('.diagnostic-section-content');
                const indicator = header.querySelector('.diagnostic-indicator');
                
                if (content.style.display === 'none') {
                    content.style.display = 'block';
                    indicator.textContent = '▼';
                } else {
                    content.style.display = 'none';
                    indicator.textContent = '▶';
                }
            });
        });
    }
    
    function createDiagnosticSection(title, content, expanded = false) {
        return `
            <div class="diagnostic-section bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <div class="diagnostic-section-header px-4 py-3 bg-gray-50 dark:bg-gray-700 cursor-pointer flex justify-between items-center hover:bg-gray-100 dark:hover:bg-gray-600">
                    <span class="font-medium text-gray-900 dark:text-gray-100">${title}</span>
                    <span class="diagnostic-indicator text-gray-500 dark:text-gray-400">${expanded ? '▼' : '▶'}</span>
                </div>
                <div class="diagnostic-section-content px-4 py-3" style="display: ${expanded ? 'block' : 'none'};">
                    ${content}
                </div>
            </div>
        `;
    }
    
    function renderRecommendations(recommendations) {
        if (recommendations.length === 0) {
            return '<p class="text-green-600 dark:text-green-400">✓ No issues found - everything looks good!</p>';
        }
        
        return recommendations.map(rec => {
            let bgColor, textColor, borderColor;
            switch(rec.severity) {
                case 'critical':
                    bgColor = 'bg-red-50 dark:bg-red-900/20';
                    textColor = 'text-red-800 dark:text-red-200';
                    borderColor = 'border-red-500';
                    break;
                case 'warning':
                    bgColor = 'bg-yellow-50 dark:bg-yellow-900/20';
                    textColor = 'text-yellow-800 dark:text-yellow-200';
                    borderColor = 'border-yellow-500';
                    break;
                default:
                    bgColor = 'bg-blue-50 dark:bg-blue-900/20';
                    textColor = 'text-blue-800 dark:text-blue-200';
                    borderColor = 'border-blue-500';
            }
            
            return `
                <div class="${bgColor} ${textColor} p-3 rounded-lg border-l-4 ${borderColor} mb-3">
                    <strong>[${rec.severity.toUpperCase()}] ${rec.category}:</strong> ${rec.message}
                </div>
            `;
        }).join('');
    }
    
    function renderConfiguration(config) {
        let html = '<div class="space-y-4">';
        
        html += '<h4 class="font-medium text-gray-900 dark:text-gray-100">Proxmox VE Instances</h4>';
        if (!config.proxmox || config.proxmox.length === 0) {
            html += '<p class="text-gray-600 dark:text-gray-400">No Proxmox instances configured</p>';
        } else {
            html += '<div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">';
            html += '<thead class="bg-gray-50 dark:bg-gray-700"><tr>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Host</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Token</th>';
            html += '</tr></thead><tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">';
            
            config.proxmox.forEach(pve => {
                html += `<tr>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pve.host}</td>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pve.name}</td>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pve.tokenConfigured ? '✓' : '✗'}</td>
                </tr>`;
            });
            html += '</tbody></table></div>';
        }
        
        html += '<h4 class="font-medium text-gray-900 dark:text-gray-100 mt-4">PBS Instances</h4>';
        if (!config.pbs || config.pbs.length === 0) {
            html += '<p class="text-gray-600 dark:text-gray-400">No PBS instances configured</p>';
        } else {
            html += '<div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">';
            html += '<thead class="bg-gray-50 dark:bg-gray-700"><tr>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Host</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Node Name</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Token</th>';
            html += '</tr></thead><tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">';
            
            config.pbs.forEach(pbs => {
                const nodeNameStyle = pbs.node_name === 'NOT SET' ? 'text-red-600 dark:text-red-400 font-bold' : '';
                html += `<tr>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pbs.host}</td>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pbs.name}</td>
                    <td class="px-4 py-2 text-sm ${nodeNameStyle}">${pbs.node_name}</td>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${pbs.tokenConfigured ? '✓' : '✗'}</td>
                </tr>`;
            });
            html += '</tbody></table></div>';
        }
        
        html += '</div>';
        return html;
    }
    
    function renderPermissions(permissions) {
        let html = '<div class="space-y-4">';
        
        if (permissions.proxmox && permissions.proxmox.length > 0) {
            html += '<h4 class="font-medium text-gray-900 dark:text-gray-100">Proxmox VE Token Permissions</h4>';
            html += '<div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">';
            html += '<thead class="bg-gray-50 dark:bg-gray-700"><tr>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Instance</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Connect</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Nodes</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">VMs</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Containers</th>';
            html += '</tr></thead><tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">';
            
            permissions.proxmox.forEach(perm => {
                const checkIcon = (canDo) => canDo ? 
                    '<span class="text-green-600 dark:text-green-400">✓</span>' : 
                    '<span class="text-red-600 dark:text-red-400">✗</span>';
                html += `<tr>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${perm.name}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canConnect)}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canListNodes)} ${perm.nodeCount ? `(${perm.nodeCount})` : ''}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canListVMs)} ${perm.vmCount !== undefined ? `(${perm.vmCount})` : ''}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canListContainers)} ${perm.containerCount !== undefined ? `(${perm.containerCount})` : ''}</td>
                </tr>`;
            });
            html += '</tbody></table></div>';
        }
        
        if (permissions.pbs && permissions.pbs.length > 0) {
            html += '<h4 class="font-medium text-gray-900 dark:text-gray-100 mt-4">PBS Token Permissions</h4>';
            html += '<div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">';
            html += '<thead class="bg-gray-50 dark:bg-gray-700"><tr>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Instance</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Connect</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Datastores</th>';
            html += '<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Backups</th>';
            html += '</tr></thead><tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">';
            
            permissions.pbs.forEach(perm => {
                const checkIcon = (canDo) => canDo ? 
                    '<span class="text-green-600 dark:text-green-400">✓</span>' : 
                    '<span class="text-red-600 dark:text-red-400">✗</span>';
                html += `<tr>
                    <td class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100">${perm.name}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canConnect)}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canListDatastores)} ${perm.datastoreCount !== undefined ? `(${perm.datastoreCount})` : ''}</td>
                    <td class="px-4 py-2 text-sm">${checkIcon(perm.canListBackups)} ${perm.backupCount !== undefined ? `(${perm.backupCount})` : ''}</td>
                </tr>`;
            });
            html += '</tbody></table></div>';
        }
        
        html += '</div>';
        return html;
    }
    
    function renderSystemInfo(data) {
        let html = '<div class="space-y-2">';
        
        if (data.version) {
            html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Pulse Version:</span> <span class="text-gray-900 dark:text-gray-100">${data.version}</span></p>`;
        }
        
        if (data.state) {
            if (data.state.lastUpdate) {
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Last Update:</span> <span class="text-gray-900 dark:text-gray-100">${new Date(data.state.lastUpdate).toLocaleString()}</span></p>`;
            }
            
            if (data.state.serverUptime) {
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Server Uptime:</span> <span class="text-gray-900 dark:text-gray-100">${Math.floor(data.state.serverUptime)} seconds</span></p>`;
            }
            
            if (data.state.nodes) {
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Nodes:</span> <span class="text-gray-900 dark:text-gray-100">${data.state.nodes.count} (${data.state.nodes.names.join(', ') || 'none'})</span></p>`;
            }
            
            if (data.state.guests) {
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Total Guests:</span> <span class="text-gray-900 dark:text-gray-100">${data.state.guests.total} (${data.state.guests.vms} VMs, ${data.state.guests.containers} Containers)</span></p>`;
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Guest Status:</span> <span class="text-gray-900 dark:text-gray-100">${data.state.guests.running} running, ${data.state.guests.stopped} stopped</span></p>`;
            }
            
            if (data.state.pbs) {
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">PBS Instances:</span> <span class="text-gray-900 dark:text-gray-100">${data.state.pbs.instances}</span></p>`;
                html += `<p class="text-sm"><span class="font-medium text-gray-700 dark:text-gray-300">Total Backups:</span> <span class="text-gray-900 dark:text-gray-100">${data.state.pbs.totalBackups}</span></p>`;
            }
        }
        
        html += '</div>';
        return html;
    }
    
    function sanitizeReport(report) {
        // Deep clone the report to avoid modifying the original
        const sanitized = JSON.parse(JSON.stringify(report));
        
        // Sanitize configuration section
        if (sanitized.configuration) {
            if (sanitized.configuration.proxmox) {
                sanitized.configuration.proxmox = sanitized.configuration.proxmox.map(pve => ({
                    ...pve,
                    host: sanitizeUrl(pve.host),
                    tokenConfigured: pve.tokenConfigured,
                    selfSignedCerts: pve.selfSignedCerts
                }));
            }
            
            if (sanitized.configuration.pbs) {
                sanitized.configuration.pbs = sanitized.configuration.pbs.map(pbs => ({
                    ...pbs,
                    host: sanitizeUrl(pbs.host),
                    tokenConfigured: pbs.tokenConfigured,
                    selfSignedCerts: pbs.selfSignedCerts,
                    node_name: pbs.node_name
                }));
            }
        }
        
        // Sanitize permissions section
        if (sanitized.permissions) {
            if (sanitized.permissions.proxmox) {
                sanitized.permissions.proxmox = sanitized.permissions.proxmox.map(perm => ({
                    ...perm,
                    host: sanitizeUrl(perm.host),
                    name: sanitizeUrl(perm.name),
                    errors: perm.errors ? perm.errors.map(err => sanitizeErrorMessage(err)) : []
                }));
            }
            
            if (sanitized.permissions.pbs) {
                sanitized.permissions.pbs = sanitized.permissions.pbs.map(perm => ({
                    ...perm,
                    host: sanitizeUrl(perm.host),
                    name: sanitizeUrl(perm.name),
                    errors: perm.errors ? perm.errors.map(err => sanitizeErrorMessage(err)) : []
                }));
            }
        }
        
        // Sanitize state section
        if (sanitized.state) {
            if (sanitized.state.nodes && sanitized.state.nodes.names) {
                sanitized.state.nodes.names = sanitized.state.nodes.names.map((name, index) => `node-${index + 1}`);
            }
            
            if (sanitized.state.pbs && sanitized.state.pbs.sampleBackupIds) {
                sanitized.state.pbs.sampleBackupIds = sanitized.state.pbs.sampleBackupIds.map((id, index) => `backup-${index + 1}`);
            }
        }
        
        // Sanitize recommendations
        if (sanitized.recommendations) {
            sanitized.recommendations = sanitized.recommendations.map(rec => ({
                ...rec,
                message: sanitizeRecommendationMessage(rec.message)
            }));
        }
        
        // Add notice about sanitization
        sanitized._sanitized = {
            notice: "This diagnostic report has been sanitized for safe sharing. Hostnames, IPs, node names, and backup IDs have been anonymized while preserving structural information needed for troubleshooting.",
            timestamp: new Date().toISOString()
        };
        
        return sanitized;
    }
    
    function sanitizeUrl(url) {
        if (!url) return url;
        
        // Remove protocol if present
        let sanitized = url.replace(/^https?:\/\//, '');
        
        // Replace IP addresses
        sanitized = sanitized.replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, '[IP-ADDRESS]');
        
        // Replace hostnames (anything before port or path)
        sanitized = sanitized.replace(/^[^:/]+/, '[HOSTNAME]');
        
        // Replace ports
        sanitized = sanitized.replace(/:\d+/, ':[PORT]');
        
        return sanitized;
    }
    
    function sanitizeErrorMessage(errorMsg) {
        if (!errorMsg) return errorMsg;
        
        // Remove potential IP addresses, hostnames, and ports
        let sanitized = errorMsg
            .replace(/\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/g, '[IP-ADDRESS]')
            .replace(/https?:\/\/[^\/\s:]+(?::\d+)?/g, '[HOSTNAME]')
            .replace(/([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}/g, '[HOSTNAME]')
            .replace(/:\d{4,5}\b/g, ':[PORT]');
            
        return sanitized;
    }
    
    function sanitizeRecommendationMessage(message) {
        if (!message) return message;
        
        // Replace specific hostnames and IPs in common recommendation patterns
        let sanitized = message
            .replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, '[IP-ADDRESS]')
            .replace(/https?:\/\/[^\/\s:]+/g, '[HOSTNAME]')
            .replace(/host\s*'[^']+'/g, "host '[HOSTNAME]'")
            .replace(/host\s*"[^"]+"/g, 'host "[HOSTNAME]"')
            .replace(/node\s+'[^']+'/g, "node '[NODE-NAME]'")
            .replace(/node\s+"[^"]+"/g, 'node "[NODE-NAME]"')
            .replace(/:\d{4,5}\b/g, ':[PORT]');
            
        return sanitized;
    }
    
    function copyDiagnosticReport() {
        if (!diagnosticData) return;
        
        // Sanitize the data before copying
        const sanitizedData = sanitizeReport(diagnosticData);
        const text = JSON.stringify(sanitizedData, null, 2);
        
        navigator.clipboard.writeText(text).then(() => {
            const button = document.getElementById('copyReport');
            const originalText = button.innerHTML;
            button.innerHTML = '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg> Copied!';
            button.classList.remove('bg-gray-600', 'hover:bg-gray-700');
            button.classList.add('bg-green-600', 'hover:bg-green-700');
            
            setTimeout(() => {
                button.innerHTML = originalText;
                button.classList.remove('bg-green-600', 'hover:bg-green-700');
                button.classList.add('bg-gray-600', 'hover:bg-gray-700');
            }, 3000);
        });
    }
    
    function downloadDiagnosticReport() {
        if (!diagnosticData) return;
        
        // Sanitize the data before downloading
        const sanitizedData = sanitizeReport(diagnosticData);
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
        const filename = `pulse_diagnostics_${timestamp}.json`;
        const text = JSON.stringify(sanitizedData, null, 2);
        
        const blob = new Blob([text], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    // Public API
    // Handle update channel selection change
    function onUpdateChannelChange(value) {
        const rcWarning = document.getElementById('rc-warning');
        if (rcWarning) {
            if (value === 'rc') {
                rcWarning.classList.remove('hidden');
            } else {
                rcWarning.classList.add('hidden');
            }
        }
        
        // Re-check for updates with the selected channel (preview mode)
        // Debounce rapid changes to prevent API spam
        if (updateCheckTimeout) {
            clearTimeout(updateCheckTimeout);
        }
        updateCheckTimeout = setTimeout(() => {
            checkLatestVersion(value);
        }, 300);
    }
    
    // Switch to a specific update channel
    function switchToChannel(targetChannel) {
        const channelSelect = document.querySelector('select[name="UPDATE_CHANNEL"]');
        if (channelSelect) {
            channelSelect.value = targetChannel;
            onUpdateChannelChange(targetChannel);
            
            // Hide the warning
            const warningElement = document.getElementById('channel-mismatch-warning');
            if (warningElement) {
                warningElement.classList.add('hidden');
            }
            
            // Show success message
            const button = document.getElementById('switch-to-rc-btn');
            if (button) {
                const originalText = button.textContent;
                button.textContent = 'Switched! ✓';
                button.classList.add('text-green-600', 'dark:text-green-400');
                button.classList.remove('text-blue-600', 'dark:text-blue-400');
                
                setTimeout(() => {
                    if (warningElement) {
                        warningElement.classList.add('hidden');
                    }
                }, 2000);
            }
        }
    }
    
    // Proceed with switching to stable release (downgrade)
    function proceedWithStableSwitch() {
        const warningElement = document.getElementById('channel-mismatch-warning');
        if (warningElement) {
            warningElement.classList.add('hidden');
        }
        
        // Show confirmation that they're choosing stable
        const versionStatusElement = document.getElementById('version-status');
        if (versionStatusElement) {
            versionStatusElement.innerHTML = '<span class="text-blue-600 dark:text-blue-400">✓ Stable channel selected - save settings to apply</span>';
        }
    }
    
    // Acknowledge the user wants to stay on stable channel (legacy)
    function acknowledgeStableChoice() {
        proceedWithStableSwitch();
    }
    
    // Legacy function for backward compatibility
    function switchToRecommendedChannel() {
        const currentVersion = currentConfig.version || 'Unknown';
        const currentVersionLower = currentVersion.toLowerCase();
        const isCurrentRC = currentVersionLower.includes('-rc') || currentVersionLower.includes('-alpha') || currentVersionLower.includes('-beta');
        const recommendedChannel = isCurrentRC ? 'rc' : 'stable';
        switchToChannel(recommendedChannel);
    }

    // Clear update cache (useful after saving settings)
    function clearUpdateCache() {
        updateCache.clear();
        console.log('[Settings] Update cache cleared');
    }

    async function initializeAlertManagementTab() {
        console.log('[Settings] Initializing Alert Management tab');
        
        // Set up event listeners for the alert management tab
        setTimeout(() => {
            setupAlertManagementEvents();
            loadDynamicRules();
            loadCustomThresholds();
        }, 100);
    }

    function setupAlertManagementEvents() {
        // Refresh buttons
        const refreshDynamicBtn = document.getElementById('refresh-dynamic-rules');
        const refreshCustomBtn = document.getElementById('refresh-custom-thresholds');
        
        if (refreshDynamicBtn) {
            refreshDynamicBtn.addEventListener('click', loadDynamicRules);
        }
        
        if (refreshCustomBtn) {
            refreshCustomBtn.addEventListener('click', loadCustomThresholds);
        }
        
        // Quick action buttons
        const createRuleBtn = document.getElementById('create-threshold-rule');
        const testNotificationsBtn = document.getElementById('test-notifications');
        const exportAlertsBtn = document.getElementById('export-alerts');
        const addCustomBtn = document.getElementById('add-custom-threshold');
        
        if (createRuleBtn) {
            createRuleBtn.addEventListener('click', () => {
                // Close settings modal and open threshold creation
                closeModal();
                // Enable thresholds toggle if not already enabled
                const thresholdToggle = document.getElementById('toggle-thresholds-checkbox');
                if (thresholdToggle && !thresholdToggle.checked) {
                    thresholdToggle.click();
                }
                // Show a notification
                showNotification('Set your thresholds above and click "Create Alert Rule" to create a new dynamic threshold rule.');
            });
        }
        
        if (testNotificationsBtn) {
            testNotificationsBtn.addEventListener('click', testAllNotifications);
        }
        
        if (exportAlertsBtn) {
            exportAlertsBtn.addEventListener('click', exportAlertConfiguration);
        }
        
        if (addCustomBtn) {
            addCustomBtn.addEventListener('click', () => {
                // Switch to alerts tab to add custom threshold
                switchTab('alerts');
                setTimeout(() => {
                    // Scroll to the custom threshold section
                    const customSection = document.querySelector('h3:contains("Custom Threshold Configurations")');
                    if (customSection) {
                        customSection.scrollIntoView({ behavior: 'smooth' });
                    }
                }, 100);
            });
        }
    }

    async function loadDynamicRules() {
        const container = document.getElementById('dynamic-rules-container');
        if (!container) return;
        
        try {
            container.innerHTML = `
                <div class="flex items-center justify-center py-8">
                    <div class="text-center">
                        <svg class="mx-auto h-8 w-8 text-blue-500 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading dynamic threshold rules...</p>
                    </div>
                </div>
            `;
            
            const response = await fetch('/api/alerts/compound-rules');
            const result = await response.json();
            
            if (result.success && result.rules) {
                if (result.rules.length === 0) {
                    container.innerHTML = `
                        <div class="text-center py-8">
                            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"></path>
                            </svg>
                            <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No dynamic threshold rules</h3>
                            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Create your first rule using the threshold sliders on the dashboard.</p>
                            <div class="mt-4">
                                <button id="goto-thresholds" class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700">
                                    Go to Thresholds
                                </button>
                            </div>
                        </div>
                    `;
                    
                    // Add event listener for the goto button
                    const gotoBtn = document.getElementById('goto-thresholds');
                    if (gotoBtn) {
                        gotoBtn.addEventListener('click', () => {
                            closeModal();
                            const thresholdToggle = document.getElementById('toggle-thresholds-checkbox');
                            if (thresholdToggle && !thresholdToggle.checked) {
                                thresholdToggle.click();
                            }
                        });
                    }
                } else {
                    // Display the rules
                    container.innerHTML = `
                        <div class="space-y-4">
                            ${result.rules.map(rule => formatDynamicRuleCard(rule)).join('')}
                        </div>
                    `;
                    
                    // Add event listeners for rule actions
                    setupDynamicRuleActions();
                }
            } else {
                throw new Error(result.error || 'Failed to load rules');
            }
        } catch (error) {
            console.error('[Settings] Error loading dynamic rules:', error);
            container.innerHTML = `
                <div class="text-center py-8">
                    <svg class="mx-auto h-12 w-12 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                    </svg>
                    <p class="mt-2 text-sm text-red-600 dark:text-red-400">Failed to load dynamic rules: ${error.message}</p>
                    <button onclick="loadDynamicRules()" class="mt-2 text-sm text-blue-600 hover:text-blue-800">Try again</button>
                </div>
            `;
        }
    }

    function formatDynamicRuleCard(rule) {
        const createdDate = new Date(rule.createdAt || Date.now()).toLocaleDateString();
        const severityColor = rule.severity === 'critical' ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';
        
        const thresholdsList = rule.thresholds?.map(t => 
            `<span class="inline-block bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs rounded mr-2 mb-1">
                ${getThresholdDisplayName(t.type)} ≥ ${formatThresholdValue(t)}
            </span>`
        ).join('') || '';

        const notificationMethods = rule.notificationChannels?.map(channel => {
            switch (channel) {
                case 'local': return 'In-app';
                case 'email': return 'Email';
                case 'discord': return 'Discord';
                case 'slack': return 'Slack';
                case 'webhook': return 'Webhook';
                default: return channel;
            }
        }).join(', ') || 'None';
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-4">
                <div class="flex items-start justify-between mb-3">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-2">
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">${rule.name}</h4>
                            <span class="px-2 py-1 text-xs font-medium rounded ${severityColor}">
                                ${rule.severity?.toUpperCase() || 'WARNING'}
                            </span>
                            <span class="px-2 py-1 text-xs font-medium rounded ${rule.enabled ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}">
                                ${rule.enabled ? 'ENABLED' : 'DISABLED'}
                            </span>
                        </div>
                        ${rule.description ? `<p class="text-sm text-gray-600 dark:text-gray-400 mb-2">${rule.description}</p>` : ''}
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Created: ${createdDate} • ID: ${rule.id}
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Notifications: ${notificationMethods}
                        </div>
                    </div>
                    <div class="flex gap-2">
                        <button onclick="toggleDynamicRule('${rule.id}', ${!rule.enabled})" 
                                class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded">
                            ${rule.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button onclick="deleteDynamicRule('${rule.id}')" 
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

    function getThresholdDisplayName(type) {
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

    function formatThresholdValue(threshold) {
        if (['cpu', 'memory', 'disk'].includes(threshold.type)) {
            return `${threshold.value}%`;
        } else {
            const mb = threshold.value / (1024 * 1024);
            if (mb >= 100) return `${Math.round(mb)}MB/s`;
            if (mb >= 10) return `${Math.round(mb)}MB/s`;
            return `${Math.round(mb * 10) / 10}MB/s`;
        }
    }

    async function loadCustomThresholds() {
        const container = document.getElementById('custom-thresholds-container');
        if (!container) return;
        
        try {
            container.innerHTML = `
                <div class="flex items-center justify-center py-8">
                    <div class="text-center">
                        <svg class="mx-auto h-8 w-8 text-blue-500 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading custom thresholds...</p>
                    </div>
                </div>
            `;
            
            const response = await fetch('/api/thresholds');
            const result = await response.json();
            
            if (result.success && result.data) {
                if (result.data.length === 0) {
                    container.innerHTML = `
                        <div class="text-center py-8">
                            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
                            </svg>
                            <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No custom thresholds</h3>
                            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Create custom threshold overrides for specific VMs or containers.</p>
                            <div class="mt-4">
                                <button id="goto-custom-thresholds" class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700">
                                    Add Custom Threshold
                                </button>
                            </div>
                        </div>
                    `;
                    
                    const gotoBtn = document.getElementById('goto-custom-thresholds');
                    if (gotoBtn) {
                        gotoBtn.addEventListener('click', () => switchTab('alerts'));
                    }
                } else {
                    // Display the custom thresholds
                    container.innerHTML = `
                        <div class="space-y-4">
                            ${result.data.map(config => formatCustomThresholdCard(config)).join('')}
                        </div>
                    `;
                }
            } else {
                throw new Error(result.error || 'Failed to load custom thresholds');
            }
        } catch (error) {
            console.error('[Settings] Error loading custom thresholds:', error);
            container.innerHTML = `
                <div class="text-center py-8">
                    <svg class="mx-auto h-12 w-12 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                    </svg>
                    <p class="mt-2 text-sm text-red-600 dark:text-red-400">Failed to load custom thresholds: ${error.message}</p>
                    <button onclick="loadCustomThresholds()" class="mt-2 text-sm text-blue-600 hover:text-blue-800">Try again</button>
                </div>
            `;
        }
    }

    function formatCustomThresholdCard(config) {
        const createdDate = new Date(config.createdAt || Date.now()).toLocaleDateString();
        
        // Build thresholds display
        const thresholds = [];
        if (config.thresholds.cpu) {
            thresholds.push(`CPU: ${config.thresholds.cpu.warning || 'N/A'}%/${config.thresholds.cpu.critical || 'N/A'}%`);
        }
        if (config.thresholds.memory) {
            thresholds.push(`Memory: ${config.thresholds.memory.warning || 'N/A'}%/${config.thresholds.memory.critical || 'N/A'}%`);
        }
        if (config.thresholds.disk) {
            thresholds.push(`Disk: ${config.thresholds.disk.warning || 'N/A'}%/${config.thresholds.disk.critical || 'N/A'}%`);
        }
        
        return `
            <div class="border border-gray-200 dark:border-gray-600 rounded-lg p-4">
                <div class="flex items-start justify-between mb-3">
                    <div class="flex-1">
                        <div class="flex items-center gap-2 mb-2">
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">
                                ${config.endpointId}:${config.vmid}
                            </h4>
                            <span class="px-2 py-1 text-xs font-medium rounded ${config.enabled ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}">
                                ${config.enabled ? 'ENABLED' : 'DISABLED'}
                            </span>
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                            Node: ${config.nodeId} • Created: ${createdDate}
                        </div>
                        <div class="text-sm text-gray-600 dark:text-gray-400">
                            Thresholds: ${thresholds.join(', ') || 'None configured'}
                        </div>
                    </div>
                    <div class="flex gap-2">
                        <button onclick="editCustomThreshold('${config.endpointId}', '${config.nodeId}', '${config.vmid}')" 
                                class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded">
                            Edit
                        </button>
                        <button onclick="deleteCustomThreshold('${config.endpointId}', '${config.nodeId}', '${config.vmid}')" 
                                class="px-2 py-1 text-xs bg-red-600 hover:bg-red-700 text-white rounded">
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    // Global functions for dynamic rule management (called from HTML onclick)
    window.toggleDynamicRule = async function(ruleId, enabled) {
        try {
            const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            
            const result = await response.json();
            if (result.success) {
                loadDynamicRules(); // Reload the rules
                showNotification(`Rule ${enabled ? 'enabled' : 'disabled'} successfully`);
            } else {
                throw new Error(result.error);
            }
        } catch (error) {
            console.error('Error toggling dynamic rule:', error);
            alert(`Failed to ${enabled ? 'enable' : 'disable'} rule: ${error.message}`);
        }
    };

    window.deleteDynamicRule = async function(ruleId) {
        if (!confirm('Are you sure you want to delete this alert rule? This action cannot be undone.')) {
            return;
        }
        
        try {
            const response = await fetch(`/api/alerts/rules/${ruleId}`, {
                method: 'DELETE'
            });
            
            const result = await response.json();
            if (result.success) {
                loadDynamicRules(); // Reload the rules
                showNotification('Rule deleted successfully');
            } else {
                throw new Error(result.error);
            }
        } catch (error) {
            console.error('Error deleting dynamic rule:', error);
            alert(`Failed to delete rule: ${error.message}`);
        }
    };

    window.editCustomThreshold = function(endpointId, nodeId, vmid) {
        // Switch to alerts tab to edit
        switchTab('alerts');
        setTimeout(() => {
            // Scroll to custom threshold section and highlight it
            const customSection = document.querySelector('h3[contains("Custom Threshold Configurations")]');
            if (customSection) {
                customSection.scrollIntoView({ behavior: 'smooth' });
            }
            showNotification(`Navigate to the Custom Threshold section to edit ${endpointId}:${vmid}`);
        }, 100);
    };

    window.deleteCustomThreshold = async function(endpointId, nodeId, vmid) {
        if (!confirm(`Are you sure you want to delete the custom threshold for ${endpointId}:${vmid}?`)) {
            return;
        }
        
        try {
            const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`, {
                method: 'DELETE'
            });
            
            const result = await response.json();
            if (result.success) {
                loadCustomThresholds(); // Reload the thresholds
                showNotification('Custom threshold deleted successfully');
            } else {
                throw new Error(result.error);
            }
        } catch (error) {
            console.error('Error deleting custom threshold:', error);
            alert(`Failed to delete custom threshold: ${error.message}`);
        }
    };

    function testAllNotifications() {
        showNotification('Testing all notification methods... (Feature coming soon)');
    }

    function exportAlertConfiguration() {
        showNotification('Exporting alert configuration... (Feature coming soon)');
    }

    function showNotification(message) {
        // Create a simple notification - you can enhance this
        const notification = document.createElement('div');
        notification.className = 'fixed top-4 right-4 z-50 bg-blue-600 text-white px-4 py-2 rounded-md shadow-lg';
        notification.textContent = message;
        document.body.appendChild(notification);
        
        setTimeout(() => {
            notification.remove();
        }, 3000);
    }

    function renderAlertManagementTab() {
        return `
            <div class="space-y-6">
                <!-- Header Section -->
                <div class="text-center">
                    <h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Alert Management</h2>
                    <p class="mt-2 text-gray-600 dark:text-gray-400">
                        Manage all your alert configurations in one place
                    </p>
                </div>

                <!-- Dynamic Threshold Alert Rules Section -->
                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between">
                            <div>
                                <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Dynamic Threshold Rules</h3>
                                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                                    Global alert rules that apply to all VMs/containers meeting threshold criteria
                                </p>
                            </div>
                            <button id="refresh-dynamic-rules" class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600">
                                <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                                </svg>
                                Refresh
                            </button>
                        </div>
                    </div>
                    <div id="dynamic-rules-container" class="p-6">
                        <div class="flex items-center justify-center py-8">
                            <div class="text-center">
                                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 100 4m0-4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 100 4m0-4v2m0-6V4"></path>
                                </svg>
                                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading dynamic threshold rules...</p>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Custom Per-VM/LXC Thresholds Section -->
                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between">
                            <div>
                                <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Per-VM/LXC Custom Thresholds</h3>
                                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                                    Individual threshold overrides for specific virtual machines and containers
                                </p>
                            </div>
                            <div class="flex space-x-2">
                                <button id="refresh-custom-thresholds" class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600">
                                    <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                                    </svg>
                                    Refresh
                                </button>
                                <button id="add-custom-threshold" class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700">
                                    <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path>
                                    </svg>
                                    Add Custom Threshold
                                </button>
                            </div>
                        </div>
                    </div>
                    <div id="custom-thresholds-container" class="p-6">
                        <div class="flex items-center justify-center py-8">
                            <div class="text-center">
                                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
                                </svg>
                                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading custom thresholds...</p>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Quick Actions Section -->
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Quick Actions</h4>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <button id="create-threshold-rule" class="flex items-center justify-center px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                            </svg>
                            Create Threshold Rule
                        </button>
                        <button id="test-notifications" class="flex items-center justify-center px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-5 5v-5zm-5-5l3 3m0 0l3-3m-3 3V8"></path>
                            </svg>
                            Test Notifications
                        </button>
                        <button id="export-alerts" class="flex items-center justify-center px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                            </svg>
                            Export Configuration
                        </button>
                    </div>
                </div>
            </div>
        `;
    }
    
    return {
        init,
        openModal,
        openModalWithTab,
        closeModal,
        addPveEndpoint,
        addPbsEndpoint,
        removeEndpoint,
        testConnections,
        checkForUpdates,
        applyUpdate,
        changeTheme,
        runDiagnostics,
        copyDiagnosticReport,
        downloadDiagnosticReport,
        onUpdateChannelChange,
        switchToRecommendedChannel,
        switchToChannel,
        acknowledgeStableChoice,
        proceedWithStableSwitch,
        clearUpdateCache,
        // Expose alert-related functions for the alert management modal
        renderAlertsTab,
        renderAlertManagementTab,
        loadThresholdConfigurations,
        setupEmailProviderSelection,
        setupEmailTestButton,
        setupWebhookTestButton,
        initializeAlertManagementTab,
        getCurrentConfig: () => currentConfig
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.settings.init);
} else {
    PulseApp.ui.settings.init();
}