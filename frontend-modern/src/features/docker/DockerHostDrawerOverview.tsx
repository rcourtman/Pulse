import { For, Show, createMemo } from 'solid-js';

import {
  buildDrawerDiskListItems,
  type DrawerDiskListItem,
} from '@/components/Workloads/DrawerDiskListCard';
import { AvailabilityProbeStatusCards } from '@/components/Infrastructure/AvailabilityProbeStatusCard';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import { TechnicalDetailsDisclosure } from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerExceptionList } from '@/components/Workloads/DrawerExceptionList';
import { ALERTS_EMPTY_STATE } from '@/utils/alertOverviewPresentation';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import {
  DetailSectionTable,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { useResourceDetailDrawerDockerActionsState } from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState';
import { useWebSocket } from '@/contexts/appRuntime';
import { hostOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { getAlertsForUnifiedResource, getOwnAlertsForUnifiedResource } from '@/utils/alerts';
import {
  formatBytes,
  formatRelativeTime,
  formatSpeed,
  formatObservedSpeed,
  normalizeDiskArray,
} from '@/utils/format';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { HistoryTimeRange } from '@/api/charts';
import { GuestDrawerHistory } from '@/components/Workloads/GuestDrawerHistory';
import {
  getAlertStatedRowLabels,
  getDrawerChartAlerts,
  isDrawerAvailabilityHealthy,
  partitionDrawerExceptionRows,
} from '@/components/Workloads/drawerOverviewModel';
import type {
  GuestDrawerHistoryGroupConfig,
  GuestDrawerHistoryTarget,
} from '@/components/Workloads/guestDrawerModel';

interface DockerHostDrawerOverviewProps {
  host: Resource;
  historyTarget?: GuestDrawerHistoryTarget | null;
  historyRange?: HistoryTimeRange;
  onHistoryRangeChange?: (range: HistoryTimeRange) => void;
  historyGroups?: GuestDrawerHistoryGroupConfig[];
  /** Live readings the charts show before any history is stored. */
  currentMetrics?: Record<string, number | undefined>;
  /** True while Overview is the visible tab; hidden charts do not poll. */
  historyEnabled?: boolean;
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
  const now = useRelativeTimeNow();
  const docker = () => props.host.docker;
  const hostSourceId = createMemo(() => cleanText(docker()?.hostSourceId) || null);
  const summaryOnly = createMemo(
    () => cleanText(docker()?.collectionMode) === 'typed-helper-summary',
  );
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
    <Show when={hostSourceId() && !summaryOnly()}>
      <InfoCardFrame data-testid="docker-host-management-actions" class="max-w-md">
        <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
          Container updates
        </h3>
        <div class="space-y-1.5 text-[11px]">
          <InfoCardKeyValueRow label="Available" value={updatesAvailable()} />
          <Show when={checkedAtMillis()} keyed>
            {(timestamp) => (
              <InfoCardKeyValueRow
                label="Last checked"
                value={formatRelativeTime(timestamp, { now: now() })}
              />
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
            <div class="rounded-sm border border-red-200 bg-red-50 px-2 py-1.5 text-[10px] text-red-700 dark:border-red-700 dark:bg-red-900/25 dark:text-red-200">
              {updateActions.dockerActionError()}
            </div>
          </Show>
          <Show when={updateActions.dockerActionNote()}>
            <div class="rounded-sm border border-border bg-surface-hover px-2 py-1.5 text-[10px] text-base-content">
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
  const now = useRelativeTimeNow();
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const docker = () => props.host.docker;
  const agent = () => props.host.agent;
  const linkedAgentId = () => cleanText(agent()?.agentId);
  const summaryOnly = () => cleanText(docker()?.collectionMode) === 'typed-helper-summary';
  const hostAlerts = () =>
    getAlertsForUnifiedResource(props.host, activeAlerts, alertsActivation.detectionEnabled());
  // Needs attention lists the host's containers and services too; the charts
  // describe the host itself, so only its own alerts draw a trigger.
  const chartAlerts = () =>
    getDrawerChartAlerts(
      getOwnAlertsForUnifiedResource(props.host, activeAlerts, alertsActivation.detectionEnabled()),
      (props.historyGroups ?? []).flatMap((group) => group.series.map((series) => series.metric)),
    );
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
        value: `${formatObservedSpeed(props.host.diskIO?.readRate)} / ${formatObservedSpeed(props.host.diskIO?.writeRate)}`,
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
          // A host that is not reporting says so in front of the charts.
          props.host.status === 'offline'
            ? makeDetailRow(
                'Observed state',
                props.host.lastSeen
                  ? `Offline · last seen ${formatRelativeTime(props.host.lastSeen, { now: now() })}`
                  : 'Offline',
                { tone: 'danger' },
              )
            : null,
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

  // Inventory follows the Proxmox drawers: anything wrong is lifted in front,
  // one facts line adds what the row does not say, and the rest waits under a
  // closed Details section rather than as five equal-weight cards.
  const inventorySections = () => [...overviewSections(), ...technicalSections()];
  const inventory = () =>
    partitionDrawerExceptionRows(
      inventorySections(),
      getAlertStatedRowLabels(
        getOwnAlertsForUnifiedResource(
          props.host,
          activeAlerts,
          alertsActivation.detectionEnabled(),
        ),
      ),
    );
  const exceptionRows = () => inventory().lifted;
  const detailSections = () => inventory().details;
  const sizeFact = (
    source: { total?: number; used?: number; free?: number } | undefined,
    label: string,
  ): string => {
    const total = source?.total ?? 0;
    if (!(total > 0)) return '';
    if (source && 'usageUnavailable' in source && source.usageUnavailable === true) {
      return `${formatBytes(total)} ${label}`;
    }
    const used =
      typeof source?.used === 'number'
        ? source.used
        : typeof source?.free === 'number'
          ? total - source.free
          : undefined;
    const observation = (source as { observation?: { state?: string } } | undefined)?.observation;
    const current = !observation || observation.state === 'current';
    const reporting = props.host.status === 'online' || props.host.status === 'running';
    return reporting && current && typeof used === 'number' && used >= 0
      ? `${formatBytes(used)} of ${formatBytes(total)} ${label}`
      : `${formatBytes(total)} ${label}`;
  };
  const facts = () => {
    const system = osLabel();
    // The row already states the engine version; the facts add what it lacks.
    return [
      // A bare "linux" from the engine adds nothing; a named OS does.
      system && system.toLowerCase() !== 'linux' ? system : '',
      sizeFact(memorySource(), 'RAM'),
      sizeFact(diskSource(), 'disk'),
    ].filter(Boolean);
  };
  const availabilityChecks = () => [
    ...(props.host.availability ? [props.host.availability] : []),
    ...(props.host.availabilityChecks ?? []),
  ];
  const availabilityHealthy = () =>
    availabilityChecks().length > 0 &&
    availabilityChecks().every((check) => isDrawerAvailabilityHealthy(check, now()));
  const attentionItems = () => [
    ...(summaryOnly()
      ? [
          {
            id: `${props.host.id}:typed-helper-summary`,
            subject: 'Reduced container coverage',
            message:
              'The typed helper reports container summaries only. Stats, secondary inventory, update checks, and lifecycle actions are unavailable.',
            severity: 'warning' as const,
          },
        ]
      : []),
    ...hostAlerts().map((alert) => ({
      id: alert.id,
      ...getAlertAttentionCopy(alert, now),
      subject: cleanText(alert.resourceName) || props.host.displayName || props.host.name,
      metric: alertTypeDisplayLabel(alert.type),
      severity: alert.level,
      acknowledged: alert.acknowledged,
    })),
  ];

  return (
    <div class="space-y-3">
      <DrawerAttentionSection items={attentionItems()} />
      <Show when={attentionItems().length === 0}>
        <p data-testid="drawer-no-attention" class="text-xs text-muted">
          <span class="font-medium text-base-content">{ALERTS_EMPTY_STATE}</span>
          <Show when={availabilityHealthy()}>
            <span> · availability checks passing</span>
          </Show>
        </p>
      </Show>
      <DrawerExceptionList testId="docker-host-drawer-exceptions" rows={exceptionRows()} />
      <Show when={props.historyTarget}>
        {(target) => (
          <GuestDrawerHistory
            target={target()}
            range={props.historyRange ?? '24h'}
            onRangeChange={props.onHistoryRangeChange}
            enabled={props.historyEnabled ?? true}
            groups={props.historyGroups}
            currentMetrics={props.currentMetrics}
            alerts={chartAlerts()}
          />
        )}
      </Show>
      {/* A failing check stays in front, after the charts as in every drawer. */}
      <Show when={availabilityChecks().length > 0 && !availabilityHealthy()}>
        <div class="max-w-sm">
          <AvailabilityProbeStatusCards
            availability={props.host.availability}
            checks={props.host.availabilityChecks}
          />
        </div>
      </Show>
      <Show when={facts().length > 0}>
        <div
          data-testid="docker-host-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>{(fact) => <span class="wrap-break-word">{fact}</span>}</For>
        </div>
      </Show>
      <Show when={detailSections().length > 0 || availabilityHealthy()}>
        <TechnicalDetailsDisclosure
          dataTestId="docker-host-technical-details"
          title="Details"
          keepMounted
          subtitle={[
            ...detailSections().map((section) => section.label.toLowerCase()),
            availabilityHealthy() ? 'availability checks' : '',
          ]
            .filter(Boolean)
            .join(', ')}
          class="border-t border-border pt-2"
          contentClass="mt-2 space-y-3"
        >
          <DetailSectionTable
            sections={detailSections()}
            dataTestId="docker-host-technical-sections"
          />
          <Show when={availabilityHealthy()}>
            <div class="max-w-sm">
              <AvailabilityProbeStatusCards
                availability={props.host.availability}
                checks={props.host.availabilityChecks}
              />
            </div>
          </Show>
        </TechnicalDetailsDisclosure>
      </Show>
    </div>
  );
}
