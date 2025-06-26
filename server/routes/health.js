
const express = require('express');
const stateManager = require('../state');
const metricsHistory = require('../metricsHistory');

const router = express.Router();

// Health check endpoint - returns simple OK/NOT OK based on actual readiness
router.get('/healthz', (req, res) => {
    try {
        const state = stateManager.getState();
        const hasData = stateManager.hasData();
        const clientsInitialized = Object.keys(global.pulseApiClients?.apiClients || {}).length > 0;
        
        // Service is healthy if:
        // 1. API clients are initialized
        const isHealthy = clientsInitialized && (hasData || state.isConfigPlaceholder);
        
        if (isHealthy) {
            res.status(200).send('OK');
        } else {
            // 503 Service Unavailable - not ready yet
            res.status(503).send('Service starting up');
        }
    } catch (error) {
        console.error("Error in health check:", error);
        res.status(503).send('Service unavailable');
    }
});

// Detailed health endpoint with monitoring info
router.get('/', (req, res) => {
    try {
        const healthSummary = stateManager.getHealthSummary();
        // Add system info including placeholder status
        const state = stateManager.getState();
        healthSummary.system = {
            configPlaceholder: state.isConfigPlaceholder || false,
            hasData: stateManager.hasData(),
            clientsInitialized: Object.keys(global.pulseApiClients?.apiClients || {}).length > 0
        };
        
        // Add persistence stats
        healthSummary.persistence = {
            enabled: true,
            lastSnapshot: global.lastSnapshotStats ? {
                timestamp: global.lastSnapshotStats.timestamp,
                ageSeconds: Math.floor((Date.now() - global.lastSnapshotStats.timestamp) / 1000),
                guests: global.lastSnapshotStats.guests,
                nodes: global.lastSnapshotStats.nodes,
                totalDataPoints: global.lastSnapshotStats.totalDataPoints,
                sizeKB: Math.round(global.lastSnapshotStats.sizeBytes / 1024 * 10) / 10
            } : null,
            metricsRestored: global.metricsRestored || null,
            historyStats: metricsHistory.getStats()
        };
        
        res.json(healthSummary);
    } catch (error) {
        console.error("Error in /api/health:", error);
        res.status(500).json({ error: "Failed to fetch health information" });
    }
});

module.exports = router;
