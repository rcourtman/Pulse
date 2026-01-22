/**
 * Anomaly severity color classes for Tailwind CSS.
 * Used by metric bar components to display anomaly indicators.
 */
export const anomalySeverityClass: Record<string, string> = {
  critical: 'text-red-400',
  high: 'text-orange-400',
  medium: 'text-yellow-400',
  low: 'text-blue-400',
};

/**
 * Get the appropriate CSS class for an anomaly severity level.
 * Falls back to yellow-400 if severity is unknown.
 */
export function getAnomalySeverityClass(severity: string): string {
  return anomalySeverityClass[severity] || 'text-yellow-400';
}
