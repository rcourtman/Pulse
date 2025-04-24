// require('dotenv').config(); // Load environment variables from .env file

// --- BEGIN Environment Variable Validation ---
const primaryRequiredEnvVars = [
  'PROXMOX_HOST',
  'PROXMOX_TOKEN_ID',
  'PROXMOX_TOKEN_SECRET'
];
const placeholderValues = [
  'your-proxmox-ip-or-hostname',
  'your-api-token-id@pam!your-token-name',
  'your-api-token-secret-uuid',
  'your-password' // Added just in case password fallback is used without token
];

let missingVars = [];
let placeholderVars = [];

// Check primary vars
primaryRequiredEnvVars.forEach(varName => {
  const value = process.env[varName];
  if (!value) {
    missingVars.push(varName);
  } else if (placeholderValues.some(placeholder => value.includes(placeholder))) {
    placeholderVars.push(varName);
  }
});

if (missingVars.length > 0 || placeholderVars.length > 0) {
  console.error('\n--- Configuration Error (Primary Endpoint) ---');
  if (missingVars.length > 0) {
    console.error(`Missing required environment variables: ${missingVars.join(', ')}. These are typically set via docker-compose.yml or a .env file.`);
  }
  if (placeholderVars.length > 0) {
    console.error(`The following primary environment variables seem to contain placeholder values: ${placeholderVars.join(', ')}.`);
  }
  console.error('Please ensure valid Proxmox connection details are provided via environment variables.');
  console.error('Refer to server/.env.example for the required variable names and format.\n');
  process.exit(1); // Exit if primary configuration is invalid
}

// --- Load All Proxmox Endpoint Configurations ---
const endpoints = [];

// Load primary endpoint (index 0)
endpoints.push({
  id: 'primary', // Identifier for this endpoint
  name: process.env.PROXMOX_NODE_NAME || process.env.PROXMOX_HOST, // Use host if name not set
  host: process.env.PROXMOX_HOST,
  port: process.env.PROXMOX_PORT || '8006',
  tokenId: process.env.PROXMOX_TOKEN_ID,
  tokenSecret: process.env.PROXMOX_TOKEN_SECRET,
  enabled: process.env.PROXMOX_ENABLED !== 'false', // Currently unused, but kept for potential future use
  allowSelfSignedCerts: process.env.PROXMOX_ALLOW_SELF_SIGNED_CERTS !== 'false',
  credentials: { // Keep password fallback possibility
    username: process.env.PROXMOX_USERNAME,
    password: process.env.PROXMOX_PASSWORD,
    realm: process.env.PROXMOX_REALM || 'pam'
  }
});

// Load additional endpoints (PROXMOX_HOST_2, _3, ...)
let i = 2;
while (process.env[`PROXMOX_HOST_${i}`]) {
  const host = process.env[`PROXMOX_HOST_${i}`];
  const tokenId = process.env[`PROXMOX_TOKEN_ID_${i}`];
  const tokenSecret = process.env[`PROXMOX_TOKEN_SECRET_${i}`];

  // Minimal validation for additional endpoints
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
    id: `endpoint_${i}`, // Unique ID for this endpoint
    name: process.env[`PROXMOX_NODE_NAME_${i}`] || host, // Use host if name not set
    host: host,
    port: process.env[`PROXMOX_PORT_${i}`] || '8006',
    tokenId: tokenId,
    tokenSecret: tokenSecret,
    enabled: process.env[`PROXMOX_ENABLED_${i}`] !== 'false',
    allowSelfSignedCerts: process.env[`PROXMOX_ALLOW_SELF_SIGNED_CERTS_${i}`] !== 'false',
    credentials: { // Allow fallback for additional endpoints too
      username: process.env[`PROXMOX_USERNAME_${i}`],
      password: process.env[`PROXMOX_PASSWORD_${i}`],
      realm: process.env[`PROXMOX_REALM_${i}`] || 'pam'
    }
  });
  i++;
}

if (endpoints.length > 1) {
    console.log(`INFO: Loaded configuration for ${endpoints.length} Proxmox endpoints.`);
}

// --- END Configuration Loading ---

const fs = require('fs'); // Add fs module
const express = require('express');
const http = require('http');
const path = require('path');
const cors = require('cors');
const { Server } = require('socket.io');
const axios = require('axios');
const https = require('https');
const axiosRetry = require('axios-retry').default; // Import axios-retry

// Development specific dependencies
let chokidar;
if (process.env.NODE_ENV === 'development') {
  try {
    chokidar = require('chokidar');
  } catch (e) {
    console.warn('chokidar is not installed. Hot reload requires chokidar: npm install --save-dev chokidar');
  }
}

// --- Create API Clients for Each Endpoint ---
const apiClients = {}; // Use an object to store clients, keyed by endpoint.id
let pbsApiClient = null; // Initialize PBS client as null
let pbsConfig = null; // Initialize PBS config as null

// --- Load PBS Configuration (if provided) ---
// Check for User/Password first
if (process.env.PBS_HOST && process.env.PBS_USER && process.env.PBS_PASSWORD) {
    const pbsHost = process.env.PBS_HOST;
    const pbsUser = process.env.PBS_USER;
    const pbsPassword = process.env.PBS_PASSWORD;
    const pbsRealm = process.env.PBS_REALM || 'pbs'; // Default realm 'pbs'

    // Basic placeholder check for PBS user/pass vars
    const pbsPlaceholders = placeholderValues.filter(p =>
        pbsHost.includes(p) || pbsUser.includes(p) || pbsPassword.includes(p)
    );

    if (pbsPlaceholders.length > 0) {
        console.warn(`WARN: Skipping PBS configuration. The following variables seem to contain placeholder values: ${pbsPlaceholders.join(', ')}`);
    } else {
        pbsConfig = {
            id: 'pbs_primary_userpass',
            authMethod: 'userpass', // Indicate auth method
            name: process.env.PBS_NODE_NAME || pbsHost,
            host: pbsHost,
            port: process.env.PBS_PORT || '8007',
            user: pbsUser,
            password: pbsPassword,
            realm: pbsRealm,
            nodeName: process.env.PBS_NODE_NAME,
            allowSelfSignedCerts: process.env.PBS_ALLOW_SELF_SIGNED_CERTS !== 'false',
            enabled: true
        };
        console.log(`INFO: Found PBS configuration (User/Password): ${pbsConfig.name} (${pbsConfig.host})`);
    }
// Check for Token second (fallback)
} else if (process.env.PBS_HOST && process.env.PBS_TOKEN_ID && process.env.PBS_TOKEN_SECRET) {
    const pbsHost = process.env.PBS_HOST;
    const pbsTokenId = process.env.PBS_TOKEN_ID;
    const pbsTokenSecret = process.env.PBS_TOKEN_SECRET;

    // Basic placeholder check for PBS token vars
    const pbsPlaceholders = placeholderValues.filter(p =>
        pbsHost.includes(p) || pbsTokenId.includes(p) || pbsTokenSecret.includes(p)
    );

    if (pbsPlaceholders.length > 0) {
        console.warn(`WARN: Skipping PBS configuration (Token). The following variables seem to contain placeholder values: ${pbsPlaceholders.join(', ')}`);
    } else {
        pbsConfig = {
            id: 'pbs_primary_token',
            authMethod: 'token', // Indicate auth method
            name: process.env.PBS_NODE_NAME || pbsHost,
            host: pbsHost,
            port: process.env.PBS_PORT || '8007',
            tokenId: pbsTokenId,
            tokenSecret: pbsTokenSecret,
            nodeName: process.env.PBS_NODE_NAME,
            allowSelfSignedCerts: process.env.PBS_ALLOW_SELF_SIGNED_CERTS !== 'false',
            enabled: true
        };
        console.log(`INFO: Found PBS configuration (API Token): ${pbsConfig.name} (${pbsConfig.host})`);
    }
} else if (process.env.PBS_HOST || process.env.PBS_TOKEN_ID || process.env.PBS_TOKEN_SECRET || process.env.PBS_USER || process.env.PBS_PASSWORD) {
    // Warn if some but not all required PBS vars are set
    console.warn("WARN: Partial PBS configuration found. Please set PBS_HOST and either (PBS_TOKEN_ID + PBS_TOKEN_SECRET) or (PBS_USER + PBS_PASSWORD) to enable PBS monitoring.");
}
// --- End PBS Configuration Loading ---

endpoints.forEach(endpoint => {
  if (!endpoint.enabled) {
    console.log(`INFO: Skipping disabled endpoint: ${endpoint.name} (${endpoint.host})`);
    return; // Skip disabled endpoints
  }

  const baseURL = endpoint.host.includes('://')
    ? `${endpoint.host}/api2/json`
    : `https://${endpoint.host}:${endpoint.port}/api2/json`;

  const apiClient = axios.create({
    baseURL: baseURL,
    httpsAgent: new https.Agent({
      rejectUnauthorized: !endpoint.allowSelfSignedCerts
    }),
    headers: {
      'Content-Type': 'application/json'
    }
  });

  // Add request interceptor for authentication (specific to this endpoint)
  apiClient.interceptors.request.use(config => {
    // Add API token authentication
    if (endpoint.tokenId && endpoint.tokenSecret) {
      config.headers.Authorization = `PVEAPIToken=${endpoint.tokenId}=${endpoint.tokenSecret}`;
    }
    // Fallback to password auth if configured FOR THIS ENDPOINT
    else if (endpoint.credentials && endpoint.credentials.username && endpoint.credentials.password) {
      const { username, password, realm } = endpoint.credentials;
      config.headers.Authorization = `Basic ${Buffer.from(`${username}@${realm}:${password}`).toString('base64')}`;
      if (!realm) console.warn(`WARN: Using password auth for endpoint ${endpoint.name} without a realm specified. Defaulting to 'pam'.`);
    } else {
        // Should not happen due to validation, but log just in case
        console.error(`ERROR: Endpoint ${endpoint.name} has neither token nor full username/password credentials configured.`);
    }
    return config;
  });

  // Apply retry logic to the axios instance
  axiosRetry(apiClient, {
    retries: 3, // Number of retries
    retryDelay: (retryCount, error) => {
      console.warn(`Retrying API request for ${endpoint.name} (attempt ${retryCount}) due to error: ${error.message}`);
      return axiosRetry.exponentialDelay(retryCount); // Exponential backoff
    },
    retryCondition: (error) => {
      // Retry on network errors or specific status codes
      return (
        axiosRetry.isNetworkError(error) ||
        axiosRetry.isRetryableError(error) || // Includes 5xx errors by default
        error.response?.status === 596 // Specifically retry on 596
      );
    },
  });

  apiClients[endpoint.id] = { client: apiClient, config: endpoint }; // Store both client and config
  console.log(`INFO: Initialized API client for endpoint: ${endpoint.name} (${endpoint.host})`);
});

// --- Create PBS API Client (if configured) ---
async function getPbsAuthTicketAndSetupClient(config) {
    const pbsBaseURL = config.host.includes('://')
        ? `${config.host}` // Assume full URL if :// is present
        : `https://${config.host}:${config.port}`;

    const loginUrl = `${pbsBaseURL}/api2/json/access/ticket`;
    const httpsAgent = new https.Agent({
        rejectUnauthorized: !config.allowSelfSignedCerts
    });

    try {
        console.log(`INFO: Attempting PBS login for user ${config.user}...`);
        const response = await axios.post(loginUrl, `username=${encodeURIComponent(config.user)}&password=${encodeURIComponent(config.password)}`, {
            httpsAgent: httpsAgent,
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
        });

        const ticket = response.data.data.ticket;
        const csrfToken = response.data.data.CSRFPreventionToken;
        const username = response.data.data.username;

        if (!ticket || !csrfToken) {
            throw new Error("Login response missing ticket or CSRF token.");
        }

        console.log(`INFO: PBS login successful for ${username}. Setting up authenticated client.`);

        // Create the authenticated client instance
        const pbsAuthenticatedClient = axios.create({
            baseURL: `${pbsBaseURL}/api2/json`,
            httpsAgent: httpsAgent,
            headers: {
                'Content-Type': 'application/json'
            }
        });

        // Apply interceptor for cookie and CSRF token
        pbsAuthenticatedClient.interceptors.request.use(reqConfig => {
            reqConfig.headers['CSRFPreventionToken'] = csrfToken;
            reqConfig.headers['Cookie'] = `PBSAuthCookie=${ticket}`;
            return reqConfig;
        });

        // Apply retry logic
        axiosRetry(pbsAuthenticatedClient, {
            retries: 3,
            retryDelay: (retryCount, error) => {
                console.warn(`Retrying authenticated PBS API request (attempt ${retryCount}) due to error: ${error.message}`);
                return axiosRetry.exponentialDelay(retryCount);
            },
            retryCondition: (error) => {
                // Also retry on 401/403 in case ticket expires or CSRF mismatch?
                // For now, just standard retry conditions.
                return axiosRetry.isNetworkError(error) || axiosRetry.isRetryableError(error);
            },
        });

        return { client: pbsAuthenticatedClient, config: config }; // Return the fully configured client object

    } catch (error) {
        console.error(`ERROR: PBS login failed for user ${config.user}: ${error.message}`);
        if (error.response) {
            console.error(`ERROR: PBS login response status: ${error.response.status}`);
            console.error(`ERROR: PBS login response data: ${JSON.stringify(error.response.data)}`);
        }
        return null; // Indicate failure
    }
}

async function initializePbsClient() {
    if (pbsConfig) {
        if (pbsConfig.authMethod === 'userpass') {
            pbsApiClient = await getPbsAuthTicketAndSetupClient(pbsConfig);
            if (pbsApiClient) {
                console.log(`INFO: Initialized API client for PBS (User/Password Auth): ${pbsConfig.name} (${pbsConfig.host})`);
            } else {
                console.error("ERROR: Failed to initialize PBS client using User/Password.");
            }
        } else if (pbsConfig.authMethod === 'token') {
            // Original Token Auth Logic
            const pbsBaseURL = pbsConfig.host.includes('://')
                ? `${pbsConfig.host}/api2/json`
                : `https://${pbsConfig.host}:${pbsConfig.port}/api2/json`;

            const pbsAxiosInstance = axios.create({
                baseURL: pbsBaseURL,
                httpsAgent: new https.Agent({
                    rejectUnauthorized: !pbsConfig.allowSelfSignedCerts
                }),
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            pbsAxiosInstance.interceptors.request.use(config => {
                config.headers.Authorization = `PBSAPIToken ${pbsConfig.tokenId}:${pbsConfig.tokenSecret}`;
                return config;
            });

            axiosRetry(pbsAxiosInstance, {
                retries: 3,
                retryDelay: (retryCount, error) => {
                    console.warn(`Retrying PBS API request (Token Auth - attempt ${retryCount}) due to error: ${error.message}`);
                    return axiosRetry.exponentialDelay(retryCount);
                },
                retryCondition: (error) => {
                    return axiosRetry.isNetworkError(error) || axiosRetry.isRetryableError(error);
                },
            });

            pbsApiClient = { client: pbsAxiosInstance, config: pbsConfig };
            console.log(`INFO: Initialized API client for PBS (Token Auth): ${pbsConfig.name} (${pbsConfig.host})`);
        }
    }
}

if (Object.keys(apiClients).length === 0 && !pbsApiClient) {
    console.error("\n--- Configuration Error ---");
    console.error("No enabled Proxmox VE or PBS endpoints could be configured. Please check your .env file and environment variables.");
    process.exit(1);
}
// --- End API Client Creation ---

// Server configuration
const DEBUG_METRICS = false; // Set to true to show detailed metrics logs
const PORT = 7655; // Using a different port from the main server

// --- Define Update Intervals ---
// How often to fetch dynamic metrics (CPU, Mem, IO) for running guests
const METRIC_UPDATE_INTERVAL = 2000; // Default: 2 seconds
// How often to fetch structural data (node list, guest lists, node status)
const DISCOVERY_UPDATE_INTERVAL = 30000; // Default: 30 seconds

// Create Express app
const app = express();

// Middleware
app.use(cors());
app.use(express.json());
app.use(express.static(path.join(__dirname, '../src/public')));

// Allow iframe embedding
app.use((req, res, next) => {
  res.removeHeader('X-Frame-Options'); // Remove default Express header if present
  res.setHeader('Content-Security-Policy', "frame-ancestors *"); // Allow embedding from any origin
  next();
});

// --- Add API endpoint for version ---
let appVersion = 'unknown';
try {
  const packageJsonPath = path.join(__dirname, '../package.json');
  const packageJsonContent = fs.readFileSync(packageJsonPath, 'utf8');
  const packageJson = JSON.parse(packageJsonContent);
  appVersion = packageJson.version || 'unknown';
} catch (error) {
  console.error('Error reading version from package.json:', error.message);
}

app.get('/api/version', (req, res) => {
  res.json({ version: appVersion });
});
// --- End API endpoint ---

// --- Add API endpoint for Storage ---
app.get('/api/storage', async (req, res) => {
  // Refactored to query all configured endpoints
  const aggregatedStorageData = {};
  let hasDataFromAnyEndpoint = false;

  const endpointIds = Object.keys(apiClients);
  if (endpointIds.length === 0) {
    console.warn('/api/storage: No API clients configured.');
    return res.json({}); // Return empty if no clients
  }

  // Process each endpoint concurrently
  const endpointPromises = endpointIds.map(async (endpointId) => {
    const { client: apiClient, config: endpointConfig } = apiClients[endpointId];
    const endpointName = endpointConfig.name || endpointId; // For logging
    const endpointStorageData = {}; // Storage data specifically for this endpoint
    let nodesToQuery = [];
    let discoveryFailed = false;

    // --- Step 1: Discover nodes for this endpoint ---
    try {
      const nodesResponse = await apiClient.get('/nodes');
      const basicNodeInfo = nodesResponse.data.data || [];
      nodesToQuery = basicNodeInfo.map(n => n.node).filter(Boolean);

      if (nodesToQuery.length === 0) {
        console.warn(`/api/storage (${endpointName}): /nodes returned 0 nodes. Attempting single node discovery.`);
        try {
          const versionResponse = await apiClient.get('/version');
          const discoveredNodeName = versionResponse.data.data.node;
          if (discoveredNodeName) {
            nodesToQuery = [discoveredNodeName];
          } else {
            throw new Error("Could not determine node name from /version.");
          }
        } catch (discoveryError) {
          console.error(`/api/storage (${endpointName}): Single node discovery failed: ${discoveryError.message}`);
          discoveryFailed = true;
        }
      }
    } catch (error) {
      console.warn(`/api/storage (${endpointName}): Failed to fetch /nodes (${error.message}). Attempting single node discovery.`);
      try {
        const versionResponse = await apiClient.get('/version');
        const discoveredNodeName = versionResponse.data.data.node;
        if (discoveredNodeName) {
          nodesToQuery = [discoveredNodeName];
          console.log(`/api/storage (${endpointName}): Discovered single node via /version: ${discoveredNodeName}`);
        } else {
          throw new Error("Could not determine node name from /version.");
        }
      } catch (discoveryError) {
        console.error(`/api/storage (${endpointName}): Single node discovery failed after /nodes error: ${discoveryError.message}`);
        discoveryFailed = true;
      }
    }

    // If discovery failed for this endpoint, log and skip to the next endpoint
    if (discoveryFailed) {
      console.error(`/api/storage (${endpointName}): Failed to discover any nodes. Skipping storage fetch for this endpoint.`);
      return null; // Indicate failure for this endpoint
    }

    // --- Step 2: Fetch storage for discovered nodes in this endpoint --- 
    const storagePromises = nodesToQuery.map(async (nodeName) => {
      if (!nodeName) {
        console.warn(`/api/storage (${endpointName}): Skipping node with missing name.`);
        return; // Skip if node name is somehow invalid
      }
      try {
        const response = await apiClient.get(`/nodes/${nodeName}/storage`);
        // Add the fetched storage data under the nodeName key
        // Note: If node names collide across endpoints, the last one processed will overwrite previous ones.
        endpointStorageData[nodeName] = response.data.data || [];
      } catch (err) {
        console.error(`/api/storage (${endpointName}): Error fetching storage for node ${nodeName}: ${err.message}`);
        // Store an error object instead of data for this node
        endpointStorageData[nodeName] = { 
            error: `Failed to fetch storage: ${err.message}`, 
            endpointId: endpointId, 
            endpointName: endpointName 
        };
      }
    });

    await Promise.allSettled(storagePromises);

    // If we got any data (even errors) for nodes in this endpoint, return it
    if (Object.keys(endpointStorageData).length > 0) {
      // console.log(`/api/storage (${endpointName}): Fetched storage data for ${Object.keys(endpointStorageData).length} nodes.`);
      return endpointStorageData;
    } else if (nodesToQuery.length > 0) {
      console.warn(`/api/storage (${endpointName}): Discovered ${nodesToQuery.length} nodes but failed to fetch storage for any of them.`);
      return null; // Indicate failure if discovery worked but storage fetch failed completely
    } else {
      // Should only happen if discovery found 0 nodes initially
      console.log(`/api/storage (${endpointName}): No nodes discovered, no storage data fetched.`);
      return null; 
    }
  }); // End map over endpointIds

  // Process results from all endpoints
  const allEndpointResults = await Promise.allSettled(endpointPromises);

  allEndpointResults.forEach(endpointOutcome => {
    if (endpointOutcome.status === 'fulfilled' && endpointOutcome.value) {
      // Merge the storage data from this endpoint into the aggregated result
      Object.assign(aggregatedStorageData, endpointOutcome.value);
      hasDataFromAnyEndpoint = true; // Mark that we got some data
    } else if (endpointOutcome.status === 'rejected') {
      // Log errors from processing an endpoint promise itself (less likely)
      console.error(`/api/storage: Error processing endpoint storage fetch promise: ${endpointOutcome.reason}`);
    }
    // We already logged errors for endpoints where value is null inside the map
  });

  if (hasDataFromAnyEndpoint) {
     if (DEBUG_METRICS) {
        console.log(`/api/storage: Returning aggregated storage data for ${Object.keys(aggregatedStorageData).length} nodes across all endpoints.`);
     }
  } else {
     console.warn(`/api/storage: Failed to fetch any storage data from any configured endpoint.`);
  }
  
  res.json(aggregatedStorageData); // Return the aggregated data (or empty object if all failed)
});
// --- End Storage API endpoint ---

// Create HTTP server
const server = http.createServer(app);

// Create Socket.IO server with CORS configuration
const io = new Server(server, {
  cors: {
    origin: "*",
    methods: ["GET", "POST"]
  }
});

// --- Global State Variables ---
// These will hold the latest fetched data
let currentNodes = [];
let currentVms = [];
let currentContainers = [];
let currentMetrics = [];
let pbsData = { tasks: {}, datastores: [], nodeName: null, status: 'initializing' }; // Re-add global pbsData
let isDiscoveryRunning = false; // Prevent concurrent discovery runs
let isMetricsRunning = false;   // Prevent concurrent metric runs
let discoveryTimeoutId = null;
let metricTimeoutId = null;
// --- End Global State ---

// Helper function to fetch data for a single node WITHIN a specific endpoint
// Added apiClient and endpointId parameters
async function fetchDataForNode(apiClient, endpointId, nodeName) {
  const nodeData = {
    vms: [],
    containers: [],
    metrics: []
  };

  try {
    // Fetch VMs
    const vmsResponse = await apiClient.get(`/nodes/${nodeName}/qemu`);
    if (vmsResponse.data.data && Array.isArray(vmsResponse.data.data)) {
      // Add endpointId, node, and type to each VM
      nodeData.vms = vmsResponse.data.data.map(vm => ({ 
          ...vm, 
          node: nodeName, 
          endpointId: endpointId, 
          type: 'qemu' // **** ADD TYPE ****
      }));

      // Collect metrics for running VMs in parallel
      const vmMetricPromises = nodeData.vms
        .filter(vm => vm.status === 'running')
        .map(async (vm) => {
          try {
            const [rrdData, currentData] = await Promise.all([
              apiClient.get(`/nodes/${nodeName}/qemu/${vm.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
              apiClient.get(`/nodes/${nodeName}/qemu/${vm.vmid}/status/current`)
            ]);

            let metricData = {
              id: vm.vmid,
              guestName: vm.name,
              node: nodeName,
              type: 'qemu',
              endpointId: endpointId,
              data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
            return metricData; // Return successful metric data
          } catch (err) {
            // Add status code to error log
            const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
            console.error(`[Metrics] Failed to get metrics for VM ${vm.vmid} on node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
            return null; // Return null on error for this specific VM
          }
        });
      const vmMetricsResults = await Promise.allSettled(vmMetricPromises);
      vmMetricsResults.forEach(result => {
          if (result.status === 'fulfilled' && result.value) {
              nodeData.metrics.push(result.value);
          }
          // Optionally log rejected promises if needed:
          // else if (result.status === 'rejected') { console.error(...) }
      });
    }
  } catch (err) {
    // Add status code to error log
    const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
    console.error(`[Discovery] Error fetching VMs from node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
    // Continue to fetch containers even if VMs fail
  }

  try {
    // Fetch containers
    const ctsResponse = await apiClient.get(`/nodes/${nodeName}/lxc`);
    if (ctsResponse.data.data && Array.isArray(ctsResponse.data.data)) {
       // Add endpointId, node, and type to each container
      nodeData.containers = ctsResponse.data.data.map(ct => ({ 
          ...ct, 
          node: nodeName, 
          endpointId: endpointId, 
          type: 'lxc' // **** ADD TYPE ****
      }));

      // Collect metrics for running containers in parallel
      const ctMetricPromises = nodeData.containers
        .filter(ct => ct.status === 'running')
        .map(async (ct) => {
          try {
            const [rrdData, currentData] = await Promise.all([
               apiClient.get(`/nodes/${nodeName}/lxc/${ct.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
               apiClient.get(`/nodes/${nodeName}/lxc/${ct.vmid}/status/current`)
            ]);

            let metricData = {
              id: ct.vmid,
              guestName: ct.name,
              node: nodeName,
              type: 'lxc',
              endpointId: endpointId,
              data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
             return metricData; // Return successful metric data
          } catch (err) {
            // Add status code to error log
            const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
            console.error(`[Metrics] Failed to get metrics for container ${ct.vmid} on node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
            return null; // Return null on error for this specific container
          }
        });

       const ctMetricsResults = await Promise.allSettled(ctMetricPromises);
        ctMetricsResults.forEach(result => {
            if (result.status === 'fulfilled' && result.value) {
                nodeData.metrics.push(result.value);
            }
             // Optionally log rejected promises if needed
        });
    }
  } catch (err) {
    // Add status code to error log
    const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
    console.error(`[Discovery] Error fetching containers from node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
  }

  // Return collected data (VMs, Containers) for this node from this endpoint
  // Metrics are handled separately in fetchMetricsData
  return {
      vms: nodeData.vms,
      containers: nodeData.containers
  };
}

// --- Refactored Data Fetching Logic ---

/**
 * Fetches structural data: node list, node statuses, VM list, Container list
 * across ALL configured endpoints.
 */
async function fetchDiscoveryData() {
  console.log("[Discovery Cycle] Starting fetch across all endpoints...");
  let aggregatedResult = { nodes: [], vms: [], containers: [], pbs: pbsData || {} }; // Initialize with current PBS data

  // --- PVE Data Fetching ---
  const pveEndpointIds = Object.keys(apiClients).filter(id => !apiClients[id].isPbs); // Exclude PBS client if present

  // Initialize temporary accumulators for PVE data
  let tempNodes = [];
  let tempVms = [];
  let tempContainers = [];

  if (pveEndpointIds.length > 0) {
      const pvePromises = pveEndpointIds.map(endpointId =>
          (async () => {
              const { client: apiClient, config } = apiClients[endpointId];
              const endpointName = config.name || endpointId; // Use configured name or ID
              console.log(`[Discovery Cycle] Fetching PVE discovery data for endpoint: ${endpointName}`);
              try {
                  // Get all nodes for the endpoint
                  const nodesResponse = await apiClient.get('/nodes');
                  const nodes = nodesResponse.data.data; // Assuming structure { data: { data: [...] } }
                  if (!nodes || !Array.isArray(nodes)) {
                      console.warn(`[Discovery Cycle - ${endpointName}] No nodes found or unexpected format.`);
                      return { endpointId: endpointName, status: 'fulfilled', value: { nodes: [], vms: [], containers: [] } }; // Return empty structure on node failure
                  }

                  // Fetch VMs and Containers for each node in parallel
                  const guestPromises = nodes.map(node => fetchDataForNode(apiClient, endpointName, node.node)); // Pass endpointName

                  const guestResults = await Promise.allSettled(guestPromises);

                  let endpointNodes = [];
                  let endpointVms = [];
                  let endpointContainers = [];

                  // Process node info first
                   nodes.forEach(nodeInfo => {
                      endpointNodes.push({
                          ...nodeInfo,
                          endpointId: endpointName, // Add endpointId/name
                          id: `${endpointName}-${nodeInfo.node}` // Create unique ID
                      });
                  });


                  // Process results for VMs and Containers
                  guestResults.forEach(result => {
                      if (result.status === 'fulfilled' && result.value) {
                           // Add endpointId to each vm and container
                           result.value.vms.forEach(vm => {
                               vm.endpointId = endpointId; // Use the internal endpointId, not endpointName
                               vm.id = `${endpointName}-${vm.node}-${vm.vmid}`; // Keep unique ID using endpointName
                           });
                           result.value.containers.forEach(ct => {
                               ct.endpointId = endpointId; // Use the internal endpointId, not endpointName
                               ct.id = `${endpointName}-${ct.node}-${ct.vmid}`; // Keep unique ID using endpointName
                           });
                          endpointVms.push(...result.value.vms);
                          endpointContainers.push(...result.value.containers);
                      } else if (result.status === 'rejected') {
                          // Log node-specific failure if needed, but continue processing others
                          console.error(`[Discovery Cycle - ${endpointName}] Failed fetching guest data for a node: ${result.reason?.message || result.reason}`);
                      }
                  });
                  console.log(`[Discovery Cycle - ${endpointName}] Completed. Found: ${endpointNodes.length} nodes, ${endpointVms.length} VMs, ${endpointContainers.length} containers.`);
                  // Return combined results for this endpoint
                  return { endpointId: endpointName, status: 'fulfilled', value: { nodes: endpointNodes, vms: endpointVms, containers: endpointContainers } };
              } catch (error) {
                  const status = error.response?.status ? ` (Status: ${error.response.status})` : '';
                  console.error(`[Discovery Cycle - ${endpointName}] Error fetching discovery data${status}: ${error.message}`);
                  // Return a rejected status for this specific endpoint promise
                   // Ensure 'reason' has necessary details if possible
                  return { endpointId: endpointName, status: 'rejected', reason: error.message || String(error) };
              }
          })() // Immediately invoke the async function
      );

      const pveOutcomes = await Promise.allSettled(pvePromises);

      // Process results from all PVE endpoints
      pveOutcomes.forEach(endpointOutcome => {
          if (endpointOutcome.status === 'fulfilled' && endpointOutcome.value.status === 'fulfilled' && endpointOutcome.value.value) {
              // Successfully fetched data for this endpoint
              const { nodes, vms, containers } = endpointOutcome.value.value;
              const successfulEndpointId = endpointOutcome.value.endpointId; // Use the returned endpointId/name
              // console.log(`[Discovery Cycle] Accumulating data from endpoint: ${successfulEndpointId}`); // Debug log
              tempNodes.push(...nodes);
              tempVms.push(...vms);
              tempContainers.push(...containers);
          } else {
              // Handle endpoint failures (either outer promise rejected or inner fetch failed)
              const failedEndpointId = endpointOutcome.status === 'fulfilled' ? endpointOutcome.value.endpointId : endpointOutcome.reason?.endpointId || 'Unknown Endpoint'; // Try to get endpoint ID
              const reason = endpointOutcome.status === 'rejected'
                  ? (endpointOutcome.reason?.message || endpointOutcome.reason)
                  : (endpointOutcome.value.reason || 'Unknown error'); // Reason from inner failure
              console.error(`[Discovery Cycle] Failed PVE discovery for endpoint: ${failedEndpointId}. Reason: ${reason}`);
          }
          // Deprecated logging, keep for now if needed:
          /* else if (endpointOutcome.status === 'rejected') { // Outer promise rejected
              // Extract endpoint ID if possible from error or context if you modify the promise creation
              const failedEndpointId = 'Unknown Endpoint'; // Placeholder - needs better error handling context
              const reason = endpointOutcome.reason?.message || endpointOutcome.reason;
              console.error(`[Discovery Cycle] Failed to process PVE endpoint discovery promise for ${failedEndpointId}: ${reason}`);
          } else if (endpointOutcome.value.status !== 'fulfilled'){ // Catches inner rejections
               console.error(`[Discovery Cycle] Failed PVE discovery for endpoint: ${endpointOutcome.value.endpointId}`);
          } */
      });

  } else {
      console.log("[Discovery Cycle] No PVE endpoints configured.");
  }

  // --- Fetch PBS Data (if configured) ---
  let newPbsData = null; // Use null to indicate fetch status
  if (pbsApiClient) {
      console.log("INFO: Fetching PBS discovery data...");
      let pbsNodeName = pbsApiClient.config.nodeName; // Use configured name if available
      // Ensure pbsApiClient.client exists before trying to use it
      if (!pbsApiClient.client) {
          console.error("ERROR: pbsApiClient.client is not initialized!");
      } else {
          if (!pbsNodeName) {
              pbsNodeName = await fetchPbsNodeName(pbsApiClient); // Pass the whole object
              // Store detected name back in config for future use within this run
              if (pbsNodeName && pbsNodeName !== 'localhost') {
                  pbsApiClient.config.nodeName = pbsNodeName;
              }
          }

          try {
              const [tasks, datastores] = await Promise.all([
                  fetchPbsTaskData(pbsApiClient, pbsNodeName), // Pass the whole object
                  fetchPbsDatastoreData(pbsApiClient) // Pass the whole object
              ]);
              // Assign fetched data to the newPbsData object *only on success*
              newPbsData = { tasks, datastores, nodeName: pbsNodeName, status: 'ok' };
              console.log("INFO: Finished fetching PBS discovery data successfully.");
          } catch (pbsError) {
              console.error(`ERROR: Failed fetching PBS data during discovery cycle: ${pbsError.message}`);
              // Do NOT update pbsData here, let it keep the old value
              // newPbsData remains null to indicate failure of this fetch attempt
               if (pbsError.response?.status === 401) {
                  console.error("ERROR: PBS API Authentication Expired or Invalid (401). Re-login needed.");
                  // Future improvement: Trigger re-login here
               }
          }
      }
  } else {
       console.log("[Discovery Cycle] No PBS client configured.");
       // Set specific status if unconfigured
       newPbsData = { tasks: {}, datastores: [], nodeName: null, status: 'unconfigured' };
  }
  // --- End Fetch PBS Data ---

  // --- Update Global State ---
  // Only update pbsData if the fetch was successful (newPbsData is not null)
  if (newPbsData !== null) {
    pbsData = newPbsData;
  }
  // Always update PVE data using the accumulated temporary variables
  aggregatedResult.nodes = tempNodes;
  aggregatedResult.vms = tempVms;
  aggregatedResult.containers = tempContainers;

  // Add the *final, potentially updated* pbsData state to the result to be emitted
  aggregatedResult.pbs = pbsData;

  if (DEBUG_METRICS) {
    console.log(`[Discovery Cycle] Aggregated PVE results. Total: ${aggregatedResult.nodes.length} nodes, ${aggregatedResult.vms.length} VMs, ${aggregatedResult.containers.length} containers across ${pveEndpointIds.length} configured PVE endpoints.`);
  }
  console.log('INFO: Discovery cycle completed.');
  return aggregatedResult;
}

/**
 * Fetches dynamic metric data ONLY for currently known running VMs and Containers
 * across ALL configured endpoints.
 */
async function fetchMetricsData(runningVms, runningContainers) {
    console.log(`[Metrics Cycle] Starting metrics fetch for ${runningVms.length} VMs, ${runningContainers.length} Containers...`);
    const allMetrics = [];
    const metricPromises = [];

    // Group running guests by endpointId and then by nodeName
    const guestsByEndpointNode = {};

    [...runningVms, ...runningContainers].forEach(guest => {
        if (!guest.endpointId || !guest.node || !guest.vmid || !guest.type) {
            console.warn(`[Metrics Cycle] Skipping guest with missing info:`, guest);
            return;
        }
        const { endpointId, node, vmid, type, name } = guest; // Include name for logging

        if (!guestsByEndpointNode[endpointId]) {
            guestsByEndpointNode[endpointId] = {};
        }
        if (!guestsByEndpointNode[endpointId][node]) {
            guestsByEndpointNode[endpointId][node] = [];
        }
        // Store minimal info needed for fetch, ensuring name exists
        guestsByEndpointNode[endpointId][node].push({ vmid, type, name: name || 'unknown' }); 
    });

    // Iterate through endpoints that have running guests
    for (const endpointId in guestsByEndpointNode) {
        if (!apiClients[endpointId]) {
            console.warn(`[Metrics Cycle] API client for endpoint ${endpointId} not found. Skipping metrics fetch for its guests.`);
            continue;
        }
        const { client: apiClient, config: endpointConfig } = apiClients[endpointId];
        const endpointName = endpointConfig.name || endpointId;

        // Iterate through nodes within this endpoint that have running guests
        for (const nodeName in guestsByEndpointNode[endpointId]) {
            const guestsOnNode = guestsByEndpointNode[endpointId][nodeName];

            // Create promises for fetching metrics for guests on this specific node
            guestsOnNode.forEach(guestInfo => {
                const { vmid, type, name: guestName } = guestInfo;
                metricPromises.push(
                    (async () => {
                        try {
                            const pathPrefix = type === 'qemu' ? 'qemu' : 'lxc';
                            const [rrdData, currentData] = await Promise.all([
                                apiClient.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
                                apiClient.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/status/current`)
                            ]);

                            const metricData = {
                                id: vmid,
                                guestName: guestName, // Keep guest name
                                node: nodeName,
                                type: type,
                                endpointId: endpointId, // Add endpointId
                                endpointName: endpointName, // Add readable name
                                data: rrdData?.data?.data?.length > 0 ? rrdData.data.data : [],
                                current: currentData?.data?.data || null
                            };
                            return metricData;
                        } catch (err) {
                            // Log error but don't crash the whole cycle
                             // Add status code to error log
                            const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
                            // Check if error is due to guest being stopped (400 Bad Request often indicates this for status/current)
                            if (err.response && err.response.status === 400) {
                                console.warn(`[Metrics Cycle - ${endpointName}] Guest ${type} ${vmid} (${guestName}) on node ${nodeName} might be stopped or inaccessible (Status: 400). Skipping metrics.`);
                            } else {
                                console.error(`[Metrics Cycle - ${endpointName}] Failed to get metrics for ${type} ${vmid} (${guestName}) on node ${nodeName}${status}: ${err.message}`);
                            }
                            return null; // Return null on error for this specific guest
                        }
                    })() // Immediately invoke the async function
                );
            }); // End foreach guest on node
        } // End foreach node in endpoint
    } // End foreach endpoint

    // Wait for all metric fetch promises to settle
    const metricResults = await Promise.allSettled(metricPromises);

    metricResults.forEach(result => {
        if (result.status === 'fulfilled' && result.value) {
            allMetrics.push(result.value);
        }
        // Optional: Log rejected promises if needed (errors are already logged individually)
        // else if (result.status === 'rejected') { console.error(...) }
    });

    if (DEBUG_METRICS) {
        console.log(`[Metrics Cycle] Completed. Fetched metrics for ${allMetrics.length} running guests.`);
    }
    return allMetrics;
}

// --- Socket.io connection handling (Initial data fetch needs update) ---
io.on('connection', (socket) => {
  console.log(`[socket] Client connected. Total clients: ${io.engine.clientsCount}`);
  
  // Send initial data immediately if available
  if (currentNodes.length > 0 || currentVms.length > 0 || currentContainers.length > 0) {
    console.log('[socket] Sending existing data to new client.');
    socket.emit('rawData', {
        nodes: currentNodes,
        vms: currentVms,
        containers: currentContainers,
        metrics: currentMetrics
    });
  } else {
    // If no data yet, trigger a discovery cycle (if not already running)
    console.log('[socket] No data yet, triggering initial discovery for new client...');
    if (!isDiscoveryRunning) {
        runDiscoveryCycle(); 
    }
  }
  
  // Handle disconnect
  socket.on('disconnect', () => {
    setTimeout(() => {
        console.log(`[socket] Client disconnected. Total clients: ${io.engine.clientsCount}`);
        // Optional: Stop polling if client count drops to 0? (Handled in run cycles)
    }, 100);
  });
});

// --- New Update Cycle Logic ---

// Discovery Cycle Runner
async function runDiscoveryCycle() {
  if (isDiscoveryRunning) {
    // console.log('[Discovery Cycle] Already running, skipping.');
    return;
  }
  isDiscoveryRunning = true;

  try {
    const discoveryData = await fetchDiscoveryData(); // This now returns {nodes, vms, containers, pbs}
    
    // Update global state variables from the returned data
    currentNodes = discoveryData.nodes || []; // Use fallback for safety
    currentVms = discoveryData.vms || [];
    currentContainers = discoveryData.containers || [];
    // Only update pbsData if it's present in the result (handles initial state)
    if (discoveryData.pbs) { 
      pbsData = discoveryData.pbs; 
    } else {
      // If fetchDiscoveryData somehow didn't return pbs, keep old state or default
      pbsData = pbsData || { tasks: {}, datastores: [], nodeName: null, status: 'error' }; 
    }

    // Emit combined data using the updated global variables
    if (io.engine.clientsCount > 0) {
        if (DEBUG_METRICS) {
             console.log('[Discovery Cycle] Emitting updated structural data including PBS.');
        }
        io.emit('rawData', {
            nodes: currentNodes,
            vms: currentVms,
            containers: currentContainers,
            metrics: currentMetrics, // Use the current global metrics state
            pbs: pbsData          // Use the updated global pbs state
        });
    }
  } catch (error) {
      console.error(`[Discovery Cycle] Error during execution: ${error.message}`, error.stack);
  } finally {
      isDiscoveryRunning = false;
      // Schedule the next discovery cycle
      scheduleNextDiscovery();
  }
}

// Metric Cycle Runner
async function runMetricCycle() {
  if (isMetricsRunning) {
    // console.log('[Metrics Cycle] Already running, skipping.');
    return;
  }
  // Only run if clients are connected
  if (io.engine.clientsCount === 0) {
    // console.log('[Metrics Cycle] No clients connected, skipping fetch.');
    scheduleNextMetric(); // Still schedule next check
    return;
  }
  
  isMetricsRunning = true;

  try {
    // Filter for running guests based on the MOST RECENT state from discovery
    const runningVms = currentVms.filter(vm => vm.status === 'running');
    const runningContainers = currentContainers.filter(ct => ct.status === 'running');

    if (runningVms.length > 0 || runningContainers.length > 0) {
        // Fetch metrics using the dedicated function
        const fetchedMetrics = await fetchMetricsData(runningVms, runningContainers);
        currentMetrics = fetchedMetrics; // Update global metrics state
        // Emit combined data
        // console.log('[Metrics Cycle] Emitting updated metrics data.'); // Reduced verbosity
        io.emit('rawData', {
            nodes: currentNodes, // Send current nodes/vms/cts state
            vms: currentVms,
            containers: currentContainers,
            metrics: currentMetrics // Send the newly fetched metrics
        });
    } else {
        // console.log('[Metrics Cycle] No running guests found, skipping metric fetch.');
        currentMetrics = []; // Clear metrics if no guests running
         // Emit state update even if metrics were cleared
        io.emit('rawData', {
            nodes: currentNodes, vms: currentVms,
            containers: currentContainers, metrics: currentMetrics
        });
    }

  } catch (error) {
      console.error(`[Metrics Cycle] Error during execution: ${error.message}`, error.stack);
  } finally {
      isMetricsRunning = false;
      // Schedule the next metric cycle
      scheduleNextMetric();
  }
}

// Schedulers using setTimeout
function scheduleNextDiscovery() {
  if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
  discoveryTimeoutId = setTimeout(runDiscoveryCycle, DISCOVERY_UPDATE_INTERVAL);
}

function scheduleNextMetric() {
  if (metricTimeoutId) clearTimeout(metricTimeoutId);
  metricTimeoutId = setTimeout(runMetricCycle, METRIC_UPDATE_INTERVAL);
}

// Start the initial cycles
console.log('Starting initial data fetch cycles...');
runDiscoveryCycle(); // Run discovery first
// Metrics will be triggered after discovery or by its own timer if clients connect later
scheduleNextMetric(); // Start scheduling metrics right away

// --- End New Update Cycle Logic ---

// --- PBS Data Fetching Functions ---

async function fetchPbsNodeName(pbsClient) {
    // Attempts to fetch the node name from the PBS API
    try {
        const response = await pbsClient.client.get('/nodes');
        if (response.data && response.data.data && response.data.data.length > 0) {
            // Assuming the first node listed is the one we're connected to
            const nodeName = response.data.data[0].node;
            console.log(`INFO: Detected PBS node name: ${nodeName}`);
            return nodeName;
        } else {
            console.warn("WARN: Could not automatically detect PBS node name from API. Response format unexpected.", response.data);
            return 'localhost'; // Fallback
        }
    } catch (error) {
        console.error(`ERROR: Failed to fetch PBS nodes list: ${error.message}`);
        return 'localhost'; // Fallback on error
    }
}

async function fetchPbsTaskData(pbsClient, nodeName) {
    // Fetches recent backup task history from PBS
    if (!nodeName) {
        console.warn("WARN: Cannot fetch PBS task data without node name.");
        return { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } };
    }
    try {
        // Fetch tasks from the last 7 days (adjust as needed)
        const sinceTimestamp = Math.floor((Date.now() - 7 * 24 * 60 * 60 * 1000) / 1000);
        const response = await pbsClient.client.get(`/nodes/${nodeName}/tasks`, {
            params: {
                // typefilter: 'backup', // Filter specifically for backup tasks if needed - type seems inconsistent?
                since: sinceTimestamp,
                limit: 200, // Limit the number of tasks fetched
                errors: 1 // Include tasks with errors
            }
        });

        let tasks = response.data && response.data.data ? response.data.data : [];
        // Filter further if needed - e.g., task 'type' might be 'backup', 'sync', 'gc', 'verify'
        // We might only care about 'backup' type for this summary
        tasks = tasks.filter(task => task.type === 'backup' || task.worker_type === 'backup'); // Adjust filter based on actual API response

        let okCount = 0;
        let failedCount = 0;
        let lastOkTimestamp = 0;
        let lastFailedTimestamp = 0;

        tasks.forEach(task => {
            if (task.status === 'OK') {
                okCount++;
                if (task.endtime > lastOkTimestamp) lastOkTimestamp = task.endtime;
            } else if (task.status && task.status !== 'running') { // Count non-OK, non-running tasks as failed
                failedCount++;
                if (task.endtime > lastFailedTimestamp) lastFailedTimestamp = task.endtime;
            }
        });

        console.log(`INFO: Fetched ${tasks.length} PBS backup tasks. OK: ${okCount}, Failed: ${failedCount}`);

        return {
            recentTasks: tasks.slice(0, 20), // Return only the latest 20 for potential UI display
            summary: {
                ok: okCount,
                failed: failedCount,
                total: tasks.length,
                lastOk: lastOkTimestamp || null,
                lastFailed: lastFailedTimestamp || null,
            }
        };

    } catch (error) {
        console.error(`ERROR: Failed to fetch PBS task data for node ${nodeName}: ${error.message}`);
        if (error.response?.status === 401) {
             console.error("ERROR: PBS API authentication failed (401). Check PBS_TOKEN_ID and PBS_TOKEN_SECRET.");
        } else if (error.response?.status === 403) {
            console.error("ERROR: PBS API authorization failed (403). Check token permissions.");
        }
        // Return empty data on error
        return { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } };
    }
}

async function fetchPbsDatastoreData(pbsClient) {
    // Fetches datastore usage details from PBS using the /status/datastore-usage endpoint
    let datastores = [];
    try {
        // Fetch usage stats for all datastores at once
        const usageResponse = await pbsClient.client.get('/status/datastore-usage');
        const usageData = usageResponse.data && usageResponse.data.data ? usageResponse.data.data : [];

        if (usageData.length > 0) {
            console.log(`INFO: Fetched status for ${usageData.length} PBS datastores via /status/datastore-usage.`);
            // Map the received data to the expected format
            datastores = usageData.map(ds => ({
                name: ds.store, // Field name might be 'store' or 'name'
                path: ds.path || 'N/A', // Path might not be directly in usage stats
                total: ds.total,
                used: ds.used,
                available: ds.avail, // Field name might be 'avail' or 'available'
                // Add other relevant fields if available, e.g., gc status
                gcStatus: ds['garbage-collection-status'] || 'unknown'
            }));
        } else {
            console.warn("WARN: PBS /status/datastore-usage returned empty data. Falling back to /config/datastore.");
            // Fallback to fetching config if usage endpoint is empty
            throw new Error("Empty data from /status/datastore-usage");
        }

    } catch (usageError) {
        console.error(`ERROR: Failed to fetch PBS datastore usage via /status/datastore-usage: ${usageError.message}. Trying fallback /config/datastore.`);
        // --- Fallback Logic --- 
        try {
            const configResponse = await pbsClient.client.get('/config/datastore');
            const datastoresConfig = configResponse.data && configResponse.data.data ? configResponse.data.data : [];
            if (datastoresConfig.length > 0) {
                console.log(`INFO: Fetched config for ${datastoresConfig.length} PBS datastores (fallback). Status unavailable.`);
                 datastores = datastoresConfig.map(dsConfig => ({
                    name: dsConfig.name,
                    path: dsConfig.path,
                    total: null,
                    used: null,
                    available: null,
                    gcStatus: 'unknown (config only)'
                }));
            }
        } catch (configError) {
            console.error(`ERROR: Fallback fetch of PBS datastore config failed: ${configError.message}`);
             if (configError.response?.status === 401 || configError.response?.status === 403) {
                console.error("ERROR: PBS API authentication/authorization failed for datastore config access.");
             }
             // Keep datastores as empty array if both attempts fail
        }
        // --- End Fallback --- 
    }

    return datastores;
}

// --- END PBS Data Fetching Functions ---

// Start the server
async function startServer() {
    await initializePbsClient(); // Initialize PBS client (might fail)

    server.listen(PORT, () => {
        console.log(`Server listening on port ${PORT}`);
        
        // Setup hot reload in development mode
        if (process.env.NODE_ENV === 'development' && chokidar) {
          const publicPath = path.join(__dirname, '../src/public');
          console.log(`Watching for changes in ${publicPath}`);
          const watcher = chokidar.watch(publicPath, { 
            ignored: /(^|[\\\/])\./, // ignore dotfiles
            persistent: true,
            ignoreInitial: true // Don't trigger on initial scan
          });
          
          watcher.on('change', (filePath) => {
            // console.log(`File changed: ${filePath}. Triggering hot reload.`);
            io.emit('hotReload'); // Notify clients to reload
          });
          
          watcher.on('error', error => console.error(`Watcher error: ${error}`));
        }
    });
}

startServer();