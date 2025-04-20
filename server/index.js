require('dotenv').config(); // Load environment variables from .env file

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
const UPDATE_INTERVAL = 2000; // 2 seconds for updates
const PORT = 7655; // Using a different port from the main server

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

// Create HTTP server
const server = http.createServer(app);

// Create Socket.IO server with CORS configuration
const io = new Server(server, {
  cors: {
    origin: "*",
    methods: ["GET", "POST"]
  }
});

// Variables to track connected clients
let connectedClients = 0;
let initialNodesLogged = false; // Flag to log node count only once

// Helper function to get raw Proxmox data
async function fetchRawProxmoxData() {
  // const configuredNodeName = proxmoxConfig.node1.name; // No longer needed for API calls
  const rawData = {
    nodes: [],
    vms: [],
    containers: [],
    metrics: []
  };

  let nodesToQuery = [];
  let discoveredNodeName = null; // To store the node name discovered via /version

  try {
    // Attempt to fetch cluster nodes
    const nodesResponse = await proxmoxApi.get('/nodes');
    nodesToQuery = nodesResponse.data.data || [];
    rawData.nodes = nodesToQuery; // Store the nodes list if successful

    if (nodesToQuery.length === 0) {
      console.warn('Proxmox API returned 0 nodes from /nodes endpoint. Attempting single node discovery.');
      // If /nodes returns empty, still try to discover the single node via /version
      try {
        const versionResponse = await proxmoxApi.get('/version');
        discoveredNodeName = versionResponse.data.data.node;
        if (!discoveredNodeName) {
            throw new Error("Could not determine node name from /version endpoint.");
        }
        console.log(`Discovered single node name: ${discoveredNodeName}`);
        nodesToQuery = [{ node: discoveredNodeName }]; // Use the discovered name

        // Optionally, try to get status for the single node
        try {
            const statusResponse = await proxmoxApi.get(`/nodes/${discoveredNodeName}/status`);
            rawData.nodes = [statusResponse.data.data]; // Use actual status if available
        } catch (statusError) {
             console.warn(`Could not fetch status for discovered single node ${discoveredNodeName}: ${statusError.message}`);
             rawData.nodes = [{ node: discoveredNodeName, status: 'unknown' }]; // Fallback to discovered name
        }
      } catch (discoveryError) {
          console.error(`Single node discovery failed: ${discoveryError.message}. Cannot proceed.`);
          // Return empty data if we can't even discover the node name
          return { nodes: [], vms: [], containers: [], metrics: [] };
      }
    }
  } catch (error) {
    console.warn(`Failed to fetch /nodes (attempting single node discovery): ${error.message}`);
    // Assume single node mode if /nodes fails, try discovery via /version
    try {
      const versionResponse = await proxmoxApi.get('/version');
      discoveredNodeName = versionResponse.data.data.node;
       if (!discoveredNodeName) {
            throw new Error("Could not determine node name from /version endpoint.");
        }
      console.log(`Discovered single node name: ${discoveredNodeName}`);
      nodesToQuery = [{ node: discoveredNodeName }]; // Use the discovered name

      // Optionally, try to get status for the single node
      try {
          const statusResponse = await proxmoxApi.get(`/nodes/${discoveredNodeName}/status`);
          rawData.nodes = [statusResponse.data.data]; // Use actual status if available
      } catch (statusError) {
          console.warn(`Could not fetch status for discovered single node ${discoveredNodeName}: ${statusError.message}`);
          rawData.nodes = [{ node: discoveredNodeName, status: 'unknown' }]; // Fallback to discovered name
      }
    } catch (discoveryError) {
        console.error(`Single node discovery failed after /nodes error: ${discoveryError.message}. Cannot proceed.`);
         // Return empty data if discovery fails
         return { nodes: [], vms: [], containers: [], metrics: [] };
    }
  }

  // For each node (either from /nodes or the single discovered node), fetch guests and metrics
  for (const node of nodesToQuery) {
    const nodeName = node.node; // Use the node name from the list (discovered or from /nodes)
    if (!nodeName) {
        console.error("Node object missing 'node' property:", node);
        continue; // Skip if node name is missing
    }
    try {
      // Fetch VMs
      const vmsResponse = await proxmoxApi.get(`/nodes/${nodeName}/qemu`);
      if (vmsResponse.data.data && Array.isArray(vmsResponse.data.data)) {
        rawData.vms.push(...vmsResponse.data.data.map(vm => ({
          ...vm,
          node: nodeName // Ensure node name is correctly assigned
        })));

        // Collect metrics for running VMs
        for (const vm of vmsResponse.data.data.filter(vm => vm.status === 'running')) {
          try {
            // Get traditional RRD data
            const rrdData = await proxmoxApi.get(`/nodes/${nodeName}/qemu/${vm.vmid}/rrddata`, {
              params: { timeframe: 'hour', cf: 'AVERAGE' }
            });

            // Try to get real-time "current" values
            const currentData = await proxmoxApi.get(`/nodes/${nodeName}/qemu/${vm.vmid}/status/current`);

            let metricData = {
              id: vm.vmid, name: vm.name, node: nodeName, type: 'qemu', data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
            rawData.metrics.push(metricData);

            if (rrdData?.data?.data?.length > 1 && DEBUG_METRICS) {
              const newest = rrdData.data.data[rrdData.data.data.length - 1].time;
              const secondNewest = rrdData.data.data[rrdData.data.data.length - 2].time;
              console.log(`VM ${vm.name || vm.vmid} metrics update interval: ${newest - secondNewest} seconds`);
              if (currentData?.data?.data) console.log(`VM ${vm.name || vm.vmid} current metrics: CPU=${currentData.data.data.cpu}, Memory=${currentData.data.data.mem}`, `[RRD last update: ${new Date(newest * 1000).toISOString()}]`);
            }
          } catch (err) {
            console.error(`Failed to get metrics for VM ${vm.vmid} on node ${nodeName}: ${err.message}`);
          }
        }
      }
    } catch (err) {
      console.error(`Error fetching VMs from ${nodeName}: ${err.message}`);
    }

    try {
      // Fetch containers
      const ctsResponse = await proxmoxApi.get(`/nodes/${nodeName}/lxc`);
      if (ctsResponse.data.data && Array.isArray(ctsResponse.data.data)) {
        rawData.containers.push(...ctsResponse.data.data.map(ct => ({
          ...ct,
          node: nodeName // Ensure node name is correctly assigned
        })));

        // Collect metrics for running containers
        for (const ct of ctsResponse.data.data.filter(ct => ct.status === 'running')) {
          try {
            // Get traditional RRD data
            const rrdData = await proxmoxApi.get(`/nodes/${nodeName}/lxc/${ct.vmid}/rrddata`, {
              params: { timeframe: 'hour', cf: 'AVERAGE' }
            });

            // Try to get real-time "current" values
            const currentData = await proxmoxApi.get(`/nodes/${nodeName}/lxc/${ct.vmid}/status/current`);

            let metricData = {
              id: ct.vmid, name: ct.name, node: nodeName, type: 'lxc', data: [],
              current: currentData?.data?.data || null
            };
            if (rrdData?.data?.data?.length > 0) metricData.data = rrdData.data.data;
            rawData.metrics.push(metricData);

             if (rrdData?.data?.data?.length > 1 && DEBUG_METRICS) {
              const newest = rrdData.data.data[rrdData.data.data.length - 1].time;
              const secondNewest = rrdData.data.data[rrdData.data.data.length - 2].time;
               console.log(`Container ${ct.name || ct.vmid} metrics update interval: ${newest - secondNewest} seconds`);
               if (currentData?.data?.data) console.log(`Container ${ct.name || ct.vmid} current metrics: CPU=${currentData.data.data.cpu}, Memory=${currentData.data.data.mem}`, `[RRD last update: ${new Date(newest * 1000).toISOString()}]`);
            }
          } catch (err) {
            console.error(`Failed to get metrics for container ${ct.vmid} on node ${nodeName}: ${err.message}`);
          }
        }
      }
    } catch (err) {
      console.error(`Error fetching containers from ${nodeName}: ${err.message}`);
    }
  } // End loop through nodesToQuery

  // console.log(`Collected raw data: ${rawData.nodes.length} nodes, ${rawData.vms.length} VMs, ${rawData.containers.length} containers, ${rawData.metrics.length} metric sets`);
  return rawData;
}

// Socket.io connection handling
io.on('connection', (socket) => {
  connectedClients++;
  // console.log(`Client connected. Total clients: ${connectedClients}`);
  
  // Fetch and send initial data, log node count on first successful fetch
  fetchRawProxmoxData().then(data => {
    socket.emit('rawData', data);
    if (!initialNodesLogged && data && data.nodes && data.nodes.length > 0) {
      console.log(`Initial connection successful. Found ${data.nodes.length} Proxmox node(s).`);
      initialNodesLogged = true;
    } else if (!initialNodesLogged && data && data.nodes && data.nodes.length === 0) {
      console.log('Initial connection successful. Found 0 Proxmox nodes.');
      initialNodesLogged = true;
    } else if (!initialNodesLogged) {
      // Log if initial fetch failed but connection handler still ran
      console.warn('Initial Proxmox data fetch failed or returned no nodes.');
      initialNodesLogged = true; // Prevent repeated warnings
    }
  }).catch(error => {
    console.error('Error fetching initial Proxmox data for client:', error.message);
    if (!initialNodesLogged) {
        initialNodesLogged = true; // Prevent repeated warnings even on error
    }
  });
  
  // Handle disconnect
  socket.on('disconnect', () => {
    connectedClients--;
    // console.log(`Client disconnected. Total clients: ${connectedClients}`);
  });
});

// Periodic update interval for all connected clients
const updateInterval = setInterval(async () => {
  if (connectedClients > 0) {
    try {
      // console.log(`Updating raw data for ${connectedClients} client(s)...`);
      const data = await fetchRawProxmoxData();
      io.emit('rawData', data);
    } catch (error) {
      console.error(`Error in update interval: ${error.message}`);
    }
  }
}, UPDATE_INTERVAL);

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