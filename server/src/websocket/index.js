const logger = require('../utils/logger');
const ProxmoxService = require('../services/proxmoxService');
const { loadProxmoxNodesFromEnv, getPollingInterval } = require('../config/proxmoxNodes');

// Cache for active connections and data
const activeConnections = new Map();
const resourceCache = new Map();
const lastFetchTimes = new Map();

// Set default polling interval
const DEFAULT_POLLING_INTERVAL = getPollingInterval();

// Connection monitoring
const connectionFailures = new Map();
const MAX_FAILURES = 5;

// Track global polling tasks
const pollingTasks = new Map();

/**
 * Set up WebSocket server
 */
const setupWebSocketServer = (io) => {
  // Load pre-configured nodes
  const configuredNodes = loadProxmoxNodesFromEnv();
  
  // Create a namespace for the WebSocket API
  const wsNamespace = io.of('/api/ws');
  
  // Configure socket.io parameters for more reliable connections
  io.engine.pingTimeout = 10000; // How long to wait for a ping response (ms)
  io.engine.pingInterval = 15000; // How often to ping clients (ms)
  
  // Log configured nodes
  logger.info(`WebSocket server initialized with ${configuredNodes.length} configured nodes`);
  configuredNodes.forEach(node => {
    logger.debug(`Configured node: ${node.name} (${node.url})`);
  });
  
  // Log the polling interval
  logger.info(`WebSocket server using default polling interval of ${DEFAULT_POLLING_INTERVAL}ms`);
  
  // Enable Socket.IO debug logs in development
  if (process.env.NODE_ENV !== 'production') {
    io.engine.on('connection', (socket) => {
      logger.debug(`Socket.IO engine connection: ${socket.id}`);
    });
    
    io.engine.on('close', (socket) => {
      logger.debug(`Socket.IO engine closed: ${socket.id}`);
    });
  }
  
  // Create Proxmox service instances for each configured node
  const proxmoxServices = new Map();
  configuredNodes.forEach(node => {
    try {
      const service = new ProxmoxService(node.url, node.tokenId, node.token);
      proxmoxServices.set(node.id, {
        service,
        config: node
      });
      logger.debug(`Created Proxmox service for node ${node.id} (${node.name})`);
    } catch (error) {
      logger.error(`Failed to create Proxmox service for node ${node.id} (${node.name}): ${error.message}`);
    }
  });
  
  // Start polling for all configured nodes
  startGlobalPolling(proxmoxServices);
  
  // Handle WebSocket connections
  wsNamespace.on('connection', (socket) => {
    logger.info(`New WebSocket connection: ${socket.id}`);
    
    // Initialize connection state
    connectionFailures.set(socket.id, 0);
    
    // Send available nodes list immediately
    const availableNodes = configuredNodes.map(node => ({
      id: node.id,
      name: node.name,
      url: node.url
    }));
    socket.emit('nodes', availableNodes);
    
    // Handle node subscription
    socket.on('subscribe', async (data) => {
      try {
        const { nodeId } = data;
        
        if (!nodeId) {
          throw new Error('Missing nodeId parameter');
        }
        
        // Find the node configuration
        const nodeConfig = configuredNodes.find(node => 
          node.id === nodeId || 
          node.id === parseInt(nodeId, 10) || 
          `node-${node.id}` === nodeId
        );
        
        if (!nodeConfig) {
          throw new Error(`Node not found: ${nodeId}`);
        }
        
        logger.info(`Socket ${socket.id} subscribing to node ${nodeId}: ${nodeConfig.name}`);
        
        // Register the subscription
        if (!activeConnections.has(socket.id)) {
          activeConnections.set(socket.id, { nodeId });
        } else {
          activeConnections.get(socket.id).nodeId = nodeId;
        }
        
        // Acknowledge subscription
        socket.emit('subscribed', { 
          nodeId,
          nodeName: nodeConfig.name,
          timestamp: Date.now()
        });
        
        // Send 'connecting' status
        socket.emit('connecting', { 
          nodeId, 
          nodeName: nodeConfig.name,
          timestamp: Date.now() 
        });
        
        // Send initial data if available
        if (resourceCache.has(nodeId)) {
          const cachedData = resourceCache.get(nodeId);
          socket.emit('data', {
            timestamp: cachedData.timestamp,
            resources: processResources(cachedData.resources),
            nodeId
          });
          
          // Send connected status after sending data
          socket.emit('connected', { 
            nodeId, 
            nodeName: nodeConfig.name,
            timestamp: Date.now() 
          });
        } else {
          // We don't have cached data, so wait for the next polling cycle
          // The global polling mechanism will fetch and broadcast the data
          
          // Try to validate connection just to make sure it works
          try {
            const service = proxmoxServices.get(nodeId)?.service;
            if (service) {
              await service.validateConnection();
              socket.emit('connected', { 
                nodeId, 
                nodeName: nodeConfig.name,
                timestamp: Date.now() 
              });
            } else {
              throw new Error(`No Proxmox service found for node ${nodeId}`);
            }
          } catch (error) {
            logger.error(`Connection validation failed for node ${nodeId}: ${error.message}`);
            socket.emit('error', {
              nodeId,
              message: `Failed to connect to Proxmox: ${error.message}`,
              code: 'PROXMOX_CONNECTION_ERROR',
              timestamp: Date.now()
            });
          }
        }
      } catch (error) {
        logger.error(`Subscription error: ${error.message}`);
        socket.emit('error', { 
          message: error.message,
          code: 'SUBSCRIPTION_ERROR',
          timestamp: Date.now() 
        });
      }
    });
    
    // Handle unsubscribe
    socket.on('unsubscribe', () => {
      if (activeConnections.has(socket.id)) {
        const { nodeId } = activeConnections.get(socket.id);
        logger.info(`Socket ${socket.id} unsubscribed from node ${nodeId}`);
        activeConnections.delete(socket.id);
        socket.emit('unsubscribed', { timestamp: Date.now() });
      }
    });
    
    // Handle disconnection
    socket.on('disconnect', (reason) => {
      logger.info(`WebSocket disconnected: ${socket.id}, reason: ${reason}`);
      activeConnections.delete(socket.id);
      connectionFailures.delete(socket.id);
    });
    
    // Handle ping/pong for connection liveliness
    socket.on('ping', (data) => {
      try {
        // Reset failure count on successful ping
        connectionFailures.set(socket.id, 0);
        
        // Send pong response
        socket.emit('pong', { 
          type: 'pong', 
          timestamp: Date.now(),
          received: data ? data.timestamp : null 
        });
      } catch (error) {
        logger.error(`Error handling ping from ${socket.id}: ${error.message}`);
      }
    });
    
    // Support for legacy message-based ping/pong
    socket.on('message', (data) => {
      try {
        const message = typeof data === 'string' ? JSON.parse(data) : data;
        if (message && message.type === 'ping') {
          socket.emit('pong', { 
            type: 'pong', 
            timestamp: Date.now(),
            received: message.timestamp
          });
        }
      } catch (error) {
        // Ignore invalid messages
        logger.debug(`Ignoring invalid message: ${error.message}`);
      }
    });
  });
  
  /**
   * Start global polling for all Proxmox nodes
   */
  function startGlobalPolling(proxmoxServices) {
    // Clean up any existing polling tasks
    pollingTasks.forEach(task => clearInterval(task));
    pollingTasks.clear();
    
    // Start a polling task for each node
    proxmoxServices.forEach(({ service, config }, nodeId) => {
      const pollNode = async () => {
        try {
          // Check if we should fetch new data based on time since last fetch
          const now = Date.now();
          const lastFetch = lastFetchTimes.get(nodeId) || 0;
          const timeElapsed = now - lastFetch;
          
          // Skip if we fetched recently (avoid duplicate fetches)
          if (timeElapsed < DEFAULT_POLLING_INTERVAL * 0.8) {
            return;
          }
          
          // Check if we have any active subscribers for this node
          const subscribers = getNodeSubscribers(nodeId);
          if (subscribers.length === 0) {
            // Only log at debug level since this is expected behavior
            logger.debug(`No active subscribers for node ${nodeId}, skipping fetch`);
            return;
          }
          
          // Update the last fetch time
          lastFetchTimes.set(nodeId, now);
          
          // Fetch resources from Proxmox
          logger.debug(`Fetching resources for node ${nodeId}: ${config.name}`);
          const resources = await service.getResources();
          
          // Store in cache
          resourceCache.set(nodeId, {
            timestamp: now,
            resources
          });
          
          // Broadcast to all subscribers
          broadcastNodeData(nodeId, resources);
          
        } catch (error) {
          logger.error(`Error fetching data for node ${nodeId}: ${error.message}`);
          
          // Record the error
          recordNodeError(nodeId, error);
          
          // Broadcast error to all subscribers
          const subscriberSockets = getNodeSubscribers(nodeId);
          subscriberSockets.forEach(socketId => {
            const socket = wsNamespace.sockets.get(socketId);
            if (socket) {
              // Increment connection failure count
              const failureCount = (connectionFailures.get(socketId) || 0) + 1;
              connectionFailures.set(socketId, failureCount);
              
              socket.emit('error', {
                nodeId,
                message: `Error fetching Proxmox data: ${error.message}`,
                code: 'DATA_FETCH_ERROR',
                attempt: failureCount,
                timestamp: Date.now()
              });
              
              // If we've had too many failures, emit a critical error
              if (failureCount >= MAX_FAILURES) {
                socket.emit('error', {
                  nodeId,
                  message: `Multiple connection failures (${failureCount}). Please check your Proxmox server.`,
                  code: 'MULTIPLE_FETCH_FAILURES',
                  critical: true,
                  timestamp: Date.now()
                });
              }
            }
          });
        }
      };
      
      // Initial poll
      pollNode();
      
      // Set up recurring polling
      const task = setInterval(pollNode, DEFAULT_POLLING_INTERVAL);
      pollingTasks.set(nodeId, task);
      
      logger.debug(`Started polling task for node ${nodeId} with interval ${DEFAULT_POLLING_INTERVAL}ms`);
    });
  }
  
  // Track node errors (for potential future health monitoring)
  const nodeErrors = new Map();
  
  function recordNodeError(nodeId, error) {
    if (!nodeErrors.has(nodeId)) {
      nodeErrors.set(nodeId, []);
    }
    
    const errors = nodeErrors.get(nodeId);
    errors.push({
      timestamp: Date.now(),
      message: error.message,
      code: error.code || 'UNKNOWN_ERROR'
    });
    
    // Keep only the last 10 errors
    if (errors.length > 10) {
      errors.shift();
    }
  }
  
  /**
   * Get all socket IDs subscribed to a node
   */
  function getNodeSubscribers(nodeId) {
    const subscribers = [];
    
    activeConnections.forEach((connection, socketId) => {
      if (connection.nodeId === nodeId) {
        subscribers.push(socketId);
      }
    });
    
    return subscribers;
  }
  
  /**
   * Broadcast data to all subscribers of a node
   */
  function broadcastNodeData(nodeId, resources) {
    const subscribers = getNodeSubscribers(nodeId);
    
    if (subscribers.length === 0) {
      logger.debug(`No subscribers for node ${nodeId}, skipping broadcast`);
      return;
    }
    
    logger.debug(`Broadcasting data to ${subscribers.length} subscribers for node ${nodeId}`);
    
    // Get the node config
    const nodeConfig = configuredNodes.find(node => node.id === nodeId);
    const nodeName = nodeConfig ? nodeConfig.name : nodeId;
    
    // Process the resources
    const processedResources = processResources(resources);
    
    // Broadcast to all subscribers
    subscribers.forEach(socketId => {
      const socket = wsNamespace.sockets.get(socketId);
      if (socket) {
        // Reset failure count on successful data broadcast
        connectionFailures.set(socketId, 0);
        
        socket.emit('data', {
          timestamp: Date.now(),
          resources: processedResources,
          nodeId,
          nodeName
        });
      }
    });
  }
  
  /**
   * Process resources to ensure consistent data format
   */
  function processResources(resources) {
    return resources.map(resource => {
      // Make a copy to avoid modifying the original
      const processed = { ...resource };
      
      // Process CPU values (Proxmox returns CPU as a decimal between 0-1)
      if (typeof processed.cpu === 'number') {
        // Store the raw CPU value for reference
        processed._rawCpu = processed.cpu;
        // Convert to percentage (0-100 range)
        processed.cpu = Math.min(processed.cpu * 100, 100);
      }
      
      // Process memory values
      if (processed.mem !== undefined && processed.maxmem && processed.maxmem > 0) {
        // Store raw memory values for reference
        processed._rawMem = processed.mem;
        processed._rawMaxMem = processed.maxmem;
        // Calculate memory percentage
        processed.memory = Math.min((processed.mem / processed.maxmem) * 100, 100);
      }
      
      return processed;
    });
  }
  
  // Implement health check endpoint
  io.httpServer.on('request', (req, res) => {
    if (req.url === '/api/health' || req.url === '/health') {
      // Check if headers are already sent
      if (res.headersSent) {
        logger.warn('Attempted to send health check response when headers were already sent');
        return;
      }
      
      res.writeHead(200, { 'Content-Type': 'application/json' });
      
      // Check if all polling tasks are running
      const allTasksRunning = [...pollingTasks.keys()].length === configuredNodes.length;
      
      res.end(JSON.stringify({
        status: 'ok',
        uptime: process.uptime(),
        timestamp: Date.now(),
        pollingTasks: [...pollingTasks.keys()].length,
        activeConnections: activeConnections.size,
        allTasksRunning
      }));
    }
  });
  
  // Graceful shutdown handler
  process.on('SIGINT', () => {
    logger.info('Shutting down WebSocket server...');
    pollingTasks.forEach(task => clearInterval(task));
    pollingTasks.clear();
    process.exit(0);
  });
};

module.exports = setupWebSocketServer; 