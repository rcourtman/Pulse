// Properly typed TypeScript interfaces for Pulse API

export interface State {
  nodes: Node[];
  vms: VM[];
  containers: Container[];
  storage: Storage[];
  pbs: PBSInstance[];
  pbsBackups: PBSBackup[];
  metrics: Metric[];
  pveBackups: PVEBackups;
  performance: Performance;
  connectionHealth: Record<string, boolean>;
  stats: Stats;
  activeAlerts: Alert[];
  recentlyResolved: ResolvedAlert[];
  lastUpdate: string;
}

export interface Node {
  id: string;
  name: string;
  instance: string;
  host: string;
  status: string;
  type: string;
  cpu: number;
  memory: Memory;
  disk: Disk;
  uptime: number;
  loadAverage: number[];
  kernelVersion: string;
  pveVersion: string;
  cpuInfo: CPUInfo;
  lastSeen: string;
  connectionHealth: string;
  isClusterMember?: boolean;  // True if part of a cluster
  clusterName?: string;        // Name of cluster (empty if standalone)
}

export interface VM {
  id: string;
  vmid: number;
  name: string;
  node: string;
  instance: string;
  status: string;
  type: string;
  cpu: number;
  cpus: number;
  memory: Memory;
  disk: Disk;
  networkIn: number;
  networkOut: number;
  diskRead: number;
  diskWrite: number;
  uptime: number;
  template: boolean;
  lastBackup: string;
  tags: string[];
  lock: string;
  lastSeen: string;
}

export interface Container {
  id: string;
  vmid: number;
  name: string;
  node: string;
  instance: string;
  status: string;
  type: string;
  cpu: number;
  cpus: number;
  memory: Memory;
  disk: Disk;
  networkIn: number;
  networkOut: number;
  diskRead: number;
  diskWrite: number;
  uptime: number;
  template: boolean;
  lastBackup: string;
  tags: string[];
  lock: string;
  lastSeen: string;
}

export interface Storage {
  id: string;
  name: string;
  node: string;
  instance: string;
  type: string;
  status: string;
  total: number;
  used: number;
  free: number;
  usage: number;
  content: string;
  shared: boolean;
  enabled: boolean;
  active: boolean;
}

export interface PBSInstance {
  id: string;
  name: string;
  host: string;
  status: string;
  version: string;
  cpu: number;
  memory: number;
  memoryUsed: number;
  memoryTotal: number;
  uptime: number;
  datastores: PBSDatastore[];
  backupJobs: PBSBackupJob[];
  syncJobs: PBSSyncJob[];
  verifyJobs: PBSVerifyJob[];
  pruneJobs: PBSPruneJob[];
  garbageJobs: PBSGarbageJob[];
  connectionHealth: string;
  lastSeen: string;
}

export interface PBSDatastore {
  name: string;
  total: number;
  used: number;
  free: number;
  usage: number;
  status: string;
  error: string;
  namespaces: PBSNamespace[];
  deduplicationFactor?: number;
}

export interface PBSNamespace {
  path: string;
  parent: string;
  depth: number;
}

export interface PBSBackup {
  id: string;
  instance: string;
  datastore: string;
  namespace: string;
  backupType: string;
  vmid: string;
  backupTime: string;
  size: number;
  protected: boolean;
  verified: boolean;
  comment: string;
  files: string[];
  owner?: string;
}

export interface PBSBackupJob {
  id: string;
  store: string;
  type: string;
  vmid: string;
  lastBackup: string;
  nextRun: string;
  status: string;
  error: string;
}

export interface PBSSyncJob {
  id: string;
  store: string;
  remote: string;
  status: string;
  lastSync: string;
  nextRun: string;
  error: string;
}

export interface PBSVerifyJob {
  id: string;
  store: string;
  status: string;
  lastVerify: string;
  nextRun: string;
  error: string;
}

export interface PBSPruneJob {
  id: string;
  store: string;
  status: string;
  lastPrune: string;
  nextRun: string;
  error: string;
}

export interface PBSGarbageJob {
  id: string;
  store: string;
  status: string;
  lastGarbage: string;
  nextRun: string;
  removedBytes: number;
  error: string;
}

export interface Memory {
  total: number;
  used: number;
  free: number;
  usage: number;
}

export interface Disk {
  total: number;
  used: number;
  free: number;
  usage: number;
}

export interface CPUInfo {
  model: string;
  cores: number;
  sockets: number;
  mhz: string;
}

export interface Metric {
  timestamp: string;
  type: string;
  id: string;
  values: Record<string, number | string | boolean>;
}

export interface BackupTask {
  id: string;
  node: string;
  type: string;
  vmid: number;
  status: string;
  startTime: string;
  endTime?: string;
  size?: number;
  error?: string;
}

export interface StorageBackup {
  id: string;
  storage: string;
  node: string;
  type: string;
  vmid: number;
  time: string;
  ctime: number;
  size: number;
  format: string;
  notes?: string;
  protected: boolean;
  volid: string;
  isPBS: boolean;
  verified: boolean;
  verification?: string;
  encryption?: string;
}

export interface PVEBackups {
  backupTasks: BackupTask[];
  storageBackups: StorageBackup[];
  guestSnapshots: GuestSnapshot[];
}

export interface GuestSnapshot {
  id: string;
  name: string;
  node: string;
  type: string;
  vmid: number;
  time: string;
  description: string;
  parent: string;
  vmstate: boolean;
}

export interface Performance {
  apiCallDuration: Record<string, number>;
  lastPollDuration: number;
  pollingStartTime: string;
  totalApiCalls: number;
  failedApiCalls: number;
  cacheHits?: number;
  cacheMisses?: number;
}

export interface Stats {
  startTime: string;
  uptime: number;
  pollingCycles: number;
  webSocketClients: number;
  version: string;
}

// Alert types
export interface Alert {
  id: string;
  type: string;
  level: 'warning' | 'critical';
  resourceId: string;
  resourceName: string;
  node: string;
  instance: string;
  message: string;
  value: number;
  threshold: number;
  startTime: string;
  lastSeen?: string;
  acknowledged: boolean;
  ackTime?: string;
  ackUser?: string;
  metadata?: Record<string, unknown>;
}

export interface ResolvedAlert extends Alert {
  resolvedTime: string;
}

// WebSocket message types
export type WSMessage = 
  | { type: 'initialState'; data: State }
  | { type: 'rawData'; data: State }
  | { type: 'error'; error: string }
  | { type: 'ping'; data?: unknown }
  | { type: 'pong'; data?: unknown }
  | { type: 'welcome'; data?: unknown }
  | { type: 'alert'; data: Alert }
  | { type: 'alertResolved'; data: { alertId: string } }
  | { type: 'settingsUpdate'; data: { theme?: string } }
  | { type: 'update:progress'; data: {
      phase: string;
      progress: number;
      message: string;
    }
  }
  | { type: 'node_auto_registered'; data: {
      type: string;
      host: string;
      name: string;
      tokenId: string;
      hasToken: boolean;
      verifySSL?: boolean;
      status?: string;
    }}
  | { type: 'node_deleted'; data: { nodeType: string } }
  | { type: 'nodes_changed'; data?: unknown }
  | { type: 'discovery_update'; data: {
      servers: Array<{
        ip: string;
        port: number;
        type: string;
        version: string;
        hostname?: string;
        release?: string;
      }>;
      errors?: string[];
      timestamp?: number;
    }};

// Utility types
export type Status = 'running' | 'stopped' | 'paused' | 'unknown';
export type GuestType = 'qemu' | 'lxc';