import { ProxmoxService } from './proxmox';
import { logger } from '../utils/logger';
import { v4 as uuidv4 } from 'uuid';
import { Server as SocketIOServer } from 'socket.io';
import fs from 'fs';
import path from 'path';

interface NodeStatus {
  cpu: number;
  memory: {
    total: number;
    used: number;
    free: number;
  };
  uptime: number;
}

interface MonitoredNode {
  id: string;
  service: ProxmoxService;
  status?: NodeStatus;
  errorCount?: number;
}

class NodeMonitor {
  private nodes: Map<string, MonitoredNode>;
  private updateInterval: number;
  private intervalId?: NodeJS.Timeout;
  private io: SocketIOServer;

  constructor(io: SocketIOServer, updateInterval = 2000) {
    this.nodes = new Map();
    this.updateInterval = updateInterval;
    this.io = io;
    logger.info('NodeMonitor initialized');
    this.loadNodes();
  }

  addNode(credentials: any): string {
    const nodeId = uuidv4();
    const service = new ProxmoxService(credentials);
    
    this.nodes.set(nodeId, {
      id: nodeId,
      service
    });

    this.io.emit('nodeStatus', {
      nodeId,
      name: service.getNodeName(),
      host: service.getHost(),
      tokenId: service.getTokenId(),
      status: 'online'
    });

    logger.info(`Node added: ${nodeId}`);
    this.saveNodes();
    return nodeId;
  }

  removeNode(nodeId: string): boolean {
    const removed = this.nodes.delete(nodeId);
    if (removed) {
      this.io.emit('nodeRemoved', nodeId);
      logger.info(`Node removed: ${nodeId}`);
      this.saveNodes();
    }
    return removed;
  }

  getNode(nodeId: string): MonitoredNode | undefined {
    return this.nodes.get(nodeId);
  }

  getAllNodes(): MonitoredNode[] {
    return Array.from(this.nodes.values());
  }

  async updateNodeStatus(nodeId: string): Promise<NodeStatus | null> {
    const node = this.nodes.get(nodeId);
    if (!node) {
      logger.warn(`Unknown node: ${nodeId}`);
      return null;
    }

    try {
      const nodeStatus = await node.service.getNodeStatus();

      const status: NodeStatus = {
        cpu: nodeStatus.cpu,
        memory: {
          total: nodeStatus.memory.total,
          used: nodeStatus.memory.used,
          free: nodeStatus.memory.free
        },
        uptime: nodeStatus.uptime
      };

      node.status = status;

      // Emit the updated status to all clients
      this.io.emit('nodeStatus', {
        nodeId: node.id,
        name: node.service.getNodeName(),
        host: node.service.getHost(),
        tokenId: node.service.getTokenId(),
        status: 'online',
        metrics: status
      });

      this.saveNodes();
      return status;
    } catch (error) {
      // Don't remove the node on first error, increment error count
      node.errorCount = (node.errorCount || 0) + 1;
      
      logger.error(`Status update failed for node ${nodeId}`, { 
        error,
        errorCount: node.errorCount 
      });

      // Only remove node after multiple consecutive failures
      if (node.errorCount > 3) {
        logger.error(`Removing node ${nodeId} after ${node.errorCount} consecutive failures`);
        this.removeNode(nodeId);
      } else {
        // Just mark as error but keep trying
        this.io.emit('nodeStatus', {
          nodeId,
          name: node.service.getNodeName(),
          host: node.service.getHost(),
          tokenId: node.service.getTokenId(),
          status: 'error',
          metrics: node.status // Keep last known metrics
        });
      }

      this.saveNodes();
      return null;
    }
  }

  startMonitoring(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
    }

    this.intervalId = setInterval(async () => {
      for (const [nodeId, node] of this.nodes) {
        try {
          const status = await this.updateNodeStatus(nodeId);
          if (status) {
            // Reset error count on successful update
            node.errorCount = 0;
          }
        } catch (error) {
          logger.error(`Monitor update failed for node ${nodeId}`, { error });
        }
      }
    }, this.updateInterval);

    logger.info('Node monitoring started');
  }

  stopMonitoring(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = undefined;
      logger.info('Node monitoring stopped');
    }
  }

  private getNodesFilePath(): string {
    const configDir = process.env.CONFIG_DIR || './config';
    if (!fs.existsSync(configDir)) {
      fs.mkdirSync(configDir, { recursive: true });
    }
    return path.join(configDir, 'nodes.json');
  }

  private saveNodes() {
    const nodes = Array.from(this.nodes.entries()).map(([id, node]) => ({
      id,
      host: node.service.getHost(),
      tokenId: node.service.getTokenId(),
      tokenSecret: node.service.credentials.tokenSecret,
      nodeName: node.service.getNodeName()
    }));
    
    fs.writeFileSync(this.getNodesFilePath(), JSON.stringify(nodes, null, 2));
  }

  private loadNodes() {
    try {
      const nodesPath = this.getNodesFilePath();
      if (fs.existsSync(nodesPath)) {
        const nodes = JSON.parse(fs.readFileSync(nodesPath, 'utf8'));
        nodes.forEach(node => this.addNode(node));
      }
    } catch (error) {
      logger.error('Failed to load nodes:', error);
    }
  }
}

let monitor: NodeMonitor | null = null;

export function initMonitor(io: SocketIOServer, updateInterval?: number): NodeMonitor {
  if (!monitor) {
    monitor = new NodeMonitor(io, updateInterval);
    monitor.startMonitoring();
    logger.info('Monitor initialized successfully');
  }
  return monitor;
}

export function getMonitor(): NodeMonitor {
  if (!monitor) {
    throw new Error('Monitor not initialized');
  }
  return monitor;
} 