import { t } from '@/i18n';
import type { AlertTab } from '@/features/alerts/types';

export interface AlertTabPresentationOptions {
  isActive: boolean;
  isDisabled: boolean;
  collapsed?: boolean;
}

export interface AlertTabGroup {
  id: 'status' | 'configuration';
  label: string;
  items: { id: AlertTab; label: string }[];
}

export const ALERT_TAB_GROUP_STATUS_LABEL = 'Status';
export const ALERT_TAB_GROUP_CONFIGURATION_LABEL = 'Configuration';
export const ALERT_TAB_OVERVIEW_LABEL = 'Overview';
export const ALERT_TAB_HISTORY_LABEL = 'History';
export const ALERT_TAB_THRESHOLDS_LABEL = 'Thresholds';
export const ALERT_TAB_DESTINATIONS_LABEL = 'Notifications';
export const ALERT_TAB_SCHEDULE_LABEL = 'Schedule';

function getLocalizedAlertTabGroups(): AlertTabGroup[] {
  return [
    {
      id: 'status',
      label: t('alerts.tabs.group.status'),
      items: [
        { id: 'overview', label: t('alerts.tabs.overview') },
        { id: 'history', label: t('alerts.tabs.history') },
      ],
    },
    {
      id: 'configuration',
      label: t('alerts.tabs.group.configuration'),
      items: [
        { id: 'thresholds', label: t('alerts.tabs.thresholds') },
        { id: 'destinations', label: t('alerts.tabs.destinations') },
        { id: 'schedule', label: t('alerts.tabs.schedule') },
      ],
    },
  ];
}

export function isAlertsConfigurationTab(tab: AlertTab): boolean {
  return tab === 'thresholds' || tab === 'destinations' || tab === 'schedule';
}

export function getAlertsTabGroups(options?: { readOnly?: boolean }): AlertTabGroup[] {
  const groups = getLocalizedAlertTabGroups();
  if (options?.readOnly) {
    return groups
      .filter((group) => group.id === 'status')
      .map((group) => ({
        ...group,
        items: [...group.items],
      }));
  }
  return groups.map((group) => ({
    ...group,
    items: [...group.items],
  }));
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
  return `flex-shrink-0 whitespace-nowrap rounded-md px-3 py-1.5 text-[11px] font-medium transition-all sm:flex-1 sm:min-w-0 sm:px-4 sm:py-2 sm:text-xs ${tone}`;
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
    return t('alerts.tabs.disabledTitle');
  }
  if (collapsed) {
    return label;
  }
  return undefined;
}
