import type { Alert } from '@/types/api';
import type { StorageAlertRowState } from './storageAlertState';

export interface StorageRowAlertPresentation {
  rowClass: string;
  dataAlertState: 'unacknowledged' | 'acknowledged' | 'none';
  dataAlertSeverity: string;
  dataResourceHighlighted: 'true' | 'false';
}

const BASE_ROW_CLASSES = ['transition-all duration-200', 'hover:bg-surface-hover'];
const STORAGE_ROW_CRITICAL_ALERT_ACCENT_CLASS = 'shadow-[inset_4px_0_0_0_#ef4444]';
const STORAGE_ROW_WARNING_ALERT_ACCENT_CLASS = 'shadow-[inset_4px_0_0_0_#eab308]';
const STORAGE_ROW_ACKNOWLEDGED_ALERT_ACCENT_CLASS =
  'shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]';

export const getStorageRowAlertPresentation = (options: {
  alertState: StorageAlertRowState;
  parentNodeOnline: boolean;
  isExpanded: boolean;
  isResourceHighlighted: boolean;
}): StorageRowAlertPresentation => {
  const showAlertHighlight = options.alertState.hasUnacknowledgedAlert && options.parentNodeOnline;
  const hasAcknowledgedOnlyAlert =
    options.alertState.hasAcknowledgedOnlyAlert && options.parentNodeOnline;

  const classes = [...BASE_ROW_CLASSES];
  if (showAlertHighlight) {
    classes.push(
      options.alertState.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/25'
        : 'bg-yellow-50 dark:bg-yellow-950/25',
    );
    classes.push(
      options.alertState.severity === 'critical'
        ? STORAGE_ROW_CRITICAL_ALERT_ACCENT_CLASS
        : STORAGE_ROW_WARNING_ALERT_ACCENT_CLASS,
    );
  } else if (options.isResourceHighlighted) {
    classes.push('bg-blue-50 dark:bg-blue-900/25 ring-1 ring-blue-300 dark:ring-blue-600');
  } else if (hasAcknowledgedOnlyAlert) {
    classes.push('bg-surface-alt', STORAGE_ROW_ACKNOWLEDGED_ALERT_ACCENT_CLASS);
  }

  if (options.isExpanded) {
    classes.push('bg-surface-alt');
  }

  return {
    rowClass: classes.join(' '),
    dataAlertState: showAlertHighlight
      ? 'unacknowledged'
      : hasAcknowledgedOnlyAlert
        ? 'acknowledged'
        : 'none',
    dataAlertSeverity: options.alertState.severity || 'none',
    dataResourceHighlighted: options.isResourceHighlighted ? 'true' : 'false',
  };
};

const forecastDaysToFull = (alert: Alert): number | null => {
  const value = alert.metadata?.forecastDaysToFull;
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null;
};

// One short reason for a highlighted storage row. A fill forecast says when
// the pool runs out; a usage threshold says which limit was crossed (the bar
// already shows the percentage); anything else falls back to the alert text.
// The compact form fits the phone layout's narrow State column.
export const describeStorageAlertHeadline = (
  alert: Alert,
  options: { compact?: boolean } = {},
): string => {
  const days = forecastDaysToFull(alert);
  if (days !== null) {
    if (days < 1) return options.compact ? 'Full <1d' : 'Full within a day';
    const rounded = Math.max(1, Math.round(days));
    if (options.compact) return `Full in ~${rounded}d`;
    return `Full in ~${rounded} ${rounded === 1 ? 'day' : 'days'}`;
  }
  const status = alert.type === 'usage' ? alert.metricStatus : undefined;
  if (status && status.phase !== 'breaching') {
    // Usage has dropped back under the limit but the alert holds until it
    // reaches the clear level, so "Over" would contradict the bar beside it.
    const recovery = `${Math.round(status.recovery)}%`;
    if (status.phase === 'recovering') {
      return options.compact ? 'Recovering' : `Recovering, clears at ${recovery} or lower`;
    }
    return options.compact
      ? `Clears ≤${recovery}`
      : `Under ${Math.round(status.trigger)}% limit, clears at ${recovery} or lower`;
  }
  if (alert.type === 'usage' && alert.threshold > 0 && alert.threshold < 100) {
    return options.compact ? `Over ${alert.threshold}%` : `Over ${alert.threshold}% usage limit`;
  }
  return alert.message?.trim() || 'Active alert';
};

const alertSeverityRank = (level: string | undefined): number =>
  level === 'critical' ? 3 : level === 'warning' ? 2 : level === 'info' ? 1 : 0;

// The open alert that explains the row: most severe first, then the one
// that started most recently so a fresh forecast outranks an old notice.
export const pickStorageHeadlineAlert = (alerts: Alert[]): Alert | null => {
  const open = alerts.filter((alert) => !alert.acknowledged);
  if (open.length === 0) return null;
  return [...open].sort((a, b) => {
    const severity = alertSeverityRank(b.level) - alertSeverityRank(a.level);
    if (severity !== 0) return severity;
    return Date.parse(b.startTime) - Date.parse(a.startTime);
  })[0];
};
