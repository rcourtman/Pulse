import { batch, createEffect, createSignal, onCleanup } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { apiFetch } from '@/utils/apiClient';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type { Resource, PlatformType, SourceType, ResourceStatus, ResourceType } from '@/types/resource';

const UNIFIED_RESOURCES_URL = '/api/v2/resources?type=host,pbs,pmg,k8s_cluster,k8s_node';

type V2MetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
};

type V2KubernetesData = {
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

type V2Resource = {
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
    cpu?: V2MetricValue;
    memory?: V2MetricValue;
    disk?: V2MetricValue;
    netIn?: V2MetricValue;
    netOut?: V2MetricValue;
    diskRead?: V2MetricValue;
    diskWrite?: V2MetricValue;
  };
  parentId?: string;
  tags?: string[];
  proxmox?: { nodeName?: string; clusterName?: string; uptime?: number; temperature?: number };
  agent?: { agentId?: string; hostname?: string; uptimeSeconds?: number; temperature?: number };
  docker?: { hostname?: string; temperature?: number };
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
  kubernetes?: V2KubernetesData;
  discoveryTarget?: {
    resourceType?: string;
    hostId?: string;
    resourceId?: string;
    hostname?: string;
  };
};

type V2ListResponse = {
  data?: V2Resource[];
  resources?: V2Resource[];
};

const resolvePlatformType = (sources: string[] | undefined): PlatformType => {
  const set = new Set((sources || []).map((s) => s.toLowerCase()));
  if (set.has('proxmox')) return 'proxmox-pve';
  if (set.has('pbs')) return 'proxmox-pbs';
  if (set.has('pmg')) return 'proxmox-pmg';
  if (set.has('docker')) return 'docker';
  if (set.has('kubernetes')) return 'kubernetes';
  if (set.has('agent')) return 'host-agent';
  return 'host-agent';
};

const resolveSourceType = (sources: string[] | undefined): SourceType => {
  const set = new Set((sources || []).map((s) => s.toLowerCase()));
  const hasAgent = set.has('agent');
  const hasOther =
    set.has('proxmox') || set.has('docker') || set.has('kubernetes') || set.has('pbs') || set.has('pmg');
  if (hasAgent && hasOther) return 'hybrid';
  if (hasAgent) return 'agent';
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
    case 'k8s-node':
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
      return 'storage';
    default:
      return 'host';
  }
};

const metricToResourceMetric = (metric?: V2MetricValue) => {
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

const toResource = (v2: V2Resource): Resource => {
  const sources = v2.sources || [];
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
        resourceType: v2.discoveryTarget.resourceType as 'host' | 'vm' | 'lxc' | 'docker' | 'k8s',
        hostId: v2.discoveryTarget.hostId,
        resourceId: v2.discoveryTarget.resourceId,
        hostname: v2.discoveryTarget.hostname,
      }
      : undefined;

  return {
    id: v2.id,
    type: resolveType(v2.type),
    name,
    displayName: name,
    platformId,
    platformType: resolvePlatformType(sources),
    sourceType: resolveSourceType(sources),
    parentId: v2.parentId,
    clusterId: v2.identity?.clusterName || v2.proxmox?.clusterName,
    status: resolveStatus(v2.status),
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
      v2.kubernetes?.temperature,
    tags: v2.tags,
    lastSeen: Number.isFinite(lastSeen) ? lastSeen : Date.now(),
    identity: {
      hostname: v2.identity?.hostnames?.[0],
      machineId: v2.identity?.machineId,
      ips: v2.identity?.ipAddresses,
    },
    discoveryTarget,
    platformData: {
      sources: v2.sources,
      sourceStatus: v2.sourceStatus,
      proxmox: v2.proxmox,
      agent: v2.agent,
      docker: v2.docker,
      pbs: v2.pbs,
      pmg: v2.pmg,
      kubernetes: v2.kubernetes,
      metrics: v2.metrics,
      discoveryTarget: v2.discoveryTarget,
    },
  };
};

async function fetchUnifiedResources(): Promise<Resource[]> {
  const response = await apiFetch(UNIFIED_RESOURCES_URL, { cache: 'no-store' });
  if (!response.ok) {
    throw new Error('Failed to fetch unified resources');
  }

  const data = (await response.json()) as V2ListResponse | V2Resource[];

  const rawResources = Array.isArray(data)
    ? data
    : data.data && Array.isArray(data.data)
      ? data.data
      : data.resources && Array.isArray(data.resources)
        ? data.resources
        : [];

  return rawResources.map((resource) => toResource(resource as V2Resource));
}

export function useUnifiedResources() {
  const [resources, setResources] = createStore<Resource[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<unknown>(undefined);
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;
  let inFlightRefetch: Promise<Resource[]> | null = null;

  const applyResources = (next: Resource[]) => {
    setResources(reconcile(next, { key: 'id' }));
  };

  const refetch = async () => {
    if (inFlightRefetch) {
      return inFlightRefetch;
    }

    const request = (async () => {
      setLoading(true);
      try {
        const fetched = await fetchUnifiedResources();
        batch(() => {
          applyResources(fetched);
          setError(undefined);
        });
        return fetched;
      } catch (err) {
        setError(err);
        throw err;
      } finally {
        setLoading(false);
        inFlightRefetch = null;
      }
    })();

    inFlightRefetch = request;
    return request;
  };

  const mutate = (value: Resource[] | ((prev: Resource[]) => Resource[])) => {
    const current = resources as unknown as Resource[];
    const next = typeof value === 'function' ? value(current) : value;
    applyResources(next ?? []);
    return resources as unknown as Resource[];
  };

  void refetch().catch(() => undefined);

  const scheduleRefetch = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }
    refreshHandle = setTimeout(() => {
      refreshHandle = undefined;
      if (!loading()) {
        void refetch().catch(() => undefined);
      }
    }, 800);
  };

  createEffect(() => {
    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      return;
    }

    // Reconcile() often preserves top-level array identity, so subscribing to
    // wsStore.state.resources (or other arrays) can miss metric-only updates.
    // lastUpdate is bumped for every usable payload and gives us a stable
    // refetch trigger for fresh info/drawer metrics.
    void wsStore.state.lastUpdate;

    scheduleRefetch();
  });

  onCleanup(() => {
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

export default useUnifiedResources;
