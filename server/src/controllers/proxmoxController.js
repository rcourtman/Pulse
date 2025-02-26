const axios = require('axios');
const logger = require('../utils/logger');
const ProxmoxService = require('../services/proxmoxService');
const { loadProxmoxNodesFromEnv } = require('../config/proxmoxNodes');

// Cache for configured nodes
let configuredNodes = null;

/**
 * Get pre-configured Proxmox nodes from environment variables
 */
exports.getConfiguredNodes = async (req, res, next) => {
  try {
    // Load nodes from environment if not already loaded
    if (!configuredNodes) {
      configuredNodes = loadProxmoxNodesFromEnv();
    }
    
    // Check if this is a v2 API request (direct /api/nodes endpoint)
    const isV2Request = req.path === '/nodes';
    
    if (isV2Request) {
      // For v2 API, return a simple array of nodes
      const nodes = configuredNodes.map(node => ({
        id: node.id.replace('node-', ''), // Remove 'node-' prefix
        name: node.name,
        url: node.url,
      }));
      
      logger.info(`Returning ${nodes.length} nodes for v2 API request`);
      return res.status(200).json(nodes);
    } else {
      // For legacy API, return the old format
      // Return nodes without sensitive token information
      const safeNodes = configuredNodes.map(node => ({
        id: node.id,
        name: node.name,
        url: node.url,
        tokenId: node.tokenId,
        // Don't include the actual token for security reasons
      }));
      
      logger.info(`Returning ${safeNodes.length} nodes for legacy API request`);
      return res.status(200).json({ 
        success: true, 
        data: safeNodes,
        count: safeNodes.length
      });
    }
  } catch (error) {
    logger.error('Error fetching configured Proxmox nodes:', error);
    
    return res.status(500).json({
      error: error.message || 'Failed to fetch configured Proxmox nodes',
    });
  }
};

/**
 * Validate Proxmox connection
 */
exports.validateConnection = async (req, res, next) => {
  try {
    const { url, tokenId, token } = req.body;
    
    if (!url || !tokenId || !token) {
      return res.status(400).json({
        error: 'Missing required parameters: url, tokenId, token',
      });
    }
    
    const proxmoxService = new ProxmoxService(url, tokenId, token);
    const result = await proxmoxService.validateConnection();
    
    return res.status(200).json({ success: true, data: result });
  } catch (error) {
    logger.error('Error validating Proxmox connection:', error);
    
    return res.status(error.response?.status || 500).json({
      error: error.message || 'Failed to validate Proxmox connection',
    });
  }
};

/**
 * Get resources from Proxmox
 */
exports.getResources = async (req, res, next) => {
  try {
    const { url, tokenId, token } = req.query;
    
    if (!url || !tokenId || !token) {
      return res.status(400).json({
        error: 'Missing required parameters: url, tokenId, token',
      });
    }
    
    const proxmoxService = new ProxmoxService(url, tokenId, token);
    const rawResources = await proxmoxService.getResources();
    
    // Return the raw, unmodified data from Proxmox
    return res.status(200).json({ 
      success: true, 
      data: rawResources,
      raw: true // Flag to indicate this is raw data
    });
  } catch (error) {
    logger.error('Error fetching Proxmox resources:', error);
    
    return res.status(error.response?.status || 500).json({
      error: error.message || 'Failed to fetch Proxmox resources',
    });
  }
}; 