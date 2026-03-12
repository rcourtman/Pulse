export const ALERTS_EMPTY_STATE = 'No active alerts';
export const ALERTS_THRESHOLD_HINT = 'Alerts will appear here when thresholds are exceeded';
export const ALERT_TIMELINE_LOADING_STATE = 'Loading timeline...';
export const ALERT_TIMELINE_FILTER_EMPTY_STATE =
  'No timeline events match the selected filters.';
export const ALERT_TIMELINE_EMPTY_STATE = 'No timeline events yet.';
export const ALERT_TIMELINE_UNAVAILABLE_STATE = 'No incident timeline available.';
export const ALERT_TIMELINE_FAILURE_STATE = 'Failed to load timeline.';
export const ALERT_TIMELINE_RETRY_LABEL = 'Retry';
export const ALERT_HISTORY_SEARCH_PLACEHOLDER = 'Search alerts...';
export const ALERT_HISTORY_EMPTY_STATE = 'No alerts found';
export const ALERT_HISTORY_EMPTY_DESCRIPTION = 'Try adjusting your filters or check back later';
export const ALERT_BUCKET_EMPTY_LABEL = 'No alerts';
export const ALERT_HISTORY_LOADING_STATE = 'Loading alert history...';
export const ALERTS_PAGE_DEFAULT_TITLE = 'Alerts';
export const ALERTS_PAGE_DEFAULT_DESCRIPTION = 'Manage alerting configuration.';
export const ALERTS_PAGE_OVERVIEW_TITLE = 'Alerts Overview';
export const ALERTS_PAGE_OVERVIEW_DESCRIPTION =
  'Monitor active alerts, acknowledgements, and recent status changes across platforms.';
export const ALERTS_PAGE_THRESHOLDS_TITLE = 'Alert Thresholds';
export const ALERTS_PAGE_THRESHOLDS_DESCRIPTION =
  'Tune resource thresholds and override rules for nodes, guests, and containers.';
export const ALERTS_PAGE_DESTINATIONS_TITLE = 'Notification Destinations';
export const ALERTS_PAGE_DESTINATIONS_DESCRIPTION =
  'Configure email, webhooks, and escalation paths for alert delivery.';
export const ALERTS_PAGE_SCHEDULE_TITLE = 'Maintenance Schedule';
export const ALERTS_PAGE_SCHEDULE_DESCRIPTION =
  'Set quiet hours and maintenance windows to suppress alerts when expected changes occur.';
export const ALERTS_PAGE_HISTORY_TITLE = 'Alert History';
export const ALERTS_PAGE_HISTORY_DESCRIPTION =
  'Review previously triggered alerts and their resolution timeline.';

export interface AlertOverviewCardPresentation {
  cardClassName: string;
  iconClassName: string;
  resourceClassName: string;
}

export function getAlertsPageHeaderMeta() {
  return {
    overview: {
      title: ALERTS_PAGE_OVERVIEW_TITLE,
      description: ALERTS_PAGE_OVERVIEW_DESCRIPTION,
    },
    thresholds: {
      title: ALERTS_PAGE_THRESHOLDS_TITLE,
      description: ALERTS_PAGE_THRESHOLDS_DESCRIPTION,
    },
    destinations: {
      title: ALERTS_PAGE_DESTINATIONS_TITLE,
      description: ALERTS_PAGE_DESTINATIONS_DESCRIPTION,
    },
    schedule: {
      title: ALERTS_PAGE_SCHEDULE_TITLE,
      description: ALERTS_PAGE_SCHEDULE_DESCRIPTION,
    },
    history: {
      title: ALERTS_PAGE_HISTORY_TITLE,
      description: ALERTS_PAGE_HISTORY_DESCRIPTION,
    },
    default: {
      title: ALERTS_PAGE_DEFAULT_TITLE,
      description: ALERTS_PAGE_DEFAULT_DESCRIPTION,
    },
  } as const;
}

export function getAlertListEmptyState(showAcknowledged: boolean): string {
  return showAcknowledged ? ALERTS_EMPTY_STATE : 'No unacknowledged alerts';
}

export function getAlertTimelineLoadingState() {
  return {
    text: ALERT_TIMELINE_LOADING_STATE,
  } as const;
}

export function getAlertTimelineFilterEmptyState() {
  return {
    text: ALERT_TIMELINE_FILTER_EMPTY_STATE,
  } as const;
}

export function getAlertTimelineEmptyState() {
  return {
    text: ALERT_TIMELINE_EMPTY_STATE,
  } as const;
}

export function getAlertTimelineUnavailableState() {
  return {
    text: ALERT_TIMELINE_UNAVAILABLE_STATE,
  } as const;
}

export function getAlertTimelineFailureState() {
  return {
    text: ALERT_TIMELINE_FAILURE_STATE,
    actionLabel: ALERT_TIMELINE_RETRY_LABEL,
  } as const;
}

export function getAlertHistorySearchPlaceholder() {
  return ALERT_HISTORY_SEARCH_PLACEHOLDER;
}

export function getAlertHistoryEmptyState() {
  return {
    title: ALERT_HISTORY_EMPTY_STATE,
    description: ALERT_HISTORY_EMPTY_DESCRIPTION,
  } as const;
}

export function getAlertHistoryLoadingState() {
  return {
    text: ALERT_HISTORY_LOADING_STATE,
  } as const;
}

export function getAlertBucketCountLabel(count: number) {
  return count === 0 ? ALERT_BUCKET_EMPTY_LABEL : `${count} alert${count === 1 ? '' : 's'}`;
}

export function getAlertOverviewCardPresentation(
  level: 'critical' | 'warning' | string,
  acknowledged: boolean,
  processing: boolean,
): AlertOverviewCardPresentation {
  const opacityClass = processing ? 'opacity-50' : acknowledged ? 'opacity-60' : '';
  const stateClass = acknowledged
    ? 'border-border bg-surface-alt'
    : level === 'critical'
      ? 'border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900'
      : 'border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900';
  const iconClassName = acknowledged
    ? 'mr-3 mt-0.5 transition-all text-green-600 dark:text-green-400'
    : level === 'critical'
      ? 'mr-3 mt-0.5 transition-all text-red-600 dark:text-red-400'
      : 'mr-3 mt-0.5 transition-all text-yellow-600 dark:text-yellow-400';
  const resourceClassName =
    level === 'critical'
      ? 'text-sm font-medium truncate text-red-700 dark:text-red-400'
      : 'text-sm font-medium truncate text-yellow-700 dark:text-yellow-400';

  return {
    cardClassName: ['border rounded-md p-3 sm:p-4 transition-all', opacityClass, stateClass]
      .filter(Boolean)
      .join(' '),
    iconClassName,
    resourceClassName,
  };
}

export function getAlertOverviewAcknowledgedBadgeClass(): string {
  return 'px-2 py-0.5 text-xs bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded';
}

export function getAlertOverviewStartedAtClass(): string {
  return 'mt-1 text-xs text-muted';
}

export function getAlertOverviewPrimaryActionClass(acknowledged: boolean): string {
  const stateClass = acknowledged
    ? 'text-base-content border-border hover:bg-surface-hover'
    : 'text-yellow-700 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700 hover:bg-yellow-50 dark:hover:bg-yellow-900';
  return `px-3 py-1.5 text-xs font-medium border rounded-md transition-all disabled:opacity-50 disabled:cursor-not-allowed ${stateClass}`;
}

export function getAlertOverviewSecondaryActionClass(): string {
  return 'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover';
}
