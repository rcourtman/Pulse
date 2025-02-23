import axios, { AxiosInstance } from 'axios';
import https from 'https';
import { ProxmoxToken } from '../types/proxmox';
import { logger } from '../server';

interface ProxmoxCredentials {
  host: string;
  tokenId: string;
  tokenSecret: string;
  node?: string;
}

interface NodeStatus {
  cpu: number;
  memory: {
    total: number;
    used: number;
    free: number;
  };
  uptime: number;
  loadavg: [number, number, number];
  disk: {
    total: number;
    used: number;
    free: number;
  };
}

export class ProxmoxService {
  private client: AxiosInstance;
  private baseUrl: string;
  private nodeName: string;
  private credentials: ProxmoxCredentials;

  constructor(credentials: ProxmoxCredentials) {
    this.baseUrl = credentials.host;
    this.nodeName = credentials.node || 'minipc';
    this.credentials = credentials;

    this.client = axios.create({
      baseURL: this.baseUrl,
      headers: {
        'Authorization': `PVEAPIToken=${credentials.tokenId}=${credentials.tokenSecret}`
      },
      httpsAgent: new https.Agent({
        rejectUnauthorized: false
      })
    });

    logger.info(`ProxmoxService initialized for ${this.nodeName}`);
  }

  getNodeName(): string {
    return this.nodeName;
  }

  getHost(): string {
    return this.baseUrl;
  }

  getTokenId(): string {
    return this.credentials.tokenId;
  }

  async validate(): Promise<void> {
    try {
      const response = await this.client.get('/api2/json/version');
      logger.debug('Proxmox API version response:', response.data);
      
      // Get node info as part of validation
      const nodesResponse = await this.client.get('/api2/json/nodes');
      logger.debug('Proxmox nodes response:', nodesResponse.data);
      
    } catch (error) {
      logger.error('Validation failed:', error);
      throw new Error('Failed to validate Proxmox connection');
    }
  }

  async getVersion(): Promise<any> {
    try {
      const response = await this.client.get('/api2/json/version');
      return response.data.data;
    } catch (error) {
      logger.error(`Failed to get version for ${this.nodeName}`, { error });
      throw error;
    }
  }

  async getNodeStatus(): Promise<NodeStatus> {
    try {
      const response = await this.client.get(`/api2/json/nodes/${this.nodeName}/status`);
      const data = response.data.data;
      
      return {
        cpu: data.cpu,
        memory: {
          total: data.memory.total,
          used: data.memory.used,
          free: data.memory.free
        },
        uptime: data.uptime,
        loadavg: data.loadavg,
        disk: {
          total: data.rootfs.total,
          used: data.rootfs.used,
          free: data.rootfs.free
        }
      };
    } catch (error) {
      logger.error(`Failed to get status for ${this.nodeName}`, { error });
      throw error;
    }
  }

  async getNodes(): Promise<any[]> {
    try {
      const response = await this.client.get('/api2/json/nodes');
      const nodes = response.data.data;
      logger.debug('Retrieved nodes from Proxmox:', nodes);
      return nodes; // This should contain node objects with 'node' property
    } catch (error) {
      logger.error('Failed to get nodes:', error);
      throw error;
    }
  }
} 