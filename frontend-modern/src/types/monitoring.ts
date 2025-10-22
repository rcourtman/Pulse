// Monitoring data types

export interface CPUInfo {
  usage: number;
  cores: number;
  sockets: number;
  model: string;
}

export interface MemoryInfo {
  used: number;
  total: number;
  free: number;
  usage: number;
}

export interface DiskInfo {
  used: number;
  total: number;
  free: number;
  usage: number;
  device?: string;
  filesystem?: string;
}

export interface NetworkInfo {
  in: number;
  out: number;
  device?: string;
}

export interface Node {
  id: string;
  name: string;
  displayName?: string;
  type: string;
  status: 'online' | 'offline' | 'unknown';
  instance: string;
  uptime: number;
  cpu: CPUInfo;
  memory: MemoryInfo;
  disk?: DiskInfo;
  network?: NetworkInfo;
  loadAverage?: [number, number, number];
  kernelVersion?: string;
}

export interface GuestNetworkInterface {
  name?: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
}

export interface Guest {
  id: string;
  vmid: number;
  name: string;
  node: string;
  type: 'qemu' | 'lxc';
  status: 'running' | 'stopped' | 'paused' | 'suspended';
  instance: string;
  uptime?: number;
  cpu: {
    usage: number;
    cores: number;
    limit?: number;
  };
  memory: {
    used: number;
    total: number;
    usage: number;
  };
  disk: {
    read: number;
    write: number;
    used?: number;
    total?: number;
  };
  network: {
    in: number;
    out: number;
  };
  osName?: string;
  osVersion?: string;
  agentVersion?: string;
  ipAddresses?: string[];
  networkInterfaces?: GuestNetworkInterface[];
  template?: boolean;
  haState?: string;
  protection?: boolean;
  tags?: string[];
}

export type VM = Guest & { type: 'qemu' };
export type Container = Guest & { type: 'lxc' };

export interface Storage {
  id: string;
  name: string;
  type: string;
  status: 'online' | 'offline' | 'unknown';
  node: string;
  instance: string;
  used: number;
  total: number;
  available: number;
  usage: number;
  content: string[];
  shared?: boolean;
  enabled?: boolean;
}

export interface CephCluster {
  id: string;
  instance: string;
  name: string;
  fsid?: string;
  health: string;
  healthMessage?: string;
  totalBytes: number;
  usedBytes: number;
  availableBytes: number;
  usagePercent: number;
  numMons: number;
  numMgrs: number;
  numOsds: number;
  numOsdsUp: number;
  numOsdsIn: number;
  numPGs: number;
  pools?: CephPool[];
  services?: CephServiceStatus[];
  lastUpdated: number;
}

export interface CephPool {
  id: number;
  name: string;
  storedBytes: number;
  availableBytes: number;
  objects: number;
  percentUsed: number;
}

export interface CephServiceStatus {
  type: string;
  running: number;
  total: number;
  message?: string;
}

export interface Backup {
  id: string;
  vmid: number;
  name: string;
  type: 'VM' | 'CT';
  size: number;
  backupTime: number;
  node: string;
  storage?: string;
  instance: string;
  notes?: string;
  protected?: boolean;
  encrypted?: boolean;
  verified?: boolean;
  format?: string;
}

export interface PBSBackup extends Backup {
  datastore: string;
  snapshotTime: number;
  owner?: string;
  fingerprint?: string;
}

export interface Alert {
  id: string;
  resourceId: string;
  resourceName: string;
  node: string;
  instance: string;
  type: string;
  level: 'warning' | 'critical';
  message: string;
  value: number;
  threshold: number;
  startTime: string;
  lastSeen?: string;
  acknowledged: boolean;
  acknowledgedBy?: string;
  acknowledgedAt?: string;
}
