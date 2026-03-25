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
import { getActionableAgentIdFromResource } from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
export { getSourceTypeLabel as formatSourceType } from '@/utils/sourceTypePresentation';

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

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const getPreferredHostLabel = (resource: Resource): string =>
  getPreferredResourceHostname(resource) ||
  getPreferredInfrastructureDisplayName(resource) ||
  resource.id;

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
    displayName: getPreferredInfrastructureDisplayName(resource),
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
    displayName: getPreferredInfrastructureDisplayName(resource),
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
  return titleCaseDelimitedLabel(clean);
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

export const formatInteger = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return new Intl.NumberFormat().format(Math.round(value));
};

export const ALIAS_COLLAPSE_THRESHOLD = 4;
