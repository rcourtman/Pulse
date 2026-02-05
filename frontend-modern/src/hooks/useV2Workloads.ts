import { createMemo, createResource, type Accessor } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { WorkloadGuest, WorkloadType } from '@/types/workloads';

const V2_WORKLOADS_URL = '/api/v2/resources?type=vm,lxc,docker_container';

type V2MetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
};

type V2NetworkInterface = {
  name?: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
};

type V2DiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  total?: number;
  used?: number;
  free?: number;
};

type V2Resource = {
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
    cpu?: V2MetricValue;
    memory?: V2MetricValue;
    disk?: V2MetricValue;
    netIn?: V2MetricValue;
    netOut?: V2MetricValue;
  };
  parentId?: string;
  tags?: string[];
  vmid?: number;
  node?: string;
  instance?: string;
  proxmox?: {
    nodeName?: string;
    clusterName?: string;
    uptime?: number;
  };
  agent?: {
    hostname?: string;
    uptimeSeconds?: number;
    osName?: string;
    osVersion?: string;
    agentVersion?: string;
    networkInterfaces?: V2NetworkInterface[];
    disks?: V2DiskInfo[];
  };
  docker?: {
    hostname?: string;
    image?: string;
    imageName?: string;
    imageRef?: string;
  };
  kubernetes?: {
    clusterName?: string;
    namespace?: string;
    context?: string;
  };
};

type V2ListResponse = {
  data?: V2Resource[];
  resources?: V2Resource[];
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

const buildMetric = (metric?: V2MetricValue) => {
  const total = metric?.total ?? 0;
  const used = metric?.used ?? 0;
  const free = metric?.total !== undefined && metric?.used !== undefined
    ? Math.max(0, total - used)
    : 0;
  const usage = metric?.percent ?? metric?.value ?? (total > 0 ? (used / total) * 100 : 0);
  return { total, used, free, usage };
};

const mapNetworkInterfaces = (interfaces?: V2NetworkInterface[]) =>
  interfaces?.map((iface) => ({
    name: iface.name,
    mac: iface.mac,
    addresses: iface.addresses,
    rxBytes: iface.rxBytes,
    txBytes: iface.txBytes,
  }));

const mapDisks = (disks?: V2DiskInfo[]) =>
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

const resolveWorkloadsPayload = (payload: unknown): V2Resource[] => {
  if (Array.isArray(payload)) return payload as V2Resource[];
  if (!payload || typeof payload !== 'object') return [];
  const record = payload as V2ListResponse;
  if (Array.isArray(record.data)) return record.data;
  if (Array.isArray(record.resources)) return record.resources;
  return [];
};

const mapResourceToWorkload = (resource: V2Resource): WorkloadGuest | null => {
  const workloadType = resolveWorkloadType(resource.type);
  if (!workloadType) return null;

  const name = (resource.name || resource.id || '').toString().trim();
  const node = resource.node ?? resource.proxmox?.nodeName ?? '';
  const instance =
    resource.instance ??
    resource.proxmox?.clusterName ??
    resource.identity?.clusterName ??
    '';
  const vmid = typeof resource.vmid === 'number' ? resource.vmid : 0;
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

  return {
    id: resource.id,
    vmid: Number.isFinite(vmid) ? vmid : 0,
    name: name || resource.id,
    node,
    instance,
    status: normalizeWorkloadStatus(resource.status),
    type: workloadType === 'vm' ? 'vm' : workloadType === 'lxc' ? 'lxc' : 'docker',
    cpu: cpuPercent / 100,
    cpus: 1,
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
    diskRead: 0,
    diskWrite: 0,
    uptime: resource.agent?.uptimeSeconds ?? resource.proxmox?.uptime ?? 0,
    template: false,
    lastBackup: 0,
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
        : undefined,
    namespace:
      workloadType === 'k8s'
        ? resource.kubernetes?.namespace
        : undefined,
    contextLabel:
      workloadType === 'docker'
        ? resource.docker?.hostname
        : workloadType === 'k8s'
          ? resource.kubernetes?.clusterName
          : undefined,
    platformType: resolvePlatformType(resource.sources),
  };
};

async function fetchV2Workloads(): Promise<WorkloadGuest[]> {
  const response = await apiFetchJSON<unknown>(V2_WORKLOADS_URL, { cache: 'no-store' });
  const resources = resolveWorkloadsPayload(response);
  return resources
    .map((resource) => mapResourceToWorkload(resource))
    .filter((resource): resource is WorkloadGuest => !!resource);
}

export function useV2Workloads(enabled: Accessor<boolean> = () => true) {
  const source = createMemo(() => (enabled() ? 'v2-workloads' : null));
  const [workloads, { refetch, mutate }] = createResource(source, fetchV2Workloads, {
    initialValue: [],
  });

  return {
    workloads,
    refetch,
    mutate,
    loading: () => workloads.loading,
    error: () => workloads.error,
  };
}

export default useV2Workloads;
