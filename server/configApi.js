const fs = require('fs').promises;
const fsSync = require('fs');
const path = require('path');
const { loadConfiguration } = require('./configLoader');
const { initializeApiClients } = require('./apiClients');
const customThresholdManager = require('./customThresholds');
const ValidationMiddleware = require('./middleware/validation');
const { sanitizeConfig, audit } = require('./security');

class ConfigApi {
    constructor() {
        // Use persistent config directory if it exists (for Docker), otherwise use project root
        const configDir = path.join(__dirname, '../config');
        const configEnvPath = path.join(configDir, '.env');
        const projectRootEnv = path.join(__dirname, '../.env');
        
        // Check if we're in a Docker environment with persistent config volume
        // Use config/.env if the config directory exists (Docker persistent volume), otherwise use project root .env
        if (fsSync.existsSync(configDir)) {
            this.envPath = configEnvPath;
            this.configDir = configDir;
        } else {
            this.envPath = projectRootEnv;
            this.configDir = path.dirname(projectRootEnv);
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
                    enabled: config.PROXMOX_ENABLED !== 'false',
                    hasToken: !!config.PROXMOX_TOKEN_SECRET,
                    // Don't send the secret
                } : null,
                pbs: config.PBS_HOST ? {
                    host: config.PBS_HOST,
                    port: config.PBS_PORT || '8007',
                    tokenId: config.PBS_TOKEN_ID,
                    hasToken: !!config.PBS_TOKEN_SECRET,
                    // Don't send the secret
                } : null,
                advanced: {
                    metricInterval: config.PULSE_METRIC_INTERVAL_MS,
                    discoveryInterval: config.PULSE_DISCOVERY_INTERVAL_MS,
                    updateChannel: config.UPDATE_CHANNEL || 'stable',
                    allowEmbedding: config.ALLOW_EMBEDDING === 'true',
                    allowedEmbedOrigins: config.ALLOWED_EMBED_ORIGINS || '',
                    alerts: {
                        enabled: config.ALERTS_ENABLED !== 'false',
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
                    },
                    smtp: {
                        host: config.SMTP_HOST,
                        port: config.SMTP_PORT,
                        user: config.SMTP_USER,
                        from: config.ALERT_FROM_EMAIL,
                        to: config.ALERT_TO_EMAIL,
                        secure: config.SMTP_SECURE === 'true'
                        // Don't send password for security
                    },
                    security: {
                        mode: config.SECURITY_MODE || 'private',
                        auditLog: config.AUDIT_LOG === 'true',
                        bcryptRounds: parseInt(config.BCRYPT_ROUNDS || '10', 10),
                        maxLoginAttempts: parseInt(config.MAX_LOGIN_ATTEMPTS || '5', 10),
                        lockoutDuration: parseInt(config.LOCKOUT_DURATION || '900000', 10),
                        sessionTimeout: parseInt(config.SESSION_TIMEOUT_HOURS || '24', 10) * 3600000,
                        hasAdminPassword: !!config.ADMIN_PASSWORD,
                        hasSessionSecret: !!config.SESSION_SECRET
                    }
                }
            };
            
            // Add all additional endpoint configurations to the response
            // This allows the settings modal to properly display them
            Object.keys(config).forEach(key => {
                // Include all additional PVE and PBS endpoint variables, webhook config, and email config
                if ((key.startsWith('PROXMOX_') && key.includes('_')) || 
                    (key.startsWith('PBS_') && key.includes('_')) ||
                    key.startsWith('WEBHOOK_') ||
                    key.startsWith('SMTP_') ||
                    key.startsWith('ALERT_')) {
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
            // Make this asynchronous to prevent frontend freezing
            setImmediate(() => {
                this.reloadConfiguration().catch(error => {
                    console.error('Error during async configuration reload:', error);
                });
            });
            
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
            
            // Only update PBS_TOKEN_SECRET if it's provided
            if (config.pbs.tokenSecret) {
                existingConfig.PBS_TOKEN_SECRET = config.pbs.tokenSecret;
            }
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
                if (alerts.enabled !== undefined) {
                    existingConfig.ALERTS_ENABLED = alerts.enabled ? 'true' : 'false';
                }
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
            
            // Iframe embedding settings
            if (config.advanced.allowEmbedding !== undefined) {
                existingConfig.ALLOW_EMBEDDING = config.advanced.allowEmbedding ? 'true' : 'false';
            }
            if (config.advanced.allowedEmbedOrigins !== undefined) {
                existingConfig.ALLOWED_EMBED_ORIGINS = config.advanced.allowedEmbedOrigins;
            }
            
            // Security settings
            if (config.advanced.security) {
                const security = config.advanced.security;
                if (security.mode !== undefined) {
                    existingConfig.SECURITY_MODE = security.mode;
                }
                if (security.auditLog !== undefined) {
                    existingConfig.AUDIT_LOG = security.auditLog ? 'true' : 'false';
                }
                if (security.bcryptRounds !== undefined) {
                    existingConfig.BCRYPT_ROUNDS = String(security.bcryptRounds);
                }
                if (security.maxLoginAttempts !== undefined) {
                    existingConfig.MAX_LOGIN_ATTEMPTS = String(security.maxLoginAttempts);
                }
                if (security.lockoutDuration !== undefined) {
                    existingConfig.LOCKOUT_DURATION = String(security.lockoutDuration);
                }
                // New session and cookie settings
                if (security.sessionTimeout !== undefined) {
                    // Convert from milliseconds to hours for storage
                    existingConfig.SESSION_TIMEOUT_HOURS = String(Math.round(security.sessionTimeout / 3600000));
                }
                // Handle password changes specially
                if (security.adminPassword && security.adminPassword !== '***CHANGE_ME***' && security.adminPassword !== '***REDACTED***') {
                    const bcrypt = require('bcryptjs');
                    const rounds = parseInt(existingConfig.BCRYPT_ROUNDS || '10', 10);
                    existingConfig.ADMIN_PASSWORD = bcrypt.hashSync(security.adminPassword, rounds);
                }
                if (security.sessionSecret && security.sessionSecret !== '***GENERATE_ME***' && security.sessionSecret !== '***REDACTED***') {
                    existingConfig.SESSION_SECRET = security.sessionSecret;
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
            // Handle empty values for fields that need to be cleared
            if (key === 'ALLOWED_EMBED_ORIGINS' && value === '') {
                existingConfig[key] = '';
                return;
            }
            
            if (value !== undefined && value !== '') {
                // CRITICAL: Never save redacted values - skip any redacted tokens/secrets
                if (value === '***REDACTED***' || value === '[REDACTED]') {
                    console.log(`[Config] Skipping redacted value for ${key}`);
                    return; // Skip redacted values to preserve original
                }
                
                // Special validation for UPDATE_CHANNEL
                if (key === 'UPDATE_CHANNEL') {
                    const validChannels = ['stable', 'rc'];
                    if (!validChannels.includes(value)) {
                        console.warn(`WARN: Invalid UPDATE_CHANNEL value "${value}" in config. Skipping.`);
                        return; // Skip this invalid value
                    }
                }
                
                // Handle password changes specially - hash before storing
                if (key === 'ADMIN_PASSWORD' && value.trim() !== '') {
                    const bcrypt = require('bcryptjs');
                    const rounds = parseInt(existingConfig.BCRYPT_ROUNDS || config.BCRYPT_ROUNDS || '10', 10);
                    existingConfig[key] = bcrypt.hashSync(value, rounds);
                } else {
                    existingConfig[key] = value;
                }
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
            
            console.log('[testConfig] Testing with config keys:', Object.keys(config).filter(k => k.includes('PROXMOX')));
            console.log('[testConfig] Token values:', {
                primary_id: config.PROXMOX_TOKEN_ID,
                primary_secret_provided: !!config.PROXMOX_TOKEN_SECRET,
                primary_secret_exists: !!existingConfig.PROXMOX_TOKEN_SECRET,
                endpoint2_id: config.PROXMOX_TOKEN_ID_2,
                endpoint2_secret_provided: !!config.PROXMOX_TOKEN_SECRET_2,
                endpoint2_secret_exists: !!existingConfig.PROXMOX_TOKEN_SECRET_2
            });
            
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
                    // Handle empty token secret by using existing config
                    const secret = (config.PROXMOX_TOKEN_SECRET && config.PROXMOX_TOKEN_SECRET.trim() !== '') 
                        ? config.PROXMOX_TOKEN_SECRET 
                        : existingConfig.PROXMOX_TOKEN_SECRET;
                    if (secret) {
                        console.log(`[testConfig] Adding primary PVE endpoint:`);
                        console.log(`  - Host: ${config.PROXMOX_HOST}`);
                        console.log(`  - TokenID: ${config.PROXMOX_TOKEN_ID}`);
                        console.log(`  - Secret from: ${config.PROXMOX_TOKEN_SECRET ? 'request' : 'existing config'}`);
                        console.log(`  - Secret length: ${secret ? secret.length : 0}`);
                        testEndpoints.push({
                            id: 'test-primary',
                            name: 'Primary PVE',
                            host: config.PROXMOX_HOST,
                            port: parseInt(config.PROXMOX_PORT) || 8006,
                            tokenId: config.PROXMOX_TOKEN_ID,
                            tokenSecret: secret,
                            enabled: config.PROXMOX_ENABLED !== 'false',
                            allowSelfSignedCerts: true
                        });
                    } else {
                        console.log('[testConfig] No secret found for primary PVE endpoint');
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
                            // Handle empty token secret by using existing config
                            const secretKey = `PROXMOX_TOKEN_SECRET_${index}`;
                            const secret = (config[secretKey] && config[secretKey].trim() !== '') 
                                ? config[secretKey] 
                                : existingConfig[secretKey];
                            if (secret) {
                                testEndpoints.push({
                                    id: `test-endpoint-${index}`,
                                    name: `PVE Endpoint ${index}`,
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
                            name: 'Primary PBS',
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
                                    name: `PBS Endpoint ${index}`,
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
                // Check what's missing to provide better error message
                const missingSecrets = [];
                
                if (config.PROXMOX_HOST && config.PROXMOX_TOKEN_ID && !config.PROXMOX_TOKEN_SECRET && !existingConfig.PROXMOX_TOKEN_SECRET) {
                    missingSecrets.push('Primary PVE token secret');
                }
                
                if (config.PBS_HOST && config.PBS_TOKEN_ID && !config.PBS_TOKEN_SECRET && !existingConfig.PBS_TOKEN_SECRET) {
                    missingSecrets.push('Primary PBS token secret');
                }
                
                // Check additional endpoints
                Object.keys(config).forEach(key => {
                    const pveMatch = key.match(/^PROXMOX_HOST_(\d+)$/);
                    if (pveMatch) {
                        const index = pveMatch[1];
                        if (config[`PROXMOX_TOKEN_ID_${index}`] && !config[`PROXMOX_TOKEN_SECRET_${index}`] && !existingConfig[`PROXMOX_TOKEN_SECRET_${index}`]) {
                            missingSecrets.push(`PVE Endpoint ${index} token secret`);
                        }
                    }
                    
                    const pbsMatch = key.match(/^PBS_HOST_(\d+)$/);
                    if (pbsMatch) {
                        const index = pbsMatch[1];
                        if (config[`PBS_TOKEN_ID_${index}`] && !config[`PBS_TOKEN_SECRET_${index}`] && !existingConfig[`PBS_TOKEN_SECRET_${index}`]) {
                            missingSecrets.push(`PBS Endpoint ${index} token secret`);
                        }
                    }
                });
                
                if (missingSecrets.length > 0) {
                    return {
                        success: false,
                        error: `Missing token secrets for: ${missingSecrets.join(', ')}. Please enter the token secret(s) to test the connection.`
                    };
                }
                
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
                        console.log(`[testConfig] Testing ${endpoint.name} with token: ${endpoint.tokenId}=<hidden>`);
                        await client.client.get('/nodes');
                        console.log(`[testConfig] ${endpoint.name} test successful`);
                    }
                } catch (error) {
                    console.error(`[testConfig] ${endpoint.name} test failed:`, error.response?.status, error.response?.data || error.message);
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
            // Ensure the config directory exists before writing
            await fs.mkdir(this.configDir, { recursive: true });
            await fs.writeFile(this.envPath, lines.join('\n'), 'utf8');
        } catch (writeError) {
            console.error('[ConfigApi.writeEnvFile] Error writing file:', writeError);
            throw writeError;
        }
    }

    /**
     * Update a single environment variable in the .env file
     * This is used for persisting individual setting changes like alert rule states
     */
    async updateEnvironmentVariable(variableName, value) {
        try {
            // Read current configuration
            const config = await this.readEnvFile();
            
            // Update the specific variable
            config[variableName] = value;
            
            // Write back to file
            await this.writeEnvFile(config);
            
            // Update the current process environment
            process.env[variableName] = value;
            
            console.log(`[ConfigApi] Updated environment variable ${variableName} = ${value}`);
            
            return { success: true };
        } catch (error) {
            console.error(`[ConfigApi] Error updating environment variable ${variableName}:`, error);
            throw error;
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
                if (key.startsWith('PROXMOX_') || key.startsWith('PBS_') || key.startsWith('PULSE_') || key.startsWith('ALERT_') || key.startsWith('GLOBAL_')) {
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
            
            // Reinitialize API clients asynchronously
            initializeApiClients(endpoints, pbsConfigs).then(({ apiClients, pbsApiClients }) => {
                // Update global references
                if (global.pulseApiClients) {
                    global.pulseApiClients.apiClients = apiClients;
                    global.pulseApiClients.pbsApiClients = pbsApiClients;
                }
                
                console.log('API clients reinitialized after configuration reload');
                
                if (endpoints.length > 0 || pbsConfigs.length > 0) {
                    console.log('Triggering discovery cycle after configuration reload...');
                    // Import and call runDiscoveryCycle if available
                    if (global.runDiscoveryCycle && typeof global.runDiscoveryCycle === 'function') {
                        setTimeout(() => {
                            global.runDiscoveryCycle();
                        }, 2000); // Give time for API clients to be ready
                    }
                }
            }).catch(error => {
                console.error('Error reinitializing API clients:', error);
            });
            
            // Update global config placeholder status
            if (global.pulseConfigStatus) {
                global.pulseConfigStatus.isPlaceholder = isConfigPlaceholder;
            }
            
            // Update last reload time to prevent file watcher from triggering
            if (global.lastReloadTime !== undefined) {
                global.lastReloadTime = Date.now();
            }
            
            // Refresh AlertManager rules and email configuration asynchronously
            setImmediate(async () => {
                try {
                    const alertManager = stateManager.getAlertManager();
                    if (alertManager) {
                        if (typeof alertManager.refreshRules === 'function') {
                            await alertManager.refreshRules();
                            console.log('Alert rules refreshed after configuration reload');
                        }
                        if (typeof alertManager.reloadEmailConfiguration === 'function') {
                            await alertManager.reloadEmailConfiguration();
                            console.log('Email configuration reloaded after configuration reload');
                        }
                    } else {
                        console.warn('AlertManager not available');
                    }
                } catch (alertError) {
                    console.error('Error refreshing AlertManager configuration:', alertError);
                    // Don't fail the entire reload if alert refresh fails
                }
            });
            
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
                
                // Audit log
                if (req.auth?.user) {
                    audit.configRead(req.auth.user, req);
                }
                
                // Sanitize sensitive data
                const sanitized = sanitizeConfig(config);
                res.json(sanitized);
            } catch (error) {
                res.status(500).json({ error: 'Failed to read configuration' });
            }
        });

        // Save configuration
        app.post('/api/config', async (req, res) => {
            try {
                const result = await this.saveConfig(req.body);
                
                // Audit log
                if (req.auth?.user) {
                    audit.configChanged(req.body, req.auth.user, req);
                }
                
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
                console.log('[API /api/config/test] Request body type:', typeof req.body);
                console.log('[API /api/config/test] Request body keys:', Object.keys(req.body || {}));
                
                // Log specific values to debug
                console.log('[API /api/config/test] Token IDs received:', {
                    primary: req.body.PROXMOX_TOKEN_ID,
                    endpoint2: req.body.PROXMOX_TOKEN_ID_2
                });
                
                const result = await this.testConfig(req.body);
                res.json(result);
            } catch (error) {
                console.error('[API /api/config/test] Error:', error);
                res.status(500).json({ 
                    success: false, 
                    error: 'Internal server error',
                    message: error.message || 'Failed to test configuration' 
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

// Create a singleton instance for use by other modules
const configApiInstance = new ConfigApi();

module.exports = ConfigApi;
module.exports.updateEnvironmentVariable = (variableName, value) => configApiInstance.updateEnvironmentVariable(variableName, value);