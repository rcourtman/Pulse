/**
 * Load Proxmox nodes from environment variables
 */
const logger = require('../utils/logger');

const loadProxmoxNodesFromEnv = () => {
  const nodes = [];
  let nodeIndex = 1;
  
  // Keep looking for nodes until we don't find any more
  while (process.env[`PROXMOX_NODE_${nodeIndex}_HOST`]) {
    const host = process.env[`PROXMOX_NODE_${nodeIndex}_HOST`];
    const tokenId = process.env[`PROXMOX_NODE_${nodeIndex}_TOKEN_ID`];
    const tokenSecret = process.env[`PROXMOX_NODE_${nodeIndex}_TOKEN_SECRET`];
    const name = process.env[`PROXMOX_NODE_${nodeIndex}_NAME`] || `Proxmox Node ${nodeIndex}`;
    
    if (host && tokenId && tokenSecret) {
      nodes.push({
        id: `node-${nodeIndex}`,
        name,
        url: host,
        tokenId,
        token: tokenSecret,
      });
      
      logger.info(`Loaded Proxmox node: ${name} (${host})`);
    } else {
      logger.warn(`Incomplete configuration for Proxmox node ${nodeIndex}, skipping`);
    }
    
    nodeIndex++;
  }
  
  if (nodes.length === 0) {
    logger.warn('No Proxmox nodes configured in environment variables');
  } else {
    logger.info(`Loaded ${nodes.length} Proxmox node(s) from environment variables`);
  }
  
  return nodes;
};

/**
 * Get the polling interval from environment variables
 * @returns {number} Polling interval in milliseconds
 */
const getPollingInterval = () => {
  const interval = parseInt(process.env.POLLING_INTERVAL, 10);
  // Increase default polling interval to 5 seconds for more noticeable changes
  return isNaN(interval) ? 5000 : interval; // Default to 5000ms (5 seconds)
};

module.exports = {
  loadProxmoxNodesFromEnv,
  getPollingInterval,
}; 