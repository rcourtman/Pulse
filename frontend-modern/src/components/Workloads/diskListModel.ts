import type { Disk } from '@/types/api';

import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

export interface DiskListProps {
  disks: Disk[];
  diskStatusReason?: string;
  thresholds?: MetricDisplayThresholds | null;
}

export interface WorkloadsDiskPresentation {
  key: string;
  label: string;
  labelTitle?: string;
  progressClass: string;
  progressValue: number | null;
  progressWidth: string;
  typeLabel: string;
  usageText: string;
  usagePercentLabel: string;
}

export const hasWorkloadsDiskCapacity = (disk: Disk): disk is Disk & { total: number } =>
  typeof disk.total === 'number' && Number.isFinite(disk.total) && disk.total > 0;

// The poller reports usage -1 for mounts it can only see in the container
// config (capacity may be known, live usage is not).
export const isWorkloadsDiskUsageUnknown = (disk: Disk): boolean =>
  typeof disk.used !== 'number' ||
  !Number.isFinite(disk.used) ||
  disk.used < 0 ||
  (typeof disk.usage === 'number' && (!Number.isFinite(disk.usage) || disk.usage < 0));

// Capacity alone is not a measurement. In particular, an omitted `used`
// field must not become a healthy empty filesystem, even if usage/free exist.
export const getWorkloadsDiskUsagePercent = (disk: Disk): number | null => {
  const { total, used } = disk;
  if (
    !hasWorkloadsDiskCapacity(disk) ||
    isWorkloadsDiskUsageUnknown(disk) ||
    typeof total !== 'number' ||
    typeof used !== 'number'
  ) {
    return null;
  }

  const percent = (used / total) * 100;
  return Number.isFinite(percent) ? percent : null;
};

export const getWorkloadsDiskLabel = (disk: Disk): string =>
  disk.mountpoint || disk.device || 'Unknown';

export const getWorkloadsDiskLabelTitle = (label: string): string | undefined =>
  label !== 'Unknown' ? label : undefined;

export const getWorkloadsDiskUsageText = (disk: Disk): string => {
  if (!hasWorkloadsDiskCapacity(disk)) return 'Usage unavailable';
  const used =
    getWorkloadsDiskUsagePercent(disk) === null || typeof disk.used !== 'number'
      ? '?'
      : formatBytes(disk.used);
  return `${used}/${formatBytes(disk.total)}`;
};

export const getWorkloadsDiskUsagePercentLabel = (disk: Disk): string => {
  const percent = getWorkloadsDiskUsagePercent(disk);
  return percent === null ? '—' : `${percent.toFixed(0)}%`;
};

export const getWorkloadsDiskProgressClass = (
  disk: Disk,
  thresholds?: MetricDisplayThresholds | null,
): string => {
  const percent = getWorkloadsDiskUsagePercent(disk);
  return percent === null ? 'bg-surface-hover' : getMetricColorClass(percent, 'disk', thresholds);
};

export const getWorkloadsDiskProgressValue = (disk: Disk): number | null =>
  getWorkloadsDiskUsagePercent(disk);

export const getWorkloadsDiskProgressWidth = (disk: Disk): string => {
  const percent = getWorkloadsDiskUsagePercent(disk);
  return percent === null ? '0%' : `${Math.min(percent, 100)}%`;
};

export const getWorkloadsDiskTypeLabel = (disk: Disk): string => disk.type?.toUpperCase() ?? '';

export const buildWorkloadsDiskPresentation = (
  disk: Disk,
  index: number,
  thresholds?: MetricDisplayThresholds | null,
): WorkloadsDiskPresentation => {
  const label = getWorkloadsDiskLabel(disk);

  return {
    key: `${disk.mountpoint ?? ''}:${disk.device ?? ''}:${index}`,
    label,
    labelTitle: getWorkloadsDiskLabelTitle(label),
    progressClass: getWorkloadsDiskProgressClass(disk, thresholds),
    progressValue: getWorkloadsDiskProgressValue(disk),
    progressWidth: getWorkloadsDiskProgressWidth(disk),
    typeLabel: getWorkloadsDiskTypeLabel(disk),
    usageText: getWorkloadsDiskUsageText(disk),
    usagePercentLabel: getWorkloadsDiskUsagePercentLabel(disk),
  };
};
