import { For, Show, Suspense, createMemo, lazy } from 'solid-js';

import { ALERTS_EMPTY_STATE } from '@/utils/alertOverviewPresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
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
import { formatTemperature, getCpuTemperature, getTemperatureTextClass } from '@/utils/temperature';

import { buildDrawerDiskListItems } from './DrawerDiskListCard';
import type { DrawerTrendCardConfig } from './DrawerTrendCards';
import {
  collectDrawerExceptionRows,
  getDrawerMetricAlert,
  withoutDrawerExceptionRows,
} from './drawerOverviewModel';
import type {
  GuestDrawerHistoryDeferredMetric,
  GuestDrawerHistoryTarget,
} from './guestDrawerModel';

// Loaded once a node drawer opens, so the Proxmox table chunk does not carry it.
const DrawerTrendCards = lazy(() => import('./DrawerTrendCards'));

const hostSeriesColor = (metric: string): string =>
  HOST_METRICS_HISTORY_GROUPS.flatMap((group) => group.series).find(
    (series) => series.metric === metric,
  )?.color ?? 'currentColor';

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
  /** True while Overview is the visible tab; hidden trends do not poll. */
  trendsEnabled?: boolean;
  /** The node's own alerts, without its guests', for the trend cards. */
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

  const overviewSections = () => {
    const primaryTemperature = getCpuTemperature(props.node.temperature);
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

  // The table withholds readings from an offline or stale node; the trend
  // cards do the same instead of presenting the last numbers as live.
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

  const trendCards = (): DrawerTrendCardConfig[] => {
    const card = (
      metric: string,
      label: string,
      unit: DrawerTrendCardConfig['unit'],
      current: number | undefined,
      detail?: string,
      deferred?: GuestDrawerHistoryDeferredMetric,
    ): DrawerTrendCardConfig => {
      const alert = getDrawerMetricAlert(props.metricAlerts, metric);
      const withheld = liveReadingsWithheld();
      return {
        metric,
        label,
        unit,
        color: hostSeriesColor(metric),
        current: withheld ? undefined : current,
        deferred: withheld ? { message: withheld } : deferred,
        detail: withheld ? undefined : detail,
        alertSeverity: alert?.severity,
        alertThreshold: alert?.threshold,
      };
    };
    const memory = props.node.memory;
    const disk = props.node.disk;
    // The adapter fills missing usage with zero; a known total is the
    // evidence that a percentage was measured at all.
    const memoryValue =
      memory?.usageUnavailable || !((memory?.total ?? 0) > 0)
        ? undefined
        : finitePercent(memory?.usage);
    // A retained or undated reading is shown as last known, the same rule the
    // guest cards follow, rather than as a live value.
    const memoryReading = getMemoryObservationPresentation(memoryValue, memory?.observation);
    const memoryDeferred: GuestDrawerHistoryDeferredMetric | undefined =
      memoryReading && memoryReading.state !== 'current'
        ? {
            lastKnownValue: memoryReading.state === 'unavailable' ? undefined : memoryValue,
            ...(memoryReading.state === 'unknown'
              ? { valueLabel: 'freshness unknown' as const }
              : {}),
            message: memoryReading.message,
          }
        : undefined;
    const memoryCurrent = memoryDeferred ? undefined : memoryValue;
    const diskMeasured = (disk?.total ?? 0) > 0;
    const temperature = getCpuTemperature(props.node.temperature);
    return [
      card(
        'cpu',
        'CPU',
        '%',
        finitePercent(props.node.cpu),
        loadAverageLabel() !== '-' ? `load ${props.node.loadAverage?.[0]?.toFixed(2)}` : undefined,
      ),
      card(
        'memory',
        'Memory',
        '%',
        memoryCurrent,
        (memoryCurrent ?? memoryDeferred?.lastKnownValue) !== undefined
          ? `${formatBytes(memory.used ?? 0)} of ${formatBytes(memory.total)}`
          : undefined,
        memoryDeferred,
      ),
      ...(typeof temperature === 'number'
        ? [card('temperature', 'CPU temperature', 'C', temperature)]
        : []),
      card(
        'disk',
        'Root disk',
        '%',
        diskMeasured ? finitePercent(disk?.usage) : undefined,
        diskMeasured
          ? `${formatBytes(disk?.used ?? 0)} of ${formatBytes(disk?.total ?? 0)}`
          : undefined,
      ),
    ];
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
      (props.node.memory?.total ?? 0) > 0 ? `${formatBytes(props.node.memory.total)} RAM` : '',
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

      <Show when={props.historyTarget}>
        <Suspense fallback={<div class="h-28" aria-hidden="true" />}>
          <DrawerTrendCards
            target={props.historyTarget ?? null}
            enabled={props.trendsEnabled ?? true}
            cards={trendCards()}
          />
        </Suspense>
      </Show>

      <Show when={exceptionRows().length > 0}>
        <TechnicalDetailsSection
          dataTestId="node-drawer-exceptions"
          sections={[{ label: 'Worth checking', rows: exceptionRows() }]}
        />
      </Show>

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
