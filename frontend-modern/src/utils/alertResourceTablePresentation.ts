export const ALERT_RESOURCE_TABLE_EMPTY_STATE = 'No resources available.';
export const ALERT_RESOURCE_TABLE_CUSTOM_BADGE_LABEL = 'Custom';
export const ALERT_RESOURCE_TABLE_METRIC_DISABLED_PLACEHOLDER = 'Off';
export const ALERT_RESOURCE_TABLE_OVERRIDE_NOTE_PLACEHOLDER =
  'Add a note about this override (optional)';
export const ALERT_RESOURCE_TABLE_EDIT_NOTE_PLACEHOLDER = 'Add a note...';
export const ALERT_RESOURCE_TABLE_RESET_FACTORY_DEFAULTS_LABEL =
  'Reset to factory defaults';
export const ALERT_RESOURCE_TABLE_REVERT_TO_DEFAULTS_LABEL = 'Revert to defaults';
export const ALERT_RESOURCE_TABLE_ALERT_DELAY_LABEL = 'Alert Delay (s)';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_LABEL = 'Off';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_LABEL = 'Warn';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL = 'Crit';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_TITLE =
  'Offline alerts disabled for this resource.';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_TITLE =
  'Offline alerts will raise warning-level notifications.';
export const ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE =
  'Offline alerts will raise critical-level notifications.';
export const ALERT_RESOURCE_TABLE_ENABLE_METRIC_TITLE = 'Click to enable this metric';
export const ALERT_RESOURCE_TABLE_DISABLE_METRIC_TITLE =
  'Set to -1 to disable alerts for this metric';
export const ALERT_RESOURCE_TABLE_EDIT_METRIC_TITLE = 'Click to edit this metric';

export function getAlertResourceTableEmptyState(emptyMessage?: string) {
  return emptyMessage || ALERT_RESOURCE_TABLE_EMPTY_STATE;
}

export function getAlertResourceTableNoResultsState(title: string) {
  return `No ${title.toLowerCase()} found`;
}

export function getAlertResourceTableCustomBadgeLabel() {
  return ALERT_RESOURCE_TABLE_CUSTOM_BADGE_LABEL;
}

export function getAlertResourceTableMetricPlaceholder(isDisabled: boolean) {
  return isDisabled ? ALERT_RESOURCE_TABLE_METRIC_DISABLED_PLACEHOLDER : '';
}

export function getAlertResourceTableOverrideNotePlaceholder() {
  return ALERT_RESOURCE_TABLE_OVERRIDE_NOTE_PLACEHOLDER;
}

export function getAlertResourceTableEditNotePlaceholder() {
  return ALERT_RESOURCE_TABLE_EDIT_NOTE_PLACEHOLDER;
}

export function getAlertResourceTableResetFactoryDefaultsLabel() {
  return ALERT_RESOURCE_TABLE_RESET_FACTORY_DEFAULTS_LABEL;
}

export function getAlertResourceTableRevertToDefaultsLabel() {
  return ALERT_RESOURCE_TABLE_REVERT_TO_DEFAULTS_LABEL;
}

export function getAlertResourceTableAlertDelayLabel() {
  return ALERT_RESOURCE_TABLE_ALERT_DELAY_LABEL;
}

export type AlertResourceTableOfflineState = 'off' | 'warning' | 'critical';

export function getAlertResourceTableOfflineStateOrder(): AlertResourceTableOfflineState[] {
  return ['off', 'warning', 'critical'];
}

export function getAlertResourceTableOfflineStatePresentation(
  state: AlertResourceTableOfflineState,
) {
  switch (state) {
    case 'off':
      return {
        label: ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_LABEL,
        className: 'bg-surface-alt text-muted hover:bg-surface-hover',
        title: ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_TITLE,
      } as const;
    case 'warning':
      return {
        label: ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_LABEL,
        className:
          'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800',
        title: ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_TITLE,
      } as const;
    case 'critical':
    default:
      return {
        label: ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL,
        className:
          'bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-900 dark:text-red-200 dark:hover:bg-red-800',
        title: ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE,
      } as const;
  }
}

export function getAlertResourceTableMetricInputTitle(isDisabled: boolean) {
  return isDisabled
    ? ALERT_RESOURCE_TABLE_ENABLE_METRIC_TITLE
    : ALERT_RESOURCE_TABLE_DISABLE_METRIC_TITLE;
}

export function getAlertResourceTableEditMetricTitle() {
  return ALERT_RESOURCE_TABLE_EDIT_METRIC_TITLE;
}
