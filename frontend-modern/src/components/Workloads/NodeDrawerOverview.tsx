import { TechnicalDetailsSection } from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import type { Alert, Disk, Node, Temperature } from '@/types/api';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { formatBytes, normalizeDiskArray } from '@/utils/format';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getProxmoxUpdateEvidencePresentation } from '@/utils/proxmoxUpdateEvidence';
import { formatTemperature, getCpuTemperature, getTemperatureTextClass } from '@/utils/temperature';

import { buildDrawerDiskListItems } from './DrawerDiskListCard';

interface NodeDrawerOverviewProps {
  node: Node;
  disks?: Disk[];
  temperatureThresholds?: MetricDisplayThresholds | null;
  alerts?: Alert[];
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

  return rows;
};

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
      makeDetailRow(row.label, row.value, {
        title: row.title,
        tone: getDetailTone(row.valueClass),
      }),
    ),
  );

export function NodeDrawerOverview(props: NodeDrawerOverviewProps) {
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
  const updateEvidence = () => getProxmoxUpdateEvidencePresentation(props.node);

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
        rows: toDetailRows(getThermalRows(props.node.temperature, props.temperatureThresholds)),
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
        label: 'Operator context',
        rows: compactDetailRows([
          makeDetailRow('Platform', versionLabel() ? `PVE ${versionLabel()}` : null),
          makeDetailRow(
            'Pulse coverage',
            linkedAgentId() ? `Agent ${stripAgentPrefix(linkedAgentId())}` : 'PVE API only',
          ),
          updates
            ? makeDetailRow('Updates', updates.value, {
                tone: updates.tone,
                title: updates.title,
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

  return (
    <div class="space-y-3">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          message: alert.message,
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
      <TechnicalDetailsSection
        dataTestId="node-technical-details"
        sections={[...overviewSections(), ...technicalSections()]}
      />
    </div>
  );
}
