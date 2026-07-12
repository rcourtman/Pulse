import { describe, expect, it } from 'vitest';
import {
  ALERT_RESOURCE_INCIDENT_LOAD_FAILURE,
  ALERT_RESOURCE_INCIDENT_NOTE_SAVE_FAILURE,
  ALERT_RESOURCE_INCIDENT_VIEW_TITLE,
  getAlertHistoryStatusPresentation,
  getAlertResourceIncidentLoadFailure,
  getAlertResourceIncidentNoteSaveFailure,
  getAlertResourceIncidentTruncatedEventsLabel,
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';

// Branch-coverage companion to alertIncidentPresentation.test.ts. The sibling
// suite already exercises the canonical 'active'/'acknowledged'/'resolved'
// strings and three shapes of getAlertResourceIncidentTruncatedEventsLabel;
// these tests drive the remaining branches: null/undefined/whitespace
// normalization, case-insensitive matching, the `||` fallback in the default
// arm, the singular `event` pluralization branch, strict-less/zero boundaries,
// a non-number totalCount (null), and the three previously-uncovered
// constant-returning accessors.

describe('getAlertHistoryStatusPresentation — branch coverage', () => {
  it('matches the "active" arm case-insensitively after trim/lowercase', () => {
    expect(getAlertHistoryStatusPresentation('  ACTIVE  ')).toStrictEqual({
      label: 'active',
      className:
        'text-xs px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 font-medium',
      rowClassName: 'bg-red-50 dark:bg-red-900',
    });
  });

  it('matches the "acknowledged" arm case-insensitively (capitalized input)', () => {
    expect(getAlertHistoryStatusPresentation('Acknowledged')).toStrictEqual({
      label: 'acknowledged',
      className:
        'text-xs px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
      rowClassName: '',
    });
  });

  it('hits the default arm with a truthy normalized value (|| left operand)', () => {
    // 'pending' is unrecognized but non-empty, so `normalized || 'resolved'`
    // keeps the left operand verbatim.
    expect(getAlertHistoryStatusPresentation('pending')).toStrictEqual({
      label: 'pending',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('lowercases an unrecognized status used as the label', () => {
    expect(getAlertHistoryStatusPresentation('PENDING')).toStrictEqual({
      label: 'pending',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('falls back to "resolved" when status is null (?? right operand)', () => {
    expect(getAlertHistoryStatusPresentation(null)).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('falls back to "resolved" when status is undefined (?? right operand)', () => {
    expect(getAlertHistoryStatusPresentation(undefined)).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('falls back to "resolved" for a whitespace-only status (|| right operand)', () => {
    expect(getAlertHistoryStatusPresentation('   ')).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('falls back to "resolved" for an empty-string status (?? passes "" through, || right operand)', () => {
    expect(getAlertHistoryStatusPresentation('')).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });
});

describe('getAlertResourceIncidentTruncatedEventsLabel — branch coverage', () => {
  it('uses the singular "event" form when totalCount === 1 (ternary "" arm)', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(1, 1)).toBe('Showing 1 event');
  });

  it('returns "Showing <totalCount> events" when totalCount is strictly less than count', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(10, 3)).toBe('Showing 3 events');
  });

  it('uses the plural "events" form when totalCount === 0 (ternary "s" arm)', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(10, 0)).toBe('Showing 0 events');
  });

  it('falls back to the count-only form when totalCount is null (typeof !== "number")', () => {
    expect(
      getAlertResourceIncidentTruncatedEventsLabel(6, null as unknown as number | undefined),
    ).toBe('Showing last 6 events');
  });

  it('falls back to the count-only form when totalCount is explicitly undefined', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(6, undefined)).toBe(
      'Showing last 6 events',
    );
  });

  it('renders the "last <count> of <total>" form when totalCount exceeds count', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(6, 9)).toBe('Showing last 6 of 9 events');
  });
});

describe('getAlertResourceIncidentLoadFailure — branch coverage', () => {
  it('returns the canonical load-failure string and matches the exported constant', () => {
    expect(getAlertResourceIncidentLoadFailure()).toBe('Failed to load resource incidents');
    expect(getAlertResourceIncidentLoadFailure()).toBe(ALERT_RESOURCE_INCIDENT_LOAD_FAILURE);
  });
});

describe('getAlertResourceIncidentNoteSaveFailure — branch coverage', () => {
  it('returns the canonical note-save-failure string and matches the exported constant', () => {
    expect(getAlertResourceIncidentNoteSaveFailure()).toBe('Failed to save incident note');
    expect(getAlertResourceIncidentNoteSaveFailure()).toBe(ALERT_RESOURCE_INCIDENT_NOTE_SAVE_FAILURE);
  });
});

describe('getAlertResourceIncidentViewTitle — branch coverage', () => {
  it('returns the canonical view-title string and matches the exported constant', () => {
    expect(getAlertResourceIncidentViewTitle()).toBe('View incidents for this resource');
    expect(getAlertResourceIncidentViewTitle()).toBe(ALERT_RESOURCE_INCIDENT_VIEW_TITLE);
  });
});
