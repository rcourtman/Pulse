import { Component, Show, createMemo, For } from 'solid-js';
import type { Disk, Host, HostNetworkInterface, HostSensorSummary, Memory, Node } from '@/types/api';
import type { Resource, ResourceMetric } from '@/types/resource';
import { getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent } from '@/types/resource';
import { formatBytes, formatUptime, formatRelativeTime, formatAbsoluteTime, formatPercent } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/Dashboard/TagBadges';
import { buildMetricKey } from '@/utils/metricsKeys';
import { getHostStatusIndicator } from '@/utils/status';
import { getPlatformBadge, getSourceBadge, getTypeBadge, getUnifiedSourceBadges } from './resourceBadges';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';

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
};

const metricSublabel = (metric?: ResourceMetric) => {
  if (!metric || typeof metric.used !== 'number' || typeof metric.total !== 'number') return undefined;
  return `${formatBytes(metric.used)}/${formatBytes(metric.total)}`;
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
  const displayName = createMemo(() => getDisplayName(props.resource));
  const statusIndicator = createMemo(() => getHostStatusIndicator({ status: props.resource.status }));
  const lastSeen = createMemo(() => formatRelativeTime(props.resource.lastSeen));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(props.resource.lastSeen));
  const metricKey = createMemo(() => buildMetricKey('host', props.resource.id));

  const cpuPercent = createMemo(() => (props.resource.cpu ? Math.round(getCpuPercent(props.resource)) : null));
  const memoryPercent = createMemo(() => (props.resource.memory ? Math.round(getMemoryPercent(props.resource)) : null));
  const diskPercent = createMemo(() => (props.resource.disk ? Math.round(getDiskPercent(props.resource)) : null));

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

      <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
          <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Metrics</div>
          <div class="space-y-2">
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">CPU</div>
              <Show when={cpuPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={cpuPercent() ?? 0}
                  label={formatPercent(cpuPercent() ?? 0)}
                  type="cpu"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">Memory</div>
              <Show when={memoryPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={memoryPercent() ?? 0}
                  label={formatPercent(memoryPercent() ?? 0)}
                  sublabel={metricSublabel(props.resource.memory)}
                  type="memory"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">Disk</div>
              <Show when={diskPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={diskPercent() ?? 0}
                  label={formatPercent(diskPercent() ?? 0)}
                  sublabel={metricSublabel(props.resource.disk)}
                  type="disk"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
          </div>
        </div>

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
  );
};

export default ResourceDetailDrawer;
