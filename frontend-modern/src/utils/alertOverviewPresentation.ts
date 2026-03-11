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
