import { describe, expect, it } from 'vitest';
import {
  ALERT_BUCKET_EMPTY_LABEL,
  ALERT_HISTORY_EMPTY_DESCRIPTION,
  ALERT_HISTORY_EMPTY_STATE,
  ALERT_HISTORY_LOADING_STATE,
  ALERT_HISTORY_SEARCH_PLACEHOLDER,
  ALERTS_EMPTY_STATE,
  ALERTS_PAGE_DEFAULT_DESCRIPTION,
  ALERTS_PAGE_DEFAULT_TITLE,
  ALERTS_PAGE_DESTINATIONS_DESCRIPTION,
  ALERTS_PAGE_DESTINATIONS_TITLE,
  ALERTS_PAGE_HISTORY_DESCRIPTION,
  ALERTS_PAGE_HISTORY_TITLE,
  ALERTS_PAGE_OVERVIEW_DESCRIPTION,
  ALERTS_PAGE_OVERVIEW_TITLE,
  ALERTS_PAGE_SCHEDULE_DESCRIPTION,
  ALERTS_PAGE_SCHEDULE_TITLE,
  ALERTS_PAGE_THRESHOLDS_DESCRIPTION,
  ALERTS_PAGE_THRESHOLDS_TITLE,
  ALERT_TIMELINE_EMPTY_STATE,
  ALERT_TIMELINE_FAILURE_STATE,
  ALERT_TIMELINE_FILTER_EMPTY_STATE,
  ALERT_TIMELINE_LOADING_STATE,
  ALERT_TIMELINE_RETRY_LABEL,
  ALERT_TIMELINE_UNAVAILABLE_STATE,
  ALERTS_THRESHOLD_HINT,
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
  getAlertHistorySearchPlaceholder,
  getAlertBucketCountLabel,
  getAlertTimelineEmptyState,
  getAlertTimelineFailureState,
  getAlertTimelineFilterEmptyState,
  getAlertTimelineLoadingState,
  getAlertTimelineUnavailableState,
  getAlertListEmptyState,
  getAlertsPageHeaderMeta,
} from '@/utils/alertOverviewPresentation';

describe('alertOverviewPresentation', () => {
  it('returns canonical alert overview empty-state copy', () => {
    expect(ALERTS_EMPTY_STATE).toBe('No active alerts');
    expect(ALERTS_THRESHOLD_HINT).toBe('Alerts will appear here when thresholds are exceeded');
    expect(getAlertListEmptyState(true)).toBe('No active alerts');
    expect(getAlertListEmptyState(false)).toBe('No unacknowledged alerts');
  });

  it('returns canonical alert history search and empty-state copy', () => {
    expect(ALERT_HISTORY_SEARCH_PLACEHOLDER).toBe('Search alerts...');
    expect(getAlertHistorySearchPlaceholder()).toBe('Search alerts...');
    expect(ALERT_HISTORY_EMPTY_STATE).toBe('No alerts found');
    expect(ALERT_HISTORY_EMPTY_DESCRIPTION).toBe(
      'Try adjusting your filters or check back later',
    );
    expect(getAlertHistoryEmptyState()).toEqual({
      title: 'No alerts found',
      description: 'Try adjusting your filters or check back later',
    });
    expect(ALERT_HISTORY_LOADING_STATE).toBe('Loading alert history...');
    expect(getAlertHistoryLoadingState()).toEqual({
      text: 'Loading alert history...',
    });
  });

  it('returns canonical alerts page header metadata', () => {
    expect(ALERTS_PAGE_DEFAULT_TITLE).toBe('Alerts');
    expect(ALERTS_PAGE_DEFAULT_DESCRIPTION).toBe('Manage alerting configuration.');
    expect(ALERTS_PAGE_OVERVIEW_TITLE).toBe('Alerts Overview');
    expect(ALERTS_PAGE_OVERVIEW_DESCRIPTION).toContain('recent status changes');
    expect(ALERTS_PAGE_THRESHOLDS_TITLE).toBe('Alert Thresholds');
    expect(ALERTS_PAGE_THRESHOLDS_DESCRIPTION).toContain('override rules');
    expect(ALERTS_PAGE_DESTINATIONS_TITLE).toBe('Notification Destinations');
    expect(ALERTS_PAGE_DESTINATIONS_DESCRIPTION).toContain('escalation paths');
    expect(ALERTS_PAGE_SCHEDULE_TITLE).toBe('Maintenance Schedule');
    expect(ALERTS_PAGE_SCHEDULE_DESCRIPTION).toContain('quiet hours');
    expect(ALERTS_PAGE_HISTORY_TITLE).toBe('Alert History');
    expect(ALERTS_PAGE_HISTORY_DESCRIPTION).toContain('resolution timeline');
    expect(getAlertsPageHeaderMeta()).toEqual({
      overview: {
        title: 'Alerts Overview',
        description:
          'Monitor active alerts, acknowledgements, and recent status changes across platforms.',
      },
      thresholds: {
        title: 'Alert Thresholds',
        description:
          'Tune resource thresholds and override rules for nodes, guests, and containers.',
      },
      destinations: {
        title: 'Notification Destinations',
        description: 'Configure email, webhooks, and escalation paths for alert delivery.',
      },
      schedule: {
        title: 'Maintenance Schedule',
        description:
          'Set quiet hours and maintenance windows to suppress alerts when expected changes occur.',
      },
      history: {
        title: 'Alert History',
        description: 'Review previously triggered alerts and their resolution timeline.',
      },
      default: {
        title: 'Alerts',
        description: 'Manage alerting configuration.',
      },
    });
  });

  it('formats canonical alert bucket count labels', () => {
    expect(ALERT_BUCKET_EMPTY_LABEL).toBe('No alerts');
    expect(getAlertBucketCountLabel(0)).toBe('No alerts');
    expect(getAlertBucketCountLabel(1)).toBe('1 alert');
    expect(getAlertBucketCountLabel(3)).toBe('3 alerts');
  });

  it('returns canonical incident timeline state copy', () => {
    expect(ALERT_TIMELINE_LOADING_STATE).toBe('Loading timeline...');
    expect(ALERT_TIMELINE_FILTER_EMPTY_STATE).toBe(
      'No timeline events match the selected filters.',
    );
    expect(ALERT_TIMELINE_EMPTY_STATE).toBe('No timeline events yet.');
    expect(ALERT_TIMELINE_UNAVAILABLE_STATE).toBe('No incident timeline available.');
    expect(ALERT_TIMELINE_FAILURE_STATE).toBe('Failed to load timeline.');
    expect(ALERT_TIMELINE_RETRY_LABEL).toBe('Retry');
    expect(getAlertTimelineLoadingState()).toEqual({ text: ALERT_TIMELINE_LOADING_STATE });
    expect(getAlertTimelineFilterEmptyState()).toEqual({
      text: ALERT_TIMELINE_FILTER_EMPTY_STATE,
    });
    expect(getAlertTimelineEmptyState()).toEqual({ text: ALERT_TIMELINE_EMPTY_STATE });
    expect(getAlertTimelineUnavailableState()).toEqual({
      text: ALERT_TIMELINE_UNAVAILABLE_STATE,
    });
    expect(getAlertTimelineFailureState()).toEqual({
      text: ALERT_TIMELINE_FAILURE_STATE,
      actionLabel: ALERT_TIMELINE_RETRY_LABEL,
    });
  });
});
