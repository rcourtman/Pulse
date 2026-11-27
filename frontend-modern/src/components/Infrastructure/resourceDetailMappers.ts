import type {
  Disk,
  Agent,
  HostNetworkInterface,
  HostRAIDArray,
  HostSensorSummary,
  Memory,
  Node,
  PBSBackupJob,
  PBSGarbageJob,
  PBSJobHealthEvidence,
  PBSPruneJob,
  PBSSyncJob,
  PBSVerifyJob,
} from '@/types/api';
import type { Resource, ResourceMetric, ResourceVMwareMeta } from '@/types/resource';
import { formatTemperature } from '@/utils/temperature';
import { getPhysicalDiskTemperaturePresentation } from '@/features/storageBackups/diskTemperaturePresentation';
import { formatBytes } from '@/utils/format';
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
  memory?: Partial<Memory>;
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
  commandsEnabled?: boolean;
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
  jobHealthEvidenceCount?: number;
  backupJobs?: PBSBackupJob[];
  syncJobs?: PBSSyncJob[];
  verifyJobs?: PBSVerifyJob[];
  pruneJobs?: PBSPruneJob[];
  garbageJobs?: PBSGarbageJob[];
  jobHealthEvidence?: PBSJobHealthEvidence[];
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
  vmware?: ResourceVMwareMeta;
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
  collectionMode?: string;
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

const buildUsageParts = (
  total: number,
  used: number,
  free: number | undefined,
  fallbackUsage: number,
) => ({
  total,
  used,
  free: free ?? Math.max(total - used, 0),
  usage: total > 0 ? used / total : fallbackUsage,
});

export const buildMemory = (metric?: ResourceMetric, fallback?: Partial<Memory>): Memory => {
  const usageUnavailable = metric == null && fallback?.usageUnavailable === true;
  const total = metric?.total ?? fallback?.total ?? 0;
  const used = metric?.used ?? fallback?.used ?? 0;
  const parts = buildUsageParts(
    total,
    used,
    usageUnavailable ? (fallback?.free ?? 0) : (metric?.free ?? fallback?.free),
    fallback?.usage ?? 0,
  );
  return {
    ...parts,
    ...(usageUnavailable ? { usage: 0, usageUnavailable: true } : {}),
  };
};

export const buildDisk = (metric?: ResourceMetric, fallback?: Partial<Disk>): Disk => ({
  ...buildUsageParts(
    metric?.total ?? fallback?.total ?? 0,
    metric?.used ?? fallback?.used ?? 0,
    metric?.free ?? fallback?.free,
    fallback?.usage ?? 0,
  ),
  mountpoint: fallback?.mountpoint,
  type: fallback?.type,
  device: fallback?.device,
});

export const toAgentDisks = (disks?: AgentDiskInfo[]): Disk[] | undefined => {
  if (!disks?.length) return undefined;
  return disks.map((disk) => ({
    ...buildUsageParts(disk.total ?? 0, disk.used ?? 0, disk.free, 0),
    mountpoint: disk.mountpoint ?? disk.device,
    type: disk.filesystem ?? disk.type,
    device: disk.device,
  }));
};

export const toNodeFromProxmox = (resource: Resource): Node | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const proxmox = platformData?.proxmox;
  if (!proxmox) return null;

  const memory = buildMemory(resource.memory, proxmox.memory);
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
    // A silent agent's uptime is from its last report, not how long the
    // machine has been up.
    uptimeSeconds: resource.agent?.stale ? 0 : (agent.uptimeSeconds ?? resource.uptime ?? 0),
    lastSeen: resource.lastSeen,
    agentVersion: agent.agentVersion,
    commandsEnabled: agent.commandsEnabled,
    tags: resource.tags,
  } as Agent;
};

export const formatSensorName = (name: string) => {
  let clean = name.replace(/^[a-z]+\d*_/i, '');
  return titleCaseDelimitedLabel(clean);
};

const formatSensorHardwareName = (name: string) =>
  titleCaseDelimitedLabel(name).replace(/\b(Cpu|Dram|Gpu|Vram|Vrm)\b/g, (match) =>
    match.toUpperCase(),
  );

const formatPowerSensorLabel = (name: string) => {
  const label = formatSensorHardwareName(name);
  return /\b(power|watts?|wattage)\b/i.test(label) ? label : `${label} Power`;
};

const formatFanSensorLabel = (name: string) => {
  const label = formatSensorHardwareName(name);
  return /\bfan\b/i.test(label) ? label : `${label} Fan`;
};

const formatGPUStatsLabel = (id: string | undefined, index: number): string => {
  const trimmed = (id || '').trim();
  return trimmed ? `GPU ${trimmed}` : `GPU ${index + 1}`;
};

const formatGPUStatsValue = (gpu: NonNullable<HostSensorSummary['gpu']>[number]) => {
  const parts: string[] = [];
  if (gpu.name) parts.push(gpu.name);
  if (typeof gpu.temperatureCelsius === 'number' && Number.isFinite(gpu.temperatureCelsius)) {
    parts.push(formatTemperature(gpu.temperatureCelsius));
  }
  if (typeof gpu.utilizationPercent === 'number' && Number.isFinite(gpu.utilizationPercent)) {
    parts.push(`${Math.round(Math.max(0, gpu.utilizationPercent))}%`);
  }
  if (
    typeof gpu.memoryTotalBytes === 'number' &&
    Number.isFinite(gpu.memoryTotalBytes) &&
    gpu.memoryTotalBytes > 0
  ) {
    const used =
      typeof gpu.memoryUsedBytes === 'number' && Number.isFinite(gpu.memoryUsedBytes)
        ? Math.max(0, gpu.memoryUsedBytes)
        : 0;
    parts.push(`${formatBytes(used)} / ${formatBytes(gpu.memoryTotalBytes)}`);
  }
  return parts.join(' · ');
};

const formatPowerWatts = (value: number) => {
  if (!Number.isFinite(value)) return '';
  if (Math.abs(value) >= 100) return `${Math.round(value).toLocaleString()} W`;
  return `${value.toFixed(1)} W`;
};

const sortedFiniteEntries = (record?: Record<string, number>) =>
  Object.entries(record ?? {})
    .filter(([, value]) => Number.isFinite(value))
    .sort(([a], [b]) => a.localeCompare(b));

export const buildPowerRows = (power?: HostSensorSummary['powerWatts']) =>
  sortedFiniteEntries(power).map(([name, raw]) => {
    const label = formatPowerSensorLabel(name);
    const value = formatPowerWatts(raw);
    return { label, value, valueTitle: `${label} ${value}` };
  });

const formatFanRPM = (value: number) => {
  if (!Number.isFinite(value)) return '';
  return `${Math.round(Math.max(0, value)).toLocaleString()} RPM`;
};

const buildTypedGPUTemperatureKeys = (gpus?: HostSensorSummary['gpu']) => {
  const keys = new Set<string>();
  gpus?.forEach((gpu) => {
    const id = (gpu.id || '').trim();
    if (!id) return;
    if (typeof gpu.temperatureCelsius !== 'number' || !Number.isFinite(gpu.temperatureCelsius))
      return;
    keys.add(`gpu_nvidia_${id}`);
  });
  return keys;
};

// Matches the backend's reason for readings a host agent past its reporting
// lease no longer collects (models.HostAgentStoppedReportingReason).
export const HOST_AGENT_STOPPED_REPORTING_REASON = 'host agent stopped reporting';

export type TemperatureRowsOptions = {
  /**
   * Why the machine's own sensor readings are retained rather than current,
   * such as a host agent past its reporting lease. Disk rows keep their own
   * collection state instead.
   */
  lastKnownReason?: string;
};

export const buildTemperatureRows = (
  sensors?: HostSensorSummary,
  options: TemperatureRowsOptions = {},
) => {
  const rows: { label: string; value: string; valueTitle?: string }[] = [];
  const thermalState = sensors?.thermalState;
  const typedGPUTemperatureKeys = buildTypedGPUTemperatureKeys(sensors?.gpu);
  if (thermalState?.pressure) {
    rows.push({
      label: 'Thermal pressure',
      value: titleCaseDelimitedLabel(thermalState.pressure),
      valueTitle: thermalState.source
        ? `${titleCaseDelimitedLabel(thermalState.pressure)} via ${thermalState.source}`
        : titleCaseDelimitedLabel(thermalState.pressure),
    });
  }
  for (const [name, value] of sortedFiniteEntries(thermalState?.limitsPercent)) {
    if (value >= 100) continue;
    const label = formatSensorName(name);
    const pct = `${Math.round(value)}%`;
    rows.push({ label, value: pct, valueTitle: `${label} ${pct}` });
  }
  sensors?.gpu?.forEach((gpu, index) => {
    const value = formatGPUStatsValue(gpu);
    if (value) rows.push({ label: formatGPUStatsLabel(gpu.id, index), value, valueTitle: value });
  });
  for (const [name, temp] of sortedFiniteEntries(sensors?.temperatureCelsius)) {
    if (!typedGPUTemperatureKeys.has(name)) {
      rows.push({
        label: formatSensorName(name),
        value: formatTemperature(temp),
        valueTitle: `${temp.toFixed(1)}°C`,
      });
    }
  }
  for (const [name, value] of sortedFiniteEntries(sensors?.additional)) {
    rows.push({
      label: formatSensorHardwareName(name),
      value: formatTemperature(value),
      valueTitle: `${value.toFixed(1)}°C`,
    });
  }
  for (const [name, raw] of sortedFiniteEntries(sensors?.fanRpm)) {
    const label = formatFanSensorLabel(name);
    const value = formatFanRPM(raw);
    rows.push({ label, value, valueTitle: `${label} ${value}` });
  }

  rows.push(...buildPowerRows(sensors?.powerWatts));

  // This card has no other cue that a value is not current, so a retained
  // sensor reading says so in its value, as a retained disk reading does.
  const lastKnownReason = options.lastKnownReason?.trim();
  if (lastKnownReason) {
    const title = `Last known reading, not current: ${lastKnownReason}`;
    for (const row of rows) {
      row.value = `${row.value} (last known)`;
      row.valueTitle = title;
    }
  }

  const smart = sensors?.smart;
  if (smart) {
    smart
      .filter((disk) => !disk.standby)
      .sort((a, b) => a.device.localeCompare(b.device))
      .forEach((disk) => {
        const temperature = getPhysicalDiskTemperaturePresentation(disk);
        if (!temperature) return;
        // A retained reading, such as a silent agent's, says so in the value
        // because this card has no other cue that it is not current.
        rows.push({
          label: `Disk ${disk.device}`,
          value: temperature.current ? temperature.label : `${temperature.label} (last known)`,
          valueTitle: temperature.title ?? `${disk.temperature.toFixed(1)}°C`,
        });
      });
  }

  return rows;
};

const formatCustomSensorValue = (value: number, unit?: string): string => {
  const formatted = new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 3,
  }).format(value);
  const normalizedUnit = (unit || '').trim();
  if (!normalizedUnit) return formatted;
  if (normalizedUnit === '%' || normalizedUnit.startsWith('°'))
    return `${formatted}${normalizedUnit}`;
  return `${formatted} ${normalizedUnit}`;
};

const formatCustomSensorAge = (seconds: number): string => {
  const normalized = Math.max(0, Math.floor(seconds));
  if (normalized < 60) return `${normalized}s ago`;
  if (normalized < 3600) return `${Math.floor(normalized / 60)}m ago`;
  if (normalized < 86400) return `${Math.floor(normalized / 3600)}h ago`;
  return `${Math.floor(normalized / 86400)}d ago`;
};

export const buildCustomSensorRows = (sensors?: HostSensorSummary) =>
  (sensors?.custom ?? [])
    .filter((metric) => metric.id?.trim() && metric.name?.trim())
    .sort((a, b) =>
      [a.group, a.subgroup, a.name]
        .map((part) => part?.trim() ?? '')
        .join('\u0000')
        .localeCompare(
          [b.group, b.subgroup, b.name].map((part) => part?.trim() ?? '').join('\u0000'),
        ),
    )
    .map((metric) => {
      const status = (metric.status || 'unknown').trim().toLowerCase();
      const hasValue = typeof metric.value === 'number' && Number.isFinite(metric.value);
      const kind = (metric.kind || 'number').trim().toLowerCase();
      let measured = 'Unavailable';
      if (hasValue && kind === 'boolean') {
        measured = (metric.value as number) >= 0.5 ? 'Online' : 'Offline';
      } else if (hasValue && kind === 'timestamp') {
        measured = formatCustomSensorAge(metric.value as number);
      } else if (hasValue) {
        measured = formatCustomSensorValue(metric.value as number, metric.unit);
      }
      const value = metric.stale ? `${measured} (stale)` : measured;
      const details = [
        status === 'ok' ? 'OK' : titleCaseDelimitedLabel(status),
        metric.error?.trim(),
      ].filter(Boolean);
      const label = [metric.group, metric.subgroup, metric.name]
        .map((part) => part?.trim())
        .filter(Boolean)
        .join(' / ');
      return {
        label,
        value,
        valueTitle: details.length > 0 ? `${value} · ${details.join(' · ')}` : value,
      };
    });

export const formatInteger = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return new Intl.NumberFormat().format(Math.round(value));
};
