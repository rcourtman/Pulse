import { describe, expect, it } from 'vitest';
import {
  getHistoryRangeDays,
  getUnlockedHistoryRangeOptions,
  resolveHistoryRangeWithinLimit,
  type HistoryRangeOption,
} from '@/components/Storage/historyRangeAccess';

const options: readonly HistoryRangeOption[] = [
  { value: '30m', label: '30m' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '14d', label: '14d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
];

describe('historyRangeAccess', () => {
  it('converts chart range tokens to day windows', () => {
    expect(getHistoryRangeDays('30m')).toBeCloseTo(1 / 48);
    expect(getHistoryRangeDays('24h')).toBe(1);
    expect(getHistoryRangeDays('14d')).toBe(14);
  });

  it('filters range options to the runtime history entitlement', () => {
    expect(getUnlockedHistoryRangeOptions(options, 7).map((option) => option.value)).toEqual([
      '30m',
      '24h',
      '7d',
    ]);
    expect(getUnlockedHistoryRangeOptions(options, 14).map((option) => option.value)).toEqual([
      '30m',
      '24h',
      '7d',
      '14d',
    ]);
    expect(getUnlockedHistoryRangeOptions(options, 90).map((option) => option.value)).toEqual([
      '30m',
      '24h',
      '7d',
      '14d',
      '30d',
      '90d',
    ]);
  });

  it('clamps a selected range down to the highest unlocked range', () => {
    expect(resolveHistoryRangeWithinLimit('90d', options, 7)).toBe('7d');
    expect(resolveHistoryRangeWithinLimit('90d', options, 14)).toBe('14d');
    expect(resolveHistoryRangeWithinLimit('90d', options, 90)).toBe('90d');
  });
});
