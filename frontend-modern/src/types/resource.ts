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
  PBSBackupJob,
  PBSGarbageJob,
  PBSJobHealthEvidence,
  PBSPruneJob,
  PBSSyncJob,
  PBSVerifyJob,
} from '@/types/api';
import {
  PLATFORM_TYPE_KEYS as GENERATED_PLATFORM_TYPE_KEYS,
  type GeneratedPlatformType,
} from '@/utils/platformSupportManifest.generated';

// Resource types - what kind of entity is being monitored
export type ResourceType =
  | 'agent' // Unified infrastructure machine / Pulse Unified Agent
  | 'docker-host' // Docker/Podman host
  | 'k8s-cluster' // Kubernetes cluster
  | 'k8s-node' // Kubernetes node
  | 'vm' // Proxmox VM
  | 'system-container' // System container (LXC, Incus, BSD jail)
  | 'app-container' // Application container (Docker, Podman)
  | 'oci-container' // OCI container (Proxmox VE 9.1+)
  | 'pod' // Kubernetes pod
  | 'jail' // BSD jail
  | 'docker-service' // Docker Swarm service
  | 'docker-image' // Docker/Podman image
  | 'docker-volume' // Docker/Podman volume
  | 'docker-network' // Docker/Podman network
  | 'docker-task' // Docker Swarm task
  | 'docker-swarm-node' // Docker Swarm node
  | 'docker-secret' // Docker Swarm secret metadata
  | 'docker-config' // Docker Swarm config metadata
  | 'k8s-deployment' // Kubernetes deployment
  | 'k8s-replicaset' // Kubernetes replica set
  | 'k8s-service' // Kubernetes service
  | 'k8s-namespace' // Kubernetes namespace
  | 'k8s-statefulset' // Kubernetes stateful set
  | 'k8s-daemonset' // Kubernetes daemon set
  | 'k8s-job' // Kubernetes job
  | 'k8s-cronjob' // Kubernetes cron job
  | 'k8s-ingress' // Kubernetes ingress
  | 'k8s-endpoint-slice' // Kubernetes endpoint slice
  | 'k8s-network-policy' // Kubernetes network policy
  | 'k8s-persistent-volume' // Kubernetes persistent volume
  | 'k8s-persistent-volume-claim' // Kubernetes persistent volume claim
  | 'k8s-storage-class' // Kubernetes storage class
  | 'k8s-configmap' // Kubernetes config map
  | 'k8s-secret' // Kubernetes secret metadata
  | 'k8s-serviceaccount' // Kubernetes service account
  | 'k8s-resource-quota' // Kubernetes resource quota
  | 'k8s-limit-range' // Kubernetes limit range
  | 'k8s-pod-disruption-budget' // Kubernetes pod disruption budget
  | 'k8s-horizontal-pod-autoscaler' // Kubernetes horizontal pod autoscaler
  | 'k8s-event' // Kubernetes event
  | 'storage' // Storage resource
  | 'network' // Virtual/network topology resource
  | 'datastore' // PBS datastore
  | 'pool' // ZFS/Ceph pool
  | 'dataset' // ZFS dataset
  | 'pbs' // Proxmox Backup Server
  | 'pmg' // Proxmox Mail Gateway
  | 'physical_disk' // Physical disk
  | 'network-share' // SMB/NFS network share
  | 'ceph' // Ceph cluster
  | 'network-endpoint'; // Agentless availability endpoint

// Platform types - which system the resource comes from
export const PLATFORM_TYPES = GENERATED_PLATFORM_TYPE_KEYS;
export type PlatformType = GeneratedPlatformType | 'availability' | 'generic' | 'unraid';

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

// Provider-native incident associated with a unified resource.
export interface ResourceIncident {
  provider?: string;
  nativeId?: string;
  code: string;
  severity: string;
  source?: string;
  summary: string;
  startedAt?: string;
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

export type ResourceSensitivity = 'public' | 'internal' | 'sensitive' | 'restricted';

export type ResourceRoutingScope = 'cloud-summary' | 'local-first' | 'local-only';

export type ResourceRedactionHint = 'hostname' | 'ip-address' | 'platform-id' | 'alias' | 'path';

export interface ResourceRoutingPolicy {
  scope: ResourceRoutingScope;
  redact?: ResourceRedactionHint[];
}

export interface ResourcePolicy {
  sensitivity: ResourceSensitivity;
  routing: ResourceRoutingPolicy;
}

export interface ResourcePolicyPostureSummary {
  totalResources: number;
  sensitivityCounts: Partial<Record<ResourceSensitivity, number>>;
  routingCounts: Partial<Record<ResourceRoutingScope, number>>;
  redactionCounts: Partial<Record<ResourceRedactionHint, number>>;
}

export const requiresGovernedResourceDisplay = (policy?: ResourcePolicy | null): boolean => {
  if (!policy) return false;
  return policy.routing.scope === 'local-only' || (policy.routing.redact?.length ?? 0) > 0;
};

export type ResourceApprovalLevel = 'none' | 'dry_run_only' | 'admin' | 'mfa';
export type ResourceChangeConfidence = 'high' | 'medium' | 'low';
export type ResourceChangeKind =
  | 'state_transition'
  | 'activity'
  | 'restart'
  | 'config_update'
  | 'metric_anomaly'
  | 'relationship_change'
  | 'capability_change'
  | 'alert_fired'
  | 'alert_acknowledged'
  | 'alert_unacknowledged'
  | 'alert_resolved'
  | 'command_executed'
  | 'runbook_executed';
export type ResourceChangeSourceType =
  | 'platform_event'
  | 'pulse_diff'
  | 'heuristic'
  | 'user_action'
  | 'agent_action';
export type ResourceChangeSourceAdapter =
  | 'docker_adapter'
  | 'proxmox_adapter'
  | 'truenas_adapter'
  | 'vmware_adapter'
  | 'agent:ops-helper'
  | string;

export type ResourceFacetSourceAdapter =
  | 'docker_adapter'
  | 'proxmox_adapter'
  | 'truenas_adapter'
  | 'vmware_adapter'
  | 'agent:ops-helper';

export interface ResourceFacetCounts {
  recentChanges: number;
  recentChangeKinds?: Partial<Record<ResourceChangeKind, number>>;
  recentChangeSourceTypes?: Partial<Record<ResourceChangeSourceType, number>>;
  recentChangeSourceAdapters?: Partial<Record<ResourceFacetSourceAdapter, number>>;
}

export type ResourceRelationshipType =
  | 'runs_on'
  | 'depends_on'
  | 'mounted_to'
  | 'exposed_by'
  | 'owned_by'
  | string;

export interface ResourceRelationship {
  sourceId: string;
  targetId: string;
  type: ResourceRelationshipType;
  confidence: number;
  active: boolean;
  discoverer: string;
  observedAt: string;
  lastSeenAt: string;
  metadata?: Record<string, unknown>;
}

export interface ResourceCapabilityParam {
  name: string;
  type: string;
  required: boolean;
  enum?: string[];
  pattern?: string;
  defaultValue?: unknown;
  isSensitive: boolean;
  description?: string;
}

export interface ResourceCapability {
  name: string;
  type: string;
  description?: string;
  minimumApprovalLevel?: string;
  platform?: string;
  params?: ResourceCapabilityParam[];
}

export interface ResourceChange {
  id: string;
  observedAt: string;
  occurredAt?: string;
  resourceId: string;
  kind: ResourceChangeKind;
  from?: string;
  to?: string;
  sourceType: ResourceChangeSourceType;
  sourceAdapter?: ResourceChangeSourceAdapter;
  confidence: ResourceChangeConfidence;
  actor?: string;
  relatedResources?: string[];
  reason?: string;
  metadata?: Record<string, unknown>;
}

export type MetricsHistoryTargetResourceType =
  | 'agent'
  | 'vm'
  | 'system-container'
  | 'oci-container'
  | 'app-container'
  | 'storage'
  | 'docker-host'
  | 'k8s-cluster'
  | 'k8s-node'
  | 'k8s-deployment'
  | 'pod'
  | 'disk';

export interface ResourceMetricsTarget {
  resourceType: MetricsHistoryTargetResourceType;
  resourceId: string;
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
  pool?: string;
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
  jobHealthEvidenceCount?: number;
  backupJobs?: PBSBackupJob[];
  syncJobs?: PBSSyncJob[];
  verifyJobs?: PBSVerifyJob[];
  pruneJobs?: PBSPruneJob[];
  garbageJobs?: PBSGarbageJob[];
  jobHealthEvidence?: PBSJobHealthEvidence[];
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

export interface ResourceCephPoolMeta {
  name: string;
  storedBytes: number;
  availableBytes: number;
  objects: number;
  percentUsed: number;
}

export interface ResourceCephServiceMeta {
  type: string;
  running: number;
  total: number;
}

export interface ResourceCephMeta {
  fsid?: string;
  healthStatus: string;
  healthMessage?: string;
  numMons: number;
  numMgrs: number;
  numOsds: number;
  numOsdsUp: number;
  numOsdsIn: number;
  numPGs: number;
  pools?: ResourceCephPoolMeta[];
  services?: ResourceCephServiceMeta[];
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
  spunDown?: boolean;
  readCount?: number;
  writeCount?: number;
  errorCount?: number;
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

export interface ResourceAgentUnraidDisk {
  name?: string;
  device?: string;
  role?: string;
  status?: string;
  rawStatus?: string;
  model?: string;
  serial?: string;
  filesystem?: string;
  transport?: string;
  sizeBytes?: number;
  usedBytes?: number;
  freeBytes?: number;
  temperature?: number;
  spunDown?: boolean;
  readCount?: number;
  writeCount?: number;
  errorCount?: number;
  slot?: number;
}

export interface ResourceAgentUnraidMeta {
  arrayStarted?: boolean;
  arrayState?: string;
  syncAction?: string;
  syncProgress?: number;
  syncErrors?: number;
  numProtected?: number;
  numDisabled?: number;
  numInvalid?: number;
  numMissing?: number;
  disks?: ResourceAgentUnraidDisk[];
  risk?: ResourceStorageRisk;
  riskSummary?: string;
  postureSummary?: string;
  protectionReduced?: boolean;
  protectionSummary?: string;
  rebuildInProgress?: boolean;
  rebuildSummary?: string;
}

export interface ResourceAgentMeta {
  agentId?: string;
  agentVersion?: string;
  hostname?: string;
  platform?: string;
  hostProfile?: string;
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
  unraid?: ResourceAgentUnraidMeta;
  storageRisk?: ResourceStorageRisk;
  storageRiskSummary?: string;
  storagePostureSummary?: string;
  protectionReduced?: boolean;
  protectionSummary?: string;
  rebuildInProgress?: boolean;
  rebuildSummary?: string;
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
  pool?: string;
  instance?: string;
  clusterName?: string;
  cpus?: number;
  template?: boolean;
  disks?: Disk[];
  swapUsed?: number;
  swapTotal?: number;
  balloon?: number;
  isOci?: boolean;
  osTemplate?: string;
  pveVersion?: string;
  kernelVersion?: string;
}

// Docker runtime, container, and Swarm service projection emitted by the
// canonical adapter. Host resources use the runtime fields on the Docker
// platform hosts table; `app-container` resources use the container fields
// on the Docker containers table; `docker-service` resources use the service
// fields on the Swarm services table.
export interface ResourceDockerMeta {
  serviceId?: string;
  serviceName?: string;
  hostSourceId?: string;
  containerId?: string;
  hostname?: string;
  displayName?: string;
  customDisplayName?: string;
  machineId?: string;
  temperature?: number;
  runtime?: string;
  runtimeVersion?: string;
  dockerVersion?: string;
  os?: string;
  kernelVersion?: string;
  architecture?: string;
  agentVersion?: string;
  uptimeSeconds?: number;
  containerCount?: number;
  imageCount?: number;
  volumeCount?: number;
  networkCount?: number;
  nodeCount?: number;
  secretCount?: number;
  configCount?: number;
  updatesAvailableCount?: number;
  updatesLastCheckedAt?: string;
  imagesUsage?: DockerStorageUsageMeta;
  containersUsage?: DockerStorageUsageMeta;
  volumesUsage?: DockerStorageUsageMeta;
  buildCacheUsage?: DockerStorageUsageMeta;
  command?: Record<string, unknown>;
  image?: string;
  imageId?: string;
  repoTags?: string[];
  repoDigests?: string[];
  sizeBytes?: number;
  sharedSizeBytes?: number;
  imageContainers?: number;
  containerState?: string;
  health?: string;
  restartCount?: number;
  exitCode?: number;
  ports?: Array<{
    privatePort?: number;
    publicPort?: number;
    protocol?: string;
    ip?: string;
  }>;
  mounts?: Array<{
    type?: string;
    source?: string;
    destination?: string;
    mode?: string;
    rw?: boolean;
  }>;
  networks?: Array<{
    name?: string;
    ipv4?: string;
    ipv6?: string;
  }>;
  updateStatus?: {
    updateAvailable?: boolean;
    currentDigest?: string;
    latestDigest?: string;
    lastChecked?: string;
    error?: string;
  };
  serviceUpdate?: {
    state?: string;
    message?: string;
    completedAt?: string;
  };
  mode?: string;
  desiredTasks?: number;
  runningTasks?: number;
  completedTasks?: number;
  endpointPorts?: Array<{
    protocol?: string;
    targetPort?: number;
    publishedPort?: number;
    publishMode?: string;
  }>;
  labels?: Record<string, string>;
  volumeName?: string;
  driver?: string;
  mountpoint?: string;
  scope?: string;
  createdAt?: string;
  refCount?: number;
  options?: Record<string, string>;
  networkId?: string;
  enableIpv4?: boolean;
  enableIpv6?: boolean;
  internal?: boolean;
  attachable?: boolean;
  ingress?: boolean;
  configOnly?: boolean;
  subnets?: Array<{
    subnet?: string;
    gateway?: string;
  }>;
  taskId?: string;
  nodeId?: string;
  nodeName?: string;
  nodeRole?: string;
  availability?: string;
  address?: string;
  managerReachability?: string;
  managerAddress?: string;
  leader?: boolean;
  engineVersion?: string;
  nanoCpus?: number;
  memoryBytes?: number;
  engineLabels?: Record<string, string>;
  secretId?: string;
  secretName?: string;
  configId?: string;
  configName?: string;
  templatingDriver?: string;
  objectCreatedAt?: string;
  objectUpdatedAt?: string;
  slot?: number;
  desiredState?: string;
  currentState?: string;
  error?: string;
  message?: string;
  startedAt?: string;
  completedAt?: string;
  swarm?: {
    clusterId?: string;
    clusterName?: string;
    nodeId?: string;
    nodeRole?: string;
    localState?: string;
    controlAvailable?: boolean;
    scope?: string;
    error?: string;
  };
}

export interface DockerStorageUsageMeta {
  totalCount?: number;
  activeCount?: number;
  totalSizeBytes?: number;
  reclaimableBytes?: number;
}

export interface ResourceTrueNASAppHostPort {
  hostPort?: number;
  hostIp?: string;
}

export interface ResourceTrueNASAppPort {
  containerPort?: number;
  protocol?: string;
  hostPorts?: ResourceTrueNASAppHostPort[];
}

export interface ResourceTrueNASAppVolume {
  source?: string;
  destination?: string;
  mode?: string;
  type?: string;
}

export interface ResourceTrueNASAppContainer {
  id?: string;
  serviceName?: string;
  image?: string;
  state?: string;
  portConfig?: ResourceTrueNASAppPort[];
  volumeMounts?: ResourceTrueNASAppVolume[];
}

export interface ResourceTrueNASAppNetwork {
  id?: string;
  name?: string;
  labels?: Record<string, string>;
}

export interface ResourceTrueNASAppStats {
  intervalSeconds?: number;
  collectedAt?: string;
}

export interface ResourceTrueNASAppMeta {
  id?: string;
  name?: string;
  state?: string;
  version?: string;
  humanVersion?: string;
  customApp?: boolean;
  upgradeAvailable?: boolean;
  imageUpdatesAvailable?: boolean;
  notes?: string;
  containerCount?: number;
  usedHostIps?: string[];
  usedPorts?: ResourceTrueNASAppPort[];
  containers?: ResourceTrueNASAppContainer[];
  volumes?: ResourceTrueNASAppVolume[];
  images?: string[];
  networks?: ResourceTrueNASAppNetwork[];
  stats?: ResourceTrueNASAppStats;
}

export interface ResourceTrueNASVMMeta {
  id?: string;
  name?: string;
  description?: string;
  state?: string;
  domainState?: string;
  pid?: number;
  vcpus?: number;
  cores?: number;
  threads?: number;
  memoryBytes?: number;
  minMemoryBytes?: number;
  cpuMode?: string;
  cpuModel?: string;
  bootloader?: string;
  autostart?: boolean;
  suspendOnSnapshot?: boolean;
  trustedPlatformModule?: boolean;
  secureBoot?: boolean;
  time?: string;
  archType?: string;
  machineType?: string;
  uuid?: string;
  displayAvailable?: boolean;
  deviceCount?: number;
  diskCount?: number;
  nicCount?: number;
  displayCount?: number;
  cdromCount?: number;
  usbCount?: number;
  pciCount?: number;
}

export interface ResourceTrueNASShareMeta {
  id?: string;
  name?: string;
  protocol?: string;
  path?: string;
  dataset?: string;
  relativePath?: string;
  comment?: string;
  enabled?: boolean;
  readOnly?: boolean;
  browsable?: boolean;
  locked?: boolean;
  accessBasedEnumeration?: boolean;
  auditEnabled?: boolean;
  exposeSnapshots?: boolean;
  aliases?: string[];
  hosts?: string[];
  networks?: string[];
  security?: string[];
  mapRootUser?: string;
  mapRootGroup?: string;
  mapAllUser?: string;
  mapAllGroup?: string;
}

export interface ResourceTrueNASServiceMeta {
  id?: string;
  service?: string;
  enabled?: boolean;
  state?: string;
  pids?: number[];
}

export interface ResourceTrueNASMeta {
  hostname?: string;
  version?: string;
  uptimeSeconds?: number;
  storageRisk?: ResourceStorageRisk;
  storageRiskSummary?: string;
  storagePostureSummary?: string;
  protectionReduced?: boolean;
  protectionSummary?: string;
  rebuildInProgress?: boolean;
  rebuildSummary?: string;
  app?: ResourceTrueNASAppMeta;
  vm?: ResourceTrueNASVMMeta;
  share?: ResourceTrueNASShareMeta;
  services?: ResourceTrueNASServiceMeta[];
}

export interface ResourceKubernetesMetricCapabilities {
  nodeCpuMemory?: boolean;
  nodeTelemetry?: boolean;
  podCpuMemory?: boolean;
  podNetwork?: boolean;
  podEphemeralDisk?: boolean;
  podDiskIo?: boolean;
}

export interface ResourceKubernetesPodContainerStatus {
  name?: string;
  image?: string;
  ready?: boolean;
  restartCount?: number;
  state?: string;
  reason?: string;
  message?: string;
}

export interface ResourceKubernetesMeta {
  clusterId?: string;
  agentId?: string;
  clusterName?: string;
  resourceUid?: string;
  resourceKind?: string;
  context?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
  uptimeSeconds?: number;
  temperature?: number;
  pendingUninstall?: boolean;
  metricCapabilities?: ResourceKubernetesMetricCapabilities;
  // Pod and workload-controller fields populated by the canonical
  // Kubernetes adapter. Surfaced on Kubernetes platform-page native tables
  // where CPU/memory/uptime/temperature columns from the generic
  // infrastructure table are not the meaningful operator columns.
  podPhase?: string;
  podReason?: string;
  podMessage?: string;
  podContainers?: ResourceKubernetesPodContainerStatus[];
  restarts?: number;
  ownerKind?: string;
  ownerName?: string;
  image?: string;
  labels?: Record<string, string>;
  deploymentUid?: string;
  replicaSetUid?: string;
  statefulSetUid?: string;
  daemonSetUid?: string;
  serviceUid?: string;
  jobUid?: string;
  cronJobUid?: string;
  ingressUid?: string;
  endpointSliceUid?: string;
  networkPolicyUid?: string;
  persistentVolumeUid?: string;
  persistentVolumeClaimUid?: string;
  storageClassUid?: string;
  configMapUid?: string;
  secretUid?: string;
  serviceAccountUid?: string;
  resourceQuotaUid?: string;
  limitRangeUid?: string;
  podDisruptionBudgetUid?: string;
  horizontalPodAutoscalerUid?: string;
  eventUid?: string;
  namespaceUid?: string;
  desiredReplicas?: number;
  updatedReplicas?: number;
  readyReplicas?: number;
  availableReplicas?: number;
  currentReplicas?: number;
  fullyLabeledReplicas?: number;
  observedGeneration?: number;
  desiredNumberScheduled?: number;
  currentNumberScheduled?: number;
  numberReady?: number;
  numberAvailable?: number;
  numberUnavailable?: number;
  numberMisscheduled?: number;
  serviceName?: string;
  serviceType?: string;
  clusterIp?: string;
  externalIps?: string[];
  servicePorts?: Array<{
    name?: string;
    protocol?: string;
    port?: number;
    targetPort?: string;
    nodePort?: number;
  }>;
  selector?: Record<string, string>;
  succeeded?: number;
  failed?: number;
  active?: number;
  schedule?: string;
  suspend?: boolean;
  lastScheduleTime?: string;
  lastSuccessfulTime?: string;
  startTime?: string;
  completionTime?: string;
  className?: string;
  hosts?: string[];
  addresses?: string[];
  addressType?: string;
  endpointCount?: number;
  readyEndpointCount?: number;
  endpointPorts?: Array<{
    name?: string;
    protocol?: string;
    port?: number;
    appProtocol?: string;
  }>;
  policyTypes?: string[];
  ingressRuleCount?: number;
  egressRuleCount?: number;
  phase?: string;
  storageClass?: string;
  provisioner?: string;
  volumeBindingMode?: string;
  allowVolumeExpansion?: boolean;
  parameterKeys?: string[];
  dataKeys?: string[];
  binaryDataKeys?: string[];
  immutable?: boolean;
  metadataOnly?: boolean;
  secretType?: string;
  automountServiceAccountToken?: boolean;
  secretCount?: number;
  imagePullSecrets?: string[];
  hard?: Record<string, string>;
  used?: Record<string, string>;
  limitTypes?: string[];
  minAvailable?: string;
  maxUnavailable?: string;
  desiredHealthy?: number;
  currentHealthy?: number;
  disruptionsAllowed?: number;
  expectedPods?: number;
  targetKind?: string;
  targetName?: string;
  minReplicas?: number;
  maxReplicas?: number;
  metricTypes?: string[];
  capacityBytes?: number;
  requestedBytes?: number;
  accessModes?: string[];
  reclaimPolicy?: string;
  claimNamespace?: string;
  claimName?: string;
  volumeName?: string;
  eventType?: string;
  reason?: string;
  message?: string;
  involvedKind?: string;
  involvedName?: string;
  count?: number;
  firstSeen?: string;
  eventTime?: string;
  createdAt?: string;
  // Cluster-only fields surfaced on the Kubernetes platform-page
  // Clusters table for at-a-glance fleet posture.
  version?: string;
  server?: string;
  // Node-only fields surfaced on the Kubernetes platform-page Nodes
  // table. The canonical adapter populates these on `k8s-node` rows
  // and on `agent` rows whose linked host has been merged into a K8s
  // node by the backend registry.
  nodeUid?: string;
  kubeletVersion?: string;
  containerRuntimeVersion?: string;
  osImage?: string;
  architecture?: string;
  kernelVersion?: string;
  roles?: string[];
  ready?: boolean;
  capacityCpuCores?: number;
  allocatableCpuCores?: number;
  capacityMemoryBytes?: number;
  allocatableMemoryBytes?: number;
  capacityPods?: number;
  allocatablePods?: number;
}

// Proxmox Mail Gateway projection emitted by the canonical adapter for
// `pmg` resources. Surfaced on the Proxmox platform-page Mail Gateway
// table where queue posture / spam / virus / quarantine counts are
// the operator columns, not generic CPU/Memory bars.
export interface ResourcePMGMeta {
  instanceId?: string;
  hostname?: string;
  version?: string;
  nodeCount?: number;
  uptimeSeconds?: number;
  connectionHealth?: string;
  mailCountTotal?: number;
  spamIn?: number;
  virusIn?: number;
  quarantine?: number;
  queueIncoming?: number;
  queueActive?: number;
  queueDeferred?: number;
  queueHold?: number;
  queueTotal?: number;
  lastUpdated?: string | number;
}

export interface ResourceVMwareSnapshot {
  snapshot?: string;
  name?: string;
  description?: string;
  id?: number;
  createdAt?: string | number;
  state?: string;
  quiesced?: boolean;
  replaySupported?: boolean;
  current?: boolean;
  children?: ResourceVMwareSnapshot[];
}

export interface ResourceVMwareNetworkAdapter {
  nic?: string;
  label?: string;
  type?: string;
  macType?: string;
  macAddress?: string;
  pciSlotNumber?: number;
  backingType?: string;
  networkId?: string;
  networkName?: string;
  distributedSwitchUuid?: string;
  distributedPort?: string;
  opaqueNetworkType?: string;
  opaqueNetworkId?: string;
  hostDevice?: string;
  state?: string;
  startConnected?: boolean;
  allowGuestControl?: boolean;
  wakeOnLanEnabled?: boolean;
  uptCompatibilityEnabled?: boolean;
  uptV2CompatibilityEnabled?: boolean;
}

export interface ResourceVMwareVirtualDisk {
  disk?: string;
  label?: string;
  type?: string;
  idePrimary?: boolean;
  ideMaster?: boolean;
  scsiBus?: number;
  scsiUnit?: number;
  sataBus?: number;
  sataUnit?: number;
  nvmeBus?: number;
  nvmeUnit?: number;
  backingType?: string;
  vmdkFile?: string;
  datastoreName?: string;
  capacityBytes?: number;
}

export interface ResourceVMwareTools {
  autoUpdateSupported?: boolean;
  installAttemptCount?: number;
  errorMessage?: string;
  versionNumber?: number;
  version?: string;
  upgradePolicy?: string;
  versionStatus?: string;
  installType?: string;
  runState?: string;
  guestRebootRequested?: boolean;
  guestRebootComponents?: string[];
  guestRebootRequestTime?: string | number;
}

export interface ResourceVMwareBootDevice {
  type?: string;
  nic?: string;
  disks?: string[];
}

export interface ResourceVMwareHardware {
  guestOs?: string;
  instantCloneFrozen?: boolean;
  version?: string;
  upgradePolicy?: string;
  upgradeVersion?: string;
  upgradeStatus?: string;
  upgradeErrorMessage?: string;
  bootType?: string;
  efiLegacyBoot?: boolean;
  bootNetworkProtocol?: string;
  bootDelayMilliseconds?: number;
  bootRetry?: boolean;
  bootRetryDelayMilliseconds?: number;
  enterSetupMode?: boolean;
  bootDevices?: ResourceVMwareBootDevice[];
  cpuCoresPerSocket?: number;
  cpuHotAddEnabled?: boolean;
  cpuHotRemoveEnabled?: boolean;
  memoryHotAddEnabled?: boolean;
  memoryHotAddIncrementMib?: number;
  memoryHotAddLimitMib?: number;
}

export interface ResourceVMwareMeta {
  connectionId?: string;
  connectionName?: string;
  vcenterHost?: string;
  managedObjectId?: string;
  entityType?: string;
  hostUuid?: string;
  datacenterId?: string;
  datacenterName?: string;
  computeResourceId?: string;
  computeResourceName?: string;
  clusterId?: string;
  clusterName?: string;
  clusterHaEnabled?: boolean;
  clusterDrsEnabled?: boolean;
  folderId?: string;
  folderName?: string;
  resourcePoolId?: string;
  resourcePoolName?: string;
  runtimeHostId?: string;
  runtimeHostName?: string;
  connectionState?: string;
  powerState?: string;
  overallStatus?: string;
  cpuCount?: number;
  memorySizeMib?: number;
  datastoreType?: string;
  datastoreIds?: string[];
  datastoreNames?: string[];
  datastoreUrl?: string;
  datastoreAccessible?: boolean;
  multipleHostAccess?: boolean;
  maintenanceMode?: string;
  networkType?: string;
  networkHostIds?: string[];
  networkHostNames?: string[];
  networkVmIds?: string[];
  networkVmNames?: string[];
  instanceUuid?: string;
  biosUuid?: string;
  guestOsFamily?: string;
  guestHostname?: string;
  guestIpAddresses?: string[];
  activeAlarmCount?: number;
  activeAlarmSummary?: string;
  recentTaskCount?: number;
  recentTaskSummary?: string;
  snapshotCount?: number;
  currentSnapshotId?: string;
  snapshotTree?: ResourceVMwareSnapshot[];
  networkAdapters?: ResourceVMwareNetworkAdapter[];
  virtualDisks?: ResourceVMwareVirtualDisk[];
  tools?: ResourceVMwareTools;
  hardware?: ResourceVMwareHardware;
}

export interface ResourceAvailabilityMeta {
  targetId?: string;
  name?: string;
  address?: string;
  protocol?: string;
  port?: number;
  path?: string;
  enabled?: boolean;
  available?: boolean;
  lastChecked?: string;
  lastSuccess?: string;
  latencyMillis?: number;
  consecutiveFailures?: number;
  lastError?: string;
  failureThreshold?: number;
  pollIntervalSeconds?: number;
  timeoutMillis?: number;
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
  platformScopes?: string[];
  sourceType: SourceType;
  sources?: string[];

  // Hierarchy
  parentId?: string; // Parent resource (e.g., VM -> Node)
  parentName?: string; // Parent display identifier when available
  childCount?: number; // Backend-derived count of direct child resources
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
  incidents?: ResourceIncident[];
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
  metricsTarget?: ResourceMetricsTarget;

  // Backend-provided canonical identity contract for labels and stable aliases
  canonicalIdentity?: ResourceCanonicalIdentity;
  policy?: ResourcePolicy;
  aiSafeSummary?: string;
  capabilities?: ResourceCapability[];
  relationships?: ResourceRelationship[];
  recentChanges?: ResourceChange[];
  facetCounts?: ResourceFacetCounts;

  // Common source facets (optional; not all backends/state payloads include these).
  // Prefer these over casting `platformData` when available.
  agent?: ResourceAgentMeta;
  kubernetes?: ResourceKubernetesMeta;
  docker?: ResourceDockerMeta;
  truenas?: ResourceTrueNASMeta;
  pmg?: ResourcePMGMeta;
  vmware?: ResourceVMwareMeta;
  proxmox?: ResourceProxmoxMeta;
  pbs?: ResourcePBSMeta;
  availability?: ResourceAvailabilityMeta;
  physicalDisk?: ResourcePhysicalDiskMeta;
  storage?: ResourceStorageMeta;
  ceph?: ResourceCephMeta;

  // Platform-specific data (varies by type)
  platformData?: Record<string, unknown>;
}

/**
 * Helper type guards
 */
export function isInfrastructure(r: Resource): boolean {
  return [
    'agent',
    'docker-host',
    'docker-swarm-node',
    'k8s-cluster',
    'k8s-node',
    'network-endpoint',
  ].includes(r.type);
}

export function isWorkload(r: Resource): boolean {
  return ['vm', 'system-container', 'app-container', 'oci-container', 'pod', 'jail'].includes(
    r.type,
  );
}

export function isStorage(r: Resource): boolean {
  return [
    'storage',
    'datastore',
    'pool',
    'dataset',
    'physical_disk',
    'network-share',
    'ceph',
  ].includes(r.type);
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
