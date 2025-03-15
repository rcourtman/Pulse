import { EventEmitter } from 'events';
import { io, Socket } from 'socket.io-client';
import { createLogger } from '../utils/logger';
import { NodeConfig, ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxEvent } from '../types';
import { vmTemplates, containerTemplates, getRandomVMStatus, getRandomContainerStatus } from '../mock/templates';
import { customMockData } from '../mock/custom-data';
import config from '../config';

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

  constructor(config: NodeConfig) {
    super();
    this.config = config;
    this.logger.info(`MockClient created for node: ${config.name}`);
    
    // Generate mock VMs and containers based on node ID
    this.generateMockData();
  }

  /**
   * Check if the node is part of a cluster (mock implementation)
   * @returns Object containing isCluster (boolean) and clusterName (string)
   */
  async isNodeInCluster(): Promise<{ isCluster: boolean; clusterName: string }> {
    // Simulate a delay to mimic network request
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Auto-detect cluster based on environment variable
    const autoDetectCluster = process.env.PROXMOX_AUTO_DETECT_CLUSTER !== 'false';
    
    if (autoDetectCluster) {
      this.logger.info(`Mock node is part of cluster: ${this.mockClusterName}`);
      return { isCluster: true, clusterName: this.mockClusterName };
    } else {
      this.logger.info('Mock node is not part of a cluster');
      return { isCluster: false, clusterName: '' };
    }
  }

  /**
   * Generate mock VMs and containers
   */
  private generateMockData(): void {
    const nodeId = this.config.id;
    const nodeName = this.config.name;
    
    // Find the custom data for this node
    const customNode = customMockData.nodes.find(n => n.id === nodeId || n.name === nodeName);
    
    if (customNode) {
      this.logger.info(`Using custom mock data for node ${nodeName} (id: ${nodeId})`);
      this.logger.info(`Custom node data: ${JSON.stringify({
        id: customNode.id,
        name: customNode.name,
        guestCount: customNode.guests.length,
        vmCount: customNode.guests.filter(g => g.type === 'vm').length,
        containerCount: customNode.guests.filter(g => g.type === 'ct').length
      })}`);
      
      // Process custom guests
      customNode.guests.forEach(guest => {
        this.logger.info(`Processing guest: ${guest.id} (${guest.name}, type: ${guest.type})`);
        
        // Check if this is a shared guest (ID 999 or 888)
        const isSharedGuest = guest.id === '999' || guest.id === '888';
        
        // For shared guests, we need to determine if this node should be the primary
        // We'll use a simple rule: the node with the lowest ID is the primary
        let shouldBeRunning = guest.status === 'running';
        
        if (isSharedGuest && shouldBeRunning) {
          // Find all nodes that have this guest
          const nodesWithGuest = customMockData.nodes
            .filter(n => n.guests.some(g => g.id === guest.id))
            .map(n => n.id);
          
          // Sort node IDs alphabetically
          const sortedNodeIds = [...nodesWithGuest].sort();
          
          // The primary node is the first one alphabetically
          const primaryNodeId = sortedNodeIds[0];
          
          // Only mark as running if this is the primary node
          shouldBeRunning = nodeId === primaryNodeId;
          
          this.logger.info(`Shared guest ${guest.id} (${guest.name}) on node ${nodeId}: primaryNode=${primaryNodeId}, shouldBeRunning=${shouldBeRunning}`);
        }
        
        if (guest.type === 'vm') {
          // Extract the VMID from the guest ID
          const vmid = parseInt(guest.id.split('-').pop() || '100');
          
          // Always use node-specific IDs
          const id = `${nodeId}-vm-${vmid}`;
          
          // Create a VM from the custom data
          const vm: ProxmoxVM = {
            id: id,
            name: guest.name,
            node: nodeId,
            status: shouldBeRunning ? 'running' : 'stopped',
            vmid: vmid,
            type: 'qemu',
            cpus: 2 + Math.floor(Math.random() * 6),
            maxmem: guest.memory,
            memory: shouldBeRunning ? Math.floor(guest.memory * 0.7) : 0,
            maxdisk: 50 * 1024 * 1024 * 1024,
            disk: 20 * 1024 * 1024 * 1024,
            uptime: shouldBeRunning ? 3600 * (1 + Math.floor(Math.random() * 72)) : 0,
            cpu: shouldBeRunning ? guest.cpu : 0,
            netout: shouldBeRunning ? Math.random() * 1024 * 1024 * 10 : 0,
            netin: shouldBeRunning ? Math.random() * 1024 * 1024 * 5 : 0,
            diskwrite: shouldBeRunning ? Math.random() * 1024 * 1024 : 0,
            diskread: shouldBeRunning ? Math.random() * 1024 * 1024 * 2 : 0,
            template: false
          };
          
          this.mockVMs.push(vm);
        } else if (guest.type === 'ct') {
          // Extract the VMID from the guest ID
          const vmid = parseInt(guest.id.split('-').pop() || '200');
          
          // Always use node-specific IDs
          const id = `${nodeId}-ct-${vmid}`;
          
          // Create a container from the custom data
          const container: ProxmoxContainer = {
            id: id,
            name: guest.name,
            node: nodeId,
            status: shouldBeRunning ? 'running' : 'stopped',
            vmid: vmid,
            type: 'lxc',
            cpus: 1 + Math.floor(Math.random() * 4),
            maxmem: guest.memory,
            memory: shouldBeRunning ? Math.floor(guest.memory * 0.6) : 0,
            maxdisk: 20 * 1024 * 1024 * 1024,
            disk: 8 * 1024 * 1024 * 1024,
            uptime: shouldBeRunning ? 3600 * (1 + Math.floor(Math.random() * 48)) : 0,
            cpu: shouldBeRunning ? guest.cpu : 0,
            netout: shouldBeRunning ? Math.random() * 1024 * 1024 * 5 : 0,
            netin: shouldBeRunning ? Math.random() * 1024 * 1024 * 3 : 0,
            diskwrite: shouldBeRunning ? Math.random() * 1024 * 512 : 0,
            diskread: shouldBeRunning ? Math.random() * 1024 * 1024 : 0,
            template: false
          };
          
          this.mockContainers.push(container);
        }
      });
      
      this.logger.info(`Generated ${this.mockVMs.length} VMs and ${this.mockContainers.length} containers for node ${nodeName}`);
    } else {
      this.logger.warn(`No custom data found for node ${nodeName}, using generated data`);
      
      // Generate 3-5 VMs per node
      const vmCount = 3 + Math.floor(Math.random() * 3);
      this.logger.info(`Generating ${vmCount} mock VMs for node ${nodeName}`);
      
      for (let i = 0; i < vmCount; i++) {
        // Generate the VM ID - always use node-specific IDs
        const vmid = 100 + i;
        const vmId = `${nodeId}-vm-${vmid}`;
        
        const status = getRandomVMStatus();
        // Use a better name from the list, wrapping around if needed
        const template = vmTemplates[i % vmTemplates.length];
        const cpuCores = 1 + Math.floor(Math.random() * 8);
        const memoryMB = 1024 * (1 + Math.floor(Math.random() * 16));
        
        const vm: ProxmoxVM = {
          id: vmId,
          name: `${nodeId}-${template.name}`, // Prefix with node ID to make unique
          node: nodeId,
          status: status,
          vmid: vmid,
          type: 'qemu',
          cpus: cpuCores,
          maxmem: memoryMB * 1024 * 1024,
          memory: status === 'running' ? Math.floor(memoryMB * 0.7) * 1024 * 1024 : 0,
          maxdisk: 50 * 1024 * 1024 * 1024,
          disk: 20 * 1024 * 1024 * 1024,
          uptime: status === 'running' ? 3600 * (1 + Math.floor(Math.random() * 72)) : 0,
          cpu: status === 'running' ? Math.random() * 0.8 : 0,
          netout: status === 'running' ? Math.random() * 1024 * 1024 * 10 : 0,
          netin: status === 'running' ? Math.random() * 1024 * 1024 * 5 : 0,
          diskwrite: status === 'running' ? Math.random() * 1024 * 1024 : 0,
          diskread: status === 'running' ? Math.random() * 1024 * 1024 * 2 : 0,
          template: false
        };
        
        this.mockVMs.push(vm);
      }
      
      // Generate 2-4 containers per node
      const containerCount = 2 + Math.floor(Math.random() * 3);
      this.logger.info(`Generating ${containerCount} mock containers for node ${nodeName}`);
      
      for (let i = 0; i < containerCount; i++) {
        // Generate the container ID - always use node-specific IDs
        const vmid = 200 + i;
        const containerId = `${nodeId}-ct-${vmid}`;
        
        const status = getRandomContainerStatus();
        // Use a better name from the list, wrapping around if needed
        const template = containerTemplates[i % containerTemplates.length];
        const cpuCores = 1 + Math.floor(Math.random() * 4);
        const memoryMB = 512 * (1 + Math.floor(Math.random() * 8));
        
        const container: ProxmoxContainer = {
          id: containerId,
          name: `${nodeId}-${template.name}`, // Prefix with node ID to make unique
          node: nodeId,
          status: status,
          vmid: vmid,
          type: 'lxc',
          cpus: cpuCores,
          maxmem: memoryMB * 1024 * 1024,
          memory: status === 'running' ? Math.floor(memoryMB * 0.6) * 1024 * 1024 : 0,
          maxdisk: 20 * 1024 * 1024 * 1024,
          disk: 8 * 1024 * 1024 * 1024,
          uptime: status === 'running' ? 3600 * (1 + Math.floor(Math.random() * 48)) : 0,
          cpu: status === 'running' ? Math.random() * 0.6 : 0,
          netout: status === 'running' ? Math.random() * 1024 * 1024 * 5 : 0,
          netin: status === 'running' ? Math.random() * 1024 * 1024 * 3 : 0,
          diskwrite: status === 'running' ? Math.random() * 1024 * 512 : 0,
          diskread: status === 'running' ? Math.random() * 1024 * 1024 : 0,
          template: false
        };
        
        this.mockContainers.push(container);
      }
      
      this.logger.info(`Generated ${this.mockVMs.length} VMs and ${this.mockContainers.length} containers for node ${nodeName}`);
    }
  }

  /**
   * Connect to the mock server
   */
  async connect(): Promise<boolean> {
    try {
      this.logger.info(`Connecting to mock server at ${this.mockServerUrl}`);
      
      this.socket = io(this.mockServerUrl, {
        reconnection: true,
        reconnectionAttempts: 5,
        reconnectionDelay: 1000,
        timeout: 5000
      });
      
      const socket = this.socket; // Store reference to avoid null check issues
      
      return new Promise((resolve) => {
        if (!socket) {
          this.logger.error('Socket is null');
          resolve(false);
          return;
        }
        
        socket.on('connect', () => {
          this.logger.info('Connected to mock server');
          this.connected = true;
          
          // Register for this node
          socket.emit('register', { nodeId: this.config.id, nodeName: this.config.name });
          
          // Send initial data to the mock server
          socket.emit('updateVMs', { nodeId: this.config.id, vms: this.mockVMs });
          socket.emit('updateContainers', { nodeId: this.config.id, containers: this.mockContainers });
          
          resolve(true);
        });
        
        socket.on('disconnect', () => {
          this.logger.info('Disconnected from mock server');
          this.connected = false;
          this.emit('disconnect');
        });
        
        socket.on('error', (error) => {
          this.logger.error('Socket error', { error });
          this.emit('error', error);
        });
        
        socket.on('message', (message) => {
          this.logger.debug('Received message from mock server', { message });
          this.emit('message', message);
        });
        
        socket.on('nodeStatus', (status) => {
          this.logger.debug('Received node status from mock server', { status });
          this.emit('nodeStatus', status);
        });
        
        socket.on('vmList', (vms) => {
          this.logger.debug('Received VM list from mock server', { count: vms.length });
          this.emit('vmList', vms);
        });
        
        socket.on('containerList', (containers) => {
          this.logger.debug('Received container list from mock server', { count: containers.length });
          this.emit('containerList', containers);
        });
        
        socket.on('event', (event) => {
          this.logger.debug('Received event from mock server', { event });
          this.emit('event', event);
        });
        
        // Handle guests data from the mock server
        socket.on('guests', (data) => {
          this.logger.info(`Received ${data.guests.length} guests from mock server`);
          
          // Update our local VMs and containers based on the data from the server
          // This ensures we respect the primary node assignment from the server
          data.guests.forEach((guest: any) => {
            if (guest.type === 'vm') {
              // Find the VM in our local list
              const vmIndex = this.mockVMs.findIndex(vm => vm.id === guest.id || vm.name === guest.name);
              if (vmIndex >= 0) {
                // Update the VM status to match what the server says
                this.mockVMs[vmIndex].status = guest.status;
                this.logger.debug(`Updated VM ${guest.name} status to ${guest.status}`);
              }
            } else if (guest.type === 'ct') {
              // Find the container in our local list
              const ctIndex = this.mockContainers.findIndex(ct => ct.id === guest.id || ct.name === guest.name);
              if (ctIndex >= 0) {
                // Update the container status to match what the server says
                this.mockContainers[ctIndex].status = guest.status;
                this.logger.debug(`Updated container ${guest.name} status to ${guest.status}`);
              }
            }
          });
        });
        
        // Handle connection timeout
        setTimeout(() => {
          if (!this.connected) {
            this.logger.error('Connection to mock server timed out');
            resolve(false);
          }
        }, 5000);
      });
    } catch (error) {
      this.logger.error('Error connecting to mock server', { error });
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
    // If connected to mock server, request status
    if (this.socket && this.connected) {
      this.socket.emit('getNodeStatus', { nodeId: this.config.id });
    }
    
    // Return a mock status object
    return {
      id: this.config.id,
      name: this.config.name,
      configName: this.config.name,
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
   * Get virtual machines
   */
  async getVMs(): Promise<ProxmoxVM[]> {
    // If connected to mock server, request VMs
    if (this.socket && this.connected) {
      this.socket.emit('getVMs', { nodeId: this.config.id });
    }
    
    // Simulate some CPU and memory usage changes for running VMs
    this.mockVMs.forEach(vm => {
      // Only update metrics for VMs that are running on this node
      if (vm.status === 'running' && vm.node === this.config.id) {
        // Update metrics for running VMs
        if (vm.cpu !== undefined) {
          // Generate more dynamic CPU values that fluctuate between 5% and 80%
          const randomFactor = Math.random() * 0.3 - 0.15; // Random value between -0.15 and 0.15
          
          // If VM is under high load (simulating stress test), keep CPU high with less fluctuation
          if (vm.cpu > 0.7) {  // Over 70% CPU
            // Use smaller variations for high CPU usage to simulate more stable stress test
            const highLoadRandomFactor = Math.random() * 0.1 - 0.02; // Bias toward staying high (-0.02 to 0.08)
            vm.cpu = Math.min(0.95, Math.max(0.65, vm.cpu + highLoadRandomFactor));
          } else {
            // Normal variations for regular usage
            vm.cpu = Math.min(0.8, Math.max(0.05, vm.cpu + randomFactor));
          }
        }
        
        // Update memory with more variation
        const memoryVariation = Math.random() * 0.2 - 0.1; // Random value between -0.1 and 0.1
        vm.memory = Math.min(vm.maxmem, Math.max(vm.maxmem * 0.2, vm.memory + (vm.maxmem * memoryVariation)));
        
        // Update network metrics with more realistic values
        // Network traffic tends to come in bursts, so simulate that
        const burstFactor = Math.random() > 0.7 ? 3 : 1; // 30% chance of a traffic burst
        vm.netout += Math.random() * 1024 * 1024 * burstFactor; // Bytes per update
        vm.netin += Math.random() * 1024 * 512 * burstFactor; // Bytes per update
        
        // Update disk metrics
        vm.diskwrite += Math.random() * 1024 * 100; // Bytes per update
        vm.diskread += Math.random() * 1024 * 200; // Bytes per update
        
        vm.uptime += 10; // Add 10 seconds to uptime
      } else if (vm.status !== 'running') {
        // For non-running VMs, ensure metrics are zeroed out
        if (vm.cpu !== undefined) {
          vm.cpu = 0;
        }
        vm.memory = 0;
        vm.netout = 0;
        vm.netin = 0;
        vm.diskwrite = 0;
        vm.diskread = 0;
      }
    });
    
    return this.mockVMs;
  }

  /**
   * Get containers
   */
  async getContainers(): Promise<ProxmoxContainer[]> {
    // If connected to mock server, request containers
    if (this.socket && this.connected) {
      this.socket.emit('getContainers', { nodeId: this.config.id });
    }
    
    // Simulate some CPU and memory usage changes for running containers
    this.mockContainers.forEach(container => {
      // Only update metrics for containers that are running on this node
      if (container.status === 'running' && container.node === this.config.id) {
        // Update metrics for running containers
        if (container.cpu !== undefined) {
          // Generate more dynamic CPU values that fluctuate between 2% and 60%
          const randomFactor = Math.random() * 0.25 - 0.125; // Random value between -0.125 and 0.125
          
          // If container is under high load (simulating stress test), keep CPU high with less fluctuation
          if (container.cpu > 0.5) {  // Over 50% CPU
            // Use smaller variations for high CPU usage to simulate more stable stress test
            const highLoadRandomFactor = Math.random() * 0.08 - 0.01; // Bias toward staying high (-0.01 to 0.07)
            container.cpu = Math.min(0.85, Math.max(0.45, container.cpu + highLoadRandomFactor));
          } else {
            // Normal variations for regular usage
            container.cpu = Math.min(0.6, Math.max(0.02, container.cpu + randomFactor));
          }
        }
        
        // Update memory with more variation
        const memoryVariation = Math.random() * 0.16 - 0.08; // Random value between -0.08 and 0.08
        container.memory = Math.min(container.maxmem, Math.max(container.maxmem * 0.1, container.memory + (container.maxmem * memoryVariation)));
        
        // Update network metrics with more realistic values
        // Network traffic tends to come in bursts, so simulate that
        const burstFactor = Math.random() > 0.8 ? 2 : 1; // 20% chance of a traffic burst
        container.netout += Math.random() * 1024 * 512 * burstFactor; // Bytes per update
        container.netin += Math.random() * 1024 * 256 * burstFactor; // Bytes per update
        
        // Update disk metrics
        container.diskwrite += Math.random() * 1024 * 50; // Bytes per update
        container.diskread += Math.random() * 1024 * 100; // Bytes per update
        
        container.uptime += 10; // Add 10 seconds to uptime
      } else if (container.status !== 'running') {
        // For non-running containers, ensure metrics are zeroed out
        if (container.cpu !== undefined) {
          container.cpu = 0;
        }
        container.memory = 0;
        container.netout = 0;
        container.netin = 0;
        container.diskwrite = 0;
        container.diskread = 0;
      }
    });
    
    return this.mockContainers;
  }

  /**
   * Set up event polling
   */
  setupEventPolling(): void {
    // No need to do anything here since we're using socket.io events
    this.logger.info('Event polling set up via socket.io connection');
    
    // Connect if not already connected
    if (!this.connected) {
      this.connect().catch(error => {
        this.logger.error('Failed to connect during setupEventPolling', { error });
      });
    }
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
   * Get all VMs and containers from all nodes in the cluster
   * This is a mock implementation of the /cluster/resources endpoint
   */
  async getClusterResources(): Promise<{ vms: ProxmoxVM[]; containers: ProxmoxContainer[] }> {
    this.logger.info('Getting cluster resources (mock implementation)');
    
    // Simulate a delay to mimic network request
    await new Promise(resolve => setTimeout(resolve, 300));
    
    // For the mock implementation, we'll gather all guests from customMockData
    // to simulate what would come from the real cluster resources endpoint
    const allVMs: ProxmoxVM[] = [];
    const allContainers: ProxmoxContainer[] = [];
    
    // Process each node in our custom mock data
    for (const node of customMockData.nodes) {
      const nodeId = node.id;
      
      // Process each guest for this node
      for (const guest of node.guests) {
        // Check if this is a VM or container
        if (guest.type === 'vm') {
          // Extract the VMID from the guest ID
          const vmid = parseInt(guest.id.split('-').pop() || '100');
          
          // For cluster resource endpoints, always use cluster-level IDs
          const id = `${this.mockClusterName}-vm-${vmid}`;
          
          // Create a VM from the custom data
          const vm: ProxmoxVM = {
            id: id,
            name: guest.name,
            node: node.name, // Use the node name, not ID
            status: guest.status === 'running' ? 'running' : 'stopped',
            vmid: vmid,
            type: 'qemu',
            cpus: 2 + Math.floor(Math.random() * 6),
            maxmem: guest.memory,
            memory: guest.status === 'running' ? Math.floor(guest.memory * 0.7) : 0,
            maxdisk: guest.disk.total,
            disk: guest.disk.used,
            uptime: guest.status === 'running' ? 3600 * (1 + Math.floor(Math.random() * 72)) : 0,
            cpu: guest.status === 'running' ? guest.cpu : 0,
            netout: guest.status === 'running' ? Math.random() * 1024 * 1024 * 10 : 0,
            netin: guest.status === 'running' ? Math.random() * 1024 * 1024 * 5 : 0,
            diskwrite: guest.status === 'running' ? Math.random() * 1024 * 1024 : 0,
            diskread: guest.status === 'running' ? Math.random() * 1024 * 1024 * 2 : 0,
            template: false
          };
          
          allVMs.push(vm);
        } else if (guest.type === 'ct') {
          // Extract the VMID from the guest ID
          const vmid = parseInt(guest.id.split('-').pop() || '200');
          
          // For cluster resource endpoints, always use cluster-level IDs
          const id = `${this.mockClusterName}-ct-${vmid}`;
          
          // Create a container from the custom data
          const container: ProxmoxContainer = {
            id: id,
            name: guest.name,
            node: node.name, // Use the node name, not ID
            status: guest.status === 'running' ? 'running' : 'stopped',
            vmid: vmid,
            type: 'lxc',
            cpus: 1 + Math.floor(Math.random() * 4),
            maxmem: guest.memory,
            memory: guest.status === 'running' ? Math.floor(guest.memory * 0.6) : 0,
            maxdisk: guest.disk.total,
            disk: guest.disk.used,
            uptime: guest.status === 'running' ? 3600 * (1 + Math.floor(Math.random() * 48)) : 0,
            cpu: guest.status === 'running' ? guest.cpu : 0,
            netout: guest.status === 'running' ? Math.random() * 1024 * 1024 * 5 : 0,
            netin: guest.status === 'running' ? Math.random() * 1024 * 1024 * 3 : 0,
            diskwrite: guest.status === 'running' ? Math.random() * 1024 * 512 : 0,
            diskread: guest.status === 'running' ? Math.random() * 1024 * 1024 : 0,
            template: false
          };
          
          allContainers.push(container);
        }
      }
    }
    
    this.logger.info(`Returning ${allVMs.length} VMs and ${allContainers.length} containers from cluster resources endpoint`);
    
    return { 
      vms: allVMs, 
      containers: allContainers 
    };
  }

  /**
   * Get cluster resources (VMs and containers from all nodes in cluster)
   * @returns Promise with cluster resources
   */
}
