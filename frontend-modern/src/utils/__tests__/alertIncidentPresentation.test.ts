import { describe, expect, it } from 'vitest';
import {
  ALERT_RESOURCE_INCIDENT_EMPTY_STATE,
  ALERT_RESOURCE_INCIDENT_LOADING_STATE,
  ALERT_RESOURCE_INCIDENT_NOTE_PLACEHOLDER,
  ALERT_RESOURCE_INCIDENT_NO_EVENTS_FILTERED_STATE,
  ALERT_RESOURCE_INCIDENT_PANEL_TITLE,
  ALERT_RESOURCE_INCIDENT_REFRESHING_LABEL,
  ALERT_RESOURCE_INCIDENT_REFRESH_LABEL,
  ALERT_RESOURCE_INCIDENT_SAVE_NOTE_LABEL,
  ALERT_RESOURCE_INCIDENT_SAVING_NOTE_LABEL,
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
  getAlertResourceIncidentAcknowledgedByLabel,
  getAlertResourceIncidentCountLabel,
  getAlertResourceIncidentEmptyState,
  getAlertResourceIncidentFilteredEventsEmptyState,
  getAlertResourceIncidentLoadingState,
  getAlertResourceIncidentNotePlaceholder,
  getAlertResourceIncidentPanelTitle,
  getAlertResourceIncidentRecentEventsSummary,
  getAlertResourceIncidentRefreshLabel,
  getAlertResourceIncidentSaveNoteLabel,
  getAlertResourceIncidentToggleLabel,
  getAlertIncidentStatusPresentation,
  getAlertIncidentEventFilterActionButtonClass,
  getAlertIncidentEventFilterChipClass,
  getAlertIncidentEventFilterContainerClass,
  getAlertIncidentEventFilterLabelClass,
  normalizeAlertIncidentStatus,
} from '@/utils/alertIncidentPresentation';

describe('alertIncidentPresentation', () => {
  it('normalizes alert incident status canonically', () => {
    expect(normalizeAlertIncidentStatus('open', true)).toBe('acknowledged');
    expect(normalizeAlertIncidentStatus('open', false)).toBe('open');
    expect(normalizeAlertIncidentStatus('resolved', false)).toBe('resolved');
    expect(normalizeAlertIncidentStatus('closed', false)).toBe('resolved');
    expect(normalizeAlertIncidentStatus(undefined, false)).toBe('unknown');
  });

  it('returns canonical incident status presentation classes', () => {
    expect(getAlertIncidentStatusPresentation('open', true)).toEqual({
      label: 'acknowledged',
      className:
        'px-2 py-0.5 rounded bg-emerald-100 dark:bg-emerald-900 text-emerald-700 dark:text-emerald-300',
    });

    expect(getAlertIncidentStatusPresentation('open', false)).toEqual({
      label: 'open',
      className:
        'px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
    });

    expect(getAlertIncidentStatusPresentation('resolved', false)).toEqual({
      label: 'resolved',
      className: 'px-2 py-0.5 rounded bg-surface-hover text-base-content',
    });
  });

  it('returns canonical incident level badge classes', () => {
    expect(getAlertIncidentLevelBadgeClass('critical')).toContain('bg-red-100');
    expect(getAlertIncidentLevelBadgeClass('warning')).toContain('bg-yellow-100');
    expect(getAlertIncidentLevelBadgeClass(undefined)).toContain('bg-yellow-100');
  });

  it('returns canonical alert history status presentation', () => {
    expect(getAlertHistoryStatusPresentation('active')).toEqual({
      label: 'active',
      className:
        'text-xs px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 font-medium',
      rowClassName: 'bg-red-50 dark:bg-red-900',
    });

    expect(getAlertHistoryStatusPresentation('acknowledged')).toEqual({
      label: 'acknowledged',
      className:
        'text-xs px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
      rowClassName: '',
    });

    expect(getAlertHistoryStatusPresentation('resolved')).toEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('returns canonical resource incident loading and empty-state copy', () => {
    expect(ALERT_RESOURCE_INCIDENT_PANEL_TITLE).toBe('Resource incidents');
    expect(ALERT_RESOURCE_INCIDENT_LOADING_STATE).toBe('Loading incidents...');
    expect(ALERT_RESOURCE_INCIDENT_EMPTY_STATE).toBe(
      'No incidents recorded for this resource yet.',
    );
    expect(ALERT_RESOURCE_INCIDENT_REFRESH_LABEL).toBe('Refresh');
    expect(ALERT_RESOURCE_INCIDENT_REFRESHING_LABEL).toBe('Refreshing...');
    expect(ALERT_RESOURCE_INCIDENT_NO_EVENTS_FILTERED_STATE).toBe(
      'No events match the selected filters.',
    );
    expect(ALERT_RESOURCE_INCIDENT_NOTE_PLACEHOLDER).toBe('Add a note for this incident...');
    expect(ALERT_RESOURCE_INCIDENT_SAVE_NOTE_LABEL).toBe('Save Note');
    expect(ALERT_RESOURCE_INCIDENT_SAVING_NOTE_LABEL).toBe('Saving...');
    expect(getAlertResourceIncidentPanelTitle()).toBe('Resource incidents');
    expect(getAlertResourceIncidentLoadingState()).toEqual({
      text: 'Loading incidents...',
    });
    expect(getAlertResourceIncidentEmptyState()).toEqual({
      text: 'No incidents recorded for this resource yet.',
    });
    expect(getAlertResourceIncidentCountLabel(1)).toBe('1 incident');
    expect(getAlertResourceIncidentCountLabel(3)).toBe('3 incidents');
    expect(getAlertResourceIncidentRefreshLabel(false)).toBe('Refresh');
    expect(getAlertResourceIncidentRefreshLabel(true)).toBe('Refreshing...');
    expect(getAlertResourceIncidentNotePlaceholder()).toBe('Add a note for this incident...');
    expect(getAlertResourceIncidentSaveNoteLabel(false)).toBe('Save Note');
    expect(getAlertResourceIncidentSaveNoteLabel(true)).toBe('Saving...');
    expect(getAlertResourceIncidentAcknowledgedByLabel('alice')).toBe('Acknowledged by alice');
    expect(getAlertResourceIncidentToggleLabel(false, '2/8')).toBe('Events (2/8)');
    expect(getAlertResourceIncidentToggleLabel(true, '2/8')).toBe('Hide events');
    expect(getAlertResourceIncidentFilteredEventsEmptyState()).toEqual({
      text: 'No events match the selected filters.',
    });
    expect(getAlertResourceIncidentRecentEventsSummary(6)).toBe('Showing last 6 events');
  });

  it('returns canonical incident event filter presentation classes', () => {
    expect(getAlertIncidentEventFilterContainerClass('compact')).toBe(
      'flex flex-wrap items-center gap-2 text-[10px] text-muted',
    );
    expect(getAlertIncidentEventFilterContainerClass('panel')).toBe(
      'flex flex-wrap items-center gap-1.5 rounded border border-border bg-surface-alt/50 p-2',
    );
    expect(getAlertIncidentEventFilterLabelClass('compact')).toBe(
      'uppercase tracking-wide text-[9px] text-muted',
    );
    expect(getAlertIncidentEventFilterLabelClass('panel')).toBe(
      'mr-1 text-xs font-medium text-muted',
    );
    expect(getAlertIncidentEventFilterActionButtonClass()).toBe(
      'px-2 py-0.5 rounded border border-border text-muted hover:bg-surface-hover',
    );
    expect(getAlertIncidentEventFilterChipClass(true, 'compact')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
    );
    expect(getAlertIncidentEventFilterChipClass(false, 'compact')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors border-border text-slate-500',
    );
    expect(getAlertIncidentEventFilterChipClass(false, 'panel')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors font-medium border-border text-muted hover:bg-surface-alt',
    );
  });
});
