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
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

export interface StackedDiskBarProps {
  disks?: Disk[];
  aggregateDisk?: Disk;
  mode?: 'stacked' | 'aggregate' | 'mini';
  anomaly?: AnomalyReport | null;
  thresholds?: MetricDisplayThresholds | null;
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
  inlineText: string;
  label: string;
  percent: number;
  percentLabel: string;
  shortLabel: string;
  title: string;
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
  displayPercentValue: number;
  displaySublabel: string;
  hasDisks: boolean;
  hasMultipleDisks: boolean;
  inlineDiskMode: boolean;
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
  if (disk.total > 0) {
    return (disk.used / disk.total) * 100;
  }
  if (Number.isFinite(disk.usage)) {
    return disk.usage <= 1 ? disk.usage * 100 : disk.usage;
  }
  return 0;
}

function getDiskLabel(disk: Disk, index: number): string {
  return disk.mountpoint || disk.device || `Disk ${index + 1}`;
}

function getShortDiskLabel(label: string): string {
  const trimmed = label.trim();
  if (trimmed.startsWith('/dev/')) {
    return trimmed.slice('/dev/'.length);
  }

  const parts = trimmed.split('/').filter(Boolean);
  if (trimmed === '/') {
    return '/';
  }
  if (trimmed.startsWith('/') && parts.length > 0) {
    return parts[parts.length - 1];
  }
  if (trimmed.length <= 12) {
    return trimmed;
  }

  return trimmed;
}

function estimateInlineTextWidth(text: string): number {
  return text.length * 4.6 + 4;
}

function getInlineDiskText(shortLabel: string, percentLabel: string, slotWidth: number): string {
  const fullText = `${shortLabel} ${percentLabel}`;
  if (slotWidth <= 0 || estimateInlineTextWidth(fullText) <= slotWidth) {
    return fullText;
  }
  if (estimateInlineTextWidth(shortLabel) <= slotWidth) {
    return shortLabel;
  }
  if (estimateInlineTextWidth(percentLabel) <= slotWidth) {
    return percentLabel;
  }
  return '';
}

function getStackedDiskColor(
  percent: number,
  index: number,
  thresholds?: MetricDisplayThresholds | null,
): string {
  const critical = thresholds?.critical ?? 90;
  const warning = thresholds?.warning ?? 80;
  if (percent >= critical) {
    return getMetricColorRgba(percent, 'disk', thresholds);
  }
  if (percent >= warning) {
    return getMetricColorRgba(percent, 'disk', thresholds);
  }
  return SEGMENT_COLORS[index % SEGMENT_COLORS.length];
}

function buildTooltipContent(
  disks: Disk[],
  options: {
    aggregateDisk: Disk | undefined;
    aggregateMode: boolean;
    inlineDiskMode: boolean;
    miniMode: boolean;
    thresholds?: MetricDisplayThresholds | null;
  },
): StackedDiskTooltipItem[] {
  const useUsageColors = options.aggregateMode || options.inlineDiskMode || options.miniMode;
  if (disks.length > 0) {
    return disks.map((disk, index) => {
      const percentValue = getDiskUsagePercent(disk);
      return {
        color: useUsageColors
          ? getMetricColorRgba(percentValue, 'disk', options.thresholds)
          : getStackedDiskColor(percentValue, index, options.thresholds),
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
        color: getMetricColorRgba(percentValue, 'disk', options.thresholds),
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
  const explicitStackedMode = props.mode === 'stacked';
  const inlineDiskMode = (miniMode || hasMultipleDisks) && !aggregateMode && !explicitStackedMode;
  const useStackedSegments = hasMultipleDisks && explicitStackedMode;
  const inlineDiskSlotWidth = disks.length > 0 ? containerWidth / disks.length : 0;
  const totalCapacity = hasDisks
    ? disks.reduce((sum, disk) => sum + (disk.total || 0), 0)
    : (props.aggregateDisk?.total ?? 0);
  const totalUsed = hasDisks
    ? disks.reduce((sum, disk) => sum + (disk.used || 0), 0)
    : (props.aggregateDisk?.used ?? 0);
  const overallPercent =
    totalCapacity > 0
      ? (totalUsed / totalCapacity) * 100
      : props.aggregateDisk
        ? getDiskUsagePercent(props.aggregateDisk)
        : 0;
  const anomalyRatio = formatAnomalyRatio(props.anomaly) ?? '';
  const maxInfo = getMaxDiskInfo(disks);
  const displayPercentValue = overallPercent;
  const barPercent = Math.min(displayPercentValue, 100);
  const maxLabelShort = maxInfo ? `max ${formatPercent(maxInfo.percent)}` : '';
  const maxLabelFull = maxInfo ? `Max ${formatPercent(maxInfo.percent)} (${maxInfo.label})` : '';
  const displayLabel = formatPercent(displayPercentValue);
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
      ? getMetricColorRgba(maxInfo.percent, 'disk', props.thresholds)
      : getMetricColorRgba(overallPercent, 'disk', props.thresholds);
  const segments =
    useStackedSegments && totalCapacity > 0
      ? disks.map((disk, index) => {
          const diskUsagePercent = getDiskUsagePercent(disk);
          return {
            color: getStackedDiskColor(diskUsagePercent, index, props.thresholds),
            disk,
            diskUsagePercent,
            index,
            widthPercent: Math.min((disk.used / totalCapacity) * 100, 100),
          };
        })
      : [];
  const miniDisks = disks.map((disk, index) => {
    const percent = getDiskUsagePercent(disk);
    const label = getDiskLabel(disk, index);
    const percentLabel = formatPercent(percent);
    const shortLabel = getShortDiskLabel(label);
    return {
      color: getMetricColorRgba(percent, 'disk', props.thresholds),
      inlineText: getInlineDiskText(shortLabel, percentLabel, inlineDiskSlotWidth),
      label,
      percent,
      percentLabel,
      shortLabel,
      title: `${label}: ${percentLabel} (${formatBytes(disk.used)}/${formatBytes(disk.total)})`,
    };
  });
  const tooltipContent = buildTooltipContent(disks, {
    aggregateDisk: props.aggregateDisk,
    aggregateMode,
    inlineDiskMode,
    miniMode,
    thresholds: props.thresholds,
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
      inlineDiskMode && hasDisks
        ? 'metric-text w-full h-4 min-w-0'
        : 'metric-text w-full h-4 flex items-center justify-center',
    displayLabel,
    displayPercentValue,
    displaySublabel,
    hasDisks,
    hasMultipleDisks,
    inlineDiskMode,
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
