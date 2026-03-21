import type { Disk } from '@/types/api';
import type { AnomalyReport } from '@/types/aiIntelligence';
import {
  ANOMALY_SEVERITY_CLASS,
  estimateTextWidth,
  formatAnomalyRatio,
  formatBytes,
  formatPercent,
} from '@/utils/format';
import { getMetricColorRgba } from '@/utils/metricThresholds';

export interface StackedDiskBarProps {
  disks?: Disk[];
  aggregateDisk?: Disk;
  mode?: 'stacked' | 'aggregate' | 'mini';
  anomaly?: AnomalyReport | null;
}

export interface StackedDiskSegment {
  color: string;
  disk: Disk;
  diskUsagePercent: number;
  index: number;
  widthPercent: number;
}

export interface StackedDiskTooltipItem {
  color: string;
  label: string;
  percent: string;
  total: string;
  used: string;
}

export interface StackedDiskMiniDisk {
  color: string;
  label: string;
  percent: number;
}

export interface StackedDiskMaxInfo {
  label: string;
  percent: number;
}

export interface StackedDiskBarPresentation {
  aggregateMode: boolean;
  anomalyClass: string;
  anomalyDescription?: string;
  anomalyRatio: string;
  barColor: string;
  barPercent: number;
  containerClass: string;
  displayLabel: string;
  displaySublabel: string;
  hasDisks: boolean;
  hasMultipleDisks: boolean;
  maxLabelFull: string;
  maxLabelShort: string;
  miniDisks: StackedDiskMiniDisk[];
  miniMode: boolean;
  segments: StackedDiskSegment[];
  showDiskCount: boolean;
  showMaxLabel: boolean;
  showSublabel: boolean;
  tooltipContent: StackedDiskTooltipItem[];
  tooltipTitle: string;
  useStackedSegments: boolean;
}

const SEGMENT_COLORS = [
  'rgba(34, 197, 94, 0.6)',
  'rgba(59, 130, 246, 0.6)',
  'rgba(168, 85, 247, 0.6)',
  'rgba(249, 115, 22, 0.6)',
  'rgba(236, 72, 153, 0.6)',
  'rgba(20, 184, 166, 0.6)',
];

function getDiskUsagePercent(disk: Disk): number {
  return disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
}

function getDiskLabel(disk: Disk, index: number): string {
  return disk.mountpoint || disk.device || `Disk ${index + 1}`;
}

function getStackedDiskColor(percent: number, index: number): string {
  if (percent >= 90) {
    return getMetricColorRgba(90, 'disk');
  }
  if (percent >= 80) {
    return getMetricColorRgba(80, 'disk');
  }
  return SEGMENT_COLORS[index % SEGMENT_COLORS.length];
}

function buildTooltipContent(
  disks: Disk[],
  options: {
    aggregateDisk: Disk | undefined;
    aggregateMode: boolean;
    miniMode: boolean;
  },
): StackedDiskTooltipItem[] {
  const useUsageColors = options.aggregateMode || options.miniMode;
  if (disks.length > 0) {
    return disks.map((disk, index) => {
      const percentValue = getDiskUsagePercent(disk);
      return {
        color: useUsageColors
          ? getMetricColorRgba(percentValue, 'disk')
          : getStackedDiskColor(percentValue, index),
        label: getDiskLabel(disk, index),
        percent: formatPercent(percentValue),
        total: formatBytes(disk.total),
        used: formatBytes(disk.used),
      };
    });
  }

  if (options.aggregateDisk && options.aggregateDisk.total > 0) {
    const percentValue = getDiskUsagePercent(options.aggregateDisk);
    return [
      {
        color: getMetricColorRgba(percentValue, 'disk'),
        label: 'Total',
        percent: formatPercent(percentValue),
        total: formatBytes(options.aggregateDisk.total),
        used: formatBytes(options.aggregateDisk.used),
      },
    ];
  }

  return [];
}

function getMaxDiskInfo(disks: Disk[]): StackedDiskMaxInfo | null {
  if (disks.length === 0) {
    return null;
  }

  let maxInfo: StackedDiskMaxInfo | null = null;
  for (const [index, disk] of disks.entries()) {
    const percent = getDiskUsagePercent(disk);
    if (!maxInfo || percent > maxInfo.percent) {
      maxInfo = {
        label: getDiskLabel(disk, index),
        percent,
      };
    }
  }
  return maxInfo;
}

export function buildStackedDiskBarPresentation(
  props: StackedDiskBarProps,
  containerWidth: number,
): StackedDiskBarPresentation {
  const disks = props.disks ?? [];
  const hasDisks = disks.length > 0;
  const hasMultipleDisks = disks.length > 1;
  const aggregateMode = props.mode === 'aggregate';
  const miniMode = props.mode === 'mini';
  const useStackedSegments = hasMultipleDisks && !aggregateMode && !miniMode;
  const totalCapacity = hasDisks
    ? disks.reduce((sum, disk) => sum + (disk.total || 0), 0)
    : (props.aggregateDisk?.total ?? 0);
  const totalUsed = hasDisks
    ? disks.reduce((sum, disk) => sum + (disk.used || 0), 0)
    : (props.aggregateDisk?.used ?? 0);
  const overallPercent = totalCapacity > 0 ? (totalUsed / totalCapacity) * 100 : 0;
  const barPercent = Math.min(overallPercent, 100);
  const anomalyRatio = formatAnomalyRatio(props.anomaly);
  const maxInfo = getMaxDiskInfo(disks);
  const maxLabelShort = maxInfo ? `max ${formatPercent(maxInfo.percent)}` : '';
  const maxLabelFull = maxInfo
    ? `Max ${formatPercent(maxInfo.percent)} (${maxInfo.label})`
    : '';
  const displayLabel = formatPercent(overallPercent);
  const displaySublabel = `${formatBytes(totalUsed)}/${formatBytes(totalCapacity)}`;
  const showMaxLabel =
    aggregateMode &&
    hasMultipleDisks &&
    maxLabelShort.length > 0 &&
    containerWidth >= estimateTextWidth(`${displayLabel} ${maxLabelShort}`);
  const showSublabel =
    containerWidth >=
    estimateTextWidth(
      `${displayLabel}${showMaxLabel ? ` ${maxLabelShort}` : ''} (${displaySublabel})`,
    );
  const barColor =
    aggregateMode && hasMultipleDisks && maxInfo
      ? getMetricColorRgba(maxInfo.percent, 'disk')
      : getMetricColorRgba(overallPercent, 'disk');
  const segments =
    useStackedSegments && totalCapacity > 0
      ? disks.map((disk, index) => {
          const diskUsagePercent = getDiskUsagePercent(disk);
          return {
            color: getStackedDiskColor(diskUsagePercent, index),
            disk,
            diskUsagePercent,
            index,
            widthPercent: Math.min((disk.used / totalCapacity) * 100, 100),
          };
        })
      : [];
  const miniDisks = disks.map((disk, index) => {
    const percent = getDiskUsagePercent(disk);
    return {
      color: getMetricColorRgba(percent, 'disk'),
      label: getDiskLabel(disk, index),
      percent,
    };
  });
  const tooltipContent = buildTooltipContent(disks, {
    aggregateDisk: props.aggregateDisk,
    aggregateMode,
    miniMode,
  });

  return {
    aggregateMode,
    anomalyClass: props.anomaly
      ? (ANOMALY_SEVERITY_CLASS[props.anomaly.severity] ?? 'text-yellow-400')
      : 'text-yellow-400',
    anomalyDescription: props.anomaly?.description,
    anomalyRatio,
    barColor,
    barPercent,
    containerClass:
      miniMode && hasDisks ? 'metric-text w-full' : 'metric-text w-full h-4 flex items-center justify-center',
    displayLabel,
    displaySublabel,
    hasDisks,
    hasMultipleDisks,
    maxLabelFull,
    maxLabelShort,
    miniDisks,
    miniMode,
    segments,
    showDiskCount: useStackedSegments,
    showMaxLabel,
    showSublabel,
    tooltipContent,
    tooltipTitle: hasMultipleDisks ? 'Disk Breakdown' : 'Disk Usage',
    useStackedSegments,
  };
}
