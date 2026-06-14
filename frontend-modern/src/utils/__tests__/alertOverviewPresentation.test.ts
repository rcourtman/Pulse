import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import {
  ALERT_BUCKET_EMPTY_LABEL,
  ALERT_HISTORY_ALL_TIME_FILTER_LABEL,
  ALERT_HISTORY_EMPTY_DESCRIPTION,
  ALERT_HISTORY_EMPTY_STATE,
  ALERT_HISTORY_LOADING_STATE,
  ALERT_HISTORY_SEARCH_PLACEHOLDER,
  ALERT_OVERVIEW_ACKNOWLEDGED_LABEL,
  ALERT_OVERVIEW_LAST_24_HOURS_LABEL,
  ALERT_OVERVIEW_WORKLOAD_OVERRIDES_LABEL,
  ALERTS_EMPTY_STATE,
  ALERTS_PAGE_DEFAULT_TITLE,
  ALERTS_PAGE_DEFAULT_DESCRIPTION,
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
  getAlertOverviewAcknowledgedBadgeClass,
  getAlertOverviewAcknowledgedBadgeLabel,
  getAlertOverviewAcknowledgedNotification,
  getAlertOverviewAcknowledgedToggleLabel,
  getAlertOverviewActiveSectionTitle,
  getAlertOverviewBulkAcknowledgeFailureNotification,
  getAlertOverviewBulkAcknowledgeGenericFailureNotification,
  getAlertOverviewBulkAcknowledgeLabel,
  getAlertOverviewBulkAcknowledgedNotification,
  getAlertOverviewCardPresentation,
  getAlertOverviewEmptyState,
  getAlertOverviewNodeLabel,
  getAlertOverviewPausedState,
  getAlertOverviewPrimaryActionClass,
  getAlertOverviewPrimaryActionLabel,
  getAlertOverviewRestoredNotification,
  getAlertOverviewSecondaryActionClass,
  getAlertOverviewStartedAtLabel,
  getAlertOverviewStartedAtClass,
  getAlertOverviewStatsLabels,
  getAlertOverviewTimelineActionLabel,
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
  getAlertHistorySearchPlaceholder,
  getAlertBucketCountLabel,
  getAlertFilteredEmptyState,
  getAlertTimelineAcknowledgedLabel,
  getAlertTimelineClosedAtLabel,
  getAlertTimelineEmptyState,
  getAlertTimelineEventTypeLabel,
  getAlertTimelineFailureState,
  getAlertTimelineFilterEmptyState,
  getAlertTimelineFilterLabel,
  getAlertTimelineHeading,
  getAlertTimelineLoadingState,
  getAlertTimelineNoteLabel,
  getAlertTimelineNotePlaceholder,
  getAlertTimelineOpenedAtLabel,
  getAlertTimelineQuickFilterLabel,
  getAlertTimelineSaveNoteLabel,
  getAlertTimelineUnavailableState,
  getAlertListEmptyState,
  getAlertsPageHeaderMeta,
} from '@/utils/alertOverviewPresentation';

describe('alertOverviewPresentation', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('returns canonical alert overview empty-state copy', () => {
    expect(ALERTS_EMPTY_STATE).toBe('No active alerts');
    expect(ALERTS_THRESHOLD_HINT).toBe('Alerts will appear here when thresholds are exceeded');
    expect(getAlertListEmptyState(true)).toBe('No active alerts');
    expect(getAlertListEmptyState(false)).toBe('No unacknowledged alerts');
    expect(getAlertOverviewEmptyState()).toEqual({
      title: 'No active alerts',
      description: 'Alerts will appear here when thresholds are exceeded',
    });
    expect(getAlertOverviewPausedState()).toEqual({
      title: 'Alerting is paused',
      description: 'Toggle alerts on to resume monitoring and unlock configuration tabs',
    });
  });

  it('returns canonical alert overview stat labels', () => {
    expect(ALERT_OVERVIEW_ACKNOWLEDGED_LABEL).toBe('Acknowledged');
    expect(ALERT_OVERVIEW_LAST_24_HOURS_LABEL).toBe('Triggered (24h)');
    expect(ALERT_OVERVIEW_WORKLOAD_OVERRIDES_LABEL).toBe('Workload Overrides');
    expect(getAlertOverviewStatsLabels()).toEqual({
      last24Hours: 'Triggered (24h)',
      acknowledged: 'Acknowledged',
      workloadOverrides: 'Workload Overrides',
    });
  });

  it('returns canonical alert history search and empty-state copy', () => {
    expect(ALERT_HISTORY_SEARCH_PLACEHOLDER).toBe('Search alerts...');
    expect(getAlertHistorySearchPlaceholder()).toBe('Search alerts...');
    expect(ALERT_HISTORY_ALL_TIME_FILTER_LABEL).toBe('All time');
    expect(ALERT_HISTORY_EMPTY_STATE).toBe('No alerts found');
    expect(ALERT_HISTORY_EMPTY_DESCRIPTION).toBe('Try adjusting your filters or check back later');
    expect(getAlertHistoryEmptyState()).toEqual({
      title: 'No alerts found',
      description: 'Try adjusting your filters or check back later',
    });
    expect(getAlertFilteredEmptyState('TrueNAS alerts', 'severity')).toEqual({
      title: 'No TrueNAS alerts match current filters',
      description: 'Adjust the search or severity filter to see more TrueNAS alerts.',
    });
    expect(ALERT_HISTORY_LOADING_STATE).toBe('Loading alert history...');
    expect(getAlertHistoryLoadingState()).toEqual({
      text: 'Loading alert history...',
    });
  });

  it('returns canonical alerts page header metadata', () => {
    expect(ALERTS_PAGE_DEFAULT_TITLE).toBe('Alerts');
    expect(ALERTS_PAGE_OVERVIEW_TITLE).toBe('Alerts Overview');
    expect(ALERTS_PAGE_THRESHOLDS_TITLE).toBe('Alert Thresholds');
    expect(ALERTS_PAGE_DESTINATIONS_TITLE).toBe('Notifications');
    expect(ALERTS_PAGE_SCHEDULE_TITLE).toBe('Maintenance Schedule');
    expect(ALERTS_PAGE_HISTORY_TITLE).toBe('Alert History');
    expect(ALERTS_PAGE_DEFAULT_DESCRIPTION).toBe(
      'Review active incidents, inspect alert history, and manage thresholds, notifications, and schedules.',
    );
    expect(ALERTS_PAGE_OVERVIEW_DESCRIPTION).toBe(
      'Review active incidents, confirm alert coverage, and control whether alerts are actively monitoring this install.',
    );
    expect(ALERTS_PAGE_THRESHOLDS_DESCRIPTION).toBe(
      'Tune thresholds and scoped overrides for infrastructure, workloads, and integrations.',
    );
    expect(ALERTS_PAGE_DESTINATIONS_DESCRIPTION).toBe(
      'Route alert notifications to email, Apprise, and webhook destinations.',
    );
    expect(ALERTS_PAGE_SCHEDULE_DESCRIPTION).toBe(
      'Define quiet hours, grouping, cooldowns, recovery, and escalation behavior for alert delivery.',
    );
    expect(ALERTS_PAGE_HISTORY_DESCRIPTION).toBe(
      'Search prior alerts, review incident timelines, and inspect alert frequency over time.',
    );
    expect(getAlertsPageHeaderMeta()).toEqual({
      overview: {
        title: 'Alerts Overview',
        description:
          'Review active incidents, confirm alert coverage, and control whether alerts are actively monitoring this install.',
      },
      thresholds: {
        title: 'Alert Thresholds',
        description:
          'Tune thresholds and scoped overrides for infrastructure, workloads, and integrations.',
      },
      destinations: {
        title: 'Notifications',
        description: 'Route alert notifications to email, Apprise, and webhook destinations.',
      },
      schedule: {
        title: 'Maintenance Schedule',
        description:
          'Define quiet hours, grouping, cooldowns, recovery, and escalation behavior for alert delivery.',
      },
      history: {
        title: 'Alert History',
        description:
          'Search prior alerts, review incident timelines, and inspect alert frequency over time.',
      },
      default: {
        title: 'Alerts',
        description:
          'Review active incidents, inspect alert history, and manage thresholds, notifications, and schedules.',
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
    expect(getAlertTimelineHeading()).toBe('Incident');
    expect(getAlertTimelineAcknowledgedLabel()).toBe('acknowledged');
    expect(getAlertTimelineOpenedAtLabel('1/1/2026')).toBe('opened 1/1/2026');
    expect(getAlertTimelineClosedAtLabel('1/2/2026')).toBe('closed 1/2/2026');
    expect(getAlertTimelineNoteLabel()).toBe('Incident note');
    expect(getAlertTimelineNotePlaceholder()).toBe('Add a note for this incident...');
    expect(getAlertTimelineSaveNoteLabel(false)).toBe('Save Note');
    expect(getAlertTimelineSaveNoteLabel(true)).toBe('Saving...');
    expect(getAlertTimelineFilterLabel('panel')).toBe('Filter events:');
    expect(getAlertTimelineFilterLabel('compact')).toBe('Filters');
    expect(getAlertTimelineQuickFilterLabel('all')).toBe('All');
    expect(getAlertTimelineQuickFilterLabel('none')).toBe('None');
    expect(getAlertTimelineEventTypeLabel('command')).toBe('Cmd');
    expect(getAlertTimelineEventTypeLabel('unknown_event')).toBe('unknown_event');
  });

  it('returns canonical active alert card presentation', () => {
    expect(getAlertOverviewCardPresentation('critical', false, false)).toEqual({
      cardClassName:
        'border rounded-md p-3 sm:p-4 transition-all border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900',
      iconClassName: 'mr-3 mt-0.5 transition-all text-red-600 dark:text-red-400',
      resourceClassName: 'text-sm font-medium truncate text-red-700 dark:text-red-400',
    });
    expect(getAlertOverviewCardPresentation('warning', true, true)).toEqual({
      cardClassName:
        'border rounded-md p-3 sm:p-4 transition-all opacity-50 border-border bg-surface-alt',
      iconClassName: 'mr-3 mt-0.5 transition-all text-green-600 dark:text-green-400',
      resourceClassName: 'text-sm font-medium truncate text-yellow-700 dark:text-yellow-400',
    });
    expect(getAlertOverviewAcknowledgedBadgeClass()).toBe(
      'px-2 py-0.5 text-xs bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded',
    );
    expect(getAlertOverviewStartedAtClass()).toBe('mt-1 text-xs text-muted');
    expect(getAlertOverviewPrimaryActionClass(true)).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all disabled:opacity-50 disabled:cursor-not-allowed text-base-content border-border hover:bg-surface-hover',
    );
    expect(getAlertOverviewPrimaryActionClass(false)).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all disabled:opacity-50 disabled:cursor-not-allowed text-yellow-700 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700 hover:bg-yellow-50 dark:hover:bg-yellow-900',
    );
    expect(getAlertOverviewSecondaryActionClass()).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover',
    );
  });

  it('localizes alert overview helper copy through the active locale', () => {
    setActiveLocale('es');

    expect(getAlertsPageHeaderMeta().overview.title).toBe('Resumen de alertas');
    expect(getAlertOverviewActiveSectionTitle()).toBe('Alertas activas');
    expect(getAlertOverviewStatsLabels()).toEqual({
      last24Hours: 'Activadas (24h)',
      acknowledged: 'Reconocidas',
      workloadOverrides: 'Excepciones de cargas',
    });
    expect(getAlertOverviewEmptyState()).toEqual({
      title: 'No hay alertas activas',
      description: 'Las alertas apareceran aqui cuando se superen los umbrales',
    });
    expect(getAlertOverviewPausedState().title).toBe('Las alertas estan pausadas');
    expect(getAlertListEmptyState(false)).toBe('No hay alertas sin reconocer');
    expect(getAlertOverviewAcknowledgedToggleLabel(true)).toBe('Ocultar reconocidas');
    expect(getAlertOverviewBulkAcknowledgeLabel(2, false)).toBe('Reconocer todas (2)');
    expect(getAlertOverviewBulkAcknowledgeLabel(2, true)).toBe('Reconociendo...');
    expect(getAlertOverviewAcknowledgedBadgeLabel()).toBe('Reconocida');
    expect(getAlertOverviewNodeLabel('pve-01')).toBe('en pve-01');
    expect(getAlertOverviewStartedAtLabel('1/1/2026')).toBe('Inicio: 1/1/2026');
    expect(getAlertOverviewPrimaryActionLabel({ acknowledged: false, processing: false })).toBe(
      'Reconocer',
    );
    expect(getAlertOverviewPrimaryActionLabel({ acknowledged: true, processing: false })).toBe(
      'Quitar reconocimiento',
    );
    expect(getAlertOverviewPrimaryActionLabel({ acknowledged: false, processing: true })).toBe(
      'Procesando...',
    );
    expect(getAlertOverviewTimelineActionLabel(false)).toBe('Linea de tiempo');
    expect(getAlertOverviewAcknowledgedNotification()).toBe('Alerta reconocida');
    expect(getAlertOverviewRestoredNotification()).toBe('Alerta restaurada');
    expect(getAlertOverviewBulkAcknowledgedNotification(2)).toBe('2 alertas reconocidas.');
    expect(getAlertOverviewBulkAcknowledgeFailureNotification(1)).toBe(
      'No se pudo reconocer 1 alerta.',
    );
    expect(getAlertOverviewBulkAcknowledgeGenericFailureNotification()).toBe(
      'No se pudieron reconocer las alertas',
    );
    expect(getAlertTimelineFailureState()).toEqual({
      text: 'No se pudo cargar la linea de tiempo.',
      actionLabel: 'Reintentar',
    });
    expect(getAlertTimelineFilterLabel('panel')).toBe('Filtrar eventos:');
    expect(getAlertTimelineQuickFilterLabel('none')).toBe('Ninguna');
    expect(getAlertTimelineEventTypeLabel('command')).toBe('Comando');

    setActiveLocale('de');

    expect(getAlertOverviewActiveSectionTitle()).toBe('Aktive Warnmeldungen');
    expect(getAlertTimelineHeading()).toBe('Vorfall');
    expect(getAlertTimelineEventTypeLabel('alert_resolved')).toBe('Behoben');
  });
});
