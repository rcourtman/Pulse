import { t } from '@/i18n';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';

export const ALERTS_EMPTY_STATE = 'No active alerts';
export const ALERTS_THRESHOLD_HINT = 'Alerts will appear here when thresholds are exceeded';
export const ALERT_TIMELINE_LOADING_STATE = 'Loading timeline...';
export const ALERT_TIMELINE_FILTER_EMPTY_STATE = 'No timeline events match the selected filters.';
export const ALERT_TIMELINE_EMPTY_STATE = 'No timeline events yet.';
export const ALERT_TIMELINE_UNAVAILABLE_STATE = 'No incident timeline available.';
export const ALERT_TIMELINE_FAILURE_STATE = 'Failed to load timeline.';
export const ALERT_TIMELINE_RETRY_LABEL = 'Retry';
export const ALERT_HISTORY_SEARCH_PLACEHOLDER = 'Search alerts...';
export const ALERT_HISTORY_EMPTY_STATE = 'No alerts found';
export const ALERT_HISTORY_EMPTY_DESCRIPTION = 'Try adjusting your filters or check back later';
export const ALERT_BUCKET_EMPTY_LABEL = 'No alerts';
export const ALERT_HISTORY_LOADING_STATE = 'Loading alert history...';
export const ALERT_HISTORY_ALL_TIME_FILTER_LABEL = getAllFilterOptionLabel('time');
export const ALERT_OVERVIEW_ACKNOWLEDGED_LABEL = 'Acknowledged';
export const ALERT_OVERVIEW_LAST_24_HOURS_LABEL = 'Triggered (24h)';
export const ALERT_OVERVIEW_WORKLOAD_OVERRIDES_LABEL = 'Workload Overrides';
export const ALERTS_PAGE_DEFAULT_TITLE = 'Alerts';
export const ALERTS_PAGE_OVERVIEW_TITLE = 'Alerts Overview';
export const ALERTS_PAGE_THRESHOLDS_TITLE = 'Alert Thresholds';
export const ALERTS_PAGE_DESTINATIONS_TITLE = 'Notifications';
export const ALERTS_PAGE_SCHEDULE_TITLE = 'Maintenance Schedule';
export const ALERTS_PAGE_HISTORY_TITLE = 'Alert History';
export const ALERTS_PAGE_DEFAULT_DESCRIPTION =
  'Review active incidents, inspect alert history, and manage thresholds, notifications, and schedules.';
export const ALERTS_PAGE_OVERVIEW_DESCRIPTION =
  'Review active incidents, confirm alert coverage, and control whether alerts are actively monitoring this install.';
export const ALERTS_PAGE_THRESHOLDS_DESCRIPTION =
  'Tune thresholds and scoped overrides for infrastructure, workloads, and integrations.';
export const ALERTS_PAGE_DESTINATIONS_DESCRIPTION =
  'Route alert notifications to email, Apprise, and webhook destinations.';
export const ALERTS_PAGE_SCHEDULE_DESCRIPTION =
  'Define quiet hours, grouping, cooldowns, recovery, and escalation behavior for alert delivery.';
export const ALERTS_PAGE_HISTORY_DESCRIPTION =
  'Search prior alerts, review incident timelines, and inspect alert frequency over time.';

export interface AlertOverviewCardPresentation {
  cardClassName: string;
  iconClassName: string;
  resourceClassName: string;
}

export function getAlertsPageHeaderMeta() {
  return {
    overview: {
      title: t('alerts.page.overview.title'),
      description: t('alerts.page.overview.description'),
    },
    thresholds: {
      title: t('alerts.page.thresholds.title'),
      description: t('alerts.page.thresholds.description'),
    },
    destinations: {
      title: t('alerts.page.destinations.title'),
      description: t('alerts.page.destinations.description'),
    },
    schedule: {
      title: t('alerts.page.schedule.title'),
      description: t('alerts.page.schedule.description'),
    },
    history: {
      title: t('alerts.page.history.title'),
      description: t('alerts.page.history.description'),
    },
    default: {
      title: t('alerts.page.default.title'),
      description: t('alerts.page.default.description'),
    },
  } as const;
}

export function getAlertListEmptyState(showAcknowledged: boolean): string {
  return showAcknowledged
    ? t('alerts.overview.filteredEmpty.all')
    : t('alerts.overview.filteredEmpty.unacknowledged');
}

export function getAlertOverviewEmptyState() {
  return {
    title: t('alerts.overview.empty.title'),
    description: t('alerts.overview.empty.description'),
  } as const;
}

export function getAlertOverviewPausedState() {
  return {
    title: t('alerts.overview.paused.title'),
    description: t('alerts.overview.paused.description'),
  } as const;
}

export function getAlertOverviewStatsLabels() {
  return {
    last24Hours: t('alerts.overview.stats.triggered24h'),
    acknowledged: t('alerts.overview.stats.acknowledged'),
    workloadOverrides: t('alerts.overview.stats.workloadOverrides'),
  } as const;
}

export function getAlertOverviewActiveSectionTitle(): string {
  return t('alerts.overview.section.activeAlerts');
}

export function getAlertOverviewAcknowledgedToggleLabel(showAcknowledged: boolean): string {
  return showAcknowledged
    ? t('alerts.overview.action.hideAcknowledged')
    : t('alerts.overview.action.showAcknowledged');
}

export function getAlertOverviewBulkAcknowledgeLabel(count: number, processing: boolean): string {
  return processing
    ? t('alerts.overview.action.acknowledging')
    : t('alerts.overview.action.acknowledgeAll', { count });
}

export function getAlertOverviewAcknowledgedBadgeLabel(): string {
  return t('alerts.overview.acknowledgedBadge');
}

export function getAlertOverviewNodeLabel(node: string): string {
  return t('alerts.overview.nodePrefix', { node });
}

export function getAlertOverviewStartedAtLabel(startedAt: string): string {
  return t('alerts.overview.startedAt', { startedAt });
}

export function getAlertOverviewPrimaryActionLabel({
  acknowledged,
  processing,
}: {
  acknowledged: boolean;
  processing: boolean;
}): string {
  if (processing) return t('alerts.overview.action.processing');
  return acknowledged
    ? t('alerts.overview.action.unacknowledge')
    : t('alerts.overview.action.acknowledge');
}

export function getAlertOverviewTimelineActionLabel(isExpanded: boolean): string {
  return isExpanded
    ? t('alerts.overview.action.hideTimeline')
    : t('alerts.overview.action.timeline');
}

export function getAlertOverviewRestoredNotification(): string {
  return t('alerts.overview.notification.restored');
}

export function getAlertOverviewAcknowledgedNotification(): string {
  return t('alerts.overview.notification.acknowledged');
}

export function getAlertOverviewAcknowledgementFailureNotification(
  wasAcknowledged: boolean,
): string {
  return wasAcknowledged
    ? t('alerts.overview.notification.restoreFailed')
    : t('alerts.overview.notification.acknowledgeFailed');
}

export function getAlertOverviewBulkAcknowledgedNotification(count: number): string {
  return count === 1
    ? t('alerts.overview.notification.bulkSuccess.singular', { count })
    : t('alerts.overview.notification.bulkSuccess.plural', { count });
}

export function getAlertOverviewBulkAcknowledgeFailureNotification(count: number): string {
  return count === 1
    ? t('alerts.overview.notification.bulkFailure.singular', { count })
    : t('alerts.overview.notification.bulkFailure.plural', { count });
}

export function getAlertOverviewBulkAcknowledgeGenericFailureNotification(): string {
  return t('alerts.overview.notification.bulkFailureGeneric');
}

export function getAlertTimelineLoadingState() {
  return {
    text: t('alerts.timeline.loading'),
  } as const;
}

export function getAlertTimelineFilterEmptyState() {
  return {
    text: t('alerts.timeline.filterEmpty'),
  } as const;
}

export function getAlertTimelineFilterLabel(variant: 'panel' | 'compact'): string {
  return variant === 'panel'
    ? t('alerts.timeline.filterLabel.panel')
    : t('alerts.timeline.filterLabel.compact');
}

export function getAlertTimelineQuickFilterLabel(action: 'all' | 'none'): string {
  return action === 'all'
    ? t('alerts.timeline.quickFilter.all')
    : t('alerts.timeline.quickFilter.none');
}

export function getAlertTimelineEventTypeLabel(type: string): string {
  switch (type) {
    case 'alert_fired':
      return t('alerts.timeline.event.alertFired');
    case 'alert_acknowledged':
      return t('alerts.timeline.event.alertAcknowledged');
    case 'alert_unacknowledged':
      return t('alerts.timeline.event.alertUnacknowledged');
    case 'alert_resolved':
      return t('alerts.timeline.event.alertResolved');
    case 'ai_analysis':
      return t('alerts.timeline.event.aiAnalysis');
    case 'command':
      return t('alerts.timeline.event.command');
    case 'runbook':
      return t('alerts.timeline.event.runbook');
    case 'note':
      return t('alerts.timeline.event.note');
    default:
      return type;
  }
}

export function getAlertTimelineEmptyState() {
  return {
    text: t('alerts.timeline.empty'),
  } as const;
}

export function getAlertTimelineUnavailableState() {
  return {
    text: t('alerts.timeline.unavailable'),
  } as const;
}

export function getAlertTimelineFailureState() {
  return {
    text: t('alerts.timeline.failure'),
    actionLabel: t('alerts.timeline.retry'),
  } as const;
}

export function getAlertTimelineHeading(): string {
  return t('alerts.timeline.heading');
}

export function getAlertTimelineAcknowledgedLabel(): string {
  return t('alerts.timeline.acknowledged');
}

export function getAlertTimelineOpenedAtLabel(openedAt: string): string {
  return t('alerts.timeline.openedAt', { openedAt });
}

export function getAlertTimelineClosedAtLabel(closedAt: string): string {
  return t('alerts.timeline.closedAt', { closedAt });
}

export function getAlertTimelineNoteLabel(): string {
  return t('alerts.timeline.noteLabel');
}

export function getAlertTimelineNotePlaceholder(): string {
  return t('alerts.timeline.notePlaceholder');
}

export function getAlertTimelineSaveNoteLabel(noteSaving: boolean): string {
  return noteSaving ? t('alerts.timeline.savingNote') : t('alerts.timeline.saveNote');
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

export function getAlertFilteredEmptyState(subject = 'alerts', filterLabel = 'active') {
  const normalizedSubject = subject.trim() || 'alerts';
  const normalizedFilterLabel = filterLabel.trim() || 'active';
  return {
    title: `No ${normalizedSubject} match current filters`,
    description: `Adjust the search or ${normalizedFilterLabel} filter to see more ${normalizedSubject}.`,
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
