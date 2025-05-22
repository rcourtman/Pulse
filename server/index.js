require('dotenv').config(); // Load environment variables from .env file

// Import the state manager FIRST
const stateManager = require('./state');

// --- BEGIN Configuration Loading using configLoader --- 
const { loadConfiguration, ConfigurationError } = require('./configLoader');

let endpoints;
let pbsConfigs;
let configIsPlaceholder = false; // Define placeholder flag variable here

try {
  const { endpoints: loadedEndpoints, pbsConfigs: loadedPbsConfigs, isConfigPlaceholder: loadedPlaceholderFlag } = loadConfiguration();
  endpoints = loadedEndpoints;
  pbsConfigs = loadedPbsConfigs;
  configIsPlaceholder = loadedPlaceholderFlag; // Store flag temporarily
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

// Set the placeholder status in stateManager *after* config loading is complete
stateManager.setConfigPlaceholderStatus(configIsPlaceholder);

const express = require('express');
const http = require('http');
const path = require('path');
const cors = require('cors');
const { Server } = require('socket.io');
const { URL } = require('url'); // <--- ADD: Import URL constructor
const axiosRetry = require('axios-retry').default; // Import axios-retry
const database = require('./database'); // +PulseDB

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

// +PulseDB: New API Endpoint for history
app.get('/api/history/:guestUniqueId/:metricName', (req, res) => {
    const { guestUniqueId, metricName } = req.params;
    const durationQuery = req.query.durationSeconds || req.query.duration; // Support both query param names
    const duration = durationQuery ? parseInt(durationQuery) : 3600; // Default 1 hour (3600 seconds)

    if (!guestUniqueId || !metricName) {
        return res.status(400).json({ error: 'guestUniqueId and metricName are required parameters.' });
    }
    if (isNaN(duration) || duration <= 0) {
        return res.status(400).json({ error: 'Invalid duration parameter.' });
    }

    database.getMetricsForGuest(guestUniqueId, metricName, duration, (err, data) => {
        if (err) {
            console.error(`[API /history] Error fetching history for ${guestUniqueId}/${metricName}:`, err);
            return res.status(500).json({ error: 'Failed to fetch metric history.' });
        }
        res.json(data); // Send back the array of {timestamp, value}
    });
});

// --- WebSocket Setup ---
const io = new Server(server, {
  // Optional: Configure CORS for Socket.IO if needed, separate from Express CORS
  cors: {
    origin: "*", // Allow all origins for Socket.IO, adjust as needed for security
    methods: ["GET", "POST"]
  }
});

function sendCurrentStateToSocket(socket) {
  const fullCurrentState = stateManager.getState(); // This includes isConfigPlaceholder
  const currentPlaceholderStatus = fullCurrentState.isConfigPlaceholder; // Extract for clarity if needed

  if (stateManager.hasData()) {
    socket.emit('rawData', fullCurrentState);
  } else {
    console.log('No data available yet, sending initial/loading state.');
    socket.emit('initialState', { loading: true, isConfigPlaceholder: currentPlaceholderStatus });
  }
}

io.on('connection', (socket) => {
  console.log('Client connected');
  sendCurrentStateToSocket(socket);

  socket.on('requestData', async () => {
    console.log('Client requested data');
    try {
      sendCurrentStateToSocket(socket);
      // Optionally trigger an immediate discovery cycle?
      // runDiscoveryCycle(); // Be careful with triggering cycles on demand
    } catch (error) {
      console.error('Error processing requestData event:', error);
      // Notify client of error? Consider emitting an error event to the specific socket
      // socket.emit('requestError', { message: 'Failed to process your request.' });
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

// +PulseDB: Aggregation globals
let metricAggregationBuffers = {}; // { [guestUniqueId]: { [metricKey]: [{timestamp, value}], ... } }
const AGGREGATION_INTERVAL_MS = 5000; // 5 seconds (was 60 * 1000; // 1 minute)
let lastAggregationTime = Date.now();
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

    // Emit combined data using updated state manager state (which includes the flag)
    if (io.engine.clientsCount > 0) {
        io.emit('rawData', updatedState);
    }
  } catch (error) {
      console.error(`[Discovery Cycle] Error during execution: ${error.message}`, error.stack);
  } finally {
      isDiscoveryRunning = false;
      scheduleNextDiscovery();
  }
}

// +PulseDB: New function to process and store aggregated metrics
function processAndStoreAggregatedMetrics() {
    // console.log('****** AGGREGATOR ****** Entered processAndStoreAggregatedMetrics.');
    const processingTimestamp = Date.now(); // Use milliseconds for processing timestamp

    for (const guestUniqueId in metricAggregationBuffers) {
        // console.log(`****** AGGREGATOR ****** Processing guest: ${guestUniqueId}`);
        const guestBuffers = metricAggregationBuffers[guestUniqueId];
        for (const metricKey in guestBuffers) {
            // console.log(`****** AGGREGATOR ******   Metric: ${metricKey}`);
            const dataPoints = guestBuffers[metricKey];
            // console.log(`****** AGGREGATOR ******     Found ${dataPoints.length} dataPoints.`);

            // console.log(`****** AGGREGATOR ******     Processing metricKey: '${metricKey}' BEFORE conditional checks.`);

            if (dataPoints.length === 0) continue;

            let aggregatedValue;
            const firstPointTime = dataPoints[0].timestamp;
            const lastPointTime = dataPoints[dataPoints.length - 1].timestamp;
            const durationSeconds = Math.max(1, (lastPointTime - firstPointTime) / 1000);

            if (metricKey.endsWith('_total_bytes')) {
                if (dataPoints.length < 2) {
                    aggregatedValue = 0;
                    // console.log(`****** AGGREGATOR ******     Skipping rate for ${metricKey} (guest: ${guestUniqueId}) due to insufficient data points (${dataPoints.length}).`);
                } else {
                    const totalChange = dataPoints[dataPoints.length - 1].value - dataPoints[0].value;
                    if (totalChange < 0) {
                        aggregatedValue = 0; 
                        // console.log(`****** AGGREGATOR ******     Rate for ${metricKey} (guest: ${guestUniqueId}) is 0 due to counter reset/anomaly (totalChange: ${totalChange}).`);
                    } else {
                        aggregatedValue = totalChange / durationSeconds;
                    }
                }
                const dbMetricName = metricKey.replace('_total_bytes', '_bytes_per_sec');
                // console.log(`****** AGGREGATOR ******     Attempting to insert rate for ${dbMetricName} (guest: ${guestUniqueId}), value: ${aggregatedValue}`);
                database.insertMetricData(processingTimestamp, guestUniqueId, dbMetricName, aggregatedValue, (err) => {
                    if (err) {
                        // console.error(`****** AGGREGATOR ****** DB ERROR for ${guestUniqueId} - ${dbMetricName}:`, err);
                    } else {
                        // console.log(`****** AGGREGATOR ****** DB SUCCESS for ${guestUniqueId} - ${dbMetricName}, value: ${aggregatedValue}`);
                    }
                });
            } else if (metricKey.endsWith('_usage_percent')) {
                const sum = dataPoints.reduce((acc, p) => acc + p.value, 0);
                aggregatedValue = sum / dataPoints.length;
                // console.log(`****** AGGREGATOR ******     Attempting to insert average for ${metricKey} (guest: ${guestUniqueId}), value: ${aggregatedValue}`);
                database.insertMetricData(processingTimestamp, guestUniqueId, metricKey, aggregatedValue, (err) => {
                    if (err) {
                        // console.error(`****** AGGREGATOR ****** DB ERROR for ${guestUniqueId} - ${metricKey}:`, err);
                    } else {
                        // console.log(`****** AGGREGATOR ****** DB SUCCESS for ${guestUniqueId} - ${metricKey}, value: ${aggregatedValue}`);
                    }
                });
            }
        }
        metricAggregationBuffers[guestUniqueId] = {}; 
    }
    // console.log('****** AGGREGATOR ****** Exiting processAndStoreAggregatedMetrics.');
}

async function runMetricCycle() {
    // console.log('[PulseDB DEBUG] runMetricCycle: Started.');
    const currentTime = Date.now();
    if (isMetricsRunning) return;
    if (io.engine.clientsCount === 0 && Object.keys(metricAggregationBuffers).length === 0) { // Also check buffers
        // console.log('[PulseDB DEBUG] runMetricCycle: Skipping - No clients and no buffers.');
        scheduleNextMetricCycle(METRIC_UPDATE_INTERVAL); // Still schedule, in case config changes
        return;
    }
    isMetricsRunning = true;
    try {
        if (Object.keys(apiClients).length === 0 && Object.keys(metricAggregationBuffers).length === 0) { // Also check buffers
            console.warn("[Metrics Cycle] PVE API clients not initialized and no buffers to process, skipping run.");
            // console.log('[PulseDB DEBUG] runMetricCycle: Skipping - PVE API clients not initialized and no buffers.');
            isMetricsRunning = false; // Reset flag before returning
            scheduleNextMetricCycle(METRIC_UPDATE_INTERVAL);
            return;
        }
        
        const { vms: currentVms, containers: currentContainers } = stateManager.getState();
        const runningVms = currentVms.filter(vm => vm.status === 'running');
        const runningContainers = currentContainers.filter(ct => ct.status === 'running');

        if (runningVms.length > 0 || runningContainers.length > 0) {
            // console.log(`[PulseDB DEBUG] runMetricCycle: About to call fetchMetricsData. Running VMs: ${runningVms.length}, Running CTs: ${runningContainers.length}`);
            try {
                // Pass the global aggregation buffers to fetchMetricsData
                const fetchedMetrics = await fetchMetricsData(runningVms, runningContainers, apiClients, metricAggregationBuffers);

                if (fetchedMetrics && fetchedMetrics.length >= 0) {
                   stateManager.updateMetricsData(fetchedMetrics);
                }
                // Emit rawData for live view, as before
                io.emit('rawData', stateManager.getState());
            } catch (error) {
                console.error(`[Metrics Cycle] Error during execution: ${error.message}`, error.stack);
            }
        } else {
            // console.log('[PulseDB DEBUG] runMetricCycle: No running guests found for metric fetching.');
        }

        // Aggregation Logic
        if (currentTime - lastAggregationTime >= AGGREGATION_INTERVAL_MS) {
            // console.log('[PulseDB DEBUG] runMetricCycle: Aggregation interval reached. About to call processAndStoreAggregatedMetrics.');
            processAndStoreAggregatedMetrics();
            lastAggregationTime = currentTime;
        } else {
            // console.log(`[Metrics Cycle] Next aggregation in ${Math.round((AGGREGATION_INTERVAL_MS - (currentTime - lastAggregationTime))/1000)}s`);
        }

    } catch (error) {
        console.error(`[Metrics Cycle] Error during execution: ${error.message}`, error.stack);
    } finally {
        isMetricsRunning = false;
        scheduleNextMetricCycle(METRIC_UPDATE_INTERVAL);
    }
}

function scheduleNextDiscovery() {
  if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
  // Use the constant defined earlier
  discoveryTimeoutId = setTimeout(runDiscoveryCycle, DISCOVERY_UPDATE_INTERVAL); 
}

function scheduleNextMetricCycle(delay) {
    // console.log('[PulseDB DEBUG] scheduleNextMetric: Called.');
    if (metricTimeoutId) clearTimeout(metricTimeoutId);
    // Use the constant defined earlier
    metricTimeoutId = setTimeout(runMetricCycle, delay); 
}
// --- End Schedulers ---

// --- Start the server ---
async function startServer() {
    // +PulseDB: Initialize Database first
    try {
        await new Promise((resolve, reject) => {
            database.initDatabase((err) => {
                if (err) {
                    console.error("FATAL: Failed to initialize database:", err);
                    return reject(err); // Ensure promise is rejected
                }
                console.log("[Main] Database initialized successfully.");
                // Start periodic pruning after DB is initialized
                setInterval(() => {
                    database.pruneOldData(7, (pruneErr, changes) => { // Keep 7 days of data
                        if (pruneErr) {
                            console.error("[PruneTask] Error pruning old data:", pruneErr);
                        } else if (changes > 0) {
                            console.log(`[PruneTask] Pruned ${changes} old records.`);
                        }
                    });
                }, 24 * 60 * 60 * 1000); // Run once every 24 hours
                resolve();
            });
        });
    } catch (dbInitError) {
        console.error("FATAL: Database initialization failed. Server cannot start.", dbInitError);
        process.exit(1);
    }

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

    // Ensure 'server' is your http.Server instance (e.g., const server = http.createServer(app);)
    server.listen(PORT, () => {
        console.log(`Server listening on port ${PORT}`);
        scheduleNextMetricCycle(METRIC_UPDATE_INTERVAL); 
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

// +PulseDB: Graceful shutdown
process.on('SIGINT', async () => {
    console.log('SIGINT signal received: closing HTTP server and database...');
    if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
    if (metricTimeoutId) clearTimeout(metricTimeoutId);

    // Close server, then database
    if (server && server.listening) { // 'server' is the http.Server instance from Express
        server.close(() => {
            console.log('HTTP server closed.');
            database.closeDatabase((dbErr) => {
                if (dbErr) console.error("[Shutdown] Error closing database:", dbErr);
                else console.log("[Shutdown] Database connection closed.");
                process.exit(0);
            });
        });
    } else {
        database.closeDatabase((dbErr) => {
            if (dbErr) console.error("[Shutdown] Error closing database (server not listening):", dbErr);
            else console.log("[Shutdown] Database connection closed (server not listening).");
            process.exit(0);
        });
    }
});

// Add the Express app instance if it's not globally available
// This assumes your 'server' variable is created like: 
// const app = express(); const server = http.createServer(app);
// If 'app' is already global, use that.
// For this example, let's assume 'app' needs to be explicitly set up for the new route.
// If you define 'app' elsewhere, ensure this route is added to it.

// Ensure Express app is initialized before adding routes
// This is a common pattern. If your app is initialized elsewhere, adapt as needed.
// const app = express(); // <<< ---- REMOVE THIS LINE

// Make sure to use cors and any other middleware BEFORE your routes
// app.use(cors()); // <<< ---- REMOVE THIS LINE
// app.use(express.json()); // If you need to parse JSON request bodies for other routes // <<< ---- REMOVE THIS LINE

// Re-integrate with your existing server creation if you have one
// If your server is created like: const server = http.createServer(app);
// And io is attached to that server: const io = new Server(server, ...);
// Ensure this 'app' is the one used by your http.createServer call.

// This is a placeholder for where your server and io are usually initialized.
// You will need to merge this with your existing server setup.
// For example, if you have:
// const app = express();
// ... (your middleware and existing routes) ...
// const server = http.createServer(app);
// const io = new Server(server, ...);
// Then the app.get('/api/history...') route should be added to that 'app' instance.
// And the server instance used in SIGINT should be that same server.

// The following line is only if 'server' wasn't defined before.
// Ensure this aligns with how your http server and Socket.IO are set up.
// const server = http.createServer(app); // This might conflict if server is already defined.

// The call to startServer() should ideally be the last thing.
startServer();
