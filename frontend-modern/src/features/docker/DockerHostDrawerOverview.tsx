import { Show, createMemo } from 'solid-js';

import {
  buildDrawerDiskListItems,
  type DrawerDiskListItem,
} from '@/components/Workloads/DrawerDiskListCard';
import { AvailabilityProbeStatusCards } from '@/components/Infrastructure/AvailabilityProbeStatusCard';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import { TechnicalDetailsSection } from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { useResourceDetailDrawerDockerActionsState } from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState';
import { hostOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { formatBytes, formatRelativeTime, formatSpeed, normalizeDiskArray } from '@/utils/format';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';

interface DockerHostDrawerOverviewProps {
  host: Resource;
}

interface DockerOverviewRow {
  label: string;
  value: string;
  valueClass?: string;
  title?: string;
}

type DockerHostCommandMeta = {
  type?: string;
  status?: string;
  message?: string;
  failureReason?: string;
};

const cleanText = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  return trimmed && trimmed.toLowerCase() !== 'unknown' ? trimmed : '';
};

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

const titleCase = (value: string): string =>
  value.length === 0 ? value : value.charAt(0).toUpperCase() + value.slice(1);

const getDetailTone = (valueClass: string | undefined): DetailValueTone => {
  if (valueClass?.includes('red') || valueClass?.includes('rose')) return 'danger';
  if (valueClass?.includes('yellow') || valueClass?.includes('amber')) return 'warning';
  if (valueClass?.includes('green') || valueClass?.includes('emerald')) return 'success';
  if (valueClass?.includes('blue') || valueClass?.includes('cyan')) return 'accent';
  return 'default';
};

const toDetailRows = (rows: DockerOverviewRow[]): DetailRow[] =>
  compactDetailRows(
    rows.map((row) =>
      makeDetailRow(row.label, row.value, {
        title: row.title,
        tone: getDetailTone(row.valueClass),
      }),
    ),
  );

export function DockerHostDrawerManagement(props: DockerHostDrawerOverviewProps) {
  const docker = () => props.host.docker;
  const hostSourceId = createMemo(() => cleanText(docker()?.hostSourceId) || null);
  const updatesAvailable = createMemo(() => docker()?.updatesAvailableCount ?? 0);
  const hostCommand = createMemo(() => docker()?.command as DockerHostCommandMeta | undefined);
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
  const checkedAt = () => cleanText(docker()?.updatesLastCheckedAt);
  const checkedAtMillis = createMemo(() => {
    const parsed = Date.parse(checkedAt());
    return Number.isFinite(parsed) ? parsed : null;
  });

  return (
    <Show when={hostSourceId()}>
      <InfoCardFrame data-testid="docker-host-management-actions" class="max-w-md">
        <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
          Container updates
        </h3>
        <div class="space-y-1.5 text-[11px]">
          <InfoCardKeyValueRow label="Available" value={updatesAvailable()} />
          <Show when={checkedAtMillis()} keyed>
            {(timestamp) => (
              <InfoCardKeyValueRow label="Last checked" value={formatRelativeTime(timestamp)} />
            )}
          </Show>
          <Show when={hostCommand()?.type || hostCommand()?.status}>
            <InfoCardKeyValueRow
              label={titleCase(cleanText(hostCommand()?.type).replace(/_/g, ' ') || 'Command')}
              value={titleCase(cleanText(hostCommand()?.status).replace(/_/g, ' ') || 'unknown')}
              valueClass={`truncate ${hostCommandActive() ? 'text-sky-700 dark:text-sky-300' : ''}`}
              valueTitle={hostCommand()?.failureReason || hostCommand()?.message || undefined}
            />
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
          <div class="flex flex-wrap items-center gap-2 border-t border-border pt-2">
            <button
              type="button"
              disabled={
                updateActions.dockerActionBusy() || updateActionsLoading() || hostCommandActive()
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
      </InfoCardFrame>
    </Show>
  );
}

export function DockerHostDrawerOverview(props: DockerHostDrawerOverviewProps) {
  const alertsActivation = useAlertsActivation();
  const docker = () => props.host.docker;
  const agent = () => props.host.agent;
  const linkedAgentId = () => cleanText(agent()?.agentId);
  const temperatureThresholds = createMemo(() =>
    alertsActivation.getMetricThresholds(
      'node',
      'temperature',
      hostOverrideIdCandidates(props.host),
    ),
  );

  const runtimeLabel = (): string => {
    const runtime = cleanText(docker()?.runtime) || 'Docker';
    const version = cleanText(docker()?.runtimeVersion) || cleanText(docker()?.dockerVersion);
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

  const memorySource = () => props.host.memory ?? agent()?.memory ?? docker()?.memory;
  const diskSource = () => props.host.disk;

  const runtimeRows = (): DockerOverviewRow[] => [
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

  const memoryRows = (): DockerOverviewRow[] => {
    const memory = memorySource();
    if (!memory) return [];
    const rows: DockerOverviewRow[] = [];
    if ('usageUnavailable' in memory && memory.usageUnavailable === true) {
      rows.push({ label: 'Usage', value: 'Unavailable', valueClass: 'text-muted' });
      if (typeof memory.total === 'number' && memory.total > 0) {
        rows.push({ label: 'Total', value: formatBytes(memory.total) });
      }
    } else if (typeof memory.total === 'number' && memory.total > 0) {
      rows.push({ label: 'Total', value: formatBytes(memory.total) });
      if (typeof memory.free === 'number') {
        rows.push({ label: 'Free', value: formatBytes(memory.free) });
      }
    }
    return rows;
  };

  const storageRows = (): DockerOverviewRow[] => {
    const disk = diskSource();
    if (!disk) return [];
    const rows: DockerOverviewRow[] = [];
    if (typeof disk.total === 'number' && disk.total > 0) {
      rows.push({ label: 'Total', value: formatBytes(disk.total) });
      if (typeof disk.free === 'number') {
        rows.push({ label: 'Free', value: formatBytes(disk.free) });
      }
    }
    return rows;
  };

  const telemetryRows = (): DockerOverviewRow[] => {
    const rows: DockerOverviewRow[] = [];
    const temperature = props.host.temperature ?? docker()?.temperature;
    if (typeof temperature === 'number' && temperature > 0) {
      rows.push({
        label: 'CPU temp',
        value: formatTemperature(temperature),
        valueClass: getTemperatureTextClass(temperature, temperatureThresholds()),
      });
    }
    if (
      typeof props.host.network?.rxBytes === 'number' ||
      typeof props.host.network?.txBytes === 'number'
    ) {
      rows.push({
        label: 'Network I/O',
        value: `${formatSpeed(props.host.network?.rxBytes ?? 0)} / ${formatSpeed(props.host.network?.txBytes ?? 0)}`,
      });
    }
    if (
      typeof props.host.diskIO?.readRate === 'number' ||
      typeof props.host.diskIO?.writeRate === 'number'
    ) {
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

  const technicalSections = () =>
    compactDetailSections([
      { label: 'Runtime', rows: toDetailRows(runtimeRows()) },
      { label: 'Memory', rows: toDetailRows(memoryRows()) },
      {
        label: 'Storage',
        rows:
          perDiskItems().length > 0
            ? compactDetailRows(
                perDiskItems().map((disk) =>
                  makeDetailRow(
                    disk.label,
                    `${Math.round(disk.percent)}% · ${formatBytes(disk.used)} / ${formatBytes(
                      disk.total,
                    )}`,
                    {
                      title: disk.device ? `${disk.label} · ${disk.device}` : disk.label,
                      tone: getDetailTone(disk.textClass),
                    },
                  ),
                ),
              )
            : toDetailRows(storageRows()),
      },
      { label: 'Telemetry', rows: toDetailRows(telemetryRows()) },
    ]);

  const overviewSections = () => {
    const swarm = cleanText(docker()?.swarm?.localState);
    return compactDetailSections([
      {
        label: 'Operator context',
        rows: compactDetailRows([
          makeDetailRow('Runtime', runtimeLabel()),
          makeDetailRow('System', osLabel()),
          makeDetailRow(
            'Pulse coverage',
            linkedAgentId() ? `Agent ${stripAgentPrefix(linkedAgentId())}` : 'Direct API',
          ),
          typeof docker()?.updatesAvailableCount === 'number' &&
          (docker()?.updatesAvailableCount ?? 0) > 0
            ? makeDetailRow('Updates', `${docker()?.updatesAvailableCount} available`, {
                tone: 'warning',
              })
            : null,
          makeDetailRow('Swarm', swarm ? titleCase(swarm) : null),
        ]),
      },
    ]);
  };

  return (
    <div class="space-y-3">
      <DrawerAttentionSection
        items={(props.host.alerts ?? []).map((alert) => ({
          id: alert.id,
          message: alert.message,
          severity: alert.level,
        }))}
      />
      <Show when={props.host.availability || props.host.availabilityChecks?.length}>
        <div class="max-w-sm">
          <AvailabilityProbeStatusCards
            availability={props.host.availability}
            checks={props.host.availabilityChecks}
          />
        </div>
      </Show>
      <TechnicalDetailsSection
        dataTestId="docker-host-technical-details"
        sections={[...overviewSections(), ...technicalSections()]}
      />
    </div>
  );
}
