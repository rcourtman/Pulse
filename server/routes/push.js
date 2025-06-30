const express = require('express');
const router = express.Router();
const stateManager = require('../state');
const metricsHistory = require('../metricsHistory');

// Security: Request size limit (10MB max)
router.use(express.json({ limit: '10mb' }));

// Security: Basic rate limiting (in-memory)
const rateLimitMap = new Map();
const RATE_LIMIT_WINDOW = 60000; // 1 minute
const RATE_LIMIT_MAX = 60; // 60 requests per minute

function checkRateLimit(apiKey) {
    const now = Date.now();
    const userLimits = rateLimitMap.get(apiKey) || { count: 0, resetTime: now + RATE_LIMIT_WINDOW };
    
    // Reset if window expired
    if (now > userLimits.resetTime) {
        userLimits.count = 0;
        userLimits.resetTime = now + RATE_LIMIT_WINDOW;
    }
    
    userLimits.count++;
    rateLimitMap.set(apiKey, userLimits);
    
    if (userLimits.count > RATE_LIMIT_MAX) {
        return false;
    }
    
    return true;
}

// Clean up old rate limit entries periodically
setInterval(() => {
    const now = Date.now();
    const expiredTime = now - (RATE_LIMIT_WINDOW * 2); // Clean entries older than 2 windows
    
    for (const [key, limits] of rateLimitMap) {
        if (limits.resetTime < expiredTime) {
            rateLimitMap.delete(key);
        }
    }
}, RATE_LIMIT_WINDOW * 2); // Run cleanup every 2 minutes instead of every minute

// Middleware for API key authentication
const authenticatePushRequest = (req, res, next) => {
    const apiKey = req.headers['x-api-key'] || req.headers['authorization']?.replace('Bearer ', '');
    const configuredKey = process.env.PULSE_PUSH_API_KEY;
    
    if (!configuredKey) {
        return res.status(503).json({ 
            error: 'Push metrics not configured',
            message: 'PULSE_PUSH_API_KEY environment variable not set'
        });
    }
    
    if (!apiKey) {
        console.warn(`[PUSH] Authentication failed: No API key provided from ${req.ip}`);
        return res.status(401).json({ error: 'API key required' });
    }
    
    if (apiKey !== configuredKey) {
        console.warn(`[PUSH] Authentication failed: Invalid API key from ${req.ip}`);
        return res.status(403).json({ error: 'Invalid API key' });
    }
    
    // Check rate limit
    if (!checkRateLimit(apiKey)) {
        console.warn(`[PUSH] Rate limit exceeded for ${req.ip}`);
        return res.status(429).json({ error: 'Too many requests. Please try again later.' });
    }
    
    next();
};

// Input validation helper
function validateMetricsPayload(body) {
    const { pbsId, nodeStatus, datastores, tasks, timestamp } = body;
    
    // Validate pbsId
    if (typeof pbsId !== 'string' || pbsId.length === 0 || pbsId.length > 100) {
        throw new Error('Invalid pbsId: must be string between 1-100 characters');
    }
    
    // Validate timestamp
    if (!Number.isInteger(timestamp) || timestamp < 0 || timestamp > Date.now() + 300000) {
        throw new Error('Invalid timestamp: must be valid timestamp not in future');
    }
    
    // Validate nodeStatus is object
    if (typeof nodeStatus !== 'object' || nodeStatus === null) {
        throw new Error('Invalid nodeStatus: must be object');
    }
    
    // Validate arrays
    if (datastores && (!Array.isArray(datastores) || datastores.length > 100)) {
        throw new Error('Invalid datastores: must be array with max 100 items');
    }
    
    if (tasks && (!Array.isArray(tasks) || tasks.length > 1000)) {
        throw new Error('Invalid tasks: must be array with max 1000 items');
    }
    
    return true;
}

// POST /api/push/metrics - Receive pushed metrics from remote agents
router.post('/metrics', authenticatePushRequest, async (req, res) => {
    try {
        // Validate input
        try {
            validateMetricsPayload(req.body);
        } catch (validationError) {
            console.warn(`[PUSH] Validation failed from ${req.ip}: ${validationError.message}`);
            return res.status(400).json({ error: validationError.message });
        }
        
        const { 
            pbsId,
            nodeStatus,
            datastores,
            snapshots,
            tasks,
            version,
            timestamp,
            agentVersion 
        } = req.body;
        
        // Get current state
        const currentState = stateManager.getState();
        
        // Find or create PBS entry
        let pbsEntry = currentState.pbsServers?.find(pbs => pbs.id === pbsId);
        
        if (!pbsEntry) {
            // Create new PBS entry if it doesn't exist
            pbsEntry = {
                id: pbsId,
                name: pbsId,
                pushMode: true,
                source: 'push',
                lastPushReceived: Date.now()
            };
            
            if (!currentState.pbsServers) {
                currentState.pbsServers = [];
            }
            currentState.pbsServers.push(pbsEntry);
        }
        
        // Update PBS data - explicit property assignment to prevent pollution
        pbsEntry.version = version || pbsEntry.version;
        pbsEntry.nodeStatus = nodeStatus;
        pbsEntry.datastores = datastores || pbsEntry.datastores || [];
        pbsEntry.snapshots = snapshots || pbsEntry.snapshots || [];
        pbsEntry.tasks = tasks || pbsEntry.tasks || [];
        pbsEntry.lastPushReceived = Date.now();
        pbsEntry.pushMode = true;
        pbsEntry.source = 'push';
        pbsEntry.agentVersion = agentVersion;
        pbsEntry.online = true;
        pbsEntry.error = null;
        
        // Store metrics in history
        if (nodeStatus) {
            const nodeId = `pbs-${pbsId}`;
            metricsHistory.addNodeMetricData(nodeId, {
                cpu: nodeStatus.cpu || 0,
                memory: nodeStatus.memory || { used: 0, total: 0 },
                rootfs: nodeStatus.rootfs || { used: 0, total: 0 },
                swap: nodeStatus.swap || { used: 0, total: 0 }
            });
        }
        
        // Update state
        stateManager.updatePbsServers(currentState.pbsServers);
        
        // Calculate next expected push based on agent's push interval
        const pushInterval = req.body.pushInterval || 30000; // Default 30s
        const nextExpectedPush = Date.now() + pushInterval + 5000; // Add 5s buffer
        
        res.json({ 
            success: true,
            received: timestamp,
            processed: Date.now(),
            nextExpectedBefore: nextExpectedPush
        });
        
        console.log(`[PUSH] Received metrics from PBS agent: ${pbsId} (agent v${agentVersion || 'unknown'})`);
        
    } catch (error) {
        console.error('[PUSH] Error processing pushed metrics:', error);
        res.status(500).json({ 
            error: 'Failed to process metrics',
            message: error.message 
        });
    }
});

// GET /api/push/agents - List connected push agents
router.get('/agents', authenticatePushRequest, (req, res) => {
    try {
        const currentState = stateManager.getState();
        const pushAgents = (currentState.pbsServers || [])
            .filter(pbs => pbs.pushMode)
            .map(pbs => ({
                id: pbs.id,
                name: pbs.name,
                lastPushReceived: pbs.lastPushReceived,
                agentVersion: pbs.agentVersion,
                online: pbs.online && (Date.now() - pbs.lastPushReceived < 120000), // 2 min timeout
                version: pbs.version
            }));
            
        res.json({ agents: pushAgents });
    } catch (error) {
        console.error('[PUSH] Error listing agents:', error);
        res.status(500).json({ error: 'Failed to list agents' });
    }
});

module.exports = router;