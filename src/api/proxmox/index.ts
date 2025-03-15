import { EventEmitter } from 'events';
import axios, { AxiosInstance } from 'axios';
import https from 'https';
import { createLogger } from '../../utils/logger';
import { NodeConfig, ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxEvent } from '../../types';
import { formatSafeTokenId } from '../../utils/config-validator';
import { formatBytes, bytesToMB, mbToBytes } from '../../utils/format';
import config from '../../config';
import winston from 'winston';
import { ProxmoxClientMethods } from './types';
import { isNodeInCluster } from './cluster';

// Define the class without the method implementations
export class ProxmoxClient extends EventEmitter implements ProxmoxClientMethods {
  config: NodeConfig;
  logger: winston.Logger;
  client: AxiosInstance | null = null;
  retryAttempts: number = 3;
  retryDelayMs: number = 5000;
  eventLastTimestamp = 0;
  isMockData: boolean = false;
  nodeName = '';
  useClusterResources: boolean = true;

  constructor(config: NodeConfig, ignoreSSLErrors: boolean = false) {
    super();
    this.config = config;
    this.logger = createLogger('ProxmoxClient', config.id);
    
    // Check if this is a mock data client
    this.isMockData = process.env.USE_MOCK_DATA === 'true' || process.env.MOCK_DATA_ENABLED === 'true';
    
    if (this.isMockData) {
      this.logger.info('Mock data mode enabled. Using mock data server.');
    } else {
      // Get timeout from environment variable or use default
      const apiTimeoutMs = parseInt(process.env.API_TIMEOUT_MS || '60000', 10);
      this.retryAttempts = parseInt(process.env.API_RETRY_ATTEMPTS || '3', 10);
      this.retryDelayMs = parseInt(process.env.API_RETRY_DELAY_MS || '5000', 10);
      
      // Determine if SSL verification should be disabled
      // Check multiple environment variables that could control SSL verification
      const disableSSLVerification = 
        ignoreSSLErrors || 
        process.env.PROXMOX_REJECT_UNAUTHORIZED === 'false' ||
        process.env.NODE_TLS_REJECT_UNAUTHORIZED === '0' ||
        process.env.HTTPS_REJECT_UNAUTHORIZED === 'false' ||
        process.env.PROXMOX_INSECURE === 'true' ||
        process.env.PROXMOX_VERIFY_SSL === 'false' ||
        process.env.IGNORE_SSL_ERRORS === 'true';
      
      if (disableSSLVerification) {
        this.logger.warn('SSL certificate verification is disabled. This is insecure and should only be used with trusted networks.');
      }
      
      // Create axios instance with base configuration
      const axiosConfig = {
        baseURL: `${config.host}/api2/json`,
        headers: {
          Authorization: `PVEAPIToken=${config.tokenId}=${config.tokenSecret}`
        },
        timeout: apiTimeoutMs,
        httpsAgent: new https.Agent({
          rejectUnauthorized: !disableSSLVerification
        })
      };
      
      this.client = axios.create(axiosConfig);
      this.logger.info(`Proxmox API client created with timeout: ${apiTimeoutMs}ms and ${this.retryAttempts} retry attempts`);

      // Add request interceptor for logging
      this.client.interceptors.request.use(request => {
        this.logger.debug(`API Request: ${request.method?.toUpperCase()} ${request.url}`, { 
          params: request.params 
        });
        return request;
      });

      // Add response interceptor for logging
      this.client.interceptors.response.use(
        response => {
          this.logger.debug(`API Response: ${response.status} ${response.config.method?.toUpperCase()} ${response.config.url}`, { 
            data: response.data 
          });
          return response;
        },
        async error => {
          if (error.response) {
            this.logger.error(`API Error: ${error.response.status} ${error.config?.method?.toUpperCase()} ${error.config?.url}`, { 
              data: error.response.data 
            });
          } else {
            this.logger.error(`API Error: ${error.message}`, { error });
          }
          
          // Implement retry logic for network errors and timeouts
          const config = error.config;
          
          // Only retry on network errors or timeouts, not on 4xx or 5xx responses
          if (!error.response && config && (!config.retryCount || config.retryCount < this.retryAttempts)) {
            config.retryCount = config.retryCount || 0;
            config.retryCount++;
            
            this.logger.warn(`Retrying request (attempt ${config.retryCount}/${this.retryAttempts}): ${config.method?.toUpperCase()} ${config.url}`);
            
            // Use configured retry delay instead of exponential backoff
            const delay = this.retryDelayMs;
            await new Promise(resolve => setTimeout(resolve, delay));
            
            if (this.client) {
              return this.client(config);
            }
            return Promise.reject(new Error('HTTP client is not initialized'));
          }
          
          return Promise.reject(error);
        }
      );

      // Initialize cluster detection if auto-detection is enabled
      if (config.autoDetectCluster) {
        this.initializeClusterDetection();
      }
    }
  }

  // Add the isNodeInCluster method
  isNodeInCluster = isNodeInCluster;

  // Method stubs that will be implemented by the prototype assignments
  async discoverNodeName(): Promise<string> { throw new Error('Not implemented'); }
  extractIpAddress(host: string): string { throw new Error('Not implemented'); }
  async getNodeNameAsync(): Promise<string> { throw new Error('Not implemented'); }
  getNodeName(): string { throw new Error('Not implemented'); }
  async getNodeStatus(): Promise<ProxmoxNodeStatus> { throw new Error('Not implemented'); }
  async getVirtualMachines(): Promise<ProxmoxVM[]> { throw new Error('Not implemented'); }
  async getContainers(): Promise<ProxmoxContainer[]> { throw new Error('Not implemented'); }
  async getGuestResourceUsage(type: 'qemu' | 'lxc', vmid: number): Promise<any> { throw new Error('Not implemented'); }
  async subscribeToEvents(callback: (event: ProxmoxEvent) => void): Promise<() => void> { throw new Error('Not implemented'); }
  determineEventType(event: any): 'node' | 'vm' | 'container' | 'storage' | 'pool' { throw new Error('Not implemented'); }
  setupEventPolling(): void { throw new Error('Not implemented'); }
  async getClusterResources(): Promise<{ vms: any[], containers: any[], nodes: string[] }> { throw new Error('Not implemented'); }

  /**
   * Test the connection to the Proxmox API
   */
  async testConnection(): Promise<boolean> {
    try {
      // First try to discover the node name
      const nodeName = await this.discoverNodeName();
      
      // Then try to access the node's status endpoint
      if (this.client) {
        await this.client.get(`/nodes/${nodeName}/status`);
      }
      
      this.logger.info('Connection test successful');
      return true;
    } catch (error) {
      this.logger.error('Connection test failed', { error });
      return false;
    }
  }

  /**
   * Initialize cluster detection
   * This method checks if the node is part of a cluster and updates the configuration accordingly
   */
  private async initializeClusterDetection(): Promise<void> {
    try {
      // Only auto-detect if the setting is enabled
      if (!config.autoDetectCluster) {
        this.logger.info('Cluster auto-detection is disabled. Using manual cluster mode setting.');
        return;
      }
      
      // Check if the node is part of a cluster
      const { isCluster, clusterName } = await this.isNodeInCluster();
      
      if (isCluster) {
        // If the node is part of a cluster, update the global config
        this.logger.info(`Node is part of cluster: ${clusterName}. Enabling cluster mode.`);
        
        // Update the global config to enable cluster mode
        // This will affect how IDs are generated for VMs and containers
        config.clusterMode = true;
        config.clusterName = clusterName;
      } else {
        this.logger.info('Node is not part of a cluster. Cluster mode will not be enabled.');
        // Explicitly disable cluster mode when not in a cluster
        config.clusterMode = false;
      }
    } catch (error) {
      this.logger.error('Error initializing cluster detection', { error });
    }
  }
}

// Import functionality after the class definition to avoid circular dependencies
import { 
  discoverNodeName, 
  extractIpAddress, 
  getNodeNameAsync, 
  getNodeName 
} from './node-discovery';

import { getNodeStatus } from './node-status';

import { 
  getVirtualMachines, 
  getContainers, 
  getGuestResourceUsage 
} from './guests';

import { 
  subscribeToEvents, 
  determineEventType, 
  setupEventPolling 
} from './events';

/**
 * Get all VMs and containers from all nodes in the cluster with one API call
 */
async function getClusterResources(this: ProxmoxClient): Promise<{ vms: any[], containers: any[], nodes: string[] }> {
  this.logger.debug(`Getting cluster resources for ${this.config.name}`);
  
  try {
    if (!this.useClusterResources) {
      throw new Error("Cluster resources feature is disabled");
    }
    
    const response = await this.client?.get('/cluster/resources');
    const resources = response?.data.data || [];
    
    const vms: any[] = [];
    const containers: any[] = [];
    const nodeSet = new Set<string>();
    
    // Filter resources to only include VMs and containers
    resources?.forEach((resource: any) => {
      // Track all unique node names
      if (resource.node) {
        nodeSet.add(resource.node);
      }
      
      if (resource.type === 'qemu') {
        vms.push({
          ...resource,
          node: resource.node, // Ensure node property is set
          id: `${resource.node}/${resource.id}`, // Create unique ID including node
          vmid: resource.vmid,
          name: resource.name,
          status: resource.status
        });
      } else if (resource.type === 'lxc') {
        containers.push({
          ...resource,
          node: resource.node, // Ensure node property is set
          id: `${resource.node}/${resource.id}`, // Create unique ID including node
          vmid: resource.vmid,
          name: resource.name,
          status: resource.status
        });
      }
    });
    
    const nodes = Array.from(nodeSet);
    this.logger.info(`Found ${nodes.length} unique nodes in cluster resources`);
    
    return { vms, containers, nodes };
  } catch (error) {
    this.logger.error(`Error getting cluster resources for ${this.config.name}:`, error);
    
    return {
      vms: [],
      containers: [],
      nodes: []
    };
  }
}

// Assign methods to the prototype
ProxmoxClient.prototype.discoverNodeName = discoverNodeName;
ProxmoxClient.prototype.extractIpAddress = extractIpAddress;
ProxmoxClient.prototype.getNodeNameAsync = getNodeNameAsync;
ProxmoxClient.prototype.getNodeName = getNodeName;
ProxmoxClient.prototype.getNodeStatus = getNodeStatus;
ProxmoxClient.prototype.getVirtualMachines = getVirtualMachines;
ProxmoxClient.prototype.getContainers = getContainers;
ProxmoxClient.prototype.getGuestResourceUsage = getGuestResourceUsage;
ProxmoxClient.prototype.subscribeToEvents = subscribeToEvents;
ProxmoxClient.prototype.determineEventType = determineEventType;
ProxmoxClient.prototype.setupEventPolling = setupEventPolling;
ProxmoxClient.prototype.getClusterResources = getClusterResources; 