import {
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskTemperaturePresentation,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import type { HistoryTimeRange } from '@/api/charts';
import { formatPowerOnHours } from '@/utils/format';
import { getMetricSeverity, type MetricDisplayThresholds } from '@/utils/metricThresholds';
import type { StatusIndicatorVariant } from '@/utils/status';

export function getDiskAttributeValueTextClass(ok: boolean): string {
  return ok ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400';
}

export function getDiskAttributeCardValueTextClass(card: DiskDetailAttributeCard): string {
  return card.lastKnown ? 'text-muted' : getDiskAttributeValueTextClass(card.ok);
}

export function getLinkedDiskHealthDotVariant(hasIssue: boolean): StatusIndicatorVariant {
  return hasIssue ? 'warning' : 'success';
}

export function getLinkedDiskTemperatureTextClass(
  tempCelsius: number,
  thresholds?: MetricDisplayThresholds | null,
): string {
  if (!Number.isFinite(tempCelsius) || tempCelsius <= 0) {
    return 'text-muted';
  }
  const severity = getMetricSeverity(tempCelsius, 'diskTemperature', thresholds);
  if (severity === 'critical') {
    return 'text-red-500';
  }
  if (severity === 'warning') {
    return 'text-yellow-500';
  }
  return 'text-muted';
}

export type DiskDetailAttributeCard = {
  label: string;
  value: string;
  ok: boolean;
  /** A retained value that is not a current reading. */
  lastKnown?: boolean;
};

export type DiskDetailChartOption = {
  value: HistoryTimeRange;
  label: string;
};

export type DiskDetailLiveChartConfig = {
  label: string;
  unit: string;
  metric: 'disk';
  series: 'read' | 'write' | 'io';
};

export type DiskDetailHistoryChartConfig = {
  metric:
    'smart_temp' | 'smart_reallocated_sectors' | 'smart_percentage_used' | 'smart_available_spare';
  label: string;
  unit: string;
  color: string;
};

export const DISK_DETAIL_HISTORY_RANGE_OPTIONS: readonly DiskDetailChartOption[] = [
  { value: '1h', label: 'Last 1 hour' },
  { value: '6h', label: 'Last 6 hours' },
  { value: '12h', label: 'Last 12 hours' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
  { value: '14d', label: 'Last 14 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
] as const;

export const DISK_DETAIL_LIVE_CHARTS: readonly DiskDetailLiveChartConfig[] = [
  { label: 'Read', unit: 'B/s', metric: 'disk', series: 'read' },
  { label: 'Write', unit: 'B/s', metric: 'disk', series: 'write' },
  { label: 'Busy', unit: '%', metric: 'disk', series: 'io' },
] as const;

export const getDiskDetailLiveBadgeLabel = (): string => 'Real-time';

export const getDiskDetailHistoryFallbackMessage = (): string =>
  'Historical disk charts are unavailable until Pulse can resolve a stable identity for this disk.';

export type DiskDetailHealthPresentation = {
  label: string;
  summary: string;
  tone: string;
};

// The drawer header carries the disk's verdict and its reason. The table row
// shows only the verdict word, so this is where the reason is read in full.
export const DISK_DETAIL_HEADER_STACK_CLASS = 'min-w-0 space-y-1';
export const DISK_DETAIL_HEALTH_ROW_CLASS =
  'flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-[11px]';
export const DISK_DETAIL_HEALTH_LABEL_CLASS = 'font-semibold';
export const DISK_DETAIL_HEALTH_SUMMARY_CLASS =
  'min-w-0 max-w-full whitespace-normal wrap-break-word text-muted';

export function getDiskDetailHealthPresentation(
  disk: PhysicalDiskPresentationData,
): DiskDetailHealthPresentation {
  const status = getPhysicalDiskHealthStatus(disk);
  return {
    label: status.label,
    summary: getPhysicalDiskHealthSummary(status),
    tone: status.tone,
  };
}

export function getDiskDetailAttributeCards(
  disk: PhysicalDiskPresentationData,
  diskTempThresholds?: MetricDisplayThresholds | null,
): DiskDetailAttributeCard[] {
  // Temperature is reported independently of optional extended SMART attributes.
  const attrs = disk.smartAttributes ?? {};

  const cards: DiskDetailAttributeCard[] = [];
  const isNvme = disk.type?.toLowerCase() === 'nvme';

  if (attrs.powerOnHours != null) {
    cards.push({
      label: 'Power-On Time',
      value: formatPowerOnHours(attrs.powerOnHours),
      ok: true,
    });
  }

  const temperature = getPhysicalDiskTemperaturePresentation(disk);
  if (temperature?.current) {
    cards.push({
      label: 'Temperature',
      value: temperature.label,
      ok: getMetricSeverity(disk.temperature, 'diskTemperature', diskTempThresholds) !== 'critical',
    });
  } else if (temperature) {
    // A retained reading is neither healthy nor hot now. The collection
    // message above the cards says why it is not current.
    cards.push({
      label: 'Last known temperature',
      value: temperature.label,
      ok: true,
      lastKnown: true,
    });
  }

  if (attrs.powerCycles != null) {
    cards.push({
      label: 'Power Cycles',
      value: attrs.powerCycles.toLocaleString(),
      ok: true,
    });
  }

  if (!isNvme && attrs.reallocatedSectors != null) {
    cards.push({
      label: 'Reallocated Sectors',
      value: attrs.reallocatedSectors.toString(),
      ok: attrs.reallocatedSectors === 0,
    });
  }
  if (!isNvme && attrs.pendingSectors != null) {
    cards.push({
      label: 'Pending Sectors',
      value: attrs.pendingSectors.toString(),
      ok: attrs.pendingSectors === 0,
    });
  }
  if (!isNvme && attrs.offlineUncorrectable != null) {
    cards.push({
      label: 'Offline Uncorrectable',
      value: attrs.offlineUncorrectable.toString(),
      ok: attrs.offlineUncorrectable === 0,
    });
  }
  if (!isNvme && attrs.udmaCrcErrors != null) {
    cards.push({
      label: 'CRC Errors',
      value: attrs.udmaCrcErrors.toString(),
      ok: attrs.udmaCrcErrors === 0,
    });
  }

  if (isNvme && attrs.percentageUsed != null) {
    cards.push({
      label: 'Life Used',
      value: `${attrs.percentageUsed}%`,
      ok: attrs.percentageUsed <= 90,
    });
  }
  if (isNvme && attrs.availableSpare != null) {
    cards.push({
      label: 'Available Spare',
      value: `${attrs.availableSpare}%`,
      ok: attrs.availableSpare >= 20,
    });
  }
  if (isNvme && attrs.mediaErrors != null) {
    cards.push({
      label: 'Media Errors',
      value: attrs.mediaErrors.toString(),
      ok: attrs.mediaErrors === 0,
    });
  }
  if (isNvme && attrs.unsafeShutdowns != null) {
    cards.push({
      label: 'Unsafe Shutdowns',
      value: attrs.unsafeShutdowns.toLocaleString(),
      ok: true,
    });
  }

  return cards;
}

const DISK_TEMPERATURE_HISTORY_CHART: DiskDetailHistoryChartConfig = {
  metric: 'smart_temp',
  label: 'Temperature',
  unit: 'C',
  color: '#ef4444',
};

const ATA_DISK_HISTORY_CHARTS: readonly DiskDetailHistoryChartConfig[] = [
  DISK_TEMPERATURE_HISTORY_CHART,
  {
    metric: 'smart_reallocated_sectors',
    label: 'Reallocated Sectors',
    unit: 'sectors',
    color: '#f59e0b',
  },
];

const NVME_DISK_HISTORY_CHARTS: readonly DiskDetailHistoryChartConfig[] = [
  DISK_TEMPERATURE_HISTORY_CHART,
  {
    metric: 'smart_percentage_used',
    label: 'Life Used',
    unit: '%',
    color: '#f59e0b',
  },
  {
    metric: 'smart_available_spare',
    label: 'Available Spare',
    unit: '%',
    color: '#10b981',
  },
];

export function getDiskDetailHistoryCharts(
  disk: PhysicalDiskPresentationData,
): readonly DiskDetailHistoryChartConfig[] {
  // Missing current collection does not erase stored observations. Ask for the
  // existing disk-family catalog and let the store return samples or an honest
  // empty series; never synthesize a value from the current snapshot.
  // Stable entries also keep Solid's reference-keyed For from remounting charts
  // and restarting History reads on every ordinary disk snapshot replacement.
  return disk.type?.toLowerCase() === 'nvme' ? NVME_DISK_HISTORY_CHARTS : ATA_DISK_HISTORY_CHARTS;
}
