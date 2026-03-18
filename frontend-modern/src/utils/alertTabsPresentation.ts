export interface AlertTabPresentationOptions {
  isActive: boolean;
  isDisabled: boolean;
  collapsed?: boolean;
}

export const ALERT_TAB_GROUP_STATUS_LABEL = 'Status';
export const ALERT_TAB_GROUP_CONFIGURATION_LABEL = 'Configuration';
export const ALERT_TAB_OVERVIEW_LABEL = 'Overview';
export const ALERT_TAB_HISTORY_LABEL = 'History';
export const ALERT_TAB_THRESHOLDS_LABEL = 'Thresholds';
export const ALERT_TAB_DESTINATIONS_LABEL = 'Notifications';
export const ALERT_TAB_SCHEDULE_LABEL = 'Schedule';

export function getAlertsTabGroups() {
  return [
    {
      id: 'status',
      label: ALERT_TAB_GROUP_STATUS_LABEL,
      items: [
        { id: 'overview', label: ALERT_TAB_OVERVIEW_LABEL },
        { id: 'history', label: ALERT_TAB_HISTORY_LABEL },
      ],
    },
    {
      id: 'configuration',
      label: ALERT_TAB_GROUP_CONFIGURATION_LABEL,
      items: [
        { id: 'thresholds', label: ALERT_TAB_THRESHOLDS_LABEL },
        { id: 'destinations', label: ALERT_TAB_DESTINATIONS_LABEL },
        { id: 'schedule', label: ALERT_TAB_SCHEDULE_LABEL },
      ],
    },
  ] as const;
}

export function getAlertsSidebarTabClass({
  isActive,
  isDisabled,
  collapsed = false,
}: AlertTabPresentationOptions): string {
  const layout = collapsed ? 'justify-center px-2 py-2.5' : 'gap-2.5 px-3 py-2';
  const tone = isDisabled
    ? 'cursor-not-allowed bg-surface-alt text-muted'
    : isActive
      ? 'bg-blue-50 text-blue-600 dark:bg-blue-900 dark:text-blue-200'
      : 'hover:bg-surface-hover hover:text-base-content';
  return `flex w-full items-center rounded-md text-sm font-medium transition-colors ${layout} ${tone}`;
}

export function getAlertsMobileTabClass({
  isActive,
  isDisabled,
}: AlertTabPresentationOptions): string {
  const tone = isDisabled
    ? 'cursor-not-allowed bg-surface-alt text-muted'
    : isActive
      ? 'bg-surface text-base-content shadow-sm'
      : 'text-muted hover:text-base-content';
  return `flex-1 min-w-0 rounded-md px-2 py-1.5 text-[11px] font-medium transition-all sm:px-4 sm:py-2 sm:text-xs ${tone}`;
}

export function getAlertsTabTitle({
  isDisabled,
  collapsed = false,
  label,
}: {
  isDisabled: boolean;
  collapsed?: boolean;
  label: string;
}): string | undefined {
  if (isDisabled) {
    return 'Enable alerts to configure this setting';
  }
  if (collapsed) {
    return label;
  }
  return undefined;
}
