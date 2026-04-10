import { batch, createEffect, createSignal, onCleanup } from 'solid-js';
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
import {
  getPreferredNormalizedPlatformId,
  getPreferredResourceClusterName,
} from '@/utils/resourceIdentity';
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
    clusterId: getPreferredResourceClusterName(v2),
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
};

export function useUnifiedResources(options?: UseUnifiedResourcesOptions) {
  const query = normalizeUnifiedResourcesQuery(options?.query ?? DEFAULT_UNIFIED_RESOURCES_QUERY);
  const cacheKey = (options?.cacheKey || query || 'all').trim();
  const typeFilter = parseUnifiedResourcesTypeFilter(query);
  const supportsCanonicalWsHydration = query === '' || typeFilter !== null;
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

  const refetch = async () => runRefetch({ force: true, source: 'manual' });

  const mutate = (value: Resource[] | ((prev: Resource[]) => Resource[])) => {
    const current = resources as unknown as Resource[];
    const next = typeof value === 'function' ? value(current) : value;
    applyResources(next ?? []);
    return resources as unknown as Resource[];
  };

  // If cache is stale, refresh it in the background without blocking initial render.
  if (!hasFreshUnifiedResourcesCache(cacheEntry)) {
    void runRefetch({ source: 'initial' }).catch((err) => {
      logger.warn('[useUnifiedResources] Failed background refresh for stale cache', err);
    });
  }

  const scheduleRefetch = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }

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
    const currentOrgScope = orgScope();
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
    const projectedResources = filterCanonicalUnifiedResources(wsResources, query, typeFilter);
    const now = Date.now();
    const allResourcesEntry = getUnifiedResourcesCacheEntry(
      buildScopedUnifiedResourcesCacheKey(ALL_RESOURCES_CACHE_KEY, currentOrgScope),
    );
    setUnifiedResourcesCache(allResourcesEntry, wsResources, now);
    allResourcesEntry.lastFetchAt = now;

    if (projectedResources === null) {
      return;
    }

    setUnifiedResourcesCache(cacheEntry, projectedResources, now);
    cacheEntry.lastFetchAt = now;
    batch(() => {
      setResources(reconcile(projectedResources, { key: 'id' }));
      setError(undefined);
      setLoading(false);
    });
  });

  createEffect(() => {
    orgScope();

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

    const scopedResources = cacheEntry.resources;
    batch(() => {
      setError(undefined);
      setResources(reconcile(scopedResources, { key: 'id' }));
      setLoading(!cacheEntry.hasSnapshot);
    });

    if (!hasFreshUnifiedResourcesCache(cacheEntry)) {
      void runRefetch({ force: true, source: 'initial' }).catch(() => undefined);
    }
  });

  onCleanup(() => {
    unsubscribeOrgSwitch();
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }
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
