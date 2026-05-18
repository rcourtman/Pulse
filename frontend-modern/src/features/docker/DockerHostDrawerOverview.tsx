import { For, Show } from 'solid-js';

import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import { formatBytes, formatRelativeTime, formatSpeed, formatUptime } from '@/utils/format';
import { normalizeDiskArray } from '@/utils/format';
import { getMetricColorRgba, getMetricTextColorClass } from '@/utils/metricThresholds';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';

import { hasDockerSwarmEvidence } from './dockerPageModel';

interface DockerHostDrawerOverviewProps {
  host: Resource;
}

interface DockerOverviewRow {
  label: string;
  value: string;
  valueClass?: string;
  title?: string;
}

const DOCKER_OVERVIEW_CARD_CLASS = 'rounded border border-border bg-surface p-3 shadow-sm';

const cleanText = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  return trimmed && trimmed.toLowerCase() !== 'unknown' ? trimmed : '';
};

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

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

const getNumericField = (value: unknown, field: string): number | undefined => {
  if (!value || typeof value !== 'object') return undefined;
  const fieldValue = (value as Record<string, unknown>)[field];
  return typeof fieldValue === 'number' ? fieldValue : undefined;
};

const formatLastSeenRow = (
  value: string | number | null | undefined,
): DockerOverviewRow | null => {
  if (value == null) return null;
  const parsed = typeof value === 'number' ? new Date(value) : new Date(String(value));
  if (Number.isNaN(parsed.getTime())) return null;
  const iso = parsed.toISOString();
  return {
    label: 'Last seen',
    value: formatRelativeTime(iso, { compact: true }) || '-',
    title: parsed.toLocaleString(),
  };
};

const titleCase = (value: string): string =>
  value.length === 0 ? value : value.charAt(0).toUpperCase() + value.slice(1);

interface DiskListItem {
  key: string;
  label: string;
  device: string;
  percent: number;
  used: number;
  total: number;
  color: string;
  textClass: string;
}

const getDiskUsagePercent = (disk: Disk): number => {
  if (disk.total > 0 && Number.isFinite(disk.used)) {
    return (disk.used / disk.total) * 100;
  }
  if (Number.isFinite(disk.usage)) {
    return disk.usage <= 1 ? disk.usage * 100 : disk.usage;
  }
  return 0;
};

const getDiskLabel = (disk: Disk, index: number): string =>
  disk.mountpoint || disk.device || `Disk ${index + 1}`;

const buildDiskListItems = (disks: Disk[]): DiskListItem[] =>
  disks.map((disk, index) => {
    const percent = getDiskUsagePercent(disk);
    return {
      key: `${disk.device ?? ''}|${disk.mountpoint ?? ''}|${index}`,
      label: getDiskLabel(disk, index),
      device: disk.device ?? '',
      percent,
      used: disk.used ?? 0,
      total: disk.total ?? 0,
      color: getMetricColorRgba(percent, 'disk'),
      textClass: getMetricTextColorClass(percent, 'disk'),
    };
  });

const DiskListCard = (props: { disks: DiskListItem[] }) => (
  <div
    class={`${DOCKER_OVERVIEW_CARD_CLASS} basis-[calc(50%-0.75rem)] grow-[2]`}
    data-testid="docker-host-drawer-disks"
  >
    <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
      Storage
    </h3>
    <div class="space-y-2 text-[11px]">
      <For each={props.disks}>
        {(disk) => (
          <div class="space-y-1">
            <div class="flex items-baseline justify-between gap-2 min-w-0">
              <span
                class="truncate font-medium text-base-content"
                title={disk.device ? `${disk.label} · ${disk.device}` : disk.label}
              >
                {disk.label}
              </span>
              <span class={`shrink-0 tabular-nums ${disk.textClass}`}>
                {`${Math.round(disk.percent)}%`}
                <span class="ml-1 text-muted">
                  ({formatBytes(disk.used)}
                  <Show when={disk.total > 0}> / {formatBytes(disk.total)}</Show>)
                </span>
              </span>
            </div>
            <div class="relative h-1.5 w-full overflow-hidden rounded bg-surface-hover">
              <div
                class="absolute inset-y-0 left-0 rounded"
                style={{
                  width: `${Math.max(0, Math.min(100, disk.percent))}%`,
                  background: disk.color,
                }}
              />
            </div>
          </div>
        )}
      </For>
    </div>
  </div>
);

const DetailCard = (props: { title: string; rows: DockerOverviewRow[] }) => (
  <Show when={props.rows.length > 0}>
    <div class={DOCKER_OVERVIEW_CARD_CLASS}>
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
    </div>
  </Show>
);

export function DockerHostDrawerOverview(props: DockerHostDrawerOverviewProps) {
  const docker = () => props.host.docker;
  const agent = () => props.host.agent;
  const linkedAgentId = () => cleanText(agent()?.agentId);

  const runtimeLabel = (): string => {
    const runtime = cleanText(docker()?.runtime) || 'Docker';
    const version =
      cleanText(docker()?.runtimeVersion) || cleanText(docker()?.dockerVersion);
    if (!version) return titleCase(runtime);
    return `${titleCase(runtime)} ${version}`;
  };

  const osLabel = (): string => {
    const dockerOs = cleanText(docker()?.os);
    const agentOs =
      cleanText(agent()?.osName) && cleanText(agent()?.osVersion)
        ? `${cleanText(agent()?.osName)} ${cleanText(agent()?.osVersion)}`
        : cleanText(agent()?.osName) || cleanText(agent()?.osVersion);
    return dockerOs || agentOs || '';
  };

  const uptimeSeconds = (): number => {
    if (typeof props.host.uptime === 'number' && props.host.uptime > 0) return props.host.uptime;
    if (typeof docker()?.uptimeSeconds === 'number' && (docker()?.uptimeSeconds ?? 0) > 0) {
      return docker()!.uptimeSeconds!;
    }
    return 0;
  };

  const memorySource = () => props.host.memory ?? agent()?.memory;
  const diskSource = () => props.host.disk;

  const systemRows = (): DockerOverviewRow[] => [
    {
      label: 'Name',
      value: cleanText(props.host.name) || props.host.id,
      title: props.host.name,
    },
    ...(cleanText(docker()?.hostname)
      ? [
          {
            label: 'Hostname',
            value: cleanText(docker()?.hostname),
            title: docker()?.hostname,
          } satisfies DockerOverviewRow,
        ]
      : []),
    { label: 'Status', value: formatStatus(props.host.status) },
    ...(uptimeSeconds() > 0
      ? [{ label: 'Uptime', value: formatUptime(uptimeSeconds()) } satisfies DockerOverviewRow]
      : []),
    ...(formatLastSeenRow(props.host.lastSeen) ? [formatLastSeenRow(props.host.lastSeen)!] : []),
  ];

  const runtimeRows = (): DockerOverviewRow[] => [
    { label: 'Engine', value: runtimeLabel(), title: docker()?.runtimeVersion },
    ...(osLabel()
      ? [{ label: 'OS', value: osLabel(), title: osLabel() } satisfies DockerOverviewRow]
      : []),
    ...(cleanText(docker()?.kernelVersion) || cleanText(agent()?.kernelVersion)
      ? [
          {
            label: 'Kernel',
            value: cleanText(docker()?.kernelVersion) || cleanText(agent()?.kernelVersion),
            title: docker()?.kernelVersion || agent()?.kernelVersion,
          } satisfies DockerOverviewRow,
        ]
      : []),
    ...(cleanText(docker()?.architecture)
      ? [
          {
            label: 'Arch',
            value: cleanText(docker()?.architecture),
          } satisfies DockerOverviewRow,
        ]
      : []),
    ...(cleanText(docker()?.agentVersion)
      ? [
          {
            label: 'Agent',
            value: cleanText(docker()?.agentVersion),
          } satisfies DockerOverviewRow,
        ]
      : []),
  ];

  const containerRows = (): DockerOverviewRow[] => {
    const count = docker()?.containerCount;
    const rows: DockerOverviewRow[] = [];
    if (typeof count === 'number') {
      rows.push({ label: 'Containers', value: `${count}` });
    }
    if (typeof docker()?.updatesAvailableCount === 'number') {
      rows.push({ label: 'Updates', value: `${docker()!.updatesAvailableCount}` });
    }
    if (hasDockerSwarmEvidence(props.host)) {
      const role = cleanText(docker()?.swarm?.nodeRole);
      if (role) rows.push({ label: 'Swarm role', value: titleCase(role) });
      const state = cleanText(docker()?.swarm?.localState);
      if (state) rows.push({ label: 'Swarm state', value: titleCase(state) });
      const cluster = cleanText(docker()?.swarm?.clusterName);
      if (cluster) rows.push({ label: 'Swarm cluster', value: cluster });
    }
    return rows;
  };

  const memoryRows = (): DockerOverviewRow[] => {
    const memory = memorySource();
    if (!memory) return [];
    const rows: DockerOverviewRow[] = [];
    if (typeof memory.total === 'number' && memory.total > 0) {
      rows.push({
        label: 'Usage',
        value: `${getUsedPercent(memory.used, memory.total)} · ${formatBytes(memory.used || 0)}`,
      });
      rows.push({ label: 'Total', value: formatBytes(memory.total) });
      if (typeof memory.free === 'number') {
        rows.push({ label: 'Free', value: formatBytes(memory.free) });
      }
    } else if (typeof getNumericField(memory, 'current') === 'number') {
      rows.push({ label: 'Usage', value: formatPercent(getNumericField(memory, 'current')) });
    } else if (typeof getNumericField(memory, 'usage') === 'number') {
      rows.push({ label: 'Usage', value: formatPercent(getNumericField(memory, 'usage')) });
    }
    return rows;
  };

  const storageRows = (): DockerOverviewRow[] => {
    const disk = diskSource();
    if (!disk) return [];
    const rows: DockerOverviewRow[] = [];
    if (typeof disk.total === 'number' && disk.total > 0) {
      rows.push({
        label: 'Usage',
        value: `${formatPercent(disk.current)} · ${formatBytes(disk.used || 0)}`,
      });
      rows.push({ label: 'Total', value: formatBytes(disk.total) });
      if (typeof disk.free === 'number') {
        rows.push({ label: 'Free', value: formatBytes(disk.free) });
      }
    } else if (typeof disk.current === 'number') {
      rows.push({ label: 'Usage', value: formatPercent(disk.current) });
    }
    return rows;
  };

  const telemetryRows = (): DockerOverviewRow[] => {
    const rows: DockerOverviewRow[] = [];
    rows.push({
      label: 'Connection',
      value: formatStatus(props.host.status),
    });
    rows.push({
      label: 'Agent ID',
      value: linkedAgentId() ? stripAgentPrefix(linkedAgentId()) : 'Direct API',
      title: linkedAgentId() || undefined,
    });
    const temperature = props.host.temperature ?? docker()?.temperature;
    if (typeof temperature === 'number' && temperature > 0) {
      rows.push({
        label: 'CPU temp',
        value: formatTemperature(temperature),
        valueClass: getTemperatureTextClass(temperature),
      });
    }
    if (typeof props.host.network?.rxBytes === 'number' || typeof props.host.network?.txBytes === 'number') {
      rows.push({
        label: 'Network I/O',
        value: `${formatSpeed(props.host.network?.rxBytes ?? 0)} / ${formatSpeed(props.host.network?.txBytes ?? 0)}`,
      });
    }
    if (typeof props.host.diskIO?.readRate === 'number' || typeof props.host.diskIO?.writeRate === 'number') {
      rows.push({
        label: 'Disk I/O',
        value: `${formatSpeed(props.host.diskIO?.readRate ?? 0)} / ${formatSpeed(props.host.diskIO?.writeRate ?? 0)}`,
      });
    }
    return rows;
  };

  const perDiskItems = (): DiskListItem[] => {
    const disks = normalizeDiskArray(agent()?.disks) ?? [];
    if (disks.length < 2) return [];
    return buildDiskListItems(disks);
  };

  return (
    <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
      <DetailCard title="System" rows={systemRows()} />
      <DetailCard title="Runtime" rows={runtimeRows()} />
      <DetailCard title="Containers" rows={containerRows()} />
      <DetailCard title="Memory" rows={memoryRows()} />
      <Show
        when={perDiskItems().length > 0}
        fallback={<DetailCard title="Storage" rows={storageRows()} />}
      >
        <DiskListCard disks={perDiskItems()} />
      </Show>
      <DetailCard title="Telemetry" rows={telemetryRows()} />
    </div>
  );
}
