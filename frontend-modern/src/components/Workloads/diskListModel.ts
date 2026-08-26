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

export const hasWorkloadsDiskCapacity = (disk: Disk): boolean =>
  typeof disk.total === 'number' && disk.total > 0;

// The poller reports usage -1 for mounts it can only see in the container
// config (capacity may be known, live usage is not).
export const isWorkloadsDiskUsageUnknown = (disk: Disk): boolean =>
  typeof disk.usage === 'number' && disk.usage < 0;

export const getWorkloadsDiskUsagePercent = (disk: Disk): number => {
  const total = disk.total ?? 0;
  if (total <= 0) {
    return 0;
  }

  return ((disk.used ?? 0) / total) * 100;
};

export const getWorkloadsDiskLabel = (disk: Disk): string =>
  disk.mountpoint || disk.device || 'Unknown';

export const getWorkloadsDiskLabelTitle = (label: string): string | undefined =>
  label !== 'Unknown' ? label : undefined;

export const getWorkloadsDiskUsageText = (disk: Disk): string => {
  if (isWorkloadsDiskUsageUnknown(disk)) {
    return hasWorkloadsDiskCapacity(disk)
      ? `?/${formatBytes(disk.total ?? 0)}`
      : 'Usage unavailable';
  }
  return hasWorkloadsDiskCapacity(disk)
    ? `${formatBytes(disk.used ?? 0)}/${formatBytes(disk.total ?? 0)}`
    : 'Usage unavailable';
};

export const getWorkloadsDiskUsagePercentLabel = (disk: Disk): string =>
  hasWorkloadsDiskCapacity(disk) && !isWorkloadsDiskUsageUnknown(disk)
    ? `${getWorkloadsDiskUsagePercent(disk).toFixed(0)}%`
    : '—';

export const getWorkloadsDiskProgressClass = (
  disk: Disk,
  thresholds?: MetricDisplayThresholds | null,
): string => getMetricColorClass(getWorkloadsDiskUsagePercent(disk), 'disk', thresholds);

export const getWorkloadsDiskProgressValue = (disk: Disk): number | null =>
  hasWorkloadsDiskCapacity(disk) && !isWorkloadsDiskUsageUnknown(disk)
    ? getWorkloadsDiskUsagePercent(disk)
    : null;

export const getWorkloadsDiskProgressWidth = (disk: Disk): string =>
  isWorkloadsDiskUsageUnknown(disk)
    ? '0%'
    : `${Math.min(getWorkloadsDiskUsagePercent(disk), 100)}%`;

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
