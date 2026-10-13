import { For, Show, Suspense, createMemo, lazy } from 'solid-js';

import { ALERTS_EMPTY_STATE } from '@/utils/alertOverviewPresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { HelpIcon } from '@/components/shared/HelpIcon';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import type { HistoryTimeRange } from '@/api/charts';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import { DrawerExceptionList } from './DrawerExceptionList';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import type { Alert, Disk, HostGPUSensor, Node, Temperature } from '@/types/api';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import { formatBytes, formatRelativeTime, normalizeDiskArray } from '@/utils/format';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getMemoryObservationPresentation } from '@/utils/memoryObservation';
import { getProxmoxUpdateEvidencePresentation } from '@/utils/proxmoxUpdateEvidence';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';

import { buildDrawerDiskListItems } from './DrawerDiskListCard';
import {
  collectDrawerExceptionRows,
  getDrawerChartAlerts,
  withoutDrawerExceptionRows,
} from './drawerOverviewModel';
import {
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  type GuestDrawerHistoryDeferredMetric,
  type GuestDrawerHistoryGroupConfig,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';

// Loaded once a node drawer opens, so the Proxmox table chunk does not carry it.
const GuestDrawerHistory = lazy(() =>
  import('./GuestDrawerHistory').then(({ GuestDrawerHistory }) => ({
    default: GuestDrawerHistory,
  })),
);

const finitePercent = (value: number | null | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0
    ? Math.min(100, value)
    : undefined;

interface NodeDrawerOverviewProps {
  node: Node;
  disks?: Disk[];
  temperatureThresholds?: MetricDisplayThresholds | null;
  alerts?: Alert[];
  historyTarget?: GuestDrawerHistoryTarget | null;
  historyRange?: HistoryTimeRange;
  onHistoryRangeChange?: (range: HistoryTimeRange) => void;
  historyGroups?: GuestDrawerHistoryGroupConfig[];
  /** Live readings the charts fall back to before any history is stored. */
  currentMetrics?: Record<string, number | undefined>;
  /** True while Overview is the visible tab; hidden charts do not poll. */
  historyEnabled?: boolean;
  /** The node's own alerts, without its guests', for the charts' alert lines. */
  metricAlerts?: Alert[];
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

const formatLoadAverage = (loadAverage: number[] | undefined): string => {
  const values = (loadAverage || []).filter((value) => Number.isFinite(value));
  if (values.length === 0) return '-';
  return values.map((value) => value.toFixed(2)).join(' / ');
};

const hasPositiveNumber = (value: number | null | undefined): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value > 0;

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
  typedGPUAvailable = false,
): NodeOverviewRow[] => {
  if (!temperature?.available) return [];

  const rows: NodeOverviewRow[] = [];
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

  if (!typedGPUAvailable && temperature.gpu?.length) {
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

  return rows;
};

const formatGPUValue = (gpu: HostGPUSensor): string => {
  const parts: string[] = [];
  const name = cleanText(gpu.name);
  if (name) parts.push(name);
  if (
    typeof gpu.temperatureCelsius === 'number' &&
    Number.isFinite(gpu.temperatureCelsius) &&
    gpu.temperatureCelsius > 0
  ) {
    parts.push(formatTemperature(gpu.temperatureCelsius));
  }
  if (
    typeof gpu.utilizationPercent === 'number' &&
    Number.isFinite(gpu.utilizationPercent) &&
    gpu.utilizationPercent >= 0
  ) {
    parts.push(`${Math.round(Math.min(100, gpu.utilizationPercent))}%`);
  }
  if (
    typeof gpu.memoryTotalBytes === 'number' &&
    Number.isFinite(gpu.memoryTotalBytes) &&
    gpu.memoryTotalBytes > 0
  ) {
    const used =
      typeof gpu.memoryUsedBytes === 'number' &&
      Number.isFinite(gpu.memoryUsedBytes) &&
      gpu.memoryUsedBytes >= 0
        ? gpu.memoryUsedBytes
        : 0;
    parts.push(`${formatBytes(used)} / ${formatBytes(gpu.memoryTotalBytes)}`);
  }
  return parts.join(' · ');
};

const getGPUTelemetryRows = (node: Node): NodeOverviewRow[] =>
  (node.sensors?.gpu ?? []).flatMap((gpu, index) => {
    const value = formatGPUValue(gpu);
    if (!value) return [];
    const id = cleanText(gpu.id);
    return [{ label: id ? `GPU ${id}` : `GPU ${index + 1}`, value, title: value }];
  });

const getDetailTone = (valueClass: string | undefined): DetailValueTone => {
  if (valueClass?.includes('red') || valueClass?.includes('rose')) return 'danger';
  if (valueClass?.includes('yellow') || valueClass?.includes('amber')) return 'warning';
  if (valueClass?.includes('green') || valueClass?.includes('emerald')) return 'success';
  if (valueClass?.includes('blue') || valueClass?.includes('cyan')) return 'accent';
  return 'default';
};

const toDetailRows = (rows: NodeOverviewRow[]): DetailRow[] =>
  compactDetailRows(
    rows.map((row) =>
      // Details is where a reader goes for the full value; wrap rather than
      // cut it off behind an ellipsis and a hover-only title.
      makeDetailRow(row.label, row.value, {
        title: row.title,
        tone: getDetailTone(row.valueClass),
        wrap: true,
      }),
    ),
  );

export function NodeDrawerOverview(props: NodeDrawerOverviewProps) {
  const now = useRelativeTimeNow();
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
  const updateEvidence = () =>
    getProxmoxUpdateEvidencePresentation(props.node, (value) =>
      formatRelativeTime(value, { now: now() }),
    );

  const platformRows = (): NodeOverviewRow[] => [
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
    ...(props.node.instance && props.node.instance !== clusterLabel()
      ? [{ label: 'Instance', value: props.node.instance } satisfies NodeOverviewRow]
      : []),
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
    { label: 'Total', value: formatBytes(props.node.memory?.total || 0) },
    ...(props.node.memory?.cache
      ? [
          {
            label: 'Reclaimable cache',
            value: formatBytes(props.node.memory.cache),
          } satisfies NodeOverviewRow,
        ]
      : []),
    ...(props.node.memory?.usageUnavailable === true
      ? []
      : [{ label: 'Free', value: formatBytes(props.node.memory?.free || 0) }]),
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
    { label: 'Root total', value: formatBytes(props.node.disk?.total || 0) },
    { label: 'Root free', value: formatBytes(props.node.disk?.free || 0) },
  ];

  const networkRows = (): NodeOverviewRow[] =>
    (props.node.networkInterfaces || []).reduce<NodeOverviewRow[]>((rows, networkInterface) => {
      const name = cleanText(networkInterface.name);
      if (!name) return rows;
      const addresses = (networkInterface.addresses || [])
        .map((address) => cleanText(address))
        .filter(Boolean);
      const mac = cleanText(networkInterface.mac);
      rows.push({
        label: name,
        value: addresses.join(' / ') || mac || 'Configured',
        title: [...addresses, ...(mac ? [mac] : [])].join(' · ') || undefined,
      });
      return rows;
    }, []);

  const telemetryRows = (): NodeOverviewRow[] => [
    {
      label: 'Temp monitor',
      value: formatTemperatureMonitoring(props.node.temperatureMonitoringEnabled),
    },
  ];

  const perDiskItems = () => {
    const disks = normalizeDiskArray(props.disks) ?? [];
    if (disks.length < 2) return [];
    return buildDrawerDiskListItems(disks);
  };

  const thermalRows = () =>
    getThermalRows(
      props.node.temperature,
      props.temperatureThresholds,
      (props.node.sensors?.gpu?.length ?? 0) > 0,
    );
  const isSensorRow = (row: NodeOverviewRow): boolean =>
    row.label === 'NVMe' || row.label === 'GPU';

  const technicalSections = () =>
    compactDetailSections([
      { label: 'Platform', rows: toDetailRows(platformRows()) },
      { label: 'Hardware', rows: toDetailRows(hardwareRows()) },
      { label: 'Memory', rows: toDetailRows(memoryRows()) },
      { label: 'Network', rows: toDetailRows(networkRows()) },
      {
        label: 'Storage',
        rows:
          perDiskItems().length > 0
            ? compactDetailRows(
                perDiskItems().map((disk) =>
                  makeDetailRow(disk.label, disk.usageText, {
                    title: disk.device ? `${disk.label} · ${disk.device}` : disk.label,
                    tone: disk.percent === null ? 'muted' : getDetailTone(disk.textClass),
                    wrap: true,
                  }),
                ),
              )
            : toDetailRows(storageRows()),
      },
      { label: 'Telemetry', rows: toDetailRows(telemetryRows()) },
      {
        label: 'Thermals',
        rows: toDetailRows(thermalRows().filter((row) => !isSensorRow(row))),
      },
    ]);

  // GPU and NVMe readings carry no heat judgement of their own, so nothing
  // would lift a hot one out of Details; they stay visible instead.
  const sensorSections = () =>
    compactDetailSections([
      {
        label: 'Sensors',
        rows: toDetailRows([
          ...thermalRows().filter(isSensorRow),
          ...getGPUTelemetryRows(props.node),
        ]),
      },
    ]);

  const updateAccessGuidance = (
    <Show when={updateEvidence()?.helpContentId}>
      <div class="space-y-1 text-left text-[11px] font-normal text-muted">
        <p class="whitespace-normal wrap-anywhere">
          Update checks use separate Proxmox access. Other monitoring can still work.
        </p>
        <HelpIcon
          contentId="updates.proxmox.access"
          triggerLabel="Why update checks are unavailable"
          size="xs"
          class="-ml-2"
        />
      </div>
    </Show>
  );

  // Inventory may move into Details, but denied-update guidance stays visible
  // and owned by the drawer so readings do not recreate an open help dialog.
  const overviewSections = () => {
    const primaryTemperature = props.node.cpuTemperature;
    const updates = updateEvidence();
    const temperatureClass =
      typeof primaryTemperature === 'number'
        ? getTemperatureTextClass(primaryTemperature, props.temperatureThresholds)
        : '';
    return compactDetailSections([
      {
        label: 'Summary',
        rows: compactDetailRows([
          makeDetailRow('Platform', versionLabel() ? `PVE ${versionLabel()}` : null),
          makeDetailRow(
            'Pulse coverage',
            linkedAgentId() ? `Agent ${stripAgentPrefix(linkedAgentId())}` : 'PVE API only',
            { wrap: true },
          ),
          updates
            ? makeDetailRow('Updates', updates.value, {
                tone: updates.tone,
                title: updates.title,
                wrap: true,
              })
            : null,
          typeof primaryTemperature === 'number' &&
          (temperatureClass.includes('yellow') || temperatureClass.includes('red'))
            ? makeDetailRow('Temperature', formatTemperature(primaryTemperature), {
                tone: temperatureClass.includes('red') ? 'danger' : 'warning',
              })
            : null,
        ]),
      },
    ]);
  };

  const exceptionRows = createMemo(() =>
    collectDrawerExceptionRows([...overviewSections(), ...technicalSections()]),
  );

  // The table withholds readings from an offline or stale node; the charts do
  // the same instead of presenting the last numbers as live.
  const liveReadingsWithheld = (): string | null => {
    const health = (props.node.connectionHealth || '').trim().toLowerCase();
    if (props.node.status === 'offline') return 'Node is offline. Showing its history only.';
    if (
      props.node.status === 'unknown' ||
      health === 'error' ||
      health === 'degraded' ||
      health === 'stale'
    ) {
      return 'Proxmox readings for this node are stale. Showing its history only.';
    }
    return null;
  };

  // The adapter fills missing usage with zero; a known total is the evidence
  // that a percentage was measured at all.
  const memoryValue = () => {
    const memory = props.node.memory;
    return memory?.usageUnavailable || !((memory?.total ?? 0) > 0)
      ? undefined
      : finitePercent(memory?.usage);
  };
  const diskMeasured = () => (props.node.disk?.total ?? 0) > 0;
  // A retained or undated memory reading is shown as last known, the same rule
  // the guest drawer follows, rather than as a live value.
  const deferredMetrics = createMemo(
    (): Record<string, GuestDrawerHistoryDeferredMetric | undefined> | undefined => {
      if (liveReadingsWithheld()) return undefined;
      const memoryReading = getMemoryObservationPresentation(
        memoryValue(),
        props.node.memory?.observation,
      );
      if (!memoryReading || memoryReading.state === 'current') return undefined;
      return {
        memory: {
          lastKnownValue: memoryReading.state === 'unavailable' ? undefined : memoryValue(),
          ...(memoryReading.state === 'unknown'
            ? { valueLabel: 'freshness unknown' as const }
            : {}),
          message: memoryReading.message,
        },
      };
    },
  );
  // Live readings the charts show before any history is stored. A withheld
  // node offers none, and deferred memory is qualified above instead.
  const liveMetrics = createMemo((): Record<string, number | undefined> | undefined => {
    if (liveReadingsWithheld()) return undefined;
    return {
      ...props.currentMetrics,
      cpu: finitePercent(props.node.cpu),
      memory: deferredMetrics()?.memory ? undefined : memoryValue(),
      disk: diskMeasured() ? finitePercent(props.node.disk?.usage) : undefined,
    };
  });
  const chartAlerts = createMemo(() =>
    getDrawerChartAlerts(
      props.metricAlerts,
      (props.historyGroups ?? HOST_METRICS_HISTORY_GROUPS).flatMap((group) =>
        group.series.map((series) => series.metric),
      ),
    ),
  );

  // The sizes the charts' percentages are of. A withheld node states only its
  // capacity, not a used amount that would read as live.
  const sizeFact = (total: number | undefined, used: number | undefined, label: string) => {
    if (!((total ?? 0) > 0)) return '';
    return !liveReadingsWithheld() && typeof used === 'number' && used >= 0
      ? `${formatBytes(used)} of ${formatBytes(total ?? 0)} ${label}`
      : `${formatBytes(total ?? 0)} ${label}`;
  };

  const facts = () => {
    const model = cleanText(props.node.cpuInfo?.model);
    const cores = props.node.cpuInfo?.cores;
    const firstAddress = (props.node.networkInterfaces || [])
      .flatMap((networkInterface) => networkInterface.addresses || [])
      .map((address) => cleanText(address))
      .find(Boolean);
    return [
      versionLabel() ? `Proxmox VE ${versionLabel()}` : '',
      [model, hasPositiveNumber(cores) ? `${cores} logical CPUs` : ''].filter(Boolean).join(' · '),
      sizeFact(
        props.node.memory?.total,
        memoryValue() !== undefined ? props.node.memory?.used : undefined,
        'RAM',
      ),
      diskMeasured() ? sizeFact(props.node.disk?.total, props.node.disk?.used, 'root disk') : '',
      firstAddress ?? '',
      clusterLabel() && clusterLabel() !== 'Standalone' ? `Cluster ${clusterLabel()}` : '',
    ].filter((fact) => fact.trim().length > 0);
  };

  return (
    <div class="space-y-3" data-testid="node-drawer-overview">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          ...getAlertAttentionCopy(alert, now),
          subject:
            cleanText(alert.resourceName) ||
            cleanText(alert.nodeDisplayName) ||
            cleanText(alert.node) ||
            props.node.name,
          metric: alertTypeDisplayLabel(alert.type),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <Show when={(props.alerts ?? []).length === 0}>
        <p data-testid="drawer-no-attention" class="text-xs font-medium text-base-content">
          {ALERTS_EMPTY_STATE}
        </p>
      </Show>

      <DrawerExceptionList testId="node-drawer-exceptions" rows={exceptionRows()} />

      <Show when={props.historyTarget}>
        <Show when={liveReadingsWithheld()}>
          {(message) => (
            <p data-testid="node-drawer-readings-withheld" class="text-xs text-muted">
              {message()}
            </p>
          )}
        </Show>
        <Suspense fallback={<div class="h-48" aria-hidden="true" />}>
          <GuestDrawerHistory
            target={props.historyTarget ?? null}
            range={props.historyRange ?? GUEST_DRAWER_HISTORY_DEFAULT_RANGE}
            onRangeChange={props.onHistoryRangeChange}
            enabled={props.historyEnabled ?? true}
            groups={props.historyGroups}
            currentMetrics={liveMetrics()}
            deferredMetrics={deferredMetrics()}
            alerts={chartAlerts()}
          />
        </Suspense>
      </Show>

      {updateAccessGuidance}

      <Show when={sensorSections().length > 0}>
        <TechnicalDetailsSection dataTestId="node-drawer-sensors" sections={sensorSections()} />
      </Show>

      <Show when={facts().length > 0}>
        <div
          data-testid="node-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>{(fact) => <span class="wrap-break-word">{fact}</span>}</For>
        </div>
      </Show>

      <TechnicalDetailsDisclosure
        dataTestId="node-technical-details"
        title="Details"
        keepMounted
        subtitle="platform, hardware, memory, network, storage and sensors"
        class="border-t border-border pt-2"
        contentClass="mt-2"
        sections={withoutDrawerExceptionRows([...overviewSections(), ...technicalSections()])}
      />
    </div>
  );
}
