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
    
    // Check for persistent config directory (Docker) or use project root
    const configDir = path.join(__dirname, '../config');
    const configEnvPath = path.join(configDir, '.env');
    const projectEnvPath = path.join(__dirname, '../.env');
    
    let updateChannel = 'stable'; // Default value
    let envFilePath = null;
    
    // Check config/.env first (Docker persistent config), then fallback to .env
    if (fs.existsSync(configEnvPath)) {
        envFilePath = configEnvPath;
    } else if (fs.existsSync(projectEnvPath)) {
        envFilePath = projectEnvPath;
    }
    
    if (envFilePath) {
        try {
            const configContent = fs.readFileSync(envFilePath, 'utf8');
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
    const portVar = `PBS_PORT${suffix}`;
    const selfSignedVar = `PBS_ALLOW_SELF_SIGNED_CERTS${suffix}`;
    const enabledVar = `PBS_ENABLED${suffix}`;
    const resilientDnsVar = `PBS_RESILIENT_DNS${suffix}`;
    const namespaceVar = `PBS_NAMESPACE${suffix}`;
    const namespaceAutoVar = `PBS_NAMESPACE_AUTO${suffix}`;
    const namespaceIncludeVar = `PBS_NAMESPACE_INCLUDE${suffix}`;
    const namespaceExcludeVar = `PBS_NAMESPACE_EXCLUDE${suffix}`;

    const pbsHostUrl = process.env[hostVar];
    if (!pbsHostUrl) {
        return false; // No more PBS configs if PBS_HOST is missing
    }

    let pbsHostname = pbsHostUrl;
    try {
        const parsedUrl = new URL(pbsHostUrl);
        pbsHostname = parsedUrl.hostname;
    } catch (e) {
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
                name: pbsHostname,
                host: pbsHostUrl,
                port: process.env[portVar] || (pbsHostUrl && pbsHostUrl.includes('://') ? '' : '8007'),
                tokenId: pbsTokenId,
                tokenSecret: pbsTokenSecret,
                namespace: process.env[namespaceVar] || '', // Default to root namespace if not specified
                namespaces: process.env[namespaceVar] ? process.env[namespaceVar].split(',').map(ns => ns.trim()).filter(ns => ns !== undefined) : null, // Support comma-separated namespaces, null for auto-discovery
                allowSelfSignedCerts: process.env[selfSignedVar] !== 'false',
                enabled: process.env[enabledVar] !== 'false',
                useResilientDns: process.env[resilientDnsVar] === 'true'
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

    function createProxmoxEndpointConfig(idPrefix, index, hostEnv, portEnv, tokenIdEnv, tokenSecretEnv, enabledEnv, selfSignedEnv) {
        const host = process.env[hostEnv];
        const tokenId = process.env[tokenIdEnv];
        const tokenSecret = process.env[tokenSecretEnv];

        if (index !== null && (!tokenId || !tokenSecret)) {
            return null;
        }
        if (index !== null && placeholderValues.some(p => host.includes(p) || tokenId.includes(p) || tokenSecret.includes(p))) {
            return null;
        }
        
        // Check for resilient DNS configuration
        const resilientDnsEnv = index ? `PROXMOX_RESILIENT_DNS_${index}` : 'PROXMOX_RESILIENT_DNS';
        const useResilientDns = process.env[resilientDnsEnv] === 'true';
        
        return {
            id: index ? `${idPrefix}_${index}` : idPrefix,
            name: null, // No explicit names needed
            host: host,
            port: process.env[portEnv] || (host && host.includes('://') ? '' : '8006'),
            tokenId: tokenId,
            tokenSecret: tokenSecret,
            enabled: process.env[enabledEnv] !== 'false',
            allowSelfSignedCerts: process.env[selfSignedEnv] !== 'false',
            useResilientDns: useResilientDns,
        };
    }

    const primaryEndpoint = createProxmoxEndpointConfig(
        'primary', 
        null, // No index for primary
        'PROXMOX_HOST', 
        'PROXMOX_PORT', 
        'PROXMOX_TOKEN_ID', 
        'PROXMOX_TOKEN_SECRET', 
        'PROXMOX_ENABLED', 
        'PROXMOX_ALLOW_SELF_SIGNED_CERTS'
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
            `PROXMOX_ALLOW_SELF_SIGNED_CERTS_${i}`
        );
        if (additionalEndpoint) {
            endpoints.push(additionalEndpoint);
        }
    }

    if (endpoints.length > 1) {
    }

    // --- Load All PBS Configurations ---
    const pbsConfigs = [];
    // Load primary PBS config
    const primaryPbsResult = loadPbsConfig();