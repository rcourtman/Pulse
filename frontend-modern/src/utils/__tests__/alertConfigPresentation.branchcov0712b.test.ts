import { describe, expect, it } from 'vitest';
import {
  ALERT_CONFIG_SUMMARY_GROUPING_PREFIX,
  getAlertConfigSummaryEscalation,
  getAlertConfigSummaryGrouping,
} from '@/utils/alertConfigPresentation';

// Branch-coverage companion to alertConfigPresentation.test.ts.
// The sibling test only exercises getAlertConfigSummaryGrouping(10, true, false)
// (the "by node" arm) and getAlertConfigSummaryEscalation(2) (the plural arm).
// This file drives the remaining arms of every conditional in those two functions.
//
// Branches under test:
//  getAlertConfigSummaryGrouping:
//    - `byNode && 'node'` short-circuit (truthy -> 'node', falsy -> false)
//    - `byGuest && 'workload'` short-circuit (truthy -> 'workload', falsy -> false)
//    - `.filter(Boolean)` dropping the false entries
//    - `.join(' and ')` producing 0, 1, or 2 targets
//    - ternary `groupingTargets ? by-suffix : bare-window`
//  getAlertConfigSummaryEscalation:
//    - ternary `levelCount === 1 ? '' : 's'`

const GROUPING_PREFIX = ALERT_CONFIG_SUMMARY_GROUPING_PREFIX;

describe('alertConfigPresentation — branch coverage (branchcov2)', () => {
  describe('getAlertConfigSummaryGrouping', () => {
    it('emits the bare-window arm when neither flag is set (groupingTargets falsy)', () => {
      // Drives both `&&` short-circuits to their falsy arm, leaves
      // groupingTargets as '' after filter+join, and takes the ternary's
      // falsy branch — omitting the " by ..." suffix entirely.
      expect(getAlertConfigSummaryGrouping(0, false, false)).toBe(
        `${GROUPING_PREFIX} 0 minute windows`,
      );
      expect(getAlertConfigSummaryGrouping(15, false, false)).toBe(
        `${GROUPING_PREFIX} 15 minute windows`,
      );
    });

    it('uses only "workload" when byNode is false and byGuest is true', () => {
      // Exercises the byNode `&&` falsy arm and the byGuest `&&` truthy arm;
      // groupingTargets is truthy so the "by workload" suffix branch is taken.
      expect(getAlertConfigSummaryGrouping(5, false, true)).toBe(
        `${GROUPING_PREFIX} 5 minute windows by workload`,
      );
    });

    it('uses only "node" when byNode is true and byGuest is false', () => {
      // Mirrors the sibling test's arm but with a different window value to
      // confirm windowMinutes interpolation is independent of the flag logic.
      expect(getAlertConfigSummaryGrouping(30, true, false)).toBe(
        `${GROUPING_PREFIX} 30 minute windows by node`,
      );
    });

    it('joins both targets with " and " when both flags are true', () => {
      // The only path through which `.join(' and ')` produces a multi-target
      // string: both `&&` expressions resolve to their string operands.
      expect(getAlertConfigSummaryGrouping(10, true, true)).toBe(
        `${GROUPING_PREFIX} 10 minute windows by node and workload`,
      );
    });

    it('interpolates the exact numeric window value verbatim (no pluralization/coercion)', () => {
      // Pins the `${windowMinutes}` slot so future refactors can't quietly
      // round, clamp, or pluralize the window length.
      expect(getAlertConfigSummaryGrouping(1, true, true)).toContain(
        '1 minute windows by node and workload',
      );
      expect(getAlertConfigSummaryGrouping(120, false, false)).toContain(
        '120 minute windows',
      );
    });
  });

  describe('getAlertConfigSummaryEscalation', () => {
    it('uses the singular "level" form when exactly one level is configured', () => {
      // The only input that takes the `levelCount === 1 ? ''` truthy arm
      // (sibling test only covers the plural arm via levelCount=2).
      expect(getAlertConfigSummaryEscalation(1)).toBe(
        '• 1 escalation level configured',
      );
    });

    it('uses the plural "levels" form for counts other than 1', () => {
      // Drives the ternary's falsy arm across representative boundaries:
      // zero, two, and a larger count.
      expect(getAlertConfigSummaryEscalation(0)).toBe(
        '• 0 escalation levels configured',
      );
      expect(getAlertConfigSummaryEscalation(2)).toBe(
        '• 2 escalation levels configured',
      );
      expect(getAlertConfigSummaryEscalation(5)).toBe(
        '• 5 escalation levels configured',
      );
    });

    it('interpolates the count verbatim into the prefix bullet', () => {
      // Confirms the template literal wiring: bullet, count, then the fixed
      // trailing copy. Guards against accidental prefix changes.
      expect(getAlertConfigSummaryEscalation(3)).toBe(
        '• 3 escalation levels configured',
      );
      expect(getAlertConfigSummaryEscalation(1)).toMatch(
        /^• 1 escalation level configured$/,
      );
    });
  });
});
