import { Server as SocketIOServer } from 'socket.io';
import { logger } from '../utils/logger';
import { ProxmoxService } from './proxmox';
import { v4 as uuidv4 } from 'uuid';

interface Node {
  id: string;
  host: string;
  tokenId: string;
  tokenSecret: string;
  nodeName?: string;
  status?: 'online' | 'error';
}

interface MonitorConfig {
  maxNodes?: number;          // Maximum number of nodes allowed
  updateInterval?: number;    // How often to update metrics (ms)
  maxRetries?: number;       // Max retries before marking node as failed
  timeout?: number;          // API call timeout
}

export class NodeMonitor {
  private nodes: Map<string, Node> = new Map();
  private io: SocketIOServer;
  private config: MonitorConfig;

  constructor(io: SocketIOServer, config: MonitorConfig = {}) {
    this.io = io;
    this.config = {
      maxNodes: process.env.MAX_NODES ? parseInt(process.env.MAX_NODES) : 100,
      updateInterval: process.env.UPDATE_INTERVAL ? parseInt(process.env.UPDATE_INTERVAL) : 5000,
      maxRetries: process.env.MAX_RETRIES ? parseInt(process.env.MAX_RETRIES) : 3,
      timeout: process.env.API_TIMEOUT ? parseInt(process.env.API_TIMEOUT) : 5000,
      ...config
    };
    logger.info('NodeMonitor initialized with config:', this.config);
  }

  addNode(nodeData: Omit<Node, 'id'>): string {
    // Check node limit
    if (this.nodes.size >= this.config.maxNodes!) {
      throw new Error(`Maximum number of nodes (${this.config.maxNodes}) reached`);
    }

    const id = uuidv4();
    const node: Node = { 
      ...nodeData, 
      id,
      status: 'online' 
    };

    // Check for existing node with same host
    const existingNode = Array.from(this.nodes.values())
      .find(n => n.host.replace(/\/$/, '') === node.host.replace(/\/$/, ''));
    
    if (existingNode) {
      // Update existing node instead of adding new one
      this.nodes.set(existingNode.id, {
        ...existingNode,
        ...nodeData,
        status: 'online'
      });
      
      // Emit update event
      this.emitNodeStatus(existingNode.id);
      return existingNode.id;
    }
    
    // Add new node
    this.nodes.set(id, node);
    
    // Emit new node event
    this.emitNodeStatus(id);
    logger.info('Added new node:', { id, host: node.host });
    
    return id;
  }

  getNodes(): Node[] {
    return Array.from(this.nodes.values());
  }

  private emitNodeStatus(nodeId: string) {
    const node = this.nodes.get(nodeId);
    if (!node) return;

    this.io.emit('nodeStatus', {
      nodeId: node.id,
      name: node.nodeName,
      host: node.host,
      tokenId: node.tokenId,
      status: 'online'  // We can update this based on actual status later
    });
    
    logger.debug('Emitted node status:', { nodeId, host: node.host });
  }

  getStats(): MonitorStats {
    return {
      totalNodes: this.nodes.size,
      maxNodes: this.config.maxNodes!,
      activeConnections: this.io.engine.clientsCount,
      updateInterval: this.config.updateInterval!,
      memoryUsage: process.memoryUsage()
    };
  }
}

let monitor: NodeMonitor | null = null;

export function initMonitor(io: SocketIOServer, options?: { quiet: boolean }): NodeMonitor {
  if (!monitor) {
    monitor = new NodeMonitor(io);
    if (!options?.quiet) {
      logger.info('Monitor initialized successfully');
    }
  }
  return monitor;
}

export function getMonitor(): NodeMonitor {
  if (!monitor) {
    throw new Error('Monitor not initialized');
  }
  return monitor;
} 