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
import type { MemoryObservationPresentation } from '@/utils/memoryObservation';

export interface StackedMemoryBarProps {
  used: number;
  total: number;
  unavailable?: boolean;
  /** Already classified by the owning surface; never infer freshness from capacity. */
  reading?: Pick<MemoryObservationPresentation, 'state' | 'message'> | null;
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
  /** Optional severity value when the bar geometry uses a different denominator. */
  severityPercent?: number;
  /** Render a direct used-vs-total comparison instead of composition/free rows. */
  comparisonTotalLabel?: string;
  tooltipTitle?: string;
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
  swapBarColor: string;
  tooltipRows: StackedMemoryTooltipRow[];
  tooltipMessage?: string;
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

const RETAINED_MEMORY_COLOR = 'rgba(148, 163, 184, 0.5)';

const isMemoryUnavailable = (props: StackedMemoryBarProps): boolean =>
  props.unavailable === true || props.reading?.state === 'unavailable';

// A retained value can still explain the last report, but it cannot assert
// today's pressure or anomaly. Unannotated platform bars keep their behaviour.
const isMemoryCurrent = (props: StackedMemoryBarProps): boolean =>
  !isMemoryUnavailable(props) && (!props.reading || props.reading.state === 'current');

// Cache can never exceed the non-used pages; clamp so a momentarily
// inconsistent snapshot (used drifting past total - cache) cannot push the
// segments or the reconciliation row past 100%.
function getEffectiveCache(props: StackedMemoryBarProps): number {
  const cache = props.cache || 0;
  if (cache <= 0 || props.total <= 0) return 0;
  return Math.min(cache, Math.max(0, props.total - props.used));
}

function getUtilizationPercent(props: StackedMemoryBarProps): number {
  if (isMemoryUnavailable(props)) return 0;
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
  if (isMemoryUnavailable(props)) {
    return [];
  }
  if (props.total <= 0) {
    if (utilizationPercent <= 0) {
      return [];
    }
    return [
      {
        color: isMemoryCurrent(props)
          ? getMetricColorRgba(utilizationPercent, 'memory', props.thresholds)
          : RETAINED_MEMORY_COLOR,
        label: 'Utilization',
        leftPercent: 0,
        widthPercent: utilizationPercent,
      },
    ];
  }

  const balloon = props.balloon || 0;
  const hasActiveBallooning = balloon > 0 && balloon < props.total;
  const usedPercent = (props.used / props.total) * 100;
  const severityPercent = Number.isFinite(props.severityPercent)
    ? (props.severityPercent as number)
    : usedPercent;
  const cache = getEffectiveCache(props);
  const cachePercent = (cache / props.total) * 100;

  const segments: StackedMemorySegment[] = [];
  if (props.used > 0) {
    segments.push({
      color: isMemoryCurrent(props)
        ? getMetricColorRgba(severityPercent, 'memory', props.thresholds)
        : RETAINED_MEMORY_COLOR,
      label: 'Active',
      leftPercent: 0,
      widthPercent: usedPercent,
    });
  }

  // Reclaimable buff/cache rides between active and the balloon limit, like v5.
  if (cache > 0) {
    segments.push({
      color: isMemoryCurrent(props) ? MEMORY_COLORS.cache : RETAINED_MEMORY_COLOR,
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
        color: isMemoryCurrent(props) ? MEMORY_COLORS.balloon : RETAINED_MEMORY_COLOR,
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

  if (isMemoryUnavailable(props)) {
    rows.push({
      borderTop: false,
      label: 'Usage',
      labelClass: 'text-muted',
      value: 'Unavailable',
    });
    if (props.total > 0) {
      rows.push({
        borderTop: true,
        label: 'Total',
        labelClass: 'text-muted',
        value: formatBytes(props.total),
      });
    }
  } else if (props.total > 0) {
    const usedPercent = (props.used / props.total) * 100;
    const severityPercent = Number.isFinite(props.severityPercent)
      ? (props.severityPercent as number)
      : usedPercent;
    rows.push({
      borderTop: false,
      label: 'Used',
      labelClass: isMemoryCurrent(props)
        ? USED_LABEL_CLASS[getMetricSeverity(severityPercent, 'memory', props.thresholds)]
        : 'text-muted',
      value: formatBytes(props.used),
    });

    if (props.comparisonTotalLabel) {
      rows.push({
        borderTop: true,
        label: props.comparisonTotalLabel,
        labelClass: 'text-muted',
        value: formatBytes(props.total),
      });
      return rows;
    }

    if (cache > 0) {
      rows.push({
        borderTop: true,
        label: 'Reclaimable cache',
        labelClass: isMemoryCurrent(props) ? 'text-amber-400' : 'text-muted',
        value: formatBytes(cache),
      });
    }

    if (hasActiveBallooning) {
      rows.push({
        borderTop: true,
        label: 'Balloon Limit',
        labelClass: isMemoryCurrent(props) ? 'text-blue-400' : 'text-muted',
        value: formatBytes(balloon),
      });
    }

    // Truly free pages exclude the reclaimable cache; capped at the balloon
    // limit when ballooning is active (the guest cannot use past it).
    const ceiling = hasActiveBallooning ? balloon : props.total;
    rows.push({
      borderTop: true,
      label: 'Free',
      labelClass: 'text-muted',
      value: formatBytes(Math.max(0, ceiling - props.used - cache)),
    });

    // Some providers count reclaimable cache as used; keep the shared default
    // source-neutral and let provider-owned surfaces name their comparison UI.
    if (cache > 0) {
      rows.push({
        borderTop: true,
        label: props.cacheInclusiveLabel ?? 'Used with cache',
        labelClass: isMemoryCurrent(props) ? 'text-slate-500 italic' : 'text-muted',
        value: formatPercent(((props.used + cache) / props.total) * 100),
      });
    }
  } else {
    rows.push({
      borderTop: true,
      label: 'Utilization',
      labelClass: isMemoryCurrent(props) ? 'text-blue-300' : 'text-muted',
      value: displayLabel,
    });
  }

  if (!isMemoryUnavailable(props) && props.total > 0 && hasSwap) {
    rows.push({
      borderTop: true,
      label: 'Swap',
      labelClass: isMemoryCurrent(props) ? 'text-amber-400' : 'text-muted',
      value: `${formatBytes(props.swapUsed || 0)} / ${formatBytes(props.swapTotal || 0)}`,
    });
  }

  return rows;
}

// Horizontal padding of the label chip inside the bar (px-1 in StackedMemoryBar.tsx).
const LABEL_PADDING_PX = 8;

export function buildStackedMemoryBarPresentation(
  props: StackedMemoryBarProps,
  containerWidth: number,
): StackedMemoryBarPresentation {
  const unavailable = isMemoryUnavailable(props);
  const current = isMemoryCurrent(props);
  const anomaly = current ? props.anomaly : undefined;
  const utilizationPercent = getUtilizationPercent(props);
  const displayLabel = formatPercent(utilizationPercent);
  const displaySublabel =
    !unavailable && props.total > 0 ? `${formatBytes(props.used)}/${formatBytes(props.total)}` : '';
  const anomalyRatio = formatAnomalyRatio(anomaly) ?? '';
  // The anomaly marker shares the label's line whenever it renders.
  const anomalyMarker = anomaly?.description && anomalyRatio ? ` ${anomalyRatio}` : '';
  const showSublabel =
    displaySublabel.length > 0 &&
    containerWidth >=
      estimateTextWidth(`${displayLabel}${anomalyMarker}`, { detail: ` (${displaySublabel})` }) +
        LABEL_PADDING_PX;

  return {
    anomalyClass: anomaly
      ? (ANOMALY_SEVERITY_CLASS[anomaly.severity] ?? 'text-yellow-400')
      : 'text-yellow-400',
    anomalyDescription: anomaly?.description,
    anomalyRatio,
    displayLabel,
    displayPercentValue: utilizationPercent,
    displaySublabel,
    segments: getSegments(props, utilizationPercent),
    showSublabel,
    showSwapBar: !unavailable && (props.swapTotal || 0) > 0 && (props.swapUsed || 0) > 0,
    swapBarPercent:
      !unavailable && props.swapTotal && props.swapTotal > 0
        ? Math.min(((props.swapUsed || 0) / props.swapTotal) * 100, 100)
        : 0,
    swapBarColor: current ? 'rgb(168 85 247)' : RETAINED_MEMORY_COLOR,
    tooltipRows: getTooltipRows(props, displayLabel),
    tooltipMessage: props.reading?.message,
    tooltipTitle: props.tooltipTitle ?? 'Memory Composition',
    unavailable,
  };
}
