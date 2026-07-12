import { describe, expect, it } from 'vitest';
import type { RecoveryOutcome } from '@/types/recovery';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeBarClass,
  getRecoveryOutcomeLabel,
  getRecoveryOutcomeTextClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';

// A value that is not part of the canonical outcome set; used to drive the
// `default` arms of the switch statements and the fallthrough branch of
// normalizeRecoveryOutcome. Cast through `unknown` to satisfy strict typing.
const NON_CANONICAL = 'partial' as unknown as RecoveryOutcome;

const BADGE_BASE = 'inline-flex items-center rounded-full px-2 py-1 text-xs font-medium';

describe('recoveryOutcomePresentation — branch coverage (branchcov2)', () => {
  describe('normalizeRecoveryOutcome', () => {
    it('treats null/undefined/empty as falsy and falls back to unknown', () => {
      // Exercises the `(value || '')` defensive branch for every falsy input.
      expect(normalizeRecoveryOutcome(null)).toBe('unknown');
      expect(normalizeRecoveryOutcome(undefined)).toBe('unknown');
      expect(normalizeRecoveryOutcome('')).toBe('unknown');
    });

    it('treats whitespace-only input as unknown after trim', () => {
      expect(normalizeRecoveryOutcome('   ')).toBe('unknown');
    });

    it('matches the exact canonical tokens (case-insensitive)', () => {
      // Each of these hits a distinct early-return arm.
      expect(normalizeRecoveryOutcome('SUCCESS')).toBe('success');
      expect(normalizeRecoveryOutcome('Warning')).toBe('warning');
      expect(normalizeRecoveryOutcome('RUNNING')).toBe('running');
      expect(normalizeRecoveryOutcome('Unknown')).toBe('unknown');
    });

    it('maps the "error" alias onto failed', () => {
      // "error" is the only failed-alias not exercised by the sibling test.
      expect(normalizeRecoveryOutcome('error')).toBe('failed');
      expect(normalizeRecoveryOutcome(' ERROR ')).toBe('failed');
    });

    it('hits the explicit unknown arm distinctly from the default fallthrough', () => {
      // 'unknown' takes the explicit `if (normalized === 'unknown')` arm while
      // a non-canonical token reaches the final default return.
      expect(normalizeRecoveryOutcome('unknown')).toBe('unknown');
      expect(normalizeRecoveryOutcome('nope')).toBe('unknown');
    });
  });

  describe('getRecoveryOutcomeLabel', () => {
    it('returns a human label for every canonical outcome', () => {
      expect(getRecoveryOutcomeLabel('success')).toBe('Healthy');
      expect(getRecoveryOutcomeLabel('warning')).toBe('Warning');
      expect(getRecoveryOutcomeLabel('failed')).toBe('Failed');
      expect(getRecoveryOutcomeLabel('running')).toBe('Running');
      expect(getRecoveryOutcomeLabel('unknown')).toBe('Unknown');
    });

    it('routes non-canonical outcomes through the default arm', () => {
      expect(getRecoveryOutcomeLabel(NON_CANONICAL)).toBe('Unknown');
    });
  });

  describe('getRecoveryOutcomeBadgeClass', () => {
    it('renders the full badge class for success and warning', () => {
      expect(getRecoveryOutcomeBadgeClass('success')).toBe(
        `${BADGE_BASE} bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300`,
      );
      expect(getRecoveryOutcomeBadgeClass('warning')).toBe(
        `${BADGE_BASE} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`,
      );
    });

    it('renders the full badge class for failed and running (strict equality)', () => {
      // Sibling test only asserted substring membership; pin the whole string.
      expect(getRecoveryOutcomeBadgeClass('failed')).toBe(
        `${BADGE_BASE} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300`,
      );
      expect(getRecoveryOutcomeBadgeClass('running')).toBe(
        `${BADGE_BASE} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300`,
      );
    });

    it('falls back to the neutral badge for the default arm', () => {
      expect(getRecoveryOutcomeBadgeClass('unknown')).toBe(
        `${BADGE_BASE} bg-surface-alt text-muted`,
      );
      expect(getRecoveryOutcomeBadgeClass(NON_CANONICAL)).toBe(
        `${BADGE_BASE} bg-surface-alt text-muted`,
      );
    });
  });

  describe('getRecoveryOutcomeBarClass', () => {
    it('returns a solid bar color for each canonical outcome', () => {
      expect(getRecoveryOutcomeBarClass('success')).toBe('bg-emerald-500');
      expect(getRecoveryOutcomeBarClass('warning')).toBe('bg-amber-400');
      expect(getRecoveryOutcomeBarClass('failed')).toBe('bg-red-500');
      expect(getRecoveryOutcomeBarClass('running')).toBe('bg-blue-500');
    });

    it('falls back to gray for the default arm', () => {
      expect(getRecoveryOutcomeBarClass('unknown')).toBe('bg-gray-400');
      expect(getRecoveryOutcomeBarClass(NON_CANONICAL)).toBe('bg-gray-400');
    });
  });

  describe('getRecoveryOutcomeTextClass', () => {
    it('returns a dark-mode-aware text tone for each canonical outcome', () => {
      expect(getRecoveryOutcomeTextClass('success')).toBe('text-emerald-600 dark:text-emerald-400');
      expect(getRecoveryOutcomeTextClass('warning')).toBe('text-amber-600 dark:text-amber-400');
      expect(getRecoveryOutcomeTextClass('failed')).toBe('text-red-600 dark:text-red-400');
      expect(getRecoveryOutcomeTextClass('running')).toBe('text-blue-600 dark:text-blue-400');
    });

    it('falls back to text-muted for the default arm', () => {
      expect(getRecoveryOutcomeTextClass('unknown')).toBe('text-muted');
      expect(getRecoveryOutcomeTextClass(NON_CANONICAL)).toBe('text-muted');
    });
  });
});
