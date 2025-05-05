const { URL } = require('url');

// Align placeholder values with install script
const placeholderValues = [
  'https://proxmox_host:8006', // Match install script
  'user@pam!tokenid',        // Match install script
  'YOUR_API_SECRET_HERE'     // Match install script
];

// Error class for configuration issues
class ConfigurationError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ConfigurationError';
  }
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
        console.warn(`WARN: Could not parse PBS_HOST URL "${pbsHostUrl}". Using full value as fallback name.`);
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
             console.log(`INFO: Found PBS configuration ${index || 'primary'} (API Token): ${config.name} (${config.host})`);
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
      require('dotenv').config();
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
        } else if (placeholderValues.some(placeholder => value.includes(placeholder))) {
            placeholderVars.push(varName);
        }
    });

    // Throw error only if required vars are MISSING
    if (missingVars.length > 0) {
        let errorMessages = ['--- Configuration Error (Primary Endpoint) ---'];
        errorMessages.push(`Missing required environment variables: ${missingVars.join(', ')}.`);
        errorMessages.push('Please ensure valid Proxmox connection details are provided.');
        errorMessages.push('Refer to server/.env.example for the required variable names and format.');
        throw new ConfigurationError(errorMessages.join('\n'));
    }

    // Set the flag if placeholders were detected (but don't throw error)
    if (placeholderVars.length > 0) {
        isConfigPlaceholder = true;
        console.warn(`WARN: Primary Proxmox environment variables seem to contain placeholder values: ${placeholderVars.join(', ')}. Pulse may not function correctly until configured.`);
    }

    // --- Load All Proxmox Endpoint Configurations ---
    const endpoints = [];

    // Load primary endpoint (index 0)
    endpoints.push({
        id: 'primary',
        name: process.env.PROXMOX_NODE_NAME || process.env.PROXMOX_HOST,
        host: process.env.PROXMOX_HOST,
        port: process.env.PROXMOX_PORT || '8006',
        tokenId: process.env.PROXMOX_TOKEN_ID,
        tokenSecret: process.env.PROXMOX_TOKEN_SECRET,
        enabled: process.env.PROXMOX_ENABLED !== 'false',
        allowSelfSignedCerts: process.env.PROXMOX_ALLOW_SELF_SIGNED_CERTS !== 'false',
    });

    // Load additional Proxmox endpoints
    let i = 2;
    while (process.env[`PROXMOX_HOST_${i}`]) {
        const host = process.env[`PROXMOX_HOST_${i}`];
        const tokenId = process.env[`PROXMOX_TOKEN_ID_${i}`];
        const tokenSecret = process.env[`PROXMOX_TOKEN_SECRET_${i}`];

        if (!tokenId || !tokenSecret) {
            console.warn(`WARN: Skipping endpoint ${i} (Host: ${host}). Missing PROXMOX_TOKEN_ID_${i} or PROXMOX_TOKEN_SECRET_${i}.`);
            i++;
            continue;
        }
        if (placeholderValues.some(p => host.includes(p) || tokenId.includes(p) || tokenSecret.includes(p))) {
            console.warn(`WARN: Skipping endpoint ${i} (Host: ${host}). Environment variables seem to contain placeholder values.`);
            i++;
            continue;
        }

        endpoints.push({
            id: `endpoint_${i}`,
            name: process.env[`PROXMOX_NODE_NAME_${i}`] || host,
            host: host,
            port: process.env[`PROXMOX_PORT_${i}`] || '8006',
            tokenId: tokenId,
            tokenSecret: tokenSecret,
            enabled: process.env[`PROXMOX_ENABLED_${i}`] !== 'false',
            allowSelfSignedCerts: process.env[`PROXMOX_ALLOW_SELF_SIGNED_CERTS_${i}`] !== 'false',
        });
        i++;
    }

    if (endpoints.length > 1) {
        console.log(`INFO: Loaded configuration for ${endpoints.length} Proxmox endpoints.`);
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
    let pbsIndex = 2;
    let pbsResult = loadPbsConfig(pbsIndex);
    while (pbsResult.found) { // Continue as long as a PBS_HOST_n was found
        if (pbsResult.config) {
            pbsConfigs.push(pbsResult.config);
        }
        pbsIndex++;
        pbsResult = loadPbsConfig(pbsIndex);
    }

    if (pbsConfigs.length > 0) {
        console.log(`INFO: Loaded configuration for ${pbsConfigs.length} PBS instances.`);
    } else {
        console.log("INFO: No PBS instances configured.");
    }

    // --- Final Validation ---
    const enabledEndpoints = endpoints.filter(e => e.enabled);
    if (enabledEndpoints.length === 0 && pbsConfigs.length === 0) {
         // Throw error instead of exiting
        throw new ConfigurationError('\n--- Configuration Error ---\nNo enabled Proxmox VE or PBS endpoints could be configured. Please check your .env file and environment variables.\n');
    }

    console.log('INFO: Configuration loaded successfully.');
    // Return the flag along with endpoints and pbsConfigs
    return { endpoints, pbsConfigs, isConfigPlaceholder };
}

module.exports = { loadConfiguration, ConfigurationError }; // Export the function and error class 