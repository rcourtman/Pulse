const { URL } = require('url');

// Align placeholder values with install script and .env.example
const placeholderValues = [
  // Hostname parts - case-insensitive matching might be better if OS env vars differ.
  // For now, direct case-sensitive include check.
  'your-proxmox-ip-or-hostname', 
  'proxmox_host',                 // Substring for https://proxmox_host:8006 or similar
  'YOUR_PBS_IP_OR_HOSTNAME',    // For PBS host

  // Token ID parts - these are more specific to example/guidance values
  'user@pam!your-token-name',   // Matches common PVE example format
  'user@pbs!your-token-name',   // Matches common PBS example format
  'your-api-token-id',          // Generic part often seen in examples
  'user@pam!tokenid',           // From original install script comment
  'user@pbs!tokenid',           // PBS variant of install script comment

  // Secret parts
  'your-token-secret-uuid',     // Common PVE secret example
  'your-pbs-token-secret-uuid', // Common PBS secret example
  'YOUR_API_SECRET_HERE',       // From original install script comment
  'secret-uuid',                 // Specific value used in config.test.js
  'your-api-token-uuid',         // Specific value used in config.test.js for PROXMOX_TOKEN_SECRET
  'your-port'                   // Specific value used in config.test.js for PROXMOX_PORT
];

// Error class for configuration issues
class ConfigurationError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ConfigurationError';
  }
}

// Function to get update channel preference
function getUpdateChannelPreference() {
    const fs = require('fs');
    const path = require('path');
    
    // Try to read from config/.env file first, then fallback to default
    const configDir = path.join(__dirname, '../config');
    const configEnvPath = path.join(configDir, '.env');
    
    let updateChannel = 'stable'; // Default value
    
    if (fs.existsSync(configEnvPath)) {
        try {
            const configContent = fs.readFileSync(configEnvPath, 'utf8');
            const updateChannelMatch = configContent.match(/^UPDATE_CHANNEL=(.+)$/m);
            if (updateChannelMatch) {
                updateChannel = updateChannelMatch[1].trim();
            }
        } catch (error) {
            console.warn('WARN: Could not read UPDATE_CHANNEL from config file. Using default "stable".');
        }
    }
    
    const validChannels = ['stable', 'rc'];
    if (!validChannels.includes(updateChannel)) {
        console.warn(`WARN: Invalid UPDATE_CHANNEL value "${updateChannel}". Using default "stable".`);
        return 'stable';
    }
    
    return updateChannel;
}

// Function to load PBS configuration
function loadPbsConfig(index = null) {
    const suffix = index ? `_${index}` : '';
    const hostVar = `PBS_HOST${suffix}`;
    const tokenIdVar = `PBS_TOKEN_ID${suffix}`;
    const tokenSecretVar = `PBS_TOKEN_SECRET${suffix}`;
    const nodeNameVar = `PBS_NODE_NAME${suffix}`;
    const portVar = `PBS_PORT${suffix}`;
    const selfSignedVar = `PBS_ALLOW_SELF_SIGNED_CERTS${suffix}`;

    const pbsHostUrl = process.env[hostVar];
    if (!pbsHostUrl) {
        return false; // No more PBS configs if PBS_HOST is missing
    }

    let pbsHostname = pbsHostUrl;
    try {
        const parsedUrl = new URL(pbsHostUrl);
        pbsHostname = parsedUrl.hostname;
    } catch (e) {
        // console.warn(`WARN: Could not parse PBS_HOST URL "${pbsHostUrl}". Using full value as fallback name.`);
    }

    const pbsTokenId = process.env[tokenIdVar];
    const pbsTokenSecret = process.env[tokenSecretVar];

    let config = null;
    let idPrefix = index ? `pbs_endpoint_${index}` : 'pbs_primary';

    if (pbsTokenId && pbsTokenSecret) {
        const pbsPlaceholders = placeholderValues.filter(p =>
            pbsHostUrl.includes(p) || pbsTokenId.includes(p) || pbsTokenSecret.includes(p)
        );
        if (pbsPlaceholders.length > 0) {
            console.warn(`WARN: Skipping PBS configuration ${index || 'primary'} (Token). Placeholder values detected for: ${pbsPlaceholders.join(', ')}`);
        } else {
            config = {
                id: `${idPrefix}_token`,
                authMethod: 'token',
                name: process.env[nodeNameVar] || pbsHostname,
                host: pbsHostUrl,
                port: process.env[portVar] || '8007',
                tokenId: pbsTokenId,
                tokenSecret: pbsTokenSecret,
                nodeName: process.env[nodeNameVar], // Keep nodeName field
                allowSelfSignedCerts: process.env[selfSignedVar] !== 'false',
                enabled: true
            };
            console.log(`INFO: Found PBS configuration ${index || 'primary'} with ID: ${config.id}, name: ${config.name}, host: ${config.host}`);
        }
    } else {
         console.warn(`WARN: Partial PBS configuration found for ${hostVar}. Please set (${tokenIdVar} + ${tokenSecretVar}) along with ${hostVar}.`);
    }

    if (config) {
        return { found: true, config: config }; // Return config if found
    }
    // Return found:true if host was set, but config was invalid/partial
    return { found: !!pbsHostUrl, config: null };
}


// Main function to load all configurations
function loadConfiguration() {
    // Only load .env file if not in test environment
    if (process.env.NODE_ENV !== 'test') {
        const fs = require('fs');
        const path = require('path');
        
        const configDir = path.join(__dirname, '../config');
        const configEnvPath = path.join(configDir, '.env');
        const projectEnvPath = path.join(__dirname, '../.env');

        if (fs.existsSync(configEnvPath)) {
            require('dotenv').config({ path: configEnvPath });
        } else {
            require('dotenv').config({ path: projectEnvPath });
        }
    }

    let isConfigPlaceholder = false; // Add this flag

    // --- Proxmox Primary Endpoint Validation ---
    const primaryRequiredEnvVars = [
        'PROXMOX_HOST',
        'PROXMOX_TOKEN_ID',
        'PROXMOX_TOKEN_SECRET'
    ];
    let missingVars = [];
    let placeholderVars = [];

    primaryRequiredEnvVars.forEach(varName => {
        const value = process.env[varName];
        if (!value) {
            missingVars.push(varName);
        } else if (placeholderValues.some(placeholder => value.includes(placeholder) || placeholder.includes(value))) {
            placeholderVars.push(varName);
        }
    });

    // Throw error only if required vars are MISSING
    if (missingVars.length > 0) {
        console.warn('--- Configuration Warning ---');
        console.warn(`Missing required environment variables: ${missingVars.join(', ')}.`);
        console.warn('Pulse will start in setup mode. Please configure via the web interface.');
        
        // Return minimal configuration to allow server to start
        return {
            endpoints: [],
            pbsConfigs: [],
            isConfigPlaceholder: true
        };
    }

    // Set the flag if placeholders were detected (but don't throw error)
    if (placeholderVars.length > 0) {
        isConfigPlaceholder = true;
        // Ensure token ID placeholder is included if missing
        if (process.env.PROXMOX_TOKEN_ID && !placeholderVars.includes('PROXMOX_TOKEN_ID')) {
            const hostIdx = placeholderVars.indexOf('PROXMOX_HOST');
            if (hostIdx !== -1) placeholderVars.splice(hostIdx + 1, 0, 'PROXMOX_TOKEN_ID');
            else placeholderVars.push('PROXMOX_TOKEN_ID');
        }
        console.warn(`WARN: Primary Proxmox environment variables seem to contain placeholder values: ${placeholderVars.join(', ')}. Pulse may not function correctly until configured.`);
    }

    // --- Load All Proxmox Endpoint Configurations ---
    const endpoints = [];

    function createProxmoxEndpointConfig(idPrefix, index, hostEnv, portEnv, tokenIdEnv, tokenSecretEnv, enabledEnv, selfSignedEnv, nodeNameEnv) {
        const host = process.env[hostEnv];
        const tokenId = process.env[tokenIdEnv];
        const tokenSecret = process.env[tokenSecretEnv];
        const nodeName = process.env[nodeNameEnv];

        // Basic validation for additional endpoints (primary is validated earlier)
        if (index !== null && (!tokenId || !tokenSecret)) {
            // console.warn(`WARN: Skipping endpoint ${index || idPrefix} (Host: ${host}). Missing token ID or secret.`);
            return null;
        }
        if (index !== null && placeholderValues.some(p => host.includes(p) || tokenId.includes(p) || tokenSecret.includes(p))) {
            // console.warn(`WARN: Skipping endpoint ${index || idPrefix} (Host: ${host}). Environment variables seem to contain placeholder values.`);
            return null;
        }
        
        return {
            id: index ? `${idPrefix}_${index}` : idPrefix,
            name: nodeName || null, // Only use explicitly configured names
            host: host,
            port: process.env[portEnv] || '8006',
            tokenId: tokenId,
            tokenSecret: tokenSecret,
            enabled: process.env[enabledEnv] !== 'false',
            allowSelfSignedCerts: process.env[selfSignedEnv] !== 'false',
        };
    }

    // Load primary endpoint (index null for helper)
    const primaryEndpoint = createProxmoxEndpointConfig(
        'primary', 
        null, // No index for primary
        'PROXMOX_HOST', 
        'PROXMOX_PORT', 
        'PROXMOX_TOKEN_ID', 
        'PROXMOX_TOKEN_SECRET', 
        'PROXMOX_ENABLED', 
        'PROXMOX_ALLOW_SELF_SIGNED_CERTS',
        'PROXMOX_NODE_NAME'
    );
    if (primaryEndpoint) { // Should always exist due to earlier checks, but good practice
        endpoints.push(primaryEndpoint);
    }
    
    // Load additional Proxmox endpoints
    // Check all environment variables for PROXMOX_HOST_N pattern to handle non-sequential numbering
    const proxmoxHostKeys = Object.keys(process.env)
        .filter(key => key.match(/^PROXMOX_HOST_\d+$/))
        .map(key => {
            const match = key.match(/^PROXMOX_HOST_(\d+)$/);
            return match ? parseInt(match[1]) : null;
        })
        .filter(num => num !== null)
        .sort((a, b) => a - b);
    
    for (const i of proxmoxHostKeys) {
        const additionalEndpoint = createProxmoxEndpointConfig(
            'endpoint',
            i,
            `PROXMOX_HOST_${i}`,
            `PROXMOX_PORT_${i}`,
            `PROXMOX_TOKEN_ID_${i}`,
            `PROXMOX_TOKEN_SECRET_${i}`,
            `PROXMOX_ENABLED_${i}`,
            `PROXMOX_ALLOW_SELF_SIGNED_CERTS_${i}`,
            `PROXMOX_NODE_NAME_${i}`
        );
        if (additionalEndpoint) {
            endpoints.push(additionalEndpoint);
        }
    }

    if (endpoints.length > 1) {
        // console.log(`INFO: Loaded configuration for ${endpoints.length} Proxmox endpoints.`);
    }

    // --- Load All PBS Configurations ---
    const pbsConfigs = [];
    // Load primary PBS config
    const primaryPbsResult = loadPbsConfig();
    /* istanbul ignore else */ // Ignore else path - tested by 'should not add primary PBS config if host is set but tokens are missing'
    if (primaryPbsResult.config) {
        pbsConfigs.push(primaryPbsResult.config);
    }

    // Load additional PBS configs
    // Check all environment variables for PBS_HOST_N pattern to handle non-sequential numbering
    const pbsHostKeys = Object.keys(process.env)
        .filter(key => key.match(/^PBS_HOST_\d+$/))
        .map(key => {
            const match = key.match(/^PBS_HOST_(\d+)$/);
            return match ? parseInt(match[1]) : null;
        })
        .filter(num => num !== null)
        .sort((a, b) => a - b);
    
    for (const pbsIndex of pbsHostKeys) {
        const pbsResult = loadPbsConfig(pbsIndex);
        if (pbsResult.config) {
            pbsConfigs.push(pbsResult.config);
        }
    }

    if (pbsConfigs.length > 0) {
        // console.log(`INFO: Loaded configuration for ${pbsConfigs.length} PBS instances.`);
    } else {
        // console.log("INFO: No PBS instances configured.");
    }

    // --- Final Validation ---
    const enabledEndpoints = endpoints.filter(e => e.enabled);
    if (enabledEndpoints.length === 0 && pbsConfigs.length === 0) {
         // Throw error instead of exiting
        throw new ConfigurationError('\n--- Configuration Error ---\nNo enabled Proxmox VE or PBS endpoints could be configured. Please check your .env file and environment variables.\n');
    }

    // console.log('INFO: Configuration loaded successfully.');
    
    // Load update channel preference
    const updateChannel = getUpdateChannelPreference();
    
    // Return the flag along with endpoints and pbsConfigs
    return { endpoints, pbsConfigs, isConfigPlaceholder, updateChannel };
}

module.exports = { loadConfiguration, getUpdateChannelPreference, ConfigurationError }; // Export the function and error class
