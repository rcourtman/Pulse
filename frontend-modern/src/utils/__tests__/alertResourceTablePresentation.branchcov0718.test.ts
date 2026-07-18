import { describe, expect, it } from 'vitest';
import {
  // Metric-input title constants — the sibling test only asserts these via
  // the getAlertResourceTable*Title helpers, never pinning the constants
  // themselves. Each `export const` is its own coverage statement, so
  // importing them here guards against a silent rename.
  ALERT_RESOURCE_TABLE_DISABLE_METRIC_TITLE,
  ALERT_RESOURCE_TABLE_EDIT_METRIC_TITLE,
  ALERT_RESOURCE_TABLE_ENABLE_METRIC_TITLE,
  ALERT_RESOURCE_TABLE_EMPTY_STATE,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL,
  ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE,
  getAlertResourceTableEmptyState,
  getAlertResourceTableNoResultsState,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  type AlertResourceTableOfflineState,
} from '@/utils/alertResourceTablePresentation';

// Residual branch-coverage probes for alertResourceTablePresentation.
//
// The sibling test (alertResourceTablePresentation.test.ts) already exercises
// the canonical happy arm of every exported helper and drives the module to
// 100% v8 line/branch coverage. This file targets the *residual behavioral
// arms* the sibling never trips — the falsy-empty-string corner of the
// `emptyMessage || DEFAULT` short-circuit, the defensive `default` arm of
// the offline-state switch (reached only by a non-union sentinel cast
// through the param type), the toLowerCase boundary inputs for the
// no-results formatter, and the three metric-title constants the sibling
// never imports. None of these assertions duplicate the sibling.

describe('alertResourceTablePresentation.branchcov0718', () => {
  describe('getAlertResourceTableEmptyState — falsy empty-string corner', () => {
    // The sibling test covers the two obvious arms: undefined (-> default)
    // and a non-empty custom message (-> custom). The `||` short-circuit
    // also has a third corner: an explicitly empty string is falsy, so it
    // must fall back to the canonical default rather than render as blank.
    it('falls back to the canonical default when handed an empty string', () => {
      expect(getAlertResourceTableEmptyState('')).toBe(ALERT_RESOURCE_TABLE_EMPTY_STATE);
      expect(getAlertResourceTableEmptyState('')).toBe('No resources available.');
    });

    it('still prefers a non-empty whitespace-only message (truthy, not trimmed)', () => {
      // The guard is truthiness, not a trimmed check, so a string of only
      // spaces is truthy and is returned verbatim. Locking this prevents a
      // well-meaning refactor from silently introducing a `.trim()` that
      // would change observable behavior.
      expect(getAlertResourceTableEmptyState('   ')).toBe('   ');
    });
  });

  describe('getAlertResourceTableNoResultsState — toLowerCase boundary inputs', () => {
    // The sibling test passes only the title-cased single word 'Guests'.
    // Probe the lowercase-forcing interpolation across casing, word count,
    // and the empty-string boundary.
    it('lowercases an all-uppercase acronym title', () => {
      expect(getAlertResourceTableNoResultsState('VMS')).toBe('No vms found');
    });

    it('lowercases a multi-word title phrase', () => {
      expect(getAlertResourceTableNoResultsState('Alert Rules')).toBe(
        'No alert rules found',
      );
    });

    it('passes an already-lowercase title through unchanged', () => {
      expect(getAlertResourceTableNoResultsState('guests')).toBe('No guests found');
    });

    it('composes around an empty title without special-casing', () => {
      // No defensive guard on the title, so an empty string interpolates
      // verbatim into the template rather than rendering a fallback.
      expect(getAlertResourceTableNoResultsState('')).toBe('No  found');
    });
  });

  describe('getAlertResourceTableOfflineStatePresentation — defensive default arm', () => {
    // The switch's `case 'critical':` falls through to `default:`, so the
    // sibling's three named-state calls only ever exercise the three case
    // labels. The `default` arm itself is reached solely by a value the
    // union does not name; cast a sentinel string through the param type to
    // trip it and confirm the defensive fallback renders the critical tone.
    it('routes an unknown state through the default (critical-toned) arm', () => {
      const bogus = 'unknown' as unknown as AlertResourceTableOfflineState;
      expect(getAlertResourceTableOfflineStatePresentation(bogus)).toEqual({
        label: ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_LABEL,
        className:
          'bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-900 dark:text-red-200 dark:hover:bg-red-800',
        title: ALERT_RESOURCE_TABLE_OFFLINE_STATE_CRITICAL_TITLE,
      });
    });

    it('renders the critical label and title copy for the default arm', () => {
      const bogus = 'info' as unknown as AlertResourceTableOfflineState;
      const presentation = getAlertResourceTableOfflineStatePresentation(bogus);
      expect(presentation.label).toBe('Crit');
      expect(presentation.title).toBe(
        'Offline alerts will raise critical-level notifications.',
      );
    });
  });

  describe('getAlertResourceTableOfflineStateOrder — freshness guarantee', () => {
    // The sibling test asserts the literal contents once. The helper
    // returns a fresh array literal on every call, so callers can mutate
    // the result without side-effecting subsequent callers — pin that
    // independence so a future memoization does not silently break it.
    it('returns equal but independent array instances across calls', () => {
      const first = getAlertResourceTableOfflineStateOrder();
      const second = getAlertResourceTableOfflineStateOrder();
      expect(second).toEqual(['off', 'warning', 'critical']);
      expect(first).toEqual(second);
      expect(first).not.toBe(second);

      // Mutating the first call must not bleed into the second.
      first.push('critical');
      expect(second).toEqual(['off', 'warning', 'critical']);
    });
  });

  describe('metric-title constants — residual pins', () => {
    // The sibling test reads these values only indirectly through the
    // getAlertResourceTableMetricInputTitle / getAlertResourceTableEditMetricTitle
    // helpers. Pin the underlying constants directly so a rename is caught
    // even if the helper return values stay stable.
    it('exposes the enable/disable metric-input title constants', () => {
      expect(ALERT_RESOURCE_TABLE_ENABLE_METRIC_TITLE).toBe(
        'Click to enable this metric',
      );
      expect(ALERT_RESOURCE_TABLE_DISABLE_METRIC_TITLE).toBe(
        'Set to -1 to disable alerts for this metric',
      );
    });

    it('exposes the edit-metric title constant', () => {
      expect(ALERT_RESOURCE_TABLE_EDIT_METRIC_TITLE).toBe('Click to edit this metric');
    });
  });
});
