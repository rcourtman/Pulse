import type { StorageAlertRowState } from './storageAlertState';

export interface StorageRowAlertPresentation {
  rowClass: string;
  rowStyle: Record<string, string>;
  dataAlertState: 'unacknowledged' | 'acknowledged' | 'none';
  dataAlertSeverity: string;
  dataResourceHighlighted: 'true' | 'false';
}

const BASE_ROW_CLASSES = ['transition-all duration-200', 'hover:bg-surface-hover'];

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
  } else if (options.isResourceHighlighted) {
    classes.push('bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600');
  } else if (hasAcknowledgedOnlyAlert) {
    classes.push('bg-surface-alt');
  }

  if (options.isExpanded) {
    classes.push('bg-surface-alt');
  }

  let rowStyle: Record<string, string> = {};
  if (showAlertHighlight) {
    rowStyle = {
      'box-shadow': `inset 4px 0 0 0 ${
        options.alertState.severity === 'critical' ? '#ef4444' : '#eab308'
      }`,
    };
  } else if (hasAcknowledgedOnlyAlert) {
    rowStyle = {
      'box-shadow': 'inset 4px 0 0 0 rgba(156, 163, 175, 0.8)',
    };
  }

  return {
    rowClass: classes.join(' '),
    rowStyle,
    dataAlertState: showAlertHighlight
      ? 'unacknowledged'
      : hasAcknowledgedOnlyAlert
        ? 'acknowledged'
        : 'none',
    dataAlertSeverity: options.alertState.severity || 'none',
    dataResourceHighlighted: options.isResourceHighlighted ? 'true' : 'false',
  };
};
