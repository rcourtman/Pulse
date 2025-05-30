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
            // Read existing .env file to preserve other settings
            const existingConfig = await this.readEnvFile();
            
            // Update with new values
            if (config.proxmox) {
                existingConfig.PROXMOX_HOST = config.proxmox.host;
                existingConfig.PROXMOX_PORT = config.proxmox.port || '8006';
                existingConfig.PROXMOX_TOKEN_ID = config.proxmox.tokenId;
                existingConfig.PROXMOX_TOKEN_SECRET = config.proxmox.tokenSecret;
                // Always allow self-signed certificates by default for Proxmox
                existingConfig.PROXMOX_ALLOW_SELF_SIGNED_CERT = 'true';
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
            
            // Write back to .env file
            await this.writeEnvFile(existingConfig);
            
            // Reload configuration in the application
            await this.reloadConfiguration();
            
            return { success: true };
        } catch (error) {
            console.error('Error saving configuration:', error);
            throw error;
        }
    }

    /**
     * Test configuration by attempting to connect
     */
    async testConfig(config) {
        try {
            // Create temporary endpoint configuration
            const testEndpoints = [{
                id: 'test-primary',
                name: 'Test Primary',
                host: config.proxmox.host,
                port: parseInt(config.proxmox.port) || 8006,
                tokenId: config.proxmox.tokenId,
                tokenSecret: config.proxmox.tokenSecret,
                enabled: true,
                allowSelfSignedCerts: true  // Allow self-signed certificates for testing
            }];
            
            const testPbsConfigs = config.pbs ? [{
                id: 'test-pbs',
                name: 'Test PBS',
                host: config.pbs.host,
                port: parseInt(config.pbs.port) || 8007,
                tokenId: config.pbs.tokenId,
                tokenSecret: config.pbs.tokenSecret,
                allowSelfSignedCerts: true  // Allow self-signed certificates for testing
            }] : [];
            
            // Try to initialize API clients with test config
            const { apiClients, pbsApiClients } = await initializeApiClients(testEndpoints, testPbsConfigs);
            
            // Try a simple API call to verify connection
            const testClient = apiClients['test-primary'];
            if (testClient) {
                await testClient.client.get('/nodes');
            }
            
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
        
        await fs.writeFile(this.envPath, lines.join('\n'), 'utf8');
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
                await this.saveConfig(req.body);
                res.json({ success: true });
            } catch (error) {
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
    }
}

module.exports = ConfigApi;