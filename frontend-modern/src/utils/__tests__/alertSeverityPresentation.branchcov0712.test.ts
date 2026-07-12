import { describe, expect, it } from 'vitest';
import {
  getAlertSeverityCompactLabel,
  getAlertSeverityIndicator,
  getAlertSeverityTextClass,
} from '@/utils/alertSeverityPresentation';

// `normalizeAlertSeverity` is a module-private (non-exported) helper, so its
// branches are exercised indirectly through the exported wrappers below. Each
// probe uses a wrapper whose output makes the normalized value observable.

describe('alertSeverityPresentation — branch coverage (batch 2)', () => {
  describe('getAlertSeverityIndicator — bucket ?? level / level ?? bucket', () => {
    it('uses bucket for variant and level for label when both are provided', () => {
      expect(getAlertSeverityIndicator('restart_loop', 'critical')).toStrictEqual({
        variant: 'danger',
        label: 'Restart Loop',
      });
    });

    it('falls back to level for both variant and label when bucket is undefined', () => {
      expect(getAlertSeverityIndicator('critical', undefined)).toStrictEqual({
        variant: 'danger',
        label: 'Critical',
      });
    });

    it('falls back to level for both variant and label when bucket is null', () => {
      expect(getAlertSeverityIndicator('warning', null)).toStrictEqual({
        variant: 'warning',
        label: 'Warning',
      });
    });

    it('falls back to bucket for both variant and label when level is undefined', () => {
      expect(getAlertSeverityIndicator(undefined, 'warning')).toStrictEqual({
        variant: 'warning',
        label: 'Warning',
      });
    });

    it('falls back to bucket for both variant and label when level is null', () => {
      expect(getAlertSeverityIndicator(null, 'info')).toStrictEqual({
        variant: 'muted',
        label: 'Info',
      });
    });

    it('returns muted/Info defaults when both level and bucket are undefined', () => {
      expect(getAlertSeverityIndicator(undefined, undefined)).toStrictEqual({
        variant: 'muted',
        label: 'Info',
      });
    });

    it('returns muted/Info defaults when both level and bucket are null', () => {
      expect(getAlertSeverityIndicator(null, null)).toStrictEqual({
        variant: 'muted',
        label: 'Info',
      });
    });

    it('treats empty-string bucket as a real (non-nullish) value for variant', () => {
      // `bucket ?? level`: '' is not nullish, so variant derives from '' →
      // normalize('') === '' → default 'muted'. `level ?? bucket`: level is
      // provided, so label still derives from level.
      expect(getAlertSeverityIndicator('critical', '')).toStrictEqual({
        variant: 'muted',
        label: 'Critical',
      });
    });

    it('treats empty-string level as a real (non-nullish) value for label', () => {
      // `level ?? bucket`: '' is not nullish, so label derives from '' →
      // formatAlertSeverityLabel('') returns the fallback 'Info'. `bucket ??
      // level`: bucket is provided, so variant still derives from bucket.
      expect(getAlertSeverityIndicator('', 'critical')).toStrictEqual({
        variant: 'danger',
        label: 'Info',
      });
    });
  });

  describe('getAlertSeverityTextClass — switch arms', () => {
    it('returns the full critical text class for "critical"', () => {
      expect(getAlertSeverityTextClass('critical')).toBe('text-red-600 dark:text-red-400');
    });

    it('returns the full warning text class for "warning"', () => {
      expect(getAlertSeverityTextClass('warning')).toBe('text-amber-600 dark:text-amber-400');
    });

    it('falls through to the default blue class for "info" (no explicit info case)', () => {
      expect(getAlertSeverityTextClass('info')).toBe('text-blue-600 dark:text-blue-400');
    });

    it('falls through to the default blue class for unknown severities', () => {
      expect(getAlertSeverityTextClass('maintenance')).toBe('text-blue-600 dark:text-blue-400');
    });
  });

  describe('normalizeAlertSeverity — nullish / trim / lowercase / coercion (via wrappers)', () => {
    it('coerces undefined to empty string via the nullish-coalesce arm (default switch arm)', () => {
      const level = undefined as unknown as Parameters<typeof getAlertSeverityTextClass>[0];
      expect(getAlertSeverityTextClass(level)).toBe('text-blue-600 dark:text-blue-400');
    });

    it('coerces null to empty string via the nullish-coalesce arm (default switch arm)', () => {
      const level = null as unknown as Parameters<typeof getAlertSeverityTextClass>[0];
      expect(getAlertSeverityTextClass(level)).toBe('text-blue-600 dark:text-blue-400');
    });

    it('keeps an empty string as empty (non-nullish) hitting the default arm', () => {
      expect(getAlertSeverityTextClass('')).toBe('text-blue-600 dark:text-blue-400');
    });

    it('trims surrounding whitespace then matches the critical arm', () => {
      expect(getAlertSeverityTextClass('  critical  ')).toBe('text-red-600 dark:text-red-400');
    });

    it('lowercases mixed-case input to match the warning arm', () => {
      expect(getAlertSeverityTextClass('Warning')).toBe('text-amber-600 dark:text-amber-400');
    });

    it('String()-coerces a non-string number (0) to "0" hitting the default arm', () => {
      const level = 0 as unknown as Parameters<typeof getAlertSeverityTextClass>[0];
      expect(getAlertSeverityTextClass(level)).toBe('text-blue-600 dark:text-blue-400');
    });
  });

  describe('getAlertSeverityCompactLabel — map hit vs uppercased fallback', () => {
    it('returns mapped compact labels for the three known severities', () => {
      expect(getAlertSeverityCompactLabel('critical')).toBe('CRIT');
      expect(getAlertSeverityCompactLabel('warning')).toBe('WARN');
      expect(getAlertSeverityCompactLabel('info')).toBe('INFO');
    });

    it('hits the map after normalizing whitespace and casing', () => {
      expect(getAlertSeverityCompactLabel('  Critical  ')).toBe('CRIT');
      expect(getAlertSeverityCompactLabel('WARNING')).toBe('WARN');
    });

    it('falls back to the uppercased ORIGINAL level for unknown severities', () => {
      expect(getAlertSeverityCompactLabel('maintenance')).toBe('MAINTENANCE');
    });

    it('preserves surrounding whitespace on the original level in the fallback', () => {
      // The map lookup uses the trimmed/lowercased `normalized`, but the
      // fallback uses the raw `level`, so whitespace survives uppercasing.
      expect(getAlertSeverityCompactLabel('  maintenance  ')).toBe('  MAINTENANCE  ');
    });

    it('String()-coerces a non-string number (0) to "0" in the fallback path', () => {
      const level = 0 as unknown as Parameters<typeof getAlertSeverityCompactLabel>[0];
      expect(getAlertSeverityCompactLabel(level)).toBe('0');
    });

    it('coerces null to "NULL" in the fallback path', () => {
      const level = null as unknown as Parameters<typeof getAlertSeverityCompactLabel>[0];
      expect(getAlertSeverityCompactLabel(level)).toBe('NULL');
    });

    it('coerces undefined to "UNDEFINED" in the fallback path', () => {
      const level = undefined as unknown as Parameters<typeof getAlertSeverityCompactLabel>[0];
      expect(getAlertSeverityCompactLabel(level)).toBe('UNDEFINED');
    });
  });
});
