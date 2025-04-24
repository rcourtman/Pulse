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

if (Object.keys(apiClients).length === 0) {
    console.error("\n--- Configuration Error ---");
    console.error("No enabled Proxmox endpoints could be configured. Please check your .env file and environment variables.");
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
app.use(express.static(path.join(__dirname, '../public')));

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
  console.log('[Discovery Cycle] Starting fetch across all endpoints...');
  const aggregatedResult = {
    nodes: [],
    vms: [],
    containers: []
  };

  // Get a list of endpoint IDs to iterate over
  const endpointIds = Object.keys(apiClients);
  if (endpointIds.length === 0) {
      console.warn("[Discovery Cycle] No API clients configured. Skipping fetch.");
      return aggregatedResult; // Return empty if no clients
  }

  // Use Promise.allSettled to fetch from all endpoints concurrently
  const discoveryPromises = endpointIds.map(async (endpointId) => {
    const { client: apiClient, config: endpointConfig } = apiClients[endpointId];
    const endpointName = endpointConfig.name || endpointId; // For logging
    const endpointResult = { nodes: [], vms: [], containers: [] }; // Results for this specific endpoint
    let nodesToQuery = [];
    let basicNodeInfo = []; // Basic info from /nodes for this endpoint

    console.log(`[Discovery Cycle] Fetching discovery data for endpoint: ${endpointName}`);

    // --- Step 1: Fetch Node List and Basic Info for this endpoint ---
    try {
      const nodesResponse = await apiClient.get('/nodes');
      basicNodeInfo = nodesResponse.data.data || [];
      nodesToQuery = basicNodeInfo.map(n => n.node).filter(Boolean);

      if (nodesToQuery.length === 0) {
        console.warn(`[Discovery Cycle - ${endpointName}] /nodes returned 0 nodes. Attempting single node discovery.`);
        try {
          const versionResponse = await apiClient.get('/version');
          const discoveredNodeName = versionResponse.data.data.node;
          if (discoveredNodeName) {
            nodesToQuery = [discoveredNodeName];
            // Synthesize basic info for the single node
            basicNodeInfo = [{ node: discoveredNodeName, ip: 'unknown', status: 'unknown' }];
          } else {
            throw new Error("Could not determine node name from /version.");
          }
        } catch (discoveryError) {
          // Add status code to error log
          const status = discoveryError.response?.status ? ` (Status: ${discoveryError.response.status})` : '';
          console.error(`[Discovery Cycle - ${endpointName}] Single node discovery failed${status}: ${discoveryError.message}. Cannot proceed for this endpoint.`);
          return { status: 'rejected', value: null, endpointId: endpointId, reason: discoveryError }; // Indicate failure for this endpoint
        }
      }
    } catch (error) {
      // Add status code to error log
      const status = error.response?.status ? ` (Status: ${error.response.status})` : '';
      console.warn(`[Discovery Cycle - ${endpointName}] Failed to fetch /nodes${status} (${error.message}). Attempting single node discovery.`);
      try {
        const versionResponse = await apiClient.get('/version');
        const discoveredNodeName = versionResponse.data.data.node;
        if (discoveredNodeName) {
            nodesToQuery = [discoveredNodeName];
            basicNodeInfo = [{ node: discoveredNodeName, ip: 'unknown', status: 'unknown' }];
        } else {
            throw new Error("Could not determine node name from /version.");
        }
      } catch (discoveryError) {
          // Add status code to error log
          const status = discoveryError.response?.status ? ` (Status: ${discoveryError.response.status})` : '';
          console.error(`[Discovery Cycle - ${endpointName}] Single node discovery failed after /nodes error${status}: ${discoveryError.message}. Cannot proceed for this endpoint.`);
           return { status: 'rejected', value: null, endpointId: endpointId, reason: discoveryError }; // Indicate failure for this endpoint
      }
    }

    // --- Step 2: Fetch Detailed Status for Each Node in this endpoint --- 
    const nodeStatusPromises = nodesToQuery.map(async (nodeName) => {
      if (!nodeName) return null;
      try {
          const statusResponse = await apiClient.get(`/nodes/${nodeName}/status`);
          let statusData = statusResponse.data.data;
          const basicInfo = basicNodeInfo.find(n => n.node === nodeName);

          // Ensure statusData is an object and add necessary fields
          if (typeof statusData !== 'object' || statusData === null) {
             statusData = {}; // Initialize if not an object
          }
          if (!statusData.node) { statusData.node = nodeName; }
          statusData.ip = basicInfo?.ip || 'fetch_error'; // Add IP from basic info
          statusData.endpointId = endpointId; // **** ADD ENDPOINT ID ****
          statusData.endpointName = endpointName; // Add human-readable name too
          return statusData;

      } catch (statusError) {
           // Add status code to error log
           const status = statusError.response?.status ? ` (Status: ${statusError.response.status})` : '';
           console.warn(`[Discovery Cycle - ${endpointName}] Could not fetch status for node ${nodeName}${status}: ${statusError.message}`);
           const basicInfo = basicNodeInfo.find(n => n.node === nodeName);
           // Return a synthesized offline status object
           return {
               node: nodeName,
               status: 'offline', // Explicitly mark as offline on error
               ip: basicInfo?.ip || 'fetch_error',
               cpu: 0, // Provide default numeric values
               memory: { total: 0, used: 0 },
               rootfs: { total: 0, used: 0 },
               swap: { total: 0, used: 0 },
               uptime: 0,
               endpointId: endpointId, // **** ADD ENDPOINT ID ****
               endpointName: endpointName
            };
      }
    });
    const detailedNodeResults = await Promise.allSettled(nodeStatusPromises);
    endpointResult.nodes = detailedNodeResults
                        .filter(result => result.status === 'fulfilled' && result.value)
                        .map(result => result.value);

    // Fallback if all status calls failed for this endpoint but /nodes worked
    if (endpointResult.nodes.length === 0 && basicNodeInfo.length > 0) {
        console.warn(`[Discovery Cycle - ${endpointName}] All node status fetches failed. Falling back to basic node info from /nodes.`);
        endpointResult.nodes = basicNodeInfo.map(node => ({
            ...node,
            ip: node.ip || 'unknown',
            status: 'unknown', // Mark status as unknown
            cpu: 0, memory: { total: 0, used: 0 }, rootfs: { total: 0, used: 0 },
            swap: { total: 0, used: 0 }, uptime: 0,
            endpointId: endpointId, // **** ADD ENDPOINT ID ****
            endpointName: endpointName
        }));
    }


    // --- Step 3: Fetch VM/Container Lists for Each Node in this endpoint --- 
    // Only query nodes we successfully got status for (or synthesized)
    const finalNodeNames = endpointResult.nodes.map(n => n.node).filter(Boolean);
    const guestListPromises = finalNodeNames.map(async (nodeName) => {
      try {
         // Use the modified fetchDataForNode which now requires apiClient and endpointId
         // It returns { vms: [], containers: [] } with endpointId already added
         // Pass the endpointId explicitly here
         const nodeGuestData = await fetchDataForNode(apiClient, endpointId, nodeName);
         // The endpointId and type are added within fetchDataForNode now
         // return { vms: nodeGuestData.vms, containers: nodeGuestData.containers };
         // Simplified return, type/endpointId added in fetchDataForNode
         return nodeGuestData; 
      } catch (guestError) {
          // Add status code to error log
          const status = guestError.response?.status ? ` (Status: ${guestError.response.status})` : '';
          console.error(`[Discovery Cycle - ${endpointName}] Error fetching guest data for node ${nodeName}${status}: ${guestError.message}`);
          return { vms: [], containers: [] }; // Return empty on error for this node
      }
    });

    const guestListResults = await Promise.allSettled(guestListPromises);
    guestListResults.forEach(result => {
        if (result.status === 'fulfilled' && result.value) {
            endpointResult.vms.push(...result.value.vms);
            endpointResult.containers.push(...result.value.containers);
        }
    });

    console.log(`[Discovery Cycle - ${endpointName}] Completed. Found: ${endpointResult.nodes.length} nodes, ${endpointResult.vms.length} VMs, ${endpointResult.containers.length} containers.`);
    return { status: 'fulfilled', value: endpointResult, endpointId: endpointId }; // Return structured result

  }); // End map over endpointIds

  // Process results from all endpoints
  const allEndpointResults = await Promise.allSettled(discoveryPromises);

  allEndpointResults.forEach(endpointOutcome => {
      if (endpointOutcome.status === 'fulfilled' && endpointOutcome.value.status === 'fulfilled') {
          const endpointData = endpointOutcome.value.value;
          aggregatedResult.nodes.push(...endpointData.nodes);
          aggregatedResult.vms.push(...endpointData.vms);
          aggregatedResult.containers.push(...endpointData.containers);
      } else if (endpointOutcome.status === 'rejected') {
          // Log errors from the outer Promise.allSettled (e.g., client setup issues or inner rejects)
          const reason = endpointOutcome.reason || endpointOutcome.value?.reason || 'Unknown reason';
          const failedEndpointId = endpointOutcome.value?.endpointId || 'Unknown endpoint';
          console.error(`[Discovery Cycle] Failed to process endpoint discovery promise for ${failedEndpointId}: ${reason}`);
      } else if (endpointOutcome.value.status !== 'fulfilled'){ 
           // Log errors from the inner endpoint processing (already logged, but maybe add summary)
           console.error(`[Discovery Cycle] Failed discovery for endpoint: ${endpointOutcome.value.endpointId}`);
      }
  });

  if (DEBUG_METRICS) {
    console.log(`[Discovery Cycle] Aggregated results. Total: ${aggregatedResult.nodes.length} nodes, ${aggregatedResult.vms.length} VMs, ${aggregatedResult.containers.length} containers across ${endpointIds.length} configured endpoints.`);
  }
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
    const discoveryData = await fetchDiscoveryData();
    // Update global state
    currentNodes = discoveryData.nodes;
    currentVms = discoveryData.vms;
    currentContainers = discoveryData.containers;

    // Emit combined data only if clients are connected
    if (io.engine.clientsCount > 0) {
        if (DEBUG_METRICS) {
             console.log('[Discovery Cycle] Emitting updated structural data.');
        }
        io.emit('rawData', {
            nodes: currentNodes,
            vms: currentVms,
            containers: currentContainers,
            metrics: currentMetrics // Send latest (potentially empty) metrics
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

// Start the server
server.listen(PORT, () => {
  console.log(`Server listening on port ${PORT}`);
  
  // Setup hot reload in development mode
  if (process.env.NODE_ENV === 'development' && chokidar) {
    const publicPath = path.join(__dirname, '../public');
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