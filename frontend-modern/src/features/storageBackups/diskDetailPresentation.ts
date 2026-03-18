import type { PhysicalDiskPresentationData } from '@/features/storageBackups/diskPresentation';
import type { HistoryTimeRange } from '@/api/charts';
import { formatPowerOnHours } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';

export function getDiskAttributeValueTextClass(ok: boolean): string {
  return ok ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400';
}

export function getLinkedDiskHealthDotClass(hasIssue: boolean): string {
  return hasIssue ? 'h-2 w-2 rounded-full bg-yellow-500' : 'h-2 w-2 rounded-full bg-green-500';
}

export function getLinkedDiskTemperatureTextClass(tempCelsius: number): string {
  if (!Number.isFinite(tempCelsius) || tempCelsius <= 0) {
    return 'text-muted';
  }
  if (tempCelsius > 60) {
    return 'text-red-500';
  }
  if (tempCelsius > 50) {
    return 'text-yellow-500';
  }
  return 'text-muted';
}

export type DiskDetailAttributeCard = {
  label: string;
  value: string;
  ok: boolean;
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
  metric: 'smart_temp' | 'smart_reallocated_sectors' | 'smart_percentage_used' | 'smart_available_spare';
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
  'Install the Pulse agent for detailed SMART monitoring and historical charts.';

export function getDiskDetailAttributeCards(
  disk: PhysicalDiskPresentationData,
): DiskDetailAttributeCard[] {
  const attrs = disk.smartAttributes;
  if (!attrs) return [];

  const cards: DiskDetailAttributeCard[] = [];
  const isNvme = disk.type?.toLowerCase() === 'nvme';

  if (attrs.powerOnHours != null) {
    cards.push({
      label: 'Power-On Time',
      value: formatPowerOnHours(attrs.powerOnHours),
      ok: true,
    });
  }

  if (disk.temperature > 0) {
    cards.push({
      label: 'Temperature',
      value: formatTemperature(disk.temperature),
      ok: disk.temperature <= 60,
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

export function getDiskDetailHistoryCharts(
  disk: PhysicalDiskPresentationData,
): DiskDetailHistoryChartConfig[] {
  const attrs = disk.smartAttributes;
  const charts: DiskDetailHistoryChartConfig[] = [];
  const isNvme = disk.type?.toLowerCase() === 'nvme';

  if (disk.temperature > 0) {
    charts.push({
      metric: 'smart_temp',
      label: 'Temperature',
      unit: 'C',
      color: '#ef4444',
    });
  }

  if (!isNvme && attrs?.reallocatedSectors != null) {
    charts.push({
      metric: 'smart_reallocated_sectors',
      label: 'Reallocated Sectors',
      unit: 'sectors',
      color: '#f59e0b',
    });
  }

  if (isNvme && attrs?.percentageUsed != null) {
    charts.push({
      metric: 'smart_percentage_used',
      label: 'Life Used',
      unit: '%',
      color: '#f59e0b',
    });
  }

  if (isNvme && attrs?.availableSpare != null) {
    charts.push({
      metric: 'smart_available_spare',
      label: 'Available Spare',
      unit: '%',
      color: '#10b981',
    });
  }

  return charts;
}
