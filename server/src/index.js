const express = require('express');
const http = require('http');
const cors = require('cors');
const helmet = require('helmet');
const { Server } = require('socket.io');
const path = require('path');
const dotenv = require('dotenv');
const logger = require('./utils/logger');
const setupWebSocketServer = require('./websocket');
const apiRoutes = require('./routes');
const { loadProxmoxNodesFromEnv } = require('./config/proxmoxNodes');

// Load environment variables
dotenv.config();

// Create Express app
const app = express();
const server = http.createServer(app);

// Set up Socket.IO
const io = new Server(server, {
  cors: {
    origin: process.env.CLIENT_URL || '*',
    methods: ['GET', 'POST'],
  },
  allowEIO3: true,
  transports: ['websocket', 'polling'],
  pingTimeout: 30000,
  pingInterval: 10000,
  upgradeTimeout: 15000,
  maxHttpBufferSize: 1e8, // 100MB
  ...(process.env.NODE_ENV !== 'production' ? { debug: true } : {})
});

// Middleware
app.use(helmet());
app.use(cors({
  origin: process.env.CLIENT_URL || '*',
}));
app.use(express.json());

// API routes
app.use('/api', apiRoutes);

// Serve static files from the React app in production
if (process.env.NODE_ENV === 'production') {
  app.use(express.static(path.join(__dirname, '../../client/dist')));
  
  app.get('*', (req, res) => {
    res.sendFile(path.join(__dirname, '../../client/dist/index.html'));
  });
}

// Load pre-configured Proxmox nodes
const configuredNodes = loadProxmoxNodesFromEnv();
logger.info(`Loaded ${configuredNodes.length} Proxmox node(s) from environment variables`);

// Set up WebSocket server
setupWebSocketServer(io);

// Error handling middleware
app.use((err, req, res, next) => {
  logger.error(`Error: ${err.message}`);
  res.status(err.status || 500).json({
    error: {
      message: err.message || 'Internal Server Error',
    },
  });
});

// Start server
const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  logger.info(`Server running on port ${PORT}`);
});

// Handle unhandled promise rejections
process.on('unhandledRejection', (err) => {
  logger.error('Unhandled Promise Rejection:', err);
});

module.exports = server; 