const fs = require('fs').promises;
const path = require('path');
const { loadConfiguration } = require('./configLoader');
const { initializeApiClients } = require('./apiClients');

class ConfigApi {
    constructor() {
        this.envPath = path.join(__dirname, '../.env');
    }

    /**
     * Get current configuration (without secrets)
     */
    async getConfig() {
        try {
            const config = await this.readEnvFile();
            
            return {
                proxmox: config.PROXMOX_HOST ? {
                    host: config.PROXMOX_HOST,
                    port: config.PROXMOX_PORT || '8006',
                    tokenId: config.PROXMOX_TOKEN_ID,
                    nodeName: config.PROXMOX_NODE_NAME,
                    enabled: config.PROXMOX_ENABLED !== 'false',
                    // Don't send the secret
                } : null,
                pbs: config.PBS_HOST ? {
                    host: config.PBS_HOST,
                    port: config.PBS_PORT || '8007',
                    tokenId: config.PBS_TOKEN_ID,
                    nodeName: config.PBS_NODE_NAME,
                    // Don't send the secret
                } : null,
                advanced: {
                    metricInterval: config.PULSE_METRIC_INTERVAL_MS,
                    discoveryInterval: config.PULSE_DISCOVERY_INTERVAL_MS,
                    alerts: {
                        cpu: {
                            enabled: config.ALERT_CPU_ENABLED !== 'false',
                            threshold: config.ALERT_CPU_THRESHOLD
                        },
                        memory: {
                            enabled: config.ALERT_MEMORY_ENABLED !== 'false',
                            threshold: config.ALERT_MEMORY_THRESHOLD
                        },
                        disk: {
                            enabled: config.ALERT_DISK_ENABLED !== 'false',
                            threshold: config.ALERT_DISK_THRESHOLD
                        },
                        down: {
                            enabled: config.ALERT_DOWN_ENABLED !== 'false'
                        }
                    }
                }
            };
        } catch (error) {
            console.error('Error reading configuration:', error);
            return { proxmox: null, pbs: null, advanced: {} };
        }
    }

    /**
     * Save configuration to .env file
     */
    async saveConfig(config) {
        try {
            console.log('[ConfigApi.saveConfig] Called with:', JSON.stringify(config, null, 2));
            console.log('[ConfigApi.saveConfig] .env path:', this.envPath);
            
            // Read existing .env file to preserve other settings
            const existingConfig = await this.readEnvFile();
            console.log('[ConfigApi.saveConfig] Existing config keys:', Object.keys(existingConfig));
            
            // Handle both old structured format and new raw .env variable format
            if (config.proxmox || config.pbs || config.advanced) {
                // Old structured format
                this.handleStructuredConfig(config, existingConfig);
            } else {
                // New raw .env variable format from settings form
                this.handleRawEnvConfig(config, existingConfig);
            }
            
            // Write back to .env file
            console.log('[ConfigApi.saveConfig] Writing config with keys:', Object.keys(existingConfig));
            await this.writeEnvFile(existingConfig);
            console.log('[ConfigApi.saveConfig] .env file written successfully');
            
            // Reload configuration in the application
            console.log('[ConfigApi.saveConfig] Reloading configuration...');
            await this.reloadConfiguration();
            console.log('[ConfigApi.saveConfig] Configuration reloaded successfully');
            
            return { success: true };
        } catch (error) {
            console.error('Error saving configuration:', error);
            throw error;
        }
    }

    /**
     * Handle structured configuration format (old setup flow)
     */
    handleStructuredConfig(config, existingConfig) {
        // Update with new values
        if (config.proxmox) {
            console.log('[ConfigApi.saveConfig] Updating Proxmox config');
            existingConfig.PROXMOX_HOST = config.proxmox.host;
            existingConfig.PROXMOX_PORT = config.proxmox.port || '8006';
            existingConfig.PROXMOX_TOKEN_ID = config.proxmox.tokenId;
            
            // Only update token secret if provided (allows keeping existing secret)
            if (config.proxmox.tokenSecret) {
                existingConfig.PROXMOX_TOKEN_SECRET = config.proxmox.tokenSecret;
            }
            
            // Always allow self-signed certificates by default for Proxmox
            existingConfig.PROXMOX_ALLOW_SELF_SIGNED_CERT = 'true';
        } else {
            console.log('[ConfigApi.saveConfig] No Proxmox config provided');
        }
        
        if (config.pbs) {
            existingConfig.PBS_HOST = config.pbs.host;
            existingConfig.PBS_PORT = config.pbs.port || '8007';
            existingConfig.PBS_TOKEN_ID = config.pbs.tokenId;
            existingConfig.PBS_TOKEN_SECRET = config.pbs.tokenSecret;
            if (config.pbs.nodeName) {
                existingConfig.PBS_NODE_NAME = config.pbs.nodeName;
            }
            // Always allow self-signed certificates by default for PBS
            existingConfig.PBS_ALLOW_SELF_SIGNED_CERT = 'true';
        }
        
        // Add advanced settings
        if (config.advanced) {
            // Service intervals
            if (config.advanced.metricInterval) {
                existingConfig.PULSE_METRIC_INTERVAL_MS = config.advanced.metricInterval;
            }
            if (config.advanced.discoveryInterval) {
                existingConfig.PULSE_DISCOVERY_INTERVAL_MS = config.advanced.discoveryInterval;
            }
            
            // Alert settings
            if (config.advanced.alerts) {
                const alerts = config.advanced.alerts;
                if (alerts.cpu) {
                    existingConfig.ALERT_CPU_ENABLED = alerts.cpu.enabled ? 'true' : 'false';
                    if (alerts.cpu.threshold) {
                        existingConfig.ALERT_CPU_THRESHOLD = alerts.cpu.threshold;
                    }
                }
                if (alerts.memory) {
                    existingConfig.ALERT_MEMORY_ENABLED = alerts.memory.enabled ? 'true' : 'false';
                    if (alerts.memory.threshold) {
                        existingConfig.ALERT_MEMORY_THRESHOLD = alerts.memory.threshold;
                    }
                }
                if (alerts.disk) {
                    existingConfig.ALERT_DISK_ENABLED = alerts.disk.enabled ? 'true' : 'false';
                    if (alerts.disk.threshold) {
                        existingConfig.ALERT_DISK_THRESHOLD = alerts.disk.threshold;
                    }
                }
                if (alerts.down) {
                    existingConfig.ALERT_DOWN_ENABLED = alerts.down.enabled ? 'true' : 'false';
                }
            }
        }
    }

    /**
     * Handle raw .env variable format (new settings form)
     */
    handleRawEnvConfig(config, existingConfig) {
        console.log('[ConfigApi.saveConfig] Processing raw .env variable format');
        
        // Directly update existing config with new values
        Object.entries(config).forEach(([key, value]) => {
            if (value !== undefined && value !== '') {
                console.log(`[ConfigApi.saveConfig] Setting ${key} = ${value}`);
                existingConfig[key] = value;
            }
        });
        
        // Set default self-signed cert allowance for any Proxmox/PBS endpoints
        Object.keys(existingConfig).forEach(key => {
            if (key.startsWith('PROXMOX_HOST') && existingConfig[key]) {
                const suffix = key.replace('PROXMOX_HOST', '');
                const certKey = `PROXMOX_ALLOW_SELF_SIGNED_CERT${suffix}`;
                if (!existingConfig[certKey]) {
                    existingConfig[certKey] = 'true';
                }
            }
            if (key.startsWith('PBS_HOST') && existingConfig[key]) {
                const suffix = key.replace('PBS_HOST', '');
                const certKey = `PBS_ALLOW_SELF_SIGNED_CERT${suffix}`;
                if (!existingConfig[certKey]) {
                    existingConfig[certKey] = 'true';
                }
            }
        });
    }

    /**
     * Test configuration by attempting to connect
     */
    async testConfig(config) {
        try {
            console.log('[ConfigApi.testConfig] Testing config:', JSON.stringify(config, null, 2));
            
            // Handle both old structured format and new raw .env format
            let proxmoxHost, proxmoxPort, proxmoxTokenId, proxmoxTokenSecret;
            let pbsHost, pbsPort, pbsTokenId, pbsTokenSecret;
            
            if (config.proxmox) {
                // Old structured format
                proxmoxHost = config.proxmox.host;
                proxmoxPort = config.proxmox.port;
                proxmoxTokenId = config.proxmox.tokenId;
                proxmoxTokenSecret = config.proxmox.tokenSecret;
                
                if (config.pbs) {
                    pbsHost = config.pbs.host;
                    pbsPort = config.pbs.port;
                    pbsTokenId = config.pbs.tokenId;
                    pbsTokenSecret = config.pbs.tokenSecret;
                }
            } else {
                // New raw .env format
                proxmoxHost = config.PROXMOX_HOST;
                proxmoxPort = config.PROXMOX_PORT;
                proxmoxTokenId = config.PROXMOX_TOKEN_ID;
                proxmoxTokenSecret = config.PROXMOX_TOKEN_SECRET;
                
                pbsHost = config.PBS_HOST;
                pbsPort = config.PBS_PORT;
                pbsTokenId = config.PBS_TOKEN_ID;
                pbsTokenSecret = config.PBS_TOKEN_SECRET;
            }
            
            const testEndpoints = [];
            const testPbsConfigs = [];
            
            // Test Proxmox endpoint if configured
            if (proxmoxHost && proxmoxTokenId && proxmoxTokenSecret) {
                testEndpoints.push({
                    id: 'test-primary',
                    name: 'Test Primary',
                    host: proxmoxHost,
                    port: parseInt(proxmoxPort) || 8006,
                    tokenId: proxmoxTokenId,
                    tokenSecret: proxmoxTokenSecret,
                    enabled: true,
                    allowSelfSignedCerts: true
                });
            }
            
            // Test PBS endpoint if configured
            if (pbsHost && pbsTokenId && pbsTokenSecret) {
                testPbsConfigs.push({
                    id: 'test-pbs',
                    name: 'Test PBS',
                    host: pbsHost,
                    port: parseInt(pbsPort) || 8007,
                    tokenId: pbsTokenId,
                    tokenSecret: pbsTokenSecret,
                    allowSelfSignedCerts: true
                });
            }
            
            if (testEndpoints.length === 0) {
                return {
                    success: false,
                    error: 'No Proxmox server configured to test'
                };
            }
            
            // Try to initialize API clients with test config
            const { apiClients, pbsApiClients } = await initializeApiClients(testEndpoints, testPbsConfigs);
            
            // Try a simple API call to verify connection
            const testClient = apiClients['test-primary'];
            if (testClient) {
                await testClient.client.get('/nodes');
            }
            
            console.log('[ConfigApi.testConfig] Connection test successful');
            return { success: true };
        } catch (error) {
            console.error('Configuration test failed:', error);
            return { 
                success: false, 
                error: error.message || 'Failed to connect to Proxmox server'
            };
        }
    }

    /**
     * Read .env file and parse it
     */
    async readEnvFile() {
        try {
            const content = await fs.readFile(this.envPath, 'utf8');
            const config = {};
            
            content.split('\n').forEach(line => {
                const trimmedLine = line.trim();
                if (trimmedLine && !trimmedLine.startsWith('#')) {
                    const [key, ...valueParts] = trimmedLine.split('=');
                    if (key) {
                        // Handle values that might contain = signs
                        let value = valueParts.join('=').trim();
                        // Remove quotes if present
                        if ((value.startsWith('"') && value.endsWith('"')) || 
                            (value.startsWith("'") && value.endsWith("'"))) {
                            value = value.slice(1, -1);
                        }
                        config[key.trim()] = value;
                    }
                }
            });
            
            return config;
        } catch (error) {
            if (error.code === 'ENOENT') {
                // .env file doesn't exist yet
                return {};
            }
            throw error;
        }
    }

    /**
     * Write configuration back to .env file
     */
    async writeEnvFile(config) {
        const lines = [];
        
        // Add header
        lines.push('# Pulse Configuration');
        lines.push('# Generated by Pulse Web Configuration');
        lines.push('');
        
        // Group related settings
        const groups = {
            'Proxmox VE Settings': ['PROXMOX_HOST', 'PROXMOX_PORT', 'PROXMOX_TOKEN_ID', 'PROXMOX_TOKEN_SECRET', 'PROXMOX_ALLOW_SELF_SIGNED_CERT'],
            'Proxmox Backup Server Settings': ['PBS_HOST', 'PBS_PORT', 'PBS_TOKEN_ID', 'PBS_TOKEN_SECRET', 'PBS_NODE_NAME', 'PBS_ALLOW_SELF_SIGNED_CERT'],
            'Pulse Service Settings': ['PULSE_METRIC_INTERVAL_MS', 'PULSE_DISCOVERY_INTERVAL_MS'],
            'Alert System Configuration': [
                'ALERT_CPU_ENABLED', 'ALERT_CPU_THRESHOLD', 'ALERT_CPU_DURATION',
                'ALERT_MEMORY_ENABLED', 'ALERT_MEMORY_THRESHOLD', 'ALERT_MEMORY_DURATION',
                'ALERT_DISK_ENABLED', 'ALERT_DISK_THRESHOLD', 'ALERT_DISK_DURATION',
                'ALERT_DOWN_ENABLED', 'ALERT_DOWN_DURATION'
            ],
            'Other Settings': [] // Will contain all other keys
        };
        
        // Find other keys not in predefined groups
        Object.keys(config).forEach(key => {
            let found = false;
            Object.values(groups).forEach(groupKeys => {
                if (groupKeys.includes(key)) found = true;
            });
            if (!found && key !== '') {
                groups['Other Settings'].push(key);
            }
        });
        
        // Write each group
        Object.entries(groups).forEach(([groupName, keys]) => {
            if (keys.length > 0 && keys.some(key => config[key])) {
                lines.push(`# ${groupName}`);
                keys.forEach(key => {
                    if (config[key] !== undefined && config[key] !== '') {
                        const value = config[key];
                        // Quote values that contain spaces or special characters
                        const needsQuotes = value.includes(' ') || value.includes('#') || value.includes('=');
                        lines.push(`${key}=${needsQuotes ? `"${value}"` : value}`);
                    }
                });
                lines.push('');
            }
        });
        
        try {
            await fs.writeFile(this.envPath, lines.join('\n'), 'utf8');
            console.log(`[ConfigApi.writeEnvFile] Successfully wrote ${lines.length} lines to ${this.envPath}`);
        } catch (writeError) {
            console.error('[ConfigApi.writeEnvFile] Error writing file:', writeError);
            throw writeError;
        }
    }

    /**
     * Reload configuration without restarting the server
     */
    async reloadConfiguration() {
        try {
            // Clear the require cache for dotenv
            delete require.cache[require.resolve('dotenv')];
            
            // Reload environment variables
            require('dotenv').config();
            
            // Reload configuration
            const { endpoints, pbsConfigs, isConfigPlaceholder } = loadConfiguration();
            
            // Get state manager instance
            const stateManager = require('./state');
            
            // Update configuration status
            stateManager.setConfigPlaceholderStatus(isConfigPlaceholder);
            stateManager.setEndpointConfigurations(endpoints, pbsConfigs);
            
            // Reinitialize API clients
            const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);
            
            // Update global references
            if (global.pulseApiClients) {
                global.pulseApiClients.apiClients = apiClients;
                global.pulseApiClients.pbsApiClients = pbsApiClients;
            }
            
            // Update global config placeholder status
            if (global.pulseConfigStatus) {
                global.pulseConfigStatus.isPlaceholder = isConfigPlaceholder;
            }
            
            // Update last reload time to prevent file watcher from triggering
            if (global.lastReloadTime !== undefined) {
                global.lastReloadTime = Date.now();
            }
            
            // Trigger a discovery cycle if we have endpoints configured
            if (endpoints.length > 0) {
                console.log('Triggering discovery cycle after configuration reload...');
                // Import and call runDiscoveryCycle if available
                if (global.runDiscoveryCycle && typeof global.runDiscoveryCycle === 'function') {
                    setTimeout(() => {
                        global.runDiscoveryCycle();
                    }, 1000); // Give a moment for everything to settle
                }
            }
            
            console.log('Configuration reloaded successfully');
            return true;
        } catch (error) {
            console.error('Error reloading configuration:', error);
            throw error;
        }
    }

    /**
     * Set up API routes
     */
    setupRoutes(app) {
        // Get current configuration
        app.get('/api/config', async (req, res) => {
            try {
                const config = await this.getConfig();
                res.json(config);
            } catch (error) {
                res.status(500).json({ error: 'Failed to read configuration' });
            }
        });

        // Save configuration
        app.post('/api/config', async (req, res) => {
            try {
                console.log('[API /api/config] POST received with body:', JSON.stringify(req.body, null, 2));
                const result = await this.saveConfig(req.body);
                console.log('[API /api/config] Save result:', result);
                res.json({ success: true });
            } catch (error) {
                console.error('[API /api/config] Error:', error);
                res.status(500).json({ 
                    success: false, 
                    error: error.message || 'Failed to save configuration' 
                });
            }
        });

        // Test configuration
        app.post('/api/config/test', async (req, res) => {
            try {
                const result = await this.testConfig(req.body);
                res.json(result);
            } catch (error) {
                res.status(500).json({ 
                    success: false, 
                    error: error.message || 'Failed to test configuration' 
                });
            }
        });

        // Reload configuration
        app.post('/api/config/reload', async (req, res) => {
            try {
                await this.reloadConfiguration();
                res.json({ success: true });
            } catch (error) {
                res.status(500).json({ 
                    success: false, 
                    error: error.message || 'Failed to reload configuration' 
                });
            }
        });
        
        // Debug endpoint to check .env file
        app.get('/api/config/debug', async (req, res) => {
            try {
                const fs = require('fs');
                const envExists = fs.existsSync(this.envPath);
                const envContent = envExists ? await fs.promises.readFile(this.envPath, 'utf8') : 'File does not exist';
                res.json({ 
                    path: this.envPath,
                    exists: envExists,
                    content: envContent.substring(0, 500) + '...' // First 500 chars
                });
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });
    }
}

module.exports = ConfigApi;