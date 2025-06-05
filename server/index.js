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
        // Check if we're in test mode
        if (process.env.UPDATE_TEST_MODE === 'true') {
            const testVersion = process.env.UPDATE_TEST_VERSION || '99.99.99';
            const currentVersion = updateManager.currentVersion;
            
            // Create mock update data
            const mockUpdateInfo = {
                currentVersion: currentVersion,
                latestVersion: testVersion,
                updateAvailable: true,
                isDocker: updateManager.isDockerEnvironment(),
                releaseNotes: 'Test release for update mechanism testing\n\n- Testing download functionality\n- Testing backup process\n- Testing installation process',
                releaseUrl: 'https://github.com/rcourtman/Pulse/releases/test',
                publishedAt: new Date().toISOString(),
                assets: [{
                    name: 'pulse-v' + testVersion + '.tar.gz',
                    size: 1024000,
                    downloadUrl: 'http://localhost:3000/api/test/mock-update.tar.gz'
                }]
            };
            
            console.log('[UpdateManager] Test mode enabled, returning mock update info');
            return res.json(mockUpdateInfo);
        }
        
        // Allow override of update channel via query parameter for preview
        const channelOverride = req.query.channel;
        const updateInfo = await updateManager.checkForUpdates(channelOverride);
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

                // Apply update (pass download URL for version extraction)
                await updateManager.applyUpdate(updateFile, (progress) => {
                    io.emit('updateProgress', progress);
                }, downloadUrl);

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

// Mock update tarball endpoint for testing (not actually used in test mode)
app.get('/api/test/mock-update.tar.gz', (req, res) => {
    if (process.env.UPDATE_TEST_MODE !== 'true') {
        return res.status(404).json({ error: 'Test mode not enabled' });
    }
    
    // This endpoint exists just to make the URL valid
    // The actual download is handled differently in test mode
    res.status(200).json({ 
        message: 'Test mode active - download handled internally',
        testMode: true 
    });
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

// Enhanced alerts/rules endpoints to handle compound threshold rules
app.get('/api/alerts/compound-rules', (req, res) => {
    try {
        const allRules = stateManager.alertManager.getRules();
        const compoundRules = allRules.filter(rule => rule.type === 'compound_threshold');
        res.json({ success: true, rules: compoundRules });
    } catch (error) {
        console.error("Error fetching compound threshold rules:", error);
        res.status(500).json({ error: "Failed to fetch compound threshold rules" });
    }
});

// Debug endpoint to manually reload alert rules
app.post('/api/alerts/rules/reload', async (req, res) => {
    try {
        console.log('[DEBUG] Manually reloading alert rules...');
        await stateManager.alertManager.loadAlertRules();
        const allRules = stateManager.alertManager.getRules();
        console.log(`[DEBUG] Total rules after reload: ${allRules.length}`);
        res.json({ success: true, message: "Alert rules reloaded", rulesCount: allRules.length });
    } catch (error) {
        console.error("Error reloading alert rules:", error);
        res.status(500).json({ error: "Failed to reload alert rules" });
    }
});

// Endpoint to trigger immediate alert evaluation
app.post('/api/alerts/evaluate', async (req, res) => {
    try {
        console.log('[AlertManager] Triggering immediate alert evaluation...');
        stateManager.alertManager.evaluateCurrentState();
        res.json({ success: true, message: "Alert evaluation triggered" });
    } catch (error) {
        console.error("Error triggering alert evaluation:", error);
        res.status(500).json({ error: "Failed to trigger alert evaluation" });
    }
});

// Simple endpoint to get just the alert enabled/disabled status
app.get('/api/alerts/status', (req, res) => {
    try {
        const { loadConfiguration } = require('./configLoader');
        const { endpoints, pbsConfigs, isConfigPlaceholder } = loadConfiguration();
        
        // Read the environment variables directly
        const alertStatus = {
            cpu: process.env.ALERT_CPU_ENABLED !== 'false',
            memory: process.env.ALERT_MEMORY_ENABLED !== 'false', 
            disk: process.env.ALERT_DISK_ENABLED !== 'false',
            down: process.env.ALERT_DOWN_ENABLED === 'true'
        };
        
        res.json({ success: true, alerts: alertStatus });
    } catch (error) {
        console.error("Error getting alert status:", error);
        res.status(500).json({ error: "Failed to get alert status" });
    }
});


// Version API endpoint
app.get('/api/version', async (req, res) => {
    try {
        const packageJson = require('../package.json');
        const currentVersion = packageJson.version || 'N/A';
        
        // Use UpdateManager to check for updates respecting user's channel preference
        const updateInfo = await updateManager.checkForUpdates();
        
        res.json({ 
            version: currentVersion,
            latestVersion: updateInfo.latestVersion,
            updateAvailable: updateInfo.hasUpdate
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

// Test email endpoint
app.post('/api/test-email', async (req, res) => {
    try {
        const { host, port, user, pass, from, to, secure } = req.body;
        
        if (!host || !port || !user || !pass || !from || !to) {
            return res.status(400).json({
                success: false,
                error: 'All email fields are required for testing'
            });
        }
        
        // Create a temporary transporter for testing
        const nodemailer = require('nodemailer');
        const testTransporter = nodemailer.createTransport({
            host: host,
            port: parseInt(port),
            secure: secure === true, // true for 465, false for other ports
            requireTLS: true, // Force TLS encryption
            auth: {
                user: user,
                pass: pass
            },
            tls: {
                // Do not fail on invalid certs for testing
                rejectUnauthorized: false
            }
        });
        
        // Send test email
        const testMailOptions = {
            from: from,
            to: to,
            subject: '🧪 Pulse Email Test - Configuration Successful',
            text: `
This is a test email from your Pulse monitoring system.

If you received this email, your SMTP configuration is working correctly!

Configuration used:
- SMTP Host: ${host}
- SMTP Port: ${port}
- Secure: ${secure ? 'Yes' : 'No'}
- From: ${from}
- To: ${to}

You will now receive alert notifications when VMs/LXCs exceed their configured thresholds.

Best regards,
Pulse Monitoring System
            `,
            html: `
                <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
                    <div style="background: #059669; color: white; padding: 20px; border-radius: 8px 8px 0 0;">
                        <h1 style="margin: 0; font-size: 24px;">🧪 Pulse Email Test</h1>
                        <p style="margin: 5px 0 0 0; opacity: 0.9;">Configuration Successful!</p>
                    </div>
                    
                    <div style="background: #f0fdf4; padding: 20px; border-left: 4px solid #059669;">
                        <p style="margin: 0 0 15px 0; color: #065f46;">
                            <strong>Congratulations!</strong> If you received this email, your SMTP configuration is working correctly.
                        </p>
                        
                        <h3 style="color: #065f46; margin: 15px 0 10px 0;">Configuration Details:</h3>
                        <ul style="color: #047857; margin: 0; padding-left: 20px;">
                            <li><strong>SMTP Host:</strong> ${host}</li>
                            <li><strong>SMTP Port:</strong> ${port}</li>
                            <li><strong>Secure:</strong> ${secure ? 'Yes (SSL/TLS)' : 'No (STARTTLS)'}</li>
                            <li><strong>From Address:</strong> ${from}</li>
                            <li><strong>To Address:</strong> ${to}</li>
                        </ul>
                        
                        <p style="margin: 15px 0 0 0; color: #065f46;">
                            You will now receive alert notifications when VMs/LXCs exceed their configured thresholds.
                        </p>
                    </div>
                    
                    <div style="background: white; padding: 20px; border-radius: 0 0 8px 8px; border-top: 1px solid #d1fae5;">
                        <p style="margin: 0; color: #6b7280; font-size: 12px; text-align: center;">
                            This test email was sent by your Pulse monitoring system.
                        </p>
                    </div>
                </div>
            `
        };
        
        await testTransporter.sendMail(testMailOptions);
        testTransporter.close();
        
        console.log(`[EMAIL TEST] Test email sent successfully to: ${to}`);
        res.json({
            success: true,
            message: 'Test email sent successfully!'
        });
        
    } catch (error) {
        console.error('[EMAIL TEST] Failed to send test email:', error);
        res.status(400).json({
            success: false,
            error: error.message || 'Failed to send test email'
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
        const pveBackups = updatedState.pveBackups || {};
        console.log(`[Discovery Broadcast] Broadcasting state with PVE backups: ${(pveBackups.backupTasks || []).length} tasks, ${(pveBackups.storageBackups || []).length} storage, ${(pveBackups.guestSnapshots || []).length} snapshots`);
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
