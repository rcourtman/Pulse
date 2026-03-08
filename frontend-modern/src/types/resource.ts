/**
 * Unified Resource Types
 *
 * These types define the unified resource model that normalizes all monitored
 * entities (VMs, containers, agents, etc.) into a common structure.
 *
 * The frontend receives these via WebSocket state.resources[].
 */

import type {
  Disk,
  HostNetworkInterface,
  HostSensorSummary,
  HostRAIDArray,
  Memory,
} from '@/types/api';

// Resource types - what kind of entity is being monitored
export type ResourceType =
  | 'agent' // Unified infrastructure machine / Pulse Unified Agent
  | 'docker-host' // Docker/Podman host
  | 'k8s-cluster' // Kubernetes cluster
  | 'k8s-node' // Kubernetes node
  | 'truenas' // TrueNAS system
  | 'vm' // Proxmox VM
  | 'system-container' // System container (LXC, Incus, BSD jail)
  | 'app-container' // Application container (Docker, Podman)
  | 'oci-container' // OCI container (Proxmox VE 9.1+)
  | 'pod' // Kubernetes pod
  | 'jail' // BSD jail
  | 'docker-service' // Docker Swarm service
  | 'k8s-deployment' // Kubernetes deployment
  | 'k8s-service' // Kubernetes service
  | 'storage' // Storage resource
  | 'datastore' // PBS datastore
  | 'pool' // ZFS/Ceph pool
  | 'dataset' // ZFS dataset
  | 'pbs' // Proxmox Backup Server
  | 'pmg' // Proxmox Mail Gateway
  | 'physical_disk' // Physical disk
  | 'ceph'; // Ceph cluster

// Platform types - which system the resource comes from
export type PlatformType =
  | 'proxmox-pve'
  | 'proxmox-pbs'
  | 'proxmox-pmg'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'agent';

// Source types - how data is collected
export type SourceType =
  | 'api' // Data from polling an API
  | 'agent' // Data pushed from agent
  | 'hybrid'; // Both sources, agent preferred

// Resource status - operational state
export type ResourceStatus =
  | 'online'
  | 'offline'
  | 'running'
  | 'stopped'
  | 'degraded'
  | 'paused'
  | 'unknown';

// Metric value with optional limits
export interface ResourceMetric {
  current: number; // Current value (percentage or bytes)
  total?: number; // Total capacity (bytes) - null for percentages
  used?: number; // Used amount (bytes)
  free?: number; // Free amount (bytes)
}

// Network I/O metrics (rates in bytes/sec from backend)
export interface ResourceNetwork {
  rxBytes: number; // Inbound rate (bytes/sec)
  txBytes: number; // Outbound rate (bytes/sec)
}

// Disk I/O metrics (rates in bytes/sec from backend)
export interface ResourceDiskIO {
  readRate: number; // Read rate (bytes/sec)
  writeRate: number; // Write rate (bytes/sec)
}

// Alert associated with a resource
export interface ResourceAlert {
  id: string;
  type: string; // cpu, memory, disk, temperature, etc.
  level: string; // warning, critical
  message: string;
  value: number;
  threshold: number;
  startTime: number; // Unix milliseconds
}

// Identity information for deduplication
export interface ResourceIdentity {
  hostname?: string;
  machineId?: string;
  ips?: string[];
  clusterName?: string;
}

export interface ResourceDiscoveryTarget {
  resourceType: 'agent' | 'vm' | 'system-container' | 'app-container' | 'pod' | 'disk' | 'ceph';
  agentId: string;
  resourceId: string;
  hostname?: string;
}

export interface ResourceCanonicalIdentity {
  displayName?: string;
  hostname?: string;
  platformId?: string;
  primaryId?: string;
  aliases?: string[];
}

export interface ResourceStorageConsumer {
  resourceId?: string;
  resourceType: ResourceType | string;
  name: string;
  diskCount?: number;
}

export interface ResourceStorageRiskReason {
  code: string;
  severity: string;
  summary: string;
}

export interface ResourceStorageRisk {
  level: string;
  reasons?: ResourceStorageRiskReason[];
}

export interface ResourceStorageMeta {
  type?: string;
  content?: string;
  contentTypes?: string[];
  shared?: boolean;
  isCeph?: boolean;
  isZfs?: boolean;
  platform?: string;
  topology?: string;
  protection?: string;
  risk?: ResourceStorageRisk;
  riskSummary?: string;
  consumerCount?: number;
  consumerTypes?: string[];
  topConsumers?: ResourceStorageConsumer[];
  consumerImpactSummary?: string;
  postureSummary?: string;
  protectionReduced?: boolean;
  protectionSummary?: string;
  rebuildInProgress?: boolean;
  rebuildSummary?: string;
  nodes?: string[];
  path?: string;
  zfsPoolState?: string;
  zfsReadErrors?: number;
  zfsWriteErrors?: number;
  zfsChecksumErrors?: number;
  arrayState?: string;
  syncAction?: string;
  syncProgress?: number;
  numProtected?: number;
  numDisabled?: number;
  numInvalid?: number;
  numMissing?: number;
}

export interface ResourcePBSMeta {
  instanceId?: string;
  hostname?: string;
  version?: string;
  uptimeSeconds?: number;
  datastoreCount?: number;
  backupJobCount?: number;
  syncJobCount?: number;
  verifyJobCount?: number;
  pruneJobCount?: number;
  garbageJobCount?: number;
  connectionHealth?: string;
  affectedDatastoreCount?: number;
  affectedDatastores?: string[];
  affectedDatastoreSummary?: string;
  protectedWorkloadCount?: number;
  protectedWorkloadTypes?: string[];
  protectedWorkloadNames?: string[];
  protectedWorkloadSummary?: string;
  postureSummary?: string;
  storageRisk?: ResourceStorageRisk;
}

export interface ResourcePhysicalDiskRisk {
  level: string;
  reasons?: ResourceStorageRiskReason[];
}

export interface ResourcePhysicalDiskMeta {
  devPath?: string;
  model?: string;
  serial?: string;
  wwn?: string;
  diskType?: string;
  sizeBytes?: number;
  health?: string;
  wearout?: number;
  temperature?: number;
  rpm?: number;
  used?: string;
  storageRole?: string;
  storageGroup?: string;
  storageState?: string;
  smart?: {
    powerOnHours?: number;
    powerCycles?: number;
    reallocatedSectors?: number;
    pendingSectors?: number;
    offlineUncorrectable?: number;
    udmaCrcErrors?: number;
    percentageUsed?: number;
    availableSpare?: number;
    mediaErrors?: number;
    unsafeShutdowns?: number;
  };
  risk?: ResourcePhysicalDiskRisk;
}

export interface ResourceAgentDisk {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
}

export interface ResourceAgentMeta {
  agentId?: string;
  agentVersion?: string;
  hostname?: string;
  platform?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  uptimeSeconds?: number;
  cpuCount?: number;
  memory?: Partial<Memory>;
  networkInterfaces?: HostNetworkInterface[];
  disks?: ResourceAgentDisk[];
  sensors?: HostSensorSummary;
  raid?: HostRAIDArray[];
  commandsEnabled?: boolean;
  tokenId?: string;
  tokenName?: string;
  tokenHint?: string;
  tokenLastUsedAt?: number;
}

export interface ResourceProxmoxMeta {
  vmid?: number;
  node?: string;
  nodeName?: string;
  instance?: string;
  clusterName?: string;
  cpus?: number;
  template?: boolean;
  disks?: Disk[];
  swapUsed?: number;
  swapTotal?: number;
  balloon?: number;
}

export interface ResourceKubernetesMetricCapabilities {
  nodeCpuMemory?: boolean;
  nodeTelemetry?: boolean;
  podCpuMemory?: boolean;
  podNetwork?: boolean;
  podEphemeralDisk?: boolean;
  podDiskIo?: boolean;
}

export interface ResourceKubernetesMeta {
  clusterId?: string;
  agentId?: string;
  clusterName?: string;
  context?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
  uptimeSeconds?: number;
  temperature?: number;
  pendingUninstall?: boolean;
  metricCapabilities?: ResourceKubernetesMetricCapabilities;
}

/**
 * The core unified Resource type.
 * This is what the frontend receives from WebSocket state.resources[].
 */
export interface Resource {
  // Identity
  id: string;
  type: ResourceType;
  technology?: string; // Implementation detail: "qemu", "lxc", "docker", "podman", etc.
  name: string;
  displayName: string;

  // Platform/Source
  platformId: string;
  platformType: PlatformType;
  sourceType: SourceType;

  // Hierarchy
  parentId?: string; // Parent resource (e.g., VM -> Node)
  parentName?: string; // Parent display identifier when available
  clusterId?: string; // Cluster membership

  // Universal Metrics
  status: ResourceStatus;
  cpu?: ResourceMetric;
  memory?: ResourceMetric;
  disk?: ResourceMetric;
  network?: ResourceNetwork;
  diskIO?: ResourceDiskIO;
  temperature?: number;
  uptime?: number; // Seconds

  // Metadata
  tags?: string[];
  labels?: Record<string, string>;
  lastSeen: number; // Unix milliseconds
  alerts?: ResourceAlert[];
  incidentCount?: number;
  incidentCode?: string;
  incidentSeverity?: string;
  incidentSummary?: string;
  incidentCategory?: string;
  incidentLabel?: string;
  incidentPriority?: number;
  incidentImpactSummary?: string;
  incidentUrgency?: string;
  incidentAction?: string;

  // Identity for deduplication
  identity?: ResourceIdentity;

  // Canonical discovery request coordinates from backend
  discoveryTarget?: ResourceDiscoveryTarget;

  // Metrics history query coordinates from backend
  metricsTarget?: { resourceType: string; resourceId: string };

  // Backend-provided canonical identity contract for labels and stable aliases
  canonicalIdentity?: ResourceCanonicalIdentity;

  // Common source facets (optional; not all backends/state payloads include these).
  // Prefer these over casting `platformData` when available.
  agent?: ResourceAgentMeta;
  kubernetes?: ResourceKubernetesMeta;
  proxmox?: ResourceProxmoxMeta;
  pbs?: ResourcePBSMeta;
  physicalDisk?: ResourcePhysicalDiskMeta;
  storage?: ResourceStorageMeta;

  // Platform-specific data (varies by type)
  platformData?: Record<string, unknown>;
}

/**
 * Helper type guards
 */
export function isInfrastructure(r: Resource): boolean {
  return ['agent', 'docker-host', 'k8s-cluster', 'k8s-node', 'truenas'].includes(r.type);
}

export function isWorkload(r: Resource): boolean {
  return ['vm', 'system-container', 'app-container', 'oci-container', 'pod', 'jail'].includes(
    r.type,
  );
}

export function isStorage(r: Resource): boolean {
  return ['storage', 'datastore', 'pool', 'dataset', 'physical_disk', 'ceph'].includes(r.type);
}

/**
 * Resource filtering options
 */
export interface ResourceFilter {
  types?: ResourceType[];
  platforms?: PlatformType[];
  statuses?: ResourceStatus[];
  parentId?: string;
  clusterId?: string;
  hasAlerts?: boolean;
  search?: string;
}

/**
 * Helper to get effective display name
 */
export function getDisplayName(r: Resource): string {
  return r.displayName || r.name;
}

/**
 * Helper to get CPU percentage
 */
export function getCpuPercent(r: Resource): number {
  return r.cpu?.current ?? 0;
}

/**
 * Helper to get memory percentage
 */
export function getMemoryPercent(r: Resource): number {
  if (!r.memory) return 0;
  if (r.memory.total && r.memory.used) {
    return (r.memory.used / r.memory.total) * 100;
  }
  return r.memory.current;
}

/**
 * Helper to get disk percentage
 */
export function getDiskPercent(r: Resource): number {
  if (!r.disk) return 0;
  if (r.disk.total && r.disk.used) {
    return (r.disk.used / r.disk.total) * 100;
  }
  return r.disk.current;
}
