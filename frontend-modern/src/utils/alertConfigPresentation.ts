export const ALERT_CONFIG_UNSAVED_CHANGES = 'You have unsaved changes';
export const ALERT_CONFIG_SAVE_CHANGES = 'Save Changes';
export const ALERT_CONFIG_RESET_DEFAULTS = 'Reset to defaults';
export const ALERT_CONFIG_RESET_DEFAULTS_TITLE =
  'Restore quiet hours, cooldown, grouping, and escalation settings to their defaults';
export const ALERT_CONFIG_LEAVE_CONFIRMATION =
  'You have unsaved changes that will be lost. Discard changes and leave?';

export const ALERT_CONFIG_SCHEDULING_TITLE = 'Alert scheduling';
export const ALERT_CONFIG_SCHEDULING_DESCRIPTION = 'Configure when and how alerts are delivered';
export const ALERT_CONFIG_QUIET_HOURS_TITLE = 'Quiet hours';
export const ALERT_CONFIG_QUIET_HOURS_DESCRIPTION =
  'Pause non-critical alerts during specific times.';
export const ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL = 'Start time';
export const ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL = 'End time';
export const ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL = 'Timezone';
export const ALERT_CONFIG_COOLDOWN_TITLE = 'Alert cooldown';
export const ALERT_CONFIG_COOLDOWN_DESCRIPTION = 'Limit alert frequency to prevent spam.';
export const ALERT_CONFIG_COOLDOWN_PERIOD_LABEL = 'Cooldown period';
export const ALERT_CONFIG_COOLDOWN_PERIOD_SUFFIX = 'minutes';
export const ALERT_CONFIG_COOLDOWN_PERIOD_HELP =
  'Minimum time between alerts for the same issue';
export const ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL = 'Max alerts / hour';
export const ALERT_CONFIG_COOLDOWN_MAX_ALERTS_SUFFIX = 'alerts';
export const ALERT_CONFIG_COOLDOWN_MAX_ALERTS_HELP = 'Per guest/metric combination';
export const ALERT_CONFIG_GROUPING_TITLE = 'Smart grouping';
export const ALERT_CONFIG_GROUPING_DESCRIPTION = 'Bundle similar alerts together.';
export const ALERT_CONFIG_GROUPING_WINDOW_LABEL = 'Grouping window';
export const ALERT_CONFIG_GROUPING_WINDOW_HELP =
  'Alerts within this window are grouped together. Set to 0 to send immediately.';
export const ALERT_CONFIG_GROUPING_STRATEGY_LABEL = 'Grouping strategy';
export const ALERT_CONFIG_GROUPING_BY_NODE = 'By Node';
export const ALERT_CONFIG_GROUPING_BY_GUEST = 'By Guest';
export const ALERT_CONFIG_QUIET_HOUR_SUPPRESS_OPTIONS = [
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
] as const;
export const ALERT_CONFIG_RECOVERY_TITLE = 'Recovery notifications';
export const ALERT_CONFIG_RECOVERY_DESCRIPTION =
  'Send a follow-up when an alert returns to normal.';
export const ALERT_CONFIG_ESCALATION_TITLE = 'Alert escalation';
export const ALERT_CONFIG_ESCALATION_DESCRIPTION =
  'Notify additional contacts for persistent issues.';
export const ALERT_CONFIG_SUMMARY_QUIET_HOURS_PREFIX = '• Quiet hours active from';
export const ALERT_CONFIG_SUMMARY_SUPPRESSING_PREFIX = '• Suppressing';
export const ALERT_CONFIG_SUMMARY_SUPPRESSING_SUFFIX = 'during quiet hours';
export const ALERT_CONFIG_SUMMARY_COOLDOWN_SUFFIX = 'alerts per hour';
export const ALERT_CONFIG_SUMMARY_GROUPING_PREFIX = '• Grouping alerts within';
export const ALERT_CONFIG_SUMMARY_RECOVERY = '• Recovery notifications enabled when alerts clear';
export const ALERT_CONFIG_RECOVERY_HELP =
  'Sends on the same channels as live alerts to confirm when a condition clears.';
export const ALERT_CONFIG_ESCALATION_HELP =
  'Define escalation levels for unresolved alerts:';
export const ALERT_CONFIG_ESCALATION_AFTER_LABEL = 'After';
export const ALERT_CONFIG_ESCALATION_NOTIFY_LABEL = 'Notify';
export const ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX = 'min';
export const ALERT_CONFIG_ESCALATION_NOTIFY_EMAIL = 'Email';
export const ALERT_CONFIG_ESCALATION_NOTIFY_WEBHOOKS = 'Webhooks';
export const ALERT_CONFIG_ESCALATION_NOTIFY_ALL = 'All Channels';
export const ALERT_CONFIG_ESCALATION_REMOVE_TITLE = 'Remove escalation level';
export const ALERT_CONFIG_ESCALATION_ADD_LABEL = 'Add Escalation Level';
export const ALERT_CONFIG_SUMMARY_TITLE = 'Configuration summary';
export const ALERT_CONFIG_SUMMARY_DESCRIPTION = 'Preview of the active schedule settings.';
export const ALERT_CONFIG_SUMMARY_ALL_DISABLED =
  '• All notification controls are disabled - alerts will be sent immediately';
export const ALERT_CONFIG_DISCARDED_SUCCESS = 'Changes discarded';
export const ALERT_CONFIG_RELOAD_FAILURE = 'Failed to reload configuration';
export const ALERT_CONFIG_SAVE_SUCCESS = 'Configuration saved successfully!';
export const ALERT_CONFIG_SAVE_FAILURE = 'Failed to save configuration';
export const ALERT_CONFIG_DISCARD_LABEL = 'Discard';
export const ALERT_CONFIG_DISCARDING_LABEL = 'Discarding...';
export const ALERT_CONFIG_SWARM_GAP_VALIDATION =
  'Swarm service critical gap must be greater than or equal to the warning gap when enabled.';

export const ALERT_CONFIG_TOGGLE_ENABLED = 'Enabled';
export const ALERT_CONFIG_TOGGLE_DISABLED = 'Disabled';

export function getAlertConfigUnsavedChangesLabel() {
  return ALERT_CONFIG_UNSAVED_CHANGES;
}

export function getAlertConfigSaveChangesLabel() {
  return ALERT_CONFIG_SAVE_CHANGES;
}

export function getAlertConfigResetDefaultsLabel() {
  return ALERT_CONFIG_RESET_DEFAULTS;
}

export function getAlertConfigResetDefaultsTitle() {
  return ALERT_CONFIG_RESET_DEFAULTS_TITLE;
}

export function getAlertConfigLeaveConfirmation() {
  return ALERT_CONFIG_LEAVE_CONFIRMATION;
}

export function getAlertConfigToggleStatusLabel(enabled: boolean) {
  return enabled ? ALERT_CONFIG_TOGGLE_ENABLED : ALERT_CONFIG_TOGGLE_DISABLED;
}

export function getAlertConfigSummaryQuietHours(start: string, end: string, timezone: string) {
  return `${ALERT_CONFIG_SUMMARY_QUIET_HOURS_PREFIX} ${start} to ${end} (${timezone})`;
}

export function getAlertConfigSummarySuppressing(items: string[]) {
  return `${ALERT_CONFIG_SUMMARY_SUPPRESSING_PREFIX} ${items.join(', ')} ${ALERT_CONFIG_SUMMARY_SUPPRESSING_SUFFIX}`;
}

export function getAlertConfigSummaryCooldown(minutes: number, maxAlerts: number) {
  return `• ${minutes} minute cooldown between alerts, max ${maxAlerts} ${ALERT_CONFIG_SUMMARY_COOLDOWN_SUFFIX}`;
}

export function getAlertConfigSummaryGrouping(
  windowMinutes: number,
  byNode: boolean,
  byGuest: boolean,
) {
  const groupingTargets = [byNode && 'node', byGuest && 'guest'].filter(Boolean).join(' and ');
  return groupingTargets
    ? `${ALERT_CONFIG_SUMMARY_GROUPING_PREFIX} ${windowMinutes} minute windows by ${groupingTargets}`
    : `${ALERT_CONFIG_SUMMARY_GROUPING_PREFIX} ${windowMinutes} minute windows`;
}

export function getAlertConfigSummaryRecoveryEnabled() {
  return ALERT_CONFIG_SUMMARY_RECOVERY;
}

export function getAlertConfigSummaryEscalation(levelCount: number) {
  return `• ${levelCount} escalation level${levelCount === 1 ? '' : 's'} configured`;
}

export function getAlertConfigRecoveryHelp() {
  return ALERT_CONFIG_RECOVERY_HELP;
}

export function getAlertConfigEscalationHelp() {
  return ALERT_CONFIG_ESCALATION_HELP;
}

export function getAlertConfigEscalationNotifyLabel(
  type: 'email' | 'webhook' | 'all',
) {
  switch (type) {
    case 'email':
      return ALERT_CONFIG_ESCALATION_NOTIFY_EMAIL;
    case 'webhook':
      return ALERT_CONFIG_ESCALATION_NOTIFY_WEBHOOKS;
    default:
      return ALERT_CONFIG_ESCALATION_NOTIFY_ALL;
  }
}

export function getAlertConfigSummaryAllDisabled() {
  return ALERT_CONFIG_SUMMARY_ALL_DISABLED;
}

export function getAlertConfigDiscardedSuccess() {
  return ALERT_CONFIG_DISCARDED_SUCCESS;
}

export function getAlertConfigReloadFailure() {
  return ALERT_CONFIG_RELOAD_FAILURE;
}

export function getAlertConfigSaveSuccess() {
  return ALERT_CONFIG_SAVE_SUCCESS;
}

export function getAlertConfigSaveFailure() {
  return ALERT_CONFIG_SAVE_FAILURE;
}

export function getAlertConfigDiscardLabel(isReloading: boolean) {
  return isReloading ? ALERT_CONFIG_DISCARDING_LABEL : ALERT_CONFIG_DISCARD_LABEL;
}

export function getAlertConfigSwarmGapValidationError() {
  return ALERT_CONFIG_SWARM_GAP_VALIDATION;
}

export function getAlertConfigQuietHourSuppressOptions() {
  return ALERT_CONFIG_QUIET_HOUR_SUPPRESS_OPTIONS;
}
