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
  progressWidth: string;
  typeLabel: string;
  usageText: string;
  usagePercentLabel: string;
}

export const hasWorkloadsDiskCapacity = (disk: Disk): boolean =>
  typeof disk.total === 'number' && disk.total > 0;

export const getWorkloadsDiskUsagePercent = (disk: Disk): number => {
  if (!hasWorkloadsDiskCapacity(disk)) {
    return 0;
  }

  return (disk.used / disk.total) * 100;
};

export const getWorkloadsDiskLabel = (disk: Disk): string =>
  disk.mountpoint || disk.device || 'Unknown';

export const getWorkloadsDiskLabelTitle = (label: string): string | undefined =>
  label !== 'Unknown' ? label : undefined;

export const getWorkloadsDiskUsageText = (disk: Disk): string =>
  hasWorkloadsDiskCapacity(disk)
    ? `${formatBytes(disk.used)}/${formatBytes(disk.total)}`
    : 'Usage unavailable';

export const getWorkloadsDiskUsagePercentLabel = (disk: Disk): string =>
  hasWorkloadsDiskCapacity(disk) ? `${getWorkloadsDiskUsagePercent(disk).toFixed(0)}%` : '—';

export const getWorkloadsDiskProgressClass = (
  disk: Disk,
  thresholds?: MetricDisplayThresholds | null,
): string => getMetricColorClass(getWorkloadsDiskUsagePercent(disk), 'disk', thresholds);

export const getWorkloadsDiskProgressWidth = (disk: Disk): string =>
  `${Math.min(getWorkloadsDiskUsagePercent(disk), 100)}%`;

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
    progressWidth: getWorkloadsDiskProgressWidth(disk),
    typeLabel: getWorkloadsDiskTypeLabel(disk),
    usageText: getWorkloadsDiskUsageText(disk),
    usagePercentLabel: getWorkloadsDiskUsagePercentLabel(disk),
  };
};
