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
                    // Don't send the secret
                } : null
            };
        } catch (error) {
            console.error('Error reading configuration:', error);
            return { proxmox: null, pbs: null };
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
            }
            
            if (config.pbs) {
                existingConfig.PBS_HOST = config.pbs.host;
                existingConfig.PBS_PORT = config.pbs.port || '8007';
                existingConfig.PBS_TOKEN_ID = config.pbs.tokenId;
                existingConfig.PBS_TOKEN_SECRET = config.pbs.tokenSecret;
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
                enabled: true
            }];
            
            const testPbsConfigs = config.pbs ? [{
                id: 'test-pbs',
                name: 'Test PBS',
                host: config.pbs.host,
                port: parseInt(config.pbs.port) || 8007,
                tokenId: config.pbs.tokenId,
                tokenSecret: config.pbs.tokenSecret
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
            'Proxmox VE Settings': ['PROXMOX_HOST', 'PROXMOX_PORT', 'PROXMOX_TOKEN_ID', 'PROXMOX_TOKEN_SECRET'],
            'Proxmox Backup Server Settings': ['PBS_HOST', 'PBS_PORT', 'PBS_TOKEN_ID', 'PBS_TOKEN_SECRET'],
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