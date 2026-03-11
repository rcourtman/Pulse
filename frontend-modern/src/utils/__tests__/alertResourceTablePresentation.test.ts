import { describe, expect, it } from 'vitest';
import {
  ALERT_RESOURCE_TABLE_ALERT_DELAY_LABEL,
  ALERT_RESOURCE_TABLE_CUSTOM_BADGE_LABEL,
  ALERT_RESOURCE_TABLE_EDIT_NOTE_PLACEHOLDER,
  ALERT_RESOURCE_TABLE_EMPTY_STATE,
  ALERT_RESOURCE_TABLE_METRIC_DISABLED_PLACEHOLDER,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_LABEL,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_TITLE,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_LABEL,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_TITLE,
  ALERT_RESOURCE_TABLE_OVERRIDE_NOTE_PLACEHOLDER,
  ALERT_RESOURCE_TABLE_RESET_FACTORY_DEFAULTS_LABEL,
  ALERT_RESOURCE_TABLE_REVERT_TO_DEFAULTS_LABEL,
  getAlertResourceTableAlertDelayLabel,
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditMetricTitle,
  getAlertResourceTableEditNotePlaceholder,
  getAlertResourceTableEmptyState,
  getAlertResourceTableMetricInputTitle,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableNoResultsState,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableOverrideNotePlaceholder,
  getAlertResourceTableResetFactoryDefaultsLabel,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';

describe('alertResourceTablePresentation', () => {
  it('uses the canonical default empty-state copy', () => {
    expect(ALERT_RESOURCE_TABLE_EMPTY_STATE).toBe('No resources available.');
    expect(getAlertResourceTableEmptyState()).toBe('No resources available.');
  });

  it('prefers a caller-provided empty message', () => {
    expect(getAlertResourceTableEmptyState('Nothing configured')).toBe('Nothing configured');
  });

  it('formats the canonical no-results message from the table title', () => {
    expect(getAlertResourceTableNoResultsState('Guests')).toBe('No guests found');
  });

  it('returns canonical resource-table operational vocabulary', () => {
    expect(ALERT_RESOURCE_TABLE_CUSTOM_BADGE_LABEL).toBe('Custom');
    expect(ALERT_RESOURCE_TABLE_METRIC_DISABLED_PLACEHOLDER).toBe('Off');
    expect(ALERT_RESOURCE_TABLE_OVERRIDE_NOTE_PLACEHOLDER).toBe(
      'Add a note about this override (optional)',
    );
    expect(ALERT_RESOURCE_TABLE_EDIT_NOTE_PLACEHOLDER).toBe('Add a note...');
    expect(ALERT_RESOURCE_TABLE_RESET_FACTORY_DEFAULTS_LABEL).toBe(
      'Reset to factory defaults',
    );
    expect(ALERT_RESOURCE_TABLE_REVERT_TO_DEFAULTS_LABEL).toBe('Revert to defaults');
    expect(ALERT_RESOURCE_TABLE_ALERT_DELAY_LABEL).toBe('Alert Delay (s)');
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_LABEL).toBe('Off');
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_LABEL).toBe('Warn');
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL).toBe('Crit');
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_OFF_TITLE).toBe(
      'Offline alerts disabled for this resource.',
    );
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_WARNING_TITLE).toBe(
      'Offline alerts will raise warning-level notifications.',
    );
    expect(ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE).toBe(
      'Offline alerts will raise critical-level notifications.',
    );
    expect(getAlertResourceTableCustomBadgeLabel()).toBe('Custom');
    expect(getAlertResourceTableMetricPlaceholder(true)).toBe('Off');
    expect(getAlertResourceTableMetricPlaceholder(false)).toBe('');
    expect(getAlertResourceTableOverrideNotePlaceholder()).toBe(
      'Add a note about this override (optional)',
    );
    expect(getAlertResourceTableEditNotePlaceholder()).toBe('Add a note...');
    expect(getAlertResourceTableResetFactoryDefaultsLabel()).toBe(
      'Reset to factory defaults',
    );
    expect(getAlertResourceTableRevertToDefaultsLabel()).toBe('Revert to defaults');
    expect(getAlertResourceTableAlertDelayLabel()).toBe('Alert Delay (s)');
    expect(getAlertResourceTableOfflineStateOrder()).toEqual(['off', 'warning', 'critical']);
    expect(getAlertResourceTableOfflineStatePresentation('off')).toEqual({
      label: 'Off',
      className: 'bg-surface-alt text-muted hover:bg-surface-hover',
      title: 'Offline alerts disabled for this resource.',
    });
    expect(getAlertResourceTableOfflineStatePresentation('warning')).toEqual({
      label: 'Warn',
      className:
        'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800',
      title: 'Offline alerts will raise warning-level notifications.',
    });
    expect(getAlertResourceTableOfflineStatePresentation('critical')).toEqual({
      label: 'Crit',
      className:
        'bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-900 dark:text-red-200 dark:hover:bg-red-800',
      title: 'Offline alerts will raise critical-level notifications.',
    });
    expect(getAlertResourceTableMetricInputTitle(true)).toBe(
      'Click to enable this metric',
    );
    expect(getAlertResourceTableMetricInputTitle(false)).toBe(
      'Set to -1 to disable alerts for this metric',
    );
    expect(getAlertResourceTableEditMetricTitle()).toBe('Click to edit this metric');
  });
});
