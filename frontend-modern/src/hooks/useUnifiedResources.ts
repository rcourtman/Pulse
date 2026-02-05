import { createEffect, createResource, onCleanup } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type { Resource, PlatformType, SourceType, ResourceStatus, ResourceType } from '@/types/resource';

const UNIFIED_RESOURCES_URL = '/api/v2/resources?type=host';

type V2MetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
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
  };
  parentId?: string;
  tags?: string[];
  proxmox?: { nodeName?: string; clusterName?: string; uptime?: number };
  agent?: { hostname?: string; uptimeSeconds?: number; temperature?: number };
  docker?: { hostname?: string };
  pbs?: Record<string, unknown>;
  kubernetes?: Record<string, unknown>;
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
    case 'vm':
      return 'vm';
    case 'lxc':
      return 'container';
    case 'container':
      return 'container';
    case 'storage':
      return 'storage';
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
    uptime: v2.agent?.uptimeSeconds ?? v2.proxmox?.uptime,
    temperature: v2.agent?.temperature,
    tags: v2.tags,
    lastSeen: Number.isFinite(lastSeen) ? lastSeen : Date.now(),
    identity: {
      hostname: v2.identity?.hostnames?.[0],
      machineId: v2.identity?.machineId,
      ips: v2.identity?.ipAddresses,
    },
    platformData: {
      sources: v2.sources,
      sourceStatus: v2.sourceStatus,
      proxmox: v2.proxmox,
      agent: v2.agent,
      docker: v2.docker,
      pbs: v2.pbs,
      kubernetes: v2.kubernetes,
      metrics: v2.metrics,
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
  const [resources, { refetch, mutate }] = createResource<Resource[]>(fetchUnifiedResources, {
    initialValue: [],
  });
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;

  const scheduleRefetch = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }
    refreshHandle = setTimeout(() => {
      refreshHandle = undefined;
      if (!resources.loading) {
        void refetch();
      }
    }, 800);
  };

  createEffect(() => {
    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      return;
    }
    // Track resource-adjacent updates from the WebSocket store.
    // Accessing these arrays makes this effect react to updates.
    void wsStore.state.resources;
    void wsStore.state.nodes;
    void wsStore.state.hosts;
    void wsStore.state.dockerHosts;
    void wsStore.state.kubernetesClusters;
    void wsStore.state.pbs;
    void wsStore.state.pmg;

    scheduleRefetch();
  });

  onCleanup(() => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }
  });

  return {
    resources,
    refetch,
    mutate,
    loading: () => resources.loading,
    error: () => resources.error,
  };
}

export default useUnifiedResources;
