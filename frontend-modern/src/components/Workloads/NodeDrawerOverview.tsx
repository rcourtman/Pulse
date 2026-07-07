import { For, Show } from 'solid-js';

import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import type { Disk, Node, Temperature } from '@/types/api';
import {
  formatBytes,
  formatRelativeTime,
  formatSpeed,
  formatUptime,
  normalizeDiskArray,
} from '@/utils/format';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getNodeDisplayName } from '@/utils/nodes';
import { formatTemperature, getCpuTemperature, getTemperatureTextClass } from '@/utils/temperature';

import { DrawerDiskListCard, buildDrawerDiskListItems } from './DrawerDiskListCard';

interface NodeDrawerOverviewProps {
  node: Node;
  disks?: Disk[];
  temperatureThresholds?: MetricDisplayThresholds | null;
}

interface NodeOverviewRow {
  label: string;
  value: string;
  valueClass?: string;
  title?: string;
}

const cleanText = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  return trimmed && trimmed.toLowerCase() !== 'unknown' ? trimmed : '';
};

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

const getNodeVersionLabel = (node: Node): string => {
  const version = cleanText(node.pveVersion);
  if (!version) return '';
  return (
    version.match(/pve-manager\/([^/\s]+)/i)?.[1] || version.match(/\d+(?:\.\d+)+/)?.[0] || version
  );
};

const formatStatus = (value: string | null | undefined): string => {
  const status = cleanText(value);
  if (!status) return '-';
  return status.charAt(0).toUpperCase() + status.slice(1);
};

const formatPercent = (value: number | null | undefined): string => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
  const normalized = value <= 1 ? value * 100 : value;
  return `${Math.round(Math.max(0, normalized))}%`;
};

const getUsedPercent = (used?: number | null, total?: number | null): string => {
  if (typeof used !== 'number' || typeof total !== 'number' || total <= 0) return '-';
  return formatPercent((used / total) * 100);
};

const formatLoadAverage = (loadAverage: number[] | undefined): string => {
  const values = (loadAverage || []).filter((value) => Number.isFinite(value));
  if (values.length === 0) return '-';
  return values.map((value) => value.toFixed(2)).join(' / ');
};

const hasPositiveNumber = (value: number | null | undefined): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value > 0;

const formatLastSeen = (value: string | null | undefined): NodeOverviewRow | null => {
  const lastSeen = cleanText(value);
  if (!lastSeen) return null;
  const parsed = new Date(lastSeen);
  return {
    label: 'Last seen',
    value: formatRelativeTime(lastSeen, { compact: true }) || '-',
    title: Number.isNaN(parsed.getTime()) ? lastSeen : parsed.toLocaleString(),
  };
};

const formatTemperatureMonitoring = (value: boolean | null | undefined): string => {
  if (value === true) return 'Enabled';
  if (value === false) return 'Disabled';
  return 'Inherited';
};

const pushTemperature = (
  rows: NodeOverviewRow[],
  label: string,
  value: number | null | undefined,
  thresholds: MetricDisplayThresholds | null | undefined,
): void => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return;
  rows.push({
    label,
    value: formatTemperature(value),
    valueClass: getTemperatureTextClass(value, thresholds),
  });
};

const getThermalRows = (
  temperature: Temperature | undefined,
  thresholds: MetricDisplayThresholds | null | undefined,
): NodeOverviewRow[] => {
  if (!temperature?.available) return [];

  const rows: NodeOverviewRow[] = [];
  const primary = getCpuTemperature(temperature);

  pushTemperature(rows, 'CPU current', primary, thresholds);
  pushTemperature(rows, 'CPU package', temperature.cpuPackage, thresholds);
  pushTemperature(rows, 'CPU low', temperature.cpuMin, thresholds);
  pushTemperature(rows, 'CPU record', temperature.cpuMaxRecord, thresholds);

  if (temperature.nvme?.length) {
    rows.push({
      label: 'NVMe',
      value: temperature.nvme
        .slice(0, 2)
        .map((drive) => `${drive.device} ${formatTemperature(drive.temp)}`)
        .join(' / '),
      title: temperature.nvme
        .map((drive) => `${drive.device} ${formatTemperature(drive.temp)}`)
        .join(' / '),
    });
  }

  if (temperature.gpu?.length) {
    const gpuLabels = temperature.gpu
      .map((gpu) => {
        const value = [gpu.edge, gpu.junction, gpu.mem].find(hasPositiveNumber);
        return value ? `${gpu.device} ${formatTemperature(value)}` : '';
      })
      .filter(Boolean);

    if (gpuLabels.length > 0) {
      rows.push({
        label: 'GPU',
        value: gpuLabels.slice(0, 2).join(' / '),
        title: gpuLabels.join(' / '),
      });
    }
  }

  const lastUpdate = formatLastSeen(temperature.lastUpdate);
  if (lastUpdate) rows.push({ ...lastUpdate, label: 'Updated' });

  return rows;
};

const DetailCard = (props: { title: string; rows: NodeOverviewRow[] }) => (
  <Show when={props.rows.length > 0}>
    <InfoCardFrame>
      <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
        {props.title}
      </h3>
      <div class="space-y-1.5 text-[11px]">
        <For each={props.rows}>
          {(row) => (
            <div class="flex items-center justify-between gap-2 min-w-0">
              <span class="shrink-0 text-muted">{row.label}</span>
              <span
                class={`truncate text-right font-medium ${row.valueClass ?? 'text-base-content'}`}
                title={row.title ?? row.value}
              >
                {row.value}
              </span>
            </div>
          )}
        </For>
      </div>
    </InfoCardFrame>
  </Show>
);

export function NodeDrawerOverview(props: NodeDrawerOverviewProps) {
  const displayName = () => getNodeDisplayName(props.node);
  const versionLabel = () => getNodeVersionLabel(props.node);
  const linkedAgentId = () => cleanText(props.node.linkedAgentId);
  const clusterLabel = () =>
    cleanText(props.node.clusterName) ||
    (props.node.isClusterMember ? props.node.instance || 'Member' : 'Standalone');
  const clockLabel = () => {
    const clock = cleanText(props.node.cpuInfo?.mhz);
    return clock && clock !== '0' ? clock : '';
  };
  const loadAverageLabel = () => formatLoadAverage(props.node.loadAverage);

  const systemRows = (): NodeOverviewRow[] => [
    { label: 'Name', value: displayName(), title: props.node.name },
    { label: 'Host', value: cleanText(props.node.host) || '-', title: props.node.host },
    { label: 'Status', value: formatStatus(props.node.status) },
    ...(props.node.uptime > 0
      ? [{ label: 'Uptime', value: formatUptime(props.node.uptime) } satisfies NodeOverviewRow]
      : []),
    ...(formatLastSeen(props.node.lastSeen) ? [formatLastSeen(props.node.lastSeen)!] : []),
  ];

  const platformRows = (): NodeOverviewRow[] => [
    ...(versionLabel()
      ? [
          {
            label: 'PVE',
            value: `PVE ${versionLabel()}`,
            title: props.node.pveVersion,
          } satisfies NodeOverviewRow,
        ]
      : []),
    ...(cleanText(props.node.kernelVersion)
      ? [
          {
            label: 'Kernel',
            value: cleanText(props.node.kernelVersion),
            title: props.node.kernelVersion,
          } satisfies NodeOverviewRow,
        ]
      : []),
    {
      label: 'Cluster',
      value: clusterLabel(),
    },
    { label: 'Instance', value: props.node.instance || '-' },
  ];

  const hardwareRows = (): NodeOverviewRow[] => [
    ...(cleanText(props.node.cpuInfo?.model)
      ? [
          {
            label: 'CPU model',
            value: cleanText(props.node.cpuInfo?.model),
            title: props.node.cpuInfo?.model,
          } satisfies NodeOverviewRow,
        ]
      : []),
    ...(hasPositiveNumber(props.node.cpuInfo?.cores)
      ? [{ label: 'Cores', value: `${props.node.cpuInfo.cores}` } satisfies NodeOverviewRow]
      : []),
    ...(hasPositiveNumber(props.node.cpuInfo?.sockets)
      ? [{ label: 'Sockets', value: `${props.node.cpuInfo.sockets}` } satisfies NodeOverviewRow]
      : []),
    ...(clockLabel() ? [{ label: 'Clock', value: clockLabel() } satisfies NodeOverviewRow] : []),
    ...(loadAverageLabel() !== '-'
      ? [{ label: 'Load avg', value: loadAverageLabel() } satisfies NodeOverviewRow]
      : []),
  ];

  const memoryRows = (): NodeOverviewRow[] => [
    {
      label: 'Usage',
      value: `${getUsedPercent(props.node.memory?.used, props.node.memory?.total)} · ${formatBytes(
        props.node.memory?.used || 0,
      )}`,
    },
    { label: 'Total', value: formatBytes(props.node.memory?.total || 0) },
    ...(props.node.memory?.cache
      ? [
          {
            label: 'Reclaimable cache',
            value: formatBytes(props.node.memory.cache),
          } satisfies NodeOverviewRow,
        ]
      : []),
    { label: 'Free', value: formatBytes(props.node.memory?.free || 0) },
    ...(props.node.memory?.swapTotal
      ? [
          {
            label: 'Swap',
            value: `${formatBytes(props.node.memory.swapUsed || 0)} / ${formatBytes(
              props.node.memory.swapTotal,
            )}`,
          } satisfies NodeOverviewRow,
        ]
      : []),
  ];

  const storageRows = (): NodeOverviewRow[] => [
    {
      label: 'Root usage',
      value: `${formatPercent(props.node.disk?.usage)} · ${formatBytes(props.node.disk?.used || 0)}`,
    },
    { label: 'Root total', value: formatBytes(props.node.disk?.total || 0) },
    { label: 'Root free', value: formatBytes(props.node.disk?.free || 0) },
    {
      label: 'Disk I/O',
      value: `${formatSpeed(props.node.diskRead ?? 0)} / ${formatSpeed(props.node.diskWrite ?? 0)}`,
    },
  ];

  const telemetryRows = (): NodeOverviewRow[] => [
    {
      label: 'Connection',
      value: formatStatus(props.node.connectionHealth || props.node.status),
    },
    {
      label: 'Agent',
      value: linkedAgentId() ? stripAgentPrefix(linkedAgentId()) : 'PVE API only',
      title: linkedAgentId() || undefined,
    },
    {
      label: 'Temp monitor',
      value: formatTemperatureMonitoring(props.node.temperatureMonitoringEnabled),
    },
    ...(typeof props.node.pendingUpdates === 'number'
      ? [{ label: 'Updates', value: `${props.node.pendingUpdates}` } satisfies NodeOverviewRow]
      : []),
    {
      label: 'Network I/O',
      value: `${formatSpeed(props.node.networkIn ?? 0)} / ${formatSpeed(props.node.networkOut ?? 0)}`,
    },
  ];

  const perDiskItems = () => {
    const disks = normalizeDiskArray(props.disks) ?? [];
    if (disks.length < 2) return [];
    return buildDrawerDiskListItems(disks);
  };

  return (
    <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
      <DetailCard title="System" rows={systemRows()} />
      <DetailCard title="Platform" rows={platformRows()} />
      <DetailCard title="Hardware" rows={hardwareRows()} />
      <DetailCard title="Memory" rows={memoryRows()} />
      <Show
        when={perDiskItems().length > 0}
        fallback={<DetailCard title="Storage" rows={storageRows()} />}
      >
        <DrawerDiskListCard disks={perDiskItems()} testId="node-drawer-disks" />
      </Show>
      <DetailCard title="Telemetry" rows={telemetryRows()} />
      <DetailCard
        title="Thermals"
        rows={getThermalRows(props.node.temperature, props.temperatureThresholds)}
      />
    </div>
  );
}
