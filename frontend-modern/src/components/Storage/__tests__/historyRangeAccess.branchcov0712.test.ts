import { describe, expect, it } from 'vitest';
import {
  getHistoryRangeDays,
  resolveHistoryRangeWithinLimit,
  type HistoryRangeOption,
} from '@/components/Storage/historyRangeAccess';
import type { HistoryTimeRange } from '@/api/charts';

const options: readonly HistoryRangeOption[] = [
  { value: '30m', label: '30m' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '14d', label: '14d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
];

describe('getHistoryRangeDays (branch coverage)', () => {
  it('returns 0 when the range token does not match the <number><m|h|d> shape', () => {
    // Hits the `!match` early-return guard from several malformed shapes.
    expect(getHistoryRangeDays('' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('abc' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('12' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('7w' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('ms' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays(' 24h' as HistoryTimeRange)).toBe(0);
  });

  it('returns 0 when the numeric portion parses to a non-positive value', () => {
    // `0m` matches the regex, but parseInt yields 0 which trips `value <= 0`.
    expect(getHistoryRangeDays('0m' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('0h' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('0d' as HistoryTimeRange)).toBe(0);
    expect(getHistoryRangeDays('00d' as HistoryTimeRange)).toBe(0);
  });

  it('converts minutes to a fractional day value', () => {
    // `case 'm'` arm: value / (24 * 60).
    expect(getHistoryRangeDays('30m')).toBeCloseTo(1 / 48);
    expect(getHistoryRangeDays('1m' as HistoryTimeRange)).toBeCloseTo(1 / (24 * 60));
    expect(getHistoryRangeDays('60m' as HistoryTimeRange)).toBeCloseTo(1 / 24);
  });

  it('converts hours to a fractional day value', () => {
    // `case 'h'` arm: value / 24.
    expect(getHistoryRangeDays('24h')).toBe(1);
    expect(getHistoryRangeDays('12h' as HistoryTimeRange)).toBe(0.5);
    expect(getHistoryRangeDays('1h' as HistoryTimeRange)).toBeCloseTo(1 / 24);
  });

  it('passes through whole-day values unchanged', () => {
    // `case 'd'` arm: returns value verbatim.
    expect(getHistoryRangeDays('7d')).toBe(7);
    expect(getHistoryRangeDays('30d')).toBe(30);
    expect(getHistoryRangeDays('90d')).toBe(90);
  });
});

describe('resolveHistoryRangeWithinLimit (branch coverage)', () => {
  it('returns the current range verbatim when it is already inside the unlocked set', () => {
    // `available.some(...) === true` branch.
    // With max 7d, the available set is ['30m','24h','7d']; '7d' is a member.
    expect(resolveHistoryRangeWithinLimit('7d', options, 7)).toBe('7d');
    // A locked current range that still happens to be unlocked (e.g. 24h at 7d cap).
    expect(resolveHistoryRangeWithinLimit('24h', options, 7)).toBe('24h');
  });

  it('clamps down to the highest unlocked range when the current range is locked out', () => {
    // `available.some(...) === false` then `available.length > 0` branch.
    expect(resolveHistoryRangeWithinLimit('90d', options, 7)).toBe('7d');
    expect(resolveHistoryRangeWithinLimit('30d', options, 1)).toBe('24h');
  });

  it('falls back to the first option when no option is unlocked but options exist', () => {
    // `available.length === 0` then `options.length > 0` branch -> options[0].value.
    // maxHistoryDays 0 excludes every standard option (30m alone is ~0.0208 days).
    expect(resolveHistoryRangeWithinLimit('7d', options, 0)).toBe('30m');
    expect(resolveHistoryRangeWithinLimit('90d', options, 0)).toBe('30m');
  });

  it("falls back to '24h' when both available and options are empty", () => {
    // `available.length === 0` then `options.length === 0` branch -> literal '24h'.
    expect(resolveHistoryRangeWithinLimit('7d', [], 7)).toBe('24h');
    expect(resolveHistoryRangeWithinLimit('90d', [], 0)).toBe('24h');
  });

  it('clamps down to the last option when the available set is a strict subset', () => {
    // Distinct from the first clamp test: a middle current range ('14d') locked
    // out at max 1 day resolves to the highest unlocked ('24h'), exercising the
    // `available[available.length - 1].value` indexing with a multi-element set.
    expect(resolveHistoryRangeWithinLimit('14d', options, 1)).toBe('24h');
  });
});
