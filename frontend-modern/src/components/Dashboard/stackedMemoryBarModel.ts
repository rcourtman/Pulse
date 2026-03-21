import type { AnomalyReport } from '@/types/aiIntelligence';
import {
  ANOMALY_SEVERITY_CLASS,
  estimateTextWidth,
  formatAnomalyRatio,
  formatBytes,
  formatPercent,
} from '@/utils/format';
import { getMetricColorRgba } from '@/utils/metricThresholds';

export interface StackedMemoryBarProps {
  used: number;
  total: number;
  percentOnly?: number;
  swapUsed?: number;
  swapTotal?: number;
  balloon?: number;
  resourceId?: string;
  anomaly?: AnomalyReport | null;
}

export interface StackedMemorySegment {
  color: string;
  label: string;
  leftPercent: number;
  widthPercent: number;
}

export interface StackedMemoryTooltipRow {
  borderTop: boolean;
  label: string;
  labelClass: string;
  value: string;
}

export interface StackedMemoryBarPresentation {
  anomalyClass: string;
  anomalyDescription?: string;
  anomalyRatio: string;
  displayLabel: string;
  displaySublabel: string;
  segments: StackedMemorySegment[];
  showSublabel: boolean;
  showSwapBar: boolean;
  swapBarPercent: number;
  tooltipRows: StackedMemoryTooltipRow[];
  tooltipTitle: string;
}

const MEMORY_COLORS = {
  active: 'rgba(34, 197, 94, 0.6)',
  balloon: 'rgba(59, 130, 246, 0.6)',
  swap: 'rgba(168, 85, 247, 0.6)',
};

function getUtilizationPercent(props: StackedMemoryBarProps): number {
  if (props.total > 0) {
    return (props.used / props.total) * 100;
  }
  if (Number.isFinite(props.percentOnly)) {
    return Math.max(0, Math.min(props.percentOnly as number, 100));
  }
  return 0;
}

function getSegments(props: StackedMemoryBarProps, utilizationPercent: number): StackedMemorySegment[] {
  if (props.total <= 0) {
    if (utilizationPercent <= 0) {
      return [];
    }
    return [
      {
        color: getMetricColorRgba(utilizationPercent, 'memory'),
        label: 'Utilization',
        leftPercent: 0,
        widthPercent: utilizationPercent,
      },
    ];
  }

  const balloon = props.balloon || 0;
  const hasActiveBallooning = balloon > 0 && balloon < props.total;
  const usedPercent = (props.used / props.total) * 100;

  if (!hasActiveBallooning) {
    if (props.used <= 0) {
      return [];
    }
    return [
      {
        color: getMetricColorRgba(usedPercent, 'memory'),
        label: 'Active',
        leftPercent: 0,
        widthPercent: usedPercent,
      },
    ];
  }

  const segments: StackedMemorySegment[] = [];
  if (props.used > 0) {
    segments.push({
      color: getMetricColorRgba(usedPercent, 'memory'),
      label: 'Active',
      leftPercent: 0,
      widthPercent: usedPercent,
    });
  }

  const balloonLimitPercent = Math.max(0, (balloon / props.total) * 100 - usedPercent);
  if (balloonLimitPercent > 0 && balloon > props.used) {
    segments.push({
      color: MEMORY_COLORS.balloon,
      label: 'Balloon',
      leftPercent: usedPercent,
      widthPercent: balloonLimitPercent,
    });
  }

  return segments;
}

function getTooltipRows(
  props: StackedMemoryBarProps,
  displayLabel: string,
): StackedMemoryTooltipRow[] {
  const rows: StackedMemoryTooltipRow[] = [];
  const balloon = props.balloon || 0;
  const hasActiveBallooning = props.total > 0 && balloon > 0 && balloon < props.total;
  const hasSwap = (props.swapTotal || 0) > 0;

  if (props.total > 0) {
    rows.push({
      borderTop: false,
      label: 'Used',
      labelClass: 'text-green-400',
      value: formatBytes(props.used),
    });

    if (hasActiveBallooning) {
      rows.push({
        borderTop: true,
        label: 'Balloon Limit',
        labelClass: 'text-blue-400',
        value: formatBytes(balloon),
      });
    }

    rows.push({
      borderTop: true,
      label: 'Free',
      labelClass: 'text-slate-400',
      value: formatBytes(props.total - props.used),
    });
  } else {
    rows.push({
      borderTop: true,
      label: 'Utilization',
      labelClass: 'text-blue-300',
      value: displayLabel,
    });
  }

  if (props.total > 0 && hasSwap) {
    rows.push({
      borderTop: true,
      label: 'Swap',
      labelClass: 'text-purple-400',
      value: `${formatBytes(props.swapUsed || 0)} / ${formatBytes(props.swapTotal || 0)}`,
    });
  }

  return rows;
}

export function buildStackedMemoryBarPresentation(
  props: StackedMemoryBarProps,
  containerWidth: number,
): StackedMemoryBarPresentation {
  const utilizationPercent = getUtilizationPercent(props);
  const displayLabel = formatPercent(utilizationPercent);
  const displaySublabel =
    props.total > 0 ? `${formatBytes(props.used)}/${formatBytes(props.total)}` : '';
  const showSublabel =
    displaySublabel.length > 0 &&
    containerWidth >= estimateTextWidth(`${displayLabel} (${displaySublabel})`);

  return {
    anomalyClass: props.anomaly
      ? (ANOMALY_SEVERITY_CLASS[props.anomaly.severity] ?? 'text-yellow-400')
      : 'text-yellow-400',
    anomalyDescription: props.anomaly?.description,
    anomalyRatio: formatAnomalyRatio(props.anomaly),
    displayLabel,
    displaySublabel,
    segments: getSegments(props, utilizationPercent),
    showSublabel,
    showSwapBar: (props.swapTotal || 0) > 0 && (props.swapUsed || 0) > 0,
    swapBarPercent:
      props.swapTotal && props.swapTotal > 0
        ? Math.min(((props.swapUsed || 0) / props.swapTotal) * 100, 100)
        : 0,
    tooltipRows: getTooltipRows(props, displayLabel),
    tooltipTitle: 'Memory Composition',
  };
}

