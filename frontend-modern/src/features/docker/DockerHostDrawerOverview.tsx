import { For, Show, createMemo } from 'solid-js';

import {
  DrawerDiskListCard,
  buildDrawerDiskListItems,
  type DrawerDiskListItem,
} from '@/components/Workloads/DrawerDiskListCard';
import { useResourceDetailDrawerDockerActionsState } from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import type { Resource } from '@/types/resource';
import {
  formatBytes,
  formatRelativeTime,
  formatSpeed,
  formatUptime,
  normalizeDiskArray,
} from '@/utils/format';
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
    const updates = docker()?.updatesAvailableCount;
    if (typeof updates === 'number') {
      rows.push({
        label: 'Updates',
        value: `${updates}`,
        valueClass: updates > 0 ? 'text-sky-700 dark:text-sky-300' : undefined,
      });
    }
    const checkedAt = cleanText(docker()?.updatesLastCheckedAt);
    if (checkedAt) {
      const parsed = Date.parse(checkedAt);
      if (Number.isFinite(parsed)) {
        rows.push({
          label: 'Checked',
          value: formatRelativeTime(parsed),
          title: new Date(parsed).toLocaleString(),
        });
      }
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

  type DockerHostCommandMeta = {
    type?: string;
    status?: string;
    message?: string;
    failureReason?: string;
  };

  const hostSourceId = createMemo(() => cleanText(docker()?.hostSourceId) || null);
  const updatesAvailable = createMemo(() => docker()?.updatesAvailableCount ?? 0);
  const hostCommand = createMemo(
    () => docker()?.command as DockerHostCommandMeta | undefined,
  );
  const hostCommandActive = createMemo(() =>
    ['queued', 'dispatched', 'acknowledged', 'in_progress'].includes(
      cleanText(hostCommand()?.status).toLowerCase(),
    ),
  );
  const updateActions = useResourceDetailDrawerDockerActionsState({
    dockerHostSourceId: hostSourceId,
    dockerUpdatesAvailable: updatesAvailable,
  });
  const updateActionsLoading = () => !areSystemSettingsLoaded();
  const updateAllHidden = () => shouldHideDockerUpdateActions();

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

  const perDiskItems = (): DrawerDiskListItem[] => {
    const disks = normalizeDiskArray(agent()?.disks) ?? [];
    if (disks.length < 2) return [];
    return buildDrawerDiskListItems(disks);
  };

  return (
    <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
      <DetailCard title="System" rows={systemRows()} />
      <DetailCard title="Runtime" rows={runtimeRows()} />
      <div class={DOCKER_OVERVIEW_CARD_CLASS} data-testid="docker-host-drawer-containers-card">
        <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
          Containers
        </h3>
        <div class="space-y-1.5 text-[11px]">
          <For each={containerRows()}>
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
          <Show when={hostSourceId()}>
            <div class="space-y-1.5 border-t border-border pt-2">
              <Show when={hostCommand()?.type || hostCommand()?.status}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted">
                    {titleCase(cleanText(hostCommand()?.type).replace(/_/g, ' ') || 'Command')}
                  </span>
                  <span
                    class={`truncate text-right font-medium ${
                      hostCommandActive()
                        ? 'text-sky-700 dark:text-sky-300'
                        : 'text-base-content'
                    }`}
                    title={hostCommand()?.failureReason || hostCommand()?.message || undefined}
                  >
                    {titleCase(cleanText(hostCommand()?.status).replace(/_/g, ' ') || 'unknown')}
                  </span>
                </div>
              </Show>
              <Show when={updateActions.dockerActionError()}>
                <div class="rounded border border-red-200 bg-red-50 px-2 py-1.5 text-[10px] text-red-700 dark:border-red-700 dark:bg-red-900 dark:text-red-200">
                  {updateActions.dockerActionError()}
                </div>
              </Show>
              <Show when={updateActions.dockerActionNote()}>
                <div class="rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px] text-base-content">
                  {updateActions.dockerActionNote()}
                </div>
              </Show>
              <div class="flex flex-wrap items-center gap-2 pt-0.5">
                <button
                  type="button"
                  disabled={
                    updateActions.dockerActionBusy() ||
                    updateActionsLoading() ||
                    hostCommandActive()
                  }
                  onClick={() => void updateActions.queueDockerUpdateCheck()}
                  class="rounded-md border border-border bg-surface px-2.5 py-1 text-[11px] font-semibold text-base-content hover:bg-surface-hover disabled:opacity-60"
                  title={updateActionsLoading() ? 'Loading settings...' : undefined}
                >
                  Check updates
                </button>
                <Show when={!updateAllHidden()}>
                  <button
                    type="button"
                    disabled={
                      updateActions.dockerActionBusy() ||
                      updateActionsLoading() ||
                      hostCommandActive() ||
                      updatesAvailable() <= 0
                    }
                    onClick={() => void updateActions.queueDockerUpdateAll()}
                    class="rounded-md border border-sky-200 bg-sky-600 px-2.5 py-1 text-[11px] font-semibold text-white hover:bg-sky-700 disabled:opacity-60 disabled:hover:bg-sky-600 dark:border-sky-700 dark:bg-sky-600 dark:hover:bg-sky-500 dark:disabled:hover:bg-sky-600"
                  >
                    {updateActions.confirmUpdateAll()
                      ? 'Confirm update'
                      : `Update all${updatesAvailable() > 0 ? ` (${updatesAvailable()})` : ''}`}
                  </button>
                </Show>
              </div>
            </div>
          </Show>
        </div>
      </div>
      <DetailCard title="Memory" rows={memoryRows()} />
      <Show
        when={perDiskItems().length > 0}
        fallback={<DetailCard title="Storage" rows={storageRows()} />}
      >
        <DrawerDiskListCard disks={perDiskItems()} testId="docker-host-drawer-disks" />
      </Show>
      <DetailCard title="Telemetry" rows={telemetryRows()} />
    </div>
  );
}
