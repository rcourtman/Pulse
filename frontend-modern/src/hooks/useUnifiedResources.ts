import { batch, createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { canonicalizeMetricsHistoryTargetType } from '@/api/charts';
import { readAPIErrorMessage } from '@/api/responseUtils';
import { apiFetch, getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { asTrimmedString } from '@/utils/stringUtils';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type {
  Resource,
  ResourceChange,
  ResourceFacetCounts,
  ResourceDiscoveryTarget,
  ResourceMetricsTarget,
  ResourcePBSMeta,
  ResourceStatus,
  ResourceStorageMeta,
  ResourceType,
  ResourceVMwareMeta,
} from '@/types/resource';
import { normalizeDiskArray } from '@/utils/format';
import { logger } from '@/utils/logger';
import { eventBus } from '@/stores/events';
import { canonicalDiscoveryResourceType } from '@/utils/discoveryTarget';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';
import { getPreferredNormalizedPlatformId } from '@/utils/resourceIdentity';
import { getExplicitResourceClusterName } from '@/utils/agentResources';
import {
  resolvePlatformTypeFromSources,
  resolveSourceTypeFromSources,
} from '@/utils/sourcePlatforms';

const UNIFIED_RESOURCES_BASE_URL = '/api/resources';
const DEFAULT_UNIFIED_RESOURCES_QUERY = 'type=agent,docker-host,pbs,pmg,k8s-cluster,k8s-node';
const STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY =
  'type=storage,pbs,pmg,vm,system-container,pod,agent,k8s-cluster,k8s-node,physical_disk,ceph';
const UNIFIED_RESOURCES_PAGE_LIMIT = 100;
const UNIFIED_RESOURCES_MAX_PAGES = 20;
const UNIFIED_RESOURCES_CACHE_MAX_AGE_MS = 15_000;
const UNIFIED_RESOURCES_WS_DEBOUNCE_MS = 800;
const UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS = 2_500;
const UNIFIED_RESOURCES_WS_INITIAL_HYDRATION_WAIT_MS = 1_200;

type APIMetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
};

type APIAgentDiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
};

type APIAgentNetworkInterface = {
  name: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
  speedMbps?: number;
};

type APIAgentSensorSummary = {
  temperatureCelsius?: Record<string, number>;
  fanRpm?: Record<string, number>;
  additional?: Record<string, number>;
  smart?: Array<{
    device: string;
    model?: string;
    serial?: string;
    wwn?: string;
    type?: string;
    temperature: number;
    health?: string;
    standby?: boolean;
  }>;
};

type APIHostRAIDDevice = {
  device: string;
  state: string;
  slot: number;
};

type APIHostRAIDArray = {
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
  devices: APIHostRAIDDevice[];
  rebuildPercent: number;
  rebuildSpeed?: string;
};

type JsonRecord = Record<string, unknown>;

type APIKubernetesData = {
  clusterId?: string;
  clusterName?: string;
  context?: string;
  nodeName?: string;
  uptimeSeconds?: number;
  temperature?: number;
  metricCapabilities?: {
    nodeCpuMemory?: boolean;
    nodeTelemetry?: boolean;
    podCpuMemory?: boolean;
    podNetwork?: boolean;
    podEphemeralDisk?: boolean;
    podDiskIo?: boolean;
  };
};

type APIResource = {
  id: string;
  type?: string;
  name?: string;
  status?: string;
  lastSeen?: string;
  parentName?: string;
  sources?: string[];
  sourceStatus?: Record<string, { status: string; lastSeen: string; error?: string }>;
  identity?: {
    machineId?: string;
    hostnames?: string[];
    ipAddresses?: string[];
    clusterName?: string;
  };
  metrics?: {
    cpu?: APIMetricValue;
    memory?: APIMetricValue;
    disk?: APIMetricValue;
    netIn?: APIMetricValue;
    netOut?: APIMetricValue;
    diskRead?: APIMetricValue;
    diskWrite?: APIMetricValue;
  };
  parentId?: string;
  tags?: string[];
  proxmox?: {
    nodeName?: string;
    clusterName?: string;
    instance?: string;
    vmid?: number;
    cpus?: number;
    uptime?: number;
    temperature?: number;
    template?: boolean;
    disks?: APIAgentDiskInfo[];
    swapUsed?: number;
    swapTotal?: number;
    balloon?: number;
  };
  agent?: {
    agentId?: string;
    agentVersion?: string;
    hostname?: string;
    platform?: string;
    osName?: string;
    osVersion?: string;
    kernelVersion?: string;
    architecture?: string;
    uptimeSeconds?: number;
    temperature?: number;
    cpuCount?: number;
    memory?: {
      total?: number;
      used?: number;
      free?: number;
      usage?: number;
      swapUsed?: number;
      swapTotal?: number;
    };
    networkInterfaces?: APIAgentNetworkInterface[];
    disks?: APIAgentDiskInfo[];
    sensors?: APIAgentSensorSummary;
    raid?: APIHostRAIDArray[];
    commandsEnabled?: boolean;
    tokenId?: string;
    tokenName?: string;
    tokenHint?: string;
    tokenLastUsedAt?: number;
  };
  docker?: {
    hostSourceId?: string;
    hostname?: string;
    temperature?: number;
    runtime?: string;
    runtimeVersion?: string;
    dockerVersion?: string;
    os?: string;
    kernelVersion?: string;
    architecture?: string;
    agentVersion?: string;
    uptimeSeconds?: number;
    swarm?: unknown;
    containerCount?: number;
    updatesAvailableCount?: number;
    updatesLastCheckedAt?: string;
    command?: Record<string, unknown>;
  };
  pbs?: {
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
    storageRisk?: {
      level?: string;
      reasons?: Array<{
        code?: string;
        severity?: string;
        summary?: string;
      }>;
    };
  };
  storage?: {
    type?: string;
    content?: string;
    contentTypes?: string[];
    shared?: boolean;
    isCeph?: boolean;
    isZfs?: boolean;
    platform?: string;
    topology?: string;
    protection?: string;
    risk?: {
      level?: string;
      reasons?: Array<{
        code?: string;
        severity?: string;
        summary?: string;
      }>;
    };
    riskSummary?: string;
    consumerCount?: number;
    consumerTypes?: string[];
    topConsumers?: Array<{
      resourceId?: string;
      resourceType?: string;
      name?: string;
      diskCount?: number;
    }>;
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
  };
  pmg?: {
    instanceId?: string;
    hostname?: string;
    version?: string;
    nodeCount?: number;
    uptimeSeconds?: number;
    queueActive?: number;
    queueDeferred?: number;
    queueHold?: number;
    queueIncoming?: number;
    queueTotal?: number;
    mailCountTotal?: number;
    spamIn?: number;
    virusIn?: number;
    connectionHealth?: string;
    lastUpdated?: string;
  };
  kubernetes?: APIKubernetesData;
  vmware?: {
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
  };
  recentChanges?: ResourceChange[];
  facetCounts?: ResourceFacetCounts;
  physicalDisk?: {
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
  };
  ceph?: {
    fsid?: string;
    healthStatus?: string;
    healthMessage?: string;
    numMons?: number;
    numMgrs?: number;
    numOsds?: number;
    numOsdsUp?: number;
    numOsdsIn?: number;
    numPGs?: number;
    pools?: Array<{
      name: string;
      storedBytes: number;
      availableBytes: number;
      objects: number;
      percentUsed: number;
    }>;
    services?: Array<{
      type: string;
      running: number;
      total: number;
    }>;
  };
  discoveryTarget?: {
    resourceType?: string;
    agentId?: string;
    resourceId?: string;
    hostname?: string;
  };
  metricsTarget?: {
    resourceType?: string;
    resourceId?: string;
  };
  canonicalIdentity?: {
    displayName?: string;
    hostname?: string;
    platformId?: string;
    primaryId?: string;
    aliases?: string[];
  };
  policy?: {
    sensitivity?: string;
    routing?: {
      scope?: string;
      redact?: string[];
    };
  };
  aiSafeSummary?: string;
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
};

type APIListResponse = {
  data?: APIResource[];
  resources?: APIResource[];
  meta?: {
    totalPages?: number;
  };
};

type UnifiedResourcesCacheEntry = {
  resources: Resource[];
  hasSnapshot: boolean;
  cachedAt: number;
  lastFetchAt: number;
  sharedFetch: Promise<Resource[]> | null;
};

const unifiedResourcesCaches = new Map<string, UnifiedResourcesCacheEntry>();
const ALL_RESOURCES_CACHE_KEY = 'all-resources';

const buildScopedUnifiedResourcesCacheKey = (cacheKey: string, orgScope: string): string =>
  `${encodeURIComponent(orgScope)}::${cacheKey}`;

const resolveStatus = (status?: string): ResourceStatus => {
  const normalized = (status || '').toLowerCase();
  if (normalized === 'online' || normalized === 'running') return 'online';
  if (normalized === 'offline' || normalized === 'stopped') return 'offline';
  if (normalized === 'warning' || normalized === 'degraded') return 'degraded';
  if (normalized === 'paused') return 'paused';
  return 'unknown';
};

const resolveType = (value?: string): ResourceType => {
  const normalized = (value || '').toLowerCase();
  const canonicalFrontendType = canonicalizeFrontendResourceType(normalized);
  switch (canonicalFrontendType) {
    case 'agent':
    case 'storage':
    case 'docker-host':
    case 'k8s-cluster':
    case 'k8s-node':
    case 'k8s-deployment':
    case 'k8s-service':
    case 'vm':
    case 'system-container':
    case 'oci-container':
    case 'app-container':
    case 'pod':
    case 'pbs':
    case 'pmg':
    case 'ceph':
      return canonicalFrontendType;
    case 'disk':
      return 'physical_disk';
    default:
      break;
  }

  switch (normalized) {
    case 'jail':
      return 'jail';
    case 'docker-service':
      return 'docker-service';
    case 'k8s-deployment':
      return 'k8s-deployment';
    case 'k8s-service':
      return 'k8s-service';
    case 'storage':
      return 'storage';
    case 'datastore':
      return 'datastore';
    case 'pool':
      return 'pool';
    case 'dataset':
      return 'dataset';
    case 'physical_disk':
    case 'physical-disk':
      return 'physical_disk';
    default:
      return 'agent';
  }
};

const resolveDiscoveryResourceType = (
  value?: string,
): ResourceDiscoveryTarget['resourceType'] | undefined => {
  const normalized = canonicalDiscoveryResourceType(value);
  switch (normalized) {
    case 'agent':
      return 'agent';
    case 'vm':
      return 'vm';
    case 'system-container':
      return 'system-container';
    case 'app-container':
      return 'app-container';
    case 'pod':
      return 'pod';
    case 'disk':
      return 'disk';
    case 'ceph':
      return 'ceph';
    default:
      return undefined;
  }
};

const resolveMetricsTarget = (
  resourceType: string | undefined,
  metricsTarget?: { resourceType?: string; resourceId?: string },
): ResourceMetricsTarget | undefined => {
  const canonicalType = metricsTarget?.resourceType
    ? canonicalizeMetricsHistoryTargetType(metricsTarget.resourceType, resourceType)
    : null;
  const resourceID = asTrimmedString(metricsTarget?.resourceId);
  return canonicalType && resourceID
    ? { resourceType: canonicalType, resourceId: resourceID }
    : undefined;
};

const metricToResourceMetric = (metric?: APIMetricValue) => {
  if (!metric) return undefined;
  const used = metric.used ?? undefined;
  const total = metric.total ?? undefined;
  const current = metric.percent ?? metric.value ?? 0;
  const free = total !== undefined && used !== undefined ? total - used : undefined;
  return {
    current,
    total,
    used,
    free,
  };
};

const asRecord = (value: unknown): JsonRecord | undefined =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as JsonRecord) : undefined;

const mergeStringArrays = (
  incoming?: string[],
  existing?: string[],
): string[] | undefined => {
  const merged = [...(incoming ?? []), ...(existing ?? [])]
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => Boolean(value));
  return merged.length > 0 ? Array.from(new Set(merged)) : undefined;
};

const mergeRecord = <T extends JsonRecord>(incoming?: T, existing?: T): T | undefined => {
  if (!incoming) return existing;
  if (!existing) return incoming;
  return { ...existing, ...incoming };
};

const mergePlatformData = (
  incomingValue: Resource['platformData'],
  existingValue: Resource['platformData'],
): Resource['platformData'] => {
  const incoming = asRecord(incomingValue);
  const existing = asRecord(existingValue);
  if (!incoming) return existingValue;
  if (!existing) return incomingValue;

  const merged: JsonRecord = { ...existing, ...incoming };
  for (const key of [
    'agent',
    'docker',
    'proxmox',
    'pbs',
    'pmg',
    'kubernetes',
    'vmware',
    'storage',
    'physicalDisk',
    'ceph',
    'metrics',
    'discoveryTarget',
  ]) {
    const nested = mergeRecord(asRecord(incoming[key]), asRecord(existing[key]));
    if (nested) {
      merged[key] = nested;
    }
  }

  const sourceStatus = mergeRecord(
    asRecord(incoming.sourceStatus),
    asRecord(existing.sourceStatus),
  );
  if (sourceStatus) {
    merged.sourceStatus = sourceStatus;
  }

  const sources = mergeStringArrays(
    Array.isArray(incoming.sources) ? (incoming.sources as string[]) : undefined,
    Array.isArray(existing.sources) ? (existing.sources as string[]) : undefined,
  );
  if (sources) {
    merged.sources = sources;
  }

  return merged;
};

const deriveLegacySourceList = (resource: Resource): string[] | undefined => {
  switch (resource.platformType) {
    case 'proxmox-pve':
      return resource.sourceType === 'hybrid' ? ['proxmox', 'agent'] : ['proxmox'];
    case 'docker':
      return ['docker'];
    case 'kubernetes':
      return resource.sourceType === 'hybrid' ? ['agent', 'kubernetes'] : ['kubernetes'];
    case 'proxmox-pbs':
      return ['pbs'];
    case 'proxmox-pmg':
      return ['pmg'];
    case 'truenas':
      return ['truenas'];
    case 'vmware-vsphere':
      return ['vmware'];
    default:
      return resource.sourceType === 'agent' ? ['agent'] : undefined;
  }
};

const canonicalizeLegacyPlatformData = (resource: Resource): Resource['platformData'] => {
  const platformData = asRecord(resource.platformData);
  if (!platformData) {
    return resource.platformData;
  }

  const normalized: JsonRecord = { ...platformData };
  const normalizedSources =
    Array.isArray(platformData.sources) && platformData.sources.length > 0
      ? (platformData.sources as string[])
      : deriveLegacySourceList(resource);
  if (normalizedSources && normalizedSources.length > 0) {
    normalized.sources = normalizedSources;
  }

  if (!asRecord(platformData.agent)) {
    const agentPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['agentVersion', 'agentVersion'],
      ['hostname', 'hostname'],
      ['platform', 'platform'],
      ['osName', 'osName'],
      ['osVersion', 'osVersion'],
      ['kernelVersion', 'kernelVersion'],
      ['architecture', 'architecture'],
      ['commandsEnabled', 'commandsEnabled'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        agentPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.memory !== undefined) agentPayload.memory = platformData.memory;
    if (platformData.interfaces !== undefined) agentPayload.networkInterfaces = platformData.interfaces;
    if (platformData.disks !== undefined) agentPayload.disks = platformData.disks;
    if (Object.keys(agentPayload).length > 0) {
      normalized.agent = agentPayload;
    }
  }

  if (!asRecord(platformData.docker)) {
    const dockerPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['runtime', 'runtime'],
      ['runtimeVersion', 'runtimeVersion'],
      ['dockerVersion', 'dockerVersion'],
      ['os', 'os'],
      ['kernelVersion', 'kernelVersion'],
      ['architecture', 'architecture'],
      ['agentVersion', 'agentVersion'],
      ['hostname', 'hostname'],
      ['displayName', 'displayName'],
      ['machineId', 'machineId'],
      ['containerCount', 'containerCount'],
      ['uptimeSeconds', 'uptimeSeconds'],
      ['intervalSeconds', 'intervalSeconds'],
      ['temperature', 'temperature'],
      ['hostSourceId', 'hostSourceId'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        dockerPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.swarm !== undefined) dockerPayload.swarm = platformData.swarm;
    if (platformData.interfaces !== undefined) dockerPayload.networkInterfaces = platformData.interfaces;
    if (platformData.disks !== undefined) dockerPayload.disks = platformData.disks;
    if (Object.keys(dockerPayload).length > 0) {
      normalized.docker = dockerPayload;
    }
  }

  if (!asRecord(platformData.proxmox)) {
    const proxmoxPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['instance', 'instance'],
      ['node', 'nodeName'],
      ['clusterName', 'clusterName'],
      ['vmid', 'vmid'],
      ['cpus', 'cpus'],
      ['template', 'template'],
      ['swapUsed', 'swapUsed'],
      ['swapTotal', 'swapTotal'],
      ['balloon', 'balloon'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        proxmoxPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.disks !== undefined) proxmoxPayload.disks = platformData.disks;
    if (Object.keys(proxmoxPayload).length > 0) {
      normalized.proxmox = proxmoxPayload;
    }
  }

  if (!asRecord(platformData.pbs)) {
    const pbsPayload: JsonRecord = {};
    if (platformData.host !== undefined) pbsPayload.hostname = platformData.host;
    if (platformData.version !== undefined) pbsPayload.version = platformData.version;
    if (platformData.connectionHealth !== undefined) {
      pbsPayload.connectionHealth = platformData.connectionHealth;
    }
    if (platformData.numDatastores !== undefined) {
      pbsPayload.datastoreCount = platformData.numDatastores;
    }
    if (Object.keys(pbsPayload).length > 0) {
      normalized.pbs = pbsPayload;
    }
  }

  if (!asRecord(platformData.pmg)) {
    const pmgPayload: JsonRecord = {};
    if (platformData.host !== undefined) pmgPayload.hostname = platformData.host;
    if (platformData.version !== undefined) pmgPayload.version = platformData.version;
    if (platformData.connectionHealth !== undefined) {
      pmgPayload.connectionHealth = platformData.connectionHealth;
    }
    for (const [legacyKey, nextKey] of [
      ['nodeCount', 'nodeCount'],
      ['queueActive', 'queueActive'],
      ['queueDeferred', 'queueDeferred'],
      ['queueHold', 'queueHold'],
      ['queueIncoming', 'queueIncoming'],
      ['queueTotal', 'queueTotal'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        pmgPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (Object.keys(pmgPayload).length > 0) {
      normalized.pmg = pmgPayload;
    }
  }

  if (!asRecord(platformData.kubernetes)) {
    const kubernetesPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['clusterId', 'clusterId'],
      ['context', 'context'],
      ['nodeName', 'nodeName'],
      ['namespace', 'namespace'],
      ['clusterName', 'clusterName'],
      ['pendingUninstall', 'pendingUninstall'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        kubernetesPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (Object.keys(kubernetesPayload).length > 0) {
      normalized.kubernetes = kubernetesPayload;
    }
  }

  return normalized;
};

const canonicalizeRealtimeResource = (resource: Resource): Resource => {
  const platformData = canonicalizeLegacyPlatformData(resource);
  const platformRecord = asRecord(platformData);
  const normalizedBase = {
    ...resource,
    platformData,
  };
  return {
    ...normalizedBase,
    clusterId: resource.clusterId ?? getExplicitResourceClusterName(normalizedBase),
    platformData,
    agent: resource.agent ?? (platformRecord?.agent as Resource['agent']),
    proxmox: resource.proxmox ?? (platformRecord?.proxmox as Resource['proxmox']),
    pbs: resource.pbs ?? (platformRecord?.pbs as Resource['pbs']),
    kubernetes: resource.kubernetes ?? (platformRecord?.kubernetes as Resource['kubernetes']),
    vmware: resource.vmware ?? (platformRecord?.vmware as Resource['vmware']),
    storage: resource.storage ?? (platformRecord?.storage as Resource['storage']),
    physicalDisk:
      resource.physicalDisk ?? (platformRecord?.physicalDisk as Resource['physicalDisk']),
  };
};

const mergeCanonicalIdentity = (
  incoming?: Resource['canonicalIdentity'],
  existing?: Resource['canonicalIdentity'],
): Resource['canonicalIdentity'] => {
  if (!incoming) return existing;
  if (!existing) return incoming;
  const aliases = mergeStringArrays(incoming.aliases, existing.aliases);
  return {
    ...existing,
    ...incoming,
    aliases,
  };
};

const mergeCanonicalResource = (incoming: Resource, existing?: Resource): Resource => {
  if (!existing) {
    return incoming;
  }
  return {
    ...existing,
    ...incoming,
    clusterId: incoming.clusterId ?? existing.clusterId,
    discoveryTarget: incoming.discoveryTarget ?? existing.discoveryTarget,
    metricsTarget: incoming.metricsTarget ?? existing.metricsTarget,
    canonicalIdentity: mergeCanonicalIdentity(incoming.canonicalIdentity, existing.canonicalIdentity),
    policy: incoming.policy ?? existing.policy,
    aiSafeSummary: incoming.aiSafeSummary ?? existing.aiSafeSummary,
    recentChanges: incoming.recentChanges ?? existing.recentChanges,
    facetCounts: incoming.facetCounts ?? existing.facetCounts,
    diskIO: incoming.diskIO ?? existing.diskIO,
    agent: mergeRecord(incoming.agent as JsonRecord | undefined, existing.agent as JsonRecord | undefined) as Resource['agent'],
    proxmox: mergeRecord(incoming.proxmox as JsonRecord | undefined, existing.proxmox as JsonRecord | undefined) as Resource['proxmox'],
    pbs: mergeRecord(incoming.pbs as JsonRecord | undefined, existing.pbs as JsonRecord | undefined) as Resource['pbs'],
    kubernetes: mergeRecord(incoming.kubernetes as JsonRecord | undefined, existing.kubernetes as JsonRecord | undefined) as Resource['kubernetes'],
    vmware: mergeRecord(incoming.vmware as JsonRecord | undefined, existing.vmware as JsonRecord | undefined) as Resource['vmware'],
    storage: mergeRecord(incoming.storage as JsonRecord | undefined, existing.storage as JsonRecord | undefined) as Resource['storage'],
    physicalDisk: mergeRecord(incoming.physicalDisk as JsonRecord | undefined, existing.physicalDisk as JsonRecord | undefined) as Resource['physicalDisk'],
    identity: mergeRecord(incoming.identity as JsonRecord | undefined, existing.identity as JsonRecord | undefined) as Resource['identity'],
    platformData: mergePlatformData(incoming.platformData, existing.platformData),
    tags: incoming.tags && incoming.tags.length > 0 ? incoming.tags : existing.tags,
    labels:
      incoming.labels && Object.keys(incoming.labels).length > 0 ? incoming.labels : existing.labels,
  };
};

const mergeCanonicalResourceSnapshot = (
  incoming: Resource[],
  existing: Resource[],
): Resource[] => {
  if (incoming.length === 0) {
    return [];
  }
  const existingById = new Map(existing.map((resource) => [resource.id, resource] as const));
  return incoming.map((resource) =>
    mergeCanonicalResource(canonicalizeRealtimeResource(resource), existingById.get(resource.id)),
  );
};

const toResource = (v2: APIResource): Resource => {
  const sources = (v2.sources || []).filter(
    (s): s is string => typeof s === 'string' && s.trim().length > 0,
  );
  const lastSeen = v2.lastSeen ? Date.parse(v2.lastSeen) : NaN;
  const canonical = v2.canonicalIdentity;
  const name = asTrimmedString(canonical?.displayName) || v2.name || v2.id;
  const platformId = asTrimmedString(canonical?.platformId) || getPreferredNormalizedPlatformId(v2);

  const discoveryResourceType = resolveDiscoveryResourceType(v2.discoveryTarget?.resourceType);
  const discoveryAgentId = v2.discoveryTarget?.agentId;
  const discoveryTarget =
    discoveryResourceType && discoveryAgentId && v2.discoveryTarget?.resourceId
      ? {
          resourceType: discoveryResourceType,
          agentId: discoveryAgentId,
          resourceId: v2.discoveryTarget.resourceId,
          hostname: v2.discoveryTarget.hostname,
        }
      : undefined;

  const metricsTarget = resolveMetricsTarget(v2.type, v2.metricsTarget);
  return {
    id: v2.id,
    type: resolveType(v2.type),
    name,
    displayName: name,
    platformId,
    platformType: resolvePlatformTypeFromSources(sources) || 'agent',
    sourceType: resolveSourceTypeFromSources(sources),
    parentId: v2.parentId,
    parentName: v2.parentName,
    clusterId: getExplicitResourceClusterName(v2),
    status: resolveStatus(v2.status),
    incidentCount: v2.incidentCount,
    incidentCode: v2.incidentCode,
    incidentSeverity: v2.incidentSeverity,
    incidentSummary: v2.incidentSummary,
    incidentCategory: v2.incidentCategory,
    incidentLabel: v2.incidentLabel,
    incidentPriority: v2.incidentPriority,
    incidentImpactSummary: v2.incidentImpactSummary,
    incidentUrgency: v2.incidentUrgency,
    incidentAction: v2.incidentAction,
    agent: v2.agent,
    kubernetes: v2.kubernetes,
    vmware: v2.vmware as ResourceVMwareMeta | undefined,
    pbs: v2.pbs as ResourcePBSMeta | undefined,
    physicalDisk: v2.physicalDisk,
    storage: v2.storage as ResourceStorageMeta | undefined,
    proxmox: v2.proxmox
      ? {
          vmid: v2.proxmox.vmid,
          node: v2.proxmox.nodeName,
          instance: v2.proxmox.instance,
          cpus: v2.proxmox.cpus,
          template: v2.proxmox.template,
          disks: normalizeDiskArray(v2.proxmox.disks),
          swapUsed: v2.proxmox.swapUsed,
          swapTotal: v2.proxmox.swapTotal,
          balloon: v2.proxmox.balloon,
        }
      : undefined,
    cpu: metricToResourceMetric(v2.metrics?.cpu),
    memory: metricToResourceMetric(v2.metrics?.memory),
    disk: metricToResourceMetric(v2.metrics?.disk),
    network:
      v2.metrics?.netIn || v2.metrics?.netOut
        ? {
            rxBytes: v2.metrics?.netIn?.value ?? 0,
            txBytes: v2.metrics?.netOut?.value ?? 0,
          }
        : undefined,
    diskIO:
      v2.metrics?.diskRead || v2.metrics?.diskWrite
        ? {
            readRate: v2.metrics?.diskRead?.value ?? 0,
            writeRate: v2.metrics?.diskWrite?.value ?? 0,
          }
        : undefined,
    uptime:
      v2.agent?.uptimeSeconds ??
      v2.proxmox?.uptime ??
      v2.pbs?.uptimeSeconds ??
      v2.pmg?.uptimeSeconds ??
      v2.kubernetes?.uptimeSeconds,
    temperature:
      v2.agent?.temperature ??
      v2.proxmox?.temperature ??
      v2.docker?.temperature ??
      v2.kubernetes?.temperature ??
      v2.physicalDisk?.temperature,
    tags: v2.tags,
    lastSeen: Number.isFinite(lastSeen) ? lastSeen : Date.now(),
    identity: {
      hostname: asTrimmedString(canonical?.hostname) || v2.identity?.hostnames?.[0],
      machineId: v2.identity?.machineId,
      ips: v2.identity?.ipAddresses,
    },
    discoveryTarget,
    metricsTarget,
    canonicalIdentity: v2.canonicalIdentity,
    policy: v2.policy as Resource['policy'],
    aiSafeSummary: v2.aiSafeSummary as Resource['aiSafeSummary'],
    recentChanges: v2.recentChanges,
    facetCounts: v2.facetCounts,
    platformData: {
      sources,
      sourceStatus: v2.sourceStatus,
      proxmox: v2.proxmox,
      agent: v2.agent,
      docker: v2.docker,
      pbs: v2.pbs,
      storage: v2.storage,
      pmg: v2.pmg,
      kubernetes: v2.kubernetes,
      vmware: v2.vmware,
      physicalDisk: v2.physicalDisk,
      ceph: v2.ceph,
      metrics: v2.metrics,
      discoveryTarget: v2.discoveryTarget,
    },
  };
};

const normalizeUnifiedResourcesQuery = (query?: string): string =>
  (query || '').trim().replace(/^\?+/, '');

const buildUnifiedResourcesUrl = (query: string, page: number): string => {
  const params = new URLSearchParams(query);
  params.set('page', String(page));
  params.set('limit', String(UNIFIED_RESOURCES_PAGE_LIMIT));
  return `${UNIFIED_RESOURCES_BASE_URL}?${params.toString()}`;
};

const resolveResourcesPayload = (payload: unknown): { data: APIResource[]; totalPages: number } => {
  if (Array.isArray(payload)) {
    return { data: payload as APIResource[], totalPages: 1 };
  }
  if (!payload || typeof payload !== 'object') {
    return { data: [], totalPages: 1 };
  }
  const record = payload as APIListResponse;
  const data = Array.isArray(record.data)
    ? record.data
    : Array.isArray(record.resources)
      ? record.resources
      : [];
  const totalPages = Number.isFinite(record.meta?.totalPages)
    ? Math.max(1, Number(record.meta?.totalPages))
    : 1;
  return { data, totalPages };
};

const dedupeResources = (resources: APIResource[]): APIResource[] => {
  const ids = new Set<string>();
  const deduped: APIResource[] = [];
  for (const resource of resources) {
    if (!resource?.id || ids.has(resource.id)) continue;
    ids.add(resource.id);
    deduped.push(resource);
  }
  return deduped;
};

const getUnifiedResourcesCacheEntry = (cacheKey: string): UnifiedResourcesCacheEntry => {
  const existing = unifiedResourcesCaches.get(cacheKey);
  if (existing) {
    return existing;
  }
  const created: UnifiedResourcesCacheEntry = {
    resources: [],
    hasSnapshot: false,
    cachedAt: 0,
    lastFetchAt: 0,
    sharedFetch: null,
  };
  unifiedResourcesCaches.set(cacheKey, created);
  return created;
};

const hasFreshUnifiedResourcesCache = (entry: UnifiedResourcesCacheEntry) =>
  entry.hasSnapshot && Date.now() - entry.cachedAt <= UNIFIED_RESOURCES_CACHE_MAX_AGE_MS;

const setUnifiedResourcesCache = (
  entry: UnifiedResourcesCacheEntry,
  resources: Resource[],
  at = Date.now(),
) => {
  entry.resources = resources;
  entry.hasSnapshot = true;
  entry.cachedAt = at;
};

const parseUnifiedResourcesTypeFilter = (query: string): Set<ResourceType> | null => {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  if (normalizedQuery === '') {
    return null;
  }

  const params = new URLSearchParams(normalizedQuery);
  const types = new Set<ResourceType>();

  for (const [key, value] of params.entries()) {
    if (key !== 'type') {
      return null;
    }

    value
      .split(',')
      .map((candidate) => asTrimmedString(candidate))
      .filter((candidate): candidate is string => candidate !== undefined)
      .forEach((candidate) => {
        types.add(resolveType(candidate));
      });
  }

  return types.size > 0 ? types : null;
};

const filterCanonicalUnifiedResources = (
  resources: Resource[],
  query: string,
  typeFilter: Set<ResourceType> | null,
): Resource[] | null => {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  if (normalizedQuery === '') {
    return resources;
  }
  if (!typeFilter) {
    return null;
  }
  return resources.filter((resource) => typeFilter.has(resolveType(resource.type)));
};

const seedUnifiedResourcesCacheFromAllResources = (
  entry: UnifiedResourcesCacheEntry,
  cacheKey: string,
  query: string,
  orgScope: string,
) => {
  if (entry.hasSnapshot || cacheKey === ALL_RESOURCES_CACHE_KEY) {
    return entry;
  }

  const typeFilter = parseUnifiedResourcesTypeFilter(query);
  if (!typeFilter) {
    return entry;
  }

  const allResourcesEntry = getUnifiedResourcesCacheEntry(
    buildScopedUnifiedResourcesCacheKey(ALL_RESOURCES_CACHE_KEY, orgScope),
  );
  if (!hasFreshUnifiedResourcesCache(allResourcesEntry)) {
    return entry;
  }

  entry.resources = allResourcesEntry.resources.filter((resource) =>
    typeFilter.has(resolveType(resource.type)),
  );
  entry.hasSnapshot = true;
  entry.cachedAt = allResourcesEntry.cachedAt;
  entry.lastFetchAt = allResourcesEntry.lastFetchAt;

  return entry;
};

async function fetchUnifiedResources(query: string): Promise<Resource[]> {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  const allRawResources: APIResource[] = [];
  let totalPages = 1;

  for (let page = 1; page <= totalPages && page <= UNIFIED_RESOURCES_MAX_PAGES; page += 1) {
    const response = await apiFetch(buildUnifiedResourcesUrl(normalizedQuery, page), {
      cache: 'no-store',
    });
    if (!response.ok) {
      const message = await readAPIErrorMessage(
        response,
        `Failed to fetch unified resources (HTTP ${response.status})`,
      );
      throw new Error(message);
    }

    const payload = (await response.json()) as APIListResponse | APIResource[];
    const resolved = resolveResourcesPayload(payload);
    allRawResources.push(...resolved.data);
    totalPages = Math.max(totalPages, resolved.totalPages);
  }

  return dedupeResources(allRawResources).map((resource) => toResource(resource));
}

const fetchUnifiedResourcesShared = async (
  entry: UnifiedResourcesCacheEntry,
  query: string,
  force = false,
): Promise<Resource[]> => {
  if (!force && hasFreshUnifiedResourcesCache(entry)) {
    return entry.resources;
  }

  if (entry.sharedFetch) {
    return entry.sharedFetch;
  }

  const request = (async () => {
    const fetched = await fetchUnifiedResources(query);
    const now = Date.now();
    setUnifiedResourcesCache(entry, fetched, now);
    entry.lastFetchAt = now;
    return fetched;
  })();

  entry.sharedFetch = request;

  try {
    return await request;
  } finally {
    if (entry.sharedFetch === request) {
      entry.sharedFetch = null;
    }
  }
};

const shouldThrottleWsRefetch = (entry: UnifiedResourcesCacheEntry) =>
  Date.now() - entry.lastFetchAt < UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS;

export const __resetUnifiedResourcesCacheForTests = () => {
  unifiedResourcesCaches.clear();
};

export const getCachedUnifiedResources = (options?: {
  cacheKey?: string;
  orgID?: string | null;
}): Resource[] => {
  const cacheKey = (options?.cacheKey || ALL_RESOURCES_CACHE_KEY).trim();
  const scopedCacheKey = buildScopedUnifiedResourcesCacheKey(
    cacheKey,
    normalizeOrgScope(options?.orgID ?? getOrgID()),
  );
  return getUnifiedResourcesCacheEntry(scopedCacheKey).resources;
};

type UseUnifiedResourcesOptions = {
  query?: string;
  cacheKey?: string;
  initialHydration?: 'immediate' | 'prefer-ws';
  enabled?: Accessor<boolean>;
};

export function useUnifiedResources(options?: UseUnifiedResourcesOptions) {
  const query = normalizeUnifiedResourcesQuery(options?.query ?? DEFAULT_UNIFIED_RESOURCES_QUERY);
  const cacheKey = (options?.cacheKey || query || 'all').trim();
  const initialHydration = options?.initialHydration ?? 'immediate';
  const enabled = options?.enabled ?? (() => true);
  const typeFilter = parseUnifiedResourcesTypeFilter(query);
  const supportsCanonicalWsHydration = query === '' || typeFilter !== null;
  const prefersWsInitialHydration =
    initialHydration === 'prefer-ws' && supportsCanonicalWsHydration;
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const resolveScopedCacheKey = () => buildScopedUnifiedResourcesCacheKey(cacheKey, orgScope());
  let cacheEntry = seedUnifiedResourcesCacheFromAllResources(
    getUnifiedResourcesCacheEntry(resolveScopedCacheKey()),
    cacheKey,
    query,
    orgScope(),
  );
  const initialResources = cacheEntry.resources;
  const hasCachedResources = cacheEntry.hasSnapshot;

  const [resources, setResources] = createStore<Resource[]>(initialResources);
  const [loading, setLoading] = createSignal(!hasCachedResources);
  const [error, setError] = createSignal<unknown>(undefined);
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;
  let initialHydrationHandle: ReturnType<typeof setTimeout> | undefined;
  let inFlightRefetch: Promise<Resource[]> | null = null;
  let wsInitialized = false;
  let lastWsUpdateToken = '';
  let blockedWsHydrationToken: string | null = null;
  let scopeVersion = 0;

  const applyResources = (
    next: Resource[],
    targetEntry: UnifiedResourcesCacheEntry = cacheEntry,
  ) => {
    setUnifiedResourcesCache(targetEntry, next);
    if (targetEntry !== cacheEntry) {
      return;
    }
    setResources(reconcile(next, { key: 'id' }));
  };

  const runRefetch = async (options?: {
    force?: boolean;
    source?: 'initial' | 'ws' | 'manual';
  }) => {
    if (!enabled()) {
      return resources as unknown as Resource[];
    }
    if (inFlightRefetch) {
      return inFlightRefetch;
    }

    const force = options?.force === true;
    const source = options?.source ?? 'manual';

    if (!force && source === 'ws' && shouldThrottleWsRefetch(cacheEntry)) {
      return resources as unknown as Resource[];
    }

    const shouldForceNetwork = force || source === 'ws';
    const shouldShowLoading = force || !cacheEntry.hasSnapshot;
    if (shouldShowLoading) {
      setLoading(true);
    }

    const requestVersion = scopeVersion;
    const entryForRequest = cacheEntry;
    const request = (async () => {
      try {
        const fetched = await fetchUnifiedResourcesShared(
          entryForRequest,
          query,
          shouldForceNetwork,
        );
        if (requestVersion !== scopeVersion || entryForRequest !== cacheEntry) {
          return resources as unknown as Resource[];
        }
        batch(() => {
          applyResources(fetched, entryForRequest);
          setError(undefined);
        });
        return fetched;
      } catch (err) {
        setError(err);
        throw err;
      } finally {
        inFlightRefetch = null;
        if (shouldShowLoading) {
          setLoading(false);
        }
      }
    })();

    inFlightRefetch = request;
    return request;
  };

  const refetch = async () => {
    if (!enabled()) {
      return resources as unknown as Resource[];
    }
    return runRefetch({ force: true, source: 'manual' });
  };

  const mutate = (value: Resource[] | ((prev: Resource[]) => Resource[])) => {
    const current = resources as unknown as Resource[];
    const next = typeof value === 'function' ? value(current) : value;
    applyResources(next ?? []);
    return resources as unknown as Resource[];
  };

  const clearInitialHydrationTimeout = () => {
    if (initialHydrationHandle !== undefined) {
      clearTimeout(initialHydrationHandle);
      initialHydrationHandle = undefined;
    }
  };

  const clearRefreshTimeout = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
      refreshHandle = undefined;
    }
  };

  const shouldPreferWsInitialHydration = () =>
    prefersWsInitialHydration &&
    !cacheEntry.hasSnapshot &&
    !wsStore.initialDataReceived() &&
    (!Array.isArray(wsStore.state.resources) || wsStore.state.resources.length === 0);

  const scheduleInitialHydrationFallback = () => {
    if (initialHydrationHandle !== undefined) {
      return;
    }
    initialHydrationHandle = setTimeout(() => {
      initialHydrationHandle = undefined;
      if (cacheEntry.hasSnapshot || wsStore.initialDataReceived()) {
        return;
      }
      void runRefetch({ source: 'initial' }).catch((err) => {
        logger.warn('[useUnifiedResources] Failed deferred initial refresh', err);
      });
    }, UNIFIED_RESOURCES_WS_INITIAL_HYDRATION_WAIT_MS);
  };

  const scheduleRefetch = () => {
    clearRefreshTimeout();

    const elapsedSinceFetch = Date.now() - cacheEntry.lastFetchAt;
    const minIntervalDelay = Math.max(
      0,
      UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS - elapsedSinceFetch,
    );
    const delay = Math.max(UNIFIED_RESOURCES_WS_DEBOUNCE_MS, minIntervalDelay);

    refreshHandle = setTimeout(() => {
      refreshHandle = undefined;
      void runRefetch({ source: 'ws' }).catch((err) => {
        logger.debug('[useUnifiedResources] WebSocket-triggered refetch failed', err);
      });
    }, delay);
  };

  createEffect(() => {
    orgScope();

    if (!enabled()) {
      clearInitialHydrationTimeout();
      clearRefreshTimeout();
      setLoading(false);
      return;
    }

    if (hasFreshUnifiedResourcesCache(cacheEntry)) {
      clearInitialHydrationTimeout();
      return;
    }

    if (shouldPreferWsInitialHydration()) {
      scheduleInitialHydrationFallback();
      return;
    }

    void runRefetch({ source: 'initial' }).catch((err) => {
      logger.warn('[useUnifiedResources] Failed background refresh for stale cache', err);
    });
  });

  createEffect(() => {
    const currentOrgScope = orgScope();
    if (!enabled()) {
      return;
    }
    if (!supportsCanonicalWsHydration) {
      return;
    }
    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');
    if (blockedWsHydrationToken !== null) {
      if (
        lastUpdateToken.length === 0 ||
        lastUpdateToken === '0' ||
        lastUpdateToken === blockedWsHydrationToken
      ) {
        return;
      }
      blockedWsHydrationToken = null;
    }

    const wsResources = Array.isArray(wsStore.state.resources) ? wsStore.state.resources : [];
    const allResourcesEntry = getUnifiedResourcesCacheEntry(
      buildScopedUnifiedResourcesCacheKey(ALL_RESOURCES_CACHE_KEY, currentOrgScope),
    );
    const mergedWsResources = mergeCanonicalResourceSnapshot(
      wsResources,
      allResourcesEntry.resources,
    );
    const projectedResources = filterCanonicalUnifiedResources(mergedWsResources, query, typeFilter);
    const now = Date.now();
    clearInitialHydrationTimeout();
    setUnifiedResourcesCache(allResourcesEntry, mergedWsResources, now);
    allResourcesEntry.lastFetchAt = now;

    if (projectedResources === null) {
      return;
    }

    const mergedProjectedResources = mergeCanonicalResourceSnapshot(
      projectedResources,
      cacheEntry.resources,
    );
    setUnifiedResourcesCache(cacheEntry, mergedProjectedResources, now);
    cacheEntry.lastFetchAt = now;
    batch(() => {
      setResources(reconcile(mergedProjectedResources, { key: 'id' }));
      setError(undefined);
      setLoading(false);
    });
  });

  createEffect(() => {
    orgScope();

    if (!enabled()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      clearInitialHydrationTimeout();
      clearRefreshTimeout();
      return;
    }

    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');

    if (!wsInitialized) {
      wsInitialized = true;
      lastWsUpdateToken = lastUpdateToken;
      if (!supportsCanonicalWsHydration) {
        scheduleRefetch();
      }
      return;
    }

    if (lastUpdateToken === lastWsUpdateToken) {
      return;
    }

    lastWsUpdateToken = lastUpdateToken;
    if (!supportsCanonicalWsHydration) {
      scheduleRefetch();
    }
  });

  const unsubscribeOrgSwitch = eventBus.on('org_switched', (nextOrgID?: string) => {
    const nextOrgScope = normalizeOrgScope(nextOrgID);
    if (nextOrgScope === orgScope()) {
      return;
    }

    scopeVersion += 1;
    blockedWsHydrationToken = supportsCanonicalWsHydration
      ? String(wsStore.state.lastUpdate ?? '')
      : null;
    setOrgScope(nextOrgScope);
    cacheEntry = seedUnifiedResourcesCacheFromAllResources(
      getUnifiedResourcesCacheEntry(resolveScopedCacheKey()),
      cacheKey,
      query,
      nextOrgScope,
    );
    inFlightRefetch = null;
    wsInitialized = false;
    lastWsUpdateToken = '';
    clearInitialHydrationTimeout();

    const scopedResources = cacheEntry.resources;
    batch(() => {
      setError(undefined);
      setResources(reconcile(scopedResources, { key: 'id' }));
      setLoading(enabled() && !cacheEntry.hasSnapshot);
    });

    if (enabled() && !hasFreshUnifiedResourcesCache(cacheEntry)) {
      if (shouldPreferWsInitialHydration()) {
        scheduleInitialHydrationFallback();
      } else {
        void runRefetch({ force: true, source: 'initial' }).catch(() => undefined);
      }
    }
  });

  onCleanup(() => {
    unsubscribeOrgSwitch();
    clearInitialHydrationTimeout();
    clearRefreshTimeout();
  });

  return {
    resources: () => resources,
    refetch,
    mutate,
    loading,
    error,
  };
}

export function useStorageRecoveryResources() {
  return useUnifiedResources({
    query: STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY,
    cacheKey: 'storage-recovery',
  });
}

export default useUnifiedResources;
