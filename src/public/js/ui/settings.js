PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.settings = (() => {
    let isInitialized = false;

    function init() {
        if (isInitialized) return;
        
        console.log('[Settings] Initializing settings module...');
        
        // Set up form submission handler
        const form = document.getElementById('settings-config-form');
        if (form) {
            form.addEventListener('submit', handleFormSubmission);
        }
        
        isInitialized = true;
        console.log('[Settings] Settings module initialized');
    }

    function load() {
        console.log('[Settings] Loading current configuration...');
        loadCurrentConfig();
    }

    async function loadCurrentConfig() {
        try {
            const response = await fetch('/api/config');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const config = await response.json();
            console.log('[Settings] Current config loaded:', config);
            
            // Populate Proxmox fields
            if (config.proxmox) {
                document.getElementById('settings-proxmox-host').value = config.proxmox.host || '';
                document.getElementById('settings-proxmox-port').value = config.proxmox.port || '';
                document.getElementById('settings-proxmox-token-id').value = config.proxmox.tokenId || '';
                // Don't populate the secret for security
                document.getElementById('settings-proxmox-token-secret').placeholder = 'Enter new secret or leave blank to keep current';
            }
            
        } catch (error) {
            console.error('[Settings] Failed to load current configuration:', error);
            showError('Failed to load current configuration: ' + error.message);
        }
    }

    async function handleFormSubmission(e) {
        e.preventDefault();
        
        const formData = new FormData(e.target);
        const config = {
            proxmox: {
                host: formData.get('proxmox-host'),
                port: formData.get('proxmox-port') || '8006',
                tokenId: formData.get('proxmox-token-id'),
                tokenSecret: formData.get('proxmox-token-secret')
            }
        };

        // Validate required fields
        if (!config.proxmox.host || !config.proxmox.tokenId) {
            showError('Please fill in all required fields (Host and Token ID)');
            return;
        }

        // If token secret is empty, don't include it (keep existing)
        if (!config.proxmox.tokenSecret) {
            delete config.proxmox.tokenSecret;
        }

        hideMessages();
        const button = document.getElementById('settings-save-button');
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = 'Saving...';

        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                showSuccess('Configuration saved successfully!');
                // Reload the current config to show updated values
                setTimeout(() => {
                    loadCurrentConfig();
                }, 1000);
            } else {
                showError(result.error || 'Failed to save configuration');
            }
        } catch (error) {
            showError('Failed to save configuration: ' + error.message);
        } finally {
            button.disabled = false;
            button.textContent = originalText;
        }
    }

    async function testConnection() {
        const formData = new FormData(document.getElementById('settings-config-form'));
        const config = {
            proxmox: {
                host: formData.get('proxmox-host'),
                port: formData.get('proxmox-port') || '8006',
                tokenId: formData.get('proxmox-token-id'),
                tokenSecret: formData.get('proxmox-token-secret')
            }
        };

        // Validate required fields
        if (!config.proxmox.host || !config.proxmox.tokenId || !config.proxmox.tokenSecret) {
            showError('Please fill in all required fields to test connection');
            return;
        }

        hideMessages();
        const button = event.target;
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = 'Testing...';

        try {
            const response = await fetch('/api/config/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });

            const result = await response.json();
            
            if (response.ok && result.success) {
                showSuccess('Connection test successful!');
                setTimeout(() => hideMessages(), 3000);
            } else {
                showError(result.error || 'Connection test failed');
            }
        } catch (error) {
            showError('Failed to test connection: ' + error.message);
        } finally {
            button.disabled = false;
            button.textContent = originalText;
        }
    }

    function togglePasswordVisibility(inputId) {
        const input = document.getElementById(inputId);
        if (input) {
            input.type = input.type === 'password' ? 'text' : 'password';
        }
    }

    function showError(message) {
        const errorDiv = document.getElementById('settings-error-message');
        const errorText = document.getElementById('settings-error-text');
        const successDiv = document.getElementById('settings-success-message');
        
        if (errorText && errorDiv && successDiv) {
            errorText.textContent = message;
            errorDiv.classList.remove('hidden');
            successDiv.classList.add('hidden');
        }
    }

    function showSuccess(message) {
        const errorDiv = document.getElementById('settings-error-message');
        const successDiv = document.getElementById('settings-success-message');
        const successText = document.getElementById('settings-success-text');
        
        if (successText && successDiv && errorDiv) {
            successText.textContent = message;
            errorDiv.classList.add('hidden');
            successDiv.classList.remove('hidden');
        }
    }

    function hideMessages() {
        const errorDiv = document.getElementById('settings-error-message');
        const successDiv = document.getElementById('settings-success-message');
        
        if (errorDiv) errorDiv.classList.add('hidden');
        if (successDiv) successDiv.classList.add('hidden');
    }

    // Public API
    return {
        init,
        load,
        testConnection,
        togglePasswordVisibility
    };
})();