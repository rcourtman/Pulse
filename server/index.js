require('dotenv').config(); // Load environment variables from .env file

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
const { URL } = require('url'); // <--- ADD: Import URL constructor
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
const pbsConfigs = []; // Array to hold all parsed PBS configurations
const pbsApiClients = {}; // Object to hold initialized clients, keyed by pbsConfig.id

// --- Load PBS Configuration (if provided) ---
function loadPbsConfig(index = null) {
    const suffix = index ? `_${index}` : '';
    const hostVar = `PBS_HOST${suffix}`;
    const userVar = `PBS_USER${suffix}`;
    const passVar = `PBS_PASSWORD${suffix}`;
    const realmVar = `PBS_REALM${suffix}`;
    const tokenIdVar = `PBS_TOKEN_ID${suffix}`;
    const tokenSecretVar = `PBS_TOKEN_SECRET${suffix}`;
    const nodeNameVar = `PBS_NODE_NAME${suffix}`;
    const portVar = `PBS_PORT${suffix}`;
    const selfSignedVar = `PBS_ALLOW_SELF_SIGNED_CERTS${suffix}`;

    const pbsHostUrl = process.env[hostVar]; // Rename variable to reflect it's a URL
    if (!pbsHostUrl) {
        // No more PBS configs if PBS_HOST is missing
        return false; // Indicate no more configs found
    }

    // ---> ADDED: URL Parsing for Hostname Fallback <---\
    let pbsHostname = pbsHostUrl; // Default to full URL if parsing fails
    try {
        const parsedUrl = new URL(pbsHostUrl);
        pbsHostname = parsedUrl.hostname; // Extract just the hostname
    } catch (e) {
        console.warn(`WARN: Could not parse PBS_HOST URL "${pbsHostUrl}". Using full value as fallback name.`);
    }
    // ---> END ADDED <---\

    const pbsUser = process.env[userVar];
    const pbsPassword = process.env[passVar];
    const pbsTokenId = process.env[tokenIdVar];
    const pbsTokenSecret = process.env[tokenSecretVar];

    let config = null;
    let idPrefix = index ? `pbs_endpoint_${index}` : 'pbs_primary';

    // Check User/Password first
    if (pbsUser && pbsPassword) {
        const pbsPlaceholders = placeholderValues.filter(p =>
            pbsHostUrl.includes(p) || pbsUser.includes(p) || pbsPassword.includes(p) // Check against URL
        );
        if (pbsPlaceholders.length > 0) {
            console.warn(`WARN: Skipping PBS configuration ${index || 'primary'} (User/Pass). Placeholder values detected for: ${pbsPlaceholders.join(', ')}`);
        } else {
            config = {
                id: `${idPrefix}_userpass`,
                authMethod: 'userpass',
                name: process.env[nodeNameVar] || pbsHostname, // User-defined name or parsed hostname
                host: pbsHostUrl, // Keep original full URL here
                port: process.env[portVar] || '8007',
                user: pbsUser,
                password: pbsPassword,
                realm: process.env[realmVar] || 'pbs',
                nodeName: process.env[nodeNameVar], // Store explicitly set node name
                allowSelfSignedCerts: process.env[selfSignedVar] !== 'false',
                enabled: true // Assuming enabled if configured
            };
            console.log(`INFO: Found PBS configuration ${index || 'primary'} (User/Password): ${config.name} (${config.host})`);
        }
    }
    // Check Token second
    else if (pbsTokenId && pbsTokenSecret) {
        const pbsPlaceholders = placeholderValues.filter(p =>
            pbsHostUrl.includes(p) || pbsTokenId.includes(p) || pbsTokenSecret.includes(p) // Check against URL
        );
        if (pbsPlaceholders.length > 0) {
            console.warn(`WARN: Skipping PBS configuration ${index || 'primary'} (Token). Placeholder values detected for: ${pbsPlaceholders.join(', ')}`);
        } else {
            config = {
                id: `${idPrefix}_token`,
                authMethod: 'token',
                name: process.env[nodeNameVar] || pbsHostname,
                host: pbsHostUrl, // Keep original full URL here
                port: process.env[portVar] || '8007',
                tokenId: pbsTokenId,
                tokenSecret: pbsTokenSecret,
                nodeName: process.env[nodeNameVar],
                allowSelfSignedCerts: process.env[selfSignedVar] !== 'false',
                enabled: true
            };
             console.log(`INFO: Found PBS configuration ${index || 'primary'} (API Token): ${config.name} (${config.host})`);
        }
    }
    // Warn if host is set but auth is incomplete
    else {
         console.warn(`WARN: Partial PBS configuration found for ${hostVar}. Please set either (${userVar} + ${passVar}) or (${tokenIdVar} + ${tokenSecretVar}) along with ${hostVar}.`);
    }

    if (config) {
        pbsConfigs.push(config);
        return true; // Indicate a config was found and added
    }
    return true; // Indicate we should check the next index even if this one was partial/invalid
}

// Load primary PBS config (index=null)
loadPbsConfig();

// Load additional PBS configs (index=2, 3, ...)
let pbsIndex = 2;
while (loadPbsConfig(pbsIndex)) {
    pbsIndex++;
}

if (pbsConfigs.length > 0) {
    console.log(`INFO: Loaded configuration for ${pbsConfigs.length} PBS instances.`);
} else {
     console.log("INFO: No PBS instances configured.");
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

async function initializeAllPbsClients() {
    if (pbsConfigs.length === 0) return;

    console.log(`INFO: Initializing API clients for ${pbsConfigs.length} PBS instances...`);
    const initPromises = pbsConfigs.map(async (config) => {
        let clientData = null;
        try {
            if (config.authMethod === 'userpass') {
                clientData = await getPbsAuthTicketAndSetupClient(config);
                if (clientData) {
                    console.log(`INFO: [PBS Init] Successfully initialized client for instance '${config.name}' (User/Password Auth)`);
                } else {
                    console.error(`ERROR: Failed to initialize PBS client (User/Password) for: ${config.name} (${config.host})`);
                }
            } else if (config.authMethod === 'token') {
                // Token Auth Logic (adapted from old initializePbsClient)
                const pbsBaseURL = config.host.includes('://')
                    ? `${config.host}/api2/json`
                    : `https://${config.host}:${config.port}/api2/json`;

                const pbsAxiosInstance = axios.create({
                    baseURL: pbsBaseURL,
                    httpsAgent: new https.Agent({
                        rejectUnauthorized: !config.allowSelfSignedCerts
                    }),
                    headers: { 'Content-Type': 'application/json' }
                });

                pbsAxiosInstance.interceptors.request.use(reqConfig => {
                    reqConfig.headers.Authorization = `PBSAPIToken ${config.tokenId}:${config.tokenSecret}`;
                    return reqConfig;
                });

                axiosRetry(pbsAxiosInstance, {
                    retries: 3,
                    retryDelay: (retryCount, error) => {
                        console.warn(`Retrying PBS API request for ${config.name} (Token Auth - attempt ${retryCount}) due to error: ${error.message}`);
                        return axiosRetry.exponentialDelay(retryCount);
                    },
                    retryCondition: (error) => {
                        return axiosRetry.isNetworkError(error) || axiosRetry.isRetryableError(error);
                    },
                });

                clientData = { client: pbsAxiosInstance, config: config };
                 console.log(`INFO: [PBS Init] Successfully initialized client for instance '${config.name}' (Token Auth)`);
            } else {
                 console.error(`ERROR: Unknown authMethod '${config.authMethod}' for PBS config: ${config.name}`);
            }

            if (clientData) {
                pbsApiClients[config.id] = clientData; // Store successful client keyed by config ID
            }
        } catch (error) {
             console.error(`ERROR: Unhandled exception during PBS client initialization for ${config.name}: ${error.message}`);
        }
    });

    await Promise.allSettled(initPromises);
    console.log(`INFO: [PBS Init] Finished initialization. ${Object.keys(pbsApiClients).length} / ${pbsConfigs.length} PBS clients initialized successfully.`);
}

if (Object.keys(apiClients).length === 0 && pbsConfigs.length === 0) {
    console.error("\n--- Configuration Error ---");
    console.error("No enabled Proxmox VE or PBS endpoints could be configured. Please check your .env file and environment variables.");
    process.exit(1);
}
// --- End API Client Creation ---

// Server configuration
const DEBUG_METRICS = false; // Set to true to show detailed metrics logs
const PORT = 7655; // Using a different port from the main server

// --- Define Update Intervals (Configurable via Env Vars) ---
const METRIC_UPDATE_INTERVAL = parseInt(process.env.PULSE_METRIC_INTERVAL_MS, 10) || 2000; // Default: 2 seconds
const DISCOVERY_UPDATE_INTERVAL = parseInt(process.env.PULSE_DISCOVERY_INTERVAL_MS, 10) || 30000; // Default: 30 seconds

console.log(`INFO: Using Metric Update Interval: ${METRIC_UPDATE_INTERVAL}ms`);
console.log(`INFO: Using Discovery Update Interval: ${DISCOVERY_UPDATE_INTERVAL}ms`);

// Create Express app
const app = express();
const server = http.createServer(app); // Create HTTP server instance

// Middleware
app.use(cors());
app.use(express.json());

// Define the public directory path
const publicDir = path.join(__dirname, '../src/public');

// Serve static files (CSS, JS, images) from the public directory
app.use(express.static(publicDir, { index: false }));

// Route to serve the main HTML file for the root path
app.get('/', (req, res) => {
  const indexPath = path.join(publicDir, 'index.html');
  res.sendFile(indexPath, (err) => {
    if (err) {
      console.error(`Error sending index.html: ${err.message}`);
      // Avoid sending error details to the client for security
      res.status(err.status || 500).send('Internal Server Error loading page.');
    }
  });
});

// --- API Routes ---
// Example API Route (Add your actual API routes here)
app.get('/api/version', (req, res) => {
    try {
        const packageJson = require('../package.json');
        res.json({ version: packageJson.version || 'N/A' });
    } catch (error) {
         console.error("Error reading package.json for version:", error);
         res.status(500).json({ error: "Could not retrieve version" });
    }
});

app.get('/api/storage', async (req, res) => {
    try {
        // Transform currentNodes into the format expected by updateStorageInfo
        const storageInfoByNode = {};
        (currentNodes || []).forEach(node => {
            // Assuming storage details are fetched and stored within the node object
            // during the discovery cycle. Need to find where Proxmox stores this.
            // Use the `storage` property added by the updated `fetchDataForNode` function
            storageInfoByNode[node.node] = node.storage || []; // Use node name as key
            if (!node.storage) {
                // This warning should ideally not appear now unless the storage fetch itself failed
                console.warn(`[API /api/storage] No storage data found for node: ${node.node}. Sending empty array.`);
            }
        });
        res.json(storageInfoByNode); // Return the transformed object
    } catch (error) {
        console.error("Error in /api/storage:", error);
        res.status(500).json({ globalError: error.message || "Failed to fetch storage details." });
    }
});

// --- WebSocket Setup ---
const io = new Server(server, {
  // Optional: Configure CORS for Socket.IO if needed, separate from Express CORS
  cors: {
    origin: "*", // Allow all origins for Socket.IO, adjust as needed for security
    methods: ["GET", "POST"]
  }
});

io.on('connection', (socket) => {
  console.log('Client connected');
  // Immediately send current data if available
  // if (cachedDiscoveryData) {
  //   socket.emit('rawData', cachedDiscoveryData);
  // }

  socket.on('requestData', async () => {
    console.log('Client requested data');
    try {
      // Send the current known state immediately
      if (currentNodes.length > 0 || currentVms.length > 0 || currentContainers.length > 0 || pbsDataArray.length > 0) {
          socket.emit('rawData', {
              nodes: currentNodes,
              vms: currentVms,
              containers: currentContainers,
              metrics: currentMetrics,
              pbs: pbsDataArray
          });
      } else {
         console.log('No data available yet on request, waiting for next cycle.');
         // Optionally trigger an immediate discovery cycle?
         // runDiscoveryCycle(); // Be careful with triggering cycles on demand
      }
    } catch (error) {
      console.error('Error fetching data on client request:', error);
      // Notify client of error?
    }
  });

  socket.on('disconnect', () => {
    console.log('Client disconnected');
  });
});

// --- Global State Variables ---
// These will hold the latest fetched data
let currentNodes = [];
let currentVms = [];
let currentContainers = [];
let currentMetrics = [];
let pbsDataArray = []; // Array to hold data for each PBS instance
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
    metrics: [],
    nodeStatus: null, // Initialize node status object
    storage: [] // Initialize storage array
  };

  // Fetch node status ONLY (removed concurrent /cpu fetch)
  try {
    const statusResult = await apiClient.get(`/nodes/${nodeName}/status`);
    if (statusResult.data && statusResult.data.data) {
      nodeData.nodeStatus = statusResult.data.data;
    } else {
      console.warn(`[Discovery] Node status for ${nodeName} (Endpoint: ${endpointId}) was empty or malformed.`);
      nodeData.nodeStatus = {}; // Ensure nodeStatus is an object even on failure
    }
  } catch (err) {
    const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
    console.error(`[Discovery] Error fetching status for node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
    nodeData.nodeStatus = {}; // Ensure nodeStatus is an object even on failure
  }

  // ---> ADDED: Fetch Node Storage <---
  try {
    const storageResult = await apiClient.get(`/nodes/${nodeName}/storage`);
    if (storageResult.data && storageResult.data.data && Array.isArray(storageResult.data.data)) {
      nodeData.storage = storageResult.data.data; // Store storage array
    } else {
      console.warn(`[Discovery] Storage data for ${nodeName} (Endpoint: ${endpointId}) was empty or malformed.`);
      nodeData.storage = []; // Default to empty array on failure/malformed
    }
  } catch (err) {
    const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
    console.error(`[Discovery] Error fetching storage for node ${nodeName} (Endpoint: ${endpointId})${status}: ${err.message}`);
    nodeData.storage = []; // Default to empty array on error
  }
  // ---> END ADDED <---

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

  // Return collected data (VMs, Containers, Node Status) for this node
  // Metrics are handled separately
  return {
      vms: nodeData.vms,
      containers: nodeData.containers,
      nodeStatus: nodeData.nodeStatus, // Return node status
      storage: nodeData.storage // Return storage array
  };
}

// --- Refactored Data Fetching Logic ---

/**
 * Fetches structural data: node list, node statuses, VM list, Container list
 * across ALL configured endpoints.
 */
async function fetchDiscoveryData() {
  console.log("[Discovery Cycle] Starting fetch across all endpoints...");
  let aggregatedResult = { nodes: [], vms: [], containers: [], pbs: [] }; // pbs is now an array

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

                  // Process node info first, merging status later
                   nodes.forEach(nodeInfo => {
                      endpointNodes.push({
                          ...nodeInfo,
                          endpointId: endpointName, // Add endpointId/name
                          id: `${endpointName}-${nodeInfo.node}`, // Create unique ID
                          // Initialize status fields, using maxcpu from nodeInfo if available
                          cpu: null,
                          maxcpu: nodeInfo.maxcpu || null, // <-- Use maxcpu from initial call
                          mem: null,
                          maxmem: nodeInfo.maxmem || null, // <-- Also use maxmem from initial call
                          disk: null,
                          maxdisk: null,
                          uptime: 0, // Default uptime to 0
                          loadavg: null,
                          status: nodeInfo.status || 'unknown' // Use API status or default
                      });
                  });

                  // Process results for Guests and Node Status
                  guestResults.forEach((result, index) => {
                      const correspondingNodeName = nodes[index]?.node; // Get node name for matching
                      if (!correspondingNodeName) return; // Skip if node info is missing

                      const targetNodeIndex = endpointNodes.findIndex(n => n.node === correspondingNodeName);
                      if (targetNodeIndex === -1) return; // Skip if node not found in our list

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

                          // Merge node status if available
                          if (result.value.nodeStatus) {
                            const statusData = result.value.nodeStatus;
                            // Merge specific fields we care about, BUT DO NOT overwrite maxcpu/maxmem
                            endpointNodes[targetNodeIndex].cpu = statusData.cpu;
                            // endpointNodes[targetNodeIndex].maxcpu = statusData.maxcpu; // Already set from nodeInfo
                            endpointNodes[targetNodeIndex].mem = statusData.memory?.used || statusData.mem;
                            // endpointNodes[targetNodeIndex].maxmem = statusData.memory?.total || statusData.maxmem; // Already set from nodeInfo
                            endpointNodes[targetNodeIndex].disk = statusData.rootfs?.used || statusData.disk;
                            endpointNodes[targetNodeIndex].maxdisk = statusData.rootfs?.total || statusData.maxdisk;
                            endpointNodes[targetNodeIndex].uptime = statusData.uptime;
                            endpointNodes[targetNodeIndex].loadavg = statusData.loadavg; // Add loadavg
                            // Update status if uptime indicates online, otherwise keep original list status
                            endpointNodes[targetNodeIndex].status = statusData.uptime > 0 ? 'online' : endpointNodes[targetNodeIndex].status;
                          }

                          // ---> ADDED: Merge node storage if available <---
                          if (result.value.storage && Array.isArray(result.value.storage)) {
                            endpointNodes[targetNodeIndex].storage = result.value.storage;
                          }
                          // ---> END ADDED <---

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
  const pbsClientIds = Object.keys(pbsApiClients);
  const pbsDataResults = []; // Array to hold results for each PBS instance

  if (pbsClientIds.length > 0) {
      console.log(`INFO: Fetching discovery data for ${pbsClientIds.length} PBS instances...`);
      const pbsPromises = pbsClientIds.map(async (pbsClientId) => {
          const { client: pbsClientInstance, config: pbsInstanceConfig } = pbsApiClients[pbsClientId];
          const instanceName = pbsInstanceConfig.name; // Use the configured name
          let instanceData = { 
              pbsEndpointId: pbsClientId, // Identifier for this PBS instance
              pbsInstanceName: instanceName, // Human-readable name
              status: 'error', // Default to error
              nodeName: pbsInstanceConfig.nodeName, // Start with configured node name
              backupTasks: { recentTasks: [], summary: {} },
              datastores: [],
              verificationTasks: { summary: {} },
              syncTasks: { summary: {} },
              pruneTasks: { summary: {} }
          }; 

          try {
              console.log(`INFO: [PBS Discovery - ${instanceName}] Starting detailed data fetch...`);

              // Ensure client is valid (redundant check, should be caught in init)
              if (!pbsClientInstance) {
                  throw new Error(`Client not initialized for PBS instance: ${instanceName}`);
              }

              // Determine node name if not pre-configured
              if (!instanceData.nodeName) {
                  instanceData.nodeName = await fetchPbsNodeName({ client: pbsClientInstance, config: pbsInstanceConfig }); // Pass the object
                  // Store detected name back in config for future use (within this run cycle)
                  if (instanceData.nodeName && instanceData.nodeName !== 'localhost') {
                      pbsInstanceConfig.nodeName = instanceData.nodeName; // Update the config object directly
                  }
              }

              // Only proceed if we have a node name
              if (instanceData.nodeName) {
                  // Fetch datastores first, then snapshots, then tasks
                  const datastoresResult = await fetchPbsDatastoreData({ client: pbsClientInstance, config: pbsInstanceConfig });

                  // Fetch snapshots for each datastore
                  const snapshotFetchPromises = (datastoresResult || []).map(async (ds) => {
                      const storeName = ds.name; // Assuming 'name' holds the datastore ID
                      if (!storeName) {
                          console.warn(`WARN: [PBS Discovery - ${instanceName}] Skipping snapshot fetch for datastore with no name:`, ds);
                          ds.snapshots = []; // Ensure snapshots array exists even if skipped
                          ds.snapshotError = 'Missing datastore name';
                          return ds; // Return the datastore object as is
                      }
                      try {
                          console.log(`INFO: [PBS Discovery - ${instanceName}] Fetching snapshots for datastore '${storeName}'...`);
                          const snapshotResponse = await pbsClientInstance.get(`/admin/datastore/${storeName}/snapshots`);
                          ds.snapshots = snapshotResponse.data?.data ?? [];
                          ds.snapshotError = null;
                          // console.log(`INFO: [PBS Discovery - ${instanceName}] Fetched ${ds.snapshots.length} snapshots for datastore ${storeName}.`);
                      } catch (snapshotError) {
                          const status = snapshotError.response?.status ? ` (Status: ${snapshotError.response.status})` : '';
                          console.error(`ERROR: [PBS Discovery - ${instanceName}] Failed to fetch snapshots for datastore ${storeName}${status}: ${snapshotError.message}`);
                          ds.snapshots = []; // Ensure snapshots array exists on error
                          ds.snapshotError = snapshotError.message;
                          // Propagate specific auth errors if needed
                          if (snapshotError.response?.status === 401 || snapshotError.response?.status === 403) {
                              // Optionally re-throw or handle critical permission errors differently
                          }
                      }
                      return ds; // Return the datastore object with snapshots added
                  });

                  // Wait for all snapshot fetches for this instance to complete
                  const datastoresWithSnapshots = await Promise.all(snapshotFetchPromises);

                  // Fetch tasks after getting datastores and snapshots
                  const allTasksResult = await fetchAllPbsTasksForProcessing({ client: pbsClientInstance, config: pbsInstanceConfig }, instanceData.nodeName); // New function call

                  // Assign datastore results
                  instanceData.datastores = datastoresWithSnapshots; // Use the array that now includes snapshots

                  // Process the single task list for all summaries and details
                  if (allTasksResult && allTasksResult.tasks) {
                      const processedTasks = processPbsTasks(allTasksResult.tasks);
                      instanceData.backupTasks = processedTasks.backupTasks;
                      instanceData.verificationTasks = processedTasks.verificationTasks;
                      instanceData.syncTasks = processedTasks.syncTasks;
                      instanceData.pruneTasks = processedTasks.pruneTasks;
                  }

                  // If tasks failed to fetch (allTasksResult.error is true), summaries will remain default/empty

                  instanceData.status = 'ok'; // Mark as OK if datastores fetch succeeded (tasks handled separately)
                  console.log(`INFO: [PBS Discovery - ${instanceName}] Successfully fetched and processed data (Node: ${instanceData.nodeName}).`);
              } else {
                  console.error(`ERROR: Could not determine node name for PBS instance ${instanceName}, cannot fetch task data.`);
                  instanceData.status = 'error'; // Keep status as error
              }

          } catch (pbsError) {
              console.error(`ERROR: [PBS Discovery - ${instanceName}] Fetch failed: ${pbsError.message}`);
              instanceData.status = 'error'; // Ensure status is error on any failure
              if (pbsError.response?.status === 401) {
                  console.error(`ERROR: PBS API Authentication Expired/Invalid (401) for ${instanceName}.`);
              }
              // instanceData already defaults to error state with empty data structures
          }
          return instanceData; // Return the data object for this instance (ok or error state)
      }); // End map over pbsClientIds

      const settledPbsResults = await Promise.allSettled(pbsPromises);
      settledPbsResults.forEach(result => {
          if (result.status === 'fulfilled') {
              pbsDataResults.push(result.value); // Add the instance data (ok or error)
          } else {
              // This should ideally not happen if errors are caught within the promise
              console.error(`ERROR: Unhandled rejection fetching PBS data: ${result.reason}`);
              // Could push a generic error object here if needed
          }
      });

  } else {
      console.log("[Discovery Cycle] No PBS instances configured or initialized.");
      // pbsDataResults remains empty
  }
  // --- End Fetch PBS Data ---

  // --- Update Global State ---
  // ---> CHANGE: Update pbsDataArray (the global state)
  pbsDataArray = pbsDataResults; // Replace the global state with the newly fetched array
  // <--- END CHANGE

  // Always update PVE data
  aggregatedResult.nodes = tempNodes;
  aggregatedResult.vms = tempVms;
  aggregatedResult.containers = tempContainers;

  // ---> CHANGE: Add the final pbsDataArray state to the result
  // aggregatedResult.pbs = pbsData;
  aggregatedResult.pbs = pbsDataArray;
  // <--- END CHANGE

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

  // ---> CHANGE: Send initial status based on pbsConfigs array
  let initialPbsStatuses = pbsConfigs.map(conf => ({ 
      pbsEndpointId: conf.id,
      pbsInstanceName: conf.name,
      status: 'configured' // Assume configured initially
  }));
  if (initialPbsStatuses.length === 0) {
      initialPbsStatuses.push({ pbsEndpointId: 'none', pbsInstanceName: 'None', status: 'unconfigured' });
  }
  console.log(`[socket] Sending initial PBS status array to new client:`, initialPbsStatuses);
  socket.emit('pbsInitialStatus', initialPbsStatuses); // Send the array
  // ---> END CHANGE <---

  // ---> CHANGE: Send initial data using pbsDataArray
  if (currentNodes.length > 0 || currentVms.length > 0 || currentContainers.length > 0) {
    console.log('[socket] Sending existing PVE/Metric data to new client.');

    // If pbsDataArray is empty but configs exist, send the initial configured statuses
    let pbsToSend = (pbsDataArray && pbsDataArray.length > 0) ? pbsDataArray : initialPbsStatuses.map(s => ({ ...s })); // Use copy

    socket.emit('rawData', {
        nodes: currentNodes,
        vms: currentVms,
        containers: currentContainers,
        metrics: currentMetrics,
        pbs: pbsToSend // Send the array of PBS data/statuses
    });
  } else {
    // If no data yet, trigger a discovery cycle (if not already running)
    console.log('[socket] No PVE data yet, triggering initial discovery for new client...');
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
    const discoveryData = await fetchDiscoveryData(); // Returns {nodes, vms, containers, pbs: pbsDataArray}
    
    // Update global state variables
    currentNodes = discoveryData.nodes || [];
    currentVms = discoveryData.vms || [];
    currentContainers = discoveryData.containers || [];
    // ---> CHANGE: Update pbsDataArray
    pbsDataArray = discoveryData.pbs || []; // Update the global array
    // <--- END CHANGE

    // Emit combined data
    if (io.engine.clientsCount > 0) {
        if (DEBUG_METRICS) {
             console.log('[Discovery Cycle] Emitting updated structural data including PBS.');
        }
        io.emit('rawData', {
            nodes: currentNodes,
            vms: currentVms,
            containers: currentContainers,
            pbs: pbsDataArray
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

        // ---> MODIFIED: Avoid clearing metrics on transient fetch failure <---
        if (fetchedMetrics && fetchedMetrics.length > 0) {
            // Success: Update metrics if data was actually returned
            currentMetrics = fetchedMetrics; 
             console.log(`[Metrics Cycle] Successfully updated metrics for ${currentMetrics.length} guests.`);
        } else if (fetchedMetrics && fetchedMetrics.length === 0) {
            // Fetch likely failed temporarily, keep previous metrics
            console.warn('[Metrics Cycle] fetchMetricsData returned empty array despite running guests. Preserving previous metrics state.');
            // Do NOT update currentMetrics = [] here
        } else {
            // Handle unexpected null/undefined return from fetchMetricsData (shouldn't happen)
             console.error('[Metrics Cycle] fetchMetricsData returned unexpected value. Preserving previous metrics state.', fetchedMetrics);
        }
        // ---> END MODIFICATION <---

        // Emit combined data (always emit, even if metrics weren't updated this cycle)
        io.emit('rawData', {
            nodes: currentNodes,
            vms: currentVms,
            containers: currentContainers,
            pbs: pbsDataArray,
            metrics: currentMetrics
        });
    } else {
        // console.log('[Metrics Cycle] No running guests found, skipping metric fetch.');
        currentMetrics = []; // Clear metrics if no guests running - THIS IS CORRECT
         // Emit state update even if metrics were cleared
        io.emit('rawData', {
            nodes: currentNodes, vms: currentVms,
            containers: currentContainers, metrics: currentMetrics,
            pbs: pbsDataArray
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

// Modified fetchPbsTaskData to include more details and handle backup tasks specifically
/*
async function fetchPbsTaskData(pbsClient, nodeName) {
    // ... (keep original function body commented out or remove) ...
}
*/

// NEW function to fetch the full task list once for processing
async function fetchAllPbsTasksForProcessing(pbsClient, nodeName) {
    console.log(`INFO: Fetching all relevant PBS tasks (7 days) for node ${nodeName}...`);
    if (!nodeName) {
        console.warn("WARN: Cannot fetch PBS task data without node name.");
        return { tasks: null, error: true };
    }
    try {
        const sinceTimestamp = Math.floor((Date.now() - 7 * 24 * 60 * 60 * 1000) / 1000);
        const response = await pbsClient.client.get(`/nodes/${nodeName}/tasks`, {
            params: {
                since: sinceTimestamp,
                limit: 1000, // Fetch a larger number to cover 7 days of various tasks
                errors: 1,
            }
        });
        const allTasks = response.data?.data ?? [];
        console.log(`INFO: Fetched ${allTasks.length} tasks from PBS for processing.`);
        return { tasks: allTasks, error: false };
    } catch (error) {
        console.error(`ERROR: Failed to fetch PBS task list for node ${nodeName}: ${error.message}`);
        if (error.response?.status === 401) console.error("ERROR: PBS API authentication failed (401).");
        else if (error.response?.status === 403) console.error("ERROR: PBS API authorization failed (403).");
        return { tasks: null, error: true };
    }
}

// NEW function to process the fetched task list into required structures
function processPbsTasks(allTasks) {
    if (!allTasks) return { // Return default structure if tasks are null
        backupTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        verificationTasks: { summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        syncTasks: { summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        pruneTasks: { summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } }
    };

    const taskResults = {
        backup: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        verify: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        sync: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        pruneGc: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 } // Combined prune/gc
    };

    // Define type mappings
    const taskTypeMap = {
        backup: 'backup',
        verify: 'verify',
        sync: 'sync',
        garbage_collection: 'pruneGc',
        prune: 'pruneGc'
    };

    allTasks.forEach(task => {
        const taskType = task.worker_type || task.type;
        const categoryKey = taskTypeMap[taskType];

        if (categoryKey) {
            const category = taskResults[categoryKey];
            category.list.push(task); // Add raw task for potential future use

            const isOk = task.status === 'OK';
            const isFailed = task.status && task.status !== 'OK' && task.status !== 'running';

            if (isOk) {
                category.ok++;
                if (task.endtime > category.lastOk) category.lastOk = task.endtime;
            } else if (isFailed) {
                category.failed++;
                if (task.endtime > category.lastFailed) category.lastFailed = task.endtime;
            }
        }
    });

    // Helper to create detailed task object
    const createDetailedTask = (task) => ({
        upid: task.upid,
        node: task.node,
        type: task.worker_type || task.type,
        id: task.worker_id || task.id,
        status: task.status,
        startTime: task.starttime,
        endTime: task.endtime,
        duration: task.endtime && task.starttime ? task.endtime - task.starttime : null,
    });

    // Process and sort recent tasks for each category
    const sortTasksDesc = (a, b) => (b.startTime || 0) - (a.startTime || 0);

    const recentBackupTasks = taskResults.backup.list.map(createDetailedTask).sort(sortTasksDesc).slice(0, 20);
    const recentVerifyTasks = taskResults.verify.list.map(createDetailedTask).sort(sortTasksDesc).slice(0, 20);
    const recentSyncTasks = taskResults.sync.list.map(createDetailedTask).sort(sortTasksDesc).slice(0, 20);
    const recentPruneGcTasks = taskResults.pruneGc.list.map(createDetailedTask).sort(sortTasksDesc).slice(0, 20);

    console.log(`INFO: Processed PBS Tasks - Backup: ${taskResults.backup.list.length} (OK: ${taskResults.backup.ok}, Fail: ${taskResults.backup.failed}), Verify: ${taskResults.verify.list.length} (OK: ${taskResults.verify.ok}, Fail: ${taskResults.verify.failed}), Sync: ${taskResults.sync.list.length} (OK: ${taskResults.sync.ok}, Fail: ${taskResults.sync.failed}), Prune/GC: ${taskResults.pruneGc.list.length} (OK: ${taskResults.pruneGc.ok}, Fail: ${taskResults.pruneGc.failed})`);

    // Return the structured data expected by fetchDiscoveryData
    return {
        backupTasks: {
            recentTasks: recentBackupTasks,
            summary: {
                ok: taskResults.backup.ok,
                failed: taskResults.backup.failed,
                total: taskResults.backup.list.length,
                lastOk: taskResults.backup.lastOk || null,
                lastFailed: taskResults.backup.lastFailed || null,
            }
        },
        verificationTasks: {
            recentTasks: recentVerifyTasks,
            summary: {
                ok: taskResults.verify.ok,
                failed: taskResults.verify.failed,
                total: taskResults.verify.list.length,
                lastOk: taskResults.verify.lastOk || null,
                lastFailed: taskResults.verify.lastFailed || null,
            }
        },
        syncTasks: {
            recentTasks: recentSyncTasks,
            summary: {
                ok: taskResults.sync.ok,
                failed: taskResults.sync.failed,
                total: taskResults.sync.list.length,
                lastOk: taskResults.sync.lastOk || null,
                lastFailed: taskResults.sync.lastFailed || null,
            }
        },
        pruneTasks: {
            recentTasks: recentPruneGcTasks,
            summary: {
                ok: taskResults.pruneGc.ok,
                failed: taskResults.pruneGc.failed,
                total: taskResults.pruneGc.list.length,
                lastOk: taskResults.pruneGc.lastOk || null,
                lastFailed: taskResults.pruneGc.lastFailed || null,
            }
        }
    };
}

// DEPRECATED: Helper function to fetch and summarize tasks by type(s)
/*
async function fetchPbsTaskSummaryByType(pbsClient, nodeName, taskTypes) {
    // ... (keep original function body commented out or remove) ...
}
*/

async function fetchPbsDatastoreData(pbsClient) {
    // Fetches datastore usage details from PBS using the /status/datastore-usage endpoint
    console.log("INFO: Fetching PBS datastore data...");
    let datastores = [];
    try {
        // Fetch usage stats for all datastores at once
        const usageResponse = await pbsClient.client.get('/status/datastore-usage');
        const usageData = usageResponse.data?.data ?? [];

        if (usageData.length > 0) {
            console.log(`INFO: Fetched status for ${usageData.length} PBS datastores via /status/datastore-usage.`);
            // Map the received data to the expected format
            datastores = usageData.map(ds => ({
                name: ds.store,
                path: ds.path || 'N/A',
                total: ds.total,
                used: ds.used,
                available: ds.avail,
                // Ensure gcStatus is included and defaults gracefully
                gcStatus: ds['garbage-collection-status'] || 'unknown'
            }));
        } else {
            console.warn("WARN: PBS /status/datastore-usage returned empty data. Falling back to /config/datastore.");
            throw new Error("Empty data from /status/datastore-usage"); // Trigger fallback
        }

    } catch (usageError) {
        console.error(`ERROR: Failed to fetch PBS datastore usage via /status/datastore-usage: ${usageError.message}. Trying fallback /config/datastore.`);
        // --- Fallback Logic ---
        try {
            const configResponse = await pbsClient.client.get('/config/datastore');
            const datastoresConfig = configResponse.data?.data ?? [];
            if (datastoresConfig.length > 0) {
                console.log(`INFO: Fetched config for ${datastoresConfig.length} PBS datastores (fallback). Status unavailable.`);
                 datastores = datastoresConfig.map(dsConfig => ({
                    name: dsConfig.name,
                    path: dsConfig.path,
                    total: null, // Usage/Status info unavailable from config
                    used: null,
                    available: null,
                    gcStatus: 'unknown (config only)' // Explicitly mark GC status
                }));
            } else {
                 console.warn("WARN: Fallback fetch of PBS datastore config also returned empty data.");
            }
        } catch (configError) {
            console.error(`ERROR: Fallback fetch of PBS datastore config failed: ${configError.message}`);
             if (configError.response?.status === 401 || configError.response?.status === 403) {
                console.error("ERROR: PBS API authentication/authorization failed for datastore config access.");
             }
             // Keep datastores as empty array if both primary and fallback attempts fail
        }
        // --- End Fallback ---
    }

    console.log(`INFO: Finished fetching PBS datastore data. Found ${datastores.length} datastores.`);
    return datastores;
}

// --- END PBS Data Fetching Functions ---

// Start the server
async function startServer() {
    await initializeAllPbsClients();

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