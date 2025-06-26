// Load environment variables from .env file
// Check for persistent config directory (Docker) or use project root
const fs = require('fs');
const path = require('path');

const configDir = path.join(__dirname, '../config');
const configEnvPath = path.join(configDir, '.env');
const projectEnvPath = path.join(__dirname, '.env');

if (fs.existsSync(configEnvPath)) {
    require('dotenv').config({ path: configEnvPath });
} else {
    require('dotenv').config({ path: projectEnvPath });
}

// Import the state manager FIRST
const stateManager = require('./state');

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

// Hot reload dependencies (always try to load for development convenience)
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
// Note: Client initialization is now async and happens in startServer()
// --- END API Client Initialization ---

// Configuration API
const ConfigApi = require('./configApi');
const configApi = new ConfigApi();


// --- Data Fetching (Imported) ---
const { fetchDiscoveryData, fetchMetricsData } = require('./dataFetcher');
// --- END Data Fetching ---

// Server configuration
const PORT = parseInt(process.env.PORT, 10) || 7655;

// --- Define Update Intervals (Configurable via Env Vars) ---
const METRIC_UPDATE_INTERVAL = parseInt(process.env.PULSE_METRIC_INTERVAL_MS, 10) || 2000; // Default: 2 seconds
const DISCOVERY_UPDATE_INTERVAL = parseInt(process.env.PULSE_DISCOVERY_INTERVAL_MS, 10) || 30000; // Default: 30 seconds

console.log(`INFO: Using Metric Update Interval: ${METRIC_UPDATE_INTERVAL}ms`);
console.log(`INFO: Using Discovery Update Interval: ${DISCOVERY_UPDATE_INTERVAL}ms`);

// Initialize enhanced state management
stateManager.init();

const { createServer } = require('./server');
const { app, server } = createServer();

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
        
        // Check if this is a development build (ahead of all releases)
        const isDevelopmentBuild = currentVersion.includes('-dev.') || currentVersion.includes('-dirty');
        
        if (!isDevelopmentBuild) {
            try {
                // Check for channel override from query parameter (for preview functionality)
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
        // Get time range from query parameter (default to 60 minutes)
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

// Test email endpoint
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

// Test webhook endpoint
app.post('/api/test-webhook', async (req, res) => {
    try {
        const { url, enabled } = req.body;
        
        if (!url) {
            return res.status(400).json({
                success: false,
                error: 'Webhook URL is required for testing'
            });
        }
        
        // Create test webhook payload
        const axios = require('axios');
        
        // Detect webhook type based on URL
        const isDiscord = url.includes('discord.com/api/webhooks') || url.includes('discordapp.com/api/webhooks');
        const isSlack = url.includes('slack.com/') || url.includes('hooks.slack.com');
        
        let testPayload;
        
        if (isDiscord) {
            // Discord-specific format
            testPayload = {
                embeds: [{
                    title: '🧪 Webhook Test Alert',
                    description: 'This is a test alert to verify webhook configuration',
                    color: 3447003, // Blue
                    fields: [
                        {
                            name: 'VM/LXC',
                            value: 'Test-VM (qemu 999)',
                            inline: true
                        },
                        {
                            name: 'Node',
                            value: 'test-node',
                            inline: true
                        },
                        {
                            name: 'Status',
                            value: 'running',
                            inline: true
                        },
                        {
                            name: 'Metric',
                            value: 'TEST',
                            inline: true
                        },
                        {
                            name: 'Current Value',
                            value: '75%',
                            inline: true
                        },
                        {
                            name: 'Threshold',
                            value: '80%',
                            inline: true
                        }
                    ],
                    footer: {
                        text: 'Pulse Monitoring System - Test Message'
                    },
                    timestamp: new Date().toISOString()
                }]
            };
        } else if (isSlack) {
            // Slack-specific format
            testPayload = {
                text: '🧪 *Webhook Test Alert*',
                attachments: [{
                    color: 'good',
                    fields: [
                        {
                            title: 'VM/LXC',
                            value: 'Test-VM (qemu 999)',
                            short: true
                        },
                        {
                            title: 'Node',
                            value: 'test-node',
                            short: true
                        },
                        {
                            title: 'Status',
                            value: 'Webhook configuration test successful!',
                            short: false
                        }
                    ],
                    footer: 'Pulse Monitoring - Test',
                    ts: Math.floor(Date.now() / 1000)
                }]
            };
        } else {
            // Generic webhook format with all fields (backward compatibility)
            testPayload = {
                timestamp: new Date().toISOString(),
                alert: {
                    id: 'test-alert-' + Date.now(),
                    rule: {
                        name: 'Webhook Test Alert',
                        description: 'This is a test alert to verify webhook configuration',
                        severity: 'info',
                        metric: 'test'
                    },
                    guest: {
                        name: 'Test-VM',
                        id: '999',
                        type: 'qemu',
                        node: 'test-node',
                        status: 'running'
                    },
                    value: 75,
                    threshold: 80,
                    emoji: '🧪'
                },
                // Include both formats for generic webhooks
                embeds: [{
                    title: '🧪 Webhook Test Alert',
                    description: 'This is a test alert to verify webhook configuration',
                    color: 3447003,
                    fields: [
                        {
                            name: 'VM/LXC',
                            value: 'Test-VM (qemu 999)',
                            inline: true
                        },
                        {
                            name: 'Node',
                            value: 'test-node',
                            inline: true
                        },
                        {
                            name: 'Status',
                            value: 'running',
                            inline: true
                        }
                    ],
                    footer: {
                        text: 'Pulse Monitoring System - Test Message'
                    },
                    timestamp: new Date().toISOString()
                }],
                text: '🧪 *Webhook Test Alert*',
                attachments: [{
                    color: 'good',
                    fields: [
                        {
                            title: 'VM/LXC',
                            value: 'Test-VM (qemu 999)',
                            short: true
                        },
                        {
                            title: 'Status',
                            value: 'Webhook configuration test successful!',
                            short: false
                        }
                    ],
                    footer: 'Pulse Monitoring - Test',
                    ts: Math.floor(Date.now() / 1000)
                }]
            };
        }
        
        // Send test webhook
        const response = await axios.post(url, testPayload, {
            headers: {
                'Content-Type': 'application/json',
                'User-Agent': 'Pulse-Monitoring/1.0'
            },
            timeout: 10000, // 10 second timeout
            maxRedirects: 3
        });
        
        console.log(`[WEBHOOK TEST] Test webhook sent successfully to: ${url} (${response.status})`);
        res.json({
            success: true,
            message: 'Test webhook sent successfully!',
            status: response.status
        });
        
    } catch (error) {
        console.error('[WEBHOOK TEST] Failed to send test webhook:', error);
        
        let errorMessage = 'Failed to send test webhook';
        if (error.response) {
            errorMessage = `Webhook failed: ${error.response.status} ${error.response.statusText}`;
        } else if (error.request) {
            errorMessage = `Webhook failed: No response from ${url}`;
        } else {
            errorMessage = `Webhook failed: ${error.message}`;
        }
        
        res.status(400).json({
            success: false,
            error: errorMessage
        });
    }
});


// Get webhook status and configuration
app.get('/api/webhook-status', (req, res) => {
    try {
        const alertManager = stateManager.alertManager;
        const webhookEnabled = process.env.GLOBAL_WEBHOOK_ENABLED === 'true';
        const webhookUrl = process.env.WEBHOOK_URL;
        
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


// --- Update Cycle Logic --- 
// Uses imported fetch functions and updates global state
async function runDiscoveryCycle() {
  if (isDiscoveryRunning) {
    console.warn('[Discovery Cycle] Previous cycle still running, skipping this run');
    return;
  }
  isDiscoveryRunning = true;
  
  const startTime = Date.now();
  const MAX_DISCOVERY_TIME = 120000; // 2 minutes max
  
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

    // ... (logging summary) ...
    const updatedState = stateManager.getState(); // Get the fully updated state
    console.log(`[Discovery Cycle] Updated state. Nodes: ${updatedState.nodes.length}, VMs: ${updatedState.vms.length}, CTs: ${updatedState.containers.length}, PBS: ${updatedState.pbs.length}`);

    // Emit combined data using updated state manager state (which includes the flag)
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
  if (io.engine.clientsCount === 0) {
    scheduleNextMetric(); 
    return;
  }
  isMetricsRunning = true;
  
  const startTime = Date.now();
  const MAX_METRICS_TIME = 30000; // 30 seconds max
  
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
           
           // Add metrics to history for charts
           fetchedMetrics.forEach(metricData => {
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
           
           // Fetch fresh node status for each unique node that has running guests
           const nodeStatusPromises = [];
           
           for (const [endpointId, nodes] of nodesByEndpoint) {
               if (!currentApiClients[endpointId]) continue;
               const { client: apiClientInstance } = currentApiClients[endpointId];
               
               for (const nodeName of nodes) {
                   nodeStatusPromises.push(
                       apiClientInstance.get(`/nodes/${nodeName}/status`)
                           .then(response => ({
                               endpointId,
                               node: nodeName,
                               data: response.data.data
                           }))
                           .catch(err => {
                               console.error(`[Metrics] Failed to fetch node status for ${nodeName}:`, err.message);
                               return null;
                           })
                   );
               }
           }
           
           // Fetch all node statuses in parallel
           const nodeStatuses = await Promise.all(nodeStatusPromises);
           
           // Add fresh node metrics to history
           nodeStatuses.forEach(nodeStatus => {
               if (nodeStatus && nodeStatus.data) {
                   const nodeId = `node-${nodeStatus.node}`;
                   metricsHistory.addNodeMetricData(nodeId, nodeStatus.data);
               }
           });
           
           // Emit only metrics updates if needed, or rely on full rawData updates?
           // Consider emitting a smaller 'metricsUpdate' event if performance is key
           // io.emit('metricsUpdate', stateManager.getState().metrics);
        }

        // Emit rawData with updated global state (which includes metrics, alerts, and placeholder flag)
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
           // (Avoid unnecessary emits if no one is listening and nothing changed except clearing metrics)
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
    }, 5000);
    
    // Clear timers
    if (discoveryTimeoutId) clearTimeout(discoveryTimeoutId);
    if (metricTimeoutId) clearTimeout(metricTimeoutId);
    
    // Clean up file watchers
    if (envWatcher) {
        envWatcher.close();
        envWatcher = null;
    }
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
                        300000 // Max 5 minutes backoff
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

    server.listen(PORT, '0.0.0.0', () => {
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
              /\.log$/,        // ignore log files
              /\.tmp$/,        // ignore temp files
              /\.test\.js$/,   // ignore test files
              /coverage/,      // ignore coverage directory
              /temp/,          // ignore temp directory
              /alert-rules\.json$/, // ignore alert rules runtime data
              /custom-thresholds\.json$/, // ignore custom thresholds runtime data
              /active-alerts\.json$/, // ignore active alerts runtime data
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

