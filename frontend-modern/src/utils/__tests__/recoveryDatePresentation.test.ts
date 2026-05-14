import { describe, expect, it } from 'vitest';
import {
  formatRecoveryTimeOnly,
  getRecoveryCompactAxisLabel,
  getRecoveryFilterDateLabel,
  getRecoveryFullDateLabel,
  getRecoveryNiceAxisMax,
  getRecoveryPrettyDateLabel,
  normalizeRecoveryDateSearchText,
  parseRecoveryDateKey,
  recoveryDateKeyFromTimestamp,
  resolveRecoveryDateSearchKey,
} from '@/utils/recoveryDatePresentation';

describe('recoveryDatePresentation', () => {
  it('builds and parses recovery date keys', () => {
    const timestamp = Date.UTC(2026, 2, 9, 12, 30, 0);
    const key = recoveryDateKeyFromTimestamp(timestamp);
    expect(key).toBe('2026-03-09');

    const parsed = parseRecoveryDateKey(key);
    expect(parsed.getFullYear()).toBe(2026);
    expect(parsed.getMonth()).toBe(2);
    expect(parsed.getDate()).toBe(9);
  });

  it('formats recovery date labels', () => {
    const key = '2026-03-09';
    expect(getRecoveryPrettyDateLabel(key)).toContain('Mar');
    expect(getRecoveryFullDateLabel(key)).toContain('2026');
    expect(getRecoveryFilterDateLabel(key)).toBe('Mar 9, 2026');
    expect(getRecoveryCompactAxisLabel(key, 30)).toBe('9');
    expect(getRecoveryCompactAxisLabel('2026-04-01', 30)).toBe('4/1');
    expect(getRecoveryCompactAxisLabel(key, 90)).toBe('3/9');
  });

  it('formats time-only and nice axis max values', () => {
    expect(formatRecoveryTimeOnly(null)).toBe('—');
    expect(formatRecoveryTimeOnly(Date.UTC(2026, 2, 9, 7, 5, 0))).toMatch(/\d{2}:\d{2}/);
    expect(getRecoveryNiceAxisMax(0)).toBe(1);
    expect(getRecoveryNiceAxisMax(3)).toBe(3);
    expect(getRecoveryNiceAxisMax(37)).toBe(50);
    expect(getRecoveryNiceAxisMax(101)).toBe(200);
  });

  it('resolves clear date search text to recovery day keys', () => {
    const candidateKeys = ['2026-02-13', '2026-02-14'];

    expect(normalizeRecoveryDateSearchText('Friday, Feb 14th')).toBe('friday feb 14');
    expect(resolveRecoveryDateSearchKey('2026-02-14', candidateKeys)).toBe('2026-02-14');
    expect(resolveRecoveryDateSearchKey('Feb 14', candidateKeys)).toBe('2026-02-14');
    expect(resolveRecoveryDateSearchKey('February 14, 2026', candidateKeys)).toBe('2026-02-14');
    expect(resolveRecoveryDateSearchKey('Feb 14', [], new Date(2026, 4, 14))).toBe('2026-02-14');
    expect(resolveRecoveryDateSearchKey('VM 123', candidateKeys)).toBeNull();
    expect(resolveRecoveryDateSearchKey('Feb', candidateKeys)).toBeNull();
  });
});
