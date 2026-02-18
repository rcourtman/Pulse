import { batch, createEffect, createSignal, onCleanup } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { readAPIErrorMessage } from '@/api/responseUtils';
import { apiFetch, getOrgID } from '@/utils/apiClient';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type { Resource, PlatformType, SourceType, ResourceStatus, ResourceType } from '@/types/resource';
import { logger } from '@/utils/logger';
import { eventBus } from '@/stores/events';

const UNIFIED_RESOURCES_BASE_URL = '/api/resources';
const DEFAULT_UNIFIED_RESOURCES_QUERY = 'type=host,pbs,pmg,k8s_cluster,k8s_node';
const STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY =
  'type=storage,pbs,pmg,vm,lxc,container,pod,host,k8s_cluster,k8s_node,physical_disk,ceph';
// Backwards-compat for older call sites.
const _STORAGE_BACKUPS_UNIFIED_RESOURCES_QUERY = STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY;
const UNIFIED_RESOURCES_PAGE_LIMIT = 100;
const UNIFIED_RESOURCES_MAX_PAGES = 20;
const UNIFIED_RESOURCES_CACHE_MAX_AGE_MS = 15_000;
const UNIFIED_RESOURCES_WS_DEBOUNCE_MS = 800;
const UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS = 2_500;
const DEFAULT_ORG_SCOPE = 'default';

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
  proxmox?: { nodeName?: string; clusterName?: string; uptime?: number; temperature?: number };
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
    memory?: { total?: number; used?: number; free?: number; usage?: number };
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
    hostId?: string;
    resourceId?: string;
    hostname?: string;
  };
  metricsTarget?: {
    resourceType?: string;
    resourceId?: string;
  };
};

type APIListResponse = {
  data?: APIResource[];
  resources?: APIResource[];
  meta?: {
    totalPages?: number;
  };
};

type SourceFlags = {
  hasAgent: boolean;
  hasProxmox: boolean;
  hasDocker: boolean;
  hasKubernetes: boolean;
  hasPbs: boolean;
  hasPmg: boolean;
};

type UnifiedResourcesCacheEntry = {
  resources: Resource[];
  cachedAt: number;
  lastFetchAt: number;
  sharedFetch: Promise<Resource[]> | null;
};

const unifiedResourcesCaches = new Map<string, UnifiedResourcesCacheEntry>();

const normalizeOrgScope = (orgID?: string | null): string => {
  const normalized = (orgID || '').trim();
  return normalized || DEFAULT_ORG_SCOPE;
};

const buildScopedUnifiedResourcesCacheKey = (cacheKey: string, orgScope: string): string =>
  `${encodeURIComponent(orgScope)}::${cacheKey}`;

const readSourceFlags = (sources: string[] | undefined): SourceFlags => {
  const flags: SourceFlags = {
    hasAgent: false,
    hasProxmox: false,
    hasDocker: false,
    hasKubernetes: false,
    hasPbs: false,
    hasPmg: false,
  };

  if (!sources || sources.length === 0) {
    return flags;
  }

  for (const source of sources) {
    switch (source.toLowerCase()) {
      case 'agent':
        flags.hasAgent = true;
        break;
      case 'proxmox':
        flags.hasProxmox = true;
        break;
      case 'docker':
        flags.hasDocker = true;
        break;
      case 'kubernetes':
        flags.hasKubernetes = true;
        break;
      case 'pbs':
        flags.hasPbs = true;
        break;
      case 'pmg':
        flags.hasPmg = true;
        break;
      default:
        break;
    }
  }

  return flags;
};

const resolvePlatformType = (flags: SourceFlags): PlatformType => {
  if (flags.hasProxmox) return 'proxmox-pve';
  if (flags.hasPbs) return 'proxmox-pbs';
  if (flags.hasPmg) return 'proxmox-pmg';
  if (flags.hasDocker) return 'docker';
  if (flags.hasKubernetes) return 'kubernetes';
  if (flags.hasAgent) return 'host-agent';
  return 'host-agent';
};

const resolveSourceType = (flags: SourceFlags): SourceType => {
  const hasOther =
    flags.hasProxmox || flags.hasDocker || flags.hasKubernetes || flags.hasPbs || flags.hasPmg;
  if (flags.hasAgent && hasOther) return 'hybrid';
  if (flags.hasAgent) return 'agent';
  return 'api';
};

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
  switch (normalized) {
    case 'host':
      return 'host';
    case 'node':
      return 'node';
    case 'docker-host':
      return 'docker-host';
    case 'k8s-cluster':
      return 'k8s-cluster';
    case 'k8s_cluster':
      return 'k8s-cluster';
    case 'k8s-node':
      return 'k8s-node';
    case 'k8s_node':
      return 'k8s-node';
    case 'truenas':
      return 'truenas';
    case 'vm':
      return 'vm';
    case 'lxc':
      return 'container';
    case 'oci-container':
      return 'oci-container';
    case 'container':
      return 'container';
    case 'docker-container':
      return 'docker-container';
    case 'docker_container':
      return 'docker-container';
    case 'pod':
      return 'pod';
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
    case 'pbs':
      return 'pbs';
    case 'pmg':
      return 'pmg';
    case 'ceph':
      return 'ceph';
    case 'physical_disk':
      return 'physical_disk';
    case 'physical-disk':
      return 'physical_disk';
    default:
      return 'host';
  }
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
  const sources = (v2.sources || []).filter((s): s is string => typeof s === 'string' && s.trim().length > 0);
  const sourceFlags = readSourceFlags(sources);
  const lastSeen = v2.lastSeen ? Date.parse(v2.lastSeen) : NaN;
  const name = v2.name || v2.id;
  const platformId =
    v2.proxmox?.nodeName ||
    v2.agent?.hostname ||
    v2.docker?.hostname ||
    name ||
    v2.id;

  const discoveryTarget =
    v2.discoveryTarget?.resourceType &&
      v2.discoveryTarget?.hostId &&
      v2.discoveryTarget?.resourceId
      ? {
        resourceType: v2.discoveryTarget.resourceType as 'host' | 'vm' | 'lxc' | 'docker' | 'k8s' | 'disk' | 'ceph',
        hostId: v2.discoveryTarget.hostId,
        resourceId: v2.discoveryTarget.resourceId,
        hostname: v2.discoveryTarget.hostname,
      }
      : undefined;

  const metricsTarget =
    v2.metricsTarget?.resourceType && v2.metricsTarget?.resourceId
      ? { resourceType: v2.metricsTarget.resourceType, resourceId: v2.metricsTarget.resourceId }
      : undefined;

  return {
    id: v2.id,
    type: resolveType(v2.type),
    name,
    displayName: name,
    platformId,
    platformType: resolvePlatformType(sourceFlags),
    sourceType: resolveSourceType(sourceFlags),
    parentId: v2.parentId,
    clusterId: v2.identity?.clusterName || v2.proxmox?.clusterName,
    status: resolveStatus(v2.status),
    agent: v2.agent,
    kubernetes: v2.kubernetes,
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
      hostname: v2.identity?.hostnames?.[0],
      machineId: v2.identity?.machineId,
      ips: v2.identity?.ipAddresses,
    },
    discoveryTarget,
    metricsTarget,
    platformData: {
      sources,
      sourceStatus: v2.sourceStatus,
      proxmox: v2.proxmox,
      agent: v2.agent,
      docker: v2.docker,
      pbs: v2.pbs,
      pmg: v2.pmg,
      kubernetes: v2.kubernetes,
      physicalDisk: v2.physicalDisk,
      ceph: v2.ceph,
      metrics: v2.metrics,
      discoveryTarget: v2.discoveryTarget,
    },
  };
};

const normalizeUnifiedResourcesQuery = (query?: string): string => (query || '').trim().replace(/^\?+/, '');

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
    cachedAt: 0,
    lastFetchAt: 0,
    sharedFetch: null,
  };
  unifiedResourcesCaches.set(cacheKey, created);
  return created;
};

const hasFreshUnifiedResourcesCache = (entry: UnifiedResourcesCacheEntry) =>
  entry.resources.length > 0 && Date.now() - entry.cachedAt <= UNIFIED_RESOURCES_CACHE_MAX_AGE_MS;

const setUnifiedResourcesCache = (
  entry: UnifiedResourcesCacheEntry,
  resources: Resource[],
  at = Date.now(),
) => {
  entry.resources = resources;
  entry.cachedAt = at;
};

async function fetchUnifiedResources(query: string): Promise<Resource[]> {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  const allRawResources: APIResource[] = [];
  let totalPages = 1;

  for (let page = 1; page <= totalPages && page <= UNIFIED_RESOURCES_MAX_PAGES; page += 1) {
    const response = await apiFetch(buildUnifiedResourcesUrl(normalizedQuery, page), { cache: 'no-store' });
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

type UseUnifiedResourcesOptions = {
  query?: string;
  cacheKey?: string;
};

export function useUnifiedResources(options?: UseUnifiedResourcesOptions) {
  const query = normalizeUnifiedResourcesQuery(options?.query ?? DEFAULT_UNIFIED_RESOURCES_QUERY);
  const cacheKey = (options?.cacheKey || query || 'all').trim();
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const resolveScopedCacheKey = () => buildScopedUnifiedResourcesCacheKey(cacheKey, orgScope());
  let cacheEntry = getUnifiedResourcesCacheEntry(resolveScopedCacheKey());
  const initialResources = cacheEntry.resources;
  const hasCachedResources = initialResources.length > 0;

  const [resources, setResources] = createStore<Resource[]>(initialResources);
  const [loading, setLoading] = createSignal(!hasCachedResources);
  const [error, setError] = createSignal<unknown>(undefined);
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;
  let inFlightRefetch: Promise<Resource[]> | null = null;
  let wsInitialized = false;
  let lastWsUpdateToken = '';
  let scopeVersion = 0;

  const applyResources = (next: Resource[], targetEntry: UnifiedResourcesCacheEntry = cacheEntry) => {
    setUnifiedResourcesCache(targetEntry, next);
    if (targetEntry !== cacheEntry) {
      return;
    }
    setResources(reconcile(next, { key: 'id' }));
  };

  const runRefetch = async (options?: { force?: boolean; source?: 'initial' | 'ws' | 'manual' }) => {
    if (inFlightRefetch) {
      return inFlightRefetch;
    }

    const force = options?.force === true;
    const source = options?.source ?? 'manual';

    if (!force && source === 'ws' && shouldThrottleWsRefetch(cacheEntry)) {
      return resources as unknown as Resource[];
    }

    const shouldForceNetwork = force || source === 'ws';
    const shouldShowLoading = force || (resources as unknown as Resource[]).length === 0;
    if (shouldShowLoading) {
      setLoading(true);
    }

    const requestVersion = scopeVersion;
    const entryForRequest = cacheEntry;
    const request = (async () => {
      try {
        const fetched = await fetchUnifiedResourcesShared(entryForRequest, query, shouldForceNetwork);
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
    const minIntervalDelay = Math.max(0, UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS - elapsedSinceFetch);
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

    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');

    if (!wsInitialized) {
      wsInitialized = true;
      lastWsUpdateToken = lastUpdateToken;
      scheduleRefetch();
      return;
    }

    if (lastUpdateToken === lastWsUpdateToken) {
      return;
    }

    lastWsUpdateToken = lastUpdateToken;
    scheduleRefetch();
  });

  const unsubscribeOrgSwitch = eventBus.on('org_switched', (nextOrgID?: string) => {
    const nextOrgScope = normalizeOrgScope(nextOrgID);
    if (nextOrgScope === orgScope()) {
      return;
    }

    scopeVersion += 1;
    setOrgScope(nextOrgScope);
    cacheEntry = getUnifiedResourcesCacheEntry(resolveScopedCacheKey());
    inFlightRefetch = null;
    wsInitialized = false;
    lastWsUpdateToken = '';

    const scopedResources = cacheEntry.resources;
    batch(() => {
      setError(undefined);
      setResources(reconcile(scopedResources, { key: 'id' }));
      setLoading(scopedResources.length === 0);
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
    cacheKey: 'storage-backups',
  });
}

// Backwards-compat for older call sites.
export function useStorageBackupsResources() {
  return useStorageRecoveryResources();
}

export default useUnifiedResources;
