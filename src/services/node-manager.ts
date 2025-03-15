import { EventEmitter } from 'events';
import { ProxmoxClient } from '../api/proxmox-client';
import { MockClient } from '../api/mock-client';
import { createLogger } from '../utils/logger';
import config from '../config';
import { NodeConfig, ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxGuest, ProxmoxEvent } from '../types';

export class NodeManager extends EventEmitter {
  private nodes: Map<string, ProxmoxClient | MockClient> = new Map();
  private nodeStatus: Map<string, ProxmoxNodeStatus> = new Map();
  private vms: Map<string, ProxmoxVM> = new Map();
  private containers: Map<string, ProxmoxContainer> = new Map();
  private eventUnsubscribers: Map<string, () => void> = new Map();
  private logger = createLogger('NodeManager');
  private pollingInterval: NodeJS.Timeout | null = null;
  private isMockData: boolean = false;

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
    // Add all nodes from configuration
    for (const nodeConfig of config.nodes) {
      try {
        await this.addNode(nodeConfig);
      } catch (error) {
        this.logger.error(`Failed to initialize node ${nodeConfig.name}`, { error });
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
        client = new ProxmoxClient(nodeConfig, config.ignoreSSLErrors);
      }
      
      // Test connection
      const connected = await client.testConnection();
      
      if (!connected) {
        this.logger.error(`Failed to connect to node: ${nodeConfig.name}`);
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
      
      // Set up event polling
      client.setupEventPolling();
      
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
  private startPolling(): void {
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
    }
    
    this.logger.info(`Starting polling for node updates (interval: ${config.nodePollingIntervalMs}ms)`);
    
    // Use a more efficient polling approach that staggers requests to avoid overwhelming the system
    // and to make metrics more responsive
    this.pollingInterval = setInterval(async () => {
      const nodeIds = Array.from(this.nodes.keys());
      
      // If we have multiple nodes, stagger their polling to avoid all nodes being polled at once
      // This helps prevent unrealistic rate calculations due to timing issues
      if (nodeIds.length > 1) {
        // Process one node at a time with a small delay between each
        for (let i = 0; i < nodeIds.length; i++) {
          const nodeId = nodeIds[i];
          try {
            await this.refreshNodeData(nodeId);
            
            // Add a small delay between node polls to avoid timing issues
            // but only if there are more nodes to process
            if (i < nodeIds.length - 1) {
              await new Promise(resolve => setTimeout(resolve, 200));
            }
          } catch (error) {
            this.logger.error(`Error polling node: ${nodeId}`, { error });
          }
        }
      } else if (nodeIds.length === 1) {
        // If we only have one node, just poll it directly
        try {
          await this.refreshNodeData(nodeIds[0]);
        } catch (error) {
          this.logger.error(`Error polling node: ${nodeIds[0]}`, { error });
        }
      }
    }, config.nodePollingIntervalMs);
  }

  /**
   * Stop polling for updates
   */
  stopPolling(): void {
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
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
      
      // Get node status
      this.logger.debug(`Getting status for node: ${nodeId}`);
      const nodeStatus = await client.getNodeStatus();
      this.logger.debug(`Received status for node: ${nodeId}`, { status: nodeStatus.status });
      
      // Update node status
      this.handleNodeStatusUpdate(nodeId, nodeStatus);
      
      // Only fetch VMs and containers if node is online
      if (nodeStatus.status === 'online') {
        // Check if we should use the cluster resources endpoint
        const useClusterResources = config.useClusterResourcesEndpoint && 
                                    config.clusterMode && 
                                    client instanceof ProxmoxClient;
        
        if (useClusterResources) {
          this.logger.info(`Using cluster resources endpoint for node: ${nodeId}`);
          
          try {
            // Get all resources from the cluster endpoint
            const proxmoxClient = client as ProxmoxClient;
            const clusterResources = await proxmoxClient.getClusterResources();
            
            // Process VMs from cluster resources
            this.logger.debug(`Received ${clusterResources.vms.length} VMs from cluster resources`);
            
            // Process all nodes' VMs through this single node
            for (const nodeConfig of config.nodes) {
              const nodeVMs = clusterResources.vms.filter((vm: ProxmoxVM) => vm.node === nodeConfig.name);
              this.handleVMListUpdate(nodeConfig.id, nodeVMs);
            }
            
            // Process containers from cluster resources
            this.logger.debug(`Received ${clusterResources.containers.length} containers from cluster resources`);
            
            // Process all nodes' containers through this single node
            for (const nodeConfig of config.nodes) {
              const nodeContainers = clusterResources.containers.filter((container: ProxmoxContainer) => container.node === nodeConfig.name);
              this.handleContainerListUpdate(nodeConfig.id, nodeContainers);
            }
            
            // Emit metrics update event
            this.logger.debug(`Emitting cluster metrics update`);
            this.emit('metricsUpdated', {
              nodeId,
              timestamp: Date.now(),
              nodeStatus,
              vms: clusterResources.vms,
              containers: clusterResources.containers
            });
          } catch (error) {
            this.logger.error(`Error using cluster resources endpoint for node: ${nodeId}`, { error });
            
            // Fall back to standard approach if cluster resources fails
            this.logger.warn(`Falling back to standard node-by-node approach for ${nodeId}`);
            await this.fetchNodeResourcesStandard(client, nodeId, nodeStatus);
          }
        } else {
          // Standard approach - get VMs and containers for just this node
          await this.fetchNodeResourcesStandard(client, nodeId, nodeStatus);
        }
      }
      
      this.logger.debug(`Refresh complete for node: ${nodeId}`);
    } catch (error) {
      this.logger.error(`Error refreshing node data: ${nodeId}`, { error });
      
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
   * Fetch node resources using the standard node-by-node approach
   */
  private async fetchNodeResourcesStandard(
    client: ProxmoxClient | MockClient, 
    nodeId: string, 
    nodeStatus: ProxmoxNodeStatus
  ): Promise<void> {
    // Get VMs
    this.logger.debug(`Getting VMs for node: ${nodeId}`);
    let vms: ProxmoxVM[] = [];
    
    // Handle different client types
    if (client instanceof ProxmoxClient) {
      vms = await client.getVirtualMachines();
    } else {
      vms = await client.getVMs();
    }
    
    this.logger.debug(`Received ${vms.length} VMs for node: ${nodeId}`);
    this.handleVMListUpdate(nodeId, vms);
    
    // Get containers
    this.logger.debug(`Getting containers for node: ${nodeId}`);
    const containers = await client.getContainers();
    this.logger.debug(`Received ${containers.length} containers for node: ${nodeId}`);
    this.handleContainerListUpdate(nodeId, containers);
    
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