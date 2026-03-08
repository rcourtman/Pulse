import type {
  Disk,
  Agent,
  HostNetworkInterface,
  HostRAIDArray,
  HostSensorSummary,
  Memory,
  Node,
} from '@/types/api';
import type { Resource, ResourceMetric } from '@/types/resource';
import { formatTemperature } from '@/utils/temperature';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import {
  canonicalDiscoveryResourceType,
  isAgentDiscoveryResourceType,
} from '@/utils/discoveryTarget';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
} from '@/utils/agentResources';
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';

export type ProxmoxPlatformData = {
  nodeName?: string;
  clusterName?: string;
  vmid?: number;
  pveVersion?: string;
  kernelVersion?: string;
  uptime?: number;
  cpuInfo?: { model?: string; cores?: number; sockets?: number };
};

export type AgentDiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
};

export type AgentPlatformData = {
  agentId?: string;
  agentVersion?: string;
  hostname?: string;
  platform?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  uptimeSeconds?: number;
  networkInterfaces?: HostNetworkInterface[];
  disks?: AgentDiskInfo[];
  sensors?: HostSensorSummary;
  raid?: HostRAIDArray[];
  cpuCount?: number;
  memory?: Partial<Memory>;
};

export type PBSPlatformData = {
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

export type PMGPlatformData = {
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

export type KubernetesMetricCapabilities = {
  nodeCpuMemory?: boolean;
  nodeTelemetry?: boolean;
  podCpuMemory?: boolean;
  podNetwork?: boolean;
  podEphemeralDisk?: boolean;
  podDiskIo?: boolean;
};

export type KubernetesPlatformData = {
  clusterId?: string;
  agentId?: string;
  clusterName?: string;
  context?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
  metricCapabilities?: KubernetesMetricCapabilities;
};

export type PlatformData = {
  sources?: string[];
  proxmox?: ProxmoxPlatformData;
  agent?: AgentPlatformData;
  sourceStatus?: Record<string, { status?: string; lastSeen?: string | number; error?: string }>;
  docker?: Record<string, unknown>;
  pbs?: PBSPlatformData;
  pmg?: PMGPlatformData;
  kubernetes?: KubernetesPlatformData;
  metrics?: Record<string, unknown>;
  identityMatch?: unknown;
  matchResults?: unknown;
  matchCandidates?: unknown;
  matches?: unknown;
};

export type DockerRuntimeCommand = {
  id?: string;
  type?: string;
  status?: string;
  message?: string;
  failureReason?: string;
  createdAt?: string;
  updatedAt?: string;
  acknowledgedAt?: string;
  completedAt?: string;
  failedAt?: string;
};

export type DockerPlatformData = {
  hostSourceId?: string;
  containerId?: string;
  hostname?: string;
  runtime?: string;
  runtimeVersion?: string;
  dockerVersion?: string;
  os?: string;
  kernelVersion?: string;
  architecture?: string;
  agentVersion?: string;
  uptimeSeconds?: number;
  swarm?: {
    nodeId?: string;
    nodeRole?: string;
    localState?: string;
    controlAvailable?: boolean;
    clusterId?: string;
    clusterName?: string;
    scope?: string;
    error?: string;
  };
  containerCount?: number;
  updatesAvailableCount?: number;
  updatesLastCheckedAt?: string;
  command?: DockerRuntimeCommand;
};

export type DiscoveryConfig = {
  resourceType: DiscoveryResourceType;
  agentId: string;
  resourceId: string;
  hostname: string;
  metadataKind: 'guest' | 'agent';
  metadataId: string;
  targetLabel: string;
};

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const getPreferredHostLabel = (resource: Resource): string =>
  getPreferredResourceHostname(resource) ||
  getPreferredResourceDisplayName(resource) ||
  resource.id;

export const toDiscoveryConfig = (resource: Resource): DiscoveryConfig | null => {
  const explicitDiscoveryTarget = resource.discoveryTarget;
  const explicitDiscoveryAgentId = asString(
    (explicitDiscoveryTarget as { agentId?: unknown } | undefined)?.agentId,
  );

  if (
    explicitDiscoveryTarget &&
    explicitDiscoveryTarget.resourceType &&
    explicitDiscoveryAgentId &&
    explicitDiscoveryTarget.resourceId
  ) {
    const explicitResourceType = canonicalDiscoveryResourceType(
      explicitDiscoveryTarget.resourceType,
    );
    const resourceType = (() => {
      switch (explicitResourceType) {
        case 'agent':
          return 'agent';
        case 'vm':
        case 'system-container':
        case 'app-container':
        case 'pod':
          return explicitResourceType;
        default:
          return null;
      }
    })();

    if (resourceType) {
      const hostname = explicitDiscoveryTarget.hostname || getPreferredHostLabel(resource);
      const isHostDiscovery = isAgentDiscoveryResourceType(resourceType);
      const targetLabel = isHostDiscovery
        ? 'agent'
        : resourceType === 'app-container'
          ? 'container'
          : resourceType === 'pod'
            ? 'workload'
            : 'guest';
      return {
        resourceType,
        agentId: explicitDiscoveryAgentId,
        resourceId: explicitDiscoveryTarget.resourceId,
        hostname,
        metadataKind: isHostDiscovery ? 'agent' : 'guest',
        metadataId: explicitDiscoveryTarget.resourceId,
        targetLabel,
      };
    }
  }

  const platformData = resource.platformData as PlatformData | undefined;
  const dockerPlatformData = platformData?.docker as DockerPlatformData | undefined;
  const kubernetesPlatformData = platformData?.kubernetes;
  const proxmoxVmid =
    asNumber(resource.proxmox?.vmid) ??
    asNumber(platformData?.proxmox?.vmid) ??
    asNumber((platformData as { vmid?: unknown } | undefined)?.vmid);
  const vmidResourceId =
    proxmoxVmid !== undefined && proxmoxVmid > 0 ? String(proxmoxVmid) : undefined;
  const proxmoxNodeName =
    asString(resource.proxmox?.nodeName) ||
    platformData?.proxmox?.nodeName ||
    asString((platformData as { nodeName?: unknown } | undefined)?.nodeName);
  const actionableAgentId = getActionableAgentIdFromResource(resource);
  const actionableDockerHostId = getActionableDockerRuntimeIdFromResource(resource);
  const actionableKubernetesId = getActionableKubernetesClusterIdFromResource(resource);
  const kubernetesAgentId =
    asString(resource.kubernetes?.agentId) ||
    asString(kubernetesPlatformData?.agentId) ||
    actionableKubernetesId ||
    asString(resource.kubernetes?.clusterId) ||
    asString(kubernetesPlatformData?.clusterId) ||
    asString(resource.clusterId) ||
    asString(resource.kubernetes?.context) ||
    asString(kubernetesPlatformData?.context) ||
    asString(resource.kubernetes?.clusterName) ||
    asString(kubernetesPlatformData?.clusterName);
  const kubernetesResourceId =
    asString(resource.kubernetes?.podUid) ||
    asString(kubernetesPlatformData?.podUid) ||
    (() => {
      const namespace =
        asString(resource.kubernetes?.namespace) || asString(kubernetesPlatformData?.namespace);
      const podName =
        asString(resource.kubernetes?.podName) ||
        asString(kubernetesPlatformData?.podName) ||
        asString(resource.name);
      return namespace && podName ? `${namespace}/${podName}` : undefined;
    })();
  const agentLookupId =
    actionableDockerHostId ||
    actionableKubernetesId ||
    actionableAgentId ||
    proxmoxNodeName ||
    platformData?.agent?.hostname ||
    asString(dockerPlatformData?.hostname) ||
    getPreferredResourceHostname(resource) ||
    getPreferredResourceDisplayName(resource) ||
    resource.platformId ||
    resource.id;
  const agentLikeId = agentLookupId;
  const workloadAgentId =
    proxmoxNodeName ||
    actionableDockerHostId ||
    kubernetesAgentId ||
    actionableAgentId ||
    asString(resource.parentName) ||
    resource.parentId ||
    getPreferredResourceHostname(resource) ||
    resource.platformId ||
    resource.id;
  const hostname = getPreferredHostLabel(resource);

  switch (resource.type) {
    case 'agent':
    case 'docker-host':
    case 'pbs':
    case 'pmg':
    case 'k8s-cluster':
    case 'k8s-node':
    case 'truenas':
      return {
        resourceType: 'agent',
        agentId: agentLikeId,
        resourceId: agentLikeId,
        hostname,
        metadataKind: 'agent',
        metadataId: agentLikeId,
        targetLabel: 'agent',
      };
    case 'vm':
      return {
        resourceType: 'vm',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'system-container':
    case 'oci-container':
      return {
        resourceType: 'system-container',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'app-container':
      return {
        resourceType: 'app-container',
        agentId: workloadAgentId,
        resourceId: asString(dockerPlatformData?.containerId) || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'container',
      };
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      return {
        resourceType: 'pod',
        agentId: workloadAgentId,
        resourceId: kubernetesResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'workload',
      };
    default:
      return null;
  }
};

export const buildMemory = (metric?: ResourceMetric, fallback?: Partial<Memory>): Memory => {
  const total = metric?.total ?? fallback?.total ?? 0;
  const used = metric?.used ?? fallback?.used ?? 0;
  const free = metric?.free ?? fallback?.free ?? Math.max(total - used, 0);
  const usage = total > 0 ? used / total : (fallback?.usage ?? 0);
  return {
    total,
    used,
    free,
    usage,
  };
};

export const buildDisk = (metric?: ResourceMetric, fallback?: Partial<Disk>): Disk => {
  const total = metric?.total ?? fallback?.total ?? 0;
  const used = metric?.used ?? fallback?.used ?? 0;
  const free = metric?.free ?? fallback?.free ?? Math.max(total - used, 0);
  const usage = total > 0 ? used / total : (fallback?.usage ?? 0);
  return {
    total,
    used,
    free,
    usage,
    mountpoint: fallback?.mountpoint,
    type: fallback?.type,
    device: fallback?.device,
  };
};

export const toAgentDisks = (disks?: AgentDiskInfo[]): Disk[] | undefined => {
  if (!disks || disks.length === 0) return undefined;
  return disks.map((disk) => {
    const total = disk.total ?? 0;
    const used = disk.used ?? 0;
    const free = disk.free ?? Math.max(total - used, 0);
    const usage = total > 0 ? used / total : 0;
    return {
      total,
      used,
      free,
      usage,
      mountpoint: disk.mountpoint ?? disk.device,
      type: disk.filesystem ?? disk.type,
      device: disk.device,
    };
  });
};

export const toNodeFromProxmox = (resource: Resource): Node | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const proxmox = platformData?.proxmox;
  if (!proxmox) return null;

  const memory = buildMemory(resource.memory);
  const disk = buildDisk(resource.disk);
  const lastSeen = Number.isFinite(resource.lastSeen)
    ? new Date(resource.lastSeen).toISOString()
    : new Date().toISOString();
  const linkedAgentId =
    asString((platformData as { linkedAgentId?: unknown } | undefined)?.linkedAgentId) ||
    getActionableAgentIdFromResource(resource);

  return {
    id: resource.id,
    name: proxmox.nodeName ?? getPreferredHostLabel(resource),
    displayName: getPreferredResourceDisplayName(resource),
    instance: resource.platformId ?? resource.id,
    host: proxmox.nodeName ?? getPreferredHostLabel(resource),
    status: resource.status,
    type: resource.type,
    cpu: resource.cpu?.current ?? 0,
    memory,
    disk,
    uptime: resource.uptime ?? proxmox.uptime ?? 0,
    loadAverage: [],
    kernelVersion: proxmox.kernelVersion ?? 'Unknown',
    pveVersion: proxmox.pveVersion ?? 'Unknown',
    cpuInfo: {
      model: proxmox.cpuInfo?.model ?? 'Unknown',
      cores: proxmox.cpuInfo?.cores ?? 0,
      sockets: proxmox.cpuInfo?.sockets ?? 0,
      mhz: '0',
    },
    lastSeen,
    connectionHealth: resource.status ?? 'unknown',
    linkedAgentId,
  } as Node;
};

export const toAgentFromResource = (
  resource: Resource,
  explicitAgent?: AgentPlatformData,
): Agent | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const agent = explicitAgent ?? platformData?.agent;
  if (!agent) return null;

  const proxmoxCores = platformData?.proxmox?.cpuInfo?.cores;
  const cpuCount = [
    agent.cpuCount,
    (agent as { cpuCores?: number }).cpuCores,
    (agent as { cores?: number }).cores,
    proxmoxCores,
  ].find((value) => typeof value === 'number' && value > 0);

  const hostname = agent.hostname ?? getPreferredHostLabel(resource);
  const agentId = getActionableAgentIdFromResource(resource) || resource.id;

  return {
    id: agentId,
    hostname,
    displayName: getPreferredResourceDisplayName(resource),
    platform: agent.platform,
    osName: agent.osName ?? 'Unknown',
    osVersion: agent.osVersion ?? '',
    kernelVersion: agent.kernelVersion ?? 'Unknown',
    architecture: agent.architecture ?? 'Unknown',
    cpuCount,
    memory: buildMemory(resource.memory, agent.memory),
    disks: toAgentDisks(agent.disks),
    networkInterfaces: agent.networkInterfaces,
    sensors: agent.sensors,
    raid: agent.raid,
    status: resource.status,
    uptimeSeconds: agent.uptimeSeconds ?? resource.uptime ?? 0,
    lastSeen: resource.lastSeen,
    agentVersion: agent.agentVersion,
    tags: resource.tags,
  } as Agent;
};

export const formatSensorName = (name: string) => {
  let clean = name.replace(/^[a-z]+\d*_/i, '');
  clean = clean.replace(/_/g, ' ');
  return clean.replace(/\b\w/g, (char) => char.toUpperCase());
};

export const buildTemperatureRows = (sensors?: HostSensorSummary) => {
  const rows: { label: string; value: string; valueTitle?: string }[] = [];
  const temps = sensors?.temperatureCelsius;
  if (temps) {
    const entries = Object.entries(temps).sort(([a], [b]) => a.localeCompare(b));
    entries.forEach(([name, temp]) => {
      rows.push({
        label: formatSensorName(name),
        value: formatTemperature(temp),
        valueTitle: `${temp.toFixed(1)}°C`,
      });
    });
  }

  const smart = sensors?.smart;
  if (smart) {
    smart
      .filter((disk) => !disk.standby && Number.isFinite(disk.temperature))
      .sort((a, b) => a.device.localeCompare(b.device))
      .forEach((disk) => {
        rows.push({
          label: `Disk ${disk.device}`,
          value: formatTemperature(disk.temperature),
          valueTitle: `${disk.temperature.toFixed(1)}°C`,
        });
      });
  }

  return rows;
};

export const normalizeHealthLabel = (value?: string): string => {
  const raw = (value || '').trim();
  if (!raw) return 'Unknown';
  if (raw.length <= 3) return raw.toUpperCase();
  return raw.charAt(0).toUpperCase() + raw.slice(1);
};

export const healthToneClass = (value?: string): string => {
  const normalized = (value || '').trim().toLowerCase();
  if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) {
    return 'text-emerald-600 dark:text-emerald-400';
  }
  if (['degraded', 'warning', 'stale'].includes(normalized)) {
    return 'text-amber-600 dark:text-amber-400';
  }
  if (['offline', 'down', 'disconnected', 'error', 'failed'].includes(normalized)) {
    return 'text-red-600 dark:text-red-400';
  }
  return 'text-base-content';
};

export const formatInteger = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return new Intl.NumberFormat().format(Math.round(value));
};

export const ALIAS_COLLAPSE_THRESHOLD = 4;

export const formatSourceType = (value: Resource['sourceType']): string => {
  switch (value) {
    case 'hybrid':
      return 'Hybrid';
    case 'agent':
      return 'Agent';
    case 'api':
      return 'API';
    default:
      return value;
  }
};
