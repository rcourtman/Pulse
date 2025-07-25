// Load environment variables from .env file
// Check for persistent config directory (Docker) or use project root
const fs = require('fs');
const path = require('path');

const configDir = path.join(__dirname, '../config');
const configEnvPath = path.join(configDir, '.env');
const projectEnvPath = path.join(__dirname, '..', '.env');

if (fs.existsSync(configEnvPath)) {
    require('dotenv').config({ path: configEnvPath });
} else {
    require('dotenv').config({ path: projectEnvPath });
}

// Import configuration constants
const { SERVER_DEFAULTS, TIMEOUTS, UPDATE_INTERVALS, PERFORMANCE_THRESHOLDS, RETRY_CONFIG } = require('./config/constants');

// Import the state manager FIRST
const stateManager = require('./state');

// Import security module
const { initializeSecurity, shutdownAudit } = require('./security');

// Import metrics history system
const metricsHistory = require('./metricsHistory');

// Import metrics persistence
const MetricsPersistence = require('./metricsPersistence');
const metricsPersistence = new MetricsPersistence();

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
const axios = require('axios');
const axiosRetry = require('axios-retry').default; // Import axios-retry

let chokidar;
try {
  chokidar = require('chokidar');
} catch (e) {
  console.warn('chokidar is not installed. Hot reload requires chokidar: npm install chokidar');
}

// --- API Client Initialization ---
const { initializeApiClients } = require('./apiClients');
let apiClients = {};   // Initialize as empty objects
let pbsApiClients = {};
// --- END API Client Initialization ---

// Create global diagnostic tool instance for error logging
let diagnosticTool = null;

// Configuration API
const ConfigApi = require('./configApi');
const configApi = new ConfigApi();


// --- Data Fetching (Imported) ---
const { fetchDiscoveryData, fetchMetricsData, fetchStoppedGuestUptime } = require('./dataFetcher');
// --- END Data Fetching ---

// Server configuration
const PORT = (() => {
    const envPort = process.env.PORT;
    if (!envPort) return SERVER_DEFAULTS.PORT;
    
    const parsed = parseInt(envPort, 10);
    if (isNaN(parsed)) {
        console.warn(`WARNING: Invalid PORT value "${envPort}" - using default port ${SERVER_DEFAULTS.PORT}`);
        return SERVER_DEFAULTS.PORT;
    }
    if (parsed < 1 || parsed > 65535) {
        console.warn(`WARNING: PORT ${parsed} is out of valid range (1-65535) - using default port ${SERVER_DEFAULTS.PORT}`);
        return SERVER_DEFAULTS.PORT;
    }
    return parsed;
})();

// --- Define Update Intervals (Configurable via Env Vars) ---
const METRIC_UPDATE_INTERVAL = UPDATE_INTERVALS.METRICS;
const DISCOVERY_UPDATE_INTERVAL = UPDATE_INTERVALS.DISCOVERY;

console.log(`INFO: Using Metric Update Interval: ${METRIC_UPDATE_INTERVAL}ms`);
console.log(`INFO: Using Discovery Update Interval: ${DISCOVERY_UPDATE_INTERVAL}ms`);

// Initialize enhanced state management
stateManager.init();

const { createServer } = require('./server');
const { app, server } = createServer();

// Mount alerts router
const alertsRouter = require('./routes/alerts');
app.use('/api/alerts', alertsRouter);


// State endpoint - returns current state data
app.get('/api/state', (req, res) => {
    try {
        const currentState = stateManager.getState();
        res.json(currentState);
    } catch (error) {
        console.error('[State API] Error getting state:', error);
        res.status(500).json({ error: 'Failed to get state' });
    }
});

// Version API endpoint
app.get('/api/version', async (req, res) => {
    try {
        const { getCurrentVersionInfo } = require('./versionUtils');
        
        // Get version info using centralized logic
        const versionInfo = getCurrentVersionInfo();
        const currentVersion = versionInfo.version;
        const gitBranch = versionInfo.gitBranch;
        const isDevelopment = versionInfo.isDevelopment;
        
        let latestVersion = currentVersion;
        let updateAvailable = false;
        let releaseUrl = null;
        
        const isDevelopmentBuild = currentVersion.includes('-dev.') || currentVersion.includes('-dirty');
        
        if (!isDevelopmentBuild) {
            try {
                const channelOverride = req.query.channel;
                
                // Try to check for updates with optional channel override
                const updateInfo = await updateManager.checkForUpdates(channelOverride);
                latestVersion = updateInfo.latestVersion || currentVersion;
                updateAvailable = updateInfo.updateAvailable || false;
                releaseUrl = updateInfo.releaseUrl;
            } catch (updateError) {
                // Log the error but continue with current version info
                console.error("[Version API] Error checking for updates:", updateError.message);
            }
        }
        
        res.json({ 
            version: currentVersion,
            latestVersion: latestVersion,
            updateAvailable: updateAvailable,
            gitBranch: gitBranch,
            isDevelopment: isDevelopment,
            releaseUrl: releaseUrl
        });
    } catch (error) {
         console.error("[Version API] Error in version endpoint:", error);
         // Still try to return current version if possible
         try {
             const packageJson = require('../package.json');
             res.json({ 
                 version: packageJson.version || 'N/A',
                 latestVersion: packageJson.version || 'N/A',
                 updateAvailable: false
             });
         } catch (fallbackError) {
             res.status(500).json({ error: "Could not retrieve version" });
         }
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

// Chart data API endpoint
app.get('/api/charts', async (req, res) => {
    try {
        const timeRangeMinutes = parseInt(req.query.range) || 60;
        
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
        
        const guestChartData = metricsHistory.getAllGuestChartData(guestInfoMap, timeRangeMinutes);
        const nodeChartData = metricsHistory.getAllNodeChartData(timeRangeMinutes);
        const stats = metricsHistory.getStats();
        
        res.json({
            data: guestChartData,
            nodeData: nodeChartData,
            stats: stats,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error("Error in /api/charts:", error);
        res.status(500).json({ error: error.message || "Failed to fetch chart data." });
    }
});

app.get('/api/storage-charts', async (req, res) => {
    try {
        const timeRangeMinutes = parseInt(req.query.range) || 60;
        
        const storageChartData = metricsHistory.getAllStorageChartData(timeRangeMinutes);
        const stats = metricsHistory.getStats();
        
        res.json({
            data: storageChartData,
            stats: stats,
            timestamp: Date.now()
        });
    } catch (error) {
        console.error("Error in /api/storage-charts:", error);
        res.status(500).json({ error: error.message || "Failed to fetch storage chart data." });
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

// Rate limiter status endpoint (admin only - should be protected in production)
app.get('/api/rate-limits', (req, res) => {
    try {
        // Get stats from all rate limiters used in server.js
        const stats = {
            timestamp: Date.now(),
            limiters: {},
            globalStats: {
                totalRequests: 0,
                blockedRequests: 0,
                allowedRequests: 0
            }
        };
        
        // Note: In production, you'd need to access the rate limiters from server.js
        // For now, return a placeholder response
        res.json({
            message: 'Rate limit stats endpoint ready',
            note: 'Stats collection requires rate limiter instances to be exposed',
            timestamp: Date.now()
        });
    } catch (error) {
        console.error('[Rate Limits] Error getting status:', error);
        res.status(500).json({ error: 'Failed to get rate limit status' });
    }
});

// --- Diagnostic Endpoint ---
app.get('/api/diagnostics', async (req, res) => {
    try {
        console.log('Running diagnostics...');
        
        // Use the persistent diagnosticTool instance if available
        let toolToUse = diagnosticTool;
        
        // If no persistent instance (shouldn't happen), create a temporary one
        if (!toolToUse) {
            const DiagnosticTool = require('./diagnostics');
            toolToUse = new DiagnosticTool(stateManager, metricsHistory, apiClients, pbsApiClients);
        }
        
        const report = await toolToUse.runDiagnostics();
        
        // The report already includes summary from generateReport
        res.json(report);
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

// Test email endpoint - MOVED TO routes/alerts.js
// Commented out to avoid duplicate route conflict
/*
app.post('/api/test-email', async (req, res) => {
    try {
        const { host, port, user, pass, from, to, secure } = req.body;
        
        // Use the unified sendTestEmail method with custom config
        const testResult = await stateManager.alertManager.sendTestEmail({
            host, port, user, pass, from, to, secure
        });
        
        if (testResult.success) {
            console.log(`[EMAIL TEST] Test email sent successfully to: ${to}`);
            res.json({
                success: true,
                message: 'Test email sent successfully!'
            });
        } else {
            console.error('[EMAIL TEST] Failed to send test email:', testResult.error);
            res.status(400).json({
                success: false,
                error: testResult.error || 'Failed to send test email'
            });
        }
        
    } catch (error) {
        console.error('[EMAIL TEST] Error sending test email:', error);
        res.status(500).json({
            success: false,
            error: 'Internal server error while sending test email'
        });
    }
});
*/

// Get webhook status and configuration
app.get('/api/webhook-status', (req, res) => {
    try {
        const alertManager = stateManager.alertManager;
        const webhookUrl = process.env.WEBHOOK_URL;
        const webhookEnabled = !!webhookUrl; // Webhook is enabled if URL exists
        
        // Get cooldown information
        const cooldownConfig = alertManager.webhookCooldownConfig;
        const activeCooldowns = [];
        
        for (const [key, info] of alertManager.webhookCooldowns) {
            if (info.cooldownUntil && info.cooldownUntil > Date.now()) {
                activeCooldowns.push({
                    key,
                    cooldownUntil: new Date(info.cooldownUntil).toISOString(),
                    remainingMinutes: Math.ceil((info.cooldownUntil - Date.now()) / 60000),
                    lastSent: info.lastSent ? new Date(info.lastSent).toISOString() : null
                });
            }
        }
        
        // Get recent webhook notifications
        const recentWebhooks = [];
        for (const [alertId, status] of alertManager.notificationStatus) {
            if (status.webhookSent) {
                recentWebhooks.push({
                    alertId,
                    sentAt: new Date(status.timestamp || Date.now()).toISOString(),
                    channels: status.channels
                });
            }
        }
        
        res.json({
            enabled: webhookEnabled,
            configured: !!webhookUrl,
            url: webhookUrl ? webhookUrl.replace(/\/[^\/]+$/, '/***') : null, // Hide token part
            cooldownConfig,
            activeCooldowns,
            recentWebhooks: recentWebhooks.slice(0, 10),
            totalWebhooksSent: recentWebhooks.length
        });
    } catch (error) {
        console.error('[WEBHOOK STATUS] Failed to get status:', error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

// Global error handler for unhandled API errors
app.use((err, req, res, next) => {
    console.error('Unhandled API error:', err);
    
    // Log error to diagnostic tool if available
    if (diagnosticTool) {
        diagnosticTool.logError(err, `API Error: ${req.method} ${req.url}`);
    }
    
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
app.use('/api/*splat', (req, res) => {
    res.status(404).json({
        success: false,
        error: 'API endpoint not found'
    });
});

// --- WebSocket Setup ---
const { initializeSocket } = require('./socket');
const io = initializeSocket(server);

// --- Global State Variables ---
// These will hold the latest fetched data
let isDiscoveryRunning = false; // Prevent concurrent discovery runs
let isMetricsRunning = false;   // Prevent concurrent metric runs
let discoveryTimeoutId = null;
let metricTimeoutId = null;
// --- End Global State ---


// --- Update Cycle Logic --- 
// Uses imported fetch functions and updates global state
async function runDiscoveryCycle() {
  if (isDiscoveryRunning) {
    console.warn('[Discovery Cycle] Previous cycle still running, skipping this run');
    return;
  }
  isDiscoveryRunning = true;
  
  const startTime = Date.now();
  const MAX_DISCOVERY_TIME = PERFORMANCE_THRESHOLDS.MAX_DISCOVERY_TIME;
  
  // Set a timeout to force completion if cycle takes too long
  const timeoutHandle = setTimeout(() => {
    console.error('[Discovery Cycle] Cycle exceeded maximum time, forcing completion');
    isDiscoveryRunning = false;
  }, MAX_DISCOVERY_TIME);
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
    
    // Debug: Log what we got from discovery
    console.log(`[Discovery Cycle] Discovery data received:`, {
        nodes: discoveryData.nodes ? discoveryData.nodes.length : 0,
        vms: discoveryData.vms ? discoveryData.vms.length : 0,
        containers: discoveryData.containers ? discoveryData.containers.length : 0,
        pbs: discoveryData.pbs ? discoveryData.pbs.length : 0,
        firstNode: discoveryData.nodes && discoveryData.nodes.length > 0 ? discoveryData.nodes[0] : null
    });
    
    const duration = Date.now() - startTime;
    
    // Update state using the enhanced state manager
    stateManager.updateDiscoveryData(discoveryData, duration, errors);
    // No need to store in global vars anymore

    // Add node metrics to history
    if (discoveryData.nodes && discoveryData.nodes.length > 0) {
        discoveryData.nodes.forEach(node => {
            if (node && node.node) {
                const nodeId = `node-${node.node}`;
                metricsHistory.addNodeMetricData(nodeId, node);
            }
        });
    }

    // Add storage metrics to history
    if (discoveryData.nodes && discoveryData.nodes.length > 0) {
        discoveryData.nodes.forEach(node => {
            if (node && node.node && node.storage && Array.isArray(node.storage)) {
                node.storage.forEach(storage => {
                    if (storage && storage.storage) {
                        const storageId = `${node.node}-${storage.storage}`;
                        metricsHistory.addStorageMetricData(storageId, {
                            ...storage,
                            node: node.node
                        });
                    }
                });
            }
        });
    }

    // ... (logging summary) ...
    const updatedState = stateManager.getState(); // Get the fully updated state
    console.log(`[Discovery Cycle] Updated state. Nodes: ${updatedState.nodes.length}, VMs: ${updatedState.vms.length}, CTs: ${updatedState.containers.length}, PBS: ${updatedState.pbs.length}`);

    if (io.engine.clientsCount > 0) {
        try {
            const pveBackups = updatedState.pveBackups || {};
            console.log(`[Discovery Broadcast] Broadcasting state with PVE backups: ${(pveBackups.backupTasks || []).length} tasks, ${(pveBackups.storageBackups || []).length} storage, ${(pveBackups.guestSnapshots || []).length} snapshots`);
            // Test serialization first to catch circular reference errors
            JSON.stringify(updatedState);
            io.emit('rawData', updatedState);
        } catch (serializationError) {
            console.error('[Discovery Broadcast] Error serializing state data:', serializationError.message);
            // Don't emit anything if serialization fails
        }
    }
  } catch (error) {
      console.error(`[Discovery Cycle] Error during execution: ${error.message}`, error.stack);
      errors.push({ type: 'discovery', message: error.message, endpointId: 'general' });
      
      // Log error to diagnostic tool if available
      if (diagnosticTool) {
        diagnosticTool.logError(error, 'Discovery Cycle');
      }
      
      const duration = Date.now() - startTime;
      stateManager.updateDiscoveryData({ nodes: [], vms: [], containers: [], pbs: [] }, duration, errors);
  } finally {
      clearTimeout(timeoutHandle);
      isDiscoveryRunning = false;
      scheduleNextDiscovery();
  }
}

async function runMetricCycle() {
  if (isMetricsRunning) {
    console.warn('[Metrics Cycle] Previous cycle still running, skipping this run');
    return;
  }
  // Disabled: Allow metrics to run even without clients for alerts to work
  // if (io.engine.clientsCount === 0) {
  //   scheduleNextMetric(); 
  //   return;
  // }
  isMetricsRunning = true;
  
  const startTime = Date.now();
  const MAX_METRICS_TIME = PERFORMANCE_THRESHOLDS.MAX_METRICS_TIME;
  
  // Set a timeout to force completion if cycle takes too long
  const timeoutHandle = setTimeout(() => {
    console.error('[Metrics Cycle] Cycle exceeded maximum time, forcing completion');
    isMetricsRunning = false;
  }, MAX_METRICS_TIME);
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
           
           // Fetch uptime for stopped VMs to ensure accurate values
           const stoppedVms = currentVms.filter(vm => vm.status !== 'running');
           const stoppedContainers = currentContainers.filter(ct => ct.status !== 'running');
           
           let allMetrics = fetchedMetrics;
           if (stoppedVms.length > 0 || stoppedContainers.length > 0) {
               const stoppedMetrics = await fetchStoppedGuestUptime(stoppedVms, stoppedContainers, currentApiClients);
               // Combine running and stopped metrics
               allMetrics = [...fetchedMetrics, ...stoppedMetrics];
               // Update state with all metrics
               stateManager.updateMetricsData(allMetrics, duration, errors);
           }
           
           // Add metrics to history for charts
           allMetrics.forEach(metricData => {
               if (metricData && metricData.current) {
                   const guestId = `${metricData.endpointId}-${metricData.node}-${metricData.id}`;
                   metricsHistory.addMetricData(guestId, metricData.current);
                   
               }
           });
           
           // Also update node metrics during metrics cycle for consistent data collection frequency
           // Build a map of unique nodes from running guests
           const nodesByEndpoint = new Map();
           [...runningVms, ...runningContainers].forEach(guest => {
               if (!guest.endpointId || !guest.node) return;
               
               if (!nodesByEndpoint.has(guest.endpointId)) {
                   nodesByEndpoint.set(guest.endpointId, new Set());
               }
               nodesByEndpoint.get(guest.endpointId).add(guest.node);
           });
           
           // Node status is already fetched during discovery cycle (every 30s)
           // Use existing node data from state instead of fetching again
           const currentNodes = stateManager.getState().nodes || [];
           
           // Add node metrics to history from existing data
           currentNodes.forEach(node => {
               if (node && node.status) {
                   const nodeId = `node-${node.node}`;
                   // Use the existing node status data that was fetched during discovery
                   metricsHistory.addNodeMetricData(nodeId, node.status);
               }
           });
           
           // Emit only metrics updates if needed, or rely on full rawData updates?
           // Consider emitting a smaller 'metricsUpdate' event if performance is key
        }

        try {
            const currentState = stateManager.getState();
            // Test serialization first to catch circular reference errors
            JSON.stringify(currentState);
            io.emit('rawData', currentState);
        } catch (serializationError) {
            console.error('[Metrics Broadcast] Error serializing state data:', serializationError.message);
            // Don't emit anything if serialization fails
        }
    } else {
        const currentMetrics = stateManager.getState().metrics;
        if (currentMetrics.length > 0) {
           console.log('[Metrics Cycle] No running guests found, clearing metrics.');
           stateManager.clearMetricsData(); // Clear metrics
           // Emit state update with cleared metrics only if clients are connected
           if (io.engine.clientsCount > 0) {
               try {
                   const currentState = stateManager.getState();
                   // Test serialization first to catch circular reference errors
                   JSON.stringify(currentState);
                   io.emit('rawData', currentState);
               } catch (serializationError) {
                   console.error('[Metrics Clear Broadcast] Error serializing state data:', serializationError.message);
                   // Don't emit anything if serialization fails
               }
           }
        }
    }
  } catch (error) {
      console.error(`[Metrics Cycle] Error during execution: ${error.message}`, error.stack);
      errors.push({ type: 'metrics', message: error.message, endpointId: 'general' });
      
      // Log error to diagnostic tool if available
      if (diagnosticTool) {
        diagnosticTool.logError(error, 'Metrics Cycle');
      }
      
      const duration = Date.now() - startTime;
      stateManager.updateMetricsData([], duration, errors);
  } finally {
      clearTimeout(timeoutHandle);
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
    }, TIMEOUTS.GRACEFUL_SHUTDOWN);
    
    // Clear timers
    if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
    if (metricTimeoutId) clearTimeout(metricTimeoutId);
    
    // Clean up file watchers
    // envWatcher disabled - no need to close
    // if (envWatcher) {
    //     envWatcher.close();
    //     envWatcher = null;
    // }
    if (devWatcher) {
        devWatcher.close();
        devWatcher = null;
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
let devWatcher = null;
let reloadDebounceTimer = null;
let lastReloadTime = 0;
global.lastReloadTime = 0;  // Make it globally accessible
global.isUIUpdatingEnv = false;  // Flag to track UI updates to .env

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
            // Skip reload if UI is updating the file
            if (global.isUIUpdatingEnv) {
                console.log('.env file changed by UI, skipping hot reload');
                return;
            }
            
            // Debounce the reload to avoid multiple reloads for rapid changes
            clearTimeout(reloadDebounceTimer);
            reloadDebounceTimer = setTimeout(async () => {
                const now = Date.now();
                if (now - global.lastReloadTime < 2000) {
                    console.log('.env file changed but skipping reload (too recent)');
                    return;
                }
                
                console.log('.env file changed externally, triggering service restart...');
                global.lastReloadTime = now;
                
                try {
                    // Notify connected clients about pending restart
                    io.emit('serviceRestarting', { 
                        message: 'Configuration changed, service is restarting...',
                        timestamp: Date.now()
                    });
                    
                    // Use systemctl to restart the service
                    const { exec } = require('child_process');
                    exec('systemctl restart pulse', (error, stdout, stderr) => {
                        if (error) {
                            console.error('Failed to restart service:', error);
                            // Fall back to configuration reload
                            configApi.reloadConfiguration().catch(err => {
                                console.error('Fallback reload also failed:', err);
                            });
                        } else {
                            console.log('Service restart triggered successfully');
                        }
                    });
                } catch (error) {
                    console.error('Failed to trigger restart:', error);
                    
                    // Fall back to configuration reload
                    try {
                        await configApi.reloadConfiguration();
                        console.log('Fell back to configuration reload');
                    } catch (reloadError) {
                        console.error('Fallback reload also failed:', reloadError);
                    }
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
    // Initialize security
    try {
        await initializeSecurity();
    } catch (error) {
        console.error('[Security] Failed to initialize security:', error);
        // Continue startup even if security fails in open mode
    }
    
    // Load persisted metrics before starting
    try {
        const loadResult = await metricsPersistence.loadSnapshot(metricsHistory);
        if (loadResult) {
            // Calculate more detailed age information
            const ageMinutes = loadResult.ageHours * 60;
            let ageString;
            
            if (ageMinutes < 1) {
                ageString = `${Math.round(ageMinutes * 60)} seconds ago`;
            } else if (ageMinutes < 60) {
                ageString = `${ageMinutes.toFixed(1)} minutes ago`;
            } else {
                ageString = `${loadResult.ageHours.toFixed(1)} hours ago`;
            }
            
            console.log(`[MetricsPersistence] Restored ${loadResult.totalPoints} data points from ${ageString}`);
            // Also log to the health endpoint for visibility
            global.metricsRestored = {
                points: loadResult.totalPoints,
                ageHours: loadResult.ageHours,
                ageMinutes: ageMinutes,
                ageString: ageString,
                timestamp: new Date().toISOString()
            };
        } else {
            console.log('[MetricsPersistence] No snapshot found or snapshot too old');
        }
    } catch (error) {
        console.error('[MetricsPersistence] Failed to load persisted metrics:', error.message);
    }

    // Set up intelligent snapshot saving
    // More frequent saves initially, then back off
    const snapshotSchedule = [
        { delay: 30 * 1000, interval: 30 * 1000 },      // First 2 min: every 30s
        { delay: 2 * 60 * 1000, interval: 60 * 1000 },  // 2-5 min: every 60s
        { delay: 5 * 60 * 1000, interval: 2 * 60 * 1000 } // After 5 min: every 2 min
    ];
    
    let currentScheduleIndex = 0;
    let snapshotInterval;
    
    let scheduleTransitionTimeout = null;
    
    const scheduleSnapshot = () => {
        // Clear any pending transition timeout to prevent race conditions
        if (scheduleTransitionTimeout) {
            clearTimeout(scheduleTransitionTimeout);
            scheduleTransitionTimeout = null;
        }
        
        if (currentScheduleIndex < snapshotSchedule.length) {
            const schedule = snapshotSchedule[currentScheduleIndex];
            
            // Clear existing interval if any
            if (snapshotInterval) {
                clearInterval(snapshotInterval);
                snapshotInterval = null;
            }
            
            // Save snapshot function with error tracking and exponential backoff
            const saveSnapshotWithTracking = async () => {
                // Check if we should skip due to exponential backoff
                if (global.snapshotErrorCount > 0) {
                    const backoffDelay = Math.min(
                        schedule.interval * Math.pow(2, global.snapshotErrorCount - 1),
                        RETRY_CONFIG.MAX_BACKOFF
                    );
                    const timeSinceLastError = Date.now() - (global.lastSnapshotError || 0);
                    
                    if (timeSinceLastError < backoffDelay) {
                        // Still in backoff period, skip this save
                        return;
                    }
                }
                
                try {
                    const stats = await metricsPersistence.saveSnapshot(metricsHistory);
                    // Store last snapshot info globally for health endpoint
                    global.lastSnapshotStats = {
                        ...stats,
                        timestamp: Date.now()
                    };
                    // Reset error count on success
                    global.snapshotErrorCount = 0;
                    global.lastSnapshotError = null;
                } catch (error) {
                    console.error(`[MetricsPersistence] Failed to save snapshot (attempt ${global.snapshotErrorCount + 1}):`, error.message);
                    // Track errors for exponential backoff
                    global.snapshotErrorCount = (global.snapshotErrorCount || 0) + 1;
                    global.lastSnapshotError = Date.now();
                    
                    // Log warning if errors persist
                    if (global.snapshotErrorCount >= 5) {
                        console.error('[MetricsPersistence] Snapshot saves failing repeatedly. Check disk space and permissions.');
                    }
                }
            };
            
            // Set new interval
            snapshotInterval = setInterval(saveSnapshotWithTracking, schedule.interval);
            
            // Also save immediately when transitioning to a new schedule
            if (currentScheduleIndex > 0) {
                saveSnapshotWithTracking();
            }
            
            // Schedule next transition
            currentScheduleIndex++;
            if (currentScheduleIndex < snapshotSchedule.length) {
                const nextSchedule = snapshotSchedule[currentScheduleIndex];
                const transitionDelay = nextSchedule.delay - schedule.delay;
                scheduleTransitionTimeout = setTimeout(scheduleSnapshot, transitionDelay);
            }
        }
    };
    
    // Start snapshot scheduling
    scheduleTransitionTimeout = setTimeout(scheduleSnapshot, snapshotSchedule[0].delay);

    // Save snapshot on graceful shutdown
    const gracefulShutdown = async (signal) => {
        console.log(`\nReceived ${signal}, saving metrics snapshot before exit...`);
        try {
            await metricsPersistence.saveSnapshot(metricsHistory);
            console.log('[MetricsPersistence] Final snapshot saved');
        } catch (error) {
            console.error('[MetricsPersistence] Failed to save final snapshot:', error.message);
        }
        process.exit(0);
    };

    process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
    process.on('SIGINT', () => gracefulShutdown('SIGINT'));

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
            
            // Initialize diagnostic tool now that we have API clients
            diagnosticTool = new DiagnosticTool(stateManager, metricsHistory, apiClients, pbsApiClients);
            
            // Make diagnosticTool globally available for error logging
            global.diagnosticTool = diagnosticTool;
            
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
        
        // Initialize diagnostic tool even in setup mode
        diagnosticTool = new DiagnosticTool(stateManager, metricsHistory, apiClients, pbsApiClients);
        
        // Make diagnosticTool globally available for error logging
        global.diagnosticTool = diagnosticTool;
    } 

    server.listen(PORT, '0.0.0.0', () => {
        console.log(`Server listening on port ${PORT}`);
        console.log(`Enhanced monitoring with alerts enabled`);
        console.log(`Health endpoint: http://localhost:${PORT}/api/health`);
        console.log(`Performance metrics: http://localhost:${PORT}/api/performance`);
        console.log(`Alerts API: http://localhost:${PORT}/api/alerts`);
        
        // Schedule the first metric run *after* the initial discovery completes and server is listening
        scheduleNextMetric(); 
        
        // DISABLED: .env file watcher causes issues with user experience
        // - Toast notifications disappear when the service restarts
        // - WebSocket connections are reset, losing real-time updates
        // - The app already has reloadConfiguration() which applies settings without restart
        // setupEnvFileWatcher();
        
        // Setup hot reload in development mode
        if (process.env.NODE_ENV === 'development' && chokidar) {
          console.log('[Hot Reload] Development mode detected - initializing hot reload...');
          const watchPaths = [
            path.join(__dirname, '../src/public'),    // Frontend files
            path.join(__dirname, './'),                // Server files
            path.join(__dirname, '../data'),           // Config files
            path.join(__dirname, '../package.json'),  // Package.json for auto-restart on version updates
          ];
          
          devWatcher = chokidar.watch(watchPaths, { 
            ignored: [
              /(^|[\\\/])\../, // ignore dotfiles
              /node_modules/,  // ignore node_modules
              /\.env$/,        // ignore .env file to prevent reload loops
              /\.log$/,        // ignore log files
              /\.tmp$/,        // ignore temp files
              /\.test\.js$/,   // ignore test files
              /coverage/,      // ignore coverage directory
              /temp/,          // ignore temp directory
              /alert-rules\.json$/, // ignore alert rules runtime data
              /custom-thresholds\.json$/, // ignore custom thresholds runtime data
              /active-alerts\.json$/, // ignore active alerts runtime data
              /alert-history\.json$/, // ignore alert history runtime data
              /notification-history\.json$/, // ignore notification history runtime data
              /metrics-snapshot\.json\.gz$/, // ignore metrics persistence snapshots
              /\.metrics-snapshot\.tmp\.gz$/, // ignore metrics persistence temp files
              /\.locks\// // ignore lock files directory
            ],
            persistent: true,
            ignoreInitial: true // Don't trigger on initial scan
          });
          
          devWatcher.on('change', (path) => {
            console.log(`[Hot Reload] File changed: ${path}`);
            io.emit('hotReload'); // Notify clients to reload
          });
          
          devWatcher.on('add', () => {
            io.emit('hotReload'); // Notify clients to reload
          });
          
          devWatcher.on('unlink', () => {
            io.emit('hotReload'); // Notify clients to reload
          });
          
          devWatcher.on('error', error => console.error(`[Hot Reload] Watcher error: ${error}`));
        }
    });
}

startServer();

// Graceful shutdown handlers
process.on('SIGTERM', async () => {
    console.log('[Server] SIGTERM received, shutting down gracefully...');
    await shutdownAudit();
    process.exit(0);
});

process.on('SIGINT', async () => {
    console.log('[Server] SIGINT received, shutting down gracefully...');
    await shutdownAudit();
    process.exit(0);
});

