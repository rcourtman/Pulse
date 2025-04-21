require('dotenv').config(); // Load environment variables from .env file

// --- BEGIN Environment Variable Validation ---
const requiredEnvVars = [
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

requiredEnvVars.forEach(varName => {
  const value = process.env[varName];
  if (!value) {
    missingVars.push(varName);
  } else if (placeholderValues.some(placeholder => value.includes(placeholder))) {
    placeholderVars.push(varName);
  }
});

if (missingVars.length > 0 || placeholderVars.length > 0) {
  console.error('\n--- Configuration Error ---');
  if (missingVars.length > 0) {
    console.error(`Missing required environment variables in server/.env: ${missingVars.join(', ')}`);
  }
  if (placeholderVars.length > 0) {
    console.error(`Environment variables seem to contain placeholder values in server/.env: ${placeholderVars.join(', ')}`);
  }
  console.error('Please ensure server/.env exists and contains valid Proxmox connection details.');
  console.error('Refer to server/.env.example for the required format.\n');
  process.exit(1); // Exit if configuration is invalid
}
// --- END Environment Variable Validation ---

const fs = require('fs'); // Add fs module
const express = require('express');
const http = require('http');
const path = require('path');
const cors = require('cors');
const { Server } = require('socket.io');
const axios = require('axios');
const https = require('https');

// Development specific dependencies
let chokidar;
if (process.env.NODE_ENV === 'development') {
  try {
    chokidar = require('chokidar');
  } catch (e) {
    console.warn('chokidar is not installed. Hot reload requires chokidar: npm install --save-dev chokidar');
  }
}

// Proxmox node configuration - using the same config as the main server
const proxmoxConfig = {
  node1: {
    name: process.env.PROXMOX_NODE_NAME || 'minipc',
    host: process.env.PROXMOX_HOST || 'https://192.168.0.132:8006',
    port: process.env.PROXMOX_PORT || '8006',
    tokenId: process.env.PROXMOX_TOKEN_ID || 'root@pam!pulse',
    tokenSecret: process.env.PROXMOX_TOKEN_SECRET || 'e1850350-6afc-4b8e-ae28-472152af84f9',
    enabled: process.env.PROXMOX_ENABLED !== 'false',
    allowSelfSignedCerts: process.env.PROXMOX_ALLOW_SELF_SIGNED_CERTS !== 'false',
    credentials: {
      username: process.env.PROXMOX_USERNAME || 'root',
      password: process.env.PROXMOX_PASSWORD || 'password',
      realm: process.env.PROXMOX_REALM || 'pam'
    }
  }
};

// Server configuration
const DEBUG_METRICS = false; // Set to true to show detailed metrics logs
const PORT = 7655; // Using a different port from the main server

// --- Define Update Intervals ---
// How often to fetch dynamic metrics (CPU, Mem, IO) for running guests
const METRIC_UPDATE_INTERVAL = 2000; // Default: 2 seconds
// How often to fetch structural data (node list, guest lists, node status)
const DISCOVERY_UPDATE_INTERVAL = 30000; // Default: 30 seconds

// Create Proxmox API client
const proxmoxApi = axios.create({
  baseURL: proxmoxConfig.node1.host.includes('://') 
    ? `${proxmoxConfig.node1.host}/api2/json` 
    : `https://${proxmoxConfig.node1.host}:${proxmoxConfig.node1.port}/api2/json`,
  httpsAgent: new https.Agent({
    rejectUnauthorized: !proxmoxConfig.node1.allowSelfSignedCerts
  }),
  headers: {
    'Content-Type': 'application/json'
  }
});

// Add request interceptor for authentication
proxmoxApi.interceptors.request.use(config => {
  // Add API token authentication 
  if (proxmoxConfig.node1.tokenId && proxmoxConfig.node1.tokenSecret) {
    config.headers.Authorization = `PVEAPIToken=${proxmoxConfig.node1.tokenId}=${proxmoxConfig.node1.tokenSecret}`;
  }
  // Fallback to password auth if configured
  else if (proxmoxConfig.node1.credentials) {
    const { username, password, realm } = proxmoxConfig.node1.credentials;
    config.headers.Authorization = `Basic ${Buffer.from(`${username}@${realm}:${password}`).toString('base64')}`;
  }
  
  return config;
});

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
  const storageData = {};
  let nodesToQuery = [];
  let discoveryFailed = false; // Flag for discovery failure

  try {
    // Reuse the node discovery logic from fetchRawProxmoxData
    const nodesResponse = await proxmoxApi.get('/nodes');
    const basicNodeInfo = nodesResponse.data.data || [];
    nodesToQuery = basicNodeInfo.map(n => n.node); // Get just the names for further queries

    if (nodesToQuery.length === 0) {
       console.warn('/api/storage: Proxmox API returned 0 nodes from /nodes. Attempting single node discovery.');
       try {
         const versionResponse = await proxmoxApi.get('/version');
         const discoveredNodeName = versionResponse.data.data.node;
         if (discoveredNodeName) {
           nodesToQuery = [discoveredNodeName];
         } else {
            throw new Error("Could not determine node name from /version.");
         }
       } catch (discoveryError) {
           console.error(`/api/storage: Single node discovery failed: ${discoveryError.message}`);
           discoveryFailed = true; // Mark discovery as failed
       }
    } else {
      // console.log(`/api/storage: Found ${nodesToQuery.length} nodes.`); // Reduced verbosity
    }
  } catch (error) {
    console.warn(`/api/storage: Failed to fetch /nodes (${error.message}). Attempting single node discovery.`);
     try {
       const versionResponse = await proxmoxApi.get('/version');
       const discoveredNodeName = versionResponse.data.data.node;
       if (discoveredNodeName) {
         nodesToQuery = [discoveredNodeName];
         console.log(`/api/storage: Discovered single node: ${discoveredNodeName}`);
       } else {
         throw new Error("Could not determine node name from /version.");
       }
     } catch (discoveryError) {
        console.error(`/api/storage: Single node discovery failed after /nodes error: ${discoveryError.message}`);
        discoveryFailed = true; // Mark discovery as failed
     }
  }

  // If discovery failed entirely, return a specific error structure
  if (discoveryFailed) {
    return res.status(500).json({ globalError: 'Failed to discover any Proxmox nodes to query for storage.' });
  }

  // Fetch storage for each node in parallel
  const storagePromises = nodesToQuery.map(async (nodeName) => {
    if (!nodeName) {
      console.warn('/api/storage: Skipping node with missing name:', nodeName);
      return; // Skip if node name is missing
    }
    try {
      const response = await proxmoxApi.get(`/nodes/${nodeName}/storage`);
      storageData[nodeName] = response.data.data || []; // Store storage info per node
      // console.log(`/api/storage: Successfully fetched storage for node ${nodeName}`); // Reduced verbosity
    } catch (err) {
      console.error(`/api/storage: Error fetching storage for node ${nodeName}: ${err.message}`);
      storageData[nodeName] = { error: `Failed to fetch storage: ${err.message}` }; // Indicate error for this node
    }
  });

  await Promise.allSettled(storagePromises);

  if (Object.keys(storageData).length === 0 && nodesToQuery.length > 0) {
    // This case might happen if all parallel fetches failed for discovered nodes
    // We still return the object, but it might contain only node names with error objects.
    console.warn('/api/storage: Failed to fetch storage for any discovered node.');
  }
  
  // console.log(`/api/storage: Returning data for ${Object.keys(storageData).length} nodes.`); // Reduced verbosity
  res.json(storageData); // Return the potentially mixed success/error data per node
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

// Helper function to fetch data for a single node
async function fetchDataForNode(nodeName) {
  const nodeData = {
    vms: [],
    containers: [],
    metrics: []
  };

  try {
    // Fetch VMs
    const vmsResponse = await proxmoxApi.get(`/nodes/${nodeName}/qemu`);
    if (vmsResponse.data.data && Array.isArray(vmsResponse.data.data)) {
      nodeData.vms = vmsResponse.data.data.map(vm => ({ ...vm, node: nodeName }));

      // Collect metrics for running VMs in parallel
      const vmMetricPromises = nodeData.vms
        .filter(vm => vm.status === 'running')
        .map(async (vm) => {
          try {
            const [rrdData, currentData] = await Promise.all([
              proxmoxApi.get(`/nodes/${nodeName}/qemu/${vm.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
              proxmoxApi.get(`/nodes/${nodeName}/qemu/${vm.vmid}/status/current`)
            ]);

            let metricData = {
              id: vm.vmid, name: vm.name, node: nodeName, type: 'qemu', data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
            return metricData; // Return successful metric data
          } catch (err) {
            console.error(`Failed to get metrics for VM ${vm.vmid} on node ${nodeName}: ${err.message}`);
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
    console.error(`Error fetching VMs from ${nodeName}: ${err.message}`);
    // Continue to fetch containers even if VMs fail
  }

  try {
    // Fetch containers
    const ctsResponse = await proxmoxApi.get(`/nodes/${nodeName}/lxc`);
    if (ctsResponse.data.data && Array.isArray(ctsResponse.data.data)) {
      nodeData.containers = ctsResponse.data.data.map(ct => ({ ...ct, node: nodeName }));

      // Collect metrics for running containers in parallel
      const ctMetricPromises = nodeData.containers
        .filter(ct => ct.status === 'running')
        .map(async (ct) => {
          try {
            const [rrdData, currentData] = await Promise.all([
               proxmoxApi.get(`/nodes/${nodeName}/lxc/${ct.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
               proxmoxApi.get(`/nodes/${nodeName}/lxc/${ct.vmid}/status/current`)
            ]);

            let metricData = {
              id: ct.vmid, name: ct.name, node: nodeName, type: 'lxc', data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
             return metricData; // Return successful metric data
          } catch (err) {
            console.error(`Failed to get metrics for container ${ct.vmid} on node ${nodeName}: ${err.message}`);
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
    console.error(`Error fetching containers from ${nodeName}: ${err.message}`);
  }

  return nodeData; // Return collected data for this node
}

// --- Refactored Data Fetching Logic ---

/**
 * Fetches structural data: node list, node statuses, VM list, Container list.
 */
async function fetchDiscoveryData() {
  console.log('[Discovery Cycle] Starting fetch...');
  const discoveryResult = {
    nodes: [],
    vms: [],
    containers: []
  };
  let nodesToQuery = [];
  let basicNodeInfo = []; // Store basic info separately

  // --- Step 1: Fetch Node List and Basic Info --- 
  try {
    const nodesResponse = await proxmoxApi.get('/nodes');
    basicNodeInfo = nodesResponse.data.data || [];
    nodesToQuery = basicNodeInfo.map(n => n.node).filter(Boolean);

    if (nodesToQuery.length === 0) {
      console.warn('[Discovery Cycle] /nodes returned 0 nodes. Attempting single node discovery.');
      try {
        const versionResponse = await proxmoxApi.get('/version');
        const discoveredNodeName = versionResponse.data.data.node;
        if (discoveredNodeName) {
          nodesToQuery = [discoveredNodeName];
          // We need some basic info for the single node if /nodes failed
          basicNodeInfo = [{ node: discoveredNodeName, ip: 'unknown', status: 'unknown' }]; 
        } else {
          throw new Error("Could not determine node name from /version.");
        }
      } catch (discoveryError) {
        console.error(`[Discovery Cycle] Single node discovery failed: ${discoveryError.message}. Cannot proceed.`);
        return discoveryResult; // Return empty structure on fatal discovery error
      }
    }
  } catch (error) {
    console.warn(`[Discovery Cycle] Failed to fetch /nodes (${error.message}). Attempting single node discovery.`);
    try {
      const versionResponse = await proxmoxApi.get('/version');
      const discoveredNodeName = versionResponse.data.data.node;
      if (discoveredNodeName) {
          nodesToQuery = [discoveredNodeName];
          basicNodeInfo = [{ node: discoveredNodeName, ip: 'unknown', status: 'unknown' }];
      } else {
          throw new Error("Could not determine node name from /version.");
      }
    } catch (discoveryError) {
        console.error(`[Discovery Cycle] Single node discovery failed after /nodes error: ${discoveryError.message}. Cannot proceed.`);
        return discoveryResult;
    }
  }

  // --- Step 2: Fetch Detailed Status for Each Node (and merge IP) ---
  const nodeStatusPromises = nodesToQuery.map(async (nodeName) => {
    if (!nodeName) return null; 
    try {
        const statusResponse = await proxmoxApi.get(`/nodes/${nodeName}/status`);
        const statusData = statusResponse.data.data;
        const basicInfo = basicNodeInfo.find(n => n.node === nodeName);
        if (statusData) {
            if (!statusData.node) { statusData.node = nodeName; }
            statusData.ip = basicInfo?.ip || 'fetch_error';
        } else {
             return { node: nodeName, status: 'unknown', ip: basicInfo?.ip || 'fetch_error' };
        }
        return statusData;
    } catch (statusError) {
         console.warn(`[Discovery Cycle] Could not fetch status for node ${nodeName}: ${statusError.message}`);
         const basicInfo = basicNodeInfo.find(n => n.node === nodeName);
         return basicInfo
                ? { ...basicInfo, status: 'offline', ip: basicInfo.ip || 'fetch_error' }
                : { node: nodeName, status: 'offline', ip: 'fetch_error' };
    }
  });
  const detailedNodeResults = await Promise.allSettled(nodeStatusPromises);
  discoveryResult.nodes = detailedNodeResults
                      .filter(result => result.status === 'fulfilled' && result.value)
                      .map(result => result.value);
  // Fallback if all status calls failed
  if (discoveryResult.nodes.length === 0 && basicNodeInfo.length > 0) {
      console.warn("[Discovery Cycle] All node status fetches failed. Falling back to basic node info from /nodes.");
      discoveryResult.nodes = basicNodeInfo.map(node => ({ ...node, ip: node.ip || 'unknown' }));
  }
  
  // --- Step 3: Fetch VM/Container Lists for Each Node --- 
  const finalNodeNames = discoveryResult.nodes.map(n => n.node).filter(Boolean);
  const guestListPromises = finalNodeNames.map(async (nodeName) => {
    let vms = [];
    let containers = [];
    try {
        const vmsResponse = await proxmoxApi.get(`/nodes/${nodeName}/qemu`);
        if (vmsResponse.data.data && Array.isArray(vmsResponse.data.data)) {
           vms = vmsResponse.data.data.map(vm => ({ ...vm, node: nodeName }));
        }
    } catch (err) { console.error(`[Discovery Cycle] Error fetching VMs from ${nodeName}: ${err.message}`); }
    try {
        const ctsResponse = await proxmoxApi.get(`/nodes/${nodeName}/lxc`);
        if (ctsResponse.data.data && Array.isArray(ctsResponse.data.data)) {
           containers = ctsResponse.data.data.map(ct => ({ ...ct, node: nodeName }));
        }
    } catch (err) { console.error(`[Discovery Cycle] Error fetching containers from ${nodeName}: ${err.message}`); }
    return { vms, containers };
  });

  const guestListResults = await Promise.allSettled(guestListPromises);
  guestListResults.forEach(result => {
      if (result.status === 'fulfilled' && result.value) {
          discoveryResult.vms.push(...result.value.vms);
          discoveryResult.containers.push(...result.value.containers);
      }
  });

  console.log(`[Discovery Cycle] Completed. Found: ${discoveryResult.nodes.length} nodes, ${discoveryResult.vms.length} VMs, ${discoveryResult.containers.length} containers.`);
  return discoveryResult;
}

/**
 * Fetches dynamic metric data ONLY for currently known running VMs and Containers.
 */
async function fetchMetricsData(runningVms, runningContainers) {
  // console.log(`[Metrics Cycle] Starting fetch for ${runningVms.length} VMs, ${runningContainers.length} CTs...`); // Reduced verbosity
  let metrics = [];

  // --- Fetch VM Metrics --- 
  const vmMetricPromises = runningVms.map(async (vm) => {
    try {
      const [rrdData, currentData] = await Promise.all([
        proxmoxApi.get(`/nodes/${vm.node}/qemu/${vm.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
        proxmoxApi.get(`/nodes/${vm.node}/qemu/${vm.vmid}/status/current`)
      ]);
      let metricData = {
        id: vm.vmid, name: vm.name, node: vm.node, type: 'qemu', data: [],
        current: currentData?.data?.data || null
      };
      if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
      return metricData;
    } catch (err) {
      // Log less verbosely for metrics errors
      // console.error(`[Metrics Cycle] Failed metrics for VM ${vm.vmid} on ${vm.node}: ${err.message}`);
      return null; // Return null on error for this specific VM
    }
  });

  // --- Fetch Container Metrics --- 
  const ctMetricPromises = runningContainers.map(async (ct) => {
    try {
      const [rrdData, currentData] = await Promise.all([
         proxmoxApi.get(`/nodes/${ct.node}/lxc/${ct.vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
         proxmoxApi.get(`/nodes/${ct.node}/lxc/${ct.vmid}/status/current`)
      ]);
      let metricData = {
        id: ct.vmid, name: ct.name, node: ct.node, type: 'lxc', data: [],
        current: currentData?.data?.data || null
      };
      if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
       return metricData;
    } catch (err) {
      // console.error(`[Metrics Cycle] Failed metrics for CT ${ct.vmid} on ${ct.node}: ${err.message}`);
      return null; // Return null on error for this specific container
    }
  });

  // --- Combine Results --- 
  const allPromises = [...vmMetricPromises, ...ctMetricPromises];
  const metricResults = await Promise.allSettled(allPromises);
  metricResults.forEach(result => {
       if (result.status === 'fulfilled' && result.value) {
          metrics.push(result.value);
      }
  });
  // console.log(`[Metrics Cycle] Completed. Fetched ${metrics.length} metric sets.`); // Reduced verbosity
  return metrics;
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
      console.log('[Discovery Cycle] Emitting updated structural data.');
      io.emit('rawData', {
          nodes: currentNodes,
          vms: currentVms,
          containers: currentContainers,
          metrics: currentMetrics // Include latest metrics
      });
      // Trigger metrics immediately after discovery if needed?
      // if (!isMetricsRunning) runMetricCycle(); 
    }
  } catch (error) {
      console.error(`[Discovery Cycle] Error during execution: ${error.message}`);
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
    // Filter for running guests based on current state
    const runningVms = currentVms.filter(vm => vm.status === 'running');
    const runningContainers = currentContainers.filter(ct => ct.status === 'running');

    if (runningVms.length > 0 || runningContainers.length > 0) {
        currentMetrics = await fetchMetricsData(runningVms, runningContainers);
        // Emit combined data
        // console.log('[Metrics Cycle] Emitting updated metrics data.'); // Reduced verbosity
        io.emit('rawData', {
            nodes: currentNodes,
            vms: currentVms,
            containers: currentContainers,
            metrics: currentMetrics
        });
    } else {
        // console.log('[Metrics Cycle] No running guests found, skipping metric fetch.');
        currentMetrics = []; // Clear metrics if no guests running
         // Optionally emit state if metrics were cleared?
        io.emit('rawData', {
            nodes: currentNodes, vms: currentVms,
            containers: currentContainers, metrics: currentMetrics
        });
    }

  } catch (error) {
      console.error(`[Metrics Cycle] Error during execution: ${error.message}`);
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
      ignored: /(^|[\\\/])\\./, // ignore dotfiles
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