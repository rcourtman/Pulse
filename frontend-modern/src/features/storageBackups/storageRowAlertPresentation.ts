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
  const showAlertHighlight =
    options.alertState.hasUnacknowledgedAlert && options.parentNodeOnline;
  const hasAcknowledgedOnlyAlert =
    options.alertState.hasAcknowledgedOnlyAlert && options.parentNodeOnline;

  const classes = [...BASE_ROW_CLASSES];
  if (showAlertHighlight) {
    classes.push(
      options.alertState.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950'
        : 'bg-yellow-50 dark:bg-yellow-950',
    );
    classes.push(
      options.alertState.severity === 'critical'
        ? STORAGE_ROW_CRITICAL_ALERT_ACCENT_CLASS
        : STORAGE_ROW_WARNING_ALERT_ACCENT_CLASS,
    );
  } else if (options.isResourceHighlighted) {
    classes.push('bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600');
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
