import { describe, expect, it } from 'vitest';
import {
  ALERT_TAB_DESTINATIONS_LABEL,
  ALERT_TAB_GROUP_CONFIGURATION_LABEL,
  ALERT_TAB_GROUP_STATUS_LABEL,
  ALERT_TAB_HISTORY_LABEL,
  ALERT_TAB_OVERVIEW_LABEL,
  ALERT_TAB_SCHEDULE_LABEL,
  ALERT_TAB_THRESHOLDS_LABEL,
  getAlertsTabGroups,
  getAlertsMobileTabClass,
  getAlertsSidebarTabClass,
  getAlertsTabTitle,
} from '@/utils/alertTabsPresentation';

describe('alertTabsPresentation', () => {
  it('returns active sidebar presentation', () => {
    expect(getAlertsSidebarTabClass({ isActive: true, isDisabled: false })).toBe(
      'flex w-full items-center rounded-md text-sm font-medium transition-colors gap-2.5 px-3 py-2 bg-blue-50 text-blue-600 dark:bg-blue-900 dark:text-blue-200',
    );
  });

  it('returns disabled mobile presentation', () => {
    expect(getAlertsMobileTabClass({ isActive: false, isDisabled: true })).toBe(
      'flex-1 min-w-0 rounded-md px-2 py-1.5 text-[11px] font-medium transition-all sm:px-4 sm:py-2 sm:text-xs cursor-not-allowed bg-surface-alt text-muted',
    );
  });

  it('returns the correct tab title', () => {
    expect(getAlertsTabTitle({ isDisabled: true, label: 'Overview' })).toBe(
      'Enable alerts to configure this setting',
    );
    expect(getAlertsTabTitle({ isDisabled: false, collapsed: true, label: 'Overview' })).toBe(
      'Overview',
    );
    expect(getAlertsTabTitle({ isDisabled: false, collapsed: false, label: 'Overview' })).toBe(
      undefined,
    );
  });

  it('exports canonical alerts tab groups and labels', () => {
    expect(ALERT_TAB_GROUP_STATUS_LABEL).toBe('Status');
    expect(ALERT_TAB_GROUP_CONFIGURATION_LABEL).toBe('Configuration');
    expect(ALERT_TAB_OVERVIEW_LABEL).toBe('Overview');
    expect(ALERT_TAB_HISTORY_LABEL).toBe('History');
    expect(ALERT_TAB_THRESHOLDS_LABEL).toBe('Thresholds');
    expect(ALERT_TAB_DESTINATIONS_LABEL).toBe('Notifications');
    expect(ALERT_TAB_SCHEDULE_LABEL).toBe('Schedule');
    expect(getAlertsTabGroups()).toEqual([
      {
        id: 'status',
        label: 'Status',
        items: [
          { id: 'overview', label: 'Overview' },
          { id: 'history', label: 'History' },
        ],
      },
      {
        id: 'configuration',
        label: 'Configuration',
        items: [
          { id: 'thresholds', label: 'Thresholds' },
          { id: 'destinations', label: 'Notifications' },
          { id: 'schedule', label: 'Schedule' },
        ],
      },
    ]);
  });
});
