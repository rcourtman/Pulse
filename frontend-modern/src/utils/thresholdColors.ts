/**
 * Threshold-based color utilities for metric bars.
 * Provides consistent coloring across CPU, memory, and disk usage displays.
 */

export type ThresholdColor = 'red' | 'yellow' | 'green';

export interface ThresholdConfig {
  critical: number;  // >= this value = red
  warning: number;   // >= this value = yellow
}

/**
 * Default thresholds for different metric types.
 */
export const METRIC_THRESHOLDS: Record<string, ThresholdConfig> = {
  cpu: { critical: 90, warning: 80 },
  memory: { critical: 85, warning: 75 },
  disk: { critical: 90, warning: 80 },
  generic: { critical: 90, warning: 75 },
};

/**
 * Determine the threshold color based on percentage and metric type.
 */
export function getThresholdColor(
  percentage: number,
  metricType: 'cpu' | 'memory' | 'disk' | 'generic' = 'generic'
): ThresholdColor {
  const thresholds = METRIC_THRESHOLDS[metricType] || METRIC_THRESHOLDS.generic;

  if (percentage >= thresholds.critical) return 'red';
  if (percentage >= thresholds.warning) return 'yellow';
  return 'green';
}

/**
 * CSS classes for threshold-based bar colors.
 */
export const THRESHOLD_BAR_CLASSES: Record<ThresholdColor, string> = {
  red: 'bg-red-500/60 dark:bg-red-500/50',
  yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
  green: 'bg-green-500/60 dark:bg-green-500/50',
};

/**
 * Get the CSS class for a bar based on percentage and metric type.
 */
export function getThresholdBarClass(
  percentage: number,
  metricType: 'cpu' | 'memory' | 'disk' | 'generic' = 'generic'
): string {
  const color = getThresholdColor(percentage, metricType);
  return THRESHOLD_BAR_CLASSES[color] || THRESHOLD_BAR_CLASSES.green;
}

/**
 * RGBA colors for threshold-based fills (for inline styles).
 */
export const THRESHOLD_RGBA_COLORS: Record<ThresholdColor, string> = {
  red: 'rgba(239, 68, 68, 0.6)',    // red-500
  yellow: 'rgba(234, 179, 8, 0.6)', // yellow-500
  green: 'rgba(34, 197, 94, 0.6)',  // green-500
};

/**
 * Get the RGBA color for a bar based on percentage.
 * Used for inline styles in stacked bars.
 */
export function getThresholdRgbaColor(percentage: number): string {
  if (percentage >= 90) return THRESHOLD_RGBA_COLORS.red;
  if (percentage >= 80) return THRESHOLD_RGBA_COLORS.yellow;
  return THRESHOLD_RGBA_COLORS.green;
}
