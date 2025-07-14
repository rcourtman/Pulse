PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.settings = (() => {
    const logger = PulseApp.utils.createLogger('Settings');
    let currentConfig = {};
    let isInitialized = false;
    let activeTab = 'proxmox';
    let latestReleaseData = null; // Store the latest release data
    let updateCache = new Map(); // Cache update check results to reduce API calls
    let updateCheckTimeout = null; // Debounce rapid channel changes
    let formDataCache = {}; // Store form data between tab switches
    let originalFormData = null; // Store original form data to detect changes
    let hasUnsavedChanges = false; // Track if form has unsaved changes

    function init() {
        if (isInitialized) return;
        
        
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
            cancelButton.addEventListener('click', () => {
                hasUnsavedChanges = false;
                originalFormData = null;
                formDataCache = {};
                closeModal();
            });
        }

        if (saveButton) {
            saveButton.addEventListener('click', saveConfiguration);
        }

        // Setup modal with modalManager - handles click outside and escape key automatically
        if (modal) {
            PulseApp.modalManager.setupModal(modal, {
                closeButton: closeButton,
                onClose: () => {
                    hasUnsavedChanges = false;
                    originalFormData = null;
                    preserveCurrentFormData();
                    formDataCache = {};
                }
            });
        }

        // Set up tab navigation
        setupTabNavigation();

        isInitialized = true;
    }

    function setupTabNavigation() {
        const tabButtons = document.querySelectorAll('.settings-tab');
        
        tabButtons.forEach(button => {
            button.addEventListener('click', async (e) => {
                e.preventDefault();
                const tabName = e.currentTarget.getAttribute('data-tab');
                await switchTab(tabName);
            });
        });
    }
    

    async function switchTab(tabName) {
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
        
        if (activeTab === 'system') {
            await loadCurrentVersion();
        }
        
        // Restore form data for the new tab
        restoreFormData();
    }

    async function openModal() {
        await openModalWithTab('proxmox');
    }
    
    async function openModalWithTab(tabName) {
        // Reset state
        hasUnsavedChanges = false;
        originalFormData = null;
        formDataCache = {};
        
        // Show the modal using modalManager
        PulseApp.modalManager.openModal('#settings-modal');

        // Load current configuration
        await loadConfiguration();
        
        // Switch to requested tab
        await switchTab(tabName);
    }

    function closeModal() {
        // Preserve current form data before closing
        preserveCurrentFormData();
        
        // Use modalManager to close
        PulseApp.modalManager.closeModal('#settings-modal');
        
        // Clear form data cache since modal is being closed
        formDataCache = {};
    }

    async function loadConfiguration() {
        try {
            const data = await PulseApp.apiClient.get('/api/config');
            currentConfig = data;
            renderTabContent();
        } catch (error) {
            PulseApp.apiClient.handleError(error, 'Load configuration', showMessage);
        }
    }
    
    async function loadCurrentVersion() {
        try {
            const versionData = await PulseApp.apiClient.get('/api/version');
            const currentVersionElement = document.getElementById('current-version');
            const deploymentTypeElement = document.getElementById('deployment-type');
            
            if (currentVersionElement && versionData.version) {
                currentVersionElement.textContent = versionData.version;
                // Update currentConfig with the dynamic version for consistency
                currentConfig.version = versionData.version;
                
                // Update deployment type indicator
                if (deploymentTypeElement && versionData.isDocker !== undefined) {
                    deploymentTypeElement.textContent = versionData.isDocker ? 'Docker' : 'Native';
                    deploymentTypeElement.className = 'text-xs text-gray-500 dark:text-gray-400 mt-1';
                    if (versionData.isDocker) {
                        deploymentTypeElement.innerHTML = '<span class="inline-flex items-center gap-1"><svg class="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.185.185 0 00-.185.185v1.888c0 .102.082.185.185.185m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.185.185 0 00-.185.185v1.887c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.185.185 0 00-.185.185v1.887c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.185.185 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 00.185-.185V9.006a.185.185 0 00-.184-.186h-2.12a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.376 11.376 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288Z"/></svg> Docker</span>';
                    }
                }
            }
            
            // Auto-check for updates
            await checkLatestVersion();
        } catch (error) {
            console.warn('Could not load current version:', error);
            const currentVersionElement = document.getElementById('current-version');
            if (currentVersionElement) {
                currentVersionElement.textContent = currentConfig.version || 'Unknown';
            }
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

        let content = '';

        switch (activeTab) {
            case 'proxmox':
                content = renderProxmoxTab(proxmox, safeConfig);
                break;
            case 'pbs':
                content = renderPBSTab(pbs, safeConfig);
                break;
            case 'notifications':
                content = renderNotificationsTab(safeConfig);
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
            setupChangeTracking();
        }, 0);
        
        // Load existing additional endpoints for Proxmox and PBS tabs
        if (activeTab === 'proxmox') {
            loadExistingPveEndpoints();
        } else if (activeTab === 'pbs') {
            loadExistingPbsEndpoints();
        } else if (activeTab === 'system') {
            // Auto-check for latest version when system tab is opened
            checkLatestVersion();
            
            // Ensure embed origins field visibility matches checkbox state
            setTimeout(() => {
                const embedCheckbox = document.querySelector('input[name="ALLOW_EMBEDDING"]');
                if (embedCheckbox) {
                    updateEmbedOriginVisibility(embedCheckbox.checked);
                }
            }, 100);
        }
    }

    function renderProxmoxTab(proxmox, config) {
        let host = proxmox?.host || config.PROXMOX_HOST || '';
        let port = proxmox?.port || config.PROXMOX_PORT || '';
        const tokenId = proxmox?.tokenId || config.PROXMOX_TOKEN_ID || '';
        const enabled = proxmox?.enabled !== undefined ? proxmox.enabled : (config.PROXMOX_ENABLED !== 'false');
        
        // Check if this is an existing configuration with a stored secret
        // Use the hasToken flag from the API response
        const hasStoredToken = proxmox?.hasToken || false;
        
        // Clean the host value if it contains protocol or port, and extract port if needed
        if (host) {
            const originalHost = host;
            host = host.replace(/^https?:\/\//, '');
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
                            Need help creating API tokens?
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
                               oninput="PulseApp.ui.settings.validateHost(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT"
                               value="${port}"
                               placeholder="8006"
                               oninput="PulseApp.ui.settings.validatePort(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox VE web interface port</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token ID <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_TOKEN_ID" required
                               value="${tokenId}"
                               placeholder="root@pam!token-name"
                               oninput="PulseApp.ui.settings.validateTokenId(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token Secret ${hasStoredToken ? '' : '<span class="text-red-500">*</span>'}
                        </label>
                        <div class="relative">
                            <input type="password" name="PROXMOX_TOKEN_SECRET"
                                   ${hasStoredToken ? 'value="••••••••••••••••••••••••••••••••••••"' : ''}
                                   placeholder="${hasStoredToken ? '' : 'Enter token secret'}"
                                   oninput="if(this.value.includes('•')) this.value = this.value.replace(/•/g, '');"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Enabled</label>
                        <input type="checkbox" name="PROXMOX_ENABLED" ${enabled ? 'checked' : ''}
                               class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                    </div>
                </div>
            </div>

            <!-- Additional Endpoints Container -->
            <div id="pve-endpoints-container" class="space-y-4 mt-4">
                <!-- Additional endpoints will be added here -->
            </div>
            
            <!-- Test Connections -->
            <div class="flex justify-between items-center mt-6">
                <button type="button" onclick="PulseApp.ui.settings.testConnections()" 
                        class="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors">
                    Test All Connections
                </button>
            </div>
            
            <!-- Add More Button at Bottom -->
            <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                <button type="button" onclick="PulseApp.ui.settings.addPveEndpoint()" 
                        class="w-full flex items-center justify-center gap-2 px-4 py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-blue-500 dark:hover:border-blue-400 text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 text-sm rounded-md transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                    </svg>
                    Add Another Proxmox VE Server
                </button>
            </div>
        `;
    }

    function renderPBSTab(pbs, config) {
        let host = pbs?.host || config.PBS_HOST || '';
        let port = pbs?.port || config.PBS_PORT || '';
        const tokenId = pbs?.tokenId || config.PBS_TOKEN_ID || '';
        
        // Check if this is an existing configuration with a stored secret
        // Use the hasToken flag from the API response
        const hasStoredToken = pbs?.hasToken || false;
        
        // Clean the host value if it contains protocol or port, and extract port if needed
        if (host) {
            const originalHost = host;
            host = host.replace(/^https?:\/\//, '');
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
                        <a href="https://pbs.proxmox.com/docs/user-management.html#api-tokens" target="_blank" rel="noopener noreferrer" 
                           class="text-blue-600 dark:text-blue-400 hover:underline ml-1">
                            Need help creating PBS API tokens?
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
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PBS_TOKEN_ID"
                               value="${tokenId}"
                               placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token Secret ${hasStoredToken ? '' : '<span class="text-red-500">*</span>'}
                        </label>
                        <div class="relative">
                            <input type="password" name="PBS_TOKEN_SECRET"
                                   ${hasStoredToken ? 'value="••••••••••••••••••••••••••••••••••••"' : ''}
                                   placeholder="${hasStoredToken ? '' : 'Enter token secret'}"
                                   oninput="if(this.value.includes('•')) this.value = this.value.replace(/•/g, '');"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                    </div>
                </div>
            </div>

            <!-- Additional Endpoints Container -->
            <div id="pbs-endpoints-container" class="space-y-4 mt-4">
                <!-- Additional endpoints will be added here -->
            </div>
            
            <!-- Test Connections -->
            <div class="flex justify-between items-center mt-6">
                <button type="button" onclick="PulseApp.ui.settings.testConnections()" 
                        class="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-md transition-colors">
                    Test All Connections
                </button>
            </div>
            
            <!-- Add More Button at Bottom -->
            <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                <button type="button" onclick="PulseApp.ui.settings.addPbsEndpoint()" 
                        class="w-full flex items-center justify-center gap-2 px-4 py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-blue-500 dark:hover:border-blue-400 text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 text-sm rounded-md transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                    </svg>
                    Add Another Proxmox Backup Server
                </button>
            </div>
        `;
    }

    function renderNotificationsTab(config) {
        return `
            <!-- Email Configuration -->
            <div id="email-config-section" class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 space-y-4 mb-6">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Email Configuration</h4>
                
                <!-- Email Provider Selection -->
                <div class="mb-4">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Email Provider</label>
                    <div class="flex gap-3">
                        <button type="button" 
                                id="email-provider-sendgrid"
                                onclick="PulseApp.ui.settings.setEmailProvider('sendgrid')"
                                class="px-4 py-2 text-sm font-medium rounded-md border ${config.EMAIL_PROVIDER === 'sendgrid' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-700 dark:text-blue-300' : 'bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'}">
                            SendGrid
                        </button>
                        <button type="button" 
                                id="email-provider-smtp"
                                onclick="PulseApp.ui.settings.setEmailProvider('smtp')"
                                class="px-4 py-2 text-sm font-medium rounded-md border ${config.EMAIL_PROVIDER !== 'sendgrid' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-700 dark:text-blue-300' : 'bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'}">
                            SMTP
                        </button>
                    </div>
                    <input type="hidden" name="EMAIL_PROVIDER" value="${config.EMAIL_PROVIDER || 'smtp'}">
                </div>
                
                <!-- SendGrid Configuration -->
                <div id="sendgrid-config" class="${config.EMAIL_PROVIDER === 'sendgrid' ? '' : 'hidden'}">
                    <div class="grid grid-cols-1 gap-3">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SendGrid API Key</label>
                            <input type="password" name="SENDGRID_API_KEY" 
                                   value="${config.SENDGRID_API_KEY || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="SG.xxxxxxxxxxxxxxxxxxxx">
                            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                                <a href="https://app.sendgrid.com/settings/api_keys" target="_blank" class="text-blue-600 dark:text-blue-400 hover:underline">
                                    Get your SendGrid API key →
                                </a>
                            </p>
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">From Email</label>
                            <input type="email" name="SENDGRID_FROM_EMAIL" 
                                   value="${config.SENDGRID_FROM_EMAIL || config.ALERT_FROM_EMAIL || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="alerts@yourdomain.com">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">To Email</label>
                            <input type="email" name="ALERT_TO_EMAIL_SENDGRID" 
                                   value="${config.ALERT_TO_EMAIL || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="admin@yourdomain.com">
                        </div>
                    </div>
                </div>
                
                <!-- SMTP Configuration -->
                <div id="smtp-config" class="${config.EMAIL_PROVIDER === 'sendgrid' ? 'hidden' : ''}">
                    <div class="grid grid-cols-1 gap-3">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">From Email</label>
                            <input type="email" name="ALERT_FROM_EMAIL" 
                                   value="${config.ALERT_FROM_EMAIL || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="alerts@yourdomain.com">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">To Email</label>
                            <input type="email" name="ALERT_TO_EMAIL" 
                                   value="${config.ALERT_TO_EMAIL || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="admin@yourdomain.com">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SMTP Server</label>
                            <input type="text" name="SMTP_HOST" 
                                   value="${config.SMTP_HOST || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="smtp.gmail.com">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">SMTP Port</label>
                            <input type="number" name="SMTP_PORT" 
                                   value="${config.SMTP_PORT || 587}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="587">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Username</label>
                            <input type="text" name="SMTP_USER" 
                                   value="${config.SMTP_USER || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="your.email@gmail.com">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Password</label>
                            <input type="password" name="SMTP_PASS" 
                                   value="${config.SMTP_PASS || ''}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" 
                                   placeholder="Enter password">
                        </div>
                    </div>
                </div>
                
                <!-- Email Provider Setup Guides -->
                <div class="mt-4 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                    <div class="flex items-start">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-blue-600 dark:text-blue-400 mr-3 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <div class="w-full">
                            <h4 class="text-sm font-medium text-blue-800 dark:text-blue-200 mb-2">Email Provider Setup Guides</h4>
                            <p class="text-sm text-blue-700 dark:text-blue-300 mb-3">Most email providers require app-specific passwords for security. Follow these guides to set up your email:</p>
                            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                                <a href="https://support.google.com/accounts/answer/185833" target="_blank" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 underline">
                                    Gmail App Password →
                                </a>
                                <a href="https://support.microsoft.com/en-us/account-billing/using-app-passwords-with-apps-that-don-t-support-two-step-verification-5896ed9b-4263-e681-128a-a6f2979a7944" target="_blank" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 underline">
                                    Outlook App Password →
                                </a>
                                <a href="https://help.yahoo.com/kb/generate-third-party-passwords-sln15241.html" target="_blank" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 underline">
                                    Yahoo App Password →
                                </a>
                                <a href="https://support.apple.com/en-us/102654" target="_blank" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 underline">
                                    iCloud App Password →
                                </a>
                            </div>
                        </div>
                    </div>
                </div>
                
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                    <label class="flex items-center">
                        <input type="checkbox" name="SMTP_SECURE" 
                               ${config.SMTP_SECURE !== false ? 'checked' : ''}
                               class="h-4 w-4 text-blue-600 border-gray-300 rounded">
                        <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Use SSL/TLS encryption</span>
                    </label>
                    <div class="flex gap-3">
                        <button type="button" id="test-email-btn" 
                                onclick="PulseApp.ui.settings.testEmailConfiguration()"
                                class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                            Test Email
                        </button>
                    </div>
                </div>
            </div>

            <!-- Webhook Configuration -->
            <div id="webhook-config-section" class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                <div class="flex items-center justify-between mb-3">
                    <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Webhook Configuration</h4>
                    <span id="webhook-status-indicator" class="text-xs"></span>
                </div>
                <div class="space-y-3">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Webhook URL</label>
                        <input type="url" name="WEBHOOK_URL" 
                               id="webhook-url-input"
                               value="${config.WEBHOOK_URL || ''}"
                               class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100" 
                               placeholder="https://discord.com/api/webhooks/... or https://hooks.slack.com/..."
                               oninput="PulseApp.ui.settings.updateWebhookStatus()">
                        <div class="mt-2 text-xs text-gray-600 dark:text-gray-400">
                            <details class="cursor-pointer">
                                <summary class="hover:text-gray-800 dark:hover:text-gray-200">
                                    <i class="fas fa-question-circle"></i> How to get a webhook URL?
                                </summary>
                                <div class="mt-2 space-y-3 bg-gray-50 dark:bg-gray-800 p-3 rounded-md">
                                    <div>
                                        <strong class="text-gray-700 dark:text-gray-300">Discord:</strong>
                                        <ol class="ml-4 mt-1 text-xs space-y-1">
                                            <li>1. Go to Server Settings → Integrations → Webhooks</li>
                                            <li>2. Click "New Webhook"</li>
                                            <li>3. Copy the Webhook URL</li>
                                        </ol>
                                    </div>
                                    <div>
                                        <strong class="text-gray-700 dark:text-gray-300">Slack:</strong>
                                        <ol class="ml-4 mt-1 text-xs space-y-1">
                                            <li>1. Go to your Slack workspace → Apps</li>
                                            <li>2. Search for "Incoming Webhooks"</li>
                                            <li>3. Add to channel and copy the URL</li>
                                        </ol>
                                    </div>
                                    <div>
                                        <strong class="text-gray-700 dark:text-gray-300">Home Assistant:</strong>
                                        <ol class="ml-4 mt-1 text-xs space-y-1">
                                            <li>1. Go to Settings → Automations</li>
                                            <li>2. Create automation with "Webhook" trigger</li>
                                            <li>3. Copy the webhook URL</li>
                                        </ol>
                                    </div>
                                    <div>
                                        <strong class="text-gray-700 dark:text-gray-300">Microsoft Teams:</strong>
                                        <ol class="ml-4 mt-1 text-xs space-y-1">
                                            <li>1. Right-click channel → Connectors</li>
                                            <li>2. Configure "Incoming Webhook"</li>
                                            <li>3. Copy the webhook URL</li>
                                        </ol>
                                    </div>
                                    <div class="pt-2 border-t border-gray-200 dark:border-gray-600">
                                        <p class="text-xs">
                                            <i class="fas fa-info-circle text-blue-500"></i>
                                            Pulse automatically formats messages for Discord and Slack.
                                            Other services receive standard JSON payloads.
                                        </p>
                                    </div>
                                </div>
                            </details>
                        </div>
                    </div>
                    <div class="flex justify-between items-center pt-3 border-t border-gray-200 dark:border-gray-700">
                        <span id="webhook-cooldown-info" class="text-xs text-gray-600 dark:text-gray-400"></span>
                        <button type="button" id="test-webhook-btn" 
                                onclick="PulseApp.ui.settings.testWebhookConfiguration()"
                                class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md transition-colors">
                            Test Webhook
                        </button>
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
                <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Allow Iframe Embedding
                            </label>
                            <p class="text-sm text-gray-500 dark:text-gray-400">
                                Enable this to embed Pulse in other applications (e.g., Homepage, Organizr)
                            </p>
                        </div>
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" name="ALLOW_EMBEDDING" value="true"
                                   ${advanced.allowEmbedding === true ? 'checked' : ''}
                                   onchange="PulseApp.ui.settings.updateEmbedOriginVisibility(this.checked)"
                                   class="sr-only peer">
                            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                        </label>
                    </div>
                    <div id="embedOriginsContainer" class="mt-4" style="${advanced.allowEmbedding === true ? '' : 'display: none;'}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Allowed Embed Origins
                        </label>
                        <input type="text" name="ALLOWED_EMBED_ORIGINS" 
                               value="${advanced.allowedEmbedOrigins || ''}"
                               placeholder="http://homepage.lan:3000, https://dashboard.example.com"
                               class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Comma-separated list of origins allowed to embed Pulse. Leave empty for same-origin only.
                        </p>
                    </div>
                    <div class="mt-2 space-y-1">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                            <strong>Security Note:</strong> Enabling this allows Pulse to be embedded in iframes. Only enable if you trust the embedding applications.
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                            <strong>Note:</strong> The page will automatically reload when this setting is changed.
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                            <a href="https://github.com/rcourtman/Pulse#iframe-embedding-support" target="_blank" rel="noopener noreferrer" 
                               class="text-blue-600 dark:text-blue-400 hover:underline">
                                View embedding documentation →
                            </a>
                        </p>
                    </div>
                </div>
                
                <!-- Reverse Proxy Settings -->
                <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Reverse Proxy Configuration</h4>
                    <div class="space-y-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Trust Proxy
                            </label>
                            <select name="TRUST_PROXY" 
                                    onchange="PulseApp.ui.settings.updateTrustProxyVisibility(this.value)"
                                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-gray-200 text-sm">
                                <option value="" ${!advanced.trustProxy || advanced.trustProxy === '' ? 'selected' : ''}>Disabled (Direct connection)</option>
                                <option value="true" ${advanced.trustProxy === 'true' ? 'selected' : ''}>Trust all proxies (⚠️ use with caution)</option>
                                <option value="1" ${advanced.trustProxy === '1' ? 'selected' : ''}>Behind 1 proxy (e.g., Nginx, Caddy)</option>
                                <option value="2" ${advanced.trustProxy === '2' ? 'selected' : ''}>Behind 2 proxies (e.g., Cloudflare + Nginx)</option>
                                <option value="custom" ${advanced.trustProxy && !['true', '1', '2', ''].includes(advanced.trustProxy) ? 'selected' : ''}>Custom (specific IPs/subnets)</option>
                            </select>
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                Configure if Pulse is behind a reverse proxy (Nginx, Caddy, Traefik, etc.)
                            </p>
                        </div>
                        
                        <div id="trustProxyCustom" style="${advanced.trustProxy && !['true', '1', '2', ''].includes(advanced.trustProxy) ? '' : 'display: none;'}">
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Trusted Proxy IPs
                            </label>
                            <input type="text" name="TRUST_PROXY_CUSTOM" 
                                   value="${advanced.trustProxy && !['true', '1', '2', ''].includes(advanced.trustProxy) ? advanced.trustProxy : ''}"
                                   placeholder="10.0.0.0/8, 172.16.0.0/12, 192.168.1.1"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-gray-200 text-sm">
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                Comma-separated list of trusted proxy IPs or subnets
                            </p>
                        </div>
                        
                        <div class="text-xs text-gray-500 dark:text-gray-400">
                            <a href="https://github.com/rcourtman/Pulse/blob/main/docs/REVERSE_PROXY.md" target="_blank" rel="noopener noreferrer" 
                               class="text-blue-600 dark:text-blue-400 hover:underline">
                                View reverse proxy setup guide →
                            </a>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Security Settings -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Security Settings</h3>
                
                <!-- Security Mode -->
                <div class="mb-6">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Security Mode
                    </label>
                    <select name="SECURITY_MODE" 
                            onchange="PulseApp.ui.settings.updateSecurityOptionsVisibility(this.value)"
                            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                        <option value="public" ${(!advanced.security?.mode || advanced.security?.mode === 'public' || advanced.security?.mode === 'open') ? 'selected' : ''}>
                            Public - No authentication (trusted networks only)
                        </option>
                        <option value="private" ${(advanced.security?.mode === 'private' || advanced.security?.mode === 'secure' || advanced.security?.mode === 'strict') ? 'selected' : ''}>
                            Private - Authentication required
                        </option>
                    </select>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        Public mode provides no security. Private mode requires login to access Pulse.
                    </p>
                </div>

                <!-- Security options container - visibility controlled by JavaScript -->
                <div id="security-options-container" style="display: ${(!advanced.security?.mode || advanced.security?.mode === 'public' || advanced.security?.mode === 'open') ? 'none' : 'block'}">
                <!-- Admin Password -->
                <div class="mb-6">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Admin Password ${advanced.security?.hasAdminPassword ? '(Currently Set)' : '(Not Set)'}
                    </label>
                    <input type="password" name="ADMIN_PASSWORD" 
                           placeholder="${advanced.security?.hasAdminPassword ? 'Enter new password to change' : 'Set admin password'}"
                           class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        Leave blank to keep current password. Username is always 'admin'.
                    </p>
                </div>

                <!-- Session Secret -->
                <div class="mb-6">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Session Secret ${advanced.security?.hasSessionSecret ? '(Currently Set)' : '(Not Set)'}
                    </label>
                    <div class="flex space-x-2">
                        <input type="text" name="SESSION_SECRET" 
                               placeholder="${advanced.security?.hasSessionSecret ? 'Session secret is set' : 'Generate or enter session secret'}"
                               class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                        <button type="button" onclick="PulseApp.ui.settings.generateSessionSecret()" 
                                class="px-3 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
                            Generate
                        </button>
                    </div>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        64+ character random string for session encryption. Required for secure/strict modes.
                    </p>
                </div>

                <!-- Session Timeout -->
                <div class="mb-6">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Session Timeout (hours)
                    </label>
                    <input type="number" name="SESSION_TIMEOUT_HOURS" min="1" max="168" 
                           value="${(advanced.security?.sessionTimeout || 86400000) / 3600000}"
                           onchange="this.setAttribute('data-ms', this.value * 3600000)"
                           data-ms="${advanced.security?.sessionTimeout || 86400000}"
                           class="w-32 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        How long sessions stay active (24 hours = 1 day, 168 hours = 1 week)
                    </p>
                </div>

                <!-- Authentication Options -->
                <div class="space-y-4 mb-6">

                    <div class="flex items-center justify-between">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                Enable Audit Logging
                            </label>
                            <p class="text-xs text-gray-500 dark:text-gray-400">
                                Log security events to /opt/pulse/data/audit.log
                            </p>
                        </div>
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" name="AUDIT_LOG" value="true"
                                   ${advanced.security?.auditLog === true ? 'checked' : ''}
                                   class="sr-only peer">
                            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                        </label>
                    </div>
                </div>

                <!-- Advanced Security Settings -->
                <details class="mb-4">
                    <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100">
                        Advanced Security Options ▼
                    </summary>
                    <div class="mt-4 space-y-4 pl-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Bcrypt Rounds
                            </label>
                            <input type="number" name="BCRYPT_ROUNDS" min="8" max="20" 
                                   value="${advanced.security?.bcryptRounds || 10}"
                                   class="w-32 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                Password hashing strength (10 is standard, higher is more secure but slower)
                            </p>
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Max Login Attempts
                            </label>
                            <input type="number" name="MAX_LOGIN_ATTEMPTS" min="3" max="20" 
                                   value="${advanced.security?.maxLoginAttempts || 5}"
                                   class="w-32 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                Failed attempts before account lockout
                            </p>
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Lockout Duration (minutes)
                            </label>
                            <input type="number" name="LOCKOUT_DURATION" min="5" max="1440" 
                                   value="${(advanced.security?.lockoutDuration || 900000) / 60000}"
                                   onchange="this.setAttribute('data-ms', this.value * 60000)"
                                   data-ms="${advanced.security?.lockoutDuration || 900000}"
                                   class="w-32 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:text-gray-200 text-sm">
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                How long to lock account after max attempts
                            </p>
                        </div>

                    </div>
                </details>
                </div>

                <div class="mt-4">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                        <strong>Note:</strong> Changing security mode requires a service restart.
                        <a href="https://github.com/rcourtman/Pulse/blob/main/SECURITY.md" target="_blank" rel="noopener noreferrer" 
                           class="text-blue-600 dark:text-blue-400 hover:underline ml-1">
                            View security documentation →
                        </a>
                    </p>
                </div>
            </div>

            <!-- Alert Settings -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Alert System</h3>
                <div class="flex items-center justify-between">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Enable Alerts
                        </label>
                        <p class="text-sm text-gray-500 dark:text-gray-400">
                            Master switch to enable or disable all alert notifications
                        </p>
                    </div>
                    <label class="relative inline-flex items-center cursor-pointer">
                        <input type="checkbox" name="ALERTS_ENABLED" value="true"
                               ${advanced.alerts?.enabled !== false ? 'checked' : ''}
                               class="sr-only peer">
                        <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </label>
                </div>
                <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                        When disabled, no alerts will be triggered or sent regardless of threshold settings.
                        Configure alert thresholds using the Alerts toggle in the dashboard table, and notification settings in the <a href="#" onclick="PulseApp.ui.settings.openModalWithTab('notifications'); return false;" class="text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-200">Notifications</a> tab.
                    </p>
                </div>
            </div>

            <!-- Update Management -->
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Software Updates</h3>
                
                <!-- Always Visible Update Status Card -->
                <div class="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg mb-4">
                    <div class="flex items-start justify-between">
                        <div class="flex-1">
                            <!-- Version Info -->
                            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
                                <div>
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Current Version</p>
                                    <p id="current-version" class="text-lg font-mono font-semibold text-gray-900 dark:text-gray-100">Loading...</p>
                                    <p id="deployment-type" class="text-xs text-gray-500 dark:text-gray-400 mt-1"></p>
                                </div>
                                <div>
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                        <span id="latest-version-label">Latest Version</span>
                                    </p>
                                    <p id="latest-version" class="text-lg font-mono font-semibold text-gray-900 dark:text-gray-100">-</p>
                                    <p id="last-check-time" class="text-xs text-gray-500 dark:text-gray-400 mt-1"></p>
                                </div>
                            </div>
                            
                            <!-- Update Status Indicator -->
                            <div id="update-status-indicator" class="mb-4">
                                <!-- This will be populated dynamically -->
                            </div>
                            
                            <!-- Update Channel Selection -->
                            <div class="flex items-center gap-4 mb-4">
                                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Channel:</label>
                                <div class="flex gap-2">
                                    <button type="button" 
                                            onclick="PulseApp.ui.settings.switchToChannel('stable')"
                                            class="channel-btn px-3 py-1 text-xs font-medium rounded-md transition-colors ${(!advanced.updateChannel || advanced.updateChannel === 'stable') ? 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 border border-blue-300 dark:border-blue-700' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-600'}"
                                            data-channel="stable">
                                        Stable
                                    </button>
                                    <button type="button"
                                            onclick="PulseApp.ui.settings.switchToChannel('rc')"
                                            class="channel-btn px-3 py-1 text-xs font-medium rounded-md transition-colors ${advanced.updateChannel === 'rc' ? 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 border border-amber-300 dark:border-amber-700' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-600'}"
                                            data-channel="rc">
                                        RC
                                    </button>
                                </div>
                                <div class="ml-auto">
                                    <button type="button" 
                                            onclick="PulseApp.ui.settings.checkForUpdates()" 
                                            id="check-updates-button"
                                            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md flex items-center gap-2 transition-colors">
                                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                        </svg>
                                        Check Now
                                    </button>
                                </div>
                            </div>
                            
                            <!-- Channel Warning (if needed) -->
                            <div id="channel-warning" class="hidden">
                                <!-- This will be populated when switching channels -->
                            </div>
                        </div>
                    </div>
                </div>
                
                <!-- Update Available Section (hidden by default) -->
                <div id="update-available-section" class="hidden">
                    <div class="p-4 bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/20 dark:to-emerald-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                        <div class="flex items-start gap-3">
                            <svg class="w-5 h-5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                            </svg>
                            <div class="flex-1">
                                <h4 class="text-sm font-semibold text-green-800 dark:text-green-200 mb-1">
                                    Update Available: <span id="update-version" class="font-mono"></span>
                                </h4>
                                <p id="update-summary" class="text-sm text-green-700 dark:text-green-300 mb-3"></p>
                                
                                <!-- Deployment-specific instructions -->
                                <div id="update-instructions" class="mb-3">
                                    <!-- This will be populated based on deployment type -->
                                </div>
                                
                                <!-- Expandable changelog -->
                                <details class="group">
                                    <summary class="cursor-pointer text-sm text-green-600 dark:text-green-400 hover:text-green-800 dark:hover:text-green-200 font-medium">
                                        View changelog
                                    </summary>
                                    <div class="mt-3 space-y-3">
                                        <div id="changelog-content" class="bg-white dark:bg-gray-800 rounded-md p-3 text-sm">
                                            <!-- Changelog will be loaded here -->
                                        </div>
                                    </div>
                                </details>
                            </div>
                        </div>
                    </div>
                </div>
                
                <!-- Update Progress (hidden by default) -->
                <div id="update-progress" class="hidden">
                    <div class="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                        <div class="mb-3">
                            <p class="text-sm font-medium text-gray-700 dark:text-gray-300" id="update-progress-text">Preparing update...</p>
                        </div>
                        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                            <div id="update-progress-bar" class="bg-blue-600 h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
                        </div>
                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                            Do not close this window or refresh the page. The page will refresh automatically when the update is complete.
                        </p>
                    </div>
                </div>
                
                <!-- Automatic Updates -->
                <div class="p-4 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <div class="flex items-center justify-between mb-3">
                        <div>
                            <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200">Automatic Updates</h4>
                            <p class="text-xs text-gray-600 dark:text-gray-400 mt-0.5">Check for updates automatically</p>
                        </div>
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" name="AUTO_UPDATE_ENABLED" ${advanced.autoUpdate?.enabled !== false ? 'checked' : ''}
                                   class="sr-only peer">
                            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                        </label>
                    </div>
                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Check Interval
                            </label>
                            <select name="AUTO_UPDATE_CHECK_INTERVAL"
                                    class="w-full px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                                <option value="6" ${advanced.autoUpdate?.checkInterval == 6 ? 'selected' : ''}>Every 6 hours</option>
                                <option value="12" ${advanced.autoUpdate?.checkInterval == 12 ? 'selected' : ''}>Every 12 hours</option>
                                <option value="24" ${(!advanced.autoUpdate?.checkInterval || advanced.autoUpdate?.checkInterval == 24) ? 'selected' : ''}>Daily</option>
                                <option value="168" ${advanced.autoUpdate?.checkInterval == 168 ? 'selected' : ''}>Weekly</option>
                            </select>
                        </div>
                        <div>
                            <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Update Time
                            </label>
                            <input type="time" name="AUTO_UPDATE_TIME"
                                   value="${advanced.autoUpdate?.time || '02:00'}"
                                   class="w-full px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                    </div>
                </div>
                
                <!-- Hidden elements for compatibility -->
                <div id="update-details" class="hidden"></div>
                <input type="hidden" name="UPDATE_CHANNEL" value="${advanced.updateChannel || 'stable'}">
            </div>
        `;
    }


    function renderDiagnosticsTab() {
        return `
            <div class="space-y-6">
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-2">System Diagnostics</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
                        View comprehensive diagnostic information about your Pulse configuration, permissions, and system status.
                    </p>
                    
                    <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4 mb-4">
                        <div class="flex items-start">
                            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 mr-2 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            <div>
                                <p class="text-sm text-blue-800 dark:text-blue-200">
                                    The diagnostics page provides detailed information about:
                                </p>
                                <ul class="mt-2 text-sm text-blue-700 dark:text-blue-300 list-disc list-inside space-y-1">
                                    <li>API token permissions and security status</li>
                                    <li>Connection status for all endpoints</li>
                                    <li>Backup access and storage permissions</li>
                                    <li>Configuration recommendations</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                    
                    <a href="/diagnostics.html" 
                       target="_blank"
                       class="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h8a2 2 0 012 2M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                        Open Diagnostics
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"></path>
                        </svg>
                    </a>
                </div>
                
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-2">Quick Tips</h4>
                    <ul class="text-sm text-gray-600 dark:text-gray-400 space-y-2">
                        <li class="flex items-start">
                            <svg class="w-4 h-4 text-gray-400 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            Run diagnostics when experiencing connection issues or missing data
                        </li>
                        <li class="flex items-start">
                            <svg class="w-4 h-4 text-gray-400 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            Share the diagnostics URL with support for troubleshooting
                        </li>
                        <li class="flex items-start">
                            <svg class="w-4 h-4 text-gray-400 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            The diagnostics page automatically refreshes when accessed
                        </li>
                    </ul>
                </div>
            </div>
        `;
    }


    // Additional endpoint management functions
    function addPveEndpoint(existingData = null, forceIndex = null) {
        const container = document.getElementById('pve-endpoints-container');
        if (!container) return;

        // Hide empty state if this is the first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

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
        
        // Use forced index if provided, otherwise find the first unused index starting from 2
        if (forceIndex !== null) {
            index = forceIndex;
        } else {
            while (usedIndexes.has(index)) {
                index++;
            }
        }
        
        // Check if this endpoint has existing data (i.e., a stored token secret)
        const tokenSecretKey = `PROXMOX_TOKEN_SECRET_${index}`;
        // The secret will be '***REDACTED***' if it exists
        const hasStoredToken = existingData && existingData[tokenSecretKey] && 
                              (existingData[tokenSecretKey] === '***REDACTED***' || existingData[tokenSecretKey].trim() !== '');
        
        const endpointHtml = `
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-4 right-4 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <div class="mb-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox VE Server #${index}</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">Additional PVE server configuration</p>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Host Address <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_HOST_${index}" required
                               placeholder="proxmox${index}.example.com"
                               oninput="PulseApp.ui.settings.validateHost(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PROXMOX_PORT_${index}" 
                               placeholder="8006"
                               oninput="PulseApp.ui.settings.validatePort(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox VE web interface port</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token ID <span class="text-red-500">*</span>
                        </label>
                        <input type="text" name="PROXMOX_TOKEN_ID_${index}" required
                               placeholder="root@pam!token-name"
                               oninput="PulseApp.ui.settings.validateTokenId(this)"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token Secret ${hasStoredToken ? '' : '<span class="text-red-500">*</span>'}
                        </label>
                        <div class="relative">
                            <input type="password" name="PROXMOX_TOKEN_SECRET_${index}" 
                                   ${hasStoredToken ? 'value="••••••••••••••••••••••••••••••••••••"' : ''}
                                   placeholder="${hasStoredToken ? '' : 'Enter token secret'}"
                                   oninput="if(this.value.includes('•')) this.value = this.value.replace(/•/g, '');"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Enabled</label>
                        <input type="checkbox" name="PROXMOX_ENABLED_${index}" checked
                               class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded">
                    </div>
                </div>
            </div>
        `;

        container.insertAdjacentHTML('beforeend', endpointHtml);
    }

    function addPbsEndpoint(existingData = null, forceIndex = null) {
        const container = document.getElementById('pbs-endpoints-container');
        if (!container) return;

        // Hide empty state if this is the first endpoint
        const emptyState = container.querySelector('.border-dashed');
        if (emptyState) {
            emptyState.style.display = 'none';
        }

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
        
        // Use forced index if provided, otherwise find the first unused index starting from 2
        if (forceIndex !== null) {
            index = forceIndex;
        } else {
            while (usedIndexes.has(index)) {
                index++;
            }
        }
        
        // Check if this endpoint has existing data (i.e., a stored token secret)
        const tokenSecretKey = `PBS_TOKEN_SECRET_${index}`;
        // The secret will be '***REDACTED***' if it exists
        const hasStoredToken = existingData && existingData[tokenSecretKey] && 
                              (existingData[tokenSecretKey] === '***REDACTED***' || existingData[tokenSecretKey].trim() !== '');
        
        const endpointHtml = `
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 relative">
                <button type="button" onclick="PulseApp.ui.settings.removeEndpoint(this)" 
                        class="absolute top-4 right-4 text-red-600 hover:text-red-800" title="Remove this server">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
                <div class="mb-4">
                    <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox Backup Server #${index}</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">Additional PBS server configuration</p>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host Address</label>
                        <input type="text" name="PBS_HOST_${index}" 
                               placeholder="pbs${index}.example.com"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">IP address or hostname only (without port number)</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                        <input type="number" name="PBS_PORT_${index}" 
                               placeholder="8007"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Default Proxmox Backup Server web interface port</p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Token ID</label>
                        <input type="text" name="PBS_TOKEN_ID_${index}" 
                               placeholder="root@pam!token-name"
                               class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            API Token Secret ${hasStoredToken ? '' : '<span class="text-red-500">*</span>'}
                        </label>
                        <div class="relative">
                            <input type="password" name="PBS_TOKEN_SECRET_${index}" 
                                   ${hasStoredToken ? 'value="••••••••••••••••••••••••••••••••••••"' : ''}
                                   placeholder="${hasStoredToken ? '' : 'Enter token secret'}"
                                   oninput="if(this.value.includes('•')) this.value = this.value.replace(/•/g, '');"
                                   class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                        </div>
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
                addPveEndpoint(config, i);
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
                            
                            // Don't set the value for token secret fields if they're redacted
                            if (configKey.includes('TOKEN_SECRET') && value === '***REDACTED***') {
                                // Skip - the placeholder will show the bullets
                            } else {
                                input.value = value;
                            }
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
                addPbsEndpoint(config, i);
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
                            
                            // Don't set the value for token secret fields if they're redacted
                            if (configKey.includes('TOKEN_SECRET') && value === '***REDACTED***') {
                                // Skip - the placeholder will show the bullets
                            } else {
                                input.value = value;
                            }
                        }
                    }
                });
            }
        }
    }

    async function testConnections() {
        // Show testing notification after a short delay to avoid flash for quick tests
        const toastTimeout = setTimeout(() => {
            PulseApp.ui.toast.showToast('Testing connections...', 'info');
        }, 500);
        
        const config = collectFormData();
        
        // Keep all config but send empty string for redacted token secrets
        // The backend needs TOKEN_ID to identify which endpoint to test
        const testConfig = {};
        Object.entries(config).forEach(([key, value]) => {
            // Send empty string for redacted token SECRETS so backend knows to use stored value
            if (key.includes('TOKEN_SECRET') && (value === '***REDACTED***' || value.includes('•') || value.includes('\u2022'))) {
                testConfig[key] = '';
            } else {
                // Include everything else as-is
                testConfig[key] = value;
            }
        });
        
        console.log('Testing connections with config:', testConfig);
        const testButton = event?.target || document.querySelector('[onclick*="testConnections"]');
        
        // Clear any previous test indicators
        clearAllTestIndicators();
        
        // Disable button during test
        if (testButton) {
            testButton.disabled = true;
            testButton.textContent = 'Testing...';
        }
        
        try {
            const result = await PulseApp.apiClient.post('/api/config/test', testConfig);
            
            // Clear the testing toast if it hasn't shown yet
            clearTimeout(toastTimeout);
            
            if (result.success) {
                PulseApp.ui.toast.success('All connections tested successfully!');
                // Show success indicators for all configured endpoints
                showTestSuccessIndicators(config);
            } else {
                PulseApp.ui.toast.error(result.error || 'Connection test failed');
                // Try to parse error to show which endpoints failed
                showTestFailureIndicators(config, result.error);
            }
        } catch (error) {
            // Clear the testing toast if it hasn't shown yet
            clearTimeout(toastTimeout);
            
            console.error('Test connections error:', error);
            // Show the actual error message from the server
            PulseApp.ui.toast.error(error.message || 'Failed to test connections');
            showTestFailureIndicators(config, error.message);
        } finally {
            // Re-enable button
            if (testButton) {
                testButton.disabled = false;
                testButton.textContent = 'Test All Connections';
            }
        }
    }
    
    function clearAllTestIndicators() {
        // Remove all existing test indicators
        document.querySelectorAll('.test-indicator').forEach(el => el.remove());
    }
    
    function showTestSuccessIndicators(config) {
        // Show success for primary PVE
        if (config.PROXMOX_HOST && config.PROXMOX_TOKEN_ID) {
            showEndpointTestResult('proxmox-primary', true);
        }
        
        // Show success for additional PVE endpoints
        Object.keys(config).forEach(key => {
            const match = key.match(/^PROXMOX_HOST_(\d+)$/);
            if (match && config[`PROXMOX_TOKEN_ID_${match[1]}`]) {
                showEndpointTestResult(`proxmox-${match[1]}`, true);
            }
        });
        
        // Show success for primary PBS
        if (config.PBS_HOST && config.PBS_TOKEN_ID) {
            showEndpointTestResult('pbs-primary', true);
        }
        
        // Show success for additional PBS endpoints
        Object.keys(config).forEach(key => {
            const match = key.match(/^PBS_HOST_(\d+)$/);
            if (match && config[`PBS_TOKEN_ID_${match[1]}`]) {
                showEndpointTestResult(`pbs-${match[1]}`, true);
            }
        });
    }
    
    function showTestFailureIndicators(config, errorMessage) {
        // Try to parse which endpoints failed from error message
        const failedEndpoints = [];
        if (errorMessage && errorMessage.includes('Connection test failed for:')) {
            // Extract failed endpoint names from error message
            const match = errorMessage.match(/Connection test failed for: (.+)/);
            if (match) {
                failedEndpoints.push(...match[1].split(', ').map(e => e.split(':')[0].trim()));
            }
        }
        
        // Show appropriate indicators based on what's configured
        if (config.PROXMOX_HOST && config.PROXMOX_TOKEN_ID) {
            const failed = failedEndpoints.includes('Primary PVE');
            showEndpointTestResult('proxmox-primary', !failed, failed ? 'Connection failed' : '');
        }
        
        // Check additional PVE endpoints
        Object.keys(config).forEach(key => {
            const match = key.match(/^PROXMOX_HOST_(\d+)$/);
            if (match && config[`PROXMOX_TOKEN_ID_${match[1]}`]) {
                const failed = failedEndpoints.includes(`PVE Endpoint ${match[1]}`);
                showEndpointTestResult(`proxmox-${match[1]}`, !failed, failed ? 'Connection failed' : '');
            }
        });
        
        // Check PBS endpoints
        if (config.PBS_HOST && config.PBS_TOKEN_ID) {
            const failed = failedEndpoints.includes('Primary PBS');
            showEndpointTestResult('pbs-primary', !failed, failed ? 'Connection failed' : '');
        }
        
        Object.keys(config).forEach(key => {
            const match = key.match(/^PBS_HOST_(\d+)$/);
            if (match && config[`PBS_TOKEN_ID_${match[1]}`]) {
                const failed = failedEndpoints.includes(`PBS Endpoint ${match[1]}`);
                showEndpointTestResult(`pbs-${match[1]}`, !failed, failed ? 'Connection failed' : '');
            }
        });
    }
    
    function showEndpointTestResult(endpointId, success, errorMessage = '') {
        // Find the endpoint container
        let container;
        if (endpointId === 'proxmox-primary') {
            container = document.querySelector('#proxmox-servers .bg-gray-50');
        } else if (endpointId === 'pbs-primary') {
            container = document.querySelector('#pbs-servers .bg-gray-50');
        } else if (endpointId.startsWith('proxmox-')) {
            const index = endpointId.replace('proxmox-', '');
            container = document.querySelector(`#proxmox-endpoint-${index}`);
        } else if (endpointId.startsWith('pbs-')) {
            const index = endpointId.replace('pbs-', '');
            container = document.querySelector(`#pbs-endpoint-${index}`);
        }
        
        if (!container) return;
        
        // Remove any existing indicator
        const existingIndicator = container.querySelector('.test-indicator');
        if (existingIndicator) existingIndicator.remove();
        
        // Create new indicator
        const indicator = document.createElement('div');
        indicator.className = 'test-indicator mt-2 p-2 rounded-md flex items-center gap-2 text-sm';
        
        if (success) {
            indicator.className += ' bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400';
            indicator.innerHTML = `
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                <span>Connection successful</span>
            `;
        } else {
            indicator.className += ' bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400';
            indicator.innerHTML = `
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
                <span>${errorMessage || 'Connection failed'}</span>
            `;
        }
        
        // Insert after the last form group in the container
        const lastFormGroup = container.querySelector('.grid:last-of-type');
        if (lastFormGroup) {
            lastFormGroup.after(indicator);
        } else {
            container.appendChild(indicator);
        }
    }

    function generateSessionSecret() {
        // Generate a cryptographically secure random string
        const array = new Uint8Array(32);
        crypto.getRandomValues(array);
        const secret = Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
        
        // Set the value in the input field
        const input = document.querySelector('input[name="SESSION_SECRET"]');
        if (input) {
            input.value = secret;
            // Trigger change event to enable save button
            input.dispatchEvent(new Event('input', { bubbles: true }));
        }
    }

    function updateSecurityOptionsVisibility(mode) {
        const container = document.getElementById('security-options-container');
        if (container) {
            container.style.display = (mode === 'public' || mode === 'open') ? 'none' : 'block';
        }
        
        // Trigger change tracking
        trackChanges();
    }
    
    function updateTrustProxyVisibility(value) {
        const customDiv = document.getElementById('trustProxyCustom');
        if (customDiv) {
            customDiv.style.display = value === 'custom' ? 'block' : 'none';
        }
        
        // Trigger change tracking
        trackChanges();
    }
    
    function updateEmbedOriginVisibility(isChecked) {
        const container = document.getElementById('embedOriginsContainer');
        if (container) {
            container.style.display = isChecked ? 'block' : 'none';
        }
        
        // Trigger change tracking
        trackChanges();
    }

    async function saveConfiguration() {
        const saveButton = document.getElementById('settings-save-button');
        if (!saveButton) return;

        const buttonState = PulseApp.utils.setButtonLoading(saveButton, 'Saving...');

        try {
            // Preserve current tab data before collecting all data
            preserveCurrentFormData();
            const config = collectAllTabsData();
            
            const result = await PulseApp.apiClient.post('/api/config', config);
            
            if (result.success) {
                // Reset unsaved changes state
                hasUnsavedChanges = false;
                originalFormData = collectAllTabsData();
                
                // Check if embedding settings were changed (normalize to boolean for comparison)
                const originalEmbedding = currentConfig.advanced?.allowEmbedding === true;
                // If ALLOW_EMBEDDING is undefined, preserve the original value (don't consider it changed)
                const newEmbedding = config.ALLOW_EMBEDDING !== undefined ? 
                    (config.ALLOW_EMBEDDING === 'true' || config.ALLOW_EMBEDDING === true) : 
                    originalEmbedding;
                const originalOrigins = (currentConfig.advanced?.allowedEmbedOrigins || '').trim();
                // If ALLOWED_EMBED_ORIGINS is undefined, preserve the original value
                const newOrigins = config.ALLOWED_EMBED_ORIGINS !== undefined ? 
                    (config.ALLOWED_EMBED_ORIGINS || '').trim() : 
                    originalOrigins;
                const embeddingChanged = originalEmbedding !== newEmbedding || originalOrigins !== newOrigins;
                
                // Check if security mode changed (normalize to string for comparison)
                const originalSecurityMode = (currentConfig.advanced?.security?.mode || 'private').toString();
                // If SECURITY_MODE is undefined, preserve the original value
                const newSecurityMode = config.SECURITY_MODE !== undefined ? 
                    (config.SECURITY_MODE || 'private').toString() : 
                    originalSecurityMode;
                const securityModeChanged = originalSecurityMode !== newSecurityMode;
                
                // Debug logging
                console.log('Restart detection debug:', {
                    embedding: {
                        original: originalEmbedding,
                        new: newEmbedding,
                        changed: embeddingChanged,
                        originsOld: originalOrigins,
                        originsNew: newOrigins
                    },
                    security: {
                        original: originalSecurityMode,
                        new: newSecurityMode,
                        changed: securityModeChanged
                    },
                    rawValues: {
                        'config.ALLOW_EMBEDDING': config.ALLOW_EMBEDDING,
                        'currentConfig.advanced?.allowEmbedding': currentConfig.advanced?.allowEmbedding
                    }
                });
                
                if (embeddingChanged || securityModeChanged) {
                    const reason = embeddingChanged && securityModeChanged ? 'iframe and security settings' : 
                                 embeddingChanged ? 'iframe settings' : 'security settings';
                    showSuccessToast('Configuration Saved', `Restarting Pulse to apply ${reason}...`);
                    
                    // Trigger service restart
                    setTimeout(async () => {
                        try {
                            const response = await fetch('/api/service/restart', {
                                method: 'POST',
                                headers: {
                                    'Content-Type': 'application/json',
                                    'X-CSRF-Token': sessionStorage.getItem('csrfToken') || ''
                                },
                                body: JSON.stringify({ reason: reason })
                            });
                            
                            if (!response.ok) {
                                throw new Error('Failed to restart service');
                            }
                            
                            // Show restart message
                            PulseApp.ui.toast.showToast('Pulse is restarting. The page will reload automatically when ready.', 'info');
                            
                            // Start checking if service is back online
                            let retries = 0;
                            const maxRetries = 30; // 30 seconds
                            const checkInterval = setInterval(async () => {
                                retries++;
                                try {
                                    const healthResponse = await fetch('/api/health', { method: 'GET' });
                                    if (healthResponse.ok) {
                                        clearInterval(checkInterval);
                                        window.location.reload();
                                    }
                                } catch (e) {
                                    // Service still down, keep trying
                                }
                                
                                if (retries >= maxRetries) {
                                    clearInterval(checkInterval);
                                    PulseApp.ui.toast.showToast('Service restart is taking longer than expected. Please check the service status.', 'error');
                                }
                            }, 1000);
                            
                        } catch (error) {
                            console.error('Failed to restart service:', error);
                            PulseApp.ui.toast.showToast('Could not restart the service. Please restart manually: sudo systemctl restart pulse', 'error');
                        }
                    }, 1500);
                } else {
                    showSuccessToast('Configuration Saved', 'Your settings have been applied successfully');
                    
                    // Update alert mode notification status if alerts module is available
                    if (PulseApp.ui.alerts && PulseApp.ui.alerts.updateNotificationStatus) {
                        PulseApp.ui.alerts.updateNotificationStatus();
                    }
                    
                    // Update save button state
                    trackChanges();
                    
                    // Keep modal open so users can continue making changes
                }
            } else {
                showMessage(result.error || 'Failed to save configuration', 'error');
            }
        } catch (error) {
            console.error('Save configuration error:', error);
            showMessage('Failed to save configuration: ' + error.message, 'error');
        } finally {
            PulseApp.utils.resetButton(saveButton, buttonState);
        }
    }

    function collectFormData() {
        const form = document.getElementById('settings-form');
        if (!form) return {};

        const formData = new FormData(form);
        const config = {};

        // Build the config object from form data
        for (const [name, value] of formData.entries()) {
            // Skip empty values except for password fields (which might be empty if unchanged)
            // and ALLOWED_EMBED_ORIGINS (which needs to be cleared when empty)
            if (value.trim() === '' && 
                !name.includes('TOKEN_SECRET') && 
                !name.includes('ADMIN_PASSWORD') && 
                !name.includes('SESSION_SECRET') &&
                name !== 'ALLOWED_EMBED_ORIGINS') continue;
            
            // Skip token secret fields that contain any bullet points
            if (name.includes('TOKEN_SECRET') && (value.includes('•') || value.includes('\u2022'))) {
                continue;
            }
            
            // Skip SMTP password if it's redacted
            if (name === 'SMTP_PASS' && value === '***REDACTED***') {
                continue;
            }

            // Handle checkbox values
            if (form.querySelector(`[name="${name}"]`).type === 'checkbox') {
                config[name] = 'true';
            } else {
                // Clean PBS_HOST entries - remove protocol and port if included
                if (name.startsWith('PBS_HOST')) {
                    let cleanHost = value.trim();
                    cleanHost = cleanHost.replace(/^https?:\/\//, '');
                    const portMatch = cleanHost.match(/^([^:]+)(:\d+)?$/);
                    if (portMatch) {
                        cleanHost = portMatch[1];
                    }
                    config[name] = cleanHost;
                } else if (name === 'LOCKOUT_DURATION') {
                    // Handle special data-ms attribute for lockout duration
                    const input = form.querySelector(`[name="${name}"]`);
                    config[name] = input.getAttribute('data-ms') || (parseInt(value) * 60000).toString();
                } else if (name === 'TRUST_PROXY' && value === 'custom') {
                    // Handle custom trust proxy value
                    const customValue = formData.get('TRUST_PROXY_CUSTOM');
                    if (customValue && customValue.trim()) {
                        config[name] = customValue.trim();
                    } else {
                        config[name] = '';
                    }
                } else if (name === 'TRUST_PROXY_CUSTOM') {
                    // Skip this as it's handled above
                    continue;
                } else if (name === 'ALERT_TO_EMAIL_SENDGRID') {
                    // Map SendGrid TO email to common field
                    config['ALERT_TO_EMAIL'] = value;
                    // Don't include the original field name
                    continue;
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
                    // Skip token secret fields that contain bullet points
                    if (name.includes('TOKEN_SECRET') && typeof value === 'string' && (value.includes('•') || value.includes('\u2022'))) {
                        return; // Skip this field
                    }
                    
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
                    // Don't cache token secret fields that contain bullet points
                    if (name.includes('TOKEN_SECRET') && (value.includes('•') || value.includes('\u2022'))) {
                        continue; // Skip caching this field
                    }
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
        }
        
        // For other tabs, check if any field has a non-empty value
        return Object.values(data).some(value => {
            if (typeof value === 'string' && value.trim()) return true;
            if (typeof value === 'boolean' && value) return true;
            if (typeof value === 'number' && value !== 0) return true;
            return false;
        });
    }

    function setupChangeTracking() {
        const form = document.getElementById('settings-form');
        if (!form) return;
        
        // Store original form data when first rendered
        if (!originalFormData) {
            originalFormData = collectAllTabsData();
        }
        
        // Add change event listeners to all form inputs
        form.addEventListener('input', trackChanges);
        form.addEventListener('change', trackChanges);
    }
    
    function trackChanges() {
        const currentData = collectAllTabsData();
        const originalDataStr = JSON.stringify(originalFormData || {});
        const currentDataStr = JSON.stringify(currentData);
        
        hasUnsavedChanges = originalDataStr !== currentDataStr;
        
        // Update save button state
        const saveButton = document.getElementById('settings-save-button');
        if (saveButton) {
            if (hasUnsavedChanges) {
                saveButton.textContent = 'Save Changes';
                saveButton.classList.remove('opacity-50', 'cursor-not-allowed');
                saveButton.disabled = false;
            } else {
                saveButton.textContent = 'Save';
                saveButton.classList.add('opacity-50', 'cursor-not-allowed');
                saveButton.disabled = true;
            }
        }
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

    // Field validation functions
    function validateHost(input) {
        const value = input.value.trim();
        const errorEl = input.nextElementSibling;
        
        if (!value && input.hasAttribute('required')) {
            showFieldError(input, 'This field is required');
            return false;
        }
        
        // Basic hostname/IP validation
        const hostnameRegex = /^[a-zA-Z0-9.-]+$/;
        const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
        
        if (value && !hostnameRegex.test(value) && !ipRegex.test(value)) {
            showFieldError(input, 'Please enter a valid hostname or IP address');
            return false;
        }
        
        clearFieldError(input);
        return true;
    }
    
    function validatePort(input) {
        const value = input.value.trim();
        
        if (value) {
            const port = parseInt(value);
            if (isNaN(port) || port < 1 || port > 65535) {
                showFieldError(input, 'Port must be between 1 and 65535');
                return false;
            }
        }
        
        clearFieldError(input);
        return true;
    }
    
    function validateTokenId(input) {
        const value = input.value.trim();
        
        if (!value && input.hasAttribute('required')) {
            showFieldError(input, 'This field is required');
            return false;
        }
        
        // Token ID format: user@realm!tokenname
        if (value && !value.match(/^[^@]+@[^!]+![^!]+$/)) {
            showFieldError(input, 'Format: user@realm!tokenname (e.g., monitor@pam!pulse)');
            return false;
        }
        
        clearFieldError(input);
        return true;
    }
    
    function showFieldError(input, message) {
        input.classList.add('border-red-500', 'focus:ring-red-500', 'focus:border-red-500');
        input.classList.remove('border-gray-300', 'dark:border-gray-600');
        
        // Find or create error element
        let errorEl = input.parentElement.querySelector('.field-error');
        if (!errorEl) {
            errorEl = document.createElement('p');
            errorEl.className = 'field-error mt-1 text-xs text-red-600 dark:text-red-400';
            input.parentElement.appendChild(errorEl);
        }
        errorEl.textContent = message;
        
        input.setAttribute('aria-invalid', 'true');
        input.setAttribute('aria-describedby', errorEl.id || 'error-' + Math.random().toString(36).substr(2, 9));
    }
    
    function clearFieldError(input) {
        input.classList.remove('border-red-500', 'focus:ring-red-500', 'focus:border-red-500');
        input.classList.add('border-gray-300', 'dark:border-gray-600');
        
        const errorEl = input.parentElement.querySelector('.field-error');
        if (errorEl) {
            errorEl.remove();
        }
        
        input.setAttribute('aria-invalid', 'false');
        input.removeAttribute('aria-describedby');
    }

    function showSuccessToast(title, subtitle) {
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
    }

    // Check for latest version from GitHub releases
    // @param {string} channelOverride - Optional channel to check instead of saved config
    async function checkLatestVersion(channelOverride = null) {
        const latestVersionElement = document.getElementById('latest-version');
        const latestVersionLabelElement = document.getElementById('latest-version-label');
        const updateStatusIndicator = document.getElementById('update-status-indicator');
        const lastCheckTime = document.getElementById('last-check-time');
        const checkButton = document.getElementById('check-updates-button');
        
        if (!latestVersionElement) return;
        
        // Update button state
        if (checkButton) {
            checkButton.disabled = true;
            checkButton.innerHTML = '<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Checking...';
        }
        
        try {
            latestVersionElement.textContent = 'Checking...';
            latestVersionElement.className = 'text-lg font-mono font-semibold text-gray-500 dark:text-gray-400';
            
            // Check cache first to reduce API calls
            const cacheKey = channelOverride || 'default';
            const cacheExpiry = 5 * 60 * 1000; // 5 minutes
            const cachedResult = updateCache.get(cacheKey);
            
            let data;
            if (cachedResult && (Date.now() - cachedResult.timestamp) < cacheExpiry) {
                data = cachedResult.data;
            } else {
                // Use the server's update check API with optional channel override
                const url = channelOverride ? `/api/updates/check?channel=${channelOverride}` : '/api/updates/check';
                data = await PulseApp.apiClient.get(url);
                
                // Cache the result
                updateCache.set(cacheKey, {
                    data: data,
                    timestamp: Date.now()
                });
            }
            
            // Update last check time
            if (lastCheckTime) {
                lastCheckTime.textContent = 'Last checked: just now';
            }
            
            // Add preview indicator if using channel override
            const isPreview = !!channelOverride;
            
            if (data && data.latestVersion) {
                const latestVersion = data.latestVersion;
                const currentVersion = data.currentVersion || currentConfig.version || 'Unknown';
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
                        latestVersionLabelElement.textContent = 'Latest RC';
                    } else {
                        latestVersionLabelElement.textContent = 'Latest Stable';
                    }
                }
                
                // Check for channel mismatch and show recommendations
                const currentVersionLower = currentVersion.toLowerCase();
                const isCurrentRC = currentVersionLower.includes('-rc') || currentVersionLower.includes('-alpha') || currentVersionLower.includes('-beta');
                const shouldShowRecommendation = (!isCurrentRC && updateChannel === 'rc');
                
                // Show channel warning if needed
                const channelWarning = document.getElementById('channel-warning');
                if (channelWarning) {
                    if (shouldShowRecommendation && !isPreview) {
                        channelWarning.innerHTML = `
                            <div class="p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md">
                                <div class="flex items-start gap-2">
                                    <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                                    </svg>
                                    <div class="text-xs">
                                        <p class="font-medium text-amber-800 dark:text-amber-200">Release Candidate Channel</p>
                                        <p class="text-amber-700 dark:text-amber-300 mt-0.5">RC versions may contain bugs. Consider switching to stable for production use.</p>
                                    </div>
                                </div>
                            </div>
                        `;
                        channelWarning.classList.remove('hidden');
                    } else {
                        channelWarning.classList.add('hidden');
                    }
                }
                
                // Check if running a development version
                const isDevVersion = currentVersion.includes('-dev') || currentVersion.includes('+dev');
                
                const isDowngradeToStable = isCurrentRC && updateChannel === 'stable' && 
                    currentVersion !== latestVersion;
                
                // Never show updates for development versions
                if (!isDevVersion && (data.updateAvailable || isDowngradeToStable)) {
                    latestVersionElement.className = 'text-lg font-mono font-semibold text-green-600 dark:text-green-400';
                    
                    // Update status indicator
                    if (updateStatusIndicator) {
                        let statusText;
                        let statusIcon;
                        if (isDowngradeToStable) {
                            statusText = 'Switch to stable available';
                            statusIcon = '⬇️';
                        } else {
                            statusText = updateChannel === 'rc' ? 'RC update available' : 'Update available';
                            statusIcon = '🎉';
                        }
                        
                        updateStatusIndicator.innerHTML = `
                            <div class="flex items-center gap-2 p-2 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md">
                                <span class="text-lg">${statusIcon}</span>
                                <span class="text-sm font-medium text-green-800 dark:text-green-200">${statusText}</span>
                            </div>
                        `;
                    }
                    
                    // Convert server response to match GitHub API format for showUpdateDetails
                    const releaseData = {
                        tag_name: 'v' + latestVersion,
                        body: data.releaseNotes,
                        published_at: data.publishedAt,
                        html_url: data.releaseUrl,
                        assets: data.assets,
                        isDocker: data.isDocker,
                        updateChannel: updateChannel,
                        isDowngrade: isDowngradeToStable
                    };
                    
                    // Show update details
                    showUpdateDetails(releaseData);
                } else {
                    // Up to date - hide any update details
                    hideUpdateDetails();
                    
                    latestVersionElement.className = 'text-lg font-mono font-semibold text-gray-700 dark:text-gray-300';
                    
                    // Update status indicator
                    if (updateStatusIndicator) {
                        let statusText;
                        let statusIcon;
                        
                        if (isDevVersion) {
                            statusText = 'Development version';
                            statusIcon = '🚧';
                            latestVersionElement.className = 'text-lg font-mono font-semibold text-purple-600 dark:text-purple-400';
                        } else if (isCurrentRC && updateChannel === 'stable' && currentVersion === latestVersion) {
                            statusText = 'Up to date (same as stable)';
                            statusIcon = '✅';
                        } else if (isCurrentRC && updateChannel === 'stable') {
                            statusText = 'Running RC version';
                            statusIcon = '🧪';
                        } else {
                            statusText = updateChannel === 'rc' ? 'Up to date (RC channel)' : 'Up to date';
                            statusIcon = '✅';
                        }
                        
                        const bgColor = isDevVersion ? 'bg-purple-50 dark:bg-purple-900/20' : 'bg-gray-50 dark:bg-gray-800';
                        const borderColor = isDevVersion ? 'border-purple-200 dark:border-purple-800' : 'border-gray-200 dark:border-gray-700';
                        const textColor = isDevVersion ? 'text-purple-800 dark:text-purple-200' : 'text-gray-700 dark:text-gray-300';
                        
                        updateStatusIndicator.innerHTML = `
                            <div class="flex items-center gap-2 p-2 ${bgColor} border ${borderColor} rounded-md">
                                <span class="text-lg">${statusIcon}</span>
                                <span class="text-sm font-medium ${textColor}">${statusText}</span>
                            </div>
                        `;
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
            latestVersionElement.className = 'text-lg font-mono font-semibold text-red-500';
            
            // Update status indicator with error
            if (updateStatusIndicator) {
                let errorMessage = 'Failed to check for updates';
                if (error.message.includes('500')) {
                    errorMessage = 'Server error';
                } else if (error.message.includes('403') || error.message.includes('429')) {
                    errorMessage = 'Rate limited';
                } else if (error.message.includes('Failed to fetch')) {
                    errorMessage = 'Network error';
                }
                
                updateStatusIndicator.innerHTML = `
                    <div class="flex items-center gap-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
                        <span class="text-lg">❌</span>
                        <span class="text-sm font-medium text-red-800 dark:text-red-200">${errorMessage}</span>
                    </div>
                `;
            }
            
            // If it's a rate limit issue, suggest using cached data if available
            if (error.message.includes('403') || error.message.includes('429')) {
                const cacheKey = channelOverride || 'default';
                const staleCache = updateCache.get(cacheKey);
                if (staleCache) {
                    // Use cached data
                    setTimeout(() => {
                        const cachedData = staleCache.data;
                        latestVersionElement.textContent = cachedData.latestVersion || 'Unknown';
                        latestVersionElement.className = 'text-lg font-mono font-semibold text-gray-700 dark:text-gray-300';
                        if (updateStatusIndicator) {
                            updateStatusIndicator.innerHTML = `
                                <div class="flex items-center gap-2 p-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md">
                                    <span class="text-lg">⚠️</span>
                                    <span class="text-sm font-medium text-amber-800 dark:text-amber-200">Using cached data</span>
                                </div>
                            `;
                        }
                    }, 100);
                }
            }
        } finally {
            // Restore button state
            if (checkButton) {
                checkButton.disabled = false;
                checkButton.innerHTML = '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg> Check Now';
            }
        }
    }
    
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
        const updateSection = document.getElementById('update-available-section');
        if (!updateSection) return;
        
        // Store the release data for use in applyUpdate
        latestReleaseData = releaseData;
        
        const updateVersion = document.getElementById('update-version');
        const updateSummary = document.getElementById('update-summary');
        const updateInstructions = document.getElementById('update-instructions');
        const changelogContent = document.getElementById('changelog-content');
        
        if (updateVersion) {
            updateVersion.textContent = releaseData.tag_name;
        }
        
        // Generate update summary
        if (updateSummary) {
            const publishedDate = releaseData.published_at ? PulseApp.utils.formatDate(releaseData.published_at) : '';
            let summaryText = `Published ${publishedDate}`;
            
            if (releaseData.isDowngrade) {
                summaryText = 'Switch from RC to stable release • ' + summaryText;
            } else if (releaseData.updateChannel === 'rc') {
                summaryText = 'Release candidate version • ' + summaryText;
            }
            
            updateSummary.textContent = summaryText;
        }
        
        // Show deployment-specific instructions
        if (updateInstructions) {
            if (releaseData.isDocker) {
                updateInstructions.innerHTML = `
                    <div class="bg-white dark:bg-gray-800 rounded-md p-3 border border-gray-200 dark:border-gray-700">
                        <p class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-2">Docker Update Instructions:</p>
                        <pre class="bg-gray-100 dark:bg-gray-900 rounded p-2 text-xs overflow-x-auto"><code>docker pull rcourtman/pulse:${releaseData.tag_name}
docker compose up -d</code></pre>
                    </div>
                `;
            } else {
                updateInstructions.innerHTML = `
                    <button type="button" 
                            onclick="PulseApp.ui.settings.applyUpdate()" 
                            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-md flex items-center gap-2 transition-colors">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                        </svg>
                        Apply Update
                    </button>
                `;
            }
        }
        
        // Show changelog
        if (changelogContent && releaseData.body) {
            const htmlContent = releaseData.body
                .replace(/### (.*)/g, '<h4 class="font-semibold text-sm mt-3 mb-1 text-gray-800 dark:text-gray-200">$1</h4>')
                .replace(/## (.*)/g, '<h3 class="font-semibold text-base mt-3 mb-2 text-gray-800 dark:text-gray-200">$1</h3>')
                .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
                .replace(/\*(.*?)\*/g, '<em>$1</em>')
                .replace(/- (.*)/g, '<li class="ml-4 text-gray-700 dark:text-gray-300">• $1</li>')
                .replace(/\n/g, '<br>');
            
            changelogContent.innerHTML = htmlContent;
        }
        
        updateSection.classList.remove('hidden');
        
        // TODO: Implement commit differences in changelog section if needed
        // For now, skip loadVersionChanges as it references old UI elements
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
        // This function is temporarily disabled as it references old UI elements
        // TODO: Reimplement for new UI if needed
        return;
        
        const currentVersion = currentConfig.version || 'Unknown';
        const changesLoading = document.getElementById('changes-loading');
        const changesSummaryText = document.getElementById('changes-summary-text');
        const changesList = document.getElementById('changes-list');
        const changesError = document.getElementById('changes-error');
        
        if (!changesLoading || !changesSummaryText || !changesList || !changesError) {
            console.log('[Settings] Version changes elements not found in new UI, skipping');
            return;
        }
        
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
            // Skip version comparison if versions are the same
            const cleanCurrentVersion = currentVersion.replace(/^v/, '');
            const cleanTargetVersion = targetVersion.replace(/^v/, '');
            
            if (cleanCurrentVersion === cleanTargetVersion) {
                changesLoading.classList.add('hidden');
                changesSummaryText.innerHTML = '<span class="text-gray-600 dark:text-gray-300">You are already on this version.</span>';
                changesSummaryText.classList.remove('hidden');
                return;
            }
            
            const isDevVersion = cleanCurrentVersion.includes('-dev') || cleanCurrentVersion.includes('+');
            
            if (isDevVersion) {
                // For development versions, skip API comparison and show simple message
                console.log(`[Settings] Development version detected (${cleanCurrentVersion}), skipping GitHub API comparison`);
                changesLoading.classList.add('hidden');
                changesSummaryText.innerHTML = '<span class="text-blue-600 dark:text-blue-400">📦 Update available from development version to release version</span>';
                changesSummaryText.classList.remove('hidden');
                return;
            }
            
            // For RC versions, check if they exist as real tags before comparing
            // If current version is a dynamic RC (like 3.24.0-rc30), try to find the closest stable version
            let baseVersion, headVersion;
            
            const gitDescribeMatch = cleanCurrentVersion.match(/^(\d+\.\d+\.\d+(?:-rc\d+)?)-\d+-g[a-f0-9]+(?:-dirty)?$/);
            let actualCurrentVersion = cleanCurrentVersion;
            if (gitDescribeMatch) {
                actualCurrentVersion = gitDescribeMatch[1];
                console.log(`[Settings] Git describe format detected, using base tag: ${actualCurrentVersion}`);
            }
            
            if (isCurrentRC && !isTargetRC) {
                // Current is RC, target is stable - compare from target to current
                baseVersion = targetVersion;
                headVersion = actualCurrentVersion;
                
                // If current RC version looks dynamically generated (high RC number), 
                // try to find the actual base stable version for comparison
                const rcMatch = actualCurrentVersion.match(/^(\d+\.\d+\.\d+)-rc(\d+)$/);
                if (rcMatch && parseInt(rcMatch[2]) > 10) {
                    // High RC number suggests dynamic versioning, use the base version
                    const baseStableVersion = rcMatch[1];
                    console.log(`[Settings] Dynamic RC detected (${actualCurrentVersion}), using base version ${baseStableVersion} for comparison`);
                    headVersion = baseStableVersion;
                }
            } else {
                // Normal comparison
                baseVersion = actualCurrentVersion;
                headVersion = targetVersion;
            }
            
            // Strip any existing 'v' prefix to avoid double prefixes
            const cleanBaseVersion = baseVersion.replace(/^v/, '');
            const cleanHeadVersion = headVersion.replace(/^v/, '');
            
            const compareUrl = `https://api.github.com/repos/rcourtman/Pulse/compare/v${cleanBaseVersion}...v${cleanHeadVersion}`;
            console.log(`[Settings] Comparing versions: v${cleanBaseVersion} to v${cleanHeadVersion}`);
            
            const response = await fetch(compareUrl);
            if (!response.ok) {
                if (response.status === 404) {
                    // Version not found - provide a helpful fallback message
                    console.log(`[Settings] Handled 404 for version comparison: ${compareUrl} - showing fallback message`);
                    changesLoading.classList.add('hidden');
                    changesSummaryText.innerHTML = '<span class="text-amber-600 dark:text-amber-300">⚠️ Version comparison unavailable - this may be a development or pre-release version.</span>';
                    changesSummaryText.classList.remove('hidden');
                    return;
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
            logger.error('Error fetching version changes:', error);
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
            summaryParts.push(`<span class="text-purple-600 dark:text-purple-400">${categories.docs.length} documentation</span>`);
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
            const date = PulseApp.utils.formatDate(commit.commit.author.date);
            
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
                icon = '';
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
        const updateSection = document.getElementById('update-available-section');
        if (updateSection) {
            updateSection.classList.add('hidden');
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
        
        if (!latestReleaseData || !latestReleaseData.assets || latestReleaseData.assets.length === 0) {
            logger.error('No release data or assets:', { latestReleaseData, hasAssets: !!latestReleaseData?.assets, assetCount: latestReleaseData?.assets?.length });
            showMessage('No update information available. Please check for updates first.', 'error');
            return;
        }
        
        // Find the tarball asset
        const tarballAsset = latestReleaseData.assets.find(asset => 
            asset.name.endsWith('.tar.gz') && asset.name.includes('pulse')
        );
        
        
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
        PulseApp.ui.toast.confirm(
            `Update to version ${latestReleaseData.tag_name}? The application will restart automatically and refresh the page when complete.`,
            async () => {
                await _performUpdate(latestReleaseData, tarballAsset);
            }
        );
    }

    async function _performUpdate(latestReleaseData, tarballAsset) {
        
        try {
            // Hide update details and show progress
            const updateSection = document.getElementById('update-available-section');
            const updateProgress = document.getElementById('update-progress');
            const progressBar = document.getElementById('update-progress-bar');
            const progressText = document.getElementById('update-progress-text');
            
            if (updateSection) updateSection.classList.add('hidden');
            if (updateProgress) updateProgress.classList.remove('hidden');
            
            // Start the update
            await PulseApp.apiClient.post('/api/updates/apply', {
                downloadUrl: tarballAsset.downloadUrl || tarballAsset.browser_download_url
            });
            
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
                        progressText.textContent = 'Update complete! Service restarting...';
                    }
                    showMessage('Update applied successfully. Service is restarting and will refresh automatically.', 'success');
                    
                    // Wait 3 seconds to ensure service has time to start shutting down
                    setTimeout(() => {
                        // Poll health endpoint until service is back up, then refresh
                        pollHealthAndRefresh();
                    }, 3000);
                });
                
                window.socket.on('updateError', (data) => {
                    showMessage(`Update failed: ${data.error}`, 'error');
                    const updateSection = document.getElementById('update-available-section');
                    if (updateSection) updateSection.classList.remove('hidden');
                    if (updateProgress) updateProgress.classList.add('hidden');
                });
            }
            
            showMessage('Update started. Please wait...', 'info');
            
        } catch (error) {
            console.error('Error applying update:', error);
            showMessage(`Failed to apply update: ${error.message}`, 'error');
            
            // Restore UI state
            const updateSection = document.getElementById('update-available-section');
            const updateProgress = document.getElementById('update-progress');
            if (updateSection) updateSection.classList.remove('hidden');
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
                PulseApp.ui.toast.warning('Please select a VM/LXC from the dropdown first.');
                return;
            }
            
            
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
                    PulseApp.ui.toast.error('Failed to save threshold configuration: ' + result.error);
                }
            } catch (error) {
                PulseApp.ui.toast.error('Error saving threshold configuration: ' + error.message);
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
                PulseApp.ui.toast.error('Failed to load threshold configuration: ' + result.error);
            }
        } catch (error) {
            PulseApp.ui.toast.error('Error loading threshold configuration: ' + error.message);
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
                    PulseApp.ui.toast.error('Failed to toggle threshold configuration: ' + toggleResult.error);
                }
            }
        } catch (error) {
            PulseApp.ui.toast.error('Error toggling threshold configuration: ' + error.message);
        }
    }
    
    async function deleteThresholdConfiguration(endpointId, nodeId, vmid) {
        PulseApp.ui.toast.confirm(
            `Are you sure you want to delete the custom threshold configuration for ${endpointId}:${nodeId}:${vmid}?`,
            async () => {
                await _performDeleteThresholdConfiguration(endpointId, nodeId, vmid);
            }
        );
    }

    async function _performDeleteThresholdConfiguration(endpointId, nodeId, vmid) {
        
        try {
            // Get CSRF token for DELETE request
            const csrfToken = sessionStorage.getItem('csrfToken') || '';
            
            const response = await fetch(`/api/thresholds/${endpointId}/${nodeId}/${vmid}`, {
                method: 'DELETE',
                headers: {
                    'X-CSRF-Token': csrfToken
                }
            });
            
            const result = await response.json();
            
            if (result.success) {
                await loadThresholdConfigurations(); // Reload the list
            } else {
                PulseApp.ui.toast.error('Failed to delete threshold configuration: ' + result.error);
            }
        } catch (error) {
            PulseApp.ui.toast.error('Error deleting threshold configuration: ' + error.message);
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
                        PulseApp.ui.toast.error('Test email failed: ' + result.error);
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    }
                } catch (error) {
                    testBtn.textContent = '✗ Error';
                    testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                    PulseApp.ui.toast.error('Error sending test email: ' + error.message);
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
                    
                    const response = await fetch('/api/alerts/test-webhook', {
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
                        PulseApp.ui.toast.error('Test webhook failed: ' + result.error);
                        setTimeout(() => {
                            testBtn.textContent = originalText;
                            testBtn.className = 'px-3 py-1 bg-purple-600 hover:bg-purple-700 text-white text-xs font-medium rounded transition-colors';
                        }, 3000);
                    }
                } catch (error) {
                    testBtn.textContent = '✗ Error';
                    testBtn.className = 'px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors';
                    PulseApp.ui.toast.error('Error sending test webhook: ' + error.message);
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
    function sanitizeUrl(url) {
        if (!url) return url;
        
        // Remove protocol if present
        let sanitized = url.replace(/^https?:\/\//, '');
        
        // Replace IP addresses
        sanitized = sanitized.replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, '[IP-ADDRESS]');
        
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
        
        // Clear stored release data when switching channels to prevent cross-channel confusion
        latestReleaseData = null;
        hideUpdateDetails();
        
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
        // Update hidden input
        const hiddenInput = document.querySelector('input[name="UPDATE_CHANNEL"]');
        if (hiddenInput) {
            hiddenInput.value = targetChannel;
        }
        
        // Update button states
        const channelButtons = document.querySelectorAll('.channel-btn');
        channelButtons.forEach(btn => {
            const btnChannel = btn.getAttribute('data-channel');
            if (btnChannel === targetChannel) {
                // Active state
                btn.className = 'channel-btn px-3 py-1 text-xs font-medium rounded-md transition-colors';
                if (targetChannel === 'rc') {
                    btn.classList.add('bg-amber-100', 'dark:bg-amber-900', 'text-amber-700', 'dark:text-amber-300', 'border', 'border-amber-300', 'dark:border-amber-700');
                } else {
                    btn.classList.add('bg-blue-100', 'dark:bg-blue-900', 'text-blue-700', 'dark:text-blue-300', 'border', 'border-blue-300', 'dark:border-blue-700');
                }
            } else {
                // Inactive state
                btn.className = 'channel-btn px-3 py-1 text-xs font-medium rounded-md transition-colors bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-600';
            }
        });
        
        // Store the channel preference
        if (currentConfig.advanced) {
            currentConfig.advanced.updateChannel = targetChannel;
        } else {
            currentConfig.advanced = { updateChannel: targetChannel };
        }
        
        // Clear stored release data when switching channels
        latestReleaseData = null;
        hideUpdateDetails();
        
        // Check for updates with the new channel
        checkLatestVersion(targetChannel);
        
        // Show/hide RC warning
        onUpdateChannelChange(targetChannel);
    }
    
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

    function clearUpdateCache() {
        updateCache.clear();
    }



    async function testEmailConfiguration() {
        const formData = collectFormData();
        const testButton = document.getElementById('test-email-btn');
        
        if (!testButton) return;
        
        const originalText = testButton.textContent;
        testButton.disabled = true;
        testButton.textContent = 'Testing...';
        
        try {
            let testConfig;
            
            if (formData.EMAIL_PROVIDER === 'sendgrid') {
                testConfig = {
                    emailProvider: 'sendgrid',
                    sendgridApiKey: formData.SENDGRID_API_KEY,
                    from: formData.SENDGRID_FROM_EMAIL,
                    to: formData.ALERT_TO_EMAIL || formData.ALERT_TO_EMAIL_SENDGRID
                };
            } else {
                testConfig = {
                    emailProvider: 'smtp',
                    from: formData.ALERT_FROM_EMAIL,
                    to: formData.ALERT_TO_EMAIL,
                    host: formData.SMTP_HOST,
                    port: parseInt(formData.SMTP_PORT) || 587,
                    user: formData.SMTP_USER,
                    pass: formData.SMTP_PASS === '***REDACTED***' ? '' : formData.SMTP_PASS,
                    secure: formData.SMTP_SECURE === 'on'
                };
            }
            
            // If password is missing, add empty string to signal server to use stored password
            if (testConfig.emailProvider === 'smtp' && !testConfig.pass) {
                testConfig.pass = '';
            }
            
            const response = await PulseApp.apiClient.post('/api/alerts/test-email', testConfig);
            
            if (response.success) {
                PulseApp.ui.toast.success('Test email sent successfully!');
            } else {
                PulseApp.ui.toast.error(`Email test failed: ${response.error}`);
            }
        } catch (error) {
            PulseApp.ui.toast.error(error.message || 'Failed to test email');
        } finally {
            testButton.disabled = false;
            testButton.textContent = originalText;
        }
    }

    async function testWebhookConfiguration() {
        console.log('[Settings] Testing webhook configuration...');
        const formData = collectFormData();
        const testButton = document.getElementById('test-webhook-btn');
        
        if (!testButton) {
            logger.error('Test button not found');
            return;
        }
        
        const originalText = testButton.textContent;
        testButton.disabled = true;
        testButton.textContent = 'Testing...';
        
        try {
            // First check webhook status
            console.log('[Settings] Checking webhook status...');
            const statusResponse = await PulseApp.apiClient.get('/api/webhook-status');
            console.log('[Settings] Webhook status:', statusResponse);
            
            
            if (!formData.WEBHOOK_URL) {
                PulseApp.ui.toast.error('Please enter a webhook URL');
                testButton.disabled = false;
                testButton.textContent = originalText;
                return;
            }
            
            // Send test webhook
            console.log('[Settings] Sending test webhook to:', formData.WEBHOOK_URL);
            const response = await PulseApp.apiClient.post('/api/alerts/test-webhook', {
                url: formData.WEBHOOK_URL
            });
            console.log('[Settings] Test webhook response:', response);
            
            if (response.success) {
                PulseApp.ui.toast.success('Webhook test sent successfully!');
                
                // Show additional status info if there are cooldowns
                if (statusResponse.activeCooldowns && statusResponse.activeCooldowns.length > 0) {
                    const cooldown = statusResponse.activeCooldowns[0];
                    PulseApp.ui.toast.showToast(`Note: Some alerts may be on cooldown for ${cooldown.remainingMinutes} more minutes`, 'info');
                }
            } else {
                PulseApp.ui.toast.error(`Webhook test failed: ${response.error}`);
            }
        } catch (error) {
            logger.error('Test webhook error:', error);
            PulseApp.ui.toast.error(error.message || 'Failed to test webhook');
        } finally {
            testButton.disabled = false;
            testButton.textContent = originalText;
        }
    }
    
    async function updateWebhookStatus() {
        try {
            const statusResponse = await PulseApp.apiClient.get('/api/webhook-status');
            const statusIndicator = document.getElementById('webhook-status-indicator');
            const cooldownInfo = document.getElementById('webhook-cooldown-info');
            
            if (!statusIndicator) return;
            
            if (!statusResponse.enabled) {
                statusIndicator.innerHTML = '<span class="text-red-600 dark:text-red-400">🔴 Disabled</span>';
                if (cooldownInfo) cooldownInfo.textContent = 'Webhooks are globally disabled';
            } else if (statusResponse.configured) {
                statusIndicator.innerHTML = '<span class="text-green-600 dark:text-green-400">🟢 Enabled</span>';
                
                // Show cooldown info if any
                if (cooldownInfo && statusResponse.activeCooldowns && statusResponse.activeCooldowns.length > 0) {
                    const cooldown = statusResponse.activeCooldowns[0];
                    cooldownInfo.textContent = `Cooldown active: ${cooldown.remainingMinutes}m remaining`;
                } else if (cooldownInfo) {
                    cooldownInfo.textContent = `Cooldown: ${statusResponse.cooldownConfig.defaultCooldownMinutes}m between alerts`;
                }
            } else {
                statusIndicator.innerHTML = '<span class="text-yellow-600 dark:text-yellow-400">🟡 Not configured</span>';
                if (cooldownInfo) cooldownInfo.textContent = '';
            }
        } catch (error) {
            console.error('Failed to update webhook status:', error);
        }
    }
    
    async function pollHealthAndRefresh() {
        console.log('[Update] Starting health endpoint polling...');
        const maxAttempts = 60; // 60 attempts = 1 minute max
        const pollInterval = 1000; // 1 second between attempts
        let attempts = 0;
        let lastError = null;
        
        // Show progress in the update progress section if available
        const progressText = document.getElementById('update-progress-text');
        const progressBar = document.getElementById('update-progress-bar');
        
        // Show clear message about what's happening
        showMessage('🔄 Waiting for service to restart... The page will refresh automatically when ready.', 'info');
        
        if (progressText) {
            progressText.textContent = 'Waiting for service to restart...';
        }
        
        const checkHealth = async () => {
            attempts++;
            
            // Update progress text with attempts
            if (progressText) {
                progressText.textContent = `Checking service status... (${attempts}/${maxAttempts})`;
            }
            
            try {
                const response = await fetch('/api/health', {
                    method: 'GET',
                    cache: 'no-cache'
                });
                
                if (response.ok) {
                    // Health check passed, but wait a bit more to ensure full initialization
                    console.log('[Update] Health check passed, waiting for full initialization...');
                    showMessage('✅ Service is ready! Refreshing page in 3 seconds...', 'success');
                    
                    if (progressText) {
                        progressText.textContent = 'Service ready! Refreshing page...';
                    }
                    
                    if (progressBar) {
                        progressBar.style.width = '100%';
                    }
                    
                    // Show countdown to make it clear when refresh will happen
                    let countdown = 3;
                    const countdownInterval = setInterval(() => {
                        countdown--;
                        if (progressText) {
                            progressText.textContent = `Service ready! Refreshing page in ${countdown}...`;
                        }
                        if (countdown <= 0) {
                            clearInterval(countdownInterval);
                            console.log('[Update] Refreshing page...');
                            window.location.reload();
                        }
                    }, 1000);
                    
                    return true;
                }
                lastError = `Status: ${response.status}`;
            } catch (error) {
                lastError = error.message;
                console.log(`[Update] Health check attempt ${attempts}/${maxAttempts} failed:`, lastError);
            }
            
            if (attempts >= maxAttempts) {
                console.error('[Update] Max health check attempts reached');
                showMessage('⚠️ Service restart is taking longer than expected. You may need to refresh the page manually.', 'warning');
                
                if (progressText) {
                    progressText.textContent = 'Restart taking longer than expected. You may need to refresh manually.';
                }
                
                return false;
            }
            
            // Continue polling
            setTimeout(checkHealth, pollInterval);
        };
        
        // Start polling after a short initial delay
        setTimeout(checkHealth, 2000); // Wait 2 seconds before first check
    }
    
    function setEmailProvider(provider) {
        const sendgridBtn = document.getElementById('email-provider-sendgrid');
        const smtpBtn = document.getElementById('email-provider-smtp');
        const sendgridConfig = document.getElementById('sendgrid-config');
        const smtpConfig = document.getElementById('smtp-config');
        const providerInput = document.querySelector('input[name="EMAIL_PROVIDER"]');
        
        if (provider === 'sendgrid') {
            sendgridBtn.className = 'px-4 py-2 text-sm font-medium rounded-md border bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-700 dark:text-blue-300';
            smtpBtn.className = 'px-4 py-2 text-sm font-medium rounded-md border bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700';
            sendgridConfig.classList.remove('hidden');
            smtpConfig.classList.add('hidden');
        } else {
            smtpBtn.className = 'px-4 py-2 text-sm font-medium rounded-md border bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-700 dark:text-blue-300';
            sendgridBtn.className = 'px-4 py-2 text-sm font-medium rounded-md border bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700';
            smtpConfig.classList.remove('hidden');
            sendgridConfig.classList.add('hidden');
        }
        
        if (providerInput) {
            providerInput.value = provider;
        }
        
        // Trigger change tracking
        trackChanges();
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
        validateHost,
        validatePort,
        validateTokenId,
        generateSessionSecret,
        updateSecurityOptionsVisibility,
        updateTrustProxyVisibility,
        updateEmbedOriginVisibility,
        onUpdateChannelChange,
        setEmailProvider,
        switchToRecommendedChannel,
        switchToChannel,
        acknowledgeStableChoice,
        proceedWithStableSwitch,
        clearUpdateCache,
        getCurrentConfig: () => currentConfig,
        testEmailConfiguration,
        testWebhookConfiguration,
        updateWebhookStatus
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.settings.init);
} else {
    PulseApp.ui.settings.init();
}