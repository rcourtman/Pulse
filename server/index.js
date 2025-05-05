require('dotenv').config(); // Load environment variables from .env file

// --- BEGIN Configuration Loading using configLoader --- 
const { loadConfiguration, ConfigurationError } = require('./configLoader');

let endpoints;
let pbsConfigs;
let isConfigPlaceholder = false; // Add global flag

try {
  ({ endpoints, pbsConfigs, isConfigPlaceholder } = loadConfiguration());
} catch (error) {
  if (error instanceof ConfigurationError) {
    console.error(error.message);
    process.exit(1); // Exit if configuration loading failed
  } else {
    console.error('An unexpected error occurred during configuration loading:', error);
    process.exit(1); // Exit on other unexpected errors during load
  }
}
// --- END Configuration Loading ---

const fs = require('fs'); // Add fs module
const express = require('express');
const http = require('http');
const path = require('path');
const cors = require('cors');
const { Server } = require('socket.io');
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

// --- API Client Initialization ---
const { initializeApiClients } = require('./apiClients');
let apiClients = {};   // Initialize as empty objects
let pbsApiClients = {};
// Note: Client initialization is now async and happens in startServer()
// --- END API Client Initialization ---

// --- REMOVED OLD CLIENT INIT LOGIC --- 
// The following blocks were moved to apiClients.js
// endpoints.forEach(endpoint => { ... });
// async function initializeAllPbsClients() { ... }
// --- END REMOVED OLD CLIENT INIT LOGIC ---

// --- Data Fetching (Imported) ---
const { fetchDiscoveryData, fetchMetricsData } = require('./dataFetcher');
// --- END Data Fetching ---

// Import the state manager
const stateManager = require('./state');

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
// Health check endpoint
app.get('/healthz', (req, res) => {
    res.status(200).send('OK');
});

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
        // Get current nodes from state manager
        const { nodes: currentNodes } = stateManager.getState();
        const storageInfoByNode = {};
        (currentNodes || []).forEach(node => {
            storageInfoByNode[node.node] = node.storage || []; 
        });
        res.json(storageInfoByNode); 
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
  const currentState = stateManager.getState();
  if (stateManager.hasData()) {
      // Include flag in rawData
      socket.emit('rawData', { ...currentState, isConfigPlaceholder });
  } else {
      console.log('No data available yet on connect, sending initial state.');
      // Include flag in initialState
      socket.emit('initialState', { loading: true, isConfigPlaceholder });
  }

  socket.on('requestData', async () => {
    console.log('Client requested data');
    try {
      const currentState = stateManager.getState();
      if (stateManager.hasData()) {
          // Include flag in rawData
          socket.emit('rawData', { ...currentState, isConfigPlaceholder });
      } else {
         console.log('No data available yet on request, sending loading state.');
         // Include flag in initialState
         socket.emit('initialState', { loading: true, isConfigPlaceholder });
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
// let currentNodes = [];
// let currentVms = [];
// let currentContainers = [];
// let currentMetrics = [];
// let pbsDataArray = []; // Array to hold data for each PBS instance
let isDiscoveryRunning = false; // Prevent concurrent discovery runs
let isMetricsRunning = false;   // Prevent concurrent metric runs
let discoveryTimeoutId = null;
let metricTimeoutId = null;
// --- End Global State ---

// --- Data Fetching Helper Functions (MOVED TO dataFetcher.js) ---
// async function fetchDataForNode(...) { ... } // MOVED

// --- Main Data Fetching Logic (MOVED TO dataFetcher.js) ---
// async function fetchDiscoveryData(...) { ... } // MOVED
// async function fetchMetricsData(...) { ... } // MOVED

// --- Update Cycle Logic --- 
// Uses imported fetch functions and updates global state
async function runDiscoveryCycle() {
  if (isDiscoveryRunning) return;
  isDiscoveryRunning = true;
  try {
    if (Object.keys(apiClients).length === 0 && Object.keys(pbsApiClients).length === 0) {
        console.warn("[Discovery Cycle] API clients not initialized yet, skipping run.");
    return;
  }
    // Use imported fetchDiscoveryData
    const discoveryData = await fetchDiscoveryData(apiClients, pbsApiClients);
    
    // Update state using the state manager
    stateManager.updateDiscoveryData(discoveryData);
    // No need to store in global vars anymore

    // ... (logging summary) ...
    const updatedState = stateManager.getState(); // Get the fully updated state
    console.log(`[Discovery Cycle] Updated state. Nodes: ${updatedState.nodes.length}, VMs: ${updatedState.vms.length}, CTs: ${updatedState.containers.length}, PBS: ${updatedState.pbs.length}`);

    // Emit combined data using updated state manager state, including the flag
    if (io.engine.clientsCount > 0) {
        io.emit('rawData', { ...updatedState, isConfigPlaceholder });
    }
  } catch (error) {
      console.error(`[Discovery Cycle] Error during execution: ${error.message}`, error.stack);
  } finally {
      isDiscoveryRunning = false;
      scheduleNextDiscovery();
  }
}

async function runMetricCycle() {
  if (isMetricsRunning) return;
  if (io.engine.clientsCount === 0) {
    scheduleNextMetric(); 
    return;
  }
  isMetricsRunning = true;
  try {
    if (Object.keys(apiClients).length === 0) {
        console.warn("[Metrics Cycle] PVE API clients not initialized yet, skipping run.");
        return;
    }
    // Use global state for running guests
    const { vms: currentVms, containers: currentContainers } = stateManager.getState();
    const runningVms = currentVms.filter(vm => vm.status === 'running');
    const runningContainers = currentContainers.filter(ct => ct.status === 'running');

    if (runningVms.length > 0 || runningContainers.length > 0) {
        // Use imported fetchMetricsData
        const fetchedMetrics = await fetchMetricsData(runningVms, runningContainers, apiClients);

        // Update metrics state
        if (fetchedMetrics && fetchedMetrics.length >= 0) { // Allow empty array to clear metrics
           stateManager.updateMetricsData(fetchedMetrics);
           // Emit only metrics updates if needed, or rely on full rawData updates?
           // Consider emitting a smaller 'metricsUpdate' event if performance is key
           // io.emit('metricsUpdate', stateManager.getState().metrics);
        }

        // Emit rawData with updated global state (including metrics)
        io.emit('rawData', stateManager.getState());
    } else {
        const currentMetrics = stateManager.getState().metrics;
        if (currentMetrics.length > 0) {
           console.log('[Metrics Cycle] No running guests found, clearing metrics.');
           stateManager.clearMetricsData(); // Clear metrics
           // Emit state update with cleared metrics only if clients are connected
           // (Avoid unnecessary emits if no one is listening and nothing changed except clearing metrics)
           if (io.engine.clientsCount > 0) {
               io.emit('rawData', stateManager.getState());
           }
        }
    }
  } catch (error) {
      console.error(`[Metrics Cycle] Error during execution: ${error.message}`, error.stack);
  } finally {
      isMetricsRunning = false;
      scheduleNextMetric();
  }
}

// --- Schedulers --- 
function scheduleNextDiscovery() {
  if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
  // Use the constant defined earlier
  discoveryTimeoutId = setTimeout(runDiscoveryCycle, DISCOVERY_UPDATE_INTERVAL); 
}

function scheduleNextMetric() {
  if (metricTimeoutId) clearTimeout(metricTimeoutId);
   // Use the constant defined earlier
  metricTimeoutId = setTimeout(runMetricCycle, METRIC_UPDATE_INTERVAL); 
}
// --- End Schedulers ---

// --- Start the server ---
async function startServer() {
    try {
        // Use the correct initializer function name
        const initializedClients = await initializeApiClients(endpoints, pbsConfigs);
        apiClients = initializedClients.apiClients;
        pbsApiClients = initializedClients.pbsApiClients;
        console.log("INFO: All API clients initialized.");
    } catch (initError) {
        console.error("FATAL: Failed to initialize API clients:", initError);
        process.exit(1); // Exit if clients can't be initialized
    }
    
    await runDiscoveryCycle(); 

    server.listen(PORT, () => {
        console.log(`Server listening on port ${PORT}`);
        // Schedule the first metric run *after* the initial discovery completes and server is listening
        scheduleNextMetric(); 
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

// --- PBS Data Fetching Functions (MOVED TO dataFetcher.js / pbsUtils.js) ---
// async function fetchPbsNodeName(...) { ... } // MOVED
// async function fetchAllPbsTasksForProcessing(...) { ... } // MOVED
// function processPbsTasks(...) { ... } // MOVED
// async function fetchPbsDatastoreData(...) { ... } // MOVED