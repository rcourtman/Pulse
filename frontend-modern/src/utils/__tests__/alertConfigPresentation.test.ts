import { describe, expect, it } from 'vitest';
import {
  ALERT_CONFIG_COOLDOWN_DESCRIPTION,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_HELP,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_SUFFIX,
  ALERT_CONFIG_COOLDOWN_PERIOD_HELP,
  ALERT_CONFIG_COOLDOWN_PERIOD_LABEL,
  ALERT_CONFIG_COOLDOWN_PERIOD_SUFFIX,
  ALERT_CONFIG_COOLDOWN_TITLE,
  ALERT_CONFIG_ESCALATION_DESCRIPTION,
  ALERT_CONFIG_ESCALATION_NOTIFY_ALL,
  ALERT_CONFIG_ESCALATION_NOTIFY_EMAIL,
  ALERT_CONFIG_ESCALATION_NOTIFY_WEBHOOKS,
  ALERT_CONFIG_ESCALATION_TITLE,
  ALERT_CONFIG_GROUPING_BY_GUEST,
  ALERT_CONFIG_GROUPING_BY_NODE,
  ALERT_CONFIG_GROUPING_DESCRIPTION,
  ALERT_CONFIG_GROUPING_STRATEGY_LABEL,
  ALERT_CONFIG_GROUPING_WINDOW_HELP,
  ALERT_CONFIG_GROUPING_WINDOW_LABEL,
  ALERT_CONFIG_GROUPING_TITLE,
  ALERT_CONFIG_DISCARDED_SUCCESS,
  ALERT_CONFIG_DISCARDING_LABEL,
  ALERT_CONFIG_DISCARD_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TITLE,
  ALERT_CONFIG_QUIET_HOURS_DESCRIPTION,
  ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL,
  ALERT_CONFIG_RELOAD_FAILURE,
  ALERT_CONFIG_RECOVERY_HELP,
  ALERT_CONFIG_RECOVERY_DESCRIPTION,
  ALERT_CONFIG_RECOVERY_TITLE,
  ALERT_CONFIG_RESET_DEFAULTS,
  ALERT_CONFIG_RESET_DEFAULTS_TITLE,
  ALERT_CONFIG_SAVE_FAILURE,
  ALERT_CONFIG_SAVE_SUCCESS,
  ALERT_CONFIG_SAVE_CHANGES,
  ALERT_CONFIG_SCHEDULING_DESCRIPTION,
  ALERT_CONFIG_SCHEDULING_TITLE,
  ALERT_CONFIG_SUMMARY_ALL_DISABLED,
  ALERT_CONFIG_SUMMARY_DESCRIPTION,
  ALERT_CONFIG_SUMMARY_TITLE,
  ALERT_CONFIG_SUMMARY_RECOVERY,
  ALERT_CONFIG_TOGGLE_DISABLED,
  ALERT_CONFIG_TOGGLE_ENABLED,
  ALERT_CONFIG_UNSAVED_CHANGES,
  getAlertConfigDiscardedSuccess,
  getAlertConfigDiscardLabel,
  getAlertConfigEscalationHelp,
  getAlertConfigEscalationNotifyLabel,
  getAlertConfigQuietHourSuppressOptions,
  getAlertConfigLeaveConfirmation,
  getAlertConfigReloadFailure,
  getAlertConfigRecoveryHelp,
  getAlertConfigResetDefaultsLabel,
  getAlertConfigResetDefaultsTitle,
  getAlertConfigSaveFailure,
  getAlertConfigSaveChangesLabel,
  getAlertConfigSaveSuccess,
  getAlertConfigSummaryCooldown,
  getAlertConfigSummaryEscalation,
  getAlertConfigSummaryGrouping,
  getAlertConfigSummaryAllDisabled,
  getAlertConfigSummaryQuietHours,
  getAlertConfigSummaryRecoveryEnabled,
  getAlertConfigSummarySuppressing,
  getAlertConfigToggleStatusLabel,
  getAlertConfigUnsavedChangesLabel,
} from '@/utils/alertConfigPresentation';

describe('alertConfigPresentation', () => {
  it('returns canonical alert config shell vocabulary', () => {
    expect(ALERT_CONFIG_UNSAVED_CHANGES).toBe('You have unsaved changes');
    expect(ALERT_CONFIG_SAVE_CHANGES).toBe('Save Changes');
    expect(ALERT_CONFIG_RESET_DEFAULTS).toBe('Reset to defaults');
    expect(ALERT_CONFIG_RESET_DEFAULTS_TITLE).toBe(
      'Restore quiet hours, cooldown, grouping, and escalation settings to their defaults',
    );
    expect(ALERT_CONFIG_SCHEDULING_TITLE).toBe('Alert scheduling');
    expect(ALERT_CONFIG_SCHEDULING_DESCRIPTION).toBe(
      'Configure when and how alerts are delivered',
    );
    expect(ALERT_CONFIG_QUIET_HOURS_TITLE).toBe('Quiet hours');
    expect(ALERT_CONFIG_QUIET_HOURS_DESCRIPTION).toBe(
      'Pause non-critical alerts during specific times.',
    );
    expect(ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL).toBe('Start time');
    expect(ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL).toBe('End time');
    expect(ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL).toBe('Timezone');
    expect(ALERT_CONFIG_COOLDOWN_TITLE).toBe('Alert cooldown');
    expect(ALERT_CONFIG_COOLDOWN_DESCRIPTION).toBe(
      'Limit alert frequency to prevent spam.',
    );
    expect(ALERT_CONFIG_COOLDOWN_PERIOD_LABEL).toBe('Cooldown period');
    expect(ALERT_CONFIG_COOLDOWN_PERIOD_SUFFIX).toBe('minutes');
    expect(ALERT_CONFIG_COOLDOWN_PERIOD_HELP).toBe(
      'Minimum time between alerts for the same issue',
    );
    expect(ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL).toBe('Max alerts / hour');
    expect(ALERT_CONFIG_COOLDOWN_MAX_ALERTS_SUFFIX).toBe('alerts');
    expect(ALERT_CONFIG_COOLDOWN_MAX_ALERTS_HELP).toBe('Per guest/metric combination');
    expect(ALERT_CONFIG_GROUPING_TITLE).toBe('Smart grouping');
    expect(ALERT_CONFIG_GROUPING_DESCRIPTION).toBe('Bundle similar alerts together.');
    expect(ALERT_CONFIG_GROUPING_WINDOW_LABEL).toBe('Grouping window');
    expect(ALERT_CONFIG_GROUPING_WINDOW_HELP).toBe(
      'Alerts within this window are grouped together. Set to 0 to send immediately.',
    );
    expect(ALERT_CONFIG_GROUPING_STRATEGY_LABEL).toBe('Grouping strategy');
    expect(ALERT_CONFIG_GROUPING_BY_NODE).toBe('By Node');
    expect(ALERT_CONFIG_GROUPING_BY_GUEST).toBe('By Guest');
    expect(getAlertConfigQuietHourSuppressOptions()).toEqual([
      {
        key: 'performance',
        label: 'Performance alerts',
        description: 'CPU, memory, disk, and network thresholds stay quiet.',
      },
      {
        key: 'storage',
        label: 'Storage alerts',
        description: 'Silence storage usage, disk health, and ZFS events.',
      },
      {
        key: 'offline',
        label: 'Offline & power state',
        description: 'Skip connectivity and powered-off alerts during backups.',
      },
    ]);
    expect(ALERT_CONFIG_RECOVERY_TITLE).toBe('Recovery notifications');
    expect(ALERT_CONFIG_RECOVERY_DESCRIPTION).toBe(
      'Send a follow-up when an alert returns to normal.',
    );
    expect(ALERT_CONFIG_ESCALATION_TITLE).toBe('Alert escalation');
    expect(ALERT_CONFIG_ESCALATION_DESCRIPTION).toBe(
      'Notify additional contacts for persistent issues.',
    );
    expect(ALERT_CONFIG_RECOVERY_HELP).toBe(
      'Sends on the same channels as live alerts to confirm when a condition clears.',
    );
    expect(ALERT_CONFIG_ESCALATION_NOTIFY_EMAIL).toBe('Email');
    expect(ALERT_CONFIG_ESCALATION_NOTIFY_WEBHOOKS).toBe('Webhooks');
    expect(ALERT_CONFIG_ESCALATION_NOTIFY_ALL).toBe('All Channels');
    expect(ALERT_CONFIG_SUMMARY_TITLE).toBe('Configuration summary');
    expect(ALERT_CONFIG_SUMMARY_DESCRIPTION).toBe(
      'Preview of the active schedule settings.',
    );
    expect(ALERT_CONFIG_SUMMARY_ALL_DISABLED).toBe(
      '• All notification controls are disabled - alerts will be sent immediately',
    );
    expect(ALERT_CONFIG_DISCARDED_SUCCESS).toBe('Changes discarded');
    expect(ALERT_CONFIG_RELOAD_FAILURE).toBe('Failed to reload configuration');
    expect(ALERT_CONFIG_SAVE_SUCCESS).toBe('Configuration saved successfully!');
    expect(ALERT_CONFIG_SAVE_FAILURE).toBe('Failed to save configuration');
    expect(ALERT_CONFIG_DISCARD_LABEL).toBe('Discard');
    expect(ALERT_CONFIG_DISCARDING_LABEL).toBe('Discarding...');
    expect(ALERT_CONFIG_TOGGLE_ENABLED).toBe('Enabled');
    expect(ALERT_CONFIG_TOGGLE_DISABLED).toBe('Disabled');
    expect(getAlertConfigUnsavedChangesLabel()).toBe('You have unsaved changes');
    expect(getAlertConfigSaveChangesLabel()).toBe('Save Changes');
    expect(getAlertConfigResetDefaultsLabel()).toBe('Reset to defaults');
    expect(getAlertConfigResetDefaultsTitle()).toBe(
      'Restore quiet hours, cooldown, grouping, and escalation settings to their defaults',
    );
    expect(getAlertConfigDiscardedSuccess()).toBe('Changes discarded');
    expect(getAlertConfigReloadFailure()).toBe('Failed to reload configuration');
    expect(getAlertConfigSaveSuccess()).toBe('Configuration saved successfully!');
    expect(getAlertConfigSaveFailure()).toBe('Failed to save configuration');
    expect(getAlertConfigDiscardLabel(false)).toBe('Discard');
    expect(getAlertConfigDiscardLabel(true)).toBe('Discarding...');
    expect(getAlertConfigLeaveConfirmation()).toBe(
      'You have unsaved changes that will be lost. Discard changes and leave?',
    );
    expect(getAlertConfigToggleStatusLabel(true)).toBe('Enabled');
    expect(getAlertConfigToggleStatusLabel(false)).toBe('Disabled');
    expect(getAlertConfigSummaryQuietHours('22:00', '07:00', 'Europe/London')).toBe(
      '• Quiet hours active from 22:00 to 07:00 (Europe/London)',
    );
    expect(getAlertConfigSummarySuppressing(['Performance alerts', 'Storage alerts'])).toBe(
      '• Suppressing Performance alerts, Storage alerts during quiet hours',
    );
    expect(getAlertConfigSummaryCooldown(15, 4)).toBe(
      '• 15 minute cooldown between alerts, max 4 alerts per hour',
    );
    expect(getAlertConfigSummaryGrouping(10, true, false)).toBe(
      '• Grouping alerts within 10 minute windows by node',
    );
    expect(getAlertConfigRecoveryHelp()).toBe(
      'Sends on the same channels as live alerts to confirm when a condition clears.',
    );
    expect(getAlertConfigEscalationHelp()).toBe(
      'Define escalation levels for unresolved alerts:',
    );
    expect(getAlertConfigEscalationNotifyLabel('email')).toBe('Email');
    expect(getAlertConfigEscalationNotifyLabel('webhook')).toBe('Webhooks');
    expect(getAlertConfigEscalationNotifyLabel('all')).toBe('All Channels');
    expect(getAlertConfigSummaryRecoveryEnabled()).toBe(
      ALERT_CONFIG_SUMMARY_RECOVERY,
    );
    expect(getAlertConfigSummaryEscalation(2)).toBe('• 2 escalation levels configured');
    expect(getAlertConfigSummaryAllDisabled()).toBe(
      '• All notification controls are disabled - alerts will be sent immediately',
    );
  });
});
