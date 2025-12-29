// Properly typed TypeScript interfaces for Pulse API

import type { Resource } from './resource';

export interface State {
  nodes: Node[];
  vms: VM[];
  containers: Container[];
  dockerHosts: DockerHost[];
  removedDockerHosts?: RemovedDockerHost[];
  kubernetesClusters?: KubernetesCluster[];
  removedKubernetesClusters?: RemovedKubernetesCluster[];
  hosts: Host[];
  replicationJobs: ReplicationJob[];
  storage: Storage[];
  cephClusters: CephCluster[];
  physicalDisks: PhysicalDisk[];
  pbs: PBSInstance[];
  pmg: PMGInstance[];
  pbsBackups: PBSBackup[];
  pmgBackups: PMGBackup[];
  backups: Backups;
  metrics: Metric[];
  pveBackups: PVEBackups;
  performance: Performance;
  connectionHealth: Record<string, boolean>;
  stats: Stats;
  activeAlerts: Alert[];
  recentlyResolved: ResolvedAlert[];
  lastUpdate: string;
  temperatureMonitoringEnabled?: boolean;
  // Unified resources (new data model - eventually replaces legacy arrays above)
  resources?: Resource[];
}

export interface RemovedDockerHost {
  id: string;
  hostname?: string;
  displayName?: string;
  removedAt: number;
}

export interface KubernetesCluster {
  id: string;
  agentId: string;
  name?: string;
  displayName?: string;
  customDisplayName?: string;
  server?: string;
  context?: string;
  version?: string;
  status: string;
  lastSeen: number;
  intervalSeconds: number;
  agentVersion?: string;
  tokenId?: string;
  tokenName?: string;
  tokenHint?: string;
  tokenLastUsedAt?: number;
  hidden?: boolean;
  pendingUninstall?: boolean;
  nodes?: KubernetesNode[];
  pods?: KubernetesPod[];
  deployments?: KubernetesDeployment[];
}

export interface RemovedKubernetesCluster {
  id: string;
  name?: string;
  displayName?: string;
  removedAt: number;
}

export interface KubernetesNode {
  uid: string;
  name: string;
  ready: boolean;
  unschedulable?: boolean;
  kubeletVersion?: string;
  containerRuntimeVersion?: string;
  osImage?: string;
  kernelVersion?: string;
  architecture?: string;
  capacityCpuCores?: number;
  capacityMemoryBytes?: number;
  capacityPods?: number;
  allocatableCpuCores?: number;
  allocatableMemoryBytes?: number;
  allocatablePods?: number;
  roles?: string[];
}

export interface KubernetesPodContainer {
  name: string;
  image?: string;
  ready: boolean;
  restartCount?: number;
  state?: string;
  reason?: string;
  message?: string;
}

export interface KubernetesPod {
  uid: string;
  name: string;
  namespace: string;
  nodeName?: string;
  phase?: string;
  reason?: string;
  message?: string;
  qosClass?: string;
  createdAt?: number;
  startTime?: number;
  restarts?: number;
  labels?: Record<string, string>;
  ownerKind?: string;
  ownerName?: string;
  containers?: KubernetesPodContainer[];
}

export interface KubernetesDeployment {
  uid: string;
  name: string;
  namespace: string;
  desiredReplicas?: number;
  updatedReplicas?: number;
  readyReplicas?: number;
  availableReplicas?: number;
  labels?: Record<string, string>;
}

export interface Node {
  id: string;
  name: string;
  displayName?: string;
  instance: string;
  host: string;
  guestURL?: string; // Optional guest-accessible URL (for navigation)
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
  temperature?: Temperature; // CPU/NVMe temperatures
  temperatureMonitoringEnabled?: boolean | null; // Per-node temperature monitoring override
  lastSeen: string;
  connectionHealth: string;
  isClusterMember?: boolean; // True if part of a cluster
  clusterName?: string; // Name of cluster (empty if standalone)
  linkedHostAgentId?: string; // ID of host agent running on this node (for merging)
}

export interface GuestNetworkInterface {
  name?: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
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
  disks?: Disk[];
  diskStatusReason?: string;
  ipAddresses?: string[];
  osName?: string;
  osVersion?: string;
  agentVersion?: string;
  networkInterfaces?: GuestNetworkInterface[];
  networkIn: number;
  networkOut: number;
  diskRead: number;
  diskWrite: number;
  uptime: number;
  template: boolean;
  lastBackup: number;
  tags: string[] | string | null;
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
  disks?: Disk[];
  ipAddresses?: string[];
  osName?: string;
  osVersion?: string;
  agentVersion?: string;
  networkInterfaces?: GuestNetworkInterface[];
  networkIn: number;
  networkOut: number;
  diskRead: number;
  diskWrite: number;
  uptime: number;
  template: boolean;
  lastBackup: number;
  tags: string[] | string | null;
  lock: string;
  lastSeen: string;
  // OCI container support (Proxmox VE 9.1+)
  isOci?: boolean; // True if this is an OCI container
  osTemplate?: string; // Template or OCI image used (e.g., "oci:docker.io/library/alpine:latest")
}

export interface DockerHost {
  id: string;
  agentId: string;
  hostname: string;
  displayName: string;
  customDisplayName?: string;
  machineId?: string;
  os?: string;
  kernelVersion?: string;
  architecture?: string;
  runtime?: string;
  runtimeVersion?: string;
  dockerVersion?: string;
  cpus: number;
  totalMemoryBytes: number;
  uptimeSeconds: number;
  cpuUsagePercent?: number;
  loadAverage?: number[];
  memory?: Memory;
  disks?: Disk[];
  networkInterfaces?: HostNetworkInterface[];
  status: string;
  lastSeen: number;
  intervalSeconds: number;
  agentVersion?: string;
  containers: DockerContainer[];
  services?: DockerService[];
  tasks?: DockerTask[];
  swarm?: DockerSwarmInfo;
  tokenId?: string;
  tokenName?: string;
  tokenHint?: string;
  tokenLastUsedAt?: number;
  revokedTokenId?: string;
  tokenRevokedAt?: number;
  hidden?: boolean;
  pendingUninstall?: boolean;
  command?: DockerHostCommand;
  isLegacy?: boolean;
}

export interface DockerHostCommand {
  id: string;
  type: string;
  status: string;
  message?: string;
  createdAt: number;
  updatedAt: number;
  dispatchedAt?: number;
  acknowledgedAt?: number;
  completedAt?: number;
  failedAt?: number;
  failureReason?: string;
  expiresAt?: number;
}

export interface DockerService {
  id: string;
  name: string;
  stack?: string;
  image?: string;
  mode?: string;
  desiredTasks?: number;
  runningTasks?: number;
  completedTasks?: number;
  labels?: Record<string, string>;
  endpointPorts?: DockerServicePort[];
  updateStatus?: DockerServiceUpdate;
  createdAt?: number;
  updatedAt?: number;
}

export interface DockerServicePort {
  name?: string;
  protocol?: string;
  targetPort?: number;
  publishedPort?: number;
  publishMode?: string;
}

export interface DockerServiceUpdate {
  state?: string;
  message?: string;
  completedAt?: number;
}

export interface DockerTask {
  id: string;
  serviceId?: string;
  serviceName?: string;
  slot?: number;
  nodeId?: string;
  nodeName?: string;
  desiredState?: string;
  currentState?: string;
  error?: string;
  message?: string;
  containerId?: string;
  containerName?: string;
  createdAt?: number | string; // Unix timestamp or ISO string
  updatedAt?: number | string; // Unix timestamp or ISO string
  startedAt?: number | string; // Unix timestamp or ISO string
  completedAt?: number | string; // Unix timestamp or ISO string
}

export interface DockerSwarmInfo {
  nodeId?: string;
  nodeRole?: string;
  localState?: string;
  controlAvailable?: boolean;
  clusterId?: string;
  clusterName?: string;
  scope?: string;
  error?: string;
}

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  imageDigest?: string; // Current image digest (sha256:...)
  state: string;
  status: string;
  health?: string;
  cpuPercent: number;
  memoryUsageBytes: number;
  memoryLimitBytes: number;
  memoryPercent: number;
  uptimeSeconds: number;
  restartCount: number;
  exitCode: number;
  createdAt: number;
  startedAt?: number | null;
  finishedAt?: number | null;
  ports?: DockerContainerPort[];
  labels?: Record<string, string>;
  env?: string[];
  networks?: DockerContainerNetwork[];
  writableLayerBytes?: number;
  rootFilesystemBytes?: number;
  blockIo?: DockerContainerBlockIO;
  mounts?: DockerContainerMount[];
  podman?: PodmanContainerMetadata;
  updateStatus?: DockerContainerUpdateStatus; // Image update detection status
}

// Update status for container images
export interface DockerContainerUpdateStatus {
  updateAvailable: boolean;
  currentDigest?: string;
  latestDigest?: string;
  lastChecked: number;
  error?: string;
}

export interface DockerContainerPort {
  privatePort: number;
  publicPort?: number;
  protocol: string;
  ip?: string;
}

export interface DockerContainerNetwork {
  name: string;
  ipv4?: string;
  ipv6?: string;
}

export interface DockerContainerBlockIO {
  readBytes?: number;
  writeBytes?: number;
  readRateBytesPerSecond?: number;
  writeRateBytesPerSecond?: number;
}

export interface DockerContainerMount {
  type?: string;
  source?: string;
  destination?: string;
  mode?: string;
  rw?: boolean;
  propagation?: string;
  name?: string;
  driver?: string;
}

export interface PodmanContainerMetadata {
  podName?: string;
  podId?: string;
  infra?: boolean;
  composeProject?: string;
  composeService?: string;
  composeWorkdir?: string;
  composeConfigHash?: string;
  autoUpdatePolicy?: string;
  autoUpdateRestart?: string;
  userNamespace?: string;
}

export interface Host {
  id: string;
  hostname: string;
  displayName: string;
  platform?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  cpuCount?: number;
  cpuUsage?: number;
  loadAverage?: number[];
  memory: Memory;
  disks?: Disk[];
  diskIO?: HostDiskIO[];
  networkInterfaces?: HostNetworkInterface[];
  sensors?: HostSensorSummary;
  raid?: HostRAIDArray[];
  status: string;
  uptimeSeconds?: number;
  lastSeen: number;
  intervalSeconds?: number;
  agentVersion?: string;
  commandsEnabled?: boolean; // Whether AI command execution is enabled on this agent
  tokenId?: string;
  tokenName?: string;
  tokenHint?: string;
  tokenLastUsedAt?: number;
  revokedTokenId?: string;
  tokenRevokedAt?: number;
  tags?: string[];
  isLegacy?: boolean;
  // Linking: When this host agent is running on a known PVE entity
  linkedNodeId?: string; // ID of PVE node this agent is running on
  linkedVmId?: string; // ID of VM this agent is running inside
  linkedContainerId?: string; // ID of container this agent is running inside
}

export interface HostNetworkInterface {
  name: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
  speedMbps?: number;
}

export interface HostSensorSummary {
  temperatureCelsius?: Record<string, number>;
  fanRpm?: Record<string, number>;
  additional?: Record<string, number>;
  smart?: HostDiskSMART[]; // S.M.A.R.T. disk data
}

export interface HostDiskSMART {
  device: string;        // Device name (e.g., sda)
  model?: string;        // Disk model
  serial?: string;       // Serial number
  wwn?: string;          // World Wide Name
  type?: string;         // Transport type: sata, sas, nvme
  temperature: number;   // Temperature in Celsius
  health?: string;       // PASSED, FAILED, UNKNOWN
  standby?: boolean;     // True if disk was in standby
}

export interface HostRAIDArray {
  device: string;
  name?: string;
  level: string;
  state: string;
  totalDevices: number;
  activeDevices: number;
  workingDevices: number;
  failedDevices: number;
  spareDevices: number;
  uuid?: string;
  devices: HostRAIDDevice[];
  rebuildPercent: number;
  rebuildSpeed?: string;
}

export interface HostRAIDDevice {
  device: string;
  state: string;
  slot: number;
}

export interface HostDiskIO {
  device: string;
  readBytes?: number;
  writeBytes?: number;
  readOps?: number;
  writeOps?: number;
  readTimeMs?: number;
  writeTimeMs?: number;
  ioTimeMs?: number;
}

export interface HostLookupResponse {
  success: boolean;
  host: {
    id: string;
    hostname: string;
    displayName?: string;
    status: string;
    connected: boolean;
    lastSeen: number;
    agentVersion?: string;
  };
}

export interface ReplicationJob {
  id: string;
  instance: string;
  jobId: string;
  jobNumber?: number;
  guest?: string;
  guestId?: number;
  guestName?: string;
  guestType?: string;
  guestNode?: string;
  sourceNode?: string;
  sourceStorage?: string;
  targetNode?: string;
  targetStorage?: string;
  schedule?: string;
  type?: string;
  enabled: boolean;
  state?: string;
  status?: string;
  lastSyncStatus?: string;
  lastSyncTime?: number;
  lastSyncUnix?: number;
  lastSyncDurationSeconds?: number;
  lastSyncDurationHuman?: string;
  nextSyncTime?: number;
  nextSyncUnix?: number;
  durationSeconds?: number;
  durationHuman?: string;
  failCount?: number;
  error?: string;
  comment?: string;
  removeJob?: string;
  rateLimitMbps?: number;
  polledAt?: number;
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
  // Added for deduplication in storage view
  nodes?: string[];
  nodeIds?: string[];
  nodeCount?: number;
  pbsNames?: string[];
  // ZFS pool status
  zfsPool?: ZFSPool;
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

export interface ZFSPool {
  name: string;
  state: string; // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
  status: string; // Healthy, Degraded, Faulted, etc.
  scan: string; // Current scan status (scrub, resilver, none)
  readErrors: number;
  writeErrors: number;
  checksumErrors: number;
  devices: ZFSDevice[];
}

export interface ZFSDevice {
  name: string;
  type: string; // disk, mirror, raidz, raidz2, raidz3, spare, log, cache
  state: string; // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
  readErrors: number;
  writeErrors: number;
  checksumErrors: number;
  message?: string;
}

export interface PBSInstance {
  id: string;
  name: string;
  host: string;
  guestURL?: string; // Optional guest-accessible URL (for navigation)
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

export interface PMGInstance {
  id: string;
  name: string;
  host: string;
  guestURL?: string; // Optional guest-accessible URL (for navigation)
  status: string;
  version: string;
  nodes?: PMGNodeStatus[];
  mailStats?: PMGMailStats;
  mailCount?: PMGMailCountPoint[];
  spamDistribution?: PMGSpamBucket[];
  quarantine?: PMGQuarantineTotals;
  connectionHealth: string;
  lastSeen: string;
  lastUpdated: string;
}

export interface PMGNodeStatus {
  name: string;
  status: string;
  role?: string;
  uptime?: number;
  loadAvg?: string;
  queueStatus?: PMGQueueStatus;
}

export interface PMGMailStats {
  timeframe: string;
  countTotal: number;
  countIn: number;
  countOut: number;
  spamIn: number;
  spamOut: number;
  virusIn: number;
  virusOut: number;
  bouncesIn: number;
  bouncesOut: number;
  bytesIn: number;
  bytesOut: number;
  greylistCount: number;
  junkIn: number;
  averageProcessTimeMs: number;
  rblRejects: number;
  pregreetRejects: number;
  updatedAt: string;
}

export interface PMGMailCountPoint {
  timestamp: string;
  count: number;
  countIn: number;
  countOut: number;
  spamIn: number;
  spamOut: number;
  virusIn: number;
  virusOut: number;
  rblRejects: number;
  pregreet: number;
  bouncesIn: number;
  bouncesOut: number;
  greylist: number;
  index: number;
  timeframe: string;
  windowStart?: string;
  windowEnd?: string;
}

export interface PMGSpamBucket {
  score: string;
  count: number;
}

export interface PMGQuarantineTotals {
  spam: number;
  virus: number;
  attachment?: number;
  blacklisted?: number;
}

export interface PMGQueueStatus {
  active: number;
  deferred: number;
  hold: number;
  incoming: number;
  total: number;
  oldestAge: number;
  updatedAt: string;
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

export interface PMGBackup {
  id: string;
  instance: string;
  node: string;
  filename: string;
  backupTime: string;
  size: number;
}

export interface Backups {
  pve: PVEBackups;
  pbs: PBSBackup[];
  pmg: PMGBackup[];
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
  balloon?: number;
  swapUsed?: number;
  swapTotal?: number;
}

export interface Disk {
  total: number;
  used: number;
  free: number;
  usage: number;
  mountpoint?: string;
  type?: string;
  device?: string;
}

export interface PhysicalDisk {
  id: string;
  node: string;
  instance: string;
  devPath: string;
  device?: string;
  model: string;
  serial: string;
  type: 'nvme' | 'sata' | 'sas' | string;
  size: number;
  health: 'PASSED' | 'FAILED' | 'UNKNOWN' | string;
  wearout: number; // 0-100, 100 is best, -1 when the controller doesn't report it
  temperature: number;
  rpm: number;
  used: string;
  lastChecked: string;
  smart?: unknown;
}

export interface CPUInfo {
  model: string;
  cores: number;
  sockets: number;
  mhz: string;
}

export interface Temperature {
  cpuPackage?: number; // CPU package temperature (primary metric)
  cpuMax?: number; // Highest core temperature
  cpuMin?: number; // Minimum recorded CPU temperature (since monitoring started)
  cpuMaxRecord?: number; // Maximum recorded CPU temperature (since monitoring started)
  minRecorded?: string; // When minimum temperature was recorded
  maxRecorded?: string; // When maximum temperature was recorded
  cores?: CoreTemp[]; // Individual core temperatures
  gpu?: GPUTemp[]; // GPU temperatures
  nvme?: NVMeTemp[]; // NVMe drive temperatures
  available: boolean; // Whether any temperature data is available
  hasCPU?: boolean; // Whether CPU temperature data is available
  hasGPU?: boolean; // Whether GPU temperature data is available
  hasNVMe?: boolean; // Whether NVMe temperature data is available
  lastUpdate: string; // When this data was collected
}

export interface CoreTemp {
  core: number;
  temp: number;
}

export interface GPUTemp {
  device: string; // GPU device identifier (e.g., "amdgpu-pci-0400")
  edge?: number; // Edge temperature
  junction?: number; // Junction/hotspot temperature
  mem?: number; // Memory temperature
}

export interface NVMeTemp {
  device: string;
  temp: number;
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
  instance: string;
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
  instance: string;
  type: string;
  vmid: number;
  time: string;
  description: string;
  parent: string;
  vmstate: boolean;
  sizeBytes?: number;
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

export interface IncidentEvent {
  id: string;
  type: string;
  timestamp: string;
  summary: string;
  details?: Record<string, unknown>;
}

export interface Incident {
  id: string;
  alertId: string;
  alertType: string;
  level: string;
  resourceId: string;
  resourceName: string;
  resourceType?: string;
  node?: string;
  instance?: string;
  message?: string;
  status: string;
  openedAt: string;
  closedAt?: string;
  acknowledged: boolean;
  ackUser?: string;
  ackTime?: string;
  events?: IncidentEvent[];
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
  | {
    type: 'update:progress';
    data: {
      phase: string;
      progress: number;
      message: string;
    };
  }
  | {
    type: 'node_auto_registered';
    data: {
      type: string;
      host: string;
      name: string;
      tokenId: string;
      hasToken: boolean;
      verifySSL?: boolean;
      status?: string;
    };
  }
  | { type: 'node_deleted'; data: { nodeType: string } }
  | { type: 'nodes_changed'; data?: unknown }
  | {
    type: 'discovery_update';
    data: {
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
      immediate?: boolean;
      scanning?: boolean;
      cached?: boolean;
    };
  }
  | {
    type: 'discovery_started';
    data?: {
      subnet?: string;
      timestamp?: number;
      scanning?: boolean;
    };
  }
  | {
    type: 'discovery_complete';
    data?: {
      timestamp?: number;
      scanning?: boolean;
    };
  };

// Utility types
export type Status = 'running' | 'stopped' | 'paused' | 'unknown';
export type GuestType = 'qemu' | 'lxc';
