const fs = require('fs').promises;
const fsSync = require('fs');
const path = require('path');
const { loadConfiguration } = require('./configLoader');
const { initializeApiClients } = require('./apiClients');
const customThresholdManager = require('./customThresholds');

class ConfigApi {
    constructor() {
        // Use persistent config directory if it exists (for Docker), otherwise use project root
        const configDir = path.join(__dirname, '../config');
        const configEnvPath = path.join(configDir, '.env');
        const projectRootEnv = path.join(__dirname, '../.env');
        
        // Check if we're in a Docker environment with persistent config volume
        // Use config/.env if it exists, otherwise fall back to project root .env
        if (fsSync.existsSync(configEnvPath)) {
            this.envPath = configEnvPath;
        } else {
            this.envPath = projectRootEnv;
        }
    }

    /**
     * Get current configuration (without secrets)
     */
    async getConfig() {
        try {
            const config = await this.readEnvFile();
            const packageJson = require('../package.json');
            
            // Build the response structure including all additional endpoints
            const response = {
                version: packageJson.version,
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
                    updateChannel: config.UPDATE_CHANNEL || 'stable',
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
                    },
                    webhook: {
                        url: config.WEBHOOK_URL,
                        enabled: config.WEBHOOK_ENABLED === 'true'
                    }
                }
            };
            
            // Add all additional endpoint configurations to the response
            // This allows the settings modal to properly display them
            Object.keys(config).forEach(key => {
                // Include all additional PVE and PBS endpoint variables and webhook config
                if ((key.startsWith('PROXMOX_') && key.includes('_')) || 
                    (key.startsWith('PBS_') && key.includes('_')) ||
                    key.startsWith('WEBHOOK_')) {
                    response[key] = config[key];
                }
            });
            
            return response;
        } catch (error) {
            console.error('Error reading configuration:', error);
            return { version: 'unknown', proxmox: null, pbs: null, advanced: {} };
        }
    }

    /**
     * Save configuration to .env file
     */
    async saveConfig(config) {
        try {
            // Read existing .env file to preserve other settings
            const existingConfig = await this.readEnvFile();
            
            // Handle both old structured format and new raw .env variable format
            if (config.proxmox || config.pbs || config.advanced) {
                // Old structured format
                this.handleStructuredConfig(config, existingConfig);
            } else {
                // New raw .env variable format from settings form
                this.handleRawEnvConfig(config, existingConfig);
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
     * Handle structured configuration format (old setup flow)
     */
    handleStructuredConfig(config, existingConfig) {
        // Update with new values
        if (config.proxmox) {
            existingConfig.PROXMOX_HOST = config.proxmox.host;
            existingConfig.PROXMOX_PORT = config.proxmox.port || '8006';
            existingConfig.PROXMOX_TOKEN_ID = config.proxmox.tokenId;
            
            // Only update token secret if provided (allows keeping existing secret)
            if (config.proxmox.tokenSecret) {
                existingConfig.PROXMOX_TOKEN_SECRET = config.proxmox.tokenSecret;
            }
            
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
            
            // Webhook settings
            if (config.advanced.webhook) {
                const webhook = config.advanced.webhook;
                if (webhook.url !== undefined) {
                    existingConfig.WEBHOOK_URL = webhook.url;
                }
                if (webhook.enabled !== undefined) {
                    existingConfig.WEBHOOK_ENABLED = webhook.enabled ? 'true' : 'false';
                }
            }
        }
    }

    /**
     * Handle raw .env variable format (new settings form)
     */
    handleRawEnvConfig(config, existingConfig) {
        // First, identify which additional endpoints exist in the current config
        const existingPveEndpoints = new Set();
        const existingPbsEndpoints = new Set();
        
        Object.keys(existingConfig).forEach(key => {
            const pveMatch = key.match(/^PROXMOX_HOST_(\d+)$/);
            if (pveMatch) {
                existingPveEndpoints.add(pveMatch[1]);
            }
            const pbsMatch = key.match(/^PBS_HOST_(\d+)$/);
            if (pbsMatch) {
                existingPbsEndpoints.add(pbsMatch[1]);
            }
        });
        
        // Identify which endpoints are in the new config
        const newPveEndpoints = new Set();
        const newPbsEndpoints = new Set();
        
        Object.keys(config).forEach(key => {
            const pveMatch = key.match(/^PROXMOX_HOST_(\d+)$/);
            if (pveMatch) {
                newPveEndpoints.add(pveMatch[1]);
            }
            const pbsMatch = key.match(/^PBS_HOST_(\d+)$/);
            if (pbsMatch) {
                newPbsEndpoints.add(pbsMatch[1]);
            }
        });
        
        // Only remove endpoints that exist in current config but not in new config
        // AND the new config contains at least one endpoint of the same type
        // This prevents removing PBS endpoints when only adding PVE endpoints (and vice versa)
        if (newPveEndpoints.size > 0) {
            existingPveEndpoints.forEach(index => {
                if (!newPveEndpoints.has(index)) {
                    // Remove all related PVE configuration variables
                    delete existingConfig[`PROXMOX_HOST_${index}`];
                    delete existingConfig[`PROXMOX_PORT_${index}`];
                    delete existingConfig[`PROXMOX_TOKEN_ID_${index}`];
                    delete existingConfig[`PROXMOX_TOKEN_SECRET_${index}`];
                    delete existingConfig[`PROXMOX_NODE_NAME_${index}`];
                    delete existingConfig[`PROXMOX_ENABLED_${index}`];
                    delete existingConfig[`PROXMOX_ALLOW_SELF_SIGNED_CERT_${index}`];
                    delete existingConfig[`PROXMOX_ALLOW_SELF_SIGNED_CERTS_${index}`];
                }
            });
        }
        
        if (newPbsEndpoints.size > 0) {
            existingPbsEndpoints.forEach(index => {
                if (!newPbsEndpoints.has(index)) {
                    // Remove all related PBS configuration variables
                    delete existingConfig[`PBS_HOST_${index}`];
                    delete existingConfig[`PBS_PORT_${index}`];
                    delete existingConfig[`PBS_TOKEN_ID_${index}`];
                    delete existingConfig[`PBS_TOKEN_SECRET_${index}`];
                    delete existingConfig[`PBS_NODE_NAME_${index}`];
                    delete existingConfig[`PBS_ALLOW_SELF_SIGNED_CERT_${index}`];
                    delete existingConfig[`PBS_ALLOW_SELF_SIGNED_CERTS_${index}`];
                }
            });
        }
        
        // Update existing config with new values
        Object.entries(config).forEach(([key, value]) => {
            if (value !== undefined && value !== '') {
                // Special validation for UPDATE_CHANNEL
                if (key === 'UPDATE_CHANNEL') {
                    const validChannels = ['stable', 'rc'];
                    if (!validChannels.includes(value)) {
                        console.warn(`WARN: Invalid UPDATE_CHANNEL value "${value}" in config. Skipping.`);
                        return; // Skip this invalid value
                    }
                }
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
            const testEndpoints = [];
            const testPbsConfigs = [];
            const existingConfig = await this.readEnvFile();
            const failedEndpoints = [];
            
            // Handle both old structured format and new raw .env format
            if (config.proxmox) {
                // Old structured format - test primary endpoint only
                const { host, port, tokenId, tokenSecret } = config.proxmox;
                
                if (host && tokenId) {
                    const secret = tokenSecret || existingConfig.PROXMOX_TOKEN_SECRET;
                    if (secret) {
                        testEndpoints.push({
                            id: 'test-primary',
                            name: 'Primary PVE',
                            host,
                            port: parseInt(port) || 8006,
                            tokenId,
                            tokenSecret: secret,
                            enabled: true,
                            allowSelfSignedCerts: true
                        });
                    }
                }
                
                if (config.pbs) {
                    const { host, port, tokenId, tokenSecret } = config.pbs;
                    if (host && tokenId) {
                        const secret = tokenSecret || existingConfig.PBS_TOKEN_SECRET;
                        if (secret) {
                            testPbsConfigs.push({
                                id: 'test-pbs-primary',
                                name: 'Primary PBS',
                                host,
                                port: parseInt(port) || 8007,
                                tokenId,
                                tokenSecret: secret,
                                allowSelfSignedCerts: true
                            });
                        }
                    }
                }
            } else {
                // New raw .env format - test all endpoints including additional ones
                
                // Test primary PVE endpoint
                if (config.PROXMOX_HOST && config.PROXMOX_TOKEN_ID) {
                    const secret = config.PROXMOX_TOKEN_SECRET || existingConfig.PROXMOX_TOKEN_SECRET;
                    if (secret) {
                        testEndpoints.push({
                            id: 'test-primary',
                            name: config.PROXMOX_NODE_NAME || 'Primary PVE',
                            host: config.PROXMOX_HOST,
                            port: parseInt(config.PROXMOX_PORT) || 8006,
                            tokenId: config.PROXMOX_TOKEN_ID,
                            tokenSecret: secret,
                            enabled: config.PROXMOX_ENABLED !== 'false',
                            allowSelfSignedCerts: true
                        });
                    }
                }
                
                // Test additional PVE endpoints
                Object.keys(config).forEach(key => {
                    const match = key.match(/^PROXMOX_HOST_(\d+)$/);
                    if (match) {
                        const index = match[1];
                        const host = config[`PROXMOX_HOST_${index}`];
                        const tokenId = config[`PROXMOX_TOKEN_ID_${index}`];
                        const enabled = config[`PROXMOX_ENABLED_${index}`] !== 'false';
                        
                        if (host && tokenId && enabled) {
                            const secret = config[`PROXMOX_TOKEN_SECRET_${index}`] || existingConfig[`PROXMOX_TOKEN_SECRET_${index}`];
                            if (secret) {
                                testEndpoints.push({
                                    id: `test-endpoint-${index}`,
                                    name: config[`PROXMOX_NODE_NAME_${index}`] || `PVE Endpoint ${index}`,
                                    host,
                                    port: parseInt(config[`PROXMOX_PORT_${index}`]) || 8006,
                                    tokenId,
                                    tokenSecret: secret,
                                    enabled: true,
                                    allowSelfSignedCerts: true
                                });
                            }
                        }
                    }
                });
                
                // Test primary PBS endpoint
                if (config.PBS_HOST && config.PBS_TOKEN_ID) {
                    const secret = config.PBS_TOKEN_SECRET || existingConfig.PBS_TOKEN_SECRET;
                    if (secret) {
                        testPbsConfigs.push({
                            id: 'test-pbs-primary',
                            name: config.PBS_NODE_NAME || 'Primary PBS',
                            host: config.PBS_HOST,
                            port: parseInt(config.PBS_PORT) || 8007,
                            tokenId: config.PBS_TOKEN_ID,
                            tokenSecret: secret,
                            allowSelfSignedCerts: true
                        });
                    }
                }
                
                // Test additional PBS endpoints
                Object.keys(config).forEach(key => {
                    const match = key.match(/^PBS_HOST_(\d+)$/);
                    if (match) {
                        const index = match[1];
                        const host = config[`PBS_HOST_${index}`];
                        const tokenId = config[`PBS_TOKEN_ID_${index}`];
                        
                        if (host && tokenId) {
                            const secret = config[`PBS_TOKEN_SECRET_${index}`] || existingConfig[`PBS_TOKEN_SECRET_${index}`];
                            if (secret) {
                                testPbsConfigs.push({
                                    id: `test-pbs-${index}`,
                                    name: config[`PBS_NODE_NAME_${index}`] || `PBS Endpoint ${index}`,
                                    host,
                                    port: parseInt(config[`PBS_PORT_${index}`]) || 8007,
                                    tokenId,
                                    tokenSecret: secret,
                                    allowSelfSignedCerts: true
                                });
                            }
                        }
                    }
                });
            }
            
            if (testEndpoints.length === 0 && testPbsConfigs.length === 0) {
                return {
                    success: false,
                    error: 'No endpoints configured to test. Please ensure host, token ID, and token secret are provided.'
                };
            }
            
            // Test all endpoints
            const { apiClients, pbsApiClients } = await initializeApiClients(testEndpoints, testPbsConfigs);
            
            // Test each PVE endpoint
            for (const endpoint of testEndpoints) {
                try {
                    const client = apiClients[endpoint.id];
                    if (client) {
                        await client.client.get('/nodes');
                    }
                } catch (error) {
                    failedEndpoints.push(`${endpoint.name}: ${error.message}`);
                }
            }
            
            // Test each PBS endpoint
            for (const pbsConfig of testPbsConfigs) {
                try {
                    const client = pbsApiClients[pbsConfig.id];
                    if (client) {
                        await client.client.get('/nodes');
                    }
                } catch (error) {
                    failedEndpoints.push(`${pbsConfig.name}: ${error.message}`);
                }
            }
            
            if (failedEndpoints.length > 0) {
                return {
                    success: false,
                    error: `Connection test failed for: ${failedEndpoints.join(', ')}`
                };
            }
            
            return { success: true };
        } catch (error) {
            console.error('Configuration test failed:', error);
            return { 
                success: false, 
                error: error.message || 'Failed to test endpoint connections'
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
            'Primary Proxmox VE Settings': ['PROXMOX_HOST', 'PROXMOX_PORT', 'PROXMOX_TOKEN_ID', 'PROXMOX_TOKEN_SECRET', 'PROXMOX_NODE_NAME', 'PROXMOX_ENABLED', 'PROXMOX_ALLOW_SELF_SIGNED_CERT'],
            'Additional Proxmox VE Endpoints': [], // Will be populated dynamically
            'Primary Proxmox Backup Server Settings': ['PBS_HOST', 'PBS_PORT', 'PBS_TOKEN_ID', 'PBS_TOKEN_SECRET', 'PBS_NODE_NAME', 'PBS_ALLOW_SELF_SIGNED_CERT'],
            'Additional PBS Endpoints': [], // Will be populated dynamically
            'Pulse Service Settings': ['PULSE_METRIC_INTERVAL_MS', 'PULSE_DISCOVERY_INTERVAL_MS'],
            'Alert System Configuration': [
                'ALERT_CPU_ENABLED', 'ALERT_CPU_THRESHOLD', 'ALERT_CPU_DURATION',
                'ALERT_MEMORY_ENABLED', 'ALERT_MEMORY_THRESHOLD', 'ALERT_MEMORY_DURATION',
                'ALERT_DISK_ENABLED', 'ALERT_DISK_THRESHOLD', 'ALERT_DISK_DURATION',
                'ALERT_DOWN_ENABLED', 'ALERT_DOWN_DURATION'
            ],
            'Other Settings': [] // Will contain all other keys
        };
        
        // Collect additional endpoint configurations
        const additionalPveEndpoints = {};
        const additionalPbsEndpoints = {};
        
        Object.keys(config).forEach(key => {
            // Check for additional PVE endpoints
            const pveMatch = key.match(/^PROXMOX_(.+)_(\d+)$/);
            if (pveMatch) {
                const [, type, index] = pveMatch;
                if (!additionalPveEndpoints[index]) {
                    additionalPveEndpoints[index] = [];
                }
                additionalPveEndpoints[index].push(key);
            }
            
            // Check for additional PBS endpoints
            const pbsMatch = key.match(/^PBS_(.+)_(\d+)$/);
            if (pbsMatch) {
                const [, type, index] = pbsMatch;
                if (!additionalPbsEndpoints[index]) {
                    additionalPbsEndpoints[index] = [];
                }
                additionalPbsEndpoints[index].push(key);
            }
        });
        
        // Find other keys not in predefined groups or additional endpoints
        Object.keys(config).forEach(key => {
            let found = false;
            
            // Check if it's in a predefined group
            Object.values(groups).forEach(groupKeys => {
                if (groupKeys.includes(key)) found = true;
            });
            
            // Check if it's an additional endpoint key
            if (key.match(/^PROXMOX_.+_\d+$/) || key.match(/^PBS_.+_\d+$/)) {
                found = true;
            }
            
            if (!found && key !== '') {
                groups['Other Settings'].push(key);
            }
        });
        
        // Write primary settings first
        ['Primary Proxmox VE Settings', 'Primary Proxmox Backup Server Settings'].forEach(groupName => {
            const keys = groups[groupName];
            if (keys.length > 0 && keys.some(key => config[key])) {
                lines.push(`# ${groupName}`);
                keys.forEach(key => {
                    if (config[key] !== undefined && config[key] !== '' && config[key] !== null) {
                        const value = String(config[key]); // Ensure value is a string
                        const needsQuotes = value.includes(' ') || value.includes('#') || value.includes('=');
                        lines.push(`${key}=${needsQuotes ? `"${value}"` : value}`);
                    }
                });
                lines.push('');
            }
        });
        
        // Write additional PVE endpoints
        if (Object.keys(additionalPveEndpoints).length > 0) {
            lines.push('# Additional Proxmox VE Endpoints');
            Object.keys(additionalPveEndpoints).sort((a, b) => parseInt(a) - parseInt(b)).forEach(index => {
                lines.push(`# PVE Endpoint ${index}`);
                const orderedKeys = [
                    `PROXMOX_HOST_${index}`,
                    `PROXMOX_PORT_${index}`,
                    `PROXMOX_TOKEN_ID_${index}`,
                    `PROXMOX_TOKEN_SECRET_${index}`,
                    `PROXMOX_NODE_NAME_${index}`,
                    `PROXMOX_ENABLED_${index}`,
                    `PROXMOX_ALLOW_SELF_SIGNED_CERT_${index}`,
                    `PROXMOX_ALLOW_SELF_SIGNED_CERTS_${index}`
                ];
                orderedKeys.forEach(key => {
                    if (config[key] !== undefined && config[key] !== '' && config[key] !== null) {
                        const value = String(config[key]); // Ensure value is a string
                        const needsQuotes = value.includes(' ') || value.includes('#') || value.includes('=');
                        lines.push(`${key}=${needsQuotes ? `"${value}"` : value}`);
                    }
                });
            });
            lines.push('');
        }
        
        // Write additional PBS endpoints
        if (Object.keys(additionalPbsEndpoints).length > 0) {
            lines.push('# Additional PBS Endpoints');
            Object.keys(additionalPbsEndpoints).sort((a, b) => parseInt(a) - parseInt(b)).forEach(index => {
                lines.push(`# PBS Endpoint ${index}`);
                const orderedKeys = [
                    `PBS_HOST_${index}`,
                    `PBS_PORT_${index}`,
                    `PBS_TOKEN_ID_${index}`,
                    `PBS_TOKEN_SECRET_${index}`,
                    `PBS_NODE_NAME_${index}`,
                    `PBS_ALLOW_SELF_SIGNED_CERT_${index}`,
                    `PBS_ALLOW_SELF_SIGNED_CERTS_${index}`
                ];
                orderedKeys.forEach(key => {
                    if (config[key] !== undefined && config[key] !== '' && config[key] !== null) {
                        const value = String(config[key]); // Ensure value is a string
                        const needsQuotes = value.includes(' ') || value.includes('#') || value.includes('=');
                        lines.push(`${key}=${needsQuotes ? `"${value}"` : value}`);
                    }
                });
            });
            lines.push('');
        }
        
        // Write remaining groups
        ['Pulse Service Settings', 'Alert System Configuration', 'Other Settings'].forEach(groupName => {
            const keys = groups[groupName];
            if (keys.length > 0 && keys.some(key => config[key])) {
                lines.push(`# ${groupName}`);
                keys.forEach(key => {
                    if (config[key] !== undefined && config[key] !== '' && config[key] !== null) {
                        const value = String(config[key]); // Ensure value is a string
                        const needsQuotes = value.includes(' ') || value.includes('#') || value.includes('=');
                        lines.push(`${key}=${needsQuotes ? `"${value}"` : value}`);
                    }
                });
                lines.push('');
            }
        });
        
        try {
            await fs.writeFile(this.envPath, lines.join('\n'), 'utf8');
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
            
            // Clear all environment variables that might be cached
            Object.keys(process.env).forEach(key => {
                if (key.startsWith('PROXMOX_') || key.startsWith('PBS_') || key.startsWith('PULSE_') || key.startsWith('ALERT_')) {
                    delete process.env[key];
                }
            });
            
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
            
            // Refresh AlertManager rules based on new environment variables
            try {
                const alertManager = stateManager.getAlertManager();
                if (alertManager && typeof alertManager.refreshRules === 'function') {
                    alertManager.refreshRules();
                    console.log('Alert rules refreshed after configuration reload');
                } else {
                    console.warn('AlertManager not available or refreshRules method not found');
                }
            } catch (alertError) {
                console.error('Error refreshing alert rules:', alertError);
                // Don't fail the entire reload if alert refresh fails
            }
            
            // Trigger a discovery cycle if we have any endpoints configured (PVE or PBS)
            if (endpoints.length > 0 || pbsConfigs.length > 0) {
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
        console.log('[ConfigApi] setupRoutes called, registering all endpoints...');
        
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
                const result = await this.saveConfig(req.body);
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

        // === Custom Threshold API Endpoints ===
        // Note: Threshold routes are now handled by thresholdRoutes.js
        console.log('[ConfigApi] Custom threshold endpoints handled by separate thresholdRoutes module');

    }
}

module.exports = ConfigApi;