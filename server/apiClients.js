const axios = require('axios');
const https = require('https');
const axiosRetry = require('axios-retry').default;
const ResilientApiClient = require('./resilientApiClient');
const dnsResolver = require('./dnsResolver');
const { createLogger } = require('./utils/logger');
const { TIMEOUTS, RETRY_CONFIG } = require('./config/constants');

const logger = createLogger('ApiClients');

/**
 * Creates a request interceptor for PVE API authentication.
 * @param {Object} endpoint - The PVE endpoint configuration.
 * @returns {Function} - An Axios request interceptor function.
 */
function createPveAuthInterceptor(endpoint) {
  return config => {
    if (endpoint.tokenId && endpoint.tokenSecret) {
      config.headers.Authorization = `PVEAPIToken=${endpoint.tokenId}=${endpoint.tokenSecret}`;
    } else {
      // Error condition for missing credentials
      console.error(`ERROR: Endpoint ${endpoint.name} is missing required API token credentials.`);
    }
    return config;
  };
}

/**
 * Logs a warning and calculates exponential delay for PVE retries.
 * @param {string} endpointName - The name of the PVE endpoint.
 * @param {number} retryCount - The current retry attempt number.
 * @param {Error} error - The error that caused the retry.
 * @returns {number} - The delay in milliseconds.
 */
function pveRetryDelayLogger(endpointName, retryCount, error) {
  console.warn(`Retrying PVE API request for ${endpointName} (attempt ${retryCount}) due to error: ${error.message}`);
  return axiosRetry.exponentialDelay(retryCount);
}

/**
 * Checks if an error warrants retrying a PVE API call.
 * @param {Error} error - The error object.
 * @returns {boolean} - True if the request should be retried, false otherwise.
 */
function pveRetryConditionChecker(error) {
  return (
    axiosRetry.isNetworkError(error) ||
    axiosRetry.isRetryableError(error) ||
    error.response?.status === 596 // Specific PVE status code
  );
}

/**
 * Initializes Axios clients for Proxmox VE endpoints.
 * @param {Array} endpoints - Array of PVE endpoint configuration objects.
 * @returns {Object} - Object containing initialized PVE API clients keyed by endpoint ID.
 */
function initializePveClients(endpoints) {
  const apiClients = {};
  console.log(`INFO: Initializing API clients for ${endpoints.length} Proxmox VE endpoints...`);

  endpoints.forEach(endpoint => {
    if (!endpoint.enabled) {
      console.log(`INFO: Skipping disabled PVE endpoint: ${endpoint.name} (${endpoint.host}`);
      return; // Skip disabled endpoints
    }

    // Build base URL - only add port if it's explicitly configured and not empty
    let baseURL;
    if (endpoint.host.includes('://')) {
      // Host already has protocol, use as-is
      baseURL = `${endpoint.host}/api2/json`;
    } else {
      // Add https:// protocol
      baseURL = `https://${endpoint.host}`;
      // Only add port if it's explicitly set and not empty
      if (endpoint.port && endpoint.port !== '') {
        baseURL += `:${endpoint.port}`;
      }
      baseURL += '/api2/json';
    }

    const authInterceptor = createPveAuthInterceptor(endpoint);
    const retryConfig = {
      retryDelayLogger: pveRetryDelayLogger.bind(null, endpoint.name),
      retryConditionChecker: pveRetryConditionChecker,
    };
    
    // Enable resilient DNS handling if specified in endpoint config or if host ends with .lan
    const useResilientDns = endpoint.useResilientDns || endpoint.host.includes('.lan');
    
    const apiClient = createApiClientInstance(baseURL, endpoint.allowSelfSignedCerts, authInterceptor, retryConfig, useResilientDns);

    apiClients[endpoint.id] = { client: apiClient, config: endpoint };
    console.log(`INFO: Initialized PVE API client for endpoint: ${endpoint.name} (${endpoint.host})${useResilientDns ? ' with resilient DNS' : ''}`);
  });

  return apiClients;
}

// Generic function to create an Axios API client instance
function createApiClientInstance(baseURL, allowSelfSignedCerts, authInterceptor, retryConfig, useResilientClient = false) {
  const baseConfig = {
    baseURL: baseURL,
    timeout: 30000, // 30 second timeout
    httpsAgent: new https.Agent({
      rejectUnauthorized: !allowSelfSignedCerts
    }),
    headers: {
      'Content-Type': 'application/json'
    }
  };

  if (useResilientClient) {
    const hostname = dnsResolver.extractHostname(baseURL);
    if (hostname && !hostname.match(/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/)) {
      console.log(`[ApiClients] Creating resilient client for hostname: ${hostname}`);
      return new ResilientApiClient(baseConfig, authInterceptor);
    }
  }

  // Standard axios client
  const apiClient = axios.create(baseConfig);

  if (authInterceptor) {
    apiClient.interceptors.request.use(authInterceptor);
  }

  if (retryConfig) {
    axiosRetry(apiClient, {
      retries: retryConfig.retries || 3,
      retryDelay: retryConfig.retryDelayLogger,
      retryCondition: retryConfig.retryConditionChecker,
    });
  }
  return apiClient;
}

/**
 * Logs a warning and calculates exponential delay for PBS retries.
 * @param {string} configName - The name of the PBS configuration.
 * @param {number} retryCount - The current retry attempt number.
 * @param {Error} error - The error that caused the retry.
 * @returns {number} - The delay in milliseconds.
 */
function pbsRetryDelayLogger(configName, retryCount, error) {
  console.warn(`Retrying PBS API request for ${configName} (Token Auth - attempt ${retryCount}) due to error: ${error.message}`);
  return axiosRetry.exponentialDelay(retryCount);
}

/**
 * Checks if an error warrants retrying a PBS API call.
 * @param {Error} error - The error object.
 * @returns {boolean} - True if the request should be retried, false otherwise.
 */
function pbsRetryConditionChecker(error) {
  return axiosRetry.isNetworkError(error) || axiosRetry.isRetryableError(error);
}

/**
 * Creates a request interceptor for PBS API authentication (Token Auth).
 * @param {Object} config - The PBS configuration object.
 * @returns {Function} - An Axios request interceptor function.
 */
function createPbsAuthInterceptor(config) {
  return reqConfig => {
    reqConfig.headers.Authorization = `PBSAPIToken=${config.tokenId}:${config.tokenSecret}`;
    return reqConfig;
  };
}

/**
 * Initializes Axios clients for Proxmox Backup Server instances.
 * @param {Array} pbsConfigs - Array of PBS configuration objects.
 * @returns {Promise<Object>} - Promise resolving to an object containing initialized PBS API clients keyed by config ID.
 */
async function initializePbsClients(pbsConfigs) {
  const pbsApiClients = {};
  if (pbsConfigs.length === 0) {
      console.log("INFO: No PBS instances configured, skipping PBS client initialization.");
      return pbsApiClients;
  }

  console.log(`INFO: Initializing API clients for ${pbsConfigs.length} PBS instances...`);
  const initPromises = pbsConfigs.map(async (config) => {
      // Skip disabled PBS configurations
      if (!config.enabled) {
          console.log(`INFO: Skipping disabled PBS endpoint: ${config.name} (${config.host}`);
          return;
      }
      
      let clientData = null;
      try {
          if (config.authMethod === 'token') {
              // Build base URL - only add port if it's explicitly configured and not empty
              let pbsBaseURL;
              if (config.host.includes('://')) {
                  // Host already has protocol, use as-is
                  pbsBaseURL = `${config.host}/api2/json`;
              } else {
                  // Add https:// protocol
                  pbsBaseURL = `https://${config.host}`;
                  // Only add port if it's explicitly set and not empty
                  if (config.port && config.port !== '') {
                      pbsBaseURL += `:${config.port}`;
                  }
                  pbsBaseURL += '/api2/json';
              }
              
              const authInterceptor = createPbsAuthInterceptor(config);
              const retryConfig = {
                retryDelayLogger: pbsRetryDelayLogger.bind(null, config.name),
                retryConditionChecker: pbsRetryConditionChecker,
              };

              // Enable resilient DNS handling if specified in config or if host ends with .lan
              const useResilientDns = config.useResilientDns || config.host.includes('.lan');
              
              const pbsAxiosInstance = createApiClientInstance(pbsBaseURL, config.allowSelfSignedCerts, authInterceptor, retryConfig, useResilientDns);
              
              // IMPORTANT: Remove Content-Type header for PBS GET requests
              // PBS has a bug where it ignores namespace parameter when Content-Type is set on GET requests
              pbsAxiosInstance.interceptors.request.use(request => {
                  if (request.method?.toLowerCase() === 'get') {
                      delete request.headers['Content-Type'];
                  }
                  return request;
              });
              
              clientData = { client: pbsAxiosInstance, config: config };
              console.log(`INFO: [PBS Init] Successfully initialized client for instance '${config.name}' (Token Auth)${useResilientDns ? ' with resilient DNS' : ''}`);
          } else {
              console.error(`ERROR: Unexpected authMethod '${config.authMethod}' found during PBS client initialization for: ${config.name}`);
          }

          if (clientData) {
              pbsApiClients[config.id] = clientData;
          }
      } catch (error) {
          console.error(`ERROR: Unhandled exception during PBS client initialization for ${config.name}: ${error.message}`, error.stack);
      }
      // We don't return clientData here, we modify pbsApiClients directly
  });

  await Promise.allSettled(initPromises);
  console.log(`INFO: [PBS Init] Finished initialization. ${Object.keys(pbsApiClients).length} / ${pbsConfigs.length} PBS clients initialized successfully.`);
  return pbsApiClients;
}

/**
 * Initializes all Proxmox VE and PBS API clients.
 * @param {Array} endpoints - Array of PVE endpoint configuration objects.
 * @param {Array} pbsConfigs - Array of PBS configuration objects.
 * @returns {Promise<Object>} - Promise resolving to an object containing { apiClients, pbsApiClients }.
 */
async function initializeApiClients(endpoints, pbsConfigs) {
    const apiClients = initializePveClients(endpoints);
    const pbsApiClients = await initializePbsClients(pbsConfigs); // Wait for PBS clients to initialize
    return { apiClients, pbsApiClients };
}

// Export the new helper functions for potential direct testing
module.exports = {
  initializeApiClients,
  createPveAuthInterceptor,
  createPbsAuthInterceptor,
  pveRetryDelayLogger,
  pveRetryConditionChecker,
  pbsRetryDelayLogger,
  pbsRetryConditionChecker,
  createApiClientInstance,
};
