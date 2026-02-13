import { createMemo, createResource, onCleanup, createEffect, type Accessor } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import { logger } from '@/utils/logger';

const WORKLOADS_URL = '/api/resources?type=vm,lxc,docker_container,pod';
const WORKLOADS_PAGE_LIMIT = 200;
const WORKLOADS_MAX_PAGES = 20;
const WORKLOADS_CACHE_MAX_AGE_MS = 15_000;

type APIMetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
};

type APINetworkInterface = {
  name?: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
};

type APIDiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  total?: number;
  used?: number;
  free?: number;
};

type APIResource = {
  id: string;
  type?: string;
  name?: string;
  status?: string;
  lastSeen?: string;
  sources?: string[];
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
  parentName?: string;
  tags?: string[];
  vmid?: number;
  node?: string;
  instance?: string;
  proxmox?: {
    nodeName?: string;
    clusterName?: string;
    instance?: string;
    vmid?: number;
    cpus?: number;
    uptime?: number;
    template?: boolean;
    lastBackup?: string;
  };
  agent?: {
    hostname?: string;
    uptimeSeconds?: number;
    osName?: string;
    osVersion?: string;
    agentVersion?: string;
    networkInterfaces?: APINetworkInterface[];
    disks?: APIDiskInfo[];
  };
  docker?: {
    containerId?: string;
    hostname?: string;
    image?: string;
    imageName?: string;
    imageRef?: string;
    uptimeSeconds?: number;
  };
  kubernetes?: {
    clusterId?: string;
    agentId?: string;
    clusterName?: string;
    namespace?: string;
    context?: string;
    nodeName?: string;
    podUid?: string;
    image?: string;
    uptimeSeconds?: number;
  };
};

type APIListResponse = {
  data?: APIResource[];
  resources?: APIResource[];
  meta?: {
    page?: number;
    limit?: number;
    total?: number;
    totalPages?: number;
  };
};

let cachedWorkloads: WorkloadGuest[] = [];
let cachedWorkloadsAt = 0;
let sharedFetchWorkloads: Promise<WorkloadGuest[]> | null = null;

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && Object.getPrototypeOf(value) === Object.prototype;

const isDeepEqual = (left: unknown, right: unknown): boolean => {
  if (Object.is(left, right)) {
    return true;
  }

  if (typeof left !== typeof right) {
    return false;
  }

  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) {
      return false;
    }
    for (let i = 0; i < left.length; i += 1) {
      if (!isDeepEqual(left[i], right[i])) {
        return false;
      }
    }
    return true;
  }

  if (!isPlainObject(left) || !isPlainObject(right)) {
    return false;
  }

  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  if (leftKeys.length !== rightKeys.length) {
    return false;
  }

  for (const key of leftKeys) {
    if (!Object.prototype.hasOwnProperty.call(right, key) || !isDeepEqual(left[key], right[key])) {
      return false;
    }
  }

  return true;
};

const areWorkloadsEqual = (current: WorkloadGuest[], next: WorkloadGuest[]): boolean => {
  if (current === next) {
    return true;
  }
  if (current.length !== next.length) {
    return false;
  }
  for (let i = 0; i < current.length; i += 1) {
    const currentWorkload = current[i];
    const nextWorkload = next[i];
    if (currentWorkload.id !== nextWorkload.id) {
      return false;
    }
    if (!isDeepEqual(currentWorkload, nextWorkload)) {
      return false;
    }
  }
  return true;
};

const normalizeWorkloadStatus = (status?: string | null): string => {
  const normalized = (status || '').trim().toLowerCase();
  if (!normalized) return 'unknown';
  if (normalized === 'online' || normalized === 'healthy') return 'running';
  if (normalized === 'offline') return 'stopped';
  return normalized;
};

const resolveWorkloadType = (value?: string | null): WorkloadType | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'vm' || normalized === 'qemu') return 'vm';
  if (normalized === 'lxc') return 'lxc';
  if (normalized === 'container' || normalized === 'docker-container' || normalized === 'docker_container') {
    return 'docker';
  }
  if (normalized === 'pod' || normalized === 'k8s' || normalized === 'kubernetes') return 'k8s';
  return null;
};

const resolvePlatformType = (sources?: string[]): string | undefined => {
  const set = new Set((sources || []).map((source) => source.toLowerCase()));
  if (set.has('proxmox')) return 'proxmox-pve';
  if (set.has('pbs')) return 'proxmox-pbs';
  if (set.has('pmg')) return 'proxmox-pmg';
  if (set.has('docker')) return 'docker';
  if (set.has('kubernetes')) return 'kubernetes';
  if (set.has('agent')) return 'host-agent';
  return undefined;
};

const buildMetric = (metric?: APIMetricValue) => {
  const total = metric?.total ?? 0;
  const used = metric?.used ?? 0;
  const free = metric?.total !== undefined && metric?.used !== undefined
    ? Math.max(0, total - used)
    : 0;
  const usage = metric?.percent ?? metric?.value ?? (total > 0 ? (used / total) * 100 : 0);
  return { total, used, free, usage };
};

const mapNetworkInterfaces = (interfaces?: APINetworkInterface[]) =>
  interfaces?.map((iface) => ({
    name: iface.name,
    mac: iface.mac,
    addresses: iface.addresses,
    rxBytes: iface.rxBytes,
    txBytes: iface.txBytes,
  }));

const mapDisks = (disks?: APIDiskInfo[]) =>
  disks?.map((disk) => {
    const total = disk.total ?? 0;
    const used = disk.used ?? 0;
    const free = disk.free ?? (total > 0 ? Math.max(0, total - used) : 0);
    const usage = total > 0 ? (used / total) * 100 : 0;
    return {
      total,
      used,
      free,
      usage,
      mountpoint: disk.mountpoint,
      type: disk.filesystem,
      device: disk.device,
    };
  });

const toIsoString = (value?: string): string => {
  if (!value) return new Date().toISOString();
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) return new Date().toISOString();
  return new Date(parsed).toISOString();
};

const resolveWorkloadsPayload = (payload: unknown): { data: APIResource[]; totalPages: number } => {
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

const buildWorkloadsUrl = (page: number) =>
  `${WORKLOADS_URL}&page=${page}&limit=${WORKLOADS_PAGE_LIMIT}`;

const mapResourceToWorkload = (resource: APIResource): WorkloadGuest | null => {
  const workloadType = resolveWorkloadType(resource.type);
  if (!workloadType) return null;

  const name = (resource.name || resource.id || '').toString().trim();
  const node = resource.node ?? resource.proxmox?.nodeName ?? resource.kubernetes?.nodeName ?? '';
  const instance =
    resource.instance ??
    resource.proxmox?.instance ??
    resource.kubernetes?.clusterId ??
    resource.kubernetes?.clusterName ??
    resource.kubernetes?.context ??
    resource.proxmox?.clusterName ??
    resource.identity?.clusterName ??
    '';
  const vmid =
    typeof resource.vmid === 'number' ? resource.vmid
    : typeof resource.proxmox?.vmid === 'number' ? resource.proxmox.vmid
    : 0;
  const rawDisplayId = resource.id;
  const displayId =
    workloadType === 'vm' || workloadType === 'lxc'
      ? vmid > 0
        ? String(vmid)
        : undefined
      : rawDisplayId
        ? rawDisplayId.length > 12
          ? rawDisplayId.slice(0, 12)
          : rawDisplayId
        : undefined;

  const cpuPercent = resource.metrics?.cpu?.percent ?? resource.metrics?.cpu?.value ?? 0;

  // For PVE guests, use the legacy ID format (instance:node:vmid) so metrics keys
  // match what the backend charts API returns. Without this, sparklines
  // show no data because the hashed unified ID doesn't match any backend keys.
  const guestId = (() => {
    if ((workloadType === 'vm' || workloadType === 'lxc') && instance && node && vmid > 0) {
      return `${instance}:${node}:${vmid}`;
    }
    if (workloadType === 'docker') {
      return resource.docker?.containerId || resource.id;
    }
    if (workloadType === 'k8s') {
      const clusterId = resource.kubernetes?.clusterId;
      const podUid = resource.kubernetes?.podUid;
      if (clusterId && podUid) {
        return `k8s:${clusterId}:pod:${podUid}`;
      }
    }
    return resource.id;
  })();

  return {
    id: guestId,
    vmid: Number.isFinite(vmid) ? vmid : 0,
    name: name || resource.id,
    node,
    instance,
    status: normalizeWorkloadStatus(resource.status),
    type:
      workloadType === 'vm'
        ? 'vm'
        : workloadType === 'lxc'
          ? 'lxc'
          : workloadType === 'k8s'
            ? 'k8s'
            : 'docker',
    cpu: cpuPercent / 100,
    cpus: resource.proxmox?.cpus ?? 1,
    memory: buildMetric(resource.metrics?.memory),
    disk: buildMetric(resource.metrics?.disk),
    disks: mapDisks(resource.agent?.disks),
    diskStatusReason: undefined,
    ipAddresses: resource.identity?.ipAddresses ?? [],
    osName: resource.agent?.osName,
    osVersion: resource.agent?.osVersion,
    agentVersion: resource.agent?.agentVersion,
    networkInterfaces: mapNetworkInterfaces(resource.agent?.networkInterfaces),
    networkIn: resource.metrics?.netIn?.value ?? 0,
    networkOut: resource.metrics?.netOut?.value ?? 0,
    diskRead: resource.metrics?.diskRead?.value ?? 0,
    diskWrite: resource.metrics?.diskWrite?.value ?? 0,
    uptime: resource.proxmox?.uptime ?? resource.agent?.uptimeSeconds ?? resource.docker?.uptimeSeconds ?? resource.kubernetes?.uptimeSeconds ?? 0,
    template: resource.proxmox?.template ?? false,
    lastBackup: (() => {
      if (!resource.proxmox?.lastBackup) return 0;
      const parsed = Date.parse(resource.proxmox.lastBackup);
      // Go zero time "0001-01-01T00:00:00Z" parses to a large negative number
      return parsed > 0 ? parsed : 0;
    })(),
    tags: resource.tags ?? [],
    lock: '',
    lastSeen: toIsoString(resource.lastSeen),
    isOci: false,
    osTemplate: undefined,
    workloadType,
    displayId,
    image:
      workloadType === 'docker'
        ? resource.docker?.image || resource.docker?.imageName || resource.docker?.imageRef
        : workloadType === 'k8s'
          ? resource.kubernetes?.image
        : undefined,
    namespace:
      workloadType === 'k8s'
        ? resource.kubernetes?.namespace
        : undefined,
    contextLabel:
      workloadType === 'vm' || workloadType === 'lxc'
        ? node
          ? instance && instance !== node
            ? `${node} (${instance})`
            : node
          : undefined
        : workloadType === 'docker'
        ? resource.parentName || resource.docker?.hostname
        : workloadType === 'k8s'
          ? resource.kubernetes?.clusterName || resource.kubernetes?.context
          : undefined,
    platformType: resolvePlatformType(resource.sources),
  };
};

async function fetchWorkloads(): Promise<WorkloadGuest[]> {
  const firstResponse = await apiFetchJSON<unknown>(buildWorkloadsUrl(1), { cache: 'no-store' });
  const firstPage = resolveWorkloadsPayload(firstResponse);
  const allResources: APIResource[] = [...firstPage.data];

  const totalPages = Math.min(firstPage.totalPages, WORKLOADS_MAX_PAGES);
  if (totalPages > 1) {
    const pageRequests: Promise<unknown>[] = [];
    for (let page = 2; page <= totalPages; page++) {
      pageRequests.push(apiFetchJSON<unknown>(buildWorkloadsUrl(page), { cache: 'no-store' }));
    }
    const settled = await Promise.allSettled(pageRequests);
    for (const result of settled) {
      if (result.status !== 'fulfilled') continue;
      const pageData = resolveWorkloadsPayload(result.value);
      allResources.push(...pageData.data);
    }
  }

  const dedupedResources = Array.from(
    new Map(allResources.map((resource) => [resource.id, resource])).values(),
  );

  return dedupedResources
    .map((resource) => mapResourceToWorkload(resource))
    .filter((resource): resource is WorkloadGuest => !!resource);
}

const DEFAULT_POLL_INTERVAL_MS = 5_000;

const hasFreshWorkloadsCache = () =>
  cachedWorkloads.length > 0 && Date.now() - cachedWorkloadsAt <= WORKLOADS_CACHE_MAX_AGE_MS;

const setWorkloadsCache = (workloads: WorkloadGuest[], at = Date.now()) => {
  cachedWorkloads = workloads;
  cachedWorkloadsAt = at;
};

const fetchWorkloadsShared = async (force = false): Promise<WorkloadGuest[]> => {
  if (!force && hasFreshWorkloadsCache()) {
    return cachedWorkloads;
  }

  if (sharedFetchWorkloads) {
    return sharedFetchWorkloads;
  }

  const request = (async () => {
    const previous = cachedWorkloads;
    const fetched = await fetchWorkloads();
    if (areWorkloadsEqual(previous, fetched)) {
      cachedWorkloadsAt = Date.now();
      return previous;
    }
    setWorkloadsCache(fetched);
    return fetched;
  })();

  sharedFetchWorkloads = request;

  try {
    return await request;
  } finally {
    if (sharedFetchWorkloads === request) {
      sharedFetchWorkloads = null;
    }
  }
};

export const __resetWorkloadsCacheForTests = () => {
  cachedWorkloads = [];
  cachedWorkloadsAt = 0;
  sharedFetchWorkloads = null;
};

export function useWorkloads(enabled: Accessor<boolean> = () => true) {
  const source = createMemo(() => (enabled() ? 'workloads' : null));
  const [workloads, { mutate: resourceMutate }] = createResource(
    source,
    () => fetchWorkloadsShared(),
    {
      initialValue: cachedWorkloads,
    },
  );

  const mutate = (value: WorkloadGuest[] | ((prev: WorkloadGuest[]) => WorkloadGuest[])) =>
    resourceMutate((previous) => {
      const current = previous ?? [];
      const next = typeof value === 'function' ? value(current) : value;
      const normalized = next ?? [];
      if (areWorkloadsEqual(current, normalized)) {
        setWorkloadsCache(current);
        return current;
      }
      setWorkloadsCache(normalized);
      return normalized;
    });

  const applyWorkloads = (next: WorkloadGuest[]) => {
    resourceMutate((previous) => {
      const current = previous ?? [];
      if (areWorkloadsEqual(current, next)) {
        setWorkloadsCache(current);
        return current;
      }
      setWorkloadsCache(next);
      return next;
    });
  };

  const refetch = async () => {
    const data = await fetchWorkloadsShared(true);
    applyWorkloads(data);
    return data;
  };

  if (!hasFreshWorkloadsCache()) {
    void fetchWorkloadsShared()
      .then(applyWorkloads)
      .catch((err) => {
        logger.warn('[useWorkloads] Failed to refresh stale workloads cache', err);
      });
  }

  // Poll for fresh metrics while enabled.
  // Use mutate() instead of refetch() so the resource never enters a loading
  // state during polls. refetch() sets loading=true which triggers the app-level
  // <Suspense> boundary, briefly unmounting the entire page every poll cycle.
  createEffect(() => {
    if (!enabled()) return;
    const id = setInterval(async () => {
      try {
        const data = await fetchWorkloadsShared(true);
        applyWorkloads(data);
      } catch (err) {
        // Keep showing the last successful snapshot while polling recovers.
        logger.debug('[useWorkloads] Poll refresh failed; retaining cached data', err);
      }
    }, DEFAULT_POLL_INTERVAL_MS);
    onCleanup(() => clearInterval(id));
  });

  return {
    workloads,
    refetch,
    mutate,
    loading: () => workloads.loading,
    error: () => workloads.error,
  };
}

export default useWorkloads;
