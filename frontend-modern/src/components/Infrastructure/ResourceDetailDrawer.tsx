import { Component, Show, Suspense, createMemo, For, createSignal, createEffect } from 'solid-js';
import type { Disk, Host, HostNetworkInterface, HostSensorSummary, Memory, Node } from '@/types/api';
import type { Resource, ResourceMetric } from '@/types/resource';
import { getDisplayName } from '@/types/resource';
import { formatUptime, formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/Dashboard/TagBadges';
import { getHostStatusIndicator } from '@/utils/status';
import { getPlatformBadge, getSourceBadge, getTypeBadge, getUnifiedSourceBadges } from './resourceBadges';
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

type PlatformData = {
  sources?: string[];
  proxmox?: ProxmoxPlatformData;
  agent?: AgentPlatformData;
  sourceStatus?: Record<string, { status?: string; lastSeen?: string | number; error?: string }>;
  docker?: Record<string, unknown>;
  pbs?: Record<string, unknown>;
  kubernetes?: Record<string, unknown>;
  metrics?: Record<string, unknown>;
  identityMatch?: unknown;
  matchResults?: unknown;
  matchCandidates?: unknown;
  matches?: unknown;
};

type DiscoveryConfig = {
  resourceType: DiscoveryResourceType;
  hostId: string;
  resourceId: string;
  hostname: string;
  metadataKind: 'guest' | 'host';
  metadataId: string;
  targetLabel: string;
};

const toDiscoveryConfig = (resource: Resource): DiscoveryConfig | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const hostLookupId =
    platformData?.agent?.agentId ||
    resource.identity?.hostname ||
    platformData?.agent?.hostname ||
    platformData?.proxmox?.nodeName ||
    ((platformData?.docker as { hostname?: string } | undefined)?.hostname) ||
    resource.name ||
    resource.id;
  const hostLikeId = hostLookupId;
  const workloadHostId = resource.platformId || resource.parentId || resource.id;
  const hostname = resource.identity?.hostname || resource.displayName || resource.name || resource.id;

  switch (resource.type) {
    case 'host':
    case 'node':
    case 'docker-host':
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

export const ResourceDetailDrawer: Component<ResourceDetailDrawerProps> = (props) => {
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

  const proxmoxNode = createMemo(() => toNodeFromProxmox(props.resource));
  const agentHost = createMemo(() => toHostFromAgent(props.resource));
  const temperatureRows = createMemo(() => buildTemperatureRows(agentHost()?.sensors));

  const platformData = createMemo(() => props.resource.platformData as PlatformData | undefined);
  const sourceStatus = createMemo(() => platformData()?.sourceStatus ?? {});
  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const hasMergedSources = createMemo(() => mergedSources().length > 1);
  const discoveryConfig = createMemo(() => toDiscoveryConfig(props.resource));
  const sourceSections = createMemo(() => {
    const data = platformData();
    if (!data) return [];
    const sections = [
      { id: 'proxmox', label: 'Proxmox', payload: data.proxmox },
      { id: 'agent', label: 'Agent', payload: data.agent },
      { id: 'docker', label: 'Docker', payload: data.docker },
      { id: 'pbs', label: 'PBS', payload: data.pbs },
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
            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Status</div>
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
              <Show when={props.resource.platformId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Platform ID</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.platformId}>
                    {props.resource.platformId}
                  </span>
                </div>
              </Show>
              <Show when={props.resource.clusterId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Cluster</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.clusterId}>
                    {props.resource.clusterId}
                  </span>
                </div>
              </Show>
              <Show when={props.resource.parentId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Parent</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.parentId}>
                    {props.resource.parentId}
                  </span>
                </div>
              </Show>
            </div>
          </div>

          <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Identity</div>
            <div class="space-y-1.5 text-[11px]">
              <Show when={props.resource.identity?.hostname}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Hostname</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.identity?.hostname}>
                    {props.resource.identity?.hostname}
                  </span>
                </div>
              </Show>
              <Show when={props.resource.identity?.machineId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-gray-500 dark:text-gray-400">Machine ID</span>
                  <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.identity?.machineId}>
                    {props.resource.identity?.machineId}
                  </span>
                </div>
              </Show>
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
            </div>
          </div>
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

export default ResourceDetailDrawer;
