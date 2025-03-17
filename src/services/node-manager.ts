import { EventEmitter } from 'events';
import { ProxmoxClient } from '../api/proxmox-client';
import { MockClient } from '../api/mock-client';
import { createLogger } from '../utils/logger';
import config from '../config';
import { NodeConfig, ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxGuest, ProxmoxEvent } from '../types';
import * as fs from 'fs';
import * as path from 'path';

export class NodeManager extends EventEmitter {
  private nodes: Map<string, ProxmoxClient | MockClient> = new Map();
  private nodeStatus: Map<string, ProxmoxNodeStatus> = new Map();
  private vms: Map<string, ProxmoxVM> = new Map();
  private containers: Map<string, ProxmoxContainer> = new Map();
  private eventUnsubscribers: Map<string, () => void> = new Map();
  private logger = createLogger('NodeManager');
  private pollingInterval: NodeJS.Timeout | null = null;
  private isMockData: boolean = false;
  private pollingTimer: NodeJS.Timeout | null = null;

  constructor() {
    super();
    this.logger.info('Initializing NodeManager');
    this.isMockData = process.env.USE_MOCK_DATA === 'true' || process.env.MOCK_DATA_ENABLED === 'true';
    this.initializeNodes();
  }

  /**
   * Initialize nodes from configuration
   */
  private async initializeNodes(): Promise<void> {
    // First check if auto-detection is enabled
    const autoDetectCluster = process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true';
    
    // Only use the explicit settings if auto-detection is disabled
    const forcedClusterMode = !autoDetectCluster && (
      process.env.PROXMOX_CLUSTER_MODE === 'true' || 
      process.env.MOCK_CLUSTER_ENABLED === 'true' || 
      process.env.MOCK_CLUSTER_MODE === 'true'
    );
    
    // Determine the initial cluster mode
    let initialClusterMode = forcedClusterMode;
    
    // When using mock data with auto-detection, default to cluster mode initially
    if (autoDetectCluster && (process.env.USE_MOCK_DATA === 'true' || 
                              process.env.MOCK_DATA_ENABLED === 'true')) {
      initialClusterMode = true;
    }
    
    // Find any node marked as a cluster entry point
    const clusterEntryPoints = config.nodes.filter(node => node.isClusterEntryPoint);
    
    if (autoDetectCluster) {
      this.logger.info('Cluster auto-detection is enabled - will connect to all nodes first');
      
      // With auto-detection, add all nodes but set a flag for the client to detect cluster status
      for (const nodeConfig of config.nodes) {
        // Set the auto-detect flag on each node config
        nodeConfig.autoDetectCluster = true;
        
        try {
          await this.addNode(nodeConfig);
        } catch (error) {
          this.logger.error(`Failed to initialize node ${nodeConfig.name}`, { error });
        }
      }
    } else if (initialClusterMode) {
      this.logger.info(`Initializing in cluster mode (enabled by environment variables)`);
      
      if (clusterEntryPoints.length > 0) {
        // In cluster entry point mode, only add the entry point node
        this.logger.info(`Using cluster entry point: ${clusterEntryPoints[0].name}`);
        try {
          await this.addNode(clusterEntryPoints[0]);
          this.logger.info(`Initialized cluster with entry point node ${clusterEntryPoints[0].name}`);
        } catch (error) {
          this.logger.error(`Failed to initialize cluster entry point node ${clusterEntryPoints[0].name}`, { error });
        }
      } else {
        // No explicit entry point but cluster mode is enabled
        // Use the first node as the entry point
        this.logger.info(`No explicit cluster entry point found, using first node as entry point`);
        try {
          await this.addNode(config.nodes[0]);
          this.logger.info(`Initialized cluster with first node ${config.nodes[0].name} as entry point`);
        } catch (error) {
          this.logger.error(`Failed to initialize first node ${config.nodes[0].name} as cluster entry point`, { error });
        }
      }
    } else {
      // Standard mode - add all nodes from configuration
      this.logger.info(`Initializing with ${config.nodes.length} nodes from configuration (cluster mode disabled)`);
      for (const nodeConfig of config.nodes) {
        try {
          await this.addNode(nodeConfig);
        } catch (error) {
          this.logger.error(`Failed to initialize node ${nodeConfig.name}`, { error });
        }
      }
    }

    // Start polling for updates
    this.startPolling();
  }

  /**
   * Add a new node
   */
  async addNode(nodeConfig: NodeConfig): Promise<boolean> {
    try {
      this.logger.info(`Adding node: ${nodeConfig.name} (${nodeConfig.host})`);
      
      // Create client based on whether mock data is enabled
      let client;
      if (this.isMockData) {
        client = new MockClient(nodeConfig);
      } else {
        client = new ProxmoxClient(nodeConfig);
      }
      
      // Test connection - both client types should implement this
      try {
        // For API compatibility, attempt to use testConnection if it exists
        if (typeof client.testConnection === 'function') {
          const connected = await client.testConnection();
          if (!connected) {
            this.logger.error(`Failed to connect to node: ${nodeConfig.name}`);
            return false;
          }
        } else {
          // If testConnection doesn't exist, just log a warning and continue
          this.logger.warn(`No test connection method available for client type, assuming connected`);
        }
      } catch (error) {
        this.logger.error(`Error testing connection to node: ${nodeConfig.name}`, { error });
        return false;
      }
      
      // Store the client
      this.nodes.set(nodeConfig.id, client);
      
      // Set up event handlers
      client.on('nodeStatus', (status: ProxmoxNodeStatus) => {
        this.handleNodeStatusUpdate(nodeConfig.id, status);
      });
      
      client.on('vmList', (vms: ProxmoxVM[]) => {
        this.handleVMListUpdate(nodeConfig.id, vms);
      });
      
      client.on('containerList', (containers: ProxmoxContainer[]) => {
        this.handleContainerListUpdate(nodeConfig.id, containers);
      });
      
      client.on('event', (event: ProxmoxEvent) => {
        this.handleEvent(nodeConfig.id, event);
      });
      
      // Set up event polling if the client supports it
      if (typeof client.setupEventPolling === 'function') {
        client.setupEventPolling();
      } else {
        this.logger.debug(`Client type does not support setupEventPolling, skipping`);
      }
      
      // Refresh data immediately
      await this.refreshNodeData(nodeConfig.id);
      
      this.logger.info(`Node added successfully: ${nodeConfig.name}`);
      return true;
    } catch (error) {
      this.logger.error(`Error adding node: ${nodeConfig.name}`, { error });
      return false;
    }
  }

  /**
   * Remove a node
   */
  removeNode(nodeId: string): boolean {
    try {
      this.logger.info(`Removing node: ${nodeId}`);
      
      // Unsubscribe from events
      const unsubscribe = this.eventUnsubscribers.get(nodeId);
      if (unsubscribe) {
        unsubscribe();
        this.eventUnsubscribers.delete(nodeId);
      }
      
      // Remove from nodes map
      this.nodes.delete(nodeId);
      
      // Remove node status
      this.nodeStatus.delete(nodeId);
      
      // Remove VMs and containers for this node
      for (const [vmId, vm] of this.vms.entries()) {
        if (vm.node === nodeId) {
          this.vms.delete(vmId);
        }
      }
      
      for (const [containerId, container] of this.containers.entries()) {
        if (container.node === nodeId) {
          this.containers.delete(containerId);
        }
      }
      
      this.logger.info(`Node removed successfully: ${nodeId}`);
      return true;
    } catch (error) {
      this.logger.error(`Error removing node: ${nodeId}`, { error });
      return false;
    }
  }

  /**
   * Start polling for updates
   */
  private setupDiagnosticLogger() {
    const logDir = path.join(process.cwd(), 'logs');
    if (!fs.existsSync(logDir)) {
      fs.mkdirSync(logDir, { recursive: true });
    }
    
    const logFile = path.join(logDir, 'node-polling.log');
    this.logger.info(`Setting up diagnostic logger to ${logFile}`);
    
    // Clear the log file on startup
    fs.writeFileSync(logFile, `=== Node Polling Log Started at ${new Date().toISOString()} ===\n\n`);
    
    return (message: string) => {
      const timestamp = new Date().toISOString();
      fs.appendFileSync(logFile, `[${timestamp}] ${message}\n`);
    };
  }

  private logPolling = this.setupDiagnosticLogger();

  private startPolling(): void {
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
    }

    this.logger.info(`Starting polling for node updates (interval: ${config.nodePollingIntervalMs}ms)`);
    this.logPolling(`Starting polling with interval: ${config.nodePollingIntervalMs}ms`);
    
    // Log current node state
    this.logPolling(`Current nodes: ${Array.from(this.nodes.keys()).join(', ')}`);
    
    // Use a more efficient polling approach that staggers requests to avoid overwhelming the system
    // and to make metrics more responsive
    this.pollingInterval = setInterval(async () => {
      this.logger.debug('Poll cycle starting...');
      this.logPolling(`=== POLL CYCLE STARTING ===`);
      
      const nodeIds = Array.from(this.nodes.keys());
      this.logPolling(`Polling ${nodeIds.length} nodes: ${nodeIds.join(', ')}`);
      
      // If we have multiple nodes, stagger their polling to avoid all nodes being polled at once
      // This helps prevent unrealistic rate calculations due to timing issues
      if (nodeIds.length > 1) {
        // Process one node at a time with a small delay between each
        for (let i = 0; i < nodeIds.length; i++) {
          const nodeId = nodeIds[i];
          try {
            const startTime = Date.now();
            this.logger.debug(`Polling node ${nodeId} for updates...`);
            this.logPolling(`Polling node ${nodeId}...`);
            
            await this.refreshNodeData(nodeId);
            
            const duration = Date.now() - startTime;
            this.logger.debug(`Poll completed for node ${nodeId} in ${duration}ms`);
            this.logPolling(`Poll completed for node ${nodeId} in ${duration}ms`);
            
            // Add a small delay between node polls to avoid timing issues
            // but only if there are more nodes to process
            if (i < nodeIds.length - 1) {
              await new Promise(resolve => setTimeout(resolve, 200));
            }
          } catch (error) {
            this.logger.error(`Error polling node: ${nodeId}`, { error });
            this.logPolling(`ERROR polling node ${nodeId}: ${error}`);
          }
        }
      } else if (nodeIds.length === 1) {
        // If we only have one node, just poll it directly
        try {
          const nodeId = nodeIds[0];
          const startTime = Date.now();
          this.logger.debug(`Polling node ${nodeId} for updates...`);
          this.logPolling(`Polling node ${nodeId}...`);
          
          await this.refreshNodeData(nodeId);
          
          const duration = Date.now() - startTime;
          this.logger.debug(`Poll completed for node ${nodeId} in ${duration}ms`);
          this.logPolling(`Poll completed for node ${nodeId} in ${duration}ms`);
        } catch (error) {
          this.logger.error(`Error polling node: ${nodeIds[0]}`, { error });
          this.logPolling(`ERROR polling node ${nodeIds[0]}: ${error}`);
        }
      }
      this.logger.debug('Poll cycle completed');
      this.logPolling(`=== POLL CYCLE COMPLETED ===`);
    }, config.nodePollingIntervalMs);
  }

  /**
   * Stop polling for updates
   */
  stopPolling(): void {
    if (this.pollingTimer) {
      clearInterval(this.pollingTimer);
      this.pollingTimer = null;
      this.logger.info('Polling stopped');
    }
  }

  /**
   * Refresh data for a specific node
   */
  async refreshNodeData(nodeId: string): Promise<void> {
    const client = this.nodes.get(nodeId);
    if (!client) {
      throw new Error(`Node not found: ${nodeId}`);
    }
    
    try {
      this.logger.debug(`Refreshing data for node: ${nodeId}`);
      
      // Skip refreshing data for MockClient in cluster mode to prevent overriding socket assignments
      const clusterMode = process.env.PROXMOX_CLUSTER_MODE === 'true' || 
                         process.env.MOCK_CLUSTER_ENABLED === 'true' || 
                         process.env.MOCK_CLUSTER_MODE === 'true' ||
                         (process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true' && 
                          (process.env.USE_MOCK_DATA === 'true' || 
                           process.env.MOCK_DATA_ENABLED === 'true'));
      
      if (client instanceof MockClient && clusterMode) {
        this.logger.debug(`Skipping data refresh for MockClient in cluster mode: ${nodeId}`);
        this.logPolling(`SKIP: Skipping MockClient refresh in cluster mode for node ${nodeId}`);
        return;
      }
      
      // Get node status - check if method exists first
      let nodeStatus: ProxmoxNodeStatus;
      if (typeof client.getNodeStatus === 'function') {
        this.logger.debug(`Getting status for node: ${nodeId}`);
        nodeStatus = await client.getNodeStatus();
        this.logger.debug(`Received status for node: ${nodeId}`, { status: nodeStatus.status });
        
        // Update node status
        this.handleNodeStatusUpdate(nodeId, nodeStatus);
      } else {
        this.logger.warn(`Client doesn't support getNodeStatus method, using default status`);
        // Create a default status if the method doesn't exist
        nodeStatus = {
          id: nodeId,
          name: nodeId,
          configName: nodeId,
          status: 'online',
          uptime: 0,
          cpu: 0,
          memory: { total: 0, used: 0, free: 0, usedPercentage: 0 },
          swap: { total: 0, used: 0, free: 0, usedPercentage: 0 },
          disk: { total: 0, used: 0, free: 0, usedPercentage: 0 },
          loadAverage: [0, 0, 0],
          cpuInfo: { cores: 0, sockets: 0, model: '' }
        };
        this.handleNodeStatusUpdate(nodeId, nodeStatus);
      }
      
      // Only fetch VMs and containers if node is online
      if (nodeStatus.status === 'online') {
        // Get VMs
        let vms: ProxmoxVM[] = [];
        if (typeof client.getVirtualMachines === 'function') {
          this.logger.debug(`Getting VMs for node: ${nodeId}`);
          vms = await client.getVirtualMachines();
          this.logger.debug(`Received ${vms.length} VMs for node: ${nodeId}`);
          this.logPolling(`Fetched ${vms.length} VMs for node ${nodeId}`);
          this.handleVMListUpdate(nodeId, vms);
        } else {
          this.logger.warn(`Client doesn't support getVirtualMachines method, using empty array`);
        }
        
        // Get containers
        let containers: ProxmoxContainer[] = [];
        if (typeof client.getContainers === 'function') {
          this.logger.debug(`Getting containers for node: ${nodeId}`);
          containers = await client.getContainers();
          this.logger.debug(`Received ${containers.length} containers for node: ${nodeId}`);
          this.logPolling(`Fetched ${containers.length} containers for node ${nodeId}`);
          this.handleContainerListUpdate(nodeId, containers);
        } else {
          this.logger.warn(`Client doesn't support getContainers method, using empty array`);
        }
        
        // Log containers and their node assignments
        if (containers.length > 0) {
          const nodeAssignments: Record<string, string[]> = {};
          containers.forEach((container: ProxmoxContainer) => {
            if (!nodeAssignments[container.node]) {
              nodeAssignments[container.node] = [];
            }
            nodeAssignments[container.node].push(`${container.id} (${container.name})`);
          });
          
          this.logPolling(`Container node assignments for ${nodeId}:`);
          Object.keys(nodeAssignments).forEach(node => {
            this.logPolling(`  Node ${node}: ${nodeAssignments[node].length} containers`);
          });
        }
        
        // Emit metrics update event
        this.logger.debug(`Emitting metrics update for node: ${nodeId}`);
        this.emit('metricsUpdated', {
          nodeId,
          timestamp: Date.now(),
          nodeStatus,
          vms,
          containers
        });
      }
      
      this.logger.debug(`Refresh complete for node: ${nodeId}`);
      this.logPolling(`Refresh complete for node: ${nodeId}`);
    } catch (error) {
      this.logger.error(`Error refreshing node data: ${nodeId}`, { error });
      this.logPolling(`ERROR refreshing node ${nodeId}: ${error}`);
      
      // If we can't connect, mark the node as offline
      const currentStatus = this.nodeStatus.get(nodeId);
      if (currentStatus && currentStatus.status !== 'offline') {
        const offlineStatus: ProxmoxNodeStatus = {
          ...currentStatus,
          status: 'offline'
        };
        this.handleNodeStatusUpdate(nodeId, offlineStatus);
      }
    }
  }

  /**
   * Handle event from Proxmox
   */
  private handleEvent(nodeId: string, event: ProxmoxEvent): void {
    // Only process important events
    if (this.isImportantEvent(event)) {
      this.logger.debug(`Received important event: ${event.description}`, { event });
      
      // Emit event
      this.emit('event', event);
      
      // Refresh node data immediately if it's an important event
      // This makes the UI update much faster in response to events
      this.refreshNodeData(nodeId).catch(error => {
        this.logger.error(`Error refreshing node after event: ${nodeId}`, { error });
      });
    }
  }

  /**
   * Check if an event is important enough to trigger a refresh
   */
  private isImportantEvent(event: ProxmoxEvent): boolean {
    // VM or container status change events
    if (event.type === 'vm' || event.type === 'container') {
      const description = event.description.toLowerCase();
      return (
        description.includes('start') ||
        description.includes('stop') ||
        description.includes('shutdown') ||
        description.includes('reset') ||
        description.includes('resume') ||
        description.includes('suspend') ||
        description.includes('create') ||
        description.includes('delete') ||
        description.includes('migrate') ||
        description.includes('clone')
      );
    }
    
    // Also consider node events important
    if (event.type === 'node') {
      return true;
    }
    
    return false;
  }

  /**
   * Get all nodes
   */
  getNodes(): ProxmoxNodeStatus[] {
    return Array.from(this.nodeStatus.values());
  }

  /**
   * Get a specific node
   */
  getNode(nodeId: string): ProxmoxNodeStatus | undefined {
    return this.nodeStatus.get(nodeId);
  }

  /**
   * Get all VMs, optionally filtered by node
   */
  getVMs(nodeId?: string): ProxmoxVM[] {
    const vms = Array.from(this.vms.values());
    if (nodeId) {
      return vms.filter(vm => vm.node === nodeId);
    }
    return vms;
  }

  /**
   * Get a specific VM
   */
  getVM(vmId: string): ProxmoxVM | undefined {
    return this.vms.get(vmId);
  }

  /**
   * Get all containers, optionally filtered by node
   */
  getContainers(nodeId?: string): ProxmoxContainer[] {
    const containers = Array.from(this.containers.values());
    if (nodeId) {
      return containers.filter(container => container.node === nodeId);
    }
    return containers;
  }

  /**
   * Get a specific container
   */
  getContainer(containerId: string): ProxmoxContainer | undefined {
    return this.containers.get(containerId);
  }

  /**
   * Get all guests (VMs and containers), optionally filtered by node
   */
  getGuests(nodeId?: string): ProxmoxGuest[] {
    const vms = this.getVMs(nodeId);
    const containers = this.getContainers(nodeId);
    
    // Combine VMs and containers
    return [...vms, ...containers];
  }

  /**
   * Get a specific guest (VM or container)
   */
  getGuest(guestId: string): ProxmoxGuest | undefined {
    return this.vms.get(guestId) || this.containers.get(guestId);
  }

  /**
   * Shutdown the node manager
   */
  async shutdown(): Promise<void> {
    this.logger.info('Shutting down NodeManager');
    
    // Stop polling
    this.stopPolling();
    
    // Unsubscribe from all events
    for (const [nodeId, unsubscribe] of this.eventUnsubscribers.entries()) {
      this.logger.debug(`Unsubscribing from events for node: ${nodeId}`);
      unsubscribe();
    }
    
    this.eventUnsubscribers.clear();
    this.logger.info('NodeManager shutdown complete');
  }

  /**
   * Handle node status update
   */
  private handleNodeStatusUpdate(nodeId: string, status: ProxmoxNodeStatus): void {
    this.logger.debug(`Received node status update for ${nodeId}`);
    
    // Check if status changed
    const previousStatus = this.nodeStatus.get(nodeId);
    const statusChanged = !previousStatus || previousStatus.status !== status.status;
    
    // Update node status
    this.nodeStatus.set(nodeId, status);
    
    // Emit event if status changed
    if (statusChanged) {
      this.logger.debug(`Node status changed for ${nodeId}: ${previousStatus?.status || 'unknown'} -> ${status.status}`);
      this.emit('nodeStatusChanged', status);
    }
  }

  /**
   * Handle VM list update
   */
  private handleVMListUpdate(nodeId: string, vms: ProxmoxVM[]): void {
    this.logger.debug(`Received VM list update for ${nodeId}: ${vms.length} VMs`);
    
    // Update VMs
    for (const vm of vms) {
      const previousVM = this.vms.get(vm.id);
      const statusChanged = !previousVM || previousVM.status !== vm.status;
      
      this.vms.set(vm.id, vm);
      
      if (statusChanged) {
        this.logger.debug(`VM status changed for ${vm.id}: ${previousVM?.status || 'unknown'} -> ${vm.status}`);
        this.emit('guestStatusChanged', vm);
      }
    }
  }

  /**
   * Handle container list update
   */
  private handleContainerListUpdate(nodeId: string, containers: ProxmoxContainer[]): void {
    this.logger.debug(`Received container list update for ${nodeId}: ${containers.length} containers`);
    
    // Update containers
    for (const container of containers) {
      const previousContainer = this.containers.get(container.id);
      const statusChanged = !previousContainer || previousContainer.status !== container.status;
      
      this.containers.set(container.id, container);
      
      if (statusChanged) {
        this.logger.debug(`Container status changed for ${container.id}: ${previousContainer?.status || 'unknown'} -> ${container.status}`);
        this.emit('guestStatusChanged', container);
      }
    }
  }
}

// Export singleton instance
export const nodeManager = new NodeManager(); 