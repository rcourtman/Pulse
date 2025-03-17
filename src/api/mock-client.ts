import { EventEmitter } from 'events';
import { io, Socket } from 'socket.io-client';
import { createLogger } from '../utils/logger';
import { NodeConfig, ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxEvent } from '../types';
import config from '../config';
import axios from 'axios';
import * as fs from 'fs';
import * as path from 'path';

/**
 * Mock client for Proxmox API
 * This client connects to the mock data server instead of a real Proxmox server
 */
export class MockClient extends EventEmitter {
  private config: NodeConfig;
  private logger = createLogger('MockClient');
  private mockServerUrl = `http://localhost:${process.env.MOCK_SERVER_PORT || '7656'}`;
  private socket: Socket | null = null;
  private connected = false;
  private pollingInterval: NodeJS.Timeout | null = null;
  private mockVMs: ProxmoxVM[] = [];
  private mockContainers: ProxmoxContainer[] = [];
  private mockClusterName: string = process.env.MOCK_CLUSTER_NAME || 'mock-cluster';

  /**
   * Helper function to extract vmid from Proxmox-style ID
   * Examples:
   *   "qemu/101" -> "101"
   *   "qemu/101-node-node-1" -> "101"
   *   "lxc/201" -> "201"
   *   101 -> "101"
   */
  private getVmidFromId(id: string | number): string {
    if (typeof id === 'number') return String(id);
    
    // If it's in Proxmox format like "qemu/101", extract just the ID part
    const parts = String(id).split('/');
    if (parts.length <= 1) return String(id);
    
    // Handle node-specific IDs with various formats:
    // - "qemu/101-node-node-1" (old format)
    // - "qemu/101:node-1" (new format)
    const idPart = parts[1];
    const nodeSpecificParts = idPart.includes('-node-') 
      ? idPart.split('-node-') 
      : idPart.split(':');
    
    // Return just the numeric part
    return nodeSpecificParts[0];
  }

  // Add file logger for diagnostic tracking
  private setupDiagnosticLogger() {
    const logDir = path.join(process.cwd(), 'logs');
    if (!fs.existsSync(logDir)) {
      fs.mkdirSync(logDir, { recursive: true });
    }
    
    const logFile = path.join(logDir, 'guest-assignments.log');
    this.logger.info(`Setting up diagnostic logger to ${logFile}`);
    
    // Clear the log file on startup
    fs.writeFileSync(logFile, `=== Guest Assignment Log Started at ${new Date().toISOString()} ===\n\n`);
    
    return (message: string) => {
      const timestamp = new Date().toISOString();
      fs.appendFileSync(logFile, `[${timestamp}] ${message}\n`);
    };
  }

  private logAssignment = this.setupDiagnosticLogger();

  constructor(config: NodeConfig) {
    super();
    this.config = config;
    this.logger.info(`MockClient created for node: ${config.name}`);
    
    // Initialize empty arrays - will be populated from server data
    this.mockVMs = [];
    this.mockContainers = [];
    
    // Log initial setup
    this.logAssignment(`Client created for node: ${config.name} (${config.id}), isClusterEntryPoint: ${config.isClusterEntryPoint || false}`);
  }

  /**
   * Check if the node is part of a cluster (mock implementation)
   * @returns Object containing isCluster (boolean) and clusterName (string)
   */
  async isNodeInCluster(): Promise<{ isCluster: boolean; clusterName: string }> {
    // Simulate a delay to mimic network request
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Determine if we're in cluster mode by checking all environment variables
    const clusterMode = process.env.PROXMOX_CLUSTER_MODE === 'true' || 
                       process.env.MOCK_CLUSTER_ENABLED === 'true' || 
                       process.env.MOCK_CLUSTER_MODE === 'true' ||
                       (process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true' && 
                        (process.env.USE_MOCK_DATA === 'true' || 
                         process.env.MOCK_DATA_ENABLED === 'true'));
                       
    // Also check if this specific node is a cluster entry point
    const isEntryPoint = this.config.isClusterEntryPoint === true;
    
    this.logger.debug(`Cluster mode detection: environment=${clusterMode}, node entry point=${isEntryPoint}`);
    
    const isCluster = clusterMode || isEntryPoint;
    
    if (isCluster) {
      this.logger.info(`Mock node is part of cluster: ${this.mockClusterName}`);
      return { isCluster: true, clusterName: this.mockClusterName };
    } else {
      this.logger.info('Mock node is not part of a cluster');
      return { isCluster: false, clusterName: '' };
    }
  }

  /**
   * Private helper to check if we're in cluster mode
   */
  private isInClusterMode(): boolean {
    const clusterMode = process.env.PROXMOX_CLUSTER_MODE === 'true' || 
                       process.env.MOCK_CLUSTER_ENABLED === 'true' || 
                       process.env.MOCK_CLUSTER_MODE === 'true' ||
                       (process.env.PROXMOX_AUTO_DETECT_CLUSTER === 'true' && 
                        (process.env.USE_MOCK_DATA === 'true' || 
                         process.env.MOCK_DATA_ENABLED === 'true'));
                       
    const isEntryPoint = this.config.isClusterEntryPoint === true;
    
    return clusterMode || isEntryPoint;
  }

  /**
   * Connect to the mock server
   */
  async connect(): Promise<boolean> {
    try {
      this.logger.info(`Connecting to mock server at ${this.mockServerUrl}`);
      
      // Create a socket connection
      this.socket = io(this.mockServerUrl, {
        transports: ['websocket'],
        reconnection: true,
        reconnectionDelay: 1000,
        reconnectionDelayMax: 5000,
        reconnectionAttempts: Infinity
      });
      
      // Wait for the socket to connect
      await new Promise<void>((resolve, reject) => {
        this.socket!.on('connect', () => {
          this.logger.info('Connected to mock server');
          this.connected = true;
          resolve();
        });
        
        this.socket!.on('connect_error', (error) => {
          this.logger.error(`Error connecting to mock server: ${error.message}`);
          reject(error);
        });
        
        // Set a timeout to reject if connection takes too long
        setTimeout(() => {
          reject(new Error('Connection timeout'));
        }, 5000);
      });
      
      // Set up socket event listeners
      this.setupSocketEvents();
      
      // Check if this is a cluster mode connection
      const isClusterMode = this.isInClusterMode();
      
      // Get the node ID to register with the mock server
      const nodeId = this.config.id;
      const nodeName = this.config.name;
      const isClusterEntryPoint = this.config.isClusterEntryPoint === true;
      
      // Register with the mock server
      this.logger.info(`Registering with server as nodeId=${nodeId}, nodeName=${nodeName}, cluster entry point: ${isClusterEntryPoint}, cluster mode: ${isClusterMode}`);
      this.socket!.emit('registerNode', { 
        nodeId, 
        nodeName,
        isClusterEntryPoint,
        isClusterMode
      });
      
      // Do NOT call setupEventPolling here - it would create a circular dependency
      
      return true;
    } catch (error) {
      this.logger.error(`Failed to connect to mock server: ${error}`);
      this.socket = null;
      this.connected = false;
      return false;
    }
  }

  /**
   * Disconnect from the mock server
   */
  disconnect(): void {
    if (this.socket) {
      this.socket.disconnect();
      this.socket = null;
    }
    
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
    }
    
    this.connected = false;
    this.logger.info('Disconnected from mock server');
  }

  /**
   * Test connection to the mock server
   */
  async testConnection(): Promise<boolean> {
    try {
      if (!this.connected) {
        return await this.connect();
      }
      return true;
    } catch (error) {
      this.logger.error('Error testing connection to mock server', { error });
      return false;
    }
  }

  /**
   * Get node status
   */
  async getNodeStatus(): Promise<ProxmoxNodeStatus> {
    // Get current node configuration
    const currentNodeId = this.config.id;
    const currentNodeName = this.config.name;
    
    // If connected to mock server, request status
    if (this.socket && this.connected) {
      this.socket.emit('getNodeStatus', { nodeId: currentNodeId });
    }
    
    // Return a mock status object
    return {
      id: currentNodeId,
      name: currentNodeName,
      configName: currentNodeName,
      status: this.connected ? 'online' : 'offline',
      uptime: 3600 * 24 * 3, // 3 days
      cpu: 0.15, // 15% CPU usage
      memory: {
        total: 16 * 1024 * 1024 * 1024, // 16GB
        used: 4 * 1024 * 1024 * 1024,   // 4GB
        free: 12 * 1024 * 1024 * 1024,  // 12GB
        usedPercentage: 25              // 25%
      },
      swap: {
        total: 4 * 1024 * 1024 * 1024,  // 4GB
        used: 512 * 1024 * 1024,        // 512MB
        free: 3.5 * 1024 * 1024 * 1024, // 3.5GB
        usedPercentage: 12.5            // 12.5%
      },
      disk: {
        total: 500 * 1024 * 1024 * 1024, // 500GB
        used: 100 * 1024 * 1024 * 1024,  // 100GB
        free: 400 * 1024 * 1024 * 1024,  // 400GB
        usedPercentage: 20               // 20%
      },
      loadAverage: [0.1, 0.15, 0.2],
      cpuInfo: {
        cores: 8,
        sockets: 1,
        model: 'Mock CPU @ 3.5GHz'
      }
    };
  }

  /**
   * Get list of virtual machines
   */
  async getVirtualMachines(): Promise<ProxmoxVM[]> {
    this.logger.debug(`Returning ${this.mockVMs.length} VMs with the following node assignments:`);
    const nodeAssignments = new Map<string, string[]>();
    
    // Group VMs by node for better logging
    this.mockVMs.forEach(vm => {
      if (!nodeAssignments.has(vm.node)) {
        nodeAssignments.set(vm.node, []);
      }
      nodeAssignments.get(vm.node)?.push(vm.id);
    });
    
    // Log the groups
    for (const [node, vms] of nodeAssignments.entries()) {
      this.logger.debug(`Node ${node}: ${vms.length} VMs: ${vms.join(', ')}`);
    }
    
    return this.mockVMs;
  }

  /**
   * Get list of containers
   */
  async getContainers(): Promise<ProxmoxContainer[]> {
    this.logger.debug(`Returning ${this.mockContainers.length} containers with the following node assignments:`);
    const nodeAssignments = new Map<string, string[]>();
    
    // Group containers by node for better logging
    this.mockContainers.forEach(container => {
      if (!nodeAssignments.has(container.node)) {
        nodeAssignments.set(container.node, []);
      }
      nodeAssignments.get(container.node)?.push(container.id);
    });
    
    // Log the groups
    for (const [node, containers] of nodeAssignments.entries()) {
      this.logger.debug(`Node ${node}: ${containers.length} containers: ${containers.join(', ')}`);
    }
    
    return this.mockContainers;
  }

  /**
   * Set up event polling
   */
  setupEventPolling(): void {
    this.logger.info('Event polling set up via socket.io connection');
    
    // Don't attempt to connect if we're already connected or in the process of connecting
    if (!this.connected && !this.socket) {
      this.connect().catch(error => {
        this.logger.error('Failed to connect during setupEventPolling', { error });
      });
    }
    
    // Clear any existing polling interval
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
    }
    
    // Don't set up periodic API polling in cluster mode - rely on socket updates
    const isClusterMode = this.isInClusterMode();
    if (isClusterMode) {
      this.logger.info('In cluster mode - disabling periodic API polling to prevent conflicts with socket updates');
      return;
    }
    
    // Set up periodic API polling to refresh data from endpoints
    // This ensures we have fresh data even if socket updates fail
    this.pollingInterval = setInterval(async () => {
      try {
        // Only fetch from API in non-cluster mode
        this.logger.debug('Polling cluster resources API endpoint for fresh data');
        // Get VMs and containers from API
        await this.getVirtualMachines();
        await this.getContainers();
        
        // Emit events with the updated data
        this.emit('vmList', this.mockVMs);
        this.emit('containerList', this.mockContainers);
      } catch (error) {
        this.logger.error('Error during API polling', { error });
      }
    }, 10000); // Poll every 10 seconds
  }

  /**
   * Stop event polling
   */
  stopEventPolling(): void {
    // No need to do anything here since we're using socket.io events
    this.logger.info('Event polling stopped');
    
    // Disconnect if connected
    if (this.connected) {
      this.disconnect();
    }
  }

  /**
   * Set up socket event listeners
   */
  private setupSocketEvents(): void {
    if (!this.socket) {
      this.logger.error('Cannot set up socket events: no socket connection');
      return;
    }

    const currentNodeId = this.config.id;
    const currentNodeName = this.config.name;
    
    this.socket.on('disconnect', () => {
      this.logger.info('Disconnected from mock server');
      this.connected = false;
    });
    
    this.socket.on('error', (error) => {
      this.logger.error('Socket error', { error });
    });
    
    // Add handler for when we first receive the guests list
    this.socket.on('connect', () => {
      this.logger.info('Socket connected - waiting for initial guest data');
      
      // Log existing assignments right after connection
      if (this.mockVMs.length > 0 || this.mockContainers.length > 0) {
        this.logger.info('Current guest node assignments on connect:');
        
        const nodeVmCounts = new Map<string, number>();
        const nodeContainerCounts = new Map<string, number>();
        
        // Count VMs per node
        this.mockVMs.forEach(vm => {
          if (!nodeVmCounts.has(vm.node)) {
            nodeVmCounts.set(vm.node, 0);
          }
          nodeVmCounts.set(vm.node, nodeVmCounts.get(vm.node)! + 1);
        });
        
        // Count containers per node
        this.mockContainers.forEach(container => {
          if (!nodeContainerCounts.has(container.node)) {
            nodeContainerCounts.set(container.node, 0);
          }
          nodeContainerCounts.set(container.node, nodeContainerCounts.get(container.node)! + 1);
        });
        
        // Log VM summary by node
        for (const [node, count] of nodeVmCounts.entries()) {
          this.logger.info(`Node ${node}: ${count} VMs`);
        }
        
        // Log container summary by node
        for (const [node, count] of nodeContainerCounts.entries()) {
          this.logger.info(`Node ${node}: ${count} containers`);
        }
      }
    });
    
    this.socket.on('guests', (data) => {
      try {
        // Log the received data
        this.logger.info(`Received ${data.guests.length} guests from mock server`);
        
        // First, analyze what nodes are represented in the data
        const nodeIds = [...new Set(data.guests.map((g: any) => g.nodeName || g.nodeId || g.node))];
        this.logger.info(`Guest node IDs found in data: ${nodeIds.join(', ')}`);
        
        // Group guests by node
        const guestsByNode: Record<string, any[]> = {};
        
        // Initialize empty arrays for each node
        nodeIds.forEach(nodeId => {
          if (nodeId) {
            guestsByNode[nodeId as string] = [];
          }
        });
        
        // Assign each guest to its correct node
        data.guests.forEach((guest: any) => {
          const nodeId = guest.nodeName || guest.nodeId || guest.node;
          if (nodeId && guestsByNode[nodeId]) {
            guestsByNode[nodeId].push(guest);
          } else if (nodeId) {
            // If this is a new node we haven't seen before
            guestsByNode[nodeId] = [guest];
          } else {
            this.logger.warn(`Guest ${String(guest.id)} has no node reference, skipping`);
          }
        });
        
        // Log the distribution to the diagnostic log
        this.logAssignment(`RECEIVED GUESTS BY NODE:`);
        Object.entries(guestsByNode).forEach(([nodeId, guests]) => {
          this.logAssignment(`  - Node ${nodeId}: ${guests.length} guests`);
          this.logger.info(`Node ${nodeId}: ${guests.length} guests`);
          
          // Log the guest IDs for each node
          const guestIds = guests.map(g => String(g.id)).join(', ');
          this.logAssignment(`    Guest IDs: ${guestIds}`);
        });
        
        // Clear existing guests to avoid duplication
        this.mockVMs = [];
        this.mockContainers = [];
        
        // Process and store each node's guests separately
        let totalVMs = 0;
        let totalContainers = 0;
        
        for (const [nodeId, nodeGuests] of Object.entries(guestsByNode)) {
          const result = this.processGuestsForNode(nodeGuests, nodeId);
          
          // Add the VMs and containers to our collections
          this.mockVMs.push(...result.vms);
          this.mockContainers.push(...result.containers);
          
          totalVMs += result.vms.length;
          totalContainers += result.containers.length;
          
          this.logger.info(`Processed ${result.vms.length} VMs and ${result.containers.length} containers for node ${nodeId}`);
        }
        
        // Log the total processed
        this.logger.info(`Total processed: ${totalVMs} VMs and ${totalContainers} containers`);
        this.logAssignment(`AFTER PROCESSING: ${totalVMs} VMs, ${totalContainers} containers`);
        
        // Log detailed node assignments
        this.logAssignment(`FINAL ASSIGNMENTS:`);
        const finalNodeGroups = new Map<string, { vms: number, containers: number }>();
        
        // Group VMs by node
        this.mockVMs.forEach(vm => {
          const nodeId = vm.node;
          if (!finalNodeGroups.has(nodeId)) {
            finalNodeGroups.set(nodeId, { vms: 0, containers: 0 });
          }
          finalNodeGroups.get(nodeId)!.vms++;
        });
        
        // Group containers by node
        this.mockContainers.forEach(container => {
          const nodeId = container.node;
          if (!finalNodeGroups.has(nodeId)) {
            finalNodeGroups.set(nodeId, { vms: 0, containers: 0 });
          }
          finalNodeGroups.get(nodeId)!.containers++;
        });
        
        // Log the final distribution
        finalNodeGroups.forEach((counts, nodeId) => {
          this.logAssignment(`  - Node ${nodeId}: ${counts.vms} VMs, ${counts.containers} containers`);
        });
        
        // Emit events to notify subscribers
        this.emit('vmList', this.mockVMs);
        this.emit('containerList', this.mockContainers);
        
        // Combine both types of guests for a single guests event
        const allGuests = [...this.mockVMs, ...this.mockContainers];
        this.emit('guests', allGuests);
      } catch (error) {
        this.logger.error('Error processing guests data', { error });
      }
    });
  }

  // Add a function to detect and log node changes
  private trackNodeChanges(newGuests: any[], source: string): void {
    const currentVMMap = new Map<string, string>();
    const currentContainerMap = new Map<string, string>();
    
    // Build maps of current assignments by vmid
    this.mockVMs.forEach(vm => {
      const vmidStr = this.getVmidFromId(vm.id);
      currentVMMap.set(vmidStr, vm.node);
    });
    
    this.mockContainers.forEach(container => {
      const vmidStr = this.getVmidFromId(container.id);
      currentContainerMap.set(vmidStr, container.node);
    });
    
    // Check each new guest for node changes - use vmid for comparison
    newGuests.forEach(guest => {
      const vmidStr = String(typeof guest.id === 'number' ? guest.id : this.getVmidFromId(guest.id));
      const newNode = guest.node;
      const guestType = guest.type === 'qemu' || guest.type === 'vm' ? 'VM' : 'Container';
      
      if (guestType === 'VM' && currentVMMap.has(vmidStr)) {
        const oldNode = currentVMMap.get(vmidStr);
        if (oldNode !== newNode) {
          this.logger.warn(`[${source}] NODE CHANGE DETECTED: ${guestType} ${vmidStr} (${guest.name}) moved from ${oldNode} to ${newNode}`);
        }
      } else if (guestType === 'Container' && currentContainerMap.has(vmidStr)) {
        const oldNode = currentContainerMap.get(vmidStr);
        if (oldNode !== newNode) {
          this.logger.warn(`[${source}] NODE CHANGE DETECTED: ${guestType} ${vmidStr} (${guest.name}) moved from ${oldNode} to ${newNode}`);
        }
      }
    });
  }

  private async fetchContainers(): Promise<ProxmoxContainer[]> {
    try {
      this.logger.info('Using cluster resources endpoint to get containers from all nodes');
      const response = await axios.get(`${this.mockServerUrl}/api2/json/cluster/resources`, {
        params: { type: 'lxc' }
      });

      if (!response.data || !response.data.data || !Array.isArray(response.data.data)) {
        this.logger.warn('Invalid response format from cluster resources endpoint');
        return this.mockContainers; // Fall back to cached containers
      }
      
      const data = response.data.data;
      
      // Filter to only get containers (not nodes or other resources)
      const containers = data
        .filter((item: any) => item.type === 'lxc')
        .map((container: any) => {
          // Get the vmid
          const vmid = container.vmid || parseInt(String(container.id).replace(/\D/g, ''), 10);
          
          // Format the ID in Proxmox format
          // Don't add node-specific suffix for API data - these have reliable node assignments
          const proxmoxId = `lxc/${vmid}`;
          
          return {
            id: proxmoxId, // Use standard Proxmox format ID
            name: container.name,
            status: container.status,
            node: container.node,
            vmid: vmid,
            type: 'lxc',
            cpus: container.maxcpu || 1,
            cpu: container.cpu || 0,
            memory: container.mem || 0,
            maxmem: container.maxmem || 1024 * 1024 * 1024,
            disk: container.disk || 0,
            maxdisk: container.maxdisk || 10 * 1024 * 1024 * 1024,
            uptime: container.uptime || 0,
            netin: 0,
            netout: 0,
            diskread: 0,
            diskwrite: 0,
            template: false
          } as ProxmoxContainer;
        });
      
      // Debug log for container node assignments
      this.logger.debug('Containers fetched from cluster resources endpoint:');
      containers.forEach((container: ProxmoxContainer) => {
        this.logger.debug(`Container ${container.id} (${container.name}) assigned to node: ${container.node}`);
      });
      
      this.logger.info(`Retrieved ${containers.length} containers from cluster resources endpoint`);
      return containers;
    } catch (error) {
      this.logger.error('Error fetching containers:', { error });
      return [];
    }
  }

  /**
   * Fetch virtual machines from the API endpoint
   * This is used for API polling, not socket updates
   */
  private async fetchVirtualMachines(): Promise<ProxmoxVM[]> {
    try {
      this.logger.info('Using cluster resources endpoint to get VMs from all nodes');
      const response = await axios.get(`${this.mockServerUrl}/api2/json/cluster/resources`, {
        params: { type: 'qemu' }
      });

      if (!response.data || !response.data.data || !Array.isArray(response.data.data)) {
        this.logger.warn('Invalid response format from cluster resources endpoint');
        return this.mockVMs; // Fall back to cached VMs
      }
      
      const data = response.data.data;
      
      // Filter to only get VMs (not nodes or other resources)
      const vms = data
        .filter((item: any) => item.type === 'qemu')
        .map((vm: any) => {
          // Get the vmid
          const vmid = vm.vmid || parseInt(String(vm.id).replace(/\D/g, ''), 10);
          
          // Format the ID in Proxmox format
          // Don't add node-specific suffix for API data - these have reliable node assignments
          const proxmoxId = `qemu/${vmid}`;
          
          return {
            id: proxmoxId, // Use standard Proxmox format ID
            name: vm.name,
            status: vm.status,
            node: vm.node,
            vmid: vmid,
            type: 'qemu',
            cpus: vm.maxcpu || 1,
            cpu: vm.cpu || 0,
            memory: vm.mem || 0,
            maxmem: vm.maxmem || 2 * 1024 * 1024 * 1024,
            disk: vm.disk || 0,
            maxdisk: vm.maxdisk || 20 * 1024 * 1024 * 1024,
            uptime: vm.uptime || 0,
            netin: 0,
            netout: 0,
            diskread: 0,
            diskwrite: 0,
            template: false
          } as ProxmoxVM;
        });
      
      // Debug log for VM node assignments
      this.logger.debug('VMs fetched from cluster resources endpoint:');
      vms.forEach((vm: ProxmoxVM) => {
        this.logger.debug(`VM ${vm.id} (${vm.name}) assigned to node: ${vm.node}`);
      });
      
      this.logger.info(`Retrieved ${vms.length} VMs from cluster resources endpoint`);
      return vms;
    } catch (error) {
      this.logger.error('Error fetching VMs:', { error });
      return [];
    }
  }

  private async processNodeResources(): Promise<void> {
    this.logger.warn('⚠️ POLL MECHANISM ACTIVE - this may override socket assignments');
    this.logAssignment('API POLL: Starting API polling for node resources');
    
    try {
      // Get VMs from cluster resources endpoint
      const vms = await this.fetchVirtualMachines();
      
      // Get containers from cluster resources endpoint
      const containers = await this.fetchContainers();
      
      // Debug mappings of current assignments - use vmid for keys
      const currentNodeMap = new Map<string, string>();
      [...this.mockVMs, ...this.mockContainers].forEach(guest => {
        const vmidStr = this.getVmidFromId(guest.id);
        currentNodeMap.set(vmidStr, guest.node);
      });
      
      // Log differences to find inconsistencies
      let changesDetected = 0;
      [...vms, ...containers].forEach(guest => {
        const vmidStr = this.getVmidFromId(guest.id);
        if (currentNodeMap.has(vmidStr)) {
          const oldNode = currentNodeMap.get(vmidStr);
          if (oldNode !== guest.node) {
            changesDetected++;
            this.logger.warn(`⚠️ POLL OVERRIDE: Guest ${vmidStr} would move from ${oldNode} to ${guest.node}`);
            this.logAssignment(`POLL CONFLICT: Guest ${vmidStr} (${guest.name}) would move from ${oldNode} to ${guest.node}`);
          }
        }
      });
      
      if (changesDetected > 0) {
        this.logAssignment(`API POLL: Detected ${changesDetected} guests with node changes`);
      } else {
        this.logAssignment(`API POLL: No node changes detected`);
      }

      // Always preserve existing containers and their node assignments
      if (this.mockContainers.length > 0) {
        const existingContainerMap = new Map<string, ProxmoxContainer>();
        this.mockContainers.forEach(container => {
          const vmidStr = this.getVmidFromId(container.id);
          existingContainerMap.set(vmidStr, container);
        });
        
        // Only update container properties, but preserve node assignment
        let containerPreservations = 0;
        containers.forEach((container: ProxmoxContainer) => {
          const vmidStr = this.getVmidFromId(container.id);
          if (existingContainerMap.has(vmidStr)) {
            const existingContainer = existingContainerMap.get(vmidStr)!;
            const existingNode = existingContainer.node;
            
            // Log if node assignment would change and always preserve existing assignments
            if (existingNode !== container.node) {
              containerPreservations++;
              this.logger.warn(`⚠️ POLL BLOCK: Preserving container ${vmidStr} (${container.name}): API says ${container.node}, keeping ${existingNode}`);
              this.logAssignment(`POLL PRESERVE: Container ${vmidStr} (${container.name}): API says ${container.node}, keeping ${existingNode}`);
              container.node = existingNode;
            }
          }
        });
        
        if (containerPreservations > 0) {
          this.logAssignment(`API POLL: Preserved ${containerPreservations} container node assignments`);
        }
      }
      
      // Similarly for VMs - preserve their node assignments too
      if (this.mockVMs.length > 0) {
        const existingVMMap = new Map<string, ProxmoxVM>();
        this.mockVMs.forEach(vm => {
          const vmidStr = this.getVmidFromId(vm.id);
          existingVMMap.set(vmidStr, vm);
        });
        
        // Only update VM properties, but preserve node assignment
        let vmPreservations = 0;
        vms.forEach((vm: ProxmoxVM) => {
          const vmidStr = this.getVmidFromId(vm.id);
          if (existingVMMap.has(vmidStr)) {
            const existingVM = existingVMMap.get(vmidStr)!;
            const existingNode = existingVM.node;
            
            // Log if node assignment would change and always preserve existing assignments
            if (existingNode !== vm.node) {
              vmPreservations++;
              this.logger.warn(`⚠️ POLL BLOCK: Preserving VM ${vmidStr} (${vm.name}): API says ${vm.node}, keeping ${existingNode}`);
              this.logAssignment(`POLL PRESERVE: VM ${vmidStr} (${vm.name}): API says ${vm.node}, keeping ${existingNode}`);
              vm.node = existingNode;
            }
          }
        });
        
        if (vmPreservations > 0) {
          this.logAssignment(`API POLL: Preserved ${vmPreservations} VM node assignments`);
        }
      }
      
      // Now set the updated collections - but ONLY if this is the first time (empty collections)
      const nodeId = this.config.id;
      const nodeName = this.config.name;
      
      if (this.mockVMs.length === 0 && this.mockContainers.length === 0) {
        // First load, go ahead and set collections
        this.logger.info('First load - setting initial guest collections');
        this.logAssignment(`FIRST LOAD: Setting initial guest collections`);
        
        if (this.isInClusterMode() && this.config.isClusterEntryPoint) {
          this.mockVMs = vms;
          this.mockContainers = containers;
          this.logger.info('Node is a cluster entry point - including all guests from all nodes');
          this.logAssignment(`CLUSTER MODE: Including all guests from all nodes as cluster entry point`);
        } else {
          // Filter for specific node in non-cluster mode
          this.mockVMs = vms.filter(vm => vm.node === nodeId || vm.node === nodeName);
          this.mockContainers = containers.filter(container => container.node === nodeId || container.node === nodeName);
          this.logAssignment(`NON-CLUSTER MODE: Filtered guests for node ${nodeName} (${nodeId})`);
        }
        
        // Force a refresh of the data
        this.emit('vmList', this.mockVMs);
        this.emit('containerList', this.mockContainers);
        
        // Log initial assignments
        const initialNodeGroups = new Map<string, { vms: number, containers: number }>();
        this.mockVMs.forEach(vm => {
          const nodeId = vm.node;
          if (!initialNodeGroups.has(nodeId)) {
            initialNodeGroups.set(nodeId, { vms: 0, containers: 0 });
          }
          initialNodeGroups.get(nodeId)!.vms++;
        });
        
        this.mockContainers.forEach(container => {
          const nodeId = container.node;
          if (!initialNodeGroups.has(nodeId)) {
            initialNodeGroups.set(nodeId, { vms: 0, containers: 0 });
          }
          initialNodeGroups.get(nodeId)!.containers++;
        });
        
        this.logAssignment(`INITIAL ASSIGNMENTS:`);
        initialNodeGroups.forEach((counts, nodeId) => {
          this.logAssignment(`  - Node ${nodeId}: ${counts.vms} VMs, ${counts.containers} containers`);
        });
      } else {
        this.logger.info('Skipping collection updates from processNodeResources to avoid overriding socket assignments');
        this.logAssignment(`API POLL: Skipping guest updates to avoid overriding socket assignments`);
      }
    } catch (error) {
      this.logger.error('Error processing node resources:', { error });
      this.logAssignment(`ERROR: Failed to process node resources: ${error}`);
    }
  }

  /**
   * Helper function to detect if a guest is shared across multiple nodes
   * 
   * In the mock data, we have the 'shared' property explicitly set.
   * However, we can also detect shared guests by looking for guests with the same VMID
   * that appear on multiple nodes.
   */
  private isGuestShared(guest: any, guestsByVmid: Map<string, string[]>): boolean {
    // If the guest has an explicit 'shared' property, use that
    if (guest.shared !== undefined) {
      return guest.shared;
    }
    
    // Otherwise, check if this guest's vmid appears on multiple nodes
    const vmid = typeof guest.id === 'number' ? 
      String(guest.id) : 
      this.getVmidFromId(guest.id);
      
    // If we don't have a map of nodes by vmid, consider it not shared
    if (!guestsByVmid.has(vmid)) {
      return false;
    }
    
    // If this vmid appears on more than one node, it's shared
    return guestsByVmid.get(vmid)!.length > 1;
  }
  
  private processGuestsForNode(guests: any[], nodeId: string): { vms: ProxmoxVM[], containers: ProxmoxContainer[] } {
    const vms: ProxmoxVM[] = [];
    const containers: ProxmoxContainer[] = [];
    
    // First build a map of vmids to the nodes they appear on
    const guestsByVmid = new Map<string, string[]>();
    
    // Analyze all guests to find which ones appear on multiple nodes
    guests.forEach((guest: any) => {
      const vmid = typeof guest.id === 'number' ? 
        String(guest.id) : 
        this.getVmidFromId(guest.id);
        
      if (!guestsByVmid.has(vmid)) {
        guestsByVmid.set(vmid, []);
      }
      
      // Record the node this guest appears on
      const guestNodeId = guest.node || guest.nodeId || guest.nodeName;
      if (guestNodeId && !guestsByVmid.get(vmid)!.includes(guestNodeId)) {
        guestsByVmid.get(vmid)!.push(guestNodeId);
      }
    });
    
    // Log which guests are shared across multiple nodes
    for (const [vmid, nodes] of guestsByVmid.entries()) {
      if (nodes.length > 1) {
        this.logger.debug(`Guest with VMID ${vmid} is shared across nodes: ${nodes.join(', ')}`);
      }
    }
    
    // Now process each guest
    guests.forEach((guest: any) => {
      // Get the numeric ID (vmid) directly from the guest
      const vmid = typeof guest.id === 'number' ? guest.id : parseInt(String(guest.id), 10);
      
      // Check if this guest is shared across multiple nodes
      const isShared = this.isGuestShared(guest, guestsByVmid);
      
      // Format the ID string in Proxmox format for external use
      const guestType = guest.type === 'qemu' || guest.type === 'vm' ? 'qemu' : 'lxc';
      
      // Important: Include a unique identifier for the guest on this specific node
      // This prevents duplicates when the same guest ID appears on multiple nodes
      const proxmoxId = isShared ? 
        `${guestType}/${vmid}:${nodeId}` : // Use colon separator which is clearer in UI
        `${guestType}/${vmid}`; // Standard format for non-shared
      
      if (guest.type === 'qemu' || guest.type === 'vm') {
        // Process VM
        const vm: ProxmoxVM = {
          id: proxmoxId, // Proxmox-style ID for the UI with node suffix for shared guests
          name: guest.name,
          status: guest.status,
          node: nodeId,
          vmid: vmid, // Just the numeric ID for vmid
          type: 'qemu',
          cpus: guest.cpus || 1,
          cpu: guest.cpu || 0,
          memory: typeof guest.memory === 'number' ? guest.memory : (guest.memory?.used || 512 * 1024 * 1024),
          maxmem: typeof guest.memory === 'number' ? guest.memory : (guest.memory?.total || 1024 * 1024 * 1024),
          disk: typeof guest.disk === 'number' ? guest.disk : (guest.disk?.used || 5 * 1024 * 1024 * 1024),
          maxdisk: typeof guest.disk === 'number' ? guest.disk : (guest.disk?.total || 10 * 1024 * 1024 * 1024),
          uptime: guest.uptime || 0,
          netin: guest.netin || 0,
          netout: guest.netout || 0,
          diskread: guest.diskread || 0,
          diskwrite: guest.diskwrite || 0,
          template: guest.template || false
        };
        vms.push(vm);
      } else if (guest.type === 'lxc' || guest.type === 'ct') {
        // Process container
        const container: ProxmoxContainer = {
          id: proxmoxId, // Proxmox-style ID for the UI with node suffix for shared guests
          name: guest.name,
          status: guest.status,
          node: nodeId,
          vmid: vmid, // Just the numeric ID for vmid
          type: 'lxc',
          cpus: guest.cpus || 1,
          cpu: guest.cpu || 0,
          memory: typeof guest.memory === 'number' ? guest.memory : (guest.memory?.used || 512 * 1024 * 1024),
          maxmem: typeof guest.memory === 'number' ? guest.memory : (guest.memory?.total || 1024 * 1024 * 1024),
          disk: typeof guest.disk === 'number' ? guest.disk : (guest.disk?.used || 5 * 1024 * 1024 * 1024),
          maxdisk: typeof guest.disk === 'number' ? guest.disk : (guest.disk?.total || 10 * 1024 * 1024 * 1024),
          uptime: guest.uptime || 0,
          netin: guest.netin || 0,
          netout: guest.netout || 0,
          diskread: guest.diskread || 0,
          diskwrite: guest.diskwrite || 0,
          template: guest.template || false
        };
        containers.push(container);
      }
    });
    
    return { vms, containers };
  }
} 