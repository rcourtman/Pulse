import express from 'express';
import { createServer } from 'http';
import { Server } from 'socket.io';
import cors from 'cors';
import NodeCache from 'node-cache';
import dotenv from 'dotenv';
import proxmoxRoutes from './routes/proxmox.routes';
import { initMonitor, getMonitor } from './services/monitorService';
import { logger } from './utils/logger';
import { ProxmoxService } from './services/proxmox';
import crypto from 'crypto';
import { config } from 'dotenv';

// Load environment variables
dotenv.config();
config();

// Initialize cache
const cache = new NodeCache({ stdTTL: 100, checkperiod: 120 });

// Get allowed origins from environment variables with a safe default
const allowedOrigins = process.env.ALLOWED_ORIGINS?.split(',') || ['http://localhost:5173'];

// Create Express app
const app = express();

// Configure CORS
app.use(cors({
  origin: allowedOrigins,
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  credentials: true,
  allowedHeaders: ['Content-Type', 'Authorization']
}));

// Add body parser middleware
app.use(express.json());

// Create HTTP server
const httpServer = createServer(app);

// Initialize Socket.IO with matching CORS config
const io = new Server(httpServer, {
  cors: {
    origin: allowedOrigins,
    methods: ['GET', 'POST'],
    credentials: true
  },
  pingTimeout: 60000, // Increase ping timeout
  transports: ['websocket', 'polling']
});

// Initialize the monitor with a quiet option
const monitor = initMonitor(io, { quiet: true });

// Update the interface to match our Node type
interface ProxmoxConfig {
  host: string;
  tokenId: string;
  tokenSecret: string;
}

async function initializeDefaultProxmox() {
  try {
    const proxmoxConfigs: ProxmoxConfig[] = [];
    let counter = 1;
    
    // Add the default (unnumbered) configuration
    if (process.env.PROXMOX_HOST && process.env.PROXMOX_TOKEN_ID && process.env.PROXMOX_TOKEN_SECRET) {
      const host = process.env.PROXMOX_HOST.replace(/\/$/, ''); // Normalize host
      
      // Check if this host is already configured
      const existingNodes = monitor?.getNodes() || [];
      const isDuplicate = existingNodes.some(node => 
        node.host.replace(/\/$/, '') === host
      );

      if (!isDuplicate) {
        proxmoxConfigs.push({
          host: process.env.PROXMOX_HOST,
          tokenId: process.env.PROXMOX_TOKEN_ID,
          tokenSecret: process.env.PROXMOX_TOKEN_SECRET
        });
      }
    }

    // Look for numbered configurations
    while (true) {
      const suffix = counter === 1 ? '' : `_${counter}`;
      const host = process.env[`PROXMOX_HOST${suffix}`];
      const tokenId = process.env[`PROXMOX_TOKEN_ID${suffix}`];
      const tokenSecret = process.env[`PROXMOX_TOKEN_SECRET${suffix}`];

      if (!host || !tokenId || !tokenSecret) break;

      proxmoxConfigs.push({
        host,
        tokenId,
        tokenSecret
      });
      counter++;
    }

    // Initialize each Proxmox configuration
    for (const config of proxmoxConfigs) {
      // Normalize the host URL by removing trailing slash
      const normalizedConfigHost = config.host.replace(/\/$/, '');
      
      // Check if this server is already configured by matching host
      const isDuplicate = (monitor?.getNodes() || []).some(node => {
        if (!node?.host) return false;
        const normalizedNodeHost = node.host.replace(/\/$/, '');
        return normalizedNodeHost === normalizedConfigHost;
      });

      if (!isDuplicate) {
        const service = new ProxmoxService({
          host: config.host,
          tokenId: config.tokenId,
          tokenSecret: config.tokenSecret
        });

        try {
          // Validate connection and get node info
          await service.validate();
          const nodes = await service.getNodes();
          
          if (nodes && nodes.length > 0) {
            // Log the node data to see what we're getting
            logger.debug('Node data from Proxmox:', nodes[0]);

            monitor.addNode({
              host: config.host,
              tokenId: config.tokenId,
              tokenSecret: config.tokenSecret,
              nodeName: nodes[0].node || nodes[0].name || 'Unknown Node' // Try both properties
            });
            
            logger.info(`Successfully connected to Proxmox server: ${config.host} (${nodes[0].node || 'Unknown Node'})`);
          }
        } catch (error) {
          logger.error(`Failed to connect to Proxmox server: ${config.host}`, error);
        }
      }
    }
  } catch (error) {
    logger.error('Error initializing default Proxmox configurations', error);
  }
}

// Socket.IO connection handling
io.on('connection', (socket) => {
  logger.info('Client connected');
  
  // Send current nodes to newly connected client
  const nodes = monitor.getNodes();
  logger.info('Sending nodes to new client:', nodes);
  
  nodes.forEach(node => {
    socket.emit('nodeStatus', {
      nodeId: node.id,
      name: node.nodeName,
      host: node.host,
      tokenId: node.tokenId,
      status: 'online'
    });
  });

  socket.on('getNodes', () => {
    logger.info('Client requested nodes');
    nodes.forEach(node => {
      socket.emit('nodeStatus', {
        nodeId: node.id,
        name: node.nodeName,
        host: node.host,
        tokenId: node.tokenId,
        status: 'online'
      });
    });
  });

  socket.on('disconnect', (reason) => {
    logger.info('Client disconnected:', reason);
  });
});

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ status: 'ok', timestamp: new Date().toISOString() });
});

// Basic health check route
app.get('/', (req, res) => {
  res.json({ status: 'ok', message: 'Pulse API is running' });
});

// Add routes
app.use('/api/proxmox', proxmoxRoutes);

// Catch-all route
app.get('*', (req, res) => {
  res.status(404).json({ status: 'not_found', message: 'Route not found' });
});

// Error handling middleware
app.use((err: Error, req: express.Request, res: express.Response, next: express.NextFunction) => {
  logger.error('Server error:', err);
  res.status(500).json({ error: 'Internal Server Error' });
});

// Start server
const PORT = process.env.PORT || 3000;
httpServer.listen(PORT, async () => {
  logger.info(`Server running on port ${PORT}`);
  await initializeDefaultProxmox();
});

export { app, io, logger, cache };