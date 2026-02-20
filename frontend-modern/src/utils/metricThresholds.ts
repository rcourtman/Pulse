/**
 * Centralized metric threshold definitions and color utilities.
 *
 * ALL metric color/threshold logic MUST live here.
 * Do NOT define threshold constants or color- functions
 * in components — import from this module instead.
 */

export type MetricType = 'cpu' | 'memory' | 'disk';

export const METRIC_THRESHOLDS: Record<MetricType, { warning: number; critical: number }> = {
  cpu:    { warning: 80, critical: 90 },
  memory: { warning: 75, critical: 85 },
  disk:   { warning: 80, critical: 90 },
};

export type MetricSeverity = 'normal' | 'warning' | 'critical';

/** Determine severity level from a percentage value and metric type. */
export function getMetricSeverity(value: number, metric: MetricType): MetricSeverity {
  const t = METRIC_THRESHOLDS[metric];
  if (value >= t.critical) return 'critical';
  if (value >= t.warning) return 'warning';
  return 'normal';
}

// -- Tailwind background classes (progress bars) --

const BG_CLASSES: Record<MetricSeverity, string> = {
  critical: 'bg-red-500 dark:bg-red-500',
  warning:  'bg-yellow-500 dark:bg-yellow-500',
  normal:   'bg-green-500 dark:bg-green-500',
};

export function getMetricColorClass(value: number, metric: MetricType): string {
  return BG_CLASSES[getMetricSeverity(value, metric)];
}

// -- RGBA colors (canvas rendering, inline styles) --

const RGBA_COLORS: Record<MetricSeverity, string> = {
  critical: 'rgba(239, 68, 68, 0.6)',
  warning:  'rgba(234, 179, 8, 0.6)',
  normal:   'rgba(34, 197, 94, 0.6)',
};

export function getMetricColorRgba(value: number, metric: MetricType): string {
  return RGBA_COLORS[getMetricSeverity(value, metric)];
}

// -- Hex colors (sparkline canvas) --

const HEX_COLORS: Record<MetricSeverity, string> = {
  critical: '#ef4444',
  warning:  '#eab308',
  normal:   '#22c55e',
};

export function getMetricColorHex(value: number, metric: MetricType): string {
  return HEX_COLORS[getMetricSeverity(value, metric)];
}

// -- Text color classes (labels, percentage text) --

const TEXT_CLASSES: Record<MetricSeverity, string> = {
  critical: 'text-red-600 dark:text-red-400',
  warning:  'text-yellow-600 dark:text-yellow-400',
  normal:   'text-slate-500 dark:text-slate-400',
};

export function getMetricTextColorClass(value: number, metric: MetricType): string {
  return TEXT_CLASSES[getMetricSeverity(value, metric)];
}
