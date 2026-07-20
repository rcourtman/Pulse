import { afterEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import {
  formatPvePollingDuration,
  getBackupIntervalSummary,
} from '@/utils/systemSettingsPresentation';

// Focused branch-coverage companion to systemSettingsPresentation.test.ts.
// The sibling test only exercised formatPvePollingDuration(90, 'de') and a
// handful of getBackupIntervalSummary rows; these cases drive the remaining
// arms of every conditional/ternary in the two target functions.

describe('systemSettingsPresentation — branch coverage (branchcov2)', () => {
  afterEach(() => {
    // Keep locale mutations local to this file so they cannot leak into peers.
    setActiveLocale(DEFAULT_LOCALE);
  });

  describe('formatPvePollingDuration', () => {
    it('returns the under-minute copy when seconds is below sixty (true arm of `seconds < 60`)', () => {
      expect(formatPvePollingDuration(59, 'en')).toBe('under a minute');
      expect(formatPvePollingDuration(30, 'es')).toBe('menos de un minuto');
      expect(formatPvePollingDuration(0, 'en')).toBe('under a minute');
      // Defensive negative still takes the early-return arm.
      expect(formatPvePollingDuration(-1, 'en')).toBe('under a minute');
    });

    it('treats exactly sixty seconds as one whole minute (boundary false arm + singular `minute` key)', () => {
      // seconds === 60 is the first value where `seconds < 60` is false; it also
      // forces minutes === 1 (singular) and minutes % 1 === 0 (maxFrac 0).
      expect(formatPvePollingDuration(60, 'en')).toBe('1 minute');
      expect(formatPvePollingDuration(60, 'de')).toBe('1 Minute');
    });

    it('formats whole-number minutes with no fraction (true arm of `minutes % 1 === 0`, plural)', () => {
      // 120s -> 2 whole minutes -> maximumFractionDigits 0 -> plural key.
      expect(formatPvePollingDuration(120, 'en')).toBe('2 minutes');
      expect(formatPvePollingDuration(1800, 'en')).toBe('30 minutes');
    });

    it('formats fractional minutes with one decimal (false arm of `minutes % 1 === 0`, plural)', () => {
      // 90s -> 1.5 -> maximumFractionDigits 1; 1.5 !== 1 so the plural key wins.
      expect(formatPvePollingDuration(90, 'en')).toBe('1.5 minutes');
      // 150s -> 2.5 -> exercises the fractional arm again with a different count.
      expect(formatPvePollingDuration(150, 'en')).toBe('2.5 minutes');
    });

    it('falls back to getActiveLocale() when locale is omitted (false arm of `locale ?? getActiveLocale()`)', () => {
      // No locale arg -> resolvePresentationLocale must consult getActiveLocale().
      setActiveLocale('es');
      expect(formatPvePollingDuration(30)).toBe('menos de un minuto');
      setActiveLocale('de');
      expect(formatPvePollingDuration(60)).toBe('1 Minute');
    });

    it('propagates NaN seconds through the >= 60 path without throwing (defensive)', () => {
      // NaN < 60 is false, so it falls through to the minutes formatting where
      // both ternaries evaluate their falsy arms (NaN === 0 is false, NaN === 1
      // is false), yielding the literal "NaN" token in the plural copy.
      expect(formatPvePollingDuration(Number.NaN, 'en')).toBe('NaN minutes');
    });
  });

  describe('getBackupIntervalSummary', () => {
    it('disables polling regardless of interval when the flag is falsy (true arm of `!backupPollingEnabled`)', () => {
      // A valid hourly interval is ignored once the disabled flag is set, and
      // the disabled branch wins over a non-boolean falsy value (JS coercion).
      expect(getBackupIntervalSummary(false, 3600)).toBe('Backup polling is disabled.');
      expect(getBackupIntervalSummary(0 as unknown as boolean, 86400)).toBe(
        'Backup polling is disabled.',
      );
    });

    it('treats negative intervals as the default cadence (< 0 portion of `backupPollingInterval <= 0`)', () => {
      // The sibling test only used 0; these hit the strictly-negative side of
      // the `<= 0` guard, including a large negative that would otherwise be a
      // whole number of days.
      expect(getBackupIntervalSummary(true, -1)).toBe(
        'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).',
      );
      expect(getBackupIntervalSummary(true, -86400)).toBe(
        'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).',
      );
    });

    it('uses the singular "minute" for intervals that round to one minute (true arm of `minutes === 1`)', () => {
      // 60s: not divisible by 86400 or 3600, round(60/60) === 1 -> singular.
      expect(getBackupIntervalSummary(true, 60)).toBe('Pulse checks backups every minute.');
    });

    it('clamps sub-minute intervals up to one minute via Math.max (defensive clamp branch)', () => {
      // 1s -> round(1/60) === 0 -> Math.max(1, 0) === 1, so "minute" not "0 minutes".
      expect(getBackupIntervalSummary(true, 1)).toBe('Pulse checks backups every minute.');
      // 30s -> round(0.5) === 1 -> already 1; Math.max is a no-op here but the
      // path is shared with the clamp, so we pin it as the non-clamped sibling.
      expect(getBackupIntervalSummary(true, 30)).toBe('Pulse checks backups every minute.');
    });

    it('falls through the day and hour guards to the plural minutes fallback (false arms of `% 86400` and `% 3600`)', () => {
      // 5400s = 90 min: not divisible by an hour or a day -> minutes fallback, plural.
      expect(getBackupIntervalSummary(true, 5400)).toBe('Pulse checks backups every 90 minutes.');
      // 300s = 5 min: same fall-through, plural count of 5.
      expect(getBackupIntervalSummary(true, 300)).toBe('Pulse checks backups every 5 minutes.');
    });

    it('honors a truthy non-boolean flag as enabled (coercion through `!backupPollingEnabled`)', () => {
      // A non-empty string is truthy, so the disabled guard is skipped and the
      // interval is formatted normally. Cast satisfies the declared boolean type.
      expect(getBackupIntervalSummary('true' as unknown as boolean, 3600)).toBe(
        'Pulse checks backups every hour.',
      );
    });
  });
});
