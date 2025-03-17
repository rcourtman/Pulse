/**
 * Mock Data Server for Pulse
 * 
 * This script generates simulated data for development and testing.
 * It overrides the socket connection to provide consistent, visually appealing data.
 * 
 * Usage:
 * 1. Run this script with Node.js
 * 2. Start the backend with mock data enabled
 * 3. Start the frontend
 */

import express from 'express';
import cors from 'cors';
import http from 'http';
import { Server, Socket } from 'socket.io';
import path from 'path';
import { createLogger } from '../utils/logger';
import { ProxmoxEvent } from '../types';
import { customMockData } from './custom-data';
import bodyParser from 'body-parser';

const logger = createLogger('MockServer');

// Helper function to check if we're in cluster mode
function isClusterModeEnabled(): boolean {
  return process.env.PROXMOX_CLUSTER_MODE === 'true' || 
         process.env.MOCK_CLUSTER_ENABLED === 'true' || 
         process.env.MOCK_CLUSTER_MODE === 'true' ||
         (process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true' && 
          (process.env.USE_MOCK_DATA === 'true' || 
           process.env.MOCK_DATA_ENABLED === 'true'));
}

// Helper function to get filtered nodes based on cluster mode
function getFilteredNodes(): Map<string, any> {
  const clusterMode = isClusterModeEnabled();
  logger.debug(`Getting filtered nodes. Cluster mode: ${clusterMode}, Available nodes: ${nodes.size}`);
  
  if (clusterMode) {
    // Only include the entry point node in cluster mode
    const allNodes = Array.from(nodes.values());
    if (allNodes.length === 0) {
      logger.warn('No nodes available in the nodes map!');
      return nodes; // Return original nodes map (which is empty)
    }
    
    // Use the first node as the cluster entry point
    const entryPointNode = allNodes[0];
    logger.debug(`Using ${entryPointNode.name} (${entryPointNode.id}) as cluster entry point`);
    
    // Create a renamed clone of the entry point for cluster mode
    const clusterEntryPoint = {
      ...entryPointNode,
      name: 'pve-cluster-01', // Rename for consistency in cluster mode
      id: entryPointNode.id
    };
    
    const filteredNodes = new Map();
    filteredNodes.set(clusterEntryPoint.id, clusterEntryPoint);
    
    logger.debug(`Returning 1 node for cluster mode: pve-cluster-01`);
    return filteredNodes;
  }
  
  // Return all nodes if not in cluster mode
  logger.debug(`Returning all ${nodes.size} nodes (non-cluster mode)`);
  return nodes;
}

// Create Express app and HTTP server
const app = express();
app.use(cors());
app.use(bodyParser.json());
const server = http.createServer(app);
const io = new Server(server, {
  cors: {
    origin: '*',
    methods: ['GET', 'POST']
  }
});

// Port for the mock server
const PORT = (global as any).MOCK_SERVER_PORT ? parseInt((global as any).MOCK_SERVER_PORT, 10) : 7656;
// Host for the mock server (default to 0.0.0.0 to bind to all interfaces)
const HOST = (global as any).HOST || process.env.MOCK_SERVER_HOST || '0.0.0.0';

// Log the configuration
logger.info(`Mock server configured to use host: ${HOST}, port: ${PORT}`);

// Add a health check endpoint at the root path
app.get('/', (req, res) => {
  res.status(200).json({ status: 'ok', message: 'Mock server is running' });
});

// HA states from the Proxmox documentation
const HA_STATES = {
  STARTED: 'started',    // Resource is started and managed by HA
  STOPPED: 'stopped',    // Resource is stopped but still managed by HA
  DISABLED: 'disabled',  // Resource is disabled in HA
  IGNORED: 'ignored',    // Resource is ignored by HA
  ERROR: 'error',        // Resource is in error state
  FENCE: 'fence',        // Node needs to be fenced
  MIGRATE: 'migrate',    // Resource is being migrated
  RELOCATE: 'relocate',  // Resource is being relocated
  RECOVERY: 'recovery'   // Resource is in recovery process
};

// Migration interval for testing HA status changes
const MIGRATION_INTERVAL_MS = 10000; // 10 seconds between migrations

// Generate a random number between min and max
const randomBetween = (min: number, max: number): number => Math.floor(Math.random() * (max - min + 1) + min);

// Generate a random floating point number between min and max with specified precision
const randomFloatBetween = (min: number, max: number, precision = 2): number => {
  const value = Math.random() * (max - min) + min;
  return parseFloat(value.toFixed(precision));
};

// Generate a random IP address
const randomIP = (): string => {
  return `192.168.${randomBetween(1, 254)}.${randomBetween(1, 254)}`;
};

// Generate a random MAC address
const randomMAC = (): string => {
  return Array(6).fill(0).map(() => {
    const part = randomBetween(0, 255).toString(16);
    return part.length === 1 ? `0${part}` : part;
  }).join(':');
};

// Store connected clients
const clients = new Map();

// Define types for our mock data
interface MockNode {
  id: string;
  name: string;
  status: string;
  cpu: { usage: number; cores: number; };
  memory: { used: number; total: number; };
  uptime?: number;
  isClusterEntryPoint?: boolean;
}

interface MockVM {
  id: string | number;
  name: string;
  type: string;
  status: string;
  cpu: number | { usage: number; cores: number; };
  memory: any; // Can be a number or an object
  disk: { used: number; total: number; };
  shared?: boolean;
  primaryNode?: string;
  node?: string;
  nodeId?: string;
  nodeName?: string;
  uptime?: number;
  hastate?: string; // Add hastate field for HA status
}

interface MockMetric {
  guestId: string | number;
  timestamp: number;
  metrics: {
    cpu: number;
    memory: {
      total: number;
      used: number;
      percentUsed: number;
    };
    disk: {
      total: number;
      used: number;
      percentUsed: number;
    };
    network: {
      inRate: number;
      outRate: number;
      history?: Array<{
        in: number;
        out: number;
      }>;
    };
  };
  history?: {
    cpu: number[];
    memory: number[];
    disk: number[];
  };
}

// Store node data
const nodes = new Map<string, MockNode>();

// Store guest data - use a nested map structure: Map<nodeId, Array<guest>>
const guests = new Map<string, MockVM[]>();

// Initialize resources (nodes and guests)
const initResources = () => {
  logger.info('Initializing resources...');
  
  const customNodes = customMockData.nodes;
  
  for (const customNode of customNodes) {
    // Add node
    const node: MockNode = {
      id: customNode.id,
      name: customNode.name,
      status: customNode.status,
      cpu: customNode.cpu,
      memory: customNode.memory,
      uptime: randomBetween(60 * 60 * 24 * 7, 60 * 60 * 24 * 120) // Between 1 week and 4 months
    };
    
    nodes.set(node.id, node);
    
    // Add guests with consistent node references
    const nodeGuests: MockVM[] = [];
    
    // Use the node name for all node references (critical fix)
    const nodeName = node.name;
    
    // Process guests with consistent node references
    customNode.guests.forEach(customGuest => {
      // IMPORTANT: For each guest in each node, create a COMPLETELY NEW OBJECT
      // This ensures that shared guests (with the same ID) are actually different objects
      // and don't get lost in deduplication
      const guest: MockVM = {
        id: customGuest.id,
        name: customGuest.name,
        type: customGuest.type,
        status: customGuest.status,
        cpu: customGuest.cpu,
        memory: customGuest.memory,
        disk: customGuest.disk,
        shared: customGuest.shared,
        primaryNode: customGuest.primaryNode,
        // CRITICAL: Set all node references to the node name consistently
        node: nodeName,
        nodeId: nodeName,
        nodeName: nodeName,
        uptime: customGuest.status === 'running' ? randomBetween(3600, 2592000) : 0, // Between 1 hour and 30 days if running
        hastate: customGuest.status === 'running' ? 'started' : 'stopped' // Set hastate based on guest status
      };
      
      nodeGuests.push(guest);
    });
    
    guests.set(node.id, nodeGuests);
    
    logger.info(`Added node ${node.name} (${node.id}) with ${nodeGuests.length} guests`);
    
    // Log the guest IDs to make sure we have the right guests on each node
    const guestIds = nodeGuests.map(g => g.id).join(', ');
    logger.info(`Node ${node.name} guests: ${guestIds}`);
  }
  
  // Log the total guests for each node
  for (const [nodeId, nodeGuests] of guests.entries()) {
    const nodeName = Array.from(nodes.values()).find(n => n.id === nodeId)?.name || nodeId;
    logger.info(`Node ${nodeName} has ${nodeGuests.length} guests after initialization`);
  }
  
  logger.info(`Initialized ${nodes.size} nodes and ${Array.from(guests.values()).flat().length} guests`);
};

// Call this function during server initialization
initResources();

// Store metrics data
const metrics = new Map();

// Add a new map to track the primary node for each guest in cluster mode
const primaryNodeForGuest = new Map<string, string>();

// Initialize primary nodes for the guests
const initializePrimaryNodes = () => {
  // Clear the existing map to start fresh
  primaryNodeForGuest.clear();
  
  // Collection of all guests by node - for building shared guest mapping
  const guestNodeMap = new Map<string, string[]>();
  
  // For each unique guest ID, identify which nodes it exists on
  for (const [nodeId, nodeGuests] of guests.entries()) {
    for (const guest of nodeGuests) {
      const guestId = guest.id.toString();
      if (!guestNodeMap.has(guestId)) {
        guestNodeMap.set(guestId, []);
      }
      guestNodeMap.get(guestId)?.push(nodeId);
    }
  }
  
  // Log shared guests
  const sharedGuests = Array.from(guestNodeMap.entries())
    .filter(([_, nodes]) => nodes.length > 1);
  
  logger.info(`Found ${sharedGuests.length} shared guests across multiple nodes`);
  sharedGuests.forEach(([guestId, nodes]) => {
    logger.info(`Guest ${guestId} exists on nodes: ${nodes.join(', ')}`);
  });
  
  // For each guest, assign a primary node
  for (const [guestId, nodes] of guestNodeMap.entries()) {
    // TESTING: Mark every third guest as shared regardless of node count
    // This helps test the UI with more shared guests
    const guestIdNumber = parseInt(guestId, 10);
    const forceShared = guestIdNumber % 3 === 0;
    
    // If it exists on multiple nodes (or we're forcing it for testing), mark as shared
    if (nodes.length > 1 || forceShared) {
      // Sort nodes alphabetically for consistent testing
      const sortedNodes = [...nodes].sort();
      
      // Choose the first node as primary (alphabetically)
      const primaryNodeId = sortedNodes[0];
      primaryNodeForGuest.set(guestId, primaryNodeId);
      
      logger.info(`Assigned primary node ${primaryNodeId} for shared guest ${guestId}`);
      
      // Mark this guest as shared on all nodes
      for (const nodeId of nodes) {
        const nodeGuests = guests.get(nodeId);
        if (nodeGuests) {
          const guestIndex = nodeGuests.findIndex(g => g.id.toString() === guestId);
          if (guestIndex !== -1) {
            nodeGuests[guestIndex].shared = true;
          }
        }
      }
    } else {
      // For non-shared guests, still assign a primary node (same as the only node it's on)
      const primaryNodeId = nodes[0];
      primaryNodeForGuest.set(guestId, primaryNodeId);
    }
  }
  
  // Now set hastate and guest status for each guest on each node
  for (const [nodeId, nodeGuests] of guests.entries()) {
    for (const guest of nodeGuests) {
      const guestId = guest.id.toString();
  const primaryNodeId = primaryNodeForGuest.get(guestId);
      
      // Update status based on whether this is the primary node
      const isPrimary = nodeId === primaryNodeId;
      
      // NEVER set hastate to IGNORED, always use a meaningful state
      // Set baseline state - 70% started, 10% error, 10% migrate, 5% recovery, 5% disabled
      const randomValue = Math.random();
      let hastate;
      
      if (randomValue < 0.7) {
        hastate = HA_STATES.STARTED;
      } else if (randomValue < 0.8) {
        hastate = HA_STATES.ERROR;
      } else if (randomValue < 0.9) {
        hastate = HA_STATES.MIGRATE;
      } else if (randomValue < 0.95) {
        hastate = HA_STATES.RECOVERY;
      } else {
        hastate = HA_STATES.DISABLED;
      }
      
      // Set the hastate and appropriate status based on the state
      if (guest.shared) {
        if (isPrimary) {
          guest.hastate = hastate;
          
          // Set corresponding status based on hastate
          if (hastate === HA_STATES.MIGRATE || hastate === HA_STATES.RELOCATE) {
            guest.status = 'migrating';
          } else if (hastate === HA_STATES.ERROR || hastate === HA_STATES.FENCE) {
            guest.status = 'error';
          } else if (hastate === HA_STATES.STOPPED || hastate === HA_STATES.DISABLED) {
            guest.status = 'stopped'; 
          } else {
            guest.status = 'running';
          }
          
          logger.info(`Set shared guest ${guest.id} on primary node ${nodeId} to ${guest.hastate}`);
        } else {
          // For non-primary nodes, set to stopped
          guest.hastate = HA_STATES.STOPPED;
          guest.status = 'stopped';
          logger.info(`Set shared guest ${guest.id} on secondary node ${nodeId} to ${guest.hastate}`);
        }
      } else {
        // For non-shared guests, set hastate directly
        guest.hastate = hastate;
        
        // Set status based on hastate
        if (hastate === HA_STATES.MIGRATE || hastate === HA_STATES.RELOCATE) {
          guest.status = 'migrating';
        } else if (hastate === HA_STATES.ERROR || hastate === HA_STATES.FENCE) {
          guest.status = 'error';
        } else if (hastate === HA_STATES.STOPPED || hastate === HA_STATES.DISABLED) {
          guest.status = 'stopped';
        } else {
          guest.status = 'running';
        }
        
        logger.info(`Set non-shared guest ${guest.id} on node ${nodeId} to ${guest.hastate}`);
      }
    }
  }
  
  logger.info('Primary node initialization complete');
};

// Call this function during server initialization
initializePrimaryNodes();

// Initialize the server
// ... existing code ...

// Force an immediate run of migrations to see some state changes
setTimeout(() => {
  logger.info('Running initial migrations for visual testing');
  // Perform several migrations to ensure we see state changes
  for (let i = 0; i < 5; i++) {
    performRandomMigration();
  }
  
  // Then set up the regular schedule
  scheduleMigrations();
}, 3000); // wait 3 seconds after server start

// Add a filter to resources endpoint to only return the entry point node in cluster mode
app.get('/api2/json/cluster/resources', (req, res) => {
  const clusterMode = isClusterModeEnabled();
  
  // Get all guests - flatten the nested map structure
  const allGuests: MockVM[] = [];
  for (const nodeGuests of guests.values()) {
    // Make sure we're getting a COPY of the guests, not a reference
    // This ensures each node's guests maintain their node association
    nodeGuests.forEach(guest => {
      allGuests.push({...guest});
    });
  }
  
  // Log what we're about to return
  logger.info(`Cluster resources endpoint called. Type: ${req.query.type}, Total guests: ${allGuests.length}`);
  
  // DEBUG: Log hastate values for each guest
  allGuests.forEach(guest => {
    logger.info(`Guest ${guest.id} hastate: ${guest.hastate || 'undefined'}`);
  });
  
  if (clusterMode) {
    logger.info('Returning cluster resources with preserved node associations');
    
    // We need to return all nodes in the response, but with a flag indicating which one is the entry point
    const allNodes = Array.from(nodes.values());
    if (allNodes.length === 0) {
      logger.warn('No nodes found! Returning empty response');
      res.json({ data: [] });
      return;
    }
    
    // Create an entry point node but keep all other nodes as well
    const entryPointNode = allNodes[0];
    const clusterEntryPoint = {
      ...entryPointNode,
      name: 'pve-cluster-01', // Rename entry point for clarity
      id: entryPointNode.id,
      isClusterEntryPoint: true
    };
    
    // Replace the first node with our entry point node
    const formattedNodes = [
      clusterEntryPoint,
      ...allNodes.slice(1)
    ];
    
    // Filter by requested type if specified
    const requestedType = req.query.type as string;
    let filteredGuests = allGuests;
    
    if (requestedType) {
      logger.info(`Filtering by type: ${requestedType}`);
      filteredGuests = allGuests.filter(guest => {
        if (requestedType === 'vm' || requestedType === 'qemu') {
          return guest.type === 'qemu' || guest.type === 'vm';
        } else if (requestedType === 'lxc') {
          return guest.type === 'lxc' || guest.type === 'ct';
        }
        return true;
      });
    }
    
    logger.info(`Returning ${filteredGuests.length} guests for type ${requestedType || 'all'}`);
    
    // Format response with all nodes and all guests, preserving original node associations
    const formattedResources = [
      // Include all nodes (with the first one being the cluster entry point)
      ...formattedNodes.map(node => ({
        type: 'node',
        node: node.name,
        name: node.name,
        status: 'online',
        id: node.name,
        cpu: 0.1,
        maxcpu: 8,
        mem: 4 * 1024 * 1024 * 1024,
        maxmem: 16 * 1024 * 1024 * 1024,
        disk: 100 * 1024 * 1024 * 1024,
        maxdisk: 500 * 1024 * 1024 * 1024,
        uptime: 3600 * 24,
        isClusterEntryPoint: node.isClusterEntryPoint || false
      })),
      // Include all guests with their original node associations
      ...filteredGuests.map(guest => {
        // Get the node name from the node ID mapping
        const nodeObj = Array.from(nodes.values()).find(n => n.id === (guest.nodeId || guest.node));
        const nodeName = nodeObj ? nodeObj.name : (guest.nodeName || 'unknown');
        
        return {
          type: guest.type === 'lxc' || guest.type === 'ct' ? 'lxc' : 'qemu',
          node: nodeName,
          name: guest.name,
          status: guest.status,
          vmid: typeof guest.id === 'number' ? guest.id : parseInt(String(guest.id).replace(/\D/g, '')) || parseInt(String(guest.id)),
          id: guest.id.toString(), // Convert to string to maintain compatibility with the rest of the code
          cpu: guest.status === 'running' ? Math.random() * 0.5 : 0,
          maxcpu: typeof guest.cpu === 'object' && guest.cpu && 'cores' in guest.cpu ? guest.cpu.cores : 1,
          mem: guest.status === 'running' ? (typeof guest.memory === 'object' ? guest.memory.used : guest.memory * 0.6) : 0,
          maxmem: typeof guest.memory === 'object' ? guest.memory.total : guest.memory,
          disk: guest.disk?.used || 10 * 1024 * 1024 * 1024 * 0.5,
          maxdisk: guest.disk?.total || 10 * 1024 * 1024 * 1024,
          uptime: guest.status === 'running' ? 3600 * 12 : 0,
          hastate: guest.hastate || (
            // If hastate is not explicitly set, derive it from status
            guest.status === 'running' ? HA_STATES.STARTED : 
            guest.status === 'error' ? HA_STATES.ERROR :
            guest.status === 'migrating' ? HA_STATES.MIGRATE :
            HA_STATES.STOPPED
          )
        };
      })
    ];
    
    res.json({ data: formattedResources });
    return;
  }
  
  // Normal non-cluster mode - return all resources
  logger.info('Returning all cluster resources (non-cluster mode)');
  
  // Get all nodes and guests
  const allNodes = Array.from(nodes.values());
  
  // Filter by requested type if specified
  const requestedType = req.query.type as string;
  let filteredGuests = allGuests;
  
  if (requestedType) {
    logger.info(`Filtering by type: ${requestedType}`);
    filteredGuests = allGuests.filter(guest => {
      if (requestedType === 'vm' || requestedType === 'qemu') {
        return guest.type === 'qemu' || guest.type === 'vm';
      } else if (requestedType === 'lxc') {
        return guest.type === 'lxc' || guest.type === 'ct';
      }
      return true;
    });
  }
  
  logger.info(`Returning ${filteredGuests.length} guests for type ${requestedType || 'all'}`);
  
  // Format the response for non-cluster mode
  const formattedResources = [
    // Include all nodes
    ...allNodes.map(node => ({
      type: 'node',
      node: node.name,
      name: node.name,
      status: 'online',
      id: node.name,  // Use name as ID for consistency
      cpu: 0.1,
      maxcpu: 8,
      mem: 4 * 1024 * 1024 * 1024,
      maxmem: 16 * 1024 * 1024 * 1024,
      disk: 100 * 1024 * 1024 * 1024,
      maxdisk: 500 * 1024 * 1024 * 1024,
      uptime: 3600 * 24
    })),
    // Include all guests
    ...filteredGuests.map(guest => {
      // Get the node name from the node ID mapping
      const nodeObj = Array.from(nodes.values()).find(n => n.id === (guest.nodeId || guest.node));
      const nodeName = nodeObj ? nodeObj.name : (guest.nodeName || 'unknown');
      
      return {
        type: guest.type === 'lxc' || guest.type === 'ct' ? 'lxc' : 'qemu',
        node: nodeName,
        name: guest.name,
        status: guest.status,
        vmid: typeof guest.id === 'number' ? guest.id : parseInt(String(guest.id).replace(/\D/g, '')) || parseInt(String(guest.id)),
        id: guest.id.toString(), // Convert to string to maintain compatibility with the rest of the code
        cpu: guest.status === 'running' ? Math.random() * 0.5 : 0,
        maxcpu: typeof guest.cpu === 'object' && guest.cpu && 'cores' in guest.cpu ? guest.cpu.cores : 1,
        mem: guest.status === 'running' ? (typeof guest.memory === 'object' ? guest.memory.used : guest.memory * 0.6) : 0,
        maxmem: typeof guest.memory === 'object' ? guest.memory.total : guest.memory,
        disk: guest.disk?.used || 10 * 1024 * 1024 * 1024 * 0.5,
        maxdisk: guest.disk?.total || 10 * 1024 * 1024 * 1024,
        uptime: guest.status === 'running' ? 3600 * 24 : 0,
        hastate: guest.hastate || (
          // If hastate is not explicitly set, derive it from status
          guest.status === 'running' ? HA_STATES.STARTED : 
          guest.status === 'error' ? HA_STATES.ERROR :
          guest.status === 'migrating' ? HA_STATES.MIGRATE :
          HA_STATES.STOPPED
        )
      };
    })
  ];
  
  res.json({ data: formattedResources });
});

// Add cluster status endpoint to accurately reflect cluster configuration
app.get('/api2/json/cluster/status', (req, res) => {
  // Check all possible ways cluster mode could be enabled
  const clusterMode = process.env.PROXMOX_CLUSTER_MODE === 'true' || 
                     process.env.MOCK_CLUSTER_ENABLED === 'true' || 
                     process.env.MOCK_CLUSTER_MODE === 'true' ||
                     (process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true' && 
                      (process.env.USE_MOCK_DATA === 'true' || 
                       process.env.MOCK_DATA_ENABLED === 'true'));
  
  if (clusterMode) {
    // Cluster is enabled - return proper cluster status
    logger.info('Returning mock cluster status (enabled)');
    res.json({
      data: [
        {
          type: 'cluster',
          name: 'mock-cluster',
          version: 1,
          nodes: Array.from(nodes.values()).map(node => node.name),
          quorate: 1,
          id: 'mock-cluster'
        },
        ...Array.from(nodes.values()).map(node => ({
          type: 'node',
          name: node.name,
          id: node.id,
          ip: `192.168.0.${10 + parseInt(node.id.replace('node-', ''), 10)}`,
          online: node.status === 'online' ? 1 : 0
        }))
      ]
    });
  } else {
    // Cluster is not enabled - return empty response
    logger.info('Returning mock cluster status (disabled)');
    res.json({
      data: []
    });
  }
});

// Add an endpoint to let clients know if they should use cluster mode
app.get('/api/cluster-status', (req, res) => {
  const clusterMode = isClusterModeEnabled();
  
  res.json({
    clusterEnabled: clusterMode,
    clusterName: 'mock-cluster',
    entryPointNode: clusterMode ? Array.from(getFilteredNodes().values())[0] : null
  });
});

/**
 * IMPORTANT: Guest generation has been consolidated.
 * All mock guests come from customMockData in custom-data.ts.
 * Do not add guests programmatically to avoid duplicates.
 * Each node has a maximum of 10 guests to prevent UI overload.
 */

// Generate metrics for guests
const generateMetrics = (nodeId: string, nodeGuests: MockVM[]): MockMetric[] => {
  const metrics: MockMetric[] = [];
  
  // Log the number of guests passed to generateMetrics
  logger.debug(`Generating metrics for node ${nodeId} with ${nodeGuests.length} guests`);
  
  // Process ALL guests, not just running ones
  nodeGuests.forEach((guest: MockVM) => {
    // Generate metrics for all guests, but with special handling for non-running guests
    const isRunningAndPrimary = guest.status === 'running' && isNodePrimaryForGuest(nodeId, guest.id.toString());
    
    logger.debug(`Generating metrics for guest ${guest.id} on node ${nodeId} (running: ${guest.status === 'running'}, primary: ${isNodePrimaryForGuest(nodeId, guest.id.toString())})`);
    
    // Set appropriate values based on guest status
    const cpuUsage = isRunningAndPrimary ? 
      (typeof guest.cpu === 'number' ? guest.cpu * 100 : (guest.cpu && 'usage' in guest.cpu ? guest.cpu.usage * 100 : 0)) : 
      0;
    
    // Handle different memory structures
    const memoryTotal = typeof guest.memory === 'number' ? guest.memory : guest.memory.total;
    const memoryUsed = isRunningAndPrimary ? 
      (typeof guest.memory === 'number' ? Math.floor(guest.memory * 0.7) : guest.memory.used) : 
      0;
    
    // Handle disk - provide basic values even for stopped guests
    const diskTotal = guest.disk?.total || 1073741824; // 1GB default
    const diskUsed = guest.disk?.used || 536870912; // 512MB default
    const diskPercentUsed = (diskUsed / diskTotal) * 100;
    
    // Generate network metrics - zero for non-running guests
    // Use realistic values for data center environments with various workloads
    const guestIdStr = String(guest.id);
    const lastDigit = parseInt(guestIdStr.slice(-1));
    const isHighTrafficVM = [1, 5, 9].includes(lastDigit); // ~30% are high traffic
    const isMediumTrafficVM = [2, 3, 7].includes(lastDigit); // ~30% are medium traffic
    // All others are low traffic (~40%)
    
    let networkIn, networkOut;
    
    if (isHighTrafficVM) {
      // High-traffic VMs: database servers, file servers, streaming servers, etc.
      networkIn = isRunningAndPrimary ? randomFloatBetween(500, 2000) : 0; // 500KB/s-2MB/s baseline
      networkOut = isRunningAndPrimary ? randomFloatBetween(200, 1000) : 0; // 200KB/s-1MB/s baseline
    } else if (isMediumTrafficVM) {
      // Medium traffic: application servers, web servers, etc.
      networkIn = isRunningAndPrimary ? randomFloatBetween(100, 500) : 0;  // 100-500 KB/s baseline
      networkOut = isRunningAndPrimary ? randomFloatBetween(50, 200) : 0; // 50-200 KB/s baseline
    } else {
      // Lower traffic: utility servers, monitoring, etc.
      networkIn = isRunningAndPrimary ? randomFloatBetween(20, 100) : 0; // 20-100 KB/s baseline
      networkOut = isRunningAndPrimary ? randomFloatBetween(10, 50) : 0; // 10-50 KB/s baseline
    }
    
    metrics.push({
      guestId: guest.id.toString(),
      timestamp: Date.now(),
      metrics: {
        cpu: cpuUsage,
        memory: {
          total: memoryTotal,
          used: memoryUsed,
          percentUsed: (memoryUsed / memoryTotal) * 100
        },
        disk: {
          total: diskTotal,
          used: diskUsed,
          percentUsed: diskPercentUsed
        },
        network: {
          inRate: networkIn,
          outRate: networkOut,
          history: Array(10).fill(0).map(() => ({
            in: isRunningAndPrimary ? randomBetween(Math.max(0, networkIn * 0.8), networkIn * 1.2) : 0,
            out: isRunningAndPrimary ? randomBetween(Math.max(0, networkOut * 0.8), networkOut * 1.2) : 0
          }))
        }
      },
      history: {
        cpu: Array(10).fill(0).map(() => 
          isRunningAndPrimary ? randomFloatBetween(Math.max(0, cpuUsage - 20), Math.min(100, cpuUsage + 20)) : 0
        ),
        memory: Array(10).fill(0).map(() => 
          isRunningAndPrimary ? randomFloatBetween(Math.max(0, memoryUsed - 15), Math.min(100, memoryUsed + 15)) : 0
        ),
        disk: Array(10).fill(0).map(() => 
          isRunningAndPrimary ? randomFloatBetween(Math.max(0, diskUsed - 5), Math.min(100, diskUsed + 5)) : diskPercentUsed
        )
      }
    });
  });
  
  return metrics;
};

// Send initial data to the client
const sendInitialData = (socket: Socket) => {
  logger.info(`Sending initial data to client ${socket.id}`);
  
  // Convert nodes map to array
  const nodeArray = Array.from(nodes.values());
  
  // Send node data
  socket.emit('nodes', { nodes: nodeArray });
  
  // Get client info
  const clientInfo = clients.get(socket.id);
  
  if (clientInfo?.nodeId) {
    // Client has registered for a specific node
    const nodeId = clientInfo.nodeId;
    logger.info(`Client ${socket.id} is registered for node ${nodeId}`);
    
    // Get the node's guests - these already include shared guests from our distribution
    const nodeGuests = guests.get(nodeId) || [];
    logger.info(`Node ${nodeId} has ${nodeGuests.length} guests (including shared guests)`);
    
    // Send ALL guests for this node - already properly set up from our distribution
    socket.emit('guests', { guests: nodeGuests });
    
    // Generate and send metrics for ALL guests
    const nodeMetrics = generateMetrics(nodeId, nodeGuests);
    metrics.set(nodeId, nodeMetrics);
    socket.emit('metrics', { metrics: nodeMetrics });
    
    logger.info(`Sent initial data for node ${nodeId}: ${nodeGuests.length} guests, ${nodeMetrics.length} metrics`);
  } else {
    // Check if this client is a dashboard client (special case)
    const isDashboard = clientInfo?.clientType === 'dashboard';
    
    if (isDashboard) {
      // For dashboard clients, send all guests with node information
      // This is necessary for the dashboard to show the correct guest counts
      
      // Collect all guests from all nodes WITHOUT deduplication
      const allGuests: MockVM[] = [];
      
      // Process each node and add its guests to the array
      for (const [nodeId, nodeGuests] of guests.entries()) {
        // Get the node name for this node
        const nodeName = nodeArray.find(n => n.id === nodeId)?.name || nodeId;
        
        // Process each guest to ensure it has proper node references
        const processedGuests = nodeGuests.map((guest: MockVM) => ({
          ...guest,
          // Set node/nodeId/nodeName to the ACTUAL node this guest is on
          node: nodeName,
          nodeId: nodeName,
          nodeName: nodeName
        }));
        
        // Add ALL guests from this node without checking for duplicates
        allGuests.push(...processedGuests);
      }
      
      // Send all guests to the dashboard client
      socket.emit('guests', { guests: allGuests });
      
      // Log distribution of guests by node for debugging
      const guestsByNode = new Map<string, number>();
      allGuests.forEach((guest: MockVM) => {
        const nodeName = guest.node || '';
        if (!guestsByNode.has(nodeName)) {
          guestsByNode.set(nodeName, 0);
        }
        guestsByNode.set(nodeName, guestsByNode.get(nodeName)! + 1);
      });
      
      logger.info(`Guest distribution in initial data:`);
      guestsByNode.forEach((count, node) => {
        logger.info(`  Node ${node}: ${count} guests`);
      });
      
      logger.info(`Sent all guests (${allGuests.length}) to dashboard client`);
    } else {
      // For other unregistered clients, send an empty array
      socket.emit('guests', { guests: [] });
      logger.info(`Sent empty guest list to unregistered client`);
    }
  }
};

/**
 * Get the node ID associated with a socket
 */
const getNodeIdFromSocket = (socket: Socket): string | undefined => {
  const clientInfo = clients.get(socket.id);
  return clientInfo?.nodeId;
};

// Function to update metrics for a socket
const updateMetrics = (socket: Socket) => {
  // Get the node ID from the socket
  const nodeId = getNodeIdFromSocket(socket);
  if (!nodeId) {
    return;
  }
  
  // Get the guests for this node
  const nodeGuests = guests.get(nodeId) || [];
  if (nodeGuests.length === 0) {
    return;
  }
  
  // Get the current metrics for this node
  const currentMetrics = metrics.get(nodeId) || [];
  if (currentMetrics.length === 0) {
    return;
  }
  
  // Update metrics for ALL guests, not just running ones
  // This ensures a consistent list of guests in metrics
  const updatedMetrics = currentMetrics.map((metric: MockMetric) => {
    const guest = nodeGuests.find((g: MockVM) => g.id.toString() === metric.guestId);
    if (!guest) {
      return metric; // Keep existing metric if guest not found
    }
    
    const isRunningAndPrimary = guest.status === 'running' && isNodePrimaryForGuest(nodeId, guest.id.toString());
    
    // For non-running guests, keep metrics at zero or static values
    if (!isRunningAndPrimary) {
      return {
        ...metric,
        timestamp: Date.now(),
        metrics: {
          ...metric.metrics,
          cpu: 0,
          memory: {
            ...metric.metrics.memory,
            used: 0,
            percentUsed: 0
          },
          network: {
            ...metric.metrics.network,
            inRate: 0,
            outRate: 0,
            history: Array(10).fill(0).map(() => ({ in: 0, out: 0 }))
          }
        },
        history: {
          ...(metric.history || {}),
          cpu: metric.history?.cpu?.map(() => 0) || [],
          memory: metric.history?.memory?.map(() => 0) || [],
          // Keep disk history as is - disk doesn't change when VM is off
        }
      };
    }
    
    // For primary node running guests, continue with normal metric updates
    // Update CPU usage with more dynamic variations
    const currentCpu = metric.metrics.cpu;
    
    // Create more realistic CPU patterns - sometimes spikes, sometimes gradual changes
    let cpuDelta;
    if (Math.random() < 0.1) {
      // 10% chance of a significant spike or drop - reduce magnitude from ±15 to ±8
      cpuDelta = randomFloatBetween(-8, 8);
    } else if (Math.random() < 0.3) {
      // 30% chance of a moderate change - reduce magnitude from ±8 to ±5
      cpuDelta = randomFloatBetween(-5, 5);
    } else {
      // 60% chance of a small change - reduce magnitude from ±3 to ±2
      cpuDelta = randomFloatBetween(-2, 2);
    }
    
    // Apply trend bias - if CPU is high, more likely to go down, if low, more likely to go up
    if (currentCpu > 70) {
      cpuDelta -= 1.5; // Bias towards decreasing when high (increased from -1 to -1.5)
    } else if (currentCpu < 20) {
      cpuDelta += 1.5; // Bias towards increasing when low (increased from +1 to +1.5)
    }
    
    // During stress tests, CPU should stay high with smaller fluctuations
    // This simulates a VM under constant load better
    if (currentCpu > 80) {
      // If CPU is already very high (likely stress test), keep it high with smaller variations
      cpuDelta = randomFloatBetween(-3, 1); // More likely to stay high
    }
    
    const newCpu = Math.max(1, Math.min(100, currentCpu + cpuDelta));
    
    // Update memory usage with more realistic variations
    const currentMemoryPercent = metric.metrics.memory.percentUsed;
    
    // Memory tends to change more gradually than CPU
    let memoryDelta;
    if (Math.random() < 0.05) {
      // 5% chance of a larger memory change (application started/stopped)
      memoryDelta = randomFloatBetween(-8, 8);
    } else if (Math.random() < 0.2) {
      // 20% chance of a moderate change
      memoryDelta = randomFloatBetween(-4, 4);
    } else {
      // 75% chance of a small change
      memoryDelta = randomFloatBetween(-2, 2);
    }
    
    // Memory often correlates with CPU changes
    if (cpuDelta > 5) {
      memoryDelta += 1; // If CPU spiked up, memory likely increases too
    } else if (cpuDelta < -5) {
      memoryDelta -= 0.5; // If CPU dropped significantly, memory might decrease too
    }
    
    const newMemoryPercent = Math.max(5, Math.min(95, currentMemoryPercent + memoryDelta));
    
    // Check if memory is a number or an object
    const memoryTotal = typeof guest.memory === 'number' ? guest.memory : guest.memory.total;
    const newMemoryUsed = (memoryTotal * newMemoryPercent) / 100;
    
    // Update disk usage (disk changes are typically slower than CPU/memory)
    const diskTotal = guest.disk?.total || 1073741824; // 1GB default
    const diskUsed = guest.disk?.used || 536870912; // 512MB default
    const currentDiskPercent = diskUsed / diskTotal * 100;
    
    // Disk usage typically increases slowly over time with occasional drops (cleanup)
    let diskDelta;
    if (Math.random() < 0.02) {
      // 2% chance of disk cleanup (significant drop)
      diskDelta = randomFloatBetween(-5, -1);
    } else if (Math.random() < 0.1) {
      // 10% chance of a larger increase (file download, log growth)
      diskDelta = randomFloatBetween(0.5, 2);
    } else {
      // 88% chance of a very small increase
      diskDelta = randomFloatBetween(0, 0.5);
    }
    
    const newDiskPercent = Math.max(10, Math.min(90, currentDiskPercent + diskDelta));
    const newDiskUsed = (diskTotal * newDiskPercent) / 100;
    
    // Base rates that traffic should regress toward (in KB/s) - by VM type
    const getBaselineRates = (guestId: string | number): { inRate: number; outRate: number } => {
      const guestIdStr = String(guestId);
      const lastDigit = parseInt(guestIdStr.slice(-1));
      const isHighTrafficVM = [1, 5, 9].includes(lastDigit); // ~30% are high traffic
      const isMediumTrafficVM = [2, 3, 7].includes(lastDigit); // ~30% are medium traffic
      
      if (isHighTrafficVM) {
        return { inRate: 500, outRate: 200 }; // 500 KB/s in, 200 KB/s out baseline
      } else if (isMediumTrafficVM) {
        return { inRate: 100, outRate: 50 }; // 100 KB/s in, 50 KB/s out baseline
      } else {
        return { inRate: 20, outRate: 10 }; // 20 KB/s in, 10 KB/s out baseline
      }
    };
    
    // Get max rates by VM type (in KB/s)
    const getMaxRates = (guestId: string | number): { inRate: number; outRate: number } => {
      const guestIdStr = String(guestId);
      const lastDigit = parseInt(guestIdStr.slice(-1));
      const isHighTrafficVM = [1, 5, 9].includes(lastDigit);
      const isMediumTrafficVM = [2, 3, 7].includes(lastDigit);
      
      // Base max rates (in KB/s)
      const baseMaxIn = 5000; // 5 MB/s (~40 Mbps)
      const baseMaxOut = 3000; // 3 MB/s (~24 Mbps)
      
      if (isHighTrafficVM) {
        return { inRate: baseMaxIn, outRate: baseMaxOut };
      } else if (isMediumTrafficVM) {
        return { inRate: baseMaxIn * 0.5, outRate: baseMaxOut * 0.5 };
      } else {
        return { inRate: baseMaxIn * 0.2, outRate: baseMaxOut * 0.2 };
      }
    };
    
    // Update network usage with more realistic variations
    const currentNetworkIn = metric.metrics.network.inRate;
    const currentNetworkOut = metric.metrics.network.outRate;
    
    // Get baseline and max rates for this specific guest
    const baselineRates = getBaselineRates(metric.guestId);
    const maxRates = getMaxRates(metric.guestId);
    
    // Check VM type for probability calculations
    const guestIdStr = String(metric.guestId);
    const lastDigit = parseInt(guestIdStr.slice(-1));
    const isHighTrafficVM = [1, 5, 9].includes(lastDigit);
    const isMediumTrafficVM = [2, 3, 7].includes(lastDigit);
    
    // Calculate probabilities based on VM type
    let burstProbability, resetProbability;
    
    if (isHighTrafficVM) {
      burstProbability = 0.15; // 15% chance of burst
      resetProbability = 0.01; // 1% chance of reset
    } else if (isMediumTrafficVM) {
      burstProbability = 0.07; // 7% chance of burst
      resetProbability = 0.05; // 5% chance of reset
    } else {
      burstProbability = 0.02; // 2% chance of burst
      resetProbability = 0.2; // 20% chance of reset
    }
    
    // Variables for new rates
    let newNetworkIn, newNetworkOut;
    
    // Periodically reset to baseline to prevent continuous growth
    const forceReset = Math.random() < resetProbability;
    
    if (forceReset) {
      // Reset to baseline with a small amount of randomness
      newNetworkIn = (baselineRates.inRate + randomFloatBetween(0, 1)) * 1024; // KB/s to B/s
      newNetworkOut = (baselineRates.outRate + randomFloatBetween(0, 0.5)) * 1024; // KB/s to B/s
    } else if (Math.random() < burstProbability) {
      // Generate a traffic burst appropriate for the VM type
      let burstInSize, burstOutSize;
      
      if (isHighTrafficVM) {
        // Larger bursts for high-traffic VMs
        burstInSize = randomFloatBetween(800, 3000); // 800-3000 KB/s
        burstOutSize = randomFloatBetween(400, 1500); // 400-1500 KB/s
      } else if (isMediumTrafficVM) {
        // Moderate bursts for medium-traffic VMs
        burstInSize = randomFloatBetween(300, 1200); // 300-1200 KB/s
        burstOutSize = randomFloatBetween(150, 600); // 150-600 KB/s
      } else {
        // Small bursts for low-traffic VMs
        burstInSize = randomFloatBetween(100, 500); // 100-500 KB/s
        burstOutSize = randomFloatBetween(50, 250); // 50-250 KB/s
      }
      
      // Apply burst (convert KB/s to B/s) and stay within limits
      newNetworkIn = Math.min(maxRates.inRate * 1024, currentNetworkIn + (burstInSize * 1024));
      newNetworkOut = Math.min(maxRates.outRate * 1024, currentNetworkOut + (burstOutSize * 1024));
    } else {
      // Normal decay toward baseline
      const inRateDistanceFromBaseline = Math.max(0, currentNetworkIn - (baselineRates.inRate * 1024));
      const outRateDistanceFromBaseline = Math.max(0, currentNetworkOut - (baselineRates.outRate * 1024));
      
      // Strong decay rate (70-90%)
      const inDecayRate = 0.7 + (inRateDistanceFromBaseline / (maxRates.inRate * 1024 * 2)) * 0.2;
      const outDecayRate = 0.7 + (outRateDistanceFromBaseline / (maxRates.outRate * 1024 * 2)) * 0.2;
      
      // Calculate decay amount
      const inDecayAmount = inRateDistanceFromBaseline * Math.min(0.9, inDecayRate);
      const outDecayAmount = outRateDistanceFromBaseline * Math.min(0.9, outDecayRate);
      
      // Small random fluctuation (convert KB/s to B/s)
      const randomFluctuation = randomFloatBetween(-1, 0.5) * 1024;
      
      // Apply decay with fluctuation
      newNetworkIn = Math.max(
        baselineRates.inRate * 1024 / 2, // Ensure at least half baseline
        currentNetworkIn - inDecayAmount + randomFluctuation
      );
      
      newNetworkOut = Math.max(
        baselineRates.outRate * 1024 / 2, // Ensure at least half baseline
        currentNetworkOut - outDecayAmount + (randomFluctuation / 2)
      );
    }
    
    // Final safety caps
    newNetworkIn = Math.min(maxRates.inRate * 1024, Math.max(1024, newNetworkIn));
    newNetworkOut = Math.min(maxRates.outRate * 1024, Math.max(512, newNetworkOut));
    
    // Update network history
    const networkHistory = [...(metric.metrics.network.history?.slice(1) || []), { in: newNetworkIn, out: newNetworkOut }];
    
    // Update CPU history
    const cpuHistory = [...(metric.history?.cpu?.slice(1) || []), newCpu];
    
    // Update memory history
    const memoryHistory = [...(metric.history?.memory?.slice(1) || []), newMemoryPercent];
    
    // Update disk history (less frequent changes)
    const diskHistory = [...(metric.history?.disk?.slice(1) || []), newDiskPercent];
    
    // Return updated metric
    return {
      ...metric,
      timestamp: Date.now(),
      metrics: {
        ...metric.metrics,
        cpu: newCpu,
        memory: {
          ...metric.metrics.memory,
          used: newMemoryUsed,
          percentUsed: newMemoryPercent
        },
        disk: {
          ...metric.metrics.disk,
          used: newDiskUsed,
          percentUsed: newDiskPercent
        },
        network: {
          ...metric.metrics.network,
          inRate: newNetworkIn,
          outRate: newNetworkOut,
          history: networkHistory
        }
      },
      history: {
        ...(metric.history || {}),
        cpu: cpuHistory,
        memory: memoryHistory,
        disk: diskHistory
      }
    };
  });
  
  // Update metrics for this node
  metrics.set(nodeId, updatedMetrics);
  
  // Emit metrics update to the client
  socket.emit('metrics', { metrics: updatedMetrics });
};

// Socket.io connection handler
io.on('connection', (socket: Socket) => {
  // Track client connection
  clients.set(socket.id, { 
    socket,
    nodeId: null,
    nodeName: null,
    isClusterEntryPoint: false,
    isClusterMode: false
  });
  
  logger.info(`Client connected: ${socket.id}. Total clients: ${clients.size}`);
  
  // Handle registration
  socket.on('registerNode', (data) => {
    const { nodeId, nodeName, isClusterEntryPoint, isClusterMode } = data;
    logger.info(`Node registration: ${nodeName} (${nodeId}), cluster entry point: ${isClusterEntryPoint}, cluster mode: ${isClusterMode}`);
    
    // Store client details
    clients.set(socket.id, {
      socket,
      nodeId,
      nodeName,
      isClusterEntryPoint,
      isClusterMode
    });
    
    // Get all nodes data to ensure consistent mapping of IDs to names
    const allNodes = Array.from(nodes.values());
    
    // Build a mapping of node ID to node name for consistent references
    const nodeIdToName = new Map<string, string>();
    allNodes.forEach(node => {
      nodeIdToName.set(node.id, node.name);
    });
    
    // Process all guests to ensure consistent node names
    const allProcessedGuests: MockVM[] = [];
    
    for (const [nodeId, nodeGuests] of guests.entries()) {
      const nodeName = nodeIdToName.get(nodeId) || nodeId;
      
      // Process guests for this node, making sure they have consistent node names
      nodeGuests.forEach((guest: MockVM) => {
        // Create a copy of each guest with proper node references
        allProcessedGuests.push({
          ...guest,
          // Set node/nodeId/nodeName to the ACTUAL node this guest is on
          node: nodeName,
          nodeId: nodeName,
          nodeName: nodeName
        });
      });
    }
    
    // Send initial data to the client
    if (isClusterEntryPoint || isClusterMode) {
      // In cluster mode or for cluster entry points, send ALL guests
      logger.info(`Sending all guests to cluster entry point node ${nodeName} (total: ${allProcessedGuests.length})`);
      
      // Send all guests with consistent node names
      socket.emit('guests', { guests: allProcessedGuests });
      
      // Also send all nodes data
      socket.emit('nodes', { nodes: allNodes });
    } else {
      // In non-cluster mode, send only this node's guests
      logger.info(`Sending node-specific guests to node ${nodeName}`);
      
      // Filter guests for this specific node
      const nodeGuests = allProcessedGuests.filter(guest => 
        guest.node === nodeName || guest.nodeId === nodeName || guest.nodeName === nodeName
      );
      
      logger.info(`Node ${nodeName} has ${nodeGuests.length} filtered guests`);
      socket.emit('guests', { guests: nodeGuests });
      
      // Also send all nodes data
      socket.emit('nodes', { nodes: allNodes });
    }
  });
  
  // Backward compatibility for old 'register' event
  socket.on('register', (data) => {
    const { nodeId, nodeName: clientNodeName } = data;
    logger.info(`Legacy node registration: ${clientNodeName} (${nodeId})`);
    
    // Update client tracking
    clients.set(socket.id, {
      socket,
      nodeId,
      nodeName: clientNodeName,
      isClusterEntryPoint: false,
      isClusterMode: false
    });
    
    // Get all nodes data to ensure consistent mapping of IDs to names
    const allNodes = Array.from(nodes.values());
    
    // Build a mapping of node ID to node name for consistent references
    const nodeIdToName = new Map<string, string>();
    allNodes.forEach(node => {
      nodeIdToName.set(node.id, node.name);
    });
    
    // Process guests for this specific node to ensure consistent node names
    const mappedNodeName = nodeIdToName.get(nodeId) || nodeId;
    
    // Get all guests and process them for consistency
    const allProcessedGuests: MockVM[] = [];
    
    for (const [guestNodeId, nodeGuests] of guests.entries()) {
      const guestNodeName = nodeIdToName.get(guestNodeId) || guestNodeId;
      
      // Process guests for this node, making sure they have consistent node names
      nodeGuests.forEach((guest: MockVM) => {
        allProcessedGuests.push({
          ...guest,
          node: guestNodeName,
          nodeId: guestNodeName,
          nodeName: guestNodeName
        });
      });
    }
    
    // Filter guests for this specific node - don't deduplicate
    const nodeGuests = allProcessedGuests.filter(guest => 
      guest.node === mappedNodeName || guest.nodeId === mappedNodeName || guest.nodeName === mappedNodeName
    );
    
    logger.info(`Filtered ${nodeGuests.length} guests for node ${mappedNodeName}`);
    
    // Send just this node's guests with consistent node names
    socket.emit('guests', { guests: nodeGuests });
    
    // Also send all nodes data
    socket.emit('nodes', { nodes: allNodes });
  });

  // Handle disconnection
  socket.on('disconnect', () => {
    clients.delete(socket.id);
    logger.info(`Client disconnected: ${socket.id}. Total clients: ${clients.size}`);
  });
  
  // Additional event handlers for the mock server can be added here
});

// Start periodic updates (once every 10 seconds)
if (process.env.MOCK_ENABLE_UPDATES !== 'false') {
  // Disable global updates as they're not needed - each socket handles its own updates
  // const updateInterval = setInterval(() => {
  //   // When cluster mode is enabled, we need to be careful about which guests are running on which nodes
  //   updateMetrics();
  // }, 10000);
}

// Disable migrations that would create or modify guests
// setInterval(migrateRandomGuest, 10000);

// Start the server
server.listen(PORT, HOST, () => {
  logger.info(`Mock data server running on ${HOST}:${PORT}`);
});

// Export for testing
export default server; 

// Set up periodic updates for all connected clients
// Disabled to prevent flickering and race conditions
// setInterval(() => {
//   // Get all nodes data to ensure consistent mapping of IDs to names
//   const allNodes = Array.from(nodes.values());
//   
//   // Build a mapping of node ID to node name for consistent references
//   const nodeIdToName = new Map<string, string>();
//   allNodes.forEach(node => {
//     nodeIdToName.set(node.id, node.name);
//   });
//   
//   // Create a consistent set of guests with proper node references
//   const allGuestsByNode = new Map<string, MockVM[]>();
//   
//   // First prepare all guests with consistent node names
//   for (const [nodeId, nodeGuests] of guests.entries()) {
//     const nodeName = nodeIdToName.get(nodeId) || nodeId;
//     
//     // Process guests for this node, making sure they have consistent references
//     const processedGuests = nodeGuests.map((guest: MockVM) => {
//       return {
//         ...guest,
//         node: nodeName,
//         nodeId: nodeName,
//         nodeName: nodeName
//       };
//     });
//     
//     // Store using the node name as the key
//     allGuestsByNode.set(nodeName, processedGuests);
//   }
//   
//   // Process each connected client
//   for (const [clientId, client] of clients.entries()) {
//     if (!client.socket) continue;
//     
//     try {
//       // Get the client's node name from the mapping
//       const nodeName = nodeIdToName.get(client.nodeId || '') || client.nodeName || '';
//       
//       if (client.isClusterEntryPoint || client.isClusterMode) {
//         // For cluster mode, gather all guests but preserve their original node associations
//         const allGuests: MockVM[] = [];
//         for (const guests of allGuestsByNode.values()) {
//           allGuests.push(...guests);
//         }
//         
//         // Send all guests in cluster mode
//         logger.debug(`Sending ${allGuests.length} guests to cluster client ${clientId}`);
//         client.socket.emit('guests', { guests: allGuests });
//       } else if (nodeName) {
//         // In non-cluster mode, only send this node's guests
//         const nodeGuests = allGuestsByNode.get(nodeName) || [];
//         logger.debug(`Sending ${nodeGuests.length} guests to node client ${clientId} (${nodeName})`);
//         client.socket.emit('guests', { guests: nodeGuests });
//       }
//     } catch (error) {
//       logger.error(`Error sending update to client ${clientId}`, { error });
//     }
//   }
// }, 5000); // Reduced frequency: update every 5 seconds instead of 2

// Update the function that handles migrations to use more realistic state transitions
function scheduleMigrations() {
  // Only execute migrations in cluster mode
  if (!isClusterModeEnabled()) {
    return;
  }
  
  logger.info('Setting up scheduled migrations every 5 seconds');
  
  // Set up an interval to periodically migrate guests
  setInterval(() => {
    // 1. Randomly change some HA states
    const randomStateChanges = Math.floor(Math.random() * 5) + 3; // 3-7 state changes
    
    // Create an array of all shared guests
    interface GuestWithNode extends MockVM {
      nodeId: string;
    }

    const allGuests: GuestWithNode[] = [];
    for (const [nodeId, nodeGuests] of guests.entries()) {
      for (const guest of nodeGuests) {
        if (!allGuests.some(g => g.id === guest.id)) {
          allGuests.push({...guest, nodeId});
        }
      }
    }
    
    // Randomly shuffle to get random selection
    const shuffledGuests = allGuests.sort(() => 0.5 - Math.random());
    
    // Take the first few for state changes
    const guestsToChange = shuffledGuests.slice(0, randomStateChanges);
    
    // Apply state changes to selected guests
    for (const guest of guestsToChange) {
      // Decide on a new state - mimic some real-world scenarios
      let newState;
      
      // Choose state transition based on current state - simulating real behavior
      if (!guest.hastate || guest.hastate === HA_STATES.IGNORED) {
        // If no HA state, assign one of the active states
        const activeStates = [
          HA_STATES.STARTED, HA_STATES.STARTED, HA_STATES.STARTED, // 3x weight for started
          HA_STATES.ERROR, HA_STATES.MIGRATE
        ];
        newState = activeStates[Math.floor(Math.random() * activeStates.length)];
      }
      else if (guest.hastate === HA_STATES.STARTED) {
        // Started -> Migrate or Error
        newState = Math.random() < 0.3 ? HA_STATES.ERROR : HA_STATES.MIGRATE;
      } else if (guest.hastate === HA_STATES.ERROR) {
        // Error -> Recovery
        newState = HA_STATES.RECOVERY; 
      } else if (guest.hastate === HA_STATES.RECOVERY) {
        // Recovery -> Started or Error
        newState = Math.random() < 0.7 ? HA_STATES.STARTED : HA_STATES.ERROR;
      } else if (guest.hastate === HA_STATES.MIGRATE) {
        // Migrate -> Started (on a different node)
        newState = HA_STATES.STARTED;
        
        // For migrations, sometimes actually move the guest
        if (Math.random() < 0.7) {
          // Find other nodes that have this guest
          const nodesWithGuest = [];
          for (const [nodeId, nodeGuests] of guests.entries()) {
            if (nodeGuests.some(g => g.id === guest.id) && nodeId !== guest.nodeId) {
              nodesWithGuest.push(nodeId);
            }
          }
          
          if (nodesWithGuest.length > 0) {
            // Pick a random node to be the new primary
            const newPrimaryNode = nodesWithGuest[Math.floor(Math.random() * nodesWithGuest.length)];
            primaryNodeForGuest.set(guest.id.toString(), newPrimaryNode);
            logger.info(`Migrated guest ${guest.id} from ${guest.nodeId} to ${newPrimaryNode}`);
          }
        }
      } else {
        // Any other state - generally move toward STARTED
        const possibleStates = [
          HA_STATES.STARTED, HA_STATES.STARTED, HA_STATES.STARTED, // 3x weight for started
          HA_STATES.MIGRATE, HA_STATES.ERROR, HA_STATES.DISABLED
        ];
        newState = possibleStates[Math.floor(Math.random() * possibleStates.length)];
      }
      
      // Apply the new state to all instances of this guest
      for (const [nodeId, nodeGuests] of guests.entries()) {
        for (let i = 0; i < nodeGuests.length; i++) {
          const g = nodeGuests[i];
          if (g.id === guest.id) {
            // Check if this is the primary node
            const isPrimary = primaryNodeForGuest.get(g.id.toString()) === nodeId;
            
            // Set appropriate state and status
            if (isPrimary) {
              g.hastate = newState;
              
              // Update status based on hastate
              if (newState === HA_STATES.MIGRATE || newState === HA_STATES.RELOCATE) {
                g.status = 'migrating';
              } else if (newState === HA_STATES.ERROR || newState === HA_STATES.FENCE) {
                g.status = 'error';
              } else if (newState === HA_STATES.STOPPED || newState === HA_STATES.DISABLED) {
                g.status = 'stopped';
              } else {
                g.status = 'running';
              }
            } else {
              // For non-primary nodes, keep them stopped
              g.hastate = HA_STATES.STOPPED;
              g.status = 'stopped';
            }
            
            // Log the status change
            logger.info(`State change for guest ${g.id} on node ${nodeId}: ${g.hastate} (status: ${g.status})`);
          }
        }
      }
      
      // Emit update event to all connected clients
      io.emit('update', {
        type: 'guestStatusChange',
        guestId: guest.id,
        hastate: newState,
        timestamp: Date.now()
      });
    }
    
    // 2. Also occasionally perform traditional migrations - these are more for metrics changes
    const shouldMigrate = Math.random() < 0.5; // 50% chance of a migration (increased from 30%)
    
    if (shouldMigrate) {
      performRandomMigration();
    }
  }, 5000); // Run every 5 seconds instead of 10 seconds
}

// Helper function to perform a random migration between nodes
function performRandomMigration() {
  // Find a shared guest to migrate
  const availableGuests: Array<{id: string, currentPrimary: string, otherNode?: string}> = [];
  
  // Get all guests that exist on multiple nodes
  for (const [guestId, primaryNode] of primaryNodeForGuest.entries()) {
    // Count how many nodes have this guest
    let nodeCount = 0;
    let lastNode = '';
    
    for (const [nodeId, nodeGuests] of guests.entries()) {
      if (nodeGuests.some(g => g.id.toString() === guestId)) {
        nodeCount++;
        lastNode = nodeId;
      }
    }
    
    // Only include guests that exist on multiple nodes
    if (nodeCount > 1) {
      availableGuests.push({
        id: guestId,
        currentPrimary: primaryNode,
        otherNode: lastNode !== primaryNode ? lastNode : undefined
      });
    }
  }
  
  // If we have guests to migrate
  if (availableGuests.length > 0) {
    // Pick a random guest
    const guestToMigrate = availableGuests[Math.floor(Math.random() * availableGuests.length)];
    
    if (!guestToMigrate.otherNode) {
      // Find another node for this guest
      for (const [nodeId, nodeGuests] of guests.entries()) {
        if (nodeId !== guestToMigrate.currentPrimary && 
            nodeGuests.some(g => g.id.toString() === guestToMigrate.id)) {
          guestToMigrate.otherNode = nodeId;
          break;
        }
      }
    }
    
    // Only proceed if we have a valid otherNode
    if (guestToMigrate.otherNode) {
      // First, mark the guest as migrating on the current primary
      for (const [nodeId, nodeGuests] of guests.entries()) {
        if (nodeId === guestToMigrate.currentPrimary) {
          for (let i = 0; i < nodeGuests.length; i++) {
            if (nodeGuests[i].id.toString() === guestToMigrate.id) {
              // Set the guest as migrating
              nodeGuests[i].status = 'migrating';
              nodeGuests[i].hastate = HA_STATES.MIGRATE;
              logger.info(`Starting migration of guest ${guestToMigrate.id} from ${guestToMigrate.currentPrimary} to ${guestToMigrate.otherNode}`);
              
              // Emit update for the migrating guest
              io.emit('update', {
                type: 'guestStatusChange',
                guestId: guestToMigrate.id,
                status: 'migrating',
                hastate: HA_STATES.MIGRATE,
                timestamp: Date.now()
              });
            }
          }
        }
      }
      
      // Wait a short time, then complete the migration
      setTimeout(() => {
        // Update the primary node
        primaryNodeForGuest.set(guestToMigrate.id, guestToMigrate.otherNode!);
        
        // Update status for all instances of this guest
        for (const [nodeId, nodeGuests] of guests.entries()) {
          for (let i = 0; i < nodeGuests.length; i++) {
            if (nodeGuests[i].id.toString() === guestToMigrate.id) {
              // Update status based on whether this is now the primary
              const isPrimary = nodeId === guestToMigrate.otherNode;
              
              if (isPrimary) {
                nodeGuests[i].status = 'running';
                nodeGuests[i].hastate = HA_STATES.STARTED;
                logger.info(`Migration complete: guest ${guestToMigrate.id} is now running on ${nodeId}`);
              } else {
                nodeGuests[i].status = 'stopped';
                nodeGuests[i].hastate = HA_STATES.STOPPED;
                logger.info(`Guest ${guestToMigrate.id} is now stopped on ${nodeId}`);
              }
            }
          }
        }
        
        // Emit update for the migrated guest
        io.emit('update', {
          type: 'guestMigrated',
          guestId: guestToMigrate.id,
          fromNode: guestToMigrate.currentPrimary,
          toNode: guestToMigrate.otherNode!,
          timestamp: Date.now()
        });
      }, 3000); // 3 second for migration to complete
    } else {
      logger.warn(`Could not find another node for guest ${guestToMigrate.id}, skipping migration`);
    }
  }
}

// Add the cluster/ha/resources endpoint to show HA managed resources
app.get('/api2/json/cluster/ha/resources', (req, res) => {
  logger.info('Cluster HA resources endpoint called');
  
  if (!isClusterModeEnabled()) {
    logger.info('Cluster mode is disabled, returning empty array');
    res.json({ data: [] });
    return;
  }
  
  // Get all guests
  const allGuests: MockVM[] = [];
  for (const nodeGuests of guests.values()) {
    nodeGuests.forEach(guest => {
      allGuests.push({...guest});
    });
  }
  
  // Filter to only get shared guests that are managed by HA
  const haResources = allGuests
    .filter(guest => guest.shared) // Only include shared guests
    .filter((guest, index, self) => {
      // Deduplicate guests by ID (only include one instance of each shared guest)
      return index === self.findIndex(g => g.id.toString() === guest.id.toString());
    })
    .map(guest => {
      // Transform to HA resource format
      const guestId = guest.id.toString();
      const vmid = typeof guest.id === 'number' ? guest.id : parseInt(String(guest.id).replace(/\D/g, '')) || parseInt(String(guest.id));
      const type = guest.type === 'lxc' || guest.type === 'ct' ? 'ct' : 'vm';
      
      return {
        sid: `${type}:${vmid}`, // Service ID in the format type:id
        type: type, // Resource type (vm or ct)
        status: guest.hastate || 'started', // Current status in HA
        state: guest.hastate || 'started', // Requested state
        digest: '1a2b3c4d5e6f7g8h9i0j', // Mock digest value
        max_restart: 1, // Default max restart attempts
        max_relocate: 1, // Default max relocate attempts
        group: 'default' // Default group
      };
    });
  
  res.json({ data: haResources });
});

// Add the cluster/ha/status endpoint to show HA status information
app.get('/api2/json/cluster/ha/status', (req, res) => {
  logger.info('Cluster HA status endpoint called');
  
  if (!isClusterModeEnabled()) {
    logger.info('Cluster mode is disabled, returning empty array');
    res.json({ data: [] });
    return;
  }
  
  // Get all nodes
  const allNodes = Array.from(nodes.values());
  
  // Create service status entries
  const serviceStatuses = [];
  
  // Add shared guests (which are HA managed)
  for (const [guestId, primaryNodeId] of primaryNodeForGuest.entries()) {
    // Find this guest
    let haGuest: MockVM | undefined;
    
    // Look through all nodes to find the guest
    for (const nodeGuests of guests.values()) {
      const foundGuest = nodeGuests.find(g => g.id.toString() === guestId);
      if (foundGuest) {
        haGuest = foundGuest;
        break;
      }
    }
    
    if (!haGuest) continue;
    
    // Get the guest type
    const guestType = haGuest.type === 'lxc' || haGuest.type === 'ct' ? 'ct' : 'vm';
    const vmid = typeof haGuest.id === 'number' ? haGuest.id : parseInt(String(haGuest.id).replace(/\D/g, '')) || parseInt(String(haGuest.id));
    
    // Create the service status entry
    serviceStatuses.push({
      state: haGuest.hastate || HA_STATES.STARTED,
      sid: `${guestType}:${vmid}`,
      node: primaryNodeId,
      crm_state: haGuest.hastate || HA_STATES.STARTED,
      request_state: haGuest.hastate || HA_STATES.STARTED
    });
  }
  
  // Create node status entries
  const nodeStatuses = allNodes.map(node => ({
    id: node.id,
    name: node.name,
    type: 'node',
    status: 'online',
    quorum: true // All nodes have quorum in our mock environment
  }));
  
  // Combine both for the final response
  const statusData = [
    // Add a cluster entry
    {
      type: 'cluster',
      name: 'Proxmox HA Cluster',
      quorate: true,
      enabled: true
    },
    // Add node entries
    ...nodeStatuses,
    // Add service entries
    ...serviceStatuses
  ];
  
  res.json({ data: statusData });
});

// Function to check if a node is the primary for a guest
function isNodePrimaryForGuest(nodeId: string, guestId: string): boolean {
  const primaryNodeId = primaryNodeForGuest.get(guestId);
  return primaryNodeId === nodeId;
}