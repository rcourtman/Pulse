import { describe, expect, it } from 'vitest';

import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentEventFilterActionButtonClass,
  getAlertIncidentEventFilterChipClass,
  getAlertIncidentEventFilterContainerClass,
  getAlertIncidentEventFilterLabelClass,
  getAlertIncidentLevelBadgeClass,
  getAlertIncidentNoteSaveButtonClass,
  getAlertIncidentNoteTextareaClass,
  getAlertIncidentStatusPresentation,
  getAlertIncidentTimelineCommandClass,
  getAlertIncidentTimelineDetailClass,
  getAlertIncidentTimelineEventCardClass,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
  getAlertResourceIncidentAcknowledgedByLabel,
  getAlertIncidentAcknowledgedBadgeClass,
  getAlertResourceIncidentActivityChipClass,
  getAlertResourceIncidentActivitySummaryClass,
  getAlertResourceIncidentCardClass,
  getAlertResourceIncidentCountLabel,
  getAlertResourceIncidentEmptyState,
  getAlertResourceIncidentFilteredEventsEmptyState,
  getAlertResourceIncidentLoadFailure,
  getAlertResourceIncidentLoadingState,
  getAlertResourceIncidentNotePlaceholder,
  getAlertResourceIncidentNoteSaveFailure,
  getAlertResourceIncidentNoteSavedLabel,
  getAlertResourceIncidentPanelTitle,
  getAlertResourceIncidentRecentEventsSummary,
  getAlertResourceIncidentRefreshLabel,
  getAlertResourceIncidentSaveNoteLabel,
  getAlertResourceIncidentSummaryRowClass,
  getAlertResourceIncidentTimelineFailure,
  getAlertResourceIncidentToggleButtonClass,
  getAlertResourceIncidentToggleLabel,
  getAlertResourceIncidentTruncatedEventsLabel,
  getAlertResourceIncidentViewTitle,
  normalizeAlertIncidentStatus,
} from '@/utils/alertIncidentPresentation';

// Supplemental branch-coverage suite. The pre-existing branchcov suite already
// exercises the happy paths; this file targets the nullish / empty / boundary /
// falsy-guard / switch-default arms that those tests leave open. Every assertion
// pins a concrete return value computed from the source so a regression flips it.

describe('normalizeAlertIncidentStatus — branch coverage', () => {
  it('coerces an undefined status through the ??-coalesce arm onto "unknown"', () => {
    // (undefined ?? '') -> '' -> trim/lower -> '' -> '' || 'unknown' -> 'unknown'.
    expect(normalizeAlertIncidentStatus(undefined)).toBe('unknown');
    // acknowledged flag is ignored once the normalized status is empty.
    expect(normalizeAlertIncidentStatus(undefined, true)).toBe('unknown');
  });

  it('coerces a null status through the ??-coalesce arm onto "unknown"', () => {
    expect(normalizeAlertIncidentStatus(null)).toBe('unknown');
  });

  it('treats a whitespace-only status as empty (trim -> "" -> `|| "unknown"`)  ', () => {
    expect(normalizeAlertIncidentStatus('   ')).toBe('unknown');
  });

  it('reaches "acknowledged" via `normalized === "open" && acknowledged` true arm', () => {
    expect(normalizeAlertIncidentStatus('open', true)).toBe('acknowledged');
  });

  it('honours the &&-short-circuit false arm: open + falsy acknowledged stays "open"', () => {
    // acknowledged === false -> `&&` yields false -> falls to the bare `=== 'open'` branch.
    expect(normalizeAlertIncidentStatus('open', false)).toBe('open');
    // acknowledged === undefined (omitted) is also falsy.
    expect(normalizeAlertIncidentStatus('open')).toBe('open');
    // A truthy-looking-but-falsy 0 also keeps the bare-open branch.
    expect(normalizeAlertIncidentStatus('open', 0 as unknown as boolean)).toBe('open');
  });

  it('maps the "closed" alias onto "resolved" via the `||` second operand', () => {
    expect(normalizeAlertIncidentStatus('closed')).toBe('resolved');
  });

  it('maps an exact "resolved" through the `||` first operand', () => {
    expect(normalizeAlertIncidentStatus('resolved')).toBe('resolved');
  });

  it('returns the trimmed/lowercased token verbatim for an unrecognised status (truthy `||` left)', () => {
    // toLowerCase() arm: 'INVESTIGATING' -> 'investigating', which is truthy.
    expect(normalizeAlertIncidentStatus('INVESTIGATING')).toBe('investigating');
  });

  it('normalises cased/whitespace-padded "open" before matching', () => {
    // Confirms trim().toLowerCase() runs before the `=== 'open'` comparison.
    expect(normalizeAlertIncidentStatus('  OPEN  ', true)).toBe('acknowledged');
    expect(normalizeAlertIncidentStatus('  OPEN  ', false)).toBe('open');
  });
});

describe('getAlertIncidentStatusPresentation — switch branch coverage', () => {
  it('hits the "acknowledged" case directly and via the open+acknowledged normalisation', () => {
    const expected = {
      label: 'acknowledged',
      className:
        'px-2 py-0.5 rounded bg-emerald-100 dark:bg-emerald-900 text-emerald-700 dark:text-emerald-300',
    };
    expect(getAlertIncidentStatusPresentation('acknowledged')).toStrictEqual(expected);
    // open + acknowledged flag normalises to 'acknowledged' label -> same case.
    expect(getAlertIncidentStatusPresentation('open', true)).toStrictEqual(expected);
  });

  it('hits the "open" case', () => {
    expect(getAlertIncidentStatusPresentation('open')).toStrictEqual({
      label: 'open',
      className:
        'px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
    });
  });

  it('routes "resolved" through the default case', () => {
    expect(getAlertIncidentStatusPresentation('resolved')).toStrictEqual({
      label: 'resolved',
      className: 'px-2 py-0.5 rounded bg-surface-hover text-base-content',
    });
  });

  it('routes a nullish status onto the default case with the "unknown" label', () => {
    expect(getAlertIncidentStatusPresentation(undefined)).toStrictEqual({
      label: 'unknown',
      className: 'px-2 py-0.5 rounded bg-surface-hover text-base-content',
    });
  });

  it('routes an unrecognised status onto the default case, echoing the normalised label', () => {
    expect(getAlertIncidentStatusPresentation('fizzing')).toStrictEqual({
      label: 'fizzing',
      className: 'px-2 py-0.5 rounded bg-surface-hover text-base-content',
    });
  });
});

describe('getAlertIncidentLevelBadgeClass — branch coverage', () => {
  it('returns the critical palette for level === "critical"', () => {
    expect(getAlertIncidentLevelBadgeClass('critical')).toBe(
      'px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
    );
  });

  it('falls back to the warning palette for the canonical "warning" level', () => {
    expect(getAlertIncidentLevelBadgeClass('warning')).toBe(
      'px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
    );
  });

  it('falls back to the warning palette for a null level', () => {
    expect(getAlertIncidentLevelBadgeClass(null)).toBe(
      'px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
    );
  });

  it('falls back to the warning palette for an undefined level', () => {
    expect(getAlertIncidentLevelBadgeClass(undefined)).toBe(
      'px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
    );
  });

  it('falls back to the warning palette for any non-"critical" string', () => {
    expect(getAlertIncidentLevelBadgeClass('info')).toBe(
      'px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
    );
  });
});

describe('getAlertHistoryStatusPresentation — branch coverage', () => {
  it('returns the active presentation for "active"', () => {
    expect(getAlertHistoryStatusPresentation('active')).toStrictEqual({
      label: 'active',
      className:
        'text-xs px-2 py-0.5 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 font-medium',
      rowClassName: 'bg-red-50 dark:bg-red-900',
    });
  });

  it('normalises a cased "ACTIVE" to the active branch (toLowerCase arm)', () => {
    expect(getAlertHistoryStatusPresentation('ACTIVE')).toStrictEqual(
      getAlertHistoryStatusPresentation('active'),
    );
  });

  it('returns the acknowledged presentation with an empty rowClassName', () => {
    expect(getAlertHistoryStatusPresentation('acknowledged')).toStrictEqual({
      label: 'acknowledged',
      className:
        'text-xs px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300',
      rowClassName: '',
    });
  });

  it('returns the resolved presentation for "resolved" (falls through both guards)', () => {
    expect(getAlertHistoryStatusPresentation('resolved')).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('uses the truthy `||` left operand for an unrecognised non-empty status', () => {
    expect(getAlertHistoryStatusPresentation('suppressed')).toStrictEqual({
      label: 'suppressed',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('substitutes "resolved" for an empty normalised status via the `|| "resolved"` fallback', () => {
    // undefined -> '' -> '' || 'resolved'.
    expect(getAlertHistoryStatusPresentation(undefined)).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('substitutes "resolved" for a null status via the ??-coalesce arm then || fallback', () => {
    expect(getAlertHistoryStatusPresentation(null)).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });

  it('substitutes "resolved" for a whitespace-only status (trim -> "" -> || fallback)', () => {
    expect(getAlertHistoryStatusPresentation('   ')).toStrictEqual({
      label: 'resolved',
      className: 'text-xs px-2 py-0.5 rounded bg-surface-hover text-base-content',
      rowClassName: '',
    });
  });
});

describe('ternary label helpers — both arms', () => {
  it('getAlertResourceIncidentCountLabel pluralises on count === 1 boundary', () => {
    expect(getAlertResourceIncidentCountLabel(0)).toBe('0 incidents');
    expect(getAlertResourceIncidentCountLabel(1)).toBe('1 incident');
    expect(getAlertResourceIncidentCountLabel(2)).toBe('2 incidents');
  });

  it('getAlertResourceIncidentRefreshLabel toggles on isLoading', () => {
    expect(getAlertResourceIncidentRefreshLabel(true)).toBe('Refreshing...');
    expect(getAlertResourceIncidentRefreshLabel(false)).toBe('Refresh');
  });

  it('getAlertResourceIncidentToggleLabel toggles on isExpanded and interpolates filteredLabel', () => {
    expect(getAlertResourceIncidentToggleLabel(true, '3 of 5')).toBe('Hide events');
    expect(getAlertResourceIncidentToggleLabel(false, '3 of 5')).toBe('Events (3 of 5)');
  });

  it('getAlertResourceIncidentSaveNoteLabel toggles on isSaving', () => {
    expect(getAlertResourceIncidentSaveNoteLabel(true)).toBe('Saving...');
    expect(getAlertResourceIncidentSaveNoteLabel(false)).toBe('Save Note');
  });
});

describe('non-branching label/state helpers — concrete value pins', () => {
  it('getAlertResourceIncidentAcknowledgedByLabel interpolates the user', () => {
    expect(getAlertResourceIncidentAcknowledgedByLabel('ada')).toBe('Acknowledged by ada');
  });

  it('getAlertResourceIncidentRecentEventsSummary interpolates the count', () => {
    expect(getAlertResourceIncidentRecentEventsSummary(7)).toBe('Showing last 7 events');
  });

  it('getAlertResourceIncidentLoadingState returns the loading const', () => {
    expect(getAlertResourceIncidentLoadingState()).toStrictEqual({
      text: 'Loading incidents...',
    });
  });

  it('getAlertResourceIncidentEmptyState returns the empty const', () => {
    expect(getAlertResourceIncidentEmptyState()).toStrictEqual({
      text: 'No incidents recorded for this resource yet.',
    });
  });

  it('getAlertResourceIncidentFilteredEventsEmptyState returns the filtered-empty const', () => {
    expect(getAlertResourceIncidentFilteredEventsEmptyState()).toStrictEqual({
      text: 'No events match the selected filters.',
    });
  });

  it('getAlertResourceIncidentPanelTitle returns the panel title const', () => {
    expect(getAlertResourceIncidentPanelTitle()).toBe('Resource incidents');
  });

  it('getAlertResourceIncidentNotePlaceholder returns the placeholder const', () => {
    expect(getAlertResourceIncidentNotePlaceholder()).toBe('Add a note for this incident...');
  });

  it('getAlertResourceIncidentLoadFailure returns the load-failure const', () => {
    expect(getAlertResourceIncidentLoadFailure()).toBe('Failed to load resource incidents');
  });

  it('getAlertResourceIncidentTimelineFailure returns the timeline-failure const', () => {
    expect(getAlertResourceIncidentTimelineFailure()).toBe('Failed to load incident timeline');
  });

  it('getAlertResourceIncidentNoteSavedLabel returns the note-saved const', () => {
    expect(getAlertResourceIncidentNoteSavedLabel()).toBe('Incident note saved');
  });

  it('getAlertResourceIncidentNoteSaveFailure returns the note-save-failure const', () => {
    expect(getAlertResourceIncidentNoteSaveFailure()).toBe('Failed to save incident note');
  });

  it('getAlertResourceIncidentViewTitle returns the view title const', () => {
    expect(getAlertResourceIncidentViewTitle()).toBe('View incidents for this resource');
  });
});

describe('event-filter class helpers — variant + selected branch coverage', () => {
  it('getAlertIncidentEventFilterContainerClass returns the panel layout for variant "panel"', () => {
    expect(getAlertIncidentEventFilterContainerClass('panel')).toBe(
      'flex flex-wrap items-center gap-1.5 rounded border border-border bg-surface-alt/50 p-2',
    );
  });

  it('getAlertIncidentEventFilterContainerClass returns the compact layout for variant "compact"', () => {
    expect(getAlertIncidentEventFilterContainerClass('compact')).toBe(
      'flex flex-wrap items-center gap-2 text-[10px] text-muted',
    );
  });

  it('getAlertIncidentEventFilterLabelClass returns the panel label class for variant "panel"', () => {
    expect(getAlertIncidentEventFilterLabelClass('panel')).toBe(
      'mr-1 text-xs font-medium text-muted',
    );
  });

  it('getAlertIncidentEventFilterLabelClass returns the compact label class for variant "compact"', () => {
    expect(getAlertIncidentEventFilterLabelClass('compact')).toBe(
      'uppercase tracking-wide text-[9px] text-muted',
    );
  });

  it('getAlertIncidentEventFilterActionButtonClass returns the constant action class', () => {
    expect(getAlertIncidentEventFilterActionButtonClass()).toBe(
      'px-2 py-0.5 rounded border border-border text-muted hover:bg-surface-hover',
    );
  });

  it('getAlertIncidentEventFilterChipClass returns the selected (blue) chip regardless of variant', () => {
    const selected = 'px-2 py-0.5 rounded border text-[10px] transition-colors border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300';
    // selected short-circuits before the variant check, so both variants are equal.
    expect(getAlertIncidentEventFilterChipClass(true, 'compact')).toBe(selected);
    expect(getAlertIncidentEventFilterChipClass(true, 'panel')).toBe(selected);
  });

  it('getAlertIncidentEventFilterChipClass returns the unselected panel chip (variant === "panel")', () => {
    expect(getAlertIncidentEventFilterChipClass(false, 'panel')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors font-medium border-border text-muted hover:bg-surface-alt',
    );
  });

  it('getAlertIncidentEventFilterChipClass returns the unselected compact chip (variant !== "panel")', () => {
    expect(getAlertIncidentEventFilterChipClass(false, 'compact')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors border-border text-slate-500',
    );
  });
});

describe('timeline + card class helpers — branch coverage', () => {
  it('getAlertIncidentTimelineEventCardClass uses bg-surface-alt for variant "alt"', () => {
    expect(getAlertIncidentTimelineEventCardClass('alt')).toBe(
      'rounded border border-border bg-surface-alt p-2',
    );
  });

  it('getAlertIncidentTimelineEventCardClass uses bg-surface for variant "surface"', () => {
    expect(getAlertIncidentTimelineEventCardClass('surface')).toBe(
      'rounded border border-border bg-surface p-2',
    );
  });

  it('pins the remaining constant class helpers (regression guards)', () => {
    expect(getAlertIncidentAcknowledgedBadgeClass()).toBe(
      'px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    );
    expect(getAlertIncidentNoteTextareaClass()).toBe(
      'w-full rounded border border-border bg-surface p-2 text-xs text-base-content',
    );
    expect(getAlertIncidentNoteSaveButtonClass()).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed',
    );
    expect(getAlertIncidentTimelineMetaRowClass()).toBe(
      'flex flex-wrap items-center gap-2 text-xs text-muted',
    );
    expect(getAlertIncidentTimelineHeadingClass()).toBe('font-medium text-base-content');
    expect(getAlertIncidentTimelineDetailClass()).toBe('mt-1 text-xs text-base-content');
    expect(getAlertIncidentTimelineCommandClass()).toBe('mt-1 font-mono text-xs text-base-content');
    expect(getAlertIncidentTimelineOutputClass()).toBe('mt-1 text-xs text-muted');
    expect(getAlertResourceIncidentCardClass()).toBe('rounded border border-border bg-surface p-3');
    expect(getAlertResourceIncidentSummaryRowClass()).toBe(
      'mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted',
    );
    expect(getAlertResourceIncidentActivitySummaryClass()).toBe(
      'flex flex-wrap items-center gap-1.5',
    );
    expect(getAlertResourceIncidentActivityChipClass()).toBe(
      'rounded bg-surface-alt px-2 py-0.5 text-[10px] font-medium text-base-content',
    );
    expect(getAlertResourceIncidentToggleButtonClass()).toBe(
      'px-2 py-1 text-[10px] border rounded-md border-border text-muted hover:bg-surface-hover',
    );
  });
});

describe('getAlertResourceIncidentTruncatedEventsLabel — branch coverage', () => {
  it('returns the bare "Showing last N events" when totalCount is undefined (typeof false arm)', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(5)).toBe('Showing last 5 events');
  });

  it('returns the bare form when totalCount is a non-number type (typeof false arm)', () => {
    expect(
      getAlertResourceIncidentTruncatedEventsLabel(5, 'many' as unknown as number),
    ).toBe('Showing last 5 events');
  });

  it('returns "Showing N events" when totalCount is a number <= count (singular at 1)', () => {
    // totalCount <= count true arm; totalCount === 1 true arm -> 'event'.
    expect(getAlertResourceIncidentTruncatedEventsLabel(5, 1)).toBe('Showing 1 event');
  });

  it('pluralises within the totalCount <= count branch when totalCount > 1', () => {
    // totalCount === 1 false arm -> 'events'.
    expect(getAlertResourceIncidentTruncatedEventsLabel(5, 3)).toBe('Showing 3 events');
  });

  it('honours the <= boundary at equality (count === totalCount)', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(5, 5)).toBe('Showing 5 events');
  });

  it('returns "Showing last N events" for totalCount 0 (0 <= count true arm, plural)', () => {
    expect(getAlertResourceIncidentTruncatedEventsLabel(3, 0)).toBe('Showing 0 events');
  });

  it('returns "Showing last count of totalCount events" when totalCount > count', () => {
    // totalCount <= count false arm.
    expect(getAlertResourceIncidentTruncatedEventsLabel(5, 12)).toBe(
      'Showing last 5 of 12 events',
    );
  });
});
