export type AlertIncidentLevel = 'warning' | 'critical' | string | null | undefined;
export type AlertIncidentStatus = 'open' | 'acknowledged' | 'resolved' | 'unknown';

export interface AlertIncidentStatusPresentation {
  label: AlertIncidentStatus | string;
  className: string;
}

export interface AlertHistoryStatusPresentation {
  label: string;
  className: string;
  rowClassName: string;
}

export const ALERT_RESOURCE_INCIDENT_PANEL_TITLE = 'Resource incidents';
export const ALERT_RESOURCE_INCIDENT_LOADING_STATE = 'Loading incidents...';
export const ALERT_RESOURCE_INCIDENT_EMPTY_STATE = 'No incidents recorded for this resource yet.';
export const ALERT_RESOURCE_INCIDENT_REFRESH_LABEL = 'Refresh';
export const ALERT_RESOURCE_INCIDENT_REFRESHING_LABEL = 'Refreshing...';
export const ALERT_RESOURCE_INCIDENT_NO_EVENTS_FILTERED_STATE =
  'No events match the selected filters.';
export const ALERT_RESOURCE_INCIDENT_NOTE_PLACEHOLDER = 'Add a note for this incident...';
export const ALERT_RESOURCE_INCIDENT_SAVE_NOTE_LABEL = 'Save Note';
export const ALERT_RESOURCE_INCIDENT_SAVING_NOTE_LABEL = 'Saving...';
export const ALERT_RESOURCE_INCIDENT_LOAD_FAILURE = 'Failed to load resource incidents';
export const ALERT_RESOURCE_INCIDENT_TIMELINE_FAILURE = 'Failed to load incident timeline';
export const ALERT_RESOURCE_INCIDENT_NOTE_SAVED = 'Incident note saved';
export const ALERT_RESOURCE_INCIDENT_NOTE_SAVE_FAILURE = 'Failed to save incident note';
export const ALERT_RESOURCE_INCIDENT_VIEW_TITLE = 'View incidents for this resource';

const ALERT_INCIDENT_STATUS_BASE = 'px-2 py-0.5 rounded';
const ALERT_INCIDENT_LEVEL_BASE = 'px-2 py-0.5 rounded';
const ALERT_INCIDENT_EVENT_FILTER_BUTTON_BASE =
  'px-2 py-0.5 rounded border text-[10px] transition-colors';

export type AlertIncidentEventFilterVariant = 'compact' | 'panel';

export function normalizeAlertIncidentStatus(
  status?: string | null,
  acknowledged?: boolean,
): AlertIncidentStatus | string {
  const normalized = (status ?? '').trim().toLowerCase();
  if (normalized === 'open' && acknowledged) return 'acknowledged';
  if (normalized === 'open') return 'open';
  if (normalized === 'resolved' || normalized === 'closed') return 'resolved';
  return normalized || 'unknown';
}

export function getAlertIncidentStatusPresentation(
  status?: string | null,
  acknowledged?: boolean,
): AlertIncidentStatusPresentation {
  const label = normalizeAlertIncidentStatus(status, acknowledged);
  switch (label) {
    case 'acknowledged':
      return {
        label,
        className: `${ALERT_INCIDENT_STATUS_BASE} bg-emerald-100 dark:bg-emerald-900 text-emerald-700 dark:text-emerald-300`,
      };
    case 'open':
      return {
        label,
        className: `${ALERT_INCIDENT_STATUS_BASE} bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300`,
      };
    default:
      return {
        label,
        className: `${ALERT_INCIDENT_STATUS_BASE} bg-surface-hover text-base-content`,
      };
  }
}

export function getAlertIncidentLevelBadgeClass(level: AlertIncidentLevel): string {
  if (level === 'critical') {
    return `${ALERT_INCIDENT_LEVEL_BASE} bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300`;
  }

  return `${ALERT_INCIDENT_LEVEL_BASE} bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300`;
}

export function getAlertHistoryStatusPresentation(status?: string | null): AlertHistoryStatusPresentation {
  const normalized = (status ?? '').trim().toLowerCase();

  if (normalized === 'active') {
    return {
      label: 'active',
      className:
        'text-xs px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 font-medium',
      rowClassName: 'bg-red-50 dark:bg-red-900',
    };
  }

  if (normalized === 'acknowledged') {
    return {
      label: 'acknowledged',
      className:
        'text-xs px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
      rowClassName: '',
    };
  }

  return {
    label: normalized || 'resolved',
    className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
    rowClassName: '',
  };
}

export function getAlertResourceIncidentLoadingState() {
  return {
    text: ALERT_RESOURCE_INCIDENT_LOADING_STATE,
  } as const;
}

export function getAlertResourceIncidentEmptyState() {
  return {
    text: ALERT_RESOURCE_INCIDENT_EMPTY_STATE,
  } as const;
}

export function getAlertResourceIncidentPanelTitle() {
  return ALERT_RESOURCE_INCIDENT_PANEL_TITLE;
}

export function getAlertResourceIncidentCountLabel(count: number) {
  return `${count} incident${count === 1 ? '' : 's'}`;
}

export function getAlertResourceIncidentRefreshLabel(isLoading: boolean) {
  return isLoading ? ALERT_RESOURCE_INCIDENT_REFRESHING_LABEL : ALERT_RESOURCE_INCIDENT_REFRESH_LABEL;
}

export function getAlertResourceIncidentAcknowledgedByLabel(user: string) {
  return `Acknowledged by ${user}`;
}

export function getAlertResourceIncidentToggleLabel(isExpanded: boolean, filteredLabel: string) {
  return isExpanded ? 'Hide events' : `Events (${filteredLabel})`;
}

export function getAlertResourceIncidentFilteredEventsEmptyState() {
  return {
    text: ALERT_RESOURCE_INCIDENT_NO_EVENTS_FILTERED_STATE,
  } as const;
}

export function getAlertResourceIncidentRecentEventsSummary(count: number) {
  return `Showing last ${count} events`;
}

export function getAlertResourceIncidentNotePlaceholder() {
  return ALERT_RESOURCE_INCIDENT_NOTE_PLACEHOLDER;
}

export function getAlertResourceIncidentSaveNoteLabel(isSaving: boolean) {
  return isSaving ? ALERT_RESOURCE_INCIDENT_SAVING_NOTE_LABEL : ALERT_RESOURCE_INCIDENT_SAVE_NOTE_LABEL;
}

export function getAlertResourceIncidentLoadFailure() {
  return ALERT_RESOURCE_INCIDENT_LOAD_FAILURE;
}

export function getAlertResourceIncidentTimelineFailure() {
  return ALERT_RESOURCE_INCIDENT_TIMELINE_FAILURE;
}

export function getAlertResourceIncidentNoteSavedLabel() {
  return ALERT_RESOURCE_INCIDENT_NOTE_SAVED;
}

export function getAlertResourceIncidentNoteSaveFailure() {
  return ALERT_RESOURCE_INCIDENT_NOTE_SAVE_FAILURE;
}

export function getAlertResourceIncidentViewTitle() {
  return ALERT_RESOURCE_INCIDENT_VIEW_TITLE;
}

export function getAlertIncidentEventFilterContainerClass(
  variant: AlertIncidentEventFilterVariant,
): string {
  if (variant === 'panel') {
    return 'flex flex-wrap items-center gap-1.5 rounded border border-border bg-surface-alt/50 p-2';
  }

  return 'flex flex-wrap items-center gap-2 text-[10px] text-muted';
}

export function getAlertIncidentEventFilterLabelClass(
  variant: AlertIncidentEventFilterVariant,
): string {
  if (variant === 'panel') {
    return 'mr-1 text-xs font-medium text-muted';
  }

  return 'uppercase tracking-wide text-[9px] text-muted';
}

export function getAlertIncidentEventFilterActionButtonClass(): string {
  return 'px-2 py-0.5 rounded border border-border text-muted hover:bg-surface-hover';
}

export function getAlertIncidentEventFilterChipClass(
  selected: boolean,
  variant: AlertIncidentEventFilterVariant,
): string {
  if (selected) {
    return `${ALERT_INCIDENT_EVENT_FILTER_BUTTON_BASE} border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300`;
  }

  if (variant === 'panel') {
    return `${ALERT_INCIDENT_EVENT_FILTER_BUTTON_BASE} font-medium border-border text-muted hover:bg-surface-alt`;
  }

  return `${ALERT_INCIDENT_EVENT_FILTER_BUTTON_BASE} border-border text-slate-500`;
}

export function getAlertIncidentAcknowledgedBadgeClass(): string {
  return 'px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300';
}

export function getAlertIncidentTimelineEventCardClass(variant: 'surface' | 'alt'): string {
  return `rounded border border-border ${variant === 'alt' ? 'bg-surface-alt' : 'bg-surface'} p-2`;
}

export function getAlertIncidentNoteTextareaClass(): string {
  return 'w-full rounded border border-border bg-surface p-2 text-xs text-base-content';
}

export function getAlertIncidentNoteSaveButtonClass(): string {
  return 'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed';
}

export function getAlertIncidentTimelineMetaRowClass(): string {
  return 'flex flex-wrap items-center gap-2 text-xs text-muted';
}

export function getAlertIncidentTimelineHeadingClass(): string {
  return 'font-medium text-base-content';
}

export function getAlertIncidentTimelineDetailClass(): string {
  return 'mt-1 text-xs text-base-content';
}

export function getAlertIncidentTimelineCommandClass(): string {
  return 'mt-1 font-mono text-xs text-base-content';
}

export function getAlertIncidentTimelineOutputClass(): string {
  return 'mt-1 text-xs text-muted';
}
