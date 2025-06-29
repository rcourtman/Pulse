#!/usr/bin/env node

/**
 * Pulse PBS Agent - Collects PBS metrics and pushes them to a Pulse server
 * 
 * This agent is designed to run on PBS servers that cannot be reached directly
 * by the Pulse monitoring server (e.g., behind firewalls, in isolated networks).
 * 
 * Environment variables:
 * - PULSE_SERVER_URL: URL of the Pulse server (required)
 * - PULSE_API_KEY: API key for authentication (required)
 * - PBS_API_URL: PBS API URL (default: https://localhost:8007)
 * - PBS_API_TOKEN: PBS API token (required)
 * - PBS_FINGERPRINT: PBS server fingerprint (optional, disables cert verification if not provided)
 * - PUSH_INTERVAL: Interval between pushes in seconds (default: 30)
 * - AGENT_ID: Unique identifier for this PBS instance (default: hostname)
 */

const https = require('https');
const axios = require('axios');
const os = require('os');

// Load environment variables
require('dotenv').config();

// Configuration
const config = {
    pulseServerUrl: process.env.PULSE_SERVER_URL,
    pulseApiKey: process.env.PULSE_API_KEY,
    pbsApiUrl: process.env.PBS_API_URL || 'https://localhost:8007',
    pbsApiToken: process.env.PBS_API_TOKEN,
    pbsFingerprint: process.env.PBS_FINGERPRINT,
    pushInterval: parseInt(process.env.PUSH_INTERVAL || '30') * 1000,
    agentId: process.env.AGENT_ID || os.hostname(),
    agentVersion: '1.0.0'
};

// Validate configuration
if (!config.pulseServerUrl) {
    console.error('ERROR: PULSE_SERVER_URL environment variable is required');
    process.exit(1);
}

if (!config.pulseApiKey) {
    console.error('ERROR: PULSE_API_KEY environment variable is required');
    process.exit(1);
}

if (!config.pbsApiToken) {
    console.error('ERROR: PBS_API_TOKEN environment variable is required');
    process.exit(1);
}

// Create axios instance for PBS API
const pbsApi = axios.create({
    baseURL: config.pbsApiUrl,
    headers: {
        'Authorization': `PBSAPIToken=${config.pbsApiToken}`
    },
    httpsAgent: new https.Agent({
        rejectUnauthorized: !!config.pbsFingerprint,
        ...(config.pbsFingerprint && { fingerprint: config.pbsFingerprint })
    }),
    timeout: 10000
});

// Create axios instance for Pulse API
const pulseApi = axios.create({
    baseURL: config.pulseServerUrl,
    headers: {
        'X-API-Key': config.pulseApiKey,
        'Content-Type': 'application/json'
    },
    timeout: 15000
});

/**
 * Discover PBS node name using the localhost workaround
 */
async function discoverNodeName() {
    try {
        // Use localhost as dummy node name to get a task
        const response = await pbsApi.get('/api2/json/nodes/localhost/tasks', {
            params: { limit: 1 }
        });
        
        if (response.data?.data?.length > 0) {
            const task = response.data.data[0];
            // Extract node name from UPID or task object
            if (task.node) {
                return task.node;
            } else if (task.upid) {
                // UPID format: UPID:nodename:...
                const parts = task.upid.split(':');
                if (parts.length > 1) {
                    return parts[1];
                }
            }
        }
        
        // Fallback to localhost if discovery fails
        console.warn('Could not discover node name, using localhost');
        return 'localhost';
    } catch (error) {
        console.error('Error discovering node name:', error.message);
        return 'localhost';
    }
}

/**
 * Fetch PBS node status
 */
async function fetchNodeStatus(nodeName) {
    try {
        const response = await pbsApi.get(`/api2/json/nodes/${nodeName}/status`);
        return response.data?.data || {};
    } catch (error) {
        console.error('Error fetching node status:', error.message);
        return null;
    }
}

/**
 * Fetch PBS datastore information
 */
async function fetchDatastores() {
    try {
        const response = await pbsApi.get('/api2/json/status/datastore-usage');
        return response.data?.data || [];
    } catch (error) {
        console.error('Error fetching datastores:', error.message);
        return [];
    }
}

/**
 * Fetch PBS tasks
 */
async function fetchTasks(nodeName, limit = 100) {
    try {
        const response = await pbsApi.get(`/api2/json/nodes/${nodeName}/tasks`, {
            params: { limit }
        });
        return response.data?.data || [];
    } catch (error) {
        console.error('Error fetching tasks:', error.message);
        return [];
    }
}

/**
 * Fetch PBS version
 */
async function fetchVersion() {
    try {
        const response = await pbsApi.get('/api2/json/version');
        return response.data?.data?.version || 'unknown';
    } catch (error) {
        console.error('Error fetching version:', error.message);
        return 'unknown';
    }
}

/**
 * Collect all PBS metrics
 */
async function collectMetrics() {
    console.log('Collecting PBS metrics...');
    
    // Discover node name first
    const nodeName = await discoverNodeName();
    console.log(`Using node name: ${nodeName}`);
    
    // Collect all metrics in parallel
    const [nodeStatus, datastores, tasks, version] = await Promise.all([
        fetchNodeStatus(nodeName),
        fetchDatastores(),
        fetchTasks(nodeName),
        fetchVersion()
    ]);
    
    const metrics = {
        pbsId: config.agentId,
        nodeStatus,
        datastores,
        tasks,
        version,
        timestamp: Date.now(),
        agentVersion: config.agentVersion,
        pushInterval: config.pushInterval
    };
    
    console.log(`Collected metrics: ${datastores.length} datastores, ${tasks.length} tasks`);
    return metrics;
}

/**
 * Push metrics to Pulse server with retry logic
 */
let consecutiveFailures = 0;
const MAX_CONSECUTIVE_FAILURES = 10;

async function pushMetrics(metrics) {
    try {
        console.log(`Pushing metrics to ${config.pulseServerUrl}/api/push/metrics`);
        const response = await pulseApi.post('/api/push/metrics', metrics);
        console.log('Metrics pushed successfully:', response.data);
        
        // Reset failure counter on success
        consecutiveFailures = 0;
        return true;
    } catch (error) {
        consecutiveFailures++;
        
        // Detailed error logging
        const timestamp = new Date().toISOString();
        console.error(`[${timestamp}] Error pushing metrics (attempt ${consecutiveFailures}/${MAX_CONSECUTIVE_FAILURES}):`, error.message);
        
        if (error.response) {
            console.error('Response:', error.response.status, error.response.data);
            
            // Handle specific error codes
            switch (error.response.status) {
                case 429:
                    console.error('Rate limit exceeded. Will retry in next interval.');
                    break;
                case 401:
                case 403:
                    console.error('Authentication failed. Check PULSE_API_KEY.');
                    break;
                case 503:
                    console.error('Push mode not configured on Pulse server.');
                    break;
            }
        } else if (error.code === 'ECONNREFUSED') {
            console.error('Connection refused. Is Pulse server running?');
        } else if (error.code === 'ETIMEDOUT') {
            console.error('Connection timeout. Check network connectivity.');
        } else if (error.code === 'ENOTFOUND') {
            console.error('Server not found. Check PULSE_SERVER_URL.');
        }
        
        // Exit if too many consecutive failures
        if (consecutiveFailures >= MAX_CONSECUTIVE_FAILURES) {
            console.error(`Failed ${MAX_CONSECUTIVE_FAILURES} times consecutively. Exiting.`);
            process.exit(1);
        }
        
        return false;
    }
}

/**
 * Main loop
 */
async function main() {
    console.log('Pulse PBS Agent starting...');
    console.log(`Agent ID: ${config.agentId}`);
    console.log(`Pulse Server: ${config.pulseServerUrl}`);
    console.log(`PBS API: ${config.pbsApiUrl}`);
    console.log(`Push Interval: ${config.pushInterval / 1000}s`);
    
    // Initial push
    try {
        const metrics = await collectMetrics();
        await pushMetrics(metrics);
    } catch (error) {
        console.error('Initial push failed:', error.message);
    }
    
    // Set up interval
    setInterval(async () => {
        try {
            const metrics = await collectMetrics();
            await pushMetrics(metrics);
        } catch (error) {
            console.error('Push cycle failed:', error.message);
        }
    }, config.pushInterval);
    
    console.log('Agent is running. Press Ctrl+C to stop.');
}

// Handle graceful shutdown
process.on('SIGINT', () => {
    console.log('\nShutting down agent...');
    process.exit(0);
});

process.on('SIGTERM', () => {
    console.log('\nShutting down agent...');
    process.exit(0);
});

// Start the agent
main().catch(error => {
    console.error('Fatal error:', error);
    process.exit(1);
});