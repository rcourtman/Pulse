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
import path from 'path';

// Load environment variables
if (process.env.NODE_ENV === 'production') {
  dotenv.config({ path: path.resolve(process.cwd(), '.env.production') });
} else {
  // Try to load .env.local first, fall back to .env
  const result = dotenv.config({ path: path.resolve(process.cwd(), '.env.local') });
  if (result.error) {
    dotenv.config(); // Fall back to .env
  }
}

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
    // Get all environment variables starting with PROXMOX_HOST
    const envVars = Object.entries(process.env);
    const proxmoxHosts = envVars
      .filter(([key]) => key.startsWith('PROXMOX_HOST'))
      .map(([key, host]) => {
        // Fix the index extraction
        const suffix = key === 'PROXMOX_HOST' ? '' : `_${key.split('_').pop()}`;
        
        const config = {
          host: host!,
          tokenId: process.env[`PROXMOX_TOKEN_ID${suffix}`]!,
          tokenSecret: process.env[`PROXMOX_TOKEN_SECRET${suffix}`]!
        };
        
        logger.debug('Processing Proxmox config:', { 
          key,
          suffix,
          host: config.host,
          tokenId: config.tokenId,
          hasSecret: !!config.tokenSecret
        });
        
        return config;
      })
      .filter((config): config is ProxmoxConfig => {
        const isValid = !!config.host && !!config.tokenId && !!config.tokenSecret;
        if (!isValid) {
          logger.warn('Incomplete Proxmox configuration found:', { 
            host: config.host, 
            hasTokenId: !!config.tokenId,
            hasTokenSecret: !!config.tokenSecret 
          });
        }
        return isValid;
      });

    logger.info('Environment variables:', 
      Object.keys(process.env)
        .filter(key => key.startsWith('PROXMOX_'))
        .map(key => `${key}=${key.includes('SECRET') ? '***' : process.env[key]}`)
    );

    if (proxmoxHosts.length === 0) {
      logger.warn('No valid Proxmox configurations found in environment variables');
      return;
    }

    logger.info(`Found ${proxmoxHosts.length} Proxmox configurations:`, 
      proxmoxHosts.map(h => ({ host: h.host, tokenId: h.tokenId }))
    );

    // Initialize each Proxmox host
    for (const config of proxmoxHosts) {
      const service = new ProxmoxService({
        host: config.host,
        tokenId: config.tokenId,
        tokenSecret: config.tokenSecret
      });

      try {
        await service.validate();
        const nodes = await service.getNodes();
        
        if (nodes && nodes.length > 0) {
          monitor.addNode({
            host: config.host,
            tokenId: config.tokenId,
            tokenSecret: config.tokenSecret,
            nodeName: nodes[0].node || 'Unknown Node'
          });
          
          logger.info(`Successfully connected to Proxmox server: ${config.host}`);
        }
      } catch (error) {
        logger.error(`Failed to connect to Proxmox server: ${config.host}`, error);
      }
    }
  } catch (error) {
    logger.error('Error initializing Proxmox configurations:', error);
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