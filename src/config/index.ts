import dotenv from 'dotenv';
import { AppConfig, NodeConfig } from '../types';

// Load environment variables
dotenv.config();

/**
 * Parse node configurations from environment variables
 */
function parseNodeConfigs(): NodeConfig[] {
  // Get the global auto-detect cluster setting
  const autoDetectCluster = process.env.PROXMOX_AUTO_DETECT_CLUSTER !== 'false';
  const mockClusterEnabled = process.env.MOCK_CLUSTER_MODE !== 'false' && 
                              process.env.MOCK_CLUSTER_ENABLED === 'true';
  const clusterMode = process.env.PROXMOX_CLUSTER_MODE === 'true';

  // If mock data is enabled, return mock nodes instead of real ones
  if (process.env.USE_MOCK_DATA === 'true' || process.env.MOCK_DATA_ENABLED === 'true') {
    console.log('Mock data enabled. Using mock nodes instead of real Proxmox servers.');
    
    // In mock cluster mode, we're modifying the approach to better match real Proxmox behavior
    // In a real Proxmox cluster, you can connect to ANY node and get information about ALL nodes
    // So we'll configure all mock nodes to point to the same mock server
    
    if (mockClusterEnabled || clusterMode) {
      console.log('Mock cluster mode enabled. Marking the first node as cluster entry point.');
      
      // Return all nodes but mark the first one as a cluster entry point
      return [
        {
          id: 'node-1',
          name: 'pve-cluster-01',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster: true, // Force true in cluster mode
          isClusterEntryPoint: true // Mark this node as the cluster entry point
        },
        {
          id: 'node-2',
          name: 'pve-prod-02',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster: true,
          isClusterEntryPoint: false
        },
        {
          id: 'node-3',
          name: 'pve-dev-01',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster: true,
          isClusterEntryPoint: false
        }
      ];
    } else {
      // Non-cluster mode - return all nodes separately as before
      return [
        {
          id: 'node-1',
          name: 'pve-prod-01',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster
        },
        {
          id: 'node-2',
          name: 'pve-prod-02',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster
        },
        {
          id: 'node-3',
          name: 'pve-dev-01',
          host: 'http://localhost:7656',
          tokenId: 'mock-token',
          tokenSecret: 'mock-secret',
          autoDetectCluster
        }
      ];
    }
  }

  const nodes: NodeConfig[] = [];
  
  // Parse node configurations using standard format: PROXMOX_NODE_X_NAME, PROXMOX_NODE_X_HOST, etc.
  for (let i = 1; i <= 10; i++) {
    const hostKey = `PROXMOX_NODE_${i}_HOST`;
    const nodeNameKey = `PROXMOX_NODE_${i}_NAME`;
    const tokenIdKey = `PROXMOX_NODE_${i}_TOKEN_ID`;
    const tokenSecretKey = `PROXMOX_NODE_${i}_TOKEN_SECRET`;
    
    const host = process.env[hostKey];
    const nodeName = process.env[nodeNameKey];
    const tokenId = process.env[tokenIdKey];
    const tokenSecret = process.env[tokenSecretKey];
    
    if (host && nodeName && tokenId && tokenSecret && 
        tokenSecret !== 'your-token-secret-here' && 
        tokenSecret !== 'your-token-secret') {
      nodes.push({
        id: `node-${i}`,
        name: nodeName,
        host,
        tokenId,
        tokenSecret,
        autoDetectCluster
      });
    }
  }
  
  return nodes;
}

/**
 * Validate the configuration
 */
function validateConfig(config: AppConfig): void {
  // Check if at least one node is configured
  if (config.nodes.length === 0) {
    throw new Error('No valid Proxmox VE nodes configured. Please check your environment variables.');
  }
  
  // Validate port
  if (isNaN(config.port) || config.port <= 0 || config.port > 65535) {
    throw new Error(`Invalid port: ${config.port}. Port must be a number between 1 and 65535.`);
  }
  
  // Validate metrics history minutes
  if (isNaN(config.metricsHistoryMinutes) || config.metricsHistoryMinutes <= 0) {
    throw new Error(`Invalid metrics history minutes: ${config.metricsHistoryMinutes}. Must be a positive number.`);
  }
  
  // Validate maximum realistic rate
  if (isNaN(config.maxRealisticRate) || config.maxRealisticRate <= 0) {
    console.warn(`Invalid maximum realistic rate: ${config.maxRealisticRate}. Using 125 MB/s as default.`);
    config.maxRealisticRate = 125;
  }
  
  // Validate polling intervals
  if (isNaN(config.nodePollingIntervalMs) || config.nodePollingIntervalMs < 1000) {
    console.warn(`Invalid node polling interval: ${config.nodePollingIntervalMs}. Using 15000ms as default.`);
    config.nodePollingIntervalMs = 15000;
  }
  
  if (isNaN(config.eventPollingIntervalMs) || config.eventPollingIntervalMs < 1000) {
    console.warn(`Invalid event polling interval: ${config.eventPollingIntervalMs}. Using 3000ms as default.`);
    config.eventPollingIntervalMs = 3000;
  }
  
  // Validate log level
  const validLogLevels = ['error', 'warn', 'info', 'debug', 'silly'];
  if (!validLogLevels.includes(config.logLevel)) {
    console.warn(`Invalid log level: ${config.logLevel}. Using 'info' as default.`);
    config.logLevel = 'info';
  }
}

// Default configuration
const config: AppConfig = {
  port: parseInt(process.env.PORT || '7654', 10),
  nodeEnv: process.env.NODE_ENV || 'development',
  logLevel: process.env.LOG_LEVEL || 'info',
  enableDevTools: process.env.ENABLE_DEV_TOOLS === 'true',
  metricsHistoryMinutes: parseInt(process.env.METRICS_HISTORY_MINUTES || '60', 10),
  // Maximum realistic network rate in MB/s (default: 125 MB/s = 1 Gbps)
  maxRealisticRate: parseInt(process.env.METRICS_MAX_REALISTIC_RATE || '125', 10),
  // Check multiple environment variables that could control SSL verification
  ignoreSSLErrors: process.env.IGNORE_SSL_ERRORS === 'true' || 
                   process.env.PROXMOX_REJECT_UNAUTHORIZED === 'false' ||
                   process.env.NODE_TLS_REJECT_UNAUTHORIZED === '0' ||
                   process.env.HTTPS_REJECT_UNAUTHORIZED === 'false' ||
                   process.env.PROXMOX_INSECURE === 'true' ||
                   process.env.PROXMOX_VERIFY_SSL === 'false',
  // More responsive polling intervals - reduced for maximum responsiveness
  nodePollingIntervalMs: parseInt(process.env.NODE_POLLING_INTERVAL_MS || '3000', 10),
  eventPollingIntervalMs: parseInt(process.env.EVENT_POLLING_INTERVAL_MS || '1000', 10),
  nodes: parseNodeConfigs(),
  // Auto-detect cluster mode by default, but allow override via env var
  clusterMode: process.env.PROXMOX_CLUSTER_MODE !== 'false', // Default to true unless explicitly set to false
  clusterName: process.env.PROXMOX_CLUSTER_NAME || 'proxmox-cluster',
  // Flag to indicate if cluster detection should be automatic
  autoDetectCluster: process.env.PROXMOX_AUTO_DETECT_CLUSTER !== 'false' // Default to true unless explicitly set to false
};

// Validate the configuration
validateConfig(config);

export default config; 