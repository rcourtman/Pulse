// Load environment variables from .env file
// Check for persistent config directory (Docker) or use project root
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

// Import the state manager FIRST
const stateManager = require('./state');

// Import metrics history system
const metricsHistory = require('./metricsHistory');

// Import diagnostic tool
const DiagnosticTool = require('./diagnostics');

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

// Store globally for config reload
global.pulseConfigStatus = { isPlaceholder: configIsPlaceholder };

// Set endpoint configurations for client use
stateManager.setEndpointConfigurations(endpoints, pbsConfigs);

const express = require('express');
const http = require('http');
const cors = require('cors');
const compression = require('compression');
const { Server } = require('socket.io');
const { URL } = require('url'); // <--- ADD: Import URL constructor
const axios = require('axios');
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

// Configuration API
const ConfigApi = require('./configApi');
const configApi = new ConfigApi();

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

// Initialize enhanced state management
stateManager.init();

// Create Express app
const app = express();
const server = http.createServer(app); // Create HTTP server instance

// Middleware
app.use(compression({
  filter: (req, res) => {
    // Don't compress responses with this request header
    if (req.headers['x-no-compression']) {
      return false;
    }
    // Fallback to standard filter function
    return compression.filter(req, res);
  },
  threshold: 1024, // Only compress if response is over 1KB
  level: 6 // Compression level (1-9, 6 is good balance of speed vs compression)
}));
app.use(cors());
app.use(express.json());

// Define the public directory path
const publicDir = path.join(__dirname, '../src/public');

// Serve static files (CSS, JS, images) from the public directory
app.use(express.static(publicDir, { index: false }));

// Route to serve the main HTML file for the root path
app.get('/', (req, res) => {
  // Always serve the main application with settings modal for initial configuration
  const indexPath = path.join(publicDir, 'index.html');
  res.sendFile(indexPath, (err) => {
    if (err) {
      console.error(`Error sending index.html: ${err.message}`);
      // Avoid sending error details to the client for security
      res.status(err.status || 500).send('Internal Server Error loading page.');
    }
  });
});

// Route to explicitly handle setup page
app.get('/setup.html', (req, res) => {
  const setupPath = path.join(publicDir, 'setup.html');
  res.sendFile(setupPath, (err) => {
    if (err) {
      console.error(`Error sending setup.html: ${err.message}`);
      res.status(err.status || 500).send('Internal Server Error loading setup page.');
    }
  });
});

// --- API Routes ---
// Set up configuration API routes
configApi.setupRoutes(app);

// Set up threshold API routes
const { setupThresholdRoutes } = require('./thresholdRoutes');
setupThresholdRoutes(app);

// Set up update API routes
const UpdateManager = require('./updateManager');
const updateManager = new UpdateManager();

// Check for updates endpoint
app.get('/api/updates/check', async (req, res) => {
    try {
        const updateInfo = await updateManager.checkForUpdates();
        res.json(updateInfo);
    } catch (error) {
        console.error('Error checking for updates:', error);
        res.status(500).json({ error: error.message });
    }
});

// Download and apply update endpoint
app.post('/api/updates/apply', async (req, res) => {
    try {
        const { downloadUrl } = req.body;
        
        if (!downloadUrl) {
            return res.status(400).json({ error: 'Download URL is required' });
        }

        // Send immediate response
        res.json({ 
            message: 'Update started. The application will restart automatically when complete.',
            status: 'in_progress'
        });

        // Apply update in background
        setTimeout(async () => {
            try {
                // Download update
                const updateFile = await updateManager.downloadUpdate(downloadUrl, (progress) => {
                    io.emit('updateProgress', progress);
                });

                // Apply update
                await updateManager.applyUpdate(updateFile, (progress) => {
                    io.emit('updateProgress', progress);
                });

                io.emit('updateComplete', { success: true });
            } catch (error) {
                console.error('Error applying update:', error);
                io.emit('updateError', { error: error.message });
            }
        }, 100);

    } catch (error) {
        console.error('Error initiating update:', error);
        res.status(500).json({ error: error.message });
    }
});

// Update status endpoint
app.get('/api/updates/status', (req, res) => {
    try {
        const status = updateManager.getUpdateStatus();
        res.json(status);
    } catch (error) {
        console.error('Error getting update status:', error);
        res.status(500).json({ error: error.message });
    }
});

// Health check endpoint
app.get('/healthz', (req, res) => {
    res.status(200).send('OK');
});

// Enhanced health endpoint with detailed monitoring info
app.get('/api/health', (req, res) => {
    try {
        const healthSummary = stateManager.getHealthSummary();
        // Add system info including placeholder status
        const state = stateManager.getState();
        healthSummary.system = {
            configPlaceholder: state.isConfigPlaceholder || false,
            hasData: stateManager.hasData(),
            clientsInitialized: Object.keys(global.pulseApiClients?.apiClients || {}).length > 0
        };
        res.json(healthSummary);
    } catch (error) {
        console.error("Error in /api/health:", error);
        res.status(500).json({ error: "Failed to fetch health information" });
    }
});

// Performance metrics endpoint
app.get('/api/performance', (req, res) => {
    try {
        const limit = parseInt(req.query.limit) || 50;
        const performanceHistory = stateManager.getPerformanceHistory(limit);
        const connectionHealth = stateManager.getConnectionHealth();
        
        res.json({
            history: performanceHistory,
            connections: connectionHealth,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error("Error in /api/performance:", error);
        res.status(500).json({ error: "Failed to fetch performance data" });
    }
});

// Enhanced alerts endpoint with filtering
app.get('/api/alerts', (req, res) => {
    try {
        const filters = {
            severity: req.query.severity,
            group: req.query.group,
            node: req.query.node,
            acknowledged: req.query.acknowledged === 'true' ? true : 
                         req.query.acknowledged === 'false' ? false : undefined
        };
        
        const alertInfo = {
            active: stateManager.alertManager.getActiveAlerts(filters),
            stats: stateManager.alertManager.getEnhancedAlertStats(),
            rules: stateManager.alertManager.getRules()
        };
        
        res.json(alertInfo);
    } catch (error) {
        console.error("Error in /api/alerts:", error);
        res.status(500).json({ error: "Failed to fetch alert information" });
    }
});

// Alert history endpoint with pagination and filtering
app.get('/api/alerts/history', (req, res) => {
    try {
        const limit = parseInt(req.query.limit) || 100;
        const filters = {
            severity: req.query.severity,
            group: req.query.group,
            node: req.query.node
        };
        
        const history = stateManager.alertManager.getAlertHistory(limit, filters);
        res.json({ history, timestamp: Date.now() });
    } catch (error) {
        console.error("Error in /api/alerts/history:", error);
        res.status(500).json({ error: "Failed to fetch alert history" });
    }
});

// Alert acknowledgment endpoint
app.post('/api/alerts/:alertId/acknowledge', (req, res) => {
    try {
        const alertId = req.params.alertId;
        const { userId = 'api-user', note = '' } = req.body;
        
        const success = stateManager.alertManager.acknowledgeAlert(alertId, userId, note);
        
        if (success) {
            res.json({ success: true, message: "Alert acknowledged successfully" });
        } else {
            res.status(404).json({ error: "Alert not found" });
        }
    } catch (error) {
        console.error("Error acknowledging alert:", error);
        res.status(400).json({ error: error.message });
    }
});

// Alert suppression endpoint
app.post('/api/alerts/suppress', (req, res) => {
    try {
        const { ruleId, guestFilter = {}, duration = 3600000, reason = '' } = req.body;
        
        if (!ruleId) {
            return res.status(400).json({ error: "ruleId is required" });
        }
        
        const success = stateManager.alertManager.suppressAlert(ruleId, guestFilter, duration, reason);
        
        if (success) {
            res.json({ success: true, message: "Alert rule suppressed successfully" });
        } else {
            res.status(400).json({ error: "Failed to suppress alert rule" });
        }
    } catch (error) {
        console.error("Error suppressing alert:", error);
        res.status(400).json({ error: error.message });
    }
});

// Alert groups endpoint
app.get('/api/alerts/groups', (req, res) => {
    try {
        const stats = stateManager.alertManager.getEnhancedAlertStats();
        res.json({ groups: stats.groups });
    } catch (error) {
        console.error("Error in /api/alerts/groups:", error);
        res.status(500).json({ error: "Failed to fetch alert groups" });
    }
});

// Notification channels endpoint
app.get('/api/alerts/channels', (req, res) => {
    try {
        const stats = stateManager.alertManager.getEnhancedAlertStats();
        res.json({ channels: stats.channels });
    } catch (error) {
        console.error("Error in /api/alerts/channels:", error);
        res.status(500).json({ error: "Failed to fetch notification channels" });
    }
});

// Enhanced alert metrics endpoint
app.get('/api/alerts/metrics', (req, res) => {
    try {
        const stats = stateManager.alertManager.getEnhancedAlertStats();
        res.json({
            metrics: stats.metrics,
            summary: {
                active: stats.active,
                acknowledged: stats.acknowledged,
                escalated: stats.escalated,
                suppressed: stats.suppressedRules
            },
            trends: {
                last24Hours: stats.last24Hours,
                lastHour: stats.lastHour
            },
            timestamp: Date.now()
        });
    } catch (error) {
        console.error("Error in /api/alerts/metrics:", error);
        res.status(500).json({ error: "Failed to fetch alert metrics" });
    }
});

// Alert rules management with filtering
app.get('/api/alerts/rules', (req, res) => {
    try {
        const filters = {
            group: req.query.group,
            severity: req.query.severity
        };
        
        const rules = stateManager.alertManager.getRules(filters);
        res.json({ rules });
    } catch (error) {
        console.error("Error in /api/alerts/rules:", error);
        res.status(500).json({ error: "Failed to fetch alert rules" });
    }
});

// Create new alert rule
app.post('/api/alerts/rules', (req, res) => {
    try {
        const rule = req.body;
        const newRule = stateManager.alertManager.addRule(rule);
        res.json({ success: true, message: "Rule added successfully", rule: newRule });
    } catch (error) {
        console.error("Error adding alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Update alert rule
app.put('/api/alerts/rules/:id', (req, res) => {
    try {
        const ruleId = req.params.id;
        const updates = req.body;
        const success = stateManager.alertManager.updateRule(ruleId, updates);
        
        if (success) {
            res.json({ success: true, message: "Rule updated successfully" });
        } else {
            res.status(404).json({ error: "Rule not found" });
        }
    } catch (error) {
        console.error("Error updating alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Delete alert rule
app.delete('/api/alerts/rules/:id', (req, res) => {
    try {
        const ruleId = req.params.id;
        const success = stateManager.alertManager.removeRule(ruleId);
        
        if (success) {
            res.json({ success: true, message: "Rule removed successfully" });
        } else {
            res.status(404).json({ error: "Rule not found" });
        }
    } catch (error) {
        console.error("Error removing alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Version check functionality
let latestVersionCache = null;
let lastVersionCheck = 0;
const VERSION_CHECK_INTERVAL = 6 * 60 * 60 * 1000; // 6 hours

async function checkLatestVersion() {
    const now = Date.now();
    
    // Return cached version if still fresh
    if (latestVersionCache && (now - lastVersionCheck) < VERSION_CHECK_INTERVAL) {
        return latestVersionCache;
    }
    
    try {
        const response = await axios.get('https://api.github.com/repos/rcourtman/Pulse/releases/latest', {
            timeout: 5000,
            headers: {
                'Accept': 'application/vnd.github.v3+json'
            }
        });
        
        if (response.data && response.data.tag_name) {
            // Remove 'v' prefix if present
            const version = response.data.tag_name.replace(/^v/, '');
            latestVersionCache = version;
            lastVersionCheck = now;
            return version;
        }
    } catch (error) {
        console.error('Error checking latest version:', error.message);
    }
    
    return null;
}

// Version API endpoint
app.get('/api/version', async (req, res) => {
    try {
        const packageJson = require('../package.json');
        const currentVersion = packageJson.version || 'N/A';
        
        // Check for latest version
        const latestVersion = await checkLatestVersion();
        
        res.json({ 
            version: currentVersion,
            latestVersion: latestVersion,
            updateAvailable: latestVersion && latestVersion !== currentVersion && 
                            compareVersions(latestVersion, currentVersion) > 0
        });
    } catch (error) {
         console.error("Error in version endpoint:", error);
         res.status(500).json({ error: "Could not retrieve version" });
    }
});

// Simple version comparison function
function compareVersions(v1, v2) {
    const parts1 = v1.split('.').map(Number);
    const parts2 = v2.split('.').map(Number);
    
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
        const part1 = parts1[i] || 0;
        const part2 = parts2[i] || 0;
        
        if (part1 > part2) return 1;
        if (part1 < part2) return -1;
    }
    
    return 0;
}

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

// Chart data API endpoint
app.get('/api/charts', async (req, res) => {
    try {
        // Get current guest info for context
        const currentState = stateManager.getState();
        const guestInfoMap = {};
        
        // Build guest info map
        [...(currentState.vms || []), ...(currentState.containers || [])].forEach(guest => {
            const guestId = `${guest.endpointId}-${guest.node}-${guest.vmid}`;
            guestInfoMap[guestId] = {
                maxmem: guest.maxmem,
                maxdisk: guest.maxdisk,
                type: guest.type
            };
        });
        
        const chartData = metricsHistory.getAllGuestChartData(guestInfoMap);
        const stats = metricsHistory.getStats();
        
        res.json({
            data: chartData,
            stats: stats,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error("Error in /api/charts:", error);
        res.status(500).json({ error: error.message || "Failed to fetch chart data." });
    }
});


// Direct state inspection endpoint
app.get('/api/diagnostics-state', (req, res) => {
    try {
        const state = stateManager.getState();
        const summary = {
            timestamp: new Date().toISOString(),
            last_update: state.lastUpdate,
            update_age_seconds: state.lastUpdate ? Math.floor((Date.now() - new Date(state.lastUpdate).getTime()) / 1000) : null,
            guests_count: state.guests?.length || 0,
            nodes_count: state.nodes?.length || 0,
            pbs_count: state.pbs?.length || 0,
            sample_guests: state.guests?.slice(0, 5).map(g => ({
                vmid: g.vmid,
                name: g.name,
                type: g.type,
                status: g.status
            })) || [],
            sample_backups: [],
            errors: state.errors || []
        };
        
        // Get sample backups
        if (state.pbs && Array.isArray(state.pbs)) {
            state.pbs.forEach(pbsInstance => {
                if (pbsInstance.datastores) {
                    pbsInstance.datastores.forEach(ds => {
                        if (ds.snapshots && ds.snapshots.length > 0) {
                            ds.snapshots.slice(0, 5).forEach(snap => {
                                summary.sample_backups.push({
                                    store: ds.store,
                                    backup_id: snap['backup-id'],
                                    backup_type: snap['backup-type'],
                                    backup_time: new Date(snap['backup-time'] * 1000).toISOString()
                                });
                            });
                        }
                    });
                }
            });
        }
        
        res.json(summary);
    } catch (error) {
        console.error("State inspection error:", error);
        res.status(500).json({ error: error.message });
    }
});

// Quick diagnostic check endpoint
app.get('/api/diagnostics/check', async (req, res) => {
    try {
        // Use cached result if available and recent
        const cacheKey = 'diagnosticCheck';
        const cached = global.diagnosticCache?.[cacheKey];
        if (cached && (Date.now() - cached.timestamp) < 60000) { // Cache for 1 minute
            return res.json(cached.result);
        }

        // Run a quick check
        delete require.cache[require.resolve('./diagnostics')];
        const DiagnosticTool = require('./diagnostics');
        const diagnosticTool = new DiagnosticTool(stateManager, metricsHistory, apiClients, pbsApiClients);
        const report = await diagnosticTool.runDiagnostics();
        
        const hasIssues = report.recommendations && 
            report.recommendations.some(r => r.severity === 'critical' || r.severity === 'warning');
        
        const result = {
            hasIssues,
            criticalCount: report.recommendations?.filter(r => r.severity === 'critical').length || 0,
            warningCount: report.recommendations?.filter(r => r.severity === 'warning').length || 0
        };
        
        // Cache the result
        if (!global.diagnosticCache) global.diagnosticCache = {};
        global.diagnosticCache[cacheKey] = { timestamp: Date.now(), result };
        
        res.json(result);
    } catch (error) {
        console.error("Error in diagnostic check:", error);
        res.json({ hasIssues: false }); // Don't show icon on error
    }
});

// Raw state endpoint - shows everything
app.get('/api/raw-state', (req, res) => {
    const state = stateManager.getState();
    const rawState = stateManager.state || {};
    res.json({
        lastUpdate: state.lastUpdate,
        statsLastUpdated: state.stats?.lastUpdated,
        rawStateLastUpdated: rawState.stats?.lastUpdated,
        guestsLength: state.guests?.length,
        rawGuestsLength: rawState.guests?.length,
        guestsType: Array.isArray(state.guests) ? 'array' : typeof state.guests,
        allKeys: Object.keys(state),
        rawKeys: Object.keys(rawState),
        serverUptime: process.uptime(),
        // Sample guest to see structure
        firstGuest: state.guests?.[0],
        rawFirstGuest: rawState.guests?.[0]
    });
});

// --- Diagnostic Endpoint ---
app.get('/api/diagnostics', async (req, res) => {
    try {
        console.log('Running diagnostics...');
        // Force reload the diagnostic module to get latest changes
        delete require.cache[require.resolve('./diagnostics')];
        const DiagnosticTool = require('./diagnostics');
        const diagnosticTool = new DiagnosticTool(stateManager, metricsHistory, apiClients, pbsApiClients);
        const report = await diagnosticTool.runDiagnostics();
        
        // Format the report for easy reading
        const formattedReport = {
            ...report,
            summary: {
                hasIssues: report.recommendations && report.recommendations.some(r => r.severity === 'critical'),
                criticalIssues: report.recommendations ? report.recommendations.filter(r => r.severity === 'critical').length : 0,
                warnings: report.recommendations ? report.recommendations.filter(r => r.severity === 'warning').length : 0,
                info: report.recommendations ? report.recommendations.filter(r => r.severity === 'info').length : 0,
                isTimingIssue: report.state && report.state.dataAge === null && report.state.serverUptime < 90
            }
        };
        
        res.json(formattedReport);
    } catch (error) {
        console.error("Error running diagnostics:", error);
        console.error("Stack trace:", error.stack);
        res.status(500).json({ 
            error: "Failed to run diagnostics", 
            details: error.message,
            stack: error.stack
        });
    }
});

// Global error handler for unhandled API errors
app.use((err, req, res, next) => {
    console.error('Unhandled API error:', err);
    
    // Ensure we always return JSON for API routes
    if (req.url.startsWith('/api/')) {
        return res.status(500).json({
            success: false,
            error: 'Internal server error',
            message: err.message
        });
    }
    
    // For non-API routes, return HTML error
    res.status(500).send('Internal Server Error');
});

// 404 handler for API routes
app.use('/api/*', (req, res) => {
    res.status(404).json({
        success: false,
        error: 'API endpoint not found'
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
  const fullCurrentState = stateManager.getState(); // This includes isConfigPlaceholder and alerts
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

// Set up alert event forwarding to connected clients
stateManager.alertManager.on('alert', (alert) => {
    if (io.engine.clientsCount > 0) {
        io.emit('alert', alert);
    }
});

stateManager.alertManager.on('alertResolved', (alert) => {
    if (io.engine.clientsCount > 0) {
        io.emit('alertResolved', alert);
    }
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
  
  const startTime = Date.now();
  let errors = [];
  
  try {
    // Use global API clients if local ones aren't set
    const currentApiClients = global.pulseApiClients ? global.pulseApiClients.apiClients : apiClients;
    const currentPbsApiClients = global.pulseApiClients ? global.pulseApiClients.pbsApiClients : pbsApiClients;
    
    if (Object.keys(currentApiClients).length === 0 && Object.keys(currentPbsApiClients).length === 0) {
        console.warn("[Discovery Cycle] API clients not initialized yet, skipping run.");
    return;
  }
    // Use imported fetchDiscoveryData
    const discoveryData = await fetchDiscoveryData(currentApiClients, currentPbsApiClients);
    
    const duration = Date.now() - startTime;
    
    // Update state using the enhanced state manager
    stateManager.updateDiscoveryData(discoveryData, duration, errors);
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
      errors.push({ type: 'discovery', message: error.message, endpointId: 'general' });
      
      const duration = Date.now() - startTime;
      stateManager.updateDiscoveryData({ nodes: [], vms: [], containers: [], pbs: [] }, duration, errors);
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
  
  const startTime = Date.now();
  let errors = [];
  
  try {
    // Use global API clients if local ones aren't set
    const currentApiClients = global.pulseApiClients ? global.pulseApiClients.apiClients : apiClients;
    
    if (Object.keys(currentApiClients).length === 0) {
        console.warn("[Metrics Cycle] PVE API clients not initialized yet, skipping run.");
        return;
    }
    // Use global state for running guests
    const { vms: currentVms, containers: currentContainers } = stateManager.getState();
    const runningVms = currentVms.filter(vm => vm.status === 'running');
    const runningContainers = currentContainers.filter(ct => ct.status === 'running');

    if (runningVms.length > 0 || runningContainers.length > 0) {
        // Use imported fetchMetricsData
        const fetchedMetrics = await fetchMetricsData(runningVms, runningContainers, currentApiClients);

        const duration = Date.now() - startTime;

        // Update metrics state with enhanced error tracking
        if (fetchedMetrics && fetchedMetrics.length >= 0) { // Allow empty array to clear metrics
           stateManager.updateMetricsData(fetchedMetrics, duration, errors);
           
           // Add metrics to history for charts
           fetchedMetrics.forEach(metricData => {
               if (metricData && metricData.current) {
                   const guestId = `${metricData.endpointId}-${metricData.node}-${metricData.id}`;
                   metricsHistory.addMetricData(guestId, metricData.current);
               }
           });
           
           // Emit only metrics updates if needed, or rely on full rawData updates?
           // Consider emitting a smaller 'metricsUpdate' event if performance is key
           // io.emit('metricsUpdate', stateManager.getState().metrics);
        }

        // Emit rawData with updated global state (which includes metrics, alerts, and placeholder flag)
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
      errors.push({ type: 'metrics', message: error.message, endpointId: 'general' });
      
      const duration = Date.now() - startTime;
      stateManager.updateMetricsData([], duration, errors);
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

// Graceful shutdown handling
let shutdownInProgress = false;

function gracefulShutdown(signal) {
    if (shutdownInProgress) {
        console.log(`\nReceived ${signal} again, force exiting...`);
        process.exit(1);
    }
    
    shutdownInProgress = true;
    console.log(`\n${signal} signal received: closing HTTP server and cleaning up...`);
    
    // Force exit after 5 seconds if graceful shutdown takes too long
    const forceExitTimer = setTimeout(() => {
        console.log('Force exiting after 5 seconds...');
        process.exit(1);
    }, 5000);
    
    // Clear timers
    if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
    if (metricTimeoutId) clearTimeout(metricTimeoutId);
    
    // Clean up file watchers
    if (envWatcher) {
        envWatcher.close();
        envWatcher = null;
    }
    clearTimeout(reloadDebounceTimer);
    
    // Close WebSocket connections
    if (io) {
        io.close();
    }
    
    // Close server
    server.close((err) => {
        if (err) {
            console.error('Error closing server:', err);
        } else {
            console.log('HTTP server closed.');
        }
        
        // Cleanup state manager
        try {
            stateManager.destroy();
        } catch (cleanupError) {
            console.error('Error during state manager cleanup:', cleanupError);
        }
        
        clearTimeout(forceExitTimer);
        console.log('Cleanup completed. Exiting...');
        process.exit(0);
    });
    
    // If server.close doesn't call the callback (no active connections), 
    // still proceed with cleanup after a short delay
    setTimeout(() => {
        if (shutdownInProgress) {
            try {
                stateManager.destroy();
            } catch (cleanupError) {
                console.error('Error during fallback state manager cleanup:', cleanupError);
            }
            clearTimeout(forceExitTimer);
            console.log('Fallback cleanup completed. Exiting...');
            process.exit(0);
        }
    }, 1000);
}

process.on('SIGINT', () => gracefulShutdown('SIGINT'));
process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));

// --- Environment File Watcher ---
let envWatcher = null;
let reloadDebounceTimer = null;
let lastReloadTime = 0;
global.lastReloadTime = 0;  // Make it globally accessible

function setupEnvFileWatcher() {
    // Use same logic as ConfigApi to find .env file
    const configDir = path.join(__dirname, '../config');
    const configEnvPath = path.join(configDir, '.env');
    const projectEnvPath = path.join(__dirname, '../.env');
    
    const envPath = fs.existsSync(configEnvPath) ? configEnvPath : projectEnvPath;
    
    // Check if the file exists
    if (!fs.existsSync(envPath)) {
        console.log('No .env file found, skipping file watcher setup');
        return;
    }
    
    console.log('Setting up .env file watcher for automatic configuration reload');
    
    envWatcher = fs.watch(envPath, (eventType, filename) => {
        if (eventType === 'change') {
            // Debounce the reload to avoid multiple reloads for rapid changes
            clearTimeout(reloadDebounceTimer);
            reloadDebounceTimer = setTimeout(async () => {
                // Prevent reload if we just reloaded within the last 2 seconds (from API save)
                const now = Date.now();
                if (now - global.lastReloadTime < 2000) {
                    console.log('.env file changed but skipping reload (too recent)');
                    return;
                }
                
                console.log('.env file changed, reloading configuration...');
                global.lastReloadTime = now;
                
                try {
                    await configApi.reloadConfiguration();
                    
                    // Notify connected clients about configuration change
                    io.emit('configurationReloaded', { 
                        message: 'Configuration has been updated',
                        timestamp: Date.now()
                    });
                    
                    console.log('Configuration reloaded successfully');
                } catch (error) {
                    console.error('Failed to reload configuration:', error);
                    
                    // Notify clients about the error
                    io.emit('configurationError', {
                        message: 'Failed to reload configuration',
                        error: error.message,
                        timestamp: Date.now()
                    });
                }
            }, 1000); // Wait 1 second after last change before reloading
        }
    });
    
    envWatcher.on('error', (error) => {
        console.error('Error watching .env file:', error);
    });
}

// --- Start the server ---
async function startServer() {
    // Only initialize API clients if we have endpoints configured
    if (endpoints.length > 0 || pbsConfigs.length > 0) {
        try {
            // Use the correct initializer function name
            const initializedClients = await initializeApiClients(endpoints, pbsConfigs);
            apiClients = initializedClients.apiClients;
            pbsApiClients = initializedClients.pbsApiClients;
            
            // Store globally for config reload
            global.pulseApiClients = { apiClients, pbsApiClients };
            global.runDiscoveryCycle = runDiscoveryCycle;
            
            console.log("INFO: All API clients initialized.");
        } catch (initError) {
            console.error("FATAL: Failed to initialize API clients:", initError);
            process.exit(1); // Exit if clients can't be initialized
        }
        
        // Run initial discovery cycle in background after server starts
        setImmediate(() => {
            runDiscoveryCycle().catch(error => {
                console.error('Error in initial discovery cycle:', error);
            });
        });
    } else {
        console.log("INFO: No endpoints configured. Starting in setup mode.");
        // Initialize empty clients for consistency
        apiClients = {};
        pbsApiClients = {};
        global.pulseApiClients = { apiClients, pbsApiClients };
        global.runDiscoveryCycle = runDiscoveryCycle;
    } 

    server.listen(PORT, () => {
        console.log(`Server listening on port ${PORT}`);
        console.log(`Enhanced monitoring with alerts enabled`);
        console.log(`Health endpoint: http://localhost:${PORT}/api/health`);
        console.log(`Performance metrics: http://localhost:${PORT}/api/performance`);
        console.log(`Alerts API: http://localhost:${PORT}/api/alerts`);
        
        // Schedule the first metric run *after* the initial discovery completes and server is listening
        scheduleNextMetric(); 
        
        // Watch .env file for changes
        setupEnvFileWatcher();
        
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
