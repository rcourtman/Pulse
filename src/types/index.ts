/**
 * Type definitions for Pulse for Proxmox VE
 * Note: Proxmox® is a registered trademark of Proxmox Server Solutions GmbH.
 * These type definitions are for interfacing with the Proxmox® VE API.
 */

// Node configuration type
export interface NodeConfig {
  id: string;
  name: string;
  host: string;
  tokenId: string;
  tokenSecret: string;
  autoDetectCluster?: boolean;
  isClusterEntryPoint?: boolean;
}

// Proxmox API response types
export interface ProxmoxNodeStatus {
  id: string;
  name: string;
  configName: string;
  status: 'online' | 'offline';
  uptime: number;
  cpu: number;
  memory: {
    total: number;
    used: number;
    free: number;
    usedPercentage: number;
  };
  swap: {
    total: number;
    used: number;
    free: number;
    usedPercentage: number;
  };
  disk: {
    total: number;
    used: number;
    free: number;
    usedPercentage: number;
  };
  loadAverage: [number, number, number];
  cpuInfo: {
    cores: number;
    sockets: number;
    model: string;
  };
}

// VM type
export interface ProxmoxVM {
  id: string;
  name: string;
  status: 'running' | 'stopped' | 'paused';
  node: string;
  vmid: number;
  cpus: number;
  cpu?: number;
  memory: number;
  maxmem: number;
  disk: number;
  maxdisk: number;
  uptime: number;
  netin: number;
  netout: number;
  diskread: number;
  diskwrite: number;
  template: boolean;
  type: 'qemu';
}

// Container type
export interface ProxmoxContainer {
  id: string;
  name: string;
  status: 'running' | 'stopped' | 'paused' | 'unknown';
  node: string;
  vmid: number;
  cpus: number;
  cpu?: number;
  memory: number;
  maxmem: number;
  disk: number;
  maxdisk: number;
  uptime: number;
  netin: number;
  netout: number;
  diskread: number;
  diskwrite: number;
  template: boolean;
  type: 'lxc';
}

// Union type for VM and Container
export type ProxmoxGuest = ProxmoxVM | ProxmoxContainer;

// Event data structure
export interface ProxmoxEvent {
  id: string;
  node: string;
  type: 'node' | 'vm' | 'container' | 'storage' | 'pool';
  eventTime: number;
  user: string;
  description: string;
  details?: Record<string, any>;
}

// Normalized metrics format
export interface MetricsData {
  timestamp: number;
  nodeId: string;
  guestId?: string;
  type: 'node' | 'vm' | 'container';
  metrics: {
    // For CPU, we could use a Uint8 (0-255) if we store as percentage
    // or a fixed-point number with 2 decimal places multiplied by 100
    cpu?: number;
    memory?: {
      // For large values like total/used memory, consider using
      // compression techniques or storing in KB/MB instead of bytes
      total: number;
      used: number;
      // For percentages, we could use Uint8 (0-255) values
      // where 255 represents 100%
      usedPercentage: number;
      // Unit for memory values (bytes, KB, MB, GB)
      unit?: 'bytes' | 'KB' | 'MB' | 'GB';
    };
    network?: {
      // For cumulative values, consider delta encoding
      in: number;
      out: number;
      // For rates, consider appropriate units (KB/s vs MB/s)
      // based on typical values to reduce size
      inRate?: number;
      outRate?: number;
      // Unit for network values (bytes, KB, MB, GB)
      unit?: 'bytes' | 'KB' | 'MB' | 'GB';
    };
    disk?: {
      // Similar to memory, use appropriate units
      total: number;
      used: number;
      // For percentages, use Uint8 (0-255)
      usedPercentage: number;
      readRate?: number;
      writeRate?: number;
      // Unit for disk values (bytes, KB, MB, GB)
      unit?: 'bytes' | 'KB' | 'MB' | 'GB';
    };
    uptime?: number;
    status?: string;
  };
}

// API response wrapper
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  timestamp: number;
}

// WebSocket message types
export enum WebSocketMessageType {
  METRICS_UPDATE = 'METRICS_UPDATE',
  NODE_STATUS_UPDATE = 'NODE_STATUS_UPDATE',
  GUEST_STATUS_UPDATE = 'GUEST_STATUS_UPDATE',
  EVENT = 'EVENT',
  ERROR = 'ERROR',
  CONNECTED = 'CONNECTED',
  DISCONNECTED = 'DISCONNECTED',
}

export interface WebSocketMessage<T = any> {
  type: WebSocketMessageType;
  payload: T;
  timestamp: number;
}

// App configuration
export interface AppConfig {
  port: number;
  nodeEnv: string;
  logLevel: string;
  enableDevTools: boolean;
  metricsHistoryMinutes: number;
  maxRealisticRate: number; // Maximum realistic network rate in MB/s
  ignoreSSLErrors: boolean;
  nodePollingIntervalMs: number;
  eventPollingIntervalMs: number;
  nodes: NodeConfig[];
  clusterMode: boolean;
  clusterName: string;
  autoDetectCluster: boolean;
} 