import { Show, Suspense, createMemo, For, createSignal, createEffect } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { Disk, Host, HostNetworkInterface, HostSensorSummary, Memory, Node } from '@/types/api';
import type { Resource, ResourceMetric } from '@/types/resource';
import { getDisplayName } from '@/types/resource';
import { formatUptime, formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/Dashboard/TagBadges';
import { getHostStatusIndicator } from '@/utils/status';
import { getPlatformBadge, getSourceBadge, getTypeBadge, getUnifiedSourceBadges } from './resourceBadges';
import { buildWorkloadsHref } from './workloadsLink';
import { buildServiceDetailLinks } from './serviceDetailLinks';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { ReportMergeModal } from './ReportMergeModal';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import { useBreakpoint } from '@/hooks/useBreakpoint';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
}

type ProxmoxPlatformData = {
  nodeName?: string;
  clusterName?: string;
  pveVersion?: string;
  kernelVersion?: string;
  uptime?: number;
  cpuInfo?: { model?: string; cores?: number; sockets?: number };
};

type AgentDiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
};

type AgentPlatformData = {
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
  cpuCount?: number;
  memory?: Memory;
};

type PBSPlatformData = {
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

type PMGPlatformData = {
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

type KubernetesMetricCapabilities = {
  nodeCpuMemory?: boolean;
  nodeTelemetry?: boolean;
  podCpuMemory?: boolean;
  podNetwork?: boolean;
  podEphemeralDisk?: boolean;
  podDiskIo?: boolean;
};

type KubernetesPlatformData = {
  metricCapabilities?: KubernetesMetricCapabilities;
};

type PlatformData = {
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

export type DiscoveryConfig = {
  resourceType: DiscoveryResourceType;
  hostId: string;
  resourceId: string;
  hostname: string;
  metadataKind: 'guest' | 'host';
  metadataId: string;
  targetLabel: string;
};

export const toDiscoveryConfig = (resource: Resource): DiscoveryConfig | null => {
  const explicitDiscoveryTarget = resource.discoveryTarget;
  if (
    explicitDiscoveryTarget &&
    explicitDiscoveryTarget.resourceType &&
    explicitDiscoveryTarget.hostId &&
    explicitDiscoveryTarget.resourceId
  ) {
    const resourceType = (() => {
      switch (explicitDiscoveryTarget.resourceType) {
        case 'host':
        case 'vm':
        case 'lxc':
        case 'docker':
        case 'k8s':
          return explicitDiscoveryTarget.resourceType;
        default:
          return null;
      }
    })();

    if (resourceType) {
      const hostname =
        explicitDiscoveryTarget.hostname ||
        resource.identity?.hostname ||
        resource.displayName ||
        resource.name ||
        explicitDiscoveryTarget.resourceId;
      const isHostDiscovery = resourceType === 'host';
      const targetLabel = isHostDiscovery
        ? 'host'
        : resourceType === 'docker'
          ? 'container'
          : resourceType === 'k8s'
            ? 'workload'
            : 'guest';
      return {
        resourceType,
        hostId: explicitDiscoveryTarget.hostId,
        resourceId: explicitDiscoveryTarget.resourceId,
        hostname,
        metadataKind: isHostDiscovery ? 'host' : 'guest',
        metadataId: explicitDiscoveryTarget.resourceId,
        targetLabel,
      };
    }
  }

  const platformData = resource.platformData as PlatformData | undefined;
  const hostLookupId =
    platformData?.agent?.agentId ||
    platformData?.proxmox?.nodeName ||
    platformData?.agent?.hostname ||
    ((platformData?.docker as { hostname?: string } | undefined)?.hostname) ||
    resource.identity?.hostname ||
    resource.name ||
    resource.platformId ||
    resource.id;
  const hostLikeId = hostLookupId;
  const workloadHostId = resource.platformId || resource.parentId || resource.id;
  const hostname = resource.identity?.hostname || resource.displayName || resource.name || resource.id;

  switch (resource.type) {
    case 'host':
    case 'node':
    case 'docker-host':
    case 'pbs':
    case 'pmg':
    case 'k8s-cluster':
    case 'k8s-node':
    case 'truenas':
      return {
        resourceType: 'host',
        hostId: hostLikeId,
        resourceId: hostLikeId,
        hostname,
        metadataKind: 'host',
        metadataId: hostLikeId,
        targetLabel: 'host',
      };
    case 'vm':
      return {
        resourceType: 'vm',
        hostId: workloadHostId,
        resourceId: resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'container':
    case 'oci-container':
      return {
        resourceType: 'lxc',
        hostId: workloadHostId,
        resourceId: resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'docker-container':
      return {
        resourceType: 'docker',
        hostId: workloadHostId,
        resourceId: resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'container',
      };
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      return {
        resourceType: 'k8s',
        hostId: workloadHostId,
        resourceId: resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'workload',
      };
    default:
      return null;
  }
};



const buildMemory = (metric?: ResourceMetric, fallback?: Partial<Memory>): Memory => {
  const total = metric?.total ?? fallback?.total ?? 0;
  const used = metric?.used ?? fallback?.used ?? 0;
  const free = metric?.free ?? fallback?.free ?? Math.max(total - used, 0);
  const usage = total > 0 ? used / total : fallback?.usage ?? 0;
  return {
    total,
    used,
    free,
    usage,
  };
};

const buildDisk = (metric?: ResourceMetric, fallback?: Partial<Disk>): Disk => {
  const total = metric?.total ?? fallback?.total ?? 0;
  const used = metric?.used ?? fallback?.used ?? 0;
  const free = metric?.free ?? fallback?.free ?? Math.max(total - used, 0);
  const usage = total > 0 ? used / total : fallback?.usage ?? 0;
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

const toHostDisks = (disks?: AgentDiskInfo[]): Disk[] | undefined => {
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

const toNodeFromProxmox = (resource: Resource): Node | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const proxmox = platformData?.proxmox;
  if (!proxmox) return null;

  const memory = buildMemory(resource.memory);
  const disk = buildDisk(resource.disk);
  const lastSeen = Number.isFinite(resource.lastSeen) ? new Date(resource.lastSeen).toISOString() : new Date().toISOString();

  return {
    id: resource.id,
    name: proxmox.nodeName ?? resource.name ?? resource.platformId ?? resource.id,
    displayName: resource.displayName ?? resource.name,
    instance: resource.platformId ?? resource.id,
    host: proxmox.nodeName ?? resource.platformId ?? resource.id,
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
  } as Node;
};

const toHostFromAgent = (resource: Resource): Host | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const agent = platformData?.agent;
  if (!agent) return null;

  const proxmoxCores = platformData?.proxmox?.cpuInfo?.cores;
  const cpuCount = [agent.cpuCount, (agent as { cpuCores?: number }).cpuCores, (agent as { cores?: number }).cores, proxmoxCores]
    .find((value) => typeof value === 'number' && value > 0);

  const hostname = agent.hostname ?? resource.platformId ?? resource.name ?? resource.id;

  return {
    id: resource.id,
    hostname,
    displayName: resource.displayName ?? hostname,
    platform: agent.platform,
    osName: agent.osName ?? 'Unknown',
    osVersion: agent.osVersion ?? '',
    kernelVersion: agent.kernelVersion ?? 'Unknown',
    architecture: agent.architecture ?? 'Unknown',
    cpuCount,
    memory: buildMemory(resource.memory, agent.memory),
    disks: toHostDisks(agent.disks),
    networkInterfaces: agent.networkInterfaces,
    sensors: agent.sensors,
    status: resource.status,
    uptimeSeconds: agent.uptimeSeconds ?? resource.uptime ?? 0,
    lastSeen: resource.lastSeen,
    agentVersion: agent.agentVersion,
    tags: resource.tags,
  } as Host;
};

const formatSensorName = (name: string) => {
  let clean = name.replace(/^[a-z]+\d*_/i, '');
  clean = clean.replace(/_/g, ' ');
  return clean.replace(/\b\w/g, (char) => char.toUpperCase());
};

const buildTemperatureRows = (sensors?: HostSensorSummary) => {
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

const normalizeHealthLabel = (value?: string): string => {
  const raw = (value || '').trim();
  if (!raw) return 'Unknown';
  if (raw.length <= 3) return raw.toUpperCase();
  return raw.charAt(0).toUpperCase() + raw.slice(1);
};

const healthToneClass = (value?: string): string => {
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
  return 'text-gray-700 dark:text-gray-200';
};

const formatInteger = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return new Intl.NumberFormat().format(Math.round(value));
};

const ALIAS_COLLAPSE_THRESHOLD = 4;

const formatSourceType = (value: Resource['sourceType']): string => {
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

const DrawerContent: Component<ResourceDetailDrawerProps> = (props) => {
  type DrawerTab = 'overview' | 'discovery' | 'debug';
  const [activeTab, setActiveTab] = createSignal<DrawerTab>('overview');
  const [debugEnabled] = createLocalStorageBooleanSignal(STORAGE_KEYS.DEBUG_MODE, false);
  const [copied, setCopied] = createSignal(false);
  const [showReportModal, setShowReportModal] = createSignal(false);

  const displayName = createMemo(() => getDisplayName(props.resource));
  const statusIndicator = createMemo(() => getHostStatusIndicator({ status: props.resource.status }));
  const lastSeen = createMemo(() => formatRelativeTime(props.resource.lastSeen));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(props.resource.lastSeen));

  const platformBadge = createMemo(() => getPlatformBadge(props.resource.platformType));
  const sourceBadge = createMemo(() => getSourceBadge(props.resource.sourceType));
  const typeBadge = createMemo(() => getTypeBadge(props.resource.type));
  const unifiedSourceBadges = createMemo(() => {
    const platformData = props.resource.platformData as PlatformData | undefined;
    return getUnifiedSourceBadges(platformData?.sources ?? []);
  });
  const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
  const platformData = createMemo(() => props.resource.platformData as PlatformData | undefined);
  const kubernetesCapabilityBadges = createMemo(() => {
    const capabilities = platformData()?.kubernetes?.metricCapabilities;
    if (!capabilities) return [];

    const supportedBadge = 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400';
    const unsupportedBadge = 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300';
    const badges: { label: string; classes: string; title: string }[] = [];

    if (capabilities.nodeCpuMemory) {
      badges.push({
        label: 'K8s Node CPU/Memory',
        classes: supportedBadge,
        title: 'Node CPU and memory metrics are available.',
      });
    }
    if (capabilities.nodeTelemetry) {
      badges.push({
        label: 'Node Telemetry (Agent)',
        classes: supportedBadge,
        title: 'Linked Pulse host agent provides node uptime, temperature, disk, network, and disk I/O.',
      });
    }
    if (capabilities.podCpuMemory) {
      badges.push({
        label: 'Pod CPU/Memory',
        classes: supportedBadge,
        title: 'Pod CPU and memory metrics are available.',
      });
    }
    if (capabilities.podNetwork) {
      badges.push({
        label: 'Pod Network',
        classes: supportedBadge,
        title: 'Pod network throughput is available.',
      });
    }
    if (capabilities.podEphemeralDisk) {
      badges.push({
        label: 'Pod Ephemeral Disk',
        classes: supportedBadge,
        title: 'Pod ephemeral storage usage is available.',
      });
    }
    if (!capabilities.podDiskIo) {
      badges.push({
        label: 'Pod Disk I/O Unsupported',
        classes: unsupportedBadge,
        title: 'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
      });
    }

    return badges;
  });

  const proxmoxNode = createMemo(() => toNodeFromProxmox(props.resource));
  const agentHost = createMemo(() => toHostFromAgent(props.resource));
  const temperatureRows = createMemo(() => buildTemperatureRows(agentHost()?.sensors));

  const pbsData = createMemo(() => platformData()?.pbs);
  const pmgData = createMemo(() => platformData()?.pmg);
  const pbsJobTotal = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return 0;
    return (
      (pbs.backupJobCount || 0) +
      (pbs.syncJobCount || 0) +
      (pbs.verifyJobCount || 0) +
      (pbs.pruneJobCount || 0) +
      (pbs.garbageJobCount || 0)
    );
  });
  const pmgQueueBacklog = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return 0;
    return (pmg.queueDeferred || 0) + (pmg.queueHold || 0);
  });
  const pmgUpdatedRelative = createMemo(() => {
    const raw = pmgData()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const pbsJobBreakdown = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Backup', value: pbs.backupJobCount || 0 },
      { label: 'Sync', value: pbs.syncJobCount || 0 },
      { label: 'Verify', value: pbs.verifyJobCount || 0 },
      { label: 'Prune', value: pbs.pruneJobCount || 0 },
      { label: 'Garbage', value: pbs.garbageJobCount || 0 },
    ];
  });
  const pbsVisibleJobBreakdown = createMemo(() => {
    const all = pbsJobBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgQueueBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number; warn?: boolean }>;
    return [
      { label: 'Active', value: pmg.queueActive || 0 },
      { label: 'Deferred', value: pmg.queueDeferred || 0, warn: (pmg.queueDeferred || 0) > 0 },
      { label: 'Hold', value: pmg.queueHold || 0, warn: (pmg.queueHold || 0) > 0 },
      { label: 'Incoming', value: pmg.queueIncoming || 0 },
    ];
  });
  const pmgVisibleQueueBreakdown = createMemo(() => {
    const all = pmgQueueBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgMailBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Mail', value: pmg.mailCountTotal || 0 },
      { label: 'Spam', value: pmg.spamIn || 0 },
      { label: 'Virus', value: pmg.virusIn || 0 },
    ];
  });
  const pmgVisibleMailBreakdown = createMemo(() => {
    const all = pmgMailBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo(() => platformData()?.sourceStatus ?? {});
  const sourceHealthSummary = createMemo(() => {
    const entries = Object.entries(sourceStatus());
    if (entries.length === 0) return null;

    let healthy = 0;
    let warning = 0;
    let unhealthy = 0;
    const parts: string[] = [];

    for (const [source, status] of entries) {
      const normalized = (status?.status || '').trim().toLowerCase();
      parts.push(`${source}:${normalized || 'unknown'}`);
      if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) {
        healthy += 1;
      } else if (['degraded', 'warning', 'stale'].includes(normalized)) {
        warning += 1;
      } else {
        unhealthy += 1;
      }
    }

    const total = entries.length;
    if (unhealthy > 0) {
      return {
        label: `${unhealthy}/${total} unhealthy`,
        className: 'text-red-600 dark:text-red-400',
        title: parts.join(' • '),
      };
    }
    if (warning > 0) {
      return {
        label: `${warning}/${total} degraded`,
        className: 'text-amber-600 dark:text-amber-400',
        title: parts.join(' • '),
      };
    }
    return {
      label: `${healthy}/${total} healthy`,
      className: 'text-emerald-600 dark:text-emerald-400',
      title: parts.join(' • '),
    };
  });
  const sourceSummary = createMemo(() => {
    const health = sourceHealthSummary();
    if (health) return health;
    const sources = mergedSources();
    if (sources.length === 0) return null;
    return {
      label: sources.length === 1 ? sources[0].toUpperCase() : `${sources.length} sources`,
      className: 'text-gray-700 dark:text-gray-200',
      title: sources.join(' • '),
    };
  });
  const identityAliasValues = createMemo(() => {
    const data = platformData();
    const pbs = data?.pbs;
    const pmg = data?.pmg;
    const proxmox = data?.proxmox;
    const agent = data?.agent;
    const raw = [
      props.resource.discoveryTarget?.hostId,
      props.resource.discoveryTarget?.resourceId,
      proxmox?.nodeName,
      agent?.agentId,
      agent?.hostname,
      pbs?.instanceId,
      pbs?.hostname,
      pmg?.instanceId,
      pmg?.hostname,
      props.resource.identity?.hostname,
      props.resource.identity?.machineId,
    ];
    const seen = new Set<string>();
    const deduped: string[] = [];
    for (const value of raw) {
      if (!value) continue;
      const trimmed = value.trim();
      if (!trimmed) continue;
      const normalized = trimmed.toLowerCase();
      if (seen.has(normalized)) continue;
      seen.add(normalized);
      deduped.push(trimmed);
    }
    return deduped;
  });
  const primaryIdentityRows = createMemo(() => {
    const rows: Array<{ label: string; value: string }> = [];
    if (props.resource.identity?.hostname) {
      rows.push({ label: 'Hostname', value: props.resource.identity.hostname });
    }
    if (props.resource.identity?.machineId) {
      rows.push({ label: 'Machine ID', value: props.resource.identity.machineId });
    }
    if (props.resource.clusterId) {
      rows.push({ label: 'Cluster', value: props.resource.clusterId });
    }
    if (props.resource.parentId) {
      rows.push({ label: 'Parent', value: props.resource.parentId });
    }
    if (props.resource.discoveryTarget?.resourceType) {
      rows.push({
        label: 'Discovery',
        value: `${props.resource.discoveryTarget.resourceType}:${props.resource.discoveryTarget.resourceId}`,
      });
    }
    return rows;
  });
  const identityCardHasRichData = createMemo(() =>
    primaryIdentityRows().length > 0 ||
    (props.resource.identity?.ips?.length || 0) > 0 ||
    (props.resource.tags?.length || 0) > 0 ||
    identityAliasValues().length > 0,
  );
  const aliasPreviewValues = createMemo(() => identityAliasValues().slice(0, ALIAS_COLLAPSE_THRESHOLD));
  const hasAliasOverflow = createMemo(() => identityAliasValues().length > ALIAS_COLLAPSE_THRESHOLD);
  const hasMergedSources = createMemo(() => mergedSources().length > 1);
  const discoveryConfig = createMemo(() => toDiscoveryConfig(props.resource));
  const workloadsHref = createMemo(() => buildWorkloadsHref(props.resource));
  const relatedLinks = createMemo(() => {
    const links: Array<{ href: string; label: string; ariaLabel: string }> = [];
    const workloads = workloadsHref();
    if (workloads) {
      links.push({
        href: workloads,
        label: 'Open in Workloads',
        ariaLabel: `Open related workloads for ${displayName()}`,
      });
    }
    links.push(...buildServiceDetailLinks(props.resource));
    const seen = new Set<string>();
    return links.filter((link) => {
      if (seen.has(link.href)) return false;
      seen.add(link.href);
      return true;
    });
  });
  const sourceSections = createMemo(() => {
    const data = platformData();
    if (!data) return [];
    const sections = [
      { id: 'proxmox', label: 'Proxmox', payload: data.proxmox },
      { id: 'agent', label: 'Agent', payload: data.agent },
      { id: 'docker', label: 'Containers', payload: data.docker },
      { id: 'pbs', label: 'PBS', payload: data.pbs },
      { id: 'pmg', label: 'PMG', payload: data.pmg },
      { id: 'kubernetes', label: 'Kubernetes', payload: data.kubernetes },
      { id: 'metrics', label: 'Metrics', payload: data.metrics },
    ];
    return sections.filter((section) => section.payload !== undefined);
  });
  const identityMatchInfo = createMemo(() => {
    const data = platformData();
    return (
      data?.identityMatch ??
      data?.matchResults ??
      data?.matchCandidates ??
      data?.matches ??
      undefined
    );
  });
  const debugBundle = createMemo(() => ({
    resource: props.resource,
    identity: {
      resourceIdentity: props.resource.identity,
      matchInfo: identityMatchInfo(),
    },
    sources: {
      sourceStatus: sourceStatus(),
      proxmox: platformData()?.proxmox,
      agent: platformData()?.agent,
      docker: platformData()?.docker,
      pbs: platformData()?.pbs,
      pmg: platformData()?.pmg,
      kubernetes: platformData()?.kubernetes,
      metrics: platformData()?.metrics,
    },
  }));
  const debugJson = createMemo(() => JSON.stringify(debugBundle(), null, 2));

  createEffect(() => {
    if (!debugEnabled() && activeTab() === 'debug') {
      setActiveTab('overview');
    }
  });

  const tabs = createMemo(() => {
    const base = [
      { id: 'overview' as DrawerTab, label: 'Overview' },
      { id: 'discovery' as DrawerTab, label: 'Discovery' },
    ];
    if (debugEnabled()) {
      base.push({ id: 'debug' as DrawerTab, label: 'Debug' });
    }
    return base;
  });

  const formatSourceTime = (value?: string | number) => {
    if (!value) return '';
    const timestamp = typeof value === 'number' ? value : Date.parse(value);
    if (!Number.isFinite(timestamp)) return '';
    return formatRelativeTime(timestamp);
  };

  const handleCopyJson = async () => {
    const payload = debugJson();
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(payload);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = payload;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div class="space-y-3">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              variant={statusIndicator().variant}
              title={statusIndicator().label}
              ariaLabel={statusIndicator().label}
              size="sm"
            />
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate" title={displayName()}>
              {displayName()}
            </div>
          </div>
          <div class="text-[11px] text-gray-500 dark:text-gray-400 truncate" title={props.resource.id}>
            {props.resource.id}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Show when={typeBadge()}>
              {(badge) => (
                <span class={badge().classes} title={badge().title}>
                  {badge().label}
                </span>
              )}
            </Show>
            <Show
              when={hasUnifiedSources()}
              fallback={
                <>
                  <Show when={platformBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                  <Show when={sourceBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                </>
              }
            >
              <For each={unifiedSourceBadges()}>
                {(badge) => (
                  <span class={badge.classes} title={badge.title}>
                    {badge.label}
                  </span>
                )}
              </For>
            </Show>
            <For each={kubernetesCapabilityBadges()}>
              {(badge) => (
                <span class={badge.classes} title={badge.title}>
                  {badge.label}
                </span>
              )}
            </For>
          </div>
        </div>

        <Show when={props.onClose}>
          <button
            type="button"
            onClick={() => props.onClose?.()}
            class="text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            Close
          </button>
        </Show>
      </div>

      <Show when={relatedLinks().length > 0}>
        <div class="flex items-center justify-end gap-2">
          <For each={relatedLinks()}>
            {(link) => (
              <a
                href={link.href}
                aria-label={link.ariaLabel}
                class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
              >
                {link.label}
              </a>
            )}
          </For>
          </div>
      </Show>

      <div class="flex items-center gap-6 border-b border-gray-200 dark:border-gray-700 px-1 mb-1">
        <For each={tabs()}>
          {(tab) => (
            <button
              onClick={() => setActiveTab(tab.id)}
              class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === tab.id
                ? 'text-blue-600 dark:text-blue-400'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
            >
              {tab.label}
              <Show when={activeTab() === tab.id}>
                <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
              </Show>
            </button>
          )}
        </For>
      </div>

      {/* Overview Tab */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        <Show when={proxmoxNode() || agentHost()}>
          <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
            <Show when={proxmoxNode()}>
              {(node) => (
                <>
                  <SystemInfoCard variant="node" node={node()} />
                  <HardwareCard variant="node" node={node()} />
                  <RootDiskCard node={node()} />
                </>
              )}
            </Show>
            <Show when={agentHost()}>
              {(host) => (
                <>
                  <SystemInfoCard variant="host" host={host()} />
                  <HardwareCard variant="host" host={host()} />
                  <NetworkInterfacesCard interfaces={host().networkInterfaces} />
                  <DisksCard disks={host().disks} />
                  <TemperaturesCard rows={temperatureRows()} />
                </>
              )}
            </Show>
          </div>
        </Show>

        <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3 mt-3">
          <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Runtime</div>
            <div class="space-y-1.5 text-[11px]">
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">State</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 capitalize">{props.resource.status || 'unknown'}</span>
              </div>
              <Show when={props.resource.uptime}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200">{formatUptime(props.resource.uptime ?? 0)}</span>
                </div>
              </Show>
              <Show when={props.resource.lastSeen}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Last Seen</span>
                  <span
                    class="font-medium text-gray-700 dark:text-gray-200"
                    title={lastSeenAbsolute()}
                  >
                    {lastSeen() || '—'}
                  </span>
                </div>
              </Show>
              <Show when={sourceSummary()}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Sources</span>
                  <span class={`font-medium ${sourceSummary()!.className}`} title={sourceSummary()!.title}>
                    {sourceSummary()!.label}
                  </span>
                </div>
              </Show>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Mode</span>
                <span class="font-medium text-gray-700 dark:text-gray-200">{formatSourceType(props.resource.sourceType)}</span>
              </div>
              <Show when={(props.resource.alerts?.length || 0) > 0}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Alerts</span>
                  <span class="font-medium text-amber-600 dark:text-amber-400">
                    {formatInteger(props.resource.alerts?.length)}
                  </span>
                </div>
              </Show>
              <Show when={props.resource.platformId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Platform ID</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.platformId}>
                    {props.resource.platformId}
                  </span>
                </div>
              </Show>
            </div>
          </div>

          <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Identity</div>
            <div class="space-y-1.5 text-[11px]">
              <For each={primaryIdentityRows()}>
                {(row) => (
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-gray-500 dark:text-gray-400">{row.label}</span>
                    <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={row.value}>
                      {row.value}
                    </span>
                  </div>
                )}
              </For>
              <Show when={props.resource.identity?.ips && props.resource.identity.ips.length > 0}>
                <div class="flex flex-col gap-1">
                  <span class="text-gray-500 dark:text-gray-400">IP Addresses</span>
                  <div class="flex flex-wrap gap-1">
                    <For each={props.resource.identity?.ips ?? []}>
                      {(ip) => (
                        <span
                          class="inline-flex items-center rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                          title={ip}
                        >
                          {ip}
                        </span>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
              <Show when={props.resource.tags && props.resource.tags.length > 0}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Tags</span>
                  <TagBadges tags={props.resource.tags} maxVisible={6} />
                </div>
              </Show>
              <Show when={identityAliasValues().length > 0}>
                <Show
                  when={hasAliasOverflow()}
                  fallback={
                    <div class="flex flex-col gap-1">
                      <span class="text-gray-500 dark:text-gray-400">Aliases</span>
                      <div class="flex flex-wrap gap-1">
                        <For each={aliasPreviewValues()}>
                          {(value) => (
                            <span class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-700 dark:bg-gray-700/60 dark:text-gray-200" title={value}>
                              {value}
                            </span>
                          )}
                        </For>
                      </div>
                    </div>
                  }
                >
                  <details class="rounded border border-gray-200/80 bg-white/80 px-2 py-1.5 dark:border-gray-600/60 dark:bg-gray-900/30">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-gray-600 dark:text-gray-300">
                      <span>Aliases</span>
                      <span class="text-gray-500 dark:text-gray-400">{identityAliasValues().length}</span>
                    </summary>
                    <div class="mt-2 flex flex-wrap gap-1 border-t border-gray-200/80 pt-2 dark:border-gray-600/50">
                      <For each={identityAliasValues()}>
                        {(value) => (
                          <span class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-700 dark:bg-gray-700/60 dark:text-gray-200" title={value}>
                            {value}
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                </Show>
              </Show>
              <Show when={!identityCardHasRichData()}>
                <div class="rounded border border-dashed border-gray-300 bg-gray-50/70 px-2 py-1.5 text-[10px] text-gray-500 dark:border-gray-600 dark:bg-gray-900/30 dark:text-gray-400">
                  No enriched identity metadata yet.
                </div>
              </Show>
            </div>
          </div>

          <Show when={pbsData()}>
            {(pbs) => (
              <div class="rounded border border-indigo-200 bg-indigo-50/60 p-3 shadow-sm dark:border-indigo-700/60 dark:bg-indigo-900/20">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-indigo-700 dark:text-indigo-300">PBS Service</div>
                  <Show when={pbs().hostname}>
                    <span class="max-w-[55%] truncate text-[10px] text-indigo-700/80 dark:text-indigo-300/80" title={pbs().hostname}>
                      {pbs().hostname}
                    </span>
                  </Show>
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-gray-500 dark:text-gray-400">Connection</span>
                    <span class={`font-medium ${healthToneClass(pbs().connectionHealth)}`}>
                      {normalizeHealthLabel(pbs().connectionHealth)}
                    </span>
                  </div>
                  <Show when={pbs().version}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-gray-500 dark:text-gray-400">Version</span>
                      <span class="font-medium text-gray-700 dark:text-gray-200">{pbs().version}</span>
                    </div>
                  </Show>
                  <Show when={pbs().uptimeSeconds || props.resource.uptime}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                      <span class="font-medium text-gray-700 dark:text-gray-200">
                        {formatUptime(pbs().uptimeSeconds ?? props.resource.uptime ?? 0)}
                      </span>
                    </div>
                  </Show>
                  <div class="grid grid-cols-2 gap-2 pt-1">
                    <div class="rounded border border-indigo-200/70 bg-white/70 px-2 py-1.5 dark:border-indigo-700/50 dark:bg-gray-900/30">
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">Datastores</div>
                      <div class="text-sm font-semibold text-gray-700 dark:text-gray-200">{formatInteger(pbs().datastoreCount)}</div>
                    </div>
                    <div class="rounded border border-indigo-200/70 bg-white/70 px-2 py-1.5 dark:border-indigo-700/50 dark:bg-gray-900/30">
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">Total Jobs</div>
                      <div class="text-sm font-semibold text-gray-700 dark:text-gray-200">{formatInteger(pbsJobTotal())}</div>
                    </div>
                  </div>
                  <details class="rounded border border-indigo-200/70 bg-white/70 px-2 py-1.5 dark:border-indigo-700/50 dark:bg-gray-900/30">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-gray-600 dark:text-gray-300">
                      <span>Job breakdown</span>
                      <span class="text-gray-500 dark:text-gray-400">{pbsVisibleJobBreakdown().length} types</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-indigo-200/60 pt-2 text-[10px] dark:border-indigo-700/40">
                      <For each={pbsVisibleJobBreakdown()}>
                        {(entry) => (
                          <span class="text-gray-500 dark:text-gray-400">
                            {entry.label}:{' '}
                            <span class="font-medium text-gray-700 dark:text-gray-200">{formatInteger(entry.value)}</span>
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                </div>
              </div>
            )}
          </Show>

          <Show when={pmgData()}>
            {(pmg) => (
              <div class="rounded border border-rose-200 bg-rose-50/60 p-3 shadow-sm dark:border-rose-700/60 dark:bg-rose-900/20">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-rose-700 dark:text-rose-300">Mail Gateway</div>
                  <Show when={pmg().hostname}>
                    <span class="max-w-[55%] truncate text-[10px] text-rose-700/80 dark:text-rose-300/80" title={pmg().hostname}>
                      {pmg().hostname}
                    </span>
                  </Show>
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-gray-500 dark:text-gray-400">Connection</span>
                    <span class={`font-medium ${healthToneClass(pmg().connectionHealth)}`}>
                      {normalizeHealthLabel(pmg().connectionHealth)}
                    </span>
                  </div>
                  <Show when={pmg().version}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-gray-500 dark:text-gray-400">Version</span>
                      <span class="font-medium text-gray-700 dark:text-gray-200">{pmg().version}</span>
                    </div>
                  </Show>
                  <Show when={pmg().uptimeSeconds || props.resource.uptime}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                      <span class="font-medium text-gray-700 dark:text-gray-200">
                        {formatUptime(pmg().uptimeSeconds ?? props.resource.uptime ?? 0)}
                      </span>
                    </div>
                  </Show>
                  <div class="grid grid-cols-3 gap-2 pt-1">
                    <div class="rounded border border-rose-200/70 bg-white/70 px-2 py-1.5 dark:border-rose-700/50 dark:bg-gray-900/30">
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">Nodes</div>
                      <div class="text-sm font-semibold text-gray-700 dark:text-gray-200">{formatInteger(pmg().nodeCount)}</div>
                    </div>
                    <div class="rounded border border-rose-200/70 bg-white/70 px-2 py-1.5 dark:border-rose-700/50 dark:bg-gray-900/30">
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">Queue Total</div>
                      <div class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-200'}`}>
                        {formatInteger(pmg().queueTotal)}
                      </div>
                    </div>
                    <div class="rounded border border-rose-200/70 bg-white/70 px-2 py-1.5 dark:border-rose-700/50 dark:bg-gray-900/30">
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">Backlog</div>
                      <div class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-200'}`}>
                        {formatInteger(pmgQueueBacklog())}
                      </div>
                    </div>
                  </div>
                  <details class="rounded border border-rose-200/70 bg-white/70 px-2 py-1.5 dark:border-rose-700/50 dark:bg-gray-900/30">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-gray-600 dark:text-gray-300">
                      <span>Queue breakdown</span>
                      <span class="text-gray-500 dark:text-gray-400">{pmgVisibleQueueBreakdown().length} signals</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-rose-200/60 pt-2 text-[10px] dark:border-rose-700/40">
                      <For each={pmgVisibleQueueBreakdown()}>
                        {(entry) => (
                          <span class="text-gray-500 dark:text-gray-400">
                            {entry.label}:{' '}
                            <span class={`font-medium ${entry.warn ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-200'}`}>
                              {formatInteger(entry.value)}
                            </span>
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                  <details class="rounded border border-rose-200/70 bg-white/70 px-2 py-1.5 dark:border-rose-700/50 dark:bg-gray-900/30">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-gray-600 dark:text-gray-300">
                      <span>Mail processing</span>
                      <span class="text-gray-500 dark:text-gray-400">{pmgVisibleMailBreakdown().length} signals</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-3 gap-x-3 gap-y-1 border-t border-rose-200/60 pt-2 text-[10px] dark:border-rose-700/40">
                      <For each={pmgVisibleMailBreakdown()}>
                        {(entry) => (
                          <span class="text-gray-500 dark:text-gray-400">
                            {entry.label}:{' '}
                            <span class="font-medium text-gray-700 dark:text-gray-200">{formatInteger(entry.value)}</span>
                          </span>
                        )}
                      </For>
                    </div>
                    <Show when={pmgUpdatedRelative()}>
                      <div class="mt-2 flex items-center justify-between gap-2 border-t border-rose-200/60 pt-2 text-[10px] dark:border-rose-700/40">
                        <span class="text-gray-500 dark:text-gray-400">Updated</span>
                        <span class="font-medium text-gray-700 dark:text-gray-200">{pmgUpdatedRelative()}</span>
                      </div>
                    </Show>
                  </details>
                </div>
              </div>
            )}
          </Show>
        </div>
      </div>

      {/* Discovery Tab */}
      <div class={activeTab() === 'discovery' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        <Show
          when={discoveryConfig()}
          fallback={
            <div class="rounded border border-dashed border-gray-300 bg-gray-50/70 p-4 text-sm text-gray-600 dark:border-gray-600 dark:bg-gray-900/30 dark:text-gray-300">
              Discovery is not available for this resource type yet.
            </div>
          }
        >
          {(config) => (
            <Suspense
              fallback={
                <div class="flex items-center justify-center py-8">
                  <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                  <span class="ml-2 text-sm text-gray-500 dark:text-gray-400">Loading discovery...</span>
                </div>
              }
            >
              <DiscoveryTab
                resourceType={config().resourceType}
                hostId={config().hostId}
                resourceId={config().resourceId}
                hostname={config().hostname}
                urlMetadataKind={config().metadataKind}
                urlMetadataId={config().metadataId}
                urlTargetLabel={config().targetLabel}
              />
            </Suspense>
          )}
        </Show>
      </div>

      {/* Debug Tab */}
      <Show when={debugEnabled()}>
        <div class={activeTab() === 'debug' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              Debug mode is enabled via localStorage (<code>pulse_debug_mode</code>).
            </div>
            <button
              type="button"
              onClick={handleCopyJson}
              class="rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200 dark:hover:bg-gray-800"
            >
              {copied() ? 'Copied' : 'Copy JSON'}
            </button>
          </div>

          <div class="mt-3 space-y-4">
            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Unified Resource</div>
              <pre class="max-h-[280px] overflow-auto rounded-lg bg-gray-900/90 p-3 text-[11px] text-gray-100">
                {JSON.stringify(props.resource, null, 2)}
              </pre>
            </div>

            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Identity Matching</div>
              <pre class="max-h-[220px] overflow-auto rounded-lg bg-gray-900/90 p-3 text-[11px] text-gray-100">
                {JSON.stringify(
                  {
                    identity: props.resource.identity,
                    matchInfo: identityMatchInfo(),
                  },
                  null,
                  2,
                )}
              </pre>
            </div>

            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Sources</div>
              <div class="space-y-2">
                <For each={sourceSections()}>
                  {(section) => {
                    const status = sourceStatus()[section.id];
                    const lastSeenText = formatSourceTime(status?.lastSeen);
                    return (
                      <details class="rounded-lg border border-gray-200 bg-white/70 p-3 dark:border-gray-700 dark:bg-gray-900/40">
                        <summary class="flex cursor-pointer list-none items-center justify-between text-sm font-medium text-gray-700 dark:text-gray-200">
                          <span>{section.label}</span>
                          <span class="text-[11px] text-gray-500 dark:text-gray-400">
                            {status?.status ?? 'unknown'}
                            {lastSeenText ? ` • ${lastSeenText}` : ''}
                          </span>
                        </summary>
                        <Show when={status?.error}>
                          <div class="mt-2 text-[11px] text-amber-600 dark:text-amber-300">
                            {status?.error}
                          </div>
                        </Show>
                        <pre class="mt-3 max-h-[220px] overflow-auto rounded-lg bg-gray-900/90 p-3 text-[11px] text-gray-100">
                          {JSON.stringify(section.payload ?? {}, null, 2)}
                        </pre>
                      </details>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </div>
      </Show>

      <Show when={hasMergedSources()}>
        <div class="flex items-center justify-end">
          <button
            type="button"
            onClick={() => setShowReportModal(true)}
            class="text-xs font-medium text-gray-500 transition-colors hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            Split merged resource
          </button>
        </div>
      </Show>

      <ReportMergeModal
        isOpen={showReportModal()}
        resourceId={props.resource.id}
        resourceName={displayName()}
        sources={mergedSources()}
        onClose={() => setShowReportModal(false)}
      />
    </div>
  );
};

interface MobileSheetProps {
  onClose: () => void;
  children: JSX.Element;
}

const MobileSheet: Component<MobileSheetProps> = (props) => {
  return (
    <Portal>
      {/* Backdrop */}
      <div class="fixed inset-0 bg-black/40 z-40" onClick={props.onClose} aria-hidden="true" />
      {/* Sheet panel */}
      <div class="fixed inset-x-0 bottom-0 z-50 flex flex-col max-h-[85vh] bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl overflow-hidden">
        {/* Drag handle + close button row */}
        <div class="relative flex items-center justify-center pt-3 pb-2 shrink-0">
          <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
          <button
            type="button"
            onClick={props.onClose}
            class="absolute right-3 top-2 p-1.5 rounded-full text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700"
            aria-label="Close"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        {/* Scrollable content */}
        <div class="overflow-y-auto flex-1 px-4 pb-safe-or-20">{props.children}</div>
      </div>
    </Portal>
  );
};

export const ResourceDetailDrawer: Component<ResourceDetailDrawerProps> = (props) => {
  const { isMobile } = useBreakpoint();

  return (
    <Show
      when={!isMobile()}
      fallback={
        <MobileSheet onClose={props.onClose ?? (() => {})}>
          <DrawerContent resource={props.resource} onClose={props.onClose} />
        </MobileSheet>
      }
    >
      <DrawerContent resource={props.resource} onClose={props.onClose} />
    </Show>
  );
};

export default ResourceDetailDrawer;
