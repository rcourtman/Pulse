import type { AnomalyReport } from '@/types/aiIntelligence';
import {
  ANOMALY_SEVERITY_CLASS,
  estimateTextWidth,
  formatAnomalyRatio,
  formatBytes,
  formatPercent,
} from '@/utils/format';
import { getMetricColorRgba, getMetricSeverity } from '@/utils/metricThresholds';
import type { MetricDisplayThresholds, MetricSeverity } from '@/utils/metricThresholds';

export interface StackedMemoryBarProps {
  used: number;
  total: number;
  unavailable?: boolean;
  percentOnly?: number;
  /** Reclaimable buff/cache (available - truly free); used + cache + free ≈ total. */
  cache?: number;
  cacheInclusiveLabel?: string;
  swapUsed?: number;
  swapTotal?: number;
  balloon?: number;
  resourceId?: string;
  anomaly?: AnomalyReport | null;
  thresholds?: MetricDisplayThresholds | null;
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
  displayPercentValue: number;
  displaySublabel: string;
  segments: StackedMemorySegment[];
  showSublabel: boolean;
  showSwapBar: boolean;
  swapBarPercent: number;
  tooltipRows: StackedMemoryTooltipRow[];
  tooltipTitle: string;
  unavailable: boolean;
}

// Tooltip legend for the used segment tracks the same severity that colors
// the bar, so the legend never claims green while the bar shows warning/red.
const USED_LABEL_CLASS: Record<MetricSeverity, string> = {
  normal: 'text-green-400',
  warning: 'text-yellow-400',
  critical: 'text-red-400',
};

const MEMORY_COLORS = {
  active: 'rgba(34, 197, 94, 0.6)',
  // Muted amber: reclaimable buff/cache, matching the v5 segment tone.
  cache: 'rgba(251, 191, 36, 0.45)',
  balloon: 'rgba(59, 130, 246, 0.6)',
  swap: 'rgba(168, 85, 247, 0.6)',
};

// Cache can never exceed the non-used pages; clamp so a momentarily
// inconsistent snapshot (used drifting past total - cache) cannot push the
// segments or the reconciliation row past 100%.
function getEffectiveCache(props: StackedMemoryBarProps): number {
  const cache = props.cache || 0;
  if (cache <= 0 || props.total <= 0) return 0;
  return Math.min(cache, Math.max(0, props.total - props.used));
}

function getUtilizationPercent(props: StackedMemoryBarProps): number {
  if (props.unavailable) return 0;
  if (props.total > 0) {
    return (props.used / props.total) * 100;
  }
  if (Number.isFinite(props.percentOnly)) {
    return Math.max(0, Math.min(props.percentOnly as number, 100));
  }
  return 0;
}

function getSegments(
  props: StackedMemoryBarProps,
  utilizationPercent: number,
): StackedMemorySegment[] {
  if (props.unavailable) {
    return [];
  }
  if (props.total <= 0) {
    if (utilizationPercent <= 0) {
      return [];
    }
    return [
      {
        color: getMetricColorRgba(utilizationPercent, 'memory', props.thresholds),
        label: 'Utilization',
        leftPercent: 0,
        widthPercent: utilizationPercent,
      },
    ];
  }

  const balloon = props.balloon || 0;
  const hasActiveBallooning = balloon > 0 && balloon < props.total;
  const usedPercent = (props.used / props.total) * 100;
  const cache = getEffectiveCache(props);
  const cachePercent = (cache / props.total) * 100;

  const segments: StackedMemorySegment[] = [];
  if (props.used > 0) {
    segments.push({
      color: getMetricColorRgba(usedPercent, 'memory', props.thresholds),
      label: 'Active',
      leftPercent: 0,
      widthPercent: usedPercent,
    });
  }

  // Reclaimable buff/cache rides between active and the balloon limit, like v5.
  if (cache > 0) {
    segments.push({
      color: MEMORY_COLORS.cache,
      label: 'Reclaimable',
      leftPercent: usedPercent,
      widthPercent: cachePercent,
    });
  }

  if (hasActiveBallooning) {
    const usedPlusCache = props.used + cache;
    const balloonLimitPercent = Math.max(
      0,
      (balloon / props.total) * 100 - usedPercent - cachePercent,
    );
    if (balloonLimitPercent > 0 && balloon > usedPlusCache) {
      segments.push({
        color: MEMORY_COLORS.balloon,
        label: 'Balloon',
        leftPercent: usedPercent + cachePercent,
        widthPercent: balloonLimitPercent,
      });
    }
  }

  return segments;
}

function getTooltipRows(
  props: StackedMemoryBarProps,
  displayLabel: string,
): StackedMemoryTooltipRow[] {
  const rows: StackedMemoryTooltipRow[] = [];
  const balloon = props.balloon || 0;
  const cache = getEffectiveCache(props);
  const hasActiveBallooning = props.total > 0 && balloon > 0 && balloon < props.total;
  const hasSwap = (props.swapTotal || 0) > 0;

  if (props.unavailable) {
    rows.push({
      borderTop: false,
      label: 'Usage',
      labelClass: 'text-slate-400',
      value: 'Unavailable',
    });
    if (props.total > 0) {
      rows.push({
        borderTop: true,
        label: 'Total',
        labelClass: 'text-slate-400',
        value: formatBytes(props.total),
      });
    }
  } else if (props.total > 0) {
    const usedPercent = (props.used / props.total) * 100;
    rows.push({
      borderTop: false,
      label: 'Used',
      labelClass: USED_LABEL_CLASS[getMetricSeverity(usedPercent, 'memory', props.thresholds)],
      value: formatBytes(props.used),
    });

    if (cache > 0) {
      rows.push({
        borderTop: true,
        label: 'Reclaimable cache',
        labelClass: 'text-amber-400',
        value: formatBytes(cache),
      });
    }

    if (hasActiveBallooning) {
      rows.push({
        borderTop: true,
        label: 'Balloon Limit',
        labelClass: 'text-blue-400',
        value: formatBytes(balloon),
      });
    }

    // Truly free pages exclude the reclaimable cache; capped at the balloon
    // limit when ballooning is active (the guest cannot use past it).
    const ceiling = hasActiveBallooning ? balloon : props.total;
    rows.push({
      borderTop: true,
      label: 'Free',
      labelClass: 'text-slate-400',
      value: formatBytes(Math.max(0, ceiling - props.used - cache)),
    });

    // Some providers count reclaimable cache as used; keep the shared default
    // source-neutral and let provider-owned surfaces name their comparison UI.
    if (cache > 0) {
      rows.push({
        borderTop: true,
        label: props.cacheInclusiveLabel ?? 'Used with cache',
        labelClass: 'text-slate-500 italic',
        value: formatPercent(((props.used + cache) / props.total) * 100),
      });
    }
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
      labelClass: 'text-amber-400',
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
    !props.unavailable && props.total > 0
      ? `${formatBytes(props.used)}/${formatBytes(props.total)}`
      : '';
  const showSublabel =
    displaySublabel.length > 0 &&
    containerWidth >= estimateTextWidth(`${displayLabel} (${displaySublabel})`);

  return {
    anomalyClass: props.anomaly
      ? (ANOMALY_SEVERITY_CLASS[props.anomaly.severity] ?? 'text-yellow-400')
      : 'text-yellow-400',
    anomalyDescription: props.anomaly?.description,
    anomalyRatio: formatAnomalyRatio(props.anomaly) ?? '',
    displayLabel,
    displayPercentValue: utilizationPercent,
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
    unavailable: props.unavailable === true,
  };
}
